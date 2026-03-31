package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
)

// UserHandler handles user endpoints.
type UserHandler struct {
	db     database.Store
	logger *slog.Logger
}

// NewUserHandler creates a new user handler.
func NewUserHandler(db database.Store, logger *slog.Logger) *UserHandler {
	return &UserHandler{
		db:     db,
		logger: logger,
	}
}

// GetMe handles GET /v1/users/me.
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)

	if userID == nil {
		WriteUnauthorized(w, "Not authenticated")
		return
	}

	if h.db == nil {
		WriteInternalError(w, "Database not available")
		return
	}

	user, err := h.db.GetUserByID(ctx, *userID)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("get user", "error", err)
		}
		WriteInternalError(w, "Failed to get user")
		return
	}

	WriteJSON(w, http.StatusOK, userapi.UserResponse{
		ID:       user.ID.String(),
		Handle:   user.Handle,
		Email:    user.Email,
		IsActive: user.IsActive,
	})
}

// RevokeSessions handles POST /v1/users/{id}/revoke_sessions (admin-gated).
// Invalidates all refresh sessions for the given user per local_user_accounts.md.
func (h *UserHandler) RevokeSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	if idStr == "" {
		WriteBadRequest(w, "user id required")
		return
	}
	userID, err := uuid.Parse(idStr)
	if err != nil {
		WriteBadRequest(w, "invalid user id")
		return
	}
	if h.db == nil {
		WriteInternalError(w, "Database not available")
		return
	}
	if _, err := h.db.GetUserByID(ctx, userID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			WriteNotFound(w, "User not found")
			return
		}
		if h.logger != nil {
			h.logger.Error("revoke_sessions get user", "error", err)
		}
		WriteInternalError(w, "Failed to get user")
		return
	}
	if err := h.db.InvalidateAllUserSessions(ctx, userID); err != nil {
		if h.logger != nil {
			h.logger.Error("revoke_sessions invalidate", "error", err)
		}
		WriteInternalError(w, "Failed to revoke sessions")
		return
	}
	if err := TeardownAllActivePMABindingsForUser(ctx, h.db, userID, "admin_revoke_sessions", h.logger); err != nil {
		if h.logger != nil {
			h.logger.Error("revoke_sessions pma teardown", "error", err)
		}
		WriteInternalError(w, "Failed to tear down PMA bindings")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
