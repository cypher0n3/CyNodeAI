// Package nodemanager provides node registration and capability reporting.
package nodemanager

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

// Config holds node manager configuration from the environment.
type Config struct {
	OrchestratorURL          string
	NodeSlug                 string
	NodeName                 string
	RegistrationPSK          string
	CapabilityReportInterval time.Duration
	HTTPTimeout              time.Duration
}

// LoadConfig reads configuration from the environment.
func LoadConfig() Config {
	return Config{
		OrchestratorURL:          getEnv("ORCHESTRATOR_URL", "http://localhost:8082"),
		NodeSlug:                 getEnv("NODE_SLUG", "node-01"),
		NodeName:                 getEnv("NODE_NAME", "Default Node"),
		RegistrationPSK:          getEnv("NODE_REGISTRATION_PSK", ""),
		CapabilityReportInterval: getDurationEnv("CAPABILITY_REPORT_INTERVAL", 60*time.Second),
		HTTPTimeout:              getDurationEnv("HTTP_TIMEOUT", 10*time.Second),
	}
}

// Validate returns an error if the config is invalid.
func (c *Config) Validate() error {
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

// BootstrapData holds parsed bootstrap payload data for follow-on calls.
type BootstrapData struct {
	NodeJWT       string
	ExpiresAt     string
	NodeReportURL string
	NodeConfigURL string
}

// RunOptions allows optional service starters for production; nil means skip (e.g. in tests).
// StartWorkerAPI receives the bearer token from config; callers must not log it.
// StartOllama is Phase 1 inference; if it returns an error, Run fails (fail-fast).
type RunOptions struct {
	StartWorkerAPI func(bearerToken string) error
	StartOllama     func() error
}

// Run performs registration, config fetch, service startup, config ack, then capability reporting until ctx is cancelled.
// Order: register => fetch config => start worker API (if token present) => start Ollama (if set) => config ack => capability loop.
func Run(ctx context.Context, logger *slog.Logger, cfg *Config) error {
	return RunWithOptions(ctx, logger, cfg, nil)
}

// RunWithOptions is like Run but accepts optional StartWorkerAPI and StartOllama; nil means skip that step.
func RunWithOptions(ctx context.Context, logger *slog.Logger, cfg *Config, opts *RunOptions) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	bootstrap, err := register(ctx, cfg)
	if err != nil {
		return err
	}

	if logger != nil {
		logger.Info("registered with orchestrator", "node_slug", cfg.NodeSlug, "expires_at", bootstrap.ExpiresAt)
	}

	nodeConfig, err := FetchConfig(ctx, cfg, bootstrap)
	if err != nil {
		return fmt.Errorf("fetch config: %w", err)
	}

	if err := applyConfigAndStartServices(ctx, logger, cfg, bootstrap, nodeConfig, opts); err != nil {
		return err
	}

	return runCapabilityLoop(ctx, cfg, bootstrap)
}

// applyConfigAndStartServices starts Worker API and Ollama from opts (if set), then sends config ack.
func applyConfigAndStartServices(ctx context.Context, logger *slog.Logger, cfg *Config, bootstrap *BootstrapData, nodeConfig *nodepayloads.NodeConfigurationPayload, opts *RunOptions) error {
	if opts != nil && opts.StartWorkerAPI != nil && nodeConfig != nil && nodeConfig.WorkerAPI != nil && nodeConfig.WorkerAPI.OrchestratorBearerToken != "" {
		if err := opts.StartWorkerAPI(nodeConfig.WorkerAPI.OrchestratorBearerToken); err != nil {
			return fmt.Errorf("start worker API: %w", err)
		}
		if logger != nil {
			logger.Info("worker API started")
		}
	}
	if opts != nil && opts.StartOllama != nil {
		if err := opts.StartOllama(); err != nil {
			return fmt.Errorf("start inference (Ollama): %w", err)
		}
		if logger != nil {
			logger.Info("inference container started")
		}
	}
	if err := SendConfigAck(ctx, cfg, bootstrap, nodeConfig, "applied"); err != nil {
		return fmt.Errorf("config ack: %w", err)
	}
	if logger != nil {
		logger.Info("config applied and acknowledged", "config_version", nodeConfig.ConfigVersion)
	}
	return nil
}

func runCapabilityLoop(ctx context.Context, cfg *Config, bootstrap *BootstrapData) error {
	ticker := time.NewTicker(cfg.CapabilityReportInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := reportCapabilities(ctx, cfg, bootstrap); err != nil {
				_ = err
			}
		}
	}
}

// FetchConfig fetches the node configuration from the bootstrap node_config_url (GET).
func FetchConfig(ctx context.Context, cfg *Config, bootstrap *BootstrapData) (*nodepayloads.NodeConfigurationPayload, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", bootstrap.NodeConfigURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create config request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bootstrap.NodeJWT)

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var p problem.Details
		_ = json.NewDecoder(resp.Body).Decode(&p)
		return nil, fmt.Errorf("get config: %s (%d) %s", resp.Status, resp.StatusCode, p.Detail)
	}

	var payload nodepayloads.NodeConfigurationPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if payload.Version != 1 {
		return nil, fmt.Errorf("unsupported config version %d", payload.Version)
	}
	return &payload, nil
}

// SendConfigAck sends node_config_ack_v1 to the bootstrap node_config_url (POST).
func SendConfigAck(ctx context.Context, cfg *Config, bootstrap *BootstrapData, nodeConfig *nodepayloads.NodeConfigurationPayload, status string) error {
	if nodeConfig == nil {
		return errors.New("node config is nil")
	}
	ack := nodepayloads.ConfigAck{
		Version:       1,
		NodeSlug:      nodeConfig.NodeSlug,
		ConfigVersion: nodeConfig.ConfigVersion,
		AckAt:         time.Now().UTC().Format(time.RFC3339),
		Status:        status,
	}
	body, err := json.Marshal(ack)
	if err != nil {
		return fmt.Errorf("marshal config ack: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", bootstrap.NodeConfigURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create config ack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bootstrap.NodeJWT)

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send config ack: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		var p problem.Details
		_ = json.NewDecoder(resp.Body).Decode(&p)
		return fmt.Errorf("config ack: %s (%d) %s", resp.Status, resp.StatusCode, p.Detail)
	}
	return nil
}

func register(ctx context.Context, cfg *Config) (*BootstrapData, error) {
	capability := buildCapability(cfg)

	req := nodepayloads.RegistrationRequest{
		PSK:        cfg.RegistrationPSK,
		Capability: capability,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal registration request: %w", err)
	}

	url := cfg.OrchestratorURL + "/v1/nodes/register"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create registration request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send registration request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var p problem.Details
		_ = json.NewDecoder(resp.Body).Decode(&p)
		return nil, fmt.Errorf("registration failed: %s (%d) %s", resp.Status, resp.StatusCode, p.Detail)
	}

	var bootstrap nodepayloads.BootstrapResponse
	if err := json.NewDecoder(resp.Body).Decode(&bootstrap); err != nil {
		return nil, fmt.Errorf("decode bootstrap response: %w", err)
	}

	if err := ValidateBootstrap(&bootstrap); err != nil {
		return nil, fmt.Errorf("invalid bootstrap payload: %w", err)
	}

	return &BootstrapData{
		NodeJWT:       bootstrap.Auth.NodeJWT,
		ExpiresAt:     bootstrap.Auth.ExpiresAt,
		NodeReportURL: bootstrap.Orchestrator.Endpoints.NodeReportURL,
		NodeConfigURL: bootstrap.Orchestrator.Endpoints.NodeConfigURL,
	}, nil
}

// ValidateBootstrap checks that the bootstrap payload has required fields.
func ValidateBootstrap(b *nodepayloads.BootstrapResponse) error {
	if b.Version != 1 {
		return fmt.Errorf("unsupported bootstrap version %d", b.Version)
	}
	if b.Auth.NodeJWT == "" {
		return errors.New("missing auth.node_jwt")
	}
	if b.Orchestrator.Endpoints.NodeReportURL == "" {
		return errors.New("missing orchestrator.endpoints.node_report_url")
	}
	if b.Orchestrator.Endpoints.NodeConfigURL == "" {
		return errors.New("missing orchestrator.endpoints.node_config_url")
	}
	return nil
}

func reportCapabilities(ctx context.Context, cfg *Config, bootstrap *BootstrapData) error {
	report := buildCapability(cfg)
	body, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal capability report: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", bootstrap.NodeReportURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create capability request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bootstrap.NodeJWT)

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

func buildCapability(cfg *Config) nodepayloads.CapabilityReport {
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
