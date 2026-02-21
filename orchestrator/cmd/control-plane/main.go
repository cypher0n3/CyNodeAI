// Package main provides the orchestrator control-plane API.
// See docs/tech_specs/orchestrator.md.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/handlers"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/middleware"
)

// testShutdownTimeout, when set by tests, overrides the server shutdown timeout.
var testShutdownTimeout *time.Duration

// testShutdownHook, when set by tests, is called instead of server.Shutdown so tests can force a shutdown error.
var testShutdownHook func(*http.Server, context.Context) error

// testOpenStore, when set by tests, is used instead of database.Open when store is nil so store==nil path can be covered without a real DB.
var testOpenStore func(context.Context, string) (database.Store, error)

// testDatabaseOpen, when set by tests, is used instead of database.Open when both store and testOpenStore are nil (allows covering open-success path without a real DB).
var testDatabaseOpen func(context.Context, string) (database.Store, error)

func main() {
	if code := runMain(); code != 0 {
		os.Exit(code)
	}
}

// runMain runs the control-plane and returns an exit code. Used by main and tests.
func runMain() int {
	return runMainWithContext(context.Background(), nil)
}

// resolveStore opens the DB when store is nil (using testOpenStore or database.Open). Returns (store, nil), (nil, nil) when migrateOnly after open, or (nil, err).
//
//nolint:gocognit,dupl // test hooks and real open share the same migrateOnly handling by design
func resolveStore(ctx context.Context, store database.Store, cfg *config.OrchestratorConfig, logger *slog.Logger, migrateOnly bool) (database.Store, error) {
	if store != nil {
		return store, nil
	}
	if testOpenStore != nil {
		var err error
		store, err = testOpenStore(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("failed to connect to database", "error", err)
			return nil, err
		}
		if store != nil && migrateOnly {
			logger.Info("schema applied (migrate-only)")
			return nil, nil
		}
		return store, nil
	}
	if testDatabaseOpen != nil {
		var err error
		store, err = testDatabaseOpen(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("failed to connect to database", "error", err)
			return nil, err
		}
		if store != nil && migrateOnly {
			logger.Info("schema applied (migrate-only)")
			return nil, nil
		}
		return store, nil
	}
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return nil, err
	}
	if err := db.RunSchema(ctx, logger); err != nil {
		logger.Error("failed to run schema", "error", err)
		_ = db.Close()
		return nil, err
	}
	if migrateOnly {
		logger.Info("schema applied (migrate-only)")
		_ = db.Close()
		return nil, nil
	}
	return db, nil
}

// runMainWithContext runs the control-plane with an optional store (for tests). When store is nil, opens DB from config.
// Used by tests to exercise the full success path without a real database.
func runMainWithContext(ctx context.Context, store database.Store) int {
	fs := flag.NewFlagSet("control-plane", flag.ContinueOnError)
	var migrateOnly bool
	fs.BoolVar(&migrateOnly, "migrate-only", false, "run database migrations and exit")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.LoadOrchestratorConfig()

	var err error
	store, err = resolveStore(ctx, store, cfg, logger, migrateOnly)
	if err != nil {
		return 1
	}
	if store == nil {
		return 0
	}
	if migrateOnly {
		logger.Info("schema applied (migrate-only)")
		return 0
	}

	if err := run(ctx, store, cfg, logger); err != nil {
		logger.Error("run failed", "error", err)
		return 1
	}
	return 0
}

// run bootstraps admin, starts the HTTP server and dispatcher until ctx is cancelled. Used by main and tests.
func run(ctx context.Context, store database.Store, cfg *config.OrchestratorConfig, logger *slog.Logger) error {
	if err := bootstrapAdminUser(ctx, store, cfg.BootstrapAdminPassword, logger); err != nil {
		return err
	}

	jwtManager := auth.NewJWTManager(
		cfg.JWTSecret,
		cfg.JWTAccessDuration,
		cfg.JWTRefreshDuration,
		cfg.JWTNodeDuration,
	)

	nodeHandler := handlers.NewNodeHandler(store, jwtManager, cfg.NodeRegistrationPSK, cfg.OrchestratorPublicURL, cfg.WorkerAPIBearerToken, cfg.WorkerAPITargetURL, logger)
	authMiddleware := middleware.NewAuthMiddleware(jwtManager, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", readyzHandler(store, logger))

	mux.HandleFunc("POST /v1/nodes/register", nodeHandler.Register)
	mux.Handle("GET /v1/nodes/config", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.GetConfig)))
	mux.Handle("POST /v1/nodes/config", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.ConfigAck)))
	mux.Handle("POST /v1/nodes/capability", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.ReportCapability)))

	handler := middleware.Recovery(logger)(middleware.Logging(logger)(mux))

	addr := getEnv("CONTROL_PLANE_LISTEN_ADDR", getEnv("LISTEN_ADDR", ":8082"))
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	go startDispatcher(ctx, store, logger)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting control-plane", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
	case <-done:
	case err := <-serverErr:
		return err
	}

	logger.Info("shutting down...")
	shutdownTimeout := 30 * time.Second
	if testShutdownTimeout != nil {
		shutdownTimeout = *testShutdownTimeout
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	var shutdownErr error
	if testShutdownHook != nil {
		shutdownErr = testShutdownHook(server, shutdownCtx)
	} else {
		shutdownErr = server.Shutdown(shutdownCtx)
	}
	if shutdownErr != nil {
		logger.Error("shutdown error", "error", shutdownErr)
		return shutdownErr
	}
	logger.Info("server stopped")
	return nil
}

// readyzHandler returns a handler for GET /readyz. Returns 200 with body "ready" when at least one
// inference-capable path exists (dispatchable node); otherwise 503 with an actionable reason.
// See REQ-ORCHES-0119 and CYNAI.ORCHES.Rule.HealthEndpoints.
func readyzHandler(store database.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		nodes, err := store.ListDispatchableNodes(ctx)
		if err != nil {
			if logger != nil {
				logger.Error("readyz check failed", "error", err)
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("readiness check failed (database error)"))
			return
		}
		if len(nodes) == 0 {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("no inference path available (no dispatchable nodes; register and configure a worker node or configure external provider keys)"))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func bootstrapAdminUser(ctx context.Context, db database.Store, password string, logger *slog.Logger) error {
	_, err := db.GetUserByHandle(ctx, "admin")
	if err == nil {
		logger.Info("admin user already exists")
		return nil
	}
	if !errors.Is(err, database.ErrNotFound) {
		return err
	}

	user, err := db.CreateUser(ctx, "admin", nil)
	if err != nil {
		return err
	}

	passwordHash, err := auth.HashPassword(password, nil)
	if err != nil {
		return err
	}

	_, err = db.CreatePasswordCredential(ctx, user.ID, passwordHash, "argon2id")
	if err != nil {
		return err
	}

	logger.Info("admin user created", "handle", "admin")
	return nil
}
