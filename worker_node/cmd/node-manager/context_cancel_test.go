package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestRealCmdRunner_CombinedOutputContext(t *testing.T) {
	r := realCmdRunner{}
	_, err := r.CombinedOutputContext(context.Background(), "/nonexistent-binary-xyz", "arg")
	if err == nil {
		t.Fatal("expected exec error")
	}
}

func TestStartOneManagedService_PMA_ContextCanceledDuringWait(t *testing.T) {
	withRunner(t, fakeRunner{combinedOutput: []byte("cynodeai-managed-x\n")})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := &nodepayloads.ConfigManagedService{ServiceID: "x", ServiceType: "pma", Image: "img"}
	err := startOneManagedService(ctx, "podman", svc, "x", "pma", "img", "cynodeai-managed-x")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("startOneManagedService: want context.Canceled, got %v", err)
	}
}

func TestStartOneManagedService_NewPMA_ContextCanceledAfterRun(t *testing.T) {
	withRunner(t, fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		if len(args) >= 1 && args[0] == "ps" {
			return []byte("\n"), nil // no existing container
		}
		return []byte("ok\n"), nil // `podman run` succeeds
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := &nodepayloads.ConfigManagedService{ServiceID: "p1", ServiceType: "pma", Image: "img"}
	err := startOneManagedService(ctx, "podman", svc, "p1", "pma", "img", "cynodeai-managed-p1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("startOneManagedService: want context.Canceled, got %v", err)
	}
}

func TestContextCancel_WaitForPMAReadyUDS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := waitForPMAReadyUDS(ctx, filepath.Join(t.TempDir(), "nope.sock"), time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForPMAReadyUDS: want context.Canceled, got %v", err)
	}
}

func TestWaitForPMAReadyUDS_DeadlineNoError(t *testing.T) {
	// Legacy: polling window expires without a reachable socket; not a startup failure.
	err := waitForPMAReadyUDS(context.Background(), filepath.Join(t.TempDir(), "missing.sock"), 80*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForPMAReadyUDS: want nil after deadline, got %v", err)
	}
}
