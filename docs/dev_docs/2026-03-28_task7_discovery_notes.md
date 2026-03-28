# Task 7 Discovery Notes (2026-03-28)

- [PTY / Features](#pty--features)
- [TUI Model](#tui-model)
- [Transport](#transport)
- [Gaps Closed This Session](#gaps-closed-this-session)

## PTY / Features

Plan: `docs/dev_docs/2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md` Task 7.

- Cancel-retain-partial, reconnect/thread-cache, and slash toggles are covered by `e2e_0750` / `e2e_0760` and harness helpers in `scripts/test_scripts/tui_pty_harness.py`.
- `features/cynork/cynork_tui_streaming.feature` scenarios exist; step bodies for several lines remain pending (Task 8).

## TUI Model

- `TranscriptTurn` / `TranscriptPart` live in `cynork/internal/tui/state.go`; streaming applies via `applyStreamDelta`, `applyStreamDone`, `transcript_sync.go`.
- `streamBuf` is the live visible accumulator for the Assistant scrollback line; transcript visible text is synced with `syncInFlightTranscriptVisible`.
- Gateway `ChatStream` / `ResponsesStream` emit thinking, tool_call, heartbeat, iteration via `cynork/internal/chat/transport.go` and `gateway.StreamExtra`.

## Transport

- Structured SSE events are handled in `cynork/internal/gateway/client.go` (`readChatSSEStream`, `processChatSSEDataLine`).

## Gaps Closed This Session

- Bounded reconnect after transport errors: `model_stream_recovery.go` (health checks with backoff, status bar reconnect/disconnected, generation counter vs new sends).
- `client.go` split: HTTP helpers moved to `client_http.go` to satisfy the 1000-line lint gate.
