// Embed provides the Worker API server for embedded use (node-manager single-process).
// See docs/tech_specs/worker_node.md CYNAI.WORKER.SingleProcessHostBinary.
package workerapiserver

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cypher0n3/cynodeai/worker_node/internal/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/inferenceproxy"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
)

// EmbedConfig is the configuration for running the Worker API embedded in the node-manager process.
type EmbedConfig struct {
	BearerToken    string
	StateDir       string
	TelemetryStore *telemetry.Store
	Logger         *slog.Logger
}

// RunEmbedded starts the Worker API server in the current process using env for executor and proxy config.
// Returns a channel that closes when the public API is listening, and a shutdown function to call on exit.
// The caller must call shutdown before process exit.
func RunEmbedded(ctx context.Context, cfg EmbedConfig) (readyCh <-chan struct{}, shutdown func(), err error) {
	stateDir := cfg.StateDir
	if stateDir == "" {
		stateDir = embedGetEnv("WORKER_API_STATE_DIR", filepath.Join(os.TempDir(), "cynode", "state"))
	}
	workspaceRoot := embedGetEnv("WORKER_SPACE_ROOT", filepath.Join(os.TempDir(), "cynodeai-workspaces"))
	exec := executor.New(
		embedGetEnv("CONTAINER_RUNTIME", "podman"),
		time.Duration(embedGetEnvInt("DEFAULT_TIMEOUT_SECONDS", 300))*time.Second,
		embedGetEnvInt("MAX_OUTPUT_BYTES", 262144),
		embedGetEnv("OLLAMA_UPSTREAM_URL", ""),
		embedGetEnv("INFERENCE_PROXY_IMAGE", ""),
		nil,
	)
	proxyCfg, err := loadProxyConfigFromEnv(stateDir, cfg.Logger)
	if err != nil {
		return nil, nil, err
	}
	publicMux, internalMux := buildMuxesFromEmbedConfig(exec, cfg.BearerToken, workspaceRoot, cfg.TelemetryStore, cfg.Logger, proxyCfg)
	runCfg := RunConfig{
		PublicHandler:      publicMux,
		InternalHandler:    internalMux,
		ListenAddr:         embedGetEnv("LISTEN_ADDR", ":9190"),
		InternalListenAddr: embedGetEnv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:9191"),
		StateDir:           stateDir,
		SocketByService:    proxyCfg.InternalProxy.SocketByService,
		InternalListenUnix: strings.TrimSpace(os.Getenv("WORKER_INTERNAL_LISTEN_UNIX")),
		Logger:             cfg.Logger,
	}
	srv, err := NewServer(&runCfg)
	if err != nil {
		return nil, nil, err
	}
	ready, err := srv.Start(ctx)
	if err != nil {
		return nil, nil, err
	}
	proxyCtx, cancelProxies := context.WithCancel(context.WithoutCancel(ctx))
	var proxyWg sync.WaitGroup
	startManagedServiceInferenceProxies(proxyCtx, stateDir, proxyCfg.ManagedServiceTargets, cfg.Logger, &proxyWg)
	startSBAInferenceProxy(proxyCtx, stateDir, cfg.Logger, &proxyWg)
	shutdownFn := func() {
		cancelProxies()
		proxyWg.Wait()
		clearEmbedInferenceRuntime()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		if proxyCfg.InternalProxy.SecureStore != nil {
			proxyCfg.InternalProxy.SecureStore.Close()
		}
	}
	return ready, shutdownFn, nil
}

func embedGetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func embedGetEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

var (
	embedInferenceRuntimeMu    sync.Mutex
	embedInferenceProxyCtx     context.Context
	embedInferenceProxyWg      *sync.WaitGroup
	managedPMAInferenceStarted sync.Map // serviceID -> struct{}
)

func setEmbedInferenceRuntime(pctx context.Context, wg *sync.WaitGroup) {
	embedInferenceRuntimeMu.Lock()
	defer embedInferenceRuntimeMu.Unlock()
	embedInferenceProxyCtx = pctx
	embedInferenceProxyWg = wg
}

func clearEmbedInferenceRuntime() {
	embedInferenceRuntimeMu.Lock()
	defer embedInferenceRuntimeMu.Unlock()
	embedInferenceProxyCtx = nil
	embedInferenceProxyWg = nil
}

// EnsureManagedPMAInferenceProxy starts the per-service Ollama UDS proxy for a PMA service_id if not already running.
// Call before starting a new PMA container when the orchestrator adds session-bound instances after node-manager boot.
//
//nolint:contextcheck // shared embed inference ctx or call ctx; TODO only when both unset for standalone goroutine
func EnsureManagedPMAInferenceProxy(ctx context.Context, stateDir, serviceID string, logger *slog.Logger) {
	serviceID = strings.TrimSpace(serviceID)
	if serviceID == "" {
		return
	}
	if _, loaded := managedPMAInferenceStarted.LoadOrStore(serviceID, struct{}{}); loaded {
		return
	}
	const internalProxySocketBaseDir = "run/managed_agent_proxy"
	sockDir := filepath.Join(stateDir, internalProxySocketBaseDir, serviceID)
	sockPath := filepath.Join(sockDir, "inference.sock")
	if err := os.MkdirAll(sockDir, 0o700); err != nil {
		managedPMAInferenceStarted.Delete(serviceID)
		if logger != nil {
			logger.Error("inference proxy: failed to create socket dir", "service_id", serviceID, "error", err)
		}
		return
	}
	rawUpstream := embedGetEnv("OLLAMA_UPSTREAM_URL", inferenceproxy.DefaultUpstream)
	upstreamURL := strings.ReplaceAll(rawUpstream, "host.containers.internal", "localhost")
	if logger != nil {
		logger.Info("inference proxy started", "service_id", serviceID, "sock", sockPath, "upstream", upstreamURL)
	}
	embedInferenceRuntimeMu.Lock()
	pctx := embedInferenceProxyCtx
	wg := embedInferenceProxyWg
	embedInferenceRuntimeMu.Unlock()
	if pctx == nil {
		pctx = ctx
	}
	if pctx == nil {
		pctx = context.TODO()
	}
	if wg != nil {
		wg.Add(1)
	}
	go func(sock, upstream string) {
		if wg != nil {
			defer wg.Done()
		}
		inferenceproxy.RunUDSWithUpstream(pctx, sock, upstream)
	}(sockPath, upstreamURL)
}

func startManagedServiceInferenceProxies(ctx context.Context, stateDir string, targets map[string]embedManagedServiceTarget, logger *slog.Logger, wg *sync.WaitGroup) {
	setEmbedInferenceRuntime(ctx, wg)
	for serviceID, target := range targets {
		if !strings.EqualFold(target.ServiceType, "pma") {
			continue
		}
		EnsureManagedPMAInferenceProxy(ctx, stateDir, serviceID, logger)
	}
}

// SBAInferenceProxySocketEnv is the env key for the host path to the SBA inference proxy socket.
// When set, the executor mounts this socket into SBA containers (non-pod path) for agent_inference.
const SBAInferenceProxySocketEnv = "SBA_INFERENCE_PROXY_SOCKET"

const sbaInferenceProxySubdir = "run/inference_proxy"
const sbaInferenceProxySockName = "inference-proxy.sock"

// startSBAInferenceProxy starts a single inference proxy for SBA jobs at stateDir/run/inference_proxy/inference-proxy.sock
// and sets SBA_INFERENCE_PROXY_SOCKET so the executor can bind-mount it into non-pod SBA containers.
func startSBAInferenceProxy(ctx context.Context, stateDir string, logger *slog.Logger, wg *sync.WaitGroup) {
	rawUpstream := embedGetEnv("OLLAMA_UPSTREAM_URL", inferenceproxy.DefaultUpstream)
	upstreamURL := strings.ReplaceAll(rawUpstream, "host.containers.internal", "localhost")
	sockDir := filepath.Join(stateDir, sbaInferenceProxySubdir)
	sockPath := filepath.Join(sockDir, sbaInferenceProxySockName)
	if err := os.MkdirAll(sockDir, 0o700); err != nil {
		if logger != nil {
			logger.Error("SBA inference proxy: failed to create socket dir", "error", err)
		}
		return
	}
	if logger != nil {
		logger.Info("SBA inference proxy started", "sock", sockPath, "upstream", upstreamURL)
	}
	if err := os.Setenv(SBAInferenceProxySocketEnv, sockPath); err != nil && logger != nil {
		logger.Warn("SBA inference proxy: failed to set env", "error", err)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		inferenceproxy.RunUDSWithUpstream(ctx, sockPath, upstreamURL)
	}()
}
