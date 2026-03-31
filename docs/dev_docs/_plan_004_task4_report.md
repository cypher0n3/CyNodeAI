# Plan 004 - Task 4 Report (Batch N+1 Queries)

## Summary

As of 2026-03-31, removed N+1 database access in the orchestrator `database` layer for workflow dependency checks, task summary uniqueness, and effective preference resolution.

`workflowGateCheckDeps` now loads dependency tasks with a single `WHERE id IN (?)` query via `getTasksByIDs`.

`createTaskCore` resolves duplicate normalized summaries with one `Pluck` of candidate rows (filtered in-process with a numeric-suffix regex) instead of a per-attempt `Count` loop.

`GetEffectivePreferencesForTask` loads all relevant scope rows in one query (`listPreferenceEntriesForScopes` with `OR` on `(scope_type, scope_id)` tuples), then applies the same precedence as before (system -> user -> project -> task) using `prefScopeMatches`.

## Tests

- Integration query-budget tests in `orchestrator/internal/database/batch_query_test.go` (`TestIntegration_BatchQuery_*`, `registerQueryCounter`).
- `pref_scope_match_test.go` for scope matching.
- Extra integration coverage for `listJobsPage` / `GetJobsByTaskID` / `ListSkillsForUser` pagination clamps to keep `internal/database` at the 90% threshold.

The plan draft referenced `go test ... ./orchestrator/internal/handlers/...`; batching lives in `database`, so tests run under `./orchestrator/internal/database/...` with `-run TestIntegration_BatchQuery`.

## Verification

Use **justfile** targets (repository standard; there is no root `Makefile`).

- `just test-go-cover` - coverage thresholds (Podman for DB integration tests).
- `just lint-go` - vet and staticcheck (may still report pre-existing &gt;1000-line files).
- `just ci` - full local CI (`build-dev`, `lint`, `vulncheck-go`, `test-go-cover`, `bdd-ci`).

Task name uniqueness in code matches existing behavior (`created_by` + `summary`), not `project_id` + name as in the original plan sketch.
