package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

// TestWaitForInferencePath_CancelDuringErrorBackoff verifies ctx cancel ends the poll sleep after InferencePathAvailable errors.
func TestWaitForInferencePath_CancelDuringErrorBackoff(t *testing.T) {
	testPMAPollInterval = 500 * time.Millisecond
	defer func() { testPMAPollInterval = 0 }()
	ctx, cancel := context.WithCancel(context.Background())
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("list failed")
	defer func() { mockDB.ForceError = nil }()

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	got := waitForInferencePath(ctx, mockDB, nil)
	if got {
		t.Error("expected false")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("cancel during error backoff should return quickly, took %v", time.Since(start))
	}
}

// TestWaitForInferencePath_CancelDuringNotReadyBackoff verifies ctx cancel ends the poll sleep when no inference path yet.
func TestWaitForInferencePath_CancelDuringNotReadyBackoff(t *testing.T) {
	testPMAPollInterval = 500 * time.Millisecond
	defer func() { testPMAPollInterval = 0 }()
	ctx, cancel := context.WithCancel(context.Background())
	mockDB := testutil.NewMockDB()

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	got := waitForInferencePath(ctx, mockDB, nil)
	if got {
		t.Error("expected false")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("cancel during not-ready backoff should return quickly, took %v", time.Since(start))
	}
}
