package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/worker_node/internal/nodemanager"
)

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_NM_ENV")
	if getEnv("TEST_NM_ENV", "def") != "def" {
		t.Error("expected default")
	}
	_ = os.Setenv("TEST_NM_ENV", "val")
	defer func() { _ = os.Unsetenv("TEST_NM_ENV") }()
	if getEnv("TEST_NM_ENV", "def") != "val" {
		t.Error("expected from env")
	}
}

func TestEffectiveStateDir(t *testing.T) {
	_ = os.Unsetenv("WORKER_API_STATE_DIR")
	_ = os.Unsetenv("CYNODE_STATE_DIR")
	defer func() {
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		_ = os.Unsetenv("CYNODE_STATE_DIR")
	}()
	if got := effectiveStateDir(); got != "/var/lib/cynode/state" {
		t.Errorf("default state dir: got %q", got)
	}
	_ = os.Setenv("CYNODE_STATE_DIR", "/tmp/cynode-state")
	if got := effectiveStateDir(); got != "/tmp/cynode-state" {
		t.Errorf("CYNODE_STATE_DIR: got %q", got)
	}
	_ = os.Setenv("WORKER_API_STATE_DIR", "/tmp/worker-state")
	if got := effectiveStateDir(); got != "/tmp/worker-state" {
		t.Errorf("WORKER_API_STATE_DIR precedence: got %q", got)
	}
}

func TestServiceIDPathSafe(t *testing.T) {
	for _, tt := range []struct {
		id   string
		safe bool
	}{
		{"", false},
		{"pma-main", true},
		{"svc_a", true},
		{"a/b", false},
		{"..", false},
		{"a..b", false},
		{"a\\b", false},
	} {
		if got := nodemanager.ServiceIDPathSafe(tt.id); got != tt.safe {
			t.Errorf("serviceIDPathSafe(%q)=%v, want %v", tt.id, got, tt.safe)
		}
	}
}

// TestBuildManagedServiceRunArgs_NoSecretsMount asserts that managed service container run args
// never mount the secure store path (per CYNAI.WORKER.NodeLocalSecureStore and process boundary).
func TestBuildManagedServiceRunArgs_NoSecretsMount(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	defer func() { _ = os.Unsetenv("WORKER_API_STATE_DIR") }()

	svc := &nodepayloads.ConfigManagedService{
		ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
		Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
	}
	args := buildManagedServiceRunArgs(svc, "pma-main", "pma", "pma:latest", "cynodeai-managed-pma-main")

	for i := 0; i < len(args)-1; i++ {
		if args[i] != "-v" {
			continue
		}
		mount := args[i+1]
		hostPath, _, _ := strings.Cut(mount, ":")
		if strings.Contains(hostPath, "secrets") {
			t.Errorf("managed service mount must not include secure store path: got host path %q", hostPath)
		}
	}
}

func TestRunMain_DebugLevel(t *testing.T) {
	_ = os.Setenv("NODE_MANAGER_DEBUG", "1")
	_ = os.Setenv("NODE_SLUG", "")
	_ = os.Setenv("ORCHESTRATOR_URL", "http://x")
	_ = os.Setenv("NODE_REGISTRATION_PSK", "psk")
	defer func() {
		_ = os.Unsetenv("NODE_MANAGER_DEBUG")
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
	}()
	code := runMain(context.Background())
	if code != 1 {
		t.Errorf("runMain with invalid config: got %d", code)
	}
}

func TestRunMainValidateFails(t *testing.T) {
	_ = os.Setenv("NODE_SLUG", "")
	_ = os.Setenv("ORCHESTRATOR_URL", "http://x")
	_ = os.Setenv("NODE_REGISTRATION_PSK", "psk")
	defer func() {
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
	}()

	ctx := context.Background()
	code := runMain(ctx)
	if code != 1 {
		t.Errorf("runMain should return 1 when config invalid, got %d", code)
	}
}

func TestRunMainContextCanceled(t *testing.T) {
	_ = os.Setenv("NODE_SLUG", "x")
	_ = os.Setenv("ORCHESTRATOR_URL", "http://127.0.0.1:1")
	_ = os.Setenv("NODE_REGISTRATION_PSK", "psk")
	_ = os.Setenv("HTTP_TIMEOUT", "1ms")
	defer func() {
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
		_ = os.Unsetenv("HTTP_TIMEOUT")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	code := runMain(ctx)
	if code != 1 {
		t.Errorf("runMain should return 1 when register fails, got %d", code)
	}
}

func TestRunMainSuccess(t *testing.T) {
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/nodes/register" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(nodepayloads.BootstrapResponse{
				Version:  1,
				IssuedAt: time.Now().UTC().Format(time.RFC3339),
				Orchestrator: nodepayloads.BootstrapOrchestrator{
					Endpoints: nodepayloads.BootstrapEndpoints{
						NodeReportURL: baseURL + "/v1/nodes/capability",
						NodeConfigURL: baseURL + "/v1/nodes/config",
					},
				},
				Auth: nodepayloads.BootstrapAuth{NodeJWT: "jwt", ExpiresAt: "2026-01-01T00:00:00Z"},
			})
			return
		}
		if r.URL.Path == "/v1/nodes/config" {
			if r.Method == "GET" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(nodepayloads.NodeConfigurationPayload{
					Version:       1,
					ConfigVersion: "1",
					IssuedAt:      time.Now().UTC().Format(time.RFC3339),
					NodeSlug:      "x",
					WorkerAPI:     &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "test-token"},
				})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/v1/nodes/capability" {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	baseURL = srv.URL

	_ = os.Setenv("ORCHESTRATOR_URL", srv.URL)
	_ = os.Setenv("NODE_SLUG", "x")
	_ = os.Setenv("NODE_REGISTRATION_PSK", "psk")
	_ = os.Setenv("CAPABILITY_REPORT_INTERVAL", "1h")
	_ = os.Setenv("HTTP_TIMEOUT", "5s")
	_ = os.Setenv("NODE_MANAGER_SKIP_SERVICES", "1")
	defer func() {
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
		_ = os.Unsetenv("CAPABILITY_REPORT_INTERVAL")
		_ = os.Unsetenv("HTTP_TIMEOUT")
		_ = os.Unsetenv("NODE_MANAGER_SKIP_SERVICES")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	code := runMain(ctx)
	if code != 0 {
		t.Errorf("runMain should return 0 on success, got %d", code)
	}
}

func TestStartWorkerAPI_Success(t *testing.T) {
	_ = os.Setenv("NODE_MANAGER_WORKER_API_BIN", "true")
	defer func() { _ = os.Unsetenv("NODE_MANAGER_WORKER_API_BIN") }()
	err := startWorkerAPI("secret-token")
	if err != nil {
		t.Errorf("startWorkerAPI with true binary: %v", err)
	}
}

func TestStartWorkerAPI_BinaryNotFound(t *testing.T) {
	_ = os.Setenv("NODE_MANAGER_WORKER_API_BIN", "/nonexistent/binary")
	defer func() { _ = os.Unsetenv("NODE_MANAGER_WORKER_API_BIN") }()
	err := startWorkerAPI("token")
	if err == nil {
		t.Error("startWorkerAPI should fail when binary not found")
	}
}

func TestStartWorkerAPI_AbsolutePath(t *testing.T) {
	path, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true binary not in PATH")
	}
	_ = os.Setenv("NODE_MANAGER_WORKER_API_BIN", path)
	defer func() { _ = os.Unsetenv("NODE_MANAGER_WORKER_API_BIN") }()
	err = startWorkerAPI("token")
	if err != nil {
		t.Errorf("startWorkerAPI with absolute path: %v", err)
	}
}

func TestStartWorkerAPI_LookPathFails(t *testing.T) {
	_ = os.Setenv("NODE_MANAGER_WORKER_API_BIN", "nonexistent-binary-xyz-no-slash")
	defer func() { _ = os.Unsetenv("NODE_MANAGER_WORKER_API_BIN") }()
	err := startWorkerAPI("token")
	if err == nil {
		t.Error("startWorkerAPI should fail when LookPath fails for non-absolute binary")
	}
}

func TestStartOllama_Success(t *testing.T) {
	_ = os.Setenv("CONTAINER_RUNTIME", "podman")
	_ = os.Setenv("OLLAMA_IMAGE", "alpine")
	defer func() {
		_ = os.Unsetenv("CONTAINER_RUNTIME")
		_ = os.Unsetenv("OLLAMA_IMAGE")
	}()
	err := startOllama("alpine", "")
	if err != nil {
		t.Skipf("startOllama requires podman and alpine image: %v", err)
	}
	// Clean up container so we do not leave it behind.
	_ = exec.Command("podman", "rm", "-f", "cynodeai-ollama").Run()
}

func TestStartOllama_ContainerExists(t *testing.T) {
	_ = os.Setenv("CONTAINER_RUNTIME", "podman")
	defer func() { _ = os.Unsetenv("CONTAINER_RUNTIME") }()
	_ = exec.Command("podman", "rm", "-f", "cynodeai-ollama").Run()
	create := exec.Command("podman", "run", "-d", "--name", "cynodeai-ollama", "alpine", "sleep", "300")
	if out, err := create.CombinedOutput(); err != nil {
		t.Skipf("podman or alpine not available: %v %s", err, out)
	}
	defer func() { _ = exec.Command("podman", "rm", "-f", "cynodeai-ollama").Run() }()
	err := startOllama("", "")
	if err != nil {
		t.Errorf("startOllama when container exists: %v", err)
	}
}

func TestStartOllama_RuntimeFailureCases(t *testing.T) {
	for _, tt := range []struct {
		name    string
		image   string
		variant string
	}{
		{"default image", "ollama/ollama", ""},
		{"custom image and variant", "custom-ollama", "rocm"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("CONTAINER_RUNTIME", "false")
			_ = os.Setenv("OLLAMA_IMAGE", tt.image)
			defer func() {
				_ = os.Unsetenv("CONTAINER_RUNTIME")
				_ = os.Unsetenv("OLLAMA_IMAGE")
			}()
			if err := startOllama("", tt.variant); err == nil {
				t.Error("startOllama should fail when runtime fails")
			}
		})
	}
}

func TestSanitizeContainerName(t *testing.T) {
	if got := sanitizeContainerName(" pma main "); got != "pma_main" {
		t.Fatalf("unexpected sanitized name: %q", got)
	}
	if got := sanitizeContainerName("x/y\\z"); got != "xyz" {
		t.Fatalf("unexpected sanitized name: %q", got)
	}
}

func TestDefaultPortForServiceType(t *testing.T) {
	if got := nodemanager.DefaultPortForServiceType("pma"); got != "8090" {
		t.Fatalf("unexpected PMA port: %q", got)
	}
	if got := nodemanager.DefaultPortForServiceType("unknown"); got != "" {
		t.Fatalf("unexpected unknown service port: %q", got)
	}
}

func writeFakeRuntime(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-runtime.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}
	return path
}

func TestStartManagedServices_InjectsProxyEnvWithoutAgentToken(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "runtime-args.log")
	rt := writeFakeRuntime(t, fmt.Sprintf(`#!/bin/sh
if [ "$1" = "ps" ]; then
  exit 0
fi
if [ "$1" = "run" ]; then
  printf '%%s\n' "$@" > "%s"
  exit 0
fi
if [ "$1" = "start" ]; then
  exit 0
fi
exit 0
`, logPath))
	t.Setenv("CONTAINER_RUNTIME", rt)
	services := []nodepayloads.ConfigManagedService{
		{
			ServiceID:   "pma-main",
			ServiceType: "pma",
			Image:       "ghcr.io/example/pma:latest",
			RestartPolicy: "always",
			Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
				MCPGatewayProxyURL:    "http://127.0.0.1:9191/v1/worker/internal/orchestrator/mcp:call",
				ReadyCallbackProxyURL: "http://127.0.0.1:9191/v1/worker/internal/orchestrator/agent:ready",
				AgentToken:            "must-not-be-injected",
			},
		},
	}
	if err := startManagedServices(services); err != nil {
		t.Fatalf("startManagedServices failed: %v", err)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read runtime args log: %v", err)
	}
	args := string(raw)
	if !strings.Contains(args, "MCP_GATEWAY_PROXY_URL=") || !strings.Contains(args, "READY_CALLBACK_PROXY_URL=") {
		t.Fatalf("missing expected proxy env args: %s", args)
	}
	if strings.Contains(args, "AGENT_TOKEN=") || strings.Contains(args, "must-not-be-injected") {
		t.Fatalf("agent token must not be injected in container args: %s", args)
	}
}

func TestStartManagedServices_ExistingContainerStartsInsteadOfRun(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "runtime-ops.log")
	rt := writeFakeRuntime(t, fmt.Sprintf(`#!/bin/sh
if [ "$1" = "ps" ]; then
  if [ "$2" = "-a" ]; then
    echo cynodeai-managed-pma-main
  fi
  exit 0
fi
if [ "$1" = "start" ]; then
  echo start >> "%s"
  exit 0
fi
if [ "$1" = "run" ]; then
  echo run >> "%s"
  exit 0
fi
exit 0
`, logPath, logPath))
	t.Setenv("CONTAINER_RUNTIME", rt)
	services := []nodepayloads.ConfigManagedService{
		{
			ServiceID:   "pma-main",
			ServiceType: "pma",
			Image:       "ghcr.io/example/pma:latest",
		},
	}
	if err := startManagedServices(services); err != nil {
		t.Fatalf("startManagedServices failed: %v", err)
	}
	raw, _ := os.ReadFile(logPath)
	if strings.Contains(string(raw), "run") {
		t.Fatalf("expected no run when container already exists: %s", string(raw))
	}
	if !strings.Contains(string(raw), "start") {
		t.Fatalf("expected start operation when container exists: %s", string(raw))
	}
}

func TestStartManagedServices_RunFailure(t *testing.T) {
	rt := writeFakeRuntime(t, `#!/bin/sh
if [ "$1" = "ps" ]; then
  exit 0
fi
if [ "$1" = "run" ]; then
  echo fail >&2
  exit 1
fi
exit 0
`)
	t.Setenv("CONTAINER_RUNTIME", rt)
	services := []nodepayloads.ConfigManagedService{
		{ServiceID: "pma-main", ServiceType: "pma", Image: "ghcr.io/example/pma:latest"},
	}
	if err := startManagedServices(services); err == nil {
		t.Fatal("expected startManagedServices to fail on runtime run error")
	}
}
