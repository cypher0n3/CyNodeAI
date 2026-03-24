package nodeagent

import (
	"context"
	"encoding/base64"
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
	out := buildManagedServicesStatus(nodeConfig)
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

// TestBuildManagedServicesStatus_WorkerProxyURL asserts PMA gets "ready" with worker proxy URL when NODE_ADVERTISED_WORKER_API_URL is set.
func TestBuildManagedServicesStatus_WorkerProxyURL(t *testing.T) {
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	t.Setenv("NODE_ADVERTISED_WORKER_API_URL", "http://worker:12090")
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
	out := buildManagedServicesStatus(nodeConfig)
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
	out := buildManagedServicesStatus(nodeConfig)
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
	out := buildManagedServicesStatus(nodeConfig)
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

func TestRunRegisterErrorStatusAndBadJSON(t *testing.T) {
	for name, tc := range map[string]struct {
		handler http.HandlerFunc
		errMsg  string
	}{
		"403": {
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"detail":"forbidden"}`))
			},
			"Run should fail on 403",
		},
		"bad JSON": {
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("not json"))
			},
			"Run should fail on invalid JSON",
		},
	} {
		t.Run(name, func(t *testing.T) {
			runWithServerExpectError(t, tc.handler, tc.errMsg)
		})
	}
}

func TestRunRegisterInvalidBootstrap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathReadyz {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path != pathNodesRegister {
			if r.URL.Path == "/v1/nodes/config" {
				configHandler("x")(w, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(nodepayloads.BootstrapResponse{
			Version: 1,
			Auth:    nodepayloads.BootstrapAuth{NodeJWT: ""},
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: "http://x",
					NodeConfigURL: "http://x",
				},
			},
		})
	}))
	defer server.Close()

	cfg := &Config{
		OrchestratorURL: server.URL,
		NodeSlug:        "x",
		NodeName:        "x",
		RegistrationPSK: "psk",
		HTTPTimeout:     5 * time.Second,
	}
	ctx := context.Background()
	err := Run(ctx, nil, cfg)
	if err == nil {
		t.Error("Run should fail when bootstrap missing node_jwt")
	}
}

func TestRunReportCapabilitiesErrorBranch(t *testing.T) {
	reportCount := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathNodesRegister {
			registerOKHandler(srv.URL)(w, r)
			return
		}
		if r.URL.Path == pathNodesConfig {
			configHandler("x")(w, r)
			return
		}
		if r.URL.Path == pathNodesCapability {
			reportCount++
			if reportCount > 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	cfg := &Config{
		OrchestratorURL:          srv.URL,
		NodeSlug:                 "x",
		NodeName:                 "x",
		RegistrationPSK:          "psk",
		CapabilityReportInterval: 5 * time.Millisecond,
		HTTPTimeout:              5 * time.Second,
	}
	t.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = Run(ctx, nil, cfg)
	if reportCount < 2 {
		t.Errorf("expected at least 2 capability reports, got %d", reportCount)
	}
}

func TestRunReportCapabilitiesConnectionFails(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathReadyz {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == pathNodesRegister {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(bootstrapResponsePayload("http://127.0.0.1:1", srv.URL+pathNodesConfig, "jwt", "2026-01-01T00:00:00Z"))
			return
		}
		if r.URL.Path == pathNodesConfig {
			configHandler("x")(w, r)
		}
	}))
	defer srv.Close()

	cfg := &Config{
		OrchestratorURL:          srv.URL,
		NodeSlug:                 "x",
		NodeName:                 "x",
		RegistrationPSK:          "psk",
		CapabilityReportInterval: 10 * time.Millisecond,
		HTTPTimeout:              5 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := Run(ctx, nil, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunContextCanceledAfterRegister(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathReadyz {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == pathNodesRegister {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(bootstrapResponsePayload(srv.URL+"/cap", srv.URL+pathNodesConfig, "j", "2026-01-01T00:00:00Z"))
			return
		}
		if r.URL.Path == pathNodesConfig {
			configHandler("x")(w, r)
		}
	}))
	defer srv.Close()

	cfg := &Config{
		OrchestratorURL:          srv.URL,
		NodeSlug:                 "x",
		NodeName:                 "x",
		RegistrationPSK:          "psk",
		CapabilityReportInterval: time.Hour,
		HTTPTimeout:              5 * time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, nil, cfg)
	}()
	time.Sleep(300 * time.Millisecond)
	cancel()
	err := <-errCh
	if err != nil {
		t.Errorf("Run after cancel should return nil: %v", err)
	}
}

func TestFetchConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathNodesConfig && r.Method == http.MethodGet {
			configHandler("fetch-test")(w, r)
		}
	}))
	defer srv.Close()

	bootstrap := &BootstrapData{
		NodeJWT:       "jwt",
		NodeConfigURL: srv.URL + pathNodesConfig,
	}
	cfg := &Config{HTTPTimeout: 5 * time.Second}
	ctx := context.Background()

	payload, err := FetchConfig(ctx, cfg, bootstrap)
	if err != nil {
		t.Fatalf("FetchConfig: %v", err)
	}
	if payload.NodeSlug != "fetch-test" || payload.ConfigVersion != "1" {
		t.Errorf("payload: %+v", payload)
	}
	if payload.WorkerAPI == nil || payload.WorkerAPI.OrchestratorBearerToken != "test-bearer" {
		t.Errorf("worker_api token missing")
	}
}

func TestSendConfigAck(t *testing.T) {
	ackReceived := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathNodesConfig && r.Method == http.MethodPost {
			ackReceived = true
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	bootstrap := &BootstrapData{
		NodeJWT:       "jwt",
		NodeConfigURL: srv.URL + pathNodesConfig,
	}
	cfg := &Config{HTTPTimeout: 5 * time.Second}
	ctx := context.Background()
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		Version:       1,
		ConfigVersion: "1",
		NodeSlug:      "ack-test",
	}

	err := SendConfigAck(ctx, cfg, bootstrap, nodeConfig, "applied")
	if err != nil {
		t.Fatalf("SendConfigAck: %v", err)
	}
	if !ackReceived {
		t.Error("ack was not received")
	}
}

func TestRunWithOptions_OllamaFailFast(t *testing.T) {
	t.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	srv := mockOrchWithConfig(t)
	defer srv.Close()

	cfg := &Config{
		OrchestratorURL: srv.URL,
		NodeSlug:        "x",
		NodeName:        "x",
		RegistrationPSK: "psk",
		HTTPTimeout:     5 * time.Second,
	}
	opts := &RunOptions{
		StartOllama: func(_, _ string, _ map[string]string) error { return errors.New("ollama start failed") },
	}
	ctx := context.Background()

	err := RunWithOptions(ctx, nil, cfg, opts)
	if err == nil {
		t.Fatal("RunWithOptions should fail when StartOllama returns error")
	}
	if !strings.Contains(err.Error(), "start inference") || !strings.Contains(err.Error(), "ollama start failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunWithOptions_StartWorkerAPICalled(t *testing.T) {
	t.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	srv := mockOrchWithConfig(t)
	defer srv.Close()

	var tokenReceived string
	cfg := &Config{
		OrchestratorURL:          srv.URL,
		NodeSlug:                 "x",
		NodeName:                 "x",
		RegistrationPSK:          "psk",
		CapabilityReportInterval: 20 * time.Millisecond,
		HTTPTimeout:              5 * time.Second,
	}
	opts := &RunOptions{
		StartWorkerAPI: func(tok string) error {
			tokenReceived = tok
			return nil
		},
		StartOllama: func(_, _ string, _ map[string]string) error { return nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	err := RunWithOptions(ctx, logger, cfg, opts)
	if err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	if tokenReceived != "test-bearer" {
		t.Errorf("StartWorkerAPI should receive token from config, got %q", tokenReceived)
	}
	if got := os.Getenv("WORKER_NODE_CONFIG_JSON"); got == "" {
		t.Error("expected WORKER_NODE_CONFIG_JSON to be set from node config")
	}
}

// mockOrchWithManagedPayload returns a test server that serves readyz, register, config (GET returns payload, POST 204), capability 204.
func mockOrchWithManagedPayload(payload *nodepayloads.NodeConfigurationPayload) *httptest.Server {
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
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(*payload)
				return
			}
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusNoContent)
			}
			return
		}
		if r.URL.Path == pathNodesCapability {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	baseURL = srv.URL
	return srv
}

func TestRunWithOptions_StartManagedServicesCalled(t *testing.T) {
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", testSecureStoreMasterKeyB64())
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	t.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	payloadWithManaged := nodepayloads.NodeConfigurationPayload{
		Version: 1, ConfigVersion: "1", NodeSlug: "x",
		WorkerAPI:        &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "test-bearer"},
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{Enabled: true, Image: "ollama/ollama"},
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{{
				ServiceID: "pma-main", ServiceType: "pma", Image: "ghcr.io/example/pma:latest",
				Args: []string{"--role=project_manager"}, RestartPolicy: "always",
				Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
					MCPGatewayProxyURL:    "http://127.0.0.1:12090/v1/worker/internal/orchestrator/mcp:call",
					ReadyCallbackProxyURL: "http://127.0.0.1:12090/v1/worker/internal/orchestrator/agent:ready",
					AgentToken:            "agent-tok",
				},
			}},
		},
	}
	srv := mockOrchWithManagedPayload(&payloadWithManaged)
	defer srv.Close()

	var servicesReceived []nodepayloads.ConfigManagedService
	cfg := &Config{
		OrchestratorURL: srv.URL, NodeSlug: "x", NodeName: "x", RegistrationPSK: "psk",
		CapabilityReportInterval: 20 * time.Millisecond, HTTPTimeout: 5 * time.Second,
	}
	opts := &RunOptions{
		StartWorkerAPI: func(string) error { return nil },
		StartOllama:    func(_, _ string, _ map[string]string) error { return nil },
		StartManagedServices: func(svcs []nodepayloads.ConfigManagedService) error {
			servicesReceived = append([]nodepayloads.ConfigManagedService(nil), svcs...)
			return nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if err := RunWithOptions(ctx, logger, cfg, opts); err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	if len(servicesReceived) != 1 {
		t.Fatalf("StartManagedServices should be called with 1 service, got %d", len(servicesReceived))
	}
	svc := servicesReceived[0]
	if svc.ServiceID != "pma-main" || svc.ServiceType != serviceTypePMA || svc.Image != "ghcr.io/example/pma:latest" {
		t.Errorf("unexpected service: service_id=%q service_type=%q image=%q", svc.ServiceID, svc.ServiceType, svc.Image)
	}
	if svc.Orchestrator == nil || svc.Orchestrator.AgentToken != "agent-tok" {
		t.Errorf("orchestrator block and agent_token should be passed through: %+v", svc.Orchestrator)
	}
}

func TestRunWithOptions_ManagedServicesFailFast(t *testing.T) {
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", testSecureStoreMasterKeyB64())
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	t.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	payloadWithManaged := nodepayloads.NodeConfigurationPayload{
		Version: 1, ConfigVersion: "1", NodeSlug: "x",
		WorkerAPI:        &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "tok"},
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{Enabled: true, Image: "ollama/ollama"},
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest"},
			},
		},
	}
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
		if r.URL.Path == pathNodesConfig && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(payloadWithManaged)
			return
		}
		if r.URL.Path == pathNodesConfig && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == pathNodesCapability {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	baseURL = srv.URL

	cfg := &Config{OrchestratorURL: srv.URL, NodeSlug: "x", NodeName: "x", RegistrationPSK: "psk", HTTPTimeout: 5 * time.Second}
	opts := &RunOptions{
		StartWorkerAPI:       func(string) error { return nil },
		StartOllama:          func(_, _ string, _ map[string]string) error { return nil },
		StartManagedServices: func([]nodepayloads.ConfigManagedService) error { return errors.New("managed service start failed") },
	}
	ctx := context.Background()
	err := RunWithOptions(ctx, nil, cfg, opts)
	if err == nil {
		t.Fatal("RunWithOptions should fail when StartManagedServices returns error")
	}
	if !strings.Contains(err.Error(), "start managed services") || !strings.Contains(err.Error(), "managed service start failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSyncManagedServiceAgentTokens_WriteRotateDelete(t *testing.T) {
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", testSecureStoreMasterKeyB64())
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	cfg := &Config{NodeSlug: "node-a", HTTPTimeout: 2 * time.Second}
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
	if err := syncManagedServiceAgentTokens(context.Background(), cfg, nodeConfig, nil); err != nil {
		t.Fatalf("initial sync failed: %v", err)
	}
	updatedConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "svc-a",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken:          "tok-a-rotated",
						AgentTokenExpiresAt: time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
					},
				},
				{
					ServiceID:   "svc-b",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "tok-b",
					},
				},
			},
		},
	}
	if err := syncManagedServiceAgentTokens(context.Background(), cfg, updatedConfig, nil); err != nil {
		t.Fatalf("rotation sync failed: %v", err)
	}
	removalConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "svc-b",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "tok-b",
					},
				},
			},
		},
	}
	if err := syncManagedServiceAgentTokens(context.Background(), cfg, removalConfig, nil); err != nil {
		t.Fatalf("removal sync failed: %v", err)
	}
	secretPathA := filepath.Join(os.Getenv("WORKER_API_STATE_DIR"), "secrets", "agent_tokens", "svc-a.json.enc")
	if _, err := os.Stat(secretPathA); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected svc-a token file to be deleted, stat err: %v", err)
	}
	secretPathB := filepath.Join(os.Getenv("WORKER_API_STATE_DIR"), "secrets", "agent_tokens", "svc-b.json.enc")
	if _, err := os.Stat(secretPathB); err != nil {
		t.Fatalf("expected svc-b token file to exist: %v", err)
	}
}

func TestResolveAgentTokenRef_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"agent_token":            "ref-token",
			"agent_token_expires_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		})
	}))
	defer server.Close()
	cfg := &Config{NodeSlug: "node-a", HTTPTimeout: 2 * time.Second}
	svc := nodepayloads.ConfigManagedService{
		ServiceID:   "svc-ref",
		ServiceType: "pma",
		Role:        "project_manager",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
			AgentTokenRef: &nodepayloads.ConfigManagedServiceAgentTokenRef{
				Kind: agentTokenRefKindOrchestratorEndpoint,
				URL:  server.URL,
			},
		},
	}
	token, expires, hasToken, err := resolveAgentTokenRef(context.Background(), cfg, &svc)
	if err != nil {
		t.Fatalf("resolveAgentTokenRef failed: %v", err)
	}
	if !hasToken || token != "ref-token" || expires == "" {
		t.Fatalf("unexpected resolved token tuple: hasToken=%v token=%q expires=%q", hasToken, token, expires)
	}
}

func TestResolveManagedServiceToken_InvalidDirectExpiry(t *testing.T) {
	svc := nodepayloads.ConfigManagedService{
		ServiceID:   "svc-a",
		ServiceType: "pma",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
			AgentToken:          "tok",
			AgentTokenExpiresAt: "not-a-time",
		},
	}
	_, _, _, err := resolveManagedServiceToken(context.Background(), &Config{NodeSlug: "node-a", HTTPTimeout: time.Second}, &svc)
	if err == nil {
		t.Fatal("expected invalid agent_token_expires_at to fail")
	}
}

func TestResolveAgentTokenRef_Failures(t *testing.T) {
	cfg := &Config{NodeSlug: "node-a", HTTPTimeout: time.Second}
	baseSvc := nodepayloads.ConfigManagedService{
		ServiceID:   "svc-a",
		ServiceType: "pma",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
			AgentTokenRef: &nodepayloads.ConfigManagedServiceAgentTokenRef{},
		},
	}
	for _, tt := range []struct {
		name string
		kind string
		url  string
	}{
		{"unsupported kind", "unsupported", "https://example.invalid/token"},
		{"missing url", agentTokenRefKindOrchestratorEndpoint, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := baseSvc
			svc.Orchestrator.AgentTokenRef.Kind = tt.kind
			svc.Orchestrator.AgentTokenRef.URL = tt.url
			_, _, _, err := resolveAgentTokenRef(context.Background(), cfg, &svc)
			if err == nil {
				t.Fatal("expected failure")
			}
		})
	}
	t.Run("non-2xx", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()
		svc := baseSvc
		svc.Orchestrator.AgentTokenRef.Kind = agentTokenRefKindOrchestratorEndpoint
		svc.Orchestrator.AgentTokenRef.URL = server.URL
		_, _, _, err := resolveAgentTokenRef(context.Background(), cfg, &svc)
		if err == nil {
			t.Fatal("expected non-2xx failure")
		}
	})
	t.Run("invalid json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not-json"))
		}))
		defer server.Close()
		svc := baseSvc
		svc.Orchestrator.AgentTokenRef.Kind = agentTokenRefKindOrchestratorEndpoint
		svc.Orchestrator.AgentTokenRef.URL = server.URL
		_, _, _, err := resolveAgentTokenRef(context.Background(), cfg, &svc)
		if err == nil {
			t.Fatal("expected invalid json failure")
		}
	})
	t.Run("missing token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"agent_token": ""})
		}))
		defer server.Close()
		svc := baseSvc
		svc.Orchestrator.AgentTokenRef.Kind = agentTokenRefKindOrchestratorEndpoint
		svc.Orchestrator.AgentTokenRef.URL = server.URL
		_, _, _, err := resolveAgentTokenRef(context.Background(), cfg, &svc)
		if err == nil {
			t.Fatal("expected missing token failure")
		}
	})
	t.Run("invalid expiry", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"agent_token":            "tok",
				"agent_token_expires_at": "invalid-expiry",
			})
		}))
		defer server.Close()
		svc := baseSvc
		svc.Orchestrator.AgentTokenRef.Kind = agentTokenRefKindOrchestratorEndpoint
		svc.Orchestrator.AgentTokenRef.URL = server.URL
		_, _, _, err := resolveAgentTokenRef(context.Background(), cfg, &svc)
		if err == nil {
			t.Fatal("expected invalid expiry failure")
		}
	})
}

func TestSyncManagedServiceAgentTokens_MissingMasterKeyFails(t *testing.T) {
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", "")
	cfg := &Config{NodeSlug: "node-a", HTTPTimeout: time.Second}
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
	if err := syncManagedServiceAgentTokens(context.Background(), cfg, nodeConfig, nil); err == nil {
		t.Fatal("expected syncManagedServiceAgentTokens to fail when master key is missing")
	}
}

func TestApplyWorkerProxyConfigEnv_SetsOrchestratorBaseURL(t *testing.T) {
	_ = os.Unsetenv("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL")
	cfg := &nodepayloads.NodeConfigurationPayload{
		Version: 1,
		Orchestrator: nodepayloads.ConfigOrchestrator{
			BaseURL: "http://orchestrator.example:12082",
		},
	}
	applyWorkerProxyConfigEnv(cfg)
	if got := os.Getenv("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL"); got != "http://orchestrator.example:12082" {
		t.Fatalf("unexpected ORCHESTRATOR_INTERNAL_PROXY_BASE_URL: %q", got)
	}
}

func TestBuildManagedServiceTargetsFromConfig(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-main", ServiceType: "pma"},
				{ServiceID: "other", ServiceType: "tooling_proxy"},
			},
		},
	}
	targets := buildManagedServiceTargetsFromConfig(cfg)
	got, ok := targets["pma-main"]
	if !ok {
		t.Fatalf("expected pma-main target, got %+v", targets)
	}
	if got["service_type"] != "pma" {
		t.Fatalf("unexpected service_type: %+v", got)
	}
	if !strings.HasPrefix(got["base_url"], "http+unix://") {
		t.Fatalf("expected http+unix:// base_url, got %q", got["base_url"])
	}
	if _, ok := targets["other"]; ok {
		t.Fatalf("unexpected non-pma target in mapping: %+v", targets["other"])
	}
}

// TestBuildManagedServiceTargetsFromConfig_PMAUsesUDS asserts REQ-WORKER-0174 / REQ-WORKER-0270:
// the PMA target base_url MUST be a http+unix:// URL pointing at the per-service UDS socket,
// not a TCP URL. PMA_BASE_URL (TCP) must not be used when a stateDir is available.
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
	called := false
	opts := &RunOptions{
		StartManagedServices: func([]nodepayloads.ConfigManagedService) error {
			called = true
			return nil
		},
	}
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: nil,
		},
	}
	if err := maybeStartManagedServices(context.Background(), nil, cfg, opts); err != nil {
		t.Fatalf("expected empty services no-op, got %v", err)
	}
	if called {
		t.Fatal("StartManagedServices must not be called for empty service list")
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
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
		StartManagedServices: func(_ []nodepayloads.ConfigManagedService) error {
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
	called := false
	opts := &RunOptions{
		StartManagedServices: func(_ []nodepayloads.ConfigManagedService) error {
			called = true
			return nil
		},
	}
	nodeConfig := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{Services: nil},
	}
	reconcileManagedServices(context.Background(), nil, nodeConfig, opts)
	if called {
		t.Error("StartManagedServices should not be called with empty services")
	}
}

func TestReconcileManagedServices_CallsStartManagedServices(t *testing.T) {
	called := false
	opts := &RunOptions{
		StartManagedServices: func(svcs []nodepayloads.ConfigManagedService) error {
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
		StartManagedServices: func(_ []nodepayloads.ConfigManagedService) error {
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
	opts := &RunOptions{PullModels: func(_ []string) error { called = true; return nil }}
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
	opts := &RunOptions{PullModels: func(_ []string) error { called = true; return nil }}
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
	opts := &RunOptions{PullModels: func(m []string) error { pulled <- m; return nil }}
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
	opts := &RunOptions{PullModels: func(m []string) error { pulled <- m; return nil }}
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
