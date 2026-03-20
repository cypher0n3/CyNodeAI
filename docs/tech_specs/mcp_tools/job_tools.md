# Job MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Job Tools](#job-tools)
  - [`job.get` Operation](#jobget-operation)
  - [`job.update_status` Operation](#jobupdate_status-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

These tools operate on job resources.
Jobs represent execution instances dispatched to worker nodes.
Typed operations only; raw SQL MUST NOT be exposed via MCP tools.
Tool names exposed to agents are resource-oriented (no implementation-layer prefixes such as `db.`).

MVP intentional exception: jobs are not full-CRUD via MCP tools.
The minimum MCP surface is read plus narrowly-scoped updates required by orchestrator-side agents.
Full job CRUD is exposed to user clients via the User API Gateway.

Related documents

- [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md)
- [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Job Tools

- Spec ID: `CYNAI.MCPTOO.JobTools` <a id="spec-cynai-mcptoo-jobtools"></a>

Common invocation pattern for job tools: (1) Resolve caller identity and agent type; reject if unauthenticated. (2) Check tool allowlist and per-invocation scope; deny if not allowed. (3) Validate required and optional args per tool schema. (4) Call backend (orchestrator/Data API) with RBAC; backend enforces visibility and transitions. (5) Emit audit record; return size-limited result or structured error (no secrets).

### `job.get` Operation

- **Inputs**: Required `job_id` (uuid).
  Scope: `pm`.
- **Outputs**: Job details; or not-found/access-denied.
- **Behavior**: Gateway applies common pattern; backend returns job if caller has access.
  See [job.get Algorithm](#algo-cynai-mcptoo-jobget).

#### `job.get` Algorithm

<a id="algo-cynai-mcptoo-jobget"></a>

1. Apply common pattern; validate job_id.
2. Call job read with caller context; RBAC determines visibility.
3. Return job details or not-found/access-denied; audit and return.

### `job.update_status` Operation

- **Inputs**: Required `job_id` (uuid), `status` (string).
  Scope: `pm`.
- **Outputs**: Success or error; allowed transitions enforced.
- **Behavior**: Gateway applies common pattern; backend validates transition.
  See [job.update_status Algorithm](#algo-cynai-mcptoo-jobupdatestatus).

#### `job.update_status` Algorithm

<a id="algo-cynai-mcptoo-jobupdatestatus"></a>

1. Apply common pattern; validate job_id and status (allowed transition).
2. Call job status update with caller context; enforce transition rules.
3. Return success or error; audit and return.

## Allowlist and Scope

- Spec ID: `CYNAI.MCPTOO.JobToolsAllowlist` <a id="spec-cynai-mcptoo-jobtoolsallowlist"></a>

- **Allowlist**: PMA and PAA for job tools per [Project Manager Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-pmagentallowlist) and [Project Analyst Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-paagentallowlist).
  SBA explicitly disallowed (jobs are orchestrator-side only).
- **Scope**: Default `pm` (orchestrator-side agents only); per-tool scope MUST align with [Per-tool scope: Sandbox vs PM](access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).
