# Task 8 Completion Report (Partial): BDD Streaming and Mock Gateway

- [Summary](#summary)
- [Code changes](#code-changes)
- [Tests run](#tests-run)
- [Full `just e2e` (entire Python suite)](#full-just-e2e-entire-python-suite)
- [Not done here](#not-done-here)
- [Remaining `ErrPending` (intentional)](#remaining-errpending-intentional)

## Summary

**Date:** 2026-03-28.

Implemented cynork BDD support for **streaming and in-memory TUI simulation** without a PTY: exported helpers on `tui.Model`, wired `cynorkState.bddStream`, replaced streaming-related `godog.ErrPending` steps in mock gateway, scrollback, and TUI-deferred registrars, and extended the BDD mock HTTP mux for streaming completions and `/v1/responses`.

## Code Changes

- `cynork/internal/tui/stream_bdd_sim.go` - `StreamBDD*` helpers and small getters for BDD assertions.
- `cynork/internal/tui/stream_bdd_sim_test.go` - unit tests for sim + drain edge cases.
- `cynork/_bdd/steps.go` - state fields and `Before` resets for BDD stream + mock flags.
- `cynork/_bdd/gateway_http_mock_mux.go` - `stream: true` -> SSE token chunks; `POST /v1/responses` (JSON + SSE).
- `cynork/_bdd/steps_cynork_streaming_bdd.go` - streaming feature steps and shared bdd helpers.
- `cynork/_bdd/steps_cynork_extra_mock_gateway.go`, `steps_cynork_extra_scrollback_config.go`, `steps_cynork_extra_tui_deferred.go`, `steps2.go` - wiring and step body updates.
- `features/cynork/cynork_tui_streaming.feature` - `@wip` on per-iteration amendment scenario.

## Tests Run

- `go test ./cynork/internal/tui/...`
- `go test ./cynork/_bdd -run TestCynorkBDD`
- `just test-bdd` (all modules with `_bdd`)
- `just bdd-ci` (strict Godog: `GODOG_STRICT=1` + `just test-bdd`)
- `just test-go-cover` (all modules, >=90% thresholds)
- `just e2e --tags streaming`, `just e2e --tags tui_pty`, `just e2e --tags pma_inference,chat` (Task 8 streaming / TUI / chat matrix)

## Full `just e2e` (Entire Python Suite)

Earlier **2026-03-28** run: **58 failures** (149 tests, 8 skipped) with symptoms in `e2e_0510`, `e2e_0770`, MCP **404**, artifacts **404** (stack/image drift; see orchestrator image rebuild and E2E fixes on `mvp/phase-2`).

**Follow-up:** Full `just e2e --no-build` completed **OK** (149 tests, 8 skipped, ~33 min) after E2E fixes to inference task result parsing, `/v1/responses` SSE delta shape, TUI thread-cache polling, and longer inference-task wait.
Treat full suite as the release gate when run on a fresh `just setup-dev` stack.

## Not Done Here

- None for the **full Python suite** gate once the stack matches rebuilt images and the E2E scripts above.

## Remaining `ErrPending` (Intentional)

- PTY-only steps (queued drafts, mouse scroll, composer caret, etc.).
- Queue draft feature steps.
