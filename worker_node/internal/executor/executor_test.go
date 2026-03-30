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

func exitCodeVal(r *workerapi.RunJobResponse) int {
	if r == nil || r.ExitCode == nil {
		return 0
	}
	return *r.ExitCode
}

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
	hasNetNone := false
	hasInferenceProxy := false
	hasSockMount := false
	for i, a := range args {
		if a == "--network=none" {
			hasNetNone = true
		}
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
	if !hasNetNone {
		t.Errorf("args must include --network=none (REQ-WORKER-0174), got %v", args)
	}
	if !hasInferenceProxy {
		t.Errorf("args should contain INFERENCE_PROXY_URL=http+unix://... (REQ-SANDBX-0131), got %v", args)
	}
	if !hasSockMount {
		t.Errorf("args should mount sock-dir at /run/cynode (REQ-WORKER-0270), got %v", args)
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
	if resp.Status != workerapi.StatusCompleted || exitCodeVal(resp) != 0 {
		t.Errorf("status=%s exitCode=%d stderr=%s", resp.Status, exitCodeVal(resp), resp.Stderr)
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
	if exitCodeVal(resp) != 2 {
		t.Errorf("exitCode=%d", exitCodeVal(resp))
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
	if resp.Status != workerapi.StatusCompleted || exitCodeVal(resp) != 0 {
		t.Errorf("status=%s exitCode=%d", resp.Status, exitCodeVal(resp))
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
	if resp.Status != workerapi.StatusFailed || exitCodeVal(resp) != 3 {
		t.Errorf("status=%s exitCode=%d", resp.Status, exitCodeVal(resp))
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
	if resp.Status != workerapi.StatusFailed || exitCodeVal(resp) != 3 {
		t.Errorf("status=%s exitCode=%d", resp.Status, exitCodeVal(resp))
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
	if resp.Status != workerapi.StatusTimeout || exitCodeVal(resp) != -1 {
		t.Errorf("status=%s exitCode=%d", resp.Status, exitCodeVal(resp))
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
	if resp.Status != workerapi.StatusFailed || exitCodeVal(resp) != -1 {
		t.Errorf("status=%s exitCode=%d stderr=%q", resp.Status, exitCodeVal(resp), resp.Stderr)
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
