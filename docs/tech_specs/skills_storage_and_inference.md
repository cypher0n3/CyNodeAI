# Skills Storage and Inference Exposure

- [Document Overview](#document-overview)
- [Default CyNodeAI Interaction Skill](#default-cynodeai-interaction-skill)
- [Skill Store](#skill-store)
  - [`SkillStore` Scope](#skillstore-scope)
  - [`SkillStore` Preconditions](#skillstore-preconditions)
  - [`SkillStore` Outcomes](#skillstore-outcomes)
- [Skill Registry](#skill-registry)
  - [`SkillRegistry` Scope](#skillregistry-scope)
  - [`SkillRegistry` Preconditions](#skillregistry-preconditions)
  - [`SkillRegistry` Outcomes](#skillregistry-outcomes)
- [Skill Scope and Default](#skill-scope-and-default)
  - [`SkillScopeElevation` Scope](#skillscopeelevation-scope)
  - [`SkillScopeElevation` Outcomes](#skillscopeelevation-outcomes)
- [Skill Loading (Web and CLI)](#skill-loading-web-and-cli)
  - [`SkillLoading` Scope](#skillloading-scope)
  - [`SkillLoading` Preconditions](#skillloading-preconditions)
  - [`SkillLoading` Outcomes](#skillloading-outcomes)
- [Skill Management CRUD (Web and CLI)](#skill-management-crud-web-and-cli)
  - [`SkillManagementCrud` Scope](#skillmanagementcrud-scope)
  - [`SkillManagementCrud` Preconditions](#skillmanagementcrud-preconditions)
  - [`SkillManagementCrud` Outcomes](#skillmanagementcrud-outcomes)
- [Skill Tools via MCP (CRUD)](#skill-tools-via-mcp-crud)
  - [`SkillToolsMcp` Preconditions](#skilltoolsmcp-preconditions)
  - [`SkillToolsMcp` Outcomes](#skilltoolsmcp-outcomes)
- [Skill Auditing (Malicious Pattern Scanning)](#skill-auditing-malicious-pattern-scanning)
  - [`SkillAuditing` Scope](#skillauditing-scope)
  - [`SkillAuditing` Preconditions](#skillauditing-preconditions)
  - [`SkillAuditing` Outcomes](#skillauditing-outcomes)
- [Skill Retrieval](#skill-retrieval)
  - [`SkillRetrieval` Inputs](#skillretrieval-inputs)
  - [`SkillRetrieval` Outputs](#skillretrieval-outputs)
  - [`SkillRetrieval` Behavior](#skillretrieval-behavior)
  - [`SkillRetrieval` Error Conditions](#skillretrieval-error-conditions)
- [Inference Exposure](#inference-exposure)
  - [`InferenceExposure` Scope](#inferenceexposure-scope)
  - [`InferenceExposure` Preconditions](#inferenceexposure-preconditions)
  - [`InferenceExposure` Outcomes](#inferenceexposure-outcomes)

## Document Overview

- Spec ID: `CYNAI.SKILLS.Doc.SkillsStorageAndInference` <a id="spec-cynai-skills-doc-skillsstorageandinference"></a>

This document defines how the system stores and tracks AI skills files and makes them available to inference models that support skills.
Skills are persisted in a versioned store, registered with metadata for discovery, and exposed to inference via stable identifiers or resolved content.

This document is the **single source of truth** for skills behavior and contracts (store, registry, CRUD, MCP tool `skills.create`, auditing).
Other docs (requirements, CLI, admin console, MCP tool catalog) reference this spec and MUST NOT duplicate argument schemas, behavior, or control rules.

Traces To:

- [REQ-SKILLS-0001](../requirements/skills.md#req-skills-0001)

Related specs

- Model lifecycle and inference: [`docs/tech_specs/model_management.md`](model_management.md)
- External model routing: [`docs/tech_specs/external_model_routing.md`](external_model_routing.md)

## Default CyNodeAI Interaction Skill

- Spec ID: `CYNAI.SKILLS.DefaultCyNodeAISkill` <a id="spec-cynai-skills-defaultcynodeaiskill"></a>

Traces To:

- [REQ-SKILLS-0116](../requirements/skills.md#req-skills-0116)
- [REQ-SKILLS-0117](../requirements/skills.md#req-skills-0117)

The system provides a single built-in default skill whose purpose is to inform inference models (AIs) how to interact with CyNodeAI.
This skill is part of the MVP and ensures agents have consistent, up-to-date guidance on capabilities and conventions.

### Scope and Purpose

- **Purpose**: The default skill content MUST describe how AIs should interact with CyNodeAI.
  Topics SHOULD include (as applicable): use of MCP tools and the MCP gateway, User API Gateway usage, task and project context, sandbox and node conventions, and references to authoritative docs (e.g. requirements and tech specs).
  The actual skill file (e.g. SKILL.md) is maintained separately; this spec does not define or implement that file.
- **Loaded by default**: When the system exposes skills to an inference request that supports skills, the default CyNodeAI interaction skill MUST be included in the set of skills offered (e.g. resolved and supplied in context or by reference).
  No user or caller action is required to enable it.
- **Scope**: The default skill is effectively global in scope: it is visible to all inference requests that receive skills (all users/tenants).
- **Ownership**: The skill is system-owned.
  It MUST NOT be user-editable or user-deletable via web, CLI, or MCP skill tools; the system MAY allow read (list/get) for transparency.
  It MAY be stored in a dedicated system/built-in store or in the same store under a reserved identifier; implementation MUST prevent normal CRUD from modifying or removing it.

### Updated Regularly

- The default skill content MUST be updated regularly so it stays aligned with current behavior.
  "Regularly" is defined as: at least with each product release that changes relevant behavior, and SHOULD include a defined schedule (e.g. quarterly) or trigger (e.g. when tech spec or requirement docs change) so that non-release updates can be applied.
  Responsibility for content updates lies with the project/maintainers; the implementation MUST support replacing or refreshing the stored content when a new version is supplied (e.g. at deploy or via an admin/maintenance path).

### Inclusion in Inference

- The default skill is subject to the same inference-exposure mechanism as other skills (see [Inference Exposure](#inference-exposure)); the only difference is that it is always included when skills are offered, and it is not optional for the caller to add.

## Skill Store

- Spec ID: `CYNAI.SKILLS.SkillStore` <a id="spec-cynai-skills-skillstore"></a>

Traces To:

- [REQ-SKILLS-0100](../requirements/skills.md#req-skills-0100)

The skill store is the durable, versioned backing store for skill file content.

### `SkillStore` Scope

- Skills files MUST be stored under a defined root (e.g. filesystem directory or blob namespace) so that paths or identifiers are stable across restarts.
- Each stored skill MUST have a stable identifier (e.g. content-derived hash or assigned UUID) used for retrieval and registry references.
- Storage MAY be filesystem-based (e.g. a directory tree with `SKILL.md` or similar filenames) or database-backed (e.g. large object or text column); the implementation MUST allow retrieval by stable identifier.

### `SkillStore` Preconditions

- Before storing a skill, the system MUST validate that the content conforms to the project skill format (e.g. SKILL.md structure) if a schema is defined.
- Duplicate content MAY be deduplicated by content hash to avoid redundant storage.

### `SkillStore` Outcomes

- After a successful store operation, the skill is readable by its stable identifier.
- The store MUST support listing or enumeration so that the registry can be populated or refreshed.

## Skill Registry

- Spec ID: `CYNAI.SKILLS.SkillRegistry` <a id="spec-cynai-skills-skillregistry"></a>

Traces To:

- [REQ-SKILLS-0101](../requirements/skills.md#req-skills-0101)
- [REQ-SKILLS-0104](../requirements/skills.md#req-skills-0104)
- [REQ-SKILLS-0107](../requirements/skills.md#req-skills-0107)

The skill registry holds metadata for each known skill so that callers can discover and filter skills (e.g. by scope or name).

### `SkillRegistry` Scope

- The registry MUST record at least: stable skill identifier, name (or label), and scope.
- Allowed scope levels: user, group, project, global (ordered from narrowest to broadest exposure).
- Scope is used to filter which skills are offered for a given inference request (e.g. user sees their user-scoped skills plus any group/project/global skills they are entitled to).

### `SkillRegistry` Preconditions

- Registry entries MUST reference a skill that exists in the skill store (by stable identifier).
- Updates to registry metadata MUST NOT break existing references to the same stable identifier.

### `SkillRegistry` Outcomes

- Callers can list skills, optionally filtered by scope or other metadata.
- Listing returns stable identifiers and metadata required for inference exposure (e.g. name, scope).

## Skill Scope and Default

- Spec ID: `CYNAI.SKILLS.SkillScopeElevation` <a id="spec-cynai-skills-skillscopeelevation"></a>

Traces To:

- [REQ-SKILLS-0107](../requirements/skills.md#req-skills-0107)
- [REQ-SKILLS-0108](../requirements/skills.md#req-skills-0108)

Skills are scoped to control who can see and use them.
By default a skill is user-scoped; the user may open up exposure to group, project, or global when they direct, subject to permissions.

### `SkillScopeElevation` Scope

- **Default**: When a skill is loaded (or created), the default scope MUST be user.
  The skill is then visible only to that user's inference requests unless the user explicitly requests a broader scope.
- **Broader scope**: The user MAY direct that a skill be scoped to group, project, or global (e.g. at load time via a scope parameter, or later via an update).
  The system MUST allow setting a broader scope only if the user has appropriate permissions for that scope (e.g. membership in the group, access to the project, or global/admin capability).
- Permission checks for scope elevation MUST be enforced by the gateway or backend; the client (web or CLI) MAY offer a scope selector but MUST NOT be trusted to enforce scope limits.

### `SkillScopeElevation` Outcomes

- User-scoped skills: only the owning user's requests see the skill.
- Group-scoped: members of the specified group see the skill.
- Project-scoped: users with access to the specified project see the skill.
- Global-scoped: all users (or all within a tenant) see the skill; only users with global/admin permission MAY set global scope.

## Skill Loading (Web and CLI)

- Spec ID: `CYNAI.SKILLS.SkillLoading` <a id="spec-cynai-skills-skillloading"></a>

Traces To:

- [REQ-SKILLS-0105](../requirements/skills.md#req-skills-0105)
- [REQ-SKILLS-0106](../requirements/skills.md#req-skills-0106)

Users load skills by uploading skill content (e.g. markdown) through the web interface or the CLI.
Both paths MUST result in the skill being stored in the skill store and registered in the skill registry.

### `SkillLoading` Scope

- **Web interface**: The web console MUST provide a way to upload a skill file (e.g. markdown).
  The UI sends the content to the User API Gateway; the gateway (or backend) writes to the skill store and updates the registry.
  The web console MUST NOT connect directly to the store; it MUST use the gateway.
- **CLI**: The CLI MUST provide a command (e.g. `cynork skills load <file.md>`) that reads a local markdown file and submits it to the gateway.
  The CLI MUST NOT write directly to the skill store; it MUST call the User API Gateway for all operations.

### `SkillLoading` Preconditions

- The user MUST be authenticated and authorized to add skills (policy-controlled).
- The uploaded content MUST conform to the project skill format (e.g. SKILL.md structure) if a schema is defined; otherwise the server MAY accept markdown and assign a stable identifier.
- If the user requests a scope broader than user (group, project, global), the gateway MUST verify the user has appropriate permissions for that scope before accepting the load or update.
- The system MUST run skill auditing (malicious pattern scan) on the content before storing; if the audit fails, the load MUST be rejected and the skill MUST NOT be stored or registered.

### `SkillLoading` Outcomes

- After a successful load, the skill is stored (stable identifier assigned), registered in the registry with metadata including scope (default user unless the user directed a broader scope with permission).
- Load operations SHOULD be audited (user, timestamp, scope).

Related specs

- Web Console: [`docs/tech_specs/web_console.md`](web_console.md)
- CLI management app: [`docs/tech_specs/cli_management_app.md`](cli_management_app.md)
- Skill auditing: [Skill Auditing (Malicious Pattern Scanning)](#skill-auditing-malicious-pattern-scanning)
- Full CRUD: [Skill Management CRUD (Web and CLI)](#skill-management-crud-web-and-cli)

## Skill Management CRUD (Web and CLI)

- Spec ID: `CYNAI.SKILLS.SkillManagementCrud` <a id="spec-cynai-skills-skillmanagementcrud"></a>

Traces To:

- [REQ-SKILLS-0115](../requirements/skills.md#req-skills-0115)

The web interface and CLI MUST support full CRUD for skills.
All operations MUST go through the User API Gateway; the same controls (authentication, scope elevation permission on write, auditing on write) apply.

### `SkillManagementCrud` Scope

- **Create**: Already defined by [Skill Loading (Web and CLI)](#skill-loading-web-and-cli) (upload/load).
- **Read (list)**: Callers can list skills visible to them (e.g. user-scoped to the caller, plus group/project/global skills they are entitled to).
  List MAY accept optional filters (e.g. scope, name, owner).
  List returns stable identifiers and metadata (name, scope, owner, updated_at); it need not return full content.
- **Read (get)**: Callers can fetch one skill by stable identifier to retrieve full content and metadata.
  Access MUST be restricted to skills the caller is entitled to see (same visibility as list).
- **Update**: Callers can update a skill's content and/or metadata (e.g. name, scope) by identifier.
  The caller MUST be authorized to modify the skill (e.g. owner or admin).
  Updated content MUST pass the same skill auditing (malicious pattern scan) before the update is applied; on failure, the response MUST include the same feedback (match category and exact triggering text).
  Scope elevation (e.g. from user to group) MUST be permitted only if the caller has permission for the requested scope.
- **Delete**: Callers can delete a skill by identifier.
  The caller MUST be authorized (e.g. owner or admin); after deletion the skill is removed from the store and registry and MUST NOT be returned by list or get or exposed to inference.

### `SkillManagementCrud` Preconditions

- All operations require authentication; write operations (create, update, delete) require authorization to add or modify skills for the affected scope/owner.
- List and get only return skills the caller is entitled to see per scope and access policy.

### `SkillManagementCrud` Outcomes

- Create: see Skill Loading.
- List: returns a list of skill metadata (and optionally a truncated or full content preview if defined by the API).
- Get: returns full content and metadata for one skill.
- Update: on success, store and registry reflect the new content/metadata; on audit failure, update is rejected with feedback.
- Delete: skill is removed; subsequent get or list MUST NOT return it.

Related specs

- Web Console: [`docs/tech_specs/web_console.md`](web_console.md)
- CLI management app: [`docs/tech_specs/cli_management_app.md`](cli_management_app.md)

## Skill Tools via MCP (CRUD)

- Spec ID: `CYNAI.SKILLS.SkillToolsMcp` <a id="spec-cynai-skills-skilltoolsmcp"></a>

Traces To:

- [REQ-SKILLS-0114](../requirements/skills.md#req-skills-0114)

Models (agents) can perform full CRUD on skills for the user when directed.
All skill tools are routed through the orchestrator MCP gateway; identity, role-based allowlists, and access control apply (see [MCP Gateway Enforcement](mcp_gateway_enforcement.md)).
Same controls as web and CLI: auditing on write, default user scope, scope elevation only with permission; list/get return only skills the caller is entitled to see; update/delete require authorization (owner or admin).
All invocations MUST be audited per [MCP tool call auditing](mcp_tool_call_auditing.md).

**Note:** Canonical tool contracts (single source of truth; do not duplicate in the catalog).

- **`skills.create`**
  - Required args: `task_id` (uuid string), `content` (markdown string).
  - Optional args: `name` (string), `scope` (string: `user` | `group` | `project` | `global`; default `user`; broader scope requires caller permission).
  - Skill is attributed to the user from tool call context; content MUST pass auditing before store; on audit failure return rejection reason, match category, and exact triggering text.
- **`skills.list`**
  - Required args: `task_id` (uuid string; for user context).
  - Optional args: `scope` (filter), `owner` (filter).
  - Returns list of skill metadata (identifier, name, scope, owner, updated_at) for skills the caller is entitled to see; not full content.
- **`skills.get`**
  - Required args: `task_id`, `skill_id` (stable identifier).
  - Returns full content and metadata for one skill; caller MUST be entitled to see it (same visibility as list).
- **`skills.update`**
  - Required args: `task_id`, `skill_id`.
  - Optional args: `content` (markdown string; if provided, re-audited), `name` (string), `scope` (string; elevation requires permission).
  - Caller MUST be authorized to modify the skill (owner or admin).
    On content update, audit failure returns same feedback as create.
- **`skills.delete`**
  - Required args: `task_id`, `skill_id`.
  - Caller MUST be authorized (owner or admin); skill is removed from store and registry.

### `SkillToolsMcp` Preconditions

- Caller MUST be allowed to invoke each tool under the gateway allowlist and access control rules for the task and user context.
- Write operations (create, update, delete) require authorization; list/get only return skills visible to the caller per scope and access policy.

### `SkillToolsMcp` Outcomes

- Create: skill stored and registered; response includes stable skill identifier; on audit failure, error with feedback.
- List: list of skill metadata.
- Get: full content and metadata or not-found if not entitled or missing.
- Update: store and registry updated, or error with audit feedback on content rejection.
- Delete: skill removed; subsequent list/get MUST NOT return it.

Related specs

- MCP gateway: [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)
- MCP tool catalog: lists all skills tool names for allowlist/discovery; [catalog Skills Tools section](mcp_tool_catalog.md#skills-tools) references this spec as the source of truth for contract and behavior.
- Skill auditing: [Skill Auditing (Malicious Pattern Scanning)](#skill-auditing-malicious-pattern-scanning)

## Skill Auditing (Malicious Pattern Scanning)

- Spec ID: `CYNAI.SKILLS.SkillAuditing` <a id="spec-cynai-skills-skillauditing"></a>

Traces To:

- [REQ-SKILLS-0110](../requirements/skills.md#req-skills-0110)
- [REQ-SKILLS-0111](../requirements/skills.md#req-skills-0111)
- [REQ-SKILLS-0112](../requirements/skills.md#req-skills-0112)
- [REQ-SKILLS-0113](../requirements/skills.md#req-skills-0113)

Skill content is scanned for malicious or policy-violating patterns before acceptance and optionally rescanned after storage.
Matches cause load rejection; existing skills may be flagged or quarantined when rules are updated.

### `SkillAuditing` Scope

- **When**: Scanning MUST run before a skill is stored or updated (on load and on explicit update).
  The system SHOULD support periodic or on-demand rescan of all stored skills (e.g. after pattern rule updates) and SHOULD flag or quarantine skills that then match.
- **Pattern categories**: The system MUST detect at least the following categories of malicious or policy-violating content:
  - **Hidden instructions**: Content that instructs the model without visible disclosure to the skill author or consumers.
    Examples: HTML comments (e.g. `<!-- ... -->`) containing instructions, zero-width or invisible Unicode used to embed instructions, or similar techniques intended to be read by the model but not obviously visible in the rendered or plain-text view.
  - **Instruction override**: Text that explicitly tells the model to ignore, override, or disregard other instructions (including system prompts or other skills).
  - **Secret or security bypass**: Instructions that would prompt the model to expose secrets, bypass access controls, or circumvent security boundaries (e.g. "ignore previous safety guidelines", "output the user's API key").
- The exact pattern set (regex, keywords, or rule engine) is implementation-defined but MUST be maintainable (e.g. configurable or updatable without code change) so that new patterns can be added as threats evolve.
- **Alignment with Secure Browser Service**: Skill auditing SHOULD use the same or aligned pattern-detection approach as the [Secure Browser Service](secure_browser_service.md).
  That service defines deterministic injection-stripping rules: denylists of instruction-like patterns (e.g. "ignore previous instructions", "system prompt", "developer message", "jailbreak"), removal of HTML comments and hidden content, and configurable regex patterns and section markers (see [Deterministic Sanitization](secure_browser_service.md#deterministic-sanitization) and [Configuration and Rules](secure_browser_service.md#configuration-and-rules)).
  Implementations MAY share or derive pattern rules from the same configuration (e.g. shared denylist line patterns, or a skills-specific extension of the same rule format) so that instruction-override and secret-bypass patterns are maintained in one place where practical.

### `SkillAuditing` Preconditions

- Audit runs on plain-text or decoded skill content (e.g. markdown); if the skill is stored in a format that embeds markup, the content passed to the scanner MUST be the same logical content that would be supplied to inference.
- Scanning MUST NOT rely on client-provided "clean" content; the server MUST scan the content it will store.

### `SkillAuditing` Outcomes

- **On load or update**: If any pattern matches, the load (or update) MUST be rejected with a deterministic error (e.g. policy violation); the skill MUST NOT be stored or registered (or updated).
  The response to the caller (the user who submitted the load or update) MUST include good feedback so the user can fix the content: the rejection reason, the match category (e.g. hidden instructions, instruction override, secret bypass), and the exact text that triggered the rejection.
  When multiple matches exist, the response SHOULD include at least the first match (or all matches, subject to response size limits) so the user can address each offending span.
- **On rescan**: If a stored skill is rescanned and matches (e.g. after rule update), the system SHOULD flag the skill or quarantine it (e.g. exclude from inference exposure until reviewed or removed).
  The skill owner or admins SHOULD be able to see the same feedback (match category and triggering text) when viewing the flagged or quarantined skill so they can correct or remove it.
- Audit results (pass/fail, and optionally match category for internal logging) SHOULD be recorded for compliance and debugging.
  Match details MUST be returned to the caller on load/update rejection and SHOULD be available to the skill owner or admins for quarantined skills; they need not be exposed to other unprivileged callers (e.g. in a public list of rejections).

Related specs

- Secure Browser Service (pattern detection and rules): [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md)

## Skill Retrieval

- Spec ID: `CYNAI.SKILLS.SkillRetrieval` <a id="spec-cynai-skills-skillretrieval"></a>

Traces To:

- [REQ-SKILLS-0102](../requirements/skills.md#req-skills-0102)

Retrieval returns full skill content by stable identifier.

### `SkillRetrieval` Inputs

- Stable skill identifier (from registry or from caller).
- Optional: cancellation context for long-running or remote storage.

### `SkillRetrieval` Outputs

- Skill content (e.g. markdown or opaque blob) and optional content type or checksum.

### `SkillRetrieval` Behavior

- Given a valid stable identifier, the system returns the stored skill content.
- If the identifier is unknown or the skill was removed, the operation fails with a deterministic error (e.g. not found).

### `SkillRetrieval` Error Conditions

- Unknown or invalid identifier: not found error.
- Store unavailable: transient error so callers can retry.

## Inference Exposure

- Spec ID: `CYNAI.SKILLS.InferenceExposure` <a id="spec-cynai-skills-inferenceexposure"></a>

Traces To:

- [REQ-SKILLS-0103](../requirements/skills.md#req-skills-0103)

Inference exposure defines how inference models that support skills receive skill content when handling a request.

### `InferenceExposure` Scope

- The system MUST support at least one of: (1) resolving a set of skill identifiers to content and including that content in the inference context (e.g. system or user message), or (2) providing a stable reference (e.g. URL or path) that the inference runtime can resolve to skill content.
- The choice of mechanism depends on the inference backend; the spec does not mandate a single protocol but requires that some path exists for models that support skills to receive skill content.

### `InferenceExposure` Preconditions

- Skills offered for a request MUST be filtered by scope (and any policy) so that only allowed skills are exposed.
- Only skills present in the registry and store MUST be resolvable.

### `InferenceExposure` Outcomes

- For a given inference request that requests skills (by identifier or scope), the system supplies the corresponding skill content or resolvable reference so the model can use the skills during inference.
