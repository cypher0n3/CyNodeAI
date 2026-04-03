package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestPmaRouting_BySessionBindingServiceID(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	svcID := poolServiceID(0)
	now := time.Now().UTC()
	db.SessionBindingsByKey[key] = &models.SessionBinding{
		SessionBindingBase: models.SessionBindingBase{
			BindingKey: key,
			UserID:     uid,
			SessionID:  rsID,
			ServiceID:  svcID,
			State:      models.SessionBindingStateActive,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	nodeID := uuid.New()
	n1 := &models.Node{
		NodeBase:  models.NodeBase{NodeSlug: "n1"},
		ID:        nodeID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	testutil.ApplyDispatchableWorkerFields(&n1.NodeBase, "", "")
	db.AddNode(n1)
	report := nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: now.Format(time.RFC3339),
		Node:       nodepayloads.CapabilityNode{NodeSlug: "n1"},
		ManagedServicesStatus: &nodepayloads.ManagedServicesStatus{
			Services: []nodepayloads.ManagedServiceStatus{
				{
					ServiceID:   svcID,
					ServiceType: "pma",
					State:       "ready",
					Endpoints:   []string{"http://bound-pma/proxy"},
					ReadyAt:     now.Format(time.RFC3339),
				},
				{
					ServiceID:   poolServiceID(1),
					ServiceType: "pma",
					State:       "ready",
					Endpoints:   []string{"http://wrong-pma/proxy"},
					ReadyAt:     now.Add(time.Minute).Format(time.RFC3339),
				},
			},
		},
	}
	raw, _ := json.Marshal(report)
	_ = db.SaveNodeCapabilitySnapshot(ctx, nodeID, string(raw))
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	c := h.resolvePMAEndpointCandidate(ctx, uid)
	if c.endpoint != "http://bound-pma/proxy" {
		t.Fatalf("endpoint %q want bound instance", c.endpoint)
	}
	if c.serviceID != svcID {
		t.Fatalf("service_id %q", c.serviceID)
	}
}

func TestPmaRouting_StaleBindingDoesNotFallBackToOtherPMA(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	svcID := poolServiceID(0)
	now := time.Now().UTC()
	db.SessionBindingsByKey[key] = &models.SessionBinding{
		SessionBindingBase: models.SessionBindingBase{
			BindingKey: key,
			UserID:     uid,
			SessionID:  rsID,
			ServiceID:  svcID,
			State:      models.SessionBindingStateActive,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	nodeID := uuid.New()
	n1b := &models.Node{
		NodeBase:  models.NodeBase{NodeSlug: "n1"},
		ID:        nodeID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	testutil.ApplyDispatchableWorkerFields(&n1b.NodeBase, "", "")
	db.AddNode(n1b)
	report := nodepayloads.CapabilityReport{
		Version: 1,
		Node:    nodepayloads.CapabilityNode{NodeSlug: "n1"},
		ManagedServicesStatus: &nodepayloads.ManagedServicesStatus{
			Services: []nodepayloads.ManagedServiceStatus{
				{ServiceID: "other", ServiceType: "pma", State: "ready", Endpoints: []string{"http://x"}, ReadyAt: now.Format(time.RFC3339)},
			},
		},
	}
	raw, _ := json.Marshal(report)
	_ = db.SaveNodeCapabilitySnapshot(ctx, nodeID, string(raw))
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	c := h.resolvePMAEndpointCandidate(ctx, uid)
	if c.endpoint != "" {
		t.Fatalf("expected empty endpoint when binding service_id missing from snapshot, got %q", c.endpoint)
	}
	if c.serviceID != "" {
		t.Fatalf("expected empty service_id, got %q", c.serviceID)
	}
}
