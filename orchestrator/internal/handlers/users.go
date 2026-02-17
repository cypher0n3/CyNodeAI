package handlers

import (
	"log/slog"
	"net/http"

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

// UserResponse represents user data in responses.
type UserResponse struct {
	ID       string  `json:"id"`
	Handle   string  `json:"handle"`
	Email    *string `json:"email,omitempty"`
	IsActive bool    `json:"is_active"`
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

	WriteJSON(w, http.StatusOK, UserResponse{
		ID:       user.ID.String(),
		Handle:   user.Handle,
		Email:    user.Email,
		IsActive: user.IsActive,
	})
}
