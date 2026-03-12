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
	startManagedServiceInferenceProxies(ctx, stateDir, proxyCfg.ManagedServiceTargets, cfg.Logger)
	shutdownFn := func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
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

func startManagedServiceInferenceProxies(ctx context.Context, stateDir string, targets map[string]embedManagedServiceTarget, logger *slog.Logger) {
	rawUpstream := embedGetEnv("OLLAMA_UPSTREAM_URL", inferenceproxy.DefaultUpstream)
	upstreamURL := strings.ReplaceAll(rawUpstream, "host.containers.internal", "localhost")
	const internalProxySocketBaseDir = "run/managed_agent_proxy"
	for serviceID, target := range targets {
		if !strings.EqualFold(target.ServiceType, "pma") {
			continue
		}
		sockDir := filepath.Join(stateDir, internalProxySocketBaseDir, serviceID)
		sockPath := filepath.Join(sockDir, "inference.sock")
		if err := os.MkdirAll(sockDir, 0o700); err != nil {
			if logger != nil {
				logger.Error("inference proxy: failed to create socket dir", "service_id", serviceID, "error", err)
			}
			continue
		}
		if logger != nil {
			logger.Info("inference proxy started", "service_id", serviceID, "sock", sockPath, "upstream", upstreamURL)
		}
		go func(id, sock, upstream string) {
			inferenceproxy.RunUDSWithUpstream(ctx, sock, upstream)
		}(serviceID, sockPath, upstreamURL)
	}
}
