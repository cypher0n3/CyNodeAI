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
	_, err := client.ListTasks(ListTasksRequest{Limit: 5, Offset: 10, Cursor: "2"})
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
	resp, err := client.Login(userapi.LoginRequest{Handle: "u", Password: "p"})
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
	_, err := client.Login(userapi.LoginRequest{Handle: "u", Password: "wrong"})
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
	if err := client.Health(); err != nil {
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
	_, err := client.CreateTask(&userapi.CreateTaskRequest{Prompt: "echo hi"})
	if err == nil {
		t.Fatal("expected error when no token")
	}

	client.SetToken("tok")
	resp, err := client.CreateTask(&userapi.CreateTaskRequest{Prompt: "echo hi"})
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
	resp, err := client.GetTaskResult("tid-123")
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
	err := client.Health()
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
	err := client.Health()
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
	err := client.Health()
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
	_, err := client.GetMe()
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClient_GetMe_Unauthorized(t *testing.T) {
	server := httptest.NewServer(jsonHandler(http.StatusUnauthorized, problem.Details{Detail: "expired", Status: 401}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.GetMe()
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
	user, err := client.GetMe()
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
	_, err := client.Login(userapi.LoginRequest{Handle: "u", Password: "p"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Login_DecodeError(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusOK, "not json"))
	defer server.Close()
	client := NewClient(server.URL)
	_, err := client.Login(userapi.LoginRequest{Handle: "u", Password: "p"})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClient_CreateTask_DecodeError(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusCreated, "{"))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.CreateTask(&userapi.CreateTaskRequest{Prompt: "x"})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClient_GetTaskResult_DecodeError(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusOK, "{"))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.GetTaskResult("tid")
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
	err := client.Health()
	if err == nil {
		t.Fatal("expected error when request fails")
	}
}

func TestClient_Login_DoFails(t *testing.T) {
	client := NewClient("http://localhost")
	client.HTTPClient = &http.Client{Transport: errTransport{}}
	_, err := client.Login(userapi.LoginRequest{Handle: "u", Password: "p"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_GetMe_DoFails(t *testing.T) {
	client := NewClient("http://localhost")
	client.HTTPClient = &http.Client{Transport: errTransport{}}
	client.SetToken("tok")
	_, err := client.GetMe()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_CreateTask_DoFails(t *testing.T) {
	client := NewClient("http://localhost")
	client.HTTPClient = &http.Client{Transport: errTransport{}}
	client.SetToken("tok")
	_, err := client.CreateTask(&userapi.CreateTaskRequest{Prompt: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_GetTaskResult_DoFails(t *testing.T) {
	client := NewClient("http://localhost")
	client.HTTPClient = &http.Client{Transport: errTransport{}}
	client.SetToken("tok")
	_, err := client.GetTaskResult("tid")
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
	resp, err := client.ListTasks(ListTasksRequest{Limit: 10})
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
	_, err := client.ListTasks(ListTasksRequest{})
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
	task, err := client.GetTask(testTaskID)
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
	task, err := client.GetTask("tid-only")
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
		{"GetTask_NotFound", http.StatusNotFound, problem.Details{Detail: "not found", Status: 404}, func(c *Client) error { _, err := c.GetTask("tid-missing"); return err }},
		{"CancelTask_Forbidden", http.StatusForbidden, problem.Details{Detail: "not owner", Status: 403}, func(c *Client) error { _, err := c.CancelTask(testTaskID); return err }},
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
	resp, err := client.CancelTask(testTaskID)
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
	logs, err := client.GetTaskLogs(testTaskID, "")
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
	_, err := client.GetTaskLogs("tid-missing", "stdout")
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
	resp, err := client.Chat("hello")
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
	resp, err := client.ChatWithOptions("hi", "gpt-4", testProjectID)
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
	body, err := client.PutBytes(pathV1Skill, []byte(`{"content":"# x"}`))
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
	body, err := client.PutBytes(pathV1Skill, []byte(`{}`))
	if err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	if body != nil {
		t.Errorf("body = %v, want nil", body)
	}
}

func TestClient_PutBytes_Error(t *testing.T) {
	expectHTTPError(t, jsonHandler(http.StatusBadRequest, problem.Details{Detail: "bad", Status: 400}),
		func(c *Client) error { _, err := c.PutBytes(pathV1Skill, []byte("{}")); return err })
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
	body, err := client.DeleteBytes(pathV1Prefs)
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
	body, err := client.DeleteBytes(pathV1Prefs)
	if err != nil {
		t.Fatalf("DeleteBytes: %v", err)
	}
	if body != nil {
		t.Errorf("body = %v, want nil", body)
	}
}

func TestClient_DeleteBytes_Error(t *testing.T) {
	expectHTTPError(t, jsonHandler(http.StatusForbidden, problem.Details{Detail: "forbidden", Status: 403}),
		func(c *Client) error { _, err := c.DeleteBytes(pathV1Prefs); return err })
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
	resp, err := client.ChatWithOptions("hi", "", "")
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
	_, err := client.ChatWithOptions("hi", "m", "p")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_ChatWithOptions_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusOK, "not json"))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.ChatWithOptions("hi", "", "")
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClient_ChatWithOptions_RequestFails(t *testing.T) {
	client := NewClient("http://127.0.0.1:0")
	client.SetToken("tok")
	_, err := client.ChatWithOptions("hi", "", "")
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
	resp, err := client.ResponsesWithOptions("hello", "gpt-4", testProjectID)
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
		if _, err := client.ResponsesWithOptions("hi", "", ""); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("DecodeError", func(t *testing.T) {
		server := httptest.NewServer(rawHandler(http.StatusOK, "not json"))
		defer server.Close()
		client := NewClient(server.URL)
		client.SetToken("tok")
		if _, err := client.ResponsesWithOptions("hi", "", ""); err == nil {
			t.Fatal("expected decode error")
		}
	})
}

func TestClient_ResponsesWithOptions_InvalidBaseURL(t *testing.T) {
	client := NewClient("://invalid")
	client.SetToken("tok")
	_, err := client.ResponsesWithOptions("hi", "", "")
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestClient_ResponsesWithOptions_RequestFails(t *testing.T) {
	client := NewClient("http://127.0.0.1:0")
	client.SetToken("tok")
	_, err := client.ResponsesWithOptions("hi", "", "")
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
	resp, err := client.ResponsesWithOptions("hi", "", "")
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
	resp, err := client.ResponsesWithOptions("hi", "", "")
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
	resp, err := client.Refresh("old-refresh-token")
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
	resp, err := client.ListModels()
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if resp.Object != "list" || len(resp.Data) != 1 || resp.Data[0].ID != "cynodeai.pm" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestClient_ListProjects_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"p1","name":"Proj One"},{"id":"p2","name":"Proj Two"}]}`))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(resp.Data) != 2 || resp.Data[0].ID != "p1" || resp.Data[1].Name != "Proj Two" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestClient_GetProject_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/projects/p1" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"p1","name":"Proj One"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	proj, err := client.GetProject("p1")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if proj.ID != "p1" || proj.Name != "Proj One" {
		t.Errorf("proj = %+v", proj)
	}
}

func TestClient_UnauthorizedOrBadStatus(t *testing.T) {
	unauth := jsonHandler(http.StatusUnauthorized, problem.Details{Detail: "expired", Status: 401})
	tests := []struct {
		name string
		run  func(*Client) error
	}{
		{"Chat", func(c *Client) error { _, err := c.Chat("hi"); return err }},
		{"Refresh", func(c *Client) error { _, err := c.Refresh("refresh-tok"); return err }},
		{"ListModels", func(c *Client) error {
			c.SetToken("tok")
			_, err := c.ListModels()
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(unauth)
			defer server.Close()
			client := NewClient(server.URL)
			if err := tt.run(client); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestClient_InvalidJSONResponse(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		run  func(*Client) error
	}{
		{"Refresh", []byte("not json"), func(c *Client) error { _, err := c.Refresh("tok"); return err }},
		{"ListModels", []byte("[]"), func(c *Client) error {
			c.SetToken("tok")
			_, err := c.ListModels()
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(tt.body)
			}))
			defer server.Close()
			client := NewClient(server.URL)
			if err := tt.run(client); err == nil {
				t.Fatal("expected decode error")
			}
		})
	}
}

func TestClient_Refresh_ReturnsCreated(t *testing.T) {
	// Refresh expects 200; server returns 201 so doPostJSONNoAuth returns parseError
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(userapi.LoginResponse{AccessToken: "a", RefreshToken: "r"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	_, err := client.Refresh("tok")
	if err == nil {
		t.Fatal("expected error when status is 201")
	}
}

func TestHTTPError_Error(t *testing.T) {
	// Err nil branch
	e := &HTTPError{Status: 503}
	if got := e.Error(); got != "HTTP 503" {
		t.Errorf("Error() = %q, want HTTP 503", got)
	}
	// Err non-nil branch (from parseError)
	e.Err = errors.New("detail")
	if got := e.Error(); got != "detail" {
		t.Errorf("Error() = %q, want detail", got)
	}
}

func TestClient_GetBytes_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`["a","b"]`))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	body, err := client.GetBytes("/v1/creds")
	if err != nil {
		t.Fatalf("GetBytes: %v", err)
	}
	if string(body) != `["a","b"]` {
		t.Errorf("body = %q", body)
	}
}

func TestClient_GetBytes_Unauthorized(t *testing.T) {
	server := httptest.NewServer(jsonHandler(http.StatusUnauthorized, problem.Details{Status: 401}))
	defer server.Close()
	client := NewClient(server.URL)
	_, err := client.GetBytes("/v1/creds")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_PostBytes_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.PostBytes("/v1/prefs", []byte("{}"))
	if err != nil {
		t.Fatalf("PostBytes: %v", err)
	}
}

func TestClient_PostBytes_NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	body, err := client.PostBytes("/v1/prefs", nil)
	if err != nil {
		t.Fatalf("PostBytes: %v", err)
	}
	if body != nil {
		t.Errorf("want nil body for 204, got %q", body)
	}
}

func TestClient_GetBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/creds" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`[{"id":"c1"}]`))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	body, err := client.GetBytes("/v1/creds")
	if err != nil {
		t.Fatalf("GetBytes: %v", err)
	}
	if string(body) != `[{"id":"c1"}]` {
		t.Errorf("body = %q", body)
	}
}

func expectHTTPError(t *testing.T, handler http.Handler, fn func(*Client) error) {
	t.Helper()
	server := httptest.NewServer(handler)
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	if err := fn(client); err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_GetBytes_Error(t *testing.T) {
	expectHTTPError(t, jsonHandler(http.StatusUnauthorized, problem.Details{Detail: "unauthorized", Status: 401}),
		func(c *Client) error { _, err := c.GetBytes("/v1/creds"); return err })
}

func TestClient_PostBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/prefs" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	body, err := client.PostBytes("/v1/prefs", []byte("{}"))
	if err != nil {
		t.Fatalf("PostBytes: %v", err)
	}
	if string(body) != `{}` {
		t.Errorf("body = %q", body)
	}
}

func TestClient_PostBytes_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(http.StatusForbidden, problem.Details{Detail: "forbidden", Status: 403}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.PostBytes("/v1/prefs", []byte("{}"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	e := &HTTPError{Status: 404, Err: inner}
	if e.Unwrap() != inner {
		t.Error("Unwrap should return inner")
	}
}

func TestClient_NewChatThread_Success(t *testing.T) {
	wantID := "abc123-thread-id"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != pathV1ChatThreads {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"thread_id": wantID})
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	got, err := c.NewChatThread("")
	if err != nil {
		t.Fatalf("NewChatThread: %v", err)
	}
	if got != wantID {
		t.Errorf("thread_id = %q, want %q", got, wantID)
	}
}

func TestClient_NewChatThread_WithProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("OpenAI-Project") != testProjectID {
			t.Errorf("expected OpenAI-Project header, got %q", r.Header.Get("OpenAI-Project"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"thread_id": "tid"})
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.NewChatThread(testProjectID)
	if err != nil {
		t.Fatalf("NewChatThread: %v", err)
	}
}

func TestClient_NewChatThread_Error(t *testing.T) {
	errBody := map[string]interface{}{"error": map[string]string{"message": "unauthorized", "code": "unauthorized"}}
	srv := httptest.NewServer(jsonHandler(http.StatusUnauthorized, errBody))
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.NewChatThread("")
	if err == nil {
		t.Fatal("expected error on non-201 response")
	}
}

func TestClient_NewChatThread_BadJSON(t *testing.T) {
	srv := httptest.NewServer(threadsAPIHandler(pathV1ChatThreads, http.StatusCreated, "not-json{{{"))
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.NewChatThread("")
	if err == nil {
		t.Fatal("expected error on bad JSON response")
	}
}

func TestClient_NewChatThread_NetworkError(t *testing.T) {
	c := NewClient("http://127.0.0.1:1") // nothing listening
	_, err := c.NewChatThread("")
	if err == nil {
		t.Fatal("expected error when server unreachable")
	}
}

func TestClient_NewChatThread_InvalidBaseURL(t *testing.T) {
	c := NewClient("://bad-url")
	_, err := c.NewChatThread("")
	if err == nil {
		t.Fatal("expected error on invalid base URL")
	}
}

func TestClient_ListChatThreads_Success(t *testing.T) {
	wantID := "thread-1"
	title := "First"
	body := `{"data":[{"id":"` + wantID + `","title":"` + title + `","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}]}`
	srv := httptest.NewServer(threadsAPIHandler(pathV1ChatThreads, http.StatusOK, body))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	list, err := c.ListChatThreads("", 20, 0)
	if err != nil {
		t.Fatalf("ListChatThreads: %v", err)
	}
	if len(list) != 1 || list[0].ID != wantID {
		t.Errorf("list = %+v", list)
	}
	if list[0].Title == nil || *list[0].Title != title {
		t.Errorf("list[0].Title = %v", list[0].Title)
	}
}

func TestClient_PatchThreadTitle_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != pathV1ChatThreads+"/thread-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"thread-1"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	err := c.PatchThreadTitle("thread-1", "New Title")
	if err != nil {
		t.Fatalf("PatchThreadTitle: %v", err)
	}
}

func TestClient_ListChatThreads_WithProjectAndPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != pathV1ChatThreads {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("OpenAI-Project") != "proj-1" {
			t.Errorf("OpenAI-Project = %q", r.Header.Get("OpenAI-Project"))
		}
		if r.URL.Query().Get("limit") != "5" || r.URL.Query().Get("offset") != "10" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	_, err := c.ListChatThreads("proj-1", 5, 10)
	if err != nil {
		t.Fatalf("ListChatThreads: %v", err)
	}
}

func TestClient_PatchThreadTitle_DoFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	u := srv.URL
	srv.Close()
	c := NewClient(u)
	c.SetToken("tok")
	err := c.PatchThreadTitle("thread-1", "Title")
	if err == nil {
		t.Fatal("expected error when server closed")
	}
}

func TestClient_PatchThreadTitle_Error(t *testing.T) {
	errBody := map[string]interface{}{"error": map[string]string{"message": "not found", "code": "not_found"}}
	srv := httptest.NewServer(jsonHandler(http.StatusNotFound, errBody))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	err := c.PatchThreadTitle("thread-1", "Title")
	if err == nil {
		t.Fatal("expected error from PatchThreadTitle")
	}
}

func TestClient_ListChatThreads_Error(t *testing.T) {
	body := `{"error":{"message":"unauthorized","code":"unauthorized"}}`
	srv := httptest.NewServer(threadsAPIHandler(pathV1ChatThreads, http.StatusUnauthorized, body))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	_, err := c.ListChatThreads("", 20, 0)
	if err == nil {
		t.Fatal("expected error from ListChatThreads")
	}
}

func TestClient_ListChatThreads_InvalidBaseURL(t *testing.T) {
	c := NewClient("://bad")
	_, err := c.ListChatThreads("", 20, 0)
	if err == nil {
		t.Fatal("expected error from ListChatThreads")
	}
}

func TestClient_ListChatThreads_BadJSON(t *testing.T) {
	srv := httptest.NewServer(threadsAPIHandler(pathV1ChatThreads, http.StatusOK, "not json"))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	_, err := c.ListChatThreads("", 20, 0)
	if err == nil {
		t.Fatal("expected error from ListChatThreads")
	}
}

// sseChunkLine builds one SSE data line with a chat.completion.chunk payload.
func sseChunkLine(content, finishReason string) string {
	fr := "null"
	if finishReason != "" {
		fr = `"` + finishReason + `"`
	}
	return fmt.Sprintf("data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%q},\"finish_reason\":%s}]}\n\n",
		content, fr)
}

func TestClient_ChatStream_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1ChatCompletions || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseChunkLine("hello", "")))
		_, _ = w.Write([]byte(sseChunkLine(" world", "")))
		_, _ = w.Write([]byte(sseChunkLine("", "stop")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	var got strings.Builder
	err := c.ChatStream(context.Background(), "hi", "m", "p", func(delta string) {
		got.WriteString(delta)
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if got.String() != "hello world" {
		t.Errorf("accumulated = %q, want %q", got.String(), "hello world")
	}
}

func TestClient_ChatStream_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	err := c.ChatStream(context.Background(), "hi", "", "", func(_ string) {}, nil)
	if err == nil {
		t.Fatal("expected error from ChatStream on 503")
	}
}

func TestClient_ChatStream_StructuredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: {"error":{"message":"boom","code":"fail"}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	err := c.ChatStream(context.Background(), "hi", "", "", func(_ string) {}, nil)
	if err == nil {
		t.Fatal("expected structured error from stream")
	}
}

func TestClient_ChatStream_InvalidBaseURL(t *testing.T) {
	c := NewClient("://bad")
	err := c.ChatStream(context.Background(), "hi", "", "", func(_ string) {}, nil)
	if err == nil {
		t.Fatal("expected error from invalid base URL")
	}
}

func TestClient_ResponsesStream_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1Responses || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseChunkLine("resp text", "")))
		_, _ = w.Write([]byte(sseChunkLine("", "stop")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	var got strings.Builder
	_, err := c.ResponsesStream(context.Background(), "hi", "m", "p", func(delta string) {
		got.WriteString(delta)
	}, nil)
	if err != nil {
		t.Fatalf("ResponsesStream: %v", err)
	}
	if got.String() != "resp text" {
		t.Errorf("accumulated = %q, want %q", got.String(), "resp text")
	}
}

func TestClient_ResponsesStream_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	_, err := c.ResponsesStream(context.Background(), "hi", "", "", func(_ string) {}, nil)
	if err == nil {
		t.Fatal("expected error from ResponsesStream on 502")
	}
}

func TestClient_ResponsesStream_InvalidBaseURL(t *testing.T) {
	c := NewClient("://bad")
	_, err := c.ResponsesStream(context.Background(), "hi", "", "", func(_ string) {}, nil)
	if err == nil {
		t.Fatal("expected error from invalid base URL")
	}
}

func TestClient_ResponsesStream_ReturnsStreamedResponseID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1Responses {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"response_id\":\"resp-abc-123\"}\n\n"))
		_, _ = w.Write([]byte(sseChunkLine("ok", "")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	var got strings.Builder
	respID, err := c.ResponsesStream(context.Background(), "hi", "m", "", func(delta string) {
		got.WriteString(delta)
	}, nil)
	if err != nil {
		t.Fatalf("ResponsesStream: %v", err)
	}
	if respID != "resp-abc-123" {
		t.Errorf("response_id = %q, want resp-abc-123", respID)
	}
	if got.String() != "ok" {
		t.Errorf("deltas = %q, want ok", got.String())
	}
}

func TestClient_ResponsesStream_WithProjectAndToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("OpenAI-Project") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseChunkLine("projected", "")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	var got strings.Builder
	_, err := c.ResponsesStream(context.Background(), "hi", "m", "proj-1", func(delta string) {
		got.WriteString(delta)
	}, nil)
	if err != nil {
		t.Fatalf("ResponsesStream with project: %v", err)
	}
	if got.String() != "projected" {
		t.Errorf("got %q, want %q", got.String(), "projected")
	}
}

func TestClient_ChatStream_HTTPDoError(t *testing.T) {
	// Use a closed server to force HTTP Do to fail.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	err := c.ChatStream(context.Background(), "hi", "", "", func(_ string) {}, nil)
	if err == nil {
		t.Fatal("expected error from HTTP Do on closed server")
	}
}

func TestClient_ResponsesStream_HTTPDoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	_, err := c.ResponsesStream(context.Background(), "hi", "", "", func(_ string) {}, nil)
	if err == nil {
		t.Fatal("expected error from HTTP Do on closed server")
	}
}

func TestReadChatSSEStream_SkipsNonDataLines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(": comment line\n\n"))
		_, _ = w.Write([]byte("event: ping\n\n"))
		_, _ = w.Write([]byte(sseChunkLine("hi", "")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	var got strings.Builder
	err := c.ChatStream(context.Background(), "m", "", "", func(delta string) {
		got.WriteString(delta)
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if got.String() != "hi" {
		t.Errorf("got %q, want %q", got.String(), "hi")
	}
}

func TestReadChatSSEStream_MalformedJSONChunkIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: not-json\n\n"))
		_, _ = w.Write([]byte(sseChunkLine("ok", "")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	var got strings.Builder
	err := c.ChatStream(context.Background(), "m", "", "", func(delta string) {
		got.WriteString(delta)
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if got.String() != "ok" {
		t.Errorf("got %q, want %q", got.String(), "ok")
	}
}

// errReader is an io.Reader that returns an error after a few successful reads.
type errReader struct {
	data []byte
	pos  int
	err  error
}

func (e *errReader) Read(p []byte) (int, error) {
	if e.pos >= len(e.data) {
		return 0, e.err
	}
	n := copy(p, e.data[e.pos:])
	e.pos += n
	return n, nil
}

// TestReadChatSSEStream_ScannerError verifies that a scanner error is surfaced as an error return.
func TestReadChatSSEStream_ScannerError(t *testing.T) {
	// Build a reader that returns an error after writing some data.
	r := &errReader{
		data: []byte("data: bad-json\n"),
		err:  fmt.Errorf("simulated read error"),
	}
	err := readChatSSEStream(context.Background(), r, func(_ string) {}, nil, nil)
	if err == nil {
		t.Fatal("expected error from scanner failure, got nil")
	}
}

func TestReadChatSSEStream_AmendmentEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseChunkLine("secret", "")))
		_, _ = w.Write([]byte("event: cynodeai.amendment\n"))
		_, _ = w.Write([]byte(`data: {"type":"secret_redaction","content":"SECRET_REDACTED","redaction_kinds":["api_key"]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	var deltas, amendment strings.Builder
	err := c.ChatStream(context.Background(), "m", "", "", func(d string) {
		deltas.WriteString(d)
	}, func(redacted string) {
		amendment.WriteString(redacted)
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if deltas.String() != "secret" {
		t.Errorf("deltas = %q", deltas.String())
	}
	if amendment.String() != "SECRET_REDACTED" {
		t.Errorf("amendment = %q", amendment.String())
	}
}

// pathWithID builds a URL path like "/v1/chat/threads/tid-1" for use in mock servers.
func pathWithID(base, id string) string { return base + "/" + id }

// routeHandler returns an HTTP handler that serves body on path+method; returns 404 otherwise.
func routeHandler(path, method string, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path || r.Method != method {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
}

func TestClient_GetChatThread_Success(t *testing.T) {
	body := []byte(`{"id":"` + testTaskID + `","title":"My Thread"}`)
	srv := httptest.NewServer(routeHandler(pathWithID("/v1/chat/threads", testTaskID), http.MethodGet, body))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	thread, err := c.GetChatThread(testTaskID)
	if err != nil {
		t.Fatalf("GetChatThread: %v", err)
	}
	if thread.ID != testTaskID {
		t.Errorf("ID = %q, want %s", thread.ID, testTaskID)
	}
}

func TestClient_GetChatThread_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	_, err := c.GetChatThread("missing")
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestClient_GetChatThread_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	_, err := c.GetChatThread("t1")
	if err == nil {
		t.Error("expected decode error")
	}
}

func TestClient_GetChatThread_InvalidBaseURL(t *testing.T) {
	c := NewClient("://invalid")
	_, err := c.GetChatThread("t1")
	if err == nil {
		t.Error("expected error for invalid base URL")
	}
}

func TestClient_PatchThreadTitle_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	err := c.PatchThreadTitle("t1", "New Title")
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestClient_Login_Refresh_Success(t *testing.T) {
	body := []byte(`{"access_token":"new-tok","refresh_token":"new-ref","token_type":"Bearer","expires_in":900}`)
	srv := httptest.NewServer(routeHandler("/v1/auth/refresh", http.MethodPost, body))
	defer srv.Close()
	c := NewClient(srv.URL)
	resp, err := c.Refresh("old-refresh")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if resp.AccessToken != "new-tok" {
		t.Errorf("AccessToken = %q, want new-tok", resp.AccessToken)
	}
}

func TestClient_Login_Refresh_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.Refresh("bad-refresh")
	if err == nil {
		t.Error("expected error for 401")
	}
}

func TestClient_Login_Refresh_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.Refresh("tok")
	if err == nil {
		t.Error("expected decode error")
	}
}
