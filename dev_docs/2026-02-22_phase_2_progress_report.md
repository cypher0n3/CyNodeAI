# Phase 2 Progress Report (MVP Plan)

- [Summary](#summary)
- [Completed](#completed)
- [Not done (Phase 2)](#not-done-phase-2)
- [Coverage and CI](#coverage-and-ci)
- [How to run](#how-to-run)
- [Next steps](#next-steps)

## Summary

**Date:** 2026-02-22.

Continued from Phase 1.7 and started Phase 2 (MCP in the loop) per `docs/mvp_plan.md`.
Implemented the foundation for P2-02 (audit record for every routed MCP tool call): schema, store method, and mcp-gateway wiring so the gateway writes an audit record for each tool-call request.
P2-01 (scoping/schema enforcement) and P2-03 (db.preference tools) are not started.

**E2E and chat (unchanged for Phase 2):** Full-demo E2E passes, including OpenAI-compatible chat (Test 5d).
Compose stack has user-gateway `PMA_BASE_URL` and cynode-pma `OLLAMA_BASE_URL`.
Chat routing to PMA is verified per `openai_compatible_chat_api.md`.
No Phase 2 changes to the chat path.

## Completed

Deliverables for the P2-02 foundation are below.

### P2-02 Foundation: MCP Tool Call Audit Table and Gateway Write Path

- **Schema**
  - Added `McpToolCallAuditLog` model in `orchestrator/internal/models/models.go` per `docs/tech_specs/mcp_tool_call_auditing.md` and `postgres_schema.md`.
    Columns: id, created_at, task_id, project_id, run_id, job_id, subject_type, subject_id, user_id, group_ids, role_names, tool_name, decision, status, duration_ms, error_type.
  - Included in GORM AutoMigrate in `orchestrator/internal/database/migrate.go`.
- **Store**
  - Added `CreateMcpToolCallAuditLog(ctx, rec)` to `database.Store` and implemented in `orchestrator/internal/database/mcp_audit.go`.
    Sets ID and CreatedAt when zero.
    Mock implementation in `testutil.MockDB` (supports ForceError for tests).
    Integration test `TestIntegration_McpToolCallAuditLog` in database package.
- **MCP gateway**
  - Optional DB: when `DATABASE_URL` is set, mcp-gateway opens the DB, runs schema, and uses it for audit.
    Test hooks `testStore` and `testDatabaseOpen` allow tests to inject a store without a real DB.
  - New endpoint `POST /v1/mcp/tools/call`: accepts minimal JSON `{"tool_name":"..."}`.
    Writes one audit record with decision=deny, status=error, error_type=not_implemented (tool routing not implemented yet), then returns 501.
    Every tool-call request is thus "routed" (rejected) and audited.
  - Tests: store nil (503), store set (501 after audit), invalid JSON (400), method not POST (405), audit write failure (500), run with testStore, run with testDatabaseOpen, run when DATABASE_URL invalid (Open fails).

## Not Done (Phase 2)

- **P2-01** - Enforce MCP tool scoping and schema (task_id/run_id/job_id) in the gateway.
  Requires full MCP protocol handling and per-tool schema checks.
- **P2-02** - Full "allow" path: today the gateway only writes deny + error; allow + success/error and actual tool invocation are not implemented.
- **P2-03** - Minimal MCP tool catalog slice for preferences (`db.preference.get`, `db.preference.list`, `db.preference.effective`).
  Requires `preference_entries` (and related) table(s) and an MCP tool layer that the gateway can call.

## Coverage and CI

- **mcp-gateway** uses testcontainers (same pattern as orchestrator database package).
  TestMain starts Postgres when `DATABASE_URL` is unset, sets `DATABASE_URL`, then tests run.
  `TestRun_WithRealDatabase` covers the real-DB branch (Open, RunSchema, server, tool-call audit).
  Coverage is >=90%; no justfile exception.
- **`just ci`** and **`just docs-check`** pass (lint, validate-doc-links, validate-feature-files, test-go-cover, test-bdd).

## How to Run

- **Orchestrator (with new table)**  
  `just build` then start control-plane/user-gateway as usual; migrations create `mcp_tool_call_audit_log`.
- **MCP gateway with audit**  
  Set `DATABASE_URL` to the same DSN as the orchestrator, then start mcp-gateway. `POST /v1/mcp/tools/call` with `{"tool_name":"db.preference.get"}` returns 501 and inserts one audit row.

## Next Steps

1. Implement full MCP protocol handling in the gateway (JSON-RPC, tools/call) and enforce P2-01 scoping/schema.
2. Add `preference_entries` (and any related tables) and implement P2-03 preference tools.
3. mcp-gateway tests now use testcontainers; `just test-go-cover` passes for this package.
