package workerapiserver

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestEmbedGetEnv(t *testing.T) {
	key := "TEST_EMBED_GETENV"
	_ = os.Unsetenv(key)
	if v := embedGetEnv(key, "default"); v != "default" {
		t.Errorf("got %q", v)
	}
	_ = os.Setenv(key, "set")
	defer func() { _ = os.Unsetenv(key) }()
	if v := embedGetEnv(key, "default"); v != "set" {
		t.Errorf("got %q", v)
	}
}

func TestEmbedGetEnvInt(t *testing.T) {
	key := "TEST_EMBED_GETENV_INT"
	_ = os.Unsetenv(key)
	wantDefault := 99
	if v := embedGetEnvInt(key, wantDefault); v != wantDefault {
		t.Errorf("default: got %d", v)
	}
	_ = os.Setenv(key, "42")
	defer func() { _ = os.Unsetenv(key) }()
	wantSet := 42
	if v := embedGetEnvInt(key, wantDefault); v != wantSet {
		t.Errorf("set: got %d", v)
	}
}

func TestRunEmbedded_StartFailsWithInvalidAddress(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "invalid-addr-no-port")
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	defer func() {
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
	}()
	_, _, err := RunEmbedded(context.Background(), EmbedConfig{
		BearerToken: "tok",
		StateDir:    "",
		Logger:      slog.Default(),
	})
	if err == nil {
		t.Fatal("expected Start to fail with invalid address")
	}
}

func TestRunEmbedded_InvalidNodeConfigJSON(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	t.Setenv("WORKER_NODE_CONFIG_JSON", "invalid-json")
	defer func() {
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		_ = os.Unsetenv("WORKER_NODE_CONFIG_JSON")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	ready, shutdown, err := RunEmbedded(ctx, EmbedConfig{
		BearerToken: "tok",
		StateDir:    "",
		Logger:      slog.Default(),
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	shutdown()
	cancel()
}

func TestRunEmbedded_StartAndShutdown(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	defer func() {
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	ready, shutdown, err := RunEmbedded(ctx, EmbedConfig{
		BearerToken: "test-token",
		StateDir:    "",
		Logger:      slog.Default(),
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for ready")
	}
	shutdown()
	cancel()
}

func runEmbeddedWithTargetsJSON(t *testing.T, targetsJSON string) {
	t.Helper()
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", targetsJSON)
	defer func() {
		_, _, _ = os.Unsetenv("LISTEN_ADDR"), os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR"), os.Unsetenv("WORKER_MANAGED_SERVICE_TARGETS_JSON")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready, shutdown, err := RunEmbedded(ctx, EmbedConfig{
		BearerToken: "test-token",
		StateDir:    t.TempDir(),
		Logger:      slog.Default(),
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for ready")
	}
	shutdown()
	time.Sleep(100 * time.Millisecond)
}

func TestRunEmbedded_WithManagedServiceTargets(t *testing.T) {
	runEmbeddedWithTargetsJSON(t, `{"pma1":{"service_type":"pma","base_url":"http://localhost:1"}}`)
}

func TestRunEmbedded_WithNonPMATarget_SkipsInferenceProxy(t *testing.T) {
	runEmbeddedWithTargetsJSON(t, `{"pma1":{"service_type":"pma","base_url":"http://localhost:1"},"other1":{"service_type":"other","base_url":"http://localhost:2"}}`)
}

func TestRunEmbedded_SBAInferenceProxySocketIsSet(t *testing.T) {
	_ = os.Unsetenv(SBAInferenceProxySocketEnv)
	stateDir := t.TempDir()
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:0")
	defer func() {
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready, shutdown, err := RunEmbedded(ctx, EmbedConfig{
		BearerToken: "test-token",
		StateDir:    stateDir,
		Logger:      slog.Default(),
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for ready")
	}
	defer shutdown()

	// SBA_INFERENCE_PROXY_SOCKET must be set by embed.
	sockPath := os.Getenv(SBAInferenceProxySocketEnv)
	if sockPath == "" {
		t.Fatalf("SBA_INFERENCE_PROXY_SOCKET not set after RunEmbedded (worker embed must start SBA inference proxy)")
	}

	// Socket file must appear within a reasonable timeout.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Errorf("SBA inference proxy socket %q did not appear within 5s (REQ-WORKER-0260 non-pod path)", sockPath)
}

func TestRunEmbedded_WithStateDirAsFile_MkdirAllFailsInProxyStart(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", `{"pma1":{"service_type":"pma","base_url":"http://localhost:1"}}`)
	defer func() {
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_MANAGED_SERVICE_TARGETS_JSON")
	}()
	dir := t.TempDir()
	stateDirAsFile := dir + "/file"
	if err := os.WriteFile(stateDirAsFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready, shutdown, err := RunEmbedded(ctx, EmbedConfig{
		BearerToken: "test-token",
		StateDir:    stateDirAsFile,
		Logger:      slog.Default(),
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for ready")
	}
	shutdown()
	cancel()
}
