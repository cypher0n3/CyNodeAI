package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
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
// InputMode: "prompt" (default) = interpret as natural language, use inference; "script" or "commands" = run as literal shell.
type CreateTaskRequest struct {
	Prompt       string `json:"prompt"`
	UseInference bool   `json:"use_inference,omitempty"`
	InputMode    string `json:"input_mode,omitempty"`
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

	// MVP Phase 1: create a single queued job for the task.
	inputMode := req.InputMode
	if inputMode == "" {
		inputMode = "prompt"
	}
	payload, err := marshalJobPayload(req.Prompt, req.UseInference, inputMode)
	if err != nil {
		h.logger.Error("marshal job payload", "error", err)
		WriteInternalError(w, "Failed to create task job")
		return
	}
	if _, err := h.db.CreateJob(ctx, task.ID, payload); err != nil {
		h.logger.Error("create job", "error", err)
		WriteInternalError(w, "Failed to create task job")
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

// promptModeModelCommand is the fixed command for "prompt" input_mode: calls Ollama with CYNODE_PROMPT env, prints response or error.
const promptModeModelCommand = `import os,sys,json,urllib.request
u=os.environ.get('OLLAMA_BASE_URL','http://localhost:11434')+'/api/generate'
p=os.environ.get('CYNODE_PROMPT','')
try:
  r=urllib.request.urlopen(urllib.request.Request(u,data=json.dumps({'model':'tinyllama','prompt':p,'stream':False}).encode(),headers={'Content-Type':'application/json'}),timeout=120)
  d=json.loads(r.read().decode())
  out=d.get('response','')
  if d.get('error'): out='[Ollama error] '+d.get('error','')
  print(out or '(no response)')
except Exception as e: print('[Ollama request failed]', str(e), file=sys.stderr); sys.exit(1)
`

func marshalJobPayload(prompt string, useInference bool, inputMode string) (string, error) {
	// prompt = natural language, use inference by default; script/commands = literal shell (backward compat).
	if inputMode == "prompt" {
		obj := map[string]any{
			"image":          "python:alpine",
			"command":        []string{"python3", "-c", promptModeModelCommand},
			"env":            map[string]string{"CYNODE_PROMPT": prompt},
			"use_inference":  true,
		}
		b, err := json.Marshal(obj)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	// script / commands: run prompt as literal shell command (backward compat).
	obj := map[string]any{
		"image":   "alpine:latest",
		"command": []string{"sh", "-c", prompt},
	}
	if useInference {
		obj["use_inference"] = true
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(b), nil
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
			Result:    job.Result.Ptr(),
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
