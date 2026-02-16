# RBAC and Groups

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Core Concepts](#core-concepts)
- [Group Membership Model](#group-membership-model)
- [RBAC Model](#rbac-model)
- [Policy Evaluation Integration](#policy-evaluation-integration)
- [User API and Data REST Resources](#user-api-and-data-rest-resources)
- [Future Considerations: External Group Service Integration](#future-considerations-external-group-service-integration)

## Document Overview

This document defines CyNodeAI role-based access control (RBAC) and how group membership is tracked for users.
It complements the policy model in [`docs/tech_specs/access_control.md`](access_control.md) by defining subjects, roles, and membership resolution.

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

Normative requirements

- The orchestrator MUST track groups and group membership in PostgreSQL.
- A user MAY belong to zero or more groups.
- Group membership MUST be auditable and support enable, disable, and removal.
- Group membership changes MUST be recorded with timestamps and an actor identifier.
- Group and membership records SHOULD support stable external identity mapping for future directory sync.

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

Normative requirements

- The system MUST support role assignments for users and groups.
- Role assignments MUST be scope-aware.
- Role evaluation MUST be performed at request time using current memberships and bindings.
- RBAC decisions MUST be auditable and attributable to the effective subject (user and group context).

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

## Policy Evaluation Integration

Normative requirements

- Policy evaluation MUST consider both direct user bindings and group-derived bindings.
- The orchestrator MUST resolve effective roles for a request using:
  - authenticated user identity
  - active group memberships
  - active role bindings within the relevant scope
- The resolved subject context MUST be included in audit logs.

Recommended audit fields

- `user_id`
- `group_ids` (array)
- `role_names` (array)
- `scope_type`, `scope_id`

See [`docs/tech_specs/access_control.md`](access_control.md).

## User API and Data REST Resources

Normative requirements

- The User API Gateway MUST expose groups and membership resources to authorized users.
- The Data REST API SHOULD expose:
  - groups
  - group memberships
  - roles and role bindings
- Admin operations MUST be policy-gated (e.g. create group, add member, bind role).

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
