package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// CreateTask creates a new task with a task name in the format task_name_### (e.g. task_name_001).
func (db *DB) CreateTask(ctx context.Context, createdBy *uuid.UUID, prompt string) (*models.Task, error) {
	var count int64
	if createdBy != nil {
		_ = db.db.WithContext(ctx).Model(&models.Task{}).Where("created_by = ?", createdBy).Count(&count).Error
	}
	taskName := fmt.Sprintf("task_name_%03d", count+1)
	task := &models.Task{
		ID:        uuid.New(),
		CreatedBy: createdBy,
		Status:    models.TaskStatusPending,
		Prompt:    &prompt,
		Summary:   &taskName,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.createRecord(ctx, task, "create task"); err != nil {
		return nil, err
	}
	return task, nil
}

// GetTaskByID retrieves a task by ID.
func (db *DB) GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	return getByID[models.Task](db, ctx, id, "get task by id")
}

// UpdateTaskStatus updates a task's status.
func (db *DB) UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error {
	return db.updateWhere(ctx, &models.Task{}, "id", taskID,
		map[string]interface{}{"status": status}, "update task status")
}

// UpdateTaskSummary updates a task's summary.
func (db *DB) UpdateTaskSummary(ctx context.Context, taskID uuid.UUID, summary string) error {
	return db.updateWhere(ctx, &models.Task{}, "id", taskID,
		map[string]interface{}{"summary": summary}, "update task summary")
}

// ListTasksByUser lists tasks created by a user.
func (db *DB) ListTasksByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	var tasks []*models.Task
	err := db.db.WithContext(ctx).
		Where("created_by = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&tasks).Error
	if err != nil {
		return nil, wrapErr(err, "list tasks by user")
	}
	return tasks, nil
}

// CreateJob creates a new job for a task.
func (db *DB) CreateJob(ctx context.Context, taskID uuid.UUID, payload string) (*models.Job, error) {
	job := &models.Job{
		ID:        uuid.New(),
		TaskID:    taskID,
		Status:    models.JobStatusQueued,
		Payload:   models.NewJSONBString(&payload),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.createRecord(ctx, job, "create job"); err != nil {
		return nil, err
	}
	return job, nil
}

// CreateJobCompleted creates a job that is already completed (orchestrator-side inference).
// The job is never queued so the dispatcher does not pick it up.
func (db *DB) CreateJobCompleted(ctx context.Context, taskID, jobID uuid.UUID, result string) (*models.Job, error) {
	now := time.Now().UTC()
	emptyPayload := "{}"
	job := &models.Job{
		ID:        jobID,
		TaskID:    taskID,
		Status:    models.JobStatusCompleted,
		Payload:   models.NewJSONBString(&emptyPayload),
		Result:    models.NewJSONBString(&result),
		EndedAt:   &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.createRecord(ctx, job, "create job completed"); err != nil {
		return nil, err
	}
	return job, nil
}

// GetJobByID retrieves a job by ID.
func (db *DB) GetJobByID(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	return getByID[models.Job](db, ctx, id, "get job by id")
}

// GetJobsByTaskID retrieves all jobs for a task.
func (db *DB) GetJobsByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.Job, error) {
	var jobs []*models.Job
	err := db.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("created_at ASC").
		Find(&jobs).Error
	if err != nil {
		return nil, wrapErr(err, "get jobs by task id")
	}
	return jobs, nil
}

// UpdateJobStatus updates a job's status.
func (db *DB) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	return db.updateWhere(ctx, &models.Job{}, "id", jobID,
		map[string]interface{}{"status": status}, "update job status")
}

// AssignJobToNode assigns a job to a node.
func (db *DB) AssignJobToNode(ctx context.Context, jobID, nodeID uuid.UUID) error {
	now := time.Now().UTC()
	err := db.db.WithContext(ctx).Model(&models.Job{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"node_id":    nodeID,
			"status":     models.JobStatusRunning,
			"started_at": now,
			"updated_at": now,
		}).Error
	return wrapErr(err, "assign job to node")
}

// CompleteJob marks a job as completed with a result.
func (db *DB) CompleteJob(ctx context.Context, jobID uuid.UUID, result, status string) error {
	now := time.Now().UTC()
	err := db.db.WithContext(ctx).Model(&models.Job{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"result":     models.NewJSONBString(&result),
			"status":     status,
			"ended_at":   now,
			"updated_at": now,
		}).Error
	return wrapErr(err, "complete job")
}

// GetNextQueuedJob retrieves the next queued job.
func (db *DB) GetNextQueuedJob(ctx context.Context) (*models.Job, error) {
	var job models.Job
	err := db.db.WithContext(ctx).
		Where("status = ?", models.JobStatusQueued).
		Order("created_at ASC").
		Limit(1).
		First(&job).Error
	if err != nil {
		return nil, wrapErr(err, "get next queued job")
	}
	return &job, nil
}
