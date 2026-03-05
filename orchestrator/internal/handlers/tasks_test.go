package handlers

import (
	"context"
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
