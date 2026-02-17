# APIEGR Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `APIEGR` domain.
It covers controlled egress services and their policy and auditing constraints.

## 2 Requirements

- REQ-APIEGR-0001: Controlled external API calls; policy and audit at egress.
  [CYNAI.APIEGR.Doc.ApiEgressServer](../tech_specs/api_egress_server.md#spec-cynai-apiegr-doc-apiegressserver)
  <a id="req-apiegr-0001"></a>
- REQ-APIEGR-0002: No Git credentials in sandboxes; Git ops via egress service; orchestrator as policy point; changeset artifacts tied to task_id and credential-free.
  [CYNAI.APIEGR.GitEgressArchitecture](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressarch)
  [CYNAI.APIEGR.GitEgressSandboxOutput](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressout)
  <a id="req-apiegr-0002"></a>

- REQ-APIEGR-0100: Sandboxes MUST NOT store Git credentials.
  [CYNAI.APIEGR.GitEgressArchitecture](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressarch)
  <a id="req-apiegr-0100"></a>
- REQ-APIEGR-0101: Sandboxes MUST NOT make arbitrary outbound network calls to Git hosts.
  [CYNAI.APIEGR.GitEgressArchitecture](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressarch)
  <a id="req-apiegr-0101"></a>
- REQ-APIEGR-0102: Git operations that require remote access MUST be performed by the Git egress service.
  [CYNAI.APIEGR.GitEgressArchitecture](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressarch)
  <a id="req-apiegr-0102"></a>
- REQ-APIEGR-0103: The orchestrator SHOULD act as the policy and routing point for Git egress operations.
  [CYNAI.APIEGR.GitEgressArchitecture](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressarch)
  <a id="req-apiegr-0103"></a>
- REQ-APIEGR-0104: A changeset artifact MUST be associated with a single `task_id`.
  [CYNAI.APIEGR.GitEgressSandboxOutput](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressout)
  <a id="req-apiegr-0104"></a>
- REQ-APIEGR-0105: A changeset artifact MUST NOT include credentials.
  [CYNAI.APIEGR.GitEgressSandboxOutput](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressout)
  <a id="req-apiegr-0105"></a>

- REQ-APIEGR-0106: Agents MUST never receive credentials in responses.
  [CYNAI.APIEGR.CredentialStorage](../tech_specs/api_egress_server.md#spec-cynai-apiegr-credentialstorage)
  <a id="req-apiegr-0106"></a>
- REQ-APIEGR-0107: Encryption SHOULD be envelope encryption with a master key that is not stored in PostgreSQL.
  [CYNAI.APIEGR.CredentialStorage](../tech_specs/api_egress_server.md#spec-cynai-apiegr-credentialstorage)
  <a id="req-apiegr-0107"></a>
- REQ-APIEGR-0108: The API Egress Server SHOULD be the only service with permission to decrypt credentials.
  [CYNAI.APIEGR.CredentialStorage](../tech_specs/api_egress_server.md#spec-cynai-apiegr-credentialstorage)
  <a id="req-apiegr-0108"></a>
- REQ-APIEGR-0109: Credential rotation SHOULD be supported without changing agent behavior.
  [CYNAI.APIEGR.CredentialStorage](../tech_specs/api_egress_server.md#spec-cynai-apiegr-credentialstorage)
  <a id="req-apiegr-0109"></a>
- REQ-APIEGR-0110: The API Egress Server MUST enforce access control for outbound API calls.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0110"></a>
- REQ-APIEGR-0111: Subject identity MUST be resolved to a user context.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0111"></a>
- REQ-APIEGR-0112: The requested `provider` and `operation` MUST be validated against allow policy for that subject.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0112"></a>
- REQ-APIEGR-0113: The chosen credential MUST be authorized for the request context and MUST be active.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0113"></a>
- REQ-APIEGR-0114: The service SHOULD apply per-user and per-task constraints, such as rate limits and allowed operations.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0114"></a>
- REQ-APIEGR-0115: The API Egress Server SHOULD support group-scoped credentials for shared enterprise integrations.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0115"></a>
- REQ-APIEGR-0116: A group-scoped credential MUST be selectable only when the task context includes a group identity and policy allows group usage.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0116"></a>
- REQ-APIEGR-0117: Access control rules SHOULD distinguish between user-scoped and group-scoped usage when needed.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0117"></a>
- REQ-APIEGR-0118: Policy checks SHOULD include provider allowlists, operation allowlists, and per-task constraints.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0118"></a>
- REQ-APIEGR-0119: All calls SHOULD be logged with task context, provider, operation, and timing information.
  [CYNAI.APIEGR.PolicyAuditing](../tech_specs/api_egress_server.md#spec-cynai-apiegr-policyauditing)
  <a id="req-apiegr-0119"></a>
- REQ-APIEGR-0120: Responses SHOULD be filtered to avoid accidental secret leakage.
  [CYNAI.APIEGR.PolicyAuditing](../tech_specs/api_egress_server.md#spec-cynai-apiegr-policyauditing)
  <a id="req-apiegr-0120"></a>
