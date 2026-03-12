package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

func TestCompletionsTransport_SendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "completion reply"}},
			},
		})
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	transport := &CompletionsTransport{Client: client}
	turn, err := transport.SendMessage(context.Background(), "hi", "m", "p")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if turn.VisibleText != "completion reply" {
		t.Errorf("VisibleText = %q", turn.VisibleText)
	}
}

func TestTransport_SendMessage_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	for name, transport := range map[string]ChatTransport{
		"Completions": &CompletionsTransport{Client: client},
		"Responses":   &ResponsesTransport{Client: client},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := transport.SendMessage(context.Background(), "hi", "", "")
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestResponsesTransport_SendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "r-1",
			"output": []map[string]any{{"type": "text", "text": "responses reply"}},
		})
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	transport := &ResponsesTransport{Client: client}
	turn, err := transport.SendMessage(context.Background(), "hi", "m", "p")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if turn.VisibleText != "responses reply" || turn.ResponseID != "r-1" {
		t.Errorf("turn = %+v", turn)
	}
}
