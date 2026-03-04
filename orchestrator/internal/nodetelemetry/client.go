// Package nodetelemetry provides orchestrator-side pull of worker node telemetry (node:info, node:stats)
// per REQ-ORCHES-0141, REQ-ORCHES-0142, REQ-ORCHES-0143 and worker_telemetry_api.md NodeTelemetryPull.
// Failures are non-fatal; telemetry is best-effort and non-authoritative for correctness.
package nodetelemetry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultTimeout is the per-request timeout for telemetry GETs.
const DefaultTimeout = 5 * time.Second

// Client pulls node:info and node:stats from a worker API with timeout-tolerant handling.
type Client struct {
	HTTPClient *http.Client
}

// NewClient returns a client that uses DefaultTimeout for requests.
func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: DefaultTimeout},
	}
}

// PullNodeInfo GETs /v1/worker/telemetry/node:info and returns the response body or error.
func (c *Client) PullNodeInfo(ctx context.Context, baseURL, bearer string) ([]byte, error) {
	return c.get(ctx, baseURL, "/v1/worker/telemetry/node:info", bearer)
}

// PullNodeStats GETs /v1/worker/telemetry/node:stats and returns the response body or error.
func (c *Client) PullNodeStats(ctx context.Context, baseURL, bearer string) ([]byte, error) {
	return c.get(ctx, baseURL, "/v1/worker/telemetry/node:stats", bearer)
}

func (c *Client) get(ctx context.Context, baseURL, path, bearer string) ([]byte, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	url := baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telemetry request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telemetry %s: %s", path, resp.Status)
	}
	const maxBody = 2 << 20 // 2 MiB per worker_telemetry_api.md
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("telemetry read: %w", err)
	}
	return body, nil
}
