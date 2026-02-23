package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

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
	go func() { done <- run(ctx, []string{"--role=project_manager", "--listen=127.0.0.1:0", "--instructions-root=" + dir}) }()
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
