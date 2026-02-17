// Package middleware provides HTTP middleware for CyNodeAI.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/handlers"
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

// setContextFunc derives a new context from the request context; used so contextcheck sees inheritance.
type setContextFunc func(context.Context) context.Context

// requireAuth is the shared implementation for token-based auth; getContext validates the token and returns a setter to apply to r.Context().
func (m *AuthMiddleware) requireAuth(next http.Handler, getContext func(*http.Request, string) (setContextFunc, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractBearerToken(r)
		if tokenStr == "" {
			handlers.WriteUnauthorized(w, "Missing authorization header")
			return
		}
		setCtx, err := getContext(r, tokenStr)
		if err != nil {
			handlers.WriteUnauthorized(w, "Invalid or expired token")
			return
		}
		next.ServeHTTP(w, r.WithContext(setCtx(r.Context())))
	})
}

// tokenToContext builds a getContext function: validate token string to (id, name), then set context via setCtx.
func (m *AuthMiddleware) tokenToContext(
	validate func(string) (uuid.UUID, string, error),
	setCtx func(context.Context, uuid.UUID, string) context.Context,
) func(*http.Request, string) (setContextFunc, error) {
	return func(_ *http.Request, tokenStr string) (setContextFunc, error) {
		id, name, err := validate(tokenStr)
		if err != nil {
			return nil, err
		}
		return func(c context.Context) context.Context { return setCtx(c, id, name) }, nil
	}
}

// parseTokenIDName runs getIDName(tokenStr) and parses the id string to uuid; shared by user and node auth.
func parseTokenIDName(tokenStr string, getIDName func(string) (idStr, name string, err error)) (uuid.UUID, string, error) {
	idStr, name, err := getIDName(tokenStr)
	if err != nil {
		return uuid.Nil, "", err
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, "", err
	}
	return id, name, nil
}

func (m *AuthMiddleware) userIDName(tokenStr string) (idStr, name string, err error) {
	claims, err := m.jwt.ValidateAccessToken(tokenStr)
	if err != nil {
		return "", "", err
	}
	return claims.UserID, claims.Handle, nil
}

func (m *AuthMiddleware) nodeIDName(tokenStr string) (idStr, name string, err error) {
	claims, err := m.jwt.ValidateNodeToken(tokenStr)
	if err != nil {
		return "", "", err
	}
	return claims.NodeID, claims.NodeSlug, nil
}

// RequireUserAuth middleware requires a valid user access token.
func (m *AuthMiddleware) RequireUserAuth(next http.Handler) http.Handler {
	validate := func(t string) (uuid.UUID, string, error) { return parseTokenIDName(t, m.userIDName) }
	getCtx := m.tokenToContext(validate, handlers.SetUserContext)
	return m.requireAuth(next, getCtx)
}

// RequireNodeAuth middleware requires a valid node token.
func (m *AuthMiddleware) RequireNodeAuth(next http.Handler) http.Handler {
	return m.requireAuth(next, m.tokenToContext(
		func(t string) (uuid.UUID, string, error) { return parseTokenIDName(t, m.nodeIDName) },
		handlers.SetNodeContext))
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
