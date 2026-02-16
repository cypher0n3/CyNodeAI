package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/internal/models"
)

// --- Task Operations ---

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

	_, err := db.ExecContext(ctx,
		`INSERT INTO tasks (id, created_by, status, prompt, created_at, updated_at)
                 VALUES ($1, $2, $3, $4, $5, $6)`,
		task.ID, task.CreatedBy, task.Status, task.Prompt, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	return task, nil
}

// GetTaskByID retrieves a task by ID.
func (db *DB) GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	task := &models.Task{}
	err := db.QueryRowContext(ctx,
		`SELECT id, created_by, project_id, status, prompt, acceptance_criteria, summary, metadata, created_at, updated_at
                 FROM tasks WHERE id = $1`, id).Scan(
		&task.ID, &task.CreatedBy, &task.ProjectID, &task.Status, &task.Prompt,
		&task.AcceptanceCriteria, &task.Summary, &task.Metadata, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task by id: %w", err)
	}
	return task, nil
}

// UpdateTaskStatus updates a task's status.
func (db *DB) UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE tasks SET status = $2, updated_at = $3 WHERE id = $1`,
		taskID, status, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	return nil
}

// UpdateTaskSummary updates a task's summary.
func (db *DB) UpdateTaskSummary(ctx context.Context, taskID uuid.UUID, summary string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE tasks SET summary = $2, updated_at = $3 WHERE id = $1`,
		taskID, summary, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update task summary: %w", err)
	}
	return nil
}

// ListTasksByUser lists tasks created by a user.
func (db *DB) ListTasksByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, created_by, project_id, status, prompt, acceptance_criteria, summary, metadata, created_at, updated_at
                 FROM tasks WHERE created_by = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tasks by user: %w", err)
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		task := &models.Task{}
		err := rows.Scan(&task.ID, &task.CreatedBy, &task.ProjectID, &task.Status, &task.Prompt,
			&task.AcceptanceCriteria, &task.Summary, &task.Metadata, &task.CreatedAt, &task.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// --- Job Operations ---

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

	_, err := db.ExecContext(ctx,
		`INSERT INTO jobs (id, task_id, status, payload, created_at, updated_at)
                 VALUES ($1, $2, $3, $4, $5, $6)`,
		job.ID, job.TaskID, job.Status, job.Payload, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	return job, nil
}

// GetJobByID retrieves a job by ID.
func (db *DB) GetJobByID(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	job := &models.Job{}
	err := db.QueryRowContext(ctx,
		`SELECT id, task_id, node_id, status, payload, result, lease_id, lease_expires_at, started_at, ended_at, created_at, updated_at
                 FROM jobs WHERE id = $1`, id).Scan(
		&job.ID, &job.TaskID, &job.NodeID, &job.Status, &job.Payload, &job.Result,
		&job.LeaseID, &job.LeaseExpiresAt, &job.StartedAt, &job.EndedAt, &job.CreatedAt, &job.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get job by id: %w", err)
	}
	return job, nil
}

// GetJobsByTaskID retrieves all jobs for a task.
func (db *DB) GetJobsByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.Job, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, task_id, node_id, status, payload, result, lease_id, lease_expires_at, started_at, ended_at, created_at, updated_at
                 FROM jobs WHERE task_id = $1 ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get jobs by task id: %w", err)
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		job := &models.Job{}
		err := rows.Scan(&job.ID, &job.TaskID, &job.NodeID, &job.Status, &job.Payload, &job.Result,
			&job.LeaseID, &job.LeaseExpiresAt, &job.StartedAt, &job.EndedAt, &job.CreatedAt, &job.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// UpdateJobStatus updates a job's status.
func (db *DB) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE jobs SET status = $2, updated_at = $3 WHERE id = $1`,
		jobID, status, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
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
	job := &models.Job{}
	err := db.QueryRowContext(ctx,
		`SELECT id, task_id, node_id, status, payload, result, lease_id, lease_expires_at, started_at, ended_at, created_at, updated_at
                 FROM jobs WHERE status = $1 ORDER BY created_at ASC LIMIT 1`, models.JobStatusQueued).Scan(
		&job.ID, &job.TaskID, &job.NodeID, &job.Status, &job.Payload, &job.Result,
		&job.LeaseID, &job.LeaseExpiresAt, &job.StartedAt, &job.EndedAt, &job.CreatedAt, &job.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get next queued job: %w", err)
	}
	return job, nil
}
