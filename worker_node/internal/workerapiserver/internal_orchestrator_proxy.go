package workerapiserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
)

const (
	toolsCallPath                   = "/v1/mcp/tools/call"
	internalOrchestratorHTTPTimeout = 300 * time.Second
)

var internalOrchestratorHTTPClient = &http.Client{Timeout: internalOrchestratorHTTPTimeout}

// deriveMCPToolsBaseURL returns the orchestrator base URL for MCP tool calls. MCP is served by
// the control plane at POST /v1/mcp/tools/call (same process and database as node registration).
func deriveMCPToolsBaseURL(controlPlane string) string {
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
	upstreamBase := strings.TrimSpace(cfg.MCPToolsBaseURL)
	if upstreamBase == "" {
		embedWriteProblem(w, http.StatusServiceUnavailable, problem.TypeInternal, "Service Unavailable", "orchestrator MCP tools URL not configured (set ORCHESTRATOR_MCP_TOOLS_BASE_URL or ORCHESTRATOR_INTERNAL_PROXY_BASE_URL / ORCHESTRATOR_URL)")
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

func decodeManagedProxyRequest(w http.ResponseWriter, r *http.Request, defaultPath string) (proxyReq managedProxyRequest, bodyBytes []byte, path string, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, httplimits.DefaultMaxAPIRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&proxyReq); err != nil {
		embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "invalid proxy request body")
		return managedProxyRequest{}, nil, "", false
	}
	var err error
	bodyBytes, err = base64.StdEncoding.DecodeString(proxyReq.BodyB64)
	if err != nil {
		embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "invalid body_b64")
		return managedProxyRequest{}, nil, "", false
	}
	path = strings.TrimSpace(proxyReq.Path)
	if path == "" {
		path = defaultPath
	}
	if path == "" {
		embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "path is required")
		return managedProxyRequest{}, nil, "", false
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return proxyReq, bodyBytes, path, true
}

func getAgentTokenForProxy(cfg embedInternalProxyConfig, serviceID string, w http.ResponseWriter) (*securestore.AgentTokenRecord, bool) {
	rec, err := cfg.SecureStore.GetAgentToken(serviceID)
	if err != nil {
		if errors.Is(err, securestore.ErrTokenExpired) {
			embedWriteProblem(w, http.StatusServiceUnavailable, "https://cynode.ai/problems/managed-agent-token-expired", "Service Unavailable", "managed agent token expired")
			return nil, false
		}
		embedWriteProblem(w, http.StatusServiceUnavailable, "https://cynode.ai/problems/managed-agent-token-unavailable", "Service Unavailable", "managed agent token not available")
		return nil, false
	}
	if rec == nil || strings.TrimSpace(rec.Token) == "" {
		embedWriteProblem(w, http.StatusServiceUnavailable, "https://cynode.ai/problems/managed-agent-token-unavailable", "Service Unavailable", "managed agent token not available")
		return nil, false
	}
	return rec, true
}

const internalProxyAuditSource = "worker_internal_orchestrator_proxy"

func emitInternalOrchestratorProxyAudit(ctx context.Context, cfg embedInternalProxyConfig, serviceID, destination, method, path string, statusCode int, start time.Time) {
	if cfg.ProxyAuditLogger == nil {
		return
	}
	cfg.ProxyAuditLogger.LogAttrs(ctx, slog.LevelInfo, "internal_orchestrator_proxy_audit",
		slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)),
		slog.String("source", internalProxyAuditSource),
		slog.String("destination", destination),
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status_code", statusCode),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		slog.String("service_id", serviceID),
	)
}

func writeManagedProxyJSONFromUpstream(w http.ResponseWriter, resp *http.Response) bool {
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, httplimits.DefaultMaxHTTPResponseBytes))
	if err != nil {
		embedWriteProblem(w, http.StatusBadGateway, problem.TypeInternal, "Bad Gateway", "failed to read upstream response")
		return false
	}
	proxyResp := managedProxyResponseFromHTTP(resp.StatusCode, resp.Header, respBody)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(proxyResp)
	return true
}

func handleInternalOrchestratorProxyForward(w http.ResponseWriter, r *http.Request, cfg embedInternalProxyConfig, upstreamBase, defaultPath string) {
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
	proxyReq, bodyBytes, path, ok := decodeManagedProxyRequest(w, r, defaultPath)
	if !ok {
		return
	}
	method := strings.TrimSpace(proxyReq.Method)
	if method == "" {
		method = http.MethodPost
	}
	rec, ok := getAgentTokenForProxy(cfg, serviceID, w)
	if !ok {
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
	auditStart := time.Now()
	resp, err := internalOrchestratorHTTPClient.Do(upReq)
	if err != nil {
		emitInternalOrchestratorProxyAudit(ctx, cfg, serviceID, upstreamURL, method, path, 0, auditStart)
		embedWriteProblem(w, http.StatusBadGateway, problem.TypeInternal, "Bad Gateway", fmt.Sprintf("upstream request failed: %v", err))
		return
	}
	defer func() { _ = resp.Body.Close() }()
	emitInternalOrchestratorProxyAudit(ctx, cfg, serviceID, upstreamURL, method, path, resp.StatusCode, auditStart)
	writeManagedProxyJSONFromUpstream(w, resp)
}
