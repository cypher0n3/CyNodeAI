# 2026-03-01 Repo State Review and Execution Plan

- [Scope and Inputs](#scope-and-inputs)
- [Current State Summary](#current-state-summary)
- [Go/spec Alignment Snapshot](#gospec-alignment-snapshot)
  - [Areas With Good Alignment](#areas-with-good-alignment)
  - [Areas With Partial Alignment or Drift](#areas-with-partial-alignment-or-drift)
  - [Large Planned Gaps (Expected by Phase, but Now Spec-Visible)](#large-planned-gaps-expected-by-phase-but-now-spec-visible)
- [What to Focus on Next](#what-to-focus-on-next)
- [Detailed Step-By-Step Execution Plan](#detailed-step-by-step-execution-plan)
  - [Step 0 - Establish Execution Board and Traceability](#step-0---establish-execution-board-and-traceability)
  - [Step 1 - Fix Baseline Quality Gates First](#step-1---fix-baseline-quality-gates-first)
  - [Step 2 - Reconcile Completed-Phase Claims vs Actual Behavior](#step-2---reconcile-completed-phase-claims-vs-actual-behavior)
  - [Step 3 - Close `P1/P1.7` Requirement Drift](#step-3---close-p1p17-requirement-drift)
  - [Step 4 - Complete Phase 2 MCP Core Slice](#step-4---complete-phase-2-mcp-core-slice)
  - [Step 5 - PMA Implementation Alignment Increment](#step-5---pma-implementation-alignment-increment)
  - [Step 6 - Skills Vertical Slice (Gateway + CLI + MCP)](#step-6---skills-vertical-slice-gateway--cli--mcp)
  - [Step 7 - Workflow Runner and Lease Implementation (Phase 2 Continuation)](#step-7---workflow-runner-and-lease-implementation-phase-2-continuation)
  - [Step 8 - API Egress and Telemetry Foundations](#step-8---api-egress-and-telemetry-foundations)
  - [Step 9 - Final Hardening and Acceptance](#step-9---final-hardening-and-acceptance)
- [Risks and Mitigations](#risks-and-mitigations)
- [Suggested Immediate Next Sprint Target](#suggested-immediate-next-sprint-target)
  - [Execution Tracker (This Document)](#execution-tracker-this-document)

## Scope and Inputs

Date reviewed: 2026-03-01 17:33:45 EST.

This review used the current repository `main` state with a clean working tree.
The following planning and spec documents were reviewed as primary inputs:

- `docs/mvp_plan.md`
- `docs/mvp.md`
- `docs/requirements/orches.md`
- `docs/requirements/worker.md`
- `docs/requirements/skills.md`
- `docs/tech_specs/_main.md`
- `docs/tech_specs/user_api_gateway.md`
- `docs/tech_specs/cynork_cli.md`
- `docs/tech_specs/mcp_tool_catalog.md`
- `docs/tech_specs/worker_node_payloads.md`
- `docs/tech_specs/skills_storage_and_inference.md`
- `docs/tech_specs/project_manager_agent.md`

Validation commands run:

- `just --list`
- `just lint-go`
- `just test-go`
- `just test-bdd`

## Current State Summary

High-confidence status (updated after Steps 0--3 execution):

- BDD suites are passing (`agents`, `orchestrator`, `worker_node`, `cynork`).
- Baseline was restored: `just lint-go`, `just test-go`, and `just test-bdd` pass; coverage gates at >=90% for targeted packages.
- `just ci` is green (Go lint/test/coverage/vulncheck, Python lint, markdownlint, doc-link validation, BDD).

Original snapshot (for context): the repository was not green on local quality gates; `just test-go` had failed coverage (e.g. `worker_node/cmd/node-manager` at 83.0%).
Steps 1--3 addressed baseline and requirement drift; CI is now green.

## Go/spec Alignment Snapshot

This section summarizes implementation match quality against reviewed specs and requirements.

### Areas With Good Alignment

- Node registration, capability ingestion, config payload, ULID `config_version`, and config ack flow are implemented and tested in orchestrator/worker paths.
- Worker API health endpoints (`/healthz`, `/readyz`), request-size 413 behavior, and bounded output handling are implemented.
- OpenAI-compatible chat routes are present (`GET /v1/models`, `POST /v1/chat/completions`) with PMA routing split.
- SBA runner contract and worker/orchestrator integration are implemented with BDD and unit coverage.

### Areas With Partial Alignment or Drift

- PMA startup sequencing appears to start PMA eagerly when enabled.
  This is weaker than the newer requirement language that PMA startup should be tied to first available inference path (`REQ-ORCHES-0150`).
- OpenAI chat reliability requirements for bounded wait and transient retry (`REQ-ORCHES-0131`, `REQ-ORCHES-0132`) are not clearly implemented in handler logic.
- Task create semantics in code are narrower than updated gateway/spec language.
  Current API contract does not yet cover optional task name and attachment ingestion contract in the request model.
- MCP gateway currently routes only `db.preference.get`, `db.preference.list`, and `db.preference.effective`.
  Other listed tool contracts currently return `501`.

### Large Planned Gaps (Expected by Phase, but Now Spec-Visible)

- Skills domain (`REQ-SKILLS-*`) is largely unimplemented server-side.
  CLI has stub behavior for skills and gateway lacks concrete skills endpoints.
- API Egress service is health-only and does not yet implement policy, provider operation routing, or auditing behavior.
- Worker Telemetry API is not yet implemented in worker node Go code.
- LangGraph workflow runtime integration is not yet implemented, beyond schema/model groundwork.
- PMA implementation is still simple direct inference HTTP flow and not yet aligned with langchaingo + MCP tool orchestration expectations in updated PM spec.

## What to Focus on Next

Recommended focus order:

1. Restore and lock a green baseline for lint and test gates.
2. Close high-impact requirement drift inside already-claimed completed phases.
3. Complete Phase 2 core runtime loop before opening broad Phase 4 surfaces.
4. Implement a vertical slice for skills, because requirements and specs are now explicit and cross-cutting.

Rationale:

- A non-green baseline makes every future change risky and hard to evaluate.
- Some phase-labeled "done" areas have requirement drift that should be corrected before additional expansion.
- Phase 2 completion unlocks a coherent architecture path for PMA/SBA/tooling.
- Skills and gateway/API contracts now have enough specificity to execute in bounded increments.

## Detailed Step-By-Step Execution Plan

The steps below are ordered to reduce execution risk and preserve delivery continuity.

### Step 0 - Establish Execution Board and Traceability

1. Create an execution tracker with one item per step in this plan.
2. Add requirement and spec IDs to each tracker item before implementation starts.
3. Define "done" for each item as code, tests, docs update, and passing `just` target evidence.
4. Store interim notes and command outputs in `tmp/` and final execution notes in `docs/dev_docs/`.

Exit criteria:

- Every work item has trace links to requirements and tech specs.
- Every work item has explicit validation commands.

### Step 1 - Fix Baseline Quality Gates First

1. Reproduce `just lint-go` failure and confirm whether tool-path resolution is the cause in recipe context.
2. Remediate `staticcheck` discoverability in the developer workflow so `just lint-go` is reliable on a clean machine.
3. Increase `worker_node/cmd/node-manager` coverage from 83% to >=90%.
4. Focus added tests on startup/config/inference branches currently under-covered.
5. Re-run `just lint-go`, `just test-go`, and `just test-bdd`.

Exit criteria:

- `just lint-go` passes.
- `just test-go` passes with all package thresholds.
- `just test-bdd` remains passing.

### Step 2 - Reconcile Completed-Phase Claims vs Actual Behavior

1. Audit "completed" claims in `docs/mvp_plan.md` against current behavior and tests.
2. Correct any stale claims if behavior is not yet delivered.
3. Add explicit "implemented", "partial", and "pending" markers for each P1/P1.5/P1.7/P2 item.
4. Add a short "known drifts" subsection that points to remediation tickets.

Exit criteria:

- `docs/mvp_plan.md` status language is evidence-based and not optimistic.

### Step 3 - Close `P1/P1.7` Requirement Drift

1. Implement PMA startup gating aligned to `REQ-ORCHES-0150` so PMA starts only when first inference path is available.
2. Keep readiness logic consistent with PMA lifecycle and not-ready reasons.
3. Implement chat completion reliability controls for bounded wait and transient retry behavior (`REQ-ORCHES-0131`, `REQ-ORCHES-0132`).
4. Expand task-create contract for updated semantics (task name normalization and attachment contract acceptance path) while preserving current client compatibility.
5. Add unit and BDD tests for each behavior change.

Exit criteria:

- Updated tests prove PMA startup gating and chat reliability behavior.
- Task-create contract changes are covered and backward-compatible for existing clients.

### Step 4 - Complete Phase 2 MCP Core Slice

1. Extend MCP gateway beyond current preference read/effective path.
2. Add remaining preference CRUD tools (`create`, `update`, `delete`) with typed schema checks and audit.
3. Add one artifact tool path and one task/job typed DB path to reduce 501-only surface.
4. Keep strict scoped-ID validation and audit-on-every-call behavior.
5. Add testcontainers-backed integration tests for new tool routes.

Exit criteria:

- MCP gateway supports a minimally useful multi-tool loop beyond current partial implementation.
- New MCP routes are fully tested and audited.

### Step 5 - PMA Implementation Alignment Increment

1. Define a narrow PMA refactor target for MVP: move one request path to langchaingo + MCP tool wrapper path.
2. Keep direct-inference fallback only where explicitly documented.
3. Add context composition tests for baseline + project + task + additional context ordering.
4. Add compatibility tests for orchestrator handoff request format.

Exit criteria:

- At least one production PMA path uses langchaingo + MCP tool calls.
- Context composition remains deterministic and test-covered.

### Step 6 - Skills Vertical Slice (Gateway + CLI + MCP)

1. Implement minimal server-side skill store and registry abstraction with stable IDs.
2. Add gateway endpoints for `skills list/get/load` first, then `update/delete`.
3. Add malicious-pattern scan on load/update with deterministic rejection feedback.
4. Upgrade `cynork skills` from stub to real payload handling against gateway.
5. Add MCP skills tools (`skills.create/list/get/update/delete`) mapped through gateway enforcement and auditing.
6. Ensure default CyNodeAI skill inclusion behavior is explicit and testable.

Exit criteria:

- Skills are no longer stubbed in CLI/gateway.
- Server enforces scanning and returns category + triggering text on rejection.
- MCP skills tool path works end-to-end with audit records.

### Step 7 - Workflow Runner and Lease Implementation (Phase 2 Continuation)

1. Add orchestrator-to-workflow start/resume API and enforce single active workflow per task.
2. Implement lease acquire/release/expiry semantics in orchestrator runtime paths.
3. Wire checkpoint persistence and resume flow.
4. Add failure-mode tests for duplicate start and lease contention.

Exit criteria:

- Workflow start/resume and lease behavior meet requirement-level contracts.

### Step 8 - API Egress and Telemetry Foundations

1. Implement API Egress minimal callable endpoint with authz checks and audit record creation.
2. Add provider-operation allowlist enforcement and clear error model.
3. Implement first Worker Telemetry API slice on node side with bounded query behavior.
4. Add orchestrator pull path with per-request timeout and tolerant failure handling.

Exit criteria:

- API Egress and telemetry are no longer health-only/absent stubs.
- Core enforcement and audit controls are present and tested.

### Step 9 - Final Hardening and Acceptance

1. Run `just ci` and `just docs-check`.
2. Run `just e2e --stop-on-success`.
3. Produce a final execution report in `docs/dev_docs/` summarizing delivered items vs this plan.
4. Update `docs/mvp_plan.md` remediation status and next-work section with evidence links.

Exit criteria:

- CI, docs checks, and E2E all pass.
- Status docs are updated to match observed implementation.

## Risks and Mitigations

- Risk: Expanding scope before baseline is green will hide regressions.
  Mitigation: enforce Step 1 as a hard gate.
- Risk: PMA and MCP refactors may break chat behavior.
  Mitigation: add contract tests around OpenAI-compatible endpoints before refactor.
- Risk: Skills implementation can sprawl across gateway, MCP, CLI, and storage.
  Mitigation: deliver in vertical slices (`list/get/load` first) with strict acceptance criteria.

## Suggested Immediate Next Sprint Target

Sprint target: complete Steps 1 through 3 only.

This keeps scope focused on baseline stability and high-impact drift fixes before expanding platform surface area.

### Execution Tracker (This Document)

- **Step:** 0
  - description: Establish execution board and traceability
  - requirement ids: (process)
  - spec ids: this doc
  - validation: N/A
  - status: Done
- **Step:** 1
  - description: Fix baseline quality gates
  - requirement ids: (repo tooling)
  - spec ids: justfile
  - validation: `just lint-go`, `just test-go`, `just test-bdd`
  - status: Done
- **Step:** 2
  - description: Reconcile completed-phase claims
  - requirement ids: (doc accuracy)
  - spec ids: `docs/mvp_plan.md`
  - validation: `just docs-check`
  - status: Done
- **Step:** 3
  - description: Close P1/P1.7 requirement drift
  - requirement ids: REQ-ORCHES-0150, REQ-ORCHES-0131, REQ-ORCHES-0132
  - spec ids: `orchestrator_bootstrap.md`, `openai_compatible_chat_api.md`, `user_api_gateway.md`
  - validation: unit + BDD for gating, chat, task-create
  - status: Done (coverage >=90%; `just ci` green)
- **Step:** 4
  - description: Complete Phase 2 MCP core slice
  - requirement ids: REQ-MCPGAT-0001, REQ-MCPGAT-0002, REQ-MCPTOO-0109, REQ-MCPTOO-0110
  - spec ids: `mcp_gateway_enforcement.md`, `mcp_tool_catalog.md`, `mcp_tool_call_auditing.md`, `user_preferences.md`
  - validation: testcontainers + `just test-go`; `just ci` (coverage ≥90%)
  - status: Done (implementation complete; preference CRUD, db.task.get, db.job.get, artifact.get; audit on every call; `just ci` green with coverage ≥90% for orchestrator/internal/database and cmd/mcp-gateway)
- **Step:** 5
  - description: PMA implementation alignment
  - requirement ids: REQ-PMAGNT-0001, REQ-PMAGNT-0100, REQ-PMAGNT-0101
  - spec ids: `cynode_pma.md`, `project_manager_agent.md`
  - validation: context composition tests, handoff tests
  - status: Not started
- **Step:** 6
  - description: Skills vertical slice
  - requirement ids: REQ-SKILLS-0001, REQ-SKILLS-0100--0108, REQ-SKILLS-0110--0117
  - spec ids: `skills_storage_and_inference.md`, `cynork_cli.md`, `mcp_tool_catalog.md`
  - validation: gateway + CLI + MCP E2E, audit records
  - status: Not started
- **Step:** 7
  - description: Workflow runner and lease
  - requirement ids: REQ-ORCHES-0144, REQ-ORCHES-0145, REQ-ORCHES-0146, REQ-ORCHES-0147
  - spec ids: `langgraph_mvp.md`, `orchestrator.md`, `postgres_schema.md`
  - validation: start/resume API tests, lease tests
  - status: Not started
- **Step:** 8
  - description: API Egress and telemetry
  - requirement ids: REQ-APIEGR-0001, REQ-APIEGR-0110--0119, REQ-ORCHES-0141--0143, REQ-WORKER-0200--0243
  - spec ids: `api_egress_server.md`, `access_control.md`, `worker_telemetry_api.md`
  - validation: authz + audit tests, telemetry pull tests
  - status: Not started
- **Step:** 9
  - description: Final hardening and acceptance
  - requirement ids: (CI/doc/E2E)
  - spec ids: `docs/mvp_plan.md`
  - validation: `just ci`, `just docs-check`, `just e2e --stop-on-success`
  - status: Not started

Fill **Status** per step (e.g. not started / in progress / done).
