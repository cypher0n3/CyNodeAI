package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestGreedyProvision_PersistsBindingAndMCPIntent(t *testing.T) {
	t.Cleanup(ResetGreedyPMAIssueForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	if err := GreedyProvisionPMAAfterInteractiveSession(ctx, db, uid, rsID, nil); err != nil {
		t.Fatalf("GreedyProvisionPMAAfterInteractiveSession: %v", err)
	}
	key := models.DeriveSessionBindingKey(models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil})
	b, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatalf("GetSessionBindingByKey: %v", err)
	}
	if b.ServiceID != models.PMAServiceIDForBindingKey(key) {
		t.Fatalf("service_id: %q", b.ServiceID)
	}
	issue := LastGreedyPMAIssueForTest()
	if issue == nil {
		t.Fatal("expected greedy issue record")
	}
	if issue.InvocationClass != PMACredentialInvocationClassGatewaySession {
		t.Fatalf("invocation class: %q", issue.InvocationClass)
	}
}

func TestGreedyProvision_BumpsConfigOnPMAHost(t *testing.T) {
	t.Cleanup(ResetGreedyPMAIssueForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	nid := uuid.New()
	db.Nodes[nid] = &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "alpha-node",
			Status:   models.NodeStatusActive,
		},
		ID: nid,
	}
	uid := uuid.New()
	rsID := uuid.New()
	t.Setenv("PMA_HOST_NODE_SLUG", "alpha-node")
	if err := GreedyProvisionPMAAfterInteractiveSession(ctx, db, uid, rsID, nil); err != nil {
		t.Fatalf("GreedyProvisionPMAAfterInteractiveSession: %v", err)
	}
	issue := LastGreedyPMAIssueForTest()
	if issue == nil || issue.ConfigVersionULID == "" {
		t.Fatalf("expected config version bump, issue=%v", issue)
	}
	updated := db.Nodes[nid]
	if updated.ConfigVersion == nil || *updated.ConfigVersion != issue.ConfigVersionULID {
		t.Fatalf("node config_version not updated: %v", updated.ConfigVersion)
	}
}

func TestGreedyProvision_UpsertSessionBindingError(t *testing.T) {
	t.Cleanup(ResetGreedyPMAIssueForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	db.ForceError = errors.New("upsert failed")
	if err := GreedyProvisionPMAAfterInteractiveSession(ctx, db, uuid.New(), uuid.New(), nil); err == nil {
		t.Fatal("expected error from UpsertSessionBinding")
	}
}

func TestGreedyProvision_InteractiveSessionWithoutChat(t *testing.T) {
	// REQ-ORCHES-0190: provisioning must not wait for the first cynodeai.pm chat message.
	// GreedyProvisionPMAAfterInteractiveSession is invoked from login/refresh only; this test names that contract.
	t.Cleanup(ResetGreedyPMAIssueForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	if err := GreedyProvisionPMAAfterInteractiveSession(ctx, db, uuid.New(), uuid.New(), nil); err != nil {
		t.Fatal(err)
	}
	if LastGreedyPMAIssueForTest() == nil {
		t.Fatal("expected binding + MCP intent without any chat handler")
	}
}
