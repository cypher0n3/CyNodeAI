package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

func TestClient_ListTasks_WithLimitOffset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		jsonHandler(http.StatusOK, userapi.ListTasksResponse{Tasks: []userapi.TaskResponse{}})(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.ListTasks(ListTasksRequest{Limit: 5, Offset: 10})
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
			Jobs: []userapi.JobResponse{{ID: "j1", Status: "completed", Result: strPtr("hello\n")}},
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
	if len(resp.Jobs) != 1 || resp.Jobs[0].Result == nil || *resp.Jobs[0].Result != "hello\n" {
		t.Errorf("Jobs = %+v", resp.Jobs)
	}
}

func strPtr(s string) *string { return &s }

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
			Tasks: []userapi.TaskResponse{{ID: "t1", Status: "completed"}, {TaskID: "t2", ID: "t2", Status: "queued"}},
		})(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.ListTasks(ListTasksRequest{Limit: 10})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(resp.Tasks) != 2 {
		t.Fatalf("len(Tasks)=%d", len(resp.Tasks))
	}
	if resp.Tasks[0].ResolveTaskID() != "t1" || resp.Tasks[1].ResolveTaskID() != "t2" {
		t.Errorf("Tasks[0].ResolveTaskID()=%q Tasks[1].ResolveTaskID()=%q", resp.Tasks[0].ResolveTaskID(), resp.Tasks[1].ResolveTaskID())
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

func TestClient_GetTask_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks/tid-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		jsonHandler(http.StatusOK, userapi.TaskResponse{ID: "tid-1", Status: "running"})(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	task, err := client.GetTask("tid-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.ResolveTaskID() != "tid-1" || task.Status != "running" {
		t.Errorf("task = %+v", task)
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
		{"CancelTask_Forbidden", http.StatusForbidden, problem.Details{Detail: "not owner", Status: 403}, func(c *Client) error { _, err := c.CancelTask("tid-1"); return err }},
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
		jsonHandler(http.StatusOK, userapi.CancelTaskResponse{TaskID: "tid-1", Canceled: true})(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	resp, err := client.CancelTask("tid-1")
	if err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	if !resp.Canceled || resp.TaskID != "tid-1" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestClient_GetTaskLogs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks/tid-1/logs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		jsonHandler(http.StatusOK, userapi.TaskLogsResponse{TaskID: "tid-1", Stdout: "out", Stderr: "err"})(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	logs, err := client.GetTaskLogs("tid-1", "")
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
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
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
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("OpenAI-Project") != "proj-1" {
			t.Errorf("OpenAI-Project = %q, want proj-1", r.Header.Get("OpenAI-Project"))
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
	resp, err := client.ChatWithOptions("hi", "gpt-4", "proj-1")
	if err != nil {
		t.Fatalf("ChatWithOptions: %v", err)
	}
	if resp.Response != "ok" {
		t.Errorf("response = %q", resp.Response)
	}
}

const pathV1Prefs = "/v1/prefs"

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
