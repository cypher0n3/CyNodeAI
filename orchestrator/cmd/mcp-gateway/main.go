// Package main provides the orchestrator MCP gateway service.
// When DATABASE_URL is set, the gateway opens the DB and writes an audit record for every tool call it routes (P2-02).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/skillscan"
)

const scopeTypeSystem = "system"

func main() {
	os.Exit(runMain(context.Background()))
}

// runMain sets up logger and runs the server. Returns 0 on success, 1 on failure. Used by main and tests.
func runMain(ctx context.Context) int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
	if err := run(ctx, logger); err != nil {
		logger.Error("run failed", "error", err)
		return 1
	}
	return 0
}

// testStore is set by tests to inject a mock store so run() uses it instead of opening DB.
var testStore database.Store

// testDatabaseOpen is set by tests to provide a store without opening a real DB (avoids needing RunSchema).
var testDatabaseOpen func(ctx context.Context, dsn string) (database.Store, error)

// run sets up and runs the server until ctx is canceled. Used by main and tests.
// When DATABASE_URL is set (or testStore/testDatabaseOpen is set in tests), tool-call handler writes audit records.
func run(ctx context.Context, logger *slog.Logger) error {
	var store database.Store
	switch {
	case testStore != nil:
		store = testStore
	case testDatabaseOpen != nil:
		dsn := getEnv("DATABASE_URL", "")
		var err error
		store, err = testDatabaseOpen(ctx, dsn)
		if err != nil {
			return err
		}
		logger.Info("mcp-gateway database connected")
	default:
		dsn := getEnv("DATABASE_URL", "")
		if dsn != "" {
			db, err := database.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			if err := db.RunSchema(ctx, logger); err != nil {
				return err
			}
			store = db
			logger.Info("mcp-gateway database connected")
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /v1/mcp/tools/call", toolCallHandler(store, logger))

	srv := &http.Server{
		Addr:              getEnv("LISTEN_ADDR", ":8083"),
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting mcp-gateway", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout())
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// shutdownTimeout returns server shutdown timeout from env or default. Used by run and tests.
func shutdownTimeout() time.Duration {
	const defaultSec = 10
	if s := os.Getenv("MCP_GATEWAY_SHUTDOWN_SEC"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return defaultSec * time.Second
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// toolCallRequest is a minimal body for POST /v1/mcp/tools/call (MCP tool call). Arguments per mcp_tool_catalog.md.
type toolCallRequest struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// requiredScopedIds defines which of task_id, run_id, job_id are required for a tool (REQ-MCPGAT-0103--0106, mcp_gateway_enforcement.md).
var requiredScopedIds = map[string]struct{ TaskID, RunID, JobID bool }{
	"db.preference.get":       {},
	"db.preference.list":      {},
	"db.preference.effective": {TaskID: true},
	"db.preference.create":    {},
	"db.preference.update":   {},
	"db.preference.delete":   {},
	"db.task.get":            {TaskID: true},
	"db.job.get":              {JobID: true},
	"artifact.get":           {TaskID: true},
	"skills.create":          {TaskID: true},
	"skills.list":            {TaskID: true},
	"skills.get":             {TaskID: true},
	"skills.update":          {TaskID: true},
	"skills.delete":          {TaskID: true},
}

// validateRequiredScopedIds returns an error message if required scoped ids are missing or invalid for the tool.
func validateRequiredScopedIds(toolName string, args map[string]interface{}) string {
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
	return ""
}

// writeDenyAuditAndRespond writes a deny audit record and sends a 400 response. Returns true if caller should return.
func writeDenyAuditAndRespond(ctx context.Context, w http.ResponseWriter, store database.Store, logger *slog.Logger, toolName string, args map[string]interface{}, errMsg string) bool {
	rec := &models.McpToolCallAuditLog{
		ToolName:  toolName,
		Decision:  auditDecisionDeny,
		Status:    auditStatusError,
		ErrorType: strPtr("invalid_arguments"),
		TaskID:    uuidArg(args, "task_id"),
		RunID:     uuidArg(args, "run_id"),
		JobID:     uuidArg(args, "job_id"),
	}
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

// toolCallHandler writes an audit record for every tool call (P2-02) and routes db.preference.* tools (P2-03).
// P2-01: enforces required scoped ids (task_id/run_id/job_id) per tool before routing; rejects with 400 when missing.
func toolCallHandler(store database.Store, logger *slog.Logger) http.HandlerFunc {
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
		var req toolCallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		toolName := req.ToolName
		if toolName == "" {
			toolName = "unknown"
		}
		args := req.Arguments
		if args == nil {
			args = make(map[string]interface{})
		}
		if errMsg := validateRequiredScopedIds(toolName, args); errMsg != "" {
			writeDenyAuditAndRespond(r.Context(), w, store, logger, toolName, args, errMsg)
			return
		}
		routeAndWriteAudit(r.Context(), w, store, logger, toolName, args, time.Now())
	}
}

const (
	auditDecisionDeny  = "deny"
	auditDecisionAllow = "allow"
	auditStatusError   = "error"
	auditStatusSuccess = "success"
)

// routeToolCall dispatches to preference and db tools when applicable; returns status code, response body, and audit record.
func routeToolCall(ctx context.Context, store database.Store, toolName string, args map[string]interface{}) (code int, body []byte, rec *models.McpToolCallAuditLog) {
	rec = &models.McpToolCallAuditLog{Decision: auditDecisionDeny, Status: auditStatusError, ErrorType: strPtr("not_implemented")}
	switch toolName {
	case "db.preference.get":
		code, body, rec = handlePreferenceGet(ctx, store, args, rec)
	case "db.preference.list":
		code, body, rec = handlePreferenceList(ctx, store, args, rec)
	case "db.preference.effective":
		code, body, rec = handlePreferenceEffective(ctx, store, args, rec)
	case "db.preference.create":
		code, body, rec = handlePreferenceCreate(ctx, store, args, rec)
	case "db.preference.update":
		code, body, rec = handlePreferenceUpdate(ctx, store, args, rec)
	case "db.preference.delete":
		code, body, rec = handlePreferenceDelete(ctx, store, args, rec)
	case "db.task.get":
		code, body, rec = handleTaskGet(ctx, store, args, rec)
	case "db.job.get":
		code, body, rec = handleJobGet(ctx, store, args, rec)
	case "artifact.get":
		code, body, rec = handleArtifactGet(ctx, store, args, rec)
	case "skills.create":
		code, body, rec = handleSkillsCreate(ctx, store, args, rec)
	case "skills.list":
		code, body, rec = handleSkillsList(ctx, store, args, rec)
	case "skills.get":
		code, body, rec = handleSkillsGet(ctx, store, args, rec)
	case "skills.update":
		code, body, rec = handleSkillsUpdate(ctx, store, args, rec)
	case "skills.delete":
		code, body, rec = handleSkillsDelete(ctx, store, args, rec)
	default:
		code = http.StatusNotImplemented
		body = []byte(`{"error":"tool routing not implemented"}`)
	}
	return code, body, rec
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
		rec.ErrorType = strPtr("not_found")
		return http.StatusNotFound, []byte(`{"error":"not found"}`)
	}
	if errors.Is(err, database.ErrConflict) {
		rec.ErrorType = strPtr("conflict")
		return http.StatusConflict, []byte(`{"error":"version conflict"}`)
	}
	rec.ErrorType = strPtr("internal_error")
	return http.StatusInternalServerError, []byte(`{"error":"internal error"}`)
}

func handlePreferenceGet(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	scopeType := strArg(args, "scope_type")
	key := strArg(args, "key")
	if scopeType == "" || key == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"scope_type and key required"}`), auditRec
	}
	scopeID := uuidArg(args, "scope_id")
	if scopeType != scopeTypeSystem && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
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
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"scope_type required"}`), auditRec
	}
	scopeID := uuidArg(args, "scope_id")
	if scopeType != scopeTypeSystem && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
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
		rec.ErrorType = strPtr("internal_error")
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
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"task_id required"}`), auditRec
	}
	rec.TaskID = taskID
	effective, err := store.GetEffectivePreferencesForTask(ctx, *taskID)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
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
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"scope_type, key, and value_type required"}`), auditRec
	}
	scopeID := uuidArg(args, "scope_id")
	if scopeType != scopeTypeSystem && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"scope_id required when scope_type is not system"}`), auditRec
	}
	var reason, updatedBy *string
	if r := strArg(args, "reason"); r != "" {
		reason = &r
	}
	ent, err := store.CreatePreference(ctx, scopeType, scopeID, key, value, valueType, reason, updatedBy)
	if err != nil {
		if errors.Is(err, database.ErrExists) {
			rec.Decision = auditDecisionAllow
			rec.Status = auditStatusError
			rec.ErrorType = strPtr("conflict")
			return http.StatusConflict, []byte(`{"error":"preference already exists for scope and key"}`), auditRec
		}
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
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
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"scope_type, key, and value_type required"}`), auditRec
	}
	scopeID := uuidArg(args, "scope_id")
	if scopeType != scopeTypeSystem && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
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
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"scope_type and key required"}`), auditRec
	}
	scopeID := uuidArg(args, "scope_id")
	if scopeType != scopeTypeSystem && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
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
		rec.ErrorType = strPtr("invalid_arguments")
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
		rec.ErrorType = strPtr("invalid_arguments")
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
		"id":       job.ID,
		"task_id":  job.TaskID,
		"status":   job.Status,
		"node_id":  job.NodeID,
		"payload":  job.Payload.Ptr(),
		"result":   job.Result.Ptr(),
		"created_at": job.CreatedAt,
		"updated_at": job.UpdatedAt,
	}
	body, _ = json.Marshal(out)
	return http.StatusOK, body, auditRec
}

func handleArtifactGet(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	taskID := uuidArg(args, "task_id")
	path := strArg(args, "path")
	if taskID == nil || path == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"task_id and path required"}`), auditRec
	}
	rec.TaskID = taskID
	art, err := store.GetArtifactByTaskIDAndPath(ctx, *taskID, path)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{
		"task_id":    art.TaskID,
		"path":       art.Path,
		"storage_ref": art.StorageRef,
		"size_bytes":  art.SizeBytes,
		"content_type": art.ContentType,
		"checksum_sha256": art.ChecksumSHA256,
		"created_at": art.CreatedAt,
		"updated_at": art.UpdatedAt,
	}
	if art.RunID != nil {
		out["run_id"] = art.RunID
	}
	body, _ = json.Marshal(out)
	return http.StatusOK, body, auditRec
}

// --- Skills tools (REQ-SKILLS-0114; contract in skills_storage_and_inference.md) ---

func handleSkillsCreate(ctx context.Context, store database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	taskID := uuidArg(args, "task_id")
	if taskID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"task_id required"}`), auditRec
	}
	rec.TaskID = taskID
	task, err := store.GetTaskByID(ctx, *taskID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	userID := task.CreatedBy
	if userID == nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"task has no owner"}`), auditRec
	}
	content := strArg(args, "content")
	if content == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
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
	skill, err := store.CreateSkill(ctx, name, content, scope, userID, false)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
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
	taskID := uuidArg(args, "task_id")
	if taskID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"task_id required"}`), auditRec
	}
	rec.TaskID = taskID
	task, err := store.GetTaskByID(ctx, *taskID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	userID := task.CreatedBy
	if userID == nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"task has no owner"}`), auditRec
	}
	scopeFilter := strArg(args, "scope")
	ownerFilter := strArg(args, "owner")
	skills, err := store.ListSkillsForUser(ctx, *userID, scopeFilter, ownerFilter)
	if err != nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
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
	taskID := uuidArg(args, "task_id")
	skillIDStr := strArg(args, "skill_id")
	if taskID == nil || skillIDStr == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"task_id and skill_id required"}`), auditRec
	}
	rec.TaskID = taskID
	skillID, err := uuid.Parse(skillIDStr)
	if err != nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"invalid skill_id"}`), auditRec
	}
	task, err := store.GetTaskByID(ctx, *taskID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	userID := task.CreatedBy
	if userID == nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"task has no owner"}`), auditRec
	}
	skill, err := store.GetSkillByID(ctx, skillID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	if !skill.IsSystem && (skill.OwnerID == nil || *skill.OwnerID != *userID) {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("not_found")
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
	taskID := uuidArg(args, "task_id")
	skillIDStr := strArg(args, "skill_id")
	if taskID == nil || skillIDStr == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"task_id and skill_id required"}`), auditRec
	}
	rec.TaskID = taskID
	skillID, err := uuid.Parse(skillIDStr)
	if err != nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"invalid skill_id"}`), auditRec
	}
	task, err := store.GetTaskByID(ctx, *taskID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	userID := task.CreatedBy
	if userID == nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"task has no owner"}`), auditRec
	}
	skill, err := store.GetSkillByID(ctx, skillID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	if !skill.IsSystem && (skill.OwnerID == nil || *skill.OwnerID != *userID) {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("not_found")
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
	taskID := uuidArg(args, "task_id")
	skillIDStr := strArg(args, "skill_id")
	if taskID == nil || skillIDStr == "" {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"task_id and skill_id required"}`), auditRec
	}
	rec.TaskID = taskID
	skillID, err := uuid.Parse(skillIDStr)
	if err != nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"invalid skill_id"}`), auditRec
	}
	task, err := store.GetTaskByID(ctx, *taskID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	userID := task.CreatedBy
	if userID == nil {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("internal_error")
		return http.StatusInternalServerError, []byte(`{"error":"task has no owner"}`), auditRec
	}
	skill, err := store.GetSkillByID(ctx, skillID)
	if err != nil {
		code, body := writePreferenceErrToAudit(err, rec)
		return code, body, auditRec
	}
	if !skill.IsSystem && (skill.OwnerID == nil || *skill.OwnerID != *userID) {
		rec.Decision = auditDecisionAllow
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("not_found")
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

func strPtr(s string) *string { return &s }

func skillsPolicyViolationResponse(rec *models.McpToolCallAuditLog, m *skillscan.Match, auditRec *models.McpToolCallAuditLog) (code int, body []byte, outRec *models.McpToolCallAuditLog) {
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusError
	rec.ErrorType = strPtr("policy_violation")
	out := map[string]interface{}{"error": "policy violation", "category": m.Category, "triggering_text": m.TriggeringText}
	b, _ := json.Marshal(out)
	return http.StatusBadRequest, b, auditRec
}
