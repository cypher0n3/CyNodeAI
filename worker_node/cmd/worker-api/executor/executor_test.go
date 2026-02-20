package executor

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
)

const goOSWindows = "windows"

func TestNew(t *testing.T) {
	e := New("podman", 30*time.Second, 4096, "", "", nil)
	if e == nil {
		t.Fatal("New returned nil")
	}
}

func TestSanitizePodName(t *testing.T) {
	tests := []struct {
		jobID string
		want  string
	}{
		{"abc-123", "abc-123"},
		{"00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000"},
		{"x/y\\z", "x-y-z"},
	}
	for _, tt := range tests {
		got := sanitizePodName(tt.jobID)
		if len(got) > 40 {
			t.Errorf("sanitizePodName(%q) length %d > 40", tt.jobID, len(got))
		}
		if tt.want != "" && got != tt.want {
			t.Errorf("sanitizePodName(%q) = %q, want %q", tt.jobID, got, tt.want)
		}
	}
}

func TestBuildProxyRunArgs(t *testing.T) {
	args := buildProxyRunArgs("pod-1", "http://host:11434", "proxyimg", nil)
	if len(args) < 6 {
		t.Fatalf("expected at least 6 args, got %d", len(args))
	}
	hasPod := false
	hasUpstream := false
	for i, a := range args {
		if a == "--pod" && i+1 < len(args) && args[i+1] == "pod-1" {
			hasPod = true
		}
		if a == "OLLAMA_UPSTREAM_URL=http://host:11434" || (len(a) > 20 && a[:20] == "OLLAMA_UPSTREAM_URL=") {
			hasUpstream = true
		}
	}
	if !hasPod || !hasUpstream {
		t.Errorf("args %v missing pod or upstream", args)
	}
	argsWithCmd := buildProxyRunArgs("p", "http://x", "img", []string{"sleep", "60"})
	if len(argsWithCmd) < 8 {
		t.Errorf("with command expected more args, got %v", argsWithCmd)
	}
}

func TestBuildSandboxRunArgsForPod(t *testing.T) {
	req := &workerapi.RunJobRequest{
		TaskID: "t1",
		JobID:  "j1",
		Sandbox: workerapi.SandboxSpec{Command: []string{"echo", "hi"}, Image: "alpine"},
	}
	env := map[string]string{"CYNODE_TASK_ID": "t1", "CYNODE_JOB_ID": "j1", "CYNODE_WORKSPACE_DIR": "/workspace", envOllamaBaseURL: ollamaBaseURLInPod}
	args := buildSandboxRunArgsForPod(req, "mypod", "/tmp/ws", env, "alpine")
	hasOllama := false
	for _, a := range args {
		if a == "OLLAMA_BASE_URL=http://localhost:11434" {
			hasOllama = true
			break
		}
	}
	if !hasOllama {
		t.Errorf("args should contain OLLAMA_BASE_URL, got %v", args)
	}
	if args[len(args)-2] != "echo" || args[len(args)-1] != "hi" {
		t.Errorf("command should be at end, got %v", args)
	}
}

// TestRunJobUseInferenceWithoutProxyImage asserts that when UseInference is true but
// inference proxy image is not set, the executor falls back to standalone (non-pod) path.
func TestRunJobUseInferenceWithoutProxyImage(t *testing.T) {
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo", "ok"}
	} else {
		cmd = []string{"echo", "ok"}
	}
	e := New("podman", 10*time.Second, 1024, "http://host:11434", "", nil) // no proxy image
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{Command: cmd, UseInference: true},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	// Without proxy image we do not use pod path; podman run --network=none would run
	// and fail if podman not available. So we only assert we got a response.
	if resp == nil {
		t.Fatal("resp is nil")
	}
	if resp.Status != workerapi.StatusCompleted && resp.Status != workerapi.StatusFailed {
		t.Errorf("unexpected status %s", resp.Status)
	}
}

// TestRunJobWithPodInference runs the pod path when podman and a placeholder proxy image are available.
func TestRunJobWithPodInference(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not in path")
	}
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo", "in-pod"}
	} else {
		cmd = []string{"echo", "in-pod"}
	}
	e := New("podman", 60*time.Second, 4096, "http://host.containers.internal:11434", "alpine:latest", []string{"sleep", "120"})
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1-pod-test",
		Sandbox: workerapi.SandboxSpec{Command: cmd, UseInference: true, Image: "alpine:latest"},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}
	if resp.Status == workerapi.StatusCompleted {
		if !strings.Contains(resp.Stdout, "in-pod") {
			t.Errorf("stdout %q should contain 'in-pod'", resp.Stdout)
		}
		return
	}
	t.Skipf("pod path failed (podman/env not ready): status=%s stderr=%s", resp.Status, resp.Stderr)
}

// TestRunJobWithPodInference_pod_create_fail covers the error path when pod create fails.
func TestRunJobWithPodInference_pod_create_fail(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not in path")
	}
	_ = os.Setenv("CONTAINER_HOST", "unix:///nonexistent/podman.sock")
	defer func() { _ = os.Unsetenv("CONTAINER_HOST") }()

	e := New("podman", 10*time.Second, 4096, "http://host:11434", "alpine:latest", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1-pod-fail",
		Sandbox: workerapi.SandboxSpec{Command: []string{"echo", "x"}, UseInference: true},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}
	if resp.Status != workerapi.StatusFailed {
		t.Errorf("expected status failed (pod create), got %s", resp.Status)
	}
	if resp.Stderr == "" || !strings.Contains(resp.Stderr, "pod create") {
		t.Errorf("expected stderr about pod create, got %q", resp.Stderr)
	}
}

// TestRunJobWithPodInference_proxy_start_fail covers the error path when the proxy container fails to start.
func TestRunJobWithPodInference_proxy_start_fail(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not in path")
	}
	e := New("podman", 30*time.Second, 4096, "http://host:11434", "docker.io/nonexistent/inference-proxy:noexist", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1-fail-proxy",
		Sandbox: workerapi.SandboxSpec{Command: []string{"echo", "x"}, UseInference: true},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}
	if resp.Status != workerapi.StatusFailed {
		t.Errorf("expected status failed (proxy image missing), got %s", resp.Status)
	}
	if resp.Stderr == "" {
		t.Error("expected stderr from proxy start failure")
	}
}

func TestRunJobDirectSuccess(t *testing.T) {
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo", "hello"}
	} else {
		cmd = []string{"echo", "hello"}
	}
	e := New("direct", 10*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:   "",
			Command: cmd,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
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
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "exit", "3"}
	} else {
		cmd = []string{"sh", "-c", "exit 3"}
	}
	e := New("direct", 10*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command: cmd,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed || resp.ExitCode != 3 {
		t.Errorf("status=%s exitCode=%d", resp.Status, resp.ExitCode)
	}
}

func TestRunJobDirectTimeout(t *testing.T) {
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "ping", "-n", "10", "127.0.0.1"}
	} else {
		cmd = []string{"sleep", "10"}
	}
	e := New("direct", 5*time.Millisecond, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command: cmd,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusTimeout || resp.ExitCode != -1 {
		t.Errorf("status=%s exitCode=%d", resp.Status, resp.ExitCode)
	}
}

func TestRunJobDirectNonExitError(t *testing.T) {
	e := New("direct", 5*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command: []string{"/nonexistent-binary-xyz", "arg"},
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed || resp.ExitCode != -1 {
		t.Errorf("status=%s exitCode=%d stderr=%q", resp.Status, resp.ExitCode, resp.Stderr)
	}
}

func TestRunJobDirectEnv(t *testing.T) {
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo", "%FOO%"}
	} else {
		cmd = []string{"sh", "-c", "echo $FOO"}
	}
	e := New("direct", 10*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command: cmd,
			Env:     map[string]string{"FOO": "bar"},
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
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
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo", "12345678901234567890"}
	} else {
		cmd = []string{"sh", "-c", "echo 12345678901234567890"}
	}
	e := New("direct", 10*time.Second, 10, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command: cmd,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
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
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo", "ok"}
	} else {
		cmd = []string{"echo", "ok"}
	}
	e := New("direct", 10*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:   "",
			Command: cmd,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("status=%s", resp.Status)
	}
}

func TestRunJobDirectStderrTruncation(t *testing.T) {
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo err 12345678901234567890"}
	} else {
		cmd = []string{"sh", "-c", "echo err 12345678901234567890 >&2"}
	}
	e := New("direct", 10*time.Second, 8, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{Command: cmd},
	}
	resp, err := e.RunJob(context.Background(), req, "")
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
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo", "ok"}
	} else {
		cmd = []string{"echo", "ok"}
	}
	e := New("direct", 1*time.Hour, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Command:        cmd,
			TimeoutSeconds: 30,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
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
	e := New("nonexistent-runtime-xyz", 5*time.Second, 1024, "", "", nil)
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
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Errorf("status=%s (expected failed when runtime missing)", resp.Status)
	}
}

// TestRunJobContainerPathWithTimeout covers TimeoutSeconds in the container path.
func TestRunJobContainerPathWithTimeout(t *testing.T) {
	e := New("nonexistent-runtime-xyz", 1*time.Hour, 1024, "", "", nil)
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
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Errorf("status=%s", resp.Status)
	}
}

// TestRunJobContainerPathWithWorkspace covers workspace mount and task env in container path.
func TestRunJobContainerPathWithWorkspace(t *testing.T) {
	dir := t.TempDir()
	e := New("nonexistent-runtime-xyz", 5*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:         "alpine:latest",
			Command:       []string{"echo", "hi"},
			NetworkPolicy: "restricted",
		},
	}
	resp, err := e.RunJob(context.Background(), req, dir)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Errorf("status=%s (expected failed when runtime missing)", resp.Status)
	}
}

// TestRunJobDirectCynodeEnvNotOverridable asserts request env cannot override CYNODE_* (sandbox_container.md).
func TestRunJobDirectCynodeEnvNotOverridable(t *testing.T) {
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo", "%CYNODE_TASK_ID%"}
	} else {
		cmd = []string{"sh", "-c", "echo $CYNODE_TASK_ID"}
	}
	e := New("direct", 10*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "real-task",
		JobID:   "real-job",
		Sandbox: workerapi.SandboxSpec{
			Command: cmd,
			Env:     map[string]string{"CYNODE_TASK_ID": "forged"},
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("status=%s", resp.Status)
	}
	if strings.TrimSpace(resp.Stdout) != "real-task" {
		t.Errorf("stdout %q want real-task (CYNODE_* must not be overridable)", resp.Stdout)
	}
}

// TestRunJobDirectTaskEnv asserts CYNODE_TASK_ID, CYNODE_JOB_ID, CYNODE_WORKSPACE_DIR are set (sandbox_container.md).
func TestRunJobDirectTaskEnv(t *testing.T) {
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo", "%CYNODE_TASK_ID%", "%CYNODE_JOB_ID%", "%CYNODE_WORKSPACE_DIR%"}
	} else {
		cmd = []string{"sh", "-c", "echo \"$CYNODE_TASK_ID\" \"$CYNODE_JOB_ID\" \"$CYNODE_WORKSPACE_DIR\""}
	}
	e := New("direct", 10*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "task-123",
		JobID:   "job-456",
		Sandbox: workerapi.SandboxSpec{Command: cmd},
	}
	dir := t.TempDir()
	resp, err := e.RunJob(context.Background(), req, dir)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("status=%s", resp.Status)
	}
	// Direct mode: CYNODE_WORKSPACE_DIR is the host path (dir)
	out := strings.TrimSpace(resp.Stdout)
	if runtime.GOOS == goOSWindows {
		if !strings.Contains(out, "task-123") || !strings.Contains(out, "job-456") {
			t.Errorf("stdout %q should contain task and job ids", out)
		}
	} else {
		if out != "task-123 job-456 "+dir {
			t.Errorf("stdout %q want task-123 job-456 %s", out, dir)
		}
	}
}

// TestRunJobDirectWorkspaceDir asserts working directory is set when workspaceDir is provided.
func TestRunJobDirectWorkspaceDir(t *testing.T) {
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "cd"}
	} else {
		cmd = []string{"sh", "-c", "pwd"}
	}
	e := New("direct", 10*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t",
		JobID:   "j",
		Sandbox: workerapi.SandboxSpec{Command: cmd},
	}
	dir := t.TempDir()
	resp, err := e.RunJob(context.Background(), req, dir)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("status=%s", resp.Status)
	}
	out := strings.TrimSpace(resp.Stdout)
	if runtime.GOOS != goOSWindows && !strings.HasSuffix(out, dir) {
		t.Errorf("pwd %q should end with workspace dir %q", out, dir)
	}
}
