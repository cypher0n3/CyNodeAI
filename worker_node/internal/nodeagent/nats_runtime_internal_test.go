package nodeagent

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

func TestParseNodeIDFromNodeJWT(t *testing.T) {
	t.Parallel()
	sub := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	payload, err := json.Marshal(map[string]string{"sub": sub.String()})
	if err != nil {
		t.Fatal(err)
	}
	tok := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		base64.RawURLEncoding.EncodeToString(payload) +
		".sig"
	got, err := ParseNodeIDFromNodeJWT(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got != sub {
		t.Fatalf("got %v want %v", got, sub)
	}
}

func TestNewNatsRuntime_NoNatsBlock(t *testing.T) {
	t.Parallel()
	r, err := NewNatsRuntime(t.Context(), nil, &BootstrapData{}, uuid.Nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if r != nil {
		t.Fatal("expected nil runtime")
	}
}

func TestWorkerNatsConn_SessionActivityPublish(t *testing.T) {
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

	sid := "550e8400-e29b-41d4-a716-446655440000"
	subj := "cynode.session.activity." + defaultTenantID + "." + sid
	ch := make(chan *nats.Msg, 2)
	sub, err := nc.ChanSubscribe(subj, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	r := &NatsRuntime{
		js:     js,
		nodeID: uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		slug:   "n1",
	}
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "pma-sb-abc",
					ServiceType: serviceTypePMA,
					Env: map[string]string{
						"CYNODE_SESSION_ID":  sid,
						"CYNODE_USER_ID":     "22222222-2222-4222-8222-222222222222",
						"CYNODE_TENANT_ID":   defaultTenantID,
						"CYNODE_BINDING_KEY": "bk",
					},
				},
			},
		},
	}
	if err := r.publishSessionActivity(cfg); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-ch:
		if len(msg.Data) == 0 {
			t.Fatal("empty message")
		}
		if !strings.Contains(string(msg.Data), "session.activity") {
			t.Fatalf("unexpected payload: %s", msg.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for relayed activity")
	}
}
