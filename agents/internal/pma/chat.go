// Package pma provides internal chat completion for orchestrator handoff.
// See docs/tech_specs/cynode_pma.md (request source and handoff).
package pma

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const roleUser = "user"

// pmaLangchainCompletionTimeout caps the langchaingo completion run.
// qwen3:8b at 32 768-token context can take up to ~60 s for a single thinking pass.
// With pmaMaxIterations=3 that is ~180 s worst-case, but the bound here is per-run
// (not per-iteration); the retry uses its own fresh context.  A single run should
// complete in well under 90 s on RX 7900 XT hardware.
const pmaLangchainCompletionTimeout = 90 * time.Second

// InternalChatCompletionRequest is the body for POST /internal/chat/completion (orchestrator handoff).
// Optional fields support full context order per CYNAI.PMAGNT.LLMContextComposition: project, task, additional context.
type InternalChatCompletionRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	ProjectID         string `json:"project_id,omitempty"`
	TaskID            string `json:"task_id,omitempty"`
	UserID            string `json:"user_id,omitempty"`
	AdditionalContext string `json:"additional_context,omitempty"`
}

// InternalChatCompletionResponse is the response body.
type InternalChatCompletionResponse struct {
	Content string `json:"content"`
}

// ChatCompletionHandler returns an HTTP handler for POST /internal/chat/completion.
// It uses instructionsContent as system context and calls the configured inference backend (Ollama).
func ChatCompletionHandler(instructionsContent string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req InternalChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Warn("chat completion decode error", "error", err)
			writeJSON(w, http.StatusBadRequest, InternalChatCompletionResponse{})
			return
		}
		if len(req.Messages) == 0 {
			writeJSON(w, http.StatusBadRequest, InternalChatCompletionResponse{})
			return
		}
		// Capture a detached root context before calling resolveContent so that
		// a retry after context-window overflow can use a fresh timeout independent
		// of the gateway request deadline.
		detached := context.WithoutCancel(r.Context())
		content, httpStatus := resolveContent(r.Context(), detached, instructionsContent, &req, logger)
		writeJSON(w, httpStatus, InternalChatCompletionResponse{Content: content})
	}
}

// resolveContent obtains the completion content for req, retrying with just the current user
// message if the full-history call returns empty output (context-window overflow).
// Returns (content, httpStatusCode).
func resolveContent(ctx, detachedRoot context.Context, instructionsContent string, req *InternalChatCompletionRequest, logger *slog.Logger) (content string, httpStatus int) {
	content, err := getCompletionContent(ctx, instructionsContent, req, logger)
	if err != nil {
		logger.Error("chat completion inference error", "error", err)
		return "", http.StatusInternalServerError
	}
	if strings.TrimSpace(content) != "" {
		return content, http.StatusOK
	}
	// Context may have exceeded the model's window — retry with only the current message.
	// Use a fresh context derived from the detached root captured at entry so the
	// original gateway deadline (which may be nearly expired) does not kill the retry.
	logger.Warn("chat completion produced empty output; retrying with current message only",
		"original_msg_count", len(req.Messages))
	stripped := stripToCurrentMessage(req)
	retryCtx, retryCancel := context.WithTimeout(detachedRoot, pmaLangchainCompletionTimeout)
	defer retryCancel()
	content, err = getCompletionContent(retryCtx, instructionsContent, stripped, logger)
	if err != nil {
		logger.Error("chat completion retry failed", "error", err)
		return "", http.StatusInternalServerError
	}
	if strings.TrimSpace(content) == "" {
		logger.Error("chat completion produced empty output after retry")
		return "", http.StatusInternalServerError
	}
	return content, http.StatusOK
}

// stripToCurrentMessage returns a copy of req containing only the last message.
func stripToCurrentMessage(req *InternalChatCompletionRequest) *InternalChatCompletionRequest {
	return &InternalChatCompletionRequest{
		Messages:          req.Messages[len(req.Messages)-1:],
		ProjectID:         req.ProjectID,
		TaskID:            req.TaskID,
		UserID:            req.UserID,
		AdditionalContext: req.AdditionalContext,
	}
}

// getCompletionContent runs the appropriate inference path (langchaingo for capable models, direct Ollama otherwise).
func getCompletionContent(ctx context.Context, instructionsContent string, req *InternalChatCompletionRequest, logger *slog.Logger) (string, error) {
	systemContext := buildSystemContext(instructionsContent, req)
	mcpClient := NewMCPClient()
	model := os.Getenv("INFERENCE_MODEL")
	if model == "" {
		model = pmaDefaultModel
	}
	if mcpClient.BaseURL != "" && isCapableModel(model) {
		// REQ-PMAGNT-0100/0101 / CYNAI.AGENTS.PMLlmToolImplementation: langchaingo is the
		// mandated inference path for capable models. Uses the OpenAIFunctionsAgent+MCP
		// tool loop via Ollama's native function-calling API.
		//
		// The agent executor takes a single `input` string. To preserve conversation
		// context, prior turns (all messages except the final user message) are appended
		// to the system context section, and only the current user message is used as
		// the agent `input`. This ensures the model sees the full history without it
		// being interpreted as part of the current instruction.
		systemContextWithHistory := buildSystemContextWithHistory(systemContext, req.Messages)
		currentInput := lastUserMessage(req.Messages)
		content, err := runCompletionWithLangchainWithTimeout(
			ctx,
			buildAgentInput(systemContextWithHistory, currentInput),
			mcpClient,
			logger,
			pmaLangchainCompletionTimeout,
		)
		if err != nil {
			return "", err
		}
		// If the agent returned a preamble describing a tool call it never executed
		// (model emitted explanatory text instead of a proper tool_calls payload),
		// fall back to a direct single-pass call so the user gets a meaningful answer
		// rather than "please wait while I fetch…".
		if looksLikeUnexecutedToolCall(content) {
			if logger != nil {
				logger.Warn("agent output looks like unexecuted tool call; falling back to direct inference",
					"model", model, "output_preview", truncate(content, 120))
			}
			return callInference(ctx, systemContext, req.Messages, logger)
		}
		return content, nil
	}
	// Small/smoke models (e.g. qwen3.5:0.8b) and no-MCP-gateway path both use
	// callInference (direct HTTP to Ollama with stream:false). Langchaingo's
	// ChatRequest.Stream field is omitempty so false is never sent; Ollama then
	// defaults to streaming mode where Qwen3.5 thinking-mode chunks have empty
	// message.content, causing GenerateFromSinglePrompt to return "".
	return callInference(ctx, systemContext, req.Messages, logger)
}

// buildSystemContextWithHistory appends prior conversation turns (all but the last user message)
// to the system context block. This gives the langchain agent executor the full conversation
// history as part of its system prompt, since the executor only accepts a single `input` string.
func buildSystemContextWithHistory(systemContext string, messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}) string {
	// Find prior turns: everything except the final user message.
	prior := priorMessages(messages)
	if len(prior) == 0 {
		return systemContext
	}
	var b strings.Builder
	b.WriteString(systemContext)
	b.WriteString("\n\n## Conversation history\n")
	for _, m := range prior {
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return b.String()
}

// buildAgentInput combines the system context (with history) and the current user message
// into the single input string expected by the langchain agent executor.
func buildAgentInput(systemContextWithHistory, currentInput string) string {
	if systemContextWithHistory == "" {
		return currentInput
	}
	return systemContextWithHistory + "\n\n" + currentInput
}

// lastUserMessage returns the content of the last user-role message in messages,
// or the content of the last message of any role if no user message exists.
func lastUserMessage(messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == roleUser {
			return messages[i].Content
		}
	}
	if len(messages) > 0 {
		return messages[len(messages)-1].Content
	}
	return ""
}

// priorMessages returns all messages except the final user message (i.e. the conversation
// history the model should use as context for the current turn).
func priorMessages(messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}) []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
} {
	// Find last user message index.
	lastUser := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == roleUser {
			lastUser = i
			break
		}
	}
	if lastUser <= 0 {
		return nil
	}
	return messages[:lastUser]
}

// buildSystemContext composes system context per CYNAI.PMAGNT.LLMContextComposition order:
// baseline+role (instructionsContent) -> project -> task -> user additional context.
func buildSystemContext(instructionsContent string, req *InternalChatCompletionRequest) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(instructionsContent))
	if req.ProjectID != "" {
		b.WriteString("\n\n## Project context\nproject_id: ")
		b.WriteString(req.ProjectID)
	}
	if req.TaskID != "" {
		b.WriteString("\n\n## Task context\ntask_id: ")
		b.WriteString(req.TaskID)
	}
	if req.AdditionalContext != "" {
		b.WriteString("\n\n## User additional context\n")
		b.WriteString(strings.TrimSpace(req.AdditionalContext))
	}
	return b.String()
}

// truncate returns at most n runes of s, appending "…" if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// udsPlainHost is the rewritten host used when dialing over a Unix domain socket.
// The actual connection goes through the custom transport's DialContext.
const udsPlainHost = "http://localhost"

// resolveInferenceClient converts an http+unix:// URL into a plain http://localhost
// URL and an http.Client with a Unix socket dialer. For standard http:// URLs it
// returns the input unchanged with a plain client using the given timeout.
func resolveInferenceClient(baseURL string, timeout time.Duration) (serverURL string, client *http.Client) {
	trimmed := strings.TrimSpace(baseURL)
	if strings.HasPrefix(trimmed, "http+unix://") {
		encoded := strings.TrimPrefix(trimmed, "http+unix://")
		if idx := strings.Index(encoded, "/"); idx > 0 {
			encoded = encoded[:idx]
		}
		if sockPath, err := url.PathUnescape(encoded); err == nil && sockPath != "" {
			transport := &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
				},
			}
			return udsPlainHost, &http.Client{Timeout: timeout, Transport: transport}
		}
	}
	return trimmed, &http.Client{Timeout: timeout}
}

// callInference sends messages to Ollama /api/chat using NDJSON streaming (stream:true).
// It accumulates the full visible text from streaming chunks, strips <think>...</think>
// blocks per CYNAI.PMAGNT.StreamingAssistantOutput, and returns the cleaned result.
func callInference(ctx context.Context, systemContext string, messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}, logger *slog.Logger) (string, error) {
	baseURL, model := resolveOllamaConfig()
	chatMessages := buildChatMessages(systemContext, messages)
	inferenceURL, inferenceClient := resolveInferenceClient(baseURL, inferenceHTTPTimeout)
	resp, err := doInferenceRequest(ctx, inferenceClient, inferenceURL, model, chatMessages)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	content, err := readInferenceStream(resp.Body, logger)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stripThinkBlocks(content)), nil
}

// resolveOllamaConfig returns the Ollama base URL and model from environment variables.
func resolveOllamaConfig() (baseURL, model string) {
	baseURL = os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("INFERENCE_URL")
	}
	if baseURL == "" {
		baseURL = pmaDefaultOllamaURL
	}
	model = os.Getenv("INFERENCE_MODEL")
	if model == "" {
		model = pmaDefaultModel
	}
	return baseURL, model
}

// buildChatMessages assembles the Ollama /api/chat messages array with optional system context.
func buildChatMessages(systemContext string, messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}) []map[string]string {
	chatMessages := make([]map[string]string, 0, len(messages)+1)
	if strings.TrimSpace(systemContext) != "" {
		chatMessages = append(chatMessages, map[string]string{
			"role":    "system",
			"content": strings.TrimSpace(systemContext),
		})
	}
	for _, m := range messages {
		chatMessages = append(chatMessages, map[string]string{
			"role":    strings.TrimSpace(m.Role),
			"content": m.Content,
		})
	}
	return chatMessages
}

// doInferenceRequest builds and executes the Ollama /api/chat HTTP request.
func doInferenceRequest(ctx context.Context, client *http.Client, inferenceURL, model string, chatMessages []map[string]string) (*http.Response, error) {
	chatURL := strings.TrimSuffix(inferenceURL, "/") + "/api/chat"
	body := map[string]interface{}{
		"model":    model,
		"messages": chatMessages,
		"stream":   true,
	}
	if n := ollamaNumCtxFromEnv(); n > 0 {
		body["options"] = map[string]interface{}{"num_ctx": n}
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("inference returned %s", resp.Status)
	}
	return resp, nil
}

// readInferenceStream reads Ollama NDJSON streaming response chunks and accumulates content.
// Returns the concatenated content with no think-block processing applied.
func readInferenceStream(body interface{ Read([]byte) (int, error) }, logger *slog.Logger) (string, error) {
	var sb strings.Builder
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		content, done, err := parseInferenceChunk(line, logger)
		if err != nil {
			return "", err
		}
		sb.WriteString(content)
		if done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading inference stream: %w", err)
	}
	return sb.String(), nil
}

// parseInferenceChunk parses one NDJSON line from an Ollama streaming response.
// Returns (content, done, error).
func parseInferenceChunk(line []byte, logger *slog.Logger) (content string, done bool, err error) {
	var chunk struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Done  bool   `json:"done"`
		Error string `json:"error"`
	}
	if jsonErr := json.Unmarshal(line, &chunk); jsonErr != nil {
		if logger != nil {
			logger.Warn("callInference: failed to parse chunk", "error", jsonErr)
		}
		return "", false, nil
	}
	if chunk.Error != "" {
		return "", false, fmt.Errorf("inference error: %s", chunk.Error)
	}
	return chunk.Message.Content, chunk.Done, nil
}
