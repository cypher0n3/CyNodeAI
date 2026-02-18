package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/cmd/worker-api/executor"
)

func TestGetEnv(t *testing.T) {
	os.Unsetenv("TEST_WORKER_GETENV")
	if getEnv("TEST_WORKER_GETENV", "def") != "def" {
		t.Error("getEnv default")
	}
	os.Setenv("TEST_WORKER_GETENV", "val")
	defer os.Unsetenv("TEST_WORKER_GETENV")
	if getEnv("TEST_WORKER_GETENV", "def") != "val" {
		t.Error("getEnv from env")
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Unsetenv("TEST_WORKER_INT")
	if getEnvInt("TEST_WORKER_INT", 42) != 42 {
		t.Error("getEnvInt default")
	}
	os.Setenv("TEST_WORKER_INT", "99")
	defer os.Unsetenv("TEST_WORKER_INT")
	if getEnvInt("TEST_WORKER_INT", 0) != 99 {
		t.Error("getEnvInt from env")
	}
	os.Setenv("TEST_WORKER_INT", "bad")
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
			r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", nil)
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
	os.Unsetenv("LISTEN_ADDR")
	srv := newServer(http.NewServeMux())
	if srv.Addr != ":8081" {
		t.Errorf("default addr: %s", srv.Addr)
	}
	os.Setenv("LISTEN_ADDR", ":9999")
	defer os.Unsetenv("LISTEN_ADDR")
	srv2 := newServer(http.NewServeMux())
	if srv2.Addr != ":9999" {
		t.Errorf("env addr: %s", srv2.Addr)
	}
}

func TestHandleRunJob(t *testing.T) {
	exec := executor.New("direct", 5*time.Second, 1024)
	token := "test-bearer"
	mux := newMux(exec, token, nil)

	var cmd []string
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/c", "echo", "ok"}
	} else {
		cmd = []string{"echo", "ok"}
	}
	reqBody := workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "task-1",
		JobID:   "job-1",
		Sandbox: workerapi.SandboxSpec{
			Command: cmd,
		},
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

	t.Run("wrong version", func(t *testing.T) {
		badReq := workerapi.RunJobRequest{Version: 2, TaskID: "t", JobID: "j", Sandbox: workerapi.SandboxSpec{Command: cmd}}
		body2, _ := json.Marshal(badReq)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body2))
		r.Header.Set("Authorization", "Bearer test-bearer")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status %d", w.Code)
		}
	})

	t.Run("missing task_id job_id", func(t *testing.T) {
		badReq := workerapi.RunJobRequest{Version: 1, TaskID: "", JobID: "", Sandbox: workerapi.SandboxSpec{Command: cmd}}
		body2, _ := json.Marshal(badReq)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body2))
		r.Header.Set("Authorization", "Bearer test-bearer")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status %d", w.Code)
		}
	})

	t.Run("empty command", func(t *testing.T) {
		badReq := workerapi.RunJobRequest{Version: 1, TaskID: "t", JobID: "j", Sandbox: workerapi.SandboxSpec{Command: nil}}
		body2, _ := json.Marshal(badReq)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/v1/worker/jobs:run", bytes.NewReader(body2))
		r.Header.Set("Authorization", "Bearer test-bearer")
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status %d", w.Code)
		}
	})
}

func TestHealthz(t *testing.T) {
	mux := newMux(executor.New("direct", time.Second, 1024), "token", nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Errorf("healthz: %d %s", w.Code, w.Body.String())
	}
}

func TestRunMainMissingToken(t *testing.T) {
	os.Unsetenv("WORKER_API_BEARER_TOKEN")
	defer os.Unsetenv("WORKER_API_BEARER_TOKEN")
	code := runMain(context.Background())
	if code != 1 {
		t.Errorf("runMain should return 1 when token unset, got %d", code)
	}
}

func TestRunMainWithContextCancel(t *testing.T) {
	os.Setenv("WORKER_API_BEARER_TOKEN", "test-token")
	defer os.Unsetenv("WORKER_API_BEARER_TOKEN")

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
