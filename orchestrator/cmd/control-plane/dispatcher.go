package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/dispatcher"
)

type dispatcherConfig struct {
	Enabled      bool
	PollInterval time.Duration
	HTTPTimeout  time.Duration
}

func loadDispatcherConfig() dispatcherConfig {
	return dispatcherConfig{
		Enabled:      getEnv("DISPATCHER_ENABLED", "true") == "true",
		PollInterval: getDurationEnv("DISPATCH_POLL_INTERVAL", 1*time.Second),
		HTTPTimeout:  getDurationEnv("DISPATCH_HTTP_TIMEOUT", 5*time.Minute),
	}
}

func startDispatcher(ctx context.Context, db database.Store, logger *slog.Logger) {
	cfg := loadDispatcherConfig()
	if !cfg.Enabled {
		logger.Info("dispatcher disabled")
		return
	}

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	logger.Info("dispatcher started", "poll_interval", cfg.PollInterval.String())

	for {
		select {
		case <-ctx.Done():
			logger.Info("dispatcher stopping", "reason", ctx.Err())
			return
		case <-ticker.C:
			if err := dispatcher.RunOnce(ctx, db, client, cfg.HTTPTimeout, logger); err != nil {
				if errors.Is(err, database.ErrNotFound) {
					continue
				}
				logger.Error("dispatch iteration failed", "error", err)
			}
		}
	}
}

func getDurationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
