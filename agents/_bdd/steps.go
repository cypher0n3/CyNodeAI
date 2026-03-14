// Package bdd provides Godog step definitions for the agents suite.
// Feature files live under repo features/agents/.
package bdd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cucumber/godog"

	"github.com/cypher0n3/cynodeai/agents/internal/pma"
	"github.com/cypher0n3/cynodeai/agents/internal/sba"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"log/slog"
)

type agentsTestState struct {
	// SBA job spec and validation (contract scenarios)
	sbaJobSpecBytes       []byte
	sbaValidationErr      error
	sbaValidationErrField string
	sbaResult             *sbajob.Result
	sbaResultJSON         []byte
	// SBA runner (execution scenarios)
	jobDir        string
	resultPath    string
	runnerErr     error
	result        *sbajob.Result
	lastRunCmd    string // for mock: one run_command step argv
	lastApplyDiff string // for mock: apply_unified_diff diff body (escaping)
	stdinJobJSON  string // for stdin mode: raw job JSON
	resultOutput  string // for stdin mode: result JSON written to stdout
	// Lifecycle (callback server)
	lifecycleServer   *httptest.Server
	lifecycleStatuses []string
	lifecycleMu       sync.Mutex
	// PMA internal chat completion
	pmaRequestJSON    []byte
	pmaMockInference  *httptest.Server
	pmaCapturedPrompt string
	pmaResponseStatus int
	pmaResponseBody   []byte
	pmaOldOllamaURL   string // restored in After when mock was used
	// Task result contract scenario (SBA result from task result)
	taskResultJSON []byte
	taskStatus     string
	firstJobResult map[string]interface{}
}

// InitializeAgentsSuite sets up the godog suite for agents features.
func InitializeAgentsSuite(sc *godog.ScenarioContext, state *agentsTestState) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		state.sbaJobSpecBytes = nil
		state.sbaValidationErr = nil
		state.sbaValidationErrField = ""
		state.sbaResult = nil
		state.sbaResultJSON = nil
		state.jobDir = ""
		state.resultPath = ""
		state.runnerErr = nil
		state.result = nil
		state.lastRunCmd = ""
		state.lastApplyDiff = ""
		state.stdinJobJSON = ""
		state.resultOutput = ""
		if state.lifecycleServer != nil {
			state.lifecycleServer.Close()
			state.lifecycleServer = nil
		}
		state.lifecycleStatuses = nil
		state.pmaRequestJSON = nil
		if state.pmaMockInference != nil {
			state.pmaMockInference.Close()
			state.pmaMockInference = nil
		}
		state.pmaCapturedPrompt = ""
		state.pmaResponseStatus = 0
		state.pmaResponseBody = nil
		if state.pmaOldOllamaURL != "" {
			os.Setenv("OLLAMA_BASE_URL", state.pmaOldOllamaURL)
		} else if state.pmaMockInference != nil {
			os.Unsetenv("OLLAMA_BASE_URL")
		}
		state.pmaOldOllamaURL = ""
		state.taskResultJSON = nil
		state.taskStatus = ""
		state.firstJobResult = nil
		return ctx, nil
	})

	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		if state.jobDir != "" {
			_ = os.RemoveAll(state.jobDir)
		}
		return ctx, nil
	})

	registerSBAContractSteps(sc, state)
	registerSBARunnerSteps(sc, state)
	registerSBALifecycleSteps(sc, state)
	registerPMASteps(sc, state)
}

// registerSBAContractSteps registers steps for SBA job spec and result contract validation.
func registerSBAContractSteps(sc *godog.ScenarioContext, state *agentsTestState) {
	sc.Step(`^I have a SBA job spec with protocol_version "([^"]*)" and required fields$`, func(ctx context.Context, pv string) error {
		state.sbaJobSpecBytes = []byte(fmt.Sprintf(`{
			"protocol_version": %q,
			"job_id": "j1",
			"task_id": "t1",
			"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
			"steps": []
		}`, pv))
		return nil
	})
	sc.Step(`^I have a SBA job spec with an unknown field$`, func(ctx context.Context) error {
		state.sbaJobSpecBytes = []byte(`{
			"protocol_version": "1.0",
			"job_id": "j1",
			"task_id": "t1",
			"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
			"steps": [],
			"unknown_field": "x"
		}`)
		return nil
	})
	sc.Step(`^I have a SBA job spec with protocol_version "([^"]*)" and empty job_id$`, func(ctx context.Context, pv string) error {
		state.sbaJobSpecBytes = []byte(fmt.Sprintf(`{
			"protocol_version": %q,
			"job_id": "",
			"task_id": "t1",
			"constraints": {"max_runtime_seconds": 300, "max_output_bytes": 1048576},
			"steps": []
		}`, pv))
		return nil
	})
	sc.Step(`^I validate the SBA job spec$`, func(ctx context.Context) error {
		state.sbaValidationErr = nil
		state.sbaValidationErrField = ""
		_, err := sbajob.ParseAndValidateJobSpec(state.sbaJobSpecBytes)
		if err != nil {
			state.sbaValidationErr = err
			var ve *sbajob.ValidationError
			if errors.As(err, &ve) {
				state.sbaValidationErrField = ve.Field
			}
		}
		return nil
	})
	sc.Step(`^the SBA job spec validation succeeds$`, func(ctx context.Context) error {
		if state.sbaValidationErr != nil {
			return fmt.Errorf("validation should have succeeded: %w", state.sbaValidationErr)
		}
		return nil
	})
	sc.Step(`^the SBA job spec validation fails$`, func(ctx context.Context) error {
		if state.sbaValidationErr == nil {
			return fmt.Errorf("validation should have failed")
		}
		return nil
	})
	sc.Step(`^the validation error is for field "([^"]*)"$`, func(ctx context.Context, field string) error {
		if state.sbaValidationErr == nil {
			return fmt.Errorf("no validation error to check")
		}
		if state.sbaValidationErrField != field {
			return fmt.Errorf("validation error field %q, want %q", state.sbaValidationErrField, field)
		}
		return nil
	})
	sc.Step(`^I have a SBA result with status "([^"]*)" and job_id "([^"]*)"$`, func(ctx context.Context, status, jobID string) error {
		state.sbaResult = &sbajob.Result{
			ProtocolVersion: "1.0",
			JobID:           jobID,
			Status:          status,
			Steps:           []sbajob.StepResult{},
			Artifacts:       []sbajob.ArtifactRef{},
		}
		return nil
	})
	sc.Step(`^I marshal the SBA result to JSON$`, func(ctx context.Context) error {
		if state.sbaResult == nil {
			return fmt.Errorf("no SBA result set")
		}
		var err error
		state.sbaResultJSON, err = json.Marshal(state.sbaResult)
		return err
	})
	sc.Step(`^the JSON contains "([^"]*)"$`, func(ctx context.Context, key string) error {
		if len(state.sbaResultJSON) == 0 {
			return fmt.Errorf("no JSON to check")
		}
		var m map[string]interface{}
		if err := json.Unmarshal(state.sbaResultJSON, &m); err != nil {
			return err
		}
		if _, ok := m[key]; !ok {
			return fmt.Errorf("JSON does not contain key %q", key)
		}
		return nil
	})

	// SBA result contract from task result (mock task result; no orchestrator in agents suite)
	sc.Step(`^I have a completed task that used the SBA runner$`, func(ctx context.Context) error {
		sbaResult := map[string]interface{}{
			"protocol_version": "1.0",
			"job_id":           "j1",
			"status":           "success",
			"steps":            []interface{}{},
			"artifacts":        []interface{}{},
		}
		jobResult := map[string]interface{}{
			"stdout":     "",
			"exit_code":  0,
			"sba_result": sbaResult,
		}
		jobResultBytes, _ := json.Marshal(jobResult)
		jobResultStr := string(jobResultBytes)
		taskResult := map[string]interface{}{
			"task_id": "t1",
			"status":  "completed",
			"jobs":    []interface{}{map[string]interface{}{"id": "j1", "status": "completed", "result": jobResultStr}},
		}
		var err error
		state.taskResultJSON, err = json.Marshal(taskResult)
		return err
	})
	sc.Step(`^I get the task result and extract the first job result$`, func(ctx context.Context) error {
		if len(state.taskResultJSON) == 0 {
			return fmt.Errorf("no task result in state (run I have a completed task that used the SBA runner first)")
		}
		var taskResult struct {
			Status string `json:"status"`
			Jobs   []struct {
				Result *string `json:"result"`
			} `json:"jobs"`
		}
		if err := json.Unmarshal(state.taskResultJSON, &taskResult); err != nil {
			return err
		}
		state.taskStatus = taskResult.Status
		if len(taskResult.Jobs) == 0 || taskResult.Jobs[0].Result == nil {
			return fmt.Errorf("task result has no jobs or first job has no result")
		}
		if err := json.Unmarshal([]byte(*taskResult.Jobs[0].Result), &state.firstJobResult); err != nil {
			return err
		}
		return nil
	})
	sc.Step(`^the task status is "([^"]*)"$`, func(ctx context.Context, want string) error {
		if state.taskStatus != want {
			return fmt.Errorf("task status %q, want %q", state.taskStatus, want)
		}
		return nil
	})
	sc.Step(`^the job result contains "([^"]*)"$`, func(ctx context.Context, key string) error {
		if state.firstJobResult == nil {
			return fmt.Errorf("no job result in state (run I get the task result and extract the first job result first)")
		}
		if _, ok := state.firstJobResult[key]; !ok {
			return fmt.Errorf("job result does not contain key %q", key)
		}
		return nil
	})
	sc.Step(`^the sba_result contains "([^"]*)"$`, func(ctx context.Context, key string) error {
		if state.firstJobResult == nil {
			return fmt.Errorf("no job result in state")
		}
		sbaRaw, ok := state.firstJobResult["sba_result"]
		if !ok {
			return fmt.Errorf("job result has no sba_result")
		}
		sbaMap, ok := sbaRaw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("sba_result is not an object")
		}
		if _, ok := sbaMap[key]; !ok {
			return fmt.Errorf("sba_result does not contain key %q", key)
		}
		return nil
	})
}

// registerSBARunnerSteps registers steps for cynode-sba runner execution (in-process RunJob).
func registerSBARunnerSteps(sc *godog.ScenarioContext, state *agentsTestState) {
	sc.Step(`^I have a valid job file with one run_command step "([^"]*)"$`, func(ctx context.Context, command string) error {
		dir, err := os.MkdirTemp("", "agents_bdd_job_")
		if err != nil {
			return err
		}
		state.jobDir = dir
		state.lastRunCmd = command
		argv, _ := json.Marshal([]string{"sh", "-c", command})
		job := fmt.Sprintf(`{
			"protocol_version": "1.0",
			"job_id": "j1",
			"task_id": "t1",
			"constraints": {"max_runtime_seconds": 60, "max_output_bytes": 1024},
			"steps": [{"type": "run_command", "args": {"argv": %s}}]
		}`, string(argv))
		jobPath := filepath.Join(dir, "job.json")
		return os.WriteFile(jobPath, []byte(job), 0644)
	})
	sc.Step(`^I run the SBA runner$`, func(ctx context.Context) error {
		if state.jobDir == "" {
			return fmt.Errorf("no job dir set")
		}
		jobPath := filepath.Join(state.jobDir, "job.json")
		data, err := os.ReadFile(jobPath)
		if err != nil {
			state.runnerErr = err
			return nil
		}
		spec, err := sbajob.ParseAndValidateJobSpec(data)
		if err != nil {
			state.runnerErr = err
			return nil
		}
		workspace := filepath.Join(state.jobDir, "workspace")
		if err := os.MkdirAll(workspace, 0755); err != nil {
			state.runnerErr = err
			return nil
		}
		state.resultPath = filepath.Join(state.jobDir, "result.json")
		var result *sbajob.Result
		// Use mock LLM so BDD does not require real Ollama (agent mode).
		if state.lastRunCmd != "" {
			argv, _ := json.Marshal([]string{"sh", "-c", state.lastRunCmd})
			mock := &sba.MockLLM{Responses: []string{
				fmt.Sprintf("Action: run_command\nAction Input: {\"argv\": %s}", string(argv)),
				"Final Answer: Done",
			}}
			result = sba.RunAgent(ctx, spec, workspace, &sba.RunAgentOptions{LLM: mock})
		} else if state.lastApplyDiff != "" {
			diffJSON, _ := json.Marshal(state.lastApplyDiff)
			mock := &sba.MockLLM{Responses: []string{
				fmt.Sprintf("Action: apply_unified_diff\nAction Input: {\"diff\": %s}", string(diffJSON)),
				"Final Answer: Done",
			}}
			result = sba.RunAgent(ctx, spec, workspace, &sba.RunAgentOptions{LLM: mock})
		} else {
			mock := &sba.MockLLM{}
			result = sba.RunAgent(ctx, spec, workspace, &sba.RunAgentOptions{LLM: mock})
		}
		state.result = result
		out, _ := json.MarshalIndent(result, "", "  ")
		return os.WriteFile(state.resultPath, out, 0644)
	})
	sc.Step(`^the result status is "([^"]*)"$`, func(ctx context.Context, status string) error {
		if state.result == nil {
			return fmt.Errorf("runner did not produce a result")
		}
		if state.result.Status != status {
			return fmt.Errorf("result status %q, want %q", state.result.Status, status)
		}
		return nil
	})
	sc.Step(`^the result file contains "([^"]*)"$`, func(ctx context.Context, key string) error {
		if state.resultPath == "" {
			return fmt.Errorf("no result file path")
		}
		data, err := os.ReadFile(state.resultPath)
		if err != nil {
			return err
		}
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		if _, ok := m[key]; !ok {
			return fmt.Errorf("result JSON does not contain key %q", key)
		}
		return nil
	})
	sc.Step(`^I have a valid job JSON for stdin with one run_command step "([^"]*)"$`, func(ctx context.Context, command string) error {
		state.lastRunCmd = command
		argv, _ := json.Marshal([]string{"sh", "-c", command})
		state.stdinJobJSON = fmt.Sprintf(`{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[{"type":"run_command","args":{"argv":%s}}]}`, string(argv))
		return nil
	})
	sc.Step(`^I run the SBA runner with stdin and stdout$`, func(ctx context.Context) error {
		spec, err := sbajob.ParseAndValidateJobSpec([]byte(state.stdinJobJSON))
		if err != nil {
			state.runnerErr = err
			return nil
		}
		dir, err := os.MkdirTemp("", "agents_bdd_stdin_")
		if err != nil {
			return err
		}
		state.jobDir = dir
		workspace := filepath.Join(dir, "workspace")
		if err := os.MkdirAll(workspace, 0755); err != nil {
			return err
		}
		argv, _ := json.Marshal([]string{"sh", "-c", state.lastRunCmd})
		mock := &sba.MockLLM{Responses: []string{
			fmt.Sprintf("Action: run_command\nAction Input: {\"argv\": %s}", string(argv)),
			"Final Answer: Done",
		}}
		result := sba.RunAgent(ctx, spec, workspace, &sba.RunAgentOptions{LLM: mock})
		state.result = result
		out, _ := json.MarshalIndent(result, "", "  ")
		state.resultOutput = string(out)
		return nil
	})
	sc.Step(`^the result output contains "([^"]*)"$`, func(ctx context.Context, key string) error {
		if state.resultOutput == "" {
			return fmt.Errorf("no result output")
		}
		if !strings.Contains(state.resultOutput, key) {
			return fmt.Errorf("result output does not contain %q", key)
		}
		return nil
	})
	sc.Step(`^I have a job with max_output_bytes (\d+) and one run_command that outputs (\d+) bytes$`, func(ctx context.Context, maxBytesStr, outBytesStr string) error {
		var maxBytes, outBytes int
		if _, err := fmt.Sscanf(maxBytesStr, "%d", &maxBytes); err != nil {
			return err
		}
		if _, err := fmt.Sscanf(outBytesStr, "%d", &outBytes); err != nil {
			return err
		}
		dir, err := os.MkdirTemp("", "agents_bdd_job_")
		if err != nil {
			return err
		}
		state.jobDir = dir
		// Command that outputs outBytes bytes
		cmd := fmt.Sprintf("printf '%%*s' %d '' | tr ' ' 'x'", outBytes)
		state.lastRunCmd = cmd
		argv, _ := json.Marshal([]string{"sh", "-c", cmd})
		job := fmt.Sprintf(`{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":%d},"steps":[{"type":"run_command","args":{"argv":%s}}]}`,
			maxBytes, string(argv))
		jobPath := filepath.Join(dir, "job.json")
		return os.WriteFile(jobPath, []byte(job), 0644)
	})
	sc.Step(`^the result failure_code is "([^"]*)"$`, func(ctx context.Context, code string) error {
		if state.result == nil {
			return fmt.Errorf("no result")
		}
		if state.result.FailureCode == nil || *state.result.FailureCode != code {
			got := ""
			if state.result.FailureCode != nil {
				got = *state.result.FailureCode
			}
			return fmt.Errorf("failure_code = %q, want %q", got, code)
		}
		return nil
	})
	sc.Step(`^I have a job with one apply_unified_diff step that escapes workspace$`, func(ctx context.Context) error {
		dir, err := os.MkdirTemp("", "agents_bdd_job_")
		if err != nil {
			return err
		}
		state.jobDir = dir
		state.lastApplyDiff = "--- a/../../etc/passwd\n+++ b/../../etc/passwd\n@@ -1 +1 @@\n-x\n+y\n"
		diffJSON, _ := json.Marshal(state.lastApplyDiff)
		job := fmt.Sprintf(`{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[{"type":"apply_unified_diff","args":{"diff":%s}}]}`, string(diffJSON))
		jobPath := filepath.Join(dir, "job.json")
		return os.WriteFile(jobPath, []byte(job), 0644)
	})
	sc.Step(`^the result step error contains "([^"]*)"$`, func(ctx context.Context, sub string) error {
		if state.result == nil {
			return fmt.Errorf("no result")
		}
		for _, s := range state.result.Steps {
			if strings.Contains(s.Error, sub) {
				return nil
			}
		}
		return fmt.Errorf("no step error contained %q", sub)
	})
	sc.Step(`^the result failure message contains "([^"]*)"$`, func(ctx context.Context, sub string) error {
		if state.result == nil || state.result.FailureMessage == nil {
			return fmt.Errorf("no result or failure message")
		}
		if !strings.Contains(*state.result.FailureMessage, sub) {
			return fmt.Errorf("failure_message %q does not contain %q", *state.result.FailureMessage, sub)
		}
		return nil
	})
}

func registerSBALifecycleSteps(sc *godog.ScenarioContext, state *agentsTestState) {
	sc.Step(`^I have a lifecycle callback server$`, func(ctx context.Context) error {
		state.lifecycleMu.Lock()
		state.lifecycleStatuses = nil
		state.lifecycleMu.Unlock()
		state.lifecycleServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				return
			}
			var body struct {
				Status string `json:"status"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Status != "" {
				state.lifecycleMu.Lock()
				state.lifecycleStatuses = append(state.lifecycleStatuses, body.Status)
				state.lifecycleMu.Unlock()
			}
			w.WriteHeader(http.StatusOK)
		}))
		return nil
	})
	sc.Step(`^I run the SBA runner with lifecycle callback$`, func(ctx context.Context) error {
		if state.jobDir == "" || state.lifecycleServer == nil {
			return fmt.Errorf("need job dir and lifecycle server")
		}
		jobPath := filepath.Join(state.jobDir, "job.json")
		data, err := os.ReadFile(jobPath)
		if err != nil {
			state.runnerErr = err
			return nil
		}
		spec, err := sbajob.ParseAndValidateJobSpec(data)
		if err != nil {
			state.runnerErr = err
			return nil
		}
		workspace := filepath.Join(state.jobDir, "workspace")
		if err := os.MkdirAll(workspace, 0755); err != nil {
			state.runnerErr = err
			return nil
		}
		state.resultPath = filepath.Join(state.jobDir, "result.json")
		os.Setenv("SBA_JOB_STATUS_URL", state.lifecycleServer.URL)
		defer os.Unsetenv("SBA_JOB_STATUS_URL")
		os.Setenv("SBA_USE_MOCK_LLM", "1")
		defer os.Unsetenv("SBA_USE_MOCK_LLM")
		lc := sba.NewLifecycleClient()
		lc.NotifyInProgress(ctx)
		var mock *sba.MockLLM
		if state.lastRunCmd != "" {
			argv, _ := json.Marshal([]string{"sh", "-c", state.lastRunCmd})
			mock = &sba.MockLLM{Responses: []string{
				fmt.Sprintf("Action: run_command\nAction Input: {\"argv\": %s}", string(argv)),
				"Final Answer: Done",
			}}
		} else {
			mock = &sba.MockLLM{}
		}
		result := sba.RunAgent(ctx, spec, workspace, &sba.RunAgentOptions{LLM: mock})
		lc.NotifyCompletion(ctx, result)
		state.result = result
		out, _ := json.MarshalIndent(result, "", "  ")
		return os.WriteFile(state.resultPath, out, 0644)
	})
	sc.Step(`^the lifecycle server received "([^"]*)"$`, func(ctx context.Context, status string) error {
		state.lifecycleMu.Lock()
		got := state.lifecycleStatuses
		state.lifecycleMu.Unlock()
		for _, s := range got {
			if s == status {
				return nil
			}
		}
		return fmt.Errorf("lifecycle server did not receive %q (got %v)", status, got)
	})
}

func registerPMASteps(sc *godog.ScenarioContext, state *agentsTestState) {
	sc.Step(`^I have an internal chat completion request with messages only "([^"]*)"$`, func(ctx context.Context, content string) error {
		state.pmaRequestJSON = []byte(fmt.Sprintf(`{"messages":[{"role":"user","content":%q}]}`, content))
		return nil
	})
	sc.Step(`^I have an internal chat completion request with project_id "([^"]*)" and task_id "([^"]*)" and additional_context "([^"]*)"$`,
		func(ctx context.Context, projectID, taskID, additionalContext string) error {
			state.pmaRequestJSON = []byte(fmt.Sprintf(`{"messages":[{"role":"user","content":"hi"}],"project_id":%q,"task_id":%q,"additional_context":%q}`,
				projectID, taskID, additionalContext))
			return nil
		})
	sc.Step(`^I have a mock inference server$`, func(ctx context.Context) error {
		state.pmaOldOllamaURL = os.Getenv("OLLAMA_BASE_URL")
		state.pmaMockInference = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Emit an Ollama-compatible NDJSON stream so callInference (streaming) succeeds.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"test","created_at":"","message":{"role":"assistant","content":"ok"},"done":false}` + "\n"))
			_, _ = w.Write([]byte(`{"model":"test","created_at":"","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}` + "\n"))
		}))
		os.Setenv("OLLAMA_BASE_URL", state.pmaMockInference.URL)
		os.Unsetenv("MCP_GATEWAY_URL")
		os.Unsetenv("PMA_MCP_GATEWAY_URL")
		return nil
	})
	sc.Step(`^I have a mock inference server that captures the prompt$`, func(ctx context.Context) error {
		state.pmaOldOllamaURL = os.Getenv("OLLAMA_BASE_URL")
		state.pmaMockInference = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var body struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				for _, m := range body.Messages {
					if m.Role == "system" {
						state.pmaCapturedPrompt = m.Content
						break
					}
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":{"content":"ok"},"response":"ok"}`))
		}))
		os.Setenv("OLLAMA_BASE_URL", state.pmaMockInference.URL)
		os.Unsetenv("MCP_GATEWAY_URL")
		os.Unsetenv("PMA_MCP_GATEWAY_URL")
		return nil
	})
	sc.Step(`^I send the request to the PMA internal chat completion endpoint$`, func(ctx context.Context) error {
		if len(state.pmaRequestJSON) == 0 {
			return fmt.Errorf("no PMA request body set")
		}
		handler := pma.ChatCompletionHandler("baseline", slog.Default())
		req := httptest.NewRequest(http.MethodPost, "/internal/chat/completion", strings.NewReader(string(state.pmaRequestJSON)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler(rec, req)
		state.pmaResponseStatus = rec.Code
		state.pmaResponseBody = rec.Body.Bytes()
		return nil
	})
	sc.Step(`^the response status is 200$`, func(ctx context.Context) error {
		if state.pmaResponseStatus != 200 {
			return fmt.Errorf("response status %d, want 200", state.pmaResponseStatus)
		}
		return nil
	})
	sc.Step(`^the response content is non-empty$`, func(ctx context.Context) error {
		var out struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(state.pmaResponseBody, &out); err != nil {
			return err
		}
		if out.Content == "" {
			return fmt.Errorf("response content is empty")
		}
		return nil
	})
	sc.Step(`^the captured prompt contains "([^"]*)"$`, func(ctx context.Context, sub string) error {
		if !strings.Contains(state.pmaCapturedPrompt, sub) {
			return fmt.Errorf("captured prompt does not contain %q", sub)
		}
		return nil
	})
	sc.Step(`^"([^"]*)" appears before "([^"]*)" in the captured prompt$`, func(ctx context.Context, before, after string) error {
		i := strings.Index(state.pmaCapturedPrompt, before)
		j := strings.Index(state.pmaCapturedPrompt, after)
		if i < 0 {
			return fmt.Errorf("captured prompt does not contain %q", before)
		}
		if j < 0 {
			return fmt.Errorf("captured prompt does not contain %q", after)
		}
		if i >= j {
			return fmt.Errorf("%q does not appear before %q (indices %d, %d)", before, after, i, j)
		}
		return nil
	})
}
