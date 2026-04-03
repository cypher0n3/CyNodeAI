# Plan 005a - Task 2 Completion Report (NATS Auth and Config Block)

<!-- Date: 2026-04-01 (local) -->

## Summary

Task 2 adds decentralized NATS JWT issuance (`orchestrator/internal/natsjwt`), wires `nats` objects into login/refresh (`userapi.LoginResponse`) and node bootstrap (`nodepayloads.BootstrapResponse`), loads NATS URLs from `OrchestratorConfig`, and generates a dev NATS server JWT bundle to the host cache (`NATS_DEV_JWT_DIR`, default XDG cache) via `gen-nats-dev-jwt` / setup-dev so `nats-server-dev.conf` can use operator + full resolver with JetStream on the app account.
The system account JWT has JetStream disabled to satisfy NATS 2.10 rules.

## Deliverables

- **Dependencies:** `github.com/nats-io/jwt/v2`, `github.com/nats-io/nkeys` in `orchestrator/go.mod`.
- **Issuer:** `SessionJWT` / `NodeJWT`, `RevokeSessionNatsJWT` / `RevokeJTI`, dev seeds overridable via `NATS_DEV_*` / `NATS_ACCOUNT_*` (see `.env.example`).
- **Config:** `NATSClientURL`, `NATSWebSocketURL`, `NATSAccountSeed`, `NATSAccountSigningSeed` (dev defaults applied when `DevMode` is true).
- **Handlers:** `AuthHandler` attaches `nats` on login/refresh, tracks JWT `jti` per refresh session, revokes on logout and refresh rotation; `NodeHandler` adds `nats` to bootstrap when issuer is configured.
- **Compose:** `NATS_CLIENT_URL` / `NATS_WEBSOCKET_URL` for gateway and control-plane; NATS mounts `operator.jwt` + `accounts/` resolver dir.

## Validation

- `go test ./orchestrator/internal/natsjwt/...` (tests named `TestNatsJwt_*`).
- `go test ./orchestrator/internal/handlers/...`.
- `just lint-go` (clean).

## Plan Reference

`docs/dev_docs/_plan_005a_nats+pma_session_tracking.md` (Task 2).
