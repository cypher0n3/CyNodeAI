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
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

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
	if cfg.WorkerAPIURL != "http://localhost:8081" {
		t.Errorf("default WorkerAPIURL: %s", cfg.WorkerAPIURL)
	}

	_ = os.Setenv("DISPATCHER_ENABLED", "false")
	_ = os.Setenv("DISPATCH_POLL_INTERVAL", "2s")
	_ = os.Setenv("WORKER_API_URL", "http://worker:8081")
	cfg2 := loadDispatcherConfig()
	if cfg2.Enabled {
		t.Error("DISPATCHER_ENABLED=false should set Enabled false")
	}
	if cfg2.PollInterval != 2*time.Second {
		t.Errorf("DISPATCH_POLL_INTERVAL: %v", cfg2.PollInterval)
	}
	if cfg2.WorkerAPIURL != "http://worker:8081" {
		t.Errorf("WORKER_API_URL: %s", cfg2.WorkerAPIURL)
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
	payload := `{"command":["echo","hi"]}`
	job, _ := mock.CreateJob(ctx, task.ID, payload)
	node, _ := mock.CreateNode(ctx, "node-1")
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)

	cfg := dispatcherConfig{
		WorkerAPIURL: server.URL,
		BearerToken:  "token",
		HTTPTimeout:  5 * time.Second,
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()

	err := dispatchOnce(ctx, mock, client, cfg, logger)
	if err != nil {
		t.Fatalf("dispatchOnce: %v", err)
	}

	// Job should be completed
	j, _ := mock.GetJobByID(ctx, job.ID)
	if j.Status != models.JobStatusCompleted {
		t.Errorf("job status %s", j.Status)
	}
}

func TestDispatchOnce_ErrNotFound(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()

	err := dispatchOnce(ctx, mock, client, cfg, logger)
	if err == nil {
		t.Fatal("expected ErrNotFound")
	}
}

func TestDispatchOnce_NoNodes(t *testing.T) {
	payload := `{"command":["echo","hi"]}`
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p")
	_, _ = mock.CreateJob(ctx, task.ID, payload)
	// No active node

	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()

	err := dispatchOnce(ctx, mock, client, cfg, logger)
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
	startDispatcher(ctx, mock, logger)
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

// listActiveNodesErrorStore fails ListActiveNodes so dispatchOnce returns non-ErrNotFound error.
type listActiveNodesErrorStore struct {
	*testutil.MockDB
}

func (m *listActiveNodesErrorStore) ListActiveNodes(_ context.Context) ([]*models.Node, error) {
	return nil, errors.New("list nodes error")
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
	mock := &listActiveNodesErrorStore{MockDB: base}
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

// createPasswordCredErrorStore fails CreatePasswordCredential (bootstrapAdminUser path).
type createPasswordCredErrorStore struct {
	*testutil.MockDB
}

func (m *createPasswordCredErrorStore) CreatePasswordCredential(_ context.Context, _ uuid.UUID, _ []byte, _ string) (*models.PasswordCredential, error) {
	return nil, errors.New("create credential error")
}

func TestBootstrapAdminUser_CreatePasswordCredentialFails(t *testing.T) {
	mock := &createPasswordCredErrorStore{MockDB: testutil.NewMockDB()}
	ctx := context.Background()
	logger := slog.Default()
	err := bootstrapAdminUser(ctx, mock, "adminpass", logger)
	if err == nil {
		t.Fatal("expected error when CreatePasswordCredential fails")
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
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)

	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()
	err := dispatchOnce(ctx, mock, client, cfg, logger)
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
	_, _ = mock.CreateJob(ctx, task.ID, `{"command":["echo","hi"]}`)
	node, _ := mock.CreateNode(ctx, "n1")
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	cfg := dispatcherConfig{
		WorkerAPIURL: server.URL,
		BearerToken:  "t",
		HTTPTimeout:  5 * time.Second,
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatchOnce(ctx, mock, client, cfg, logger)
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
	_, _ = mock.CreateJob(ctx, task.ID, `{"command":["echo","hi"]}`)
	node, _ := mock.CreateNode(ctx, "n1")
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)

	cfg := dispatcherConfig{
		WorkerAPIURL: server.URL,
		BearerToken:  "t",
		HTTPTimeout:  5 * time.Second,
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatchOnce(ctx, mock, client, cfg, logger)
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
	_ = os.Setenv("DATABASE_URL", "postgres://invalid:invalid@127.0.0.1:1/nonexistent?sslmode=disable")
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
	_, _ = base.CreateJob(ctx, task.ID, `{"command":["echo","hi"]}`)
	node, _ := base.CreateNode(ctx, "n1")
	_ = base.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)

	mock := &completeJobErrorStore{MockDB: base}
	cfg := dispatcherConfig{
		WorkerAPIURL: server.URL,
		BearerToken:  "t",
		HTTPTimeout:  5 * time.Second,
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatchOnce(ctx, mock, client, cfg, logger)
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
	_, _ = mock.CreateJob(ctx, task.ID, `{"command":["echo","hi"]}`)
	node, _ := mock.CreateNode(ctx, "n1")
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)

	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()
	err := dispatchOnce(ctx, mock, client, cfg, logger)
	if err == nil {
		t.Fatal("expected error when AssignJobToNode fails")
	}
}

func TestDispatchOnce_CallWorkerAPINetworkError(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p")
	_, _ = mock.CreateJob(ctx, task.ID, `{"command":["echo","hi"]}`)
	node, _ := mock.CreateNode(ctx, "n1")
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)

	cfg := dispatcherConfig{
		WorkerAPIURL: "http://127.0.0.1:19999",
		BearerToken:  "t",
		HTTPTimeout:  100 * time.Millisecond,
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatchOnce(ctx, mock, client, cfg, logger)
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
	_, _ = mock.CreateJob(ctx, task.ID, `{"command":["echo","hi"]}`)
	node, _ := mock.CreateNode(ctx, "n1")
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)

	cfg := dispatcherConfig{
		WorkerAPIURL: server.URL,
		BearerToken:  "t",
		HTTPTimeout:  5 * time.Second,
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatchOnce(ctx, mock, client, cfg, logger)
	if err != nil {
		t.Fatalf("dispatchOnce should complete job as failed on invalid JSON: %v", err)
	}
}
