# Task 2 completion report — context propagation

## Summary

Propagated `context.Context` through cynork gateway HTTP helpers and non-streaming APIs, worker node-manager pulls and PMA UDS health polling, and SBA `applyUnifiedDiffStep` (`exec.CommandContext`). Added `TestContextCancel_*` tests and `cmdContext` helper for Cobra handlers invoked with `nil *cobra.Command` in unit tests. Preserved legacy PMA wait behavior: when the poll window expires without `/healthz` 200, `waitForPMAReadyUDS` returns `nil` (same as pre-change fire-and-forget); parent cancel still surfaces `context.Canceled`.

## Worker node

- `detectExistingInference` — already accepted `ctx` (`nodemanager_config.go`); no change.
- `pullModels` → `pullModels(ctx, models)`; `cmdRunner` adds `CombinedOutputContext` for `exec.CommandContext`.
- `waitForPMAReadyUDS` → `waitForPMAReadyUDS(ctx, socketPath, timeout) error`; uses `http.NewRequestWithContext`, `select` with `ctx`/deadline, and returns `nil` on `DeadlineExceeded` only.
- `RunOptions.PullModels` → `func(ctx context.Context, models []string) error`; `RunOptions.StartManagedServices` → `func(ctx context.Context, services []nodepayloads.ConfigManagedService) error`; `maybePullModels` / `maybeStartManagedServices` / `reconcileManagedServices` pass `ctx`.
- Tests: `context_cancel_test.go`, `main_runmain_more_test.go` (`TestContextCancel_pullModels`), adjusted managed-service tests to avoid 30s PMA waits where unnecessary.

## Cynork

- `client_http.go`: `doRequest`, `doPostJSON`, `doGetJSON`, `doPostJSONNoAuth` take `ctx`; `http.NewRequestWithContext`.
- `client.go`: all non-streaming methods take `ctx` as first parameter; streaming already used `ctx`.
- `cmd/context.go`: `cmdContext(cmd *cobra.Command)` for tests with `nil` cmd.
- Session, transport, cmd, TUI: pass `cmd.Context()`, `context.Background()`, or session `ctx` as appropriate.
- `gateway/context_cancel_test.go`: `TestContextCancel_Health`.

## SBA

- `applyUnifiedDiffStep(ctx, index, raw, workspace)`; `exec.CommandContext(ctx, ...)`.
- `agent_tools.go` and `runOneStepDirect` updated; `runner_test.go`: `TestContextCancel_applyUnifiedDiffStep`.

## Validation

- `just lint-go`
- `just test-go-cover` (all modules ≥ 90%)

## Plan

YAML `st-015`–`st-026` and Task 2 markdown checklists marked completed in `docs/dev_docs/_plan_003_short_term.md`.
