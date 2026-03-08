package nodeagent

import (
	"os"
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
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "cynodeai-managed-pma-main", "")
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
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "")
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
		Env:          map[string]string{"OTHER": "val"},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "")
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
			MCPGatewayProxyURL:    proxyURLAuto,
			ReadyCallbackProxyURL: proxyURLAuto,
		},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "")
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
	args := BuildManagedServiceRunArgs(stateDir, svc, "a/b", "pma", "pma:latest", "name", "")
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-v" && strings.Contains(args[i+1], "managed_agent_proxy") {
			t.Error("unsafe service ID must not get UDS mount")
		}
	}
}

func TestBuildManagedServiceRunArgs_HealthcheckWhenPodman(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
		Healthcheck: &nodepayloads.ConfigManagedServiceHealthcheck{
			Path:           "/healthz",
			ExpectedStatus: 200,
		},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "podman")
	var hasHealthCmd, hasInterval bool
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--health-cmd" {
			hasHealthCmd = true
			if !strings.Contains(args[i+1], "localhost:8090/healthz") {
				t.Errorf("health-cmd should target 8090/healthz, got %q", args[i+1])
			}
		}
		if args[i] == "--health-interval" && args[i+1] == "10s" {
			hasInterval = true
		}
	}
	if !hasHealthCmd || !hasInterval {
		t.Errorf("podman runtime with Healthcheck should add health args; health-cmd=%v interval=%v", hasHealthCmd, hasInterval)
	}
}

func TestBuildManagedServiceRunArgs_NoHealthcheckWhenDocker(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
		Healthcheck: &nodepayloads.ConfigManagedServiceHealthcheck{Path: "/healthz"},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "docker")
	for i := range args {
		if args[i] == "--health-cmd" {
			t.Error("docker runtime must not get podman-only health-cmd")
		}
	}
}

func TestBuildManagedServiceRunArgs_InferenceEnvFromConfig(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID:   "pma-main",
		ServiceType: "pma",
		Image:       "pma:latest",
		Inference: &nodepayloads.ConfigManagedServiceInference{
			Mode:         "node_local",
			BaseURL:      "http://inference.internal:11434",
			DefaultModel: "tinyllama",
		},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "podman")
	argv := strings.Join(args, " ")
	if !strings.Contains(argv, "OLLAMA_BASE_URL=http://inference.internal:11434") {
		t.Fatalf("expected OLLAMA_BASE_URL from config inference.base_url, got args=%q", argv)
	}
	if !strings.Contains(argv, "INFERENCE_MODEL=tinyllama") {
		t.Fatalf("expected INFERENCE_MODEL from config inference.default_model, got args=%q", argv)
	}
}

func TestBuildManagedServiceRunArgs_InferenceEnvRuntimeDefault(t *testing.T) {
	tests := []struct {
		runtime       string
		wantOllamaURL string
	}{
		{"docker", "OLLAMA_BASE_URL=http://host.docker.internal:11434"},
		{"podman", "OLLAMA_BASE_URL=http://host.containers.internal:11434"},
	}
	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			stateDir := t.TempDir()
			svc := &nodepayloads.ConfigManagedService{
				ServiceID:   "pma-main",
				ServiceType: "pma",
				Image:       "pma:latest",
				Inference: &nodepayloads.ConfigManagedServiceInference{
					Mode: "node_local",
				},
			}
			args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", tt.runtime)
			argv := strings.Join(args, " ")
			if !strings.Contains(argv, tt.wantOllamaURL) {
				t.Fatalf("expected %s runtime default OLLAMA_BASE_URL, got args=%q", tt.runtime, argv)
			}
		})
	}
}

func TestBuildManagedServiceRunArgs_InferenceEnvUsesContainerHostAliasOverride(t *testing.T) {
	original := os.Getenv("CONTAINER_HOST_ALIAS")
	_ = os.Setenv("CONTAINER_HOST_ALIAS", "custom.host.internal")
	defer func() {
		if original == "" {
			_ = os.Unsetenv("CONTAINER_HOST_ALIAS")
		} else {
			_ = os.Setenv("CONTAINER_HOST_ALIAS", original)
		}
	}()
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID:   "pma-main",
		ServiceType: "pma",
		Image:       "pma:latest",
		Inference: &nodepayloads.ConfigManagedServiceInference{
			Mode: "node_local",
		},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "docker")
	argv := strings.Join(args, " ")
	if !strings.Contains(argv, "OLLAMA_BASE_URL=http://custom.host.internal:11434") {
		t.Fatalf("expected CONTAINER_HOST_ALIAS override in OLLAMA_BASE_URL, got args=%q", argv)
	}
}

func TestBuildManagedServiceRunArgs_InferenceEnvExternalHints(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID:   "pma-main",
		ServiceType: "pma",
		Image:       "pma:latest",
		Inference: &nodepayloads.ConfigManagedServiceInference{
			Mode:             "external",
			APIEgressBaseURL: "http://api-egress.internal:12084",
			ProviderID:       "openai",
			DefaultModel:     "gpt-4o-mini",
		},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "podman")
	argv := strings.Join(args, " ")
	if !strings.Contains(argv, "API_EGRESS_BASE_URL=http://api-egress.internal:12084") {
		t.Fatalf("expected API_EGRESS_BASE_URL for external mode, got args=%q", argv)
	}
	if !strings.Contains(argv, "INFERENCE_PROVIDER_ID=openai") {
		t.Fatalf("expected INFERENCE_PROVIDER_ID for external mode, got args=%q", argv)
	}
	if !strings.Contains(argv, "INFERENCE_MODEL=gpt-4o-mini") {
		t.Fatalf("expected INFERENCE_MODEL for external mode, got args=%q", argv)
	}
}
