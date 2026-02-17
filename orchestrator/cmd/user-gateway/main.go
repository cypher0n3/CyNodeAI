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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.LoadOrchestratorConfig()

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	jwtManager := auth.NewJWTManager(
		cfg.JWTSecret,
		cfg.JWTAccessDuration,
		cfg.JWTRefreshDuration,
		cfg.JWTNodeDuration,
	)
	rateLimiter := auth.NewRateLimiter(cfg.RateLimitPerMinute, time.Minute)

	authHandler := handlers.NewAuthHandler(db, jwtManager, rateLimiter, logger)
	userHandler := handlers.NewUserHandler(db, logger)
	taskHandler := handlers.NewTaskHandler(db, logger)

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
	mux.Handle("GET /v1/tasks/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTask)))
	mux.Handle("GET /v1/tasks/{id}/result", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTaskResult)))

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

	go func() {
		logger.Info("starting user-gateway", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
	logger.Info("server stopped")
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
