// Package handlers: /v1/artifacts REST API (orchestrator_artifacts_storage.md).
package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/artifacts"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// ArtifactsHandler serves scope-partitioned artifact CRUD when wired with a non-nil Service.
type ArtifactsHandler struct {
	svc    artifacts.HandlerAPI
	logger *slog.Logger
}

// NewArtifactsHandler returns a handler; svc may be nil (endpoints return 503).
func NewArtifactsHandler(svc artifacts.HandlerAPI, logger *slog.Logger) *ArtifactsHandler {
	return &ArtifactsHandler{svc: svc, logger: logger}
}

// Create handles POST /v1/artifacts (body is raw blob; scope via query).
func (h *ArtifactsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "artifacts storage not configured"})
		return
	}
	ctx := r.Context()
	userID, handle, ok := h.user(ctx, w)
	if !ok {
		return
	}
	q := r.URL.Query()
	level := strings.TrimSpace(q.Get("scope_level"))
	ownerID := parseUUIDQuery(q, "owner_user_id")
	groupID := parseUUIDQuery(q, "group_id")
	projectID := parseUUIDQuery(q, "project_id")
	jobID := parseUUIDQuery(q, "job_id")
	taskID := parseUUIDQuery(q, "task_id")
	runID := parseUUIDQuery(q, "run_id")
	if level == "" {
		WriteBadRequest(w, "scope_level query parameter required")
		return
	}
	if strings.EqualFold(level, "user") && ownerID == nil {
		ownerID = userID
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteBadRequest(w, "failed to read body")
		return
	}
	ct := r.Header.Get("Content-Type")
	var ctPtr *string
	if ct != "" {
		ctPtr = &ct
	}
	artifactPath := strings.TrimSpace(q.Get("path"))
	if artifactPath == "" {
		artifactPath = strings.TrimSpace(q.Get("filename"))
	}
	if artifactPath == "" {
		fn := r.Header.Get("X-Filename")
		if fn != "" {
			artifactPath = fn
		}
	}
	if artifactPath == "" {
		WriteBadRequest(w, "path or filename query parameter required")
		return
	}
	art, err := h.svc.CreateFromBody(ctx, *userID, handle, level, ownerID, groupID, projectID, artifactPath, body, ctPtr, jobID, taskID, runID)
	if err != nil {
		h.writeArtifactErr(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"artifact_id":     art.ID.String(),
		"storage_ref":     art.StorageRef,
		"size_bytes":      art.SizeBytes,
		"content_type":    art.ContentType,
		"checksum_sha256": art.ChecksumSHA256,
		"created_at":      art.CreatedAt,
		"scope_level":     art.ScopeLevel,
		"path":            art.Path,
	})
}

// Read handles GET /v1/artifacts/{artifact_id} (blob body).
func (h *ArtifactsHandler) Read(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "artifacts storage not configured"})
		return
	}
	ctx := r.Context()
	userID, handle, ok := h.user(ctx, w)
	if !ok {
		return
	}
	idStr := r.PathValue("artifact_id")
	if idStr == "" {
		idStr = r.PathValue("id")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		WriteBadRequest(w, "invalid artifact id")
		return
	}
	data, art, err := h.svc.GetBlob(ctx, *userID, handle, id)
	if err != nil {
		h.writeArtifactErr(w, err)
		return
	}
	if art.ContentType != nil && *art.ContentType != "" {
		w.Header().Set("Content-Type", *art.ContentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(art.Path))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Update handles PUT /v1/artifacts/{artifact_id}.
func (h *ArtifactsHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "artifacts storage not configured"})
		return
	}
	ctx := r.Context()
	userID, handle, ok := h.user(ctx, w)
	if !ok {
		return
	}
	idStr := r.PathValue("artifact_id")
	if idStr == "" {
		idStr = r.PathValue("id")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		WriteBadRequest(w, "invalid artifact id")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteBadRequest(w, "failed to read body")
		return
	}
	ct := r.Header.Get("Content-Type")
	var ctPtr *string
	if ct != "" {
		ctPtr = &ct
	}
	jobID := parseUUIDQuery(r.URL.Query(), "job_id")
	art, err := h.svc.UpdateBlob(ctx, *userID, handle, id, body, ctPtr, jobID)
	if err != nil {
		h.writeArtifactErr(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"artifact_id":     art.ID.String(),
		"size_bytes":      art.SizeBytes,
		"content_type":    art.ContentType,
		"checksum_sha256": art.ChecksumSHA256,
		"updated_at":      art.UpdatedAt,
	})
}

// Delete handles DELETE /v1/artifacts/{artifact_id}.
func (h *ArtifactsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "artifacts storage not configured"})
		return
	}
	ctx := r.Context()
	userID, handle, ok := h.user(ctx, w)
	if !ok {
		return
	}
	idStr := r.PathValue("artifact_id")
	if idStr == "" {
		idStr = r.PathValue("id")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		WriteBadRequest(w, "invalid artifact id")
		return
	}
	if err := h.svc.Delete(ctx, *userID, handle, id); err != nil {
		h.writeArtifactErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Find handles GET /v1/artifacts (JSON list).
func (h *ArtifactsHandler) Find(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "artifacts storage not configured"})
		return
	}
	ctx := r.Context()
	userID, handle, ok := h.user(ctx, w)
	if !ok {
		return
	}
	q := r.URL.Query()
	level := strings.TrimSpace(q.Get("scope_level"))
	if level == "" {
		WriteBadRequest(w, "scope_level query parameter required")
		return
	}
	p := listArtifactFindParams(q, userID, level)
	rows, err := h.svc.List(ctx, *userID, handle, p)
	if err != nil {
		h.writeArtifactErr(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"artifacts": orchestratorArtifactListJSON(rows)})
}

func listArtifactFindParams(q url.Values, userID *uuid.UUID, level string) database.ListOrchestratorArtifactsParams {
	p := database.ListOrchestratorArtifactsParams{
		ScopeLevel:        level,
		OwnerUserID:       parseUUIDQuery(q, "user_id"),
		GroupID:           parseUUIDQuery(q, "group_id"),
		ProjectID:         parseUUIDQuery(q, "project_id"),
		CorrelationTaskID: parseUUIDQuery(q, "correlation_task_id"),
	}
	if p.OwnerUserID == nil {
		p.OwnerUserID = parseUUIDQuery(q, "owner_user_id")
	}
	if strings.EqualFold(level, "user") && p.OwnerUserID == nil {
		p.OwnerUserID = userID
	}
	if lim := q.Get("limit"); lim != "" {
		if n, err := strconv.Atoi(lim); err == nil {
			p.Limit = n
		}
	}
	if off := q.Get("offset"); off != "" {
		if n, err := strconv.Atoi(off); err == nil {
			p.Offset = n
		}
	}
	return p
}

func orchestratorArtifactListJSON(rows []*models.OrchestratorArtifact) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(rows))
	for _, a := range rows {
		m := map[string]interface{}{
			"artifact_id": a.ID.String(),
			"path":        a.Path,
			"scope_level": a.ScopeLevel,
			"size_bytes":  a.SizeBytes,
			"created_at":  a.CreatedAt,
		}
		if a.ContentType != nil {
			m["content_type"] = *a.ContentType
		}
		out = append(out, m)
	}
	return out
}

func (h *ArtifactsHandler) user(ctx context.Context, w http.ResponseWriter) (*uuid.UUID, string, bool) {
	uid := getUserIDFromContext(ctx)
	if uid == nil {
		WriteUnauthorized(w, "not authenticated")
		return nil, "", false
	}
	return uid, GetHandleFromContext(ctx), true
}

func (h *ArtifactsHandler) writeArtifactErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, database.ErrNotFound):
		WriteNotFound(w, "artifact not found")
	case errors.Is(err, database.ErrExists):
		WriteConflict(w, "artifact already exists at path")
	case errors.Is(err, artifacts.ErrForbidden):
		WriteForbidden(w, "access denied")
	default:
		if err != nil && (strings.Contains(err.Error(), "forbidden") || strings.Contains(err.Error(), "Forbidden")) {
			WriteForbidden(w, "access denied")
			return
		}
		h.logger.Error("artifact handler", "error", err)
		WriteInternalError(w, "artifact operation failed")
	}
}

func parseUUIDQuery(q url.Values, key string) *uuid.UUID {
	v := strings.TrimSpace(q.Get(key))
	if v == "" {
		return nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return nil
	}
	return &id
}
