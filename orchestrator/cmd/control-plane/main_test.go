package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestLoadDispatcherConfig(t *testing.T) {
	os.Unsetenv("DISPATCHER_ENABLED")
	os.Unsetenv("DISPATCH_POLL_INTERVAL")
	os.Unsetenv("WORKER_API_URL")
	os.Unsetenv("WORKER_API_BEARER_TOKEN")
	os.Unsetenv("DISPATCH_HTTP_TIMEOUT")
	defer func() {
		os.Unsetenv("DISPATCHER_ENABLED")
		os.Unsetenv("DISPATCH_POLL_INTERVAL")
		os.Unsetenv("WORKER_API_URL")
		os.Unsetenv("WORKER_API_BEARER_TOKEN")
		os.Unsetenv("DISPATCH_HTTP_TIMEOUT")
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

	os.Setenv("DISPATCHER_ENABLED", "false")
	os.Setenv("DISPATCH_POLL_INTERVAL", "2s")
	os.Setenv("WORKER_API_URL", "http://worker:8081")
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
	os.Unsetenv("TEST_DISPATCH_DURATION")
	if getDurationEnv("TEST_DISPATCH_DURATION", 10*time.Second) != 10*time.Second {
		t.Error("default")
	}
	os.Setenv("TEST_DISPATCH_DURATION", "3m")
	defer os.Unsetenv("TEST_DISPATCH_DURATION")
	if getDurationEnv("TEST_DISPATCH_DURATION", time.Second) != 3*time.Minute {
		t.Error("parse 3m")
	}
}

func TestGetEnv(t *testing.T) {
	os.Unsetenv("TEST_CP_ENV")
	if getEnv("TEST_CP_ENV", "default") != "default" {
		t.Error("default")
	}
	os.Setenv("TEST_CP_ENV", "value")
	defer os.Unsetenv("TEST_CP_ENV")
	if getEnv("TEST_CP_ENV", "default") != "value" {
		t.Error("from env")
	}
}

func TestDispatchOnce_Success(t *testing.T) {
	workerResp := workerapi.RunJobResponse{
		Version: 1, TaskID: "t1", JobID: "j1",
		Status: workerapi.StatusCompleted, ExitCode: 0,
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(workerResp)
	}))
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
	os.Setenv("DISPATCHER_ENABLED", "false")
	defer os.Unsetenv("DISPATCHER_ENABLED")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mock := testutil.NewMockDB()
	logger := slog.Default()
	startDispatcher(ctx, mock, logger)
}

func TestStartDispatcher_NoToken(t *testing.T) {
	os.Setenv("DISPATCHER_ENABLED", "true")
	os.Unsetenv("WORKER_API_BEARER_TOKEN")
	defer func() {
		os.Unsetenv("DISPATCHER_ENABLED")
		os.Unsetenv("WORKER_API_BEARER_TOKEN")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mock := testutil.NewMockDB()
	logger := slog.Default()
	startDispatcher(ctx, mock, logger)
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

func TestDispatchOnce_WorkerAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
		t.Fatalf("dispatchOnce should complete job as failed on worker error: %v", err)
	}
}
