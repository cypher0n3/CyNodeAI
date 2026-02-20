# MVP Phase 1 Code Review Report

- [1 Executive Summary](#1-executive-summary)
- [2 Implementation vs Technical Specs](#2-implementation-vs-technical-specs)
  - [2.1 Orchestrator compliance](#21-orchestrator-compliance)
  - [2.2 Node Manager compliance](#22-node-manager-compliance)
  - [2.3 Worker API and sandbox](#23-worker-api-and-sandbox)
  - [2.4 User API gateway](#24-user-api-gateway)
- [3 Feature File Gaps (MVP Phase 1)](#3-feature-file-gaps-mvp-phase-1)
- [4 BDD Test Gaps](#4-bdd-test-gaps)
- [5 Recommendations](#5-recommendations)
- [6 References](#6-references)

## 1 Executive Summary

This report compares the current implementation (branch `mvp/phase-1`) against the Phase 1 technical specs and the MVP Phase 1 completion plan.
It identifies implementation gaps, spec deviations, and missing or incomplete feature/BDD coverage.

### 1.1 Key Findings

- **Orchestrator:** Node-aware dispatch and config delivery are implemented; `config_version` does not use ULID as required by `node_payloads.md`.
  Capability ingest stores a snapshot but the capability report schema in code is a minimal subset of the spec.
- **Node Manager:** Config fetch, startup order (register -> fetch config -> start Worker API -> start Ollama -> config ack), and config ack are implemented.
  Ollama image is taken from env (`OLLAMA_IMAGE`) rather than from orchestrator-delivered config; node startup YAML is not implemented.
- **Worker API / Sandbox:** Network policy (`none`/`restricted`), per-task workspace mount at `/workspace`, and task context env (no secrets) are implemented.
  Request size and stdout/stderr limits exist; default timeout derivation from node config/YAML is only partially applied.
- **Feature files:** Several Phase 1 scenarios are present; orchestrator fail-fast scenario scope is ambiguous (spec places fail-fast on the node, not the orchestrator).
  E2E and worker_node features are largely aligned; a few scenarios lack explicit tags or could be tightened.
- **BDD:** Many orchestrator steps return `godog.ErrSkip` when `POSTGRES_TEST_DSN` is unset or state is missing; this is intentional for DB-backed scenarios but means Phase 1 paths are not fully exercised without a DB.
  Worker-node BDD uses a mock orchestrator and "direct" executor; sandbox scenarios run but without a real container for some steps.

## 2 Implementation vs Technical Specs

The following tables compare implementation to the Phase 1 technical specs.

### 2.1 Orchestrator Compliance

| Spec / Requirement | Status | Notes |
|--------------------|--------|--------|
| Node registration (PSK to JWT), bootstrap payload | Done | Bootstrap includes `orchestrator.base_url`, `orchestrator.endpoints` (node_config_url, node_report_url, worker_registration_url), `auth.node_jwt`, `auth.expires_at`. |
| Config delivery GET/POST, config ack storage | Done | GET/POST `/v1/nodes/config`; payload shape matches `node_configuration_payload_v1`; Worker API URL and token persisted on config serve; config ack recorded. |
| `config_version` monotonic, ULID | **Gap** | `node_payloads.md` requires: "For version=1, the orchestrator MUST use a ULID encoded as a 26-character Crockford Base32 string." Implementation uses literal `"1"` or existing `node.ConfigVersion` (see `orchestrator/internal/handlers/nodes.go`). No ULID generation. |
| ListDispatchableNodes, per-node URL/token | Done | Dispatcher uses `ListDispatchableNodes`; only nodes with `config_ack_status = applied` and non-empty `worker_api_target_url` and `worker_api_bearer_token` are used. |
| Capability ingest, capability_hash | Partial | Handler stores capability snapshot and updates `capability_hash` on the node. The shared contract `nodepayloads.CapabilityReport` is a minimal subset of `node_capability_report_v1` (e.g. no `container_runtime`, `gpu`, `network`, `inference`, `tls`, or `capability_hash` in the struct). Hash is not validated or computed server-side per spec algorithm. |

### 2.2 Node Manager Compliance

| Spec / Requirement | Status | Notes |
|--------------------|--------|--------|
| Startup order: register -> fetch config -> start Worker API -> start Ollama -> config ack | Done | `nodemanager.RunWithOptions` implements this order. |
| Fetch config from bootstrap `node_config_url` | Done | `FetchConfig` uses `bootstrap.NodeConfigURL` with node JWT. |
| Worker API started with config-delivered bearer token | Done | Token from `nodeConfig.WorkerAPI.OrchestratorBearerToken`; not logged. |
| Config ack after applying config | Done | `SendConfigAck` with status "applied" after services started. |
| Fail-fast when inference (Ollama) startup fails | Done | `StartOllama` error causes `RunWithOptions` to return an error. |
| Ollama container image from orchestrator | **Gap** | `node.md` and Phase 1 flow: "Start the single Ollama container **specified by the orchestrator**." Implementation uses env `OLLAMA_IMAGE` (default `ollama/ollama`) in `node-manager/main.go`. Config payload does not currently carry an Ollama image reference; image selection by orchestrator is not implemented. |
| Node startup YAML | **Deferred** | Spec defines node startup YAML (e.g. `/etc/cynode/node.yaml`) and many options. Node Manager uses env only; no YAML loading. Completion plan and Phase 1 scope do not require full YAML for MVP. |

### 2.3 Worker API and Sandbox

| Spec / Requirement | Status | Notes |
|--------------------|--------|--------|
| `POST /v1/worker/jobs:run`, auth bearer token | Done | Token required; delivered via config. |
| `network_policy` none/restricted => deny-all | Done | Executor maps both to `--network=none`. |
| Per-task workspace at `/workspace`, task context env | Done | `prepareWorkspace` per job; mount at `/workspace`; `CYNODE_TASK_ID`, `CYNODE_JOB_ID`, `CYNODE_WORKSPACE_DIR` set; request env cannot override `CYNODE_*`. |
| No orchestrator secrets in sandbox | Done | Only task context and request `env` (with CYNODE_* override blocked). |
| Request size limit (e.g. 10 MiB) | Done | `http.MaxBytesReader(w, r.Body, 10*1024*1024)` in handler. |
| Stdout/stderr capture and truncation | Done | Executor truncates to `maxOutputBytes`; `truncated.stdout`/`truncated.stderr` in response. |
| Timeout derivation from node config/YAML | Partial | Worker API uses `DEFAULT_TIMEOUT_SECONDS` env and request `sandbox.timeout_seconds`. Spec says node default/max from node startup YAML or config `constraints.max_job_timeout_seconds`; node does not yet pass these from config to Worker API. |
| Health checks `GET /healthz`, `GET /readyz` | Partial | `GET /healthz` implemented; `GET /readyz` not present (spec requires both). |

### 2.4 User API Gateway

| Spec / Requirement | Status | Notes |
|--------------------|--------|--------|
| Local user auth (login, refresh, logout) | Done | Implemented per `local_user_accounts.md` / user-gateway. |
| Create task, retrieve task result | Done | `POST /v1/tasks`, `GET /v1/tasks/{id}/result`. |

## 3 Feature File Gaps (MVP Phase 1)

- **E2E (`features/e2e/single_node_happy_path.feature`):** Covers login, node register, node requests config, node applies config and sends config ack, create task, dispatch, execute job, result stdout, task completed.
  No scenario for "node fails to start inference and exits (fail-fast)" on the E2E path.
- **Orchestrator (`features/orchestrator/orchestrator_startup.feature`):** Single scenario: "Orchestrator fails fast when no inference path is available."
  The Phase 1 spec and completion plan define fail-fast on the **node** (Node Manager must not report ready until Ollama is running, or exit with error).
  The orchestrator itself does not currently check for an "inference path" or refuse a "ready" state.
  This scenario is either out of scope for the orchestrator or implies an unimplemented orchestrator-side readiness check; the feature file does not clarify.
- **Orchestrator task lifecycle:** Dispatcher scenario "Dispatcher uses per-node worker URL and token" is present and tagged; Background assumes "node has worker_api_target_url and bearer token in config," which depends on config delivery and config ack.
  No explicit scenario for "dispatch only targets nodes that have acknowledged config."
- **Worker node (`features/worker_node/node_manager_config_startup.feature`):** Covers config fetch via bootstrap URL, config ack after apply, and fail-fast when inference startup fails.
  Steps are implemented via mock orchestrator.
- **Worker node (`features/worker_node/worker_node_sandbox_execution.feature`):** Covers auth, run job with stdout/exit code, network_policy none/restricted, workspace and task context env, and no CYNODE_ override.
  No scenario that asserts stdout/stderr truncation or request size 413; optional for Phase 1 but would strengthen the suite.

## 4 BDD Test Gaps

- **Orchestrator suite:** Many steps return `godog.ErrSkip` when `POSTGRES_TEST_DSN` is unset or when test state (server, db, tokens) is missing.
  This is by design so that `just test-bdd` runs without a DB, but it means node registration, config fetch/ack, and dispatcher scenarios are skipped unless `POSTGRES_TEST_DSN` is set.
  Completion plan Chunk 07 asks for "no godog.ErrSkip in Phase 1 paths" when running with DB; with DB, the implemented steps should run.
  No change required for step logic; ensure CI or local validation runs BDD with DB for Phase 1 coverage.
- **Worker node suite:** Node manager scenarios use a mock orchestrator; config fetch and config ack steps are implemented.
  Sandbox scenarios use executor with `runtime: "direct"` by default, so no real container is used; network_policy and workspace behavior are still asserted via the direct runner.
  For full container-based validation, a separate run with a real runtime (e.g. podman) would be needed; not strictly required for Phase 1 BDD.
- **E2E:** `just e2e` runs the full demo (Postgres, control-plane, user-gateway, node, happy path).
  The E2E feature steps are exercised by the same flow; no separate Godog suite is referenced for e2e in the justfile (e2e is script-driven).
  The feature file serves as documentation and traceability; the script must keep steps and feature narrative aligned.

## 5 Recommendations

1. **config_version:** Implement ULID generation when creating or updating node config version (e.g. in `NodeHandler.GetConfig`) and persist it; keep lexicographic ordering for comparison as per spec.
2. **Ollama image:** Either extend the node configuration payload (and orchestrator config) to include an optional Ollama image reference for Phase 1, or document that Phase 1 uses node-local env (`OLLAMA_IMAGE`) and defer orchestrator-selected image to a later phase.
3. **Orchestrator fail-fast scenario:** Clarify in the feature file or spec whether "orchestrator fails fast when no inference path" is in scope for Phase 1.
   If yes, implement an orchestrator readiness check (e.g. require at least one dispatchable node or explicit "inference available" flag).
   If no, mark the scenario as deferred or adjust wording to node-side fail-fast only.
4. **Worker API readyz:** Add `GET /readyz` that returns 200 when the node is ready to accept jobs (and 503 otherwise) per `worker_api.md`.
5. **Feature coverage:** Add optional scenarios for request size 413 and stdout/stderr truncation in the worker_node sandbox feature if desired for Phase 1; otherwise leave for a later phase.
6. **BDD with DB:** Run `POSTGRES_TEST_DSN="..." just test-bdd` in CI or release checklist so that orchestrator node/config/dispatcher scenarios are not skipped and Phase 1 paths are validated.

## 6 References

- `dev_docs/mvp_phase1_completion_plan.md` (chunks 01-07, acceptance checklist, Phase 1 scope)
- `dev_docs/PHASE1_STATUS.md` (implementation summary, running locally)
- `docs/tech_specs/_main.md` (Phase 1 single node happy path)
- `docs/tech_specs/node.md`, `node_payloads.md`, `worker_api.md`, `sandbox_container.md`
- `features/` (orchestrator, worker_node, e2e)

Report generated 2026-02-20.
Do not update tech specs without explicit direction.
