# Projects and Scope Model

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Goals](#goals)
- [Core Concepts](#core-concepts)
- [Database Model](#database-model)
- [How Scope is Used](#how-scope-is-used)
  - [RBAC Scope](#rbac-scope)
  - [Preference Scope](#preference-scope)
  - [Task Scope](#task-scope)
- [MVP Notes](#mvp-notes)

## Spec IDs

- Spec ID: `CYNAI.ACCESS.Doc.ProjectsAndScopes` <a id="spec-cynai-access-doc-projectsandscopes"></a>

This section defines stable Spec ID anchors for referencing this document.

## Document Overview

This document defines the project and scoping model for CyNodeAI.
It makes `project_id` a first-class database entity and clarifies how project scope is applied across RBAC, user task-execution preferences, and tasks.

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
- **Project**: A named workspace boundary used for user task-execution preference scoping and authorization scoping.
- **Scope**: A tuple of `scope_type` and optional `scope_id`.
  System scope uses `scope_type=system` with `scope_id` null.

## Database Model

This section describes the project database model and how referential integrity is enforced.

### Applicable Requirements

- Spec ID: `CYNAI.ACCESS.ProjectsDatabaseModel` <a id="spec-cynai-access-projectsdb"></a>

Traces To:

- [REQ-ACCESS-0114](../requirements/access.md#req-access-0114)
- [REQ-ACCESS-0115](../requirements/access.md#req-access-0115)
- [REQ-ACCESS-0116](../requirements/access.md#req-access-0116)

Canonical tables and foreign keys are defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).

## How Scope is Used

This section describes how `project` scope applies across core subsystems.

### RBAC Scope

Role bindings support:

- System scope: `scope_type=system`, `scope_id` null.
- Project scope: `scope_type=project`, `scope_id` is a `projects.id`.

See [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).

### Preference Scope

User task-execution preferences support:

- System scope: `scope_type=system`, `scope_id` null.
- User scope: `scope_type=user`, `scope_id` is a `users.id`.
- Group scope: `scope_type=group`, `scope_id` is a `groups.id`.
- Project scope: `scope_type=project`, `scope_id` is a `projects.id`.
- Task scope: `scope_type=task`, `scope_id` is a `tasks.id`.

See [`docs/tech_specs/user_preferences.md`](user_preferences.md).

### Task Scope

Tasks MAY be associated with a project via `tasks.project_id`.
When set, the project scope SHOULD be used for:

- preference resolution (project-level overrides)
- access control policy evaluation when `project_id` is part of the request context

### Chat Scope

Chat threads MAY be associated with a project via `chat_threads.project_id`.
When set, the project scope SHOULD be used for:

- access control policy evaluation when `project_id` is part of the request context
- grouping and filtering chat history for user clients

## MVP Notes

- Project membership is derived from user identity plus RBAC bindings.
- A separate `project_memberships` table is not required for MVP.
  If added later, it MUST remain consistent with RBAC and audit requirements.
