package sbajob

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const (
	fieldProtocolVersion = "protocol_version"
	fieldJobID           = "job_id"
	fieldTaskID          = "task_id"
)

// parseAndExpectValidationError calls ParseAndValidateJobSpec and asserts the error is a ValidationError.
// If wantField is non-empty, asserts ve.Field == wantField. If msgSubstr is non-empty, asserts Message contains it.
func parseAndExpectValidationError(t *testing.T, data []byte, wantField, msgSubstr string) {
	t.Helper()
	_, err := ParseAndValidateJobSpec(data)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if wantField != "" && ve.Field != wantField {
		t.Errorf("Field = %q, want %q", ve.Field, wantField)
	}
	if msgSubstr != "" && !strings.Contains(ve.Message, msgSubstr) {
		t.Errorf("Message %q does not contain %q", ve.Message, msgSubstr)
	}
}

// parseAndExpectValidationErrorFieldContains asserts validation fails and ve.Field contains fieldSubstr.
func parseAndExpectValidationErrorFieldContains(t *testing.T, data []byte, fieldSubstr string) {
	t.Helper()
	_, err := ParseAndValidateJobSpec(data)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if !strings.Contains(ve.Field, fieldSubstr) {
		t.Errorf("Field %q does not contain %q", ve.Field, fieldSubstr)
	}
}

func TestParseAndValidateJobSpec_Valid(t *testing.T) {
	data := []byte(`{
		"protocol_version": "1.0",
		"job_id": "j1",
		"task_id": "t1",
		"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
		"steps": []
	}`)
	spec, err := ParseAndValidateJobSpec(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.JobID != "j1" || spec.TaskID != "t1" || spec.ProtocolVersion != "1.0" {
		t.Errorf("spec fields wrong: %+v", spec)
	}
	if spec.Constraints.MaxRuntimeSeconds != 300 || spec.Constraints.MaxOutputBytes != 1048576 {
		t.Errorf("constraints wrong: %+v", spec.Constraints)
	}
}

func TestParseAndValidateJobSpec_StepsOptional(t *testing.T) {
	data := []byte(`{
		"protocol_version": "1.0",
		"job_id": "j1",
		"task_id": "t1",
		"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576}
	}`)
	spec, err := ParseAndValidateJobSpec(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Steps != nil {
		t.Errorf("Steps should be nil when omitted, got len=%d", len(spec.Steps))
	}
	if EffectiveExecutionMode(spec) != ExecutionModeAgentInference {
		t.Errorf("EffectiveExecutionMode() = %q, want %q", EffectiveExecutionMode(spec), ExecutionModeAgentInference)
	}
}

func TestValidateStepExecutorJobSpec_EmptyStepsFails(t *testing.T) {
	spec := &JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     JobConstraints{MaxRuntimeSeconds: 300, MaxOutputBytes: 1048576},
		Steps:           nil,
	}
	err := ValidateStepExecutorJobSpec(spec)
	if err == nil {
		t.Fatal("expected error for nil steps")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "steps" {
		t.Errorf("expected steps ValidationError, got %v", err)
	}
	spec.Steps = []StepSpec{}
	err = ValidateStepExecutorJobSpec(spec)
	if err == nil {
		t.Fatal("expected error for empty steps")
	}
}

func TestValidateStepExecutorJobSpec_Valid(t *testing.T) {
	spec := &JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     JobConstraints{MaxRuntimeSeconds: 300, MaxOutputBytes: 1048576},
		Steps:           []StepSpec{{Type: "run_command", Args: json.RawMessage(`{"argv":["echo","hi"]}`)}},
	}
	if err := ValidateStepExecutorJobSpec(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAndValidateJobSpec_InvalidExecutionMode(t *testing.T) {
	data := []byte(`{
		"protocol_version": "1.0",
		"job_id": "j1",
		"task_id": "t1",
		"execution_mode": "invalid_mode",
		"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
		"steps": []
	}`)
	parseAndExpectValidationError(t, data, "execution_mode", "must be one of")
}

func TestEffectiveExecutionMode_BackwardCompatibleFallback(t *testing.T) {
	spec := &JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Steps:           []StepSpec{{Type: "run_command"}},
	}
	if got := EffectiveExecutionMode(spec); got != ExecutionModeDirectSteps {
		t.Errorf("EffectiveExecutionMode(with steps) = %q, want %q", got, ExecutionModeDirectSteps)
	}
	spec.ExecutionMode = ExecutionModeAgentInference
	if got := EffectiveExecutionMode(spec); got != ExecutionModeAgentInference {
		t.Errorf("EffectiveExecutionMode(explicit) = %q, want %q", got, ExecutionModeAgentInference)
	}
}

func TestParseAndValidateJobSpec_UnknownMajorVersion(t *testing.T) {
	data := []byte(`{
		"protocol_version": "99.0",
		"job_id": "j1",
		"task_id": "t1",
		"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
		"steps": []
	}`)
	parseAndExpectValidationError(t, data, fieldProtocolVersion, "unsupported")
}

func TestParseAndValidateJobSpec_UnknownFieldRejected(t *testing.T) {
	data := []byte(`{
		"protocol_version": "1.0",
		"job_id": "j1",
		"task_id": "t1",
		"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
		"steps": [],
		"unknown_field": "x"
	}`)
	parseAndExpectValidationError(t, data, "", "unknown")
}

func TestParseAndValidateJobSpec_MissingRequired(t *testing.T) {
	data := []byte(`{
		"protocol_version": "1.0",
		"job_id": "",
		"task_id": "t1",
		"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
		"steps": []
	}`)
	parseAndExpectValidationError(t, data, fieldJobID, "")
}

func TestParseAndValidateJobSpec_InvalidConstraints(t *testing.T) {
	data := []byte(`{
		"protocol_version": "1.0",
		"job_id": "j1",
		"task_id": "t1",
		"constraints": {"max_runtime_seconds": 0, "max_output_bytes": 1048576},
		"steps": []
	}`)
	parseAndExpectValidationErrorFieldContains(t, data, "max_runtime")
}

func TestValidateJobSpec_Nil(t *testing.T) {
	err := ValidateJobSpec(nil)
	if err == nil {
		t.Fatal("expected error for nil spec")
	}
}

func TestResult_MarshalRoundTrip(t *testing.T) {
	code := 1
	r := Result{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		Status:          "failure",
		Steps:           []StepResult{{Index: 0, Type: "run_command", Status: "failed", ExitCode: &code}},
		Artifacts:       nil,
		FailureCode:     strPtr("step_failed"),
		FailureMessage:  strPtr("step 0 failed"),
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var r2 Result
	if err := json.Unmarshal(b, &r2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r2.JobID != r.JobID || r2.Status != r.Status {
		t.Errorf("round trip mismatch: %+v", r2)
	}
	if r2.FailureCode == nil || *r2.FailureCode != "step_failed" {
		t.Errorf("failure_code: %v", r2.FailureCode)
	}
}

func TestParseAndValidateJobSpec_EmptyProtocolVersion(t *testing.T) {
	data := []byte(`{
		"protocol_version": "",
		"job_id": "j1",
		"task_id": "t1",
		"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
		"steps": []
	}`)
	parseAndExpectValidationError(t, data, fieldProtocolVersion, "")
}

func TestParseAndValidateJobSpec_MissingTaskID(t *testing.T) {
	data := []byte(`{
		"protocol_version": "1.0",
		"job_id": "j1",
		"task_id": "",
		"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
		"steps": []
	}`)
	parseAndExpectValidationError(t, data, fieldTaskID, "")
}

func TestParseAndValidateJobSpec_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid`)
	_, err := ParseAndValidateJobSpec(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseAndValidateJobSpec_MaxOutputBytesZero(t *testing.T) {
	data := []byte(`{
		"protocol_version": "1.0",
		"job_id": "j1",
		"task_id": "t1",
		"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 0},
		"steps": []
	}`)
	parseAndExpectValidationErrorFieldContains(t, data, "max_output")
}

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{Field: fieldJobID, Message: "required"}
	if got := ve.Error(); !strings.Contains(got, fieldJobID) || !strings.Contains(got, "required") {
		t.Errorf("Error() = %q", got)
	}
	ve2 := &ValidationError{Message: "invalid"}
	if ve2.Error() != "invalid" {
		t.Errorf("Error() without field = %q", ve2.Error())
	}
}

func TestParseAndValidateJobSpec_InvalidVersionFormat(t *testing.T) {
	data := []byte(`{
		"protocol_version": "x.y",
		"job_id": "j1",
		"task_id": "t1",
		"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
		"steps": []
	}`)
	parseAndExpectValidationError(t, data, fieldProtocolVersion, "")
}

func TestParseMajorVersion(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"1.0", 1, true},
		{"1", 1, true},
		{"2.0", 2, true},
		{"99.1", 99, true},
		{"0.1", 0, true},
		{"", 0, false},
		{"x", 0, false},
		{"1.2.3", 1, true},
	}
	for _, tt := range tests {
		got, err := parseMajorVersion(tt.in)
		if (err == nil) != tt.ok {
			t.Errorf("parseMajorVersion(%q): err=%v want ok=%v", tt.in, err, tt.ok)
			continue
		}
		if tt.ok && got != tt.want {
			t.Errorf("parseMajorVersion(%q)=%d want %d", tt.in, got, tt.want)
		}
	}
}

func strPtr(s string) *string { return &s }
