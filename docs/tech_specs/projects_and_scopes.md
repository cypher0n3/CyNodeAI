# Projects and Scope Model

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Core Concepts](#core-concepts)
- [Database Model](#database-model)
- [How Scope is Used](#how-scope-is-used)
  - [RBAC Scope](#rbac-scope)
  - [Preference Scope](#preference-scope)
  - [Task Scope](#task-scope)
- [MVP Notes](#mvp-notes)

## Document Overview

This document defines the project and scoping model for CyNodeAI.
It makes `project_id` a first-class database entity and clarifies how project scope is applied across RBAC, preferences, and tasks.

Related documents

- Postgres schema: [`docs/tech_specs/postgres_schema.md`](postgres_schema.md)
- RBAC and scopes: [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md)
- Preferences scoping: [`docs/tech_specs/user_preferences.md`](user_preferences.md)
- Access control: [`docs/tech_specs/access_control.md`](access_control.md)

## Goals

- Make `project` scope concrete and referentially valid in PostgreSQL.
- Enable project-scoped preferences and RBAC role bindings.
- Allow tasks to be associated with an optional `project_id`.

## Core Concepts

Terminology

- **Project**: A named workspace boundary used for preferences and authorization scoping.
- **Scope**: A tuple of `scope_type` and optional `scope_id`.
  System scope uses `scope_type=system` with `scope_id` null.

## Database Model

Normative requirements

- The orchestrator MUST store projects in PostgreSQL with stable identifiers.
- Projects MUST be able to be disabled without deleting records.
- Tables that reference `project_id` MUST use a foreign key to `projects.id`.

Canonical tables and foreign keys are defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).

## How Scope is Used

This section describes how `project` scope applies across core subsystems.

### RBAC Scope

Role bindings support:

- System scope: `scope_type=system`, `scope_id` null.
- Project scope: `scope_type=project`, `scope_id` is a `projects.id`.

See [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).

### Preference Scope

Preferences support:

- System scope: `scope_type=system`, `scope_id` null.
- User scope: `scope_type=user`, `scope_id` is a `users.id`.
- Project scope: `scope_type=project`, `scope_id` is a `projects.id`.
- Task scope: `scope_type=task`, `scope_id` is a `tasks.id`.

See [`docs/tech_specs/user_preferences.md`](user_preferences.md).

### Task Scope

Tasks MAY be associated with a project via `tasks.project_id`.
When set, the project scope SHOULD be used for:

- preference resolution (project-level overrides)
- access control policy evaluation when `project_id` is part of the request context

## MVP Notes

- Project membership is derived from user identity plus RBAC bindings.
- A separate `project_memberships` table is not required for MVP.
  If added later, it MUST remain consistent with RBAC and audit requirements.
