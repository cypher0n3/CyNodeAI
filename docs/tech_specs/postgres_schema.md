# Postgres Schema

- [Document Overview](#document-overview)
- [Goals and Scope](#goals-and-scope)
- [Storing This Schema in Code](#storing-this-schema-in-code)
  - [Storing This Schema in Code Applicable Requirements](#storing-this-schema-in-code-applicable-requirements)
- [Schema Overview](#schema-overview)
- [Identity and Authentication](#identity-and-authentication)
  - [Users Table](#users-table)
  - [Password Credentials Table](#password-credentials-table)
  - [Refresh Sessions Table](#refresh-sessions-table)
- [Projects](#projects)
  - [Projects Table](#projects-table)
  - [Project Plans Table](#project-plans-table)
  - [Project Plan Revisions Table](#project-plan-revisions-table)
  - [Project Git Repositories Table](#project-git-repositories-table)
- [Groups and RBAC](#groups-and-rbac)
  - [Groups Table](#groups-table)
  - [Group Memberships Table](#group-memberships-table)
  - [Roles Table](#roles-table)
  - [Role Bindings Table](#role-bindings-table)
- [Access Control](#access-control)
  - [Access Control Rules Table](#access-control-rules-table)
  - [Access Control Audit Log Table](#access-control-audit-log-table)
- [API Egress Credentials](#api-egress-credentials)
  - [API Credentials Table](#api-credentials-table)
- [Preferences](#preferences)
  - [Preference Entries Table](#preference-entries-table)
  - [Preference Audit Log Table](#preference-audit-log-table)
- [System Settings](#system-settings)
  - [System Settings Table](#system-settings-table)
  - [System Settings Audit Log Table](#system-settings-audit-log-table)
- [Personas](#personas)
  - [Personas Table](#personas-table)
- [Tasks, Jobs, and Nodes](#tasks-jobs-and-nodes)
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

Source of truth

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

#### Traces to Requirements

- [REQ-SCHEMA-0100](../requirements/schema.md#req-schema-0100)
- [REQ-SCHEMA-0101](../requirements/schema.md#req-schema-0101)
- [REQ-SCHEMA-0102](../requirements/schema.md#req-schema-0102)
- [REQ-SCHEMA-0103](../requirements/schema.md#req-schema-0103)
- [REQ-SCHEMA-0104](../requirements/schema.md#req-schema-0104)
- [REQ-SCHEMA-0105](../requirements/schema.md#req-schema-0105)

Recommended repository layout

- `db/`
  - `models/` (GORM models)
  - `ddl/` (idempotent SQL for extensions and advanced indexes/constraints)

Implementation notes

- AutoMigrate is convenient for MVP, but it can drift across versions.
  Prefer explicit version pinning and CI checks that validate the expected schema exists.
- A migration tool/library MAY be used for the DDL bootstrap step, but SQL files should remain committed to the repo.

Out of scope for this document

- Node capability report and node configuration payload wire formats (see [`docs/tech_specs/worker_node.md`](worker_node.md)).
- MCP gateway enforcement and tool allowlists (see [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)).

## Schema Overview

Logical groups

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

The orchestrator MUST store users and local auth state in PostgreSQL.
Credentials and refresh tokens MUST be stored as hashes.

Source: [`docs/tech_specs/local_user_accounts.md`](local_user_accounts.md).

### Users Table

- Spec ID: `CYNAI.SCHEMA.UsersTable` <a id="spec-cynai-schema-userstable"></a>

- `id` (uuid, pk)
- `handle` (text, unique)
- `email` (text, unique, nullable)
- `is_active` (boolean)
- `external_source` (text, nullable)
- `external_id` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`handle`)
- Index: (`email`) where not null
- Index: (`is_active`)

Reserved identities

- The handle `system` is reserved.
  The orchestrator MUST ensure a corresponding `users` row exists (the "system user") and MUST use that user id for attribution when an action is performed by the system and no human actor applies (for example `tasks.created_by` for system-created tasks).
  User creation MUST reject attempts to create or rename a user to `handle=system`.

### Password Credentials Table

- `id` (uuid, pk)
- `user_id` (uuid, fk to `users.id`)
- `password_hash` (bytea)
- `hash_alg` (text)
  - examples: argon2id, bcrypt
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Unique: (`user_id`) (one password credential per user in MVP)
- Index: (`user_id`)

### Refresh Sessions Table

- `id` (uuid, pk)
- `user_id` (uuid, fk to `users.id`)
- `refresh_token_hash` (bytea)
- `refresh_token_kid` (text, nullable)
- `is_active` (boolean)
- `expires_at` (timestamptz)
- `last_used_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`user_id`)
- Index: (`is_active`, `expires_at`)

## Projects

Projects are workspace boundaries used for authorization scope and preference resolution.

Source: [`docs/tech_specs/projects_and_scopes.md`](projects_and_scopes.md).

### Projects Table

- `id` (uuid, pk)
- `slug` (text, unique)
- `display_name` (text)
  - user-friendly title for lists and detail views
- `description` (text, nullable)
  - optional text description for the project
- `is_active` (boolean)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Index: (`slug`)
- Index: (`is_active`)

### Project Plans Table

- Spec ID: `CYNAI.SCHEMA.ProjectPlansTable` <a id="spec-cynai-schema-projectplanstable"></a>

A project MAY have multiple plans; at most one plan per project may be active at a time.
Plan state values: `draft`, `ready`, `active`, `suspended`, `completed`, `canceled` (see [Project plan state](projects_and_scopes.md#spec-cynai-access-projectplanstate)).
**Archived** is a separate boolean flag for UI/API views; archived plans MUST NOT run workflow and MUST NOT be the active plan (enforced by API).

Source: [REQ-PROJCT-0110](../requirements/projct.md#req-projct-0110), [Project plan state](projects_and_scopes.md#spec-cynai-access-projectplanstate), [REQ-PROJCT-0124](../requirements/projct.md#req-projct-0124).

- `id` (uuid, pk)
- `project_id` (uuid, fk to `projects.id`, NOT NULL)
- `plan_name` (text, nullable)
  - optional name for this plan
- `plan_body` (text, nullable)
  - plan document body; MUST be stored as Markdown (see [REQ-PROJCT-0114](../requirements/projct.md#req-projct-0114))
- `state` (text, NOT NULL)
  - one of: `draft`, `ready`, `active`, `suspended`, `completed`, `canceled`
  - only one row per project may have `state = 'active'` (enforced by partial unique index); archived plans MUST NOT have state `active` (API enforces)
- `archived` (boolean, NOT NULL, default false)
  - when true, plan is archived for history/views; workflow MUST NOT run for this plan and this plan MUST NOT be set to active; used by UIs/APIs for filtering and display
- `is_plan_locked` (boolean, default false)
  - when true, plan document (plan_name, plan_body) is read-only until unlocked; API enforces
- `plan_locked_at` (timestamptz, nullable)
- `plan_locked_by` (uuid, fk to `users.id`, nullable)
- `plan_approved_at` (timestamptz, nullable)
  - set when plan is approved (transition to ready or active); who approved and when
- `plan_approved_by` (uuid, fk to `users.id`, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `created_by` (uuid, fk to `users.id`, nullable)

Constraints

- Unique partial: (`project_id`) WHERE `state` = 'active' (at most one active plan per project)
- Index: (`project_id`)
- Index: (`project_id`, `state`)
- Index: (`state`)
- Index: (`archived`) for list/filter by archived

### Project Plan Revisions Table

- Spec ID: `CYNAI.SCHEMA.ProjectPlanRevisionsTable` <a id="spec-cynai-schema-projectplanrevisionstable"></a>

Stores a snapshot of a project plan (document, task list, and task dependencies) each time the plan or its task list or dependencies change so users can view revision history.
One row per revision; version increments per plan.

Source: [REQ-PROJCT-0119](../requirements/projct.md#req-projct-0119), [Plan revisions](projects_and_scopes.md#spec-cynai-access-projectplanrevisions).

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

Constraints

- Unique: (`plan_id`, `version`)
- Index: (`plan_id`, `created_at`)
- Index: (`plan_id`)

Behavior

- The orchestrator or gateway MUST insert a new row into `project_plan_revisions` whenever that plan's `plan_name`, `plan_body`, the set of tasks with that `plan_id`, or the set of task_dependencies for tasks in that plan changes.
- Version MUST be computed as the next integer per plan (e.g. MAX(version)+1 for that plan_id).
- Retention: implementation MAY support configurable retention (e.g. keep last N revisions per plan); minimum is to retain all revisions unless explicitly purged.

### Project Git Repositories Table

- Spec ID: `CYNAI.SCHEMA.ProjectGitReposTable` <a id="spec-cynai-schema-projectgitrepostable"></a>

Stores Git repository associations for projects so that tasks and Git egress can use project-scoped allowlists.

Source: [`docs/tech_specs/project_git_repos.md`](project_git_repos.md).

#### Git Repos Table Columns (Identity and Provider)

- `id` (uuid, pk)
- `project_id` (uuid, fk to `projects.id`, NOT NULL)
- `provider` (text, NOT NULL)
  - identifier for the Git host or service; any provider for which the system has support (e.g. github, gitlab, gitea); additional providers MAY be added without schema change
- `repo_identifier` (text, NOT NULL)
  - provider-specific identifier: for GitHub and Gitea use owner/repo; for GitLab use namespace/project (may include subgroups); semantics defined per provider in the project git repos spec
- `base_url` (text, nullable)
  - optional override for self-hosted instances (e.g. <https://gitea.example.com>, <https://gitlab.company.com>)

#### Git Repos Table Columns (Additional Information)

- `display_name` (text, nullable)
  - optional user-facing label for the repo in this project
- `description` (text, nullable)
  - optional longer description of the repo's role or purpose in this project
- `tags` (jsonb, nullable)
  - optional array of string tags for filtering or grouping (e.g. `["backend", "main"]`); structure is application-defined
- `metadata` (jsonb, nullable)
  - optional key-value data for future extension; no canonical keys required for MVP

#### Git Repos Table Columns (Timestamps)

- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Unique: (`project_id`, `provider`, `repo_identifier`)
- Index: (`project_id`)
- Index: (`provider`, `repo_identifier`) for egress lookups

## Groups and RBAC

- Spec ID: `CYNAI.SCHEMA.GroupsRbac` <a id="spec-cynai-schema-groupsrbac"></a>

The orchestrator MUST track groups, group membership, roles, and role bindings in PostgreSQL.
Policy evaluation and auditing depend on these tables.

Source: [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).

### Groups Table

- `id` (uuid, pk)
- `slug` (text, unique)
- `display_name` (text)
- `is_active` (boolean)
- `external_source` (text, nullable)
- `external_id` (text, nullable)
- `managed_by` (text, nullable)
- `last_synced_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Index: (`slug`)
- Index: (`is_active`)

### Group Memberships Table

- `id` (uuid, pk)
- `group_id` (uuid, fk to `groups.id`)
- `user_id` (uuid, fk to `users.id`)
- `is_active` (boolean)
- `external_source` (text, nullable)
- `external_id` (text, nullable)
- `managed_by` (text, nullable)
- `last_synced_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`group_id`, `user_id`)
- Index: (`group_id`)
- Index: (`user_id`)
- Index: (`is_active`)

### Roles Table

- `id` (uuid, pk)
- `name` (text, unique)
  - examples: owner, admin, operator, member, viewer
- `description` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`name`)

### Role Bindings Table

- `id` (uuid, pk)
- `subject_type` (text)
  - one of: user, group
- `subject_id` (uuid)
- `role_id` (uuid, fk to `roles.id`)
- `scope_type` (text)
  - one of: system, project
- `scope_id` (uuid, nullable)
  - null allowed only for system scope
- `is_active` (boolean)
- `managed_by` (text, nullable)
- `last_synced_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Index: (`subject_type`, `subject_id`)
- Index: (`role_id`)
- Index: (`scope_type`, `scope_id`)
- Index: (`is_active`)

## Access Control

- Spec ID: `CYNAI.SCHEMA.AccessControl` <a id="spec-cynai-schema-accesscontrol"></a>

Policy rules and access control audit log.
Used by API Egress, Secure Browser, and other policy-enforcing services.

Source: [`docs/tech_specs/access_control.md`](access_control.md).

### Access Control Rules Table

- `id` (uuid, pk)
- `subject_type` (text)
- `subject_id` (uuid, nullable)
  - null allowed only for subject_type system
- `action` (text)
- `resource_type` (text)
- `resource_pattern` (text)
- `effect` (text)
  - allow or deny
- `priority` (int)
- `conditions` (jsonb, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Index: (`subject_type`, `subject_id`)
- Index: (`action`)
- Index: (`resource_type`)
- Index: (`priority`)

### Access Control Audit Log Table

- `id` (uuid, pk)
- `subject_type` (text)
- `subject_id` (uuid, nullable)
- `action` (text)
- `resource_type` (text)
- `resource` (text)
- `decision` (text)
  - allow or deny
- `reason` (text, nullable)
- `task_id` (uuid, nullable)
- `created_at` (timestamptz)

Constraints

- Index: (`created_at`)
- Index: (`task_id`)

## API Egress Credentials

- Spec ID: `CYNAI.SCHEMA.ApiEgressCredentials` <a id="spec-cynai-schema-apiegresscredentials"></a>

Credentials for outbound API calls are stored in PostgreSQL and are only retrievable by the API Egress Server.
Agents MUST never receive credentials in responses.

Source: [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).

### API Credentials Table

Table name: `api_credentials`.

- `id` (uuid, pk)
- `owner_type` (text)
  - one of: user, group
- `owner_id` (uuid)
  - user id or group id, depending on owner_type
- `provider` (text)
- `credential_type` (text)
  - examples: api_key, oauth_token, bearer_token
- `credential_name` (text)
  - human-friendly label to support multiple keys per user and provider
- `credential_ciphertext` (bytea)
- `credential_kid` (text)
  - key identifier for envelope encryption rotation
- `is_active` (boolean)
- `expires_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`owner_type`, `owner_id`, `provider`, `credential_name`)
- Index: (`owner_type`, `owner_id`, `provider`)
- Index: (`provider`)
- Index: (`is_active`)

## Preferences

- Spec ID: `CYNAI.SCHEMA.Preferences` <a id="spec-cynai-schema-preferences"></a>

Preference entries store user task-execution preferences and constraints.
Preference entries are scoped (system, user, group, project, task) with precedence.
Deployment and service configuration (ports, hostnames, database DSNs, and secrets) MUST NOT be stored as preferences.
The distinction between preferences and system settings is defined in [User preferences (Terminology)](user_preferences.md#spec-cynai-stands-preferenceterminology).
The `users` table is shared with identity and RBAC.

Source: [`docs/tech_specs/user_preferences.md`](user_preferences.md).

### Preference Entries Table

- `id` (uuid, pk)
- `scope_type` (text)
  - one of: system, user, group, project, task
- `scope_id` (uuid, nullable)
  - null allowed only for system scope
- `key` (text)
- `value` (jsonb)
- `value_type` (text)
- `version` (int)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`scope_type`, `scope_id`, `key`)
- Index: (`scope_type`, `scope_id`)
- Index: (`key`)

### Preference Audit Log Table

- `id` (uuid, pk)
- `entry_id` (uuid, fk to `preference_entries.id`)
- `old_value` (jsonb)
- `new_value` (jsonb)
- `changed_at` (timestamptz)
- `changed_by` (text)
- `reason` (text, nullable)

Constraints

- Index: (`entry_id`)
- Index: (`changed_at`)

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
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `created_by` (uuid, fk to `users.id`, nullable)

Constraints

- Index: (`scope_type`, `scope_id`)
- Index: (`created_at`)

## Tasks, Jobs, and Nodes

The orchestrator owns task state and a queue of jobs backed by PostgreSQL.
Nodes register with the orchestrator and report capabilities; the orchestrator stores node registry and optional capability snapshot.

Sources: [`docs/tech_specs/orchestrator.md`](orchestrator.md), [`docs/tech_specs/worker_node.md`](worker_node.md), [`docs/tech_specs/langgraph_mvp.md`](langgraph_mvp.md).

### Tasks Table

- `id` (uuid, pk)
- `created_by` (uuid, fk to `users.id`)
  - creating user; set from authenticated request context when created via the gateway; for system-created and bootstrap tasks, use the reserved system user
- `project_id` (uuid, fk to `projects.id`, nullable)
  - optional project association for RBAC, preferences, and grouping; null unless explicitly set by client or PM/PA
- `plan_id` (uuid, fk to `project_plans.id`, nullable)
  - when set, task belongs to this plan; workflow for this task is gated on plan state active and on task dependencies (see [Task dependencies](#task-dependencies-table)).
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
- `summary` (text, nullable)
  - final summary written by workflow
- `metadata` (jsonb, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`created_by`)
- Index: (`project_id`)
- Index: (`plan_id`)
- Index: (`plan_id`)
- Index: (`status`)
- Index: (`closed`)
- Index: (`created_at`)

#### Task Dependencies Table

- Spec ID: `CYNAI.SCHEMA.TaskDependenciesTable` <a id="spec-cynai-schema-taskdependenciestable"></a>

Stores explicit task-within-plan dependencies; execution order and runnability are determined solely by the dependency graph (prerequisite and dependent tasks).
When a task is set to `canceled`, all tasks that depend on it (directly or transitively) MUST be set to `canceled` automatically; see [REQ-ORCHES-0154](../requirements/orches.md#req-orches-0154) and [Cancel cascades to dependents](langgraph_mvp.md#spec-cynai-orches-cancelcascadestodependents).
A task is **runnable** when all tasks it depends on have `status = 'completed'`; see [Project plan and task dependencies](langgraph_mvp.md#spec-cynai-orches-workflowplanorder) and [REQ-ORCHES-0153](../requirements/orches.md#req-orches-0153).

- `id` (uuid, pk)
- `task_id` (uuid, fk to `tasks.id`, NOT NULL)
  - the dependent task (this task runs after its dependencies)
- `depends_on_task_id` (uuid, fk to `tasks.id`, NOT NULL)
  - the task that must reach status `completed` before `task_id` may run

Constraints

- Unique: (`task_id`, `depends_on_task_id`)
- Check: `task_id != depends_on_task_id` (no self-deps)
- Application MUST ensure both tasks belong to the same plan (`tasks.plan_id` equal for both) when plan_id is set; optionally enforce via trigger or constraint.

Indexes

- Index: (`task_id`) for "what does this task depend on"
- Index: (`depends_on_task_id`) for "what tasks depend on this one"

#### Task Status and Closed State

- Spec ID: `CYNAI.SCHEMA.TaskStatusAndClosed` <a id="spec-cynai-schema-taskstatusandclosed"></a>

Task **status** is stored in `tasks.status` and represents the lifecycle state (e.g. pending, running, completed, failed, canceled, superseded).
Task **closed** is stored in `tasks.closed` (boolean): when true, the task is closed (no further work); when false, the task is open.
The system MUST keep `closed` consistent with `status` (e.g. set `closed = true` when status becomes completed, failed, canceled, or superseded).
Plan completion (set plan to completed) requires the plan to have at least one task and **all such tasks to have `closed = true`**; see [REQ-PROJCT-0121](../requirements/projct.md#req-projct-0121) and [Project plan state](projects_and_scopes.md#spec-cynai-access-projectplanstate).

### Jobs Table

- `id` (uuid, pk)
- `task_id` (uuid, fk to `tasks.id`)
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

- Index: (`task_id`)
- Index: (`persona_id`) where not null
- Index: (`node_id`)
- Index: (`status`)
- Index: (`lease_id`) where not null
- Index: (`lease_expires_at`) where not null
- Index: (`created_at`)

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

Creation order (respecting foreign keys)

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

Naming conventions

- Table names: `snake_case`, plural (e.g. `refresh_sessions`, `task_artifacts`).
- Primary key: `id` (uuid).
- Foreign keys: `{table_singular}_id` (e.g. `user_id`, `task_id`).
- Timestamps: `created_at`, `updated_at` (timestamptz); event tables use `created_at` or domain-specific names (e.g. `changed_at`, `reported_at`).
