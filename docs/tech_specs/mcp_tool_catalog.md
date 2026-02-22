# MCP Tool Catalog

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Naming and Conventions](#naming-and-conventions)
  - [Naming and Conventions Applicable Requirements](#naming-and-conventions-applicable-requirements)
- [Common Argument Requirements](#common-argument-requirements)
- [Tool Catalog](#tool-catalog)
  - [Artifact Tools](#artifact-tools)
  - [Sandbox Tools](#sandbox-tools)
  - [Sandbox Allowed Images (PM Agent)](#sandbox-allowed-images-pm-agent)
  - [Web Fetch](#web-fetch)
  - [Secure Web Search](#secure-web-search)
  - [API Egress](#api-egress)
  - [Git Egress](#git-egress)
  - [Node Tools](#node-tools)
  - [Model Registry](#model-registry)
  - [Skills Tools](#skills-tools)
  - [Help Tools](#help-tools)
  - [Database Tools](#database-tools)
- [Response and Error Model](#response-and-error-model)
  - [Response and Error Model Applicable Requirements](#response-and-error-model-applicable-requirements)

## Document Overview

This document defines the canonical MCP tool names and argument schemas for the MVP.
Tool schemas are authoritative in the MCP server implementation.
The orchestrator gateway enforces allowlists, access control, and auditing without extending MCP wire messages.

Related documents

- MCP gateway enforcement: [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)
- MCP concepts: [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md)
- Git egress tool patterns: [`docs/tech_specs/git_egress_mcp.md`](git_egress_mcp.md)
- Node and sandbox behaviors: [`docs/tech_specs/worker_node.md`](worker_node.md)

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

- Spec ID: `CYNAI.MCPTOO.SandboxTools` <a id="spec-cynai-mcptoo-sandboxtools"></a>

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

### Sandbox Allowed Images (PM Agent)

- Spec ID: `CYNAI.MCPTOO.SandboxAllowedImagesPmAgent` <a id="spec-cynai-mcptoo-sandboxallowedimagespmagent"></a>

These tools are available only to the Project Manager agent.
Adding an image to the allowed list is gated by the orchestrator system setting `agents.project_manager.sandbox.allow_add_to_allowed_images` (default `false`).
See [`docs/tech_specs/sandbox_image_registry.md`](sandbox_image_registry.md#spec-cynai-sandbx-pmagentaddtoallowedimages).

- `sandbox.allowed_images.list`
  - required args: none
  - optional args: `limit`, `cursor` (for pagination)
  - Returns the current allowed sandbox image references (or a paginated subset).
  - Gateway: allow for PM agent only.
- `sandbox.allowed_images.add`
  - required args: `image_ref` (string; OCI image reference, e.g. `docker.io/library/python:3.12` or `quay.io/org/image:tag`)
  - optional args: `name` (string; logical name for the image), `task_id` (uuid; for audit context)
  - Adds the image to the allowed list and records it in the sandbox image registry so it may be used for sandbox jobs.
  - Gateway: allow for PM agent only when `agents.project_manager.sandbox.allow_add_to_allowed_images` is `true`; otherwise MUST reject.

### Web Fetch

`web.fetch` is a policy-controlled, sanitized fetch implemented by the Secure Browser Service.

- `web.fetch`
  - required args: `task_id`, `url`

### Secure Web Search

- Spec ID: `CYNAI.MCPTOO.SecureWebSearch` <a id="spec-cynai-mcptoo-securewebsearch"></a>

Secure web search is a policy-controlled MCP tool that allows agents to run search queries without direct, unfiltered access to the open internet.
Results are returned through a secure path (e.g. Secure Browser Service or a dedicated search proxy) so that only sanitized or allowlisted search provider responses are exposed to the agent.

- `web.search`
  - required args: `task_id`, `query` (string; search query text)
  - optional args: `limit` (int; max number of results to return), `safe_filter` (boolean; when supported, request safe-search filtering)
  - Returns search results (titles, snippets, URLs) in a size-limited, policy-compliant format.
  - Implementation MUST NOT expose raw internet access; search MUST be routed through the same secure, policy-controlled mechanism as `web.fetch` (e.g. Secure Browser Service with search-specific rules) or a dedicated secure search endpoint.
  - Subject to the same access control and audit as `web.fetch`; action MAY be `web.search` in access control rules.

Traces To:

- [REQ-MCPTOO-0119](../requirements/mcptoo.md#req-mcptoo-0119)

See [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md) (or equivalent secure fetch/search integration).

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

- Spec ID: `CYNAI.MCPTOO.SkillsTools` <a id="spec-cynai-mcptoo-skillstools"></a>

Canonical tool names, argument schemas, behavior, and controls for skills tools are defined in the skills spec only.
This catalog lists tool names for allowlist and discovery; do not duplicate argument or behavior details here.

- **Skills (full CRUD)**: [Skill Tools via MCP (CRUD)](skills_storage_and_inference.md#spec-cynai-skills-skilltoolsmcp)
  - `skills.create`
  - `skills.list`
  - `skills.get`
  - `skills.update`
  - `skills.delete`

### Help Tools

- Spec ID: `CYNAI.MCPTOO.HelpTools` <a id="spec-cynai-mcptoo-helptools"></a>

On-demand documentation for how to interact with CyNodeAI.
See [Help MCP Server](mcp_tooling.md#spec-cynai-mcptoo-helpmcpserver).

- `help.get`
  - required args: `task_id` (for context and auditing)
  - optional args: `topic` (string; e.g. tool name or doc path) or `path` (string; logical path into help content).
  - Returns documentation content (e.g. markdown or plain text) for the requested topic or a default/overview when omitted.
  - Response MUST be size-limited; content MUST NOT include secrets.

### Database Tools

- Spec ID: `CYNAI.MCPTOO.DatabaseTools` <a id="spec-cynai-mcptoo-databasetools"></a>

Database tools are typed operations only.
Raw SQL MUST NOT be exposed via MCP tools.
Implementations MAY use raw SQL internally (for example pgvector similarity queries), but they MUST NOT accept arbitrary SQL from callers.

CRUD expectations

For each database-backed resource exposed via MCP tools, the default expectation is full CRUD support:

- create
- list
- get
- update
- delete

If full CRUD is not appropriate for a resource or for MVP scope, the tool catalog MUST document an intentional exception and the allowed operations.

Minimum typed tools for MVP:

- `db.task.get`
  - required args: `task_id`
- `db.task.update_status`
  - required args: `task_id`, `status`
- `db.job.get`
  - required args: `job_id`
- `db.job.update_status`
  - required args: `job_id`, `status`

Intentional exceptions (MVP)

- Tasks and jobs are not full-CRUD via MCP tools in MVP.
  The minimum MCP surface is read plus narrowly-scoped updates required by orchestrator-side agents.
  Full task CRUD is exposed to user clients via the User API Gateway.
- `db.system_setting.get`
  - required args: `key`
- `db.system_setting.list`
  - optional args: `key_prefix`, `limit`, `cursor`
  - notes: list responses MUST be size-limited and support pagination
- `db.system_setting.create`
  - required args: `key`, `value`, `value_type`
  - optional args: `reason`
  - notes: create MUST fail if the key already exists
- `db.system_setting.update`
  - required args: `key`, `value`, `value_type`
  - optional args: `expected_version`, `reason`
  - notes: when `expected_version` is provided and does not match, update MUST fail with a conflict error
- `db.system_setting.delete`
  - required args: `key`
  - optional args: `expected_version`, `reason`
- `db.preference.get`
  - required args: `scope_type`, `key`
  - optional args: `scope_id`
  - notes: `scope_id` is required when `scope_type` is not `system`
- `db.preference.list`
  - required args: `scope_type`
  - optional args: `scope_id`, `key_prefix`, `limit`, `cursor`
  - notes: list responses MUST be size-limited and support pagination
- `db.preference.create`
  - required args: `scope_type`, `key`, `value`, `value_type`
  - optional args: `scope_id`, `reason`
  - notes: create MUST fail if the scoped key already exists
- `db.preference.update`
  - required args: `scope_type`, `key`, `value`, `value_type`
  - optional args: `scope_id`, `expected_version`, `reason`
  - notes: when `expected_version` is provided and does not match, update MUST fail with a conflict error
- `db.preference.delete`
  - required args: `scope_type`, `key`
  - optional args: `scope_id`, `expected_version`, `reason`
- `db.preference.effective`
  - required args: `task_id`
  - optional args: `include_sources` (boolean)
- **Project tools (user-scoped)**  
  All project tools MUST return only projects the authenticated user is authorized to access (default project plus RBAC-scoped projects).
  See [Project Search via MCP](projects_and_scopes.md#spec-cynai-access-projectsmcpsearch).
- `db.project.get`
  - required args: `project_id` (uuid) or `slug` (text); exactly one MUST be provided
  - notes: returns the project if it is in the user's authorized set; otherwise not-found or access-denied
- `db.project.list`
  - optional args: `q` (text; filter on slug, display_name, or description), `limit`, `cursor`
  - notes: list responses MUST be size-limited and support pagination; only authorized projects are returned

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
