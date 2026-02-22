# Web Console

- [Document Overview](#document-overview)
- [Capability Parity With CLI](#capability-parity-with-cli)
- [Primary Use Cases](#primary-use-cases)
- [Security Model](#security-model)
  - [Security Model Applicable Requirements](#security-model-applicable-requirements)
- [Authentication and Authorization](#authentication-and-authorization)
- [Credential Management](#credential-management)
  - [Credential Management Applicable Requirements](#credential-management-applicable-requirements)
- [Preferences Management](#preferences-management)
  - [Preferences Management Applicable Requirements](#preferences-management-applicable-requirements)
- [System Settings Management](#system-settings-management)
- [Skills Management](#skills-management)
- [Project Management](#project-management)
- [Node Management](#node-management)
  - [Node Management Applicable Requirements](#node-management-applicable-requirements)
- [Implementation Specification (Nuxt)](#implementation-specification-nuxt)
  - [Implementation Specification Applicable Requirements](#implementation-specification-applicable-requirements)
  - [Technology Choices](#technology-choices)
  - [Application Structure](#application-structure)
  - [Authentication Model](#authentication-model)
  - [Authentication Model Applicable Requirements](#authentication-model-applicable-requirements)
  - [Gateway API Client](#gateway-api-client)
  - [UI Requirements by Domain](#ui-requirements-by-domain)
  - [Deployment Options](#deployment-options)
  - [Deployment Options Applicable Requirements](#deployment-options-applicable-requirements)
- [API Surface](#api-surface)
- [API Documentation (Swagger UI)](#api-documentation-swagger-ui)
- [Audit and Change History](#audit-and-change-history)
- [MVP Scope](#mvp-scope)

## Document Overview

This document defines an admin-focused web interface for CyNodeAI.
The web console is intended for credential upload and management, for user preferences management, systems settings management, log review, and a variety of other admin tasks.

## Capability Parity With CLI

- Spec ID: `CYNAI.CLIENT.WebConsoleCapabilityParity` <a id="spec-cynai-client-webconcapabilityparity"></a>

Traces To:

- [REQ-CLIENT-0004](../requirements/client.md#req-client-0004)

The Web Console and the CLI management app MUST offer the same administrative capabilities.
When adding or changing a capability in this spec (for example a new credential workflow, preference scope, node action, or skill operation), the [cynork CLI](cynork_cli.md) spec and implementation MUST be updated to match, and vice versa.
Use the same gateway APIs and the same authorization and auditing rules for both clients.

## Primary Use Cases

- Upload and manage external provider credentials for API Egress and Git Egress.
- View, edit, and audit user preferences across scopes (system, user, group, project, task), with an easy method to CRUD personal, group, and project preferences.
- View node inventory and manage basic node lifecycle controls.
- Full CRUD for AI skills: create (upload), list, view (content and metadata), edit (update content and/or metadata including scope, with same auditing and scope permissions), and delete; see [Skill Management CRUD](skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud).
- Basic project management (CRUD): create, list, view, edit, and delete or disable projects; each project has a user-friendly title and optional text description (see [Projects and Scope Model](projects_and_scopes.md), [REQ-PROJCT-0103](../requirements/projct.md#req-projct-0103)).
- Create tasks: submit a task as plain text or Markdown (inline or paste), attach files or other artifacts (file upload), run a script (e.g. script file upload), or run a short series of commands; same semantics as the CLI and User API Gateway (see [REQ-ORCHES-0126](../requirements/orches.md#req-orches-0126), [REQ-ORCHES-0127](../requirements/orches.md#req-orches-0127), [REQ-ORCHES-0128](../requirements/orches.md#req-orches-0128), [REQ-CLIENT-0152](../requirements/client.md#req-client-0152), [REQ-CLIENT-0154](../requirements/client.md#req-client-0154)).
- Inspect access control rules and audit decisions.

## Security Model

The following requirements apply.

### Security Model Applicable Requirements

- Spec ID: `CYNAI.WEBCON.Security` <a id="spec-cynai-webcon-security"></a>

Traces To:

- [REQ-WEBCON-0001](../requirements/webcon.md#req-webcon-0001)
- [REQ-WEBCON-0100](../requirements/webcon.md#req-webcon-0100)
- [REQ-WEBCON-0101](../requirements/webcon.md#req-webcon-0101)
- [REQ-WEBCON-0102](../requirements/webcon.md#req-webcon-0102)
- [REQ-WEBCON-0103](../requirements/webcon.md#req-webcon-0103)
- [REQ-WEBCON-0104](../requirements/webcon.md#req-webcon-0104)

Threat model notes

- Treat browsers as untrusted clients.
- Rely on the gateway for authorization, rate limiting, and auditing.
- Prefer short-lived access tokens for UI sessions.

## Authentication and Authorization

The User API Gateway MUST authenticate all web console requests.
The gateway MUST authorize actions using access control and user context.

Recommended approaches

- Local username and password for initial deployments.
- Optional enterprise SSO (OIDC/SAML) as a future extension.

## Credential Management

The web console must support managing credentials that are consumed by controlled egress services.
The gateway endpoint contract for credential operations (list, get, create, rotate, disable) is defined in [API Egress Server - Admin API (Gateway Endpoints)](api_egress_server.md#spec-cynai-apiegr-adminapigatewayendpoints); the console and cynork both use these same endpoints.

Credential types

- API Egress credentials (external model providers, SaaS APIs).
- Git Egress credentials (GitHub/GitLab/Gitea tokens or deploy keys).

### Credential Management Applicable Requirements

- Spec ID: `CYNAI.WEBCON.Credential` <a id="spec-cynai-webcon-credential"></a>

Traces To:

- [REQ-CLIENT-0116](../requirements/client.md#req-client-0116)
- [REQ-CLIENT-0117](../requirements/client.md#req-client-0117)
- [REQ-CLIENT-0118](../requirements/client.md#req-client-0118)
- [REQ-CLIENT-0119](../requirements/client.md#req-client-0119)
- [REQ-CLIENT-0120](../requirements/client.md#req-client-0120)

Recommended metadata fields shown in the UI

- Provider
- Credential name
- Owner scope (user or group)
- Active status
- Created and updated timestamps
- Last used timestamp, when available

## Preferences Management

The web console must support editing preferences stored in PostgreSQL.
It MUST provide an easy method for users to create, read, update, and delete their personal (user), group, and project preferences (full CRUD for those scopes).

### Preferences Management Applicable Requirements

- Spec ID: `CYNAI.WEBCON.Preferences` <a id="spec-cynai-webcon-preferences"></a>

Traces To:

- [REQ-CLIENT-0121](../requirements/client.md#req-client-0121)
- [REQ-CLIENT-0122](../requirements/client.md#req-client-0122)
- [REQ-CLIENT-0123](../requirements/client.md#req-client-0123)
- [REQ-CLIENT-0124](../requirements/client.md#req-client-0124)

Recommended UI behaviors

- Show the precedence model and where a value is coming from.
- Provide a diff view when editing complex JSON values.
- Require a reason field for preference changes when auditing is enabled.

Recommended keys to surface (MVP)

- `output.summary_style` (string)
  - examples: concise, detailed
- `definition_of_done.required_checks` (array)
  - examples: lint, unit_tests, docs_updated
- `language.preferred` (string)
  - examples: en, en-US
- `code.language.rank_ordered` (array)
  - Rank-ordered code language choices with optional context (project kind, task kind).
- `code.language.disallowed` (array)
  - Globally disallowed languages.
- `code.language.disallowed_by_project_kind` (object)
  - Per-project-kind disallowed languages.
- `code.language.disallowed_by_task_kind` (object)
  - Per-task-kind disallowed languages.
- `standards.markdown.line_length` (number)

## System Settings Management

- Spec ID: `CYNAI.WEBCON.SystemSettings` <a id="spec-cynai-webcon-systemsettings"></a>

Traces To:

- [REQ-CLIENT-0160](../requirements/client.md#req-client-0160)

The web console MUST support editing system settings that control orchestrator operational behavior.
System settings are not user preferences; they are operator-controlled orchestrator operational configuration.
For the full distinction, see [User preferences (Terminology)](user_preferences.md#spec-cynai-stands-preferenceterminology).

Recommended keys to surface (MVP)

Key semantics and the PM model selection/warmup algorithm are defined in [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#spec-cynai-orches-projectmanagermodelstartup).

- `agents.project_manager.model.local_default_ollama_model` (string)
- `agents.project_manager.model.selection.execution_mode` (string): `auto` | `force_local` | `force_external`
- `agents.project_manager.model.selection.mode` (string): `auto_sliding_scale` | `fixed_model`
- `agents.project_manager.model.selection.prefer_orchestrator_host` (boolean)

## Skills Management

- Spec ID: `CYNAI.WEBCON.SkillsManagement` <a id="spec-cynai-webcon-skillsmanagement"></a>

Traces To:

- [REQ-WEBCON-0112](../requirements/webcon.md#req-webcon-0112)

The web console MUST support full CRUD for AI skills (create, list, view, update, delete) via the User API Gateway.
All operations use the same controls as defined in [Skill Management CRUD](skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud): authentication, scope visibility, scope elevation permission on write, and auditing on write with rejection feedback (match category and triggering text) when content fails the security scan.

Recommended UI

- **List**: Table or list of skills visible to the user (metadata: name, scope, owner, updated_at); optional filters by scope or owner.
- **View**: Single skill detail with full content and metadata; read-only unless the user has edit permission.
- **Create**: Upload form (paste or file) for markdown content; optional name and scope (scope elevation subject to permission).
- **Edit**: Update content and/or metadata (name, scope); updated content is re-audited; on failure show rejection reason and exact triggering text.
- **Delete**: Confirm and remove skill from store and registry (restricted to owner or admin).

## Project Management

- Spec ID: `CYNAI.WEBCON.ProjectManagement` <a id="spec-cynai-webcon-projectmanagement"></a>

Traces To:

- [REQ-CLIENT-0174](../requirements/client.md#req-client-0174)

The web console MUST support basic project CRUD (create, list, view, update, delete or disable) via the User API Gateway, with the same capabilities as the CLI (see [Project Management](cli_management_app_commands_admin.md#spec-cynai-client-cliprojectmanagement)).
Projects have a user-friendly title (display name) and an optional text description for lists and detail views.

Recommended UI

- **List**: Table or list of projects (columns: title, slug, description excerpt, active, updated_at); optional filter by active status.
- **View**: Single project detail (title, slug, description, is_active, created_at, updated_at, updated_by).
- **Create**: Form with required slug and title; optional description.
- **Edit**: Update title, description, and active status.
- **Delete**: Confirm and delete (or disable) project; show warning when project is referenced by tasks or chat threads.

## Node Management

The web console should support basic node management for operators.
This includes inventory views, health status, and safe administrative controls.

### Node Management Applicable Requirements

- Spec ID: `CYNAI.WEBCON.NodeManagement` <a id="spec-cynai-webcon-nodemanagement"></a>

Traces To:

- [REQ-CLIENT-0125](../requirements/client.md#req-client-0125)
- [REQ-CLIENT-0126](../requirements/client.md#req-client-0126)
- [REQ-CLIENT-0127](../requirements/client.md#req-client-0127)
- [REQ-CLIENT-0128](../requirements/client.md#req-client-0128)

Recommended node views

- Node list and detail views.
- Capability report view (CPU, RAM, GPU presence, labels, sandbox mode).
- Current health and last heartbeat.
- Current load indicators (queue depth, concurrency), when available.
- Effective node-local constraints where applicable (sandbox mode, max concurrency).

Recommended admin actions

- Enable or disable a node for scheduling.
- Drain a node (stop assigning new jobs, allow in-flight jobs to complete).
- Request a node configuration refresh.
- Request a node to pre-pull a sandbox image, when allowed.

See [`docs/tech_specs/worker_node.md`](worker_node.md) for node lifecycle and capability reporting.

## Implementation Specification (Nuxt)

The web console SHOULD be implemented as a Nuxt application (Vue).
The console is a user-facing client and MUST only interact with CyNodeAI through the User API Gateway.

### Implementation Specification Applicable Requirements

- Spec ID: `CYNAI.WEBCON.Implementation` <a id="spec-cynai-webcon-implementation"></a>

Traces To:

- [REQ-WEBCON-0105](../requirements/webcon.md#req-webcon-0105)
- [REQ-WEBCON-0106](../requirements/webcon.md#req-webcon-0106)
- [REQ-WEBCON-0107](../requirements/webcon.md#req-webcon-0107)

### Technology Choices

Recommended baseline

- Nuxt (Vue) for the web application framework.
- TypeScript for the UI codebase.
- A component library with accessible primitives and consistent styling.
- A typed API client layer that centralizes gateway calls and auth headers.

### Application Structure

Recommended structure

- `pages/`
  - Route-level views (credentials, preferences, nodes, audit).
- `components/`
  - Reusable UI components (tables, forms, dialogs).
- `composables/`
  - `useGatewayClient` and resource-specific hooks (credentials, preferences, nodes).
- `middleware/`
  - Route protection and role checks.
- `server/`
  - Optional server routes for same-origin proxying, when desired.

### Authentication Model

The console SHOULD use gateway-issued access tokens.
Tokens SHOULD be short-lived.

Recommended approaches

- Session cookie issued by the gateway after login.
- Bearer token stored in an HttpOnly cookie and attached by the gateway proxy.

### Authentication Model Applicable Requirements

- Spec ID: `CYNAI.WEBCON.AuthModel` <a id="spec-cynai-webcon-authmodel"></a>

Traces To:

- [REQ-WEBCON-0108](../requirements/webcon.md#req-webcon-0108)
- [REQ-WEBCON-0109](../requirements/webcon.md#req-webcon-0109)

### Gateway API Client

The console SHOULD provide a single API client abstraction for the User API Gateway.
All requests MUST include request identifiers and be auditable.

Recommended behaviors

- Centralize base URL configuration.
- Centralize auth header or cookie behavior.
- Standardize error handling (401, 403, 429, 5xx).
- Support pagination and filtering for tables.

### UI Requirements by Domain

Credentials UI requirements

- Create credential (secret write-only).
- Rotate credential (secret write-only).
- Disable credential.
- List credentials with metadata only.

Preferences UI requirements

- Provide an easy method for users to create, read, update, and delete personal (user), group, and project preferences (full CRUD).
- View and edit preferences by scope.
- Show precedence and effective preferences preview.
- Validate known key types when possible.

Task creation UI requirements

- Allow submitting the task as plain text or Markdown (inline input or paste).
- Support attaching one or more files or other artifacts (file upload); same semantics as CLI attachment paths and gateway task-create API.
- Support running a script (e.g. script file upload or path input) and a short series of commands (e.g. multi-line or list input); same semantics as CLI `--script` and `--commands` and gateway.

Node UI requirements

- List nodes and show health, last heartbeat, labels, and capability summary.
- Provide actions with confirmation (enable, disable, drain, refresh config).

### Deployment Options

Recommended deployments

- Serve the console from the User API Gateway as static assets behind the same origin.
- Or deploy the console separately, but always point it at the gateway URL.

### Deployment Options Applicable Requirements

- Spec ID: `CYNAI.WEBCON.Deployment` <a id="spec-cynai-webcon-deployment"></a>

Traces To:

- [REQ-WEBCON-0110](../requirements/webcon.md#req-webcon-0110)
- [REQ-WEBCON-0111](../requirements/webcon.md#req-webcon-0111)

## API Surface

- Spec ID: `CYNAI.WEBCON.ApiSurface` <a id="spec-cynai-webcon-apisurface"></a>

The web console is a client of the User API Gateway.
It uses the Data REST API for resource-oriented access and may use dedicated admin endpoints when needed.

Minimum required API capabilities

- Task create with task body (text or Markdown), optional attachments (file upload or path semantics per gateway), script run (script file/path), or short series of commands.
- Preferences CRUD for allowed scopes.
- Effective preferences resolution for a given task or project.
- Credential metadata CRUD and secret rotation endpoints.
- Node inventory read endpoints (list, detail, health, capability report).
- Node admin action endpoints (enable, disable, drain, refresh configuration).
- Access control rule CRUD for admins.
- Audit log query endpoints for admins.

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md) and [`docs/tech_specs/data_rest_api.md`](data_rest_api.md).

## API Documentation (Swagger UI)

- Spec ID: `CYNAI.WEBCON.SwaggerUi` <a id="spec-cynai-webcon-swaggerui"></a>

Traces To:

- [REQ-WEBCON-0113](../requirements/webcon.md#req-webcon-0113)

The web console MUST provide Swagger UI (or an equivalent API documentation UI) for the User API Gateway.
Authenticated admins MUST be able to discover and try API endpoints (e.g. OpenAPI/Swagger spec served by the gateway, rendered in the console).
Access to Swagger UI MUST be subject to the same authentication and authorization as the rest of the console; the UI MUST NOT expose the ability to call endpoints the admin is not authorized to use.

## Audit and Change History

The gateway MUST emit audit logs for web console actions.
The UI SHOULD expose read-only audit views for administrators.

Recommended audit views

- Credential create, rotate, disable events (metadata only).
- Preference change history with reasons.
- Access control decisions for egress services.

## MVP Scope

Minimum viable web console

- Login and session management.
- Credential create, list, rotate, and disable for API Egress.
- Preference list and edit for system, user, group, and project scopes, with easy CRUD for personal, group, and project preferences.
- Effective preferences preview for a task.
- Node inventory list and detail views.
- Node enable, disable, and drain actions.

Future enhancements

- Group scope management and role assignment.
- Git Egress credential management.
- ACL rule editor with templates and safety rails.
- Webhooks and messaging destination management.
