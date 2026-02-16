package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTManager_GenerateAndValidateAccessToken(t *testing.T) {
	mgr := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	userID := uuid.New()
	handle := "testuser"

	token, err := mgr.GenerateAccessToken(userID, handle)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	if token == "" {
		t.Fatal("GenerateAccessToken() returned empty token")
	}

	claims, err := mgr.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	if claims.UserID != userID.String() {
		t.Errorf("claims.UserID = %v, want %v", claims.UserID, userID.String())
	}

	if claims.Handle != handle {
		t.Errorf("claims.Handle = %v, want %v", claims.Handle, handle)
	}

	if claims.TokenType != TokenTypeAccess {
		t.Errorf("claims.TokenType = %v, want %v", claims.TokenType, TokenTypeAccess)
	}
}

func TestJWTManager_GenerateAndValidateRefreshToken(t *testing.T) {
	mgr := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	userID := uuid.New()

	token, expiresAt, err := mgr.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	if token == "" {
		t.Fatal("GenerateRefreshToken() returned empty token")
	}

	if expiresAt.Before(time.Now()) {
		t.Error("expiresAt is in the past")
	}

	claims, err := mgr.ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken() error = %v", err)
	}

	if claims.UserID != userID.String() {
		t.Errorf("claims.UserID = %v, want %v", claims.UserID, userID.String())
	}

	if claims.TokenType != TokenTypeRefresh {
		t.Errorf("claims.TokenType = %v, want %v", claims.TokenType, TokenTypeRefresh)
	}
}

func TestJWTManager_GenerateAndValidateNodeToken(t *testing.T) {
	mgr := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	nodeID := uuid.New()
	nodeSlug := "test-node-01"

	token, expiresAt, err := mgr.GenerateNodeToken(nodeID, nodeSlug)
	if err != nil {
		t.Fatalf("GenerateNodeToken() error = %v", err)
	}

	if token == "" {
		t.Fatal("GenerateNodeToken() returned empty token")
	}

	if expiresAt.Before(time.Now()) {
		t.Error("expiresAt is in the past")
	}

	claims, err := mgr.ValidateNodeToken(token)
	if err != nil {
		t.Fatalf("ValidateNodeToken() error = %v", err)
	}

	if claims.NodeID != nodeID.String() {
		t.Errorf("claims.NodeID = %v, want %v", claims.NodeID, nodeID.String())
	}

	if claims.NodeSlug != nodeSlug {
		t.Errorf("claims.NodeSlug = %v, want %v", claims.NodeSlug, nodeSlug)
	}

	if claims.TokenType != TokenTypeNode {
		t.Errorf("claims.TokenType = %v, want %v", claims.TokenType, TokenTypeNode)
	}
}

func TestJWTManager_InvalidToken(t *testing.T) {
	mgr := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	_, err := mgr.ValidateToken("invalid-token")
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestJWTManager_WrongTokenType(t *testing.T) {
	mgr := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	userID := uuid.New()
	accessToken, _ := mgr.GenerateAccessToken(userID, "testuser")

	// Try to validate access token as refresh token
	_, err := mgr.ValidateRefreshToken(accessToken)
	if err != ErrInvalidToken {
		t.Errorf("ValidateRefreshToken() error = %v, want %v", err, ErrInvalidToken)
	}

	// Try to validate access token as node token
	_, err = mgr.ValidateNodeToken(accessToken)
	if err != ErrInvalidToken {
		t.Errorf("ValidateNodeToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestJWTManager_DifferentSecret(t *testing.T) {
	mgr1 := NewJWTManager("secret-1", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	mgr2 := NewJWTManager("secret-2", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	userID := uuid.New()
	token, _ := mgr1.GenerateAccessToken(userID, "testuser")

	_, err := mgr2.ValidateAccessToken(token)
	if err != ErrInvalidToken {
		t.Errorf("ValidateAccessToken() with different secret error = %v, want %v", err, ErrInvalidToken)
	}
}
