package workerapiserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewServer_NilConfig(t *testing.T) {
	_, err := NewServer(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewServer_NilPublicHandler(t *testing.T) {
	_, err := NewServer(&RunConfig{})
	if err == nil {
		t.Fatal("expected error for nil PublicHandler")
	}
}

func TestNewServer_SuccessWithDefaults(t *testing.T) {
	cfg := &RunConfig{
		PublicHandler: http.NewServeMux(),
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if srv.ListenAddr() == "" {
		t.Error("ListenAddr should be set")
	}
	if srv.InternalListenAddr() == "" {
		t.Error("InternalListenAddr should be set")
	}
}

func TestNewServer_EnvOverridesListenAddr(t *testing.T) {
	t.Setenv("LISTEN_ADDR", ":19999")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:19998")
	defer func() {
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
	}()
	cfg := &RunConfig{
		PublicHandler: http.NewServeMux(),
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if srv.ListenAddr() != ":19999" {
		t.Errorf("ListenAddr = %q", srv.ListenAddr())
	}
	if srv.InternalListenAddr() != "127.0.0.1:19998" {
		t.Errorf("InternalListenAddr = %q", srv.InternalListenAddr())
	}
}

func TestWithCallerServiceID(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Context().Value(CallerServiceIDContextKey).(string)
		w.WriteHeader(http.StatusOK)
	})
	wrapped := WithCallerServiceID(next, "svc-1")
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if got != "svc-1" {
		t.Errorf("context value = %q, want svc-1", got)
	}
}

func TestServer_StartAndShutdown(t *testing.T) {
	cfg := &RunConfig{
		PublicHandler:      http.NewServeMux(),
		InternalHandler:    http.NewServeMux(),
		ListenAddr:         "127.0.0.1:0",
		InternalListenAddr: "127.0.0.1:0",
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready, err := srv.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not become ready")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	cancel()
}

func TestServer_Run_CancelStops(t *testing.T) {
	cfg := &RunConfig{
		PublicHandler:      http.NewServeMux(),
		InternalHandler:    http.NewServeMux(),
		ListenAddr:         "127.0.0.1:0",
		InternalListenAddr: "127.0.0.1:0",
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	err = <-done
	if err != nil && err != context.Canceled {
		t.Errorf("Run: %v", err)
	}
}

func TestServer_Run_ServerErrPath(t *testing.T) {
	dir := t.TempDir()
	unixPath := filepath.Join(dir, "run.sock")
	cfg := &RunConfig{
		PublicHandler:      http.NewServeMux(),
		InternalHandler:    http.NewServeMux(),
		ListenAddr:         "127.0.0.1:0",
		InternalListenAddr: "127.0.0.1:0",
		InternalListenUnix: unixPath,
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)
	_ = os.Remove(unixPath)
	select {
	case err := <-done:
		if err != nil {
			t.Logf("Run returned error (expected after socket remove): %v", err)
		}
		cancel()
	case <-time.After(2 * time.Second):
		cancel()
		err = <-done
		if err != nil && err != context.Canceled {
			t.Logf("Run returned: %v", err)
		}
	}
}

func TestServer_StartWithInternalUnixAndPerServiceUDS(t *testing.T) {
	dir := t.TempDir()
	unixPath := filepath.Join(dir, "internal.sock")
	uds1 := filepath.Join(dir, "run", "svc1", "proxy.sock")
	uds2 := filepath.Join(dir, "run", "svc2", "proxy.sock")
	uds3 := filepath.Join(dir, "run", "svc3", "proxy.sock")
	cfg := &RunConfig{
		PublicHandler:      http.NewServeMux(),
		InternalHandler:    http.NewServeMux(),
		ListenAddr:         "127.0.0.1:0",
		InternalListenAddr: "127.0.0.1:0",
		InternalListenUnix: unixPath,
		SocketByService:    map[string]string{"svc1": uds1, "svc2": uds2, "svc3": uds3},
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready, err := srv.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	cancel()
}

func TestServer_Start_PerServiceUDSErrorPath(t *testing.T) {
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnlyDir, 0o444); err != nil {
		t.Skip("chmod read-only not supported")
	}
	defer func() { _ = os.Chmod(readOnlyDir, 0o755) }()
	udsPath := filepath.Join(readOnlyDir, "sub", "proxy.sock")
	cfg := &RunConfig{
		PublicHandler:      http.NewServeMux(),
		InternalHandler:    http.NewServeMux(),
		ListenAddr:         "127.0.0.1:0",
		InternalListenAddr: "127.0.0.1:0",
		SocketByService:    map[string]string{"svc1": udsPath},
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail when UDS dir not writable")
	}
}

func TestServer_Start_InvalidAddressFails(t *testing.T) {
	cfg := &RunConfig{
		PublicHandler:   http.NewServeMux(),
		InternalHandler: http.NewServeMux(),
		ListenAddr:      "invalid-addr-no-port",
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail with invalid address")
	}
}

func TestServer_Start_InvalidInternalListenAddrFails(t *testing.T) {
	cfg := &RunConfig{
		PublicHandler:      http.NewServeMux(),
		InternalHandler:    http.NewServeMux(),
		ListenAddr:         "127.0.0.1:0",
		InternalListenAddr: "invalid-internal-addr",
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail with invalid internal address")
	}
}

func TestServer_Start_InternalListenAddrEmpty(t *testing.T) {
	cfg := &RunConfig{
		PublicHandler:      http.NewServeMux(),
		InternalHandler:    http.NewServeMux(),
		ListenAddr:         "127.0.0.1:0",
		InternalListenAddr: "", // no internal TCP listener
		InternalListenUnix: "",
		SocketByService:    nil,
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready, err := srv.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	<-ready
	cancel()
	_ = srv.Shutdown(context.Background())
}

func TestServer_ShutdownWithCancelledContext(t *testing.T) {
	cfg := &RunConfig{
		PublicHandler:      http.NewServeMux(),
		InternalHandler:    http.NewServeMux(),
		ListenAddr:         "127.0.0.1:0",
		InternalListenAddr: "127.0.0.1:0",
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready, err := srv.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	<-ready
	cancel()
	// Shutdown with already-cancelled context may return error.
	deadCtx, deadCancel := context.WithCancel(context.Background())
	deadCancel()
	err = srv.Shutdown(deadCtx)
	if err != nil {
		t.Logf("Shutdown with cancelled context (expected): %v", err)
	}
	// Second Shutdown on already-shut-down servers.
	_ = srv.Shutdown(context.Background())
}
