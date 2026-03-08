// Package main provides the Worker API service.
// See docs/tech_specs/worker_api.md.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/cmd/worker-api/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
)

const maxManagedProxyBodyBytes = 1 << 20 // 1 MiB
const internalProxySocketBaseDir = "run/managed_agent_proxy"

type contextKey string

const callerServiceIDContextKey contextKey = "caller_service_id"

type managedServiceTarget struct {
	ServiceType string `json:"service_type"`
	BaseURL     string `json:"base_url"`
}

type managedServiceProxyRequest struct {
	Version int                 `json:"version"`
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers,omitempty"`
	BodyB64 string              `json:"body_b64,omitempty"`
}

type managedServiceProxyResponse struct {
	Version int                 `json:"version"`
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers,omitempty"`
	BodyB64 string              `json:"body_b64,omitempty"`
}

type internalOrchestratorProxyConfig struct {
	UpstreamBaseURL string
	SocketByService map[string]string // service_id -> socket path
	SecureStore     *securestore.Store
}

func main() {
	os.Exit(runMain(context.Background()))
}

// runMain builds and runs the server until ctx is canceled.
// Returns 0 on success, 1 on failure. Extracted for testability.
func runMain(ctx context.Context) int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	bearerToken := getEnv("WORKER_API_BEARER_TOKEN", "")
	if bearerToken == "" {
		logger.Error("WORKER_API_BEARER_TOKEN must be set")
		return 1
	}

	exec := executor.New(
		getEnv("CONTAINER_RUNTIME", "podman"),
		time.Duration(getEnvInt("DEFAULT_TIMEOUT_SECONDS", 300))*time.Second,
		getEnvInt("MAX_OUTPUT_BYTES", 262144), // 256 KiB default per worker_api.md
		getEnv("OLLAMA_UPSTREAM_URL", ""),
		getEnv("INFERENCE_PROXY_IMAGE", ""),
		nil,
	)
	stateDir := getEnv("WORKER_API_STATE_DIR", filepath.Join(os.TempDir(), "cynode", "state"))
	workspaceRoot := getEnv("WORKSPACE_ROOT", filepath.Join(os.TempDir(), "cynodeai-workspaces"))
	telemetryStore, cfg := setupWorkerStateAndProxyConfig(ctx, stateDir, logger)
	if telemetryStore != nil {
		defer func() { _ = telemetryStore.Close() }()
		go runRetentionAndVacuum(ctx, telemetryStore, logger)
		recordNodeBoot(ctx, telemetryStore, logger)
		logger = slog.New(&telemetry.LogHandler{
			Inner:  slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
			Store:  telemetryStore,
			Source: "worker_api",
		})
		slog.SetDefault(logger)
	}
	mux := newMux(exec, bearerToken, workspaceRoot, telemetryStore, logger, cfg.ManagedServiceTargets)
	internalMux := newInternalMux(cfg.InternalProxy, logger)
	srv := newServer(mux)
	internalSrv := newInternalServer(internalMux)
	serverErr := make(chan error, 1)
	startPublicAndInternalServers(srv, internalSrv, serverErr)
	if socketPath := strings.TrimSpace(os.Getenv("WORKER_INTERNAL_LISTEN_UNIX")); socketPath != "" {
		cleanup, exitCode := listenInternalUnix(socketPath, internalSrv, serverErr, logger)
		if exitCode != 0 {
			return exitCode
		}
		defer cleanup()
	}
	internalUDSServers, internalUDSListeners, exitCode := startInternalUDSListeners(logger, internalMux, &cfg.InternalProxy, serverErr)
	if exitCode != 0 {
		return exitCode
	}
	defer func() {
		for _, l := range internalUDSListeners {
			_ = os.Remove(l.Addr().String())
		}
	}()
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	select {
	case <-ctx.Done():
	case <-done:
	case <-serverErr:
	}
	// Shutdown with a timeout; derive from ctx so contextcheck passes, but use WithoutCancel so we get a grace period even when ctx is already canceled.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return 1
	}
	if err := internalSrv.Shutdown(shutdownCtx); err != nil {
		return 1
	}
	for _, s := range internalUDSServers {
		_ = s.Shutdown(shutdownCtx)
	}
	return 0
}

// recordNodeBoot writes one node_boot row to the telemetry store per worker_telemetry_api.md (once per process boot).
func recordNodeBoot(ctx context.Context, store *telemetry.Store, logger *slog.Logger) {
	bootID := getEnv("NODE_BOOT_ID", "")
	if bootID == "" {
		bootID = fmt.Sprintf("boot-%d", time.Now().UTC().UnixNano())
	}
	row := telemetry.NodeBootRow{
		BootID:        bootID,
		NodeSlug:      getEnv("NODE_SLUG", "default"),
		BuildVersion:  getEnv("BUILD_VERSION", "dev"),
		PlatformOS:    runtime.GOOS,
		PlatformArch:  runtime.GOARCH,
		KernelVersion: getEnv("KERNEL_VERSION", ""),
	}
	if err := store.InsertNodeBoot(ctx, &row); err != nil && logger != nil {
		logger.Warn("telemetry node_boot insert failed", "error", err)
	}
}

// doRetentionAndVacuumOnce runs retention and vacuum once; used by runRetentionAndVacuum and tests.
func doRetentionAndVacuumOnce(ctx context.Context, store *telemetry.Store, logger *slog.Logger) {
	if err := store.EnforceRetention(ctx); err != nil {
		logger.Warn("telemetry retention", "error", err)
	}
	if err := store.Vacuum(ctx); err != nil {
		logger.Warn("telemetry vacuum", "error", err)
	}
}

// retentionTickerInterval and vacuumTickerInterval are used by runRetentionAndVacuum; tests may override for coverage.
var retentionTickerInterval = time.Hour
var vacuumTickerInterval = 24 * time.Hour

func runRetentionAndVacuum(ctx context.Context, store *telemetry.Store, logger *slog.Logger) {
	doRetentionAndVacuumOnce(ctx, store, logger)
	retentionTicker := time.NewTicker(retentionTickerInterval)
	defer retentionTicker.Stop()
	vacuumTicker := time.NewTicker(vacuumTickerInterval)
	defer vacuumTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-retentionTicker.C:
			if err := store.EnforceRetention(ctx); err != nil {
				logger.Warn("telemetry retention", "error", err)
			}
		case <-vacuumTicker.C:
			if err := store.Vacuum(ctx); err != nil {
				logger.Warn("telemetry vacuum", "error", err)
			}
		}
	}
}

func newMux(exec *executor.Executor, bearerToken, workspaceRoot string, telemetryStore *telemetry.Store, logger *slog.Logger, managedServiceTargets ...map[string]managedServiceTarget) *http.ServeMux {
	mux := http.NewServeMux()
	var targets map[string]managedServiceTarget
	if len(managedServiceTargets) > 0 && managedServiceTargets[0] != nil {
		targets = managedServiceTargets[0]
	} else {
		targets = loadManagedServiceTargetsFromEnv(logger)
	}
	// REQ-WORKER-0140, REQ-WORKER-0141: unauthenticated GET /healthz; body plain text "ok" per worker_api.md
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// REQ-WORKER-0140, REQ-WORKER-0142: unauthenticated GET /readyz
	mux.HandleFunc("GET /readyz", readyzHandler(exec))
	mux.HandleFunc("POST /v1/worker/jobs:run", handleRunJob(exec, bearerToken, workspaceRoot, logger))
	mux.HandleFunc("POST /v1/worker/managed-services/{service_id}/proxy:http", handleManagedServiceProxy(bearerToken, targets, logger))
	// REQ-WORKER-0200--0243: Worker Telemetry API.
	mux.HandleFunc("GET /v1/worker/telemetry/node:info", telemetryAuth(bearerToken, handleNodeInfo(logger)))
	mux.HandleFunc("GET /v1/worker/telemetry/node:stats", telemetryAuth(bearerToken, handleNodeStats(logger)))
	if telemetryStore != nil {
		mux.HandleFunc("GET /v1/worker/telemetry/containers", telemetryAuth(bearerToken, handleListContainers(telemetryStore)))
		mux.HandleFunc("GET /v1/worker/telemetry/containers/", telemetryAuth(bearerToken, handleGetContainer(telemetryStore)))
		mux.HandleFunc("GET /v1/worker/telemetry/logs", telemetryAuth(bearerToken, handleQueryLogs(telemetryStore)))
	}
	return mux
}

func newInternalMux(cfg internalOrchestratorProxyConfig, logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/worker/internal/orchestrator/mcp:call", handleInternalOrchestratorProxy(cfg, logger, "mcp"))
	mux.HandleFunc("POST /v1/worker/internal/orchestrator/agent:ready", handleInternalOrchestratorProxy(cfg, logger, "ready"))
	return mux
}

type workerProxyConfig struct {
	ManagedServiceTargets map[string]managedServiceTarget
	InternalProxy         internalOrchestratorProxyConfig
}

func loadWorkerProxyConfig(logger *slog.Logger) workerProxyConfig {
	out := workerProxyConfig{
		ManagedServiceTargets: map[string]managedServiceTarget{},
		InternalProxy: internalOrchestratorProxyConfig{
			UpstreamBaseURL: strings.TrimSpace(os.Getenv("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL")),
			SocketByService: map[string]string{},
			SecureStore:     nil,
		},
	}
	stateDir := getEnv("WORKER_API_STATE_DIR", filepath.Join(os.TempDir(), "cynode", "state"))
	if nodeCfgRaw := strings.TrimSpace(os.Getenv("WORKER_NODE_CONFIG_JSON")); nodeCfgRaw != "" {
		applyNodeConfigToWorkerProxyConfig(&out, stateDir, nodeCfgRaw, logger)
	}
	if len(out.ManagedServiceTargets) == 0 {
		out.ManagedServiceTargets = loadManagedServiceTargetsFromEnv(logger)
	}
	if out.InternalProxy.UpstreamBaseURL == "" {
		out.InternalProxy.UpstreamBaseURL = strings.TrimSpace(os.Getenv("ORCHESTRATOR_URL"))
	}
	return out
}

func applyNodeConfigToWorkerProxyConfig(out *workerProxyConfig, stateDir, nodeCfgRaw string, logger *slog.Logger) {
	var nodeCfg nodepayloads.NodeConfigurationPayload
	if err := json.Unmarshal([]byte(nodeCfgRaw), &nodeCfg); err != nil {
		if logger != nil {
			logger.Warn("invalid WORKER_NODE_CONFIG_JSON; falling back to env-only proxy config", "error", err)
		}
		return
	}
	out.ManagedServiceTargets = deriveManagedServiceTargetsFromNodeConfig(&nodeCfg)
	if out.InternalProxy.UpstreamBaseURL == "" {
		out.InternalProxy.UpstreamBaseURL = strings.TrimSpace(nodeCfg.Orchestrator.BaseURL)
	}
	if nodeCfg.ManagedServices != nil {
		for i := range nodeCfg.ManagedServices.Services {
			svc := &nodeCfg.ManagedServices.Services[i]
			serviceID := strings.TrimSpace(svc.ServiceID)
			if serviceID != "" && svc.Orchestrator != nil {
				if path, ok := managedAgentProxySocketPath(stateDir, serviceID); ok {
					out.InternalProxy.SocketByService[serviceID] = path
				}
			}
		}
	}
}

// startPublicAndInternalServers starts srv and optionally internalSrv in goroutines; errors are sent to serverErr.
func startPublicAndInternalServers(srv, internalSrv *http.Server, serverErr chan error) {
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	if internalSrv.Addr != "" {
		go func() {
			if err := internalSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErr <- err
			}
		}()
	}
}

// setupWorkerStateAndProxyConfig opens telemetry (if available) and loads proxy config with secure store. Returns (telemetryStore, cfg).
func setupWorkerStateAndProxyConfig(ctx context.Context, stateDir string, logger *slog.Logger) (*telemetry.Store, workerProxyConfig) {
	var telemetryStore *telemetry.Store
	if ts, err := telemetry.Open(ctx, stateDir); err != nil {
		if logger != nil {
			logger.Warn("telemetry store unavailable, containers/logs endpoints disabled", "error", err)
		}
	} else {
		telemetryStore = ts
	}
	cfg := loadWorkerProxyConfig(logger)
	if store, source, err := securestore.Open(stateDir); err == nil {
		cfg.InternalProxy.SecureStore = store
		if logger != nil && source == securestore.MasterKeySourceEnvB64 {
			logger.Warn("secure store uses env_b64 master key backend; migrate to stronger host-backed key source")
		}
	} else if logger != nil {
		logger.Error("secure store unavailable; internal orchestrator proxy will fail closed", "error", err)
	}
	return telemetryStore, cfg
}

// listenInternalUnix binds the internal server to a unix socket. Returns cleanup and 0 on success, or (nil, 1) on error.
func listenInternalUnix(socketPath string, srv *http.Server, serverErr chan error, logger *slog.Logger) (cleanup func(), exitCode int) {
	if err := os.RemoveAll(socketPath); err != nil {
		logger.Error("failed to prepare internal unix socket", "path", socketPath, "error", err)
		return nil, 1
	}
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		logger.Error("failed to listen on internal unix socket", "path", socketPath, "error", err)
		return nil, 1
	}
	go func() {
		if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	return func() {
		_ = l.Close()
		_ = os.Remove(socketPath)
	}, 0
}

// startInternalUDSListeners starts per-service UDS listeners for the internal proxy. Returns (servers, listeners, 0) or (nil, nil, 1) on error.
func startInternalUDSListeners(logger *slog.Logger, internalMux *http.ServeMux, cfg *internalOrchestratorProxyConfig, serverErr chan error) ([]*http.Server, []net.Listener, int) {
	var servers []*http.Server
	var listeners []net.Listener
	for serviceID, socketPath := range cfg.SocketByService {
		if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
			logger.Error("failed to create managed-agent proxy socket directory", "service_id", serviceID, "path", socketPath, "error", err)
			return nil, nil, 1
		}
		if err := os.RemoveAll(socketPath); err != nil {
			logger.Error("failed to prepare managed-agent proxy socket", "service_id", serviceID, "path", socketPath, "error", err)
			return nil, nil, 1
		}
		l, err := net.Listen("unix", socketPath)
		if err != nil {
			logger.Error("failed to listen on managed-agent proxy socket", "service_id", serviceID, "path", socketPath, "error", err)
			return nil, nil, 1
		}
		if err := os.Chmod(socketPath, 0o600); err != nil {
			_ = l.Close()
			logger.Error("failed to set managed-agent proxy socket permissions", "service_id", serviceID, "path", socketPath, "error", err)
			return nil, nil, 1
		}
		serviceMux := withCallerServiceID(internalMux, serviceID)
		serviceSrv := newInternalServer(serviceMux)
		servers = append(servers, serviceSrv)
		listeners = append(listeners, l)
		go func(s *http.Server, listener net.Listener) {
			if err := s.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErr <- err
			}
		}(serviceSrv, l)
	}
	return servers, listeners, 0
}

// validateInternalProxyRequest checks loopback, caller identity, secure store, and upstream. On failure writes problem and returns (_, _, false).
func validateInternalProxyRequest(w http.ResponseWriter, r *http.Request, cfg internalOrchestratorProxyConfig) (serviceID string, record *securestore.AgentTokenRecord, ok bool) {
	if !isLoopbackRequest(r) {
		writeProblem(w, http.StatusForbidden, problem.TypeAuthentication, "Forbidden", "internal endpoint requires loopback or unix-socket access")
		return "", nil, false
	}
	serviceID, ok = callerServiceIDFromRequest(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "missing caller identity binding")
		return "", nil, false
	}
	if cfg.SecureStore == nil {
		writeProblem(w, http.StatusBadGateway, problem.TypeInternal, "Bad Gateway", "secure store unavailable")
		return "", nil, false
	}
	var err error
	record, err = cfg.SecureStore.GetAgentToken(serviceID)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "agent token unavailable for caller identity")
		return "", nil, false
	}
	if strings.TrimSpace(cfg.UpstreamBaseURL) == "" {
		writeProblem(w, http.StatusBadGateway, problem.TypeInternal, "Bad Gateway", "internal proxy upstream not configured")
		return "", nil, false
	}
	return serviceID, record, true
}

func managedAgentProxySocketPath(stateDir, serviceID string) (string, bool) {
	serviceID = strings.TrimSpace(serviceID)
	if serviceID == "" || strings.Contains(serviceID, "/") || strings.Contains(serviceID, "\\") || strings.Contains(serviceID, "..") {
		return "", false
	}
	base := filepath.Join(stateDir, internalProxySocketBaseDir, serviceID)
	return filepath.Join(base, "proxy.sock"), true
}

func deriveManagedServiceTargetsFromNodeConfig(cfg *nodepayloads.NodeConfigurationPayload) map[string]managedServiceTarget {
	// Node configuration does not currently include managed-service endpoint URLs.
	// Targets are supplied via worker env (hydrated by node-manager from desired state).
	_ = cfg
	return map[string]managedServiceTarget{}
}

func handleInternalOrchestratorProxy(cfg internalOrchestratorProxyConfig, logger *slog.Logger, endpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serviceID, record, ok := validateInternalProxyRequest(w, r, cfg)
		if !ok {
			return
		}
		reqPayload, reqBody, ok := decodeManagedProxyRequest(w, r)
		if !ok {
			return
		}
		target := managedServiceTarget{ServiceType: "orchestrator", BaseURL: strings.TrimSpace(cfg.UpstreamBaseURL)}
		if reqPayload.Headers == nil {
			reqPayload.Headers = map[string][]string{}
		}
		reqPayload.Headers["Authorization"] = []string{"Bearer " + record.Token}
		start := time.Now()
		respPayload, status, detail := forwardManagedProxyRequest(r.Context(), target, reqPayload, reqBody)
		if status != 0 {
			writeProblem(w, status, problem.TypeValidation, http.StatusText(status), detail)
			return
		}
		if logger != nil {
			logger.Info(
				"internal orchestrator proxy call",
				"endpoint", endpoint,
				"service_id", serviceID,
				"token_present", true,
				"method", reqPayload.Method,
				"path", reqPayload.Path,
				"upstream_status", respPayload.Status,
				"duration_ms", int(time.Since(start).Milliseconds()),
			)
		}
		writeJSON(w, http.StatusOK, respPayload)
	}
}

func withCallerServiceID(next http.Handler, serviceID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), callerServiceIDContextKey, serviceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func callerServiceIDFromRequest(r *http.Request) (string, bool) {
	serviceID, ok := r.Context().Value(callerServiceIDContextKey).(string)
	if !ok || strings.TrimSpace(serviceID) == "" {
		return "", false
	}
	return serviceID, true
}

func isLoopbackRequest(r *http.Request) bool {
	if local := r.Context().Value(http.LocalAddrContextKey); local != nil {
		if addr, ok := local.(net.Addr); ok && addr.Network() == "unix" {
			return true
		}
	}
	host := r.RemoteAddr
	if strings.Contains(host, ":") {
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func loadManagedServiceTargetsFromEnv(logger *slog.Logger) map[string]managedServiceTarget {
	raw := strings.TrimSpace(os.Getenv("WORKER_MANAGED_SERVICE_TARGETS_JSON"))
	if raw == "" {
		return map[string]managedServiceTarget{}
	}
	targets := make(map[string]managedServiceTarget)
	if err := json.Unmarshal([]byte(raw), &targets); err == nil {
		return targets
	}
	var simple map[string]string
	if err := json.Unmarshal([]byte(raw), &simple); err != nil {
		if logger != nil {
			logger.Warn("invalid WORKER_MANAGED_SERVICE_TARGETS_JSON; managed service proxy disabled", "error", err)
		}
		return map[string]managedServiceTarget{}
	}
	for serviceID, baseURL := range simple {
		targets[serviceID] = managedServiceTarget{ServiceType: "unknown", BaseURL: baseURL}
	}
	return targets
}

func handleManagedServiceProxy(bearerToken string, targets map[string]managedServiceTarget, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireBearerToken(r, bearerToken) {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		serviceID := strings.TrimSpace(r.PathValue("service_id"))
		target, ok := targets[serviceID]
		if !ok || strings.TrimSpace(target.BaseURL) == "" {
			writeProblem(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", "Unknown managed service")
			return
		}
		reqPayload, reqBody, ok := decodeManagedProxyRequest(w, r)
		if !ok {
			return
		}
		start := time.Now()
		respPayload, status, detail := forwardManagedProxyRequest(r.Context(), target, reqPayload, reqBody)
		if status != 0 {
			writeProblem(w, status, problem.TypeValidation, http.StatusText(status), detail)
			return
		}
		if logger != nil {
			logger.Info("managed service proxy call",
				"service_id", serviceID,
				"service_type", target.ServiceType,
				"method", reqPayload.Method,
				"path", reqPayload.Path,
				"upstream_status", respPayload.Status,
				"duration_ms", int(time.Since(start).Milliseconds()),
			)
		}
		writeJSON(w, http.StatusOK, respPayload)
	}
}

func decodeManagedProxyRequest(w http.ResponseWriter, r *http.Request) (*managedServiceProxyRequest, []byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxManagedProxyBodyBytes)
	var req managedServiceProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			writeProblem(w, http.StatusRequestEntityTooLarge, problem.TypeValidation, "Request Entity Too Large", "Proxy request exceeds maximum size")
			return nil, nil, false
		}
		writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "Invalid proxy request body")
		return nil, nil, false
	}
	if req.Version != 1 {
		writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "unsupported proxy request version")
		return nil, nil, false
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if !isAllowedProxyMethod(method) {
		writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "unsupported proxy method")
		return nil, nil, false
	}
	req.Method = method
	if !isSafeProxyPath(req.Path) {
		writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "invalid proxy path")
		return nil, nil, false
	}
	if req.BodyB64 == "" {
		return &req, nil, true
	}
	body, err := base64.StdEncoding.DecodeString(req.BodyB64)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "invalid body_b64")
		return nil, nil, false
	}
	if len(body) > maxManagedProxyBodyBytes {
		writeProblem(w, http.StatusRequestEntityTooLarge, problem.TypeValidation, "Request Entity Too Large", "decoded proxy request body exceeds maximum size")
		return nil, nil, false
	}
	return &req, body, true
}

func forwardManagedProxyRequest(
	ctx context.Context,
	target managedServiceTarget,
	req *managedServiceProxyRequest,
	body []byte,
) (resp *managedServiceProxyResponse, statusCode int, detail string) {
	upstreamURL := strings.TrimSuffix(target.BaseURL, "/") + req.Path
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, http.StatusBadRequest, "failed to build upstream request"
	}
	for name, values := range req.Headers {
		if !isAllowedProxyRequestHeader(name) {
			continue
		}
		for _, v := range values {
			httpReq.Header.Add(name, v)
		}
	}
	timeoutSec := getEnvInt("WORKER_MANAGED_PROXY_UPSTREAM_TIMEOUT_SEC", 30)
	if timeoutSec < 1 {
		timeoutSec = 30
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, http.StatusBadGateway, "upstream request failed"
	}
	defer func() { _ = httpResp.Body.Close() }()
	limited := io.LimitReader(httpResp.Body, maxManagedProxyBodyBytes+1)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, http.StatusBadGateway, "failed to read upstream response"
	}
	if len(respBody) > maxManagedProxyBodyBytes {
		return nil, http.StatusBadGateway, "upstream response exceeds maximum size"
	}
	respHeaders := make(map[string][]string)
	for name, values := range httpResp.Header {
		if !isAllowedProxyResponseHeader(name) {
			continue
		}
		cp := append([]string(nil), values...)
		respHeaders[name] = cp
	}
	return &managedServiceProxyResponse{
		Version: 1,
		Status:  httpResp.StatusCode,
		Headers: respHeaders,
		BodyB64: base64.StdEncoding.EncodeToString(respBody),
	}, 0, ""
}

func isAllowedProxyMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isSafeProxyPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return false
	}
	return !strings.Contains(path, "://")
}

func isAllowedProxyRequestHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "accept", "content-type", "authorization", "x-request-id":
		return true
	default:
		return false
	}
}

func isAllowedProxyResponseHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "content-type", "x-request-id", "cache-control":
		return true
	default:
		return false
	}
}

func telemetryAuth(bearerToken string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireBearerToken(r, bearerToken) {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		next(w, r)
	}
}

func handleNodeInfo(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeSlug := getEnv("NODE_SLUG", "default")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version": 1,
			"node_slug": nodeSlug,
			"build": map[string]string{"build_version": "dev"},
			"platform": map[string]string{"os": "linux", "arch": "amd64", "kernel_version": ""},
		})
	}
}

func handleNodeStats(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version":    1,
			"captured_at": time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
			"cpu":        map[string]interface{}{"cores": 0, "load1": 0.0, "load5": 0.0, "load15": 0.0},
			"memory":     map[string]interface{}{"total_mb": 0, "used_mb": 0, "free_mb": 0},
			"disk":       map[string]interface{}{"state_dir_free_mb": 0, "state_dir_total_mb": 0},
			"container_runtime": map[string]string{"runtime": getEnv("CONTAINER_RUNTIME", "podman"), "version": ""},
		})
	}
}

// readyzHandler implements REQ-WORKER-0142: 200 "ready" when ready to accept jobs, 503 otherwise.
func readyzHandler(exec *executor.Executor) http.HandlerFunc {
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

// decodeRunJobRequest decodes POST body; enforces maxBytes and returns 413 on overflow (REQ-WORKER-0145).
func decodeRunJobRequest(w http.ResponseWriter, r *http.Request, maxBytes int64) (*workerapi.RunJobRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	var req workerapi.RunJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			writeProblem(w, http.StatusRequestEntityTooLarge, problem.TypeValidation, "Request Entity Too Large", "Request body exceeds maximum size")
			return nil, false
		}
		writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", "Invalid request body")
		return nil, false
	}
	return &req, true
}

func handleRunJob(exec *executor.Executor, bearerToken, workspaceRoot string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireBearerToken(r, bearerToken) {
			writeProblem(w, http.StatusUnauthorized, problem.TypeAuthentication, "Unauthorized", "Invalid or missing bearer token")
			return
		}
		req, ok := decodeRunJobRequest(w, r, 10*1024*1024) // 10 MiB per worker_api.md (REQ-WORKER-0145)
		if !ok {
			return
		}
		if err := validateRunJobRequest(req); err != nil {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", err.Error())
			return
		}
		workspaceDir, cleanup, err := prepareWorkspace(workspaceRoot, req.JobID)
		if err != nil {
			logger.Error("workspace creation failed", "error", err, "job_id", req.JobID)
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "Workspace creation failed")
			return
		}
		if cleanup != nil {
			defer cleanup()
		}
		resp, err := exec.RunJob(r.Context(), req, workspaceDir)
		if err != nil {
			logger.Error("job execution error", "error", err)
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "Job execution failed")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func validateRunJobRequest(req *workerapi.RunJobRequest) error {
	if err := workerapi.ValidateRequest(req); err != nil {
		return err
	}
	if req.Version != 1 {
		return fmt.Errorf("unsupported version")
	}
	if req.TaskID == "" || req.JobID == "" {
		return fmt.Errorf("task_id and job_id are required")
	}
	return nil
}

// prepareWorkspace creates a per-job workspace dir under workspaceRoot.
// Returns (dir, cleanup, nil) on success; ("", nil, nil) when workspaceRoot is empty; ("", nil, err) on failure.
func prepareWorkspace(workspaceRoot, jobID string) (dir string, cleanup func(), err error) {
	if workspaceRoot == "" {
		return "", nil, nil
	}
	safeID := strings.ReplaceAll(jobID, string(filepath.Separator), "_")
	workspaceDir := filepath.Join(workspaceRoot, safeID)
	if err := os.MkdirAll(workspaceDir, 0o700); err != nil {
		return "", nil, errors.Join(fmt.Errorf("mkdir %s", workspaceDir), err)
	}
	return workspaceDir, func() { _ = os.RemoveAll(workspaceDir) }, nil
}

func newServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              getEnv("LISTEN_ADDR", ":9190"),
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		// /v1/worker/jobs:run is synchronous and can exceed 30s for SBA/inference workloads.
		// Keep write timeout disabled so long-running jobs do not terminate with EOF mid-flight.
		WriteTimeout:   0,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func newInternalServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              getEnv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:9191"),
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func requireBearerToken(r *http.Request, expected string) bool {
	authz := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(authz) <= len(prefix) || authz[:len(prefix)] != prefix {
		return false
	}
	return authz[len(prefix):] == expected
}

func writeProblem(w http.ResponseWriter, status int, typ, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem.Details{
		Type:   typ,
		Title:  title,
		Status: status,
		Detail: detail,
	})
}

func handleListContainers(store *telemetry.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeProblem(w, http.StatusMethodNotAllowed, problem.TypeValidation, "Method Not Allowed", "")
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
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "")
			return
		}
		if list == nil {
			list = []telemetry.ContainerRow{}
		}
		resp := map[string]interface{}{"version": 1, "containers": list}
		if nextToken != "" {
			resp["next_page_token"] = nextToken
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleGetContainer(store *telemetry.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeProblem(w, http.StatusMethodNotAllowed, problem.TypeValidation, "Method Not Allowed", "")
			return
		}
		containerID := strings.TrimPrefix(r.URL.Path, "/v1/worker/telemetry/containers/")
		if containerID == "" {
			writeProblem(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", "container_id required")
			return
		}
		c, err := store.GetContainer(r.Context(), containerID)
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, problem.TypeInternal, "Internal Server Error", "")
			return
		}
		if c == nil {
			writeProblem(w, http.StatusNotFound, problem.TypeNotFound, "Not Found", "container not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"version": 1, "container": c})
	}
}

func parseLogsLimit(limitParam string) int {
	const defaultLimit, maxLimit = 1000, 5000
	if limitParam == "" {
		return defaultLimit
	}
	n, err := strconv.Atoi(limitParam)
	if err != nil || n <= 0 || n > maxLimit {
		return defaultLimit
	}
	return n
}

func validateLogsQuery(sourceKind, containerID string) string {
	if sourceKind != "" || containerID != "" {
		return ""
	}
	return "source_kind+source_name or source_kind=container+container_id required"
}

func handleQueryLogs(store *telemetry.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeProblem(w, http.StatusMethodNotAllowed, problem.TypeValidation, "Method Not Allowed", "")
			return
		}
		q := r.URL.Query()
		if msg := validateLogsQuery(q.Get("source_kind"), q.Get("container_id")); msg != "" {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", msg)
			return
		}
		limit := parseLogsLimit(q.Get("limit"))
		events, truncated, nextToken, err := store.QueryLogs(r.Context(), q.Get("source_kind"), q.Get("source_name"), q.Get("container_id"), q.Get("stream"), q.Get("since"), q.Get("until"), q.Get("page_token"), limit)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, problem.TypeValidation, "Bad Request", err.Error())
			return
		}
		if events == nil {
			events = []telemetry.LogEventRow{}
		}
		resp := map[string]interface{}{"version": 1, "events": events, "truncated": truncated}
		if nextToken != "" {
			resp["next_page_token"] = nextToken
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
