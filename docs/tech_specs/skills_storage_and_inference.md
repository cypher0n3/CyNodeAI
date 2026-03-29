# Skills Storage and Inference Exposure

- [Document Overview](#document-overview)
  - [Traces to Requirements](#traces-to-requirements)
- [Default CyNodeAI Interaction Skill](#default-cynodeai-interaction-skill)
  - [Default CyNodeAI Interaction Skill Requirements Traces](#default-cynodeai-interaction-skill-requirements-traces)
  - [Scope and Purpose](#scope-and-purpose)
  - [Updated Regularly](#updated-regularly)
  - [Inclusion in Inference](#inclusion-in-inference)
- [Skill Store](#skill-store)
  - [Skill Store Requirements Traces](#skill-store-requirements-traces)
  - [`SkillStore` Scope](#skillstore-scope)
  - [`SkillStore` Preconditions](#skillstore-preconditions)
  - [`SkillStore` Name Uniqueness (Database-Backed Stores)](#skillstore-name-uniqueness-database-backed-stores)
  - [`SkillStore` Outcomes](#skillstore-outcomes)
- [Skill Registry](#skill-registry)
  - [Skill Registry Requirements Traces](#skill-registry-requirements-traces)
  - [`SkillRegistry` Scope](#skillregistry-scope)
  - [`SkillRegistry` Preconditions](#skillregistry-preconditions)
  - [`SkillRegistry` Outcomes](#skillregistry-outcomes)
- [Skill Scope and Default](#skill-scope-and-default)
  - [Skill Scope and Default Requirements Traces](#skill-scope-and-default-requirements-traces)
  - [`SkillScopeElevation` Scope](#skillscopeelevation-scope)
  - [`SkillScopeElevation` Outcomes](#skillscopeelevation-outcomes)
- [Skill Loading (Web and CLI)](#skill-loading-web-and-cli)
  - [Skill Loading (Web and CLI) Requirements Traces](#skill-loading-web-and-cli-requirements-traces)
  - [`SkillLoading` Scope](#skillloading-scope)
  - [`SkillLoading` Preconditions](#skillloading-preconditions)
  - [`SkillLoading` Outcomes](#skillloading-outcomes)
- [Skill Management CRUD (Web and CLI)](#skill-management-crud-web-and-cli)
  - [Skill Management CRUD (Web and CLI) Requirements Traces](#skill-management-crud-web-and-cli-requirements-traces)
  - [`SkillManagementCrud` Scope](#skillmanagementcrud-scope)
  - [`SkillManagementCrud` Preconditions](#skillmanagementcrud-preconditions)
  - [`SkillManagementCrud` Outcomes](#skillmanagementcrud-outcomes)
- [Effective Skill Set and Resolution (Backend Semantics)](#effective-skill-set-and-resolution-backend-semantics)
  - [Effective Skill Set (Merge)](#effective-skill-set-merge)
  - [Resolution by Logical Name](#resolution-by-logical-name)
  - [Resolution Context (`project_id`)](#resolution-context-project_id)
- [Skills MCP Tools (Reference)](#skills-mcp-tools-reference)
  - [Related Specs](#related-specs)
  - [Skills MCP Tools (Reference) Requirements Traces](#skills-mcp-tools-reference-requirements-traces)
- [Skill Auditing (Malicious Pattern Scanning)](#skill-auditing-malicious-pattern-scanning)
  - [Skill Auditing (Malicious Pattern Scanning) Requirements Traces](#skill-auditing-malicious-pattern-scanning-requirements-traces)
  - [`SkillAuditing` Scope](#skillauditing-scope)
  - [`SkillAuditing` Preconditions](#skillauditing-preconditions)
  - [`SkillAuditing` Outcomes](#skillauditing-outcomes)
- [Skill Retrieval](#skill-retrieval)
  - [Skill Retrieval Requirements Traces](#skill-retrieval-requirements-traces)
  - [`SkillRetrieval` Inputs](#skillretrieval-inputs)
  - [`SkillRetrieval` Outputs](#skillretrieval-outputs)
  - [`SkillRetrieval` Behavior](#skillretrieval-behavior)
  - [`SkillRetrieval` Error Conditions](#skillretrieval-error-conditions)
- [Inference Exposure](#inference-exposure)
  - [Inference Exposure Requirements Traces](#inference-exposure-requirements-traces)
  - [`InferenceExposure` Scope](#inferenceexposure-scope)
  - [`InferenceExposure` Preconditions](#inferenceexposure-preconditions)
  - [`InferenceExposure` Outcomes](#inferenceexposure-outcomes)

## Document Overview

- Spec ID: `CYNAI.SKILLS.Doc.SkillsStorageAndInference` <a id="spec-cynai-skills-doc-skillsstorageandinference"></a>

This document defines how the system **stores** and **tracks** AI skill files and makes them available to inference models that support skills: **durable store**, **registry**, **scope and uniqueness**, **effective skill resolution** (merge by name), **content auditing**, **retrieval**, and **inference exposure**.

**MCP tool calling** (protocol, gateway, allowlists, tokens, per-tool enforcement) is authoritative in [MCP Tooling](mcp/mcp_tooling.md), [MCP Gateway Enforcement](mcp/mcp_gateway_enforcement.md), and [MCP tool specifications](mcp_tools/README.md).
The **`skills.*`** tool **names, arguments, and gateway-facing contracts** are canonical in [Skills MCP Tools](mcp_tools/skills_tools.md#spec-cynai-skills-skilltoolsmcp).

This document is the **single source of truth** for **backend / orchestrator / database** behavior for skills: persistence rules, merge semantics, and safety scanning-not for duplicating MCP gateway mechanics.

### Traces to Requirements

- [REQ-SKILLS-0001](../requirements/skills.md#req-skills-0001)

Related specs

- Model lifecycle and inference: [`docs/tech_specs/model_management.md`](model_management.md)
- External model routing: [`docs/tech_specs/external_model_routing.md`](external_model_routing.md)

## Default CyNodeAI Interaction Skill

- Spec ID: `CYNAI.SKILLS.DefaultCyNodeAISkill` <a id="spec-cynai-skills-defaultcynodeaiskill"></a>

### Default CyNodeAI Interaction Skill Requirements Traces

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
- **Scope**: The default skill is effectively global in scope: it is visible to all inference requests that receive skills (all users).
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

### Skill Store Requirements Traces

- [REQ-SKILLS-0100](../requirements/skills.md#req-skills-0100)

The skill store is the durable, versioned backing store for skill file content.

### `SkillStore` Scope

- Skills files MUST be stored under a defined root (e.g. filesystem directory or blob namespace) so that paths or identifiers are stable across restarts.
- Each stored skill MUST have a stable identifier (e.g. content-derived hash or assigned UUID) used for retrieval and registry references.
- Storage MAY be filesystem-based (e.g. a directory tree with `SKILL.md` or similar filenames) or database-backed (e.g. large object or text column); the implementation MUST allow retrieval by stable identifier.

### `SkillStore` Preconditions

- Before storing a skill, the system MUST validate that the content conforms to the project skill format (e.g. SKILL.md structure) if a schema is defined.
- Duplicate content MAY be deduplicated by content hash to avoid redundant storage.

### `SkillStore` Name Uniqueness (Database-Backed Stores)

- Spec ID: `CYNAI.SKILLS.SkillStoreNameUniqueness` <a id="spec-cynai-skills-skillstorenameuniqueness"></a>

When skills are persisted in a **database**, the schema MUST enforce **unique skill `name` within each scope partition**; not uniqueness of `name` across the whole table.

- **Partitioning**: A partition is the set of rows that share the same **scope level** and the same **scope anchor** (the user, group, or project the row belongs to; global has a single deployment-wide partition).
- **User** scope: at most **one** stored skill per **(owning user, normalized `name`)** for that user's user-scoped skills.
- **Group** scope: at most **one** stored skill per **(group id, normalized `name`)** within that group.
- **Project** scope: at most **one** stored skill per **(project id, normalized `name`)** within that project.
- **Global** scope: at most **one** stored skill per **(normalized `name`)** in the global partition.
- **Across partitions**: The **same** logical `name` MAY exist in **different** partitions (e.g. a user-scoped skill and a project-scoped skill with the same name); that is expected and resolved by [merge precedence](#effective-skill-set-merge), not by rejecting the insert.
- **Normalization**: `name` comparison for uniqueness MUST use the **same** normalization rules as merge and list (implementation-defined but consistent).

- Create and update operations that would violate a partition's uniqueness MUST fail with a deterministic error (e.g. conflict).

### `SkillStore` Outcomes

- After a successful store operation, the skill is readable by its stable identifier.
- The store MUST support listing or enumeration so that the registry can be populated or refreshed.

## Skill Registry

- Spec ID: `CYNAI.SKILLS.SkillRegistry` <a id="spec-cynai-skills-skillregistry"></a>

### Skill Registry Requirements Traces

- [REQ-SKILLS-0101](../requirements/skills.md#req-skills-0101)
- [REQ-SKILLS-0104](../requirements/skills.md#req-skills-0104)
- [REQ-SKILLS-0107](../requirements/skills.md#req-skills-0107)

The skill registry holds metadata for each known skill so that callers can discover and filter skills (e.g. by scope or name).

### `SkillRegistry` Scope

- Spec ID: `CYNAI.SKILLS.SkillRegistryScope` <a id="spec-cynai-skills-skillregistryscope"></a>

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

### Skill Scope and Default Requirements Traces

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
- Global-scoped: all users in the deployment see the skill; only users with global/admin permission MAY set global scope.

## Skill Loading (Web and CLI)

- Spec ID: `CYNAI.SKILLS.SkillLoading` <a id="spec-cynai-skills-skillloading"></a>

### Skill Loading (Web and CLI) Requirements Traces

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
- A **non-empty skill name** MUST be supplied (e.g. from SKILL.md front matter, a required UI field, or CLI flag); loads without a resolvable name MUST be rejected.
- The uploaded content MUST conform to the project skill format (e.g. SKILL.md structure) if a schema is defined; otherwise the server MAY accept markdown and assign a stable identifier.
- If the user requests a scope broader than user (group, project, global), the gateway MUST verify the user has appropriate permissions for that scope before accepting the load or update.
- The system MUST run skill auditing (malicious pattern scan) on the content before storing; if the audit fails, the load MUST be rejected and the skill MUST NOT be stored or registered.

### `SkillLoading` Outcomes

- After a successful load, the skill is stored (stable identifier assigned), registered in the registry with metadata including scope (default user unless the user directed a broader scope with permission).
- Load operations SHOULD be audited (user, timestamp, scope).

Related specs

- Web Console: [`docs/tech_specs/web_console.md`](web_console.md)
- CLI management app: [`docs/tech_specs/cynork/cynork_cli.md`](cynork/cynork_cli.md)
- Skill auditing: [Skill Auditing (Malicious Pattern Scanning)](#skill-auditing-malicious-pattern-scanning)
- Full CRUD: [Skill Management CRUD (Web and CLI)](#skill-management-crud-web-and-cli)

## Skill Management CRUD (Web and CLI)

- Spec ID: `CYNAI.SKILLS.SkillManagementCrud` <a id="spec-cynai-skills-skillmanagementcrud"></a>

### Skill Management CRUD (Web and CLI) Requirements Traces

- [REQ-SKILLS-0115](../requirements/skills.md#req-skills-0115)

The web interface and CLI MUST support full CRUD for skills.
All operations MUST go through the User API Gateway; the same controls (authentication, scope elevation permission on write, auditing on write) apply.

### `SkillManagementCrud` Scope

- **Create**: Already defined by [Skill Loading (Web and CLI)](#skill-loading-web-and-cli) (upload/load).
- **Read (list)**: Callers can list skills visible to them (e.g. user-scoped to the caller, plus group/project/global skills they are entitled to).
  List MUST return the **effective** skill set for the caller (same merge-by-name rules as [Effective skill set (merge)](#effective-skill-set-merge)), unless a separate **non-MCP** list contract is explicitly documented for that client (e.g. a management console raw inventory-not MCP tool calls).
  List MAY accept optional filters (e.g. scope, name, owner) that narrow the candidate pool before merging.
  List returns stable identifiers and metadata (name, scope, owner, updated_at); it need not return full content.
- **Read (get)**: Callers can fetch one skill to retrieve full content and metadata.
  The **default** web and CLI get path MUST resolve by **`name`** (non-empty string) to the **effective** row using [name precedence](#effective-skill-set-merge) (same semantics as [MCP `skills.get`](mcp_tools/skills_tools.md#spec-cynai-mcptoo-skillsget); see [Resolution by logical name](#resolution-by-logical-name)).
  Fetch by **stable identifier** MUST NOT be the ordinary user-facing path; it MAY exist only on **documented non-MCP** surfaces where the caller must load a **specific stored** row when entitled (no name merge), with authorization appropriate to that surface.
  **ADMIN**-level operations MUST NOT be available via MCP tool calls (see [Skills MCP Tools](mcp_tools/skills_tools.md)).
  Access MUST be restricted to skills the caller is entitled to see (same visibility as list).
- **Update**: Callers can update a skill's content and/or metadata (e.g. name, scope) by identifier.
  The caller MUST be authorized to modify the skill (e.g. owner or delegated role per policy); **ADMIN**-level actions MUST NOT be exposed through MCP tool calls.
  Updated content MUST pass the same skill auditing (malicious pattern scan) before the update is applied; on failure, the response MUST include the same feedback (match category and exact triggering text).
  Scope elevation (e.g. from user to group) MUST be permitted only if the caller has permission for the requested scope.
- **Delete**: Callers can delete a skill by identifier.
  The caller MUST be authorized (e.g. owner or delegated role per policy); **ADMIN**-level actions MUST NOT be exposed through MCP tool calls.
  After deletion the skill is removed from the store and registry and MUST NOT be returned by list or get or exposed to inference.

### `SkillManagementCrud` Preconditions

- All operations require authentication; write operations (create, update, delete) require authorization to add or modify skills for the affected scope/owner.
- List and get only return skills the caller is entitled to see per scope and access policy.

### `SkillManagementCrud` Outcomes

- Create: see Skill Loading (including required non-empty name).
- List: returns **effective** skill metadata (and optionally a truncated or full content preview if defined by the API).
- Get: returns full content and metadata for one skill; the default path uses **`name`** and returns the **effective** row per [Resolution by logical name](#resolution-by-logical-name) (identifier-based get only on documented non-MCP surfaces, not via MCP tool calls).
- Update: on success, store and registry reflect the new content/metadata; on audit failure, update is rejected with feedback.
- Delete: skill is removed; subsequent get or list MUST NOT return it.

Related specs

- Web Console: [`docs/tech_specs/web_console.md`](web_console.md)
- CLI management app: [`docs/tech_specs/cynork/cynork_cli.md`](cynork/cynork_cli.md)

## Effective Skill Set and Resolution (Backend Semantics)

The orchestrator (and any skill service backing the User API or MCP tools) MUST implement **one** consistent model for "which skill rows apply" for a subject user and how **`name`** maps to stored rows.

That model is used for: listing skills, resolving content by logical **`name`**, and building inference exposure from the same rules.

### Effective Skill Set (Merge)

- Spec ID: `CYNAI.SKILLS.EffectiveSkillSetMerge` <a id="spec-cynai-skills-effectiveskillsetmerge"></a>

Operations that return a **list** of skills for a subject user (including MCP `skills.list` and equivalent User API list) MUST return **effective** skills for the **candidate pool** built in step 1 below: **user**-, **group**-, and **global**-level rows (and system defaults when policy includes them), and **project**-level rows **only** when **`project_id`** is supplied on calls that support it (see step 1 and [Resolution context (`project_id`)](#resolution-context-project_id)).

1. **Candidate pool**: Build the set of skill rows the subject user is entitled to see for the operation, per RBAC and membership:
   - **User**-scoped rows for the subject user.
   - **Group**-scoped rows: determine the set of **groups** the user is a member of; include every group-scoped skill row whose **group** is in that set (for a single-name resolution, keep rows whose normalized **`name`** matches).
   - **Project**-scoped rows: if **`project_id`** is supplied on the call (where the API supports it, e.g. MCP `skills.get` and `skills.list`), include only rows in the partition for that project and verify the user is entitled to that project.
     If **`project_id`** is **omitted**, **do not** include any **project**-scoped rows in the candidate pool (project-scoped skills are skipped for that call).
     Tool **Help** text and user-facing docs MUST state that **`project_id`** is required to surface project-scoped skills.
   - **Global** (and system defaults when policy includes them).
2. **Merge by `name` (cross-level precedence)**: After the candidate pool is fixed, collapse rows that share the same **`name`** (case and normalization rules are implementation-defined but MUST be consistent for merge and display):
   - **User**-scoped skill wins over **project**-scoped and **group**-scoped skills with the same name.
   - **Project**-scoped skill wins over **group**-scoped skill with the same name.
   - **Global**-scoped (and system) skills participate at the lowest precedence in this ordering when the same name appears at multiple levels: **user** > **project** > **group** > **global**.
3. **Tie-break within the same scope level**: After step 2, if **more than one** row still shares the same normalized `name` at the **same** winning scope level but with **different** scope anchors, the implementation MUST keep **exactly one** row.
   Typical case: the user is in **multiple groups**, each defining a **group**-scoped skill with that name.
   The same rule applies when multiple **project** partitions (or other distinct anchors at the same level) each contribute a row for that name.
   The winning row MUST be the one with the **latest `updated_at`** (most recently updated).
   If `updated_at` is equal, break ties by stable **`id`**.
   This rule does **not** apply to two rows in the **same** scope partition with the same name-that case remains invalid storage (step 4).
4. **Same partition, same name (disallowed)**: A conforming store MUST NOT contain two rows in the **same** scope partition with the same normalized `name` (see [`SkillStore` Name uniqueness (database-backed stores)](#skillstore-name-uniqueness-database-backed-stores)).
   Steps 2-3 merge **across** scope levels and **across** distinct partitions at the same level; they do not legitimize duplicate rows inside one partition.
   If same-partition duplicates appear (e.g. legacy import or corruption), the implementation MUST NOT silently pick one during list/get merge; it MUST surface a data error, reject the operation, or require remediation per operator policy.

The merged list is the authoritative **effective** set for inference and for any API that exposes "the skills for this user" unless a separate **raw** inventory contract is explicitly documented.

### Resolution by Logical Name

- Spec ID: `CYNAI.SKILLS.ResolutionByLogicalName` <a id="spec-cynai-skills-resolutionbylogicalname"></a>

To return **full content** for a logical skill **`name`** for a subject user (including MCP `skills.get` and the default web/CLI get path), the implementation MUST:

- Build the candidate pool per step 1 in [Effective skill set (merge)](#effective-skill-set-merge) (including **group** membership and optional **`project_id`** when supplied; see [Resolution context (`project_id`)](#resolution-context-project_id)).
- Apply the same **merge-by-`name`** rules as in steps 2-4 in [Effective skill set (merge)](#effective-skill-set-merge) (cross-level precedence, then same-level tie-break by **`updated_at`**, then **`id`**; same-partition duplicate names remain invalid).
- Return **full content and metadata** for the **single winning** row.

Call parameters are at least **`user_id`** and **`name`**; APIs that support it (e.g. MCP `skills.get`, `skills.list`) MAY take optional **`project_id`** as in [Resolution context (`project_id`)](#resolution-context-project_id).

If no candidate row exists for that name, return not-found.

### Resolution Context (`project_id`)

- Spec ID: `CYNAI.SKILLS.ResolutionContextProjectId` <a id="spec-cynai-skills-resolutioncontextprojectid"></a>

When **`project_id`** is **omitted** on a call that supports it, the candidate pool contains **no** **project**-scoped rows (see step 1 in [Effective skill set (merge)](#effective-skill-set-merge)).

When **`project_id`** is **supplied**, the candidate pool MUST include **project**-scoped rows **only** for that project's partition, and the implementation MUST verify the user is entitled to that project; entitlement failure MUST fail the call (e.g. forbidden or not-found per policy).

**Group**-scoped resolution does **not** use **`project_id`**: group skills are included by resolving the user's **group memberships**, then selecting group-scoped rows with the matching **`name`** for those groups, as in step 1.

Direct fetch by **stable identifier** (bypassing merge) is **not** the default product path; it MAY exist only on documented non-MCP surfaces when the caller must load a **specific stored** row.

**Name precedence** in this section means the ordering implied by the merge rules above (for display and conflict resolution).

## Skills MCP Tools (Reference)

Spec ID **`CYNAI.SKILLS.SkillToolsMcp`** is anchored in [Skills MCP Tools](mcp_tools/skills_tools.md#spec-cynai-skills-skilltoolsmcp).
The **`skills.*`** MCP tool **contracts** (arguments, gateway preconditions, outcomes, **ADMIN** restrictions on tool calls) are canonical there, together with [MCP Tooling](mcp/mcp_tooling.md) and [MCP Gateway Enforcement](mcp/mcp_gateway_enforcement.md).

Implementations MUST apply the **backend** rules in this document ([Skill Store](#skill-store), [Effective skill set (merge)](#effective-skill-set-merge), [Resolution by logical name](#resolution-by-logical-name), [Skill Auditing](#skill-auditing-malicious-pattern-scanning)) when executing those tools.

### Related Specs

- [Skills MCP Tools](mcp_tools/skills_tools.md#spec-cynai-skills-skilltoolsmcp) (tool catalog and contracts; spec anchor)
- [MCP tool specifications](mcp_tools/README.md)
- Skill auditing: [Skill Auditing (Malicious Pattern Scanning)](#skill-auditing-malicious-pattern-scanning)

### Skills MCP Tools (Reference) Requirements Traces

- [REQ-SKILLS-0114](../requirements/skills.md#req-skills-0114)

## Skill Auditing (Malicious Pattern Scanning)

- Spec ID: `CYNAI.SKILLS.SkillAuditing` <a id="spec-cynai-skills-skillauditing"></a>

### Skill Auditing (Malicious Pattern Scanning) Requirements Traces

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
  That service defines deterministic injection-stripping rules: denylists of instruction-like patterns (e.g. "ignore previous instructions", "system prompt", "developer message", "jailbreak"), removal of HTML comments and hidden content, and configurable regex patterns and section markers (see [Deterministic Sanitization](secure_browser_service.md#spec-cynai-browsr-deterministicsanitization) and [Configuration and Rules](secure_browser_service.md#spec-cynai-browsr-preferencesrules)).
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

### Skill Retrieval Requirements Traces

- [REQ-SKILLS-0102](../requirements/skills.md#req-skills-0102)

Retrieval returns full skill content from the [Skill Store](#skill-store) for callers that need the bytes (or text) of a skill.

**By logical name** (subject user + **`name`**, optional **`project_id`** where the API supports it): the implementation MUST use [Resolution by logical name](#resolution-by-logical-name) and [Resolution context (`project_id`)](#resolution-context-project_id), including MCP `skills.get` per [Skills MCP Tools](mcp_tools/skills_tools.md).

**By stable identifier**: load the stored row for that id when the caller is entitled (e.g. User API or internal services); MCP **`skills.get`** does not take `skill_id` (see [Skills MCP Tools](mcp_tools/skills_tools.md)).
Tool-calling and gateway rules are not specified here; see [MCP Tooling](mcp/mcp_tooling.md), [MCP Gateway Enforcement](mcp/mcp_gateway_enforcement.md), and [Skills MCP Tools](mcp_tools/skills_tools.md).

### `SkillRetrieval` Inputs

- **Logical `name`** with effective-resolution semantics, plus subject context (see [Resolution by logical name](#resolution-by-logical-name)).
- **Stable identifier** from registry or store for direct row reads when the API contract allows it.
- Optional: cancellation context for long-running or remote storage.

### `SkillRetrieval` Outputs

- Skill content (e.g. markdown or opaque blob) and optional content type or checksum.

### `SkillRetrieval` Behavior

- Given a valid stable identifier, the system returns the stored skill content for that row.
- Given a logical **name** with effective-resolution semantics, the system returns content for the **single winning** row after the same merge as list (see [Effective skill set (merge)](#effective-skill-set-merge)); if no row matches, the operation fails with a deterministic error (e.g. not found).
- If the identifier is unknown or the skill was removed, identifier-based retrieval fails with a deterministic error (e.g. not found).

### `SkillRetrieval` Error Conditions

- Unknown or invalid identifier: not found error.
- Unknown or unmatched **name** under effective-resolution semantics: not found error.
- Store unavailable: transient error so callers can retry.

## Inference Exposure

- Spec ID: `CYNAI.SKILLS.InferenceExposure` <a id="spec-cynai-skills-inferenceexposure"></a>

### Inference Exposure Requirements Traces

- [REQ-SKILLS-0103](../requirements/skills.md#req-skills-0103)

Inference exposure defines how inference models that support skills receive skill content when handling a request.

### `InferenceExposure` Scope

- The system MUST support at least one of: (1) resolving a set of skill identifiers to content and including that content in the inference context (e.g. system or user message), or (2) providing a stable reference (e.g. URL or path) that the inference runtime can resolve to skill content.
- The choice of mechanism depends on the inference backend; the spec does not mandate a single protocol but requires that some path exists for models that support skills to receive skill content.

### `InferenceExposure` Preconditions

- Skills offered for a request MUST be filtered by scope (and any policy) so that only allowed skills are exposed.
- Only skills present in the registry and store MUST be resolvable.

### `InferenceExposure` Outcomes

- For a given inference request that requests skills (by identifier, **name** with effective resolution, or scope), the system supplies the corresponding skill content or resolvable reference so the model can use the skills during inference.
  **Project**-scoped skills follow the same candidate-pool rules as list/get: they appear only when **project** context is available (e.g. `project_id` on the request or equivalent), per [Resolution context (`project_id`)](#resolution-context-project_id).
