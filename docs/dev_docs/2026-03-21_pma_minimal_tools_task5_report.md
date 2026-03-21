# Task 5 Completion: Catalog and PMA Tool Description

- [Document metadata](#document-metadata)
- [Summary](#summary)
- [Validation](#validation)
- [Notes](#notes)

## Document Metadata

- **Date:** 2026-03-21.

## Summary

- Updated [docs/tech_specs/mcp_tools/task_tools.md](../tech_specs/mcp_tools/task_tools.md) with `task.list`, `task.result`, `task.cancel`, and `task.logs` (inputs, scope, behavior, algorithms) aligned with the User API Gateway and MCP gateway implementation.
- Updated [docs/tech_specs/mcp_tools/project_tools.md](../tech_specs/mcp_tools/project_tools.md) so `project.get` documents required `user_id`, and `project.list` documents required `user_id`, optional `q` / `limit` / `offset`, and pagination notes.
- Updated [docs/tech_specs/mcp_tools/help_tools.md](../tech_specs/mcp_tools/help_tools.md) overview to reference MVP embedded/in-process help content and cross-link [mcp_tooling.md](../tech_specs/mcp/mcp_tooling.md).
- Refined the PMA MCP tool description in [agents/internal/pma/mcp_tools.go](../../agents/internal/pma/mcp_tools.go) to list the minimal tool surface and point at `docs/tech_specs/mcp_tools/`.

## Validation

- `just docs-check` and `just lint-md` (via `just ci` / `just lint`) run on the repo after doc edits.

## Notes

- PM allowlist in [access_allowlists_and_scope.md](../tech_specs/mcp_tools/access_allowlists_and_scope.md) was reviewed; no namespace change required for this task.
- Legacy gateway names (`db.preference.*`, `db.task.get`, `db.job.get`) remain in the PMA description alongside resource-oriented `task.*` and `project.*` names.
