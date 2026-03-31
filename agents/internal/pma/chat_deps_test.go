package pma

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandlerDI_UsesInjectedDeps verifies completion uses ChatDeps (Ollama URL, model) without per-request env lookup.
func TestHandlerDI_UsesInjectedDeps(t *testing.T) {
	t.Setenv("INFERENCE_MODEL", "")
	t.Setenv("OLLAMA_BASE_URL", "")
	t.Setenv("INFERENCE_URL", "")
	t.Setenv("PMA_MCP_GATEWAY_URL", "")
	t.Setenv("MCP_GATEWAY_URL", "")
	srv := newMockInferenceServer(t, http.StatusOK, `{"message":{"content":"from-deps"}}`)
	defer srv.Close()
	deps := ChatDeps{
		MCP:            &MCPClient{},
		InferenceModel: "",
		OllamaBaseURL:  srv.URL,
	}
	handler := ChatCompletionHandler("sys", slog.Default(), deps)
	body := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var out InternalChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Content != "from-deps" {
		t.Fatalf("content %q", out.Content)
	}
}
