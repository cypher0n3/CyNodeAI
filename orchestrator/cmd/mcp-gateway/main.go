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

// toolCallRequest is a minimal body for POST /v1/mcp/tools/call (MCP tool call). Full MCP protocol TBD.
type toolCallRequest struct {
	ToolName string `json:"tool_name"`
}

// toolCallHandler writes an audit record for every tool call (P2-02) and returns 501 until tool routing is implemented.
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
		rec := &models.McpToolCallAuditLog{
			ToolName: toolName,
			Decision: "deny",
			Status:   "error",
			ErrorType: strPtr("not_implemented"),
		}
		if err := store.CreateMcpToolCallAuditLog(r.Context(), rec); err != nil {
			logger.Error("create mcp tool call audit log", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte("tool routing not implemented"))
	}
}

func strPtr(s string) *string { return &s }
