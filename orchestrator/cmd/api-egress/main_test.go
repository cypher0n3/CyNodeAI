package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_AE_ENV")
	if getEnv("TEST_AE_ENV", "def") != "def" {
		t.Error("default")
	}
	_ = os.Setenv("TEST_AE_ENV", "val")
	defer func() { _ = os.Unsetenv("TEST_AE_ENV") }()
	if getEnv("TEST_AE_ENV", "def") != "val" {
		t.Error("from env")
	}
}

func TestRun_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	logger := slog.Default()
	err := run(ctx, logger)
	if err != nil {
		t.Errorf("run: %v", err)
	}
}

func TestRun_ListenAndServeFails(t *testing.T) {
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":99999")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := slog.Default()
	err := run(ctx, logger)
	if err == nil {
		t.Error("expected error when ListenAndServe fails (invalid port)")
	}
}

func TestRunMain_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := runMain(ctx)
	if code != 0 {
		t.Errorf("runMain: got %d", code)
	}
}

func TestShutdownTimeout(t *testing.T) {
	_ = os.Unsetenv("API_EGRESS_SHUTDOWN_SEC")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("default: got %v", shutdownTimeout())
	}
	_ = os.Setenv("API_EGRESS_SHUTDOWN_SEC", "5")
	defer func() { _ = os.Unsetenv("API_EGRESS_SHUTDOWN_SEC") }()
	if shutdownTimeout() != 5*time.Second {
		t.Errorf("from env: got %v", shutdownTimeout())
	}
	_ = os.Setenv("API_EGRESS_SHUTDOWN_SEC", "x")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("invalid env should use default: got %v", shutdownTimeout())
	}
}

func TestRunMain_RunFails(t *testing.T) {
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":99999")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	code := runMain(ctx)
	if code != 1 {
		t.Errorf("runMain when run fails: got %d", code)
	}
}

func TestCallHandler_MethodNotAllowed(t *testing.T) {
	h := newCallHandler(slog.Default(), "secret", "openai,github")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/call", http.NoBody)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: got %d", w.Code)
	}
}

func TestCallHandler_Unauthorized(t *testing.T) {
	h := newCallHandler(slog.Default(), "secret", "openai,github")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader([]byte(`{"provider":"openai","operation":"chat"}`)))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no bearer: got %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader([]byte(`{"provider":"openai","operation":"chat"}`)))
	r2.Header.Set("Content-Type", "application/json")
	r2.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(w2, r2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("wrong bearer: got %d", w2.Code)
	}
}

func TestCallHandler_BadRequest(t *testing.T) {
	h := newCallHandler(slog.Default(), "secret", "openai,github")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON: got %d", w.Code)
	}
}

func TestCallHandler_Forbidden(t *testing.T) {
	h := newCallHandler(slog.Default(), "secret", "openai,github")
	body := map[string]string{"provider": "unknown", "operation": "op", "task_id": "t1"}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(mustJSON(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("disallowed provider: got %d", w.Code)
	}
}

func TestCallHandler_AllowedReturns501(t *testing.T) {
	h := newCallHandler(slog.Default(), "secret", "openai,github")
	body := map[string]string{"provider": "openai", "operation": "chat", "task_id": "t2"}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/call", bytes.NewReader(mustJSON(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("allowed provider: got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["title"] != "Not Implemented" {
		t.Errorf("title: %v", resp["title"])
	}
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
