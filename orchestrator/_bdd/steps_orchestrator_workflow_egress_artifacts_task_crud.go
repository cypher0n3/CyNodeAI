// Package bdd – Godog steps split from steps_orchestrator_workflow_egress_artifacts.go (line-count guard).
package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

func registerOrchestratorWorkflowEgressArtifactsTaskCRUD(sc *godog.ScenarioContext, _ *testState) {
	sc.Step(`^I create a task with prompt "([^"]*)"$`, func(ctx context.Context, prompt string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"prompt": prompt})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create task returned %d", resp.StatusCode)
		}
		var out struct {
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		return nil
	})
	sc.Step(`^I create a task with use_inference and command "([^"]*)"$`, func(ctx context.Context, cmd string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]any{"prompt": cmd, "use_inference": true})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create task returned %d", resp.StatusCode)
		}
		var out struct {
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		return nil
	})
	sc.Step(`^I create a task with command "([^"]*)"$`, func(ctx context.Context, cmd string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		// input_mode commands so task is queued for dispatcher (no orchestrator inference).
		body, _ := json.Marshal(map[string]string{"prompt": cmd, "input_mode": "commands"})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create task returned %d", resp.StatusCode)
		}
		var out struct {
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		return nil
	})
	sc.Step(`^I receive a task ID$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.taskID == "" {
			return fmt.Errorf("no task ID")
		}
		return nil
	})
	sc.Step(`^the task status is "([^"]*)"$`, func(ctx context.Context, status string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/tasks/"+st.taskID, nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("get task returned %d", resp.StatusCode)
		}
		var out struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		if out.Status != status {
			return fmt.Errorf("task status %q, want %q", out.Status, status)
		}
		return nil
	})
	sc.Step(`^I have created a task$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I get the task status$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.taskID == "" {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/tasks/"+st.taskID, nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("get task returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^I list tasks with limit (\d+)$`, func(ctx context.Context, limit int) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/tasks?limit="+fmt.Sprint(limit), nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("list tasks returned %d", resp.StatusCode)
		}
		st.lastTaskResultBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^I receive a list of tasks$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return fmt.Errorf("no list response in state")
		}
		var out struct {
			Tasks []struct {
				TaskID string `json:"task_id"`
				Status string `json:"status"`
			} `json:"tasks"`
		}
		if err := json.Unmarshal(st.lastTaskResultBody, &out); err != nil {
			return err
		}
		if len(out.Tasks) == 0 {
			return fmt.Errorf("list of tasks is empty")
		}
		return nil
	})
}
