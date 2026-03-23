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
**Status:** Draft for review (not started).
**Audience:** Implementers and reviewers before coding.

## Goal

Deliver a single, coherent story for **privileged agent tools**:

- Built-in tools are exposed by the **agent binary** (for example one LangChain tool such as `mcp_call`) and route to the **orchestrator control plane** without requiring a separate deployable **MCP gateway service** or a maze of URLs.
- **MCP** remains the **protocol and naming contract** (tool catalog, audit, allowlists), not an extra network hop by default.
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
  `agents/internal/mcpclient/`, `agents/internal/pma/`, `agents/internal/sba/`,
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
- PMA still **documents** `db.preference.*` and similar in [agents/internal/pma/mcp_tools.go](../../agents/internal/pma/mcp_tools.go), which **violates** [Agent-Facing Tool Names](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-agentfacingtoolnames) and confuses the model.
- Optional **standalone `mcp-gateway`** duplicates the control-plane route and increases operational and mental overhead without benefit for default deployments.

## Target Architecture

- **Single execution surface:** `POST /v1/mcp/tools/call` (or a renamed alias later) is implemented **only** on the **control plane** process, sharing DB and auth with the rest of orchestration.
- **Agent default path:** PMA/SBA `mcpclient` resolves to the **worker internal proxy** URL (`http+unix://...` + `/v1/worker/internal/orchestrator/mcp:call`) when the worker injects env; the proxy forwards to the control plane with **service identity**.
- **Dev override:** a single explicit orchestrator base URL for HTTP tests or local wiring, still hitting the **same** path on the control plane.
- **Standalone `mcp-gateway` binary:** either **removed from production compose** or documented as **test-only / legacy**; not required for PMA in dev.
- **Tool naming cleanup:** router allowlists, help strings, and tests use **`preference.*`**, **`task.*`**, etc., not `db.*`.

## Scope and Spec Alignment

- Confirm with [preference_tools.md](../tech_specs/mcp_tools/preference_tools.md) that **`preference.get`**, **`preference.list`**, and related operations replace any legacy **`db.preference.*`** routes in code and copy.
- If any requirement text still says `db.*`, open a small REQ edit or trace note so **requirements**, **specs**, and **code** match.
- Update [meta.md](../../meta.md) line about "MCP database tools" if it still implies `db.*` naming once implementation moves to `preference.*`.

## Risks and Open Questions

- **Rename vs alias:** Whether to keep the path `/v1/mcp/tools/call` permanently (MCP as concept only) or add `/v1/agent/tools/call` as an alias; decide before large doc churn.
- **Backward compatibility:** External clients that pointed at `mcp-gateway:12083` need a **migration note** and compose change.
- **Dual binaries:** Keeping `cmd/mcp-gateway` for isolated tests is fine; **document** that it is not part of the default stack.

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

- [ ] Read the specifications above and list PM vs SBA allowlists as specified.
- [ ] Grep the repo for `db.`, `db.preference`, and `db.task` in `agents/`, `orchestrator/`, and `docs/`; record each hit as **remove**, **rename to preference.**, or **false positive** (comments only).

#### Red (Task 1)

- [ ] Add or update a short **inventory checklist** (can live in this plan or a scratch note under `docs/dev_docs/`) listing every file that must change in later tasks.
- [ ] Where tests encode `db.preference.get`, add failing tests or update expectations to the **new canonical names** once those names are fixed in Task 3+ (Red may be deferred if inventory-only; if so, complete Red in Task 3).

#### Green (Task 1)

- [ ] No code change required if this task is inventory-only; otherwise limit edits to **documentation of findings** only.

#### Refactor (Task 1)

- [ ] N/A unless consolidating duplicate grep results into a single table-free list in the inventory.

#### Testing (Task 1)

- [ ] Review inventory with a second pass: no remaining ambiguous `db.*` references in agent-facing descriptions.

#### Closeout (Task 1)

- [ ] Generate a **task completion report**: inventory summary, file list for Tasks 2-5, open questions.
- [ ] Mark every completed step in Task 1 with `- [x]`.
- [ ] Do not start Task 2 until this closeout is done.

---

### Task 2: Orchestrator Consolidation (Single Surface, Optional Binary)

Ensure the **control plane** is the **only** required HTTP surface for tool calls in default deployments; narrow or remove standalone **mcp-gateway** from compose and docs.

#### Task 2 Requirements and Specifications

- [docs/tech_specs/mcp/mcp_tool_call_auditing.md](../tech_specs/mcp/mcp_tool_call_auditing.md) (audit on every call).
- [docs/tech_specs/ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md).

#### Discovery (Task 2) Steps

- [ ] Read `orchestrator/cmd/control-plane/main.go` and confirm `POST /v1/mcp/tools/call` registration.
- [ ] Read `orchestrator/cmd/mcp-gateway/main.go` and list differences from `internal/mcpgateway` usage on the control plane.
- [ ] Read `orchestrator/docker-compose.yml` for `mcp-gateway` service and ports.

#### Red (Task 2)

- [ ] Add or update tests that fail if the control-plane mux does **not** register the tool route (if not already covered).
- [ ] Add a compose or doc test only if the repo already has a pattern for it; otherwise skip and document manual verification.

#### Green (Task 2)

- [ ] Remove or disable **mcp-gateway** from the **default** compose profile, or mark it **deprecated** with comments and no published host port in dev unless needed for legacy tests.
- [ ] Ensure `internal/mcpgateway` remains the **single** implementation package; `cmd/mcp-gateway` stays a thin wrapper **or** is test-only per team decision.
- [ ] Update [ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md) and orchestrator README snippets so **MCP tool calls** point to **control plane** port and path.

#### Refactor (Task 2)

- [ ] Deduplicate log strings and startup messages that still say "mcp-gateway" when the process is control-plane-only in production.

#### Testing (Task 2)

- [ ] `go test` for `orchestrator/cmd/control-plane/...` and `orchestrator/internal/mcpgateway/...`.
- [ ] `go test` for `orchestrator/cmd/mcp-gateway/...` if that binary remains.
- [ ] `just docs-check` if tech specs or README changed.

#### Closeout (Task 2)

- [ ] **Task completion report:** compose changes, port story, any kept legacy path for `mcp-gateway`.
- [ ] Mark Task 2 steps `- [x]`.
- [ ] Do not start Task 3 until closeout is done.

---

### Task 3: Worker Proxy and Environment Variable Collapse

Make the worker **internal orchestrator proxy** the **default** path for MCP tool calls from managed agents; collapse redundant **MCP base URL** env vars.

#### Task 3 Requirements and Specifications

- [docs/tech_specs/mcp/mcp_gateway_enforcement.md](../tech_specs/mcp/mcp_gateway_enforcement.md) (identity, tokens).
- Worker proxy behavior in `worker_node/internal/workerapiserver/internal_orchestrator_proxy.go`.

#### Discovery (Task 3) Steps

- [ ] Read `agents/internal/mcpclient/client.go` (`NewPMClient`, `isInternalProxyMCPURL`, `callViaWorkerInternalProxy`).
- [ ] Read `worker_node/internal/nodeagent/runargs.go` for injected `MCP_GATEWAY_*` env.
- [ ] Read `worker_node/internal/workerapiserver/embed_handlers.go` for `ORCHESTRATOR_MCP_GATEWAY_BASE_URL` and derivation.

#### Red (Task 3)

- [ ] Add failing tests if the worker derives the wrong base (for example still pointing at a separate `:12083` host) when `ORCHESTRATOR_URL` is set to the control plane.
- [ ] Add tests for env precedence: explicit override wins; otherwise derive from orchestrator/control-plane URL.

#### Green (Task 3)

- [ ] Implement **one precedence rule** documented in code comments, for example:
  - [ ] `ORCHESTRATOR_MCP_TOOLS_BASE_URL` (optional new name) **or** keep `ORCHESTRATOR_MCP_GATEWAY_BASE_URL` as deprecated alias.
  - [ ] Else worker-injected proxy URL for managed agents.
  - [ ] Else `ORCHESTRATOR_URL` / internal proxy base + fixed path.
- [ ] Update [scripts/README.md](../../scripts/README.md) or setup-dev notes only if they reference `mcp-gateway:12083` for PMA.

#### Refactor (Task 3)

- [ ] Rename symbols in worker code from `MCPGatewayBaseURL` to `MCPToolsBaseURL` **only if** it is a localized rename with no public API break, or defer to a follow-up.

#### Testing (Task 3)

- [ ] `go test ./worker_node/internal/workerapiserver/...`
- [ ] `go test ./worker_node/internal/nodeagent/...` if runargs change.

#### Closeout (Task 3)

- [ ] **Task completion report:** final env var list and precedence table in prose (no GFM table; use bullet list).
- [ ] Mark Task 3 steps `- [x]`.
- [ ] Do not start Task 4 until closeout is done.

---

### Task 4: Agent Binary (PMA and SBA) Descriptions and Defaults

Stop advertising **`db.*`** in tool descriptions; align **defaults** with Task 3 routing.

#### Task 4 Requirements and Specifications

- [docs/tech_specs/project_manager_agent.md](../tech_specs/project_manager_agent.md) (if it lists tools).
- PMA and SBA MCP tool strings in `agents/internal/pma/mcp_tools.go` and `agents/internal/sba/mcp_tools.go`.

#### Discovery (Task 4) Steps

- [ ] Read PMA and SBA `mcp_tools.go` and list every **documented** tool name.
- [ ] Compare to Task 1 inventory and to PM/SBA allowlists in specs.

#### Red (Task 4)

- [ ] Add or update unit tests that assert the **description string** does not contain `db.` (or snapshot the allowed substring set).

#### Green (Task 4)

- [ ] Replace `db.preference.*` with **`preference.*`** per [preference_tools.md](../tech_specs/mcp_tools/preference_tools.md).
- [ ] Remove or rename **`db.task.get`** / **`db.job.get`** references to **`task.*`** / **`job.*`** per catalog.
- [ ] Ensure `NewPMClient` / `NewSBAClient` use the collapsed env rules from Task 3.

#### Refactor (Task 4)

- [ ] Centralize a single **allowlist snippet** helper if duplication between PMA and SBA becomes unmaintainable (only if minimal).

#### Testing (Task 4)

- [ ] `go test ./agents/internal/pma/... ./agents/internal/sba/... ./agents/internal/mcpclient/...`

#### Closeout (Task 4)

- [ ] **Task completion report:** before/after description samples, test list.
- [ ] Mark Task 4 steps `- [x]`.
- [ ] Do not start Task 5 until closeout is done.

---

### Task 5: Orchestrator Router and Legacy `db.*` Handlers

Remove or translate **`db.*`** routes in `mcpgateway` to canonical names; return **clear errors** for old names if clients still call them during migration.

#### Task 5 Requirements and Specifications

- [docs/tech_specs/mcp_tools/README.md](../tech_specs/mcp_tools/README.md) index.
- Router and allowlist enforcement in `orchestrator/internal/mcpgateway/handlers.go` (and related files).

#### Discovery (Task 5) Steps

- [ ] Grep `orchestrator/internal/mcpgateway` for `db.` route keys or string switches.
- [ ] Map each to `preference.*`, `task.*`, or explicit **not implemented** per MVP.

#### Red (Task 5)

- [ ] Add tests that **`db.preference.get`** returns **404** or **structured error** (per chosen migration policy) and **`preference.get`** succeeds on the happy path where implemented.

#### Green (Task 5)

- [ ] Implement routing per Red tests; prefer **one canonical name** per operation.
- [ ] Update audit logging tool names to match **agent-facing** names.

#### Refactor (Task 5)

- [ ] Extract shared route registration if `db.*` and `preference.*` temporarily coexist behind a compatibility layer.

#### Testing (Task 5)

- [ ] `go test ./orchestrator/internal/mcpgateway/...`

#### Closeout (Task 5)

- [ ] **Task completion report:** migration behavior for legacy names, test evidence.
- [ ] Mark Task 5 steps `- [x]`.
- [ ] Do not start Task 6 until closeout is done.

---

### Task 6: End-To-End and Regression Validation

Prove PMA can call **help** and **task** tools through the **worker proxy** to the **control plane** in a realistic dev configuration.

#### Task 6 Requirements and Specifications

- Functional coverage per [ai_files/ai_coding_instructions.md](../../ai_files/ai_coding_instructions.md) for API-facing behavior.

#### Discovery (Task 6) Steps

- [ ] Identify existing E2E or BDD steps for PMA MCP (`agents/_bdd/`, `test_scripts/`) that need path or env updates.

#### Red (Task 6)

- [ ] Add or update a failing E2E scenario for **tool call success** against control plane (or mark scenario `@wip` only if infrastructure is missing, with an explicit reason in the task report).

#### Green (Task 6)

- [ ] Fix env wiring in test harness so **no separate mcp-gateway** is required unless explicitly testing that binary.

#### Refactor (Task 6)

- [ ] N/A unless test helpers need deduplication.

#### Testing (Task 6)

- [ ] Run targeted E2E or `just ci` per repo norms; document anything skipped.

#### Closeout (Task 6)

- [ ] **Task completion report:** commands run, results, skipped tests with reasons.
- [ ] Mark Task 6 steps `- [x]`.
- [ ] Do not start Task 7 until closeout is done.

---

### Task 7: Documentation and Normative Closeout

Promote stable outcomes from `dev_docs` into durable docs; leave this plan as historical record.

#### Task 7 Requirements and Specifications

- [docs/README.md](../README.md) and [docs/tech_specs/_main.md](../tech_specs/_main.md) for discoverability.

#### Discovery (Task 7) Steps

- [ ] List all user-visible env vars after Tasks 3-4.
- [ ] List ports after Task 2.

#### Red (Task 7)

- [ ] N/A (documentation task).

#### Green (Task 7)

- [ ] Update [meta.md](../../meta.md) if it still implies `db.*` or a separate MCP service as mandatory.
- [ ] Update [docs/tech_specs/ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md) and orchestrator README to match **single control-plane** tool endpoint.
- [ ] Add a short **migration** subsection for operators who used `mcp-gateway` on **12083** (bullet list, no table).

#### Refactor (Task 7)

- [ ] Remove duplicate paragraphs that repeat the same port guidance.

#### Testing (Task 7)

- [ ] `just docs-check`
- [ ] `just ci` before merge to mainline.

#### Closeout (Task 7)

- [ ] **Final plan completion report:** tasks completed, overall validation, remaining risks (for example optional `cmd/mcp-gateway` binary).
- [ ] Mark Task 7 and any remaining unchecked boxes in this document as `- [x]` when work is truly done.
- [ ] Optionally set **Plan Status** at top to **Closed** with date.

---

## Appendix: Suggested Env Precedence (Draft)

This appendix is **non-normative** until Task 3 implements and documents the final list.

- **Managed worker:** prefer worker-injected **`MCP_GATEWAY_PROXY_URL`** (or equivalent) pointing at **`/v1/worker/internal/orchestrator/mcp:call`**.
- **Override:** single explicit base URL env if both agent and reviewer agree on the final variable name.
- **Derived:** trim trailing slash from **`ORCHESTRATOR_URL`** (control plane) for direct HTTP tests.
