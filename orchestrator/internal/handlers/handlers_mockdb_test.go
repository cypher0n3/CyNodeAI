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

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

const testPrompt = "test prompt"
const mimeSSE = "text/event-stream"

func newMockTask(createdBy *uuid.UUID, status string, prompt *string) *models.Task {
	now := time.Now().UTC()
	ps := models.PlanningStateReady
	if status == models.TaskStatusPending {
		ps = models.PlanningStateDraft
	}
	return &models.Task{
		TaskBase: models.TaskBase{
			CreatedBy:     createdBy,
			Status:        status,
			Prompt:        prompt,
			PlanningState: ps,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// postTaskReadyExpect calls POST /v1/tasks/{id}/ready and asserts the status code.
func postTaskReadyExpect(t *testing.T, handler *TaskHandler, ctx context.Context, taskID uuid.UUID, wantCode int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/"+taskID.String()+"/ready", http.NoBody).WithContext(ctx)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()
	handler.PostTaskReady(rec, req)
	if rec.Code != wantCode {
		t.Fatalf("PostTaskReady: want %d got %d: %s", wantCode, rec.Code, rec.Body.String())
	}
	return rec
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
	postTaskReadyExpect(t, handler, ctx, taskID, http.StatusOK)
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
	postTaskReadyExpect(t, handler, ctx, taskID, http.StatusOK)
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
	postTaskReadyExpect(t, handler, ctx, taskID, http.StatusOK)
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
	postTaskReadyExpect(t, handler, ctx, taskID, http.StatusOK)
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
	var created userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.Status != userapi.StatusQueued {
		t.Errorf("expected status queued after create, got %s", created.Status)
	}
	taskID, _ := uuid.Parse(created.ResolveTaskID())
	readyRec := postTaskReadyExpect(t, handler, ctx, taskID, http.StatusOK)
	var readyResp userapi.TaskResponse
	if err := json.Unmarshal(readyRec.Body.Bytes(), &readyResp); err != nil {
		t.Fatalf("unmarshal ready: %v", err)
	}
	if readyResp.Status != userapi.StatusCompleted {
		t.Errorf("expected status completed (orchestrator inference), got %s", readyResp.Status)
	}
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
	postTaskReadyExpect(t, handler, ctx, taskID, http.StatusOK)
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
	postTaskReadyExpect(t, handler, ctx, taskID, http.StatusOK)
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
// createJobErrorStore fails only CreateJob (CreateTask handler: task created, job creation fails).
type createJobErrorStore struct {
	*testutil.MockDB
}

func (m *createJobErrorStore) CreateJob(_ context.Context, _ uuid.UUID, _ string) (*models.Job, error) {
	return nil, errors.New("create job error")
}

// createJobWithIDErrorStore fails only CreateJobWithID (UseSBA path).
type createJobWithIDErrorStore struct {
	*testutil.MockDB
}

func (m *createJobWithIDErrorStore) CreateJobWithID(_ context.Context, _, _ uuid.UUID, _ string) (*models.Job, error) {
	return nil, errors.New("create job with id error")
}

// createJobCompletedErrorStore fails only CreateJobCompleted (orchestrator inference path).
type createJobCompletedErrorStore struct {
	*testutil.MockDB
}

func (m *createJobCompletedErrorStore) CreateJobCompleted(_ context.Context, _, _ uuid.UUID, _ string) (*models.Job, error) {
	return nil, errors.New("create job completed error")
}

func TestTaskHandler_CreateTask_UseSBA_CreateJobWithIDFails(t *testing.T) {
	mockDB := &createJobWithIDErrorStore{MockDB: testutil.NewMockDB()}
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req, rec := recordedRequestJSON("POST", "/v1/tasks", userapi.CreateTaskRequest{Prompt: "p", UseSBA: true})
	req = req.WithContext(ctx)
	handler.CreateTask(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
	var created userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	taskID, err := uuid.Parse(created.ResolveTaskID())
	if err != nil {
		t.Fatal(err)
	}
	postTaskReadyExpect(t, handler, ctx, taskID, http.StatusInternalServerError)
}

func TestTaskHandler_CreateTask_CreateJobFails(t *testing.T) {
	mockDB := &createJobErrorStore{MockDB: testutil.NewMockDB()}
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req, rec := recordedRequestJSON("POST", "/v1/tasks", userapi.CreateTaskRequest{Prompt: "p"})
	req = req.WithContext(ctx)
	handler.CreateTask(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
	var created userapi.TaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	taskID, err := uuid.Parse(created.ResolveTaskID())
	if err != nil {
		t.Fatal(err)
	}
	postTaskReadyExpect(t, handler, ctx, taskID, http.StatusInternalServerError)
}
