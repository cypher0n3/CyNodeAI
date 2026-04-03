# Plan 005a - Task 5 Completion Report (Cynork NATS)

<!-- Date: 2026-04-01 (UTC) -->

## Summary

Task 5 adds optional NATS session lifecycle for cynork after login: `userapi.LoginResponse` now includes
`interactive_session_id` and `session_binding_key` (orchestrator derives the binding key from user + refresh
session lineage).
The TUI starts `internal/sessionnats.Runtime` after a successful in-TUI login (once tokens
persist), publishing `session.attached` and periodic `session.activity` via `natsutil`, with
`session.detached` on `/auth logout`, proactive token refresh (restarts NATS with the new `nats` block), and
`CleanupSessionNats` after the Bubble Tea program exits.
Disconnect pauses activity heartbeats until NATS
reconnects (client uses library reconnect defaults).

## Deliverables

- `go_shared_libs/contracts/userapi/userapi.go` - login response session fields.
- `orchestrator/internal/handlers/auth.go` - `applyInteractiveSessionFields` on login and refresh.
- `cynork/internal/sessionnats/runtime.go` - connect, attached/activity/detached publishers.
- `cynork/internal/tui/` - wire login/refresh/logout/shutdown; `loginResultMsg` carries full `LoginResponse`.
- Tests: `cynork/internal/sessionnats/*_test.go` (`TestCynorkNats_*`).

## Validation

- `go test -count=1 -run TestCynorkNats ./cynork/...`
- `go test ./cynork/...`, `go test ./orchestrator/internal/handlers/...`
- `just lint-go`

## Plan Reference

`docs/dev_docs/_plan_005a_nats+pma_session_tracking.md` (Task 5).
