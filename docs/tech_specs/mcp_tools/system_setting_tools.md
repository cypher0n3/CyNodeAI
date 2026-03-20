# System Setting MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [System Setting Tools](#system-setting-tools)
  - [`system_setting.get` Operation](#system_settingget-operation)
  - [`system_setting.list` Operation](#system_settinglist-operation)
  - [`system_setting.create` Operation](#system_settingcreate-operation)
  - [`system_setting.update` Operation](#system_settingupdate-operation)
  - [`system_setting.delete` Operation](#system_settingdelete-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

These tools operate on system settings (operator- and deployment-level configuration: orchestrator operational knobs, model selection keys, cache limits, deployment config such as ports, hostnames, database DSNs, service endpoints).
System settings are distinct from user preferences; see [Settings vs Preferences](../user_preferences.md#spec-cynai-stands-preferenceterminology) for the distinction.
Tool names exposed to agents are resource-oriented (no implementation-layer prefixes such as `db.`).

Full CRUD for system settings; list responses MUST be size-limited and support pagination.

Related documents

- [User Preferences](../user_preferences.md): distinction between preferences and system settings
- [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md): gateway enforcement mechanics
- [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md): role allowlists and scope

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## System Setting Tools

- Spec ID: `CYNAI.MCPTOO.SystemSettingTools` <a id="spec-cynai-mcptoo-systemsettingtools"></a>

Common invocation pattern for system setting tools: (1) Resolve caller identity and agent type; reject if unauthenticated. (2) Check tool allowlist and per-invocation scope; deny if not allowed. (3) Validate required and optional args per tool schema. (4) Call backend (orchestrator/Data API) with RBAC; backend enforces visibility and access. (5) Emit audit record; return size-limited result or structured error (no secrets).

### `system_setting.get` Operation

- **Inputs**: Required `key` (string).
  Scope: `pm`.
- **Outputs**: value and value_type for key; not-found if absent.
- **Behavior**: Common pattern; backend returns system setting by key.
  See [system_setting.get Algorithm](#algo-cynai-mcptoo-systemsettingget).

#### `system_setting.get` Algorithm

<a id="algo-cynai-mcptoo-systemsettingget"></a>

1. Apply common pattern; validate key (non-empty).
2. Call system_setting read by key; return value and value_type or not-found.
3. Audit and return.

### `system_setting.list` Operation

- **Inputs**: Optional `key_prefix`, `limit`, `cursor`.
  Scope: `pm`.
- **Outputs**: List of (key, value_type) or full rows; size-limited and paginated.
- **Behavior**: Common pattern; backend lists with optional key_prefix; apply limit/cursor.
  See [system_setting.list Algorithm](#algo-cynai-mcptoo-systemsettinglist).

#### `system_setting.list` Algorithm

<a id="algo-cynai-mcptoo-systemsettinglist"></a>

1. Apply common pattern; validate optional key_prefix, limit, cursor.
2. Call system_setting list with filter and pagination; enforce response size limit.
3. Return list and next cursor if applicable; audit and return.

### `system_setting.create` Operation

- **Inputs**: Required `key`, `value`, `value_type`; optional `reason`.
  Scope: `pm`.
- **Outputs**: Success or error; create MUST fail if key already exists.
- **Behavior**: Common pattern; backend inserts; conflict if key exists.
  See [system_setting.create Algorithm](#algo-cynai-mcptoo-systemsettingcreate).

#### `system_setting.create` Algorithm

<a id="algo-cynai-mcptoo-systemsettingcreate"></a>

1. Apply common pattern; validate key, value, value_type.
2. Call system_setting create; if key exists return conflict error.
3. Audit and return success or error.

### `system_setting.update` Operation

- **Inputs**: Required `key`, `value`, `value_type`; optional `expected_version`, `reason`.
  Scope: `pm`.
- **Outputs**: Success or conflict when expected_version does not match.
- **Behavior**: Common pattern; backend updates; if expected_version provided and does not match current, return conflict.
  See [system_setting.update Algorithm](#algo-cynai-mcptoo-systemsettingupdate).

#### `system_setting.update` Algorithm

<a id="algo-cynai-mcptoo-systemsettingupdate"></a>

1. Apply common pattern; validate key, value, value_type, optional expected_version.
2. Call system_setting update; if expected_version present and does not match, return conflict.
3. Audit and return.

### `system_setting.delete` Operation

- **Inputs**: Required `key`; optional `expected_version`, `reason`.
  Scope: `pm`.
- **Outputs**: Success or conflict if expected_version does not match.
- **Behavior**: Common pattern; backend deletes; optional optimistic lock.
  See [system_setting.delete Algorithm](#algo-cynai-mcptoo-systemsettingdelete).

#### `system_setting.delete` Algorithm

<a id="algo-cynai-mcptoo-systemsettingdelete"></a>

1. Apply common pattern; validate key, optional expected_version.
2. Call system_setting delete; if expected_version present and does not match, return conflict.
3. Audit and return.

## Allowlist and Scope

- Spec ID: `CYNAI.MCPTOO.SystemSettingToolsAllowlist` <a id="spec-cynai-mcptoo-systemsettingtoolsallowlist"></a>

- **Allowlist**: PMA and PAA for full CRUD (`system_setting.get`, `system_setting.list`, `system_setting.create`, `system_setting.update`, `system_setting.delete`) per [Project Manager Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-pmagentallowlist) and [Project Analyst Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-paagentallowlist).
  SBA explicitly disallowed (system settings are orchestrator-side only).
- **Scope**: Default `pm` (orchestrator-side agents only); per-tool scope MUST align with [Per-tool scope: Sandbox vs PM](access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).
