package handlers

import (
        "context"
        "encoding/json"
        "errors"
        "log/slog"
        "net/http"
        "strings"

        "github.com/google/uuid"

        "github.com/cypher0n3/cynodeai/internal/auth"
        "github.com/cypher0n3/cynodeai/internal/database"
        "github.com/cypher0n3/cynodeai/internal/models"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
        db          database.Store
        jwt         *auth.JWTManager
        rateLimiter *auth.RateLimiter
        logger      *slog.Logger
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(db database.Store, jwt *auth.JWTManager, rateLimiter *auth.RateLimiter, logger *slog.Logger) *AuthHandler {
        return &AuthHandler{
                db:          db,
                jwt:         jwt,
                rateLimiter: rateLimiter,
                logger:      logger,
        }
}

// LoginRequest represents login request body.
type LoginRequest struct {
        Handle   string `json:"handle"`
        Password string `json:"password"`
}

// LoginResponse represents login response body.
type LoginResponse struct {
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
        TokenType    string `json:"token_type"`
        ExpiresIn    int    `json:"expires_in"`
}

// loginResult contains the result of login validation.
type loginResult struct {
        user        *models.User
        errResponse func()
}

// validateLoginCredentials validates user credentials and returns user or error response.
func (h *AuthHandler) validateLoginCredentials(ctx context.Context, req LoginRequest, ipAddr, userAgent string) *loginResult {
        // Get user by handle
        user, err := h.db.GetUserByHandle(ctx, req.Handle)
        if errors.Is(err, database.ErrNotFound) {
                h.auditLog(ctx, nil, "login_failure", false, ipAddr, userAgent, "user not found")
                return &loginResult{errResponse: func() {}}
        }
        if err != nil {
                h.logger.Error("get user", "error", err)
                return nil
        }

        if !user.IsActive {
                h.auditLog(ctx, &user.ID, "login_failure", false, ipAddr, userAgent, "user inactive")
                return &loginResult{errResponse: func() {}}
        }

        // Get password credential
        cred, err := h.db.GetPasswordCredentialByUserID(ctx, user.ID)
        if errors.Is(err, database.ErrNotFound) {
                h.auditLog(ctx, &user.ID, "login_failure", false, ipAddr, userAgent, "no password credential")
                return &loginResult{errResponse: func() {}}
        }
        if err != nil {
                h.logger.Error("get password credential", "error", err)
                return nil
        }

        // Verify password
        valid, err := auth.VerifyPassword(req.Password, cred.PasswordHash)
        if err != nil || !valid {
                h.auditLog(ctx, &user.ID, "login_failure", false, ipAddr, userAgent, "invalid password")
                return &loginResult{errResponse: func() {}}
        }

        return &loginResult{user: user}
}

// Login handles POST /v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        ipAddr := getClientIP(r)

        // Rate limit by IP
        if !h.rateLimiter.Allow(ipAddr) {
                h.auditLog(ctx, nil, "login_failure", false, ipAddr, r.UserAgent(), "rate limited")
                WriteTooManyRequests(w, "Too many login attempts")
                return
        }

        var req LoginRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                WriteBadRequest(w, "Invalid request body")
                return
        }

        if req.Handle == "" || req.Password == "" {
                WriteBadRequest(w, "Handle and password are required")
                return
        }

        if h.db == nil {
                WriteInternalError(w, "Database not available")
                return
        }

        result := h.validateLoginCredentials(ctx, req, ipAddr, r.UserAgent())
        if result == nil {
                WriteInternalError(w, "Failed to authenticate")
                return
        }
        if result.user == nil {
                WriteUnauthorized(w, "Invalid credentials")
                return
        }

        h.completeLogin(w, ctx, result.user, ipAddr, r.UserAgent())
}

// completeLogin generates tokens and creates session.
func (h *AuthHandler) completeLogin(w http.ResponseWriter, ctx context.Context, user *models.User, ipAddr, userAgent string) {
        accessToken, err := h.jwt.GenerateAccessToken(user.ID, user.Handle)
        if err != nil {
                h.logger.Error("generate access token", "error", err)
                WriteInternalError(w, "Failed to generate token")
                return
        }

        refreshToken, expiresAt, err := h.jwt.GenerateRefreshToken(user.ID)
        if err != nil {
                h.logger.Error("generate refresh token", "error", err)
                WriteInternalError(w, "Failed to generate token")
                return
        }

        tokenHash := auth.HashToken(refreshToken)
        _, err = h.db.CreateRefreshSession(ctx, user.ID, tokenHash, expiresAt)
        if err != nil {
                h.logger.Error("create refresh session", "error", err)
                WriteInternalError(w, "Failed to create session")
                return
        }

        h.auditLog(ctx, &user.ID, "login_success", true, ipAddr, userAgent, "")

        WriteJSON(w, http.StatusOK, LoginResponse{
                AccessToken:  accessToken,
                RefreshToken: refreshToken,
                TokenType:    "Bearer",
                ExpiresIn:    900, // 15 minutes
        })
}

// RefreshRequest represents refresh token request body.
type RefreshRequest struct {
        RefreshToken string `json:"refresh_token"`
}

// Refresh handles POST /v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        ipAddr := getClientIP(r)

        var req RefreshRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                WriteBadRequest(w, "Invalid request body")
                return
        }

        if req.RefreshToken == "" {
                WriteBadRequest(w, "Refresh token is required")
                return
        }

        // Validate refresh token JWT
        claims, err := h.jwt.ValidateRefreshToken(req.RefreshToken)
        if err != nil {
                h.auditLog(ctx, nil, "refresh_failure", false, ipAddr, r.UserAgent(), "invalid token")
                WriteUnauthorized(w, "Invalid refresh token")
                return
        }

        userID, err := uuid.Parse(claims.UserID)
        if err != nil {
                WriteUnauthorized(w, "Invalid refresh token")
                return
        }

        // Verify session exists in database
        tokenHash := auth.HashToken(req.RefreshToken)
        session, err := h.db.GetActiveRefreshSession(ctx, tokenHash)
        if errors.Is(err, database.ErrNotFound) {
                h.auditLog(ctx, &userID, "refresh_failure", false, ipAddr, r.UserAgent(), "session not found")
                WriteUnauthorized(w, "Invalid refresh token")
                return
        }
        if err != nil {
                h.logger.Error("get refresh session", "error", err)
                WriteInternalError(w, "Failed to refresh token")
                return
        }

        // Invalidate old session (token rotation)
        if err := h.db.InvalidateRefreshSession(ctx, session.ID); err != nil {
                h.logger.Error("invalidate refresh session", "error", err)
                WriteInternalError(w, "Failed to refresh token")
                return
        }

        // Get user
        user, err := h.db.GetUserByID(ctx, userID)
        if err != nil {
                WriteUnauthorized(w, "User not found")
                return
        }

        if !user.IsActive {
                h.auditLog(ctx, &userID, "refresh_failure", false, ipAddr, r.UserAgent(), "user inactive")
                WriteUnauthorized(w, "Account is disabled")
                return
        }

        // Generate new tokens
        accessToken, err := h.jwt.GenerateAccessToken(user.ID, user.Handle)
        if err != nil {
                h.logger.Error("generate access token", "error", err)
                WriteInternalError(w, "Failed to generate token")
                return
        }

        newRefreshToken, expiresAt, err := h.jwt.GenerateRefreshToken(user.ID)
        if err != nil {
                h.logger.Error("generate refresh token", "error", err)
                WriteInternalError(w, "Failed to generate token")
                return
        }

        // Store new refresh session
        newTokenHash := auth.HashToken(newRefreshToken)
        _, err = h.db.CreateRefreshSession(ctx, user.ID, newTokenHash, expiresAt)
        if err != nil {
                h.logger.Error("create refresh session", "error", err)
                WriteInternalError(w, "Failed to create session")
                return
        }

        h.auditLog(ctx, &userID, "refresh_success", true, ipAddr, r.UserAgent(), "")

        WriteJSON(w, http.StatusOK, LoginResponse{
                AccessToken:  accessToken,
                RefreshToken: newRefreshToken,
                TokenType:    "Bearer",
                ExpiresIn:    900,
        })
}

// LogoutRequest represents logout request body.
type LogoutRequest struct {
        RefreshToken string `json:"refresh_token"`
}

// Logout handles POST /v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        ipAddr := getClientIP(r)
        userID := getUserIDFromContext(ctx)

        var req LogoutRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                WriteBadRequest(w, "Invalid request body")
                return
        }

        if req.RefreshToken != "" {
                // Invalidate specific session
                tokenHash := auth.HashToken(req.RefreshToken)
                session, err := h.db.GetActiveRefreshSession(ctx, tokenHash)
                if err == nil {
                        _ = h.db.InvalidateRefreshSession(ctx, session.ID)
                }
        }

        h.auditLog(ctx, userID, "logout", true, ipAddr, r.UserAgent(), "")

        w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) auditLog(ctx context.Context, userID *uuid.UUID, eventType string, success bool, ipAddr, userAgent, details string) {
        if h.db == nil {
                return
        }

        var ipPtr, uaPtr, detPtr *string
        if ipAddr != "" {
                ipPtr = &ipAddr
        }
        if userAgent != "" {
                uaPtr = &userAgent
        }
        if details != "" {
                detPtr = &details
        }

        if err := h.db.CreateAuthAuditLog(ctx, userID, eventType, success, ipPtr, uaPtr, detPtr); err != nil {
                if h.logger != nil {
                        h.logger.Error("create auth audit log", "error", err)
                }
        }
}

func getClientIP(r *http.Request) string {
        if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
                parts := strings.Split(xff, ",")
                return strings.TrimSpace(parts[0])
        }
        if xri := r.Header.Get("X-Real-IP"); xri != "" {
                return xri
        }
        if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
                return r.RemoteAddr[:idx]
        }
        return r.RemoteAddr
}
