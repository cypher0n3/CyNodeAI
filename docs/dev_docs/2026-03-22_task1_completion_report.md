# Task 1 Completion Report: Requirements Trace and Tool Inventory

<!-- Date: 2026-03-22 -->

## Inventory Summary

- **PM allowlist** and **SBA (worker) allowlist** are taken from `docs/tech_specs/mcp_tools/access_allowlists_and_scope.md` (sections `CYNAI.MCPGAT.PmAgentAllowlist` and `CYNAI.MCPGAT.WorkerAgentAllowlist`).
  Preference tools are catalog names `preference.*` per `docs/tech_specs/mcp_tools/preference_tools.md` and `CYNAI.MCPTOO.AgentFacingToolNames` in `docs/tech_specs/mcp/mcp_tooling.md`.
- **Langchaingo exposure:** PMA and SBA both use `NewMCPTool` wrapping `mcpclient`; PMA description string still lists legacy `db.preference.*`, `db.task.get`, `db.job.get` (to fix in Task 4).
  SBA `mcp_tools.go` description does not list `db.*`.
- **`db.*` usage:** Real MCP route keys and tests live mainly in `orchestrator/internal/mcpgateway/` and PMA tests/copy.
  Many `db.` matches under `orchestrator/internal/handlers/` are the Go store field `h.db` (false positive).

## File List for Tasks 2-5

See `docs/dev_docs/2026-03-22_task1_mcp_inventory_checklist.md` for the detailed checklist and grep triage.

## Open Questions

- **Legacy alias duration:** Task 5 will choose whether `db.preference.get` returns 404 or a structured error vs dual-register during migration; must match audit log tool names (`CYNAI` agent-facing names).
- **Draft docs:** `docs/draft_specs/` and `docs/mvp_plan.md` still mention `db.*` tool names; normative cleanup can lag code or be batch-updated in Task 7 per plan.

## Testing Gate

Second pass: agent-facing ambiguity remains in PMA `mcp_tools.go` and agent instruction markdown; orchestration router still uses `db.*` keys until Task 5.
Documented for follow-on tasks.
