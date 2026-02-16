// Package main is the entry point for the CyNodeAI Orchestrator API.
// See docs/tech_specs/orchestrator.md for architecture details.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cypher0n3/cynodeai/internal/auth"
	"github.com/cypher0n3/cynodeai/internal/config"
	"github.com/cypher0n3/cynodeai/internal/database"
	"github.com/cypher0n3/cynodeai/internal/handlers"
	"github.com/cypher0n3/cynodeai/internal/middleware"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.LoadOrchestratorConfig()

	// Connect to database
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Set migrations filesystem and run migrations
	ctx := context.Background()
	migrationsSubFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		logger.Error("failed to get migrations sub-filesystem", "error", err)
		os.Exit(1)
	}
	database.MigrationsFS = migrationsSubFS
	if err := db.RunMigrations(ctx, logger); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Bootstrap admin user
	if err := bootstrapAdminUser(ctx, db, cfg.BootstrapAdminPassword, logger); err != nil {
		logger.Error("failed to bootstrap admin user", "error", err)
		os.Exit(1)
	}

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(
		cfg.JWTSecret,
		cfg.JWTAccessDuration,
		cfg.JWTRefreshDuration,
		cfg.JWTNodeDuration,
	)

	// Initialize rate limiter
	rateLimiter := auth.NewRateLimiter(cfg.RateLimitPerMinute, time.Minute)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db, jwtManager, rateLimiter, logger)
	userHandler := handlers.NewUserHandler(db, logger)
	taskHandler := handlers.NewTaskHandler(db, logger)
	nodeHandler := handlers.NewNodeHandler(db, jwtManager, cfg.NodeRegistrationPSK, logger)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtManager, logger)

	// Setup router
	mux := http.NewServeMux()

	// Health check (no auth)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Auth endpoints (no auth required)
	mux.HandleFunc("POST /v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /v1/auth/refresh", authHandler.Refresh)

	// Node registration (PSK auth, not JWT)
	mux.HandleFunc("POST /v1/nodes/register", nodeHandler.Register)

	// Protected user endpoints
	mux.Handle("POST /v1/auth/logout", authMiddleware.RequireUserAuth(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("GET /v1/users/me", authMiddleware.RequireUserAuth(http.HandlerFunc(userHandler.GetMe)))
	mux.Handle("POST /v1/tasks", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.CreateTask)))
	mux.Handle("GET /v1/tasks/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTask)))
	mux.Handle("GET /v1/tasks/{id}/result", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTaskResult)))

	// Protected node endpoints
	mux.Handle("POST /v1/nodes/capability", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.ReportCapability)))

	// Apply middleware
	handler := middleware.Recovery(logger)(middleware.Logging(logger)(mux))

	// Create server with timeouts per go_rest_api_standards.md
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("starting orchestrator API", "addr", cfg.ListenAddr)
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

// bootstrapAdminUser creates the default admin user if it doesn't exist.
func bootstrapAdminUser(ctx context.Context, db *database.DB, password string, logger *slog.Logger) error {
	// Check if admin user exists
	_, err := db.GetUserByHandle(ctx, "admin")
	if err == nil {
		logger.Info("admin user already exists")
		return nil
	}
	if !errors.Is(err, database.ErrNotFound) {
		return err
	}

	// Create admin user
	user, err := db.CreateUser(ctx, "admin", nil)
	if err != nil {
		return err
	}

	// Hash password
	passwordHash, err := auth.HashPassword(password, nil)
	if err != nil {
		return err
	}

	// Create password credential
	_, err = db.CreatePasswordCredential(ctx, user.ID, passwordHash, "argon2id")
	if err != nil {
		return err
	}

	logger.Info("admin user created", "handle", "admin")
	return nil
}
