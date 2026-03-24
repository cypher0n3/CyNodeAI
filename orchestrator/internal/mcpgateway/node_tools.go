package mcpgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/mcptaskbridge"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func handleNodeList(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	limit, offset, _, _, errMsg := mcptaskbridge.ParseListLimitOffset(args)
	if errMsg != "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"` + errMsg + `"}`), auditRec
	}
	nodes, err := store.ListNodes(ctx, limit+1, offset)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	hasMore := len(nodes) > limit
	if hasMore {
		nodes = nodes[:limit]
	}
	out := map[string]interface{}{"nodes": nodes}
	if hasMore {
		next := offset + limit
		out["next_cursor"] = strconv.Itoa(next)
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	b, err := json.Marshal(out)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}

func handleNodeGet(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	slug := strArg(args, "node_slug")
	if slug == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"node_slug required"}`), auditRec
	}
	node, err := store.GetNodeBySlug(ctx, slug)
	if err != nil {
		code, b := writePreferenceErrToAudit(err, rec)
		return code, b, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	b, err := json.Marshal(node)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}
