package pma

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestChatCompletionHandler_MethodNotAllowed(t *testing.T) {
	handler := ChatCompletionHandler("", slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/internal/chat/completion", http.NoBody)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want 405", rec.Code)
	}
}

func TestChatCompletionHandler_BadRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"bad JSON", "{"},
		{"empty messages", `{"messages":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ChatCompletionHandler("", slog.Default())
			req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("got status %d, want 400", rec.Code)
			}
		})
	}
}

func TestChatCompletionHandler_Success(t *testing.T) {
	mockInference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":"hello"}`))
	}))
	defer mockInference.Close()
	oldURL := os.Getenv("OLLAMA_BASE_URL")
	_ = os.Setenv("OLLAMA_BASE_URL", mockInference.URL)
	defer func() {
		if oldURL != "" {
			_ = os.Setenv("OLLAMA_BASE_URL", oldURL)
		} else {
			_ = os.Unsetenv("OLLAMA_BASE_URL")
		}
	}()

	handler := ChatCompletionHandler("sys", slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"content":"hello"`)) {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestChatCompletionHandler_InferenceErrorField(t *testing.T) {
	mockInference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":"","error":"model not found"}`))
	}))
	defer mockInference.Close()
	oldURL := os.Getenv("OLLAMA_BASE_URL")
	_ = os.Setenv("OLLAMA_BASE_URL", mockInference.URL)
	defer func() {
		if oldURL != "" {
			_ = os.Setenv("OLLAMA_BASE_URL", oldURL)
		} else {
			_ = os.Unsetenv("OLLAMA_BASE_URL")
		}
	}()

	handler := ChatCompletionHandler("", slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500", rec.Code)
	}
}

func TestChatCompletionHandler_InferenceError(t *testing.T) {
	mockInference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockInference.Close()
	oldURL := os.Getenv("OLLAMA_BASE_URL")
	_ = os.Setenv("OLLAMA_BASE_URL", mockInference.URL)
	defer func() {
		if oldURL != "" {
			_ = os.Setenv("OLLAMA_BASE_URL", oldURL)
		} else {
			_ = os.Unsetenv("OLLAMA_BASE_URL")
		}
	}()

	handler := ChatCompletionHandler("", slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500", rec.Code)
	}
}

func TestBuildSystemContext(t *testing.T) {
	base := "base"
	if got := buildSystemContext(base, &InternalChatCompletionRequest{}); got != base {
		t.Errorf("buildSystemContext(instructions only) = %q, want base", got)
	}
	req := &InternalChatCompletionRequest{ProjectID: "p1"}
	if got := buildSystemContext(base, req); !strings.Contains(got, "Project context") || !strings.Contains(got, "p1") {
		t.Errorf("buildSystemContext(with project) = %q", got)
	}
	req = &InternalChatCompletionRequest{TaskID: "t1"}
	if got := buildSystemContext(base, req); !strings.Contains(got, "Task context") || !strings.Contains(got, "t1") {
		t.Errorf("buildSystemContext(with task) = %q", got)
	}
	req = &InternalChatCompletionRequest{AdditionalContext: "extra"}
	if got := buildSystemContext(base, req); !strings.Contains(got, "User additional context") || !strings.Contains(got, "extra") {
		t.Errorf("buildSystemContext(with additional) = %q", got)
	}
}
