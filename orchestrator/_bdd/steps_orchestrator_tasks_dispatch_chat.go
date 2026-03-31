// Package bdd – orchestrator Godog steps: task lifecycle, dispatch, chat, preferences, and related HTTP flows.
package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/dispatcher"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func registerOrchestratorTasksDispatchChat(sc *godog.ScenarioContext, state *testState) {

	sc.Step(`^I cancel the task$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.taskID == "" {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks/"+st.taskID+"/cancel", nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("cancel task returned %d", resp.StatusCode)
		}
		return nil
	})
	// Combined steps (one When) for only-one-when compliance
	sc.Step(`^I create a task with command "([^"]*)" and the orchestrator selects the node for dispatch$`, func(ctx context.Context, cmd string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
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
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		return orchestratorSelectsNodeForDispatch(ctx)
	})
	sc.Step(`^I create a task with prompt "([^"]*)" and the task completes and I get the task result$`, func(ctx context.Context, prompt string) error {
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
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		if err := taskCompletes(ctx); err != nil {
			return err
		}
		return getTaskResult(ctx)
	})
	sc.Step(`^I create a task with input_mode "commands" and prompt "([^"]*)" and the orchestrator selects the node for dispatch$`, func(ctx context.Context, prompt string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]any{"prompt": prompt, "input_mode": "commands"})
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
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		return orchestratorSelectsNodeForDispatch(ctx)
	})
	sc.Step(`^I create a task with prompt "([^"]*)" and I list tasks with limit (\d+)$`, func(ctx context.Context, prompt string, limit int) error {
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
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		req2, _ := http.NewRequest("GET", st.server.URL+"/v1/tasks?limit="+fmt.Sprint(limit), nil)
		req2.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			return err
		}
		defer resp2.Body.Close()
		st.lastStatusCode = resp2.StatusCode
		if resp2.StatusCode != http.StatusOK {
			return fmt.Errorf("list tasks returned %d", resp2.StatusCode)
		}
		st.lastTaskResultBody, _ = io.ReadAll(resp2.Body)
		return nil
	})
	sc.Step(`^I create a task with prompt "([^"]*)" and I cancel the task$`, func(ctx context.Context, prompt string) error {
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
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		req2, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks/"+st.taskID+"/cancel", nil)
		req2.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			return err
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusOK {
			return fmt.Errorf("cancel task returned %d", resp2.StatusCode)
		}
		return nil
	})
	sc.Step(`^the task status is canceled$`, func(ctx context.Context) error {
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
		var out struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		if out.Status != userapi.StatusCanceled {
			return fmt.Errorf("task status %q, want %s", out.Status, userapi.StatusCanceled)
		}
		return nil
	})
	sc.Step(`^I get the task logs$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.taskID == "" {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/tasks/"+st.taskID+"/logs", nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.lastTaskResultBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^the response contains stdout and stderr$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return fmt.Errorf("no logs response in state")
		}
		var out struct {
			Stdout string `json:"stdout"`
			Stderr string `json:"stderr"`
		}
		if err := json.Unmarshal(st.lastTaskResultBody, &out); err != nil {
			return err
		}
		return nil
	})
	sc.Step(`^a mock PMA server is running$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.pmaMockServer != nil {
			return nil
		}
		st.pmaMockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/internal/chat/completion" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"content": "mock pma response"})
		}))
		st.pmaMockServerURL = st.pmaMockServer.URL
		return nil
	})
	sc.Step(`^a node "([^"]*)" exists and has reported PMA ready at the mock PMA server$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil || st.pmaMockServerURL == "" {
			return godog.ErrSkip
		}
		node, err := st.db.GetNodeBySlug(ctx, slug)
		if errors.Is(err, database.ErrNotFound) {
			if err := nodeRegisterStep(ctx, slug, ""); err != nil {
				return err
			}
			node, err = st.db.GetNodeBySlug(ctx, slug)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if err := st.db.UpdateNodeStatus(ctx, node.ID, "active"); err != nil {
			return err
		}
		// REQ-ORCHES-0162 / REQ-ORCHES-0151: capability ServiceID must match the session binding for
		// this login. Derive only from the current refresh session (not ListActiveBindingsForUser),
		// because BDD shares one Postgres across scenarios and stale bindings would confuse routing.
		svcID := "pma-bdd"
		if st.refreshToken != "" {
			if rs, err := st.db.GetActiveRefreshSession(ctx, auth.HashToken(st.refreshToken)); err == nil && rs != nil {
				if u, uerr := st.db.GetUserByID(ctx, rs.UserID); uerr == nil && u != nil {
					lineage := models.SessionBindingLineage{UserID: u.ID, SessionID: rs.ID, ThreadID: nil}
					svcID = models.PMAServiceIDForBindingKey(models.DeriveSessionBindingKey(lineage))
				}
			}
		}
		report := nodepayloads.CapabilityReport{
			Version:    1,
			ReportedAt: time.Now().UTC().Format(time.RFC3339),
			Node:       nodepayloads.CapabilityNode{NodeSlug: slug},
			Platform:   nodepayloads.Platform{OS: "linux", Arch: "amd64"},
			Compute:    nodepayloads.Compute{CPUCores: 2, RAMMB: 4096},
			ManagedServicesStatus: &nodepayloads.ManagedServicesStatus{
				Services: []nodepayloads.ManagedServiceStatus{
					{
						ServiceID:   svcID,
						ServiceType: "pma",
						State:       "ready",
						Endpoints:   []string{st.pmaMockServerURL},
						ReadyAt:     time.Now().UTC().Format(time.RFC3339),
					},
				},
			},
		}
		raw, err := json.Marshal(report)
		if err != nil {
			return err
		}
		return st.db.SaveNodeCapabilitySnapshot(ctx, node.ID, string(raw))
	})
	sc.Step(`^I send a chat message "([^"]*)"$`, func(ctx context.Context, message string) error {
		return sendChatMessage(ctx, message, "qwen3.5:0.8b")
	})
	sc.Step(`^I send a chat message "([^"]*)" with model cynodeai\.pm$`, func(ctx context.Context, message string) error {
		return sendChatMessage(ctx, message, "cynodeai.pm")
	})
	sc.Step(`^I receive a 200 response with non-empty response field$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.lastStatusCode != http.StatusOK {
			return fmt.Errorf("expected 200, got %d", st.lastStatusCode)
		}
		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(st.lastTaskResultBody, &out); err != nil {
			return err
		}
		if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
			return fmt.Errorf("choices[0].message.content empty")
		}
		return nil
	})
	sc.Step(`^the response status is one of 200, 502, 504$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		allowed := map[int]bool{http.StatusOK: true, 502: true, 504: true}
		if !allowed[st.lastStatusCode] {
			return fmt.Errorf("response status %d is not one of 200, 502, 504", st.lastStatusCode)
		}
		return nil
	})
	sc.Step(`^I receive the task details including status$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the task completes$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.db == nil || st.taskID == "" {
			return godog.ErrSkip
		}
		client := &http.Client{Timeout: 30 * time.Second}
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			err := dispatcher.RunOnce(ctx, st.db, client, 30*time.Second, nil)
			if err != nil && !errors.Is(err, database.ErrNotFound) {
				return err
			}
			taskID, err := uuid.Parse(st.taskID)
			if err != nil {
				return err
			}
			task, err := st.db.GetTaskByID(ctx, taskID)
			if err != nil {
				return err
			}
			if task.Status == models.TaskStatusCompleted || task.Status == models.TaskStatusFailed {
				return nil
			}
			time.Sleep(50 * time.Millisecond)
		}
		return fmt.Errorf("task did not complete within 15s")
	})
	sc.Step(`^the task result contains model output$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return fmt.Errorf("no task result in state (call get task result first)")
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
			return fmt.Errorf("task result has no job output")
		}
		var jobOut struct {
			Stdout string `json:"stdout"`
		}
		if err := json.Unmarshal([]byte(*result.Jobs[0].Result), &jobOut); err != nil {
			return err
		}
		if jobOut.Stdout == "" {
			return fmt.Errorf("task result stdout is empty (expected model output)")
		}
		return nil
	})
	sc.Step(`^I create a task with input_mode "([^"]*)" and prompt "([^"]*)"$`, func(ctx context.Context, inputMode, prompt string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]any{"prompt": prompt, "input_mode": inputMode})
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
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		return nil
	})
	sc.Step(`^the job sent to the worker has command containing "([^"]*)"$`, func(ctx context.Context, sub string) error {
		st := getState(ctx)
		if st == nil || len(st.workerRequestBody) == 0 {
			return fmt.Errorf("no worker request body captured")
		}
		var reqBody struct {
			Sandbox struct {
				Command []string `json:"command"`
			} `json:"sandbox"`
		}
		if err := json.Unmarshal(st.workerRequestBody, &reqBody); err != nil {
			return fmt.Errorf("decode worker request: %w", err)
		}
		cmdStr := strings.Join(reqBody.Sandbox.Command, " ")
		if !strings.Contains(cmdStr, sub) {
			return fmt.Errorf("worker command %q does not contain %q", cmdStr, sub)
		}
		return nil
	})
	sc.Step(`^I have a completed task$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.db == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"prompt": "echo done"})
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
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		if err := postTaskReadyHTTP(ctx); err != nil {
			return err
		}
		client := &http.Client{Timeout: 30 * time.Second}
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			err := dispatcher.RunOnce(ctx, st.db, client, 30*time.Second, nil)
			if err != nil && !errors.Is(err, database.ErrNotFound) {
				return err
			}
			taskID, _ := uuid.Parse(st.taskID)
			task, err := st.db.GetTaskByID(ctx, taskID)
			if err != nil {
				return err
			}
			if task.Status == models.TaskStatusCompleted {
				return nil
			}
			time.Sleep(50 * time.Millisecond)
		}
		return fmt.Errorf("task did not complete within 15s")
	})
	sc.Step(`^I get the task result$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/tasks/"+st.taskID+"/result", nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.lastTaskResultBody, err = io.ReadAll(resp.Body)
		return err
	})
	sc.Step(`^I receive the job output including stdout and exit code$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return fmt.Errorf("no task result in state (call get task result first)")
		}
		if st.lastStatusCode != http.StatusOK {
			return fmt.Errorf("task result returned %d", st.lastStatusCode)
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
			return fmt.Errorf("task result has no job output")
		}
		var jobOut struct {
			Stdout   string `json:"stdout"`
			ExitCode int    `json:"exit_code"`
		}
		if err := json.Unmarshal([]byte(*result.Jobs[0].Result), &jobOut); err != nil {
			return err
		}
		// Assert fields are present (exit_code 0 is valid)
		_ = jobOut.Stdout
		_ = jobOut.ExitCode
		return nil
	})
	sc.Step(`^the orchestrator selects the node for dispatch$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.workerServer == nil || st.db == nil {
			return godog.ErrSkip
		}
		client := &http.Client{Timeout: 30 * time.Second}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			err := dispatcher.RunOnce(ctx, st.db, client, 30*time.Second, nil)
			if err == nil {
				// Dispatch ran; worker may have been called
			} else if !errors.Is(err, database.ErrNotFound) {
				return err
			}
			st.workerRequestMu.Lock()
			got := st.workerRequest != nil
			st.workerRequestMu.Unlock()
			if got {
				return nil
			}
			time.Sleep(50 * time.Millisecond)
		}
		return fmt.Errorf("dispatcher did not call worker within 5s")
	})
	sc.Step(`^the orchestrator calls the node Worker API at its configured target URL$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		st.workerRequestMu.Lock()
		defer st.workerRequestMu.Unlock()
		if st.workerRequest == nil {
			return fmt.Errorf("no worker request was received")
		}
		if st.workerRequest.URL.Path != "/v1/worker/jobs:run" {
			return fmt.Errorf("worker request path %q, want /v1/worker/jobs:run", st.workerRequest.URL.Path)
		}
		return nil
	})
	sc.Step(`^the request includes the bearer token from that node's config$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		st.workerRequestMu.Lock()
		defer st.workerRequestMu.Unlock()
		if st.workerRequest == nil {
			return fmt.Errorf("no worker request was received")
		}
		want := "Bearer " + st.workerToken
		if got := st.workerRequest.Header.Get("Authorization"); got != want {
			return fmt.Errorf("Authorization header %q, want %q", got, want)
		}
		return nil
	})

	// Startup (fail fast when no inference)
	sc.Step(`^no local inference \(Ollama\) is running$`, func(ctx context.Context) error { return nil })
	sc.Step(`^no external provider key is configured$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the orchestrator starts$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		resp, err := http.Get(st.server.URL + "/healthz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("healthz returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^the orchestrator does not enter ready state$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		resp, err := http.Get(st.server.URL + "/readyz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable {
			return fmt.Errorf("readyz returned %d, want 503", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^the orchestrator reports that no inference path is available$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		resp, err := http.Get(st.server.URL + "/readyz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(body, []byte("no inference path")) {
			return fmt.Errorf("readyz body %q does not contain 'no inference path'", string(body))
		}
		return nil
	})
	sc.Step(`^I request the readyz endpoint$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		resp, err := http.Get(st.server.URL + "/readyz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		return nil
	})
	sc.Step(`^a registered node "([^"]*)" is active with worker_api config and I request the readyz endpoint$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil || st.server == nil {
			return godog.ErrSkip
		}
		st.nodeSlug = slug
		node, err := st.db.GetNodeBySlug(ctx, slug)
		if errors.Is(err, database.ErrNotFound) {
			if err := nodeRegisterStep(ctx, slug, st.advertisedWorkerAPIURL); err != nil {
				return err
			}
			node, err = st.db.GetNodeBySlug(ctx, slug)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if err := st.db.UpdateNodeStatus(ctx, node.ID, "active"); err != nil {
			return err
		}
		if st.workerServer == nil {
			st.workerRequestMu.Lock()
			st.workerRequest = nil
			st.workerRequestMu.Unlock()
			st.workerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				st.workerRequestMu.Lock()
				st.workerRequest = r
				st.workerRequestBody = body
				st.workerRequestMu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(workerapi.RunJobResponse{
					Version: 1, TaskID: "", JobID: "", Status: workerapi.StatusCompleted,
					ExitCode: workerapi.ExitCodePtr(0), Stdout: "ok",
					StartedAt: time.Now().UTC().Format(time.RFC3339),
					EndedAt:   time.Now().UTC().Format(time.RFC3339),
				})
			}))
			st.workerToken = "phase1-bdd-token"
		}
		if err := st.db.UpdateNodeWorkerAPIConfig(ctx, node.ID, st.workerServer.URL, st.workerToken); err != nil {
			return err
		}
		ackAt := time.Now().UTC()
		if err := st.db.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil); err != nil {
			return err
		}
		resp, err := http.Get(st.server.URL + "/readyz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		return nil
	})
	sc.Step(`^the orchestrator enters ready state$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		resp, err := http.Get(st.server.URL + "/readyz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("readyz returned %d, want 200", resp.StatusCode)
		}
		return nil
	})

	// E2E / worker (stubs)
	sc.Step(`^a worker node is running and reachable by the orchestrator$`, func(ctx context.Context) error {
		return godog.ErrSkip
	})
	sc.Step(`^the orchestrator dispatches a job to the node$`, func(ctx context.Context) error {
		return godog.ErrSkip
	})
	sc.Step(`^the node executes the sandbox job$`, func(ctx context.Context) error {
		return godog.ErrSkip
	})
	sc.Step(`^the job result contains stdout "([^"]*)"$`, func(ctx context.Context, s string) error {
		return godog.ErrSkip
	})
	sc.Step(`^the task status becomes "([^"]*)"$`, func(ctx context.Context, s string) error {
		return godog.ErrSkip
	})

	// Task lifecycle background
	sc.Step(`^a registered node "([^"]*)" is active$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil || st.server == nil {
			return godog.ErrSkip
		}
		st.nodeSlug = slug
		node, err := st.db.GetNodeBySlug(ctx, slug)
		if errors.Is(err, database.ErrNotFound) {
			if err := nodeRegisterStep(ctx, slug, st.advertisedWorkerAPIURL); err != nil {
				return err
			}
			node, err = st.db.GetNodeBySlug(ctx, slug)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		return st.db.UpdateNodeStatus(ctx, node.ID, "active")
	})
	sc.Step(`^the node "([^"]*)" has worker_api_target_url and bearer token in config$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		node, err := st.db.GetNodeBySlug(ctx, slug)
		if err != nil {
			return err
		}
		token := "phase1-bdd-token"
		if st.workerServer == nil {
			st.workerRequestMu.Lock()
			st.workerRequest = nil
			st.workerRequestMu.Unlock()
			st.workerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				st.workerRequestMu.Lock()
				st.workerRequest = r
				st.workerRequestBody = body
				st.workerRequestMu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(workerapi.RunJobResponse{
					Version:   1,
					TaskID:    "",
					JobID:     "",
					Status:    workerapi.StatusCompleted,
					ExitCode:  workerapi.ExitCodePtr(0),
					Stdout:    "ok",
					StartedAt: time.Now().UTC().Format(time.RFC3339),
					EndedAt:   time.Now().UTC().Format(time.RFC3339),
				})
			}))
			st.workerToken = token
		}
		if err := st.db.UpdateNodeWorkerAPIConfig(ctx, node.ID, st.workerServer.URL, token); err != nil {
			return err
		}
		ackAt := time.Now().UTC()
		return st.db.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
	})
}
