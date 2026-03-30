// Package dispatcher: RunOnce runs a single dispatch iteration (get next job, call worker, complete job).
// Used by the control-plane loop and by BDD tests to trigger dispatch without a background ticker.
package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// RunOnce performs one dispatch iteration: get next queued job, pick a dispatchable node, set job to running, call Worker API, complete job.
// Returns nil on success, database.ErrNotFound when no queued job, or another error.
func RunOnce(ctx context.Context, db database.Store, client *http.Client, httpTimeout time.Duration, logger *slog.Logger) error {
	if client == nil {
		client = &http.Client{Timeout: httpTimeout}
	}
	job, err := db.GetNextQueuedJob(ctx)
	if err != nil {
		return err
	}
	node, workerURL, workerToken, err := pickNodeAndCredentials(ctx, db)
	if err != nil {
		return err
	}
	if err := db.AssignJobToNode(ctx, job.ID, node.ID); err != nil {
		return fmt.Errorf("assign job to node: %w", err)
	}
	_ = db.UpdateJobStatus(ctx, job.ID, models.JobStatusRunning)
	_ = db.UpdateTaskStatus(ctx, job.TaskID, models.TaskStatusRunning)
	sandbox, err := ParseSandboxSpec(job.Payload.Ptr())
	if err != nil {
		markJobAndTaskFailed(ctx, db, job, MarshalDispatchError(err))
		return nil
	}
	runReq := workerapi.RunJobRequest{
		Version: 1,
		TaskID:  job.TaskID.String(),
		JobID:   job.ID.String(),
		Sandbox: sandbox,
	}
	result, err := callWorkerAPI(ctx, client, workerURL, workerToken, &runReq)
	if err != nil {
		markJobAndTaskFailed(ctx, db, job, MarshalDispatchError(err))
		return nil
	}
	if err := applyJobResult(ctx, db, job, result); err != nil {
		return err
	}
	if logger != nil {
		logger.Info("job dispatched", "job_id", job.ID, "task_id", job.TaskID, "node_slug", node.NodeSlug, "result_status", result.Status)
	}
	return nil
}

func pickNodeAndCredentials(ctx context.Context, db database.Store) (node *models.Node, workerURL, workerToken string, err error) {
	nodes, err := db.ListDispatchableNodes(ctx)
	if err != nil {
		return nil, "", "", fmt.Errorf("list dispatchable nodes: %w", err)
	}
	if len(nodes) == 0 {
		return nil, "", "", fmt.Errorf("no dispatchable nodes (active with config ack and worker API URL/token)")
	}
	node = nodes[0]
	if node.WorkerAPITargetURL != nil {
		workerURL = *node.WorkerAPITargetURL
	}
	if node.WorkerAPIBearerToken != nil {
		workerToken = *node.WorkerAPIBearerToken
	}
	if workerURL == "" || workerToken == "" {
		return nil, "", "", fmt.Errorf("node %s has no worker API URL or token", node.NodeSlug)
	}
	return node, workerURL, workerToken, nil
}

func markJobAndTaskFailed(ctx context.Context, db database.Store, job *models.Job, result string) {
	_ = db.CompleteJob(ctx, job.ID, result, models.JobStatusFailed)
	_ = db.UpdateTaskStatus(ctx, job.TaskID, models.TaskStatusFailed)
}

func applyJobResult(ctx context.Context, db database.Store, job *models.Job, result *workerapi.RunJobResponse) error {
	normalizeSBAResultSurface(result)
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
	return nil
}

func normalizeSBAResultSurface(result *workerapi.RunJobResponse) {
	if result == nil || result.SbaResult == nil {
		return
	}
	if strings.TrimSpace(result.Stdout) != "" {
		return
	}
	answer := strings.TrimSpace(result.SbaResult.FinalAnswer)
	if answer != "" {
		result.Stdout = answer
	}
}

func callWorkerAPI(ctx context.Context, client *http.Client, workerBaseURL, bearerToken string, req *workerapi.RunJobRequest) (*workerapi.RunJobResponse, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		runResp, err := callWorkerAPIOnce(ctx, client, workerBaseURL, bearerToken, req)
		if err == nil {
			return runResp, nil
		}
		lastErr = err
		if !isTransientWorkerDispatchError(err) || attempt == 2 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func callWorkerAPIOnce(ctx context.Context, client *http.Client, workerBaseURL, bearerToken string, req *workerapi.RunJobRequest) (*workerapi.RunJobResponse, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimSuffix(workerBaseURL, "/")
	url := baseURL + "/v1/worker/jobs:run"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+bearerToken)
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("worker api returned %s", resp.Status)
	}
	var runResp workerapi.RunJobResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, httplimits.DefaultMaxHTTPResponseBytes)).Decode(&runResp); err != nil {
		return nil, err
	}
	if runResp.Version != 1 {
		return nil, fmt.Errorf("unsupported worker response version: %d", runResp.Version)
	}
	return &runResp, nil
}

func isTransientWorkerDispatchError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return errorsIsEOF(err) ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "timeout")
}

func errorsIsEOF(err error) bool {
	return err == io.EOF || strings.Contains(strings.ToLower(err.Error()), "eof")
}
