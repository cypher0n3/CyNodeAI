# PROJCT Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `PROJCT` domain.
It covers the project entity: storage in PostgreSQL, stable identifiers, user-facing title and description, disable-without-delete, referential integrity, and the project-scoped scope model used by RBAC and preferences.

Related scope and RBAC behavior: [`docs/requirements/access.md`](access.md).
Technical specifications: [`docs/tech_specs/projects_and_scopes.md`](../tech_specs/projects_and_scopes.md), [`docs/tech_specs/project_git_repos.md`](../tech_specs/project_git_repos.md).

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
  [Default project](../tech_specs/projects_and_scopes.md#spec-cynai-access-defaultproject)
  <a id="req-projct-0104"></a>
- **REQ-PROJCT-0105:** The system MUST expose MCP tools so that orchestrator-side agents (e.g. Project Manager) can search and resolve projects for the authenticated user.
  All project list and search operations MUST be scoped to projects the authenticated user is authorized to access (e.g. via RBAC and default project); the gateway MUST enforce this scope and MUST NOT return projects the user cannot access.
  [CYNAI.ACCESS.ProjectsMcpSearch](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectsmcpsearch)
  [CYNAI.MCPGAT.Doc.GatewayEnforcement](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-doc-gatewayenforcement)
  <a id="req-projct-0105"></a>

- **REQ-PROJCT-0106:** The system MUST allow multiple Git repository associations per project so that tasks and Git egress operations can use project-scoped repo allowlists.
  [CYNAI.PROJCT.ProjectGitRepos](../tech_specs/project_git_repos.md#spec-cynai-projct-projectgitrepos)
  <a id="req-projct-0106"></a>

- **REQ-PROJCT-0107:** Each project-associated repo MUST be stored with a provider identifier for a supported Git host (e.g. github, gitlab, gitea; additional providers MAY be supported later) and a provider-specific repo identifier (e.g. owner/repo or namespace/project) that can be used for clone, push, and PR operations against the corresponding Git host.
  [CYNAI.PROJCT.RepoIdentifierFormat](../tech_specs/project_git_repos.md#spec-cynai-projct-repoidentifierformat)
  <a id="req-projct-0107"></a>

- **REQ-PROJCT-0108:** The system MUST support optional base URL overrides for self-hosted Git instances (e.g. GitHub Enterprise, self-hosted GitLab or Gitea) so that clone and egress operations use the correct host.
  [CYNAI.PROJCT.RepoIdentifierFormat](../tech_specs/project_git_repos.md#spec-cynai-projct-repoidentifierformat)
  <a id="req-projct-0108"></a>

- **REQ-PROJCT-0109:** Project git repo associations MUST be manageable via the same surfaces that manage projects (e.g. MCP tools and admin clients), and list/get operations MUST be scoped to the authenticated user's authorized projects.
  [CYNAI.PROJCT.ProjectGitReposMcp](../tech_specs/project_git_repos.md#spec-cynai-projct-projectgitreposmcp)
  <a id="req-projct-0109"></a>
