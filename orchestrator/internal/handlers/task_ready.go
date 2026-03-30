package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/mcptaskbridge"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

const taskMetaKeyPendingExec = "cynode_pending_exec"

// pendingExecFlags is stored in tasks.metadata under taskMetaKeyPendingExec until the task is marked ready.
type pendingExecFlags struct {
	UseSBA       bool   `json:"use_sba"`
	UseInference bool   `json:"use_inference"`
	InputMode    string `json:"input_mode"`
}

func mergePendingExecMetadata(existing *string, req userapi.CreateTaskRequest) *string {
	flags := pendingExecFlags{
		UseSBA:       req.UseSBA,
		UseInference: req.UseInference,
		InputMode:    normalizeInputMode(req.InputMode),
	}
	var root map[string]interface{}
	if existing != nil && strings.TrimSpace(*existing) != "" {
		_ = json.Unmarshal([]byte(*existing), &root)
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	b, _ := json.Marshal(flags)
	var raw map[string]interface{}
	_ = json.Unmarshal(b, &raw)
	root[taskMetaKeyPendingExec] = raw
	out, err := json.Marshal(root)
	if err != nil {
		return existing
	}
	s := string(out)
	return &s
}

func parsePendingExecFromMetadata(meta *string) pendingExecFlags {
	def := pendingExecFlags{InputMode: InputModePrompt}
	if meta == nil || strings.TrimSpace(*meta) == "" {
		return def
	}
	var root map[string]json.RawMessage
	if json.Unmarshal([]byte(*meta), &root) != nil {
		return def
	}
	raw, ok := root[taskMetaKeyPendingExec]
	if !ok {
		return def
	}
	var p pendingExecFlags
	if json.Unmarshal(raw, &p) != nil {
		return def
	}
	if strings.TrimSpace(p.InputMode) == "" {
		p.InputMode = InputModePrompt
	}
	return p
}

// dispatchTaskJobsCore runs SBA, orchestrator inference, or sandbox jobs after planning_state=ready.
func (h *TaskHandler) dispatchTaskJobsCore(ctx context.Context, task *models.Task, exec pendingExecFlags) (*models.Task, error) {
	if task.Prompt == nil {
		return nil, fmt.Errorf("task has no prompt")
	}
	prompt := *task.Prompt
	inputMode := normalizeInputMode(exec.InputMode)

	if exec.UseSBA {
		jobID := uuid.New()
		payload, err := buildSBAJobPayload(task.ID, jobID, prompt, h.inferenceModel)
		if err != nil {
			return nil, err
		}
		if _, err := h.db.CreateJobWithID(ctx, task.ID, jobID, payload); err != nil {
			return nil, err
		}
		return h.db.GetTaskByID(ctx, task.ID)
	}

	if inputMode == InputModePrompt && strings.TrimSpace(h.inferenceURL) != "" {
		_, infErr := h.createTaskWithOrchestratorInference(ctx, task.ID, prompt)
		if infErr == nil {
			return h.db.GetTaskByID(ctx, task.ID)
		}
		h.logger.Warn("orchestrator inference failed, falling back to sandbox job", "error", infErr)
	}

	useInference := exec.UseInference
	if inputMode == InputModePrompt {
		useInference = true
	}
	if err := h.createSandboxJob(ctx, task.ID, prompt, useInference, inputMode); err != nil {
		return nil, err
	}
	return h.db.GetTaskByID(ctx, task.ID)
}

// PostTaskReady handles POST /v1/tasks/{id}/ready (REQ-ORCHES-0179): draft -> ready and start job execution.
func (h *TaskHandler) PostTaskReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	task := h.resolveTask(ctx, w, userID, r.PathValue("id"))
	if task == nil {
		return
	}
	if task.PlanningState == models.PlanningStateReady {
		paths, _ := h.db.ListArtifactPathsByTaskID(ctx, task.ID)
		WriteJSON(w, http.StatusOK, mcptaskbridge.TaskToResponse(task, mcptaskbridge.TaskStatusToSpec(task.Status), paths))
		return
	}
	if task.PlanningState != models.PlanningStateDraft {
		WriteConflict(w, "task is not in draft planning state")
		return
	}
	jobs, err := h.db.GetJobsByTaskID(ctx, task.ID)
	if err != nil {
		h.logger.Error("list jobs", "error", err)
		WriteInternalError(w, "Failed to load task")
		return
	}
	if len(jobs) > 0 {
		WriteConflict(w, "task already has jobs")
		return
	}
	if err := h.db.UpdateTaskPlanningState(ctx, task.ID, models.PlanningStateReady); err != nil {
		h.logger.Error("update planning state", "error", err)
		WriteInternalError(w, "Failed to update task")
		return
	}
	task, err = h.db.GetTaskByID(ctx, task.ID)
	if err != nil {
		h.logger.Error("get task", "error", err)
		WriteInternalError(w, "Failed to load task")
		return
	}
	paths, _ := h.db.ListArtifactPathsByTaskID(ctx, task.ID)
	exec := parsePendingExecFromMetadata(task.Metadata)
	h.dispatchTaskAfterReady(ctx, w, task, paths, exec, http.StatusOK)
}

// dispatchTaskAfterReady writes the HTTP response after jobs are queued or inference completes synchronously.
func (h *TaskHandler) dispatchTaskAfterReady(ctx context.Context, w http.ResponseWriter, task *models.Task, attachmentPaths []string, exec pendingExecFlags, successStatus int) {
	t2, err := h.dispatchTaskJobsCore(ctx, task, exec)
	if err != nil {
		if strings.Contains(err.Error(), "inference readiness") {
			WriteBadRequest(w, err.Error())
			return
		}
		h.logger.Error("dispatch task jobs", "error", err)
		WriteInternalError(w, "Failed to start task execution")
		return
	}
	WriteJSON(w, successStatus, mcptaskbridge.TaskToResponse(t2, mcptaskbridge.TaskStatusToSpec(t2.Status), attachmentPaths))
}
