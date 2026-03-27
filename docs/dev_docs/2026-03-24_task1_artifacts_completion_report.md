# Task 1 Completion Report: Orchestrator Artifacts (2026-03-24; Updated 2026-03-27)

## Summary

Implemented **explicit read grants** for user-scoped orchestrator artifacts (`artifact_read_grants` table, `GrantArtifactRead` / `HasArtifactReadGrant`, RBAC in `artifacts.Service.canReadScope`), extended **BDD** with group/project/global scope scenarios, **cross-principal read** via grant, and **MCP gateway** routing checks (PM agent may `artifact.put`; sandbox agent denied).
Wired `POST /v1/mcp/tools/call` into the orchestrator `_bdd` test mux with configurable PM/sandbox bearer tokens.

## What Passed (2026-03-27)

- `just test-go-cover` - all Go modules meet per-package coverage thresholds (including `orchestrator/internal/database` at >=90% for artifact store paths).
- `go test ./orchestrator/_bdd` (orchestrator Godog suite), including artifact scenarios.
- `go test ./orchestrator/internal/artifacts/...` (including integration tests with testcontainers/PG).
- `go test ./orchestrator/internal/database/...`.
- `just e2e --tags artifacts` (Python E2E for artifacts module; run when stack/E2E prerequisites are available).
- `just lint-go` and `just lint-go-ci` - clean in this session.
- `just docs-check` when specs or feature files change.

## Deviations and Follow-Up

- **Extract shared RBAC helpers** (Refactor): Artifact scope checks remain in `internal/artifacts`; handlers stay thin.
  No duplicate extraction was required once the read path used `checkScope` for non-user scopes.

## Files Touched (High Level)

- `orchestrator/internal/database/artifact_read_grants.go`, `migrate.go`
- `orchestrator/internal/artifacts/service.go`, `service_integration_test.go`
- `orchestrator/internal/database/orchestrator_artifacts_integration_test.go`
- `orchestrator/_bdd/steps.go` (MCP route, scope steps, grant step, login step with password)
- `features/orchestrator/orchestrator_artifacts.feature`
