// Package main: tests for node-manager cmd.
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func unsetRunMainTestEnv() {
	_ = os.Unsetenv("WORKER_API_STATE_DIR")
	_ = os.Unsetenv("ORCHESTRATOR_URL")
	_ = os.Unsetenv("NODE_SLUG")
	_ = os.Unsetenv("NODE_REGISTRATION_PSK")
	_ = os.Unsetenv("LISTEN_ADDR")
	_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
	_ = os.Unsetenv("NODE_MANAGER_SKIP_CONTAINER_CHECK")
	_ = os.Unsetenv("NODE_MANAGER_TEST_NO_EXISTING_INFERENCE")
	_ = os.Unsetenv("CYNODE_SECURE_STORE_MASTER_KEY_B64")
}

func runMainUntilCancel(t *testing.T, sleep time.Duration) int {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- runMain(ctx) }()
	t.Cleanup(cancel)
	time.Sleep(sleep)
	cancel()
	return <-done
}

func cleanupEmbeddedWorkerAPIShutdown() {
	if embeddedWorkerAPIShutdown != nil {
		embeddedWorkerAPIShutdown()
		embeddedWorkerAPIShutdown = nil
	}
}

// TestRunMain_FailsWithStartManagedServicesError_LogsComponent covers runMain when StartManagedServices fails (component "managed_services" branch).
func TestRunMain_FailsWithStartManagedServicesError_LogsComponent(t *testing.T) {
	var baseURL string
	configWithManagedService := nodepayloads.NodeConfigurationPayload{
		Version: 1, ConfigVersion: "1", IssuedAt: time.Now().UTC().Format(time.RFC3339), NodeSlug: "test",
		WorkerAPI:        &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "bearer"},
		InferenceBackend: &nodepayloads.ConfigInferenceBackend{Enabled: true, Image: "ollama/ollama"},
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-main", ServiceType: "pma", Image: "cynodeai-pma:dev"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakeOrchestratorHandler(baseURL, &configWithManagedService)(w, r)
	}))
	defer srv.Close()
	baseURL = srv.URL

	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)
	t.Setenv("NODE_SLUG", "test-node")
	t.Setenv("NODE_REGISTRATION_PSK", "test-psk")
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("NODE_MANAGER_SKIP_CONTAINER_CHECK", "1")
	t.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	defer unsetRunMainTestEnv()

	withRunner(t, fakeRunnerFunc(fakeRunnerFailRunContaining("cynodeai-managed-")))

	code := runMainUntilCancel(t, 4*time.Second)
	if code != 1 {
		t.Errorf("runMain expected 1 (start managed services failure), got %d", code)
	}
	cleanupEmbeddedWorkerAPIShutdown()
}

func TestPullModels_ExecsPullForEachModel(t *testing.T) {
	var calls [][]string
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte(""), nil
	})
	withRunner(t, fake)
	_ = os.Setenv("OLLAMA_CONTAINER_NAME", "cynodeai-ollama")
	defer func() { _ = os.Unsetenv("OLLAMA_CONTAINER_NAME") }()
	if err := pullModels([]string{"qwen3.5:9b", "qwen3:8b"}); err != nil {
		t.Fatalf("pullModels: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 exec calls, got %d: %v", len(calls), calls)
	}
	for i, model := range []string{"qwen3.5:9b", "qwen3:8b"} {
		found := false
		for _, a := range calls[i] {
			if a == model {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("call %d: expected model %q in args %v", i, model, calls[i])
		}
	}
}

func TestPullModels_SkipsEmptyModelName(t *testing.T) {
	var calls [][]string
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte(""), nil
	})
	withRunner(t, fake)
	if err := pullModels([]string{"", "qwen3:8b"}); err != nil {
		t.Fatalf("pullModels: %v", err)
	}
	if len(calls) != 1 {
		t.Errorf("expected 1 call (empty model skipped), got %d: %v", len(calls), calls)
	}
}

func TestPullModels_ReturnsFirstError(t *testing.T) {
	fake := fakeRunnerFunc(func(_ string, args ...string) ([]byte, error) {
		return []byte("no such container"), errors.New("exit status 1")
	})
	withRunner(t, fake)
	err := pullModels([]string{"qwen3.5:9b"})
	if err == nil {
		t.Error("expected error when exec fails, got nil")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestRun_PrintGPUDetect(t *testing.T) {
	var code int
	out := captureStdout(t, func() { code = run(context.Background(), []string{"--print-gpu-detect"}) })
	if code != 0 {
		t.Errorf("run(--print-gpu-detect) = %d", code)
	}
	if !strings.Contains(out, "merged_detect_gpu") {
		t.Errorf("expected JSON with merged_detect_gpu: %q", out[:min(200, len(out))])
	}
}

func TestRun_PrintSBARunArgs(t *testing.T) {
	var code int
	out := captureStdout(t, func() { code = run(context.Background(), []string{"--print-sba-run-args"}) })
	if code != 0 {
		t.Errorf("run(--print-sba-run-args) = %d", code)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(out, "run") {
		t.Errorf("output should contain run args: %q", out)
	}
}

func TestRun_PrintSBARunArgsWithFlags(t *testing.T) {
	out := captureStdout(t, func() {
		code := run(context.Background(), []string{"--print-sba-run-args", "--sba-image", "custom:tag", "--upstream-url", "http://custom:11434"})
		if code != 0 {
			t.Errorf("code = %d", code)
		}
	})
	if !strings.Contains(out, "custom:tag") {
		t.Errorf("expected custom image in output: %q", out)
	}
}

func TestRun_PrintSBAPodRunArgs(t *testing.T) {
	var code int
	out := captureStdout(t, func() { code = run(context.Background(), []string{"--print-sba-pod-run-args"}) })
	if code != 0 {
		t.Errorf("run(--print-sba-pod-run-args) = %d", code)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestRun_PrintManagedServiceRunArgs(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	defer func() { _ = os.Unsetenv("WORKER_API_STATE_DIR") }()
	var code int
	out := captureStdout(t, func() { code = run(context.Background(), []string{"--print-managed-service-run-args"}) })
	if code != 0 {
		t.Errorf("run(--print-managed-service-run-args) = %d", code)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(out, "run") {
		t.Errorf("output should contain run: %q", out)
	}
}

func TestRun_PrintManagedServiceRunArgsWithFlags(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	defer func() { _ = os.Unsetenv("WORKER_API_STATE_DIR") }()
	out := captureStdout(t, func() {
		code := run(context.Background(), []string{"--print-managed-service-run-args", "--service-id", "my-pma", "--service-type", "pma", "--service-image", "myimg:latest"})
		if code != 0 {
			t.Errorf("code = %d", code)
		}
	})
	if !strings.Contains(out, "myimg:latest") {
		t.Errorf("expected service image in output: %q", out)
	}
}

func TestRun_NoFlagCallsRunMain(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	t.Setenv("ORCHESTRATOR_URL", "http://127.0.0.1:19999")
	t.Setenv("NODE_SLUG", "test-node")
	t.Setenv("NODE_REGISTRATION_PSK", "test-psk")
	t.Setenv("NODE_MANAGER_SKIP_SERVICES", "1")
	defer func() {
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
		_ = os.Unsetenv("NODE_MANAGER_SKIP_SERVICES")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- run(ctx, []string{}) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	code := <-done
	if code != 1 {
		t.Errorf("run(no args) should delegate to runMain and get 1, got %d", code)
	}
}
