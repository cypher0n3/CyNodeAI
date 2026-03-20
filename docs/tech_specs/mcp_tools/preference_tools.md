# Preference MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Preference Tools](#preference-tools)
  - [`preference.get` Operation](#preferenceget-operation)
  - [`preference.list` Through `preference.delete` Operations](#preferencelist-through-preferencedelete-operations)
  - [`preference.effective` Operation](#preferenceeffective-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

These tools operate on user task-execution preferences (standards, constraints, acceptance criteria, writing style, language preferences, code language preferences, security constraints, definition-of-done, reporting style).
Preferences are scoped (system, user, group, project, task) and support effective resolution via scope precedence.
Tool names exposed to agents are resource-oriented (no implementation-layer prefixes such as `db.`).

Full CRUD for preferences; scope_type and scope_id define the scope; list responses MUST be size-limited and support pagination.

Related documents

- [User Preferences](../user_preferences.md): preference data model, scope semantics, effective resolution algorithm
- [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md): gateway enforcement mechanics
- [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md): role allowlists and scope

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Preference Tools

- Spec ID: `CYNAI.MCPTOO.PreferenceTools` <a id="spec-cynai-mcptoo-preferencetools"></a>

Common invocation pattern for preference tools: (1) Resolve caller identity and agent type; reject if unauthenticated. (2) Check tool allowlist and per-invocation scope; deny if not allowed. (3) Validate required and optional args per tool schema. (4) Call backend (orchestrator/Data API) with RBAC; backend enforces visibility and scope. (5) Emit audit record; return size-limited result or structured error (no secrets).

### `preference.get` Operation

- **Inputs**: Required `scope_type`, `key`; optional `scope_id` (required when scope_type is not `system`).
  Scope: `pm` (PMA, PAA); `sandbox` or `both` for read-only (SBA).
- **Outputs**: value and value_type for scoped key; not-found if absent.
- **Behavior**: Common pattern; backend returns preference by (scope_type, scope_id, key).
  See [preference.get Algorithm](#algo-cynai-mcptoo-preferenceget).

#### `preference.get` Algorithm

<a id="algo-cynai-mcptoo-preferenceget"></a>

1. Apply common pattern; validate scope_type, key, scope_id when required.
2. Call preference read; return value and value_type or not-found.
3. Audit and return.

### `preference.list` Through `preference.delete` Operations

- **Inputs/Outputs**: Per catalog (list: scope_type, optional scope_id, key_prefix, limit, cursor; create/update/delete: required scope and key, optional expected_version, reason).
  Scope: `pm` (PMA, PAA); `sandbox` or `both` for read-only list (SBA).
- **Behavior**: Each follows common pattern; backend enforces scope, uniqueness on create, and optimistic lock (expected_version) on update/delete.
  List MUST be size-limited and paginated.
  Create MUST fail if scoped key exists; update/delete MUST fail with conflict when expected_version does not match.
- **Algorithms**: Validate args, call backend with RBAC, handle conflict, audit and return.
  See [User Preferences](../user_preferences.md) for scope and merge semantics.

### `preference.effective` Operation

- **Inputs**: Required `task_id` (uuid); optional `include_sources` (boolean).
  Scope: `pm` (PMA, PAA); `sandbox` or `both` for read-only (SBA).
- **Outputs**: Effective preference set for task (merged by scope precedence); optionally with source breakdown.
- **Behavior**: Common pattern; resolve task and its scope chain (user, project, group, system); merge preferences by precedence; return merged result.
  See [preference.effective Algorithm](#algo-cynai-mcptoo-preferenceeffective).

#### `preference.effective` Algorithm

<a id="algo-cynai-mcptoo-preferenceeffective"></a>

1. Apply common pattern; validate task_id.
2. Resolve task and scope chain (user, project, group, system) for task.
3. Load preferences for each scope; merge by precedence (e.g. user over project over group over system).
4. If include_sources, attach source scope per key; enforce response size limit.
5. Audit and return.

#### Traces To

- [REQ-MCPTOO-0117](../../requirements/mcptoo.md#req-mcptoo-0117)

## Allowlist and Scope

- Spec ID: `CYNAI.MCPTOO.PreferenceToolsAllowlist` <a id="spec-cynai-mcptoo-preferencetoolsallowlist"></a>

- **Allowlist**: PMA and PAA for full CRUD (`preference.get`, `preference.list`, `preference.create`, `preference.update`, `preference.delete`, `preference.effective`) per [Project Manager Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-pmagentallowlist) and [Project Analyst Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-paagentallowlist).
  SBA read-only (`preference.get`, `preference.list`, `preference.effective`) per [Worker Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-workeragentallowlist).
- **Scope**: Default `pm` for write operations; `sandbox` or `both` for read operations (SBA needs access to effective preferences for the task/context); per-tool scope MUST align with [Per-tool scope: Sandbox vs PM](access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).
