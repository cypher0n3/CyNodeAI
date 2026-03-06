# APIEGR Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `APIEGR` domain.
It covers controlled egress services and their policy and auditing constraints.

## 2 Requirements

- **REQ-APIEGR-0001:** Controlled external API calls; policy and audit at egress.
  [CYNAI.APIEGR.Doc.ApiEgressServer](../tech_specs/api_egress_server.md#spec-cynai-apiegr-doc-apiegressserver)
  <a id="req-apiegr-0001"></a>
- **REQ-APIEGR-0002:** No Git credentials in sandboxes; Git ops via egress service; orchestrator as policy point; changeset artifacts tied to task_id and credential-free.
  [CYNAI.APIEGR.GitEgressArchitecture](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressarch)
  [CYNAI.APIEGR.GitEgressSandboxOutput](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressout)
  <a id="req-apiegr-0002"></a>

- **REQ-APIEGR-0100:** Sandboxes MUST NOT store Git credentials.
  [CYNAI.APIEGR.GitEgressArchitecture](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressarch)
  <a id="req-apiegr-0100"></a>
- **REQ-APIEGR-0101:** Sandboxes MUST NOT make arbitrary outbound network calls to Git hosts.
  This includes remote-affecting Git operations such as `git clone`, `git fetch`, `git pull`, `git push`, submodule fetch/update, and Git LFS downloads.
  [CYNAI.APIEGR.GitEgressArchitecture](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressarch)
  <a id="req-apiegr-0101"></a>
- **REQ-APIEGR-0102:** Git operations that require remote access MUST be performed by the Git egress service.
  Sandboxes MAY run local-only Git commands (for example `git status`, `git diff`, `git commit`) on the mounted workspace, but they MUST NOT contact Git remotes directly.
  [CYNAI.APIEGR.GitEgressArchitecture](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressarch)
  <a id="req-apiegr-0102"></a>
- **REQ-APIEGR-0103:** The orchestrator SHOULD act as the policy and routing point for Git egress operations.
  [CYNAI.APIEGR.GitEgressArchitecture](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressarch)
  <a id="req-apiegr-0103"></a>
- **REQ-APIEGR-0104:** A changeset artifact MUST be associated with a single `task_id`.
  [CYNAI.APIEGR.GitEgressSandboxOutput](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressout)
  <a id="req-apiegr-0104"></a>
- **REQ-APIEGR-0105:** A changeset artifact MUST NOT include credentials.
  [CYNAI.APIEGR.GitEgressSandboxOutput](../tech_specs/git_egress_mcp.md#spec-cynai-apiegr-gitegressout)
  <a id="req-apiegr-0105"></a>

- **REQ-APIEGR-0106:** Agents MUST never receive credentials in responses.
  [CYNAI.APIEGR.CredentialStorage](../tech_specs/api_egress_server.md#spec-cynai-apiegr-credentialstorage)
  <a id="req-apiegr-0106"></a>
- **REQ-APIEGR-0107:** Encryption SHOULD be envelope encryption with a master key that is not stored in PostgreSQL.
  [CYNAI.APIEGR.CredentialStorage](../tech_specs/api_egress_server.md#spec-cynai-apiegr-credentialstorage)
  <a id="req-apiegr-0107"></a>
- **REQ-APIEGR-0108:** The API Egress Server SHOULD be the only service with permission to decrypt credentials.
  [CYNAI.APIEGR.CredentialStorage](../tech_specs/api_egress_server.md#spec-cynai-apiegr-credentialstorage)
  [REQ-STANDS-0133](../requirements/stands.md#req-stands-0133) (Go code that decrypts or holds credentials)
  <a id="req-apiegr-0108"></a>
- **REQ-APIEGR-0109:** Credential rotation SHOULD be supported without changing agent behavior.
  [CYNAI.APIEGR.CredentialStorage](../tech_specs/api_egress_server.md#spec-cynai-apiegr-credentialstorage)
  <a id="req-apiegr-0109"></a>
- **REQ-APIEGR-0110:** The API Egress Server MUST enforce access control for outbound API calls.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0110"></a>
- **REQ-APIEGR-0111:** Subject identity MUST be resolved to a user context.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0111"></a>
- **REQ-APIEGR-0112:** The requested `provider` and `operation` MUST be validated against allow policy for that subject.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0112"></a>
- **REQ-APIEGR-0113:** The chosen credential MUST be authorized for the request context and MUST be active.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0113"></a>
- **REQ-APIEGR-0114:** The service SHOULD apply per-user and per-task constraints, such as rate limits and allowed operations.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0114"></a>
- **REQ-APIEGR-0115:** The API Egress Server SHOULD support group-scoped credentials for shared enterprise integrations.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0115"></a>
- **REQ-APIEGR-0116:** A group-scoped credential MUST be selectable only when the task context includes a group identity and policy allows group usage.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0116"></a>
- **REQ-APIEGR-0117:** Access control rules SHOULD distinguish between user-scoped and group-scoped usage when needed.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0117"></a>
- **REQ-APIEGR-0118:** Policy checks SHOULD include provider allowlists, operation allowlists, and per-task constraints.
  [CYNAI.APIEGR.AccessControl](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-apiegr-0118"></a>
- **REQ-APIEGR-0119:** All calls SHOULD be logged with task context, provider, operation, and timing information.
  [CYNAI.APIEGR.PolicyAuditing](../tech_specs/api_egress_server.md#spec-cynai-apiegr-policyauditing)
  <a id="req-apiegr-0119"></a>
- **REQ-APIEGR-0120:** Responses SHOULD be filtered to avoid accidental secret leakage.
  [CYNAI.APIEGR.PolicyAuditing](../tech_specs/api_egress_server.md#spec-cynai-apiegr-policyauditing)
  <a id="req-apiegr-0120"></a>
- **REQ-APIEGR-0121:** The API Egress Server MAY perform a semantic sanity check on the requested call (provider, operation, params) before execution.
  When enabled, the sanity check MUST evaluate whether the call appears to involve bulk/irreversible deletion without backup, secret exposure, or other dangerous or high-impact actions.
  [CYNAI.APIEGR.SanityCheck](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck)
  <a id="req-apiegr-0121"></a>
- **REQ-APIEGR-0122:** The sanity check MUST NOT receive or use decrypted credentials; it SHALL use only the request payload and non-secret context.
  [CYNAI.APIEGR.SanityCheck](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck)
  <a id="req-apiegr-0122"></a>
- **REQ-APIEGR-0123:** When the sanity check denies a call, the server MUST deny the request with a structured error and MUST log the denial with task context and reason/category.
  [CYNAI.APIEGR.SanityCheck](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck)
  <a id="req-apiegr-0123"></a>
- **REQ-APIEGR-0124:** Sanity check behavior SHOULD be configurable (e.g. enable/disable, or allowlist of operations that skip the check).
  [CYNAI.APIEGR.SanityCheck](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck)
  <a id="req-apiegr-0124"></a>
- **REQ-APIEGR-0125:** The sanity check SHALL use local (worker-hosted) inference by default.
  It MAY use a configurable external model via API only when explicitly configured by the user (e.g. OpenAI-compatible or provider-specific endpoint).
  When external API is configured, endpoint URL, model identifier, and authentication MUST be configurable; credentials for the external model MUST NOT be exposed to sandboxes.
  [CYNAI.APIEGR.SanityCheck](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck)
  <a id="req-apiegr-0125"></a>
- **REQ-APIEGR-0126:** When local (worker-hosted) inference is not available and the user has not explicitly configured an external LLM API for the sanity check, the sanity checker SHALL be disabled by default (the sanity check step SHALL NOT be performed).
  [CYNAI.APIEGR.SanityCheck](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck)
  <a id="req-apiegr-0126"></a>
- **REQ-APIEGR-0127:** When a Git egress request includes a task that has a non-null project_id, the Git egress service MUST validate that the requested provider and repo are associated with that project (project-scoped repo allowlist); if not associated, the request MUST be denied.
  [CYNAI.APIEGR.GitEgressProjectScope](../tech_specs/project_git_repos.md#spec-cynai-apiegr-gitegressprojectscope)
  <a id="req-apiegr-0127"></a>
