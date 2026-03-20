# Model Registry MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contracts](#tool-contracts)
  - [`model.list` Operation](#modellist-operation)
  - [`model.get` Operation](#modelget-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

Model registry tools allow agents to list available models and get model details by id.
They expose the orchestrator's model registry (or equivalent) without direct database access.

Related documents

- [Model Management](../model_management.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Tool Contracts

- Spec ID: `CYNAI.MCPTOO.ModelRegistryTools` <a id="spec-cynai-mcptoo-modelregistrytools"></a>

### `model.list` Operation

- **Inputs**: Optional `limit`, `cursor` (if supported).
  Scope: `pm` or `both`.
- **Outputs**: List of available models (id, name, optional capabilities); size-limited and paginated if needed.
- **Behavior**: Gateway checks allowlist and scope, queries model registry (orchestrator), returns models visible to caller.
  See [model.list Algorithm](#algo-cynai-mcptoo-modellist).

#### `model.list` Algorithm

<a id="algo-cynai-mcptoo-modellist"></a>

1. Resolve caller; check allowlist and scope.
2. Query model registry for available models (filter by visibility if applicable); apply limit and cursor.
3. Enforce response size limit; return list.
4. Emit audit record; return result.

### `model.get` Operation

- **Inputs**: Required `model_id` (string or uuid).
  Scope: `pm` or `both`.
- **Outputs**: Model details (id, name, capabilities, etc.); or not-found/access-denied.
- **Behavior**: Gateway checks allowlist and scope, looks up model by id in registry, returns details if visible.
  See [model.get Algorithm](#algo-cynai-mcptoo-modelget).

#### `model.get` Algorithm

<a id="algo-cynai-mcptoo-modelget"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `model_id`; look up in model registry.
3. If not found or not visible to caller, return not-found or access-denied.
4. Return model details (size-limited); emit audit record.

#### Traces To

- [REQ-MCPTOO-0106](../../requirements/mcptoo.md#req-mcptoo-0106)
- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)

## Allowlist and Scope

- **Allowlist**: PMA and PAA per [MCP Gateway Enforcement](../mcp_gateway_enforcement.md); Worker agent may be allowed for read-only model list/get when building inference requests.
- **Scope**: Default `pm` or `both` per catalog.
