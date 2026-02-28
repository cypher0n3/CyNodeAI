// Package dispatcher provides job dispatch helpers for the control-plane.
package dispatcher

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
)

// DefaultSBARunnerImage is the image used for SBA jobs when the payload does not set image (P2-10).
// Must match worker executor isSBARunnerImage (e.g. cynodeai-cynode-sba:dev).
const DefaultSBARunnerImage = "cynodeai-cynode-sba:dev"

// ParseSandboxSpec parses a JSON job payload into a SandboxSpec.
// When job_spec_json is set (SBA job), command may be empty; image may be omitted (default SBA runner).
func ParseSandboxSpec(payload *string) (workerapi.SandboxSpec, error) {
	if payload == nil || *payload == "" {
		return workerapi.SandboxSpec{}, errors.New("job payload is empty")
	}

	var spec struct {
		Image          string            `json:"image"`
		Command        []string          `json:"command"`
		Env            map[string]string `json:"env"`
		TimeoutSeconds int               `json:"timeout_seconds"`
		NetworkPolicy  string            `json:"network_policy"`
		UseInference   bool              `json:"use_inference"`
		JobSpecJSON    string            `json:"job_spec_json"`
	}
	if err := json.Unmarshal([]byte(*payload), &spec); err != nil {
		return workerapi.SandboxSpec{}, fmt.Errorf("parse payload json: %w", err)
	}
	if spec.JobSpecJSON == "" && len(spec.Command) == 0 {
		return workerapi.SandboxSpec{}, errors.New("payload.command is required when job_spec_json is not set")
	}
	image := spec.Image
	if spec.JobSpecJSON != "" && image == "" {
		image = DefaultSBARunnerImage
	}

	return workerapi.SandboxSpec{
		Image:          image,
		Command:        spec.Command,
		Env:            spec.Env,
		TimeoutSeconds: spec.TimeoutSeconds,
		NetworkPolicy:  spec.NetworkPolicy,
		UseInference:   spec.UseInference,
		JobSpecJSON:    spec.JobSpecJSON,
	}, nil
}

// MarshalDispatchError returns a JSON string for a failed job result.
func MarshalDispatchError(err error) string {
	obj := map[string]any{
		"version": 1,
		"status":  "failed",
		"error":   err.Error(),
	}
	b, _ := json.Marshal(obj)
	return string(b)
}

// SummarizeResult returns a one-line summary of a job result for task summary.
func SummarizeResult(resp *workerapi.RunJobResponse) string {
	if resp == nil {
		return ""
	}
	if resp.Status != workerapi.StatusCompleted {
		return fmt.Sprintf("job %s", resp.Status)
	}
	if resp.Stdout != "" {
		return TruncateOneLine(resp.Stdout, 200)
	}
	if resp.Stderr != "" {
		return TruncateOneLine(resp.Stderr, 200)
	}
	return "completed"
}

// TruncateOneLine takes the first line of s and truncates to maxLen.
func TruncateOneLine(s string, maxLen int) string {
	line := s
	for i := 0; i < len(line); i++ {
		if line[i] == '\n' {
			line = line[:i]
			break
		}
	}
	if len(line) > maxLen {
		return line[:maxLen]
	}
	return line
}
