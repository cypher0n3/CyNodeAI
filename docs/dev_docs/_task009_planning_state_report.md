# Task 9 Completion: `planning_state` on TaskBase

- [Sub-Issues Addressed](#sub-issues-addressed)
- [Code](#code)
- [Tests and Gates](#tests-and-gates)
- [Deviations](#deviations)

## Sub-Issues Addressed

**Date:** 2026-03-29.

1. **REQ-ORCHES-0176:** New tasks default to `planning_state=draft` (no immediate job execution on create).
2. **REQ-ORCHES-0177 / approval:** Execution is gated; `POST /v1/tasks/{id}/ready` merges pending execution metadata and dispatches jobs.
3. **REQ-ORCHES-0178:** Workflow start gate denies when `planning_state != ready` (`task not ready for workflow`).

## Code

- `orchestrator/internal/models/models.go`: `PlanningState` on `TaskBase`; constants `PlanningStateDraft` / `PlanningStateReady`.
- `orchestrator/internal/database/`: `CreateTask` sets draft; `UpdateTaskMetadata` / `UpdateTaskPlanningState`; DDL backfill `0004_task_planning_state_backfill.sql`.
- `orchestrator/internal/database/workflow_gate.go`: planning gate before plan/dependency checks.
- `orchestrator/internal/handlers/task_ready.go`, `tasks.go`: draft-only create; chat path promotes to ready; `PostTaskReady`.
- `go_shared_libs/contracts/userapi/userapi.go`: `planning_state` on task responses.
- `orchestrator/internal/mcptaskbridge/bridge.go`, `orchestrator/cmd/user-gateway/main.go`: wire response and route.
- `cynork`: `task ready`; `waitAndPrintTaskResult` calls `PostTaskReady` when draft.

## Tests and Gates

- Unit / handler tests: draft on create, gate, `PostTaskReady`, updated mock-DB flows.
- `scripts/test_scripts/e2e_0425_task_planning_state.py`: draft + workflow denial + ready + workflow allowed.
- `scripts/test_scripts/e2e_0420_task_create.py`: asserts `planning_state=draft` on create.
- `scripts/test_scripts/e2e_0500_workflow_api.py`: `task ready` before workflow start where success is expected.

Planned gates (run locally / CI):

- `just lint-go`
- `just test-go-cover`
- `just test-bdd` (orchestrator BDD suite)
- `just e2e --tags task,no_inference` (full stack; workflow case needs `WORKFLOW_RUNNER_BEARER_TOKEN`)

## Deviations

- **`e2e_0425` workflow leg** skips when `WORKFLOW_RUNNER_BEARER_TOKEN` is unset (control-plane workflow routes require bearer auth).
- Plan checklist referenced `TestPlanningState`; actual tests use names such as `TestCreateTask_ReturnsDraftPlanningState` and integration gate tests.
