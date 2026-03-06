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

- **REQ-PROJCT-0110:** A project MAY have multiple project plans (each plan is a task set with optional execution order).
  At most one plan per project MAY be active (approved for execution) at a time; all other plans in that project MUST be in draft (unapproved) or completed state.
  [CYNAI.ACCESS.ProjectPlan](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplan)
  [CYNAI.ACCESS.ProjectPlanState](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanstate)
  <a id="req-projct-0110"></a>
- **REQ-PROJCT-0111:** Tasks created under a project MAY be associated with a specific plan (plan_id) when the project has plans; execution order and runnability within that plan are defined solely by task dependencies (task_dependencies; prerequisite and dependent tasks).
  [CYNAI.ACCESS.ProjectPlan](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplan)
  [CYNAI.SCHEMA.TaskDependenciesTable](../tech_specs/postgres_schema.md#spec-cynai-schema-taskdependenciestable)
  <a id="req-projct-0111"></a>
- **REQ-PROJCT-0112:** The user's default project is a catch-all for unrelated tasks; the system (gateway, PMA) SHOULD prefer to associate to another project whenever possible (e.g. when the user or PMA can resolve a named project or the work clearly belongs to an existing non-default project).
  [CYNAI.ACCESS.DefaultProject](../tech_specs/projects_and_scopes.md#spec-cynai-access-defaultproject)
  <a id="req-projct-0112"></a>
- **REQ-PROJCT-0113:** Users MUST be able to edit and update project plans and associated tasks via client tools (Web Console, CLI, or API).
  [CYNAI.ACCESS.ProjectPlanClientEdit](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanclientedit)
  <a id="req-projct-0113"></a>
- **REQ-PROJCT-0114:** Project plan and task description text, updates, and related editable content MUST be stored as Markdown for editing in client tools.
  [CYNAI.ACCESS.ProjectPlanMarkdown](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanmarkdown)
  <a id="req-projct-0114"></a>
- **REQ-PROJCT-0115:** Users MAY lock a project plan so that only the plan document (plan name, description/body) is not editable by the agent or by client interfaces until unlocked; enforcement via API checks.
  When locked: users (via clients/API) MAY still add/remove/reorder tasks and edit task fields; agents MUST NOT change the plan or its tasks but MAY update completion status and comments on plans and tasks.
  Only the user or a principal with unlock permission MAY unlock.
  [CYNAI.ACCESS.ProjectPlanLock](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanlock)
  <a id="req-projct-0115"></a>
- **REQ-PROJCT-0116:** RBAC MUST allow assigning lock and unlock permissions for shared (group) project plans.
  [CYNAI.ACCESS.ProjectPlanLockRbac](../tech_specs/rbac_and_groups.md#spec-cynai-access-projectplanlockrbac)
  <a id="req-projct-0116"></a>
- **REQ-PROJCT-0117:** The system MUST track plan state (draft, ready, active, suspended, completed, canceled) and the archived flag; for the approved plan (ready or active), who approved it and when.
  Only one plan per project may be active at a time; workflow for tasks in that plan MAY proceed subject to the orchestrator workflow start gate (REQ-ORCHES-0152).
  [CYNAI.ACCESS.ProjectPlanApprovedState](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanapprovedstate)
  [CYNAI.ACCESS.ProjectPlanState](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanstate)
  <a id="req-projct-0117"></a>
- **REQ-PROJCT-0118:** When a plan's document (plan name, body), the plan's task list, or task dependencies are edited after the plan is active, the system MUST set that plan's state back to draft until a user (or principal with approve permission) re-approves it.
  When a plan is approved (set to ready) or activated (set to active), any other plan in the same project that is currently active MUST be set to draft, suspended, or to completed only if all tasks associated with that plan are closed (per REQ-PROJCT-0121), so that only one plan per project is active.
  [CYNAI.ACCESS.ProjectPlanAutoUnapprove](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanautounapprove)
  [CYNAI.ACCESS.ProjectPlanState](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanstate)
  <a id="req-projct-0118"></a>
- **REQ-PROJCT-0119:** The system MUST store plan revisions so users can view plan change history (e.g. per-revision snapshot or delta).
  Revisions MUST be created when the plan document, task list, or task dependencies change; storage format and retention are prescribed in tech specs.
  [CYNAI.ACCESS.ProjectPlanRevisions](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanrevisions)
  [CYNAI.SCHEMA.ProjectPlanRevisionsTable](../tech_specs/postgres_schema.md#spec-cynai-schema-projectplanrevisionstable)
  <a id="req-projct-0119"></a>
- **REQ-PROJCT-0120:** Users MUST be able to review project plans and approve (or re-approve) plans via client tools (Web Console, CLI, or API).
  Review includes viewing the plan document and task list and viewing plan revision history.
  [CYNAI.ACCESS.ProjectPlanReviewApprove](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanreviewapprove)
  [CYNAI.USRGWY.ProjectPlanApi](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-projectplanapi)
  <a id="req-projct-0120"></a>
- **REQ-PROJCT-0121:** A plan's state MAY be set to completed only when the plan has at least one task and **all tasks** associated with that plan (tasks.plan_id = plan.id) **are closed** (`tasks.closed = true`; status is tracked separately per tech specs).
  A plan with no tasks is incomplete and MUST NOT be set to completed.
  The API (gateway) MUST enforce this condition and reject requests to set a plan to completed when the plan has no tasks or not all tasks are closed; agents and clients MUST use the API and are subject to the same enforcement.
  [CYNAI.ACCESS.ProjectPlanState](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanstate)
  <a id="req-projct-0121"></a>
- **REQ-PROJCT-0122:** All plans must have at least one task to be considered ready for execution.
  When a plan is approved (set to ready) by the user, the system MUST task the PMA to add or update tasks on the plan so that it is ready for execution; this MUST be the first action after the plan is set to ready.
  The plan is then activated (ready -> active) when ready for execution.
  [CYNAI.ACCESS.ProjectPlanState](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanstate)
  [CYNAI.AGENTS.PlanApprovedPmaTasked](../tech_specs/project_manager_agent.md#spec-cynai-agents-planapprovedpmatasked)
  <a id="req-projct-0122"></a>
- **REQ-PROJCT-0123:** The system MUST support explicit task dependencies within a plan (task_dependencies table): a task may depend on one or more other tasks in the same plan; a task is runnable only when all its dependencies have status `completed`; failed or non-completed dependencies block dependents until the dependency is retried and completed; multiple tasks with no unsatisfied dependencies MAY run in parallel.
  Editing a plan's task list or task dependencies MUST trigger plan revision and (when the plan is active) auto un-approve per REQ-PROJCT-0118 and REQ-PROJCT-0119.
  [CYNAI.SCHEMA.TaskDependenciesTable](../tech_specs/postgres_schema.md#spec-cynai-schema-taskdependenciestable)
  [CYNAI.ACCESS.ProjectPlanRevisions](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanrevisions)
  <a id="req-projct-0123"></a>
- **REQ-PROJCT-0124:** The system MUST support an **archived** flag on plans (separate from state) for UI/API views and filtering.
  Archived plans MUST NOT run workflow (the workflow start gate MUST deny workflow start for tasks in an archived plan).
  Archived plans MUST NOT be the active plan (the API MUST reject setting a plan to active when archived = true, and MUST reject setting archived = true while the plan is active; the plan must be suspended or canceled first).
  [CYNAI.ACCESS.ProjectPlanState](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanstate)
  [CYNAI.SCHEMA.ProjectPlansTable](../tech_specs/postgres_schema.md#spec-cynai-schema-projectplanstable)
  <a id="req-projct-0124"></a>
