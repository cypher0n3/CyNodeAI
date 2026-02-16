// Package worker provides the Worker API for sandbox job execution.
// See docs/tech_specs/worker_api.md for the contract.
package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Job status constants.
const (
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusTimeout   = "timeout"
)

// Default container image.
const DefaultImage = "alpine:latest"

// JobRequest represents a job execution request.
// See docs/tech_specs/worker_api.md#run-job-synchronous
type JobRequest struct {
	Version int         `json:"version"`
	TaskID  string      `json:"task_id"`
	JobID   string      `json:"job_id"`
	Sandbox SandboxSpec `json:"sandbox"`
}

// SandboxSpec defines sandbox execution parameters.
type SandboxSpec struct {
	Image          string            `json:"image"`
	Command        []string          `json:"command"`
	Env            map[string]string `json:"env,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	NetworkPolicy  string            `json:"network_policy,omitempty"`
}

// JobResponse represents a job execution response.
type JobResponse struct {
	Version   int           `json:"version"`
	TaskID    string        `json:"task_id"`
	JobID     string        `json:"job_id"`
	Status    string        `json:"status"`
	ExitCode  int           `json:"exit_code,omitempty"`
	Stdout    string        `json:"stdout"`
	Stderr    string        `json:"stderr"`
	StartedAt string        `json:"started_at"`
	EndedAt   string        `json:"ended_at"`
	Truncated TruncatedInfo `json:"truncated"`
}

// TruncatedInfo indicates if output was truncated.
type TruncatedInfo struct {
	Stdout bool `json:"stdout"`
	Stderr bool `json:"stderr"`
}

// Executor executes sandbox jobs.
type Executor struct {
	runtime        string // docker or podman
	defaultTimeout time.Duration
	maxOutputBytes int
}

// NewExecutor creates a new job executor.
func NewExecutor(runtime string, defaultTimeout time.Duration, maxOutputBytes int) *Executor {
	return &Executor{
		runtime:        runtime,
		defaultTimeout: defaultTimeout,
		maxOutputBytes: maxOutputBytes,
	}
}

// RunJob executes a sandbox job and returns the result.
func (e *Executor) RunJob(ctx context.Context, req *JobRequest) (*JobResponse, error) {
	startedAt := time.Now().UTC()

	resp := &JobResponse{
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

	// Build container run command
	args := []string{"run", "--rm"}

	// Add network policy - default to restricted (no network) for all cases
	args = append(args, "--network=none")

	// Add environment variables
	for k, v := range req.Sandbox.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Add labels for tracking, image, and command
	args = append(args,
		"--label", fmt.Sprintf("cynodeai.task_id=%s", req.TaskID),
		"--label", fmt.Sprintf("cynodeai.job_id=%s", req.JobID),
		req.Sandbox.Image)
	args = append(args, req.Sandbox.Command...)

	// Execute container
	cmd := exec.CommandContext(ctx, e.runtime, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	endedAt := time.Now().UTC()
	resp.EndedAt = endedAt.Format(time.RFC3339)

	// Process output
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

	// Determine status
	if ctx.Err() == context.DeadlineExceeded {
		resp.Status = StatusTimeout
		resp.ExitCode = -1
		return resp, nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			resp.Status = StatusFailed
			resp.ExitCode = exitErr.ExitCode()
		} else {
			resp.Status = StatusFailed
			resp.ExitCode = -1
			resp.Stderr = err.Error()
		}
	} else {
		resp.Status = StatusCompleted
		resp.ExitCode = 0
	}

	return resp, nil
}

// MarshalResult converts a JobResponse to JSON for storage.
func MarshalResult(resp *JobResponse) (string, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseJobPayload parses job payload JSON into command components.
func ParseJobPayload(payload string) (string, []string, map[string]string, error) {
	var spec struct {
		Image   string            `json:"image"`
		Command []string          `json:"command"`
		Env     map[string]string `json:"env"`
	}

	if err := json.Unmarshal([]byte(payload), &spec); err != nil {
		// Try simple command format
		spec.Image = DefaultImage
		spec.Command = []string{"sh", "-c", strings.TrimSpace(payload)}
	}

	if spec.Image == "" {
		spec.Image = DefaultImage
	}

	return spec.Image, spec.Command, spec.Env, nil
}
