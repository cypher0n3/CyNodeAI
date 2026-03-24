// Package handlers implements OpenAI-compatible chat API per docs/tech_specs/openai_compatible_chat_api.md.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

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
		errCode2 := "invalid_request"
		if errCode == http.StatusInternalServerError {
			errCode2 = "internal_error"
		}
		writeOpenAIError(w, errCode, errCode2, errMsg)
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
	contextMessages := h.buildChatContextMessages(ctx, thread.ID, redacted)
	timeoutCtx, cancel := context.WithTimeout(ctx, chatCompletionTimeout)
	defer cancel()
	start := time.Now()
	responseID := "resp_" + uuid.New().String()
	responsesMeta := func() *string {
		meta := map[string]string{"response_id": responseID}
		b, _ := json.Marshal(meta)
		s := string(b)
		return &s
	}()
	if h.tryPMAStream(timeoutCtx, w, req.Stream, effectiveModel, contextMessages, thread.ID, userID, projectID, start, responseID, responsesMeta) {
		return
	}
	content, status, code, msg := h.routeAndComplete(timeoutCtx, effectiveModel, contextMessages, userContent)
	if status != 0 {
		writeCompletionError(w, req.Stream, status, code, msg)
		return
	}
	metaStr := *responsesMeta
	if _, err := h.db.AppendChatMessage(ctx, thread.ID, chatRoleAssistant, content, &metaStr); err != nil {
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
