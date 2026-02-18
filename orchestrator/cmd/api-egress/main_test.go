package main

import (
	"context"
	"log/slog"
	"os"
	"testing"
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
