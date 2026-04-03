package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"

	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func testJetStream(t *testing.T) (nc *nats.Conn, js nats.JetStreamContext, cleanup func()) {
	t.Helper()
	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	s := test.RunServer(&opts)
	var err error
	nc, err = nats.Connect(s.ClientURL())
	if err != nil {
		s.Shutdown()
		t.Fatal(err)
	}
	js, err = nc.JetStream()
	if err != nil {
		nc.Close()
		s.Shutdown()
		t.Fatal(err)
	}
	if err := natsutil.EnsureStreams(js); err != nil {
		nc.Close()
		s.Shutdown()
		t.Fatal(err)
	}
	cleanup = func() {
		nc.Close()
		s.Shutdown()
	}
	return nc, js, cleanup
}

func TestNewGatewaySessionPublisher_nilJetStream(t *testing.T) {
	if NewGatewaySessionPublisher(nil, nil) != nil {
		t.Fatal("expected nil publisher")
	}
}

func TestGatewaySessionActivity_PublishActivity_listError(t *testing.T) {
	nc, js, cleanup := testJetStream(t)
	defer cleanup()
	ctx := context.Background()
	db := testutil.NewMockDB()
	db.ForceError = errors.New("list bindings failed")
	g := NewGatewaySessionPublisher(nc, js)
	if err := g.PublishActivity(ctx, db, natsjwt.DefaultTenantID, uuid.New()); err == nil {
		t.Fatal("expected error")
	}
}

func TestGatewaySessionPublish_emptyTenantIDUsesDefault(t *testing.T) {
	nc, js, cleanup := testJetStream(t)
	defer cleanup()
	g := NewGatewaySessionPublisher(nc, js)
	sid := uuid.New().String()
	uid := uuid.New().String()
	bk := "bk-empty-tenant"
	sub, err := js.SubscribeSync("cynode.session.attached." + natsjwt.DefaultTenantID + "." + sid)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()
	if err := g.PublishAttached(context.Background(), "", sid, uid, bk); err != nil {
		t.Fatal(err)
	}
	if err := g.PublishDetached(context.Background(), "", sid, uid, bk, "logout"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid2 := uuid.New()
	rsID := uuid.MustParse(sid)
	lineage := models.SessionBindingLineage{UserID: uid2, SessionID: rsID, ThreadID: nil}
	if _, err := db.UpsertSessionBinding(ctx, lineage, "svc", models.SessionBindingStateActive); err != nil {
		t.Fatal(err)
	}
	subAct, err := js.SubscribeSync("cynode.session.activity." + natsjwt.DefaultTenantID + "." + sid)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subAct.Unsubscribe() }()
	if err := g.PublishActivity(ctx, db, "", uid2); err != nil {
		t.Fatal(err)
	}
}

func TestGatewaySessionActivity_PublishAttached(t *testing.T) {
	nc, js, cleanup := testJetStream(t)
	defer cleanup()

	g := NewGatewaySessionPublisher(nc, js)
	if g == nil {
		t.Fatal("expected publisher")
	}
	sid := uuid.New().String()
	uid := uuid.New().String()
	bk := "binding-key-test"
	subj := "cynode.session.attached." + natsjwt.DefaultTenantID + "." + sid
	sub, err := js.SubscribeSync(subj)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	if err := g.PublishAttached(context.Background(), natsjwt.DefaultTenantID, sid, uid, bk); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.NextMsg(3 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var env natsutil.Envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		t.Fatal(err)
	}
	if env.EventType != evSessionAttached {
		t.Fatalf("event_type %q", env.EventType)
	}
	if env.Payload["binding_key"] != bk {
		t.Fatalf("binding_key %v", env.Payload["binding_key"])
	}
}

func TestGatewaySessionActivity_PublishActivity(t *testing.T) {
	nc, js, cleanup := testJetStream(t)
	defer cleanup()

	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	if _, err := db.UpsertSessionBinding(ctx, lineage, models.PMAServiceIDForBindingKey(key), models.SessionBindingStateActive); err != nil {
		t.Fatal(err)
	}

	g := NewGatewaySessionPublisher(nc, js)
	sid := rsID.String()
	subj := "cynode.session.activity." + natsjwt.DefaultTenantID + "." + sid
	sub, err := js.SubscribeSync(subj)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	if err := g.PublishActivity(ctx, db, natsjwt.DefaultTenantID, uid); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.NextMsg(3 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var env natsutil.Envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		t.Fatal(err)
	}
	if env.EventType != evSessionActivity {
		t.Fatalf("event_type %q", env.EventType)
	}
}

func TestGatewaySessionActivity_PublishDetached(t *testing.T) {
	nc, js, cleanup := testJetStream(t)
	defer cleanup()

	g := NewGatewaySessionPublisher(nc, js)
	sid := uuid.New().String()
	uid := uuid.New().String()
	bk := "bk-detach"
	subj := "cynode.session.detached." + natsjwt.DefaultTenantID + "." + sid
	sub, err := js.SubscribeSync(subj)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	if err := g.PublishDetached(context.Background(), natsjwt.DefaultTenantID, sid, uid, bk, "logout"); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.NextMsg(3 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var env natsutil.Envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		t.Fatal(err)
	}
	if env.EventType != evSessionDetached {
		t.Fatalf("event_type %q", env.EventType)
	}
	if env.Payload["reason"] != "logout" {
		t.Fatalf("reason %v", env.Payload["reason"])
	}
}
