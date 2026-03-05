package nodemanager

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

const (
	pathNodesRegister   = "/v1/nodes/register"
	pathNodesConfig     = "/v1/nodes/config"
	pathNodesCapability = "/v1/nodes/capability"
)

func TestBuildCapability_SetsInference(t *testing.T) {
	t.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
	ctx := context.Background()
	cfg := &Config{
		NodeSlug:                 "test-slug",
		NodeName:                 "Test",
		AdvertisedWorkerAPIURL:   "http://worker:12090",
	}
	report := buildCapability(ctx, cfg)
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
}

func TestBuildCapability_NilConfig(t *testing.T) {
	report := buildCapability(context.Background(), nil)
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
	report := buildCapability(ctx, cfg)
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
	report := buildCapability(ctx, cfg)
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
	report := buildCapability(ctx, cfg)
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

// registerOKHandler returns a handler that responds 201 with a BootstrapResponse for the given baseURL.
func registerOKHandler(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(nodepayloads.BootstrapResponse{
			Version:  1,
			IssuedAt: time.Now().UTC().Format(time.RFC3339),
			Orchestrator: nodepayloads.BootstrapOrchestrator{
				Endpoints: nodepayloads.BootstrapEndpoints{
					NodeReportURL: baseURL + pathNodesCapability,
					NodeConfigURL: baseURL + pathNodesConfig,
				},
			},
			Auth: nodepayloads.BootstrapAuth{NodeJWT: "jwt", ExpiresAt: "2026-01-01T00:00:00Z"},
		})
	}
}

// configHandler returns a handler that responds to GET with a minimal node config and POST with 204.
// Includes InferenceBackend.Enabled so tests that pass StartOllama will invoke it (when no existing service).
func configHandler(nodeSlug string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
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
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// mockOrchWithConfig returns a test server that handles register, config GET/POST, and capability.
func mockOrchWithConfig(t *testing.T) *httptest.Server {
	t.Helper()
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	server := httptest.NewServer(handler)
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
		if r.URL.Path == pathNodesRegister {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(nodepayloads.BootstrapResponse{
				Version:  1,
				IssuedAt: time.Now().UTC().Format(time.RFC3339),
				Orchestrator: nodepayloads.BootstrapOrchestrator{
					Endpoints: nodepayloads.BootstrapEndpoints{
						NodeReportURL: "http://127.0.0.1:1",
						NodeConfigURL: srv.URL + pathNodesConfig,
					},
				},
				Auth: nodepayloads.BootstrapAuth{NodeJWT: "jwt", ExpiresAt: "2026-01-01T00:00:00Z"},
			})
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
		if r.URL.Path == pathNodesRegister {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(nodepayloads.BootstrapResponse{
				Version:  1,
				IssuedAt: time.Now().UTC().Format(time.RFC3339),
				Orchestrator: nodepayloads.BootstrapOrchestrator{
					Endpoints: nodepayloads.BootstrapEndpoints{
						NodeReportURL: srv.URL + "/cap",
						NodeConfigURL: srv.URL + pathNodesConfig,
					},
				},
				Auth: nodepayloads.BootstrapAuth{NodeJWT: "j", ExpiresAt: "2026-01-01T00:00:00Z"},
			})
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
		if r.URL.Path == pathNodesConfig && r.Method == "GET" {
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
		if r.URL.Path == pathNodesConfig && r.Method == "POST" {
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
