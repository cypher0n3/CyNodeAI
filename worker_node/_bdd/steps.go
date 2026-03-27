// Package bdd provides Godog step definitions for the worker_node suite.
// Feature files live under repo features/worker_node/.
package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/internal/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
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
	secureStoreStateDir   string
	secureStoreMasterKey  string
	secureStoreSource     securestore.MasterKeySource
	managedServiceRunArgs []string
	secureStoreOpenErr    error
	// Phase 7 desired-state: mock config may include managed_services; record StartManagedServices call
	mockConfigWithManagedServices bool
	managedServicesStarted        []nodepayloads.ConfigManagedService
	// Inference backend variant/image for config (REQ-WORKER-0253)
	mockInferenceBackendVariant string
	mockInferenceBackendImage   string
	startOllamaImage            string
	startOllamaVariant          string
	// Phase 1 inference proxy BDD
	inferenceProxyServer *httptest.Server
	// Telemetry store for Worker Telemetry API BDD (containers, logs with data)
	telemetryStore    *telemetry.Store
	telemetryStateDir string
	// Task result contract scenario (SBA result from task result; mock, no orchestrator)
	taskResultJSON []byte
	taskStatus     string
	firstJobResult map[string]interface{}
	// Worker API as managed service (container vs binary)
	workerAPIStarted            bool
	workerAPIStartedAsContainer bool
	workerAPIAsContainerImage   string
	// UDS inference proxy (REQ-WORKER-0270 / REQ-SANDBX-0131) scenario state
	inferenceProxySocketPath string
	inferenceProxyUDSCancel  context.CancelFunc
	inferenceProxyUDSDone    chan int
	sbaPodRunArgs            []string
	// SBA non-pod (direct) path UDS contract state
	sbaDirectExecutor           *executor.Executor
	sbaDirectRunArgs            []string
	sbaInferenceProxySocketPath string
}

func getWorkerState(ctx context.Context) *workerTestState {
	s, _ := ctx.Value(stateKey).(*workerTestState)
	return s
}

// workerMux builds the same routes as worker-api main for testing.
// If telemetryStore is non-nil, GET .../containers and GET .../logs use the store; otherwise they return empty.
func workerMux(exec *executor.Executor, bearerToken string, telemetryStore *telemetry.Store) *http.ServeMux {
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
			"build":    map[string]string{"build_version": "test"},
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
			"cpu":               map[string]interface{}{"cores": 0, "load1": 0.0, "load5": 0.0, "load15": 0.0},
			"memory":            map[string]interface{}{"total_mb": 0, "used_mb": 0, "free_mb": 0},
			"disk":              map[string]interface{}{"state_dir_free_mb": 0, "state_dir_total_mb": 0},
			"container_runtime": map[string]string{"runtime": getEnv("CONTAINER_RUNTIME", "direct"), "version": ""},
		})
	})
	// Containers and logs (REQ-WORKER-0240--0243); use store when non-nil, else empty.
	registerTelemetryHandlers(mux, bearerToken, telemetryStore)
	return mux
}

func registerTelemetryHandlers(mux *http.ServeMux, bearerToken string, store *telemetry.Store) {
	authTelemetry := func(w http.ResponseWriter, r *http.Request) bool {
		const prefix = "Bearer "
		authz := r.Header.Get("Authorization")
		if len(authz) <= len(prefix) || authz[:len(prefix)] != prefix || authz[len(prefix):] != bearerToken {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return false
		}
		return true
	}
	mux.HandleFunc("GET /v1/worker/telemetry/containers", func(w http.ResponseWriter, r *http.Request) {
		if !authTelemetry(w, r) {
			return
		}
		if store == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"version": 1, "containers": []interface{}{}})
			return
		}
		q := r.URL.Query()
		limit := 100
		if l := q.Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		list, nextToken, err := store.ListContainers(r.Context(), q.Get("kind"), q.Get("status"), q.Get("task_id"), q.Get("job_id"), q.Get("page_token"), limit)
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "")
			return
		}
		if list == nil {
			list = []telemetry.ContainerRow{}
		}
		resp := map[string]interface{}{"version": 1, "containers": list}
		if nextToken != "" {
			resp["next_page_token"] = nextToken
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("GET /v1/worker/telemetry/containers/", func(w http.ResponseWriter, r *http.Request) {
		if !authTelemetry(w, r) {
			return
		}
		containerID := strings.TrimPrefix(r.URL.Path, "/v1/worker/telemetry/containers/")
		if containerID == "" {
			writeProblem(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", "container_id required")
			return
		}
		if store == nil {
			writeProblem(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", "container not found")
			return
		}
		c, err := store.GetContainer(r.Context(), containerID)
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "")
			return
		}
		if c == nil {
			writeProblem(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", "container not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"version": 1, "container": c})
	})
	mux.HandleFunc("GET /v1/worker/telemetry/logs", func(w http.ResponseWriter, r *http.Request) {
		if !authTelemetry(w, r) {
			return
		}
		if store == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"version": 1, "events": []interface{}{}, "truncated": map[string]interface{}{"limited_by": "none", "max_bytes": 1048576}})
			return
		}
		q := r.URL.Query()
		if q.Get("source_kind") == "" && q.Get("container_id") == "" {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "source_kind+source_name or source_kind=container+container_id required")
			return
		}
		limit := 1000
		if l := q.Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 5000 {
				limit = n
			}
		}
		events, truncated, nextToken, err := store.QueryLogs(r.Context(), q.Get("source_kind"), q.Get("source_name"), q.Get("container_id"), q.Get("stream"), q.Get("since"), q.Get("until"), q.Get("page_token"), limit)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", err.Error())
			return
		}
		if events == nil {
			events = []telemetry.LogEventRow{}
		}
		resp := map[string]interface{}{"version": 1, "events": events, "truncated": truncated}
		if nextToken != "" {
			resp["next_page_token"] = nextToken
		}
		writeJSON(w, http.StatusOK, resp)
	})
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
	sc.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
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
		state.telemetryStateDir, _ = os.MkdirTemp("", "bdd-telemetry-")
		if state.telemetryStateDir != "" {
			if ts, err := telemetry.Open(ctx, state.telemetryStateDir); err == nil {
				state.telemetryStore = ts
			}
		}
		state.server = httptest.NewServer(workerMux(exec, state.bearerToken, state.telemetryStore))
		return context.WithValue(ctx, stateKey, state), nil
	})

	sc.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		if state.server != nil {
			state.server.Close()
		}
		if state.mockOrch != nil {
			state.mockOrch.Close()
		}
		if state.telemetryStore != nil {
			_ = state.telemetryStore.Close()
			state.telemetryStore = nil
		}
		if state.telemetryStateDir != "" {
			_ = os.RemoveAll(state.telemetryStateDir)
			state.telemetryStateDir = ""
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
		state.mockInferenceBackendVariant = ""
		state.mockInferenceBackendImage = ""
		state.startOllamaImage = ""
		state.startOllamaVariant = ""
		if state.inferenceProxyServer != nil {
			state.inferenceProxyServer.Close()
			state.inferenceProxyServer = nil
		}
		state.taskResultJSON = nil
		state.taskStatus = ""
		state.firstJobResult = nil
		_ = os.Unsetenv("CYNODE_FIPS_MODE")
		_ = os.Unsetenv("CYNODE_SECURE_STORE_MASTER_KEY_B64")
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		return ctx, nil
	})

	RegisterWorkerNodeSteps(sc, state)
	RegisterTelemetryDataSteps(sc, state)
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

// ensureNodeManagerBinary returns the path to the node-manager binary (cynodeai-wnm), building it if needed (black-box: no internal import).
// Resolves worker_node root whether tests run from repo root, worker_node, or worker_node/_bdd.
func ensureNodeManagerBinary() (string, error) {
	nodeManagerBin := "cynodeai-wnm"
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
		tryBin := filepath.Join(root, "bin", nodeManagerBin)
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
	outBin := filepath.Join(moduleRoot, "bin", nodeManagerBin)
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

// RegisterTelemetryDataSteps registers steps for Worker Telemetry API scenarios that assert on stored data (containers, logs).
