package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
}

func TestSession_SetToken_NilClient(t *testing.T) {
	session := &Session{}
	session.SetToken("x")
}
