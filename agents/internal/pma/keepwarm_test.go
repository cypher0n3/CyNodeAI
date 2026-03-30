package pma

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestKeepWarm_LoopStopsOnCancel(t *testing.T) {
	var n atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	go runKeepWarmLoop(ctx, 25*time.Millisecond, nil, func(context.Context) error {
		n.Add(1)
		return nil
	})
	time.Sleep(90 * time.Millisecond)
	cancel()
	time.Sleep(40 * time.Millisecond)
	if n.Load() < 2 {
		t.Fatalf("expected multiple keep-warm ticks, got %d", n.Load())
	}
}

func TestKeepWarmIntervalFromEnv_Default(t *testing.T) {
	t.Setenv(envPMAKeepWarmIntervalSec, "")
	if d := KeepWarmIntervalFromEnv(); d != DefaultKeepWarmInterval {
		t.Fatalf("default interval: got %v want %v", d, DefaultKeepWarmInterval)
	}
}

func TestShouldRunKeepWarm_Localhost(t *testing.T) {
	t.Setenv(envPMADisableKeepWarm, "")
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:11434")
	if !shouldRunKeepWarm() {
		t.Fatal("expected keep-warm for 127.0.0.1")
	}
}

func TestShouldRunKeepWarm_Disabled(t *testing.T) {
	t.Setenv(envPMADisableKeepWarm, "1")
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:11434")
	if shouldRunKeepWarm() {
		t.Fatal("expected disabled")
	}
}

func TestShouldRunKeepWarm_UnixSocketURL(t *testing.T) {
	t.Setenv(envPMADisableKeepWarm, "")
	t.Setenv("OLLAMA_BASE_URL", "http+unix:///run/ollama.sock")
	if !shouldRunKeepWarm() {
		t.Fatal("expected keep-warm for UDS inference URL")
	}
}

func TestDefaultKeepWarmPing_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":{"content":"ok"}}`))
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)
	t.Setenv("INFERENCE_MODEL", "m")
	if err := defaultKeepWarmPing(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := LoadModelOnStart(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStartKeepWarm_InvokesPingHook(t *testing.T) {
	var n atomic.Int32
	prev := keepWarmPingHook
	keepWarmPingHook = func(context.Context) error {
		n.Add(1)
		return nil
	}
	defer func() { keepWarmPingHook = prev }()
	t.Setenv(envPMADisableKeepWarm, "")
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:11434")
	t.Setenv(envPMAKeepWarmIntervalSec, "1")
	ctx, cancel := context.WithCancel(context.Background())
	StartKeepWarm(ctx, nil)
	time.Sleep(2500 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
	if n.Load() < 2 {
		t.Fatalf("expected at least 2 pings, got %d", n.Load())
	}
}

func TestKeepWarmIntervalFromEnv_Parse(t *testing.T) {
	t.Setenv(envPMAKeepWarmIntervalSec, "60")
	if d := KeepWarmIntervalFromEnv(); d != 60*time.Second {
		t.Fatalf("got %v", d)
	}
}

func TestKeepWarmIntervalFromEnv_InvalidFallsBack(t *testing.T) {
	t.Setenv(envPMAKeepWarmIntervalSec, "not-a-number")
	if d := KeepWarmIntervalFromEnv(); d != DefaultKeepWarmInterval {
		t.Fatalf("got %v", d)
	}
}

func TestStartKeepWarm_NoOpWhenRemoteBackend(t *testing.T) {
	var n atomic.Int32
	prev := keepWarmPingHook
	keepWarmPingHook = func(context.Context) error {
		n.Add(1)
		return nil
	}
	defer func() { keepWarmPingHook = prev }()
	t.Setenv(envPMADisableKeepWarm, "")
	t.Setenv("OLLAMA_BASE_URL", "https://api.example.com/v1")
	ctx, cancel := context.WithCancel(context.Background())
	StartKeepWarm(ctx, nil)
	time.Sleep(200 * time.Millisecond)
	cancel()
	if n.Load() != 0 {
		t.Fatalf("expected no pings for remote backend, got %d", n.Load())
	}
}
