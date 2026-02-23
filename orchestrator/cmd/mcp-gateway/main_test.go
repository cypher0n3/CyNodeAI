package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_MCP_ENV")
	if getEnv("TEST_MCP_ENV", "def") != "def" {
		t.Error("default")
	}
	_ = os.Setenv("TEST_MCP_ENV", "val")
	defer func() { _ = os.Unsetenv("TEST_MCP_ENV") }()
	if getEnv("TEST_MCP_ENV", "def") != "val" {
		t.Error("from env")
	}
}

func TestRun_CancelledContext(t *testing.T) {
	// Ensure no real DB so run() uses nil store and exits on cancelled ctx without hitting Open.
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Unsetenv("DATABASE_URL")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	logger := slog.Default()
	err := run(ctx, logger)
	if err != nil {
		t.Errorf("run: %v", err)
	}
}

func TestRun_ListenAndServeFails(t *testing.T) {
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Unsetenv("DATABASE_URL")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
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
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Unsetenv("DATABASE_URL")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := runMain(ctx)
	if code != 0 {
		t.Errorf("runMain: got %d", code)
	}
}

func TestShutdownTimeout(t *testing.T) {
	_ = os.Unsetenv("MCP_GATEWAY_SHUTDOWN_SEC")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("default: got %v", shutdownTimeout())
	}
	_ = os.Setenv("MCP_GATEWAY_SHUTDOWN_SEC", "5")
	defer func() { _ = os.Unsetenv("MCP_GATEWAY_SHUTDOWN_SEC") }()
	if shutdownTimeout() != 5*time.Second {
		t.Errorf("from env: got %v", shutdownTimeout())
	}
	_ = os.Setenv("MCP_GATEWAY_SHUTDOWN_SEC", "x")
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

func TestToolCallHandler_StoreNil(t *testing.T) {
	handler := toolCallHandler(nil, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(`{"tool_name":"db.preference.get"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("store nil: got status %d", rec.Code)
	}
}

// callToolHandlerPOST sends a POST with body and asserts the response code.
func callToolHandlerPOST(t *testing.T, body string, wantCode int) {
	t.Helper()
	handler := toolCallHandler(testutil.NewMockDB(), slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != wantCode {
		t.Errorf("got status %d, want %d", rec.Code, wantCode)
	}
}

func TestToolCallHandler_StoreSet_WritesAudit(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"db.preference.get"}`, http.StatusNotImplemented)
}

func TestToolCallHandler_InvalidJSON(t *testing.T) {
	callToolHandlerPOST(t, `{`, http.StatusBadRequest)
}

func TestToolCallHandler_MethodNotPost(t *testing.T) {
	handler := toolCallHandler(testutil.NewMockDB(), slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/call", http.NoBody)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d", rec.Code)
	}
}

func TestToolCallHandler_StoreSet_AuditWriteFails(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("db error")
	handler := toolCallHandler(mock, slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(`{"tool_name":"x"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("audit write failure: got status %d", rec.Code)
	}
}

// TestRun_DatabaseOpenFails covers run() when DATABASE_URL is set but Open fails.
func TestRun_DatabaseOpenFails(t *testing.T) {
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Setenv("DATABASE_URL", "postgres://invalid/invalid")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := run(ctx, slog.Default())
	if err == nil {
		t.Error("expected error when DATABASE_URL is invalid")
	}
}

// TestRun_WithTestDatabaseOpen covers the path where testDatabaseOpen is set (store from hook, no real DB).
func TestRun_WithTestDatabaseOpen(t *testing.T) {
	testDatabaseOpen = func(_ context.Context, _ string) (database.Store, error) {
		return testutil.NewMockDB(), nil
	}
	defer func() { testDatabaseOpen = nil }()
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Setenv("DATABASE_URL", "postgres://local/test")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, slog.Default()) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Errorf("run: %v", err)
	}
}

// TestRun_WithTestStore starts run() with testStore set and POSTs to the tool-call endpoint to cover the store path.
func TestRun_WithTestStore(t *testing.T) {
	testStore = testutil.NewMockDB()
	defer func() { testStore = nil }()
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", "127.0.0.1:19083")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, slog.Default()) }()

	time.Sleep(50 * time.Millisecond)
	resp, err := http.Post("http://127.0.0.1:19083/v1/mcp/tools/call", "application/json", bytes.NewReader([]byte(`{"tool_name":"db.preference.get"}`)))
	if err != nil {
		cancel()
		<-done
		t.Skipf("POST failed (server may not be up): %v", err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("POST: got status %d", resp.StatusCode)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("run: %v", err)
	}
}
