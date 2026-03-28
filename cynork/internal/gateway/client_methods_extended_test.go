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

// responsesOutputTextDeltaSSE is a native /v1/responses stream fragment (per CYNAI.USRGWY.StreamingPerEndpointSSEFormat).
func responsesOutputTextDeltaSSE(delta string) string {
	b, _ := json.Marshal(map[string]string{"delta": delta})
	return fmt.Sprintf("event: %s\ndata: %s\n\n", userapi.SSEEventResponseOutputTextDelta, string(b))
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
	}, nil, nil)
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
	err := c.ChatStream(context.Background(), "hi", "", "", func(_ string) {}, nil, nil)
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
	err := c.ChatStream(context.Background(), "hi", "", "", func(_ string) {}, nil, nil)
	if err == nil {
		t.Fatal("expected structured error from stream")
	}
}

func TestClient_ChatStream_InvalidBaseURL(t *testing.T) {
	c := NewClient("://bad")
	err := c.ChatStream(context.Background(), "hi", "", "", func(_ string) {}, nil, nil)
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
		_, _ = w.Write([]byte(responsesOutputTextDeltaSSE("resp text")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	var got strings.Builder
	_, err := c.ResponsesStream(context.Background(), "hi", "m", "p", func(delta string) {
		got.WriteString(delta)
	}, nil, nil)
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
	_, err := c.ResponsesStream(context.Background(), "hi", "", "", func(_ string) {}, nil, nil)
	if err == nil {
		t.Fatal("expected error from ResponsesStream on 502")
	}
}

func TestClient_ResponsesStream_InvalidBaseURL(t *testing.T) {
	c := NewClient("://bad")
	_, err := c.ResponsesStream(context.Background(), "hi", "", "", func(_ string) {}, nil, nil)
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
		_, _ = w.Write([]byte(responsesOutputTextDeltaSSE("ok")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	var got strings.Builder
	respID, err := c.ResponsesStream(context.Background(), "hi", "m", "", func(delta string) {
		got.WriteString(delta)
	}, nil, nil)
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
		_, _ = w.Write([]byte(responsesOutputTextDeltaSSE("projected")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	var got strings.Builder
	_, err := c.ResponsesStream(context.Background(), "hi", "m", "proj-1", func(delta string) {
		got.WriteString(delta)
	}, nil, nil)
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
	err := c.ChatStream(context.Background(), "hi", "", "", func(_ string) {}, nil, nil)
	if err == nil {
		t.Fatal("expected error from HTTP Do on closed server")
	}
}

func TestClient_ResponsesStream_HTTPDoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()
	c := NewClient(srv.URL)
	c.SetToken("tok")
	_, err := c.ResponsesStream(context.Background(), "hi", "", "", func(_ string) {}, nil, nil)
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
	}, nil, nil)
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
	}, nil, nil)
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
	err := readChatSSEStream(context.Background(), r, func(_ string) {}, nil, nil, nil)
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
	}, nil)
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
