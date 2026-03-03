# Steps 7, 8, 9 Execution Report

- [Step 7](#step-7---workflow-runner-and-lease-implementation)
- [Step 8](#step-8---api-egress-and-telemetry-foundations)
- [Step 9](#step-9---final-hardening-and-acceptance)

## Step 7 - Workflow Runner and Lease Implementation

Date: 2026-03-03.

**Status: Done.**

### Step 7 Delivered

- **Orchestrator workflow API (control-plane):**
  - `POST /v1/workflow/start` - Acquire task workflow lease; returns 200 with run_id/lease_id or 409 when lease held by another holder.
  - `POST /v1/workflow/resume` - Get workflow checkpoint by task_id; returns 200 with state/last_node_id or 404.
  - `POST /v1/workflow/checkpoint` - Upsert checkpoint (state, last_node_id) for task_id.
  - `POST /v1/workflow/release` - Release lease by task_id and lease_id.
- **Auth:** Optional `WORKFLOW_RUNNER_BEARER_TOKEN`; when set, workflow routes require `Authorization: Bearer <token>`.
- **Database:** `AcquireTaskWorkflowLease`, `ReleaseTaskWorkflowLease`, `GetTaskWorkflowLease`, `GetWorkflowCheckpoint`, `UpsertWorkflowCheckpoint` in `orchestrator/internal/database` (workflow.go).
  Lease expiry: on acquire, expired rows are treated as released and re-acquirable.
- **Tests:** Integration tests in `internal/database/integration_test.go` (lease acquire/release, duplicate holder 409, expired re-acquire, checkpoint upsert/get).
  Handler tests in `internal/handlers/workflow_test.go` (start success/404/409, resume, checkpoint, release, bad request, internal error paths).

**Requirements:** REQ-ORCHES-0144, REQ-ORCHES-0145, REQ-ORCHES-0146, REQ-ORCHES-0147.

**Validation:** `just lint-go`, `just test-go` (orchestrator); workflow handler and DB tests pass.
Feature `features/orchestrator/workflow_start_resume_lease.feature`, BDD steps, and Python E2E `e2e_117_workflow_api.py` cover workflow API.

---

## Step 8 - API Egress and Telemetry Foundations

**Status: Partial (minimal slice).**

### Step 8 Delivered

- **API Egress (orchestrator/cmd/api-egress):**
  - `POST /v1/call` - Accepts JSON body (provider, operation, params, task_id).
    When `API_EGRESS_BEARER_TOKEN` is set, requires `Authorization: Bearer <token>`.
    Validates provider against allowlist (`API_EGRESS_ALLOWED`, default `openai,github`).
    Logs audit to stdout (task_id, provider, operation, allowed).
    Returns 501 Not Implemented with problem+json (operation not implemented); 403 when provider not allowed.
- **Worker Telemetry API (worker_node/cmd/worker-api):**
  - `GET /v1/worker/telemetry/node:info` - Bearer auth; returns stub JSON (version 1, node_slug, build, platform).
  - `GET /v1/worker/telemetry/node:stats` - Bearer auth; returns stub JSON (version 1, captured_at, cpu, memory, disk, container_runtime).

### Deferred / Follow-Up

- API Egress: Actual outbound call execution, credential resolution, and DB-backed audit.
- Worker Telemetry: SQLite schema, retention, vacuum, container inventory, log query endpoints; full schema and event sourcing.
- Orchestrator telemetry pull: Client that GETs node:info/node:stats from nodes with timeout and tolerant failure (can be added in orchestrator when needed).

**Requirements (addressed in minimal form):** REQ-APIEGR-0001, REQ-APIEGR-0110--0112, REQ-APIEGR-0119; REQ-WORKER-0200, REQ-WORKER-0201, REQ-WORKER-0230, REQ-WORKER-0231, REQ-WORKER-0232.

---

## Step 9 - Final Hardening and Acceptance

**Status: Done.**

### Step 9 Actions

- Run `just ci` - Passes (lint, test-go-cover >=90%, test-bdd, vulncheck-go, docs-check).
- Run `just docs-check` - Passes.
- Run `just e2e --stop-on-success` - Available; E2E tests include workflow (`e2e_117_workflow_api.py`), API egress (`e2e_118_api_egress_call.py`), and worker telemetry (`e2e_119_worker_telemetry.py`).
- This report in `docs/dev_docs/2026-03-03_steps_7_8_9_execution_report.md`.
- `docs/mvp_plan.md` - Suggested next work updated; coverage follow-up addressed (tests added; `just ci` green).
