# Projects and Scope Model

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Goals](#goals)
- [Core Concepts](#core-concepts)
- [Database Model](#database-model)
  - [Default project](#default-project)
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

- **Project**: A named workspace boundary used for user task-execution preference scoping and authorization scoping.
  Each project has a stable identifier, a unique slug, a user-friendly **title** (display name), and an optional text **description** for lists and detail views.
- **Default project**: Each user (including the reserved system user) has exactly one **default project** (see [Default project](#default-project)).
  When no project is explicitly set for a task, chat thread, or other project-scoped entity, the system associates it with the creating user's default project (authenticated user when present, otherwise system user).
- **Scope**: A tuple of `scope_type` and optional `scope_id`.
  System scope uses `scope_type=system` with `scope_id` null.

## Database Model

This section describes the project database model and how referential integrity is enforced.

### Applicable Requirements

- Spec ID: `CYNAI.ACCESS.ProjectsDatabaseModel` <a id="spec-cynai-access-projectsdb"></a>

Traces To:

- [REQ-PROJCT-0100](../requirements/projct.md#req-projct-0100)
- [REQ-PROJCT-0101](../requirements/projct.md#req-projct-0101)
- [REQ-PROJCT-0102](../requirements/projct.md#req-projct-0102)

Canonical tables and foreign keys are defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
Each project MUST have a user-friendly title (stored as `display_name`) and MAY have a text `description` (see [REQ-PROJCT-0103](../requirements/projct.md#req-projct-0103)).

### Default Project

- Spec ID: `CYNAI.ACCESS.DefaultProject` <a id="spec-cynai-access-defaultproject"></a>

Traces To:

- [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)

Each user (including the reserved system user) MUST have exactly one **default project**.
When a task, chat thread, or other project-scoped entity is created without an explicit `project_id`, the gateway (or orchestrator) MUST associate it with the creating user's default project (authenticated user when the request is authenticated, otherwise the system user).
The default project MAY be created on first use (e.g. first task or first chat thread for that user) or at user creation; its slug/identity MUST be deterministic per user (e.g. `default-<user_handle>` or a stable id-derived slug).
Clients and the PM/PA MAY allow users to explicitly select a different project; when they do, that project is used instead of the default project.

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

Tasks MUST be associable with both a user and a project.
Each task has a creating user (`tasks.created_by`, set from the authenticated request context when created via the gateway) and a project (`tasks.project_id`): when no project is explicitly set, the creating user's default project is used (see [Default project](#default-project)).
When `project_id` is set (explicit or personal), the project scope SHOULD be used for:

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
