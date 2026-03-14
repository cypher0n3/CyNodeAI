package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

func TestNewSession(t *testing.T) {
	client := gateway.NewClient("http://localhost")
	session := NewSession(client)
	if session.Client != client || session.Transport == nil {
		t.Errorf("NewSession: %+v", session)
	}
}

func TestNewSessionWithResponses(t *testing.T) {
	client := gateway.NewClient("http://localhost")
	session := NewSessionWithResponses(client)
	if session.Client != client || session.Transport == nil {
		t.Errorf("NewSessionWithResponses: %+v", session)
	}
	if _, ok := session.Transport.(*ResponsesTransport); !ok {
		t.Error("expected ResponsesTransport")
	}
}

func TestSession_SetModel_SetProjectID_SetToken(t *testing.T) {
	client := gateway.NewClient("http://localhost")
	session := NewSession(client)
	session.SetModel("gpt-4")
	if session.Model != "gpt-4" {
		t.Errorf("Model = %q", session.Model)
	}
	session.SetProjectID("proj-1")
	if session.ProjectID != "proj-1" {
		t.Errorf("ProjectID = %q", session.ProjectID)
	}
	session.SetToken("new-tok")
	if client.Token != "new-tok" {
		t.Errorf("client.Token = %q", client.Token)
	}
}

func TestSession_SendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "session reply"}},
			},
		})
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	session.SetModel("m")
	session.SetProjectID("p")
	turn, err := session.SendMessage(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if turn.VisibleText != "session reply" {
		t.Errorf("VisibleText = %q", turn.VisibleText)
	}
}

func TestSession_SendMessage_ResponsesTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "resp-id",
			"output": []map[string]any{{"type": "text", "text": "responses"}},
		})
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSessionWithResponses(client)
	turn, err := session.SendMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if turn.VisibleText != "responses" || turn.ResponseID != "resp-id" {
		t.Errorf("turn = %+v", turn)
	}
}

func TestSession_NewThread(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/threads" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"thread_id": "tid-1"})
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	session.SetProjectID("proj")
	threadID, err := session.NewThread()
	if err != nil {
		t.Fatalf("NewThread: %v", err)
	}
	if threadID != "tid-1" {
		t.Errorf("threadID = %q", threadID)
	}
	if session.CurrentThreadID != "tid-1" {
		t.Errorf("CurrentThreadID = %q", session.CurrentThreadID)
	}
}

func TestSession_ListThreads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/threads" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"id": "t1", "title": "A", "created_at": "", "updated_at": ""}},
		})
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	list, err := session.ListThreads(10, 0)
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	if len(list) != 1 || list[0].ID != "t1" {
		t.Errorf("list = %+v", list)
	}
}

func TestSession_PatchThreadTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/threads/t1" || r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	err := session.PatchThreadTitle("t1", "Title")
	if err != nil {
		t.Fatalf("PatchThreadTitle: %v", err)
	}
}

func TestSession_SetCurrentThreadID(t *testing.T) {
	session := NewSession(gateway.NewClient("http://localhost"))
	session.SetCurrentThreadID("thread-abc")
	if session.CurrentThreadID != "thread-abc" {
		t.Errorf("CurrentThreadID = %q", session.CurrentThreadID)
	}
}

func TestSession_SetToken_NilClient(t *testing.T) {
	session := &Session{}
	session.SetToken("x")
}

func TestSession_SetClient(t *testing.T) {
	oldClient := gateway.NewClient("http://old")
	oldClient.SetToken("old-tok")
	session := NewSession(oldClient)
	newClient := gateway.NewClient("http://new")
	newClient.SetToken("new-tok")
	session.SetClient(newClient)
	if session.Client != newClient {
		t.Errorf("Session.Client = %p, want %p", session.Client, newClient)
	}
	if comp, ok := session.Transport.(*CompletionsTransport); ok && comp.Client != newClient {
		t.Errorf("CompletionsTransport.Client = %p, want %p (transport must use new client)", comp.Client, newClient)
	}
}

func TestSession_StreamMessage(t *testing.T) {
	sseData := func(content, finishReason string) string {
		fr := "null"
		if finishReason != "" {
			fr = `"` + finishReason + `"`
		}
		return fmt.Sprintf(`data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":%q},"finish_reason":%s}]}`,
			content, fr) + "\n\n"
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseData("stream reply", "")))
		_, _ = w.Write([]byte(sseData("", "stop")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	ch, err := session.StreamMessage(context.Background(), "hello")
	if err != nil {
		t.Fatalf("StreamMessage: %v", err)
	}
	var buf strings.Builder
	for ev := range ch {
		if ev.Done {
			if ev.Err != nil {
				t.Errorf("Done err = %v", ev.Err)
			}
			break
		}
		buf.WriteString(ev.Delta)
	}
	if buf.String() != "stream reply" {
		t.Errorf("accumulated = %q, want %q", buf.String(), "stream reply")
	}
}
