# MCP Endpoint Registry (Draft)

- [1 Overview](#1-overview)
- [2 Goals](#2-goals)
- [3 Endpoint Record Model](#3-endpoint-record-model)
- [4 Endpoint Creation Flow](#4-endpoint-creation-flow)
- [5 Default Endpoints and Bootstrap](#5-default-endpoints-and-bootstrap)
- [6 Same Endpoint, Different Users](#6-same-endpoint-different-users)
- [7 RBAC and Access Control](#7-rbac-and-access-control)
- [8 Registration API](#8-registration-api)
- [9 Resolution](#9-resolution)
- [10 Credential Injection for Agent Calls](#10-credential-injection-for-agent-calls)
- [11 Credential Storage](#11-credential-storage)
- [12 Related Documents](#12-related-documents)

## 1 Overview

This draft spec defines the **registry of external MCP server endpoints** and how the User API Gateway and MCP gateway use it.
It resolves the gaps called out in [MCP Tool Definitions](mcp_tool_def.md#spec-cynai-mcptoo-dependenciesgaps): (1) no defined registry of external MCP endpoints or resolution of `Server` to URL and credentials; (2) no defined behavior when the same external endpoint is used by different users.

The registry stores endpoint records (base URL, optional credential reference, owner, scope).
Tool definitions reference an endpoint by a **stable key** (e.g. endpoint id or slug) in the `Server` field; the gateway **resolves** that key to a target URL and credentials in request context (authenticated user, task, or agent).
RBAC controls who may register, read, update, or delete endpoint records and who may reference shared endpoints in tool definitions.

### 1.1 See Also

- [MCP Tool Definitions](mcp_tool_def.md) (defines `Server` as `default` or an API endpoint key)
- [User-Installable MCP Tools](../tech_specs/user_installable_mcp_tools.md)
- [User API Gateway](../tech_specs/user_api_gateway.md)
- [Access Control](../tech_specs/access_control.md)
- [RBAC and Groups](../tech_specs/rbac_and_groups.md)
- [Connector Framework](../tech_specs/connector_framework.md) (credential and owner patterns)

## 2 Goals

- Provide a single source of truth for external MCP server endpoints (base URL, credential reference, ownership, scope).
- Allow **per-user** endpoints (each user registers the same or different URLs with their own credentials) and optional **shared** endpoints (admin-registered, referenceable by users who have permission).
- Integrate with existing RBAC: users manage their own endpoints; admins can create and manage shared endpoints; access control rules gate use of endpoints in tool invocations.
- Expose registration and lifecycle via the User API Gateway so Web Console and CLI can manage endpoints.
- Support gateway **resolution**: given a `Server` key and request context (user, task), return base URL and credentials to call the MCP server.

## 3 Endpoint Record Model

- Spec ID: `CYNAI.MCPTOO.EndpointRecord` <a id="spec-cynai-mcptoo-endpointrecord"></a>

An **endpoint record** represents one external MCP server that can be used as the target for tool definitions (when `Server` is not `default`).

### 3.1 Recommended Fields

- `id` (uuid, pk): stable identifier for the record.
- `slug` (text, unique per owner+scope): human- and config-friendly key used as `Server` in tool definitions (e.g. `acme-mcp`, `my-openai-proxy`).
  Uniqueness: per (owner_type, owner_id) for user-scoped; global for shared.
- `display_name` (text): optional label for UI.
- `base_url` (text): base URL of the MCP server (e.g. `https://mcp.example.com/v1`).
  Must be HTTPS in production unless overridden by policy.
- `credential_ref` (text or uuid): reference to stored credentials (see [Credential storage](#11-credential-storage)).
  Optional for no-auth endpoints; required when the server requires auth.
- `owner_type` (text): one of `user` | `system`.
  `user` = owned by a user; `system` = shared endpoint (admin-managed).
- `owner_id` (uuid): for `owner_type=user`, the user id; for `owner_type=system`, null or system user id.
- `scope` (text): one of `user` | `shared`.
  `user` = only the owner (and delegates via RBAC) may use this endpoint in tool definitions.
  `shared` = any user or agent with explicit permission (e.g. access control rule or role) may reference this endpoint.
- `endpoint_type` (text, optional): kind of API endpoint for routing and validation; default `mcp`.
  Reserved values: `mcp` (MCP server); future use: `rest`, `webhook`, or other types as the registry is extended.
- `created_at`, `updated_at`, `deleted_at` (timestamptz): standard lifecycle; soft delete so tool definitions can be audited after endpoint removal.

### 3.2 Table Name (Implementation)

When implemented: e.g. `mcp_endpoint_registry` or `mcp_endpoints`.

## 4 Endpoint Creation Flow

- Spec ID: `CYNAI.MCPTOO.EndpointCreationFlow` <a id="spec-cynai-mcptoo-endpointcreationflow"></a>

All API endpoint records (including MCP server endpoints) are created through the User API Gateway only.
There is no direct database insert or agent-mediated creation; this ensures a single, auditable path and consistent RBAC.

### 4.1 Creation Path

1. **Caller:** User (or admin) authenticates to the User API Gateway and sends `POST /v1/mcp/endpoints` (or a future generalized `/v1/endpoints` if the registry supports multiple endpoint types).
2. **Authorization:** Gateway evaluates `mcp.endpoint.create` for the subject.
   For `scope=user`, the subject must be an authenticated user; `owner_id` is set to that user.
   For `scope=shared`, the subject must have an admin (or equivalent) role; `owner_type=system`.
3. **Request body:** Caller supplies at least: `slug`, `base_url`; optionally `display_name`, `credential_ref`, `scope` (default `user`), `endpoint_type` (default `mcp`).
   If the external server requires auth, `credential_ref` MUST reference a credential that already exists in the system (e.g. created via API Egress credential API or a dedicated gateway credential endpoint).
   The gateway MUST NOT accept raw secrets in the endpoint create payload; credentials are always by reference.
4. **Validation:** Gateway validates slug uniqueness for the owner/scope, base_url format (HTTPS in production unless policy allows otherwise), and that `credential_ref` (if present) is resolvable and owned by the same owner or system.
5. **Persistence:** Gateway creates the endpoint record with `owner_type`, `owner_id`, `scope`, and timestamps; soft-delete field is null.
6. **Response:** Gateway returns the created endpoint (id, slug, display_name, base_url, scope, endpoint_type; credential_ref may be redacted or omitted per policy).

Credentials for endpoints may be created in a separate step (e.g. user stores an API key or token via the API Egress credential API or a connector credential flow) and then referenced by id or slug in `credential_ref` when creating the endpoint.
This keeps secret handling in the credential store and out of the endpoint registry.

## 5 Default Endpoints and Bootstrap

- Spec ID: `CYNAI.MCPTOO.DefaultEndpointsBootstrap` <a id="spec-cynai-mcptoo-defaultendpointsbootstrap"></a>

A **default set of API endpoints** (e.g. well-known MCP servers or system-provided integration endpoints) MAY be loaded into the endpoint registry when the orchestrator boots.
These endpoints are pulled in via the orchestrator bootstrap configuration so that tool definitions can reference them by stable slug without per-user registration.

### 5.1 Default Endpoint Behavior

- **Source:** Bootstrap YAML (or equivalent) defines a list of default endpoints; see [Orchestrator Bootstrap](../tech_specs/orchestrator_bootstrap.md).
  Recommended key: `mcp_endpoints` or `api_endpoints` (array of endpoint objects).
  Each object SHOULD include: `slug` (string, stable), `display_name` (optional), `base_url` (string), `credential_ref` (optional; or credential env var name for bootstrap to resolve), `endpoint_type` (optional, default `mcp`).
  Secrets MUST NOT appear in YAML; use env var names or secrets manager references.
- **When:** During orchestrator bootstrap, after the database is available and before the gateway serves traffic, the bootstrap process reads the default endpoints and upserts them into the endpoint registry table.
- **Record shape:** Each default endpoint is stored with `owner_type=system`, `owner_id` null (or system user id), `scope=shared`, and a **stable slug** (e.g. `builtin-git`, `builtin-filesystem`) so that tool definitions and resolution can rely on predictable keys.
- **Credentials:** If a default endpoint requires auth, the bootstrap config MUST NOT contain plaintext secrets; credentials MUST be supplied via environment variables or a secrets manager.
  The bootstrap process creates or references a credential in the credential store (e.g. API Egress or dedicated store) and sets `credential_ref` on the endpoint record.
- **Idempotence:** Bootstrap SHOULD upsert default endpoints by slug (or by a dedicated `bootstrap_id` key) so that re-running bootstrap or restarting the orchestrator does not duplicate rows; updates to base_url or credential_ref may be applied when the bootstrap definition changes.
- **RBAC:** Default endpoints are shared; use is gated by `mcp.endpoint.use` (or equivalent) for the endpoint resource so that admins can restrict which users or agents may call which default endpoints.

Tool definitions that reference a default endpoint use its slug as `Server`; resolution and credential injection follow the same path as user- or admin-created endpoints (see [Resolution](#9-resolution) and [Credential injection for agent calls](#10-credential-injection-for-agent-calls)).

## 6 Same Endpoint, Different Users

- Spec ID: `CYNAI.MCPTOO.SameEndpointDifferentUsers` <a id="spec-cynai-mcptoo-sameendpointdifferentusers"></a>

**Per-user registration:** The same external URL (e.g. `https://mcp.acme.com`) MAY be registered multiple times: once per user (or per owner) with distinct `slug` and `credential_ref`.
Each user has their own row; credentials are isolated per owner.
Tool definitions that reference a user-scoped endpoint use the `Server` slug; resolution uses the **current request context** (authenticated user or task owner) to select the endpoint row and its credential.

**Shared endpoint:** An admin (or system) registers an endpoint once with `scope=shared` and `owner_type=system`.
Users do not duplicate the URL; they reference the same slug (e.g. `acme-shared`) in their tool definitions.
Access to use that slug is gated by RBAC (e.g. role or access control rule granting `mcp.endpoint.use` for resource `mcp.endpoint/<id>`).
Credentials for shared endpoints are stored under a system or service identity and MUST NOT expose user-level secrets.

**Resolution order:** When the gateway resolves `Server` for a tool invocation: (1) If `Server` is `default`, use built-in gateway.
(2) Else look up endpoint by slug (and optionally by owner/scope).
For user-scoped endpoints, the resolver MUST restrict to the current user's (or task owner's) endpoints plus shared endpoints the user is allowed to use.
(3) Return base_url and resolved credential for the call.

## 7 RBAC and Access Control

- Spec ID: `CYNAI.MCPTOO.EndpointRBAC` <a id="spec-cynai-mcptoo-endpointrbac"></a>

Access control for endpoint registry MUST align with [Access Control](../tech_specs/access_control.md) and [RBAC and Groups](../tech_specs/rbac_and_groups.md).

### 7.1 Recommended Actions and Resources

- **`mcp.endpoint.create`**: Register a new endpoint (user-scoped or, with admin role, shared).
- **`mcp.endpoint.read`**: List or get endpoint metadata (base_url redacted or omitted for non-owners unless admin).
- **`mcp.endpoint.update`**: Update endpoint (slug, display_name, base_url, credential_ref); only owner or admin.
- **`mcp.endpoint.delete`**: Soft-delete endpoint; only owner or admin.
- **`mcp.endpoint.use`**: Use the endpoint as the target of a tool definition (resolve for invocation).
  For user-scoped endpoints, the caller MUST be the owner (or delegate); for shared endpoints, the caller MUST have an allow rule for this resource.

### 7.2 Endpoint RBAC Rules

- Users MAY create endpoints with `owner_type=user`, `owner_id` = their user id, `scope=user`.
- Only users with an admin or operator role (or equivalent) MAY create `scope=shared` endpoints with `owner_type=system`.
- List/get endpoints: users see their own endpoints plus shared endpoints they are allowed to use; admins may see all.
- Resolution for tool invocation: the gateway MUST enforce that the resolved endpoint is either (a) owned by the same user (or task owner) as the request context, or (b) shared and the subject has `mcp.endpoint.use` for that endpoint resource.
- Audit: all create, update, delete, and use (invocation) events SHOULD be audited with subject, resource, and decision.

## 8 Registration API

- Spec ID: `CYNAI.MCPTOO.EndpointRegistrationAPI` <a id="spec-cynai-mcptoo-endpointregistrationapi"></a>

The User API Gateway MUST expose a REST API for endpoint lifecycle so that Web Console and CLI can register and manage endpoints without direct DB access.

### 8.1 Recommended Surface

To be aligned with [User API Gateway](../tech_specs/user_api_gateway.md) and Data REST API patterns:

- `POST /v1/mcp/endpoints`: Create endpoint (body: slug, display_name, base_url, credential_ref optional, scope; default scope=user).
  Requires `mcp.endpoint.create`; scope=shared requires admin.
- `GET /v1/mcp/endpoints`: List endpoints visible to the caller (own + allowed shared).
  Query params: optional filter by scope, owner.
- `GET /v1/mcp/endpoints/{id_or_slug}`: Get one endpoint (metadata; base_url and credential_ref handling per policy-e.g. redact for non-owners).
- `PATCH /v1/mcp/endpoints/{id_or_slug}`: Update endpoint; requires `mcp.endpoint.update` for that resource.
- `DELETE /v1/mcp/endpoints/{id_or_slug}`: Soft-delete; requires `mcp.endpoint.delete` for that resource.

Responses MUST follow gateway error and JSON conventions; list MAY be paginated.

## 9 Resolution

- Spec ID: `CYNAI.MCPTOO.EndpointResolution` <a id="spec-cynai-mcptoo-endpointresolution"></a>

When the MCP gateway executes a tool definition whose `Server` is not `default`, it **resolves** `Server` to (base_url, credentials) before sending the tool call to the external MCP server.

### 9.1 Algorithm (Conceptual)

1. Let `key` = the `Server` value from the tool definition (e.g. slug or endpoint id).
2. Resolve request context: user_id (and optionally task_id, project_id) from the authenticated caller or agent token.
3. Look up endpoint: find record where (slug = key OR id = key) AND (owner_id = context.user_id for user-scoped, OR scope = shared AND subject has `mcp.endpoint.use` for this endpoint).
4. If not found, return error (endpoint not found or not authorized).
5. Load credential from `credential_ref` if present (per [Credential storage](#11-credential-storage)).
6. Return (base_url, credential) for the gateway to perform the outbound MCP call.

### 9.2 Caching and Invalidation

The resolver MAY cache (key, context) -> (base_url, credential) with short TTL; MUST invalidate or re-check when endpoint or ACL is updated.

## 10 Credential Injection for Agent Calls

- Spec ID: `CYNAI.MCPTOO.CredentialInjection` <a id="spec-cynai-mcptoo-credentialinjection"></a>

Agents MUST NOT receive or observe API credentials for registered endpoints.
When an agent invokes a tool that targets a registered endpoint (i.e. `Server` is not `default`), the MCP gateway performs credential resolution and injection server-side; the agent's request and response contain only tool identity and business payload.

### 10.1 Injection Flow

1. **Agent request:** The agent sends a tool call (tool name, arguments) to the MCP gateway.
   The agent does not send and MUST NOT be given any credential or token for the external endpoint.
2. **Resolution:** The gateway resolves `Server` from the tool definition to an endpoint record and loads the associated credential via `credential_ref` (see [Resolution](#9-resolution) and [Credential storage](#11-credential-storage)).
   Resolution and credential load occur only in gateway/service context; plaintext credentials MUST NOT be sent to the agent or to sandbox/worker processes.
3. **Injection:** The gateway builds the outbound request to the external MCP server (or other API) and **injects** the resolved credential into that request according to the endpoint's auth scheme (e.g. `Authorization` header, `Bearer` token, or MCP-specific auth header if defined).
   The gateway acts as the client to the external server; the agent never sees the injected headers or body that contain secrets.
4. **Outbound call:** The gateway sends the request to `base_url` with credentials applied; receives the response; returns to the agent only the normalized tool result (e.g. MCP tool result content), not raw headers or credential material.

This behavior aligns with [API Egress Server](../tech_specs/api_egress_server.md) (agents do not make outbound calls with keys; the service performs the call on behalf of the subject) and [Connector Framework](../tech_specs/connector_framework.md) (worker sandboxes MUST NOT contain connector credentials; orchestrator-controlled services perform operations with policy enforcement).
Audit logs for tool invocations SHOULD record that an endpoint was used and the subject; they MUST NOT log credential plaintext.

## 11 Credential Storage

- Spec ID: `CYNAI.MCPTOO.EndpointCredentialStorage` <a id="spec-cynai-mcptoo-endpointcredentialstorage"></a>

Endpoint records store a **reference** to credentials (e.g. `credential_ref`), not the secret itself.
Credentials MUST be stored encrypted at rest and MUST follow the same patterns as [Connector Framework - Credential storage](../tech_specs/connector_framework.md#spec-cynai-connec-conncredstorage) or API Egress credential handling.

### 11.1 Recommended Approach

- Per-user endpoints: credential stored under the user's scope (e.g. in a credentials table keyed by user_id and a ref id); only the owning user (and gateway with service identity) can resolve.
- Shared endpoints: credential stored under a system or service identity; only the gateway (and admins with explicit permission) can resolve.
- `credential_ref` in the endpoint row is an opaque id or slug that the gateway resolves via a secure credential store; the registry MUST NOT persist plaintext secrets.

## 12 Related Documents

- [MCP Tool Definitions](mcp_tool_def.md): defines `Server` and tool definition structs; references this registry for resolution.
- [User-Installable MCP Tools](../tech_specs/user_installable_mcp_tools.md): registration of tools; tool definitions reference endpoints by the keys defined here.
- [User API Gateway](../tech_specs/user_api_gateway.md): exposes the registration API and enforces auth/RBAC.
- [Access Control](../tech_specs/access_control.md), [RBAC and Groups](../tech_specs/rbac_and_groups.md): actions and policy.
- [Connector Framework](../tech_specs/connector_framework.md): owner/scoped credential and lifecycle patterns.
- [MCP Gateway Enforcement](../tech_specs/mcp_gateway_enforcement.md): tool call enforcement; resolution runs before the gateway forwards the call.
