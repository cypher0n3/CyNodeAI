package pma

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStreamOllamaChatToNDJSONOutcome_InferenceNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)
	t.Setenv("INFERENCE_URL", "")
	rec := httptest.NewRecorder()
	out := streamOllamaChatToNDJSONOutcome(context.Background(), rec, "sys", &InternalChatCompletionRequest{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: "hi"}},
	}, slog.Default())
	if out != streamNDJSONError {
		t.Fatalf("got %v, want streamNDJSONError", out)
	}
}

func TestStreamOllamaChatToNDJSONOutcome_EmptyStream(t *testing.T) {
	mockInference := newMockOllamaNDJSONEmptyDoneServer(t)
	defer mockInference.Close()
	t.Setenv("OLLAMA_BASE_URL", mockInference.URL)
	t.Setenv("INFERENCE_URL", "")
	rec := httptest.NewRecorder()
	out := streamOllamaChatToNDJSONOutcome(context.Background(), rec, "sys", &InternalChatCompletionRequest{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: "hi"}},
	}, slog.Default())
	if out != streamNDJSONEmpty {
		t.Fatalf("got %v, want streamNDJSONEmpty", out)
	}
}

func TestStreamTryLangchainNDJSON_NoMCPUsesOllamaPath(t *testing.T) {
	t.Setenv("PMA_MCP_GATEWAY_URL", "")
	t.Setenv("MCP_GATEWAY_URL", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)
	rec := httptest.NewRecorder()
	out := streamTryLangchainNDJSON(context.Background(), rec, "sys", &InternalChatCompletionRequest{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: "hi"}},
	}, slog.Default())
	if out != streamNDJSONError {
		t.Fatalf("got %v, want streamNDJSONError", out)
	}
}

// TestChatCompletionHandler_StreamLangchain_EmptyStreamsRetries exercises streamCompletionLangchainToWriter
// when both the langchain attempt and the Ollama NDJSON fallback emit no deltas (streamNDJSONEmpty → retry → 500).
func TestChatCompletionHandler_LangchainHardError(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"non_stream", `{"messages":[{"role":"user","content":"hi"}]}`},
		{"stream", `{"messages":[{"role":"user","content":"hi"}],"stream":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mcpSrv := newMockMCPServer(t, `{}`)
			defer mcpSrv.Close()
			t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
			t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:1")
			t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")
			oldHook := testLLMForCompletion
			testLLMForCompletion = &mockLLM{errs: []error{errors.New("injected llm failure")}}
			defer func() { testLLMForCompletion = oldHook }()
			handler := ChatCompletionHandler("sys", slog.Default())
			req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(tc.body)))
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestChatCompletionHandler_StreamLangchain_EmptyStreamsRetries(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	ollama := newMockOllamaNDJSONEmptyDoneServer(t)
	defer ollama.Close()
	t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	t.Setenv("OLLAMA_BASE_URL", ollama.URL)
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")
	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"", ""}}
	defer func() { testLLMForCompletion = oldHook }()
	handler := ChatCompletionHandler("sys", slog.Default())
	body := `{"messages":[{"role":"user","content":"hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after empty stream retry, got %d: %s", rec.Code, rec.Body.String())
	}
}
