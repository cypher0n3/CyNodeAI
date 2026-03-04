package main

import (
	"bytes"
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
	defer func() { _ = os.Unsetenv("WORKER_API_BEARER_TOKEN") }()

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
