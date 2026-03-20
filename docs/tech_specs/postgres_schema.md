# Postgres Schema

- [Document Overview](#document-overview)
- [Goals and Scope](#goals-and-scope)
- [Storing This Schema in Code](#storing-this-schema-in-code)
  - [Storing This Schema in Code Applicable Requirements](#storing-this-schema-in-code-applicable-requirements)
- [Schema Overview](#schema-overview)
- [Identity and Authentication](#identity-and-authentication)
- [Projects](#projects)
- [Groups and RBAC](#groups-and-rbac)
- [Access Control](#access-control)
- [API Egress Credentials](#api-egress-credentials)
- [Preferences](#preferences)
- [System Settings](#system-settings)
  - [System Settings Table](#system-settings-table)
  - [System Settings Audit Log Table](#system-settings-audit-log-table)
- [Personas](#personas)
  - [Personas Table](#personas-table)
- [Tasks, Jobs, and Nodes](#tasks-jobs-and-nodes)
  - [Task vs Job (Terminology)](#task-vs-job-terminology)
  - [Tasks Table](#tasks-table)
  - [Jobs Table](#jobs-table)
  - [Nodes Table](#nodes-table)
  - [Node Capabilities Table](#node-capabilities-table)
- [Workflow Checkpoints](#workflow-checkpoints)
  - [Workflow Checkpoints Table](#workflow-checkpoints-table)
  - [Task Workflow Leases Table](#task-workflow-leases-table)
- [Sandbox Image Registry](#sandbox-image-registry)
  - [Sandbox Images Table](#sandbox-images-table)
  - [Sandbox Image Versions Table](#sandbox-image-versions-table)
  - [Node Sandbox Image Availability Table](#node-sandbox-image-availability-table)
- [Runs and Sessions](#runs-and-sessions)
  - [Runs Table](#runs-table)
  - [Sessions Table](#sessions-table)
- [Chat Threads and Messages](#chat-threads-and-messages)
  - [Source Documents](#source-documents)
  - [Chat Threads Table](#chat-threads-table)
  - [Chat Messages Table](#chat-messages-table)
  - [Chat Message Attachments Table](#chat-message-attachments-table)
- [Task Artifacts](#task-artifacts)
  - [Task Artifacts Constraints](#task-artifacts-constraints)
- [Vector Storage (`pgvector`)](#vector-storage-pgvector)
  - [Vector Storage Applicable Requirements](#vector-storage-applicable-requirements)
  - [Vector Items Table](#vector-items-table)
  - [Vector Retrieval and RBAC](#vector-retrieval-and-rbac)
- [Audit Logging](#audit-logging)
  - [Auth Audit Log Table](#auth-audit-log-table)
  - [MCP Tool Call Audit Log Table](#mcp-tool-call-audit-log-table)
  - [Other Audit Tables](#other-audit-tables)
  - [Chat Audit Log Table](#chat-audit-log-table)
- [Model Registry](#model-registry)
  - [Models Table](#models-table)
  - [Model Versions Table](#model-versions-table)
  - [Model Artifacts Table](#model-artifacts-table)
  - [Node Model Availability Table](#node-model-availability-table)
- [Table Summary and Dependencies](#table-summary-and-dependencies)

## Document Overview

This document is the single canonical specification for the CyNodeAI orchestrator PostgreSQL schema.
It consolidates and extends table definitions referenced across the tech specs so that the MVP (and later milestones) can implement the schema without ambiguity.

### Source of Truth

- This document is authoritative for table names, column names, types, and constraints.
- Where another spec defines "recommended" tables (e.g. local user accounts, RBAC), this document adopts those definitions and adds any missing tables (tasks, jobs, nodes, task artifacts, auth audit).
- Cross-references to other specs are for context and behavior; schema details here override any conflicting table definitions elsewhere.

See [`docs/tech_specs/_main.md`](_main.md) for the MVP development plan and foundations.

## Goals and Scope

- Define all tables required for MVP: users, local auth sessions, groups and RBAC, tasks, jobs, nodes, artifacts, and audit logging.
- Use a single `users` table shared by local authentication, RBAC, and preferences.
- Use consistent types: `uuid` for primary keys and foreign keys, `timestamptz` for timestamps, `text` for strings unless a more specific type is required.
- Support auditing: domain-specific audit tables where specs require it; recommended fields for each.

## Storing This Schema in Code

This tech spec is the source of truth for table names, columns, and constraints.
Implementations MUST keep the Go database models and schema bootstrap logic in sync with this document so environments can be created and upgraded deterministically.

### Storing This Schema in Code Applicable Requirements

- Spec ID: `CYNAI.SCHEMA.StoringInCode` <a id="spec-cynai-schema-storingcode"></a>

All orchestrator PostgreSQL access MUST use GORM per [Go SQL database standards](go_sql_database_standards.md#spec-cynai-stands-gosqlgorm).
GORM table models MUST follow the [GORM model structure](go_sql_database_standards.md#spec-cynai-stands-gormmodelstructure): domain base struct plus a GORM record struct that embeds GormModelUUID (or equivalent) and the domain struct; record structs live only in the database package.

#### Traces to Requirements

- [REQ-SCHEMA-0100](../requirements/schema.md#req-schema-0100)
- [REQ-SCHEMA-0101](../requirements/schema.md#req-schema-0101)
- [REQ-SCHEMA-0102](../requirements/schema.md#req-schema-0102)
- [REQ-SCHEMA-0103](../requirements/schema.md#req-schema-0103)
- [REQ-SCHEMA-0104](../requirements/schema.md#req-schema-0104)
- [REQ-SCHEMA-0105](../requirements/schema.md#req-schema-0105)
- [REQ-SCHEMA-0120](../requirements/schema.md#req-schema-0120)

### Recommended Repository Layout

- Domain base structs: `internal/models/` (or `go_shared_libs` when the type is shared with worker_node or other modules).
- GORM record structs and migrations: `internal/database/` (record types used for AutoMigrate and persistence only).
- Idempotent DDL (extensions, advanced indexes): `internal/database/ddl/` or equivalent.

### Implementation Notes

- AutoMigrate is convenient for MVP, but it can drift across versions.
  Prefer explicit version pinning and CI checks that validate the expected schema exists.
- A migration tool/library MAY be used for the DDL bootstrap step, but SQL files should remain committed to the repo.

### Out of Scope

- Node capability report and node configuration payload wire formats (see [`docs/tech_specs/worker_node.md`](worker_node.md)).
- MCP tool allowlists and per-tool scope (see [`docs/tech_specs/mcp_tools/access_allowlists_and_scope.md`](mcp_tools/access_allowlists_and_scope.md)); gateway enforcement (see [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)).

## Schema Overview

The schema is organized into logical groups of related tables.

### Logical Groups

1. **Identity and authentication:** `users`, `password_credentials`, `refresh_sessions`
2. **Projects:** `projects`, `project_plans`, `project_plan_revisions`, `project_git_repos`
3. **Groups and RBAC:** `groups`, `group_memberships`, `roles`, `role_bindings`
4. **Access control:** `access_control_rules`, `access_control_audit_log`
5. **API egress credentials:** `api_credentials`
6. **Preferences:** `preference_entries`, `preference_audit_log`
7. **Personas:** `personas` (reusable SBA role/identity descriptions; embedded inline in job spec at job-build time)
8. **Tasks, jobs, nodes, workflow:** `tasks`, `task_dependencies`, `jobs`, `nodes`, `node_capabilities`, `workflow_checkpoints`, `task_workflow_leases`
9. **Sandbox image registry:** `sandbox_images`, `sandbox_image_versions`, `node_sandbox_image_availability`
10. **Runs and sessions:** `runs`, `sessions`
11. **Chat:** `chat_threads`, `chat_messages`
12. **Task artifacts:** `task_artifacts`
13. **Vector storage (`pgvector`):** `vector_items`
14. **Audit:** `auth_audit_log`, `mcp_tool_call_audit_log` (and domain-specific audit tables above)
15. **Model registry (optional for MVP):** `models`, `model_versions`, `model_artifacts`, `node_model_availability`

## Identity and Authentication

- Spec ID: `CYNAI.SCHEMA.IdentityAuth` <a id="spec-cynai-schema-identityauth"></a>

The orchestrator stores users and local auth state in PostgreSQL.
Credentials and refresh tokens are stored as hashes.

**Schema definitions:** See [Postgres Schema](local_user_accounts.md#spec-cynai-schema-identityauth) in [`local_user_accounts.md`](local_user_accounts.md).

### Identity Tables

- `users` - See [Users Table](local_user_accounts.md#spec-cynai-schema-userstable)
- `password_credentials` - See [Password Credentials Table](local_user_accounts.md#spec-cynai-schema-passwordcredentialstable)
- `refresh_sessions` - See [Refresh Sessions Table](local_user_accounts.md#spec-cynai-schema-refreshsessionstable)

## Projects

- Spec ID: `CYNAI.SCHEMA.Projects` <a id="spec-cynai-schema-projects"></a>

Projects are workspace boundaries used for authorization scope and preference resolution.

**Schema definitions:** See [Postgres Schema](projects_and_scopes.md#spec-cynai-schema-projects) in [`projects_and_scopes.md`](projects_and_scopes.md).

### Projects Tables

- `projects` - See [Projects Table](projects_and_scopes.md#spec-cynai-schema-projectstable)
- `project_plans` - See [Project Plans Table](projects_and_scopes.md#spec-cynai-schema-projectplanstable)
- `project_plan_revisions` - See [Project Plan Revisions Table](projects_and_scopes.md#spec-cynai-schema-projectplanrevisionstable)
- `specifications` - See [Specifications Table](projects_and_scopes.md#spec-cynai-schema-specificationstable)
- `plan_specifications` - See [Plan Specifications Join Table](projects_and_scopes.md#spec-cynai-schema-planspecificationstable)
- `task_specifications` - See [Task Specifications Join Table](projects_and_scopes.md#spec-cynai-schema-taskspecificationstable)
- `project_git_repos` - See [Project Git Repositories Table](projects_and_scopes.md#spec-cynai-schema-projectgitrepostable)

## Groups and RBAC

- Spec ID: `CYNAI.SCHEMA.GroupsRbac` <a id="spec-cynai-schema-groupsrbac"></a>

The orchestrator tracks groups, group membership, roles, and role bindings in PostgreSQL.
Policy evaluation and auditing depend on these tables.

**Schema definitions:** See [Postgres Schema](rbac_and_groups.md#spec-cynai-schema-groupsrbac) in [`rbac_and_groups.md`](rbac_and_groups.md).

### Groups and RBAC Tables

- `groups` - See [Groups Table](rbac_and_groups.md#spec-cynai-schema-groupstable)
- `group_memberships` - See [Group Memberships Table](rbac_and_groups.md#spec-cynai-schema-groupmembershipstable)
- `roles` - See [Roles Table](rbac_and_groups.md#spec-cynai-schema-rolestable)
- `role_bindings` - See [Role Bindings Table](rbac_and_groups.md#spec-cynai-schema-rolebindingstable)

## Access Control

- Spec ID: `CYNAI.SCHEMA.AccessControl` <a id="spec-cynai-schema-accesscontrol"></a>

Policy rules and access control audit log.
Used by API Egress, Secure Browser, and other policy-enforcing services.

**Schema definitions:** See [Postgres Schema](access_control.md#spec-cynai-schema-accesscontrol) in [`access_control.md`](access_control.md).

### Access Control Tables

- `access_control_rules` - See [Access Control Rules Table](access_control.md#spec-cynai-schema-accesscontrolrulestable)
- `access_control_audit_log` - See [Access Control Audit Log Table](access_control.md#spec-cynai-schema-accesscontrolauditlogtable)

## API Egress Credentials

- Spec ID: `CYNAI.SCHEMA.ApiEgressCredentials` <a id="spec-cynai-schema-apiegresscredentials"></a>

Credentials for outbound API calls are stored in PostgreSQL and are only retrievable by the API Egress Server.
Agents never receive credentials in responses.

**Schema definitions:** See [Postgres Schema](api_egress_server.md#spec-cynai-schema-apiegresscredentials) in [`api_egress_server.md`](api_egress_server.md).

### API Egress Tables

- `api_credentials` - See [API Credentials Table](api_egress_server.md#spec-cynai-schema-apicredentialstable)

## Preferences

- Spec ID: `CYNAI.SCHEMA.Preferences` <a id="spec-cynai-schema-preferences"></a>

Preference entries store user task-execution preferences and constraints.
Preference entries are scoped (system, user, group, project, task) with precedence.
Deployment and service configuration (ports, hostnames, database DSNs, and secrets) are not stored as preferences.
The distinction between preferences and system settings is defined in [User preferences (Terminology)](user_preferences.md#spec-cynai-stands-preferenceterminology).
The `users` table is shared with identity and RBAC.

**Schema definitions:** See [Postgres Schema](user_preferences.md#spec-cynai-schema-preferences) in [`user_preferences.md`](user_preferences.md).

### Preferences Tables

- `preference_entries` - See [Preference Entries Table](user_preferences.md#spec-cynai-schema-preferenceentriestable)
- `preference_audit_log` - See [Preference Audit Log Table](user_preferences.md#spec-cynai-schema-preferenceauditlogtable)

## System Settings

System settings store operator-managed operational configuration and policy parameters.
System settings are not user task-execution preferences; for the distinction, see [User preferences (Terminology)](user_preferences.md#spec-cynai-stands-preferenceterminology).
System settings MUST NOT store secrets in plaintext.

### System Settings Table

Table name: `system_settings`.

- `key` (text, pk)
- `value` (jsonb)
- `value_type` (text)
  - examples: string|number|boolean|object|array
- `version` (int)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`key`)
- Index: (`updated_at`)

### System Settings Audit Log Table

Table name: `system_settings_audit_log`.

- `id` (uuid, pk)
- `key` (text)
- `old_value` (jsonb)
- `new_value` (jsonb)
- `changed_at` (timestamptz)
- `changed_by` (text)
- `reason` (text, nullable)

Constraints

- Index: (`key`)
- Index: (`changed_at`)

## Personas

Agent personas are named, reusable descriptions of how the sandbox agent should behave (role, identity, tone); they are not customer or end-user personas.
They are stored in the deployment and are queriable by agents (PMA, PAA, orchestrator job builder) via the User API Gateway or MCP.
When building a job, the builder resolves the chosen Agent persona by id (or by title with scope precedence) and embeds `title` and `description` inline into the job spec; the SBA receives only the inline object.
Editing (create, update, delete) is subject to RBAC: system-scoped personas require admin (or equivalent) role; user-/project-/group-scoped require appropriate role for that scope; see [data_rest_api.md - Core Resources](data_rest_api.md#spec-cynai-datapi-coreresources).

Source: [cynode_sba.md - Persona on the Job](cynode_sba.md#spec-cynai-sbagnt-jobpersona).

### Personas Table

- Spec ID: `CYNAI.SCHEMA.PersonasTable` <a id="spec-cynai-schema-personastable"></a>

- `id` (uuid, pk)
- `title` (text, required)
  - short human-readable label (e.g. "Backend Developer", "Security Reviewer")
- `description` (text, required)
  - short prose in the form "You are a &lt;role&gt; with &lt;background&gt; and &lt;supporting details&gt;."
- `scope_type` (text, optional)
  - e.g. `system`, `project`, `user`; determines visibility and which scope_id applies
- `scope_id` (uuid, nullable)
  - e.g. project_id or user_id when scope_type is project or user
- `default_skill_ids` (jsonb, nullable)
  - optional array of skill stable identifiers; when present, the job builder resolves and includes them in context supplied to the SBA (merged with task recommended_skill_ids; union, task overrides duplicates)
- `recommended_cloud_models` (jsonb, nullable)
  - optional map keyed by provider (e.g. openai, anthropic), value = array of model stable identifiers; orchestrator uses this to select a cloud model when the job uses this persona
- `recommended_local_model_ids` (jsonb, nullable)
  - optional array of model stable identifiers for worker-node inference; orchestrator uses this together with node availability to select a local model
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `created_by` (uuid, fk to `users.id`, nullable)

Orchestrator-seeded system personas (e.g. PMA, PAA, developer-go, test-engineer) MAY be updated on release; admin-created system personas and user/project/group-scoped personas MUST NOT be modified by release updates (implementation MUST distinguish seeded vs admin-created, e.g. by flag or convention).

#### Personas Table Constraints

- Index: (`scope_type`, `scope_id`)
- Index: (`created_at`)

## Tasks, Jobs, and Nodes

The orchestrator owns task state and a queue of jobs backed by PostgreSQL.
Nodes register with the orchestrator and report capabilities; the orchestrator stores node registry and optional capability snapshot.

Sources: [`docs/tech_specs/orchestrator.md`](orchestrator.md), [`docs/tech_specs/worker_node.md`](worker_node.md), [`docs/tech_specs/langgraph_mvp.md`](langgraph_mvp.md).

### Task vs Job (Terminology)

- Spec ID: `CYNAI.SCHEMA.TaskVsJob` <a id="spec-cynai-schema-taskvsjob"></a>

Canonical definitions so specs and implementations use consistent language.

- **Task:** A **durable work item** owned by the orchestrator and stored in the `tasks` table.
  A task is the unit of work that users and the PMA create, assign to a plan, give a persona and optional recommended skills, and order with dependencies.
  It describes *what* to do and *who* does it (one persona per task).
  A task outlives any single execution; it can be run, retried, or reassigned.
- **Job:** A **single execution unit** dispatched to a worker, stored in the `jobs` table.
  A job is the runtime instance: "run this work on this node (and in this sandbox)."
  The job payload is what the worker and SBA actually execute (e.g. job spec with persona embedded inline, context, and for bundles task_ids plus embedded full task context so the job is self-contained).
  A job is created when work is dispatched and completes (or fails); the task record is the durable authority.
- **Relationship:** The **job builder** (orchestrator or PMA) turns one or more **tasks** into a **job spec** (resolve persona, merge skills, supply per-task context).
  One **task** may result in zero or more **jobs** over time (e.g. retries, reassignment).
  One **job** may reference one task (single-task job) or 1-3 tasks in order (bundle job); the SBA runs the job and executes each referenced task in sequence when it is a bundle.
  So: **task** = durable, persona-scoped work definition; **job** = one dispatched run of that work on a worker, carrying the resolved persona and context.

### Tasks Table

- Spec ID: `CYNAI.SCHEMA.TasksTable` <a id="spec-cynai-schema-taskstable"></a>

- `id` (uuid, pk)
- `created_by` (uuid, fk to `users.id`)
  - creating user; set from authenticated request context when created via the gateway; for system-created and bootstrap tasks, use the reserved system user
- `project_id` (uuid, fk to `projects.id`, nullable)
  - optional project association for RBAC, preferences, and grouping; null unless explicitly set by client or PM/PA
- `plan_id` (uuid, fk to `project_plans.id`, nullable)
  - when set, task belongs to this plan; workflow for this task is gated on plan state active and on task dependencies (see [Task dependencies](#task-dependencies-table)).
- `planning_state` (text, NOT NULL)
  - Planning phase state; distinct from `status` (execution lifecycle).
  - Allowed values: `draft`, `ready`.
  - Initial value on create: `draft`.
  - Only tasks in `planning_state=ready` are eligible for workflow execution; see [REQ-ORCHES-0178](../requirements/orches.md#req-orches-0178).
- `persona_id` (uuid, fk to `personas.id`, nullable)
  - optional; when set, the job builder resolves this persona and embeds it in the job; at most one persona per task
- `recommended_skill_ids` (jsonb, nullable)
  - optional array of skill stable identifiers; job builder merges with persona default_skill_ids (union, task overrides duplicates) and resolves into context for the SBA
- `status` (text)
  - Task lifecycle status; stored separately from open/closed.
  - Values include: pending, running, completed, failed, canceled, superseded (see [Task status and closed state](#task-status-and-closed-state)).
- `closed` (boolean, not null)
  - Binary open/closed state; when true, the task is closed (no further work).
    MUST be set consistently when status changes (e.g. true when status is completed, failed, canceled, superseded).
- `description` (text, nullable)
  - task description for user-facing display and editing; MUST be stored as Markdown (see [REQ-PROJCT-0114](../requirements/projct.md#req-projct-0114))
- `acceptance_criteria` (jsonb, nullable)
  - structured criteria used by Project Manager for verification; any text fields used for editable criteria MUST be stored as Markdown
- `requirements` (jsonb, nullable)
  - optional array of **requirement** objects; see [Requirement object structure](#spec-cynai-schema-requirementobject)
- `steps` (jsonb, NOT NULL)
  - required **map** of step objects keyed by numeric step ID (integer, stored as JSON number or string that parses to integer); MUST be non-empty.
    Keys define order: when read, steps MUST be sorted by numeric key ascending to obtain deterministic order.
    When creating steps, assign IDs in increments of 10 (e.g. 10, 20, 30) so additional steps can be inserted between (e.g. 15) without renumbering.
    Each value is a **step** object: `complete` (boolean), `description` (string).
    Job builders or agents use the sorted sequence for job context or to-dos; executors set `complete: true` as steps finish.
    Structure may otherwise align with step-executor or SBA job step types (see [cynode_step_executor.md](cynode_step_executor.md), [cynode_sba.md](cynode_sba.md)).
- `summary` (text, nullable)
  - final summary written by workflow
- `post_execution_notes` (text, nullable)
  - Markdown notes added after task execution (e.g. by PMA, PAA, or user); for verification, handoff, or retrospective
- `comments` (jsonb, nullable)
  - same structure as plan comments; see [Comments Structure (Plans and Tasks)](projects_and_scopes.md#spec-cynai-schema-commentsstructure); may be updated by agents or users when plan is locked (per lock rules)
- `metadata` (jsonb, nullable)
- `archived_at` (timestamptz, nullable)
  - when non-null, the task is archived (soft-deleted); API/CLI "delete" sets this; archived tasks are excluded from default list views; retained for audit and history
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`created_by`)
- Index: (`project_id`)
- Index: (`plan_id`)
- Index: (`plan_id`)
- Index: (`persona_id`) where not null
- Index: (`planning_state`)
- Index: (`status`)
- Index: (`closed`)
- Index: (`archived_at`) where not null (for list filter by archived)
- Index: (`created_at`)

#### Task `planning_state` Migration

When adding `planning_state` to an existing deployment, existing tasks MUST be assigned a value:

- If `tasks.status` is `running`, `completed`, `failed`, `canceled`, or `superseded`, set `planning_state=ready`.
- If `tasks.status` is `pending`, set `planning_state=draft` or `ready` per deployment choice (e.g. treat existing pending tasks as already reviewed).

#### Requirement Object Structure

- Spec ID: `CYNAI.SCHEMA.RequirementObject` <a id="spec-cynai-schema-requirementobject"></a>

The `tasks.requirements` column holds a JSON array of **requirement** objects (or null).
Each object has:

- `ref` (string, optional): stable content reference for the requirement (e.g. `REQ-PROJCT-0122` or a task-local tag); used for display and sorting, not a database entity id
- `description` (string, required): the requirement statement; MUST be stored as Markdown
- `source` (string, optional): provenance (e.g. requirement document path, spec section, or external ref)
- `type` (string, optional): e.g. `functional`, `non_functional`, `constraint`, or host-defined
- `priority` (string or number, optional): e.g. `must` | `should` | `could`, or numeric 1-5; host-defined semantics

Order in the array is significant unless otherwise specified; consumers MAY preserve or sort by `ref` for display.

#### Task Dependencies Table

- Spec ID: `CYNAI.SCHEMA.TaskDependenciesTable` <a id="spec-cynai-schema-taskdependenciestable"></a>

Stores explicit task-within-plan dependencies; execution order and runnability are determined solely by the dependency graph (prerequisite and dependent tasks).
**Multiple prerequisites:** A task MAY depend on **multiple** other tasks; the table stores one row per (dependent task, prerequisite task).
Thus `task_id` may appear in many rows with different `depends_on_task_id` values.
The orchestrator and PMA use this structure to resolve all prerequisites for a task and to surface runnable tasks (see below).
When a task is set to `canceled`, all tasks that depend on it (directly or transitively) MUST be set to `canceled` automatically; see [REQ-ORCHES-0154](../requirements/orches.md#req-orches-0154) and [Cancel cascades to dependents](langgraph_mvp.md#spec-cynai-orches-cancelcascadestodependents).
A task is **runnable** when all tasks it depends on have `status = 'completed'`; see [Project plan and task dependencies](langgraph_mvp.md#spec-cynai-orches-workflowplanorder) and [REQ-ORCHES-0153](../requirements/orches.md#req-orches-0153).

- `id` (uuid, pk)
- `task_id` (uuid, fk to `tasks.id`, NOT NULL)
  - the dependent task (this task runs after its dependencies)
- `depends_on_task_id` (uuid, fk to `tasks.id`, NOT NULL)
  - one prerequisite that must reach status `completed` before `task_id` may run; multiple prerequisites for the same task are represented by multiple rows with the same `task_id`

Constraints

- Unique: (`task_id`, `depends_on_task_id`)
- Check: `task_id != depends_on_task_id` (no self-deps)
- Application MUST ensure both tasks belong to the same plan (`tasks.plan_id` equal for both) when plan_id is set; optionally enforce via trigger or constraint.

Indexes

- Index: (`task_id`) for "list all prerequisites of this task" (needed to decide if task is runnable)
- Index: (`depends_on_task_id`) for "list all tasks that depend on this one" (e.g. cascade cancel, or "what becomes runnable when this completes")

**Surfacing runnable tasks:** The orchestrator and PMA MUST be able to query tasks that can be executed (runnable) for a given plan.
A task is runnable when: (1) its plan's state is `active`, (2) the task is not closed, and (3) either the task has **no** rows in `task_dependencies` for this `task_id`, or **every** row for this `task_id` has `depends_on_task_id` pointing to a task with `status = 'completed'`.
Implementations SHOULD support an efficient query (e.g. tasks in plan where not closed and either no dependency rows exist for that task_id, or the set of depends_on_task_id for that task_id is a subset of completed task ids).
The indexes above support "load dependencies of task" and "check all completed"; additional indexing (e.g. on `tasks.plan_id`, `tasks.closed`) supports "all runnable tasks for plan" patterns.

#### Task Status and Closed State

- Spec ID: `CYNAI.SCHEMA.TaskStatusAndClosed` <a id="spec-cynai-schema-taskstatusandclosed"></a>

Task **status** is stored in `tasks.status` and represents the lifecycle state (e.g. pending, running, completed, failed, canceled, superseded).
Task **closed** is stored in `tasks.closed` (boolean): when true, the task is closed (no further work); when false, the task is open.
The system MUST keep `closed` consistent with `status` (e.g. set `closed = true` when status becomes completed, failed, canceled, or superseded).
Plan completion (set plan to completed) requires the plan to have at least one task and **all such tasks to have `closed = true`**; see [REQ-PROJCT-0121](../requirements/projct.md#req-projct-0121) and [Project plan state](projects_and_scopes.md#spec-cynai-access-projectplanstate).

### Jobs Table

- Spec ID: `CYNAI.SCHEMA.JobsTable` <a id="spec-cynai-schema-jobstable"></a>

- `id` (uuid, pk)
- `task_ids` (jsonb, NOT NULL)
  - map keyed by numeric order (e.g. 10, 20, 30), value = task uuid (string); single-task job = one key; bundle = 2-3 keys; execution order = sort keys ascending; same pattern as task steps
- `task_id` (uuid, fk to `tasks.id`, nullable)
  - deprecated or derived: when task_ids has a single key, may duplicate that task id for backward compatibility; prefer task_ids for new code
- `persona_id` (uuid, fk to `personas.id`, nullable)
  - optional; for indexing, reporting, and provenance; job payload carries inline `persona: { title, description }` for SBA consumption
- `node_id` (uuid, fk to `nodes.id`, nullable)
  - set when job is dispatched to a node
- `status` (text)
  - examples: queued, running, completed, failed, canceled, lease_expired
- `payload` (jsonb, nullable)
  - job input (e.g. command, image, env)
- `result` (jsonb, nullable)
  - job output and exit info
- `lease_id` (uuid, nullable)
  - idempotency / lease for retries and heartbeats
- `lease_expires_at` (timestamptz, nullable)
- `started_at` (timestamptz, nullable)
- `ended_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`task_ids`) (GIN or application-level by first task id when single-task)
- Index: (`task_id`)
- Index: (`persona_id`) where not null
- Index: (`node_id`)
- Index: (`status`)
- Index: (`lease_id`) where not null
- Index: (`lease_expires_at`) where not null
- Index: (`created_at`)

For bundle jobs, the payload MUST embed full per-task context (context.task_contexts keyed by same numeric keys as task_ids) so the job is self-contained; see [cynode_sba.md](cynode_sba.md).

### Nodes Table

- `id` (uuid, pk)
- `node_slug` (text, unique)
  - stable identifier used in registration and scheduling (e.g. from node startup YAML)
- `status` (text)
  - examples: registered, active, inactive, drained
- `config_version` (text, nullable)
  - version of last applied node configuration payload
- `worker_api_target_url` (text, nullable)
  - URL of the node Worker API for job dispatch; normally set from the node-reported `worker_api.base_url` at registration and when processing capability reports; may be overridden by operator config (e.g. same-host override); see [`worker_node_payloads.md`](worker_node_payloads.md) and [`worker_node.md`](worker_node.md)
- `worker_api_bearer_token` (text, nullable)
  - bearer token for orchestrator-to-node Worker API auth; MUST be stored encrypted at rest or in a secrets backend; populated from config delivery
- `config_ack_at` (timestamptz, nullable)
  - time of last config acknowledgement from the node
- `config_ack_status` (text, nullable)
  - status of last config ack: e.g. applied, failed
- `config_ack_error` (text, nullable)
  - error message when config_ack_status is failed
- `last_seen_at` (timestamptz, nullable)
- `last_capability_at` (timestamptz, nullable)
- `metadata` (jsonb, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`node_slug`)
- Index: (`status`)
- Index: (`last_seen_at`)

### Node Capabilities Table

Stores the last reported capability payload (full JSON of actual capabilities) for scheduling and display.
The orchestrator MUST store the capability report JSON here (or a normalized snapshot that preserves the actual capabilities per worker_node.md).

- `id` (uuid, pk)
- `node_id` (uuid, fk to `nodes.id`)
- `reported_at` (timestamptz)
- `capability_snapshot` (jsonb)
  - full capability report JSON (identity, platform, compute, gpu, sandbox, network, inference, tls, etc. per worker_node_payloads.md)

Constraints

- Unique: (`node_id`) (one row per node; overwrite on new report) or allow history (then index by `node_id`, `reported_at`)
- Index: (`node_id`)

Recommendation: one row per node, updated in place when capability report is received; alternatively, append-only with retention policy.

## Workflow Checkpoints

The LangGraph workflow engine persists checkpoint state to PostgreSQL so that workflows can resume after restarts.
The orchestrator MUST NOT run workflow steps without going through this checkpoint layer.

Source: [langgraph_mvp.md](langgraph_mvp.md) Checkpoint schema (prescriptive).

### Workflow Checkpoints Table

Table name: `workflow_checkpoints`.

- `id` (uuid, pk)
- `task_id` (uuid, fk to `tasks.id`, unique)
  - one row per task for the current checkpoint; upsert by task_id on each persist
- `state` (jsonb)
  - full state model: task_id, acceptance_criteria, preferences_effective, plan, current_step_index, attempts_by_step, last_result, verification (see [langgraph_mvp.md](langgraph_mvp.md) State Model)
- `last_node_id` (text)
  - identity of the last completed graph node
- `updated_at` (timestamptz)

Constraints

- Unique: (`task_id`)
- Index: (`task_id`)
- Index: (`updated_at`)

### Task Workflow Leases Table

Table name: `task_workflow_leases`.

The orchestrator grants and releases this lease; the workflow runner acquires or checks it via the orchestrator API.
Only one active workflow per task is allowed; the lease enforces that.

- `id` (uuid, pk)
- `task_id` (uuid, fk to `tasks.id`, unique)
  - one lease row per task
- `lease_id` (uuid)
  - idempotency/identity for the lease
- `holder_id` (text, nullable)
  - workflow runner instance identifier holding the lease
- `expires_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Unique: (`task_id`)
- Index: (`task_id`)
- Index: (`expires_at`) where not null

## Sandbox Image Registry

- Spec ID: `CYNAI.SCHEMA.SandboxImageRegistry` <a id="spec-cynai-schema-sandboximageregistry"></a>

Allowed sandbox images and their capabilities MUST be stored in PostgreSQL.
This enables agents and schedulers to pick safe, policy-approved execution environments.

Source: [`docs/tech_specs/sandbox_image_registry.md`](sandbox_image_registry.md).

### Sandbox Images Table

Table name: `sandbox_images`.

- `id` (uuid, pk)
- `name` (text)
  - logical image name (e.g. python-tools, node-build, secops)
- `description` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`name`)
- Index: (`name`)

### Sandbox Image Versions Table

Table name: `sandbox_image_versions`.

- `id` (uuid, pk)
- `sandbox_image_id` (uuid, fk to `sandbox_images.id`)
- `version` (text)
  - tag or semantic version
- `image_ref` (text)
  - OCI reference including registry and repository
- `image_digest` (text, nullable)
  - digest for pinning (recommended)
- `capabilities` (jsonb)
  - examples: runtimes, tools, network_requirements, filesystem_requirements; SHOULD include `agent_compatible` (boolean) when known, from OCI label `io.cynodeai.sandbox.agent-compatible` at ingest
- `is_allowed` (boolean)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Unique: (`sandbox_image_id`, `version`)
- Index: (`sandbox_image_id`)
- Index: (`is_allowed`)

### Node Sandbox Image Availability Table

Table name: `node_sandbox_image_availability`.

- `id` (uuid, pk)
- `node_id` (uuid, fk to `nodes.id`)
- `sandbox_image_version_id` (uuid, fk to `sandbox_image_versions.id`)
- `status` (text)
  - examples: available, pulling, failed, evicted
- `last_checked_at` (timestamptz)
- `details` (jsonb, nullable)

Constraints

- Unique: (`node_id`, `sandbox_image_version_id`)
- Index: (`node_id`)

## Runs and Sessions

- Spec ID: `CYNAI.SCHEMA.RunsSessions` <a id="spec-cynai-schema-runssessions"></a>

Runs are execution traces (workflow instance, dispatched job, or agent turn).
Sessions are user-facing containers that group runs and hold transcripts.

Source: [`docs/tech_specs/runs_and_sessions_api.md`](runs_and_sessions_api.md).

### Runs Table

- `id` (uuid, pk)
- `task_id` (uuid, fk to `tasks.id`)
- `job_id` (uuid, fk to `jobs.id`, nullable)
- `session_id` (uuid, fk to `sessions.id`, nullable)
- `parent_run_id` (uuid, fk to `runs.id`, nullable)
- `status` (text)
  - examples: pending, running, completed, failed, canceled
- `started_at` (timestamptz, nullable)
- `ended_at` (timestamptz, nullable)
- `metadata` (jsonb, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`task_id`)
- Index: (`job_id`)
- Index: (`session_id`)
- Index: (`parent_run_id`)
- Index: (`status`)
- Index: (`created_at`)

### Sessions Table

- `id` (uuid, pk)
- `parent_session_id` (uuid, fk to `sessions.id`, nullable)
- `user_id` (uuid, fk to `users.id`)
- `title` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`user_id`)
- Index: (`parent_session_id`)
- Index: (`created_at`)

## Chat Threads and Messages

Chat threads and chat messages store chat history separately from task lifecycle state.
Chat message content MUST be the amended (redacted) content.
Plaintext secrets MUST NOT be persisted in chat message content.

### Source Documents

- [`docs/tech_specs/chat_threads_and_messages.md`](chat_threads_and_messages.md)
- [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md)

### Chat Threads Table

- `id` (uuid, pk)
- `user_id` (uuid, fk to `users.id`)
- `project_id` (uuid, fk to `projects.id`, nullable)
- `session_id` (uuid, fk to `sessions.id`, nullable)
- `title` (text, nullable)
- `summary` (text, nullable)
- `archived_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`user_id`, `updated_at`)
- Index: (`project_id`, `updated_at`)

### Chat Messages Table

- `id` (uuid, pk)
- `thread_id` (uuid, fk to `chat_threads.id`)
- `role` (text)
  - examples: user, assistant, system
- `content` (text)
- `metadata` (jsonb, nullable)
- `created_at` (timestamptz)

Constraints

- Index: (`thread_id`, `created_at`)

### Chat Message Attachments Table

- Spec ID: `CYNAI.SCHEMA.ChatMessageAttachmentsTable` <a id="spec-cynai-schema-chatmessageattachmentstable"></a>

#### Chat Message Attachments Table Requirements Traces

- [REQ-SCHEMA-0114](../requirements/schema.md#req-schema-0114)

This table stores user-message file uploads or stable file references accepted through the chat `@`-reference workflow.
When the originating chat thread is project-scoped, rows in this table inherit the same project authorization boundary as the thread and message they attach to.

- `id` (uuid, pk)
- `message_id` (uuid, fk to `chat_messages.id`)
- `thread_id` (uuid, fk to `chat_threads.id`)
- `user_id` (uuid, fk to `users.id`)
- `file_id` (text)
  - stable internal identifier returned by the upload layer
- `filename` (text)
- `media_type` (text, nullable)
- `size_bytes` (bigint, nullable)
- `storage_ref` (text)
  - internal blob or object-storage reference
- `checksum_sha256` (text, nullable)
- `created_at` (timestamptz)

#### Chat Message Attachments Table Constraints

- Index: (`message_id`)
- Index: (`thread_id`, `created_at`)
- Index: (`user_id`, `created_at`)
- Unique: (`message_id`, `file_id`)
- Access to attachment rows and referenced storage MUST be enforced through the parent chat thread and message authorization, including shared-project permissions when the thread belongs to a shared project.

## Task Artifacts

- Spec ID: `CYNAI.SCHEMA.TaskArtifacts` <a id="spec-cynai-schema-taskartifacts"></a>

Artifacts are files or blobs produced or attached to a task (e.g. uploads, job outputs).
Metadata is stored in PostgreSQL; large content may be stored in object storage or node-local staging with a reference.

- `id` (uuid, pk)
- `task_id` (uuid, fk to `tasks.id`)
- `run_id` (uuid, fk to `runs.id`, nullable)
- `path` (text)
  - logical path or key within the task (e.g. `output/report.md`, `upload/input.csv`)
- `storage_ref` (text)
  - reference to blob or file (e.g. object key, node path, or inline small content indicator)
- `size_bytes` (bigint, nullable)
- `content_type` (text, nullable)
- `checksum_sha256` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

### Task Artifacts Constraints

- Unique: (`task_id`, `path`)
- Index: (`task_id`)
- Index: (`run_id`)
- Index: (`created_at`)

## Vector Storage (`pgvector`)

CyNodeAI uses PostgreSQL and pgvector for vector storage and similarity search.
Vector storage is used to support retrieval and semantic search over task-related content.

### Vector Storage Applicable Requirements

- Spec ID: `CYNAI.SCHEMA.VectorStorage` <a id="spec-cynai-schema-vectorstorage"></a>

#### Recommended Behavior

- Prefer cosine distance for similarity search.
- Use an approximate index (HNSW when available; otherwise IVFFLAT) to keep queries fast at scale.
- Store only sanitized, policy-allowed content in vector storage.
- Keep similarity search queries isolated in a repository layer so they can be tuned without changing callers.

#### Vector Storage Applicable Requirements Requirements Traces

- [REQ-SCHEMA-0106](../requirements/schema.md#req-schema-0106)
- [REQ-SCHEMA-0107](../requirements/schema.md#req-schema-0107)
- [REQ-SCHEMA-0108](../requirements/schema.md#req-schema-0108)
- [REQ-SCHEMA-0109](../requirements/schema.md#req-schema-0109)
- [REQ-SCHEMA-0110](../requirements/schema.md#req-schema-0110)
- [REQ-SCHEMA-0111](../requirements/schema.md#req-schema-0111)
- [REQ-ACCESS-0125](../requirements/access.md#req-access-0125)

### Vector Items Table

Table name: `vector_items`.

This table stores chunked text content and its embedding.
It is intentionally generic so multiple sources can be indexed (artifacts, run logs, connector documents, sanitized web pages).

- `id` (uuid, pk)
- `project_id` (uuid, fk to `projects.id`, nullable)
- `task_id` (uuid, fk to `tasks.id`, nullable)
- `namespace` (text, nullable)
  - coarse-grained policy boundary (e.g. docs, skills, project_memory, code_index); used for RBAC filtering
- `sensitivity_level` (text, nullable)
  - ordered level for role-based filtering (e.g. public, internal, confidential, restricted); query MUST enforce chunk.sensitivity_level <= role.max_sensitivity_level
- `source_type` (text)
  - examples: task_artifact, run_log, connector_doc, web_page, note
- `source_ref` (text, nullable)
  - stable identifier for the source (e.g. artifact path, URL, connector id)
- `chunk_index` (int)
- `content_text` (text)
- `content_sha256` (text)
- `embedding_model` (text)
  - examples: text-embedding-3-small, bge-m3, nomic-embed-text
- `embedding_dim` (int)
- `embedding` (vector(1536))
  - dimension is an example and MUST be set to the system's configured embedding dimension
  - the configured embedding dimension MUST match `embedding_dim`
- `metadata` (jsonb, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`project_id`)
- Index: (`project_id`, `namespace`) for RBAC-filtered similarity queries
- Index: (`task_id`)
- Index: (`source_type`)
- Index: (`content_sha256`)

Indexing guidance

- Create a pgvector index on `embedding` using cosine operators.
- The index SHOULD be paired with filters (for example `task_id`) to avoid cross-scope retrieval.
- Index type and parameters MUST be explicit and versioned in migrations.
- Use IVFFLAT for approximate search when HNSW is unavailable.
- Use HNSW when pgvector supports it and it meets performance requirements for the expected dataset size.

Example index definitions

```sql
-- IVFFLAT (approximate).
-- Tune lists and probes based on dataset size and recall requirements.
CREATE INDEX CONCURRENTLY IF NOT EXISTS vector_items_embedding_ivfflat
ON vector_items USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100);

-- HNSW (newer pgvector).
-- Tune m and ef_construction based on dataset size and recall requirements.
CREATE INDEX CONCURRENTLY IF NOT EXISTS vector_items_embedding_hnsw
ON vector_items USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 200);
```

### Vector Retrieval and RBAC

- Spec ID: `CYNAI.SCHEMA.VectorRetrievalRbac` <a id="spec-cynai-schema-vectorretrievalrbac"></a>

Vector retrieval MUST NOT bypass RBAC.
Similarity search is only allowed within an already-authorized document set; authorization MUST be applied in SQL before similarity ranking.

#### Vector Retrieval and RBAC Requirements Traces

- [REQ-ACCESS-0121](../requirements/access.md#req-access-0121)
- [REQ-ACCESS-0122](../requirements/access.md#req-access-0122)
- [REQ-ACCESS-0123](../requirements/access.md#req-access-0123)
- [REQ-ACCESS-0124](../requirements/access.md#req-access-0124)
- [REQ-SCHEMA-0111](../requirements/schema.md#req-schema-0111)
- [REQ-SCHEMA-0112](../requirements/schema.md#req-schema-0112)

#### Vector Query Flow

1. Authenticate the caller and resolve effective permissions (allowed project_ids, allowed namespaces, max_sensitivity_level for the role).
2. Build the candidate set with explicit filters: WHERE project_id IN (authorized_projects), AND namespace IN (authorized_namespaces), AND (sensitivity_level IS NULL OR sensitivity_level <= allowed_max).
3. Run similarity ranking only against the filtered candidate set (e.g. ORDER BY embedding <=> query_embedding LIMIT top_k).
4. Return results with provenance metadata; do not return full document bodies or hidden metadata beyond chunk text and allowed provenance.

RBAC filtering MUST occur in SQL before similarity scoring.
No "open" vector queries are allowed; every query MUST include explicit project/namespace/sensitivity constraints derived from the authenticated subject.

#### Vector Ingestion

- Only controlled services may insert into vector storage; ingestion MUST require write permission on the target scope (project, namespace) and correct project association per [REQ-ACCESS-0125](../requirements/access.md#req-access-0125).

#### Vector Audit

- Every retrieval MUST be logged (e.g. user_id, role, project_id, namespaces queried, chunk count returned, timestamp) per [REQ-ACCESS-0124](../requirements/access.md#req-access-0124).

#### Vector Performance

- Use composite indexes on (project_id, namespace) so that filtering reduces the candidate set before similarity search; similarity search SHOULD run only against already filtered rows.

## Audit Logging

- Spec ID: `CYNAI.SCHEMA.AuditLogging` <a id="spec-cynai-schema-auditlogging"></a>

Audit logging is implemented via domain-specific tables so that retention and query patterns can be tuned per domain.
All audit tables MUST use `timestamptz` for event time and SHOULD include subject and decision/outcome where applicable.

### Auth Audit Log Table

All authentication events MUST be audit logged.

Source: [`docs/tech_specs/local_user_accounts.md`](local_user_accounts.md).

- `id` (uuid, pk)
- `event_type` (text)
  - examples: login_success, login_failure, refresh_success, refresh_failure, logout, session_revoked, user_created, user_disabled, user_reenabled, password_changed, password_reset
- `user_id` (uuid, nullable)
  - null for pre-auth failures
- `subject_handle` (text, nullable)
  - redacted or hashed for failure events if needed
- `success` (boolean)
- `ip_address` (inet, nullable)
- `user_agent` (text, nullable)
- `reason` (text, nullable)
- `created_at` (timestamptz)

Constraints

- Index: (`created_at`)
- Index: (`user_id`)
- Index: (`event_type`)

### MCP Tool Call Audit Log Table

- Spec ID: `CYNAI.SCHEMA.McpToolCallAuditLog` <a id="spec-cynai-schema-mcptoolcallauditlog"></a>

This table stores append-only metadata for MCP tool calls routed by the orchestrator gateway.
Tool arguments and tool results are not stored in this table for MVP.

Source: [`docs/tech_specs/mcp_tool_call_auditing.md`](mcp_tool_call_auditing.md).

- `id` (uuid, pk)
- `created_at` (timestamptz)
- `task_id` (uuid, fk to `tasks.id`, nullable)
- `project_id` (uuid, fk to `projects.id`, nullable)
- `run_id` (uuid, fk to `runs.id`, nullable)
- `job_id` (uuid, fk to `jobs.id`, nullable)
- `subject_type` (text, nullable)
- `subject_id` (uuid, nullable)
- `user_id` (uuid, fk to `users.id`, nullable)
- `group_ids` (jsonb, nullable)
  - array of uuid
- `role_names` (jsonb, nullable)
  - array of string
- `tool_name` (text)
- `decision` (text)
  - allow or deny
- `status` (text)
  - success or error
- `duration_ms` (int, nullable)
- `error_type` (text, nullable)

Constraints

- Index: (`created_at`)
- Index: (`task_id`)
- Index: (`project_id`)
- Index: (`tool_name`)

### Other Audit Tables

- **Access control:** `access_control_audit_log` (see Access Control).
- **Preferences:** `preference_audit_log` (see Preferences).
- **Interactive chat:** `chat_audit_log` (see OpenAI-compatible chat API and chat threads/messages).

Additional domain-specific audit tables (e.g. MCP tool calls, connector operations, Git egress) MAY be added later; they SHOULD include at least `task_id` (nullable), subject identity, action, decision or outcome, and `created_at`.

### Chat Audit Log Table

This table stores audit records for OpenAI-compatible interactive chat requests.
It MUST NOT store full message content.

- `id` (uuid, pk)
- `created_at` (timestamptz)
- `user_id` (uuid, fk to `users.id`, nullable)
- `project_id` (uuid, fk to `projects.id`, nullable)
- `outcome` (text)
  - examples: success, error, canceled, timeout
- `error_code` (text, nullable)
- `redaction_applied` (boolean)
- `redaction_kinds` (jsonb, nullable)
  - array of string
  - examples: api_key, token, password
- `duration_ms` (int, nullable)
- `request_id` (text, nullable)

Constraints

- Index: (`created_at`)
- Index: (`user_id`)
- Index: (`project_id`)

## Model Registry

- Spec ID: `CYNAI.SCHEMA.ModelRegistry` <a id="spec-cynai-schema-modelregistry"></a>

Model metadata and node-model availability.
Optional for MVP; required when model management and node load workflow are implemented.

Source: [`docs/tech_specs/model_management.md`](model_management.md).

### Models Table

- `id` (uuid, pk)
- `name` (text)
- `vendor` (text, nullable)
- `description` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`name`, `vendor`)
- Index: (`name`)

### Model Versions Table

- `id` (uuid, pk)
- `model_id` (uuid, fk to `models.id`)
- `version` (text)
- `capabilities` (jsonb)
- `default_parameters` (jsonb, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Unique: (`model_id`, `version`)
- Index: (`model_id`)

### Model Artifacts Table

- `id` (uuid, pk)
- `model_version_id` (uuid, fk to `model_versions.id`)
- `artifact_type` (text)
- `sha256` (text)
- `size_bytes` (bigint)
- `cache_path` (text)
- `source_uri` (text, nullable)
- `created_at` (timestamptz)

Constraints

- Unique: (`sha256`)
- Index: (`model_version_id`)

### Node Model Availability Table

- `id` (uuid, pk)
- `node_id` (uuid, fk to `nodes.id`)
- `model_version_id` (uuid, fk to `model_versions.id`)
- `status` (text)
  - examples: available, loading, failed, evicted
- `last_checked_at` (timestamptz)
- `details` (jsonb, nullable)

Constraints

- Unique: (`node_id`, `model_version_id`)
- Index: (`node_id`)

## Table Summary and Dependencies

This section summarizes table creation order and naming conventions.

### Creation Order

Creation order (respecting foreign keys):

1. `users`
2. `projects`, `project_plans`, `project_plan_revisions`, `project_git_repos`, `groups`, `roles`
3. `password_credentials`, `refresh_sessions`, `group_memberships`, `role_bindings`
4. `access_control_rules`
5. `api_credentials`
6. `preference_entries`
7. `nodes`, `sandbox_images`
8. `tasks`, `task_dependencies`, `sessions`
9. `sandbox_image_versions`, `jobs`, `node_capabilities`, `workflow_checkpoints`, `task_workflow_leases`
10. `runs`, `task_artifacts`, `node_sandbox_image_availability`, `vector_items`
11. `auth_audit_log`, `mcp_tool_call_audit_log`, `access_control_audit_log`, `preference_audit_log`
12. Model registry (if used): `models`, `model_versions`, `model_artifacts`, `node_model_availability`

### Naming Conventions

- Table names: `snake_case`, plural (e.g. `refresh_sessions`, `task_artifacts`).
- Primary key: `id` (uuid).
- Foreign keys: `{table_singular}_id` (e.g. `user_id`, `task_id`).
- Timestamps: `created_at`, `updated_at` (timestamptz); event tables use `created_at` or domain-specific names (e.g. `changed_at`, `reported_at`).
