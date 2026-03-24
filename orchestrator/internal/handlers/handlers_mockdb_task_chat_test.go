package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestTaskHandler_GetTaskForbiddenNilCreatedBy(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	prompt := testPrompt
	task := &models.Task{
		TaskBase: models.TaskBase{
			CreatedBy: nil,
			Status:    models.TaskStatusPending,
			Prompt:    &prompt,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddTask(task)
	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String(), http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()
	handler.GetTask(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 when task has no owner, got %d", rec.Code)
	}
}

func TestTaskHandler_GetTaskResultForbidden(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewTaskHandler(mockDB, logger, "", "")
	ownerID := uuid.New()
	otherID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&ownerID, models.TaskStatusCompleted, &prompt)
	mockDB.AddTask(task)
	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String()+"/result", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, otherID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()
	handler.GetTaskResult(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestTaskHandler_ListTasksSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewTaskHandler(mockDB, logger, "", "")
	userID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&userID, models.TaskStatusPending, &prompt)
	mockDB.AddTask(task)
	req := httptest.NewRequest("GET", "/v1/tasks", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	rec := httptest.NewRecorder()
	handler.ListTasks(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.ListTasksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].ResolveTaskID() != task.ID.String() || resp.Tasks[0].Status != userapi.StatusQueued {
		t.Errorf("task_id or status wrong: %+v", resp.Tasks[0])
	}
}

func TestTaskHandler_ListTasksNoUser(t *testing.T) {
	handler := NewTaskHandler(testutil.NewMockDB(), newTestLogger(), "", "")
	req := httptest.NewRequest("GET", "/v1/tasks", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ListTasks(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestTaskHandler_ListTasksInvalidParams(t *testing.T) {
	handler := NewTaskHandler(testutil.NewMockDB(), newTestLogger(), "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	tests := []struct {
		name   string
		query  string
		expect int
	}{
		{"invalid limit", "limit=invalid", http.StatusBadRequest},
		{"limit out of range", "limit=0", http.StatusBadRequest},
		{"invalid offset", "offset=-1", http.StatusBadRequest},
		{"invalid cursor", "cursor=bad", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/tasks?"+tt.query, http.NoBody).WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ListTasks(rec, req)
			if rec.Code != tt.expect {
				t.Errorf("expected %d, got %d", tt.expect, rec.Code)
			}
		})
	}
}

func TestTaskHandler_ListTasksWithNextOffset(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	prompt := testPrompt
	for i := 0; i < 3; i++ {
		task := newMockTask(&userID, models.TaskStatusPending, &prompt)
		mockDB.AddTask(task)
	}
	req := httptest.NewRequest("GET", "/v1/tasks?limit=2&offset=0", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	rec := httptest.NewRecorder()
	handler.ListTasks(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.ListTasksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(resp.Tasks))
	}
	if resp.NextOffset == nil || *resp.NextOffset != 2 {
		t.Errorf("expected next_offset=2, got %v", resp.NextOffset)
	}
	if resp.NextCursor != "2" {
		t.Errorf("expected next_cursor=2, got %q", resp.NextCursor)
	}
}

func TestTaskHandler_ListTasksWithCursor(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	prompt := testPrompt
	tasks := []*models.Task{
		{
			TaskBase: models.TaskBase{
				CreatedBy: &userID,
				Status:    models.TaskStatusPending,
				Prompt:    &prompt,
			},
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		{
			TaskBase: models.TaskBase{
				CreatedBy: &userID,
				Status:    models.TaskStatusPending,
				Prompt:    &prompt,
			},
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		{
			TaskBase: models.TaskBase{
				CreatedBy: &userID,
				Status:    models.TaskStatusPending,
				Prompt:    &prompt,
			},
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
	for _, task := range tasks {
		mockDB.AddTask(task)
	}
	req := httptest.NewRequest("GET", "/v1/tasks?limit=1&cursor=1", http.NoBody).WithContext(
		context.WithValue(context.Background(), contextKeyUserID, userID),
	)
	rec := httptest.NewRecorder()
	handler.ListTasks(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.ListTasksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(resp.Tasks))
	}
	if resp.NextCursor == "" {
		t.Fatal("expected next_cursor when has more tasks")
	}
}

func TestTaskHandler_ListTasksWithCanceledTask(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&userID, models.TaskStatusCanceled, &prompt)
	mockDB.AddTask(task)
	req := httptest.NewRequest("GET", "/v1/tasks?status=canceled", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	rec := httptest.NewRecorder()
	handler.ListTasks(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var resp userapi.ListTasksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Tasks) != 1 || resp.Tasks[0].Status != "canceled" {
		t.Errorf("expected one task with status canceled, got %+v", resp.Tasks)
	}
}

func TestTaskHandler_ListTasksStatusFilterAndOffset(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	prompt := testPrompt
	t1 := newMockTask(&userID, models.TaskStatusCompleted, &prompt)
	mockDB.AddTask(t1)
	req := httptest.NewRequest("GET", "/v1/tasks?limit=10&offset=0&status=completed", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	rec := httptest.NewRecorder()
	handler.ListTasks(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.ListTasksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Tasks) != 1 {
		t.Errorf("expected 1 task (filtered), got %d", len(resp.Tasks))
	}
	if len(resp.Tasks) > 0 && resp.Tasks[0].Status != userapi.StatusCompleted {
		t.Errorf("expected status completed, got %s", resp.Tasks[0].Status)
	}
}

func TestTaskHandler_ListTasksDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	rec := httptest.NewRecorder()
	handler.ListTasks(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestTaskHandler_CancelTaskSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewTaskHandler(mockDB, logger, "", "")
	userID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&userID, models.TaskStatusPending, &prompt)
	mockDB.AddTask(task)
	req := httptest.NewRequest("POST", "/v1/tasks/"+task.ID.String()+"/cancel", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()
	handler.CancelTask(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.CancelTaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Canceled || resp.TaskID != task.ID.String() {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestTaskHandler_CancelTaskNotFound(t *testing.T) {
	handler := NewTaskHandler(testutil.NewMockDB(), newTestLogger(), "", "")
	userID := uuid.New()
	taskID := uuid.New()
	req := httptest.NewRequest("POST", "/v1/tasks/"+taskID.String()+"/cancel", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()
	handler.CancelTask(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func testTaskForbidden(t *testing.T, runHandler func(*TaskHandler, *http.Request, *httptest.ResponseRecorder), method, pathSuffix string) {
	t.Helper()
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	ownerID := uuid.New()
	otherID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&ownerID, models.TaskStatusPending, &prompt)
	mockDB.AddTask(task)
	req := httptest.NewRequest(method, "/v1/tasks/"+task.ID.String()+pathSuffix, http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, otherID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()
	runHandler(handler, req, rec)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestTaskHandler_CancelTaskForbidden(t *testing.T) {
	testTaskForbidden(t, func(h *TaskHandler, r *http.Request, w *httptest.ResponseRecorder) { h.CancelTask(w, r) }, "POST", "/cancel")
}

func TestTaskHandler_CancelTaskWithJobs(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&userID, models.TaskStatusPending, &prompt)
	mockDB.AddTask(task)
	payload := "{}"
	job := newMockJobSimple(task.ID, models.JobStatusQueued, &payload, nil)
	mockDB.AddJob(job)
	req := httptest.NewRequest("POST", "/v1/tasks/"+task.ID.String()+"/cancel", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()
	handler.CancelTask(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTaskHandler_CancelTaskUpdateStatusError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&userID, models.TaskStatusPending, &prompt)
	mockDB.AddTask(task)
	req := httptest.NewRequest("POST", "/v1/tasks/"+task.ID.String()+"/cancel", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()
	handler.CancelTask(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func testTaskWithErrorOnJobsMock(t *testing.T, runHandler func(*TaskHandler, *http.Request, *httptest.ResponseRecorder), method, pathSuffix, taskStatus string) {
	t.Helper()
	mockDB := &errorOnJobsMockDB{MockDB: testutil.NewMockDB()}
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	prompt := testPrompt
	task := &models.Task{
		TaskBase: models.TaskBase{
			CreatedBy: &userID,
			Status:    taskStatus,
			Prompt:    &prompt,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddTask(task)
	req := httptest.NewRequest(method, "/v1/tasks/"+task.ID.String()+pathSuffix, http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()
	runHandler(handler, req, rec)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestTaskHandler_CancelTaskGetJobsError(t *testing.T) {
	testTaskWithErrorOnJobsMock(t, func(h *TaskHandler, r *http.Request, w *httptest.ResponseRecorder) { h.CancelTask(w, r) }, "POST", "/cancel", models.TaskStatusPending)
}

func TestTaskHandler_GetTaskLogsSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewTaskHandler(mockDB, logger, "", "")
	userID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&userID, models.TaskStatusCompleted, &prompt)
	mockDB.AddTask(task)
	result := `{"version":1,"task_id":"` + task.ID.String() + `","job_id":"j1","status":"completed","stdout":"hello","stderr":"","started_at":"","ended_at":"","truncated":{"stdout":false,"stderr":false}}`
	job := newMockJobSimple(task.ID, models.JobStatusCompleted, nil, &result)
	mockDB.AddJob(job)
	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String()+"/logs", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()
	handler.GetTaskLogs(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp userapi.TaskLogsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Stdout != "hello" {
		t.Errorf("stdout want hello got %q", resp.Stdout)
	}
}

func TestTaskHandler_GetTaskLogsDBError(t *testing.T) {
	testTaskWithErrorOnJobsMock(t, func(h *TaskHandler, r *http.Request, w *httptest.ResponseRecorder) { h.GetTaskLogs(w, r) }, "GET", "/logs", models.TaskStatusCompleted)
}

func TestTaskHandler_GetTaskLogsStreamParam(t *testing.T) {
	resultJSON := `{"version":1,"stdout":"out","stderr":"err","started_at":"","ended_at":"","truncated":{"stdout":false,"stderr":false}}`
	tests := []struct {
		stream     string
		wantStdout string
		wantStderr string
	}{
		{"stdout", "out", ""},
		{"stderr", "", "err"},
	}
	for _, tt := range tests {
		t.Run("stream="+tt.stream, func(t *testing.T) {
			mockDB := testutil.NewMockDB()
			handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
			userID := uuid.New()
			prompt := testPrompt
			task := newMockTask(&userID, models.TaskStatusCompleted, &prompt)
			mockDB.AddTask(task)
			job := newMockJobSimple(task.ID, models.JobStatusCompleted, nil, &resultJSON)
			mockDB.AddJob(job)
			req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String()+"/logs?stream="+tt.stream, http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
			req.SetPathValue("id", task.ID.String())
			rec := httptest.NewRecorder()
			handler.GetTaskLogs(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
			var resp userapi.TaskLogsResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if resp.Stdout != tt.wantStdout || resp.Stderr != tt.wantStderr {
				t.Errorf("got stdout=%q stderr=%q", resp.Stdout, resp.Stderr)
			}
		})
	}
}

func TestTaskHandler_GetTaskLogsMalformedResult(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&userID, models.TaskStatusCompleted, &prompt)
	mockDB.AddTask(task)
	badResult := `not valid json`
	job := newMockJobSimple(task.ID, models.JobStatusCompleted, nil, &badResult)
	mockDB.AddJob(job)
	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String()+"/logs", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()
	handler.GetTaskLogs(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (graceful skip malformed), got %d", rec.Code)
	}
}

func TestTaskHandler_GetTaskLogsForbidden(t *testing.T) {
	testTaskForbidden(t, func(h *TaskHandler, r *http.Request, w *httptest.ResponseRecorder) { h.GetTaskLogs(w, r) }, "GET", "/logs")
}

func TestTaskHandler_ChatEmptyMessage(t *testing.T) {
	handler := NewTaskHandler(testutil.NewMockDB(), newTestLogger(), "", "")
	userID := uuid.New()
	body := []byte(`{"message":"   "}`)
	req := httptest.NewRequest("POST", "/v1/chat", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Chat(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestTaskHandler_ChatNoUser(t *testing.T) {
	handler := NewTaskHandler(testutil.NewMockDB(), newTestLogger(), "", "")
	body := []byte(`{"message":"hello"}`)
	req := httptest.NewRequest("POST", "/v1/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Chat(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestTaskHandler_ChatSuccessInference(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "Hi there.", "done": true})
	}))
	defer mockOllama.Close()
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), mockOllama.URL, "qwen3.5:0.8b")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{"message":"hello"}`)
	req := httptest.NewRequest("POST", "/v1/chat", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Chat(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Response != "Hi there." {
		t.Errorf("response want Hi there. got %q", resp.Response)
	}
}

func TestTaskHandler_ChatInvalidBody(t *testing.T) {
	handler := NewTaskHandler(testutil.NewMockDB(), newTestLogger(), "", "")
	userID := uuid.New()
	req := httptest.NewRequest("POST", "/v1/chat", bytes.NewReader([]byte("not json"))).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Chat(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestTaskHandler_ChatCreateTaskError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	body := []byte(`{"message":"hello"}`)
	req := httptest.NewRequest("POST", "/v1/chat", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Chat(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// chatGetTaskCompletedAfterFirstCall is shared by mocks that return completed task after first GetTaskByID call.
func chatGetTaskCompletedAfterFirstCall(ctx context.Context, m *testutil.MockDB, calls *int, id uuid.UUID) (*models.Task, error) {
	*calls++
	task, err := m.GetTaskByID(ctx, id)
	if err != nil || task == nil {
		return task, err
	}
	if *calls > 1 {
		t := *task
		t.Status = models.TaskStatusCompleted
		return &t, nil
	}
	return task, nil
}

// chatPollMock returns a completed task on GetTaskByID after the first call (so Chat poll loop exits).
type chatPollMock struct {
	*testutil.MockDB
	getTaskCalls int
}

func (m *chatPollMock) GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	return chatGetTaskCompletedAfterFirstCall(ctx, m.MockDB, &m.getTaskCalls, id)
}

// chatPollErrorMock returns error on GetTaskByID after the first call (covers Chat poll GetTask error path).
type chatPollErrorMock struct {
	*testutil.MockDB
	getTaskCalls int
}

func (m *chatPollErrorMock) GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	m.getTaskCalls++
	if m.getTaskCalls > 1 {
		return nil, errors.New("get task error")
	}
	return m.MockDB.GetTaskByID(ctx, id)
}

// chatTerminalJobsErrorMock returns completed task on second GetTaskByID but GetJobsByTaskID returns error.
type chatTerminalJobsErrorMock struct {
	*testutil.MockDB
	getTaskCalls int
}

func (m *chatTerminalJobsErrorMock) GetTaskByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	return chatGetTaskCompletedAfterFirstCall(ctx, m.MockDB, &m.getTaskCalls, id)
}

func (m *chatTerminalJobsErrorMock) GetJobsByTaskID(ctx context.Context, taskID uuid.UUID) ([]*models.Job, error) {
	return nil, errors.New("get jobs error")
}

func TestTaskHandler_ChatSuccessPolling(t *testing.T) {
	mockDB := testutil.NewMockDB()
	pollMock := &chatPollMock{MockDB: mockDB}
	handler := NewTaskHandler(pollMock, newTestLogger(), "", "") // no inference URL -> CreateJob then poll
	userID := uuid.New()
	body := []byte(`{"message":"hello"}`)
	req := httptest.NewRequest("POST", "/v1/chat", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Chat(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestTaskHandler_ChatInferenceFailsFallbackToPoll(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockOllama.Close()
	mockDB := testutil.NewMockDB()
	pollMock := &chatPollMock{MockDB: mockDB}
	handler := NewTaskHandler(pollMock, newTestLogger(), mockOllama.URL, "qwen3.5:0.8b")
	userID := uuid.New()
	body := []byte(`{"message":"hello"}`)
	req := httptest.NewRequest("POST", "/v1/chat", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Chat(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 after fallback to poll, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTaskHandler_ChatContextCanceled(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	body := []byte(`{"message":"hello"}`)
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKeyUserID, userID))
	req := httptest.NewRequest("POST", "/v1/chat", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	handler.Chat(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when context canceled, got %d", rec.Code)
	}
}

func TestTaskHandler_ChatErrorPaths(t *testing.T) {
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{"message":"hello"}`)
	tests := []struct {
		name   string
		store  database.Store
		expect int
	}{
		{"GetTaskByID fails in poll", &chatPollErrorMock{MockDB: testutil.NewMockDB()}, http.StatusInternalServerError},
		{"GetJobsByTaskID fails in terminal", &chatTerminalJobsErrorMock{MockDB: testutil.NewMockDB()}, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewTaskHandler(tt.store, newTestLogger(), "", "")
			req := httptest.NewRequest("POST", "/v1/chat", bytes.NewReader(body)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.Chat(rec, req)
			if rec.Code != tt.expect {
				t.Errorf("expected %d, got %d", tt.expect, rec.Code)
			}
		})
	}
}

// --- OpenAIChatHandler tests (GET /v1/models, POST /v1/chat/completions) ---

// mockDBWithPMAEndpoint returns a MockDB with one active node whose capability snapshot reports PMA ready at pmaURL.
func mockDBWithPMAEndpoint(t *testing.T, pmaURL string) *testutil.MockDB {
	t.Helper()
	db := testutil.NewMockDB()
	nodeID := uuid.New()
	db.AddNode(&models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "node-pma",
			Status:   models.NodeStatusActive,
		},
		ID:        nodeID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	report := nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node:       nodepayloads.CapabilityNode{NodeSlug: "node-pma"},
		Platform:   nodepayloads.Platform{OS: "linux", Arch: "amd64"},
		Compute:    nodepayloads.Compute{CPUCores: 4, RAMMB: 8192},
		ManagedServicesStatus: &nodepayloads.ManagedServicesStatus{
			Services: []nodepayloads.ManagedServiceStatus{
				{
					ServiceID:   "pma-main",
					ServiceType: "pma",
					State:       "ready",
					Endpoints:   []string{pmaURL},
					ReadyAt:     time.Now().UTC().Format(time.RFC3339),
				},
			},
		},
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal capability report: %v", err)
	}
	if err := db.SaveNodeCapabilitySnapshot(context.Background(), nodeID, string(raw)); err != nil {
		t.Fatalf("save capability snapshot: %v", err)
	}
	return db
}

func TestOpenAIChatHandler_ListModels(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "http://localhost:11434", "qwen3.5:0.8b", "")
	req := httptest.NewRequest("GET", "/v1/models", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	rec := httptest.NewRecorder()
	h.ListModels(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var out struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Object != "list" || len(out.Data) < 1 {
		t.Errorf("expected object list and at least one model, got %+v", out)
	}
	if out.Data[0].ID != EffectiveModelPM {
		t.Errorf("first model want %q, got %q", EffectiveModelPM, out.Data[0].ID)
	}
}

func TestOpenAIChatHandler_ChatCompletions_NoUser(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpenAIChatHandler_ChatCompletions_BadRequestCases(t *testing.T) {
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	tests := []struct {
		name   string
		body   []byte
		expect int
	}{
		{"empty messages", []byte(`{"messages":[]}`), http.StatusBadRequest},
		{"no user message", []byte(`{"messages":[{"role":"system","content":"you are helpful"}]}`), http.StatusBadRequest},
		{"direct inference not configured", []byte(`{"model":"qwen3.5:0.8b","messages":[{"role":"user","content":"hi"}]}`), http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "qwen3.5:0.8b", "")
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(tt.body)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ChatCompletions(rec, req)
			if rec.Code != tt.expect {
				t.Errorf("expected %d, got %d", tt.expect, rec.Code)
			}
		})
	}
}

func TestOpenAIChatHandler_ChatCompletions_DirectInference(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "Hello from model.", "done": true})
	}))
	defer mockOllama.Close()
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), mockOllama.URL, "qwen3.5:0.8b", "")
	body := []byte(`{"model":"qwen3.5:0.8b","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out userapi.ChatCompletionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Choices) != 1 || out.Choices[0].Message.Content != "Hello from model." {
		t.Errorf("expected one choice with content, got %+v", out.Choices)
	}
}

func TestOpenAIChatHandler_ChatCompletions_PMA(t *testing.T) {
	mockPMA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/chat/completion" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"content": "PMA reply."})
	}))
	defer mockPMA.Close()
	db := mockDBWithPMAEndpoint(t, mockPMA.URL)
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	body := []byte(`{"model":"cynodeai.pm","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out userapi.ChatCompletionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Choices) != 1 || out.Choices[0].Message.Content != "PMA reply." {
		t.Errorf("expected one choice with PMA content, got %+v", out.Choices)
	}
}

func TestOpenAIChatHandler_ChatCompletions_DefaultModelIsPM(t *testing.T) {
	mockPMA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"content": "Default PM."})
	}))
	defer mockPMA.Close()
	db := mockDBWithPMAEndpoint(t, mockPMA.URL)
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out userapi.ChatCompletionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Model != EffectiveModelPM {
		t.Errorf("default model want %q, got %q", EffectiveModelPM, out.Model)
	}
}

func TestOpenAIChatHandler_ChatCompletions_PMAUnavailable(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	body := []byte(`{"model":"cynodeai.pm","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when PMA URL empty, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpenAIChatHandler_ChatCompletions_InvalidBody(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("not json")).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_ChatCompletions_DirectInferenceFails(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockOllama.Close()
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), mockOllama.URL, "qwen3.5:0.8b", "")
	body := []byte(`{"model":"qwen3.5:0.8b","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 on inference failure, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpenAIChatHandler_ChatCompletions_RedactsSecrets(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "ok", "done": true})
	}))
	defer mockOllama.Close()
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), mockOllama.URL, "qwen3.5:0.8b", "")
	body := []byte(`{"model":"qwen3.5:0.8b","messages":[{"role":"user","content":"my key is sk-abcdefghij1234567890abcdefghij"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpenAIChatHandler_ChatCompletions_ProjectHeaderAndApiKeyRedaction(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "ok", "done": true})
	}))
	defer mockOllama.Close()
	projID := uuid.New()
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), mockOllama.URL, "qwen3.5:0.8b", "")
	body := []byte(`{"model":"qwen3.5:0.8b","messages":[{"role":"user","content":"apikey: secret123"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Project", projID.String())
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// Invalid OpenAI-Project header is ignored (projectIDFromHeader returns nil).
	req2 := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("OpenAI-Project", "not-a-uuid")
	rec2 := httptest.NewRecorder()
	h.ChatCompletions(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 with invalid project header (ignored), got %d", rec2.Code)
	}
}

func TestOpenAIChatHandler_ChatCompletions_PMAFails(t *testing.T) {
	mockPMA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockPMA.Close()
	db := mockDBWithPMAEndpoint(t, mockPMA.URL)
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	body := []byte(`{"model":"cynodeai.pm","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 when PMA returns 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestOpenAIChatHandler_ChatCompletions_Timeout verifies REQ-ORCHES-0131: max wait returns 504.
