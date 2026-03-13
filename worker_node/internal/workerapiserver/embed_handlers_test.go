package workerapiserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/worker_node/internal/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
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

func TestBuildMuxesFromEmbedConfig_TelemetryNodeInfoAndStats(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "bearer-tok", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/node:info", http.NoBody)
	req.Header.Set("Authorization", "Bearer bearer-tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("node:info status = %d; %s", w.Code, w.Body.String())
	}
	var info map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if info["version"] != float64(1) || info["node_slug"] == "" {
		t.Errorf("node:info = %v", info)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/node:stats", http.NoBody)
	req2.Header.Set("Authorization", "Bearer bearer-tok")
	w2 := httptest.NewRecorder()
	pub.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("node:stats status = %d", w2.Code)
	}
	var stats map[string]interface{}
	if err := json.NewDecoder(w2.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats["version"] != float64(1) || stats["captured_at"] == "" {
		t.Errorf("node:stats = %v", stats)
	}
}

func TestBuildMuxesFromEmbedConfig_TelemetryNodeStatsWithEnv(t *testing.T) {
	t.Setenv("CONTAINER_RUNTIME", "docker")
	t.Setenv("CONTAINER_RUNTIME_VERSION", "20.10")
	defer func() {
		_ = os.Unsetenv("CONTAINER_RUNTIME")
		_ = os.Unsetenv("CONTAINER_RUNTIME_VERSION")
	}()
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "tok", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/node:stats", http.NoBody)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("node:stats status = %d", w.Code)
	}
	var stats map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	rt, _ := stats["container_runtime"].(map[string]interface{})
	if rt == nil {
		t.Fatal("container_runtime missing")
	}
	if rt["runtime"] != "docker" || rt["version"] != "20.10" {
		t.Errorf("container_runtime = %v", rt)
	}
}

func TestBuildMuxesFromEmbedConfig_TelemetryContainersAndLogsEmpty(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "tok", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/containers", http.NoBody)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("containers status = %d", w.Code)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["version"] != float64(1) {
		t.Errorf("containers = %v", out)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/containers/any-id", http.NoBody)
	req2.Header.Set("Authorization", "Bearer tok")
	w2 := httptest.NewRecorder()
	pub.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("containers/: status = %d", w2.Code)
	}
	req3 := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/logs?source_kind=service&source_name=node_manager", http.NoBody)
	req3.Header.Set("Authorization", "Bearer tok")
	w3 := httptest.NewRecorder()
	pub.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("logs status = %d", w3.Code)
	}
}

func TestBuildMuxesFromEmbedConfig_TelemetryContainersEmptyIDAndLogsBadRequest(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name     string
		path     string
		wantCode int
	}{
		{"containers_empty_id", "/v1/worker/telemetry/containers/", http.StatusNotFound},
		{"logs_bad_request", "/v1/worker/telemetry/logs", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			store, err := telemetry.Open(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = store.Close() }()
			exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
			pub, _ := buildMuxesFromEmbedConfig(exec, "tok", dir, store, slog.Default(), embedProxyConfig{
				ManagedServiceTargets: map[string]embedManagedServiceTarget{},
				InternalProxy:         embedInternalProxyConfig{},
			})
			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			req.Header.Set("Authorization", "Bearer tok")
			w := httptest.NewRecorder()
			pub.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}

func TestBuildMuxesFromEmbedConfig_TelemetryUnauthorized(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "secret", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/node:info", http.NoBody)
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("node:info wrong token status = %d", w.Code)
	}
}

//nolint:gocognit,gocyclo // integration-style test with multiple subtests
func TestBuildMuxesFromEmbedConfig_TelemetryWithStore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := telemetry.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if err := store.InsertNodeBoot(ctx, &telemetry.NodeBootRow{
		BootID: "b1", NodeSlug: "test-slug", BuildVersion: "1.0",
		PlatformOS: "linux", PlatformArch: "amd64", KernelVersion: "5.0",
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for i := 1; i <= 3; i++ {
		cid := fmt.Sprintf("cid-%d", i)
		if err := store.UpsertContainerInventory(ctx, &telemetry.ContainerRow{
			ContainerID: cid, ContainerName: "test", Kind: "managed", Runtime: "podman",
			ImageRef: "img", CreatedAt: now, LastSeenAt: now, Status: "running", Labels: map[string]string{},
		}); err != nil {
			t.Fatal(err)
		}
	}
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "tok", dir, store, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})

	t.Run("node_info", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/node:info", http.NoBody)
		req.Header.Set("Authorization", "Bearer tok")
		w := httptest.NewRecorder()
		pub.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		var info map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
			t.Fatal(err)
		}
		if info["node_slug"] != "test-slug" || info["version"] != float64(1) {
			t.Errorf("node:info = %v", info)
		}
	})

	t.Run("containers_list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/containers?limit=2&kind=managed", http.NoBody)
		req.Header.Set("Authorization", "Bearer tok")
		w := httptest.NewRecorder()
		pub.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		var listResp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
			t.Fatal(err)
		}
		if next, ok := listResp["next_page_token"].(string); ok && next != "" {
			req2 := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/containers?limit=2&page_token="+next, http.NoBody)
			req2.Header.Set("Authorization", "Bearer tok")
			w2 := httptest.NewRecorder()
			pub.ServeHTTP(w2, req2)
			if w2.Code != http.StatusOK {
				t.Fatalf("page2 status = %d", w2.Code)
			}
		}
	})

	for _, tc := range []struct {
		name     string
		path     string
		wantCode int
	}{
		{"containers_by_id", "/v1/worker/telemetry/containers/cid-1", http.StatusOK},
		{"logs", "/v1/worker/telemetry/logs?source_kind=service&source_name=node_manager&limit=500", http.StatusOK},
		{"logs_invalid_limit", "/v1/worker/telemetry/logs?source_kind=service&source_name=nm&limit=invalid", http.StatusOK},
		{"container_not_found", "/v1/worker/telemetry/containers/nonexistent", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			req.Header.Set("Authorization", "Bearer tok")
			w := httptest.NewRecorder()
			pub.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tc.wantCode)
			}
		})
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

func embedTestPubMux(t *testing.T, bearer string, socketByService map[string]string) *http.ServeMux {
	t.Helper()
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		bearer, t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{SocketByService: socketByService},
		},
	)
	return pub
}

func TestManagedServiceProxyHTTPHandler_MethodNotAllowed(t *testing.T) {
	pub := embedTestPubMux(t, "token", map[string]string{"svc1": filepath.Join(t.TempDir(), "svc1", "proxy.sock")})
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/managed-services/svc1/proxy:http", http.NoBody)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_Unauthorized(t *testing.T) {
	pub := embedTestPubMux(t, "token", map[string]string{"svc1": filepath.Join(t.TempDir(), "svc1", "proxy.sock")})
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
}

func TestManagedServiceProxyHTTPHandler_ServiceNotFound(t *testing.T) {
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{SocketByService: map[string]string{}},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/unknown/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_ServiceSocketEmpty(t *testing.T) {
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": ""},
			},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_PathNormalized(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySock := filepath.Join(svcDir, "proxy.sock")
	serviceSock := filepath.Join(svcDir, "service.sock")
	ln, err := net.Listen("unix", serviceSock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	var upstreamPath string
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	go func() { _ = http.Serve(ln, upstream) }()

	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "api"} // no leading slash
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if upstreamPath != "/api" {
		t.Errorf("upstream path = %q, want /api", upstreamPath)
	}
}

func TestManagedServiceProxyHTTPHandler_InvalidBody(t *testing.T) {
	dir := t.TempDir()
	proxySock := filepath.Join(dir, "svc1", "proxy.sock")
	if err := os.MkdirAll(filepath.Dir(proxySock), 0o700); err != nil {
		t.Fatal(err)
	}
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader([]byte("not json")))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_InvalidBodyB64(t *testing.T) {
	dir := t.TempDir()
	proxySock := filepath.Join(dir, "svc1", "proxy.sock")
	if err := os.MkdirAll(filepath.Dir(proxySock), 0o700); err != nil {
		t.Fatal(err)
	}
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "POST", Path: "/chat", BodyB64: "not-valid-base64!!"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_Success(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySock := filepath.Join(svcDir, "proxy.sock")
	serviceSock := filepath.Join(svcDir, "service.sock")
	ln, err := net.Listen("unix", serviceSock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "val")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("upstream-body"))
	})
	go func() { _ = http.Serve(ln, upstream) }()

	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	body := managedProxyRequest{
		Version: 1,
		Method:  "GET",
		Path:    "/",
		Headers: map[string][]string{"Accept": {"application/json"}},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body.String())
	}
	var resp managedProxyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("resp.Status = %d", resp.Status)
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.BodyB64)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "upstream-body" {
		t.Errorf("body = %q", decoded)
	}
	if resp.Headers == nil || resp.Headers["X-Custom"] == nil || resp.Headers["X-Custom"][0] != "val" {
		t.Errorf("Headers = %v", resp.Headers)
	}
}

func TestManagedServiceProxyHTTPHandler_SuccessWithPostBody(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySock := filepath.Join(svcDir, "proxy.sock")
	serviceSock := filepath.Join(svcDir, "service.sock")
	ln, err := net.Listen("unix", serviceSock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("upstream got method %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})
	go func() { _ = http.Serve(ln, upstream) }()

	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(`{"message":"hello"}`))
	body := managedProxyRequest{
		Version: 1,
		Method:  "POST",
		Path:    "/chat",
		BodyB64: bodyB64,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body %s", w.Code, w.Body.String())
	}
	var resp managedProxyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusCreated {
		t.Errorf("resp.Status = %d", resp.Status)
	}
}

func TestManagedServiceProxyHTTPHandler_NoAuthWhenBearerEmpty(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySock := filepath.Join(svcDir, "proxy.sock")
	serviceSock := filepath.Join(svcDir, "service.sock")
	ln, err := net.Listen("unix", serviceSock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		_ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	}()

	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d (no auth when bearer empty)", w.Code)
	}
}

func TestManagedServiceProxyHTTPHandler_UpstreamError(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "svc1")
	if err := os.MkdirAll(svcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	proxySock := filepath.Join(svcDir, "proxy.sock")
	// No listener on service.sock so client.Do will fail
	pub, _ := buildMuxesFromEmbedConfig(
		executor.New("direct", 5*time.Second, 1024, "", "", nil),
		"token", t.TempDir(), nil, slog.Default(),
		embedProxyConfig{
			InternalProxy: embedInternalProxyConfig{
				SocketByService: map[string]string{"svc1": proxySock},
			},
		},
	)
	body := managedProxyRequest{Version: 1, Method: "GET", Path: "/"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/svc1/proxy:http", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502; body %s", w.Code, w.Body.String())
	}
}
