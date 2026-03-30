package nodeagent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

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
		StartManagedServices: func(_ context.Context, svcs []nodepayloads.ConfigManagedService) error {
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
		StartManagedServices: func(context.Context, []nodepayloads.ConfigManagedService) error { return errors.New("managed service start failed") },
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
