// Package main provides the node manager service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/worker_node/internal/nodemanager"
)

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

// runMain loads config and runs the node manager until ctx is canceled.
// Returns 0 on success, 1 on failure. Extracted for testability.
func runMain(ctx context.Context) int {
	level := slog.LevelInfo
	if getEnv("NODE_MANAGER_DEBUG", "") != "" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	cfg := nodemanager.LoadConfig()
	opts := &nodemanager.RunOptions{
		StartWorkerAPI:       startWorkerAPI,
		StartOllama:          startOllama,
		StartManagedServices: startManagedServices,
	}
	if getEnv("NODE_MANAGER_SKIP_SERVICES", "") != "" {
		opts = nil
	}
	if err := nodemanager.RunWithOptions(ctx, logger, &cfg, opts); err != nil {
		logger.Error("node manager failed", "error", err)
		return 1
	}
	return 0
}

// startWorkerAPI starts the worker-api process with the given bearer token in env.
// Passes through INFERENCE_PROXY_IMAGE, OLLAMA_UPSTREAM_URL, CONTAINER_RUNTIME, WORKER_API_STATE_DIR
// so inference jobs (pod path) and executor config are correct when node is started with extra_env (e.g. full-demo).
func startWorkerAPI(bearerToken string) error {
	bin := getEnv("NODE_MANAGER_WORKER_API_BIN", "worker-api")
	if !strings.Contains(bin, "/") {
		path, err := exec.LookPath(bin)
		if err != nil {
			return err
		}
		bin = path
	}
	cmd := exec.CommandContext(context.Background(), bin)
	env := os.Environ()
	env = append(env, "WORKER_API_BEARER_TOKEN="+bearerToken)
	for _, key := range []string{"INFERENCE_PROXY_IMAGE", "OLLAMA_UPSTREAM_URL", "CONTAINER_RUNTIME", "WORKER_API_STATE_DIR"} {
		if v := os.Getenv(key); v != "" {
			env = append(env, key+"="+v)
		}
	}
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// startOllama starts the Phase 1 inference container (Ollama). image/variant from config or env. Fail-fast on error.
// If a container named cynodeai-ollama already exists (e.g. from orchestrator compose), start it if stopped and return.
func startOllama(image, variant string) error {
	rt := getEnv("CONTAINER_RUNTIME", "podman")
	if image == "" {
		image = getEnv("OLLAMA_IMAGE", "ollama/ollama")
	}
	name := getEnv("OLLAMA_CONTAINER_NAME", "cynodeai-ollama")
	// Skip if already present (e.g. started by orchestrator compose with ollama profile)
	check := exec.Command(rt, "ps", "-a", "--format", "{{.Names}}")
	out, err := check.Output()
	if err == nil && strings.Contains(string(out), name) {
		_ = exec.Command(rt, "start", name).Run()
		return nil
	}
	// variant (rocm, cuda, cpu) can be used for image selection or env; Phase 1 we use image as-is.
	cmd := exec.Command(rt, "run", "-d", "--name", name, "-p", "11434:11434", image)
	if variant != "" {
		cmd.Env = append(os.Environ(), "OLLAMA_GPU_DRIVER="+variant)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
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

// waitForPMAReady polls http://127.0.0.1:port/healthz until 200 or timeout so we do not report "ready" before the container is reachable.
func waitForPMAReady(port string, timeout time.Duration) {
	url := "http://127.0.0.1:" + strings.TrimSpace(port) + "/healthz"
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// effectiveStateDir returns state directory for socket paths; matches nodemanager precedence so worker-api and node-manager agree.
func effectiveStateDir() string {
	if v := strings.TrimSpace(getEnv("WORKER_API_STATE_DIR", "")); v != "" {
		return v
	}
	if v := strings.TrimSpace(getEnv("CYNODE_STATE_DIR", "")); v != "" {
		return v
	}
	return "/var/lib/cynode/state"
}

// buildManagedServiceRunArgs returns the container run args for one managed service (delegates to nodemanager).
func buildManagedServiceRunArgs(svc *nodepayloads.ConfigManagedService, serviceID, serviceType, image, name string) []string {
	return nodemanager.BuildManagedServiceRunArgs(effectiveStateDir(), svc, serviceID, serviceType, image, name)
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
		check := exec.Command(rt, "ps", "-a", "--format", "{{.Names}}")
		out, err := check.Output()
		if err == nil && strings.Contains(string(out), name) {
			_ = exec.Command(rt, "start", name).Run()
			if strings.EqualFold(serviceType, "pma") {
				waitForPMAReady(getEnv("PMA_PORT", "8090"), 30*time.Second)
			}
			continue
		}
		args := buildManagedServiceRunArgs(svc, serviceID, serviceType, image, name)
		cmd := exec.Command(rt, args...)
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("managed service %q: %w: %s", serviceID, err, strings.TrimSpace(string(out)))
		}
		if strings.EqualFold(serviceType, "pma") {
			waitForPMAReady(getEnv("PMA_PORT", "8090"), 30*time.Second)
		}
	}
	return nil
}
