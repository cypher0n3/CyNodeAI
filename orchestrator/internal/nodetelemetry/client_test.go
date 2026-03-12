package nodetelemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPullNodeInfo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/worker/telemetry/node:info" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Error("missing or wrong Authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":1,"node_slug":"n1"}`))
	}))
	defer srv.Close()

	client := NewClient()
	ctx := context.Background()
	body, err := client.PullNodeInfo(ctx, srv.URL, "tok")
	if err != nil {
		t.Fatalf("PullNodeInfo: %v", err)
	}
	if string(body) != `{"version":1,"node_slug":"n1"}` {
		t.Errorf("body: %s", body)
	}
}

// TestPullNodeStats_Success verifies GET node:stats and response body.
//
//nolint:dupl // server setup similar to TestPullNodeInfo_BaseURLWithTrailingSlash
func TestPullNodeStats_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/worker/telemetry/node:stats" {
			t.Errorf("path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":1,"captured_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	client := NewClient()
	body, err := client.PullNodeStats(context.Background(), srv.URL, "bearer")
	if err != nil {
		t.Fatalf("PullNodeStats: %v", err)
	}
	if string(body) != `{"version":1,"captured_at":"2026-01-01T00:00:00Z"}` {
		t.Errorf("body: %s", body)
	}
}

func TestPullNodeInfo_Timeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	client := NewClient()
	client.HTTPClient = &http.Client{Timeout: 10 * time.Millisecond}
	ctx := context.Background()
	_, err := client.PullNodeInfo(ctx, srv.URL, "")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestPullNodeInfo_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := NewClient()
	_, err := client.PullNodeInfo(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected error for 503")
	}
}

func TestPullNodeInfo_ContextCanceled(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewClient()
	_, err := client.PullNodeInfo(ctx, srv.URL, "")
	if err == nil {
		t.Fatal("expected error when context canceled")
	}
}

// TestPullNodeInfo_NilHTTPClient covers get() when c.HTTPClient is nil (fallback client).
func TestPullNodeInfo_NilHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient()
	client.HTTPClient = nil
	body, err := client.PullNodeInfo(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("PullNodeInfo with nil HTTPClient: %v", err)
	}
	if string(body) != "{}" {
		t.Errorf("body: %s", body)
	}
}

// TestPullNodeInfo_BaseURLWithTrailingSlash covers TrimSuffix in get().
//
//nolint:dupl // server setup similar to TestPullNodeStats_Success
func TestPullNodeInfo_BaseURLWithTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/worker/telemetry/node:info" {
			t.Errorf("path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient()
	_, err := client.PullNodeInfo(context.Background(), srv.URL+"/", "")
	if err != nil {
		t.Fatalf("PullNodeInfo with trailing slash: %v", err)
	}
}
