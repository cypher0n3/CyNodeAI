# ACCESS Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `ACCESS` domain.
It covers authorization, policy evaluation, RBAC, groups, and scope enforcement.

## 2 Requirements

- **REQ-ACCESS-0001:** Default-deny policy; enforcement at gateway and MCP; auditable decisions.
  [CYNAI.ACCESS.Doc.AccessControl](../tech_specs/access_control.md#spec-cynai-access-doc-accesscontrol)
  [CYNAI.MCPGAT.Doc.GatewayEnforcement](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-doc-gatewayenforcement)
  <a id="req-access-0001"></a>
- **REQ-ACCESS-0002:** Groups and membership in PostgreSQL; auditable; role bindings scope-aware; policy uses user and group context.
  [CYNAI.ACCESS.Doc.RBACAndGroups](../tech_specs/rbac_and_groups.md#spec-cynai-access-doc-rbacandgroups)
  [CYNAI.ACCESS.GroupMembership](../tech_specs/rbac_and_groups.md#spec-cynai-access-groupmembership)
  <a id="req-access-0002"></a>

- **REQ-ACCESS-0100:** The orchestrator MUST track groups and group membership in PostgreSQL.
  [CYNAI.ACCESS.GroupMembership](../tech_specs/rbac_and_groups.md#spec-cynai-access-groupmembership)
  <a id="req-access-0100"></a>
- **REQ-ACCESS-0101:** A user MAY belong to zero or more groups.
  [CYNAI.ACCESS.GroupMembership](../tech_specs/rbac_and_groups.md#spec-cynai-access-groupmembership)
  <a id="req-access-0101"></a>
- **REQ-ACCESS-0102:** Group membership MUST be auditable and support enable, disable, and removal.
  [CYNAI.ACCESS.GroupMembership](../tech_specs/rbac_and_groups.md#spec-cynai-access-groupmembership)
  <a id="req-access-0102"></a>
- **REQ-ACCESS-0103:** Group membership changes MUST be recorded with timestamps and an actor identifier.
  [CYNAI.ACCESS.GroupMembership](../tech_specs/rbac_and_groups.md#spec-cynai-access-groupmembership)
  <a id="req-access-0103"></a>
- **REQ-ACCESS-0104:** Group and membership records SHOULD support stable external identity mapping for future directory sync.
  [CYNAI.ACCESS.GroupMembership](../tech_specs/rbac_and_groups.md#spec-cynai-access-groupmembership)
  <a id="req-access-0104"></a>
- **REQ-ACCESS-0105:** The system MUST support role assignments for users and groups.
  [CYNAI.ACCESS.RbacModel](../tech_specs/rbac_and_groups.md#spec-cynai-access-rbacmodel)
  <a id="req-access-0105"></a>
- **REQ-ACCESS-0106:** Role assignments MUST be scope-aware.
  [CYNAI.ACCESS.RbacModel](../tech_specs/rbac_and_groups.md#spec-cynai-access-rbacmodel)
  <a id="req-access-0106"></a>
- **REQ-ACCESS-0107:** Role evaluation MUST be performed at request time using current memberships and bindings.
  [CYNAI.ACCESS.RbacModel](../tech_specs/rbac_and_groups.md#spec-cynai-access-rbacmodel)
  <a id="req-access-0107"></a>
- **REQ-ACCESS-0108:** RBAC decisions MUST be auditable and attributable to the effective subject (user and group context).
  [CYNAI.ACCESS.RbacModel](../tech_specs/rbac_and_groups.md#spec-cynai-access-rbacmodel)
  <a id="req-access-0108"></a>
- **REQ-ACCESS-0109:** Policy evaluation MUST consider both direct user bindings and group-derived bindings.
  [CYNAI.ACCESS.PolicyEvaluation](../tech_specs/rbac_and_groups.md#spec-cynai-access-policyevaluation)
  <a id="req-access-0109"></a>
- **REQ-ACCESS-0110:** The orchestrator MUST resolve effective roles for a request using current memberships and bindings (including group-derived bindings).
  [CYNAI.ACCESS.PolicyEvaluation](../tech_specs/rbac_and_groups.md#spec-cynai-access-policyevaluation)
  <a id="req-access-0110"></a>
- **REQ-ACCESS-0111:** The resolved subject context MUST be included in audit logs.
  [CYNAI.ACCESS.PolicyEvaluation](../tech_specs/rbac_and_groups.md#spec-cynai-access-policyevaluation)
  <a id="req-access-0111"></a>
- **REQ-ACCESS-0112:** The User API Gateway MUST expose groups and membership resources to authorized users.
  [CYNAI.ACCESS.UserApiDataRest](../tech_specs/rbac_and_groups.md#spec-cynai-access-userapirest)
  <a id="req-access-0112"></a>
- **REQ-ACCESS-0113:** Admin operations for groups/RBAC MUST be policy-gated.
  [CYNAI.ACCESS.UserApiDataRest](../tech_specs/rbac_and_groups.md#spec-cynai-access-userapirest)
  <a id="req-access-0113"></a>
- **REQ-ACCESS-0117:** The system SHOULD be default deny.
  [access_control.md](../tech_specs/access_control.md)
  <a id="req-access-0117"></a>
- **REQ-ACCESS-0118:** Allow MUST be explicitly granted via policy.
  [access_control.md](../tech_specs/access_control.md)
  <a id="req-access-0118"></a>
- **REQ-ACCESS-0119:** The orchestrator and the target service SHOULD both enforce policy.
  [access_control.md](../tech_specs/access_control.md)
  <a id="req-access-0119"></a>

- **REQ-ACCESS-0120:** The Data REST API SHOULD expose groups, group memberships, roles, and role bindings to authorized users.
  [CYNAI.ACCESS.UserApiDataRest](../tech_specs/rbac_and_groups.md#spec-cynai-access-userapirest)
  <a id="req-access-0120"></a>

- **REQ-ACCESS-0121:** Vector (similarity) retrieval MUST NOT bypass RBAC.
  Similarity search is only allowed within an already-authorized document set; authorization MUST be applied before similarity ranking.
  [CYNAI.SCHEMA.VectorRetrievalRbac](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorretrievalrbac)
  <a id="req-access-0121"></a>
- **REQ-ACCESS-0122:** A user MUST NOT retrieve vector-backed content from a project they are not authorized to access.
  The system MUST filter by authorized project (and namespace when applicable) in SQL before applying similarity scoring.
  [CYNAI.SCHEMA.VectorRetrievalRbac](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorretrievalrbac)
  <a id="req-access-0122"></a>
- **REQ-ACCESS-0123:** When vector rows have a sensitivity level, a lower-privilege role MUST NOT retrieve rows with a higher sensitivity level than the role allows.
  The query MUST enforce sensitivity_level <= role.max_sensitivity_level within the authorized scope.
  [CYNAI.SCHEMA.VectorRetrievalRbac](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorretrievalrbac)
  <a id="req-access-0123"></a>
- **REQ-ACCESS-0124:** Every vector retrieval MUST be auditable (e.g. user_id, scope, namespaces queried, chunk count returned, timestamp).
  [CYNAI.SCHEMA.VectorRetrievalRbac](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorretrievalrbac)
  <a id="req-access-0124"></a>
- **REQ-ACCESS-0125:** Embedding ingestion MUST require write permission on the target scope (project and namespace) and correct project association; only controlled services MAY insert embeddings.
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  [CYNAI.SCHEMA.VectorRetrievalRbac](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorretrievalrbac)
  <a id="req-access-0125"></a>
