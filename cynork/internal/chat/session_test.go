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

const pathV1ChatThreads = "/v1/chat/threads"

func TestNewSession(t *testing.T) {
	client := gateway.NewClient("http://localhost")
	session := NewSession(client)
	if session.Client != client || session.Transport == nil {
		t.Errorf("NewSession: %+v", session)
	}
	if session.Model != gateway.ModelProjectManager {
		t.Errorf("NewSession Model = %q, want %q", session.Model, gateway.ModelProjectManager)
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
		if r.URL.Path != pathV1ChatThreads || r.Method != http.MethodPost {
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
		if r.URL.Path != pathV1ChatThreads || r.Method != http.MethodGet {
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

func patchThreadTitleOKServer(expectPath string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectPath || r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func TestSession_PatchThreadTitle(t *testing.T) {
	server := patchThreadTitleOKServer("/v1/chat/threads/t1")
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	err := session.PatchThreadTitle("t1", "Title")
	if err != nil {
		t.Fatalf("PatchThreadTitle: %v", err)
	}
}

func TestSession_PatchThreadTitle_UsesCurrentWhenThreadIDEmpty(t *testing.T) {
	server := patchThreadTitleOKServer("/v1/chat/threads/cur-tid")
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	session.SetCurrentThreadID("cur-tid")
	if err := session.PatchThreadTitle("", "Renamed"); err != nil {
		t.Fatalf("PatchThreadTitle: %v", err)
	}
}

func TestSession_ResolveThreadSelector_EmptySelector(t *testing.T) {
	session := NewSession(gateway.NewClient("http://localhost"))
	id, err := session.ResolveThreadSelector("", 10)
	if err != nil || id != "" {
		t.Fatalf("ResolveThreadSelector empty: id=%q err=%v", id, err)
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

func TestSession_ResolveThreadSelector_OrdinalAndID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathV1ChatThreads {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "tid-first", "title": "First"},
				{"id": "tid-second", "title": "Second"},
			},
		})
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	id, err := session.ResolveThreadSelector("1", 10)
	if err != nil {
		t.Fatalf("ResolveThreadSelector(1): %v", err)
	}
	if id != "tid-first" {
		t.Errorf("ordinal 1 = %q, want tid-first", id)
	}
	id, err = session.ResolveThreadSelector("tid-second", 10)
	if err != nil {
		t.Fatalf("ResolveThreadSelector(tid-second): %v", err)
	}
	if id != "tid-second" {
		t.Errorf("id = %q, want tid-second", id)
	}
}

func TestSession_EnsureThread_NewAndResume(t *testing.T) {
	var created bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathV1ChatThreads && r.Method == http.MethodPost {
			created = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"thread_id": "new-tid"})
			return
		}
		if r.URL.Path == pathV1ChatThreads && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "resumed-tid", "title": "Resumed"}},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	if err := session.EnsureThread(""); err != nil {
		t.Fatalf("EnsureThread() new: %v", err)
	}
	if !created || session.CurrentThreadID != "new-tid" {
		t.Errorf("created=%v CurrentThreadID=%q", created, session.CurrentThreadID)
	}
	if err := session.EnsureThread("1"); err != nil {
		t.Fatalf("EnsureThread(1): %v", err)
	}
	if session.CurrentThreadID != "resumed-tid" {
		t.Errorf("resume CurrentThreadID = %q", session.CurrentThreadID)
	}
}

func TestSession_EnsureThread_SkipsNewWhenThreadAlreadySet(t *testing.T) {
	var postCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathV1ChatThreads && r.Method == http.MethodPost {
			postCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"thread_id": "should-not-be-used"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	session.CurrentThreadID = "existing-tid"
	if err := session.EnsureThread(""); err != nil {
		t.Fatalf("EnsureThread: %v", err)
	}
	if postCount != 0 {
		t.Errorf("NewThread POST count = %d, want 0 (keep existing thread)", postCount)
	}
	if session.CurrentThreadID != "existing-tid" {
		t.Errorf("CurrentThreadID = %q, want existing-tid", session.CurrentThreadID)
	}
}

func TestSession_SetClient_ResponsesTransport(t *testing.T) {
	oldClient := gateway.NewClient("http://old")
	session := NewSessionWithResponses(oldClient)
	newClient := gateway.NewClient("http://new")
	session.SetClient(newClient)
	if session.Client != newClient {
		t.Errorf("Client = %p, want %p", session.Client, newClient)
	}
	if rt, ok := session.Transport.(*ResponsesTransport); ok && rt.Client != newClient {
		t.Errorf("ResponsesTransport.Client = %p, want %p", rt.Client, newClient)
	}
}

func TestSession_NewThread_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	_, err := session.NewThread()
	if err == nil {
		t.Error("expected error from NewThread on 500 response")
	}
}

func TestSession_PatchThreadTitle_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	err := session.PatchThreadTitle("t1", "Title")
	if err == nil {
		t.Error("expected error from PatchThreadTitle on 404")
	}
}

func TestSession_ResolveThreadSelector_ListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	_, err := session.ResolveThreadSelector("1", 10)
	if err == nil {
		t.Error("expected error from ResolveThreadSelector when ListThreads fails")
	}
}

func TestResolveThreadSelectorFromItems_TitleMatch(t *testing.T) {
	title := "My Thread"
	items := []gateway.ChatThreadItem{{ID: "t1", Title: &title}}
	id, err := resolveThreadSelectorFromItems("my thread", items)
	if err != nil {
		t.Fatalf("resolveThreadSelectorFromItems: %v", err)
	}
	if id != "t1" {
		t.Errorf("id = %q, want t1", id)
	}
}

func TestResolveThreadSelectorFromItems_NoMatch(t *testing.T) {
	title := "Other"
	items := []gateway.ChatThreadItem{{ID: "t1", Title: &title}}
	_, err := resolveThreadSelectorFromItems("nonexistent", items)
	if err == nil {
		t.Error("expected error for no match")
	}
}

func TestResolveThreadSelectorFromItems_EmptyList(t *testing.T) {
	_, err := resolveThreadSelectorFromItems("1", nil)
	if err == nil {
		t.Error("expected error for empty item list")
	}
}

func TestResolveThreadSelectorFromItems_IDPrefix(t *testing.T) {
	items := []gateway.ChatThreadItem{{ID: "tid-abc-123"}}
	id, err := resolveThreadSelectorFromItems("tid-abc", items)
	if err != nil {
		t.Fatalf("resolveThreadSelectorFromItems: %v", err)
	}
	if id != "tid-abc-123" {
		t.Errorf("id = %q, want tid-abc-123", id)
	}
}

func TestSession_StreamMessage_ResponsesTransport_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSessionWithResponses(client)
	ch, err := session.StreamMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("StreamMessage initial: %v", err)
	}
	var lastDelta ChatStreamDelta
	for d := range ch {
		lastDelta = d
	}
	if !lastDelta.Done || lastDelta.Err == nil {
		t.Errorf("expected Done with error; got %+v", lastDelta)
	}
}

func TestSession_EnsureThread_ResolveError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	session := NewSession(client)
	err := session.EnsureThread("some-selector")
	if err == nil {
		t.Error("expected error from EnsureThread when resolve fails")
	}
}
