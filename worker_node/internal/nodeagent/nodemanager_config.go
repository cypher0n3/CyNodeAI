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
	"github.com/cypher0n3/cynodeai/worker_node/internal/containerps"
)

// refreshNodeConfig re-fetches the node config from the orchestrator and returns the
// updated config when managed services or orchestrator-directed inference model lists change.
// Returns the current config unchanged on error or when nothing changed.
func refreshNodeConfig(ctx context.Context, logger *slog.Logger, cfg *Config, bootstrap *BootstrapData, current *nodepayloads.NodeConfigurationPayload) *nodepayloads.NodeConfigurationPayload {
	updated, err := FetchConfig(ctx, cfg, bootstrap)
	if err != nil {
		return current
	}
	ms := managedServicesConfigChanged(current, updated)
	inf := inferenceBackendPullSpecChanged(current, updated)
	if !ms && !inf {
		return current
	}
	if logger != nil {
		switch {
		case ms && inf:
			logger.Info("node config changed (managed services and inference models), reconciling")
		case ms:
			logger.Info("node config changed (managed services), reconciling")
		case inf:
			logger.Info("node config changed (inference models), reconciling pulls")
		}
	}
	return updated
}

// inferenceBackendPullSpecChanged is true when orchestrator-directed model pull inputs differ.
func inferenceBackendPullSpecChanged(oldCfg, newCfg *nodepayloads.NodeConfigurationPayload) bool {
	return inferenceBackendPullSpecKey(oldCfg) != inferenceBackendPullSpecKey(newCfg)
}

func inferenceBackendPullSpecKey(cfg *nodepayloads.NodeConfigurationPayload) string {
	if cfg == nil || cfg.InferenceBackend == nil {
		return ""
	}
	ib := cfg.InferenceBackend
	data := struct {
		Selected string   `json:"selected"`
		Ensure   []string `json:"ensure"`
	}{
		Selected: strings.TrimSpace(ib.SelectedModel),
		Ensure:   append([]string(nil), ib.ModelsToEnsure...),
	}
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(b)
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
	existingService = containerps.NameListed(string(out), name)
	if !existingService {
		return false, false
	}
	cmd2 := exec.Command(rt, "ps", "--format", "{{.Names}}")
	out2, err2 := cmd2.Output()
	if err2 != nil {
		return true, false
	}
	if !containerps.NameListed(string(out2), name) {
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
