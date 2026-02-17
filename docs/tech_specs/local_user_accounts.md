# Local User Accounts

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Threat Model](#threat-model)
- [Identity and Account Model](#identity-and-account-model)
- [Authentication Model](#authentication-model)
- [Authorization and RBAC Integration](#authorization-and-rbac-integration)
- [Credential Storage](#credential-storage)
- [Bootstrap and Administration](#bootstrap-and-administration)
- [Audit and Abuse Controls](#audit-and-abuse-controls)
- [User API Gateway Surface](#user-api-gateway-surface)
- [Future External IdP Integration](#future-external-idp-integration)

## Document Overview

This document defines secure handling of local user accounts for CyNodeAI.
Local user accounts are the MVP default authentication mechanism for self-hosted deployments.
The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
See [Identity and Authentication](postgres_schema.md#identity-and-authentication) and [Audit Logging](postgres_schema.md#audit-logging).

## Goals

- Provide a secure local authentication system for user clients of the User API Gateway.
- Store credentials securely and never expose them to agents or worker sandboxes.
- Support session issuance, rotation, and revocation for client access.
- Integrate with groups and RBAC for authorization decisions.

## Threat Model

Assumptions

- The orchestrator database may be exfiltrated.
- Clients may be malicious or compromised.
- Brute force attempts and credential stuffing are expected on exposed gateways.

Non-goals

- This document does not specify SSO as the MVP default.
- External IdP integration is future work.

## Identity and Account Model

The following requirements apply.

### Identity and Account Model Applicable Requirements

- Spec ID: `CYNAI.IDENTY.IdentityAccountModel` <a id="spec-cynai-identy-identityaccount"></a>

Traces To:

- [REQ-IDENTY-0100](../requirements/identy.md#req-identy-0100)
- [REQ-IDENTY-0101](../requirements/identy.md#req-identy-0101)
- [REQ-IDENTY-0102](../requirements/identy.md#req-identy-0102)

Recommended users table

- `id` (uuid, pk)
- `handle` (text, unique)
- `email` (text, unique, nullable)
- `is_active` (boolean)
- `external_source` (text, nullable)
- `external_id` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

## Authentication Model

The following requirements apply.

### Authentication Model Applicable Requirements

- Spec ID: `CYNAI.IDENTY.AuthenticationModel` <a id="spec-cynai-identy-authmodel"></a>

Traces To:

- [REQ-IDENTY-0103](../requirements/identy.md#req-identy-0103)
- [REQ-IDENTY-0104](../requirements/identy.md#req-identy-0104)
- [REQ-IDENTY-0105](../requirements/identy.md#req-identy-0105)
- [REQ-IDENTY-0106](../requirements/identy.md#req-identy-0106)

Recommended token model

- Access token
  - short-lived bearer token (e.g. JWT)
  - used for normal API calls
- Refresh token
  - long-lived, revocable
  - rotated on every successful refresh
  - stored by clients in a secure store

Logout and revocation

- Users MUST be able to revoke refresh tokens.
- Admins MUST be able to revoke all active sessions for a user.

## Authorization and RBAC Integration

The following requirements apply.

### Authorization and RBAC Integration Applicable Requirements

- Spec ID: `CYNAI.IDENTY.AuthorizationRbac` <a id="spec-cynai-identy-authzrbac"></a>
- Authentication identifies the user.

Traces To:

- [REQ-IDENTY-0107](../requirements/identy.md#req-identy-0107)
- [REQ-IDENTY-0108](../requirements/identy.md#req-identy-0108)

See [`docs/tech_specs/access_control.md`](access_control.md) and [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).

## Credential Storage

The following requirements apply.

### Credential Storage Applicable Requirements

- Spec ID: `CYNAI.IDENTY.CredentialStorage` <a id="spec-cynai-identy-credstorage"></a>

Traces To:

- [REQ-IDENTY-0109](../requirements/identy.md#req-identy-0109)
- [REQ-IDENTY-0110](../requirements/identy.md#req-identy-0110)
- [REQ-IDENTY-0111](../requirements/identy.md#req-identy-0111)
- [REQ-IDENTY-0112](../requirements/identy.md#req-identy-0112)

Recommended password hashing

- Use Argon2id with per-password salt and calibrated parameters.
- Alternatively, use bcrypt with a cost appropriate for the deployment.

Recommended credential tables

### Password Credentials Table

- `id` (uuid, pk)
- `user_id` (uuid)
  - foreign key to `users.id`
- `password_hash` (bytea)
- `hash_alg` (text)
  - examples: argon2id, bcrypt
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

### Refresh Sessions Table

- `id` (uuid, pk)
- `user_id` (uuid)
  - foreign key to `users.id`
- `refresh_token_hash` (bytea)
- `refresh_token_kid` (text, nullable)
  - optional key id if using a pepper or envelope scheme
- `is_active` (boolean)
- `expires_at` (timestamptz)
- `last_used_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Notes

- Refresh tokens SHOULD be stored as hashes so database exfiltration does not yield usable tokens.
- Access tokens SHOULD be short-lived so revocation lag is bounded.

## Bootstrap and Administration

The following requirements apply.

### Bootstrap and Administration Applicable Requirements

- Spec ID: `CYNAI.IDENTY.BootstrapAdministration` <a id="spec-cynai-identy-bootstrapadmin"></a>

Traces To:

- [REQ-IDENTY-0113](../requirements/identy.md#req-identy-0113)
- [REQ-IDENTY-0114](../requirements/identy.md#req-identy-0114)
- [REQ-IDENTY-0115](../requirements/identy.md#req-identy-0115)

Recommended bootstrap mechanisms

- A one-time bootstrap token printed at startup.
- A local-only bootstrap endpoint bound to localhost.
- A bootstrap file referenced by orchestrator startup configuration.

## Audit and Abuse Controls

The following requirements apply.

### Audit and Abuse Controls Applicable Requirements

- Spec ID: `CYNAI.IDENTY.AuditAbuseControls` <a id="spec-cynai-identy-auditabuse"></a>

Traces To:

- [REQ-IDENTY-0116](../requirements/identy.md#req-identy-0116)
- [REQ-IDENTY-0117](../requirements/identy.md#req-identy-0117)
- [REQ-IDENTY-0118](../requirements/identy.md#req-identy-0118)

Recommended audit events

- login success
- login failure (redacted)
- refresh success
- refresh failure
- logout
- session revoked (user or admin)
- user created, disabled, re-enabled
- password changed, password reset

## User API Gateway Surface

The following requirements apply.

### User API Gateway Surface Applicable Requirements

- Spec ID: `CYNAI.IDENTY.UserApiGatewaySurface` <a id="spec-cynai-identy-userapigateway"></a>

Traces To:

- [REQ-IDENTY-0119](../requirements/identy.md#req-identy-0119)
- [REQ-IDENTY-0120](../requirements/identy.md#req-identy-0120)

Minimum MVP endpoints

- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /users/me`
- Admin-gated
  - `POST /users`
  - `POST /users/{id}/disable`
  - `POST /users/{id}/enable`
  - `POST /users/{id}/revoke_sessions`
  - `POST /users/{id}/reset_password`

## Future External IdP Integration

CyNodeAI MAY integrate with an external IdP for authentication in the future.
If integrated, local accounts MAY remain as a break-glass mechanism for operators.
External IdP integration SHOULD reuse the same `users` table with external identity mapping fields.

See the external group service note in [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).
