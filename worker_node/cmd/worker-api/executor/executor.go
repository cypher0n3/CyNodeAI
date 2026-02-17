// Package executor runs sandbox jobs using a container runtime.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
)

// Executor executes sandbox jobs.
type Executor struct {
	runtime        string // docker or podman
	defaultTimeout time.Duration
	maxOutputBytes int
}

// New creates a new job executor.
func New(runtime string, defaultTimeout time.Duration, maxOutputBytes int) *Executor {
	return &Executor{
		runtime:        runtime,
		defaultTimeout: defaultTimeout,
		maxOutputBytes: maxOutputBytes,
	}
}

// RunJob executes a sandbox job and returns the result.
func (e *Executor) RunJob(ctx context.Context, req *workerapi.RunJobRequest) (*workerapi.RunJobResponse, error) {
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
		return e.runDirect(ctx, req, resp)
	}

	// Build container run command.
	args := []string{"run", "--rm"}

	// Default to no networking in MVP (restricted).
	args = append(args, "--network=none")

	// Add environment variables.
	for k, v := range req.Sandbox.Env {
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

	if resp.Truncated.Stdout {
		stdoutStr = stdoutStr[:e.maxOutputBytes]
	}
	if resp.Truncated.Stderr {
		stderrStr = stderrStr[:e.maxOutputBytes]
	}

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

func (e *Executor) runDirect(ctx context.Context, req *workerapi.RunJobRequest, resp *workerapi.RunJobResponse) (*workerapi.RunJobResponse, error) {
	cmd := exec.CommandContext(ctx, req.Sandbox.Command[0], req.Sandbox.Command[1:]...)

	if len(req.Sandbox.Env) > 0 {
		env := make([]string, 0, len(req.Sandbox.Env))
		for k, v := range req.Sandbox.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = append(os.Environ(), env...)
	}

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

	if resp.Truncated.Stdout {
		stdoutStr = stdoutStr[:e.maxOutputBytes]
	}
	if resp.Truncated.Stderr {
		stderrStr = stderrStr[:e.maxOutputBytes]
	}

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
