// Package sbajob defines SBA job specification and result contract types and validation.
// See docs/tech_specs/cynode_sba.md.
package sbajob

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// SupportedProtocolMajor is the only accepted major version for protocol_version.
// Unknown major versions must be refused per CYNAI.SBAGNT.ProtocolVersioning.
const SupportedProtocolMajor = 1

// SBA execution modes.
const (
	ExecutionModeAgentInference = "agent_inference"
	ExecutionModeDirectSteps    = "direct_steps"
)

// JobSpec is the shared job specification (job.json) used by both the SBA and the step executor.
// Validation MUST occur before the runner starts; unknown fields are rejected.
// Steps is optional in the schema: when absent or empty, the SBA treats it as no suggested to-dos;
// the step executor requires steps to be present and non-empty (use ValidateStepExecutorJobSpec).
type JobSpec struct {
	ProtocolVersion string         `json:"protocol_version"`
	JobID           string         `json:"job_id"`
	TaskID          string         `json:"task_id"`
	ExecutionMode   string         `json:"execution_mode,omitempty"`
	Constraints     JobConstraints `json:"constraints"`
	Steps           []StepSpec     `json:"steps,omitempty"`
	Inference       *InferenceSpec `json:"inference,omitempty"`
	Context         *ContextSpec   `json:"context,omitempty"`
}

// JobConstraints holds timeout and output limits.
type JobConstraints struct {
	MaxRuntimeSeconds int  `json:"max_runtime_seconds"`
	MaxOutputBytes    int  `json:"max_output_bytes"`
	ExtNetAllowed     bool `json:"ext_net_allowed,omitempty"`
}

// StepSpec describes a single step (MVP: run_command, write_file, read_file, apply_unified_diff, list_tree, search_files).
type StepSpec struct {
	Type string          `json:"type"`
	Args json.RawMessage `json:"args,omitempty"`
}

// InferenceSpec defines allowed models for inference.
type InferenceSpec struct {
	AllowedModels []string `json:"allowed_models"`
	Source        string   `json:"source,omitempty"`
}

// ContextSpec holds baseline, project, task, requirements, acceptance criteria, preferences, skills.
type ContextSpec struct {
	// PersonaTitle and PersonaDescription embed the Agent persona when present (REQ-SBAGNT-0113); first block in prompt.
	PersonaTitle       string            `json:"persona_title,omitempty"`
	PersonaDescription string            `json:"persona_description,omitempty"`
	BaselineContext    string            `json:"baseline_context,omitempty"`
	ProjectContext     string            `json:"project_context,omitempty"`
	TaskContext        string            `json:"task_context,omitempty"`
	Requirements       []string          `json:"requirements,omitempty"`
	AcceptanceCriteria []string          `json:"acceptance_criteria,omitempty"`
	Preferences        map[string]string `json:"preferences,omitempty"`
	AdditionalContext  string            `json:"additional_context,omitempty"`
	SkillIDs           []string          `json:"skill_ids,omitempty"`
	// Skills holds inline skill content or references for the SBA; see CYNAI.SBAGNT.JobContext.
	Skills interface{} `json:"skills,omitempty"`
}

// Result is the SBA result contract (result.json).
// See CYNAI.SBAGNT.ResultContract.
type Result struct {
	ProtocolVersion string `json:"protocol_version"`
	JobID           string `json:"job_id"`
	// Status is one of: success, failure, timeout (see CYNAI.SBAGNT.ResultContract).
	Status         string        `json:"status"`
	FinalAnswer    string        `json:"final_answer,omitempty"`
	InferenceUsed  *bool         `json:"inference_used,omitempty"`
	Steps          []StepResult  `json:"steps"`
	Artifacts      []ArtifactRef `json:"artifacts"`
	FailureCode    *string       `json:"failure_code,omitempty"`
	FailureMessage *string       `json:"failure_message,omitempty"`
}

// StepResult is the result of a single step.
type StepResult struct {
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ArtifactRef references an artifact (path or MCP ref).
type ArtifactRef struct {
	Path string `json:"path,omitempty"`
	Ref  string `json:"ref,omitempty"`
}

// ValidationError is returned when job spec validation fails.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}

// ParseAndValidateJobSpec decodes JSON and validates the job spec.
// Unknown fields are rejected. Unknown major protocol version is refused.
// Returns a ValidationError on schema or protocol errors.
func ParseAndValidateJobSpec(data []byte) (*JobSpec, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	var spec JobSpec
	if err := dec.Decode(&spec); err != nil {
		return nil, &ValidationError{Message: "invalid JSON or unknown field: " + err.Error()}
	}
	if err := ValidateJobSpec(&spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// ValidateJobSpec validates a job spec (required fields and protocol version).
// Unknown major protocol versions must be refused per REQ-SBAGNT-0100.
func ValidateJobSpec(spec *JobSpec) error {
	if spec == nil {
		return &ValidationError{Message: "job spec is nil"}
	}
	if spec.ProtocolVersion == "" {
		return &ValidationError{Field: "protocol_version", Message: "required"}
	}
	major, err := parseMajorVersion(spec.ProtocolVersion)
	if err != nil {
		return &ValidationError{Field: "protocol_version", Message: err.Error()}
	}
	if major != SupportedProtocolMajor {
		return &ValidationError{
			Field:   "protocol_version",
			Message: fmt.Sprintf("unsupported major version %d (supported: %d)", major, SupportedProtocolMajor),
		}
	}
	if spec.JobID == "" {
		return &ValidationError{Field: "job_id", Message: "required"}
	}
	if spec.TaskID == "" {
		return &ValidationError{Field: "task_id", Message: "required"}
	}
	if spec.ExecutionMode != "" &&
		spec.ExecutionMode != ExecutionModeAgentInference &&
		spec.ExecutionMode != ExecutionModeDirectSteps {
		return &ValidationError{
			Field:   "execution_mode",
			Message: "must be one of: " + ExecutionModeAgentInference + ", " + ExecutionModeDirectSteps,
		}
	}
	if spec.Constraints.MaxRuntimeSeconds <= 0 {
		return &ValidationError{Field: "constraints.max_runtime_seconds", Message: "must be positive"}
	}
	if spec.Constraints.MaxOutputBytes <= 0 {
		return &ValidationError{Field: "constraints.max_output_bytes", Message: "must be positive"}
	}
	// steps is optional: nil or empty is valid for SBA; step executor must call ValidateStepExecutorJobSpec
	return nil
}

// EffectiveExecutionMode resolves the runtime mode for a job.
// Explicit execution_mode always wins. For backward compatibility:
// - jobs with steps and no mode default to direct_steps
// - jobs without steps and no mode default to agent_inference
func EffectiveExecutionMode(spec *JobSpec) string {
	if spec == nil {
		return ExecutionModeAgentInference
	}
	mode := strings.TrimSpace(spec.ExecutionMode)
	if mode != "" {
		return mode
	}
	if len(spec.Steps) > 0 {
		return ExecutionModeDirectSteps
	}
	return ExecutionModeAgentInference
}

// ValidateStepExecutorJobSpec validates the job spec for the step executor: it MUST pass ValidateJobSpec
// and MUST have at least one step. The step executor executes steps exactly in order; use this when the
// job is intended for cynode-sse (or any step-executor runner).
func ValidateStepExecutorJobSpec(spec *JobSpec) error {
	if err := ValidateJobSpec(spec); err != nil {
		return err
	}
	if len(spec.Steps) == 0 {
		return &ValidationError{Field: "steps", Message: "required and must be non-empty for step executor"}
	}
	return nil
}

func parseMajorVersion(v string) (int, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, errors.New("empty")
	}
	parts := strings.SplitN(v, ".", 2)
	major, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || major < 0 {
		return 0, errors.New("invalid version format")
	}
	return major, nil
}
