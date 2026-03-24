package artifacts

import (
	"context"
	"log/slog"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
)

// StartBackgroundJobs runs optional hash backfill and stale cleanup loops until ctx is done.
// Both features are off unless explicitly enabled in cfg (see orchestrator_artifacts_storage.md).
func StartBackgroundJobs(ctx context.Context, svc *Service, cfg *config.OrchestratorConfig, logger *slog.Logger) {
	if svc == nil || cfg == nil {
		return
	}
	if cfg.ArtifactHashBackfillEnabled {
		interval := cfg.ArtifactHashBackfillInterval
		if interval <= 0 {
			interval = 10 * time.Minute
		}
		go runHashBackfillLoop(ctx, svc, interval, logger)
	}
	if cfg.ArtifactStaleCleanupEnabled && cfg.ArtifactStaleCleanupMaxAgeHours > 0 {
		interval := cfg.ArtifactStaleCleanupInterval
		if interval <= 0 {
			interval = time.Hour
		}
		maxAge := time.Duration(cfg.ArtifactStaleCleanupMaxAgeHours) * time.Hour
		go runStaleCleanupLoop(ctx, svc, interval, maxAge, logger)
	}
}

func runHashBackfillLoop(ctx context.Context, svc *Service, interval time.Duration, logger *slog.Logger) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			workCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			n, err := svc.BackfillMissingHashesOnce(workCtx, 50)
			cancel()
			if logger != nil && (n > 0 || err != nil) {
				logger.Info("artifact hash backfill", "updated", n, "error", err)
			}
		}
	}
}

func runStaleCleanupLoop(ctx context.Context, svc *Service, interval, maxAge time.Duration, logger *slog.Logger) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			workCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			n, err := svc.PruneStaleByMaxAgeOnce(workCtx, maxAge, 50)
			cancel()
			if logger != nil && (n > 0 || err != nil) {
				logger.Info("artifact stale cleanup", "removed", n, "error", err)
			}
		}
	}
}
