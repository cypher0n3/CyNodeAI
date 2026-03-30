// Package bdd – worker_node Godog steps: telemetry fixtures, SBA job execution, and node-manager config.
package bdd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/cypher0n3/cynodeai/worker_node/internal/nodeagent"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
)

func RegisterTelemetryDataSteps(sc *godog.ScenarioContext, state *workerTestState) {
	sc.Step(`^a sandbox container is recorded for task "([^"]*)" job "([^"]*)"$`, func(ctx context.Context, taskID, jobID string) error {
		if state.telemetryStore == nil {
			return fmt.Errorf("telemetry store not available")
		}
		now := time.Now().UTC().Format(time.RFC3339)
		row := telemetry.ContainerRow{
			ContainerID:   "bdd-sandbox-" + taskID + "-" + jobID,
			ContainerName: "sandbox-" + jobID,
			Kind:          "sandbox",
			Runtime:       "podman",
			ImageRef:      "test:latest",
			CreatedAt:     now,
			LastSeenAt:    now,
			Status:        "running",
			TaskID:        taskID,
			JobID:         jobID,
			Labels:        map[string]string{"cynodeai.task_id": taskID, "cynodeai.job_id": jobID},
		}
		return state.telemetryStore.UpsertContainerInventory(ctx, &row)
	})
	sc.Step(`^a service log event is recorded for source "([^"]*)" with message "([^"]*)"$`, func(ctx context.Context, sourceName, message string) error {
		if state.telemetryStore == nil {
			return fmt.Errorf("telemetry store not available")
		}
		in := telemetry.LogEventInput{
			LogID:      "bdd-log-" + fmt.Sprintf("%d", time.Now().UnixNano()),
			SourceKind: "service",
			SourceName: sourceName,
			Message:    message,
		}
		return state.telemetryStore.InsertLogEvent(ctx, &in)
	})
	sc.Step(`^the response contains a container with task_id "([^"]*)" and job_id "([^"]*)"$`, func(ctx context.Context, taskID, jobID string) error {
		st := getWorkerState(ctx)
		if st == nil || st.lastBody == nil {
			return fmt.Errorf("no response body")
		}
		var m map[string]interface{}
		if err := json.Unmarshal(st.lastBody, &m); err != nil {
			return err
		}
		containers, _ := m["containers"].([]interface{})
		for _, c := range containers {
			cm, _ := c.(map[string]interface{})
			if cm == nil {
				continue
			}
			t, _ := cm["task_id"].(string)
			j, _ := cm["job_id"].(string)
			if t == taskID && j == jobID {
				return nil
			}
		}
		return fmt.Errorf("response containers did not contain task_id=%q job_id=%q", taskID, jobID)
	})
	sc.Step(`^the response contains a log event with message "([^"]*)"$`, func(ctx context.Context, message string) error {
		st := getWorkerState(ctx)
		if st == nil || st.lastBody == nil {
			return fmt.Errorf("no response body")
		}
		var m map[string]interface{}
		if err := json.Unmarshal(st.lastBody, &m); err != nil {
			return err
		}
		events, _ := m["events"].([]interface{})
		for _, e := range events {
			em, _ := e.(map[string]interface{})
			if em == nil {
				continue
			}
			msg, _ := em["message"].(string)
			if msg == message {
				return nil
			}
		}
		return fmt.Errorf("response events did not contain message %q", message)
	})
}

// RegisterWorkerNodeSBASteps registers step definitions for SBA job spec and result contract (local validation, no orchestrator).
func RegisterWorkerNodeSBASteps(sc *godog.ScenarioContext, state *workerTestState) {
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

	// SBA result contract from task result (mock task result; no orchestrator in worker_node suite)
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

// RegisterNodeManagerConfigSteps registers steps for node manager config fetch and startup features.
func RegisterNodeManagerConfigSteps(sc *godog.ScenarioContext, state *workerTestState) {
	sc.Step(`^a mock orchestrator that returns bootstrap with node_config_url$`, func(ctx context.Context) error {
		state.mu.Lock()
		defer state.mu.Unlock()
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/readyz" && r.Method == http.MethodGet {
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.URL.Path == "/v1/nodes/register" && r.Method == "POST" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(nodepayloads.BootstrapResponse{
					Version:  1,
					IssuedAt: time.Now().UTC().Format(time.RFC3339),
					Orchestrator: nodepayloads.BootstrapOrchestrator{
						BaseURL: srv.URL,
						Endpoints: nodepayloads.BootstrapEndpoints{
							NodeReportURL: srv.URL + "/v1/nodes/capability",
							NodeConfigURL: srv.URL + "/v1/nodes/config",
						},
					},
					Auth: nodepayloads.BootstrapAuth{NodeJWT: "mock-jwt", ExpiresAt: "2026-12-31T00:00:00Z"},
				})
				return
			}
			if r.URL.Path == "/v1/nodes/config" {
				if r.Method == "GET" {
					state.mu.Lock()
					state.getConfigCalled = true
					withManaged := state.mockConfigWithManagedServices
					infVariant := state.mockInferenceBackendVariant
					infImage := state.mockInferenceBackendImage
					state.mu.Unlock()
					infBackend := &nodepayloads.ConfigInferenceBackend{Enabled: true}
					if infVariant != "" || infImage != "" {
						infBackend.Variant = infVariant
						infBackend.Image = infImage
					}
					payload := nodepayloads.NodeConfigurationPayload{
						Version:          1,
						ConfigVersion:    "1",
						IssuedAt:         time.Now().UTC().Format(time.RFC3339),
						NodeSlug:         "bdd-node",
						WorkerAPI:        &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "delivered-token"},
						InferenceBackend: infBackend,
					}
					if withManaged {
						payload.ManagedServices = &nodepayloads.ConfigManagedServices{
							Services: []nodepayloads.ConfigManagedService{
								{ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest"},
							},
						}
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(payload)
					return
				}
				if r.Method == "POST" {
					state.mu.Lock()
					state.postAckCalled = true
					state.mu.Unlock()
					w.WriteHeader(http.StatusNoContent)
				}
				return
			}
			if r.URL.Path == "/v1/nodes/capability" && r.Method == "POST" {
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		state.mockOrch = srv
		return nil
	})
	sc.Step(`^the mock returns node config with worker_api bearer token$`, func(ctx context.Context) error {
		return nil
	})
	sc.Step(`^the mock returns node config with managed_services containing service "([^"]*)" of type "([^"]*)"$`, func(ctx context.Context, serviceID, serviceType string) error {
		state.mu.Lock()
		state.mockConfigWithManagedServices = true
		state.mu.Unlock()
		return nil
	})
	sc.Step(`^the mock returns node config with inference_backend enabled and variant "([^"]*)" and no image$`, func(ctx context.Context, variant string) error {
		state.mu.Lock()
		state.mockInferenceBackendVariant = variant
		state.mockInferenceBackendImage = ""
		state.mu.Unlock()
		return nil
	})
	sc.Step(`^the node manager started the local inference backend with variant "([^"]*)"$`, func(ctx context.Context, wantVariant string) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.nodeManagerErr != nil {
			return fmt.Errorf("node manager failed: %w", st.nodeManagerErr)
		}
		st.mu.Lock()
		img := st.startOllamaImage
		variant := st.startOllamaVariant
		st.mu.Unlock()
		// Ollama: rocm has :rocm tag; cuda/cpu use default image (no separate tag).
		var wantImage string
		if wantVariant == "rocm" {
			wantImage = "ollama/ollama:rocm"
		} else {
			wantImage = "ollama/ollama"
		}
		if img != wantImage {
			return fmt.Errorf("StartOllama image = %q, want %q (derived from variant)", img, wantImage)
		}
		if variant != wantVariant {
			return fmt.Errorf("StartOllama variant = %q, want %q", variant, wantVariant)
		}
		return nil
	})
	sc.Step(`^the node manager runs the startup sequence against the mock orchestrator$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil || st.mockOrch == nil {
			return fmt.Errorf("mock orchestrator not started")
		}
		// Skip container runtime check so BDD does not run real podman/docker.
		prevSkip := os.Getenv("NODE_MANAGER_SKIP_CONTAINER_CHECK")
		_ = os.Setenv("NODE_MANAGER_SKIP_CONTAINER_CHECK", "1")
		// Force no existing inference so StartOllama is invoked when config has inference_backend (BDD determinism).
		prev := os.Getenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE")
		_ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
		// Skip GPU detection in BDD tests (rocm-smi/nvidia-smi not available in CI).
		prevGPU := os.Getenv("NODE_MANAGER_TEST_NO_GPU_DETECT")
		_ = os.Setenv("NODE_MANAGER_TEST_NO_GPU_DETECT", "1")
		defer func() {
			if prevSkip == "" {
				_ = os.Unsetenv("NODE_MANAGER_SKIP_CONTAINER_CHECK")
			} else {
				_ = os.Setenv("NODE_MANAGER_SKIP_CONTAINER_CHECK", prevSkip)
			}
			if prev == "" {
				_ = os.Unsetenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE")
			} else {
				_ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", prev)
			}
			if prevGPU == "" {
				_ = os.Unsetenv("NODE_MANAGER_TEST_NO_GPU_DETECT")
			} else {
				_ = os.Setenv("NODE_MANAGER_TEST_NO_GPU_DETECT", prevGPU)
			}
		}()
		// When config may include managed_services, secure store must be available (syncManagedServiceAgentTokens).
		if st.mockConfigWithManagedServices {
			if st.secureStoreStateDir == "" {
				st.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-phase7-%d", time.Now().UnixNano()))
			}
			if st.secureStoreMasterKey == "" {
				key := make([]byte, 32)
				for i := range key {
					key[i] = byte(i + 10)
				}
				st.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
			}
			_ = os.Setenv("WORKER_API_STATE_DIR", st.secureStoreStateDir)
			_ = os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", st.secureStoreMasterKey)
		}
		cfg := &nodeagent.Config{
			OrchestratorURL:          st.mockOrch.URL,
			NodeSlug:                 "bdd-node",
			NodeName:                 "BDD Node",
			RegistrationPSK:          "psk",
			CapabilityReportInterval: 50 * time.Millisecond,
			HTTPTimeout:              5 * time.Second,
		}
		opts := &nodeagent.RunOptions{
			StartWorkerAPI: func(_ string) error {
				st.mu.Lock()
				st.workerAPIStarted = true
				st.workerAPIStartedAsContainer = st.workerAPIAsContainerImage != ""
				st.mu.Unlock()
				return nil
			},
			StartOllama: func(image, variant string, _ map[string]string) error {
				if st != nil {
					st.mu.Lock()
					st.startOllamaImage = image
					st.startOllamaVariant = variant
					st.mu.Unlock()
					if st.failInferenceStartup {
						return errors.New("inference startup failed")
					}
				}
				return nil
			},
			StartManagedServices: func(_ context.Context, svcs []nodepayloads.ConfigManagedService) error {
				st.mu.Lock()
				st.managedServicesStarted = append([]nodepayloads.ConfigManagedService(nil), svcs...)
				st.mu.Unlock()
				return nil
			},
		}
		runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		runErr := nodeagent.RunWithOptions(runCtx, slog.Default(), cfg, opts)
		if runErr != nil {
			st.nodeManagerErr = runErr
		}
		return nil
	})
	sc.Step(`^the node manager requested config using the bootstrap node_config_url$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		st.mu.Lock()
		ok := st.getConfigCalled
		st.mu.Unlock()
		if !ok {
			return fmt.Errorf("node manager did not request config from mock")
		}
		return nil
	})
	sc.Step(`^the received config contains worker_api orchestrator_bearer_token$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.nodeManagerErr != nil {
			return fmt.Errorf("node manager failed: %w", st.nodeManagerErr)
		}
		return nil
	})
	sc.Step(`^the node manager sent a config acknowledgement with status "([^"]*)"$`, func(ctx context.Context, status string) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		st.mu.Lock()
		ok := st.postAckCalled
		st.mu.Unlock()
		if !ok {
			return fmt.Errorf("node manager did not send config ack")
		}
		if st.nodeManagerErr != nil {
			return fmt.Errorf("node manager failed: %w", st.nodeManagerErr)
		}
		return nil
	})
	sc.Step(`^the node is configured to start worker-api as a container with image "([^"]*)"$`, func(ctx context.Context, image string) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		st.mu.Lock()
		st.workerAPIAsContainerImage = image
		st.mu.Unlock()
		return nil
	})
	sc.Step(`^the worker API was started$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		st.mu.Lock()
		ok := st.workerAPIStarted
		st.mu.Unlock()
		if !ok {
			return fmt.Errorf("worker API was not started")
		}
		if st.nodeManagerErr != nil {
			return fmt.Errorf("node manager failed: %w", st.nodeManagerErr)
		}
		return nil
	})
	sc.Step(`^the worker API was started as a container$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		st.mu.Lock()
		ok := st.workerAPIStartedAsContainer
		st.mu.Unlock()
		if !ok {
			return fmt.Errorf("worker API was not started as a container")
		}
		return nil
	})
	sc.Step(`^the node manager is configured to fail inference startup$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		st.failInferenceStartup = true
		return nil
	})
	sc.Step(`^the node manager exits with an error$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.nodeManagerErr == nil {
			return fmt.Errorf("expected node manager to fail")
		}
		return nil
	})
	sc.Step(`^the error indicates inference startup failed$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil || st.nodeManagerErr == nil {
			return fmt.Errorf("no error to check")
		}
		msg := st.nodeManagerErr.Error()
		if !strings.Contains(msg, "inference") && !strings.Contains(msg, "Ollama") {
			return fmt.Errorf("error %q does not indicate inference startup failure", msg)
		}
		return nil
	})
	sc.Step(`^the worker API returns status 404$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.lastStatus != http.StatusNotFound {
			return fmt.Errorf("expected 404, got %d", st.lastStatus)
		}
		return nil
	})
	sc.Step(`^I POST to the worker API path "([^"]*)" with body "([^"]*)"$`, func(ctx context.Context, path, body string) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		req, err := http.NewRequest(http.MethodPost, st.server.URL+path, strings.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		return nil
	})
	sc.Step(`^the node manager started managed services from config$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		st.mu.Lock()
		svcs := st.managedServicesStarted
		st.mu.Unlock()
		if len(svcs) == 0 {
			return fmt.Errorf("expected node manager to start managed services from config, got none")
		}
		if st.nodeManagerErr != nil {
			return fmt.Errorf("node manager failed: %w", st.nodeManagerErr)
		}
		found := false
		for i := range svcs {
			if strings.TrimSpace(svcs[i].ServiceID) == "pma-main" && strings.TrimSpace(svcs[i].ServiceType) == "pma" {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("expected started services to include pma-main (pma), got %+v", svcs)
		}
		return nil
	})
}

// RegisterInferenceProxySteps registers steps for worker_inference_proxy.feature (Phase 1).
