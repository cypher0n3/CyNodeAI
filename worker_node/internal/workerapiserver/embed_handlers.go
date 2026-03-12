// embed_handlers provides proxy config loading and mux building for RunEmbedded.
package workerapiserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/worker_node/internal/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
)

type embedManagedServiceTarget struct {
	ServiceType string `json:"service_type"`
	BaseURL     string `json:"base_url"`
}

type embedInternalProxyConfig struct {
	UpstreamBaseURL string
	SocketByService map[string]string
	SecureStore     *securestore.Store
}

type embedProxyConfig struct {
	ManagedServiceTargets map[string]embedManagedServiceTarget
	InternalProxy         embedInternalProxyConfig
}

func loadProxyConfigFromEnv(stateDir string, logger *slog.Logger) (embedProxyConfig, error) {
	out := embedProxyConfig{
		ManagedServiceTargets: map[string]embedManagedServiceTarget{},
		InternalProxy: embedInternalProxyConfig{
			UpstreamBaseURL: strings.TrimSpace(os.Getenv("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL")),
			SocketByService: map[string]string{},
			SecureStore:     nil,
		},
	}
	if nodeCfgRaw := strings.TrimSpace(os.Getenv("WORKER_NODE_CONFIG_JSON")); nodeCfgRaw != "" {
		applyNodeConfigToEmbedProxyConfig(&out, stateDir, nodeCfgRaw, logger)
	}
	if len(out.ManagedServiceTargets) == 0 {
		out.ManagedServiceTargets = loadManagedServiceTargetsFromEnvEmbed(logger)
	}
	if out.InternalProxy.UpstreamBaseURL == "" {
		out.InternalProxy.UpstreamBaseURL = strings.TrimSpace(os.Getenv("ORCHESTRATOR_URL"))
	}
	if store, _, err := securestore.Open(stateDir); err == nil {
		out.InternalProxy.SecureStore = store
	} else if logger != nil {
		logger.Error("secure store unavailable; internal orchestrator proxy will fail closed", "error", err)
	}
	return out, nil
}

func applyNodeConfigToEmbedProxyConfig(out *embedProxyConfig, stateDir, nodeCfgRaw string, logger *slog.Logger) {
	var nodeCfg nodepayloads.NodeConfigurationPayload
	if err := json.Unmarshal([]byte(nodeCfgRaw), &nodeCfg); err != nil {
		if logger != nil {
			logger.Warn("invalid WORKER_NODE_CONFIG_JSON; falling back to env-only proxy config", "error", err)
		}
		return
	}
	if nodeCfg.ManagedServices != nil {
		applyManagedServicesSocketByService(out, stateDir, nodeCfg.ManagedServices.Services)
	}
	if out.InternalProxy.UpstreamBaseURL == "" && nodeCfg.Orchestrator.BaseURL != "" {
		out.InternalProxy.UpstreamBaseURL = strings.TrimSpace(nodeCfg.Orchestrator.BaseURL)
	}
}

func applyManagedServicesSocketByService(out *embedProxyConfig, stateDir string, services []nodepayloads.ConfigManagedService) {
	for i := range services {
		svc := &services[i]
		serviceID := strings.TrimSpace(svc.ServiceID)
		if serviceID == "" || svc.Orchestrator == nil {
			continue
		}
		if path, ok := managedAgentProxySocketPathEmbed(stateDir, serviceID); ok {
			out.InternalProxy.SocketByService[serviceID] = path
		}
	}
}

func managedAgentProxySocketPathEmbed(stateDir, serviceID string) (string, bool) {
	serviceID = strings.TrimSpace(serviceID)
	if serviceID == "" || strings.Contains(serviceID, "/") || strings.Contains(serviceID, "\\") || strings.Contains(serviceID, "..") {
		return "", false
	}
	base := filepath.Join(stateDir, "run", "managed_agent_proxy", serviceID)
	return filepath.Join(base, "proxy.sock"), true
}

func loadManagedServiceTargetsFromEnvEmbed(logger *slog.Logger) map[string]embedManagedServiceTarget {
	raw := strings.TrimSpace(os.Getenv("WORKER_MANAGED_SERVICE_TARGETS_JSON"))
	if raw == "" {
		return map[string]embedManagedServiceTarget{}
	}
	targets := make(map[string]embedManagedServiceTarget)
	if err := json.Unmarshal([]byte(raw), &targets); err == nil {
		return targets
	}
	var simple map[string]string
	if err := json.Unmarshal([]byte(raw), &simple); err != nil {
		if logger != nil {
			logger.Warn("invalid WORKER_MANAGED_SERVICE_TARGETS_JSON; managed service proxy disabled", "error", err)
		}
		return map[string]embedManagedServiceTarget{}
	}
	for serviceID, baseURL := range simple {
		targets[serviceID] = embedManagedServiceTarget{ServiceType: "unknown", BaseURL: baseURL}
	}
	return targets
}

func buildMuxesFromEmbedConfig(
	exec *executor.Executor,
	bearerToken, workspaceRoot string,
	telemetryStore *telemetry.Store,
	logger *slog.Logger,
	proxyCfg embedProxyConfig,
) (publicMux, internalMux *http.ServeMux) {
	publicMux = http.NewServeMux()
	publicMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	publicMux.HandleFunc("GET /readyz", embedReadyzHandler(exec))
	// Full handler set would be added here; for minimal embed we only have healthz and readyz.
	// TODO: add handleRunJob, telemetry, managed-service proxy when moving handlers from main
	internalMux = http.NewServeMux()
	// Internal proxy routes would be added when moving handleInternalOrchestratorProxy
	return publicMux, internalMux
}

func embedReadyzHandler(exec *executor.Executor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ready, reason := exec.Ready(r.Context())
		if ready {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(reason))
	}
}
