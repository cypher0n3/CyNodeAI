# Plan `_plan_002_bugs.md` -- Task 1 (Bug 3) Completion

- [Summary](#summary)
- [Validation](#validation)
- [Deviations / product notes](#deviations--product-notes)

## Summary

- Report date: 2026-03-30.
- `/auth logout` now clears `chat.Session.CurrentThreadID` in `authLogout()` (`slash.go`) so ensure-thread cannot reuse a stale thread after token clear.
- Ensure-thread scrollback landmarks distinguish new thread creation (`createdNew`), disk cache resume (`resumedFromCache`), selector resolution, and same-thread confirmation via `ensureThreadResult` fields and `ensureThreadScrollbackLine`.
- New tests: `cynork/internal/tui/bug3_thread_ux_test.go` (`TestLogoutClearsThread`, `TestLoginNewUser`, `TestScrollbackLandmark_FromEnsureThreadResult`, `TestScrollbackLandmark_ThreadSwitch`, `TestBuildEnsureThreadOutcome_resumesFromCache`); extended `TestEnsureThreadScrollbackLine` in `model_test.go`.

## Validation

- `just ci`: pass (includes `lint-go`, `lint-go-ci`, `go test -coverprofile` with >=90% on `cynork/internal/tui`, and `go test -race`)
- `just e2e --tags tui_pty,no_inference`: OK (121 tests, 5 skipped)

## Deviations / Product Notes

- `chat.Session` has no `CurrentThreadTitle` field (status bar uses truncated `CurrentThreadID` only).
  The plan checkbox referenced clearing `Session.CurrentThreadTitle`; only `CurrentThreadID` was cleared.
  Thread title in the status bar is derived from the thread id until a separate title field exists on the session model.
- In-TUI login scope (whether scrollback/transcript should reset on user switch) remains a product decision; this task only clears thread identity on logout and fixes ensure landmarks.
