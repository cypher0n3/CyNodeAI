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
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
)

const (
	pmaDefaultOllamaURL      = "http://localhost:11434"
	pmaDefaultModel          = "qwen3.5:0.8b"
	envOllamaNumCtx          = "OLLAMA_NUM_CTX"
	envPmaMaxAgentIterations = "PMA_MAX_AGENT_ITERATIONS"
	// defaultPmaMaxAgentIterations caps the langchaingo OpenAIFunctionsAgent loop.
	// Each LLM turn + tool round-trip consumes at least one iteration; 3 was too low
	// (constant ErrNotFinished → no-tools fallback → empty or fake answers). Keep
	// under gateway timeouts via LangchainCompletionTimeout (300s), not an artificially
	// tiny iteration cap.
	defaultPmaMaxAgentIterations = 20
	maxPmaMaxAgentIterations     = 80
	// inferenceHTTPTimeout is the HTTP client and streaming read timeout for direct Ollama
	// inference calls. Thinking models (qwen3:8b) can take up to 300 s on modest hardware.
	// See CYNAI.PMAGNT.StreamingAssistantOutput.
	inferenceHTTPTimeout = 300 * time.Second
	// xmlThinkOpen and xmlThinkClose delimit Qwen3/DeepSeek-R1 internal reasoning blocks.
	// extractThinkBlocks separates them for visible vs thinking (CYNAI.PMAGNT.ThinkingContentSeparation).
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

// pmaMaxAgentIterationsFromEnv returns the agent loop cap (default defaultPmaMaxAgentIterations).
// Override with PMA_MAX_AGENT_ITERATIONS (1–80) for slow models or very long tool chains.
func pmaMaxAgentIterationsFromEnv() int {
	v := strings.TrimSpace(os.Getenv(envPmaMaxAgentIterations))
	if v == "" {
		return defaultPmaMaxAgentIterations
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return defaultPmaMaxAgentIterations
	}
	if n > maxPmaMaxAgentIterations {
		return maxPmaMaxAgentIterations
	}
	return n
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
func runCompletionWithLangchain(ctx context.Context, fullPrompt string, mcpClient *MCPClient, logger *slog.Logger) (visible, thinking string, err error) {
	if mcpClient == nil || mcpClient.BaseURL == "" {
		return "", "", fmt.Errorf("MCP gateway URL not set")
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
		var err error
		llm, err = newToolsAgentLLM(baseURL, model)
		if err != nil {
			return "", "", err
		}
	}

	if logger != nil {
		logger.Debug("using langchain functions agent path", "model", model)
	}
	maxIter := pmaMaxAgentIterationsFromEnv()
	toolsList := []tools.Tool{NewMCPTool(mcpClient)}
	// OpenAIFunctionsAgent uses the native tool-calling API (Ollama <tool_call> format)
	// rather than text-based ReAct, which is required for Qwen3-family thinking models.
	agent := agents.NewOpenAIFunctionsAgent(llm, toolsList,
		agents.WithMaxIterations(maxIter),
	)
	exec := agents.NewExecutor(agent,
		agents.WithReturnIntermediateSteps(),
		agents.WithMaxIterations(maxIter),
	)
	outputs, err := exec.Call(ctx, map[string]any{"input": fullPrompt})
	if err != nil {
		return "", "", err
	}
	visible, thinking = extractOutput(outputs)
	if repaired, ok := tryRepairTextualMCPCalls(ctx, visible, mcpClient); ok {
		return repaired, thinking, nil
	}
	return visible, thinking, nil
}

// tryRepairTextualMCPCalls runs MCP when the model put OpenAI-style tool_calls JSON in the
// assistant message body instead of the API's tool_calls field. Ollama's /v1/chat/completions
// sometimes surfaces tool calls only as message.content, so the agent executor never runs tools.
func tryRepairTextualMCPCalls(ctx context.Context, content string, mcpClient *MCPClient) (string, bool) {
	s := strings.TrimSpace(content)
	if !strings.HasPrefix(s, "[") || !strings.Contains(s, `"function"`) {
		return "", false
	}
	var calls []struct {
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	if err := json.Unmarshal([]byte(s), &calls); err != nil || len(calls) == 0 {
		return "", false
	}
	tool := NewMCPTool(mcpClient)
	var parts []string
	for _, c := range calls {
		if c.Type != "function" || c.Function.Name != "mcp_call" {
			continue
		}
		input := mcpCallInputFromOpenAIArguments(c.Function.Arguments)
		out, err := tool.Call(ctx, input)
		if err != nil {
			return "mcp_call: " + err.Error(), true
		}
		parts = append(parts, out)
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "\n\n"), true
}

// mcpCallInputFromOpenAIArguments mirrors agents.OpenAIFunctionsAgent: the model may send
// either {"__arg1":"<json string>"} or a bare {"tool_name":...,"arguments":...} object.
func mcpCallInputFromOpenAIArguments(argumentsJSON string) string {
	s := strings.TrimSpace(argumentsJSON)
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return s
	}
	if v, ok := m["__arg1"]; ok {
		if inner, ok := v.(string); ok {
			return inner
		}
	}
	return s
}

// pmaStreamDeltaRunes is the max runes per {"delta":"..."} line when emitting a completed
// langchain turn. True per-token streaming requires the Streaming LLM wrapper (cynode_pma.md);
// chunking progressive deltas avoids a single giant delta so cynork can render incrementally.
const pmaStreamDeltaRunes = 24

// writeLangchainNDJSONStream writes NDJSON for stream=true (orchestrator → gateway → TUI).
// Per CYNAI.PMAGNT.PMAStreamingNDJSONFormat / REQ-PMAGNT-0122, full thinking is emitted as
// {"thinking":"..."} before visible {"delta":"..."} chunks (canonical visible text without think tags).
func writeLangchainNDJSONStream(w http.ResponseWriter, visible, thinking string) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]int{"iteration_start": 1}); err != nil {
		return err
	}
	flushResponseWriter(w)
	thinkRed, thinkFound, _ := redactKnownSecrets(thinking)
	visRed, visFound, _ := redactKnownSecrets(visible)
	kinds := secretKindsFromBuffers(visible, thinking, "")
	secretFound := visFound || thinkFound
	if strings.TrimSpace(thinkRed) != "" {
		if err := enc.Encode(map[string]string{"thinking": thinkRed}); err != nil {
			return err
		}
		flushResponseWriter(w)
	}
	visRed = strings.TrimSpace(visRed)
	if visRed != "" {
		if err := writeLangchainClassifiedVisible(enc, w, visRed); err != nil {
			return err
		}
	}
	if secretFound {
		if err := encodeOverwriteNDJSON(enc, w, 1, visRed, "iteration", "secret_redaction", kinds); err != nil {
			return err
		}
	}
	if err := enc.Encode(map[string]bool{"done": true}); err != nil {
		return err
	}
	flushResponseWriter(w)
	return nil
}

func writeLangchainClassifiedVisible(enc *json.Encoder, w http.ResponseWriter, visible string) error {
	clf := newStreamingClassifier()
	emissions := clf.Feed(visible)
	emissions = append(emissions, clf.Flush()...)
	for _, em := range emissions {
		if err := encodeLangchainClassifiedEmission(enc, w, em); err != nil {
			return err
		}
	}
	return nil
}

func encodeLangchainClassifiedEmission(enc *json.Encoder, w http.ResponseWriter, em streamEmitted) error {
	switch em.Kind {
	case streamEmitToolCall:
		if err := enc.Encode(map[string]any{
			"tool_call": map[string]string{"name": "stream", "arguments": em.Text},
		}); err != nil {
			return err
		}
		flushResponseWriter(w)
		return nil
	case streamEmitThinking:
		if err := enc.Encode(map[string]string{"thinking": em.Text}); err != nil {
			return err
		}
		flushResponseWriter(w)
		return nil
	case streamEmitDelta:
		return encodeLangchainDeltaChunks(enc, w, em.Text)
	default:
		return nil
	}
}

func encodeLangchainDeltaChunks(enc *json.Encoder, w http.ResponseWriter, text string) error {
	runes := []rune(text)
	for i := 0; i < len(runes); i += pmaStreamDeltaRunes {
		end := min(i+pmaStreamDeltaRunes, len(runes))
		chunk := string(runes[i:end])
		if err := enc.Encode(map[string]string{"delta": chunk}); err != nil {
			return err
		}
		flushResponseWriter(w)
	}
	return nil
}

func flushResponseWriter(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func extractOutput(outputs map[string]any) (visible, thinking string) {
	if v, ok := outputs["output"]; ok && v != nil {
		var s string
		if sv, ok := v.(string); ok {
			s = sv
		} else {
			s = fmt.Sprint(v)
		}
		return extractThinkBlocks(s)
	}
	return "", ""
}

// extractThinkBlocks separates model output into visible assistant text and thinking content
// (inner text of </think>...`</think>`). Per CYNAI.PMAGNT.ThinkingContentSeparation, tags MUST NOT
// appear in visible text; thinking is retained for upstream NDJSON (REQ-PMAGNT-0122).
func extractThinkBlocks(s string) (visible, thinking string) {
	var thinkParts []string
	rest := s
	for {
		start := strings.Index(rest, xmlThinkOpen)
		if start == -1 {
			break
		}
		end := strings.Index(rest[start:], xmlThinkClose)
		if end == -1 {
			thinkParts = append(thinkParts, rest[start+len(xmlThinkOpen):])
			rest = rest[:start]
			break
		}
		relEnd := start + end
		inner := rest[start+len(xmlThinkOpen) : relEnd]
		thinkParts = append(thinkParts, inner)
		rest = rest[:start] + rest[relEnd+len(xmlThinkClose):]
	}
	thinking = strings.TrimSpace(strings.Join(thinkParts, "\n"))
	visible = strings.TrimSpace(rest)
	return visible, thinking
}

// looksLikeCompleteAssistantAnswer reports whether s looks like a full assistant reply
// (markdown sections, multiple paragraphs, or lists). Such text often includes example
// JSON with "tool_name" for documentation; that must not trigger the unexecuted-tool fallback.
func looksLikeCompleteAssistantAnswer(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 80 {
		return false
	}
	// Section headings almost always mean a full assistant reply, not a tool stub.
	if strings.Contains(s, "## ") {
		return true
	}
	if len(s) < 250 {
		return false
	}
	if strings.Count(s, "\n\n") >= 2 {
		return true
	}
	if strings.Contains(s, "\n- ") || strings.Contains(s, "\n* ") {
		return true
	}
	return false
}

// stripMarkdownFencedCodeBlocks removes ``` ... ``` regions so tool-like JSON inside
// examples does not trigger containsToolJSON.
func stripMarkdownFencedCodeBlocks(s string) string {
	var b strings.Builder
	inFence := false
	for _, line := range strings.Split(s, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
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
	// Full answers with headings or structure often mention tool JSON as documentation.
	if looksLikeCompleteAssistantAnswer(s) {
		return false
	}
	// JSON block that looks like a tool invocation description (outside fenced examples).
	if containsToolJSON(stripMarkdownFencedCodeBlocks(s)) {
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
// Only the segment starting at the first `{` up to maxScan bytes is considered, so stray
// mentions later in long prose do not match.
func containsToolJSON(s string) bool {
	const toolKey = `"tool"`
	const toolNameKey = `"tool_name"`
	const maxScan = 2048
	braceIdx := strings.Index(s, "{")
	if braceIdx == -1 {
		return false
	}
	sub := s[braceIdx:]
	if len(sub) > maxScan {
		sub = sub[:maxScan]
	}
	return strings.Contains(sub, toolKey) || strings.Contains(sub, toolNameKey)
}

func runCompletionWithLangchainWithTimeout(ctx context.Context, fullPrompt string, mcpClient *MCPClient, logger *slog.Logger, timeout time.Duration) (visible, thinking string, err error) {
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

// newToolsAgentLLM returns an LLM for OpenAIFunctionsAgent + MCP tools.
//
// Langchaingo's llms/ollama GenerateContent ignores llms.WithFunctions, so tool definitions
// never reach Ollama and the model free-texts JSON "tool calls" instead of executing tools.
// Ollama's OpenAI-compatible HTTP API (/v1/chat/completions) honors tools; we use llms/openai
// with BaseURL set to <ollama-host>/v1. Token is a placeholder (Ollama does not require a real key).
func newToolsAgentLLM(baseURL, model string) (llms.Model, error) {
	u := strings.TrimSpace(baseURL)
	if u == "" {
		u = pmaDefaultOllamaURL
	}
	srvURL, httpClient := resolveOllamaClientConfig(u)
	openAIBase := strings.TrimSuffix(strings.TrimSpace(srvURL), "/") + "/v1"
	opts := []openai.Option{
		openai.WithToken("ollama"),
		openai.WithBaseURL(openAIBase),
		openai.WithModel(model),
	}
	if httpClient != nil {
		opts = append(opts, openai.WithHTTPClient(httpClient))
	}
	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create OpenAI-compatible LLM for tools: %w", err)
	}
	return llm, nil
}
