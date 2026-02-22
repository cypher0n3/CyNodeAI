// Package nodepayloads defines node registration, bootstrap, and capability payloads.
package nodepayloads

// CapabilityReport represents the capability report from a node.
// See docs/tech_specs/worker_node_payloads.md for the normative schema.
type CapabilityReport struct {
	Version    int             `json:"version"`
	ReportedAt string          `json:"reported_at"`
	Node       CapabilityNode  `json:"node"`
	Platform   Platform        `json:"platform"`
	Compute    Compute         `json:"compute"`
	Sandbox    *SandboxSupport `json:"sandbox,omitempty"`
}

type CapabilityNode struct {
	NodeSlug string   `json:"node_slug"`
	Name     string   `json:"name,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}

type Platform struct {
	OS     string `json:"os"`
	Distro string `json:"distro,omitempty"`
	Arch   string `json:"arch"`
}

type Compute struct {
	CPUCores int `json:"cpu_cores"`
	RAMMB    int `json:"ram_mb"`
}

type SandboxSupport struct {
	Supported      bool     `json:"supported"`
	Features       []string `json:"features,omitempty"`
	MaxConcurrency int      `json:"max_concurrency,omitempty"`
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
	Version         int                   `json:"version"`
	ConfigVersion   string                `json:"config_version"`
	IssuedAt        string                `json:"issued_at"`
	NodeSlug        string                `json:"node_slug"`
	Orchestrator    ConfigOrchestrator    `json:"orchestrator"`
	SandboxRegistry ConfigSandboxRegistry `json:"sandbox_registry"`
	ModelCache      ConfigModelCache      `json:"model_cache"`
	Policy          *ConfigPolicy         `json:"policy,omitempty"`
	WorkerAPI       *ConfigWorkerAPI      `json:"worker_api,omitempty"`
	Notes           string                `json:"notes,omitempty"`
	Constraints     *ConfigConstraints    `json:"constraints,omitempty"`
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

// ConfigSandboxRegistry holds sandbox registry config (minimal for Phase 1).
type ConfigSandboxRegistry struct {
	RegistryURL string `json:"registry_url"`
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
	Version             int             `json:"version"`
	NodeSlug            string          `json:"node_slug"`
	ConfigVersion       string          `json:"config_version"`
	AckAt               string          `json:"ack_at"`
	Status              string          `json:"status"`
	Error               *ConfigAckError `json:"error,omitempty"`
	EffectiveConfigHash string          `json:"effective_config_hash,omitempty"`
}

// ConfigAckError holds error details in a config ack.
type ConfigAckError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}
