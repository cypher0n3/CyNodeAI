package mcpgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func handleSystemSettingGet(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	key := strArg(args, "key")
	if key == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"key required"}`), auditRec
	}
	ent, err := store.GetSystemSetting(ctx, key)
	if err != nil {
		code, b := writePreferenceErrToAudit(err, rec)
		return code, b, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{
		"key":        ent.Key,
		"value":      ent.Value,
		"value_type": ent.ValueType,
		"version":    ent.Version,
		"updated_at": ent.UpdatedAt.Format(time.RFC3339),
		"updated_by": ent.UpdatedBy,
	}
	b, err := json.Marshal(out)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}

func handleSystemSettingList(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	keyPrefix := strArg(args, "key_prefix")
	limit := intArg(args, "limit")
	if limit <= 0 {
		limit = database.MaxSystemSettingListLimit
	}
	if limit > database.MaxSystemSettingListLimit {
		limit = database.MaxSystemSettingListLimit
	}
	cursor := strArg(args, "cursor")
	entries, nextCursor, err := store.ListSystemSettings(ctx, keyPrefix, limit, cursor)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	items := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		items = append(items, map[string]interface{}{
			"key":        e.Key,
			"value":      e.Value,
			"value_type": e.ValueType,
			"version":    e.Version,
			"updated_at": e.UpdatedAt.Format(time.RFC3339),
			"updated_by": e.UpdatedBy,
		})
	}
	out := map[string]interface{}{"entries": items, "next_cursor": nextCursor}
	b, err := json.Marshal(out)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}

func handleSystemSettingCreate(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	key := strArg(args, "key")
	value := strArg(args, "value")
	valueType := strArg(args, "value_type")
	if key == "" || valueType == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"key and value_type required"}`), auditRec
	}
	reason := strArg(args, "reason")
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	updatedBy := strArg(args, "updated_by")
	var updatedByPtr *string
	if updatedBy != "" {
		updatedByPtr = &updatedBy
	}
	ent, err := store.CreateSystemSetting(ctx, key, value, valueType, reasonPtr, updatedByPtr)
	if err != nil {
		return auditRecCreateExistsOrInternalErr(rec, err, []byte(`{"error":"already exists"}`))
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{
		"key": ent.Key, "value": ent.Value, "value_type": ent.ValueType, "version": ent.Version,
		"updated_at": ent.UpdatedAt.Format(time.RFC3339),
	}
	b, _ := json.Marshal(out)
	return http.StatusOK, b, auditRec
}

func handleSystemSettingUpdate(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	key := strArg(args, "key")
	value := strArg(args, "value")
	valueType := strArg(args, "value_type")
	if key == "" || valueType == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"key and value_type required"}`), auditRec
	}
	var expectedVersion *int
	if v, ok := args["expected_version"]; ok {
		switch n := v.(type) {
		case float64:
			ev := int(n)
			expectedVersion = &ev
		case int:
			expectedVersion = &n
		}
	}
	reason := strArg(args, "reason")
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	updatedBy := strArg(args, "updated_by")
	var updatedByPtr *string
	if updatedBy != "" {
		updatedByPtr = &updatedBy
	}
	ent, err := store.UpdateSystemSetting(ctx, key, value, valueType, expectedVersion, reasonPtr, updatedByPtr)
	if err != nil {
		code, b := writePreferenceErrToAudit(err, rec)
		return code, b, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{
		"key": ent.Key, "value": ent.Value, "value_type": ent.ValueType, "version": ent.Version,
		"updated_at": ent.UpdatedAt.Format(time.RFC3339),
	}
	b, _ := json.Marshal(out)
	return http.StatusOK, b, auditRec
}

func handleSystemSettingDelete(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	key := strArg(args, "key")
	if key == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"key required"}`), auditRec
	}
	var expectedVersion *int
	if v, ok := args["expected_version"]; ok {
		switch n := v.(type) {
		case float64:
			ev := int(n)
			expectedVersion = &ev
		case int:
			expectedVersion = &n
		}
	}
	reason := strArg(args, "reason")
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	err := store.DeleteSystemSetting(ctx, key, expectedVersion, reasonPtr)
	if err != nil {
		code, b := writePreferenceErrToAudit(err, rec)
		return code, b, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	return http.StatusOK, []byte(`{}`), auditRec
}
