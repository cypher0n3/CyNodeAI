# Node MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contracts](#tool-contracts)
  - [`node.list` Operation](#nodelist-operation)
  - [`node.get` Operation](#nodeget-operation)
  - [`node.refresh_config` Operation](#noderefresh_config-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

Node tools allow agents to list worker nodes, get node details, and request configuration refresh.
They operate on orchestrator-registered worker nodes.

Related documents

- [Worker Node](../worker_node.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Tool Contracts

- Spec ID: `CYNAI.MCPTOO.NodeTools` <a id="spec-cynai-mcptoo-nodetools"></a>

### `node.list` Operation

- **Inputs**: Optional `limit`, `cursor` (if supported).
  Scope: `pm`.
- **Outputs**: List of registered worker nodes (slug, status, optional metadata); size-limited and paginated if needed.
- **Behavior**: Gateway checks allowlist and scope, queries orchestrator node registry for nodes visible to caller, returns list.
  See [node.list Algorithm](#algo-cynai-mcptoo-nodelist).

#### `node.list` Algorithm

<a id="algo-cynai-mcptoo-nodelist"></a>

1. Resolve caller; verify PM/PA agent; deny if not on allowlist.
2. Query orchestrator for registered worker nodes (filter by visibility/RBAC if applicable).
3. Apply optional limit and cursor; enforce response size limit.
4. Emit audit record; return list.

### `node.get` Operation

- **Inputs**: Required `node_slug` (string).
  Scope: `pm`.
- **Outputs**: Node details (slug, status, config ref, optional capabilities); or not-found/access-denied.
- **Behavior**: Gateway checks allowlist and scope, looks up node by slug in orchestrator registry, returns details if visible.
  See [node.get Algorithm](#algo-cynai-mcptoo-nodeget).

#### `node.get` Algorithm

<a id="algo-cynai-mcptoo-nodeget"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `node_slug` (non-empty).
3. Look up node by slug in orchestrator node registry; if not found or not visible to caller, return not-found or access-denied.
4. Return node details (size-limited); emit audit record.

### `node.refresh_config` Operation

- **Inputs**: Required `node_slug` (string).
  Scope: `pm`.
- **Outputs**: Success or error; outcome may be async (refresh requested).
- **Behavior**: Gateway checks allowlist and scope, sends refresh-config request to orchestrator for the node; orchestrator notifies node or updates config per implementation.
  See [node.refresh_config Algorithm](#algo-cynai-mcptoo-noderefreshconfig).

#### `node.refresh_config` Algorithm

<a id="algo-cynai-mcptoo-noderefreshconfig"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `node_slug`; resolve node in registry; reject if not found or not visible.
3. Request configuration refresh for the node (e.g. send message to node manager or enqueue refresh); implementation may be async.
4. Emit audit record; return success or error (e.g. node unavailable).

#### Traces To

- [REQ-MCPTOO-0106](../../requirements/mcptoo.md#req-mcptoo-0106)
- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)

## Allowlist and Scope

- **Allowlist**: PMA and PAA per [Project Manager Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-pmagentallowlist) and [Project Analyst Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-paagentallowlist).
- **Scope**: `pm` (orchestrator-side agents only).
