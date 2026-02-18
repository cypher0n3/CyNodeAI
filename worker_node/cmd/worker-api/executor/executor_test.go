package executor

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
)

func TestNew(t *testing.T) {
	e := New("podman", 30*time.Second, 4096)
	if e == nil {
		t.Fatal("New returned nil")
	}
}

func TestRunJobDirectSuccess(t *testing.T) {
	var cmd []string
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/c", "echo", "hello"}
	} else {
		cmd = []string{"echo", "hello"}
	}
	e := New("direct", 10*time.Second, 1024)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:   "",
			Command: cmd,
		},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted || resp.ExitCode != 0 {
		t.Errorf("status=%s exitCode=%d", resp.Status, resp.ExitCode)
	}
	if resp.Stdout != "hello\n" && resp.Stdout != "hello\r\n" {
		t.Errorf("stdout=%q", resp.Stdout)
	}
}

func TestRunJobDirectExitError(t *testing.T) {
	var cmd []string
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/c", "exit", "3"}
	} else {
		cmd = []string{"sh", "-c", "exit 3"}
	}
	e := New("direct", 10*time.Second, 1024)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command: cmd,
		},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed || resp.ExitCode != 3 {
		t.Errorf("status=%s exitCode=%d", resp.Status, resp.ExitCode)
	}
}

func TestRunJobDirectTimeout(t *testing.T) {
	var cmd []string
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/c", "ping", "-n", "10", "127.0.0.1"}
	} else {
		cmd = []string{"sleep", "10"}
	}
	e := New("direct", 5*time.Millisecond, 1024)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command: cmd,
		},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusTimeout || resp.ExitCode != -1 {
		t.Errorf("status=%s exitCode=%d", resp.Status, resp.ExitCode)
	}
}

func TestRunJobDirectNonExitError(t *testing.T) {
	e := New("direct", 5*time.Second, 1024)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command: []string{"/nonexistent-binary-xyz", "arg"},
		},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed || resp.ExitCode != -1 {
		t.Errorf("status=%s exitCode=%d stderr=%q", resp.Status, resp.ExitCode, resp.Stderr)
	}
}

func TestRunJobDirectEnv(t *testing.T) {
	var cmd []string
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/c", "echo", "%FOO%"}
	} else {
		cmd = []string{"sh", "-c", "echo $FOO"}
	}
	e := New("direct", 10*time.Second, 1024)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command: cmd,
			Env:     map[string]string{"FOO": "bar"},
		},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("status=%s", resp.Status)
	}
	if resp.Stdout != "bar\n" && resp.Stdout != "bar\r\n" && resp.Stdout != "bar" {
		t.Errorf("stdout=%q (env FOO=bar)", resp.Stdout)
	}
}

func TestRunJobDirectTruncation(t *testing.T) {
	var cmd []string
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/c", "echo", "12345678901234567890"}
	} else {
		cmd = []string{"sh", "-c", "echo 12345678901234567890"}
	}
	e := New("direct", 10*time.Second, 10)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command: cmd,
		},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if !resp.Truncated.Stdout {
		t.Errorf("expected stdout truncated")
	}
	if len(resp.Stdout) > 10 {
		t.Errorf("stdout should be truncated to 10, got len %d", len(resp.Stdout))
	}
}

func TestRunJobDefaultImage(t *testing.T) {
	var cmd []string
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/c", "echo", "ok"}
	} else {
		cmd = []string{"echo", "ok"}
	}
	e := New("direct", 10*time.Second, 1024)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:   "",
			Command: cmd,
		},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("status=%s", resp.Status)
	}
}

func TestRunJobDirectStderrTruncation(t *testing.T) {
	var cmd []string
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/c", "echo err 12345678901234567890"}
	} else {
		cmd = []string{"sh", "-c", "echo err 12345678901234567890 >&2"}
	}
	e := New("direct", 10*time.Second, 8)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{Command: cmd},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if !resp.Truncated.Stderr {
		t.Errorf("expected stderr truncated")
	}
	if len(resp.Stderr) > 8 {
		t.Errorf("stderr should be truncated to 8, got len %d", len(resp.Stderr))
	}
}

func TestRunJobSandboxTimeoutSeconds(t *testing.T) {
	var cmd []string
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd", "/c", "echo", "ok"}
	} else {
		cmd = []string{"echo", "ok"}
	}
	e := New("direct", 1*time.Hour, 1024)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command:        cmd,
			TimeoutSeconds: 30,
		},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("status=%s", resp.Status)
	}
}

// TestRunJobContainerPath covers the non-direct branch when runtime binary is missing.
// This exercises container args building and setRunError when exec fails.
func TestRunJobContainerPath(t *testing.T) {
	e := New("nonexistent-runtime-xyz", 5*time.Second, 1024)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:   "alpine:latest",
			Command: []string{"echo", "hi"},
			Env:     map[string]string{"K": "V"},
		},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Errorf("status=%s (expected failed when runtime missing)", resp.Status)
	}
}

// TestRunJobContainerPathWithTimeout covers TimeoutSeconds in the container path.
func TestRunJobContainerPathWithTimeout(t *testing.T) {
	e := New("nonexistent-runtime-xyz", 1*time.Hour, 1024)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:          "alpine:latest",
			Command:        []string{"echo", "hi"},
			TimeoutSeconds: 30,
		},
	}
	resp, err := e.RunJob(context.Background(), req)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Errorf("status=%s", resp.Status)
	}
}
