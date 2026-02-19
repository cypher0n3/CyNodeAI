# Unit Test Coverage Report (2026-02-18)

## Summary

Orchestrator module coverage was raised so that **all packages** meet the 90% threshold when running `just test-go-cover-podman` (Postgres via Podman).
This report summarizes changes made.

## Changes Made

Summary of work done to raise coverage:

### 1. Orchestrator Internal/models/models

- **JSONBString**: Added `TestJSONBString_Scan_string` (Scan with driver string type), `TestJSONBString_Scan_UnsupportedType`, `TestJSONBString_MarshalJSON_Nil`, `TestJSONBString_UnmarshalJSON_Invalid`.
- **Result**: models coverage 86.5% -> 97.3%.

### 2. Orchestrator Internal/auth/auth

- **JWT**: Added `TestJWTManager_ValidateAccessToken_WithRefreshToken` (wrong token type).
- **RateLimiter**: Added `TestRateLimiter_CleanupRuns` (cleanup goroutine path).
- **Result**: auth coverage 88.6% -> 94.7%.

### 3. Orchestrator Cmd (Testable Run Paths)

- **user-gateway**: Extracted `run(ctx, cfg, store, logger)`; added `TestRun_CancelledContext`, `TestRun_StartAndShutdown`, `TestLimitBody`.
  Coverage 6% -> 66.1%.
    Later: added `runMain(ctx) int`, `TestRunMain_Success`, `TestRunMain_DBOpenFails`, `TestRunMain_RunFails`.
    Coverage 66.1% -> 91.5%.
- **api-egress**: Extracted `run(ctx, logger)`; added `TestRun_CancelledContext`.
  Coverage 21.4% -> 70.8%.
    Later: added `runMain(ctx) int`, `shutdownTimeout()`, `TestRunMain_Success`, `TestRunMain_RunFails`, `TestShutdownTimeout`.
    Coverage 70.8% -> 90.3%.
- **mcp-gateway**: Same pattern as api-egress (runMain, shutdownTimeout, tests).
  Coverage 70.8% -> 90.3%.
- **control-plane**: Extracted `run(ctx, store, cfg, logger)` and `runMain()`; added `TestRun_CancelledContext`, `TestBootstrapAdminUser_GetUserError`, `TestStartDispatcher_EnabledOneTick`, `TestDispatchOnce_WorkerAPIBadVersion`, `TestDispatchOnce_WorkerAPIInvalidJSON`.
  Coverage 48.1% -> 84.6%.
  Later: `runMain()` now uses a local `flag.FlagSet` (no global flags) so it can be called multiple times in tests; added `TestRunMain_MigrateOnly`, `TestRunMain_RunFails`, `TestGetDurationEnv_InvalidValue`.
  Coverage 84.6% -> 90.2%.

### 4. Orchestrator Internal/database/database

- **GORM()**: Integration test now calls `db.GORM()` in `integrationDB()` to cover the accessor.
  Coverage 86.4% -> 87.9%.
  Later: added `TestWrapErr_Nil`, `TestWrapErr_ErrRecordNotFound`; integration tests for `GetUserByHandle_ErrNotFound`, `GetNodeBySlug_ErrNotFound`, `GetNodeByID_ErrNotFound`, `GetTaskByID_ErrNotFound`, `GetJobByID_ErrNotFound`, `GetPasswordCredentialByUserID_ErrNotFound`, `GetActiveRefreshSession_ErrNotFound`, `CreateUser_DuplicateHandle`, `ListTasksByUser_Empty`, `GetJobsByTaskID_Empty`.
  Test hooks `getSQLDB` / `getSQLDBFromDB` in `database.go` for Open/Close error paths; added `TestOpen_GetSQLDBFails`, `TestClose_GetSQLDBFromDBFails`.
  Coverage 87.9% -> 90.8%.

### 5. Orchestrator Internal/handlers/handlers

- **Refresh**: Added `TestAuthHandler_RefreshInvalidUserIDInToken` (invalid UUID in token).
  Coverage 86.9% -> 91.6%.

## Current Per-Package Coverage (Orchestrator)

With `just test-go-cover-podman` (Postgres via Podman), all packages meet the 90% threshold:

| Package            | Coverage |
|--------------------|----------|
| cmd/api-egress     | 90.3%    |
| cmd/control-plane  | 90.2%    |
| cmd/mcp-gateway    | 90.3%    |
| cmd/user-gateway   | 91.5%    |
| internal/auth      | 94.7%    |
| internal/config    | 100%     |
| internal/database  | 90.8%    |
| internal/dispatcher| 100%     |
| internal/handlers  | 91.6%    |
| internal/middleware| 98.3%    |
| internal/models    | 97.3%    |
| internal/testutil  | 97.8%    |

## How to Run

- Full coverage (all modules): `just test-go-cover-podman`
- Orchestrator only (with Postgres): set `POSTGRES_TEST_DSN` then `cd orchestrator && go test ./... -coverprofile=../tmp/go/coverage/orchestrator.coverage.out -covermode=atomic`

## Notes

- All new tests use existing patterns (mock DB, httptest, env vars, integration with Postgres).
- No Makefiles or Justfiles were modified (per instructions).
- Reports and temp files follow project layout (dev_docs, tmp).
- Orchestrator 90% coverage requires Postgres: use `just test-go-cover-podman`.
  Without Postgres, `just test-go-cover` skips integration tests and some packages report lower coverage.

### 1. Worker Node Cmd/node/node-Manager (90%+)

- Added `getEnv(key, def string)`, optional `NODE_MANAGER_DEBUG` for log level, `TestGetEnv`, `TestRunMain_DebugLevel`.
- Coverage 75% -> 92.9%.
