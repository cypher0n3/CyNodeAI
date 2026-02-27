// Package main provides the cynode-pma agent binary.
// See docs/tech_specs/cynode_pma.md and docs/requirements/pmagnt.md.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cypher0n3/cynodeai/agents/internal/pma"
)

func main() {
	os.Exit(runWithSignal(os.Args[1:]))
}

// runWithSignal sets up signal handling and runs the server. Used by main and tests for coverage.
func runWithSignal(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, args)
}

func run(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("cynode-pma", flag.ContinueOnError)
	role := fs.String("role", "", "Agent role: project_manager or project_analyst (or set PMA_ROLE)")
	instructionsRoot := fs.String("instructions-root", pma.DefaultInstructionsRoot, "Root directory for role instruction bundles (or PMA_INSTRUCTIONS_ROOT)")
	instructionsPM := fs.String("instructions-project-manager", "", "Override path for project_manager bundle (or PMA_INSTRUCTIONS_PROJECT_MANAGER)")
	instructionsPA := fs.String("instructions-project-analyst", "", "Override path for project_analyst bundle (or PMA_INSTRUCTIONS_PROJECT_ANALYST)")
	listenAddr := fs.String("listen", ":8090", "HTTP listen address (or PMA_LISTEN_ADDR)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg := &pma.Config{
		Role:                       pma.Role(resolveRole(*role)),
		InstructionsRoot:           resolveEnv("PMA_INSTRUCTIONS_ROOT", *instructionsRoot),
		InstructionsProjectManager: resolveEnv("PMA_INSTRUCTIONS_PROJECT_MANAGER", *instructionsPM),
		InstructionsProjectAnalyst: resolveEnv("PMA_INSTRUCTIONS_PROJECT_ANALYST", *instructionsPA),
		ListenAddr:                 resolveEnv("PMA_LISTEN_ADDR", *listenAddr),
	}

	if cfg.Role != pma.RoleProjectManager && cfg.Role != pma.RoleProjectAnalyst {
		slog.Error("invalid or missing role", "role", cfg.Role, "hint", "set --role=project_manager or --role=project_analyst (or PMA_ROLE)")
		return 1
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	instructionsPath := cfg.InstructionsPath()
	roleContent, err := pma.LoadInstructions(instructionsPath)
	if err != nil {
		slog.Error("failed to load instructions", "path", instructionsPath, "error", err)
		return 1
	}
	rootDir := filepath.Dir(instructionsPath)
	defaultSkill, err := pma.LoadDefaultSkill(rootDir)
	if err != nil {
		slog.Error("failed to load default skill", "root", rootDir, "error", err)
		return 1
	}
	content := roleContent
	if defaultSkill != "" {
		content = strings.TrimSpace(content + "\n\n" + defaultSkill)
	}
	slog.Info("instructions loaded", "role", cfg.Role, "path", instructionsPath, "bytes", len(content))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /internal/chat/completion", pma.ChatCompletionHandler(content, logger))

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("cynode-pma listening", "role", cfg.Role, "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if err := shutdownServer(shutdownCtx, server); err != nil {
		logger.Error("shutdown error", "error", err)
		return 1
	}
	logger.Info("stopped")
	return 0
}

var shutdownServer = func(ctx context.Context, srv *http.Server) error { return srv.Shutdown(ctx) }

func resolveRole(flagRole string) string {
	if flagRole != "" {
		return flagRole
	}
	if v := os.Getenv("PMA_ROLE"); v != "" {
		return v
	}
	return ""
}

func resolveEnv(key, flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	return flagVal
}
