package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func projectResponseMap(p *models.Project) map[string]interface{} {
	m := map[string]interface{}{
		"id":           p.ID.String(),
		"slug":         p.Slug,
		"display_name": p.DisplayName,
		"is_active":    p.IsActive,
		"created_at":   p.CreatedAt.Format(time.RFC3339),
		"updated_at":   p.UpdatedAt.Format(time.RFC3339),
	}
	if p.Description != nil {
		m["description"] = *p.Description
	}
	return m
}

func handleProjectGet(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	userID := uuidArg(args, "user_id")
	if userID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"user_id required"}`), auditRec
	}
	rec.UserID = userID
	pid := uuidArg(args, "project_id")
	slug := strings.TrimSpace(strArg(args, "slug"))
	if (pid == nil && slug == "") || (pid != nil && slug != "") {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"exactly one of project_id or slug is required"}`), auditRec
	}
	var proj *models.Project
	var err error
	if pid != nil {
		proj, err = store.GetProjectByID(ctx, *pid)
	} else {
		proj, err = store.GetProjectBySlug(ctx, slug)
	}
	if err != nil {
		code, b := writePreferenceErrToAudit(err, rec)
		return code, b, rec
	}
	def, err := store.GetOrCreateDefaultProjectForUser(ctx, *userID)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	if proj.ID != def.ID {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("not_found")
		return http.StatusNotFound, []byte(`{"error":"not found"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	rec.ProjectID = &proj.ID
	out := projectResponseMap(proj)
	b, err := json.Marshal(out)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}

func handleProjectList(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	userID := uuidArg(args, "user_id")
	if userID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"user_id required"}`), auditRec
	}
	rec.UserID = userID
	q := strArg(args, "q")
	limit := intArg(args, "limit")
	offset := intArg(args, "offset")
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	projects, err := store.ListAuthorizedProjectsForUser(ctx, *userID, q, limit, offset)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	items := make([]map[string]interface{}, 0, len(projects))
	for _, p := range projects {
		items = append(items, projectResponseMap(p))
	}
	out := map[string]interface{}{
		"projects":    items,
		"next_cursor": "",
	}
	b, err := json.Marshal(out)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}
