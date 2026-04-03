package handlers

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
	"github.com/google/uuid"
)

func TestVariantAndVRAM_FeaturesOnlyNoVRAM(t *testing.T) {
	// Device has Features (rocm_version) but no vram_mb: use first recognizable device.
	report := &nodepayloads.CapabilityReport{
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{
				{Vendor: "AMD", Features: map[string]interface{}{"rocm_version": "5.0"}},
			},
		},
	}
	variant, vramMB := variantAndVRAM(report)
	if variant != ollamaVariantROCm {
		t.Errorf("variant = %q, want %s (features-only fallback)", variant, ollamaVariantROCm)
	}
	if vramMB != 0 {
		t.Errorf("vramMB = %d, want 0", vramMB)
	}
}

func TestDeriveInferenceBackend_ExistingServiceNotEnabled(t *testing.T) {
	// When ExistingService==true the backend config should be returned (for DesiredModels/Env)
	// but with Enabled=false so node-manager does not try to start a second Ollama container.
	nodeID := uuid.New()
	report := nodepayloads.CapabilityReport{
		Inference: &nodepayloads.InferenceInfo{Supported: true, ExistingService: true},
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{
				{VRAMMB: 20480, Features: map[string]interface{}{"rocm_version": "6.0"}},
			},
		},
	}
	raw, _ := json.Marshal(report)
	db := testutil.NewMockDB()
	db.CapabilityHistory = append(db.CapabilityHistory, &testutil.NodeCapabilitySnapshot{
		NodeID:         nodeID,
		CapabilityJSON: string(raw),
		CreatedAt:      time.Now(),
	})
	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	backend := h.deriveInferenceBackend(t.Context(), nodeID)
	if backend == nil {
		t.Fatal("expected non-nil backend config even when ExistingService=true")
	}
	if backend.Enabled {
		t.Error("Enabled should be false when ExistingService=true")
	}
	if backend.SelectedModel == "" {
		t.Error("SelectedModel should be populated even when ExistingService=true")
	}
	if _, ok := backend.Env["OLLAMA_NUM_CTX"]; !ok {
		t.Error("Env should contain OLLAMA_NUM_CTX even when ExistingService=true")
	}
}

func TestSelectPMAModel_AlwaysPicksTopTierRegardlessOfAvailable(t *testing.T) {
	// Orchestrator picks the top VRAM-tier candidate regardless of availability on the node.
	// Node-manager is responsible for pulling; the orchestrator must not fall back to
	// whatever happens to be installed.
	_ = os.Unsetenv("INFERENCE_MODEL")
	tests := []struct {
		name      string
		available []string
		vramMB    int
		wantFirst bool // want candidates[0] for the tier
	}{
		{
			name:      "top tier even when only fallback available",
			available: []string{"qwen3:8b", "qwen3.5:0.8b"},
			vramMB:    20464,
		},
		{
			name:      "top tier when desired already available",
			available: []string{"qwen3.5:9b", "qwen3:8b"},
			vramMB:    20464,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodeID := uuid.New()
			report := nodepayloads.CapabilityReport{
				Inference: &nodepayloads.InferenceInfo{Supported: true, AvailableModels: tc.available},
				GPU: &nodepayloads.GPUInfo{
					Present: true,
					Devices: []nodepayloads.GPUDevice{
						{VRAMMB: tc.vramMB, Features: map[string]interface{}{"rocm_version": "6.0"}},
					},
				},
			}
			raw, _ := json.Marshal(report)
			db := testutil.NewMockDB()
			db.CapabilityHistory = append(db.CapabilityHistory, &testutil.NodeCapabilitySnapshot{
				NodeID:         nodeID,
				CapabilityJSON: string(raw),
				CreatedAt:      time.Now(),
			})
			h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
			got := h.selectPMAModel(t.Context(), nodeID)
			want := pmaModelCandidates(tc.vramMB)[0]
			if got != want {
				t.Errorf("selectPMAModel = %q, want top tier %q", got, want)
			}
		})
	}
}

func TestDeriveInferenceBackend_IncludesSelectedModel(t *testing.T) {
	nodeID := uuid.New()
	report := nodepayloads.CapabilityReport{
		Inference: &nodepayloads.InferenceInfo{Supported: true},
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{
				{VRAMMB: 20464, Features: map[string]interface{}{"rocm_version": "6.0"}},
			},
		},
	}
	raw, _ := json.Marshal(report)
	db := testutil.NewMockDB()
	db.CapabilityHistory = append(db.CapabilityHistory, &testutil.NodeCapabilitySnapshot{
		NodeID:         nodeID,
		CapabilityJSON: string(raw),
		CreatedAt:      time.Now(),
	})
	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	backend := h.deriveInferenceBackend(t.Context(), nodeID)
	if backend == nil {
		t.Fatal("expected non-nil inference backend config")
	}
	if backend.SelectedModel == "" {
		t.Error("expected SelectedModel to be populated in inference backend config")
	}
	if len(backend.ModelsToEnsure) < 1 {
		t.Error("expected ModelsToEnsure to list at least the selected model")
	}
}

func TestBuildModelsToEnsure_DedupesAndOrdersDefaultFirst(t *testing.T) {
	got := buildModelsToEnsure(pmaModelDefault)
	if len(got) != 1 || got[0] != pmaModelDefault {
		t.Fatalf("buildModelsToEnsure(same as default) = %v, want [%s]", got, pmaModelDefault)
	}
	got = buildModelsToEnsure("qwen3:8b")
	if len(got) != 2 || got[0] != pmaModelDefault || got[1] != "qwen3:8b" {
		t.Fatalf("buildModelsToEnsure(tier) = %v, want [%s qwen3:8b]", got, pmaModelDefault)
	}
}

func TestBuildManagedServicesDesiredState_BackendEnvPropagated(t *testing.T) {
	t.Setenv("PMA_SERVICE_ID", "pma-main")
	t.Setenv("PMA_IMAGE", "pma:latest")
	t.Setenv("PMA_NODE_SLUG", "test-node")
	db := testutil.NewMockDB()
	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	node := &models.Node{
		NodeBase: models.NodeBase{NodeSlug: "test-node"},
		ID:       uuid.New(),
	}
	backendEnv := map[string]string{"OLLAMA_NUM_CTX": "32768"}
	u := uuid.New()
	rs := uuid.New()
	putMockSessionBindingByLineage(t, db, u, rs, poolServiceID(0), []byte("tok"))
	result := h.buildManagedServicesDesiredState(t.Context(), node, "http://10.0.0.1:12090", backendEnv)
	if result == nil || len(result.Services) != 2 {
		t.Fatalf("expected warm pool (assigned + idle), got %#v", result)
	}
	var svc *nodepayloads.ConfigManagedService
	for i := range result.Services {
		if result.Services[i].Env != nil && result.Services[i].Env["CYNODE_SESSION_ID"] != "" {
			svc = &result.Services[i]
			break
		}
	}
	if svc == nil {
		t.Fatalf("expected one assigned pool slot with session env, got %#v", result.Services)
	}
	if svc.ServiceID != poolServiceID(0) {
		t.Fatalf("service_id %q want %s after migrate", svc.ServiceID, poolServiceID(0))
	}
	if svc.Inference == nil {
		t.Fatal("expected inference config on managed service")
	}
	if svc.Inference.BackendEnv == nil {
		t.Fatal("expected BackendEnv to be set on managed service inference config")
	}
	if v := svc.Inference.BackendEnv["OLLAMA_NUM_CTX"]; v != "32768" {
		t.Errorf("expected OLLAMA_NUM_CTX=32768, got %q", v)
	}
}

func TestBuildManagedServicesDesiredState_ExtraDistinctServiceIDs(t *testing.T) {
	t.Setenv("PMA_SERVICE_ID", "pma-main")
	t.Setenv("PMA_IMAGE", "pma:latest")
	t.Setenv("PMA_NODE_SLUG", "node-a")
	db := testutil.NewMockDB()
	u := uuid.New()
	rs := uuid.New()
	putMockSessionBindingByLineage(t, db, u, rs, "pma-binding-b", []byte("tok2"))
	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	node := &models.Node{
		NodeBase: models.NodeBase{NodeSlug: "node-a"},
		ID:       uuid.New(),
	}
	got := h.buildManagedServicesDesiredState(t.Context(), node, "http://10.0.0.1:12090", nil)
	if got == nil || len(got.Services) != 2 {
		t.Fatalf("want warm pool (assigned + idle), got %v", got)
	}
	if got.Services[0].ServiceID != poolServiceID(0) || got.Services[0].Env == nil {
		t.Fatalf("unexpected first slot: %#v", got.Services[0])
	}
	if got.Services[0].Env["CYNODE_SESSION_ID"] != rs.String() || got.Services[0].Env["CYNODE_USER_ID"] != u.String() {
		t.Fatalf("unexpected session env: %#v", got.Services[0].Env)
	}
	if got.Services[1].ServiceID != poolServiceID(1) || got.Services[1].Env != nil {
		t.Fatalf("expected idle second slot, got %#v", got.Services[1])
	}
}

func TestBuildManagedServicesDesiredState_NilDBOrNodeReturnsNil(t *testing.T) {
	t.Setenv("PMA_SERVICE_ID", "pma-main")
	t.Setenv("PMA_IMAGE", "pma:latest")
	h := NewNodeHandler(nil, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	n := &models.Node{NodeBase: models.NodeBase{NodeSlug: "n"}, ID: uuid.New()}
	if got := h.buildManagedServicesDesiredState(t.Context(), n, "http://10.0.0.1:12090", nil); got != nil {
		t.Fatalf("nil db: want nil, got %#v", got)
	}
	db := testutil.NewMockDB()
	h2 := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	if got := h2.buildManagedServicesDesiredState(t.Context(), nil, "http://10.0.0.1:12090", nil); got != nil {
		t.Fatalf("nil node: want nil, got %#v", got)
	}
}

func TestBuildManagedServicesDesiredState_SkippedWhenPMAHostIsOtherNode(t *testing.T) {
	t.Setenv("PMA_SERVICE_ID", "pma-main")
	t.Setenv("PMA_IMAGE", "pma:latest")
	t.Setenv("PMA_HOST_NODE_SLUG", "other-node")
	db := testutil.NewMockDB()
	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	node := &models.Node{
		NodeBase: models.NodeBase{NodeSlug: "this-node"},
		ID:       uuid.New(),
	}
	if got := h.buildManagedServicesDesiredState(t.Context(), node, "http://10.0.0.1:12090", nil); got != nil {
		t.Fatalf("expected nil when PMA host is another node, got %#v", got)
	}
}

func TestBuildManagedServicesDesiredState_ListAllBindingsErrorReturnsNoPMA(t *testing.T) {
	t.Setenv("PMA_SERVICE_ID", "pma-main")
	t.Setenv("PMA_IMAGE", "pma:latest")
	t.Setenv("PMA_NODE_SLUG", "node-b")
	db := testutil.NewMockDB()
	db.ForceError = errors.New("list bindings failed")
	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
	node := &models.Node{
		NodeBase: models.NodeBase{NodeSlug: "node-b"},
		ID:       uuid.New(),
	}
	got := h.buildManagedServicesDesiredState(t.Context(), node, "http://10.0.0.1:12090", nil)
	if got == nil || len(got.Services) != 0 {
		t.Fatalf("expected empty PMA desired state when listing bindings fails: %#v", got)
	}
}
