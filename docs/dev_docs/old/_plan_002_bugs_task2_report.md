# Plan `_plan_002_bugs.md` -- Task 2 (Bug 4) Completion

- [Summary](#summary)
- [Validation](#validation)
- [Deviations](#deviations)

## Summary

- Queue model: `queuedAutoSend` (Enter while agent streaming; drained on `streamDoneMsg`), `queuedExplicit` (Ctrl+Q; not auto-drained on stream end), `pendingInterruptSend` (Ctrl+Enter / Ctrl+S send-now path).
- `handleCtrlEnterKey`, `handleCtrlQKey`, `maybeStartNextQueuedUserTurn`, `beginUserTurnStream`, `isAgentStreaming` in `model_message_apply.go`; `streamDoneMsg` path combines `maybeScheduleStreamRecovery` with `maybeStartNextQueuedUserTurn(true)`.
- `EnterBlockedWhileLoading(loading, agentStreaming, input)`; BDD `cynork_tui_bugfixes.feature` uses `agent streaming is true` with package var in `steps_cynork_tui_bugfixes.go`.
- Tests: `cynork/internal/tui/bug4_queue_test.go`; E2E: `scripts/test_scripts/e2e_0650_tui_queue_model.py`; PTY harness: `ctrl+q`, `ctrl+s` in `_NAMED_KEY_BYTES`.

## Validation

- `just ci`: pass (includes `lint-go`, `lint-go-ci`, coverage thresholds, and `go test -race`)
- `go test -cover ./cynork/...`: pass (`internal/tui` at >=90% statements)
- `just e2e --tags tui_pty,no_inference`: final gate (see final report)

## Deviations

- REQ-CLIENT-0221 in `docs/requirements/client.md` refers to secret-buffer wrapping, not the queue UX; queue behavior follows `docs/tech_specs/cynork/cynork_tui.md` Queued Drafts.
- **Ctrl+Enter** is not distinct from **Enter** in bubbletea (`KeyEnter` / `KeyCtrlM`); **Ctrl+S** is implemented as an additional send-now chord alongside `ctrl+enter` when the terminal emits that string.
- Spec items not implemented in this pass: dedicated queued-draft list UI, send-all, reorder, persistence across restarts (spec: session-scoped only for now).
