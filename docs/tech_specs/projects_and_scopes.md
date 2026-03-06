# Projects and Scope Model

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Goals](#goals)
- [Core Concepts](#core-concepts)
- [Database Model](#database-model)
  - [Default project](#default-project)
- [Project plan](#project-plan)
  - [Project plan approved state](#project-plan-approved-state)
  - [Project plan auto un-approve on edit](#project-plan-auto-un-approve-on-edit)
  - [Project plan state](#project-plan-state)
  - [Plan revisions](#plan-revisions)
  - [Project plan review and approve](#project-plan-review-and-approve)
- [How Scope is Used](#how-scope-is-used)
  - [RBAC Scope](#rbac-scope)
  - [Preference Scope](#preference-scope)
  - [Task Scope](#task-scope)
  - [Chat Scope](#chat-scope)
- [Project Search via MCP](#project-search-via-mcp)
- [MVP Notes](#mvp-notes)

## Spec IDs

- Spec ID: `CYNAI.ACCESS.Doc.ProjectsAndScopes` <a id="spec-cynai-access-doc-projectsandscopes"></a>
- [CYNAI.ACCESS.ProjectPlan](#spec-cynai-access-projectplan)
- [CYNAI.ACCESS.ProjectPlanState](#spec-cynai-access-projectplanstate)
- [CYNAI.ACCESS.ProjectPlanClientEdit](#spec-cynai-access-projectplanclientedit)
- [CYNAI.ACCESS.ProjectPlanMarkdown](#spec-cynai-access-projectplanmarkdown)
- [CYNAI.ACCESS.ProjectPlanLock](#spec-cynai-access-projectplanlock)
- [CYNAI.ACCESS.ProjectPlanLockRbac](rbac_and_groups.md#spec-cynai-access-projectplanlockrbac)
- [CYNAI.ACCESS.ProjectPlanApprovedState](#spec-cynai-access-projectplanapprovedstate)
- [CYNAI.ACCESS.ProjectPlanAutoUnapprove](#spec-cynai-access-projectplanautounapprove)
- [CYNAI.ACCESS.ProjectPlanRevisions](#spec-cynai-access-projectplanrevisions)
- [CYNAI.ACCESS.ProjectPlanReviewApprove](#spec-cynai-access-projectplanreviewapprove)

This section defines stable Spec ID anchors for referencing this document.

## Document Overview

This document defines the project and scoping model for CyNodeAI.
It makes `project_id` a first-class database entity and clarifies how project scope is applied across RBAC, user task-execution preferences, and tasks.

Related documents

- Postgres schema: [`docs/tech_specs/postgres_schema.md`](postgres_schema.md)
- Project Git repos: [`docs/tech_specs/project_git_repos.md`](project_git_repos.md)
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
The default project is a **catch-all for unrelated tasks**; the system (gateway, PMA) SHOULD **prefer to associate to another project whenever possible** (e.g. when the user or PMA can resolve a named project or the work clearly belongs to an existing non-default project).
The default project MAY be created on first use (e.g. first task or first chat thread for that user) or at user creation; its slug/identity MUST be deterministic per user (e.g. `default-<user_handle>` or a stable id-derived slug).
Clients and the PM/PA MAY allow users to explicitly select a different project; when they do, that project is used instead of the default project.

Traces To:

- [REQ-PROJCT-0112](../requirements/projct.md#req-projct-0112)

## Project Plan

- Spec ID: `CYNAI.ACCESS.ProjectPlan` <a id="spec-cynai-access-projectplan"></a>

Traces To:

- [REQ-PROJCT-0110](../requirements/projct.md#req-projct-0110)
- [REQ-PROJCT-0111](../requirements/projct.md#req-projct-0111)

A **project plan** is the set of tasks belonging to a project with optional **execution order** and **task dependencies**.
A project MAY have **multiple plans**; each plan is a first-class entity stored in the `project_plans` table; see [Project plan state](#project-plan-state) and [`postgres_schema.md`](postgres_schema.md).
Tasks are associated with a plan via `tasks.plan_id`; when set, execution order and runnability are determined solely by [task dependencies](postgres_schema.md#spec-cynai-schema-taskdependenciestable) (prerequisite and dependent tasks).

### Project Plan State

- Spec ID: `CYNAI.ACCESS.ProjectPlanState` <a id="spec-cynai-access-projectplanstate"></a>

Traces To:

- [REQ-PROJCT-0110](../requirements/projct.md#req-projct-0110)
- [REQ-PROJCT-0117](../requirements/projct.md#req-projct-0117)
- [REQ-PROJCT-0118](../requirements/projct.md#req-projct-0118)
- [REQ-PROJCT-0121](../requirements/projct.md#req-projct-0121)
- [REQ-PROJCT-0122](../requirements/projct.md#req-projct-0122)
- [REQ-PROJCT-0124](../requirements/projct.md#req-projct-0124)

Each plan has a **state**: `draft`, `ready`, `active`, `suspended`, `completed`, or `canceled`.
**At most one plan per project may be active at a time**; all other plans in that project have a non-active state.

- **draft:** Plan is not approved; workflow for tasks in this plan MUST NOT start (unless PMA handoff per workflow gate).
- **ready:** Plan is approved but not yet active; set when the user (or agent with explicit user approval) approves the plan; the system tasks the PMA to add or update tasks.
  Workflow MUST NOT run until the plan is activated (set to active).
- **active:** The single plan per project for which workflow may run; set when the plan is activated (ready -> active).
  Only one plan per project may be active.
- **suspended:** Plan was active and has been paused; workflow MUST NOT run until the plan is resumed (suspended -> active).
- **completed:** Plan was previously active and all work (tasks associated with the plan) is done; no longer the active plan.
  A plan MAY be set to completed **only when the plan has at least one task and all such tasks are closed** (`tasks.closed = true`; see [Task status and closed state](../tech_specs/postgres_schema.md#spec-cynai-schema-taskstatusandclosed)); [REQ-PROJCT-0121](../requirements/projct.md#req-projct-0121).
  **A plan with no tasks is incomplete** and MUST NOT be set to completed.
- **canceled:** Plan was abandoned (from draft, ready, active, or suspended); workflow MUST NOT run.

When a plan is set to **active**, any other plan in the same project that is currently active MUST be set to **draft**, **suspended**, or (if all its tasks are closed) **completed** so that only one plan per project is active.
**All plans must have at least one task** to be considered ready for execution; see [REQ-PROJCT-0122](../requirements/projct.md#req-projct-0122).
When a plan is approved (set to **ready**) by the user, the **first action** the system MUST take is to task the PMA to add or update tasks on the plan; the plan is then activated (ready -> active) when it is ready for execution.
Storage is prescribed in [`postgres_schema.md`](postgres_schema.md): `project_plans.state`, `project_plans.archived`, and partial unique constraint on `(project_id)` WHERE `state = 'active'`.

**Archived flag:** Plans have an **archived** boolean (separate from state) for UI/API views and filtering.
**Archived plans MUST NOT run workflow** and **MUST NOT be the active plan**; the API MUST reject setting a plan to active when `archived = true`, and MUST reject setting `archived = true` while the plan is active (the plan must be suspended or canceled first).
See [REQ-PROJCT-0124](../requirements/projct.md#req-projct-0124).

### Project Plan Client Edit

- Spec ID: `CYNAI.ACCESS.ProjectPlanClientEdit` <a id="spec-cynai-access-projectplanclientedit"></a>

Traces To:

- [REQ-PROJCT-0113](../requirements/projct.md#req-projct-0113)

Users MUST be able to edit and update project plans and associated tasks via client tools (Web Console, CLI, or API).
The gateway API and client parity (Web Console, CLI) MUST support plan and task CRUD for plans and tasks the user is authorized to access; see [`user_api_gateway.md`](user_api_gateway.md) and [`cynork_cli.md`](cynork_cli.md).

### Project Plan and Task Text as Markdown

- Spec ID: `CYNAI.ACCESS.ProjectPlanMarkdown` <a id="spec-cynai-access-projectplanmarkdown"></a>

Traces To:

- [REQ-PROJCT-0114](../requirements/projct.md#req-projct-0114)

Project plan and task description text, updates, and related editable content (e.g. plan body, task description, acceptance criteria text) MUST be stored as Markdown so clients can edit and render consistently.

### Project Plan Lock

- Spec ID: `CYNAI.ACCESS.ProjectPlanLock` <a id="spec-cynai-access-projectplanlock"></a>

Traces To:

- [REQ-PROJCT-0115](../requirements/projct.md#req-projct-0115)

Only the **plan document** (plan name, description/body) is locked; enforcement is via API checks so the plan document is not editable by the agent or by clients until unlocked.
When locked: users (via clients/API) MAY still add/remove/reorder tasks and edit task fields; agents MUST NOT change the plan or its tasks but MAY update completion status and comments on plans and tasks.
Unlock is permitted only for the user who locked the plan or a principal with unlock permission.
RBAC MUST allow assigning lock and unlock permissions for shared (group) project plans; see [Project plan lock RBAC](rbac_and_groups.md#spec-cynai-access-projectplanlockrbac) and [`access_control.md`](access_control.md).

### Project Plan Approved State

- Spec ID: `CYNAI.ACCESS.ProjectPlanApprovedState` <a id="spec-cynai-access-projectplanapprovedstate"></a>

Traces To:

- [REQ-PROJCT-0117](../requirements/projct.md#req-projct-0117)
- [REQ-AGENTS-0136](../requirements/agents.md#req-agents-0136)

The system stores **plan state** (draft, ready, active, suspended, completed, canceled), the **archived** flag, and for the approved plan (ready or active) **who** approved it and **when** (see [Project plan state](#project-plan-state)).
Storage is prescribed in [`postgres_schema.md`](postgres_schema.md): `project_plans.state`, `project_plans.archived`, `project_plans.plan_approved_at`, `project_plans.plan_approved_by`.
When a plan's state is `active` and the plan is not archived, workflow for tasks in that plan MAY be started subject to [REQ-ORCHES-0152](../requirements/orches.md#req-orches-0152).
Only a principal with `project_plan.approve` permission MAY approve (set to ready); with `project_plan.activate` MAY activate (ready -> active); see [Project plan actions](access_control.md#spec-cynai-access-projectplanactions).
**Agents MAY set plan approved state only after seeking and obtaining explicit user approval** per [REQ-AGENTS-0136](../requirements/agents.md#req-agents-0136); the MCP tool and agent instructions MUST require the agent to seek explicit user approval before invoking plan approve.
Users may also approve and activate directly via Web Console, CLI, or user-authenticated API.

### Project Plan Auto Un-Approve on Edit

- Spec ID: `CYNAI.ACCESS.ProjectPlanAutoUnapprove` <a id="spec-cynai-access-projectplanautounapprove"></a>

Traces To:

- [REQ-PROJCT-0118](../requirements/projct.md#req-projct-0118)

Whenever a plan's **document** (plan name, plan body), **plan's task list**, or **task dependencies** are updated (by any user or agent) while that plan is **active**, the system MUST set that plan's state back to **draft** (and clear `plan_approved_at`, `plan_approved_by`).
The plan remains editable (subject to lock); workflow for tasks in that plan MUST NOT start until a user (or principal with approve permission) re-approves the plan (set state to ready) and activates it (set state to active).
Implementation MUST perform this clear in the same transaction or immediately after the update that changed the plan, task list, or task dependencies; see [Workflow start gate (plan approved)](langgraph_mvp.md#spec-cynai-orches-workflowstartgateplanapproved).

### Plan Revisions

- Spec ID: `CYNAI.ACCESS.ProjectPlanRevisions` <a id="spec-cynai-access-projectplanrevisions"></a>

Traces To:

- [REQ-PROJCT-0119](../requirements/projct.md#req-projct-0119)

The system MUST store **plan revisions** per plan so users can view plan change history.
Each revision is created when that plan's document (plan name, body), task list, or task dependencies change; storage format (snapshot per version), table schema, and retention are prescribed in [`postgres_schema.md`](postgres_schema.md) [Project plan revisions table](postgres_schema.md#spec-cynai-schema-projectplanrevisionstable).
Clients MUST be able to list revisions for a plan and retrieve a specific revision (e.g. by plan_id and version) via the User API Gateway; see [Project plan review and approve](#project-plan-review-and-approve).

### Project Plan Review and Approve

- Spec ID: `CYNAI.ACCESS.ProjectPlanReviewApprove` <a id="spec-cynai-access-projectplanreviewapprove"></a>

Traces To:

- [REQ-PROJCT-0120](../requirements/projct.md#req-projct-0120)
- [REQ-CLIENT-0179](../requirements/client.md#req-client-0179)

Users MUST be able to **review** project plans (list plans per project, view plan document and task list, view revision history) and **approve** (or re-approve) a plan via the Web Console, CLI, or Data REST API.
The gateway MUST expose operations prescribed in [Project plan API](user_api_gateway.md#spec-cynai-usrgwy-projectplanapi): list plans for project (with optional filter by state and archived), get plan, list revisions for plan, approve plan (set plan to ready), activate plan (ready -> active), suspend, resume, cancel, archive, and set plan to completed (only when all tasks in the plan are closed).
Authorization for read uses `project_plan.read`; for approve uses `project_plan.approve`; for activate uses `project_plan.activate`; for archive uses `project_plan.archive` (see [Project plan actions](access_control.md#spec-cynai-access-projectplanactions)).
The Web Console and the CLI MUST provide capability parity for plan review and approve per [REQ-CLIENT-0179](../requirements/client.md#req-client-0179).

## How Scope is Used

This section describes how `project` scope applies across core subsystems.

### RBAC Scope

- Spec ID: `CYNAI.ACCESS.RbacScope` <a id="spec-cynai-access-rbacscope"></a>

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
- Git egress repo allowlist: only repos associated with the project may be used for that task (see [`project_git_repos.md`](project_git_repos.md)).

### Chat Scope

Chat threads MAY be associated with a project via `chat_threads.project_id`.
When set, the project scope SHOULD be used for:

- access control policy evaluation when `project_id` is part of the request context
- grouping and filtering chat history for user clients

## Project Search via MCP

- Spec ID: `CYNAI.ACCESS.ProjectsMcpSearch` <a id="spec-cynai-access-projectsmcpsearch"></a>

Traces To:

- [REQ-PROJCT-0105](../requirements/projct.md#req-projct-0105)

Orchestrator-side agents (e.g. Project Manager) MUST be able to search and resolve projects via MCP tools so they can associate tasks and chat with the correct project when the user names a project (by slug or id).
All project list and search operations MUST be scoped to the authenticated user: only projects the user is authorized to access (default project plus projects for which the user or their groups have a role binding) MAY be returned.
The MCP gateway MUST enforce this scope; tool implementations MUST filter by the request subject's authorized project set before returning results.
Tool names and argument schemas are defined in the [MCP tool catalog](mcp_tool_catalog.md#spec-cynai-mcptoo-databasetools) (e.g. `db.project.get`, `db.project.list`).
Auditing MUST follow [MCP tool call auditing](mcp_tool_call_auditing.md).

Search semantics (MVP)

- **List**: Return projects the user can access, with optional pagination and optional text filter on slug/display_name/description.
- **Get by id or slug**: Return a single project if it is in the user's authorized set; otherwise not-found or access-denied.
- **Vector search**: Not required for MVP.
  Project metadata (slug, title, description) is small and keyword/list + get-by-id-or-slug is sufficient for "resolve by name or id" and "list my projects."
  Vector similarity search MAY be considered later if rich project descriptions and natural-language search ("projects about X") become a requirement; see `dev_docs/projects_mcp_search_and_vector.md`.

## MVP Notes

- Project membership is derived from user identity plus RBAC bindings.
- A separate `project_memberships` table is not required for MVP.
  If added later, it MUST remain consistent with RBAC and audit requirements.
