# Projects and Scope Model

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Goals](#goals)
- [Core Concepts](#core-concepts)
- [Database Model](#database-model)
  - [Postgres Schema](#postgres-schema)
    - [Projects Table](#projects-table)
    - [Project Plans Table](#project-plans-table)
    - [Project Plan Revisions Table](#project-plan-revisions-table)
    - [Specifications and Plan/Task References](#specifications-and-plantask-references)
    - [Project Git Repositories Table](#project-git-repositories-table)
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

#### Traces to Requirements

- [REQ-PROJCT-0100](../requirements/projct.md#req-projct-0100)
- [REQ-PROJCT-0101](../requirements/projct.md#req-projct-0101)
- [REQ-PROJCT-0102](../requirements/projct.md#req-projct-0102)

Each project has a user-friendly title (stored as `display_name`) and may have a text `description` (see [REQ-PROJCT-0103](../requirements/projct.md#req-projct-0103)).

### Postgres Schema

- Spec ID: `CYNAI.SCHEMA.Projects` <a id="spec-cynai-schema-projects"></a>

Projects are workspace boundaries used for authorization scope and preference resolution.

#### Projects Table

- Spec ID: `CYNAI.SCHEMA.ProjectsTable` <a id="spec-cynai-schema-projectstable"></a>

- `id` (uuid, pk)
- `slug` (text, unique)
- `display_name` (text)
  - user-friendly title for lists and detail views
- `description` (text, nullable)
  - optional text description for the project
- `allowed_model_ids` (jsonb, nullable)
  - optional array of model stable identifiers allowed for inference when the task or job is scoped to this project; null = no restriction at project scope; effective allowed set = intersection of system, project, and user allowlists
- `is_active` (boolean)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

##### Projects Table Constraints

- Index: (`slug`)
- Index: (`is_active`)

##### Allowed Model Allowlists

System-scoped allowed models may be stored in system config or a system-level table; user-scoped allowed models may be stored in preference entries (e.g. key `allowed_model_ids`) or a user-scoped column.
The effective allowed set for a job is the intersection of applicable lists; worker node model inventory is not part of the allowed set (used for placement only).

#### Project Plans Table

- Spec ID: `CYNAI.SCHEMA.ProjectPlansTable` <a id="spec-cynai-schema-projectplanstable"></a>

A project may have multiple plans; at most one plan per project may be active at a time.
Plan state values: `draft`, `ready`, `active`, `suspended`, `completed`, `canceled` (see [Project plan state](#project-plan-state)).
**Archived** is a separate boolean flag for UI/API views; archived plans do not run workflow and are not the active plan (enforced by API).

- `id` (uuid, pk)
- `project_id` (uuid, fk to `projects.id`, NOT NULL)
- `plan_name` (text, nullable)
  - optional name for this plan
- `plan_body` (text, nullable)
  - plan document body; stored as Markdown (see [REQ-PROJCT-0114](../requirements/projct.md#req-projct-0114))
- `state` (text, NOT NULL)
  - one of: `draft`, `ready`, `active`, `suspended`, `completed`, `canceled`
  - only one row per project may have `state = 'active'` (enforced by partial unique index); archived plans do not have state `active` (API enforces)
- `archived` (boolean, NOT NULL, default false)
  - when true, plan is archived for history/views; workflow does not run for this plan and this plan is not set to active; used by UIs/APIs for filtering and display
- `is_plan_locked` (boolean, default false)
  - when true, plan document (plan_name, plan_body) is read-only until unlocked; API enforces
- `plan_locked_at` (timestamptz, nullable)
- `plan_locked_by` (uuid, fk to `users.id`, nullable)
- `plan_approved_at` (timestamptz, nullable)
  - set when plan is approved (transition to ready or active); who approved and when
- `plan_approved_by` (uuid, fk to `users.id`, nullable)
- `comments` (jsonb, nullable)
  - same structure as task comments; see [Comments Structure (Plans and Tasks)](#comments-structure-plans-and-tasks)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `created_by` (uuid, fk to `users.id`, nullable)

##### Project Plans Table Constraints

- Unique partial: (`project_id`) WHERE `state` = 'active' (at most one active plan per project)
- Index: (`project_id`)
- Index: (`project_id`, `state`)
- Index: (`state`)
- Index: (`archived`) for list/filter by archived

##### Comments Structure (Plans and Tasks)

- Spec ID: `CYNAI.SCHEMA.CommentsStructure` <a id="spec-cynai-schema-commentsstructure"></a>

Plans and tasks both use the same JSON structure for **comments** (e.g. array of comment entries).
Each entry typically has: author (or user id), timestamp, and body (text or Markdown).
Host may define the exact shape (e.g. `{ "author_id": "<uuid>", "created_at": "<timestamptz>", "body": "<text>" }`).
When plan is locked, agents may update only completion status and comments (plan or task) per lock rules.

#### Project Plan Revisions Table

- Spec ID: `CYNAI.SCHEMA.ProjectPlanRevisionsTable` <a id="spec-cynai-schema-projectplanrevisionstable"></a>

Stores a snapshot of a project plan (document, task list, and task dependencies) each time the plan or its task list or dependencies change so users can view revision history.
One row per revision; version increments per plan.

- `id` (uuid, pk)
- `plan_id` (uuid, fk to `project_plans.id`, NOT NULL)
- `version` (integer, NOT NULL)
  - monotonically increasing per plan (1, 2, 3, ...)
- `plan_name` (text, nullable)
  - snapshot of project_plans.plan_name at revision time
- `plan_body` (text, nullable)
  - snapshot of project_plans.plan_body at revision time (Markdown)
- `task_ids` (jsonb, nullable)
  - array of task UUIDs in this plan at revision time
- `task_dependencies` (jsonb, nullable)
  - array of objects: `{ "task_id": "<uuid>", "depends_on_task_id": "<uuid>" }` capturing the dependency graph at revision time
- `created_at` (timestamptz)
- `created_by` (uuid, fk to `users.id`, nullable)

##### Project Plan Revisions Table Constraints

- Unique: (`plan_id`, `version`)
- Index: (`plan_id`, `created_at`)
- Index: (`plan_id`)

##### Project Plan Revisions Table Behavior

- The orchestrator or gateway inserts a new row into `project_plan_revisions` whenever that plan's `plan_name`, `plan_body`, the set of tasks with that `plan_id`, or the set of task_dependencies for tasks in that plan changes.
- Version is computed as the next integer per plan (e.g. MAX(version)+1 for that plan_id).
- Retention: implementation may support configurable retention (e.g. keep last N revisions per plan); minimum is to retain all revisions unless explicitly purged.

#### Specifications and Plan/Task References

Project-scoped specification references; plans and tasks reference specifications via join tables (they do not own them).
Implementations use Go and GORM per [Go SQL database standards](go_sql_database_standards.md).

##### Specifications Table

- Spec ID: `CYNAI.SCHEMA.SpecificationsTable` <a id="spec-cynai-schema-specificationstable"></a>

One row per specification reference; each row is tied to a **project**.
At least one of `spec_id`, `ref`, or `description` is non-null and non-empty (application or check constraint).

- `id` (uuid, pk)
- `project_id` (uuid, fk to `projects.id`, NOT NULL)
- `spec_id` (text, nullable) - stable identifier; format host-defined
- `ref` (text, nullable) - alternative identifier (e.g. doc path, external id)
- `description` (text, nullable) - prose summary or spec description; Markdown when host uses Markdown
- `symbol` (text, nullable) - short name or symbol for the spec item
- `kind` (text, nullable) - category (e.g. Type, Operation, Rule); host-defined
- `heading` (text, nullable) - display heading or title
- `status` (text, nullable) - lifecycle or maturity (e.g. draft, stable, deprecated); host-defined
- `since` (text, nullable) - version or date introduced; format host-defined
- `document_path` (text, nullable) - path to the containing document (repo-relative or URI)
- `anchor` (text, nullable) - fragment or anchor id for direct linking
- `source` (text, nullable) - provenance, URL, or path to the spec content
- `section` (text, nullable) - section or anchor within the document
- `spec_type` (text, nullable) - categorization (e.g. tech_spec, api_spec); host-defined (column name `spec_type` avoids Go reserved word `type`)
- `sort_order` (integer, nullable) - explicit order for display within the project
- `meta` (jsonb, nullable) - variable or nested data: traces_to, see_also, contract_subsections, and any additional host-defined keys (see [Specification Meta (Jsonb)](#specification-meta-jsonb))
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `created_by` (uuid, fk to `users.id`, nullable)

##### Specifications Table Constraints

- Check or application: at least one of `spec_id`, `ref`, or `description` is non-null and non-empty.
- Index: (`project_id`)
- Unique: (`project_id`, `spec_id`) WHERE `spec_id` IS NOT NULL AND `spec_id` != '' (optional; one row per spec_id per project)
- Index: (`project_id`, `sort_order`) for ordered listing (optional)
- GIN index on `meta` (optional) for containment or key existence queries

##### Specifications Table Behavior

- The orchestrator or gateway enforces that only authorized callers can create, update, or delete specification rows; scope to the project the context is authorized to access.
- When a project is deleted, specification rows for that project_id may be deleted (cascade) or retained for audit per host policy.
- Plans and tasks reference specifications via join tables; deleting a specification row may remove or retain references in those join tables per host policy.

##### Specification Meta (Jsonb)

The `meta` column holds a single JSON object for data that is variable, repeated, or nested.
Recommended keys (all optional; host-defined semantics): `traces_to` (array of strings), `see_also` (array of strings), `contract_subsections` (array of objects with e.g. heading, anchor, kind).
Implementations use a Go struct with `json` tags (or `datatypes.JSON`) for `meta`.

##### Plan Specifications Join Table

- Spec ID: `CYNAI.SCHEMA.PlanSpecificationsTable` <a id="spec-cynai-schema-planspecificationstable"></a>

- `plan_id` (uuid, fk to `project_plans.id`, NOT NULL)
- `specification_id` (uuid, fk to `specifications.id`, NOT NULL)

##### Plan Specifications Join Table Constraints

- Unique: (`plan_id`, `specification_id`)
- Index: (`plan_id`), (`specification_id`)

When a plan is deleted, join rows for that plan_id may be cascade-deleted; when a specification is deleted, join rows may be cascade-deleted per host policy.

##### Task Specifications Join Table

- Spec ID: `CYNAI.SCHEMA.TaskSpecificationsTable` <a id="spec-cynai-schema-taskspecificationstable"></a>

- `task_id` (uuid, fk to `tasks.id`, NOT NULL)
- `specification_id` (uuid, fk to `specifications.id`, NOT NULL)

##### Task Specifications Join Table Constraints

- Unique: (`task_id`, `specification_id`)
- Index: (`task_id`), (`specification_id`)

##### Task Specifications Join Table Behavior

Application ensures the task's project (via task.project_id or task.plan_id -> project_plans.project_id) matches the specification's project_id when adding a reference.
When a task is deleted, join rows for that task_id may be cascade-deleted; when a specification is deleted, join rows may be cascade-deleted per host policy.

##### ResolveSpecificationsForPlanOrTask Operation

- Spec ID: `CYNAI.SCHEMA.ResolveSpecificationsForPlanOrTask` <a id="spec-cynai-schema-resolvespecificationsforplanortask"></a>

Resolves the set of specifications referenced by a plan or a task (for display, MCP responses, or downstream logic).

###### `ResolveSpecificationsForPlanOrTask` Algorithm

<a id="algo-cynai-schema-resolvespecificationsforplanortask"></a>

1. Given `plan_id` or `task_id`. <a id="algo-cynai-schema-resolvespecificationsforplanortask-step-1"></a>
2. If plan_id: select `specification_id` from `plan_specifications` where `plan_id` = ?; if task_id: select `specification_id` from `task_specifications` where `task_id` = ?. <a id="algo-cynai-schema-resolvespecificationsforplanortask-step-2"></a>
3. Load specification rows from `specifications` for those ids (join or IN clause); ensure only rows for the same project as the plan/task are returned (authorization). <a id="algo-cynai-schema-resolvespecificationsforplanortask-step-3"></a>
4. Return ordered list: sort by `sort_order` (nulls last), then by `ref`, then by `created_at`. <a id="algo-cynai-schema-resolvespecificationsforplanortask-step-4"></a>

##### SpecificationObject Contract (API and MCP)

- Spec ID: `CYNAI.SCHEMA.SpecificationObjectContract` <a id="spec-cynai-schema-specificationobjectcontract"></a>

**Inputs (create/update row):** Required: `project_id` (uuid); at least one of `spec_id`, `ref`, or `description`.
Optional: `symbol`, `kind`, `heading`, `status`, `since`, `document_path`, `anchor`, `source`, `section`, `spec_type`, `sort_order`, and `meta` (traces_to, see_also, contract_subsections).

**Outputs (get/list):** Same keys as the object structure, plus `id`, `project_id`, `created_at`, `updated_at` when included.

**Attach to plan or task:** Client sends `specification_id` (or list of specification_ids); the join table is updated; the specification row is not modified.

#### Project Git Repositories Table

- Spec ID: `CYNAI.SCHEMA.ProjectGitReposTable` <a id="spec-cynai-schema-projectgitrepostable"></a>

Stores Git repository associations for projects so that tasks and Git egress can use project-scoped allowlists.

##### Git Repos Table Columns (Identity and Provider)

- `id` (uuid, pk)
- `project_id` (uuid, fk to `projects.id`, NOT NULL)
- `provider` (text, NOT NULL)
  - identifier for the Git host or service; any provider for which the system has support (e.g. github, gitlab, gitea); additional providers may be added without schema change
- `repo_identifier` (text, NOT NULL)
  - provider-specific identifier: for GitHub and Gitea use owner/repo; for GitLab use namespace/project (may include subgroups); semantics defined per provider in the project git repos spec
- `base_url` (text, nullable)
  - optional override for self-hosted instances (e.g. <https://gitea.example.com>, <https://gitlab.company.com>)

##### Git Repos Table Columns (Additional Information)

- `display_name` (text, nullable)
  - optional user-facing label for the repo in this project
- `description` (text, nullable)
  - optional longer description of the repo's role or purpose in this project
- `tags` (jsonb, nullable)
  - optional array of string tags for filtering or grouping (e.g. `["backend", "main"]`); structure is application-defined
- `metadata` (jsonb, nullable)
  - optional key-value data for future extension; no canonical keys required for MVP

##### Git Repos Table Columns (Timestamps)

- `created_at` (timestamptz)
- `updated_at` (timestamptz)

##### Git Repos Table Constraints

- Unique: (`project_id`, `provider`, `repo_identifier`)
- Index: (`project_id`)
- Index: (`provider`, `repo_identifier`) for egress lookups

### Default Project

- Spec ID: `CYNAI.ACCESS.DefaultProject` <a id="spec-cynai-access-defaultproject"></a>

#### Default Project Requirements Traces

- [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)

Each user (including the reserved system user) MUST have exactly one **default project**.
When a task, chat thread, or other project-scoped entity is created without an explicit `project_id`, the gateway (or orchestrator) MUST associate it with the creating user's default project (authenticated user when the request is authenticated, otherwise the system user).
The default project is a **catch-all for unrelated tasks**; the system (gateway, PMA) SHOULD **prefer to associate to another project whenever possible** (e.g. when the user or PMA can resolve a named project or the work clearly belongs to an existing non-default project).
The default project MAY be created on first use (e.g. first task or first chat thread for that user) or at user creation; its slug/identity MUST be deterministic per user (e.g. `default-<user_handle>` or a stable id-derived slug).
Clients and the PM/PA MAY allow users to explicitly select a different project; when they do, that project is used instead of the default project.

#### Default Project Slug and Identity Traces

- [REQ-PROJCT-0112](../requirements/projct.md#req-projct-0112)

## Project Plan

- Spec ID: `CYNAI.ACCESS.ProjectPlan` <a id="spec-cynai-access-projectplan"></a>

### Project Plan Requirements Traces

- [REQ-PROJCT-0110](../requirements/projct.md#req-projct-0110)
- [REQ-PROJCT-0111](../requirements/projct.md#req-projct-0111)

A **project plan** is the set of tasks belonging to a project with optional **execution order** and **task dependencies**.
A project MAY have **multiple plans**; each plan is a first-class entity stored in the `project_plans` table; see [Project plan state](#project-plan-state) and [`postgres_schema.md`](postgres_schema.md).
Tasks are associated with a plan via `tasks.plan_id`; when set, execution order and runnability are determined solely by [task dependencies](postgres_schema.md#spec-cynai-schema-taskdependenciestable) (prerequisite and dependent tasks).

### Project Plan State

- Spec ID: `CYNAI.ACCESS.ProjectPlanState` <a id="spec-cynai-access-projectplanstate"></a>

#### Project Plan State Requirements Traces

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

#### Project Plan Client Edit Requirements Traces

- [REQ-PROJCT-0113](../requirements/projct.md#req-projct-0113)

Users MUST be able to edit and update project plans and associated tasks via client tools (Web Console, CLI, or API).
The gateway API and client parity (Web Console, CLI) MUST support plan and task CRUD for plans and tasks the user is authorized to access; see [`user_api_gateway.md`](user_api_gateway.md) and [`cynork_cli.md`](cynork/cynork_cli.md).

### Project Plan and Task Text as Markdown

- Spec ID: `CYNAI.ACCESS.ProjectPlanMarkdown` <a id="spec-cynai-access-projectplanmarkdown"></a>

#### Project Plan and Task Text as Markdown Requirements Traces

- [REQ-PROJCT-0114](../requirements/projct.md#req-projct-0114)

Project plan and task description text, updates, and related editable content (e.g. plan body, task description, acceptance criteria text) MUST be stored as Markdown so clients can edit and render consistently.

### Project Plan Lock

- Spec ID: `CYNAI.ACCESS.ProjectPlanLock` <a id="spec-cynai-access-projectplanlock"></a>

#### Project Plan Lock Requirements Traces

- [REQ-PROJCT-0115](../requirements/projct.md#req-projct-0115)

Only the **plan document** (plan name, description/body) is locked; enforcement is via API checks so the plan document is not editable by the agent or by clients until unlocked.
When locked: users (via clients/API) MAY still add/remove/reorder tasks and edit task fields; agents MUST NOT change the plan or its tasks but MAY update completion status and comments on plans and tasks.
Unlock is permitted only for the user who locked the plan or a principal with unlock permission.
RBAC MUST allow assigning lock and unlock permissions for shared (group) project plans; see [Project plan lock RBAC](rbac_and_groups.md#spec-cynai-access-projectplanlockrbac) and [`access_control.md`](access_control.md).

### Project Plan Approved State

- Spec ID: `CYNAI.ACCESS.ProjectPlanApprovedState` <a id="spec-cynai-access-projectplanapprovedstate"></a>

#### Project Plan Approved State Requirements Traces

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

#### Project Plan Auto Un-Approve on Edit Requirements Traces

- [REQ-PROJCT-0118](../requirements/projct.md#req-projct-0118)

Whenever a plan's **document** (plan name, plan body), **plan's task list**, or **task dependencies** are updated (by any user or agent) while that plan is **active**, the system MUST set that plan's state back to **draft** (and clear `plan_approved_at`, `plan_approved_by`).
The plan remains editable (subject to lock); workflow for tasks in that plan MUST NOT start until a user (or principal with approve permission) re-approves the plan (set state to ready) and activates it (set state to active).
Implementation MUST perform this clear in the same transaction or immediately after the update that changed the plan, task list, or task dependencies; see [Workflow start gate (plan approved)](langgraph_mvp.md#spec-cynai-orches-workflowstartgateplanapproved).

### Plan Revisions

- Spec ID: `CYNAI.ACCESS.ProjectPlanRevisions` <a id="spec-cynai-access-projectplanrevisions"></a>

#### Plan Revisions Requirements Traces

- [REQ-PROJCT-0119](../requirements/projct.md#req-projct-0119)

The system MUST store **plan revisions** per plan so users can view plan change history.
Each revision is created when that plan's document (plan name, body), task list, or task dependencies change; storage format (snapshot per version), table schema, and retention are prescribed in [Project Plan Revisions Table](#spec-cynai-schema-projectplanrevisionstable).
Clients MUST be able to list revisions for a plan and retrieve a specific revision (e.g. by plan_id and version) via the User API Gateway; see [Project plan review and approve](#project-plan-review-and-approve).

### Project Plan Review and Approve

- Spec ID: `CYNAI.ACCESS.ProjectPlanReviewApprove` <a id="spec-cynai-access-projectplanreviewapprove"></a>

#### Project Plan Review and Approve Requirements Traces

- [REQ-PROJCT-0120](../requirements/projct.md#req-projct-0120)
- [REQ-CLIENT-0180](../requirements/client.md#req-client-0180)

Users MUST be able to **review** project plans (list plans per project, view plan document and task list, view revision history) and **approve** (or re-approve) a plan via the Web Console, CLI, or Data REST API.
The gateway MUST expose operations prescribed in [Project plan API](user_api_gateway.md#spec-cynai-usrgwy-projectplanapi): list plans for project (with optional filter by state and archived), get plan, list revisions for plan, approve plan (set plan to ready), activate plan (ready -> active), suspend, resume, cancel, archive, and set plan to completed (only when all tasks in the plan are closed).
Authorization for read uses `project_plan.read`; for approve uses `project_plan.approve`; for activate uses `project_plan.activate`; for archive uses `project_plan.archive` (see [Project plan actions](access_control.md#spec-cynai-access-projectplanactions)).
The Web Console and the CLI MUST provide capability parity for plan review and approve per [REQ-CLIENT-0180](../requirements/client.md#req-client-0180).

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

### Project Search via MCP Requirements Traces

- [REQ-PROJCT-0105](../requirements/projct.md#req-projct-0105)

Orchestrator-side agents (e.g. Project Manager) MUST be able to search and resolve projects via MCP tools so they can associate tasks and chat with the correct project when the user names a project (by slug or id).
All project list and search operations MUST be scoped to the authenticated user: only projects the user is authorized to access (default project plus projects for which the user or their groups have a role binding) MAY be returned.
The MCP gateway MUST enforce this scope; tool implementations MUST filter by the request subject's authorized project set before returning results.
Tool names and argument schemas are defined in [Project tools](mcp_tools/project_tools.md) (e.g. `project.get`, `project.list`).
Auditing MUST follow [MCP tool call auditing](mcp/mcp_tool_call_auditing.md).

Search semantics (MVP)

- **List**: Return projects the user can access, with optional pagination and optional text filter on slug/display_name/description.
- **Get by id or slug**: Return a single project if it is in the user's authorized set; otherwise not-found or access-denied.
- **Vector search**: Not required for MVP.
  Project metadata (slug, title, description) is small and keyword/list + get-by-id-or-slug is sufficient for "resolve by name or id" and "list my projects."
  Vector similarity search MAY be considered later if rich project descriptions and natural-language search ("projects about X") become a requirement.

## MVP Notes

- Project membership is derived from user identity plus RBAC bindings.
- A separate `project_memberships` table is not required for MVP.
  If added later, it MUST remain consistent with RBAC and audit requirements.
