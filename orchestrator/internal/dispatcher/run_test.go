package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

const testPayload = `{"command":["echo","hi"]}`

func newWorkerServer(t *testing.T, resp *workerapi.RunJobResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestRunOnce_Success(t *testing.T) {
	workerResp := &workerapi.RunJobResponse{
		Version: 1, TaskID: "t1", JobID: "j1",
		Status: workerapi.StatusCompleted, ExitCode: 0,
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	}
	server := newWorkerServer(t, workerResp)
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "prompt", nil, nil)
	job, _ := mock.CreateJob(ctx, task.ID, testPayload)
	node, _ := mock.CreateNode(ctx, "node-1")
	makeDispatchable(t, mock, ctx, node, server.URL, "token")

	client := &http.Client{Timeout: 10 * time.Second}
	err := RunOnce(ctx, mock, client, 10*time.Second, slog.Default())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	j, _ := mock.GetJobByID(ctx, job.ID)
	if j.Status != models.JobStatusCompleted {
		t.Errorf("job status %s", j.Status)
	}
}

func TestRunOnce_ErrNotFound(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	client := &http.Client{}
	err := RunOnce(ctx, mock, client, 5*time.Second, nil)
	if err == nil {
		t.Fatal("expected ErrNotFound")
	}
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRunOnce_NoDispatchableNodes(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil, nil)
	_, _ = mock.CreateJob(ctx, task.ID, testPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)

	client := &http.Client{}
	err := RunOnce(ctx, mock, client, 5*time.Second, nil)
	if err == nil {
		t.Fatal("expected error when no dispatchable nodes")
	}
	if errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected non-ErrNotFound error, got %v", err)
	}
}

func TestRunOnce_WorkerReturnsFailed(t *testing.T) {
	workerResp := &workerapi.RunJobResponse{
		Version: 1, TaskID: "t1", JobID: "j1",
		Status: workerapi.StatusFailed, ExitCode: 1,
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	}
	server := newWorkerServer(t, workerResp)
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil, nil)
	job, _ := mock.CreateJob(ctx, task.ID, testPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchable(t, mock, ctx, node, server.URL, "token")

	client := &http.Client{Timeout: 10 * time.Second}
	err := RunOnce(ctx, mock, client, 10*time.Second, nil)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	j, _ := mock.GetJobByID(ctx, job.ID)
	if j.Status != models.JobStatusFailed {
		t.Errorf("job status %s", j.Status)
	}
}

func TestRunOnce_BadPayload(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil, nil)
	job, _ := mock.CreateJob(ctx, task.ID, `{"image":"x"}`) // no command
	node, _ := mock.CreateNode(ctx, "n1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	makeDispatchable(t, mock, ctx, node, server.URL, "token")

	client := &http.Client{Timeout: 5 * time.Second}
	err := RunOnce(ctx, mock, client, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	j, _ := mock.GetJobByID(ctx, job.ID)
	if j.Status != models.JobStatusFailed {
		t.Errorf("job status %s", j.Status)
	}
}

func TestRunOnce_WorkerHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil, nil)
	job, _ := mock.CreateJob(ctx, task.ID, testPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchable(t, mock, ctx, node, server.URL, "token")

	client := &http.Client{Timeout: 5 * time.Second}
	err := RunOnce(ctx, mock, client, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	j, _ := mock.GetJobByID(ctx, job.ID)
	if j.Status != models.JobStatusFailed {
		t.Errorf("job status %s", j.Status)
	}
}

func TestRunOnce_WorkerReturnsWrongVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":2,"status":"completed"}`))
	}))
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil, nil)
	job, _ := mock.CreateJob(ctx, task.ID, testPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchable(t, mock, ctx, node, server.URL, "token")

	client := &http.Client{Timeout: 5 * time.Second}
	err := RunOnce(ctx, mock, client, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	j, _ := mock.GetJobByID(ctx, job.ID)
	if j.Status != models.JobStatusFailed {
		t.Errorf("job status %s", j.Status)
	}
}

func TestRunOnce_WorkerTransientEOFThenSuccess(t *testing.T) {
	first := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if first {
			first = false
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, err := hj.Hijack()
				if err == nil {
					_ = conn.Close()
					return
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(&workerapi.RunJobResponse{
			Version: 1, TaskID: "t1", JobID: "j1",
			Status: workerapi.StatusCompleted, ExitCode: 0,
			StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
		})
	}))
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil, nil)
	job, _ := mock.CreateJob(ctx, task.ID, testPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchable(t, mock, ctx, node, server.URL, "token")

	client := &http.Client{Timeout: 5 * time.Second}
	err := RunOnce(ctx, mock, client, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	j, _ := mock.GetJobByID(ctx, job.ID)
	if j.Status != models.JobStatusCompleted {
		t.Errorf("job status %s", j.Status)
	}
}

func makeDispatchable(t *testing.T, mock *testutil.MockDB, ctx context.Context, node *models.Node, workerURL, bearerToken string) {
	t.Helper()
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	_ = mock.UpdateNodeWorkerAPIConfig(ctx, node.ID, workerURL, bearerToken)
	ackAt := time.Now().UTC()
	_ = mock.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
}

// TestRunOnce_CanceledTaskNotOverwritten verifies that a task canceled before the dispatcher
// finishes (race between cancel and job completion) stays in the canceled state.
func TestRunOnce_CanceledTaskNotOverwritten(t *testing.T) {
	workerResp := &workerapi.RunJobResponse{
		Version: 1, TaskID: "t1", JobID: "j1",
		Status: workerapi.StatusCompleted, ExitCode: 0,
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	}
	server := newWorkerServer(t, workerResp)
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "prompt", nil, nil)
	_, _ = mock.CreateJob(ctx, task.ID, testPayload)
	node, _ := mock.CreateNode(ctx, "node-1")
	makeDispatchable(t, mock, ctx, node, server.URL, "token")

	// Simulate a cancel that arrives while the dispatcher is executing.
	_ = mock.UpdateTaskStatus(ctx, task.ID, models.TaskStatusCanceled)

	client := &http.Client{Timeout: 10 * time.Second}
	err := RunOnce(ctx, mock, client, 10*time.Second, slog.Default())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	updated, _ := mock.GetTaskByID(ctx, task.ID)
	if updated.Status != models.TaskStatusCanceled {
		t.Errorf("task status after cancel = %s, want canceled", updated.Status)
	}
}

func TestNormalizeSBAResultSurface(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		final      string
		wantStdout string
	}{
		{name: "maps final answer when stdout empty", stdout: "", final: "model answer", wantStdout: "model answer"},
		{name: "preserves existing stdout", stdout: "worker stdout", final: "model answer", wantStdout: "worker stdout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &workerapi.RunJobResponse{
				Status: workerapi.StatusCompleted,
				Stdout: tt.stdout,
				SbaResult: &sbajob.Result{
					Status:      "success",
					FinalAnswer: tt.final,
				},
			}
			normalizeSBAResultSurface(resp)
			if resp.Stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", resp.Stdout, tt.wantStdout)
			}
		})
	}
}
