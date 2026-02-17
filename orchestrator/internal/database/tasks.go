package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// --- Task Operations ---

const selectTaskCols = `SELECT id, created_by, project_id, status, prompt, acceptance_criteria, summary, metadata, created_at, updated_at FROM tasks`

func taskPtrs(t *models.Task) []any {
	ptrs := make([]any, 0, 10)
	ptrs = append(ptrs, &t.ID, &t.CreatedBy, &t.ProjectID, &t.Status, &t.Prompt,
		&t.AcceptanceCriteria, &t.Summary, &t.Metadata, &t.CreatedAt, &t.UpdatedAt)
	return ptrs
}

func scanTaskRow(row *sql.Row) (*models.Task, error) { return scanOne(row, taskPtrs) }

func scanTaskRows(r *sql.Rows) (*models.Task, error) { return scanOne(r, taskPtrs) }

// CreateTask creates a new task.
func (db *DB) CreateTask(ctx context.Context, createdBy *uuid.UUID, prompt string) (*models.Task, error) {
	task := &models.Task{
		ID:        uuid.New(),
		CreatedBy: createdBy,
		Status:    models.TaskStatusPending,
		Prompt:    &prompt,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	err := db.execContext(ctx, "create task",
		`INSERT INTO tasks (id, created_by, status, prompt, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		task.ID, task.CreatedBy, task.Status, task.Prompt, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return task, nil
}

// GetTaskByID retrieves a task by ID.
func (db *DB) GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	return queryRowInto(db, ctx, "get task by id", selectTaskCols+` WHERE id = $1`, []any{id}, scanTaskRow)
}

// UpdateTaskStatus updates a task's status.
func (db *DB) UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error {
	return db.execContext(ctx, "update task status",
		`UPDATE tasks SET status = $2, updated_at = $3 WHERE id = $1`,
		taskID, status, time.Now().UTC())
}

// UpdateTaskSummary updates a task's summary.
func (db *DB) UpdateTaskSummary(ctx context.Context, taskID uuid.UUID, summary string) error {
	return db.execContext(ctx, "update task summary",
		`UPDATE tasks SET summary = $2, updated_at = $3 WHERE id = $1`,
		taskID, summary, time.Now().UTC())
}

// ListTasksByUser lists tasks created by a user.
func (db *DB) ListTasksByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	return queryRows(db, ctx, "list tasks by user", selectTaskCols+` WHERE created_by = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, []any{userID, limit, offset}, scanTaskRows)
}

// --- Job Operations ---

const selectJobCols = `SELECT id, task_id, node_id, status, payload, result, lease_id, lease_expires_at, started_at, ended_at, created_at, updated_at FROM jobs`

func jobPtrs(j *models.Job) []any {
	ptrs := make([]any, 0, 12)
	ptrs = append(ptrs, &j.ID, &j.TaskID, &j.NodeID, &j.Status, &j.Payload, &j.Result,
		&j.LeaseID, &j.LeaseExpiresAt, &j.StartedAt, &j.EndedAt, &j.CreatedAt, &j.UpdatedAt)
	return ptrs
}

func scanJobRow(row *sql.Row) (*models.Job, error) { return scanOne(row, jobPtrs) }

func scanJobRows(r *sql.Rows) (*models.Job, error) { return scanOne(r, jobPtrs) }

// CreateJob creates a new job for a task.
func (db *DB) CreateJob(ctx context.Context, taskID uuid.UUID, payload string) (*models.Job, error) {
	job := &models.Job{
		ID:        uuid.New(),
		TaskID:    taskID,
		Status:    models.JobStatusQueued,
		Payload:   &payload,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	return execAndReturn(func() error {
		return db.execContext(ctx, "create job",
			`INSERT INTO jobs (id, task_id, status, payload, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			job.ID, job.TaskID, job.Status, job.Payload, job.CreatedAt, job.UpdatedAt)
	}, job)
}

// GetJobByID retrieves a job by ID.
func (db *DB) GetJobByID(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	return queryRowInto(db, ctx, "get job by id", selectJobCols+` WHERE id = $1`, []any{id}, scanJobRow)
}

// GetJobsByTaskID retrieves all jobs for a task.
func (db *DB) GetJobsByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.Job, error) {
	return queryRows(db, ctx, "get jobs by task id", selectJobCols+` WHERE task_id = $1 ORDER BY created_at ASC`, []any{taskID}, scanJobRows)
}

// UpdateJobStatus updates a job's status.
func (db *DB) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	return db.execContext(ctx, "update job status",
		`UPDATE jobs SET status = $2, updated_at = $3 WHERE id = $1`,
		jobID, status, time.Now().UTC())
}

// AssignJobToNode assigns a job to a node.
func (db *DB) AssignJobToNode(ctx context.Context, jobID, nodeID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx,
		`UPDATE jobs SET node_id = $2, status = $3, started_at = $4, updated_at = $4 WHERE id = $1`,
		jobID, nodeID, models.JobStatusRunning, now)
	if err != nil {
		return fmt.Errorf("assign job to node: %w", err)
	}
	return nil
}

// CompleteJob marks a job as completed with a result.
func (db *DB) CompleteJob(ctx context.Context, jobID uuid.UUID, result, status string) error {
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx,
		`UPDATE jobs SET result = $2, status = $3, ended_at = $4, updated_at = $4 WHERE id = $1`,
		jobID, result, status, now)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	return nil
}

// GetNextQueuedJob retrieves the next queued job.
func (db *DB) GetNextQueuedJob(ctx context.Context) (*models.Job, error) {
	return queryRowInto(db, ctx, "get next queued job", selectJobCols+` WHERE status = $1 ORDER BY created_at ASC LIMIT 1`, []any{models.JobStatusQueued}, scanJobRow)
}
