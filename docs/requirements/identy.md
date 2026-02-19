# IDENTY Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `IDENTY` domain.
It covers identity, authentication, and session lifecycle requirements.

## 2 Requirements

- **REQ-IDENTY-0001:** Users in PostgreSQL with stable ids; disable without delete; local auth via gateway; access/refresh tokens; RBAC from groups and bindings; credential storage and audit.
  [CYNAI.IDENTY.IdentityAccountModel](../tech_specs/local_user_accounts.md#spec-cynai-identy-identityaccount)
  [CYNAI.IDENTY.AuthenticationModel](../tech_specs/local_user_accounts.md#spec-cynai-identy-authmodel)
  <a id="req-identy-0001"></a>
- **REQ-IDENTY-0100:** The system MUST store users in PostgreSQL with stable identifiers.
  [CYNAI.IDENTY.IdentityAccountModel](../tech_specs/local_user_accounts.md#spec-cynai-identy-identityaccount)
  <a id="req-identy-0100"></a>
- **REQ-IDENTY-0101:** Users MUST be able to be disabled without deleting records.
  [CYNAI.IDENTY.IdentityAccountModel](../tech_specs/local_user_accounts.md#spec-cynai-identy-identityaccount)
  <a id="req-identy-0101"></a>
- **REQ-IDENTY-0102:** User records MUST support stable external identity mapping for future sync.
  [CYNAI.IDENTY.IdentityAccountModel](../tech_specs/local_user_accounts.md#spec-cynai-identy-identityaccount)
  <a id="req-identy-0102"></a>
- **REQ-IDENTY-0103:** The User API Gateway MUST authenticate user clients using local user accounts in MVP deployments.
  [CYNAI.IDENTY.AuthenticationModel](../tech_specs/local_user_accounts.md#spec-cynai-identy-authmodel)
  <a id="req-identy-0103"></a>
- **REQ-IDENTY-0104:** Authentication MUST issue a short-lived access token and a revocable refresh token.
  [CYNAI.IDENTY.AuthenticationModel](../tech_specs/local_user_accounts.md#spec-cynai-identy-authmodel)
  <a id="req-identy-0104"></a>
- **REQ-IDENTY-0105:** Refresh tokens MUST be rotated on use.
  [CYNAI.IDENTY.AuthenticationModel](../tech_specs/local_user_accounts.md#spec-cynai-identy-authmodel)
  <a id="req-identy-0105"></a>
- **REQ-IDENTY-0106:** Tokens MUST be scoped to a user identity and MUST support revocation.
  [CYNAI.IDENTY.AuthenticationModel](../tech_specs/local_user_accounts.md#spec-cynai-identy-authmodel)
  <a id="req-identy-0106"></a>
- **REQ-IDENTY-0107:** Authorization MUST be evaluated using policy and RBAC derived from group membership and role bindings.
  [CYNAI.IDENTY.AuthorizationRbac](../tech_specs/local_user_accounts.md#spec-cynai-identy-authzrbac)
  <a id="req-identy-0107"></a>
- **REQ-IDENTY-0108:** Authorization decisions MUST be auditable and include resolved group and role context.
  [CYNAI.IDENTY.AuthorizationRbac](../tech_specs/local_user_accounts.md#spec-cynai-identy-authzrbac)
  <a id="req-identy-0108"></a>
- **REQ-IDENTY-0109:** Passwords MUST be stored only as strong password hashes.
  [CYNAI.IDENTY.CredentialStorage](../tech_specs/local_user_accounts.md#spec-cynai-identy-credstorage)
  <a id="req-identy-0109"></a>
- **REQ-IDENTY-0110:** The system MUST NOT store plaintext passwords.
  [CYNAI.IDENTY.CredentialStorage](../tech_specs/local_user_accounts.md#spec-cynai-identy-credstorage)
  <a id="req-identy-0110"></a>
- **REQ-IDENTY-0111:** Password hashing MUST use an algorithm suitable for password storage.
  [CYNAI.IDENTY.CredentialStorage](../tech_specs/local_user_accounts.md#spec-cynai-identy-credstorage)
  <a id="req-identy-0111"></a>
- **REQ-IDENTY-0112:** Password hashes and token hashes MUST be stored in PostgreSQL.
  [CYNAI.IDENTY.CredentialStorage](../tech_specs/local_user_accounts.md#spec-cynai-identy-credstorage)
  <a id="req-identy-0112"></a>
- **REQ-IDENTY-0113:** The orchestrator MUST support bootstrapping an initial admin user for a fresh install.
  [CYNAI.IDENTY.BootstrapAdministration](../tech_specs/local_user_accounts.md#spec-cynai-identy-bootstrapadmin)
  <a id="req-identy-0113"></a>
- **REQ-IDENTY-0114:** User creation SHOULD be admin-gated by default in MVP deployments.
  [CYNAI.IDENTY.BootstrapAdministration](../tech_specs/local_user_accounts.md#spec-cynai-identy-bootstrapadmin)
  <a id="req-identy-0114"></a>
- **REQ-IDENTY-0115:** Password reset SHOULD be admin-initiated in MVP deployments.
  [CYNAI.IDENTY.BootstrapAdministration](../tech_specs/local_user_accounts.md#spec-cynai-identy-bootstrapadmin)
  <a id="req-identy-0115"></a>
- **REQ-IDENTY-0116:** Authentication attempts MUST be rate-limited per user identifier and per IP.
  [CYNAI.IDENTY.AuditAbuseControls](../tech_specs/local_user_accounts.md#spec-cynai-identy-auditabuse)
  <a id="req-identy-0116"></a>
- **REQ-IDENTY-0117:** Repeated failed logins SHOULD trigger temporary lockout or stepped-up delays.
  [CYNAI.IDENTY.AuditAbuseControls](../tech_specs/local_user_accounts.md#spec-cynai-identy-auditabuse)
  <a id="req-identy-0117"></a>
- **REQ-IDENTY-0118:** All authentication events MUST be audit logged.
  [CYNAI.IDENTY.AuditAbuseControls](../tech_specs/local_user_accounts.md#spec-cynai-identy-auditabuse)
  <a id="req-identy-0118"></a>
- **REQ-IDENTY-0119:** Local user account management MUST be exposed via the User API Gateway.
  [CYNAI.IDENTY.UserApiGatewaySurface](../tech_specs/local_user_accounts.md#spec-cynai-identy-userapigateway)
  <a id="req-identy-0119"></a>
- **REQ-IDENTY-0120:** Endpoints MUST enforce authentication, authorization, and auditing.
  [CYNAI.IDENTY.UserApiGatewaySurface](../tech_specs/local_user_accounts.md#spec-cynai-identy-userapigateway)
  <a id="req-identy-0120"></a>
