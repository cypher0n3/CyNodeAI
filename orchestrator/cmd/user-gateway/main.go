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

	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/artifacts"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/handlers"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/middleware"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/readiness"
)

// testShutdownHook, when set by tests, is called instead of server.Shutdown so tests can cover the shutdown error path.
var testShutdownHook func(*http.Server, context.Context) error

// testDatabaseOpen, when set by tests, is used instead of database.Open in runMain (e.g. to run RunSchema before worker bearer migration).
var testDatabaseOpen func(context.Context, string) (*database.DB, error)

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
	if err := config.ValidateSecrets(cfg); err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}
	var db *database.DB
	var err error
	if testDatabaseOpen != nil {
		db, err = testDatabaseOpen(ctx, cfg.DatabaseURL)
	} else {
		db, err = database.Open(ctx, cfg.DatabaseURL)
	}
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	if err := database.ApplyWorkerBearerEncryptionAtStartup(ctx, db, cfg.JWTSecret); err != nil {
		logger.Error("migrate worker bearer tokens", "error", err)
		return 1
	}
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

// run sets up handlers and runs the server until ctx is canceled. Used by main and tests.
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
	openAIChatHandler := handlers.NewOpenAIChatHandler(store, logger, cfg.InferenceURL, cfg.InferenceModel, cfg.WorkerAPIBearerToken)
	skillsHandler := handlers.NewSkillsHandler(store, logger)

	var artSvc *artifacts.Service
	if db, ok := store.(*database.DB); ok {
		var aerr error
		artSvc, aerr = artifacts.NewServiceFromConfig(ctx, db, cfg)
		if aerr != nil {
			logger.Warn("artifacts backend unavailable", "error", aerr)
			artSvc = nil
		}
	}
	artifactsHandler := handlers.NewArtifactsHandler(artSvc, logger)

	if err := store.EnsureDefaultSkill(ctx, defaultSkillContent); err != nil {
		logger.Warn("ensure default skill", "error", err)
	}

	authMiddleware := middleware.NewAuthMiddleware(jwtManager, logger)

	mux := http.NewServeMux()
	plainTextOK := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}
	}
	mux.HandleFunc("GET /healthz", plainTextOK("ok"))
	mux.HandleFunc("GET /readyz", gatewayReadyzHandler(store, cfg, logger))

	maxBodyBytes := int64(cfg.MaxRequestBodyMB) * 1024 * 1024

	mux.HandleFunc("POST /v1/auth/login", httplimits.LimitBody(maxBodyBytes, authHandler.Login))
	mux.HandleFunc("POST /v1/auth/refresh", httplimits.LimitBody(maxBodyBytes, authHandler.Refresh))

	mux.Handle("POST /v1/auth/logout", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(maxBodyBytes, authHandler.Logout))))
	mux.Handle("GET /v1/users/me", authMiddleware.RequireUserAuth(http.HandlerFunc(userHandler.GetMe)))
	mux.Handle("POST /v1/users/{id}/revoke_sessions", authMiddleware.RequireAdminAuth(http.HandlerFunc(userHandler.RevokeSessions)))
	mux.Handle("POST /v1/tasks", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(maxBodyBytes, taskHandler.CreateTask))))
	mux.Handle("POST /v1/tasks/{id}/ready", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(maxBodyBytes, taskHandler.PostTaskReady))))
	mux.Handle("GET /v1/tasks", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.ListTasks)))
	mux.Handle("GET /v1/tasks/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTask)))
	mux.Handle("GET /v1/tasks/{id}/result", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTaskResult)))
	mux.Handle("POST /v1/tasks/{id}/cancel", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.CancelTask)))
	mux.Handle("GET /v1/tasks/{id}/logs", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTaskLogs)))
	mux.Handle("GET /v1/models", authMiddleware.RequireUserAuth(http.HandlerFunc(openAIChatHandler.ListModels)))
	mux.Handle("POST /v1/chat/completions", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(maxBodyBytes, openAIChatHandler.ChatCompletions))))
	mux.Handle("POST /v1/responses", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(maxBodyBytes, openAIChatHandler.Responses))))
	mux.Handle("POST /v1/chat/threads", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(maxBodyBytes, openAIChatHandler.NewThread))))
	mux.Handle("GET /v1/chat/threads", authMiddleware.RequireUserAuth(http.HandlerFunc(openAIChatHandler.ListThreads)))
	mux.Handle("GET /v1/chat/threads/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { openAIChatHandler.GetThread(w, r, r.PathValue("id")) })))
	mux.Handle("GET /v1/chat/threads/{id}/messages", authMiddleware.RequireUserAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		openAIChatHandler.ListThreadMessages(w, r, r.PathValue("id"))
	})))
	mux.Handle("PATCH /v1/chat/threads/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(maxBodyBytes, func(w http.ResponseWriter, r *http.Request) {
		openAIChatHandler.PatchThreadTitle(w, r, r.PathValue("id"))
	}))))
	mux.Handle("GET /v1/skills", authMiddleware.RequireUserAuth(http.HandlerFunc(skillsHandler.List)))
	mux.Handle("GET /v1/skills/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(skillsHandler.Get)))
	mux.Handle("POST /v1/skills/load", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(maxBodyBytes, skillsHandler.Load))))
	mux.Handle("PUT /v1/skills/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(maxBodyBytes, skillsHandler.Update))))
	mux.Handle("DELETE /v1/skills/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(skillsHandler.Delete)))

	mux.Handle("POST /v1/artifacts", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(httplimits.DefaultMaxArtifactUploadBytes, artifactsHandler.Create))))
	mux.Handle("GET /v1/artifacts", authMiddleware.RequireUserAuth(http.HandlerFunc(artifactsHandler.Find)))
	mux.Handle("GET /v1/artifacts/{artifact_id}", authMiddleware.RequireUserAuth(http.HandlerFunc(artifactsHandler.Read)))
	mux.Handle("PUT /v1/artifacts/{artifact_id}", authMiddleware.RequireUserAuth(http.HandlerFunc(httplimits.LimitBody(httplimits.DefaultMaxArtifactUploadBytes, artifactsHandler.Update))))
	mux.Handle("DELETE /v1/artifacts/{artifact_id}", authMiddleware.RequireUserAuth(http.HandlerFunc(artifactsHandler.Delete)))

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

// defaultSkillContent is the built-in CyNodeAI interaction skill (REQ-SKILLS-0116). Content updated per release.
const defaultSkillContent = `# CyNodeAI interaction

Use MCP tools and the User API Gateway as documented. Follow task and project context. See docs/requirements and docs/tech_specs for authoritative behavior.`

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// gatewayReadyzHandler returns a readiness handler per CYNAI.ORCHES.Rule.HealthEndpoints.
// Returns 200 only when an inference path exists and PMA is ready (worker-managed or local subprocess).
func gatewayReadyzHandler(store database.Store, cfg *config.OrchestratorConfig, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		hasPath, err := readiness.InferencePathAvailable(ctx, store)
		if err != nil {
			if logger != nil {
				logger.Error("readyz inference check failed", "error", err)
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("readiness check failed (database error)"))
			return
		}
		if !hasPath {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("no inference path available (no dispatchable nodes; register and configure a worker node or configure external provider keys)"))
			return
		}
		localReady := cfg != nil && cfg.PMAEnabled && readiness.PMASubprocessReady(ctx, cfg.PMAListenAddr)
		workerReady := readiness.HasWorkerReportedPMAReady(ctx, store)
		if !localReady && !workerReady {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("PMA not ready (no local PMA and no worker has reported PMA ready)"))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}
