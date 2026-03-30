# Task 4 Completion Report - Async TUI Network I/O

## Summary

Moved blocking gateway calls out of the synchronous `Update()` path for `/thread new`, `/thread switch`, and stream-recovery health checks.
`/thread new` and `/thread switch` now use `tea.Cmd` returning `threadNewResult` / `threadSwitchResult`; `applyStreamRecoveryTick` schedules `streamRecoveryHealthCheckCmd`, with outcomes handled via `streamRecoveryHealthResultMsg` (including `noClient` when the session disappears before the Cmd runs).
`CreateNewThreadID` is used in the Cmd so the Cmd closure does not mutate `Session` until `applyThreadNewResult` on the main goroutine.

## Files

- `cynork/internal/tui/model.go` - message types; `applyThreadMsgs` / `applyTokenAndGatewayMsgs` dispatch for new results.
- `cynork/internal/tui/model_thread_commands.go` - async `threadCommandNew` / `threadCommandSwitch`; `applyThreadNewResult` / `applyThreadSwitchResult` (split from `model.go` for line-count lint).
- `cynork/internal/tui/model_stream_recovery.go` - async health check + `applyStreamRecoveryHealthResult`.
- `cynork/internal/tui/model_update_noblock_test.go` - `TestUpdateNoBlock_*`.
- Tests updated: `model_thread_commands_test.go`, `model_test_viewport_clipboard_test.go`, `model_stream_recovery_test.go`, `bug3_thread_ux_test.go`.

## Validation

- `just lint-go`
- `go test -race ./cynork/...`; `just test-go-cover` (`internal/tui` >= 90%)
- `just e2e --tags tui_pty,no_inference` (122 tests, OK)

## Plan

YAML `st-036`-`st-046` and Task 4 markdown checklists marked completed in `docs/dev_docs/_plan_003_short_term.md`.
