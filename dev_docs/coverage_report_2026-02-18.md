# Unit Test Coverage Report (2026-02-18)

## Summary

Orchestrator module coverage was raised from **79.4%** to **90.1%** using `just test-go-cover-podman`.
The 90% threshold is met; this report summarizes changes made.

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
- **api-egress**: Extracted `run(ctx, logger)`; added `TestRun_CancelledContext`.
  Coverage 21.4% -> 70.8%.
- **mcp-gateway**: Same pattern as api-egress.
  Coverage 21.4% -> 70.8%.
- **control-plane**: Extracted `run(ctx, store, cfg, logger)` and `runMain()`; added `TestRun_CancelledContext`, `TestBootstrapAdminUser_GetUserError`, `TestStartDispatcher_EnabledOneTick`, `TestDispatchOnce_WorkerAPIBadVersion`, `TestDispatchOnce_WorkerAPIInvalidJSON`.
  Coverage 48.1% -> 84.6%.

### 4. Orchestrator Internal/database/database

- **GORM()**: Integration test now calls `db.GORM()` in `integrationDB()` to cover the accessor.
  Coverage 86.4% -> 87.9%.

### 5. Orchestrator Internal/handlers/handlers

- **Refresh**: Added `TestAuthHandler_RefreshInvalidUserIDInToken` (invalid UUID in token).
  Coverage 86.9% -> 91.6%.

## Current Per-Package Coverage (Orchestrator)

| Package            | Coverage |
|--------------------|----------|
| cmd/api-egress     | 70.8%    |
| cmd/control-plane  | 84.6%    |
| cmd/mcp-gateway    | 70.8%    |
| cmd/user-gateway   | 66.1%    |
| internal/auth      | 94.7%    |
| internal/config    | 100%     |
| internal/database  | 87.9%    |
| internal/dispatcher| 100%     |
| internal/handlers  | 91.6%    |
| internal/middleware| 98.3%    |
| internal/models    | 97.3%    |
| internal/testutil  | 97.8%    |
| **Total**          | **90.1%**|

## How to Run

- Full coverage (all modules): `just test-go-cover-podman`
- Orchestrator only (with Postgres): set `POSTGRES_TEST_DSN` then `cd orchestrator && go test ./... -coverprofile=../tmp/go/coverage/orchestrator.coverage.out -covermode=atomic`

## Notes

- All new tests use existing patterns (mock DB, httptest, env vars).
- No Makefiles or Justfiles were modified (per instructions).
- Reports and temp files follow project layout (dev_docs, tmp).
