# Task 1 completion report: Orchestrator artifacts (2026-03-24)

## Summary

Implemented **explicit read grants** for user-scoped orchestrator artifacts (`artifact_read_grants` table, `GrantArtifactRead` / `HasArtifactReadGrant`, RBAC in `artifacts.Service.canReadScope`), extended **BDD** with group/project/global scope scenarios, **cross-principal read** via grant, and **MCP gateway** routing checks (PM agent may `artifact.put`; sandbox agent denied). Wired `POST /v1/mcp/tools/call` into the orchestrator `_bdd` test mux with configurable PM/sandbox bearer tokens.

## What passed

- `go test ./orchestrator/_bdd` (orchestrator Godog suite), including new artifact scenarios.
- `go test ./orchestrator/internal/artifacts/...` (including integration tests with testcontainers/PG).
- `go test ./orchestrator/internal/database/...` (including `TestIntegration_ArtifactReadGrant`).
- `just e2e --tags artifacts` (Python E2E for artifacts module).
- `just lint-go` (vet + staticcheck).
- `just validate-feature-files` after adding `@req_*` tags to scenarios.

## Deviations and follow-up

- **`just test-go-cover`** still fails repo-wide for **pre-existing** gaps (e.g. cynork `internal/tui`, `tuicache`; orchestrator `internal/database` at ~85%, `internal/artifacts` ~62%, `mcpgateway`, `s3blob`, several `cmd/*` packages below 90%). Task 1 added integration coverage for grants and service read path; raising **entire** package percentages to 90% is follow-on work, not completed in this pass.
- **`just lint-go-ci`** still reports many issues across the repo (dupl, goconst, etc.); not introduced solely by this task; plan noted existing golangci noise.
- **Extract shared RBAC helpers** (Refactor): artifact scope checks remain in `internal/artifacts`; handlers stay thin. No shared extraction was necessary once read path used `checkScope` for non-user scopes; **no silent deferral**—extraction was not needed for duplicate code between handlers.

## Files touched (high level)

- `orchestrator/internal/database/artifact_read_grants.go`, `migrate.go`
- `orchestrator/internal/artifacts/service.go`, `service_integration_test.go`
- `orchestrator/internal/database/orchestrator_artifacts_integration_test.go`
- `orchestrator/_bdd/steps.go` (MCP route, scope steps, grant step, login step with password)
- `features/orchestrator/orchestrator_artifacts.feature`
