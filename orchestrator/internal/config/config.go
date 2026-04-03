// Package config provides configuration for CyNodeAI services.
// See docs/tech_specs/orchestrator_bootstrap.md for bootstrap details.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
)

// Well-known dev defaults shipped in LoadOrchestratorConfig. When ORCHESTRATOR_DEV_MODE=false,
// ValidateSecrets rejects configuration that still uses these values for production safety.
const (
	DefaultJWTSecret              = "change-me-in-production"
	DefaultNodeRegistrationPSK    = "default-psk-change-me"
	DefaultWorkerAPIBearerToken   = "dev-worker-api-token-change-me"
	DefaultBootstrapAdminPassword = "admin123"
	envOrchestratorDevMode        = "ORCHESTRATOR_DEV_MODE"
)

// OrchestratorConfig holds orchestrator configuration.
type OrchestratorConfig struct {
	// Database
	DatabaseURL string

	// Server
	ListenAddr       string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	MaxHeaderBytes   int
	MaxRequestBodyMB int

	// JWT
	JWTSecret          string
	JWTAccessDuration  time.Duration
	JWTRefreshDuration time.Duration
	JWTNodeDuration    time.Duration

	// Node Registration
	NodeRegistrationPSK string

	// Orchestrator public URL for bootstrap payload (emitted to nodes).
	OrchestratorPublicURL string

	// Worker API: bearer token delivered to nodes for orchestrator-to-node auth (Phase 1: static).
	WorkerAPIBearerToken string

	// Worker API target URL for single-node Phase 1 (optional; used when node has not yet reported its URL).
	WorkerAPITargetURL string

	// Worker internal agent token: when set, included in managed_services.services[].orchestrator.agent_token
	// so agents (e.g. PMA) can authenticate to the worker's internal orchestrator proxy. Optional; no default.
	WorkerInternalAgentToken string

	// MCPSandboxAgentBearerToken when set (with WorkerInternalAgentToken) configures a second MCP gateway
	// bearer identity for sandbox/worker agents; restricted to the worker tool allowlist (mcpgateway/allowlist.go).
	MCPSandboxAgentBearerToken string // MCP_SANDBOX_AGENT_BEARER_TOKEN; optional
	// MCPPAAgentBearerToken identifies Project Analyst (PA) agents for MCP tool-call allowlists (optional).
	MCPPAAgentBearerToken string // MCP_PA_AGENT_BEARER_TOKEN; optional

	// Bootstrap
	BootstrapAdminPassword string

	// Rate Limiting
	RateLimitPerMinute int

	// Inference (PM model): when set, prompt-mode tasks call this URL directly so prompt→model MUST work (MVP Phase 1).
	InferenceURL   string // e.g. http://localhost:11434 or http://ollama:11434
	InferenceModel string // e.g. qwen3.5:0.8b; default qwen3.5:0.8b

	// PMA (cynode-pma): when enabled, control-plane starts cynode-pma as subprocess so PM chat surface is backed by the agent; GET /readyz returns 503 until PMA is reachable. Default true per REQ-ORCHES-0120 and orchestrator.md HealthEndpoints.
	PMAEnabled          bool   // PMA_ENABLED; default true
	PMABinaryPath       string // PMA_BINARY; path to cynode-pma binary
	PMAListenAddr       string // PMA_LISTEN_ADDR; default :8090
	PMAInstructionsRoot string // PMA_INSTRUCTIONS_ROOT; optional
	// PMABaseURL is deprecated for chat routing. PMA is only reachable via worker-reported capability (orchestrator ↔ worker proxy). Kept for backward compat / other use.
	PMABaseURL string // PMA_BASE_URL; not used for resolvePMAEndpoint
	// PMA managed-service desired-state defaults (worker-managed PMA path).
	// PMA_SERVICE_ID is reserved for bootstrap/main PMA. Session-backed workers use pma-pool-* warm pool
	// sizing via PMA_WARM_POOL_MIN_FREE and PMA_WARM_POOL_MAX_SLOTS (see handlers/pma_pool.go).
	PMAServiceID       string // PMA_SERVICE_ID
	PMAImage           string // PMA_IMAGE
	PMAHostNodeSlug    string // PMA_HOST_NODE_SLUG (optional explicit placement override)
	PMAPreferHostLabel string // PMA_PREFER_HOST_LABEL

	// Workflow runner: bearer token for workflow start/resume/checkpoint/release API. When set, requests must include Authorization: Bearer <value>.
	WorkflowRunnerBearerToken string // WORKFLOW_RUNNER_BEARER_TOKEN; optional

	// NATS: URLs and account signing material (decentralized JWT). See docs/tech_specs/nats_messaging.md.
	NATSClientURL string // NATS_CLIENT_URL (fallback NATS_URL); emitted to clients in login/bootstrap.
	// NATSServerURL is the broker URL this process dials for JetStream/service connections (optional).
	// In Compose, set NATS_SERVER_URL=nats://nats:4222 while NATS_CLIENT_URL stays host-reachable for clients.
	NATSServerURL          string // NATS_SERVER_URL
	NATSWebSocketURL       string // NATS_WEBSOCKET_URL; Web Console / ws clients.
	NATSAccountSeed        string // NATS_ACCOUNT_SEED; NKey seed for account public id (dev default from natsjwt when DevMode).
	NATSAccountSigningSeed string // NATS_ACCOUNT_SIGNING_SEED; signing key for user JWTs.

	// DevMode when true allows JWT, node PSK, worker API bearer, and bootstrap admin password to match
	// the well-known defaults above (local dev and E2E). Set ORCHESTRATOR_DEV_MODE=false in production
	// and supply non-default secrets (see ValidateSecrets).
	DevMode bool

	// Artifacts (S3/MinIO): when endpoint is empty, user-gateway and MCP artifact tools stay disabled for blob ops.
	ArtifactsS3Endpoint        string // ARTIFACTS_S3_ENDPOINT
	ArtifactsS3Region          string // ARTIFACTS_S3_REGION; default us-east-1
	ArtifactsS3AccessKey       string // ARTIFACTS_S3_ACCESS_KEY
	ArtifactsS3SecretKey       string // ARTIFACTS_S3_SECRET_KEY
	ArtifactsS3Bucket          string // ARTIFACTS_S3_BUCKET
	ArtifactHashInlineMaxBytes int64  // ARTIFACT_HASH_INLINE_MAX_BYTES; default 1 MiB
	// Background hash backfill for large uploads that omitted checksum (disabled by default).
	ArtifactHashBackfillEnabled  bool          // ARTIFACT_HASH_BACKFILL_ENABLED; default false
	ArtifactHashBackfillInterval time.Duration // ARTIFACT_HASH_BACKFILL_INTERVAL; default 10m
	// Stale cleanup by max age (disabled by default; use with care).
	ArtifactStaleCleanupEnabled     bool          // ARTIFACT_STALE_CLEANUP_ENABLED; default false
	ArtifactStaleCleanupInterval    time.Duration // ARTIFACT_STALE_CLEANUP_INTERVAL; default 1h
	ArtifactStaleCleanupMaxAgeHours int           // ARTIFACT_STALE_CLEANUP_MAX_AGE_HOURS; 0 disables pruning
}

// NodeConfig holds node manager configuration.
type NodeConfig struct {
	// Orchestrator
	OrchestratorURL string
	RegistrationPSK string

	// Node identity
	NodeSlug string
	NodeName string

	// Worker API
	ListenAddr     string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxHeaderBytes int

	// Container runtime
	ContainerRuntime string

	// Sandbox defaults
	DefaultTimeoutSeconds int
	MaxOutputBytes        int
}

// LoadOrchestratorConfig loads configuration from environment.
func LoadOrchestratorConfig() *OrchestratorConfig {
	cfg := &OrchestratorConfig{
		DatabaseURL:                     getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cynodeai?sslmode=disable"),
		ListenAddr:                      getEnv("LISTEN_ADDR", ":12080"),
		ReadTimeout:                     getDurationEnv("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:                    getDurationEnv("WRITE_TIMEOUT", 300*time.Second), // chat path with thinking models can take 3+ minutes
		IdleTimeout:                     getDurationEnv("IDLE_TIMEOUT", 120*time.Second),
		MaxHeaderBytes:                  getIntEnv("MAX_HEADER_BYTES", 1<<20),
		MaxRequestBodyMB:                getIntEnv("MAX_REQUEST_BODY_MB", 10),
		JWTSecret:                       getEnv("JWT_SECRET", DefaultJWTSecret),
		JWTAccessDuration:               getDurationEnv("JWT_ACCESS_DURATION", 15*time.Minute),
		JWTRefreshDuration:              getDurationEnv("JWT_REFRESH_DURATION", 7*24*time.Hour),
		JWTNodeDuration:                 getDurationEnv("JWT_NODE_DURATION", 24*time.Hour),
		NodeRegistrationPSK:             getEnv("NODE_REGISTRATION_PSK", DefaultNodeRegistrationPSK),
		OrchestratorPublicURL:           getEnv("ORCHESTRATOR_PUBLIC_URL", "http://localhost:12082"),
		WorkerAPIBearerToken:            getEnv("WORKER_API_BEARER_TOKEN", DefaultWorkerAPIBearerToken),
		WorkerAPITargetURL:              getEnv("WORKER_API_TARGET_URL", ""),
		WorkerInternalAgentToken:        getEnv("WORKER_INTERNAL_AGENT_TOKEN", ""),
		MCPSandboxAgentBearerToken:      getEnv("MCP_SANDBOX_AGENT_BEARER_TOKEN", ""),
		MCPPAAgentBearerToken:           getEnv("MCP_PA_AGENT_BEARER_TOKEN", ""),
		BootstrapAdminPassword:          getEnv("BOOTSTRAP_ADMIN_PASSWORD", DefaultBootstrapAdminPassword),
		DevMode:                         getBoolEnv(envOrchestratorDevMode, true),
		RateLimitPerMinute:              getIntEnv("RATE_LIMIT_PER_MINUTE", 60),
		InferenceURL:                    getEnv("OLLAMA_BASE_URL", getEnv("INFERENCE_URL", "")),
		InferenceModel:                  getEnv("INFERENCE_MODEL", "qwen3.5:0.8b"),
		PMAEnabled:                      getBoolEnv("PMA_ENABLED", true),
		PMABinaryPath:                   getEnv("PMA_BINARY", "cynode-pma"),
		PMAListenAddr:                   getEnv("PMA_LISTEN_ADDR", ":8090"),
		PMAInstructionsRoot:             getEnv("PMA_INSTRUCTIONS_ROOT", ""),
		PMABaseURL:                      getEnv("PMA_BASE_URL", ""),
		PMAServiceID:                    getEnv("PMA_SERVICE_ID", "pma-main"),
		PMAImage:                        getEnv("PMA_IMAGE", "ghcr.io/cypher0n3/cynode-pma:latest"),
		PMAHostNodeSlug:                 getEnv("PMA_HOST_NODE_SLUG", ""),
		PMAPreferHostLabel:              getEnv("PMA_PREFER_HOST_LABEL", "orchestrator_host"),
		WorkflowRunnerBearerToken:       getEnv("WORKFLOW_RUNNER_BEARER_TOKEN", ""),
		ArtifactsS3Endpoint:             getEnv("ARTIFACTS_S3_ENDPOINT", ""),
		ArtifactsS3Region:               getEnv("ARTIFACTS_S3_REGION", "us-east-1"),
		ArtifactsS3AccessKey:            getEnv("ARTIFACTS_S3_ACCESS_KEY", ""),
		ArtifactsS3SecretKey:            getEnv("ARTIFACTS_S3_SECRET_KEY", ""),
		ArtifactsS3Bucket:               getEnv("ARTIFACTS_S3_BUCKET", "cynodeai-artifacts"),
		ArtifactHashInlineMaxBytes:      getInt64Env("ARTIFACT_HASH_INLINE_MAX_BYTES", 1024*1024),
		ArtifactHashBackfillEnabled:     getBoolEnv("ARTIFACT_HASH_BACKFILL_ENABLED", false),
		ArtifactHashBackfillInterval:    getDurationEnv("ARTIFACT_HASH_BACKFILL_INTERVAL", 10*time.Minute),
		ArtifactStaleCleanupEnabled:     getBoolEnv("ARTIFACT_STALE_CLEANUP_ENABLED", false),
		ArtifactStaleCleanupInterval:    getDurationEnv("ARTIFACT_STALE_CLEANUP_INTERVAL", time.Hour),
		ArtifactStaleCleanupMaxAgeHours: getIntEnv("ARTIFACT_STALE_CLEANUP_MAX_AGE_HOURS", 0),
		NATSClientURL:                   getEnv("NATS_CLIENT_URL", getEnv("NATS_URL", "nats://127.0.0.1:4222")),
		NATSServerURL:                   getEnv("NATS_SERVER_URL", ""),
		NATSWebSocketURL:                getEnv("NATS_WEBSOCKET_URL", "ws://127.0.0.1:8223/nats"),
		NATSAccountSeed:                 getEnv("NATS_ACCOUNT_SEED", ""),
		NATSAccountSigningSeed:          getEnv("NATS_ACCOUNT_SIGNING_SEED", ""),
	}
	return cfg
}

// NATSDialURL returns the broker address for server-side NATS connections (gateway, control-plane).
func (c *OrchestratorConfig) NATSDialURL() string {
	if c == nil {
		return ""
	}
	if s := strings.TrimSpace(c.NATSServerURL); s != "" {
		return s
	}
	return c.NATSClientURL
}

// ResolveNATSSeeds fills NATS account/signing seeds in DevMode when unset, using LoadDevSeeds (env, file, or generated).
func ResolveNATSSeeds(cfg *OrchestratorConfig) error {
	if cfg == nil || !cfg.DevMode {
		return nil
	}
	if cfg.NATSAccountSeed != "" && cfg.NATSAccountSigningSeed != "" {
		return nil
	}
	s, err := natsjwt.LoadDevSeeds()
	if err != nil {
		return fmt.Errorf("nats dev seeds: %w", err)
	}
	if cfg.NATSAccountSeed == "" {
		cfg.NATSAccountSeed = s.CynodeAccount
	}
	if cfg.NATSAccountSigningSeed == "" {
		cfg.NATSAccountSigningSeed = s.CynodeSigning
	}
	return nil
}

// ValidateSecrets returns an error when DevMode is false and any of JWT, node PSK, worker API bearer,
// or bootstrap admin password still equals its documented dev default. When DevMode is true, defaults are allowed.
func ValidateSecrets(cfg *OrchestratorConfig) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if cfg.DevMode {
		return nil
	}
	var bad []string
	if cfg.JWTSecret == DefaultJWTSecret {
		bad = append(bad, "JWT_SECRET")
	}
	if cfg.NodeRegistrationPSK == DefaultNodeRegistrationPSK {
		bad = append(bad, "NODE_REGISTRATION_PSK")
	}
	if cfg.WorkerAPIBearerToken == DefaultWorkerAPIBearerToken {
		bad = append(bad, "WORKER_API_BEARER_TOKEN")
	}
	if cfg.BootstrapAdminPassword == DefaultBootstrapAdminPassword {
		bad = append(bad, "BOOTSTRAP_ADMIN_PASSWORD")
	}
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf("insecure default secrets not allowed when %s=false: %s", envOrchestratorDevMode, strings.Join(bad, ", "))
}

// LoadNodeConfig loads node configuration from environment.
func LoadNodeConfig() *NodeConfig {
	return &NodeConfig{
		OrchestratorURL:       getEnv("ORCHESTRATOR_URL", "http://localhost:12082"),
		RegistrationPSK:       getEnv("NODE_REGISTRATION_PSK", DefaultNodeRegistrationPSK),
		NodeSlug:              getEnv("NODE_SLUG", "node-01"),
		NodeName:              getEnv("NODE_NAME", "Default Node"),
		ListenAddr:            getEnv("NODE_LISTEN_ADDR", ":12090"),
		ReadTimeout:           getDurationEnv("NODE_READ_TIMEOUT", 30*time.Second),
		WriteTimeout:          getDurationEnv("NODE_WRITE_TIMEOUT", 300*time.Second),
		IdleTimeout:           getDurationEnv("NODE_IDLE_TIMEOUT", 120*time.Second),
		MaxHeaderBytes:        getIntEnv("NODE_MAX_HEADER_BYTES", 1<<20),
		ContainerRuntime:      getEnv("CONTAINER_RUNTIME", "podman"),
		DefaultTimeoutSeconds: getIntEnv("DEFAULT_TIMEOUT_SECONDS", 300),
		MaxOutputBytes:        getIntEnv("MAX_OUTPUT_BYTES", 1<<20),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getIntEnv(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getInt64Env(key string, defaultVal int64) int64 {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func getBoolEnv(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		switch val {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return defaultVal
}
