// Package main is the entry point for the CyNodeAI Node Manager.
// See docs/tech_specs/node.md for architecture details.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/cypher0n3/cynodeai/internal/config"
	"github.com/cypher0n3/cynodeai/internal/handlers"
	"github.com/cypher0n3/cynodeai/internal/middleware"
	"github.com/cypher0n3/cynodeai/internal/worker"
)

// nodeCredentials holds the node's authentication state.
var nodeCredentials struct {
	jwt       string
	jwtExpiry time.Time
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.LoadNodeConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register with orchestrator
	if err := registerWithOrchestrator(ctx, cfg, logger); err != nil {
		logger.Error("failed to register with orchestrator", "error", err)
		os.Exit(1)
	}

	// Initialize executor
	executor := worker.NewExecutor(
		cfg.ContainerRuntime,
		time.Duration(cfg.DefaultTimeoutSeconds)*time.Second,
		cfg.MaxOutputBytes,
	)

	// Setup router for Worker API
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Worker API endpoints
	// See docs/tech_specs/worker_api.md
	mux.HandleFunc("POST /v1/worker/jobs:run", func(w http.ResponseWriter, r *http.Request) {
		// Validate bearer token from orchestrator
		// In production, this would validate against the token from config payload
		// For MVP, we accept any valid bearer token format

		var req worker.JobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			handlers.WriteBadRequest(w, "Invalid request body")
			return
		}

		if req.Version != 1 {
			handlers.WriteBadRequest(w, "Unsupported version")
			return
		}

		if len(req.Sandbox.Command) == 0 {
			handlers.WriteBadRequest(w, "Command is required")
			return
		}

		logger.Info("executing job",
			"task_id", req.TaskID,
			"job_id", req.JobID,
			"image", req.Sandbox.Image,
		)

		resp, err := executor.RunJob(r.Context(), &req)
		if err != nil {
			logger.Error("job execution error", "error", err)
			handlers.WriteInternalError(w, "Job execution failed")
			return
		}

		logger.Info("job completed",
			"task_id", req.TaskID,
			"job_id", req.JobID,
			"status", resp.Status,
			"exit_code", resp.ExitCode,
		)

		handlers.WriteJSON(w, http.StatusOK, resp)
	})

	// Apply middleware
	handler := middleware.Recovery(logger)(middleware.Logging(logger)(mux))

	// Create server
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
		logger.Info("starting node worker API", "addr", cfg.ListenAddr, "node_slug", cfg.NodeSlug)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("node stopped")
}

// registerWithOrchestrator registers the node with the orchestrator.
func registerWithOrchestrator(ctx context.Context, cfg *config.NodeConfig, logger *slog.Logger) error {
	// Build capability report
	capability := handlers.NodeCapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node: handlers.NodeCapabilityNode{
			NodeSlug: cfg.NodeSlug,
			Name:     cfg.NodeName,
		},
		Platform: handlers.NodeCapabilityPlatform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		Compute: handlers.NodeCapabilityCompute{
			CPUCores: runtime.NumCPU(),
			RAMMB:    4096, // Default, would detect in production
		},
		Sandbox: &handlers.NodeCapabilitySandbox{
			Supported:      true,
			Features:       []string{"netns"},
			MaxConcurrency: 4,
		},
	}

	req := handlers.NodeRegistrationRequest{
		PSK:        cfg.RegistrationPSK,
		Capability: capability,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal registration request: %w", err)
	}

	url := cfg.OrchestratorURL + "/v1/nodes/register"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create registration request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed: %s - %s", resp.Status, string(body))
	}

	var bootstrap handlers.NodeBootstrapResponse
	if err := json.NewDecoder(resp.Body).Decode(&bootstrap); err != nil {
		return fmt.Errorf("decode bootstrap response: %w", err)
	}

	nodeCredentials.jwt = bootstrap.Auth.NodeJWT
	nodeCredentials.jwtExpiry, _ = time.Parse(time.RFC3339, bootstrap.Auth.ExpiresAt)

	logger.Info("registered with orchestrator",
		"node_slug", cfg.NodeSlug,
		"jwt_expires_at", nodeCredentials.jwtExpiry,
	)

	return nil
}
