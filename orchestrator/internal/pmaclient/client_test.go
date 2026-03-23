package pmaclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const pathChatCompletion = "/internal/chat/completion"
const pathManagedProxy = "/v1/worker/managed-services/pma-main/proxy:http"

func TestCallChatCompletion_EmptyURL(t *testing.T) {
	_, err := CallChatCompletion(context.Background(), nil, "", []ChatMessage{{Role: "user", Content: "hi"}}, "")
	if err == nil {
		t.Error("expected error for empty base URL")
	}
}

func TestStreamHTTPClient_NilUsesDefaultTimeout(t *testing.T) {
	c := streamHTTPClient(nil)
	if c.Timeout != defaultPMAHTTPTimeout {
		t.Errorf("streamHTTPClient(nil) Timeout = %v, want %v", c.Timeout, defaultPMAHTTPTimeout)
	}
}

func TestStreamHTTPClient_NonNilReturnsSame(t *testing.T) {
	custom := &http.Client{Timeout: 42 * time.Second}
	c := streamHTTPClient(custom)
	if c != custom {
		t.Fatal("expected same client pointer")
	}
}

func TestCallChatCompletion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != pathChatCompletion {
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

	content, err := CallChatCompletion(context.Background(), nil, server.URL, []ChatMessage{{Role: "user", Content: "hi"}}, "")
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

	_, err := CallChatCompletion(context.Background(), nil, server.URL, []ChatMessage{{Role: "user", Content: "hi"}}, "")
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

	_, err := CallChatCompletion(context.Background(), nil, server.URL, []ChatMessage{{Role: "user", Content: "hi"}}, "")
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
	content, err := CallChatCompletion(context.Background(), client, server.URL, []ChatMessage{{Role: "user", Content: "hi"}}, "")
	if err != nil {
		t.Fatalf("CallChatCompletion: %v", err)
	}
	if content != "custom" {
		t.Errorf("content want custom, got %q", content)
	}
}

func TestCallChatCompletion_DoError(t *testing.T) {
	// Use a URL that will fail on Do (connection refused or no route).
	_, err := CallChatCompletion(context.Background(), nil, "http://127.0.0.1:19999", []ChatMessage{{Role: "user", Content: "hi"}}, "")
	if err == nil {
		t.Error("expected error when server unreachable")
	}
}

func TestCallChatCompletion_ManagedProxySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != pathManagedProxy {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req managedProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Path != pathChatCompletion {
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
	url := server.URL + pathManagedProxy
	content, err := CallChatCompletion(context.Background(), nil, url, []ChatMessage{{Role: "user", Content: "hi"}}, "")
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
	url := server.URL + pathManagedProxy
	_, err := CallChatCompletion(context.Background(), nil, url, []ChatMessage{{Role: "user", Content: "hi"}}, "")
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
	url := server.URL + pathManagedProxy
	_, err := CallChatCompletion(context.Background(), nil, url, []ChatMessage{{Role: "user", Content: "hi"}}, "")
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

// TestCallChatCompletion_ManagedProxySendsBearerToken verifies that when workerBearerToken is non-empty,
// the request to the managed proxy includes Authorization: Bearer <token>.
func TestCallChatCompletion_ManagedProxySendsBearerToken(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if r.Method != http.MethodPost || r.URL.Path != pathManagedProxy {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req managedProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		completion := CompletionResponse{Content: "ok"}
		completionRaw, _ := json.Marshal(completion)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(managedProxyResponse{
			Version: 1,
			Status:  http.StatusOK,
			BodyB64: base64.StdEncoding.EncodeToString(completionRaw),
		})
	}))
	defer server.Close()
	url := server.URL + pathManagedProxy
	_, err := CallChatCompletion(context.Background(), nil, url, []ChatMessage{{Role: "user", Content: "hi"}}, "secret-worker-token")
	if err != nil {
		t.Fatalf("CallChatCompletion via proxy: %v", err)
	}
	if authHeader != "Bearer secret-worker-token" {
		t.Errorf("expected Authorization Bearer header, got %q", authHeader)
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
	url := server.URL + pathManagedProxy
	_, err := CallChatCompletion(context.Background(), nil, url, []ChatMessage{{Role: "user", Content: "hi"}}, "")
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

func TestCallChatCompletionStream_EmptyURL(t *testing.T) {
	err := CallChatCompletionStream(context.Background(), nil, "", nil, "", func(string) error { return nil })
	if err == nil {
		t.Error("expected error for empty base URL")
	}
}

func TestCallChatCompletionStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathChatCompletion {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"delta":"hello"}` + "\n"))
		_, _ = w.Write([]byte(`{"delta":" world"}` + "\n"))
	}))
	defer server.Close()

	var got string
	err := CallChatCompletionStream(context.Background(), nil, server.URL,
		[]ChatMessage{{Role: "user", Content: "hi"}}, "", func(d string) error {
			got += d
			return nil
		})
	if err != nil {
		t.Fatalf("CallChatCompletionStream: %v", err)
	}
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestCallChatCompletionStreamWithCallbacks_IterationStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathChatCompletion {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"iteration_start":1}` + "\n"))
		_, _ = w.Write([]byte(`{"delta":"a"}` + "\n"))
		_, _ = w.Write([]byte(`{"iteration_start":2}` + "\n"))
		_, _ = w.Write([]byte(`{"delta":"b"}` + "\n"))
	}))
	defer server.Close()

	var deltas string
	var iterations []int
	cb := PMAStreamCallbacks{
		OnDelta:          func(d string) error { deltas += d; return nil },
		OnIterationStart: func(n int) error { iterations = append(iterations, n); return nil },
	}
	err := CallChatCompletionStreamWithCallbacks(context.Background(), nil, server.URL,
		[]ChatMessage{{Role: "user", Content: "hi"}}, "", cb)
	if err != nil {
		t.Fatalf("CallChatCompletionStreamWithCallbacks: %v", err)
	}
	if deltas != "ab" {
		t.Errorf("deltas = %q, want ab", deltas)
	}
	if len(iterations) != 2 || iterations[0] != 1 || iterations[1] != 2 {
		t.Errorf("iterations = %v, want [1, 2]", iterations)
	}
}

func TestCallChatCompletionStream_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	err := CallChatCompletionStream(context.Background(), nil, server.URL, nil, "", func(string) error { return nil })
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestCallChatCompletionStream_DoError(t *testing.T) {
	err := CallChatCompletionStream(context.Background(), nil, "http://127.0.0.1:19998", nil, "", func(string) error { return nil })
	if err == nil {
		t.Error("expected error for unreachable host")
	}
}

func TestCallChatCompletionStream_DeltaError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"delta":"x"}` + "\n"))
	}))
	defer server.Close()
	wantErr := fmt.Errorf("delta handler err")
	err := CallChatCompletionStream(context.Background(), nil, server.URL, nil, "", func(string) error {
		return wantErr
	})
	if err == nil || err.Error() != wantErr.Error() {
		t.Errorf("expected delta handler error, got %v", err)
	}
}

func TestCallChatCompletionStream_ContextCancelled(t *testing.T) {
	ready := make(chan struct{})
	unblock := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(ready)
		select {
		case <-unblock:
		case <-r.Context().Done():
		}
	}))
	defer func() {
		close(unblock)
		server.Close()
	}()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- CallChatCompletionStream(ctx, nil, server.URL, nil, "", func(string) error { return nil })
	}()
	<-ready
	cancel()
	err := <-done
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestCallChatCompletionStream_ManagedProxyStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathManagedProxy {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req managedProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"delta":"proxied"}` + "\n"))
	}))
	defer server.Close()
	url := server.URL + pathManagedProxy
	var got string
	err := CallChatCompletionStream(context.Background(), nil, url, nil, "tok", func(d string) error {
		got += d
		return nil
	})
	if err != nil {
		t.Fatalf("ManagedProxyStream: %v", err)
	}
	if got != "proxied" {
		t.Errorf("got %q, want proxied", got)
	}
}

func TestCallChatCompletionStream_ManagedProxyStream_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	url := server.URL + pathManagedProxy
	err := CallChatCompletionStream(context.Background(), nil, url, nil, "", func(string) error { return nil })
	if err == nil {
		t.Error("expected error for non-200 proxy response")
	}
}

func TestReadNDJSONStream_UnexpectedContentType(t *testing.T) {
	body := strings.NewReader("{}")
	err := readNDJSONStream(context.Background(), body, "text/plain", func(d string) error { return nil })
	if err == nil {
		t.Error("expected error for unexpected content type")
	}
}

func TestReadNDJSONStream_UnexpectedContentTypeSingleJSON(t *testing.T) {
	body := strings.NewReader(`{"content":"ok"}`)
	var got string
	err := readNDJSONStream(context.Background(), body, "application/octet-stream", func(d string) error {
		got = d
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error for decodable JSON body: %v", err)
	}
	if got != "ok" {
		t.Errorf("got %q, want ok", got)
	}
}

func TestProcessNDJSONLine_Empty(t *testing.T) {
	if err := processNDJSONLine([]byte(""), PMAStreamCallbacks{OnDelta: func(string) error { return nil }}); err != nil {
		t.Errorf("empty line should not error: %v", err)
	}
}

func TestProcessNDJSONLine_NoDelta(t *testing.T) {
	if err := processNDJSONLine([]byte(`{"other":"x"}`), PMAStreamCallbacks{OnDelta: func(string) error { return nil }}); err != nil {
		t.Errorf("no delta should not error: %v", err)
	}
}

func TestProcessNDJSONLine_InvalidJSON(t *testing.T) {
	if err := processNDJSONLine([]byte(`not json`), PMAStreamCallbacks{OnDelta: func(string) error { return nil }}); err != nil {
		t.Errorf("invalid JSON should not error (skipped): %v", err)
	}
}

func requireProcessNDJSONLineOK(t *testing.T, jsonl string, cb PMAStreamCallbacks) {
	t.Helper()
	if err := processNDJSONLine([]byte(jsonl), cb); err != nil {
		t.Fatal(err)
	}
}

func TestProcessNDJSONLine_Thinking(t *testing.T) {
	var captured string
	requireProcessNDJSONLineOK(t, `{"thinking":"note"}`, PMAStreamCallbacks{
		OnThinking: func(s string) error {
			captured = s
			return nil
		},
	})
	if captured != "note" {
		t.Fatalf("thinking = %q", captured)
	}
}

func TestProcessNDJSONLine_CallbackErrorPaths(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		setup func() (PMAStreamCallbacks, error)
	}{
		{
			name: "iterationStart",
			line: `{"iteration_start":1}`,
			setup: func() (PMAStreamCallbacks, error) {
				want := fmt.Errorf("stop")
				return PMAStreamCallbacks{OnIterationStart: func(int) error { return want }}, want
			},
		},
		{
			name: "delta",
			line: `{"delta":"x"}`,
			setup: func() (PMAStreamCallbacks, error) {
				want := fmt.Errorf("delta err")
				return PMAStreamCallbacks{OnDelta: func(string) error { return want }}, want
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb, want := tt.setup()
			err := processNDJSONLine([]byte(tt.line), cb)
			if err != want {
				t.Fatalf("err = %v, want %v", err, want)
			}
		})
	}
}

func TestProcessNDJSONLine_IterationStartNonIntSkipped(t *testing.T) {
	var sawIteration int
	err := processNDJSONLine([]byte(`{"iteration_start":"nope"}`), PMAStreamCallbacks{
		OnIterationStart: func(v int) error {
			sawIteration = v
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sawIteration != 0 {
		t.Errorf("iteration_start string should not invoke callback: saw=%d", sawIteration)
	}
}
