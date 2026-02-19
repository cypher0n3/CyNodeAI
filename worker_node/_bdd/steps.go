// Package bdd provides Godog step definitions for the worker_node suite.
// Feature files live under repo features/worker_node/.
package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/cmd/worker-api/executor"
)

type ctxKey int

const stateKey ctxKey = 0

type workerTestState struct {
	server      *httptest.Server
	bearerToken string
	lastStatus  int
	lastBody    []byte
}

func getWorkerState(ctx context.Context) *workerTestState {
	s, _ := ctx.Value(stateKey).(*workerTestState)
	return s
}

// workerMux builds the same routes as worker-api main for testing.
func workerMux(exec *executor.Executor, bearerToken string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /v1/worker/jobs:run", func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		authz := r.Header.Get("Authorization")
		if len(authz) <= len(prefix) || authz[:len(prefix)] != prefix || authz[len(prefix):] != bearerToken {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)
		var req workerapi.RunJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "Invalid request body")
			return
		}
		if req.Version != 1 || req.TaskID == "" || req.JobID == "" || len(req.Sandbox.Command) == 0 {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "validation failed")
			return
		}
		resp, err := exec.RunJob(r.Context(), &req)
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
		state.server = nil
		state.lastStatus = 0
		state.lastBody = nil
		return ctx, nil
	})

	RegisterWorkerNodeSteps(sc, state)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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
}
