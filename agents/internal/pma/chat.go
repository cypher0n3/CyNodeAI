// Package pma provides internal chat completion for orchestrator handoff.
// See docs/tech_specs/cynode_pma.md (request source and handoff).
package pma

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

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
		systemContext := buildSystemContext(instructionsContent, &req)
		content, err := callInference(r.Context(), systemContext, req.Messages, logger)
		if err != nil {
			logger.Error("chat completion inference error", "error", err)
			writeJSON(w, http.StatusInternalServerError, InternalChatCompletionResponse{Content: ""})
			return
		}
		writeJSON(w, http.StatusOK, InternalChatCompletionResponse{Content: content})
	}
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

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func callInference(ctx context.Context, systemContext string, messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}, logger *slog.Logger) (string, error) {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("INFERENCE_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	model := os.Getenv("INFERENCE_MODEL")
	if model == "" {
		model = "tinyllama"
	}
	var b strings.Builder
	if systemContext != "" {
		b.WriteString(systemContext)
		b.WriteString("\n\n")
	}
	for _, m := range messages {
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	b.WriteString("assistant: ")
	prompt := b.String()

	url := strings.TrimSuffix(baseURL, "/") + "/api/generate"
	body := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("inference returned %s", resp.Status)
	}
	var out struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("inference error: %s", out.Error)
	}
	return out.Response, nil
}
