package workerapiserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/worker_node/internal/executor"
)

func TestManagedAgentProxySocketPathEmbed(t *testing.T) {
	tests := []struct {
		stateDir  string
		serviceID string
		wantOK    bool
	}{
		{"/tmp", "svc-a", true},
		{"/tmp", "", false},
		{"/tmp", "a/b", false},
		{"/tmp", "a..b", false},
	}
	for _, tt := range tests {
		path, ok := managedAgentProxySocketPathEmbed(tt.stateDir, tt.serviceID)
		if ok != tt.wantOK {
			t.Errorf("managedAgentProxySocketPathEmbed(%q, %q) ok=%v want %v", tt.stateDir, tt.serviceID, ok, tt.wantOK)
		}
		if tt.wantOK && path == "" {
			t.Errorf("expected non-empty path")
		}
	}
}

func TestLoadProxyConfigFromEnv(t *testing.T) {
	dir := t.TempDir()
	logger := slog.Default()
	cfg, err := loadProxyConfigFromEnv(dir, logger)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ManagedServiceTargets == nil {
		t.Error("ManagedServiceTargets should be non-nil")
	}
	if cfg.InternalProxy.SocketByService == nil {
		t.Error("SocketByService should be non-nil")
	}
}

func TestLoadProxyConfigFromEnv_OrchestratorURLFallback(t *testing.T) {
	t.Setenv("ORCHESTRATOR_URL", "http://orch.local")
	defer func() { _ = os.Unsetenv("ORCHESTRATOR_URL") }()
	cfg, err := loadProxyConfigFromEnv(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InternalProxy.UpstreamBaseURL != "http://orch.local" {
		t.Errorf("UpstreamBaseURL = %q", cfg.InternalProxy.UpstreamBaseURL)
	}
}

func TestLoadProxyConfigFromEnv_SecureStoreUnavailable(t *testing.T) {
	dir := t.TempDir()
	// stateDir as a file (not a directory) so securestore.Open fails.
	stateDirAsFile := filepath.Join(dir, "notadir")
	if err := os.WriteFile(stateDirAsFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadProxyConfigFromEnv(stateDirAsFile, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InternalProxy.SecureStore != nil {
		t.Error("SecureStore should be nil when Open fails")
	}
}

func TestLoadProxyConfigFromEnv_WithNodeConfigJSON(t *testing.T) {
	dir := t.TempDir()
	logger := slog.Default()
	payload := nodepayloads.NodeConfigurationPayload{
		Version: 1,
		Orchestrator: nodepayloads.ConfigOrchestrator{
			BaseURL: "http://orch.example",
		},
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "pma-1",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "tok",
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(payload)
	t.Setenv("WORKER_NODE_CONFIG_JSON", string(raw))
	defer func() { _ = os.Unsetenv("WORKER_NODE_CONFIG_JSON") }()
	cfg, err := loadProxyConfigFromEnv(dir, logger)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InternalProxy.UpstreamBaseURL != "http://orch.example" {
		t.Errorf("UpstreamBaseURL = %q", cfg.InternalProxy.UpstreamBaseURL)
	}
	if len(cfg.InternalProxy.SocketByService) == 0 {
		t.Error("expected SocketByService to contain pma-1")
	}
}

func TestLoadManagedServiceTargetsFromEnvEmbed_Empty(t *testing.T) {
	_ = os.Unsetenv("WORKER_MANAGED_SERVICE_TARGETS_JSON")
	targets := loadManagedServiceTargetsFromEnvEmbed(slog.Default())
	if len(targets) != 0 {
		t.Errorf("expected empty, got %d", len(targets))
	}
}

func TestLoadManagedServiceTargetsFromEnvEmbed_SimpleMap(t *testing.T) {
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"svc1":"http://localhost:1"}`)
	defer func() { _ = os.Unsetenv("WORKER_MANAGED_SERVICE_TARGETS_JSON") }()
	targets := loadManagedServiceTargetsFromEnvEmbed(slog.Default())
	if len(targets) != 1 {
		t.Fatalf("len(targets)=%d", len(targets))
	}
	if targets["svc1"].BaseURL != "http://localhost:1" {
		t.Errorf("BaseURL = %q", targets["svc1"].BaseURL)
	}
}

func TestLoadManagedServiceTargetsFromEnvEmbed_FullJSON(t *testing.T) {
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"svc1":{"service_type":"pma","base_url":"http://localhost:2"}}`)
	defer func() { _ = os.Unsetenv("WORKER_MANAGED_SERVICE_TARGETS_JSON") }()
	targets := loadManagedServiceTargetsFromEnvEmbed(slog.Default())
	if len(targets) != 1 {
		t.Fatalf("len(targets)=%d", len(targets))
	}
	if targets["svc1"].ServiceType != "pma" || targets["svc1"].BaseURL != "http://localhost:2" {
		t.Errorf("target = %+v", targets["svc1"])
	}
}

func TestLoadManagedServiceTargetsFromEnvEmbed_InvalidJSON(t *testing.T) {
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{invalid`)
	defer func() { _ = os.Unsetenv("WORKER_MANAGED_SERVICE_TARGETS_JSON") }()
	targets := loadManagedServiceTargetsFromEnvEmbed(slog.Default())
	if len(targets) != 0 {
		t.Errorf("expected empty on invalid JSON, got %d", len(targets))
	}
}

func TestBuildMuxesFromEmbedConfig_HealthzReadyz(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, internal := buildMuxesFromEmbedConfig(exec, "token", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	if pub == nil || internal == nil {
		t.Fatal("muxes should be non-nil")
	}
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("healthz status = %d", w.Code)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	w2 := httptest.NewRecorder()
	pub.ServeHTTP(w2, req2)
	// readyz may be 200 or 503 depending on executor state
	if w2.Code != http.StatusOK && w2.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz status = %d", w2.Code)
	}
}

func TestBuildMuxesFromEmbedConfig_ReadyzNotReady(t *testing.T) {
	// Executor with non-direct runtime that is not available so Ready() returns (false, reason).
	exec := executor.New("nonexistent-runtime-xyz", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "token", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz status = %d, want 503", w.Code)
	}
	if body := w.Body.String(); body == "" || len(body) < 20 {
		t.Errorf("readyz body too short: %q", body)
	}
}

func TestBuildMuxesFromEmbedConfig_NilLogger(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, internal := buildMuxesFromEmbedConfig(exec, "token", t.TempDir(), nil, nil, embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	if pub == nil || internal == nil {
		t.Fatal("muxes should be non-nil")
	}
}

func TestApplyManagedServicesSocketByService(t *testing.T) {
	dir := t.TempDir()
	out := &embedProxyConfig{
		InternalProxy: embedInternalProxyConfig{
			SocketByService: map[string]string{},
		},
	}
	services := []nodepayloads.ConfigManagedService{
		{
			ServiceID:    "s1",
			Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
		},
		{ServiceID: ""},
		{ServiceID: "s2"},
	}
	applyManagedServicesSocketByService(out, dir, services)
	if len(out.InternalProxy.SocketByService) != 1 {
		t.Errorf("expected 1 entry, got %d", len(out.InternalProxy.SocketByService))
	}
	if _, ok := out.InternalProxy.SocketByService["s1"]; !ok {
		t.Error("expected s1")
	}
}

func TestApplyNodeConfigToEmbedProxyConfig_InvalidJSON(t *testing.T) {
	out := &embedProxyConfig{InternalProxy: embedInternalProxyConfig{SocketByService: map[string]string{}}}
	logger := slog.Default()
	applyNodeConfigToEmbedProxyConfig(out, t.TempDir(), "not json", logger)
	if len(out.InternalProxy.SocketByService) != 0 {
		t.Error("should not populate on invalid JSON")
	}
}

func TestApplyNodeConfigToEmbedProxyConfig_EmptyOrchestratorBaseURL(t *testing.T) {
	out := &embedProxyConfig{
		InternalProxy: embedInternalProxyConfig{
			SocketByService: map[string]string{},
			UpstreamBaseURL: "already-set",
		},
	}
	cfg := nodepayloads.NodeConfigurationPayload{Version: 1}
	raw, _ := json.Marshal(cfg)
	applyNodeConfigToEmbedProxyConfig(out, t.TempDir(), string(raw), slog.Default())
	if out.InternalProxy.UpstreamBaseURL != "already-set" {
		t.Errorf("UpstreamBaseURL should be unchanged: %q", out.InternalProxy.UpstreamBaseURL)
	}
}
