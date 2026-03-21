# Task MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Task Tools](#task-tools)
  - [`task.get` Operation](#taskget-operation)
  - [`task.list` Operation](#tasklist-operation)
  - [`task.result` Operation](#taskresult-operation)
  - [`task.cancel` Operation](#taskcancel-operation)
  - [`task.logs` Operation](#tasklogs-operation)
  - [`task.update_status` Operation](#taskupdate_status-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

These tools operate on task resources.
Tasks represent units of work that agents manage and execute.
Typed operations only; raw SQL MUST NOT be exposed via MCP tools.
Tool names exposed to agents are resource-oriented (no implementation-layer prefixes such as `db.`).

MVP intentional exception: tasks are not full-CRUD via MCP tools.
The minimum MCP surface is read plus narrowly-scoped updates required by orchestrator-side agents.
Full task CRUD is exposed to user clients via the User API Gateway.

Related documents

- [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md)
- [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Task Tools

- Spec ID: `CYNAI.MCPTOO.TaskTools` <a id="spec-cynai-mcptoo-tasktools"></a>

Common invocation pattern for task tools: (1) Resolve caller identity and agent type; reject if unauthenticated. (2) Check tool allowlist and per-invocation scope; deny if not allowed. (3) Validate required and optional args per tool schema. (4) Call backend (orchestrator/Data API) with RBAC; backend enforces visibility and transitions. (5) Emit audit record; return size-limited result or structured error (no secrets).

### `task.get` Operation

- **Inputs**: Required `task_id` (uuid).
  Scope: `pm`.
- **Outputs**: Task details (id, status, project_id, etc.); or not-found/access-denied.
- **Behavior**: Gateway applies common pattern; backend returns task if caller has access.
  See [task.get Algorithm](#algo-cynai-mcptoo-taskget).

#### `task.get` Algorithm

<a id="algo-cynai-mcptoo-taskget"></a>

1. Apply common task tool invocation pattern (resolve caller, allowlist, scope, validate task_id).
2. Call task read (by task_id) with caller context; RBAC determines visibility.
3. Return task details or not-found/access-denied; audit and return.

### `task.list` Operation

- **Inputs**: Required `user_id` (uuid) to scope the list to that user's tasks (same scoping as the User API Gateway list-by-user).
  Optional: `limit` (default 50, cap 200), `offset` (non-negative), `status` (filter), `cursor` (opaque; when supported, combined with limit for pagination per gateway implementation).
  Scope: `pm`.
- **Outputs**: List response aligned with the gateway (`tasks`, optional `next_cursor`); size-limited.
- **Behavior**: Gateway applies the common pattern; backend lists tasks for the given `user_id` with RBAC as for HTTP list.
  See [task.list Algorithm](#algo-cynai-mcptoo-tasklist).

#### `task.list` Algorithm

<a id="algo-cynai-mcptoo-tasklist"></a>

1. Apply common pattern; validate `user_id` and optional list parameters.
2. Call the same list path as the User API Gateway (`ListTasksByUser` / equivalent); enforce limit cap and filters.
3. Return list or error; audit and return.

### `task.result` Operation

- **Inputs**: Required `task_id` (uuid).
  Scope: `pm`.
- **Outputs**: Task status and aggregated job result (stdout/stderr and related fields when terminal), consistent with gateway task-result semantics.
- **Behavior**: Same data path as the User API Gateway task result endpoint.
  See [task.result Algorithm](#algo-cynai-mcptoo-taskresult).

#### `task.result` Algorithm

<a id="algo-cynai-mcptoo-taskresult"></a>

1. Apply common pattern; validate `task_id`.
2. Load task and jobs; build result payload; not-found if task missing or inaccessible.
3. Return payload or error; audit and return.

### `task.cancel` Operation

- **Inputs**: Required `task_id` (uuid).
  Scope: `pm`.
- **Outputs**: Success with canceled flag or structured error.
- **Behavior**: MUST invoke the **same cancel-and-stop-job path** as the User API Gateway (`CancelTask`): update task to canceled, update non-terminal jobs, and stop active jobs on the worker when implemented-no MCP-only shortcut.
  See [task.cancel Algorithm](#algo-cynai-mcptoo-taskcancel).

#### `task.cancel` Algorithm

<a id="algo-cynai-mcptoo-taskcancel"></a>

1. Apply common pattern; validate `task_id`.
2. Call shared cancel logic with the gateway (task exists -> cancel task and jobs per orchestrator rules).
3. Return success or error; audit and return.

### `task.logs` Operation

- **Inputs**: Required `task_id` (uuid); optional `stream` (`stdout` / `stderr` / `all` or implementation-defined default).
  Scope: `pm`.
- **Outputs**: Aggregated log lines from jobs for the task; size-limited.
- **Behavior**: Same aggregation rules as the User API Gateway task logs.
  See [task.logs Algorithm](#algo-cynai-mcptoo-tasklogs).

#### `task.logs` Algorithm

<a id="algo-cynai-mcptoo-tasklogs"></a>

1. Apply common pattern; validate `task_id` and optional `stream`.
2. Resolve task and job logs; not-found if task missing.
3. Return lines or error; audit and return.

### `task.update_status` Operation

- **Inputs**: Required `task_id` (uuid), `status` (string).
  Scope: `pm`.
- **Outputs**: Success or error; gateway enforces allowed status transitions.
- **Behavior**: Gateway applies common pattern; backend validates transition and applies update.
  See [task.update_status Algorithm](#algo-cynai-mcptoo-taskupdatestatus).

#### `task.update_status` Algorithm

<a id="algo-cynai-mcptoo-taskupdatestatus"></a>

1. Apply common pattern; validate task_id and status (allowed transition from current state).
2. Call task status update with caller context; RBAC and transition rules enforced.
3. Return success or conflict/invalid transition error; audit and return.

## Allowlist and Scope

- Spec ID: `CYNAI.MCPTOO.TaskToolsAllowlist` <a id="spec-cynai-mcptoo-tasktoolsallowlist"></a>

- **Allowlist**: PMA and PAA for task tools per [Project Manager Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-pmagentallowlist) and [Project Analyst Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-paagentallowlist).
  SBA explicitly disallowed (tasks are orchestrator-side only).
- **Scope**: Default `pm` (orchestrator-side agents only); per-tool scope MUST align with [Per-tool scope: Sandbox vs PM](access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).
