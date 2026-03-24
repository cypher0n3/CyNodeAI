package mcpgateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/artifacts"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/google/uuid"
)

// artifactToolService is wired from control-plane main (S3 + DB). Nil disables artifact MCP tools.
var artifactToolService *artifacts.Service

// SetArtifactToolService registers the artifacts backend for MCP tool handlers (optional).
func SetArtifactToolService(s *artifacts.Service) {
	artifactToolService = s
}

// artifactToolHandler wraps a Service MCP method as an mcpToolHandler (single implementation; avoids dupl on thin wrappers).
func artifactToolHandler(fn func(*artifacts.Service, context.Context, uuid.UUID, string, map[string]interface{}) (map[string]interface{}, error)) mcpToolHandler {
	return func(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
		return dispatchArtifactTool(ctx, store, args, rec, func(ctx context.Context, uid uuid.UUID, handle string, a map[string]interface{}) (map[string]interface{}, error) {
			return fn(artifactToolService, ctx, uid, handle, a)
		})
	}
}

// dispatchArtifactTool resolves user_id, loads the user handle, and runs the MCP artifact call.
func dispatchArtifactTool(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog,
	call func(context.Context, uuid.UUID, string, map[string]interface{}) (map[string]interface{}, error),
) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	if artifactToolService == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrServiceUnavailable
		return http.StatusServiceUnavailable, []byte(`{"error":"artifacts storage not configured"}`), auditRec
	}
	userID := uuidArg(args, "user_id")
	if userID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"user_id required"}`), auditRec
	}
	rec.UserID = userID
	u, err := store.GetUserByID(ctx, *userID)
	if err != nil {
		c, b := writePreferenceErrToAudit(err, rec)
		return c, b, auditRec
	}
	out, err := call(ctx, *userID, u.Handle, args)
	if err != nil {
		return artifactToolErr(rec, err)
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	b, _ := json.Marshal(out)
	return http.StatusOK, b, auditRec
}

func artifactToolErr(rec *models.McpToolCallAuditLog, err error) (code int, respBody []byte, auditRec *models.McpToolCallAuditLog) {
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusError
	switch {
	case errors.Is(err, database.ErrNotFound):
		rec.ErrorType = &auditErrNotFound
		return http.StatusNotFound, []byte(`{"error":"not found"}`), rec
	case errors.Is(err, artifacts.ErrForbidden):
		rec.ErrorType = &auditErrForbidden
		return http.StatusForbidden, []byte(`{"error":"forbidden"}`), rec
	default:
		rec.ErrorType = &auditErrInvalidArguments
		if err != nil {
			b, _ := json.Marshal(map[string]string{"error": err.Error()})
			return http.StatusBadRequest, b, rec
		}
		return http.StatusBadRequest, []byte(`{"error":"bad request"}`), rec
	}
}
