package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

const testProjectID = "proj-1"
const testTaskID = "tid-1"
const pathV1ChatThreads = "/v1/chat/threads"
const pathV1ChatCompletions = "/v1/chat/completions"

var testJobResultHello = "hello\n"

// threadsAPIHandler returns a handler that responds with status and body when path matches (for thread API tests).
func threadsAPIHandler(path string, status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestClient_ListTasks_WithLimitOffset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.URL.Query().Get("cursor"); got != "2" {
			t.Errorf("cursor query = %q, want 2", got)
		}
		jsonHandler(http.StatusOK, userapi.ListTasksResponse{Tasks: []userapi.TaskResponse{}})(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.ListTasks(context.Background(), ListTasksRequest{Limit: 5, Offset: 10, Cursor: "2"})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
}

func jsonHandler(status int, v any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
}

func rawHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestClient_Login(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/login" || r.Method != http.MethodPost {
			t.Errorf("path=%q method=%q", r.URL.Path, r.Method)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req userapi.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if req.Handle != "u" || req.Password != "p" {
			http.Error(w, "bad creds", http.StatusUnauthorized)
			return
		}
		jsonHandler(http.StatusOK, userapi.LoginResponse{AccessToken: "tok", TokenType: "Bearer", ExpiresIn: 900})(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.Login(context.Background(), userapi.LoginRequest{Handle: "u", Password: "p"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.AccessToken != "tok" {
		t.Errorf("AccessToken = %q, want tok", resp.AccessToken)
	}
}

func TestClient_Login_Unauthorized(t *testing.T) {
	server := httptest.NewServer(jsonHandler(http.StatusUnauthorized, problem.Details{
		Detail: "Invalid credentials", Status: 401,
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Login(context.Background(), userapi.LoginRequest{Handle: "u", Password: "wrong"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() == "" {
		t.Error("error message should contain detail")
	}
}

func TestClient_Health(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestClient_CreateTask_RequiresAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			jsonHandler(http.StatusUnauthorized, problem.Details{Detail: "Not authenticated", Status: 401})(w, r)
			return
		}
		jsonHandler(http.StatusCreated, userapi.TaskResponse{ID: "tid", Status: "queued"})(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.CreateTask(context.Background(), &userapi.CreateTaskRequest{Prompt: "echo hi"})
	if err == nil {
		t.Fatal("expected error when no token")
	}

	client.SetToken("tok")
	resp, err := client.CreateTask(context.Background(), &userapi.CreateTaskRequest{Prompt: "echo hi"})
	if err != nil {
		t.Fatalf("CreateTask with token: %v", err)
	}
	if resp.ID != "tid" {
		t.Errorf("ID = %q, want tid", resp.ID)
	}
}

func TestClient_GetTaskResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks/tid-123/result" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		jsonHandler(http.StatusOK, userapi.TaskResultResponse{
			TaskID: "tid-123", Status: "completed",
			Jobs: []userapi.JobResponse{{ID: "j1", Status: "completed", Result: &testJobResultHello}},
		})(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.GetTaskResult(context.Background(), "tid-123")
	if err != nil {
		t.Fatalf("GetTaskResult: %v", err)
	}
	if resp.TaskID != "tid-123" || resp.Status != "completed" {
		t.Errorf("resp = %+v", resp)
	}
	if len(resp.Jobs) != 1 || resp.Jobs[0].Result == nil || *resp.Jobs[0].Result != testJobResultHello {
		t.Errorf("Jobs = %+v", resp.Jobs)
	}
}

func TestClient_InvalidBaseURL(t *testing.T) {
	client := NewClient("://invalid")
	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestClient_Health_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected error for 503")
	}
}

func TestClient_Health_BodyMustContainOk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("unhealthy"))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected error when body does not contain ok")
	}
	if !strings.Contains(err.Error(), "ok") {
		t.Errorf("error should mention body/ok: %v", err)
	}
}

func TestClient_GetMe_DecodeError(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusOK, "not json"))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClient_GetMe_Unauthorized(t *testing.T) {
	server := httptest.NewServer(jsonHandler(http.StatusUnauthorized, problem.Details{Detail: "expired", Status: 401}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_GetMe_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		jsonHandler(http.StatusOK, userapi.UserResponse{ID: "u1", Handle: "alice", IsActive: true})(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	user, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if user.Handle != "alice" || user.ID != "u1" {
		t.Errorf("GetMe = %+v", user)
	}
}

func TestClient_parseError_NonJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("plain text"))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	_, err := client.Login(context.Background(), userapi.LoginRequest{Handle: "u", Password: "p"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Login_DecodeError(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusOK, "not json"))
	defer server.Close()
	client := NewClient(server.URL)
	_, err := client.Login(context.Background(), userapi.LoginRequest{Handle: "u", Password: "p"})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClient_CreateTask_DecodeError(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusCreated, "{"))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.CreateTask(context.Background(), &userapi.CreateTaskRequest{Prompt: "x"})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClient_GetTaskResult_DecodeError(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusOK, "{"))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.GetTaskResult(context.Background(), "tid")
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClient_NewClient_SetToken(t *testing.T) {
	c := NewClient("http://localhost")
	if c.BaseURL != "http://localhost" || c.HTTPClient == nil {
		t.Errorf("NewClient: %+v", c)
	}
	c.SetToken("t")
	if c.Token != "t" {
		t.Errorf("Token = %q", c.Token)
	}
}

type errTransport struct{}

func (errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("injected")
}

func TestClient_Health_DoFails(t *testing.T) {
	client := NewClient("http://localhost")
	client.HTTPClient = &http.Client{Transport: errTransport{}}
	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected error when request fails")
	}
}

func TestClient_Login_DoFails(t *testing.T) {
	client := NewClient("http://localhost")
	client.HTTPClient = &http.Client{Transport: errTransport{}}
	_, err := client.Login(context.Background(), userapi.LoginRequest{Handle: "u", Password: "p"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_GetMe_DoFails(t *testing.T) {
	client := NewClient("http://localhost")
	client.HTTPClient = &http.Client{Transport: errTransport{}}
	client.SetToken("tok")
	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_CreateTask_DoFails(t *testing.T) {
	client := NewClient("http://localhost")
	client.HTTPClient = &http.Client{Transport: errTransport{}}
	client.SetToken("tok")
	_, err := client.CreateTask(context.Background(), &userapi.CreateTaskRequest{Prompt: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_GetTaskResult_DoFails(t *testing.T) {
	client := NewClient("http://localhost")
	client.HTTPClient = &http.Client{Transport: errTransport{}}
	client.SetToken("tok")
	_, err := client.GetTaskResult(context.Background(), "tid")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_ListTasks_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		jsonHandler(http.StatusOK, userapi.ListTasksResponse{
			Tasks: []userapi.TaskResponse{
				{ID: "t1", Status: "completed"},
				{TaskID: "t2", ID: "t2", Status: "queued"},
				{TaskID: "t3", Status: "queued"},
			},
		})(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.ListTasks(context.Background(), ListTasksRequest{Limit: 10})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(resp.Tasks) != 3 {
		t.Fatalf("len(Tasks)=%d", len(resp.Tasks))
	}
	if resp.Tasks[0].ResolveTaskID() != "t1" || resp.Tasks[1].ResolveTaskID() != "t2" || resp.Tasks[2].ResolveTaskID() != "t3" {
		t.Errorf("ResolveTaskID: %q, %q, %q", resp.Tasks[0].ResolveTaskID(), resp.Tasks[1].ResolveTaskID(), resp.Tasks[2].ResolveTaskID())
	}
	if resp.Tasks[2].ID != "t3" {
		t.Errorf("normalizeTaskResponse: Tasks[2].ID = %q, want t3", resp.Tasks[2].ID)
	}
}

func TestClient_ListTasks_Unauthorized(t *testing.T) {
	server := httptest.NewServer(jsonHandler(http.StatusUnauthorized, problem.Details{Detail: "expired", Status: 401}))
	defer server.Close()
	client := NewClient(server.URL)
	_, err := client.ListTasks(context.Background(), ListTasksRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func taskGetHandler(path string, task *userapi.TaskResponse) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		jsonHandler(http.StatusOK, task)(w, r)
	}
}

func TestClient_GetTask_Success(t *testing.T) {
	server := httptest.NewServer(taskGetHandler("/v1/tasks/tid-1", &userapi.TaskResponse{ID: testTaskID, Status: "running"}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	task, err := client.GetTask(context.Background(), testTaskID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.ResolveTaskID() != testTaskID || task.Status != "running" {
		t.Errorf("task = %+v", task)
	}
}

func TestClient_GetTask_NormalizesTaskID(t *testing.T) {
	server := httptest.NewServer(taskGetHandler("/v1/tasks/tid-only", &userapi.TaskResponse{TaskID: "tid-only", Status: "completed"}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	task, err := client.GetTask(context.Background(), "tid-only")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.ID != "tid-only" || task.TaskID != "tid-only" {
		t.Errorf("normalizeTaskResponse: ID=%q TaskID=%q", task.ID, task.TaskID)
	}
}

func TestClient_ExpectError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   interface{}
		call   func(*Client) error
	}{
		{"GetTask_NotFound", http.StatusNotFound, problem.Details{Detail: "not found", Status: 404}, func(c *Client) error { _, err := c.GetTask(context.Background(), "tid-missing"); return err }},
		{"CancelTask_Forbidden", http.StatusForbidden, problem.Details{Detail: "not owner", Status: 403}, func(c *Client) error { _, err := c.CancelTask(context.Background(), testTaskID); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(jsonHandler(tt.status, tt.body))
			defer server.Close()
			client := NewClient(server.URL)
			client.SetToken("tok")
			if err := tt.call(client); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestClient_CancelTask_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks/tid-1/cancel" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		jsonHandler(http.StatusOK, userapi.CancelTaskResponse{TaskID: testTaskID, Canceled: true})(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.CancelTask(context.Background(), testTaskID)
	if err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	if !resp.Canceled || resp.TaskID != testTaskID {
		t.Errorf("resp = %+v", resp)
	}
}

func TestClient_GetTaskLogs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks/tid-1/logs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		jsonHandler(http.StatusOK, userapi.TaskLogsResponse{TaskID: testTaskID, Stdout: "out", Stderr: "err"})(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	logs, err := client.GetTaskLogs(context.Background(), testTaskID, "")
	if err != nil {
		t.Fatalf("GetTaskLogs: %v", err)
	}
	if logs.Stdout != "out" || logs.Stderr != "err" {
		t.Errorf("logs = %+v", logs)
	}
}

func TestClient_GetTaskLogs_NotFound(t *testing.T) {
	server := httptest.NewServer(jsonHandler(http.StatusNotFound, problem.Details{Detail: "not found", Status: 404}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.GetTaskLogs(context.Background(), "tid-missing", "stdout")
	if err == nil {
		t.Fatal("expected error")
	}
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 404 {
		t.Errorf("want HTTPError 404, got %T %v", err, err)
	}
}

func TestClient_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1ChatCompletions || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req userapi.ChatCompletionsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		content := ""
		if len(req.Messages) > 0 {
			content = "echo: " + req.Messages[0].Content
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": content}},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.Chat(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Response != "echo: hello" {
		t.Errorf("response = %q", resp.Response)
	}
}

func TestClient_ChatWithOptions_ModelAndProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1ChatCompletions || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("OpenAI-Project") != testProjectID {
			t.Errorf("OpenAI-Project = %q, want %s", r.Header.Get("OpenAI-Project"), testProjectID)
		}
		var req userapi.ChatCompletionsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Model != "gpt-4" {
			t.Errorf("Model = %q, want gpt-4", req.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.ChatWithOptions(context.Background(), "hi", "gpt-4", testProjectID)
	if err != nil {
		t.Fatalf("ChatWithOptions: %v", err)
	}
	if resp.Response != "ok" {
		t.Errorf("response = %q", resp.Response)
	}
}

const pathV1Prefs = "/v1/prefs"
const pathV1Skill = "/v1/skills/s1"

//nolint:dupl // PutBytes/DeleteBytes success handler pattern
func TestClient_PutBytes_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1Skill || r.Method != http.MethodPut {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	body, err := client.PutBytes(context.Background(), pathV1Skill, []byte(`{"content":"# x"}`))
	if err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	if string(body) != "{}" {
		t.Errorf("body = %q", body)
	}
}

//nolint:dupl // PutBytes/DeleteBytes no-content handler pattern
func TestClient_PutBytes_NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1Skill || r.Method != http.MethodPut {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	body, err := client.PutBytes(context.Background(), pathV1Skill, []byte(`{}`))
	if err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	if body != nil {
		t.Errorf("body = %v, want nil", body)
	}
}

func TestClient_PutBytes_Error(t *testing.T) {
	expectHTTPError(t, jsonHandler(http.StatusBadRequest, problem.Details{Detail: "bad", Status: 400}),
		func(c *Client) error {
			_, err := c.PutBytes(context.Background(), pathV1Skill, []byte("{}"))
			return err
		})
}

//nolint:dupl // PutBytes/DeleteBytes success handler pattern
func TestClient_DeleteBytes_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1Prefs || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	body, err := client.DeleteBytes(context.Background(), pathV1Prefs)
	if err != nil {
		t.Fatalf("DeleteBytes: %v", err)
	}
	if string(body) != "{}" {
		t.Errorf("body = %q", body)
	}
}

//nolint:dupl // PutBytes/DeleteBytes no-content handler pattern
func TestClient_DeleteBytes_NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1Prefs || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	body, err := client.DeleteBytes(context.Background(), pathV1Prefs)
	if err != nil {
		t.Fatalf("DeleteBytes: %v", err)
	}
	if body != nil {
		t.Errorf("body = %v, want nil", body)
	}
}

func TestClient_DeleteBytes_Error(t *testing.T) {
	expectHTTPError(t, jsonHandler(http.StatusForbidden, problem.Details{Detail: "forbidden", Status: 403}),
		func(c *Client) error { _, err := c.DeleteBytes(context.Background(), pathV1Prefs); return err })
}

func TestClient_ChatWithOptions_EmptyOptionalParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("OpenAI-Project") != "" {
			t.Errorf("OpenAI-Project should be empty, got %q", r.Header.Get("OpenAI-Project"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.ChatWithOptions(context.Background(), "hi", "", "")
	if err != nil {
		t.Fatalf("ChatWithOptions: %v", err)
	}
	if resp.Response != "ok" {
		t.Errorf("response = %q", resp.Response)
	}
}

func TestClient_ChatWithOptions_ServerError(t *testing.T) {
	server := httptest.NewServer(jsonHandler(http.StatusInternalServerError, problem.Details{Detail: "server error", Status: 500}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.ChatWithOptions(context.Background(), "hi", "m", "p")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_ChatWithOptions_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusOK, "not json"))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.ChatWithOptions(context.Background(), "hi", "", "")
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClient_ChatWithOptions_RequestFails(t *testing.T) {
	client := NewClient("http://127.0.0.1:0")
	client.SetToken("tok")
	_, err := client.ChatWithOptions(context.Background(), "hi", "", "")
	if err == nil {
		t.Fatal("expected request error")
	}
}

func TestClient_ResponsesWithOptions_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1Responses || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("OpenAI-Project") != testProjectID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "resp-123",
			"output": []map[string]any{
				{"type": "text", "text": "Hello back"},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.ResponsesWithOptions(context.Background(), "hello", "gpt-4", testProjectID)
	if err != nil {
		t.Fatalf("ResponsesWithOptions: %v", err)
	}
	if resp.VisibleText != "Hello back" || resp.ResponseID != "resp-123" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestClient_ResponsesWithOptions_ErrorPaths(t *testing.T) {
	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(jsonHandler(http.StatusBadGateway, problem.Details{Detail: "bad gateway", Status: 502}))
		defer server.Close()
		client := NewClient(server.URL)
		client.SetToken("tok")
		if _, err := client.ResponsesWithOptions(context.Background(), "hi", "", ""); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("DecodeError", func(t *testing.T) {
		server := httptest.NewServer(rawHandler(http.StatusOK, "not json"))
		defer server.Close()
		client := NewClient(server.URL)
		client.SetToken("tok")
		if _, err := client.ResponsesWithOptions(context.Background(), "hi", "", ""); err == nil {
			t.Fatal("expected decode error")
		}
	})
}

func TestClient_ResponsesWithOptions_InvalidBaseURL(t *testing.T) {
	client := NewClient("://invalid")
	client.SetToken("tok")
	_, err := client.ResponsesWithOptions(context.Background(), "hi", "", "")
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestClient_ResponsesWithOptions_RequestFails(t *testing.T) {
	client := NewClient("http://127.0.0.1:0")
	client.SetToken("tok")
	_, err := client.ResponsesWithOptions(context.Background(), "hi", "", "")
	if err == nil {
		t.Fatal("expected error when request fails")
	}
}

func TestClient_ResponsesWithOptions_MultipleTextOutputs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "r-2",
			"output": []map[string]any{
				{"type": "text", "text": "one"},
				{"type": "other"},
				{"type": "text", "text": "two"},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.ResponsesWithOptions(context.Background(), "hi", "", "")
	if err != nil {
		t.Fatalf("ResponsesWithOptions: %v", err)
	}
	if resp.VisibleText != "onetwo" || resp.ResponseID != "r-2" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestClient_ResponsesWithOptions_EmptyOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "r-empty", "output": []any{}})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.ResponsesWithOptions(context.Background(), "hi", "", "")
	if err != nil {
		t.Fatalf("ResponsesWithOptions: %v", err)
	}
	if resp.VisibleText != "" || resp.ResponseID != "r-empty" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestClient_Refresh_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/refresh" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req userapi.RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(userapi.LoginResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			TokenType:    "Bearer",
			ExpiresIn:    900,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	resp, err := client.Refresh(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if resp.AccessToken != "new-access" || resp.RefreshToken != "new-refresh" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestClient_ListModels_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ListModelsResponse{
			Object: "list",
			Data:   []ListModelEntry{{ID: "cynodeai.pm", Object: "model", Created: 0}},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if resp.Object != "list" || len(resp.Data) != 1 || resp.Data[0].ID != "cynodeai.pm" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestClient_PostTaskReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks/t-1/ready" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(userapi.TaskResponse{
			ID:            "t-1",
			Status:        "queued",
			PlanningState: userapi.PlanningStateReady,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	out, err := client.PostTaskReady(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("PostTaskReady: %v", err)
	}
	if out.ResolveTaskID() != "t-1" || out.PlanningState != userapi.PlanningStateReady {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestIsUnauthorized(t *testing.T) {
	t.Parallel()
	if IsUnauthorized(nil) {
		t.Error("nil should not be unauthorized")
	}
	if IsUnauthorized(errors.New("plain")) {
		t.Error("plain error should not be unauthorized")
	}
	u := &HTTPError{Status: http.StatusUnauthorized, Err: errors.New("expired")}
	if !IsUnauthorized(u) {
		t.Error("HTTPError 401 should be unauthorized")
	}
	if !IsUnauthorized(fmt.Errorf("wrap: %w", u)) {
		t.Error("wrapped HTTPError 401 should be unauthorized")
	}
	if IsUnauthorized(&HTTPError{Status: http.StatusForbidden, Err: errors.New("no")}) {
		t.Error("403 should not be unauthorized")
	}
}
