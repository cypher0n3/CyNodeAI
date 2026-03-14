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
- [Personas Management](#personas-management)
- [Project Management](#project-management)
- [Node Management](#node-management)
  - [Node Management Applicable Requirements](#node-management-applicable-requirements)
- [Implementation Specification (Nuxt 4)](#implementation-specification-nuxt-4)
  - [Implementation Specification Applicable Requirements](#implementation-specification-applicable-requirements)
  - [Technology Choices](#technology-choices)
  - [Runtime, Port, and Container](#runtime-port-and-container)
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

Normative requirements for the Web Console are in [docs/requirements/webcon.md](../requirements/webcon.md) (REQ-WEBCON-*).
Shared client requirements (capability parity with CLI, credentials, preferences, nodes, system settings, projects, tasks) are in [docs/requirements/client.md](../requirements/client.md).
This spec implements those requirements and adds implementation detail for the Nuxt 4 (Vue) stack, runtime (port 8080, own container in the orchestrator stack), and UI behavior.

## Capability Parity With CLI

- Spec ID: `CYNAI.CLIENT.WebConsoleCapabilityParity` <a id="spec-cynai-client-webconcapabilityparity"></a>

### Traces to Requirements

- [REQ-CLIENT-0004](../requirements/client.md#req-client-0004)

The Web Console and the CLI management app MUST offer the same administrative capabilities.
When adding or changing a capability in this spec (for example a new credential workflow, preference scope, node action, or skill operation), the [cynork CLI](cynork_cli.md) spec and implementation MUST be updated to match, and vice versa.
Use the same gateway APIs and the same authorization and auditing rules for both clients.

## Primary Use Cases

- Upload and manage external provider credentials for API Egress and Git Egress.
- View, edit, and audit user preferences across scopes (system, user, group, project, task), with an easy method to CRUD personal, group, and project preferences.
- View node inventory and manage basic node lifecycle controls.
- Full CRUD for AI skills: create (upload), list, view (content and metadata), edit (update content and/or metadata including scope, with same auditing and scope permissions), and delete; see [Skill Management CRUD](skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud).
- Full CRUD for Agent personas: create, list, view, edit, delete (RBAC per scope); same capability set as cynork per [REQ-CLIENT-0004](../requirements/client.md#req-client-0004) and [REQ-CLIENT-0179](../requirements/client.md#req-client-0179); see [cynode_sba.md - Persona on the Job](cynode_sba.md#spec-cynai-sbagnt-jobpersona).
- Basic project management (CRUD): create, list, view, edit, and delete or disable projects; each project has a user-friendly title and optional text description (see [Projects and Scope Model](projects_and_scopes.md), [REQ-PROJCT-0103](../requirements/projct.md#req-projct-0103)).
- Create tasks: submit a task as plain text or Markdown (inline or paste), attach files or other artifacts (file upload), run a script (e.g. script file upload), or run a short series of commands; same semantics as the CLI and User API Gateway (see [REQ-ORCHES-0126](../requirements/orches.md#req-orches-0126), [REQ-ORCHES-0127](../requirements/orches.md#req-orches-0127), [REQ-ORCHES-0128](../requirements/orches.md#req-orches-0128), [REQ-CLIENT-0152](../requirements/client.md#req-client-0152), [REQ-CLIENT-0154](../requirements/client.md#req-client-0154)).
- Inspect access control rules and audit decisions.

## Security Model

The following requirements apply.

### Security Model Applicable Requirements

- Spec ID: `CYNAI.WEBCON.Security` <a id="spec-cynai-webcon-security"></a>

#### Security Model Applicable Requirements Requirements Traces

- [REQ-WEBCON-0001](../requirements/webcon.md#req-webcon-0001)
- [REQ-WEBCON-0100](../requirements/webcon.md#req-webcon-0100)
- [REQ-WEBCON-0101](../requirements/webcon.md#req-webcon-0101)
- [REQ-WEBCON-0102](../requirements/webcon.md#req-webcon-0102)
- [REQ-WEBCON-0103](../requirements/webcon.md#req-webcon-0103)
- [REQ-WEBCON-0104](../requirements/webcon.md#req-webcon-0104)

Implementation MUST adhere to the following threat model.

- The browser MUST be treated as an untrusted client; no sensitive authorization decisions MUST be made in client-side code.
- Authorization, rate limiting, and auditing MUST be performed by the gateway; the console MUST NOT bypass them (see [REQ-WEBCON-0106](../requirements/webcon.md#req-webcon-0106)).
- UI sessions SHOULD use short-lived access tokens.

## Authentication and Authorization

The User API Gateway MUST authenticate all web console requests (see [REQ-WEBCON-0101](../requirements/webcon.md#req-webcon-0101)).
The gateway MUST authorize actions using access control and user context.
Implementation MUST support at least: local username and password authentication for initial deployments.
Enterprise SSO (OIDC/SAML) MAY be supported as a future extension.

## Credential Management

The web console MUST support managing credentials that are consumed by controlled egress services (see [REQ-CLIENT-0116](../requirements/client.md#req-client-0116) through [REQ-CLIENT-0120](../requirements/client.md#req-client-0120)).
The gateway endpoint contract for credential operations (list, get, create, rotate, disable) is defined in [API Egress Server - Admin API (Gateway Endpoints)](api_egress_server.md#spec-cynai-apiegr-adminapigatewayendpoints); the console and cynork MUST use these same endpoints.
Credential types: API Egress credentials (external model providers, SaaS APIs) and Git Egress credentials (GitHub/GitLab/Gitea tokens or deploy keys).

### Credential Management Applicable Requirements

- Spec ID: `CYNAI.WEBCON.Credential` <a id="spec-cynai-webcon-credential"></a>

#### Credential Management Applicable Requirements Requirements Traces

- [REQ-CLIENT-0116](../requirements/client.md#req-client-0116)
- [REQ-CLIENT-0117](../requirements/client.md#req-client-0117)
- [REQ-CLIENT-0118](../requirements/client.md#req-client-0118)
- [REQ-CLIENT-0119](../requirements/client.md#req-client-0119)
- [REQ-CLIENT-0120](../requirements/client.md#req-client-0120)

The UI MUST display credential metadata only (never plaintext secrets after creation, per [REQ-WEBCON-0103](../requirements/webcon.md#req-webcon-0103)).
Metadata fields shown MUST include: provider, credential name, owner scope (user or group), active status, created and updated timestamps; last used timestamp when available from the gateway.

## Preferences Management

The web console MUST support editing preferences stored via the gateway (see [REQ-CLIENT-0121](../requirements/client.md#req-client-0121) through [REQ-CLIENT-0124](../requirements/client.md#req-client-0124)).
It MUST provide an easy method for users to create, read, update, and delete their personal (user), group, and project preferences (full CRUD for those scopes).

### Preferences Management Applicable Requirements

- Spec ID: `CYNAI.WEBCON.Preferences` <a id="spec-cynai-webcon-preferences"></a>

#### Preferences Management Applicable Requirements Requirements Traces

- [REQ-CLIENT-0121](../requirements/client.md#req-client-0121)
- [REQ-CLIENT-0122](../requirements/client.md#req-client-0122)
- [REQ-CLIENT-0123](../requirements/client.md#req-client-0123)
- [REQ-CLIENT-0124](../requirements/client.md#req-client-0124)

UI MUST: show the precedence model and the scope from which each value is derived; provide a diff view when editing complex JSON values; require a reason field for preference changes when auditing is enabled.
The following keys MUST be surfaced for MVP:

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

### System Settings Management Requirements Traces

- [REQ-CLIENT-0160](../requirements/client.md#req-client-0160)

The web console MUST support editing system settings that control orchestrator operational behavior.
System settings are not user preferences; they are operator-controlled orchestrator operational configuration.
For the full distinction, see [User preferences (Terminology)](user_preferences.md#spec-cynai-stands-preferenceterminology).

The following system setting keys MUST be surfaced for MVP.
Key semantics and the PM model selection/warmup algorithm are defined in [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#spec-cynai-orches-projectmanagermodelstartup).

- `agents.project_manager.model.local_default_ollama_model` (string)
- `agents.project_manager.model.selection.execution_mode` (string): `auto` | `force_local` | `force_external`
- `agents.project_manager.model.selection.mode` (string): `auto_sliding_scale` | `fixed_model`
- `agents.project_manager.model.selection.prefer_orchestrator_host` (boolean)

## Skills Management

- Spec ID: `CYNAI.WEBCON.SkillsManagement` <a id="spec-cynai-webcon-skillsmanagement"></a>

### Skills Management Requirements Traces

- [REQ-WEBCON-0112](../requirements/webcon.md#req-webcon-0112)

The web console MUST support full CRUD for AI skills (create, list, view, update, delete) via the User API Gateway.
All operations use the same controls as defined in [Skill Management CRUD](skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud): authentication, scope visibility, scope elevation permission on write, and auditing on write with rejection feedback (match category and triggering text) when content fails the security scan.

UI MUST provide the following.

- **List:** table or list of skills visible to the user (metadata: name, scope, owner, updated_at); optional filters by scope or owner.
- **View:** single skill detail with full content and metadata; read-only unless the user has edit permission.
- **Create:** upload form (paste or file) for markdown content; optional name and scope (scope elevation subject to permission).
- **Edit:** update content and/or metadata (name, scope); updated content is re-audited; on failure the UI MUST show rejection reason and exact triggering text.
- **Delete:** confirm and remove skill from store and registry (restricted to owner or admin).

## Personas Management

- Spec ID: `CYNAI.WEBCON.PersonasManagement` <a id="spec-cynai-webcon-personasmanagement"></a>

### Personas Management Requirements Traces

- [REQ-CLIENT-0179](../requirements/client.md#req-client-0179)

The web console MUST support full CRUD for **Agent personas** (create, list, view, update, delete) via the User API Gateway, with the same capabilities as cynork (see [Personas Management](cli_management_app_commands_admin.md#spec-cynai-client-clipersonasmanagement)).
Agent personas are reusable SBA role/identity descriptions (not customer or end-user personas); scope_type and scope_id determine visibility; create/update/delete are subject to RBAC per scope (e.g. admin for system-scoped); see [data_rest_api.md - Core Resources](data_rest_api.md#spec-cynai-datapi-coreresources).

UI MUST provide:

- **List** - table or list of Agent personas (metadata: title, scope_type, scope_id, updated_at); optional filters by scope.
- **View** - single Agent persona detail (title, description, scope_type, scope_id, created_at, updated_at).
- **Create** - form with required title and description; optional scope_type and scope_id (scope subject to caller role).
- **Edit** - update title, description, scope_type, scope_id (only when caller has permission for that scope).
- **Delete** - confirm and remove; show warning when persona is referenced by jobs (if jobs.persona_id is present).

## Project Management

- Spec ID: `CYNAI.WEBCON.ProjectManagement` <a id="spec-cynai-webcon-projectmanagement"></a>

### Project Management Requirements Traces

- [REQ-CLIENT-0174](../requirements/client.md#req-client-0174)

The web console MUST support basic project CRUD (create, list, view, update, delete or disable) via the User API Gateway, with the same capabilities as the CLI (see [Project Management](cli_management_app_commands_admin.md#spec-cynai-client-cliprojectmanagement)).
The web console MUST support project plan review (list plans per project, view plan by plan_id, view revision history) and plan approve (re-approve) with parity to the CLI; a project may have multiple plans, with only one active at a time; see [Project Plan API](user_api_gateway.md#spec-cynai-usrgwy-projectplanapi) and [REQ-CLIENT-0180](../requirements/client.md#req-client-0180).
Projects have a user-friendly title (display name) and an optional text description for lists and detail views.

UI MUST provide: **List** - table or list of projects (columns: title, slug, description excerpt, active, updated_at); optional filter by active status.
**View** - single project detail (title, slug, description, is_active, created_at, updated_at, updated_by).
**Create** - form with required slug and title; optional description.
**Edit** - update title, description, and active status.
**Delete** - confirm and delete (or disable) project; show warning when project is referenced by tasks or chat threads.

## Node Management

The web console MUST support basic node management for operators (see [REQ-CLIENT-0125](../requirements/client.md#req-client-0125) through [REQ-CLIENT-0128](../requirements/client.md#req-client-0128)).
This MUST include inventory views, health status, and safe administrative controls.

### Node Management Applicable Requirements

- Spec ID: `CYNAI.WEBCON.NodeManagement` <a id="spec-cynai-webcon-nodemanagement"></a>

#### Node Management Applicable Requirements Requirements Traces

- [REQ-CLIENT-0125](../requirements/client.md#req-client-0125)
- [REQ-CLIENT-0126](../requirements/client.md#req-client-0126)
- [REQ-CLIENT-0127](../requirements/client.md#req-client-0127)
- [REQ-CLIENT-0128](../requirements/client.md#req-client-0128)

The UI MUST provide: node list and detail views; a capability report view (CPU, RAM, GPU presence, labels, sandbox mode); current health and last heartbeat; current load indicators (queue depth, concurrency) when available from the gateway; effective node-local constraints where applicable (sandbox mode, max concurrency).
Admin actions MUST include (with confirmation): enable or disable a node for scheduling; drain a node (stop assigning new jobs, allow in-flight jobs to complete); request a node configuration refresh; request a node to pre-pull a sandbox image when allowed by policy.

See [`docs/tech_specs/worker_node.md`](worker_node.md) for node lifecycle and capability reporting.

## Implementation Specification (Nuxt 4)

The web console MUST be implemented as a Nuxt 4 application (Vue 3).
The console is a user-facing client and MUST interact with CyNodeAI only through the User API Gateway (see [REQ-WEBCON-0101](../requirements/webcon.md#req-webcon-0101)).

### Implementation Specification Applicable Requirements

- Spec ID: `CYNAI.WEBCON.Implementation` <a id="spec-cynai-webcon-implementation"></a>

#### Implementation Specification Applicable Requirements Requirements Traces

- [REQ-WEBCON-0105](../requirements/webcon.md#req-webcon-0105)
- [REQ-WEBCON-0106](../requirements/webcon.md#req-webcon-0106)
- [REQ-WEBCON-0107](../requirements/webcon.md#req-webcon-0107)

### Technology Choices

Implementation MUST conform to the following.

- **Framework:** Nuxt 4 (Vue 3).
  Implementation MUST use Nuxt 4.x; the exact minimum version (e.g. `^4.0.0`) MUST be documented in the repository (`package.json` engines or a supported-versions doc).
- **Language:** The UI codebase MUST be TypeScript.
- **UI:** Implementation MUST use a component library that provides accessible primitives and consistent styling (permitted examples: Nuxt UI, Vuetify).
- **API layer:** Implementation MUST provide a typed API client that is the single entry point for all gateway calls; it MUST centralize gateway base URL, auth headers or cookies, and error handling.
  All gateway calls MUST go through this layer.

Build and runtime

- **Node.js:** Node is required only for build (development and CI).
  Implementation MUST document the supported Node.js version range (e.g. LTS 20.x or 22.x) in `.nvmrc` or `package.json` engines.
  Node MUST NOT be required at container runtime (see [Runtime, Port, and Container](#runtime-port-and-container)).
- **Build output:** For container deployment the console MUST be built as static assets (SSG) using `nuxt generate` (or the Nuxt 4 equivalent).
  The production container MUST serve the build output (e.g. `.output/public`) with a minimal static HTTP server only.
  Permitted run-stage servers: nginx, Caddy, or another minimal static server that serves the generated files; the run stage MUST NOT use Node or Nitro.
  When run in its own container, the console MUST serve on port 8080 by default per [REQ-WEBCON-0114](../requirements/webcon.md#req-webcon-0114).
  For local development, the Nuxt 4 dev server MAY run (Node) on a configured port.
- **Environment:** Gateway base URL MUST be configurable (at build time for static builds, e.g. `NUXT_PUBLIC_GATEWAY_URL`, or via runtime-injected config when the static server supports it).
  In the orchestrator stack the gateway is typically `http://user-gateway:12080` (service name); for browser requests from the host, the gateway URL MUST be the host-visible URL (e.g. `http://localhost:12080`) or API calls MUST be proxied same-origin.
  The deployment documentation (or this spec for the chosen deployment) MUST state how the browser resolves the gateway URL (same-origin vs separate origin).

### Runtime, Port, and Container

- Spec ID: `CYNAI.WEBCON.RuntimeAndDeployment` <a id="spec-cynai-webcon-runtimeanddeployment"></a>

#### Runtime Port and Container Requirements Traces

- [REQ-WEBCON-0114](../requirements/webcon.md#req-webcon-0114)
- [REQ-WEBCON-0115](../requirements/webcon.md#req-webcon-0115)

Default port and override

- The web console MUST listen on port 8080 by default when run as its own container.
- Port override MUST be configurable via `WEBCON_PORT` (or the static server's equivalent, e.g. nginx env or config).
  When set, the server MUST bind to the configured port.
- Bind address MUST default to `0.0.0.0` (or equivalent) in the container so the service is reachable from the host.
  See [Ports and Endpoints](ports_and_endpoints.md#spec-cynai-stands-portsandendpoints).

Container as part of orchestrator stack

- The web console MUST be runnable in its own container as part of the orchestrator stack (e.g. a service in the same docker compose file as postgres, control-plane, user-gateway, cynode-pma, ollama) per [REQ-WEBCON-0115](../requirements/webcon.md#req-webcon-0115).
- The container MUST NOT embed PostgreSQL credentials, JWT secrets, or any privileged service credentials; it MUST receive only the User API Gateway base URL (and optionally public config such as feature flags) per [REQ-WEBCON-0105](../requirements/webcon.md#req-webcon-0105).
- **Multi-stage Containerfile:** Build stage MUST use Node (npm or pnpm install, then Nuxt 4 static build via `nuxt generate`).
  Run stage MUST use a minimal static HTTP server only (permitted: nginx, Caddy, or equivalent) serving the built static assets on port 8080.
  Node MUST NOT be present or required in the run stage.
- The container MUST expose port 8080 (or the overridden port).
  The container MUST define a health check: a GET request to `/` or a dedicated health path MUST return HTTP 200 when the app is ready to serve traffic.
- In compose, the web-console service MAY depend on `user-gateway` with `condition: service_started` so the stack starts in order; the console MUST remain usable when the gateway is temporarily unavailable (UI MUST show connection errors rather than fail to load).

### Application Structure

Implementation MUST follow Nuxt 4's `app/` directory layout with the following structure.

- **`app/pages/`** - Route-level views.
  MUST include routes for credentials, preferences, nodes, and audit (or equivalent as the feature set is implemented).
- **`app/components/`** - Reusable UI components (tables, forms, dialogs).
- **`app/composables/`** - MUST include a `useGatewayClient` (or equivalent) composable and resource-specific hooks for credentials, preferences, and nodes; all gateway calls MUST go through these composables.
- **`app/middleware/`** - Route protection and role checks; unauthenticated or unauthorized access MUST be redirected or rejected.
- **`server/`** - Optional at repo root.
  Not used in static deployment; when same-origin API proxying is required, it MUST be provided by the reverse proxy or gateway, not by Nuxt server routes.

### Authentication Model

Implementation MUST use gateway-issued access tokens and MUST NOT store bearer tokens in localStorage (see [REQ-WEBCON-0108](../requirements/webcon.md#req-webcon-0108)).
Implementation MUST support logout and token invalidation (see [REQ-WEBCON-0109](../requirements/webcon.md#req-webcon-0109)).
Tokens SHOULD be short-lived.
Implementation MUST use one of: a session cookie issued by the gateway after login; or a bearer token stored in an HttpOnly cookie and attached by the gateway proxy.

### Authentication Model Applicable Requirements

- Spec ID: `CYNAI.WEBCON.AuthModel` <a id="spec-cynai-webcon-authmodel"></a>

#### Authentication Model Applicable Requirements Requirements Traces

- [REQ-WEBCON-0108](../requirements/webcon.md#req-webcon-0108)
- [REQ-WEBCON-0109](../requirements/webcon.md#req-webcon-0109)

### Gateway API Client

Implementation MUST provide a single API client abstraction for the User API Gateway (see [Technology Choices](#technology-choices)); all gateway calls MUST go through it.
The client MUST: centralize base URL configuration; centralize auth header or cookie behavior; standardize error handling for 401, 403, 429, and 5xx responses; support pagination and filtering for list/table endpoints.
All requests MUST include request identifiers so that gateway audit logs can correlate requests.

### UI Requirements by Domain

Implementation MUST satisfy the following per-domain UI contracts.

- **Credentials:** Create credential (secret write-only, per [REQ-WEBCON-0102](../requirements/webcon.md#req-webcon-0102)); rotate credential (secret write-only); disable credential; list credentials with metadata only (no plaintext secrets, per [REQ-WEBCON-0103](../requirements/webcon.md#req-webcon-0103)).
- **Preferences:** Provide an easy method for users to create, read, update, and delete personal (user), group, and project preferences (full CRUD).
  View and edit preferences by scope.
  Show precedence and effective preferences preview.
  Validate known key types when possible.
- **Task creation:** Allow submitting the task as plain text or Markdown (inline input or paste).
  Support attaching one or more files or other artifacts (file upload) with the same semantics as CLI attachment paths and the gateway task-create API.
  Support running a script (e.g. script file upload or path input) and a short series of commands (e.g. multi-line or list input) with the same semantics as CLI `--script` and `--commands` and the gateway.
- **Nodes:** List nodes and show health, last heartbeat, labels, and capability summary.
  Provide actions with confirmation: enable, disable, drain, refresh config.
- **Chat:** When the Web Console provides a chat UI, it SHOULD call the gateway chat warm-up endpoint (e.g. `POST /v1/chat/warm`) on chat view or route load, non-blocking, per [REQ-CLIENT-0177](../requirements/client.md#req-client-0177) and [Chat Model Warm-up](openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-warmup).

### Deployment Options

Implementation MUST support at least one of the following deployment modes, and the chosen mode MUST be documented (including how the browser resolves the gateway URL).

1. Console served as static assets from the User API Gateway (same origin; CORS avoided per [REQ-WEBCON-0111](../requirements/webcon.md#req-webcon-0111)).
2. Console deployed as a separate service with configurable gateway URL; the console MUST always be configured to call the gateway and MUST NOT allow overriding to a non-gateway endpoint.

Production deployments MUST enforce HTTPS per [REQ-WEBCON-0110](../requirements/webcon.md#req-webcon-0110).

### Deployment Options Applicable Requirements

- Spec ID: `CYNAI.WEBCON.Deployment` <a id="spec-cynai-webcon-deployment"></a>

#### Deployment Options Applicable Requirements Requirements Traces

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

### API Documentation (Swagger UI) Requirements Traces

- [REQ-WEBCON-0113](../requirements/webcon.md#req-webcon-0113)

The web console MUST provide Swagger UI (or an equivalent API documentation UI) for the User API Gateway.
Authenticated admins MUST be able to discover and try API endpoints (e.g. OpenAPI/Swagger spec served by the gateway, rendered in the console).
Access to Swagger UI MUST be subject to the same authentication and authorization as the rest of the console; the UI MUST NOT expose the ability to call endpoints the admin is not authorized to use.

## Audit and Change History

The gateway MUST emit audit logs for web console actions (gateway responsibility; the console MUST NOT bypass auditing).
The UI MUST expose read-only audit views for administrators.
Audit views MUST support at least: credential create, rotate, disable events (metadata only); preference change history with reasons; access control decisions for egress services.

## MVP Scope

The minimum viable web console MUST include the following.
Login and session management.
Credential create, list, rotate, and disable for API Egress.
Preference list and edit for system, user, group, and project scopes, with easy CRUD for personal, group, and project preferences.
Effective preferences preview for a task.
Node inventory list and detail views.
Node enable, disable, and drain actions.
Future enhancements (out of MVP scope): group scope management and role assignment; Git Egress credential management; ACL rule editor with templates and safety rails; webhooks and messaging destination management.
