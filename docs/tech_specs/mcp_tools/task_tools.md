# Task MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Task Tools](#task-tools)
  - [`task.get` Operation](#taskget-operation)
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
