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

func TestOllamaNumCtxFromEnv(t *testing.T) {
	t.Setenv(envOllamaNumCtx, "")
	if ollamaNumCtxFromEnv() != 0 {
		t.Fatal("unset")
	}
	t.Setenv(envOllamaNumCtx, "bad")
	if ollamaNumCtxFromEnv() != 0 {
		t.Fatal("bad int")
	}
	t.Setenv(envOllamaNumCtx, "-3")
	if ollamaNumCtxFromEnv() != 0 {
		t.Fatal("negative")
	}
	t.Setenv(envOllamaNumCtx, "4096")
	if ollamaNumCtxFromEnv() != 4096 {
		t.Fatal("4096")
	}
}

func TestLooksLikeCompleteAssistantAnswer(t *testing.T) {
	if looksLikeCompleteAssistantAnswer("short") {
		t.Fatal("short")
	}
	long79 := strings.Repeat("a", 79)
	if looksLikeCompleteAssistantAnswer(long79) {
		t.Fatal("under 80")
	}
	withHeading := strings.Repeat("x", 40) + "\n## Section\n" + strings.Repeat("y", 40)
	if !looksLikeCompleteAssistantAnswer(withHeading) {
		t.Fatal("heading")
	}
	longParas := strings.Repeat("p", 120) + "\n\n" + strings.Repeat("q", 120) + "\n\n" + strings.Repeat("r", 120)
	if !looksLikeCompleteAssistantAnswer(longParas) {
		t.Fatal("paragraphs")
	}
	withBullet := strings.Repeat("a", 200) + "\n- item\n" + strings.Repeat("b", 50)
	if !looksLikeCompleteAssistantAnswer(withBullet) {
		t.Fatal("bullet list")
	}
	withStar := strings.Repeat("a", 200) + "\n* item\n" + strings.Repeat("b", 50)
	if !looksLikeCompleteAssistantAnswer(withStar) {
		t.Fatal("star list")
	}
}

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

func TestPmaMaxAgentIterationsFromEnv(t *testing.T) {
	t.Setenv(envPmaMaxAgentIterations, "")
	if got := pmaMaxAgentIterationsFromEnv(); got != defaultPmaMaxAgentIterations {
		t.Errorf("default: got %d want %d", got, defaultPmaMaxAgentIterations)
	}
	t.Setenv(envPmaMaxAgentIterations, "5")
	if got := pmaMaxAgentIterationsFromEnv(); got != 5 {
		t.Errorf("5: got %d", got)
	}
	t.Setenv(envPmaMaxAgentIterations, "999")
	if got := pmaMaxAgentIterationsFromEnv(); got != maxPmaMaxAgentIterations {
		t.Errorf("cap: got %d want %d", got, maxPmaMaxAgentIterations)
	}
	t.Setenv(envPmaMaxAgentIterations, "0")
	if got := pmaMaxAgentIterationsFromEnv(); got != defaultPmaMaxAgentIterations {
		t.Errorf("invalid: got %d", got)
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

	content, _, err := runCompletionWithLangchain(context.Background(), "prompt text", client, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hello from agent" {
		t.Errorf("content = %q, want %q", content, "hello from agent")
	}
}

func TestExtractOutput(t *testing.T) {
	v, th := extractOutput(nil)
	if v != "" || th != "" {
		t.Errorf("extractOutput(nil) = %q, %q", v, th)
	}
	v, th = extractOutput(map[string]any{})
	if v != "" || th != "" {
		t.Errorf("extractOutput(empty) = %q, %q", v, th)
	}
	v, th = extractOutput(map[string]any{"output": "hello"})
	if v != "hello" || th != "" {
		t.Errorf("extractOutput(output=hello) = %q, %q", v, th)
	}
	v, th = extractOutput(map[string]any{"output": "  spaced  "})
	if v != "spaced" || th != "" {
		t.Errorf("extractOutput(spaced) = %q, %q", v, th)
	}
	v, th = extractOutput(map[string]any{"output": 123})
	if v != "123" || th != "" {
		t.Errorf("extractOutput(int) = %q, %q", v, th)
	}
	v, th = extractOutput(map[string]any{"output": "<think>internal</think>answer"})
	if v != "answer" || th != "internal" {
		t.Errorf("extractOutput(think block) = %q, %q want answer, internal", v, th)
	}
}

func TestExtractThinkBlocks(t *testing.T) {
	cases := []struct {
		input, wantVis, wantThink string
	}{
		{"no blocks", "no blocks", ""},
		{"<think>hidden</think>visible", "visible", "hidden"},
		{"before<think>mid</think>after", "beforeafter", "mid"},
		{"<think>a</think>x<think>b</think>y", "xy", "a\nb"},
		{"good<think>truncated", "good", "truncated"},
		{"", "", ""},
	}
	for _, c := range cases {
		gotVis, gotThink := extractThinkBlocks(c.input)
		if gotVis != c.wantVis || gotThink != c.wantThink {
			t.Errorf("extractThinkBlocks(%q) = (%q, %q), want (%q, %q)", c.input, gotVis, gotThink, c.wantVis, c.wantThink)
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
		// Long structured intro with example JSON (must not look like an unexecuted stub).
		"Hello! I'm the **Project Manager Agent** for CyNodeAI, ready to assist you with project management tasks.\n\n" +
			"## What I Can Help With\n\n" +
			"You can use MCP tools. For example: {\"tool_name\": \"help.list\", \"arguments\": {}}\n\n" +
			"Let me know if you need anything else.",
		// Tool-like JSON only inside a fenced block (documentation).
		"Here is how to call tools:\n\n```json\n{\"tool_name\": \"mcp_call\", \"arguments\": {}}\n```\n",
	}
	for _, s := range negative {
		if looksLikeUnexecutedToolCall(s) {
			t.Errorf("looksLikeUnexecutedToolCall(%q) = true, want false", s)
		}
	}
}

func TestWriteLangchainNDJSONStream_EmitsNDJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := writeLangchainNDJSONStream(rec, "streamed", ""); err != nil {
		t.Fatalf("writeLangchainNDJSONStream: %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"iteration_start"`) {
		t.Errorf("response missing iteration_start: %s", body)
	}
	if !strings.Contains(body, `"delta"`) || !strings.Contains(body, "streamed") {
		t.Errorf("response missing delta with content: %s", body)
	}
	if !strings.Contains(body, `"done"`) {
		t.Errorf("response missing done: %s", body)
	}
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected iteration_start, delta, done lines: %s", body)
	}
	var first map[string]json.RawMessage
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first line not JSON: %v", err)
	}
	if _, ok := first["iteration_start"]; !ok {
		t.Errorf("first line missing iteration_start: %s", lines[0])
	}
}

func TestWriteLangchainNDJSONStream_WithThinking(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := writeLangchainNDJSONStream(rec, "visible", "inner-think"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.Body.String(), `"thinking"`) {
		t.Errorf("expected thinking NDJSON line: %s", rec.Body.String())
	}
}

func TestWriteLangchainNDJSONStream_MultipleDeltaChunks(t *testing.T) {
	rec := httptest.NewRecorder()
	long := strings.Repeat("Z", 80)
	if err := writeLangchainNDJSONStream(rec, long, ""); err != nil {
		t.Fatal(err)
	}
	if strings.Count(rec.Body.String(), `"delta"`) < 2 {
		t.Errorf("expected multiple delta chunks for long visible text: %s", rec.Body.String())
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

// TestStreamingLLM_CallMethodDirect exercises streamingLLM.Call (single-prompt path), not only GenerateFromSinglePrompt.
func TestStreamingLLM_CallMethodDirect(t *testing.T) {
	rec := httptest.NewRecorder()
	iter := 0
	inner := &mockLLM{responses: []string{"direct-call"}}
	llm := newStreamingLLM(inner, rec, &iter)
	slm := llm.(*streamingLLM)
	s, err := slm.Call(context.Background(), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if s != "direct-call" {
		t.Errorf("Call = %q", s)
	}
}

func TestRunCompletionWithLangchain_EmptyClient(t *testing.T) {
	_, _, err := runCompletionWithLangchain(context.Background(), "prompt", nil, slog.Default())
	if err == nil {
		t.Error("expected error for nil client")
	}
	_, _, err = runCompletionWithLangchain(context.Background(), "prompt", &MCPClient{BaseURL: ""}, slog.Default())
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
	_, _, err := runCompletionWithLangchain(ctx, "p", client, slog.Default())
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

	_, _, err := runCompletionWithLangchain(context.Background(), "p", client, slog.Default())
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
	content, _, err := runCompletionWithLangchainWithTimeout(context.Background(), "p", client, slog.Default(), 0)
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

			content, _, err := runCompletionWithLangchain(context.Background(), "sys\n\nuser: hi\nassistant: ", client, slog.Default())
			if err != nil {
				t.Fatalf("runCompletionWithLangchain: %v", err)
			}
			if content != tt.wantContent {
				t.Errorf("content = %q, want %q", content, tt.wantContent)
			}
		})
	}
}

func testTryRepairTextualMCPCalls(t *testing.T, raw, mcpResponse, wantSubstring string) {
	t.Helper()
	mcpSrv := newMockMCPServer(t, mcpResponse)
	defer mcpSrv.Close()
	client := &MCPClient{BaseURL: mcpSrv.URL}
	out, ok := tryRepairTextualMCPCalls(context.Background(), raw, client)
	if !ok {
		t.Fatal("expected repair")
	}
	if !strings.Contains(out, wantSubstring) {
		t.Fatalf("got %q", out)
	}
}

// TestRunCompletionWithLangchain_OllamaNumCtxFromEnv verifies the agent path still runs when
// OLLAMA_NUM_CTX is set (used by callInference / direct Ollama; tools path uses OpenAI-compatible /v1).
// TestTryRepairTextualMCPCalls_BareArgumentsJSON covers models that omit __arg1 and send the MCP payload directly.
func TestTryRepairTextualMCPCalls_BareArgumentsJSON(t *testing.T) {
	raw := `[{"type":"function","function":{"name":"mcp_call","arguments":"{\"tool_name\":\"help.get\",\"arguments\":{}}"}}]`
	testTryRepairTextualMCPCalls(t, raw, `{"ok":true}`, `"ok":true`)
}

// TestTryRepairTextualMCPCalls_OllamaContentLeak covers Ollama returning tool_calls only in message.content.
func TestTryRepairTextualMCPCalls_OllamaContentLeak(t *testing.T) {
	raw := `[{"id":"call_r3ng0yxq","type":"function","function":{"name":"mcp_call","arguments":"{\"__arg1\":\"{\\\"tool_name\\\": \\\"help.get\\\", \\\"arguments\\\": {}}\"}"}}]`
	testTryRepairTextualMCPCalls(t, raw, `{"help":"documentation body"}`, "documentation body")
}

// TestRunCompletionWithLangchain_RepairsTextualToolCalls runs the full path when mock LLM returns content-only tool JSON.
func TestRunCompletionWithLangchain_RepairsTextualToolCalls(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{"sections":["intro"]}`)
	defer mcpSrv.Close()
	client := &MCPClient{BaseURL: mcpSrv.URL}
	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{`[{"id":"x","type":"function","function":{"name":"mcp_call","arguments":"{\"__arg1\":\"{\\\"tool_name\\\": \\\"help.list\\\", \\\"arguments\\\": {}}\"}"}}]`}}
	defer func() { testLLMForCompletion = oldHook }()
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")
	content, _, err := runCompletionWithLangchain(context.Background(), "list help", client, slog.Default())
	if err != nil {
		t.Fatalf("runCompletionWithLangchain: %v", err)
	}
	if !strings.Contains(content, "intro") {
		t.Fatalf("expected MCP body in output, got %q", content)
	}
}

func TestRunCompletionWithLangchain_OllamaNumCtxFromEnv(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()

	t.Setenv("OLLAMA_NUM_CTX", "4096")
	t.Setenv("INFERENCE_MODEL", "qwen3.5:9b")
	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"numctx-ok"}}
	defer func() { testLLMForCompletion = oldHook }()

	client := &MCPClient{BaseURL: mcpSrv.URL}
	content, _, err := runCompletionWithLangchain(context.Background(), "hi", client, slog.Default())
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
