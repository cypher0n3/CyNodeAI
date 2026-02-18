// Package main provides the node manager service.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/cypher0n3/cynodeai/worker_node/internal/nodemanager"
)

func main() {
	os.Exit(runMain(context.Background()))
}

// runMain loads config and runs the node manager until ctx is cancelled.
// Returns 0 on success, 1 on failure. Extracted for testability.
func runMain(ctx context.Context) int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	cfg := nodemanager.LoadConfig()
	if err := nodemanager.Run(ctx, logger, &cfg); err != nil {
		logger.Error("node manager failed", "error", err)
		return 1
	}
	return 0
}
