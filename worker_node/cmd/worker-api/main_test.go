package main

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
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/cmd/worker-api/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
)

const (
	testAddrLoopback   = "127.0.0.1:12345"
	testBodyProxyPostX = `{"version":1,"method":"POST","path":"/x"}`
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

func TestServerAddrFromEnv(t *testing.T) {
	check := func(t *testing.T, envKey, envValue, wantDefault, wantOverride string, mk func() *http.Server) {
		t.Helper()
		_ = os.Unsetenv(envKey)
		srv := mk()
		if srv.Addr != wantDefault {
			t.Fatalf("default addr=%q want %q", srv.Addr, wantDefault)
		}
		_ = os.Setenv(envKey, envValue)
		defer func() { _ = os.Unsetenv(envKey) }()
		srv2 := mk()
		if srv2.Addr != wantOverride {
			t.Fatalf("env addr=%q want %q", srv2.Addr, wantOverride)
		}
	}

	t.Run("public server LISTEN_ADDR", func(t *testing.T) {
		check(t, "LISTEN_ADDR", ":9999", ":9190", ":9999", func() *http.Server { return newServer(http.NewServeMux()) })
	})
	t.Run("internal server WORKER_INTERNAL_LISTEN_ADDR", func(t *testing.T) {
		check(t, "WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:9991", "127.0.0.1:9191", "127.0.0.1:9991", func() *http.Server {
			return newInternalServer(http.NewServeMux())
		})
	})
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
	t.Run("success with empty workspace root", func(t *testing.T) {
		muxEmptyRoot := newMux(executor.New("direct", 5*time.Second, 1024, "", "", nil), "test-bearer", "", nil, slog.Default())
		postRunJobSuccess(t, muxEmptyRoot, body)
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

func TestRunMainWithContextCancel_SecureStoreAvailable(t *testing.T) {
	_ = os.Setenv("WORKER_API_BEARER_TOKEN", "test-token")
	_ = os.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	_ = os.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "")
	_ = os.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	_ = os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	defer func() {
		_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		_ = os.Unsetenv("CYNODE_SECURE_STORE_MASTER_KEY_B64")
	}()

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

func TestPrepareWorkspace(t *testing.T) {
	dir, cleanup, err := prepareWorkspace("", "job-1")
	if err != nil || dir != "" || cleanup != nil {
		t.Fatalf("empty root: dir=%q err=%v", dir, err)
	}
	root := t.TempDir()
	dir, cleanup, err = prepareWorkspace(root, "job-1")
	if err != nil || dir == "" || cleanup == nil {
		t.Fatalf("normal: dir=%q err=%v", dir, err)
	}
	defer cleanup()
	if !strings.Contains(dir, "job-1") {
		t.Errorf("dir should contain job id: %s", dir)
	}
}

func TestRecordNodeBoot_InsertFails(t *testing.T) {
	ctx := context.Background()
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = store.Close()
	logger := slog.Default()
	recordNodeBoot(ctx, store, logger)
	recordNodeBoot(ctx, store, nil)
}

func TestListenInternalUnix_Success(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "internal.sock")
	srv := newInternalServer(newInternalMux(internalOrchestratorProxyConfig{}, slog.Default()))
	serverErr := make(chan error, 1)
	cleanup, code := listenInternalUnix(socketPath, srv, serverErr, slog.Default())
	if code != 0 || cleanup == nil {
		t.Fatalf("listenInternalUnix: code=%d", code)
	}
	cleanup()
}

func TestForwardManagedProxyRequest_TimeoutClamped(t *testing.T) {
	t.Setenv("WORKER_MANAGED_PROXY_UPSTREAM_TIMEOUT_SEC", "0")
	req := &managedServiceProxyRequest{Version: 1, Method: http.MethodGet, Path: "/x"}
	_, status, _ := forwardManagedProxyRequest(
		context.Background(),
		managedServiceTarget{ServiceType: "pma", BaseURL: "http://127.0.0.1:1"},
		req, nil,
	)
	if status != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", status)
	}
}

func TestDecodeManagedProxyRequest_RequestBodyTooLarge(t *testing.T) {
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"x":{"service_type":"pma","base_url":"http://127.0.0.1:1"}}`)
	mux := newMux(executor.New("direct", time.Second, 1024, "", "", nil), "token", "", nil, slog.Default())
	bigBody := bytes.Repeat([]byte("x"), maxManagedProxyBodyBytes+1)
	body := []byte(`{"version":1,"method":"POST","path":"/y","body_b64":"` + base64.StdEncoding.EncodeToString(bigBody) + `"}`)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/managed-services/x/proxy:http", bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d %s", w.Code, w.Body.String())
	}
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

// TestForwardManagedProxyRequest_UnixSocket asserts that forwardManagedProxyRequest
// can reach a target via http+unix:// (UDS transport) - required for PMA with --network=none.
func TestForwardManagedProxyRequest_UnixSocket(t *testing.T) {
	sockDir := t.TempDir()
	sockPath := filepath.Join(sockDir, "service.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	udsURL := "http+unix://" + url.PathEscape(sockPath)
	req := &managedServiceProxyRequest{
		Version: 1,
		Method:  http.MethodGet,
		Path:    "/healthz",
	}
	resp, status, detail := forwardManagedProxyRequest(
		context.Background(),
		managedServiceTarget{ServiceType: "pma", BaseURL: udsURL},
		req,
		nil,
	)
	if status != 0 {
		t.Fatalf("expected success, got status=%d detail=%q", status, detail)
	}
	if resp == nil || resp.Status != http.StatusOK {
		t.Fatalf("expected upstream 200, got %+v", resp)
	}
}

func TestLoadWorkerProxyConfig_FromNodeConfig(t *testing.T) {
	t.Setenv("WORKER_API_STATE_DIR", "/tmp/cynode-state")
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
						AgentTokenRef: &nodepayloads.ConfigManagedServiceAgentTokenRef{
							Kind: "orchestrator_endpoint",
							URL:  "http://orchestrator.internal:12082/v1/agent-tokens/pma-main",
						},
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
	if got := out.InternalProxy.SocketByService["pma-main"]; got != "/tmp/cynode-state/run/managed_agent_proxy/pma-main/proxy.sock" {
		t.Fatalf("unexpected socket path for pma-main: %q", got)
	}
}

func TestLoadWorkerProxyConfig_FallbackFromEnv(t *testing.T) {
	t.Setenv("WORKER_NODE_CONFIG_JSON", "")
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"pma-main":{"service_type":"pma","base_url":"http://127.0.0.1:8090"}}`)
	t.Setenv("ORCHESTRATOR_URL", "http://orchestrator:12082")
	out := loadWorkerProxyConfig(slog.Default())
	if out.ManagedServiceTargets["pma-main"].BaseURL != "http://127.0.0.1:8090" {
		t.Fatalf("managed targets fallback failed: %+v", out.ManagedServiceTargets)
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
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	stateDir := t.TempDir()
	store, _, err := securestore.Open(stateDir)
	if err != nil {
		t.Fatalf("securestore.Open failed: %v", err)
	}
	if err := store.PutAgentToken("pma-main", "agent-token", ""); err != nil {
		t.Fatalf("PutAgentToken failed: %v", err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp/call" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer agent-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	cfg := internalOrchestratorProxyConfig{
		UpstreamBaseURL: upstream.URL,
		SecureStore:     store,
	}
	mux := withCallerServiceID(newInternalMux(cfg, slog.Default()), "pma-main")
	body := `{"version":1,"method":"POST","path":"/mcp/call"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", bytes.NewReader([]byte(body)))
	r.RemoteAddr = testAddrLoopback
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestInternalOrchestratorProxy_AuthAndLoopback(t *testing.T) {
	cfg := internalOrchestratorProxyConfig{
		UpstreamBaseURL: "http://127.0.0.1:1",
	}
	mux := newInternalMux(cfg, slog.Default())
	body := testBodyProxyPostX

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/agent:ready", bytes.NewReader([]byte(body)))
	r.RemoteAddr = "192.0.2.10:12345"
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-loopback, got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/agent:ready", bytes.NewReader([]byte(body)))
	r2.RemoteAddr = testAddrLoopback
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing identity binding, got %d", w2.Code)
	}
}

func TestInternalOrchestratorProxy_SecureStoreUnavailable(t *testing.T) {
	cfg := internalOrchestratorProxyConfig{
		UpstreamBaseURL: "http://127.0.0.1:1",
		SecureStore:     nil,
	}
	mux := withCallerServiceID(newInternalMux(cfg, slog.Default()), "svc")
	body := testBodyProxyPostX
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", bytes.NewReader([]byte(body)))
	r.RemoteAddr = testAddrLoopback
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when secure store unavailable, got %d", w.Code)
	}
}

func TestInternalOrchestratorProxy_TokenMissingForIdentity(t *testing.T) {
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	stateDir := t.TempDir()
	store, _, err := securestore.Open(stateDir)
	if err != nil {
		t.Fatalf("securestore.Open failed: %v", err)
	}
	cfg := internalOrchestratorProxyConfig{
		UpstreamBaseURL: "http://127.0.0.1:1",
		SecureStore:     store,
	}
	mux := withCallerServiceID(newInternalMux(cfg, slog.Default()), "svc-missing")
	body := testBodyProxyPostX
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", bytes.NewReader([]byte(body)))
	r.RemoteAddr = testAddrLoopback
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when token missing for identity, got %d", w.Code)
	}
}

func TestInternalOrchestratorProxy_UpstreamMissing(t *testing.T) {
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	stateDir := t.TempDir()
	store, _, err := securestore.Open(stateDir)
	if err != nil {
		t.Fatalf("securestore.Open failed: %v", err)
	}
	if err := store.PutAgentToken("svc", "agent-token", ""); err != nil {
		t.Fatalf("PutAgentToken failed: %v", err)
	}
	cfg := internalOrchestratorProxyConfig{
		SecureStore: store,
	}
	mux := withCallerServiceID(newInternalMux(cfg, slog.Default()), "svc")
	body := testBodyProxyPostX
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/worker/internal/orchestrator/mcp:call", bytes.NewReader([]byte(body)))
	r.RemoteAddr = testAddrLoopback
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

func TestManagedAgentProxySocketPath(t *testing.T) {
	path, ok := managedAgentProxySocketPath("/tmp/cynode-state", "pma-main")
	if !ok {
		t.Fatal("expected valid service_id to build socket path")
	}
	if path != "/tmp/cynode-state/run/managed_agent_proxy/pma-main/proxy.sock" {
		t.Fatalf("unexpected socket path: %q", path)
	}
	if _, ok := managedAgentProxySocketPath("/tmp/cynode-state", ""); ok {
		t.Fatal("expected empty service_id to be rejected")
	}
	if _, ok := managedAgentProxySocketPath("/tmp/cynode-state", "../escape"); ok {
		t.Fatal("expected path traversal service_id to be rejected")
	}
}

func TestWithCallerServiceIDAndExtraction(t *testing.T) {
	var gotServiceID string
	h := withCallerServiceID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if sid, ok := callerServiceIDFromRequest(r); ok {
			gotServiceID = sid
		}
	}), "svc-x")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	h.ServeHTTP(w, r)
	if gotServiceID != "svc-x" {
		t.Fatalf("expected caller service id svc-x, got %q", gotServiceID)
	}
	if _, ok := callerServiceIDFromRequest(httptest.NewRequest(http.MethodGet, "/", http.NoBody)); ok {
		t.Fatal("expected no caller service id in bare request")
	}
}

func TestLogsHelpers(t *testing.T) {
	if got := parseLogsLimit(""); got != 1000 {
		t.Fatalf("expected default limit, got %d", got)
	}
	if got := parseLogsLimit("10"); got != 10 {
		t.Fatalf("expected parsed limit 10, got %d", got)
	}
	if got := parseLogsLimit("999999"); got != 1000 {
		t.Fatalf("expected clamped default for oversized limit, got %d", got)
	}
	if msg := validateLogsQuery("", ""); msg == "" {
		t.Fatal("expected validation error for empty source query")
	}
	if msg := validateLogsQuery("service", ""); msg != "" {
		t.Fatalf("expected empty validation error for valid query, got %q", msg)
	}
}

func TestNodeInfoAndStatsHandlers(t *testing.T) {
	t.Setenv("NODE_SLUG", "test-node")
	assertNodeInfoResponse(t, serveHandler(handleNodeInfo(nil), "/v1/worker/telemetry/node:info"))
	assertNodeStatsResponse(t, serveHandler(handleNodeStats(nil), "/v1/worker/telemetry/node:stats"))
}

func serveHandler(h http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, http.NoBody))
	return w
}

func assertNodeInfoResponse(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("node info status %d", w.Code)
	}
	var infoBody map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &infoBody); err != nil {
		t.Fatalf("node info body not JSON: %v", err)
	}
	if infoBody["version"] != float64(1) {
		t.Errorf("node info version != 1: %v", infoBody["version"])
	}
	if infoBody["node_slug"] == "" {
		t.Error("node info node_slug empty")
	}
	platform, _ := infoBody["platform"].(map[string]interface{})
	if platform == nil || platform["os"] == "" || platform["arch"] == "" {
		t.Errorf("node info platform missing os/arch: %v", infoBody)
	}
	if kv, _ := platform["kernel_version"].(string); strings.TrimSpace(kv) == "" {
		t.Errorf("node info kernel_version must be non-empty, got %q", kv)
	}
}

func assertNodeStatsResponse(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("node stats status %d", w.Code)
	}
	var statsBody map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &statsBody); err != nil {
		t.Fatalf("node stats body not JSON: %v", err)
	}
	if statsBody["version"] != float64(1) {
		t.Errorf("node stats version != 1: %v", statsBody["version"])
	}
	if statsBody["captured_at"] == "" {
		t.Error("node stats captured_at empty")
	}
	cpu, _ := statsBody["cpu"].(map[string]interface{})
	if cpu == nil || cpu["cores"].(float64) <= 0 {
		t.Errorf("node stats cpu.cores must be > 0: %v", statsBody)
	}
	mem, _ := statsBody["memory"].(map[string]interface{})
	if mem == nil || mem["total_mb"].(float64) <= 0 {
		t.Errorf("node stats memory.total_mb must be > 0: %v", statsBody)
	}
}

func TestKernelVersion(t *testing.T) {
	v := kernelVersion()
	if strings.TrimSpace(v) == "" {
		t.Error("kernelVersion() returned empty string")
	}
}

func TestMemoryStatsMB(t *testing.T) {
	total, used, free := memoryStatsMB()
	if total <= 0 {
		t.Errorf("memoryStatsMB total_mb must be > 0, got %d", total)
	}
	if used < 0 || free < 0 {
		t.Errorf("memoryStatsMB returned negative values: used=%d free=%d", used, free)
	}
}

func TestDiskStatsMB(t *testing.T) {
	total, free := diskStatsMB(".")
	if total <= 0 {
		t.Errorf("diskStatsMB total must be > 0, got %d", total)
	}
	if free < 0 {
		t.Errorf("diskStatsMB free must be >= 0, got %d", free)
	}
}

func TestContainerRuntimeInfo_Stubbed(t *testing.T) {
	old := execCmdFn
	execCmdFn = func(name string, args ...string) (string, error) {
		return "podman version 5.8.0", nil
	}
	defer func() { execCmdFn = old }()
	t.Setenv("CONTAINER_RUNTIME", "podman")
	name, ver := containerRuntimeInfo()
	if name != "podman" {
		t.Errorf("name = %q, want podman", name)
	}
	if ver != "5.8.0" {
		t.Errorf("ver = %q, want 5.8.0", ver)
	}
}

func TestContainerRuntimeInfo_UnknownFallback(t *testing.T) {
	old := execCmdFn
	execCmdFn = func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("not found")
	}
	defer func() { execCmdFn = old }()
	t.Setenv("CONTAINER_RUNTIME", "docker")
	name, ver := containerRuntimeInfo()
	if name != "docker" {
		t.Errorf("name = %q, want docker", name)
	}
	if ver != "unknown" {
		t.Errorf("ver = %q, want unknown", ver)
	}
}

func TestRunMain_WithPerServiceUnixSockets(t *testing.T) {
	t.Setenv("WORKER_API_BEARER_TOKEN", "test-token")
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "")
	t.Setenv("WORKER_INTERNAL_LISTEN_UNIX", "")
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	cfg := nodepayloads.NodeConfigurationPayload{
		Version: 1,
		Orchestrator: nodepayloads.ConfigOrchestrator{
			BaseURL: "http://orchestrator.internal:12082",
		},
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "svc-a",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "tok-a",
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
	raw, _ := json.Marshal(cfg)
	t.Setenv("WORKER_NODE_CONFIG_JSON", string(raw))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- runMain(ctx) }()
	time.Sleep(40 * time.Millisecond)
	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("runMain should return 0 with per-service sockets, got %d", code)
	}
}

func TestLoadWorkerProxyConfig_SocketPathWithoutDirectToken(t *testing.T) {
	t.Setenv("WORKER_API_STATE_DIR", "/tmp/cynode-state")
	cfg := nodepayloads.NodeConfigurationPayload{
		Version: 1,
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "svc-a",
					ServiceType: "pma",
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						MCPGatewayProxyURL: "auto",
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(cfg)
	t.Setenv("WORKER_NODE_CONFIG_JSON", string(raw))
	out := loadWorkerProxyConfig(slog.Default())
	if got := out.InternalProxy.SocketByService["svc-a"]; got == "" {
		t.Fatalf("expected socket path mapping for svc-a, got %+v", out.InternalProxy.SocketByService)
	}
}

func TestRunMain_InvalidPublicListenAddrFails(t *testing.T) {
	t.Setenv("WORKER_API_BEARER_TOKEN", "tok")
	t.Setenv("LISTEN_ADDR", "invalid-listen-addr")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "")
	if code := runMain(context.Background()); code != 0 {
		t.Fatalf("runMain should return cleanly after listen startup error path, got %d", code)
	}
}

// Note: internal server addr coverage is in TestServerAddrFromEnv.

func TestRunDiagnosticFlags_NoMatch(t *testing.T) {
	code, handled := runDiagnosticFlags([]string{"--some-other-flag"})
	if handled {
		t.Error("expected handled=false for unknown flag")
	}
	if code != 0 {
		t.Errorf("code=%d, want 0", code)
	}
}

func TestRunDiagnosticFlags_PrintManagedServiceRunArgs(t *testing.T) {
	t.Setenv("NODE_STATE_DIR", t.TempDir())
	code, handled := runDiagnosticFlags([]string{
		"--print-managed-service-run-args",
		"--service-id", "pma-main",
		"--service-type", "pma",
		"--service-image", "pma:latest",
	})
	if !handled {
		t.Error("expected handled=true")
	}
	if code != 0 {
		t.Errorf("code=%d, want 0", code)
	}
}

func TestRunDiagnosticFlags_PrintManagedServiceRunArgs_MissingArgs(t *testing.T) {
	code, handled := runDiagnosticFlags([]string{"--print-managed-service-run-args"})
	if !handled {
		t.Error("expected handled=true")
	}
	if code != 1 {
		t.Errorf("code=%d, want 1 (missing required args)", code)
	}
}

func TestRunDiagnosticFlags_PrintSBAPodRunArgs(t *testing.T) {
	code, handled := runDiagnosticFlags([]string{
		"--print-sba-pod-run-args",
		"--sba-image", "cynode-sba:dev",
		"--proxy-image", "proxy:dev",
		"--upstream-url", "http://host.containers.internal:11434",
	})
	if !handled {
		t.Error("expected handled=true")
	}
	if code != 0 {
		t.Errorf("code=%d, want 0", code)
	}
}

func TestRunDiagnosticFlags_PrintSBAPodRunArgs_MissingImage(t *testing.T) {
	code, handled := runDiagnosticFlags([]string{"--print-sba-pod-run-args"})
	if !handled {
		t.Error("expected handled=true")
	}
	if code != 1 {
		t.Errorf("code=%d, want 1 (missing sba-image)", code)
	}
}

func TestRunPrintManagedServiceRunArgs_OutputContainsUDS(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Setenv("NODE_STATE_DIR", t.TempDir())
	code := runPrintManagedServiceRunArgs([]string{
		"--service-id", "pma-main",
		"--service-type", "pma",
		"--service-image", "pma:latest",
	})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if code != 0 {
		t.Fatalf("code=%d, want 0", code)
	}
	if !strings.Contains(buf.String(), "http+unix://") {
		t.Errorf("output must contain http+unix:// (REQ-WORKER-0260), got: %s", buf.String())
	}
}

func TestRunPrintSBAPodRunArgs_OutputContainsInferenceProxyURL(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := runPrintSBAPodRunArgs([]string{
		"--sba-image", "cynode-sba:dev",
		"--proxy-image", "proxy:dev",
		"--upstream-url", "http://host.containers.internal:11434",
	})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if code != 0 {
		t.Fatalf("code=%d, want 0", code)
	}
	if !strings.Contains(buf.String(), "INFERENCE_PROXY_URL=http+unix://") {
		t.Errorf("output must contain INFERENCE_PROXY_URL=http+unix:// (REQ-SANDBX-0131), got: %s", buf.String())
	}
}

func TestResolveHostInferenceUpstream_ReplacesContainerAlias(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"http://host.containers.internal:11434", "http://localhost:11434"},
		{"http://localhost:11434", "http://localhost:11434"},
		{"http://192.168.1.1:11434", "http://192.168.1.1:11434"},
		{"http://host.containers.internal:11434/api", "http://localhost:11434/api"},
	}
	for _, c := range cases {
		got := resolveHostInferenceUpstream(c.input)
		if got != c.want {
			t.Errorf("resolveHostInferenceUpstream(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestStartManagedServiceInferenceProxies_CreatesSockFile(t *testing.T) {
	// Use os.MkdirTemp with a short prefix to stay within Unix 108-char socket path limit.
	dir, err := os.MkdirTemp("", "wapi-inf")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	targets := map[string]managedServiceTarget{
		"pma-main": {ServiceType: "pma", BaseURL: "http+unix://whatever"},
	}
	startManagedServiceInferenceProxies(ctx, dir, targets, slog.Default())
	sockPath := filepath.Join(dir, internalProxySocketBaseDir, "pma-main", "inference.sock")
	// Poll up to 500ms for the goroutine to bind the socket.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(sockPath); err != nil {
		t.Errorf("inference.sock not created at %s within 500ms: %v", sockPath, err)
	}
}

func TestStartManagedServiceInferenceProxies_SkipsNonPMA(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	targets := map[string]managedServiceTarget{
		"other-svc": {ServiceType: "sba", BaseURL: "http+unix://whatever"},
	}
	startManagedServiceInferenceProxies(ctx, dir, targets, slog.Default())
	sockPath := filepath.Join(dir, internalProxySocketBaseDir, "other-svc", "inference.sock")
	if _, err := os.Stat(sockPath); err == nil {
		t.Error("inference.sock should NOT be created for non-PMA service types")
	}
}
