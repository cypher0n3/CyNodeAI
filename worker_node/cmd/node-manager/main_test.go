// Package main: tests for node-manager cmd.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestRecordNodeBootTelemetry_InsertFails(t *testing.T) {
	ctx := context.Background()
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("telemetry Open: %v", err)
	}
	_ = store.Close()
	if err := recordNodeBootTelemetry(ctx, store, slog.Default()); err == nil {
		t.Error("recordNodeBootTelemetry with closed store should return error")
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

func TestRecordNodeManagerShutdown_InsertFails(t *testing.T) {
	ctx := context.Background()
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("telemetry Open: %v", err)
	}
	_ = store.Close()
	recordNodeManagerShutdown(ctx, store, slog.Default())
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

func TestRunTelemetryRetentionAndVacuum_EnforceRetentionError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("telemetry Open: %v", err)
	}
	_ = store.Close()
	done := make(chan struct{})
	go func() {
		runTelemetryRetentionAndVacuum(ctx, store, slog.Default())
		close(done)
	}()
	cancel()
	<-done
}

func TestRunTelemetryRetentionAndVacuum_TickerFiresWithError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store, err := telemetry.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("telemetry Open: %v", err)
	}
	oldRet, oldVac := retentionTickerInterval, vacuumTickerInterval
	retentionTickerInterval = 3 * time.Millisecond
	vacuumTickerInterval = 5 * time.Millisecond
	defer func() { retentionTickerInterval, vacuumTickerInterval = oldRet, oldVac }()
	done := make(chan struct{})
	go func() {
		runTelemetryRetentionAndVacuum(ctx, store, slog.Default())
		close(done)
	}()
	time.Sleep(2 * time.Millisecond)
	_ = store.Close()
	time.Sleep(15 * time.Millisecond)
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
	if args[0] != cmdRun {
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

func TestStartEmbeddedWorkerAPI_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	logger := slog.Default()
	_ = startEmbeddedWorkerAPI(ctx, "token", t.TempDir(), nil, logger)
	// Clean up in case the embedded server started (readyCh won race with cancelled ctx).
	t.Cleanup(func() {
		if embeddedWorkerAPIShutdown != nil {
			embeddedWorkerAPIShutdown()
			embeddedWorkerAPIShutdown = nil
		}
	})
}

func TestStartEmbeddedWorkerAPI_CancelAfterStartCausesCtxDone(t *testing.T) {
	// Cancel shortly after start; we either get ctx.Canceled (select took <-ctx.Done()) or nil (server became ready first).
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:0")
	defer func() {
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- startEmbeddedWorkerAPI(ctx, "token", stateDir, nil, slog.Default()) }()
	time.Sleep(5 * time.Millisecond)
	cancel()
	err := <-done
	if err != nil {
		t.Logf("startEmbeddedWorkerAPI returned error (ctx.Done branch): %v", err)
	}
	if embeddedWorkerAPIShutdown != nil {
		embeddedWorkerAPIShutdown()
		embeddedWorkerAPIShutdown = nil
	}
}

func TestStartEmbeddedWorkerAPI_Success(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("WORKER_API_STATE_DIR", stateDir)
	t.Setenv("LISTEN_ADDR", "127.0.0.1:0")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:0")
	defer func() {
		_ = os.Unsetenv("WORKER_API_STATE_DIR")
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := slog.Default()
	if err := startEmbeddedWorkerAPI(ctx, "token", stateDir, nil, logger); err != nil {
		t.Fatalf("startEmbeddedWorkerAPI: %v", err)
	}
	if embeddedWorkerAPIShutdown == nil {
		t.Fatal("embeddedWorkerAPIShutdown should be set")
	}
	embeddedWorkerAPIShutdown()
	embeddedWorkerAPIShutdown = nil
}

func TestStartEmbeddedWorkerAPI_RunEmbeddedFails(t *testing.T) {
	// Invalid listen address so workerapiserver.RunEmbedded fails (bind error).
	t.Setenv("LISTEN_ADDR", "256.0.0.1:0")
	t.Setenv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:0")
	defer func() {
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := slog.Default()
	err := startEmbeddedWorkerAPI(ctx, "token", t.TempDir(), nil, logger)
	if err == nil {
		t.Fatal("expected error when RunEmbedded fails")
	}
	_ = cancel
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

func TestStartOllama_ROCm_HSAOverrideGfxVersion(t *testing.T) {
	var calls [][]string
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte(""), nil
	})
	withRunner(t, fake)
	t.Setenv("HSA_OVERRIDE_GFX_VERSION", "9.0.0")
	defer func() { _ = os.Unsetenv("HSA_OVERRIDE_GFX_VERSION") }()
	if err := startOllama("ollama/ollama:rocm", "", nil); err != nil {
		t.Fatalf("startOllama rocm HSA: %v", err)
	}
	var sawHSA bool
	for _, c := range calls {
		for _, a := range c {
			if strings.HasPrefix(a, "HSA_OVERRIDE_GFX_VERSION=") {
				sawHSA = true
				break
			}
		}
	}
	if !sawHSA {
		t.Errorf("expected HSA_OVERRIDE_GFX_VERSION in args; calls=%v", calls)
	}
}

func TestStartOllama_CUDAVariant_PodmanHasDeviceArg(t *testing.T) {
	var calls [][]string
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte(""), nil
	})
	withRunner(t, fake)
	// CONTAINER_RUNTIME podman (default) with CUDA image -> nvidia.com/gpu=all
	_ = os.Unsetenv("CONTAINER_RUNTIME")
	if err := startOllama("ollama/ollama:cuda", "", nil); err != nil {
		t.Fatalf("startOllama cuda podman: %v", err)
	}
	var sawNvidia bool
	for _, c := range calls {
		for _, a := range c {
			if a == "nvidia.com/gpu=all" {
				sawNvidia = true
				break
			}
		}
	}
	if !sawNvidia {
		t.Errorf("expected nvidia.com/gpu=all for CUDA with podman; calls=%v", calls)
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
		if len(args) > 0 && args[0] == cmdRun {
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

func TestStartManagedServices_SkipsNameEqualToPrefix(t *testing.T) {
	// ServiceID that sanitizes to "" so name == managedServiceContainerPrefix; should skip.
	withRunner(t, fakeRunner{})
	services := []nodepayloads.ConfigManagedService{
		{ServiceID: "///", ServiceType: "pma", Image: "img"},
	}
	if err := startManagedServices(services); err != nil {
		t.Errorf("startManagedServices (skip invalid name) should not error: %v", err)
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
		if len(c) >= 2 && c[1] == cmdStop {
			sawStop = true
		}
		if len(c) >= 2 && c[1] == cmdRm {
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
		if len(args) >= 3 && (args[1] == cmdStop || args[1] == cmdRm) {
			return nil, fmt.Errorf("simulated %s error", args[1])
		}
		return []byte(""), nil
	})
	withRunner(t, fake)
	stopNodeManagedContainers(slog.Default()) // should not panic or return error
}

func TestStopAndRemoveContainer_LogsErrorsWhenLoggerSet(t *testing.T) {
	var stopCalled, rmCalled bool
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == cmdStop {
			stopCalled = true
			return nil, fmt.Errorf("stop failed")
		}
		if len(args) > 0 && args[0] == cmdRm {
			rmCalled = true
			return nil, fmt.Errorf("rm failed")
		}
		return []byte(""), nil
	})
	withRunner(t, fake)
	logger := slog.Default()
	stopAndRemoveContainer("podman", "cid1", logger)
	if !stopCalled || !rmCalled {
		t.Errorf("expected both stop and rm calls; stopCalled=%v rmCalled=%v", stopCalled, rmCalled)
	}
}

func TestStopNodeManagedContainers_PSError(t *testing.T) {
	// Simulate ps failing — should silently continue (best-effort).
	fake := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("simulated ps error")
	})
	withRunner(t, fake)
	stopNodeManagedContainers(nil) // should not panic
}

func TestWaitForPMAReadyUDS_EmptyPath(t *testing.T) {
	waitForPMAReadyUDS("", time.Second)
	waitForPMAReadyUDS("  ", time.Second)
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

// Command names used in runner fakes (goconst).
const cmdRun = "run"
const cmdStop = "stop"
const cmdRm = "rm"

// Fake orchestrator paths (must match nodeagent expectations).
const pathReadyz = "/readyz"
const pathNodesRegister = "/v1/nodes/register"
const pathNodesConfig = "/v1/nodes/config"
const pathNodesCapability = "/v1/nodes/capability"

// defaultNodeConfig is the config returned by fakeOrchestratorHandler when configPayload is nil.
var defaultNodeConfig = nodepayloads.NodeConfigurationPayload{
	Version: 1, ConfigVersion: "1", IssuedAt: time.Now().UTC().Format(time.RFC3339), NodeSlug: "test",
	WorkerAPI:        &nodepayloads.ConfigWorkerAPI{OrchestratorBearerToken: "bearer"},
	InferenceBackend: &nodepayloads.ConfigInferenceBackend{Enabled: true, Image: "ollama/ollama"},
}

func writeFakeBootstrapResponse(w http.ResponseWriter, baseURL string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(nodepayloads.BootstrapResponse{
		Version:  1,
		IssuedAt: time.Now().UTC().Format(time.RFC3339),
		Orchestrator: nodepayloads.BootstrapOrchestrator{
			Endpoints: nodepayloads.BootstrapEndpoints{
				NodeReportURL: baseURL + pathNodesCapability,
				NodeConfigURL: baseURL + pathNodesConfig,
			},
		},
		Auth: nodepayloads.BootstrapAuth{NodeJWT: "jwt", ExpiresAt: "2026-01-01T00:00:00Z"},
	})
}

func handleFakeNodesConfig(w http.ResponseWriter, r *http.Request, config *nodepayloads.NodeConfigurationPayload) bool {
	if r.URL.Path != pathNodesConfig {
		return false
	}
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(config)
		return true
	}
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusNoContent)
	}
	return true
}

func fakeOrchestratorHandler(baseURL string, configPayload *nodepayloads.NodeConfigurationPayload) http.HandlerFunc {
	config := configPayload
	if config == nil {
		config = &defaultNodeConfig
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathReadyz {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == pathNodesRegister && r.Method == http.MethodPost {
			writeFakeBootstrapResponse(w, baseURL)
			return
		}
		if handleFakeNodesConfig(w, r, config) {
			return
		}
		if r.URL.Path == pathNodesCapability {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// fakeRunnerFailRunContaining returns a runner that fails "run" when any arg contains substr.
func fakeRunnerFailRunContaining(substr string) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == cmdRun {
			for _, a := range args {
				if strings.Contains(a, substr) {
					return []byte(substr + " run failed"), fmt.Errorf("run failed")
				}
			}
		}
		if len(args) > 0 && args[0] == "ps" {
			return []byte(""), nil
		}
		return []byte(""), nil
	}
}

func TestRunMain_FailsWithStartInferenceError_LogsComponent(t *testing.T) {
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakeOrchestratorHandler(baseURL, nil)(w, r)
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
	defer unsetRunMainTestEnv()

	withRunner(t, fakeRunnerFunc(fakeRunnerFailRunContaining("ollama")))

	code := runMainUntilCancel(t, 3*time.Second)
	if code != 1 {
		t.Errorf("runMain expected 1 (start inference failure), got %d", code)
	}
	cleanupEmbeddedWorkerAPIShutdown()
}

func unsetRunMainTestEnv() {
	_ = os.Unsetenv("WORKER_API_STATE_DIR")
	_ = os.Unsetenv("ORCHESTRATOR_URL")
	_ = os.Unsetenv("NODE_SLUG")
	_ = os.Unsetenv("NODE_REGISTRATION_PSK")
	_ = os.Unsetenv("LISTEN_ADDR")
	_ = os.Unsetenv("WORKER_INTERNAL_LISTEN_ADDR")
	_ = os.Unsetenv("NODE_MANAGER_SKIP_CONTAINER_CHECK")
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
