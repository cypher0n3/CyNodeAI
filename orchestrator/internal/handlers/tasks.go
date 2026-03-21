package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/dispatcher"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/inference"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/mcptaskbridge"
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
		inferenceModel = "qwen3.5:0.8b"
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

const maxAttachmentPathLen = 2048

var errInvalidProjectID = errors.New("invalid project_id")

func (h *TaskHandler) resolveTaskProjectID(ctx context.Context, userID *uuid.UUID, reqProjectID *string) (*uuid.UUID, error) {
	if reqProjectID != nil && strings.TrimSpace(*reqProjectID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*reqProjectID))
		if err != nil {
			return nil, errInvalidProjectID
		}
		return &parsed, nil
	}
	if userID == nil {
		return nil, nil
	}
	proj, err := h.db.GetOrCreateDefaultProjectForUser(ctx, *userID)
	if err != nil {
		return nil, err
	}
	if proj == nil {
		return nil, nil
	}
	return &proj.ID, nil
}

// persistTaskAttachments validates and persists attachment path references for the task (REQ-ORCHES-0127).
// Returns the list of paths that were stored (for response).
func (h *TaskHandler) persistTaskAttachments(ctx context.Context, taskID uuid.UUID, raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	var stored []string
	for _, p := range raw {
		path := strings.TrimSpace(p)
		if path == "" || len(path) > maxAttachmentPathLen {
			continue
		}
		if _, err := h.db.CreateTaskArtifact(ctx, taskID, path, "", nil); err != nil {
			h.logger.Warn("create task artifact", "task_id", taskID, "path", path, "error", err)
			continue
		}
		stored = append(stored, path)
	}
	return stored
}

func decodeCreateTaskRequest(w http.ResponseWriter, r *http.Request) (userapi.CreateTaskRequest, bool) {
	var req userapi.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "Invalid request body")
		return userapi.CreateTaskRequest{}, false
	}
	if req.Prompt == "" {
		WriteBadRequest(w, "Prompt is required")
		return userapi.CreateTaskRequest{}, false
	}
	return req, true
}

func normalizeInputMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return InputModePrompt
	}
	return mode
}

func (h *TaskHandler) tryCompleteWithOrchestratorInference(
	ctx context.Context,
	w http.ResponseWriter,
	task *models.Task,
	prompt string,
	attachmentPaths []string,
	inputMode string,
) bool {
	if inputMode != InputModePrompt || strings.TrimSpace(h.inferenceURL) == "" {
		return false
	}
	job, err := h.createTaskWithOrchestratorInference(ctx, task.ID, prompt)
	if err == nil {
		_ = job // job already completed
		WriteJSON(w, http.StatusCreated, mcptaskbridge.TaskToResponse(task, mcptaskbridge.TaskStatusToSpec(models.TaskStatusCompleted), attachmentPaths))
		return true
	}
	h.logger.Warn("orchestrator inference failed, falling back to sandbox job", "error", err)
	return false
}

func (h *TaskHandler) createSandboxJob(ctx context.Context, taskID uuid.UUID, prompt string, useInference bool, inputMode string) error {
	payload, err := marshalJobPayload(prompt, useInference, inputMode)
	if err != nil {
		return fmt.Errorf("marshal job payload: %w", err)
	}
	if _, err := h.db.CreateJob(ctx, taskID, payload); err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

// CreateTask handles POST /v1/tasks.
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)

	req, ok := decodeCreateTaskRequest(w, r)
	if !ok {
		return
	}

	projectID, err := h.resolveTaskProjectID(ctx, userID, req.ProjectID)
	if err != nil {
		if errors.Is(err, errInvalidProjectID) {
			WriteBadRequest(w, "Invalid project_id")
			return
		}
		h.logger.Error("resolve task project", "error", err)
		WriteInternalError(w, "Failed to create task")
		return
	}
	task, err := h.db.CreateTask(ctx, userID, req.Prompt, req.TaskName, projectID)
	if err != nil {
		h.logger.Error("create task", "error", err)
		WriteInternalError(w, "Failed to create task")
		return
	}

	attachmentPaths := h.persistTaskAttachments(ctx, task.ID, req.Attachments)

	inputMode := normalizeInputMode(req.InputMode)

	if req.UseSBA {
		if h.createTaskSBA(ctx, w, task, req.Prompt, attachmentPaths) {
			return
		}
	}

	// Prompt mode: prefer orchestrator-side inference when configured; fall back to sandbox job path on error.
	if h.tryCompleteWithOrchestratorInference(ctx, w, task, req.Prompt, attachmentPaths, inputMode) {
		return
	}

	// Create a single queued job (sandbox path).
	useInference := req.UseInference
	if inputMode == InputModePrompt {
		useInference = true
	}
	if err := h.createSandboxJob(ctx, task.ID, req.Prompt, useInference, inputMode); err != nil {
		h.logger.Error("create sandbox job", "error", err)
		WriteInternalError(w, "Failed to create task job")
		return
	}

	WriteJSON(w, http.StatusCreated, mcptaskbridge.TaskToResponse(task, mcptaskbridge.TaskStatusToSpec(task.Status), attachmentPaths))
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
  r=urllib.request.urlopen(urllib.request.Request(u,data=json.dumps({'model':'qwen3.5:0.8b','prompt':p,'stream':False}).encode(),headers={'Content-Type':'application/json'}),timeout=120)
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

// createTaskSBA creates a single job with SBA runner (job_spec_json) and writes 201 or 5xx (P2-10).
func (h *TaskHandler) createTaskSBA(ctx context.Context, w http.ResponseWriter, task *models.Task, prompt string, attachmentPaths []string) bool {
	jobID := uuid.New()
	payload, err := buildSBAJobPayload(task.ID, jobID, prompt, h.inferenceModel)
	if err != nil {
		if strings.Contains(err.Error(), "inference readiness") {
			WriteBadRequest(w, err.Error())
			return true
		}
		h.logger.Error("build SBA job payload", "error", err)
		WriteInternalError(w, "Failed to create SBA job")
		return true
	}
	if _, err := h.db.CreateJobWithID(ctx, task.ID, jobID, payload); err != nil {
		h.logger.Error("create SBA job", "error", err)
		WriteInternalError(w, "Failed to create task job")
		return true
	}
	WriteJSON(w, http.StatusCreated, mcptaskbridge.TaskToResponse(task, mcptaskbridge.TaskStatusToSpec(task.Status), attachmentPaths))
	return true
}

// buildSBAJobPayload returns a job payload with job_spec_json and SBA runner image (P2-10).
func buildSBAJobPayload(taskID, jobID uuid.UUID, prompt, inferenceModel string) (string, error) {
	model := strings.TrimSpace(inferenceModel)
	if model == "" {
		return "", errors.New("SBA inference readiness failed: no allowed model configured")
	}
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           jobID.String(),
		TaskID:          taskID.String(),
		ExecutionMode:   sbajob.ExecutionModeAgentInference,
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 300, MaxOutputBytes: 1024},
		Inference:       &sbajob.InferenceSpec{AllowedModels: []string{model}, Source: "orchestrator"},
		Context:         &sbajob.ContextSpec{TaskContext: prompt},
		Steps:           nil,
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"job_spec_json": string(specJSON),
		"image":         dispatcher.DefaultSBARunnerImage,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

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
// resolveTask resolves a task by UUID string or normalized task name (summary) for the given user.
// Returns the task, or writes an appropriate HTTP error response and returns nil.
func (h *TaskHandler) resolveTask(ctx context.Context, w http.ResponseWriter, userID *uuid.UUID, idOrName string) *models.Task {
	if uid, err := uuid.Parse(idOrName); err == nil {
		task, err := h.db.GetTaskByID(ctx, uid)
		if errors.Is(err, database.ErrNotFound) {
			WriteNotFound(w, "Task not found")
			return nil
		}
		if err != nil {
			h.logger.Error("get task by id", "error", err)
			WriteInternalError(w, "Failed to get task")
			return nil
		}
		if task.CreatedBy == nil || userID == nil || *task.CreatedBy != *userID {
			WriteForbidden(w, "Not task owner")
			return nil
		}
		return task
	}
	if userID == nil {
		WriteNotFound(w, "Task not found")
		return nil
	}
	task, err := h.db.GetTaskBySummary(ctx, *userID, idOrName)
	if errors.Is(err, database.ErrNotFound) {
		WriteNotFound(w, "Task not found")
		return nil
	}
	if err != nil {
		h.logger.Error("get task by name", "error", err)
		WriteInternalError(w, "Failed to get task")
		return nil
	}
	return task
}

func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	task := h.resolveTask(ctx, w, userID, r.PathValue("id"))
	if task == nil {
		return
	}
	paths, _ := h.db.ListArtifactPathsByTaskID(ctx, task.ID)
	WriteJSON(w, http.StatusOK, mcptaskbridge.TaskToResponse(task, mcptaskbridge.TaskStatusToSpec(task.Status), paths))
}

// GetTaskResult handles GET /v1/tasks/{id}/result.
func (h *TaskHandler) GetTaskResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	task := h.resolveTask(ctx, w, userID, r.PathValue("id"))
	if task == nil {
		return
	}
	resp, err := mcptaskbridge.TaskResultForUser(ctx, h.db, task.ID)
	if err != nil {
		h.logger.Error("task result", "error", err)
		WriteInternalError(w, "Failed to get jobs")
		return
	}
	WriteJSON(w, http.StatusOK, resp)
}

func parseListTasksParams(r *http.Request) (limit, offset int, statusFilter, cursor string, errCode int) {
	limit = 50
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := parseInt(l, 1, 200)
		if err != nil {
			return 0, 0, "", "", http.StatusBadRequest
		}
		limit = n
	}
	offset = 0
	if o := r.URL.Query().Get("offset"); o != "" {
		n, err := parseInt(o, 0, 1<<31-1)
		if err != nil {
			return 0, 0, "", "", http.StatusBadRequest
		}
		offset = n
	}
	statusFilter = r.URL.Query().Get("status")
	if statusFilter == "canceled" {
		statusFilter = userapi.StatusCanceled
	}
	cursor = strings.TrimSpace(r.URL.Query().Get("cursor"))
	return limit, offset, statusFilter, cursor, 0
}

// ListTasks handles GET /v1/tasks.
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		WriteUnauthorized(w, "Authentication required")
		return
	}
	limit, offset, statusFilter, cursor, errCode := parseListTasksParams(r)
	if errCode != 0 {
		WriteBadRequest(w, "Invalid limit, offset, or cursor")
		return
	}
	effectiveOffset := offset
	if cursor != "" {
		n, err := strconv.Atoi(cursor)
		if err != nil || n < 0 {
			WriteBadRequest(w, "Invalid limit, offset, or cursor")
			return
		}
		effectiveOffset = n
	}
	resp, err := mcptaskbridge.ListTasksForUser(ctx, h.db, *userID, limit, effectiveOffset, statusFilter, cursor)
	if err != nil {
		h.logger.Error("list tasks", "error", err)
		WriteInternalError(w, "Failed to list tasks")
		return
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

// CancelTask handles POST /v1/tasks/{id}/cancel.
func (h *TaskHandler) CancelTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	task := h.resolveTask(ctx, w, userID, r.PathValue("id"))
	if task == nil {
		return
	}
	taskID := task.ID
	if err := mcptaskbridge.CancelTask(ctx, h.db, taskID); err != nil {
		h.logger.Error("cancel task", "error", err)
		WriteInternalError(w, "Failed to cancel task")
		return
	}
	WriteJSON(w, http.StatusOK, userapi.CancelTaskResponse{TaskID: taskID.String(), Canceled: true})
}

// GetTaskLogs handles GET /v1/tasks/{id}/logs.
func (h *TaskHandler) GetTaskLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	task := h.resolveTask(ctx, w, userID, r.PathValue("id"))
	if task == nil {
		return
	}
	stream := r.URL.Query().Get("stream")
	resp, err := mcptaskbridge.TaskLogsForUser(ctx, h.db, task.ID, stream)
	if err != nil {
		h.logger.Error("get jobs", "error", err)
		WriteInternalError(w, "Failed to get task logs")
		return
	}
	WriteJSON(w, http.StatusOK, resp)
}

// ChatRequest is the request body for POST /v1/chat (orchestrator-specific; not in userapi).
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse is the response for POST /v1/chat (orchestrator-specific).
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
		if t.Status == models.TaskStatusCompleted ||
			t.Status == models.TaskStatusFailed ||
			t.Status == models.TaskStatusCanceled ||
			t.Status == models.TaskStatusSuperseded {
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
	projectID, err := h.resolveTaskProjectID(ctx, userID, nil)
	if err != nil {
		h.logger.Error("chat resolve project", "error", err)
		WriteInternalError(w, "Failed to process message")
		return
	}
	task, err := h.db.CreateTask(ctx, userID, req.Message, nil, projectID)
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
		WriteInternalError(w, "Request canceled")
		return
	}
	WriteJSON(w, http.StatusOK, ChatResponse{Response: out})
}
