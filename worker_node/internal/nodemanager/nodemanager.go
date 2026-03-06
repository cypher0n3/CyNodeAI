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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"runtime"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
)

const (
	agentTokenRefKindOrchestratorEndpoint = "orchestrator_endpoint"
	serviceTypePMA                       = "pma"
	proxyURLAuto                         = "auto"
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
// StartOllama is Phase 1 inference; image/variant come from config or env; if it returns an error, Run fails (fail-fast).
// StartManagedServices starts orchestrator-directed managed service containers (e.g. PMA) from desired state; if it returns an error, Run fails.
type RunOptions struct {
	StartWorkerAPI      func(bearerToken string) error
	StartOllama         func(image, variant string) error
	StartManagedServices func(services []nodepayloads.ConfigManagedService) error
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

	if err := applyConfigAndStartServices(ctx, logger, cfg, bootstrap, nodeConfig, opts); err != nil {
		return err
	}

	return runCapabilityLoop(ctx, cfg, bootstrap, nodeConfig)
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
	if err := maybeStartManagedServices(ctx, logger, nodeConfig, opts); err != nil {
		return err
	}
	if err := SendConfigAck(ctx, cfg, bootstrap, nodeConfig, "applied"); err != nil {
		return fmt.Errorf("config ack: %w", err)
	}
	if logger != nil {
		logger.Info("config applied and acknowledged", "config_version", nodeConfig.ConfigVersion)
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
	pmaBaseURL := strings.TrimSpace(getEnv("PMA_BASE_URL", "http://127.0.0.1:8090"))
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
			baseURL = pmaBaseURL
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
	if err := opts.StartOllama(image, variant); err != nil {
		return fmt.Errorf("start inference (Ollama): %w", err)
	}
	if logger != nil {
		logger.Info("inference container started")
	}
	return nil
}

func maybeStartManagedServices(ctx context.Context, logger *slog.Logger, nodeConfig *nodepayloads.NodeConfigurationPayload, opts *RunOptions) error {
	if opts == nil || opts.StartManagedServices == nil ||
		nodeConfig == nil || nodeConfig.ManagedServices == nil || len(nodeConfig.ManagedServices.Services) == 0 {
		return nil
	}
	if err := opts.StartManagedServices(nodeConfig.ManagedServices.Services); err != nil {
		return fmt.Errorf("start managed services: %w", err)
	}
	if logger != nil {
		logger.Info("managed services started", "count", len(nodeConfig.ManagedServices.Services))
	}
	return nil
}

func runCapabilityLoop(ctx context.Context, cfg *Config, bootstrap *BootstrapData, nodeConfig *nodepayloads.NodeConfigurationPayload) error {
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
		Version:       1,
		NodeSlug:      nodeConfig.NodeSlug,
		ConfigVersion: nodeConfig.ConfigVersion,
		AckAt:         time.Now().UTC().Format(time.RFC3339),
		Status:        status,
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
	existingService = strings.Contains(string(out), name)
	if !existingService {
		return false, false
	}
	cmd2 := exec.Command(rt, "ps", "--format", "{{.Names}}")
	out2, err2 := cmd2.Output()
	if err2 != nil {
		return true, false
	}
	if !strings.Contains(string(out2), name) {
		return true, false
	}
	// Container is running; optionally verify reachability (e.g. HTTP to 11434).
	port := getEnv("OLLAMA_PORT", "11434")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/api/tags", http.NoBody)
	if err != nil {
		return true, true
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return true, true
	}
	_ = resp.Body.Close()
	running = resp.StatusCode == http.StatusOK
	return true, running
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
	existing, running := detectExistingInference(ctx)
	report.Inference = &nodepayloads.InferenceInfo{
		Supported:       true,
		ExistingService: existing,
		Running:         running,
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
