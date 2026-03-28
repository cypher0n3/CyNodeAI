# Task 8: `godog.ErrPending` Classification (2026-03-28)

- [Summary](#summary)
- [By file](#by-file)
- [Intent](#intent)
- [Refactor note](#refactor-note)

## Summary

Consolidated plan Task 8 asks to list every `ErrPending` in `steps2.go` and related BDD registrars. **`steps2.go` contains no `return godog.ErrPending`** (only a header comment).
Pending steps live in companion files registered from `steps2.go`.

## By File

- **File:** `steps_cynork_streaming_bdd.go`
  - kind: Fallback
  - notes: One `return godog.ErrPending` when neither mock gateway nor token stream mode is set for a step that requires one of the two paths (guard / misconfiguration).
- **File:** `steps_cynork_extra_scrollback_config.go`
  - kind: Queue draft + PTY
  - notes: Queue draft feature not implemented; several steps require PTY.
- **File:** `steps_cynork_extra_mock_gateway.go`
  - kind: Queue draft + PTY
  - notes: Queue draft not implemented; PTY-only observability.
- **File:** `steps_cynork_extra_tui_deferred.go`
  - kind: PTY + web login
  - notes: PTY login form, web login, second mock gateway, status bar: **Task 9** auth-recovery and PTY coverage aligns here.

## Intent

- **Streaming BDD:** Implemented via `stream_bdd_sim.go`, mock gateway SSE, and `steps_cynork_streaming_bdd.go` (non-pending paths).
- **PTY / real web login:** Explicitly left `ErrPending` per plan; covered by Python E2E `e2e_0750` / `e2e_0760` where applicable.

## Refactor Note

Shared SSE / scrollback helpers for BDD live in `steps_cynork_streaming_bdd.go` and `gateway_http_mock_mux.go`; further extraction is optional and not blocking Task 8 closure.
