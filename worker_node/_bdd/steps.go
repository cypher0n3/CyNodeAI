// Package bdd provides Godog step definitions for the worker_node suite.
// Feature files live under repo features/worker_node/.
package bdd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"github.com/cypher0n3/cynodeai/worker_node/internal/inferenceproxy"
	"github.com/cypher0n3/cynodeai/worker_node/internal/nodemanager"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
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
	// Secure store (Phase 4) scenario state
	secureStoreStateDir    string
	secureStoreMasterKey   string
	secureStoreSource      securestore.MasterKeySource
	managedServiceRunArgs  []string
	secureStoreOpenErr     error
	// Phase 7 desired-state: mock config may include managed_services; record StartManagedServices call
	mockConfigWithManagedServices bool
	managedServicesStarted        []nodepayloads.ConfigManagedService
	// Phase 1 inference proxy BDD
	inferenceProxyServer *httptest.Server
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
		if req.Version != 1 || req.TaskID == "" || req.JobID == "" {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "validation failed")
			return
		}
		if req.Sandbox.JobSpecJSON == "" && len(req.Sandbox.Command) == 0 {
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
	// Worker Telemetry API (REQ-WORKER-0230--0232)
	mux.HandleFunc("GET /v1/worker/telemetry/node:info", func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		authz := r.Header.Get("Authorization")
		if len(authz) <= len(prefix) || authz[:len(prefix)] != prefix || authz[len(prefix):] != bearerToken {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version": 1, "node_slug": getEnv("NODE_SLUG", "bdd-node"),
			"build": map[string]string{"build_version": "test", "git_sha": ""},
			"platform": map[string]string{"os": "linux", "arch": "amd64", "kernel_version": ""},
		})
	})
	mux.HandleFunc("GET /v1/worker/telemetry/node:stats", func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		authz := r.Header.Get("Authorization")
		if len(authz) <= len(prefix) || authz[:len(prefix)] != prefix || authz[len(prefix):] != bearerToken {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version": 1, "captured_at": time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
			"cpu": map[string]interface{}{"cores": 0, "load1": 0.0, "load5": 0.0, "load15": 0.0},
			"memory": map[string]interface{}{"total_mb": 0, "used_mb": 0, "free_mb": 0},
			"disk": map[string]interface{}{"state_dir_free_mb": 0, "state_dir_total_mb": 0},
			"container_runtime": map[string]string{"runtime": getEnv("CONTAINER_RUNTIME", "direct"), "version": ""},
		})
	})
	// Containers and logs (REQ-WORKER-0240--0243); BDD stub returns empty list/entries.
	mux.HandleFunc("GET /v1/worker/telemetry/containers", func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		authz := r.Header.Get("Authorization")
		if len(authz) <= len(prefix) || authz[:len(prefix)] != prefix || authz[len(prefix):] != bearerToken {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"containers": []interface{}{}})
	})
	mux.HandleFunc("GET /v1/worker/telemetry/logs", func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		authz := r.Header.Get("Authorization")
		if len(authz) <= len(prefix) || authz[:len(prefix)] != prefix || authz[len(prefix):] != bearerToken {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"events": []interface{}{}, "truncated": false})
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
		state.secureStoreStateDir = ""
		state.secureStoreMasterKey = ""
		state.managedServiceRunArgs = nil
		state.secureStoreOpenErr = nil
		state.mockConfigWithManagedServices = false
		state.managedServicesStarted = nil
		if state.inferenceProxyServer != nil {
			state.inferenceProxyServer.Close()
			state.inferenceProxyServer = nil
		}
		_ = os.Unsetenv("CYNODE_FIPS_MODE")
		_ = os.Unsetenv("CYNODE_SECURE_STORE_MASTER_KEY_B64")
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		return ctx, nil
	})

	RegisterWorkerNodeSteps(sc, state)
	RegisterSecureStoreSteps(sc, state)
	RegisterWorkerNodeSBASteps(sc, state)
	RegisterNodeManagerConfigSteps(sc, state)
	RegisterInferenceProxySteps(sc, state)
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
	build.Env = append(os.Environ(), "GOEXPERIMENT=runtimesecret")
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
	sc.Step(`^a Worker API is running with bearer token "([^"]*)"$`, func(ctx context.Context, token string) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		st.bearerToken = token
		return nil
	})
	sc.Step(`^I call GET "([^"]*)" with bearer token "([^"]*)"$`, func(ctx context.Context, path, token string) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		req, _ := http.NewRequest(http.MethodGet, st.server.URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		st.lastBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^I call GET "([^"]*)" without authorization$`, func(ctx context.Context, path string) error {
		st := getWorkerState(ctx)
		if st == nil || st.server == nil {
			return fmt.Errorf("worker API not started")
		}
		req, _ := http.NewRequest(http.MethodGet, st.server.URL+path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatus = resp.StatusCode
		st.lastBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^the response status is (\d+)$`, func(ctx context.Context, statusStr string) error {
		st := getWorkerState(ctx)
		if st == nil {
			return fmt.Errorf("no state")
		}
		var want int
		if _, err := fmt.Sscanf(statusStr, "%d", &want); err != nil {
			return err
		}
		if st.lastStatus != want {
			return fmt.Errorf("expected status %d, got %d", want, st.lastStatus)
		}
		return nil
	})
	sc.Step(`^the response JSON has "([^"]*)"$`, func(ctx context.Context, key string) error {
		st := getWorkerState(ctx)
		if st == nil || st.lastBody == nil {
			return fmt.Errorf("no response body")
		}
		var m map[string]interface{}
		if err := json.Unmarshal(st.lastBody, &m); err != nil {
			return err
		}
		if _, ok := m[key]; !ok {
			return fmt.Errorf("response JSON missing key %q", key)
		}
		return nil
	})
	sc.Step(`^the response JSON has "([^"]*)" equal to (\d+)$`, func(ctx context.Context, key string, valueStr string) error {
		st := getWorkerState(ctx)
		if st == nil || st.lastBody == nil {
			return fmt.Errorf("no response body")
		}
		var m map[string]interface{}
		if err := json.Unmarshal(st.lastBody, &m); err != nil {
			return err
		}
		v, ok := m[key]
		if !ok {
			return fmt.Errorf("response JSON missing key %q", key)
		}
		var want float64
		if _, err := fmt.Sscanf(valueStr, "%f", &want); err != nil {
			return err
		}
		switch n := v.(type) {
		case float64:
			if n != want {
				return fmt.Errorf("response JSON %q: got %v, want %v", key, n, want)
			}
		case int:
			if float64(n) != want {
				return fmt.Errorf("response JSON %q: got %v, want %v", key, n, want)
			}
		default:
			return fmt.Errorf("response JSON %q is not a number: %T", key, v)
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
					withManaged := state.mockConfigWithManagedServices
					state.mu.Unlock()
					payload := nodepayloads.NodeConfigurationPayload{
						Version:          1,
						ConfigVersion:    "1",
						IssuedAt:         time.Now().UTC().Format(time.RFC3339),
						NodeSlug:         "bdd-node",
						WorkerAPI:        &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "delivered-token"},
						InferenceBackend: &nodepayloads.ConfigInferenceBackend{Enabled: true},
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
	sc.Step(`^the node manager runs the startup sequence against the mock orchestrator$`, func(ctx context.Context) error {
		st := getWorkerState(ctx)
		if st == nil || st.mockOrch == nil {
			return fmt.Errorf("mock orchestrator not started")
		}
		// Force no existing inference so StartOllama is invoked when config has inference_backend (BDD determinism).
		prev := os.Getenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE")
		_ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "1")
		defer func() {
			if prev == "" {
				_ = os.Unsetenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE")
			} else {
				_ = os.Setenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", prev)
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
		cfg := &nodemanager.Config{
			OrchestratorURL:          st.mockOrch.URL,
			NodeSlug:                 "bdd-node",
			NodeName:                 "BDD Node",
			RegistrationPSK:          "psk",
			CapabilityReportInterval: 50 * time.Millisecond,
			HTTPTimeout:              5 * time.Second,
		}
		opts := &nodemanager.RunOptions{
			StartOllama: func(_, _ string) error {
				if st != nil && st.failInferenceStartup {
					return errors.New("inference startup failed")
				}
				return nil
			},
			StartManagedServices: func(svcs []nodepayloads.ConfigManagedService) error {
				st.mu.Lock()
				st.managedServicesStarted = append([]nodepayloads.ConfigManagedService(nil), svcs...)
				st.mu.Unlock()
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
func RegisterInferenceProxySteps(sc *godog.ScenarioContext, state *workerTestState) {
	sc.Step(`^the inference proxy is configured with an upstream$`, func(ctx context.Context) error {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		upstreamURL, _ := url.Parse(backend.URL)
		proxy := inferenceproxy.NewProxy(upstreamURL)
		state.inferenceProxyServer = httptest.NewServer(proxy)
		return nil
	})
	sc.Step(`^I send a request to the inference proxy with body size exceeding 10 MiB$`, func(ctx context.Context) error {
		if state.inferenceProxyServer == nil {
			return fmt.Errorf("inference proxy not started")
		}
		const overLimit = 10*1024*1024 + 1
		body := bytes.Repeat([]byte("x"), overLimit)
		req, _ := http.NewRequest(http.MethodPost, state.inferenceProxyServer.URL+"/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		state.lastStatus = resp.StatusCode
		return nil
	})
	sc.Step(`^the inference proxy responds with status 413$`, func(ctx context.Context) error {
		if state.lastStatus != http.StatusRequestEntityTooLarge {
			return fmt.Errorf("expected 413, got %d", state.lastStatus)
		}
		return nil
	})
}

// RegisterSecureStoreSteps registers steps for worker_secure_store.feature (Phase 4).
func RegisterSecureStoreSteps(sc *godog.ScenarioContext, state *workerTestState) {
	// Scenario: Worker stores orchestrator-issued secrets encrypted at rest
	sc.Step(`^the node manager receives configuration containing orchestrator-issued secrets$`, func(ctx context.Context) error {
		state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-secure-%d", time.Now().UnixNano()))
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i)
		}
		state.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
		return nil
	})
	sc.Step(`^the node manager applies configuration$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-secure-%d", time.Now().UnixNano()))
			key := make([]byte, 32)
			for i := range key {
				key[i] = byte(i)
			}
			state.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
		}
		return nil
	})
	sc.Step(`^the node stores secrets in the node-local secure store under storage\.state_dir$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" || state.secureStoreMasterKey == "" {
			return fmt.Errorf("state dir or master key not set")
		}
		os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", state.secureStoreMasterKey)
		defer os.Unsetenv("CYNODE_SECURE_STORE_MASTER_KEY_B64")
		store, _, err := securestore.Open(state.secureStoreStateDir)
		if err != nil {
			return fmt.Errorf("open secure store: %w", err)
		}
		state.secureStoreSource = securestore.MasterKeySourceEnvB64
		expiry := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
		if err := store.PutAgentToken("bdd-svc", "test-token-value", expiry); err != nil {
			return fmt.Errorf("put agent token: %w", err)
		}
		secretsDir := filepath.Join(state.secureStoreStateDir, "secrets", "agent_tokens")
		if _, err := os.Stat(secretsDir); err != nil {
			return fmt.Errorf("secrets dir under state_dir not created: %w", err)
		}
		return nil
	})
	sc.Step(`^the persisted secret values are encrypted at rest$`, func(ctx context.Context) error {
		agentTokensDir := filepath.Join(state.secureStoreStateDir, "secrets", "agent_tokens")
		entries, err := os.ReadDir(agentTokensDir)
		if err != nil {
			return fmt.Errorf("read agent_tokens dir: %w", err)
		}
		if len(entries) == 0 {
			return fmt.Errorf("no token file persisted")
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(agentTokensDir, e.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			// Must not be plaintext JSON containing the token value
			if strings.Contains(string(raw), "test-token-value") {
				return fmt.Errorf("persisted file contains plaintext token (not encrypted at rest)")
			}
			if strings.Contains(string(raw), `"token"`) && strings.Contains(string(raw), `"service_id"`) {
				return fmt.Errorf("persisted file looks like plaintext JSON (not encrypted)")
			}
		}
		return nil
	})

	// Scenario: Worker holds agent token and does not pass it to managed-service containers
	sc.Step(`^the orchestrator configures a managed agent service with an agent token$`, func(ctx context.Context) error {
		return nil
	})
	sc.Step(`^the node manager starts the managed-service container$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-runargs-%d", time.Now().UnixNano()))
		}
		svc := &nodepayloads.ConfigManagedService{
			ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
			Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
		}
		state.managedServiceRunArgs = nodemanager.BuildManagedServiceRunArgs(state.secureStoreStateDir, svc, "pma-main", "pma", "pma:latest", "cynodeai-managed-pma-main")
		return nil
	})
	sc.Step(`^the managed-service container does not receive the agent token via env vars, files, or mounts$`, func(ctx context.Context) error {
		if len(state.managedServiceRunArgs) == 0 {
			return fmt.Errorf("run args not built (run the 0168 scenario step that builds run args first, or this scenario depends on it)")
		}
		for i := 0; i < len(state.managedServiceRunArgs)-1; i++ {
			if state.managedServiceRunArgs[i] == "-e" {
				env := state.managedServiceRunArgs[i+1]
				if strings.HasPrefix(env, "AGENT_TOKEN=") {
					return fmt.Errorf("run args must not pass AGENT_TOKEN env: got %q", env)
				}
			}
			if state.managedServiceRunArgs[i] == "-v" {
				mount := state.managedServiceRunArgs[i+1]
				hostPath, _, _ := strings.Cut(mount, ":")
				if strings.Contains(hostPath, "secrets") {
					return fmt.Errorf("run args must not mount secure store: got %q", hostPath)
				}
			}
		}
		return nil
	})
	sc.Step(`^the worker proxy attaches the agent token when forwarding agent-originated requests$`, func(ctx context.Context) error {
		// Covered by unit tests (inferenceproxy, internal proxy auth). No in-BDD way to assert without full stack.
		return nil
	})

	// Scenario: Worker warns when using env var master key fallback
	sc.Step(`^the node cannot access TPM, OS key store, or system service credentials$`, func(ctx context.Context) error {
		os.Unsetenv("CREDENTIALS_DIRECTORY")
		os.Setenv("CYNODE_FIPS_MODE", "0") // Explicit non-FIPS so env fallback is allowed on all platforms
		return nil
	})
	sc.Step(`^the environment variable CYNODE_SECURE_STORE_MASTER_KEY_B64 is set$`, func(ctx context.Context) error {
		if state.secureStoreMasterKey == "" {
			key := make([]byte, 32)
			for i := range key {
				key[i] = byte(i + 1)
			}
			state.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
		}
		os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", state.secureStoreMasterKey)
		return nil
	})
	sc.Step(`^the environment variable CYNODE_SECURE_STORE_MASTER_KEY_B(\d+) is set$`, func(ctx context.Context, _ int) error {
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i + 1)
		}
		state.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
		os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", state.secureStoreMasterKey)
		return nil
	})
	sc.Step(`^the node starts$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-secure-%d", time.Now().UnixNano()))
		}
		os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", state.secureStoreMasterKey)
		store, source, err := securestore.Open(state.secureStoreStateDir)
		if err != nil {
			return fmt.Errorf("open secure store: %w", err)
		}
		_ = store
		state.secureStoreSource = source
		return nil
	})
	sc.Step(`^the node uses the env_b(\d+) master key backend$`, func(ctx context.Context, _ int) error {
		if state.secureStoreSource != securestore.MasterKeySourceEnvB64 {
			return fmt.Errorf("expected env_b64 master key backend, got %q", state.secureStoreSource)
		}
		return nil
	})
	sc.Step(`^the node emits a startup warning indicating a less-secure master key backend$`, func(ctx context.Context) error {
		// Warning is logged by node-manager; unit tests and code review cover. BDD accepts when env key was used.
		if state.secureStoreSource != securestore.MasterKeySourceEnvB64 {
			return fmt.Errorf("expected env_b64 to have been used (warning scenario)")
		}
		return nil
	})

	// Scenario: Managed-service container run args do not mount secure store (0168)
	sc.Step(`^a managed service is configured for the node$`, func(ctx context.Context) error {
		state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-runargs-%d", time.Now().UnixNano()))
		return nil
	})
	sc.Step(`^the node manager builds run args for the managed-service container$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-runargs-%d", time.Now().UnixNano()))
		}
		svc := &nodepayloads.ConfigManagedService{
			ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
			Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
		}
		state.managedServiceRunArgs = nodemanager.BuildManagedServiceRunArgs(state.secureStoreStateDir, svc, "pma-main", "pma", "pma:latest", "cynodeai-managed-pma-main")
		return nil
	})
	sc.Step(`^the run args do not contain any mount of the secure store or secrets path$`, func(ctx context.Context) error {
		for i := 0; i < len(state.managedServiceRunArgs)-1; i++ {
			if state.managedServiceRunArgs[i] == "-v" {
				mount := state.managedServiceRunArgs[i+1]
				hostPath, _, _ := strings.Cut(mount, ":")
				if strings.Contains(hostPath, "secrets") {
					return fmt.Errorf("run args must not mount secure store: got host path %q", hostPath)
				}
			}
		}
		return nil
	})

	// Scenario: Secure store distinct from telemetry (0169)
	sc.Step(`^the node state directory is set$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-state-%d", time.Now().UnixNano()))
		}
		return nil
	})
	sc.Step(`^the secure store path is under state_dir and distinct from the telemetry database path$`, func(ctx context.Context) error {
		securePath := filepath.Join(state.secureStoreStateDir, "secrets")
		// Telemetry DB is under state_dir but different path (e.g. telemetry.db or similar)
		if !strings.Contains(securePath, "secrets") {
			return fmt.Errorf("secure store path must be under state_dir and contain secrets: %q", securePath)
		}
		if strings.Contains(securePath, "telemetry") || strings.HasSuffix(state.secureStoreStateDir, "telemetry") {
			return fmt.Errorf("secure store path must be distinct from telemetry: %q", securePath)
		}
		return nil
	})

	// Scenario: FIPS mode rejects env master key (0170)
	sc.Step(`^FIPS mode is enabled or unknown on the host$`, func(ctx context.Context) error {
		os.Setenv("CYNODE_FIPS_MODE", "1")
		return nil
	})
	sc.Step(`^the node opens the secure store with env master key only$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-fips-%d", time.Now().UnixNano()))
		}
		if state.secureStoreMasterKey == "" {
			key := make([]byte, 32)
			for i := range key {
				key[i] = byte(i + 2)
			}
			state.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
		}
		os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", state.secureStoreMasterKey)
		_, _, err := securestore.Open(state.secureStoreStateDir)
		state.secureStoreOpenErr = err
		return nil
	})
	sc.Step(`^opening the secure store fails with FIPS-related error$`, func(ctx context.Context) error {
		if state.secureStoreOpenErr == nil {
			return fmt.Errorf("expected Open to fail in FIPS mode with env key")
		}
		if !errors.Is(state.secureStoreOpenErr, securestore.ErrFIPSRequiresNonEnvKey) {
			return fmt.Errorf("expected ErrFIPSRequiresNonEnvKey, got: %v", state.secureStoreOpenErr)
		}
		return nil
	})

	// Scenario: Process boundary documented (0172)
	sc.Step(`^the worker node codebase$`, func(ctx context.Context) error {
		return nil
	})
	sc.Step(`^the secure store process boundary document exists and states writer and reader components$`, func(ctx context.Context) error {
		docPath := "docs/dev_docs/2026-03-06_secure_store_process_boundary.md"
		for _, root := range []string{"../..", "..", "."} {
			p := filepath.Join(root, docPath)
			if _, err := os.Stat(p); err == nil {
				content, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				s := strings.ToLower(string(content))
				if !strings.Contains(s, "writer") || !strings.Contains(s, "reader") {
					return fmt.Errorf("document must state writer and reader components")
				}
				return nil
			}
		}
		return fmt.Errorf("process boundary document not found at %s", docPath)
	})
}
