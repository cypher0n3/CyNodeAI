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

func TestControlPlaneNats_ActivityTouchesBinding(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	if _, err := db.UpsertSessionBinding(ctx, lineage, models.PMAServiceIDForBindingKey(key), models.SessionBindingStateActive); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC().Add(-30 * time.Minute)
	db.SessionBindingsByKey[key].LastActivityAt = &before
	db.SessionBindingsByKey[key].UpdatedAt = before

	env := natsutil.Envelope{
		EventType:    evSessionActivity,
		EventVersion: natsutil.EventVersionSessionV1,
		Payload: map[string]any{
			"binding_key": key,
		},
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleSessionActivityMessage(ctx, db, raw, nil); err != nil {
		t.Fatal(err)
	}
	b, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if b.LastActivityAt == nil || !b.LastActivityAt.After(before) {
		t.Fatalf("last activity not updated: before=%v after=%v", before, b.LastActivityAt)
	}
}

func TestControlPlaneNats_DetachedLogoutTeardown(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	if _, err := db.UpsertSessionBinding(ctx, lineage, models.PMAServiceIDForBindingKey(key), models.SessionBindingStateActive); err != nil {
		t.Fatal(err)
	}

	env := natsutil.Envelope{
		EventType:    evSessionDetached,
		EventVersion: natsutil.EventVersionSessionV1,
		Payload: map[string]any{
			"reason":     "logout",
			"session_id": rsID.String(),
			"user_id":    uid.String(),
		},
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleSessionActivityMessage(ctx, db, raw, nil); err != nil {
		t.Fatal(err)
	}
	b, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != models.SessionBindingStateTeardownPending {
		t.Fatalf("state %q want teardown_pending", b.State)
	}
	if LastPMATeardownForTest() == nil {
		t.Fatal("expected teardown record")
	}
}

func TestControlPlaneNats_AttachedTeardownPending(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	if _, err := db.UpsertSessionBinding(ctx, lineage, models.PMAServiceIDForBindingKey(key), models.SessionBindingStateTeardownPending); err != nil {
		t.Fatal(err)
	}
	env := natsutil.Envelope{
		EventType:    evSessionAttached,
		EventVersion: natsutil.EventVersionSessionV1,
		Payload:      map[string]any{"binding_key": key},
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	_ = HandleSessionActivityMessage(ctx, db, raw, nil)
}

func TestControlPlaneNats_ActivityEmptyBindingKeyNoOp(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	env := natsutil.Envelope{
		EventType:    evSessionActivity,
		EventVersion: natsutil.EventVersionSessionV1,
		Payload:      map[string]any{"binding_key": ""},
	}
	raw, _ := json.Marshal(env)
	if err := HandleSessionActivityMessage(ctx, db, raw, nil); err != nil {
		t.Fatal(err)
	}
}

func TestControlPlaneNats_DetachedInvalidSessionUUID(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	env := natsutil.Envelope{
		EventType:    evSessionDetached,
		EventVersion: natsutil.EventVersionSessionV1,
		Payload: map[string]any{
			"reason":     "logout",
			"session_id": "not-a-uuid",
			"user_id":    uuid.New().String(),
		},
	}
	raw, _ := json.Marshal(env)
	if err := HandleSessionActivityMessage(ctx, db, raw, nil); err == nil {
		t.Fatal("expected uuid parse error")
	}
}

func TestRunSessionActivityConsumer_nilConn(t *testing.T) {
	ctx := context.Background()
	RunSessionActivityConsumer(ctx, testutil.NewMockDB(), nil, nil)
}

func TestSubscribeCynodeSession_JetStreamUnavailable(t *testing.T) {
	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = false
	s := test.RunServer(&opts)
	defer s.Shutdown()
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	ctx := context.Background()
	_, err = subscribeCynodeSession(nc, ctx, testutil.NewMockDB(), nil)
	if err == nil {
		t.Fatal("expected error when JetStream disabled")
	}
}

func TestIssueNATSWithServiceJWT_tokenFuncError(t *testing.T) {
	iss, err := natsjwt.NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = issueNATSWithServiceJWT("nats://127.0.0.1:4222", iss, func(time.Time) (string, error) {
		return "", errors.New("no token")
	}, false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestControlPlaneNats_UnknownEventNoOp(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	env := natsutil.Envelope{
		EventType: "unknown.event",
		Payload:   map[string]any{"x": 1},
	}
	raw, _ := json.Marshal(env)
	if err := HandleSessionActivityMessage(ctx, db, raw, nil); err != nil {
		t.Fatal(err)
	}
}

func TestControlPlaneNats_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	if err := HandleSessionActivityMessage(ctx, db, []byte("{"), nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestIssueControlPlaneNATSConnection_nilIssuer(t *testing.T) {
	nc, js, err := IssueControlPlaneNATSConnection("nats://127.0.0.1:4222", nil)
	if err != nil || nc != nil || js != nil {
		t.Fatalf("want nils, got nc=%v js=%v err=%v", nc, js, err)
	}
}

func TestIssueControlPlaneNATSConnection_emptyURL(t *testing.T) {
	iss, err := natsjwt.NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	nc, js, err := IssueControlPlaneNATSConnection("", iss)
	if err != nil || nc != nil || js != nil {
		t.Fatalf("want nils, got err=%v", err)
	}
}

func testJetStreamServerURL(t *testing.T) (clientURL string, cleanup func()) {
	t.Helper()
	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	s := test.RunServer(&opts)
	return s.ClientURL(), func() { s.Shutdown() }
}

func TestIssueControlPlaneAndGatewayNATSConnection_embeddedServer(t *testing.T) {
	url, cleanup := testJetStreamServerURL(t)
	defer cleanup()
	iss, err := natsjwt.NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		dial func(string, *natsjwt.Issuer) (*nats.Conn, nats.JetStreamContext, error)
	}{
		{"control-plane", IssueControlPlaneNATSConnection},
		{"gateway", IssueGatewayNATSConnection},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nc, js, err := tc.dial(url, iss)
			if err != nil {
				t.Fatal(err)
			}
			if nc == nil || js == nil {
				t.Fatal("expected NATS connection and JetStream")
			}
			t.Cleanup(func() { nc.Close() })
		})
	}
}

func TestRunSessionActivityConsumer_smoke(t *testing.T) {
	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	s := test.RunServer(&opts)
	defer s.Shutdown()
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if err := natsutil.EnsureStreams(js); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		RunSessionActivityConsumer(ctx, testutil.NewMockDB(), nc, nil)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not exit after cancel")
	}
}
