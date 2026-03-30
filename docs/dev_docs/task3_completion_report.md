# Task 3 Completion Report - Context-Aware Retry / Poll Backoff

## Summary

Replaced blocking `time.Sleep` in orchestrator retry and PMA readiness polling with `select` on `ctx.Done()` and `time.After`, so cancellation during backoff returns promptly.
Chat completion retries now respond with HTTP 499, `request_canceled`, and `ctx.Err().Error()` when the request context ends during backoff (stdlib has no `http.StatusClientClosedRequest` on this toolchain; used named constant `statusClientClosedRequest = 499`).

## Code

- `orchestrator/internal/handlers/openai_chat.go` - `runCompletionWithRetry`: backoff uses `select`; cancel path documented with `statusClientClosedRequest`.
- `orchestrator/cmd/control-plane/main.go` - `waitForInferencePath`: both poll sleeps after error and after "not ready" use `select` with `ctx.Done()`.
- `orchestrator/internal/handlers/openai_chat_test.go` - `TestRetryContextCancel`.
- `orchestrator/cmd/control-plane/wait_for_inference_path_test.go` - `TestWaitForInferencePath_CancelDuringErrorBackoff`, `TestWaitForInferencePath_CancelDuringNotReadyBackoff` (split from `main_test.go` to stay under the 1000-line lint cap).

## Discovery

- Primary retry backoff: `openai_chat.go` (`runCompletionWithRetry`).
- Additional poll-loop sleeps: `waitForInferencePath` in `main.go` (not a "retry" in the HTTP sense, same cancellation issue).

## Validation

- `just lint-go`
- `just test-go-cover` (all modules >= 90%; `cmd/control-plane` at 90.3% after new tests)

## Plan

YAML `st-027`-`st-035` and Task 3 markdown checklists marked completed in `docs/dev_docs/_plan_003_short_term.md`.
