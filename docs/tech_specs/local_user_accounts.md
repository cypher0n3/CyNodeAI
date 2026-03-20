# Local User Accounts

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Threat Model](#threat-model)
- [Identity and Account Model](#identity-and-account-model)
- [Postgres Schema](#postgres-schema)
  - [Users Table](#users-table)
  - [Password Credentials Table](#password-credentials-table)
  - [Refresh Sessions Table](#refresh-sessions-table)
  - [Auth Audit Log Table](#auth-audit-log-table)
- [Authentication Model](#authentication-model)
  - [Per-Request Token Validation](#per-request-token-validation)
- [Authorization and RBAC Integration](#authorization-and-rbac-integration)
- [Credential Storage](#credential-storage)
- [Bootstrap and Administration](#bootstrap-and-administration)
- [Audit and Abuse Controls](#audit-and-abuse-controls)
- [User API Gateway Surface](#user-api-gateway-surface)
- [Future External IdP Integration](#future-external-idp-integration)

## Document Overview

This document defines secure handling of local user accounts for CyNodeAI.
Local user accounts are the MVP default authentication mechanism for self-hosted deployments.

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

#### Traces to Requirements

- [REQ-IDENTY-0100](../requirements/identy.md#req-identy-0100)
- [REQ-IDENTY-0101](../requirements/identy.md#req-identy-0101)
- [REQ-IDENTY-0102](../requirements/identy.md#req-identy-0102)

## Postgres Schema

- Spec ID: `CYNAI.SCHEMA.IdentityAuth` <a id="spec-cynai-schema-identityauth"></a>

The orchestrator stores users and local auth state in PostgreSQL.
Credentials and refresh tokens are stored as hashes.

### Users Table

- Spec ID: `CYNAI.SCHEMA.UsersTable` <a id="spec-cynai-schema-userstable"></a>

- `id` (uuid, pk)
- `handle` (text, unique)
- `email` (text, unique, nullable)
- `is_active` (boolean)
- `external_source` (text, nullable)
- `external_id` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### Users Table Constraints

- Index: (`handle`)
- Index: (`email`) where not null
- Index: (`is_active`)

#### Reserved Identities

- The handle `system` is reserved.
  The orchestrator ensures a corresponding `users` row exists (the "system user") and uses that user id for attribution when an action is performed by the system and no human actor applies (for example `tasks.created_by` for system-created tasks).
  User creation rejects attempts to create or rename a user to `handle=system`.

### Password Credentials Table

- Spec ID: `CYNAI.SCHEMA.PasswordCredentialsTable` <a id="spec-cynai-schema-passwordcredentialstable"></a>

- `id` (uuid, pk)
- `user_id` (uuid, fk to `users.id`)
- `password_hash` (bytea)
- `hash_alg` (text)
  - examples: argon2id, bcrypt
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### Password Credentials Table Constraints

- Unique: (`user_id`) (one password credential per user in MVP)
- Index: (`user_id`)

### Refresh Sessions Table

- Spec ID: `CYNAI.SCHEMA.RefreshSessionsTable` <a id="spec-cynai-schema-refreshsessionstable"></a>

- `id` (uuid, pk)
- `user_id` (uuid, fk to `users.id`)
- `refresh_token_hash` (bytea)
- `refresh_token_kid` (text, nullable)
- `is_active` (boolean)
- `expires_at` (timestamptz)
- `last_used_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### Refresh Sessions Table Constraints

- Index: (`user_id`)
- Index: (`is_active`, `expires_at`)

### Auth Audit Log Table

- Spec ID: `CYNAI.SCHEMA.AuthAuditLogTable` <a id="spec-cynai-schema-authauditlogtable"></a>

Table name: `auth_audit_log`.

Authentication events are audit logged.

Source context: [Audit logging](audit_logging.md#spec-cynai-schema-auditlogging).

- `id` (uuid, pk)
- `event_type` (text)
  - examples: login_success, login_failure, refresh_success, refresh_failure, logout, session_revoked, user_created, user_disabled, user_reenabled, password_changed, password_reset
- `user_id` (uuid, nullable)
  - null for pre-auth failures
- `subject_handle` (text, nullable)
  - redacted or hashed for failure events if needed
- `success` (boolean)
- `ip_address` (inet, nullable)
- `user_agent` (text, nullable)
- `reason` (text, nullable)
- `created_at` (timestamptz)

#### Auth Audit Log Table Constraints

- Index: (`created_at`)
- Index: (`user_id`)
- Index: (`event_type`)

## Authentication Model

The following requirements apply.

### Authentication Model Applicable Requirements

- Spec ID: `CYNAI.IDENTY.AuthenticationModel` <a id="spec-cynai-identy-authmodel"></a>

#### Authentication Model Applicable Requirements Requirements Traces

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

### Per-Request Token Validation

- Spec ID: `CYNAI.IDENTY.PerRequestTokenValidation` <a id="spec-cynai-identy-perrequesttokenvalidation"></a>

#### Per-Request Token Validation Requirements Traces

- [REQ-IDENTY-0122](../requirements/identy.md#req-identy-0122)

Access tokens are validated on every request.
Revoked or expired tokens MUST be rejected; the gateway MUST NOT honor identity from an invalid or expired token.
This aligns with session and refresh-token revocation: once a session or token is revoked, subsequent requests using that token MUST receive an unauthorized response.

## Authorization and RBAC Integration

The following requirements apply.

### Authorization and RBAC Integration Applicable Requirements

- Spec ID: `CYNAI.IDENTY.AuthorizationRbac` <a id="spec-cynai-identy-authzrbac"></a>
- Authentication identifies the user.

#### Authorization and RBAC Integration Applicable Requirements Requirements Traces

- [REQ-IDENTY-0107](../requirements/identy.md#req-identy-0107)
- [REQ-IDENTY-0108](../requirements/identy.md#req-identy-0108)

See [`docs/tech_specs/access_control.md`](access_control.md) and [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).

## Credential Storage

The following requirements apply.

### Credential Storage Applicable Requirements

- Spec ID: `CYNAI.IDENTY.CredentialStorage` <a id="spec-cynai-identy-credstorage"></a>

#### Credential Storage Applicable Requirements Requirements Traces

- [REQ-IDENTY-0109](../requirements/identy.md#req-identy-0109)
- [REQ-IDENTY-0110](../requirements/identy.md#req-identy-0110)
- [REQ-IDENTY-0111](../requirements/identy.md#req-identy-0111)
- [REQ-IDENTY-0112](../requirements/identy.md#req-identy-0112)

### Password Hashing

- Use Argon2id with per-password salt and calibrated parameters.
- Alternatively, use bcrypt with a cost appropriate for the deployment.

### Credential Storage Notes

- Refresh tokens SHOULD be stored as hashes so database exfiltration does not yield usable tokens.
- Access tokens SHOULD be short-lived so revocation lag is bounded.

## Bootstrap and Administration

The following requirements apply.

### Bootstrap and Administration Applicable Requirements

- Spec ID: `CYNAI.IDENTY.BootstrapAdministration` <a id="spec-cynai-identy-bootstrapadmin"></a>

#### Bootstrap and Administration Applicable Requirements Requirements Traces

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

#### Audit and Abuse Controls Applicable Requirements Requirements Traces

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

#### User API Gateway Surface Applicable Requirements Requirements Traces

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
