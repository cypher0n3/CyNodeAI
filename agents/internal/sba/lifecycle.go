// Package sba provides job lifecycle reporting (in-progress and completion) via optional callback URL.
package sba

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
)

const (
	envStatusURL     = "SBA_JOB_STATUS_URL"
	envCallbackURL   = "SBA_CALLBACK_URL"
	lifecycleTimeout = 10 * time.Second
)

// LifecycleClient POSTs in_progress and completion to an optional status/callback URL.
type LifecycleClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewLifecycleClient reads SBA_JOB_STATUS_URL or SBA_CALLBACK_URL. Empty if unset.
func NewLifecycleClient() *LifecycleClient {
	url := os.Getenv(envStatusURL)
	if url == "" {
		url = os.Getenv(envCallbackURL)
	}
	return &LifecycleClient{
		BaseURL: url,
		HTTPClient: &http.Client{
			Timeout: lifecycleTimeout,
		},
	}
}

// NotifyInProgress POSTs in_progress (best-effort). No-op if BaseURL empty.
func (c *LifecycleClient) NotifyInProgress(ctx context.Context) {
	if c.BaseURL == "" {
		return
	}
	body := map[string]string{"status": "in_progress"}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(raw))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// NotifyCompletion POSTs completion with the result contract (best-effort). No-op if BaseURL empty.
func (c *LifecycleClient) NotifyCompletion(ctx context.Context, result *sbajob.Result) {
	if c.BaseURL == "" || result == nil {
		return
	}
	body := map[string]interface{}{"status": "completed", "result": result}
	raw, err := json.Marshal(body)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(raw))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
