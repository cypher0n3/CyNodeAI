# Admin Web Console

- [Document Overview](#document-overview)
- [Primary Use Cases](#primary-use-cases)
- [Security Model](#security-model)
- [Authentication and Authorization](#authentication-and-authorization)
- [Credential Management](#credential-management)
- [Preferences Management](#preferences-management)
- [Node Management](#node-management)
- [Implementation Specification (Nuxt)](#implementation-specification-nuxt)
  - [Technology Choices](#technology-choices)
  - [Application Structure](#application-structure)
  - [Authentication Model](#authentication-model)
  - [Gateway API Client](#gateway-api-client)
  - [UI Requirements by Domain](#ui-requirements-by-domain)
  - [Deployment Options](#deployment-options)
- [API Surface](#api-surface)
- [Audit and Change History](#audit-and-change-history)
- [MVP Scope](#mvp-scope)

## Document Overview

This document defines an admin-focused web interface for CyNodeAI.
The admin web console is intended for credential upload and management and for preferences management.

## Primary Use Cases

- Upload and manage external provider credentials for API Egress and Git Egress.
- View, edit, and audit preferences across scopes (system, user, project, task).
- View node inventory and manage basic node lifecycle controls.
- Inspect access control rules and audit decisions.

## Security Model

Normative requirements

- The web console MUST NOT connect directly to PostgreSQL.
- The web console MUST call the User API Gateway for all operations.
- Secrets MUST be write-only in the UI.
- The UI MUST never display plaintext secret values after creation.
- The UI MUST support least privilege and MUST not expose admin features to non-admin users.

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

Credential types

- API Egress credentials (external model providers, SaaS APIs).
- Git Egress credentials (GitHub/GitLab/Gitea tokens or deploy keys).

Normative requirements

- Credential create MUST accept secrets only on create or rotate operations.
- Credential read MUST return metadata only.
- Credential list MUST support filtering by provider and scope.
- Credential rotate MUST create a new encrypted secret value and invalidate the old one.
- Credential disable MUST support immediate deactivation.

Recommended metadata fields shown in the UI

- Provider
- Credential name
- Owner scope (user or group)
- Active status
- Created and updated timestamps
- Last used timestamp, when available

## Preferences Management

The web console must support editing preferences stored in PostgreSQL.

Normative requirements

- Preference edits MUST be scoped and versioned.
- The UI MUST support preference scope selection (system, user, project, task).
- The UI SHOULD provide an "effective preferences" preview for a given task or project.
- The UI SHOULD provide validation for known keys and types.

Recommended UI behaviors

- Show the precedence model and where a value is coming from.
- Provide a diff view when editing complex JSON values.
- Require a reason field for preference changes when auditing is enabled.

## Node Management

The admin web console should support basic node management for operators.
This includes inventory views, health status, and safe administrative controls.

Normative requirements

- Node management MUST be mediated by the User API Gateway.
- The UI MUST NOT connect directly to node worker APIs.
- The UI MUST clearly distinguish between node-reported state and orchestrator-derived state.
- Potentially disruptive actions MUST be gated by admin authorization and SHOULD require confirmation.

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

Normative requirements

- The console MUST not embed privileged service credentials.
- The console MUST not bypass gateway authorization and auditing.
- The console MUST treat gateway responses as the source of truth.

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

Normative requirements

- The console MUST avoid storing bearer tokens in localStorage.
- The console MUST support logout and token invalidation.

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

Normative requirements

- The console MUST enforce HTTPS in production deployments.
- CORS SHOULD be avoided by preferring same-origin hosting behind the gateway.

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
