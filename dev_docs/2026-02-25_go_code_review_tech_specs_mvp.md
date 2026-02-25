# Go Code Review: Tech Specs, Requirements, and MVP Alignment

- [1. Summary](#1-summary)
- [2. go_shared_libs: Contract and Payload Gaps](#2-go_shared_libs-contract-and-payload-gaps)
- [3. Orchestrator: Spec and Requirement Gaps](#3-orchestrator-spec-and-requirement-gaps)
- [4. Worker Node: Spec and Requirement Gaps](#4-worker-node-spec-and-requirement-gaps)
- [5. Agents (PMA and SBA): Spec and MVP Gaps](#5-agents-pma-and-sba-spec-and-mvp-gaps)
- [6. Requirements and MVP Plan Alignment](#6-requirements-and-mvp-plan-alignment)
- [7. Proposed Remediation Plan](#7-proposed-remediation-plan)
- [8. References](#8-references)

## 1. Summary

- Date: 2026-02-25.
- Scope: All Go code in `go_shared_libs/`, `orchestrator/`, `worker_node/`, `agents/` (and `cynork/` where relevant) reviewed against `docs/tech_specs/`, `docs/requirements/`, and `docs/mvp.md` / `docs/mvp_plan.md`.
- Output: Issues and gaps with a proposed remediation plan; no code changes (docs-only).

The codebase is largely aligned with Phase 1 and Phase 1.5 MVP scope (node registration, config delivery, job dispatch, sandbox execution, user auth, task APIs, inference proxy, prompt interpretation, cynode-pma).
Phase 2 foundations (MCP tool call audit) are present.
The following sections itemize **gaps and issues** where implementation diverges from tech specs, requirements, or the MVP plan, and propose remediation.

### 1.1 Key Findings

- **Worker API contract:** Response type missing `sba_result`, `step_executor_result`, and `artifacts`; Worker API healthz does not return body `ok` per spec.
- **Schema:** LangGraph Phase 2 tables (`workflow_checkpoints`, `task_workflow_leases`) are specified but not in Go models or migrations.
- **Node payloads:** Config payload uses single `SandboxRegistry` vs spec array `sandbox_registries`; capability report omits optional schema fields (container_runtime, gpu, network, inference, tls).
- **User API Gateway:** Task create does not accept or forward attachments; optional task name not clearly enforced per spec.
- **Step executor:** No `cynode-sse` binary or step-executor runner; Worker API and orchestrator do not handle `step_executor_result`.
- **SBA:** Agent implementation exists; in-progress reporting, timeout extension, and node-mediated result path (sync) integration with Worker API response are not fully wired for P2-10.
- **PMA langchaingo:** PMA is implemented as a direct HTTP client to Ollama (`/api/generate`); it does not use langchaingo for LLM or tool execution per mvp_plan Tech Spec Alignment.
- **Gateway healthz:** User API Gateway (and control-plane) GET /healthz may not return body `ok`; cynork status expects 200 with body containing `ok` per cli_management_app_commands_core.md.
- **cynork chat:** cynork chat command uses POST /v1/chat; spec and mvp_plan state legacy /v1/chat was removed and only POST /v1/chat/completions (OpenAI format) is the chat surface.
- **Bootstrap payload:** Optional fields trust, initial_config_version, pull_credentials (worker_node_payloads.md) are not in Go structs or buildBootstrapResponse.
- **Orchestrator readyz:** Spec requires actionable reason in 503 body and (when PMA enabled) no 200 until PMA is reachable; BDD readyz only checks dispatchable nodes; production wiring should be verified.

---

## 2. `go_shared_libs`: Contract and Payload Gaps

Shared Go contracts and node payloads are in `go_shared_libs/contracts/`.
Gaps against worker_api.md and worker_node_payloads.md are listed below.

### 2.1 Worker API (`contracts/workerapi`)

- **Issue:** `RunJobResponse` lacks `sba_result`, `step_executor_result`, `artifacts`
  - spec / requirement: worker_api.md Run Job response fields: optional `sba_result`, `step_executor_result`, `artifacts` for SBA/step-executor runner jobs
  - remediation: Add optional fields to `RunJobResponse`; types can reference `sbajob.Result` and a step-executor result type.
    Enables P2-10 and step-executor integration.
- **Issue:** Default image `alpine:latest` not aligned with spec example
  - spec / requirement: worker_api.md example uses `docker.io/library/bash:latest`
  - remediation: Prefer spec example or document that default is implementation choice; spec does not mandate a default.
    Low priority.

### 2.2 Node Payloads (`contracts/nodepayloads`)

- **Issue:** Node configuration: `SandboxRegistry` is single struct; spec defines `sandbox_registries` as **array**
  - spec / requirement: worker_node_payloads.md Node Config: "sandbox_registries (array of objects)"
  - remediation: Replace `ConfigSandboxRegistry` (single) with `SandboxRegistries []ConfigSandboxRegistryEntry` (or equivalent) in `NodeConfigurationPayload` and in config delivery code.
- **Issue:** Capability report: missing optional schema fields
  - spec / requirement: worker_node_payloads.md node_capability_report_v1: container_runtime, gpu, network, inference, tls
  - remediation: Add optional structs/fields to `CapabilityReport` so orchestrator can ingest and store full payload.
    Required for scheduling and display per spec.
- **Issue:** Bootstrap payload: optional fields trust, initial_config_version, pull_credentials not in struct or buildBootstrapResponse
  - spec / requirement: worker_node_payloads.md node_bootstrap_payload_v1: trust (ca_bundle_pem, pinned_spki_sha256), initial_config_version, pull_credentials (sandbox_registries array, model_cache)
  - remediation: Add optional fields to BootstrapResponse and orchestrator buildBootstrapResponse when TLS or registry credentials are used.
- **Issue:** Node config JSON tag is `sandbox_registry` (singular); spec field is `sandbox_registries` (array)
  - spec / requirement: worker_node_payloads.md node_configuration_payload_v1
  - remediation: Fix when converting to array: use json tag `sandbox_registries` and slice type.
- **Issue:** Bootstrap endpoints: `BootstrapEndpoints` uses `WorkerRegistrationURL`; config uses `ConfigEndpoints` with `WorkerAPITargetURL`
  - spec / requirement: worker_node_payloads.md bootstrap has worker_registration_url, node_config_url, node_report_url; config has worker_api_target_url, node_report_url
  - remediation: Confirm JSON tags match spec snake_case; no change if already correct.

### 2.3 SBA Job (`contracts/sbajob`)

- **Issue:** None critical
  - spec / requirement: cynode_sba.md Job spec and Result contract
  - remediation: Types and validation align with spec.

### 2.4 Problem Details (`contracts/problem`)

- **Issue:** None
  - spec / requirement: go_rest_api_standards.md Error Format (RFC 9457)
  - remediation: Aligned.

---

## 3. Orchestrator: Spec and Requirement Gaps

Orchestrator Go code lives in `orchestrator/`; database, handlers, dispatcher, and MCP audit are in scope.

### 3.1 Database and Schema

- **Issue:** `workflow_checkpoints` and `task_workflow_leases` not in Go models or AutoMigrate
  - spec / requirement: postgres_schema.md Workflow Checkpoints Table, Task Workflow Leases Table; langgraph_mvp.md; mvp_plan Phase 2
  - remediation: Add GORM models for both tables; include in `RunSchema` (AutoMigrate and/or DDL).
    Required for Phase 2 LangGraph integration.
- **Issue:** Jobs table: `result` stored as generic jsonb string
  - spec / requirement: worker_api.md: SBA/step-executor result shape in response; jobs.result
  - remediation: Current JSONBString is sufficient; ensure handlers persist full Worker API response (including `sba_result`/`step_executor_result`) when present.

### 3.2 Handlers and User API Gateway

- **Issue:** Task create: no support for **attachments**
  - spec / requirement: user_api_gateway.md: "optional attachments"; REQ-CLIENT-0157
  - remediation: Add attachment fields to create request and storage/forwarding path (or document as deferred with ticket).
- **Issue:** Task create: **task name** optional; uniqueness/normalization not clearly specified
  - spec / requirement: user_api_gateway.md: "optional task name"; orchestrator MUST accept, normalize, ensure uniqueness
  - remediation: Confirm CreateTask flow applies naming rules per project_manager_agent.md Task Naming; add tests or doc.
- **Issue:** Task create: input_mode and prompt interpretation
  - spec / requirement: orches.md REQ-ORCHES-0126-0128; mvp_plan Phase 1.5
  - remediation: Implemented (prompt/script/commands; default prompt->inference).
    No gap.

### 3.3 Dispatcher and Worker API Client

- **Issue:** Result handling: only stdout/stderr summary used; `sba_result`/`step_executor_result` not persisted when present
  - spec / requirement: worker_api.md response fields; job lifecycle result persistence
  - remediation: When Worker API response includes `sba_result` or `step_executor_result`, persist full value into `jobs.result` (and artifacts if present).
- **Issue:** Dispatch URL: uses `/v1/worker/jobs:run`
  - spec / requirement: worker_api.md
  - remediation: Correct.

### 3.4 MCP Gateway and Audit

- **Issue:** MCP tool call audit: table and store exist; gateway returns 501 (deny path)
  - spec / requirement: mcp_tool_call_auditing.md; P2-02
  - remediation: Aligned for foundation.
    Full allow path and P2-01/P2-03 pending.

### 3.5 Health Endpoints (Orchestrator / User Gateway)

- **Issue:** GET /healthz may not return body `ok`
  - spec / requirement: cli_management_app_commands_core.md: cynork status "MUST treat an HTTP 200 response body containing `ok` as healthy"; worker_api.md requires Worker API healthz body `ok`
  - remediation: Ensure User API Gateway and control-plane GET /healthz return 200 with plain text body `ok` so cynork status and probes behave per spec.
- **Issue:** GET /readyz actionable reason and PMA gating
  - spec / requirement: orchestrator.md HealthEndpoints: 503 "MUST include a reason that is actionable for an operator"; when PMA enabled, MUST NOT return 200 until PMA reachable
  - remediation: Confirm production readyz handler checks PMA when enabled and returns a stable reason string in 503 body (e.g. "no inference path", "PMA not reachable").

### 3.6 Auth, Middleware, Config

- **Issue:** None critical
  - spec / requirement: local_user_accounts.md; go_rest_api_standards.md
  - remediation: JWT, rate limit, problem details implemented.

---

## 4. Worker Node: Spec and Requirement Gaps

Worker node code is in `worker_node/`; Node Manager and Worker API surface are in scope.

### 4.1 Worker API Surface

- **Issue:** `GET /healthz`: spec requires plain text body **`ok`**; BDD/production mux only sends 200 with no body
  - spec / requirement: worker_api.md Health Checks: "returns 200 with plain text body `ok`"
  - remediation: In Worker API server (and BDD workerMux), write body `ok` for GET /healthz.
- **Issue:** `GET /readyz`: returns 200 `ready` / 503 with reason
  - spec / requirement: worker_api.md
  - remediation: Implemented in BDD mux; ensure production server matches.
- **Issue:** `POST /v1/worker/jobs:run`: response does not include `sba_result`, `step_executor_result`, `artifacts`
  - spec / requirement: worker_api.md Node-Mediated SBA Result (Sync); step-executor result
  - remediation: When job uses SBA runner image: after container exit, read `/job/result.json` (and `/job/artifacts/` if present), set `sba_result`/`artifacts` on response.
    Same for step-executor runner and `step_executor_result`.
- **Issue:** Request size limit (413) and stdout/stderr truncation
  - spec / requirement: worker_api.md; REQ-WORKER-0145-0147
  - remediation: BDD uses 10 MiB limit and Problem response; truncation is in executor.
    Verify limits match spec (e.g. 256 KiB stdout/stderr).

### 4.2 Node Manager and Config

- **Issue:** Config version: ULID and lexicographic comparison
  - spec / requirement: worker_node_payloads.md config_version ULID
  - remediation: Confirm orchestrator emits ULID and node compares lexicographically; already done per mvp_plan.
- **Issue:** Node payloads: capability report and config structures
  - spec / requirement: worker_node_payloads.md
  - remediation: See go_shared_libs nodepayloads gaps (array sandbox_registries, optional capability fields).

### 4.3 Production Worker API Entrypoint

- **Issue:** No `main.go` in worker_node (only BDD and packages)
  - spec / requirement: worker_node.md: Node runs Node Manager and Worker API
  - remediation: Confirm E2E/compose starts Worker API from another module or script; if not, add a small cmd in worker_node that wires executor and same routes as BDD workerMux (including healthz body `ok`).

---

## 5. Agents (PMA and SBA): Spec and MVP Gaps

Agents live in `agents/` (cynode-pma, SBA runner); Phase 1.7 and P2-09/P2-10 are in scope.

### 5.1 PMA (Cynode-Pma)

- **Issue:** PMA does not use langchaingo for LLM or tool execution
  - spec / requirement: mvp_plan.md Tech Spec Alignment: "PMA uses langchaingo (Go) for LLM and tool execution, including multiple simultaneous tool calls where supported"; cynode_pma.md; project_manager_agent.md
  - remediation: Refactor PMA chat path to use langchaingo (llms, agents, tools) instead of direct HTTP to Ollama `/api/generate`.
    Add MCP tools as langchaingo tools so PMA can invoke gateway-backed tools; support multiple simultaneous tool calls where the LLM supports it.
- **Issue:** Phase 1.7 deliverables (binary, role flag, instructions, orchestrator integration, readyz) are in place
  - spec / requirement: cynode_pma.md; pmagnt requirements
  - remediation: No change; note langchaingo gap above for Phase 2 MCP-in-the-loop.

### 5.2 SBA (Cynode-Sba)

- **Issue:** Result and artifact delivery: SBA writes `/job/result.json`; node-mediated sync path not fully wired in Worker API response
  - spec / requirement: cynode_sba.md Result contract; worker_api.md Node-Mediated SBA Result (Sync)
  - remediation: Worker node must build RunJob response with `sba_result` (and `artifacts`) from `/job/result.json` and `/job/artifacts/` when image is SBA runner.
    See Section 4.1.
- **Issue:** In-progress reporting: SBA must signal in-progress via outbound call; node may infer from process
  - spec / requirement: cynode_sba.md Job Lifecycle; worker_api.md in-progress reporting
  - remediation: Document or implement: either SBA calls orchestrator job-status endpoint, or node infers in-progress when container has started.
    Orchestrator must be able to set job in_progress.
- **Issue:** Timeout extension and remaining time in context
  - spec / requirement: cynode_sba.md TimeoutExtension; TimeRemaining
  - remediation: Optional for MVP; add when long-running SBA jobs are in scope.
- **Issue:** Todo list: SBA must build/manage todo from job context
  - spec / requirement: cynode_sba.md Todo List
  - remediation: Implemented in agents/sba (todo state).
    No gap.

### 5.3 Step Executor (Cynode-Sse)

- **Issue:** No `cynode-sse` binary or step-executor runner
  - spec / requirement: cynode_step_executor.md; worker_api.md step_executor_result
  - remediation: Phase 2: implement step-executor binary (read job from `/job/job.json`, execute steps in order, write result to `/job/result.json`); add step-executor runner image and Worker API handling of `step_executor_result`.
- **Issue:** Worker API and orchestrator do not handle `step_executor_result`
  - spec / requirement: worker_api.md
  - remediation: Add field to RunJobResponse (Section 2.1); node and orchestrator persist when step-executor image is used.

### 5.4 Cynork (CLI)

- **Issue:** cynork status Health() does not require body to contain `ok`
  - spec / requirement: cli_management_app_commands_core.md: "The CLI MUST treat an HTTP 200 response body containing `ok` as healthy"
  - remediation: In cynork gateway client, Health() should read response body and return error unless it contains the string "ok" (or match spec wording).
- **Issue:** cynork chat uses POST /v1/chat; gateway chat surface is only POST /v1/chat/completions
  - spec / requirement: openai_compatible_chat_api.md Single Chat Surface; mvp_plan "legacy POST /v1/chat removed"
  - remediation: Change cynork chat to use POST /v1/chat/completions with OpenAI-format messages; or document if gateway still exposes a compatibility /v1/chat wrapper.

---

## 6. Requirements and MVP Plan Alignment

Phases below are from docs/mvp.md and docs/mvp_plan.md; status is implementation vs spec.

### 6.1 Phase 1 / 1.5 (Done)

- Node registration, config delivery (ULID config_version), job dispatch, result collection: implemented.
- User API Gateway: auth, task create/get/result, input_mode (prompt/script/commands), default prompt->inference: implemented.
- Worker API: healthz/readyz, 413, truncation, Bearer auth: implemented (healthz body `ok` missing).
- Inference proxy and prompt interpretation: implemented.

### 6.2 Phase 1.7 (Done)

- cynode-pma binary and orchestrator integration; readyz gating for PMA: implemented.
- OpenAI-compatible chat (GET /v1/models, POST /v1/chat/completions, cynodeai.pm): implemented.

### 6.3 Phase 2 (In Progress)

- MCP tool call audit (P2-02): table and write path in place.
- MCP scoping/schema (P2-01), preference tools (P2-03): pending.
- **P2-09:** cynode-sba binary and SBA runner image: SBA runner logic in agents/sba; SBA runner **image** and node path that runs it and returns `sba_result`/artifacts need verification.
- **P2-10:** Worker API and orchestrator integration for SBA jobs: RunJobResponse and node response building lack `sba_result`/`artifacts`; orchestrator does not persist them to `jobs.result`.
- LangGraph: workflow_checkpoints and task_workflow_leases not in schema; workflow runner and lease not implemented.

### 6.4 Phase 3 / 4

- Not started; no code-level gaps beyond planning.

---

## 7. Proposed Remediation Plan

Remediation is ordered by priority; high priority unblocks Phase 2 SBA and spec compliance.

### 7.1 High Priority (Spec Compliance and P2 Unblock)

Address these first so Worker API matches the spec and P2-10 can persist SBA results.

#### 7.1.1 Worker API Response Contract (Go_shared_libs_shared_libs + Worker_node_node + Orchestrator)

- Add to `RunJobResponse`: `SbaResult *sbajob.Result`, `StepExecutorResult` (new type or `json.RawMessage`), `Artifacts` (spec-defined shape).
- Worker node: when job uses SBA runner image, after container exit read `/job/result.json` and optionally `/job/artifacts/`, set `sba_result` and `artifacts` on response; same pattern for step-executor when implemented.
- Orchestrator: when dispatching and receiving response, if `sba_result` or `step_executor_result` present, persist full value to `jobs.result`.

#### 7.1.2 Healthz Body `ok` (Worker API and User Gateway)

- Worker API: ensure GET /healthz returns 200 with plain text body `ok` (server and BDD workerMux).
- User API Gateway (and control-plane): ensure GET /healthz returns 200 with body `ok` so cynork status and cli_management_app_commands_core.md are satisfied.

#### 7.1.3 Schema: LangGraph Tables

- Add GORM models for `workflow_checkpoints` and `task_workflow_leases` per postgres_schema.md; add to migrate/AutoMigrate (or DDL) so Phase 2 workflow runner can use them.

### 7.2 Medium Priority (Payload and API Completeness)

Payload and gateway completeness for full spec alignment.

#### 7.2.1 Node Payloads

- Change node configuration payload to use `sandbox_registries` array; extend capability report struct with optional container_runtime, gpu, network, inference, tls for full ingest per spec.

#### 7.2.2 User API Gateway: Task Create

- Define and implement attachment handling (accept and store/forward) per user_api_gateway.md; clarify task name normalization and uniqueness.

#### 7.2.3 SBA In-Progress and Lifecycle

- Document or implement in-progress signalling (SBA outbound call or node inference); ensure orchestrator can set job to in_progress when node/SBA signals.

#### 7.2.4 `cynork`: Health Body Check and Chat Endpoint

- cynork: Health() should verify response body contains "ok" per spec; chat command should use POST /v1/chat/completions (OpenAI format) or align with gateway if a compatibility endpoint exists.

#### 7.2.5 PMA Langchaingo and Tool Execution

- Refactor PMA from direct Ollama HTTP to langchaingo (llms, agents).
- Wire MCP tools as langchaingo tools so PMA can use the gateway; support multiple simultaneous tool calls where the model allows.
- Aligns with mvp_plan Tech Spec Alignment and Phase 2 MCP-in-the-loop.

### 7.3 Lower Priority (Phase 2 and Beyond)

Phase 2 and follow-on work; not required for initial P2-10 closure.

#### 7.3.1 Cynode-Sse (Step Executor)

- Implement binary and runner image per cynode_step_executor.md; wire Worker API and orchestrator to `step_executor_result`.

#### 7.3.2 Sba Timeout Extension and Remaining Time

- Add when long-running SBA jobs are required; spec defines extension and context.

#### 7.3.3 Worker API Cmd (Production)

- If E2E/compose does not start Worker API from a dedicated binary, add `worker_node/cmd/worker-api` (or similar) that mounts the same routes as BDD workerMux.

### 7.4 Ordering Suggestion

- First: (1) response contract and (2) healthz so Worker API is spec-compliant and P2-10 can persist SBA results.
- Then: (3) schema for LangGraph so Phase 2 workflow can land.
- Then: (4)-(8) for payload, gateway completeness, cynork (health body, chat endpoint), and PMA langchaingo/tools.
- (9)-(11) as Phase 2 and follow-on work.

---

## 8. References

- Tech specs index: `docs/tech_specs/_main.md`
- MVP scope: `docs/mvp.md`
- MVP plan (task breakdown): `docs/mvp_plan.md`
- Worker API: `docs/tech_specs/worker_api.md`
- Worker node payloads: `docs/tech_specs/worker_node_payloads.md`
- CyNode SBA: `docs/tech_specs/cynode_sba.md`
- CyNode Step Executor: `docs/tech_specs/cynode_step_executor.md`
- Go REST API standards: `docs/tech_specs/go_rest_api_standards.md`
- Postgres schema: `docs/tech_specs/postgres_schema.md`
- User API Gateway: `docs/tech_specs/user_api_gateway.md`
- Requirements: `docs/requirements/README.md`
- Meta and tooling: `meta.md`, `justfile`
