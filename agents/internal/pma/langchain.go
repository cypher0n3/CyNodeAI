// Package pma provides langchaingo-based chat completion with MCP tool support.
// See docs/tech_specs/project_manager_agent.md (LLM and Tool Execution) and cynode_pma.md.
package pma

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/tools"
)

const (
	pmaDefaultOllamaURL = "http://localhost:11434"
	pmaDefaultModel     = "qwen3.5:0.8b"
	envOllamaNumCtx     = "OLLAMA_NUM_CTX"
	// pmaMaxIterations caps the langchaingo agent loop. 3 iterations is sufficient
	// for most real PM tasks (fetch context → act → summarise) while keeping total
	// agent time well under the gateway write timeout.
	pmaMaxIterations = 3
	// inferenceHTTPTimeout is the HTTP client and streaming read timeout for direct Ollama
	// inference calls. Thinking models (qwen3:8b) can take up to 300 s on modest hardware.
	// See CYNAI.PMAGNT.StreamingAssistantOutput.
	inferenceHTTPTimeout = 300 * time.Second
	// xmlThinkOpen and xmlThinkClose delimit Qwen3/DeepSeek-R1 internal reasoning blocks.
	// These are stripped from final output before returning to the caller.
	xmlThinkOpen  = "<think>"
	xmlThinkClose = "</think>"
)

// smallModelSuffixes are parameter-count suffixes that indicate a model is too
// small to reliably produce ReAct-style structured output, even when the model
// family otherwise supports tool calling (e.g. qwen3.5:0.8b, qwen3:1b).
var smallModelSuffixes = []string{
	":0.5b", ":0.6b", ":0.8b", ":1b", ":1.1b", ":1.5b", ":1.8b",
}

// capableModels lists model name prefixes that support OpenAI-compatible tool
// calling via Ollama's native function-calling API.  These use the full
// OpenAIFunctionsAgent+MCP path rather than text-based ReAct.
// Models NOT in this list fall back to direct generation (single LLM call).
// Spec ref: CYNAI.ORCHES.ProjectManagerModelStartup tier list.
//
// Note: the spec uses logical names like "qwen3.5:9b" and "qwen2.5:14b"; the
// actual Ollama Hub names are "qwen3:8b" and "qwen2.5:14b". Both forms are matched.
// Small parameter variants (see smallModelSuffixes) are excluded even when the
// family prefix matches — 0.8B models do not reliably produce structured output.
var capableModels = []string{
	"qwen3.5:",
	"qwen3.5",
	"qwen3:",
	"qwen3",
	"qwen2.5:",
	"qwen2.5",
	"llama3.",
	"llama-3.",
	"mistral",
	"mixtral",
}

// isCapableModel reports whether the named model supports OpenAI-compatible
// tool calling via Ollama.  Small parameter variants (<= ~2B, matched by
// smallModelSuffixes) always use the direct-generation path.
func isCapableModel(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, suffix := range smallModelSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return false
		}
	}
	for _, prefix := range capableModels {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// testLLMForCompletion is set by tests to avoid calling real Ollama. Production always leaves it nil.
var testLLMForCompletion llms.Model

// runCompletionWithLangchain runs one completion using langchaingo for capable models.
//
// Capable models (qwen3:8b, qwen2.5:*, etc.) use OpenAIFunctionsAgent, which drives
// tool calls via the OpenAI-compatible function-calling API that Ollama exposes for
// models with native tool support (e.g. Qwen3 <tool_call> format).
// This avoids the text-based ReAct parser which fails on thinking-model output.
//
// Per REQ-PMAGNT-0100/0101 / CYNAI.AGENTS.PMLlmToolImplementation.
// Small/smoke models must NOT be routed here. Use callInference for those.
//
// When mcpClient.BaseURL is empty the function returns an error.
func runCompletionWithLangchain(ctx context.Context, fullPrompt string, mcpClient *MCPClient, logger *slog.Logger) (string, error) {
	if mcpClient == nil || mcpClient.BaseURL == "" {
		return "", fmt.Errorf("MCP gateway URL not set")
	}
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("INFERENCE_URL")
	}
	if baseURL == "" {
		baseURL = pmaDefaultOllamaURL
	}
	model := os.Getenv("INFERENCE_MODEL")
	if model == "" {
		model = pmaDefaultModel
	}

	var llm llms.Model
	if testLLMForCompletion != nil {
		llm = testLLMForCompletion
	} else {
		ollamaURL, httpClient := resolveOllamaClientConfig(baseURL)
		opts := []ollama.Option{
			ollama.WithServerURL(ollamaURL),
			ollama.WithModel(model),
		}
		if httpClient != nil {
			opts = append(opts, ollama.WithHTTPClient(httpClient))
		}
		if n := ollamaNumCtxFromEnv(); n > 0 {
			opts = append(opts, ollama.WithRunnerNumCtx(n))
		}
		var err error
		llm, err = ollama.New(opts...)
		if err != nil {
			return "", fmt.Errorf("create ollama llm: %w", err)
		}
	}

	if logger != nil {
		logger.Debug("using langchain functions agent path", "model", model)
	}
	toolsList := []tools.Tool{NewMCPTool(mcpClient)}
	// OpenAIFunctionsAgent uses the native tool-calling API (Ollama <tool_call> format)
	// rather than text-based ReAct, which is required for Qwen3-family thinking models.
	agent := agents.NewOpenAIFunctionsAgent(llm, toolsList,
		agents.WithMaxIterations(pmaMaxIterations),
	)
	exec := agents.NewExecutor(agent,
		agents.WithReturnIntermediateSteps(),
		agents.WithMaxIterations(pmaMaxIterations),
	)
	outputs, err := exec.Call(ctx, map[string]any{"input": fullPrompt})
	if err != nil {
		return "", err
	}
	return extractOutput(outputs), nil
}

// runCompletionWithLangchainStreaming runs the same agent path as runCompletionWithLangchain
// but streams NDJSON (iteration_start, delta, done) to w. Used when req.Stream is true
// and the capable-model + MCP path is selected. Caller must set Content-Type and status.
func runCompletionWithLangchainStreaming(ctx context.Context, fullPrompt string, mcpClient *MCPClient, w http.ResponseWriter, logger *slog.Logger) error {
	if mcpClient == nil || mcpClient.BaseURL == "" {
		return fmt.Errorf("MCP gateway URL not set")
	}
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("INFERENCE_URL")
	}
	if baseURL == "" {
		baseURL = pmaDefaultOllamaURL
	}
	model := os.Getenv("INFERENCE_MODEL")
	if model == "" {
		model = pmaDefaultModel
	}

	var baseLLM llms.Model
	if testLLMForCompletion != nil {
		baseLLM = testLLMForCompletion
	} else {
		ollamaURL, httpClient := resolveOllamaClientConfig(baseURL)
		opts := []ollama.Option{
			ollama.WithServerURL(ollamaURL),
			ollama.WithModel(model),
		}
		if httpClient != nil {
			opts = append(opts, ollama.WithHTTPClient(httpClient))
		}
		if n := ollamaNumCtxFromEnv(); n > 0 {
			opts = append(opts, ollama.WithRunnerNumCtx(n))
		}
		var err error
		baseLLM, err = ollama.New(opts...)
		if err != nil {
			return fmt.Errorf("create ollama llm: %w", err)
		}
	}

	iteration := 0
	llm := newStreamingLLM(baseLLM, w, &iteration)
	if logger != nil {
		logger.Debug("using langchain streaming functions agent path", "model", model)
	}
	toolsList := []tools.Tool{NewMCPTool(mcpClient)}
	agent := agents.NewOpenAIFunctionsAgent(llm, toolsList,
		agents.WithMaxIterations(pmaMaxIterations),
	)
	exec := agents.NewExecutor(agent,
		agents.WithReturnIntermediateSteps(),
		agents.WithMaxIterations(pmaMaxIterations),
	)
	_, err := exec.Call(ctx, map[string]any{"input": fullPrompt})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]bool{"done": true}); err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func extractOutput(outputs map[string]any) string {
	if v, ok := outputs["output"]; ok && v != nil {
		var s string
		if sv, ok := v.(string); ok {
			s = sv
		} else {
			s = fmt.Sprint(v)
		}
		return strings.TrimSpace(stripThinkBlocks(s))
	}
	return ""
}

// stripThinkBlocks removes <think>...</think> blocks emitted by reasoning models
// (Qwen3, DeepSeek-R1) from s and returns the trimmed remainder.
// If the opening tag is present but the closing tag is absent (truncated output),
// everything from the opening tag onwards is dropped.
func stripThinkBlocks(s string) string {
	for {
		start := strings.Index(s, xmlThinkOpen)
		if start == -1 {
			break
		}
		end := strings.Index(s[start:], xmlThinkClose)
		if end == -1 {
			// Unterminated block — drop from opening tag to end of string.
			s = s[:start]
			break
		}
		s = s[:start] + s[start+end+len(xmlThinkClose):]
	}
	return strings.TrimSpace(s)
}

// looksLikeUnexecutedToolCall reports whether output appears to be an agent
// preamble that described intending to call a tool but never received a result.
// This happens when the LLM emits explanatory text ("I'll use db.list_tasks…")
// as its content field instead of (or alongside) a proper tool_calls payload, so
// the OpenAIFunctionsAgent returns that preamble as the final answer.
//
// Heuristics (any match → true):
//   - Output contains a JSON block whose first key is "tool" or "tool_name".
//   - Output contains a raw <tool_call> XML open tag (Ollama's native format leaked through).
//   - Output matches "please wait while I" or "let me (fetch|retrieve|get|call|use)" followed
//     by a known tool-category word.
func looksLikeUnexecutedToolCall(s string) bool {
	lower := strings.ToLower(s)
	// Raw Ollama tool_call XML leaked into content.
	if strings.Contains(lower, "<tool_call>") {
		return true
	}
	// JSON block that looks like a tool invocation description.
	if containsToolJSON(s) {
		return true
	}
	// Preamble patterns: model announcing intent but no result follows.
	preambleWords := []string{"let me fetch", "let me retrieve", "let me get", "let me call", "let me use",
		"i'll fetch", "i'll retrieve", "i'll get", "i'll call", "i'll use",
		"please wait while i", "i will fetch", "i will retrieve", "i will get", "i will call", "i will use",
		"i'm going to fetch", "i'm going to call", "i'm going to use",
	}
	for _, p := range preambleWords {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// containsToolJSON reports whether s contains a JSON object with a "tool" or "tool_name" key,
// which indicates the model described a tool call as text instead of executing it.
func containsToolJSON(s string) bool {
	const toolKey = `"tool"`
	const toolNameKey = `"tool_name"`
	braceIdx := strings.Index(s, "{")
	if braceIdx == -1 {
		return false
	}
	sub := s[braceIdx:]
	return strings.Contains(sub, toolKey) || strings.Contains(sub, toolNameKey)
}

func runCompletionWithLangchainWithTimeout(ctx context.Context, fullPrompt string, mcpClient *MCPClient, logger *slog.Logger, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return runCompletionWithLangchain(runCtx, fullPrompt, mcpClient, logger)
}

// ollamaNumCtxFromEnv reads the OLLAMA_NUM_CTX environment variable (set by the
// node-manager based on the orchestrator's hardware-derived value) and returns it
// as an int.  Returns 0 if unset or unparseable.
func ollamaNumCtxFromEnv() int {
	v := os.Getenv(envOllamaNumCtx)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// resolveOllamaClientConfig converts an http+unix:// base URL into a plain
// http://localhost URL and a custom http.Client that dials the Unix socket.
// For standard http:// URLs the input is returned unchanged with a nil client.
func resolveOllamaClientConfig(baseURL string) (serverURL string, httpClient *http.Client) {
	u, c := resolveInferenceClient(baseURL, 0)
	if !strings.HasPrefix(strings.TrimSpace(baseURL), "http+unix://") {
		return u, nil
	}
	return u, c
}
