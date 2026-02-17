# API Egress Server

- [Document Overview](#document-overview)
- [Service Purpose](#service-purpose)
- [Agent Interaction Model](#agent-interaction-model)
- [Credential Storage](#credential-storage)
  - [API Credentials Table](#api-credentials-table)
- [Access Control](#access-control)
- [Policy and Auditing](#policy-and-auditing)

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
- The API Egress credentials table is specified in the [API Egress Credentials](postgres_schema.md#api-egress-credentials) section.

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
