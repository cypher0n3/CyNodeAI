package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/inference"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// TaskHandler handles task endpoints.
type TaskHandler struct {
	db             database.Store
	logger         *slog.Logger
	inferenceURL   string
	inferenceModel string
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

// TaskResponse represents task data in responses (CLI spec: task_id, status, optional task_name).
type TaskResponse struct {
	TaskID    string    `json:"task_id"`
	Status    string    `json:"status"`
	TaskName  *string   `json:"task_name,omitempty"`
	Prompt    *string   `json:"prompt,omitempty"`
	Summary   *string   `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CLI spec status constants (used in responses and tests).
const (
	SpecStatusQueued    = "queued"
	SpecStatusCanceled  = "canceled"
	SpecStatusCompleted = "completed"
	streamParamAll      = "all"
)

// taskStatusToSpec maps internal task status to CLI spec enum (queued, running, completed, failed, canceled).
func taskStatusToSpec(status string) string {
	switch status {
	case models.TaskStatusPending:
		return SpecStatusQueued
	case models.TaskStatusCancelled:
		return SpecStatusCanceled
	default:
		return status
	}
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
				TaskID:    task.ID.String(),
				Status:    taskStatusToSpec(models.TaskStatusCompleted),
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
		TaskID:    task.ID.String(),
		Status:    taskStatusToSpec(task.Status),
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
			"image":         "python:alpine",
			"command":       []string{"python3", "-c", promptModeModelCommand},
			"env":           map[string]string{"CYNODE_PROMPT": prompt},
			"use_inference": true, // always true for prompt mode
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
	userID := getUserIDFromContext(ctx)
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
	if task.CreatedBy == nil || userID == nil || *task.CreatedBy != *userID {
		WriteForbidden(w, "Not task owner")
		return
	}

	WriteJSON(w, http.StatusOK, TaskResponse{
		TaskID:    task.ID.String(),
		Status:    taskStatusToSpec(task.Status),
		TaskName:  task.Summary,
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
	userID := getUserIDFromContext(ctx)
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
	if task.CreatedBy == nil || userID == nil || *task.CreatedBy != *userID {
		WriteForbidden(w, "Not task owner")
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
		Status: taskStatusToSpec(task.Status),
		Jobs:   jobResponses,
	})
}

// ListTasksRequest holds query params for GET /v1/tasks.
type ListTasksRequest struct {
	Limit  int    // default 50, min 1, max 200
	Offset int    // for pagination
	Status string // optional filter: queued|running|completed|failed|canceled
}

// ListTasksResponse is the response for GET /v1/tasks.
type ListTasksResponse struct {
	Tasks      []TaskResponse `json:"tasks"`
	NextOffset *int           `json:"next_offset,omitempty"`
}

func parseListTasksParams(r *http.Request) (limit, offset int, statusFilter string, errCode int) {
	limit = 50
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := parseInt(l, 1, 200)
		if err != nil {
			return 0, 0, "", http.StatusBadRequest
		}
		limit = n
	}
	offset = 0
	if o := r.URL.Query().Get("offset"); o != "" {
		n, err := parseInt(o, 0, 1<<31-1)
		if err != nil {
			return 0, 0, "", http.StatusBadRequest
		}
		offset = n
	}
	return limit, offset, r.URL.Query().Get("status"), 0
}

// ListTasks handles GET /v1/tasks.
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		WriteUnauthorized(w, "Authentication required")
		return
	}
	limit, offset, statusFilter, errCode := parseListTasksParams(r)
	if errCode != 0 {
		WriteBadRequest(w, "Invalid limit or offset")
		return
	}
	tasks, err := h.db.ListTasksByUser(ctx, *userID, limit+1, offset)
	if err != nil {
		h.logger.Error("list tasks", "error", err)
		WriteInternalError(w, "Failed to list tasks")
		return
	}
	hasMore := len(tasks) > limit
	if hasMore {
		tasks = tasks[:limit]
	}
	out := make([]TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		if statusFilter != "" && taskStatusToSpec(t.Status) != statusFilter {
			continue
		}
		out = append(out, TaskResponse{
			TaskID:    t.ID.String(),
			Status:    taskStatusToSpec(t.Status),
			TaskName:  t.Summary,
			Prompt:    t.Prompt,
			Summary:   t.Summary,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		})
	}
	resp := ListTasksResponse{Tasks: out}
	if hasMore {
		next := offset + limit
		resp.NextOffset = &next
	}
	WriteJSON(w, http.StatusOK, resp)
}

func parseInt(s string, minVal, maxVal int) (int, error) {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, err
	}
	if n < minVal || n > maxVal {
		return 0, errors.New("out of range")
	}
	return n, nil
}

// CancelTaskResponse is the response for POST /v1/tasks/{id}/cancel.
type CancelTaskResponse struct {
	TaskID   string `json:"task_id"`
	Canceled bool   `json:"canceled"`
}

// CancelTask handles POST /v1/tasks/{id}/cancel.
func (h *TaskHandler) CancelTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
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
	if task.CreatedBy == nil || userID == nil || *task.CreatedBy != *userID {
		WriteForbidden(w, "Not task owner")
		return
	}
	if err := h.db.UpdateTaskStatus(ctx, taskID, models.TaskStatusCancelled); err != nil {
		h.logger.Error("update task status", "error", err)
		WriteInternalError(w, "Failed to cancel task")
		return
	}
	jobs, err := h.db.GetJobsByTaskID(ctx, taskID)
	if err != nil {
		h.logger.Error("get jobs", "error", err)
		WriteInternalError(w, "Failed to cancel task")
		return
	}
	for _, j := range jobs {
		if j.Status != models.JobStatusCompleted && j.Status != models.JobStatusFailed && j.Status != models.JobStatusCancelled {
			_ = h.db.UpdateJobStatus(ctx, j.ID, models.JobStatusCancelled)
		}
	}
	WriteJSON(w, http.StatusOK, CancelTaskResponse{TaskID: taskID.String(), Canceled: true})
}

// GetTaskLogsResponse is the response for GET /v1/tasks/{id}/logs.
type GetTaskLogsResponse struct {
	TaskID string `json:"task_id"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

func aggregateLogsFromJobs(jobs []*models.Job, stream string) (stdout, stderr string) {
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

// GetTaskLogs handles GET /v1/tasks/{id}/logs.
func (h *TaskHandler) GetTaskLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
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
	if task.CreatedBy == nil || userID == nil || *task.CreatedBy != *userID {
		WriteForbidden(w, "Not task owner")
		return
	}
	stream := r.URL.Query().Get("stream")
	if stream == "" {
		stream = streamParamAll
	}
	jobs, err := h.db.GetJobsByTaskID(ctx, taskID)
	if err != nil {
		h.logger.Error("get jobs", "error", err)
		WriteInternalError(w, "Failed to get task logs")
		return
	}
	stdout, stderr := aggregateLogsFromJobs(jobs, stream)
	WriteJSON(w, http.StatusOK, GetTaskLogsResponse{
		TaskID: taskID.String(),
		Stdout: stdout,
		Stderr: stderr,
	})
}

// ChatRequest is the request body for POST /v1/chat.
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse is the response for POST /v1/chat.
type ChatResponse struct {
	Response string `json:"response"`
}

func (h *TaskHandler) chatResponseFromJob(job *models.Job) string {
	if job.Result.Ptr() == nil {
		return ""
	}
	var res workerapi.RunJobResponse
	if json.Unmarshal([]byte(*job.Result.Ptr()), &res) != nil {
		return ""
	}
	return res.Stdout
}

func (h *TaskHandler) chatPollUntilTerminal(ctx context.Context, taskID uuid.UUID) (response string, errCode int) {
	for {
		t, err := h.db.GetTaskByID(ctx, taskID)
		if err != nil {
			h.logger.Error("chat get task", "error", err)
			return "", http.StatusInternalServerError
		}
		if t.Status == models.TaskStatusCompleted || t.Status == models.TaskStatusFailed || t.Status == models.TaskStatusCancelled {
			jobs, err := h.db.GetJobsByTaskID(ctx, taskID)
			if err != nil {
				return "", http.StatusInternalServerError
			}
			var out strings.Builder
			for _, j := range jobs {
				out.WriteString(h.chatResponseFromJob(j))
			}
			return out.String(), 0
		}
		select {
		case <-ctx.Done():
			return "", http.StatusInternalServerError
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Chat handles POST /v1/chat. Creates a task with message as prompt, waits for terminal status, returns response from job result.
func (h *TaskHandler) Chat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		WriteUnauthorized(w, "Authentication required")
		return
	}
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		WriteBadRequest(w, "Message is required")
		return
	}
	task, err := h.db.CreateTask(ctx, userID, req.Message)
	if err != nil {
		h.logger.Error("chat create task", "error", err)
		WriteInternalError(w, "Failed to process message")
		return
	}
	if h.inferenceURL != "" {
		job, err := h.createTaskWithOrchestratorInference(ctx, task.ID, req.Message)
		if err == nil {
			WriteJSON(w, http.StatusOK, ChatResponse{Response: h.chatResponseFromJob(job)})
			return
		}
		h.logger.Warn("chat orchestrator inference failed, falling back to job", "error", err)
	}
	payload, err := marshalJobPayload(req.Message, true, InputModePrompt)
	if err != nil {
		h.logger.Error("chat marshal payload", "error", err)
		WriteInternalError(w, "Failed to process message")
		return
	}
	if _, err := h.db.CreateJob(ctx, task.ID, payload); err != nil {
		h.logger.Error("chat create job", "error", err)
		WriteInternalError(w, "Failed to process message")
		return
	}
	out, errCode := h.chatPollUntilTerminal(ctx, task.ID)
	if errCode != 0 {
		WriteInternalError(w, "Request cancelled")
		return
	}
	WriteJSON(w, http.StatusOK, ChatResponse{Response: out})
}
