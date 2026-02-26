package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

const testUserHandle = "testuser"

func TestNewUserHandler(t *testing.T) {
	handler := NewUserHandler(nil, nil)
	if handler == nil {
		t.Fatal("NewUserHandler returned nil")
	}
}

func TestGetMeNoUser(t *testing.T) {
	handler := &UserHandler{}
	req, rec := recordedRequest("GET", "/v1/users/me", nil)
	handler.GetMe(rec, req)
	assertStatusCode(t, rec, http.StatusUnauthorized)
}

func TestGetMeWithUserID(t *testing.T) {
	handler := &UserHandler{}
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req, rec := recordedRequest("GET", "/v1/users/me", nil)
	req = req.WithContext(ctx)
	handler.GetMe(rec, req)
	assertStatusCode(t, rec, http.StatusInternalServerError)
}

func TestUserResponseJSON(t *testing.T) {
	email := "test@example.com"
	resp := userapi.UserResponse{
		ID:       "user-id",
		Handle:   testUserHandle,
		Email:    &email,
		IsActive: true,
	}

	if resp.ID != "user-id" {
		t.Errorf("expected ID 'user-id', got %s", resp.ID)
	}
	if resp.Handle != testUserHandle {
		t.Errorf("expected handle %q, got %s", testUserHandle, resp.Handle)
	}
}

func TestUserResponseNilEmail(t *testing.T) {
	resp := userapi.UserResponse{
		ID:       "user-id",
		Handle:   testUserHandle,
		Email:    nil,
		IsActive: true,
	}

	if resp.Email != nil {
		t.Error("expected nil email")
	}
}

func TestGetMeNilDB(t *testing.T) {
	handler := &UserHandler{db: nil}
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req, rec := recordedRequest("GET", "/v1/users/me", nil)
	req = req.WithContext(ctx)
	handler.GetMe(rec, req)
	assertStatusCode(t, rec, http.StatusInternalServerError)
}

func TestRevokeSessions_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	user, _ := mock.CreateUser(ctx, "targetuser", nil)
	handler := NewUserHandler(mock, nil)
	mux := http.NewServeMux()
	mux.Handle("POST /v1/users/{id}/revoke_sessions", http.HandlerFunc(handler.RevokeSessions))
	req := httptest.NewRequest(http.MethodPost, "http://test/v1/users/"+user.ID.String()+"/revoke_sessions", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserID, user.ID))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("RevokeSessions: got %d, want 204", rec.Code)
	}
}

func TestRevokeSessions_ErrorCases(t *testing.T) {
	tests := []struct {
		name     string
		pathID   string
		wantCode int
	}{
		{"not_found", "550e8400-e29b-41d4-a716-446655440000", http.StatusNotFound},
		{"invalid_id", "not-a-uuid", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := testutil.NewMockDB()
			handler := NewUserHandler(mock, nil)
			mux := http.NewServeMux()
			mux.Handle("POST /v1/users/{id}/revoke_sessions", http.HandlerFunc(handler.RevokeSessions))
			req := httptest.NewRequest(http.MethodPost, "http://test/v1/users/"+tt.pathID+"/revoke_sessions", http.NoBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("got %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}

func TestRevokeSessions_NilDB(t *testing.T) {
	handler := NewUserHandler(nil, nil)
	mux := http.NewServeMux()
	mux.Handle("POST /v1/users/{id}/revoke_sessions", http.HandlerFunc(handler.RevokeSessions))
	req := httptest.NewRequest(http.MethodPost, "http://test/v1/users/550e8400-e29b-41d4-a716-446655440000/revoke_sessions", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", rec.Code)
	}
}

func TestRevokeSessions_DBErrorOnInvalidate(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	user, _ := mock.CreateUser(ctx, "u", nil)
	mock.ForceError = errors.New("db error")
	defer func() { mock.ForceError = nil }()
	handler := NewUserHandler(mock, nil)
	mux := http.NewServeMux()
	mux.Handle("POST /v1/users/{id}/revoke_sessions", http.HandlerFunc(handler.RevokeSessions))
	req := httptest.NewRequest(http.MethodPost, "http://test/v1/users/"+user.ID.String()+"/revoke_sessions", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", rec.Code)
	}
}

func TestUserResponseIsInactive(t *testing.T) {
	resp := userapi.UserResponse{
		ID:       "user-id",
		Handle:   "inactive-user",
		Email:    nil,
		IsActive: false,
	}

	if resp.IsActive {
		t.Error("expected IsActive to be false")
	}
}
