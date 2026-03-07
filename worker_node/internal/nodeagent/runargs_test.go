package nodeagent

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestBuildManagedServiceRunArgs_NoSecretsMount(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "cynodeai-managed-pma-main")
	for i := 0; i < len(args)-1; i++ {
		if args[i] != "-v" {
			continue
		}
		mount := args[i+1]
		hostPath, _, _ := strings.Cut(mount, ":")
		if strings.Contains(hostPath, "secrets") {
			t.Errorf("run args must not mount secure store: got host path %q", hostPath)
		}
	}
}

func TestBuildManagedServiceRunArgs_UDSPathWhenServiceIDPathSafe(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID: "pma-main", ServiceType: serviceTypePMA, Image: "pma:latest",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name")
	expectedHost := filepath.Join(stateDir, ManagedAgentProxySocketBaseDir, "pma-main")
	var found bool
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-v" {
			mount := args[i+1]
			hostPath, containerPath, _ := strings.Cut(mount, ":")
			if containerPath == "/run/cynode/managed_agent_proxy" && strings.Contains(hostPath, "pma-main") {
				found = true
				if hostPath != expectedHost {
					t.Errorf("expected host path %q, got %q", expectedHost, hostPath)
				}
				break
			}
		}
	}
	if !found {
		t.Error("expected -v mount for UDS dir when service ID is path-safe")
	}
}

func TestBuildManagedServiceRunArgs_NoAGENT_TOKEN(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
		Env:         map[string]string{"OTHER": "val"},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name")
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-e" {
			env := args[i+1]
			if strings.HasPrefix(env, "AGENT_TOKEN=") {
				t.Errorf("run args must not pass AGENT_TOKEN: got %q", env)
			}
		}
	}
}

func TestBuildManagedServiceRunArgs_AutoProxyURLs(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
			MCPGatewayProxyURL:   proxyURLAuto,
			ReadyCallbackProxyURL: proxyURLAuto,
		},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name")
	var hasMCP, hasReady bool
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-e" {
			env := args[i+1]
			if strings.HasPrefix(env, "MCP_GATEWAY_PROXY_URL=") && strings.Contains(env, "http+unix://") {
				hasMCP = true
			}
			if strings.HasPrefix(env, "READY_CALLBACK_PROXY_URL=") && strings.Contains(env, "http+unix://") {
				hasReady = true
			}
		}
	}
	if !hasMCP || !hasReady {
		t.Errorf("expected auto proxy URLs to be resolved to http+unix; MCP=%v Ready=%v", hasMCP, hasReady)
	}
}

func TestDefaultPortForServiceType(t *testing.T) {
	if got := DefaultPortForServiceType("pma"); got != "8090" {
		t.Errorf("pma: got %q", got)
	}
	if got := DefaultPortForServiceType("PMA"); got != "8090" {
		t.Errorf("PMA: got %q", got)
	}
	if got := DefaultPortForServiceType("unknown"); got != "" {
		t.Errorf("unknown: got %q", got)
	}
}

func TestBuildManagedServiceRunArgs_UnsafeServiceIDNoUDSMount(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID: "a/b", ServiceType: "pma", Image: "pma:latest",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "a/b", "pma", "pma:latest", "name")
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-v" && strings.Contains(args[i+1], "managed_agent_proxy") {
			t.Error("unsafe service ID must not get UDS mount")
		}
	}
}
