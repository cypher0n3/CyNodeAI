# Skills MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Names and Allowlist](#tool-names-and-allowlist)
  - [Traces To](#traces-to)
- [Invocation and Algorithm Reference](#invocation-and-algorithm-reference)

## Document Overview

Canonical tool names, argument schemas, behavior, and controls for skills tools are defined in the skills spec only.
This document lists tool names for allowlist and discovery and points to the single source of truth; it does not duplicate argument or behavior details.

Related documents

- [Skills Storage and Inference - Skill Tools via MCP (CRUD)](../skills_storage_and_inference.md#spec-cynai-skills-skilltoolsmcp)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Tool Names and Allowlist

- Spec ID: `CYNAI.MCPTOO.SkillsTools` <a id="spec-cynai-mcptoo-skillstools"></a>

Skills (full CRUD) tool names:

- `skills.create`
- `skills.list`
- `skills.get`
- `skills.update`
- `skills.delete`

Allowlist and scope are defined in [Skills Storage and Inference](../skills_storage_and_inference.md) and [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md).

### Traces To

- [REQ-MCPTOO-0106](../../requirements/mcptoo.md#req-mcptoo-0106)
- [REQ-SKILLS-0001](../../requirements/skills.md#req-skills-0001)

## Invocation and Algorithm Reference

Each invocation (`skills.create`, `skills.list`, `skills.get`, `skills.update`, `skills.delete`) is a single direct MCP tool call on `Server: default`.
Gateway flow for each: (1) Resolve caller; check allowlist and scope per [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md). (2) Validate args per schema in the skills spec. (3) Call skill store/registry backend (create, list, get, update, delete); backend enforces RBAC and scope. (4) Enforce response size limit; audit and return.
Full argument schemas, preconditions, outcomes, and algorithm-level behavior for each tool MUST be taken from [Skill Tools via MCP (CRUD)](../skills_storage_and_inference.md#spec-cynai-skills-skilltoolsmcp).
Implementations and gateway allowlists MUST NOT rely on this catalog for argument or behavior details; use the skills spec only.
