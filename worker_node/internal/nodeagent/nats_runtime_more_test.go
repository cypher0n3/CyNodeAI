package nodeagent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/natsconfig"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

func TestParseNodeIDFromNodeJWT_Errors(t *testing.T) {
	t.Parallel()
	if _, err := ParseNodeIDFromNodeJWT(""); err == nil {
		t.Fatal("empty")
	}
	if _, err := ParseNodeIDFromNodeJWT("a.b"); err == nil {
		t.Fatal("two parts")
	}
	if _, err := ParseNodeIDFromNodeJWT("a.not-base64!.c"); err == nil {
		t.Fatal("bad payload encoding")
	}
	if _, err := ParseNodeIDFromNodeJWT("a.e30.c"); err == nil {
		t.Fatal("empty json object missing sub")
	}
	badSub := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiJub3QtYS11dWlkIn0." +
		"c2ln"
	if _, err := ParseNodeIDFromNodeJWT(badSub); err == nil {
		t.Fatal("bad uuid in sub")
	}
}

func TestNewNatsRuntime_NilBootstrap(t *testing.T) {
	t.Parallel()
	r, err := NewNatsRuntime(t.Context(), nil, nil, uuid.New(), "")
	if err != nil || r != nil {
		t.Fatalf("got (%v, %v)", r, err)
	}
}

func TestNewNatsRuntime_InvalidNatsValidate(t *testing.T) {
	t.Parallel()
	b := &BootstrapData{
		Nats: &natsconfig.ClientCredentials{
			URL:          "",
			JWT:          "x",
			JWTExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	_, err := NewNatsRuntime(t.Context(), nil, b, uuid.New(), "slug")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNatsRuntime_instanceID(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	r := &NatsRuntime{nodeID: id, slug: "my-node"}
	if got := r.instanceID(); got != "my-node" {
		t.Fatalf("slug: got %q", got)
	}
	r2 := &NatsRuntime{nodeID: id}
	if got := r2.instanceID(); got != id.String() {
		t.Fatalf("node id: got %q", got)
	}
}

func TestNatsRuntime_CloseNil(t *testing.T) {
	t.Parallel()
	var r *NatsRuntime
	r.Close()
}

func TestNatsRuntime_StartConfigSubscriber_NilBump(t *testing.T) {
	t.Parallel()
	r := &NatsRuntime{}
	r.StartConfigSubscriber(t.Context(), nil, nil)
}

func TestNatsRuntime_RunSessionActivityLoop_Nil(t *testing.T) {
	t.Parallel()
	var r *NatsRuntime
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r.RunSessionActivityLoop(ctx, nil, func() *nodepayloads.NodeConfigurationPayload { return nil })
}

func TestNatsRuntime_publishSessionActivity_SkipsNonPMAWithJetStream(t *testing.T) {
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
	r := &NatsRuntime{
		js:     js,
		nodeID: uuid.New(),
	}
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "x", ServiceType: "other", Env: map[string]string{"CYNODE_SESSION_ID": "s"}},
			},
		},
	}
	if err := r.publishSessionActivity(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestNatsRuntime_publishSessionActivity_EmptyTenantUsesDefault(t *testing.T) {
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
	sid := "550e8400-e29b-41d4-a716-446655440012"
	subj := "cynode.session.activity." + defaultTenantID + "." + sid
	ch := make(chan *nats.Msg, 2)
	sub, err := nc.ChanSubscribe(subj, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	r := &NatsRuntime{
		js:     js,
		nodeID: uuid.New(),
		slug:   "n1",
	}
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "pma-sb",
					ServiceType: strings.ToLower(serviceTypePMA),
					Env: map[string]string{
						"CYNODE_SESSION_ID":  sid,
						"CYNODE_USER_ID":     "22222222-2222-4222-8222-222222222222",
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
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
