# Access Control

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Core Concepts](#core-concepts)
- [Project Plan Actions](#project-plan-actions)
  - [`ProjectPlanActions` Scope](#projectplanactions-scope)
  - [`ProjectPlanActions` Contract](#projectplanactions-contract)
- [Policy Evaluation](#policy-evaluation)
  - [Recommended Evaluation Order](#recommended-evaluation-order)
- [Postgres Schema](#postgres-schema)
  - [Access Control Rules Table](#access-control-rules-table)
  - [Access Control Audit Log Table](#access-control-audit-log-table)
- [Service Integration](#service-integration)

## Spec IDs

- Spec ID: `CYNAI.ACCESS.Doc.AccessControl` <a id="spec-cynai-access-doc-accesscontrol"></a>
- [CYNAI.ACCESS.ProjectPlanActions](#spec-cynai-access-projectplanactions)

This section defines stable Spec ID anchors for referencing this document.

## Document Overview

This document defines access control policy for services that provide controlled capabilities to agents.
It is intended to cover both the API Egress Server and the Secure Browser Service.
For how users, groups, roles, and membership feed into subject resolution for policy evaluation, see [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).

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
  - Project plan actions are defined in [Project plan actions](#spec-cynai-access-projectplanactions).
- Resource
  - What is being accessed.
  - Examples: provider and operation pairs, URL domains, URL patterns, image references and tags, destination ids and destination types.
- Context
  - Additional request metadata used for constraints.
  - Examples: `task_id`, `project_id`, request time, rate-limits, and request size.

Default stance

- The system SHOULD be default deny.
- Allow MUST be explicitly granted via policy.

## Project Plan Actions

- Spec ID: `CYNAI.ACCESS.ProjectPlanActions` <a id="spec-cynai-access-projectplanactions"></a>

The following action strings are the canonical names for project-plan operations in access control rules and audit logs.
Implementations MUST use these exact strings when evaluating or recording policy for project plan resources.

### `ProjectPlanActions` Scope

- Applies to the User API Gateway and any service that enforces project plan operations.
- Resource type for project plan operations: `project_plan`.
- Resource pattern: project identifier (e.g. `projects/<uuid>` or project slug) so that rules can scope allow/deny per project.

### `ProjectPlanActions` Contract

- **`project_plan.read`**
  - Meaning: Read plan document (name, body) and task list; list plan revisions.
  - When evaluated: GET project plan, GET plan revisions.
- **`project_plan.update`**
  - Meaning: Update plan document (name, body), task list, or task dependencies (when plan not locked).
  - When evaluated: PUT/PATCH plan, task list updates, or task dependency updates.
- **`project_plan.lock`**
  - Meaning: Lock the plan document so it is not editable until unlocked.
  - When evaluated: Lock operation.
- **`project_plan.unlock`**
  - Meaning: Unlock the plan document.
  - When evaluated: Unlock operation.
- **`project_plan.approve`**
  - Meaning: Approve plan (set state to ready).
  - When evaluated: Approve operation.
- **`project_plan.activate`**
  - Meaning: Activate plan (set state from ready to active so workflow may run).
  - When evaluated: Activate operation.
- **`project_plan.archive`**
  - Meaning: Set or clear the archived flag on a plan (for UI/API views); archived plans must not run and must not be active.
  - When evaluated: Archive / unarchive operation.

#### Rules for Action Evaluation

- The orchestrator (or gateway) MUST evaluate the above action for the authenticated subject and resource (project) before permitting the operation.
- Deny takes precedence when both allow and deny rules match.
- Default deny: if no rule matches, the operation MUST be denied.

## Policy Evaluation

The orchestrator and the target service SHOULD both enforce policy.
The orchestrator acts as the first gate, while the service acts as the final gate.

### Recommended Evaluation Order

- Verify request authenticity and subject identity (user context and task context).
- Load effective preferences for the task and user, when applicable.
- Evaluate access control rules for the subject, action, and resource.
- Apply additional constraints from preferences, such as allowlists and maximum response size.
- If any deny rule matches, deny the request.
- If at least one allow rule matches and no deny rule matches, allow the request.
- Otherwise, deny the request.

## Postgres Schema

- Spec ID: `CYNAI.SCHEMA.AccessControl` <a id="spec-cynai-schema-accesscontrol"></a>

Policy rules and access control audit log.
Used by API Egress, Secure Browser, and other policy-enforcing services.

### Access Control Rules Table

- Spec ID: `CYNAI.SCHEMA.AccessControlRulesTable` <a id="spec-cynai-schema-accesscontrolrulestable"></a>

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
  - allow or deny
- `priority` (int)
  - higher wins when multiple rules match
- `conditions` (jsonb, nullable)
  - optional constraints such as max_chars, methods, headers, or rate-limits
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

#### Access Control Rules Table Constraints

- Index: (`subject_type`, `subject_id`)
- Index: (`action`)
- Index: (`resource_type`)
- Index: (`priority`)

### Access Control Audit Log Table

- Spec ID: `CYNAI.SCHEMA.AccessControlAuditLogTable` <a id="spec-cynai-schema-accesscontrolauditlogtable"></a>

- `id` (uuid, pk)
- `subject_type` (text)
- `subject_id` (uuid, nullable)
- `action` (text)
- `resource_type` (text)
- `resource` (text)
  - normalized resolved resource, not the original pattern
- `decision` (text)
  - allow or deny
- `reason` (text, nullable)
- `task_id` (uuid, nullable)
- `created_at` (timestamptz)

#### Access Control Audit Log Table Constraints

- Index: (`created_at`)
- Index: (`task_id`)

## Service Integration

The following documents define service-specific policy checks.

- API Egress Server: [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).
  The API Egress Server may apply an additional semantic sanity check after policy evaluation (see [Sanity Check (Semantic Safety)](api_egress_server.md#spec-cynai-apiegr-sanitycheck)); policy remains the authoritative allow/deny for identity and resource, and the sanity check is a separate safety layer.
- Secure Browser Service: [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md)
- User API Gateway: [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md)
