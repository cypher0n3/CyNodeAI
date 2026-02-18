package main

import (
	"context"
	"log/slog"
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
