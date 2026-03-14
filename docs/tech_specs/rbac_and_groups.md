# RBAC and Groups

- [Document Overview](#document-overview)
- [Spec IDs](#spec-ids)
- [Goals](#goals)
- [Core Concepts](#core-concepts)
- [Group Membership Model](#group-membership-model)
- [RBAC Model](#rbac-model)
- [Policy Evaluation Integration](#policy-evaluation-integration)
- [User API and Data REST Resources](#user-api-and-data-rest-resources)
- [Future Considerations: External Group Service Integration](#future-considerations-external-group-service-integration)

## Spec IDs

- Spec ID: `CYNAI.ACCESS.Doc.RBACAndGroups` <a id="spec-cynai-access-doc-rbacandgroups"></a>

This section defines stable Spec ID anchors for referencing this document.

## Document Overview

This document defines CyNodeAI role-based access control (RBAC) and how group membership is tracked for users.
It complements the policy model in [`docs/tech_specs/access_control.md`](access_control.md) by defining subjects, roles, and membership resolution.
The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
See [Groups and RBAC](postgres_schema.md#spec-cynai-schema-groupsrbac).

## Goals

- Track group membership for users in PostgreSQL.
- Provide a role model that can be applied to groups, users, and scoped resources (system, project).
- Integrate RBAC-derived subjects into access control policy evaluation and auditing.
- Expose group and RBAC state via the User API Gateway and Data REST API.

## Core Concepts

Terminology

- **User**: An authenticated human identity.
- **Group**: A set of users.
- **Membership**: A user belongs to a group with an optional role assignment.
- **Role**: A named set of permissions.
- **Role binding**: Assigns a role to a subject (user or group) within a scope.

Scopes

- **System scope**: Applies to the entire orchestrator.
- **Project scope**: Applies to a project or workspace boundary, when used.

## Group Membership Model

The following requirements apply.

### Group Membership Model Applicable Requirements

- Spec ID: `CYNAI.ACCESS.GroupMembership` <a id="spec-cynai-access-groupmembership"></a>

#### Traces to Requirements

- [REQ-ACCESS-0100](../requirements/access.md#req-access-0100)
- [REQ-ACCESS-0101](../requirements/access.md#req-access-0101)
- [REQ-ACCESS-0102](../requirements/access.md#req-access-0102)
- [REQ-ACCESS-0103](../requirements/access.md#req-access-0103)
- [REQ-ACCESS-0104](../requirements/access.md#req-access-0104)

Recommended tables

### Groups Table

- `id` (uuid, pk)
- `slug` (text, unique)
- `display_name` (text)
- `is_active` (boolean)
- `external_source` (text, nullable)
  - examples: entra_id, google_workspace, scim
- `external_id` (text, nullable)
  - stable identifier from the external system
- `managed_by` (text, optional)
  - examples: local, external_sync
- `last_synced_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

### Group Memberships Table

- `id` (uuid, pk)
- `group_id` (uuid)
  - foreign key to `groups.id`
- `user_id` (uuid)
  - foreign key to `users.id`
- `is_active` (boolean)
- `external_source` (text, nullable)
- `external_id` (text, nullable)
- `managed_by` (text, optional)
  - examples: local, external_sync
- `last_synced_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`group_id`, `user_id`)
- Index: (`group_id`)
- Index: (`user_id`)
- Index: (`is_active`)

## RBAC Model

The following requirements apply.

### RBAC Model Applicable Requirements

- Spec ID: `CYNAI.ACCESS.RbacModel` <a id="spec-cynai-access-rbacmodel"></a>

#### RBAC Model Applicable Requirements Requirements Traces

- [REQ-ACCESS-0105](../requirements/access.md#req-access-0105)
- [REQ-ACCESS-0106](../requirements/access.md#req-access-0106)
- [REQ-ACCESS-0107](../requirements/access.md#req-access-0107)
- [REQ-ACCESS-0108](../requirements/access.md#req-access-0108)

Recommended role structure

### Roles Table

- `id` (uuid, pk)
- `name` (text, unique)
  - examples: owner, admin, operator, member, viewer
- `description` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

### Role Bindings Table

- `id` (uuid, pk)
- `subject_type` (text)
  - one of: user|group
- `subject_id` (uuid)
- `role_id` (uuid)
  - foreign key to `roles.id`
- `scope_type` (text)
  - one of: system|project
- `scope_id` (uuid, nullable)
  - null allowed only for system scope
- `is_active` (boolean)
- `managed_by` (text, optional)
  - examples: local, external_sync
- `last_synced_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Index: (`subject_type`, `subject_id`)
- Index: (`role_id`)
- Index: (`scope_type`, `scope_id`)
- Index: (`is_active`)

Permission mapping

- Roles SHOULD be mapped to allowed actions for services and tools.
- The mapping MAY be implemented as derived policy rules in `access_control_rules`.
- The system SHOULD support an operator-managed default role map.

### Project Plan Lock RBAC

- Spec ID: `CYNAI.ACCESS.ProjectPlanLockRbac` <a id="spec-cynai-access-projectplanlockrbac"></a>

#### Project Plan Lock RBAC Requirements Traces

- [REQ-PROJCT-0116](../requirements/projct.md#req-projct-0116)

RBAC MUST allow assigning **lock** and **unlock** permissions for project plans.
For shared (group) project plans, group members MAY be granted lock or unlock permission so that a principal with the appropriate role can lock or unlock the plan document.
Enforcement is via API checks when updating the plan document (name, body); see [Project plan lock](projects_and_scopes.md#spec-cynai-access-projectplanlock) and [Project plan actions](access_control.md#spec-cynai-access-projectplanactions).

RBAC MUST allow assigning **approve** permission for project plans (`project_plan.approve` per [Project plan actions](access_control.md#spec-cynai-access-projectplanactions)).
Principals with approve permission MAY set or clear the plan approved state (approve or re-approve) for projects they are authorized to access.
For shared (group) project plans, group members MAY be granted approve permission so that a principal with the appropriate role can approve the plan for workflow start.

## Policy Evaluation Integration

The following requirements apply.

### Policy Evaluation Integration Applicable Requirements

- Spec ID: `CYNAI.ACCESS.PolicyEvaluation` <a id="spec-cynai-access-policyevaluation"></a>

#### Policy Evaluation Integration Applicable Requirements Requirements Traces

- [REQ-ACCESS-0109](../requirements/access.md#req-access-0109)
- [REQ-ACCESS-0110](../requirements/access.md#req-access-0110)
- [REQ-ACCESS-0111](../requirements/access.md#req-access-0111)

Recommended audit fields

- `user_id`
- `group_ids` (array)
- `role_names` (array)
- `scope_type`, `scope_id`

See [`docs/tech_specs/access_control.md`](access_control.md).

## User API and Data REST Resources

The following requirements apply.

### User API and Data REST Resources Applicable Requirements

- Spec ID: `CYNAI.ACCESS.UserApiDataRest` <a id="spec-cynai-access-userapirest"></a>

#### User API and Data REST Resources Applicable Requirements Requirements Traces

- [REQ-ACCESS-0112](../requirements/access.md#req-access-0112)
- [REQ-ACCESS-0120](../requirements/access.md#req-access-0120)
- [REQ-ACCESS-0113](../requirements/access.md#req-access-0113)

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md) and [`docs/tech_specs/data_rest_api.md`](data_rest_api.md).

## Future Considerations: External Group Service Integration

CyNodeAI MAY integrate with an external group service or IdP to source group membership for RBAC.
If integrated, CyNodeAI SHOULD support importing or syncing groups and memberships (e.g. SCIM-style provisioning).
The integration MUST preserve the same policy evaluation and auditing model.

Integration guidance

- Group and user records SHOULD be linkable to external directories using `external_source` and `external_id`.
- The system SHOULD support idempotent sync (repeated imports do not create duplicates).
- Synced entries SHOULD be distinguishable from local entries (e.g. `managed_by=external_sync`) to support safe merge behavior.
- Soft deactivation (`is_active=false`) SHOULD be preferred over deletes for membership changes to preserve auditability.
