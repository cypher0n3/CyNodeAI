// Package handlers: dev-only PMA / session reset (local setup-dev stop).
package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/cypher0n3/cynodeai/go_shared_libs/secretutil"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
)

// DevResetPMASessionState tears down every active session binding (PMA teardown_pending + config bump)
// and invalidates all refresh sessions. After this, desired managed PMA is warm-pool idle slots only
// until a new interactive session triggers greedy assignment (pma-pool-*).
func DevResetPMASessionState(ctx context.Context, db database.Store, logger *slog.Logger) error {
	bindings, err := db.ListAllActiveSessionBindings(ctx)
	if err != nil {
		return err
	}
	for _, b := range bindings {
		if b == nil {
			continue
		}
		if err := TeardownPMAForInteractiveSession(ctx, db, b.UserID, b.SessionID, "dev_reset_session_state", logger); err != nil {
			return err
		}
	}
	return db.InvalidateAllRefreshSessions(ctx)
}

// DevResetPMASessionStateHandler is mounted on the control-plane only. It requires
// Authorization: Bearer <NODE_REGISTRATION_PSK> (same secret nodes use at registration).
func DevResetPMASessionStateHandler(db database.Store, nodeRegistrationPSK string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.TrimSpace(nodeRegistrationPSK) == "" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		token := extractBearerTokenDevReset(r)
		if !secretutil.TokenEquals(token, nodeRegistrationPSK) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := DevResetPMASessionState(r.Context(), db, logger); err != nil {
			if logger != nil {
				logger.Error("dev reset PMA session state", "error", err)
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func extractBearerTokenDevReset(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(auth) < 8 || !strings.EqualFold(auth[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(auth[7:])
}
