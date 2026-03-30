// Package main provides the cynode-pma agent binary.
// See docs/tech_specs/cynode_pma.md and docs/requirements/pmagnt.md.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cypher0n3/cynodeai/agents/internal/pma"
)

// pmaHTTPWriteTimeout is the http.Server WriteTimeout for PMA. Use 0 to disable the write
// deadline so streaming responses are not cut off before LangchainCompletionTimeout elapses.
const pmaHTTPWriteTimeout = 0

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
	listenAddr := fs.String("listen", "", "HTTP listen address (or PMA_LISTEN_ADDR; default :8090)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	listenAddrResolved := resolveEnv("PMA_LISTEN_ADDR", *listenAddr)
	if listenAddrResolved == "" {
		listenAddrResolved = ":8090"
	}
	cfg := &pma.Config{
		Role:                       pma.Role(resolveRole(*role)),
		InstructionsRoot:           resolveEnv("PMA_INSTRUCTIONS_ROOT", *instructionsRoot),
		InstructionsProjectManager: resolveEnv("PMA_INSTRUCTIONS_PROJECT_MANAGER", *instructionsPM),
		InstructionsProjectAnalyst: resolveEnv("PMA_INSTRUCTIONS_PROJECT_ANALYST", *instructionsPA),
		ListenAddr:                 listenAddrResolved,
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
		WriteTimeout:      pmaHTTPWriteTimeout,
		IdleTimeout:       120 * time.Second,
	}

	go serveHTTP(server, cfg.ListenAddr, logger)
	pma.StartKeepWarm(ctx, logger)

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

// serveHTTP starts the HTTP server on the given listen address.
// If listenAddr starts with "unix:", it binds a Unix domain socket; otherwise uses TCP.
func serveHTTP(server *http.Server, listenAddr string, logger *slog.Logger) {
	logger.Info("cynode-pma listening", "addr", listenAddr)
	var serveErr error
	if strings.HasPrefix(listenAddr, "unix:") {
		serveErr = serveUnix(server, strings.TrimPrefix(listenAddr, "unix:"), logger)
	} else {
		serveErr = server.ListenAndServe()
	}
	if serveErr != nil && serveErr != http.ErrServerClosed {
		logger.Error("server error", "error", serveErr)
	}
}

func serveUnix(server *http.Server, sockPath string, logger *slog.Logger) error {
	if err := os.MkdirAll(filepath.Dir(sockPath), 0o700); err != nil {
		logger.Error("failed to create socket directory", "path", sockPath, "error", err)
		return err
	}
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(sockPath, 0o600); err != nil {
		logger.Warn("failed to set socket permissions", "path", sockPath, "error", err)
	}
	return server.Serve(ln)
}

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
