// Package main provides the standalone MCP gateway binary (optional).
// Production deployments should serve POST /v1/mcp/tools/call on the control plane instead;
// this process exists for integration tests and local tooling.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/artifacts"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/mcpgateway"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/middleware"
)

func mcpAuthFromEnv() *mcpgateway.ToolCallAuth {
	return &mcpgateway.ToolCallAuth{
		PMToken:      getEnv("WORKER_INTERNAL_AGENT_TOKEN", ""),
		SandboxToken: getEnv("MCP_SANDBOX_AGENT_BEARER_TOKEN", ""),
		PAToken:      getEnv("MCP_PA_AGENT_BEARER_TOKEN", ""),
	}
}

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

// testShutdownHook, when set by tests, is called instead of srv.Shutdown.
var testShutdownHook func(*http.Server, context.Context) error

// openGatewayStore resolves the database handle for the standalone gateway (tests may inject mocks).
func openGatewayStore(ctx context.Context, logger *slog.Logger) (store database.Store, closeFn func(), err error) {
	switch {
	case testStore != nil:
		return testStore, nil, nil
	case testDatabaseOpen != nil:
		dsn := getEnv("DATABASE_URL", "")
		store, err := testDatabaseOpen(ctx, dsn)
		if err != nil {
			return nil, nil, err
		}
		logger.Info("deprecated standalone MCP gateway: database connected")
		return store, nil, nil
	default:
		dsn := getEnv("DATABASE_URL", "")
		if dsn == "" {
			logger.Warn("DATABASE_URL not set; POST /v1/mcp/tools/call returns 503 until a database is configured")
			return nil, nil, nil
		}
		db, err := database.Open(ctx, dsn)
		if err != nil {
			return nil, nil, err
		}
		if err := db.RunSchema(ctx, logger); err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		logger.Info("deprecated standalone MCP gateway: database connected")
		return db, func() { _ = db.Close() }, nil
	}
}

// run sets up and runs the server until ctx is canceled. Used by main and tests.
// When DATABASE_URL is set (or testStore/testDatabaseOpen is set in tests), tool-call handler writes audit records.
func run(ctx context.Context, logger *slog.Logger) error {
	cfg := config.LoadOrchestratorConfig()
	if err := config.ValidateSecrets(cfg); err != nil {
		return err
	}
	store, closeDB, err := openGatewayStore(ctx, logger)
	if err != nil {
		return err
	}
	if closeDB != nil {
		defer closeDB()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	if db, ok := store.(*database.DB); ok {
		artSvc, aerr := artifacts.NewServiceFromConfig(ctx, db, cfg)
		if aerr != nil {
			logger.Warn("artifacts backend unavailable; MCP artifact tools disabled", "error", aerr)
			mcpgateway.SetArtifactToolService(nil)
		} else {
			mcpgateway.SetArtifactToolService(artSvc)
		}
	} else {
		mcpgateway.SetArtifactToolService(nil)
	}
	mux.HandleFunc("POST /v1/mcp/tools/call", mcpgateway.ToolCallHandler(store, logger, mcpAuthFromEnv()))

	handler := middleware.Logging(logger)(mux)
	srv := &http.Server{
		Addr:              getEnv("LISTEN_ADDR", ":8083"),
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting deprecated standalone MCP gateway (use control-plane POST /v1/mcp/tools/call instead)", "addr", srv.Addr)
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
	logger.Info("shutting down deprecated standalone MCP gateway")
	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout())
	defer cancel()
	if testShutdownHook != nil {
		return testShutdownHook(srv, shutdownCtx)
	}
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
