// Package nodepayloads defines node registration, bootstrap, and capability payloads.
package nodepayloads

// CapabilityReport represents the capability report from a node.
// See docs/tech_specs/node_payloads.md for the normative schema.
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
// See docs/tech_specs/node_payloads.md node_bootstrap_payload_v1.
type BootstrapResponse struct {
	Version  int           `json:"version"`
	IssuedAt string        `json:"issued_at"`
	Auth     BootstrapAuth `json:"auth"`
}

type BootstrapAuth struct {
	NodeJWT   string `json:"node_jwt"`
	ExpiresAt string `json:"expires_at"`
}
