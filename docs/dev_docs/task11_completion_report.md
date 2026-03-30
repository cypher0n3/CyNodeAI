# Task 11 Completion Report — BDD (ACCESS, AGENTS, MCPGAT, MCPTOO)

## Summary

Executable feature files (no `@wip`) under `features/` with suite tags, user stories, and `@req_*` + `@spec_*` traceability:

- **ACCESS:** `features/e2e/access_control_gateway.feature` (`@suite_e2e`) — unauthenticated `GET /v1/tasks` expects 401; step `I call GET "..." without an Authorization header` in `e2e/_bdd/steps.go`.
- **AGENTS (SBAGNT):** `features/agents/sbagnt_runner_contract.feature` — validates protocol `1.0` job spec (reuses existing SBA contract steps).
- **MCPGAT:** `features/orchestrator/mcpgat_gateway_auth.feature` — `POST /v1/mcp/tools/call` without `Authorization` expects 401; step in `orchestrator/_bdd/steps_orchestrator_workflow_egress_artifacts.go`.
- **MCPTOO:** `features/orchestrator/mcptoo_help_catalog.feature` — PM agent `help.list` returns 200 and JSON containing `topics`; steps in same Go file.

Earlier stub filenames (`*_domain_stub.feature`) were removed in favor of the files above.

Discovery used `docs/requirements/README.md` and domain files for REQ IDs; spec tags use tech-spec anchors as before.

## Validation

- `just test-bdd` (Godog suites run these scenarios by default)
- `just validate-feature-files` and `just lint-gherkin` on `features/`

## Plan

YAML `st-112`-`st-121` and Task 11 markdown checklists updated in `docs/dev_docs/_plan_003_short_term.md`.
