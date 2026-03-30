package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/agents/internal/pma"
)

// TestWriteTimeout asserts the HTTP write deadline does not truncate streaming before the
// PMA langchain completion budget (REQ-PMAGNT-0106 / cynode_pma.md): either disabled (0) or
// at least inference timeout plus a small margin.
func TestWriteTimeout(t *testing.T) {
	const margin = 10 * time.Second
	inf := pma.LangchainCompletionTimeout
	if pmaHTTPWriteTimeout != 0 && pmaHTTPWriteTimeout < inf+margin {
		t.Fatalf("WriteTimeout %v must be 0 (disabled for streaming) or >= %v (inference + %v margin); got %v",
			pmaHTTPWriteTimeout, inf+margin, margin, pmaHTTPWriteTimeout)
	}
}

func TestResolveRole(t *testing.T) {
	if resolveRole("project_manager") != "project_manager" {
		t.Error("flag value should win")
	}
	_ = os.Setenv("PMA_ROLE", "project_analyst")
	defer func() { _ = os.Unsetenv("PMA_ROLE") }()
	if resolveRole("") != "project_analyst" {
		t.Error("env should be used when flag empty")
	}
	_ = os.Unsetenv("PMA_ROLE")
	if resolveRole("") != "" {
		t.Error("empty when both empty")
	}
}

func TestResolveEnv(t *testing.T) {
	if resolveEnv("KEY", "flag") != "flag" {
		t.Error("flag value should win")
	}
	_ = os.Setenv("PMA_TEST_KEY", "envval")
	defer func() { _ = os.Unsetenv("PMA_TEST_KEY") }()
	if resolveEnv("PMA_TEST_KEY", "") != "envval" {
		t.Error("env should be used when flag empty")
	}
	_ = os.Unsetenv("PMA_TEST_KEY")
	if resolveEnv("PMA_TEST_KEY", "default") != "default" {
		t.Error("default when both empty")
	}
}

func TestRun_LoadInstructionsErrorExitsOne(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 not reliable on Windows")
	}
	dir := t.TempDir()
	pmDir := filepath.Join(dir, "project_manager")
	if err := os.MkdirAll(pmDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(pmDir, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(pmDir, 0o700) }()
	code := run(context.Background(), []string{"--role=project_manager", "--instructions-root=" + dir})
	if code != 1 {
		t.Errorf("run() with unreadable instructions dir = %d, want 1", code)
	}
}

func TestRun_ShutdownErrorExitsOne(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.md"), []byte("inst"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := shutdownServer
	defer func() { shutdownServer = old }()
	shutdownServer = func(context.Context, *http.Server) error {
		return errors.New("injected shutdown error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"--role=project_manager", "--listen=127.0.0.1:0", "--instructions-root=" + dir})
	}()
	time.Sleep(80 * time.Millisecond)
	cancel()
	code := <-done
	if code != 1 {
		t.Errorf("run() when shutdown fails = %d, want 1", code)
	}
}

func TestRun_FlagParseError(t *testing.T) {
	code := run(context.Background(), []string{"-unknown=1"})
	if code != 1 {
		t.Errorf("run() with bad flag = %d, want 1", code)
	}
}

func TestRun_InvalidRoleExitsOne(t *testing.T) {
	code := run(context.Background(), []string{"--role=invalid"})
	if code != 1 {
		t.Errorf("run() with invalid role = %d, want 1", code)
	}
}

func TestRunWithSignal_InvalidRoleExitsOne(t *testing.T) {
	code := runWithSignal([]string{"--role=invalid"})
	if code != 1 {
		t.Errorf("runWithSignal() with invalid role = %d, want 1", code)
	}
}

func TestRun_StartsAndShutsDown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.md"), []byte("inst"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("PMA_INSTRUCTIONS_ROOT", dir)
	defer func() { _ = os.Unsetenv("PMA_INSTRUCTIONS_ROOT") }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- run(ctx, []string{"--role=project_manager", "--listen=127.0.0.1:0"}) }()

	time.Sleep(80 * time.Millisecond)
	cancel()
	code := <-done
	if code != 0 {
		t.Errorf("run() after cancel = %d, want 0", code)
	}
}

func TestRun_NonexistentInstructionsPathStillStarts(t *testing.T) {
	_ = os.Setenv("PMA_INSTRUCTIONS_ROOT", "/nonexistent/path/that/does/not/exist")
	defer func() { _ = os.Unsetenv("PMA_INSTRUCTIONS_ROOT") }()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- run(ctx, []string{"--role=project_manager", "--listen=127.0.0.1:0"}) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	code := <-done
	if code != 0 {
		t.Errorf("run() = %d, want 0", code)
	}
}

func TestRun_HealthzResponds(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("inst"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("PMA_INSTRUCTIONS_ROOT", dir)
	defer func() { _ = os.Unsetenv("PMA_INSTRUCTIONS_ROOT") }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- run(ctx, []string{"--role=project_manager", "--listen=127.0.0.1:18090"}) }()

	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		resp, err := http.Get("http://127.0.0.1:18090/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if i == 49 {
			cancel()
			<-done
			t.Fatal("healthz did not become ready")
		}
	}
	cancel()
	if c := <-done; c != 0 {
		t.Errorf("run() = %d, want 0", c)
	}
}

// TestServeUnix_ListenFailReturnsError asserts that serveUnix returns an error when the socket
// path is invalid (e.g. parent dir cannot be created).
func TestServeUnix_ListenFailReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission test not meaningful")
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	// Use a path under a non-existent read-only root to force listen failure.
	err := serveUnix(server, "/proc/nonexistent/deep/service.sock", logger)
	if err == nil {
		t.Fatal("expected error from serveUnix with unwritable path")
	}
}

// TestServeHTTP_UnixPath asserts serveHTTP dispatches to serveUnix for unix: addresses.
func TestServeHTTP_UnixPath(t *testing.T) {
	sockDir := t.TempDir()
	sockPath := filepath.Join(sockDir, "s.sock")
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	done := make(chan struct{})
	go func() {
		serveHTTP(server, "unix:"+sockPath, logger)
		close(done)
	}()
	for i := 0; i < 30; i++ {
		time.Sleep(10 * time.Millisecond)
		if _, err := net.Dial("unix", sockPath); err == nil {
			break
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	<-done
}

// TestRun_ListensOnUnixSocket asserts REQ-WORKER-0174 / REQ-WORKER-0270: when PMA_LISTEN_ADDR
// starts with "unix:", PMA MUST bind a Unix domain socket and serve /healthz over it.
// This is required for --network=none containers where TCP is not reachable from the host.
func TestRun_ListensOnUnixSocket(t *testing.T) {
	sockDir := t.TempDir()
	sockPath := filepath.Join(sockDir, "service.sock")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "inst.md"), []byte("instructions"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PMA_INSTRUCTIONS_ROOT", dir)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"--role=project_manager", "--listen=unix:" + sockPath})
	}()

	var conn net.Conn
	var dialErr error
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		conn, dialErr = net.Dial("unix", sockPath)
		if dialErr == nil {
			break
		}
	}
	if dialErr != nil {
		cancel()
		<-done
		t.Fatalf("PMA did not bind UDS socket at %q within deadline: %v", sockPath, dialErr)
	}
	_ = conn.Close()

	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", sockPath)
		},
	}
	client := &http.Client{Transport: transport}
	resp, err := client.Get("http://pma/healthz")
	cancel()
	<-done
	if err != nil {
		t.Fatalf("GET /healthz over UDS: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
