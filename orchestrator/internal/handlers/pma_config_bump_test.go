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

func TestBumpPMAHostConfigVersion_NoHostResolved(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	t.Setenv("PMA_HOST_NODE_SLUG", "")
	ver, err := BumpPMAHostConfigVersion(ctx, db, nil)
	if err != nil || ver != "" {
		t.Fatalf("want empty version and nil error, got %q %v", ver, err)
	}
}

func TestBumpPMAHostConfigVersion_HostSlugNotInActiveNodes(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	nid := uuid.New()
	db.Nodes[nid] = &models.Node{
		NodeBase: models.NodeBase{NodeSlug: "other-slug", Status: models.NodeStatusActive},
		ID:       nid,
	}
	t.Setenv("PMA_HOST_NODE_SLUG", "unknown-slug")
	ver, err := BumpPMAHostConfigVersion(ctx, db, nil)
	if err != nil || ver != "" {
		t.Fatalf("want empty when host id not found, got %q %v", ver, err)
	}
}

func TestBumpPMAHostConfigVersion_ListActiveNodesError(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	nid := uuid.New()
	db.Nodes[nid] = &models.Node{
		NodeBase: models.NodeBase{NodeSlug: "pma-host", Status: models.NodeStatusActive},
		ID:       nid,
	}
	t.Setenv("PMA_HOST_NODE_SLUG", "pma-host")
	db.ForceError = errors.New("list nodes failed")
	if _, err := BumpPMAHostConfigVersion(ctx, db, nil); err == nil {
		t.Fatal("expected error from ListActiveNodes in BumpPMAHostConfigVersion")
	}
}

func TestConfigPushNats_BumpPMAHostConfigVersion(t *testing.T) {
	t.Cleanup(func() { SetJetStreamForConfigBump(nil) })

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
	SetJetStreamForConfigBump(js)

	ctx := context.Background()
	db := testutil.NewMockDB()
	nid := uuid.New()
	db.Nodes[nid] = &models.Node{
		NodeBase: models.NodeBase{NodeSlug: "pma-host", Status: models.NodeStatusActive},
		ID:       nid,
	}
	t.Setenv("PMA_HOST_NODE_SLUG", "pma-host")

	subj := "cynode.node.config_changed." + natsjwt.DefaultTenantID + "." + nid.String()
	sub, err := js.SubscribeSync(subj)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	ver, err := BumpPMAHostConfigVersion(ctx, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ver == "" {
		t.Fatal("expected non-empty config version")
	}

	msg, err := sub.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var env natsutil.Envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		t.Fatal(err)
	}
	if env.EventType != "node.config_changed" {
		t.Fatalf("event_type %q", env.EventType)
	}
	if env.Payload["node_id"] != nid.String() {
		t.Fatalf("node_id %v", env.Payload["node_id"])
	}
	if env.Payload["config_version"] != ver {
		t.Fatalf("config_version %v want %q", env.Payload["config_version"], ver)
	}
}
