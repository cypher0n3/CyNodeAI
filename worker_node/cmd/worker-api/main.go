// Package main provides the Worker API service.
// See docs/tech_specs/worker_api.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/cmd/worker-api/executor"
)

func main() {
	os.Exit(runMain(context.Background()))
}

// runMain builds and runs the server until ctx is cancelled.
// Returns 0 on success, 1 on failure. Extracted for testability.
func runMain(ctx context.Context) int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	bearerToken := getEnv("WORKER_API_BEARER_TOKEN", "")
	if bearerToken == "" {
		logger.Error("WORKER_API_BEARER_TOKEN must be set")
		return 1
	}

	exec := executor.New(
		getEnv("CONTAINER_RUNTIME", "podman"),
		time.Duration(getEnvInt("DEFAULT_TIMEOUT_SECONDS", 300))*time.Second,
		getEnvInt("MAX_OUTPUT_BYTES", 1<<20),
		getEnv("OLLAMA_UPSTREAM_URL", ""),
		getEnv("INFERENCE_PROXY_IMAGE", ""),
		nil,
	)
	workspaceRoot := getEnv("WORKSPACE_ROOT", filepath.Join(os.TempDir(), "cynodeai-workspaces"))
	mux := newMux(exec, bearerToken, workspaceRoot, logger)
	srv := newServer(mux)

	go func() {
		_ = srv.ListenAndServe()
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return 1
	}
	return 0
}

func newMux(exec *executor.Executor, bearerToken, workspaceRoot string, logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /v1/worker/jobs:run", handleRunJob(exec, bearerToken, workspaceRoot, logger))
	return mux
}

func handleRunJob(exec *executor.Executor, bearerToken, workspaceRoot string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireBearerToken(r, bearerToken) {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)
		var req workerapi.RunJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "Invalid request body")
			return
		}
		if err := validateRunJobRequest(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", err.Error())
			return
		}
		workspaceDir, cleanup, err := prepareWorkspace(workspaceRoot, req.JobID)
		if err != nil {
			logger.Error("workspace creation failed", "error", err, "job_id", req.JobID)
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "Workspace creation failed")
			return
		}
		if cleanup != nil {
			defer cleanup()
		}
		resp, err := exec.RunJob(r.Context(), &req, workspaceDir)
		if err != nil {
			logger.Error("job execution error", "error", err)
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "Job execution failed")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func validateRunJobRequest(req *workerapi.RunJobRequest) error {
	if req.Version != 1 {
		return fmt.Errorf("unsupported version")
	}
	if req.TaskID == "" || req.JobID == "" {
		return fmt.Errorf("task_id and job_id are required")
	}
	if len(req.Sandbox.Command) == 0 {
		return fmt.Errorf("sandbox.command is required")
	}
	return nil
}

// prepareWorkspace creates a per-job workspace dir under workspaceRoot.
// Returns (dir, cleanup, nil) on success; ("", nil, nil) when workspaceRoot is empty; ("", nil, err) on failure.
func prepareWorkspace(workspaceRoot, jobID string) (dir string, cleanup func(), err error) {
	if workspaceRoot == "" {
		return "", nil, nil
	}
	safeID := strings.ReplaceAll(jobID, string(filepath.Separator), "_")
	workspaceDir := filepath.Join(workspaceRoot, safeID)
	if err := os.MkdirAll(workspaceDir, 0o700); err != nil {
		return "", nil, errors.Join(fmt.Errorf("mkdir %s", workspaceDir), err)
	}
	return workspaceDir, func() { _ = os.RemoveAll(workspaceDir) }, nil
}

func newServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              getEnv("LISTEN_ADDR", ":8081"),
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func requireBearerToken(r *http.Request, expected string) bool {
	authz := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(authz) <= len(prefix) || authz[:len(prefix)] != prefix {
		return false
	}
	return authz[len(prefix):] == expected
}

func writeProblem(w http.ResponseWriter, status int, typ, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem.Details{
		Type:   typ,
		Title:  title,
		Status: status,
		Detail: detail,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
