package sba

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
)

// mockLLM is a minimal llms.Model for unit tests.
type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return m.response, m.err
}

func (m *mockLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: m.response}},
	}, nil
}

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

func TestSetResultFromExecErr_SalvageParseError(t *testing.T) {
	result := &sbajob.Result{Status: statusFailed}
	err := errors.New("unable to parse agent output: Final answer: hello")
	setResultFromExecErr(result, context.Background(), err)
	if result.Status != statusSuccess {
		t.Fatalf("status=%q, want success", result.Status)
	}
	if !strings.Contains(result.FinalAnswer, "hello") {
		t.Fatalf("final_answer=%q", result.FinalAnswer)
	}
}

func TestIsAgentOutputParseError(t *testing.T) {
	if isAgentOutputParseError(nil) {
		t.Fatal("nil error must not be parse error")
	}
	if !isAgentOutputParseError(errors.New("unable to parse agent output:")) {
		t.Fatal("expected parse error to be detected")
	}
	if isAgentOutputParseError(errors.New("other failure")) {
		t.Fatal("unexpected parse error detection")
	}
}

func TestBuildUserPrompt_IncludesOutputContract(t *testing.T) {
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Context:         &sbajob.ContextSpec{TaskContext: "Reply with one word."},
	}
	prompt := buildUserPrompt(context.Background(), spec)
	if !strings.Contains(prompt, "Final Answer: <your answer>") {
		t.Fatalf("prompt missing Final Answer contract: %q", prompt)
	}
	if !strings.Contains(prompt, "Action Input: <valid JSON object>") {
		t.Fatalf("prompt missing Action Input contract: %q", prompt)
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

func TestIsSmallModel(t *testing.T) {
	cases := []struct {
		model string
		small bool
	}{
		{"qwen3.5:0.8b", true},
		{"qwen3:1b", true},
		{"tinyllama", true},
		{"phi3:mini", true},
		{"qwen3.5:9b", false},
		{"qwen3:8b", false},
		{"qwen2.5:14b", false},
		{"llama3.3:70b", false},
		{"mistral:7b", false},
		{"mixtral:8x7b", false},
		{"llama3.2:3b", false},
		{"", true},
	}
	for _, c := range cases {
		if got := isSmallModel(c.model); got != c.small {
			t.Errorf("isSmallModel(%q) = %v, want %v", c.model, got, c.small)
		}
	}
}

func TestBuildUserPromptFromSpec(t *testing.T) {
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Context:         &sbajob.ContextSpec{TaskContext: "do something"},
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
	}
	if got := buildUserPromptFromSpec(spec); got != "do something" {
		t.Errorf("got %q", got)
	}
	empty := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
	}
	if got := buildUserPromptFromSpec(empty); got != "" {
		t.Errorf("empty spec prompt = %q", got)
	}
}

func TestRunDirectGeneration_WithMockLLM(t *testing.T) {
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Context:         &sbajob.ContextSpec{TaskContext: "echo hello"},
	}
	result := &sbajob.Result{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		Status:          statusSuccess,
		InferenceUsed:   boolPtr(true),
	}
	opts := &RunAgentOptions{LLM: &mockLLM{response: "hello from direct gen"}}
	out := runDirectGeneration(context.Background(), spec, opts, t.TempDir(), result, "qwen3.5:0.8b")
	if out.Status != statusSuccess {
		t.Errorf("status=%q, want success", out.Status)
	}
	if out.FinalAnswer != "hello from direct gen" {
		t.Errorf("FinalAnswer=%q", out.FinalAnswer)
	}
}

func TestRunDirectGeneration_MockLLMError(t *testing.T) {
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
	}
	result := &sbajob.Result{Status: statusSuccess, InferenceUsed: boolPtr(true)}
	opts := &RunAgentOptions{LLM: &mockLLM{err: errors.New("llm failed")}}
	out := runDirectGeneration(context.Background(), spec, opts, t.TempDir(), result, "qwen3.5:0.8b")
	if out.Status != statusFailed {
		t.Errorf("status=%q, want failed", out.Status)
	}
}

func TestRunDirectGeneration_HTTPError(t *testing.T) {
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
	}
	result := &sbajob.Result{Status: statusSuccess, InferenceUsed: boolPtr(true)}
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:19997") // unreachable
	out := runDirectGeneration(context.Background(), spec, nil, t.TempDir(), result, "qwen3.5:0.8b")
	if out.Status != statusFailed {
		t.Errorf("status=%q, want failed", out.Status)
	}
}

func TestRunDirectGeneration_OllamaErrorField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
	}
	result := &sbajob.Result{Status: statusSuccess, InferenceUsed: boolPtr(true)}
	out := runDirectGeneration(context.Background(), spec, nil, t.TempDir(), result, "qwen3.5:0.8b")
	if out.Status != statusFailed {
		t.Errorf("status=%q, want failed", out.Status)
	}
	if out.FailureMessage == nil || *out.FailureMessage != "model not found" {
		t.Errorf("FailureMessage=%v", out.FailureMessage)
	}
}

func TestRunDirectGeneration_OllamaSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"content":"direct ok"},"done":true}`))
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Context:         &sbajob.ContextSpec{TaskContext: "echo test"},
	}
	result := &sbajob.Result{Status: statusSuccess, InferenceUsed: boolPtr(true)}
	out := runDirectGeneration(context.Background(), spec, nil, t.TempDir(), result, "qwen3.5:0.8b")
	if out.Status != statusSuccess {
		t.Errorf("status=%q, want success", out.Status)
	}
	if out.FinalAnswer != "direct ok" {
		t.Errorf("FinalAnswer=%q", out.FinalAnswer)
	}
}

func TestRunDirectGeneration_BadJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
	}
	result := &sbajob.Result{Status: statusSuccess, InferenceUsed: boolPtr(true)}
	out := runDirectGeneration(context.Background(), spec, nil, t.TempDir(), result, "qwen3.5:0.8b")
	if out.Status != statusFailed {
		t.Errorf("status=%q, want failed", out.Status)
	}
}

func defaultJobSpec(allowedModel string) *sbajob.JobSpec {
	return &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Inference:       &sbajob.InferenceSpec{AllowedModels: []string{allowedModel}},
		Context:         &sbajob.ContextSpec{TaskContext: "echo hello"},
	}
}

func TestRunAgent_SmallModelUsesDirectGeneration(t *testing.T) {
	spec := defaultJobSpec("qwen3.5:0.8b")
	opts := &RunAgentOptions{LLM: &mockLLM{response: "direct answer"}}
	result := RunAgent(context.Background(), spec, t.TempDir(), opts)
	if result.Status != statusSuccess {
		t.Errorf("status=%q, want success", result.Status)
	}
	if result.FinalAnswer != "direct answer" {
		t.Errorf("FinalAnswer=%q", result.FinalAnswer)
	}
}

func TestRunAgent_CapableModel_LLMError(t *testing.T) {
	// When no test LLM and Ollama is unreachable, RunAgent with a capable model
	// should fail gracefully (exec.Call returns error when context cancelled).
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:19996")
	spec := defaultJobSpec("qwen3:8b")
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()
	result := RunAgent(ctx, spec, t.TempDir(), nil)
	if result.Status == statusSuccess {
		t.Errorf("expected non-success when Ollama unreachable")
	}
}

func TestSalvageFinalAnswer_EmptySuffix(t *testing.T) {
	// When parse error suffix is empty, salvage returns "".
	err := errors.New("unable to parse agent output: ")
	if got := salvageFinalAnswerFromParseError(err); got != "" {
		t.Errorf("got %q, want empty for whitespace-only suffix", got)
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
