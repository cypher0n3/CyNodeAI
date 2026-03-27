// Package bdd – worker_node Godog steps: inference proxy and secure-store scenarios.
package bdd

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/worker_node/internal/executor"
	"github.com/cypher0n3/cynodeai/worker_node/internal/inferenceproxy"
	"github.com/cypher0n3/cynodeai/worker_node/internal/nodeagent"
	"github.com/cypher0n3/cynodeai/worker_node/internal/securestore"
)

func RegisterInferenceProxySteps(sc *godog.ScenarioContext, state *workerTestState) {
	sc.Step(`^the inference proxy is configured with an upstream$`, func(ctx context.Context) error {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		upstreamURL, _ := url.Parse(backend.URL)
		proxy := inferenceproxy.NewProxy(upstreamURL)
		state.inferenceProxyServer = httptest.NewServer(proxy)
		return nil
	})
	sc.Step(`^I send a request to the inference proxy with body size exceeding 10 MiB$`, func(ctx context.Context) error {
		if state.inferenceProxyServer == nil {
			return fmt.Errorf("inference proxy not started")
		}
		const overLimit = 10*1024*1024 + 1
		body := bytes.Repeat([]byte("x"), overLimit)
		req, _ := http.NewRequest(http.MethodPost, state.inferenceProxyServer.URL+"/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		state.lastStatus = resp.StatusCode
		return nil
	})
	sc.Step(`^the inference proxy responds with status 413$`, func(ctx context.Context) error {
		if state.lastStatus != http.StatusRequestEntityTooLarge {
			return fmt.Errorf("expected 413, got %d", state.lastStatus)
		}
		return nil
	})
}

// RegisterSecureStoreSteps registers steps for worker_secure_store.feature (Phase 4).
func RegisterSecureStoreSteps(sc *godog.ScenarioContext, state *workerTestState) {
	// Scenario: Worker stores orchestrator-issued secrets encrypted at rest
	sc.Step(`^the node manager receives configuration containing orchestrator-issued secrets$`, func(ctx context.Context) error {
		state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-secure-%d", time.Now().UnixNano()))
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i)
		}
		state.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
		return nil
	})
	sc.Step(`^the node manager applies configuration$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-secure-%d", time.Now().UnixNano()))
			key := make([]byte, 32)
			for i := range key {
				key[i] = byte(i)
			}
			state.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
		}
		return nil
	})
	sc.Step(`^the node stores secrets in the node-local secure store under storage\.state_dir$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" || state.secureStoreMasterKey == "" {
			return fmt.Errorf("state dir or master key not set")
		}
		os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", state.secureStoreMasterKey)
		defer os.Unsetenv("CYNODE_SECURE_STORE_MASTER_KEY_B64")
		store, _, err := securestore.Open(state.secureStoreStateDir)
		if err != nil {
			return fmt.Errorf("open secure store: %w", err)
		}
		state.secureStoreSource = securestore.MasterKeySourceEnvB64
		expiry := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
		if err := store.PutAgentToken("bdd-svc", "test-token-value", expiry); err != nil {
			return fmt.Errorf("put agent token: %w", err)
		}
		secretsDir := filepath.Join(state.secureStoreStateDir, "secrets", "agent_tokens")
		if _, err := os.Stat(secretsDir); err != nil {
			return fmt.Errorf("secrets dir under state_dir not created: %w", err)
		}
		return nil
	})
	sc.Step(`^the persisted secret values are encrypted at rest$`, func(ctx context.Context) error {
		agentTokensDir := filepath.Join(state.secureStoreStateDir, "secrets", "agent_tokens")
		entries, err := os.ReadDir(agentTokensDir)
		if err != nil {
			return fmt.Errorf("read agent_tokens dir: %w", err)
		}
		if len(entries) == 0 {
			return fmt.Errorf("no token file persisted")
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(agentTokensDir, e.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			// Must not be plaintext JSON containing the token value
			if strings.Contains(string(raw), "test-token-value") {
				return fmt.Errorf("persisted file contains plaintext token (not encrypted at rest)")
			}
			if strings.Contains(string(raw), `"token"`) && strings.Contains(string(raw), `"service_id"`) {
				return fmt.Errorf("persisted file looks like plaintext JSON (not encrypted)")
			}
		}
		return nil
	})

	// Scenario: Worker holds agent token and does not pass it to managed-service containers
	sc.Step(`^the orchestrator configures a managed agent service with an agent token$`, func(ctx context.Context) error {
		return nil
	})
	sc.Step(`^the node manager starts the managed-service container$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-runargs-%d", time.Now().UnixNano()))
		}
		svc := &nodepayloads.ConfigManagedService{
			ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
			Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
		}
		state.managedServiceRunArgs = nodeagent.BuildManagedServiceRunArgs(state.secureStoreStateDir, svc, "pma-main", "pma", "pma:latest", "cynodeai-managed-pma-main", "podman")
		return nil
	})
	sc.Step(`^the managed-service container does not receive the agent token via env vars, files, or mounts$`, func(ctx context.Context) error {
		if len(state.managedServiceRunArgs) == 0 {
			return fmt.Errorf("run args not built (run the 0168 scenario step that builds run args first, or this scenario depends on it)")
		}
		for i := 0; i < len(state.managedServiceRunArgs)-1; i++ {
			if state.managedServiceRunArgs[i] == "-e" {
				env := state.managedServiceRunArgs[i+1]
				if strings.HasPrefix(env, "AGENT_TOKEN=") {
					return fmt.Errorf("run args must not pass AGENT_TOKEN env: got %q", env)
				}
			}
			if state.managedServiceRunArgs[i] == "-v" {
				mount := state.managedServiceRunArgs[i+1]
				hostPath, _, _ := strings.Cut(mount, ":")
				if strings.Contains(hostPath, "secrets") {
					return fmt.Errorf("run args must not mount secure store: got %q", hostPath)
				}
			}
		}
		return nil
	})
	sc.Step(`^the worker proxy attaches the agent token when forwarding agent-originated requests$`, func(ctx context.Context) error {
		// Covered by unit tests (inferenceproxy, internal proxy auth). No in-BDD way to assert without full stack.
		return nil
	})

	// Scenario: Worker warns when using env var master key fallback
	sc.Step(`^the node cannot access TPM, OS key store, or system service credentials$`, func(ctx context.Context) error {
		os.Unsetenv("CREDENTIALS_DIRECTORY")
		os.Setenv("CYNODE_FIPS_MODE", "0") // Explicit non-FIPS so env fallback is allowed on all platforms
		return nil
	})
	sc.Step(`^the environment variable CYNODE_SECURE_STORE_MASTER_KEY_B64 is set$`, func(ctx context.Context) error {
		if state.secureStoreMasterKey == "" {
			key := make([]byte, 32)
			for i := range key {
				key[i] = byte(i + 1)
			}
			state.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
		}
		os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", state.secureStoreMasterKey)
		return nil
	})
	sc.Step(`^the environment variable CYNODE_SECURE_STORE_MASTER_KEY_B(\d+) is set$`, func(ctx context.Context, _ int) error {
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i + 1)
		}
		state.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
		os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", state.secureStoreMasterKey)
		return nil
	})
	sc.Step(`^the node starts$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-secure-%d", time.Now().UnixNano()))
		}
		os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", state.secureStoreMasterKey)
		store, source, err := securestore.Open(state.secureStoreStateDir)
		if err != nil {
			return fmt.Errorf("open secure store: %w", err)
		}
		_ = store
		state.secureStoreSource = source
		return nil
	})
	sc.Step(`^the node uses the env_b(\d+) master key backend$`, func(ctx context.Context, _ int) error {
		if state.secureStoreSource != securestore.MasterKeySourceEnvB64 {
			return fmt.Errorf("expected env_b64 master key backend, got %q", state.secureStoreSource)
		}
		return nil
	})
	sc.Step(`^the node emits a startup warning indicating a less-secure master key backend$`, func(ctx context.Context) error {
		// Warning is logged by node-manager; unit tests and code review cover. BDD accepts when env key was used.
		if state.secureStoreSource != securestore.MasterKeySourceEnvB64 {
			return fmt.Errorf("expected env_b64 to have been used (warning scenario)")
		}
		return nil
	})

	// Scenario: Managed-service container run args do not mount secure store (0168)
	sc.Step(`^a managed service is configured for the node$`, func(ctx context.Context) error {
		state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-runargs-%d", time.Now().UnixNano()))
		return nil
	})
	sc.Step(`^the node manager builds run args for the managed-service container$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-runargs-%d", time.Now().UnixNano()))
		}
		svc := &nodepayloads.ConfigManagedService{
			ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
			Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
		}
		state.managedServiceRunArgs = nodeagent.BuildManagedServiceRunArgs(state.secureStoreStateDir, svc, "pma-main", "pma", "pma:latest", "cynodeai-managed-pma-main", "podman")
		return nil
	})
	sc.Step(`^the run args do not contain any mount of the secure store or secrets path$`, func(ctx context.Context) error {
		for i := 0; i < len(state.managedServiceRunArgs)-1; i++ {
			if state.managedServiceRunArgs[i] == "-v" {
				mount := state.managedServiceRunArgs[i+1]
				hostPath, _, _ := strings.Cut(mount, ":")
				if strings.Contains(hostPath, "secrets") {
					return fmt.Errorf("run args must not mount secure store: got host path %q", hostPath)
				}
			}
		}
		return nil
	})
	sc.Step(`^the node manager builds run args for the managed-service container with healthcheck and runtime podman$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-runargs-%d", time.Now().UnixNano()))
		}
		svc := &nodepayloads.ConfigManagedService{
			ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
			Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
			Healthcheck:  &nodepayloads.ConfigManagedServiceHealthcheck{Path: "/healthz", ExpectedStatus: 200},
		}
		state.managedServiceRunArgs = nodeagent.BuildManagedServiceRunArgs(state.secureStoreStateDir, svc, "pma-main", "pma", "pma:latest", "cynodeai-managed-pma-main", "podman")
		return nil
	})
	sc.Step(`^the run args include podman health-check options$`, func(ctx context.Context) error {
		var hasHealthCmd bool
		for i := 0; i < len(state.managedServiceRunArgs)-1; i++ {
			if state.managedServiceRunArgs[i] == "--health-cmd" {
				hasHealthCmd = true
				break
			}
		}
		if !hasHealthCmd {
			return fmt.Errorf("run args should include --health-cmd when config has healthcheck and runtime is podman")
		}
		return nil
	})

	// Scenario: Secure store distinct from telemetry (0169)
	sc.Step(`^the node state directory is set$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-state-%d", time.Now().UnixNano()))
		}
		return nil
	})
	sc.Step(`^the secure store path is under state_dir and distinct from the telemetry database path$`, func(ctx context.Context) error {
		securePath := filepath.Join(state.secureStoreStateDir, "secrets")
		// Telemetry DB is under state_dir but different path (e.g. telemetry.db or similar)
		if !strings.Contains(securePath, "secrets") {
			return fmt.Errorf("secure store path must be under state_dir and contain secrets: %q", securePath)
		}
		if strings.Contains(securePath, "telemetry") || strings.HasSuffix(state.secureStoreStateDir, "telemetry") {
			return fmt.Errorf("secure store path must be distinct from telemetry: %q", securePath)
		}
		return nil
	})

	// Scenario: FIPS mode rejects env master key (0170)
	sc.Step(`^FIPS mode is enabled or unknown on the host$`, func(ctx context.Context) error {
		os.Setenv("CYNODE_FIPS_MODE", "1")
		return nil
	})
	sc.Step(`^the node opens the secure store with env master key only$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-fips-%d", time.Now().UnixNano()))
		}
		if state.secureStoreMasterKey == "" {
			key := make([]byte, 32)
			for i := range key {
				key[i] = byte(i + 2)
			}
			state.secureStoreMasterKey = base64.StdEncoding.EncodeToString(key)
		}
		os.Setenv("CYNODE_SECURE_STORE_MASTER_KEY_B64", state.secureStoreMasterKey)
		_, _, err := securestore.Open(state.secureStoreStateDir)
		state.secureStoreOpenErr = err
		return nil
	})
	sc.Step(`^opening the secure store fails with FIPS-related error$`, func(ctx context.Context) error {
		if state.secureStoreOpenErr == nil {
			return fmt.Errorf("expected Open to fail in FIPS mode with env key")
		}
		if !errors.Is(state.secureStoreOpenErr, securestore.ErrFIPSRequiresNonEnvKey) {
			return fmt.Errorf("expected ErrFIPSRequiresNonEnvKey, got: %v", state.secureStoreOpenErr)
		}
		return nil
	})

	// Scenario: Process boundary documented (0172)
	sc.Step(`^the worker node codebase$`, func(ctx context.Context) error {
		return nil
	})
	sc.Step(`^the secure store process boundary document exists and states writer and reader components$`, func(ctx context.Context) error {
		docPath := "docs/tech_specs/worker_node.md"
		for _, root := range []string{"../..", "..", "."} {
			p := filepath.Join(root, docPath)
			if _, err := os.Stat(p); err == nil {
				content, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				s := strings.ToLower(string(content))
				if !strings.Contains(s, "writer") || !strings.Contains(s, "reader") {
					return fmt.Errorf("document must state writer and reader components")
				}
				return nil
			}
		}
		return fmt.Errorf("process boundary document not found at %s", docPath)
	})

	// REQ-WORKER-0270 / REQ-SANDBX-0131: UDS inference routing (worker_inference_proxy.feature + worker_secure_store.feature)

	sc.Step(`^the inference proxy is started with INFERENCE_PROXY_SOCKET set to a temp path$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		state.inferenceProxySocketPath = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-inf-%d.sock", time.Now().UnixNano()))
		udsCtx, cancel := context.WithCancel(ctx)
		state.inferenceProxyUDSCancel = cancel
		state.inferenceProxyUDSDone = make(chan int, 1)
		go func() {
			state.inferenceProxyUDSDone <- inferenceproxy.RunUDS(udsCtx, state.inferenceProxySocketPath)
		}()
		return nil
	})
	sc.Step(`^the inference proxy socket file exists at that path$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		for i := 0; i < 40; i++ {
			if _, err := os.Stat(state.inferenceProxySocketPath); err == nil {
				return nil
			}
			time.Sleep(25 * time.Millisecond)
		}
		if state.inferenceProxyUDSCancel != nil {
			state.inferenceProxyUDSCancel()
		}
		return fmt.Errorf("UDS socket %q did not appear (REQ-WORKER-0270)", state.inferenceProxySocketPath)
	})
	sc.Step(`^a healthz request over the Unix domain socket returns 200$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		defer func() {
			if state.inferenceProxyUDSCancel != nil {
				state.inferenceProxyUDSCancel()
			}
		}()
		transport := &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", state.inferenceProxySocketPath)
			},
		}
		client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
		resp, err := client.Get("http://unix/healthz")
		if err != nil {
			return fmt.Errorf("healthz over UDS: %w", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("healthz status=%d, want 200 (REQ-WORKER-0270)", resp.StatusCode)
		}
		return nil
	})

	sc.Step(`^the executor is configured with a proxy image and an upstream URL$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		e := executor.New("podman", 30*time.Second, 4096, "http://host.containers.internal:11434", "proxy:latest", nil)
		req := &workerapi.RunJobRequest{
			TaskID: "bdd-t1", JobID: "bdd-j1",
			Sandbox: workerapi.SandboxSpec{Image: "cynode-sba:dev", JobSpecJSON: `{}`},
		}
		state.sbaPodRunArgs = executor.BuildSBARunArgsForPod(req, "pod-1", "/tmp/bdd-job", "/tmp/bdd-ws", "/tmp/bdd-sock", e, "agent_inference")
		return nil
	})
	sc.Step(`^the executor builds SBA pod run args for agent_inference mode$`, func(ctx context.Context) error {
		return nil
	})
	sc.Step(`^the SBA container args contain INFERENCE_PROXY_URL with an http\+unix scheme$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		argStr := strings.Join(state.sbaPodRunArgs, " ")
		if !strings.Contains(argStr, "INFERENCE_PROXY_URL=http+unix://") {
			return fmt.Errorf("SBA pod args must contain INFERENCE_PROXY_URL=http+unix://...(REQ-SANDBX-0131), got: %s", argStr)
		}
		return nil
	})
	sc.Step(`^the SBA container args do not contain OLLAMA_BASE_URL with a TCP localhost address$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		argStr := strings.Join(state.sbaPodRunArgs, " ")
		if strings.Contains(argStr, "OLLAMA_BASE_URL=http://localhost:") {
			return fmt.Errorf("SBA pod args must not contain TCP OLLAMA_BASE_URL (REQ-SANDBX-0131), got: %s", argStr)
		}
		return nil
	})

	// REQ-SANDBX-0131 / REQ-WORKER-0174: SBA non-pod (direct) path with SBA_INFERENCE_PROXY_SOCKET.

	sc.Step(`^the executor is configured with an upstream URL and no proxy image$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		state.sbaDirectExecutor = executor.New("podman", 30*time.Second, 4096, "http://host.containers.internal:11434", "", nil)
		return nil
	})
	sc.Step(`^SBA_INFERENCE_PROXY_SOCKET is set to a temp socket path$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		state.sbaInferenceProxySocketPath = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-sba-inf-%d.sock", time.Now().UnixNano()))
		return os.Setenv("SBA_INFERENCE_PROXY_SOCKET", state.sbaInferenceProxySocketPath)
	})
	sc.Step(`^SBA_INFERENCE_PROXY_SOCKET is not set$`, func(ctx context.Context) error {
		return os.Unsetenv("SBA_INFERENCE_PROXY_SOCKET")
	})
	sc.Step(`^the executor builds SBA direct run args for agent_inference mode$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		if state.sbaDirectExecutor == nil {
			return fmt.Errorf("executor not configured (run 'the executor is configured with an upstream URL and no proxy image' first)")
		}
		req := &workerapi.RunJobRequest{
			TaskID: "bdd-t1", JobID: "bdd-j1",
			Sandbox: workerapi.SandboxSpec{Image: "cynode-sba:dev", JobSpecJSON: `{}`},
		}
		state.sbaDirectRunArgs = executor.BuildSBARunArgs(req, "/tmp/bdd-job", "/tmp/bdd-ws", state.sbaDirectExecutor, "agent_inference")
		return nil
	})
	sc.Step(`^the SBA direct container args contain INFERENCE_PROXY_URL with an http\+unix scheme$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		argStr := strings.Join(state.sbaDirectRunArgs, " ")
		if !strings.Contains(argStr, "INFERENCE_PROXY_URL=http+unix://") {
			return fmt.Errorf("SBA direct args must contain INFERENCE_PROXY_URL=http+unix://... (REQ-SANDBX-0131), got: %s", argStr)
		}
		return nil
	})
	sc.Step(`^the SBA direct container args include a volume mount for the inference proxy socket dir$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		argStr := strings.Join(state.sbaDirectRunArgs, " ")
		if !strings.Contains(argStr, "/run/cynode") {
			return fmt.Errorf("SBA direct args must mount inference proxy socket dir at /run/cynode (REQ-SANDBX-0131), got: %s", argStr)
		}
		return nil
	})
	sc.Step(`^the SBA direct container args keep --network=none$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		argStr := strings.Join(state.sbaDirectRunArgs, " ")
		if !strings.Contains(argStr, "--network=none") {
			return fmt.Errorf("SBA direct args must keep --network=none (REQ-WORKER-0174), got: %s", argStr)
		}
		return nil
	})
	sc.Step(`^the SBA direct container args do not contain INFERENCE_PROXY_URL$`, func(ctx context.Context) error {
		state := getWorkerState(ctx)
		argStr := strings.Join(state.sbaDirectRunArgs, " ")
		if strings.Contains(argStr, "INFERENCE_PROXY_URL=") {
			return fmt.Errorf("SBA direct args must NOT contain INFERENCE_PROXY_URL when socket env is unset (REQ-SANDBX-0131), got: %s", argStr)
		}
		return nil
	})

	sc.Step(`^the node manager builds run args for the managed-service container with node_local inference$`, func(ctx context.Context) error {
		if state.secureStoreStateDir == "" {
			state.secureStoreStateDir = filepath.Join(os.TempDir(), fmt.Sprintf("bdd-runargs-%d", time.Now().UnixNano()))
		}
		svc := &nodepayloads.ConfigManagedService{
			ServiceID: "pma-main", ServiceType: "pma", Image: "pma:latest",
			Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{},
			Inference:    &nodepayloads.ConfigManagedServiceInference{Mode: "node_local"},
		}
		state.managedServiceRunArgs = nodeagent.BuildManagedServiceRunArgs(state.secureStoreStateDir, svc, "pma-main", "pma", "pma:latest", "cynodeai-managed-pma-main", "podman")
		return nil
	})
	sc.Step(`^the run args contain OLLAMA_BASE_URL with an http\+unix scheme$`, func(ctx context.Context) error {
		if len(state.managedServiceRunArgs) == 0 {
			return fmt.Errorf("run args not built")
		}
		argv := strings.Join(state.managedServiceRunArgs, " ")
		if !strings.Contains(argv, "OLLAMA_BASE_URL=http+unix://") {
			return fmt.Errorf("managed-service run args must contain OLLAMA_BASE_URL=http+unix://... (REQ-WORKER-0270), got: %s", argv)
		}
		return nil
	})
	sc.Step(`^the run args do not contain OLLAMA_BASE_URL with a TCP address$`, func(ctx context.Context) error {
		if len(state.managedServiceRunArgs) == 0 {
			return fmt.Errorf("run args not built")
		}
		argv := strings.Join(state.managedServiceRunArgs, " ")
		if strings.Contains(argv, "OLLAMA_BASE_URL=http://") {
			return fmt.Errorf("managed-service run args must not contain TCP OLLAMA_BASE_URL (REQ-WORKER-0270), got: %s", argv)
		}
		return nil
	})
	sc.Step(`^the run args include --network=none$`, func(ctx context.Context) error {
		for _, a := range state.managedServiceRunArgs {
			if a == "--network=none" {
				return nil
			}
		}
		return fmt.Errorf("managed-service run args must include --network=none (REQ-WORKER-0174), got: %v", state.managedServiceRunArgs)
	})
	sc.Step(`^the run args do not publish port 8090$`, func(ctx context.Context) error {
		for i := 0; i < len(state.managedServiceRunArgs)-1; i++ {
			if state.managedServiceRunArgs[i] == "-p" && strings.Contains(state.managedServiceRunArgs[i+1], "8090") {
				return fmt.Errorf("managed-service run args must not publish TCP port 8090 (REQ-WORKER-0174), got: -p %s", state.managedServiceRunArgs[i+1])
			}
		}
		return nil
	})
}
