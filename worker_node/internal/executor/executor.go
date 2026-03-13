// Package executor runs sandbox jobs using a container runtime.
package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
)

// Task context env keys (sandbox_container.md). Must not contain orchestrator secrets.
const (
	envTaskID            = "CYNODE_TASK_ID"
	envJobID             = "CYNODE_JOB_ID"
	envWorkspaceDir      = "CYNODE_WORKSPACE_DIR"
	envInferenceProxyURL = "INFERENCE_PROXY_URL"
	workspaceMount       = "/workspace"
	jobMount             = "/job"
	jobSpecFilename      = "job.json"
	resultFilename       = "result.json"
	runtimePodman        = "podman"
	// inferenceProxySockInContainer is the UDS socket path inside the sandbox container
	// (pod shared namespace or direct bind-mount) where the inference proxy listens.
	// REQ-SANDBX-0131 / REQ-WORKER-0260.
	inferenceProxySockInContainer = "/run/cynode/inference-proxy.sock"
)

// Executor executes sandbox jobs.
type Executor struct {
	runtime               string // docker or podman
	defaultTimeout        time.Duration
	maxOutputBytes        int
	ollamaUpstreamURL     string // when set with inferenceProxyImage, jobs with UseInference run in pod with proxy
	inferenceProxyImage   string
	inferenceProxyCommand []string // optional; when set, appended to proxy container run (e.g. ["sleep","60"] for tests)
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

	// When UseInference is set and node has proxy image + Ollama URL, run in pod with inference proxy (worker_node.md Option A).
	if req.Sandbox.UseInference && e.ollamaUpstreamURL != "" && e.inferenceProxyImage != "" && e.runtime == runtimePodman {
		return e.runJobWithPodInference(ctx, req, resp, workspaceDir)
	}

	// When job_spec_json is set and image is an SBA runner, run SBA path: write job.json, mount /job, run image entrypoint, read result.json (P2-10).
	if req.Sandbox.JobSpecJSON != "" && isSBARunnerImage(image) {
		return e.runJobSBA(ctx, req, resp, workspaceDir)
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

	runErr := cmd.Run()
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

	if runErr != nil {
		e.setRunError(resp, runErr)
		return resp, nil
	}

	resp.Status = workerapi.StatusCompleted
	resp.ExitCode = 0
	return resp, nil
}

// runJobWithPodInference runs the job in a Podman pod with an inference proxy sidecar.
// REQ-SANDBX-0131: sandbox gets INFERENCE_PROXY_URL=http+unix://... (UDS), not TCP OLLAMA_BASE_URL.
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
	proxyOut, err := runProxy.CombinedOutput()
	if err != nil {
		setPodInferenceError(resp, "proxy start", proxyOut)
		return resp, nil
	}
	proxyContainerID := strings.TrimSpace(string(proxyOut))
	if proxyContainerID == "" {
		setPodInferenceError(resp, "proxy start", []byte("missing proxy container id"))
		return resp, nil
	}
	useHealthProbe := len(e.inferenceProxyCommand) == 0
	if err := waitForProxyReady(ctx, e.runtime, proxyContainerID, 10*time.Second, useHealthProbe); err != nil {
		setPodInferenceError(resp, "proxy readiness", []byte(err.Error()))
		return resp, nil
	}

	if workspaceDir != "" {
		if err := os.MkdirAll(workspaceDir, 0o700); err != nil {
			setPodInferenceError(resp, "workspace dir", []byte(err.Error()))
			return resp, nil
		}
	}

	image := req.Sandbox.Image
	if image == "" {
		image = workerapi.DefaultImage
	}
	env := e.buildTaskEnv(req, workspaceMount)
	// REQ-SANDBX-0131: UDS only; proxy sidecar shares pod network namespace.
	env[envInferenceProxyURL] = inferenceProxyUDSURL(inferenceProxySockInContainer)
	sandboxArgs := buildSandboxRunArgsForPod(req, podName, workspaceDir, env, image)

	cmd := exec.CommandContext(ctx, e.runtime, sandboxArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
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
	if runErr != nil {
		e.setRunError(resp, runErr)
		return resp, nil
	}
	resp.Status = workerapi.StatusCompleted
	resp.ExitCode = 0
	return resp, nil
}

var probeProxyHealthFunc = probeProxyHealthOnce
var probeProxyRunningFunc = probeProxyRunningOnce

func waitForProxyReady(ctx context.Context, runtime, proxyContainerID string, timeout time.Duration, useHealthProbe bool) error {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var lastErr error
	for {
		var err error
		if useHealthProbe {
			err = probeProxyHealthFunc(deadlineCtx, runtime, proxyContainerID)
		} else {
			err = probeProxyRunningFunc(deadlineCtx, runtime, proxyContainerID)
		}
		if err == nil {
			return nil
		}
		lastErr = err
		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("proxy health probe timeout: %w", lastErr)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func probeProxyHealthOnce(ctx context.Context, runtime, proxyContainerID string) error {
	probeCmd := exec.CommandContext(
		ctx,
		runtime,
		"exec",
		proxyContainerID,
		"/inference-proxy",
		"--healthcheck-url",
		"http://127.0.0.1:11434/healthz",
	)
	if out, err := probeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exec proxy healthcheck: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func probeProxyRunningOnce(ctx context.Context, runtime, proxyContainerID string) error {
	inspectCmd := exec.CommandContext(ctx, runtime, "inspect", "-f", "{{.State.Running}}", proxyContainerID)
	out, err := inspectCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inspect proxy container: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("proxy container not running")
	}
	return nil
}

func setPodInferenceError(resp *workerapi.RunJobResponse, prefix string, out []byte) {
	resp.EndedAt = time.Now().UTC().Format(time.RFC3339)
	resp.Status = workerapi.StatusFailed
	resp.ExitCode = -1
	resp.Stderr = prefix + ": " + strings.TrimSpace(string(out))
}

// isSBARunnerImage returns true when the image is an SBA runner (cynode-sba). Per worker_api.md Node-Mediated SBA Result.
func isSBARunnerImage(image string) bool {
	return strings.Contains(image, "cynode-sba") || strings.Contains(image, "cynodeai-cynode-sba")
}

// prepareSBAJobAndWorkspace creates job dir, writes job.json and result.json, resolves workspace dir. Caller must remove jobDir.
func prepareSBAJobAndWorkspace(req *workerapi.RunJobRequest, workspaceDir string) (jobDir, workspaceDirToUse string, err error) {
	jobDir, err = os.MkdirTemp("", "cynodeai-job-")
	if err != nil {
		return "", "", fmt.Errorf("failed to create job dir: %w", err)
	}
	jobPath := filepath.Join(jobDir, jobSpecFilename)
	if err := os.WriteFile(jobPath, []byte(req.Sandbox.JobSpecJSON), 0o644); err != nil {
		_ = os.RemoveAll(jobDir)
		return "", "", fmt.Errorf("failed to write job.json: %w", err)
	}
	resultPath := filepath.Join(jobDir, resultFilename)
	if err := os.WriteFile(resultPath, []byte("\n"), 0o666); err != nil {
		_ = os.RemoveAll(jobDir)
		return "", "", fmt.Errorf("failed to pre-create result.json: %w", err)
	}
	if err := os.Chmod(resultPath, 0o666); err != nil {
		_ = os.RemoveAll(jobDir)
		return "", "", fmt.Errorf("failed to chmod result.json: %w", err)
	}
	if err := os.Chmod(jobDir, 0o777); err != nil {
		_ = os.RemoveAll(jobDir)
		return "", "", fmt.Errorf("failed to chmod job dir: %w", err)
	}
	workspaceDirToUse = workspaceDir
	if workspaceDirToUse == "" {
		workspaceDirToUse, err = os.MkdirTemp("", "cynodeai-ws-")
		if err != nil {
			_ = os.RemoveAll(jobDir)
			return "", "", fmt.Errorf("failed to create temp workspace: %w", err)
		}
	}
	if err := os.MkdirAll(workspaceDirToUse, 0o700); err != nil {
		_ = os.RemoveAll(jobDir)
		return "", "", fmt.Errorf("failed to prepare workspace dir: %w", err)
	}
	return jobDir, workspaceDirToUse, nil
}

// runJobSBA runs a job with an SBA runner image: write job_spec_json to /job/job.json, mount /job and /workspace, run container (entrypoint cynode-sba), read /job/result.json into resp.SbaResult.
func (e *Executor) runJobSBA(ctx context.Context, req *workerapi.RunJobRequest, resp *workerapi.RunJobResponse, workspaceDir string) (*workerapi.RunJobResponse, error) {
	spec, err := sbajob.ParseAndValidateJobSpec([]byte(req.Sandbox.JobSpecJSON))
	if err != nil {
		setSBAError(resp, "invalid SBA job_spec_json: "+err.Error())
		return resp, nil
	}
	executionMode := sbajob.EffectiveExecutionMode(spec)
	if executionMode != sbajob.ExecutionModeAgentInference && executionMode != sbajob.ExecutionModeDirectSteps {
		setSBAError(resp, "unsupported SBA execution_mode: "+executionMode)
		return resp, nil
	}
	jobDir, workspaceDirToUse, err := prepareSBAJobAndWorkspace(req, workspaceDir)
	if err != nil {
		setSBAError(resp, err.Error())
		return resp, nil
	}
	defer func() { _ = os.RemoveAll(jobDir) }()
	if workspaceDir == "" {
		defer func() { _ = os.RemoveAll(workspaceDirToUse) }()
	}
	if e.shouldUseSBAPodInference(executionMode) {
		return e.runJobSBAWithPodInference(ctx, req, resp, workspaceDirToUse, jobDir, executionMode)
	}

	args := buildSBARunArgs(req, jobDir, workspaceDirToUse, e, executionMode)
	fullArgv := append([]string{e.runtime}, args...)
	diag := &workerapi.RunDiagnostics{
		Runtime:          e.runtime,
		RuntimeArgv:      fullArgv,
		JobDir:           jobDir,
		WorkspaceDir:     workspaceDirToUse,
		Image:            req.Sandbox.Image,
		ContainerStarted: false,
	}
	cmd := exec.CommandContext(ctx, e.runtime, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	endedAt := time.Now().UTC()
	resp.EndedAt = endedAt.Format(time.RFC3339)
	// Container started if the runtime process ran (success or exit code); false if e.g. executable not found.
	diag.ContainerStarted = runErr == nil || isExitError(runErr)
	resp.RunDiagnostics = diag

	resp.Truncated.Stdout = len(stdout.String()) > e.maxOutputBytes
	resp.Truncated.Stderr = len(stderr.String()) > e.maxOutputBytes
	resp.Stdout = truncateUTF8(stdout.String(), e.maxOutputBytes)
	resp.Stderr = truncateUTF8(stderr.String(), e.maxOutputBytes)

	if ctx.Err() == context.DeadlineExceeded {
		resp.Status = workerapi.StatusTimeout
		resp.ExitCode = -1
		return resp, nil
	}
	if runErr != nil {
		e.setRunError(resp, runErr)
	}
	applySbaResultFromDir(jobDir, resp)
	return resp, nil
}

func (e *Executor) shouldUseSBAPodInference(executionMode string) bool {
	return executionMode == sbajob.ExecutionModeAgentInference &&
		e.runtime == runtimePodman &&
		strings.TrimSpace(e.ollamaUpstreamURL) != "" &&
		strings.TrimSpace(e.inferenceProxyImage) != ""
}

func (e *Executor) runJobSBAWithPodInference(
	ctx context.Context,
	req *workerapi.RunJobRequest,
	resp *workerapi.RunJobResponse,
	workspaceDir string,
	jobDir string,
	executionMode string,
) (*workerapi.RunJobResponse, error) {
	podName := "cynodeai-job-" + sanitizePodName(req.JobID)
	if out, err := e.createOrReplacePod(ctx, podName); err != nil {
		setPodInferenceError(resp, "pod create", out)
		return resp, nil
	}
	cleanupCtx := context.WithoutCancel(ctx)
	defer func() { _ = exec.CommandContext(cleanupCtx, e.runtime, "pod", "rm", "-f", podName).Run() }()

	proxyArgs := buildProxyRunArgs(podName, e.ollamaUpstreamURL, e.inferenceProxyImage, e.inferenceProxyCommand)
	runProxy := exec.CommandContext(ctx, e.runtime, proxyArgs...)
	proxyOut, err := runProxy.CombinedOutput()
	if err != nil {
		setPodInferenceError(resp, "proxy start", proxyOut)
		return resp, nil
	}
	proxyContainerID := strings.TrimSpace(string(proxyOut))
	if proxyContainerID == "" {
		setPodInferenceError(resp, "proxy start", []byte("missing proxy container id"))
		return resp, nil
	}
	useHealthProbe := len(e.inferenceProxyCommand) == 0
	if err := waitForProxyReady(ctx, e.runtime, proxyContainerID, 10*time.Second, useHealthProbe); err != nil {
		setPodInferenceError(resp, "proxy readiness", []byte(err.Error()))
		return resp, nil
	}

	// Ensure workspace dir exists immediately before podman run so bind-mount source is present
	// (avoids statfs "no such file or directory" when worker-api and podman run in different contexts).
	if workspaceDir != "" {
		if err := os.MkdirAll(workspaceDir, 0o700); err != nil {
			setSBAError(resp, "workspace dir: "+err.Error())
			return resp, nil
		}
	}

	args := buildSBARunArgsForPod(req, podName, jobDir, workspaceDir, e, executionMode)
	fullArgv := append([]string{e.runtime}, args...)
	resp.RunDiagnostics = &workerapi.RunDiagnostics{
		Runtime:          e.runtime,
		RuntimeArgv:      fullArgv,
		JobDir:           jobDir,
		WorkspaceDir:     workspaceDir,
		Image:            req.Sandbox.Image,
		ContainerStarted: false,
	}
	cmd := exec.CommandContext(ctx, e.runtime, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	resp.EndedAt = time.Now().UTC().Format(time.RFC3339)
	resp.RunDiagnostics.ContainerStarted = runErr == nil || isExitError(runErr)

	resp.Truncated.Stdout = len(stdout.String()) > e.maxOutputBytes
	resp.Truncated.Stderr = len(stderr.String()) > e.maxOutputBytes
	resp.Stdout = truncateUTF8(stdout.String(), e.maxOutputBytes)
	resp.Stderr = truncateUTF8(stderr.String(), e.maxOutputBytes)
	if ctx.Err() == context.DeadlineExceeded {
		resp.Status = workerapi.StatusTimeout
		resp.ExitCode = -1
		return resp, nil
	}
	if runErr != nil {
		e.setRunError(resp, runErr)
	}
	applySbaResultFromDir(jobDir, resp)
	return resp, nil
}

func (e *Executor) createOrReplacePod(ctx context.Context, podName string) ([]byte, error) {
	create := exec.CommandContext(ctx, e.runtime, "pod", "create", "--name", podName)
	out, err := create.CombinedOutput()
	if err == nil {
		return out, nil
	}
	if !strings.Contains(strings.ToLower(string(out)), "already exists") {
		return out, err
	}
	_ = exec.CommandContext(context.WithoutCancel(ctx), e.runtime, "pod", "rm", "-f", podName).Run()
	retry := exec.CommandContext(ctx, e.runtime, "pod", "create", "--name", podName)
	return retry.CombinedOutput()
}

func isExitError(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}

func setSBAError(resp *workerapi.RunJobResponse, msg string) {
	resp.EndedAt = time.Now().UTC().Format(time.RFC3339)
	resp.Status = workerapi.StatusFailed
	resp.ExitCode = -1
	resp.Stderr = msg
}

// inferenceProxyUDSURL returns the http+unix:// URL for the inference proxy socket at sockPath.
// REQ-SANDBX-0131 / REQ-WORKER-0260: sandbox containers access inference via UDS only.
func inferenceProxyUDSURL(sockPath string) string {
	return "http+unix://" + url.PathEscape(sockPath)
}

func buildSBARunArgs(req *workerapi.RunJobRequest, jobDir, workspaceDir string, e *Executor, executionMode string) []string {
	args := []string{"run", "--rm"}
	// REQ-WORKER-0174: SBA containers always run with --network=none.
	// Inference is accessed via UDS socket mount, not via network.
	args = append(args, "--network=none")
	// Rootless podman: keep host UID so the container can write to the bind-mounted jobDir (result.json).
	if e.runtime == runtimePodman {
		args = append(args, "--userns=keep-id")
	}
	// :z (SELinux shared) so the container can read/write the mount when SELinux is enabled.
	jobMountOpt := fmt.Sprintf("%s:%s", jobDir, jobMount)
	if e.runtime == runtimePodman {
		jobMountOpt += ":z"
	}
	args = append(args,
		"-v", jobMountOpt,
		"--label", fmt.Sprintf("cynodeai.task_id=%s", req.TaskID),
		"--label", fmt.Sprintf("cynodeai.job_id=%s", req.JobID),
	)
	if workspaceDir != "" {
		wsMountOpt := fmt.Sprintf("%s:%s", workspaceDir, workspaceMount)
		if e.runtime == runtimePodman {
			wsMountOpt += ":z"
		}
		args = append(args, "-v", wsMountOpt, "-w", workspaceMount)
	}
	env := e.buildTaskEnv(req, workspaceMount)
	for k, v := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, "-e", fmt.Sprintf("SBA_EXECUTION_MODE=%s", executionMode))
	if executionMode == sbajob.ExecutionModeAgentInference && strings.TrimSpace(e.ollamaUpstreamURL) != "" {
		// REQ-SANDBX-0131: inject INFERENCE_PROXY_URL (UDS) only when the worker provides a proxy socket
		// (SBA_INFERENCE_PROXY_SOCKET set by embed). Mount host socket dir so the path exists in the container.
		if hostSock := os.Getenv("SBA_INFERENCE_PROXY_SOCKET"); hostSock != "" {
			hostDir := filepath.Dir(hostSock)
			mountOpt := hostDir + ":" + "/run/cynode"
			if e.runtime == runtimePodman {
				mountOpt += ":z"
			}
			args = append(args, "-v", mountOpt, "-e", fmt.Sprintf("%s=%s", envInferenceProxyURL, inferenceProxyUDSURL(inferenceProxySockInContainer)))
		}
	}
	if executionMode == sbajob.ExecutionModeDirectSteps {
		args = append(args, "-e", "SBA_DIRECT_STEPS=1")
	}
	args = append(append(args, req.Sandbox.Image), req.Sandbox.Command...)
	return args
}

// BuildSBARunArgs is the exported entry point for BDD step definitions (non-pod / direct path).
// REQ-SANDBX-0131: exported so BDD can assert the UDS contract without importing internal test helpers.
func BuildSBARunArgs(req *workerapi.RunJobRequest, jobDir, workspaceDir string, e *Executor, executionMode string) []string {
	return buildSBARunArgs(req, jobDir, workspaceDir, e, executionMode)
}

// BuildSBARunArgsForPod is the exported entry point for BDD step definitions.
// REQ-SANDBX-0131: exported so BDD can assert the UDS contract without importing internal test helpers.
func BuildSBARunArgsForPod(req *workerapi.RunJobRequest, podName, jobDir, workspaceDir string, e *Executor, executionMode string) []string {
	return buildSBARunArgsForPod(req, podName, jobDir, workspaceDir, e, executionMode)
}

func buildSBARunArgsForPod(req *workerapi.RunJobRequest, podName, jobDir, workspaceDir string, e *Executor, executionMode string) []string {
	args := []string{"run", "--rm", "--pod", podName}
	jobMountOpt := fmt.Sprintf("%s:%s", jobDir, jobMount)
	if e.runtime == runtimePodman {
		jobMountOpt += ":z"
	}
	args = append(args,
		"-v", jobMountOpt,
		"--label", fmt.Sprintf("cynodeai.task_id=%s", req.TaskID),
		"--label", fmt.Sprintf("cynodeai.job_id=%s", req.JobID),
	)
	if workspaceDir != "" {
		wsMountOpt := fmt.Sprintf("%s:%s", workspaceDir, workspaceMount)
		if e.runtime == runtimePodman {
			wsMountOpt += ":z"
		}
		args = append(args, "-v", wsMountOpt, "-w", workspaceMount)
	}
	env := e.buildTaskEnv(req, workspaceMount)
	// REQ-SANDBX-0131: inject INFERENCE_PROXY_URL (UDS) instead of TCP OLLAMA_BASE_URL.
	// The proxy sidecar in the pod listens on inferenceProxySockInContainer (shared pod namespace).
	env[envInferenceProxyURL] = inferenceProxyUDSURL(inferenceProxySockInContainer)
	for k, v := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, "-e", fmt.Sprintf("SBA_EXECUTION_MODE=%s", executionMode))
	args = append(append(args, req.Sandbox.Image), req.Sandbox.Command...)
	return args
}

func setSbaFailedIfUnset(resp *workerapi.RunJobResponse, stderrMsg string) {
	if resp.Status != workerapi.StatusTimeout && resp.Status == "" {
		resp.Status = workerapi.StatusFailed
		resp.ExitCode = -1
		if resp.Stderr == "" {
			resp.Stderr = stderrMsg
		}
	}
}

func applySbaResultFromDir(jobDir string, resp *workerapi.RunJobResponse) {
	resultPath := filepath.Join(jobDir, resultFilename)
	data, err := os.ReadFile(resultPath)
	if err != nil {
		setSbaFailedIfUnset(resp, "result.json missing or unreadable: "+err.Error())
		return
	}
	var sbaResult sbajob.Result
	if json.Unmarshal(data, &sbaResult) != nil {
		setSbaFailedIfUnset(resp, "result.json invalid or empty (container may have exited before writing result)")
		return
	}
	resp.SbaResult = &sbaResult
	if resp.Status == workerapi.StatusTimeout {
		return
	}
	switch sbaResult.Status {
	case "success":
		resp.Status = workerapi.StatusCompleted
		resp.ExitCode = 0
	case "failure":
		resp.Status = workerapi.StatusFailed
		if resp.ExitCode == 0 {
			resp.ExitCode = 1
		}
	}
}

// buildProxyRunArgs returns the argv for running the inference proxy container in the pod.
// REQ-SANDBX-0131 / REQ-WORKER-0260: the proxy MUST listen on the UDS socket path
// (inferenceProxySockInContainer) shared within the pod so the SBA container can reach it.
func buildProxyRunArgs(podName, ollamaUpstreamURL, image string, command []string) []string {
	args := []string{"run", "-d", "--rm", "--pod", podName,
		"-e", "OLLAMA_UPSTREAM_URL=" + ollamaUpstreamURL,
		"-e", "INFERENCE_PROXY_SOCKET=" + inferenceProxySockInContainer,
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
// Request env must not override worker-reserved CYNODE_* (task id, job id, workspace); other CYNODE_* from the request (e.g. CYNODE_PROMPT) are allowed.
func (e *Executor) buildTaskEnv(req *workerapi.RunJobRequest, workspaceDirValue string) map[string]string {
	out := map[string]string{
		envTaskID:       req.TaskID,
		envJobID:        req.JobID,
		envWorkspaceDir: workspaceDirValue,
	}
	reserved := map[string]bool{envTaskID: true, envJobID: true, envWorkspaceDir: true}
	for k, v := range req.Sandbox.Env {
		if reserved[k] {
			continue
		}
		out[k] = v
	}
	return out
}

// setRunError sets resp status/exit/stderr from an execution error.
// Preserves existing resp.Stderr (container/runtime output) when appending the error.
func (e *Executor) setRunError(resp *workerapi.RunJobResponse, err error) {
	resp.Status = workerapi.StatusFailed
	errMsg := err.Error()
	if exitErr, ok := err.(*exec.ExitError); ok {
		resp.ExitCode = exitErr.ExitCode()
		if resp.Stderr != "" {
			resp.Stderr = "runtime exit " + strconv.Itoa(exitErr.ExitCode()) + ": " + errMsg + "\n--- runtime stderr ---\n" + resp.Stderr
		} else {
			resp.Stderr = "runtime exit " + strconv.Itoa(exitErr.ExitCode()) + ": " + errMsg
		}
	} else {
		resp.ExitCode = -1
		if resp.Stderr != "" {
			resp.Stderr = errMsg + "\n--- runtime stderr ---\n" + resp.Stderr
		} else {
			resp.Stderr = errMsg
		}
	}
}

func (e *Executor) runDirect(ctx context.Context, req *workerapi.RunJobRequest, resp *workerapi.RunJobResponse, workspaceDir string) (*workerapi.RunJobResponse, error) {
	if len(req.Sandbox.Command) == 0 {
		resp.EndedAt = time.Now().UTC().Format(time.RFC3339)
		resp.Status = workerapi.StatusFailed
		resp.ExitCode = -1
		resp.Stderr = "direct runtime requires sandbox.command (SBA jobs need a container runtime)"
		return resp, nil
	}
	cmd := exec.CommandContext(ctx, req.Sandbox.Command[0], req.Sandbox.Command[1:]...)

	workspaceDirValue := workspaceMount
	if workspaceDir != "" {
		workspaceDirValue = workspaceDir
		cmd.Dir = workspaceDir
	}
	env := e.buildTaskEnv(req, workspaceDirValue)
	if req.Sandbox.UseInference && e.ollamaUpstreamURL != "" {
		// Per worker_node.md and ports_and_endpoints: sandbox receives inference via UDS (INFERENCE_PROXY_URL).
		// Direct runtime has no real proxy; use a UDS-style URL so tests can assert the contract.
		env[envInferenceProxyURL] = "http+unix://%2Frun%2Finference-proxy%2Fsock"
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
