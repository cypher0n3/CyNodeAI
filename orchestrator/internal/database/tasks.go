package database

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

var taskNameNormalizeRe = regexp.MustCompile(`[^a-z0-9]+`)

// normalizeTaskName normalizes a user-supplied task name per project_manager_agent.md Task Naming: lowercase, single dashes.
func normalizeTaskName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	s = taskNameNormalizeRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// CreateTask creates a new task. If taskName is non-nil and non-empty after normalize, it is used as summary and made unique per user; otherwise task_name_###.
func (db *DB) CreateTask(ctx context.Context, createdBy *uuid.UUID, prompt string, taskName *string, projectID ...*uuid.UUID) (*models.Task, error) {
	var effectiveProjectID *uuid.UUID
	if len(projectID) > 0 {
		effectiveProjectID = projectID[0]
	}
	summary := ""
	if taskName != nil {
		summary = normalizeTaskName(*taskName)
	}
	if summary == "" {
		var count int64
		if createdBy != nil {
			_ = db.db.WithContext(ctx).Model(&TaskRecord{}).Where("created_by = ?", createdBy).Count(&count).Error
		}
		summary = fmt.Sprintf("task_name_%03d", count+1)
	} else if createdBy != nil {
		// Ensure uniqueness: if same user has a task with this summary, append -2, -3, ...
		base := summary
		for n := 2; ; n++ {
			var exists int64
			_ = db.db.WithContext(ctx).Model(&TaskRecord{}).Where("created_by = ? AND summary = ?", createdBy, summary).Limit(1).Count(&exists).Error
			if exists == 0 {
				break
			}
			summary = fmt.Sprintf("%s-%d", base, n)
		}
	}
	record := &TaskRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		TaskBase: models.TaskBase{
			CreatedBy: createdBy,
			ProjectID: effectiveProjectID,
			Status:    models.TaskStatusPending,
			Prompt:    &prompt,
			Summary:   &summary,
		},
	}
	if err := db.createRecord(ctx, record, "create task"); err != nil {
		return nil, err
	}
	return record.ToTask(), nil
}

// GetOrCreateDefaultProjectForUser gets the deterministic default project for the user, creating it when missing.
func (db *DB) GetOrCreateDefaultProjectForUser(ctx context.Context, userID uuid.UUID) (*models.Project, error) {
	slug := "default-" + userID.String()
	now := time.Now().UTC()
	var record ProjectRecord
	err := db.db.WithContext(ctx).
		Where(&ProjectRecord{ProjectBase: models.ProjectBase{Slug: slug}}).
		Attrs(ProjectRecord{
			GormModelUUID: gormmodel.GormModelUUID{
				ID:        uuid.New(),
				CreatedAt: now,
				UpdatedAt: now,
			},
			ProjectBase: models.ProjectBase{
				DisplayName: "Default Project",
				IsActive:    true,
			},
		}).
		FirstOrCreate(&record).Error
	if err != nil {
		return nil, wrapErr(err, "get or create default project")
	}
	return record.ToProject(), nil
}

// GetTaskByID retrieves a task by ID.
func (db *DB) GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	record, err := getByID[TaskRecord](db, ctx, id, "get task by id")
	if err != nil {
		return nil, err
	}
	return record.ToTask(), nil
}

// GetTaskBySummary retrieves the most recently created task matching summary for the given user.
// Returns ErrNotFound when no match exists.
func (db *DB) GetTaskBySummary(ctx context.Context, userID uuid.UUID, summary string) (*models.Task, error) {
	var record TaskRecord
	err := db.db.WithContext(ctx).
		Where("created_by = ? AND summary = ?", userID, summary).
		Order("created_at DESC").
		Limit(1).
		First(&record).Error
	if err != nil {
		return nil, wrapErr(err, "get task by summary")
	}
	return record.ToTask(), nil
}

// UpdateTaskStatus updates a task's status. Terminal statuses (completed, failed, canceled,
// superseded) are never overwritten; the update is silently skipped if the task is already
// in a terminal state so that race conditions between the dispatcher and cancel cannot
// revert a canceled task back to running or failed.
func (db *DB) UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error {
	closed := isTerminalTaskStatus(status)
	now := time.Now().UTC()
	terminalStatuses := []string{
		models.TaskStatusCompleted,
		models.TaskStatusFailed,
		models.TaskStatusCanceled,
		models.TaskStatusSuperseded,
	}
	return wrapErr(
		db.db.WithContext(ctx).Model(&TaskRecord{}).
			Where("id = ? AND status NOT IN ?", taskID, terminalStatuses).
			Updates(map[string]interface{}{
				"status":     status,
				"closed":     closed,
				"updated_at": now,
			}).Error,
		"update task status",
	)
}

func isTerminalTaskStatus(status string) bool {
	switch status {
	case models.TaskStatusCompleted, models.TaskStatusFailed, models.TaskStatusCanceled, models.TaskStatusSuperseded:
		return true
	default:
		return false
	}
}

// UpdateTaskSummary updates a task's summary.
func (db *DB) UpdateTaskSummary(ctx context.Context, taskID uuid.UUID, summary string) error {
	return db.updateWhere(ctx, &TaskRecord{}, "id", taskID,
		map[string]interface{}{"summary": summary}, "update task summary")
}

// ListTasksByUser lists tasks created by a user.
func (db *DB) ListTasksByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	var records []*TaskRecord
	err := db.db.WithContext(ctx).
		Where("created_by = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&records).Error
	if err != nil {
		return nil, wrapErr(err, "list tasks by user")
	}
	tasks := make([]*models.Task, len(records))
	for i, r := range records {
		tasks[i] = r.ToTask()
	}
	return tasks, nil
}

// CreateJob creates a new job for a task.
func (db *DB) CreateJob(ctx context.Context, taskID uuid.UUID, payload string) (*models.Job, error) {
	record := &JobRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		JobBase: models.JobBase{
			TaskID:  taskID,
			Status:  models.JobStatusQueued,
			Payload: models.NewJSONBString(&payload),
		},
	}
	if err := db.createRecord(ctx, record, "create job"); err != nil {
		return nil, err
	}
	return record.ToJob(), nil
}

// CreateJobWithID creates a new job with the given ID (used when payload must reference job_id, e.g. SBA job spec).
func (db *DB) CreateJobWithID(ctx context.Context, taskID, jobID uuid.UUID, payload string) (*models.Job, error) {
	record := &JobRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        jobID,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		JobBase: models.JobBase{
			TaskID:  taskID,
			Status:  models.JobStatusQueued,
			Payload: models.NewJSONBString(&payload),
		},
	}
	if err := db.createRecord(ctx, record, "create job with id"); err != nil {
		return nil, err
	}
	return record.ToJob(), nil
}

// CreateJobCompleted creates a job that is already completed (orchestrator-side inference).
// The job is never queued so the dispatcher does not pick it up.
func (db *DB) CreateJobCompleted(ctx context.Context, taskID, jobID uuid.UUID, result string) (*models.Job, error) {
	now := time.Now().UTC()
	emptyPayload := "{}"
	record := &JobRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        jobID,
			CreatedAt: now,
			UpdatedAt: now,
		},
		JobBase: models.JobBase{
			TaskID:  taskID,
			Status:  models.JobStatusCompleted,
			Payload: models.NewJSONBString(&emptyPayload),
			Result:  models.NewJSONBString(&result),
			EndedAt: &now,
		},
	}
	if err := db.createRecord(ctx, record, "create job completed"); err != nil {
		return nil, err
	}
	return record.ToJob(), nil
}

// GetJobByID retrieves a job by ID.
func (db *DB) GetJobByID(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	record, err := getByID[JobRecord](db, ctx, id, "get job by id")
	if err != nil {
		return nil, err
	}
	return record.ToJob(), nil
}

// GetJobsByTaskID retrieves all jobs for a task.
func (db *DB) GetJobsByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.Job, error) {
	var records []*JobRecord
	err := db.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("created_at ASC").
		Find(&records).Error
	if err != nil {
		return nil, wrapErr(err, "get jobs by task id")
	}
	jobs := make([]*models.Job, len(records))
	for i, r := range records {
		jobs[i] = r.ToJob()
	}
	return jobs, nil
}

// UpdateJobStatus updates a job's status.
func (db *DB) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	return db.updateWhere(ctx, &JobRecord{}, "id", jobID,
		map[string]interface{}{"status": status}, "update job status")
}

// AssignJobToNode assigns a job to a node.
func (db *DB) AssignJobToNode(ctx context.Context, jobID, nodeID uuid.UUID) error {
	now := time.Now().UTC()
	err := db.db.WithContext(ctx).Model(&JobRecord{}).
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
	err := db.db.WithContext(ctx).Model(&JobRecord{}).
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
	var record JobRecord
	err := db.db.WithContext(ctx).
		Where("status = ?", models.JobStatusQueued).
		Order("created_at ASC").
		Limit(1).
		First(&record).Error
	if err != nil {
		return nil, wrapErr(err, "get next queued job")
	}
	return record.ToJob(), nil
}
