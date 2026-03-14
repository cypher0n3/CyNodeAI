# API Egress Server

- [Document Overview](#document-overview)
- [Service Purpose](#service-purpose)
- [Agent Interaction Model](#agent-interaction-model)
- [Credential Storage](#credential-storage)
  - [Database Schema](#database-schema)
  - [Credential Storage Requirements Traces](#credential-storage-requirements-traces)
  - [API Credentials Table](#api-credentials-table)
- [Access Control](#access-control)
  - [Recommended Checks](#recommended-checks)
  - [Group-Scoped Credentials](#group-scoped-credentials)
  - [Access Control Requirements Traces](#access-control-requirements-traces)
- [Policy and Auditing](#policy-and-auditing)
  - [Policy and Auditing Requirements Traces](#policy-and-auditing-requirements-traces)
- [Sanity Check (Semantic Safety)](#sanity-check-semantic-safety)
  - [Sanity Checker Placement in Request Flow](#sanity-checker-placement-in-request-flow)
  - [Sanity Checker Inputs](#sanity-checker-inputs)
  - [Sanity Checker Outputs](#sanity-checker-outputs)
  - [Escalate to Human Review](#escalate-to-human-review)
  - [Sanity Checker Detection Categories](#sanity-checker-detection-categories)
  - [Sanity Checker Security](#sanity-checker-security)
  - [Sanity Checker Audit](#sanity-checker-audit)
  - [Sanity Checker Configuration](#sanity-checker-configuration)
  - [Sanity Checker Model Configuration](#sanity-checker-model-configuration)
  - [Sanity Checker Req Traces](#sanity-checker-req-traces)
- [Admin API (Gateway Endpoints)](#admin-api-gateway-endpoints)
  - [Get Credential (Metadata Only)](#get-credential-metadata-only)
  - [Create Credential](#create-credential)
  - [Rotate Credential](#rotate-credential)
  - [Disable Credential](#disable-credential)
  - [Admin API Clients](#admin-api-clients)
  - [Admin API Requirements Traces](#admin-api-requirements-traces)

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

### Database Schema

- The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
- The API Egress credentials table is specified in the [API Egress Credentials](postgres_schema.md#spec-cynai-schema-apiegresscredentials) section.

### Credential Storage Requirements Traces

- [REQ-APIEGR-0106](../requirements/apiegr.md#req-apiegr-0106)
- [REQ-APIEGR-0107](../requirements/apiegr.md#req-apiegr-0107)
- [REQ-APIEGR-0108](../requirements/apiegr.md#req-apiegr-0108)
- [REQ-APIEGR-0109](../requirements/apiegr.md#req-apiegr-0109)

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
- Go code that decrypts or holds credential plaintext (or the envelope master key) MUST use `runtime/secret` when available per [REQ-STANDS-0133](../requirements/stands.md#req-stands-0133); when not available, MUST use best-effort secure erasure before returning.

## Access Control

- Spec ID: `CYNAI.APIEGR.AccessControl` <a id="spec-cynai-apiegr-accesscontrol"></a>

The API Egress Server MUST enforce access control for outbound API calls.
Access control rules are defined in [`docs/tech_specs/access_control.md`](access_control.md).

### Recommended Checks

- Subject identity MUST be resolved to a user context.
- The requested `provider` and `operation` MUST be validated against allow policy for that subject.
- The chosen credential MUST be authorized for the request context and MUST be active.
- The service SHOULD apply per-user and per-task constraints, such as rate limits and allowed operations.

### Group-Scoped Credentials

- The API Egress Server SHOULD support group-scoped credentials for shared enterprise integrations.
- A group-scoped credential MUST be selectable only when the task context includes a group identity and policy allows group usage.
- Access control rules SHOULD distinguish between user-scoped and group-scoped usage when needed.

### Access Control Requirements Traces

- [REQ-APIEGR-0110](../requirements/apiegr.md#req-apiegr-0110)
- [REQ-APIEGR-0111](../requirements/apiegr.md#req-apiegr-0111)
- [REQ-APIEGR-0112](../requirements/apiegr.md#req-apiegr-0112)
- [REQ-APIEGR-0113](../requirements/apiegr.md#req-apiegr-0113)
- [REQ-APIEGR-0114](../requirements/apiegr.md#req-apiegr-0114)
- [REQ-APIEGR-0115](../requirements/apiegr.md#req-apiegr-0115)
- [REQ-APIEGR-0116](../requirements/apiegr.md#req-apiegr-0116)
- [REQ-APIEGR-0117](../requirements/apiegr.md#req-apiegr-0117)
- [REQ-APIEGR-0118](../requirements/apiegr.md#req-apiegr-0118)

## Policy and Auditing

- Spec ID: `CYNAI.APIEGR.PolicyAuditing` <a id="spec-cynai-apiegr-policyauditing"></a>

The orchestrator and API Egress Server enforce outbound access policy.

- Policy checks SHOULD include provider allowlists, operation allowlists, and per-task constraints.
- All calls SHOULD be logged with task context, provider, operation, and timing information.
- Responses SHOULD be filtered to avoid accidental secret leakage.

### Policy and Auditing Requirements Traces

- [REQ-APIEGR-0119](../requirements/apiegr.md#req-apiegr-0119)
- [REQ-APIEGR-0120](../requirements/apiegr.md#req-apiegr-0120)

## Sanity Check (Semantic Safety)

- Spec ID: `CYNAI.APIEGR.SanityCheck` <a id="spec-cynai-apiegr-sanitycheck"></a>

The API Egress Server MAY perform an optional semantic sanity check on the requested call before credential resolution and outbound execution.
The sanity checker evaluates whether the call appears safe to execute from a semantic standpoint, complementing access control (identity, provider/operation allowlists, credentials).
It aims to catch dangerous intents that policy alone cannot express.

### Sanity Checker Placement in Request Flow

1. Request received (provider, operation, params, task_id, subject context).
2. Access control evaluation (existing: identity, allow policy, credential selection).
3. **Sanity check (optional):** LLM-based or equivalent semantic evaluation of the intended effect of the call.
4. If sanity check passes or is disabled: resolve credential, perform outbound call, audit, return result.
5. If sanity check denies: deny with structured error, audit the denial.

### Sanity Checker Inputs

- provider, operation, params, task_id; any non-secret context needed for explanation.
- The sanity check MUST NOT receive credentials or decrypted secrets ([REQ-APIEGR-0122](../requirements/apiegr.md#req-apiegr-0122)).

### Sanity Checker Outputs

- One of: allow, deny, or escalate (for human review).
- Optional short reason or category for deny/escalate (for audit and user feedback).

### Escalate to Human Review

When the sanity checker returns **escalate**, the requested outbound call is not executed; the request is held and a human-review event is recorded (task_id, provider, operation, reason/category, timestamp).
The agent receives a structured error indicating that the call was escalated for human review.

Delivery of escalation (and other system-to-user updates) to the user is not yet fully specified.
A draft proposal for default notification connectors (Signal, Discord) that can be enabled and configured to deliver such updates is in [Default notification connectors (draft)](../draft_specs/default_messaging_connectors_proposal.md).
Until that or an equivalent mechanism is adopted, escalation is visible only via audit logs and any review UI the deployment provides; the system does not push notifications to messaging apps or other channels by default.

### Sanity Checker Detection Categories

The implementation SHOULD use an LLM or equivalent semantic evaluation to classify the request against at least: (1) bulk or irreversible deletion without backup/safety, (2) secret or credential exposure (request-side intent), (3) other dangerous or high-impact actions (billing changes, security/access changes at scale, irreversible data loss).
Allowlist-based or rule-based shortcuts for known-safe operations are permitted ([REQ-APIEGR-0121](../requirements/apiegr.md#req-apiegr-0121)).

### Sanity Checker Security

- The system MUST treat the LLM as untrusted and non-authoritative for security; it is an additional signal.
- Access control remains the mandatory gate; the sanity checker is a second layer.

### Sanity Checker Audit

- Log sanity-check result (allow/deny/escalate) and, on deny/escalate, reason/category and task context ([REQ-APIEGR-0123](../requirements/apiegr.md#req-apiegr-0123)).

### Sanity Checker Configuration

- Sanity check SHOULD be configurable per deployment (enable/disable, or per provider/operation) ([REQ-APIEGR-0124](../requirements/apiegr.md#req-apiegr-0124)).
- When disabled, behavior MUST match the flow without a sanity check step.
- Optional allowlist of (provider, operation) pairs that skip the check (e.g. read-only or pre-approved safe operations) to reduce latency and cost.

### Sanity Checker Model Configuration

- The sanity check SHALL use local (worker-hosted) inference by default ([REQ-APIEGR-0125](../requirements/apiegr.md#req-apiegr-0125)).
  It MAY use a configurable external model via API only when the user explicitly configures an external LLM API (OpenAI-compatible or provider-specific endpoint).
- When local inference is not available and no external LLM API is explicitly configured, the sanity checker SHALL be disabled by default ([REQ-APIEGR-0126](../requirements/apiegr.md#req-apiegr-0126)); the sanity check step SHALL NOT be performed.
- Recommended local models for safety classification: Llama Guard 3 (e.g. `llama-guard3:8b`, `llama-guard3:1b`); alternatives for custom prompts: SmolLM2, Llama 3.2.
- When external API is configured: endpoint URL, model identifier, and authentication MUST be configurable; credentials for the external model MUST NOT be exposed to sandboxes.
- The implementation SHOULD define timeout and behavior when the sanity-check call fails or times out (e.g. fail open vs fail closed by policy).

### Sanity Checker Req Traces

- [REQ-APIEGR-0121](../requirements/apiegr.md#req-apiegr-0121)
- [REQ-APIEGR-0122](../requirements/apiegr.md#req-apiegr-0122)
- [REQ-APIEGR-0123](../requirements/apiegr.md#req-apiegr-0123)
- [REQ-APIEGR-0124](../requirements/apiegr.md#req-apiegr-0124)
- [REQ-APIEGR-0125](../requirements/apiegr.md#req-apiegr-0125)
- [REQ-APIEGR-0126](../requirements/apiegr.md#req-apiegr-0126)

## Admin API (Gateway Endpoints)

- Spec ID: `CYNAI.APIEGR.AdminApiGatewayEndpoints` <a id="spec-cynai-apiegr-adminapigatewayendpoints"></a>

The User API Gateway exposes credential management endpoints so that the Web Console and the CLI management app (cynork) can perform the same operations via the same API.
Both clients MUST use these gateway endpoints; the endpoint contract is defined here so that capability parity (REQ-CLIENT-0004) is implementable.

Endpoint contract

- All endpoints MUST live under the gateway's versioned API prefix (e.g. `/v1/`) and MUST require authentication and authorization.
- The gateway MUST audit credential create, rotate, and disable; MUST NOT return secret values in any response; and MUST enforce scope (user/group) so that callers only access credentials they are allowed to manage.

List credentials

- `GET /v1/credentials` (or equivalent under the gateway prefix).
- Query params: optional filter by `provider`, `owner_type`, `owner_id`.
- Response: array of credential metadata only (id, owner_type, owner_id, provider, credential_name, credential_type, is_active, expires_at, created_at, updated_at; no credential_ciphertext or secret).

### Get Credential (Metadata Only)

- `GET /v1/credentials/{id}`.
- Response: single credential metadata; 404 if not found or not authorized.

### Create Credential

- `POST /v1/credentials`.
- Body: provider, credential_name, owner_type, owner_id, credential_type, and secret (write-only; never echoed in response or logs).
- Response: 201 with metadata of created credential (no secret).

### Rotate Credential

- `POST /v1/credentials/{id}/rotate` (or equivalent action).
- Body: new secret only.
- Response: 200 with updated metadata (no secret); 404 if not found or not authorized.

### Disable Credential

- `PATCH /v1/credentials/{id}` with body indicating deactivation (e.g. `is_active: false`), or dedicated `POST /v1/credentials/{id}/disable`.
- Response: 200 with updated metadata; 404 if not found or not authorized.

### Admin API Clients

- The [Web Console](web_console.md) and the [cynork CLI](cynork_cli.md) MUST use the above endpoint contract for API Egress credential operations.
- The gateway SHOULD expose this contract in its OpenAPI/Swagger spec for discovery and for the admin console Swagger UI.

### Admin API Requirements Traces

- [REQ-CLIENT-0116](../requirements/client.md#req-client-0116)
- [REQ-CLIENT-0117](../requirements/client.md#req-client-0117)
- [REQ-CLIENT-0118](../requirements/client.md#req-client-0118)
- [REQ-CLIENT-0119](../requirements/client.md#req-client-0119)
- [REQ-CLIENT-0120](../requirements/client.md#req-client-0120)
