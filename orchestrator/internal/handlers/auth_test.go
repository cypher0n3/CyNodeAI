package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
)

func TestNewAuthHandler(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, nil, "", "", nil)
	if handler == nil {
		t.Fatal("NewAuthHandler returned nil")
	}
}

func TestLoginBadRequest(t *testing.T) {
	handler := &AuthHandler{rateLimiter: auth.NewRateLimiter(10, time.Minute)}
	runHandlerTest(t, "POST", "/v1/auth/login", []byte("{invalid"), handler.Login, http.StatusBadRequest)
}

func TestLoginEmptyCredentials(t *testing.T) {
	handler := &AuthHandler{rateLimiter: auth.NewRateLimiter(10, time.Minute)}
	req, rec := recordedRequestJSON("POST", "/v1/auth/login", userapi.LoginRequest{Handle: "", Password: ""})
	handler.Login(rec, req)
	assertStatusCode(t, rec, http.StatusBadRequest)
}

func TestRefreshBadRequest(t *testing.T) {
	handler := &AuthHandler{}
	runHandlerTest(t, "POST", "/v1/auth/refresh", []byte("{invalid"), handler.Refresh, http.StatusBadRequest)
}

func TestRefreshEmptyToken(t *testing.T) {
	handler := &AuthHandler{}
	req, rec := recordedRequestJSON("POST", "/v1/auth/refresh", userapi.RefreshRequest{RefreshToken: ""})
	handler.Refresh(rec, req)
	assertStatusCode(t, rec, http.StatusBadRequest)
}

func TestLogoutBadRequest(t *testing.T) {
	handler := &AuthHandler{}
	runHandlerTest(t, "POST", "/v1/auth/logout", []byte("{invalid"), handler.Logout, http.StatusBadRequest)
}

func TestLogoutSuccess(t *testing.T) {
	handler := &AuthHandler{}
	req, rec := recordedRequestJSON("POST", "/v1/auth/logout", userapi.LogoutRequest{RefreshToken: ""})
	handler.Logout(rec, req)
	assertStatusCode(t, rec, http.StatusNoContent)
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remote   string
		expected string
	}{
		{
			name:     "X-Forwarded-For",
			headers:  map[string]string{"X-Forwarded-For": "1.2.3.4, 5.6.7.8"},
			expected: "1.2.3.4",
		},
		{
			name:     "X-Real-IP",
			headers:  map[string]string{"X-Real-IP": "10.0.0.1"},
			expected: "10.0.0.1",
		},
		{
			name:     "RemoteAddr with port",
			headers:  map[string]string{},
			remote:   "192.168.1.1:12345",
			expected: "192.168.1.1",
		},
		{
			name:     "RemoteAddr without port",
			headers:  map[string]string{},
			remote:   "192.168.1.1",
			expected: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", http.NoBody)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if tt.remote != "" {
				req.RemoteAddr = tt.remote
			}

			result := getClientIP(req)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestLoginResponseJSON(t *testing.T) {
	resp := userapi.LoginResponse{
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed userapi.LoginResponse
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.AccessToken != "access" {
		t.Errorf("expected access token 'access', got %s", parsed.AccessToken)
	}
}

func TestLoginRateLimited(t *testing.T) {
	rateLimiter := auth.NewRateLimiter(1, time.Minute) // Only allow 1 request
	handler := &AuthHandler{rateLimiter: rateLimiter}

	// First request should be allowed but fail due to nil db
	body := userapi.LoginRequest{Handle: "test", Password: "test"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody))
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	handler.Login(rec, req)

	// First request should fail with 500 (db nil), but rate limit was consumed
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("first request expected status 500 (db nil), got %d", rec.Code)
	}

	// Second request from same IP should be rate limited
	jsonBody2, _ := json.Marshal(body)
	req2 := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody2))
	req2.RemoteAddr = "192.168.1.1:5678"
	rec2 := httptest.NewRecorder()
	handler.Login(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request expected status 429, got %d", rec2.Code)
	}
}

func TestAuditLogWithLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := &AuthHandler{logger: logger}

	// Should not panic even with nil db
	handler.auditLog(context.Background(), nil, "test", true, "", "", "")
}

func TestLoginRequestJSON(t *testing.T) {
	req := userapi.LoginRequest{
		Handle:   testUserHandle,
		Password: "testpass",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed userapi.LoginRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Handle != testUserHandle {
		t.Errorf("expected handle %q, got %s", testUserHandle, parsed.Handle)
	}
}

func TestRefreshRequestJSON(t *testing.T) {
	var parsed userapi.RefreshRequest
	roundTripJSON(t, userapi.RefreshRequest{RefreshToken: "refresh-token-123"}, &parsed)
	if parsed.RefreshToken != "refresh-token-123" {
		t.Errorf("expected refresh token 'refresh-token-123', got %s", parsed.RefreshToken)
	}
}

func TestLogoutRequestJSON(t *testing.T) {
	var parsed userapi.LogoutRequest
	roundTripJSON(t, userapi.LogoutRequest{RefreshToken: "logout-token"}, &parsed)
	if parsed.RefreshToken != "logout-token" {
		t.Errorf("expected refresh token 'logout-token', got %s", parsed.RefreshToken)
	}
}

func TestRefreshInvalidJWT(t *testing.T) {
	jwtMgr := auth.NewJWTManager("secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := &AuthHandler{jwt: jwtMgr}

	body := userapi.RefreshRequest{RefreshToken: "invalid-token"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuditLogAllFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := &AuthHandler{logger: logger}

	// Test with all non-empty fields
	handler.auditLog(context.Background(), nil, "test_event", true, "127.0.0.1", "Mozilla/5.0", "test details")
}

func TestLoginResultType(t *testing.T) {
	// Test loginResult struct
	lr := &loginResult{
		user:        nil,
		errResponse: func() {},
	}

	if lr.user != nil {
		t.Error("expected nil user")
	}
}

func TestLoginNilDB(t *testing.T) {
	rateLimiter := auth.NewRateLimiter(10, time.Minute)
	handler := &AuthHandler{rateLimiter: rateLimiter, db: nil}

	body := userapi.LoginRequest{Handle: "test", Password: "test"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestRefreshValidTokenButMissingDB(t *testing.T) {
	// Create a JWT with valid user ID
	jwtMgr := auth.NewJWTManager("secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := &AuthHandler{jwt: jwtMgr, db: nil}

	// Generate a valid refresh token
	userID := uuid.New()
	token, _, _ := jwtMgr.GenerateRefreshToken(userID)

	body := userapi.RefreshRequest{RefreshToken: token}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	// This will panic due to nil db - catch it
	defer func() {
		// Expected panic due to nil db - this is acceptable behavior
		_ = recover()
	}()

	handler.Refresh(rec, req)
}

func TestLogoutWithRefreshTokenNilDB(t *testing.T) {
	handler := &AuthHandler{db: nil}

	body := userapi.LogoutRequest{RefreshToken: "some-token"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/logout", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	// This will panic due to nil db when trying to get session
	defer func() {
		// Expected panic due to nil db
		_ = recover()
	}()

	handler.Logout(rec, req)
}

func TestGetClientIPNoHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/", http.NoBody)
	req.RemoteAddr = ""

	ip := getClientIP(req)
	if ip != "" {
		t.Errorf("expected empty IP, got %s", ip)
	}
}

func TestAuditLogWithNonEmptyDetails(t *testing.T) {
	handler := &AuthHandler{}
	userID := uuid.New()

	// Should not panic with nil db and nil logger
	handler.auditLog(context.Background(), &userID, "test", true, "1.2.3.4", "agent", "details")
}

func TestAuthHandler_SetGatewaySessionPublisher(t *testing.T) {
	h := NewAuthHandler(nil, nil, nil, nil, "", "", nil)
	h.SetGatewaySessionPublisher(nil)
}

func TestAuthHandler_SetGatewaySessionPublisher_ClearsAfterNonNil(t *testing.T) {
	nc, js, cleanup := testJetStream(t)
	defer cleanup()
	h := NewAuthHandler(nil, nil, nil, nil, "", "", nil)
	h.SetGatewaySessionPublisher(NewGatewaySessionPublisher(nc, js))
	if h.gatewayPub == nil {
		t.Fatal("expected publisher")
	}
	h.SetGatewaySessionPublisher(nil)
	if h.gatewayPub != nil {
		t.Fatal("expected nil after Set(nil)")
	}
}

func TestNewAuthHandler_WithNatsIssuer(t *testing.T) {
	iss, err := natsjwt.NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(nil, nil, nil, iss, "nats://127.0.0.1:4222", "ws://127.0.0.1:8223/nats", nil)
	if h.natsJTI == nil {
		t.Fatal("expected natsJTI map")
	}
}

func TestAuthHandler_recordSessionNatsMode(t *testing.T) {
	h := NewAuthHandler(nil, nil, nil, nil, "", "", nil)
	id := uuid.New()
	h.recordSessionNatsMode(id, true)
	h.recordSessionNatsMode(id, false)
	h.sessionNatsMu.Lock()
	v := h.sessionNatsFromClient[id]
	h.sessionNatsMu.Unlock()
	if v {
		t.Fatal("expected false")
	}
}

func TestAuthHandler_registerNatsSessionAndRevoke(t *testing.T) {
	iss, err := natsjwt.NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(nil, nil, nil, iss, "", "", nil)
	sid := uuid.New()
	h.registerNatsSession(sid, "jti-test")
	h.natsMu.Lock()
	_, ok := h.natsJTI[sid]
	h.natsMu.Unlock()
	if !ok {
		t.Fatal("expected session registered")
	}
	h.revokeNatsSession(sid)
	h.natsMu.Lock()
	_, ok = h.natsJTI[sid]
	h.natsMu.Unlock()
	if ok {
		t.Fatal("expected session revoked")
	}
}

func TestAuthHandler_applyNatsToLoginResponse(t *testing.T) {
	iss, err := natsjwt.NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(nil, nil, nil, iss, "nats://127.0.0.1:4222", "ws://127.0.0.1:8223/nats", nil)
	resp := &userapi.LoginResponse{}
	sid := uuid.New()
	exp := time.Now().Add(time.Hour)
	h.applyNatsToLoginResponse(resp, sid, exp)
	if resp.Nats == nil || resp.Nats.JWT == "" || resp.Nats.URL == "" {
		t.Fatalf("expected nats block: %#v", resp.Nats)
	}
}

func TestAuthHandler_revokeNatsSession(t *testing.T) {
	iss, err := natsjwt.NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(nil, nil, nil, iss, "", "", nil)
	sid := uuid.New()
	tok, err := iss.SessionJWT(natsjwt.DefaultTenantID, sid, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	jti, err := natsjwt.ExtractJTI(tok)
	if err != nil {
		t.Fatal(err)
	}
	h.registerNatsSession(sid, jti)
	h.revokeNatsSession(sid)
	if !iss.Revoked(jti) {
		t.Fatal("expected jti revoked")
	}
}
