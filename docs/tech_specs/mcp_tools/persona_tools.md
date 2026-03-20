# Persona MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contracts](#tool-contracts)
  - [`persona.list` Operation](#personalist-operation)
  - [`persona.get` Operation](#personaget-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

These tools operate on **Agent personas** (SBA role/identity descriptions), not customer or end-user personas.
PMA, PAA, and any service that builds SBA jobs MUST be able to query Agent personas (list, get) via MCP so they can resolve a chosen persona by id and embed it inline in the job spec.
When resolving by title or when multiple personas match, the builder MUST apply the most specific scope that matches (user over project over group over system).
CRUD for Agent personas is exposed to user clients via the User API Gateway (Data REST API), with RBAC restricting who may create/update/delete per scope; agents use these read tools for job building only.

Related documents

- [Project Manager Agent - Persona assignment and resolution](../project_manager_agent.md#spec-cynai-agents-personaassignment)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Tool Contracts

- Spec ID: `CYNAI.MCPTOO.PersonaTools` <a id="spec-cynai-mcptoo-personatools"></a>

### `persona.list` Operation

- **Inputs**: Optional `scope_type`, `scope_id`, `limit`, `cursor`.
  Scope: `pm`.
- **Outputs**: List of Agent personas (id, title, scope) visible to caller; paginated and size-limited.
- **Behavior**: Gateway checks allowlist and scope, queries persona store (Data API or equivalent) with scope filter, returns list.
  When multiple personas match, most-specific scope (user over project over group over system) applies per [Project Manager Agent - Persona assignment](../project_manager_agent.md#spec-cynai-agents-personaassignment).
  See [persona.list Algorithm](#algo-cynai-mcptoo-personalist).

#### `persona.list` Algorithm

<a id="algo-cynai-mcptoo-personalist"></a>

1. Resolve caller (orchestrator-side or worker proxy context); check allowlist and scope.
2. Apply optional scope_type, scope_id to filter; resolve caller's visible personas (RBAC/scopes).
3. Query persona store with filter; apply limit and cursor; enforce response size limit.
4. Emit audit record; return list.

### `persona.get` Operation

- **Inputs**: Required `persona_id` (uuid).
  Scope: `pm` or both (SBA via worker proxy).
- **Outputs**: Full Agent persona (id, title, description, scope_type, scope_id, default_skill_ids, recommended_cloud_models, recommended_local_model_ids when present, created_at, updated_at) for embedding in job spec.
- **Behavior**: Gateway checks allowlist and scope (PM/PA or worker proxy for SBA), looks up persona by id, returns full record if visible.
  See [persona.get Algorithm](#algo-cynai-mcptoo-personaget).

#### `persona.get` Algorithm

<a id="algo-cynai-mcptoo-personaget"></a>

1. Resolve caller (orchestrator or worker proxy); check allowlist and scope.
2. Validate `persona_id` (uuid); look up in persona store.
3. If not found or not visible to caller (scope/RBAC), return not-found or access-denied.
4. Return full persona (title, description, scope fields, default_skill_ids, recommended_cloud_models, recommended_local_model_ids when present, timestamps); enforce size limit.
5. Emit audit record; return result.

#### Traces To

- [REQ-MCPTOO-0120](../../requirements/mcptoo.md#req-mcptoo-0120)
- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)

## Allowlist and Scope

- **Allowlist**: PMA and PAA for list and get; Worker agent allowlist MUST include `persona.get` (and optionally `persona.list`) for the sandbox agent when persona resolution is needed in the sandbox context; when agents run on worker nodes, persona access MUST go through the worker proxy to the orchestrator MCP gateway.
- **Scope**: `pm` for orchestrator-side; `both` or sandbox for `persona.get` when exposed to SBA via worker proxy.
