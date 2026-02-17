package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
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
	resp := UserResponse{
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
	resp := UserResponse{
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
