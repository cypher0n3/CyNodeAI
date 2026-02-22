# API Egress Server

- [Document Overview](#document-overview)
- [Service Purpose](#service-purpose)
- [Agent Interaction Model](#agent-interaction-model)
- [Credential Storage](#credential-storage)
  - [API Credentials Table](#api-credentials-table)
- [Access Control](#access-control)
- [Policy and Auditing](#policy-and-auditing)
- [Admin API (Gateway Endpoints)](#admin-api-gateway-endpoints)

## Document Overview

- Spec ID: `CYNAI.APIEGR.Doc.ApiEgressServer` <a id="spec-cynai-apiegr-doc-apiegressserver"></a>

This document defines the API Egress Server, a service that performs outbound API calls on behalf of agents.
It is designed to keep external API keys out of sandbox containers while providing controlled, auditable network egress.

## Service Purpose

- Provide a single, controlled network boundary for outbound requests.
- Ensure API keys are never exposed to sandbox containers or to agent processes.
- Centralize rate limiting, allowlists, and auditing for external API access.

## Agent Interaction Model

Agents do not make outbound network calls directly.
Instead, they submit a structured request to the orchestrator, which routes approved calls to the API Egress Server.

Minimum request fields

- `provider`: logical service name (e.g. github, slack, openai)
- `operation`: named action (e.g. create_issue, post_message)
- `params`: json object containing inputs for the operation
- `task_id`: task context for auditing and traceability

Minimum response fields

- `status`: success|error
- `result`: json object (provider response, normalized)
- `error`: structured error object when `status` is error

## Credential Storage

- Spec ID: `CYNAI.APIEGR.CredentialStorage` <a id="spec-cynai-apiegr-credentialstorage"></a>

Credentials are stored in PostgreSQL and are only retrievable by the API Egress Server.
Agents MUST never receive credentials in responses.

Traces To:

- [REQ-APIEGR-0106](../requirements/apiegr.md#req-apiegr-0106)
- [REQ-APIEGR-0107](../requirements/apiegr.md#req-apiegr-0107)
- [REQ-APIEGR-0108](../requirements/apiegr.md#req-apiegr-0108)
- [REQ-APIEGR-0109](../requirements/apiegr.md#req-apiegr-0109)

Database schema

- The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
- The API Egress credentials table is specified in the [API Egress Credentials](postgres_schema.md#spec-cynai-schema-apiegresscredentials) section.

### API Credentials Table

- `id` (uuid, pk)
- `owner_type` (text)
  - one of: user|group
- `owner_id` (uuid)
  - user id or group id, depending on owner_type
- `provider` (text)
- `credential_type` (text)
  - examples: api_key, oauth_token, bearer_token
- `credential_name` (text)
  - human-friendly label to support multiple keys per user and provider
- `credential_ciphertext` (bytea)
- `credential_kid` (text)
  - key identifier for envelope encryption rotation
- `is_active` (boolean)
- `expires_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`owner_type`, `owner_id`, `provider`, `credential_name`)
- Index: (`owner_type`, `owner_id`, `provider`)
- Index: (`provider`)

Security notes

- Encryption SHOULD be envelope encryption with a master key that is not stored in PostgreSQL.
- The API Egress Server SHOULD be the only service with permission to decrypt credentials.
- Credential rotation SHOULD be supported without changing agent behavior.

## Access Control

- Spec ID: `CYNAI.APIEGR.AccessControl` <a id="spec-cynai-apiegr-accesscontrol"></a>

The API Egress Server MUST enforce access control for outbound API calls.
Access control rules are defined in [`docs/tech_specs/access_control.md`](access_control.md).

Traces To:

- [REQ-APIEGR-0110](../requirements/apiegr.md#req-apiegr-0110)
- [REQ-APIEGR-0111](../requirements/apiegr.md#req-apiegr-0111)
- [REQ-APIEGR-0112](../requirements/apiegr.md#req-apiegr-0112)
- [REQ-APIEGR-0113](../requirements/apiegr.md#req-apiegr-0113)
- [REQ-APIEGR-0114](../requirements/apiegr.md#req-apiegr-0114)
- [REQ-APIEGR-0115](../requirements/apiegr.md#req-apiegr-0115)
- [REQ-APIEGR-0116](../requirements/apiegr.md#req-apiegr-0116)
- [REQ-APIEGR-0117](../requirements/apiegr.md#req-apiegr-0117)
- [REQ-APIEGR-0118](../requirements/apiegr.md#req-apiegr-0118)

Recommended checks

- Subject identity MUST be resolved to a user context.
- The requested `provider` and `operation` MUST be validated against allow policy for that subject.
- The chosen credential MUST be authorized for the request context and MUST be active.
- The service SHOULD apply per-user and per-task constraints, such as rate limits and allowed operations.

Group-scoped credentials

- The API Egress Server SHOULD support group-scoped credentials for shared enterprise integrations.
- A group-scoped credential MUST be selectable only when the task context includes a group identity and policy allows group usage.
- Access control rules SHOULD distinguish between user-scoped and group-scoped usage when needed.

## Policy and Auditing

- Spec ID: `CYNAI.APIEGR.PolicyAuditing` <a id="spec-cynai-apiegr-policyauditing"></a>

The orchestrator and API Egress Server enforce outbound access policy.

Traces To:

- [REQ-APIEGR-0119](../requirements/apiegr.md#req-apiegr-0119)
- [REQ-APIEGR-0120](../requirements/apiegr.md#req-apiegr-0120)

- Policy checks SHOULD include provider allowlists, operation allowlists, and per-task constraints.
- All calls SHOULD be logged with task context, provider, operation, and timing information.
- Responses SHOULD be filtered to avoid accidental secret leakage.

## Admin API (Gateway Endpoints)

- Spec ID: `CYNAI.APIEGR.AdminApiGatewayEndpoints` <a id="spec-cynai-apiegr-adminapigatewayendpoints"></a>

The User API Gateway exposes credential management endpoints so that the Web Console and the CLI management app (cynork) can perform the same operations via the same API.
Both clients MUST use these gateway endpoints; the endpoint contract is defined here so that capability parity (REQ-CLIENT-0004) is implementable.

Traces To:

- [REQ-CLIENT-0116](../requirements/client.md#req-client-0116)
- [REQ-CLIENT-0117](../requirements/client.md#req-client-0117)
- [REQ-CLIENT-0118](../requirements/client.md#req-client-0118)
- [REQ-CLIENT-0119](../requirements/client.md#req-client-0119)
- [REQ-CLIENT-0120](../requirements/client.md#req-client-0120)

Endpoint contract

- All endpoints MUST live under the gateway's versioned API prefix (e.g. `/v1/`) and MUST require authentication and authorization.
- The gateway MUST audit credential create, rotate, and disable; MUST NOT return secret values in any response; and MUST enforce scope (user/group) so that callers only access credentials they are allowed to manage.

List credentials

- `GET /v1/credentials` (or equivalent under the gateway prefix).
- Query params: optional filter by `provider`, `owner_type`, `owner_id`.
- Response: array of credential metadata only (id, owner_type, owner_id, provider, credential_name, credential_type, is_active, expires_at, created_at, updated_at; no credential_ciphertext or secret).

Get credential (metadata only)

- `GET /v1/credentials/{id}`.
- Response: single credential metadata; 404 if not found or not authorized.

Create credential

- `POST /v1/credentials`.
- Body: provider, credential_name, owner_type, owner_id, credential_type, and secret (write-only; never echoed in response or logs).
- Response: 201 with metadata of created credential (no secret).

Rotate credential

- `POST /v1/credentials/{id}/rotate` (or equivalent action).
- Body: new secret only.
- Response: 200 with updated metadata (no secret); 404 if not found or not authorized.

Disable credential

- `PATCH /v1/credentials/{id}` with body indicating deactivation (e.g. `is_active: false`), or dedicated `POST /v1/credentials/{id}/disable`.
- Response: 200 with updated metadata; 404 if not found or not authorized.

Clients

- The [Web Console](web_console.md) and the [cynork CLI](cynork_cli.md) MUST use the above endpoint contract for API Egress credential operations.
- The gateway SHOULD expose this contract in its OpenAPI/Swagger spec for discovery and for the admin console Swagger UI.
