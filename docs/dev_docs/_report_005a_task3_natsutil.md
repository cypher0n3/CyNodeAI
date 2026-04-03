# Plan 005a - Task 3 Completion Report (Shared NATS Client)

<!-- Date: 2026-04-01 (UTC) -->

## Summary

Task 3 adds `go_shared_libs/natsutil`: `NatsConfig` (embedding the orchestrator `nats` JSON shape), `Validate`, `Connect(*NatsConfig)` with TLS CA bundle and bearer user JWT, `EnsureStreams` for JetStream `CYNODE_SESSION`, `CloseConn`, envelope types, and publishers for session lifecycle and `node.config_changed` (payload pointers).
Optional `subjects` is supported on `contracts/natsconfig.ClientCredentials`.
Worker `NodeJWT` now sets bearer token so `natsutil.Connect` can use JWT-only auth.

## Deliverables

- **Package:** `go_shared_libs/natsutil` (`config`, `connect`, `streams`, `envelope`, `publish`, `close`).
- **Contracts:** `subjects` field on `ClientCredentials`.
- **Issuer:** `NodeJWT` sets `BearerToken` (aligned with `SessionJWT`).

## Validation

- `go test ./go_shared_libs/... ./orchestrator/internal/natsjwt/...`
- `just lint-go`

## Plan Reference

`docs/dev_docs/_plan_005a_nats+pma_session_tracking.md` (Task 3).
