package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/natsconfig"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
)

// authGatewayPublisher publishes session lifecycle for HTTP-only clients.
// *GatewaySessionPublisher implements it; tests may substitute a spy.
type authGatewayPublisher interface {
	PublishAttached(ctx context.Context, tenantID, sessionID, userID, bindingKey string) error
	PublishDetached(ctx context.Context, tenantID, sessionID, userID, bindingKey, reason string) error
}

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	db          database.Store
	jwt         *auth.JWTManager
	rateLimiter *auth.RateLimiter
	logger      *slog.Logger
	natsIssuer  *natsjwt.Issuer
	natsURL     string
	natsWS      string
	natsMu      sync.Mutex
	natsJTI     map[uuid.UUID]string // refresh session id -> NATS user JWT jti
	// gatewayPub publishes session lifecycle for HTTP-only logins (no nats block).
	gatewayPub authGatewayPublisher
	// sessionNatsFromClient records whether login/refresh returned a nats block (client uses NATS).
	sessionNatsFromClient map[uuid.UUID]bool
	sessionNatsMu         sync.Mutex
}

// NewAuthHandler creates a new auth handler.
// When natsIssuer is non-nil, login/refresh responses include a nats config block and JWT revocation is tracked.
func NewAuthHandler(db database.Store, jwt *auth.JWTManager, rateLimiter *auth.RateLimiter, natsIssuer *natsjwt.Issuer, natsURL, natsWebSocketURL string, logger *slog.Logger) *AuthHandler {
	h := &AuthHandler{
		db:                    db,
		jwt:                   jwt,
		rateLimiter:           rateLimiter,
		logger:                logger,
		natsIssuer:            natsIssuer,
		natsURL:               strings.TrimSpace(natsURL),
		natsWS:                strings.TrimSpace(natsWebSocketURL),
		sessionNatsFromClient: make(map[uuid.UUID]bool),
	}
	if natsIssuer != nil {
		h.natsJTI = make(map[uuid.UUID]string)
	}
	return h
}

// authLog returns the configured logger or a discard sink so login/refresh never nil-deref slog.
var authDiscardLogger = slog.New(slog.DiscardHandler)

func (h *AuthHandler) authLog() *slog.Logger {
	if h.logger != nil {
		return h.logger
	}
	return authDiscardLogger
}

// SetGatewaySessionPublisher sets the optional JetStream publisher for session.attached from login
// and session.activity from gateway API traffic (see OpenAIChatHandler.SetGatewaySessionActivity).
func (h *AuthHandler) SetGatewaySessionPublisher(p *GatewaySessionPublisher) {
	if p == nil {
		h.gatewayPub = nil
		return
	}
	h.gatewayPub = p
}

func (h *AuthHandler) recordSessionNatsMode(refreshID uuid.UUID, clientHasNats bool) {
	h.sessionNatsMu.Lock()
	h.sessionNatsFromClient[refreshID] = clientHasNats
	h.sessionNatsMu.Unlock()
}

func (h *AuthHandler) registerNatsSession(sessionID uuid.UUID, jti string) {
	if h.natsIssuer == nil || h.natsJTI == nil {
		return
	}
	h.natsMu.Lock()
	h.natsJTI[sessionID] = jti
	h.natsMu.Unlock()
}

func (h *AuthHandler) revokeNatsSession(sessionID uuid.UUID) {
	if h.natsIssuer == nil || h.natsJTI == nil {
		return
	}
	h.natsMu.Lock()
	jti, ok := h.natsJTI[sessionID]
	delete(h.natsJTI, sessionID)
	h.natsMu.Unlock()
	if ok && jti != "" {
		h.natsIssuer.RevokeJTI(jti)
	}
}

func applyInteractiveSessionFields(resp *userapi.LoginResponse, userID, refreshSessionID uuid.UUID) {
	resp.InteractiveSessionID = refreshSessionID.String()
	resp.SessionBindingKey = models.DeriveSessionBindingKey(models.SessionBindingLineage{
		UserID: userID, SessionID: refreshSessionID, ThreadID: nil,
	})
}

func (h *AuthHandler) applyNatsToLoginResponse(resp *userapi.LoginResponse, refreshSessionID uuid.UUID, expiresAt time.Time) {
	if h.natsIssuer == nil {
		return
	}
	tok, err := h.natsIssuer.SessionJWT(natsjwt.DefaultTenantID, refreshSessionID, expiresAt)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("nats session jwt", "error", err)
		}
		return
	}
	jti, jerr := natsjwt.ExtractJTI(tok)
	if jerr == nil {
		h.registerNatsSession(refreshSessionID, jti)
	}
	resp.Nats = &natsconfig.ClientCredentials{
		URL:          h.natsURL,
		WebSocketURL: h.natsWS,
		JWT:          tok,
		JWTExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	}
}

// loginResult contains the result of login validation.
type loginResult struct {
	user        *models.User
	errResponse func()
}

// validateLoginCredentials validates user credentials and returns user or error response.
func (h *AuthHandler) validateLoginCredentials(ctx context.Context, req userapi.LoginRequest, ipAddr, userAgent string) *loginResult {
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

	var req userapi.LoginRequest
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
		h.authLog().Error("generate access token", "error", err)
		WriteInternalError(w, "Failed to generate token")
		return
	}

	refreshToken, expiresAt, err := h.jwt.GenerateRefreshToken(user.ID)
	if err != nil {
		h.authLog().Error("generate refresh token", "error", err)
		WriteInternalError(w, "Failed to generate token")
		return
	}

	tokenHash := auth.HashToken(refreshToken)
	refreshRec, err := h.db.CreateRefreshSession(ctx, user.ID, tokenHash, expiresAt)
	if err != nil {
		h.authLog().Error("create refresh session", "error", err)
		WriteInternalError(w, "Failed to create session")
		return
	}
	if err := GreedyProvisionPMAAfterInteractiveSession(ctx, h.db, user.ID, refreshRec.ID, h.logger); err != nil {
		h.authLog().Warn("greedy pma provision after login", "error", err)
	}

	h.auditLog(ctx, &user.ID, "login_success", true, ipAddr, userAgent, "")

	resp := userapi.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    900, // 15 minutes
	}
	applyInteractiveSessionFields(&resp, user.ID, refreshRec.ID)
	h.applyNatsToLoginResponse(&resp, refreshRec.ID, expiresAt)
	h.recordSessionNatsMode(refreshRec.ID, resp.Nats != nil)
	if h.gatewayPub != nil && resp.Nats == nil && resp.InteractiveSessionID != "" {
		_ = h.gatewayPub.PublishAttached(ctx, natsjwt.DefaultTenantID, resp.InteractiveSessionID, user.ID.String(), resp.SessionBindingKey)
	}
	WriteJSON(w, http.StatusOK, resp)
}

// Refresh handles POST /v1/auth/refresh.
//
//nolint:gocyclo // refresh is sequential token rotation and response assembly
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ipAddr := getClientIP(r)

	var req userapi.RefreshRequest
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
		h.authLog().Error("get refresh session", "error", err)
		WriteInternalError(w, "Failed to refresh token")
		return
	}

	h.revokeNatsSession(session.ID)

	// Invalidate old session (token rotation)
	if err := h.db.InvalidateRefreshSession(ctx, session.ID); err != nil {
		h.authLog().Error("invalidate refresh session", "error", err)
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
		h.authLog().Error("generate access token", "error", err)
		WriteInternalError(w, "Failed to generate token")
		return
	}

	newRefreshToken, expiresAt, err := h.jwt.GenerateRefreshToken(user.ID)
	if err != nil {
		h.authLog().Error("generate refresh token", "error", err)
		WriteInternalError(w, "Failed to generate token")
		return
	}

	// Store new refresh session
	newTokenHash := auth.HashToken(newRefreshToken)
	newRefresh, err := h.db.CreateRefreshSession(ctx, user.ID, newTokenHash, expiresAt)
	if err != nil {
		h.authLog().Error("create refresh session", "error", err)
		WriteInternalError(w, "Failed to create session")
		return
	}
	if err := GreedyProvisionPMAAfterInteractiveSession(ctx, h.db, user.ID, newRefresh.ID, h.logger); err != nil {
		h.authLog().Warn("greedy pma provision after refresh", "error", err)
	}

	h.auditLog(ctx, &userID, "refresh_success", true, ipAddr, r.UserAgent(), "")

	resp := userapi.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}
	applyInteractiveSessionFields(&resp, user.ID, newRefresh.ID)
	h.applyNatsToLoginResponse(&resp, newRefresh.ID, expiresAt)
	h.recordSessionNatsMode(newRefresh.ID, resp.Nats != nil)
	if h.gatewayPub != nil && resp.Nats == nil && resp.InteractiveSessionID != "" {
		_ = h.gatewayPub.PublishAttached(ctx, natsjwt.DefaultTenantID, resp.InteractiveSessionID, user.ID.String(), resp.SessionBindingKey)
	}
	WriteJSON(w, http.StatusOK, resp)
}

// Logout handles POST /v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ipAddr := getClientIP(r)
	userID := getUserIDFromContext(ctx)

	var req userapi.LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "Invalid request body")
		return
	}

	if req.RefreshToken != "" {
		// Invalidate specific session
		tokenHash := auth.HashToken(req.RefreshToken)
		session, err := h.db.GetActiveRefreshSession(ctx, tokenHash)
		if err == nil {
			h.sessionNatsMu.Lock()
			hadClientNats := h.sessionNatsFromClient[session.ID]
			delete(h.sessionNatsFromClient, session.ID)
			h.sessionNatsMu.Unlock()
			h.revokeNatsSession(session.ID)
			_ = h.db.InvalidateRefreshSession(ctx, session.ID)
			_ = TeardownPMAForInteractiveSession(ctx, h.db, session.UserID, session.ID, "logout", h.logger)
			if h.gatewayPub != nil && !hadClientNats {
				bk := models.DeriveSessionBindingKey(models.SessionBindingLineage{
					UserID: session.UserID, SessionID: session.ID, ThreadID: nil,
				})
				_ = h.gatewayPub.PublishDetached(ctx, natsjwt.DefaultTenantID, session.ID.String(), session.UserID.String(), bk, "logout")
			}
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

	if err := h.db.CreateAuthAuditLog(ctx, userID, eventType, success, ipPtr, uaPtr, nil, detPtr); err != nil {
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
