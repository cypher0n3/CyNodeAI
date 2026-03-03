package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// WorkflowHandler handles workflow start/resume/checkpoint/release (REQ-ORCHES-0144--0147).
type WorkflowHandler struct {
	db     database.Store
	logger *slog.Logger
}

// NewWorkflowHandler creates a workflow handler.
func NewWorkflowHandler(db database.Store, logger *slog.Logger) *WorkflowHandler {
	return &WorkflowHandler{db: db, logger: logger}
}

// Default lease TTL when not provided by client.
const defaultLeaseTTL = 1 * time.Hour

// StartWorkflowRequest is the body for POST /v1/workflow/start.
type StartWorkflowRequest struct {
	TaskID         string `json:"task_id"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	HolderID       string `json:"holder_id"`
	ExpiresInSec   *int   `json:"expires_in_sec,omitempty"`
}

// StartWorkflowResponse is the response for workflow start.
type StartWorkflowResponse struct {
	RunID   string `json:"run_id,omitempty"`
	Status  string `json:"status"` // "started" | "already_running"
	LeaseID string `json:"lease_id,omitempty"`
}

// ResumeWorkflowRequest is the body for POST /v1/workflow/resume.
type ResumeWorkflowRequest struct {
	TaskID string `json:"task_id"`
}

// ResumeWorkflowResponse is the response for workflow resume.
type ResumeWorkflowResponse struct {
	TaskID     string  `json:"task_id"`
	State      *string `json:"state,omitempty"`
	LastNodeID string  `json:"last_node_id,omitempty"`
	UpdatedAt  string  `json:"updated_at,omitempty"`
}

// CheckpointRequest is the body for POST /v1/workflow/checkpoint.
type CheckpointRequest struct {
	TaskID     string  `json:"task_id"`
	State      *string `json:"state,omitempty"`
	LastNodeID string  `json:"last_node_id,omitempty"`
}

// ReleaseWorkflowRequest is the body for POST /v1/workflow/release.
type ReleaseWorkflowRequest struct {
	TaskID  string `json:"task_id"`
	LeaseID string `json:"lease_id"`
}

// Start handles POST /v1/workflow/start.
func (h *WorkflowHandler) Start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "https://cynode.ai/specs/method-not-allowed", "Method Not Allowed", "")
		return
	}
	var req StartWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "invalid JSON body")
		return
	}
	taskID, err := uuid.Parse(req.TaskID)
	if err != nil || req.HolderID == "" {
		WriteBadRequest(w, "task_id and holder_id required")
		return
	}
	if _, err := h.db.GetTaskByID(r.Context(), taskID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			WriteNotFound(w, "task not found")
			return
		}
		h.logger.Error("get task failed", "error", err, "task_id", taskID)
		WriteInternalError(w, "failed to get task")
		return
	}
	ttl := defaultLeaseTTL
	if req.ExpiresInSec != nil && *req.ExpiresInSec > 0 {
		ttl = time.Duration(*req.ExpiresInSec) * time.Second
	}
	expiresAt := time.Now().UTC().Add(ttl)
	leaseID := uuid.New()
	if req.IdempotencyKey != "" {
		if id, err := uuid.Parse(req.IdempotencyKey); err == nil {
			leaseID = id
		}
	}
	lease, err := h.db.AcquireTaskWorkflowLease(r.Context(), taskID, leaseID, req.HolderID, expiresAt)
	if err != nil {
		if errors.Is(err, database.ErrLeaseHeld) {
			WriteConflict(w, "lease held by another workflow runner")
			return
		}
		h.logger.Error("acquire lease failed", "error", err, "task_id", taskID)
		WriteInternalError(w, "failed to acquire lease")
		return
	}
	WriteJSON(w, http.StatusOK, StartWorkflowResponse{
		RunID:   lease.LeaseID.String(),
		LeaseID: lease.LeaseID.String(),
		Status:  "started",
	})
}

// Resume handles POST /v1/workflow/resume.
func (h *WorkflowHandler) Resume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "https://cynode.ai/specs/method-not-allowed", "Method Not Allowed", "")
		return
	}
	var req ResumeWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "invalid JSON body")
		return
	}
	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		WriteBadRequest(w, "task_id required")
		return
	}
	cp, err := h.db.GetWorkflowCheckpoint(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			WriteNotFound(w, "no checkpoint for task")
			return
		}
		h.logger.Error("get checkpoint failed", "error", err, "task_id", taskID)
		WriteInternalError(w, "failed to get checkpoint")
		return
	}
	WriteJSON(w, http.StatusOK, ResumeWorkflowResponse{
		TaskID:     cp.TaskID.String(),
		State:      cp.State,
		LastNodeID: cp.LastNodeID,
		UpdatedAt:  cp.UpdatedAt.Format(time.RFC3339),
	})
}

// SaveCheckpoint handles POST /v1/workflow/checkpoint.
func (h *WorkflowHandler) SaveCheckpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "https://cynode.ai/specs/method-not-allowed", "Method Not Allowed", "")
		return
	}
	var req CheckpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "invalid JSON body")
		return
	}
	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		WriteBadRequest(w, "task_id required")
		return
	}
	cp := &models.WorkflowCheckpoint{TaskID: taskID, State: req.State, LastNodeID: req.LastNodeID}
	if err := h.db.UpsertWorkflowCheckpoint(r.Context(), cp); err != nil {
		h.logger.Error("upsert checkpoint failed", "error", err, "task_id", taskID)
		WriteInternalError(w, "failed to save checkpoint")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Release handles POST /v1/workflow/release.
func (h *WorkflowHandler) Release(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "https://cynode.ai/specs/method-not-allowed", "Method Not Allowed", "")
		return
	}
	var req ReleaseWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "invalid JSON body")
		return
	}
	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		WriteBadRequest(w, "task_id required")
		return
	}
	leaseID, err := uuid.Parse(req.LeaseID)
	if err != nil {
		WriteBadRequest(w, "lease_id required")
		return
	}
	if err := h.db.ReleaseTaskWorkflowLease(r.Context(), taskID, leaseID); err != nil {
		h.logger.Error("release lease failed", "error", err, "task_id", taskID)
		WriteInternalError(w, "failed to release lease")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
