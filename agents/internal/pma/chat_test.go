package pma

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestChatCompletionHandler_MethodNotAllowed(t *testing.T) {
	handler := ChatCompletionHandler("", slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/internal/chat/completion", http.NoBody)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want 405", rec.Code)
	}
}

func TestChatCompletionHandler_BadRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"bad JSON", "{"},
		{"empty messages", `{"messages":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ChatCompletionHandler("", slog.Default())
			req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("got status %d, want 400", rec.Code)
			}
		})
	}
}

func TestChatCompletionHandler_Success(t *testing.T) {
	mockInference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":"hello"}`))
	}))
	defer mockInference.Close()
	oldURL := os.Getenv("OLLAMA_BASE_URL")
	_ = os.Setenv("OLLAMA_BASE_URL", mockInference.URL)
	defer func() {
		if oldURL != "" {
			_ = os.Setenv("OLLAMA_BASE_URL", oldURL)
		} else {
			_ = os.Unsetenv("OLLAMA_BASE_URL")
		}
	}()

	handler := ChatCompletionHandler("sys", slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"content":"hello"`)) {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestChatCompletionHandler_InferenceErrorField(t *testing.T) {
	mockInference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":"","error":"model not found"}`))
	}))
	defer mockInference.Close()
	oldURL := os.Getenv("OLLAMA_BASE_URL")
	_ = os.Setenv("OLLAMA_BASE_URL", mockInference.URL)
	defer func() {
		if oldURL != "" {
			_ = os.Setenv("OLLAMA_BASE_URL", oldURL)
		} else {
			_ = os.Unsetenv("OLLAMA_BASE_URL")
		}
	}()

	handler := ChatCompletionHandler("", slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500", rec.Code)
	}
}

func TestChatCompletionHandler_InferenceError(t *testing.T) {
	mockInference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockInference.Close()
	oldURL := os.Getenv("OLLAMA_BASE_URL")
	_ = os.Setenv("OLLAMA_BASE_URL", mockInference.URL)
	defer func() {
		if oldURL != "" {
			_ = os.Setenv("OLLAMA_BASE_URL", oldURL)
		} else {
			_ = os.Unsetenv("OLLAMA_BASE_URL")
		}
	}()

	handler := ChatCompletionHandler("", slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500", rec.Code)
	}
}

func TestBuildSystemContext(t *testing.T) {
	base := "base"
	if got := buildSystemContext(base, &InternalChatCompletionRequest{}); got != base {
		t.Errorf("buildSystemContext(instructions only) = %q, want base", got)
	}
	req := &InternalChatCompletionRequest{ProjectID: "p1"}
	if got := buildSystemContext(base, req); !strings.Contains(got, "Project context") || !strings.Contains(got, "p1") {
		t.Errorf("buildSystemContext(with project) = %q", got)
	}
	req = &InternalChatCompletionRequest{TaskID: "t1"}
	if got := buildSystemContext(base, req); !strings.Contains(got, "Task context") || !strings.Contains(got, "t1") {
		t.Errorf("buildSystemContext(with task) = %q", got)
	}
	req = &InternalChatCompletionRequest{AdditionalContext: "extra"}
	if got := buildSystemContext(base, req); !strings.Contains(got, "User additional context") || !strings.Contains(got, "extra") {
		t.Errorf("buildSystemContext(with additional) = %q", got)
	}
}

// TestBuildSystemContext_CompositionOrder verifies REQ-PMAGNT-0108 / CYNAI.PMAGNT.LLMContextComposition:
// order is baseline+role -> project -> task -> user additional context.
func TestBuildSystemContext_CompositionOrder(t *testing.T) {
	baseline := "baseline and role instructions"
	req := &InternalChatCompletionRequest{
		ProjectID:         "proj-1",
		TaskID:            "task-1",
		AdditionalContext: "user extra",
	}
	got := buildSystemContext(baseline, req)
	// Must start with baseline.
	if !strings.HasPrefix(got, strings.TrimSpace(baseline)) {
		t.Errorf("context must start with baseline, got prefix %q", got[:min(50, len(got))])
	}
	// Required order: Project before Task before User additional context.
	idxProject := strings.Index(got, "## Project context")
	idxTask := strings.Index(got, "## Task context")
	idxAdditional := strings.Index(got, "## User additional context")
	if idxProject < 0 || idxTask < 0 || idxAdditional < 0 {
		t.Errorf("missing section: project=%d task=%d additional=%d", idxProject, idxTask, idxAdditional)
	}
	if idxProject >= idxTask || idxTask >= idxAdditional {
		t.Errorf("wrong order: project=%d task=%d additional=%d (must be project < task < additional)", idxProject, idxTask, idxAdditional)
	}
	if !strings.Contains(got, "proj-1") || !strings.Contains(got, "task-1") || !strings.Contains(got, "user extra") {
		t.Errorf("context must contain request ids and additional text")
	}
}

func TestChatCompletionHandler_SuccessWithMCPPath(t *testing.T) {
	mcpSrv := newMockMCPServer(t, `{}`)
	defer mcpSrv.Close()

	oldEnv := os.Getenv("PMA_MCP_GATEWAY_URL")
	_ = os.Setenv("PMA_MCP_GATEWAY_URL", mcpSrv.URL)
	defer func() {
		if oldEnv != "" {
			_ = os.Setenv("PMA_MCP_GATEWAY_URL", oldEnv)
		} else {
			_ = os.Unsetenv("PMA_MCP_GATEWAY_URL")
		}
	}()

	oldHook := testLLMForCompletion
	testLLMForCompletion = &mockLLM{responses: []string{"Final Answer: pma response"}}
	defer func() { testLLMForCompletion = oldHook }()

	handler := ChatCompletionHandler("sys", slog.Default())
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", bytes.NewReader([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, body %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"content":"pma response"`)) {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestBuildFullPrompt(t *testing.T) {
	got := buildFullPrompt("sys", []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{{Role: "user", Content: "hi"}})
	if !strings.HasPrefix(got, "sys") {
		t.Errorf("buildFullPrompt should start with system context, got %q", got[:min(20, len(got))])
	}
	if !strings.Contains(got, "user: hi") {
		t.Errorf("buildFullPrompt should contain user message: %q", got)
	}
	if !strings.HasSuffix(got, "assistant: ") {
		t.Errorf("buildFullPrompt should end with assistant: , got %q", got[len(got)-20:])
	}
}

// newMockMCPServer starts an httptest.Server that responds 200 for POST /v1/mcp/tools/call with body.
func newMockMCPServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/mcp/tools/call" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

// TestHandoffRequest_MessagesOnly verifies orchestrator handoff format compatibility:
// body with only "messages" (no project_id, task_id, additional_context) decodes and yields baseline-only context.
func TestHandoffRequest_MessagesOnly(t *testing.T) {
	// Same shape as orchestrator pmaclient.CompletionRequest.
	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	var req InternalChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("decode handoff body: %v", err)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "hi" {
		t.Errorf("unexpected decoded request: %+v", req)
	}
	if req.ProjectID != "" || req.TaskID != "" || req.AdditionalContext != "" {
		t.Errorf("optional fields should be empty when not sent: project_id=%q task_id=%q additional_context=%q",
			req.ProjectID, req.TaskID, req.AdditionalContext)
	}
	baseline := "baseline"
	ctx := buildSystemContext(baseline, &req)
	if ctx != baseline {
		t.Errorf("context with messages-only handoff must be baseline only, got %q", ctx)
	}
}
