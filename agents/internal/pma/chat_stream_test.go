package pma

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	lcagents "github.com/tmc/langchaingo/agents"
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

func TestEmitNDJSONEmissions_AllKindsAndSkips(t *testing.T) {
	rec := httptest.NewRecorder()
	var enc *json.Encoder
	started := false
	emissions := []streamEmitted{
		{Kind: streamEmitDelta, Text: ""},
		{Kind: streamEmitThinking, Text: "  \t"},
		{Kind: streamEmitDelta, Text: "a"},
		{Kind: streamEmitThinking, Text: "note"},
		{Kind: streamEmitToolCall, Text: `{"k":1}`},
		{Kind: streamEmitKind("unknown"), Text: "ignored"},
	}
	emitted, hdrSent, err := emitNDJSONEmissions(rec, &enc, &started, emissions)
	if err != nil {
		t.Fatal(err)
	}
	if !emitted {
		t.Fatal("expected at least one emission")
	}
	if hdrSent {
		t.Fatal("headersSentOnErr should be false on success")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "iteration_start") || !strings.Contains(body, `"delta":"a"`) {
		t.Fatalf("body missing expected lines: %s", body)
	}
	if !strings.Contains(body, "thinking") || !strings.Contains(body, "tool_call") {
		t.Fatalf("body missing thinking/tool_call: %s", body)
	}
}

func TestStreamLangchainNDJSONToWriter_OK(t *testing.T) {
	rec := httptest.NewRecorder()
	out := streamLangchainNDJSONToWriter(rec, "visible-out", "think-bit", slog.Default())
	if out != streamNDJSONOK {
		t.Fatalf("want streamNDJSONOK, got %v", out)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	b := rec.Body.String()
	if !strings.Contains(b, "visible-out") {
		t.Fatalf("body: %s", b)
	}
}

func TestStreamCapableModelNDJSON_LangchainSuccess(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"langchain-stream-ok"}}
	defer func() { testLLMForCompletion = oldHook }()
	rec := httptest.NewRecorder()
	req := &InternalChatCompletionRequest{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: "hi"}},
	}
	out := streamCapableModelNDJSON(context.Background(), rec, "sys", req, NewMCPClient(), "qwen3.5:9b", slog.Default())
	if out != streamNDJSONOK {
		t.Fatalf("want streamNDJSONOK, got %v", out)
	}
	if !strings.Contains(rec.Body.String(), "langchain-stream-ok") {
		t.Fatalf("body: %s", rec.Body.String())
	}
}

func TestStreamCapableModelNDJSON_HardLangchainError(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{errs: []error{errors.New("hard langchain failure")}}
	defer func() { testLLMForCompletion = oldHook }()
	rec := httptest.NewRecorder()
	req := &InternalChatCompletionRequest{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: "hi"}},
	}
	out := streamCapableModelNDJSON(context.Background(), rec, "sys", req, NewMCPClient(), "qwen3.5:9b", nil)
	if out != streamNDJSONError {
		t.Fatalf("want streamNDJSONError, got %v", out)
	}
}

func TestStreamCapableModelNDJSON_AgentNotFinishedFallsBackToOllama(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	ollama := newMockOllamaStreamContentChunksServer(t, []string{"fallback-delta"})
	defer ollama.Close()
	t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	t.Setenv("OLLAMA_BASE_URL", ollama.URL)
	t.Setenv("INFERENCE_URL", "")
	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{errs: []error{lcagents.ErrNotFinished}}
	defer func() { testLLMForCompletion = oldHook }()
	rec := httptest.NewRecorder()
	req := &InternalChatCompletionRequest{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: "hi"}},
	}
	out := streamCapableModelNDJSON(context.Background(), rec, "sys", req, NewMCPClient(), "qwen3.5:9b", slog.Default())
	if out != streamNDJSONOK {
		t.Fatalf("want streamNDJSONOK after Ollama fallback, got %v", out)
	}
	if !strings.Contains(rec.Body.String(), "fallback-delta") {
		t.Fatalf("body: %s", rec.Body.String())
	}
}

func TestStreamOllamaChatToNDJSONOutcome_EmitsToolCallNDJSON(t *testing.T) {
	open := "\u003ctool_call\u003e"
	close := "\u003c/tool_call\u003e"
	srv := newMockOllamaStreamContentChunksServer(t, []string{open, `{"fn":1}`, close, " tail"})
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
	if out != streamNDJSONOK {
		t.Fatalf("want streamNDJSONOK, got %v", out)
	}
	b := rec.Body.String()
	if !strings.Contains(b, "tool_call") || !strings.Contains(b, " tail") {
		t.Fatalf("expected tool_call and trailing delta in body: %s", b)
	}
}

// newMockOllamaStreamContentChunksServer streams Ollama-style NDJSON lines with message.content per chunk; last chunk sets done:true.
func newMockOllamaStreamContentChunksServer(t *testing.T, contents []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for i, c := range contents {
			done := i == len(contents)-1
			line, err := json.Marshal(map[string]interface{}{
				"message": map[string]string{"content": c},
				"done":    done,
			})
			if err != nil {
				t.Error(err)
				return
			}
			_, _ = w.Write(line)
			_, _ = w.Write([]byte("\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
}
