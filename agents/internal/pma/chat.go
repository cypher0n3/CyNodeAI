// Package pma provides internal chat completion for orchestrator handoff.
// See docs/tech_specs/cynode_pma.md (request source and handoff).
package pma

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/tmc/langchaingo/agents"
)

const roleUser = "user"

// LangchainCompletionTimeout caps each getCompletionContent attempt (primary and empty-output retry).
// Align with orchestrator chatCompletionTimeout / pmaclient defaultPMAHTTPTimeout (300s): qwen3 with
// tools can use multiple agent iterations and must not hit context deadline before the gateway does.
const LangchainCompletionTimeout = 300 * time.Second

// InternalChatCompletionRequest is the body for POST /internal/chat/completion (orchestrator handoff).
// Optional fields support full context order per CYNAI.PMAGNT.LLMContextComposition: project, task, additional context.
// When Stream is true the response is application/x-ndjson: Ollama-backed paths emit real per-chunk deltas;
// completed langchain turns emit progressive delta chunks (see pmaStreamDeltaRunes in langchain.go).
type InternalChatCompletionRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	ProjectID         string `json:"project_id,omitempty"`
	TaskID            string `json:"task_id,omitempty"`
	UserID            string `json:"user_id,omitempty"`
	AdditionalContext string `json:"additional_context,omitempty"`
	Stream            bool   `json:"stream,omitempty"`
}

// InternalChatCompletionResponse is the response body.
// Thinking holds extracted think-block reasoning (REQ-PMAGNT-0117/0122); visible text is Content.
type InternalChatCompletionResponse struct {
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"`
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
		if req.Stream && canStreamCompletion(&req) {
			streamCompletionToWriter(r.Context(), w, instructionsContent, &req, logger)
			return
		}
		if req.Stream && !canStreamCompletion(&req) {
			streamCompletionLangchainToWriter(detached, w, instructionsContent, &req, logger)
			return
		}
		content, thinking, httpStatus := resolveContent(detached, instructionsContent, &req, logger)
		writeJSON(w, httpStatus, InternalChatCompletionResponse{Content: content, Thinking: thinking})
	}
}

// canStreamCompletion returns true when the request always uses direct Ollama streaming (no MCP
// or non-capable model). Capable+MCP still uses stream=true on this endpoint but takes the
// streamCompletionLangchainToWriter path (real Ollama deltas on inference fallback; chunked deltas
// after a completed langchain turn).
func canStreamCompletion(req *InternalChatCompletionRequest) bool {
	mcpClient := NewMCPClient()
	model := os.Getenv("INFERENCE_MODEL")
	if model == "" {
		model = pmaDefaultModel
	}
	return mcpClient.BaseURL == "" || !isCapableModel(model)
}

// streamCompletionLangchainToWriter runs the capable-model + MCP path with real Ollama NDJSON
// streaming when falling back to direct inference; completed langchain turns emit progressive
// delta chunks (pmaStreamDeltaRunes). Mirrors resolveContent retries on empty output.
func streamCompletionLangchainToWriter(detachedRoot context.Context, w http.ResponseWriter, instructionsContent string, req *InternalChatCompletionRequest, logger *slog.Logger) {
	runCtx, cancel := context.WithTimeout(detachedRoot, LangchainCompletionTimeout)
	defer cancel()
	out := streamTryLangchainNDJSON(runCtx, w, instructionsContent, req, logger)
	if out == streamNDJSONOK {
		return
	}
	if out == streamNDJSONError {
		writeJSON(w, http.StatusInternalServerError, InternalChatCompletionResponse{})
		return
	}
	// Empty — retry with only the current message (same as resolveContent).
	if logger != nil {
		logger.Warn("stream completion produced empty output; retrying with current message only",
			"original_msg_count", len(req.Messages))
	}
	retryCtx, retryCancel := context.WithTimeout(detachedRoot, LangchainCompletionTimeout)
	defer retryCancel()
	stripped := stripToCurrentMessage(req)
	out = streamTryLangchainNDJSON(retryCtx, w, instructionsContent, stripped, logger)
	if out != streamNDJSONOK {
		writeJSON(w, http.StatusInternalServerError, InternalChatCompletionResponse{})
	}
}

type streamNDJSONOutcome int

const (
	streamNDJSONOK streamNDJSONOutcome = iota
	streamNDJSONEmpty
	streamNDJSONError
)

// streamTryLangchainNDJSON performs one completion attempt for the streaming capable-model path.
func streamTryLangchainNDJSON(ctx context.Context, w http.ResponseWriter, instructionsContent string, req *InternalChatCompletionRequest, logger *slog.Logger) streamNDJSONOutcome {
	systemContext := buildSystemContext(instructionsContent, req)
	mcpClient := NewMCPClient()
	model := os.Getenv("INFERENCE_MODEL")
	if model == "" {
		model = pmaDefaultModel
	}
	if mcpClient.BaseURL != "" && isCapableModel(model) {
		return streamCapableModelNDJSON(ctx, w, systemContext, req, mcpClient, model, logger)
	}
	return streamOllamaChatToNDJSONOutcome(ctx, w, systemContext, req, logger)
}

func streamCapableModelNDJSON(ctx context.Context, w http.ResponseWriter, systemContext string, req *InternalChatCompletionRequest, mcpClient *MCPClient, model string, logger *slog.Logger) streamNDJSONOutcome {
	systemContextWithHistory := buildSystemContextWithHistory(systemContext, req.Messages)
	currentInput := lastUserMessage(req.Messages)
	visible, thinkLC, err := runCompletionWithLangchainWithTimeout(
		ctx,
		buildAgentInput(systemContextWithHistory, currentInput),
		mcpClient,
		logger,
		LangchainCompletionTimeout,
	)
	if err != nil {
		if errors.Is(err, agents.ErrNotFinished) || errors.Is(err, agents.ErrAgentNoReturn) {
			if logger != nil {
				logger.Warn("langchain agent did not complete; falling back to direct inference", "error", err)
			}
			return streamOllamaChatToNDJSONOutcome(ctx, w, systemContext, req, logger)
		}
		return streamNDJSONError
	}
	if looksLikeUnexecutedToolCall(visible) {
		if logger != nil {
			logger.Warn("agent output looks like unexecuted tool call; falling back to direct inference",
				"model", model, "output_preview", truncate(visible, 120))
		}
		return streamOllamaChatToNDJSONOutcome(ctx, w, systemContext, req, logger)
	}
	if strings.TrimSpace(visible) == "" {
		if logger != nil {
			logger.Warn("langchain agent returned empty output; falling back to direct inference", "model", model)
		}
		return streamOllamaChatToNDJSONOutcome(ctx, w, systemContext, req, logger)
	}
	return streamLangchainNDJSONToWriter(w, visible, thinkLC, logger)
}

func streamLangchainNDJSONToWriter(w http.ResponseWriter, visible, thinkLC string, logger *slog.Logger) streamNDJSONOutcome {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flushResponseWriter(w)
	if err := writeLangchainNDJSONStream(w, visible, thinkLC); err != nil {
		if logger != nil {
			if isClientDisconnect(err) {
				logger.Debug("client disconnected during langchain NDJSON stream", "error", err)
			} else {
				logger.Error("write langchain NDJSON stream", "error", err)
			}
		}
	}
	return streamNDJSONOK
}

// streamOllamaChatToNDJSONOutcome streams Ollama /api/chat (stream:true) to NDJSON deltas.
func streamOllamaChatToNDJSONOutcome(ctx context.Context, w http.ResponseWriter, systemContext string, req *InternalChatCompletionRequest, logger *slog.Logger) streamNDJSONOutcome {
	chatMessages := buildChatMessages(systemContext, req.Messages)
	had, err, headersSent := streamOllamaChatToNDJSON(ctx, w, chatMessages, logger)
	if err != nil {
		if headersSent {
			if logger != nil {
				if isClientDisconnect(err) {
					logger.Debug("client disconnected during Ollama NDJSON stream", "error", err)
				} else {
					logger.Error("stream Ollama NDJSON failed after response start", "error", err)
				}
			}
			return streamNDJSONOK
		}
		if logger != nil {
			logger.Error("stream Ollama NDJSON failed", "error", err)
		}
		return streamNDJSONError
	}
	if !had {
		return streamNDJSONEmpty
	}
	return streamNDJSONOK
}

// streamOllamaChatToNDJSON writes NDJSON (iteration_start, deltas from Ollama chunks).
// Headers and 200 are sent only when the first non-empty delta arrives so an empty model
// stream does not commit a response (callers can retry or return 500).
// Returns whether any non-empty delta was emitted, an error if any, and whether HTTP headers
// were already sent (callers must not send a JSON error body if true).
func streamOllamaChatToNDJSON(ctx context.Context, w http.ResponseWriter, chatMessages []map[string]string, logger *slog.Logger) (hadDelta bool, err error, headersSent bool) {
	baseURL, model := resolveOllamaConfig()
	inferenceURL, inferenceClient := resolveInferenceClient(baseURL, inferenceHTTPTimeout)
	resp, err := doInferenceRequest(ctx, inferenceClient, inferenceURL, model, chatMessages, true)
	if err != nil {
		return false, err, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("inference returned %s", resp.Status), false
	}
	scanner := bufio.NewScanner(resp.Body)
	const maxScan = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxScan)
	var enc *json.Encoder
	started := false
	clf := newStreamingClassifier()
	hadDelta, err, started = scanOllamaInferenceNDJSON(ctx, w, scanner, clf, logger, &enc, &started)
	if err != nil {
		return hadDelta, err, started
	}
	if started && enc != nil {
		if err := enc.Encode(map[string]bool{"done": true}); err != nil {
			return hadDelta, err, true
		}
		flushResponseWriter(w)
	}
	return hadDelta, nil, started
}

func scanOllamaInferenceNDJSON(ctx context.Context, w http.ResponseWriter, scanner *bufio.Scanner, clf *streamingClassifier, logger *slog.Logger, enc **json.Encoder, started *bool) (hadDelta bool, err error, headersSent bool) {
	for scanner.Scan() {
		if ctx.Err() != nil {
			return hadDelta, ctx.Err(), *started
		}
		line := scanner.Bytes()
		content, done, err := parseInferenceChunk(line, logger)
		if err != nil {
			return hadDelta, err, *started
		}
		emitted, hdrsSent, err := emitNDJSONEmissions(w, enc, started, clf.Feed(content))
		if err != nil {
			return hadDelta, err, hdrsSent
		}
		if emitted {
			hadDelta = true
		}
		if done {
			break
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return hadDelta, fmt.Errorf("reading inference stream: %w", scanErr), *started
	}
	emitted, hdrsSent, err := emitNDJSONEmissions(w, enc, started, clf.Flush())
	if err != nil {
		return hadDelta, err, hdrsSent
	}
	if emitted {
		hadDelta = true
	}
	return hadDelta, nil, *started
}

// emitNDJSONEmissions writes classifier emissions (delta, thinking, tool_call) to the NDJSON stream.
func emitNDJSONEmissions(w http.ResponseWriter, enc **json.Encoder, started *bool, emissions []streamEmitted) (emittedAny, headersSentOnErr bool, err error) {
	for _, em := range emissions {
		if !emissionWorthEmitting(em) {
			continue
		}
		if err := ensureNDJSONStreamStarted(w, enc, started); err != nil {
			return emittedAny, true, err
		}
		ok, encErr := encodeNDJSONStreamEmission(*enc, em)
		if encErr != nil {
			return emittedAny, true, encErr
		}
		if !ok {
			continue
		}
		flushResponseWriter(w)
		emittedAny = true
	}
	return emittedAny, false, nil
}

func emissionWorthEmitting(em streamEmitted) bool {
	if em.Kind == streamEmitDelta {
		return em.Text != ""
	}
	return strings.TrimSpace(em.Text) != ""
}

func ensureNDJSONStreamStarted(w http.ResponseWriter, enc **json.Encoder, started *bool) error {
	if *started {
		return nil
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flushResponseWriter(w)
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false)
	if err := e.Encode(map[string]int{"iteration_start": 1}); err != nil {
		return err
	}
	flushResponseWriter(w)
	*enc = e
	*started = true
	return nil
}

func encodeNDJSONStreamEmission(enc *json.Encoder, em streamEmitted) (encoded bool, err error) {
	switch em.Kind {
	case streamEmitDelta:
		return true, enc.Encode(map[string]string{"delta": em.Text})
	case streamEmitThinking:
		return true, enc.Encode(map[string]string{"thinking": em.Text})
	case streamEmitToolCall:
		return true, enc.Encode(map[string]any{
			"tool_call": map[string]string{"name": "stream", "arguments": em.Text},
		})
	default:
		return false, nil
	}
}

// streamCompletionToWriter runs direct inference and streams Ollama NDJSON (real token chunks).
func streamCompletionToWriter(ctx context.Context, w http.ResponseWriter, instructionsContent string, req *InternalChatCompletionRequest, logger *slog.Logger) {
	systemContext := buildSystemContext(instructionsContent, req)
	chatMessages := buildChatMessages(systemContext, req.Messages)
	had, err, headersSent := streamOllamaChatToNDJSON(ctx, w, chatMessages, logger)
	if err != nil {
		logStreamCompletionInferenceError(logger, err, headersSent)
		if !headersSent {
			writeJSON(w, http.StatusInternalServerError, InternalChatCompletionResponse{})
		}
		return
	}
	if !had {
		if logger != nil {
			logger.Error("stream completion: Ollama returned no content")
		}
		writeJSON(w, http.StatusInternalServerError, InternalChatCompletionResponse{})
	}
}

func logStreamCompletionInferenceError(logger *slog.Logger, err error, headersSent bool) {
	if logger == nil {
		return
	}
	if headersSent {
		if isClientDisconnect(err) {
			logger.Debug("stream completion: client disconnected", "error", err)
		} else {
			logger.Error("stream completion inference failed after response start", "error", err)
		}
		return
	}
	logger.Error("stream completion inference request failed", "error", err)
}

// streamCompletionWriteChunk parses one inference chunk, optionally encodes a delta and flushes.
// Returns (streamDone, stopLoop). stopLoop is true on encode error or parse error.
func streamCompletionWriteChunk(enc *json.Encoder, w http.ResponseWriter, line []byte, logger *slog.Logger) (streamDone, stopLoop bool) {
	if len(line) == 0 {
		return false, false
	}
	content, done, err := parseInferenceChunk(line, logger)
	if err != nil {
		logger.Warn("stream completion chunk error", "error", err)
		return false, true
	}
	if content != "" {
		if encErr := enc.Encode(map[string]string{"delta": content}); encErr != nil {
			return false, true
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	return done, false
}

// resolveContent obtains the completion content for req, retrying with just the current user
// message if the full-history call returns empty output (context-window overflow).
// Returns (content, httpStatusCode).
//
// detachedRoot must be from context.WithoutCancel(r.Context()) (see ChatCompletionHandler): the
// primary completion uses a timeout derived from it, not the HTTP request context. Otherwise the
// orchestrator/gateway deadline on the incoming request cancels the LLM+MCP run early and surfaces
// as 500 on streaming proxy calls even though the gateway client timeout is much longer.
func resolveContent(detachedRoot context.Context, instructionsContent string, req *InternalChatCompletionRequest, logger *slog.Logger) (content, thinking string, httpStatus int) {
	runCtx, cancel := context.WithTimeout(detachedRoot, LangchainCompletionTimeout)
	defer cancel()
	content, thinking, err := getCompletionContent(runCtx, instructionsContent, req, logger)
	if err != nil {
		logger.Error("chat completion inference error", "error", err)
		return "", "", http.StatusInternalServerError
	}
	if strings.TrimSpace(content) != "" || strings.TrimSpace(thinking) != "" {
		return content, thinking, http.StatusOK
	}
	// Context may have exceeded the model's window — retry with only the current message.
	// Use a fresh context derived from the detached root captured at entry so the
	// original gateway deadline (which may be nearly expired) does not kill the retry.
	logger.Warn("chat completion produced empty output; retrying with current message only",
		"original_msg_count", len(req.Messages))
	stripped := stripToCurrentMessage(req)
	retryCtx, retryCancel := context.WithTimeout(detachedRoot, LangchainCompletionTimeout)
	defer retryCancel()
	content, thinking, err = getCompletionContent(retryCtx, instructionsContent, stripped, logger)
	if err != nil {
		logger.Error("chat completion retry failed", "error", err)
		return "", "", http.StatusInternalServerError
	}
	if strings.TrimSpace(content) == "" && strings.TrimSpace(thinking) == "" {
		logger.Error("chat completion produced empty output after retry")
		return "", "", http.StatusInternalServerError
	}
	return content, thinking, http.StatusOK
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
func getCompletionContent(ctx context.Context, instructionsContent string, req *InternalChatCompletionRequest, logger *slog.Logger) (content, thinking string, err error) {
	systemContext := buildSystemContext(instructionsContent, req)
	mcpClient := NewMCPClient()
	model := os.Getenv("INFERENCE_MODEL")
	if model == "" {
		model = pmaDefaultModel
	}
	if mcpClient.BaseURL != "" && isCapableModel(model) {
		return getCompletionContentCapableLangchain(ctx, systemContext, req, mcpClient, model, logger)
	}
	// Small/smoke models (e.g. qwen3.5:0.8b) and no-MCP-gateway path use callInference
	// (Ollama /api/chat with stream:true per CYNAI.PMAGNT.StreamingAssistantOutput).
	return callInference(ctx, systemContext, req.Messages, logger)
}

func getCompletionContentCapableLangchain(ctx context.Context, systemContext string, req *InternalChatCompletionRequest, mcpClient *MCPClient, model string, logger *slog.Logger) (content, thinking string, err error) {
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
	visible, thinkLC, err := runCompletionWithLangchainWithTimeout(
		ctx,
		buildAgentInput(systemContextWithHistory, currentInput),
		mcpClient,
		logger,
		LangchainCompletionTimeout,
	)
	if err != nil {
		if errors.Is(err, agents.ErrNotFinished) || errors.Is(err, agents.ErrAgentNoReturn) {
			if logger != nil {
				logger.Warn("langchain agent did not complete; falling back to direct inference",
					"error", err)
			}
			return callInference(ctx, systemContext, req.Messages, logger)
		}
		return "", "", err
	}
	if looksLikeUnexecutedToolCall(visible) {
		if logger != nil {
			logger.Warn("agent output looks like unexecuted tool call; falling back to direct inference",
				"model", model, "output_preview", truncate(visible, 120))
		}
		return callInference(ctx, systemContext, req.Messages, logger)
	}
	if strings.TrimSpace(visible) == "" {
		if logger != nil {
			logger.Warn("langchain agent returned empty output; falling back to direct inference", "model", model)
		}
		return callInference(ctx, systemContext, req.Messages, logger)
	}
	return visible, thinkLC, nil
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

// isClientDisconnect reports whether err is a benign disconnect (client closed the connection
// while the server was still writing the streaming response).
func isClientDisconnect(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var ne *net.OpError
	if errors.As(err, &ne) {
		if errors.Is(ne.Err, syscall.EPIPE) || errors.Is(ne.Err, syscall.ECONNRESET) {
			return true
		}
	}
	s := err.Error()
	return strings.Contains(s, "broken pipe") || strings.Contains(s, "connection reset by peer")
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

// callInference uses Ollama /api/chat with stream:true (CYNAI.PMAGNT.StreamingAssistantOutput).
func callInference(ctx context.Context, systemContext string, messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}, logger *slog.Logger) (visible, thinking string, err error) {
	baseURL, model := resolveOllamaConfig()
	chatMessages := buildChatMessages(systemContext, messages)
	inferenceURL, inferenceClient := resolveInferenceClient(baseURL, inferenceHTTPTimeout)
	resp, err := doInferenceRequest(ctx, inferenceClient, inferenceURL, model, chatMessages, true)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := readInferenceOllamaChatBody(resp.Body, logger)
	if err != nil {
		return "", "", err
	}
	visible, thinking = extractThinkBlocks(raw)
	return strings.TrimSpace(visible), strings.TrimSpace(thinking), nil
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
// callInference and streamCompletionToWriter both use stream:true per CYNAI.PMAGNT.StreamingAssistantOutput;
// readInferenceOllamaChatBody accepts a single JSON response or NDJSON.
func doInferenceRequest(ctx context.Context, client *http.Client, inferenceURL, model string, chatMessages []map[string]string, stream bool) (*http.Response, error) {
	chatURL := strings.TrimSuffix(inferenceURL, "/") + "/api/chat"
	body := map[string]interface{}{
		"model":    model,
		"messages": chatMessages,
		"stream":   stream,
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

// readInferenceOllamaChatBody reads the full /api/chat response body: a single JSON object when
// the server closes after one chunk, or NDJSON lines when stream:true (accumulated raw text).
func readInferenceOllamaChatBody(body io.Reader, logger *slog.Logger) (string, error) {
	b, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return "", nil
	}
	var resp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(b, &resp); err == nil {
		if resp.Error != "" {
			return "", fmt.Errorf("inference error: %s", resp.Error)
		}
		return resp.Message.Content, nil
	}
	return readInferenceStream(bytes.NewReader(b), logger)
}

// readInferenceStream reads Ollama NDJSON streaming response chunks and accumulates content.
// Returns the concatenated content with no think-block processing applied.
func readInferenceStream(body interface{ Read([]byte) (int, error) }, logger *slog.Logger) (string, error) {
	var acc []byte
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
		appendStreamBufferSecure(&acc, []byte(content))
		if done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading inference stream: %w", err)
	}
	return string(acc), nil
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
