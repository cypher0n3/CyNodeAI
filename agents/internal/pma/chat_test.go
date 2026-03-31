package pma

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// newMockInferenceServer starts an httptest.Server that responds to /api/chat.
// When the request has "stream": false (callInference), it returns a single JSON object
// with message.content from the mock body. When stream is true or omitted (Ollama default),
// it emits Ollama-compatible NDJSON for streamCompletionToWriter.
func newMockInferenceServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(mockInferenceHandler(status, body))
}

func mockInferenceUseStream(reqBody []byte) bool {
	var req struct {
		Stream *bool `json:"stream"`
	}
	_ = json.Unmarshal(reqBody, &req)
	return req.Stream == nil || *req.Stream
}

type mockInferenceParsed struct {
	content   string
	errField  string
	badConfig bool
}

func mockInferenceParseConfigJSON(body string) mockInferenceParsed {
	var single struct {
		Response string `json:"response"`
		Message  struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &single); err != nil {
		return mockInferenceParsed{badConfig: true}
	}
	if single.Error != "" {
		return mockInferenceParsed{errField: single.Error}
	}
	content := single.Message.Content
	if content == "" {
		content = single.Response
	}
	return mockInferenceParsed{content: content}
}

func mockInferenceWriteJSONError(w http.ResponseWriter, useStream bool, msg string) {
	if useStream {
		_, _ = w.Write([]byte(`{"error":"` + msg + `","done":true}` + "\n"))
		return
	}
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}

func mockInferenceWriteNonStreamOK(w http.ResponseWriter, content string) {
	out, _ := json.Marshal(map[string]interface{}{
		"message": map[string]string{"role": "assistant", "content": content},
		"done":    true,
	})
	_, _ = w.Write(out)
}

func mockInferenceWriteStreamOK(w http.ResponseWriter, content string) {
	chunk, _ := json.Marshal(map[string]interface{}{
		"message": map[string]string{"content": content},
		"done":    false,
	})
	_, _ = w.Write(chunk)
	_, _ = w.Write([]byte("\n"))
	done, _ := json.Marshal(map[string]interface{}{
		"message": map[string]string{"content": ""},
		"done":    true,
	})
	_, _ = w.Write(done)
	_, _ = w.Write([]byte("\n"))
}

func mockInferenceHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = w.Write([]byte(body))
			return
		}
		reqBody, _ := io.ReadAll(r.Body)
		useStream := mockInferenceUseStream(reqBody)
		parsed := mockInferenceParseConfigJSON(body)
		if parsed.badConfig {
			mockInferenceWriteJSONError(w, useStream, "mock parse error")
			return
		}
		if parsed.errField != "" {
			mockInferenceWriteJSONError(w, useStream, parsed.errField)
			return
		}
		if !useStream {
			mockInferenceWriteNonStreamOK(w, parsed.content)
			return
		}
		mockInferenceWriteStreamOK(w, parsed.content)
	}
}

func TestChatCompletionHandler_MethodNotAllowed(t *testing.T) {
	handler := ChatCompletionHandler("", slog.Default(), NewChatDepsFromEnv())
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
			handler := ChatCompletionHandler("", slog.Default(), NewChatDepsFromEnv())
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
	mockInference := newMockInferenceServer(t, http.StatusOK, `{"response":"hello"}`)
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

	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
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
	mockInference := newMockInferenceServer(t, http.StatusOK, `{"response":"","error":"model not found"}`)
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

	handler := ChatCompletionHandler("", slog.Default(), NewChatDepsFromEnv())
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

	handler := ChatCompletionHandler("", slog.Default(), NewChatDepsFromEnv())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500", rec.Code)
	}
}

func TestPrepareChatCompletionRequest_MergesFilesIntoLastUser(t *testing.T) {
	req := &InternalChatCompletionRequest{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "user", Content: "hello"},
		},
		ChatFiles: []ChatFileRef{
			{Name: "notes.txt", MIMEType: "text/plain", Text: "line1"},
		},
	}
	if err := prepareChatCompletionRequest(req); err != nil {
		t.Fatal(err)
	}
	if len(req.ChatFiles) != 0 {
		t.Fatal("ChatFiles should be cleared after merge")
	}
	if !strings.Contains(req.Messages[0].Content, "## Chat file: notes.txt") || !strings.Contains(req.Messages[0].Content, "line1") {
		t.Fatalf("content = %q", req.Messages[0].Content)
	}
}

func TestPrepareChatCompletionRequest_UnsupportedMIME(t *testing.T) {
	req := &InternalChatCompletionRequest{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: "x"}},
		ChatFiles: []ChatFileRef{
			{Name: "b.bin", MIMEType: "application/octet-stream", Text: "x"},
		},
	}
	if err := prepareChatCompletionRequest(req); err == nil {
		t.Fatal("expected error")
	}
}

func TestChatCompletionHandler_UnsupportedChatFileMIME(t *testing.T) {
	handler := ChatCompletionHandler("", slog.Default(), NewChatDepsFromEnv())
	body := `{"messages":[{"role":"user","content":"hi"}],"chat_files":[{"name":"x.bin","mime_type":"application/octet-stream","text":"data"}]}`
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", rec.Code)
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

// TestBuildSystemContext_CompositionOrder verifies REQ-PMAGNT-0108 / CYNAI.PMAGNT.LLMContextComposition:
// order is baseline+role -> project -> task -> user additional context.
func TestBuildSystemContext_CompositionOrder(t *testing.T) {
	baseline := "baseline and role instructions"
	req := &InternalChatCompletionRequest{
		ProjectID:         "proj-1",
		TaskID:            "task-1",
		AdditionalContext: "user extra",
	}
	got := buildSystemContext(baseline, req)
	// Must start with baseline.
	if !strings.HasPrefix(got, strings.TrimSpace(baseline)) {
		t.Errorf("context must start with baseline, got prefix %q", got[:min(50, len(got))])
	}
	// Required order: Project before Task before User additional context.
	idxProject := strings.Index(got, "## Project context")
	idxTask := strings.Index(got, "## Task context")
	idxAdditional := strings.Index(got, "## User additional context")
	if idxProject < 0 || idxTask < 0 || idxAdditional < 0 {
		t.Errorf("missing section: project=%d task=%d additional=%d", idxProject, idxTask, idxAdditional)
	}
	if idxProject >= idxTask || idxTask >= idxAdditional {
		t.Errorf("wrong order: project=%d task=%d additional=%d (must be project < task < additional)", idxProject, idxTask, idxAdditional)
	}
	if !strings.Contains(got, "proj-1") || !strings.Contains(got, "task-1") || !strings.Contains(got, "user extra") {
		t.Errorf("context must contain request ids and additional text")
	}
}

func TestChatCompletionHandler_SuccessWithMCPPath(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()

	t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("INFERENCE_URL", "")
	// Use a capable model so the OpenAIFunctionsAgent path is exercised.
	// OpenAIFunctionsAgent returns choice.Content directly (no "Final Answer:" prefix).
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")

	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"pma response"}}
	defer func() { testLLMForCompletion = oldHook }()

	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, body %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"content":"pma response"`)) {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// TestChatCompletionHandler_StreamLangchainPath exercises streamCompletionLangchainToWriter
// when Stream=true and canStreamCompletion is false (MCP + capable model).
func TestChatCompletionHandler_StreamLangchainPath(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()

	t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")

	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"streamed"}}
	defer func() { testLLMForCompletion = oldHook }()

	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
	body := `{"messages":[{"role":"user","content":"hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/x-ndjson" {
		t.Errorf("Content-Type = %s", rec.Header().Get("Content-Type"))
	}
	b := rec.Body.Bytes()
	if !bytes.Contains(b, []byte(`"iteration_start"`)) {
		t.Errorf("body missing iteration_start: %s", b)
	}
	if !bytes.Contains(b, []byte(`"delta"`)) || !bytes.Contains(b, []byte("streamed")) {
		t.Errorf("body missing delta with completion: %s", b)
	}
	if !bytes.Contains(b, []byte(`"done"`)) {
		t.Errorf("body missing done: %s", b)
	}
}

// TestChatCompletionHandler_SmallModelDirectGeneration verifies that when INFERENCE_MODEL
// is a small-variant model (e.g. qwen3.5:0.8b), the direct callInference path is used
// (bypassing langchaingo, which cannot reliably return content for Qwen3.5 thinking mode
// because its ChatRequest.Stream is omitempty and defaults to streaming where content
// chunks are empty). The handler should call Ollama /api/chat directly with stream:false.
func TestChatCompletionHandler_SmallModelDirectGeneration(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"hello from qwen3.5"},"done":true}`))
	}))
	defer mockOllama.Close()

	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	t.Setenv("OLLAMA_BASE_URL", mockOllama.URL)
	t.Setenv("INFERENCE_MODEL", "qwen3.5:0.8b")

	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, body %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"content":"hello from qwen3.5"`)) {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func testChatCompletionCapableModelEmptyOutput(t *testing.T, reqBody string) {
	t.Helper()
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	ollama := newMockInferenceServer(t, http.StatusOK, `{"message":{"content":""}}`)
	defer ollama.Close()
	t.Setenv("OLLAMA_BASE_URL", ollama.URL)
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")

	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"", ""}}
	defer func() { testLLMForCompletion = oldHook }()

	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, body %s", rec.Code, rec.Body.String())
	}
}

// TestChatCompletionHandler_CapableModelEmptyOutputReturns500 verifies that when a capable
// model's agent loop returns empty output on both the original and retry attempt, a 500 is returned.
func TestChatCompletionHandler_CapableModelEmptyOutputReturns500(t *testing.T) {
	// Direct inference fallback (stream:false) also returns empty body — persistent empty.
	testChatCompletionCapableModelEmptyOutput(t, `{"messages":[{"role":"user","content":"hi"}]}`)
}

// TestChatCompletionHandler_CapableModelEmptyOutputStreamReturns500 verifies stream=true uses
// resolveContent and returns 500 when completion stays empty (no silent 200 with empty body).
func TestChatCompletionHandler_CapableModelEmptyOutputStreamReturns500(t *testing.T) {
	testChatCompletionCapableModelEmptyOutput(t, `{"messages":[{"role":"user","content":"hi"}],"stream":true}`)
}

// TestChatCompletionHandler_CapableModel_UnexecutedToolCallFallback verifies that when the
// agent loop returns a preamble that looks like an unexecuted tool call, the handler falls
// back to direct callInference and returns that result to the caller instead of the preamble.
func TestChatCompletionHandler_CapableModel_UnexecutedToolCallFallback(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()

	// Set up a real mock Ollama server for the callInference fallback.
	fallbackResponse := `{"message":{"role":"assistant","content":"Here are your tasks: none found."}}`
	ollamaSrv := newMockInferenceServer(t, http.StatusOK, fallbackResponse)
	defer ollamaSrv.Close()

	t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	t.Setenv("OLLAMA_BASE_URL", ollamaSrv.URL)
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")

	oldHook := testLLMForCompletion
	// Mock LLM returns a preamble that looks like an unexecuted tool call.
	testLLMForCompletion = &mockLLM{responses: []string{"Let me fetch the task list for you."}}
	defer func() { testLLMForCompletion = oldHook }()

	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion",
		bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"what tasks can you see?"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after fallback, got %d: %s", rec.Code, rec.Body.String())
	}
	var out InternalChatCompletionResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Content != "Here are your tasks: none found." {
		t.Errorf("expected fallback content, got %q", out.Content)
	}
}

// TestChatCompletionHandler_CapableModel_ThinkBlockStripped verifies that <think> blocks
// in a capable model's output are stripped before returning to the caller.
func TestChatCompletionHandler_CapableModel_ThinkBlockStripped(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	t.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")

	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"<think>internal reasoning</think>The answer is 42."}}
	defer func() { testLLMForCompletion = oldHook }()

	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion",
		bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"what is 6x7?"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out InternalChatCompletionResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Content != "The answer is 42." {
		t.Errorf("think block not stripped, got %q", out.Content)
	}
}

func makeMessages(pairs ...string) []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
} {
	var out []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: pairs[i], Content: pairs[i+1]})
	}
	return out
}

func TestBuildSystemContextWithHistory(t *testing.T) {
	// No prior messages — returns systemContext unchanged.
	got := buildSystemContextWithHistory("sys", makeMessages("user", "hi"))
	if got != "sys" {
		t.Errorf("single message: expected %q, got %q", "sys", got)
	}

	// Two prior turns + current user — history section appears.
	msgs := makeMessages("user", "turn1", "assistant", "reply1", "user", "turn2")
	got = buildSystemContextWithHistory("sys", msgs)
	if !strings.Contains(got, "Conversation history") {
		t.Errorf("expected history header, got %q", got)
	}
	if !strings.Contains(got, "user: turn1") {
		t.Errorf("expected first user turn in history, got %q", got)
	}
	if !strings.Contains(got, "assistant: reply1") {
		t.Errorf("expected assistant turn in history, got %q", got)
	}
	// Current user message (turn2) must NOT be in the history section.
	if strings.Contains(got, "turn2") {
		t.Errorf("current user message should not appear in history section, got %q", got)
	}
}

func TestLastUserMessage(t *testing.T) {
	msgs := makeMessages("user", "first", "assistant", "reply", "user", "second")
	if got := lastUserMessage(msgs); got != "second" {
		t.Errorf("lastUserMessage = %q, want %q", got, "second")
	}
	if got := lastUserMessage(makeMessages("user", "only")); got != "only" {
		t.Errorf("single message: got %q", got)
	}
	if got := lastUserMessage(nil); got != "" {
		t.Errorf("nil messages: got %q", got)
	}
}

func TestPriorMessages(t *testing.T) {
	// Only one message — no prior.
	if got := priorMessages(makeMessages("user", "hi")); got != nil {
		t.Errorf("single message: expected nil prior, got %v", got)
	}
	// Three messages: prior should be first two.
	msgs := makeMessages("user", "a", "assistant", "b", "user", "c")
	prior := priorMessages(msgs)
	if len(prior) != 2 {
		t.Fatalf("expected 2 prior messages, got %d", len(prior))
	}
	if prior[0].Content != "a" || prior[1].Content != "b" {
		t.Errorf("prior = %v", prior)
	}
}

func TestBuildAgentInput(t *testing.T) {
	got := buildAgentInput("sys", "hello")
	if !strings.Contains(got, "sys") || !strings.Contains(got, "hello") {
		t.Errorf("buildAgentInput missing parts: %q", got)
	}
	if buildAgentInput("", "hi") != "hi" {
		t.Errorf("empty context should return just the input")
	}
}

// newMockMCPServer starts an httptest.Server that responds 200 for POST /v1/mcp/tools/call with body.
func newMockMCPServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/mcp/tools/call" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

// newMockOllamaNDJSONEmptyDoneServer responds with one NDJSON line: empty assistant content and done:true.
func newMockOllamaNDJSONEmptyDoneServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		done, _ := json.Marshal(map[string]interface{}{
			"message": map[string]string{"content": ""},
			"done":    true,
		})
		_, _ = fmt.Fprintln(w, string(done))
	}))
}

// TestHandoffRequest_MessagesOnly verifies orchestrator handoff format compatibility:
// body with only "messages" (no project_id, task_id, additional_context) decodes and yields baseline-only context.
func TestHandoffRequest_MessagesOnly(t *testing.T) {
	// Same shape as orchestrator pmaclient.CompletionRequest.
	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	var req InternalChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("decode handoff body: %v", err)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "hi" {
		t.Errorf("unexpected decoded request: %+v", req)
	}
	if req.ProjectID != "" || req.TaskID != "" || req.AdditionalContext != "" {
		t.Errorf("optional fields should be empty when not sent: project_id=%q task_id=%q additional_context=%q",
			req.ProjectID, req.TaskID, req.AdditionalContext)
	}
	baseline := "baseline"
	ctx := buildSystemContext(baseline, &req)
	if ctx != baseline {
		t.Errorf("context with messages-only handoff must be baseline only, got %q", ctx)
	}
}

func TestResolveInferenceClient_TCPPassthrough(t *testing.T) {
	url, client := resolveInferenceClient("http://localhost:11434", 0)
	if url != "http://localhost:11434" {
		t.Errorf("expected passthrough URL, got %q", url)
	}
	switch {
	case client == nil:
		t.Fatal("expected non-nil http.Client for TCP URL")
	case client.Transport != nil:
		t.Error("expected nil transport (default) for TCP URL")
	}
}

func TestResolveInferenceClient_UnixSocket(t *testing.T) {
	sockURL := "http+unix://%2Ftmp%2Ftest.sock"
	url, client := resolveInferenceClient(sockURL, 0)
	if url != udsPlainHost {
		t.Errorf("expected %q for unix socket URL, got %q", udsPlainHost, url)
	}
	switch {
	case client == nil:
		t.Fatal("expected non-nil http.Client for unix socket URL")
	case client.Transport == nil:
		t.Error("expected custom transport for unix socket URL")
	}
}

func TestResolveInferenceClient_UnixSocketWithPath(t *testing.T) {
	sockURL := "http+unix://%2Ftmp%2Ftest.sock/some/path"
	url, client := resolveInferenceClient(sockURL, 0)
	if url != udsPlainHost {
		t.Errorf("expected %q (path stripped), got %q", udsPlainHost, url)
	}
	if client == nil || client.Transport == nil {
		t.Error("expected custom transport for unix socket URL with path")
	}
}

func TestResolveInferenceClient_InvalidEncoding(t *testing.T) {
	// Invalid percent-encoding should fall back gracefully.
	sockURL := "http+unix://%%%invalid"
	url, client := resolveInferenceClient(sockURL, 0)
	// Falls back to returning the original URL with a plain client.
	if url != sockURL {
		t.Errorf("expected original URL on bad encoding, got %q", url)
	}
	if client == nil {
		t.Error("expected non-nil client even on bad encoding")
	}
}

func TestChatCompletionHandler_EmptyOutputRetriesWithCurrentMessage(t *testing.T) {
	// First call returns empty message (simulating context-overflow), second returns content.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if callCount == 1 {
			// First Ollama call returns empty message (context overflow).
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":""}}`))
		} else {
			// Retry returns a real answer.
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Hello from retry!"}}`))
		}
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)
	t.Setenv("INFERENCE_MODEL", pmaDefaultModel)

	// Send two messages so the handler has history to potentially overflow on.
	body, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": "first message"},
			{"role": "assistant", "content": "first reply"},
			{"role": "user", "content": "second message"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after retry, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp InternalChatCompletionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Content != "Hello from retry!" {
		t.Errorf("expected retry content, got %q", resp.Content)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 Ollama calls (original + retry), got %d", callCount)
	}
}

// TestCallInference_StreamWithEmptyLinesAndThinkBlocks covers the streaming scanner path:
// empty lines skipped, think blocks stripped from accumulated content.
func TestCallInference_StreamWithEmptyLinesAndThinkBlocks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Empty line (should be skipped).
		_, _ = w.Write([]byte("\n"))
		// Chunk with thinking content.
		chunk1, _ := json.Marshal(map[string]interface{}{
			"message": map[string]string{"content": "<think>internal</think>"},
			"done":    false,
		})
		_, _ = w.Write(chunk1)
		_, _ = w.Write([]byte("\n"))
		// Chunk with visible content.
		chunk2, _ := json.Marshal(map[string]interface{}{
			"message": map[string]string{"content": "visible"},
			"done":    false,
		})
		_, _ = w.Write(chunk2)
		_, _ = w.Write([]byte("\n"))
		// Done chunk.
		done, _ := json.Marshal(map[string]interface{}{
			"message": map[string]string{"content": ""},
			"done":    true,
		})
		_, _ = w.Write(done)
		_, _ = w.Write([]byte("\n"))
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)
	t.Setenv("INFERENCE_MODEL", "qwen3.5:0.8b")

	handler := ChatCompletionHandler("", slog.Default(), NewChatDepsFromEnv())
	reqBody, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest("POST", "/internal/chat/completion", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body.String())
	}
	var out InternalChatCompletionResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Content != "visible" {
		t.Errorf("expected think blocks stripped, got %q", out.Content)
	}
}

// TestCallInference_StreamErrorChunk verifies that an error in a streaming chunk propagates.
func TestCallInference_StreamErrorChunk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		errChunk, _ := json.Marshal(map[string]interface{}{
			"error": "context length exceeded",
			"done":  true,
		})
		_, _ = w.Write(errChunk)
		_, _ = w.Write([]byte("\n"))
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)
	t.Setenv("INFERENCE_MODEL", "qwen3.5:0.8b")

	handler := ChatCompletionHandler("", slog.Default(), NewChatDepsFromEnv())
	reqBody, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest("POST", "/internal/chat/completion", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on stream error chunk, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCanStreamCompletion_NoMCPClient(t *testing.T) {
	// With no MCP gateway env vars set, BaseURL is empty so canStreamCompletion returns true.
	t.Setenv("PMA_MCP_GATEWAY_URL", "")
	t.Setenv("MCP_GATEWAY_URL", "")
	deps := NewChatDepsFromEnv()
	req := &InternalChatCompletionRequest{
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: "hi"}},
	}
	if !canStreamCompletion(req, deps) {
		t.Error("canStreamCompletion should return true when no MCP gateway is configured")
	}
}

func TestChatCompletionHandler_StreamTrue_Success(t *testing.T) {
	mockInference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		chunk, _ := json.Marshal(map[string]interface{}{
			"message": map[string]string{"content": "streamed"},
			"done":    false,
		})
		_, _ = w.Write(chunk)
		_, _ = w.Write([]byte("\n"))
		done, _ := json.Marshal(map[string]interface{}{
			"message": map[string]string{"content": ""},
			"done":    true,
		})
		_, _ = w.Write(done)
		_, _ = w.Write([]byte("\n"))
	}))
	defer mockInference.Close()
	t.Setenv("OLLAMA_BASE_URL", mockInference.URL)
	t.Setenv("PMA_MCP_GATEWAY_URL", "")
	t.Setenv("MCP_GATEWAY_URL", "")

	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
	body, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   true,
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "streamed") {
		t.Errorf("expected 'streamed' in body; got %q", rec.Body.String())
	}
}

func TestChatCompletionHandler_StreamTrue_InferenceError(t *testing.T) {
	mockInference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockInference.Close()
	t.Setenv("OLLAMA_BASE_URL", mockInference.URL)
	t.Setenv("PMA_MCP_GATEWAY_URL", "")
	t.Setenv("MCP_GATEWAY_URL", "")

	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
	body, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   true,
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on inference error, got %d", rec.Code)
	}
}

// TestChatCompletionHandler_StreamTrue_NoContentDeltas covers streamCompletionToWriter when Ollama
// returns 200 but no non-empty content chunks (had=false → 500).
func TestChatCompletionHandler_StreamTrue_NoContentDeltas(t *testing.T) {
	mockInference := newMockOllamaNDJSONEmptyDoneServer(t)
	defer mockInference.Close()
	t.Setenv("OLLAMA_BASE_URL", mockInference.URL)
	t.Setenv("PMA_MCP_GATEWAY_URL", "")
	t.Setenv("MCP_GATEWAY_URL", "")
	handler := ChatCompletionHandler("sys", slog.Default(), NewChatDepsFromEnv())
	body, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   true,
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when stream has no content, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStreamCompletionWriteChunk_Empty(t *testing.T) {
	rec := httptest.NewRecorder()
	enc := json.NewEncoder(rec)
	done, stop := streamCompletionWriteChunk(enc, rec, []byte{}, slog.Default())
	if done || stop {
		t.Errorf("empty line: done=%v stop=%v, want false,false", done, stop)
	}
}

func TestStreamCompletionWriteChunk_DoneChunk(t *testing.T) {
	rec := httptest.NewRecorder()
	enc := json.NewEncoder(rec)
	line, _ := json.Marshal(map[string]interface{}{"done": true, "message": map[string]string{"content": ""}})
	done, stop := streamCompletionWriteChunk(enc, rec, line, slog.Default())
	if !done || stop {
		t.Errorf("done chunk: done=%v stop=%v, want true,false", done, stop)
	}
}

func TestStreamCompletionWriteChunk_ContentChunk(t *testing.T) {
	rec := httptest.NewRecorder()
	enc := json.NewEncoder(rec)
	line, _ := json.Marshal(map[string]interface{}{"done": false, "message": map[string]string{"content": "tok"}})
	done, stop := streamCompletionWriteChunk(enc, rec, line, slog.Default())
	if done || stop {
		t.Errorf("content chunk: done=%v stop=%v, want false,false", done, stop)
	}
	if !strings.Contains(rec.Body.String(), "tok") {
		t.Errorf("expected 'tok' in output; got %q", rec.Body.String())
	}
}

func TestStreamCompletionWriteChunk_ErrorChunk(t *testing.T) {
	rec := httptest.NewRecorder()
	enc := json.NewEncoder(rec)
	line, _ := json.Marshal(map[string]interface{}{"done": false, "message": map[string]string{}, "error": "inference failed"})
	done, stop := streamCompletionWriteChunk(enc, rec, line, slog.Default())
	if done || !stop {
		t.Errorf("error chunk: done=%v stop=%v, want false,true", done, stop)
	}
}

func TestStreamCompletionWriteChunk_InvalidJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	enc := json.NewEncoder(rec)
	done, stop := streamCompletionWriteChunk(enc, rec, []byte(`{invalid`), slog.Default())
	if done || stop {
		t.Errorf("invalid JSON: done=%v stop=%v, want false,false", done, stop)
	}
}
