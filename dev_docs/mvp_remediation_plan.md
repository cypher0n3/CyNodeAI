# MVP Code Review and Remediation Plan

- [1. Summary](#1-summary)
  - [1.1 Key Findings (Code Review)](#11-key-findings-code-review)
- [2. Go Code Review: Tech Specs and MVP Alignment](#2-go-code-review-tech-specs-and-mvp-alignment)
  - [2.1 `go_shared_libs` - Contract and Payload Gaps](#21-go_shared_libs---contract-and-payload-gaps)
  - [2.2 Orchestrator: Spec and Requirement Gaps](#22-orchestrator-spec-and-requirement-gaps)
  - [2.3 Worker Node: Spec and Requirement Gaps](#23-worker-node-spec-and-requirement-gaps)
  - [2.4 Agents (PMA and SBA): Spec and MVP Gaps](#24-agents-pma-and-sba-spec-and-mvp-gaps)
  - [2.5 Requirements and MVP Plan Alignment](#25-requirements-and-mvp-plan-alignment)
- [3. SBA Gaps and Remediation](#3-sba-gaps-and-remediation)
  - [3.1 Spec and Contract Design](#31-spec-and-contract-design)
  - [3.2 Implementation and Tests](#32-implementation-and-tests)
  - [3.3 SBA Remediation Plan (Spec and Code)](#33-sba-remediation-plan-spec-and-code)
- [4. Combined Remediation Plan](#4-combined-remediation-plan)
  - [4.0 Rank-Ordered Master List (All Issues)](#40-rank-ordered-master-list-all-issues)
  - [4.1 High Priority (Spec Compliance and P2 Unblock)](#41-high-priority-spec-compliance-and-p2-unblock)
  - [4.2 Medium Priority (Payload, API, SBA Spec)](#42-medium-priority-payload-api-sba-spec)
  - [4.3 Lower Priority (Phase 2 and Beyond)](#43-lower-priority-phase-2-and-beyond)
  - [4.4 Ordering Suggestion](#44-ordering-suggestion)
- [5. References](#5-references)

## 1. Summary

**Date:** 2026-02-25 (updated 2026-02-26 with additional issues).

This doc consolidates:

- **Go code review:** All Go code in `go_shared_libs/`, `orchestrator/`, `worker_node/`, `agents/` (and `cynork/` where relevant) reviewed against `docs/tech_specs/`, `docs/requirements/`, and `docs/mvp.md` / `docs/mvp_plan.md`.
  Output: issues and gaps with proposed remediation; no code changes (docs-only).
- **SBA gaps:** Job contract design analysis (spec/requirement clarifications), SBA code review (node-manager BDD step, optional doc comments), and initial build-out summary.
  Remediation ordered by dependency: spec updates first, then code/contract changes.

The codebase is largely aligned with Phase 1 and Phase 1.5 MVP scope.
Phase 2 foundations (MCP tool call audit) are present.
The following sections itemize **gaps and issues** and propose remediation.

### 1.1 Key Findings (Code Review)

- **Worker API contract:** Response type missing `sba_result`, `step_executor_result`, and `artifacts`; Worker API healthz does not return body `ok` per spec.
- **Schema:** LangGraph Phase 2 tables (`workflow_checkpoints`, `task_workflow_leases`) are specified but not in Go models or migrations.
- **Node payloads:** Config payload uses single `SandboxRegistry` vs spec array `sandbox_registries`; capability report omits optional schema fields (container_runtime, gpu, network, inference, tls).
- **User API Gateway:** Task create does not accept or forward attachments; optional task name not clearly enforced per spec.
- **Step executor:** No `cynode-sse` binary or step-executor runner; Worker API and orchestrator do not handle `step_executor_result`.
- **SBA:** Agent implementation exists; in-progress reporting, timeout extension, and node-mediated result path (sync) integration with Worker API response are not fully wired for P2-10.
- **PMA langchaingo:** PMA is implemented as a direct HTTP client to Ollama; it does not use langchaingo for LLM or tool execution per mvp_plan Tech Spec Alignment.
- **Gateway healthz:** User API Gateway (and control-plane) GET /healthz may not return body `ok`; cynork status expects 200 with body containing `ok` per cli_management_app_commands_core.md.
- **cynork chat:** cynork chat command uses POST /v1/chat; spec and mvp_plan state legacy /v1/chat was removed and only POST /v1/chat/completions (OpenAI format) is the chat surface.
- **Bootstrap payload:** Optional fields trust, initial_config_version, pull_credentials (worker_node_payloads.md) are not in Go structs or buildBootstrapResponse.
- **Orchestrator readyz:** Spec requires actionable reason in 503 body and (when PMA enabled) no 200 until PMA is reachable; BDD readyz only checks dispatchable nodes; production wiring should be verified.
- **Local user accounts:** Admin MVP endpoints (POST /users, POST /users/{id}/disable, enable, revoke_sessions, reset_password) per local_user_accounts.md are not implemented; database has InvalidateAllUserSessions but no HTTP routes/handlers.

---

## 2. Go Code Review: Tech Specs and MVP Alignment

Gaps by area below.

### 2.1 `go_shared_libs` - Contract and Payload Gaps

Shared Go contracts and node payloads are in `go_shared_libs/contracts/`.
Gaps against worker_api.md and worker_node_payloads.md.

#### 2.1.1 Worker API (`contracts/workerapi`)

- **Issue:** `RunJobResponse` lacks `sba_result`, `step_executor_result`, `artifacts` (spec: worker_api.md).
  Remediation: Add optional fields to `RunJobResponse`; types can reference `sbajob.Result` and a step-executor result type.
  Enables P2-10 and step-executor integration.
- **Issue:** Default image `alpine:latest` not aligned with spec example (`docker.io/library/bash:latest`).
  Low priority; document or prefer spec example.

#### 2.1.2 Node Payloads (`contracts/nodepayloads`)

- **Issue:** Node configuration: `SandboxRegistry` is single struct; spec defines `sandbox_registries` as **array**.
  Remediation: Replace `ConfigSandboxRegistry` (single) with `SandboxRegistries []ConfigSandboxRegistryEntry` (or equivalent) in `NodeConfigurationPayload` and config delivery code.
- **Issue:** Node config JSON tag is `sandbox_registry` (singular); spec field is `sandbox_registries` (array).
  Remediation: Use json tag `sandbox_registries` and slice type.
- **Issue:** Capability report: missing optional schema fields (container_runtime, gpu, network, inference, tls).
  Remediation: Add optional structs/fields to `CapabilityReport` so orchestrator can ingest and store full payload.
  Required for scheduling and display per spec.
- **Issue:** Bootstrap payload: optional fields trust, initial_config_version, pull_credentials not in struct or buildBootstrapResponse.
  Spec: worker_node_payloads.md node_bootstrap_payload_v1: trust (ca_bundle_pem, pinned_spki_sha256), initial_config_version, pull_credentials (sandbox_registries array, model_cache).
  Remediation: Add optional fields to BootstrapResponse and orchestrator buildBootstrapResponse when TLS or registry credentials are used.
- **Issue:** Bootstrap endpoints: `BootstrapEndpoints` uses `WorkerRegistrationURL`; config uses `ConfigEndpoints` with `WorkerAPITargetURL`.
  Spec: bootstrap has worker_registration_url, node_config_url, node_report_url; config has worker_api_target_url, node_report_url.
  Remediation: Confirm JSON tags match spec snake_case; no change if already correct.

#### 2.1.3 SBA Job (`contracts/sbajob`)

- Types and validation align with spec.
  See [Section 3](#3-sba-gaps-and-remediation) for SBA-specific spec/contract design gaps.

#### 2.1.4 Problem Details (`contracts/problem`)

- Aligned with go_rest_api_standards.md (RFC 9457).

### 2.2 Orchestrator: Spec and Requirement Gaps

- **Schema:** Add GORM models for `workflow_checkpoints` and `task_workflow_leases` per postgres_schema.md; include in `RunSchema` (AutoMigrate and/or DDL).
  Required for Phase 2 LangGraph integration.
  Jobs table: `result` stored as generic jsonb string; current JSONBString is sufficient; ensure handlers persist full Worker API response (including `sba_result`/`step_executor_result`) when present.
- **Task create:** No support for attachments (user_api_gateway.md; REQ-CLIENT-0157).
  Remediation: Add attachment fields to create request and storage/forwarding path (or document as deferred with ticket).
  Optional task name: user_api_gateway.md says orchestrator MUST accept, normalize, ensure uniqueness; confirm CreateTask applies naming rules per project_manager_agent.md Task Naming; add tests or doc.
  input_mode and prompt interpretation (REQ-ORCHES-0126-0128): implemented (prompt/script/commands; default prompt->inference); no gap.
- **Dispatcher:** Result handling: only stdout/stderr summary used today; when Worker API response includes `sba_result` or `step_executor_result`, persist full value into `jobs.result` (and artifacts if present).
  Dispatch URL uses `/v1/worker/jobs:run`; correct per spec.
- **MCP:** Tool call audit table and store exist; gateway returns 501 (deny path).
  Aligned for foundation.
  Full allow path and P2-01/P2-03 pending.
- **Health:** GET /healthz: ensure User API Gateway and control-plane return 200 with plain text body `ok` so cynork status and cli_management_app_commands_core.md are satisfied.
  GET /readyz: 503 MUST include a reason that is actionable for an operator; when PMA enabled, MUST NOT return 200 until PMA reachable.
  Confirm production readyz handler checks PMA when enabled and returns a stable reason string in 503 body (e.g. "no inference path", "PMA not reachable").
  Auth, middleware, config: JWT, rate limit, problem details implemented; no critical gap.
- **Local user accounts (admin MVP endpoints):** local_user_accounts.md lists Minimum MVP endpoints (admin-gated): POST /users, POST /users/{id}/disable, enable, revoke_sessions, reset_password.
  User logout (POST /v1/auth/logout) revokes the sent refresh token; "Users MUST be able to revoke refresh tokens" is satisfied.
  Database has InvalidateAllUserSessions; there is no HTTP route or handler for POST /users/{id}/revoke_sessions or the other admin lifecycle endpoints.
  Remediation: implement at least POST /users/{id}/revoke_sessions so "Admins MUST be able to revoke all active sessions for a user"; optionally document deferral of the rest with a ticket.

### 2.3 Worker Node: Spec and Requirement Gaps

- **Worker API:** GET /healthz: spec requires plain text body **`ok`**; BDD/production mux only sends 200 with no body today.
  Remediation: in Worker API server (and BDD workerMux), write body `ok` for GET /healthz.
  GET /readyz: returns 200 `ready` / 503 with reason; implemented in BDD mux; ensure production server matches.
  POST /v1/worker/jobs:run: response does not include `sba_result`, `step_executor_result`, `artifacts`; when job uses SBA runner image, after container exit read `/job/result.json` (and `/job/artifacts/` if present), set `sba_result`/`artifacts` on response; same for step-executor runner and `step_executor_result`.
  Request size limit (413) and stdout/stderr truncation: worker_api.md; REQ-WORKER-0145-0147.
  BDD uses 10 MiB limit and Problem response; truncation is in executor; verify limits match spec (e.g. 256 KiB stdout/stderr).
- **Node payloads:** See go_shared_libs nodepayloads (array sandbox_registries, optional capability fields).
  Config version: ULID and lexicographic comparison per worker_node_payloads.md; confirm orchestrator emits ULID and node compares lexicographically; already done per mvp_plan.
- **Production:** worker_node.md says node runs Node Manager and Worker API.
  Confirm E2E/compose starts Worker API from another module or script; if not, add a small cmd in worker_node that wires executor and same routes as BDD workerMux (including healthz body `ok`).

### 2.4 Agents (PMA and SBA): Spec and MVP Gaps

- **PMA:** PMA is implemented as direct HTTP to Ollama `/api/generate`; mvp_plan Tech Spec Alignment requires "PMA uses langchaingo (Go) for LLM and tool execution, including multiple simultaneous tool calls where supported".
  Remediation: refactor PMA chat path to use langchaingo (llms, agents, tools); add MCP tools as langchaingo tools so PMA can invoke gateway-backed tools; support multiple simultaneous tool calls where the LLM supports it.
  Phase 1.7 deliverables (binary, role flag, instructions, orchestrator integration, readyz) are in place; note langchaingo gap for Phase 2 MCP-in-the-loop.
- **SBA:** Result/artifact delivery: SBA writes `/job/result.json`; node-mediated sync path not fully wired in Worker API response.
  Worker node must build RunJob response with `sba_result` (and `artifacts`) from `/job/result.json` and `/job/artifacts/` when image is SBA runner.
  In-progress reporting: SBA must signal in-progress via outbound call; node may infer from process; document or implement (SBA calls orchestrator job-status endpoint or node infers in-progress when container started); orchestrator must be able to set job in_progress.
  Timeout extension and remaining time: cynode_sba.md TimeoutExtension, TimeRemaining; optional for MVP; add when long-running SBA jobs are in scope.
  Todo list: implemented in agents/sba (todo state); no gap.
- **Step executor (cynode-sse):** No `cynode-sse` binary or step-executor runner.
  Spec: cynode_step_executor.md; worker_api.md step_executor_result.
  Remediation (Phase 2): implement step-executor binary (read job from `/job/job.json`, execute steps in order, write result to `/job/result.json`); add step-executor runner image and Worker API handling of `step_executor_result`.
  Add field to RunJobResponse; node and orchestrator persist when step-executor image is used.
- **cynork:** cynork status Health() does not require body to contain `ok`; cli_management_app_commands_core.md says CLI MUST treat HTTP 200 response body containing `ok` as healthy.
  Remediation: in cynork gateway client, Health() should read response body and return error unless it contains the string "ok".
  cynork chat uses POST /v1/chat; gateway chat surface is only POST /v1/chat/completions (openai_compatible_chat_api.md Single Chat Surface; mvp_plan "legacy POST /v1/chat removed").
  Remediation: change cynork chat to use POST /v1/chat/completions with OpenAI-format messages; or document if gateway still exposes a compatibility /v1/chat wrapper.

### 2.5 Requirements and MVP Plan Alignment

- **Phase 1 / 1.5:** Node registration, config delivery, job dispatch, result collection, User API Gateway (auth, task create/get/result, input_mode), Worker API healthz/readyz/413/truncation/Bearer: implemented (healthz body `ok` missing).
  Inference proxy and prompt interpretation: implemented.
- **Phase 1.7:** cynode-pma binary and orchestrator integration; OpenAI-compatible chat: implemented.
- **Phase 2:** MCP tool call audit (P2-02) table and write path in place.
  P2-09: SBA runner logic in agents/sba; SBA runner **image** and node path that returns `sba_result`/artifacts need verification.
  P2-10: RunJobResponse and node response building lack `sba_result`/`artifacts`; orchestrator does not persist them.
  LangGraph: workflow_checkpoints and task_workflow_leases not in schema; workflow runner and lease not implemented.
- **Phase 3 / 4:** Not started.

---

## 3. SBA Gaps and Remediation

Open items from SBA contract design analysis and code review.
Remediation ordered by dependency: spec updates first, then code.

### 3.1 Spec and Contract Design

All from analysis of `go_shared_libs/contracts/sbajob/sbajob.go` vs `docs/tech_specs/cynode_sba.md` and `docs/requirements/sbagnt.md`.
No code changes proposed until spec approval.

- **Inference model selection.** `AllowedModels []string` may be insufficient (e.g. provider, fallback order, capabilities).
  Spec: allowlist of opaque model identifiers; runtime maps to endpoints.
  Gap: No explicit "future considerations" for richer model entries; optional extended shape not defined.
- **Context preferences shape.** `preferences` as `map[string]string` is flat vs scoped semantics (scope_type, scope_id, key, value, value_type).
  Spec: effective preference map; orchestrator resolves and attaches.
  Gap: Spec does not clearly state that `context.preferences` is the effective map (key to JSON value) and link to user_preferences.md; no note on possible future structured form.
- **Skills type.** `Skills` as `interface{}` lacks type safety and clear contract.
  Spec: skills may be inline content, stable ids, or paths; multiple shapes allowed.
  Gap: Spec does not enumerate allowed JSON shapes (e.g. array of ids, array of objects with id/content, map id to content); implementors cannot replace `interface{}` with a union type without spec definition.
- **Step structure and execution flexibility.**
  Fixed step `type` + `args` may be too rigid.
  Spec: MVP step types; SBA builds todo list and may use inference and MCP throughout.
  Gap: Not explicitly stated that job steps are initial/suggested and SBA MAY use inference and MCP at any time; no extensibility note for future step types (e.g. `call_tool`, `call_llm`).

### 3.2 Implementation and Tests

- **Node manager BDD step (worker_node/_bdd/steps.go).**
  The step "the node manager runs the startup sequence against the mock orchestrator" was changed to run the node-manager **binary** via `exec` instead of in-process `nodemanager.RunWithOptions`.
  Gap: Exec'ing the binary drops use of `nodemanager.RunWithOptions` and `st.failInferenceStartup`, so the @wip fail-fast scenario cannot pass; adds binary discovery/build and subprocess overhead. **Required fix:** Refactor to call an **exported library entrypoint** (e.g. `nodemanager.RunWithOptions`) so the real code path is exercised and options like `failInferenceStartup` remain injectable; remove or avoid reliance on `ensureNodeManagerBinary` for this step.
  Use `context.WithTimeout` and cancel so the runner can be stopped with no goroutine leak.
- **Result status (optional).**
  Result `status` is not validated as one of `success|failure|timeout` in the contract package.
  Spec defines that set; acceptable but could be documented or enforced if same types used for ingestion.
- **ContextSpec.
  Skills (optional).**
  Add a short comment that it may be inline content or structured data per CYNAI SBAGNT JobContext.

### 3.3 SBA Remediation Plan (Spec and Code)

Ordered by dependency: spec updates first, then code.

#### 3.3.1 Spec Updates (`cynode_sba.md`)

- **Inference model.**
  Option A (recommended for MVP): Add one paragraph under JobInferenceModel that `allowed_models` is an allowlist of opaque model identifiers and the runtime maps them to endpoints; add "Future considerations" for structured model entries (provider, priority, capabilities) if requirements emerge.
  Option B: Add optional extended shape (e.g. object with id, optional source, priority/capabilities) with "array of strings" remaining valid; then update Go contract in a separate change.
- **Preferences shape.**
  Option A (recommended): Clarify in cynode_sba.md that `context.preferences` is the effective preference map (key to JSON value) as produced by the orchestrator's resolution; link to user_preferences.md; note that value types may be string, number, boolean, object, or array; document possible future structured form (e.g. array of entries with key, value, value_type, scope_type).
  Option B: Define PreferenceEntry in spec and allow map or array form; then update Go contract.
- **Skills type.**
  Option A (recommended): In cynode_sba.md, define allowed JSON shapes for `skills` (e.g. array of strings (ids), array of objects with id and optional content, or map id to content); then in a separate code change update the Go contract to use a tagged union or generic type instead of `interface{}`.
  Option B (minimal): Keep spec as-is; in Go, replace `interface{}` with `json.RawMessage` and add comment that SBA/orchestrator unmarshals into one of the documented shapes.
- **Step flexibility.**
  Option A (recommended): Add a short subsection under Step Types (MVP) or Execution Model in cynode_sba.md: job steps are the initial/suggested sequence; the SBA MAY use inference and MCP tools at any time; the SBA builds and updates a todo list from requirements, acceptance criteria, and steps; current step types are MVP primitives and future minor versions MAY add step types (e.g. `call_tool`, `call_llm`) or an extension point for custom step types.
  Option B: Define that `steps[].type` may be MVP types or implementation-defined (e.g. namespaced); document unknown-type handling per protocol version; then Go contract can keep `Type string` and `Args json.RawMessage` with documented behavior.

#### 3.3.2 Code Refactor (`worker_node`)

- Refactor the node manager BDD step to call an exported library entrypoint (e.g. `nodemanager.RunWithOptions`) instead of exec'ing the binary.
  Pass config derived from env and inject `st.failInferenceStartup` via options so the @wip fail-fast scenario can pass when enabled.
  Remove or avoid reliance on `ensureNodeManagerBinary` for this step.
  When using the in-process entrypoint, ensure `context.WithTimeout` (or equivalent) and cancel are used so the runner can be stopped on timeout with no goroutine leak.

#### 3.3.3 Optional Doc Comments

- One-line comment on `ContextSpec.Skills` (allowed shapes per spec).
- One-line comment on `Result.Status` (allowed values: success|failure|timeout).

---

## 4. Combined Remediation Plan

Ordered by priority; high priority unblocks Phase 2 SBA and spec compliance.

### 4.0 Rank-Ordered Master List (All Issues)

Most important first.
Each item appears in one of the priority tiers below with full detail.

1. **Worker API response contract** - Add sba_result, step_executor_result, artifacts; node and orchestrator persist (P2-10).
2. **Healthz body `ok`** - Worker API and User API Gateway/control-plane return 200 with body `ok` (spec + cynork status).
3. **Schema: LangGraph tables** - workflow_checkpoints, task_workflow_leases in GORM/migrations.
4. **Local user accounts: Admin MVP endpoints** - At least POST /users/{id}/revoke_sessions; optionally disable, enable, reset_password, POST /users.
5. **Node payloads** - sandbox_registries array; capability report optional fields.
6. **User API Gateway: Task create** - Attachments; task name normalization/uniqueness.
7. **cynork** - Health() verify body contains "ok"; chat use POST /v1/chat/completions; update `cynork_cli.md` implemented list.
8. **SBA in-progress and lifecycle** - Signalling; orchestrator can set job in_progress.
9. **PMA langchaingo** - Refactor from direct Ollama HTTP to langchaingo; MCP tools; multiple tool calls.
10. **SBA spec and BDD refactor** - Spec updates (inference, preferences, skills, steps); node manager BDD step use nodemanager.RunWithOptions.
11. **Schema: Sandbox image registry tables** - Confirm GORM/DDL for sandbox_images, sandbox_image_versions, node_sandbox_image_availability; add if missing.
12. **Worker stdout/stderr 256 KiB** - Verification only: executor default and tests match spec.
13. **REQ-CLIENT-0004 parity baseline** - Document current CLI capability set for future Web Console parity.
14. **CyNode-Sse (step executor)** - Binary, runner image, Worker API/orchestrator step_executor_result (Phase 2).
15. **SBA timeout extension and remaining time** - When long-running SBA jobs in scope.
16. **Worker API cmd (production)** - Dedicated binary if E2E/compose does not provide one.

### 4.1 High Priority (Spec Compliance and P2 Unblock)

- **Worker API response contract (go_shared_libs + worker_node + orchestrator):** Add to `RunJobResponse`: `SbaResult *sbajob.Result`, `StepExecutorResult` (new type or `json.RawMessage`), `Artifacts` (spec-defined shape).
  Worker node: when job uses SBA runner image, after container exit read `/job/result.json` and optionally `/job/artifacts/`, set `sba_result` and `artifacts` on response; same pattern for step-executor when implemented.
  Orchestrator: when dispatching and receiving response, if `sba_result` or `step_executor_result` present, persist full value to `jobs.result`.
- **Healthz body `ok`:** Worker API: ensure GET /healthz returns 200 with plain text body `ok` (server and BDD workerMux).
  User API Gateway (and control-plane): ensure GET /healthz returns 200 with body `ok` so cynork status and cli_management_app_commands_core.md are satisfied.
- **Schema: LangGraph tables:** Add GORM models for `workflow_checkpoints` and `task_workflow_leases` per postgres_schema.md; add to migrate/AutoMigrate (or DDL) so Phase 2 workflow runner can use them.

### 4.2 Medium Priority (Payload, API, SBA Spec)

- **Local user accounts: Admin MVP endpoints:** Implement at least POST /users/{id}/revoke_sessions per local_user_accounts.md ("Admins MUST be able to revoke all active sessions for a user").
  Optionally implement POST /users, POST /users/{id}/disable, enable, reset_password; or document deferral with a ticket.
- **Node payloads:** Use `sandbox_registries` array in node configuration; extend capability report with optional container_runtime, gpu, network, inference, tls.
- **User API Gateway: Task create:** Define and implement attachment handling; clarify task name normalization and uniqueness.
- **cynork:** Health() verify body contains "ok"; chat use POST /v1/chat/completions or align with gateway.
  Update `cynork_cli.md` so the "Implemented on gateway" list shows POST /v1/chat/completions (remove or correct POST /v1/chat) to avoid spec drift.
- **SBA in-progress and lifecycle:** Document or implement in-progress signalling; ensure orchestrator can set job to in_progress.
- **PMA langchaingo:** Refactor PMA from direct Ollama HTTP to langchaingo (llms, agents); wire MCP tools as langchaingo tools; support multiple simultaneous tool calls where model allows.
- **SBA spec and BDD refactor:** Apply SBA spec updates per [Section 3.3](#33-sba-remediation-plan-spec-and-code); refactor node manager BDD step to use `nodemanager.RunWithOptions`.

### 4.3 Lower Priority (Phase 2 and Beyond)

- **Schema: Sandbox image registry tables:** Confirm GORM models and DDL for sandbox_images, sandbox_image_versions, node_sandbox_image_availability are present and in schema bootstrap; add if missing so schema matches postgres_schema.md for MVP (full registry behavior deferred).
- **Worker stdout/stderr 256 KiB:** Verification only; ensure worker node executor uses 256 KiB default and BDD/unit tests document it (worker_api.md REQ-WORKER-0146/0147).
- **REQ-CLIENT-0004 parity baseline:** Document current CLI capability set (gateway endpoints and commands) as the parity baseline for when the Web Console is implemented (REQ-CLIENT-0004).
  Optionally add a short "Parity baseline" subsection to `cynork_cli.md` or to this plan.
- **CyNode-Sse (step executor):** Implement step-executor binary (read job from `/job/job.json`, execute steps in order, write result to `/job/result.json`) and runner image per cynode_step_executor.md; wire Worker API and orchestrator to `step_executor_result`.
- **SBA timeout extension and remaining time:** Add when long-running SBA jobs are required; spec defines extension and context (cynode_sba.md TimeoutExtension, TimeRemaining).
- **Worker API cmd (production):** If E2E/compose does not start Worker API from a dedicated binary, add `worker_node/cmd/worker-api` (or similar) that mounts the same routes as BDD workerMux.

### 4.4 Ordering Suggestion

First: (1) response contract and (2) healthz so Worker API is spec-compliant and P2-10 can persist SBA results.
Then: (3) schema for LangGraph.
Then: (4)-(10) admin endpoints, payload, gateway, cynork (+ doc), SBA in-progress, PMA langchaingo, SBA spec/BDD refactor.
(11)-(16) schema sandbox registry verification, stdout/stderr verification, parity baseline doc, step executor, SBA timeout, Worker API cmd.

---

## 5. References

- Tech specs index: `docs/tech_specs/_main.md`
- MVP scope: `docs/mvp.md`; MVP plan: `docs/mvp_plan.md`
- Worker API: `docs/tech_specs/worker_api.md`
- Worker node payloads: `docs/tech_specs/worker_node_payloads.md`
- CyNode SBA: `docs/tech_specs/cynode_sba.md`
- CyNode Step Executor: `docs/tech_specs/cynode_step_executor.md`
- Go REST API standards: `docs/tech_specs/go_rest_api_standards.md`
- Postgres schema: `docs/tech_specs/postgres_schema.md`
- User API Gateway: `docs/tech_specs/user_api_gateway.md`
- Requirements: `docs/requirements/README.md`; SBA: `docs/requirements/sbagnt.md`
- Orchestrator: `docs/tech_specs/orchestrator.md`
- CLI management (healthz, status): `docs/tech_specs/cli_management_app_commands_core.md`
- MCP tool call auditing: `docs/tech_specs/mcp_tool_call_auditing.md`
- Local user accounts: `docs/tech_specs/local_user_accounts.md`
- OpenAI-compatible chat API: `docs/tech_specs/openai_compatible_chat_api.md`
- LangGraph MVP: `docs/tech_specs/langgraph_mvp.md`
- Meta and tooling: `meta.md`, `justfile`

**Source docs consolidated into this file:** Go code review (tech specs, requirements, MVP alignment); SBA contract design analysis; SBA code review; SBA initial build-out summary; additional issues (2026-02-26: local user admin endpoints, REQ-CLIENT-0004 parity baseline, cynork_cli.md chat endpoint, sandbox image registry schema, worker stdout/stderr verification).
Original dev_docs files can be deleted after this consolidation.
SBA build-out key content preserved: spec trace REQ-SBAGNT-0001; sbajob contract types and validation; `worker_node_sba.feature` scenarios (REQ-SBAGNT-0100/0101/0103, ProtocolVersioning, SchemaValidation, ResultContract); RegisterWorkerNodeSBASteps; `just ci` passes.
