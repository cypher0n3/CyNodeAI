// Package bdd provides Godog step definitions for the worker_node suite.
// Feature files live under repo features/worker_node/.
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
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/cmd/worker-api/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/nodemanager"
)

type ctxKey int

const stateKey ctxKey = 0

type workerTestState struct {
	server               *httptest.Server
	bearerToken          string
	lastStatus           int
	lastBody             []byte
	mockOrch             *httptest.Server
	getConfigCalled      bool
	postAckCalled        bool
	nodeManagerErr       error
	failInferenceStartup bool
	mu                   sync.Mutex
	// SBA job spec validation (local, no orchestrator)
	sbaJobSpecBytes       []byte
	sbaValidationErr      error
	sbaValidationErrField string
	sbaResult             *sbajob.Result
	sbaResultJSON         []byte
}

func getWorkerState(ctx context.Context) *workerTestState {
	s, _ := ctx.Value(stateKey).(*workerTestState)
	return s
}

// workerMux builds the same routes as worker-api main for testing.
func workerMux(exec *executor.Executor, bearerToken string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ready, reason := exec.Ready(r.Context())
		if ready {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(reason))
	})
	mux.HandleFunc("POST /v1/worker/jobs:run", func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		authz := r.Header.Get("Authorization")
		if len(authz) <= len(prefix) || authz[:len(prefix)] != prefix || authz[len(prefix):] != bearerToken {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		maxRequestBytes := int64(10 * 1024 * 1024)
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
		var req workerapi.RunJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if err != nil && strings.Contains(err.Error(), "request body too large") {
				writeProblem(w, http.StatusRequestEntityTooLarge, problem.TypeValidation, "Request Entity Too Large", "Request body exceeds maximum size")
				return
			}
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "Invalid request body")
			return
		}
		if req.Version != 1 || req.TaskID == "" || req.JobID == "" || len(req.Sandbox.Command) == 0 {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "validation failed")
			return
		}
		resp, err := exec.RunJob(r.Context(), &req, "")
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "Job execution failed")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
	return mux
}

func writeProblem(w http.ResponseWriter, status int, typ, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem.Details{Type: typ, Title: title, Status: status, Detail: detail})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// InitializeWorkerNodeSuite sets up the godog suite for worker_node features.
func InitializeWorkerNodeSuite(sc *godog.ScenarioContext, state *workerTestState) {
	sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
		exec := executor.New(
			getEnv("CONTAINER_RUNTIME", "direct"),
			30*time.Second,
			1<<20,
			getEnv("OLLAMA_UPSTREAM_URL", "http://localhost:11434"),
			"",
			nil,
		)
		state.bearerToken = "test-bearer-token"
		if t := os.Getenv("WORKER_API_BEARER_TOKEN"); t != "" {
			state.bearerToken = t
		}
		state.server = httptest.NewServer(workerMux(exec, state.bearerToken))
		return context.WithValue(ctx, stateKey, state), nil
	})

	sc.After(func(ctx context.Context, s *godog.Scenario, err error) (context.Context, error) {
		if state.server != nil {
			state.server.Close()
		}
		if state.mockOrch != nil {
			state.mockOrch.Close()
		}
		state.server = nil
		state.mockOrch = nil
		state.lastStatus = 0
		state.lastBody = nil
		state.getConfigCalled = false
		state.postAckCalled = false
		state.nodeManagerErr = nil
		state.failInferenceStartup = false
		state.sbaJobSpecBytes = nil
		state.sbaValidationErr = nil
		state.sbaValidationErrField = ""
		state.sbaResult = nil
		state.sbaResultJSON = nil
		return ctx, nil
	})

	RegisterWorkerNodeSteps(sc, state)
	RegisterWorkerNodeSBASteps(sc, state)
	RegisterNodeManagerConfigSteps(sc, state)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ensureNodeManagerBinary returns the path to the node-manager binary, building it if needed (black-box: no internal import).
// Resolves worker_node root whether tests run from repo root, worker_node, or worker_node/_bdd.
func ensureNodeManagerBinary() (string, error) {
	if p := os.Getenv("NODE_MANAGER_BIN"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("NODE_MANAGER_BIN=%q not found: %w", p, err)
		}
		return p, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	// Candidate worker_node roots: parent of cwd (when cwd is _bdd), cwd (when in worker_node), cwd/worker_node (when in repo root).
	tryDirs := []string{filepath.Dir(cwd), cwd}
	if b := filepath.Join(cwd, "worker_node"); dirExists(b) {
		tryDirs = append(tryDirs, b)
	}
	for _, root := range tryDirs {
		tryBin := filepath.Join(root, "bin", "node-manager")
		if dirExists(filepath.Join(root, "cmd", "node-manager")) && fileExists(tryBin) {
			return tryBin, nil
		}
	}
	// Build: find worker_node root (has cmd/node-manager).
	var moduleRoot string
	for _, root := range tryDirs {
		cmdDir := filepath.Join(root, "cmd", "node-manager")
		if fileExists(cmdDir) || dirExists(cmdDir) {
			moduleRoot = root
			break
		}
	}
	if moduleRoot == "" {
		return "", fmt.Errorf("node-manager source not found (run from repo root or worker_node)")
	}
	outBin := filepath.Join(moduleRoot, "bin", "node-manager")
	if err := os.MkdirAll(filepath.Dir(outBin), 0o755); err != nil {
		return "", fmt.Errorf("mkdir bin: %w", err)
	}
	build := exec.Command("go", "build", "-o", outBin, "./cmd/node-manager")
	build.Dir = moduleRoot
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build node-manager: %w: %s", err, bytes.TrimSpace(out))
	}
	return outBin, nil
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// RegisterWorkerNodeSteps registers step definitions for worker_node features.
func RegisterWorkerNodeSteps(sc *godog.ScenarioContext, state *workerTestState) {
	sc.Step(`^the worker API is running$`, func(ctx context.Context) error {
		if getWorkerState(ctx) == nil || getWorkerState(ctx).server == nil {
			return fmt.Errorf("worker API not started")
		}
		return nil
	})
	sc.Step(`^the worker API is configured with a valid bearer token$`, func(ctx context.Context) error {
		return nil
	})
	sc.Step(`^I call the worker API without a bearer token$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		body := []byte(`{"version":1,"task_id":"t1","job_id":"j1","sandbox":{"image":"alpine:latest","command":["echo","x"]}}`)
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/worker/jobs:run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		st.lastBody, _ = json.Marshal(map[string]string{"read": "skipped"})
		return nil
	})
	sc.Step(`^the worker API rejects the request$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.lastStatus != http.StatusUnauthorized {
			return fmt.Errorf("expected 401, got %d", st.lastStatus)
		}
		return nil
	})
	sc.Step(`^I call the worker API with an invalid bearer token$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		body := []byte(`{"version":1,"task_id":"t1","job_id":"j1","sandbox":{"image":"alpine:latest","command":["echo","x"]}}`)
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/worker/jobs:run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer invalid-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		return nil
	})
	sc.Step(`^I call GET /readyz on the worker API$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		resp, err := http.Get(st.server.URL + "/readyz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		body, _ := io.ReadAll(resp.Body)
		st.lastBody, _ = json.Marshal(map[string]string{"body": string(body)})
		return nil
	})
	sc.Step(`^the worker API returns status 200$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.lastStatus != http.StatusOK {
			return fmt.Errorf("expected 200, got %d", st.lastStatus)
		}
		return nil
	})
	sc.Step(`^the response body is "([^"]*)"$`, func(ctx context.Context, want string) error {
		st := getWorkerState(ctx)
		if st == nil || st.lastBody == nil {
			return fmt.Errorf("no response body")
		}
		var m map[string]string
		if err := json.Unmarshal(st.lastBody, &m); err != nil {
			return err
		}
		got := m["body"]
		if got != want {
			return fmt.Errorf("response body %q, want %q", got, want)
		}
		return nil
	})
	sc.Step(`^I submit a sandbox job request with body size exceeding the limit$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		// Body > 10 MiB so server returns 413; use valid JSON shape so error is "request body too large"
		big := bytes.Repeat([]byte("x"), 11*1024*1024)
		body := []byte(`{"version":1,"task_id":"t","job_id":"j","sandbox":{"image":"a","command":["`)
		body = append(body, big...)
		body = append(body, []byte(`"]}}`)...)
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/worker/jobs:run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.bearerToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		return nil
	})
	sc.Step(`^the worker API returns status 413$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.lastStatus != http.StatusRequestEntityTooLarge {
			return fmt.Errorf("expected 413, got %d", st.lastStatus)
		}
		return nil
	})
	sc.Step(`^I submit a sandbox job that runs command "([^"]*)"$`, func(ctx context.Context, cmd string) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		body, _ := json.Marshal(map[string]interface{}{
			"version": 1,
			"task_id": "bdd-task",
			"job_id":  "bdd-job",
			"sandbox": map[string]interface{}{
				"image":   "alpine:latest",
				"command": []string{"sh", "-c", cmd},
			},
		})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/worker/jobs:run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.bearerToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		st.lastBody = nil
		if resp.StatusCode == http.StatusOK {
			var dec workerapi.RunJobResponse
			if err := json.NewDecoder(resp.Body).Decode(&dec); err == nil {
				st.lastBody, _ = json.Marshal(dec)
			}
		}
		return nil
	})
	sc.Step(`^the sandbox job result contains stdout "([^"]*)"$`, func(ctx context.Context, want string) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.lastStatus != http.StatusOK {
			return fmt.Errorf("expected 200, got %d", st.lastStatus)
		}
		var dec workerapi.RunJobResponse
		if err := json.Unmarshal(st.lastBody, &dec); err != nil {
			return err
		}
		if strings.TrimSpace(dec.Stdout) != want {
			return fmt.Errorf("stdout %q, want %q", dec.Stdout, want)
		}
		return nil
	})
	sc.Step(`^the sandbox job exit code is (\d+)$`, func(ctx context.Context, codeStr string) error {
		var code int
		if _, err := fmt.Sscanf(codeStr, "%d", &code); err != nil {
			return err
		}
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.lastStatus != http.StatusOK {
			return fmt.Errorf("expected 200, got %d", st.lastStatus)
		}
		var dec workerapi.RunJobResponse
		if err := json.Unmarshal(st.lastBody, &dec); err != nil {
			return err
		}
		if dec.ExitCode != code {
			return fmt.Errorf("exit code %d, want %d", dec.ExitCode, code)
		}
		return nil
	})
	sc.Step(`^I submit a sandbox job with network_policy "([^"]*)" that runs command "([^"]*)"$`, func(ctx context.Context, networkPolicy, cmd string) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		body, _ := json.Marshal(map[string]interface{}{
			"version": 1,
			"task_id": "bdd-task",
			"job_id":  "bdd-job",
			"sandbox": map[string]interface{}{
				"image":          "alpine:latest",
				"command":        []string{"sh", "-c", cmd},
				"network_policy": networkPolicy,
			},
		})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/worker/jobs:run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.bearerToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		st.lastBody = nil
		if resp.StatusCode == http.StatusOK {
			var dec workerapi.RunJobResponse
			if err := json.NewDecoder(resp.Body).Decode(&dec); err == nil {
				st.lastBody, _ = json.Marshal(dec)
			}
		}
		return nil
	})
	sc.Step(`^the sandbox job completes successfully$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.lastStatus != http.StatusOK {
			return fmt.Errorf("expected 200, got %d", st.lastStatus)
		}
		var dec workerapi.RunJobResponse
		if err := json.Unmarshal(st.lastBody, &dec); err != nil {
			return err
		}
		if dec.Status != workerapi.StatusCompleted {
			return fmt.Errorf("status %q, want completed", dec.Status)
		}
		return nil
	})
	sc.Step(`^the sandbox job result stdout contains "([^"]*)"$`, func(ctx context.Context, want string) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		if st.lastStatus != http.StatusOK {
			return fmt.Errorf("expected 200, got %d", st.lastStatus)
		}
		var dec workerapi.RunJobResponse
		if err := json.Unmarshal(st.lastBody, &dec); err != nil {
			return err
		}
		if !strings.Contains(dec.Stdout, want) {
			return fmt.Errorf("stdout %q does not contain %q", dec.Stdout, want)
		}
		return nil
	})
	sc.Step(`^I submit a sandbox job with use_inference that runs command "([^"]*)"$`, func(ctx context.Context, cmd string) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		body, _ := json.Marshal(map[string]interface{}{
			"version": 1,
			"task_id": "bdd-task",
			"job_id":  "bdd-job",
			"sandbox": map[string]interface{}{
				"image":         "alpine:latest",
				"command":       []string{"sh", "-c", cmd},
				"use_inference": true,
			},
		})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/worker/jobs:run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.bearerToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		st.lastBody = nil
		if resp.StatusCode == http.StatusOK {
			var dec workerapi.RunJobResponse
			if err := json.NewDecoder(resp.Body).Decode(&dec); err == nil {
				st.lastBody, _ = json.Marshal(dec)
			}
		}
		return nil
	})
	sc.Step(`^I submit a sandbox job with env "([^"]*)" that runs command "([^"]*)"$`, func(ctx context.Context, envKV, cmd string) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		env := make(map[string]string)
		if envKV != "" {
			parts := strings.SplitN(envKV, "=", 2)
			if len(parts) == 2 {
				env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		body, _ := json.Marshal(map[string]interface{}{
			"version": 1,
			"task_id": "bdd-task",
			"job_id":  "bdd-job",
			"sandbox": map[string]interface{}{
				"image":   "alpine:latest",
				"command": []string{"sh", "-c", cmd},
				"env":     env,
			},
		})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/worker/jobs:run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.bearerToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		st.lastBody = nil
		if resp.StatusCode == http.StatusOK {
			var dec workerapi.RunJobResponse
			if err := json.NewDecoder(resp.Body).Decode(&dec); err == nil {
				st.lastBody, _ = json.Marshal(dec)
			}
		}
		return nil
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
}

// RegisterNodeManagerConfigSteps registers steps for node manager config fetch and startup features.
func RegisterNodeManagerConfigSteps(sc *godog.ScenarioContext, state *workerTestState) {
	sc.Step(`^a mock orchestrator that returns bootstrap with node_config_url$`, func(ctx context.Context) error {
		state.mu.Lock()
		defer state.mu.Unlock()
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
					state.mu.Unlock()
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(nodepayloads.NodeConfigurationPayload{
						Version:       1,
						ConfigVersion: "1",
						IssuedAt:      time.Now().UTC().Format(time.RFC3339),
						NodeSlug:      "bdd-node",
						WorkerAPI:     &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "delivered-token"},
					})
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
	sc.Step(`^the node manager runs the startup sequence against the mock orchestrator$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil || st.mockOrch == nil {
			return fmt.Errorf("mock orchestrator not started")
		}
		cfg := &nodemanager.Config{
			OrchestratorURL:          st.mockOrch.URL,
			NodeSlug:                 "bdd-node",
			NodeName:                 "BDD Node",
			RegistrationPSK:          "psk",
			CapabilityReportInterval: 50 * time.Millisecond,
			HTTPTimeout:              5 * time.Second,
		}
		opts := &nodemanager.RunOptions{
			StartOllama: func() error {
				if st != nil && st.failInferenceStartup {
					return errors.New("inference startup failed")
				}
				return nil
			},
		}
		runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		runErr := nodemanager.RunWithOptions(runCtx, slog.Default(), cfg, opts)
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
}
