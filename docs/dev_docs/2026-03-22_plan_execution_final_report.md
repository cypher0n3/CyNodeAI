# Orchestrator Tool Routing and MCP Consolidation: Final Report

<!-- Date: 2026-03-22 -->

## Summary

Implementation matches the execution plan in
`docs/dev_docs/2026-03-22_orchestrator_tool_routing_and_mcp_consolidation_plan.md`.
MCP tool calls are canonical (`preference.*`, `task.get`, `job.get`, etc.) on the **control-plane**
`POST /v1/mcp/tools/call` route; legacy `db.*` tool names return **404** with a structured JSON error
and audit.
Standalone **`mcp-gateway`** is behind compose profile **`legacy-mcp-gateway`**.
Worker and
`mcpclient` honor **`ORCHESTRATOR_MCP_TOOLS_BASE_URL`** with **`ORCHESTRATOR_MCP_GATEWAY_BASE_URL`** as
deprecated alias, then orchestrator URL / proxy injection.

## Task Reports (By Number)

- **Task 1:** Inventory and completion notes in `docs/dev_docs/2026-03-22_task1_mcp_inventory_checklist.md` and `docs/dev_docs/2026-03-22_task1_completion_report.md`.
- **Task 2:** Control-plane `registerMCPToolRoute` + test; compose profile for deprecated service; `ports_and_endpoints.md`, `orchestrator/README.md`, `scripts/README.md`; standalone binary log strings clarified.
- **Task 3:** `embed_handlers.go` / `MCPToolsBaseURL`, `embed_handlers_env_test.go`; `mcpclient` env precedence for PM/SBA clients.
- **Task 4:** PMA/SBA descriptions, `TestMCPTool_Description_NoDbPrefix`, `agents/instructions/project_manager/02_tools.md`, PMA tests use `preference.get` / `task.get`.
- **Task 5:** `orchestrator/internal/mcpgateway/handlers.go` canonical routes + `isLegacyDbPrefixedToolName` + `writeLegacyDbToolRemoved`.
- **Task 6:** `scripts/test_scripts/e2e_0660_worker_pma_proxy.py` asserts proxy path `/v1/mcp/tools/call`.
  No BDD changes required (`agents/_bdd/` had no `mcp-gateway` / `12083` hits).
- **Task 7:** This report; env and port lists below; `meta.md` still describes MCP tools (not `db.*` as product names) and no mandatory separate MCP service for normal operation.

## User-Visible Environment Variables (After Tasks 3-4)

- **Worker embedded proxy (orchestrator base for tools):** `ORCHESTRATOR_MCP_TOOLS_BASE_URL` (preferred), `ORCHESTRATOR_MCP_GATEWAY_BASE_URL` (deprecated alias), then derive from `ORCHESTRATOR_INTERNAL_PROXY_BASE_URL` / `ORCHESTRATOR_URL`.
- **PMA client (`NewPMClient`):** `PMA_MCP_GATEWAY_URL`, `MCP_GATEWAY_URL`, `MCP_GATEWAY_PROXY_URL`, `ORCHESTRATOR_MCP_TOOLS_BASE_URL`, `ORCHESTRATOR_MCP_GATEWAY_BASE_URL`, `ORCHESTRATOR_URL`.
- **SBA client (`NewSBAClient`):** `SBA_MCP_GATEWAY_URL`, `MCP_GATEWAY_URL`, `ORCHESTRATOR_MCP_TOOLS_BASE_URL`, `ORCHESTRATOR_MCP_GATEWAY_BASE_URL`, `ORCHESTRATOR_URL`.
- **Managed agents (unchanged injection):** `MCP_GATEWAY_URL`, `PMA_MCP_GATEWAY_URL`, `MCP_GATEWAY_PROXY_URL` from nodeagent run args when orchestrator config supplies auto proxy URLs.

## Ports (After Task 2)

- **12082:** Control plane (MCP tool calls here: `POST /v1/mcp/tools/call`).
- **12083:** Deprecated standalone `mcp-gateway`; not started unless `docker compose --profile legacy-mcp-gateway` (see `orchestrator/docker-compose.yml`).

## Remaining Risks

- **`cmd/mcp-gateway`** binary remains for integration tests and legacy use; production path is control plane only.
- **Draft / historical docs** (`docs/mvp_plan.md`, some `docs/draft_specs/`) may still mention `db.*` tool names; normative specs and code paths are aligned.

## Validation

- **Docs:** `just docs-check` run on changed Markdown paths (pass).
- **Go:** Targeted `go test` for orchestrator (including `cmd/mcp-gateway` after panic hardening), worker_node, agents (pass).
- **Full CI:** Run `just ci` from the repo root before merge (repo standard; no Makefile in tree).

## Skipped / Optional

- **`RUN_E2E=1`:** Full Python E2E suite not run unless explicitly enabled (per `justfile`).
