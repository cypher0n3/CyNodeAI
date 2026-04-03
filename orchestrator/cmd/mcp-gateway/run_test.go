package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
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

// TestOpenGatewayStore_ErrorPaths covers database.Open, RunSchema, and Apply failure branches (test hooks).
func TestOpenGatewayStore_ErrorPaths(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set")
	}
	tests := []struct {
		name string
		arm  func(bool)
	}{
		{"open", func(v bool) { testForceOpenGatewayStoreOpenError = v }},
		{"run_schema", func(v bool) { testForceOpenGatewayStoreRunSchemaError = v }},
		{"apply", func(v bool) { testForceOpenGatewayStoreApplyError = v }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.arm(true)
			defer tt.arm(false)
			_, _, err := openGatewayStore(context.Background(), slog.Default(), config.LoadOrchestratorConfig())
			if err == nil {
				t.Fatal("expected error from failure path")
			}
		})
	}
}

// TestOpenGatewayStore_EmptyDSN covers the warn branch when DATABASE_URL is unset (no test hooks).
func TestOpenGatewayStore_EmptyDSN(t *testing.T) {
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Unsetenv("DATABASE_URL")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	cfg := config.LoadOrchestratorConfig()
	store, closeFn, err := openGatewayStore(context.Background(), slog.Default(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if store != nil || closeFn != nil {
		t.Fatalf("expected nil store and nil closeFn, got store=%v closeFn!=nil %v", store, closeFn != nil)
	}
}

func TestRun_WithTestStore_NoDatabaseURL(t *testing.T) {
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Unsetenv("DATABASE_URL")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	testStore = testutil.NewMockDB()
	defer func() { testStore = nil }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := run(ctx, slog.Default())
	if err != nil {
		t.Errorf("run: %v", err)
	}
}

func TestRun_CanceledContext(t *testing.T) {
	// Ensure no real DB so run() uses nil store and exits on canceled ctx without hitting Open.
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

func TestRun_ShutdownHookReturnsError(t *testing.T) {
	testShutdownHook = func(*http.Server, context.Context) error {
		return errors.New("shutdown failed")
	}
	defer func() { testShutdownHook = nil }()
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
	_ = os.Setenv("LISTEN_ADDR", ":19084")
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
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected shutdown error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not return")
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
	_ = os.Setenv("MCP_GATEWAY_SHUTDOWN_SEC", "0")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("zero should use default: got %v", shutdownTimeout())
	}
	_ = os.Setenv("MCP_GATEWAY_SHUTDOWN_SEC", "-1")
	if shutdownTimeout() != 10*time.Second {
		t.Errorf("negative should use default: got %v", shutdownTimeout())
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

func TestRunMain_WhenRunFails_ReturnsOne(t *testing.T) {
	testDatabaseOpen = func(_ context.Context, _ string) (database.Store, error) {
		return nil, errors.New("open failed")
	}
	defer func() { testDatabaseOpen = nil }()
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Setenv("DATABASE_URL", "postgres://local/test")
	defer func() {
		if oldDSN == "" {
			_ = os.Unsetenv("DATABASE_URL")
		} else {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		}
	}()
	if rc := runMain(context.Background()); rc != 1 {
		t.Errorf("runMain: want exit 1, got %d", rc)
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

// TestRun_TestDatabaseOpenReturnsError covers run() when testDatabaseOpen is set but returns an error.
func TestRun_TestDatabaseOpenReturnsError(t *testing.T) {
	testDatabaseOpen = func(_ context.Context, _ string) (database.Store, error) {
		return nil, errors.New("open failed")
	}
	defer func() { testDatabaseOpen = nil }()
	_ = os.Setenv("DATABASE_URL", "postgres://local/test")
	defer func() { _ = os.Unsetenv("DATABASE_URL") }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := run(ctx, slog.Default())
	if err == nil {
		t.Error("expected error when testDatabaseOpen fails")
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
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("run: %v", err)
	}
}

// TestRun_WithTestStore starts run() with testStore set and POSTs to the tool-call endpoint to cover the store path.
func TestRun_WithTestStore(t *testing.T) {
	// Clear agent bearer env so standalone gateway matches unit tests (no auth required). CI/dev shells often export these from .env.dev.
	oldPM := os.Getenv("WORKER_INTERNAL_AGENT_TOKEN")
	oldSand := os.Getenv("MCP_SANDBOX_AGENT_BEARER_TOKEN")
	oldPA := os.Getenv("MCP_PA_AGENT_BEARER_TOKEN")
	_ = os.Unsetenv("WORKER_INTERNAL_AGENT_TOKEN")
	_ = os.Unsetenv("MCP_SANDBOX_AGENT_BEARER_TOKEN")
	_ = os.Unsetenv("MCP_PA_AGENT_BEARER_TOKEN")
	defer func() {
		restore := func(k, v string) {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
		restore("WORKER_INTERNAL_AGENT_TOKEN", oldPM)
		restore("MCP_SANDBOX_AGENT_BEARER_TOKEN", oldSand)
		restore("MCP_PA_AGENT_BEARER_TOKEN", oldPA)
	}()

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
	// Use a non-routed tool so gateway returns 501.
	resp, err := http.Post("http://127.0.0.1:19083/v1/mcp/tools/call", "application/json", bytes.NewReader([]byte(`{"tool_name":"other.tool"}`)))
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
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("run: %v", err)
	}
}

// TestRun_ListenAndServeInvalidPort covers the serverErr branch when ListenAndServe fails (invalid port).
func TestRun_ListenAndServeInvalidPort(t *testing.T) {
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":99999")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Unsetenv("DATABASE_URL")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	testStore = testutil.NewMockDB()
	defer func() { testStore = nil }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := run(ctx, slog.Default())
	if err == nil {
		t.Error("expected error from invalid listen address")
	}
}

func TestRun_RejectDefaultSecretsWhenDevModeFalse(t *testing.T) {
	oldDev := os.Getenv("ORCHESTRATOR_DEV_MODE")
	_ = os.Setenv("ORCHESTRATOR_DEV_MODE", "false")
	defer func() {
		if oldDev != "" {
			_ = os.Setenv("ORCHESTRATOR_DEV_MODE", oldDev)
		} else {
			_ = os.Unsetenv("ORCHESTRATOR_DEV_MODE")
		}
	}()
	_ = os.Unsetenv("JWT_SECRET")
	_ = os.Unsetenv("NODE_REGISTRATION_PSK")
	_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
	_ = os.Unsetenv("BOOTSTRAP_ADMIN_PASSWORD")
	err := run(context.Background(), slog.Default())
	if err == nil {
		t.Fatal("expected validation error when ORCHESTRATOR_DEV_MODE=false and defaults")
	}
}
