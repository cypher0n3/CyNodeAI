package main

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_UG_ENV")
	if getEnv("TEST_UG_ENV", "def") != "def" {
		t.Error("default")
	}
	_ = os.Setenv("TEST_UG_ENV", "val")
	defer func() { _ = os.Unsetenv("TEST_UG_ENV") }()
	if getEnv("TEST_UG_ENV", "def") != "val" {
		t.Error("from env")
	}
}

func TestLimitBody(t *testing.T) {
	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}
	wrapped := limitBody(100, next)
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("small"))
	rec := httptest.NewRecorder()
	wrapped(rec, req)
	if !called {
		t.Error("next handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d", rec.Code)
	}
}

func TestRun_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	err := run(ctx, cfg, mockDB, logger)
	if err != nil {
		t.Errorf("run with cancelled context: %v", err)
	}
}

func TestRun_StartAndShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	go func() {
		_ = run(ctx, cfg, mockDB, logger)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
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
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	err := run(ctx, cfg, mockDB, logger)
	if err == nil {
		t.Error("expected error when ListenAndServe fails (invalid port)")
	}
}

func TestRunMain_Success(t *testing.T) {
	pgTest := os.Getenv("POSTGRES_TEST_DSN")
	if pgTest == "" {
		t.Skip("POSTGRES_TEST_DSN not set; need DB for runMain success path")
	}
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Setenv("DATABASE_URL", pgTest)
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { done <- runMain(ctx) }()
	time.Sleep(80 * time.Millisecond)
	cancel()
	time.Sleep(150 * time.Millisecond)
	code := <-done
	if code != 0 {
		t.Errorf("runMain success path: got %d", code)
	}
}

func TestRunMain_DBOpenFails(t *testing.T) {
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Setenv("DATABASE_URL", "postgres://invalid:invalid@127.0.0.1:1/none?sslmode=disable")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	code := runMain(context.Background())
	if code != 1 {
		t.Errorf("runMain with bad DB: got %d", code)
	}
}

func TestRunMain_RunFails(t *testing.T) {
	if pg := os.Getenv("POSTGRES_TEST_DSN"); pg != "" {
		oldDSN := os.Getenv("DATABASE_URL")
		_ = os.Setenv("DATABASE_URL", pg)
		defer func() {
			if oldDSN != "" {
				_ = os.Setenv("DATABASE_URL", oldDSN)
			} else {
				_ = os.Unsetenv("DATABASE_URL")
			}
		}()
	}
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

