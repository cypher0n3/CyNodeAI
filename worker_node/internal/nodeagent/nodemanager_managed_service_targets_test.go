package nodeagent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

// TestBuildManagedServiceTargetsFromConfig_PMAUsesUDS asserts REQ-WORKER-0174 / REQ-WORKER-0270:
// the PMA target base_url MUST be a http+unix:// URL pointing at the per-service UDS socket,
// not a TCP URL. PMA_BASE_URL (TCP) must not be used when a stateDir is available.
// TestMultiPMA_BuildManagedServiceTargets_TwoPMADistinctUDS asserts REQ-WORKER-0176: two PMA service_ids
// produce distinct http+unix base URLs (independent proxy UDS per instance).
func TestMultiPMA_BuildManagedServiceTargets_TwoPMADistinctUDS(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-a", ServiceType: "pma"},
				{ServiceID: "pma-b", ServiceType: "pma"},
			},
		},
	}
	targets := buildManagedServiceTargetsFromConfig(cfg)
	if len(targets) != 2 {
		t.Fatalf("want 2 targets, got %d: %+v", len(targets), targets)
	}
	a := targets["pma-a"]["base_url"]
	b := targets["pma-b"]["base_url"]
	if a == b || !strings.Contains(a, "pma-a") || !strings.Contains(b, "pma-b") {
		t.Fatalf("expected distinct UDS URLs, a=%q b=%q", a, b)
	}
}

func TestBuildManagedServiceTargetsFromConfig_PMAUsesUDS(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-main", ServiceType: "pma"},
			},
		},
	}
	targets := buildManagedServiceTargetsFromConfig(cfg)
	got, ok := targets["pma-main"]
	if !ok {
		t.Fatalf("expected pma-main target, got %+v", targets)
	}
	baseURL := got["base_url"]
	if !strings.HasPrefix(baseURL, "http+unix://") {
		t.Errorf("PMA target base_url must be http+unix://, got %q", baseURL)
	}
	if strings.Contains(baseURL, "8090") || strings.Contains(baseURL, "127.0.0.1") {
		t.Errorf("PMA target base_url must not contain TCP address, got %q", baseURL)
	}
	if !strings.Contains(baseURL, "pma-main") {
		t.Errorf("PMA target base_url must reference service ID path, got %q", baseURL)
	}
}

// TestBuildManagedServiceTargetsFromConfig_PMASocketPathUsesStateDir asserts the UDS socket
// path is rooted under WORKER_API_STATE_DIR / ManagedAgentProxySocketBaseDir.
func TestBuildManagedServiceTargetsFromConfig_PMASocketPathUsesStateDir(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-main", ServiceType: "pma"},
			},
		},
	}
	targets := buildManagedServiceTargetsFromConfig(cfg)
	got := targets["pma-main"]["base_url"]
	expectedDirFragment := filepath.Join(stateDir, ManagedAgentProxySocketBaseDir, "pma-main")
	if !strings.Contains(got, url.PathEscape(expectedDirFragment)) && !strings.Contains(got, expectedDirFragment) {
		t.Errorf("PMA UDS URL must reference state_dir path %q, got %q", expectedDirFragment, got)
	}
}

func TestApplyWorkerProxyConfigEnv_SetsManagedServiceTargetsEnv(t *testing.T) {
	_ = os.Unsetenv("WORKER_MANAGED_SERVICE_TARGETS_JSON")
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-main", ServiceType: "pma"},
			},
		},
	}
	applyWorkerProxyConfigEnv(cfg)
	if got := os.Getenv("WORKER_MANAGED_SERVICE_TARGETS_JSON"); got == "" || !strings.Contains(got, "pma-main") {
		t.Fatalf("expected WORKER_MANAGED_SERVICE_TARGETS_JSON to be populated, got %q", got)
	}
}

func TestApplyWorkerProxyConfigEnv_RedactsAgentTokensFromEnvPayload(t *testing.T) {
	_ = os.Unsetenv("WORKER_NODE_CONFIG_JSON")
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "pma-main",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "super-secret-agent-token",
						AgentTokenRef: &nodepayloads.ConfigManagedServiceAgentTokenRef{
							Kind: agentTokenRefKindOrchestratorEndpoint,
							URL:  "https://example.invalid/token",
						},
					},
				},
			},
		},
	}
	applyWorkerProxyConfigEnv(cfg)
	raw := os.Getenv("WORKER_NODE_CONFIG_JSON")
	if raw == "" {
		t.Fatal("expected WORKER_NODE_CONFIG_JSON to be set")
	}
	if strings.Contains(raw, "super-secret-agent-token") {
		t.Fatalf("WORKER_NODE_CONFIG_JSON must not include agent_token material: %s", raw)
	}
	if strings.Contains(raw, agentTokenRefKindOrchestratorEndpoint) || strings.Contains(raw, "example.invalid/token") {
		t.Fatalf("WORKER_NODE_CONFIG_JSON must not include agent_token_ref material: %s", raw)
	}
}

func TestEffectiveStateDirPrecedence(t *testing.T) {
	t.Setenv("WORKER_API_STATE_DIR", "/tmp/worker-api-state")
	t.Setenv("CYNODE_STATE_DIR", "/tmp/cynode-state")
	if got := effectiveStateDir(); got != "/tmp/worker-api-state" {
		t.Fatalf("expected WORKER_API_STATE_DIR precedence, got %q", got)
	}
	t.Setenv("WORKER_API_STATE_DIR", "")
	if got := effectiveStateDir(); got != "/tmp/cynode-state" {
		t.Fatalf("expected CYNODE_STATE_DIR fallback, got %q", got)
	}
	t.Setenv("CYNODE_STATE_DIR", "")
	if got := effectiveStateDir(); got != "/var/lib/cynode/state" {
		t.Fatalf("expected default when both unset, got %q", got)
	}
}

func TestSanitizeNodeConfigForWorkerEnv_NilAndNoManagedServices(t *testing.T) {
	if sanitizeNodeConfigForWorkerEnv(nil) != nil {
		t.Fatal("expected nil input to return nil")
	}
	cfg := &nodepayloads.NodeConfigurationPayload{Version: 1, NodeSlug: "x"}
	out := sanitizeNodeConfigForWorkerEnv(cfg)
	if out == nil || out.NodeSlug != "x" {
		t.Fatalf("unexpected output for non-managed config: %+v", out)
	}
}

func TestResolveManagedServiceToken_NoOrchestrator(t *testing.T) {
	empty := nodepayloads.ConfigManagedService{}
	token, expires, hasToken, err := resolveManagedServiceToken(context.Background(), &Config{NodeSlug: "x", HTTPTimeout: time.Second}, &empty)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasToken || token != "" || expires != "" {
		t.Fatalf("expected no token, got hasToken=%v token=%q expires=%q", hasToken, token, expires)
	}
}

func TestSyncManagedServiceAgentTokens_NoManagedServices(t *testing.T) {
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", testSecureStoreMasterKeyB64())
	cfg := &Config{NodeSlug: "x", HTTPTimeout: time.Second}
	initial := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "svc-a",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "tok-a",
					},
				},
			},
		},
	}
	if err := syncManagedServiceAgentTokens(context.Background(), cfg, initial, nil); err != nil {
		t.Fatalf("seed sync failed: %v", err)
	}
	emptyManagedServices := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{Services: []nodepayloads.ConfigManagedService{}},
	}
	if err := syncManagedServiceAgentTokens(context.Background(), cfg, emptyManagedServices, nil); err != nil {
		t.Fatalf("expected explicit empty managed_services sync to prune tokens, got error: %v", err)
	}
	secretPath := filepath.Join(os.Getenv("WORKER_API_STATE_DIR"), "secrets", "agent_tokens", "svc-a.json.enc")
	if _, err := os.Stat(secretPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected token file to be removed when managed_services is explicitly empty, stat err: %v", err)
	}
}

func TestSanitizeNodeConfigForWorkerEnv_ManagedServicesRedaction(t *testing.T) {
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID: "svc-a",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "tok-a",
						AgentTokenRef: &nodepayloads.ConfigManagedServiceAgentTokenRef{
							Kind: agentTokenRefKindOrchestratorEndpoint,
							URL:  "https://example.invalid",
						},
					},
				},
			},
		},
	}
	out := sanitizeNodeConfigForWorkerEnv(cfg)
	if out.ManagedServices == nil || len(out.ManagedServices.Services) != 1 {
		t.Fatalf("unexpected sanitized output: %+v", out)
	}
	orch := out.ManagedServices.Services[0].Orchestrator
	if orch == nil {
		t.Fatal("expected orchestrator block")
	}
	if orch.AgentToken != "" || orch.AgentTokenRef != nil {
		t.Fatalf("expected token fields redacted, got %+v", orch)
	}
	if cfg.ManagedServices.Services[0].Orchestrator.AgentToken == "" {
		t.Fatal("expected original config to stay unchanged")
	}
}

func TestApplyConfigAndStartServices_FailsWhenSecureStoreSyncFails(t *testing.T) {
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", "")
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	cfg := &Config{NodeSlug: "x", HTTPTimeout: time.Second}
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "svc-a",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "tok-a",
					},
				},
			},
		},
	}
	err := applyConfigAndStartServices(context.Background(), nil, cfg, &BootstrapData{}, nodeConfig, nil)
	if err == nil {
		t.Fatal("expected applyConfigAndStartServices to fail when secure store sync fails")
	}
}

func TestResolveAgentTokenRef_RequestFailure(t *testing.T) {
	cfg := &Config{NodeSlug: "node-a", HTTPTimeout: 5 * time.Millisecond}
	svc := nodepayloads.ConfigManagedService{
		ServiceID:   "svc-ref",
		ServiceType: "pma",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
			AgentTokenRef: &nodepayloads.ConfigManagedServiceAgentTokenRef{
				Kind: agentTokenRefKindOrchestratorEndpoint,
				URL:  "http://127.0.0.1:1",
			},
		},
	}
	if _, _, _, err := resolveAgentTokenRef(context.Background(), cfg, &svc); err == nil {
		t.Fatal("expected resolveAgentTokenRef to fail for unreachable endpoint")
	}
}

func TestMaybeStartOllama_NoOpPaths(t *testing.T) {
	cfg := &nodepayloads.NodeConfigurationPayload{}
	if err := maybeStartOllama(context.Background(), nil, cfg, nil, false); err != nil {
		t.Fatalf("expected nil opts no-op, got %v", err)
	}
	called := false
	opts := &RunOptions{
		StartOllama: func(_, _ string, _ map[string]string) error {
			called = true
			return nil
		},
	}
	if err := maybeStartOllama(context.Background(), nil, cfg, opts, true); err != nil {
		t.Fatalf("expected existing-service no-op, got %v", err)
	}
	if called {
		t.Fatal("StartOllama must not be called when existing service is true")
	}
}

func TestMaybeStartOllama_DerivesImageFromVariantWhenImageAbsent(t *testing.T) {
	// When orchestrator sends variant but omits image, node MUST derive image from variant
	// per worker_node_payloads and REQ-WORKER-0253. Ollama has :rocm tag; cuda uses default image.
	t.Setenv("OLLAMA_IMAGE", "ollama/ollama:rocm")
	defer func() { _ = os.Unsetenv("OLLAMA_IMAGE") }()
	var capturedImage, capturedVariant string
	opts := &RunOptions{
		StartOllama: func(image, variant string, _ map[string]string) error {
			capturedImage = image
			capturedVariant = variant
			return nil
		},
	}
	cfg := &nodepayloads.NodeConfigurationPayload{
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{
			Enabled: true,
			Image:   "",
			Variant: "cuda",
		},
	}
	if err := maybeStartOllama(context.Background(), nil, cfg, opts, false); err != nil {
		t.Fatalf("maybeStartOllama: %v", err)
	}
	if capturedImage != "ollama/ollama" {
		t.Errorf("image = %q, want ollama/ollama (cuda uses default; derived from variant, not OLLAMA_IMAGE)", capturedImage)
	}
	if capturedVariant != "cuda" {
		t.Errorf("variant = %q, want cuda", capturedVariant)
	}
	// rocm variant uses explicit :rocm tag
	cfg.InferenceBackend.Variant = "rocm"
	if err := maybeStartOllama(context.Background(), nil, cfg, opts, false); err != nil {
		t.Fatalf("maybeStartOllama rocm: %v", err)
	}
	if capturedImage != "ollama/ollama:rocm" {
		t.Errorf("image = %q, want ollama/ollama:rocm for rocm variant", capturedImage)
	}
}

func TestMaybeStartManagedServices_NoOpPaths(t *testing.T) {
	if err := maybeStartManagedServices(context.Background(), nil, nil, nil); err != nil {
		t.Fatalf("expected nil opts no-op, got %v", err)
	}
	var gotLen int
	called := false
	opts := &RunOptions{
		StartManagedServices: func(_ context.Context, svcs []nodepayloads.ConfigManagedService) error {
			called = true
			gotLen = len(svcs)
			return nil
		},
	}
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: nil,
		},
	}
	if err := maybeStartManagedServices(context.Background(), nil, cfg, opts); err != nil {
		t.Fatalf("expected empty services reconcile, got %v", err)
	}
	if !called {
		t.Fatal("StartManagedServices must be called for empty service list (teardown reconcile)")
	}
	if gotLen != 0 {
		t.Fatalf("expected empty slice, got len=%d", gotLen)
	}
}

func TestApplyConfigAndStartServices_NilNodeConfigFailsAck(t *testing.T) {
	cfg := &Config{HTTPTimeout: time.Second}
	bootstrap := &BootstrapData{NodeJWT: "jwt", NodeConfigURL: "http://127.0.0.1:1"}
	err := applyConfigAndStartServices(context.Background(), nil, cfg, bootstrap, nil, nil)
	if err == nil {
		t.Fatal("expected applyConfigAndStartServices to fail when node config is nil")
	}
}

func TestResolveManagedServiceToken_AgentTokenRefPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"agent_token": "ref-token"})
	}))
	defer server.Close()
	svc := nodepayloads.ConfigManagedService{
		ServiceID:   "svc-ref",
		ServiceType: "pma",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
			AgentTokenRef: &nodepayloads.ConfigManagedServiceAgentTokenRef{
				Kind: agentTokenRefKindOrchestratorEndpoint,
				URL:  server.URL,
			},
		},
	}
	token, _, hasToken, err := resolveManagedServiceToken(context.Background(), &Config{NodeSlug: "x", HTTPTimeout: time.Second}, &svc)
	if err != nil {
		t.Fatalf("resolveManagedServiceToken failed: %v", err)
	}
	if !hasToken || token != "ref-token" {
		t.Fatalf("unexpected token resolution result: hasToken=%v token=%q", hasToken, token)
	}
}

func TestFetchConfig_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	bootstrap := &BootstrapData{NodeJWT: "jwt", NodeConfigURL: srv.URL + pathNodesConfig}
	cfg := &Config{HTTPTimeout: 5 * time.Second}
	ctx := context.Background()

	_, err := FetchConfig(ctx, cfg, bootstrap)
	if err == nil {
		t.Fatal("FetchConfig should fail on 204")
	}
}

func TestSendConfigAck_NilConfig(t *testing.T) {
	bootstrap := &BootstrapData{NodeJWT: "jwt", NodeConfigURL: "http://x"}
	cfg := &Config{HTTPTimeout: 5 * time.Second}
	ctx := context.Background()

	err := SendConfigAck(ctx, cfg, bootstrap, nil, "applied")
	if err == nil {
		t.Fatal("SendConfigAck should fail when node config is nil")
	}
}

func TestFetchConfig_BadJSON(t *testing.T) {
	srv := newTestServerJSONFixedBody(t, "not json")
	bootstrap := &BootstrapData{NodeJWT: "jwt", NodeConfigURL: srv.URL}
	cfg := &Config{HTTPTimeout: 5 * time.Second}
	ctx := context.Background()
	_, err := FetchConfig(ctx, cfg, bootstrap)
	if err == nil {
		t.Fatal("FetchConfig should fail on invalid JSON")
	}
}

func TestSendConfigAck_Non204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	bootstrap := &BootstrapData{NodeJWT: "jwt", NodeConfigURL: srv.URL}
	cfg := &Config{HTTPTimeout: 5 * time.Second}
	ctx := context.Background()
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		Version: 1, ConfigVersion: "1", NodeSlug: "x",
	}
	err := SendConfigAck(ctx, cfg, bootstrap, nodeConfig, "applied")
	if err == nil {
		t.Fatal("SendConfigAck should fail on 400")
	}
}

func TestReconcileManagedServices_NilOpts(t *testing.T) {
	// Should not panic or call anything when opts is nil.
	reconcileManagedServices(context.Background(), nil, &nodepayloads.NodeConfigurationPayload{}, nil)
}

func TestReconcileManagedServices_NilNodeConfig(t *testing.T) {
	called := false
	opts := &RunOptions{
		StartManagedServices: func(context.Context, []nodepayloads.ConfigManagedService) error {
			called = true
			return nil
		},
	}
	reconcileManagedServices(context.Background(), nil, nil, opts)
	if called {
		t.Error("StartManagedServices should not be called with nil nodeConfig")
	}
}

func TestReconcileManagedServices_EmptyServices(t *testing.T) {
	var gotLen int
	called := false
	opts := &RunOptions{
		StartManagedServices: func(_ context.Context, svcs []nodepayloads.ConfigManagedService) error {
			called = true
			gotLen = len(svcs)
			return nil
		},
	}
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{Services: nil},
	}
	reconcileManagedServices(context.Background(), nil, nodeConfig, opts)
	if !called {
		t.Error("StartManagedServices should be called with empty slice to reconcile managed teardown")
	}
	if gotLen != 0 {
		t.Errorf("expected empty slice, got len=%d", gotLen)
	}
}

func TestReconcileManagedServices_CallsStartManagedServices(t *testing.T) {
	called := false
	opts := &RunOptions{
		StartManagedServices: func(_ context.Context, svcs []nodepayloads.ConfigManagedService) error {
			called = true
			return nil
		},
	}
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{{ServiceID: "pma-main", ServiceType: "pma"}},
		},
	}
	reconcileManagedServices(context.Background(), slog.Default(), nodeConfig, opts)
	if !called {
		t.Error("StartManagedServices should be called when services are present")
	}
}

func TestReconcileManagedServices_LogsOnError(t *testing.T) {
	opts := &RunOptions{
		StartManagedServices: func(context.Context, []nodepayloads.ConfigManagedService) error {
			return errors.New("container gone")
		},
	}
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{{ServiceID: "pma-main", ServiceType: "pma"}},
		},
	}
	// Should not panic; error is logged only.
	reconcileManagedServices(context.Background(), slog.Default(), nodeConfig, opts)
}

func TestInferenceBackendPullSpecChanged(t *testing.T) {
	ib := func(selected string, ensure ...string) *nodepayloads.NodeConfigurationPayload {
		return &nodepayloads.NodeConfigurationPayload{
			InferenceBackend: &nodepayloads.ConfigInferenceBackend{
				SelectedModel:  selected,
				ModelsToEnsure: append([]string(nil), ensure...),
			},
		}
	}
	if inferenceBackendPullSpecChanged(ib("a", "x"), ib("a", "x")) {
		t.Error("same pull spec should not report changed")
	}
	if !inferenceBackendPullSpecChanged(ib("a"), ib("b")) {
		t.Error("different selected_model should report changed")
	}
	if !inferenceBackendPullSpecChanged(ib("a", "m1"), ib("a", "m2")) {
		t.Error("different models_to_ensure should report changed")
	}
	if !inferenceBackendPullSpecChanged(nil, ib("a")) {
		t.Error("nil old with inference should report changed")
	}
}

func TestManagedServicesConfigChanged(t *testing.T) {
	svc := func(model string) *nodepayloads.NodeConfigurationPayload {
		return &nodepayloads.NodeConfigurationPayload{
			ManagedServices: &nodepayloads.ConfigManagedServices{
				Services: []nodepayloads.ConfigManagedService{
					{
						ServiceID:   "pma-main",
						ServiceType: "pma",
						Inference:   &nodepayloads.ConfigManagedServiceInference{DefaultModel: model},
					},
				},
			},
		}
	}
	if managedServicesConfigChanged(svc(testOllamaModelQwen38), svc(testOllamaModelQwen38)) {
		t.Error("same config should not report changed")
	}
	if !managedServicesConfigChanged(svc("qwen3.5:0.8b"), svc(testOllamaModelQwen38)) {
		t.Error("different model should report changed")
	}
	if !managedServicesConfigChanged(nil, svc(testOllamaModelQwen38)) {
		t.Error("nil old should report changed")
	}
	if !managedServicesConfigChanged(svc(testOllamaModelQwen38), nil) {
		t.Error("nil updated should report changed")
	}
	if managedServicesConfigChanged(nil, nil) {
		t.Error("both nil should not report changed")
	}
}

func TestQueryOllamaTags_ReturnsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[{"name":"`+testOllamaModelQwen38+`"},{"name":"qwen3.5:0.8b"}]}`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	names, ok := queryOllamaTags(context.Background(), u.Port())
	if !ok {
		t.Fatal("queryOllamaTags() ok = false, want true")
	}
	if len(names) != 2 || names[0] != testOllamaModelQwen38 || names[1] != "qwen3.5:0.8b" {
		t.Errorf("queryOllamaTags() = %v, want [%s qwen3.5:0.8b]", names, testOllamaModelQwen38)
	}
}

func TestQueryOllamaTags_ServerDown(t *testing.T) {
	names, ok := queryOllamaTags(context.Background(), "19999")
	if ok || names != nil {
		t.Errorf("queryOllamaTags() on closed port: ok=%v names=%v, want false/nil", ok, names)
	}
}

func TestDetectAvailableModels_SkipsWhenTestFlag(t *testing.T) {
	_ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	defer func() { _ = os.Unsetenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE") }()
	if got := detectAvailableModels(context.Background()); got != nil {
		t.Errorf("detectAvailableModels() = %v, want nil when test flag set", got)
	}
}

func TestBuildCapability_AvailableModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[{"name":"`+testOllamaModelQwen38+`"}]}`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port := u.Port()
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-rt")
	const scriptBody = "#!/bin/sh\n[ \"$1\" = ps ] && echo cynodeai-ollama\n"
	if err := os.WriteFile(script, []byte(scriptBody), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	_ = os.Unsetenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE")
	defer func() { _ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1") }()
	_ = os.Setenv("CONTAINER_RUNTIME", script)
	defer func() { _ = os.Unsetenv("CONTAINER_RUNTIME") }()
	_ = os.Setenv("OLLAMA_CONTAINER_NAME", "cynodeai-ollama")
	defer func() { _ = os.Unsetenv("OLLAMA_CONTAINER_NAME") }()
	_ = os.Setenv("OLLAMA_PORT", port)
	defer func() { _ = os.Unsetenv("OLLAMA_PORT") }()
	cfg := &Config{NodeSlug: "x", NodeName: "y"}
	report := buildCapability(context.Background(), cfg, nil)
	if report.Inference == nil {
		t.Fatal("Inference is nil")
	}
	if len(report.Inference.AvailableModels) == 0 || report.Inference.AvailableModels[0] != testOllamaModelQwen38 {
		t.Errorf("AvailableModels = %v, want [%s]", report.Inference.AvailableModels, testOllamaModelQwen38)
	}
}

func TestMaybePullModels_NilOpts(t *testing.T) {
	maybePullModels(context.Background(), nil, &nodepayloads.NodeConfigurationPayload{
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{SelectedModel: testOllamaModelQwen38},
	}, nil)
}

func TestMaybePullModels_NilPullModels(t *testing.T) {
	maybePullModels(context.Background(), nil, &nodepayloads.NodeConfigurationPayload{
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{SelectedModel: testOllamaModelQwen38},
	}, &RunOptions{})
}

func TestMaybePullModels_NoSelectedModel(t *testing.T) {
	called := false
	opts := &RunOptions{PullModels: func(context.Context, []string) error { called = true; return nil }}
	maybePullModels(context.Background(), nil, &nodepayloads.NodeConfigurationPayload{
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{},
	}, opts)
	if called {
		t.Error("PullModels should not be called when SelectedModel is empty")
	}
}

func TestMaybePullModels_AlreadyAvailable(t *testing.T) {
	_ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	defer func() { _ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1") }()
	called := false
	opts := &RunOptions{PullModels: func(context.Context, []string) error { called = true; return nil }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[{"name":"`+testOllamaModelQwen38+`"}]}`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	_ = os.Unsetenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE")
	defer func() { _ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1") }()
	_ = os.Setenv("OLLAMA_PORT", u.Port())
	defer func() { _ = os.Unsetenv("OLLAMA_PORT") }()
	maybePullModels(context.Background(), nil, &nodepayloads.NodeConfigurationPayload{
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{SelectedModel: testOllamaModelQwen38},
	}, opts)
	time.Sleep(20 * time.Millisecond)
	if called {
		t.Error("PullModels should not be called when selected model is already available")
	}
}

func TestMaybePullModels_MissingModel(t *testing.T) {
	_ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	defer func() { _ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1") }()
	pulled := make(chan []string, 1)
	opts := &RunOptions{PullModels: func(_ context.Context, m []string) error { pulled <- m; return nil }}
	maybePullModels(context.Background(), nil, &nodepayloads.NodeConfigurationPayload{
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{SelectedModel: "qwen3.5:9b"},
	}, opts)
	select {
	case got := <-pulled:
		if len(got) != 1 || got[0] != "qwen3.5:9b" {
			t.Errorf("expected pull of [qwen3.5:9b], got %v", got)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("PullModels was not called within timeout")
	}
}

func TestMaybePullModels_ModelsToEnsurePullsOnlyMissing(t *testing.T) {
	_ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	defer func() { _ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1") }()
	pulled := make(chan []string, 1)
	opts := &RunOptions{PullModels: func(_ context.Context, m []string) error { pulled <- m; return nil }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"models":[{"name":"qwen3.5:0.8b"}]}`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	_ = os.Unsetenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE")
	defer func() { _ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1") }()
	_ = os.Setenv("OLLAMA_PORT", u.Port())
	defer func() { _ = os.Unsetenv("OLLAMA_PORT") }()
	maybePullModels(context.Background(), nil, &nodepayloads.NodeConfigurationPayload{
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{
			SelectedModel:  testOllamaModelQwen38,
			ModelsToEnsure: []string{"qwen3.5:0.8b", testOllamaModelQwen38},
		},
	}, opts)
	select {
	case got := <-pulled:
		if len(got) != 1 || got[0] != testOllamaModelQwen38 {
			t.Errorf("expected pull of missing model only [%s], got %v", testOllamaModelQwen38, got)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("PullModels was not called within timeout")
	}
}
