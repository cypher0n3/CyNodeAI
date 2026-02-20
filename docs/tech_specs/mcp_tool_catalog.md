# MCP Tool Catalog

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Naming and Conventions](#naming-and-conventions)
- [Common Argument Requirements](#common-argument-requirements)
- [Tool Catalog](#tool-catalog)
  - [Artifact Tools](#artifact-tools)
  - [Sandbox Tools](#sandbox-tools)
  - [Web Fetch](#web-fetch)
  - [API Egress](#api-egress)
  - [Git Egress](#git-egress)
  - [Node Tools](#node-tools)
  - [Model Registry](#model-registry)
  - [Skills Tools](#skills-tools)
  - [Help Tools](#help-tools)
  - [Database Tools](#database-tools)
- [Response and Error Model](#response-and-error-model)

## Document Overview

This document defines the canonical MCP tool names and argument schemas for the MVP.
Tool schemas are authoritative in the MCP server implementation.
The orchestrator gateway enforces allowlists, access control, and auditing without extending MCP wire messages.

Related documents

- MCP gateway enforcement: [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)
- MCP concepts: [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md)
- Git egress tool patterns: [`docs/tech_specs/git_egress_mcp.md`](git_egress_mcp.md)
- Node and sandbox behaviors: [`docs/tech_specs/node.md`](node.md)

## Goals

- Publish a stable MVP tool surface with explicit names.
- Require explicit task scoping via tool arguments for task-scoped tools.
- Enable deterministic policy enforcement and auditing at the gateway.

## Naming and Conventions

The following requirements apply.

### Naming and Conventions Applicable Requirements

- Spec ID: `CYNAI.MCPTOO.ToolCatalogNaming` <a id="spec-cynai-mcptoo-toolnaming"></a>

Traces To:

- [REQ-MCPTOO-0106](../requirements/mcptoo.md#req-mcptoo-0106)
- [REQ-MCPTOO-0107](../requirements/mcptoo.md#req-mcptoo-0107)
- [REQ-MCPTOO-0108](../requirements/mcptoo.md#req-mcptoo-0108)

## Common Argument Requirements

Task scoping

- Any task-scoped tool MUST accept `task_id` (uuid) as an argument.
- Any run-scoped tool SHOULD accept `run_id` (uuid) as an argument.
- Any job-scoped tool SHOULD accept `job_id` (uuid) as an argument.

Size limits

- Tools that accept user-provided text MUST enforce size limits.
- Tools MUST reject requests that exceed configured limits.

## Tool Catalog

This section lists the canonical tool names and required arguments.
Optional arguments MAY be added later as optional fields.

### Artifact Tools

- `artifact.put`
  - required args: `task_id`, `path`, `content_bytes_base64`
- `artifact.get`
  - required args: `task_id`, `path`
- `artifact.list`
  - required args: `task_id`

### Sandbox Tools

- `sandbox.create`
  - required args: `task_id`, `job_id`, `image_ref`
- `sandbox.exec`
  - required args: `task_id`, `job_id`, `command`, `argv`
- `sandbox.put_file`
  - required args: `task_id`, `job_id`, `path`, `content_bytes_base64`
- `sandbox.get_file`
  - required args: `task_id`, `job_id`, `path`
- `sandbox.stream_logs`
  - required args: `task_id`, `job_id`
- `sandbox.destroy`
  - required args: `task_id`, `job_id`

### Web Fetch

`web.fetch` is a policy-controlled, sanitized fetch implemented by the Secure Browser Service.

- `web.fetch`
  - required args: `task_id`, `url`

### API Egress

`api.call` is routed through the API Egress Server.

- `api.call`
  - required args: `task_id`, `provider`, `operation`, `params`

### Git Egress

Canonical tool names are defined by the Git Egress MCP spec.
This catalog uses the same tool names.

- `git.repo.validate`
  - required args: `task_id`, `provider`, `repo`
- `git.changeset.apply`
  - required args: `task_id`, `provider`, `repo`, `params`
- `git.commit.create`
  - required args: `task_id`, `provider`, `repo`, `params`
- `git.branch.create`
  - required args: `task_id`, `provider`, `repo`, `params`
- `git.push`
  - required args: `task_id`, `provider`, `repo`, `params`
- `git.pr.create`
  - required args: `task_id`, `provider`, `repo`, `params`

### Node Tools

- `node.list`
  - required args: none
- `node.get`
  - required args: `node_slug`
- `node.refresh_config`
  - required args: `node_slug`

### Model Registry

- `model.list`
  - required args: none
- `model.get`
  - required args: `model_id`

### Skills Tools

Canonical tool names, argument schemas, behavior, and controls for skills tools are defined in the skills spec only.
This catalog lists tool names for allowlist and discovery; do not duplicate argument or behavior details here.

- **Skills (full CRUD)**: [Skill Tools via MCP (CRUD)](skills_storage_and_inference.md#skill-tools-via-mcp-crud)
  - `skills.create`
  - `skills.list`
  - `skills.get`
  - `skills.update`
  - `skills.delete`

### Help Tools

On-demand documentation for how to interact with CyNodeAI.
See [Help MCP Server](mcp_tooling.md#help-mcp-server).

- `help.get`
  - required args: `task_id` (for context and auditing)
  - optional args: `topic` (string; e.g. tool name or doc path) or `path` (string; logical path into help content).
  - Returns documentation content (e.g. markdown or plain text) for the requested topic or a default/overview when omitted.
  - Response MUST be size-limited; content MUST NOT include secrets.

### Database Tools

Database tools are typed operations only.
Raw SQL MUST NOT be exposed via MCP tools.
Implementations MAY use raw SQL internally (for example pgvector similarity queries), but they MUST NOT accept arbitrary SQL from callers.

Minimum typed tools for MVP:

- `db.task.get`
  - required args: `task_id`
- `db.task.update_status`
  - required args: `task_id`, `status`
- `db.job.get`
  - required args: `job_id`
- `db.job.update_status`
  - required args: `job_id`, `status`
- `db.preference.effective`
  - required args: `task_id`

## Response and Error Model

The following requirements apply.

### Response and Error Model Applicable Requirements

- Spec ID: `CYNAI.MCPTOO.ToolCatalogResponseError` <a id="spec-cynai-mcptoo-toolresponse"></a>

Traces To:

- [REQ-MCPTOO-0109](../requirements/mcptoo.md#req-mcptoo-0109)
- [REQ-MCPTOO-0110](../requirements/mcptoo.md#req-mcptoo-0110)

Recommended fields

- `status`: success or error
- `result`: object on success
- `error`: object on error
  - `type`, `message`, `details` (optional)
