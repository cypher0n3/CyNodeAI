package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
)

func TestAuthMiddleware_RequireUserAuth_Valid(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	jwt := auth.NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	mw := NewAuthMiddleware(jwt, logger)

	userID := uuid.New()
	token, _ := jwt.GenerateAccessToken(userID, "testuser")

	var handlerCalled bool
	handler := mw.RequireUserAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	if !handlerCalled {
		t.Error("handler was not called")
	}
}

func TestAuthMiddleware_RequireUserAuth_Missing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	jwt := auth.NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	mw := NewAuthMiddleware(jwt, logger)

	handler := mw.RequireUserAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_RequireUserAuth_Invalid(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	jwt := auth.NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	mw := NewAuthMiddleware(jwt, logger)

	handler := mw.RequireUserAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_RequireNodeAuth_Valid(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	jwt := auth.NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	mw := NewAuthMiddleware(jwt, logger)

	nodeID := uuid.New()
	token, _, _ := jwt.GenerateNodeToken(nodeID, "test-node")

	var handlerCalled bool
	handler := mw.RequireNodeAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	if !handlerCalled {
		t.Error("handler was not called")
	}
}

func TestAuthMiddleware_RequireNodeAuth_WrongTokenType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	jwt := auth.NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	mw := NewAuthMiddleware(jwt, logger)

	// Use user access token instead of node token
	userID := uuid.New()
	token, _ := jwt.GenerateAccessToken(userID, "testuser")

	handler := mw.RequireNodeAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid", "Bearer token123", "token123"},
		{"case insensitive", "bearer token123", "token123"},
		{"missing", "", ""},
		{"wrong scheme", "Basic token123", ""},
		{"no token", "Bearer", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", http.NoBody)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			got := extractBearerToken(req)
			if got != tt.want {
				t.Errorf("extractBearerToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
