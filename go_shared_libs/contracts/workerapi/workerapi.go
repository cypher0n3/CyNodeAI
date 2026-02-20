// Package workerapi defines the Worker API contract payloads.
package workerapi

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
	// so the sandbox can call http://localhost:11434 for node-local Ollama (node.md Option A).
	UseInference bool `json:"use_inference,omitempty"`
}

// RunJobResponse represents a synchronous job execution response.
type RunJobResponse struct {
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

// DefaultSandboxSpec returns a SandboxSpec with DefaultImage and empty command.
// Callers should set Command before use.
func DefaultSandboxSpec() SandboxSpec {
	return SandboxSpec{
		Image:   DefaultImage,
		Command: nil,
		Env:     nil,
	}
}

// ValidateRequest returns an error if the request is invalid for execution.
func ValidateRequest(req *RunJobRequest) error {
	if req == nil {
		return &RequestValidationError{Reason: "request is nil"}
	}
	if len(req.Sandbox.Command) == 0 {
		return &RequestValidationError{Reason: "sandbox.command is required"}
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
