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

func makeCapabilityReportWithPMAReady(nodeSlug string) nodepayloads.CapabilityReport {
	return nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node:       nodepayloads.CapabilityNode{NodeSlug: nodeSlug},
		Platform:   nodepayloads.Platform{OS: "linux", Arch: "amd64"},
		Compute:    nodepayloads.Compute{CPUCores: 4, RAMMB: 8192},
		ManagedServicesStatus: &nodepayloads.ManagedServicesStatus{
			Services: []nodepayloads.ManagedServiceStatus{
				{
					ServiceID:   "pma-main",
					ServiceType: "pma",
					State:       "ready",
					Endpoints: []string{
						"http://worker.local/v1/worker/managed-services/pma-main/proxy:http",
					},
					ReadyAt: time.Now().UTC().Format(time.RFC3339),
				},
			},
		},
	}
}

func TestResolvePMAEndpoint_FromManagedServicesStatus(t *testing.T) {
	db := testutil.NewMockDB()
	nodeID := uuid.New()
	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "node-01",
			Status:   models.NodeStatusActive,
		},
		ID:        nodeID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	db.AddNode(node)
	report := makeCapabilityReportWithPMAReady("node-01")
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if saveErr := db.SaveNodeCapabilitySnapshot(context.Background(), nodeID, string(raw)); saveErr != nil {
		t.Fatalf("save capability snapshot: %v", saveErr)
	}
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	got := h.resolvePMAEndpoint(context.Background())
	want := "http://worker.local/v1/worker/managed-services/pma-main/proxy:http"
	if got != want {
		t.Errorf("resolvePMAEndpoint() = %q, want %q", got, want)
	}
}

func TestResolvePMAEndpoint_RequiresReadyService(t *testing.T) {
	db := testutil.NewMockDB()
	nodeID := uuid.New()
	db.AddNode(&models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "node-02",
			Status:   models.NodeStatusActive,
		},
		ID:        nodeID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	report := nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node:       nodepayloads.CapabilityNode{NodeSlug: "node-02"},
		Platform:   nodepayloads.Platform{OS: "linux", Arch: "amd64"},
		Compute:    nodepayloads.Compute{CPUCores: 4, RAMMB: 8192},
		ManagedServicesStatus: &nodepayloads.ManagedServicesStatus{
			Services: []nodepayloads.ManagedServiceStatus{
				{ServiceID: "pma-main", ServiceType: "pma", State: "starting", Endpoints: []string{"http://ignored"}},
			},
		},
	}
	raw, _ := json.Marshal(report)
	_ = db.SaveNodeCapabilitySnapshot(context.Background(), nodeID, string(raw))
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	if got := h.resolvePMAEndpoint(context.Background()); got != "" {
		t.Errorf("resolvePMAEndpoint() = %q, want empty for non-ready service", got)
	}
}

func TestResolvePMAEndpoint_PicksMostRecentReadyAt(t *testing.T) {
	db := testutil.NewMockDB()
	now := time.Now().UTC()
	addNodeWithReady := func(slug, endpoint string, readyAt time.Time) {
		nodeID := uuid.New()
		db.AddNode(&models.Node{
			NodeBase: models.NodeBase{
				NodeSlug: slug,
				Status:   models.NodeStatusActive,
			},
			ID:        nodeID,
			CreatedAt: now,
			UpdatedAt: now,
		})
		report := nodepayloads.CapabilityReport{
			Version:    1,
			ReportedAt: now.Format(time.RFC3339),
			Node:       nodepayloads.CapabilityNode{NodeSlug: slug},
			Platform:   nodepayloads.Platform{OS: "linux", Arch: "amd64"},
			Compute:    nodepayloads.Compute{CPUCores: 4, RAMMB: 8192},
			ManagedServicesStatus: &nodepayloads.ManagedServicesStatus{
				Services: []nodepayloads.ManagedServiceStatus{
					{
						ServiceID:   "pma-main",
						ServiceType: "pma",
						State:       "ready",
						Endpoints:   []string{endpoint},
						ReadyAt:     readyAt.Format(time.RFC3339),
					},
				},
			},
		}
		raw, _ := json.Marshal(report)
		_ = db.SaveNodeCapabilitySnapshot(context.Background(), nodeID, string(raw))
	}
	addNodeWithReady("node-old", "http://old", now.Add(-10*time.Minute))
	addNodeWithReady("node-new", "http://new", now)
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	if got := h.resolvePMAEndpoint(context.Background()); got != "http://new" {
		t.Errorf("resolvePMAEndpoint() = %q, want most recent endpoint", got)
	}
}

func TestResolvePMAEndpointCandidate_UsesNodeWorkerBearerToken(t *testing.T) {
	db := testutil.NewMockDB()
	nodeID := uuid.New()
	workerToken := "rotated-worker-token"
	db.AddNode(&models.Node{
		NodeBase: models.NodeBase{
			NodeSlug:             "node-token",
			Status:               models.NodeStatusActive,
			WorkerAPIBearerToken: &workerToken,
		},
		ID:        nodeID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	report := makeCapabilityReportWithPMAReady("node-token")
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if err := db.SaveNodeCapabilitySnapshot(context.Background(), nodeID, string(raw)); err != nil {
		t.Fatalf("save capability snapshot: %v", err)
	}
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "fallback-global-token")
	got := h.resolvePMAEndpointCandidate(context.Background())
	if got.endpoint == "" {
		t.Fatalf("resolvePMAEndpointCandidate() returned empty endpoint")
	}
	if got.workerAPIBearerToken != workerToken {
		t.Fatalf("resolvePMAEndpointCandidate() worker token=%q want=%q", got.workerAPIBearerToken, workerToken)
	}
}
