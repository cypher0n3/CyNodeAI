// Package sba implements the SBA agent loop (langchaingo) as the single execution mode.
package sba

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/schema"
)

const (
	defaultOllamaURL = "http://localhost:11434"
	defaultModel     = "llama3.2"

	failureCodeConstraintViolation = "constraint_violation"
	failureCodeStepFailed          = "step_failed"
)

// RunAgentOptions allows overriding the LLM and job dir for todo/artifacts.
type RunAgentOptions struct {
	// LLM overrides the model used when non-nil (e.g. for tests with mock LLM).
	LLM llms.Model
	// JobDir is the job directory (/job/); when set, initial todo is persisted and result.Artifacts from jobDir/artifacts/.
	JobDir string
}

// RunAgent runs the agent loop and returns the result contract.
// When opts.LLM is nil, an Ollama LLM is created from OLLAMA_BASE_URL and spec.Inference.AllowedModels.
func RunAgent(ctx context.Context, spec *sbajob.JobSpec, workspace string, opts *RunAgentOptions) *sbajob.Result {
	if workspace == "" {
		workspace = "/workspace"
	}
	result := &sbajob.Result{
		ProtocolVersion: spec.ProtocolVersion,
		JobID:           spec.JobID,
		Status:          statusSuccess,
		InferenceUsed:   boolPtr(true),
		Steps:           nil,
		Artifacts:       nil,
	}

	llm, err := getLLM(spec, opts)
	if err != nil {
		result.Status = statusFailed
		code := "schema_validation"
		msg := err.Error()
		result.FailureCode = &code
		result.FailureMessage = &msg
		return result
	}

	if opts != nil && opts.JobDir != "" {
		WriteTodo(opts.JobDir, BuildInitialTodo(spec))
	}

	toolEnv := &ToolEnv{
		Workspace:      workspace,
		MaxOutputBytes: spec.Constraints.MaxOutputBytes,
	}
	agentCtx := ContextWithToolEnv(ctx, toolEnv)

	toolsList := NewLocalTools()
	if mcp := NewMCPClient(); mcp.BaseURL != "" {
		toolsList = append(toolsList, NewMCPTool(mcp))
	}
	agent := agents.NewOneShotAgent(llm, toolsList,
		agents.WithMaxIterations(50),
	)
	exec := agents.NewExecutor(agent,
		agents.WithReturnIntermediateSteps(),
		agents.WithMaxIterations(50),
	)

	userPrompt := buildUserPrompt(agentCtx, spec)
	outputs, err := exec.Call(agentCtx, map[string]any{"input": userPrompt})
	if err != nil {
		setResultFromExecErr(result, ctx, err)
		return result
	}
	if final, ok := outputs["output"].(string); ok {
		result.FinalAnswer = strings.TrimSpace(final)
	}
	processStepsToResult(outputs, result)
	if opts != nil && opts.JobDir != "" {
		collectArtifactsToResult(opts.JobDir, result)
	}
	return result
}

func boolPtr(v bool) *bool {
	return &v
}

func setResultFromExecErr(result *sbajob.Result, ctx context.Context, err error) {
	if ctx.Err() == context.DeadlineExceeded || errors.Is(err, context.DeadlineExceeded) {
		result.Status = statusTimeout
		code := statusTimeout
		msg := "job exceeded max_runtime_seconds"
		result.FailureCode = &code
		result.FailureMessage = &msg
		return
	}
	if IsConstraintViolation(err) {
		result.Status = statusFailed
		code := failureCodeConstraintViolation
		msg := err.Error()
		result.FailureCode = &code
		result.FailureMessage = &msg
		return
	}
	if IsExtNetRequired(err) {
		result.Status = statusFailed
		code := "ext_net_required"
		msg := err.Error()
		result.FailureCode = &code
		result.FailureMessage = &msg
		return
	}
	result.Status = statusFailed
	code := failureCodeStepFailed
	msg := err.Error()
	result.FailureCode = &code
	result.FailureMessage = &msg
}

func processStepsToResult(outputs map[string]any, result *sbajob.Result) {
	steps, ok := outputs["intermediateSteps"].([]schema.AgentStep)
	if !ok {
		return
	}
	for i, s := range steps {
		sr := sbajob.StepResult{
			Index:  i,
			Type:   s.Action.Tool,
			Status: statusSuccess,
			Output: s.Observation,
		}
		if strings.HasPrefix(s.Observation, "error: ") {
			sr.Status = statusFailed
			sr.Error = strings.TrimPrefix(s.Observation, "error: ")
			result.Status = statusFailed
			code := failureCodeStepFailed
			msg := sr.Error
			result.FailureCode = &code
			result.FailureMessage = &msg
		}
		result.Steps = append(result.Steps, sr)
	}
}

func collectArtifactsToResult(jobDir string, result *sbajob.Result) {
	artifactsDir := jobDir + "/artifacts"
	entries, err := os.ReadDir(artifactsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			result.Artifacts = append(result.Artifacts, sbajob.ArtifactRef{Path: "artifacts/" + e.Name()})
		}
	}
}

func getLLM(spec *sbajob.JobSpec, opts *RunAgentOptions) (llms.Model, error) {
	if opts != nil && opts.LLM != nil {
		return opts.LLM, nil
	}
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = defaultOllamaURL
	}
	model := defaultModel
	if spec.Inference != nil && len(spec.Inference.AllowedModels) > 0 {
		model = spec.Inference.AllowedModels[0]
	}
	llm, err := ollama.New(
		ollama.WithServerURL(baseURL),
		ollama.WithModel(model),
	)
	if err != nil {
		return nil, fmt.Errorf("create ollama llm: %w", err)
	}
	return llm, nil
}

func buildUserPrompt(ctx context.Context, spec *sbajob.JobSpec) string {
	var b strings.Builder
	appendTimeRemaining(&b, ctx)
	if spec.Context != nil {
		appendContextBlock(&b, spec.Context)
	}
	appendStepsBlock(&b, spec.Steps)
	if b.Len() == 0 {
		return "Complete the task. Use the available tools as needed."
	}
	return strings.TrimSpace(b.String())
}

func appendTimeRemaining(b *strings.Builder, ctx context.Context) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return
	}
	if d := time.Until(deadline); d > 0 {
		fmt.Fprintf(b, "Time remaining: %.0f seconds.\n\n", d.Seconds())
	}
}

func appendContextBlock(b *strings.Builder, c *sbajob.ContextSpec) {
	if c.TaskContext != "" {
		b.WriteString("Task: " + c.TaskContext + "\n\n")
	}
	if c.BaselineContext != "" {
		b.WriteString("Baseline: " + c.BaselineContext + "\n\n")
	}
	if c.ProjectContext != "" {
		b.WriteString("Project: " + c.ProjectContext + "\n\n")
	}
	for _, r := range c.Requirements {
		b.WriteString("- " + r + "\n")
	}
	if len(c.Requirements) > 0 {
		b.WriteString("\n")
	}
	for _, ac := range c.AcceptanceCriteria {
		b.WriteString("Acceptance: " + ac + "\n")
	}
	if c.AdditionalContext != "" {
		b.WriteString("\n" + c.AdditionalContext + "\n")
	}
}

func appendStepsBlock(b *strings.Builder, steps []sbajob.StepSpec) {
	if len(steps) == 0 {
		return
	}
	b.WriteString("\nSuggested steps (you may use tools to accomplish these or similar):\n")
	for i, s := range steps {
		fmt.Fprintf(b, "%d. %s\n", i+1, s.Type)
	}
}
