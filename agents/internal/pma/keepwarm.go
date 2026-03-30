// Package pma — node-local model keep-warm (REQ-PMAGNT-0129).
package pma

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	envPMAKeepWarmIntervalSec = "PMA_KEEP_WARM_INTERVAL_SEC"
	envPMADisableKeepWarm     = "PMA_DISABLE_KEEP_WARM"
)

// DefaultKeepWarmInterval is the default seconds between keep-warm pings (tech spec: 300s).
const DefaultKeepWarmInterval = 300 * time.Second

// keepWarmPingHook is swapped in tests to avoid real HTTP.
var keepWarmPingHook = defaultKeepWarmPing

// KeepWarmIntervalFromEnv returns PMA_KEEP_WARM_INTERVAL_SEC as duration, or DefaultKeepWarmInterval.
func KeepWarmIntervalFromEnv() time.Duration {
	v := strings.TrimSpace(os.Getenv(envPMAKeepWarmIntervalSec))
	if v == "" {
		return DefaultKeepWarmInterval
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return DefaultKeepWarmInterval
	}
	return time.Duration(n) * time.Second
}

// shouldRunKeepWarm limits keep-warm to node-local inference (Ollama on loopback / UDS).
func shouldRunKeepWarm() bool {
	if strings.TrimSpace(os.Getenv(envPMADisableKeepWarm)) == "1" {
		return false
	}
	baseURL, _ := resolveOllamaConfig()
	u := strings.ToLower(strings.TrimSpace(baseURL))
	if u == "" {
		return false
	}
	if strings.HasPrefix(u, "http+unix:") {
		return true
	}
	if strings.Contains(u, "localhost") || strings.Contains(u, "127.0.0.1") {
		return true
	}
	return false
}

// StartKeepWarm runs until ctx is cancelled: initial load ping plus periodic minimal /api/chat calls.
func StartKeepWarm(ctx context.Context, logger *slog.Logger) {
	if !shouldRunKeepWarm() {
		if logger != nil {
			logger.Debug("keep-warm disabled or not node-local inference backend")
		}
		return
	}
	interval := KeepWarmIntervalFromEnv()
	go runKeepWarmLoop(ctx, interval, logger, keepWarmPingHook)
}

// runKeepWarmLoop issues an immediate ping then ticks until ctx.Done().
func runKeepWarmLoop(ctx context.Context, interval time.Duration, logger *slog.Logger, ping func(context.Context) error) {
	if interval <= 0 {
		return
	}
	if err := ping(ctx); err != nil && logger != nil {
		logger.Debug("keep-warm initial ping", "error", err)
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := ping(ctx); err != nil && logger != nil {
				logger.Debug("keep-warm ping", "error", err)
			}
		}
	}
}

func defaultKeepWarmPing(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	baseURL, model := resolveOllamaConfig()
	inferenceURL, inferenceClient := resolveInferenceClient(baseURL, inferenceHTTPTimeout)
	chatMessages := []map[string]string{
		{"role": "user", "content": "."},
	}
	resp, err := doInferenceRequest(ctx, inferenceClient, inferenceURL, model, chatMessages, false)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("inference returned %s", resp.Status)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// LoadModelOnStart sends one minimal non-streaming request so the effective model is resident (REQ-PMAGNT-0129 load).
// Exported for tests; production uses StartKeepWarm's initial ping.
func LoadModelOnStart(ctx context.Context) error {
	return defaultKeepWarmPing(ctx)
}
