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

Normative requirements

- The system MUST store users in PostgreSQL with stable identifiers.
- Users MUST be able to be disabled without deleting records.
- User records MUST support stable external identity mapping for future sync.

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

Normative requirements

- The User API Gateway MUST authenticate user clients using local user accounts in MVP deployments.
- Authentication MUST issue a short-lived access token and a revocable refresh token.
- Refresh tokens MUST be rotated on use.
- Tokens MUST be scoped to a user identity and MUST support revocation.

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

Normative requirements

- Authentication identifies the user.
- Authorization MUST be evaluated using policy and RBAC derived from group membership and role bindings.
- Authorization decisions MUST be auditable and include resolved group and role context.

See [`docs/tech_specs/access_control.md`](access_control.md) and [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).

## Credential Storage

Normative requirements

- Passwords MUST be stored only as strong password hashes.
- The system MUST NOT store plaintext passwords.
- Password hashing MUST use an algorithm suitable for password storage.
- Password hashes and token hashes MUST be stored in PostgreSQL.

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

Normative requirements

- The orchestrator MUST support bootstrapping an initial admin user for a fresh install.
- User creation SHOULD be admin-gated by default in MVP deployments.
- Password reset SHOULD be admin-initiated in MVP deployments.

Recommended bootstrap mechanisms

- A one-time bootstrap token printed at startup.
- A local-only bootstrap endpoint bound to localhost.
- A bootstrap file referenced by orchestrator startup configuration.

## Audit and Abuse Controls

Normative requirements

- Authentication attempts MUST be rate-limited per user identifier and per IP.
- Repeated failed logins SHOULD trigger temporary lockout or stepped-up delays.
- All authentication events MUST be audit logged.

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

Normative requirements

- Local user account management MUST be exposed via the User API Gateway.
- Endpoints MUST enforce authentication, authorization, and auditing.

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
