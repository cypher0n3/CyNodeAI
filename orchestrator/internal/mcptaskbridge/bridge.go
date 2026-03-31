// Package mcptaskbridge provides task list/result/cancel/logs logic shared between the MCP gateway
// and the User API gateway handlers (orchestrator/internal/handlers/tasks.go).
package mcptaskbridge

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

const streamParamAll = "all"

// TaskStatusToSpec maps internal task status to user API status (queued, running, ...).
func TaskStatusToSpec(status string) string {
	switch status {
	case models.TaskStatusPending:
		return userapi.StatusQueued
	case models.TaskStatusCanceled:
		return userapi.StatusCanceled
	case models.TaskStatusSuperseded:
		return userapi.StatusSuperseded
	default:
		return status
	}
}

// TaskToResponse builds the user API task payload (GET/POST /v1/tasks).
func TaskToResponse(t *models.Task, status string, attachmentPaths []string) userapi.TaskResponse {
	resp := userapi.TaskResponse{
		ID:        t.ID.String(),
		TaskID:    t.ID.String(),
		Status:    status,
		TaskName:  t.Summary,
		Prompt:    t.Prompt,
		Summary:   t.Summary,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
	}
	if strings.TrimSpace(t.PlanningState) != "" {
		resp.PlanningState = t.PlanningState
	}
	if len(attachmentPaths) > 0 {
		resp.Attachments = attachmentPaths
	}
	return resp
}

// JobToResponse maps a job row to the user API job shape.
func JobToResponse(job *models.Job) userapi.JobResponse {
	resp := userapi.JobResponse{ID: job.ID.String(), Status: job.Status, Result: job.Result.Ptr()}
	if job.StartedAt != nil {
		s := job.StartedAt.Format(time.RFC3339)
		resp.StartedAt = &s
	}
	if job.EndedAt != nil {
		s := job.EndedAt.Format(time.RFC3339)
		resp.EndedAt = &s
	}
	return resp
}

// ParseListLimitOffset parses limit (default 50, max 200), offset, optional cursor, and status filter.
func ParseListLimitOffset(args map[string]interface{}) (limit, offset int, statusFilter, cursor, errMsg string) {
	limit = 50
	if v := intArg(args, "limit"); v > 0 {
		limit = v
		if limit > 200 {
			limit = 200
		}
	}
	offset = intArg(args, "offset")
	if offset < 0 {
		offset = 0
	}
	cursor = strings.TrimSpace(strArg(args, "cursor"))
	if cursor != "" {
		n, err := strconv.Atoi(cursor)
		if err != nil || n < 0 {
			return 0, 0, "", "", "invalid cursor"
		}
		offset = n
	}
	statusFilter = strings.TrimSpace(strArg(args, "status"))
	if statusFilter == "canceled" {
		statusFilter = userapi.StatusCanceled
	}
	return limit, offset, statusFilter, cursor, ""
}

func strArg(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	v, _ := args[key].(string)
	return v
}

func intArg(args map[string]interface{}, key string) int {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// ListTasksForUser lists tasks for a user (same semantics as GET /v1/tasks).
func ListTasksForUser(ctx context.Context, store database.TaskStore, userID uuid.UUID, limit, offset int, statusFilter, cursor string) (userapi.ListTasksResponse, error) {
	var zero userapi.ListTasksResponse
	effectiveOffset := offset
	tasks, err := store.ListTasksByUser(ctx, userID, limit+1, effectiveOffset)
	if err != nil {
		return zero, err
	}
	hasMore := len(tasks) > limit
	if hasMore {
		tasks = tasks[:limit]
	}
	out := make([]userapi.TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		if statusFilter != "" && TaskStatusToSpec(t.Status) != statusFilter {
			continue
		}
		paths, _ := store.ListArtifactPathsByTaskID(ctx, t.ID)
		out = append(out, TaskToResponse(t, TaskStatusToSpec(t.Status), paths))
	}
	resp := userapi.ListTasksResponse{Tasks: out}
	if hasMore {
		next := effectiveOffset + limit
		resp.NextOffset = &next
		resp.NextCursor = strconv.Itoa(next)
	}
	return resp, nil
}

// TaskResultForUser builds GET /v1/tasks/{id}/result payload.
// limit and offset select a page of jobs (see database.DefaultJobPageLimit / MaxJobPageLimit).
func TaskResultForUser(ctx context.Context, store database.TaskStore, taskID uuid.UUID, limit, offset int) (userapi.TaskResultResponse, error) {
	var zero userapi.TaskResultResponse
	task, err := store.GetTaskByID(ctx, taskID)
	if err != nil {
		return zero, err
	}
	jobs, total, err := store.ListJobsForTask(ctx, task.ID, limit, offset)
	if err != nil {
		return zero, err
	}
	jobResponses := make([]userapi.JobResponse, 0, len(jobs))
	for _, job := range jobs {
		jobResponses = append(jobResponses, JobToResponse(job))
	}
	out := userapi.TaskResultResponse{
		TaskID:     task.ID.String(),
		Status:     TaskStatusToSpec(task.Status),
		Jobs:       jobResponses,
		TotalCount: int(total),
		NextCursor: "",
	}
	if strings.TrimSpace(task.PlanningState) != "" {
		out.PlanningState = task.PlanningState
	}
	if int64(offset)+int64(len(jobs)) < total && len(jobs) > 0 {
		next := offset + len(jobs)
		out.NextOffset = &next
		out.NextCursor = strconv.Itoa(next)
	}
	return out, nil
}

// CancelTask mirrors POST /v1/tasks/{id}/cancel (task status + non-terminal jobs canceled).
func CancelTask(ctx context.Context, store database.TaskStore, taskID uuid.UUID) error {
	if err := store.UpdateTaskStatus(ctx, taskID, models.TaskStatusCanceled); err != nil {
		return err
	}
	jobs, err := store.GetJobsByTaskID(ctx, taskID)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		if j.Status != models.JobStatusCompleted && j.Status != models.JobStatusFailed && j.Status != models.JobStatusCanceled {
			if err := store.UpdateJobStatus(ctx, j.ID, models.JobStatusCanceled); err != nil {
				return err
			}
		}
	}
	return nil
}

// AggregateLogsFromJobs collects stdout/stderr from terminal job results (same as TaskHandler).
func AggregateLogsFromJobs(jobs []*models.Job, stream string) (stdout, stderr string) {
	var outStd, outErr strings.Builder
	for _, job := range jobs {
		if job.Result.Ptr() == nil {
			continue
		}
		var res workerapi.RunJobResponse
		if json.Unmarshal([]byte(*job.Result.Ptr()), &res) != nil {
			continue
		}
		if stream == "stdout" || stream == streamParamAll {
			outStd.WriteString(res.Stdout)
		}
		if stream == "stderr" || stream == streamParamAll {
			outErr.WriteString(res.Stderr)
		}
	}
	return outStd.String(), outErr.String()
}

// TaskLogsForUser builds GET /v1/tasks/{id}/logs payload. Logs are aggregated only for the requested job page.
func TaskLogsForUser(ctx context.Context, store database.TaskStore, taskID uuid.UUID, stream string, limit, offset int) (userapi.TaskLogsResponse, error) {
	var zero userapi.TaskLogsResponse
	if stream == "" {
		stream = streamParamAll
	}
	task, err := store.GetTaskByID(ctx, taskID)
	if err != nil {
		return zero, err
	}
	jobs, total, err := store.ListJobsForTask(ctx, task.ID, limit, offset)
	if err != nil {
		return zero, err
	}
	stdout, stderr := AggregateLogsFromJobs(jobs, stream)
	out := userapi.TaskLogsResponse{
		TaskID:     task.ID.String(),
		Stdout:     stdout,
		Stderr:     stderr,
		TotalCount: int(total),
		NextCursor: "",
	}
	if int64(offset)+int64(len(jobs)) < total && len(jobs) > 0 {
		next := offset + len(jobs)
		out.NextOffset = &next
		out.NextCursor = strconv.Itoa(next)
	}
	return out, nil
}
