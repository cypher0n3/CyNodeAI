// Package nodeagent provides node registration and capability reporting.
// It is used by the node-manager command (cmd/node-manager).
package nodeagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
)

const (
	agentTokenRefKindOrchestratorEndpoint = "orchestrator_endpoint"
	serviceTypePMA                        = "pma"
	proxyURLAuto                          = "auto"
)

// Config holds node manager configuration from the environment.
type Config struct {
	OrchestratorURL          string
	NodeSlug                 string
	NodeName                 string
	RegistrationPSK          string
	AdvertisedWorkerAPIURL   string // base_url sent at registration and in capability reports (required for dispatch)
	CapabilityReportInterval time.Duration
	HTTPTimeout              time.Duration
}

// LoadConfig reads configuration from the environment.
func LoadConfig() Config {
	return Config{
		OrchestratorURL:          getEnv("ORCHESTRATOR_URL", "http://localhost:12082"),
		NodeSlug:                 getEnv("NODE_SLUG", "node-01"),
		NodeName:                 getEnv("NODE_NAME", "Default Node"),
		RegistrationPSK:          getEnv("NODE_REGISTRATION_PSK", ""),
		AdvertisedWorkerAPIURL:   getEnv("NODE_ADVERTISED_WORKER_API_URL", ""),
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
// StartOllama is Phase 1 inference; image/variant/env come from config or env; if it returns an error, Run fails (fail-fast).
// env carries orchestrator-derived container environment variables (e.g. OLLAMA_NUM_CTX).
// StartManagedServices starts orchestrator-directed managed service containers (e.g. PMA) from desired state; if it returns an error, Run fails.
type RunOptions struct {
	StartWorkerAPI       func(bearerToken string) error
	StartOllama          func(image, variant string, env map[string]string) error
	StartManagedServices func(ctx context.Context, services []nodepayloads.ConfigManagedService) error
	// PullModels is called in the background after the inference backend starts.
	// It receives the ordered list of desired model names and is expected to pull
	// any that are not already available. May be nil (pull skipped).
	PullModels func(ctx context.Context, models []string) error
}

// Run performs registration, config fetch, service startup, config ack, then capability reporting until ctx is canceled.
// Order (per worker_node.md Node Startup Procedure): wait for orchestrator readiness => run startup checks
// (container runtime, existing inference detection) => register => fetch config => start worker API => start Ollama (if instructed) => config ack => capability loop.
func Run(ctx context.Context, logger *slog.Logger, cfg *Config) error {
	return RunWithOptions(ctx, logger, cfg, nil)
}

// RunWithOptions is like Run but accepts optional StartWorkerAPI and StartOllama; nil means skip that step.
// The node is responsible for waiting for the orchestrator control-plane to be reachable before
// registering (see worker_node.md / startup procedure); the script does not poll for control-plane readiness.
// Per Node Startup Procedure and Node Startup Checks (worker_node.md): the node performs initial
// startup checks (container runtime, existing inference detection) before registering with the orchestrator.
func RunWithOptions(ctx context.Context, logger *slog.Logger, cfg *Config, opts *RunOptions) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := waitForOrchestratorReadiness(ctx, logger, cfg); err != nil {
		return err
	}
	if err := runStartupChecks(ctx, logger, cfg); err != nil {
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
	managedCount := 0
	if nodeConfig.ManagedServices != nil {
		managedCount = len(nodeConfig.ManagedServices.Services)
	}
	if logger != nil {
		logger.Info("config fetched",
			"config_version", nodeConfig.ConfigVersion,
			"inference_backend", nodeConfig.InferenceBackend != nil,
			"managed_services_count", managedCount)
	}

	if err := applyConfigAndStartServices(ctx, logger, cfg, bootstrap, nodeConfig, opts); err != nil {
		return err
	}

	return runCapabilityLoop(ctx, logger, cfg, bootstrap, nodeConfig, opts)
}

// applyConfigAndStartServices starts Worker API and Ollama from opts (if set), then sends config ack.
// OLLAMA is started only when no existing host inference is detected and config instructs (inference_backend.enabled).
func applyConfigAndStartServices(ctx context.Context, logger *slog.Logger, cfg *Config, bootstrap *BootstrapData, nodeConfig *nodepayloads.NodeConfigurationPayload, opts *RunOptions) error {
	if err := syncManagedServiceAgentTokens(ctx, cfg, nodeConfig, logger); err != nil {
		return err
	}
	applyWorkerProxyConfigEnv(nodeConfig)
	if opts != nil && opts.StartWorkerAPI != nil && nodeConfig != nil && nodeConfig.WorkerAPI != nil && nodeConfig.WorkerAPI.OrchestratorBearerToken != "" {
		if err := opts.StartWorkerAPI(nodeConfig.WorkerAPI.OrchestratorBearerToken); err != nil {
			return fmt.Errorf("start worker API: %w", err)
		}
		if logger != nil {
			logger.Info("worker API started")
		}
	}
	existingService, _ := detectExistingInference(ctx)
	if err := maybeStartOllama(ctx, logger, nodeConfig, opts, existingService); err != nil {
		return err
	}
	maybePullModels(ctx, logger, nodeConfig, opts)
	// Send config ack before starting managed services so the node becomes dispatchable immediately.
	// Otherwise a PMA start failure would block ack and readyz would never see an inference path.
	if err := SendConfigAck(ctx, cfg, bootstrap, nodeConfig, "applied"); err != nil {
		return fmt.Errorf("config ack: %w", err)
	}
	if logger != nil {
		logger.Info("config applied and acknowledged", "config_version", nodeConfig.ConfigVersion)
	}
	if err := maybeStartManagedServices(ctx, logger, nodeConfig, opts); err != nil {
		return err
	}
	return nil
}

func effectiveStateDir() string {
	if v := strings.TrimSpace(getEnv("WORKER_API_STATE_DIR", "")); v != "" {
		return v
	}
	if v := strings.TrimSpace(getEnv("CYNODE_STATE_DIR", "")); v != "" {
		return v
	}
	return "/var/lib/cynode/state"
}

type resolvedAgentToken struct {
	token     string
	expiresAt string
}

func computeDesiredAgentTokens(ctx context.Context, cfg *Config, nodeConfig *nodepayloads.NodeConfigurationPayload) (map[string]resolvedAgentToken, error) {
	desired := map[string]resolvedAgentToken{}
	for i := range nodeConfig.ManagedServices.Services {
		svc := &nodeConfig.ManagedServices.Services[i]
		serviceID := strings.TrimSpace(svc.ServiceID)
		if serviceID == "" {
			continue
		}
		token, expiresAt, hasToken, err := resolveManagedServiceToken(ctx, cfg, svc)
		if err != nil {
			return nil, fmt.Errorf("resolve managed service agent token (service_id=%s): %w", serviceID, err)
		}
		if hasToken {
			desired[serviceID] = resolvedAgentToken{token: token, expiresAt: expiresAt}
		}
	}
	return desired, nil
}

func syncManagedServiceAgentTokens(ctx context.Context, cfg *Config, nodeConfig *nodepayloads.NodeConfigurationPayload, logger *slog.Logger) error {
	if nodeConfig == nil || nodeConfig.ManagedServices == nil {
		return nil
	}
	store, source, err := securestore.Open(effectiveStateDir())
	if err != nil {
		return fmt.Errorf("secure store unavailable for managed service token lifecycle: %w", err)
	}
	defer store.Close()
	if logger != nil && source == securestore.MasterKeySourceEnvB64 {
		logger.Warn("secure store uses env_b64 master key backend; migrate to stronger host-backed key source")
	}
	desired, err := computeDesiredAgentTokens(ctx, cfg, nodeConfig)
	if err != nil {
		return err
	}
	existing, err := store.ListAgentTokenServiceIDs()
	if err != nil {
		return fmt.Errorf("list stored managed service agent tokens: %w", err)
	}
	return reconcileAgentTokenStore(store, desired, existing)
}

func reconcileAgentTokenStore(store *securestore.Store, desired map[string]resolvedAgentToken, existing []string) error {
	for _, serviceID := range existing {
		if _, keep := desired[serviceID]; !keep {
			if err := store.DeleteAgentToken(serviceID); err != nil {
				return fmt.Errorf("delete stale managed service agent token (service_id=%s): %w", serviceID, err)
			}
		}
	}
	for serviceID, tok := range desired {
		if err := store.PutAgentToken(serviceID, tok.token, tok.expiresAt); err != nil {
			return fmt.Errorf("write managed service agent token (service_id=%s): %w", serviceID, err)
		}
	}
	return nil
}

func resolveManagedServiceToken(ctx context.Context, cfg *Config, svc *nodepayloads.ConfigManagedService) (token, expiresAt string, hasToken bool, err error) {
	if svc == nil || svc.Orchestrator == nil {
		return "", "", false, nil
	}
	directToken := strings.TrimSpace(svc.Orchestrator.AgentToken)
	if directToken != "" {
		expires := strings.TrimSpace(svc.Orchestrator.AgentTokenExpiresAt)
		if expires != "" {
			if _, parseErr := time.Parse(time.RFC3339, expires); parseErr != nil {
				return "", "", false, fmt.Errorf("invalid agent_token_expires_at: %w", parseErr)
			}
		}
		return directToken, expires, true, nil
	}
	if svc.Orchestrator.AgentTokenRef == nil {
		return "", "", false, nil
	}
	return resolveAgentTokenRef(ctx, cfg, svc)
}

func resolveAgentTokenRef(ctx context.Context, cfg *Config, svc *nodepayloads.ConfigManagedService) (token, expiresAt string, hasToken bool, err error) {
	if svc == nil || svc.Orchestrator == nil {
		return "", "", false, nil
	}
	ref := svc.Orchestrator.AgentTokenRef
	if ref == nil {
		return "", "", false, nil
	}
	if strings.TrimSpace(ref.Kind) != agentTokenRefKindOrchestratorEndpoint {
		return "", "", false, errors.New("unsupported agent_token_ref.kind")
	}
	refURL := strings.TrimSpace(ref.URL)
	if refURL == "" {
		return "", "", false, errors.New("agent_token_ref.url is required")
	}
	token, expiresAt, err = doAgentTokenRefRequest(ctx, cfg, svc, refURL)
	if err != nil {
		return "", "", false, err
	}
	return token, expiresAt, true, nil
}

func doAgentTokenRefRequest(ctx context.Context, cfg *Config, svc *nodepayloads.ConfigManagedService, refURL string) (token, expiresAt string, err error) {
	reqBody := map[string]string{
		"node_slug":    strings.TrimSpace(cfg.NodeSlug),
		"service_id":   strings.TrimSpace(svc.ServiceID),
		"service_type": strings.TrimSpace(svc.ServiceType),
	}
	if role := strings.TrimSpace(svc.Role); role != "" {
		reqBody["role"] = role
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("marshal agent_token_ref request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refURL, bytes.NewReader(raw))
	if err != nil {
		return "", "", fmt.Errorf("create agent_token_ref request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("agent_token_ref request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("agent_token_ref non-2xx status: %d", resp.StatusCode)
	}
	var tokenResp struct {
		AgentToken          string `json:"agent_token"`
		AgentTokenExpiresAt string `json:"agent_token_expires_at,omitempty"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, httplimits.DefaultMaxHTTPResponseBytes)).Decode(&tokenResp); err != nil {
		return "", "", fmt.Errorf("decode agent_token_ref response: %w", err)
	}
	token = strings.TrimSpace(tokenResp.AgentToken)
	if token == "" {
		return "", "", errors.New("agent_token_ref response missing agent_token")
	}
	expiresAt = strings.TrimSpace(tokenResp.AgentTokenExpiresAt)
	if expiresAt != "" {
		if _, parseErr := time.Parse(time.RFC3339, expiresAt); parseErr != nil {
			return "", "", fmt.Errorf("invalid agent_token_ref response agent_token_expires_at: %w", parseErr)
		}
	}
	return token, expiresAt, nil
}

func applyWorkerProxyConfigEnv(nodeConfig *nodepayloads.NodeConfigurationPayload) {
	if nodeConfig == nil {
		return
	}
	sanitized := sanitizeNodeConfigForWorkerEnv(nodeConfig)
	if raw, err := json.Marshal(sanitized); err == nil {
		_ = os.Setenv("WORKER_NODE_CONFIG_JSON", string(raw))
	}
	if nodeConfig.Orchestrator.BaseURL != "" {
		_ = os.Setenv("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL", nodeConfig.Orchestrator.BaseURL)
	}
	if targets := buildManagedServiceTargetsFromConfig(nodeConfig); len(targets) > 0 {
		if raw, err := json.Marshal(targets); err == nil {
			_ = os.Setenv("WORKER_MANAGED_SERVICE_TARGETS_JSON", string(raw))
		}
	}
}

func sanitizeNodeConfigForWorkerEnv(nodeConfig *nodepayloads.NodeConfigurationPayload) *nodepayloads.NodeConfigurationPayload {
	if nodeConfig == nil {
		return nil
	}
	cp := *nodeConfig
	if nodeConfig.ManagedServices != nil {
		managed := *nodeConfig.ManagedServices
		if len(nodeConfig.ManagedServices.Services) > 0 {
			managed.Services = make([]nodepayloads.ConfigManagedService, 0, len(nodeConfig.ManagedServices.Services))
			for i := range nodeConfig.ManagedServices.Services {
				svc := &nodeConfig.ManagedServices.Services[i]
				svcCopy := *svc
				if svc.Orchestrator != nil {
					orch := *svc.Orchestrator
					orch.AgentToken = ""
					orch.AgentTokenRef = nil
					svcCopy.Orchestrator = &orch
				}
				managed.Services = append(managed.Services, svcCopy)
			}
		}
		cp.ManagedServices = &managed
	}
	return &cp
}

func buildManagedServiceTargetsFromConfig(nodeConfig *nodepayloads.NodeConfigurationPayload) map[string]map[string]string {
	targets := map[string]map[string]string{}
	if nodeConfig == nil || nodeConfig.ManagedServices == nil {
		return targets
	}
	stateDir := strings.TrimSpace(getEnv("WORKER_API_STATE_DIR", filepath.Join(os.TempDir(), "cynode", "state")))
	for i := range nodeConfig.ManagedServices.Services {
		svc := &nodeConfig.ManagedServices.Services[i]
		serviceID := strings.TrimSpace(svc.ServiceID)
		serviceType := strings.TrimSpace(svc.ServiceType)
		if serviceID == "" || serviceType == "" {
			continue
		}
		baseURL := ""
		switch serviceType {
		case serviceTypePMA:
			// REQ-WORKER-0174 / REQ-WORKER-0270: PMA runs with --network=none; reach it over UDS.
			// The per-service socket dir is mounted inside the container and the PMA writes
			// service.sock there. Use http+unix:// so the worker-api proxy uses a unix transport.
			sockPath := filepath.Join(stateDir, ManagedAgentProxySocketBaseDir, serviceID, "service.sock")
			baseURL = "http+unix://" + url.PathEscape(sockPath)
		default:
			continue
		}
		targets[serviceID] = map[string]string{
			"service_type": serviceType,
			"base_url":     baseURL,
		}
	}
	return targets
}

func maybeStartOllama(ctx context.Context, logger *slog.Logger, nodeConfig *nodepayloads.NodeConfigurationPayload, opts *RunOptions, existingService bool) error {
	if opts == nil || opts.StartOllama == nil || existingService ||
		nodeConfig == nil || nodeConfig.InferenceBackend == nil || !nodeConfig.InferenceBackend.Enabled {
		return nil
	}
	image := nodeConfig.InferenceBackend.Image
	variant := strings.TrimSpace(nodeConfig.InferenceBackend.Variant)
	if image == "" && variant != "" {
		// Ollama has rocm tag; cuda/cpu use default image (no separate tag).
		switch variant {
		case "rocm":
			image = "ollama/ollama:rocm"
		default:
			image = "ollama/ollama"
		}
	} else if image == "" {
		image = getEnv("OLLAMA_IMAGE", "ollama/ollama")
	}
	if err := opts.StartOllama(image, variant, nodeConfig.InferenceBackend.Env); err != nil {
		return fmt.Errorf("start inference (Ollama): %w", err)
	}
	if logger != nil {
		logger.Info("inference container started")
	}
	return nil
}

func modelsMissingFromAvailable(models []string, avail map[string]bool) []string {
	var missing []string
	for _, want := range models {
		w := strings.TrimSpace(want)
		if w == "" {
			continue
		}
		if avail[strings.ToLower(w)] {
			continue
		}
		missing = append(missing, w)
	}
	return missing
}

// maybePullModels launches a background goroutine to pull orchestrator-directed models that are not
// yet available on the inference backend. Uses inference_backend.models_to_ensure when non-empty;
// otherwise falls back to selected_model only. The goroutine is detached (not tied to ctx) so a
// slow pull does not block the rest of startup; the next capability report cycle will pick
// up newly available models and the orchestrator will update managed-service config.
func maybePullModels(ctx context.Context, logger *slog.Logger, nodeConfig *nodepayloads.NodeConfigurationPayload, opts *RunOptions) {
	if opts == nil || opts.PullModels == nil {
		return
	}
	if nodeConfig == nil || nodeConfig.InferenceBackend == nil {
		return
	}
	models := modelsToPullFromConfig(nodeConfig)
	if len(models) == 0 {
		return
	}
	available := detectAvailableModels(ctx)
	avail := map[string]bool{}
	for _, m := range available {
		avail[strings.ToLower(strings.TrimSpace(m))] = true
	}
	missing := modelsMissingFromAvailable(models, avail)
	if len(missing) == 0 {
		return
	}
	if logger != nil {
		logger.Info("pulling inference models in background", "models", missing)
	}
	go func() {
		if err := opts.PullModels(ctx, missing); err != nil && logger != nil {
			logger.Warn("model pull failed", "error", err, "models", missing)
		}
	}()
}

func modelsToPullFromConfig(nodeConfig *nodepayloads.NodeConfigurationPayload) []string {
	if nodeConfig == nil || nodeConfig.InferenceBackend == nil {
		return nil
	}
	ib := nodeConfig.InferenceBackend
	if len(ib.ModelsToEnsure) > 0 {
		out := make([]string, 0, len(ib.ModelsToEnsure))
		seen := map[string]bool{}
		for _, m := range ib.ModelsToEnsure {
			s := strings.TrimSpace(m)
			if s == "" {
				continue
			}
			k := strings.ToLower(s)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, s)
		}
		return out
	}
	s := strings.TrimSpace(ib.SelectedModel)
	if s == "" {
		return nil
	}
	return []string{s}
}

func maybeStartManagedServices(ctx context.Context, logger *slog.Logger, nodeConfig *nodepayloads.NodeConfigurationPayload, opts *RunOptions) error {
	if opts == nil || opts.StartManagedServices == nil {
		if logger != nil {
			logger.Debug("managed services skipped", "reason", "no_start_managed_services_runner")
		}
		return nil
	}
	if nodeConfig == nil || nodeConfig.ManagedServices == nil || len(nodeConfig.ManagedServices.Services) == 0 {
		if logger != nil {
			logger.Info("managed services skipped", "reason", "no_managed_services_in_config")
		}
		return nil
	}
	if err := opts.StartManagedServices(ctx, nodeConfig.ManagedServices.Services); err != nil {
		if logger != nil {
			logger.Error("managed services start failed", "error", err, "count", len(nodeConfig.ManagedServices.Services))
		}
		return fmt.Errorf("start managed services: %w", err)
	}
	if logger != nil {
		ids := make([]string, 0, len(nodeConfig.ManagedServices.Services))
		for i := range nodeConfig.ManagedServices.Services {
			ids = append(ids, nodeConfig.ManagedServices.Services[i].ServiceID)
		}
		logger.Info("managed services started", "count", len(nodeConfig.ManagedServices.Services), "service_ids", ids)
	}
	return nil
}

func runCapabilityLoop(ctx context.Context, logger *slog.Logger, cfg *Config, bootstrap *BootstrapData, nodeConfig *nodepayloads.NodeConfigurationPayload, opts *RunOptions) error {
	// Report immediately so orchestrator readyz can become 200 without waiting for first tick.
	if err := reportCapabilities(ctx, cfg, bootstrap, nodeConfig); err != nil {
		_ = err
	}
	ticker := time.NewTicker(cfg.CapabilityReportInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := reportCapabilities(ctx, cfg, bootstrap, nodeConfig); err != nil {
				_ = err
			}
			prev := nodeConfig
			nodeConfig = refreshNodeConfig(ctx, logger, cfg, bootstrap, nodeConfig)
			if inferenceBackendPullSpecChanged(prev, nodeConfig) {
				maybePullModels(ctx, logger, nodeConfig, opts)
			}
			reconcileManagedServices(ctx, logger, nodeConfig, opts)
		}
	}
}
