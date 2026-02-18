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

func main() {
	if code := runMain(); code != 0 {
		os.Exit(code)
	}
}

// runMain runs the control-plane and returns an exit code. Used by main and tests.
func runMain() int {
	var migrateOnly bool
	flag.BoolVar(&migrateOnly, "migrate-only", false, "run database migrations and exit")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.LoadOrchestratorConfig()

	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return 1
	}
	defer func() { _ = db.Close() }()

	if err := db.RunSchema(ctx, logger); err != nil {
		logger.Error("failed to run schema", "error", err)
		return 1
	}
	if migrateOnly {
		logger.Info("schema applied (migrate-only)")
		return 0
	}

	var store database.Store = db
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

	nodeHandler := handlers.NewNodeHandler(store, jwtManager, cfg.NodeRegistrationPSK, cfg.OrchestratorPublicURL, logger)
	authMiddleware := middleware.NewAuthMiddleware(jwtManager, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("POST /v1/nodes/register", nodeHandler.Register)
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
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
		return err
	}
	logger.Info("server stopped")
	return nil
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
