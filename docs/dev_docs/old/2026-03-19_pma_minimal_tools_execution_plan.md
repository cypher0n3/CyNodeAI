# PMA Minimal Tools: Detailed Execution Plan

- [Plan Status](#plan-status)
- [Goal](#goal)
- [References](#references)
- [Constraints](#constraints)
- [Clarifications and Assumptions](#clarifications-and-assumptions)
- [Execution Plan](#execution-plan)

## Plan Status

**Created:** 2026-03-19.
**Closed out:** 2026-03-21 (see [Implementation summary](#implementation-summary-2026-03-21)).
**Scope:** Build minimal necessary MCP tools so the Project Manager Agent can use them: task operations that already exist via the orchestrator User API Gateway, plus the basic help tool.
Shareable hosting in agents or go_shared_libs where it makes sense.

## Goal

Deliver the minimal necessary tool surface for PMA so the agent can:

- Perform task operations that already exist via the orchestrator REST API: list tasks, get task, get task result, cancel task, and get task logs (in addition to the existing `task.get`).
- Resolve project context via `project.get` and `project.list` (authorized set per user/scope).
- Call a basic help tool (`help.get`) for on-demand documentation.
- Use a shared MCP client where both PMA and SBA (sandbox agent) can share the same gateway-call logic.

Implementation must live in the right places: MCP tool handlers in the orchestrator MCP gateway; shared client code in a common space (agents) so it can be reused by PMA and SBA.

## References

- Requirements: [docs/requirements/agents.md](../../requirements/agents.md) (REQ-AGENTS-0109, 0110, 0137),
  [docs/requirements/pmagnt.md](../../requirements/pmagnt.md) (startup context, task/job tools),
  [docs/requirements/mcptoo.md](../../requirements/mcptoo.md) (REQ-MCPTOO-0116 help tools),
  [docs/requirements/mcpgat.md](../../requirements/mcpgat.md) (allowlists, scoped ids).
- Tech specs: [docs/tech_specs/project_manager_agent.md](../../tech_specs/project_manager_agent.md),
  [docs/tech_specs/mcp_tools/](../../tech_specs/mcp_tools/README.md),
  [docs/tech_specs/mcp_tools/access_allowlists_and_scope.md](../../tech_specs/mcp_tools/access_allowlists_and_scope.md),
  [docs/tech_specs/mcp/mcp_gateway_enforcement.md](../../tech_specs/mcp/mcp_gateway_enforcement.md),
  [docs/tech_specs/mcp/mcp_tooling.md](../../tech_specs/mcp/mcp_tooling.md),
  [docs/tech_specs/user_api_gateway.md](../../tech_specs/user_api_gateway.md),
  [docs/tech_specs/cli_management_app_commands_tasks.md](../../tech_specs/cli_management_app_commands_tasks.md).
- Implementation: `orchestrator/cmd/mcp-gateway/main.go`, `orchestrator/internal/handlers/tasks.go`,
  `orchestrator/internal/database/`, `agents/internal/pma/` (mcp_client, mcp_tools),
  `agents/internal/sba/` (mcp_client, mcp_tools), `go_shared_libs/` (contracts only).

## Constraints

- Requirements and tech specs are source of truth; implementation is brought into compliance.
- BDD/TDD: add or update failing tests before implementation; do not proceed to the next task until the current task's Testing gate and Closeout are complete.
- Use repo just targets: `just ci`, `just test-go-cover`, `just lint`, `just docs-check`; E2E as needed for PMA chat with MCP tools.
- Shareable code: MCP gateway client used by both PMA and SBA MUST live in a single place under `agents/` (e.g. `agents/internal/mcpclient` or `agents/internal/shared`).
  Do not add MCP client to `go_shared_libs` unless both orchestrator and worker_node need to call the gateway from non-agent code (current spec: agents call MCP; worker proxy forwards; so agent-side client only).
- Do not modify Makefiles or Justfiles unless explicitly directed.
  Do not relax linter rules.

## Clarifications and Assumptions

The following are fixed for this plan so there is no ambiguity:

- **Task list scope:** `task.list` requires `user_id` (UUID) to scope the list, matching the existing User API Gateway behavior (`ListTasksByUser`).
  PMA obtains `user_id` from the thread or request context when invoking the tool.
  Optional args: `limit`, `offset` (or `cursor`), `status` (filter).
  Response shape aligns with gateway list response (task_id, status, planning_state, task_name when present, etc.).
- **Help tool:** `help.get` is implemented per [Help tools](../../tech_specs/mcp_tools/help_tools.md): required arg `task_id` (for context and auditing), optional `topic` and `path`.
  Response is size-limited documentation content (e.g. markdown); no secrets.
  **Help content source (MVP):** Content is sourced from **embedded strings or a small in-process map** in the MCP gateway binary only (no file system or external docs at runtime).
  When `topic`/`path` are omitted, return a single default overview string (e.g. how to use MCP tools, gateway usage, task/project context).
  When `topic` is provided, return a predefined snippet for that topic key if present, else the default overview.
  A spec update (mcp_tooling.md or mcp_tools/) SHOULD state this explicitly to remove ambiguity.
- **Minimal set in scope:** This plan includes (1) task operations that already exist via the orchestrator API (list, get, result, cancel, logs), (2) the basic help tool, and (3) **`project.get` and `project.list`**.
  It does **not** include: `system_setting.get`/`list`, `task.create`/`task.update`, or other catalog tools beyond the minimal set.
  Those can be follow-up work.
- **Existing `task.get`:** Already implemented in the MCP gateway; no change in this plan except to ensure it remains on the PM allowlist and is covered by tests.
- **Naming:** Tool names are agent-facing and resource-oriented (e.g. `project.get`, `task.get`); no `db.` or other implementation-layer prefix (see [MCP Tooling - Agent-facing tool names](../../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-agentfacingtoolnames)).
  New tools in this plan: `task.list`, `task.result`, `task.cancel`, `task.logs`, `help.get`, `project.get`, `project.list`.
  Catalog documents the full minimal set with argument schemas.

- **Task cancel and stop-job:** `task.cancel` MUST trigger the **same cancel-and-stop-job path** as the User API Gateway (e.g. `CancelTask` in `orchestrator/internal/handlers/tasks.go`): update task status to canceled, update non-terminal job statuses in the DB, and when the orchestrator implements sending stop-job to the worker for active jobs, the MCP handler MUST use that same path (shared helper or identical sequence).
  No separate "MCP-only" cancel that only updates DB.

## Execution Plan

Execute tasks in order.
Each task is self-contained with Discovery, Red, Green, Refactor, Testing, and Closeout.
Do not start a later task until the current task's Testing gate and Closeout are complete.

---

### Task 1: Implement `help.get` in the MCP Gateway

Implement the basic help tool so PMA (and other allowlisted agents) can request on-demand documentation.
Read-only; no state changes.

#### Task 1 Requirements and Specifications

- [docs/tech_specs/mcp_tools/](../../tech_specs/mcp_tools/README.md) (CYNAI.MCPTOO.HelpTools): `help.get` with required `task_id`, optional `topic`, `path`.
- [docs/tech_specs/mcp/mcp_tooling.md](../../tech_specs/mcp/mcp_tooling.md) (Help MCP Server): purpose, read-only, allowlists, size-limited, no secrets.
- [docs/requirements/mcptoo.md](../../requirements/mcptoo.md) REQ-MCPTOO-0116.
- [docs/tech_specs/mcp_tools/access_allowlists_and_scope.md](../../tech_specs/mcp_tools/access_allowlists_and_scope.md) (PM Agent allowlist includes `help.*`).

#### Discovery (Task 1) Steps

- [ ] Read the requirements and specs listed in Task 1 Requirements and Specifications.
- [ ] Inspect `orchestrator/cmd/mcp-gateway/main.go`: `requiredScopedIds`, `routeToolCall` switch, audit record handling.
- [ ] Confirm `help.get` is not yet implemented (returns 501 Not Implemented or equivalent).
- [ ] Confirm help content source per plan: embedded strings or in-process map in the mcp-gateway binary only; default overview when topic/path omitted; optional topic key for predefined snippets.
  If [mcp_tooling.md](../../tech_specs/mcp/mcp_tooling.md) or [mcp_tools/](../../tech_specs/mcp_tools/README.md) does not state this, add a spec update step in Green to document it.

#### Red (Task 1)

- [ ] Add or update tests that call `POST /v1/mcp/tools/call` with `tool_name: "help.get"` and assert:
  - [ ] Missing `task_id` returns 400 with "task_id required".
  - [ ] Valid `task_id` (and optional `topic`/`path`) returns 200 and a non-empty, size-limited response body (no secrets).
  - [ ] Audit record is written with allow, success, and task_id.
- [ ] Run the new or updated tests and confirm they fail before implementation.
- [ ] Validation gate: do not proceed until failing tests prove the gap.

#### Green (Task 1)

- [ ] Add `help.get` to `requiredScopedIds` with `TaskID: true`.
- [ ] Add case `"help.get"` in `routeToolCall`; implement `handleHelpGet(ctx, store, args, rec)` that returns documentation content from **embedded strings or a small in-process map only** (no file system or external docs).
  Default overview when topic/path omitted; when `topic` is provided, return predefined snippet for that key if present, else default.
  Enforce response size limit (e.g. 32 KiB) and no secrets.
- [ ] If the tech spec does not already state the help content source (embedded/in-process only for MVP), update [mcp_tooling.md](../../tech_specs/mcp/mcp_tooling.md) or [mcp_tools/](../../tech_specs/mcp_tools/README.md) to add one sentence: e.g. "For MVP, help content is sourced from embedded strings or an in-process map in the MCP gateway; no file system or external docs at runtime."
- [ ] Run the targeted tests until they pass.
- [ ] Validation gate: do not proceed until tests are green.

#### Refactor (Task 1)

- [ ] Extract help content or lookup logic into a clear, testable helper if needed; keep main handler thin.
  Re-run tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 1)

- [ ] Run `just test-go-cover` for the mcp-gateway package (and any new test files).
- [ ] Run `just lint` for changed files.
- [ ] Confirm implementation matches mcp_tools and mcp_tooling for help.get.
- [ ] Validation gate: do not start Task 2 until all Task 1 checks pass.

#### Closeout (Task 1)

- [ ] Generate a **task completion report** for Task 1: what was done (help.get handler, requiredScopedIds, tests), what passed, any deviations or notes.
- [ ] Mark every completed step in this task's section with `- [x]`.
  Do not start Task 2 until this closeout is done.

---

### Task 2: Implement Task MCP Tools (List, Result, Cancel, Logs) in the MCP Gateway

Add MCP tools that mirror the existing orchestrator task API so PMA can list tasks, get task result, cancel a task, and get task logs using the same backend as the User API Gateway (store). `task.get` already exists; add `task.list`, `task.result`, `task.cancel`, `task.logs`.

#### Task 2 Requirements and Specifications

- [docs/tech_specs/mcp_tools/](../../tech_specs/mcp_tools/README.md) (Database Tools, task scoping, common argument requirements).
- [docs/tech_specs/mcp_tools/access_allowlists_and_scope.md](../../tech_specs/mcp_tools/access_allowlists_and_scope.md) (PM Agent allowlist: resource tools such as `task.*`, `project.*`).
- [docs/tech_specs/user_api_gateway.md](../../tech_specs/user_api_gateway.md) (task list, get, result, cancel, logs).
- [docs/tech_specs/project_manager_agent.md](../../tech_specs/project_manager_agent.md) (startup context: task and job read/write tools).
- Implementation: `orchestrator/internal/handlers/tasks.go` (ListTasks, GetTaskResult, CancelTask, GetTaskLogs), `orchestrator/internal/database/` (ListTasksByUser, GetTaskByID, GetJobsByTaskID, UpdateTaskStatus, ListArtifactPathsByTaskID, job result aggregation), `orchestrator/internal/models` (Task, Job status), `go_shared_libs/contracts/userapi` (TaskResponse, ListTasksResponse, etc.) if response shapes are shared.

#### Discovery (Task 2) Steps

- [ ] Read the requirements and specs listed in Task 2 Requirements and Specifications.
- [ ] Inspect `orchestrator/internal/handlers/tasks.go`: how ListTasks, GetTaskResult, CancelTask, GetTaskLogs use the store and what response shapes they produce.
- [ ] Inspect `orchestrator/internal/database/` for ListTasksByUser, GetTaskByID, GetJobsByTaskID, UpdateTaskStatus, and any logs/artifact helpers used by GetTaskLogs.
- [ ] Confirm MCP gateway has access to the same Store interface; identify any missing store methods (e.g. for task logs) and add them if necessary.
- [ ] Define exact argument schemas for each new tool:
  - `task.list`: required `user_id` (UUID); optional `limit` (default 50, cap 200), `offset` or `cursor`, `status` (filter).
  - `task.result`: required `task_id`; response includes task status, jobs, stdout/stderr when terminal.
  - `task.cancel`: required `task_id`.
    MUST trigger the **same cancel-and-stop-job path** as the User API Gateway: update task status to canceled, update job statuses in DB, and send stop-job to worker for active jobs when the orchestrator implements it (shared helper or same sequence as `CancelTask`).
  - `task.logs`: required `task_id`; optional `stream` (stdout/stderr/all).
    Return log lines; size-limited.

#### Red (Task 2)

- [ ] Add or update tests for each new tool in `orchestrator/cmd/mcp-gateway/`:
  - [ ] `task.list`: missing user_id -> 400; valid user_id returns 200 with tasks array (and next_cursor when applicable); status filter applied.
  - [ ] `task.result`: missing task_id -> 400; not found -> 404; valid task_id returns 200 with task and jobs/result shape.
  - [ ] `task.cancel`: missing task_id -> 400; not found -> 404; valid task_id returns 200 and task status updated to canceled.
  - [ ] `task.logs`: missing task_id -> 400; not found -> 404; valid task_id returns 200 with lines; stream param respected.
- [ ] Run the new or updated tests and confirm they fail before implementation.
- [ ] Validation gate: do not proceed until failing tests prove the gap.

#### Green (Task 2)

- [ ] Add each tool to `requiredScopedIds` (task_id for result, cancel, logs; user_id for list).
- [ ] Implement `handleTaskList`, `handleTaskResult`, `handleTaskCancel`, `handleTaskLogs` in mcp-gateway, reusing store calls and response shapes consistent with the user-gateway task handlers.
  For **cancel**, invoke the same cancel-and-stop-job path as the gateway (extract shared logic from `CancelTask` if needed so MCP and HTTP handler both use it).
  Do not duplicate business logic; call the same store methods and build the same response structures.
- [ ] Add the four cases to `routeToolCall`.
- [ ] Run the targeted tests until they pass.
- [ ] Validation gate: do not proceed until tests are green.

#### Refactor (Task 2)

- [ ] If response building is duplicated between user-gateway handlers and mcp-gateway, extract a shared helper (e.g. in `orchestrator/internal/handlers` or `orchestrator/internal/taskresponse`) so both use the same types and logic.
  Keep tests green.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 2)

- [ ] Run `just test-go-cover` for orchestrator (mcp-gateway and handlers if touched).
- [ ] Run `just lint` for changed files.
- [ ] Optionally run E2E that exercises PMA chat with MCP tools (e.g. task list, task get, task cancel) if such tests exist or are added in a later task.
- [ ] Validation gate: do not start Task 3 until all Task 2 checks pass.

#### Closeout (Task 2)

- [ ] Generate a **task completion report** for Task 2: what was done (four new handlers, requiredScopedIds, tests), what passed, any deviations or notes.
- [ ] Mark every completed step in this task's section with `- [x]`.
  Do not start Task 3 until this closeout is done.

---

### Task 3: Implement `project.get` and `project.list` in the MCP Gateway

Add MCP tools so PMA can resolve project context (by id or slug) and list projects the caller is authorized to see.
Per catalog: project tools return only projects in the caller's authorized set; list is size-limited and paginated.

#### Task 3 Requirements and Specifications

- [docs/tech_specs/mcp_tools/project_tools.md](../../tech_specs/mcp_tools/project_tools.md) (Project tools: project.get, project.list; required/optional args, authorized set).
- [docs/tech_specs/mcp_tools/access_allowlists_and_scope.md](../../tech_specs/mcp_tools/access_allowlists_and_scope.md) (PM Agent allowlist: resource tools such as `task.*`, `project.*`).
- [docs/tech_specs/projects_and_scopes.md](../../tech_specs/projects_and_scopes.md) (project model, scope, RBAC).
- Implementation: `orchestrator/internal/database/` (Store methods for projects if any), or user-gateway project handlers; confirm how "authorized set" is resolved for MCP (e.g. user_id in args for list/get when PM token has no bound user).

#### Discovery (Task 3) Steps

- [ ] Read the requirements and specs listed in Task 3 Requirements and Specifications.
- [ ] Inspect catalog: `project.get` requires `project_id` (uuid) or `slug` (text), exactly one; `project.list` optional `q`, `limit`, `cursor`; only authorized projects returned.
- [ ] Identify how the User API Gateway or data layer resolves "authorized projects" for a user (e.g. default project + RBAC).
  For MCP, determine how to pass scope: e.g. required `user_id` for list and for get-by-slug when slug is ambiguous, or context from request.
  Document the chosen contract in the plan or spec.
- [ ] Confirm Store (or equivalent) has GetProjectByID, GetProjectBySlug, ListProjects (with user/scope) or equivalent; add if missing.

#### Red (Task 3)

- [ ] Add or update tests for `project.get` and `project.list`: missing required args -> 400; not found or not authorized -> 404; valid args return 200 with project(s); list is paginated and size-limited.
- [ ] Run the new or updated tests and confirm they fail before implementation.
- [ ] Validation gate: do not proceed until failing tests prove the gap.

#### Green (Task 3)

- [ ] Add `project.get` and `project.list` to `requiredScopedIds` (define required args per catalog; if user_id is needed for scope, add it).
- [ ] Implement `handleProjectGet` and `handleProjectList` in mcp-gateway; reuse store or gateway logic for authorized-set resolution.
  Return response shapes consistent with catalog (project id, slug, display_name, description, etc.).
- [ ] Add both cases to `routeToolCall`.
- [ ] Run the targeted tests until they pass.
- [ ] Validation gate: do not proceed until tests are green.

#### Refactor (Task 3)

- [ ] Extract shared project response building if duplicated between gateway and mcp-gateway.
  Keep tests green.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 3)

- [ ] Run `just test-go-cover` for orchestrator (mcp-gateway).
  Run `just lint` for changed files.
- [ ] Validation gate: do not start Task 4 until all Task 3 checks pass.

#### Closeout (Task 3)

- [ ] Generate a **task completion report** for Task 3: what was done (project get/list handlers, requiredScopedIds, tests), what passed, any deviations or notes.
- [ ] Mark every completed step in this task's section with `- [x]`.
  Do not start Task 4 until this closeout is done.

---

### Task 4: Extract Shared MCP Client for Agents (PMA and SBA)

Move the MCP gateway client and the generic MCP tool wrapper into a shared package under `agents/` so both PMA and SBA use the same code.
This avoids duplication and ensures consistent behavior (URL, path, request/response shape, error handling).

#### Task 4 Requirements and Specifications

- [docs/tech_specs/mcp/mcp_gateway_enforcement.md](../../tech_specs/mcp/mcp_gateway_enforcement.md) (gateway is the enforcement point; agents call via client).
- [docs/tech_specs/project_manager_agent.md](../../tech_specs/project_manager_agent.md) (PMA uses MCP tools via gateway).
- [docs/tech_specs/cynode_sba.md](../../tech_specs/cynode_sba.md) (SBA uses MCP tools via worker proxy to gateway).
- Implementation: `agents/internal/pma/mcp_client.go`, `agents/internal/pma/mcp_tools.go`, `agents/internal/sba/mcp_client.go`, `agents/internal/sba/mcp_tools.go`.
  Target: single shared package e.g. `agents/internal/mcpclient` (or `agents/internal/shared/mcp`).

#### Discovery (Task 4) Steps

- [ ] Read the requirements and specs listed in Task 4 Requirements and Specifications.
- [ ] Diff `agents/internal/pma/mcp_client.go` and `agents/internal/sba/mcp_client.go`: identify differences (e.g. base URL env var name, path, timeouts).
  Decide on a single contract (e.g. one struct, configurable base URL and optional auth).
- [ ] Diff `agents/internal/pma/mcp_tools.go` and `agents/internal/sba/mcp_tools.go`: both provide a langchaingo tool that takes JSON `{tool_name, arguments}` and call the client.
  Description text differs (PM allowlist vs worker allowlist).
  Plan: shared client + shared generic MCP tool type; each agent passes its own description string or a constructor that takes description.
- [ ] Confirm no other packages in the repo import the pma/sba MCP client or tools; update import paths when moving.

#### Red (Task 4)

- [ ] Add a new package `agents/internal/mcpclient` (or chosen name) with Client and Call method, and a generic Tool that takes client + description.
  Add unit tests for the client (e.g. with httptest.Server) that verify request body and response handling.
- [ ] Add or update tests in `agents/internal/pma` and `agents/internal/sba` that use the shared client/tool and assert behavior unchanged (or improved).
  Run tests and confirm pma/sba now depend on the shared package and tests pass after migration.
- [ ] Validation gate: do not proceed until tests prove the shared package works and agents use it.

#### Green (Task 4)

- [ ] Implement the shared client in `agents/internal/mcpclient`: struct with BaseURL, Call(ctx, toolName, arguments) (body []byte, statusCode int, err error).
  Match current pma/sba behavior (JSON request/response, error handling).
- [ ] Implement a shared langchaingo Tool type (e.g. `NewMCPTool(client *Client, description string)`) that forwards to client and returns the tool description.
  Move or reuse the same logic as in pma and sba.
- [ ] Update `agents/internal/pma` to use the shared client and tool; pass PM-specific description.
  Remove or thin `pma/mcp_client.go` and `pma/mcp_tools.go` to wrappers or delete and use shared.
- [ ] Update `agents/internal/sba` to use the shared client and tool; pass SBA-specific description.
  Remove or thin `sba/mcp_client.go` and `sba/mcp_tools.go` similarly.
- [ ] Run all agent tests (`just test-go-cover` for agents module) until they pass.
- [ ] Validation gate: do not proceed until tests are green.

#### Refactor (Task 4)

- [ ] Clean up any duplicated comments or env var handling; ensure both PMA and SBA configs (e.g. PMA_MCP_GATEWAY_URL vs MCP_GATEWAY_URL) are documented and work with the shared client.
  Keep tests green.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 4)

- [ ] Run `just test-go-cover` for the agents module.
- [ ] Run `just lint` for agents.
- [ ] Validation gate: do not start Task 5 until all Task 4 checks pass.

#### Closeout (Task 4)

- [ ] Generate a **task completion report** for Task 4: what was done (shared mcpclient package, pma/sba migrated), what passed, any deviations or notes.
- [ ] Mark every completed step in this task's section with `- [x]`.
  Do not start Task 5 until this closeout is done.

---

### Task 5: Catalog, Allowlist, and PMA Tool Description Updates

Update [MCP tool specifications](../../tech_specs/mcp_tools/README.md) (e.g. [Task tools](../../tech_specs/mcp_tools/task_tools.md) and [Project tools](../../tech_specs/mcp_tools/project_tools.md)) to document the new task tools (`task.list`, `task.result`, `task.cancel`, `task.logs`) and project tools (`project.get`, `project.list`), and ensure the PM allowlist and PMA tool description (in code) are accurate and complete for the minimal set.

#### Task 5 Requirements and Specifications

- [docs/tech_specs/mcp_tools/](../../tech_specs/mcp_tools/README.md) (Database Tools, Help Tools).
- [docs/tech_specs/mcp_tools/access_allowlists_and_scope.md](../../tech_specs/mcp_tools/access_allowlists_and_scope.md) (Project Manager Agent allowlist).
- [docs/tech_specs/project_manager_agent.md](../../tech_specs/project_manager_agent.md) (startup context, tool access).
- Implementation: `agents/internal/pma/mcp_tools.go` (or shared tool) description string; `orchestrator/cmd/mcp-gateway/main.go` (requiredScopedIds is the implemented set).

#### Discovery (Task 5) Steps

- [x] Read the requirements and specs listed in Task 5 Requirements and Specifications.
- [x] Open [mcp_tools/task_tools.md](../../tech_specs/mcp_tools/task_tools.md): Task Tools section lists task.get and task.update_status as minimum; add task.list, task.result, task.cancel, task.logs with required/optional args and behavior.
  Open [mcp_tools/project_tools.md](../../tech_specs/mcp_tools/project_tools.md): ensure project.get and project.list are documented.
  Help Tools section already describes help.get; ensure it matches implementation.
- [x] Confirm [access_allowlists_and_scope.md](../../tech_specs/mcp_tools/access_allowlists_and_scope.md) PM allowlist already includes resource tools and `help.*`; no change needed unless a new namespace was introduced.
- [x] Update the PMA (or shared) tool description string so the LLM sees the full minimal set: task.get, task.list, task.result, task.cancel, task.logs, project.get, project.list, help.get, and existing preference.*, job.get, artifact.get, skills.*.

#### Red (Task 5)

- [x] N/A for code-only changes to docs and description string.
  If BDD scenarios or E2E exist that assert tool names or descriptions, add or update them and run to confirm current state.
- [x] Validation gate: proceed to Green.

#### Green (Task 5)

- [x] Edit [mcp_tools/task_tools.md](../../tech_specs/mcp_tools/task_tools.md): add entries for `task.list` (required user_id; optional limit, offset/cursor, status), `task.result` (required task_id), `task.cancel` (required task_id), `task.logs` (required task_id; optional stream).
  Keep existing task.get and task.update_status; ensure project.get and project.list are documented per catalog.
  Align wording with "task operations that exist via User API Gateway."
- [x] Ensure Help Tools section for help.get matches implementation (required task_id; optional topic, path; size-limited; no secrets; content source: embedded/in-process only for MVP).
- [x] Update the tool description in the agents code (PMA tool or shared MCP tool constructor) to list the minimal set including task.list, task.result, task.cancel, task.logs, project.get, project.list, help.get.
- [x] Run `just lint-md` on changed markdown files; run `just docs-check` if applicable.
- [x] Validation gate: do not proceed until docs and code are consistent.

#### Refactor (Task 5)

- [x] None required unless description string is moved to a constant or generated from catalog; optional.
- [x] Validation gate: proceed.

#### Testing (Task 5)

- [x] Run `just docs-check` and `just lint` for changed files.
- [x] Run `just ci` to ensure full pipeline passes.
- [x] Validation gate: do not start Task 6 until all Task 5 checks pass.

#### Closeout (Task 5)

- [x] Generate a **task completion report** for Task 5: what was done (catalog update, tool description update), what passed, any deviations or notes.
- [x] Mark every completed step in this task's section with `- [x]`.
  Do not start Task 6 until this closeout is done.

---

### Task 6: Documentation and Closeout

Update dev_docs and any cross-cutting documentation; produce the final plan completion report.

#### Task 6 Requirements and Specifications

- [meta.md/../../../meta.md) (docs layout, dev_docs).
- This plan document.

#### Discovery (Task 6) Steps

- [x] Review all tasks: ensure no required step was skipped; ensure each closeout report is summarized.
- [x] Identify any user-facing or developer-facing docs that reference "PMA tools" or "minimum MCP set" and update them to reflect the new minimal set (e.g. development_setup.md if it lists MCP tools).

#### Red (Task 6)

- [x] N/A.

#### Green (Task 6)

- [x] Update any docs that list "minimum PMA chat tool set" or "MCP tools for PMA" to include help.get and the new task tools (task.list, result, cancel, logs).
  For example [docs/dev_docs/old/2026-03-14_plan_after_tui_fix.md](2026-03-14_plan_after_tui_fix.md) or [docs/development_setup.md](../../development_setup.md) if they mention the tool set.
- [x] Add a short "Implementation summary" to this plan (or a linked dev_doc) listing: which tools were added (help.get; task.list, result, cancel, logs; project.get, project.list), where the shared MCP client lives, and how to run tests.

#### Refactor (Task 6)

- [x] None.

#### Testing (Task 6)

- [x] Run `just docs-check` and `just ci` one final time.
- [x] Validation gate: plan complete when all tasks are closed out and ci passes.

#### Closeout (Task 6)

- [x] Generate the **final plan completion report**: which tasks were completed, overall validation status, and any remaining risks or follow-up (e.g. system_setting.get/list, db.task.create/update for future work).
- [x] Mark every completed step in this task's section with `- [x]`.

## Implementation Summary (2026-03-21)

See [PMA minimal tools plan completion (archived)](../2026-03-29_review_consolidated_summary.md) and [PMA minimal tools task 5 report (archived)](../2026-03-29_review_consolidated_summary.md).
