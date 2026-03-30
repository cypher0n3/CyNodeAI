// Package bdd – orchestrator Godog steps: workflow resume/release, API egress stub, artifacts, MCP tool calls.
package bdd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
)

func registerOrchestratorWorkflowEgressArtifacts(sc *godog.ScenarioContext, state *testState) {

	sc.Step(`^I resume workflow for task$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.taskID == "" {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"task_id": st.taskID})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/resume", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.lastTaskResultBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^workflow resume response status is (\d+)$`, func(ctx context.Context, want int) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.lastStatusCode != want {
			return fmt.Errorf("workflow resume status got %d, want %d", st.lastStatusCode, want)
		}
		return nil
	})
	sc.Step(`^workflow resume response includes last_node_id "([^"]*)"$`, func(ctx context.Context, want string) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return godog.ErrSkip
		}
		var out struct {
			LastNodeID string `json:"last_node_id"`
		}
		if err := json.Unmarshal(st.lastTaskResultBody, &out); err != nil {
			return err
		}
		if out.LastNodeID != want {
			return fmt.Errorf("workflow resume last_node_id got %q, want %q", out.LastNodeID, want)
		}
		return nil
	})
	sc.Step(`^workflow resume state contains substring "([^"]*)"$`, func(ctx context.Context, needle string) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return godog.ErrSkip
		}
		var out struct {
			State *string `json:"state"`
		}
		if err := json.Unmarshal(st.lastTaskResultBody, &out); err != nil {
			return err
		}
		if out.State == nil || !strings.Contains(*out.State, needle) {
			return fmt.Errorf("workflow resume state missing %q in %v", needle, out.State)
		}
		return nil
	})
	sc.Step(`^I store the lease_id from workflow start response$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.workflowStartBody) == 0 {
			return godog.ErrSkip
		}
		var out struct {
			LeaseID string `json:"lease_id"`
		}
		if err := json.Unmarshal(st.workflowStartBody, &out); err != nil {
			return err
		}
		st.storedLeaseID = out.LeaseID
		return nil
	})
	sc.Step(`^I release workflow for task with stored lease_id$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.taskID == "" || st.storedLeaseID == "" {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"task_id": st.taskID, "lease_id": st.storedLeaseID})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/release", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("workflow release returned %d", resp.StatusCode)
		}
		return nil
	})
	// Compound workflow steps (one When per scenario for only-one-when)
	sc.Step(`^I create a task with prompt "([^"]*)" and start workflow for task with holder "([^"]*)"$`, func(ctx context.Context, prompt, holder string) error {
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
		if err := postTaskReadyHTTP(ctx); err != nil {
			return err
		}
		body2, _ := json.Marshal(map[string]string{"task_id": st.taskID, "holder_id": holder})
		req2, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/start", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			return err
		}
		defer resp2.Body.Close()
		st.lastStatusCode = resp2.StatusCode
		st.workflowStartBody, _ = io.ReadAll(resp2.Body)
		return nil
	})
	sc.Step(`^I create a task with prompt "([^"]*)" and start workflow for task with holder "([^"]*)" and start workflow for task with holder "([^"]*)"$`, func(ctx context.Context, prompt, h1, h2 string) error {
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
		if err := postTaskReadyHTTP(ctx); err != nil {
			return err
		}
		body2, _ := json.Marshal(map[string]string{"task_id": st.taskID, "holder_id": h1})
		req2, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/start", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		if resp2, err := http.DefaultClient.Do(req2); err != nil {
			return err
		} else {
			resp2.Body.Close()
		}
		body3, _ := json.Marshal(map[string]string{"task_id": st.taskID, "holder_id": h2})
		req3, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/start", bytes.NewReader(body3))
		req3.Header.Set("Content-Type", "application/json")
		resp3, err := http.DefaultClient.Do(req3)
		if err != nil {
			return err
		}
		defer resp3.Body.Close()
		st.lastStatusCode = resp3.StatusCode
		st.workflowStartBody, _ = io.ReadAll(resp3.Body)
		return nil
	})
	sc.Step(`^I create a task with prompt "([^"]*)" and start workflow for task with holder "([^"]*)" and save checkpoint for task with last_node_id "([^"]*)" and resume workflow for task$`, func(ctx context.Context, prompt, holder, nodeID string) error {
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
		if err := postTaskReadyHTTP(ctx); err != nil {
			return err
		}
		body2, _ := json.Marshal(map[string]string{"task_id": st.taskID, "holder_id": holder})
		req2, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/start", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		if resp2, err := http.DefaultClient.Do(req2); err != nil {
			return err
		} else {
			resp2.Body.Close()
		}
		body3, _ := json.Marshal(map[string]string{"task_id": st.taskID, "last_node_id": nodeID})
		req3, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/checkpoint", bytes.NewReader(body3))
		req3.Header.Set("Content-Type", "application/json")
		if resp3, err := http.DefaultClient.Do(req3); err != nil {
			return err
		} else if resp3.StatusCode != http.StatusNoContent {
			resp3.Body.Close()
			return fmt.Errorf("checkpoint returned %d", resp3.StatusCode)
		} else {
			resp3.Body.Close()
		}
		body4, _ := json.Marshal(map[string]string{"task_id": st.taskID})
		req4, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/resume", bytes.NewReader(body4))
		req4.Header.Set("Content-Type", "application/json")
		resp4, err := http.DefaultClient.Do(req4)
		if err != nil {
			return err
		}
		defer resp4.Body.Close()
		st.lastStatusCode = resp4.StatusCode
		st.lastTaskResultBody, _ = io.ReadAll(resp4.Body)
		return nil
	})
	sc.Step(`^I create a task with prompt "([^"]*)" and start workflow for task with holder "([^"]*)" and save verification review checkpoint and resume workflow for task$`, func(ctx context.Context, prompt, holder string) error {
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
		if err := postTaskReadyHTTP(ctx); err != nil {
			return err
		}
		body2, _ := json.Marshal(map[string]string{"task_id": st.taskID, "holder_id": holder})
		req2, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/start", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		if resp2, err := http.DefaultClient.Do(req2); err != nil {
			return err
		} else {
			resp2.Body.Close()
		}
		verifyState := `{"pma_tasked_paa":true,"paa_outcome":"accepted","findings":"criteria met"}`
		type checkpointBody struct {
			TaskID     string `json:"task_id"`
			LastNodeID string `json:"last_node_id"`
			State      string `json:"state"`
		}
		body3, _ := json.Marshal(checkpointBody{
			TaskID:     st.taskID,
			LastNodeID: "verify_step_result",
			State:      verifyState,
		})
		req3, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/checkpoint", bytes.NewReader(body3))
		req3.Header.Set("Content-Type", "application/json")
		if resp3, err := http.DefaultClient.Do(req3); err != nil {
			return err
		} else if resp3.StatusCode != http.StatusNoContent {
			resp3.Body.Close()
			return fmt.Errorf("checkpoint returned %d", resp3.StatusCode)
		} else {
			resp3.Body.Close()
		}
		body4, _ := json.Marshal(map[string]string{"task_id": st.taskID})
		req4, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/resume", bytes.NewReader(body4))
		req4.Header.Set("Content-Type", "application/json")
		resp4, err := http.DefaultClient.Do(req4)
		if err != nil {
			return err
		}
		defer resp4.Body.Close()
		st.lastStatusCode = resp4.StatusCode
		st.lastTaskResultBody, _ = io.ReadAll(resp4.Body)
		return nil
	})
	sc.Step(`^I create a task with prompt "([^"]*)" and start workflow for task with holder "([^"]*)" and store the lease_id from workflow start response and release workflow for task with stored lease_id and start workflow for task with holder "([^"]*)"$`, func(ctx context.Context, prompt, h1, h2 string) error {
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
		if err := postTaskReadyHTTP(ctx); err != nil {
			return err
		}
		body2, _ := json.Marshal(map[string]string{"task_id": st.taskID, "holder_id": h1})
		req2, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/start", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			return err
		}
		defer resp2.Body.Close()
		st.workflowStartBody, _ = io.ReadAll(resp2.Body)
		var startOut struct {
			LeaseID string `json:"lease_id"`
		}
		_ = json.Unmarshal(st.workflowStartBody, &startOut)
		st.storedLeaseID = startOut.LeaseID
		body3, _ := json.Marshal(map[string]string{"task_id": st.taskID, "lease_id": st.storedLeaseID})
		req3, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/release", bytes.NewReader(body3))
		req3.Header.Set("Content-Type", "application/json")
		if resp3, err := http.DefaultClient.Do(req3); err != nil {
			return err
		} else if resp3.StatusCode != http.StatusNoContent {
			resp3.Body.Close()
			return fmt.Errorf("release returned %d", resp3.StatusCode)
		} else {
			resp3.Body.Close()
		}
		body4, _ := json.Marshal(map[string]string{"task_id": st.taskID, "holder_id": h2})
		req4, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/start", bytes.NewReader(body4))
		req4.Header.Set("Content-Type", "application/json")
		resp4, err := http.DefaultClient.Do(req4)
		if err != nil {
			return err
		}
		defer resp4.Body.Close()
		st.lastStatusCode = resp4.StatusCode
		st.workflowStartBody, _ = io.ReadAll(resp4.Body)
		return nil
	})
	sc.Step(`^I create a task with prompt "([^"]*)" and start workflow for task with holder "([^"]*)" and start workflow for task again with holder "([^"]*)"$`, func(ctx context.Context, prompt, holder, holderAgain string) error {
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
		if err := postTaskReadyHTTP(ctx); err != nil {
			return err
		}
		body2, _ := json.Marshal(map[string]string{"task_id": st.taskID, "holder_id": holder})
		req2, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/start", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			return err
		}
		defer resp2.Body.Close()
		st.lastStatusCode = resp2.StatusCode
		st.workflowStartBody, _ = io.ReadAll(resp2.Body)
		if st.lastStatusCode != http.StatusOK {
			return nil
		}
		var startOut struct {
			LeaseID string `json:"lease_id"`
		}
		if err := json.Unmarshal(st.workflowStartBody, &startOut); err != nil || startOut.LeaseID == "" {
			return fmt.Errorf("first start response missing lease_id (need for idempotency)")
		}
		body3, _ := json.Marshal(map[string]string{
			"task_id":         st.taskID,
			"holder_id":       holderAgain,
			"idempotency_key": startOut.LeaseID,
		})
		req3, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/start", bytes.NewReader(body3))
		req3.Header.Set("Content-Type", "application/json")
		resp3, err := http.DefaultClient.Do(req3)
		if err != nil {
			return err
		}
		defer resp3.Body.Close()
		st.lastStatusCode = resp3.StatusCode
		st.workflowStartBody, _ = io.ReadAll(resp3.Body)
		return nil
	})
	// API egress stub (POST /v1/call)
	sc.Step(`^the API egress stub is configured with bearer token "([^"]*)" and allowlist "([^"]*)"$`, func(ctx context.Context, bearer, allowlist string) error {
		st := getState(ctx)
		if st == nil {
			return nil
		}
		st.egressBearer = bearer
		st.egressAllowlist = allowlist
		return nil
	})
	sc.Step(`^the API egress stub is configured with bearer token "([^"]*)"$`, func(ctx context.Context, bearer string) error {
		st := getState(ctx)
		if st == nil {
			return nil
		}
		st.egressBearer = bearer
		return nil
	})
	sc.Step(`^I call POST "([^"]*)" with bearer "([^"]*)" and body provider "([^"]*)" operation "([^"]*)" task_id "([^"]*)"$`, func(ctx context.Context, path, bearer, provider, operation, taskID string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"provider": provider, "operation": operation, "task_id": taskID})
		req, _ := http.NewRequest("POST", st.server.URL+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+bearer)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.lastResponseBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^I call POST "([^"]*)" without bearer with body provider "([^"]*)" operation "([^"]*)" task_id "([^"]*)"$`, func(ctx context.Context, path, provider, operation, taskID string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"provider": provider, "operation": operation, "task_id": taskID})
		req, _ := http.NewRequest("POST", st.server.URL+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.lastResponseBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^the response status is (\d+)$`, func(ctx context.Context, statusStr string) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		var want int
		if _, err := fmt.Sscanf(statusStr, "%d", &want); err != nil {
			return err
		}
		if st.lastStatusCode != want {
			return fmt.Errorf("expected status %d, got %d", want, st.lastStatusCode)
		}
		return nil
	})
	sc.Step(`^the response JSON has "([^"]*)" equal to "([^"]*)"$`, func(ctx context.Context, key, want string) error {
		st := getState(ctx)
		if st == nil || st.lastResponseBody == nil {
			return godog.ErrSkip
		}
		var m map[string]interface{}
		if err := json.Unmarshal(st.lastResponseBody, &m); err != nil {
			return err
		}
		v, ok := m[key]
		if !ok {
			return fmt.Errorf("response JSON missing key %q", key)
		}
		s, _ := v.(string)
		if s != want {
			return fmt.Errorf("response JSON %q: got %q, want %q", key, s, want)
		}
		return nil
	})
	sc.Step(`^the response JSON has "([^"]*)" containing "([^"]*)"$`, func(ctx context.Context, key, sub string) error {
		st := getState(ctx)
		if st == nil || st.lastResponseBody == nil {
			return godog.ErrSkip
		}
		var m map[string]interface{}
		if err := json.Unmarshal(st.lastResponseBody, &m); err != nil {
			return err
		}
		v, ok := m[key]
		if !ok {
			return fmt.Errorf("response JSON missing key %q", key)
		}
		s, _ := v.(string)
		if !strings.Contains(s, sub) {
			return fmt.Errorf("response JSON %q %q does not contain %q", key, s, sub)
		}
		return nil
	})
	sc.Step(`^a user exists with handle "([^"]*)" and password "([^"]*)"$`, func(ctx context.Context, handle, password string) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		_, err := st.db.GetUserByHandle(ctx, handle)
		if err == nil {
			return nil
		}
		if !errors.Is(err, database.ErrNotFound) {
			return err
		}
		user, err := st.db.CreateUser(ctx, handle, nil)
		if err != nil {
			return err
		}
		hash, err := auth.HashPassword(password, nil)
		if err != nil {
			return err
		}
		_, err = st.db.CreatePasswordCredential(ctx, user.ID, hash, "argon2id")
		return err
	})
	sc.Step(`^I POST /v1/artifacts with query "([^"]*)" and body "([^"]*)"$`, func(ctx context.Context, query, body string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		req, err := http.NewRequest(http.MethodPost, st.server.URL+"/v1/artifacts"+query, strings.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		req.Header.Set("Content-Type", "text/plain")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.lastResponseBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^I store the artifact_id from the last JSON response$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.lastResponseBody == nil {
			return godog.ErrSkip
		}
		var m map[string]interface{}
		if err := json.Unmarshal(st.lastResponseBody, &m); err != nil {
			return err
		}
		v, ok := m["artifact_id"]
		if !ok {
			return fmt.Errorf("artifact_id missing in response")
		}
		s, _ := v.(string)
		if s == "" {
			return fmt.Errorf("empty artifact_id")
		}
		st.artifactID = s
		return nil
	})
	sc.Step(`^I GET the stored artifact blob$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.artifactID == "" {
			return godog.ErrSkip
		}
		req, err := http.NewRequest(http.MethodGet, st.server.URL+"/v1/artifacts/"+st.artifactID, http.NoBody)
		if err != nil {
			return err
		}
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
	sc.Step(`^the last raw response body equals "([^"]*)"$`, func(ctx context.Context, want string) error {
		st := getState(ctx)
		if st == nil || st.lastResponseBody == nil {
			return godog.ErrSkip
		}
		if string(st.lastResponseBody) != want {
			return fmt.Errorf("body %q want %q", string(st.lastResponseBody), want)
		}
		return nil
	})
	sc.Step(`^the BDD group id is "([^"]*)"$`, func(ctx context.Context, id string) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("invalid group id: %w", err)
		}
		st.bddGroupID = id
		return nil
	})
	sc.Step(`^the default project id for handle "([^"]*)" is resolved$`, func(ctx context.Context, handle string) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		u, err := st.db.GetUserByHandle(ctx, handle)
		if err != nil {
			return err
		}
		p, err := st.db.GetOrCreateDefaultProjectForUser(ctx, u.ID)
		if err != nil {
			return err
		}
		st.bddProjectID = p.ID.String()
		return nil
	})
	sc.Step(`^I POST /v1/artifacts with group scope path "([^"]*)" and body "([^"]*)"$`, func(ctx context.Context, path, body string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.bddGroupID == "" {
			return fmt.Errorf("group scope POST requires BDD group id")
		}
		q := fmt.Sprintf("?scope_level=group&group_id=%s&path=%s", st.bddGroupID, url.QueryEscape(path))
		return bddPostArtifact(st, q, body)
	})
	sc.Step(`^I POST /v1/artifacts with project scope path "([^"]*)" and body "([^"]*)"$`, func(ctx context.Context, path, body string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.bddProjectID == "" {
			return fmt.Errorf("project scope POST requires resolved project id")
		}
		q := fmt.Sprintf("?scope_level=project&project_id=%s&path=%s", st.bddProjectID, url.QueryEscape(path))
		return bddPostArtifact(st, q, body)
	})
	sc.Step(`^I POST /v1/artifacts with global scope path "([^"]*)" and body "([^"]*)"$`, func(ctx context.Context, path, body string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		q := "?scope_level=global&path=" + url.QueryEscape(path)
		return bddPostArtifact(st, q, body)
	})
	sc.Step(`^user "([^"]*)" has a read grant for the stored artifact$`, func(ctx context.Context, handle string) error {
		st := getState(ctx)
		if st == nil || st.db == nil || st.artifactID == "" {
			return godog.ErrSkip
		}
		u, err := st.db.GetUserByHandle(ctx, handle)
		if err != nil {
			return err
		}
		aid, err := uuid.Parse(st.artifactID)
		if err != nil {
			return err
		}
		return st.db.GrantArtifactRead(ctx, aid, u.ID)
	})
	sc.Step(`^the MCP agent "([^"]*)" calls artifact.put for user "([^"]*)" path "([^"]*)" with text "([^"]*)"$`, func(ctx context.Context, agentRole, userHandle, artifactPath, text string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.db == nil {
			return godog.ErrSkip
		}
		u, err := st.db.GetUserByHandle(ctx, userHandle)
		if err != nil {
			return err
		}
		b64 := base64.StdEncoding.EncodeToString([]byte(text))
		payload := map[string]interface{}{
			"tool_name": "artifact.put",
			"arguments": map[string]interface{}{
				"user_id":              u.ID.String(),
				"scope":                "user",
				"path":                 artifactPath,
				"content_bytes_base64": b64,
			},
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, st.server.URL+"/v1/mcp/tools/call", bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		switch strings.ToLower(strings.TrimSpace(agentRole)) {
		case "pm":
			req.Header.Set("Authorization", "Bearer "+st.pmMCPAgentToken)
		case "sandbox":
			req.Header.Set("Authorization", "Bearer "+st.sandboxMCPAgentToken)
		default:
			return fmt.Errorf("unknown MCP agent role %q", agentRole)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.lastResponseBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^the MCP agent "([^"]*)" calls skills.create for user "([^"]*)" with content "([^"]*)" including extraneous task_id$`, func(ctx context.Context, agentRole, userHandle, content string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.db == nil {
			return godog.ErrSkip
		}
		u, err := st.db.GetUserByHandle(ctx, userHandle)
		if err != nil {
			return err
		}
		payload := map[string]interface{}{
			"tool_name": "skills.create",
			"arguments": map[string]interface{}{
				"user_id": u.ID.String(),
				"name":    "bdd-mcp-skill",
				"content": content,
				"scope":   "user",
				"task_id": uuid.New().String(),
			},
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, st.server.URL+"/v1/mcp/tools/call", bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		switch strings.ToLower(strings.TrimSpace(agentRole)) {
		case "pm":
			req.Header.Set("Authorization", "Bearer "+st.pmMCPAgentToken)
		case "sandbox":
			req.Header.Set("Authorization", "Bearer "+st.sandboxMCPAgentToken)
		default:
			return fmt.Errorf("unknown MCP agent role %q", agentRole)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.lastResponseBody, _ = io.ReadAll(resp.Body)
		return nil
	})
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
