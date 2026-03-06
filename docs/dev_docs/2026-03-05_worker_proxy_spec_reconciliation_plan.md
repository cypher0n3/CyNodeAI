# Worker Proxy Spec Reconciliation Plan

## Metadata

- Date: 2026-03-05
- Last updated: 2026-03-06 (Phase 4a added)
- Scope: Worker proxy behavior only.
- Status: Draft implementation reconciliation plan.
- Inputs reviewed:
  - `docs/requirements/worker.md`
  - `docs/tech_specs/worker_node.md`
  - `docs/tech_specs/worker_api.md`
  - `docs/tech_specs/worker_node_payloads.md`
  - `docs/tech_specs/orchestrator.md`
  - `docs/tech_specs/ports_and_endpoints.md`
  - `docs/tech_specs/mcp_gateway_enforcement.md`
  - `docs/dev_docs/2026-03-05_pma_managed_service_spec_check.md`
  - `docs/dev_docs/2026-03-05_worker_agent_token_secure_holding_spec_gaps.md`
  - `worker_node/internal/inferenceproxy/proxy.go`
  - `worker_node/cmd/inference-proxy/main.go`
  - `worker_node/cmd/worker-api/main.go`
  - `worker_node/cmd/worker-api/executor/executor.go`

## Goal

Bring worker proxy behavior to spec-compliant state for both inference proxying and managed agent bidirectional proxying.
This plan is limited to code and tests.
It does not propose spec changes.

## Important Scope Note (Missing Subsystems)

This plan touches worker proxy endpoints and routing, but several spec-required subsystems that the proxy depends on are not yet implemented end-to-end.
The phases below must explicitly include building these subsystems (still within existing specs) to avoid "false completion" where endpoints exist but the security model is non-compliant.

### Recent Spec Updates Reflected in This Plan

- **worker_node.md:** Agent-To-Orchestrator UDS Binding (Required) now normatively defines host path `<state_dir>/run/managed_agent_proxy/<service_id>/proxy.sock`, container mount `/run/cynode/managed_agent_proxy/`, HTTP over UDS, and identity from accepting listener.
  Sockets MUST NOT be under the secure store path.
- **worker_node_payloads.md:** Capability report `managed_services.features` (e.g. `agent_orchestrator_proxy_identity_bound`, `agent_proxy_urls_auto`); `managed_services_status.services[].agent_to_orchestrator_proxy` with `mcp_gateway_proxy_url`, `ready_callback_proxy_url`, `binding`.
  When `binding=per_service_uds`, URLs MUST be `http+unix://...`.
  Config may set `orchestrator.mcp_gateway_proxy_url` and `ready_callback_proxy_url` to `auto`; worker then generates identity-bound endpoints, injects into container, and reports in `managed_services_status`.
  Config ack MUST include `managed_services_status` with generated endpoints when config had `auto`.
- **orchestrator.md:** `CYNAI.ORCHES.ManagedServicesWorkerManaged` - orchestrator MAY set proxy URL fields to `auto`; MUST route using worker-reported endpoints only (no compose DNS or direct host-port); ingests/stores `agent_to_orchestrator_proxy` for diagnostics and reconciliation.

## Execution Order (Mandatory)

Build and test new required components before any dependent work.

1. **Phase 1, 2, 3** (already done or in progress): inference proxy stabilization, SBA inference path, internal proxy endpoint scaffolding.
2. **Phase 4, then Phase 4a, then Phase 5** (next): Build and test the new required subsystems first.
   - Phase 4: Node-local secure store and agent token lifecycle (write/rotate/delete on config apply).
   - Phase 4a: Post-quantum KEM for secure store encryption at rest (REQ-WORKER-0173, NodeLocalSecureStore); fallback to AES-256-GCM only when PQ not permitted.
   - Phase 5: Identity-bound per-service UDS binding and transport.
   - Do not proceed to Phase 6, 7, or 8 until Phase 4, Phase 4a, and Phase 5 are implemented and tested.
3. **Phase 6** (depends on 4 and 5): Switch internal proxy auth to worker-held tokens; remove agent-supplied auth.
4. **Phase 7**: Desired-state wiring for managed service targets (can overlap with 6 once 4 and 5 are done).
5. **Phase 8** (depends on 5 and 7): Managed-service observed state reporting and routing source of truth; `auto` proxy URLs.

## Execution Status

- **Phase 1: Completed.**
  - [x] Default inference-proxy bind tightened to loopback semantics.
  - [x] Inference-proxy `/healthz` endpoint added.
  - [x] Fixed sleep removed from pod inference path in favor of bounded active readiness probing.
  - [x] Added unit tests for new phase 1 behavior.
- **Phase 2: Completed.**
  - [x] Added SBA agent-inference pod plus proxy sidecar execution path.
  - [x] SBA pod mode now injects `OLLAMA_BASE_URL=http://localhost:11434`.
  - [x] Preserved direct-steps SBA mode behavior and network isolation for non-pod SBA path.
  - [x] Added and updated unit tests for SBA pod-mode selection, args, diagnostics, and failure branches.
  - [x] Stabilized broad chat and non-SBA task-path runtime failures by ensuring PMA and OLLAMA are started coherently in setup flow.
  - [x] Removed worker-side long-run EOF failure mode by disabling 30s write-timeout for synchronous `/v1/worker/jobs:run`.
  - [x] Removed SBA `/job/result.json` permission-denied failures by forcing writable file mode after pre-create.
  - SBA inference E2E acceptance is deferred to Phase 8 (SBA inference depends on worker proxy being fully spec-compliant; see Phase 8).
- **Phase 3: Completed (endpoint scaffolding only; spec-compliant auth in Phase 5 + Phase 6).**
  - [x] Added internal worker proxy endpoints:
    - `POST /v1/worker/internal/orchestrator/mcp:call`
    - `POST /v1/worker/internal/orchestrator/agent:ready`
  - [x] Added interim auth checks (header-based) for internal proxy calls.
  - [x] Added audit logging fields for internal proxy calls.
  - [x] Added unit tests for internal proxy happy-path and auth/loopback failures.
- **Phase 4: Completed (required before Phase 6, 7, 8).**
  - [x] Implemented encrypted-at-rest node-local secure store under `<state_dir>/secrets/agent_tokens` with strict file permissions (0700 dirs, 0600 files), fail-closed master-key validation, and AES-256-GCM envelope format.
  - [x] Implemented config-apply token lifecycle in Node Manager: write/rotate/delete by `service_id`, including `agent_token_ref` (`kind=orchestrator_endpoint`) resolution and expiry validation.
  - [x] Removed managed-service container `AGENT_TOKEN` env injection; agent tokens remain worker-held.
  - [x] Redacted `agent_token` and `agent_token_ref` from `WORKER_NODE_CONFIG_JSON` worker env payload.
  - [x] Master key precedence implemented in spec order: TPM (stub), OS key store (stub), systemd credential, env fallback.
    TPM and OS key store sources return not-configured and are ready for future implementation.
  - [x] FIPS mode enforcement: when host reports FIPS mode (Linux `/proc/sys/crypto/fips_enabled`), env fallback is rejected (fail closed); FIPS-approved algorithm (AES-256-GCM) only.
  - [x] Best-effort secure erasure (zeroing) for master key and plaintext; Go 1.26 `runtime/secret` integration remains optional (SHOULD when available).
  - [x] Secure-store process-boundary documented in `docs/dev_docs/2026-03-06_secure_store_process_boundary.md`; unit test asserts managed-service container run args never mount the secrets path.
- **Phase 4a: Not started (addresses [phase1-4 gap report](2026-03-06_phase1_4_independent_validation_gap_report.md) Phase 4 gaps).**
  - [ ] Use post-quantum KEM (e.g. NIST FIPS 203 ML-KEM) by default to protect secure store key material; use strong symmetric AEAD (e.g. AES-256-GCM) for ciphertext; per-record nonce.
  - [ ] When PQ KEM is not available or not permitted (e.g. FIPS validated module without ML-KEM), use only FIPS-approved symmetric AEAD (e.g. AES-256-GCM) with per-record nonce.
  - [ ] Satisfy REQ-WORKER-0173 and NodeLocalSecureStore encryption-at-rest spec.
  - (Phase 4 validation doc vs test location: no change required; phase4_validation.md and plan line 90 are correct.)
- **Phase 5: In progress (required before Phase 6, 8).**
  - [x] Added dedicated internal listener configuration (`WORKER_INTERNAL_LISTEN_ADDR`).
  - [x] Added optional internal Unix domain socket listener (`WORKER_INTERNAL_LISTEN_UNIX`).
  - [x] Kept internal proxy routes off the public Worker API mux.
  - [x] Implemented worker-side per-service UDS listener paths at `<state_dir>/run/managed_agent_proxy/<service_id>/proxy.sock` (0700 parent dir, 0600 socket) with caller identity bound from accepting listener context.
  - [x] Container mount injection: node-manager now mounts only `<state_dir>/run/managed_agent_proxy/<service_id>` at `/run/cynode/managed_agent_proxy` for each managed service (path-safe `service_id` only).
  - [x] Report `managed_services.features` including `agent_orchestrator_proxy_identity_bound` (and when supported, `agent_proxy_urls_auto`).
  - [x] Enforce that only the identity-binding socket path is mounted into managed-service containers (never the secure store path).
  - [ ] Tests for `http+unix://...` URL reporting when `binding=per_service_uds` (Phase 8 reporting will consume this; UDS path format already tested in worker-api).
    UDS identity-resolution tests for token/service mismatch are now in place.
- **Phase 6: Completed.**
  - [x] Removed agent-supplied auth for internal proxy calls; worker resolves `service_id` from identity-bound transport and attaches worker-held tokens loaded from the secure store.
  - [x] Expanded audit attribution to include managed-service identity (service_id) and request context fields (without token material).
  - [x] Unit tests assert: no token in agent request; unknown identity fails closed; missing secure store fails closed.
  - [ ] Contract and end-to-end validation for internal agent-to-orchestrator proxy path (auth is now spec-compliant; E2E still pending).
- **Phase 7: In progress (desired-state wiring; can continue in parallel after Phase 4, 4a, and 5).**
  - [x] Worker API now consumes node-config payload (`WORKER_NODE_CONFIG_JSON`) to derive managed service proxy targets and (temporarily) to accept managed-service agent tokens for internal proxy auth; this MUST be replaced by secure-store backed worker-held tokens (Phase 4 + Phase 6).
  - [x] Node Manager now injects node-config JSON and orchestrator internal proxy base URL into worker runtime env on config application.
  - [x] Node Manager starts managed service containers from `managed_services.services[]` desired state when enabled.
  - [x] Orchestrator can include an agent token for managed service desired state (delivered to worker in node config).
  - [x] Node Manager materializes managed-service target env for routing (currently PMA -> node-local PMA base URL), while Worker API avoids deriving targets from unrelated inference config fields.
  - [x] Existing env-based target mapping remains as fallback.
  - [ ] Desired-state convergence and refresh behavior still needs runtime validation against full managed-service lifecycle.
- **Phase 8: In progress (blocked on Phase 5 and Phase 7).**
  - [x] Implemented managed-service observed state reporting (`managed_services_status`) in capability reports and config ack scaffolding.
  - [x] When config sets proxy URLs to `auto`: node-manager generates identity-bound endpoints, injects into container env, and reports them in `managed_services_status.services[].agent_to_orchestrator_proxy` with `binding=per_service_uds` and `http+unix://...` URLs.
  - [ ] Shift orchestrator and worker routing to worker-reported endpoints only (no compose DNS or direct host-port).
  - [ ] SBA inference E2E acceptance (e2e_140, e2e_145); run after worker proxy is spec-compliant (Phases 4, 5, 6 done).
    Resolve any remaining SBA pod workspace mount flakiness (`statfs ... /tmp/cynodeai-workspaces/...`) once full proxy path is in place.

## Normative Targets

This section lists the exact spec anchors this reconciliation plan targets.

### Inference Proxy Scope

- `CYNAI.WORKER.NodeLocalInference` in `docs/tech_specs/worker_node.md`.
- `CYNAI.STANDS.InferenceOllamaAndProxy` in `docs/tech_specs/ports_and_endpoints.md`.
- Requirement trace: `REQ-WORKER-0114`, `REQ-WORKER-0115`.

### Managed Agent Proxy Scope

- `CYNAI.WORKER.ManagedAgentProxyBidirectional` in `docs/tech_specs/worker_api.md`.
- `CYNAI.WORKER.WorkerProxyBidirectionalManagedAgents` in `docs/tech_specs/worker_node.md`.
- `CYNAI.MCPGAT.AgentTokensWorkerProxyOnly` in `docs/tech_specs/mcp_gateway_enforcement.md`.
- `CYNAI.WORKER.AgentTokensWorkerHeldOnly` in `docs/tech_specs/worker_node.md`.
- `CYNAI.WORKER.AgentTokenStorageAndLifecycle` in `docs/tech_specs/worker_node.md`.
- `CYNAI.WORKER.NodeLocalSecureStore` in `docs/tech_specs/worker_node.md`.
- Agent-To-Orchestrator UDS Binding (Required) in `docs/tech_specs/worker_node.md` (host path `<state_dir>/run/managed_agent_proxy/<service_id>/proxy.sock`, container mount `/run/cynode/managed_agent_proxy/`, identity from accepting listener).
- `CYNAI.WORKER.Payload.CapabilityReportV1` and `managed_services_status.services[].agent_to_orchestrator_proxy` in `docs/tech_specs/worker_node_payloads.md`.
- `node_configuration_payload_v1.managed_services.services[].orchestrator.{mcp_gateway_proxy_url,ready_callback_proxy_url}` (including `auto` sentinel) and config ack `managed_services_status` in `docs/tech_specs/worker_node_payloads.md`.
- `CYNAI.ORCHES.ManagedServicesWorkerManaged` in `docs/tech_specs/orchestrator.md` (routing from worker-reported endpoints, optional `auto` for proxy URLs).
- Requirement trace: `REQ-WORKER-0162`, `REQ-WORKER-0163`.

## Current Implementation Assessment

This section summarizes what currently exists and where the gaps remain.

### Implemented Baseline

- Inference proxy has the required request size limit of 10 MiB and request timeout of 120s.
- Worker API has orchestrator to managed-service proxy endpoint `POST /v1/worker/managed-services/{service_id}/proxy:http`.
- Worker API applies header allowlists and response body limits for that endpoint.
- Internal agent-to-orchestrator proxy endpoints exist, but current auth behavior is interim and is not spec-compliant until identity-bound transport and secure-store token attachment are implemented.

### Spec Compliance Gaps

1. **SBA inference E2E flakiness remains.**
   SBA inference is routed through the pod + proxy sidecar path, but E2E instability remains in workspace mounting for `e2e_140` and `e2e_145`.

2. **Worker-held tokens are not yet implemented end-to-end.**
   Updated specs require agent tokens to be delivered to the worker in config, written to the node-local secure store keyed by managed-service identity, and attached by the worker proxy when forwarding.
   Tokens MUST NOT be passed into agent containers or exposed via env vars, mounts, files, logs, or debug endpoints.
   Current implementation still uses env and in-memory token lists for internal proxy auth and does not implement secure store lifecycle.
   See `CYNAI.WORKER.AgentTokensWorkerHeldOnly`, `CYNAI.WORKER.AgentTokenStorageAndLifecycle`, and `CYNAI.WORKER.NodeLocalSecureStore`.

3. **Internal proxy caller identity selection needs spec-aligned determinism.**
   Updated specs require the worker proxy to deterministically select the correct token for the calling managed agent runtime by service identity.
   The implementation needs an explicit mechanism to determine the calling `service_id` for internal proxy calls without requiring secrets inside the agent container.

4. **Managed service observed-state reporting and routing are incomplete.**
   Specs require the worker to report `managed_services_status` and for the orchestrator to route to managed services using worker-reported endpoints.
   Current routing still relies on node-local target env mappings (e.g. `PMA_BASE_URL`) and does not fully use observed-state as the source of truth.

## Not-Yet-Built Components and Code Paths (Explicit)

This section enumerates the concrete missing subsystems that must exist for the worker proxy to be spec-compliant, even if some proxy routes already exist.

### Worker Node Secure Store (Host-Only Secret Persistence)

- **What is missing**
  - A node-local secure store implementation that persists orchestrator-issued secrets (agent tokens, worker bearer token, pull creds) under `${storage.state_dir}/secrets/` with encryption at rest.
  - Master key acquisition precedence (TPM, OS key store, systemd credential, env var fallback) and fail-closed behavior.
  - Permissions and "no container mount" boundary enforcement.
- **Why it matters**
  - Spec requires tokens to be durable across restarts and never exposed to managed-service containers or sandboxes.
- **Spec anchors**
  - `CYNAI.WORKER.NodeLocalSecureStore`
  - `CYNAI.WORKER.AgentTokenStorageAndLifecycle`
  - `CYNAI.WORKER.AgentTokensWorkerHeldOnly`
  - Supporting design details: `docs/dev_docs/2026-03-05_worker_agent_token_secure_holding_spec_gaps.md`

### Agent Token Lifecycle on Config Apply (Write, Rotate, Delete)

- **What is missing**
  - Node Manager (or config-apply component) writes tokens to the secure store keyed by `service_id` when applying `managed_services.services[]`.
  - Resolution path for `agent_token_ref` during config apply (where the reference is resolved and how failures fail closed).
  - Token delete/overwrite on service removal or token rotation.
  - Token expiry handling when expiry fields exist.
- **Why it matters**
  - Without lifecycle, the proxy cannot deterministically attach the right token, and old tokens may remain usable longer than allowed.
- **Spec anchors**
  - `CYNAI.WORKER.AgentTokenStorageAndLifecycle` algorithm steps 1, 4, 5

### Identity-Bound Internal Proxy Transport (Deterministic `service_id` Resolution)

- **What is missing**
  - Implementation of the **required** per-service UDS binding defined in worker_node.md:
    - Host: `<storage.state_dir>/run/managed_agent_proxy/<service_id>/` (dir 0700), `<service_id>/proxy.sock` (0600); sockets MUST NOT be under the secure store path.
    - Container: mount only that service's host dir at `/run/cynode/managed_agent_proxy/`; agent uses `/run/cynode/managed_agent_proxy/proxy.sock`.
    - Worker internal proxy serves HTTP over this UDS and resolves `service_id` from which UDS listener accepted the connection (no secrets in request).
  - Capability report: `managed_services.features` including `agent_orchestrator_proxy_identity_bound` (and when supported, `agent_proxy_urls_auto`).
- **Why it matters**
  - Spec requires the worker MUST NOT rely on any secret being present in the agent container or request to establish identity.
  - Header-based "agent token presented by agent" is explicitly non-compliant with `AgentTokensWorkerHeldOnly`.
- **Spec anchors**
  - `docs/tech_specs/worker_node.md` Agent-To-Orchestrator UDS Binding (Required)
  - `docs/tech_specs/worker_api.md` `CYNAI.WORKER.ManagedAgentProxyBidirectional`
  - `CYNAI.WORKER.AgentTokenStorageAndLifecycle` algorithm step 3
  - `docs/tech_specs/worker_node_payloads.md` capability report `managed_services.features`, `managed_services_status.services[].agent_to_orchestrator_proxy.binding`

### Removal of Agent-Supplied Auth for Internal Proxy Calls (Fail-Closed)

- **What is missing**
  - Internal proxy endpoints (`/v1/worker/internal/orchestrator/*`) must transition from "agent supplies token headers" to "worker resolves identity by binding and attaches token from secure store".
  - Unknown/ambiguous identity must fail closed, with audit-safe error responses that do not leak secrets.
- **Why it matters**
  - The spec is explicit that the managed agent container MUST NOT receive or present agent tokens.
- **Spec anchors**
  - `CYNAI.WORKER.AgentTokensWorkerHeldOnly`
  - `CYNAI.MCPGAT.AgentTokensWorkerProxyOnly`

### Managed-Service Observed State Reporting and Routing Source of Truth

- **What is missing**
  - Worker reports `managed_services_status` in capability reports and in config ack (required when config contains agent-runtime managed services).
  - When config sets `orchestrator.mcp_gateway_proxy_url` or `ready_callback_proxy_url` to `auto`, worker MUST generate identity-bound endpoints, inject them into the managed service container, and report them in `managed_services_status.services[].agent_to_orchestrator_proxy` (with `binding` e.g. `per_service_uds`; URLs `http+unix://...` when UDS).
  - Orchestrator routes to managed services using worker-reported endpoints only (no compose DNS or direct host-port).
- **Why it matters**
  - Desired state without observed state leaves the orchestrator blind to actual endpoints and readiness; routing cannot be resilient.
- **Spec anchors**
  - `docs/tech_specs/worker_node.md` managed service section (`managed_services_status`)
  - `docs/tech_specs/worker_node_payloads.md` `node_configuration_payload_v1.managed_services.services[].orchestrator` (`auto`), config ack `managed_services_status`
  - `docs/tech_specs/orchestrator.md` `CYNAI.ORCHES.ManagedServicesWorkerManaged`

## Resolved Spec Gaps and Ambiguities

This section records spec gaps or ambiguity that affected the plan and are now resolved via spec and requirement updates.

### `agent_token_ref` Schema and Resolution

- **Status:** Resolved.
- **Resolution:** Defined `agent_token_ref` schema and defined a worker-side resolution contract and failure behavior.
- **Spec anchors:**
  - [`CYNAI.WORKER.Payload.AgentTokenRef`](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-agenttokenref)
  - [`CYNAI.WORKER.AgentTokenRefResolution`](../tech_specs/worker_node.md#spec-cynai-worker-agenttokenrefresolution)
- **Requirements:** [`REQ-WORKER-0171`](../requirements/worker.md#req-worker-0171).

### Token Expiry Field in Config Payload

- **Status:** Resolved.
- **Resolution:** Added `managed_services.services[].orchestrator.agent_token_expires_at` and referenced it from the worker token lifecycle algorithm.
- **Spec anchors:**
  - [`CYNAI.WORKER.Payload.ConfigurationV1`](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1)
  - [`CYNAI.WORKER.AgentTokenStorageAndLifecycle`](../tech_specs/worker_node.md#spec-cynai-worker-agenttokenstorageandlifecycle)

### Secure Store: FIPS Mode and Go 1.26 `runtime/secret`

- **Status:** Resolved.
- **Resolution:** Worker secure store spec already defined FIPS requirements and `runtime/secret` guidance, and now explicitly requires best-effort secure erasure when `runtime/secret` is not available.
- **Spec anchors:** [`CYNAI.WORKER.NodeLocalSecureStore`](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalsecurestore).
- **Requirements:** [`REQ-WORKER-0170`](../requirements/worker.md#req-worker-0170).

### Capability Leases vs Agent Tokens (Scope)

- **Status:** Resolved.
- **Resolution:** Clarified scope in the worker spec.
  The managed agent internal proxy token lifecycle is agent-token-only.
  Capability leases remain part of the secure store scope but are for other interfaces (e.g. node-local sandbox control).
- **Spec anchors:** [`CYNAI.WORKER.AgentTokensWorkerHeldOnly`](../tech_specs/worker_node.md#spec-cynai-worker-agenttokensworkerheldonly).

### Orchestrator-Side Responsibilities (Out of Scope)

- **Clarification:** This plan is worker-only.
  Full end-to-end compliance also requires orchestrator to: issue agent tokens (and optionally support agent_token_ref), associate user context for PAA/user-scoped agents, ingest and store `managed_services_status` and `agent_to_orchestrator_proxy`, and route to managed services using worker-reported endpoints only.
- **Impact:** Worker proxy can be spec-compliant without orchestrator changes, but E2E (including SBA inference and PMA handoff) depends on orchestrator behavior.
- **Recommendation:** Keep plan scope as-is; call out orchestrator work in a separate checklist or doc if needed for release.

### Node Manager vs Worker API Process Boundary

- **Status:** Resolved.
- **Resolution:** Defined a secure store process boundary spec item and added a worker requirement for multi-process deployments.
- **Spec anchors:** [`CYNAI.WORKER.SecureStoreProcessBoundary`](../tech_specs/worker_node.md#spec-cynai-worker-securestoreprocessboundary).
- **Requirements:** [`REQ-WORKER-0172`](../requirements/worker.md#req-worker-0172).

### Binding Type: `per_service_loopback_listener`

- **Status:** Resolved.
- **Resolution:** Clarified that the normative identity binding for managed agent internal proxy is per-service UDS.
  Loopback binding remains a payload-level allowed value but is not specified as an identity-binding mechanism in the worker spec.
- **Spec anchors:**
  - [`CYNAI.WORKER.AgentTokenStorageAndLifecycle`](../tech_specs/worker_node.md#spec-cynai-worker-agenttokenstorageandlifecycle)
  - [`CYNAI.WORKER.Payload.CapabilityReportV1`](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-capabilityreport-v1)

## Reconciliation Plan

This section defines phased implementation work in execution order.

## Phase 1: Stabilize Inference Proxy Data Plane

Phase summary: remove startup timing fragility and tighten proxy exposure model.

### Phase 1 Change Set

- Make inference proxy bind explicit loopback by default for pod use (`127.0.0.1:11434`).
- Add `/healthz` endpoint in inference proxy process for deterministic readiness checks.
- Replace fixed `sleep 2s` in pod inference path with bounded active probe loop against proxy health.
- Add retry budget for proxy container startup and probe steps.

### Phase 1 Acceptance Criteria

- Pod inference startup does not rely on sleep timing.
- Proxy readiness failures return deterministic worker error details.
- Existing inference proxy tests are extended for bind and health behaviors.

## Phase 2: Route SBA Inference Through Node-Local Proxy Model

Phase summary: make SBA inference follow the same node-local proxy topology as other inference workloads.

### Phase 2 Change Set

- Introduce SBA inference execution path that uses pod + proxy sidecar when execution mode requires inference.
- Ensure SBA container sees `OLLAMA_BASE_URL=http://localhost:11434` in proxy mode.
- Keep direct-steps SBA mode on `--network=none` and no proxy.
- Preserve timeout and output limit semantics already enforced by Worker API.

### Phase 2 Acceptance Criteria

- SBA inference jobs use localhost proxy endpoint in runtime argv and diagnostics.
- SBA inference E2E scenarios pass without host-direct upstream requirement.
- Regression tests confirm direct-steps mode remains network isolated.

## Phase 3: Implement Agent to Orchestrator Internal Proxy Endpoints

Phase summary: implement missing worker internal proxy endpoints required for bidirectional managed agent routing.

### Phase 3 Change Set

- Implement `POST /v1/worker/internal/orchestrator/mcp:call`.
- Implement `POST /v1/worker/internal/orchestrator/agent:ready`.
- Enforce request and response size caps and strict header allowlists.
- Add explicit upstream allowlist mapping from applied config model.

### Phase 3 Acceptance Criteria

- Endpoints exist and pass contract tests.
- Endpoints enforce strict request validation (size caps, header allowlists) and fail closed on invalid requests.
- Endpoints are not considered spec-compliant for managed-agent use until:
  - calling identity is resolved via identity-bound transport (no secrets in agent container or request), and
  - worker-held tokens are loaded from the node-local secure store and attached by the worker proxy.

## Phase 4: Implement Node-Local Secure Store + Agent Token Lifecycle (Prerequisite)

Phase summary: implement the worker node secure store and token lifecycle required before identity-bound internal proxy auth can be made spec-compliant.

### Phase 4 Change Set

- Implement `CYNAI.WORKER.NodeLocalSecureStore` backing and master key acquisition precedence.
- Enforce strict file ownership and permissions, host-only boundary, and encryption at rest.
- When host is in FIPS mode, use only FIPS-approved algorithms and FIPS-validated modules per worker_node.md.
- Use Go 1.26 `runtime/secret` (or best-effort erasure on unsupported platforms) for master key and decrypted plaintext handling per worker_node.md.
- On config apply, resolve `managed_services.services[].orchestrator.agent_token` or `agent_token_ref` (when spec defines resolution) and write to secure store keyed by `service_id`.
- Implement token rotation and deletion on config updates and service removal.
- Implement expiry handling when expiry is provided (e.g. `agent_token_expires_at` when defined in payload); fail closed on expired token; request config refresh where applicable.
- Ensure tokens and master key material never appear in env vars, mounts, files accessible to containers, logs, telemetry, or debug endpoints.

### Phase 4 Acceptance Criteria

- Secure store works across restarts and fails closed on invalid master key configuration.
- Tokens are durable across restarts and are overwritten or deleted on config updates and service removal.
- `agent_token_ref` resolution failures fail closed without leaking secrets.
- No managed-service container or sandbox can access the secure store path (only identity-binding sockets may be mounted later).

## Phase 4A: Post-Quantum KEM for Secure Store Encryption at Rest

Phase summary: address the Phase 4 gap identified in the [phase1-4 independent validation gap report](2026-03-06_phase1_4_independent_validation_gap_report.md).
Spec and REQ-WORKER-0173 now require post-quantum key encapsulation by default with FIPS-approved symmetric fallback when PQ is not permitted; implementation currently uses AES-256-GCM only.

### Phase 4A Change Set

- Use a post-quantum key encapsulation mechanism (e.g. NIST FIPS 203 ML-KEM) by default to protect the key material used for secure store encryption at rest; use a strong symmetric AEAD (e.g. AES-256-GCM) for the ciphertext; each record MUST use a distinct nonce.
- When the post-quantum KEM is not available or not permitted (e.g. FIPS-only environment where the validated cryptographic module does not yet include ML-KEM), use only a FIPS-approved symmetric AEAD (e.g. AES-256-GCM) with a per-record nonce.
- Preserve backward compatibility or migration path for existing AES-256-GCM-only envelopes (e.g. envelope version/algorithm id so readers can distinguish PQ-wrapped vs AEAD-only).
- No change required for "Phase 4 validation doc vs test location" gap: phase4_validation.md and the plan's unit-test assertion wording are correct; no code or doc change for that item.

### Phase 4A Acceptance Criteria

- Default path uses post-quantum KEM to protect key material and AES-256-GCM for ciphertext; per-record nonce.
- Fallback path uses only FIPS-approved symmetric AEAD when PQ KEM is not permitted; behavior consistent with REQ-WORKER-0170 in FIPS mode.
- REQ-WORKER-0173 and NodeLocalSecureStore encryption-at-rest bullets are satisfied.
- Unit tests cover both default (PQ KEM + AEAD) and fallback (AEAD-only) code paths where applicable.

## Phase 5: Identity-Bound Internal Proxy Transport and Binding Hardening

Phase summary: implement the required per-service UDS binding and optional loopback so the worker can deterministically resolve calling `service_id` without secrets in the agent container or request.

### Phase 5 Change Set

- Implement the normative Agent-To-Orchestrator UDS binding from worker_node.md:
  - Host: base `${storage.state_dir}/run/managed_agent_proxy/` (or `/var/lib/cynode/state/run/managed_agent_proxy/` when unset); per service `<base>/<service_id>/` (0700), `<base>/<service_id>/proxy.sock` (0600).
  - Sockets MUST NOT be under the secure store path (`.../secrets/`).
  - Container: mount only that service's host directory at `/run/cynode/managed_agent_proxy/`; agent uses `/run/cynode/managed_agent_proxy/proxy.sock`.
  - Worker internal proxy serves HTTP over this UDS; resolve `service_id` from the UDS listener that accepted the connection.
- Do not expose internal proxy routes on non-loopback listeners (when loopback is also supported).
- Capability report: include `managed_services.features` with `agent_orchestrator_proxy_identity_bound`; when `auto` URL support is implemented, add `agent_proxy_urls_auto`.
- Ensure only the identity-binding socket path is mounted into the managed service container; never mount the secure store path.

### Phase 5 Acceptance Criteria

- Internal proxy endpoints are unreachable from non-loopback interfaces (where applicable).
- Unknown, ambiguous, or unresolvable caller identities fail closed.
- Tests cover UDS binding identity resolution; when `binding=per_service_uds`, reported proxy URLs use `http+unix://...` per payloads spec.

## Phase 6: Switch Internal Proxy Auth to Worker-Held Tokens + Audit Attribution

Phase summary: remove agent-supplied credentials entirely and use the secure store to attach the correct agent token for agent-originated requests.

### Phase 6 Change Set

- Remove interim header-based auth that assumes the agent can present tokens.
- For internal proxy calls:
  - Resolve `service_id` via identity-bound transport (Phase 5).
  - Load the corresponding token from the node-local secure store (Phase 4).
  - Attach the token to outbound requests to orchestrator MCP gateway and callbacks.
  - Unknown or missing token MUST fail closed.
- Expand audit attribution to include `service_id`, `service_type`, proxy operation, and timing (without any token material).

### Phase 6 Acceptance Criteria

- Internal proxy calls succeed without any token present in the agent container environment or request.
- Unit and integration tests assert tokens are not leaked via logs or process env.
- Audit logs include managed agent identity and request context fields where available.

## Phase 7: Desired-State Wiring for Managed Service Targets

Phase summary: move routing source of truth from env-driven mapping to orchestrator-directed desired state.

### Phase 7 Change Set

- Replace env-only service target mapping with node-config-driven desired state source.
- Keep env source as optional dev fallback behind explicit flag.

### Phase 7 Acceptance Criteria

- Managed-service proxy routing survives config refreshes and restarts.
- No manual env injection is required for production path.

## Phase 8: Managed-Service Observed State Reporting + Routing Source of Truth

Phase summary: implement managed-service observed state reporting, support `auto` for agent-to-orchestrator proxy URLs, and shift routing to worker-reported endpoints.

### Phase 8 Change Set

- Implement worker reporting of `managed_services_status` in capability reports and in config ack (required when config contains agent-runtime managed services per worker_node_payloads config ack schema).
- When config sets `orchestrator.mcp_gateway_proxy_url` or `ready_callback_proxy_url` to `auto`: worker MUST generate identity-bound endpoints (e.g. per Phase 5 UDS), inject them into the managed service container, and report them in `managed_services_status.services[].agent_to_orchestrator_proxy` with `mcp_gateway_proxy_url`, `ready_callback_proxy_url`, and `binding` (e.g. `per_service_uds`); when UDS, URLs MUST be `http+unix://...` per payloads spec.
- Config ack: when applied config had `auto` for either proxy URL, worker MUST include generated concrete endpoints in config ack `managed_services_status` and MUST have injected those values into the container.
- Orchestrator and worker routing use worker-reported endpoints as the production source of truth; no compose DNS or direct host-port for managed services.
- Keep env-derived target mapping as an explicitly-scoped development fallback only.

### Phase 8 Acceptance Criteria

- Orchestrator-to-managed-service proxy routing uses worker-reported endpoints in production path.
- When orchestrator sends `auto` for proxy URLs, worker generates identity-bound endpoints, injects into container, and reports in `managed_services_status` and config ack.
- Managed-service status reporting is stable across restarts and config refresh; `binding` and URL format conform to worker_node_payloads.
- Routing fails closed when endpoints are missing or inconsistent with desired state.

## Test and Validation Plan

This section defines verification layers and target suites.

### Unit Test Coverage

- `inferenceproxy`: health endpoint, bind behavior, timeout propagation, body limit.
- `worker-api/executor`: SBA inference proxy path, readiness probe logic, failure branches.
- `worker-api`: internal proxy endpoint auth, header allowlist, size-limit, route exposure.
- `securestore`: master key precedence and validation, encryption-at-rest write/read, token lifecycle (write/rotate/delete/expiry), and "no log" invariants.

### Integration Test Coverage

- Worker API + executor with fake upstream inference service and proxy sidecar.
- Managed-service proxy end to end against local mock service.
- Agent to orchestrator internal proxy calls via loopback and UDS.
- Node Manager config apply writes tokens to secure store and managed-service container cannot access secure store path.

### End-To-End Targets

- `e2e_090_task_inference` stability across repeated runs.
- `e2e_123_sba_task`, `e2e_140_sba_task_inference`, `e2e_145_sba_inference_reply`.
- New E2E for internal agent to orchestrator proxy path once endpoints are wired.

## Delivery Order and Risk

- **Mandatory order:** Phase 1 -> Phase 2 -> Phase 3 (done or in progress).
  Then **Phase 4 -> Phase 4a -> Phase 5** (build and test new required components first).
  Then Phase 6 -> Phase 7 -> Phase 8.
  Do not start Phase 6, 7, or 8 until Phase 4, Phase 4a, and Phase 5 are implemented and tested.
- Highest operational risk is in Phase 2 because it changes SBA inference execution topology.
- Highest security risk is unresolved until Phase 6 is complete (Phase 4, 4a, and 5 are prerequisites).

## Definition of Done

- Worker proxy implementation conforms to listed spec IDs and requirement traces.
- SBA and inference E2Es are green in repeated full-demo runs.
- Internal proxy endpoints are implemented, identity-bound (loopback or UDS constrained), and worker-held token forwarding is secure-store backed.
- Managed-service routing uses desired state plus worker-reported observed endpoints as the production source of truth.
