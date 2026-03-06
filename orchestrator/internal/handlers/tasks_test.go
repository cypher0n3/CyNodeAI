package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestNewTaskHandler(t *testing.T) {
	handler := NewTaskHandler(nil, nil, "", "")
	if handler == nil {
		t.Fatal("NewTaskHandler returned nil")
	}
}

func TestCreateTaskBadRequest(t *testing.T) {
	handler := &TaskHandler{}
	runHandlerTest(t, "POST", "/v1/tasks", []byte("{invalid"), handler.CreateTask, http.StatusBadRequest)
}

func TestCreateTaskEmptyPrompt(t *testing.T) {
	handler := &TaskHandler{}
	req, rec := recordedRequestJSON("POST", "/v1/tasks", userapi.CreateTaskRequest{Prompt: ""})
	handler.CreateTask(rec, req)
	assertStatusCode(t, rec, http.StatusBadRequest)
}

func TestGetTaskOrResultInvalidID(t *testing.T) {
	handler := &TaskHandler{}
	tests := []struct {
		name   string
		path   string
		handle func(http.ResponseWriter, *http.Request)
	}{
		{"GetTask", "/v1/tasks/invalid-uuid", handler.GetTask},
		{"GetTaskResult", "/v1/tasks/invalid-uuid/result", handler.GetTaskResult},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, rec := recordedRequest("GET", tt.path, nil)
			req.SetPathValue("id", "invalid-uuid")
			tt.handle(rec, req)
			assertStatusCode(t, rec, http.StatusBadRequest)
		})
	}
}

func TestTaskResponseJSON(t *testing.T) {
	prompt := "test prompt"
	var parsed userapi.TaskResponse
	roundTripJSON(t, userapi.TaskResponse{TaskID: "test-id", Status: userapi.StatusQueued, Prompt: &prompt}, &parsed)
	if parsed.ResolveTaskID() != "test-id" || parsed.Status != userapi.StatusQueued {
		t.Errorf("got TaskID %q status %q", parsed.ResolveTaskID(), parsed.Status)
	}
}

func TestJobResponseJSON(t *testing.T) {
	result := "test result"
	var parsed userapi.JobResponse
	roundTripJSON(t, userapi.JobResponse{ID: "job-id", Status: "completed", Result: &result}, &parsed)
	if parsed.Status != userapi.StatusCompleted {
		t.Errorf("expected status 'completed', got %s", parsed.Status)
	}
}

func TestTaskResultResponseJSON(t *testing.T) {
	var parsed userapi.TaskResultResponse
	roundTripJSON(t, userapi.TaskResultResponse{TaskID: "task-id", Status: "running", Jobs: []userapi.JobResponse{}}, &parsed)
	if parsed.TaskID != "task-id" || parsed.Status != "running" {
		t.Errorf("got TaskID %q Status %q", parsed.TaskID, parsed.Status)
	}
}

func TestCreateTaskRequestJSON(t *testing.T) {
	var parsed userapi.CreateTaskRequest
	roundTripJSON(t, userapi.CreateTaskRequest{Prompt: "test prompt"}, &parsed)
	if parsed.Prompt != "test prompt" {
		t.Errorf("expected prompt 'test prompt', got %s", parsed.Prompt)
	}
	roundTripJSON(t, userapi.CreateTaskRequest{Prompt: "x", UseInference: true}, &parsed)
	if parsed.Prompt != "x" || !parsed.UseInference {
		t.Errorf("expected prompt 'x' UseInference true, got %q %v", parsed.Prompt, parsed.UseInference)
	}
	roundTripJSON(t, userapi.CreateTaskRequest{Prompt: "p", UseSBA: true}, &parsed)
	if parsed.Prompt != "p" || !parsed.UseSBA {
		t.Errorf("expected prompt 'p' UseSBA true, got %q %v", parsed.Prompt, parsed.UseSBA)
	}
	projectID := uuid.New().String()
	roundTripJSON(t, userapi.CreateTaskRequest{Prompt: "p", ProjectID: &projectID}, &parsed)
	if parsed.ProjectID == nil || *parsed.ProjectID != projectID {
		t.Errorf("expected project_id %q, got %v", projectID, parsed.ProjectID)
	}
}

func TestTaskStatusToSpec(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: models.TaskStatusPending, want: userapi.StatusQueued},
		{in: models.TaskStatusCancelled, want: userapi.StatusCancelled},
		{in: models.TaskStatusSuperseded, want: userapi.StatusSuperseded},
		{in: models.TaskStatusCompleted, want: models.TaskStatusCompleted},
	}
	for _, tt := range tests {
		if got := taskStatusToSpec(tt.in); got != tt.want {
			t.Errorf("taskStatusToSpec(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveTaskProjectID(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, slog.Default(), "", "")
	userID := uuid.New()

	reqProjectID := uuid.New().String()
	projectID, err := handler.resolveTaskProjectID(context.Background(), &userID, &reqProjectID)
	if err != nil {
		t.Fatalf("resolveTaskProjectID explicit: %v", err)
	}
	if projectID == nil || projectID.String() != reqProjectID {
		t.Fatalf("resolved project_id = %v, want %s", projectID, reqProjectID)
	}

	bad := "bad-id"
	_, err = handler.resolveTaskProjectID(context.Background(), &userID, &bad)
	if !errors.Is(err, errInvalidProjectID) {
		t.Fatalf("expected errInvalidProjectID, got %v", err)
	}

	projectID, err = handler.resolveTaskProjectID(context.Background(), &userID, nil)
	if err != nil {
		t.Fatalf("resolveTaskProjectID default: %v", err)
	}
	if projectID == nil {
		t.Fatal("expected default project id")
	}

	projectID, err = handler.resolveTaskProjectID(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("resolveTaskProjectID nil user: %v", err)
	}
	if projectID != nil {
		t.Fatalf("expected nil project id for nil user, got %v", projectID)
	}
}

func TestParseListTasksParams_StatusAliasCanceled(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks?status=canceled", http.NoBody)
	_, _, status, _, errCode := parseListTasksParams(req)
	if errCode != 0 {
		t.Fatalf("unexpected errCode=%d", errCode)
	}
	if status != userapi.StatusCancelled {
		t.Fatalf("status=%q, want %q", status, userapi.StatusCancelled)
	}
}

func TestParseListTasksParams_CursorRoundTrip(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks?cursor=20", http.NoBody)
	_, _, _, cursor, errCode := parseListTasksParams(req)
	if errCode != 0 {
		t.Fatalf("unexpected errCode=%d", errCode)
	}
	if cursor != "20" {
		t.Fatalf("cursor=%q", cursor)
	}
}

func TestPersistTaskAttachments_FilterAndStore(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, slog.Default(), "", "")
	taskID := uuid.New()
	tooLong := strings.Repeat("x", maxAttachmentPathLen+1)
	in := []string{"", "  ", tooLong, "valid/a.txt"}
	stored := handler.persistTaskAttachments(context.Background(), taskID, in)
	if len(stored) != 1 || stored[0] != "valid/a.txt" {
		t.Fatalf("stored=%v", stored)
	}
}

func TestPersistTaskAttachments_CreateArtifactError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("db error")
	handler := NewTaskHandler(mockDB, slog.Default(), "", "")
	stored := handler.persistTaskAttachments(context.Background(), uuid.New(), []string{"valid/a.txt"})
	if len(stored) != 0 {
		t.Fatalf("expected no stored paths on artifact error, got %v", stored)
	}
}

func TestBuildSBAJobPayload(t *testing.T) {
	taskID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	jobID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	payload, err := buildSBAJobPayload(taskID, jobID, "test prompt", "tinyllama")
	if err != nil {
		t.Fatalf("buildSBAJobPayload: %v", err)
	}
	if payload == "" {
		t.Fatal("expected non-empty payload")
	}
	if !strings.Contains(payload, "job_spec_json") ||
		!strings.Contains(payload, "1.0") ||
		!strings.Contains(payload, sbajob.ExecutionModeAgentInference) ||
		!strings.Contains(payload, "allowed_models") {
		t.Errorf("payload missing expected SBA fields: %s", payload)
	}
}

func TestBuildSBAJobPayload_RequiresInferenceModel(t *testing.T) {
	taskID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	jobID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	_, err := buildSBAJobPayload(taskID, jobID, "test prompt", "")
	if err == nil {
		t.Fatal("expected inference readiness error")
	}
}

func TestCreateTaskSBA_InferenceReadinessFailureReturnsBadRequest(t *testing.T) {
	handler := &TaskHandler{
		db:             testutil.NewMockDB(),
		logger:         slog.Default(),
		inferenceModel: "",
	}
	task := &models.Task{
		ID:     uuid.New(),
		Status: models.TaskStatusPending,
	}
	rec := httptest.NewRecorder()
	handled := handler.createTaskSBA(context.Background(), rec, task, "prompt", nil)
	if !handled {
		t.Fatal("expected createTaskSBA to handle response")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
