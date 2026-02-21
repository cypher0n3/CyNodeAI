package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/inference"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// TaskHandler handles task endpoints.
type TaskHandler struct {
	db              database.Store
	logger          *slog.Logger
	inferenceURL    string
	inferenceModel  string
}

// NewTaskHandler creates a new task handler. inferenceURL and inferenceModel are optional; when set, prompt-mode tasks call the model directly so prompt→model MUST work.
func NewTaskHandler(db database.Store, logger *slog.Logger, inferenceURL, inferenceModel string) *TaskHandler {
	if inferenceModel == "" {
		inferenceModel = "tinyllama"
	}
	return &TaskHandler{
		db:             db,
		logger:         logger,
		inferenceURL:   inferenceURL,
		inferenceModel: inferenceModel,
	}
}

// InputModePrompt is the default: natural-language prompt goes to the PM model.
const InputModePrompt = "prompt"

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

	inputMode := req.InputMode
	if inputMode == "" {
		inputMode = InputModePrompt
	}

	// Prompt mode: send prompt to the model so it MUST work (MVP Phase 1). Prefer orchestrator-side inference when configured.
	if inputMode == InputModePrompt && h.inferenceURL != "" {
		job, inferErr := h.createTaskWithOrchestratorInference(ctx, task.ID, req.Prompt)
		if inferErr == nil {
			_ = job // job already completed
			WriteJSON(w, http.StatusCreated, TaskResponse{
				ID:        task.ID.String(),
				Status:    models.TaskStatusCompleted,
				Prompt:    task.Prompt,
				CreatedAt: task.CreatedAt,
				UpdatedAt: task.UpdatedAt,
			})
			return
		}
		// Fall back to sandbox job path on inference failure
		h.logger.Warn("orchestrator inference failed, falling back to sandbox job", "error", inferErr)
	}

	// Create a single queued job (sandbox path).
	useInference := req.UseInference
	if inputMode == InputModePrompt {
		useInference = true
	}
	payload, err := marshalJobPayload(req.Prompt, useInference, inputMode)
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

// createTaskWithOrchestratorInference calls the PM model with the prompt and stores the result as a completed job.
// Returns the job and nil on success; on failure returns nil, error (caller may fall back to sandbox path).
func (h *TaskHandler) createTaskWithOrchestratorInference(ctx context.Context, taskID uuid.UUID, prompt string) (*models.Job, error) {
	response, err := inference.CallGenerate(ctx, nil, h.inferenceURL, h.inferenceModel, prompt)
	if err != nil {
		return nil, err
	}
	jobID := uuid.New()
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	result := workerapi.RunJobResponse{
		Version:   1,
		TaskID:    taskID.String(),
		JobID:     jobID.String(),
		Status:    workerapi.StatusCompleted,
		ExitCode:  0,
		Stdout:    response,
		Stderr:    "",
		StartedAt: nowStr,
		EndedAt:   nowStr,
		Truncated: workerapi.TruncatedInfo{Stdout: false, Stderr: false},
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	job, err := h.db.CreateJobCompleted(ctx, taskID, jobID, string(resultJSON))
	if err != nil {
		return nil, err
	}
	_ = h.db.UpdateTaskStatus(ctx, taskID, models.TaskStatusCompleted)
	summary := response
	if len(summary) > 200 {
		summary = summary[:200]
	}
	_ = h.db.UpdateTaskSummary(ctx, taskID, summary)
	return job, nil
}

// promptModeModelCommand is the fixed command for "prompt" input_mode: calls Ollama with CYNODE_PROMPT env, prints response or error.
// Handles both single-JSON and NDJSON (streamed) responses from /api/generate.
const promptModeModelCommand = `import os,sys,json,urllib.request
u=os.environ.get('OLLAMA_BASE_URL','http://localhost:11434')+'/api/generate'
p=os.environ.get('CYNODE_PROMPT','')
try:
  r=urllib.request.urlopen(urllib.request.Request(u,data=json.dumps({'model':'tinyllama','prompt':p,'stream':False}).encode(),headers={'Content-Type':'application/json'}),timeout=120)
  raw=r.read().decode()
  out=''
  err_msg=''
  try:
    d=json.loads(raw)
    out=d.get('response','')
    if d.get('error'): err_msg=d.get('error','')
  except json.JSONDecodeError:
    for line in raw.split('\n'):
      line=line.strip()
      if not line: continue
      try: d=json.loads(line)
      except json.JSONDecodeError: continue
      if d.get('error'): err_msg=d.get('error','')
      out+=d.get('response','')
  if err_msg: out='[Ollama error] '+err_msg
  print(out or '(no response)')
except Exception as e: print('[Ollama request failed]', str(e), file=sys.stderr); sys.exit(1)
`

func marshalJobPayload(prompt string, useInference bool, inputMode string) (string, error) {
	// input_mode "prompt": prompt is intended for the PM model; PM decides (e.g. run in sandbox).
	// Stand-in: we dispatch a sandbox job that calls the model with CYNODE_PROMPT; use_inference always true.
	if inputMode == InputModePrompt {
		obj := map[string]any{
			"image":          "python:alpine",
			"command":        []string{"python3", "-c", promptModeModelCommand},
			"env":            map[string]string{"CYNODE_PROMPT": prompt},
			"use_inference":  true, // always true for prompt mode
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
