# Task 1: MCP Tool Name Inventory (Scratch)

<!-- Created: 2026-03-22 -->

## PM vs SBA Allowlists (From Specs)

Source: `docs/tech_specs/mcp_tools/access_allowlists_and_scope.md`.

### Project Manager Agent (`CYNAI.MCPGAT.PmAgentAllowlist`)

Namespaces (summary): `task.*`, `project.*`, `preference.*`, `job.*`, `system_setting.*`, optional future `specification.*`, `plan.specifications.set`, `task.specifications.set`, `specification.help`, `persona.list`, `persona.get`, `node.*`, `sandbox.*`, `artifact.*`, `artifacts.*`, `model.*`, `connector.*`, `web.fetch`, `web.search`, `api.call`, `git.*`, `help.*`.

### Worker / Sandbox Agent (`CYNAI.MCPGAT.WorkerAgentAllowlist`)

Namespaces (summary): `artifact.*`, `memory.*`, read-only `preference.get`, `preference.list`, `preference.effective`, `skills.list`, `skills.get`, `persona.get` (optional `persona.list`), `web.fetch`, `web.search`, `api.call`, optional `help.*`.

Explicitly disallowed for worker: `task.*`, `project.*`, `job.*`, `system_setting.*`, `node.*`, `sandbox.*`.

### Langchaingo Registration (Code)

- **PMA:** `agents/internal/pma/langchain.go` appends `NewMCPTool(mcpClient)` to the tool list; `agents/internal/pma/mcp_tools.go` implements `NewMCPTool` via `mcpclient.NewLangchainTool`.
- **SBA:** `agents/internal/sba/agent.go` appends `NewMCPTool(mcp)`; `agents/internal/sba/mcp_tools.go` implements `MCPTool` (description lists sandbox allowlist; no `db.*` in SBA copy today).

## `db.*` Grep Triage (Agents/, Orchestrator/, Docs/)

Classification: **rename** (agent-facing MCP name -> catalog), **remove** (wrong doc), **false positive** (Go `h.db`, test structs, `db.Raw()`, normative "must not use db.").

### Agents Package

| Location | Action |
| --- | --- |
| `agents/internal/pma/mcp_tools.go` (description string) | **rename** `db.preference.*`, `db.task.get`, `db.job.get` -> `preference.*`, `task.get`, `job.get` (Task 4). |
| `agents/internal/pma/mcp_client_test.go`, `mcp_tools_test.go` | **rename** test tool names when router accepts canonical names (Tasks 4-5). |
| `agents/internal/pma/langchain_test.go` (synthetic LLM outputs) | **rename** or keep as negative examples only if tests document legacy emission (review in Task 4). |
| `agents/internal/pma/langchain.go` comment | **rename** example text away from `db.list_tasks` if still misleading. |
| `agents/instructions/project_manager/02_tools.md` | **remove/replace** `db.*` bullet with MCP catalog language (Task 4 / doc pass). |
| `agents/instructions/sandbox_agent/*.md` | **remove** "do not invoke db.*" phrasing in favor of forbidden namespaces per spec (`task.*`, `node.*`, `sandbox.*` as already stated). |
| `agents/instructions/project_analyst/02_tools.md` | **rename** `db.read` / `db.write` to catalog-consistent verification wording. |

### Orchestrator Service

| Location | Action |
| --- | --- |
| `orchestrator/internal/mcpgateway/handlers.go` | **rename** route keys to canonical `preference.*`, `task.get`, `job.get`; keep legacy `db.*` only if compatibility layer in Task 5. |
| `orchestrator/internal/mcpgateway/handlers_test.go` | **rename** requests/expectations to canonical names + legacy error policy tests (Task 5). |
| `orchestrator/internal/mcptaskbridge/bridge_test.go` | **false positive** (`db` as in-memory mock struct). |
| `orchestrator/internal/handlers/*.go` | **false positive** (`h.db` store field). |

### Docs (Non-Exhaustive; Focus on Agent-Visible)

| Location | Action |
| --- | --- |
| `docs/mvp_plan.md`, `docs/draft_specs/*` | Historical / draft `db.*` names: **note** as draft or update when touching (Task 7 as needed). |
| Normative specs citing "no `db.`" | **false positive** (forbidden prefix, not a tool route). |

## Files Expected to Change in Tasks 2-5 (Checklist)

- [ ] `orchestrator/cmd/control-plane/main.go` (+ tests): MCP route registration guard (Task 2).
- [ ] `orchestrator/docker-compose.yml`: `mcp-gateway` not in default profile (Task 2).
- [ ] `orchestrator/cmd/mcp-gateway/main.go`: log strings / deprecation clarity (Task 2).
- [ ] `docs/tech_specs/ports_and_endpoints.md`, `orchestrator/README.md` (Task 2).
- [ ] `agents/internal/mcpclient/client.go` (Task 3-4).
- [ ] `worker_node/internal/nodeagent/runargs.go`, `worker_node/internal/workerapiserver/embed_handlers.go` (+ tests) (Task 3).
- [ ] `agents/internal/pma/mcp_tools.go`, `agents/internal/sba/mcp_tools.go` (+ tests) (Task 4).
- [ ] `orchestrator/internal/mcpgateway/handlers.go` (+ tests) (Task 5).
- [ ] `scripts/README.md` if it references `12083` for PMA (Task 3).

## Task 1 Red (Tests)

Inventory-only pass: expectations using `db.preference.get` remain until **Task 3-5** implement canonical names and router behavior; Red for those tests completes there (per plan).
