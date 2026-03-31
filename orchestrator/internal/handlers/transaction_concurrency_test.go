package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
)

const postgresTestDSNEnv = "POSTGRES_TEST_DSN"

// testIntegrationDB opens Postgres when POSTGRES_TEST_DSN is set; skips otherwise.
func testIntegrationDB(t *testing.T) (*database.DB, context.Context) {
	t.Helper()
	dsn := os.Getenv(postgresTestDSNEnv)
	if dsn == "" {
		t.Skipf("set %s for transaction concurrency tests", postgresTestDSNEnv)
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunSchema(ctx, slog.Default()); err != nil {
		t.Fatalf("RunSchema: %v", err)
	}
	return db, ctx
}

// TestLeaseTx verifies concurrent workflow lease acquires: at most one holder wins; others get ErrLeaseHeld.
func TestLeaseTx(t *testing.T) {
	db, ctx := testIntegrationDB(t)
	task, err := db.CreateTask(ctx, nil, "lease-tx-test", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	const n = 24
	var okCount int32
	var heldCount int32
	var badErr int32
	var wg sync.WaitGroup
	expiresAt := time.Now().UTC().Add(time.Hour)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := db.AcquireTaskWorkflowLease(ctx, task.ID, uuid.New(), fmt.Sprintf("holder-%d", idx), expiresAt)
			if err == nil {
				atomic.AddInt32(&okCount, 1)
				return
			}
			if errors.Is(err, database.ErrLeaseHeld) {
				atomic.AddInt32(&heldCount, 1)
				return
			}
			atomic.AddInt32(&badErr, 1)
		}(i)
	}
	wg.Wait()
	if badErr != 0 {
		t.Fatalf("unexpected errors from concurrent acquire: %d", badErr)
	}
	if okCount != 1 {
		t.Fatalf("expected exactly 1 successful acquire, got %d", okCount)
	}
	if heldCount != n-1 {
		t.Fatalf("expected %d ErrLeaseHeld, got %d", n-1, heldCount)
	}
}

// TestTaskCreateTx verifies concurrent task creation with the same normalized name yields unique summaries (no lost updates).
//
//nolint:gocognit // table of concurrent task creation scenarios.
func TestTaskCreateTx(t *testing.T) {
	db, ctx := testIntegrationDB(t)
	u, err := db.CreateUser(ctx, "task-tx-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	name := "My-Shared-Task-Name"
	const n = 16
	var wg sync.WaitGroup
	summaries := make(map[string]int)
	var mu sync.Mutex
	var firstErr error
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := db.CreateTask(ctx, &u.ID, "prompt", &name, nil)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			if task.Summary == nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = errors.New("nil summary")
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			summaries[*task.Summary]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("CreateTask: %v", firstErr)
	}
	if len(summaries) != n {
		t.Fatalf("expected %d distinct summaries, got %d: %v", n, len(summaries), summaries)
	}
}

// TestPreferenceUpsertTx verifies concurrent creates for the same key: one succeeds, rest get ErrExists.
func TestPreferenceUpsertTx(t *testing.T) {
	db, ctx := testIntegrationDB(t)
	key := "pref-tx-" + uuid.New().String()
	const n = 12
	var wg sync.WaitGroup
	var okCount int32
	var existsCount int32
	var badErr int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := db.CreatePreference(ctx, "system", nil, key, `"v"`, "string", nil, nil)
			if err == nil {
				atomic.AddInt32(&okCount, 1)
				return
			}
			if errors.Is(err, database.ErrExists) {
				atomic.AddInt32(&existsCount, 1)
				return
			}
			atomic.AddInt32(&badErr, 1)
		}()
	}
	wg.Wait()
	if badErr != 0 {
		t.Fatalf("unexpected errors from concurrent CreatePreference: %d", badErr)
	}
	if okCount != 1 {
		t.Fatalf("expected exactly 1 create, got %d", okCount)
	}
	if existsCount != n-1 {
		t.Fatalf("expected %d ErrExists, got %d", n-1, existsCount)
	}
}
