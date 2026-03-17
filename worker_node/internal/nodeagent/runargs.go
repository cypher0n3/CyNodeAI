// Package nodeagent: BuildManagedServiceRunArgs builds container run args for a managed service.
// Used by node-manager main and by BDD to assert no secure-store mount (REQ-WORKER-0168).

package nodeagent

import (
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

// ManagedAgentProxySocketBaseDir is the path suffix under state_dir for per-service UDS.
const ManagedAgentProxySocketBaseDir = "run/managed_agent_proxy"

// BuildManagedServiceRunArgs returns the container run args for one managed service (env, mounts, image, args).
// stateDir is the node state directory (e.g. from WORKER_API_STATE_DIR or CYNODE_STATE_DIR).
// runtime is the container runtime ("podman" or "docker"); healthcheck is only added for podman.
// Used by cmd/node-manager and by BDD to verify run args never mount the secure store.
//
// REQ-WORKER-0174: managed-service containers MUST run with --network=none; inference and proxy
// traffic routes exclusively through UDS sockets mounted into the container.
// REQ-WORKER-0270: node_local inference is injected as OLLAMA_BASE_URL=http+unix://... pointing at
// the per-service inference socket (not a TCP URL).
func BuildManagedServiceRunArgs(stateDir string, svc *nodepayloads.ConfigManagedService, serviceID, serviceType, image, name, runtime string) []string {
	args := []string{"run", "-d", "--name", name}
	if strings.TrimSpace(runtime) == "podman" {
		uid := os.Getuid()
		gid := os.Getgid()
		// Keep host UID/GID mapping so per-service UDS owned by the worker user stays accessible in-container.
		args = append(args, "--userns=keep-id", "--user", strconv.Itoa(uid)+":"+strconv.Itoa(gid))
	}
	// REQ-WORKER-0174: network isolation; inference and proxy routes use UDS sockets only.
	args = append(args, "--network=none")
	if strings.TrimSpace(svc.RestartPolicy) == "always" {
		args = append(args, "--restart", "always")
	}
	// NOTE: port publish intentionally removed (REQ-WORKER-0174 / REQ-WORKER-0270):
	// orchestrator-to-agent traffic routes through the worker proxy via UDS, not direct TCP.
	if hc := podmanHealthcheckArgs(svc, serviceType, runtime); len(hc) > 0 {
		args = append(args, hc...)
	}
	if serviceIDPathSafe(serviceID) {
		hostUDSDir := filepath.Join(stateDir, ManagedAgentProxySocketBaseDir, serviceID)
		args = append(args, "-v", hostUDSDir+":/run/cynode/managed_agent_proxy")
	}
	// REQ-WORKER-0174: PMA listens on UDS socket so orchestrator traffic reaches it via worker proxy.
	// The socket is at the fixed path inside the container's mounted UDS dir.
	if strings.ToLower(strings.TrimSpace(serviceType)) == serviceTypePMA && serviceIDPathSafe(serviceID) {
		pmaListenSock := inferenceSocketContainerPath + "/service.sock"
		args = append(args, "-e", "PMA_LISTEN_ADDR=unix:"+pmaListenSock)
	}
	if svc.Orchestrator != nil {
		mcpURL, readyURL := applyAutoProxyURLs(stateDir, serviceID, svc.Orchestrator.MCPGatewayProxyURL, svc.Orchestrator.ReadyCallbackProxyURL)
		if mcpURL != "" {
			args = append(args, "-e", "MCP_GATEWAY_URL="+mcpURL, "-e", "PMA_MCP_GATEWAY_URL="+mcpURL, "-e", "MCP_GATEWAY_PROXY_URL="+mcpURL)
		}
		if readyURL != "" {
			args = append(args, "-e", "READY_CALLBACK_PROXY_URL="+readyURL)
		}
	}
	args = applyManagedServiceInferenceEnv(args, stateDir, serviceID, svc)
	for k, v := range svc.Env {
		if k != "" {
			args = append(args, "-e", k+"="+v)
		}
	}
	args = append(args, image)
	args = append(args, svc.Args...)
	return args
}

// ServiceIDPathSafe returns true if serviceID is safe for use in a filesystem path (no traversal).
func ServiceIDPathSafe(serviceID string) bool {
	return serviceID != "" &&
		!strings.Contains(serviceID, "/") &&
		!strings.Contains(serviceID, "..") &&
		!strings.Contains(serviceID, "\\")
}

func serviceIDPathSafe(serviceID string) bool {
	return ServiceIDPathSafe(serviceID)
}

// DefaultPortForServiceType previously returned the TCP port for a service type.
// REQ-WORKER-0174 / REQ-WORKER-0270: TCP port publishing is removed; all inter-service
// communication uses UDS sockets. Retained for reference and tests; always returns "".
func DefaultPortForServiceType(_ string) string {
	return ""
}

// internalPortForServiceType returns the container-internal listen port for a service type.
// NOTE: this port is NOT published to the host (REQ-WORKER-0174).
// It is only used for in-container health-check commands.
func internalPortForServiceType(serviceType string) string {
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case serviceTypePMA:
		return "8090"
	default:
		return ""
	}
}

// pmaServiceSockPath is the path inside the container where PMA listens (UDS). Used for health check.
const pmaServiceSockPath = inferenceSocketContainerPath + "/service.sock"

// podmanHealthcheckArgs returns podman --health-* args when runtime is podman and svc has a healthcheck; otherwise nil.
// For PMA (REQ-WORKER-0174 / REQ-WORKER-0270) the container has no TCP; health check uses curl over UDS.
// For other service types with a healthcheck, no UDS path is defined so we skip (caller can add later if needed).
func podmanHealthcheckArgs(svc *nodepayloads.ConfigManagedService, serviceType, runtime string) []string {
	if strings.TrimSpace(runtime) != "podman" || svc.Healthcheck == nil {
		return nil
	}
	path := strings.TrimSpace(svc.Healthcheck.Path)
	if path == "" {
		path = "/healthz"
	}
	// PMA listens only on UDS; use curl --unix-socket (PMA image must include curl).
	if strings.ToLower(strings.TrimSpace(serviceType)) == serviceTypePMA {
		healthCmd := "curl -sf --unix-socket " + pmaServiceSockPath + " http://localhost" + path + " || exit 1"
		return []string{
			"--health-cmd", "CMD-SHELL " + healthCmd,
			"--health-interval", "10s",
			"--health-timeout", "3s",
			"--health-retries", "3",
			"--health-start-period", "5s",
		}
	}
	port := internalPortForServiceType(serviceType)
	if port == "" {
		return nil
	}
	healthURL := "http://localhost:" + port + path
	return []string{
		"--health-cmd", "CMD-SHELL wget -q -O /dev/null " + healthURL + " || exit 1",
		"--health-interval", "10s",
		"--health-timeout", "3s",
		"--health-retries", "3",
		"--health-start-period", "5s",
	}
}

func httpUnixProxyURL(udsSocketPath, endpointPath string) string {
	udsSocketPath = strings.TrimSpace(udsSocketPath)
	endpointPath = strings.TrimSpace(endpointPath)
	if udsSocketPath == "" || endpointPath == "" {
		return ""
	}
	return "http+unix://" + url.PathEscape(udsSocketPath) + endpointPath
}

func applyAutoProxyURLs(stateDir, serviceID, mcpURL, readyURL string) (resolvedMCP, resolvedReady string) {
	mcpURL = strings.TrimSpace(mcpURL)
	readyURL = strings.TrimSpace(readyURL)
	if !serviceIDPathSafe(serviceID) {
		return mcpURL, readyURL
	}
	udsSock := filepath.Join(stateDir, ManagedAgentProxySocketBaseDir, serviceID, "proxy.sock")
	if mcpURL == proxyURLAuto {
		resolvedMCP = httpUnixProxyURL(udsSock, "/v1/worker/internal/orchestrator/mcp:call")
	} else {
		resolvedMCP = mcpURL
	}
	if readyURL == proxyURLAuto {
		resolvedReady = httpUnixProxyURL(udsSock, "/v1/worker/internal/orchestrator/agent:ready")
	} else {
		resolvedReady = readyURL
	}
	return resolvedMCP, resolvedReady
}

// inferenceSocketContainerPath is the path inside the managed-service container where the
// per-service UDS socket directory is mounted. The inference socket lives at
// <containerPath>/inference.sock.
const inferenceSocketContainerPath = "/run/cynode/managed_agent_proxy"

// applyManagedServiceInferenceEnv injects inference-related env vars into the run args.
// REQ-WORKER-0270: node_local mode injects OLLAMA_BASE_URL as a http+unix:// URL pointing at
// the per-service inference socket mounted at inferenceSocketContainerPath/inference.sock.
// TCP URLs are never injected for node_local mode.
// sortedKeys returns the keys of m in sorted order for deterministic output.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func applyManagedServiceInferenceEnv(args []string, stateDir, serviceID string, svc *nodepayloads.ConfigManagedService) []string {
	if svc == nil || svc.Inference == nil {
		return args
	}
	inf := svc.Inference
	mode := strings.ToLower(strings.TrimSpace(inf.Mode))
	if mode == "" {
		mode = "node_local"
	}
	switch mode {
	case "node_local", "remote_node":
		// REQ-WORKER-0270: UDS only. The inference proxy sidecar (managed by worker-api) listens
		// on <stateDir>/run/managed_agent_proxy/<serviceID>/inference.sock (host path) which is
		// mounted inside the container at inferenceSocketContainerPath/inference.sock.
		inferenceSock := inferenceSocketContainerPath + "/inference.sock"
		udsURL := "http+unix://" + url.PathEscape(inferenceSock)
		args = append(args, "-e", "OLLAMA_BASE_URL="+udsURL)
		// Forward backend-derived env vars (e.g. OLLAMA_NUM_CTX sized to GPU VRAM) so the
		// managed agent can use them in per-request API options. These come directly from the
		// orchestrator config payload — no OS-level env leakage through node-manager.
		for _, k := range sortedKeys(inf.BackendEnv) {
			if v := strings.TrimSpace(inf.BackendEnv[k]); v != "" {
				args = append(args, "-e", k+"="+v)
			}
		}
	case "external":
		// External routing: keep egress hints available to the agent runtime.
		if apiEgressURL := strings.TrimSpace(inf.APIEgressBaseURL); apiEgressURL != "" {
			args = append(args, "-e", "API_EGRESS_BASE_URL="+apiEgressURL)
		}
		if providerID := strings.TrimSpace(inf.ProviderID); providerID != "" {
			args = append(args, "-e", "INFERENCE_PROVIDER_ID="+providerID)
		}
	}
	if model := strings.TrimSpace(inf.DefaultModel); model != "" {
		args = append(args, "-e", "INFERENCE_MODEL="+model)
	}
	return args
}
