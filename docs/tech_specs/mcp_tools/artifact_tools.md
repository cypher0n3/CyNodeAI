# Artifact MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Task-Scoped (Path-Based) Tools](#task-scoped-path-based-tools)
  - [`artifact.put` Operation](#artifactput-operation)
  - [`artifact.get` Operation](#artifactget-operation)
  - [`artifact.list` Operation](#artifactlist-operation)
- [Unified API (Artifact\_id-Based) Tools](#unified-api-artifact_id-based-tools)
  - [`artifacts.create` Operation](#artifactscreate-operation)
  - [`artifacts.get` Operation](#artifactsget-operation)
  - [`artifacts.update` Operation](#artifactsupdate-operation)
  - [`artifacts.delete` Operation](#artifactsdelete-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

This spec defines the artifact MCP tools used by the Project Manager Agent (PMA) and Project Analyst Agent (PAA) to read and write task-scoped artifacts and to perform full CRUD on artifacts via the unified artifacts API.
Implementation MUST use the [unified artifacts API](../orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsapicrud) with scope and RBAC enforced by the gateway.

Related documents

- [Orchestrator Artifacts Storage](../orchestrator_artifacts_storage.md)

## Definition Compliance

Tool definitions for these tools MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, and `Tools` (single direct `ToolInvocation` per catalog tool name).
Endpoint resolution applies only when defining external tools (Server not `default`).

## Task-Scoped (Path-Based) Tools

- Spec ID: `CYNAI.MCPTOO.ArtifactTools` <a id="spec-cynai-mcptoo-artifacttools"></a>

PMA and PAA MUST use these MCP tools to access task-scoped artifacts by path.
Each tool is a single direct invocation; required arguments conform to [Common Argument Requirements](../mcp/mcp_tooling.md#spec-cynai-mcptoo-commonargumentrequirements) (task scoping, size limits).

### `artifact.put` Operation

- Spec ID: `CYNAI.MCPTOO.ArtifactPut` <a id="spec-cynai-mcptoo-artifactput"></a>

#### `artifact.put` Inputs

- **Required args**: `task_id` (uuid), `path` (string), `content_bytes_base64` (string).
- **Scope**: Default `pm`.

#### `artifact.put` Outputs

- On success: structured result with `status: success` and confirmation (e.g. path or artifact_id); response MUST be size-limited per [Response and Error Model](../mcp/mcp_tooling.md#spec-cynai-mcptoo-toolresponse).
- On error: `status: error`, `error` object with `type`, `message`, optional `details`; MUST NOT leak secrets.

#### `artifact.put` Behavior

Gateway resolves caller, enforces allowlist and scope, validates and size-limits input, then performs task-scoped artifact write via the artifacts backend.
See [artifact.put Algorithm](#algo-cynai-mcptoo-artifactput).

#### `artifact.put` Algorithm

<a id="algo-cynai-mcptoo-artifactput"></a>

1. Resolve caller identity and agent type from request context; reject if unauthenticated. <a id="algo-cynai-mcptoo-artifactput-step-1"></a>
2. Check tool allowlist and per-invocation scope for this agent; deny if not allowed. <a id="algo-cynai-mcptoo-artifactput-step-2"></a>
3. Validate `task_id` format (uuid) and that caller has access to the task (RBAC/project). <a id="algo-cynai-mcptoo-artifactput-step-3"></a>
4. Validate `path` (non-empty, safe; reject path traversal or invalid characters). <a id="algo-cynai-mcptoo-artifactput-step-4"></a>
5. Decode `content_bytes_base64`; enforce configured max blob size; reject if over limit. <a id="algo-cynai-mcptoo-artifactput-step-5"></a>
6. Call artifacts API (or task-scoped backend) to write blob at task + path; apply RBAC at API layer. <a id="algo-cynai-mcptoo-artifactput-step-6"></a>
7. Emit audit record (tool name, task_id, decision, outcome). <a id="algo-cynai-mcptoo-artifactput-step-7"></a>
8. Return success result (path or artifact reference); ensure response is size-limited. <a id="algo-cynai-mcptoo-artifactput-step-8"></a>

#### `artifact.put` Error Conditions

- **Invalid argument**: missing or malformed `task_id`, `path`, or `content_bytes_base64`; return 400-style error.
- **Size limit exceeded**: content exceeds configured max; return error with type and message, no secrets.
- **Access denied**: caller not on allowlist, scope mismatch, or RBAC denies task/path; return 403-style error.
- **Not found**: task_id does not exist or caller has no access; return 404-style error.
- **Backend failure**: artifacts API or storage error; return 502/503-style error with safe message.

#### Traces to (`artifact.put`)

- [REQ-MCPTOO-0106](../../requirements/mcptoo.md#req-mcptoo-0106)
- [REQ-MCPTOO-0107](../../requirements/mcptoo.md#req-mcptoo-0107)

### `artifact.get` Operation

- Spec ID: `CYNAI.MCPTOO.ArtifactGet` <a id="spec-cynai-mcptoo-artifactget"></a>

#### `artifact.get` Inputs

- **Required args**: `task_id` (uuid), `path` (string).
- **Scope**: Default `pm`.

#### `artifact.get` Outputs

- On success: result object containing artifact content (e.g. `content_bytes_base64` or inline) and optional metadata; response MUST be size-limited and schema-validated.
- On error: `status: error`, `error` object; MUST NOT leak secrets.

#### `artifact.get` Behavior

Gateway enforces allowlist and scope, validates task_id and path, then reads from the task-scoped artifact store and returns size-limited content.
See [artifact.get Algorithm](#algo-cynai-mcptoo-artifactget).

#### `artifact.get` Algorithm

<a id="algo-cynai-mcptoo-artifactget"></a>

1. Resolve caller identity and agent type; reject if unauthenticated.
2. Check tool allowlist and scope; deny if not allowed.
3. Validate `task_id` and `path`; verify caller has access to the task.
4. Call artifacts backend to read blob at task + path; enforce RBAC.
5. If blob exceeds response size limit, truncate or return error per policy.
6. Emit audit record.
7. Return result with content and optional metadata (size-limited, schema-validated).

#### `artifact.get` Error Conditions

- **Invalid argument**: missing or malformed `task_id` or `path`.
- **Access denied**: allowlist/scope or RBAC denies access.
- **Not found**: no artifact at path for task, or task not found/accessible.
- **Backend failure**: storage or API error; return safe error.

#### Traces to (`artifact.get`)

- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)

### `artifact.list` Operation

- Spec ID: `CYNAI.MCPTOO.ArtifactList` <a id="spec-cynai-mcptoo-artifactlist"></a>

#### `artifact.list` Inputs

- **Required args**: `task_id` (uuid).
- **Optional args**: `limit`, `cursor` (when pagination is supported).
- **Scope**: Default `pm`.

#### `artifact.list` Outputs

- On success: list of artifact paths (and optionally metadata); MUST be size-limited and paginated if needed; include `cursor` for next page when applicable.
- On error: `status: error`, `error` object.

#### `artifact.list` Behavior

Gateway enforces allowlist and scope, validates task_id, then lists task-scoped artifact paths from the backend with size and pagination limits.
See [artifact.list Algorithm](#algo-cynai-mcptoo-artifactlist).

#### `artifact.list` Algorithm

<a id="algo-cynai-mcptoo-artifactlist"></a>

1. Resolve caller identity and agent type; reject if unauthenticated.
2. Check tool allowlist and scope; deny if not allowed.
3. Validate `task_id`; verify caller has access to the task.
4. Call artifacts backend to list paths (and optional metadata) for task; apply optional limit and cursor.
5. Enforce response size limit; truncate or paginate as configured.
6. Emit audit record.
7. Return list result (paths, optional cursor).

#### `artifact.list` Error Conditions

- **Invalid argument**: missing or malformed `task_id`.
- **Access denied**: allowlist/scope or RBAC denies access.
- **Not found**: task not found or not accessible.
- **Backend failure**: storage or API error.

#### Traces to (`artifact.list`)

- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)

## Unified API (Artifact_id-Based) Tools

- Spec ID: `CYNAI.MCPTOO.ArtifactsUnifiedTools` <a id="spec-cynai-mcptoo-artifactsunifiedtools"></a>

These tools provide full CRUD by `artifact_id` so PMA and PAA can create, read, update, and delete artifacts via the unified artifacts API.
Gateway stores blobs and returns `artifact_id`; scope is expressed in the request (e.g. `task_id`, `thread_id`, `project_id`).

### `artifacts.create` Operation

- **Inputs**: Required scope (e.g. `task_id`, `thread_id`, or `project_id`), content (e.g. base64 or URL); optional `filename`, `content_type`.
  Scope: `pm`.
- **Outputs**: On success, result with `artifact_id`; on error, structured error.
- **Behavior**: Gateway validates scope and content, enforces size limits, calls unified artifacts API (POST), stores blob and metadata, returns artifact_id.
  See [Orchestrator Artifacts Storage - Create](../orchestrator_artifacts_storage.md).
- **Algorithm**: (1) Resolve caller and check allowlist/scope. (2) Validate scope keys and content; enforce max size. (3) Call POST /v1/artifacts (or equivalent) with scope and blob; RBAC enforced by API. (4) Audit and return artifact_id or error.
- **Error conditions**: Invalid scope or content; size exceeded; access denied; backend failure.

### `artifacts.get` Operation

- **Inputs**: Required `artifact_id` (uuid).
  Scope: `pm`.
- **Outputs**: Artifact blob and metadata; size-limited.
- **Behavior**: Gateway resolves caller, checks allowlist and RBAC for artifact, calls GET by artifact_id, returns size-limited content.
- **Algorithm**: (1) Resolve caller; check allowlist/scope. (2) Validate artifact_id. (3) Call read on artifacts API; RBAC determines visibility. (4) Apply response size limit. (5) Audit and return result or not-found/denied.
- **Error conditions**: Invalid artifact_id; access denied; not found; backend failure.

### `artifacts.update` Operation

- **Inputs**: Required `artifact_id` (uuid), content (replacement blob).
  Scope: `pm`.
- **Outputs**: Success confirmation or error.
- **Behavior**: Gateway validates artifact_id and content size, enforces RBAC, calls PUT on artifacts API to replace blob.
- **Algorithm**: (1) Resolve caller; check allowlist/scope. (2) Validate artifact_id and content; enforce size limit. (3) Call update on artifacts API; RBAC enforced. (4) Audit and return.
- **Error conditions**: Invalid args; size exceeded; access denied; not found; backend failure.

### `artifacts.delete` Operation

- **Inputs**: Required `artifact_id` (uuid).
  Scope: `pm`.
- **Outputs**: Success confirmation or error.
- **Behavior**: Gateway validates artifact_id, enforces RBAC, calls DELETE on artifacts API.
- **Algorithm**: (1) Resolve caller; check allowlist/scope. (2) Validate artifact_id. (3) Call delete on artifacts API; RBAC enforced. (4) Audit and return.
- **Error conditions**: Invalid artifact_id; access denied; not found; backend failure.

#### Traces to (Artifacts CRUD)

- [REQ-MCPTOO-0118](../../requirements/mcptoo.md#req-mcptoo-0118)

## Allowlist and Scope

- Spec ID: `CYNAI.MCPTOO.ArtifactToolsAllowlist` <a id="spec-cynai-mcptoo-artifacttoolsallowlist"></a>

- **Allowlist**: PMA and PAA per [Project Manager Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-pmagentallowlist) and [Project Analyst Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-paagentallowlist).
  Gateway MUST allow `artifact.*` and `artifacts.*` for these agents.
- **Scope**: All artifact tools are PM/PA-only by default; per-tool scope MUST align with [Per-tool scope: Sandbox vs PM](access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).
