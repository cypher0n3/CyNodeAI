// Package workerapi defines the Worker API contract payloads.
package workerapi

import (
	"encoding/json"
	"strings"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
)

// Job status constants.
const (
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusTimeout   = "timeout"
)

// DefaultImage is used when no image is specified.
const DefaultImage = "alpine:latest"

// RunJobRequest represents a synchronous job execution request.
// See docs/tech_specs/worker_api.md#run-job-synchronous.
type RunJobRequest struct {
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
	// UseInference when true asks the node to run the job in a pod with an inference proxy
	// so the sandbox can call http://localhost:11434 for node-local Ollama (worker_node.md Option A).
	UseInference bool `json:"use_inference,omitempty"`
	// JobSpecJSON is the SBA job specification JSON when the job uses an SBA runner image (P2-10).
	// The node writes this to /job/job.json and runs the container with /job bind-mounted; on exit reads /job/result.json into response sba_result.
	JobSpecJSON string `json:"job_spec_json,omitempty"`
}

// RunDiagnostics holds troubleshooting data for container runs (e.g. SBA).
// Populated by the worker so failed jobs can be debugged (exact runtime command, mount paths, etc.).
type RunDiagnostics struct {
	// Runtime is the container runtime binary used (e.g. "podman", "docker").
	Runtime string `json:"runtime"`
	// RuntimeArgv is the full command line: runtime + all arguments (so the exact podman/docker invocation).
	RuntimeArgv []string `json:"runtime_argv"`
	// JobDir is the host path mounted at /job (SBA: job.json and result.json).
	JobDir string `json:"job_dir,omitempty"`
	// WorkspaceDir is the host path mounted at /workspace (empty if not used).
	WorkspaceDir string `json:"workspace_dir,omitempty"`
	// Image is the image name used (e.g. "cynodeai-cynode-sba:dev").
	Image string `json:"image,omitempty"`
	// ContainerStarted is true if the runtime process was started and ran to completion (exit 0 or non-zero).
	// False if the run failed before starting (e.g. image not found, executable not found).
	ContainerStarted bool `json:"container_started"`
}

// RunJobResponse represents a synchronous job execution response.
// See docs/tech_specs/worker_api.md#run-job-synchronous.
type RunJobResponse struct {
	Version            int               `json:"version"`
	TaskID             string            `json:"task_id"`
	JobID              string            `json:"job_id"`
	Status             string            `json:"status"`
	ExitCode           *int              `json:"exit_code,omitempty"`
	Stdout             string            `json:"stdout"`
	Stderr             string            `json:"stderr"`
	StartedAt          string            `json:"started_at"`
	EndedAt            string            `json:"ended_at"`
	Truncated          TruncatedInfo     `json:"truncated"`
	SbaResult          *sbajob.Result    `json:"sba_result,omitempty"`
	StepExecutorResult json.RawMessage   `json:"step_executor_result,omitempty"`
	Artifacts          []json.RawMessage `json:"artifacts,omitempty"`
	// RunDiagnostics is set for container runs (e.g. SBA) to aid troubleshooting when a job fails.
	RunDiagnostics *RunDiagnostics `json:"run_diagnostics,omitempty"`
}

// TruncatedInfo indicates if output was truncated.
type TruncatedInfo struct {
	Stdout bool `json:"stdout"`
	Stderr bool `json:"stderr"`
}

// DefaultSandboxSpec returns a SandboxSpec with DefaultImage and empty command.
// Callers should set Command before use.
func DefaultSandboxSpec() SandboxSpec {
	return SandboxSpec{
		Image:   DefaultImage,
		Command: nil,
		Env:     nil,
	}
}

// ExitCodePtr returns a pointer to c for RunJobResponse.ExitCode so JSON includes exit_code when 0.
func ExitCodePtr(c int) *int {
	p := c
	return &p
}

// ValidateRequest returns an error if the request is invalid for execution.
// When job_spec_json is set (SBA runner job), command may be empty (node uses image entrypoint).
func ValidateRequest(req *RunJobRequest) error {
	if req == nil {
		return &RequestValidationError{Reason: "request is nil"}
	}
	if req.Version != 1 {
		return &RequestValidationError{Reason: "version must be 1"}
	}
	if strings.TrimSpace(req.TaskID) == "" {
		return &RequestValidationError{Reason: "task_id is required"}
	}
	if strings.TrimSpace(req.JobID) == "" {
		return &RequestValidationError{Reason: "job_id is required"}
	}
	if req.Sandbox.JobSpecJSON == "" && len(req.Sandbox.Command) == 0 {
		return &RequestValidationError{Reason: "sandbox.command is required when job_spec_json is not set"}
	}
	if req.Sandbox.TimeoutSeconds < 0 {
		return &RequestValidationError{Reason: "sandbox.timeout_seconds must be non-negative"}
	}
	switch strings.ToLower(strings.TrimSpace(req.Sandbox.NetworkPolicy)) {
	case "", "none", "restricted", "allow":
	default:
		return &RequestValidationError{Reason: "sandbox.network_policy must be empty, none, restricted, or allow"}
	}
	return nil
}

// RequestValidationError is returned when RunJobRequest is invalid.
type RequestValidationError struct {
	Reason string
}

func (e *RequestValidationError) Error() string {
	return e.Reason
}
