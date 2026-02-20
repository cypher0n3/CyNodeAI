# Admin Web Console

- [Document Overview](#document-overview)
- [Capability Parity with CLI](#capability-parity-with-cli)
- [Primary Use Cases](#primary-use-cases)
- [Security Model](#security-model)
  - [Security Model Applicable Requirements](#security-model-applicable-requirements)
- [Authentication and Authorization](#authentication-and-authorization)
- [Credential Management](#credential-management)
  - [Credential Management Applicable Requirements](#credential-management-applicable-requirements)
- [Preferences Management](#preferences-management)
  - [Preferences Management Applicable Requirements](#preferences-management-applicable-requirements)
- [Skills Management](#skills-management)
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
The admin web console is intended for credential upload and management and for preferences management.

## Capability Parity With CLI

- Spec ID: `CYNAI.CLIENT.AdminWebConsoleCapabilityParity` <a id="spec-cynai-client-awccapabilityparity"></a>

Traces To:

- [REQ-CLIENT-0004](../requirements/client.md#req-client-0004)

The Admin Web Console and the CLI management app MUST offer the same administrative capabilities.
When adding or changing a capability in this spec (for example a new credential workflow, preference scope, node action, or skill operation), the [CLI management app](cli_management_app.md) spec and implementation MUST be updated to match, and vice versa.
Use the same gateway APIs and the same authorization and auditing rules for both clients.

## Primary Use Cases

- Upload and manage external provider credentials for API Egress and Git Egress.
- View, edit, and audit preferences across scopes (system, user, project, task).
- View node inventory and manage basic node lifecycle controls.
- Full CRUD for AI skills: create (upload), list, view (content and metadata), edit (update content and/or metadata including scope, with same auditing and scope permissions), and delete; see [Skill Management CRUD](skills_storage_and_inference.md#skill-management-crud-web-and-cli).
- Inspect access control rules and audit decisions.

## Security Model

The following requirements apply.

### Security Model Applicable Requirements

- Spec ID: `CYNAI.CLIENT.AdminWebConsoleSecurity` <a id="spec-cynai-client-awcsecurity"></a>

Traces To:

- [REQ-CLIENT-0108](../requirements/client.md#req-client-0108)
- [REQ-CLIENT-0109](../requirements/client.md#req-client-0109)
- [REQ-CLIENT-0110](../requirements/client.md#req-client-0110)
- [REQ-CLIENT-0111](../requirements/client.md#req-client-0111)
- [REQ-CLIENT-0112](../requirements/client.md#req-client-0112)

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

The admin web console must support managing credentials that are consumed by controlled egress services.
The gateway endpoint contract for credential operations (list, get, create, rotate, disable) is defined in [API Egress Server - Admin API (Gateway Endpoints)](api_egress_server.md#admin-api-gateway-endpoints); the console and cynork both use these same endpoints.

Credential types

- API Egress credentials (external model providers, SaaS APIs).
- Git Egress credentials (GitHub/GitLab/Gitea tokens or deploy keys).

### Credential Management Applicable Requirements

- Spec ID: `CYNAI.CLIENT.AdminWebConsoleCredential` <a id="spec-cynai-client-awccredential"></a>

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

### Preferences Management Applicable Requirements

- Spec ID: `CYNAI.CLIENT.AdminWebConsolePreferences` <a id="spec-cynai-client-awcpreferences"></a>

Traces To:

- [REQ-CLIENT-0121](../requirements/client.md#req-client-0121)
- [REQ-CLIENT-0122](../requirements/client.md#req-client-0122)
- [REQ-CLIENT-0123](../requirements/client.md#req-client-0123)
- [REQ-CLIENT-0124](../requirements/client.md#req-client-0124)

Recommended UI behaviors

- Show the precedence model and where a value is coming from.
- Provide a diff view when editing complex JSON values.
- Require a reason field for preference changes when auditing is enabled.

## Skills Management

- Spec ID: `CYNAI.CLIENT.AdminWebConsoleSkillsManagement` <a id="spec-cynai-client-awcskillsmanagement"></a>

Traces To:

- [REQ-CLIENT-0147](../requirements/client.md#req-client-0147)

The web console MUST support full CRUD for AI skills (create, list, view, update, delete) via the User API Gateway.
All operations use the same controls as defined in [Skill Management CRUD](skills_storage_and_inference.md#skill-management-crud-web-and-cli): authentication, scope visibility, scope elevation permission on write, and auditing on write with rejection feedback (match category and triggering text) when content fails the security scan.

Recommended UI

- **List**: Table or list of skills visible to the user (metadata: name, scope, owner, updated_at); optional filters by scope or owner.
- **View**: Single skill detail with full content and metadata; read-only unless the user has edit permission.
- **Create**: Upload form (paste or file) for markdown content; optional name and scope (scope elevation subject to permission).
- **Edit**: Update content and/or metadata (name, scope); updated content is re-audited; on failure show rejection reason and exact triggering text.
- **Delete**: Confirm and remove skill from store and registry (restricted to owner or admin).

## Node Management

The admin web console should support basic node management for operators.
This includes inventory views, health status, and safe administrative controls.

### Node Management Applicable Requirements

- Spec ID: `CYNAI.CLIENT.AdminWebConsoleNodeManagement` <a id="spec-cynai-client-awcnodemgmt"></a>

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

See [`docs/tech_specs/node.md`](node.md) for node lifecycle and capability reporting.

## Implementation Specification (Nuxt)

The admin web console SHOULD be implemented as a Nuxt application (Vue).
The console is a user-facing client and MUST only interact with CyNodeAI through the User API Gateway.

### Implementation Specification Applicable Requirements

- Spec ID: `CYNAI.CLIENT.AdminWebConsoleImplementation` <a id="spec-cynai-client-awcimpl"></a>

Traces To:

- [REQ-CLIENT-0129](../requirements/client.md#req-client-0129)
- [REQ-CLIENT-0130](../requirements/client.md#req-client-0130)
- [REQ-CLIENT-0131](../requirements/client.md#req-client-0131)

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

- Spec ID: `CYNAI.CLIENT.AdminWebConsoleAuthModel` <a id="spec-cynai-client-awcauth"></a>

Traces To:

- [REQ-CLIENT-0132](../requirements/client.md#req-client-0132)
- [REQ-CLIENT-0133](../requirements/client.md#req-client-0133)

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

- View and edit preferences by scope.
- Show precedence and effective preferences preview.
- Validate known key types when possible.

Node UI requirements

- List nodes and show health, last heartbeat, labels, and capability summary.
- Provide actions with confirmation (enable, disable, drain, refresh config).

### Deployment Options

Recommended deployments

- Serve the console from the User API Gateway as static assets behind the same origin.
- Or deploy the console separately, but always point it at the gateway URL.

### Deployment Options Applicable Requirements

- Spec ID: `CYNAI.CLIENT.AdminWebConsoleDeployment` <a id="spec-cynai-client-awcdeploy"></a>

Traces To:

- [REQ-CLIENT-0134](../requirements/client.md#req-client-0134)
- [REQ-CLIENT-0135](../requirements/client.md#req-client-0135)

## API Surface

The web console is a client of the User API Gateway.
It uses the Data REST API for resource-oriented access and may use dedicated admin endpoints when needed.

Minimum required API capabilities

- Preferences CRUD for allowed scopes.
- Effective preferences resolution for a given task or project.
- Credential metadata CRUD and secret rotation endpoints.
- Node inventory read endpoints (list, detail, health, capability report).
- Node admin action endpoints (enable, disable, drain, refresh configuration).
- Access control rule CRUD for admins.
- Audit log query endpoints for admins.

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md) and [`docs/tech_specs/data_rest_api.md`](data_rest_api.md).

## API Documentation (Swagger UI)

- Spec ID: `CYNAI.CLIENT.AdminWebConsoleSwaggerUi` <a id="spec-cynai-client-awcswaggerui"></a>

Traces To:

- [REQ-CLIENT-0148](../requirements/client.md#req-client-0148)

The admin web console MUST provide Swagger UI (or an equivalent API documentation UI) for the User API Gateway.
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

Minimum viable admin console

- Login and session management.
- Credential create, list, rotate, and disable for API Egress.
- Preference list and edit for system and user scopes.
- Effective preferences preview for a task.
- Node inventory list and detail views.
- Node enable, disable, and drain actions.

Future enhancements

- Group scope management and role assignment.
- Git Egress credential management.
- ACL rule editor with templates and safety rails.
- Webhooks and messaging destination management.
