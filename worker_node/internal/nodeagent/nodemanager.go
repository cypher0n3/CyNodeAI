// Package nodeagent provides node registration and capability reporting.
// It is used by the node-manager command (cmd/node-manager).
package nodeagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
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
	StartManagedServices func(services []nodepayloads.ConfigManagedService) error
	// PullModels is called in the background after the inference backend starts.
	// It receives the ordered list of desired model names and is expected to pull
	// any that are not already available. May be nil (pull skipped).
	PullModels func(models []string) error
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
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
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
			// REQ-WORKER-0174 / REQ-WORKER-0260: PMA runs with --network=none; reach it over UDS.
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
	variant := nodeConfig.InferenceBackend.Variant
	if image == "" {
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

// maybePullModels launches a background goroutine to pull the orchestrator-selected model if it is not
// yet available on the inference backend. It is a no-op when PullModels is unset or when
// the config carries no SelectedModel. The goroutine is detached (not tied to ctx) so a
// slow pull does not block the rest of startup; the next capability report cycle will pick
// up newly available models and the orchestrator will update managed-service config.
func maybePullModels(ctx context.Context, logger *slog.Logger, nodeConfig *nodepayloads.NodeConfigurationPayload, opts *RunOptions) {
	if opts == nil || opts.PullModels == nil {
		return
	}
	if nodeConfig == nil || nodeConfig.InferenceBackend == nil {
		return
	}
	selected := strings.TrimSpace(nodeConfig.InferenceBackend.SelectedModel)
	if selected == "" {
		return
	}
	available := detectAvailableModels(ctx)
	for _, m := range available {
		if strings.EqualFold(m, selected) {
			return
		}
	}
	if logger != nil {
		logger.Info("pulling selected inference model in background", "model", selected)
	}
	go func() {
		if err := opts.PullModels([]string{selected}); err != nil && logger != nil {
			logger.Warn("model pull failed", "error", err, "model", selected)
		}
	}()
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
	if err := opts.StartManagedServices(nodeConfig.ManagedServices.Services); err != nil {
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
			nodeConfig = refreshNodeConfig(ctx, logger, cfg, bootstrap, nodeConfig)
			reconcileManagedServices(ctx, logger, nodeConfig, opts)
		}
	}
}

// refreshNodeConfig re-fetches the node config from the orchestrator and returns the
// updated config when managed services settings have changed (e.g. after the orchestrator
// selects a better model based on the newly submitted capability snapshot).
// Returns the current config unchanged on error or when nothing changed.
func refreshNodeConfig(ctx context.Context, logger *slog.Logger, cfg *Config, bootstrap *BootstrapData, current *nodepayloads.NodeConfigurationPayload) *nodepayloads.NodeConfigurationPayload {
	updated, err := FetchConfig(ctx, cfg, bootstrap)
	if err != nil {
		return current
	}
	if !managedServicesConfigChanged(current, updated) {
		return current
	}
	if logger != nil {
		logger.Info("node config changed, reconciling managed services")
	}
	return updated
}

// managedServicesConfigChanged returns true when the managed services section of two
// node configs differs in a way that warrants a managed service reconciliation restart.
func managedServicesConfigChanged(old, updated *nodepayloads.NodeConfigurationPayload) bool {
	if old == nil || updated == nil {
		return old != updated
	}
	oldJSON, _ := json.Marshal(old.ManagedServices)
	newJSON, _ := json.Marshal(updated.ManagedServices)
	return !bytes.Equal(oldJSON, newJSON)
}

// reconcileManagedServices restarts any managed service containers that have stopped.
// This handles cases where a container exits unexpectedly (e.g. OOM, crash) or is
// removed externally. Best-effort: errors are logged, not propagated.
func reconcileManagedServices(ctx context.Context, logger *slog.Logger, nodeConfig *nodepayloads.NodeConfigurationPayload, opts *RunOptions) {
	if opts == nil || opts.StartManagedServices == nil {
		return
	}
	if nodeConfig == nil || nodeConfig.ManagedServices == nil || len(nodeConfig.ManagedServices.Services) == 0 {
		return
	}
	if err := opts.StartManagedServices(nodeConfig.ManagedServices.Services); err != nil {
		if logger != nil {
			logger.Warn("managed services reconcile error", "error", err)
		}
	}
}

// FetchConfig fetches the node configuration from the bootstrap node_config_url (GET).
func FetchConfig(ctx context.Context, cfg *Config, bootstrap *BootstrapData) (*nodepayloads.NodeConfigurationPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bootstrap.NodeConfigURL, http.NoBody)
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
		Version:               1,
		NodeSlug:              nodeConfig.NodeSlug,
		ConfigVersion:         nodeConfig.ConfigVersion,
		AckAt:                 time.Now().UTC().Format(time.RFC3339),
		Status:                status,
		ManagedServicesStatus: buildManagedServicesStatus(nodeConfig),
	}
	body, err := json.Marshal(ack)
	if err != nil {
		return fmt.Errorf("marshal config ack: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bootstrap.NodeConfigURL, bytes.NewReader(body))
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

// waitForOrchestratorReadiness polls the control-plane /readyz until it returns 200 or 503
// (control-plane is listening). Per worker_node startup procedure and orchestrator_bootstrap.md:
// the node handles orchestrator readiness; the dev script does not poll before starting the node.
const defaultOrchestratorReadinessTimeout = 90 * time.Second
const orchestratorReadinessInterval = 1 * time.Second

func waitForOrchestratorReadiness(ctx context.Context, logger *slog.Logger, cfg *Config) error {
	timeout := defaultOrchestratorReadinessTimeout
	if d := getDurationEnv("NODE_MANAGER_READINESS_TIMEOUT", 0); d > 0 {
		timeout = d
	}
	readyzURL := strings.TrimSuffix(cfg.OrchestratorURL, "/") + "/readyz"
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, readyzURL, http.NoBody)
		if err != nil {
			time.Sleep(orchestratorReadinessInterval)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(orchestratorReadinessInterval)
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable {
			if logger != nil {
				logger.Info("orchestrator control-plane reachable", "readyz", resp.StatusCode)
			}
			return nil
		}
		time.Sleep(orchestratorReadinessInterval)
	}
	return fmt.Errorf("orchestrator control-plane not reachable at %s within %v", readyzURL, timeout)
}

// runStartupChecks runs initial startup checks before registering with the orchestrator.
// Per docs/tech_specs/worker_node.md Node Startup Procedure (steps 3–4) and Node Startup Checks:
// verify container runtime can run containers; detect existing inference (OLLAMA) and log so
// the capability report sent at registration is accurate. The node MUST NOT report ready until
// these pass; we run them before register so we fail fast if the runtime is unavailable.
func runStartupChecks(ctx context.Context, logger *slog.Logger, cfg *Config) error {
	if err := checkContainerRuntime(ctx, logger, cfg); err != nil {
		return fmt.Errorf("startup check (container runtime): %w", err)
	}
	existing, running := detectExistingInference(ctx)
	if logger != nil {
		logger.Info("startup check: existing inference", "existing_service", existing, "running", running)
	}
	return nil
}

const containerRuntimeCheckTimeout = 30 * time.Second

// checkContainerRuntime verifies the configured container runtime (Podman or Docker) can create
// and run a container. Per worker_node.md Node Startup Checks: if the runtime is unavailable
// or fails the check, the node MUST NOT report ready. Skip when NODE_MANAGER_SKIP_CONTAINER_CHECK is set (e.g. tests).
func checkContainerRuntime(ctx context.Context, logger *slog.Logger, cfg *Config) error {
	if getEnv("NODE_MANAGER_SKIP_CONTAINER_CHECK", "") != "" {
		if logger != nil {
			logger.Info("startup check: container runtime skipped (NODE_MANAGER_SKIP_CONTAINER_CHECK)")
		}
		return nil
	}
	rt := getEnv("CONTAINER_RUNTIME", "podman")
	// Use a minimal image and run a no-op; image may be pulled on first run.
	image := getEnv("NODE_MANAGER_RUNTIME_CHECK_IMAGE", "docker.io/library/busybox:latest")
	checkCtx, cancel := context.WithTimeout(ctx, containerRuntimeCheckTimeout)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, rt, "run", "--rm", image, "true")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s run --rm %s true: %w (output: %s)", rt, image, err, strings.TrimSpace(string(out)))
	}
	if logger != nil {
		logger.Info("startup check: container runtime OK", "runtime", rt)
	}
	return nil
}

func register(ctx context.Context, cfg *Config) (*BootstrapData, error) {
	capability := buildCapability(ctx, cfg, nil)

	req := nodepayloads.RegistrationRequest{
		PSK:        cfg.RegistrationPSK,
		Capability: capability,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal registration request: %w", err)
	}

	registerURL := cfg.OrchestratorURL + "/v1/nodes/register"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, bytes.NewReader(body))
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

func reportCapabilities(ctx context.Context, cfg *Config, bootstrap *BootstrapData, nodeConfig *nodepayloads.NodeConfigurationPayload) error {
	report := buildCapability(ctx, cfg, nodeConfig)
	body, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal capability report: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bootstrap.NodeReportURL, bytes.NewReader(body))
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

// containerNameMatches reports whether the podman ps output contains an exact container name
// match. podman ps --format {{.Names}} emits one name per line; we check each line to avoid
// false positives where the target is a prefix of another container name
// (e.g. "cynodeai-ollama" matching "cynodeai-ollama-proxy-test").
func containerNameMatches(psOutput, name string) bool {
	for _, line := range strings.Split(psOutput, "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

// detectExistingInference returns whether an inference container/service already exists and is running.
// Used to set inference.existing_service and inference.running in capability reports.
// When NODE_MANAGER_TEST_NO_EXISTING_INFERENCE is set (e.g. unit tests), returns false, false.
func detectExistingInference(ctx context.Context) (existingService, running bool) {
	if getEnv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "") != "" {
		return false, false
	}
	rt := getEnv("CONTAINER_RUNTIME", "podman")
	name := getEnv("OLLAMA_CONTAINER_NAME", "cynodeai-ollama")
	cmd := exec.Command(rt, "ps", "-a", "--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return false, false
	}
	existingService = containerNameMatches(string(out), name)
	if !existingService {
		return false, false
	}
	cmd2 := exec.Command(rt, "ps", "--format", "{{.Names}}")
	out2, err2 := cmd2.Output()
	if err2 != nil {
		return true, false
	}
	if !containerNameMatches(string(out2), name) {
		return true, false
	}
	port := getEnv("OLLAMA_PORT", "11434")
	_, running = queryOllamaTags(ctx, port)
	return true, running
}

// ollamaTagsResponse is the shape of GET /api/tags from Ollama.
type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// queryOllamaTags fetches /api/tags from Ollama; returns (names, ok).
// When NODE_MANAGER_TEST_NO_EXISTING_INFERENCE is set, returns (nil, false).
func queryOllamaTags(ctx context.Context, port string) ([]string, bool) {
	tagsURL := "http://127.0.0.1:" + port + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, http.NoBody)
	if err != nil {
		return nil, false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()
	var tags ollamaTagsResponse
	if json.NewDecoder(resp.Body).Decode(&tags) != nil {
		return nil, true
	}
	names := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		if n := strings.TrimSpace(m.Name); n != "" {
			names = append(names, n)
		}
	}
	return names, true
}

// detectAvailableModels returns the list of Ollama model names on this node.
// Returns nil when inference is disabled in tests or Ollama is unreachable.
func detectAvailableModels(ctx context.Context) []string {
	if getEnv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE", "") != "" {
		return nil
	}
	port := getEnv("OLLAMA_PORT", "11434")
	names, _ := queryOllamaTags(ctx, port)
	return names
}

func buildCapability(ctx context.Context, cfg *Config, nodeConfig *nodepayloads.NodeConfigurationPayload) nodepayloads.CapabilityReport {
	if cfg == nil {
		return nodepayloads.CapabilityReport{
			Version:    1,
			ReportedAt: time.Now().UTC().Format(time.RFC3339),
			Platform:   nodepayloads.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH},
			Compute:    nodepayloads.Compute{CPUCores: runtime.NumCPU(), RAMMB: 4096},
			Sandbox:    &nodepayloads.SandboxSupport{Supported: true, Features: []string{"netns"}, MaxConcurrency: 4},
		}
	}
	report := nodepayloads.CapabilityReport{
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
		ManagedServices: &nodepayloads.ManagedServices{
			Supported: true,
			Features: []string{
				"service_containers",
				"agent_orchestrator_proxy_bidirectional",
				"agent_orchestrator_proxy_identity_bound",
				"agent_proxy_urls_auto",
			},
		},
	}
	if strings.TrimSpace(cfg.AdvertisedWorkerAPIURL) != "" {
		report.WorkerAPI = &nodepayloads.WorkerAPIReport{BaseURL: strings.TrimSpace(cfg.AdvertisedWorkerAPIURL)}
	}
	report.GPU = cachedGPUInfo(ctx)
	existing, running := detectExistingInference(ctx)
	report.Inference = &nodepayloads.InferenceInfo{
		Supported:       true,
		ExistingService: existing,
		Running:         running,
		AvailableModels: detectAvailableModels(ctx),
	}
	report.ManagedServicesStatus = buildManagedServicesStatus(nodeConfig)
	return report
}

func buildManagedServicesStatus(nodeConfig *nodepayloads.NodeConfigurationPayload) *nodepayloads.ManagedServicesStatus {
	if nodeConfig == nil || nodeConfig.ManagedServices == nil || len(nodeConfig.ManagedServices.Services) == 0 {
		return nil
	}
	stateDir := effectiveStateDir()
	out := &nodepayloads.ManagedServicesStatus{Services: []nodepayloads.ManagedServiceStatus{}}
	for i := range nodeConfig.ManagedServices.Services {
		svc := &nodeConfig.ManagedServices.Services[i]
		serviceID := strings.TrimSpace(svc.ServiceID)
		serviceType := strings.TrimSpace(svc.ServiceType)
		if serviceID == "" || serviceType == "" {
			continue
		}
		status := nodepayloads.ManagedServiceStatus{
			ServiceID:   serviceID,
			ServiceType: serviceType,
			State:       "starting",
		}
		// PMA: report "ready" with endpoint so orchestrator can route chat (REQ-ORCHES-0162).
		// The gateway must call the worker's proxy URL (not direct PMA) so the request reaches this node's PMA.
		// Prefer worker proxy URL when NODE_ADVERTISED_WORKER_API_URL is set; fallback to PMA_ADVERTISED_URL for backward compat.
		if serviceType == serviceTypePMA {
			workerBase := strings.TrimSpace(getEnv("NODE_ADVERTISED_WORKER_API_URL", ""))
			if workerBase != "" {
				proxyURL := strings.TrimSuffix(workerBase, "/") + "/v1/worker/managed-services/" + serviceID + "/proxy:http"
				status.State = "ready"
				status.Endpoints = []string{proxyURL}
				status.ReadyAt = time.Now().UTC().Format(time.RFC3339)
			} else {
				advertised := strings.TrimSpace(getEnv("PMA_ADVERTISED_URL", getEnv("PMA_BASE_URL", "")))
				if advertised != "" {
					status.State = "ready"
					status.Endpoints = []string{advertised}
					status.ReadyAt = time.Now().UTC().Format(time.RFC3339)
				}
			}
		}
		status.AgentToOrchestratorProxy = buildAgentToOrchestratorProxyStatus(stateDir, serviceID, svc.Orchestrator)
		out.Services = append(out.Services, status)
	}
	return out
}

func buildAgentToOrchestratorProxyStatus(
	stateDir, serviceID string,
	orch *nodepayloads.ConfigManagedServiceOrchestrator,
) *nodepayloads.AgentToOrchestratorProxyStatus {
	if orch == nil {
		return nil
	}
	mcpURL := strings.TrimSpace(orch.MCPGatewayProxyURL)
	readyURL := strings.TrimSpace(orch.ReadyCallbackProxyURL)
	if mcpURL == "" && readyURL == "" {
		return nil
	}
	socketPath := filepath.Join(stateDir, "run", "managed_agent_proxy", serviceID, "proxy.sock")
	escaped := url.PathEscape(socketPath)
	out := &nodepayloads.AgentToOrchestratorProxyStatus{Binding: "per_service_uds"}
	if mcpURL == proxyURLAuto {
		out.MCPGatewayProxyURL = "http+unix://" + escaped + "/v1/worker/internal/orchestrator/mcp:call"
	} else if mcpURL != "" {
		out.MCPGatewayProxyURL = mcpURL
	}
	if readyURL == proxyURLAuto {
		out.ReadyCallbackProxyURL = "http+unix://" + escaped + "/v1/worker/internal/orchestrator/agent:ready"
	} else if readyURL != "" {
		out.ReadyCallbackProxyURL = readyURL
	}
	return out
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
