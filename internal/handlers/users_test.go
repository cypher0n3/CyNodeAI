package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestNewUserHandler(t *testing.T) {
	handler := NewUserHandler(nil, nil)
	if handler == nil {
		t.Fatal("NewUserHandler returned nil")
	}
}

func TestGetMeNoUser(t *testing.T) {
	handler := &UserHandler{}

	req := httptest.NewRequest("GET", "/v1/users/me", http.NoBody)
	rec := httptest.NewRecorder()

	handler.GetMe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestGetMeWithUserID(t *testing.T) {
	handler := &UserHandler{}

	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/users/me", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	// This will fail because db is nil, but we're testing the context extraction
	handler.GetMe(rec, req)

	// Should get internal error because db is nil (not unauthorized)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 (db nil), got %d", rec.Code)
	}
}

func TestUserResponseJSON(t *testing.T) {
	email := "test@example.com"
	resp := UserResponse{
		ID:       "user-id",
		Handle:   "testuser",
		Email:    &email,
		IsActive: true,
	}

	if resp.ID != "user-id" {
		t.Errorf("expected ID 'user-id', got %s", resp.ID)
	}
	if resp.Handle != "testuser" {
		t.Errorf("expected handle 'testuser', got %s", resp.Handle)
	}
}

func TestUserResponseNilEmail(t *testing.T) {
	resp := UserResponse{
		ID:       "user-id",
		Handle:   "testuser",
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
	req := httptest.NewRequest("GET", "/v1/users/me", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetMe(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestUserResponseIsInactive(t *testing.T) {
	resp := UserResponse{
		ID:       "user-id",
		Handle:   "inactive-user",
		Email:    nil,
		IsActive: false,
	}

	if resp.IsActive {
		t.Error("expected IsActive to be false")
	}
}
