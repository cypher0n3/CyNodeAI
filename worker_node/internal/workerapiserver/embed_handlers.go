// embed_handlers provides proxy config loading and mux building for RunEmbedded.
package workerapiserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/worker_node/internal/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
)

// managedProxyRequest is the JSON body for the managed-service proxy (orchestrator -> worker -> PMA).
// Must match orchestrator/internal/pmaclient/client.go.
type managedProxyRequest struct {
	Version int                 `json:"version"`
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers,omitempty"`
	BodyB64 string              `json:"body_b64,omitempty"`
}

// managedProxyResponse is the JSON response from the managed-service proxy.
type managedProxyResponse struct {
	Version int                 `json:"version"`
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers,omitempty"`
	BodyB64 string              `json:"body_b64,omitempty"`
}

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
	publicMux.HandleFunc("POST /v1/worker/managed-services/{id}/proxy:http", managedServiceProxyHTTPHandler(bearerToken, proxyCfg.InternalProxy.SocketByService, logger))
	internalMux = http.NewServeMux()
	// Internal proxy routes would be added when moving handleInternalOrchestratorProxy
	return publicMux, internalMux
}

func managedServiceProxyHTTPHandler(bearerToken string, socketByService map[string]string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if bearerToken != "" {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || strings.TrimSpace(auth[7:]) != bearerToken {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
		}
		serviceID := r.PathValue("id")
		if serviceID == "" {
			http.Error(w, "missing service id", http.StatusBadRequest)
			return
		}
		proxySock, ok := socketByService[serviceID]
		if !ok || proxySock == "" {
			if logger != nil {
				logger.Warn("managed-service proxy: no socket for service", "service_id", serviceID)
			}
			http.Error(w, "service not found", http.StatusNotFound)
			return
		}
		// PMA listens on service.sock; SocketByService has proxy.sock (worker internal listener). Use service.sock for upstream.
		socketPath := filepath.Join(filepath.Dir(proxySock), "service.sock")
		var proxyReq managedProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&proxyReq); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		body, err := base64.StdEncoding.DecodeString(proxyReq.BodyB64)
		if err != nil {
			http.Error(w, "invalid body_b64", http.StatusBadRequest)
			return
		}
		path := proxyReq.Path
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if network == "tcp" && (addr == "proxy:80" || addr == "proxy") {
					return net.Dial("unix", socketPath)
				}
				return nil, fmt.Errorf("managed proxy only supports unix socket, got %q %q", network, addr)
			},
		}
		client := &http.Client{Transport: transport, Timeout: 120 * time.Second}
		upstreamURL := "http://proxy" + path
		upstreamReq, err := http.NewRequestWithContext(r.Context(), proxyReq.Method, upstreamURL, nil)
		if err != nil {
			if logger != nil {
				logger.Error("managed-service proxy: new request", "error", err)
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if proxyReq.Method != http.MethodGet && proxyReq.Method != http.MethodHead && len(body) > 0 {
			upstreamReq.Body = io.NopCloser(bytes.NewReader(body))
			upstreamReq.ContentLength = int64(len(body))
		}
		for k, v := range proxyReq.Headers {
			for _, vv := range v {
				upstreamReq.Header.Add(k, vv)
			}
		}
		upstreamReq.Host = "proxy"
		resp, err := client.Do(upstreamReq)
		if err != nil {
			if logger != nil {
				logger.Error("managed-service proxy: upstream request", "service_id", serviceID, "error", err)
			}
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			if logger != nil {
				logger.Error("managed-service proxy: read upstream body", "error", err)
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		proxyResp := managedProxyResponse{
			Version: 1,
			Status:  resp.StatusCode,
			Headers: map[string][]string{},
			BodyB64: base64.StdEncoding.EncodeToString(respBody),
		}
		for k, v := range resp.Header {
			if len(v) > 0 {
				proxyResp.Headers[k] = v
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(proxyResp)
	}
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
