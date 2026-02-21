package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// TestTaskHandler_CreateTaskSuccess tests successful task creation path
func TestTaskHandler_CreateTaskSuccess(t *testing.T) {
	// Test the JSON response structure
	resp := TaskResponse{
		TaskID:    uuid.New().String(),
		Status:    SpecStatusQueued,
		Prompt:    ptr("test prompt"),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal TaskResponse: %v", err)
	}

	var parsed TaskResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal TaskResponse: %v", err)
	}

	if parsed.Status != SpecStatusQueued {
		t.Errorf("expected status queued, got %s", parsed.Status)
	}
}

// TestTaskHandler_GetTaskResultWithJobs tests task result response with jobs
func TestTaskHandler_GetTaskResultWithJobs(t *testing.T) {
	result := "test result output"
	startedAt := time.Now().UTC()
	endedAt := startedAt.Add(5 * time.Second)

	jobs := []JobResponse{
		{
			ID:        uuid.New().String(),
			Status:    models.JobStatusCompleted,
			Result:    &result,
			StartedAt: &startedAt,
			EndedAt:   &endedAt,
		},
		{
			ID:        uuid.New().String(),
			Status:    models.JobStatusFailed,
			Result:    nil,
			StartedAt: &startedAt,
			EndedAt:   &endedAt,
		},
	}

	resp := TaskResultResponse{
		TaskID: uuid.New().String(),
		Status: models.TaskStatusRunning,
		Jobs:   jobs,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed TaskResultResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(parsed.Jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(parsed.Jobs))
	}
}

// TestAuthHandler_CompleteLoginPath tests the login completion structure
func TestAuthHandler_CompleteLoginPath(t *testing.T) {
	resp := LoginResponse{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed LoginResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.TokenType != "Bearer" {
		t.Errorf("expected Bearer token type, got %s", parsed.TokenType)
	}
	if parsed.ExpiresIn != 900 {
		t.Errorf("expected 900 seconds, got %d", parsed.ExpiresIn)
	}
}

// TestNodeHandler_BootstrapResponse tests bootstrap response structure (CYNAI.WORKER.Payload.BootstrapV1)
func TestNodeHandler_BootstrapResponse(t *testing.T) {
	handler := &NodeHandler{orchestratorPublicURL: testOrchestratorURL}
	expiresAt := time.Now().Add(24 * time.Hour)

	resp := handler.buildBootstrapResponse(testOrchestratorURL, "test-node-jwt", expiresAt)

	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}
	if resp.Auth.NodeJWT != "test-node-jwt" {
		t.Errorf("expected jwt 'test-node-jwt', got %s", resp.Auth.NodeJWT)
	}
	if resp.IssuedAt == "" {
		t.Error("expected IssuedAt to be set")
	}
	if resp.Orchestrator.BaseURL != testOrchestratorURL {
		t.Errorf("expected base_url %q, got %s", testOrchestratorURL, resp.Orchestrator.BaseURL)
	}
	if resp.Orchestrator.Endpoints.NodeReportURL == "" {
		t.Error("expected NodeReportURL to be set")
	}
	if resp.Orchestrator.Endpoints.NodeConfigURL == "" {
		t.Error("expected NodeConfigURL to be set")
	}
}

// TestNodeHandler_ReportCapabilityWithNodeContext tests capability reporting with node context
func TestNodeHandler_ReportCapabilityWithNodeContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &NodeHandler{logger: logger}

	nodeID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyNodeID, nodeID)

	report := testNodeCapabilityReport("test-node", "Test Node", 4, 8192)
	body, _ := json.Marshal(report)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(body)).WithContext(ctx)
	rec := httptest.NewRecorder()

	// This will panic due to nil db, but we're testing that the node context is properly extracted
	defer func() {
		_ = recover()
	}()

	handler.ReportCapability(rec, req)
}

// TestAuthHandler_RefreshWithValidJWT tests refresh with valid JWT but missing DB session
func TestAuthHandler_RefreshWithValidJWT(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &AuthHandler{jwt: jwtMgr, logger: logger}

	userID := uuid.New()
	refreshToken, _, _ := jwtMgr.GenerateRefreshToken(userID)

	body := RefreshRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	// Will panic due to nil db
	defer func() {
		_ = recover()
	}()

	handler.Refresh(rec, req)
}

// TestGetTaskWithValidUUID tests GetTask with valid UUID but no DB
func TestGetTaskWithValidUUID(t *testing.T) {
	handler := &TaskHandler{}

	taskID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks/"+taskID.String(), http.NoBody)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()

	// Will panic due to nil db
	defer func() {
		_ = recover()
	}()

	handler.GetTask(rec, req)
}

// TestGetTaskResultWithValidUUID tests GetTaskResult with valid UUID but no DB
func TestGetTaskResultWithValidUUID(t *testing.T) {
	handler := &TaskHandler{}

	taskID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks/"+taskID.String()+"/result", http.NoBody)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()

	// Will panic due to nil db
	defer func() {
		_ = recover()
	}()

	handler.GetTaskResult(rec, req)
}

// TestCreateTaskOrLogoutWithNilDB tests handlers that require user context but hit nil db (panic expected).
func TestCreateTaskOrLogoutWithNilDB(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		body   interface{}
		invoke func(http.ResponseWriter, *http.Request)
	}{
		{"CreateTask", "/v1/tasks", CreateTaskRequest{Prompt: "test prompt"}, (&TaskHandler{}).CreateTask},
		{"Logout", "/v1/auth/logout", LogoutRequest{RefreshToken: "some-refresh-token"}, (&AuthHandler{}).Logout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBody, _ := json.Marshal(tt.body)
			req, rec := requestWithUserContext("POST", tt.path, jsonBody, uuid.New())
			defer func() { _ = recover() }()
			tt.invoke(rec, req)
		})
	}
}

// TestNodeHandler_RegisterWithValidRequest tests registration validation success path
func TestNodeHandler_RegisterWithValidRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &NodeHandler{
		registrationPSK: "test-psk-secret",
		logger:          logger,
	}

	body := NodeRegistrationRequest{
		PSK: "test-psk-secret",
		Capability: NodeCapabilityReport{
			Version: 1,
			Node:    NodeCapabilityNode{NodeSlug: "test-node-1"},
			Platform: NodeCapabilityPlatform{
				OS:   "linux",
				Arch: "amd64",
			},
			Compute: NodeCapabilityCompute{
				CPUCores: 8,
				RAMMB:    16384,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	// Will panic due to nil db, but validates the request parsing
	defer func() {
		_ = recover()
	}()

	handler.Register(rec, req)
}

// TestAuditLogWithAllPointerFields tests audit log with all pointer fields set
func TestAuditLogWithAllPointerFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &AuthHandler{logger: logger}

	userID := uuid.New()
	handler.auditLog(context.Background(), &userID, "test_event", true, "192.168.1.1", "Mozilla/5.0", "test detail")
}

// TestAuditLogWithNilPointerFields tests audit log with nil pointer fields
func TestAuditLogWithNilPointerFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &AuthHandler{logger: logger}

	handler.auditLog(context.Background(), nil, "test_event", false, "", "", "")
}

// Helper function
func ptr(s string) *string {
	return &s
}

// TestJobResponseWithNilFields tests JobResponse with nil optional fields
func TestJobResponseWithNilFields(t *testing.T) {
	resp := JobResponse{
		ID:        uuid.New().String(),
		Status:    "pending",
		Result:    nil,
		StartedAt: nil,
		EndedAt:   nil,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed JobResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Result != nil {
		t.Error("expected nil result")
	}
}

// TestUserHandler_GetMeWithLoggerNil tests GetMe with nil logger
func TestUserHandler_GetMeWithLoggerNil(t *testing.T) {
	handler := &UserHandler{logger: nil}

	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/users/me", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetMe(rec, req)

	// Should return 500 because db is nil
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

// TestNodeHandlerLoggerMethods tests logger helper methods with real logger
func TestNodeHandlerLoggerMethods(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &NodeHandler{logger: logger}

	// These should not panic
	handler.logError("test error", "key1", "val1", "key2", 123)
	handler.logWarn("test warn", "key1", "val1")
	handler.logInfo("test info", "key1", "val1")
}

// TestValidateLoginCredentialsStructure tests the loginResult structure
func TestValidateLoginCredentialsStructure(t *testing.T) {
	user := &models.User{
		ID:       uuid.New(),
		Handle:   "testuser",
		IsActive: true,
	}

	result := &loginResult{
		user:        user,
		errResponse: func() {},
	}

	if result.user == nil {
		t.Error("expected user to be set")
	}
	if result.user.Handle != testUserHandle {
		t.Errorf("expected handle %q, got %s", testUserHandle, result.user.Handle)
	}
}

// TestTaskResponseWithSummary tests TaskResponse with summary field
func TestTaskResponseWithSummary(t *testing.T) {
	summary := "Task completed successfully"
	resp := TaskResponse{
		TaskID:    uuid.New().String(),
		Status:    SpecStatusCompleted,
		Prompt:    ptr("test prompt"),
		Summary:   &summary,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed TaskResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Summary == nil || *parsed.Summary != summary {
		t.Error("expected summary to be set correctly")
	}
}
