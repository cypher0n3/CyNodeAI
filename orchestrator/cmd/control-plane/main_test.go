package main

import (
	"context"
	"encoding/json"
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
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

const testJobPayload = `{"command":["echo","hi"]}`

func TestLoadDispatcherConfig(t *testing.T) {
	_ = os.Unsetenv("DISPATCHER_ENABLED")
	_ = os.Unsetenv("DISPATCH_POLL_INTERVAL")
	_ = os.Unsetenv("WORKER_API_URL")
	_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
	_ = os.Unsetenv("DISPATCH_HTTP_TIMEOUT")
	defer func() {
		_ = os.Unsetenv("DISPATCHER_ENABLED")
		_ = os.Unsetenv("DISPATCH_POLL_INTERVAL")
		_ = os.Unsetenv("WORKER_API_URL")
		_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
		_ = os.Unsetenv("DISPATCH_HTTP_TIMEOUT")
	}()

	cfg := loadDispatcherConfig()
	if !cfg.Enabled {
		t.Error("default Enabled should be true")
	}
	if cfg.PollInterval != 1*time.Second {
		t.Errorf("default PollInterval: %v", cfg.PollInterval)
	}

	_ = os.Setenv("DISPATCHER_ENABLED", "false")
	_ = os.Setenv("DISPATCH_POLL_INTERVAL", "2s")
	cfg2 := loadDispatcherConfig()
	if cfg2.Enabled {
		t.Error("DISPATCHER_ENABLED=false should set Enabled false")
	}
	if cfg2.PollInterval != 2*time.Second {
		t.Errorf("DISPATCH_POLL_INTERVAL: %v", cfg2.PollInterval)
	}
}

func TestGetDurationEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_DISPATCH_DURATION")
	if getDurationEnv("TEST_DISPATCH_DURATION", 10*time.Second) != 10*time.Second {
		t.Error("default")
	}
	_ = os.Setenv("TEST_DISPATCH_DURATION", "3m")
	defer func() { _ = os.Unsetenv("TEST_DISPATCH_DURATION") }()
	if getDurationEnv("TEST_DISPATCH_DURATION", time.Second) != 3*time.Minute {
		t.Error("parse 3m")
	}
}

func TestGetDurationEnv_InvalidValue(t *testing.T) {
	_ = os.Setenv("TEST_DISPATCH_DURATION_BAD", "not-a-duration")
	defer func() { _ = os.Unsetenv("TEST_DISPATCH_DURATION_BAD") }()
	if getDurationEnv("TEST_DISPATCH_DURATION_BAD", 7*time.Second) != 7*time.Second {
		t.Error("invalid duration should return default")
	}
}

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_CP_ENV")
	if getEnv("TEST_CP_ENV", "default") != "default" {
		t.Error("default")
	}
	_ = os.Setenv("TEST_CP_ENV", "value")
	defer func() { _ = os.Unsetenv("TEST_CP_ENV") }()
	if getEnv("TEST_CP_ENV", "default") != "value" {
		t.Error("from env")
	}
}

// newWorkerServerOK returns a test server that responds 200 with the given worker response. Caller must defer server.Close().
func newWorkerServerOK(workerResp *workerapi.RunJobResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(workerResp)
	}))
}

// makeDispatchableNode sets the node as active with config ack applied and worker API URL/token (for ListDispatchableNodes).
func makeDispatchableNode(t *testing.T, mock *testutil.MockDB, ctx context.Context, node *models.Node, workerURL, bearerToken string) {
	t.Helper()
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	_ = mock.UpdateNodeWorkerAPIConfig(ctx, node.ID, workerURL, bearerToken)
	ackAt := time.Now().UTC()
	_ = mock.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
}

func TestDispatchOnce_Success(t *testing.T) {
	workerResp := workerapi.RunJobResponse{
		Version: 1, TaskID: "t1", JobID: "j1",
		Status: workerapi.StatusCompleted, ExitCode: 0,
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	}
	server := newWorkerServerOK(&workerResp)
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "prompt")
	payload := testJobPayload
	job, _ := mock.CreateJob(ctx, task.ID, payload)
	node, _ := mock.CreateNode(ctx, "node-1")
	makeDispatchableNode(t, mock, ctx, node, server.URL, "token")

	cfg := dispatcherConfig{HTTPTimeout: 5 * time.Second}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()

	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err != nil {
		t.Fatalf("dispatchOnce: %v", err)
	}

	// Job should be completed
	j, _ := mock.GetJobByID(ctx, job.ID)
	if j.Status != models.JobStatusCompleted {
		t.Errorf("job status %s", j.Status)
	}
}

func TestDispatchOnce_NoDispatchableNodes(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p")
	_, _ = mock.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	// No worker API config or config ack -> not dispatchable

	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err == nil {
		t.Fatal("expected error when no dispatchable nodes")
	}
}

func TestDispatchOnce_ErrNotFound(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()

	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err == nil {
		t.Fatal("expected ErrNotFound")
	}
}

func TestDispatchOnce_NoNodes(t *testing.T) {
	payload := testJobPayload
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p")
	_, _ = mock.CreateJob(ctx, task.ID, payload)
	// No active node

	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()

	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err == nil {
		t.Fatal("expected no active nodes error")
	}
}

func TestStartDispatcher_Disabled(t *testing.T) {
	_ = os.Setenv("DISPATCHER_ENABLED", "false")
	defer func() { _ = os.Unsetenv("DISPATCHER_ENABLED") }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mock := testutil.NewMockDB()
	logger := slog.Default()
	startDispatcher(ctx, mock, logger)
}

func TestStartDispatcher_NoToken(t *testing.T) {
	_ = os.Setenv("DISPATCHER_ENABLED", "true")
	_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
	defer func() {
		_ = os.Unsetenv("DISPATCHER_ENABLED")
		_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mock := testutil.NewMockDB()
	logger := slog.Default()
	// Dispatcher no longer exits early when token unset (uses per-node token); run in goroutine and cancel.
	go startDispatcher(ctx, mock, logger)
	<-time.After(25 * time.Millisecond)
	cancel()
	<-time.After(10 * time.Millisecond)
}

func TestStartDispatcher_EnabledOneTick(t *testing.T) {
	_ = os.Setenv("DISPATCHER_ENABLED", "true")
	_ = os.Setenv("WORKER_API_BEARER_TOKEN", "test-token")
	_ = os.Setenv("DISPATCH_POLL_INTERVAL", "1ms")
	defer func() {
		_ = os.Unsetenv("DISPATCHER_ENABLED")
		_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
		_ = os.Unsetenv("DISPATCH_POLL_INTERVAL")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	mock := testutil.NewMockDB()
	logger := slog.Default()
	go startDispatcher(ctx, mock, logger)
	// Allow one tick (dispatchOnce returns ErrNotFound when queue empty)
	<-time.After(20 * time.Millisecond)
	cancel()
	<-time.After(10 * time.Millisecond)
}

// listDispatchableNodesErrorStore fails ListDispatchableNodes so dispatchOnce returns non-ErrNotFound error.
type listDispatchableNodesErrorStore struct {
	*testutil.MockDB
}

func (m *listDispatchableNodesErrorStore) ListDispatchableNodes(_ context.Context) ([]*models.Node, error) {
	return nil, errors.New("list nodes error")
}

func (m *listDispatchableNodesErrorStore) ListActiveNodes(ctx context.Context) ([]*models.Node, error) {
	return m.MockDB.ListActiveNodes(ctx)
}

func TestStartDispatcher_DispatchOnceReturnsError(t *testing.T) {
	_ = os.Setenv("DISPATCHER_ENABLED", "true")
	_ = os.Setenv("WORKER_API_BEARER_TOKEN", "token")
	_ = os.Setenv("DISPATCH_POLL_INTERVAL", "1ms")
	defer func() {
		_ = os.Unsetenv("DISPATCHER_ENABLED")
		_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
		_ = os.Unsetenv("DISPATCH_POLL_INTERVAL")
	}()
	base := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := base.CreateTask(ctx, nil, "p")
	_, _ = base.CreateJob(ctx, task.ID, `{"command":["x"]}`)
	mock := &listDispatchableNodesErrorStore{MockDB: base}
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.Default()
	go startDispatcher(ctx, mock, logger)
	<-time.After(25 * time.Millisecond)
	cancel()
	<-time.After(10 * time.Millisecond)
}

func TestBootstrapAdminUser_AlreadyExists(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	_, _ = mock.CreateUser(ctx, "admin", nil)
	logger := slog.Default()
	err := bootstrapAdminUser(ctx, mock, "password", logger)
	if err != nil {
		t.Fatalf("bootstrapAdminUser: %v", err)
	}
}

func TestBootstrapAdminUser_Create(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	logger := slog.Default()
	err := bootstrapAdminUser(ctx, mock, "adminpass", logger)
	if err != nil {
		t.Fatalf("bootstrapAdminUser: %v", err)
	}
	u, err := mock.GetUserByHandle(ctx, "admin")
	if err != nil || u == nil {
		t.Fatalf("admin user not created: %v", err)
	}
}

func TestBootstrapAdminUser_GetUserError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("db error")
	ctx := context.Background()
	logger := slog.Default()
	err := bootstrapAdminUser(ctx, mock, "adminpass", logger)
	if err == nil {
		t.Fatal("expected error when GetUserByHandle fails with non-ErrNotFound")
	}
}

// createUserErrorStore fails CreateUser (bootstrapAdminUser path).
type createUserErrorStore struct {
	*testutil.MockDB
}

func (m *createUserErrorStore) CreateUser(_ context.Context, _ string, _ *string) (*models.User, error) {
	return nil, errors.New("create user error")
}

// createPasswordCredErrorStore fails CreatePasswordCredential (bootstrapAdminUser path).
type createPasswordCredErrorStore struct {
	*testutil.MockDB
}

func (m *createPasswordCredErrorStore) CreatePasswordCredential(_ context.Context, _ uuid.UUID, _ []byte, _ string) (*models.PasswordCredential, error) {
	return nil, errors.New("create credential error")
}

func TestBootstrapAdminUser_CreateUserFails(t *testing.T) {
	testBootstrapAdminUserFails(t, &createUserErrorStore{MockDB: testutil.NewMockDB()}, "CreateUser")
}

func TestBootstrapAdminUser_CreatePasswordCredentialFails(t *testing.T) {
	testBootstrapAdminUserFails(t, &createPasswordCredErrorStore{MockDB: testutil.NewMockDB()}, "CreatePasswordCredential")
}

func testBootstrapAdminUserFails(t *testing.T, store database.Store, which string) {
	t.Helper()
	ctx := context.Background()
	logger := slog.Default()
	err := bootstrapAdminUser(ctx, store, "adminpass", logger)
	if err == nil {
		t.Fatalf("expected error when %s fails", which)
	}
}

func TestRun_BootstrapAdminUserFails(t *testing.T) {
	ctx := context.Background()
	cfg := config.LoadOrchestratorConfig()
	mock := &createPasswordCredErrorStore{MockDB: testutil.NewMockDB()}
	logger := slog.Default()
	err := run(ctx, mock, cfg, logger)
	if err == nil {
		t.Fatal("run should return error when bootstrapAdminUser fails")
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
	ctx := context.Background()
	cfg := config.LoadOrchestratorConfig()
	mock := testutil.NewMockDB()
	logger := slog.Default()
	err := run(ctx, mock, cfg, logger)
	if err == nil {
		t.Fatal("run should return error when ListenAndServe fails (invalid port)")
	}
}

func TestDispatchOnce_InvalidPayload(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p")
	_, _ = mock.CreateJob(ctx, task.ID, "not-valid-json")
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, "http://localhost:8081", "token")

	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err != nil {
		t.Fatalf("dispatchOnce with bad payload should complete job as failed: %v", err)
	}
}

func runDispatchOnceWithWorkerStatus(t *testing.T, statusCode int) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))
	defer server.Close()
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p")
	_, _ = mock.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, server.URL, "t")
	cfg := dispatcherConfig{HTTPTimeout: 5 * time.Second}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err != nil {
		t.Fatalf("dispatchOnce should complete job as failed: %v", err)
	}
}

func TestDispatchOnce_WorkerAPIError(t *testing.T) {
	runDispatchOnceWithWorkerStatus(t, http.StatusInternalServerError)
}

func TestDispatchOnce_WorkerAPIBadVersion(t *testing.T) {
	server := newWorkerServerOK(&workerapi.RunJobResponse{
		Version: 0, TaskID: "t1", JobID: "j1",
		Status: workerapi.StatusCompleted, ExitCode: 0,
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	})
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p")
	_, _ = mock.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, server.URL, "t")

	cfg := dispatcherConfig{HTTPTimeout: 5 * time.Second}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err != nil {
		t.Fatalf("dispatchOnce should complete job as failed on bad version: %v", err)
	}
}

func TestRun_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	err := run(ctx, mockDB, cfg, logger)
	if err != nil {
		t.Errorf("run: %v", err)
	}
}

// TestRun_ShutdownSucceeds covers the shutdown success path (server started, then ctx cancelled, shutdown succeeds).
func TestRun_ShutdownSucceeds(t *testing.T) {
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

	// Allow server to start and dispatcher to tick once.
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-done
	if err != nil {
		t.Errorf("run after cancel: %v", err)
	}
}

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
		Status: workerapi.StatusCompleted, ExitCode: 0,
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
	task, _ := mock.CreateTask(ctx, nil, "p")
	payload := testJobPayload
	_, _ = mock.CreateJob(ctx, task.ID, payload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, server.URL, "token")

	runCtx, cancel := context.WithCancel(context.Background())
	cfg := config.LoadOrchestratorConfig()
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
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"control-plane"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mockDB := testutil.NewMockDB()
	code := runMainWithContext(ctx, mockDB)
	if code != 0 {
		t.Errorf("runMainWithContext(cancelled ctx, mockDB) = %d, want 0", code)
	}
}

// TestRunMainWithContext_StoreFromTestOpener covers the store==nil path when testOpenStore is set (no real DB).
func TestRunMainWithContext_StoreFromTestOpener(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"control-plane"}

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
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"control-plane"}
	_ = os.Setenv("LISTEN_ADDR", ":0")
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
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"control-plane"}
	_ = os.Setenv("LISTEN_ADDR", ":0")
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
		Status: workerapi.StatusCompleted, ExitCode: 0,
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	}
	server := newWorkerServerOK(&workerResp)
	defer server.Close()

	base := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := base.CreateTask(ctx, nil, "p")
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
	task, _ := mock.CreateTask(ctx, nil, "p")
	_, _ = mock.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock.MockDB, ctx, node, "http://localhost:8081", "t")

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
	task, _ := mock.CreateTask(ctx, nil, "p")
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
	task, _ := mock.CreateTask(ctx, nil, "p")
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
