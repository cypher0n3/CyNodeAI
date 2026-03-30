package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/dispatcher"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/nodetelemetry"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

// TestRun_ShutdownFails covers the path where server shutdown returns an error.
func TestRun_ShutdownFails(t *testing.T) {
	testShutdownHook = func(s *http.Server, ctx context.Context) error {
		_ = s.Shutdown(ctx) // stop server so test doesn't leak goroutines
		return errors.New("shutdown failed")
	}
	defer func() { testShutdownHook = nil }()

	oldListen := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":0")
	defer func() {
		if oldListen != "" {
			_ = os.Setenv("LISTEN_ADDR", oldListen)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()

	done := make(chan error, 1)
	go func() { done <- run(ctx, mockDB, cfg, logger) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-done
	if err == nil {
		t.Error("expected shutdown error, got nil")
	}
}

// TestRun_DispatcherRunsOneTick runs run() with dispatcher enabled and a mock worker so the dispatcher loop ticks and runs dispatchOnce.
func TestRun_DispatcherRunsOneTick(t *testing.T) {
	workerResp := workerapi.RunJobResponse{
		Version: 1, TaskID: "t1", JobID: "j1",
		Status: workerapi.StatusCompleted, ExitCode: workerapi.ExitCodePtr(0),
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	}
	server := newWorkerServerOK(&workerResp)
	defer server.Close()

	_ = os.Setenv("LISTEN_ADDR", ":0")
	_ = os.Setenv("DISPATCH_POLL_INTERVAL", "15ms")
	defer func() {
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("DISPATCH_POLL_INTERVAL")
	}()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	payload := testJobPayload
	_, _ = mock.CreateJob(ctx, task.ID, payload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, server.URL, "token")

	runCtx, cancel := context.WithCancel(context.Background())
	cfg := config.LoadOrchestratorConfig()
	cfg.PMAEnabled = false
	logger := slog.Default()

	done := make(chan error, 1)
	go func() { done <- run(runCtx, mock, cfg, logger) }()

	time.Sleep(60 * time.Millisecond) // allow server start and one dispatcher tick
	cancel()
	err := <-done
	if err != nil {
		t.Errorf("run: %v", err)
	}
}

func TestRunMain_FlagParseError(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"control-plane", "-unknown-flag"}
	code := runMain()
	if code != 1 {
		t.Errorf("runMain() with bad flag = %d, want 1", code)
	}
}

func TestRunMain_OpenFails(t *testing.T) {
	oldArgs := os.Args
	oldDB := os.Getenv("DATABASE_URL")
	defer func() {
		os.Args = oldArgs
		if oldDB != "" {
			_ = os.Setenv("DATABASE_URL", oldDB)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	os.Args = []string{"control-plane"}
	_ = os.Setenv("DATABASE_URL", "postgres://invalid:invalid@127.0.0.1:1/nonexistent?sslmode=disable&connect_timeout=2")
	code := runMain()
	if code != 1 {
		t.Errorf("runMain() = %d, want 1", code)
	}
}

// TestRunMain_MigrateOnly runs runMain with -migrate-only and real Postgres; expects exit 0. Skips without POSTGRES_TEST_DSN.
func TestRunMain_MigrateOnly(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_TEST_DSN to run")
	}
	oldArgs := os.Args
	oldDB := os.Getenv("DATABASE_URL")
	defer func() {
		os.Args = oldArgs
		if oldDB != "" {
			_ = os.Setenv("DATABASE_URL", oldDB)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	os.Args = []string{"control-plane", "-migrate-only"}
	_ = os.Setenv("DATABASE_URL", dsn)
	code := runMain()
	if code != 0 {
		t.Errorf("runMain(-migrate-only) = %d, want 0", code)
	}
}

// TestRunMainWithContext_Success covers the success path (store provided, run returns nil).
func TestRunMainWithContext_Success(t *testing.T) {
	oldArgs := os.Args
	oldPMA := os.Getenv("PMA_ENABLED")
	defer func() {
		os.Args = oldArgs
		if oldPMA != "" {
			_ = os.Setenv("PMA_ENABLED", oldPMA)
		} else {
			_ = os.Unsetenv("PMA_ENABLED")
		}
	}()
	os.Args = []string{"control-plane"}
	_ = os.Setenv("PMA_ENABLED", "false")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mockDB := testutil.NewMockDB()
	code := runMainWithContext(ctx, mockDB)
	if code != 0 {
		t.Errorf("runMainWithContext(canceled ctx, mockDB) = %d, want 0", code)
	}
}

// TestRunMainWithContext_StoreFromTestOpener covers the store==nil path when testOpenStore is set (no real DB).
func TestRunMainWithContext_StoreFromTestOpener(t *testing.T) {
	oldArgs := os.Args
	oldPMA := os.Getenv("PMA_ENABLED")
	defer func() {
		os.Args = oldArgs
		if oldPMA != "" {
			_ = os.Setenv("PMA_ENABLED", oldPMA)
		} else {
			_ = os.Unsetenv("PMA_ENABLED")
		}
	}()
	os.Args = []string{"control-plane"}
	_ = os.Setenv("PMA_ENABLED", "false")

	testOpenStore = func(_ context.Context, _ string) (database.Store, error) {
		return testutil.NewMockDB(), nil
	}
	defer func() { testOpenStore = nil }()

	_ = os.Setenv("LISTEN_ADDR", ":0")
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- runMainWithContext(ctx, nil) }()
	time.Sleep(150 * time.Millisecond)
	cancel()
	code := <-done
	if code != 0 {
		t.Errorf("runMainWithContext with testOpenStore: exit code %d", code)
	}
}

// TestRunMainWithContext_TestOpenerMigrateOnly covers store==nil, testOpenStore set, and migrateOnly true.
//
//nolint:dupl // mirrors DatabaseOpenerMigrateOnly for testOpenStore hook
func TestRunMainWithContext_TestOpenerMigrateOnly(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"control-plane", "-migrate-only"}

	testOpenStore = func(_ context.Context, _ string) (database.Store, error) {
		return testutil.NewMockDB(), nil
	}
	defer func() { testOpenStore = nil }()

	code := runMainWithContext(context.Background(), nil)
	if code != 0 {
		t.Errorf("expected exit 0 (migrate-only with testOpenStore), got %d", code)
	}
}

// TestRunMainWithContext_TestOpenerReturnsError covers testOpenStore returning an error.
//
//nolint:dupl // mirrors DatabaseOpenerReturnsError for testOpenStore hook
func TestRunMainWithContext_TestOpenerReturnsError(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"control-plane"}

	testOpenStore = func(_ context.Context, _ string) (database.Store, error) {
		return nil, errors.New("open failed")
	}
	defer func() { testOpenStore = nil }()

	code := runMainWithContext(context.Background(), nil)
	if code != 1 {
		t.Errorf("expected exit 1 when testOpenStore fails, got %d", code)
	}
}

// TestRunMainWithContext_DatabaseOpenerSuccess covers resolveStore using testDatabaseOpen (success, no migrateOnly).
func TestRunMainWithContext_DatabaseOpenerSuccess(t *testing.T) {
	oldArgs := os.Args
	oldPMA := os.Getenv("PMA_ENABLED")
	defer func() {
		os.Args = oldArgs
		if oldPMA != "" {
			_ = os.Setenv("PMA_ENABLED", oldPMA)
		} else {
			_ = os.Unsetenv("PMA_ENABLED")
		}
	}()
	os.Args = []string{"control-plane"}
	_ = os.Setenv("LISTEN_ADDR", ":0")
	_ = os.Setenv("PMA_ENABLED", "false")
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()

	testDatabaseOpen = func(_ context.Context, _ string) (database.Store, error) {
		return testutil.NewMockDB(), nil
	}
	defer func() { testDatabaseOpen = nil }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := runMainWithContext(ctx, nil)
	if code != 0 {
		t.Errorf("expected exit 0 when testDatabaseOpen succeeds, got %d", code)
	}
}

// TestRunMainWithContext_DatabaseOpenerMigrateOnly covers resolveStore using testDatabaseOpen with migrateOnly.
//
//nolint:dupl // mirrors TestOpenerMigrateOnly for testDatabaseOpen hook
func TestRunMainWithContext_DatabaseOpenerMigrateOnly(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"control-plane", "-migrate-only"}

	testDatabaseOpen = func(_ context.Context, _ string) (database.Store, error) {
		return testutil.NewMockDB(), nil
	}
	defer func() { testDatabaseOpen = nil }()

	code := runMainWithContext(context.Background(), nil)
	if code != 0 {
		t.Errorf("expected exit 0 (migrate-only with testDatabaseOpen), got %d", code)
	}
}

// TestRunMainWithContext_DatabaseOpenerReturnsError covers testDatabaseOpen returning an error.
//
//nolint:dupl // mirrors TestOpenerReturnsError for testDatabaseOpen hook
func TestRunMainWithContext_DatabaseOpenerReturnsError(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"control-plane"}

	testDatabaseOpen = func(_ context.Context, _ string) (database.Store, error) {
		return nil, errors.New("database open failed")
	}
	defer func() { testDatabaseOpen = nil }()

	code := runMainWithContext(context.Background(), nil)
	if code != 1 {
		t.Errorf("expected exit 1 when testDatabaseOpen fails, got %d", code)
	}
}

// TestRunMainWithContext_RunReturnsError covers runMainWithContext returning 1 when run() returns an error.
func TestRunMainWithContext_RunReturnsError(t *testing.T) {
	oldArgs := os.Args
	oldPMA := os.Getenv("PMA_ENABLED")
	defer func() {
		os.Args = oldArgs
		if oldPMA != "" {
			_ = os.Setenv("PMA_ENABLED", oldPMA)
		} else {
			_ = os.Unsetenv("PMA_ENABLED")
		}
	}()
	os.Args = []string{"control-plane"}
	_ = os.Setenv("LISTEN_ADDR", ":0")
	_ = os.Setenv("PMA_ENABLED", "false")
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()

	testOpenStore = func(_ context.Context, _ string) (database.Store, error) {
		return testutil.NewMockDB(), nil
	}
	testShutdownHook = func(s *http.Server, ctx context.Context) error {
		_ = s.Shutdown(ctx)
		return errors.New("shutdown failed")
	}
	defer func() {
		testOpenStore = nil
		testShutdownHook = nil
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- runMainWithContext(ctx, nil) }()
	time.Sleep(80 * time.Millisecond)
	cancel()
	code := <-done
	if code != 1 {
		t.Errorf("expected exit 1 when run() fails, got %d", code)
	}
}

// TestRunMainWithContext_MigrateOnlyWithStore covers migrate-only with injected store (no DB open).
func TestRunMainWithContext_MigrateOnlyWithStore(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"control-plane", "-migrate-only"}

	ctx := context.Background()
	mockDB := testutil.NewMockDB()
	code := runMainWithContext(ctx, mockDB)
	if code != 0 {
		t.Errorf("runMainWithContext(ctx, mockDB) with -migrate-only = %d, want 0", code)
	}
}

// TestRunMain_RunFails runs runMain with real Postgres but run() fails (invalid port); expects exit 1. Skips without POSTGRES_TEST_DSN.
func TestRunMain_RunFails(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_TEST_DSN to run")
	}
	oldArgs := os.Args
	oldDB := os.Getenv("DATABASE_URL")
	oldListen := os.Getenv("LISTEN_ADDR")
	defer func() {
		os.Args = oldArgs
		if oldDB != "" {
			_ = os.Setenv("DATABASE_URL", oldDB)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
		if oldListen != "" {
			_ = os.Setenv("LISTEN_ADDR", oldListen)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()
	os.Args = []string{"control-plane"}
	_ = os.Setenv("DATABASE_URL", dsn)
	_ = os.Setenv("LISTEN_ADDR", ":99999")
	code := runMain()
	if code != 1 {
		t.Errorf("runMain() with invalid port = %d, want 1", code)
	}
}

// completeJobErrorStore fails CompleteJob so dispatchOnce returns error after worker succeeds.
type completeJobErrorStore struct {
	*testutil.MockDB
}

func (m *completeJobErrorStore) CompleteJob(_ context.Context, _ uuid.UUID, _, _ string) error {
	return errors.New("complete job error")
}

func TestDispatchOnce_CompleteJobFails(t *testing.T) {
	workerResp := workerapi.RunJobResponse{
		Version: 1, TaskID: "t1", JobID: "j1",
		Status: workerapi.StatusCompleted, ExitCode: workerapi.ExitCodePtr(0),
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	}
	server := newWorkerServerOK(&workerResp)
	defer server.Close()

	base := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := base.CreateTask(ctx, nil, "p", nil)
	_, _ = base.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := base.CreateNode(ctx, "n1")
	makeDispatchableNode(t, base, ctx, node, server.URL, "t")

	mock := &completeJobErrorStore{MockDB: base}
	cfg := dispatcherConfig{HTTPTimeout: 5 * time.Second}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err == nil {
		t.Fatal("expected error when CompleteJob fails")
	}
}

func TestDispatchOnce_CallWorkerAPINonOK(t *testing.T) {
	runDispatchOnceWithWorkerStatus(t, http.StatusNotFound)
}

// assignJobErrorStore fails AssignJobToNode so dispatchOnce returns error.
type assignJobErrorStore struct {
	*testutil.MockDB
}

func (m *assignJobErrorStore) AssignJobToNode(_ context.Context, _, _ uuid.UUID) error {
	return errors.New("assign job error")
}

func TestDispatchOnce_AssignJobToNodeFails(t *testing.T) {
	mock := &assignJobErrorStore{MockDB: testutil.NewMockDB()}
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	_, _ = mock.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock.MockDB, ctx, node, testWorkerAPIURL, "t")

	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err == nil {
		t.Fatal("expected error when AssignJobToNode fails")
	}
}

func TestDispatchOnce_CallWorkerAPINetworkError(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	_, _ = mock.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, "http://127.0.0.1:19999", "t")

	cfg := dispatcherConfig{HTTPTimeout: 100 * time.Millisecond}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err != nil {
		t.Fatalf("dispatchOnce should complete job as failed on network error: %v", err)
	}
}

func TestDispatchOnce_WorkerAPIInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	_, _ = mock.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, server.URL, "t")

	cfg := dispatcherConfig{HTTPTimeout: 5 * time.Second}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err != nil {
		t.Fatalf("dispatchOnce should complete job as failed on invalid JSON: %v", err)
	}
}

func TestPullNodeTelemetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	ctx := context.Background()
	client := nodetelemetry.NewClient()
	logger := slog.Default()
	pullNodeTelemetry(ctx, client, srv.URL, "", "node-1", logger)
}

func TestPullNodeTelemetry_infoOK_statsFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/worker/telemetry/node:info":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case "/v1/worker/telemetry/node:stats":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	client := nodetelemetry.NewClient()
	logger := slog.Default()
	pullNodeTelemetry(ctx, client, srv.URL, "", "node-1", logger)
}

func TestPullNodeTelemetry_infoFail_statsOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/worker/telemetry/node:info":
			w.WriteHeader(http.StatusInternalServerError)
		case "/v1/worker/telemetry/node:stats":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	client := nodetelemetry.NewClient()
	logger := slog.Default()
	pullNodeTelemetry(ctx, client, srv.URL, "", "node-1", logger)
}

// telemetryListErrorStore fails ListDispatchableNodes for runTelemetryPullLoop tests.
type telemetryListErrorStore struct {
	*testutil.MockDB
}

func (m *telemetryListErrorStore) ListDispatchableNodes(_ context.Context) ([]*models.Node, error) {
	return nil, errors.New("list nodes failed")
}

func TestRunTelemetryPullLoop_ListFails(t *testing.T) {
	store := &telemetryListErrorStore{MockDB: testutil.NewMockDB()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	orig := telemetryPullInterval
	telemetryPullInterval = 15 * time.Millisecond
	defer func() { telemetryPullInterval = orig }()

	done := make(chan struct{})
	go func() {
		runTelemetryPullLoop(ctx, store, slog.Default())
		close(done)
	}()
	time.Sleep(25 * time.Millisecond) // one tick
	cancel()
	<-done
}

func TestRunTelemetryPullLoop_OneTick(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, srv.URL, "tok")

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	orig := telemetryPullInterval
	telemetryPullInterval = 15 * time.Millisecond
	defer func() { telemetryPullInterval = orig }()

	done := make(chan struct{})
	go func() {
		runTelemetryPullLoop(runCtx, mock, slog.Default())
		close(done)
	}()
	time.Sleep(35 * time.Millisecond) // allow one tick to run
	cancel()
	<-done
}
