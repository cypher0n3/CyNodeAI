package pmaclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallChatCompletion_EmptyURL(t *testing.T) {
	_, err := CallChatCompletion(context.Background(), nil, "", []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for empty base URL")
	}
}

func TestCallChatCompletion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/chat/completion" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req CompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CompletionResponse{Content: "ok"})
	}))
	defer server.Close()

	content, err := CallChatCompletion(context.Background(), nil, server.URL, []ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("CallChatCompletion: %v", err)
	}
	if content != "ok" {
		t.Errorf("content want ok, got %q", content)
	}
}

func TestCallChatCompletion_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := CallChatCompletion(context.Background(), nil, server.URL, []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestCallChatCompletion_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	_, err := CallChatCompletion(context.Background(), nil, server.URL, []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestCallChatCompletion_WithCustomClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(CompletionResponse{Content: "custom"})
	}))
	defer server.Close()

	client := &http.Client{}
	content, err := CallChatCompletion(context.Background(), client, server.URL, []ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("CallChatCompletion: %v", err)
	}
	if content != "custom" {
		t.Errorf("content want custom, got %q", content)
	}
}

func TestCallChatCompletion_DoError(t *testing.T) {
	// Use a URL that will fail on Do (connection refused or no route).
	_, err := CallChatCompletion(context.Background(), nil, "http://127.0.0.1:19999", []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error when server unreachable")
	}
}
