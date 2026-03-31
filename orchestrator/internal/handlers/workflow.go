package handlers

import (
	"context"
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
	db     database.WorkflowHandlerDeps
	logger *slog.Logger
}

// NewWorkflowHandler creates a workflow handler.
func NewWorkflowHandler(db database.WorkflowHandlerDeps, logger *slog.Logger) *WorkflowHandler {
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

// startGateDenied runs the workflow start gate; returns true if the response was written (deny or error).
func (h *WorkflowHandler) startGateDenied(w http.ResponseWriter, r *http.Request, task *models.Task, taskID uuid.UUID) bool {
	requestedByPMA := r.Header.Get("X-Cynode-Workflow-Requested-By") == "pma"
	denyReason, err := h.db.EvaluateWorkflowStartGate(r.Context(), task, requestedByPMA)
	if err != nil {
		h.logger.Error("workflow start gate failed", "error", err, "task_id", taskID)
		WriteInternalError(w, "failed to evaluate workflow start gate")
		return true
	}
	if denyReason != "" {
		WriteConflict(w, denyReason)
		return true
	}
	return false
}

// startAcquireAndRespond acquires the workflow lease and writes the start response or an error.
// When the lease is already held by the same holder (idempotent re-request), returns 200 with status "already_running" per REQ-ORCHES-0145.
//
//nolint:gocognit,gocyclo // lease idempotency, gate denials, and error mapping.
func (h *WorkflowHandler) startAcquireAndRespond(w http.ResponseWriter, r *http.Request, taskID uuid.UUID, req *StartWorkflowRequest) {
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
	var idempotentLease *models.TaskWorkflowLease
	var startedLease *models.TaskWorkflowLease
	var hadLeaseRow bool
	err := h.db.WithTx(r.Context(), func(ctx context.Context, tx database.Store) error {
		existing, err := tx.GetTaskWorkflowLease(ctx, taskID)
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return err
		}
		hadLeaseRow = err == nil && existing != nil
		if err == nil && existing != nil && existing.HolderID != nil && *existing.HolderID == req.HolderID &&
			existing.LeaseID == leaseID && existing.ExpiresAt != nil && existing.ExpiresAt.After(time.Now().UTC()) {
			idempotentLease = existing
			return nil
		}
		lease, err := tx.AcquireTaskWorkflowLease(ctx, taskID, leaseID, req.HolderID, expiresAt)
		if err != nil {
			return err
		}
		startedLease = lease
		return nil
	})
	if err != nil {
		if errors.Is(err, database.ErrLeaseHeld) {
			WriteConflict(w, "lease held by another workflow runner")
			return
		}
		h.logger.Error("acquire lease failed", "error", err, "task_id", taskID)
		WriteInternalError(w, "failed to acquire lease")
		return
	}
	if idempotentLease != nil {
		WriteJSON(w, http.StatusOK, StartWorkflowResponse{
			RunID:   idempotentLease.LeaseID.String(),
			LeaseID: idempotentLease.LeaseID.String(),
			Status:  "already_running",
		})
		return
	}
	status := "started"
	if req.IdempotencyKey != "" && hadLeaseRow && startedLease != nil &&
		startedLease.HolderID != nil && *startedLease.HolderID == req.HolderID &&
		startedLease.LeaseID == leaseID {
		// Lease row existed before Acquire; Acquire returned idempotent success (REQ-ORCHES-0145).
		status = "already_running"
	}
	WriteJSON(w, http.StatusOK, StartWorkflowResponse{
		RunID:   startedLease.LeaseID.String(),
		LeaseID: startedLease.LeaseID.String(),
		Status:  status,
	})
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
	task, err := h.db.GetTaskByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			WriteNotFound(w, "task not found")
			return
		}
		h.logger.Error("get task failed", "error", err, "task_id", taskID)
		WriteInternalError(w, "failed to get task")
		return
	}
	if h.startGateDenied(w, r, task, taskID) {
		return
	}
	h.startAcquireAndRespond(w, r, taskID, &req)
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
	cp := &models.WorkflowCheckpoint{
		WorkflowCheckpointBase: models.WorkflowCheckpointBase{
			TaskID:     taskID,
			State:      req.State,
			LastNodeID: req.LastNodeID,
		},
	}
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
