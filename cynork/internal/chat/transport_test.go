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

const testPathCompletions = "/v1/chat/completions"
const testPathResponses = "/v1/responses"

func TestCompletionsTransport_SendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testPathCompletions || r.Method != http.MethodPost {
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
		if r.URL.Path != testPathResponses || r.Method != http.MethodPost {
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

// sseChunk builds an SSE data line for a chat.completion.chunk with the given content.
func sseChunk(content, finishReason string) string {
	fr := "null"
	if finishReason != "" {
		fr = `"` + finishReason + `"`
	}
	return fmt.Sprintf(`data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":%q},"finish_reason":%s}]}`,
		content, fr) + "\n\n"
}

func TestCompletionsTransport_StreamMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testPathCompletions || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseChunk("hello", "")))
		_, _ = w.Write([]byte(sseChunk(" world", "")))
		_, _ = w.Write([]byte(sseChunk("", "stop")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	transport := &CompletionsTransport{Client: client}
	ch, err := transport.StreamMessage(context.Background(), "hi", "m", "p")
	if err != nil {
		t.Fatalf("StreamMessage: %v", err)
	}
	var buf strings.Builder
	for ev := range ch {
		if ev.Done {
			if ev.Err != nil {
				t.Errorf("Done event err = %v", ev.Err)
			}
			break
		}
		buf.WriteString(ev.Delta)
	}
	if buf.String() != "hello world" {
		t.Errorf("accumulated = %q, want %q", buf.String(), "hello world")
	}
}

func TestResponsesTransport_StreamMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testPathResponses || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseChunk("resp", "")))
		_, _ = w.Write([]byte(sseChunk("", "stop")))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	client := gateway.NewClient(server.URL)
	client.SetToken("tok")
	transport := &ResponsesTransport{Client: client}
	ch, err := transport.StreamMessage(context.Background(), "hi", "m", "p")
	if err != nil {
		t.Fatalf("StreamMessage: %v", err)
	}
	var buf strings.Builder
	for ev := range ch {
		if ev.Done {
			break
		}
		buf.WriteString(ev.Delta)
	}
	if buf.String() != "resp" {
		t.Errorf("accumulated = %q, want %q", buf.String(), "resp")
	}
}

func TestTransport_StreamMessage_Error(t *testing.T) {
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
			ch, err := transport.StreamMessage(context.Background(), "hi", "", "")
			if err != nil {
				return // error at call time is also acceptable
			}
			var gotErr bool
			for ev := range ch {
				if ev.Done && ev.Err != nil {
					gotErr = true
				}
			}
			if !gotErr {
				t.Error("expected an error in stream")
			}
		})
	}
}
