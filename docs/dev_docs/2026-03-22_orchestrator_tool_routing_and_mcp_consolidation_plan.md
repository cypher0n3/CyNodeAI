---
name: Orchestrator Tool Routing and MCP Consolidation
overview: |
  Unify privileged agent tool routing: PMA langchaingo tools and mcpclient to the orchestrator
  control-plane (worker internal proxy by default), MCP as protocol and naming contract, no db.*
  agent-facing names, collapsed MCP base URL env vars, deprecated standalone mcp-gateway on 12083.
# Todos: one entry per `- [ ]` step in the Execution Plan body (order preserved). Each depends on the prior step.
todos:
  - id: mcp-routing-step-001
    content: "Read the specifications above and list PM vs SBA allowlists as specified."
    status: completed
  - id: mcp-routing-step-002
    content: "Grep the repo for `NewMCPTool`, `NewLangchainTool`, and langchaingo `tools.Tool` registration in `agents/internal/pma/` and `agents/internal/sba/` to align inventory with how catalog names are exposed to the model."
    status: completed
    dependencies:
      - mcp-routing-step-001
  - id: mcp-routing-step-003
    content: "Grep the repo for `db.`, `db.preference`, and `db.task` in `agents/`, `orchestrator/`, and `docs/`; record each hit as **remove**, **rename to preference.**, or **false positive** (comments only)."
    status: completed
    dependencies:
      - mcp-routing-step-002
  - id: mcp-routing-step-004
    content: "Add or update a short **inventory checklist** (can live in this plan or a scratch note under `docs/dev_docs/`) listing every file that must change in later tasks."
    status: completed
    dependencies:
      - mcp-routing-step-003
  - id: mcp-routing-step-005
    content: "Where tests encode `db.preference.get`, add failing tests or update expectations to the **new canonical names** once those names are fixed in Task 3+ (Red may be deferred if inventory-only; if so, complete Red in Task 3)."
    status: completed
    dependencies:
      - mcp-routing-step-004
  - id: mcp-routing-step-006
    content: "No code change required if this task is inventory-only; otherwise limit edits to **documentation of findings** only."
    status: completed
    dependencies:
      - mcp-routing-step-005
  - id: mcp-routing-step-007
    content: "N/A unless consolidating duplicate grep results into a single table-free list in the inventory."
    status: completed
    dependencies:
      - mcp-routing-step-006
  - id: mcp-routing-step-008
    content: "Review inventory with a second pass: no remaining ambiguous `db.*` references in agent-facing descriptions."
    status: completed
    dependencies:
      - mcp-routing-step-007
  - id: mcp-routing-step-009
    content: "Generate a **task completion report**: inventory summary, file list for Tasks 2-5, open questions."
    status: completed
    dependencies:
      - mcp-routing-step-008
  - id: mcp-routing-step-010
    content: "Mark every completed step in Task 1 with `- [x]`."
    status: completed
    dependencies:
      - mcp-routing-step-009
  - id: mcp-routing-step-011
    content: "Do not start Task 2 until this closeout is done."
    status: completed
    dependencies:
      - mcp-routing-step-010
  - id: mcp-routing-step-012
    content: "Read `orchestrator/cmd/control-plane/main.go` and confirm `POST /v1/mcp/tools/call` registration."
    status: completed
    dependencies:
      - mcp-routing-step-011
  - id: mcp-routing-step-013
    content: "Read `orchestrator/cmd/mcp-gateway/main.go` and list differences from `internal/mcpgateway` usage on the control plane."
    status: completed
    dependencies:
      - mcp-routing-step-012
  - id: mcp-routing-step-014
    content: "Read `orchestrator/docker-compose.yml` for `mcp-gateway` service and ports."
    status: completed
    dependencies:
      - mcp-routing-step-013
  - id: mcp-routing-step-015
    content: "Add or update tests that fail if the control-plane mux does **not** register the tool route (if not already covered)."
    status: completed
    dependencies:
      - mcp-routing-step-014
  - id: mcp-routing-step-016
    content: "Add a compose or doc test only if the repo already has a pattern for it; otherwise skip and document manual verification."
    status: completed
    dependencies:
      - mcp-routing-step-015
  - id: mcp-routing-step-017
    content: "Remove or disable **mcp-gateway** from the **default** compose profile, or mark it **deprecated** with comments and no published host port in dev unless needed for legacy tests."
    status: completed
    dependencies:
      - mcp-routing-step-016
  - id: mcp-routing-step-018
    content: "Ensure `internal/mcpgateway` remains the **single** implementation package; `cmd/mcp-gateway` stays a thin wrapper **or** is test-only per team decision."
    status: completed
    dependencies:
      - mcp-routing-step-017
  - id: mcp-routing-step-019
    content: "Update [ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md) and orchestrator README snippets so **MCP tool calls** point to **control plane** port and path."
    status: completed
    dependencies:
      - mcp-routing-step-018
  - id: mcp-routing-step-020
    content: "Deduplicate log strings and startup messages that still say \"mcp-gateway\" when the process is control-plane-only in production."
    status: completed
    dependencies:
      - mcp-routing-step-019
  - id: mcp-routing-step-021
    content: "`go test` for `orchestrator/cmd/control-plane/...` and `orchestrator/internal/mcpgateway/...`."
    status: completed
    dependencies:
      - mcp-routing-step-020
  - id: mcp-routing-step-022
    content: "`go test` for `orchestrator/cmd/mcp-gateway/...` if that binary remains."
    status: completed
    dependencies:
      - mcp-routing-step-021
  - id: mcp-routing-step-023
    content: "`just docs-check` if tech specs or README changed."
    status: completed
    dependencies:
      - mcp-routing-step-022
  - id: mcp-routing-step-024
    content: "**Task completion report:** compose changes, port story, any kept legacy path for `mcp-gateway`."
    status: completed
    dependencies:
      - mcp-routing-step-023
  - id: mcp-routing-step-025
    content: "Mark Task 2 steps `- [x]`."
    status: completed
    dependencies:
      - mcp-routing-step-024
  - id: mcp-routing-step-026
    content: "Do not start Task 3 until closeout is done."
    status: completed
    dependencies:
      - mcp-routing-step-025
  - id: mcp-routing-step-027
    content: "Read `agents/internal/mcpclient/client.go` (`NewPMClient`, `isInternalProxyMCPURL`, `callViaWorkerInternalProxy`)."
    status: completed
    dependencies:
      - mcp-routing-step-026
  - id: mcp-routing-step-028
    content: "Read `worker_node/internal/nodeagent/runargs.go` for injected `MCP_GATEWAY_*` env."
    status: completed
    dependencies:
      - mcp-routing-step-027
  - id: mcp-routing-step-029
    content: "Read `worker_node/internal/workerapiserver/embed_handlers.go` for `ORCHESTRATOR_MCP_GATEWAY_BASE_URL` and derivation."
    status: completed
    dependencies:
      - mcp-routing-step-028
  - id: mcp-routing-step-030
    content: "Add failing tests if the worker derives the wrong base (for example still pointing at a separate `:12083` host) when `ORCHESTRATOR_URL` is set to the control plane."
    status: completed
    dependencies:
      - mcp-routing-step-029
  - id: mcp-routing-step-031
    content: "Add tests for env precedence: explicit override wins; otherwise derive from orchestrator/control-plane URL."
    status: completed
    dependencies:
      - mcp-routing-step-030
  - id: mcp-routing-step-032
    content: "Implement **one precedence rule** documented in code comments, for example:"
    status: completed
    dependencies:
      - mcp-routing-step-031
  - id: mcp-routing-step-033
    content: "`ORCHESTRATOR_MCP_TOOLS_BASE_URL` (optional new name) **or** keep `ORCHESTRATOR_MCP_GATEWAY_BASE_URL` as deprecated alias."
    status: completed
    dependencies:
      - mcp-routing-step-032
  - id: mcp-routing-step-034
    content: "Else worker-injected proxy URL for managed agents."
    status: completed
    dependencies:
      - mcp-routing-step-033
  - id: mcp-routing-step-035
    content: "Else `ORCHESTRATOR_URL` / internal proxy base + fixed path."
    status: completed
    dependencies:
      - mcp-routing-step-034
  - id: mcp-routing-step-036
    content: "Update [scripts/README.md](../../scripts/README.md) or setup-dev notes only if they reference `mcp-gateway:12083` for PMA."
    status: completed
    dependencies:
      - mcp-routing-step-035
  - id: mcp-routing-step-037
    content: "Rename symbols in worker code from `MCPGatewayBaseURL` to `MCPToolsBaseURL` **only if** it is a localized rename with no public API break, or defer to a follow-up."
    status: completed
    dependencies:
      - mcp-routing-step-036
  - id: mcp-routing-step-038
    content: "`go test ./worker_node/internal/workerapiserver/...`"
    status: completed
    dependencies:
      - mcp-routing-step-037
  - id: mcp-routing-step-039
    content: "`go test ./worker_node/internal/nodeagent/...` if runargs change."
    status: completed
    dependencies:
      - mcp-routing-step-038
  - id: mcp-routing-step-040
    content: "**Task completion report:** final env var list and precedence table in prose (no GFM table; use bullet list)."
    status: completed
    dependencies:
      - mcp-routing-step-039
  - id: mcp-routing-step-041
    content: "Mark Task 3 steps `- [x]`."
    status: completed
    dependencies:
      - mcp-routing-step-040
  - id: mcp-routing-step-042
    content: "Do not start Task 4 until closeout is done."
    status: completed
    dependencies:
      - mcp-routing-step-041
  - id: mcp-routing-step-043
    content: "Read PMA and SBA `mcp_tools.go` and list every **documented** tool name; read PMA `langchain.go` (and SBA wiring) for where **`NewMCPTool`** is registered on the **langchaingo** agent."
    status: completed
    dependencies:
      - mcp-routing-step-042
  - id: mcp-routing-step-044
    content: "Compare to Task 1 inventory and to PM/SBA allowlists in specs."
    status: completed
    dependencies:
      - mcp-routing-step-043
  - id: mcp-routing-step-045
    content: "Add or update unit tests that assert the **description string** does not contain `db.` (or snapshot the allowed substring set)."
    status: completed
    dependencies:
      - mcp-routing-step-044
  - id: mcp-routing-step-046
    content: "Replace `db.preference.*` with **`preference.*`** per [preference_tools.md](../tech_specs/mcp_tools/preference_tools.md)."
    status: completed
    dependencies:
      - mcp-routing-step-045
  - id: mcp-routing-step-047
    content: "Remove or rename **`db.task.get`** / **`db.job.get`** references to **`task.*`** / **`job.*`** per catalog."
    status: completed
    dependencies:
      - mcp-routing-step-046
  - id: mcp-routing-step-048
    content: "Ensure `NewPMClient` / `NewSBAClient` use the collapsed env rules from Task 3."
    status: completed
    dependencies:
      - mcp-routing-step-047
  - id: mcp-routing-step-049
    content: "Centralize a single **allowlist snippet** helper if duplication between PMA and SBA becomes unmaintainable (only if minimal)."
    status: completed
    dependencies:
      - mcp-routing-step-048
  - id: mcp-routing-step-050
    content: "`go test ./agents/internal/pma/... ./agents/internal/sba/... ./agents/internal/mcpclient/...`"
    status: completed
    dependencies:
      - mcp-routing-step-049
  - id: mcp-routing-step-051
    content: "**Task completion report:** before/after description samples, test list."
    status: completed
    dependencies:
      - mcp-routing-step-050
  - id: mcp-routing-step-052
    content: "Mark Task 4 steps `- [x]`."
    status: completed
    dependencies:
      - mcp-routing-step-051
  - id: mcp-routing-step-053
    content: "Do not start Task 5 until closeout is done."
    status: completed
    dependencies:
      - mcp-routing-step-052
  - id: mcp-routing-step-054
    content: "Grep `orchestrator/internal/mcpgateway` for `db.` route keys or string switches."
    status: completed
    dependencies:
      - mcp-routing-step-053
  - id: mcp-routing-step-055
    content: "Map each to `preference.*`, `task.*`, or explicit **not implemented** per MVP."
    status: completed
    dependencies:
      - mcp-routing-step-054
  - id: mcp-routing-step-056
    content: "Add tests that **`db.preference.get`** returns **404** or **structured error** (per chosen migration policy) and **`preference.get`** succeeds on the happy path where implemented."
    status: completed
    dependencies:
      - mcp-routing-step-055
  - id: mcp-routing-step-057
    content: "Implement routing per Red tests; prefer **one canonical name** per operation."
    status: completed
    dependencies:
      - mcp-routing-step-056
  - id: mcp-routing-step-058
    content: "Update audit logging tool names to match **agent-facing** names."
    status: completed
    dependencies:
      - mcp-routing-step-057
  - id: mcp-routing-step-059
    content: "Extract shared route registration if `db.*` and `preference.*` temporarily coexist behind a compatibility layer."
    status: completed
    dependencies:
      - mcp-routing-step-058
  - id: mcp-routing-step-060
    content: "`go test ./orchestrator/internal/mcpgateway/...`"
    status: completed
    dependencies:
      - mcp-routing-step-059
  - id: mcp-routing-step-061
    content: "**Task completion report:** migration behavior for legacy names, test evidence."
    status: completed
    dependencies:
      - mcp-routing-step-060
  - id: mcp-routing-step-062
    content: "Mark Task 5 steps `- [x]`."
    status: completed
    dependencies:
      - mcp-routing-step-061
  - id: mcp-routing-step-063
    content: "Do not start Task 6 until closeout is done."
    status: completed
    dependencies:
      - mcp-routing-step-062
  - id: mcp-routing-step-064
    content: "Identify existing E2E or BDD steps for PMA MCP (`agents/_bdd/`, `test_scripts/`) that need path or env updates."
    status: completed
    dependencies:
      - mcp-routing-step-063
  - id: mcp-routing-step-065
    content: "Add or update a failing E2E scenario for **tool call success** against control plane (or mark scenario `@wip` only if infrastructure is missing, with an explicit reason in the task report)."
    status: completed
    dependencies:
      - mcp-routing-step-064
  - id: mcp-routing-step-066
    content: "Fix env wiring in test harness so **no separate mcp-gateway** is required unless explicitly testing that binary."
    status: completed
    dependencies:
      - mcp-routing-step-065
  - id: mcp-routing-step-067
    content: "N/A unless test helpers need deduplication."
    status: completed
    dependencies:
      - mcp-routing-step-066
  - id: mcp-routing-step-068
    content: "Run targeted E2E or `just ci` per repo norms; document anything skipped."
    status: completed
    dependencies:
      - mcp-routing-step-067
  - id: mcp-routing-step-069
    content: "**Task completion report:** commands run, results, skipped tests with reasons."
    status: completed
    dependencies:
      - mcp-routing-step-068
  - id: mcp-routing-step-070
    content: "Mark Task 6 steps `- [x]`."
    status: completed
    dependencies:
      - mcp-routing-step-069
  - id: mcp-routing-step-071
    content: "Do not start Task 7 until closeout is done."
    status: completed
    dependencies:
      - mcp-routing-step-070
  - id: mcp-routing-step-072
    content: "List all user-visible env vars after Tasks 3-4."
    status: completed
    dependencies:
      - mcp-routing-step-071
  - id: mcp-routing-step-073
    content: "List ports after Task 2."
    status: completed
    dependencies:
      - mcp-routing-step-072
  - id: mcp-routing-step-074
    content: "N/A (documentation task)."
    status: completed
    dependencies:
      - mcp-routing-step-073
  - id: mcp-routing-step-075
    content: "Re-read [meta.md](../../meta.md) after implementation; confirm no regression to `db.*` naming or a mandatory separate MCP service."
    status: completed
    dependencies:
      - mcp-routing-step-074
  - id: mcp-routing-step-076
    content: "Verify [orchestrator/README.md](../../orchestrator/README.md), [scripts/README.md](../../scripts/README.md), [orchestrator/systemd/README.md](../../orchestrator/systemd/README.md), and [orchestrator/docker-compose.yml](../../orchestrator/docker-compose.yml) after **removing** the deprecated `mcp-gateway` service from compose (no separate migration doc required pre-MVP)."
    status: completed
    dependencies:
      - mcp-routing-step-075
  - id: mcp-routing-step-077
    content: "Remove duplicate paragraphs that repeat the same port guidance."
    status: completed
    dependencies:
      - mcp-routing-step-076
  - id: mcp-routing-step-078
    content: "`just docs-check`"
    status: completed
    dependencies:
      - mcp-routing-step-077
  - id: mcp-routing-step-079
    content: "`just ci` before merge to mainline."
    status: completed
    dependencies:
      - mcp-routing-step-078
  - id: mcp-routing-step-080
    content: "**Final plan completion report:** tasks completed, overall validation, remaining risks (for example **deprecated** `cmd/mcp-gateway` until deleted)."
    status: completed
    dependencies:
      - mcp-routing-step-079
  - id: mcp-routing-step-081
    content: "Mark Task 7 and any remaining unchecked boxes in this document as `- [x]` when work is truly done."
    status: completed
    dependencies:
      - mcp-routing-step-080
  - id: mcp-routing-step-082
    content: "Optionally set **Plan Status** at top to **Closed** with date."
    status: completed
    dependencies:
      - mcp-routing-step-081
---

# Orchestrator Tool Routing and MCP Consolidation: Execution Plan

- [Plan Status](#plan-status)
- [Goal](#goal)
- [References](#references)
- [Constraints](#constraints)
- [Problem Statement](#problem-statement)
- [Target Architecture](#target-architecture)
- [Scope and Spec Alignment](#scope-and-spec-alignment)
- [Risks and Open Questions](#risks-and-open-questions)
- [Execution Plan](#execution-plan)
  - [Task 1: Requirements Trace and Tool Name Inventory](#task-1-requirements-trace-and-tool-name-inventory)
  - [Task 2: Orchestrator Consolidation (Single Surface, Optional Binary)](#task-2-orchestrator-consolidation-single-surface-optional-binary)
  - [Task 3: Worker Proxy and Environment Variable Collapse](#task-3-worker-proxy-and-environment-variable-collapse)
  - [Task 4: Agent Binary (PMA and SBA) Descriptions and Defaults](#task-4-agent-binary-pma-and-sba-descriptions-and-defaults)
  - [Task 5: Orchestrator Router and Legacy `db.*` Handlers](#task-5-orchestrator-router-and-legacy-db-handlers)
  - [Task 6: End-To-End and Regression Validation](#task-6-end-to-end-and-regression-validation)
  - [Task 7: Documentation and Normative Closeout](#task-7-documentation-and-normative-closeout)
- [Appendix: Suggested Env Precedence (Draft)](#appendix-suggested-env-precedence-draft)

## Plan Status

**Created:** 2026-03-22.
**Closed:** 2026-03-23 (implementation complete; see `docs/dev_docs/2026-03-22_plan_execution_final_report.md`).
**Status:** Closed.

## Goal

Deliver a single, coherent story for **privileged agent tools**:

- **PMA** registers **langchaingo** `tools.Tool` value(s) (for example `NewMCPTool` in `agents/internal/pma/mcp_tools.go`) so the LLM can invoke **catalog operations** such as `task.*`, `help.*`, and other PM-allowlisted names; those handlers run **inside the PMA binary**, not in a separate agent-side gateway process.
  Each invocation is **translated** in-process into HTTP to the **orchestrator control-plane** tool endpoint via **`mcpclient`**, using the collapsed routing from Task 3 (for example the worker internal proxy), and the response is returned to the model, without a separate deployable **MCP gateway service** or maze of URLs for that path.
- **MCP** remains the **protocol and naming contract** (tool catalog, audit, allowlists).
  Default deployments still use the **normal** path (agent to worker proxy to orchestrator); the goal is **no redundant second gateway process or port** for the same enforcement, not zero hops.
- **Agent-facing tool names** match specs: resource-oriented names only; **no `db.*` prefixes** (replace with `preference.*`, `task.*`, and other catalog names per [MCP Tooling](../tech_specs/mcp/mcp_tooling.md)).
- **Configuration collapses** to a small set of environment variables with clear precedence, with **managed deployments** defaulting to the **worker internal proxy** (service identity) and **plain HTTP orchestrator URL** only where appropriate for dev or tests.

## References

- Requirements: [docs/requirements/mcptoo.md](../requirements/mcptoo.md), [docs/requirements/mcpgat.md](../requirements/mcpgat.md), [docs/requirements/pmagnt.md](../requirements/pmagnt.md), [docs/requirements/agents.md](../requirements/agents.md).
- Tech specs: [docs/tech_specs/mcp/mcp_tooling.md](../tech_specs/mcp/mcp_tooling.md) (agent-facing names, MCP role),
  [docs/tech_specs/mcp_tools/README.md](../tech_specs/mcp_tools/README.md),
  [docs/tech_specs/mcp_tools/preference_tools.md](../tech_specs/mcp_tools/preference_tools.md),
  [docs/tech_specs/mcp_tools/access_allowlists_and_scope.md](../tech_specs/mcp_tools/access_allowlists_and_scope.md),
  [docs/tech_specs/mcp/mcp_gateway_enforcement.md](../tech_specs/mcp/mcp_gateway_enforcement.md),
  [docs/tech_specs/ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md).
- Implementation areas: `orchestrator/cmd/control-plane/`, `orchestrator/internal/mcpgateway/`, `orchestrator/cmd/mcp-gateway/` (optional binary),
  `worker_node/internal/workerapiserver/` (internal orchestrator proxy),
  `agents/internal/mcpclient/` (HTTP client and `langchain_tool.go` bridge),
  `agents/internal/pma/` and `agents/internal/sba/` (langchaingo agents; `mcp_tools.go` registers tools that call `mcpclient`),
  `orchestrator/docker-compose.yml`, `scripts/justfile` (validation targets).

## Constraints

- Requirements and tech specs are the source of truth; where [meta.md](../../meta.md) or older docs contradict them (for example `db.*` or a separate gateway service), **update normative docs** or **meta** after REQ/spec review.
- BDD/TDD: add or update failing tests before behavior changes; each task closes with a Testing gate before the next task starts.
- Use repo `just` targets for validation (`just ci`, `just test-go-cover`, `just lint`, `just docs-check`) unless a task specifies narrower commands.
- Do not relax linters or coverage thresholds.
- Prefer minimal diffs: no drive-by refactors outside the task scope.
- **Do not link to this file** from durable documentation ([meta.md](../../meta.md) marks `dev_docs/` as non-canonical); promote outcomes into `docs/tech_specs/` or `README` files when the work completes.

## Problem Statement

- Operators and agents see **404** or **misrouted** calls when multiple bases exist (`PMA_MCP_GATEWAY_URL`, `MCP_GATEWAY_URL`, `ORCHESTRATOR_MCP_GATEWAY_BASE_URL`, separate `mcp-gateway` port).
- PMA still **documents** `db.preference.*` and similar in [agents/internal/pma/mcp_tools.go](../../agents/internal/pma/mcp_tools.go) (the **langchaingo** tool description for `NewMCPTool`), which **violates** [Agent-Facing Tool Names](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-agentfacingtoolnames) and confuses the model.
- The **standalone `mcp-gateway`** service (port **12083**) is **deprecated**; it duplicates the control-plane route and should be removed from compose when touched.

## Target Architecture

- **Single execution surface:** `POST /v1/mcp/tools/call` (or a renamed alias later) is implemented **only** on the **control plane** process, sharing DB and auth with the rest of orchestration.
- **PMA/SBA tool path:** **langchaingo** tool handlers in the agent process call **`mcpclient`**; they do not embed orchestrator URLs in prompts for the LLM to use directly.
- **Agent default path:** That **`mcpclient`** stack resolves to the **worker internal proxy** URL (`http+unix://...` + `/v1/worker/internal/orchestrator/mcp:call`) when the worker injects env; the proxy forwards to the control plane with **service identity**.
- **Dev override:** a single explicit orchestrator base URL for HTTP tests or local wiring, still hitting the **same** path on the control plane.
- **Standalone `mcp-gateway` binary (12083):** **deprecated**; remove from compose when convenient; PMA uses the control-plane route only.
- **Tool naming cleanup:** router allowlists, help strings, and tests use **`preference.*`**, **`task.*`**, etc., not `db.*`.

## Scope and Spec Alignment

- Confirm with [preference_tools.md](../tech_specs/mcp_tools/preference_tools.md) that **`preference.get`**, **`preference.list`**, and related operations replace any legacy **`db.preference.*`** routes in code and copy.
- If any requirement text still says `db.*`, open a small REQ edit or trace note so **requirements**, **specs**, and **code** match.
- [meta.md](../../meta.md) and normative specs now describe **MCP tools** and catalog names such as **`preference.*`** (not `db.*`); keep code and tests aligned with that language.

## Risks and Open Questions

- **Rename vs alias:** Whether to keep the path `/v1/mcp/tools/call` permanently (MCP as concept only) or add `/v1/agent/tools/call` as an alias; decide before large doc churn.
- **Compose cleanup:** Remove the deprecated `mcp-gateway` service from `orchestrator/docker-compose.yml` when that file is next edited (no legacy support requirement pre-MVP).
- **Dual binaries:** `cmd/mcp-gateway` remains in-tree until removed; treat it as **deprecated**, not a supported deployment path.

## Execution Plan

Execute tasks in order.
Do not start the next task until the current task's **Testing** gate and **Closeout** are complete.

---

### Task 1: Requirements Trace and Tool Name Inventory

Establish a definitive list of **allowed PM and SBA tool names** and explicitly mark **`db.*` as removed** everywhere it appears in agent-visible strings, tests, and router maps.

#### Task 1 Requirements and Specifications

- [docs/tech_specs/mcp/mcp_tooling.md](../tech_specs/mcp/mcp_tooling.md) (`CYNAI.MCPTOO.AgentFacingToolNames`).
- [docs/tech_specs/mcp_tools/preference_tools.md](../tech_specs/mcp_tools/preference_tools.md).
- [docs/tech_specs/mcp_tools/access_allowlists_and_scope.md](../tech_specs/mcp_tools/access_allowlists_and_scope.md).

#### Discovery (Task 1) Steps

- [x] Read the specifications above and list PM vs SBA allowlists as specified.
- [x] Grep the repo for `NewMCPTool`, `NewLangchainTool`, and langchaingo `tools.Tool` registration in `agents/internal/pma/` and `agents/internal/sba/` to align inventory with how catalog names are exposed to the model.
- [x] Grep the repo for `db.`, `db.preference`, and `db.task` in `agents/`, `orchestrator/`, and `docs/`; record each hit as **remove**, **rename to preference.**, or **false positive** (comments only).

#### Red (Task 1)

- [x] Add or update a short **inventory checklist** (can live in this plan or a scratch note under `docs/dev_docs/`) listing every file that must change in later tasks.
- [x] Where tests encode `db.preference.get`, add failing tests or update expectations to the **new canonical names** once those names are fixed in Task 3+ (Red may be deferred if inventory-only; if so, complete Red in Task 3).

#### Green (Task 1)

- [x] No code change required if this task is inventory-only; otherwise limit edits to **documentation of findings** only.

#### Refactor (Task 1)

- [x] N/A unless consolidating duplicate grep results into a single table-free list in the inventory.

#### Testing (Task 1)

- [x] Review inventory with a second pass: no remaining ambiguous `db.*` references in agent-facing descriptions.

#### Closeout (Task 1)

- [x] Generate a **task completion report**: inventory summary, file list for Tasks 2-5, open questions.
- [x] Mark every completed step in Task 1 with `- [x]`.
- [x] Do not start Task 2 until this closeout is done.

---

### Task 2: Orchestrator Consolidation (Single Surface, Optional Binary)

Ensure the **control plane** is the **only** required HTTP surface for tool calls in default deployments; narrow or remove standalone **mcp-gateway** from compose and docs.

#### Task 2 Requirements and Specifications

- [docs/tech_specs/mcp/mcp_tool_call_auditing.md](../tech_specs/mcp/mcp_tool_call_auditing.md) (audit on every call).
- [docs/tech_specs/ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md).

#### Discovery (Task 2) Steps

- [x] Read `orchestrator/cmd/control-plane/main.go` and confirm `POST /v1/mcp/tools/call` registration.
- [x] Read `orchestrator/cmd/mcp-gateway/main.go` and list differences from `internal/mcpgateway` usage on the control plane.
- [x] Read `orchestrator/docker-compose.yml` for `mcp-gateway` service and ports.

#### Red (Task 2)

- [x] Add or update tests that fail if the control-plane mux does **not** register the tool route (if not already covered).
- [x] Add a compose or doc test only if the repo already has a pattern for it; otherwise skip and document manual verification.

#### Green (Task 2)

- [x] Remove or disable **mcp-gateway** from the **default** compose profile, or mark it **deprecated** with comments and no published host port in dev unless needed for legacy tests.
- [x] Ensure `internal/mcpgateway` remains the **single** implementation package; `cmd/mcp-gateway` stays a thin wrapper **or** is test-only per team decision.
- [x] Update [ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md) and orchestrator README snippets so **MCP tool calls** point to **control plane** port and path.

#### Refactor (Task 2)

- [x] Deduplicate log strings and startup messages that still say "mcp-gateway" when the process is control-plane-only in production.

#### Testing (Task 2)

- [x] `go test` for `orchestrator/cmd/control-plane/...` and `orchestrator/internal/mcpgateway/...`.
- [x] `go test` for `orchestrator/cmd/mcp-gateway/...` if that binary remains.
- [x] `just docs-check` if tech specs or README changed.

#### Closeout (Task 2)

- [x] **Task completion report:** compose changes, port story, any kept legacy path for `mcp-gateway`.
- [x] Mark Task 2 steps `- [x]`.
- [x] Do not start Task 3 until closeout is done.

---

### Task 3: Worker Proxy and Environment Variable Collapse

Make the worker **internal orchestrator proxy** the **default HTTP transport** for **langchaingo** tool handlers that call **`mcpclient`** (PMA/SBA); collapse redundant **MCP base URL** env vars.

#### Task 3 Requirements and Specifications

- [docs/tech_specs/mcp/mcp_gateway_enforcement.md](../tech_specs/mcp/mcp_gateway_enforcement.md) (identity, tokens).
- Worker proxy behavior in `worker_node/internal/workerapiserver/internal_orchestrator_proxy.go`.

#### Discovery (Task 3) Steps

- [x] Read `agents/internal/mcpclient/client.go` (`NewPMClient`, `isInternalProxyMCPURL`, `callViaWorkerInternalProxy`).
- [x] Read `worker_node/internal/nodeagent/runargs.go` for injected `MCP_GATEWAY_*` env.
- [x] Read `worker_node/internal/workerapiserver/embed_handlers.go` for `ORCHESTRATOR_MCP_GATEWAY_BASE_URL` and derivation.

#### Red (Task 3)

- [x] Add failing tests if the worker derives the wrong base (for example still pointing at a separate `:12083` host) when `ORCHESTRATOR_URL` is set to the control plane.
- [x] Add tests for env precedence: explicit override wins; otherwise derive from orchestrator/control-plane URL.

#### Green (Task 3)

- [x] Implement **one precedence rule** documented in code comments, for example:
  - [x] `ORCHESTRATOR_MCP_TOOLS_BASE_URL` (optional new name) **or** keep `ORCHESTRATOR_MCP_GATEWAY_BASE_URL` as deprecated alias.
  - [x] Else worker-injected proxy URL for managed agents.
  - [x] Else `ORCHESTRATOR_URL` / internal proxy base + fixed path.
- [x] Update [scripts/README.md](../../scripts/README.md) or setup-dev notes only if they reference `mcp-gateway:12083` for PMA.

#### Refactor (Task 3)

- [x] Rename symbols in worker code from `MCPGatewayBaseURL` to `MCPToolsBaseURL` **only if** it is a localized rename with no public API break, or defer to a follow-up.

#### Testing (Task 3)

- [x] `go test ./worker_node/internal/workerapiserver/...`
- [x] `go test ./worker_node/internal/nodeagent/...` if runargs change.

#### Closeout (Task 3)

- [x] **Task completion report:** final env var list and precedence table in prose (no GFM table; use bullet list).
- [x] Mark Task 3 steps `- [x]`.
- [x] Do not start Task 4 until closeout is done.

---

### Task 4: Agent Binary (PMA and SBA) Descriptions and Defaults

Stop advertising **`db.*`** in **langchaingo** tool descriptions (`mcp_tools.go`, including PMA `NewMCPTool` copy and SBA `MCPTool`); align **`mcpclient`** defaults with Task 3 routing.

#### Task 4 Requirements and Specifications

- [docs/tech_specs/project_manager_agent.md](../tech_specs/project_manager_agent.md) (if it lists tools).
- PMA and SBA MCP tool strings in `agents/internal/pma/mcp_tools.go` and `agents/internal/sba/mcp_tools.go`.

#### Discovery (Task 4) Steps

- [x] Read PMA and SBA `mcp_tools.go` and list every **documented** tool name; read PMA `langchain.go` (and SBA wiring) for where **`NewMCPTool`** is registered on the **langchaingo** agent.
- [x] Compare to Task 1 inventory and to PM/SBA allowlists in specs.

#### Red (Task 4)

- [x] Add or update unit tests that assert the **description string** does not contain `db.` (or snapshot the allowed substring set).

#### Green (Task 4)

- [x] Replace `db.preference.*` with **`preference.*`** per [preference_tools.md](../tech_specs/mcp_tools/preference_tools.md).
- [x] Remove or rename **`db.task.get`** / **`db.job.get`** references to **`task.*`** / **`job.*`** per catalog.
- [x] Ensure `NewPMClient` / `NewSBAClient` use the collapsed env rules from Task 3.

#### Refactor (Task 4)

- [x] Centralize a single **allowlist snippet** helper if duplication between PMA and SBA becomes unmaintainable (only if minimal).

#### Testing (Task 4)

- [x] `go test ./agents/internal/pma/... ./agents/internal/sba/... ./agents/internal/mcpclient/...`

#### Closeout (Task 4)

- [x] **Task completion report:** before/after description samples, test list.
- [x] Mark Task 4 steps `- [x]`.
- [x] Do not start Task 5 until closeout is done.

---

### Task 5: Orchestrator Router and Legacy `db.*` Handlers

Remove or translate **`db.*`** routes in `mcpgateway` to canonical names; return **clear errors** for old names if clients still call them during migration.

#### Task 5 Requirements and Specifications

- [docs/tech_specs/mcp_tools/README.md](../tech_specs/mcp_tools/README.md) index.
- Router and allowlist enforcement in `orchestrator/internal/mcpgateway/handlers.go` (and related files).

#### Discovery (Task 5) Steps

- [x] Grep `orchestrator/internal/mcpgateway` for `db.` route keys or string switches.
- [x] Map each to `preference.*`, `task.*`, or explicit **not implemented** per MVP.

#### Red (Task 5)

- [x] Add tests that **`db.preference.get`** returns **404** or **structured error** (per chosen migration policy) and **`preference.get`** succeeds on the happy path where implemented.

#### Green (Task 5)

- [x] Implement routing per Red tests; prefer **one canonical name** per operation.
- [x] Update audit logging tool names to match **agent-facing** names.

#### Refactor (Task 5)

- [x] Extract shared route registration if `db.*` and `preference.*` temporarily coexist behind a compatibility layer.

#### Testing (Task 5)

- [x] `go test ./orchestrator/internal/mcpgateway/...`

#### Closeout (Task 5)

- [x] **Task completion report:** migration behavior for legacy names, test evidence.
- [x] Mark Task 5 steps `- [x]`.
- [x] Do not start Task 6 until closeout is done.

---

### Task 6: End-To-End and Regression Validation

Prove PMA **langchaingo** tool invocation (catalog names such as **help** and **task**) reaches the **control plane** through **`mcpclient`** and the **worker proxy** in a realistic dev configuration.

#### Task 6 Requirements and Specifications

- Functional coverage per [ai_files/ai_coding_instructions.md](../../ai_files/ai_coding_instructions.md) for API-facing behavior.

#### Discovery (Task 6) Steps

- [x] Identify existing E2E or BDD steps for PMA MCP (`agents/_bdd/`, `test_scripts/`) that need path or env updates.

#### Red (Task 6)

- [x] Add or update a failing E2E scenario for **tool call success** against control plane (or mark scenario `@wip` only if infrastructure is missing, with an explicit reason in the task report).

#### Green (Task 6)

- [x] Fix env wiring in test harness so **no separate mcp-gateway** is required unless explicitly testing that binary.

#### Refactor (Task 6)

- [x] N/A unless test helpers need deduplication.

#### Testing (Task 6)

- [x] Run targeted E2E or `just ci` per repo norms; document anything skipped.

#### Closeout (Task 6)

- [x] **Task completion report:** commands run, results, skipped tests with reasons.
- [x] Mark Task 6 steps `- [x]`.
- [x] Do not start Task 7 until closeout is done.

---

### Task 7: Documentation and Normative Closeout

Promote stable outcomes from `dev_docs` into durable docs; leave this plan as historical record.

**Already applied (2026-03-22):** Normative updates span [meta.md](../../meta.md), [REQ-PMAGNT-0106](../requirements/pmagnt.md#req-pmagnt-0106), [ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md), [mcp_gateway_enforcement.md](../tech_specs/mcp/mcp_gateway_enforcement.md), [mcp_tooling.md](../tech_specs/mcp/mcp_tooling.md), [cynode_pma.md](../tech_specs/cynode_pma.md), and [project_manager_agent.md](../tech_specs/project_manager_agent.md).

README and deployment notes: [orchestrator/README.md](../../orchestrator/README.md), [scripts/README.md](../../scripts/README.md), [orchestrator/systemd/README.md](../../orchestrator/systemd/README.md), and header comments in [orchestrator/docker-compose.yml](../../orchestrator/docker-compose.yml).

They state that the **orchestrator MCP gateway** is the logical enforcement point on the **control-plane** HTTP surface, and that standalone **12083** / `mcp-gateway` is **deprecated**.

#### Task 7 Requirements and Specifications

- [docs/README.md](../README.md) and [docs/tech_specs/_main.md](../tech_specs/_main.md) for discoverability.

#### Discovery (Task 7) Steps

- [x] List all user-visible env vars after Tasks 3-4.
- [x] List ports after Task 2.

#### Red (Task 7)

- [x] N/A (documentation task).

#### Green (Task 7)

- [x] Re-read [meta.md](../../meta.md) after implementation; confirm no regression to `db.*` naming or a mandatory separate MCP service.
- [x] Verify [orchestrator/README.md](../../orchestrator/README.md), [scripts/README.md](../../scripts/README.md), [orchestrator/systemd/README.md](../../orchestrator/systemd/README.md), and [orchestrator/docker-compose.yml](../../orchestrator/docker-compose.yml) after **removing** the deprecated `mcp-gateway` service from compose (no separate migration doc required pre-MVP).

#### Refactor (Task 7)

- [x] Remove duplicate paragraphs that repeat the same port guidance.

#### Testing (Task 7)

- [x] `just docs-check`
- [x] `just ci` before merge to mainline.

#### Closeout (Task 7)

- [x] **Final plan completion report:** tasks completed, overall validation, remaining risks (for example **deprecated** `cmd/mcp-gateway` until deleted).
- [x] Mark Task 7 and any remaining unchecked boxes in this document as `- [x]` when work is truly done.
- [x] Optionally set **Plan Status** at top to **Closed** with date.

---

## Appendix: Suggested Env Precedence (Draft)

This appendix is **non-normative** until Task 3 implements and documents the final list.

- **Managed worker:** prefer worker-injected **`MCP_GATEWAY_PROXY_URL`** (or equivalent) pointing at **`/v1/worker/internal/orchestrator/mcp:call`**.
- **Override:** single explicit base URL env if both agent and reviewer agree on the final variable name.
- **Derived:** trim trailing slash from **`ORCHESTRATOR_URL`** (control plane) for direct HTTP tests.
