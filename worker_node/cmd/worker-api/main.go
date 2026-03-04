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
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/cmd/worker-api/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
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
		getEnvInt("MAX_OUTPUT_BYTES", 262144), // 256 KiB default per worker_api.md
		getEnv("OLLAMA_UPSTREAM_URL", ""),
		getEnv("INFERENCE_PROXY_IMAGE", ""),
		nil,
	)
	workspaceRoot := getEnv("WORKSPACE_ROOT", filepath.Join(os.TempDir(), "cynodeai-workspaces"))
	stateDir := getEnv("WORKER_API_STATE_DIR", filepath.Join(os.TempDir(), "cynode", "state"))
	var telemetryStore *telemetry.Store
	if ts, err := telemetry.Open(ctx, stateDir); err != nil {
		logger.Warn("telemetry store unavailable, containers/logs endpoints disabled", "error", err)
	} else {
		telemetryStore = ts
		defer func() { _ = telemetryStore.Close() }()
		go runRetentionAndVacuum(ctx, telemetryStore, logger)
	}
	mux := newMux(exec, bearerToken, workspaceRoot, telemetryStore, logger)
	srv := newServer(mux)

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	select {
	case <-ctx.Done():
	case <-done:
	case <-serverErr:
	}
	// Shutdown with a timeout; derive from ctx so contextcheck passes, but use WithoutCancel so we get a grace period even when ctx is already cancelled.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return 1
	}
	return 0
}

// doRetentionAndVacuumOnce runs retention and vacuum once; used by runRetentionAndVacuum and tests.
func doRetentionAndVacuumOnce(ctx context.Context, store *telemetry.Store, logger *slog.Logger) {
	if err := store.EnforceRetention(ctx); err != nil {
		logger.Warn("telemetry retention", "error", err)
	}
	if err := store.Vacuum(ctx); err != nil {
		logger.Warn("telemetry vacuum", "error", err)
	}
}

// retentionTickerInterval and vacuumTickerInterval are used by runRetentionAndVacuum; tests may override for coverage.
var retentionTickerInterval = time.Hour
var vacuumTickerInterval = 24 * time.Hour

func runRetentionAndVacuum(ctx context.Context, store *telemetry.Store, logger *slog.Logger) {
	doRetentionAndVacuumOnce(ctx, store, logger)
	retentionTicker := time.NewTicker(retentionTickerInterval)
	defer retentionTicker.Stop()
	vacuumTicker := time.NewTicker(vacuumTickerInterval)
	defer vacuumTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-retentionTicker.C:
			if err := store.EnforceRetention(ctx); err != nil {
				logger.Warn("telemetry retention", "error", err)
			}
		case <-vacuumTicker.C:
			if err := store.Vacuum(ctx); err != nil {
				logger.Warn("telemetry vacuum", "error", err)
			}
		}
	}
}

func newMux(exec *executor.Executor, bearerToken, workspaceRoot string, telemetryStore *telemetry.Store, logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	// REQ-WORKER-0140, REQ-WORKER-0141: unauthenticated GET /healthz; body plain text "ok" per worker_api.md
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// REQ-WORKER-0140, REQ-WORKER-0142: unauthenticated GET /readyz
	mux.HandleFunc("GET /readyz", readyzHandler(exec))
	mux.HandleFunc("POST /v1/worker/jobs:run", handleRunJob(exec, bearerToken, workspaceRoot, logger))
	// REQ-WORKER-0200--0243: Worker Telemetry API.
	mux.HandleFunc("GET /v1/worker/telemetry/node:info", telemetryAuth(bearerToken, handleNodeInfo(logger)))
	mux.HandleFunc("GET /v1/worker/telemetry/node:stats", telemetryAuth(bearerToken, handleNodeStats(logger)))
	if telemetryStore != nil {
		mux.HandleFunc("GET /v1/worker/telemetry/containers", telemetryAuth(bearerToken, handleListContainers(telemetryStore)))
		mux.HandleFunc("GET /v1/worker/telemetry/containers/", telemetryAuth(bearerToken, handleGetContainer(telemetryStore)))
		mux.HandleFunc("GET /v1/worker/telemetry/logs", telemetryAuth(bearerToken, handleQueryLogs(telemetryStore)))
	}
	return mux
}

func telemetryAuth(bearerToken string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireBearerToken(r, bearerToken) {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		next(w, r)
	}
}

func handleNodeInfo(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeSlug := getEnv("NODE_SLUG", "default")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version": 1,
			"node_slug": nodeSlug,
			"build": map[string]string{"build_version": "dev", "git_sha": ""},
			"platform": map[string]string{"os": "linux", "arch": "amd64", "kernel_version": ""},
		})
	}
}

func handleNodeStats(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version":    1,
			"captured_at": time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
			"cpu":        map[string]interface{}{"cores": 0, "load1": 0.0, "load5": 0.0, "load15": 0.0},
			"memory":     map[string]interface{}{"total_mb": 0, "used_mb": 0, "free_mb": 0},
			"disk":       map[string]interface{}{"state_dir_free_mb": 0, "state_dir_total_mb": 0},
			"container_runtime": map[string]string{"runtime": getEnv("CONTAINER_RUNTIME", "podman"), "version": ""},
		})
	}
}

// readyzHandler implements REQ-WORKER-0142: 200 "ready" when ready to accept jobs, 503 otherwise.
func readyzHandler(exec *executor.Executor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	}
}

// decodeRunJobRequest decodes POST body; enforces maxBytes and returns 413 on overflow (REQ-WORKER-0145).
func decodeRunJobRequest(w http.ResponseWriter, r *http.Request, maxBytes int64) (*workerapi.RunJobRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	var req workerapi.RunJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err != nil && strings.Contains(err.Error(), "request body too large") {
			writeProblem(w, http.StatusRequestEntityTooLarge, problem.TypeValidation, "Request Entity Too Large", "Request body exceeds maximum size")
			return nil, false
		}
		writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "Invalid request body")
		return nil, false
	}
	return &req, true
}

func handleRunJob(exec *executor.Executor, bearerToken, workspaceRoot string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireBearerToken(r, bearerToken) {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		req, ok := decodeRunJobRequest(w, r, 10*1024*1024) // 10 MiB per worker_api.md (REQ-WORKER-0145)
		if !ok {
			return
		}
		if err := validateRunJobRequest(req); err != nil {
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
		resp, err := exec.RunJob(r.Context(), req, workspaceDir)
		if err != nil {
			logger.Error("job execution error", "error", err)
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "Job execution failed")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func validateRunJobRequest(req *workerapi.RunJobRequest) error {
	if err := workerapi.ValidateRequest(req); err != nil {
		return err
	}
	if req.Version != 1 {
		return fmt.Errorf("unsupported version")
	}
	if req.TaskID == "" || req.JobID == "" {
		return fmt.Errorf("task_id and job_id are required")
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
		Addr:              getEnv("LISTEN_ADDR", ":9190"),
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

func handleListContainers(store *telemetry.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeProblem(w, http.StatusMethodNotAllowed, problem.TypeValidation, "Method Not Allowed", "")
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
	}
}

func handleGetContainer(store *telemetry.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeProblem(w, http.StatusMethodNotAllowed, problem.TypeValidation, "Method Not Allowed", "")
			return
		}
		containerID := strings.TrimPrefix(r.URL.Path, "/v1/worker/telemetry/containers/")
		if containerID == "" {
			writeProblem(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", "container_id required")
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
	}
}

func parseLogsLimit(limitParam string) int {
	const defaultLimit, maxLimit = 1000, 5000
	if limitParam == "" {
		return defaultLimit
	}
	n, err := strconv.Atoi(limitParam)
	if err != nil || n <= 0 || n > maxLimit {
		return defaultLimit
	}
	return n
}

func validateLogsQuery(sourceKind, containerID string) string {
	if sourceKind != "" || containerID != "" {
		return ""
	}
	return "source_kind+source_name or source_kind=container+container_id required"
}

func handleQueryLogs(store *telemetry.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeProblem(w, http.StatusMethodNotAllowed, problem.TypeValidation, "Method Not Allowed", "")
			return
		}
		q := r.URL.Query()
		if msg := validateLogsQuery(q.Get("source_kind"), q.Get("container_id")); msg != "" {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", msg)
			return
		}
		limit := parseLogsLimit(q.Get("limit"))
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
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
