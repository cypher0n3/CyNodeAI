# Artifact MCP Tools

- [Document Overview](#document-overview)
- [Artifact Scope, Ownership, and RBAC](#artifact-scope-ownership-and-rbac)
- [Definition Compliance](#definition-compliance)
- [Path-Based Tools](#path-based-tools)
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

This spec defines the artifact MCP tools used by the Project Manager Agent (PMA), Project Analyst Agent (PAA), and (where allowed) the Sandbox Agent (SBA) to create, read, list, update, and delete **artifacts**.

Artifacts are **not** scoped to tasks or jobs.
Each artifact is stored in a **scope partition**: **user**, **group**, **project**, or **global**.
The scope defines **ownership** (which partition the path lives in); **RBAC** and policy determine whether a caller may read, create, update, or delete a blob in that partition, including access by principals who are not the original owner when policy allows.

**Jobs** (and optionally **tasks**) are **associated** with an artifact for **lineage and audit** (e.g. which job created or last modified the blob); they do **not** form the storage namespace.
Implementation MUST use the [unified artifacts API](../orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsapicrud) with scope and RBAC enforced by the gateway.

Related documents

- [Orchestrator Artifacts Storage](../orchestrator_artifacts_storage.md)

## Artifact Scope, Ownership, and RBAC

- Spec ID: `CYNAI.MCPTOO.ArtifactScopeRbac` <a id="spec-cynai-mcptoo-artifactscoperbac"></a>

**Scope level** (`user` | `group` | `project` | `global`) selects the **owner partition** for the artifact (same idea as skill scope partitions in [`SkillRegistry` Scope](../skills_storage_and_inference.md#spec-cynai-skills-skillregistryscope), applied to blob storage).

- **User**: Path is unique per owning user (subject user for the row).
- **Group**: Path is unique within a **group**; `group_id` is required.
- **Project**: Path is unique within a **project**; `project_id` is required.
- **Global**: Deployment-wide partition; only principals with appropriate **global** permission may create or mutate (policy-defined).

### Role-Based Access Control

- The gateway MUST enforce [RBAC for artifacts](../orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsrbac): the artifact's scope row determines **default** ownership; **role bindings and access policy** MAY grant **read**, **write**, or **delete** to **other users or groups** (e.g. project members editing a project-scoped artifact created by another user, or group members accessing group-scoped blobs).
- Deny by default when no rule permits the operation for the subject.

### Job and Task Association (Lineage Only)

- Optional **`job_id`** on a write records which job performed the create or update (for audit and analysis).
- Optional **`task_id`** MAY be supplied for **correlation** with task context when available; it MUST NOT be used as the primary key for artifact storage or authorization.

## Definition Compliance

Tool definitions for these tools MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, and `Tools` (single direct `ToolInvocation` per catalog tool name).
Endpoint resolution applies only when defining external tools (Server not `default`).

## Path-Based Tools

- Spec ID: `CYNAI.MCPTOO.ArtifactTools` <a id="spec-cynai-mcptoo-artifacttools"></a>

PMA, PAA, and SBA (when on the worker allowlist) use these tools to read and write artifacts **by path** within a **declared scope**.

Common arguments:

- **`scope`** (string, required for put/list; required for get unless implied): `user` | `group` | `project` | `global`.
- **Scope anchors** (required depending on `scope`): `group_id` (uuid) when `scope=group`; `project_id` (uuid) when `scope=project`; when `scope=user`, the effective user is the subject user (or explicit `user_id` when the gateway allows acting for that user).
- **`path`** (string): Logical path within the partition (non-empty; safe path; no traversal outside the partition).
- **`job_id`** (uuid, optional): Records lineage for create/update when the call originates from a job (e.g. SBA); gateway MAY infer from job-bound MCP context.
- **`task_id`** (uuid, optional): Correlation only; not used for addressing or primary authorization.

### `artifact.put` Operation

- Spec ID: `CYNAI.MCPTOO.ArtifactPut` <a id="spec-cynai-mcptoo-artifactput"></a>

#### `artifact.put` Inputs

- **Required args**: `path` (string), `content_bytes_base64` (string), **`scope`** (`user` | `group` | `project` | `global`), and scope anchors as required (`group_id`, `project_id`, or neither for user/global per catalog rules).
- **Optional args**: `job_id` (uuid; lineage), `task_id` (uuid; correlation only), `user_id` (uuid; when gateway allows acting for a subject user).
- **Scope (MCP)**: Default `pm` for PMA/PAA; sandbox (or both) for SBA per allowlist.

#### `artifact.put` Outputs

- On success: structured result with `status: success` and confirmation (e.g. path and `artifact_id`); response MUST be size-limited per [Response and Error Model](../mcp/mcp_tooling.md#spec-cynai-mcptoo-toolresponse).
- On error: `status: error`, `error` object with `type`, `message`, optional `details`; MUST NOT leak secrets.

#### `artifact.put` Behavior

Gateway resolves caller, enforces allowlist and MCP scope, validates scope anchors and RBAC for **write**, then writes the blob via the artifacts backend.
See [artifact.put Algorithm](#algo-cynai-mcptoo-artifactput).

#### `artifact.put` Algorithm

<a id="algo-cynai-mcptoo-artifactput"></a>

1. Resolve caller identity and agent type; reject if unauthenticated.
2. Check tool allowlist and per-invocation MCP scope; deny if not allowed.
3. Validate `scope`, path, and scope anchors; resolve optional `job_id` / `task_id` from arguments or job-bound context.
4. **RBAC:** Determine whether the subject may **create or overwrite** an artifact in this scope partition (owner rules and grants per [RBAC for artifacts](../orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsrbac)); if not, return 403.
5. Decode `content_bytes_base64`; enforce max blob size; reject if over limit.
6. Call artifacts API to write blob at **scope + path**; persist lineage (`job_id`, `task_id` metadata) per backend schema.
7. Emit audit record (tool name, scope, path, job_id if present, decision, outcome).
8. Return success result with `artifact_id` or reference; size-limited response.

#### `artifact.put` Error Conditions

- **Invalid argument**: missing or malformed `path`, `content_bytes_base64`, or scope fields.
- **Size limit exceeded**: content exceeds configured max.
- **Access denied**: allowlist, MCP scope, or RBAC denies write.
- **Not found**: referenced `group_id` / `project_id` does not exist or caller has no access.
- **Backend failure**: artifacts API or storage error; safe message only.

#### Traces to (`artifact.put`)

- [REQ-MCPTOO-0106](../../requirements/mcptoo.md#req-mcptoo-0106)
- [REQ-MCPTOO-0107](../../requirements/mcptoo.md#req-mcptoo-0107)

### `artifact.get` Operation

- Spec ID: `CYNAI.MCPTOO.ArtifactGet` <a id="spec-cynai-mcptoo-artifactget"></a>

#### `artifact.get` Inputs

- **Required args**: `path` (string), **`scope`** and scope anchors (same rules as `artifact.put`), unless the gateway resolves scope unambiguously from authenticated context (document in Help when inference is allowed).
- **Optional args**: `task_id` (correlation only; ignored for lookup).
- **Scope (MCP)**: Default `pm`.

#### `artifact.get` Outputs

- On success: result object with content (e.g. `content_bytes_base64`) and optional metadata; size-limited and schema-validated.
- On error: structured error; MUST NOT leak secrets.

#### `artifact.get` Behavior

Gateway enforces allowlist, MCP scope, **RBAC for read**, then reads from the artifact store for **scope + path**.
See [artifact.get Algorithm](#algo-cynai-mcptoo-artifactget).

#### `artifact.get` Algorithm

<a id="algo-cynai-mcptoo-artifactget"></a>

1. Resolve caller; check allowlist and MCP scope.
2. Validate `path`, `scope`, and anchors.
3. **RBAC:** Allow read if policy permits (owner or grant).
4. Fetch blob; enforce response size limit.
5. Audit; return content.

#### `artifact.get` Error Conditions

- Invalid args; access denied; not found; backend failure.

#### Traces to (`artifact.get`)

- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)

### `artifact.list` Operation

- Spec ID: `CYNAI.MCPTOO.ArtifactList` <a id="spec-cynai-mcptoo-artifactlist"></a>

#### `artifact.list` Inputs

- **Required args**: **`scope`** and scope anchors (same as above) for the partition to list.
- **Optional args**: `limit`, `cursor`, `task_id` (filter by correlation metadata only, if backend supports it).
- **Scope (MCP)**: Default `pm`.

#### `artifact.list` Outputs

- On success: list of paths (and optional metadata); size-limited; paginated when applicable.

#### `artifact.list` Behavior

Gateway enforces RBAC for **list** in the given scope, then lists paths in that partition.
See [artifact.list Algorithm](#algo-cynai-mcptoo-artifactlist).

#### `artifact.list` Algorithm

<a id="algo-cynai-mcptoo-artifactlist"></a>

1. Resolve caller; check allowlist and MCP scope.
2. Validate scope and anchors.
3. **RBAC:** Caller must be allowed to list artifacts in this partition.
4. Call backend list; apply limit and cursor.
5. Audit; return list.

#### `artifact.list` Error Conditions

- Invalid args; access denied; backend failure.

#### Traces to (`artifact.list`)

- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)

## Unified API (Artifact\_id-Based) Tools

- Spec ID: `CYNAI.MCPTOO.ArtifactsUnifiedTools` <a id="spec-cynai-mcptoo-artifactsunifiedtools"></a>

Full CRUD by `artifact_id`.
Create requests MUST include **scope** and anchors (user / group / project / global), not task or job as the storage key.
**Job** and **task** ids MAY appear only as optional **lineage** or **correlation** metadata on create or update.

### `artifacts.create` Operation

- **Inputs**: Required **`scope`** and anchors, content (e.g. base64); optional `filename`, `content_type`, `job_id`, `task_id` (metadata only).
- **Scope (MCP)**: `pm` (and sandbox when allowed for SBA).
- **Behavior**: Gateway validates scope, RBAC for create, size limits, calls unified artifacts API; stores lineage fields when provided.

### `artifacts.get` Operation

- **Inputs**: Required `artifact_id` (uuid).
- **Behavior**: Resolve row; **RBAC** for read (owner or grant); return blob.

### `artifacts.update` Operation

- **Inputs**: Required `artifact_id`, replacement content; optional `job_id` for last-modified lineage.
- **Behavior**: **RBAC** for write; update blob and metadata.

### `artifacts.delete` Operation

- **Inputs**: Required `artifact_id`; optional `job_id` for audit.
- **Behavior**: **RBAC** for delete; remove or soft-delete per backend.

#### Traces to (Artifacts CRUD)

- [REQ-MCPTOO-0118](../../requirements/mcptoo.md#req-mcptoo-0118)

## Allowlist and Scope

- Spec ID: `CYNAI.MCPTOO.ArtifactToolsAllowlist` <a id="spec-cynai-mcptoo-artifacttoolsallowlist"></a>

- **Allowlist**: PMA and PAA per [Project Manager Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-pmagentallowlist) and [Project Analyst Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-paagentallowlist); SBA per [Worker Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-workeragentallowlist) when `artifact.*` is enabled.
- **MCP scope**: Align with [Per-tool scope: Sandbox vs PM](access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).
- Gateway MUST enforce **artifact RBAC** on every call regardless of allowlist (allowlist is not authorization by itself).
