// Package nodemanager: BuildManagedServiceRunArgs builds container run args for a managed service.
// Used by node-manager main and by BDD to assert no secure-store mount (REQ-WORKER-0168).

package nodemanager

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

// ManagedAgentProxySocketBaseDir is the path suffix under state_dir for per-service UDS.
const ManagedAgentProxySocketBaseDir = "run/managed_agent_proxy"

// BuildManagedServiceRunArgs returns the container run args for one managed service (env, mounts, image, args).
// stateDir is the node state directory (e.g. from WORKER_API_STATE_DIR or CYNODE_STATE_DIR).
// Used by cmd/node-manager and by BDD to verify run args never mount the secure store.
func BuildManagedServiceRunArgs(stateDir string, svc *nodepayloads.ConfigManagedService, serviceID, serviceType, image, name string) []string {
	args := []string{"run", "-d", "--name", name}
	if strings.TrimSpace(svc.RestartPolicy) == "always" {
		args = append(args, "--restart", "always")
	}
	if port := defaultPortForServiceType(serviceType); port != "" {
		args = append(args, "-p", port+":"+port)
	}
	if serviceIDPathSafe(serviceID) {
		hostUDSDir := filepath.Join(stateDir, ManagedAgentProxySocketBaseDir, serviceID)
		args = append(args, "-v", hostUDSDir+":/run/cynode/managed_agent_proxy")
	}
	if svc.Orchestrator != nil {
		mcpURL, readyURL := applyAutoProxyURLs(stateDir, serviceID, svc.Orchestrator.MCPGatewayProxyURL, svc.Orchestrator.ReadyCallbackProxyURL)
		if mcpURL != "" {
			args = append(args, "-e", "MCP_GATEWAY_PROXY_URL="+mcpURL)
		}
		if readyURL != "" {
			args = append(args, "-e", "READY_CALLBACK_PROXY_URL="+readyURL)
		}
	}
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

// DefaultPortForServiceType returns the host port to publish for the service type (e.g. PMA 8090).
func DefaultPortForServiceType(serviceType string) string {
	return defaultPortForServiceType(serviceType)
}

func defaultPortForServiceType(serviceType string) string {
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "pma":
		return "8090"
	default:
		return ""
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
