# Worker Proxy Spec Reconciliation Plan

## Metadata

- Date: 2026-03-05
- Scope: Worker proxy behavior only.
- Status: Draft implementation reconciliation plan.
- Inputs reviewed:
  - `docs/requirements/worker.md`
  - `docs/tech_specs/worker_node.md`
  - `docs/tech_specs/worker_api.md`
  - `docs/tech_specs/ports_and_endpoints.md`
  - `worker_node/internal/inferenceproxy/proxy.go`
  - `worker_node/cmd/inference-proxy/main.go`
  - `worker_node/cmd/worker-api/main.go`
  - `worker_node/cmd/worker-api/executor/executor.go`

## Goal

Bring worker proxy behavior to spec-compliant state for both inference proxying and managed agent bidirectional proxying.
This plan is limited to code and tests.
It does not propose spec changes.

## Execution Status

- Phase 1: Completed.
  - [x] Default inference-proxy bind tightened to loopback semantics.
  - [x] Inference-proxy `/healthz` endpoint added.
  - [x] Fixed sleep removed from pod inference path in favor of bounded active readiness probing.
  - [x] Added unit tests for new phase 1 behavior.
- Phase 2: In progress.
  - [x] Added SBA agent-inference pod plus proxy sidecar execution path.
  - [x] SBA pod mode now injects `OLLAMA_BASE_URL=http://localhost:11434`.
  - [x] Preserved direct-steps SBA mode behavior and network isolation for non-pod SBA path.
  - [x] Added and updated unit tests for SBA pod-mode selection, args, diagnostics, and failure branches.
  - [x] Stabilized broad chat and non-SBA task-path runtime failures by ensuring PMA and OLLAMA are started coherently in setup flow.
  - [x] Removed worker-side long-run EOF failure mode by disabling 30s write-timeout for synchronous `/v1/worker/jobs:run`.
  - [x] Removed SBA `/job/result.json` permission-denied failures by forcing writable file mode after pre-create.
  - [ ] SBA inference E2E acceptance remains open; latest full-suite runs still show SBA pod workspace mount flakiness (`statfs ... /tmp/cynodeai-workspaces/...: no such file or directory`) in `e2e_140` and `e2e_145`.
- Phase 3: In progress.
  - [x] Added internal worker proxy endpoints:
    - `POST /v1/worker/internal/orchestrator/mcp:call`
    - `POST /v1/worker/internal/orchestrator/agent:ready`
  - [x] Added strict auth checks for agent token and capability-lease style token headers.
  - [x] Added audit logging fields for internal proxy calls.
  - [x] Added unit tests for internal proxy happy-path and auth/loopback failures.
  - [ ] Contract and end-to-end validation remains open.
- Phase 4: In progress.
  - [x] Added dedicated internal listener configuration (`WORKER_INTERNAL_LISTEN_ADDR`).
  - [x] Added optional internal Unix domain socket listener (`WORKER_INTERNAL_LISTEN_UNIX`).
  - [x] Kept internal proxy routes off the public Worker API mux.
  - [ ] Full authz scope validation and audit attribution expansion remains open.
- Phase 5: In progress.
  - [x] Worker API now consumes node-config payload (`WORKER_NODE_CONFIG_JSON`) to derive managed service proxy targets and internal agent tokens.
  - [x] Node Manager now injects node-config JSON and orchestrator internal proxy base URL into worker runtime env on config application.
  - [x] Node Manager now materializes managed-service target env from desired service types (currently PMA -> node-local PMA base URL), while Worker API avoids deriving targets from unrelated inference config fields.
  - [x] Existing env-based target mapping remains as fallback.
  - [ ] Desired-state convergence and refresh behavior still needs runtime validation against full managed-service lifecycle.

## Normative Targets

This section lists the exact spec anchors this reconciliation plan targets.

### Inference Proxy Scope

- `CYNAI.WORKER.NodeLocalInference` in `docs/tech_specs/worker_node.md`.
- `CYNAI.STANDS.InferenceOllamaAndProxy` in `docs/tech_specs/ports_and_endpoints.md`.
- Requirement trace: `REQ-WORKER-0114`, `REQ-WORKER-0115`.

### Managed Agent Proxy Scope

- `CYNAI.WORKER.ManagedAgentProxyBidirectional` in `docs/tech_specs/worker_api.md`.
- `CYNAI.WORKER.WorkerProxyBidirectionalManagedAgents` in `docs/tech_specs/worker_node.md`.
- Requirement trace: `REQ-WORKER-0162`, `REQ-WORKER-0163`.

## Current Implementation Assessment

This section summarizes what currently exists and where the gaps remain.

### Implemented Baseline

- Inference proxy has the required request size limit of 10 MiB and request timeout of 120s.
- Worker API has orchestrator to managed-service proxy endpoint `POST /v1/worker/managed-services/{service_id}/proxy:http`.
- Worker API applies header allowlists and response body limits for that endpoint.

### Spec Compliance Gaps

1. **SBA inference does not use proxy sidecar path.**
   In `executor.go`, non-SBA jobs with `use_inference` can use pod + proxy sidecar, but SBA jobs use direct `OLLAMA_BASE_URL=<upstream>` instead of `localhost` in an isolated pod path.
   This diverges from `CYNAI.WORKER.NodeLocalInference` Option A semantics.

2. **Inference proxy bind behavior is broader than spec intent.**
   `cmd/inference-proxy` listens on `:11434`.
   Spec language expects localhost endpoint semantics for sandbox usage.
   Binding should be explicit to loopback in pod namespace (`127.0.0.1:11434`) for least exposure.

3. **Proxy startup/readiness handshake is weak.**
   Pod inference path sleeps for a fixed 2 seconds after proxy start.
   There is no readiness probe or bounded retry loop before launching sandbox.
   This is likely a root cause for flakiness and transient EOF behavior.

4. **Agent to orchestrator proxy endpoints are missing.**
   Worker API currently does not implement:
   - `POST /v1/worker/internal/orchestrator/mcp:call`
   - `POST /v1/worker/internal/orchestrator/agent:ready`
   This is a direct gap vs `REQ-WORKER-0162`.

5. **Internal proxy auth model is not implemented per spec.**
   Spec requires orchestrator-issued agent credentials or capability leases for internal proxy calls and fail-closed behavior.
   Current code only uses worker bearer token for orchestrator to managed-service calls.

6. **Internal proxy binding constraints are not implemented.**
   Spec requires loopback or UDS only for agent to orchestrator proxy endpoints.
   No dedicated internal listener exists yet.

7. **Managed-service target source diverges from desired-state model.**
   Current target mapping comes from `WORKER_MANAGED_SERVICE_TARGETS_JSON` env var.
   Spec model is orchestrator-directed managed services desired state.

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
- Add explicit upstream allowlist mapping from applied managed-service config model.

### Phase 3 Acceptance Criteria

- Endpoints exist and pass contract tests.
- Requests fail closed on missing auth or invalid scope.
- Proxy forwarding behavior is auditable and deterministic.

## Phase 4: Internal Proxy AuthN/AuthZ and Binding Hardening

Phase summary: enforce loopback or UDS binding and lease based authorization for internal proxy usage.

### Phase 4 Change Set

- Add dedicated internal listener config for loopback and optional UDS.
- Do not expose internal proxy routes on non-loopback listeners.
- Validate orchestrator-issued agent token or capability lease on internal proxy requests.
- Enforce task/context scoping checks from lease claims.

### Phase 4 Acceptance Criteria

- Internal proxy endpoints are unreachable from non-loopback interfaces.
- AuthN/AuthZ tests cover valid, expired, malformed, and out-of-scope credentials.
- Audit logs include agent identity and task context.

## Phase 5: Desired-State Wiring for Managed Service Targets

Phase summary: move routing source of truth from env-driven mapping to orchestrator-directed desired state.

### Phase 5 Change Set

- Replace env-only service target mapping with node-config-driven desired state source.
- Keep env source as optional dev fallback behind explicit flag.
- Ensure worker reports managed service observed endpoints consistent with proxy routing.

### Phase 5 Acceptance Criteria

- Managed-service proxy routing survives config refreshes and restarts.
- No manual env injection is required for production path.

## Test and Validation Plan

This section defines verification layers and target suites.

### Unit Test Coverage

- `inferenceproxy`: health endpoint, bind behavior, timeout propagation, body limit.
- `worker-api/executor`: SBA inference proxy path, readiness probe logic, failure branches.
- `worker-api`: internal proxy endpoint auth, header allowlist, size-limit, route exposure.

### Integration Test Coverage

- Worker API + executor with fake upstream inference service and proxy sidecar.
- Managed-service proxy end to end against local mock service.
- Agent to orchestrator internal proxy calls via loopback and UDS.

### End-To-End Targets

- `e2e_090_task_inference` stability across repeated runs.
- `e2e_123_sba_task`, `e2e_140_sba_task_inference`, `e2e_145_sba_inference_reply`.
- New E2E for internal agent to orchestrator proxy path once endpoints are wired.

## Delivery Order and Risk

- Recommended order: Phase 1 -> Phase 2 -> Phase 3 -> Phase 4 -> Phase 5.
- Highest operational risk is in Phase 2 because it changes SBA inference execution topology.
- Highest security risk is unresolved until Phase 4 is complete.

## Definition of Done

- Worker proxy implementation conforms to listed spec IDs and requirement traces.
- SBA and inference E2Es are green in repeated full-demo runs.
- Internal proxy endpoints are implemented, loopback or UDS constrained, and lease-authenticated.
