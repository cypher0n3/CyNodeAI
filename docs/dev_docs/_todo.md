# TODO

## 1 Immediate (Security and Correctness)

**Status (2026-03-30):** Tracked items below were executed per [_plan_001_immediate.md](_plan_001_immediate.md); see [_plan_001_final_report.md](_plan_001_final_report.md) and per-task `_task*.md` reports in this directory.

These must be addressed before any production deployment:

1. **Fix authorization fail-open.**
   MCP gateway must reject no-token requests.
   Implement PM and PA allowlists.
   Fix system skill mutation guard. ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
2. **Replace all `!=` token comparisons** with `subtle.ConstantTimeCompare` across orchestrator, worker node, and cynork. ([Report 1](old/2026-03-29_review_report_1_orchestrator.md), [Report 2](old/2026-03-29_review_report_2_worker_node.md), [Report 4](old/2026-03-29_review_report_4_cynork.md))
3. **Add startup validation** rejecting insecure default secrets outside dev mode. ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
4. **Implement `planning_state`** on TaskBase (REQ-ORCHES-0176/0177/0178). ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
5. **Fix pod network isolation** for sandbox containers (REQ-WORKER-0174). ([Report 2](old/2026-03-29_review_report_2_worker_node.md))
6. **Fix container name matching** in `startOneManagedService`. ([Report 2](old/2026-03-29_review_report_2_worker_node.md))
7. **Add `Close()` to securestore** that zeros key material. ([Report 2](old/2026-03-29_review_report_2_worker_node.md))
8. **Implement PMA keep-warm** (REQ-PMAGNT-0129), **secret scan** (REQ-PMAGNT-0125), and **overwrite events** (REQ-PMAGNT-0124). ([Report 3](old/2026-03-29_review_report_3_agents.md))
9. **Fix SBA prompt construction** (REQ-SBAGNT-0113): add persona, skills, preferences; fix context ordering. ([Report 3](old/2026-03-29_review_report_3_agents.md))
10. **Fix PMA WriteTimeout** (120s < inference timeout 300s). ([Report 3](old/2026-03-29_review_report_3_agents.md))
11. **Fix cynork `runEnsureThread` data race.** ([Report 4](old/2026-03-29_review_report_4_cynork.md))
12. **Change `RunJobResponse.ExitCode`** from `int` to `*int`. ([Report 5](old/2026-03-29_review_report_5_shared_libs.md))
13. **Add GitHub Actions CI workflow.** ([Report 6](old/2026-03-29_review_report_6_testing.md))

Plan: [_plan_001_immediate.md](_plan_001_immediate.md)

## 2 Bugs

Plan: [_plan_002_bugs.md](_plan_002_bugs.md)

- **Bug 3 (`/auth login` thread UX):** Resolved per [_plan_002_bugs.md](_plan_002_bugs.md) Task 1 (logout clears thread id; ensure-thread landmarks).
  See [_plan_002_bugs_task1_report.md](_plan_002_bugs_task1_report.md).
  Product follow-up: in-TUI login transcript scope still TBD.
- **Bug 4 (slash/shell blocked while streaming + queue model):** Resolved per the same plan Task 2 (queue model, send-now, Ctrl+Q).
  See [_plan_002_bugs_task2_report.md](_plan_002_bugs_task2_report.md).
  Follow-up: full draft-queue UI and persistence per spec are not in scope of this plan.

## 3 Short-Term (High-Severity Issues)

Plan: [_plan_003_short_term.md](_plan_003_short_term.md)

**Status (2026-03-30):** Completed per the plan; see `task1_completion_report.md` through `task12_completion_report.md` and the final report in this directory.

Address within 1-2 sprints (tracking list retained for context):

1. **Add `http.MaxBytesReader` and `io.LimitReader`** across all modules for unbounded reads. ([Consolidated summary](old/2026-03-29_review_consolidated_summary.md))
2. **Add `context.Context`** to all functions performing network I/O without it. ([Report 2](old/2026-03-29_review_report_2_worker_node.md), [Report 3](old/2026-03-29_review_report_3_agents.md), [Report 4](old/2026-03-29_review_report_4_cynork.md))
3. **Replace `time.Sleep` with context-aware select** in retry loops. ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
4. **Move synchronous network I/O to async `tea.Cmd`** in TUI. ([Report 4](old/2026-03-29_review_report_4_cynork.md))
5. **Add auth checks to workflow handlers.** ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
6. **Encrypt `worker_api_bearer_token`** in DB. ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
7. **Add audit logging to internal orchestrator proxy.** ([Report 2](old/2026-03-29_review_report_2_worker_node.md))
8. **Close lifecycle response bodies** in SBA. ([Report 3](old/2026-03-29_review_report_3_agents.md))
9. **Set default HTTP client timeout** in cynork.
   Make `Client.Token`/`BaseURL` unexported with synchronized accessors. ([Report 4](old/2026-03-29_review_report_4_cynork.md))
10. **Extract SBA result status constants** and create shared status mapping. ([Report 5](old/2026-03-29_review_report_5_shared_libs.md))
11. **Add BDD feature files** for ACCESS, AGENTS, MCPGAT, MCPTOO domains. ([Report 6](old/2026-03-29_review_report_6_testing.md))
12. **Add E2E to CI** (`just e2e -tags no_inference`). ([Report 6](old/2026-03-29_review_report_6_testing.md))

## 4 Planned (Medium-Severity Improvements)

Plan: [_plan_004_planned.md](_plan_004_planned.md)

**Status (2026-03-30):** Completed; see [_plan_004_final_report.md](_plan_004_final_report.md).

Address within the next release cycle:

1. **Wrap database operations in transactions** (lease, checkpoint, task create, preference upsert). ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
2. **Split `Store` interface** into focused sub-interfaces. ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
3. **Add pagination** to unbounded queries. ([Report 1](old/2026-03-29_review_report_1_orchestrator.md), [Report 2](old/2026-03-29_review_report_2_worker_node.md))
4. **Batch N+1 queries.** ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
5. **Add AAD to GCM** and HKDF to PQ path in secure store. ([Report 2](old/2026-03-29_review_report_2_worker_node.md))
6. **Add GORM index tags** on telemetry query-hot columns. ([Report 2](old/2026-03-29_review_report_2_worker_node.md))
7. **Inject dependencies into PMA handler** (eliminate per-request `os.Getenv`/`NewMCPClient`). ([Report 3](old/2026-03-29_review_report_3_agents.md))
8. **Unify TUI dual scrollback model.** ([Report 4](old/2026-03-29_review_report_4_cynork.md))
9. **Add validation** to `workerapi.RunJobRequest` and `nodepayloads`. ([Report 5](old/2026-03-29_review_report_5_shared_libs.md))
10. **Merge BDD coverage into Go profiles** or document as separate metric. ([Report 6](old/2026-03-29_review_report_6_testing.md))

## 5 Per-Session-Binding PMA Provisioning

Plan: [_plan_005_pma_provisioning.md](_plan_005_pma_provisioning.md)

Normative refs (requirements):

- [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188) -- one managed `cynode-pma` per session binding.
- [REQ-ORCHES-0190](../requirements/orches.md#req-orches-0190) -- greedy provisioning on interactive login (do not wait for first chat).
- [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191) -- stale PMA teardown.
- [REQ-ORCHES-0162](../requirements/orches.md#req-orches-0162) -- route `cynodeai.pm` to the instance for the active binding.
- [REQ-ORCHES-0151](../requirements/orches.md#req-orches-0151) -- track each PMA by `service_id` and binding.
- [REQ-WORKER-0176](../requirements/worker.md#req-worker-0176) -- multiple concurrent PMA instances on the worker.

Related tech specs: [CYNAI.ORCHES.PmaInstancePerSessionBinding](../tech_specs/orchestrator_bootstrap.md#spec-cynai-orches-pmainstancepersessionbinding), [CYNAI.ORCHES.PmaGreedyProvisioningOnLogin](../tech_specs/orchestrator.md#spec-cynai-orches-pmagreedyprovisioningonlogin).

Note: Bootstrap/readiness vs per-binding instances -- see [REQ-ORCHES-0150](../requirements/orches.md#req-orches-0150) and **Align PMA startup** in [section 7](#7-longer-term-maintenance-and-debt). ([Report 1](old/2026-03-29_review_report_1_orchestrator.md), [Report 2](old/2026-03-29_review_report_2_worker_node.md))

### 5.1 Orchestrator -- Session Binding Model

Persist or derive a stable **session binding** key for interactive gateway sessions (user + session/thread lineage per tech spec) so each binding maps to at most one PMA `service_id`.

### 5.2 Orchestrator -- Greedy Provisioning ([0190](../requirements/orches.md#req-orches-0190))

On successful auth and when the user **establishes or resumes** an interactive session, **before** first chat completion: ensure desired managed-service state includes a PMA instance for that binding, push node configuration, and issue or refresh PMA MCP credentials with invocation class **user_gateway_session** (see [0186](../requirements/orches.md#req-orches-0186) / [0187](../requirements/orches.md#req-orches-0187)).

### 5.3 Orchestrator -- Routing ([0162](../requirements/orches.md#req-orches-0162))

Resolve `model=cynodeai.pm` chat to the worker-mediated endpoint for the PMA instance tied to the **active** session binding; track `service_id` + binding in control-plane state ([0151](../requirements/orches.md#req-orches-0151)).

### 5.4 Orchestrator -- Teardown ([0191](../requirements/orches.md#req-orches-0191))

On session end, logout, idle beyond policy, or credential expiry: update desired state to stop the instance, invalidate PMA MCP credentials for that binding, and avoid unbounded idle containers.

### 5.5 Worker Node ([0176](../requirements/worker.md#req-worker-0176), [0175](../requirements/worker.md#req-worker-0175))

Reconcile **multiple** `managed_services.services[]` PMA entries with distinct `service_id` values; independent health, restart, and proxy UDS per instance; fail closed if a binding's token cannot be resolved for proxy.

### 5.6 Clients (Cynork / Gateway Contract)

Ensure the gateway session (JWT / refresh / thread context) carries what the orchestrator needs to attribute requests to the correct binding; add or extend APIs only if the binding cannot be inferred from existing auth + thread IDs.

### 5.7 E2E and BDD Tests

BDD and/or E2E: second interactive session (or second user) yields a distinct PMA instance or `service_id`; logout or session end triggers teardown path; greedy path does not wait for first chat message.

## 6 Phase 2 MCP in the Loop

Plan: [_plan_006_phase2_mcp.md](_plan_006_phase2_mcp.md)

- **P2-06:** Go-native workflow runner (state machine) wired to MCP and Worker API.
  Only a stdlib reference runner (`minimal_runner.py`) exists to validate the API contract.
  Production-grade multi-step workflow orchestration (Load Task Context => Plan Steps => Dispatch Step => Collect Result => Verify => Finalize) is not implemented as a first-class component.
- **P2-08:** Full verification-loop with PMA tasking Project Analyst and writing findings back through the workflow is a persistence contract only; the multi-agent round-trip is not automated end-to-end.

## 7 Longer-Term (Maintenance and Debt)

Plan: [_plan_007_longer_term.md](_plan_007_longer_term.md)

1. **Adopt versioned migrations** to replace AutoMigrate. ([Report 1](old/2026-03-29_review_report_1_orchestrator.md), [Report 2](old/2026-03-29_review_report_2_worker_node.md))
2. **Align PMA startup** with worker-instruction model (REQ-ORCHES-0150). ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
3. **Implement continuous PMA monitoring** (REQ-ORCHES-0129). ([Report 1](old/2026-03-29_review_report_1_orchestrator.md))
4. **Extract centralized config package** for worker node. ([Report 2](old/2026-03-29_review_report_2_worker_node.md))
5. **Replace global mutable test hooks** with dependency injection across all modules. ([Consolidated summary](old/2026-03-29_review_consolidated_summary.md))
6. **Add load/performance testing** and chaos/failure E2E scenarios. ([Report 6](old/2026-03-29_review_report_6_testing.md))
