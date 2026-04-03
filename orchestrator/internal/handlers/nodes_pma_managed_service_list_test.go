package handlers

import (
	"context"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
	"github.com/google/uuid"
)

func TestBuildPMAManagedServiceList_SkipsStaleBindingWithoutRefreshSession(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	userID := uuid.New()
	sessID := uuid.New()
	nid := uuid.New()
	db.Nodes[nid] = &models.Node{
		NodeBase: models.NodeBase{NodeSlug: "pma-host", Status: models.NodeStatusActive},
		ID:       nid,
	}
	t.Setenv("PMA_HOST_NODE_SLUG", "pma-host")

	lineage := models.SessionBindingLineage{UserID: userID, SessionID: sessID, ThreadID: nil}
	if _, err := db.UpsertSessionBinding(ctx, lineage, "pma-sb-norefresh", models.SessionBindingStateActive); err != nil {
		t.Fatal(err)
	}

	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	svcs := h.buildPMAManagedServiceList(ctx, "pma-main", "img", "http://host:11434", "m", nil, "tok")
	if len(svcs) != 1 || svcs[0].ServiceID != poolServiceID(0) || svcs[0].Env != nil {
		t.Fatalf("want one idle warm slot after stale teardown, got %#v", svcs)
	}
	rec := LastPMATeardownForTest()
	if rec == nil || rec.Reason != "managed_services_refresh_missing" {
		t.Fatalf("expected teardown managed_services_refresh_missing, got %+v", rec)
	}
}

func TestBuildPMAManagedServiceList_IncludesBindingWhenRefreshSessionActive(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	userID := uuid.New()
	sessID := uuid.New()
	nid := uuid.New()
	db.Nodes[nid] = &models.Node{
		NodeBase: models.NodeBase{NodeSlug: "pma-host", Status: models.NodeStatusActive},
		ID:       nid,
	}
	t.Setenv("PMA_HOST_NODE_SLUG", "pma-host")

	lineage := models.SessionBindingLineage{UserID: userID, SessionID: sessID, ThreadID: nil}
	if _, err := db.UpsertSessionBinding(ctx, lineage, "pma-sb-ok", models.SessionBindingStateActive); err != nil {
		t.Fatal(err)
	}
	addTestRefreshSession(t, db, userID, sessID, []byte("hash"))

	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	svcs := h.buildPMAManagedServiceList(ctx, "pma-main", "img", "http://host:11434", "m", nil, "tok")
	if len(svcs) != 2 {
		t.Fatalf("want assigned + idle warm slot, got %d services", len(svcs))
	}
	if svcs[0].ServiceID != poolServiceID(0) || svcs[0].Env == nil {
		t.Fatalf("unexpected first service %#v", svcs[0])
	}
	if svcs[1].ServiceID != poolServiceID(1) || svcs[1].Env != nil {
		t.Fatalf("unexpected idle slot %#v", svcs[1])
	}
	if LastPMATeardownForTest() != nil {
		t.Fatalf("unexpected teardown %+v", LastPMATeardownForTest())
	}
}

func TestBuildPMAManagedServiceList_SkipsInactiveRefreshSession(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	userID := uuid.New()
	sessID := uuid.New()
	nid := uuid.New()
	db.Nodes[nid] = &models.Node{
		NodeBase: models.NodeBase{NodeSlug: "pma-host", Status: models.NodeStatusActive},
		ID:       nid,
	}
	t.Setenv("PMA_HOST_NODE_SLUG", "pma-host")

	lineage := models.SessionBindingLineage{UserID: userID, SessionID: sessID, ThreadID: nil}
	if _, err := db.UpsertSessionBinding(ctx, lineage, "pma-sb-inactive", models.SessionBindingStateActive); err != nil {
		t.Fatal(err)
	}
	addTestRefreshSession(t, db, userID, sessID, []byte("hash2"))
	if err := db.InvalidateRefreshSession(ctx, sessID); err != nil {
		t.Fatal(err)
	}

	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	svcs := h.buildPMAManagedServiceList(ctx, "pma-main", "img", "http://host:11434", "m", nil, "tok")
	if len(svcs) != 1 || svcs[0].ServiceID != poolServiceID(0) || svcs[0].Env != nil {
		t.Fatalf("want one idle warm slot after inactive teardown, got %#v", svcs)
	}
	rec := LastPMATeardownForTest()
	if rec == nil || rec.Reason != "managed_services_refresh_inactive" {
		t.Fatalf("expected teardown managed_services_refresh_inactive, got %+v", rec)
	}
}
