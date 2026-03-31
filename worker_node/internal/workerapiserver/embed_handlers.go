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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
	"github.com/cypher0n3/cynodeai/go_shared_libs/secretutil"
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
	// MCPToolsBaseURL is the orchestrator base URL for POST /v1/mcp/tools/call (served by the
	// control plane). Precedence: ORCHESTRATOR_MCP_TOOLS_BASE_URL, then deprecated alias
	// ORCHESTRATOR_MCP_GATEWAY_BASE_URL, then ORCHESTRATOR_INTERNAL_PROXY_BASE_URL / ORCHESTRATOR_URL
	// (same origin as the control plane in the standard compose layout).
	MCPToolsBaseURL string
	SocketByService map[string]string
	SecureStore     *securestore.Store
	// ProxyAuditLogger receives structured JSON audit lines for each proxied orchestrator request (REQ-WORKER-0163).
	// When nil at mux build time, buildMuxesFromEmbedConfig sets it from the embed server logger.
	ProxyAuditLogger *slog.Logger
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
	// MCP tools base URL (orchestrator control plane): explicit env wins, then derive from orchestrator URL.
	out.InternalProxy.MCPToolsBaseURL = strings.TrimSpace(os.Getenv("ORCHESTRATOR_MCP_TOOLS_BASE_URL"))
	if out.InternalProxy.MCPToolsBaseURL == "" {
		out.InternalProxy.MCPToolsBaseURL = strings.TrimSpace(os.Getenv("ORCHESTRATOR_MCP_GATEWAY_BASE_URL"))
	}
	if out.InternalProxy.MCPToolsBaseURL == "" {
		out.InternalProxy.MCPToolsBaseURL = deriveMCPToolsBaseURL(out.InternalProxy.UpstreamBaseURL)
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

// embedRunner is satisfied by *executor.Executor and by test doubles for jobs:run/readyz.
type embedRunner interface {
	RunJob(ctx context.Context, req *workerapi.RunJobRequest, workspaceDir string) (*workerapi.RunJobResponse, error)
	Ready(ctx context.Context) (bool, string)
}

func buildMuxesFromEmbedConfig(
	runner embedRunner,
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
	publicMux.HandleFunc("GET /readyz", embedReadyzHandler(runner))
	publicMux.HandleFunc("POST /v1/worker/jobs:run", embedJobsRunHandler(runner, workspaceRoot, bearerToken))
	publicMux.HandleFunc("POST /v1/worker/managed-services/{id}/proxy:http", managedServiceProxyHTTPHandler(bearerToken, proxyCfg.InternalProxy.SocketByService, logger))
	registerEmbedTelemetryHandlers(publicMux, bearerToken, telemetryStore, logger)
	internalMux = http.NewServeMux()
	ip := proxyCfg.InternalProxy
	if ip.ProxyAuditLogger == nil {
		ip.ProxyAuditLogger = logger
	}
	if ip.ProxyAuditLogger == nil {
		ip.ProxyAuditLogger = slog.Default()
	}
	registerInternalOrchestratorProxyHandlers(internalMux, ip)
	return publicMux, internalMux
}

func managedServiceProxyHTTPHandler(bearerToken string, socketByService map[string]string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		httplimits.WrapRequestBody(w, r, httplimits.DefaultMaxAPIRequestBodyBytes)
		proxyReq, body, socketPath, errCode, err := validateManagedProxyRequest(r, bearerToken, socketByService, logger)
		if err != nil {
			if errCode == http.StatusUnauthorized {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(errCode)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			http.Error(w, err.Error(), errCode)
			return
		}
		if wantsStream(body) {
			errCode, err := doManagedProxyUpstreamStream(r.Context(), proxyReq, body, socketPath, r.PathValue("id"), w, logger)
			if err != nil {
				http.Error(w, err.Error(), errCode)
				return
			}
			return
		}
		proxyResp, errCode, err := doManagedProxyUpstream(r.Context(), proxyReq, body, socketPath, r.PathValue("id"), logger)
		if err != nil {
			http.Error(w, err.Error(), errCode)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(proxyResp)
	}
}

// wantsStream returns true if the decoded proxy body is JSON with "stream": true (e.g. PMA chat completion stream).
func wantsStream(body []byte) bool {
	var v struct {
		Stream bool `json:"stream"`
	}
	return json.Unmarshal(body, &v) == nil && v.Stream
}

// doManagedProxyUpstreamStream forwards the request to the managed service and streams the upstream response body back.
// Used when the request body has "stream": true so the client receives token-by-token output.
func doManagedProxyUpstreamStream(ctx context.Context, proxyReq *managedProxyRequest, body []byte, socketPath, serviceID string, w http.ResponseWriter, logger *slog.Logger) (errCode int, err error) {
	path := proxyReq.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	upstreamReq, err := buildManagedProxyUpstreamRequest(ctx, proxyReq, body, path)
	if err != nil {
		if logger != nil {
			logger.Error("managed-service proxy stream: new request", "error", err)
		}
		return http.StatusInternalServerError, fmt.Errorf("internal error")
	}
	resp, err := managedProxyHTTPClientStream(socketPath).Do(upstreamReq)
	if err != nil {
		if logger != nil {
			logger.Error("managed-service proxy stream: upstream request", "service_id", serviceID, "error", err)
		}
		return http.StatusBadGateway, fmt.Errorf("upstream error")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("upstream returned %s", resp.Status)
	}
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if resp.Header.Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/x-ndjson")
	}
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	_, err = io.Copy(w, resp.Body)
	if err != nil && logger != nil {
		logger.Warn("managed-service proxy stream: copy body", "error", err)
	}
	return 0, nil
}

func validateManagedProxyRequest(r *http.Request, bearerToken string, socketByService map[string]string, logger *slog.Logger) (req *managedProxyRequest, body []byte, socketPath string, errCode int, err error) {
	if bearerToken != "" && !embedBearerOK(r.Header.Get("Authorization"), bearerToken) {
		return nil, nil, "", http.StatusUnauthorized, fmt.Errorf("unauthorized")
	}
	serviceID := r.PathValue("id")
	if serviceID == "" {
		return nil, nil, "", http.StatusBadRequest, fmt.Errorf("missing service id")
	}
	proxySock, ok := socketByService[serviceID]
	if !ok || proxySock == "" {
		if logger != nil {
			logger.Warn("managed-service proxy: no socket for service", "service_id", serviceID)
		}
		return nil, nil, "", http.StatusNotFound, fmt.Errorf("service not found")
	}
	socketPath = filepath.Join(filepath.Dir(proxySock), "service.sock")
	var proxyReq managedProxyRequest
	if err = json.NewDecoder(r.Body).Decode(&proxyReq); err != nil {
		return nil, nil, "", http.StatusBadRequest, fmt.Errorf("invalid request body")
	}
	body, err = base64.StdEncoding.DecodeString(proxyReq.BodyB64)
	if err != nil {
		return nil, nil, "", http.StatusBadRequest, fmt.Errorf("invalid body_b64")
	}
	return &proxyReq, body, socketPath, 0, nil
}

func doManagedProxyUpstream(ctx context.Context, proxyReq *managedProxyRequest, body []byte, socketPath, serviceID string, logger *slog.Logger) (managedProxyResponse, int, error) {
	path := proxyReq.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	client := managedProxyHTTPClient(socketPath)
	upstreamReq, err := buildManagedProxyUpstreamRequest(ctx, proxyReq, body, path)
	if err != nil {
		if logger != nil {
			logger.Error("managed-service proxy: new request", "error", err)
		}
		return managedProxyResponse{}, http.StatusInternalServerError, fmt.Errorf("internal error")
	}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		if logger != nil {
			logger.Error("managed-service proxy: upstream request", "service_id", serviceID, "error", err)
		}
		return managedProxyResponse{}, http.StatusBadGateway, fmt.Errorf("upstream error")
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, httplimits.DefaultMaxHTTPResponseBytes))
	if err != nil {
		if logger != nil {
			logger.Error("managed-service proxy: read upstream body", "error", err)
		}
		return managedProxyResponse{}, http.StatusInternalServerError, fmt.Errorf("internal error")
	}
	return managedProxyResponseFromHTTP(resp.StatusCode, resp.Header, respBody), 0, nil
}

func managedProxyTransport(socketPath string) *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if network == "tcp" && (addr == "proxy:80" || addr == "proxy") {
				return net.Dial("unix", socketPath)
			}
			return nil, fmt.Errorf("managed proxy only supports unix socket, got %q %q", network, addr)
		},
	}
}

// managedProxyHTTPClient is used for buffered upstream responses (non-streaming proxy calls).
func managedProxyHTTPClient(socketPath string) *http.Client {
	return &http.Client{Transport: managedProxyTransport(socketPath), Timeout: 300 * time.Second}
}

// managedProxyHTTPClientStream uses the same wall timeout as orchestrator PMA streaming (300s).
func managedProxyHTTPClientStream(socketPath string) *http.Client {
	return &http.Client{Transport: managedProxyTransport(socketPath), Timeout: 300 * time.Second}
}

func buildManagedProxyUpstreamRequest(ctx context.Context, proxyReq *managedProxyRequest, body []byte, path string) (*http.Request, error) {
	upstreamURL := "http://proxy" + path
	req, err := http.NewRequestWithContext(ctx, proxyReq.Method, upstreamURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	if proxyReq.Method != http.MethodGet && proxyReq.Method != http.MethodHead && len(body) > 0 {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
	}
	for k, v := range proxyReq.Headers {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}
	req.Host = "proxy"
	return req, nil
}

func managedProxyResponseFromHTTP(statusCode int, header http.Header, respBody []byte) managedProxyResponse {
	out := managedProxyResponse{
		Version: 1,
		Status:  statusCode,
		Headers: map[string][]string{},
		BodyB64: base64.StdEncoding.EncodeToString(respBody),
	}
	for k, v := range header {
		if len(v) > 0 {
			out.Headers[k] = v
		}
	}
	return out
}

func embedWriteProblem(w http.ResponseWriter, status int, typ, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem.Details{Type: typ, Title: title, Status: status, Detail: detail})
}

// embedBearerOK reports whether Authorization matches the expected bearer token (constant-time compare).
func embedBearerOK(authHeader, bearerToken string) bool {
	if bearerToken == "" {
		return true
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	got := strings.TrimSpace(authHeader[len("Bearer "):])
	return secretutil.TokenEquals(got, bearerToken)
}

func embedTelemetryAuth(w http.ResponseWriter, r *http.Request, bearerToken string) bool {
	if bearerToken != "" && !embedBearerOK(r.Header.Get("Authorization"), bearerToken) {
		embedWriteProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
		return false
	}
	return true
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

var kernelVersionPath = "/proc/sys/kernel/osrelease" // override in tests to exercise error path

func kernelVersionFromOS() string {
	b, err := os.ReadFile(kernelVersionPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func embedNodeInfoFromStore(ctx context.Context, store *telemetry.Store, logger *slog.Logger) (nodeSlug, buildVersion, platformOS, platformArch, kernelVersion string) {
	nodeSlug = firstNonEmpty(strings.TrimSpace(os.Getenv("NODE_SLUG")), "embedded-node")
	buildVersion = firstNonEmpty(strings.TrimSpace(os.Getenv("BUILD_VERSION")), "dev")
	platformOS, platformArch, kernelVersion = "linux", runtime.GOARCH, ""
	if store == nil {
		kernelVersion = firstNonEmpty(kernelVersionFromOS(), kernelVersion)
		return nodeSlug, buildVersion, platformOS, platformArch, kernelVersion
	}
	row, err := store.GetLatestNodeBoot(ctx)
	if err != nil && logger != nil {
		logger.Warn("telemetry GetLatestNodeBoot", "error", err)
	}
	if row != nil {
		nodeSlug = firstNonEmpty(row.NodeSlug, nodeSlug)
		buildVersion = firstNonEmpty(row.BuildVersion, buildVersion)
		platformOS = firstNonEmpty(row.PlatformOS, platformOS)
		platformArch = firstNonEmpty(row.PlatformArch, platformArch)
		kernelVersion = firstNonEmpty(row.KernelVersion, kernelVersion)
	}
	if kernelVersion == "" {
		kernelVersion = kernelVersionFromOS()
	}
	return nodeSlug, buildVersion, platformOS, platformArch, kernelVersion
}

func embedNodeInfoHandler(bearerToken string, store *telemetry.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !embedTelemetryAuth(w, r, bearerToken) {
			return
		}
		nodeSlug, buildVersion, platformOS, platformArch, kernelVersion := embedNodeInfoFromStore(r.Context(), store, logger)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"version": 1, "node_slug": nodeSlug,
			"build":    map[string]string{"build_version": buildVersion},
			"platform": map[string]string{"os": platformOS, "arch": platformArch, "kernel_version": kernelVersion},
		})
	}
}

func embedNodeStatsHandler(bearerToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !embedTelemetryAuth(w, r, bearerToken) {
			return
		}
		cores := runtime.NumCPU()
		if cores <= 0 {
			cores = 1
		}
		rt := strings.TrimSpace(os.Getenv("CONTAINER_RUNTIME"))
		if rt != "docker" && rt != "podman" {
			rt = "podman"
		}
		rtVersion := strings.TrimSpace(os.Getenv("CONTAINER_RUNTIME_VERSION"))
		if rtVersion == "" {
			rtVersion = "dev"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"version": 1, "captured_at": time.Now().UTC().Format(time.RFC3339),
			"cpu":               map[string]interface{}{"cores": cores, "load1": 0.0, "load5": 0.0, "load15": 0.0},
			"memory":            map[string]interface{}{"total_mb": 1024, "used_mb": 0, "free_mb": 1024},
			"disk":              map[string]interface{}{"state_dir_free_mb": 100, "state_dir_total_mb": 100},
			"container_runtime": map[string]string{"runtime": rt, "version": rtVersion},
		})
	}
}

func embedTelemetryContainersEmptyHandler(bearerToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !embedTelemetryAuth(w, r, bearerToken) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"version": 1, "containers": []interface{}{}})
	}
}

func embedTelemetryContainerNotFoundHandler(bearerToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !embedTelemetryAuth(w, r, bearerToken) {
			return
		}
		embedWriteProblem(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", "container not found")
	}
}

func embedTelemetryLogsEmptyHandler(bearerToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !embedTelemetryAuth(w, r, bearerToken) {
			return
		}
		q := r.URL.Query()
		if q.Get("source_kind") == "" && q.Get("container_id") == "" {
			embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "source_kind+source_name or source_kind=container+container_id required")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"version": 1, "events": []interface{}{},
			"truncated": map[string]interface{}{"limited_by": "none", "max_bytes": 1048576},
		})
	}
}

func embedTelemetryContainersHandler(bearerToken string, store *telemetry.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !embedTelemetryAuth(w, r, bearerToken) {
			return
		}
		q := r.URL.Query()
		limit := 100
		if l := q.Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		list, nextToken, err := store.ListContainers(r.Context(), q.Get("kind"), q.Get("status"), q.Get("task_id"), q.Get("job_id"), q.Get("page_token"), limit)
		if err != nil {
			embedWriteProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "")
			return
		}
		if list == nil {
			list = []telemetry.ContainerRow{}
		}
		resp := map[string]interface{}{"version": 1, "containers": list}
		if nextToken != "" {
			resp["next_page_token"] = nextToken
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func embedTelemetryContainerByIDHandler(bearerToken string, store *telemetry.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !embedTelemetryAuth(w, r, bearerToken) {
			return
		}
		containerID := strings.TrimPrefix(r.URL.Path, "/v1/worker/telemetry/containers/")
		if containerID == "" {
			embedWriteProblem(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", "container_id required")
			return
		}
		c, err := store.GetContainer(r.Context(), containerID)
		if err != nil {
			embedWriteProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "")
			return
		}
		if c == nil {
			embedWriteProblem(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", "container not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"version": 1, "container": c})
	}
}

func embedTelemetryLogsLimit(q string) int {
	limit := 1000
	if q == "" {
		return limit
	}
	if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 5000 {
		return n
	}
	return limit
}

func embedTelemetryLogsHandler(bearerToken string, store *telemetry.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !embedTelemetryAuth(w, r, bearerToken) {
			return
		}
		q := r.URL.Query()
		if q.Get("source_kind") == "" && q.Get("container_id") == "" {
			embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "source_kind+source_name or source_kind=container+container_id required")
			return
		}
		limit := embedTelemetryLogsLimit(q.Get("limit"))
		events, truncated, nextToken, err := store.QueryLogs(r.Context(), q.Get("source_kind"), q.Get("source_name"), q.Get("container_id"), q.Get("stream"), q.Get("since"), q.Get("until"), q.Get("page_token"), limit)
		if err != nil {
			embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", err.Error())
			return
		}
		if events == nil {
			events = []telemetry.LogEventRow{}
		}
		resp := map[string]interface{}{"version": 1, "events": events, "truncated": truncated}
		if nextToken != "" {
			resp["next_page_token"] = nextToken
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func registerEmbedTelemetryHandlers(mux *http.ServeMux, bearerToken string, store *telemetry.Store, logger *slog.Logger) {
	mux.HandleFunc("GET /v1/worker/telemetry/node:info", embedNodeInfoHandler(bearerToken, store, logger))
	mux.HandleFunc("GET /v1/worker/telemetry/node:stats", embedNodeStatsHandler(bearerToken))
	if store == nil {
		mux.HandleFunc("GET /v1/worker/telemetry/containers", embedTelemetryContainersEmptyHandler(bearerToken))
		mux.HandleFunc("GET /v1/worker/telemetry/containers/", embedTelemetryContainerNotFoundHandler(bearerToken))
		mux.HandleFunc("GET /v1/worker/telemetry/logs", embedTelemetryLogsEmptyHandler(bearerToken))
		return
	}
	mux.HandleFunc("GET /v1/worker/telemetry/containers", embedTelemetryContainersHandler(bearerToken, store))
	mux.HandleFunc("GET /v1/worker/telemetry/containers/", embedTelemetryContainerByIDHandler(bearerToken, store))
	mux.HandleFunc("GET /v1/worker/telemetry/logs", embedTelemetryLogsHandler(bearerToken, store))
}

func embedReadyzHandler(runner embedRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ready, reason := runner.Ready(r.Context())
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

const embedJobsRunMaxBodyBytes = 10 * 1024 * 1024

func embedJobsRunHandler(runner embedRunner, workspaceRoot, bearerToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !embedTelemetryAuth(w, r, bearerToken) {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, embedJobsRunMaxBodyBytes)
		var req workerapi.RunJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if strings.Contains(err.Error(), "request body too large") {
				embedWriteProblem(w, http.StatusRequestEntityTooLarge, problem.TypeValidation, "Request Entity Too Large", "Request body exceeds maximum size")
				return
			}
			embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "Invalid request body")
			return
		}
		if err := workerapi.ValidateRequest(&req); err != nil {
			embedWriteProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", err.Error())
			return
		}
		resp, err := runner.RunJob(r.Context(), &req, workspaceRoot)
		if err != nil {
			embedWriteProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "Job execution failed")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
