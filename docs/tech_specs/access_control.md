# Access Control

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Core Concepts](#core-concepts)
- [Policy Evaluation](#policy-evaluation)
- [Proposed Tables](#proposed-tables)
- [Service Integration](#service-integration)

## Spec IDs

- Spec ID: `CYNAI.ACCESS.Doc.AccessControl` <a id="spec-cynai-access-doc-accesscontrol"></a>

This section defines stable Spec ID anchors for referencing this document.

## Document Overview

This document defines access control policy for services that provide controlled capabilities to agents.
It is intended to cover both the API Egress Server and the Secure Browser Service.
For how users, groups, roles, and membership feed into subject resolution for policy evaluation, see [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).
The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
See [Access Control](postgres_schema.md#spec-cynai-schema-accesscontrol) and [Audit Logging](postgres_schema.md#spec-cynai-schema-auditlogging).

## Core Concepts

Access control is evaluated for a request using the following information.

- Subject
  - Who is requesting the action.
  - Examples: a user, the Project Manager Agent acting for a user, a task-scoped agent.
- Credential owner
  - Which identity owns the credential used for the action.
  - Examples: user-scoped credential vs group-scoped credential.
- Action
  - What is being attempted.
  - Examples: `api.call`, `web.fetch`, `web.search`, `sandbox_image.publish`, `sandbox_image.use`, `messaging.configure`, `messaging.send`, `mcp.tool.invoke`.
  - Git egress examples: `git.push`, `git.pr.create`.
- Resource
  - What is being accessed.
  - Examples: provider and operation pairs, URL domains, URL patterns, image references and tags, destination ids and destination types.
- Context
  - Additional request metadata used for constraints.
  - Examples: `task_id`, `project_id`, request time, rate-limits, and request size.

Default stance

- The system SHOULD be default deny.
- Allow MUST be explicitly granted via policy.

## Policy Evaluation

The orchestrator and the target service SHOULD both enforce policy.
The orchestrator acts as the first gate, while the service acts as the final gate.

Recommended evaluation order

- Verify request authenticity and subject identity (user context and task context).
- Load effective preferences for the task and user, when applicable.
- Evaluate access control rules for the subject, action, and resource.
- Apply additional constraints from preferences, such as allowlists and maximum response size.
- If any deny rule matches, deny the request.
- If at least one allow rule matches and no deny rule matches, allow the request.
- Otherwise, deny the request.

## Proposed Tables

These tables provide a simple, auditable policy model.
They are a starting point and can be extended later.

### Access Control Rules Table

- `id` (uuid, pk)
- `subject_type` (text)
  - examples: system, user, group, project, task, agent
- `subject_id` (uuid, nullable)
  - null is allowed only for `system` subject_type
- `action` (text)
  - examples: api.call, web.fetch, web.search
- `resource_type` (text)
  - examples: api.provider_operation, web.domain, web.url_pattern
- `resource_pattern` (text)
  - exact match or pattern, depending on resource_type
- `effect` (text)
  - allow|deny
- `priority` (int)
  - higher wins when multiple rules match
- `conditions` (jsonb, nullable)
  - optional constraints such as max_chars, methods, headers, or rate-limits
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Index: (`subject_type`, `subject_id`)
- Index: (`action`)
- Index: (`resource_type`)
- Index: (`priority`)

### Access Control Audit Log Table

- `id` (uuid, pk)
- `subject_type` (text)
- `subject_id` (uuid, nullable)
- `action` (text)
- `resource_type` (text)
- `resource` (text)
  - normalized resolved resource, not the original pattern
- `decision` (text)
  - allow|deny
- `reason` (text)
- `task_id` (uuid, nullable)
- `created_at` (timestamptz)

Constraints

- Index: (`created_at`)
- Index: (`task_id`)

## Service Integration

The following documents define service-specific policy checks.

- API Egress Server: [`docs/tech_specs/api_egress_server.md`](api_egress_server.md)
- Secure Browser Service: [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md)
- User API Gateway: [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md)
