package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

// TestOpenAIChatHandler_ChatCompletions_Stream_Success verifies that stream=true returns
// Server-Sent Events with the expected chat.completion.chunk format and [DONE].
func TestOpenAIChatHandler_ChatCompletions_Stream_Success(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "SSE reply.", "done": true})
	}))
	defer mockOllama.Close()
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), mockOllama.URL, "m", "")
	body := []byte(`{"model":"m","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).
		WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if ct != mimeSSE {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	body2 := rec.Body.String()
	if !strings.Contains(body2, "data:") {
		t.Errorf("stream response missing data: lines: %s", body2)
	}
	if !strings.Contains(body2, "[DONE]") {
		t.Errorf("stream response missing [DONE]: %s", body2)
	}
}

// TestOpenAIChatHandler_ChatCompletions_Stream_PMAUnavailable verifies that stream=true with
// PMA unavailable returns an SSE error event followed by [DONE].
func TestOpenAIChatHandler_ChatCompletions_Stream_PMAUnavailable(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	body := []byte(`{"model":"cynodeai.pm","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).
		WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	ct := rec.Header().Get("Content-Type")
	if ct != mimeSSE {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	body2 := rec.Body.String()
	if !strings.Contains(body2, "[DONE]") {
		t.Errorf("stream error response missing [DONE]: %s", body2)
	}
}

// TestOpenAIChatHandler_Responses_Stream_Success verifies that stream=true on /v1/responses
// returns SSE events.
func TestOpenAIChatHandler_Responses_Stream_Success(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "stream resp", "done": true})
	}))
	defer mockOllama.Close()
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), mockOllama.URL, "m", "")
	body := []byte(`{"model":"m","stream":true,"input":"hello"}`)
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Responses(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if ct != mimeSSE {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	body2 := rec.Body.String()
	if !strings.Contains(body2, "[DONE]") {
		t.Errorf("stream response missing [DONE]: %s", body2)
	}
}

// TestOpenAIChatHandler_Responses_Stream_PMAUnavailable verifies SSE error response for
// streaming Responses when PMA is unavailable.
func TestOpenAIChatHandler_Responses_Stream_PMAUnavailable(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	body := []byte(`{"model":"cynodeai.pm","stream":true,"input":"hello"}`)
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Responses(rec, req)
	ct := rec.Header().Get("Content-Type")
	if ct != mimeSSE {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	body2 := rec.Body.String()
	if !strings.Contains(body2, "[DONE]") {
		t.Errorf("stream error response missing [DONE]: %s", body2)
	}
}
