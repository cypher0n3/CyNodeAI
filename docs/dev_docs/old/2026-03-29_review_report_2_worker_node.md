# Review Report 2: Worker Node

- [1 Summary](#1-summary)
- [2 Specification Compliance](#2-specification-compliance)
- [3 Architectural Issues](#3-architectural-issues)
- [4 Concurrency and Safety](#4-concurrency-and-safety)
- [5 Security Risks](#5-security-risks)
- [6 Performance Concerns](#6-performance-concerns)
- [7 Maintainability Issues](#7-maintainability-issues)
- [8 Recommended Actions](#8-recommended-actions)

## 1 Summary

This report covers the `worker_node/` module: 51 Go files across 2 binaries (`node-manager`, `inference-proxy`) and ~6 internal packages (`nodeagent`, `workerapiserver`, `executor`, `securestore`, `telemetry`, `config`).

The worker node is functional for its MVP scope with working registration, config lifecycle, capability reporting, managed service orchestration, sandbox execution, secure store encryption, and telemetry.
However, the review surfaces **2 critical**, **10 high**, **16 medium**, and **10 low** severity findings.

The most impactful gaps are:

- **REQ-WORKER-0174 (network restriction) violated** -- pod-based sandbox containers share the pod's network namespace, enabling direct internet access that bypasses the worker proxy.
- **Container name matching bug** -- `startOneManagedService` uses `strings.Contains` for container detection, causing false positives when one container name is a prefix of another.
- **Secure store key material never zeroed** -- the AES master key and ML-KEM decapsulation key persist in heap for the process lifetime with no `Close()` method.
- **Bearer token comparison is not constant-time** in the embedded worker API handlers.
- **Unbounded goroutine spawning** in the telemetry slog handler creates one goroutine per log record.

## 2 Specification Compliance

Gaps identified against requirements and technical specifications.

### 2.1 Critical Gaps

- ❌ **REQ-WORKER-0174 -- Pod-based sandbox containers have unrestricted network access.**
  `buildSandboxRunArgsForPod` (`executor.go:753-771`) and `buildSBARunArgsForPod` (`executor.go:660-695`) create sandbox containers inside a Podman pod created without `--network=none` (`executor.go:190`).
  Since all containers in a pod share the network namespace, the sandbox inherits the proxy sidecar's network access.
  The spec states: "All inbound and outbound traffic to or from those agents MUST route through worker proxies; there MUST be no direct network path that bypasses the worker proxy."
  The sandbox can reach external hosts directly via TCP, bypassing the UDS proxy.

### 2.2 High-Severity Gaps

- ⚠️ **REQ-WORKER-0163 -- No audit logging for internal orchestrator proxy requests.**
  `handleInternalOrchestratorProxyForward` (`internal_orchestrator_proxy.go:126-175`) does not emit any audit record.
  The `serviceID` is extracted from context but never logged alongside the forwarded request method, path, or response status.

- ⚠️ **Hardcoded RAM in capability report.**
  `buildCapability` (`nodemanager_config.go:469-471`) always reports `RAMMB: 4096` regardless of actual system memory.
  The orchestrator receives inaccurate compute data, affecting scheduling and resource decisions.

### 2.3 Medium-Severity Gaps

- **REQ-WORKER-0164 partial violation -- Error messages in internal proxy leak upstream details.**
  `handleInternalOrchestratorProxyForward` (`internal_orchestrator_proxy.go:170`) returns `fmt.Sprintf("upstream request failed: %v", err)` which may contain orchestrator URL, DNS resolution errors, and certificate details.
  Should return a generic error.

- **REQ-WORKER-0253 -- Latent compliance risk in OLLAMA_IMAGE fallback.**
  `startOllama` (`main.go:469-471`) has a secondary `OLLAMA_IMAGE` env fallback that applies when image is empty regardless of variant.
  Currently unreachable for variant-supplied paths, but if `maybeStartOllama` logic changes, this becomes a spec violation.

- **REQ-WORKER-0232 -- Node stats endpoint returns hardcoded values.**
  `embedNodeStatsHandler` (`embed_handlers.go:492-494`) returns `"total_mb": 1024, "used_mb": 0, "free_mb": 1024` which are fabricated.
  This misleads orchestrator capacity planning.

- **REQ-WORKER-0272/0273 -- Compliant.**
  `RunEmbedded` correctly starts the Worker API in-process with the node manager.

- **REQ-WORKER-0265 -- Compliant.**
  `detectGPU` (`gpu.go:51-68`) correctly merges devices from both NVIDIA and ROCm with `Vendor` and `VRAMMB`.

## 3 Architectural Issues

Structural and design concerns in the worker node codebase.

### 3.1 Node Agent

- ❌ **Container name matching inconsistency.**
  `startOneManagedService` (`main.go:599`) uses `strings.Contains(string(out), name)` for container name detection.
  `startOllama` (`main.go:477`) correctly uses `containerNameExact`.
  The nodeagent package has `containerNameMatches` (identical logic).
  `strings.Contains` causes false-positive matches when one container name is a prefix of another (e.g., `cynodeai-managed-pma` matches `cynodeai-managed-pma-test`).

- ⚠️ **`os.Setenv` for IPC.** `applyWorkerProxyConfigEnv` (`nodemanager.go:343-353`) uses `os.Setenv` to pass configuration to the embedded worker API.
  Environment variables are global process state -- not goroutine-safe, visible to all child processes, and create hidden coupling.

- ⚠️ **Extensive function duplication across packages.**
  `getEnv` is defined 4 times (`main.go:195`, `nodemanager_config.go:574`, `server.go:312`, `_bdd/steps.go:378`).
  `effectiveStateDir` is defined identically in `main.go:581` and `nodemanager.go:177`.
  `containerNameExact` (`main.go:408`) and `containerNameMatches` (`nodemanager_config.go:360`) are identical functions with different names.

- **Test infrastructure in production code.**
  Environment-gated test bypasses leak into production:
  `NODE_MANAGER_TEST_NO_EXISTING_INFERENCE`, `NODE_MANAGER_TEST_NO_GPU_DETECT`, `NODE_MANAGER_SKIP_CONTAINER_CHECK`, `NODE_MANAGER_SKIP_SERVICES`.
  Should be replaced by dependency injection or build tags.

### 3.2 Worker API Server

- **`internalOrchestratorHTTPClient` is a package-level mutable variable** (`internal_orchestrator_proxy.go:24`).
  Tests override it, creating a data race if tests run in parallel.

- **`os.Setenv` in `startSBAInferenceProxy`** (`embed.go:148`) is a process-global side effect for passing socket path to the executor.

- **Fragile URL parsing.**
  Container ID extracted via `strings.TrimPrefix` (`embed_handlers.go:574`) instead of `r.PathValue()`.

### 3.3 Secure Store

- ⚠️ **No centralized config package.**
  Configuration is scattered across modules.
  Secure store reads env vars directly, telemetry hardcodes paths.
  Impossible to validate all config at startup.

### 3.4 Telemetry Subsystem

- **AutoMigrate runs on every startup** (`store.go:42-51`).
  The existing `SchemaVersion` table suggests versioned migrations were intended but not implemented.

## 4 Concurrency and Safety

Goroutine lifecycle, context propagation, and synchronization issues.

### 4.1 Goroutine Lifecycle

- ❌ **Unbounded goroutines in LogHandler.** `Handle` (`sloghandler.go:57-60`) spawns a goroutine for every `slog.Info/Warn/Error` call.
  Under high-throughput logging, this creates thousands of goroutines contending on the single SQLite connection (`SetMaxOpenConns(1)`).
  Uses `context.WithoutCancel`, so goroutines survive parent cancellation and can write to a closing database.

- ⚠️ **`maybePullModels` goroutine leak.** (`nodemanager.go:484-489`) launches a detached goroutine not tied to context.
  During shutdown, model pulls continue indefinitely.
  Called again on config refresh, potentially spawning multiple concurrent pull goroutines with no coordination.

- ⚠️ **`StartDetached` goroutine leak.** (`main.go:52-62`) spawns background goroutine `go func() { _ = cmd.Wait() }()` to reap detached processes.
  If the process never exits, this goroutine leaks.

- ⚠️ **`serverErr` channel capacity too small.** (`server.go:98`) `make(chan error, 1)`.
  Public, internal TCP, internal Unix, and per-service UDS listeners each send to `serverErr` on failure.
  If two or more listeners fail simultaneously, all but the first block forever.

### 4.2 Missing Context Propagation

- **`waitForPMAReadyUDS`** (`main.go:555-578`) accepts no `context.Context`.
  If parent context is cancelled during startup (SIGTERM), the 30-second polling loop continues.

- **`detectExistingInference`** (`nodemanager_config.go:372-398`) uses bare `exec.Command` instead of `exec.CommandContext`.

- **`pullModels`** (`main.go:508-524`) takes no `context.Context`.
  Long-running `ollama pull` commands are uninterruptible.

### 4.3 Other Issues

- **Package-level test variables without synchronization** in securestore (`fips.go:19-22`, `fips_linux.go:13-14`).

- **TOCTOU race on UDS socket permissions** (`server.go:239-246`): socket created then `os.Chmod` applied, leaving a window with default permissions.

## 5 Security Risks

Vulnerabilities organized by severity level.

### 5.1 Critical Severity

- ❌ **Secure store key material never zeroed.**
  `Store` struct (`store.go:75-80`): neither `key` (AES master key) nor `kemKey` (ML-KEM decapsulation key) is ever zeroed.
  No `Close()` or `Destroy()` method exists.
  Keys sit in heap memory for the process lifetime.

### 5.2 High Severity

- ⚠️ **Bearer token comparison not constant-time.**
  `embed_handlers.go:280` and `embed_handlers.go:405`: `strings.TrimSpace(auth[7:]) != bearerToken` is vulnerable to timing side-channel attacks.
  Must use `subtle.ConstantTimeCompare`.

- ⚠️ **`runDirect` inherits all host environment variables.**
  `executor.go:854`: `cmd.Env = append(os.Environ(), envSlice...)` passes ALL host env vars (including `CYNODE_SECURE_STORE_MASTER_KEY_B64`, orchestrator tokens) to the child process.
  REQ-WORKER-0164 states agents "MUST NOT be given tokens or secrets directly."

- ⚠️ **`zeroBytes` may be optimized away by compiler.**
  `store.go:396-400`: Go's compiler can eliminate dead stores.
  The `RunWithSecret` fallback (`secret_fallback.go:8-11`) is a no-op on current builds.

- ⚠️ **Plaintext token returned as immutable `string`.**
  `AgentTokenRecord.Token` (`store.go:67-72`) is a `string` -- callers cannot zero it after use.
  Should be `[]byte` with explicit zeroing.

### 5.3 Medium Severity

- **No AAD in GCM Seal/Open** (`store.go:376-377`, `store.go:571`).
  Envelope `version` and `algorithm` fields are not authenticated.
  An attacker could swap headers to force a different code path.

- **PQ path uses KEM shared secret directly as AES key without KDF** (`store.go:556-572`).
  NIST SP 800-227 recommends running KEM output through a KDF.
  No hybrid with master key -- if KEM is broken, no fallback.

- **KEM private key encrypted with the same master key it "protects"** (`store.go:483-487`).
  If master key is compromised, KEM provides no additional protection.

- **Unbounded HTTP response body** in multiple functions: `register` (`nodemanager_config.go:293`), `FetchConfig` (`nodemanager_config.go:125`), `doAgentTokenRefRequest` (`nodemanager.go:322`).

- **Socket host directory is world-writable** (`executor.go:205,499`): `os.Chmod(sockHostDir, 0o777)`.

- **`doManagedProxyUpstream` reads entire upstream response** without limit (`embed_handlers.go:331`).

- **`writeManagedProxyJSONFromUpstream` reads entire upstream response** without limit (`internal_orchestrator_proxy.go:114`).

- **No `MaxBytesReader` on `validateManagedProxyRequest`** (`embed_handlers.go:297`).

- **Sensitive data in env.** `applyWorkerProxyConfigEnv` serializes node config to `WORKER_NODE_CONFIG_JSON`, visible in `/proc/<pid>/environ`.

### 5.4 Low Severity

- No length limit on `serviceID` in secure store; potential filesystem path exhaustion.
- `PutAgentToken` leaves `.tmp` file on Rename failure.
- No master key rotation mechanism; key rotation makes existing tokens unreadable.
- Container labels accept unsanitized input from orchestrator (`executor.go:142-145`).

## 6 Performance Concerns

Storage, network, and resource efficiency issues.

### 6.1 Storage Efficiency

- ⚠️ **`node_boot` table has no retention policy; grows forever.**
  `EnforceRetention` (`store.go:69-83`) covers `log_event` (7d), `container_event` (30d), `container_inventory` (30d) but never `node_boot`.

- ⚠️ **No secondary indexes on query-hot columns.**
  `ContainerInventory` and `LogEvent` models have no GORM index tags on `occurred_at`, `source_kind`, `container_id`, `status`, `kind`.
  All `WHERE` and `ORDER BY` clauses hit unindexed columns.

- **Bytes truncation can return zero events, creating irrecoverable pagination hole** (`logs.go:83-107`).
  If the first row's message exceeds 1 MiB, the loop returns empty with no `nextToken`.

### 6.2 Network and Allocation

- **HTTP client created per request** in nodemanager_config.go (lines 111, 159, 187, 279, 309, 415).
  Defeats connection pooling; full TCP handshake per request.

- **GPU cache holds mutex during detection** (`gpu.go:34-43`).
  5-second detection timeout blocks concurrent callers.

- **Public HTTP server has `WriteTimeout: 0`** (`server.go:104`).
  Slow or malicious clients hold connections indefinitely.

- **`managedProxyHTTPClient` creates new client per request** (`embed_handlers.go:353-354`).

## 7 Maintainability Issues

- **Error swallowing in capability loop.** `runCapabilityLoop` (`nodemanager.go:551-553`): `_ = err` silently discards capability report errors.

- **Error swallowing in `refreshNodeConfig`** (`nodemanager_config.go:28-49`): returns old config silently on fetch failure with no logging.

- **`Shutdown` returns only first error** (`server.go:306-308`); should use `errors.Join`.

- **`json.Marshal` and `json.Unmarshal` errors silently discarded** in telemetry events (`events.go:14-16`, `logs.go:120-121`, `containers.go:85`).

- **Duplicate bearer auth check** with different error formats (`embed_handlers.go:278-282` vs `embed_handlers.go:402-410`).

- **Hardcoded Ollama port binding** (`main.go:444`): `-p 11434:11434` instead of respecting `OLLAMA_PORT`.

- **No retry on registration or config fetch.**
  Single-attempt; transient 5xx causes permanent failure.

- **`MaxConcurrency` hardcoded to 4** (`nodemanager_config.go:476`).

- **Dead method check** in `embedJobsRunHandler` (`embed_handlers.go:667-669`): route already restricts to POST.

## 8 Recommended Actions

Remediation items organized by priority tier.

### 8.1 P0 -- Immediate (Correctness and Security)

1. **Fix pod network isolation.**
   Create pods with `--network=none` and run proxy sidecar in a separate network namespace, or restructure so the proxy runs outside the pod while the sandbox pod is fully isolated.
2. **Fix container name matching.**
   Replace `strings.Contains` in `startOneManagedService` with `containerNameExact`.
3. **Add `Close()` to `securestore.Store`** that zeros `key` and `kemKey`.
   Change `AgentTokenRecord.Token` to `[]byte`.
4. **Replace bearer token comparisons** with `subtle.ConstantTimeCompare` in `embed_handlers.go`.
5. **Sanitize `runDirect` environment.**
   Construct `cmd.Env` from an explicit allowlist instead of `os.Environ()`.

### 8.2 P1 -- Short-Term (High-Severity Issues)

1. **Add bounded worker pool** to `LogHandler` instead of unbounded goroutine spawning per log record.
2. **Track `maybePullModels` goroutine** in a `sync.WaitGroup` so shutdown can wait for it.
3. **Add `context.Context`** to `waitForPMAReadyUDS`, `pullModels`, and `detectExistingInference`.
4. **Detect actual system RAM** (e.g., `/proc/meminfo`) instead of hardcoding 4096 MB.
5. **Add audit logging** to `handleInternalOrchestratorProxyForward`.
6. **Add `io.LimitReader`** on all HTTP response body decoding.
7. **Increase `serverErr` channel capacity** to match the number of listener goroutines.

### 8.3 P2 -- Planned (Medium-Severity Improvements)

1. **Add AAD to GCM** binding envelope version and algorithm.
   Add HKDF to PQ path combining master key and KEM shared secret.
2. **Add GORM index tags** on `occurred_at`, `source_kind`, `container_id`, `status`, `kind`.
3. **Add `node_boot` retention** to `EnforceRetention`.
4. **Handle bytes-truncation zero-events edge case** by returning at least one event.
5. **Extract shared utilities** (`getEnv`, `effectiveStateDir`, `sortedKeys`, container name helpers) into a shared internal package.
6. **Replace `os.Setenv`-based IPC** with struct-based config passing.
7. **Share a single `http.Client`** across registration/config/capability lifecycle.
8. **Add `http.MaxBytesReader`** to `validateManagedProxyRequest`.
9. **Return generic errors** from internal proxy instead of leaking upstream details.

### 8.4 P3 -- Longer-Term (Maintenance and Debt)

1. **Extract centralized config package** with a single struct validated at startup.
2. **Replace env-gated test bypasses** with dependency injection.
3. **Implement versioned migrations** using the existing `SchemaVersion` table.
4. **Add retry with backoff** to registration and config fetch.
5. **Log capability report and config refresh errors** instead of silently discarding.
6. **Set finite `WriteTimeout`** on public HTTP server.
