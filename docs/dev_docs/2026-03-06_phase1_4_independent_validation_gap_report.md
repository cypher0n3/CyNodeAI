# Phase 1-4 Independent Validation Gap Report

- [Summary](#summary)
- [Phase 1: Inference Proxy Data Plane](#phase-1-inference-proxy-data-plane)
- [Phase 2: SBA Inference Through Node-Local Proxy](#phase-2-sba-inference-through-node-local-proxy)
- [Phase 3: Internal Proxy Endpoint Scaffolding](#phase-3-internal-proxy-endpoint-scaffolding)
- [Phase 4: Node-Local Secure Store and Agent Token Lifecycle](#phase-4-node-local-secure-store-and-agent-token-lifecycle)
- [Recommendations](#recommendations)
- [Traceability](#traceability)

## Summary

Date: 2026-03-06.
Scope: Phases 1-4 of [worker proxy spec reconciliation plan](2026-03-05_worker_proxy_spec_reconciliation_plan.md).
Method: Independent code and spec review; no reliance on existing validation reports.
Spec anchors: [reconciliation plan Normative Targets](2026-03-05_worker_proxy_spec_reconciliation_plan.md#normative-targets) and referenced tech specs/requirements.

Phases 1-4 are **substantially compliant** with the stated specs and requirements.
No blocking implementation gaps were found.
The report below records verified compliance per phase and minor gaps or documentation inaccuracies for remediation.

## Phase 1: Inference Proxy Data Plane

Reconciliation plan claims: loopback bind, `/healthz`, fixed sleep removed in favor of bounded readiness probe, unit tests.

### Phase 1 Verification

- Inference proxy default bind is `127.0.0.1:11434` (`worker_node/cmd/inference-proxy/main.go`).
  Aligns with CYNAI.WORKER.NodeLocalInference and REQ-WORKER-0114/0115 (node-local, Ollama not public).
- `GET /healthz` is implemented and returns 200 with body `ok`.
- Pod inference path uses bounded active probe: `waitForProxyReady` with 10s deadline and `probeProxyHealthOnce` against `http://127.0.0.1:11434/healthz` (executor.go).
  No fixed sleep in the production path.
- Proxy enforces 10 MiB request body and 120s per-request timeout (inferenceproxy package; worker_node.md).
- Unit tests: `inference-proxy/main_test.go` (healthz, healthcheck URL, bind); executor tests for proxy readiness timeout and probe behavior.

Phase 1 gaps: none.

## Phase 2: SBA Inference Through Node-Local Proxy

Reconciliation plan claims: SBA agent-inference pod plus proxy sidecar, `OLLAMA_BASE_URL=http://localhost:11434` in pod, direct-steps preserved, result.json writable, unit tests.

### Phase 2 Verification

- `shouldUseSBAPodInference` selects pod path when execution mode is AgentInference, runtime is podman, and ollama URL and proxy image are set.
- `runJobSBAWithPodInference` creates pod, starts proxy container, waits via `waitForProxyReady` (health probe when not using placeholder command), then runs SBA container in pod with `buildSBARunArgsForPod`.
- Pod SBA env sets `envOllamaBaseURL = ollamaBaseURLInPod` (`http://localhost:11434`) in executor.
- Direct-steps path uses `buildSBARunArgs` with `--network=none` when not AgentInference or when ollama URL is unset.
- `prepareSBAJobAndWorkspace` pre-creates result.json with `os.WriteFile(..., 0o666)` and `os.Chmod(resultPath, 0o666)` so the container can write.
- Unit tests cover SBA pod inference path, readiness probe timeout, and direct vs pod mode.

Phase 2 gaps: none.
SBA inference E2E is explicitly deferred to Phase 8 in the plan; no gap for Phase 2 scope.

## Phase 3: Internal Proxy Endpoint Scaffolding

Reconciliation plan claims: endpoints `POST /v1/worker/internal/orchestrator/mcp:call` and `agent:ready`, interim auth (header-based) replaced in Phase 6, audit logging, unit tests.

### Phase 3 Verification

- Both endpoints are registered on the internal mux (`worker_node/cmd/worker-api/main.go`).
- Current auth: `validateInternalProxyRequest` requires loopback or Unix socket, then caller identity from context (set by identity-bound transport in Phase 5/6), then token from secure store; no agent-supplied token in request.
  Phase 6 completed: agent-supplied auth removed; worker-held tokens from secure store.
- Request body size cap: `decodeManagedProxyRequest` uses `maxManagedProxyBodyBytes` (1 MiB) via `MaxBytesReader`; response body similarly limited.
- Audit logging: "internal orchestrator proxy call" with endpoint, service_id, method, path, upstream_status, duration_ms (no token material).
- Unit tests: loopback vs non-loopback, missing identity, missing secure store, happy path with store (main_test.go).

Phase 3 gaps: none for Phase 3 scope (scaffolding; spec-compliant auth is Phase 5+6, which are implemented).

## Phase 4: Node-Local Secure Store and Agent Token Lifecycle

Reconciliation plan claims: encrypted-at-rest store under `<state_dir>/secrets/agent_tokens`, 0700/0600, fail-closed master key, AES-256-GCM, master key precedence (TPM/OS/systemd/env), FIPS reject env when FIPS on, best-effort secure erasure, token lifecycle (write/rotate/delete, agent_token_ref, expiry), no AGENT_TOKEN in container, redaction from WORKER_NODE_CONFIG_JSON, process-boundary doc, unit test for no secrets mount.

### Phase 4 Verification

- Path and permissions: Store root `state_dir/secrets` (0700); token dir `secrets/agent_tokens`; token files `service_id.json.enc` (0600).
  Default state_dir when unset: `/var/lib/cynode/state` (store.go).
- Algorithm: Spec requires post-quantum KEM (e.g. FIPS 203 ML-KEM) by default with AES-256-GCM for ciphertext; fallback to AES-256-GCM only when PQ not permitted (e.g. FIPS module without ML-KEM).
  Current implementation: AES-256-GCM, per-record nonce; envelope version and algorithm checked on read (implementation gap for PQ KEM remains until code is updated).
- Master key precedence: `resolveMasterKey()` order: TPM (stub, returns not configured), OS key store (stub), system credential (`CREDENTIALS_DIRECTORY` + file), env `CYNODE_SECURE_STORE_MASTER_KEY_B64`.
  Decode validates base64 and 32-byte length; invalid key returns ErrMasterKeyInvalid.
- FIPS: `isFIPSMode()` true when Linux `/proc/sys/crypto/fips_enabled` is "1" (fips_linux.go), or when env override or platform returns unknown (fail closed).
  When FIPS is on/unknown, `Open()` rejects env-backed key with `ErrFIPSRequiresNonEnvKey`.
- Secure erasure: `zeroBytes` used for plaintext and key in store and after decrypt; no runtime/secret in code (spec says SHOULD when available).
- Token lifecycle: `syncManagedServiceAgentTokens` on config apply: `computeDesiredAgentTokens` (direct token + agent_token_ref resolution), `reconcileAgentTokenStore` (delete stale, put desired).
  `resolveAgentTokenRef` implements kind=orchestrator_endpoint HTTP contract; resolution failures fail closed.
  Expiry: `GetAgentToken` checks `ExpiresAt` and returns `ErrTokenExpired` when not before now.
- No token in container: Node Manager does not inject AGENT_TOKEN into managed-service env; run args built by `BuildManagedServiceRunArgs` (nodemanager/runargs.go) only add UDS mount and proxy URL env vars.
- Redaction: `sanitizeNodeConfigForWorkerEnv` zeros `AgentToken` and sets `AgentTokenRef = nil` for each service before marshalling to `WORKER_NODE_CONFIG_JSON`.
  Unit test `TestSanitizeNodeConfigForWorkerEnv_RedactsTokenFields` asserts redaction.
- Process boundary: [2026-03-06_secure_store_process_boundary.md](2026-03-06_secure_store_process_boundary.md) documents writer (Node Manager), reader (Worker API internal proxy), same/split process, and no mount of `state_dir/secrets` into containers.
- No secrets mount: `BuildManagedServiceRunArgs` only adds a single `-v` mount: `stateDir/run/managed_agent_proxy/<service_id>:/run/cynode/managed_agent_proxy`.
  No path contains `secrets`.
  Unit test `TestBuildManagedServiceRunArgs_NoSecretsMount` in `worker_node/cmd/node-manager/main_test.go` iterates `-v` args and asserts no host path contains "secrets".
  BDD scenario "Managed-service container run args do not mount secure store" and steps in `_bdd/steps.go` assert the same using `nodemanager.BuildManagedServiceRunArgs`.

### Phase 4 Gaps

1. **Resolved (spec and requirements):** worker_node.md NodeLocalSecureStore previously stated "post-quantum resistant symmetric algorithm (e.g. AES-256-GCM)" (AES-256-GCM is not post-quantum).
  Spec now requires post-quantum KEM (e.g. NIST FIPS 203 ML-KEM) as default to protect key material, with AES-256-GCM for ciphertext; fallback to FIPS-approved symmetric AEAD only when PQ KEM is not available or not permitted.
  REQ-WORKER-0173 added: encryption at rest MUST use post-quantum KEM when permitted and MUST fall back to FIPS-approved symmetric AEAD when not.
  Implementation still uses AES-256-GCM only; code changes required to add ML-KEM and fallback logic (out of scope for this doc update).

2. Phase 4 validation doc vs test location:** [2026-03-06_phase4_validation.md](2026-03-06_phase4_validation.md) lists `TestBuildManagedServiceRunArgs_NoSecretsMount` under `cmd/node-manager/main_test.go`, which is correct.
  The reconciliation plan line 90 says "unit test asserts managed-service container run args never mount the secrets path"; the assertion is implemented both as that unit test and as the BDD step.
  No change required; noted for clarity.

## Recommendations

- Spec and requirements: NodeLocalSecureStore encryption wording updated; REQ-WORKER-0173 added.
  Implementation: add post-quantum KEM (e.g. ML-KEM) default and AES-256-GCM-only fallback when PQ not permitted to satisfy the updated spec and REQ-WORKER-0173.
- **No further doc changes required** for the Phase 4 spec/req gap; remaining work is implementation (PQ KEM + fallback).

## Traceability

- Phase 1: CYNAI.WORKER.NodeLocalInference, CYNAI.STANDS.InferenceOllamaAndProxy, REQ-WORKER-0114, REQ-WORKER-0115.
- Phase 2: Same inference scope; SBA path and direct-steps behavior per plan.
- Phase 3: CYNAI.WORKER.ManagedAgentProxyBidirectional (agent-to-orchestrator side); endpoints and auth evolution per plan.
- Phase 4: CYNAI.WORKER.NodeLocalSecureStore, AgentTokenStorageAndLifecycle, AgentTokensWorkerHeldOnly, SecureStoreProcessBoundary, REQ-WORKER-0165 through REQ-WORKER-0173.
