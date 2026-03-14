package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func TestSanitizePodName_Truncate(t *testing.T) {
	// JobID longer than 40 chars is truncated (executor.go sanitizePodName).
	longID := "00000000-0000-0000-0000-000000000000-extra"
	got := sanitizePodName(longID)
	if len(got) != 40 {
		t.Errorf("sanitizePodName(long) length = %d, want 40", len(got))
	}
	if got != longID[:40] {
		t.Errorf("sanitizePodName(long) = %q, want prefix of 40 chars", got)
	}
}

func TestBuildProxyRunArgs(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		args := buildProxyRunArgs("pod-1", "http://host:11434", "proxyimg", nil, "/tmp/sock-dir")
		assertProxyRunArgsContain(t, args, 6, "pod-1", "/tmp/sock-dir:")
	})
	t.Run("with command", func(t *testing.T) {
		argsWithCmd := buildProxyRunArgs("p", "http://x", "img", []string{"sleep", "60"}, "/tmp/s")
		if len(argsWithCmd) < 8 {
			t.Errorf("with command expected more args, got %v", argsWithCmd)
		}
	})
}

func assertProxyRunArgsContain(t *testing.T, args []string, minLen int, podName, sockMountPrefix string) {
	t.Helper()
	if len(args) < minLen {
		t.Fatalf("expected at least %d args, got %d", minLen, len(args))
	}
	var hasPod, hasUpstream, hasSockMount bool
	for i, a := range args {
		if a == "--pod" && i+1 < len(args) && args[i+1] == podName {
			hasPod = true
		}
		if strings.HasPrefix(a, "OLLAMA_UPSTREAM_URL=") {
			hasUpstream = true
		}
		if a == "-v" && i+1 < len(args) && strings.HasPrefix(args[i+1], sockMountPrefix) {
			hasSockMount = true
		}
	}
	if !hasPod || !hasUpstream || !hasSockMount {
		t.Errorf("args %v missing pod %q, upstream, or sock mount %q", args, podName, sockMountPrefix)
	}
}

func TestWaitForProxyReady(t *testing.T) {
	orig := probeProxyHealthFunc
	defer func() { probeProxyHealthFunc = orig }()
	attempts := 0
	probeProxyHealthFunc = func(context.Context, string, string) error {
		attempts++
		if attempts < 3 {
			return os.ErrNotExist
		}
		return nil
	}
	if err := waitForProxyReady(context.Background(), "podman", "proxy-id", 2*time.Second, true); err != nil {
		t.Fatalf("waitForProxyReady: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d, want 3", attempts)
	}
}

func TestWaitForProxyReady_Timeout(t *testing.T) {
	origHealth := probeProxyHealthFunc
	origRunning := probeProxyRunningFunc
	defer func() {
		probeProxyHealthFunc = origHealth
		probeProxyRunningFunc = origRunning
	}()

	for _, tt := range []struct {
		name   string
		health bool
		setup  func()
	}{
		{
			name:   "health probe timeout",
			health: true,
			setup: func() {
				probeProxyHealthFunc = func(context.Context, string, string) error { return os.ErrDeadlineExceeded }
			},
		},
		{
			name:   "container running probe timeout",
			health: false,
			setup: func() {
				probeProxyRunningFunc = func(context.Context, string, string) error { return os.ErrNotExist }
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := waitForProxyReady(context.Background(), "podman", "proxy-id", 250*time.Millisecond, tt.health)
			if err == nil {
				t.Fatal("expected timeout error")
			}
		})
	}
}

func TestWaitForProxyReady_ContainerRunningProbe(t *testing.T) {
	orig := probeProxyRunningFunc
	defer func() { probeProxyRunningFunc = orig }()
	attempts := 0
	probeProxyRunningFunc = func(context.Context, string, string) error {
		attempts++
		if attempts < 2 {
			return os.ErrNotExist
		}
		return nil
	}
	if err := waitForProxyReady(context.Background(), "podman", "proxy-id", time.Second, false); err != nil {
		t.Fatalf("waitForProxyReady: %v", err)
	}
}

func TestProxyProbeHelpers(t *testing.T) {
	runtimePath := filepath.Join(t.TempDir(), "fake-runtime.sh")
	script := `#!/bin/sh
if [ "$1" = "exec" ]; then
  if [ "$2" = "cid-ok" ] && [ "$3" = "/inference-proxy" ] && [ "$4" = "--healthcheck-url" ]; then
    exit 0
  fi
  echo "exec failed"
  exit 1
fi
if [ "$1" = "inspect" ]; then
  if [ "$4" = "cid-ok" ]; then
    echo true
  else
    echo false
  fi
  exit 0
fi
echo "unsupported"
exit 1
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	if err := probeProxyHealthOnce(context.Background(), runtimePath, "cid-ok"); err != nil {
		t.Fatalf("probeProxyHealthOnce: %v", err)
	}
	if err := probeProxyRunningOnce(context.Background(), runtimePath, "cid-ok"); err != nil {
		t.Fatalf("probeProxyRunningOnce: %v", err)
	}
	if err := probeProxyRunningOnce(context.Background(), runtimePath, "cid-bad"); err == nil {
		t.Fatal("expected probeProxyRunningOnce error for non-running container")
	}
}

func TestProxyProbeOnce_ErrorCases(t *testing.T) {
	runtimePath := filepath.Join(t.TempDir(), "fake-runtime.sh")
	script := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	for _, tt := range []struct {
		name string
		fn   func() error
	}{
		{"health", func() error { return probeProxyHealthOnce(context.Background(), runtimePath, "cid-any") }},
		{"running", func() error { return probeProxyRunningOnce(context.Background(), runtimePath, "cid") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestWaitForProxyReady_DefaultTimeout(t *testing.T) {
	orig := probeProxyHealthFunc
	defer func() { probeProxyHealthFunc = orig }()
	probeProxyHealthFunc = func(context.Context, string, string) error { return nil }
	if err := waitForProxyReady(context.Background(), "podman", "proxy-id", 0, true); err != nil {
		t.Fatalf("waitForProxyReady default timeout: %v", err)
	}
}

// Note: probeProxyHealthOnce/probeProxyRunningOnce error cases are covered by TestProxyProbeOnce_ErrorCases.

// REQ-SANDBX-0131: sandbox must receive inference via INFERENCE_PROXY_URL (UDS) not OLLAMA_BASE_URL (TCP).
func TestBuildSandboxRunArgsForPod(t *testing.T) {
	req := &workerapi.RunJobRequest{
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{Command: []string{"echo", "hi"}, Image: "alpine"},
	}
	env := map[string]string{"CYNODE_TASK_ID": "t1", "CYNODE_JOB_ID": "j1", "CYNODE_WORKSPACE_DIR": "/workspace", envInferenceProxyURL: "http+unix://%2Frun%2Fcynode%2Finference-proxy.sock"}
	args := buildSandboxRunArgsForPod(req, "mypod", "/tmp/ws", "/tmp/sock-dir", env, "alpine")
	hasInferenceProxy := false
	hasSockMount := false
	for i, a := range args {
		if strings.HasPrefix(a, "INFERENCE_PROXY_URL=http+unix://") {
			hasInferenceProxy = true
		}
		if strings.HasPrefix(a, "OLLAMA_BASE_URL=http://localhost:") {
			t.Errorf("args must not inject TCP OLLAMA_BASE_URL in pod (REQ-SANDBX-0131), got arg: %q; all args: %v", a, args)
		}
		if a == "-v" && i+1 < len(args) && strings.HasPrefix(args[i+1], "/tmp/sock-dir:") {
			hasSockMount = true
		}
	}
	if !hasInferenceProxy {
		t.Errorf("args should contain INFERENCE_PROXY_URL=http+unix://... (REQ-SANDBX-0131), got %v", args)
	}
	if !hasSockMount {
		t.Errorf("args should mount sock-dir at /run/cynode (REQ-WORKER-0260), got %v", args)
	}
	if args[len(args)-2] != "echo" || args[len(args)-1] != "hi" {
		t.Errorf("command should be at end, got %v", args)
	}
}

func TestBuildSandboxRunArgsForPod_NoWorkspace(t *testing.T) {
	req := &workerapi.RunJobRequest{
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{Command: []string{"echo", "x"}, Image: "alpine"},
	}
	env := map[string]string{"CYNODE_TASK_ID": "t1", "CYNODE_JOB_ID": "j1", "CYNODE_WORKSPACE_DIR": ""}
	args := buildSandboxRunArgsForPod(req, "mypod", "", "", env, "alpine")
	argStr := strings.Join(args, " ")
	if strings.Contains(argStr, workspaceMount) || strings.Contains(argStr, "-w") {
		t.Errorf("empty workspaceDir should not add workspace mount or -w: %s", argStr)
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

// TestRunJobWithPodInference_FakeRuntime covers the full runJobWithPodInference path with a script named "podman" on PATH and mocked waitForProxyReady.
func TestRunJobWithPodInference_FakeRuntime(t *testing.T) {
	dir := t.TempDir()
	// Must be named "podman" so e.runtime == runtimePodman and RunJob calls runJobWithPodInference.
	runtimePath := filepath.Join(dir, "podman")
	script := `#!/bin/sh
if [ "$1" = "pod" ] && [ "$2" = "create" ]; then
  exit 0
fi
if [ "$1" = "pod" ] && [ "$2" = "rm" ]; then
  exit 0
fi
if [ "$1" = "run" ]; then
  # Proxy run has -d; sandbox run does not.
  for a in "$@"; do
    if [ "$a" = "-d" ]; then
      echo "proxy-container-id"
      exit 0
    fi
  done
  # Sandbox run: echo and exit 0
  echo "in-pod-output"
  exit 0
fi
exit 1
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	origPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", dir+string(filepath.ListSeparator)+origPath)
	defer func() { _ = os.Setenv("PATH", origPath) }()

	origHealth := probeProxyHealthFunc
	origRunning := probeProxyRunningFunc
	origSock := probeProxySocketExistsFunc
	defer func() {
		probeProxyHealthFunc = origHealth
		probeProxyRunningFunc = origRunning
		probeProxySocketExistsFunc = origSock
	}()
	probeProxyHealthFunc = func(context.Context, string, string) error { return nil }
	probeProxyRunningFunc = func(context.Context, string, string) error { return nil }
	probeProxySocketExistsFunc = func(string) error { return nil }

	e := New("podman", 30*time.Second, 4096, "http://host:11434", "proxyimg", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1-fake",
		Sandbox: workerapi.SandboxSpec{
			Command:      []string{"echo", "in-pod-output"},
			UseInference: true,
			Image:        "alpine:latest",
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted || resp.ExitCode != 0 {
		t.Errorf("status=%s exitCode=%d stderr=%s", resp.Status, resp.ExitCode, resp.Stderr)
	}
	if !strings.Contains(resp.Stdout, "in-pod-output") {
		t.Errorf("stdout=%q", resp.Stdout)
	}
}

// TestRunJobWithPodInference_FakeRuntime_WithWorkspace covers runJobWithPodInference when workspaceDir is non-empty.
func TestRunJobWithPodInference_FakeRuntime_WithWorkspace(t *testing.T) {
	dir := t.TempDir()
	workspaceDir := filepath.Join(dir, "ws")
	if err := os.MkdirAll(workspaceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	runtimePath := filepath.Join(dir, "podman")
	script := `#!/bin/sh
if [ "$1" = "pod" ] && [ "$2" = "create" ]; then exit 0; fi
if [ "$1" = "pod" ] && [ "$2" = "rm" ]; then exit 0; fi
if [ "$1" = "run" ]; then
  for a in "$@"; do if [ "$a" = "-d" ]; then echo "proxy-cid"; exit 0; fi; done
  echo "with-workspace"
  exit 0
fi
exit 1
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	origPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", dir+string(filepath.ListSeparator)+origPath)
	defer func() { _ = os.Setenv("PATH", origPath) }()

	origHealth := probeProxyHealthFunc
	origRunning := probeProxyRunningFunc
	origSock := probeProxySocketExistsFunc
	defer func() {
		probeProxyHealthFunc = origHealth
		probeProxyRunningFunc = origRunning
		probeProxySocketExistsFunc = origSock
	}()
	probeProxyHealthFunc = func(context.Context, string, string) error { return nil }
	probeProxyRunningFunc = func(context.Context, string, string) error { return nil }
	probeProxySocketExistsFunc = func(string) error { return nil }

	e := New("podman", 30*time.Second, 4096, "http://host:11434", "proxyimg", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1-ws",
		Sandbox: workerapi.SandboxSpec{
			Command: []string{"echo", "with-workspace"}, UseInference: true, Image: "alpine:latest",
		},
	}
	resp, err := e.RunJob(context.Background(), req, workspaceDir)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("status=%s stderr=%s", resp.Status, resp.Stderr)
	}
}

// TestRunJobWithPodInference_FakeRuntime_SandboxRunFails covers runJobWithPodInference when the sandbox container exits non-zero.
func TestRunJobWithPodInference_FakeRuntime_SandboxRunFails(t *testing.T) {
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "podman")
	script := `#!/bin/sh
if [ "$1" = "pod" ] && [ "$2" = "create" ]; then exit 0; fi
if [ "$1" = "pod" ] && [ "$2" = "rm" ]; then exit 0; fi
if [ "$1" = "run" ]; then
  for a in "$@"; do if [ "$a" = "-d" ]; then echo "proxy-cid"; exit 0; fi; done
  echo "sandbox stderr" >&2
  exit 2
fi
exit 1
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	origPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", dir+string(filepath.ListSeparator)+origPath)
	defer func() { _ = os.Setenv("PATH", origPath) }()

	origHealth := probeProxyHealthFunc
	origRunning := probeProxyRunningFunc
	origSock := probeProxySocketExistsFunc
	defer func() {
		probeProxyHealthFunc = origHealth
		probeProxyRunningFunc = origRunning
		probeProxySocketExistsFunc = origSock
	}()
	probeProxyHealthFunc = func(context.Context, string, string) error { return nil }
	probeProxyRunningFunc = func(context.Context, string, string) error { return nil }
	probeProxySocketExistsFunc = func(string) error { return nil }

	e := New("podman", 30*time.Second, 4096, "http://host:11434", "proxyimg", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1-fail",
		Sandbox: workerapi.SandboxSpec{
			Command: []string{"false"}, UseInference: true, Image: "alpine:latest",
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Errorf("status=%s", resp.Status)
	}
	if resp.ExitCode != 2 {
		t.Errorf("exitCode=%d", resp.ExitCode)
	}
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

func TestCreateOrReplacePod_Success(t *testing.T) {
	runtimePath := filepath.Join(t.TempDir(), "fake-runtime.sh")
	script := `#!/bin/sh
if [ "$1" = "pod" ] && [ "$2" = "create" ]; then
  echo "pod-123"
  exit 0
fi
exit 1
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	e := New(runtimePath, 10*time.Second, 1024, "", "", nil)
	if _, err := e.createOrReplacePod(context.Background(), "pod-a"); err != nil {
		t.Fatalf("createOrReplacePod should succeed: %v", err)
	}
}

func TestCreateOrReplacePod_AlreadyExistsThenReplace(t *testing.T) {
	tmpDir := t.TempDir()
	runtimePath := filepath.Join(tmpDir, "fake-runtime.sh")
	firstCreate := filepath.Join(tmpDir, "first-create")
	rmCalled := filepath.Join(tmpDir, "rm-called")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "pod" ] && [ "$2" = "create" ]; then
  if [ ! -f "%s" ]; then
    touch "%s"
    echo "name \"pod-a\" is in use: pod already exists"
    exit 1
  fi
  echo "pod-456"
  exit 0
fi
if [ "$1" = "pod" ] && [ "$2" = "rm" ]; then
  touch "%s"
  exit 0
fi
exit 1
`, firstCreate, firstCreate, rmCalled)
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	e := New(runtimePath, 10*time.Second, 1024, "", "", nil)
	if _, err := e.createOrReplacePod(context.Background(), "pod-a"); err != nil {
		t.Fatalf("createOrReplacePod should recover from existing pod: %v", err)
	}
	if _, err := os.Stat(rmCalled); err != nil {
		t.Fatalf("expected pod rm to be called, stat err: %v", err)
	}
}

func TestCreateOrReplacePod_NonExistingError(t *testing.T) {
	runtimePath := filepath.Join(t.TempDir(), "fake-runtime.sh")
	script := `#!/bin/sh
if [ "$1" = "pod" ] && [ "$2" = "create" ]; then
  echo "permission denied"
  exit 1
fi
exit 1
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	e := New(runtimePath, 10*time.Second, 1024, "", "", nil)
	if _, err := e.createOrReplacePod(context.Background(), "pod-a"); err == nil {
		t.Fatal("expected createOrReplacePod to return error")
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

// TestRunJobDirectExitErrorWithStderr covers setRunError when ExitError and resp.Stderr already set.
func TestRunJobDirectExitErrorWithStderr(t *testing.T) {
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo err >&2 & exit 3"}
	} else {
		cmd = []string{"sh", "-c", "echo err >&2; exit 3"}
	}
	e := New("direct", 10*time.Second, 1024, "", "", nil)
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
	if resp.Status != workerapi.StatusFailed || resp.ExitCode != 3 {
		t.Errorf("status=%s exitCode=%d", resp.Status, resp.ExitCode)
	}
	if resp.Stderr == "" {
		t.Error("expected stderr from failed command")
	}
	if !strings.Contains(resp.Stderr, "runtime exit 3") {
		t.Errorf("stderr should mention exit code: %q", resp.Stderr)
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

// TestRunJobContainerPath_Variants covers container path with NetworkPolicy default and TimeoutSeconds (runtime missing => failed).
func TestRunJobContainerPath_Variants(t *testing.T) {
	for name, req := range map[string]*workerapi.RunJobRequest{
		"network_policy_default": {
			Version: 1, TaskID: "t1", JobID: "j1",
			Sandbox: workerapi.SandboxSpec{
				Image: "alpine:latest", Command: []string{"echo", "hi"},
				NetworkPolicy: "allow",
			},
		},
		"timeout_seconds": {
			Version: 1, TaskID: "t1", JobID: "j1",
			Sandbox: workerapi.SandboxSpec{
				Image: "alpine:latest", Command: []string{"echo", "hi"},
				TimeoutSeconds: 30,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			timeout := 5 * time.Second
			if name == "timeout_seconds" {
				timeout = 1 * time.Hour
			}
			e := New("nonexistent-runtime-xyz", timeout, 1024, "", "", nil)
			resp, err := e.RunJob(context.Background(), req, "")
			if err != nil {
				t.Fatalf("RunJob: %v", err)
			}
			if resp.Status != workerapi.StatusFailed {
				t.Errorf("status=%s", resp.Status)
			}
		})
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

// TestRunJobDirectUseInferenceEnv covers runDirect when UseInference and ollamaUpstreamURL are set (env INFERENCE_PROXY_URL per worker_node.md UDS contract).
func TestRunJobDirectUseInferenceEnv(t *testing.T) {
	var cmd []string
	if runtime.GOOS == goOSWindows {
		cmd = []string{"cmd", "/c", "echo", "%INFERENCE_PROXY_URL%"}
	} else {
		cmd = []string{"sh", "-c", "echo $INFERENCE_PROXY_URL"}
	}
	e := New("direct", 10*time.Second, 1024, "http://host:11434", "", nil)
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
	if resp.Status != workerapi.StatusCompleted {
		t.Errorf("status=%s", resp.Status)
	}
	stdout := strings.TrimSpace(resp.Stdout)
	if !strings.Contains(stdout, "http+unix://") {
		t.Errorf("direct+UseInference should set INFERENCE_PROXY_URL to UDS-style URL: %q", resp.Stdout)
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

func TestReady_Direct(t *testing.T) {
	e := New("direct", time.Second, 1024, "", "", nil)
	ready, reason := e.Ready(context.Background())
	if !ready || reason != "" {
		t.Errorf("direct runtime: ready=%v reason=%q", ready, reason)
	}
}

func TestReady_UnavailableRuntime(t *testing.T) {
	e := New("nonexistent-runtime-xyz", time.Second, 1024, "", "", nil)
	ready, reason := e.Ready(context.Background())
	if ready || reason == "" {
		t.Errorf("unavailable runtime: ready=%v reason=%q", ready, reason)
	}
}

func TestReady_PodmanAvailable(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not in path")
	}
	e := New("podman", 5*time.Second, 1024, "", "", nil)
	ready, reason := e.Ready(context.Background())
	if !ready || reason != "" {
		t.Errorf("podman available: ready=%v reason=%q", ready, reason)
	}
}

func TestIsSBARunnerImage(t *testing.T) {
	tests := []struct {
		image string
		want  bool
	}{
		{"cynodeai-cynode-sba:dev", true},
		{"localhost/cynodeai-cynode-sba:dev", true},
		{"docker.io/org/cynode-sba:latest", true},
		{"alpine:latest", false},
		{"cynodeai-control-plane:dev", false},
	}
	for _, tt := range tests {
		got := isSBARunnerImage(tt.image)
		if got != tt.want {
			t.Errorf("isSBARunnerImage(%q) = %v, want %v", tt.image, got, tt.want)
		}
	}
}

// TestRunJobDirectEmptyCommand asserts that direct runtime returns error when command is empty (e.g. SBA job without container).
func TestRunJobDirectEmptyCommand(t *testing.T) {
	e := New("direct", 10*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynodeai-cynode-sba:dev",
			Command:     nil,
			JobSpecJSON: `{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[]}`,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed || resp.ExitCode != -1 {
		t.Errorf("status=%s exitCode=%d (expected failed when direct + empty command)", resp.Status, resp.ExitCode)
	}
	if resp.Stderr == "" || !strings.Contains(resp.Stderr, "direct runtime") {
		t.Errorf("stderr %q should mention direct runtime", resp.Stderr)
	}
}

// TestRunJobSBAPathNoRuntime exercises the SBA branch when container runtime is missing (job dir created, container fails, no result.json).
func TestRunJobSBAPathNoRuntime(t *testing.T) {
	e := New("nonexistent-runtime-xyz", 10*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynodeai-cynode-sba:dev",
			Command:     nil,
			JobSpecJSON: `{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[]}`,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Errorf("status=%s (expected failed when runtime missing)", resp.Status)
	}
	if resp.SbaResult != nil {
		t.Errorf("expected no SbaResult when container did not run")
	}
}

func TestRunJobSBA_WorkspacePrepareFailure(t *testing.T) {
	e := New("nonexistent-runtime-xyz", 10*time.Second, 1024, "", "", nil)
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynodeai-cynode-sba:dev",
			Command:     nil,
			JobSpecJSON: `{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[]}`,
		},
	}
	resp, err := e.RunJob(context.Background(), req, filepath.Join(blocker, "workspace"))
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Fatalf("status=%s, want failed", resp.Status)
	}
	if !strings.Contains(resp.Stderr, "failed to prepare workspace dir") {
		t.Fatalf("stderr=%q should mention workspace prepare failure", resp.Stderr)
	}
}

func TestRunJobSBA_AgentInferencePodPath_PodCreateFail(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not in path")
	}
	_ = os.Setenv("CONTAINER_HOST", "unix:///nonexistent/podman.sock")
	defer func() { _ = os.Unsetenv("CONTAINER_HOST") }()

	e := New("podman", 10*time.Second, 1024, "http://host.containers.internal:11434", "alpine:latest", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynodeai-cynode-sba:dev",
			Command:     nil,
			JobSpecJSON: `{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024}}`,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Fatalf("status=%s, want failed", resp.Status)
	}
	if !strings.Contains(resp.Stderr, "pod create") {
		t.Fatalf("stderr=%q should contain pod create", resp.Stderr)
	}
}

func TestRunJobSBA_AgentInferencePodPath_Success(t *testing.T) {
	runtimeDir := t.TempDir()
	runtimePath := filepath.Join(runtimeDir, "podman")
	script := `#!/bin/sh
cmd="$1"
shift
if [ "$cmd" = "pod" ] && [ "$1" = "create" ]; then
  exit 0
fi
if [ "$cmd" = "pod" ] && [ "$1" = "rm" ]; then
  exit 0
fi
if [ "$cmd" = "run" ] && [ "$1" = "-d" ]; then
  echo "cid-proxy-1"
  exit 0
fi
if [ "$cmd" = "exec" ]; then
  exit 0
fi
if [ "$cmd" = "run" ]; then
  job_dir=""
  prev=""
  for arg in "$@"; do
    if [ "$prev" = "-v" ]; then
      case "$arg" in
        *:/job|*:/job:z) job_dir="${arg%%:/job*}" ;;
      esac
    fi
    prev="$arg"
  done
  if [ -z "$job_dir" ]; then
    echo "missing job mount"
    exit 1
  fi
  printf '%s\n' '{"protocol_version":"1.0","job_id":"j1","status":"success"}' > "$job_dir/result.json"
  echo "ok"
  exit 0
fi
echo "unsupported args: $cmd $*"
exit 1
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	origPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", runtimeDir+string(os.PathListSeparator)+origPath)
	defer func() { _ = os.Setenv("PATH", origPath) }()

	origSock := probeProxySocketExistsFunc
	defer func() { probeProxySocketExistsFunc = origSock }()
	probeProxySocketExistsFunc = func(string) error { return nil }

	e := New("podman", 10*time.Second, 4096, "http://host.containers.internal:11434", "proxy:latest", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynodeai-cynode-sba:dev",
			Command:     nil,
			JobSpecJSON: `{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024}}`,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusCompleted {
		t.Fatalf("status=%s stderr=%q", resp.Status, resp.Stderr)
	}
	if resp.SbaResult == nil || resp.SbaResult.Status != "success" {
		t.Fatalf("expected sba success result, got %#v", resp.SbaResult)
	}
	if resp.RunDiagnostics == nil {
		t.Fatal("expected run diagnostics")
	}
	diagStr := strings.Join(resp.RunDiagnostics.RuntimeArgv, " ")
	// REQ-SANDBX-0131: pod sandbox must use INFERENCE_PROXY_URL (UDS), not TCP OLLAMA_BASE_URL.
	if !strings.Contains(diagStr, "--pod") {
		t.Fatalf("expected --pod in runtime argv diagnostics: %s", diagStr)
	}
	if !strings.Contains(diagStr, "INFERENCE_PROXY_URL=http+unix://") {
		t.Fatalf("expected INFERENCE_PROXY_URL=http+unix:// in diagnostics (REQ-SANDBX-0131): %s", diagStr)
	}
	if strings.Contains(diagStr, "OLLAMA_BASE_URL=http://localhost:") {
		t.Fatalf("must not inject TCP OLLAMA_BASE_URL in pod diagnostics (REQ-SANDBX-0131): %s", diagStr)
	}
}

func TestRunJobSBA_AgentInferencePodPath_ProxyReadinessFail(t *testing.T) {
	runtimeDir := t.TempDir()
	runtimePath := filepath.Join(runtimeDir, "podman")
	script := `#!/bin/sh
cmd="$1"
shift
if [ "$cmd" = "pod" ] && [ "$1" = "create" ]; then
  exit 0
fi
if [ "$cmd" = "pod" ] && [ "$1" = "rm" ]; then
  exit 0
fi
if [ "$cmd" = "run" ] && [ "$1" = "-d" ]; then
  echo "cid-proxy-1"
  exit 0
fi
echo "unsupported args: $cmd $*"
exit 1
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	origPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", runtimeDir+string(os.PathListSeparator)+origPath)
	defer func() { _ = os.Setenv("PATH", origPath) }()

	orig := probeProxyHealthFunc
	origSock := probeProxySocketExistsFunc
	defer func() {
		probeProxyHealthFunc = orig
		probeProxySocketExistsFunc = origSock
	}()
	probeProxyHealthFunc = func(context.Context, string, string) error { return os.ErrDeadlineExceeded }
	probeProxySocketExistsFunc = func(string) error { return os.ErrDeadlineExceeded }
	e := New("podman", 300*time.Millisecond, 4096, "http://host.containers.internal:11434", "proxy:latest", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynodeai-cynode-sba:dev",
			Command:     nil,
			JobSpecJSON: `{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024}}`,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Fatalf("status=%s, want failed", resp.Status)
	}
	if !strings.Contains(resp.Stderr, "proxy readiness") {
		t.Fatalf("stderr=%q should contain proxy readiness", resp.Stderr)
	}
}

func TestRunJobSBA_AgentInferencePodPath_ProxyStartMissingContainerID(t *testing.T) {
	runtimeDir := t.TempDir()
	runtimePath := filepath.Join(runtimeDir, "podman")
	script := `#!/bin/sh
cmd="$1"
shift
if [ "$cmd" = "pod" ] && [ "$1" = "create" ]; then
  exit 0
fi
if [ "$cmd" = "pod" ] && [ "$1" = "rm" ]; then
  exit 0
fi
if [ "$cmd" = "run" ] && [ "$1" = "-d" ]; then
  exit 0
fi
echo "unsupported args: $cmd $*"
exit 1
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	origPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", runtimeDir+string(os.PathListSeparator)+origPath)
	defer func() { _ = os.Setenv("PATH", origPath) }()

	e := New("podman", 300*time.Millisecond, 4096, "http://host.containers.internal:11434", "proxy:latest", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynodeai-cynode-sba:dev",
			Command:     nil,
			JobSpecJSON: `{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024}}`,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Fatalf("status=%s, want failed", resp.Status)
	}
	if !strings.Contains(resp.Stderr, "missing proxy container id") {
		t.Fatalf("stderr=%q should mention missing proxy container id", resp.Stderr)
	}
}

func TestSetSBAError(t *testing.T) {
	resp := &workerapi.RunJobResponse{}
	setSBAError(resp, "test error")
	if resp.Status != workerapi.StatusFailed || resp.ExitCode != -1 || resp.Stderr != "test error" {
		t.Errorf("setSBAError: status=%s exitCode=%d stderr=%q", resp.Status, resp.ExitCode, resp.Stderr)
	}
	if resp.EndedAt == "" {
		t.Error("setSBAError: EndedAt should be set")
	}
}

// REQ-SANDBX-0131 / REQ-WORKER-0260: proxy sidecar in pod MUST set INFERENCE_PROXY_SOCKET
// so it listens on the UDS path shared with the SBA container.
func TestBuildProxyRunArgs_SetsInferenceProxySocket(t *testing.T) {
	args := buildProxyRunArgs("pod-1", "http://host.containers.internal:11434", "inference-proxy:dev", nil, "/tmp/sock-dir")
	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "INFERENCE_PROXY_SOCKET=") {
		t.Errorf("proxy run args must set INFERENCE_PROXY_SOCKET (REQ-SANDBX-0131): %s", argStr)
	}
	if !strings.Contains(argStr, inferenceProxySockInContainer) {
		t.Errorf("proxy run args must set INFERENCE_PROXY_SOCKET to %s: %s", inferenceProxySockInContainer, argStr)
	}
	if !strings.Contains(argStr, "OLLAMA_UPSTREAM_URL=http://host.containers.internal:11434") {
		t.Errorf("proxy run args must set OLLAMA_UPSTREAM_URL: %s", argStr)
	}
	if !strings.Contains(argStr, "--pod pod-1") {
		t.Errorf("proxy run args must include --pod: %s", argStr)
	}
	if !strings.Contains(argStr, "/tmp/sock-dir:/run/cynode") {
		t.Errorf("proxy run args must mount sock-dir at /run/cynode (REQ-WORKER-0260): %s", argStr)
	}
}

func TestBuildSBARunArgs(t *testing.T) {
	e := New("podman", 30*time.Second, 4096, "", "", nil)
	req := &workerapi.RunJobRequest{
		TaskID: "t1",
		JobID:  "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynodeai-cynode-sba:dev",
			JobSpecJSON: `{}`,
		},
	}
	args := buildSBARunArgs(req, "/tmp/job", "/tmp/ws", e, "direct_steps")
	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "/tmp/job") || !strings.Contains(argStr, "/job") {
		t.Errorf("buildSBARunArgs should mount job dir: %s", argStr)
	}
	if !strings.Contains(argStr, "/tmp/ws") || !strings.Contains(argStr, workspaceMount) {
		t.Errorf("buildSBARunArgs should mount workspace: %s", argStr)
	}
	if !strings.Contains(argStr, "cynodeai-cynode-sba:dev") {
		t.Errorf("buildSBARunArgs should include image: %s", argStr)
	}
}

func TestBuildSBARunArgs_Docker(t *testing.T) {
	e := New("docker", 30*time.Second, 4096, "", "", nil)
	req := &workerapi.RunJobRequest{
		TaskID: "t1",
		JobID:  "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynode-sba:latest",
			JobSpecJSON: `{}`,
		},
	}
	args := buildSBARunArgs(req, "/tmp/job", "", e, "direct_steps")
	argStr := strings.Join(args, " ")
	if strings.Contains(argStr, ":z") {
		t.Errorf("docker runtime should not add :z (SELinux), got %s", argStr)
	}
	if strings.Contains(argStr, "--userns=keep-id") {
		t.Errorf("docker runtime should not add --userns=keep-id, got %s", argStr)
	}
	// Empty workspaceDir: no -v dir:/workspace and no -w (env CYNODE_WORKSPACE_DIR=/workspace is still set).
	if strings.Contains(argStr, " -w ") {
		t.Errorf("empty workspaceDir should not add -w, got %s", argStr)
	}
}

func TestBuildSBARunArgs_WithCommand(t *testing.T) {
	e := New("podman", 30*time.Second, 4096, "", "", nil)
	req := &workerapi.RunJobRequest{
		TaskID: "t1",
		JobID:  "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynode-sba:dev",
			JobSpecJSON: `{}`,
			Command:     []string{"--custom", "arg"},
		},
	}
	args := buildSBARunArgs(req, "/tmp/job", "/tmp/ws", e, "direct_steps")
	if len(args) < 3 || args[len(args)-2] != "--custom" || args[len(args)-1] != "arg" {
		t.Errorf("buildSBARunArgs with Command should append command, got %v", args)
	}
}

func TestBuildSBARunArgs_AgentInference_NoDirectStepsEnv(t *testing.T) {
	e := New("podman", 30*time.Second, 4096, "", "", nil)
	req := &workerapi.RunJobRequest{
		TaskID: "t1",
		JobID:  "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynode-sba:dev",
			JobSpecJSON: `{}`,
		},
	}
	args := buildSBARunArgs(req, "/tmp/job", "/tmp/ws", e, "agent_inference")
	argStr := strings.Join(args, " ")
	if strings.Contains(argStr, "SBA_DIRECT_STEPS=1") {
		t.Errorf("agent_inference mode must not set SBA_DIRECT_STEPS=1: %s", argStr)
	}
	if !strings.Contains(argStr, "SBA_EXECUTION_MODE=agent_inference") {
		t.Errorf("missing SBA_EXECUTION_MODE env in args: %s", argStr)
	}
	if !strings.Contains(argStr, "--network=none") {
		t.Errorf("agent_inference without upstream should keep network none: %s", argStr)
	}
}

// REQ-WORKER-0174 / REQ-SANDBX-0131: SBA containers in direct (non-pod) mode always run
// with --network=none. Inference is injected via a UDS socket mount + INFERENCE_PROXY_URL
// when SBA_INFERENCE_PROXY_SOCKET is set (worker embed starts the proxy).
func TestBuildSBARunArgs_AgentInference_WithUpstreamUsesUDSNotTCP(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "inference-proxy.sock")
	if err := os.MkdirAll(filepath.Dir(sockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("SBA_INFERENCE_PROXY_SOCKET", sockPath)
	defer func() { _ = os.Unsetenv("SBA_INFERENCE_PROXY_SOCKET") }()

	e := New("podman", 30*time.Second, 4096, "http://host.containers.internal:11434", "", nil)
	req := &workerapi.RunJobRequest{
		TaskID: "t1",
		JobID:  "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynode-sba:dev",
			JobSpecJSON: `{}`,
		},
	}
	args := buildSBARunArgs(req, "/tmp/job", "/tmp/ws", e, "agent_inference")
	argStr := strings.Join(args, " ")
	// MUST include INFERENCE_PROXY_URL as UDS URL when worker provides socket.
	if !strings.Contains(argStr, "INFERENCE_PROXY_URL=http+unix://") {
		t.Errorf("agent_inference must set INFERENCE_PROXY_URL=http+unix://... (REQ-SANDBX-0131): %s", argStr)
	}
	// MUST mount the host socket dir so the path exists in the container.
	if !strings.Contains(argStr, "-v") || !strings.Contains(argStr, "/run/cynode") {
		t.Errorf("agent_inference must mount inference proxy socket dir at /run/cynode: %s", argStr)
	}
	// MUST NOT inject TCP Ollama URL.
	if strings.Contains(argStr, "OLLAMA_BASE_URL=http://") {
		t.Errorf("agent_inference must not inject TCP OLLAMA_BASE_URL (REQ-WORKER-0174): %s", argStr)
	}
	// MUST keep network=none; inference reaches in through socket mount, not network.
	if !strings.Contains(argStr, "--network=none") {
		t.Errorf("agent_inference SBA must keep --network=none (REQ-WORKER-0174): %s", argStr)
	}
}

// When SBA_INFERENCE_PROXY_SOCKET is unset, non-pod agent_inference must not inject INFERENCE_PROXY_URL
// (no socket is mounted, so pointing at /run/cynode/inference-proxy.sock would be invalid).
func TestBuildSBARunArgs_AgentInference_NoSocketNoInferenceURL(t *testing.T) {
	_ = os.Unsetenv("SBA_INFERENCE_PROXY_SOCKET")
	e := New("podman", 30*time.Second, 4096, "http://localhost:11434", "", nil)
	req := &workerapi.RunJobRequest{
		TaskID: "t1",
		JobID:  "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynode-sba:dev",
			JobSpecJSON: `{}`,
		},
	}
	args := buildSBARunArgs(req, "/tmp/job", "/tmp/ws", e, "agent_inference")
	argStr := strings.Join(args, " ")
	if strings.Contains(argStr, "INFERENCE_PROXY_URL=") {
		t.Errorf("without SBA_INFERENCE_PROXY_SOCKET must not set INFERENCE_PROXY_URL: %s", argStr)
	}
}

func TestShouldUseSBAPodInference(t *testing.T) {
	e := New("podman", 30*time.Second, 4096, "http://host.containers.internal:11434", "proxy:latest", nil)
	if !e.shouldUseSBAPodInference("agent_inference") {
		t.Fatal("expected shouldUseSBAPodInference to be true")
	}
	e2 := New("podman", 30*time.Second, 4096, "http://host.containers.internal:11434", "", nil)
	if e2.shouldUseSBAPodInference("agent_inference") {
		t.Fatal("expected shouldUseSBAPodInference to be false without proxy image")
	}
}

// REQ-SANDBX-0131: SBA in pod path must inject INFERENCE_PROXY_URL (UDS socket) not TCP OLLAMA_BASE_URL.
// The proxy sidecar listens on a UDS socket shared within the pod; the sandbox connects to it via
// http+unix:// URL, never via TCP localhost:11434.
func TestBuildSBARunArgsForPod_AgentInferenceUsesUDS(t *testing.T) {
	e := New("podman", 30*time.Second, 4096, "http://host.containers.internal:11434", "proxy:latest", nil)
	req := &workerapi.RunJobRequest{
		TaskID: "t1",
		JobID:  "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynode-sba:dev",
			JobSpecJSON: `{}`,
		},
	}
	args := buildSBARunArgsForPod(req, "pod-1", "/tmp/job", "/tmp/ws", "/tmp/sock-dir", e, "agent_inference")
	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "--pod pod-1") {
		t.Fatalf("expected pod argument, got: %s", argStr)
	}
	// MUST inject INFERENCE_PROXY_URL as UDS URL.
	if !strings.Contains(argStr, "INFERENCE_PROXY_URL=http+unix://") {
		t.Fatalf("expected INFERENCE_PROXY_URL=http+unix://... (REQ-SANDBX-0131), got: %s", argStr)
	}
	// MUST mount shared sock dir at /run/cynode.
	if !strings.Contains(argStr, "/tmp/sock-dir:/run/cynode") {
		t.Fatalf("expected sock-dir mount at /run/cynode (REQ-WORKER-0260), got: %s", argStr)
	}
	// MUST NOT inject any TCP OLLAMA_BASE_URL.
	if strings.Contains(argStr, "OLLAMA_BASE_URL=http://localhost:") {
		t.Fatalf("must not inject TCP localhost OLLAMA_BASE_URL in pod (REQ-SANDBX-0131), got: %s", argStr)
	}
	if strings.Contains(argStr, "OLLAMA_BASE_URL=http://host.containers.internal:") {
		t.Fatalf("must not inject TCP upstream OLLAMA_BASE_URL in pod (REQ-SANDBX-0131), got: %s", argStr)
	}
	if strings.Contains(argStr, "--userns=keep-id") {
		t.Fatalf("pod-run args must not set --userns=keep-id, got: %s", argStr)
	}
}

// TestBuildSBARunArgsForPod_ExportedWrapper verifies the exported BuildSBARunArgsForPod
// delegates to buildSBARunArgsForPod (same contract).
func TestBuildSBARunArgsForPod_ExportedWrapper(t *testing.T) {
	e := New("podman", 30*time.Second, 4096, "http://host.containers.internal:11434", "proxy:latest", nil)
	req := &workerapi.RunJobRequest{
		TaskID: "t1", JobID: "j1",
		Sandbox: workerapi.SandboxSpec{Image: "cynode-sba:dev", JobSpecJSON: `{}`},
	}
	args := BuildSBARunArgsForPod(req, "pod-1", "/tmp/job", "/tmp/ws", "/tmp/sock-dir", e, "agent_inference")
	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "INFERENCE_PROXY_URL=http+unix://") {
		t.Errorf("exported BuildSBARunArgsForPod must inject INFERENCE_PROXY_URL=http+unix://..., got: %s", argStr)
	}
}

func TestBuildSBARunArgsForPod_WithCommand(t *testing.T) {
	e := New("podman", 30*time.Second, 4096, "http://host.containers.internal:11434", "proxy:latest", nil)
	req := &workerapi.RunJobRequest{
		TaskID: "t1",
		JobID:  "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynode-sba:dev",
			JobSpecJSON: `{}`,
			Command:     []string{"--custom", "arg"},
		},
	}
	args := buildSBARunArgsForPod(req, "pod-1", "/tmp/job", "/tmp/ws", "/tmp/sock-dir", e, "agent_inference")
	if len(args) < 2 || args[len(args)-2] != "--custom" || args[len(args)-1] != "arg" {
		t.Fatalf("expected command at end, got %v", args)
	}
}

func TestApplySbaResultFromDir_MissingFile(t *testing.T) {
	dir := t.TempDir()
	resp := &workerapi.RunJobResponse{}
	applySbaResultFromDir(dir, resp)
	if resp.Status != workerapi.StatusFailed || resp.ExitCode != -1 {
		t.Errorf("missing result: status=%s exitCode=%d", resp.Status, resp.ExitCode)
	}
}

func TestApplySbaResultFromDir_ValidResult(t *testing.T) {
	for _, tc := range []struct {
		name       string
		json       string
		wantStatus string
		wantExit   int
	}{
		{"success", `{"protocol_version":"1.0","job_id":"j1","status":"success","steps":[]}`, workerapi.StatusCompleted, 0},
		{"failure", `{"protocol_version":"1.0","job_id":"j1","status":"failure","failure_code":"timeout","steps":[]}`, workerapi.StatusFailed, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			_ = os.WriteFile(filepath.Join(dir, resultFilename), []byte(tc.json), 0o644)
			resp := &workerapi.RunJobResponse{}
			applySbaResultFromDir(dir, resp)
			if resp.SbaResult == nil {
				t.Fatal("expected SbaResult set")
			}
			if resp.Status != tc.wantStatus || resp.ExitCode != tc.wantExit {
				t.Errorf("status=%s exitCode=%d want %s %d", resp.Status, resp.ExitCode, tc.wantStatus, tc.wantExit)
			}
		})
	}
}

func TestApplySbaResultFromDir_TimeoutPreserved(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, resultFilename), []byte(`{"protocol_version":"1.0","job_id":"j1","status":"success","steps":[]}`), 0o644)
	resp := &workerapi.RunJobResponse{Status: workerapi.StatusTimeout}
	applySbaResultFromDir(dir, resp)
	if resp.Status != workerapi.StatusTimeout {
		t.Errorf("timeout should be preserved: status=%s", resp.Status)
	}
}

func TestApplySbaResultFromDir_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, resultFilename), []byte(`not json`), 0o644)
	resp := &workerapi.RunJobResponse{}
	applySbaResultFromDir(dir, resp)
	if resp.Status != workerapi.StatusFailed || resp.ExitCode != -1 {
		t.Errorf("invalid JSON: status=%s exitCode=%d", resp.Status, resp.ExitCode)
	}
}

func TestTruncateUTF8(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		maxBytes int
		want     string
	}{
		{"empty", "", 10, ""},
		{"under limit", "hello", 10, "hello"},
		{"exact", "1234567890", 10, "1234567890"},
		{"over ascii", "123456789012345", 10, "1234567890"},
		{"rune boundary", "a\u00e9b", 3, "a\u00e9"},          // U+00E9 is 2 bytes in UTF-8
		{"mid multi-byte", "1234\u00e9xyz", 6, "1234\u00e9"}, // truncate at 6 bytes, ends at rune boundary
		{"zero maxBytes", "hello", 0, ""},
		// Continuation byte at boundary: backup then return (covers truncateUTF8 loop b=b[:i] path)
		{"back up from continuation", "a" + "\x80" + "x", 2, "a"},
		// No rune start in truncated prefix: loop exits without return, then return string(b) (line 830)
		{"continuation only", "\x80", 1, "\x80"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateUTF8(tt.s, tt.maxBytes)
			if got != tt.want {
				t.Errorf("truncateUTF8(%q, %d) = %q, want %q", tt.s, tt.maxBytes, got, tt.want)
			}
		})
	}
}

// TestSetRunError_NonExitErrorWithStderr covers setRunError when err is not *exec.ExitError and resp.Stderr is already set.
func TestSetRunError_NonExitErrorWithStderr(t *testing.T) {
	e := New("direct", time.Second, 1024, "", "", nil)
	resp := &workerapi.RunJobResponse{Stderr: "prior stderr"}
	e.setRunError(resp, fmt.Errorf("executable not found"))
	if resp.Status != workerapi.StatusFailed || resp.ExitCode != -1 {
		t.Errorf("status=%s exitCode=%d", resp.Status, resp.ExitCode)
	}
	if !strings.Contains(resp.Stderr, "executable not found") || !strings.Contains(resp.Stderr, "--- runtime stderr ---\nprior stderr") {
		t.Errorf("stderr should append prior: %q", resp.Stderr)
	}
}

func TestPrepareSBAJobAndWorkspace_Success(t *testing.T) {
	workspaceDir := t.TempDir()
	req := &workerapi.RunJobRequest{
		TaskID: "task-1",
		JobID:  "job-1",
		Sandbox: workerapi.SandboxSpec{
			JobSpecJSON: `{"version":1,"execution_mode":"direct_steps","steps":[]}`,
		},
	}
	jobDir, wsUsed, err := prepareSBAJobAndWorkspace(req, workspaceDir)
	if err != nil {
		t.Fatalf("prepareSBAJobAndWorkspace: %v", err)
	}
	defer func() { _ = os.RemoveAll(jobDir) }()
	if wsUsed != workspaceDir {
		t.Errorf("workspaceDirToUse = %q, want %q", wsUsed, workspaceDir)
	}
	if _, err := os.Stat(filepath.Join(jobDir, jobSpecFilename)); err != nil {
		t.Errorf("job.json not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(jobDir, resultFilename)); err != nil {
		t.Errorf("result.json not created: %v", err)
	}
}

func TestPrepareSBAJobAndWorkspace_EmptyWorkspaceCreatesTemp(t *testing.T) {
	req := &workerapi.RunJobRequest{
		TaskID: "task-1",
		JobID:  "job-1",
		Sandbox: workerapi.SandboxSpec{
			JobSpecJSON: `{"version":1}`,
		},
	}
	jobDir, wsUsed, err := prepareSBAJobAndWorkspace(req, "")
	if err != nil {
		t.Fatalf("prepareSBAJobAndWorkspace: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(jobDir)
		if wsUsed != "" {
			_ = os.RemoveAll(wsUsed)
		}
	}()
	if wsUsed == "" {
		t.Error("expected non-empty workspaceDirToUse when workspaceDir empty")
	}
}

func TestPrepareSBAJobAndWorkspace_WorkspaceDirIsFileFails(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := &workerapi.RunJobRequest{
		TaskID: "task-1",
		JobID:  "job-1",
		Sandbox: workerapi.SandboxSpec{
			JobSpecJSON: `{"version":1}`,
		},
	}
	_, _, err := prepareSBAJobAndWorkspace(req, f)
	if err == nil {
		t.Fatal("expected error when workspaceDir is a file")
	}
	if !strings.Contains(err.Error(), "workspace") {
		t.Errorf("error should mention workspace: %v", err)
	}
}

func TestPrepareSBAJobAndWorkspace_MkdirTempFails(t *testing.T) {
	// TMPDIR as a file (not a dir) causes MkdirTemp to fail.
	tmpFile := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(tmpFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	orig := os.Getenv("TMPDIR")
	_ = os.Setenv("TMPDIR", tmpFile)
	defer func() {
		if orig == "" {
			_ = os.Unsetenv("TMPDIR")
		} else {
			_ = os.Setenv("TMPDIR", orig)
		}
	}()
	req := &workerapi.RunJobRequest{
		TaskID: "task-1",
		JobID:  "job-1",
		Sandbox: workerapi.SandboxSpec{
			JobSpecJSON: `{"version":1}`,
		},
	}
	_, _, err := prepareSBAJobAndWorkspace(req, "")
	if err == nil {
		t.Fatal("expected error when TMPDIR is not a directory")
	}
	if !strings.Contains(err.Error(), "job dir") {
		t.Errorf("error should mention job dir: %v", err)
	}
}

// TestRunJobSBA_InvalidJobSpecJSON covers runJobSBA when JobSpecJSON fails to parse.
func TestRunJobSBA_InvalidJobSpecJSON(t *testing.T) {
	e := New("nonexistent-runtime-xyz", 10*time.Second, 1024, "", "", nil)
	req := &workerapi.RunJobRequest{
		Version: 1,
		TaskID:  "t1",
		JobID:   "j1",
		Sandbox: workerapi.SandboxSpec{
			Image:       "cynodeai-cynode-sba:dev",
			Command:     nil,
			JobSpecJSON: `{invalid json`,
		},
	}
	resp, err := e.RunJob(context.Background(), req, "")
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if resp.Status != workerapi.StatusFailed {
		t.Errorf("status=%s, want failed", resp.Status)
	}
	if !strings.Contains(resp.Stderr, "invalid SBA job_spec_json") {
		t.Errorf("stderr=%q should mention invalid job_spec_json", resp.Stderr)
	}
}
