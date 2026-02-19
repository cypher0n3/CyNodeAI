// Package main provides the node manager service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/cypher0n3/cynodeai/worker_node/internal/nodemanager"
)

func main() {
	os.Exit(runMain(context.Background()))
}

// getEnv returns the environment variable key if set, otherwise def. Used for optional main-level config.
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// runMain loads config and runs the node manager until ctx is cancelled.
// Returns 0 on success, 1 on failure. Extracted for testability.
func runMain(ctx context.Context) int {
	level := slog.LevelInfo
	if getEnv("NODE_MANAGER_DEBUG", "") != "" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	cfg := nodemanager.LoadConfig()
	opts := &nodemanager.RunOptions{
		StartWorkerAPI: startWorkerAPI,
		StartOllama:    startOllama,
	}
	if getEnv("NODE_MANAGER_SKIP_SERVICES", "") != "" {
		opts = nil
	}
	if err := nodemanager.RunWithOptions(ctx, logger, &cfg, opts); err != nil {
		logger.Error("node manager failed", "error", err)
		return 1
	}
	return 0
}

// startWorkerAPI starts the worker-api process with the given bearer token in env.
// The token must not be logged. Returns when the process has been started (or an error).
func startWorkerAPI(bearerToken string) error {
	bin := getEnv("NODE_MANAGER_WORKER_API_BIN", "worker-api")
	if !strings.Contains(bin, "/") {
		path, err := exec.LookPath(bin)
		if err != nil {
			return err
		}
		bin = path
	}
	cmd := exec.CommandContext(context.Background(), bin)
	cmd.Env = append(os.Environ(), "WORKER_API_BEARER_TOKEN="+bearerToken)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// startOllama starts the Phase 1 inference container (Ollama). Fail-fast on error.
func startOllama() error {
	runtime := getEnv("CONTAINER_RUNTIME", "podman")
	image := getEnv("OLLAMA_IMAGE", "ollama/ollama")
	name := "cynodeai-ollama"
	cmd := exec.Command(runtime, "run", "-d", "--name", name, image)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
