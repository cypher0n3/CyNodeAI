// Package config provides configuration for CyNodeAI services.
// See docs/tech_specs/orchestrator_bootstrap.md for bootstrap details.
package config

import (
	"os"
	"strconv"
	"time"
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

	// Bootstrap
	BootstrapAdminPassword string

	// Rate Limiting
	RateLimitPerMinute int

	// Inference (PM model): when set, prompt-mode tasks call this URL directly so prompt→model MUST work (MVP Phase 1).
	InferenceURL   string // e.g. http://localhost:11434 or http://ollama:11434
	InferenceModel string // e.g. tinyllama; default tinyllama
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
	return &OrchestratorConfig{
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cynodeai?sslmode=disable"),
		ListenAddr:             getEnv("LISTEN_ADDR", ":8080"),
		ReadTimeout:            getDurationEnv("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:           getDurationEnv("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:            getDurationEnv("IDLE_TIMEOUT", 120*time.Second),
		MaxHeaderBytes:         getIntEnv("MAX_HEADER_BYTES", 1<<20),
		MaxRequestBodyMB:       getIntEnv("MAX_REQUEST_BODY_MB", 10),
		JWTSecret:              getEnv("JWT_SECRET", "change-me-in-production"),
		JWTAccessDuration:      getDurationEnv("JWT_ACCESS_DURATION", 15*time.Minute),
		JWTRefreshDuration:     getDurationEnv("JWT_REFRESH_DURATION", 7*24*time.Hour),
		JWTNodeDuration:        getDurationEnv("JWT_NODE_DURATION", 24*time.Hour),
		NodeRegistrationPSK:    getEnv("NODE_REGISTRATION_PSK", "default-psk-change-me"),
		OrchestratorPublicURL:  getEnv("ORCHESTRATOR_PUBLIC_URL", "http://localhost:8082"),
		WorkerAPIBearerToken:   getEnv("WORKER_API_BEARER_TOKEN", "phase1-static-token"),
		WorkerAPITargetURL:     getEnv("WORKER_API_TARGET_URL", ""),
		BootstrapAdminPassword: getEnv("BOOTSTRAP_ADMIN_PASSWORD", "admin123"),
		RateLimitPerMinute:     getIntEnv("RATE_LIMIT_PER_MINUTE", 60),
		InferenceURL:           getEnv("OLLAMA_BASE_URL", getEnv("INFERENCE_URL", "")),
		InferenceModel:         getEnv("INFERENCE_MODEL", "tinyllama"),
	}
}

// LoadNodeConfig loads node configuration from environment.
func LoadNodeConfig() *NodeConfig {
	return &NodeConfig{
		OrchestratorURL:       getEnv("ORCHESTRATOR_URL", "http://localhost:8080"),
		RegistrationPSK:       getEnv("NODE_REGISTRATION_PSK", "default-psk-change-me"),
		NodeSlug:              getEnv("NODE_SLUG", "node-01"),
		NodeName:              getEnv("NODE_NAME", "Default Node"),
		ListenAddr:            getEnv("NODE_LISTEN_ADDR", ":8081"),
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

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}
