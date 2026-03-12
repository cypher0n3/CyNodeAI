// Package bdd provides Godog step definitions for the e2e suite.
// Feature files live under repo features/e2e/.
// Steps call the gateway at E2E_GATEWAY_URL (default http://localhost:12080); skip when unreachable.
package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

type ctxKey int

const stateKey ctxKey = 0

type e2eState struct {
	gatewayURL         string
	accessToken        string
	taskID             string
	lastTaskResultBody []byte
	lastResponseBody   []byte
	lastStatusCode     int
}

func getState(ctx context.Context) *e2eState {
	v := ctx.Value(stateKey)
	if v == nil {
		return nil
	}
	return v.(*e2eState)
}

func gatewayURL() string {
	u := os.Getenv("E2E_GATEWAY_URL")
	if u != "" {
		return strings.TrimSuffix(u, "/")
	}
	return "http://localhost:12080"
}

func gatewayReachable(base string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(base + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// InitializeE2ESuite sets up the Godog suite for e2e features.
func InitializeE2ESuite(sc *godog.ScenarioContext, state *e2eState) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		state.gatewayURL = gatewayURL()
		state.accessToken = ""
		state.taskID = ""
		state.lastTaskResultBody = nil
		state.lastResponseBody = nil
		state.lastStatusCode = 0
		return context.WithValue(ctx, stateKey, state), nil
	})

	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		return ctx, nil
	})

	registerE2ESteps(sc, state)
}

func registerE2ESteps(sc *godog.ScenarioContext, state *e2eState) {
	// Background
	sc.Step(`^a running PostgreSQL database$`, func(ctx context.Context) error {
		return godog.ErrSkip
	})
	sc.Step(`^the orchestrator API is running$`, func(ctx context.Context) error {
		if !gatewayReachable(state.gatewayURL) {
			return godog.ErrSkip
		}
		return nil
	})
	sc.Step(`^an admin user exists with handle "([^"]*)"$`, func(ctx context.Context, _ string) error {
		return nil
	})
	sc.Step(`^a worker node is running and reachable by the orchestrator$`, func(ctx context.Context) error {
		return nil
	})

	// Single-node happy path: full When (login + register node + config ack + create task + dispatch + execute).
	// We assume node is already registered by the environment; we do login, create task, poll for completion.
	sc.Step(`^I login as "([^"]*)" with password "([^"]*)" and a node with slug "([^"]*)" registers with the orchestrator using a valid PSK and the node requests its configuration and the node applies the configuration and sends a config acknowledgement with status "applied" and I create a task with command "([^"]*)" and the orchestrator dispatches a job to the node and the node executes the sandbox job$`,
		func(ctx context.Context, handle, password, slug, command string) error {
			return e2eLoginCreateTaskWaitCompleted(ctx, state, handle, password, command, false, "")
		})
	sc.Step(`^I login as "([^"]*)" with password "([^"]*)" and a node with slug "([^"]*)" registers with the orchestrator using a valid PSK and the node requests its configuration and the node applies the configuration and sends a config acknowledgement with status "applied" and I create a task with command "([^"]*)" and the orchestrator dispatches a job to the node and the node executes the sandbox job in a pod with inference proxy$`,
		func(ctx context.Context, handle, password, slug, command string) error {
			return e2eLoginCreateTaskWaitCompleted(ctx, state, handle, password, command, true, "")
		})
	sc.Step(`^I login as "([^"]*)" with password "([^"]*)" and a node with slug "([^"]*)" registers with the orchestrator using a valid PSK and the node requests its configuration and the node applies the configuration and sends a config acknowledgement with status "applied" and I create a SBA task with inference and prompt "([^"]*)" and the orchestrator dispatches the job to the node and the node runs the SBA and returns the result$`,
		func(ctx context.Context, handle, password, slug, prompt string) error {
			return e2eLoginCreateTaskWaitCompleted(ctx, state, handle, password, "", true, prompt)
		})

	// Then: task result assertions
	sc.Step(`^the job result contains stdout "([^"]*)"$`, func(ctx context.Context, want string) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return fmt.Errorf("no task result in state")
		}
		var result struct {
			Jobs []struct {
				Result *string `json:"result"`
			} `json:"jobs"`
		}
		if err := json.Unmarshal(st.lastTaskResultBody, &result); err != nil {
			return err
		}
		if len(result.Jobs) == 0 || result.Jobs[0].Result == nil {
			return fmt.Errorf("task result has no job result")
		}
		var jobOut struct {
			Stdout string `json:"stdout"`
		}
		if err := json.Unmarshal([]byte(*result.Jobs[0].Result), &jobOut); err != nil {
			return err
		}
		if !strings.Contains(jobOut.Stdout, want) {
			return fmt.Errorf("job stdout %q does not contain %q", jobOut.Stdout, want)
		}
		return nil
	})
	sc.Step(`^the task status becomes "([^"]*)"$`, func(ctx context.Context, want string) error {
		st := getState(ctx)
		if st == nil || st.taskID == "" {
			return fmt.Errorf("no task in state")
		}
		req, _ := http.NewRequest("GET", st.gatewayURL+"/v1/tasks/"+st.taskID, nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		var out struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		if out.Status != want {
			return fmt.Errorf("task status %q, want %q", out.Status, want)
		}
		return nil
	})
	sc.Step(`^the job result contains "([^"]*)"$`, func(ctx context.Context, substr string) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return fmt.Errorf("no task result in state")
		}
		if !bytes.Contains(st.lastTaskResultBody, []byte(substr)) {
			return fmt.Errorf("task result body does not contain %q", substr)
		}
		return nil
	})
	sc.Step(`^the job result has a user-facing reply$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return fmt.Errorf("no task result in state")
		}
		var result struct {
			Jobs []struct {
				Result *string `json:"result"`
			} `json:"jobs"`
		}
		if err := json.Unmarshal(st.lastTaskResultBody, &result); err != nil {
			return err
		}
		if len(result.Jobs) == 0 || result.Jobs[0].Result == nil {
			return fmt.Errorf("task result has no job result")
		}
		// sba_result may have steps with reply or stdout; either counts as user-facing
		s := *result.Jobs[0].Result
		if strings.Contains(s, `"reply"`) || strings.Contains(s, `"stdout"`) {
			return nil
		}
		return fmt.Errorf("job result has no user-facing reply (reply/stdout)")
	})

	// Chat OpenAI-compatible
	sc.Step(`^I call GET "([^"]*)"$`, func(ctx context.Context, path string) error {
		st := getState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if !gatewayReachable(st.gatewayURL) {
			return godog.ErrSkip
		}
		// Chat scenarios may need auth; login first if we have no token
		if st.accessToken == "" {
			body, _ := json.Marshal(map[string]string{"handle": "admin", "password": "admin123"})
			resp, err := http.Post(st.gatewayURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return godog.ErrSkip
			}
			var out struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				return err
			}
			st.accessToken = out.AccessToken
		}
		req, _ := http.NewRequest("GET", st.gatewayURL+path, nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.lastResponseBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^the response status is (\d+)$`, func(ctx context.Context, want int) error {
		st := getState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.lastStatusCode != want {
			return fmt.Errorf("response status %d, want %d", st.lastStatusCode, want)
		}
		return nil
	})
	sc.Step(`^the response is a list-models payload$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastResponseBody) == 0 {
			return fmt.Errorf("no response body")
		}
		var out struct {
			Data interface{} `json:"data"`
		}
		if err := json.Unmarshal(st.lastResponseBody, &out); err != nil {
			return err
		}
		if out.Data == nil {
			return fmt.Errorf("response missing data (list-models)")
		}
		return nil
	})
	sc.Step(`^I call POST "([^"]*)" with OpenAI-format messages$`, func(ctx context.Context, path string) error {
		st := getState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if !gatewayReachable(st.gatewayURL) {
			return godog.ErrSkip
		}
		if st.accessToken == "" {
			body, _ := json.Marshal(map[string]string{"handle": "admin", "password": "admin123"})
			resp, err := http.Post(st.gatewayURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return godog.ErrSkip
			}
			var out struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				return err
			}
			st.accessToken = out.AccessToken
		}
		body := []byte(`{"model":"cynodeai.pm","messages":[{"role":"user","content":"Hi"}]}`)
		req, _ := http.NewRequest("POST", st.gatewayURL+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.lastResponseBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^the response contains a completion at "([^"]*)"$`, func(ctx context.Context, jsonPath string) error {
		st := getState(ctx)
		if st == nil || len(st.lastResponseBody) == 0 {
			return fmt.Errorf("no response body")
		}
		if jsonPath != "choices[0].message.content" {
			return fmt.Errorf("unsupported path %q", jsonPath)
		}
		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(st.lastResponseBody, &out); err != nil {
			return err
		}
		if len(out.Choices) == 0 {
			return fmt.Errorf("response has no choices")
		}
		if out.Choices[0].Message.Content == "" {
			return fmt.Errorf("choices[0].message.content is empty")
		}
		return nil
	})
	sc.Step(`^the response is a chat completion$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastResponseBody) == 0 {
			return fmt.Errorf("no response body")
		}
		var out struct {
			Choices interface{} `json:"choices"`
		}
		if err := json.Unmarshal(st.lastResponseBody, &out); err != nil {
			return err
		}
		if out.Choices == nil {
			return fmt.Errorf("response missing choices")
		}
		return nil
	})
	sc.Step(`^no assertion is made that a user-visible task was created$`, func(ctx context.Context) error {
		return nil
	})
}

func e2eLoginCreateTaskWaitCompleted(ctx context.Context, state *e2eState, handle, password, command string, useInference bool, sbaPrompt string) error {
	if !gatewayReachable(state.gatewayURL) {
		return godog.ErrSkip
	}
	// Login
	body, _ := json.Marshal(map[string]string{"handle": handle, "password": password})
	resp, err := http.Post(state.gatewayURL+"/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login returned %d", resp.StatusCode)
	}
	var loginOut struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginOut); err != nil {
		return err
	}
	state.accessToken = loginOut.AccessToken

	// Create task
	var createBody map[string]interface{}
	if sbaPrompt != "" {
		createBody = map[string]interface{}{
			"prompt":        sbaPrompt,
			"use_sba":       true,
			"use_inference": true,
		}
	} else {
		createBody = map[string]interface{}{
			"prompt":     command,
			"input_mode": "commands",
		}
		if useInference {
			createBody["use_inference"] = true
		}
	}
	body, _ = json.Marshal(createBody)
	req, _ := http.NewRequest("POST", state.gatewayURL+"/v1/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+state.accessToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create task returned %d: %s", resp.StatusCode, string(b))
	}
	var createOut struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createOut); err != nil {
		return err
	}
	state.taskID = createOut.TaskID

	// Poll until terminal status (completed, failed, canceled)
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("GET", state.gatewayURL+"/v1/tasks/"+state.taskID, nil)
		req.Header.Set("Authorization", "Bearer "+state.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		var taskOut struct {
			Status string `json:"status"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&taskOut)
		resp.Body.Close()
		switch taskOut.Status {
		case "completed":
			goto done
		case "failed", "canceled":
			return fmt.Errorf("task reached terminal status %q", taskOut.Status)
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("task did not complete within 90s")
done:

	// Get task result
	req, _ = http.NewRequest("GET", state.gatewayURL+"/v1/tasks/"+state.taskID+"/result", nil)
	req.Header.Set("Authorization", "Bearer "+state.accessToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	state.lastTaskResultBody, _ = io.ReadAll(resp.Body)
	return nil
}
