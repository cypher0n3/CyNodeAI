# MVP Tech Specs and Requirements Gaps Analysis

- [1 Summary](#1-summary)
- [2 Phase 0 Foundations Gaps](#2-phase-0-foundations-gaps)
- [3 Phase 1 Spec and Requirements Gaps](#3-phase-1-spec-and-requirements-gaps)
- [4 Schema and Config Storage Gaps](#4-schema-and-config-storage-gaps)
- [5 Requirements vs MVP Scope](#5-requirements-vs-mvp-scope)
- [6 BDD and Traceability Gaps](#6-bdd-and-traceability-gaps)
- [7 Recommendations](#7-recommendations)

## 1 Summary

This document identifies major gaps and issues between the tech specs (`docs/tech_specs/`), requirements (`docs/requirements/`), and the MVP development plan (`docs/tech_specs/_main.md` lines 113-158, `dev_docs/mvp_phase1_completion_plan.md`).
It focuses on Phase 0 and Phase 1 and does not change any specs; it calls out ambiguities and missing definitions for resolution.

Key findings (updated for current completion plan):

- **Resolved by completion plan:** Bootstrap payload shape (Phase 1 minimal subset in Chunk 02), Worker API auth (static token via config, HTTP for internal traffic in Chunk 03), config delivery API (`node_config_url` GET/POST and paths from bootstrap),
  inference precondition ownership (node enforces: Node Manager must not report ready until Ollama is running; see Section 3.1.0.2), and Phase 1 scope decisions in Section 1.1 (config_version, worker_api_target_url stored per node, network_policy=restricted as deny-all, no resource limits, workspace at /workspace, config ack applied/failed only).
- **Phase 0:** MCP gateway enforcement is fully specified but Phase 1 does not use the MCP gateway; Phase 1 task execution is direct (orchestrator -> Worker API).
  The completion plan does not explicitly state this; stating it would avoid confusion.
- **Schema:** The completion plan locks that `worker_api_target_url` is "provided by orchestrator bootstrap configuration and stored per node" but `postgres_schema.md` does not define the columns or table for that storage (or for the bearer token).
  Config ack recording is required by Chunk 03; the schema doc does not define where acks are stored.
- **Requirements vs MVP:** Many ORCHES, USRGWY, and WORKER requirements are Phase 2 or later; the MVP plan does not list which requirements are deferred.
- **BDD:** No scenarios yet for config delivery/ack or inference precondition; adding them would align with Chunks 03, 04, 06 and the definition of done.

## 2 Phase 0 Foundations Gaps

Issues in the foundations that affect or clarify Phase 1.

### 2.1 MCP Gateway and Phase 1

- Phase 0 requires defining MCP gateway enforcement and tool allowlists (`_main.md` Phase 0).
- Phase 1 does not include "MCP in the loop"; job dispatch is orchestrator -> Worker API (HTTP), not via MCP tools.
- **Gap:** The MVP plan does not state that Phase 1 task execution bypasses the MCP gateway.
  Clarifying this avoids confusion about whether Phase 1 must implement the gateway for the happy path.

### 2.2 Postgres Schema Completeness for Phase 1

- Phase 0 requires schema for "users, local auth sessions, groups and RBAC, tasks, jobs, nodes, artifacts, and audit logging."
- `postgres_schema.md` defines `nodes` with `config_version` but not where the orchestrator stores the node configuration payload it delivers (Worker API URL, bearer token, etc.).
- See [Section 4](#4-schema-and-config-storage-gaps).

## 3 Phase 1 Spec and Requirements Gaps

Spec and requirement ambiguities that affect Phase 1 implementation.

### 3.1 Resolved by Completion Plan (Section 1.1 and Chunks 01-03)

The following are now locked in `dev_docs/mvp_phase1_completion_plan.md`:

- **Bootstrap payload:** Phase 1 minimal subset defined in Chunk 02 (Section 3.2.1): `version`, `issued_at`, `orchestrator.base_url`, `orchestrator.endpoints` (worker_registration_url, node_report_url, node_config_url), `auth.node_jwt`, `auth.expires_at`.
  Rest optional for Phase 1.
- **Worker API auth:** Static bearer token delivered via node config; HTTP for component-to-component traffic (Chunk 03, Section 1).
- **Config delivery:** `node_config_url` GET returns `node_configuration_payload_v1`; POST accepts `node_config_ack_v1`.
  Endpoint paths are in the bootstrap payload (Section 1, Chunk 03).
- **Inference precondition and fail-fast:** Resolved in Section 3.1.0.1 and 3.1.0.2.
  For MVP Phase 1, inference is node-local Ollama only.
  The **node** enforces ready state: the Node Manager must not report the node as ready for job dispatch until the Ollama container is running; otherwise it must exit with an error or remain in a non-ready state and report the failure.

### 3.2 Sandbox Constraints and Worker API

- `worker_api.md` allows `network_policy`: `restricted` or `none`; requires basic safety limits (REQ-WORKER-0105); node must not expose credentials to sandboxes (REQ-WORKER-0104).
- `sandbox_container.md` specifies `/workspace`, env vars (`CYNODE_TASK_ID`, `CYNODE_JOB_ID`, `CYNODE_WORKSPACE_DIR`), and no secrets in env.
- **Resolved for Phase 1:** Completion plan Section 1.1 locks: `network_policy=restricted` treated as deny-all (equivalent to `none`); no CPU, memory, or PIDs limits for MVP Phase 1; per-job workspace mounted read/write at `/workspace`.
  Chunk 05 implements workspace mount and env per sandbox_container.md.
  No remaining spec ambiguity for Phase 1.

## 4 Schema and Config Storage Gaps

PostgreSQL schema does not fully define storage for node config and acks.
The completion plan has locked behavior; the schema doc still omits definitions.

### 4.1 Storing Delivered Node Config for Dispatch

- The completion plan Section 1.1 states: `worker_api_target_url` is "provided by orchestrator bootstrap configuration and stored per node."
  Chunk 06 requires the dispatcher to use per-node endpoint and per-node bearer token from config delivery.
- `postgres_schema.md` defines `nodes.config_version` but does not define:
  - Where `worker_api_target_url` is stored (column on `nodes` or elsewhere).
  - Where the bearer token (or a reference) is stored for the dispatcher and for serving the node config endpoint.
- **Gap:** Implementation will store these per node; the schema doc should define the columns or a dedicated table (or explicit `nodes.metadata` keys) so that migrations and tooling stay consistent.

### 4.2 Config Acknowledgement Storage

- `node_payloads.md` defines `node_config_ack_v1`; the completion plan Section 1.1 locks config ack status to `applied` and `failed` only.
  Chunk 03 requires POST to `node_config_url` to accept acks and "record node acknowledgements for visibility."
- **Gap:** `postgres_schema.md` does not define where acks are stored (e.g. columns on `nodes` such as `config_ack_at`, `config_ack_status`, `config_ack_error`, or a separate node_config_acks table).
  The schema doc should specify one approach for Phase 1.

## 5 Requirements vs MVP Scope

Many requirements apply to later phases; Phase 1 scope is not fully mapped.

### 5.1 Orchestrator (ORCHES) Requirements

- REQ-ORCHES-0100--0108 (scheduler, cron, time-zone, run history, MCP exposure, User API Gateway for schedule/queue) are not in Phase 1.
- REQ-ORCHES-0112, 0113 (config at registration, dynamic config, capability ingest) are Phase 1.
- REQ-ORCHES-0120--0124 (tasks, create task, dispatch, persist results, read task state) are Phase 1.
- **Gap:** There is no single document that maps requirement IDs to MVP phases.
  A short "MVP requirement scope" section in the completion plan or in `_main.md` would make it explicit which REQ-* are in scope for Phase 1 and which are deferred.

### 5.2 User Gateway (USRGWY) Requirements

- Phase 1 User API Gateway: "local user auth (login and refresh), create task, and retrieve task result."
- USRGWY requirements include runs/sessions, logs, transcripts, retention, streaming, background process management, messaging destinations (REQ-USRGWY-0100--0120), and admin console (REQ-USRGWY-0126).
- **Gap:** Phase 1 does not require runs/sessions API, streaming, or admin console; these are later phases.
  Explicitly marking which USRGWY requirements are Phase 1 vs deferred would align requirements with the MVP plan.

### 5.3 Worker (WORKER) Requirements

- REQ-WORKER-0112: "The node MUST stream sandbox stdout and stderr back to the orchestrator."
  The initial Worker API is synchronous and returns stdout/stderr in the response; streaming is not required for Phase 1.
- REQ-WORKER-0116--0119: Node MCP server, allowlist, auditing for sandbox MCP are Phase 2 (MCP in the loop).
- **Gap:** Same as above: a phase-to-requirement mapping would clarify that streaming and node MCP are post-Phase 1.

## 6 BDD and Traceability Gaps

Acceptance tests and traceability for Phase 1.

### 6.1 Feature Coverage

- `features/single_node_happy_path.feature` covers: login, refresh, logout, user info, node registration, capability reporting, create task, get task status, end-to-end task execution, get task result.
- **Gaps:**
  - No scenario for "node fetches config after registration" or "node sends config ack."
  - No scenario for "dispatcher uses per-node worker URL and token from config."
  - No scenario for sandbox constraints (workspace mount, network_policy, resource limits) or inference precondition (fail fast when no inference path).
- Adding minimal scenarios for config delivery and config ack would strengthen the Phase 1 definition of done and align with Chunks 03, 04, and 06.

### 6.2 Requirement and Spec Anchors

- Many scenarios already tag `@req_*` and `@spec_*`; good for traceability.
- New scenarios for config delivery and inference precondition should trace to the relevant REQ and spec IDs (e.g. REQ-WORKER-0135, REQ-BOOTST-0002, node_payloads.md, node.md).

## 7 Recommendations

1. **Done (completion plan):** Phase 1 acceptance criteria, inference precondition, and fail-fast are locked in Section 3.1.0.
   Inference enforcement is defined: the node (Node Manager) refuses ready until Ollama is running.
   Bootstrap minimal subset and config delivery API are defined in Chunks 02 and 03; Worker API auth and transport decisions are in Section 1 and Chunk 03.
2. **Extend postgres_schema.md** (or a referenced spec) to define where the orchestrator stores:
   - Per-node config used for delivery and dispatch (at least `worker_api_target_url` and bearer token or reference), and
   - Config acknowledgement (per-node columns or dedicated table).
3. **Optional:** Add a short MVP requirement scope to the completion plan or `_main.md`: list REQ-* IDs in Phase 1 vs deferred (ORCHES, USRGWY, WORKER, BOOTST, IDENTY, SANDBX, SCHEMA).
4. **Optional:** State in the MVP plan that Phase 1 task execution does not use the MCP gateway (direct HTTP to Worker API).
5. **Optional:** Extend `single_node_happy_path.feature` with scenarios for config delivery/ack and inference precondition, tagged with requirement and spec anchors.

---

Generated 2026-02-17.
Updated to align with `dev_docs/mvp_phase1_completion_plan.md` (Section 1.1, Chunks 01-03, 3.1.0).
Do not update tech specs without explicit direction; this report is for gap identification only.
