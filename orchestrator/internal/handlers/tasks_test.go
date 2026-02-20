package handlers

import (
	"net/http"
	"testing"
)

func TestNewTaskHandler(t *testing.T) {
	handler := NewTaskHandler(nil, nil)
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
	req, rec := recordedRequestJSON("POST", "/v1/tasks", CreateTaskRequest{Prompt: ""})
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
	var parsed TaskResponse
	roundTripJSON(t, TaskResponse{ID: "test-id", Status: "pending", Prompt: &prompt}, &parsed)
	if parsed.ID != "test-id" || parsed.Status != "pending" {
		t.Errorf("got ID %q status %q", parsed.ID, parsed.Status)
	}
}

func TestJobResponseJSON(t *testing.T) {
	result := "test result"
	var parsed JobResponse
	roundTripJSON(t, JobResponse{ID: "job-id", Status: "completed", Result: &result}, &parsed)
	if parsed.Status != "completed" {
		t.Errorf("expected status 'completed', got %s", parsed.Status)
	}
}

func TestTaskResultResponseJSON(t *testing.T) {
	var parsed TaskResultResponse
	roundTripJSON(t, TaskResultResponse{TaskID: "task-id", Status: "running", Jobs: []JobResponse{}}, &parsed)
	if parsed.TaskID != "task-id" || parsed.Status != "running" {
		t.Errorf("got TaskID %q Status %q", parsed.TaskID, parsed.Status)
	}
}

func TestCreateTaskRequestJSON(t *testing.T) {
	var parsed CreateTaskRequest
	roundTripJSON(t, CreateTaskRequest{Prompt: "test prompt"}, &parsed)
	if parsed.Prompt != "test prompt" {
		t.Errorf("expected prompt 'test prompt', got %s", parsed.Prompt)
	}
	roundTripJSON(t, CreateTaskRequest{Prompt: "x", UseInference: true}, &parsed)
	if parsed.Prompt != "x" || !parsed.UseInference {
		t.Errorf("expected prompt 'x' UseInference true, got %q %v", parsed.Prompt, parsed.UseInference)
	}
}
