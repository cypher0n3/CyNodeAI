package natsutil_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"

	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
)

func TestPublishSessionLifecycle_JetStream(t *testing.T) {
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

	prod := natsutil.Producer{Service: "test", InstanceID: "i1"}
	scope := natsutil.Scope{TenantID: "t1", Sensitivity: "internal"}
	sid := "550e8400-e29b-41d4-a716-446655440000"
	corr := natsutil.Correlation{}

	ch := make(chan *nats.Msg, 4)
	sub, err := nc.ChanSubscribe("cynode.session.activity.t1."+sid, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = sub.Unsubscribe()
	}()

	ts := time.Now().UTC().Format(time.RFC3339)
	payload := natsutil.SessionActivityPayloadV1{
		SessionID:  sid,
		UserID:     "u1",
		BindingKey: "bk",
		ClientType: "cynork",
		Ts:         ts,
	}
	if err := natsutil.PublishSessionActivity(nc, js, "t1", sid, prod, scope, corr, &payload); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-ch:
		var env natsutil.Envelope
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			t.Fatal(err)
		}
		if env.EventType != "session.activity" {
			t.Fatalf("event_type %q", env.EventType)
		}
		if env.Payload["session_id"] != sid {
			t.Fatalf("payload session_id %v", env.Payload["session_id"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestPublishSessionAttached_Detached_ConfigChanged_JetStream(t *testing.T) {
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

	prod := natsutil.Producer{Service: "test", InstanceID: "i1"}
	scope := natsutil.Scope{TenantID: "t1", Sensitivity: "internal"}
	sid := "550e8400-e29b-41d4-a716-446655440001"
	corr := natsutil.Correlation{}
	ts := time.Now().UTC().Format(time.RFC3339)

	if err := natsutil.PublishSessionAttached(nc, js, "t1", sid, prod, scope, corr, &natsutil.SessionAttachedPayloadV1{
		SessionID: sid, UserID: "u1", BindingKey: "bk", ClientType: "http", Ts: ts,
	}); err != nil {
		t.Fatal(err)
	}
	if err := natsutil.PublishSessionDetached(nc, js, "t1", sid, prod, scope, corr, &natsutil.SessionDetachedPayloadV1{
		SessionID: sid, UserID: "u1", BindingKey: "bk", Reason: "logout", Ts: ts,
	}); err != nil {
		t.Fatal(err)
	}
	if err := natsutil.PublishConfigChanged(js, "t1", "node-1", prod, scope, corr, &natsutil.NodeConfigChangedPayloadV1{
		NodeID: "node-1", ConfigVersion: "v2", Ts: ts,
	}); err != nil {
		t.Fatal(err)
	}
}
