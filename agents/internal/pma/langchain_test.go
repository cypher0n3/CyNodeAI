package pma

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestIsCapableModel(t *testing.T) {
	cases := []struct {
		model  string
		expect bool
	}{
		// Spec logical names.
		{"qwen3.5:9b", true},
		{"qwen3.5", true},
		{"qwen2.5:14b", true},
		// Actual Ollama Hub names (qwen3.5 → qwen3 on Hub).
		{"qwen3:8b", true},
		{"qwen3", true},
		{"qwen2.5:14b", true},
		{"llama3.3:70b", true},
		{"llama3.2:3b", true},
		{"mistral:7b", true},
		{"mixtral:8x7b", true},
		// Small variants of capable families — excluded by smallModelSuffixes.
		{"qwen3.5:0.8b", false},
		{"qwen3:1b", false},
		{"qwen2.5:1.5b", false},
		// Other small/smoke models — direct generation path (no ReAct).
		{"tinyllama", false},
		{"tinyllama:1.1b", false},
		{"phi3:mini", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isCapableModel(c.model); got != c.expect {
			t.Errorf("isCapableModel(%q) = %v, want %v", c.model, got, c.expect)
		}
	}
}

// TestRunCompletionWithLangchain_CapableModelOnly verifies that runCompletionWithLangchain
// always uses the agent loop (small models are no longer routed here; chat.go routes
// small models to callInference before calling runCompletionWithLangchain).
func TestRunCompletionWithLangchain_CapableModelOnly(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	client := &MCPClient{BaseURL: mcpSrv.URL}

	oldHook := testLLMForCompletion
	// OpenAIFunctionsAgent returns choice.Content directly (no "Final Answer:" stripping).
	testLLMForCompletion = &mockLLM{responses: []string{"hello from agent"}}
	defer func() { testLLMForCompletion = oldHook }()

	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b") // capable → functions agent loop

	content, err := runCompletionWithLangchain(context.Background(), "prompt text", client, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hello from agent" {
		t.Errorf("content = %q, want %q", content, "hello from agent")
	}
}

func TestExtractOutput(t *testing.T) {
	if got := extractOutput(nil); got != "" {
		t.Errorf("extractOutput(nil) = %q", got)
	}
	if got := extractOutput(map[string]any{}); got != "" {
		t.Errorf("extractOutput(empty) = %q", got)
	}
	if got := extractOutput(map[string]any{"output": "hello"}); got != "hello" {
		t.Errorf("extractOutput(output=hello) = %q", got)
	}
	if got := extractOutput(map[string]any{"output": "  spaced  "}); got != "spaced" {
		t.Errorf("extractOutput(spaced) = %q", got)
	}
	if got := extractOutput(map[string]any{"output": 123}); got != "123" {
		t.Errorf("extractOutput(int) = %q", got)
	}
	// Think blocks are stripped by extractOutput.
	if got := extractOutput(map[string]any{"output": "<think>internal</think>answer"}); got != "answer" {
		t.Errorf("extractOutput(think block) = %q, want %q", got, "answer")
	}
}

func TestStripThinkBlocks(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"no blocks", "no blocks"},
		{"<think>hidden</think>visible", "visible"},
		{"before<think>mid</think>after", "beforeafter"},
		{"<think>a</think>x<think>b</think>y", "xy"},
		// Unterminated block — drop from opening tag.
		{"good<think>truncated", "good"},
		// Empty string.
		{"", ""},
	}
	for _, c := range cases {
		if got := stripThinkBlocks(c.input); got != c.want {
			t.Errorf("stripThinkBlocks(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestLooksLikeUnexecutedToolCall(t *testing.T) {
	positive := []string{
		// Ollama raw XML leaked into content.
		"I'll call the tool <tool_call>db.list_tasks</tool_call>",
		// JSON block with tool key.
		`Here is what I'll do: {"tool": "db.list_tasks", "params": {}}`,
		`{"tool_name": "db.task.get", "arguments": {"id": "123"}}`,
		// Model preamble patterns.
		"Let me fetch the task list for you.",
		"I'll retrieve the project tasks via the MCP gateway.",
		"Please wait while I fetch this information.",
		"I will get the task list now.",
		"I'm going to call the MCP tool.",
	}
	for _, s := range positive {
		if !looksLikeUnexecutedToolCall(s) {
			t.Errorf("looksLikeUnexecutedToolCall(%q) = false, want true", s)
		}
	}
	negative := []string{
		"Here are the tasks: task-1, task-2, task-3.",
		"The project has 5 open tasks.",
		"No tasks found in the current project.",
		"",
		"I cannot access tasks without an MCP gateway configured.",
	}
	for _, s := range negative {
		if looksLikeUnexecutedToolCall(s) {
			t.Errorf("looksLikeUnexecutedToolCall(%q) = true, want false", s)
		}
	}
}

func TestRunCompletionWithLangchainStreaming_EmitsNDJSON(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	client := &MCPClient{BaseURL: mcpSrv.URL}
	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"streamed"}}
	defer func() { testLLMForCompletion = oldHook }()
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")

	rec := httptest.NewRecorder()
	err := runCompletionWithLangchainStreaming(context.Background(), "prompt", client, rec, slog.Default())
	if err != nil {
		t.Fatalf("runCompletionWithLangchainStreaming: %v", err)
	}
	body := rec.Body.String()
	// Must contain iteration_start and done (mock LLM invokes StreamingFunc so delta lines present).
	if !strings.Contains(body, `"iteration_start"`) {
		t.Errorf("response missing iteration_start: %s", body)
	}
	if !strings.Contains(body, `"done"`) {
		t.Errorf("response missing done: %s", body)
	}
	// Parse first line as JSON (iteration_start).
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) < 1 {
		t.Fatalf("expected at least one NDJSON line: %s", body)
	}
	var first map[string]json.RawMessage
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first line not JSON: %v", err)
	}
	if _, ok := first["iteration_start"]; !ok {
		t.Errorf("first line missing iteration_start: %s", lines[0])
	}
}

// TestStreamingLLM_Call exercises GenerateFromSinglePrompt on the streaming LLM.
func TestStreamingLLM_Call(t *testing.T) {
	rec := httptest.NewRecorder()
	iter := 0
	inner := &mockLLM{responses: []string{"call-result"}}
	llm := newStreamingLLM(inner, rec, &iter)
	s, err := llms.GenerateFromSinglePrompt(context.Background(), llm, "prompt")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if s != "call-result" {
		t.Errorf("got %q, want call-result", s)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"iteration_start"`) {
		t.Errorf("body missing iteration_start: %s", body)
	}
	if !strings.Contains(body, `"delta"`) {
		t.Errorf("body missing delta: %s", body)
	}
}

func TestRunCompletionWithLangchain_EmptyClient(t *testing.T) {
	_, err := runCompletionWithLangchain(context.Background(), "prompt", nil, slog.Default())
	if err == nil {
		t.Error("expected error for nil client")
	}
	_, err = runCompletionWithLangchain(context.Background(), "prompt", &MCPClient{BaseURL: ""}, slog.Default())
	if err == nil {
		t.Error("expected error for empty BaseURL")
	}
}

func TestRunCompletionWithLangchain_CanceledContext(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	client := &MCPClient{BaseURL: mcpSrv.URL}
	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"ok"}}
	defer func() { testLLMForCompletion = oldHook }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runCompletionWithLangchain(ctx, "p", client, slog.Default())
	if err == nil {
		t.Error("expected error with canceled context")
	}
}

func TestRunCompletionWithLangchain_OllamaBranchUnreachable(t *testing.T) {
	// When testLLMForCompletion is nil, runCompletionWithLangchain uses Ollama. Use invalid URL so ollama.New or first call fails.
	oldURL := os.Getenv("OLLAMA_BASE_URL")
	_ = os.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:19998")
	defer func() {
		if oldURL != "" {
			_ = os.Setenv("OLLAMA_BASE_URL", oldURL)
		} else {
			_ = os.Unsetenv("OLLAMA_BASE_URL")
		}
	}()
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()
	client := NewMCPClient()
	client.BaseURL = mcpSrv.URL
	// Ensure we hit the Ollama branch (no test hook).
	oldHook := testLLMForCompletion
	testLLMForCompletion = nil
	defer func() { testLLMForCompletion = oldHook }()

	_, err := runCompletionWithLangchain(context.Background(), "p", client, slog.Default())
	if err == nil {
		t.Error("expected error when Ollama unreachable")
	}
}

func TestRunCompletionWithLangchainWithTimeout_DefaultTimeout(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{"result":"ok"}`)
	defer mcpSrv.Close()
	client := &MCPClient{BaseURL: mcpSrv.URL}
	oldHook := testLLMForCompletion
	// OpenAIFunctionsAgent returns choice.Content directly.
	testLLMForCompletion = &mockLLM{responses: []string{"timeout-default"}}
	defer func() { testLLMForCompletion = oldHook }()
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b") // capable model → functions agent loop
	// timeout=0 triggers the default-timeout branch.
	content, err := runCompletionWithLangchainWithTimeout(context.Background(), "p", client, slog.Default(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "timeout-default" {
		t.Errorf("content = %q", content)
	}
}

func TestRunCompletionWithLangchain_WithMockLLMAndMCP_CapableModels(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		llmAnswer   string
		wantContent string
	}{
		// OpenAIFunctionsAgent returns choice.Content directly (no "Final Answer:" prefix).
		{"qwen3.5:9b", "qwen3.5:9b", "hello from pma", "hello from pma"},
		{"qwen2.5:14b variant", "qwen2.5:14b", "capable answer", "capable answer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpSrv := newMockMCPServer(t, `{"result":"ok"}`)
			defer mcpSrv.Close()

			client := &MCPClient{BaseURL: mcpSrv.URL}
			oldHook := testLLMForCompletion
			testLLMForCompletion = &mockLLM{responses: []string{tt.llmAnswer}}
			defer func() { testLLMForCompletion = oldHook }()

			t.Setenv("INFERENCE_MODEL", tt.model)

			content, err := runCompletionWithLangchain(context.Background(), "sys\n\nuser: hi\nassistant: ", client, slog.Default())
			if err != nil {
				t.Fatalf("runCompletionWithLangchain: %v", err)
			}
			if content != tt.wantContent {
				t.Errorf("content = %q, want %q", content, tt.wantContent)
			}
		})
	}
}

// TestRunCompletionWithLangchain_OllamaNumCtxFromEnv exercises ollamaNumCtxFromEnv by running
// the real Ollama client path (testLLMForCompletion=nil) with OLLAMA_NUM_CTX set and a mock server.
func TestRunCompletionWithLangchain_OllamaNumCtxFromEnv(t *testing.T) {
	mockOllama := newMockInferenceServer(t, 200, `{"message":{"role":"assistant","content":"numctx-ok"},"done":true}`)
	defer mockOllama.Close()
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()

	t.Setenv("OLLAMA_BASE_URL", mockOllama.URL)
	t.Setenv("OLLAMA_NUM_CTX", "4096")
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")
	oldHook := testLLMForCompletion
	testLLMForCompletion = nil
	defer func() { testLLMForCompletion = oldHook }()

	client := &MCPClient{BaseURL: mcpSrv.URL}
	content, err := runCompletionWithLangchain(context.Background(), "hi", client, slog.Default())
	if err != nil {
		t.Fatalf("runCompletionWithLangchain: %v", err)
	}
	if content != "numctx-ok" {
		t.Errorf("content = %q, want numctx-ok", content)
	}
}

func TestResolveOllamaClientConfig_TCP(t *testing.T) {
	url, client := resolveOllamaClientConfig("http://localhost:11434")
	if url != "http://localhost:11434" {
		t.Errorf("expected passthrough URL, got %q", url)
	}
	if client != nil {
		t.Error("expected nil client for TCP URL (use default)")
	}
}

func TestResolveOllamaClientConfig_UnixSocket(t *testing.T) {
	url, client := resolveOllamaClientConfig("http+unix://%2Ftmp%2Ftest.sock")
	if url != udsPlainHost {
		t.Errorf("expected %q for unix socket, got %q", udsPlainHost, url)
	}
	if client == nil {
		t.Fatal("expected non-nil http.Client for unix socket")
	}
	if client.Transport == nil {
		t.Error("expected custom transport for unix socket")
	}
}
