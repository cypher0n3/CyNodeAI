package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/internal/models"
)

// --- Task Tests ---

func TestCreateTask(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	userID := uuid.New()
	prompt := "test prompt"

	mock.ExpectExec(`INSERT INTO tasks`).
		WithArgs(sqlmock.AnyArg(), &userID, models.TaskStatusPending, &prompt, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	task, err := db.CreateTask(context.Background(), &userID, prompt)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if task.Status != models.TaskStatusPending {
		t.Errorf("expected status pending, got %s", task.Status)
	}
}

func TestCreateTaskError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`INSERT INTO tasks`).
		WillReturnError(errors.New("db error"))

	_, err := db.CreateTask(context.Background(), nil, "prompt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetTaskByID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()
	prompt := "test prompt"

	rows := sqlmock.NewRows([]string{"id", "created_by", "project_id", "status", "prompt", "acceptance_criteria", "summary", "metadata", "created_at", "updated_at"}).
		AddRow(taskID, &userID, nil, models.TaskStatusPending, &prompt, nil, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM tasks WHERE id`).
		WithArgs(taskID).
		WillReturnRows(rows)

	task, err := db.GetTaskByID(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if task.ID != taskID {
		t.Errorf("expected taskID %v, got %v", taskID, task.ID)
	}
}

func TestGetTaskByIDNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	mock.ExpectQuery(`SELECT .* FROM tasks WHERE id`).
		WithArgs(taskID).
		WillReturnError(sql.ErrNoRows)

	_, err := db.GetTaskByID(context.Background(), taskID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	mock.ExpectExec(`UPDATE tasks SET status`).
		WithArgs(taskID, models.TaskStatusCompleted, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := db.UpdateTaskStatus(context.Background(), taskID, models.TaskStatusCompleted)
	if err != nil {
		t.Fatalf("UpdateTaskStatus failed: %v", err)
	}
}

func TestUpdateTaskSummary(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	mock.ExpectExec(`UPDATE tasks SET summary`).
		WithArgs(taskID, "new summary", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := db.UpdateTaskSummary(context.Background(), taskID, "new summary")
	if err != nil {
		t.Fatalf("UpdateTaskSummary failed: %v", err)
	}
}

func TestListTasksByUser(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	userID := uuid.New()
	taskID := uuid.New()
	now := time.Now().UTC()
	prompt := "test prompt"

	rows := sqlmock.NewRows([]string{"id", "created_by", "project_id", "status", "prompt", "acceptance_criteria", "summary", "metadata", "created_at", "updated_at"}).
		AddRow(taskID, &userID, nil, models.TaskStatusPending, &prompt, nil, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM tasks WHERE created_by`).
		WithArgs(userID, 10, 0).
		WillReturnRows(rows)

	tasks, err := db.ListTasksByUser(context.Background(), userID, 10, 0)
	if err != nil {
		t.Fatalf("ListTasksByUser failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

func TestListTasksByUserEmpty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	userID := uuid.New()
	rows := sqlmock.NewRows([]string{"id", "created_by", "project_id", "status", "prompt", "acceptance_criteria", "summary", "metadata", "created_at", "updated_at"})

	mock.ExpectQuery(`SELECT .* FROM tasks WHERE created_by`).
		WithArgs(userID, 10, 0).
		WillReturnRows(rows)

	tasks, err := db.ListTasksByUser(context.Background(), userID, 10, 0)
	if err != nil {
		t.Fatalf("ListTasksByUser failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

// --- Job Tests ---

func TestCreateJob(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	payload := `{"test": "data"}`

	mock.ExpectExec(`INSERT INTO jobs`).
		WithArgs(sqlmock.AnyArg(), taskID, models.JobStatusQueued, payload, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	job, err := db.CreateJob(context.Background(), taskID, payload)
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	if job.TaskID != taskID {
		t.Errorf("expected taskID %v, got %v", taskID, job.TaskID)
	}
}

func TestGetJobByID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	jobID := uuid.New()
	taskID := uuid.New()
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"id", "task_id", "node_id", "status", "payload", "result", "lease_id", "lease_expires_at", "started_at", "ended_at", "created_at", "updated_at"}).
		AddRow(jobID, taskID, nil, models.JobStatusQueued, "{}", nil, nil, nil, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM jobs WHERE id`).
		WithArgs(jobID).
		WillReturnRows(rows)

	job, err := db.GetJobByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("GetJobByID failed: %v", err)
	}
	if job.ID != jobID {
		t.Errorf("expected jobID %v, got %v", jobID, job.ID)
	}
}

func TestGetJobByIDNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	jobID := uuid.New()
	mock.ExpectQuery(`SELECT .* FROM jobs WHERE id`).
		WithArgs(jobID).
		WillReturnError(sql.ErrNoRows)

	_, err := db.GetJobByID(context.Background(), jobID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetJobsByTaskID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	jobID := uuid.New()
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"id", "task_id", "node_id", "status", "payload", "result", "lease_id", "lease_expires_at", "started_at", "ended_at", "created_at", "updated_at"}).
		AddRow(jobID, taskID, nil, models.JobStatusQueued, "{}", nil, nil, nil, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM jobs WHERE task_id`).
		WithArgs(taskID).
		WillReturnRows(rows)

	jobs, err := db.GetJobsByTaskID(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetJobsByTaskID failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
}

func TestUpdateJobStatus(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	jobID := uuid.New()
	mock.ExpectExec(`UPDATE jobs SET status`).
		WithArgs(jobID, models.JobStatusRunning, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := db.UpdateJobStatus(context.Background(), jobID, models.JobStatusRunning)
	if err != nil {
		t.Fatalf("UpdateJobStatus failed: %v", err)
	}
}

func TestAssignJobToNode(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	jobID := uuid.New()
	nodeID := uuid.New()

	mock.ExpectExec(`UPDATE jobs SET node_id`).
		WithArgs(jobID, nodeID, models.JobStatusRunning, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := db.AssignJobToNode(context.Background(), jobID, nodeID)
	if err != nil {
		t.Fatalf("AssignJobToNode failed: %v", err)
	}
}

func TestCompleteJob(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	jobID := uuid.New()
	result := `{"output": "test"}`

	mock.ExpectExec(`UPDATE jobs SET result`).
		WithArgs(jobID, result, models.JobStatusCompleted, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := db.CompleteJob(context.Background(), jobID, result, models.JobStatusCompleted)
	if err != nil {
		t.Fatalf("CompleteJob failed: %v", err)
	}
}

func TestGetNextQueuedJob(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	jobID := uuid.New()
	taskID := uuid.New()
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"id", "task_id", "node_id", "status", "payload", "result", "lease_id", "lease_expires_at", "started_at", "ended_at", "created_at", "updated_at"}).
		AddRow(jobID, taskID, nil, models.JobStatusQueued, "{}", nil, nil, nil, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM jobs WHERE status = .* ORDER BY created_at`).
		WithArgs(models.JobStatusQueued).
		WillReturnRows(rows)

	job, err := db.GetNextQueuedJob(context.Background())
	if err != nil {
		t.Fatalf("GetNextQueuedJob failed: %v", err)
	}
	if job.ID != jobID {
		t.Errorf("expected jobID %v, got %v", jobID, job.ID)
	}
}

func TestGetNextQueuedJobNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .* FROM jobs WHERE status = .* ORDER BY created_at`).
		WithArgs(models.JobStatusQueued).
		WillReturnError(sql.ErrNoRows)

	_, err := db.GetNextQueuedJob(context.Background())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Additional Error Tests ---

func TestGetTaskByIDError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	mock.ExpectQuery(`SELECT .* FROM tasks WHERE id`).
		WithArgs(taskID).
		WillReturnError(errors.New("db error"))

	_, err := db.GetTaskByID(context.Background(), taskID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateTaskStatusError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	mock.ExpectExec(`UPDATE tasks SET status`).
		WillReturnError(errors.New("db error"))

	err := db.UpdateTaskStatus(context.Background(), taskID, models.TaskStatusCompleted)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateTaskSummaryError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	mock.ExpectExec(`UPDATE tasks SET summary`).
		WillReturnError(errors.New("db error"))

	err := db.UpdateTaskSummary(context.Background(), taskID, "summary")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListTasksByUserError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	userID := uuid.New()
	mock.ExpectQuery(`SELECT .* FROM tasks WHERE created_by`).
		WillReturnError(errors.New("db error"))

	_, err := db.ListTasksByUser(context.Background(), userID, 10, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateJobError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	mock.ExpectExec(`INSERT INTO jobs`).
		WillReturnError(errors.New("db error"))

	_, err := db.CreateJob(context.Background(), taskID, "{}")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetJobByIDError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	jobID := uuid.New()
	mock.ExpectQuery(`SELECT .* FROM jobs WHERE id`).
		WithArgs(jobID).
		WillReturnError(errors.New("db error"))

	_, err := db.GetJobByID(context.Background(), jobID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetJobsByTaskIDError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	mock.ExpectQuery(`SELECT .* FROM jobs WHERE task_id`).
		WillReturnError(errors.New("db error"))

	_, err := db.GetJobsByTaskID(context.Background(), taskID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateJobStatusError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	jobID := uuid.New()
	mock.ExpectExec(`UPDATE jobs SET status`).
		WillReturnError(errors.New("db error"))

	err := db.UpdateJobStatus(context.Background(), jobID, models.JobStatusRunning)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAssignJobToNodeError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE jobs SET node_id`).
		WillReturnError(errors.New("db error"))

	err := db.AssignJobToNode(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCompleteJobError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE jobs SET result`).
		WillReturnError(errors.New("db error"))

	err := db.CompleteJob(context.Background(), uuid.New(), "{}", models.JobStatusCompleted)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetNextQueuedJobError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT .* FROM jobs WHERE status`).
		WillReturnError(errors.New("db error"))

	_, err := db.GetNextQueuedJob(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetJobsByTaskIDEmpty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	taskID := uuid.New()
	rows := sqlmock.NewRows([]string{"id", "task_id", "node_id", "status", "payload", "result", "lease_id", "lease_expires_at", "started_at", "ended_at", "created_at", "updated_at"})

	mock.ExpectQuery(`SELECT .* FROM jobs WHERE task_id`).
		WithArgs(taskID).
		WillReturnRows(rows)

	jobs, err := db.GetJobsByTaskID(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetJobsByTaskID failed: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}
