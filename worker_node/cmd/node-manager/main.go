// Package main provides the node manager service.
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
	"runtime"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := loadConfig()
	if err := cfg.validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	nodeJWT, expiresAt, err := register(ctx, &cfg)
	if err != nil {
		logger.Error("failed to register node", "error", err)
		os.Exit(1)
	}

	logger.Info("registered with orchestrator", "node_slug", cfg.NodeSlug, "expires_at", expiresAt)

	// Report capabilities periodically.
	ticker := time.NewTicker(cfg.CapabilityReportInterval)
	defer ticker.Stop()

	for {
		if err := reportCapabilities(ctx, &cfg, nodeJWT); err != nil {
			logger.Error("capability report failed", "error", err)
		}
		<-ticker.C
	}
}

type config struct {
	OrchestratorURL          string
	NodeSlug                 string
	NodeName                 string
	RegistrationPSK          string
	CapabilityReportInterval time.Duration
	HTTPTimeout              time.Duration
}

func loadConfig() config {
	return config{
		OrchestratorURL:          getEnv("ORCHESTRATOR_URL", "http://localhost:8082"),
		NodeSlug:                 getEnv("NODE_SLUG", "node-01"),
		NodeName:                 getEnv("NODE_NAME", "Default Node"),
		RegistrationPSK:          getEnv("NODE_REGISTRATION_PSK", ""),
		CapabilityReportInterval: getDurationEnv("CAPABILITY_REPORT_INTERVAL", 60*time.Second),
		HTTPTimeout:              getDurationEnv("HTTP_TIMEOUT", 10*time.Second),
	}
}

func (c *config) validate() error {
	if c.OrchestratorURL == "" {
		return errors.New("ORCHESTRATOR_URL is required")
	}
	if c.NodeSlug == "" {
		return errors.New("NODE_SLUG is required")
	}
	if c.RegistrationPSK == "" {
		return errors.New("NODE_REGISTRATION_PSK is required")
	}
	return nil
}

func register(ctx context.Context, cfg *config) (nodeJWT, expiresAt string, err error) {
	capability := buildCapability(cfg)

	req := nodepayloads.RegistrationRequest{
		PSK:        cfg.RegistrationPSK,
		Capability: capability,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", "", fmt.Errorf("marshal registration request: %w", err)
	}

	url := cfg.OrchestratorURL + "/v1/nodes/register"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("create registration request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("send registration request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var p problem.Details
		_ = json.NewDecoder(resp.Body).Decode(&p)
		return "", "", fmt.Errorf("registration failed: %s (%d) %s", resp.Status, resp.StatusCode, p.Detail)
	}

	var bootstrap nodepayloads.BootstrapResponse
	if err := json.NewDecoder(resp.Body).Decode(&bootstrap); err != nil {
		return "", "", fmt.Errorf("decode bootstrap response: %w", err)
	}

	return bootstrap.Auth.NodeJWT, bootstrap.Auth.ExpiresAt, nil
}

func reportCapabilities(ctx context.Context, cfg *config, nodeJWT string) error {
	report := buildCapability(cfg)
	body, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal capability report: %w", err)
	}

	url := cfg.OrchestratorURL + "/v1/nodes/capability"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create capability request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+nodeJWT)

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send capability request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		var p problem.Details
		_ = json.NewDecoder(resp.Body).Decode(&p)
		return fmt.Errorf("capability report failed: %s (%d) %s", resp.Status, resp.StatusCode, p.Detail)
	}

	return nil
}

func buildCapability(cfg *config) nodepayloads.CapabilityReport {
	return nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node: nodepayloads.CapabilityNode{
			NodeSlug: cfg.NodeSlug,
			Name:     cfg.NodeName,
		},
		Platform: nodepayloads.Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		Compute: nodepayloads.Compute{
			CPUCores: runtime.NumCPU(),
			RAMMB:    4096,
		},
		Sandbox: &nodepayloads.SandboxSupport{
			Supported:      true,
			Features:       []string{"netns"},
			MaxConcurrency: 4,
		},
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDurationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
