package sba

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/tmc/langchaingo/schema"
)

func TestRunJob_WrapsRunAgent(t *testing.T) {
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Steps:           nil,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	result := RunJob(ctx, spec, t.TempDir())
	if result == nil {
		t.Fatal("RunJob returned nil")
	}
	if result.Status != statusTimeout {
		t.Errorf("Status = %q (expected timeout with 0 deadline)", result.Status)
	}
}

func TestSetResultFromExecErr_ExtNetRequired(t *testing.T) {
	result := &sbajob.Result{Status: statusSuccess}
	setResultFromExecErr(result, context.Background(), ErrExtNetRequired)
	if result.Status != statusFailed {
		t.Errorf("Status = %q", result.Status)
	}
	if result.FailureCode == nil || *result.FailureCode != "ext_net_required" {
		t.Errorf("FailureCode = %v", result.FailureCode)
	}
}

func TestSetResultFromExecErr_StepFailed(t *testing.T) {
	result := &sbajob.Result{Status: statusSuccess}
	err := &constraintViolationError{msg: "x"}
	setResultFromExecErr(result, context.Background(), err)
	if result.FailureCode == nil || *result.FailureCode != failureCodeConstraintViolation {
		t.Errorf("constraint_violation: FailureCode = %v", result.FailureCode)
	}
	result2 := &sbajob.Result{Status: statusSuccess}
	setResultFromExecErr(result2, context.Background(), context.Canceled)
	if result2.Status != statusFailed {
		t.Errorf("Status = %q", result2.Status)
	}
	if result2.FailureCode == nil || *result2.FailureCode != failureCodeStepFailed {
		t.Errorf("FailureCode = %v", result2.FailureCode)
	}
}

func TestProcessStepsToResult_ErrorObservation(t *testing.T) {
	result := &sbajob.Result{Status: statusSuccess}
	outputs := map[string]any{
		"intermediateSteps": []schema.AgentStep{
			{Action: schema.AgentAction{Tool: "run_command"}, Observation: "ok"},
			{Action: schema.AgentAction{Tool: "read_file"}, Observation: "error: file not found"},
		},
	}
	processStepsToResult(outputs, result)
	if result.Status != statusFailed {
		t.Errorf("Status = %q", result.Status)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("len(Steps) = %d", len(result.Steps))
	}
	if result.Steps[1].Status != statusFailed || result.Steps[1].Error != "file not found" {
		t.Errorf("Steps[1] = %+v", result.Steps[1])
	}
}

func TestProcessStepsToResult_NotSteps_Noop(t *testing.T) {
	result := &sbajob.Result{Status: statusSuccess}
	processStepsToResult(map[string]any{"other": 1}, result)
	if len(result.Steps) != 0 {
		t.Errorf("Steps = %v", result.Steps)
	}
}

func TestBuildUserPrompt_NoDeadline_NoTimeRemaining(t *testing.T) {
	ctx := context.Background()
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Context:         &sbajob.ContextSpec{TaskContext: "Do X", BaselineContext: "Baseline", ProjectContext: "Proj", AcceptanceCriteria: []string{"AC1"}, AdditionalContext: "Extra"},
		Steps:           nil,
	}
	prompt := buildUserPrompt(ctx, spec)
	if prompt == "" {
		t.Fatal("empty prompt")
	}
	if !strings.Contains(prompt, "Task: Do X") || !strings.Contains(prompt, "Baseline") || !strings.Contains(prompt, "Project: Proj") || !strings.Contains(prompt, "Acceptance: AC1") || !strings.Contains(prompt, "Extra") {
		t.Errorf("prompt missing context: %s", prompt)
	}
}

func TestBuildUserPrompt_Empty_ReturnsDefault(t *testing.T) {
	prompt := buildUserPrompt(context.Background(), &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Steps:           nil,
	})
	if prompt != "Complete the task. Use the available tools as needed." {
		t.Errorf("prompt = %q", prompt)
	}
}

func TestBuildUserPrompt_WithSteps_IncludesSuggestedSteps(t *testing.T) {
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Steps:           []sbajob.StepSpec{{Type: "run_command"}, {Type: "read_file"}},
	}
	prompt := buildUserPrompt(context.Background(), spec)
	if !strings.Contains(prompt, "Suggested steps") || !strings.Contains(prompt, "run_command") || !strings.Contains(prompt, "read_file") {
		t.Errorf("prompt = %s", prompt)
	}
}

func TestAppendTimeRemaining_PastDeadline_NoOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), -1)
	defer cancel()
	var b strings.Builder
	appendTimeRemaining(&b, ctx)
	if b.Len() != 0 {
		t.Errorf("expected no output for past deadline, got %q", b.String())
	}
}

func TestCollectArtifactsToResult_SkipsDirs(t *testing.T) {
	dir := t.TempDir()
	artifactsDir := dir + "/artifacts"
	_ = os.MkdirAll(artifactsDir, 0o755)
	_ = os.WriteFile(artifactsDir+"/f.txt", []byte("x"), 0o644)
	_ = os.Mkdir(artifactsDir+"/sub", 0o755)
	result := &sbajob.Result{}
	collectArtifactsToResult(dir, result)
	if len(result.Artifacts) != 1 || result.Artifacts[0].Path != "artifacts/f.txt" {
		t.Errorf("Artifacts = %v (expected only file, not subdir)", result.Artifacts)
	}
}
