# Plan 004 -- Task 1 Completion Report

<!-- Date: 2026-03-30 -->

## Summary

Implemented transactional boundaries for orchestrator workflow lease + checkpoint,
task creation (name uniqueness loop), preference create/update, and system setting
create/update.
Added `database.Store.WithTx` for multi-step handler flows.
Adjusted `WorkflowHandler.startAcquireAndRespond` so idempotent workflow starts
return `already_running` when a lease row already exists and `AcquireTaskWorkflowLease`
returns idempotent success (fixes `e2e_0500` same-holder test).

## Code Locations (Actual Paths)

- `orchestrator/internal/database/database.go` -- `WithTx`, `Store` extension
- `orchestrator/internal/database/workflow.go` -- `AcquireTaskWorkflowLease`,
  `UpsertWorkflowCheckpoint` in SQL transactions
- `orchestrator/internal/database/tasks.go` -- `CreateTask` in transaction
- `orchestrator/internal/database/preferences.go` -- `CreatePreference`,
  `UpdatePreference` in transactions
- `orchestrator/internal/database/system_settings.go` -- `CreateSystemSetting`,
  `UpdateSystemSetting` in transactions
- `orchestrator/internal/handlers/workflow.go` -- `WithTx` around get + acquire;
  `hadLeaseRow` + status `already_running` for idempotent re-start
- `orchestrator/internal/testutil/mock_db.go` -- `WithTx`, `GetTaskWorkflowLeaseErr`
- `orchestrator/internal/handlers/workflow_test.go` -- idempotent + GetLease error tests
- `orchestrator/internal/handlers/transaction_concurrency_test.go` -- integration
  tests (skip without `POSTGRES_TEST_DSN`)

## Validation

- `go vet ./orchestrator/...`, `staticcheck ./orchestrator/...`: pass
- `go test ./orchestrator/...`: pass; `internal/handlers` coverage ~90.3% statements
- `just lint-go`: fails on pre-existing files over 1000 lines (not introduced here)
- `just e2e --tags task,no_inference`: full run should be re-executed after the
  idempotency fix; an earlier run showed
  `test_workflow_start_same_holder_returns_200_already_running` FAIL before the
  `hadLeaseRow` status fix

## Risks / Follow-Up

- Nested GORM transactions (`WithTx` + `AcquireTaskWorkflowLease` internal
  transaction): savepoints; monitor for deadlocks under load.
- Re-run E2E when the dev stack is stable to confirm workflow idempotency and
  task lifecycle.
