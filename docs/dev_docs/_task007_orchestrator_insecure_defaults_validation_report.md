# Task 7 Completion: Startup Validation for Insecure Defaults

- [Changes](#changes)
- [Tests and Gates](#tests-and-gates)
- [Deviations](#deviations)

## Changes

**Date:** 2026-03-29.

- `orchestrator/internal/config/config.go`: named defaults (`DefaultJWTSecret`, `DefaultNodeRegistrationPSK`, `DefaultWorkerAPIBearerToken`, `DefaultBootstrapAdminPassword`); `DevMode` from `ORCHESTRATOR_DEV_MODE` (default true); `ValidateSecrets` rejects unchanged defaults when `DevMode` is false.
- Startup: `ValidateSecrets` after config load in `orchestrator/cmd/control-plane/main.go`, `user-gateway/main.go`, `mcp-gateway/main.go` (`run`), `api-egress/main.go`.
- Tests: `orchestrator/internal/config/config_test.go`; gateway coverage tests for `ORCHESTRATOR_DEV_MODE=false` with defaults in `user-gateway/main_test.go`, `mcp-gateway/run_test.go`.

## Tests and Gates

- `just lint-go`
- `just test-go-cover`
- `just e2e --tags gateway,no_inference` (118 tests, OK; some skips unrelated to this task)

## Deviations

- None.
