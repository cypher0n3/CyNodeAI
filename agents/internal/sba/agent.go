// Package sba implements the SBA agent loop (langchaingo) as the single execution mode.
package sba

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
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
	// udsRewrittenServerURL is the server URL returned when using http+unix; HTTP libs expect a host.
	udsRewrittenServerURL = "http://localhost"
	defaultModel          = "llama3.2"

	failureCodeConstraintViolation = "constraint_violation"
	failureCodeStepFailed          = "step_failed"

	// inferenceHTTPTimeout is the HTTP client timeout for inference requests.
	// Set generously to accommodate slow local models while still bounding runaway requests.
	inferenceHTTPTimeout = 300 * time.Second
)

// smallModelSuffixes lists parameter-count suffixes that indicate a model too small
// to reliably produce ReAct-style structured output. These use direct generation
// (single Ollama call, no agent loop) to avoid langchaingo streaming issues with
// Qwen3.5's thinking mode (empty message.content in streaming chunks).
var smallModelSuffixes = []string{
	":0.5b", ":0.6b", ":0.8b", ":1b", ":1.1b", ":1.5b", ":1.8b",
}

// isSmallModel returns true when the named model is too small for the ReAct agent loop.
func isSmallModel(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, suffix := range smallModelSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	// Models not in any known capable family default to direct generation.
	capablePrefixes := []string{
		"qwen3.5:", "qwen3.5", "qwen3:", "qwen3", "qwen2.5:", "qwen2.5",
		"llama3.", "llama-3.", "mistral", "mixtral",
	}
	for _, p := range capablePrefixes {
		if strings.HasPrefix(lower, p) {
			return false
		}
	}
	return true
}

// RunAgentOptions allows overriding the LLM and job dir for todo/artifacts.
type RunAgentOptions struct {
	// LLM overrides the model used when non-nil (e.g. for tests with mock LLM).
	LLM llms.Model
	// JobDir is the job directory (/job/); when set, initial todo is persisted and result.Artifacts from jobDir/artifacts/.
	JobDir string
}

// resolveInferenceURL returns the effective inference base URL and an http.Client.
// Priority: INFERENCE_PROXY_URL (UDS) > OLLAMA_BASE_URL > defaultOllamaURL.
// For http+unix:// URLs the returned client dials the Unix socket; the returned
// serverURL is rewritten to http://localhost so standard HTTP libraries can use it.
func resolveInferenceURL() (serverURL string, client *http.Client) {
	raw := os.Getenv("INFERENCE_PROXY_URL")
	if raw == "" {
		raw = os.Getenv("OLLAMA_BASE_URL")
	}
	if raw == "" {
		raw = defaultOllamaURL
	}
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "http+unix://") {
		encoded := strings.TrimPrefix(trimmed, "http+unix://")
		if idx := strings.Index(encoded, "/"); idx > 0 {
			encoded = encoded[:idx]
		}
		if sockPath, err := url.PathUnescape(encoded); err == nil && sockPath != "" {
			transport := &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
				},
			}
			return udsRewrittenServerURL, &http.Client{Timeout: inferenceHTTPTimeout, Transport: transport}
		}
	}
	return trimmed, &http.Client{Timeout: inferenceHTTPTimeout}
}

// RunAgent runs the agent loop and returns the result contract.
// When opts.LLM is nil, an Ollama LLM is created from INFERENCE_PROXY_URL or OLLAMA_BASE_URL and spec.Inference.AllowedModels.
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

	// Determine model for routing decision before creating the LLM.
	modelForRouting := defaultModel
	if envModel := os.Getenv("INFERENCE_MODEL"); envModel != "" {
		modelForRouting = envModel
	}
	if spec.Inference != nil && len(spec.Inference.AllowedModels) > 0 {
		modelForRouting = spec.Inference.AllowedModels[0]
	}

	if isSmallModel(modelForRouting) {
		// Small models (e.g. qwen3.5:0.8b) cannot produce ReAct output reliably.
		// Langchaingo's streaming accumulation also returns empty content for
		// Qwen3.5 thinking-mode responses. Use direct Ollama /api/chat call.
		return runDirectGeneration(ctx, spec, opts, workspace, result, modelForRouting)
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

// runDirectGeneration handles small models by calling Ollama /api/chat directly
// (stream:false explicitly), bypassing the langchaingo agent loop.
func runDirectGeneration(ctx context.Context, spec *sbajob.JobSpec, opts *RunAgentOptions, workspace string, result *sbajob.Result, model string) *sbajob.Result {
	if opts != nil && opts.LLM != nil {
		// Test hook: use GenerateFromSinglePrompt on the mock LLM.
		content, err := llms.GenerateFromSinglePrompt(ctx, opts.LLM, buildUserPromptFromSpec(spec))
		if err != nil {
			result.Status = statusFailed
			code := failureCodeStepFailed
			msg := err.Error()
			result.FailureCode = &code
			result.FailureMessage = &msg
			return result
		}
		result.FinalAnswer = strings.TrimSpace(content)
		result.InferenceUsed = boolPtr(true)
		return result
	}
	serverURL, client := resolveInferenceURL()
	prompt := buildUserPromptFromSpec(spec)
	payload, _ := json.Marshal(map[string]interface{}{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
		"stream":   false,
	})
	reqURL := strings.TrimSuffix(serverURL, "/") + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		result.Status = statusFailed
		code := failureCodeStepFailed
		msg := fmt.Sprintf("direct gen request: %v", err)
		result.FailureCode = &code
		result.FailureMessage = &msg
		return result
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		result.Status = statusFailed
		code := failureCodeStepFailed
		msg := fmt.Sprintf("direct gen http: %v", err)
		result.FailureCode = &code
		result.FailureMessage = &msg
		return result
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		result.Status = statusFailed
		code := failureCodeStepFailed
		msg := fmt.Sprintf("direct gen decode: %v", err)
		result.FailureCode = &code
		result.FailureMessage = &msg
		return result
	}
	if out.Error != "" {
		result.Status = statusFailed
		code := failureCodeStepFailed
		result.FailureCode = &code
		result.FailureMessage = &out.Error
		return result
	}
	result.FinalAnswer = strings.TrimSpace(out.Message.Content)
	result.InferenceUsed = boolPtr(true)
	return result
}

// buildUserPromptFromSpec returns a simple text prompt from the job spec context.
func buildUserPromptFromSpec(spec *sbajob.JobSpec) string {
	if spec.Context != nil && spec.Context.TaskContext != "" {
		return spec.Context.TaskContext
	}
	return ""
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
	if answer := salvageFinalAnswerFromParseError(err); answer != "" {
		// Some small models emit non-ReAct text; salvage it as user-facing answer.
		result.Status = statusSuccess
		result.FinalAnswer = answer
		return
	}
	result.Status = statusFailed
	code := failureCodeStepFailed
	msg := err.Error()
	result.FailureCode = &code
	result.FailureMessage = &msg
}

func salvageFinalAnswerFromParseError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if !isAgentOutputParseError(err) {
		return ""
	}
	const prefix = "unable to parse agent output:"
	lower := strings.ToLower(msg)
	idx := strings.Index(lower, prefix)
	if idx < 0 || idx+len(prefix) >= len(msg) {
		return ""
	}
	raw := strings.TrimSpace(msg[idx+len(prefix):])
	return raw
}

func isAgentOutputParseError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "unable to parse agent output:")
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
	serverURL, httpClient := resolveInferenceURL()
	// Model priority: job spec AllowedModels > INFERENCE_MODEL env > defaultModel.
	model := os.Getenv("INFERENCE_MODEL")
	if model == "" {
		model = defaultModel
	}
	if spec.Inference != nil && len(spec.Inference.AllowedModels) > 0 {
		model = spec.Inference.AllowedModels[0]
	}
	ollamaOpts := []ollama.Option{
		ollama.WithServerURL(serverURL),
		ollama.WithModel(model),
		ollama.WithHTTPClient(httpClient),
	}
	llm, err := ollama.New(ollamaOpts...)
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
	appendAgentOutputContract(&b)
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

func appendAgentOutputContract(b *strings.Builder) {
	b.WriteString("\nRequired response format (must follow exactly):\n")
	b.WriteString("- If you can answer directly: Final Answer: <your answer>\n")
	b.WriteString("- If you need a tool:\n")
	b.WriteString("  Action: <tool_name>\n")
	b.WriteString("  Action Input: <valid JSON object>\n")
}
