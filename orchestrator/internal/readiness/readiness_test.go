package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

// erroringStore implements database.Store by delegating to testutil.MockDB but
// returning injected errors for ListDispatchableNodes and/or HasAnyActiveApiCredential.
type erroringStore struct {
	*testutil.MockDB
	listErr error
	credErr error
}

func (s *erroringStore) ListDispatchableNodes(ctx context.Context) ([]*models.Node, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.MockDB.ListDispatchableNodes(ctx)
}

func (s *erroringStore) HasAnyActiveApiCredential(ctx context.Context) (bool, error) {
	if s.credErr != nil {
		return false, s.credErr
	}
	return s.MockDB.HasAnyActiveApiCredential(ctx)
}

const (
	testConfigAckApplied = "applied"
	testWorkerURL        = "http://worker:8080"
	testBearerToken      = "bearer"
)

func TestInferencePathAvailable_NoNodes(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	ok, err := InferencePathAvailable(ctx, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false when no nodes and no credentials")
	}
}

func TestInferencePathAvailable_WithDispatchableNode(t *testing.T) {
	mock := testutil.NewMockDB()
	nodeID := uuid.New()
	mock.Nodes[nodeID] = &models.Node{
		NodeBase: models.NodeBase{
			Status:               models.NodeStatusActive,
			ConfigAckStatus:      ptr(testConfigAckApplied),
			WorkerAPITargetURL:   ptr(testWorkerURL),
			WorkerAPIBearerToken: ptr(testBearerToken),
		},
		ID: nodeID,
	}
	ctx := context.Background()
	ok, err := InferencePathAvailable(ctx, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true when dispatchable node exists")
	}
}

func TestInferencePathAvailable_WithActiveCredential(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.HasAnyActiveApiCredentialResult = true
	ctx := context.Background()
	ok, err := InferencePathAvailable(ctx, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true when HasAnyActiveApiCredential is true")
	}
}

func TestInferencePathAvailable_StoreErrors(t *testing.T) {
	tests := []struct {
		name  string
		store *erroringStore
	}{
		{"ListDispatchableNodes error", &erroringStore{MockDB: testutil.NewMockDB(), listErr: errors.New("list err")}},
		{"HasAnyActiveApiCredential error", &erroringStore{MockDB: testutil.NewMockDB(), credErr: errors.New("cred err")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ok, err := InferencePathAvailable(ctx, tt.store)
			if err == nil {
				t.Error("expected error")
			}
			if ok {
				t.Error("expected false on error")
			}
		})
	}
}

func TestHasWorkerReportedPMAReady_NoNodes(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	if HasWorkerReportedPMAReady(ctx, mock) {
		t.Error("expected false when no nodes")
	}
}

func ptr(s string) *string { return &s }

func TestHasWorkerReportedPMAReady_WithPMAReady(t *testing.T) {
	mock := testutil.NewMockDB()
	nodeID := uuid.New()
	mock.Nodes[nodeID] = &models.Node{
		NodeBase: models.NodeBase{
			Status:               models.NodeStatusActive,
			ConfigAckStatus:      ptr(testConfigAckApplied),
			WorkerAPITargetURL:   ptr(testWorkerURL),
			WorkerAPIBearerToken: ptr(testBearerToken),
		},
		ID: nodeID,
	}
	report := nodepayloads.CapabilityReport{
		ManagedServicesStatus: &nodepayloads.ManagedServicesStatus{
			Services: []nodepayloads.ManagedServiceStatus{
				{ServiceType: "pma", State: "ready", Endpoints: []string{"http://pma:11434"}},
			},
		},
	}
	capJSON, _ := json.Marshal(report)
	_ = mock.SaveNodeCapabilitySnapshot(context.Background(), nodeID, string(capJSON))
	ctx := context.Background()
	if !HasWorkerReportedPMAReady(ctx, mock) {
		t.Error("expected true when node has PMA ready in snapshot")
	}
}

func TestHasWorkerReportedPMAReady_NoPMAInSnapshot(t *testing.T) {
	mock := testutil.NewMockDB()
	nodeID := uuid.New()
	mock.Nodes[nodeID] = &models.Node{
		NodeBase: models.NodeBase{
			Status:               models.NodeStatusActive,
			ConfigAckStatus:      ptr(testConfigAckApplied),
			WorkerAPITargetURL:   ptr(testWorkerURL),
			WorkerAPIBearerToken: ptr(testBearerToken),
		},
		ID: nodeID,
	}
	report := nodepayloads.CapabilityReport{
		ManagedServicesStatus: &nodepayloads.ManagedServicesStatus{
			Services: []nodepayloads.ManagedServiceStatus{
				{ServiceType: "other", State: "ready", Endpoints: []string{"http://x:1"}},
			},
		},
	}
	capJSON, _ := json.Marshal(report)
	_ = mock.SaveNodeCapabilitySnapshot(context.Background(), nodeID, string(capJSON))
	ctx := context.Background()
	if HasWorkerReportedPMAReady(ctx, mock) {
		t.Error("expected false when no PMA service in snapshot")
	}
}

func TestPMASubprocessReady_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	ctx := context.Background()
	// listenAddr format "host:port"; use the server's listener addr
	addr := srv.Listener.Addr().String()
	if !PMASubprocessReady(ctx, addr) {
		t.Error("expected true when healthz returns 200")
	}
}

func TestPMASubprocessReady_NoColon(t *testing.T) {
	ctx := context.Background()
	if PMASubprocessReady(ctx, "no-colon") {
		t.Error("expected false when listenAddr has no colon")
	}
}

func TestPMASubprocessReady_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	ctx := context.Background()
	addr := srv.Listener.Addr().String()
	if PMASubprocessReady(ctx, addr) {
		t.Error("expected false when healthz returns non-200")
	}
}

func TestPMASubprocessReady_EmptyPort(t *testing.T) {
	ctx := context.Background()
	if PMASubprocessReady(ctx, "127.0.0.1:") {
		t.Error("expected false when port is empty")
	}
}

func TestHasWorkerReportedPMAReady_InvalidSnapshotJSON(t *testing.T) {
	mock := testutil.NewMockDB()
	nodeID := uuid.New()
	mock.Nodes[nodeID] = &models.Node{
		NodeBase: models.NodeBase{
			Status:               models.NodeStatusActive,
			ConfigAckStatus:      ptr(testConfigAckApplied),
			WorkerAPITargetURL:   ptr(testWorkerURL),
			WorkerAPIBearerToken: ptr(testBearerToken),
		},
		ID: nodeID,
	}
	_ = mock.SaveNodeCapabilitySnapshot(context.Background(), nodeID, `{invalid json}`)
	ctx := context.Background()
	if HasWorkerReportedPMAReady(ctx, mock) {
		t.Error("expected false when snapshot JSON is invalid")
	}
}

func TestHasWorkerReportedPMAReady_NilManagedServicesStatus(t *testing.T) {
	mock := testutil.NewMockDB()
	nodeID := uuid.New()
	mock.Nodes[nodeID] = &models.Node{
		NodeBase: models.NodeBase{
			Status:               models.NodeStatusActive,
			ConfigAckStatus:      ptr(testConfigAckApplied),
			WorkerAPITargetURL:   ptr(testWorkerURL),
			WorkerAPIBearerToken: ptr(testBearerToken),
		},
		ID: nodeID,
	}
	report := nodepayloads.CapabilityReport{ManagedServicesStatus: nil}
	capJSON, _ := json.Marshal(report)
	_ = mock.SaveNodeCapabilitySnapshot(context.Background(), nodeID, string(capJSON))
	ctx := context.Background()
	if HasWorkerReportedPMAReady(ctx, mock) {
		t.Error("expected false when ManagedServicesStatus is nil")
	}
}
