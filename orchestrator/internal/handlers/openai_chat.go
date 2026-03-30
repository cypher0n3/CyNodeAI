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
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/secretutil"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/inference"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/pmaclient"
)

// Max total wait for chat completion (REQ-ORCHES-0131).
// chatCompletionTimeout is the maximum wall time for a single chat completion request.
// Must be at or below the user-gateway WRITE_TIMEOUT (300 s) and aligned with pmaclient defaultPMAHTTPTimeout.
const chatCompletionTimeout = 300 * time.Second

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

// statusClientClosedRequest is HTTP 499 (client closed request), used when the request context is canceled during completion.
const statusClientClosedRequest = 499

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

// writeCompletionError writes either an SSE error or a JSON error depending on whether streaming was requested.
func writeCompletionError(w http.ResponseWriter, stream bool, status int, code, msg string) {
	if stream {
		prepareSSEResponse(w)
		writeSSEError(w, code, msg)
		return
	}
	writeOpenAIError(w, status, code, msg)
}

// tryPMAStream attempts true token-by-token streaming via PMA when stream=true and model==PM.
// responseID and assistantMeta are caller-supplied (chat uses a plain UUID, responses adds metadata).
// Returns true if the response was fully handled (caller must return).
func (h *OpenAIChatHandler) tryPMAStream(ctx context.Context, w http.ResponseWriter, stream bool, effectiveModel string, contextMessages []userapi.ChatMessage, threadID uuid.UUID, userID, projectID *uuid.UUID, start time.Time, responseID string, assistantMeta *string) bool {
	if !stream || effectiveModel != EffectiveModelPM {
		return false
	}
	cand := h.resolvePMAEndpointCandidate(ctx)
	if cand.endpoint == "" {
		return false
	}
	prepareSSEResponse(w)
	if err := h.completeViaPMAStream(ctx, w, cand, contextMessages, threadID, userID, projectID, start, effectiveModel, responseID, assistantMeta); err != nil {
		writeSSEError(w, "stream_error", err.Error())
	}
	return true
}

// ChatCompletions handles POST /v1/chat/completions with pipeline: auth (already done), decode, project_id, redact, persist user message, route, persist assistant, return.
// When stream=true is requested the response uses Server-Sent Events per CYNAI.USRGWY.OpenAIChatApi.Streaming.
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
	contextMessages := h.buildChatContextMessages(ctx, thread.ID, redacted)
	// REQ-ORCHES-0131: enforce maximum total wait duration.
	timeoutCtx, cancel := context.WithTimeout(ctx, chatCompletionTimeout)
	defer cancel()
	start := time.Now()
	if h.tryPMAStream(timeoutCtx, w, req.Stream, effectiveModel, contextMessages, thread.ID, userID, projectID, start, uuid.New().String(), nil) {
		return
	}
	content, status, code, msg := h.routeAndComplete(timeoutCtx, effectiveModel, contextMessages, lastUserContent)
	if status != 0 {
		writeCompletionError(w, req.Stream, status, code, msg)
		return
	}
	if _, err := h.db.AppendChatMessage(ctx, thread.ID, chatRoleAssistant, content, nil); err != nil {
		h.logger.Error("append assistant message", "error", err)
	}
	durationMs := int(time.Since(start).Milliseconds())
	_ = h.db.CreateChatAuditLog(ctx, &models.ChatAuditLog{
		ChatAuditLogBase: models.ChatAuditLogBase{
			UserID:           userID,
			ProjectID:        projectID,
			Outcome:          "success",
			RedactionApplied: len(kinds) > 0,
			RedactionKinds:   kindsJSON(kinds),
			DurationMs:       &durationMs,
		},
	})
	if req.Stream {
		completionID := uuid.New().String()
		prepareSSEResponse(w)
		emitDegradedStreamingFallback(timeoutCtx, w, completionID, effectiveModel, content, false)
		return
	}
	writeOpenAIJSON(w, http.StatusOK, buildChatCompletionsResponse(effectiveModel, content))
}

// buildChatContextMessages loads thread history and builds the context message slice for routing.
// Falls back to fallback messages if history is unavailable or empty.
func (h *OpenAIChatHandler) buildChatContextMessages(ctx context.Context, threadID uuid.UUID, fallback []userapi.ChatMessage) []userapi.ChatMessage {
	history, err := h.db.ListChatMessages(ctx, threadID, chatHistoryLimit)
	if err != nil || len(history) == 0 {
		if err != nil {
			h.logger.Warn("failed to load chat history; using request messages", "error", err)
		}
		return fallback
	}
	return trimHistoryToCharBudget(history, chatHistoryCharBudget)
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

// pmaStreamToolRec records one streamed tool call for persistence metadata.
type pmaStreamToolRec struct {
	name string
	args string
}

// pmaRelayState holds PMA stream accumulation and SSE emission for one streaming completion.
type pmaRelayState struct {
	w              http.ResponseWriter
	visibleB       strings.Builder
	thinkingB      strings.Builder
	toolRecs       []pmaStreamToolRec
	responsesMode  bool
	chunkID        string
	effectiveModel string
}

func (s *pmaRelayState) emitVisibleChunkToSSE(delta string) error {
	if s.responsesMode {
		p, _ := json.Marshal(map[string]string{"delta": delta})
		writeSSENamedEvent(s.w, userapi.SSEEventResponseOutputTextDelta, string(p))
		return nil
	}
	chunk := buildChatCompletionChunk(s.chunkID, s.effectiveModel, delta, nil)
	b, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	writeSSEEvent(s.w, string(b))
	return nil
}

func (s *pmaRelayState) appendVisibleAndEmit(delta string) error {
	_, _ = s.visibleB.WriteString(delta)
	return s.emitVisibleChunkToSSE(delta)
}

func (s *pmaRelayState) emitIterationStartEvent(iteration int) error {
	payload := userapi.SSEIterationStartPayload{Iteration: iteration}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	writeSSENamedEvent(s.w, userapi.SSEEventIterationStart, string(b))
	return nil
}

func (s *pmaRelayState) emitThinkingAndBuffer(th string) error {
	secretutil.RunWithSecret(func() {
		_, _ = s.thinkingB.WriteString(th)
	})
	payload := userapi.SSEThinkingDeltaPayload{Content: th}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	writeSSENamedEvent(s.w, userapi.SSEEventThinkingDelta, string(b))
	return nil
}

func (s *pmaRelayState) recordToolAndEmit(name, args string) error {
	s.toolRecs = append(s.toolRecs, pmaStreamToolRec{name: name, args: args})
	payload := userapi.SSEToolCallPayload{Name: name, Arguments: args}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	writeSSENamedEvent(s.w, userapi.SSEEventToolCall, string(b))
	return nil
}

func (s *pmaRelayState) relayAmendment(a *pmaclient.PMAAmendment) error {
	if a == nil {
		return nil
	}
	p := userapi.SSEAmendmentPayload{
		Type: a.Type, Content: a.Content, Scope: a.Scope, Iteration: a.Iteration,
		Reason: a.Reason, RedactionKinds: a.RedactionKinds,
	}
	if a.Type == "secret_redaction" {
		p.Redacted = a.Content
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	writeSSENamedEvent(s.w, userapi.SSEEventAmendment, string(b))
	if a.Type == "secret_redaction" {
		s.visibleB.Reset()
		_, _ = s.visibleB.WriteString(a.Content)
	}
	return nil
}

func (s *pmaRelayState) streamCallbacks() pmaclient.PMAStreamCallbacks {
	return pmaclient.PMAStreamCallbacks{
		OnDelta:          s.appendVisibleAndEmit,
		OnThinking:       s.emitThinkingAndBuffer,
		OnIterationStart: s.emitIterationStartEvent,
		OnToolCall:       s.recordToolAndEmit,
		OnAmendment:      s.relayAmendment,
	}
}

func writePMAStreamInitialSSE(w http.ResponseWriter, responsesMode bool, chunkID, effectiveModel string) {
	if responsesMode {
		b, _ := json.Marshal(map[string]string{"response_id": chunkID})
		writeSSEEvent(w, string(b))
	}
	if !responsesMode {
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
	}
}

func writePMAStreamTerminalSSE(w http.ResponseWriter, responsesMode bool, chunkID, effectiveModel string) {
	stop := "stop"
	if responsesMode {
		comp, _ := json.Marshal(map[string]interface{}{
			"type": "response.completed",
			"response": map[string]interface{}{
				"id":     chunkID,
				"object": "response",
				"status": "completed",
			},
		})
		writeSSENamedEvent(w, userapi.SSEEventResponseCompleted, string(comp))
	} else {
		final := buildChatCompletionChunk(chunkID, effectiveModel, "", &stop)
		if b, err := json.Marshal(final); err == nil {
			writeSSEEvent(w, string(b))
		}
	}
	writeSSEDone(w)
}

func persistStreamedAssistantTurn(h *OpenAIChatHandler, ctx context.Context, threadID uuid.UUID, userID, projectID *uuid.UUID, start time.Time, content string, rkVisible []string, meta *string) {
	if _, err := h.db.AppendChatMessage(ctx, threadID, chatRoleAssistant, content, meta); err != nil {
		h.logger.Error("append assistant message (stream)", "error", err)
	}
	durationMs := int(time.Since(start).Milliseconds())
	_ = h.db.CreateChatAuditLog(ctx, &models.ChatAuditLog{
		ChatAuditLogBase: models.ChatAuditLogBase{
			UserID:           userID,
			ProjectID:        projectID,
			Outcome:          "success",
			RedactionApplied: len(rkVisible) > 0,
			RedactionKinds:   kindsJSON(rkVisible),
			DurationMs:       &durationMs,
		},
	})
}

// completeViaPMAStream streams completion from PMA token-by-token, persists visible text and optional structured parts, then sends terminal SSE.
// chunkID is the SSE chunk id (completion id) or response id for /v1/responses. assistantMeta, when non-nil, selects native Responses SSE mapping.
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
	responsesMode := assistantMeta != nil
	relay := &pmaRelayState{
		w: w, responsesMode: responsesMode, chunkID: chunkID, effectiveModel: effectiveModel,
	}
	writePMAStreamInitialSSE(w, responsesMode, chunkID, effectiveModel)
	if err := pmaclient.CallChatCompletionStreamWithCallbacks(ctx, nil, cand.endpoint, msgs, workerToken, relay.streamCallbacks()); err != nil {
		return err
	}
	rawVisible := relay.visibleB.String()
	rawThinking := relay.thinkingB.String()
	redactedVisible, rkVisible := redactStringContent(rawVisible)
	redactedThinking, _ := redactStringContent(rawThinking)
	redactedTools := make([]pmaStreamToolRec, len(relay.toolRecs))
	for i, tc := range relay.toolRecs {
		ra, _ := redactStringContent(tc.args)
		redactedTools[i] = pmaStreamToolRec{name: tc.name, args: ra}
	}
	meta := mergeAssistantStreamMetadata(assistantMeta, redactedVisible, redactedThinking, redactedTools)
	persistStreamedAssistantTurn(h, ctx, threadID, userID, projectID, start, redactedVisible, rkVisible, meta)
	writePMAStreamTerminalSSE(w, responsesMode, chunkID, effectiveModel)
	return nil
}

func redactStringContent(s string) (redacted string, kinds []string) {
	amended, rk := redactSecrets([]userapi.ChatMessage{{Role: chatRoleUser, Content: s}})
	return amended[0].Content, rk
}

// mergeAssistantStreamMetadata builds JSON metadata with response_id (if any) and structured parts when thinking or tool calls exist.
func mergeAssistantStreamMetadata(responseMeta *string, visible, thinking string, tools []pmaStreamToolRec) *string {
	if thinking == "" && len(tools) == 0 {
		return responseMeta
	}
	base := map[string]interface{}{}
	if responseMeta != nil && strings.TrimSpace(*responseMeta) != "" {
		_ = json.Unmarshal([]byte(*responseMeta), &base)
	}
	var parts []map[string]interface{}
	if visible != "" {
		parts = append(parts, map[string]interface{}{"type": "text", "text": visible})
	}
	if thinking != "" {
		parts = append(parts, map[string]interface{}{"type": "thinking", "text": thinking})
	}
	for _, tc := range tools {
		parts = append(parts, map[string]interface{}{
			"type": "tool_call", "name": tc.name, "arguments": tc.args,
		})
	}
	base["parts"] = parts
	b, err := json.Marshal(base)
	if err != nil {
		return responseMeta
	}
	s := string(b)
	return &s
}

// emitDegradedStreamingFallback emits heartbeat then a single visible delta per CYNAI.USRGWY.StreamingHeartbeatFallback (no fake chunking).
func emitDegradedStreamingFallback(ctx context.Context, w http.ResponseWriter, chunkID, model, content string, responsesMode bool) {
	_ = ctx
	hb, _ := json.Marshal(userapi.SSEHeartbeatPayload{ElapsedS: 0, Status: "processing"})
	writeSSENamedEvent(w, userapi.SSEEventHeartbeat, string(hb))
	redacted, _ := redactStringContent(content)
	stop := "stop"
	if responsesMode {
		d, _ := json.Marshal(map[string]string{"delta": redacted})
		writeSSENamedEvent(w, userapi.SSEEventResponseOutputTextDelta, string(d))
		comp, _ := json.Marshal(map[string]interface{}{
			"type": "response.completed",
			"response": map[string]interface{}{
				"id":     chunkID,
				"object": "response",
				"status": "completed",
			},
		})
		writeSSENamedEvent(w, userapi.SSEEventResponseCompleted, string(comp))
		writeSSEDone(w)
		return
	}
	open := userapi.ChatCompletionChunk{
		ID:      chunkID,
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
	chunk := buildChatCompletionChunk(chunkID, model, redacted, nil)
	if b, err := json.Marshal(chunk); err == nil {
		writeSSEEvent(w, string(b))
	}
	final := buildChatCompletionChunk(chunkID, model, "", &stop)
	if b, err := json.Marshal(final); err == nil {
		writeSSEEvent(w, string(b))
	}
	writeSSEDone(w)
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
		select {
		case <-ctx.Done():
			return "", statusClientClosedRequest, "request_canceled", ctx.Err().Error()
		case <-time.After(backoff):
		}
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

// writeSSENamedEvent writes event name and data line (cynodeai.* extension events) and flushes.
func writeSSENamedEvent(w http.ResponseWriter, eventName, data string) {
	bw := bufio.NewWriter(w)
	_, _ = fmt.Fprintf(bw, "event: %s\ndata: %s\n\n", eventName, data)
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
