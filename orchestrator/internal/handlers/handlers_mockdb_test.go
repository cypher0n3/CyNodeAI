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
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

const testPrompt = "test prompt"
const mimeSSE = "text/event-stream"

func newMockTask(createdBy *uuid.UUID, status string, prompt *string) *models.Task {
	now := time.Now().UTC()
	return &models.Task{
		TaskBase: models.TaskBase{
			CreatedBy: createdBy,
			Status:    status,
			Prompt:    prompt,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

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

func newMockJobSimple(taskID uuid.UUID, status string, payload, result *string) *models.Job {
	now := time.Now().UTC()
	jb := models.JobBase{TaskID: taskID, Status: status}
	if payload != nil {
		jb.Payload = models.NewJSONBString(payload)
	}
	if result != nil {
		jb.Result = models.NewJSONBString(result)
	}
	return &models.Job{
		JobBase:   jb,
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

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

	ctx := context.WithValue(context.Background(), contextKeyUserID, user.ID)
	body := userapi.LogoutRequest{RefreshToken: refreshToken}
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

// --- Task Handler Tests ---

func TestTaskHandler_CreateTaskWithMockDB(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger, "", "")

	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{Prompt: "test prompt for task"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateTask(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != userapi.StatusQueued {
		t.Errorf("expected status queued, got %s", resp.Status)
	}
}

func TestTaskHandler_CreateTask_WithTaskName(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewTaskHandler(mockDB, logger, "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	taskName := "my-custom-task"
	body := userapi.CreateTaskRequest{Prompt: "prompt", TaskName: &taskName}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.TaskName == nil || *resp.TaskName != "my-custom-task" {
		t.Errorf("expected task_name my-custom-task in response, got %v", resp.TaskName)
	}
}

func TestTaskHandler_CreateTask_WithAttachments(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewTaskHandler(mockDB, logger, "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{
		Prompt:      "prompt",
		Attachments: []string{"a.txt", "subdir/b.csv"},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Attachments) != 2 {
		t.Errorf("expected 2 attachments in response, got %d: %v", len(resp.Attachments), resp.Attachments)
	}
	taskID, _ := uuid.Parse(resp.ResolveTaskID())
	paths, err := mockDB.ListArtifactPathsByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("ListArtifactPathsByTaskID: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 artifact paths stored, got %d: %v", len(paths), paths)
	}
}

func TestTaskHandler_CreateTask_WithProjectID(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	projectID := uuid.New().String()
	body := userapi.CreateTaskRequest{Prompt: "prompt", ProjectID: &projectID}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	taskID, err := uuid.Parse(resp.ResolveTaskID())
	if err != nil {
		t.Fatalf("parse task ID: %v", err)
	}
	task, err := mockDB.GetTaskByID(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if task.ProjectID == nil || task.ProjectID.String() != projectID {
		t.Fatalf("expected task.project_id=%s, got %v", projectID, task.ProjectID)
	}
}

func TestTaskHandler_CreateTask_DefaultProjectAssignedWhenProjectIDOmitted(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{Prompt: "prompt"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	taskID, err := uuid.Parse(resp.ResolveTaskID())
	if err != nil {
		t.Fatalf("parse task ID: %v", err)
	}
	task, err := mockDB.GetTaskByID(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if task.ProjectID == nil {
		t.Fatal("expected default project to be assigned")
	}
}

func TestTaskHandler_CreateTask_InvalidProjectID(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	bad := "not-a-uuid"
	body := userapi.CreateTaskRequest{Prompt: "prompt", ProjectID: &bad}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTaskHandler_CreateTaskWithUseInference_StoresUseInferenceInJobPayload(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewTaskHandler(mockDB, logger, "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{Prompt: "echo hi", UseInference: true}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	taskID, err := uuid.Parse(resp.ResolveTaskID())
	if err != nil {
		t.Fatalf("parse task ID: %v", err)
	}
	jobs, err := mockDB.GetJobsByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("GetJobsByTaskID: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	payload := jobs[0].Payload.Ptr()
	if payload == nil {
		t.Fatal("job payload is nil")
	}
	var pl struct {
		UseInference bool `json:"use_inference"`
	}
	if err := json.Unmarshal([]byte(*payload), &pl); err != nil {
		t.Fatalf("unmarshal job payload: %v", err)
	}
	if !pl.UseInference {
		t.Error("expected job payload use_inference true")
	}
}

func TestTaskHandler_CreateTask_UseSBA_StoresSBAJobPayload(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{Prompt: "sba prompt", UseSBA: true}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	taskID, err := uuid.Parse(resp.ResolveTaskID())
	if err != nil {
		t.Fatalf("parse task ID: %v", err)
	}
	jobs, err := mockDB.GetJobsByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("GetJobsByTaskID: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	payload := jobs[0].Payload.Ptr()
	if payload == nil {
		t.Fatal("job payload is nil")
	}
	var pl struct {
		JobSpecJSON string `json:"job_spec_json"`
		Image       string `json:"image"`
	}
	if err := json.Unmarshal([]byte(*payload), &pl); err != nil {
		t.Fatalf("unmarshal job payload: %v", err)
	}
	if pl.JobSpecJSON == "" {
		t.Error("expected job_spec_json in payload")
	}
	if pl.Image == "" {
		t.Error("expected image in payload")
	}
	var jobSpec struct {
		ExecutionMode string `json:"execution_mode"`
		Inference     struct {
			AllowedModels []string `json:"allowed_models"`
		} `json:"inference"`
		Steps []struct{} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(pl.JobSpecJSON), &jobSpec); err != nil {
		t.Fatalf("unmarshal job_spec_json: %v", err)
	}
	if jobSpec.ExecutionMode != "agent_inference" {
		t.Errorf("execution_mode want agent_inference got %q", jobSpec.ExecutionMode)
	}
	if len(jobSpec.Inference.AllowedModels) == 0 {
		t.Error("expected non-empty inference.allowed_models")
	}
	if len(jobSpec.Steps) != 0 {
		t.Errorf("expected no forced placeholder steps, got %d", len(jobSpec.Steps))
	}
}

func TestTaskHandler_CreateTask_InputModePrompt_StoresPromptJobPayload(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{Prompt: "What is 2+2?", InputMode: "prompt"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	taskID, _ := uuid.Parse(resp.ResolveTaskID())
	jobs, _ := mockDB.GetJobsByTaskID(ctx, taskID)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	var pl struct {
		Image string            `json:"image"`
		Env   map[string]string `json:"env"`
	}
	if err := json.Unmarshal([]byte(*jobs[0].Payload.Ptr()), &pl); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if pl.Image != "python:alpine" {
		t.Errorf("expected image python:alpine, got %s", pl.Image)
	}
	if pl.Env["CYNODE_PROMPT"] != "What is 2+2?" {
		t.Errorf("expected CYNODE_PROMPT in env, got %v", pl.Env)
	}
}

func TestTaskHandler_CreateTask_PromptMode_OrchestratorInference_CreateJobCompletedFails_FallsBackToSandbox(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "ok", "done": true})
	}))
	defer mockOllama.Close()

	mockDB := &createJobCompletedErrorStore{MockDB: testutil.NewMockDB()}
	handler := NewTaskHandler(mockDB, newTestLogger(), mockOllama.URL, "qwen3.5:0.8b")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{Prompt: "hi", InputMode: InputModePrompt}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	taskID, _ := uuid.Parse(resp.ResolveTaskID())
	jobs, _ := mockDB.GetJobsByTaskID(ctx, taskID)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job (sandbox fallback), got %d", len(jobs))
	}
	if jobs[0].Status != models.JobStatusQueued {
		t.Errorf("expected job queued (sandbox), got %s", jobs[0].Status)
	}
}

func TestTaskHandler_CreateTask_PromptMode_OrchestratorInference(t *testing.T) {
	// Mock Ollama: return a valid generate response so prompt→model path completes.
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "I am qwen3.5:0.8b.", "done": true})
	}))
	defer mockOllama.Close()

	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), mockOllama.URL, "qwen3.5:0.8b")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{Prompt: "What model are you?", InputMode: InputModePrompt}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != userapi.StatusCompleted {
		t.Errorf("expected status completed (orchestrator inference), got %s", resp.Status)
	}
	taskID, _ := uuid.Parse(resp.ResolveTaskID())
	jobs, _ := mockDB.GetJobsByTaskID(ctx, taskID)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Status != models.JobStatusCompleted {
		t.Errorf("job status want completed got %s", jobs[0].Status)
	}
	if jobs[0].Result.Ptr() == nil {
		t.Fatal("job result empty")
	}
	var jobResult struct {
		Stdout string `json:"stdout"`
	}
	if err := json.Unmarshal([]byte(*jobs[0].Result.Ptr()), &jobResult); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if jobResult.Stdout != "I am qwen3.5:0.8b." {
		t.Errorf("stdout want 'I am qwen3.5:0.8b.' got %q", jobResult.Stdout)
	}
}

func TestTaskHandler_CreateTask_InputModeCommands_WithUseInference_StoresUseInferenceInPayload(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{Prompt: "echo hi", InputMode: "commands", UseInference: true}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	taskID, _ := uuid.Parse(resp.ResolveTaskID())
	jobs, _ := mockDB.GetJobsByTaskID(ctx, taskID)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	var pl struct {
		UseInference bool `json:"use_inference"`
	}
	if err := json.Unmarshal([]byte(*jobs[0].Payload.Ptr()), &pl); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if !pl.UseInference {
		t.Error("expected use_inference true in payload for commands+use_inference")
	}
}

func TestTaskHandler_CreateTask_InputModeCommands_StoresShellJobPayload(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{Prompt: "echo hello", InputMode: "commands"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var resp userapi.TaskResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	taskID, _ := uuid.Parse(resp.ResolveTaskID())
	jobs, _ := mockDB.GetJobsByTaskID(ctx, taskID)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	var pl struct {
		Command []string `json:"command"`
	}
	if err := json.Unmarshal([]byte(*jobs[0].Payload.Ptr()), &pl); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	cmdStr := strings.Join(pl.Command, " ")
	if !strings.Contains(cmdStr, "echo hello") {
		t.Errorf("expected command to contain 'echo hello', got %s", cmdStr)
	}
}

func TestTaskHandler_CreateTaskDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger, "", "")

	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := userapi.CreateTaskRequest{Prompt: testPrompt}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateTask(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}
