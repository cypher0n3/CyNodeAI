// Package nodepayloads defines node registration, bootstrap, and capability payloads.
package nodepayloads

// CapabilityReport represents the capability report from a node.
// See docs/tech_specs/worker_node_payloads.md node_capability_report_v1.
type CapabilityReport struct {
	Version               int                    `json:"version"`
	ReportedAt            string                 `json:"reported_at"`
	Node                  CapabilityNode         `json:"node"`
	Platform              Platform               `json:"platform"`
	ContainerRuntime      *ContainerRuntime      `json:"container_runtime,omitempty"`
	Compute               Compute                `json:"compute"`
	GPU                   *GPUInfo               `json:"gpu,omitempty"`
	Sandbox               *SandboxSupport        `json:"sandbox,omitempty"`
	Network               *NetworkInfo           `json:"network,omitempty"`
	Inference             *InferenceInfo         `json:"inference,omitempty"`
	ManagedServices       *ManagedServices       `json:"managed_services,omitempty"`
	ManagedServicesStatus *ManagedServicesStatus `json:"managed_services_status,omitempty"`
	TLS                   *TLSInfo               `json:"tls,omitempty"`
	// WorkerAPI is the node-reported Worker API address; orchestrator uses it for dispatch unless an explicit override is set.
	WorkerAPI *WorkerAPIReport `json:"worker_api,omitempty"`
}

// WorkerAPIReport is the node-reported Worker API address in capability and registration payloads.
// See docs/tech_specs/worker_node_payloads.md capability report worker_api.
type WorkerAPIReport struct {
	BaseURL string `json:"base_url"`
}

type CapabilityNode struct {
	NodeSlug string   `json:"node_slug"`
	Name     string   `json:"name,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}

type Platform struct {
	OS            string `json:"os"`
	Distro        string `json:"distro,omitempty"`
	Arch          string `json:"arch"`
	KernelVersion string `json:"kernel_version,omitempty"`
}

type ContainerRuntime struct {
	Runtime           string `json:"runtime"`
	Version           string `json:"version,omitempty"`
	RootlessSupported bool   `json:"rootless_supported,omitempty"`
	RootlessEnabled   bool   `json:"rootless_enabled,omitempty"`
}

type Compute struct {
	CPUModel      string `json:"cpu_model,omitempty"`
	CPUCores      int    `json:"cpu_cores"`
	RAMMB         int    `json:"ram_mb"`
	StorageFreeMB int    `json:"storage_free_mb,omitempty"`
}

type GPUDevice struct {
	Vendor   string                 `json:"vendor,omitempty"`
	Model    string                 `json:"model,omitempty"`
	DeviceID string                 `json:"device_id,omitempty"`
	VRAMMB   int                    `json:"vram_mb,omitempty"`
	Features map[string]interface{} `json:"features,omitempty"`
}

type GPUInfo struct {
	Present bool        `json:"present"`
	Devices []GPUDevice `json:"devices,omitempty"`
}

type SandboxSupport struct {
	Supported      bool     `json:"supported"`
	Features       []string `json:"features,omitempty"`
	MaxConcurrency int      `json:"max_concurrency,omitempty"`
}

type NetworkInfo struct {
	OrchestratorReachable bool   `json:"orchestrator_reachable,omitempty"`
	OutboundPolicy        string `json:"outbound_policy,omitempty"`
}

// InferenceInfo describes node inference capability per worker_node_payloads.md.
// ExistingService: true when node uses a host-existing inference service (node did not start it).
// Running: true when inference is currently available (node-managed or existing on host).
// AvailableModels: Ollama model names currently loaded/pulled on this node (e.g. ["qwen3:8b", "qwen3.5:0.8b"]).
type InferenceInfo struct {
	Supported       bool     `json:"supported"`
	Mode            string   `json:"mode,omitempty"`
	ExistingService bool     `json:"existing_service,omitempty"`
	Running         bool     `json:"running,omitempty"`
	AvailableModels []string `json:"available_models,omitempty"`
}

// ManagedServices declares whether worker-managed long-lived services are supported by the node.
type ManagedServices struct {
	Supported bool     `json:"supported,omitempty"`
	Features  []string `json:"features,omitempty"`
}

// ManagedServicesStatus reports observed state of worker-managed services.
type ManagedServicesStatus struct {
	Services []ManagedServiceStatus `json:"services,omitempty"`
}

// ManagedServiceStatus is one observed managed service state in capability reports.
type ManagedServiceStatus struct {
	ServiceID                string                          `json:"service_id"`
	ServiceType              string                          `json:"service_type"`
	State                    string                          `json:"state"`
	Endpoints                []string                        `json:"endpoints,omitempty"`
	AgentToOrchestratorProxy *AgentToOrchestratorProxyStatus `json:"agent_to_orchestrator_proxy,omitempty"`
	ReadyAt                  string                          `json:"ready_at,omitempty"`
	Image                    string                          `json:"image,omitempty"`
	ContainerID              string                          `json:"container_id,omitempty"`
	RestartCount             int                             `json:"restart_count,omitempty"`
	ObservedGeneration       string                          `json:"observed_generation,omitempty"`
	LastError                string                          `json:"last_error,omitempty"`
}

// AgentToOrchestratorProxyStatus reports identity-bound internal proxy endpoints for a managed service.
type AgentToOrchestratorProxyStatus struct {
	MCPGatewayProxyURL    string `json:"mcp_gateway_proxy_url,omitempty"`
	ReadyCallbackProxyURL string `json:"ready_callback_proxy_url,omitempty"`
	Binding               string `json:"binding,omitempty"` // per_service_loopback_listener | per_service_uds | other
}

type TLSInfo struct {
	TrustMaterialStatus string `json:"trust_material_status,omitempty"`
}

// RegistrationRequest represents the node registration request with PSK.
type RegistrationRequest struct {
	PSK        string           `json:"psk"`
	Capability CapabilityReport `json:"capability"`
}

// BootstrapResponse represents the bootstrap payload returned on registration.
// Spec CYNAI.WORKER.Payload.BootstrapV1; see docs/tech_specs/worker_node_payloads.md node_bootstrap_payload_v1.
type BootstrapResponse struct {
	Version      int                   `json:"version"`
	IssuedAt     string                `json:"issued_at"`
	Orchestrator BootstrapOrchestrator `json:"orchestrator"`
	Auth         BootstrapAuth         `json:"auth"`
}

// BootstrapOrchestrator contains orchestrator base URL and endpoint URLs.
type BootstrapOrchestrator struct {
	BaseURL   string             `json:"base_url"`
	Endpoints BootstrapEndpoints `json:"endpoints"`
}

// BootstrapEndpoints contains absolute URLs for node-orchestrator communication.
type BootstrapEndpoints struct {
	WorkerRegistrationURL string `json:"worker_registration_url"`
	NodeReportURL         string `json:"node_report_url"`
	NodeConfigURL         string `json:"node_config_url"`
}

type BootstrapAuth struct {
	NodeJWT   string `json:"node_jwt"`
	ExpiresAt string `json:"expires_at"`
}

// SupportedBootstrapVersion returns true if v is a supported bootstrap payload version.
func SupportedBootstrapVersion(v int) bool {
	return v == 1
}

// NodeConfigurationPayload is the node configuration payload (node_configuration_payload_v1).
// Spec CYNAI.WORKER.Payload.ConfigurationV1; see docs/tech_specs/worker_node_payloads.md.
type NodeConfigurationPayload struct {
	Version           int                          `json:"version"`
	ConfigVersion     string                       `json:"config_version"`
	IssuedAt          string                       `json:"issued_at"`
	NodeSlug          string                       `json:"node_slug"`
	Orchestrator      ConfigOrchestrator           `json:"orchestrator"`
	SandboxRegistries []ConfigSandboxRegistryEntry `json:"sandbox_registries,omitempty"`
	ModelCache        ConfigModelCache             `json:"model_cache"`
	Policy            *ConfigPolicy                `json:"policy,omitempty"`
	WorkerAPI         *ConfigWorkerAPI             `json:"worker_api,omitempty"`
	InferenceBackend  *ConfigInferenceBackend      `json:"inference_backend,omitempty"`
	ManagedServices   *ConfigManagedServices       `json:"managed_services,omitempty"`
	Notes             string                       `json:"notes,omitempty"`
	Constraints       *ConfigConstraints           `json:"constraints,omitempty"`
}

// ConfigInferenceBackend instructs the node to start the inference backend (e.g. OLLAMA).
// When absent or Enabled=false the node MUST NOT start an inference container.
// Spec worker_node_payloads.md inference_backend.
type ConfigInferenceBackend struct {
	Enabled bool   `json:"enabled,omitempty"`
	Image   string `json:"image,omitempty"`
	Variant string `json:"variant,omitempty"`
	Port    int    `json:"port,omitempty"`
	// Env is a set of environment variables to pass to the inference container.
	// The orchestrator uses this to tune runtime parameters such as OLLAMA_NUM_CTX
	// based on reported hardware (GPU VRAM, RAM). Node-manager passes each entry
	// as a -e flag when starting the container.
	Env map[string]string `json:"env,omitempty"`
	// SelectedModel is the single model the orchestrator has chosen for this node.
	// Node-manager ensures it is present (pulling in the background if necessary) and
	// sets it as INFERENCE_MODEL for managed service containers. The orchestrator is the
	// sole authority on model selection; node-manager must not substitute an alternative.
	SelectedModel string `json:"selected_model,omitempty"`
}

// ConfigManagedServices is desired state for worker-managed long-lived services.
type ConfigManagedServices struct {
	Services []ConfigManagedService `json:"services,omitempty"`
}

// ConfigManagedService defines desired state for one managed service.
type ConfigManagedService struct {
	ServiceID     string                            `json:"service_id"`
	ServiceType   string                            `json:"service_type"`
	Image         string                            `json:"image"`
	Args          []string                          `json:"args,omitempty"`
	Env           map[string]string                 `json:"env,omitempty"`
	Healthcheck   *ConfigManagedServiceHealthcheck  `json:"healthcheck,omitempty"`
	RestartPolicy string                            `json:"restart_policy,omitempty"`
	Role          string                            `json:"role,omitempty"`
	Inference     *ConfigManagedServiceInference    `json:"inference,omitempty"`
	Orchestrator  *ConfigManagedServiceOrchestrator `json:"orchestrator,omitempty"`
}

// ConfigManagedServiceHealthcheck defines HTTP health probing for managed services.
type ConfigManagedServiceHealthcheck struct {
	Path           string `json:"path,omitempty"`
	ExpectedStatus int    `json:"expected_status,omitempty"`
}

// ConfigManagedServiceInference configures agent inference mode.
type ConfigManagedServiceInference struct {
	Mode             string `json:"mode,omitempty"`
	BaseURL          string `json:"base_url,omitempty"`
	APIEgressBaseURL string `json:"api_egress_base_url,omitempty"`
	ProviderID       string `json:"provider_id,omitempty"`
	DefaultModel     string `json:"default_model,omitempty"`
	WarmupRequired   bool   `json:"warmup_required,omitempty"`
	// BackendEnv carries environment variables derived from the inference backend
	// configuration (e.g. OLLAMA_NUM_CTX sized to GPU VRAM). Node-manager passes
	// these as -e flags to the managed service container so the agent can use them
	// in per-request API options without OS-level env leakage.
	BackendEnv map[string]string `json:"backend_env,omitempty"`
}

// ConfigManagedServiceOrchestrator configures agent-to-orchestrator proxy routes and auth.
type ConfigManagedServiceOrchestrator struct {
	MCPGatewayProxyURL    string                             `json:"mcp_gateway_proxy_url,omitempty"`
	ReadyCallbackProxyURL string                             `json:"ready_callback_proxy_url,omitempty"`
	AgentToken            string                             `json:"agent_token,omitempty"`
	AgentTokenExpiresAt   string                             `json:"agent_token_expires_at,omitempty"`
	AgentTokenRef         *ConfigManagedServiceAgentTokenRef `json:"agent_token_ref,omitempty"`
}

// ConfigManagedServiceAgentTokenRef defines worker-only token reference resolution settings.
type ConfigManagedServiceAgentTokenRef struct {
	Kind string `json:"kind,omitempty"`
	URL  string `json:"url,omitempty"`
}

// ConfigOrchestrator contains orchestrator base URL and endpoints for node config.
type ConfigOrchestrator struct {
	BaseURL   string          `json:"base_url"`
	Endpoints ConfigEndpoints `json:"endpoints"`
}

// ConfigEndpoints contains endpoint URLs in the node configuration payload.
type ConfigEndpoints struct {
	WorkerAPITargetURL string `json:"worker_api_target_url"`
	NodeReportURL      string `json:"node_report_url"`
}

// ConfigSandboxRegistryEntry is one entry in the sandbox_registries array (rank-ordered).
type ConfigSandboxRegistryEntry struct {
	RegistryURL        string `json:"registry_url"`
	PullToken          string `json:"pull_token,omitempty"`
	PullTokenExpiresAt string `json:"pull_token_expires_at,omitempty"`
}

// ConfigModelCache holds model cache config (minimal for Phase 1).
type ConfigModelCache struct {
	CacheURL string `json:"cache_url"`
}

// ConfigPolicy holds policy overrides (optional for Phase 1).
type ConfigPolicy struct {
	Sandbox *ConfigSandboxPolicy `json:"sandbox,omitempty"`
}

// ConfigSandboxPolicy holds sandbox policy fields.
type ConfigSandboxPolicy struct {
	DefaultNetworkPolicy string `json:"default_network_policy,omitempty"`
}

// ConfigWorkerAPI holds Worker API auth token for orchestrator-to-node calls.
type ConfigWorkerAPI struct {
	OrchestratorBearerToken          string `json:"orchestrator_bearer_token,omitempty"`
	OrchestratorBearerTokenExpiresAt string `json:"orchestrator_bearer_token_expires_at,omitempty"`
}

// ConfigConstraints holds optional request/job limits.
type ConfigConstraints struct {
	MaxRequestBytes      int `json:"max_request_bytes,omitempty"`
	MaxJobTimeoutSeconds int `json:"max_job_timeout_seconds,omitempty"`
}

// ConfigAck is the node configuration acknowledgement (node_config_ack_v1).
// Spec CYNAI.WORKER.Payload.ConfigAckV1; see docs/tech_specs/worker_node_payloads.md.
type ConfigAck struct {
	Version               int                    `json:"version"`
	NodeSlug              string                 `json:"node_slug"`
	ConfigVersion         string                 `json:"config_version"`
	AckAt                 string                 `json:"ack_at"`
	Status                string                 `json:"status"`
	Error                 *ConfigAckError        `json:"error,omitempty"`
	EffectiveConfigHash   string                 `json:"effective_config_hash,omitempty"`
	ManagedServicesStatus *ManagedServicesStatus `json:"managed_services_status,omitempty"`
}

// ConfigAckError holds error details in a config ack.
type ConfigAckError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}
