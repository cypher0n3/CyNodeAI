// Package main provides the user-facing API gateway.
// See docs/tech_specs/user_api_gateway.md.
package main

import (
	"context"
	"errors"
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
	os.Exit(runMain(context.Background()))
}

// runMain loads config, opens DB, and runs the server. Returns 0 on success, 1 on failure. Used by main and tests.
func runMain(ctx context.Context) int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
	cfg := config.LoadOrchestratorConfig()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	return runMainWithStore(ctx, cfg, db, logger)
}

// runMainWithStore runs the server with the given store. Returns 0 on success, 1 on failure. Used by tests to hit success path without opening DB.
func runMainWithStore(ctx context.Context, cfg *config.OrchestratorConfig, store database.Store, logger *slog.Logger) int {
	if err := run(ctx, cfg, store, logger); err != nil {
		logger.Error("run failed", "error", err)
		return 1
	}
	return 0
}

// run sets up handlers and runs the server until ctx is cancelled. Used by main and tests.
func run(ctx context.Context, cfg *config.OrchestratorConfig, store database.Store, logger *slog.Logger) error {
	jwtManager := auth.NewJWTManager(
		cfg.JWTSecret,
		cfg.JWTAccessDuration,
		cfg.JWTRefreshDuration,
		cfg.JWTNodeDuration,
	)
	rateLimiter := auth.NewRateLimiter(cfg.RateLimitPerMinute, time.Minute)

	authHandler := handlers.NewAuthHandler(store, jwtManager, rateLimiter, logger)
	userHandler := handlers.NewUserHandler(store, logger)
	taskHandler := handlers.NewTaskHandler(store, logger, cfg.InferenceURL, cfg.InferenceModel)

	authMiddleware := middleware.NewAuthMiddleware(jwtManager, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	maxBodyBytes := int64(cfg.MaxRequestBodyMB) * 1024 * 1024

	mux.HandleFunc("POST /v1/auth/login", limitBody(maxBodyBytes, authHandler.Login))
	mux.HandleFunc("POST /v1/auth/refresh", limitBody(maxBodyBytes, authHandler.Refresh))

	mux.Handle("POST /v1/auth/logout", authMiddleware.RequireUserAuth(http.HandlerFunc(limitBody(maxBodyBytes, authHandler.Logout))))
	mux.Handle("GET /v1/users/me", authMiddleware.RequireUserAuth(http.HandlerFunc(userHandler.GetMe)))
	mux.Handle("POST /v1/tasks", authMiddleware.RequireUserAuth(http.HandlerFunc(limitBody(maxBodyBytes, taskHandler.CreateTask))))
	mux.Handle("GET /v1/tasks", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.ListTasks)))
	mux.Handle("GET /v1/tasks/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTask)))
	mux.Handle("GET /v1/tasks/{id}/result", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTaskResult)))
	mux.Handle("POST /v1/tasks/{id}/cancel", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.CancelTask)))
	mux.Handle("GET /v1/tasks/{id}/logs", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTaskLogs)))
	mux.Handle("POST /v1/chat", authMiddleware.RequireUserAuth(http.HandlerFunc(limitBody(maxBodyBytes, taskHandler.Chat))))

	handler := middleware.Recovery(logger)(middleware.Logging(logger)(mux))

	addr := getEnv("USER_GATEWAY_LISTEN_ADDR", getEnv("LISTEN_ADDR", ":8080"))
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting user-gateway", "addr", server.Addr)
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

func limitBody(maxBytes int64, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next(w, r)
	}
}
