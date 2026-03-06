package main

import (
	"bytes"
	"encoding/base64"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/cmd/worker-api/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
)

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_WORKER_GETENV")
	if getEnv("TEST_WORKER_GETENV", "def") != "def" {
		t.Error("getEnv default")
	}
	_ = os.Setenv("TEST_WORKER_GETENV", "val")
	defer func() { _ = os.Unsetenv("TEST_WORKER_GETENV") }()
	if getEnv("TEST_WORKER_GETENV", "def") != "val" {
		t.Error("getEnv from env")
	}
}

func TestGetEnvInt(t *testing.T) {
	_ = os.Unsetenv("TEST_WORKER_INT")
	if getEnvInt("TEST_WORKER_INT", 42) != 42 {
		t.Error("getEnvInt default")
	}
	_ = os.Setenv("TEST_WORKER_INT", "99")
	defer func() { _ = os.Unsetenv("TEST_WORKER_INT") }()
	if getEnvInt("TEST_WORKER_INT", 0) != 99 {
		t.Error("getEnvInt from env")
	}
	_ = os.Setenv("TEST_WORKER_INT", "bad")
	if getEnvInt("TEST_WORKER_INT", 7) != 7 {
		t.Error("getEnvInt invalid should use default")
	}
}

func TestRequireBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		authz    string
		expected string
		want     bool
	}{
		{"empty", "", "token", false},
		{"no Bearer prefix", "token", "token", false},
		{"wrong token", "Bearer wrong", "token", false},
		{"valid", "Bearer token", "token", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", http.NoBody)
			if tt.authz != "" {
				r.Header.Set("Authorization", tt.authz)
			}
			got := requireBearerToken(r, tt.expected)
			if got != tt.want {
				t.Errorf("requireBearerToken = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteProblem(t *testing.T) {
	w := httptest.NewRecorder()
	writeProblem(w, http.StatusBadRequest, "urn:test", "Bad", "detail here")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d", w.Code)
	}
	var d struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
	}
	if err := json.NewDecoder(w.Body).Decode(&d); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if d.Type != "urn:test" || d.Title != "Bad" || d.Detail != "detail here" {
		t.Errorf("body %+v", d)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"k": "v"})
	if w.Code != http.StatusOK {
		t.Errorf("status %d", w.Code)
	}
	var m map[string]string
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["k"] != "v" {
		t.Errorf("body %+v", m)
	}
}

func TestNewServer(t *testing.T) {
	_ = os.Unsetenv("LISTEN_ADDR")
	srv := newServer(http.NewServeMux())
	if srv.Addr != ":9190" {
		t.Errorf("default addr: %s", srv.Addr)
	}
	_ = os.Setenv("LISTEN_ADDR", ":9999")
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()
	srv2 := newServer(http.NewServeMux())
	if srv2.Addr != ":9999" {
		t.Errorf("env addr: %s", srv2.Addr)
	}
}

func runJobCmd() []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", "echo", "ok"}
	}
	return []string{"echo", "ok"}
}

func TestHandleRunJob(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024, "", "", nil)
	mux := newMux(exec, "test-bearer", "", nil, slog.Default())
	cmd := runJobCmd()
	reqBody := workerapi.RunJobRequest{
		Version: 1, TaskID: "task-1", JobID: "job-1",
		Sandbox: workerapi.SandboxSpec{Command: cmd},
	}
	body, _ := json.Marshal(reqBody)

	t.Run("unauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body))
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status %d", w.Code)
		}
	})
	t.Run("success", func(t *testing.T) {
		postRunJobSuccess(t, mux, body)
	})
	t.Run("success with workspace root", func(t *testing.T) {
		muxWithWorkspace := newMux(executor.New("direct", 5*time.Second, 1024, "", "", nil), "test-bearer", t.TempDir(), nil, slog.Default())
		postRunJobSuccess(t, muxWithWorkspace, body)
	})
	t.Run("workspace creation failure returns 500", func(t *testing.T) {
		dir := t.TempDir()
		blocker := filepath.Join(dir, "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		// workspaceDir will be blocker/job-1; MkdirAll fails because blocker is a file
		muxBad := newMux(executor.New("direct", 5*time.Second, 1024, "", "", nil), "test-bearer", blocker, nil, slog.Default())
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer test-bearer")
		muxBad.ServeHTTP(w, r)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status %d want 500", w.Code)
		}
	})
	t.Run("bad request body", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader([]byte("not json")))
		r.Header.Set("Authorization", "Bearer test-bearer")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status %d", w.Code)
		}
	})
	postRunJobExpectBadRequest(t, mux, &workerapi.RunJobRequest{Version: 2, TaskID: "t", JobID: "j", Sandbox: workerapi.SandboxSpec{Command: cmd}}, "wrong version")
	postRunJobExpectBadRequest(t, mux, &workerapi.RunJobRequest{Version: 1, TaskID: "", JobID: "", Sandbox: workerapi.SandboxSpec{Command: cmd}}, "missing task_id job_id")
	postRunJobExpectBadRequest(t, mux, &workerapi.RunJobRequest{Version: 1, TaskID: "t", JobID: "j", Sandbox: workerapi.SandboxSpec{Command: nil}}, "empty command")
}

func postRunJobSuccess(t *testing.T, mux *http.ServeMux, body []byte) {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer test-bearer")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status %d body %s", w.Code, w.Body.String())
	}
	var resp workerapi.RunJobResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("resp status %s", resp.Status)
	}
}

// postRunJobExpectBadRequest POSTs the request to mux with Bearer test-bearer and asserts 400.
func postRunJobExpectBadRequest(t *testing.T, mux *http.ServeMux, badReq *workerapi.RunJobRequest, subName string) {
	t.Helper()
	body2, _ := json.Marshal(badReq)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body2))
	r.Header.Set("Authorization", "Bearer test-bearer")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("%s: status %d", subName, w.Code)
	}
}

func TestHealthz(t *testing.T) {
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "token", "", nil, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Errorf("healthz: %d %s", w.Code, w.Body.String())
	}
}

func TestReadyz(t *testing.T) {
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "token", "", nil, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "ready" {
		t.Errorf("readyz: %d %s", w.Code, w.Body.String())
	}
}

func TestReadyz_notReady(t *testing.T) {
	exec := executor.New("nonexistent-runtime-xyz", time.Second, 1024, "", "", nil)
	mux := newMux(exec, "token", "", nil, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz when not ready: %d %s", w.Code, w.Body.String())
	}
}

func TestRunJob_RequestTooLarge(t *testing.T) {
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "token", "", nil, slog.Default())
	// Body > 10 MiB so MaxBytesReader triggers; use valid JSON shape so the error is "request body too large" not "invalid JSON"
	big := bytes.Repeat([]byte("x"), 11*1024*1024)
	body := []byte(`{"version":1,"task_id":"t","job_id":"j","sandbox":{"image":"a","command":["`)
	body = append(body, big...)
	body = append(body, []byte(`"]}}`)...)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", w.Code)
	}
}

func TestRunMainMissingToken(t *testing.T) {
	_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
	defer func() { _ = os.Unsetenv("WORKER_API_BEARER_TOKEN") }()
	code := runMain(context.Background())
	if code != 1 {
		t.Errorf("runMain should return 1 when token unset, got %d", code)
	}
}

func TestRunMainWithContextCancel(t *testing.T) {
	_ = os.Setenv("WORKER_API_BEARER_TOKEN", "test-token")
	_ = os.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	_ = os.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "")
	blocker := filepath.Join(t.TempDir(), "state-blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0o600)
	_ = os.Setenv("WORKER_API_STATE_DIR", blocker)
	defer func() { _ = os.Unsetenv("WORKER_API_BEARER_TOKEN") }()
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()
	defer func() { _ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR") }()
	defer func() { _ = os.Unsetenv("WORKER_API_STATE_DIR") }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- runMain(ctx)
	}()
	cancel()
	code := <-done
	if code != 0 {
		t.Errorf("runMain after cancel should return 0, got %d", code)
	}
}

func TestRunMain_InternalUnixListenFailure(t *testing.T) {
	_ = os.Setenv("WORKER_API_BEARER_TOKEN", "test-token")
	_ = os.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	_ = os.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "")
	_ = os.Setenv("WORKER_INTERNAL_LISTEN_UNIX", "/this/path/does/not/exist/worker.sock")
	blocker := filepath.Join(t.TempDir(), "state-blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0o600)
	_ = os.Setenv("WORKER_API_STATE_DIR", blocker)
	defer func() { _ = os.Unsetenv("WORKER_API_BEARER_TOKEN") }()
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()
	defer func() { _ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR") }()
	defer func() { _ = os.Unsetenv("WORKER_INTERNAL_LISTEN_UNIX") }()
	defer func() { _ = os.Unsetenv("WORKER_API_STATE_DIR") }()

	code := runMain(context.Background())
	if code != 1 {
		t.Fatalf("runMain should fail when unix socket path cannot be listened, got %d", code)
	}
}

func TestTelemetryEndpoints(t *testing.T) {
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "telemetry-token", "", nil, slog.Default())
	telemetryGetAndCheck(t, mux, "/v1/worker/telemetry/node:info", "node_slug")
	telemetryGetAndCheck(t, mux, "/v1/worker/telemetry/node:stats", "captured_at")
}

func telemetryGetAndCheck(t *testing.T, mux *http.ServeMux, path, requiredKey string) {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	r.Header.Set("Authorization", "Bearer telemetry-token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("%s: got %d %s", path, w.Code, w.Body.String())
		return
	}
	var m map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["version"] != float64(1) || m[requiredKey] == nil {
		t.Errorf("%s body %+v", path, m)
	}
}

func TestTelemetryUnauthorized(t *testing.T) {
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "telemetry-token", "", nil, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/node:info", http.NoBody)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("node:info no bearer: got %d", w.Code)
	}
}

func telemetryMuxWithStore(t *testing.T) (context.Context, *telemetry.Store, *http.ServeMux) {
	t.Helper()
	ctx := context.Background()
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("telemetry Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	doRetentionAndVacuumOnce(ctx, store, slog.Default())
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "telemetry-token", "", store, slog.Default())
	return ctx, store, mux
}

func telemetryGET(t *testing.T, mux *http.ServeMux, path string, wantCode int) {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	r.Header.Set("Authorization", "Bearer telemetry-token")
	mux.ServeHTTP(w, r)
	if w.Code != wantCode {
		t.Errorf("%s: got %d want %d", path, w.Code, wantCode)
	}
}

func TestTelemetryListContainersEmpty(t *testing.T) {
	_, _, mux := telemetryMuxWithStore(t)
	telemetryGET(t, mux, "/v1/worker/telemetry/containers", http.StatusOK)
}

func TestTelemetryGetContainerNotFound(t *testing.T) {
	_, _, mux := telemetryMuxWithStore(t)
	telemetryGET(t, mux, "/v1/worker/telemetry/containers/nonexistent", http.StatusNotFound)
}

func TestTelemetryLogsMissingParams(t *testing.T) {
	_, _, mux := telemetryMuxWithStore(t)
	telemetryGET(t, mux, "/v1/worker/telemetry/logs", http.StatusBadRequest)
}

func TestTelemetryLogsOkEmpty(t *testing.T) {
	_, _, mux := telemetryMuxWithStore(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/logs?source_kind=service&source_name=worker-api", http.NoBody)
	r.Header.Set("Authorization", "Bearer telemetry-token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("logs: %d", w.Code)
		return
	}
	var m map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["version"] != float64(1) {
		t.Errorf("body %+v", m)
	}
	if _, ok := m["events"]; !ok {
		t.Errorf("body missing events key %+v", m)
	}
	if _, ok := m["truncated"]; !ok {
		t.Errorf("body missing truncated key %+v", m)
	}
}

func TestTelemetryContainersMethodNotAllowed(t *testing.T) {
	_, _, mux := telemetryMuxWithStore(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/telemetry/containers", http.NoBody)
	r.Header.Set("Authorization", "Bearer telemetry-token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST containers: %d", w.Code)
	}
}

func TestTelemetryListContainersWithLimit(t *testing.T) {
	_, _, mux := telemetryMuxWithStore(t)
	telemetryGET(t, mux, "/v1/worker/telemetry/containers?limit=50", http.StatusOK)
}

func TestTelemetryLogsWithLimit(t *testing.T) {
	_, _, mux := telemetryMuxWithStore(t)
	telemetryGET(t, mux, "/v1/worker/telemetry/logs?source_kind=service&source_name=x&limit=100", http.StatusOK)
}

func TestTelemetryGetContainerFound(t *testing.T) {
	ctx, store, mux := telemetryMuxWithStore(t)
	if err := store.InsertTestContainer(ctx, "found-id", "found-name", "managed", "running", "task-1", "job-1"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/containers/found-id", http.NoBody)
	r.Header.Set("Authorization", "Bearer telemetry-token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("get container: %d %s", w.Code, w.Body.String())
		return
	}
	var m map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	cont, _ := m["container"].(map[string]interface{})
	if cont == nil || cont["container_id"] != "found-id" {
		t.Errorf("container: %+v", m)
	}
}

func TestTelemetryListContainersNextPageToken(t *testing.T) {
	ctx, store, mux := telemetryMuxWithStore(t)
	_ = store.InsertTestContainer(ctx, "p1", "n1", "managed", "running", "", "")
	_ = store.InsertTestContainer(ctx, "p2", "n2", "managed", "running", "", "")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/containers?limit=1", http.NoBody)
	r.Header.Set("Authorization", "Bearer telemetry-token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("list: %d", w.Code)
		return
	}
	var m map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["next_page_token"] == nil || m["next_page_token"] == "" {
		t.Errorf("expected next_page_token: %+v", m)
	}
}

func TestTelemetryGetContainerEmptyID(t *testing.T) {
	_, _, mux := telemetryMuxWithStore(t)
	telemetryGET(t, mux, "/v1/worker/telemetry/containers/", http.StatusNotFound)
}

func TestRunRetentionAndVacuum_TickerBranches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	oldRet, oldVac := retentionTickerInterval, vacuumTickerInterval
	retentionTickerInterval = 2 * time.Millisecond
	vacuumTickerInterval = 5 * time.Millisecond
	defer func() { retentionTickerInterval, vacuumTickerInterval = oldRet, oldVac }()
	done := make(chan struct{})
	go func() {
		runRetentionAndVacuum(ctx, store, slog.Default())
		close(done)
	}()
	time.Sleep(15 * time.Millisecond)
	cancel()
	<-done
}

func TestDoRetentionAndVacuumOnce_ErrorPaths(t *testing.T) {
	ctx := context.Background()
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = store.Close()
	doRetentionAndVacuumOnce(ctx, store, slog.Default())
}

func TestTelemetryContainersClosedStore(t *testing.T) {
	ctx := context.Background()
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("telemetry Open: %v", err)
	}
	_ = store.Close()
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "telemetry-token", "", store, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/containers", http.NoBody)
	r.Header.Set("Authorization", "Bearer telemetry-token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("closed store list containers: %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/containers/some-id", http.NoBody)
	r2.Header.Set("Authorization", "Bearer telemetry-token")
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("closed store get container: %d", w2.Code)
	}
	w3 := httptest.NewRecorder()
	r3 := httptest.NewRequest(http.MethodGet, "/v1/worker/telemetry/logs?source_kind=service&source_name=x", http.NoBody)
	r3.Header.Set("Authorization", "Bearer telemetry-token")
	mux.ServeHTTP(w3, r3)
	if w3.Code != http.StatusBadRequest {
		t.Errorf("closed store logs: %d", w3.Code)
	}
}

func TestManagedServiceProxy_Success(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/chat/completion" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Request-ID") != "req-123" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "req-123")
		_, _ = w.Write([]byte(`{"content":"ok"}`))
	}))
	defer upstream.Close()
	targetsJSON := map[string]managedServiceTarget{
		"pma-main": {ServiceType: "pma", BaseURL: upstream.URL},
	}
	rawTargets, _ := json.Marshal(targetsJSON)
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", string(rawTargets))
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "token", "", nil, slog.Default())
	reqBody := managedServiceProxyRequest{
		Version: 1,
		Method:  http.MethodPost,
		Path:    "/internal/chat/completion",
		Headers: map[string][]string{
			"X-Request-ID": {"req-123"},
			"X-Unsafe":     {"blocked"},
		},
		BodyB64: base64.StdEncoding.EncodeToString([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)),
	}
	rawReq, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/pma-main/proxy:http", bytes.NewReader(rawReq))
	r.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp managedServiceProxyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("upstream status %d", resp.Status)
	}
	decoded, _ := base64.StdEncoding.DecodeString(resp.BodyB64)
	if !strings.Contains(string(decoded), `"content":"ok"`) {
		t.Fatalf("unexpected proxied body: %s", string(decoded))
	}
	if _, ok := resp.Headers["X-Request-Id"]; !ok {
		t.Fatalf("expected response allowlisted header, got %+v", resp.Headers)
	}
}

func TestManagedServiceProxy_AuthAndUnknownService(t *testing.T) {
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"pma-main":{"service_type":"pma","base_url":"http://127.0.0.1:1"}}`)
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "token", "", nil, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/pma-main/proxy:http", bytes.NewReader([]byte(`{}`)))
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/unknown/proxy:http", bytes.NewReader([]byte(`{}`)))
	r2.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w2.Code)
	}
}

func TestManagedServiceProxy_RequestValidation(t *testing.T) {
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"pma-main":{"service_type":"pma","base_url":"http://127.0.0.1:1"}}`)
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "token", "", nil, slog.Default())
	cases := []struct {
		name string
		body string
		code int
	}{
		{name: "invalid json", body: `not-json`, code: http.StatusBadRequest},
		{name: "bad version", body: `{"version":2,"method":"POST","path":"/x"}`, code: http.StatusBadRequest},
		{name: "bad method", body: `{"version":1,"method":"TRACE","path":"/x"}`, code: http.StatusBadRequest},
		{name: "bad path", body: `{"version":1,"method":"POST","path":"http://x"}`, code: http.StatusBadRequest},
		{name: "bad body_b64", body: `{"version":1,"method":"POST","path":"/x","body_b64":"%%%"}`, code: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/pma-main/proxy:http", bytes.NewReader([]byte(tc.body)))
			r.Header.Set("Authorization", "Bearer token")
			mux.ServeHTTP(w, r)
			if w.Code != tc.code {
				t.Fatalf("status %d want %d body %s", w.Code, tc.code, w.Body.String())
			}
		})
	}
}

func TestManagedServiceProxy_UpstreamErrors(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("a"), maxManagedProxyBodyBytes+4))
	}))
	defer upstream.Close()
	targetsJSON := map[string]managedServiceTarget{
		"pma-main": {ServiceType: "pma", BaseURL: upstream.URL},
	}
	rawTargets, _ := json.Marshal(targetsJSON)
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", string(rawTargets))
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "token", "", nil, slog.Default())
	reqBody := `{"version":1,"method":"GET","path":"/x"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/pma-main/proxy:http", bytes.NewReader([]byte(reqBody)))
	r.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body %s", w.Code, w.Body.String())
	}
}

func TestLoadManagedServiceTargetsFromEnv(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", "")
		got := loadManagedServiceTargetsFromEnv(slog.Default())
		if len(got) != 0 {
			t.Fatalf("expected empty targets, got %+v", got)
		}
	})
	t.Run("typed map", func(t *testing.T) {
		t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"pma-main":{"service_type":"pma","base_url":"http://x"}}`)
		got := loadManagedServiceTargetsFromEnv(slog.Default())
		if got["pma-main"].ServiceType != "pma" || got["pma-main"].BaseURL != "http://x" {
			t.Fatalf("unexpected targets: %+v", got)
		}
	})
	t.Run("simple map fallback", func(t *testing.T) {
		t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"svc":"http://y"}`)
		got := loadManagedServiceTargetsFromEnv(slog.Default())
		if got["svc"].BaseURL != "http://y" {
			t.Fatalf("unexpected targets: %+v", got)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{`)
		got := loadManagedServiceTargetsFromEnv(slog.Default())
		if len(got) != 0 {
			t.Fatalf("expected empty targets on invalid json, got %+v", got)
		}
	})
}

func TestManagedProxyHelpers(t *testing.T) {
	if !isAllowedProxyMethod(http.MethodPatch) || isAllowedProxyMethod(http.MethodOptions) {
		t.Fatal("method allowlist mismatch")
	}
	if !isSafeProxyPath("/ok") || isSafeProxyPath("http://bad") || isSafeProxyPath("bad") {
		t.Fatal("path validation mismatch")
	}
	if !isAllowedProxyRequestHeader("Content-Type") || isAllowedProxyRequestHeader("X-Unsafe") {
		t.Fatal("request header allowlist mismatch")
	}
	if !isAllowedProxyResponseHeader("X-Request-Id") || isAllowedProxyResponseHeader("Set-Cookie") {
		t.Fatal("response header allowlist mismatch")
	}
}

func TestForwardManagedProxyRequest_ConnectionError(t *testing.T) {
	req := &managedServiceProxyRequest{
		Version: 1, Method: http.MethodGet, Path: "/x",
	}
	_, status, detail := forwardManagedProxyRequest(
		context.Background(),
		managedServiceTarget{ServiceType: "pma", BaseURL: "http://127.0.0.1:1"},
		req,
		nil,
	)
	if status != http.StatusBadGateway || detail == "" {
		t.Fatalf("expected bad gateway detail, got status=%d detail=%q", status, detail)
	}
}

func TestLoadWorkerProxyConfig_FromNodeConfig(t *testing.T) {
	cfg := nodepayloads.NodeConfigurationPayload{
		Version: 1,
		Orchestrator: nodepayloads.ConfigOrchestrator{
			BaseURL: "http://orchestrator.internal:12082",
		},
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "pma-main",
					ServiceType: "pma",
					Inference:   &nodepayloads.ConfigManagedServiceInference{BaseURL: "http://127.0.0.1:8090"},
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "agent-token-1",
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(cfg)
	t.Setenv("WORKER_NODE_CONFIG_JSON", string(raw))
	t.Setenv("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL", "")
	out := loadWorkerProxyConfig(slog.Default())
	if len(out.ManagedServiceTargets) != 0 {
		t.Fatalf("expected no direct managed targets from node config, got %+v", out.ManagedServiceTargets)
	}
	if out.InternalProxy.UpstreamBaseURL != "http://orchestrator.internal:12082" {
		t.Fatalf("unexpected internal upstream: %q", out.InternalProxy.UpstreamBaseURL)
	}
	if out.InternalProxy.AllowedTokens["agent-token-1"] != "pma-main" {
		t.Fatalf("expected allowed token from node config, got %+v", out.InternalProxy.AllowedTokens)
	}
}

func TestLoadWorkerProxyConfig_FallbackFromEnv(t *testing.T) {
	t.Setenv("WORKER_NODE_CONFIG_JSON", "")
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"pma-main":{"service_type":"pma","base_url":"http://127.0.0.1:8090"}}`)
	t.Setenv("WORKER_INTERNAL_AGENT_TOKENS_JSON", `{"tok":"svc"}`)
	t.Setenv("ORCHESTRATOR_URL", "http://orchestrator:12082")
	out := loadWorkerProxyConfig(slog.Default())
	if out.ManagedServiceTargets["pma-main"].BaseURL != "http://127.0.0.1:8090" {
		t.Fatalf("managed targets fallback failed: %+v", out.ManagedServiceTargets)
	}
	if out.InternalProxy.AllowedTokens["tok"] != "svc" {
		t.Fatalf("internal tokens fallback failed: %+v", out.InternalProxy.AllowedTokens)
	}
	if out.InternalProxy.UpstreamBaseURL != "http://orchestrator:12082" {
		t.Fatalf("internal upstream fallback failed: %q", out.InternalProxy.UpstreamBaseURL)
	}
}

func TestLoadWorkerProxyConfig_InvalidNodeConfigFallsBack(t *testing.T) {
	t.Setenv("WORKER_NODE_CONFIG_JSON", `{`)
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"svc":{"service_type":"pma","base_url":"http://127.0.0.1:8090"}}`)
	out := loadWorkerProxyConfig(slog.Default())
	if out.ManagedServiceTargets["svc"].BaseURL != "http://127.0.0.1:8090" {
		t.Fatalf("expected env fallback targets, got %+v", out.ManagedServiceTargets)
	}
}

func TestInternalOrchestratorProxy_MCP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp/call" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	cfg := internalOrchestratorProxyConfig{
		UpstreamBaseURL: upstream.URL,
		AllowedTokens:   map[string]string{"agent-token": "pma-main"},
	}
	mux := newInternalMux(cfg, slog.Default())
	body := `{"version":1,"method":"POST","path":"/mcp/call"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", bytes.NewReader([]byte(body)))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Authorization", "Bearer agent-token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestInternalOrchestratorProxy_AuthAndLoopback(t *testing.T) {
	cfg := internalOrchestratorProxyConfig{
		UpstreamBaseURL: "http://127.0.0.1:1",
		AllowedTokens:   map[string]string{"agent-token": "svc"},
	}
	mux := newInternalMux(cfg, slog.Default())
	body := `{"version":1,"method":"POST","path":"/x"}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/agent:ready", bytes.NewReader([]byte(body)))
	r.RemoteAddr = "192.0.2.10:12345"
	r.Header.Set("Authorization", "Bearer agent-token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-loopback, got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/agent:ready", bytes.NewReader([]byte(body)))
	r2.RemoteAddr = "127.0.0.1:12345"
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d", w2.Code)
	}
}

func TestInternalOrchestratorProxy_UpstreamMissing(t *testing.T) {
	cfg := internalOrchestratorProxyConfig{
		AllowedTokens: map[string]string{"agent-token": "svc"},
	}
	mux := newInternalMux(cfg, slog.Default())
	body := `{"version":1,"method":"POST","path":"/x"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", bytes.NewReader([]byte(body)))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Authorization", "Bearer agent-token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for missing upstream, got %d", w.Code)
	}
}

func TestPublicMux_DoesNotExposeInternalProxyRoutes(t *testing.T) {
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "token", "", nil, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", bytes.NewReader([]byte(`{}`)))
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for internal route on public mux, got %d", w.Code)
	}
}

func TestLoadInternalProxyTokensFromEnv(t *testing.T) {
	t.Setenv("WORKER_INTERNAL_AGENT_TOKENS_JSON", `{"tok-a":"svc-a"}`)
	got := loadInternalProxyTokensFromEnv()
	if got["tok-a"] != "svc-a" {
		t.Fatalf("unexpected internal tokens: %+v", got)
	}
	t.Setenv("WORKER_INTERNAL_AGENT_TOKENS_JSON", `{`)
	got = loadInternalProxyTokensFromEnv()
	if len(got) != 0 {
		t.Fatalf("expected empty tokens for invalid json, got %+v", got)
	}
}

func TestDeriveManagedServiceTargetsFromNodeConfig(t *testing.T) {
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "svc-1",
					ServiceType: "pma",
					Inference:   &nodepayloads.ConfigManagedServiceInference{BaseURL: "http://127.0.0.1:8090"},
				},
				{
					ServiceID:   "svc-2",
					ServiceType: "pma",
				},
			},
		},
	}
	got := deriveManagedServiceTargetsFromNodeConfig(cfg)
	if len(got) != 0 {
		t.Fatalf("expected no targets derived directly from node config, got %+v", got)
	}
}

type fakeAddr struct {
	network string
	addr    string
}

func (f fakeAddr) Network() string { return f.network }
func (f fakeAddr) String() string  { return f.addr }

func TestIsLoopbackRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	r.RemoteAddr = "127.0.0.1:1234"
	if !isLoopbackRequest(r) {
		t.Fatal("expected loopback remote addr to pass")
	}
	r2 := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	r2.RemoteAddr = "192.0.2.1:1234"
	if isLoopbackRequest(r2) {
		t.Fatal("expected non-loopback remote addr to fail")
	}
	r3 := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	r3 = r3.WithContext(context.WithValue(r3.Context(), http.LocalAddrContextKey, fakeAddr{network: "unix", addr: "/tmp/worker.sock"}))
	r3.RemoteAddr = "192.0.2.1:1234"
	if !isLoopbackRequest(r3) {
		t.Fatal("expected unix local addr to pass")
	}
}

func TestAuthenticateInternalProxyRequest(t *testing.T) {
	cfg := internalOrchestratorProxyConfig{
		AllowedTokens: map[string]string{"tok-a": "svc-a", "lease-a": "svc-lease"},
	}
	r := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	r.Header.Set("Authorization", "Bearer tok-a")
	if _, sid, ok := authenticateInternalProxyRequest(r, cfg); !ok || sid != "svc-a" {
		t.Fatalf("bearer auth mismatch: ok=%v sid=%q", ok, sid)
	}
	r2 := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	r2.Header.Set("X-Cynode-Capability-Lease", "lease-a")
	if _, sid, ok := authenticateInternalProxyRequest(r2, cfg); !ok || sid != "svc-lease" {
		t.Fatalf("lease auth mismatch: ok=%v sid=%q", ok, sid)
	}
	r3 := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	r3.Header.Set("Authorization", "Bearer bad-token")
	if _, _, ok := authenticateInternalProxyRequest(r3, cfg); ok {
		t.Fatal("expected bad bearer token to fail")
	}
}

func TestNewInternalServerAddr(t *testing.T) {
	_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
	srv := newInternalServer(http.NewServeMux())
	if srv.Addr != "127.0.0.1:9191" {
		t.Fatalf("unexpected default internal addr: %q", srv.Addr)
	}
	_ = os.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:9991")
	defer func() { _ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR") }()
	srv2 := newInternalServer(http.NewServeMux())
	if srv2.Addr != "127.0.0.1:9991" {
		t.Fatalf("unexpected overridden internal addr: %q", srv2.Addr)
	}
}
