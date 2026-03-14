// Package handlers implements OpenAI-compatible chat API per docs/tech_specs/openai_compatible_chat_api.md.
package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/inference"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/pmaclient"
)

// Max total wait for chat completion (REQ-ORCHES-0131).
// Spec allows 90-120 s; set to 120 s to accommodate slow local model loading.
// chatCompletionTimeout is the maximum wall time for a single chat completion request.
// Must be below the user-gateway WRITE_TIMEOUT (300 s).
// Bound: 2 × pmaLangchainCompletionTimeout (2×90 s = 180 s) + proxy overhead ≈ 200 s.
const chatCompletionTimeout = 200 * time.Second

// chatHistoryLimit caps the number of prior messages loaded from the thread for context.
// Prevents unbounded context growth for long-running sessions.
const chatHistoryLimit = 40

// chatHistoryCharBudget caps the total character count of history messages sent to the
// agent. With OLLAMA_NUM_CTX=32 768 and ~4 chars/token, total capacity is ~131 000 chars.
// We reserve ~40 000 chars for system prompt, tool schemas, and the current turn,
// leaving 24 000 chars (~6 000 tokens, roughly 10-15 short exchanges) for history.
const chatHistoryCharBudget = 24000

// Retries for transient inference failures (REQ-ORCHES-0132).
const chatCompletionMaxRetries = 3
const chatCompletionBackoffBase = 500 * time.Millisecond

// Effective model default per spec: omitted or empty model MUST behave as cynodeai.pm.
const EffectiveModelPM = "cynodeai.pm"
const managedServiceTypePMA = "pma"

const secretRedacted = "SECRET_REDACTED"

const completionFailedMsg = "Completion failed"
const inferenceFailedCode = "orchestrator_inference_failed"

const chatRoleUser = "user"
const chatRoleAssistant = "assistant"

var (
	apiKeyLikePattern = regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token|bearer)\s*[:=]\s*[a-zA-Z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+`)
	skPrefixPattern   = regexp.MustCompile(`sk-[a-zA-Z0-9\-._]{20,}`)
)

// OpenAIChatHandler handles GET /v1/models and POST /v1/chat/completions.
type OpenAIChatHandler struct {
	db                   database.Store
	logger               *slog.Logger
	inferenceURL         string
	inferenceModel       string
	workerAPIBearerToken string
}

// NewOpenAIChatHandler creates a handler for the OpenAI-compatible chat surface.
// PMA routing is only via worker-reported endpoints (capability managed_services_status); no env fallback.
// workerAPIBearerToken is sent when calling worker proxy URLs so the worker-api accepts the request.
func NewOpenAIChatHandler(db database.Store, logger *slog.Logger, inferenceURL, inferenceModel, workerAPIBearerToken string) *OpenAIChatHandler {
	if inferenceModel == "" {
		inferenceModel = pmaModelDefault
	}
	return &OpenAIChatHandler{
		db:                   db,
		logger:               logger,
		inferenceURL:         inferenceURL,
		inferenceModel:       inferenceModel,
		workerAPIBearerToken: workerAPIBearerToken,
	}
}

// ListModels returns GET /v1/models in OpenAI list-models format.
// Exposes cynodeai.pm and the configured inference model(s) the user is authorized to use.
func (h *OpenAIChatHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	modelList := []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
	}{
		{ID: EffectiveModelPM, Object: "model", Created: 0},
	}
	if h.inferenceModel != "" {
		modelList = append(modelList, struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
		}{ID: h.inferenceModel, Object: "model", Created: 0})
	}
	writeOpenAIJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   modelList,
	})
}

// ChatCompletions handles POST /v1/chat/completions with pipeline: auth (already done), decode, project_id, redact, persist user message, route, persist assistant, return.
// When stream=true is requested the response uses Server-Sent Events per CYNAI.USRGWY.OpenAIChatApi.Streaming.
//
//nolint:gocognit,gocyclo // single HTTP handler with sequential validation and branching for stream vs non-stream
func (h *OpenAIChatHandler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		writeOpenAIError(w, http.StatusUnauthorized, "authentication_required", "Authentication required")
		return
	}
	req, errCode, errMsg := h.decodeAndValidateChatRequest(r)
	if errCode != 0 {
		writeOpenAIError(w, errCode, "invalid_request", errMsg)
		return
	}
	// Effective model per spec: request "model" if present and non-empty, else cynodeai.pm.
	effectiveModel := req.Model
	if strings.TrimSpace(effectiveModel) == "" {
		effectiveModel = EffectiveModelPM
	}
	projectID := projectIDFromHeader(r)
	redacted, kinds := redactSecrets(req.Messages)
	thread, err := h.db.GetOrCreateActiveChatThread(ctx, *userID, projectID)
	if err != nil {
		h.logger.Error("get or create chat thread", "error", err)
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to get chat thread")
		return
	}
	lastUserContent := lastUserMessageContent(redacted)
	if lastUserContent == "" {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "At least one user message is required")
		return
	}
	if _, err := h.db.AppendChatMessage(ctx, thread.ID, chatRoleUser, lastUserContent, nil); err != nil {
		h.logger.Error("append user message", "error", err)
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to persist message")
		return
	}
	// Load full thread history (capped) to give agents multi-turn context.
	threadHistory, err := h.db.ListChatMessages(ctx, thread.ID, chatHistoryLimit)
	if err != nil {
		h.logger.Warn("failed to load chat history; falling back to single message", "error", err)
		threadHistory = nil
	}
	var contextMessages []userapi.ChatMessage
	if len(threadHistory) > 0 {
		contextMessages = trimHistoryToCharBudget(threadHistory, chatHistoryCharBudget)
	} else {
		contextMessages = redacted
	}
	// REQ-ORCHES-0131: enforce maximum total wait duration.
	timeoutCtx, cancel := context.WithTimeout(ctx, chatCompletionTimeout)
	defer cancel()
	start := time.Now()
	// True token-by-token streaming when client asked for stream and we route to PMA (proxy or direct).
	if req.Stream && effectiveModel == EffectiveModelPM {
		cand := h.resolvePMAEndpointCandidate(timeoutCtx)
		if cand.endpoint != "" {
			prepareSSEResponse(w)
			err := h.completeViaPMAStream(timeoutCtx, w, cand, contextMessages, thread.ID, userID, projectID, start, effectiveModel, uuid.New().String(), nil)
			if err != nil {
				writeSSEError(w, "stream_error", err.Error())
			}
			return
		}
	}
	content, status, code, msg := h.routeAndComplete(timeoutCtx, effectiveModel, contextMessages, lastUserContent)
	if status != 0 {
		if req.Stream {
			// Headers not yet sent; we can send the pre-stream SSE error.
			prepareSSEResponse(w)
			writeSSEError(w, code, msg)
		} else {
			writeOpenAIError(w, status, code, msg)
		}
		return
	}
	if _, err := h.db.AppendChatMessage(ctx, thread.ID, chatRoleAssistant, content, nil); err != nil {
		h.logger.Error("append assistant message", "error", err)
	}
	durationMs := int(time.Since(start).Milliseconds())
	_ = h.db.CreateChatAuditLog(ctx, &models.ChatAuditLog{
		UserID:           userID,
		ProjectID:        projectID,
		Outcome:          "success",
		RedactionApplied: len(kinds) > 0,
		RedactionKinds:   kindsJSON(kinds),
		DurationMs:       &durationMs,
	})
	if req.Stream {
		// Emit content as SSE degraded-mode stream (full turn in one delta).
		// Per spec: when upstream cannot provide true token deltas, still emit SSE
		// with bounded in-progress status plus a terminal completion event.
		completionID := uuid.New().String()
		prepareSSEResponse(w)
		emitContentAsSSE(w, completionID, effectiveModel, content)
		return
	}
	writeOpenAIJSON(w, http.StatusOK, buildChatCompletionsResponse(effectiveModel, content)) //nolint:exhaustruct // response struct built inline; exhaustruct wants all fields set
}

func decodeOpenAIRequest(r *http.Request, dest interface{}) (status int, errMsg string) {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		return http.StatusBadRequest, "Invalid request body"
	}
	return 0, ""
}

// decodeAndValidateOpenAIRequest decodes the body into dest then runs validate. Returns (0, "") on success.
func decodeAndValidateOpenAIRequest(r *http.Request, dest interface{}, validate func() (int, string)) (status int, errMsg string) {
	if status, errMsg = decodeOpenAIRequest(r, dest); status != 0 {
		return status, errMsg
	}
	return validate()
}

func (h *OpenAIChatHandler) decodeAndValidateChatRequest(r *http.Request) (req userapi.ChatCompletionsRequest, status int, errMsg string) {
	status, errMsg = decodeAndValidateOpenAIRequest(r, &req, func() (int, string) {
		if chatRequestMessagesEmpty(&req) {
			return http.StatusBadRequest, "messages is required and must be non-empty"
		}
		return 0, ""
	})
	return req, status, errMsg
}

func chatRequestMessagesEmpty(req *userapi.ChatCompletionsRequest) bool {
	return len(req.Messages) == 0
}

func lastUserMessageContent(redacted []userapi.ChatMessage) string {
	for i := len(redacted) - 1; i >= 0; i-- {
		if redacted[i].Role == chatRoleUser {
			return redacted[i].Content
		}
	}
	return ""
}

// isTransientInferenceError returns true for errors that warrant retry (REQ-ORCHES-0132).
func isTransientInferenceError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "connection refused") || strings.Contains(s, "returned 5")
}

// routeAndComplete implements Chat Completion Routing Path per openai_compatible_chat_api.md § Chat Completion Routing Path.
// Enforces max wait via ctx timeout (REQ-ORCHES-0131) and retries transient failures (REQ-ORCHES-0132).
func (h *OpenAIChatHandler) routeAndComplete(ctx context.Context, effectiveModel string, redacted []userapi.ChatMessage, lastUserContent string) (content string, status int, code, msg string) {
	if effectiveModel == EffectiveModelPM {
		return h.completeViaPMA(ctx, effectiveModel, redacted)
	}
	return h.completeViaDirectInference(ctx, effectiveModel, lastUserContent)
}

func (h *OpenAIChatHandler) completeViaPMA(ctx context.Context, effectiveModel string, redacted []userapi.ChatMessage) (content string, status int, code, msg string) {
	candidate := h.resolvePMAEndpointCandidate(ctx)
	if candidate.endpoint == "" {
		h.logger.Warn("PMA base URL not configured; cannot route to cynodeai.pm")
		return "", http.StatusServiceUnavailable, "model_unavailable", "PM agent is not available"
	}
	workerToken := strings.TrimSpace(candidate.workerAPIBearerToken)
	tokenSource := "node"
	if workerToken == "" {
		workerToken = h.workerAPIBearerToken
		tokenSource = "global"
	}
	h.logger.Info(
		"pma proxy auth token selected",
		"token_source",
		tokenSource,
		"token_len",
		len(workerToken),
		"endpoint",
		candidate.endpoint,
	)
	msgs := make([]pmaclient.ChatMessage, 0, len(redacted))
	for _, m := range redacted {
		msgs = append(msgs, pmaclient.ChatMessage{Role: m.Role, Content: m.Content})
	}
	call := func() (string, error) {
		return pmaclient.CallChatCompletion(ctx, nil, candidate.endpoint, msgs, workerToken)
	}
	return h.runCompletionWithRetry(ctx, effectiveModel, "pma", "PMA chat completion failed", call)
}

// completeViaPMAStream streams completion from PMA token-by-token, persists the full reply and audit, then sends [DONE].
// chunkID is used as the SSE chunk id (e.g. completion id or response_id); assistantMeta is optional message metadata (e.g. response_id for /v1/responses).
// Caller must have called prepareSSEResponse(w) before calling this.
func (h *OpenAIChatHandler) completeViaPMAStream(ctx context.Context, w http.ResponseWriter, cand pmaEndpointCandidate, redacted []userapi.ChatMessage, threadID uuid.UUID, userID, projectID *uuid.UUID, start time.Time, effectiveModel, chunkID string, assistantMeta *string) error {
	workerToken := strings.TrimSpace(cand.workerAPIBearerToken)
	if workerToken == "" {
		workerToken = h.workerAPIBearerToken
	}
	msgs := make([]pmaclient.ChatMessage, 0, len(redacted))
	for _, m := range redacted {
		msgs = append(msgs, pmaclient.ChatMessage{Role: m.Role, Content: m.Content})
	}
	var fullContent strings.Builder
	stop := "stop"
	open := userapi.ChatCompletionChunk{
		ID:      chunkID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   effectiveModel,
		Choices: []userapi.ChatCompletionChunkChoice{
			{Index: 0, Delta: userapi.ChatCompletionChunkDelta{Role: "assistant"}, FinishReason: nil},
		},
	}
	if b, err := json.Marshal(open); err == nil {
		writeSSEEvent(w, string(b))
	}
	onDelta := func(delta string) error {
		fullContent.WriteString(delta)
		chunk := buildChatCompletionChunk(chunkID, effectiveModel, delta, nil)
		b, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		writeSSEEvent(w, string(b))
		return nil
	}
	if err := pmaclient.CallChatCompletionStream(ctx, nil, cand.endpoint, msgs, workerToken, onDelta); err != nil {
		return err
	}
	content := fullContent.String()
	if _, err := h.db.AppendChatMessage(ctx, threadID, chatRoleAssistant, content, assistantMeta); err != nil {
		h.logger.Error("append assistant message (stream)", "error", err)
	}
	durationMs := int(time.Since(start).Milliseconds())
	_ = h.db.CreateChatAuditLog(ctx, &models.ChatAuditLog{
		UserID:           userID,
		ProjectID:        projectID,
		Outcome:          "success",
		RedactionApplied: false,
		DurationMs:       &durationMs,
	})
	final := buildChatCompletionChunk(chunkID, effectiveModel, "", &stop)
	if b, err := json.Marshal(final); err == nil {
		writeSSEEvent(w, string(b))
	}
	writeSSEDone(w)
	return nil
}

// resolvePMAEndpoint returns the PMA base URL for chat routing.
// Only worker-reported endpoints from capability snapshots (managed_services_status) are used; no other path is allowed.
func (h *OpenAIChatHandler) resolvePMAEndpoint(ctx context.Context) string {
	return h.resolvePMAEndpointCandidate(ctx).endpoint
}

func (h *OpenAIChatHandler) resolvePMAEndpointCandidate(ctx context.Context) pmaEndpointCandidate {
	if h.db == nil {
		return pmaEndpointCandidate{}
	}
	candidates := h.collectReadyPMACandidates(ctx)
	if len(candidates) == 0 {
		return pmaEndpointCandidate{}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].readyAt.Equal(candidates[j].readyAt) {
			return candidates[i].endpoint < candidates[j].endpoint
		}
		return candidates[i].readyAt.After(candidates[j].readyAt)
	})
	return candidates[0]
}

type pmaEndpointCandidate struct {
	endpoint             string
	readyAt              time.Time
	workerAPIBearerToken string
}

func (h *OpenAIChatHandler) collectReadyPMACandidates(ctx context.Context) []pmaEndpointCandidate {
	nodes, err := h.db.ListActiveNodes(ctx)
	if err != nil || len(nodes) == 0 {
		return nil
	}
	candidates := make([]pmaEndpointCandidate, 0)
	for _, node := range nodes {
		snap, snapErr := h.db.GetLatestNodeCapabilitySnapshot(ctx, node.ID)
		if snapErr != nil || strings.TrimSpace(snap) == "" {
			continue
		}
		nodeToken := ""
		if node.WorkerAPIBearerToken != nil {
			nodeToken = strings.TrimSpace(*node.WorkerAPIBearerToken)
		}
		candidates = append(candidates, readyPMACandidatesFromSnapshot(snap, nodeToken)...)
	}
	return candidates
}

func readyPMACandidatesFromSnapshot(snapshot, workerAPIBearerToken string) []pmaEndpointCandidate {
	var report nodepayloads.CapabilityReport
	if json.Unmarshal([]byte(snapshot), &report) != nil || report.ManagedServicesStatus == nil {
		return nil
	}
	candidates := make([]pmaEndpointCandidate, 0)
	for i := range report.ManagedServicesStatus.Services {
		svc := &report.ManagedServicesStatus.Services[i]
		if svc.ServiceType != managedServiceTypePMA || svc.State != "ready" || len(svc.Endpoints) == 0 {
			continue
		}
		readyAt := time.Time{}
		if t, parseErr := time.Parse(time.RFC3339, svc.ReadyAt); parseErr == nil {
			readyAt = t.UTC()
		}
		candidates = append(candidates, pmaEndpointCandidate{
			endpoint:             svc.Endpoints[0],
			readyAt:              readyAt,
			workerAPIBearerToken: workerAPIBearerToken,
		})
	}
	return candidates
}

func (h *OpenAIChatHandler) completeViaDirectInference(ctx context.Context, effectiveModel, lastUserContent string) (content string, status int, code, msg string) {
	if h.inferenceURL == "" {
		return "", http.StatusBadRequest, "invalid_request", "Direct inference not configured for this model"
	}
	modelID := effectiveModel
	if modelID != h.inferenceModel {
		modelID = h.inferenceModel
	}
	call := func() (string, error) {
		return inference.CallGenerate(ctx, nil, h.inferenceURL, modelID, lastUserContent)
	}
	return h.runCompletionWithRetry(ctx, effectiveModel, "direct_inference", "direct inference failed", call)
}

// runCompletionWithRetry runs the given call with exponential backoff; returns (content, 0, "", "") on success.
func (h *OpenAIChatHandler) runCompletionWithRetry(ctx context.Context, effectiveModel, pathLabel, failLogMsg string, call func() (string, error)) (content string, status int, code, msg string) {
	var err error
	for attempt := 0; attempt < chatCompletionMaxRetries; attempt++ {
		content, err = call()
		if err == nil {
			h.logger.Info("chat completion path", "path", pathLabel, "model", effectiveModel)
			return content, 0, "", ""
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return "", http.StatusGatewayTimeout, "cynodeai_completion_timeout", "Completion did not finish before the maximum wait duration"
		}
		if !isTransientInferenceError(err) || attempt == chatCompletionMaxRetries-1 {
			h.logger.Error(failLogMsg, "error", err)
			return "", http.StatusBadGateway, inferenceFailedCode, completionFailedMsg
		}
		backoff := chatCompletionBackoffBase * time.Duration(1<<uint(attempt))
		time.Sleep(backoff)
	}
	h.logger.Error(failLogMsg+" after retries", "error", err)
	return "", http.StatusBadGateway, inferenceFailedCode, completionFailedMsg
}

func buildChatCompletionsResponse(model, content string) userapi.ChatCompletionsResponse {
	return userapi.ChatCompletionsResponse{
		ID:      uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []userapi.ChatCompletionsChoice{
			{
				Index: 0,
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Role: chatRoleAssistant, Content: content},
				FinishReason: "stop",
			},
		},
	}
}

func projectIDFromHeader(r *http.Request) *uuid.UUID {
	v := r.Header.Get("OpenAI-Project")
	if v == "" {
		return nil
	}
	id, err := uuid.Parse(strings.TrimSpace(v))
	if err != nil {
		return nil
	}
	return &id
}

func redactSecrets(messages []userapi.ChatMessage) (amended []userapi.ChatMessage, kinds []string) {
	amended = make([]userapi.ChatMessage, 0, len(messages))
	seen := make(map[string]bool)
	for _, m := range messages {
		content := m.Content
		if skPrefixPattern.MatchString(content) {
			content = skPrefixPattern.ReplaceAllString(content, secretRedacted)
			if !seen["api_key"] {
				kinds = append(kinds, "api_key")
				seen["api_key"] = true
			}
		}
		if apiKeyLikePattern.MatchString(content) {
			content = apiKeyLikePattern.ReplaceAllString(content, "${1}: "+secretRedacted)
			if !seen["api_key"] {
				kinds = append(kinds, "api_key")
				seen["api_key"] = true
			}
		}
		amended = append(amended, userapi.ChatMessage{Role: m.Role, Content: content})
	}
	return amended, kinds
}

func kindsJSON(kinds []string) *string {
	if len(kinds) == 0 {
		return nil
	}
	b, _ := json.Marshal(kinds)
	s := string(b)
	return &s
}

func writeOpenAIJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeOpenAIError(w http.ResponseWriter, status int, code, message string) {
	writeOpenAIJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    "cynodeai_error",
			"param":   nil,
			"code":    code,
		},
	})
}

// writeSSEEvent writes one Server-Sent Event line to w and flushes if possible.
// Per CYNAI.USRGWY.OpenAIChatApi.Streaming: events must be flushed promptly.
func writeSSEEvent(w http.ResponseWriter, data string) {
	bw := bufio.NewWriter(w)
	_, _ = fmt.Fprintf(bw, "data: %s\n\n", data)
	_ = bw.Flush()
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// writeSSEDone writes the terminal [DONE] event per OpenAI SSE protocol.
func writeSSEDone(w http.ResponseWriter) {
	writeSSEEvent(w, "[DONE]")
}

// writeSSEError writes a terminal error event as a JSON data line then [DONE].
// Clients that support structured error events will see the code; others see [DONE].
func writeSSEError(w http.ResponseWriter, code, message string) {
	errData, _ := json.Marshal(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    "cynodeai_error",
			"code":    code,
		},
	})
	writeSSEEvent(w, string(errData))
	writeSSEDone(w)
}

// prepareSSEResponse sets the response headers for a Server-Sent Events stream
// and writes the 200 status code. Must be called before any SSE events are written
// and before any JSON error is written.
func prepareSSEResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
}

// buildChatCompletionChunk builds a streaming chunk SSE payload for one delta.
// finishReason is nil for intermediate chunks and "stop" for the final chunk.
func buildChatCompletionChunk(id, model, delta string, finishReason *string) userapi.ChatCompletionChunk {
	return userapi.ChatCompletionChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []userapi.ChatCompletionChunkChoice{
			{
				Index: 0,
				Delta: userapi.ChatCompletionChunkDelta{
					Role:    "assistant",
					Content: delta,
				},
				FinishReason: finishReason,
			},
		},
	}
}

// sseChunkSize is the approximate rune count per content delta in degraded-mode streaming
// so the TUI receives incremental updates instead of one large event.
const sseChunkSize = 48

// emitContentAsSSE emits content as SSE events: an opening role chunk, content delta chunks
// (split so the client sees incremental updates), and a final stop chunk, then [DONE].
// Degraded mode when upstream cannot provide token-by-token deltas (CYNAI.USRGWY.OpenAIChatApi.Streaming).
func emitContentAsSSE(w http.ResponseWriter, id, model, content string) {
	stop := "stop"
	// Opening chunk: role only, no content.
	open := userapi.ChatCompletionChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []userapi.ChatCompletionChunkChoice{
			{Index: 0, Delta: userapi.ChatCompletionChunkDelta{Role: "assistant"}, FinishReason: nil},
		},
	}
	if b, err := json.Marshal(open); err == nil {
		writeSSEEvent(w, string(b))
	}
	// Emit content in small chunks so the TUI can show incremental streaming.
	runes := []rune(content)
	for i := 0; i < len(runes); {
		end := i + sseChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		delta := string(runes[i:end])
		i = end
		chunk := buildChatCompletionChunk(id, model, delta, nil)
		if b, err := json.Marshal(chunk); err == nil {
			writeSSEEvent(w, string(b))
		}
	}
	// Final stop chunk.
	final := buildChatCompletionChunk(id, model, "", &stop)
	if b, err := json.Marshal(final); err == nil {
		writeSSEEvent(w, string(b))
	}
	writeSSEDone(w)
}

// trimHistoryToCharBudget returns the most recent messages from history whose cumulative
// character count fits within budget. At least the last message is always included
// regardless of budget so the current user turn is never dropped.
func trimHistoryToCharBudget(history []*models.ChatMessage, budget int) []userapi.ChatMessage {
	if len(history) == 0 {
		return nil
	}
	// Always include the last message; walk backwards including more until budget exceeded.
	total := len(history[len(history)-1].Content)
	start := len(history) - 1
	for start > 0 {
		prev := start - 1
		if total+len(history[prev].Content) > budget {
			break
		}
		total += len(history[prev].Content)
		start = prev
	}
	out := make([]userapi.ChatMessage, 0, len(history)-start)
	for _, m := range history[start:] {
		out = append(out, userapi.ChatMessage{Role: m.Role, Content: m.Content})
	}
	return out
}

// NewThread handles POST /v1/chat/threads — unconditionally creates a new conversation thread
// for the authenticated user and returns its ID. Request body may include project_id and title (optional).
func (h *OpenAIChatHandler) NewThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		writeOpenAIError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	projectID := projectIDFromHeader(r)
	var body struct {
		ProjectID *string `json:"project_id,omitempty"`
		Title     *string `json:"title,omitempty"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ProjectID != nil && strings.TrimSpace(*body.ProjectID) != "" {
			if id, err := uuid.Parse(strings.TrimSpace(*body.ProjectID)); err == nil {
				projectID = &id
			}
		}
	}
	thread, err := h.db.CreateChatThread(ctx, *userID, projectID, body.Title)
	if err != nil {
		h.logger.Error("create chat thread", "error", err)
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to create thread")
		return
	}
	writeOpenAIJSON(w, http.StatusCreated, map[string]string{
		"thread_id": thread.ID.String(),
	})
}

// Responses handles POST /v1/responses (OpenAI Responses API). Same pipeline as chat/completions:
// decode input, resolve thread (from previous_response_id or active), redact, persist user, route, persist assistant with response_id in metadata, return responses-format.
//
//nolint:gocognit,gocyclo // single HTTP handler with sequential validation and branching for stream vs non-stream
func (h *OpenAIChatHandler) Responses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		writeOpenAIError(w, http.StatusUnauthorized, "authentication_required", "Authentication required")
		return
	}
	req, errCode, errMsg := h.decodeAndValidateResponsesRequest(r)
	if errCode != 0 {
		writeOpenAIError(w, errCode, "invalid_request", errMsg)
		return
	}
	effectiveModel := strings.TrimSpace(req.Model)
	if effectiveModel == "" {
		effectiveModel = EffectiveModelPM
	}
	projectID := projectIDFromHeader(r)
	thread, errCode, errMsg := h.resolveThreadForResponses(ctx, *userID, projectID, req.PreviousResponseID)
	if errCode != 0 {
		code := "invalid_request"
		if errCode == http.StatusInternalServerError {
			code = "internal_error"
		}
		writeOpenAIError(w, errCode, code, errMsg)
		return
	}
	if thread == nil {
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to resolve thread")
		return
	}
	userContent := extractUserContentFromResponsesInput(req.Input)
	if userContent == "" {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "input is required (plain string or message array with at least one user message)")
		return
	}
	redacted, kinds := redactSecrets([]userapi.ChatMessage{{Role: chatRoleUser, Content: userContent}})
	userContent = redacted[0].Content
	if _, err := h.db.AppendChatMessage(ctx, thread.ID, chatRoleUser, userContent, nil); err != nil {
		h.logger.Error("append user message", "error", err)
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to persist message")
		return
	}
	contextMessages := h.responsesContextMessages(ctx, thread.ID, redacted)
	timeoutCtx, cancel := context.WithTimeout(ctx, chatCompletionTimeout)
	defer cancel()
	start := time.Now()
	if req.Stream && effectiveModel == EffectiveModelPM {
		cand := h.resolvePMAEndpointCandidate(timeoutCtx)
		if cand.endpoint != "" {
			responseID := "resp_" + uuid.New().String()
			meta := map[string]string{"response_id": responseID}
			metaJSON, _ := json.Marshal(meta)
			metaStr := string(metaJSON)
			prepareSSEResponse(w)
			err := h.completeViaPMAStream(timeoutCtx, w, cand, contextMessages, thread.ID, userID, projectID, start, effectiveModel, responseID, &metaStr)
			if err != nil {
				writeSSEError(w, "stream_error", err.Error())
			}
			return
		}
	}
	content, status, code, msg := h.routeAndComplete(timeoutCtx, effectiveModel, contextMessages, userContent)
	if status != 0 {
		if req.Stream {
			prepareSSEResponse(w)
			writeSSEError(w, code, msg)
		} else {
			writeOpenAIError(w, status, code, msg)
		}
		return
	}
	responseID := "resp_" + uuid.New().String()
	meta := map[string]string{"response_id": responseID}
	metaJSON, _ := json.Marshal(meta)
	metaStr := string(metaJSON)
	if _, err := h.db.AppendChatMessage(ctx, thread.ID, chatRoleAssistant, content, &metaStr); err != nil {
		h.logger.Error("append assistant message", "error", err)
	}
	durationMs := int(time.Since(start).Milliseconds())
	_ = h.db.CreateChatAuditLog(ctx, &models.ChatAuditLog{
		UserID:           userID,
		ProjectID:        projectID,
		Outcome:          "success",
		RedactionApplied: len(kinds) > 0,
		RedactionKinds:   kindsJSON(kinds),
		DurationMs:       &durationMs,
	})
	if req.Stream {
		// Degraded-mode SSE stream for /v1/responses when upstream is blocking.
		prepareSSEResponse(w)
		emitContentAsSSE(w, responseID, effectiveModel, content)
		return
	}
	writeOpenAIJSON(w, http.StatusOK, userapi.ResponsesCreateResponse{
		ID:      responseID,
		Object:  "response",
		Created: time.Now().Unix(),
		Output:  []userapi.ResponsesOutputText{{Type: "output_text", Text: content}},
	})
}

func (h *OpenAIChatHandler) resolveThreadForResponses(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, previousResponseID string) (thread *models.ChatThread, status int, errMsg string) {
	if strings.TrimSpace(previousResponseID) != "" {
		thread, err := h.db.GetThreadByResponseID(ctx, strings.TrimSpace(previousResponseID), userID)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return nil, http.StatusBadRequest, "previous_response_id not found or not owned by you"
			}
			h.logger.Error("get thread by response id", "error", err)
			return nil, http.StatusInternalServerError, "Failed to resolve continuation"
		}
		return thread, 0, ""
	}
	thread, err := h.db.GetOrCreateActiveChatThread(ctx, userID, projectID)
	if err != nil {
		h.logger.Error("get or create chat thread", "error", err)
		return nil, http.StatusInternalServerError, "Failed to get chat thread"
	}
	return thread, 0, ""
}

func (h *OpenAIChatHandler) responsesContextMessages(ctx context.Context, threadID uuid.UUID, redacted []userapi.ChatMessage) []userapi.ChatMessage {
	threadHistory, err := h.db.ListChatMessages(ctx, threadID, chatHistoryLimit)
	if err != nil {
		h.logger.Warn("failed to load chat history", "error", err)
		return redacted
	}
	if len(threadHistory) == 0 {
		return redacted
	}
	return trimHistoryToCharBudget(threadHistory, chatHistoryCharBudget)
}

func extractUserContentFromResponsesInput(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	if s := parseResponsesInputAsString(input); s != "" {
		return s
	}
	return parseResponsesInputAsMessageArray(input)
}

func parseResponsesInputAsString(input json.RawMessage) string {
	var s string
	if err := json.Unmarshal(input, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func parseResponsesInputAsMessageArray(input json.RawMessage) string {
	var arr []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
		Text    string          `json:"text"`
	}
	if err := json.Unmarshal(input, &arr); err != nil || len(arr) == 0 {
		return ""
	}
	for i := len(arr) - 1; i >= 0; i-- {
		if arr[i].Role != chatRoleUser {
			continue
		}
		text := arr[i].Text
		if text == "" && len(arr[i].Content) > 0 {
			text = extractTextFromMessageContent(arr[i].Content)
		}
		if text != "" {
			return strings.TrimSpace(text)
		}
		break
	}
	return ""
}

func extractTextFromMessageContent(content json.RawMessage) string {
	var contentArr []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &contentArr) == nil && len(contentArr) > 0 {
		return contentArr[0].Text
	}
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s
	}
	return ""
}

func (h *OpenAIChatHandler) decodeAndValidateResponsesRequest(r *http.Request) (req userapi.ResponsesCreateRequest, status int, errMsg string) {
	if status, errMsg = decodeOpenAIRequest(r, &req); status != 0 {
		return req, status, errMsg
	}
	if responsesInputEmpty(&req) {
		return req, http.StatusBadRequest, "input is required"
	}
	return req, 0, ""
}

func responsesInputEmpty(req *userapi.ResponsesCreateRequest) bool {
	return len(req.Input) == 0
}

// ListThreads handles GET /v1/chat/threads (list threads for user, recent-first, pagination).
func (h *OpenAIChatHandler) ListThreads(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		writeOpenAIError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	projectID := projectIDFromHeader(r)
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}
	threads, err := h.db.ListChatThreads(ctx, *userID, projectID, limit, offset)
	if err != nil {
		h.logger.Error("list chat threads", "error", err)
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to list threads")
		return
	}
	type threadItem struct {
		ID        string  `json:"id"`
		Title     *string `json:"title,omitempty"`
		CreatedAt string  `json:"created_at"`
		UpdatedAt string  `json:"updated_at"`
	}
	items := make([]threadItem, 0, len(threads))
	for _, t := range threads {
		items = append(items, threadItem{
			ID:        t.ID.String(),
			Title:     t.Title,
			CreatedAt: t.CreatedAt.Format(time.RFC3339),
			UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
		})
	}
	writeOpenAIJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// GetThread handles GET /v1/chat/threads/{id}.
func (h *OpenAIChatHandler) GetThread(w http.ResponseWriter, r *http.Request, threadID string) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		writeOpenAIError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	id, err := uuid.Parse(threadID)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "invalid thread id")
		return
	}
	thread, err := h.db.GetChatThreadByID(ctx, id, *userID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeOpenAIError(w, http.StatusNotFound, "not_found", "Thread not found")
			return
		}
		h.logger.Error("get chat thread", "error", err)
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to get thread")
		return
	}
	writeOpenAIJSON(w, http.StatusOK, map[string]interface{}{
		"id":         thread.ID.String(),
		"title":      thread.Title,
		"created_at": thread.CreatedAt.Format(time.RFC3339),
		"updated_at": thread.UpdatedAt.Format(time.RFC3339),
	})
}

// ListThreadMessages handles GET /v1/chat/threads/{id}/messages.
func (h *OpenAIChatHandler) ListThreadMessages(w http.ResponseWriter, r *http.Request, threadID string) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		writeOpenAIError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	id, err := uuid.Parse(threadID)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "invalid thread id")
		return
	}
	_, err = h.db.GetChatThreadByID(ctx, id, *userID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeOpenAIError(w, http.StatusNotFound, "not_found", "Thread not found")
			return
		}
		h.logger.Error("get chat thread", "error", err)
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to get thread")
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, parseErr := strconv.Atoi(l); parseErr == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	msgs, err := h.db.ListChatMessages(ctx, id, limit)
	if err != nil {
		h.logger.Error("list chat messages", "error", err)
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to list messages")
		return
	}
	items := make([]map[string]interface{}, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, map[string]interface{}{
			"id":         m.ID.String(),
			"role":       m.Role,
			"content":    m.Content,
			"metadata":   m.Metadata,
			"created_at": m.CreatedAt.Format(time.RFC3339),
		})
	}
	writeOpenAIJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// PatchThreadTitle handles PATCH /v1/chat/threads/{id} (rename: update title).
func (h *OpenAIChatHandler) PatchThreadTitle(w http.ResponseWriter, r *http.Request, threadID string) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		writeOpenAIError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	id, err := uuid.Parse(threadID)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "invalid thread id")
		return
	}
	var body struct {
		Title *string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "title is required in body")
		return
	}
	if err := h.db.UpdateChatThreadTitle(ctx, id, *userID, *body.Title); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeOpenAIError(w, http.StatusNotFound, "not_found", "Thread not found")
			return
		}
		h.logger.Error("update chat thread title", "error", err)
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to update thread")
		return
	}
	writeOpenAIJSON(w, http.StatusOK, map[string]string{"id": threadID})
}
