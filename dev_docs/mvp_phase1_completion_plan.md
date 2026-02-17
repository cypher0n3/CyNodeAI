# MVP Phase 1 Completion Plan

- [1 Spec Gaps and Clarifications Needed](#1-spec-gaps-and-clarifications-needed)
- [2 Current State Versus Phase 1 Requirements](#2-current-state-versus-phase-1-requirements)
  - [2.1 Orchestrator Requirements](#21-orchestrator-requirements)
  - [2.2 Node Requirements](#22-node-requirements)
  - [2.3 User API Gateway Requirements](#23-user-api-gateway-requirements)
- [3 Phase 1 Work Plan (4-6 Hour Chunks)](#3-phase-1-work-plan-4-6-hour-chunks)
  - [3.1 Chunk 01 (4-6 Hours): Lock the Phase 1 Acceptance Criteria](#31-chunk-01-4-6-hours-lock-the-phase-1-acceptance-criteria)
  - [3.2 Chunk 02 (4-6 Hours): Make Bootstrap Payload Spec-Compliant Enough to Enable Config Delivery](#32-chunk-02-4-6-hours-make-bootstrap-payload-spec-compliant-enough-to-enable-config-delivery)
  - [3.3 Chunk 03 (4-6 Hours): Implement Minimum Node Config Delivery API in the Control Plane](#33-chunk-03-4-6-hours-implement-minimum-node-config-delivery-api-in-the-control-plane)
  - [3.4 Chunk 04 (4-6 Hours): Update Node Manager to Fetch Config and Start Node Services in the Spec Order](#34-chunk-04-4-6-hours-update-node-manager-to-fetch-config-and-start-node-services-in-the-spec-order)
  - [3.5 Chunk 05 (4-6 Hours): Make Worker API Sandbox Execution Match Phase 1 Spec Constraints](#35-chunk-05-4-6-hours-make-worker-api-sandbox-execution-match-phase-1-spec-constraints)
  - [3.6 Chunk 06 (4-6 Hours): Make Orchestrator Dispatch Node-Aware (Single Node, Spec-Shaped)](#36-chunk-06-4-6-hours-make-orchestrator-dispatch-node-aware-single-node-spec-shaped)
  - [3.7 Chunk 07 (4-6 Hours): End-to-End Phase 1 Demo Hardening](#37-chunk-07-4-6-hours-end-to-end-phase-1-demo-hardening)
- [4 Phase 1 Definition of Done](#4-phase-1-definition-of-done)
  - [4.1 Startup Preconditions for Inference](#41-startup-preconditions-for-inference)
- [5 References](#5-references)

## 1 Spec Gaps and Clarifications Needed

This plan was generated on 2026-02-17.

The items below are blocking ambiguities or known implementation-versus-spec mismatches that must be resolved to claim Phase 1 compliance.

- The node bootstrap payload implemented today does not match the normative bootstrap payload shape.
  The spec requires orchestrator endpoints (including a node config URL), and optionally trust and initial config version.
  We need to confirm whether Phase 1 requires full bootstrap payload compliance, or whether we can defer some fields while still keeping wire compatibility.
  See [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md).

- Worker API authentication is specified as a bearer token delivered via the orchestrator node configuration payload.
  The current implementation uses environment variables on both sides.
  We need to decide the minimum Phase 1 config delivery behavior for this token (static token versus expiring token with refresh).
  See [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md) and [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md).

- "Config delivery" is a Phase 1 requirement, but the control-plane currently has no node configuration endpoints.
  We need to confirm the minimal Phase 1 API surface for config delivery and acknowledgement.
  The spec defines a configuration payload and an acknowledgement payload, but it does not mandate the exact REST endpoint paths for config delivery.
  See [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md) and [`docs/tech_specs/orchestrator.md`](../docs/tech_specs/orchestrator.md).

## 2 Current State Versus Phase 1 Requirements

This section cross-checks the status report in [`dev_docs/PHASE1_STATUS.md`](./PHASE1_STATUS.md) against the code currently in the repo and the Phase 1 bullets in [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).

### 2.1 Orchestrator Requirements

- Node registration (PSK => JWT) is implemented.
  The control-plane exposes `POST /v1/nodes/register` and issues a node JWT.
  See [`orchestrator/cmd/control-plane/main.go`](../orchestrator/cmd/control-plane/main.go) and [`orchestrator/internal/handlers/nodes.go`](../orchestrator/internal/handlers/nodes.go).

- Capability ingest is implemented, but the payload schema is incomplete versus the normative spec.
  The control-plane stores capability snapshots and updates a capability hash.
  See [`orchestrator/internal/handlers/nodes.go`](../orchestrator/internal/handlers/nodes.go) and [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md).

- Configuration delivery is not implemented.
  There is no node config endpoint, and the bootstrap response does not include the required orchestrator endpoints or a node config URL.
  See [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md) and [`docs/tech_specs/orchestrator.md`](../docs/tech_specs/orchestrator.md).

- Job dispatch is implemented, but it is single-node and not node-address-aware.
  The dispatcher runs inside the control-plane process and calls a single Worker API URL from environment configuration.
  It does not use per-node worker endpoints from a node configuration payload.
  See [`orchestrator/cmd/control-plane/dispatcher.go`](../orchestrator/cmd/control-plane/dispatcher.go) and [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md).

- Result collection is implemented.
  The dispatcher stores the Worker API response JSON on the job, updates job and task statuses, and stores a task summary.
  See [`orchestrator/cmd/control-plane/dispatcher.go`](../orchestrator/cmd/control-plane/dispatcher.go).

### 2.2 Node Requirements

- The Node Manager contacts the orchestrator on startup.
  It registers using the PSK and then periodically reports capabilities.
  See [`worker_node/cmd/node-manager/main.go`](../worker_node/cmd/node-manager/main.go).

- The Node Manager does not fetch node configuration from the orchestrator.
  This is required to satisfy Phase 1 "config delivery" and is also needed to deliver the Worker API bearer token as specified.
  See [`docs/tech_specs/node.md`](../docs/tech_specs/node.md) and [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md).

- The Node Manager does not manage an Ollama container lifecycle.
  The node spec defines a startup flow that starts the Worker API and then starts a single orchestrator-selected Ollama container when inference is configured.
  See [`docs/tech_specs/node.md`](../docs/tech_specs/node.md).

- The Worker API supports synchronous job execution and returns stdout, stderr, exit code, and timestamps.
  It runs commands via Podman or Docker using `run --rm` and captures stdout and stderr.
  See [`worker_node/cmd/worker-api/main.go`](../worker_node/cmd/worker-api/main.go) and [`worker_node/cmd/worker-api/executor/executor.go`](../worker_node/cmd/worker-api/executor/executor.go).

- The Worker API does not fully implement sandbox constraints expected by the specs.
  It always uses `--network=none` and does not apply a per-job network policy, per-task workspace mounts, or basic resource limits (CPU, memory, PIDs) when supported.
  See [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md) and [`docs/tech_specs/sandbox_container.md`](../docs/tech_specs/sandbox_container.md).

### 2.3 User API Gateway Requirements

- Local user authentication (login, refresh, logout) is implemented.
  The user gateway provides `POST /v1/auth/login`, `POST /v1/auth/refresh`, and `POST /v1/auth/logout`.
  See [`orchestrator/cmd/user-gateway/main.go`](../orchestrator/cmd/user-gateway/main.go) and [`docs/tech_specs/local_user_accounts.md`](../docs/tech_specs/local_user_accounts.md).

- Task creation and result retrieval are implemented.
  The user gateway provides `POST /v1/tasks` and `GET /v1/tasks/{id}/result`.
  See [`orchestrator/internal/handlers/tasks.go`](../orchestrator/internal/handlers/tasks.go) and [`orchestrator/cmd/user-gateway/main.go`](../orchestrator/cmd/user-gateway/main.go).

## 3 Phase 1 Work Plan (4-6 Hour Chunks)

This is the minimal, spec-driven work needed to finish MVP Phase 1, based on the Phase 1 bullets in [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).

Each chunk is designed to be a focused 4-6 hour block with clear outputs and validation.

### 3.1 Chunk 01 (4-6 Hours): Lock the Phase 1 Acceptance Criteria

This chunk makes Phase 1 "done" measurable and aligned to the authoritative specs.

#### 3.1.1 Chunk 01 Tasks

- Extract the explicit Phase 1 happy path requirements from [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md) into a short acceptance checklist inside this document.
- Define the Phase 1 inference precondition.
  Ensure Phase 1 requires at least one inference-capable execution path in the overall system (node-local inference container such as Ollama, or an external provider API key for inference routing).
  In the single-node case, define the fail-fast behavior when neither local inference nor external inference credentials are available.
- Decide and document whether Phase 1 inference requires node-local inference container lifecycle management, or whether Phase 1 can use external inference only (with sandbox execution still node-local).
- Decide the minimum required node config delivery semantics for Phase 1.
  At minimum, this should cover the Worker API bearer token delivery requirement in [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md).

#### 3.1.2 Chunk 01 Deliverables

- Updated `dev_docs/mvp_phase1_completion_plan.md` with an explicit acceptance checklist and resolved scope notes for the items in "Spec Gaps and Clarifications Needed."

#### 3.1.3 Chunk 01 Validation

- Run `just lint-md dev_docs/mvp_phase1_completion_plan.md`.

### 3.2 Chunk 02 (4-6 Hours): Make Bootstrap Payload Spec-Compliant Enough to Enable Config Delivery

This chunk aligns node registration bootstrap behavior with [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md) so a node can discover where to fetch its configuration.

#### 3.2.1 Chunk 02 Tasks

- Update the shared contracts in [`go_shared_libs/contracts/nodepayloads/nodepayloads.go`](../go_shared_libs/contracts/nodepayloads/nodepayloads.go) to include the missing bootstrap payload fields required by the spec.
- Update the control-plane node registration handler to emit the normative bootstrap shape, including orchestrator base URL and endpoint URLs for registration, capability reporting, and node config delivery.
  See [`orchestrator/internal/handlers/nodes.go`](../orchestrator/internal/handlers/nodes.go).
- Update the node manager registration client to parse the updated bootstrap payload.
  See [`worker_node/cmd/node-manager/main.go`](../worker_node/cmd/node-manager/main.go).

#### 3.2.2 Chunk 02 Deliverables

- Node registration returns a spec-aligned bootstrap payload shape.
- Node manager can register and parse the bootstrap payload shape without relying on hard-coded endpoint paths.

#### 3.2.3 Chunk 02 Validation

- Run `just test-go`.
- Run `just lint-go-ci`.

### 3.3 Chunk 03 (4-6 Hours): Implement Minimum Node Config Delivery API in the Control Plane

This chunk implements Phase 1 "config delivery" on the orchestrator side.

#### 3.3.1 Chunk 03 Tasks

- Define and implement a minimal node configuration retrieval endpoint on the control-plane.
  The endpoint must return the normative payload shape from [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md) `node_configuration_payload_v1`.
- Include at least the Worker API bearer token field in the node configuration payload so the node can authenticate Worker API requests as specified in [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md).
- Persist a per-node config version and emit it in the payload.
  Use a monotonic `config_version` string and store it in PostgreSQL.
- Add a minimal config acknowledgement endpoint and record node acknowledgements for visibility.
  Use the acknowledgement shape in [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md) `node_config_ack_v1`.

#### 3.3.2 Chunk 03 Deliverables

- Control-plane supports node configuration delivery and acknowledgement for registered nodes.
- The node configuration includes `worker_api.orchestrator_bearer_token` in a way that can replace env-based token distribution.

#### 3.3.3 Chunk 03 Validation

- Run `just test-go`.
- Run `just lint-go-ci`.

### 3.4 Chunk 04 (4-6 Hours): Update Node Manager to Fetch Config and Start Node Services in the Spec Order

This chunk implements the node startup sequence that fetches orchestrator config before starting node services.

#### 3.4.1 Chunk 04 Tasks

- Extend the node manager to fetch the node configuration payload after registration.
  Use the config endpoint URL provided by the bootstrap payload.
- Add a config polling loop consistent with the update guidance in [`docs/tech_specs/node.md`](../docs/tech_specs/node.md).
  For Phase 1, it is acceptable to fetch config on startup only, but the code should be structured to support polling.
- Start the Worker API service using the delivered bearer token instead of requiring manual env configuration.
  Confirm that the token is treated as a secret and is not logged.
- If Phase 1 requires Ollama lifecycle management, implement a minimal "start single Ollama container" path that is driven by configuration and runs after config fetch.
  If Phase 1 is sandbox-only, explicitly enforce inference disabled and do not start Ollama.

#### 3.4.2 Chunk 04 Deliverables

- Node manager performs: register => fetch config => start worker api => optional start ollama => report config ack.
- Worker API bearer token distribution is end-to-end via orchestrator config delivery.

#### 3.4.3 Chunk 04 Validation

- Run `just test-go`.
- Run `just lint-go-ci`.

### 3.5 Chunk 05 (4-6 Hours): Make Worker API Sandbox Execution Match Phase 1 Spec Constraints

This chunk ensures sandbox execution behavior matches the Phase 1 expectations in the Worker API and sandbox specs.

#### 3.5.1 Chunk 05 Tasks

- Implement `network_policy` support as defined in [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md).
  Map `none` => `--network=none` and `restricted` => a clearly documented restricted policy for Phase 1.
- Add basic safety limits when supported by the runtime.
  Implement reasonable defaults for CPU and memory limits, and PIDs limits where supported.
  See [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md).
- Mount a per-task workspace directory into the container and set an explicit working directory (for example `/workspace`).
  See [`docs/tech_specs/sandbox_container.md`](../docs/tech_specs/sandbox_container.md).
- Ensure environment variables include non-secret task context variables (task id, job id, workspace dir) and do not include orchestrator secrets.
  See [`docs/tech_specs/sandbox_container.md`](../docs/tech_specs/sandbox_container.md).

#### 3.5.2 Chunk 05 Deliverables

- Worker API executes sandbox jobs with the expected baseline constraints and predictable working directory behavior.

#### 3.5.3 Chunk 05 Validation

- Run `just test-go`.
- Run `just lint-go-ci`.

### 3.6 Chunk 06 (4-6 Hours): Make Orchestrator Dispatch Node-Aware (Single Node, Spec-Shaped)

This chunk keeps the Phase 1 dispatcher simple while aligning it to the node config delivery model.

#### 3.6.1 Chunk 06 Tasks

- Stop using a single global Worker API URL from env for dispatch.
  Instead, use the per-node `worker_api_target_url` delivered in the node config payload shape and stored per node.
  See [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md).
- Stop using a single global Worker API bearer token from env for dispatch.
  Use the per-node bearer token as stored via config delivery.
- Ensure dispatch only targets nodes that are active and have acknowledged a config version that contains Worker API connectivity details.

#### 3.6.2 Chunk 06 Deliverables

- The dispatcher uses per-node endpoint and per-node bearer token derived from the config delivery flow.

#### 3.6.3 Chunk 06 Validation

- Run `just test-go`.
- Run `just lint-go-ci`.

### 3.7 Chunk 07 (4-6 Hours): End-to-End Phase 1 Demo Hardening

This chunk ensures the Phase 1 happy path works from scratch, using only `just` targets and without manual wiring.

#### 3.7.1 Chunk 07 Tasks

- Update the existing happy path test flow so it does not rely on manual environment variables for worker api token distribution.
  The Node Manager should be able to boot, fetch config, start worker api, and accept dispatch with no extra manual steps beyond the orchestrator bootstrap variables.
- Run the Phase 1 BDD feature scenarios and ensure they pass.
  See [`features/single_node_happy_path.feature`](../features/single_node_happy_path.feature).
- Ensure the documentation in [`dev_docs/PHASE1_STATUS.md`](./PHASE1_STATUS.md) is consistent with the actual Phase 1 wiring.
  Do not update tech specs without explicit direction.

#### 3.7.2 Chunk 07 Deliverables

- A clean local run path that passes the Phase 1 happy path using project tooling.

#### 3.7.3 Chunk 07 Validation

- Run `just ci`.
- Run `just e2e`.

## 4 Phase 1 Definition of Done

Phase 1 is complete when all of the following are true.

### 4.1 Startup Preconditions for Inference

Phase 1 requires at least one inference-capable execution path in the overall system.

- Inference MAY be provided by a node-local inference container such as Ollama.
- Inference MAY be provided via external model routing when an external AI API key is configured and policy allows it.
- In the single-node case, the system must fail fast (or refuse to enter a ready state) if the node cannot run an inference container and there is no external AI API key configured.

- The Phase 1 bullets in [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md) are met without relying on undocumented manual steps.
- Node registration, capability ingest, config delivery, job dispatch, and result collection behave in a way that is compliant with:
  [`docs/tech_specs/node.md`](../docs/tech_specs/node.md),
  [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md),
  and [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md).
- User authentication and the task endpoints required for the Phase 1 happy path are implemented and behave as specified in:
  [`docs/tech_specs/local_user_accounts.md`](../docs/tech_specs/local_user_accounts.md)
  and [`docs/tech_specs/user_api_gateway.md`](../docs/tech_specs/user_api_gateway.md).
- `just ci` passes.
- The Phase 1 end-to-end happy path passes under `just e2e`.

## 5 References

Specs

- [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md)
- [`docs/tech_specs/orchestrator.md`](../docs/tech_specs/orchestrator.md)
- [`docs/tech_specs/node.md`](../docs/tech_specs/node.md)
- [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md)
- [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md)
- [`docs/tech_specs/sandbox_container.md`](../docs/tech_specs/sandbox_container.md)
- [`docs/tech_specs/user_api_gateway.md`](../docs/tech_specs/user_api_gateway.md)
- [`docs/tech_specs/local_user_accounts.md`](../docs/tech_specs/local_user_accounts.md)

Implementation entry points (for Phase 1 wiring)

- [`orchestrator/cmd/control-plane/main.go`](../orchestrator/cmd/control-plane/main.go)
- [`orchestrator/cmd/control-plane/dispatcher.go`](../orchestrator/cmd/control-plane/dispatcher.go)
- [`orchestrator/cmd/user-gateway/main.go`](../orchestrator/cmd/user-gateway/main.go)
- [`worker_node/cmd/node-manager/main.go`](../worker_node/cmd/node-manager/main.go)
- [`worker_node/cmd/worker-api/main.go`](../worker_node/cmd/worker-api/main.go)
- [`worker_node/cmd/worker-api/executor/executor.go`](../worker_node/cmd/worker-api/executor/executor.go)
- [`features/single_node_happy_path.feature`](../features/single_node_happy_path.feature)
