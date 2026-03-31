package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func newMockRefreshSession(userID uuid.UUID, tokenHash []byte, expiresAt time.Time) *models.RefreshSession {
	now := time.Now().UTC()
	return &models.RefreshSession{
		RefreshSessionBase: models.RefreshSessionBase{
			UserID:           userID,
			RefreshTokenHash: tokenHash,
			IsActive:         true,
			ExpiresAt:        expiresAt,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Tests for completeLogin error paths
type sessionErrorMockDB struct {
	*testutil.MockDB
	failOnCreateSession bool
}

func (m *sessionErrorMockDB) CreateRefreshSession(ctx context.Context, _ uuid.UUID, _ []byte, _ time.Time) (*models.RefreshSession, error) {
	if m.failOnCreateSession {
		return nil, errors.New("session creation failed")
	}
	return m.MockDB.CreateRefreshSession(ctx, uuid.New(), nil, time.Now())
}

func TestAuthHandler_LoginSessionCreationError(t *testing.T) {
	mockDB := &sessionErrorMockDB{
		MockDB:              testutil.NewMockDB(),
		failOnCreateSession: true,
	}
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	user := &models.User{
		UserBase: models.UserBase{
			Handle:   "testuser",
			IsActive: true,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	passwordHash, _ := auth.HashPassword("testpassword123", nil)
	cred := &models.PasswordCredential{
		PasswordCredentialBase: models.PasswordCredentialBase{
			UserID:       user.ID,
			PasswordHash: passwordHash,
			HashAlg:      "argon2id",
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddPasswordCredential(cred)

	body := userapi.LoginRequest{Handle: "testuser", Password: "testpassword123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- Auth Handler Tests ---

func TestAuthHandler_LoginSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	// Create user and password
	user := &models.User{
		UserBase: models.UserBase{
			Handle:   "testuser",
			IsActive: true,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	passwordHash, _ := auth.HashPassword("testpassword123", nil)
	cred := &models.PasswordCredential{
		PasswordCredentialBase: models.PasswordCredentialBase{
			UserID:       user.ID,
			PasswordHash: passwordHash,
			HashAlg:      "argon2id",
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddPasswordCredential(cred)

	body := userapi.LoginRequest{Handle: "testuser", Password: "testpassword123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp userapi.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected access token")
	}
	if resp.RefreshToken == "" {
		t.Error("expected refresh token")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("expected Bearer token type, got %s", resp.TokenType)
	}
}

func TestAuthHandler_LoginUserNotFound(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	body := userapi.LoginRequest{Handle: "nonexistent", Password: "anypassword"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuthHandler_LoginInvalidPassword(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	user := &models.User{
		UserBase: models.UserBase{
			Handle:   "testuser",
			IsActive: true,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	passwordHash, _ := auth.HashPassword("correctpassword", nil)
	cred := &models.PasswordCredential{
		PasswordCredentialBase: models.PasswordCredentialBase{
			UserID:       user.ID,
			PasswordHash: passwordHash,
			HashAlg:      "argon2id",
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddPasswordCredential(cred)

	body := userapi.LoginRequest{Handle: "testuser", Password: "wrongpassword"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func mockDBAddInactiveUser(db *testutil.MockDB, now time.Time) *models.User {
	u := &models.User{}
	u.ID, u.Handle, u.IsActive, u.CreatedAt, u.UpdatedAt = uuid.New(), "inactiveuser", false, now, now
	db.AddUser(u)
	return u
}

func mockDBAddUserNoCred(db *testutil.MockDB, now time.Time) *models.User {
	u := &models.User{
		UserBase:  models.UserBase{Handle: "nopassworduser", IsActive: true},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	db.AddUser(u)
	return u
}

func TestAuthHandler_LoginUnauthorizedCases(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name          string
		setupUser     func(*testutil.MockDB, time.Time) *models.User
		addCredential bool
		loginHandle   string
		loginPassword string
		wantStatus    int
	}{
		{"inactive user", mockDBAddInactiveUser, true, "inactiveuser", "password", http.StatusUnauthorized},
		{"no password credential", mockDBAddUserNoCred, false, "nopassworduser", "password", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := testutil.NewMockDB()
			u := tt.setupUser(mockDB, now)
			if tt.addCredential {
				hash, _ := auth.HashPassword(tt.loginPassword, nil)
				mockDB.AddPasswordCredential(&models.PasswordCredential{
					PasswordCredentialBase: models.PasswordCredentialBase{
						UserID:       u.ID,
						PasswordHash: hash,
						HashAlg:      "bcrypt",
					},
					ID:        uuid.New(),
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
			handler := NewAuthHandler(mockDB, auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour), auth.NewRateLimiter(100, time.Minute), newTestLogger())
			req, rec := recordedRequestJSON("POST", "/v1/auth/login", userapi.LoginRequest{Handle: tt.loginHandle, Password: tt.loginPassword})
			handler.Login(rec, req)
			assertStatusCode(t, rec, tt.wantStatus)
		})
	}
}

func TestAuthHandler_LoginDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	body := userapi.LoginRequest{Handle: "testuser", Password: "testpassword"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestAuthHandler_RefreshSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	user := &models.User{
		UserBase: models.UserBase{
			Handle:   "testuser",
			IsActive: true,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	// Generate refresh token
	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)

	session := newMockRefreshSession(user.ID, tokenHash, expiresAt)
	mockDB.AddRefreshSession(session)

	body := userapi.RefreshRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp userapi.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected new access token")
	}
}

func TestAuthHandler_RefreshInvalidUserIDInToken(t *testing.T) {
	secret := "test-secret-key-1234567890123456"
	jwtMgr := auth.NewJWTManager(secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewAuthHandler(testutil.NewMockDB(), jwtMgr, auth.NewRateLimiter(100, time.Minute), newTestLogger())
	now := time.Now()
	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cynodeai",
			Subject:   "not-a-valid-uuid",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			ID:        uuid.New().String(),
		},
		TokenType: auth.TokenTypeRefresh,
		UserID:    "not-a-valid-uuid",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	req, rec := recordedRequestJSON("POST", "/v1/auth/refresh", userapi.RefreshRequest{RefreshToken: tokenStr})
	handler.Refresh(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthHandler_RefreshInactiveUser(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	user := &models.User{
		UserBase: models.UserBase{
			Handle:   "testuser",
			IsActive: false, // Inactive
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)

	session := newMockRefreshSession(user.ID, tokenHash, expiresAt)
	mockDB.AddRefreshSession(session)

	body := userapi.RefreshRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuthHandler_RefreshSessionNotFound(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	user := &models.User{
		UserBase: models.UserBase{
			Handle:   "testuser",
			IsActive: true,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	refreshToken, _, _ := jwtMgr.GenerateRefreshToken(user.ID)
	// No session added

	body := userapi.RefreshRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuthHandler_LogoutWithRefreshToken(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	user := &models.User{
		UserBase: models.UserBase{
			Handle:   "testuser",
			IsActive: true,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)

	session := newMockRefreshSession(user.ID, tokenHash, expiresAt)
	mockDB.AddRefreshSession(session)

	ctx := context.Background()
	lineage := models.SessionBindingLineage{UserID: user.ID, SessionID: session.ID, ThreadID: nil}
	bindKey := models.DeriveSessionBindingKey(lineage)
	if _, err := mockDB.UpsertSessionBinding(ctx, lineage, models.PMAServiceIDForBindingKey(bindKey), models.SessionBindingStateActive); err != nil {
		t.Fatal(err)
	}

	reqCtx := context.WithValue(ctx, contextKeyUserID, user.ID)
	body := userapi.LogoutRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/logout", bytes.NewBuffer(jsonBody)).WithContext(reqCtx)
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}
	b, err := mockDB.GetSessionBindingByKey(ctx, bindKey)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != models.SessionBindingStateTeardownPending {
		t.Fatalf("binding state %q want teardown_pending after logout", b.State)
	}
	if rec := LastPMATeardownForTest(); rec == nil || rec.BindingKey != bindKey {
		t.Fatalf("expected PMA teardown record for binding %q, got %+v", bindKey, rec)
	}
}

// --- User Handler Tests ---

func TestUserHandler_GetMeSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewUserHandler(mockDB, logger)

	email := "test@example.com"
	user := &models.User{
		UserBase: models.UserBase{
			Handle:   "testuser",
			Email:    &email,
			IsActive: true,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	ctx := context.WithValue(context.Background(), contextKeyUserID, user.ID)
	req := httptest.NewRequest("GET", "/v1/users/me", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp userapi.UserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Handle != "testuser" {
		t.Errorf("expected handle 'testuser', got %s", resp.Handle)
	}
}

func TestUserHandler_GetMeDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	logger := newTestLogger()

	handler := NewUserHandler(mockDB, logger)

	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/users/me", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetMe(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

// invalidateSessionErrorStore fails only InvalidateRefreshSession (Refresh token rotation).
type invalidateSessionErrorStore struct {
	*testutil.MockDB
}

func (m *invalidateSessionErrorStore) InvalidateRefreshSession(_ context.Context, _ uuid.UUID) error {
	return errors.New("invalidate session error")
}

// refreshWithStore runs Refresh with a pre-configured user and session on base, using store as the handler's DB. Asserts rec.Code == expectedCode.
func refreshWithStore(t *testing.T, base *testutil.MockDB, store database.Store, expectedCode int) {
	t.Helper()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewAuthHandler(store, jwtMgr, auth.NewRateLimiter(100, time.Minute), newTestLogger())
	user := &models.User{
		UserBase:  models.UserBase{Handle: "u", IsActive: true},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	base.AddUser(user)
	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)
	session := newMockRefreshSession(user.ID, tokenHash, expiresAt)
	base.AddRefreshSession(session)
	req, rec := recordedRequestJSON("POST", "/v1/auth/refresh", userapi.RefreshRequest{RefreshToken: refreshToken})
	handler.Refresh(rec, req)
	if rec.Code != expectedCode {
		t.Errorf("expected status %d, got %d", expectedCode, rec.Code)
	}
}

func TestAuthHandler_Refresh_InvalidateSessionFails(t *testing.T) {
	base := testutil.NewMockDB()
	refreshWithStore(t, base, &invalidateSessionErrorStore{MockDB: base}, http.StatusInternalServerError)
}

// getUserByIDErrorStore fails GetUserByID (Refresh path: user not found).
type getUserByIDErrorStore struct {
	*testutil.MockDB
}

func (m *getUserByIDErrorStore) GetUserByID(_ context.Context, _ uuid.UUID) (*models.User, error) {
	return nil, errors.New("user not found")
}

func TestAuthHandler_Refresh_GetUserByIDFails(t *testing.T) {
	base := testutil.NewMockDB()
	refreshWithStore(t, base, &getUserByIDErrorStore{MockDB: base}, http.StatusUnauthorized)
}

func TestAuthHandler_Refresh_GetActiveRefreshSessionError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("db error")
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewAuthHandler(mockDB, jwtMgr, auth.NewRateLimiter(100, time.Minute), newTestLogger())

	refreshToken, _, _ := jwtMgr.GenerateRefreshToken(uuid.New())

	req, rec := recordedRequestJSON("POST", "/v1/auth/refresh", userapi.RefreshRequest{RefreshToken: refreshToken})
	handler.Refresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestAuthHandler_Refresh_CreateSessionFails(t *testing.T) {
	mockDB := &sessionErrorMockDB{MockDB: testutil.NewMockDB(), failOnCreateSession: true}
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewAuthHandler(mockDB, jwtMgr, auth.NewRateLimiter(100, time.Minute), newTestLogger())

	user := &models.User{
		UserBase:  models.UserBase{Handle: "u", IsActive: true},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)
	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)
	session := newMockRefreshSession(user.ID, tokenHash, expiresAt)
	mockDB.AddRefreshSession(session)

	req, rec := recordedRequestJSON("POST", "/v1/auth/refresh", userapi.RefreshRequest{RefreshToken: refreshToken})
	handler.Refresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}
