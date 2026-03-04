# 2026-03-03 Remediation Plan Execution Report

## P2-1: Restore and Lock Quality-Gate Reliability (Done)

- **Python lint:** Fixed E501 line length in `scripts/test_scripts/e2e_118_api_egress_call.py` (split long fail message).
- **Markdown lint:** Updated `docs/dev_docs/2026-03-03_rank_ordered_remediation_plan.md` so all headings are unique and H2+ have content (per project markdown rules).
- **Invalid JSON integration test:** Plan referenced `TestIntegration_GetEffectivePreferencesForTask_InvalidJSON`.
  The `preference_entries.value` column is JSONB; Postgres rejects invalid JSON on insert, so an integration test that inserts invalid JSON cannot be schema-valid.
  Removed that integration test and added a comment that the skip-on-unmarshal path is defensive and parser behavior is covered by `TestParsePreferenceValue_InvalidJSON` in `preferences_test.go`.
- **Validation:** `just ci` and `just test-bdd` pass.

## P0-1: Complete API Egress Access-Control and Credential Model (Done)

- **Models:** Added `AccessControlRule`, `AccessControlAuditLog`, `ApiCredential`; added to migrate and table-name test.
- **Database:** New `access_control.go` with `ListAccessControlRulesForApiCall`, `CreateAccessControlAuditLog`, `HasActiveApiCredentialForUserAndProvider`; Store and MockDB updated.
- **API Egress:** When `API_EGRESS_DSN` is set: resolve subject from task_id, evaluate provider+operation policy, require active credential, audit to `access_control_audit_log`.
  Deny and no-credential return 403 with reason.
  When DSN unset, bearer + allowlist unchanged.
- **Tests:** api-egress handler tests with mock store; testcontainers test for access control and API credential.
- **Validation:** `just ci` passes.

## P0-2: Worker Telemetry API and SQLite Store (Done)

- **SQLite store:** `worker_node/internal/telemetry/` with Store (Open, Close, migrations), EnforceRetention (7d logs, 30d events/inventory), Vacuum; ListContainers, GetContainer, QueryLogs (1 MiB cap, truncated metadata); InsertTestContainer for tests.
- **Worker-api:** `WORKER_API_STATE_DIR`; open telemetry store on startup (graceful degradation if unavailable); `doRetentionAndVacuumOnce` + `runRetentionAndVacuum(ctx, …)` with configurable tickers; routes `GET /v1/worker/telemetry/containers`, `GET /v1/worker/telemetry/containers/`, `GET /v1/worker/telemetry/logs` when store non-nil; bearer auth via telemetryAuth.
- **Tests:** Telemetry package tests (Open/Close, migrations, ListContainers/GetContainer/QueryLogs, retention, vacuum, bytes truncation); worker-api tests with store (list/get container, logs, method not allowed, closed-store error paths); runRetentionAndVacuum ticker branches; coverage >=90%.
- **Lint:** errcheck (Close, rows.Close, Chmod), gocognit (listContainersQuery, scanContainerRow, buildLogsQuery, scanLogEventRow), goconst (limitedBy* in logs.go), dupl (split into separate Test functions + telemetryGET helper).
- **Validation:** `just ci` passes.

## P0-3: Workflow Start Gate for Plan State and Dependencies (Done)

- **Models:** Task.PlanID, Task.Closed; ProjectPlan (id, project_id, state, archived); TaskDependency (task_id, depends_on_task_id).
  Migrate: ProjectPlan, TaskDependency added to AutoMigrate.
- **DB:** `workflow_gate.go`: EvaluateWorkflowStartGate(ctx, task, requestedByPMA) per langgraph_mvp.md.
  No plan_id -> allow; plan archived -> deny "plan is archived"; plan state != active and !requestedByPMA -> deny "plan not active".
  PMA handoff skips state check.
  Dependency check: any depends_on_task_id with status != completed -> deny "dependencies not satisfied".
  getPlanByID, listTaskDependencyIDs.
- **Handler:** Start: after GetTaskByID, call EvaluateWorkflowStartGate(task, requestedByPMA).
  requestedByPMA from header `X-CyNode-Workflow-Requested-By: pma`.
  If denyReason non-empty -> 409 Conflict with reason; if err -> 500.
- **Tests:** Handler: gate denied (mock deny reason), gate error (mock err -> 500).
  Integration: no plan (allow), plan not found (deny), draft (deny), PMA handoff draft (allow), archived (deny), active (allow), dependencies not satisfied, dependency task not found.
- **Validation:** `just test-go-cover` passes for database and handlers (>=90%).
  mcp-gateway TestRun_WithTestStore can be flaky in full suite; passes in isolation.

## Session Follow-On (2026-03-03)

This section records fixes and P1-1 completion from a follow-on session.

### P2-1 (Reconfirmed)

- **mcp-gateway:** CI was failing on `TestRun_WithTestStore` (run returns context.Canceled after intentional cancel).
  Fixed by accepting `context.Canceled` in tests that cancel the context (`TestRun_WithTestDatabaseOpen`, `TestRun_WithTestStore`).
- **api-egress:** `TestRun_CancelledContext` updated to accept `context.Canceled` when context is pre-cancelled.
- **Validation:** `just ci` passes.

### P0-1 (Augmented)

- **Audit record:** Access control audit log now includes `subject_id` (resolved user from task).
  `evaluateWithStore` returns (subjectID, decision, reason); `auditLog` takes subjectID and sets `rec.SubjectID`.
- **Lint:** gocritic nilValReturn fix: return `nil` instead of `subjectID` when subjectID is nil.

### P1-1: PMA Startup and Readiness With Full Inference-Path Rule (Done)

- **Spec:** orchestrator_bootstrap.md defines inference path as (1) dispatchable node that reported ready, or (2) LLM API key configured for PMA via API Egress.
- **Database:** `HasAnyActiveApiCredential(ctx) (bool, error)` in `access_control.go`; added to Store and MockDB (`HasAnyActiveApiCredentialResult`).
- **Control-plane:** `inferencePathAvailable(ctx, store)` returns true when `ListDispatchableNodes` has entries or `HasAnyActiveApiCredential` is true.
  `waitForInferencePath` and `readyzHandler` use `inferencePathAvailable` so external provider keys count as an inference path.
- **Tests:** testcontainers test asserts HasAnyActiveApiCredential before/after cred and cancelled-context error path; integration test for cancelled context; control-plane tests unchanged (mock defaults to no external cred).
- **Coverage:** database package coverage restored to >=90% via HasAnyActiveApiCredential test coverage; test complexity kept under gocognit limit via helper `tcAssertHasAnyActiveApiCredential`.
- **Validation:** `just ci` passes.

### P1-2: Task Attachment Ingestion and Propagation (Done)

- **DB:** `artifacts.go`: `CreateTaskArtifact`, `ListArtifactPathsByTaskID`; Store and mock.
- **Contract:** `userapi.TaskResponse.Attachments []string`.
- **Handler:** `persistTaskAttachments` (path non-empty, max 2048); create/Get/List return attachment paths.
- **Tests:** handlers mock test with attachments; testcontainers artifact CRUD and duplicate-path error; `tcAssertTaskArtifacts` helper for gocognit.
- **cynork:** task list rangeValCopy fix (index loop).
- **Validation:** `just ci` passes.

### P1-3: Already_Running Idempotency for Workflow Start (Done)

- **Handler:** In `startAcquireAndRespond`, before `AcquireTaskWorkflowLease`: if `GetTaskWorkflowLease` returns existing lease with same holder, same leaseID, and not expired, return 200 with `Status: "already_running"` (REQ-ORCHES-0145).
- **Validation:** Covered by existing workflow handler tests.

### P1-4: Orchestrator Worker Telemetry Pull With Timeout (Done)

- **Package:** `orchestrator/internal/nodetelemetry`: `Client`, `PullNodeInfo`, `PullNodeStats`; per-request timeout; `http.NoBody`; body cap 2 MiB; errors wrapped.
- **Control-plane:** `runTelemetryPullLoop(ctx, store, logger)` started with PMA; 60s ticker; `ListDispatchableNodes` (5s timeout); per-node pull with timeout; `pullNodeTelemetry` helper for gocognit.
- **Tests:** nodetelemetry: success, timeout, 503, context canceled, nil HTTPClient, baseURL trailing slash; control-plane: `TestPullNodeTelemetry`, `TestRunTelemetryPullLoop_ListFails`, `TestRunTelemetryPullLoop_OneTick`.
- **Lint:** gocritic httpNoBody; gocognit reduced via `pullNodeTelemetry`; dupl nolint on test server setup.
- **Coverage:** nodetelemetry and control-plane >=90%.
- **Validation:** `just ci` passes.

## BDD and E2E Validation (Re-Validated 2026-03-04)

Feature files and Python e2e tests aligned with the plan:

- **Item:** P0-1 API Egress
  - feature file: `features/orchestrator/api_egress_call.feature`
  - e2e script: `e2e_118_api_egress_call.py`
  - notes: Allowed 501, disallowed 403, missing bearer 401
- **Item:** P0-2 Worker telemetry
  - feature file: `features/worker_node/worker_telemetry.feature`
  - e2e script: `e2e_119_worker_telemetry.py`
  - notes: node:info, node:stats, 401; **added** containers list and logs scenarios + e2e tests
- **Item:** P0-3 Workflow
  - feature file: `features/orchestrator/workflow_start_resume_lease.feature`
  - e2e script: `e2e_117_workflow_api.py`
  - notes: Start, duplicate 409, resume, release; **added** same-holder already_running scenario + e2e (idempotency_key=lease_id)
- **Item:** P1-1 Readiness
  - feature file: `features/orchestrator/orchestrator_startup.feature`
  - e2e script: -
  - notes: No inference path -> not ready; inference path -> ready
- **Item:** P1-2 Attachments
  - feature file: `features/cynork/cynork_tasks.feature` (Create task with attachment paths)
  - e2e script: **added** `e2e_050_task_create.py` test_task_create_with_attachments
  - notes: BDD existed; e2e create-with-attachments and get attachments added
- **Item:** P1-3 Already_running
  - feature file: (above workflow feature)
  - e2e script: (above e2e_117)
  - notes: Scenario and step `workflow start response has status "already_running"`; compound step sends idempotency_key
- **Item:** P1-4 Telemetry pull
  - feature file: -
  - e2e script: -
  - notes: Internal control-plane loop; no user-facing BDD/e2e required per plan

- **BDD:** `just test-bdd` passes (orchestrator, worker_node, cynork, agents).
- **CI:** `just ci` passes (Go lint/test/cover, Python lint, BDD).
