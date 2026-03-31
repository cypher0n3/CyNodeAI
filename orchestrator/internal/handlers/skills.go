// Package handlers: skills endpoints per docs/tech_specs/skills_storage_and_inference.md.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/skillscan"
)

const skillScopeUser = "user"

// SkillsHandler handles GET/POST /v1/skills and GET/PUT/DELETE /v1/skills/{id}.
type SkillsHandler struct {
	db     database.SkillStore
	logger *slog.Logger
}

// NewSkillsHandler creates a skills handler.
func NewSkillsHandler(db database.SkillStore, logger *slog.Logger) *SkillsHandler {
	return &SkillsHandler{db: db, logger: logger}
}

// SkillLoadRequest is the body for POST /v1/skills/load.
type SkillLoadRequest struct {
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
	Scope   string `json:"scope,omitempty"`
}

// SkillUpdateRequest is the body for PUT /v1/skills/{id}.
type SkillUpdateRequest struct {
	Content *string `json:"content,omitempty"`
	Name    *string `json:"name,omitempty"`
	Scope   *string `json:"scope,omitempty"`
}

// SkillRejectResponse is returned on audit failure (REQ-SKILLS-0113).
type SkillRejectResponse struct {
	Error          string `json:"error"`
	Category       string `json:"category"`
	TriggeringText string `json:"triggering_text"`
}

func skillToMeta(s *models.Skill) map[string]interface{} {
	m := map[string]interface{}{
		"id":         s.ID.String(),
		"name":       s.Name,
		"scope":      s.Scope,
		"updated_at": s.UpdatedAt,
	}
	if s.OwnerID != nil {
		m["owner_id"] = s.OwnerID.String()
	}
	return m
}

// List handles GET /v1/skills. Optional query: scope, owner.
func (h *SkillsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		WriteUnauthorized(w, "not authenticated")
		return
	}
	scopeFilter := r.URL.Query().Get("scope")
	ownerFilter := r.URL.Query().Get("owner")
	limit, offset, ok := parseLimitOffsetQuery(r, database.DefaultSkillPageLimit, database.MaxSkillPageLimit)
	if !ok {
		WriteBadRequest(w, "Invalid limit or offset")
		return
	}
	skills, total, err := h.db.ListSkillsForUser(ctx, *userID, scopeFilter, ownerFilter, limit, offset)
	if err != nil {
		h.logger.Error("list skills", "error", err)
		WriteInternalError(w, "Failed to list skills")
		return
	}
	items := make([]map[string]interface{}, 0, len(skills))
	for _, s := range skills {
		items = append(items, skillToMeta(s))
	}
	out := map[string]interface{}{
		"skills":      items,
		"total_count": total,
	}
	if int64(offset)+int64(len(skills)) < total && len(skills) > 0 {
		next := offset + len(skills)
		out["next_offset"] = next
		out["next_cursor"] = strconv.Itoa(next)
	}
	WriteJSON(w, http.StatusOK, out)
}

// Get handles GET /v1/skills/{id}.
func (h *SkillsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		WriteUnauthorized(w, "not authenticated")
		return
	}
	idStr := r.PathValue("id")
	if idStr == "" {
		WriteBadRequest(w, "skill id required")
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		WriteBadRequest(w, "invalid skill id")
		return
	}
	skill, err := h.db.GetSkillByID(ctx, id)
	if err != nil {
		if err == database.ErrNotFound {
			WriteNotFound(w, "skill not found")
			return
		}
		h.logger.Error("get skill", "error", err)
		WriteInternalError(w, "Failed to get skill")
		return
	}
	// Only return if user owns it or it is system default.
	if !skill.IsSystem && (skill.OwnerID == nil || *skill.OwnerID != *userID) {
		WriteNotFound(w, "skill not found")
		return
	}
	out := skillToMeta(skill)
	out["content"] = skill.Content
	WriteJSON(w, http.StatusOK, out)
}

// Load handles POST /v1/skills/load.
func (h *SkillsHandler) Load(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		WriteUnauthorized(w, "not authenticated")
		return
	}
	var req SkillLoadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "Invalid request body")
		return
	}
	if req.Content == "" {
		WriteBadRequest(w, "content required")
		return
	}
	if m := skillscan.ScanContent(req.Content); m != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SkillRejectResponse{
			Error:          "policy violation",
			Category:       m.Category,
			TriggeringText: m.TriggeringText,
		})
		return
	}
	scope := req.Scope
	if scope == "" {
		scope = skillScopeUser
	}
	name := req.Name
	if name == "" {
		name = "Untitled skill"
	}
	skill, err := h.db.CreateSkill(ctx, name, req.Content, scope, userID, false)
	if err != nil {
		h.logger.Error("create skill", "error", err)
		WriteInternalError(w, "Failed to create skill")
		return
	}
	WriteJSON(w, http.StatusCreated, skillToMeta(skill))
}

// Update handles PUT /v1/skills/{id}.
func (h *SkillsHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		WriteUnauthorized(w, "not authenticated")
		return
	}
	id, req, ok := parseSkillIDAndBody(w, r, "id")
	if !ok {
		return
	}
	existing, ok := h.getSkillForWrite(ctx, w, id, userID)
	if !ok {
		return
	}
	_ = existing
	if req.Content != nil {
		if m := skillscan.ScanContent(*req.Content); m != nil {
			writeSkillReject(w, m)
			return
		}
	}
	skill, err := h.db.UpdateSkill(ctx, id, req.Name, req.Content, req.Scope)
	if err != nil {
		if err == database.ErrNotFound {
			WriteNotFound(w, "skill not found")
			return
		}
		h.logger.Error("update skill", "error", err)
		WriteInternalError(w, "Failed to update skill")
		return
	}
	WriteJSON(w, http.StatusOK, skillToMeta(skill))
}

func parseSkillIDAndBody(w http.ResponseWriter, r *http.Request, pathKey string) (id uuid.UUID, req SkillUpdateRequest, ok bool) {
	idStr := r.PathValue(pathKey)
	if idStr == "" {
		WriteBadRequest(w, "skill id required")
		return uuid.Nil, SkillUpdateRequest{}, false
	}
	parsed, err := uuid.Parse(idStr)
	if err != nil {
		WriteBadRequest(w, "invalid skill id")
		return uuid.Nil, SkillUpdateRequest{}, false
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "Invalid request body")
		return uuid.Nil, SkillUpdateRequest{}, false
	}
	return parsed, req, true
}

func (h *SkillsHandler) getSkillForWrite(ctx context.Context, w http.ResponseWriter, id uuid.UUID, userID *uuid.UUID) (*models.Skill, bool) {
	existing, err := h.db.GetSkillByID(ctx, id)
	if err != nil {
		if err == database.ErrNotFound {
			WriteNotFound(w, "skill not found")
			return nil, false
		}
		h.logger.Error("get skill for update", "error", err)
		WriteInternalError(w, "Failed to get skill")
		return nil, false
	}
	if !existing.IsSystem && (existing.OwnerID == nil || *existing.OwnerID != *userID) {
		WriteNotFound(w, "skill not found")
		return nil, false
	}
	return existing, true
}

func writeSkillReject(w http.ResponseWriter, m *skillscan.Match) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(SkillRejectResponse{
		Error:          "policy violation",
		Category:       m.Category,
		TriggeringText: m.TriggeringText,
	})
}

// Delete handles DELETE /v1/skills/{id}.
func (h *SkillsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		WriteUnauthorized(w, "not authenticated")
		return
	}
	idStr := r.PathValue("id")
	if idStr == "" {
		WriteBadRequest(w, "skill id required")
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		WriteBadRequest(w, "invalid skill id")
		return
	}
	existing, err := h.db.GetSkillByID(ctx, id)
	if err != nil {
		if err == database.ErrNotFound {
			WriteNotFound(w, "skill not found")
			return
		}
		h.logger.Error("get skill for delete", "error", err)
		WriteInternalError(w, "Failed to get skill")
		return
	}
	if !existing.IsSystem && (existing.OwnerID == nil || *existing.OwnerID != *userID) {
		WriteNotFound(w, "skill not found")
		return
	}
	if err := h.db.DeleteSkill(ctx, id); err != nil {
		if err == database.ErrNotFound {
			WriteNotFound(w, "skill not found")
			return
		}
		h.logger.Error("delete skill", "error", err)
		WriteInternalError(w, "Failed to delete skill")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
