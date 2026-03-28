# Task 7 Completion Report (2026-03-28)

- [Summary](#summary)
- [Implementation](#implementation)
- [Verification](#verification)
- [Follow-On (Task 8+)](#follow-on-task-8)

## Summary

Plan: `docs/dev_docs/2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md` Task 7.

Completed remaining Task 7 items: gateway client file split for lint, TUI **bounded backoff reconnect** after recoverable stream errors (health probe, capped attempts, status-bar `Reconnecting...` / disconnected), unit tests (including recovery paths and transcript/thinking/amendment cases), E2E hardening for PTY slash/composer tests, and documentation in this dev_docs folder.

## Implementation

- **`cynork/internal/gateway/client_http.go`:** `doRequest`, `doPostJSON`, `doPostJSONNoAuth`, `doGetJSON`, `HTTPError`, `parseError` moved from `client.go` (file now under 1000 lines).
- **`cynork/internal/tui/model_stream_recovery.go`:** `isRecoverableGatewayStreamError`, `maybeScheduleStreamRecovery` after `streamDoneMsg`, `streamRecoveryTickMsg` with exponential backoff (cap 5s, max 5 attempts), `streamRecoveryGen` incremented on each new `streamCmd` to drop stale ticks, status bar shows reconnect/disconnected, scrollback line on restore or give-up.
- **`cynork/internal/tui/model.go` / `model_message_apply.go`:** recovery fields; `viewStatusBar` reconnect tail; `streamCmd` resets recovery generation; comment clarifying `streamBuf` vs transcript sync.
- **Tests:** `model_stream_recovery_test.go`, extra cases in `transcript_streaming_test.go`.
- **E2E:** `e2e_0760` shell escape - accept composer `│ >` when scrollback non-empty (landmark only on empty scrollback). `e2e_0765` history - `/clear` before Ctrl+Up/Down so assertions do not match `You:` lines in scrollback.

## Verification

- `just test-go-cover` - pass (including `cynork/internal/tui` >= 90%).
- `just lint-go` - pass.
- `just lint-python` (`scripts/test_scripts`) - pass.
- `just test-bdd` - pass.
- `just setup-dev restart --force` then `just e2e --tags tui_pty` - 50 tests OK (2 skipped).

## Follow-On (Task 8+)

- BDD: streaming/PTY steps still pending in `cynork/_bdd` where marked `ErrPending`.
- Optional: emit prompt-ready landmark when scrollback is non-empty (spec/E2E ergonomics); current E2E uses composer `│ >` where needed.
