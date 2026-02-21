// Package executor runs sandbox jobs using a container runtime.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
)

// Task context env keys (sandbox_container.md). Must not contain orchestrator secrets.
const (
	envTaskID          = "CYNODE_TASK_ID"
	envJobID           = "CYNODE_JOB_ID"
	envWorkspaceDir    = "CYNODE_WORKSPACE_DIR"
	envOllamaBaseURL   = "OLLAMA_BASE_URL"
	workspaceMount     = "/workspace"
	ollamaBaseURLInPod = "http://localhost:11434"
)

// Executor executes sandbox jobs.
type Executor struct {
	runtime                 string   // docker or podman
	defaultTimeout          time.Duration
	maxOutputBytes          int
	ollamaUpstreamURL       string   // when set with inferenceProxyImage, jobs with UseInference run in pod with proxy
	inferenceProxyImage     string
	inferenceProxyCommand   []string // optional; when set, appended to proxy container run (e.g. ["sleep","60"] for tests)
}

// New creates a new job executor. ollamaUpstreamURL and inferenceProxyImage are optional;
// when both are set, jobs with Sandbox.UseInference run in a pod with an inference proxy sidecar.
// inferenceProxyCommand is optional; when non-nil, appended to the proxy container run (for testing with a placeholder image).
func New(runtime string, defaultTimeout time.Duration, maxOutputBytes int, ollamaUpstreamURL, inferenceProxyImage string, inferenceProxyCommand []string) *Executor {
	return &Executor{
		runtime:               runtime,
		defaultTimeout:        defaultTimeout,
		maxOutputBytes:        maxOutputBytes,
		ollamaUpstreamURL:     ollamaUpstreamURL,
		inferenceProxyImage:   inferenceProxyImage,
		inferenceProxyCommand: inferenceProxyCommand,
	}
}

// Ready reports whether the executor can accept job requests (container runtime available).
// Used for GET /readyz. For "direct" runtime always returns true.
func (e *Executor) Ready(ctx context.Context) (ready bool, reason string) {
	if e.runtime == "direct" {
		return true, ""
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.runtime, "info")
	if _, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Sprintf("%s not available: %v", e.runtime, err)
	}
	return true, ""
}

// RunJob executes a sandbox job and returns the result.
// workspaceDir is the host path for the per-task workspace; if non-empty it is mounted at /workspace
// and task context env vars are set. See docs/tech_specs/sandbox_container.md.
func (e *Executor) RunJob(ctx context.Context, req *workerapi.RunJobRequest, workspaceDir string) (*workerapi.RunJobResponse, error) {
	startedAt := time.Now().UTC()

	resp := &workerapi.RunJobResponse{
		Version:   1,
		TaskID:    req.TaskID,
		JobID:     req.JobID,
		StartedAt: startedAt.Format(time.RFC3339),
	}

	timeout := e.defaultTimeout
	if req.Sandbox.TimeoutSeconds > 0 {
		timeout = time.Duration(req.Sandbox.TimeoutSeconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	image := req.Sandbox.Image
	if image == "" {
		image = workerapi.DefaultImage
	}

	// "direct" executes the command in-process (inside the worker-api container).
	// This is useful for containerized dev environments where running podman-in-podman
	// is undesirable. Production deployments SHOULD use a real container runtime.
	if e.runtime == "direct" {
		return e.runDirect(ctx, req, resp, workspaceDir)
	}

	// When UseInference is set and node has proxy image + Ollama URL, run in pod with inference proxy (node.md Option A).
	if req.Sandbox.UseInference && e.ollamaUpstreamURL != "" && e.inferenceProxyImage != "" && e.runtime == "podman" {
		return e.runJobWithPodInference(ctx, req, resp, workspaceDir)
	}

	// Build container run command.
	args := []string{"run", "--rm"}

	// Phase 1: none and restricted both mean deny-all (worker_api.md).
	switch strings.ToLower(strings.TrimSpace(req.Sandbox.NetworkPolicy)) {
	case "none", "restricted", "":
		args = append(args, "--network=none")
	default:
		args = append(args, "--network=none")
	}

	if workspaceDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s", workspaceDir, workspaceMount), "-w", workspaceMount)
	}

	env := e.buildTaskEnv(req, workspaceMount)
	for k, v := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Add labels for tracking.
	args = append(args,
		"--label", fmt.Sprintf("cynodeai.task_id=%s", req.TaskID),
		"--label", fmt.Sprintf("cynodeai.job_id=%s", req.JobID),
		image,
	)
	args = append(args, req.Sandbox.Command...)

	cmd := exec.CommandContext(ctx, e.runtime, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	endedAt := time.Now().UTC()
	resp.EndedAt = endedAt.Format(time.RFC3339)

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	resp.Truncated.Stdout = len(stdoutStr) > e.maxOutputBytes
	resp.Truncated.Stderr = len(stderrStr) > e.maxOutputBytes
	stdoutStr = truncateUTF8(stdoutStr, e.maxOutputBytes)
	stderrStr = truncateUTF8(stderrStr, e.maxOutputBytes)

	resp.Stdout = stdoutStr
	resp.Stderr = stderrStr

	if ctx.Err() == context.DeadlineExceeded {
		resp.Status = workerapi.StatusTimeout
		resp.ExitCode = -1
		return resp, nil
	}

	if err != nil {
		e.setRunError(resp, err)
		return resp, nil
	}

	resp.Status = workerapi.StatusCompleted
	resp.ExitCode = 0
	return resp, nil
}

// runJobWithPodInference runs the job in a Podman pod with an inference proxy sidecar;
// sandbox gets OLLAMA_BASE_URL=http://localhost:11434. Per node.md Option A.
func (e *Executor) runJobWithPodInference(ctx context.Context, req *workerapi.RunJobRequest, resp *workerapi.RunJobResponse, workspaceDir string) (*workerapi.RunJobResponse, error) {
	podName := "cynodeai-job-" + sanitizePodName(req.JobID)
	createPod := exec.CommandContext(ctx, e.runtime, "pod", "create", "--name", podName)
	if out, err := createPod.CombinedOutput(); err != nil {
		setPodInferenceError(resp, "pod create", out)
		return resp, nil
	}
	cleanupCtx := context.WithoutCancel(ctx)
	defer func() { _ = exec.CommandContext(cleanupCtx, e.runtime, "pod", "rm", "-f", podName).Run() }()

	proxyArgs := buildProxyRunArgs(podName, e.ollamaUpstreamURL, e.inferenceProxyImage, e.inferenceProxyCommand)
	runProxy := exec.CommandContext(ctx, e.runtime, proxyArgs...)
	if out, err := runProxy.CombinedOutput(); err != nil {
		setPodInferenceError(resp, "proxy start", out)
		return resp, nil
	}

	time.Sleep(2 * time.Second)

	image := req.Sandbox.Image
	if image == "" {
		image = workerapi.DefaultImage
	}
	env := e.buildTaskEnv(req, workspaceMount)
	env[envOllamaBaseURL] = ollamaBaseURLInPod
	sandboxArgs := buildSandboxRunArgsForPod(req, podName, workspaceDir, env, image)

	cmd := exec.CommandContext(ctx, e.runtime, sandboxArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	endedAt := time.Now().UTC()
	resp.EndedAt = endedAt.Format(time.RFC3339)

	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	resp.Truncated.Stdout = len(stdoutStr) > e.maxOutputBytes
	resp.Truncated.Stderr = len(stderrStr) > e.maxOutputBytes
	stdoutStr = truncateUTF8(stdoutStr, e.maxOutputBytes)
	stderrStr = truncateUTF8(stderrStr, e.maxOutputBytes)
	resp.Stdout = stdoutStr
	resp.Stderr = stderrStr

	if ctx.Err() == context.DeadlineExceeded {
		resp.Status = workerapi.StatusTimeout
		resp.ExitCode = -1
		return resp, nil
	}
	if err != nil {
		e.setRunError(resp, err)
		return resp, nil
	}
	resp.Status = workerapi.StatusCompleted
	resp.ExitCode = 0
	return resp, nil
}

func setPodInferenceError(resp *workerapi.RunJobResponse, prefix string, out []byte) {
	resp.EndedAt = time.Now().UTC().Format(time.RFC3339)
	resp.Status = workerapi.StatusFailed
	resp.ExitCode = -1
	resp.Stderr = prefix + ": " + strings.TrimSpace(string(out))
}

// buildProxyRunArgs returns the argv for running the inference proxy container in the pod.
func buildProxyRunArgs(podName, ollamaUpstreamURL, image string, command []string) []string {
	args := []string{"run", "-d", "--rm", "--pod", podName,
		"-e", "OLLAMA_UPSTREAM_URL=" + ollamaUpstreamURL,
		image,
	}
	if len(command) > 0 {
		args = append(args, command...)
	}
	return args
}

// buildSandboxRunArgsForPod returns the argv for running the sandbox container in the pod (with OLLAMA_BASE_URL set).
func buildSandboxRunArgsForPod(req *workerapi.RunJobRequest, podName, workspaceDir string, env map[string]string, image string) []string {
	args := []string{"run", "--rm", "--pod", podName}
	if workspaceDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s", workspaceDir, workspaceMount), "-w", workspaceMount)
	}
	for k, v := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args,
		"--label", fmt.Sprintf("cynodeai.task_id=%s", req.TaskID),
		"--label", fmt.Sprintf("cynodeai.job_id=%s", req.JobID),
		image,
	)
	return append(args, req.Sandbox.Command...)
}

func sanitizePodName(jobID string) string {
	var b strings.Builder
	for _, r := range jobID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	s := b.String()
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// buildTaskEnv returns env for the sandbox: task context first, then request env.
// Request env must not override CYNODE_* (no orchestrator secrets in sandbox).
func (e *Executor) buildTaskEnv(req *workerapi.RunJobRequest, workspaceDirValue string) map[string]string {
	out := map[string]string{
		envTaskID:       req.TaskID,
		envJobID:        req.JobID,
		envWorkspaceDir: workspaceDirValue,
	}
	for k, v := range req.Sandbox.Env {
		if strings.HasPrefix(k, "CYNODE_") {
			continue
		}
		out[k] = v
	}
	return out
}

// setRunError sets resp status/exit/stderr from an execution error.
func (e *Executor) setRunError(resp *workerapi.RunJobResponse, err error) {
	resp.Status = workerapi.StatusFailed
	if exitErr, ok := err.(*exec.ExitError); ok {
		resp.ExitCode = exitErr.ExitCode()
	} else {
		resp.ExitCode = -1
		resp.Stderr = err.Error()
	}
}

func (e *Executor) runDirect(ctx context.Context, req *workerapi.RunJobRequest, resp *workerapi.RunJobResponse, workspaceDir string) (*workerapi.RunJobResponse, error) {
	cmd := exec.CommandContext(ctx, req.Sandbox.Command[0], req.Sandbox.Command[1:]...)

	workspaceDirValue := workspaceMount
	if workspaceDir != "" {
		workspaceDirValue = workspaceDir
		cmd.Dir = workspaceDir
	}
	env := e.buildTaskEnv(req, workspaceDirValue)
	if req.Sandbox.UseInference && e.ollamaUpstreamURL != "" {
		env[envOllamaBaseURL] = ollamaBaseURLInPod
	}
	envSlice := make([]string, 0, len(env))
	for k, v := range env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = append(os.Environ(), envSlice...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	endedAt := time.Now().UTC()
	resp.EndedAt = endedAt.Format(time.RFC3339)

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	resp.Truncated.Stdout = len(stdoutStr) > e.maxOutputBytes
	resp.Truncated.Stderr = len(stderrStr) > e.maxOutputBytes
	stdoutStr = truncateUTF8(stdoutStr, e.maxOutputBytes)
	stderrStr = truncateUTF8(stderrStr, e.maxOutputBytes)

	resp.Stdout = stdoutStr
	resp.Stderr = stderrStr

	if ctx.Err() == context.DeadlineExceeded {
		resp.Status = workerapi.StatusTimeout
		resp.ExitCode = -1
		return resp, nil
	}

	if err != nil {
		e.setRunError(resp, err)
		return resp, nil
	}

	resp.Status = workerapi.StatusCompleted
	resp.ExitCode = 0
	return resp, nil
}

// truncateUTF8 truncates s to at most maxBytes bytes while preserving valid UTF-8
// (no rune is cut in the middle). Per worker_api.md stdout/stderr capture limits.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	b := []byte(s[:maxBytes])
	for i := len(b) - 1; i >= 0; i-- {
		if utf8.RuneStart(b[i]) {
			_, n := utf8.DecodeRune(b[i:])
			if n > 0 && i+n <= len(b) {
				return string(b[:i+n])
			}
			b = b[:i]
		}
	}
	return string(b)
}
