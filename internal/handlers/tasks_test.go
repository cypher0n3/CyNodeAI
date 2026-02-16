package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	// Test invalid JSON
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBufferString("{invalid"))
	rec := httptest.NewRecorder()

	handler.CreateTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestCreateTaskEmptyPrompt(t *testing.T) {
	handler := &TaskHandler{}

	body := CreateTaskRequest{Prompt: ""}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/tasks", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.CreateTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestGetTaskInvalidID(t *testing.T) {
	handler := &TaskHandler{}

	req := httptest.NewRequest("GET", "/v1/tasks/invalid-uuid", http.NoBody)
	req.SetPathValue("id", "invalid-uuid")
	rec := httptest.NewRecorder()

	handler.GetTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestGetTaskResultInvalidID(t *testing.T) {
	handler := &TaskHandler{}

	req := httptest.NewRequest("GET", "/v1/tasks/invalid-uuid/result", http.NoBody)
	req.SetPathValue("id", "invalid-uuid")
	rec := httptest.NewRecorder()

	handler.GetTaskResult(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestTaskResponseJSON(t *testing.T) {
	prompt := "test prompt"
	resp := TaskResponse{
		ID:     "test-id",
		Status: "pending",
		Prompt: &prompt,
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed TaskResponse
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %s", parsed.ID)
	}
}

func TestJobResponseJSON(t *testing.T) {
	result := "test result"
	resp := JobResponse{
		ID:     "job-id",
		Status: "completed",
		Result: &result,
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed JobResponse
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Status != "completed" {
		t.Errorf("expected status 'completed', got %s", parsed.Status)
	}
}

func TestTaskResultResponseJSON(t *testing.T) {
	resp := TaskResultResponse{
		TaskID: "task-id",
		Status: "running",
		Jobs:   []JobResponse{},
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed TaskResultResponse
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.TaskID != "task-id" {
		t.Errorf("expected task ID 'task-id', got %s", parsed.TaskID)
	}
}

func TestCreateTaskRequestJSON(t *testing.T) {
	req := CreateTaskRequest{Prompt: "test prompt"}

	jsonData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed CreateTaskRequest
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Prompt != "test prompt" {
		t.Errorf("expected prompt 'test prompt', got %s", parsed.Prompt)
	}
}
