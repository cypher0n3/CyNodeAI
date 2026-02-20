# MVP Phase 1 Completion Plan

- [1 Current Status](#1-current-status)
- [2 Phase 1 Scope and Decisions](#2-phase-1-scope-and-decisions)
- [3 Current State Versus Phase 1 Requirements](#3-current-state-versus-phase-1-requirements)
  - [3.1 Orchestrator Requirements](#31-orchestrator-requirements)
  - [3.2 Node Requirements](#32-node-requirements)
  - [3.3 User API Gateway Requirements](#33-user-api-gateway-requirements)
- [4 Phase 1 Work Plan (4-6 Hour Chunks)](#4-phase-1-work-plan-4-6-hour-chunks)
  - [4.1 Chunk 01 (4-6 Hours): Lock the Phase 1 Acceptance Criteria](#41-chunk-01-4-6-hours-lock-the-phase-1-acceptance-criteria)
  - [4.2 Chunk 02 (4-6 Hours): Make Bootstrap Payload Spec-Compliant Enough to Enable Config Delivery](#42-chunk-02-4-6-hours-make-bootstrap-payload-spec-compliant-enough-to-enable-config-delivery)
  - [4.3 Chunk 03 (4-6 Hours): Implement Minimum Node Config Delivery API in the Control Plane](#43-chunk-03-4-6-hours-implement-minimum-node-config-delivery-api-in-the-control-plane)
  - [4.4 Chunk 04 (4-6 Hours): Update Node Manager to Fetch Config and Start Node Services in the Spec Order](#44-chunk-04-4-6-hours-update-node-manager-to-fetch-config-and-start-node-services-in-the-spec-order)
  - [4.5 Chunk 05 (4-6 Hours): Make Worker API Sandbox Execution Match Phase 1 Spec Constraints](#45-chunk-05-4-6-hours-make-worker-api-sandbox-execution-match-phase-1-spec-constraints)
  - [4.6 Chunk 06 (4-6 Hours): Make Orchestrator Dispatch Node-Aware (Single Node, Spec-Shaped)](#46-chunk-06-4-6-hours-make-orchestrator-dispatch-node-aware-single-node-spec-shaped)
  - [4.7 Chunk 07 (4-6 Hours): End-to-End Phase 1 Demo Hardening](#47-chunk-07-4-6-hours-end-to-end-phase-1-demo-hardening)
- [5 Phase 1 Definition of Done](#5-phase-1-definition-of-done)
  - [5.1 Startup Preconditions for Inference](#51-startup-preconditions-for-inference)
- [6 MVP Requirement Scope (Phase 1 vs Deferred)](#6-mvp-requirement-scope-phase-1-vs-deferred)
- [7 References](#7-references)

## 1 Current Status

As of 2026-02-19, gap closure status is documented in [`dev_docs/mvp_specs_gaps_closure_status.md`](./mvp_specs_gaps_closure_status.md).
The blocking schema gap is closed; Phase 1 dispatch (direct HTTP to Worker API, no MCP gateway) and BDD scenarios are closed in permanent specs and suite-scoped feature files under [`features/`](../features/).

Implementation progress versus this plan (branch `mvp/phase-1`, checked 2026-02-19):

- Chunk 01: complete (acceptance checklist and Phase 1 inference precondition / fail-fast behavior).
- Chunk 02: complete (bootstrap payload: `orchestrator.base_url`, `orchestrator.endpoints`, `auth.node_jwt`).
- Chunk 03: complete (node config delivery API: GET/POST `node_config_url`, config ack storage).
- Remaining: Chunks 04-07.

This document includes an explicit **MVP requirement scope** (Section 6) mapping requirement IDs to Phase 1 in-scope vs deferred.

## 2 Phase 1 Scope and Decisions

Phase 1 scope and decisions (including config refresh on startup only and long-lived node JWT with re-register on expiry) are defined in the [Phase 1 Single Node Happy Path](../docs/tech_specs/_main.md#phase-1-single-node-happy-path) section of [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).
All other Phase 1 behavior is specified in [`docs/tech_specs/`](../docs/tech_specs/) and [`docs/requirements/`](../docs/requirements/).
The work chunks in [Section 4](#4-phase-1-work-plan-4-6-hour-chunks) are build-only against those specs (see [Section 6](#6-mvp-requirement-scope-phase-1-vs-deferred) for requirement coverage).

## 3 Current State Versus Phase 1 Requirements

This section cross-checks the status report in [`dev_docs/PHASE1_STATUS.md`](./PHASE1_STATUS.md) against the code currently in the repo and the Phase 1 bullets in [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).

### 3.1 Orchestrator Requirements

- Node registration (PSK => JWT) is implemented.
  The control-plane exposes `POST /v1/nodes/register` and issues a node JWT.
  See [`orchestrator/cmd/control-plane/main.go`](../orchestrator/cmd/control-plane/main.go) and [`orchestrator/internal/handlers/nodes.go`](../orchestrator/internal/handlers/nodes.go).

- Capability ingest is implemented, but the payload schema is incomplete versus the normative spec.
  The control-plane stores capability snapshots and updates a capability hash.
  See [`orchestrator/internal/handlers/nodes.go`](../orchestrator/internal/handlers/nodes.go) and [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md).

- Configuration delivery is implemented (orchestrator side).
  The control-plane exposes:
  - `GET /v1/nodes/config`: returns `node_configuration_payload_v1`.
  - `POST /v1/nodes/config`: accepts `node_config_ack_v1` and records config acknowledgement on the node.
  The node bootstrap response includes `orchestrator.base_url` and `orchestrator.endpoints` (including `node_config_url`).
  See [`orchestrator/cmd/control-plane/main.go`](../orchestrator/cmd/control-plane/main.go), [`orchestrator/internal/handlers/nodes.go`](../orchestrator/internal/handlers/nodes.go), and [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md).

- Job dispatch is implemented, but it is single-node and not node-address-aware.
  The dispatcher runs inside the control-plane process and calls a single Worker API URL from environment configuration.
  It does not use per-node worker endpoints from a node configuration payload.
  See [`orchestrator/cmd/control-plane/dispatcher.go`](../orchestrator/cmd/control-plane/dispatcher.go) and [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md).

- Result collection is implemented.
  The dispatcher stores the Worker API response JSON on the job, updates job and task statuses, and stores a task summary.
  See [`orchestrator/cmd/control-plane/dispatcher.go`](../orchestrator/cmd/control-plane/dispatcher.go).

### 3.2 Node Requirements

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

### 3.3 User API Gateway Requirements

- Local user authentication (login, refresh, logout) is implemented.
  The user gateway provides `POST /v1/auth/login`, `POST /v1/auth/refresh`, and `POST /v1/auth/logout`.
  See [`orchestrator/cmd/user-gateway/main.go`](../orchestrator/cmd/user-gateway/main.go) and [`docs/tech_specs/local_user_accounts.md`](../docs/tech_specs/local_user_accounts.md).

- Task creation and result retrieval are implemented.
  The user gateway provides `POST /v1/tasks` and `GET /v1/tasks/{id}/result`.
  See [`orchestrator/internal/handlers/tasks.go`](../orchestrator/internal/handlers/tasks.go) and [`orchestrator/cmd/user-gateway/main.go`](../orchestrator/cmd/user-gateway/main.go).

## 4 Phase 1 Work Plan (4-6 Hour Chunks)

This is the minimal, spec-driven work needed to finish MVP Phase 1, based on the Phase 1 bullets in [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).

Each chunk is designed to be a focused 4-6 hour block with clear outputs and validation.

### 4.1 Chunk 01 (4-6 Hours): Lock the Phase 1 Acceptance Criteria

This chunk makes Phase 1 "done" measurable and aligned to the authoritative specs.

#### 4.1.0 Phase 1 Acceptance Checklist (Chunk 01 Deliverable)

Derived from [Phase 1 Single Node Happy Path](../docs/tech_specs/_main.md#phase-1-single-node-happy-path) in [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).

- [ ] Orchestrator: node registration (PSK to JWT), capability ingest, config delivery, job dispatch, result collection.
- [ ] Node: Node Manager startup sequence contacts orchestrator before starting the single Ollama container.
- [ ] Node: Worker API receives a job, runs a sandbox container, returns a result.
- [ ] System: at least one inference-capable path available (MVP Phase 1: node-local Ollama only; see inference precondition below).
- [ ] System: single-node fail-fast when inference is unavailable (see below).
- [ ] User API Gateway: local user auth (login and refresh), create task, retrieve task result.

##### 4.1.0.1 Phase 1 Inference Precondition (Mvp Phase 1)

The system must have at least one inference-capable execution path.
For MVP Phase 1, inference is provided solely by a node-local Ollama container started and managed by the Node Manager.
External model routing is out of scope.

##### 4.1.0.2 Fail-Fast (Single-Node)

If the node cannot run the inference container (e.g. Ollama fails to start), the system must fail fast or refuse to enter a ready state.
The Node Manager must not report the node as ready for job dispatch until the Ollama container is running; otherwise it must exit with an error or remain in a non-ready state and report the failure.

#### 4.1.1 Chunk 01 Tasks

- [x] Extract the explicit Phase 1 happy path requirements from [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md) into a short acceptance checklist inside this document (see [Section 4.1.0](#410-phase-1-acceptance-checklist-chunk-01-deliverable)).
- [x] Define the Phase 1 inference precondition and single-node fail-fast behavior per [Section 2](#2-phase-1-scope-and-decisions) (see [Section 4.1.0](#410-phase-1-acceptance-checklist-chunk-01-deliverable)).

#### 4.1.2 Chunk 01 Deliverables

- [x] Updated `dev_docs/mvp_phase1_completion_plan.md` with an explicit acceptance checklist and inference precondition/fail-fast (Section 4.1.0).
- Spec gaps in Section 2 are resolved via Section 2.1 and the config delivery / Worker API auth decisions recorded earlier in this document.

#### 4.1.3 Chunk 01 Validation

- Run `just lint-md dev_docs/mvp_phase1_completion_plan.md`.

### 4.2 Chunk 02 (4-6 Hours): Make Bootstrap Payload Spec-Compliant Enough to Enable Config Delivery

This chunk aligns node registration bootstrap behavior with [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md) so a node can discover where to fetch its configuration.

Status: complete (2026-02-19).

#### 4.2.1 Phase 1 Minimal Bootstrap Subset (Definition)

For Phase 1, we do not need full bootstrap payload completeness.
We do need a stable, spec-shaped bootstrap that lets the node discover the correct orchestrator URLs and authenticate subsequent requests without hard-coded paths.

The Phase 1 minimal subset is:

- Orchestrator emits, in its bootstrap response (spec-shaped):
  - `version`, `issued_at`.
  - `orchestrator.base_url` and `orchestrator.endpoints`:
    - `worker_registration_url`
    - `node_report_url`
    - `node_config_url` (used for config retrieval and config ack; see Chunk 03).
  - `auth.node_jwt` and `auth.expires_at`.
- Node consumes the bootstrap response by:
  - Persisting the node JWT and using it for subsequent orchestrator calls.
  - Using the returned endpoint URLs (not hard-coded paths) for capability reporting and config retrieval.

Anything beyond this (for example trust bundles and an initial config version hint) is optional for Phase 1 and can be added later without breaking wire compatibility.

#### 4.2.2 Chunk 02 Tasks

- Update the shared contracts in [`go_shared_libs/contracts/nodepayloads/nodepayloads.go`](../go_shared_libs/contracts/nodepayloads/nodepayloads.go) to include the Phase 1 minimal bootstrap subset fields.
- Update the control-plane node registration handler to emit the normative bootstrap shape, including orchestrator base URL and endpoint URLs for registration, capability reporting, and node config delivery.
  See [`orchestrator/internal/handlers/nodes.go`](../orchestrator/internal/handlers/nodes.go).
- Update the node manager registration client to parse the updated bootstrap payload.
  See [`worker_node/cmd/node-manager/main.go`](../worker_node/cmd/node-manager/main.go).

#### 4.2.3 Chunk 02 Deliverables

- Node registration returns a spec-aligned bootstrap payload shape.
- Node manager can register and parse the bootstrap payload shape and uses returned endpoint URLs for follow-on calls (capability reporting today; config retrieval in Chunk 04).

#### 4.2.4 Chunk 02 Validation

- Run `just test-go`.
- Run `just lint-go-ci`.

### 4.3 Chunk 03 (4-6 Hours): Implement Minimum Node Config Delivery API in the Control Plane

This chunk implements Phase 1 "config delivery" on the orchestrator side.

Status: complete (2026-02-19).

#### 4.3.1 MVP Phase 1 Decisions (Config Delivery and Transport)

- Worker API auth token semantics:
  - Use a static, unchanging bearer token for MVP Phase 1.
  - Defer expiring tokens and refresh until a later phase.
- Transport:
  - Use HTTP for component-to-component traffic for MVP Phase 1 (internal network only).
  - Defer HTTPS, mTLS, and related hardening until a later phase.

#### 4.3.2 Chunk 03 Tasks

- Implement a minimal node configuration retrieval endpoint on the control-plane.
  The endpoint must return the normative payload shape from [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md) `node_configuration_payload_v1`.
- Implement the concrete MVP Phase 1 endpoint paths and methods for `node_config_url` and `node_report_url` and ensure they are emitted in `node_bootstrap_payload_v1`.
- Include at least the Worker API bearer token field in the node configuration payload so the node can authenticate Worker API requests as specified in [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md).
- Persist a per-node config version and emit it in the payload.
  Use a monotonic `config_version` string and store it in PostgreSQL.
- Implement config acknowledgement using the same `node_config_url` (preferred for MVP Phase 1):
  - `POST node_config_url`: accept `node_config_ack_v1` and record node acknowledgements for visibility.

#### 4.3.3 Chunk 03 Deliverables

- Control-plane supports node configuration delivery and acknowledgement for registered nodes.
- The node configuration includes `worker_api.orchestrator_bearer_token` in a way that can replace env-based token distribution.

#### 4.3.4 Chunk 03 Validation

- Run `just test-go`.
- Run `just lint-go-ci`.

### 4.4 Chunk 04 (4-6 Hours): Update Node Manager to Fetch Config and Start Node Services in the Spec Order

This chunk implements the node startup sequence that fetches orchestrator config before starting node services.

#### 4.4.1 Chunk 04 Tasks

- Extend the node manager to fetch the node configuration payload after registration.
  Use the config endpoint URL provided by the bootstrap payload.
- Fetch node configuration on startup only for MVP Phase 1 (no polling).
- Start the Worker API service using the delivered bearer token instead of requiring manual env configuration.
  Confirm that the token is treated as a secret and is not logged.
- Implement the Phase 1 inference startup behavior as locked in [Section 2](#2-phase-1-scope-and-decisions).
- Add or update BDD coverage for the node manager config fetch + startup sequence:
  - Create or update `features/worker_node/` feature coverage to assert:
    - The node manager fetches config via `node_config_url` (from bootstrap payload, not hard-coded paths).
    - The node manager uses `worker_api.orchestrator_bearer_token` from config (no manual env token wiring).
    - The node manager submits config acknowledgement (`node_config_ack_v1`) after applying config (Phase 1: startup-only).
    - Fail-fast / non-ready behavior is exercised for inference startup failures (per the locked Phase 1 precondition).
  - Implement step definitions for the above in the worker-node BDD suite.

#### 4.4.2 Chunk 04 Deliverables

- Node manager performs: register => fetch config => start worker api => start ollama => report config ack.
- Worker API bearer token distribution is end-to-end via orchestrator config delivery.
- Feature coverage exists/updated under `features/worker_node/` and the worker-node BDD suite executes the config fetch/startup path (including a config ack).

#### 4.4.3 Chunk 04 Validation

- Run `just validate-feature-files`.
- Run `just test-go`.
- Run `POSTGRES_TEST_DSN="postgres://..." just test-bdd` (orchestrator config ack steps require a DB).
- Run `just lint-go-ci`.

### 4.5 Chunk 05 (4-6 Hours): Make Worker API Sandbox Execution Match Phase 1 Spec Constraints

This chunk ensures sandbox execution behavior matches the Phase 1 expectations in the Worker API and sandbox specs.

#### 4.5.1 Chunk 05 Tasks

- Implement `network_policy` support as defined in [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md).
  Map `none` => `--network=none`.
  Map `restricted` according to the locked decision in [Section 2](#2-phase-1-scope-and-decisions).
- Do not implement CPU, memory, or PIDs limits for MVP Phase 1 (defer to a later phase).
- Mount a per-task workspace directory into the container and set an explicit working directory (for example `/workspace`).
  Implement the locked workspace layout from [Section 2](#2-phase-1-scope-and-decisions).
  See [`docs/tech_specs/sandbox_container.md`](../docs/tech_specs/sandbox_container.md).
- Ensure environment variables include non-secret task context variables (task id, job id, workspace dir) and do not include orchestrator secrets.
  See [`docs/tech_specs/sandbox_container.md`](../docs/tech_specs/sandbox_container.md).
- Add or update worker-node BDD coverage for Phase 1 sandbox constraints:
  - Update `features/worker_node/worker_node_sandbox_execution.feature` to include scenarios that assert:
    - `network_policy` behavior matches Phase 1 decisions (`restricted` treated as deny-all; `none` is deny-all).
    - The container runs with a deterministic working directory (`/workspace`) and a per-task workspace mount.
    - Sandbox environment contains task/job/workspace context and does not include orchestrator secrets.
  - Implement/update step definitions to validate the above (including inspecting the returned run result / metadata as needed).

#### 4.5.2 Chunk 05 Deliverables

- Worker API executes sandbox jobs with the expected baseline constraints and predictable working directory behavior.
- Feature coverage exists/updated under `features/worker_node/` for sandbox constraints, and step definitions are implemented (no `godog.ErrSkip`).

#### 4.5.3 Chunk 05 Validation

- Run `just validate-feature-files`.
- Run `just test-go`.
- Run `POSTGRES_TEST_DSN="postgres://..." just test-bdd` (runs the worker-node suite and orchestrator suite; orchestrator scenarios require DB for DB-backed steps).
- Run `just lint-go-ci`.
- Run `just ci`.
  Fix any issues, re-run, repeat.

### 4.6 Chunk 06 (4-6 Hours): Make Orchestrator Dispatch Node-Aware (Single Node, Spec-Shaped)

This chunk keeps the Phase 1 dispatcher simple while aligning it to the node config delivery model.

#### 4.6.1 Chunk 06 Tasks

- Stop using a single global Worker API URL from env for dispatch.
  Instead, use the per-node `worker_api_target_url` delivered in the node config payload shape and stored per node.
  See [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md).
- Stop using a single global Worker API bearer token from env for dispatch.
  Use the per-node bearer token as stored via config delivery.
- Ensure dispatch only targets nodes that are active and have acknowledged a config version that contains Worker API connectivity details.
- Add or update orchestrator BDD coverage for node-aware dispatch:
  - Update `features/orchestrator/orchestrator_task_lifecycle.feature` to assert:
    - The dispatcher uses per-node worker target URL and bearer token derived from config delivery (not global env wiring).
    - Dispatch selects only nodes that are active and have acknowledged an applicable config version.
  - Implement/update step definitions so these scenarios execute end-to-end against the control-plane test server.

#### 4.6.2 Chunk 06 Deliverables

- The dispatcher uses per-node endpoint and per-node bearer token derived from the config delivery flow.
- Feature coverage exists/updated under `features/orchestrator/` for node-aware dispatch, and step definitions are implemented (no `godog.ErrSkip`).

#### 4.6.3 Chunk 06 Validation

- Run `just validate-feature-files`.
- Run `just test-go`.
- Run `POSTGRES_TEST_DSN="postgres://..." just test-bdd`.
- Run `just lint-go-ci`.

### 4.7 Chunk 07 (4-6 Hours): End-to-End Phase 1 Demo Hardening

This chunk ensures the Phase 1 happy path works from scratch, using only `just` targets and without manual wiring.

#### 4.7.1 Chunk 07 Tasks

- Update the existing happy path test flow so it does not rely on manual environment variables for worker api token distribution.
  The Node Manager should be able to boot, fetch config, start worker api, and accept dispatch with no extra manual steps beyond the orchestrator bootstrap variables.
- Flesh out and run Phase 1 BDD feature scenarios (no skips) to cover the remaining chunk behaviors:
  - Ensure suite-scoped features under `features/orchestrator/` and `features/worker_node/` include coverage for Chunks 04-06.
  - Ensure the end-to-end feature `features/e2e/single_node_happy_path.feature` does not depend on manual Worker API token wiring (token delivered via config).
  - Implement any missing step definitions so `just test-bdd` is fully meaningful for Phase 1 (no `godog.ErrSkip` in Phase 1 paths).
- Ensure the documentation in [`dev_docs/PHASE1_STATUS.md`](./PHASE1_STATUS.md) is consistent with the actual Phase 1 wiring.
  Do not update tech specs without explicit direction.

#### 4.7.2 Chunk 07 Deliverables

- A clean local run path that passes the Phase 1 happy path using project tooling.
- All Phase 1 feature files are present/updated and step definitions implemented for the remaining chunks; Phase 1 BDD suites execute without skips for Phase 1 scenarios.

#### 4.7.3 Chunk 07 Validation

- Run `just validate-feature-files`.
- Run `just ci`.
- Run `just e2e`.

## 5 Phase 1 Definition of Done

Phase 1 is complete when all of the following are true.

### 5.1 Startup Preconditions for Inference

Phase 1 requires at least one inference-capable execution path in the overall system.

- For MVP Phase 1, inference is provided by node-local inference (a single Ollama container managed by the Node Manager).
- External model routing is out of scope for MVP Phase 1.
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

## 6 MVP Requirement Scope (Phase 1 vs Deferred)

This section maps requirement domains and selected IDs to Phase 1 in-scope vs deferred, addressing the optional gap in [`dev_docs/mvp_specs_gaps_closure_status.md`](./mvp_specs_gaps_closure_status.md) Section 2.4.
Canonical requirement text lives in [`docs/requirements/`](../docs/requirements/); this table is a scope index only.

| Domain | Phase 1 in scope | Deferred (later phases) |
|--------|------------------|--------------------------|
| **ORCHES** | 0001 (subset: config delivery), 0100, 0112, 0113, 0120, 0121, 0122, 0123 | 0101-0111 (cron, external AI, API Egress), 0108, 0114-0115 |
| **BOOTST** | 0002 (fail-fast), 0100, 0101 | 0102 (full MCP/gateway read) |
| **WORKER** | 0001, 0002, 0100-0111, 0114-0115 (Ollama), 0120-0122, 0127, 0135 (config ack), 0136-0138 | 0112-0113 (streaming, task/job association beyond baseline), 0116-0119 (node MCP server), 0123-0134 (sandbox-only, credential rotation) |
| **USRGWY** | 0001 (subset), 0122, 0124, 0125; minimal task create/result | 0100-0121 (runs, sessions, retention, streaming), 0123, 0126 |
| **IDENTY** | 0001 (subset), 0100-0106, 0109-0112, 0113 (bootstrap admin) | 0107-0108 (full RBAC/audit), 0114+ |
| **SANDBX** | 0001 (subset), 0100-0109 (no creds in sandbox, workspace, exec, logs, Ollama private) | Remaining SANDBX |
| **SCHEMA** | As applied to Phase 1 services (tasks, jobs, nodes, users, auth) | Full schema scope |
| **STANDS** | As applied to Phase 1 Go REST APIs | Full standards scope |
| **MCPGAT, MCPTOO, APIEGR, MODELS, AGENTS, etc.** | - | All deferred to Phase 2+ |

Phase 1 does not use the MCP gateway for job dispatch (direct HTTP to Worker API; see [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md) Phase 1 bullets).

## 7 References

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
- [`features/e2e/single_node_happy_path.feature`](../features/e2e/single_node_happy_path.feature)
- [`features/orchestrator/node_registration_and_config.feature`](../features/orchestrator/node_registration_and_config.feature)
- [`features/orchestrator/orchestrator_task_lifecycle.feature`](../features/orchestrator/orchestrator_task_lifecycle.feature)
- [`features/worker_node/worker_node_sandbox_execution.feature`](../features/worker_node/worker_node_sandbox_execution.feature)
