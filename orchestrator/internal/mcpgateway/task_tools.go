package mcpgateway

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/mcptaskbridge"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func handleTaskList(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	uid := uuidArg(args, "user_id")
	if uid == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"user_id required"}`), auditRec
	}
	rec.UserID = uid
	limit, offset, statusFilter, cursor, errMsg := mcptaskbridge.ParseListLimitOffset(args)
	if errMsg != "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"` + errMsg + `"}`), auditRec
	}
	resp, err := mcptaskbridge.ListTasksForUser(ctx, store, *uid, limit, offset, statusFilter, cursor)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	b, err := json.Marshal(resp)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}

func handleTaskResult(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	taskID := uuidArg(args, "task_id")
	if taskID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"task_id required"}`), auditRec
	}
	rec.TaskID = taskID
	resp, err := mcptaskbridge.TaskResultForUser(ctx, store, *taskID)
	if err != nil {
		if err == database.ErrNotFound {
			code, b := writePreferenceErrToAudit(err, rec)
			return code, b, rec
		}
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	b, err := json.Marshal(resp)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}

func handleTaskCancel(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	taskID := uuidArg(args, "task_id")
	if taskID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"task_id required"}`), auditRec
	}
	rec.TaskID = taskID
	if _, err := store.GetTaskByID(ctx, *taskID); err != nil {
		code, b := writePreferenceErrToAudit(err, rec)
		return code, b, rec
	}
	if err := mcptaskbridge.CancelTask(ctx, store, *taskID); err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	b, err := json.Marshal(userapi.CancelTaskResponse{TaskID: taskID.String(), Canceled: true})
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}

func handleTaskLogs(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	taskID := uuidArg(args, "task_id")
	if taskID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"task_id required"}`), auditRec
	}
	rec.TaskID = taskID
	stream := strArg(args, "stream")
	resp, err := mcptaskbridge.TaskLogsForUser(ctx, store, *taskID, stream)
	if err != nil {
		if err == database.ErrNotFound {
			code, b := writePreferenceErrToAudit(err, rec)
			return code, b, rec
		}
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	b, err := json.Marshal(resp)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}
