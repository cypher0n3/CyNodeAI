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

