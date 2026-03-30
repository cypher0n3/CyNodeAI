package mcpgateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/artifacts"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/skillscan"
)

const scopeTypeSystem = "system"

// toolCallRequest is a minimal body for POST /v1/mcp/tools/call (MCP tool call). Arguments per docs/tech_specs/mcp_tools/.
type toolCallRequest struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// auditRecCreateExistsOrInternalErr maps Create* ErrExists / other store errors to MCP HTTP responses and audit fields.
func auditRecCreateExistsOrInternalErr(rec *models.McpToolCallAuditLog, err error, existsBody []byte) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	if errors.Is(err, database.ErrExists) {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrConflict
		return http.StatusConflict, existsBody, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusError
	rec.ErrorType = &auditErrInternalError
	return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
}

func decodeToolCallRequest(r *http.Request) (toolName string, args map[string]interface{}, ok bool) {
	var req toolCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", nil, false
	}
	toolName = req.ToolName
	if toolName == "" {
		toolName = "unknown"
	}
	args = req.Arguments
	if args == nil {
		args = make(map[string]interface{})
	}
	return toolName, args, true
}

// requiredScopedIds defines which of task_id, run_id, job_id, user_id are required for a tool (REQ-MCPGAT-0103--0106, mcp_gateway_enforcement.md).
var requiredScopedIds = map[string]struct{ TaskID, RunID, JobID, UserID bool }{
	"preference.get":        {},
	"preference.list":       {},
	"preference.effective":  {TaskID: true},
	"preference.create":     {},
	"preference.update":     {},
	"preference.delete":     {},
	"task.get":              {TaskID: true},
	"job.get":               {JobID: true},
	"artifact.get":          {UserID: true},
	"artifact.put":          {UserID: true},
	"artifact.list":         {UserID: true},
	"skills.create":         {UserID: true},
	"skills.list":           {UserID: true},
	"skills.get":            {UserID: true},
	"skills.update":         {UserID: true},
	"skills.delete":         {UserID: true},
	"help.list":             {},
	"help.get":              {},
	"task.list":             {UserID: true},
	"task.result":           {TaskID: true},
	"task.cancel":           {TaskID: true},
	"task.logs":             {TaskID: true},
	"project.list":          {UserID: true},
	"project.get":           {UserID: true},
	"node.list":             {},
	"node.get":              {},
	"system_setting.get":    {},
	"system_setting.list":   {},
	"system_setting.create": {},
	"system_setting.update": {},
	"system_setting.delete": {},
}

// ValidateRequiredScopedIds returns an error message if required scoped ids are missing or invalid for the tool.
func ValidateRequiredScopedIds(toolName string, args map[string]interface{}) string {
	req, ok := requiredScopedIds[toolName]
	if !ok {
		return "" // unknown tool; routing layer will reject with 501
	}
	if req.TaskID && uuidArg(args, "task_id") == nil {
		return "task_id required"
	}
	if req.RunID && uuidArg(args, "run_id") == nil {
		return "run_id required"
	}
	if req.JobID && uuidArg(args, "job_id") == nil {
		return "job_id required"
	}
	if req.UserID && uuidArg(args, "user_id") == nil {
		return "user_id required"
	}
	return ""
}

// isLegacyDbPrefixedToolName reports agent-facing legacy names that used a db.* prefix (removed; use catalog names).
func isLegacyDbPrefixedToolName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "db.")
}

func newDenyAuditRecord(toolName, errorType string, args map[string]interface{}) *models.McpToolCallAuditLog {
	return &models.McpToolCallAuditLog{
		McpToolCallAuditLogBase: models.McpToolCallAuditLogBase{
			ToolName:  toolName,
			Decision:  auditDecisionDeny,
			Status:    auditStatusError,
			ErrorType: &errorType,
			TaskID:    uuidArg(args, "task_id"),
			RunID:     uuidArg(args, "run_id"),
			JobID:     uuidArg(args, "job_id"),
			UserID:    uuidArg(args, "user_id"),
		},
	}
}

// writeLegacyDbToolRemoved writes a deny audit and 404 for deprecated db.* tool names.
func writeLegacyDbToolRemoved(ctx context.Context, w http.ResponseWriter, store database.Store, logger *slog.Logger, toolName string, args map[string]interface{}, start time.Time) {
	rec := newDenyAuditRecord(toolName, "legacy_tool_name_removed", args)
	ms := int(time.Since(start).Milliseconds())
	rec.DurationMs = &ms
	if err := store.CreateMcpToolCallAuditLog(ctx, rec); err != nil {
		logger.Error("create mcp tool call audit log", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"legacy db.* tool names are not supported; use catalog names (e.g. preference.get, task.get)"}`))
}

// writeDenyAuditAndRespond writes a deny audit record and sends a 400 response. Returns true if caller should return.
func writeDenyAuditAndRespond(ctx context.Context, w http.ResponseWriter, store database.Store, logger *slog.Logger, toolName string, args map[string]interface{}, errMsg string) bool {
	rec := newDenyAuditRecord(toolName, "invalid_arguments", args)
	if err := store.CreateMcpToolCallAuditLog(ctx, rec); err != nil {
		logger.Error("create mcp tool call audit log", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(`{"error":"` + errMsg + `"}`))
	return true
}

// routeAndWriteAudit runs the tool, fills audit scoped ids and duration, writes audit, and sends the response.
func routeAndWriteAudit(ctx context.Context, w http.ResponseWriter, store database.Store, logger *slog.Logger, toolName string, args map[string]interface{}, start time.Time) {
	code, body, rec := routeToolCall(ctx, store, toolName, args)
	rec.ToolName = toolName
	if rec.TaskID == nil {
		rec.TaskID = uuidArg(args, "task_id")
	}
	if rec.RunID == nil {
		rec.RunID = uuidArg(args, "run_id")
	}
	if rec.JobID == nil {
		rec.JobID = uuidArg(args, "job_id")
	}
	if rec.UserID == nil {
		rec.UserID = uuidArg(args, "user_id")
	}
	if rec.DurationMs == nil {
		ms := int(time.Since(start).Milliseconds())
		rec.DurationMs = &ms
	}
	if err := store.CreateMcpToolCallAuditLog(ctx, rec); err != nil {
		logger.Error("create mcp tool call audit log", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

// toolCallHandler writes an audit record for every tool call (P2-02) and routes MCP tools (catalog names).
// P2-01: enforces required scoped ids (task_id/run_id/job_id) per tool before routing; rejects with 400 when missing.
// When auth is non-nil and agent bearer tokens are configured, callers must authenticate and
// PM / sandbox / PA roles are restricted to their allowlists (see allowlist.go).
func ToolCallHandler(store database.Store, logger *slog.Logger, auth *ToolCallAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("database not configured"))
			return
		}
		toolName, args, ok := decodeToolCallRequest(r)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		start := time.Now()
		if isLegacyDbPrefixedToolName(toolName) {
			writeLegacyDbToolRemoved(r.Context(), w, store, logger, toolName, args, start)
			return
		}
		if !tryAgentAllowlist(r.Context(), w, r, store, logger, toolName, args, auth, start) {
			return
		}
		if errMsg := ValidateRequiredScopedIds(toolName, args); errMsg != "" {
			writeDenyAuditAndRespond(r.Context(), w, store, logger, toolName, args, errMsg)
			return
		}
		routeAndWriteAudit(r.Context(), w, store, logger, toolName, args, start)
	}
}

const (
	auditDecisionDeny  = "deny"
	auditDecisionAllow = "allow"
	auditStatusError   = "error"
	auditStatusSuccess = "success"
)

// mcpToolHandler routes one tool name to a handler (shared signature with routeToolCall).
type mcpToolHandler func(context.Context, database.Store, map[string]interface{}, *models.McpToolCallAuditLog) (int, []byte, *models.McpToolCallAuditLog)

// mcpToolRoutes maps tool_name to handler. Populated in init after handlers are linked.
var mcpToolRoutes map[string]mcpToolHandler

func init() {
	mcpToolRoutes = map[string]mcpToolHandler{
		"preference.get":        handlePreferenceGet,
		"preference.list":       handlePreferenceList,
		"preference.effective":  handlePreferenceEffective,
		"preference.create":     handlePreferenceCreate,
		"preference.update":     handlePreferenceUpdate,
		"preference.delete":     handlePreferenceDelete,
		"task.get":              handleTaskGet,
		"help.list":             handleHelpList,
		"help.get":              handleHelpGet,
		"task.list":             handleTaskList,
		"task.result":           handleTaskResult,
		"task.cancel":           handleTaskCancel,
		"task.logs":             handleTaskLogs,
		"project.get":           handleProjectGet,
		"project.list":          handleProjectList,
		"job.get":               handleJobGet,
		"artifact.get":          artifactToolHandler((*artifacts.Service).MCPGet),
		"artifact.put":          artifactToolHandler((*artifacts.Service).MCPPut),
		"artifact.list":         artifactToolHandler((*artifacts.Service).MCPList),
		"skills.create":         handleSkillsCreate,
		"skills.list":           handleSkillsList,
		"skills.get":            handleSkillsGet,
		"skills.update":         handleSkillsUpdate,
		"skills.delete":         handleSkillsDelete,
		"node.list":             handleNodeList,
		"node.get":              handleNodeGet,
		"system_setting.get":    handleSystemSettingGet,
		"system_setting.list":   handleSystemSettingList,
		"system_setting.create": handleSystemSettingCreate,
		"system_setting.update": handleSystemSettingUpdate,
		"system_setting.delete": handleSystemSettingDelete,
	}
}

// routeToolCall dispatches to preference and db tools when applicable; returns status code, response body, and audit record.
func routeToolCall(ctx context.Context, store database.Store, toolName string, args map[string]interface{}) (code int, body []byte, rec *models.McpToolCallAuditLog) {
	rec = &models.McpToolCallAuditLog{
		McpToolCallAuditLogBase: models.McpToolCallAuditLogBase{
			Decision:  auditDecisionDeny,
			Status:    auditStatusError,
			ErrorType: &auditErrNotImplemented,
		},
	}
	if fn, ok := mcpToolRoutes[toolName]; ok {
		return fn(ctx, store, args, rec)
	}
	return http.StatusNotImplemented, []byte(`{"error":"tool routing not implemented"}`), rec
}

func strArg(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	v, _ := args[key].(string)
	return v
}

func intArg(args map[string]interface{}, key string) int {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func uuidArg(args map[string]interface{}, key string) *uuid.UUID {
	if args == nil {
		return nil
	}
	s, ok := args[key].(string)
	if !ok || s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

// writePreferenceErrToAudit sets rec for preference errors and returns HTTP code and body.
func writePreferenceErrToAudit(err error, rec *models.McpToolCallAuditLog) (code int, body []byte) {
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusError
	if errors.Is(err, database.ErrNotFound) {
		rec.ErrorType = &auditErrNotFound
		return http.StatusNotFound, []byte(`{"error":"not found"}`)
	}
	if errors.Is(err, database.ErrConflict) {
		rec.ErrorType = &auditErrConflict
		return http.StatusConflict, []byte(`{"error":"version conflict"}`)
	}
	rec.ErrorType = &auditErrInternalError
	return http.StatusInternalServerError, []byte(`{"error":"internal error"}`)
}

func handlePreferenceGet(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	scopeType := strArg(args, "scope_type")
	key := strArg(args, "key")
	if scopeType == "" || key == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"scope_type and key required"}`), auditRec
	}
	scopeID := uuidArg(args, "scope_id")
	if scopeType != scopeTypeSystem && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"scope_id required when scope_type is not system"}`), auditRec
	}
	ent, err := store.GetPreference(ctx, scopeType, scopeID, key)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{
		"scope_type": ent.ScopeType,
		"scope_id":   ent.ScopeID,
		"key":        ent.Key,
		"value":      ent.Value,
		"value_type": ent.ValueType,
		"version":    ent.Version,
	}
	body, _ = json.Marshal(out)
	return http.StatusOK, body, auditRec
}

func handlePreferenceList(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	scopeType := strArg(args, "scope_type")
	if scopeType == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"scope_type required"}`), auditRec
	}
	scopeID := uuidArg(args, "scope_id")
	if scopeType != scopeTypeSystem && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"scope_id required when scope_type is not system"}`), auditRec
	}
	keyPrefix := strArg(args, "key_prefix")
	limit := intArg(args, "limit")
	if limit <= 0 {
		limit = database.MaxPreferenceListLimit
	}
	cursor := strArg(args, "cursor")
	entries, nextCursor, err := store.ListPreferences(ctx, scopeType, scopeID, keyPrefix, limit, cursor)
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
			"scope_type": e.ScopeType,
			"scope_id":   e.ScopeID,
			"key":        e.Key,
			"value":      e.Value,
			"value_type": e.ValueType,
			"version":    e.Version,
		})
	}
	out := map[string]interface{}{"entries": items, "next_cursor": nextCursor}
	body, _ = json.Marshal(out)
	return http.StatusOK, body, auditRec
}

func handlePreferenceEffective(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	taskID := uuidArg(args, "task_id")
	if taskID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"task_id required"}`), auditRec
	}
	rec.TaskID = taskID
	effective, err := store.GetEffectivePreferencesForTask(ctx, *taskID)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	body, _ = json.Marshal(map[string]interface{}{"effective": effective})
	return http.StatusOK, body, auditRec
}

func handlePreferenceCreate(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	scopeType := strArg(args, "scope_type")
	key := strArg(args, "key")
	value := strArg(args, "value")
	valueType := strArg(args, "value_type")
	if scopeType == "" || key == "" || valueType == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"scope_type, key, and value_type required"}`), auditRec
	}
	scopeID := uuidArg(args, "scope_id")
	if scopeType != scopeTypeSystem && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"scope_id required when scope_type is not system"}`), auditRec
	}
	var reason, updatedBy *string
	if r := strArg(args, "reason"); r != "" {
		reason = &r
	}
	ent, err := store.CreatePreference(ctx, scopeType, scopeID, key, value, valueType, reason, updatedBy)
	if err != nil {
		return auditRecCreateExistsOrInternalErr(rec, err, []byte(`{"error":"preference already exists for scope and key"}`))
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{
		"scope_type": ent.ScopeType,
		"scope_id":   ent.ScopeID,
		"key":        ent.Key,
		"value":      ent.Value,
		"value_type": ent.ValueType,
		"version":    ent.Version,
	}
	body, _ = json.Marshal(out)
	return http.StatusOK, body, auditRec
}

func handlePreferenceUpdate(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	scopeType := strArg(args, "scope_type")
	key := strArg(args, "key")
	value := strArg(args, "value")
	valueType := strArg(args, "value_type")
	if scopeType == "" || key == "" || valueType == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"scope_type, key, and value_type required"}`), auditRec
	}
	scopeID := uuidArg(args, "scope_id")
	if scopeType != scopeTypeSystem && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"scope_id required when scope_type is not system"}`), auditRec
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
	var reason, updatedBy *string
	if r := strArg(args, "reason"); r != "" {
		reason = &r
	}
	ent, err := store.UpdatePreference(ctx, scopeType, scopeID, key, value, valueType, expectedVersion, reason, updatedBy)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{
		"scope_type": ent.ScopeType,
		"scope_id":   ent.ScopeID,
		"key":        ent.Key,
		"value":      ent.Value,
		"value_type": ent.ValueType,
		"version":    ent.Version,
	}
	body, _ = json.Marshal(out)
	return http.StatusOK, body, auditRec
}

func handlePreferenceDelete(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	scopeType := strArg(args, "scope_type")
	key := strArg(args, "key")
	if scopeType == "" || key == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"scope_type and key required"}`), auditRec
	}
	scopeID := uuidArg(args, "scope_id")
	if scopeType != scopeTypeSystem && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"scope_id required when scope_type is not system"}`), auditRec
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
	var reason *string
	if r := strArg(args, "reason"); r != "" {
		reason = &r
	}
	err := store.DeletePreference(ctx, scopeType, scopeID, key, expectedVersion, reason)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	return http.StatusOK, []byte(`{}`), auditRec
}

func handleTaskGet(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	taskID := uuidArg(args, "task_id")
	if taskID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"task_id required"}`), auditRec
	}
	rec.TaskID = taskID
	task, err := store.GetTaskByID(ctx, *taskID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{
		"id":         task.ID,
		"status":     task.Status,
		"prompt":     task.Prompt,
		"summary":    task.Summary,
		"created_by": task.CreatedBy,
		"project_id": task.ProjectID,
		"created_at": task.CreatedAt,
		"updated_at": task.UpdatedAt,
	}
	body, _ = json.Marshal(out)
	return http.StatusOK, body, auditRec
}

func handleJobGet(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	jobID := uuidArg(args, "job_id")
	if jobID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"job_id required"}`), auditRec
	}
	rec.JobID = jobID
	job, err := store.GetJobByID(ctx, *jobID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{
		"id":         job.ID,
		"task_id":    job.TaskID,
		"status":     job.Status,
		"node_id":    job.NodeID,
		"payload":    job.Payload.Ptr(),
		"result":     job.Result.Ptr(),
		"created_at": job.CreatedAt,
		"updated_at": job.UpdatedAt,
	}
	body, _ = json.Marshal(out)
	return http.StatusOK, body, auditRec
}

// --- Skills tools (REQ-SKILLS-0114; contract in skills_storage_and_inference.md) ---

func handleSkillsCreate(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
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
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	content := strArg(args, "content")
	if content == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"content required"}`), auditRec
	}
	if m := skillscan.ScanContent(content); m != nil {
		return skillsPolicyViolationResponse(rec, m, auditRec)
	}
	name := strArg(args, "name")
	if name == "" {
		name = "Untitled skill"
	}
	scope := strArg(args, "scope")
	if scope == "" {
		scope = "user"
	}
	skill, err := store.CreateSkill(ctx, name, content, scope, &u.ID, false)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{"id": skill.ID.String(), "name": skill.Name, "scope": skill.Scope, "updated_at": skill.UpdatedAt}
	b, _ := json.Marshal(out)
	return http.StatusOK, b, auditRec
}

func handleSkillsList(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	userID := uuidArg(args, "user_id")
	if userID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"user_id required"}`), auditRec
	}
	rec.UserID = userID
	if _, err := store.GetUserByID(ctx, *userID); err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	scopeFilter := strArg(args, "scope")
	ownerFilter := strArg(args, "owner")
	skills, err := store.ListSkillsForUser(ctx, *userID, scopeFilter, ownerFilter)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	items := make([]map[string]interface{}, 0, len(skills))
	for _, s := range skills {
		items = append(items, map[string]interface{}{
			"id":         s.ID.String(),
			"name":       s.Name,
			"scope":      s.Scope,
			"updated_at": s.UpdatedAt,
		})
	}
	out := map[string]interface{}{"skills": items}
	b, _ := json.Marshal(out)
	return http.StatusOK, b, auditRec
}

func handleSkillsGet(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	userID := uuidArg(args, "user_id")
	skillIDStr := strArg(args, "skill_id")
	if userID == nil || skillIDStr == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"user_id and skill_id required"}`), auditRec
	}
	rec.UserID = userID
	skillID, err := uuid.Parse(skillIDStr)
	if err != nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"invalid skill_id"}`), auditRec
	}
	skill, err := store.GetSkillByID(ctx, skillID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	if !skill.IsSystem && (skill.OwnerID == nil || *skill.OwnerID != *userID) {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrNotFound
		return http.StatusNotFound, []byte(`{"error":"not found"}`), auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{"id": skill.ID.String(), "name": skill.Name, "scope": skill.Scope, "content": skill.Content, "updated_at": skill.UpdatedAt}
	b, _ := json.Marshal(out)
	return http.StatusOK, b, auditRec
}

func handleSkillsUpdate(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	userID := uuidArg(args, "user_id")
	skillIDStr := strArg(args, "skill_id")
	if userID == nil || skillIDStr == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"user_id and skill_id required"}`), auditRec
	}
	rec.UserID = userID
	skillID, err := uuid.Parse(skillIDStr)
	if err != nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"invalid skill_id"}`), auditRec
	}
	skill, err := store.GetSkillByID(ctx, skillID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	if skill.IsSystem {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrForbidden
		return http.StatusForbidden, []byte(`{"error":"system skills cannot be modified"}`), auditRec
	}
	if skill.OwnerID == nil || *skill.OwnerID != *userID {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrNotFound
		return http.StatusNotFound, []byte(`{"error":"not found"}`), auditRec
	}
	var name, content, scope *string
	if v := strArg(args, "name"); v != "" {
		name = &v
	}
	if v := strArg(args, "content"); v != "" {
		if m := skillscan.ScanContent(v); m != nil {
			return skillsPolicyViolationResponse(rec, m, auditRec)
		}
		content = &v
	}
	if v := strArg(args, "scope"); v != "" {
		scope = &v
	}
	updated, err := store.UpdateSkill(ctx, skillID, name, content, scope)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{"id": updated.ID.String(), "name": updated.Name, "scope": updated.Scope, "updated_at": updated.UpdatedAt}
	b, _ := json.Marshal(out)
	return http.StatusOK, b, auditRec
}

func handleSkillsDelete(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	userID := uuidArg(args, "user_id")
	skillIDStr := strArg(args, "skill_id")
	if userID == nil || skillIDStr == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"user_id and skill_id required"}`), auditRec
	}
	rec.UserID = userID
	skillID, err := uuid.Parse(skillIDStr)
	if err != nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInvalidArguments
		return http.StatusBadRequest, []byte(`{"error":"invalid skill_id"}`), auditRec
	}
	skill, err := store.GetSkillByID(ctx, skillID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	if skill.IsSystem {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrForbidden
		return http.StatusForbidden, []byte(`{"error":"system skills cannot be deleted"}`), auditRec
	}
	if skill.OwnerID == nil || *skill.OwnerID != *userID {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrNotFound
		return http.StatusNotFound, []byte(`{"error":"not found"}`), auditRec
	}
	if err := store.DeleteSkill(ctx, skillID); err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	return http.StatusOK, []byte(`{}`), auditRec
}

func skillsPolicyViolationResponse(rec *models.McpToolCallAuditLog, m *skillscan.Match, auditRec *models.McpToolCallAuditLog) (code int, body []byte, outRec *models.McpToolCallAuditLog) {
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusError
	rec.ErrorType = &auditErrPolicyViolation
	out := map[string]interface{}{"error": "policy violation", "category": m.Category, "triggering_text": m.TriggeringText}
	b, _ := json.Marshal(out)
	return http.StatusBadRequest, b, auditRec
}
