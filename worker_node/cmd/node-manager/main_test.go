package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
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

func TestRunMainContextCancelled(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
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

func TestStartOllama_Success(t *testing.T) {
	_ = os.Setenv("CONTAINER_RUNTIME", "podman")
	_ = os.Setenv("OLLAMA_IMAGE", "alpine")
	defer func() {
		_ = os.Unsetenv("CONTAINER_RUNTIME")
		_ = os.Unsetenv("OLLAMA_IMAGE")
	}()
	err := startOllama()
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
	err := startOllama()
	if err != nil {
		t.Errorf("startOllama when container exists: %v", err)
	}
}
