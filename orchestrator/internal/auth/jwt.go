// Package auth provides authentication utilities.
// See docs/tech_specs/local_user_accounts.md for requirements.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType distinguishes access vs refresh tokens.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
	TokenTypeNode    TokenType = "node"
)

// Claims represents JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	TokenType TokenType `json:"token_type"`
	UserID    string    `json:"user_id,omitempty"`
	Handle    string    `json:"handle,omitempty"`
	NodeID    string    `json:"node_id,omitempty"`
	NodeSlug  string    `json:"node_slug,omitempty"`
}

// JWTManager handles JWT operations.
type JWTManager struct {
	secret          []byte
	accessDuration  time.Duration
	refreshDuration time.Duration
	nodeDuration    time.Duration
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(secret string, accessDuration, refreshDuration, nodeDuration time.Duration) *JWTManager {
	return &JWTManager{
		secret:          []byte(secret),
		accessDuration:  accessDuration,
		refreshDuration: refreshDuration,
		nodeDuration:    nodeDuration,
	}
}

// GenerateAccessToken generates an access token for a user.
func (m *JWTManager) GenerateAccessToken(userID uuid.UUID, handle string) (string, error) {
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cynodeai",
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessDuration)),
			ID:        uuid.New().String(),
		},
		TokenType: TokenTypeAccess,
		UserID:    userID.String(),
		Handle:    handle,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// GenerateRefreshToken generates a refresh token for a user.
func (m *JWTManager) GenerateRefreshToken(userID uuid.UUID) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.refreshDuration)
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cynodeai",
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.New().String(),
		},
		TokenType: TokenTypeRefresh,
		UserID:    userID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(m.secret)
	return tokenStr, expiresAt, err
}

// GenerateNodeToken generates a JWT for a node.
func (m *JWTManager) GenerateNodeToken(nodeID uuid.UUID, nodeSlug string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.nodeDuration)
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cynodeai",
			Subject:   nodeID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.New().String(),
		},
		TokenType: TokenTypeNode,
		NodeID:    nodeID.String(),
		NodeSlug:  nodeSlug,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(m.secret)
	return tokenStr, expiresAt, err
}

// ErrInvalidToken is returned when token validation fails.
var ErrInvalidToken = errors.New("invalid token")

// ValidateToken validates a JWT and returns claims.
func (m *JWTManager) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ValidateAccessToken validates an access token.
func (m *JWTManager) ValidateAccessToken(tokenStr string) (*Claims, error) {
	claims, err := m.ValidateToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// ValidateRefreshToken validates a refresh token.
func (m *JWTManager) ValidateRefreshToken(tokenStr string) (*Claims, error) {
	claims, err := m.ValidateToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeRefresh {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// ValidateNodeToken validates a node token.
func (m *JWTManager) ValidateNodeToken(tokenStr string) (*Claims, error) {
	claims, err := m.ValidateToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeNode {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
