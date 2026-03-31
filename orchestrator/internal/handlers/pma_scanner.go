package handlers

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// RunPMABindingScanner periodically tears down PMA bindings whose refresh session expired, is inactive, or idle beyond policy.
func RunPMABindingScanner(ctx context.Context, db database.Store, logger *slog.Logger) {
	idle := pmaIdleTimeout()
	if err := scanPMABindingsOnce(ctx, db, idle, logger); err != nil && logger != nil {
		logger.Warn("pma binding scanner initial scan", "error", err)
	}
	interval := pmaScannerInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := scanPMABindingsOnce(ctx, db, idle, logger); err != nil && logger != nil {
				logger.Warn("pma binding scanner tick", "error", err)
			}
		}
	}
}

func scanPMABindingsOnce(ctx context.Context, db database.Store, idle time.Duration, logger *slog.Logger) error {
	now := time.Now().UTC()
	bindings, err := db.ListAllActiveSessionBindings(ctx)
	if err != nil {
		return err
	}
	for _, b := range bindings {
		if b == nil {
			continue
		}
		if err := scanOnePMABinding(ctx, db, b, idle, now, logger); err != nil {
			return err
		}
	}
	return nil
}

func scanOnePMABinding(ctx context.Context, db database.Store, b *models.SessionBinding, idle time.Duration, now time.Time, logger *slog.Logger) error {
	rs, err := db.GetRefreshSessionByID(ctx, b.SessionID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return TeardownPMAForInteractiveSession(ctx, db, b.UserID, b.SessionID, "refresh_session_missing", logger)
		}
		return err
	}
	if !rs.IsActive || rs.ExpiresAt.Before(now) {
		return TeardownPMAForInteractiveSession(ctx, db, b.UserID, b.SessionID, "refresh_session_expired_or_inactive", logger)
	}
	act := b.UpdatedAt
	if b.LastActivityAt != nil {
		act = *b.LastActivityAt
	}
	if idle > 0 && now.Sub(act) > idle {
		return TeardownPMAForInteractiveSession(ctx, db, b.UserID, b.SessionID, "idle_timeout", logger)
	}
	return nil
}

func pmaScannerInterval() time.Duration {
	return parseEnvDuration("PMA_BINDING_SCAN_INTERVAL_SEC", 60*time.Second, time.Second)
}

func pmaIdleTimeout() time.Duration {
	return parseEnvDuration("PMA_BINDING_IDLE_TIMEOUT_MIN", 30*time.Minute, time.Minute)
}

func parseEnvDuration(key string, def, unit time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return time.Duration(n) * unit
}
