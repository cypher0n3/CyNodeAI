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
const chatCompletionTimeout = 90 * time.Second

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
	db             database.Store
	logger         *slog.Logger
	inferenceURL   string
	inferenceModel string
}

// NewOpenAIChatHandler creates a handler for the OpenAI-compatible chat surface.
// PMA routing is only via worker-reported endpoints (capability managed_services_status); no env fallback.
func NewOpenAIChatHandler(db database.Store, logger *slog.Logger, inferenceURL, inferenceModel string) *OpenAIChatHandler {
	if inferenceModel == "" {
		inferenceModel = "tinyllama"
	}
	return &OpenAIChatHandler{
		db:             db,
		logger:         logger,
		inferenceURL:   inferenceURL,
		inferenceModel: inferenceModel,
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
	// REQ-ORCHES-0131: enforce maximum total wait duration.
	timeoutCtx, cancel := context.WithTimeout(ctx, chatCompletionTimeout)
	defer cancel()
	start := time.Now()
	content, status, code, msg := h.routeAndComplete(timeoutCtx, effectiveModel, redacted, lastUserContent)
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
	pmaEndpoint := h.resolvePMAEndpoint(ctx)
	if pmaEndpoint == "" {
		h.logger.Warn("PMA base URL not configured; cannot route to cynodeai.pm")
		return "", http.StatusServiceUnavailable, "model_unavailable", "PM agent is not available"
	}
	msgs := make([]pmaclient.ChatMessage, 0, len(redacted))
	for _, m := range redacted {
		msgs = append(msgs, pmaclient.ChatMessage{Role: m.Role, Content: m.Content})
	}
	var err error
	for attempt := 0; attempt < chatCompletionMaxRetries; attempt++ {
		content, err = pmaclient.CallChatCompletion(ctx, nil, pmaEndpoint, msgs)
		if err == nil {
			h.logger.Info("chat completion path", "path", "pma", "model", effectiveModel)
			return content, 0, "", ""
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return "", http.StatusGatewayTimeout, "cynodeai_completion_timeout", "Completion did not finish before the maximum wait duration"
		}
		if !isTransientInferenceError(err) || attempt == chatCompletionMaxRetries-1 {
			h.logger.Error("PMA chat completion failed", "error", err)
			return "", http.StatusBadGateway, inferenceFailedCode, completionFailedMsg
		}
		backoff := chatCompletionBackoffBase * time.Duration(1<<uint(attempt))
		time.Sleep(backoff)
	}
	h.logger.Error("PMA chat completion failed after retries", "error", err)
	return "", http.StatusBadGateway, inferenceFailedCode, completionFailedMsg
}

// resolvePMAEndpoint returns the PMA base URL for chat routing.
// Only worker-reported endpoints from capability snapshots (managed_services_status) are used; no other path is allowed.
func (h *OpenAIChatHandler) resolvePMAEndpoint(ctx context.Context) string {
	if h.db == nil {
		return ""
	}
	candidates := h.collectReadyPMACandidates(ctx)
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].readyAt.Equal(candidates[j].readyAt) {
			return candidates[i].endpoint < candidates[j].endpoint
		}
		return candidates[i].readyAt.After(candidates[j].readyAt)
	})
	return candidates[0].endpoint
}

type pmaEndpointCandidate struct {
	endpoint string
	readyAt  time.Time
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
		candidates = append(candidates, readyPMACandidatesFromSnapshot(snap)...)
	}
	return candidates
}

func readyPMACandidatesFromSnapshot(snapshot string) []pmaEndpointCandidate {
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
			endpoint: svc.Endpoints[0],
			readyAt:  readyAt,
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
	var err error
	for attempt := 0; attempt < chatCompletionMaxRetries; attempt++ {
		content, err = inference.CallGenerate(ctx, nil, h.inferenceURL, modelID, lastUserContent)
		if err == nil {
			h.logger.Info("chat completion path", "path", "direct_inference", "model", effectiveModel)
			return content, 0, "", ""
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return "", http.StatusGatewayTimeout, "cynodeai_completion_timeout", "Completion did not finish before the maximum wait duration"
		}
		if !isTransientInferenceError(err) || attempt == chatCompletionMaxRetries-1 {
			h.logger.Error("direct inference failed", "error", err)
			return "", http.StatusBadGateway, inferenceFailedCode, completionFailedMsg
		}
		backoff := chatCompletionBackoffBase * time.Duration(1<<uint(attempt))
		time.Sleep(backoff)
	}
	h.logger.Error("direct inference failed after retries", "error", err)
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
