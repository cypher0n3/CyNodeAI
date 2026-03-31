# Task 4 Completion: PMA Teardown (REQ-ORCHES-0191)

<!-- Date: 2026-03-31 (UTC) -->

## Behavior

- **Teardown path:** `TeardownPMAForInteractiveSession` marks the binding `teardown_pending`, bumps PMA host `config_version` (`BumpPMAHostConfigVersion`), and records `LastPMATeardownForTest` for diagnostics.
- **Logout:** After invalidating the refresh session, `auth.go` calls teardown for that interactive session.
- **Admin revoke:** `RevokeSessions` calls `TeardownAllActivePMABindingsForUser` after `InvalidateAllUserSessions` (`users.go`; handler uses `database.Store`).
- **Activity:** `TouchPMABindingActivity` (on `SessionBindingStore`) runs when `model=cynodeai.pm` so idle policy uses `last_activity_at` (`openai_chat.go`).
- **Scanner:** `RunPMABindingScanner` in `user-gateway` `main.go`; `scanPMABindingsOnce` tears down bindings whose refresh row is missing, inactive/expired, or idle beyond `PMA_BINDING_IDLE_TIMEOUT_MIN` (default 30m), with tick interval `PMA_BINDING_SCAN_INTERVAL_SEC` (default 60s).

## Tests

- `pma_teardown_test.go`: `TestPmaTeardown_*`, `TestTouchPMABindingActivity_*`, `TestScanPMABindingsOnce_*`.
- `handlers_mockdb_auth_test.go`: `TestAuthHandler_LogoutWithRefreshToken` asserts binding -> `teardown_pending` and teardown record.

## Validation

- `just lint-go` and `go test -cover ./orchestrator/...` passed in this session.

## Plan Reference

`docs/dev_docs/_plan_005_pma_provisioning.md` (Task 4).
