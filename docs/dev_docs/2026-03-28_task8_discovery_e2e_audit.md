# Task 8 Discovery: E2E Streaming Matrix (Audit Notes)

- [Scope](#scope)
- [Status](#status)
- [BDD](#bdd)

## Scope

**Date:** 2026-03-28.

Task 8 in the consolidated plan calls out Python E2E files: `e2e_0610`, `e2e_0620`, `e2e_0630`, `e2e_0640`, `e2e_0650`, `e2e_0750`, `e2e_0760`.

## Status

This iteration focused on **cynork Godog BDD**: in-memory TUI streaming simulation (`internal/tui/stream_bdd_sim.go`), mock gateway `stream: true` SSE for `/v1/chat/completions`, stub `POST /v1/responses` streaming for dual-surface scenarios, and replacement of streaming-related `ErrPending` steps.

A full `just e2e` / tagged E2E matrix run was **not** executed in this session; re-run per `#### Testing (Task 8)` in the consolidated plan before closing the task completely.

Repo BDD: `just test-bdd 15m` completed successfully (2026-03-28).

## BDD

- Per-iteration amendment scenario in `features/cynork/cynork_tui_streaming.feature` is tagged **`@wip`**: current TUI amendment handling replaces the full visible accumulator (see unit test `TestApplyStreamDelta_AmendmentIsPerTurnVisibleReplace`), not iteration-scoped slices.

- Queue drafts, composer caret, and other **PTY-only** steps remain `ErrPending` by design.
