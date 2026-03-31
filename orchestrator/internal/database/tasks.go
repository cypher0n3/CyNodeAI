package database

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

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

// ensureUniqueTaskSummaryForUser picks a free summary for the user (base or base-2, base-3, …).
func (db *DB) ensureUniqueTaskSummaryForUser(ctx context.Context, createdBy *uuid.UUID, base string) (string, error) {
	var raw []string
	if err := db.db.WithContext(ctx).Model(&TaskRecord{}).
		Where("created_by = ? AND (summary = ? OR summary LIKE ?)", createdBy, base, base+"-%").
		Pluck("summary", &raw).Error; err != nil {
		return "", wrapErr(err, "list task summaries for uniqueness")
	}
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(base) + `(-[0-9]+)?$`)
	taken := make(map[string]struct{})
	for _, s := range raw {
		if re.MatchString(s) {
			taken[s] = struct{}{}
		}
	}
	if _, exists := taken[base]; !exists {
		return base, nil
	}
	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s-%d", base, n)
		if _, exists := taken[cand]; !exists {
			return cand, nil
		}
	}
}

// CreateTask creates a new task. If taskName is non-nil and non-empty after normalize, it is used as summary and made unique per user; otherwise task_name_###.
//
//nolint:dupl // same transaction wrapper pattern as workflow lease acquire (intentional).
func (db *DB) CreateTask(ctx context.Context, createdBy *uuid.UUID, prompt string, taskName *string, projectID ...*uuid.UUID) (*models.Task, error) {
	var out *models.Task
	err := db.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		d := &DB{db: tx, workerBearerKey: db.workerBearerKey}
		t, err := d.createTaskCore(ctx, createdBy, prompt, taskName, projectID...)
		if err != nil {
			return err
		}
		out = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (db *DB) createTaskCore(ctx context.Context, createdBy *uuid.UUID, prompt string, taskName *string, projectID ...*uuid.UUID) (*models.Task, error) {
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
		var err error
		summary, err = db.ensureUniqueTaskSummaryForUser(ctx, createdBy, summary)
		if err != nil {
			return nil, err
		}
	}
	record := &TaskRecord{
		GormModelUUID: newGormModelUUIDNow(),
		TaskBase: models.TaskBase{
			CreatedBy:     createdBy,
			ProjectID:     effectiveProjectID,
			Status:        models.TaskStatusPending,
			Prompt:        &prompt,
			Summary:       &summary,
			PlanningState: models.PlanningStateDraft,
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
	return getDomainByID(db, ctx, id, "get task by id", (*TaskRecord).ToTask)
}

// getTasksByIDs loads tasks by primary key in one query. IDs missing from the result were not found.
func (db *DB) getTasksByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*models.Task, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]*models.Task{}, nil
	}
	var records []TaskRecord
	if err := db.db.WithContext(ctx).Where("id IN ?", ids).Find(&records).Error; err != nil {
		return nil, wrapErr(err, "get tasks by ids")
	}
	out := make(map[uuid.UUID]*models.Task, len(records))
	for i := range records {
		t := records[i].ToTask()
		out[t.ID] = t
	}
	return out, nil
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

// UpdateTaskMetadata updates tasks.metadata (JSON text / jsonb).
func (db *DB) UpdateTaskMetadata(ctx context.Context, taskID uuid.UUID, metadata *string) error {
	now := time.Now().UTC()
	return db.updateWhere(ctx, &TaskRecord{}, "id", taskID,
		map[string]interface{}{"metadata": metadata, "updated_at": now}, "update task metadata")
}

// UpdateTaskPlanningState sets tasks.planning_state (REQ-ORCHES-0176, 0179).
//
//nolint:dupl // intentional parallel to UpdateNodeCapability (different model/columns).
func (db *DB) UpdateTaskPlanningState(ctx context.Context, taskID uuid.UUID, planningState string) error {
	now := time.Now().UTC()
	return db.updateWhere(ctx, &TaskRecord{}, "id", taskID,
		map[string]interface{}{"planning_state": planningState, "updated_at": now}, "update task planning state")
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
	return getDomainByID(db, ctx, id, "get job by id", (*JobRecord).ToJob)
}

const (
	// DefaultJobPageLimit is the default page size for ListJobsForTask when limit <= 0.
	DefaultJobPageLimit = 50
	// MaxJobPageLimit is the maximum page size for ListJobsForTask.
	MaxJobPageLimit = 100
	jobFetchChunk   = 100
)

func (db *DB) listJobsPage(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]*models.Job, error) {
	if limit <= 0 {
		limit = DefaultJobPageLimit
	}
	if offset < 0 {
		offset = 0
	}
	var records []*JobRecord
	err := db.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error
	if err != nil {
		return nil, wrapErr(err, "list jobs page")
	}
	jobs := make([]*models.Job, len(records))
	for i, r := range records {
		jobs[i] = r.ToJob()
	}
	return jobs, nil
}

// ListJobsForTask returns one page of jobs for a task (created_at ASC) and the total job count.
// limit<=0 uses DefaultJobPageLimit; limit is clamped to MaxJobPageLimit.
func (db *DB) ListJobsForTask(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]*models.Job, int64, error) {
	var total int64
	if err := db.db.WithContext(ctx).Model(&JobRecord{}).Where("task_id = ?", taskID).Count(&total).Error; err != nil {
		return nil, 0, wrapErr(err, "count jobs for task")
	}
	if limit <= 0 {
		limit = DefaultJobPageLimit
	}
	if limit > MaxJobPageLimit {
		limit = MaxJobPageLimit
	}
	if offset < 0 {
		offset = 0
	}
	jobs, err := db.listJobsPage(ctx, taskID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}

// GetJobsByTaskID retrieves all jobs for a task by paging internally (bounded query size per round-trip).
func (db *DB) GetJobsByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.Job, error) {
	var all []*models.Job
	for offset := 0; ; offset += jobFetchChunk {
		jobs, err := db.listJobsPage(ctx, taskID, jobFetchChunk, offset)
		if err != nil {
			return nil, err
		}
		all = append(all, jobs...)
		if len(jobs) < jobFetchChunk {
			break
		}
	}
	return all, nil
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
