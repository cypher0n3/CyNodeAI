# 2026-03-03 Rank-Ordered Remediation Plan

## Scope

This document defines a strict rank-ordered remediation plan for closing current
Go implementation drift against:

- `docs/dev_docs/2026-03-01_repo_state_and_execution_plan.md`
- `docs/mvp_plan.md`
- `docs/requirements/*` (normative)
- `docs/tech_specs/*`

## Baseline Evidence

- Latest local quality run: `just ci` currently fails in
  `orchestrator/internal/database` integration tests
  (`TestIntegration_GetEffectivePreferencesForTask_InvalidJSON`).
- `just test-bdd` is passing.
- Workflow start/resume/checkpoint/release API foundation exists, but plan/dependency
  start gates are not implemented yet.
- API egress and worker telemetry are implemented as minimal slices and remain
  materially below full MVP requirement coverage.

## Execution Status

Completed (as of 2026-03-03): P2-1, P0-1, P0-2, P0-3, P1-1, P1-2, P1-3, P1-4.
See `docs/dev_docs/2026-03-03_remediation_execution_report.md`.

## Rank-Ordered Remediation Items

Items below are ordered by dependency; execute in the order given in Dependency Order.

### P0-1: Complete API Egress Access-Control and Credential Model

Access-control and credential checks per API egress requirements.

#### Why This Rank (P0-1)

- Current implementation only enforces bearer and provider allowlist.
- High drift from `REQ-APIEGR-0111` through `REQ-APIEGR-0113`.

#### Traceability Links (P0-1)

- Requirements: `REQ-APIEGR-0110`, `REQ-APIEGR-0111`, `REQ-APIEGR-0112`,
  `REQ-APIEGR-0113`, `REQ-APIEGR-0119`, `REQ-APIEGR-0001`
- Specs: `api_egress_server.md`, `access_control.md`

#### Implementation Targets (P0-1)

- `orchestrator/cmd/api-egress/main.go`
- supporting orchestrator storage/policy integration surfaces

#### Remediation Scope (P0-1)

- Resolve subject identity from request context (not only static bearer match).
- Enforce provider + operation policy checks.
- Add credential authorization checks (`active`, owner/scope constraints).
- Preserve and extend audit event shape (allow/deny, task context, decision reason).

#### Acceptance Criteria (P0-1)

- Disallowed subject/provider/operation combinations return explicit denial.
- Unauthorized credential usage is blocked even when provider is allowed.
- Audit records contain subject, task, provider, operation, decision, and reason.

#### Validation Steps (P0-1)

- `just test-go`
- targeted API egress unit/integration tests
- BDD/e2e: `features/orchestrator/api_egress_call.feature`,
  `scripts/test_scripts/e2e_118_api_egress_call.py`

### P0-2: Implement Worker Telemetry API Contract and SQLite Telemetry Store

SQLite store, retention, and telemetry endpoints per worker telemetry spec.

#### Why This Rank (P0-2)

- Current node telemetry surface is stub-only (`node:info`, `node:stats`).
- Major unmet requirements across storage, retention, log query, and inventory.

#### Traceability Links (P0-2)

- Requirements: `REQ-WORKER-0210`, `REQ-WORKER-0211`, `REQ-WORKER-0212`,
  `REQ-WORKER-0220`, `REQ-WORKER-0221`, `REQ-WORKER-0222`,
  `REQ-WORKER-0230`, `REQ-WORKER-0231`, `REQ-WORKER-0232`,
  `REQ-WORKER-0233`, `REQ-WORKER-0234`, `REQ-WORKER-0240`,
  `REQ-WORKER-0241`, `REQ-WORKER-0242`, `REQ-WORKER-0243`
- Specs: `worker_telemetry_api.md`

#### Implementation Targets (P0-2)

- `worker_node/cmd/worker-api/main.go`
- new worker telemetry persistence and retention packages/files

#### Remediation Scope (P0-2)

- Add SQLite telemetry DB at `${storage.state_dir}/telemetry/telemetry.db`.
- Implement required schema and startup migrations.
- Implement retention enforcement and daily vacuum scheduling.
- Add telemetry endpoints: containers list/get and logs query with strict bounds.
- Enforce source filter and pagination semantics for log queries.

#### Acceptance Criteria (P0-2)

- Endpoint contract matches required request/response fields and limits.
- Log query enforces 1 MiB max response and truncation metadata.
- Telemetry rows preserve `task_id` and `job_id` where known.

#### Validation Steps (P0-2)

- `just test-go`
- worker telemetry unit/integration coverage
- BDD/e2e: `features/worker_node/worker_telemetry.feature`,
  `scripts/test_scripts/e2e_119_worker_telemetry.py`

### P0-3: Enforce Workflow Start Gate for Plan State and Dependencies

Plan and dependency gates before workflow start per langgraph_mvp.

#### Why This Rank (P0-3)

- Current workflow start checks task existence and lease only.
- Required gating for plan approval/dependencies is missing.

#### Traceability Links (P0-3)

- Requirements: `REQ-ORCHES-0152`, `REQ-ORCHES-0153`
- Specs: `langgraph_mvp.md` (`WorkflowStartGatePlanApproved`,
  `WorkflowPlanOrder`)

#### Implementation Targets (P0-3)

- `orchestrator/internal/handlers/workflow.go`
- database layer for plan/dependency lookup and gate evaluation

#### Remediation Scope (P0-3)

- Implement plan gate algorithm before lease acquisition.
- Deny start when plan not active unless explicit PMA handoff path applies.
- Enforce dependency-complete checks for `task_dependencies`.
- Return defined conflict/forbidden errors with clear reason strings.

#### Acceptance Criteria (P0-3)

- Planned task with unsatisfied dependency cannot start.
- Planned task with non-active plan cannot start unless PMA override semantics apply.
- Unplanned task path remains startable under existing lease semantics.

#### Validation Steps (P0-3)

- `just test-go`
- workflow handler + DB integration tests
- BDD/e2e: `features/orchestrator/workflow_start_resume_lease.feature`,
  `scripts/test_scripts/e2e_117_workflow_api.py`

### P1-1: Align PMA Startup and Readiness Gating With Full Inference-Path Rule

Readiness includes external inference path; see orchestrator_bootstrap.

#### Why This Rank (P1-1)

- Current readiness/startup depends on dispatchable nodes only.
- External-key inference path semantics are not modeled.

#### Traceability Links (P1-1)

- Requirements: `REQ-ORCHES-0150`, `REQ-ORCHES-0120`
- Specs: `orchestrator_bootstrap.md`

#### Implementation Targets (P1-1)

- `orchestrator/cmd/control-plane/main.go`
- relevant config and readiness helpers

#### Remediation Scope (P1-1)

- Expand inference-path check to include configured PMA external-provider path.
- Ensure PMA startup is triggered by either valid local inference path or valid
  external key path.
- Keep readiness reasons explicit and actionable.

#### Acceptance Criteria (P1-1)

- PMA does not start before first valid inference path.
- Ready state is consistent with PMA reachability and inference-path availability.

#### Validation Steps (P1-1)

- `just test-go`
- control-plane readiness/startup tests

### P1-2: Implement Task Attachment Ingestion and Propagation

Task create must persist and expose attachment references per user_api_gateway.

#### Why This Rank (P1-2)

- Contract accepts attachments, but task create path ignores them.
- This is direct drift against user gateway requirements.

#### Traceability Links (P1-2)

- Requirements: `REQ-ORCHES-0127`, `REQ-CLIENT-0157`
- Specs: `user_api_gateway.md`, task-create sections

#### Implementation Targets (P1-2)

- `orchestrator/internal/handlers/tasks.go`
- database/artifact persistence paths
- shared request/response contracts as needed

#### Remediation Scope (P1-2)

- Validate and persist attachment references on task creation.
- Connect attachment metadata to task/job execution context.
- Expose consistent retrieval semantics for API and CLI consumers.

#### Acceptance Criteria (P1-2)

- Create-task with attachments stores and returns usable attachment linkage.
- Downstream task execution can access declared attachments.

#### Validation Steps (P1-2)

- `just test-go`
- BDD/e2e attachment scenarios in orchestrator and cynork suites

### P1-3: Add Explicit "Already_running_running" Workflow Start Idempotency Response

Support idempotent start with explicit already_running where the contract allows.

#### Why This Rank (P1-3)

- Workflow spec allows 409 or explicit already-running response.
- Current behavior always returns 409 unless same holder + same lease ID.

#### Traceability Links (P1-3)

- Requirements: `REQ-ORCHES-0145`
- Specs: `langgraph_mvp.md` (`WorkflowStartResumeAPI`)

#### Implementation Targets (P1-3)

- `orchestrator/internal/handlers/workflow.go`
- `orchestrator/internal/database/workflow.go`

#### Remediation Scope (P1-3)

- Support idempotent start behavior with explicit `already_running` status where
  contract calls for it.
- Preserve single-active-workflow guarantee.

#### Acceptance Criteria (P1-3)

- Duplicate start semantics are deterministic and documented in response payloads.
- No second workflow instance starts for same task.

#### Validation Steps (P1-3)

- `just test-go`
- workflow handler tests for both 409 and already-running paths (as designed)

### P1-4: Implement Orchestrator-Side Worker Telemetry Pull With Timeout Tolerance

Orchestrator pulls node:info and node:stats with timeout-tolerant failure handling.

#### Why This Rank (P1-4)

- Worker telemetry endpoint foundation exists, but orchestrator consumption path is absent.

#### Traceability Links (P1-4)

- Requirements: `REQ-ORCHES-0141`, `REQ-ORCHES-0142`, `REQ-ORCHES-0143`
- Specs: `worker_telemetry_api.md` (`NodeTelemetryPull`)

#### Implementation Targets (P1-4)

- orchestrator runtime service/client layer for worker telemetry calls

#### Remediation Scope (P1-4)

- Add orchestrator client calls for `node:info` and `node:stats`.
- Enforce per-request timeout and tolerant failure handling.
- Ensure telemetry data remains non-authoritative for correctness decisions.

#### Acceptance Criteria (P1-4)

- Node telemetry pull failures degrade gracefully without destabilizing control-plane.
- Telemetry pull logic is test-covered for timeout/unavailable node cases.

#### Validation Steps (P1-4)

- `just test-go`
- targeted orchestrator telemetry pull tests

### P2-1: Restore and Lock Quality-Gate Reliability

Fix failing integration test and ensure `just ci` passes.

#### Why This Rank (P2-1)

- Current branch fails `just ci`, reducing confidence in all further remediation.

#### Traceability Links (P2-1)

- Plan alignment: `docs/mvp_plan.md` quality-gate expectations

#### Implementation Targets (P2-1)

- `orchestrator/internal/database/integration_test.go`
- any related DB helper code if required

#### Remediation Scope (P2-1)

- Fix invalid JSON preference integration test setup so it remains deterministic
  and schema-valid.
- Reconfirm package coverage thresholds remain above gate levels.

#### Acceptance Criteria (P2-1)

- `just ci` passes locally.
- No regression in BDD pass state.

#### Validation Steps (P2-1)

- `just ci`
- `just test-bdd`

## Dependency Order

Execute in this exact order:

1. `P2-1` (re-establish green baseline)
2. `P0-1`
3. `P0-2`
4. `P0-3`
5. `P1-1`
6. `P1-2`
7. `P1-3`
8. `P1-4`

## Notes on Deferred Scope

- This plan does not add new scope beyond current MVP and published requirements.
- Optional sanity-check enhancements in API Egress (`REQ-APIEGR-0121` onward)
  remain lower priority than closing mandatory MVP gaps above.
