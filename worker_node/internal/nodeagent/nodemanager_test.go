package nodeagent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
)

const testOllamaModelQwen38 = "qwen3:8b"

func TestMain(m *testing.M) {
	// Skip container runtime startup check in tests (no podman/docker or image in test env).
	_ = os.Setenv("NODE_MANAGER_SKIP_CONTAINER_CHECK", "1")
	// Skip GPU detection in tests (rocm-smi/nvidia-smi may not be present or may be slow).
	_ = os.Setenv("NODE_MANAGER_TEST_NO_GPU_DETECT", "1")
	os.Exit(m.Run())
}

const (
	pathNodesRegister   = "/v1/nodes/register"
	pathNodesConfig     = "/v1/nodes/config"
	pathNodesCapability = "/v1/nodes/capability"
	pathReadyz          = "/readyz"
	stateReady          = "ready"
)

func testSecureStoreMasterKeyB64() string {
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func TestRunStartupChecks_SkipWhenEnvSet(t *testing.T) {
	t.Setenv("NODE_MANAGER_SKIP_CONTAINER_CHECK", "1")
	t.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	ctx := context.Background()
	cfg := &Config{OrchestratorURL: "http://x", NodeSlug: "x", RegistrationPSK: "psk"}
	err := runStartupChecks(ctx, nil, cfg)
	if err != nil {
		t.Fatalf("runStartupChecks with skip set: %v", err)
	}
}

func TestCheckContainerRuntime_FailsWhenRuntimeUnavailable(t *testing.T) {
	t.Setenv("NODE_MANAGER_SKIP_CONTAINER_CHECK", "")
	t.Setenv("CONTAINER_RUNTIME", "nonexistent-runtime-binary-xyz")
	ctx := context.Background()
	cfg := &Config{}
	err := checkContainerRuntime(ctx, nil, cfg)
	if err == nil {
		t.Fatal("checkContainerRuntime should fail when runtime binary is unavailable")
	}
	if !strings.Contains(err.Error(), "startup check") && !strings.Contains(err.Error(), "nonexistent-runtime-binary-xyz") {
		t.Errorf("error should mention startup check or runtime: %v", err)
	}
}

func TestInferenceBackendPullSpecKey(t *testing.T) {
	if inferenceBackendPullSpecKey(nil) != "" {
		t.Fatal("nil cfg")
	}
	cfg := &nodepayloads.NodeConfigurationPayload{
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{
			SelectedModel:  "m1",
			ModelsToEnsure: []string{"a", "b"},
		},
	}
	if inferenceBackendPullSpecKey(cfg) == "" {
		t.Fatal("expected non-empty key")
	}
}

func TestReconcileAgentTokenStore_writesDesired(t *testing.T) {
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", testSecureStoreMasterKeyB64())
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	st, _, err := securestore.Open(effectiveStateDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	desired := map[string]resolvedAgentToken{
		"desired-svc": {token: "tok", expiresAt: ""},
	}
	if err := reconcileAgentTokenStore(st, desired, nil); err != nil {
		t.Fatalf("reconcileAgentTokenStore: %v", err)
	}
}

func TestReconcileAgentTokenStore_removesStale(t *testing.T) {
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", testSecureStoreMasterKeyB64())
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	st, _, err := securestore.Open(effectiveStateDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.PutAgentToken("orphan-svc", "tok", ""); err != nil {
		t.Fatalf("PutAgentToken: %v", err)
	}
	if err := reconcileAgentTokenStore(st, map[string]resolvedAgentToken{}, []string{"orphan-svc"}); err != nil {
		t.Fatalf("reconcileAgentTokenStore: %v", err)
	}
}

func TestComputeDesiredAgentTokens_skipsEmptyServiceID(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{}
	nc := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "   "},
				{
					ServiceID: "ok",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "direct-token",
					},
				},
			},
		},
	}
	got, err := computeDesiredAgentTokens(ctx, cfg, nc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries", len(got))
	}
	if _, ok := got["ok"]; !ok {
		t.Fatal("missing ok service")
	}
}

func TestModelsToPullFromConfig_dedupAndSelectedFallback(t *testing.T) {
	if modelsToPullFromConfig(nil) != nil {
		t.Fatal("nil config")
	}
	cfg := &nodepayloads.NodeConfigurationPayload{
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{
			ModelsToEnsure: []string{"a", " A ", "", "a"},
		},
	}
	got := modelsToPullFromConfig(cfg)
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %#v", got)
	}
	cfg2 := &nodepayloads.NodeConfigurationPayload{
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{
			SelectedModel: "  pick  ",
		},
	}
	got2 := modelsToPullFromConfig(cfg2)
	if len(got2) != 1 || got2[0] != "pick" {
		t.Fatalf("got %#v", got2)
	}
}

func TestModelsMissingFromAvailable(t *testing.T) {
	miss := modelsMissingFromAvailable([]string{" A ", "b", ""}, map[string]bool{"a": true})
	if len(miss) != 1 || miss[0] != "b" {
		t.Fatalf("got %#v", miss)
	}
	if len(modelsMissingFromAvailable(nil, map[string]bool{"x": true})) != 0 {
		t.Fatal("expected empty for nil models slice")
	}
}

func TestBuildCapability_SetsInference(t *testing.T) {
	t.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	ctx := context.Background()
	cfg := &Config{
		NodeSlug:               "test-slug",
		NodeName:               "Test",
		AdvertisedWorkerAPIURL: "http://worker:12090",
	}
	report := buildCapability(ctx, cfg, nil)
	if report.Inference == nil {
		t.Fatal("buildCapability should set Inference when cfg is non-nil")
	}
	if !report.Inference.Supported || report.Inference.ExistingService {
		t.Errorf("with test env: expected supported=true existing_service=false, got %+v", report.Inference)
	}
	if report.WorkerAPI == nil || report.WorkerAPI.BaseURL != "http://worker:12090" {
		t.Errorf("expected worker_api.base_url from config, got %+v", report.WorkerAPI)
	}
	if report.ManagedServices == nil || !report.ManagedServices.Supported {
		t.Errorf("expected managed_services.supported=true, got %+v", report.ManagedServices)
	}
	hasIdentityBound := false
	for _, feature := range report.ManagedServices.Features {
		if feature == "agent_orchestrator_proxy_identity_bound" {
			hasIdentityBound = true
			break
		}
	}
	if !hasIdentityBound {
		t.Errorf("expected managed_services.features to include agent_orchestrator_proxy_identity_bound, got %+v", report.ManagedServices.Features)
	}
}

func TestBuildCapability_NilConfig(t *testing.T) {
	report := buildCapability(context.Background(), nil, nil)
	if report.Inference != nil {
		t.Error("buildCapability with nil cfg should not set Inference")
	}
	if report.Version != 1 || report.Node.NodeSlug != "" {
		t.Errorf("nil cfg: expected minimal report, got %+v", report)
	}
}

// TestBuildCapability_ManagedServicesStatus_HttpUnixURLsWhenAuto asserts that when config sets
// proxy URLs to "auto", managed_services_status reports binding=per_service_uds and http+unix:// URLs
// (Phase 5 reconciliation plan: URL reporting when binding=per_service_uds).
func TestBuildCapability_ManagedServicesStatus_HttpUnixURLsWhenAuto(t *testing.T) {
	t.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	defer func() { _ = os.Unsetenv("WORKER_API_STATE_DIR") }()

	ctx := context.Background()
	cfg := &Config{NodeSlug: "test-slug", NodeName: "Test", AdvertisedWorkerAPIURL: "http://worker:12090"}
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "pma-main",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						MCPGatewayProxyURL:    "auto",
						ReadyCallbackProxyURL: "auto",
					},
				},
			},
		},
	}
	report := buildCapability(ctx, cfg, nodeConfig)
	if report.ManagedServicesStatus == nil || len(report.ManagedServicesStatus.Services) == 0 {
		t.Fatalf("expected managed_services_status with one service, got %+v", report.ManagedServicesStatus)
	}
	proxy := report.ManagedServicesStatus.Services[0].AgentToOrchestratorProxy
	if proxy == nil {
		t.Fatal("expected agent_to_orchestrator_proxy when orchestrator URLs are auto")
	}
	if proxy.Binding != "per_service_uds" {
		t.Errorf("expected binding=per_service_uds, got %q", proxy.Binding)
	}
	if proxy.MCPGatewayProxyURL == "" || !strings.HasPrefix(proxy.MCPGatewayProxyURL, "http+unix://") {
		t.Errorf("expected MCP gateway proxy URL with http+unix:// prefix, got %q", proxy.MCPGatewayProxyURL)
	}
	if !strings.Contains(proxy.MCPGatewayProxyURL, "/v1/worker/internal/orchestrator/mcp:call") {
		t.Errorf("expected MCP URL to contain mcp:call path, got %q", proxy.MCPGatewayProxyURL)
	}
	if proxy.ReadyCallbackProxyURL == "" || !strings.HasPrefix(proxy.ReadyCallbackProxyURL, "http+unix://") {
		t.Errorf("expected ready callback proxy URL with http+unix:// prefix, got %q", proxy.ReadyCallbackProxyURL)
	}
	if !strings.Contains(proxy.ReadyCallbackProxyURL, "/v1/worker/internal/orchestrator/agent:ready") {
		t.Errorf("expected ready URL to contain agent:ready path, got %q", proxy.ReadyCallbackProxyURL)
	}
	svc := report.ManagedServicesStatus.Services[0]
	if svc.State != stateReady {
		t.Errorf("expected PMA state %s, got %q", stateReady, svc.State)
	}
	wantEP := "http://worker:12090/v1/worker/managed-services/pma-main/proxy:http"
	if len(svc.Endpoints) != 1 || svc.Endpoints[0] != wantEP {
		t.Errorf("expected endpoint %q, got %v", wantEP, svc.Endpoints)
	}
}

// TestBuildManagedServicesStatus_PMAAdvertisedURLFallback asserts that when NODE_ADVERTISED_WORKER_API_URL
// is unset but PMA_ADVERTISED_URL is set, PMA service reports ready with that URL.
func TestBuildManagedServicesStatus_PMAAdvertisedURLFallback(t *testing.T) {
	t.Setenv("NODE_ADVERTISED_WORKER_API_URL", "")
	t.Setenv("PMA_ADVERTISED_URL", "http://pma.example:8090")
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	defer func() {
		_ = os.Unsetenv("NODE_ADVERTISED_WORKER_API_URL")
		_ = os.Unsetenv("PMA_ADVERTISED_URL")
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
	}()
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-main", ServiceType: "pma"},
			},
		},
	}
	out := buildManagedServicesStatus(nodeConfig, "")
	if out == nil || len(out.Services) != 1 {
		t.Fatalf("expected one service, got %+v", out)
	}
	s := out.Services[0]
	if s.State != stateReady {
		t.Errorf("expected state %s, got %q", stateReady, s.State)
	}
	if len(s.Endpoints) != 1 || s.Endpoints[0] != "http://pma.example:8090" {
		t.Errorf("expected PMA_ADVERTISED_URL endpoint, got %v", s.Endpoints)
	}
}

// TestBuildManagedServicesStatus_WorkerProxyURL asserts PMA gets "ready" with worker proxy URL from
// the advertised base argument (no env required; matches Config.AdvertisedWorkerAPIURL).
func TestBuildManagedServicesStatus_WorkerProxyURL(t *testing.T) {
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	t.Setenv("NODE_ADVERTISED_WORKER_API_URL", "")
	defer func() {
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		_ = os.Unsetenv("NODE_ADVERTISED_WORKER_API_URL")
	}()
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-1", ServiceType: "pma"},
			},
		},
	}
	out := buildManagedServicesStatus(nodeConfig, "http://worker:12090")
	if out == nil || len(out.Services) != 1 {
		t.Fatalf("expected one service, got %+v", out)
	}
	s := out.Services[0]
	if s.State != stateReady {
		t.Errorf("expected state %s, got %q", s.State, stateReady)
	}
	want := "http://worker:12090/v1/worker/managed-services/pma-1/proxy:http"
	if len(s.Endpoints) != 1 || s.Endpoints[0] != want {
		t.Errorf("expected worker proxy endpoint %q, got %v", want, s.Endpoints)
	}
}

// TestBuildManagedServicesStatus_NonPMAServiceStaysStarting asserts that non-PMA service types keep state "starting".
func TestBuildManagedServicesStatus_NonPMAServiceStaysStarting(t *testing.T) {
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	defer func() { _ = os.Unsetenv("WORKER_API_STATE_DIR") }()
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "other-svc", ServiceType: "tooling_proxy"},
			},
		},
	}
	out := buildManagedServicesStatus(nodeConfig, "")
	if out == nil || len(out.Services) != 1 {
		t.Fatalf("expected one service, got %+v", out)
	}
	if out.Services[0].State != "starting" {
		t.Errorf("expected state starting for non-PMA, got %q", out.Services[0].State)
	}
	if len(out.Services[0].Endpoints) != 0 {
		t.Errorf("expected no endpoints for non-PMA, got %v", out.Services[0].Endpoints)
	}
}

// TestBuildManagedServicesStatus_ExplicitProxyURLs asserts agent_to_orchestrator_proxy when URLs are not "auto".
func TestBuildManagedServicesStatus_ExplicitProxyURLs(t *testing.T) {
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	defer func() { _ = os.Unsetenv("WORKER_API_STATE_DIR") }()
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "pma-main",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						MCPGatewayProxyURL:    "http://worker:12090/mcp",
						ReadyCallbackProxyURL: "http://worker:12090/" + stateReady,
					},
				},
			},
		},
	}
	out := buildManagedServicesStatus(nodeConfig, "")
	if out == nil || len(out.Services) != 1 {
		t.Fatalf("expected one service, got %+v", out)
	}
	proxy := out.Services[0].AgentToOrchestratorProxy
	if proxy == nil {
		t.Fatal("expected agent_to_orchestrator_proxy")
	}
	if proxy.MCPGatewayProxyURL != "http://worker:12090/mcp" || proxy.ReadyCallbackProxyURL != "http://worker:12090/"+stateReady {
		t.Errorf("expected explicit URLs, got MCP=%q Ready=%q", proxy.MCPGatewayProxyURL, proxy.ReadyCallbackProxyURL)
	}
}

// TestBuildCapability_DetectExistingInferenceFails covers detectExistingInference when exec fails (e.g. no container runtime).
func TestBuildCapability_DetectExistingInferenceFails(t *testing.T) {
	_ = os.Unsetenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE")
	defer func() { _ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1") }()
	_ = os.Setenv("CONTAINER_RUNTIME", "/nonexistent/podman")
	defer func() { _ = os.Unsetenv("CONTAINER_RUNTIME") }()
	ctx := context.Background()
	cfg := &Config{NodeSlug: "x", NodeName: "y"}
	report := buildCapability(ctx, cfg, nil)
	// When detection fails we get (false, false); Inference should still be set.
	if report.Inference == nil {
		t.Fatal("buildCapability should set Inference when cfg non-nil")
	}
	if report.Inference.ExistingService || report.Inference.Running {
		t.Errorf("when exec fails expected existing_service=false running=false, got %+v", report.Inference)
	}
}

// TestBuildCapability_DetectExistingInferenceContainerExists uses a fake runtime script to cover "container exists" path.
func TestBuildCapability_DetectExistingInferenceContainerExists(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-runtime")
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
	ctx := context.Background()
	cfg := &Config{NodeSlug: "x", NodeName: "y"}
	report := buildCapability(ctx, cfg, nil)
	if report.Inference == nil {
		t.Fatal("buildCapability should set Inference")
	}
	// Script echoes container name so existingService=true; no real HTTP so running may be false.
	if !report.Inference.ExistingService {
		t.Errorf("expected existing_service=true when script outputs container name, got %+v", report.Inference)
	}
}

// TestBuildCapability_DetectExistingInferenceRunning covers the path when container is running and HTTP returns 200.
func TestBuildCapability_DetectExistingInferenceRunning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	port := u.Port()
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-runtime")
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
	ctx := context.Background()
	cfg := &Config{NodeSlug: "x", NodeName: "y"}
	report := buildCapability(ctx, cfg, nil)
	if report.Inference == nil {
		t.Fatal("buildCapability should set Inference")
	}
	if !report.Inference.ExistingService || !report.Inference.Running {
		t.Errorf("expected existing_service=true running=true when HTTP 200, got %+v", report.Inference)
	}
}

func TestLoadConfig(t *testing.T) {
	_ = os.Unsetenv("ORCHESTRATOR_URL")
	_ = os.Unsetenv("NODE_SLUG")
	_ = os.Unsetenv("CAPABILITY_REPORT_INTERVAL")
	_ = os.Unsetenv("HTTP_TIMEOUT")
	defer func() {
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("CAPABILITY_REPORT_INTERVAL")
		_ = os.Unsetenv("HTTP_TIMEOUT")
	}()

	cfg := LoadConfig()
	if cfg.OrchestratorURL != "http://localhost:12082" || cfg.NodeSlug != "node-01" {
		t.Errorf("defaults: %+v", cfg)
	}
	if cfg.CapabilityReportInterval != 60*time.Second {
		t.Errorf("default interval: %v", cfg.CapabilityReportInterval)
	}

	_ = os.Setenv("ORCHESTRATOR_URL", "http://x")
	_ = os.Setenv("NODE_SLUG", "s")
	_ = os.Setenv("HTTP_TIMEOUT", "2m")
	cfg2 := LoadConfig()
	if cfg2.OrchestratorURL != "http://x" || cfg2.NodeSlug != "s" {
		t.Errorf("env: %+v", cfg2)
	}
	if cfg2.HTTPTimeout != 2*time.Minute {
		t.Errorf("HTTP_TIMEOUT: %v", cfg2.HTTPTimeout)
	}

	_ = os.Setenv("HTTP_TIMEOUT", "invalid")
	cfg3 := LoadConfig()
	if cfg3.HTTPTimeout != 10*time.Second {
		t.Errorf("invalid HTTP_TIMEOUT should use default: %v", cfg3.HTTPTimeout)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		c    Config
		want bool
	}{
		{"valid", Config{OrchestratorURL: "http://x", NodeSlug: "s", RegistrationPSK: "psk"}, false},
		{"missing url", Config{OrchestratorURL: "", NodeSlug: "s", RegistrationPSK: "psk"}, true},
		{"missing slug", Config{OrchestratorURL: "http://x", NodeSlug: "", RegistrationPSK: "psk"}, true},
		{"missing psk", Config{OrchestratorURL: "http://x", NodeSlug: "s", RegistrationPSK: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if (err != nil) != tt.want {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.want)
			}
		})
	}
}

func TestValidateBootstrap(t *testing.T) {
	tests := []struct {
		name    string
		b       nodepayloads.BootstrapResponse
		wantErr bool
	}{
		{"valid", nodepayloads.BootstrapResponse{
			Version: 1,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "http://x", NodeConfigURL: "http://x",
				},
			},
		}, false},
		{"wrong version", nodepayloads.BootstrapResponse{
			Version: 2,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "http://x", NodeConfigURL: "http://x",
				},
			},
		}, true},
		{"missing node_jwt", nodepayloads.BootstrapResponse{
			Version: 1,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: ""},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "http://x", NodeConfigURL: "http://x",
				},
			},
		}, true},
		{"missing node_report_url", nodepayloads.BootstrapResponse{
			Version: 1,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "", NodeConfigURL: "http://x",
				},
			},
		}, true},
		{"missing node_config_url", nodepayloads.BootstrapResponse{
			Version: 1,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: "jwt"},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "http://x", NodeConfigURL: "",
				},
			},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBootstrap(&tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBootstrap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// bootstrapResponsePayload builds a BootstrapResponse for tests. reportURL and configURL are the endpoints; jwt and expiresAt set Auth.
func bootstrapResponsePayload(reportURL, configURL, jwt, expiresAt string) nodepayloads.BootstrapResponse {
	return nodepayloads.BootstrapResponse{
		Version:  1,
		IssuedAt: time.Now().UTC().Format(time.RFC3339),
		Orchestrator: nodepayloads.BootstrapOrchestrator{
			Endpoints: nodepayloads.BootstrapEndpoints{NodeReportURL: reportURL, NodeConfigURL: configURL},
		},
		Auth: nodepayloads.BootstrapAuth{NodeJWT: jwt, ExpiresAt: expiresAt},
	}
}

// registerOKHandler returns a handler that responds 201 with a BootstrapResponse for the given baseURL.
func registerOKHandler(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(bootstrapResponsePayload(baseURL+pathNodesCapability, baseURL+pathNodesConfig, "jwt", "2026-01-01T00:00:00Z"))
	}
}

// configHandler returns a handler that responds to GET with a minimal node config and POST with 204.
// Includes InferenceBackend.Enabled so tests that pass StartOllama will invoke it (when no existing service).
func configHandler(nodeSlug string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(nodepayloads.NodeConfigurationPayload{
				Version:          1,
				ConfigVersion:    "1",
				IssuedAt:         time.Now().UTC().Format(time.RFC3339),
				NodeSlug:         nodeSlug,
				WorkerAPI:        &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "test-bearer"},
				InferenceBackend: &nodepayloads.ConfigInferenceBackend{Enabled: true, Image: "ollama/ollama"},
			})
			return
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// mockOrchWithConfig returns a test server that handles readyz, register, config GET/POST, and capability.
func mockOrchWithConfig(t *testing.T) *httptest.Server {
	t.Helper()
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathReadyz {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == pathNodesRegister {
			registerOKHandler(baseURL)(w, r)
			return
		}
		if r.URL.Path == pathNodesConfig {
			configHandler("x")(w, r)
			return
		}
		if r.URL.Path == pathNodesCapability {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	baseURL = srv.URL
	return srv
}

func TestRun(t *testing.T) {
	reportCalled := false
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathReadyz {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == pathNodesRegister {
			registerOKHandler(srv.URL)(w, r)
			return
		}
		if r.URL.Path == pathNodesConfig {
			configHandler("run-test")(w, r)
			return
		}
		if r.URL.Path == pathNodesCapability {
			reportCalled = true
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	cfg := &Config{
		OrchestratorURL:          srv.URL,
		NodeSlug:                 "run-test",
		NodeName:                 "Run Test",
		RegistrationPSK:          "psk",
		CapabilityReportInterval: 10 * time.Millisecond,
		HTTPTimeout:              5 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := Run(ctx, nil, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !reportCalled {
		t.Error("Run should have called reportCapabilities")
	}
}

func TestRunValidateFails(t *testing.T) {
	cfg := &Config{OrchestratorURL: "http://x", NodeSlug: "", RegistrationPSK: "psk"}
	ctx := context.Background()
	err := Run(ctx, nil, cfg)
	if err == nil {
		t.Error("Run should fail when config invalid")
	}
}

func TestRunRegisterFails(t *testing.T) {
	t.Setenv("NODE_MANAGER_READINESS_TIMEOUT", "50ms")
	cfg := &Config{
		OrchestratorURL: "http://127.0.0.1:1",
		NodeSlug:        "x",
		NodeName:        "x",
		RegistrationPSK: "psk",
		HTTPTimeout:     1 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := Run(ctx, nil, cfg)
	if err == nil {
		t.Error("Run should fail when register fails")
	}
}

func runWithServerExpectError(t *testing.T, handler http.HandlerFunc, errMsg string) {
	t.Helper()
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathReadyz {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler(w, r)
	}
	server := httptest.NewServer(http.HandlerFunc(wrapped))
	defer server.Close()
	cfg := &Config{
		OrchestratorURL: server.URL,
		NodeSlug:        "x",
		NodeName:        "x",
		RegistrationPSK: "psk",
		HTTPTimeout:     5 * time.Second,
	}
	err := Run(context.Background(), nil, cfg)
	if err == nil {
		t.Error(errMsg)
	}
}
