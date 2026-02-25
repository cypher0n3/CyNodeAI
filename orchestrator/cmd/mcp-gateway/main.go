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
)

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

// run sets up and runs the server until ctx is cancelled. Used by main and tests.
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

// toolCallHandler writes an audit record for every tool call (P2-02) and routes db.preference.* tools (P2-03).
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
		start := time.Now()
		code, body, rec := routeToolCall(r.Context(), store, toolName, req.Arguments)
		rec.ToolName = toolName
		if rec.DurationMs == nil {
			ms := int(time.Since(start).Milliseconds())
			rec.DurationMs = &ms
		}
		if err := store.CreateMcpToolCallAuditLog(r.Context(), rec); err != nil {
			logger.Error("create mcp tool call audit log", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write(body)
	}
}

const (
	auditDecisionDeny  = "deny"
	auditDecisionAllow = "allow"
	auditStatusError   = "error"
	auditStatusSuccess = "success"
)

// routeToolCall dispatches to preference tools when applicable; returns status code, response body, and audit record.
func routeToolCall(ctx context.Context, store database.Store, toolName string, args map[string]interface{}) (code int, body []byte, rec *models.McpToolCallAuditLog) {
	rec = &models.McpToolCallAuditLog{Decision: auditDecisionDeny, Status: auditStatusError, ErrorType: strPtr("not_implemented")}
	switch toolName {
	case "db.preference.get":
		code, body, rec = handlePreferenceGet(ctx, store, args, rec)
	case "db.preference.list":
		code, body, rec = handlePreferenceList(ctx, store, args, rec)
	case "db.preference.effective":
		code, body, rec = handlePreferenceEffective(ctx, store, args, rec)
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
	if scopeType != "system" && scopeID == nil {
		rec.Decision = auditDecisionDeny
		rec.Status = auditStatusError
		rec.ErrorType = strPtr("invalid_arguments")
		return http.StatusBadRequest, []byte(`{"error":"scope_id required when scope_type is not system"}`), auditRec
	}
	ent, err := store.GetPreference(ctx, scopeType, scopeID, key)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			rec.Decision = auditDecisionAllow
			rec.Status = auditStatusError
			rec.ErrorType = strPtr("not_found")
			return http.StatusNotFound, []byte(`{"error":"not found"}`), auditRec
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
	if scopeType != "system" && scopeID == nil {
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

func strPtr(s string) *string { return &s }
