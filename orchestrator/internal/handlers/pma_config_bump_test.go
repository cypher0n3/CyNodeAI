package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
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
