package workerapiserver

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
)

const (
	internalOrchestratorProxyMaxBody = 4 << 20
	toolsCallPath                    = "/v1/mcp/tools/call"
	internalOrchestratorHTTPTimeout  = 300 * time.Second
)

var internalOrchestratorHTTPClient = &http.Client{Timeout: internalOrchestratorHTTPTimeout}

// deriveMCPGatewayBaseURL returns the orchestrator base URL for MCP tool calls. MCP is served by
// the control plane at POST /v1/mcp/tools/call (same process and database as node registration).
// Override with ORCHESTRATOR_MCP_GATEWAY_BASE_URL when needed.
func deriveMCPGatewayBaseURL(controlPlane string) string {
	return strings.TrimRight(strings.TrimSpace(controlPlane), "/")
}

func registerInternalOrchestratorProxyHandlers(mux *http.ServeMux, cfg embedInternalProxyConfig) {
	if mux == nil {
		return
	}
	mux.HandleFunc("POST /v1/worker/internal/orchestrator/mcp:call", func(w http.ResponseWriter, r *http.Request) {
		handleInternalOrchestratorMCPCall(w, r, cfg)
	})
	mux.HandleFunc("POST /v1/worker/internal/orchestrator/agent:ready", func(w http.ResponseWriter, r *http.Request) {
		handleInternalOrchestratorAgentReady(w, r, cfg)
	})
}

func handleInternalOrchestratorMCPCall(w http.ResponseWriter, r *http.Request, cfg embedInternalProxyConfig) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	upstreamBase := strings.TrimSpace(cfg.MCPGatewayBaseURL)
	if upstreamBase == "" {
		embedWriteProblem(w, http.StatusServiceUnavailable, problem.TypeInternal, "Service Unavailable", "orchestrator MCP tools URL not configured (set ORCHESTRATOR_MCP_GATEWAY_BASE_URL or ORCHESTRATOR_INTERNAL_PROXY_BASE_URL / ORCHESTRATOR_URL)")
		return
	}
	handleInternalOrchestratorProxyForward(w, r, cfg, upstreamBase, toolsCallPath)
}

func handleInternalOrchestratorAgentReady(w http.ResponseWriter, r *http.Request, cfg embedInternalProxyConfig) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	upstreamBase := strings.TrimSpace(cfg.UpstreamBaseURL)
	if upstreamBase == "" {
		embedWriteProblem(w, http.StatusServiceUnavailable, problem.TypeInternal, "Service Unavailable", "orchestrator base URL not configured")
		return
	}
	handleInternalOrchestratorProxyForward(w, r, cfg, upstreamBase, "")
}

func handleInternalOrchestratorProxyForward(w http.ResponseWriter, r *http.Request, cfg embedInternalProxyConfig, upstreamBase string, defaultPath string) {
	ctx := r.Context()
	serviceID, _ := ctx.Value(CallerServiceIDContextKey).(string)
	if strings.TrimSpace(serviceID) == "" {
		embedWriteProblem(w, http.StatusForbidden, "https://cynode.ai/problems/managed-agent-identity-unresolved", "Forbidden", "managed agent identity could not be resolved from the transport binding")
		return
	}
	if cfg.SecureStore == nil {
		embedWriteProblem(w, http.StatusServiceUnavailable, "https://cynode.ai/problems/managed-agent-token-unavailable", "Service Unavailable", "secure store not available")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, internalOrchestratorProxyMaxBody)
	var proxyReq managedProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&proxyReq); err != nil {
		embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "invalid proxy request body")
		return
	}
	bodyBytes, err := base64.StdEncoding.DecodeString(proxyReq.BodyB64)
	if err != nil {
		embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "invalid body_b64")
		return
	}
	path := strings.TrimSpace(proxyReq.Path)
	if path == "" {
		path = defaultPath
	}
	if path == "" {
		embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "path is required")
		return
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	method := strings.TrimSpace(proxyReq.Method)
	if method == "" {
		method = http.MethodPost
	}
	rec, err := cfg.SecureStore.GetAgentToken(serviceID)
	if err != nil {
		if errors.Is(err, securestore.ErrTokenExpired) {
			embedWriteProblem(w, http.StatusServiceUnavailable, "https://cynode.ai/problems/managed-agent-token-expired", "Service Unavailable", "managed agent token expired")
			return
		}
		embedWriteProblem(w, http.StatusServiceUnavailable, "https://cynode.ai/problems/managed-agent-token-unavailable", "Service Unavailable", "managed agent token not available")
		return
	}
	if rec == nil || strings.TrimSpace(rec.Token) == "" {
		embedWriteProblem(w, http.StatusServiceUnavailable, "https://cynode.ai/problems/managed-agent-token-unavailable", "Service Unavailable", "managed agent token not available")
		return
	}
	upstreamURL := strings.TrimRight(upstreamBase, "/") + path
	var bodyReader io.Reader
	if method != http.MethodGet && method != http.MethodHead && len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}
	upReq, err := http.NewRequestWithContext(ctx, method, upstreamURL, bodyReader)
	if err != nil {
		embedWriteProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "failed to build upstream request")
		return
	}
	for k, v := range proxyReq.Headers {
		for _, vv := range v {
			upReq.Header.Add(k, vv)
		}
	}
	if len(bodyBytes) > 0 && upReq.Header.Get("Content-Type") == "" {
		upReq.Header.Set("Content-Type", "application/json")
	}
	upReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(rec.Token))
	resp, err := internalOrchestratorHTTPClient.Do(upReq)
	if err != nil {
		embedWriteProblem(w, http.StatusBadGateway, problem.TypeInternal, "Bad Gateway", fmt.Sprintf("upstream request failed: %v", err))
		return
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		embedWriteProblem(w, http.StatusBadGateway, problem.TypeInternal, "Bad Gateway", "failed to read upstream response")
		return
	}
	proxyResp := managedProxyResponseFromHTTP(resp.StatusCode, resp.Header, respBody)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(proxyResp)
}
