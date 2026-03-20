# Project MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Project Tools](#project-tools)
  - [`project.get` Operation](#projectget-operation)
  - [`project.list` Operation](#projectlist-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

These tools operate on project resources.
Projects are workspace boundaries used for authorization scope and preference resolution.
Typed operations only; raw SQL MUST NOT be exposed via MCP tools.
Tool names exposed to agents are resource-oriented (no implementation-layer prefixes such as `db.`).

All project tools MUST return only projects the authenticated user is authorized to access (default project plus RBAC-scoped projects).

Related documents

- [Projects and Scopes - Project Search via MCP](../projects_and_scopes.md#spec-cynai-access-projectsmcpsearch)
- [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md)
- [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Project Tools

- Spec ID: `CYNAI.MCPTOO.ProjectTools` <a id="spec-cynai-mcptoo-projecttools"></a>

Common invocation pattern for project tools: (1) Resolve caller identity and agent type; reject if unauthenticated. (2) Check tool allowlist and per-invocation scope; deny if not allowed. (3) Validate required and optional args per tool schema. (4) Call backend (orchestrator/Data API) with RBAC; backend enforces visibility and authorized set. (5) Emit audit record; return size-limited result or structured error (no secrets).

### `project.get` Operation

- **Inputs**: Required `project_id` (uuid) or `slug` (text); exactly one MUST be provided.
  Scope: `pm`.
- **Outputs**: Project details if in caller's authorized set; otherwise not-found or access-denied.
- **Behavior**: Common pattern; backend returns project only if caller has access (default project plus RBAC).
  See [project.get Algorithm](#algo-cynai-mcptoo-projectget).

#### `project.get` Algorithm

<a id="algo-cynai-mcptoo-projectget"></a>

1. Apply common pattern; validate that exactly one of project_id or slug is provided.
2. Call project read (by id or slug) with caller context; backend filters by authorized set.
3. Return project or not-found/access-denied; audit and return.

### `project.list` Operation

- **Inputs**: Optional `q` (filter slug, display_name, description), `limit`, `cursor`.
  Scope: `pm`.
- **Outputs**: List of authorized projects; size-limited and paginated.
- **Behavior**: Common pattern; backend lists only projects in caller's authorized set, with optional q filter.
  See [project.list Algorithm](#algo-cynai-mcptoo-projectlist).

#### `project.list` Algorithm

<a id="algo-cynai-mcptoo-projectlist"></a>

1. Apply common pattern; validate optional q, limit, cursor.
2. Call project list with caller context (authorized set only) and optional q; apply limit and cursor.
3. Enforce response size limit; return list and next cursor; audit and return.

## Allowlist and Scope

- Spec ID: `CYNAI.MCPTOO.ProjectToolsAllowlist` <a id="spec-cynai-mcptoo-projecttoolsallowlist"></a>

- **Allowlist**: PMA and PAA for project tools per [Project Manager Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-pmagentallowlist) and [Project Analyst Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-paagentallowlist).
  SBA explicitly disallowed (projects are orchestrator-side only).
- **Scope**: Default `pm` (orchestrator-side agents only); per-tool scope MUST align with [Per-tool scope: Sandbox vs PM](access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).
