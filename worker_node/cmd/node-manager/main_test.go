// Package main: tests for node-manager cmd.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/worker_node/internal/telemetry"
)

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("NODE_MANAGER_TEST_GETENV")
	if got := getEnv("NODE_MANAGER_TEST_GETENV", "default"); got != "default" {
		t.Errorf("getEnv default = %q", got)
	}
	_ = os.Setenv("NODE_MANAGER_TEST_GETENV", "set")
	defer func() { _ = os.Unsetenv("NODE_MANAGER_TEST_GETENV") }()
	if got := getEnv("NODE_MANAGER_TEST_GETENV", "default"); got != "set" {
		t.Errorf("getEnv set = %q", got)
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
		t.Errorf("default = %q", got)
	}
	_ = os.Setenv("CYNODE_STATE_DIR", "/cynode-state")
	if got := effectiveStateDir(); got != "/cynode-state" {
		t.Errorf("CYNODE_STATE_DIR = %q", got)
	}
	_ = os.Setenv("WORKER_API_STATE_DIR", "/worker-state")
	if got := effectiveStateDir(); got != "/worker-state" {
		t.Errorf("WORKER_API_STATE_DIR = %q", got)
	}
}

func TestSanitizeContainerName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"pma-main", "pma-main"},
		{"svc_1", "svc_1"},
		{"a.b", "a.b"},
		{"  spaces  ", "spaces"}, // TrimSpace then map: leading/trailing removed
		{"mixed space", "mixed_space"},
	}
	for _, tt := range tests {
		got := sanitizeContainerName(tt.in)
		if got != tt.want {
			t.Errorf("sanitizeContainerName(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
	// Invalid chars dropped
	got := sanitizeContainerName("bad/slash")
	if strings.Contains(got, "/") {
		t.Errorf("sanitizeContainerName should drop slash: %q", got)
	}
}

func TestRecordNodeBootTelemetry(t *testing.T) {
	ctx := context.Background()
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("telemetry Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	logger := slog.Default()
	_ = os.Setenv("NODE_BOOT_ID", "test-boot-id")
	_ = os.Setenv("NODE_SLUG", "test-node")
	defer func() {
		_ = os.Unsetenv("NODE_BOOT_ID")
		_ = os.Unsetenv("NODE_SLUG")
	}()
	if err := recordNodeBootTelemetry(ctx, store, logger); err != nil {
		t.Errorf("recordNodeBootTelemetry: %v", err)
	}
	// With nil logger (branch coverage)
	_ = os.Unsetenv("NODE_BOOT_ID")
	if err := recordNodeBootTelemetry(ctx, store, nil); err != nil {
		t.Errorf("recordNodeBootTelemetry nil logger: %v", err)
	}
}

func TestRecordNodeManagerShutdown(t *testing.T) {
	ctx := context.Background()
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("telemetry Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	logger := slog.Default()
	recordNodeManagerShutdown(ctx, store, logger)
	recordNodeManagerShutdown(ctx, store, nil)
}

func TestRunTelemetryRetentionAndVacuum(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("telemetry Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	logger := slog.Default()
	done := make(chan struct{})
	go func() {
		runTelemetryRetentionAndVacuum(ctx, store, logger)
		close(done)
	}()
	cancel()
	<-done
}

func TestRunTelemetryRetentionAndVacuum_TickerBranches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("telemetry Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	oldRet, oldVac := retentionTickerInterval, vacuumTickerInterval
	retentionTickerInterval = 2 * time.Millisecond
	vacuumTickerInterval = 5 * time.Millisecond
	defer func() { retentionTickerInterval, vacuumTickerInterval = oldRet, oldVac }()
	done := make(chan struct{})
	go func() {
		runTelemetryRetentionAndVacuum(ctx, store, slog.Default())
		close(done)
	}()
	time.Sleep(15 * time.Millisecond)
	cancel()
	<-done
}

func TestBuildManagedServiceRunArgs(t *testing.T) {
	_ = os.Setenv("WORKER_API_STATE_DIR", t.TempDir())
	defer func() { _ = os.Unsetenv("WORKER_API_STATE_DIR") }()
	svc := &nodepayloads.ConfigManagedService{
		ServiceID:   "pma-main",
		ServiceType: "pma",
		Image:       "example/pma:latest",
	}
	args := buildManagedServiceRunArgs("podman", svc, "pma-main", "pma", "example/pma:latest", "cynodeai-managed-pma-main")
	if len(args) == 0 {
		t.Error("buildManagedServiceRunArgs returned empty")
	}
	if args[0] != "run" {
		t.Errorf("expected run first, got %q", args[0])
	}
}

func TestRunMain_FailsWhenOrchestratorUnreachable(t *testing.T) {
	stateDir := t.TempDir()
	_ = os.Setenv("WORKER_API_STATE_DIR", stateDir)
	_ = os.Setenv("ORCHESTRATOR_URL", "http://127.0.0.1:19999")
	_ = os.Setenv("NODE_SLUG", "test-node")
	_ = os.Setenv("NODE_REGISTRATION_PSK", "test-psk")
	_ = os.Setenv("NODE_MANAGER_SKIP_SERVICES", "1")
	defer func() {
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
		_ = os.Unsetenv("NODE_MANAGER_SKIP_SERVICES")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- runMain(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	code := <-done
	if code != 1 {
		t.Errorf("runMain should return 1 when orchestrator unreachable, got %d", code)
	}
}

func TestRunMain_DebugLogLevel(t *testing.T) {
	stateDir := t.TempDir()
	_ = os.Setenv("WORKER_API_STATE_DIR", stateDir)
	_ = os.Setenv("ORCHESTRATOR_URL", "http://127.0.0.1:19998")
	_ = os.Setenv("NODE_SLUG", "test-node")
	_ = os.Setenv("NODE_REGISTRATION_PSK", "test-psk")
	_ = os.Setenv("NODE_MANAGER_SKIP_SERVICES", "1")
	_ = os.Setenv("NODE_MANAGER_DEBUG", "1")
	defer func() {
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
		_ = os.Unsetenv("NODE_MANAGER_SKIP_SERVICES")
		_ = os.Unsetenv("NODE_MANAGER_DEBUG")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- runMain(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	code := <-done
	if code != 1 {
		t.Errorf("runMain with debug: got %d", code)
	}
}

type fakeRunner struct {
	lookPathErr    error
	combinedOutput []byte
	combinedErr    error
	startErr       error
}

func (f fakeRunner) LookPath(string) (string, error) {
	if f.lookPathErr != nil {
		return "", f.lookPathErr
	}
	return "/fake/worker-api", nil
}
func (f fakeRunner) CombinedOutput(string, ...string) ([]byte, error) {
	return f.combinedOutput, f.combinedErr
}
func (f fakeRunner) StartDetached(string, []string, []string) error {
	return f.startErr
}

// fakeRunnerFunc is a cmdRunner whose CombinedOutput delegates to a user-supplied function,
// useful for tests that need different responses per call.
type fakeRunnerFunc func(name string, args ...string) ([]byte, error)

func (f fakeRunnerFunc) LookPath(bin string) (string, error) { return "/fake/" + bin, nil }
func (f fakeRunnerFunc) CombinedOutput(name string, args ...string) ([]byte, error) {
	return f(name, args...)
}
func (fakeRunnerFunc) StartDetached(string, []string, []string) error { return nil }

func withRunner(t *testing.T, r cmdRunner) {
	t.Helper()
	old := runner
	t.Cleanup(func() { runner = old })
	runner = r
}

func TestStartWorkerAPIBinary(t *testing.T) {
	withRunner(t, fakeRunner{})

	if err := startWorkerAPIBinary("token"); err != nil {
		t.Errorf("startWorkerAPIBinary: %v", err)
	}
}

func TestStartWorkerAPIBinary_LookPathFails(t *testing.T) {
	withRunner(t, fakeRunner{lookPathErr: os.ErrNotExist})

	if err := startWorkerAPIBinary("token"); err == nil {
		t.Error("expected error when LookPath fails")
	}
}

func TestStartWorkerAPIBinary_RealRunner(t *testing.T) {
	withRunner(t, realCmdRunner{})

	_ = os.Setenv("NODE_MANAGER_WORKER_API_BIN", "true")
	defer func() { _ = os.Unsetenv("NODE_MANAGER_WORKER_API_BIN") }()

	if err := startWorkerAPIBinary("token"); err != nil {
		t.Errorf("startWorkerAPIBinary with real runner: %v", err)
	}
}

func TestStartWorkerAPI_BinaryAndContainerPath(t *testing.T) {
	tests := []struct {
		name    string
		setup   func()
		wantErr bool
	}{
		{
			name: "binary",
			setup: func() {
				_ = os.Unsetenv("NODE_MANAGER_WORKER_API_IMAGE")
			},
		},
		{
			name: "container",
			setup: func() {
				_ = os.Setenv("NODE_MANAGER_WORKER_API_IMAGE", "worker-api:test")
				t.Cleanup(func() { _ = os.Unsetenv("NODE_MANAGER_WORKER_API_IMAGE") })
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withRunner(t, fakeRunner{})
			tt.setup()
			err := startWorkerAPI("token")
			if (err != nil) != tt.wantErr {
				t.Errorf("startWorkerAPI() err = %v", err)
			}
		})
	}
}

func TestStartWorkerAPIContainer_ExistingContainer(t *testing.T) {
	withRunner(t, fakeRunner{
		combinedOutput: []byte("cynodeai-worker-api\n"),
	})

	if err := startWorkerAPIContainer("img", "tok"); err != nil {
		t.Errorf("startWorkerAPIContainer existing: %v", err)
	}
}

func TestStartWorkerAPIContainer_StartExistingFails(t *testing.T) {
	withRunner(t, fakeRunner{
		combinedOutput: []byte("cynodeai-worker-api\n"),
		combinedErr:    fmt.Errorf("start failed"),
	})

	if err := startWorkerAPIContainer("img", "tok"); err == nil {
		t.Error("expected error when start existing fails")
	}
}

func TestStartWorkerAPIContainer_RunNew(t *testing.T) {
	withRunner(t, fakeRunner{}) // ps returns empty, so we run new container

	if err := startWorkerAPIContainer("img", "tok"); err != nil {
		t.Errorf("startWorkerAPIContainer run new: %v", err)
	}
}

func TestStartWorkerAPIContainer_RunNewFails(t *testing.T) {
	withRunner(t, fakeRunner{combinedErr: fmt.Errorf("run failed")})

	if err := startWorkerAPIContainer("img", "tok"); err == nil {
		t.Error("expected error when run new fails")
	}
}

func TestStartOllama(t *testing.T) {
	withRunner(t, fakeRunner{})

	if err := startOllama("ollama/ollama", "", nil); err != nil {
		t.Errorf("startOllama: %v", err)
	}
}

func TestStartOllama_ROCmImage_HasDeviceArgs(t *testing.T) {
	var calls [][]string
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte(""), nil
	})
	withRunner(t, fake)
	if err := startOllama("ollama/ollama:rocm", "", nil); err != nil {
		t.Fatalf("startOllama rocm: %v", err)
	}
	var sawKFD bool
	for _, c := range calls {
		for _, a := range c {
			if a == "/dev/kfd" {
				sawKFD = true
			}
		}
	}
	if !sawKFD {
		t.Errorf("expected /dev/kfd device arg for ROCm image; calls=%v", calls)
	}
}

func TestStartOllama_CUDAVariant_HasGPUsArg(t *testing.T) {
	var calls [][]string
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte(""), nil
	})
	withRunner(t, fake)
	t.Setenv("CONTAINER_RUNTIME", "docker")
	if err := startOllama("ollama/ollama:cuda", "", nil); err != nil {
		t.Fatalf("startOllama cuda: %v", err)
	}
	var sawGPUs bool
	for _, c := range calls {
		for _, a := range c {
			if a == "--gpus" || a == "all" {
				sawGPUs = true
			}
		}
	}
	if !sawGPUs {
		t.Errorf("expected --gpus all for CUDA image (docker runtime); calls=%v", calls)
	}
}

func TestStartOllama_RunFails(t *testing.T) {
	withRunner(t, fakeRunner{combinedErr: fmt.Errorf("ollama run failed")})

	if err := startOllama("img", "", nil); err == nil {
		t.Error("expected error when run fails")
	}
}

func TestStartOllama_ExistingContainer(t *testing.T) {
	withRunner(t, fakeRunner{
		combinedOutput: []byte("cynodeai-ollama\n"),
	})

	if err := startOllama("", "", nil); err != nil {
		t.Errorf("startOllama existing: %v", err)
	}
}

func TestStartOllama_EnvVarsPassedAsFlags(t *testing.T) {
	var runArgs []string
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "run" {
			runArgs = append([]string{name}, args...)
		}
		return []byte(""), nil
	})
	withRunner(t, fake)
	env := map[string]string{
		"OLLAMA_NUM_CTX": "32768",
	}
	if err := startOllama("ollama/ollama", "", env); err != nil {
		t.Fatalf("startOllama: %v", err)
	}
	// Verify -e OLLAMA_NUM_CTX=32768 appears in the run args.
	var found bool
	for i, a := range runArgs {
		if a == "-e" && i+1 < len(runArgs) && runArgs[i+1] == "OLLAMA_NUM_CTX=32768" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -e OLLAMA_NUM_CTX=32768 in run args; got %v", runArgs)
	}
}

func TestStartManagedServices_SuccessAndSkips(t *testing.T) {
	tests := []struct {
		name     string
		services []nodepayloads.ConfigManagedService
		wantErr  bool
	}{
		{
			name: "one service",
			services: []nodepayloads.ConfigManagedService{
				{ServiceID: "pma-1", ServiceType: "pma", Image: "pma:latest"},
			},
		},
		{
			name: "skip empty id/type/image",
			services: []nodepayloads.ConfigManagedService{
				{ServiceID: "", ServiceType: "pma", Image: "img"},
				{ServiceID: "x", ServiceType: "", Image: "img"},
				{ServiceID: "x", ServiceType: "pma", Image: ""},
			},
		},
		{
			name: "skip whitespace-only id",
			services: []nodepayloads.ConfigManagedService{
				{ServiceID: "  \t  ", ServiceType: "pma", Image: "img"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withRunner(t, fakeRunner{})
			err := startManagedServices(tt.services)
			if (err != nil) != tt.wantErr {
				t.Errorf("startManagedServices() err = %v", err)
			}
		})
	}
}

func TestStartManagedServices_PropagatesError(t *testing.T) {
	withRunner(t, fakeRunner{combinedErr: fmt.Errorf("container run failed")})

	services := []nodepayloads.ConfigManagedService{
		{ServiceID: "pma-1", ServiceType: "pma", Image: "pma:latest"},
	}
	if err := startManagedServices(services); err == nil {
		t.Error("expected error from startOneManagedService")
	}
}

func TestStartOneManagedService_ExistingContainer(t *testing.T) {
	withRunner(t, fakeRunner{
		combinedOutput: []byte("cynodeai-managed-pma-1\n"),
	})

	svc := &nodepayloads.ConfigManagedService{
		ServiceID:   "pma-1",
		ServiceType: "pma",
		Image:       "pma:latest",
	}
	if err := startOneManagedService("podman", svc, "pma-1", "pma", "pma:latest", "cynodeai-managed-pma-1"); err != nil {
		t.Errorf("startOneManagedService existing: %v", err)
	}
}

func TestStartOneManagedService_RunFails(t *testing.T) {
	withRunner(t, fakeRunner{combinedErr: fmt.Errorf("run failed")})

	svc := &nodepayloads.ConfigManagedService{
		ServiceID:   "pma-1",
		ServiceType: "pma",
		Image:       "pma:latest",
	}
	if err := startOneManagedService("podman", svc, "pma-1", "pma", "pma:latest", "cynodeai-managed-pma-1"); err == nil {
		t.Error("expected error when run fails")
	}
}

func TestStopNodeManagedContainers(t *testing.T) {
	var calls [][]string
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		// Simulate one managed container ID returned for the first filter.
		if len(args) > 0 && args[len(args)-1] == "name=cynodeai-managed-" {
			return []byte("abc123\n"), nil
		}
		return []byte(""), nil
	})
	withRunner(t, fake)
	stopNodeManagedContainers(nil) // logger=nil is fine
	// Expect at least one stop and one rm call.
	var sawStop, sawRM bool
	for _, c := range calls {
		if len(c) >= 2 && c[1] == "stop" {
			sawStop = true
		}
		if len(c) >= 2 && c[1] == "rm" {
			sawRM = true
		}
	}
	if !sawStop {
		t.Error("expected at least one 'stop' call for managed container")
	}
	if !sawRM {
		t.Error("expected at least one 'rm' call for managed container")
	}
}

func TestStopNodeManagedContainers_StopAndRmErrors(t *testing.T) {
	// Simulate stop/rm errors for a returned container ID — should still complete without panic.
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[len(args)-1] == "name=cynodeai-managed-" {
			return []byte("abc123\n"), nil
		}
		if len(args) >= 3 && (args[1] == "stop" || args[1] == "rm") {
			return nil, fmt.Errorf("simulated %s error", args[1])
		}
		return []byte(""), nil
	})
	withRunner(t, fake)
	stopNodeManagedContainers(slog.Default()) // should not panic or return error
}

func TestStopNodeManagedContainers_PSError(t *testing.T) {
	// Simulate ps failing — should silently continue (best-effort).
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("simulated ps error")
	})
	withRunner(t, fake)
	stopNodeManagedContainers(nil) // should not panic
}

func TestWaitForPMAReadyUDS_Success(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "service.sock")
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer func() { _ = listener.Close() }()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(listener) }()
	defer func() { _ = srv.Close() }()
	waitForPMAReadyUDS(sockPath, 2*time.Second)
}

func TestRunMain_TelemetryStoreUnavailable(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("WORKER_API_STATE_DIR", blocker)
	_ = os.Setenv("ORCHESTRATOR_URL", "http://127.0.0.1:19997")
	_ = os.Setenv("NODE_SLUG", "test-node")
	_ = os.Setenv("NODE_REGISTRATION_PSK", "test-psk")
	_ = os.Setenv("NODE_MANAGER_SKIP_SERVICES", "1")
	defer func() {
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		_ = os.Unsetenv("ORCHESTRATOR_URL")
		_ = os.Unsetenv("NODE_SLUG")
		_ = os.Unsetenv("NODE_REGISTRATION_PSK")
		_ = os.Unsetenv("NODE_MANAGER_SKIP_SERVICES")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- runMain(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	code := <-done
	if code != 1 {
		t.Errorf("runMain with bad state dir: got %d", code)
	}
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
