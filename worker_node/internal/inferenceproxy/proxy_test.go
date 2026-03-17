package inferenceproxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewProxy_forwards_to_upstream(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()

	u, _ := url.Parse(backend.URL)
	proxy := NewProxy(u)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:11434/", http.NoBody)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("got body %q, want \"ok\"", rec.Body.String())
	}
}

func TestNewProxy_rejects_large_body(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("backend should not be called for oversized body")
	}))
	defer backend.Close()

	u, _ := url.Parse(backend.URL)
	proxy := NewProxy(u)

	// Body larger than limit (use 1 byte over to avoid allocating 10 MiB in test)
	body := make([]byte, MaxRequestBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "http://localhost:11434/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("got status %d, want 413", rec.Code)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }
func (errReader) Close() error             { return nil }

func errReadCloser() io.ReadCloser { return errReader{} }

func TestNewProxy_read_error_returns_500(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("backend should not be called")
	}))
	defer backend.Close()

	u, _ := url.Parse(backend.URL)
	proxy := NewProxy(u)

	req := httptest.NewRequest(http.MethodPost, "http://localhost:11434/", http.NoBody)
	req.Body = errReadCloser()
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500", rec.Code)
	}
}

// REQ-WORKER-0270: RunUDS starts a UDS listener; healthz is reachable and returns 200.
func TestRunUDS_HealthzReachableOverSocket(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	_ = os.Setenv("OLLAMA_UPSTREAM_URL", backend.URL)
	defer func() { _ = os.Unsetenv("OLLAMA_UPSTREAM_URL") }()

	sockPath := filepath.Join(t.TempDir(), "inference.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { done <- RunUDS(ctx, sockPath) }()

	var transport *http.Transport
	for i := 0; i < 40; i++ {
		if _, err := os.Stat(sockPath); err == nil {
			transport = &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", sockPath)
				},
			}
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if transport == nil {
		t.Fatalf("UDS socket %q did not appear", sockPath)
	}

	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	resp, err := client.Get("http://unix/healthz")
	if err != nil {
		t.Fatalf("healthz over UDS: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz status=%d, want 200", resp.StatusCode)
	}
	cancel()
	if code := <-done; code != 0 {
		t.Errorf("RunUDS exit code=%d", code)
	}
}

func TestRunUDS_InvalidUpstream_ReturnsOne(t *testing.T) {
	_ = os.Setenv("OLLAMA_UPSTREAM_URL", "://invalid")
	defer func() { _ = os.Unsetenv("OLLAMA_UPSTREAM_URL") }()
	sockPath := filepath.Join(t.TempDir(), "bad.sock")
	code := RunUDS(context.Background(), sockPath)
	if code != 1 {
		t.Errorf("RunUDS with invalid upstream = %d, want 1", code)
	}
}

func TestRunUDS_ListenFail_ReturnsOne(t *testing.T) {
	_ = os.Unsetenv("OLLAMA_UPSTREAM_URL")
	// Use a path in a non-existent directory to force listen failure.
	code := RunUDS(context.Background(), "/nonexistent/path/to/sock")
	if code != 1 {
		t.Errorf("RunUDS with bad path = %d, want 1", code)
	}
}
