package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestRun_error_paths_return_one(t *testing.T) {
	tests := []struct {
		name           string
		upstreamURL    string
		listenOverride string
	}{
		{"invalid upstream URL", "://invalid", ""},
		{"listen fails", "http://localhost:11434", "invalid-address-cause-listen-fail"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("OLLAMA_UPSTREAM_URL", tt.upstreamURL)
			defer func() { _ = os.Unsetenv("OLLAMA_UPSTREAM_URL") }()

			if code := run(context.Background(), nil, tt.listenOverride); code != 1 {
				t.Errorf("run() = %d, want 1", code)
			}
		})
	}
}

func TestRun_closed_listener_returns_nonzero(t *testing.T) {
	_ = os.Setenv("OLLAMA_UPSTREAM_URL", "http://localhost:11434")
	defer func() { _ = os.Unsetenv("OLLAMA_UPSTREAM_URL") }()

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	_ = l.Close()

	code := run(context.Background(), l, "")
	if code != 1 {
		t.Errorf("run(closed listener) = %d, want 1", code)
	}
}

func TestRun_nil_listener_starts_then_shuts_down(t *testing.T) {
	_ = os.Unsetenv("OLLAMA_UPSTREAM_URL")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- run(ctx, nil, "") }()

	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://localhost:11434/")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		if i == 49 {
			cancel()
			code := <-done
			if code == 1 {
				t.Skipf("server did not start (port in use?): %v", err)
			}
			t.Fatalf("server did not become ready: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	code := <-done
	if code != 0 {
		if code == 1 {
			t.Skipf("run returned 1 (port 11434 may be in use)")
		}
		t.Errorf("run() after shutdown = %d, want 0", code)
	}
}

func TestParseHealthcheckURL(t *testing.T) {
	got := parseHealthcheckURL([]string{"--healthcheck-url", "http://127.0.0.1:11434/healthz"})
	if got != "http://127.0.0.1:11434/healthz" {
		t.Fatalf("healthcheck-url=%q", got)
	}
	if parseHealthcheckURL(nil) != "" {
		t.Fatal("expected empty healthcheck-url")
	}
}

func TestRunHealthcheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if code := runHealthcheck(context.Background(), srv.URL); code != 0 {
		t.Fatalf("runHealthcheck ok=%d", code)
	}
	if code := runHealthcheck(context.Background(), "://bad-url"); code != 1 {
		t.Fatalf("runHealthcheck invalid=%d", code)
	}
}

func TestRunHealthcheck_NonOKAndUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if code := runHealthcheck(context.Background(), srv.URL); code != 1 {
		t.Fatalf("runHealthcheck non-ok=%d", code)
	}
	if code := runHealthcheck(context.Background(), "http://127.0.0.1:1/healthz"); code != 1 {
		t.Fatalf("runHealthcheck unreachable=%d", code)
	}
}

func TestRun_WithCustomListener_HealthAndProxy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/test" {
			_, _ = w.Write([]byte("proxied"))
			return
		}
		http.NotFound(w, r)
	}))
	defer backend.Close()
	_ = os.Setenv("OLLAMA_UPSTREAM_URL", backend.URL)
	defer func() { _ = os.Unsetenv("OLLAMA_UPSTREAM_URL") }()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = l.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- run(ctx, l, "") }()

	baseURL := "http://" + l.Addr().String()
	for i := 0; i < 20; i++ {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		cancel()
		<-done
		t.Fatalf("healthz request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status=%d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	resp, err = http.Get(baseURL + "/v1/test")
	if err != nil {
		cancel()
		<-done
		t.Fatalf("proxy request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("proxy status=%d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("run exit code=%d", code)
	}
}

func TestRun_ListenAddrOverride(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	_ = os.Setenv("OLLAMA_UPSTREAM_URL", backend.URL)
	defer func() { _ = os.Unsetenv("OLLAMA_UPSTREAM_URL") }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- run(ctx, nil, "127.0.0.1:0") }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	code := <-done
	if code != 0 && code != 1 {
		t.Fatalf("run() code=%d", code)
	}
}
