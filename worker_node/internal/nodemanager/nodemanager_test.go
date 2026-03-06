package nodemanager

import (
	"encoding/base64"
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

func TestMain(m *testing.M) {
	// Skip container runtime startup check in tests (no podman/docker or image in test env).
	_ = os.Setenv("NODE_MANAGER_SKIP_CONTAINER_CHECK", "1")
	os.Exit(m.Run())
}

const (
	pathNodesRegister   = "/v1/nodes/register"
	pathNodesConfig     = "/v1/nodes/config"
	pathNodesCapability = "/v1/nodes/capability"
	pathReadyz          = "/readyz"
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

func TestBuildCapability_SetsInference(t *testing.T) {
	t.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	ctx := context.Background()
	cfg := &Config{
		NodeSlug:                 "test-slug",
		NodeName:                 "Test",
		AdvertisedWorkerAPIURL:   "http://worker:12090",
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
				Version:       1,
				ConfigVersion: "1",
				IssuedAt:      time.Now().UTC().Format(time.RFC3339),
				NodeSlug:      nodeSlug,
				WorkerAPI:     &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "test-bearer"},
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

func TestRunContextCancelledAfterRegister(t *testing.T) {
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
		StartOllama: func(_, _ string) error { return errors.New("ollama start failed") },
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
		StartOllama: func(_, _ string) error { return nil },
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
		WorkerAPI: &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "test-bearer"},
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
		StartOllama:    func(_, _ string) error { return nil },
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
		WorkerAPI:         &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "tok"},
		InferenceBackend:  &nodepayloads.ConfigInferenceBackend{Enabled: true, Image: "ollama/ollama"},
		ManagedServices:   &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest"},
			},
		},
	}
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathReadyz { w.WriteHeader(http.StatusOK); return }
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
		if r.URL.Path == pathNodesConfig && r.Method == http.MethodPost { w.WriteHeader(http.StatusNoContent); return }
		if r.URL.Path == pathNodesCapability { w.WriteHeader(http.StatusNoContent) }
	}))
	defer srv.Close()
	baseURL = srv.URL

	cfg := &Config{OrchestratorURL: srv.URL, NodeSlug: "x", NodeName: "x", RegistrationPSK: "psk", HTTPTimeout: 5 * time.Second}
	opts := &RunOptions{
		StartWorkerAPI:       func(string) error { return nil },
		StartOllama:         func(_, _ string) error { return nil },
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
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-main", ServiceType: "pma"},
				{ServiceID: "other", ServiceType: "tooling_proxy"},
			},
		},
	}
	_ = os.Setenv("PMA_BASE_URL", "http://127.0.0.1:8090")
	defer func() { _ = os.Unsetenv("PMA_BASE_URL") }()
	targets := buildManagedServiceTargetsFromConfig(cfg)
	got, ok := targets["pma-main"]
	if !ok {
		t.Fatalf("expected pma-main target, got %+v", targets)
	}
	if got["base_url"] != "http://127.0.0.1:8090" || got["service_type"] != "pma" {
		t.Fatalf("unexpected pma target values: %+v", got)
	}
	if _, ok := targets["other"]; ok {
		t.Fatalf("unexpected non-pma target in mapping: %+v", targets["other"])
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
		StartOllama: func(_, _ string) error {
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
