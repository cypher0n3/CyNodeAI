// Package main provides the node manager service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
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

// runMain loads config and runs the node manager until ctx is cancelled.
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
// The token must not be logged. Returns when the process has been started (or an error).
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
	cmd.Env = append(os.Environ(), "WORKER_API_BEARER_TOKEN="+bearerToken)
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

// defaultPortForServiceType returns the host port to publish for the service type (e.g. PMA 8090).
func defaultPortForServiceType(serviceType string) string {
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "pma":
		return "8090"
	default:
		return ""
	}
}

// startManagedServices starts each desired managed service container (e.g. PMA) from config.
// Containers are named cynodeai-managed-<service_id>. If a container already exists, it is started if stopped.
// Orchestrator URLs and agent token are passed as env so the agent can use the worker proxy.
func startManagedServices(services []nodepayloads.ConfigManagedService) error {
	rt := getEnv("CONTAINER_RUNTIME", "podman")
	for _, svc := range services {
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
		// Check if container already exists
		check := exec.Command(rt, "ps", "-a", "--format", "{{.Names}}")
		out, err := check.Output()
		if err == nil && strings.Contains(string(out), name) {
			_ = exec.Command(rt, "start", name).Run()
			continue
		}
		// Build run args: -d --name <name> [--restart always] [-p host:container] [-e K=V...] image [args...]
		args := []string{"run", "-d", "--name", name}
		if strings.TrimSpace(svc.RestartPolicy) == "always" {
			args = append(args, "--restart", "always")
		}
		if port := defaultPortForServiceType(serviceType); port != "" {
			args = append(args, "-p", port+":"+port)
		}
		// Env: orchestrator proxy URLs and agent token so agent uses worker proxy
		if svc.Orchestrator != nil {
			if u := strings.TrimSpace(svc.Orchestrator.MCPGatewayProxyURL); u != "" {
				args = append(args, "-e", "MCP_GATEWAY_PROXY_URL="+u)
			}
			if u := strings.TrimSpace(svc.Orchestrator.ReadyCallbackProxyURL); u != "" {
				args = append(args, "-e", "READY_CALLBACK_PROXY_URL="+u)
			}
			if t := strings.TrimSpace(svc.Orchestrator.AgentToken); t != "" {
				args = append(args, "-e", "AGENT_TOKEN="+t)
			}
		}
		for k, v := range svc.Env {
			if k == "" {
				continue
			}
			args = append(args, "-e", k+"="+v)
		}
		args = append(args, image)
		args = append(args, svc.Args...)
		cmd := exec.Command(rt, args...)
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("managed service %q: %w: %s", serviceID, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
