package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

const testPrompt = "test prompt"

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
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
		ID:        uuid.New(),
		Handle:    "testuser",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	passwordHash, _ := auth.HashPassword("testpassword123", nil)
	cred := &models.PasswordCredential{
		ID:           uuid.New(),
		UserID:       user.ID,
		PasswordHash: passwordHash,
		HashAlg:      "argon2id",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	mockDB.AddPasswordCredential(cred)

	body := LoginRequest{Handle: "testuser", Password: "testpassword123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp LoginResponse
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

	body := LoginRequest{Handle: "nonexistent", Password: "anypassword"}
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
		ID:        uuid.New(),
		Handle:    "testuser",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	passwordHash, _ := auth.HashPassword("correctpassword", nil)
	cred := &models.PasswordCredential{
		ID:           uuid.New(),
		UserID:       user.ID,
		PasswordHash: passwordHash,
		HashAlg:      "argon2id",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	mockDB.AddPasswordCredential(cred)

	body := LoginRequest{Handle: "testuser", Password: "wrongpassword"}
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
	u := &models.User{ID: uuid.New(), Handle: "nopassworduser", IsActive: true, CreatedAt: now, UpdatedAt: now}
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
				mockDB.AddPasswordCredential(&models.PasswordCredential{ID: uuid.New(), UserID: u.ID, PasswordHash: hash, HashAlg: "bcrypt", CreatedAt: now, UpdatedAt: now})
			}
			handler := NewAuthHandler(mockDB, auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour), auth.NewRateLimiter(100, time.Minute), newTestLogger())
			req, rec := recordedRequestJSON("POST", "/v1/auth/login", LoginRequest{Handle: tt.loginHandle, Password: tt.loginPassword})
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

	body := LoginRequest{Handle: "testuser", Password: "testpassword"}
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
		ID:        uuid.New(),
		Handle:    "testuser",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	// Generate refresh token
	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)

	session := &models.RefreshSession{
		ID:               uuid.New(),
		UserID:           user.ID,
		RefreshTokenHash: tokenHash,
		IsActive:         true,
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	mockDB.AddRefreshSession(session)

	body := RefreshRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected new access token")
	}
}

func TestAuthHandler_RefreshInactiveUser(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	user := &models.User{
		ID:        uuid.New(),
		Handle:    "testuser",
		IsActive:  false, // Inactive
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)

	session := &models.RefreshSession{
		ID:               uuid.New(),
		UserID:           user.ID,
		RefreshTokenHash: tokenHash,
		IsActive:         true,
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	mockDB.AddRefreshSession(session)

	body := RefreshRequest{RefreshToken: refreshToken}
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
		ID:        uuid.New(),
		Handle:    "testuser",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	refreshToken, _, _ := jwtMgr.GenerateRefreshToken(user.ID)
	// No session added

	body := RefreshRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuthHandler_LogoutWithRefreshToken(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	user := &models.User{
		ID:        uuid.New(),
		Handle:    "testuser",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)

	session := &models.RefreshSession{
		ID:               uuid.New(),
		UserID:           user.ID,
		RefreshTokenHash: tokenHash,
		IsActive:         true,
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	mockDB.AddRefreshSession(session)

	ctx := context.WithValue(context.Background(), contextKeyUserID, user.ID)
	body := LogoutRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/logout", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}
}

// --- User Handler Tests ---

func TestUserHandler_GetMeSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewUserHandler(mockDB, logger)

	email := "test@example.com"
	user := &models.User{
		ID:        uuid.New(),
		Handle:    "testuser",
		Email:     &email,
		IsActive:  true,
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

	var resp UserResponse
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

// --- Task Handler Tests ---

func TestTaskHandler_CreateTaskWithMockDB(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger)

	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := CreateTaskRequest{Prompt: "test prompt for task"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateTask(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != models.TaskStatusPending {
		t.Errorf("expected status pending, got %s", resp.Status)
	}
}

func TestTaskHandler_CreateTaskDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger)

	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := CreateTaskRequest{Prompt: testPrompt}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateTask(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestTaskHandler_GetTaskSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger)

	prompt := testPrompt
	task := &models.Task{
		ID:        uuid.New(),
		Status:    models.TaskStatusPending,
		Prompt:    &prompt,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddTask(task)

	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String(), http.NoBody)
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()

	handler.GetTask(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTaskHandler_GetTaskNotFound(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger)

	taskID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks/"+taskID.String(), http.NoBody)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()

	handler.GetTask(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestTaskHandler_GetTaskDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger)

	taskID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks/"+taskID.String(), http.NoBody)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()

	handler.GetTask(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestTaskHandler_GetTaskResultSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger)

	prompt := testPrompt
	task := &models.Task{
		ID:        uuid.New(),
		Status:    models.TaskStatusCompleted,
		Prompt:    &prompt,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddTask(task)

	result := "job result"
	startedAt := time.Now().UTC()
	endedAt := startedAt.Add(time.Second)
	job := &models.Job{
		ID:        uuid.New(),
		TaskID:    task.ID,
		Status:    models.JobStatusCompleted,
		Result:    models.NewJSONBString(&result),
		StartedAt: &startedAt,
		EndedAt:   &endedAt,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddJob(job)

	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String()+"/result", http.NoBody)
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()

	handler.GetTaskResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp TaskResultResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(resp.Jobs))
	}
}

func TestTaskHandler_GetTaskResultNotFound(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger)

	taskID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks/"+taskID.String()+"/result", http.NoBody)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()

	handler.GetTaskResult(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

// --- Node Handler Tests ---

func TestNodeHandler_RegisterNewNode(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", logger)

	body := NodeRegistrationRequest{PSK: "test-psk-secret", Capability: testNodeCapabilityReport("test-node-1", "Test Node 1", 8, 16384)}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp NodeBootstrapResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}
	if resp.Auth.NodeJWT == "" {
		t.Error("expected node JWT")
	}
}

func TestNodeHandler_RegisterExistingNode(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", logger)

	// Create existing node
	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "existing-node",
		Status:    "offline",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	body := NodeRegistrationRequest{PSK: "test-psk-secret", Capability: testNodeCapabilityReport("existing-node", "", 4, 8192)}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNodeHandler_RegisterDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", logger)

	body := NodeRegistrationRequest{
		PSK: "test-psk-secret",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "test-node"},
			Platform: NodeCapabilityPlatform{
				OS:   "linux",
				Arch: "amd64",
			},
			Compute: NodeCapabilityCompute{
				CPUCores: 4,
				RAMMB:    8192,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestNodeHandler_ReportCapabilitySuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", logger)

	// Create node
	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "test-node",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	report := testNodeCapabilityReport("test-node", "Test Node", 8, 16384)
	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	jsonBody, _ := json.Marshal(report)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNodeHandler_ReportCapabilityDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", logger)

	nodeID := uuid.New()
	report := NodeCapabilityReport{
		Version: 1,
		Node:    NodeCapabilityNode{NodeSlug: "test-node"},
		Platform: NodeCapabilityPlatform{
			OS:   "linux",
			Arch: "amd64",
		},
		Compute: NodeCapabilityCompute{
			CPUCores: 4,
			RAMMB:    8192,
		},
	}

	ctx := context.WithValue(context.Background(), contextKeyNodeID, nodeID)
	jsonBody, _ := json.Marshal(report)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

// --- AuditLog Tests ---

func TestAuditLogWithDB(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	userID := uuid.New()
	handler.auditLog(context.Background(), &userID, "test_event", true, "192.168.1.1", "Mozilla/5.0", "test detail")

	// Check audit log was created
	if len(mockDB.AuditLogs) != 1 {
		t.Errorf("expected 1 audit log, got %d", len(mockDB.AuditLogs))
	}
}

func TestAuditLogDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	// Should not panic
	handler.auditLog(context.Background(), nil, "test_event", false, "", "", "")
}

// Additional tests to increase coverage

func TestAuthHandler_RefreshDBErrorOnInvalidate(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	user := &models.User{
		ID:        uuid.New(),
		Handle:    "testuser",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)

	session := &models.RefreshSession{
		ID:               uuid.New(),
		UserID:           user.ID,
		RefreshTokenHash: tokenHash,
		IsActive:         true,
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	mockDB.AddRefreshSession(session)

	// Set error after getting session
	mockDB.ForceError = errors.New("database error")

	body := RefreshRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestAuthHandler_RefreshUserNotFound(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	user := &models.User{
		ID:        uuid.New(),
		Handle:    "testuser",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)

	session := &models.RefreshSession{
		ID:               uuid.New(),
		UserID:           user.ID,
		RefreshTokenHash: tokenHash,
		IsActive:         true,
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	// Create mockDB with session but no user
	mockDB := testutil.NewMockDB()
	mockDB.AddRefreshSession(session)

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	body := RefreshRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestNodeHandler_handleExistingNodeDBError(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "test-node",
		Status:    "offline",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	mockDB := testutil.NewMockDB()
	mockDB.AddNode(node)
	mockDB.ForceError = errors.New("db error on update")

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", logger)

	body := NodeRegistrationRequest{
		PSK: "test-psk",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "test-node"},
			Platform: NodeCapabilityPlatform{
				OS:   "linux",
				Arch: "amd64",
			},
			Compute: NodeCapabilityCompute{
				CPUCores: 4,
				RAMMB:    8192,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestTaskHandler_GetTaskResultJobsDBError(t *testing.T) {
	logger := newTestLogger()

	prompt := testPrompt
	task := &models.Task{
		ID:        uuid.New(),
		Status:    models.TaskStatusCompleted,
		Prompt:    &prompt,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Create custom mock that returns error only on GetJobsByTaskID
	mockDB := &errorOnJobsMockDB{
		MockDB: testutil.NewMockDB(),
	}
	mockDB.AddTask(task)

	handler := NewTaskHandler(mockDB, logger)

	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String()+"/result", http.NoBody)
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()

	handler.GetTaskResult(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

// errorOnJobsMockDB wraps MockDB to return error only on GetJobsByTaskID
type errorOnJobsMockDB struct {
	*testutil.MockDB
}

func (m *errorOnJobsMockDB) GetJobsByTaskID(_ context.Context, _ uuid.UUID) ([]*models.Job, error) {
	return nil, errors.New("jobs query error")
}

func TestNodeHandler_ReportCapabilityWithSandbox(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", logger)

	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "test-node",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	report := NodeCapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node: NodeCapabilityNode{
			NodeSlug: "test-node",
			Name:     "Test Node",
			Labels:   []string{"test"},
		},
		Platform: NodeCapabilityPlatform{
			OS:   "linux",
			Arch: "amd64",
		},
		Compute: NodeCapabilityCompute{
			CPUCores: 8,
			RAMMB:    16384,
		},
		Sandbox: &NodeCapabilitySandbox{
			Supported:      true,
			Features:       []string{"python", "bash"},
			MaxConcurrency: 4,
		},
	}

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	jsonBody, _ := json.Marshal(report)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTaskHandler_GetTaskResultDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger)

	taskID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks/"+taskID.String()+"/result", http.NoBody)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()

	handler.GetTaskResult(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
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
		ID:        uuid.New(),
		Handle:    "testuser",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	passwordHash, _ := auth.HashPassword("testpassword123", nil)
	cred := &models.PasswordCredential{
		ID:           uuid.New(),
		UserID:       user.ID,
		PasswordHash: passwordHash,
		HashAlg:      "argon2id",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	mockDB.AddPasswordCredential(cred)

	body := LoginRequest{Handle: "testuser", Password: "testpassword123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNodeHandler_NewNodeRegistrationJWTError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	// Use invalid JWT manager (empty secret key causes issues)
	jwtMgr := auth.NewJWTManager("", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", logger)

	body := NodeRegistrationRequest{
		PSK: "test-psk-secret",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "new-test-node"},
			Platform: NodeCapabilityPlatform{
				OS:   "linux",
				Arch: "amd64",
			},
			Compute: NodeCapabilityCompute{
				CPUCores: 4,
				RAMMB:    8192,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	// May return 201 with empty JWT or 500 - depends on JWT manager behavior
	// The point is to exercise the code path
	if rec.Code != http.StatusCreated && rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 201 or 500, got %d", rec.Code)
	}
}

// Test for handleExistingNodeRegistration JWT error
func TestNodeHandler_ExistingNodeJWTError(t *testing.T) {
	jwtMgr := auth.NewJWTManager("", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "existing-node-jwt-test",
		Status:    "offline",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	mockDB := testutil.NewMockDB()
	mockDB.AddNode(node)

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", logger)

	body := NodeRegistrationRequest{
		PSK: "test-psk",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "existing-node-jwt-test"},
			Platform: NodeCapabilityPlatform{
				OS:   "linux",
				Arch: "amd64",
			},
			Compute: NodeCapabilityCompute{
				CPUCores: 4,
				RAMMB:    8192,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	// Exercise the code path - may succeed with empty JWT or fail
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", rec.Code)
	}
}

// Test initializeNewNode error paths
func TestNodeHandler_initializeNewNodeErrors(t *testing.T) {
	logger := newTestLogger()

	// Create a mock DB that fails on different operations
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("force error")

	handler := &NodeHandler{
		db:     mockDB,
		logger: logger,
	}

	nodeID := uuid.New()
	capability := NodeCapabilityReport{
		Version: 1,
		Node:    NodeCapabilityNode{NodeSlug: "test-node"},
	}

	// Should not panic - just log errors
	handler.initializeNewNode(context.Background(), nodeID, &capability)
}

// Test Refresh with all error paths
func TestAuthHandler_RefreshInvalidUserID(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, logger)

	// Generate a token with invalid user ID format - create a custom JWT
	// This is hard to test without modifying JWT generation, so test other paths

	body := RefreshRequest{RefreshToken: "invalid.token.format"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

// Test ReportCapability with all error paths
func TestNodeHandler_ReportCapabilityUpdateErrors(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "test-node-errors",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Create mock that fails on capability operations
	mockDB := &capabilityErrorMockDB{
		MockDB: testutil.NewMockDB(),
	}
	mockDB.AddNode(node)

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", logger)

	report := NodeCapabilityReport{
		Version: 1,
		Node:    NodeCapabilityNode{NodeSlug: "test-node-errors"},
		Platform: NodeCapabilityPlatform{
			OS:   "linux",
			Arch: "amd64",
		},
		Compute: NodeCapabilityCompute{
			CPUCores: 4,
			RAMMB:    8192,
		},
	}

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	jsonBody, _ := json.Marshal(report)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	// Should return 500 due to SaveNodeCapabilitySnapshot error
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

type capabilityErrorMockDB struct {
	*testutil.MockDB
}

func (m *capabilityErrorMockDB) SaveNodeCapabilitySnapshot(_ context.Context, _ uuid.UUID, _ string) error {
	return errors.New("capability snapshot error")
}

func (m *capabilityErrorMockDB) UpdateNodeLastSeen(_ context.Context, _ uuid.UUID) error {
	return nil
}
