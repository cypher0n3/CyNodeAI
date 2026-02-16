package handlers

import (
        "encoding/json"
        "errors"
        "log/slog"
        "net/http"
        "time"

        "github.com/google/uuid"

        "github.com/cypher0n3/cynodeai/internal/database"
)

// TaskHandler handles task endpoints.
type TaskHandler struct {
        db     database.Store
        logger *slog.Logger
}

// NewTaskHandler creates a new task handler.
func NewTaskHandler(db database.Store, logger *slog.Logger) *TaskHandler {
        return &TaskHandler{
                db:     db,
                logger: logger,
        }
}

// CreateTaskRequest represents task creation request.
type CreateTaskRequest struct {
        Prompt string `json:"prompt"`
}

// TaskResponse represents task data in responses.
type TaskResponse struct {
        ID        string    `json:"id"`
        Status    string    `json:"status"`
        Prompt    *string   `json:"prompt,omitempty"`
        Summary   *string   `json:"summary,omitempty"`
        CreatedAt time.Time `json:"created_at"`
        UpdatedAt time.Time `json:"updated_at"`
}

// CreateTask handles POST /v1/tasks.
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        userID := getUserIDFromContext(ctx)

        var req CreateTaskRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                WriteBadRequest(w, "Invalid request body")
                return
        }

        if req.Prompt == "" {
                WriteBadRequest(w, "Prompt is required")
                return
        }

        task, err := h.db.CreateTask(ctx, userID, req.Prompt)
        if err != nil {
                h.logger.Error("create task", "error", err)
                WriteInternalError(w, "Failed to create task")
                return
        }

        WriteJSON(w, http.StatusCreated, TaskResponse{
                ID:        task.ID.String(),
                Status:    task.Status,
                Prompt:    task.Prompt,
                CreatedAt: task.CreatedAt,
                UpdatedAt: task.UpdatedAt,
        })
}

// GetTask handles GET /v1/tasks/{id}.
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        taskIDStr := r.PathValue("id")

        taskID, err := uuid.Parse(taskIDStr)
        if err != nil {
                WriteBadRequest(w, "Invalid task ID")
                return
        }

        task, err := h.db.GetTaskByID(ctx, taskID)
        if errors.Is(err, database.ErrNotFound) {
                WriteNotFound(w, "Task not found")
                return
        }
        if err != nil {
                h.logger.Error("get task", "error", err)
                WriteInternalError(w, "Failed to get task")
                return
        }

        WriteJSON(w, http.StatusOK, TaskResponse{
                ID:        task.ID.String(),
                Status:    task.Status,
                Prompt:    task.Prompt,
                Summary:   task.Summary,
                CreatedAt: task.CreatedAt,
                UpdatedAt: task.UpdatedAt,
        })
}

// TaskResultResponse represents task result data.
type TaskResultResponse struct {
        TaskID string        `json:"task_id"`
        Status string        `json:"status"`
        Jobs   []JobResponse `json:"jobs"`
}

// JobResponse represents job data in responses.
type JobResponse struct {
        ID        string     `json:"id"`
        Status    string     `json:"status"`
        Result    *string    `json:"result,omitempty"`
        StartedAt *time.Time `json:"started_at,omitempty"`
        EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// GetTaskResult handles GET /v1/tasks/{id}/result.
func (h *TaskHandler) GetTaskResult(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        taskIDStr := r.PathValue("id")

        taskID, err := uuid.Parse(taskIDStr)
        if err != nil {
                WriteBadRequest(w, "Invalid task ID")
                return
        }

        task, err := h.db.GetTaskByID(ctx, taskID)
        if errors.Is(err, database.ErrNotFound) {
                WriteNotFound(w, "Task not found")
                return
        }
        if err != nil {
                h.logger.Error("get task", "error", err)
                WriteInternalError(w, "Failed to get task")
                return
        }

        jobs, err := h.db.GetJobsByTaskID(ctx, taskID)
        if err != nil {
                h.logger.Error("get jobs", "error", err)
                WriteInternalError(w, "Failed to get jobs")
                return
        }

        jobResponses := make([]JobResponse, 0, len(jobs))
        for _, job := range jobs {
                jobResponses = append(jobResponses, JobResponse{
                        ID:        job.ID.String(),
                        Status:    job.Status,
                        Result:    job.Result,
                        StartedAt: job.StartedAt,
                        EndedAt:   job.EndedAt,
                })
        }

        WriteJSON(w, http.StatusOK, TaskResultResponse{
                TaskID: task.ID.String(),
                Status: task.Status,
                Jobs:   jobResponses,
        })
}
