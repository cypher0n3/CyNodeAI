package pma

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

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
	testLLMForCompletion = &mockLLM{responses: []string{"Final Answer: ok"}}
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

func TestRunCompletionWithLangchain_WithMockLLMAndMCP(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{"result":"ok"}`)
	defer mcpSrv.Close()

	client := &MCPClient{BaseURL: mcpSrv.URL}
	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"Final Answer: hello from pma"}}
	defer func() { testLLMForCompletion = oldHook }()

	content, err := runCompletionWithLangchain(context.Background(), "sys\n\nuser: hi\nassistant: ", client, slog.Default())
	if err != nil {
		t.Fatalf("runCompletionWithLangchain: %v", err)
	}
	if content != "hello from pma" {
		t.Errorf("content = %q", content)
	}
}
