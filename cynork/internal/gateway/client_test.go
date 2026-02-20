package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if req.Handle != "u" || req.Password != "p" {
			http.Error(w, "bad creds", http.StatusUnauthorized)
			return
		}
		jsonHandler(http.StatusOK, LoginResponse{AccessToken: "tok", TokenType: "Bearer", ExpiresIn: 900})(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.Login(LoginRequest{Handle: "u", Password: "p"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.AccessToken != "tok" {
		t.Errorf("AccessToken = %q, want tok", resp.AccessToken)
	}
}

func TestClient_Login_Unauthorized(t *testing.T) {
	server := httptest.NewServer(jsonHandler(http.StatusUnauthorized, ProblemDetails{
		Detail: "Invalid credentials", Status: 401,
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Login(LoginRequest{Handle: "u", Password: "wrong"})
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
			jsonHandler(http.StatusUnauthorized, ProblemDetails{Detail: "Not authenticated", Status: 401})(w, r)
			return
		}
		jsonHandler(http.StatusCreated, TaskResponse{ID: "tid", Status: "queued"})(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.CreateTask(CreateTaskRequest{Prompt: "echo hi"})
	if err == nil {
		t.Fatal("expected error when no token")
	}

	client.SetToken("tok")
	resp, err := client.CreateTask(CreateTaskRequest{Prompt: "echo hi"})
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
		jsonHandler(http.StatusOK, TaskResultResponse{
			TaskID: "tid-123", Status: "completed",
			Jobs: []JobResponse{{ID: "j1", Status: "completed", Result: strPtr("hello\n")}},
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
	server := httptest.NewServer(jsonHandler(http.StatusUnauthorized, ProblemDetails{Detail: "expired", Status: 401}))
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
		jsonHandler(http.StatusOK, UserResponse{ID: "u1", Handle: "alice", IsActive: true})(w, r)
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
	_, err := client.Login(LoginRequest{Handle: "u", Password: "p"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Login_DecodeError(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusOK, "not json"))
	defer server.Close()
	client := NewClient(server.URL)
	_, err := client.Login(LoginRequest{Handle: "u", Password: "p"})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestClient_CreateTask_DecodeError(t *testing.T) {
	server := httptest.NewServer(rawHandler(http.StatusCreated, "{"))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.CreateTask(CreateTaskRequest{Prompt: "x"})
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
	_, err := client.Login(LoginRequest{Handle: "u", Password: "p"})
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
	_, err := client.CreateTask(CreateTaskRequest{Prompt: "x"})
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
