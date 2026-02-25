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

	"github.com/cypher0n3/cynodeai/agents/internal/sba"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
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
	lastRunCmd    string   // for mock: one run_command step argv
	lastApplyDiff string   // for mock: apply_unified_diff diff body (escaping)
	stdinJobJSON  string   // for stdin mode: raw job JSON
	resultOutput  string   // for stdin mode: result JSON written to stdout
	// Lifecycle (callback server)
	lifecycleServer   *httptest.Server
	lifecycleStatuses []string
	lifecycleMu       sync.Mutex
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
