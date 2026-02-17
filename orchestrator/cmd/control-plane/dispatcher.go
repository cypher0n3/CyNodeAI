package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

type dispatcherConfig struct {
	Enabled      bool
	PollInterval time.Duration
	WorkerAPIURL string
	BearerToken  string
	HTTPTimeout  time.Duration
}

func loadDispatcherConfig() dispatcherConfig {
	return dispatcherConfig{
		Enabled:      getEnv("DISPATCHER_ENABLED", "true") == "true",
		PollInterval: getDurationEnv("DISPATCH_POLL_INTERVAL", 1*time.Second),
		WorkerAPIURL: getEnv("WORKER_API_URL", "http://localhost:8081"),
		BearerToken:  getEnv("WORKER_API_BEARER_TOKEN", ""),
		HTTPTimeout:  getDurationEnv("DISPATCH_HTTP_TIMEOUT", 5*time.Minute),
	}
}

func startDispatcher(ctx context.Context, db database.Store, logger *slog.Logger) {
	cfg := loadDispatcherConfig()
	if !cfg.Enabled {
		logger.Info("dispatcher disabled")
		return
	}
	if cfg.BearerToken == "" {
		logger.Warn("dispatcher enabled but WORKER_API_BEARER_TOKEN is empty; dispatcher will not run")
		return
	}

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	logger.Info("dispatcher started", "poll_interval", cfg.PollInterval.String(), "worker_api_url", cfg.WorkerAPIURL)

	for {
		select {
		case <-ctx.Done():
			logger.Info("dispatcher stopping", "reason", ctx.Err())
			return
		case <-ticker.C:
			if err := dispatchOnce(ctx, db, client, cfg, logger); err != nil {
				if errors.Is(err, database.ErrNotFound) {
					continue
				}
				logger.Error("dispatch iteration failed", "error", err)
			}
		}
	}
}

func dispatchOnce(ctx context.Context, db database.Store, client *http.Client, cfg dispatcherConfig, logger *slog.Logger) error {
	job, err := db.GetNextQueuedJob(ctx)
	if err != nil {
		return err
	}

	nodes, err := db.ListActiveNodes(ctx)
	if err != nil {
		return fmt.Errorf("list active nodes: %w", err)
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no active nodes available")
	}
	node := nodes[0]

	if err := db.AssignJobToNode(ctx, job.ID, node.ID); err != nil {
		return fmt.Errorf("assign job to node: %w", err)
	}
	_ = db.UpdateTaskStatus(ctx, job.TaskID, models.TaskStatusRunning)

	sandbox, err := parseSandboxSpec(job.Payload.Ptr())
	if err != nil {
		_ = db.CompleteJob(ctx, job.ID, marshalDispatchError(err), models.JobStatusFailed)
		_ = db.UpdateTaskStatus(ctx, job.TaskID, models.TaskStatusFailed)
		return nil
	}

	runReq := workerapi.RunJobRequest{
		Version: 1,
		TaskID:  job.TaskID.String(),
		JobID:   job.ID.String(),
		Sandbox: sandbox,
	}

	result, err := callWorkerAPI(ctx, client, cfg, &runReq)
	if err != nil {
		_ = db.CompleteJob(ctx, job.ID, marshalDispatchError(err), models.JobStatusFailed)
		_ = db.UpdateTaskStatus(ctx, job.TaskID, models.TaskStatusFailed)
		return nil
	}

	resultJSON, _ := json.Marshal(result)

	jobStatus := models.JobStatusCompleted
	taskStatus := models.TaskStatusCompleted
	if result.Status != workerapi.StatusCompleted {
		jobStatus = models.JobStatusFailed
		taskStatus = models.TaskStatusFailed
	}

	if err := db.CompleteJob(ctx, job.ID, string(resultJSON), jobStatus); err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	_ = db.UpdateTaskStatus(ctx, job.TaskID, taskStatus)

	if taskStatus == models.TaskStatusCompleted {
		summary := summarizeResult(result)
		_ = db.UpdateTaskSummary(ctx, job.TaskID, summary)
	}

	logger.Info("job dispatched",
		"job_id", job.ID,
		"task_id", job.TaskID,
		"node_id", node.ID,
		"node_slug", node.NodeSlug,
		"result_status", result.Status,
	)

	return nil
}

func parseSandboxSpec(payload *string) (workerapi.SandboxSpec, error) {
	if payload == nil || *payload == "" {
		return workerapi.SandboxSpec{}, errors.New("job payload is empty")
	}

	var spec struct {
		Image          string            `json:"image"`
		Command        []string          `json:"command"`
		Env            map[string]string `json:"env"`
		TimeoutSeconds int               `json:"timeout_seconds"`
		NetworkPolicy  string            `json:"network_policy"`
	}
	if err := json.Unmarshal([]byte(*payload), &spec); err != nil {
		return workerapi.SandboxSpec{}, fmt.Errorf("parse payload json: %w", err)
	}
	if len(spec.Command) == 0 {
		return workerapi.SandboxSpec{}, errors.New("payload.command is required")
	}

	return workerapi.SandboxSpec{
		Image:          spec.Image,
		Command:        spec.Command,
		Env:            spec.Env,
		TimeoutSeconds: spec.TimeoutSeconds,
		NetworkPolicy:  spec.NetworkPolicy,
	}, nil
}

func callWorkerAPI(ctx context.Context, client *http.Client, cfg dispatcherConfig, req *workerapi.RunJobRequest) (*workerapi.RunJobResponse, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := cfg.WorkerAPIURL + "/v1/worker/jobs:run"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.BearerToken)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("worker api returned %s", resp.Status)
	}

	var runResp workerapi.RunJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&runResp); err != nil {
		return nil, err
	}

	if runResp.Version != 1 {
		return nil, fmt.Errorf("unsupported worker response version: %d", runResp.Version)
	}

	return &runResp, nil
}

func summarizeResult(resp *workerapi.RunJobResponse) string {
	if resp == nil {
		return ""
	}
	if resp.Status != workerapi.StatusCompleted {
		return fmt.Sprintf("job %s", resp.Status)
	}
	if resp.Stdout != "" {
		return truncateOneLine(resp.Stdout, 200)
	}
	if resp.Stderr != "" {
		return truncateOneLine(resp.Stderr, 200)
	}
	return "completed"
}

func truncateOneLine(s string, maxLen int) string {
	line := s
	for i := 0; i < len(line); i++ {
		if line[i] == '\n' {
			line = line[:i]
			break
		}
	}
	if len(line) > maxLen {
		return line[:maxLen]
	}
	return line
}

func marshalDispatchError(err error) string {
	obj := map[string]any{
		"version": 1,
		"status":  "failed",
		"error":   err.Error(),
	}
	b, _ := json.Marshal(obj)
	return string(b)
}

func getDurationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
