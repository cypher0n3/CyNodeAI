// Package middleware provides HTTP middleware for CyNodeAI.
package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/internal/auth"
	"github.com/cypher0n3/cynodeai/internal/handlers"
)

// AuthMiddleware provides JWT authentication middleware.
type AuthMiddleware struct {
	jwt    *auth.JWTManager
	logger *slog.Logger
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(jwt *auth.JWTManager, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwt:    jwt,
		logger: logger,
	}
}

// RequireUserAuth middleware requires a valid user access token.
func (m *AuthMiddleware) RequireUserAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractBearerToken(r)
		if tokenStr == "" {
			handlers.WriteUnauthorized(w, "Missing authorization header")
			return
		}

		claims, err := m.jwt.ValidateAccessToken(tokenStr)
		if err != nil {
			handlers.WriteUnauthorized(w, "Invalid or expired token")
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			handlers.WriteUnauthorized(w, "Invalid token claims")
			return
		}

		ctx := handlers.SetUserContext(r.Context(), userID, claims.Handle)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireNodeAuth middleware requires a valid node token.
func (m *AuthMiddleware) RequireNodeAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractBearerToken(r)
		if tokenStr == "" {
			handlers.WriteUnauthorized(w, "Missing authorization header")
			return
		}

		claims, err := m.jwt.ValidateNodeToken(tokenStr)
		if err != nil {
			handlers.WriteUnauthorized(w, "Invalid or expired token")
			return
		}

		nodeID, err := uuid.Parse(claims.NodeID)
		if err != nil {
			handlers.WriteUnauthorized(w, "Invalid token claims")
			return
		}

		ctx := handlers.SetNodeContext(r.Context(), nodeID, claims.NodeSlug)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return parts[1]
}
