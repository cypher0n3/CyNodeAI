# PROJCT Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `PROJCT` domain.
It covers the project entity: storage in PostgreSQL, stable identifiers, user-facing title and description, disable-without-delete, referential integrity, and the project-scoped scope model used by RBAC and preferences.

Related scope and RBAC behavior: [`docs/requirements/access.md`](access.md).
Technical specification: [`docs/tech_specs/projects_and_scopes.md`](../tech_specs/projects_and_scopes.md).

## 2 Requirements

- **REQ-PROJCT-0001:** Projects in PostgreSQL with stable ids; disable without delete; FKs to projects.id.
  [CYNAI.ACCESS.Doc.ProjectsAndScopes](../tech_specs/projects_and_scopes.md#spec-cynai-access-doc-projectsandscopes)
  [CYNAI.ACCESS.ProjectsDatabaseModel](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectsdb)
  <a id="req-projct-0001"></a>
- **REQ-PROJCT-0100:** The orchestrator MUST store projects in PostgreSQL with stable identifiers.
  [CYNAI.ACCESS.ProjectsDatabaseModel](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectsdb)
  <a id="req-projct-0100"></a>
- **REQ-PROJCT-0101:** Projects MUST be able to be disabled without deleting records.
  [CYNAI.ACCESS.ProjectsDatabaseModel](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectsdb)
  <a id="req-projct-0101"></a>
- **REQ-PROJCT-0102:** Tables that reference `project_id` MUST use a foreign key to `projects.id`.
  [CYNAI.ACCESS.ProjectsDatabaseModel](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectsdb)
  <a id="req-projct-0102"></a>
- **REQ-PROJCT-0103:** Each project MUST have a user-friendly title (display name) and MAY have a text description for user-facing lists and detail views.
  [CYNAI.ACCESS.ProjectsDatabaseModel](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectsdb)
  <a id="req-projct-0103"></a>
- **REQ-PROJCT-0104:** Each user (including the reserved system user) MUST have a **default project**.
  When a task, chat thread, or other project-scoped entity is created without an explicit `project_id`, the system MUST associate it with the creating user's default project (authenticated user when present, otherwise system user).
  The default project MAY be created on first use or at user creation; its identity MUST be deterministic per user (e.g. slug derived from user id or handle).
  [CYNAI.ACCESS.Doc.ProjectsAndScopes](../tech_specs/projects_and_scopes.md#spec-cynai-access-doc-projectsandscopes)
  [Default project](projects_and_scopes.md#default-project)
  <a id="req-projct-0104"></a>
