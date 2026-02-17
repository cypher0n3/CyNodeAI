// Package main provides the Worker API service.
// See docs/tech_specs/worker_api.md.
package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cypher0n3/cynodeai/contracts/problem"
	"github.com/cypher0n3/cynodeai/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/worker_api/executor"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	bearerToken := getEnv("WORKER_API_BEARER_TOKEN", "")
	if bearerToken == "" {
		logger.Error("WORKER_API_BEARER_TOKEN must be set")
		os.Exit(1)
	}

	runtime := getEnv("CONTAINER_RUNTIME", "podman")
	defaultTimeoutSeconds := getEnvInt("DEFAULT_TIMEOUT_SECONDS", 300)
	maxOutputBytes := getEnvInt("MAX_OUTPUT_BYTES", 1<<20)

	exec := executor.New(runtime, time.Duration(defaultTimeoutSeconds)*time.Second, maxOutputBytes)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("POST /v1/worker/jobs:run", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearerToken(r, bearerToken) {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)

		var req workerapi.RunJobRequest
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "Invalid request body")
			return
		}

		if req.Version != 1 {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "Unsupported version")
			return
		}
		if req.TaskID == "" || req.JobID == "" {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "task_id and job_id are required")
			return
		}
		if len(req.Sandbox.Command) == 0 {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "sandbox.command is required")
			return
		}

		resp, err := exec.RunJob(r.Context(), &req)
		if err != nil {
			logger.Error("job execution error", "error", err)
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "Job execution failed")
			return
		}

		writeJSON(w, http.StatusOK, resp)
	})

	srv := &http.Server{
		Addr:              getEnv("LISTEN_ADDR", ":8081"),
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	logger.Info("starting worker-api", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server error", "error", err)
		os.Exit(1)
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
