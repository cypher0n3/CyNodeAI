// Package main provides the node manager service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/worker_node/internal/nodeagent"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
	"github.com/google/uuid"
)

// cmdRunner is used for exec operations so tests can inject a fake. Production uses realCmdRunner.
type cmdRunner interface {
	LookPath(string) (string, error)
	CombinedOutput(string, ...string) ([]byte, error)
	StartDetached(string, []string, []string) error
}

var runner cmdRunner = realCmdRunner{}

type realCmdRunner struct{}

func (realCmdRunner) LookPath(bin string) (string, error) { return exec.LookPath(bin) }
func (realCmdRunner) CombinedOutput(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}
func (realCmdRunner) StartDetached(name string, args, env []string) error {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func main() {
	os.Exit(runMain(context.Background()))
}

// getEnv returns the environment variable key if set, otherwise def. Used for optional main-level config.
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// stopNodeManagedContainers stops and removes all node-manager-owned containers:
// - cynodeai-managed-* (PMA and other managed service containers)
// - cynodeai-worker-api (worker-api container when run as a managed service)
//
// This is called on graceful shutdown so the dev setup-dev stop sequence does not
// leave stray containers behind. Best-effort: errors are logged but not returned.
func stopNodeManagedContainers(logger *slog.Logger) {
	rt := getEnv("CONTAINER_RUNTIME", "podman")
	for _, f := range []string{"cynodeai-managed-", "cynodeai-worker-api"} {
		stopManagedContainersMatching(rt, f, logger)
	}
}

func stopManagedContainersMatching(rt, nameFilter string, logger *slog.Logger) {
	out, err := runner.CombinedOutput(rt, "ps", "-aq", "--filter", "name="+nameFilter)
	if err != nil {
		return
	}
	for _, id := range strings.Fields(strings.TrimSpace(string(out))) {
		if id != "" {
			stopAndRemoveContainer(rt, id, logger)
		}
	}
}

func stopAndRemoveContainer(rt, id string, logger *slog.Logger) {
	if _, stopErr := runner.CombinedOutput(rt, "stop", "--time", "5", id); stopErr != nil && logger != nil {
		logger.Warn("stop managed container", "id", id, "error", stopErr)
	}
	if _, rmErr := runner.CombinedOutput(rt, "rm", "-f", id); rmErr != nil && logger != nil {
		logger.Warn("rm managed container", "id", id, "error", rmErr)
	}
}

// runMain loads config and runs the node manager until ctx is canceled.
// Node-manager owns the telemetry DB per spec: ensures dir, records node_boot, runs retention/vacuum, records shutdown.
// Returns 0 on success, 1 on failure. Extracted for testability.
func runMain(ctx context.Context) int {
	level := slog.LevelInfo
	if getEnv("NODE_MANAGER_DEBUG", "") != "" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	// Cancelable context so signal handler can stop the node.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	// On exit (signal or error), stop containers that node-manager started so the
	// dev setup-dev stop sequence does not leave stray managed-service containers.
	defer func() {
		if logger != nil {
			logger.Info("node manager stopping managed containers")
		}
		stopNodeManagedContainers(logger)
	}()
	// Telemetry: node-manager owns DB lifecycle (REQ-WORKER-0210, 0220–0222, 0258).
	stateDir := effectiveStateDir()
	telemetryStore, err := telemetry.Open(runCtx, stateDir)
	if err != nil {
		logger.Warn("telemetry store unavailable, lifecycle events will not be persisted", "error", err)
	} else {
		defer func() {
			recordNodeManagerShutdown(runCtx, telemetryStore, logger)
			_ = telemetryStore.Close()
		}()
		// All logs go to stdout (foreground) and to the telemetry DB when store is available.
		innerHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
		logger = slog.New(&telemetry.LogHandler{Inner: innerHandler, Store: telemetryStore, Source: "node_manager"})
		slog.SetDefault(logger)
		if err := recordNodeBootTelemetry(runCtx, telemetryStore, logger); err != nil {
			logger.Warn("telemetry node_boot failed", "error", err)
		}
		go runTelemetryRetentionAndVacuum(runCtx, telemetryStore, logger)
		// Worker-api (binary or container) must not record node_boot again; node-manager already did.
		_ = os.Setenv("NODE_SKIP_NODE_BOOT_RECORD", "1")
	}
	cfg := nodeagent.LoadConfig()
	opts := &nodeagent.RunOptions{
		StartWorkerAPI:       startWorkerAPI,
		StartOllama:          startOllama,
		StartManagedServices: startManagedServices,
		PullModels:           pullModels,
	}
	if getEnv("NODE_MANAGER_SKIP_SERVICES", "") != "" {
		opts = nil
	}
	if err := nodeagent.RunWithOptions(runCtx, logger, &cfg, opts); err != nil {
		// Clear startup-failure message to stdout and telemetry (nodeagent wraps e.g. "start worker API: ...").
		attrs := []any{"error", err, "phase", "startup"}
		if msg := err.Error(); strings.Contains(msg, "start worker API") {
			attrs = append(attrs, "component", "api_service")
		} else if strings.Contains(msg, "start inference") {
			attrs = append(attrs, "component", "inference")
		} else if strings.Contains(msg, "start managed services") {
			attrs = append(attrs, "component", "managed_services")
		}
		logger.Error("node manager failed to start", attrs...)
		return 1
	}
	return 0
}

// recordNodeBootTelemetry writes one node_boot row (node-manager owns this per spec).
func recordNodeBootTelemetry(ctx context.Context, store *telemetry.Store, logger *slog.Logger) error {
	bootID := getEnv("NODE_BOOT_ID", "")
	if bootID == "" {
		bootID = fmt.Sprintf("boot-%d", time.Now().UTC().UnixNano())
	}
	row := &telemetry.NodeBootRow{
		BootID:        bootID,
		NodeSlug:      getEnv("NODE_SLUG", "default"),
		BuildVersion:  getEnv("BUILD_VERSION", "dev"),
		PlatformOS:    runtime.GOOS,
		PlatformArch:  runtime.GOARCH,
		KernelVersion: getEnv("KERNEL_VERSION", ""),
	}
	if err := store.InsertNodeBoot(ctx, row); err != nil {
		return err
	}
	if logger != nil {
		logger.Info("telemetry node_boot recorded", "boot_id", bootID, "node_slug", row.NodeSlug)
	}
	return nil
}

// retentionTickerInterval and vacuumTickerInterval are used by runTelemetryRetentionAndVacuum; tests may override for coverage.
var retentionTickerInterval = time.Hour
var vacuumTickerInterval = 24 * time.Hour

// runTelemetryRetentionAndVacuum runs retention on startup and hourly, vacuum daily (REQ-WORKER-0220, 0221, 0222).
func runTelemetryRetentionAndVacuum(ctx context.Context, store *telemetry.Store, logger *slog.Logger) {
	if err := store.EnforceRetention(ctx); err != nil && logger != nil {
		logger.Warn("telemetry retention on startup", "error", err)
	}
	retentionTicker := time.NewTicker(retentionTickerInterval)
	defer retentionTicker.Stop()
	vacuumTicker := time.NewTicker(vacuumTickerInterval)
	defer vacuumTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-retentionTicker.C:
			if err := store.EnforceRetention(ctx); err != nil && logger != nil {
				logger.Warn("telemetry retention", "error", err)
			}
		case <-vacuumTicker.C:
			if err := store.Vacuum(ctx); err != nil && logger != nil {
				logger.Warn("telemetry vacuum", "error", err)
			}
		}
	}
}

// recordNodeManagerShutdown writes a service log event before exit (REQ-WORKER-0258, worker_telemetry_api.md).
func recordNodeManagerShutdown(ctx context.Context, store *telemetry.Store, logger *slog.Logger) {
	err := store.InsertLogEvent(ctx, &telemetry.LogEventInput{
		LogID:      uuid.New().String(),
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		SourceKind: "service",
		SourceName: "node_manager",
		Level:      "info",
		Message:    "node manager shutdown",
	})
	if err != nil && logger != nil {
		logger.Warn("telemetry shutdown log failed", "error", err)
	}
}

// workerAPIContainerName is the name of the worker-api container when run as a managed service.
const workerAPIContainerName = "cynodeai-worker-api"

// startWorkerAPI starts the worker-api process or container. When NODE_MANAGER_WORKER_API_IMAGE is set,
// starts worker-api as a container (worker-managed service); otherwise starts the binary from
// NODE_MANAGER_WORKER_API_BIN. Passes through INFERENCE_PROXY_IMAGE, OLLAMA_UPSTREAM_URL, CONTAINER_RUNTIME,
// WORKER_API_STATE_DIR so inference jobs and executor config are correct.
// Inference backend env vars (e.g. OLLAMA_NUM_CTX) are NOT propagated here; they are passed
// directly to the Ollama container via startOllama and to managed service containers via
// ConfigManagedServiceInference.BackendEnv in the orchestrator config payload.
func startWorkerAPI(bearerToken string) error {
	if image := strings.TrimSpace(getEnv("NODE_MANAGER_WORKER_API_IMAGE", "")); image != "" {
		return startWorkerAPIContainer(image, bearerToken)
	}
	return startWorkerAPIBinary(bearerToken)
}

func startWorkerAPIBinary(bearerToken string) error {
	bin := getEnv("NODE_MANAGER_WORKER_API_BIN", "worker-api")
	if !strings.Contains(bin, "/") {
		path, err := runner.LookPath(bin)
		if err != nil {
			return err
		}
		bin = path
	}
	env := os.Environ()
	env = append(env, "WORKER_API_BEARER_TOKEN="+bearerToken)
	for _, key := range []string{"INFERENCE_PROXY_IMAGE", "OLLAMA_UPSTREAM_URL", "CONTAINER_RUNTIME", "WORKER_API_STATE_DIR"} {
		if v := os.Getenv(key); v != "" {
			env = append(env, key+"="+v)
		}
	}
	return runner.StartDetached(bin, nil, env)
}

// startWorkerAPIContainer starts worker-api as a container (worker-managed service). Uses effectiveStateDir
// for the state volume so telemetry and secure store persist. If the container already exists, starts it.
func startWorkerAPIContainer(image, bearerToken string) error {
	rt := getEnv("CONTAINER_RUNTIME", "podman")
	stateDir := effectiveStateDir()
	out, err := runner.CombinedOutput(rt, "ps", "-a", "--format", "{{.Names}}")
	if err == nil && strings.Contains(string(out), workerAPIContainerName) {
		if out2, err2 := runner.CombinedOutput(rt, "start", workerAPIContainerName); err2 != nil {
			return fmt.Errorf("start existing worker-api container: %w: %s", err2, strings.TrimSpace(string(out2)))
		}
		return nil
	}
	// run -d --name cynodeai-worker-api -p 12090:12090 -e ... -v stateDir:/var/lib/cynode/state image
	args := []string{
		"run", "-d", "--name", workerAPIContainerName,
		"-p", "12090:12090",
		"-e", "WORKER_API_BEARER_TOKEN=" + bearerToken,
		"-e", "WORKER_API_STATE_DIR=/var/lib/cynode/state",
		"-e", "LISTEN_ADDR=:12090",
		"-e", "NODE_SKIP_NODE_BOOT_RECORD=1",
		"-v", stateDir + ":/var/lib/cynode/state",
	}
	for _, key := range []string{"INFERENCE_PROXY_IMAGE", "OLLAMA_UPSTREAM_URL", "CONTAINER_RUNTIME"} {
		if v := os.Getenv(key); v != "" {
			args = append(args, "-e", key+"="+v)
		}
	}
	args = append(args, image)
	if out, err := runner.CombinedOutput(rt, args...); err != nil {
		return fmt.Errorf("worker-api container: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// startOllama starts the Phase 1 inference container (Ollama). image/variant from config or env. Fail-fast on error.
// If a container named cynodeai-ollama already exists (e.g. from orchestrator compose), start it if stopped and return.
// containerNameExact reports whether psOutput (from podman ps --format {{.Names}}) contains
// name as an exact line match, avoiding false positives where the target is a prefix of
// another container name (e.g. "cynodeai-ollama" must not match "cynodeai-ollama-proxy-test").
func containerNameExact(psOutput, name string) bool {
	for _, line := range strings.Split(psOutput, "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

func startOllama(image, variant string, env map[string]string) error {
	rt := getEnv("CONTAINER_RUNTIME", "podman")
	if image == "" {
		image = getEnv("OLLAMA_IMAGE", "ollama/ollama")
	}
	name := getEnv("OLLAMA_CONTAINER_NAME", "cynodeai-ollama")
	// Skip if already present (e.g. started by orchestrator compose with ollama profile).
	// Use exact name matching to avoid false positives when the target name is a prefix of
	// another container (e.g. "cynodeai-ollama" must not match "cynodeai-ollama-proxy-test").
	out, err := runner.CombinedOutput(rt, "ps", "-a", "--format", "{{.Names}}")
	if err == nil && containerNameExact(string(out), name) {
		_, _ = runner.CombinedOutput(rt, "start", name)
		return nil
	}
	// Build run args; add GPU device access when the image variant suggests a GPU runtime.
	// ROCm (AMD): expose /dev/kfd and /dev/dri; set HSA_OVERRIDE_GFX_VERSION when env provides it.
	// NVIDIA/CUDA: add --gpus all (Docker) or --device nvidia.com/gpu=all (Podman with CDI).
	// CPU-only: no additional flags.
	args := []string{"run", "-d", "--name", name, "-p", "11434:11434"}
	// Persist downloaded models across restarts using a named volume.
	// The volume name is derived from the container name to allow multiple nodes
	// to have separate model stores when OLLAMA_CONTAINER_NAME is overridden.
	ollamaVolume := getEnv("OLLAMA_MODELS_VOLUME", name+"-models")
	args = append(args, "-v", ollamaVolume+":/root/.ollama")
	isROCm := strings.Contains(strings.ToLower(image), "rocm") ||
		strings.ToLower(getEnv("OLLAMA_GPU_VARIANT", "")) == "rocm"
	isCUDA := strings.Contains(strings.ToLower(image), "cuda") ||
		strings.Contains(strings.ToLower(image), "nvidia") ||
		strings.ToLower(getEnv("OLLAMA_GPU_VARIANT", "")) == "cuda"
	if isROCm {
		args = append(args, "--device", "/dev/kfd", "--device", "/dev/dri", "--group-add", "video")
		if gfxVer := getEnv("HSA_OVERRIDE_GFX_VERSION", ""); gfxVer != "" {
			args = append(args, "-e", "HSA_OVERRIDE_GFX_VERSION="+gfxVer)
		}
	} else if isCUDA {
		if rt == "docker" {
			args = append(args, "--gpus", "all")
		} else {
			args = append(args, "--device", "nvidia.com/gpu=all")
		}
	}
	// Pass orchestrator-derived env vars (e.g. OLLAMA_NUM_CTX sized to GPU VRAM).
	// Keys are sorted for deterministic arg ordering.
	for _, k := range sortedKeys(env) {
		args = append(args, "-e", k+"="+env[k])
	}
	args = append(args, image)
	if out, err := runner.CombinedOutput(rt, args...); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// pullModels calls `ollama pull` for each model in the list sequentially inside the
// running Ollama container. It is called in the background by node-manager so a slow
// pull does not block node startup or managed-service reconciliation.
func pullModels(models []string) error {
	name := getEnv("OLLAMA_CONTAINER_NAME", "cynodeai-ollama")
	rt := getEnv("CONTAINER_RUNTIME", "podman")
	var firstErr error
	for _, model := range models {
		if model == "" {
			continue
		}
		out, err := runner.CombinedOutput(rt, "exec", name, "ollama", "pull", model)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("pull %q: %w: %s", model, err, strings.TrimSpace(string(out)))
			}
		}
	}
	return firstErr
}

// sortedKeys returns the keys of m in sorted order for deterministic iteration.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// managedServiceContainerPrefix is the prefix for managed service container names.
const managedServiceContainerPrefix = "cynodeai-managed-"

// sanitizeContainerName returns a container-safe name (alphanumeric, underscore, hyphen, period).
func sanitizeContainerName(serviceID string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			return r
		}
		if unicode.IsSpace(r) {
			return '_'
		}
		return -1
	}, strings.TrimSpace(serviceID))
}

// waitForPMAReadyUDS polls GET /healthz over the PMA Unix domain socket until 200 or timeout.
// socketPath is the host path to the PMA service.sock (e.g. stateDir/run/managed_agent_proxy/<serviceID>/service.sock).
// REQ-WORKER-0174 / REQ-WORKER-0260: PMA uses UDS only; no TCP health check.
func waitForPMAReadyUDS(socketPath string, timeout time.Duration) {
	path := strings.TrimSpace(socketPath)
	if path == "" {
		return
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", path)
		},
	}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://unix/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// effectiveStateDir returns state directory for socket paths; matches nodeagent precedence so worker-api and node-manager agree.
func effectiveStateDir() string {
	if v := strings.TrimSpace(getEnv("WORKER_API_STATE_DIR", "")); v != "" {
		return v
	}
	if v := strings.TrimSpace(getEnv("CYNODE_STATE_DIR", "")); v != "" {
		return v
	}
	return "/var/lib/cynode/state"
}

// buildManagedServiceRunArgs returns the container run args for one managed service (delegates to nodeagent).
func buildManagedServiceRunArgs(rt string, svc *nodepayloads.ConfigManagedService, serviceID, serviceType, image, name string) []string {
	return nodeagent.BuildManagedServiceRunArgs(effectiveStateDir(), svc, serviceID, serviceType, image, name, rt)
}

// startOneManagedService ensures the managed service container is running and, for PMA, waits for /healthz over UDS.
func startOneManagedService(rt string, svc *nodepayloads.ConfigManagedService, serviceID, serviceType, image, name string) error {
	out, err := runner.CombinedOutput(rt, "ps", "-a", "--format", "{{.Names}}")
	if err == nil && strings.Contains(string(out), name) {
		_, _ = runner.CombinedOutput(rt, "start", name)
		if strings.EqualFold(serviceType, "pma") {
			sockPath := filepath.Join(effectiveStateDir(), nodeagent.ManagedAgentProxySocketBaseDir, serviceID, "service.sock")
			waitForPMAReadyUDS(sockPath, 30*time.Second)
		}
		return nil
	}
	args := buildManagedServiceRunArgs(rt, svc, serviceID, serviceType, image, name)
	if out, err := runner.CombinedOutput(rt, args...); err != nil {
		return fmt.Errorf("managed service %q: %w: %s", serviceID, err, strings.TrimSpace(string(out)))
	}
	if strings.EqualFold(serviceType, "pma") {
		sockPath := filepath.Join(effectiveStateDir(), nodeagent.ManagedAgentProxySocketBaseDir, serviceID, "service.sock")
		waitForPMAReadyUDS(sockPath, 30*time.Second)
	}
	return nil
}

// startManagedServices starts each desired managed service container (e.g. PMA) from config.
// Containers are named cynodeai-managed-<service_id>. If a container already exists, it is started if stopped.
// For PMA, waits for the service to respond on /healthz before returning so the orchestrator does not get "ready" before the container is reachable.
func startManagedServices(services []nodepayloads.ConfigManagedService) error {
	rt := getEnv("CONTAINER_RUNTIME", "podman")
	for i := range services {
		svc := &services[i]
		serviceID := strings.TrimSpace(svc.ServiceID)
		serviceType := strings.TrimSpace(svc.ServiceType)
		image := strings.TrimSpace(svc.Image)
		if serviceID == "" || serviceType == "" || image == "" {
			continue
		}
		name := managedServiceContainerPrefix + sanitizeContainerName(serviceID)
		if name == managedServiceContainerPrefix {
			continue
		}
		if err := startOneManagedService(rt, svc, serviceID, serviceType, image, name); err != nil {
			return err
		}
	}
	return nil
}
