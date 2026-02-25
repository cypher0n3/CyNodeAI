# Phase 2 P2-03: Preference Tools Implementation Report

- [Summary](#summary)
- [Deliverables](#deliverables)
- [CI and Coverage](#ci-and-coverage)
- [References](#references)

## Summary

**Date:** 2026-02-23.

Implemented the minimal MCP tool catalog slice for preferences (P2-03) per `docs/mvp_plan.md`: schema and store for preference entries, and MCP gateway allow path for `db.preference.get`, `db.preference.list`, and `db.preference.effective`.

Lint and markdown fixes were applied; `just lint-go-ci` and `just docs-check` pass.
Coverage for `orchestrator/internal/database` and `orchestrator/cmd/mcp-gateway` depends on testcontainers (database) and additional handler coverage (mcp-gateway).
See CI and Coverage below.

## Deliverables

- **Schema:** `PreferenceEntry` and `PreferenceAuditLog` models in `orchestrator/internal/models/models.go`; added to GORM AutoMigrate in `orchestrator/internal/database/migrate.go`.
  Unique index on `(scope_type, scope_id, key)` for preference entries.
- **Store:** `GetPreference(ctx, scopeType, scopeID, key)`, `ListPreferences(ctx, scopeType, scopeID, keyPrefix, limit, cursor)` with size cap and pagination, `GetEffectivePreferencesForTask(ctx, taskID)` with precedence (task > project > user > system; group omitted for MVP).
  Implemented in `orchestrator/internal/database/preferences.go`.
  Helper `effectiveScopesForTask` and `ParsePreferenceValue` for reuse.
- **MockDB:** `GetPreference`, `ListPreferences`, `GetEffectivePreferencesForTask` and helper `matchPreferenceEntry` in `orchestrator/internal/testutil/mock_db.go`.
- **MCP gateway:** Request body extended with optional `arguments` map. `routeToolCall` dispatches `db.preference.get`, `db.preference.list`, `db.preference.effective` to handlers that validate args, call store, write one audit record (decision allow/deny, status success/error), and return 200/4xx/5xx with JSON.
  Constants used for audit decision/status (goconst).
  Named return values (gocritic).
  All preference handlers return allow+success or allow/deny+error and audit.
- **Tests:** Integration tests for preferences (get, list, effective, cursor, user scope, effective with project); unit tests for `ParsePreferenceValue`; mcp-gateway tests for preference get (found, not found, bad args), list (empty, user scope, limit/cursor), effective (success, bad args), and store error path.
  Cognitive complexity reduced by splitting tests and extracting helpers.
- **Docs:** Fixed `dev_docs/2026-02-22_traceability_followup.md` for markdownlint (ascii-only, no-h1-content).

## CI and Coverage

- **Lint:** `just lint-go-ci` passes (gocognit, goconst, gocritic addressed).
- **Docs:** `just docs-check` passes.
- **Database package:** Reaches 90%+ when integration tests run (testcontainers starts Postgres and sets `POSTGRES_TEST_DSN`, or `POSTGRES_TEST_DSN` is set and Postgres is available).
  When run as part of `go test ./...` without a DB, integration tests skip and package coverage is low (~5%).
  Run `go test ./internal/database -count=1` with testcontainers or set `POSTGRES_TEST_DSN` for full coverage.
- **mcp-gateway package:** New tests raise coverage; full 90% may require additional handler-branch tests or running in an environment where all tests execute.

## References

- `docs/mvp_plan.md` (P2-03, Phase 2)
- `docs/tech_specs/mcp_tool_catalog.md` (Database Tools, db.preference.get/list/effective)
- `docs/tech_specs/user_preferences.md` (Effective resolution, MCP preference tools)
- `docs/tech_specs/postgres_schema.md` (Preference entries, preference_audit_log)
- `docs/tech_specs/mcp_tool_call_auditing.md` (Audit fields)
