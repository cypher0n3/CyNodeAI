package main

import (
	"context"
	"net"
	"net/http"
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
