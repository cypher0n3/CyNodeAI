// Package handlers implements OpenAI-compatible chat API per docs/tech_specs/openai_compatible_chat_api.md.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
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
	if _, err := h.db.AppendChatMessage(ctx, thread.ID, "user", lastUserContent, nil); err != nil {
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
	content, status, code, msg := h.routeAndComplete(timeoutCtx, effectiveModel, contextMessages, lastUserContent)
	if status != 0 {
		writeOpenAIError(w, status, code, msg)
		return
	}
	if _, err := h.db.AppendChatMessage(ctx, thread.ID, "assistant", content, nil); err != nil {
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
	writeOpenAIJSON(w, http.StatusOK, buildChatCompletionsResponse(effectiveModel, content)) //nolint:exhaustruct // response struct built inline; exhaustruct wants all fields set
}

func (h *OpenAIChatHandler) decodeAndValidateChatRequest(r *http.Request) (req userapi.ChatCompletionsRequest, status int, errMsg string) {
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, http.StatusBadRequest, "Invalid request body"
	}
	if len(req.Messages) == 0 {
		return req, http.StatusBadRequest, "messages is required and must be non-empty"
	}
	return req, 0, ""
}

func lastUserMessageContent(redacted []userapi.ChatMessage) string {
	for i := len(redacted) - 1; i >= 0; i-- {
		if redacted[i].Role == "user" {
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
				}{Role: "assistant", Content: content},
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
// for the authenticated user and returns its ID. Clients use this when the user explicitly
// requests a fresh context (e.g. /thread new or --thread-new flag).
func (h *OpenAIChatHandler) NewThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := getUserIDFromContext(ctx)
	if userID == nil {
		writeOpenAIError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	projectID := projectIDFromHeader(r)
	thread, err := h.db.CreateChatThread(ctx, *userID, projectID)
	if err != nil {
		h.logger.Error("create chat thread", "error", err)
		writeOpenAIError(w, http.StatusInternalServerError, "internal_error", "Failed to create thread")
		return
	}
	writeOpenAIJSON(w, http.StatusCreated, map[string]string{
		"thread_id": thread.ID.String(),
	})
}
