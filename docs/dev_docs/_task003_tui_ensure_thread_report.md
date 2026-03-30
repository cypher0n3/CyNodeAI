# Task 3 completion: runEnsureThread data race

**Date:** 2026-03-29

## Changes

- `cynork/internal/chat/session.go`: Added `CreateNewThreadID()` (API only, no `CurrentThreadID` mutation); `NewThread()` delegates to it.
- `cynork/internal/tui/model.go`: Replaced inline `runEnsureThread` mutations with `buildEnsureThreadOutcome()` that only reads session/cache and returns `ensureThreadResult` (including `userID` for cache write). Removed `tryResumeThreadFromCache` / `persistCurrentThreadAfterEnsure`. `applyEnsureThreadResult` now sets `Session.CurrentThreadID`, writes last-thread cache, and appends scrollback (main goroutine only). Documented tea.Cmd must not mutate model/session.
- `cynork/internal/tui/ensure_thread_test.go`: `TestEnsureThread_OutcomeDoesNotMutateSessionBeforeApply`, `TestEnsureThread_ConcurrentReadCurrentThreadIDDuringOutcomeBuild` (with `-race`).
- `cynork/internal/tui/model_test_streaming_slash_threads_test.go`: Init test asserts no `CurrentThreadID` before `Update`; success test asserts session updated after apply.

## Tests

- `go test -race -v -run TestEnsureThread ./cynork/internal/tui/...`
- `go test -race ./cynork/...`, `go test -cover ./cynork/...` (>= 90%)
- `just e2e --tags tui_pty,no_inference` (118 tests, 5 skipped)

## Deviations

- Concurrent `View()` from multiple goroutines is invalid for Bubble Tea and races on viewport state; the race regression test uses concurrent reads of `CurrentThreadID` during outcome build instead.
