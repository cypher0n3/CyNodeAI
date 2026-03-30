package workerapiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/internal/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
)

// failingJobRunner implements embedRunner and returns an error from RunJob for testing the 500 path.
type failingJobRunner struct {
	*executor.Executor
}

func (failingJobRunner) RunJob(_ context.Context, _ *workerapi.RunJobRequest, _ string) (*workerapi.RunJobResponse, error) {
	return nil, errors.New("injected failure")
}

func (f failingJobRunner) Ready(ctx context.Context) (ready bool, msg string) {
	return f.Executor.Ready(ctx)
}

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

func TestEmbedBearerOK(t *testing.T) {
	if !embedBearerOK("ignored", "") {
		t.Fatal("empty expected token should allow any auth header")
	}
	if !embedBearerOK("Bearer secret", "secret") {
		t.Fatal("matching token should succeed")
	}
	if embedBearerOK("Bearer x", "secret") {
		t.Fatal("wrong token should fail")
	}
	if embedBearerOK("Basic secret", "secret") {
		t.Fatal("non-Bearer scheme should fail")
	}
	if embedBearerOK("Bearer secret", "secret ") {
		t.Fatal("trailing space on expected token should not match")
	}
}

// TestBearerAuth verifies embed telemetry routes reject wrong bearer and accept the configured token.
func TestBearerAuth(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "good-bearer", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	t.Run("wrong_rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/node:info", http.NoBody)
		req.Header.Set("Authorization", "Bearer bad")
		w := httptest.NewRecorder()
		pub.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("code=%d want 401", w.Code)
		}
	})
	t.Run("correct_ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/node:info", http.NoBody)
		req.Header.Set("Authorization", "Bearer good-bearer")
		w := httptest.NewRecorder()
		pub.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("code=%d body=%s", w.Code, w.Body.String())
		}
	})
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

func TestBuildMuxesFromEmbedConfig_JobsRun_Unauthorized(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "secret", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	body, _ := json.Marshal(workerapi.RunJobRequest{
		Version: 1, TaskID: "t1", JobID: "j1",
		Sandbox: workerapi.SandboxSpec{Command: []string{"true"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body))
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("jobs:run without bearer status = %d, want 401", w.Code)
	}
}

func TestBuildMuxesFromEmbedConfig_JobsRun_Success(t *testing.T) {
	exec := executor.New("direct", 10*time.Second, 1024, "", "", nil)
	dir := t.TempDir()
	pub, _ := buildMuxesFromEmbedConfig(exec, "tok", dir, nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	body, _ := json.Marshal(workerapi.RunJobRequest{
		Version: 1, TaskID: "t1", JobID: "j1",
		Sandbox: workerapi.SandboxSpec{Command: []string{"true"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("jobs:run status = %d, body: %s", w.Code, w.Body.String())
	}
	var resp workerapi.RunJobResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("jobs:run status = %q, want completed", resp.Status)
	}
}

func TestBuildMuxesFromEmbedConfig_JobsRun_InvalidBody(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "tok", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader([]byte("not json")))
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("jobs:run invalid body status = %d, want 400", w.Code)
	}
}

func TestBuildMuxesFromEmbedConfig_JobsRun_BodyTooLarge(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "tok", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	// Valid JSON with >10MB string so Decode hits MaxBytesReader limit (same pattern as _bdd/steps.go).
	big := bytes.Repeat([]byte("x"), 11*1024*1024)
	body := []byte(`{"version":1,"task_id":"t","job_id":"j","sandbox":{"image":"a","command":["`)
	body = append(body, big...)
	body = append(body, []byte(`"]}}`)...)
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("jobs:run body too large status = %d, want 413", w.Code)
	}
}

func TestBuildMuxesFromEmbedConfig_JobsRun_ValidationFailed(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "tok", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	body, _ := json.Marshal(workerapi.RunJobRequest{
		Version: 1, TaskID: "t1", JobID: "j1",
		Sandbox: workerapi.SandboxSpec{}, // no Command, no JobSpecJSON
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("jobs:run validation status = %d, want 400", w.Code)
	}
}

func TestBuildMuxesFromEmbedConfig_JobsRun_RunnerReturnsError(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	runner := failingJobRunner{Executor: exec}
	pub, _ := buildMuxesFromEmbedConfig(runner, "tok", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	body, _ := json.Marshal(workerapi.RunJobRequest{
		Version: 1, TaskID: "t1", JobID: "j1",
		Sandbox: workerapi.SandboxSpec{Command: []string{"true"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("jobs:run when runner returns error: status = %d, want 500", w.Code)
	}
}

func TestBuildMuxesFromEmbedConfig_EmbedPublicHTTP_Status(t *testing.T) {
	emptyCfg := embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	}
	tests := []struct {
		name     string
		method   string
		path     string
		auth     string
		wantCode int
	}{
		{"jobs_run_GET_method_not_allowed", http.MethodGet, "/v1/worker/jobs:run", "Bearer tok", http.StatusMethodNotAllowed},
		{"telemetry_node_info_unauthorized", http.MethodGet, "/v1/worker/telemetry/node:info", "Bearer wrong", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
			pub, _ := buildMuxesFromEmbedConfig(exec, "secret", t.TempDir(), nil, slog.Default(), emptyCfg)
			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			req.Header.Set("Authorization", tt.auth)
			w := httptest.NewRecorder()
			pub.ServeHTTP(w, req)
			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
		})
	}
}

func TestBuildMuxesFromEmbedConfig_NodeInfo_KernelVersionReadFails(t *testing.T) {
	oldPath := kernelVersionPath
	kernelVersionPath = "/nonexistent/path/for/coverage"
	defer func() { kernelVersionPath = oldPath }()
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "tok", t.TempDir(), nil, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/node:info", http.NoBody)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("node:info status = %d", w.Code)
	}
	var data map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	platform, _ := data["platform"].(map[string]interface{})
	if platform != nil && platform["kernel_version"] != "" {
		t.Errorf("expected empty kernel_version when read fails, got %q", platform["kernel_version"])
	}
}

func TestBuildMuxesFromEmbedConfig_NodeInfo_StoreEmptyKernelFallback(t *testing.T) {
	oldPath := kernelVersionPath
	kernelVersionPath = "/nonexistent/kernel-version-for-test"
	defer func() { kernelVersionPath = oldPath }()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := telemetry.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if err := store.InsertNodeBoot(ctx, &telemetry.NodeBootRow{
		BootID: "b-empty-kernel", NodeSlug: "slug", BuildVersion: "1.0",
		PlatformOS: "linux", PlatformArch: "amd64", KernelVersion: "",
	}); err != nil {
		t.Fatal(err)
	}
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	pub, _ := buildMuxesFromEmbedConfig(exec, "tok", dir, store, slog.Default(), embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy:         embedInternalProxyConfig{},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/node:info", http.NoBody)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	pub.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("node:info status = %d", w.Code)
	}
	var data map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	platform, _ := data["platform"].(map[string]interface{})
	if platform == nil || platform["kernel_version"] != "" {
		t.Errorf("expected empty kernel_version from fallback when store row omits it, got %v", platform)
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
