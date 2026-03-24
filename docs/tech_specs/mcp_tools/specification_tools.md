# Specification MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Specification Tools](#specification-tools)
  - [`specification.create` Through `specification.delete` Operations](#specificationcreate-through-specificationdelete-operations)
  - [`plan.specifications.set` Operation](#planspecificationsset-operation)
  - [`task.specifications.set` Operation](#taskspecificationsset-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

These tools operate on specification resources and their relationships to plans and tasks.
When the host has a `specifications` table, these tools apply.
Typed operations only; raw SQL MUST NOT be exposed via MCP tools.
Tool names exposed to agents are resource-oriented (no implementation-layer prefixes such as `db.`).

Allowlist and scope rules match task write tools (PMA may write; SBA read-only; PAA per catalog).

Related documents

- [Postgres Schema - Specifications Table](../projects_and_scopes.md#spec-cynai-schema-specificationstable)
- [Postgres Schema - SpecificationObject contract](../projects_and_scopes.md#spec-cynai-schema-specificationobjectcontract)
- [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md)
- [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Specification Tools

- Spec ID: `CYNAI.MCPTOO.SpecificationTools` <a id="spec-cynai-mcptoo-specificationtools"></a>

Common invocation pattern for specification tools: (1) Resolve caller identity and agent type; reject if unauthenticated. (2) Check tool allowlist and per-invocation scope; deny if not allowed. (3) Validate required and optional args per tool schema. (4) Call backend (orchestrator/Data API) with RBAC; backend enforces visibility and project matching. (5) Emit audit record; return size-limited result or structured error (no secrets).

### `specification.create` Through `specification.delete` Operations

- **Inputs/Outputs**: Per catalog (create: project_id, at least one of spec_id/ref/description, optional fields; list: project_id, limit, cursor; get: specification_id; update: specification_id + optional fields; delete: specification_id).
  Scope: PMA write; PAA/SBA per catalog.
- **Behavior**: Common pattern; backend enforces project_id and specification object contract per [SpecificationObject contract](../projects_and_scopes.md#spec-cynai-schema-specificationobjectcontract).
  Create/list/get/update/delete follow standard CRUD; list and get enforce project visibility.
- **Algorithms**: Resolve caller and allowlist; validate args (e.g. project_id present, specification_id for get/update/delete); call backend; ensure task's project matches specification's project_id when linking; audit and return.

### `plan.specifications.set` Operation

- **Inputs**: Required `plan_id` (uuid), `specification_ids` (array of uuid).
  Scope: `pm`.
- **Outputs**: Success or error.
- **Behavior**: Common pattern; replace plan's specification references via join table; validate plan and each specification exist and caller has access.
  See [plan.specifications.set Algorithm](#algo-cynai-mcptoo-planspecificationsset).

#### `plan.specifications.set` Algorithm

<a id="algo-cynai-mcptoo-planspecificationsset"></a>

1. Apply common pattern; validate plan_id and specification_ids (array of uuid).
2. Resolve plan and each specification; verify caller has access and specifications belong to same project as plan (or per schema rules).
3. Replace join table rows linking plan to specifications; commit.
4. Audit and return.

### `task.specifications.set` Operation

- **Inputs**: Required `task_id` (uuid), `specification_ids` (array of uuid).
  Scope: `pm`.
- **Outputs**: Success or error; application MUST ensure task's project matches each specification's project_id.
- **Behavior**: Common pattern; replace task's specification references; validate task and each specification; enforce task project matches specification project_id.
  See [task.specifications.set Algorithm](#algo-cynai-mcptoo-taskspecificationsset).

#### `task.specifications.set` Algorithm

<a id="algo-cynai-mcptoo-taskspecificationsset"></a>

1. Apply common pattern; validate required `task_id` and `specification_ids` (array of uuid).
2. Resolve task and each specification; MUST ensure task's project_id matches each specification's project_id; reject if any mismatch.
3. Replace join table rows linking task to specifications; commit.
4. Audit and return.

#### Traces To

- [REQ-MCPTOO-0118](../../requirements/mcptoo.md#req-mcptoo-0118)
- [REQ-MCPTOO-0106](../../requirements/mcptoo.md#req-mcptoo-0106)

## Allowlist and Scope

- Spec ID: `CYNAI.MCPTOO.SpecificationToolsAllowlist` <a id="spec-cynai-mcptoo-specificationtoolsallowlist"></a>

- **Allowlist**: PMA and PAA for specification tools per [Project Manager Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-pmagentallowlist) and [Project Analyst Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-paagentallowlist).
  SBA read-only for specifications when exposed; PAA per catalog for write tools.
- **Scope**: Default `pm` for write and sensitive read; per-tool scope MUST align with [Per-tool scope: Sandbox vs PM](access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).
