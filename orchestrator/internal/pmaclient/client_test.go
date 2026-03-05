package pmaclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallChatCompletion_EmptyURL(t *testing.T) {
	_, err := CallChatCompletion(context.Background(), nil, "", []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for empty base URL")
	}
}

func TestCallChatCompletion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/chat/completion" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req CompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CompletionResponse{Content: "ok"})
	}))
	defer server.Close()

	content, err := CallChatCompletion(context.Background(), nil, server.URL, []ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("CallChatCompletion: %v", err)
	}
	if content != "ok" {
		t.Errorf("content want ok, got %q", content)
	}
}

func TestCallChatCompletion_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := CallChatCompletion(context.Background(), nil, server.URL, []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestCallChatCompletion_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	_, err := CallChatCompletion(context.Background(), nil, server.URL, []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestCallChatCompletion_WithCustomClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(CompletionResponse{Content: "custom"})
	}))
	defer server.Close()

	client := &http.Client{}
	content, err := CallChatCompletion(context.Background(), client, server.URL, []ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("CallChatCompletion: %v", err)
	}
	if content != "custom" {
		t.Errorf("content want custom, got %q", content)
	}
}

func TestCallChatCompletion_DoError(t *testing.T) {
	// Use a URL that will fail on Do (connection refused or no route).
	_, err := CallChatCompletion(context.Background(), nil, "http://127.0.0.1:19999", []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error when server unreachable")
	}
}

func TestCallChatCompletion_ManagedProxySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/worker/managed-services/pma-main/proxy:http" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req managedProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Path != "/internal/chat/completion" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		completion := CompletionResponse{Content: "proxied-ok"}
		completionRaw, _ := json.Marshal(completion)
		_ = json.NewEncoder(w).Encode(managedProxyResponse{
			Version: 1,
			Status:  http.StatusOK,
			BodyB64: base64.StdEncoding.EncodeToString(completionRaw),
		})
	}))
	defer server.Close()
	url := server.URL + "/v1/worker/managed-services/pma-main/proxy:http"
	content, err := CallChatCompletion(context.Background(), nil, url, []ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("CallChatCompletion via proxy: %v", err)
	}
	if content != "proxied-ok" {
		t.Errorf("content want proxied-ok, got %q", content)
	}
}

func TestCallChatCompletion_ManagedProxyUpstreamError(t *testing.T) {
	assertManagedProxyCallError(t, managedProxyResponse{
		Version: 1,
		Status:  http.StatusBadGateway,
		BodyB64: "",
	})
}

func TestCallChatCompletion_ManagedProxyTransportStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	url := server.URL + "/v1/worker/managed-services/pma-main/proxy:http"
	_, err := CallChatCompletion(context.Background(), nil, url, []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error when managed proxy endpoint returns non-200 status")
	}
}

func TestCallChatCompletion_ManagedProxyInvalidBase64(t *testing.T) {
	assertManagedProxyCallError(t, managedProxyResponse{
		Version: 1,
		Status:  http.StatusOK,
		BodyB64: "%%%not-base64%%%",
	})
}

func TestCallChatCompletion_ManagedProxyInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()
	url := server.URL + "/v1/worker/managed-services/pma-main/proxy:http"
	_, err := CallChatCompletion(context.Background(), nil, url, []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for invalid JSON managed proxy response")
	}
}

func TestLooksLikeManagedProxyEndpoint(t *testing.T) {
	if !looksLikeManagedProxyEndpoint("http://x/v1/worker/managed-services/svc/proxy:http") {
		t.Error("expected managed proxy URL to match")
	}
	if looksLikeManagedProxyEndpoint("http://x/internal/chat/completion") {
		t.Error("direct PMA URL should not be treated as managed proxy URL")
	}
}

func TestCallChatCompletion_ManagedProxyInvalidCompletionJSON(t *testing.T) {
	assertManagedProxyCallError(t, managedProxyResponse{
		Version: 1,
		Status:  http.StatusOK,
		BodyB64: base64.StdEncoding.EncodeToString([]byte("{not-json")),
	})
}

func assertManagedProxyCallError(t *testing.T, resp managedProxyResponse) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()
	url := server.URL + "/v1/worker/managed-services/pma-main/proxy:http"
	_, err := CallChatCompletion(context.Background(), nil, url, []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatalf("expected managed proxy call to fail for response %+v", resp)
	}
}

// pmaHandoffBody mirrors the PMA InternalChatCompletionRequest so we can verify
// the orchestrator handoff format is compatible (messages required; project_id, task_id, additional_context optional).
type pmaHandoffBody struct {
	Messages          []ChatMessage `json:"messages"`
	ProjectID         string        `json:"project_id,omitempty"`
	TaskID            string        `json:"task_id,omitempty"`
	UserID            string        `json:"user_id,omitempty"`
	AdditionalContext string        `json:"additional_context,omitempty"`
}

// TestHandoffRequestFormat verifies the request body we send to PMA is valid and decodable as PMA handoff (messages only).
func TestHandoffRequestFormat(t *testing.T) {
	msgs := []ChatMessage{{Role: "user", Content: "hello"}}
	body := CompletionRequest{Messages: msgs}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded pmaHandoffBody
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("PMA handoff decode: %v", err)
	}
	if len(decoded.Messages) != 1 || decoded.Messages[0].Content != "hello" {
		t.Errorf("decoded messages = %+v", decoded.Messages)
	}
	if decoded.ProjectID != "" || decoded.TaskID != "" || decoded.AdditionalContext != "" {
		t.Errorf("optional fields should be empty in minimal handoff: %+v", decoded)
	}
}
