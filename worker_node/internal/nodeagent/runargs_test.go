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

// TestDefaultPortForServiceType_NowReturnsEmpty verifies the function always returns ""
// since REQ-WORKER-0174 removed TCP port publishing for managed services.
func TestDefaultPortForServiceType_NowReturnsEmpty(t *testing.T) {
	for _, svcType := range []string{"pma", "PMA", "unknown"} {
		if got := DefaultPortForServiceType(svcType); got != "" {
			t.Errorf("DefaultPortForServiceType(%q) = %q, want empty (REQ-WORKER-0174)", svcType, got)
		}
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
			// PMA uses UDS only; health check must use curl --unix-socket (REQ-WORKER-0174 / REQ-WORKER-0260).
			if !strings.Contains(args[i+1], "curl") || !strings.Contains(args[i+1], "service.sock") || !strings.Contains(args[i+1], "/healthz") {
				t.Errorf("health-cmd for PMA should use curl over UDS (service.sock), got %q", args[i+1])
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

// REQ-WORKER-0260: node_local inference for managed services MUST inject UDS OLLAMA_BASE_URL,
// not a TCP URL. The worker exposes an inference UDS socket per service at
// <state_dir>/run/managed_agent_proxy/<service_id>/inference.sock.
func TestBuildManagedServiceRunArgs_InferenceEnvNodeLocalIsUDS(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID:   "pma-main",
		ServiceType: "pma",
		Image:       "pma:latest",
		Inference: &nodepayloads.ConfigManagedServiceInference{
			Mode:         "node_local",
			DefaultModel: "qwen3.5:0.8b",
		},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "podman")
	argv := strings.Join(args, " ")
	// MUST inject http+unix:// URL, not TCP.
	if !strings.Contains(argv, "OLLAMA_BASE_URL=http+unix://") {
		t.Fatalf("node_local inference must inject http+unix:// OLLAMA_BASE_URL (REQ-WORKER-0260), got args=%q", argv)
	}
	// MUST NOT inject TCP endpoints.
	if strings.Contains(argv, "OLLAMA_BASE_URL=http://") {
		t.Fatalf("node_local inference must not inject TCP OLLAMA_BASE_URL (REQ-WORKER-0260), got args=%q", argv)
	}
	// Socket path must reference the per-service inference socket.
	expectedSockDir := filepath.Join(stateDir, ManagedAgentProxySocketBaseDir, "pma-main")
	if !strings.Contains(argv, expectedSockDir) {
		t.Fatalf("OLLAMA_BASE_URL must reference per-service socket dir %q, got args=%q", expectedSockDir, argv)
	}
	if !strings.Contains(argv, "INFERENCE_MODEL=qwen3.5:0.8b") {
		t.Fatalf("expected INFERENCE_MODEL from config inference.default_model, got args=%q", argv)
	}
}

// REQ-WORKER-0260: when inference.base_url is set (explicit override), it must still be UDS —
// a literal TCP URL is not accepted as a managed-service inference override.
func TestBuildManagedServiceRunArgs_InferenceEnvNodeLocalExplicitURLMustBeUDS(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID:   "pma-main",
		ServiceType: "pma",
		Image:       "pma:latest",
		Inference: &nodepayloads.ConfigManagedServiceInference{
			Mode:    "node_local",
			BaseURL: "http://inference.internal:11434",
		},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "podman")
	argv := strings.Join(args, " ")
	// Even with explicit base_url, the output MUST be UDS (TCP override not honoured per spec).
	if strings.Contains(argv, "OLLAMA_BASE_URL=http://inference.internal:11434") {
		t.Fatalf("node_local inference must not inject TCP OLLAMA_BASE_URL even with explicit base_url (REQ-WORKER-0260), got args=%q", argv)
	}
	if !strings.Contains(argv, "OLLAMA_BASE_URL=http+unix://") {
		t.Fatalf("node_local inference must inject http+unix:// OLLAMA_BASE_URL (REQ-WORKER-0260), got args=%q", argv)
	}
}

// REQ-WORKER-0174: managed-service containers MUST be started with network restriction.
func TestBuildManagedServiceRunArgs_NetworkNone(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID:    "pma-main",
		ServiceType:  "pma",
		Image:        "pma:latest",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "podman")
	found := false
	for _, a := range args {
		if a == "--network=none" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("managed-service run args must include --network=none (REQ-WORKER-0174), got args=%v", args)
	}
}

// REQ-WORKER-0260 / REQ-WORKER-0174: PMA must NOT publish TCP port 8090.
// Orchestrator-to-agent traffic routes through worker proxy, not direct TCP.
func TestBuildManagedServiceRunArgs_NoPMAPortPublish(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID:    "pma-main",
		ServiceType:  "pma",
		Image:        "pma:latest",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "podman")
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-p" && strings.Contains(args[i+1], "8090") {
			t.Errorf("managed-service run args must not publish TCP port 8090 (REQ-WORKER-0174/0260), got -p %s", args[i+1])
		}
	}
}

// REQ-WORKER-0260: per-service inference socket directory is mounted into the container
// (the directory contains both proxy.sock and inference.sock).
func TestBuildManagedServiceRunArgs_InferenceSocketMountedWithProxySocket(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
		Inference:    &nodepayloads.ConfigManagedServiceInference{Mode: "node_local"},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "podman")
	expectedHost := filepath.Join(stateDir, ManagedAgentProxySocketBaseDir, "pma-main")
	var foundMount bool
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-v" {
			hostPath, containerPath, _ := strings.Cut(args[i+1], ":")
			if containerPath == "/run/cynode/managed_agent_proxy" && hostPath == expectedHost {
				foundMount = true
				break
			}
		}
	}
	if !foundMount {
		t.Errorf("per-service UDS directory must be mounted at /run/cynode/managed_agent_proxy; args=%v", args)
	}
}

// TestBuildManagedServiceRunArgs_PMAListenAddrIsUDS asserts that PMA containers receive
// PMA_LISTEN_ADDR=unix:<path> so the PMA binds a UDS socket in the mounted dir.
// Required for --network=none containers (REQ-WORKER-0174).
func TestBuildManagedServiceRunArgs_PMAListenAddrIsUDS(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID:   "pma-main",
		ServiceType: "pma",
		Image:       "pma:latest",
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "podman")
	argv := strings.Join(args, " ")
	if !strings.Contains(argv, "PMA_LISTEN_ADDR=unix:") {
		t.Fatalf("expected PMA_LISTEN_ADDR=unix: in args, got %q", argv)
	}
	if !strings.Contains(argv, "service.sock") {
		t.Fatalf("expected service.sock in PMA_LISTEN_ADDR, got %q", argv)
	}
	if !strings.Contains(argv, inferenceSocketContainerPath) {
		t.Fatalf("expected container mount path %q in PMA_LISTEN_ADDR, got %q", inferenceSocketContainerPath, argv)
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

// TestBuildManagedServiceRunArgs_BackendEnvPassedToContainer asserts that BackendEnv from
// ConfigManagedServiceInference is injected as -e flags into the container run args,
// allowing orchestrator-derived settings (e.g. OLLAMA_NUM_CTX sized to GPU VRAM) to reach
// managed service containers without going through OS-level env of node-manager.
func TestBuildManagedServiceRunArgs_BackendEnvPassedToContainer(t *testing.T) {
	stateDir := t.TempDir()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID:   "pma-main",
		ServiceType: "pma",
		Image:       "pma:latest",
		Inference: &nodepayloads.ConfigManagedServiceInference{
			Mode: "node_local",
			BackendEnv: map[string]string{
				"OLLAMA_NUM_CTX":        "32768",
				"OLLAMA_CONTEXT_LENGTH": "32768",
			},
		},
	}
	args := BuildManagedServiceRunArgs(stateDir, svc, "pma-main", "pma", "pma:latest", "name", "podman")
	argv := strings.Join(args, " ")
	if !strings.Contains(argv, "OLLAMA_NUM_CTX=32768") {
		t.Errorf("expected OLLAMA_NUM_CTX=32768 in run args; got %q", argv)
	}
	if !strings.Contains(argv, "OLLAMA_CONTEXT_LENGTH=32768") {
		t.Errorf("expected OLLAMA_CONTEXT_LENGTH=32768 in run args; got %q", argv)
	}
}

func TestInternalPortForServiceType(t *testing.T) {
	if got := internalPortForServiceType("pma"); got != "8090" {
		t.Errorf("pma port = %q, want 8090", got)
	}
	if got := internalPortForServiceType("unknown"); got != "" {
		t.Errorf("unknown type port = %q, want empty", got)
	}
}
