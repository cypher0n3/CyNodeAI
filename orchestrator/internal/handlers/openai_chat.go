// Package handlers implements OpenAI-compatible chat API per docs/tech_specs/openai_compatible_chat_api.md.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/inference"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/pmaclient"
)

// Effective model default per spec: omitted or empty model MUST behave as cynodeai.pm.
const EffectiveModelPM = "cynodeai.pm"

const secretRedacted = "SECRET_REDACTED"

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
	pmaBaseURL     string
}

// NewOpenAIChatHandler creates a handler for the OpenAI-compatible chat surface.
func NewOpenAIChatHandler(db database.Store, logger *slog.Logger, inferenceURL, inferenceModel, pmaBaseURL string) *OpenAIChatHandler {
	if inferenceModel == "" {
		inferenceModel = "tinyllama"
	}
	return &OpenAIChatHandler{
		db:             db,
		logger:         logger,
		inferenceURL:   inferenceURL,
		inferenceModel: inferenceModel,
		pmaBaseURL:     pmaBaseURL,
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

// ChatCompletionsRequest is the OpenAI chat-completions request body (subset we use).
type ChatCompletionsRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// ChatCompletionsResponse is the OpenAI chat-completions response (subset we use).
type ChatCompletionsResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
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
	start := time.Now()
	content, status, code, msg := h.routeAndComplete(ctx, effectiveModel, redacted, lastUserContent)
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
	writeOpenAIJSON(w, http.StatusOK, buildChatCompletionsResponse(effectiveModel, content))
}

func (h *OpenAIChatHandler) decodeAndValidateChatRequest(r *http.Request) (req ChatCompletionsRequest, status int, errMsg string) {
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, http.StatusBadRequest, "Invalid request body"
	}
	if len(req.Messages) == 0 {
		return req, http.StatusBadRequest, "messages is required and must be non-empty"
	}
	return req, 0, ""
}

func lastUserMessageContent(redacted []struct{ Role, Content string }) string {
	for i := len(redacted) - 1; i >= 0; i-- {
		if redacted[i].Role == "user" {
			return redacted[i].Content
		}
	}
	return ""
}

// routeAndComplete implements Chat Completion Routing Path per openai_compatible_chat_api.md § Chat Completion Routing Path.
// Effective model: request body "model" if present and non-empty (after trim), else cynodeai.pm.
// - effectiveModel == cynodeai.pm → hand off to PM agent (cynode-pma); do not call inference directly.
// - effectiveModel != cynodeai.pm → route to direct inference (Ollama/API Egress); do not invoke PM agent.
func (h *OpenAIChatHandler) routeAndComplete(ctx context.Context, effectiveModel string, redacted []struct{ Role, Content string }, lastUserContent string) (content string, status int, code, msg string) {
	if effectiveModel == EffectiveModelPM {
		if h.pmaBaseURL == "" {
			h.logger.Warn("PMA base URL not configured; cannot route to cynodeai.pm")
			return "", http.StatusServiceUnavailable, "model_unavailable", "PM agent is not available"
		}
		msgs := make([]pmaclient.ChatMessage, 0, len(redacted))
		for _, m := range redacted {
			msgs = append(msgs, pmaclient.ChatMessage{Role: m.Role, Content: m.Content})
		}
		var err error
		content, err = pmaclient.CallChatCompletion(ctx, nil, h.pmaBaseURL, msgs)
		if err != nil {
			h.logger.Error("PMA chat completion failed", "error", err)
			return "", http.StatusBadGateway, "orchestrator_inference_failed", "Completion failed"
		}
		h.logger.Info("chat completion path", "path", "pma", "model", effectiveModel)
		return content, 0, "", ""
	}
	if h.inferenceURL == "" {
		return "", http.StatusBadRequest, "invalid_request", "Direct inference not configured for this model"
	}
	modelID := effectiveModel
	if modelID != h.inferenceModel {
		modelID = h.inferenceModel
	}
	var err error
	content, err = inference.CallGenerate(ctx, nil, h.inferenceURL, modelID, lastUserContent)
	if err != nil {
		h.logger.Error("direct inference failed", "error", err)
		return "", http.StatusBadGateway, "orchestrator_inference_failed", "Completion failed"
	}
	h.logger.Info("chat completion path", "path", "direct_inference", "model", effectiveModel)
	return content, 0, "", ""
}

func buildChatCompletionsResponse(model, content string) ChatCompletionsResponse {
	return ChatCompletionsResponse{
		ID:      uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []struct {
			Index        int    `json:"index"`
			Message      struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{Index: 0, Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{Role: "assistant", Content: content}, FinishReason: "stop"},
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

func redactSecrets(messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}) (amended []struct {
	Role    string
	Content string
}, kinds []string) {
	amended = make([]struct {
		Role    string
		Content string
	}, 0, len(messages))
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
		amended = append(amended, struct {
			Role    string
			Content string
		}{Role: m.Role, Content: content})
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
			"code":   code,
		},
	})
}
