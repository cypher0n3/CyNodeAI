# Skills MCP Tools

- [Document Overview](#document-overview)
- [Normative Split (MCP vs Backend)](#normative-split-mcp-vs-backend)
- [Definition Compliance](#definition-compliance)
- [Tool Names and Allowlist](#tool-names-and-allowlist)
  - [Traces To](#traces-to)
- [Tool Contracts](#tool-contracts)
  - [Preconditions (Gateway)](#preconditions-gateway)
  - [Tool: `skills.create`](#tool-skillscreate)
  - [Tool: `skills.list`](#tool-skillslist)
  - [Tool: `skills.get`](#tool-skillsget)
  - [Tool: `skills.update`](#tool-skillsupdate)
  - [Tool: `skills.delete`](#tool-skillsdelete)
  - [Tool Outcomes](#tool-outcomes)
- [Invocation and Backend Handoff](#invocation-and-backend-handoff)
  - [Gateway Flow](#gateway-flow)
  - [Related Specs](#related-specs)

## Document Overview

This document is the **canonical MCP catalog** for built-in **`skills.*`** tools: names, argument shapes, gateway-facing behavior, and outcomes.

How MCP tool calling, tokens, allowlists, gateway enforcement, and auditing work is defined in [MCP Tooling](../mcp/mcp_tooling.md), [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md), [MCP tool call auditing](../mcp/mcp_tool_call_auditing.md), and [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md)-not duplicated here.

**Backend** semantics the orchestrator MUST apply when implementing these tools (durable store, registry, **effective skill set** merge, partition **name** uniqueness, content scanning, inference exposure) are in [Skills Storage and Inference Exposure](../skills_storage_and_inference.md).

Related documents

- [Skills Storage and Inference Exposure](../skills_storage_and_inference.md)
- [MCP tool specifications](README.md)

## Normative Split (MCP vs Backend)

- **Concern:** Wire protocol, tool names, gateway flow, allowlists, **ADMIN** restrictions on tool calls
  - authoritative doc: [MCP Tooling](../mcp/mcp_tooling.md), [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md), this file
- **Concern:** Database schema, store/registry, merge-by-name algorithm, auditing **skill content** (patterns), User API vs MCP
  - authoritative doc: [Skills Storage and Inference Exposure](../skills_storage_and_inference.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

**ADMIN:** Skill tools MUST NOT be invocable with **ADMIN**-level principals; the gateway MUST reject such invocations per [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md).

## Tool Names and Allowlist

- Spec ID: `CYNAI.MCPTOO.SkillsTools` <a id="spec-cynai-mcptoo-skillstools"></a>

Skills (full CRUD) tool names:

- `skills.create`
- `skills.list`
- `skills.get`
- `skills.update`
- `skills.delete`

Allowlist and per-tool scope: [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md).

### Traces To

- [REQ-MCPTOO-0106](../../requirements/mcptoo.md#req-mcptoo-0106)
- [REQ-SKILLS-0001](../../requirements/skills.md#req-skills-0001)

## Tool Contracts

- Spec ID: `CYNAI.SKILLS.SkillToolsMcp` <a id="spec-cynai-skills-skilltoolsmcp"></a>

### Preconditions (Gateway)

- Caller MUST be allowed to invoke each tool under the gateway allowlist and access control rules for the subject user (`user_id`).
  Skill **visibility and mutability** follow each record's `user` / `group` / `project` / `global` scope and [Skill Scope and Default](../skills_storage_and_inference.md#spec-cynai-skills-skillscopeelevation).
  The `user_id` argument does **not** mean every skill is user-scoped.
- **ADMIN**-level principals or roles MUST NOT use these tools; the gateway MUST reject ADMIN-level invocation paths for skill tools.
- Write operations require authorization; list/get only return skills visible to the caller per scope and access policy.
- All invocations MUST be audited per [MCP tool call auditing](../mcp/mcp_tool_call_auditing.md).

### Tool: `skills.create`

- Required args: `user_id` (uuid string; subject user for authorization and default ownership when `scope` is `user` or omitted), **`name`** (non-empty string), `content` (markdown string).
- Optional args: `scope` (string: `user` | `group` | `project` | `global`; default `user`; broader scope requires caller permission per [Skill Scope and Default](../skills_storage_and_inference.md#spec-cynai-skills-skillscopeelevation)).
- Skills are **not** task resources; do **not** pass `task_id`.
  Each stored skill has its own **scope** (user, group, project, or global), independent of the MCP `user_id` field name-see [SkillRegistry Scope](../skills_storage_and_inference.md#spec-cynai-skills-skillregistryscope).
  The gateway validates `user_id` and that the caller may act for that user; content MUST pass skill auditing before store; on audit failure return rejection reason, match category, and exact triggering text.
- Database-backed storage MUST reject creates that would violate **name uniqueness within the scope partition** (see [SkillStore name uniqueness](../skills_storage_and_inference.md#spec-cynai-skills-skillstorenameuniqueness)).

### Tool: `skills.list`

- Required args: `user_id` (uuid string; subject user for the call).
- Optional args: `scope` (filter), `owner` (filter)-when present, narrow which stored rows are considered **before** building the effective set (same semantics as narrowing the candidate pool); **`project_id`** (uuid string)-when **omitted**, **project**-scoped skills are **not** included in the effective set; when **present**, restrict **project**-scoped candidates to that project (subject must be entitled); see [Resolution context (`project_id`)](../skills_storage_and_inference.md#spec-cynai-skills-resolutioncontextprojectid).
- Returns the **effective** skill set for the current context: metadata only (identifier, name, scope, owner, `updated_at` as applicable); not full content.
  The response MUST be the merged view defined in [Effective skill set (merge)](../skills_storage_and_inference.md#spec-cynai-skills-effectiveskillsetmerge), not a raw concatenation of every stored row.

### Tool: `skills.get`

- Spec ID: `CYNAI.MCPTOO.SkillsGet` <a id="spec-cynai-mcptoo-skillsget"></a>

- Required args: `user_id` (subject user for the call), **`name`** (non-empty string). **`skill_id`** is **not** a supported argument on this tool (agents resolve by **name** only; see [Resolution by logical name](../skills_storage_and_inference.md#spec-cynai-skills-resolutionbylogicalname)).
- Optional arg: **`project_id`** (uuid string)-when **omitted**, **project**-scoped skills are **not** candidates for resolution; when **present**, resolve in **project** context (subject must be entitled to the project); see [Resolution context (`project_id`)](../skills_storage_and_inference.md#spec-cynai-skills-resolutioncontextprojectid).
- Returns full content and metadata for the **effective** winning row for that `name`; caller MUST be entitled to see it (same visibility as list).

### Tool: `skills.update`

- Required args: `user_id`, `skill_id`.
- Optional args: `content` (markdown string; if provided, re-audited), `name` (string), `scope` (string; elevation requires permission).
- Caller MUST be authorized to modify the skill under non-admin gateway rules for the subject user.
  On content update, audit failure returns same feedback as create.

### Tool: `skills.delete`

- Required args: `user_id`, `skill_id`.
- Caller MUST be authorized to delete under non-admin gateway rules for the subject user; skill is removed from store and registry.
  Authorization is evaluated against the skill's scope and ownership, not only "user-scoped" rows.

### Tool Outcomes

- Create: skill stored and registered; response includes stable skill identifier; on audit failure, error with feedback.
- List: **effective** list of skill metadata (one row per `name` after merge per [Effective skill set (merge)](../skills_storage_and_inference.md#spec-cynai-skills-effectiveskillsetmerge)).
- Get: full content and metadata or not-found if not entitled or missing; not-found if no effective row exists for the requested **`name`** (see [Resolution by logical name](../skills_storage_and_inference.md#spec-cynai-skills-resolutionbylogicalname)).
- Update: store and registry updated, or error with audit feedback on content rejection.
- Delete: skill removed; subsequent list/get MUST NOT return it.

The **`skills.list`** response includes **`skill_id`** in metadata for `skills.update` and `skills.delete` so the agent can target the row the effective list surfaced for each name.

## Invocation and Backend Handoff

Each invocation is a single direct MCP tool call on `Server: default`.

### Gateway Flow

1. Resolve caller; check allowlist and scope per [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md).
2. Drop [extraneous arguments](../mcp/mcp_tooling.md#spec-cynai-mcptoo-extraneousarguments); validate args per this document.
3. Invoke orchestrator skill store/registry implementation; backend enforces RBAC, [effective merge](../skills_storage_and_inference.md#spec-cynai-skills-effectiveskillsetmerge), and [SkillStore](../skills_storage_and_inference.md#spec-cynai-skills-skillstore) rules.
4. Enforce response size limit; audit and return.

Tool definition **Help** text (and embedded help) MUST state that **`project_id`** is required for **project**-scoped skills to appear in list/get results, per [Effective skill set (merge)](../skills_storage_and_inference.md#spec-cynai-skills-effectiveskillsetmerge).

### Related Specs

- [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md)
- [Skills Storage and Inference Exposure](../skills_storage_and_inference.md) (persistence and domain semantics)
- [Skill Auditing (Malicious Pattern Scanning)](../skills_storage_and_inference.md#spec-cynai-skills-skillauditing)
