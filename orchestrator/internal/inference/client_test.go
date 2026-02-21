package inference

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCallGenerate_SingleJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" || r.Method != http.MethodPost {
			t.Errorf("unexpected path/method: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(GenerateChunk{Response: "Hello", Done: true})
	}))
	defer server.Close()

	ctx := context.Background()
	out, err := CallGenerate(ctx, nil, server.URL, "tinyllama", "Hi")
	if err != nil {
		t.Fatalf("CallGenerate: %v", err)
	}
	if out != "Hello" {
		t.Errorf("got %q want Hello", out)
	}
}

func TestCallGenerate_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(GenerateChunk{Error: "model not found"})
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := CallGenerate(ctx, nil, server.URL, "tinyllama", "Hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "inference error: model not found" {
		t.Errorf("got %v", err)
	}
}

func TestCallGenerate_EmptyURL(t *testing.T) {
	ctx := context.Background()
	_, err := CallGenerate(ctx, nil, "", "tinyllama", "Hi")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestCallGenerate_NDJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"response":"Hel","done":false}
{"response":"lo","done":true}
`))
	}))
	defer server.Close()
	ctx := context.Background()
	out, err := CallGenerate(ctx, nil, server.URL, "m", "Hi")
	if err != nil {
		t.Fatalf("CallGenerate: %v", err)
	}
	if out != "Hello" {
		t.Errorf("got %q want Hello", out)
	}
}

func TestCallGenerate_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	ctx := context.Background()
	_, err := CallGenerate(ctx, nil, server.URL, "m", "Hi")
	if err == nil {
		t.Fatal("expected error")
	}
}

func Test_parseGenerateResponseNDJSON_ErrorInChunk(t *testing.T) {
	_, err := parseGenerateResponseNDJSON(`{"error":"model not found"}`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCallGenerate_ClientDoError(t *testing.T) {
	// Use a URL that will fail (connection refused). Short timeout so test is fast.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	client := &http.Client{Timeout: 50 * time.Millisecond}
	_, err := CallGenerate(ctx, client, "http://127.0.0.1:19999", "m", "Hi")
	if err == nil {
		t.Fatal("expected error")
	}
}
