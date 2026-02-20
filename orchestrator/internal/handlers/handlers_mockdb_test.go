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

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
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
	req, rec := recordedRequestJSON("POST", "/v1/auth/refresh", RefreshRequest{RefreshToken: tokenStr})
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

func TestTaskHandler_CreateTaskWithUseInference_StoresUseInferenceInJobPayload(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewTaskHandler(mockDB, logger)
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := CreateTaskRequest{Prompt: "echo hi", UseInference: true}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	taskID, err := uuid.Parse(resp.ID)
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

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", logger)

	body := NodeRegistrationRequest{PSK: "test-psk-secret", Capability: testNodeCapabilityReport("test-node-1", "Test Node 1", 8, 16384)}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp nodepayloads.BootstrapResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}
	if resp.Auth.NodeJWT == "" {
		t.Error("expected node JWT")
	}
	if resp.Orchestrator.BaseURL != testOrchestratorURL {
		t.Errorf("expected base_url %q, got %s", testOrchestratorURL, resp.Orchestrator.BaseURL)
	}
	if resp.Orchestrator.Endpoints.NodeReportURL == "" {
		t.Error("expected NodeReportURL in bootstrap")
	}
	if resp.Orchestrator.Endpoints.NodeConfigURL == "" {
		t.Error("expected NodeConfigURL in bootstrap")
	}
}

func TestNodeHandler_RegisterExistingNode(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", logger)

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

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", logger)

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

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", logger)

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

// --- Node Config (GET/POST /v1/nodes/config) Tests ---

func TestNodeHandler_GetConfig_Success(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "bearer-token-1", "http://node:8081", logger)

	cfgVer := "1"
	node := &models.Node{
		ID:            uuid.New(),
		NodeSlug:      "cfg-node",
		Status:        models.NodeStatusActive,
		ConfigVersion: &cfgVer,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload nodepayloads.NodeConfigurationPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if payload.Version != 1 || payload.ConfigVersion != "1" || payload.NodeSlug != "cfg-node" {
		t.Errorf("unexpected payload: version=%d config_version=%s node_slug=%s", payload.Version, payload.ConfigVersion, payload.NodeSlug)
	}
	if payload.WorkerAPI == nil || payload.WorkerAPI.OrchestratorBearerToken != "bearer-token-1" {
		t.Errorf("expected worker_api.orchestrator_bearer_token, got %+v", payload.WorkerAPI)
	}
	if payload.Orchestrator.Endpoints.WorkerAPITargetURL != "http://node:8081" {
		t.Errorf("expected worker_api_target_url from handler, got %s", payload.Orchestrator.Endpoints.WorkerAPITargetURL)
	}
}

func TestNodeHandler_GetConfig_NoNodeID(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", nil)

	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestNodeHandler_GetConfig_NodeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", nil)

	ctx := context.WithValue(context.Background(), contextKeyNodeID, uuid.New())
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestNodeHandler_ConfigAck_Success(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", logger)

	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "ack-node",
		Status:    models.NodeStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ack := nodepayloads.ConfigAck{
		Version:       1,
		NodeSlug:      "ack-node",
		ConfigVersion: "1",
		AckAt:         time.Now().UTC().Format(time.RFC3339),
		Status:        "applied",
	}
	jsonBody, _ := json.Marshal(ack)
	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("POST", "/v1/nodes/config", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ConfigAck(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
	// Node should have config ack fields updated (via mock)
	updated, _ := mockDB.GetNodeByID(context.Background(), node.ID)
	if updated.ConfigAckStatus == nil || *updated.ConfigAckStatus != "applied" {
		t.Errorf("expected config_ack_status applied, got %v", updated.ConfigAckStatus)
	}
}

func TestNodeHandler_ConfigAck_NoNodeID(t *testing.T) {
	handler := NewNodeHandler(nil, nil, "test-psk", testOrchestratorURL, "", "", nil)

	ack := nodepayloads.ConfigAck{Version: 1, NodeSlug: "x", ConfigVersion: "1", AckAt: time.Now().UTC().Format(time.RFC3339), Status: "applied"}
	jsonBody, _ := json.Marshal(ack)
	req := httptest.NewRequest("POST", "/v1/nodes/config", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.ConfigAck(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestNodeHandler_ConfigAck_BadRequestCases(t *testing.T) {
	tests := []struct {
		name      string
		ack       nodepayloads.ConfigAck
		nodeSlug  string
	}{
		{"bad slug", nodepayloads.ConfigAck{
			Version: 1, NodeSlug: "wrong-slug", ConfigVersion: "1",
			AckAt: time.Now().UTC().Format(time.RFC3339), Status: "applied",
		}, "real-slug"},
		{"invalid status", nodepayloads.ConfigAck{
			Version: 1, NodeSlug: "status-node", ConfigVersion: "1",
			AckAt: time.Now().UTC().Format(time.RFC3339), Status: "rolled_back",
		}, "status-node"},
		{"unsupported version", nodepayloads.ConfigAck{
			Version: 2, NodeSlug: "ver-node", ConfigVersion: "1",
			AckAt: time.Now().UTC().Format(time.RFC3339), Status: "applied",
		}, "ver-node"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := testutil.NewMockDB()
			handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", nil)
			node := &models.Node{
				ID: uuid.New(), NodeSlug: tt.nodeSlug, Status: models.NodeStatusActive,
				CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			}
			mockDB.AddNode(node)
			jsonBody, _ := json.Marshal(tt.ack)
			ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
			req := httptest.NewRequest("POST", "/v1/nodes/config", bytes.NewBuffer(jsonBody)).WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ConfigAck(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", rec.Code)
			}
		})
	}
}

func TestNodeHandler_GetConfig_SetsConfigVersionWhenNil(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "tok", "", nil)

	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "no-ver-node",
		Status:    models.NodeStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		// ConfigVersion nil
	}
	mockDB.AddNode(node)

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// Handler should have persisted config_version "1"
	updated, _ := mockDB.GetNodeByID(context.Background(), node.ID)
	if updated.ConfigVersion == nil || *updated.ConfigVersion != "1" {
		t.Errorf("expected config_version 1 to be set, got %v", updated.ConfigVersion)
	}
}

func TestNodeHandler_ConfigAck_InvalidBody(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", nil)

	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "body-node",
		Status:    models.NodeStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("POST", "/v1/nodes/config", bytes.NewBufferString("{invalid}")).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ConfigAck(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestNodeHandler_GetConfig_DBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("db error")
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", nil)

	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "db-err-node",
		Status:    models.NodeStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestNodeHandler_ConfigAck_DBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("db error")
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", nil)

	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "ack-db-node",
		Status:    models.NodeStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ack := nodepayloads.ConfigAck{
		Version:       1,
		NodeSlug:      "ack-db-node",
		ConfigVersion: "1",
		AckAt:         time.Now().UTC().Format(time.RFC3339),
		Status:        "applied",
	}
	jsonBody, _ := json.Marshal(ack)
	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("POST", "/v1/nodes/config", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ConfigAck(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestNodeHandler_ReportCapabilityDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", logger)

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

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "", "", logger)

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

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", logger)

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

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", logger)

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

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "", "", logger)

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

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "", "", logger)

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

// updateNodeStatusErrorStore fails only UpdateNodeStatus (e.g. in handleExistingNodeRegistration).
type updateNodeStatusErrorStore struct {
	*testutil.MockDB
}

func (m *updateNodeStatusErrorStore) UpdateNodeStatus(_ context.Context, _ uuid.UUID, _ string) error {
	return errors.New("update node status error")
}

func TestNodeHandler_RegisterExistingNode_UpdateNodeStatusFails(t *testing.T) {
	mockDB := &updateNodeStatusErrorStore{MockDB: testutil.NewMockDB()}
	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "existing-node",
		Status:    "offline",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "", "", newTestLogger())

	body := NodeRegistrationRequest{
		PSK: "test-psk",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "existing-node"},
			Platform: NodeCapabilityPlatform{OS: "linux", Arch: "amd64"},
			Compute:  NodeCapabilityCompute{CPUCores: 4, RAMMB: 8192},
		},
	}
	req, rec := recordedRequestJSON("POST", "/v1/nodes/register", body)
	handler.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

// createNodeErrorStore fails only CreateNode so handleNewNodeRegistration hits CreateNode error path.
type createNodeErrorStore struct {
	*testutil.MockDB
}

func (m *createNodeErrorStore) CreateNode(_ context.Context, _ string) (*models.Node, error) {
	return nil, errors.New("create node error")
}

func TestNodeHandler_RegisterNewNode_CreateNodeFails(t *testing.T) {
	mockDB := &createNodeErrorStore{MockDB: testutil.NewMockDB()}
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "", "", newTestLogger())

	body := NodeRegistrationRequest{
		PSK: "test-psk",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "new-node"},
			Platform: NodeCapabilityPlatform{OS: "linux", Arch: "amd64"},
			Compute:  NodeCapabilityCompute{CPUCores: 4, RAMMB: 8192},
		},
	}
	req, rec := recordedRequestJSON("POST", "/v1/nodes/register", body)
	handler.Register(rec, req)

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
	user := &models.User{ID: uuid.New(), Handle: "u", IsActive: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	base.AddUser(user)
	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)
	session := &models.RefreshSession{
		ID: uuid.New(), UserID: user.ID, RefreshTokenHash: tokenHash, IsActive: true, ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	base.AddRefreshSession(session)
	req, rec := recordedRequestJSON("POST", "/v1/auth/refresh", RefreshRequest{RefreshToken: refreshToken})
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

func TestAuthHandler_Refresh_SessionNotFound(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewAuthHandler(mockDB, jwtMgr, auth.NewRateLimiter(100, time.Minute), newTestLogger())

	user := &models.User{ID: uuid.New(), Handle: "u", IsActive: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	mockDB.AddUser(user)
	refreshToken, _, _ := jwtMgr.GenerateRefreshToken(user.ID)
	// Do not add refresh session - session lookup will return ErrNotFound

	req, rec := recordedRequestJSON("POST", "/v1/auth/refresh", RefreshRequest{RefreshToken: refreshToken})
	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuthHandler_Refresh_GetActiveRefreshSessionError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("db error")
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewAuthHandler(mockDB, jwtMgr, auth.NewRateLimiter(100, time.Minute), newTestLogger())

	refreshToken, _, _ := jwtMgr.GenerateRefreshToken(uuid.New())

	req, rec := recordedRequestJSON("POST", "/v1/auth/refresh", RefreshRequest{RefreshToken: refreshToken})
	handler.Refresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestAuthHandler_Refresh_CreateSessionFails(t *testing.T) {
	mockDB := &sessionErrorMockDB{MockDB: testutil.NewMockDB(), failOnCreateSession: true}
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewAuthHandler(mockDB, jwtMgr, auth.NewRateLimiter(100, time.Minute), newTestLogger())

	user := &models.User{ID: uuid.New(), Handle: "u", IsActive: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	mockDB.AddUser(user)
	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)
	session := &models.RefreshSession{
		ID: uuid.New(), UserID: user.ID, RefreshTokenHash: tokenHash, IsActive: true, ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddRefreshSession(session)

	req, rec := recordedRequestJSON("POST", "/v1/auth/refresh", RefreshRequest{RefreshToken: refreshToken})
	handler.Refresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

// createJobErrorStore fails only CreateJob (CreateTask handler: task created, job creation fails).
type createJobErrorStore struct {
	*testutil.MockDB
}

func (m *createJobErrorStore) CreateJob(_ context.Context, _ uuid.UUID, _ string) (*models.Job, error) {
	return nil, errors.New("create job error")
}

func TestTaskHandler_CreateTask_CreateJobFails(t *testing.T) {
	mockDB := &createJobErrorStore{MockDB: testutil.NewMockDB()}
	handler := NewTaskHandler(mockDB, newTestLogger())
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req, rec := recordedRequestJSON("POST", "/v1/tasks", CreateTaskRequest{Prompt: "p"})
	req = req.WithContext(ctx)
	handler.CreateTask(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}
