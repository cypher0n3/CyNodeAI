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
  - [System Settings Tables](#system-settings-tables)
- [Personas](#personas)
  - [Personas Tables](#personas-tables)
- [Tasks, Jobs, and Nodes](#tasks-jobs-and-nodes)
  - [Task vs Job (Terminology)](#task-vs-job-terminology)
  - [Tasks Table](#tasks-table)
  - [Requirement Object Structure](#requirement-object-structure)
  - [Task Dependencies Table](#task-dependencies-table)
  - [Task Status and Closed State](#task-status-and-closed-state)
  - [Jobs Table](#jobs-table)
  - [Nodes Table](#nodes-table)
  - [Node Capabilities Table](#node-capabilities-table)
- [Workflow Checkpoints](#workflow-checkpoints)
  - [Workflow Checkpoints Table](#workflow-checkpoints-table)
  - [Task Workflow Leases Table](#task-workflow-leases-table)
- [Sandbox Image Registry](#sandbox-image-registry)
  - [Sandbox Image Registry Tables](#sandbox-image-registry-tables)
- [Runs and Sessions](#runs-and-sessions)
  - [Runs and Sessions Tables](#runs-and-sessions-tables)
- [Chat Threads and Messages](#chat-threads-and-messages)
  - [Chat Threads and Messages Tables](#chat-threads-and-messages-tables)
- [Task Artifacts](#task-artifacts)
  - [Task Artifacts Tables](#task-artifacts-tables)
- [Vector Storage (`pgvector`)](#vector-storage-pgvector)
  - [Vector Storage Tables](#vector-storage-tables)
  - [Vector Retrieval and RBAC](#vector-retrieval-and-rbac)
- [Audit Logging](#audit-logging)
  - [Auth Audit Log Table](#auth-audit-log-table)
  - [MCP Tool Call Audit Log Table](#mcp-tool-call-audit-log-table)
  - [Chat Audit Log Table](#chat-audit-log-table)
  - [Other Audit Tables](#other-audit-tables)
- [Model Registry](#model-registry)
  - [Model Registry Tables](#model-registry-tables)
- [Table Summary and Dependencies](#table-summary-and-dependencies)

## Document Overview

This document is the index and cross-reference hub for the CyNodeAI orchestrator PostgreSQL schema.
Authoritative column-level definitions live in the linked domain tech specs; this document lists table groups, stable identifiers, and creation order.

### Source of Truth

- Domain tech specs are authoritative for column names, types, and constraints for their tables; this document links to them and summarizes dependencies.
- Where another spec defines identity or RBAC tables, this document references those definitions and lists orchestrator tables that depend on them.
- Cross-references to other specs are for context and behavior; when a domain spec and this index disagree on schema details, the domain spec wins.

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
- MCP tool allowlists and per-tool scope (see [`docs/tech_specs/mcp_tools/access_allowlists_and_scope.md`](mcp_tools/access_allowlists_and_scope.md)); gateway enforcement (see [`docs/tech_specs/mcp/mcp_gateway_enforcement.md`](mcp/mcp_gateway_enforcement.md)).

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

- Spec ID: `CYNAI.SCHEMA.SystemSettings` <a id="spec-cynai-schema-systemsettings"></a>

System settings store operator-managed operational configuration and policy parameters.
System settings are not user task-execution preferences; for the distinction, see [User preferences (Terminology)](user_preferences.md#spec-cynai-stands-preferenceterminology).
System settings do not store secrets in plaintext.

**Schema definitions:** See [Postgres Schema](orchestrator_bootstrap.md#spec-cynai-schema-systemsettings) in [`orchestrator_bootstrap.md`](orchestrator_bootstrap.md).

### System Settings Tables

- `system_settings` - See [System Settings Table](orchestrator_bootstrap.md#spec-cynai-schema-systemsettingstable)
- `system_settings_audit_log` - See [System Settings Audit Log Table](orchestrator_bootstrap.md#spec-cynai-schema-systemsettingsauditlogtable)

## Personas

- Spec ID: `CYNAI.SCHEMA.Personas` <a id="spec-cynai-schema-personas"></a>

Agent personas are named, reusable descriptions of how the sandbox agent should behave (role, identity, tone); they are not customer or end-user personas.
They are stored in the deployment and are queriable by agents (PMA, PAA, orchestrator job builder) via the User API Gateway or MCP.
When building a job, the builder resolves the chosen Agent persona by id (or by title with scope precedence) and embeds `title` and `description` inline into the job spec; the SBA receives only the inline object.
Editing (create, update, delete) is subject to RBAC: system-scoped personas require admin (or equivalent) role; user-/project-/group-scoped require appropriate role for that scope; see [data_rest_api.md - Core Resources](data_rest_api.md#spec-cynai-datapi-coreresources).

**Schema definitions:** See [Postgres Schema](personas.md#spec-cynai-schema-personas) in [`personas.md`](personas.md).

### Personas Tables

- Spec ID: `CYNAI.SCHEMA.PersonasTable` <a id="spec-cynai-schema-personastable"></a>

- `personas` - See [Personas Table](personas.md#spec-cynai-schema-personastable)

## Tasks, Jobs, and Nodes

- Spec ID: `CYNAI.SCHEMA.TasksJobsNodes` <a id="spec-cynai-schema-tasksjobsnodes"></a>

The orchestrator owns task state and a queue of jobs backed by PostgreSQL.
Nodes register with the orchestrator and report capabilities; the orchestrator stores node registry and optional capability snapshot.

Sources: [`orchestrator.md`](orchestrator.md), [`worker_node.md`](worker_node.md), [`workflow_mvp.md`](workflow_mvp.md).

**Schema definitions:** See [Postgres Schema](orchestrator.md#spec-cynai-schema-tasksjobsnodes) in [`orchestrator.md`](orchestrator.md).

### Task vs Job (Terminology)

- Spec ID: `CYNAI.SCHEMA.TaskVsJob` <a id="spec-cynai-schema-taskvsjob"></a>

**Schema definitions:** See [Task vs Job (Terminology)](orchestrator.md#spec-cynai-schema-taskvsjob).

### Tasks Table

- Spec ID: `CYNAI.SCHEMA.TasksTable` <a id="spec-cynai-schema-taskstable"></a>

**Schema definitions:** See [Tasks Table](orchestrator.md#spec-cynai-schema-taskstable).

#### Requirement Object Structure

- Spec ID: `CYNAI.SCHEMA.RequirementObject` <a id="spec-cynai-schema-requirementobject"></a>

**Schema definitions:** See [Requirement object structure](orchestrator.md#spec-cynai-schema-requirementobject).

### Task Dependencies Table

- Spec ID: `CYNAI.SCHEMA.TaskDependenciesTable` <a id="spec-cynai-schema-taskdependenciestable"></a>

**Schema definitions:** See [Task Dependencies Table](orchestrator.md#spec-cynai-schema-taskdependenciestable).

### Task Status and Closed State

- Spec ID: `CYNAI.SCHEMA.TaskStatusAndClosed` <a id="spec-cynai-schema-taskstatusandclosed"></a>

**Schema definitions:** See [Task Status and Closed State](orchestrator.md#spec-cynai-schema-taskstatusandclosed).

### Jobs Table

- Spec ID: `CYNAI.SCHEMA.JobsTable` <a id="spec-cynai-schema-jobstable"></a>

**Schema definitions:** See [Jobs Table](orchestrator.md#spec-cynai-schema-jobstable).

### Nodes Table

- Spec ID: `CYNAI.SCHEMA.NodesTable` <a id="spec-cynai-schema-nodestable"></a>

**Schema definitions:** See [Nodes Table](orchestrator.md#spec-cynai-schema-nodestable).

### Node Capabilities Table

- Spec ID: `CYNAI.SCHEMA.NodeCapabilitiesTable` <a id="spec-cynai-schema-nodecapabilitiestable"></a>

**Schema definitions:** See [Node Capabilities Table](orchestrator.md#spec-cynai-schema-nodecapabilitiestable).

## Workflow Checkpoints

- Spec ID: `CYNAI.SCHEMA.WorkflowCheckpoints` <a id="spec-cynai-schema-workflowcheckpoints"></a>

The workflow engine persists checkpoint state to PostgreSQL so that workflows can resume after restarts.
The orchestrator does not run workflow steps without going through this checkpoint layer.

Source: [workflow_mvp.md](workflow_mvp.md) (checkpoint schema and workflow API).

**Schema definitions:** See [Postgres Schema](workflow_mvp.md#spec-cynai-schema-workflowcheckpoints) in [`workflow_mvp.md`](workflow_mvp.md).

### Workflow Checkpoints Table

- Spec ID: `CYNAI.SCHEMA.WorkflowCheckpointsTable` <a id="spec-cynai-schema-workflowcheckpointstable"></a>

**Schema definitions:** See [Workflow Checkpoints Table](workflow_mvp.md#spec-cynai-schema-workflowcheckpointstable).

### Task Workflow Leases Table

- Spec ID: `CYNAI.SCHEMA.TaskWorkflowLeasesTable` <a id="spec-cynai-schema-taskworkflowleasestable"></a>

**Schema definitions:** See [Task Workflow Leases Table](workflow_mvp.md#spec-cynai-schema-taskworkflowleasestable).

## Sandbox Image Registry

- Spec ID: `CYNAI.SCHEMA.SandboxImageRegistry` <a id="spec-cynai-schema-sandboximageregistry"></a>

Allowed sandbox images and their capabilities are stored in PostgreSQL.
This enables agents and schedulers to pick safe, policy-approved execution environments.

Source: [`sandbox_image_registry.md`](sandbox_image_registry.md).

**Schema definitions:** See [Postgres Schema](sandbox_image_registry.md#spec-cynai-schema-sandboximageregistry) in [`sandbox_image_registry.md`](sandbox_image_registry.md).

### Sandbox Image Registry Tables

- `sandbox_images` - See [Sandbox Images Table](sandbox_image_registry.md#spec-cynai-schema-sandboximageregistry)
- `sandbox_image_versions` - See [Sandbox Image Versions Table](sandbox_image_registry.md#spec-cynai-schema-sandboximageregistry)
- `node_sandbox_image_availability` - See [Node Sandbox Image Availability Table](sandbox_image_registry.md#spec-cynai-schema-sandboximageregistry)

## Runs and Sessions

- Spec ID: `CYNAI.SCHEMA.RunsSessions` <a id="spec-cynai-schema-runssessions"></a>

Runs are execution traces (workflow instance, dispatched job, or agent turn).
Sessions are user-facing containers that group runs and hold transcripts.

Source: [`runs_and_sessions_api.md`](runs_and_sessions_api.md).

**Schema definitions:** See [Postgres Schema](runs_and_sessions_api.md#spec-cynai-schema-runssessions) in [`runs_and_sessions_api.md`](runs_and_sessions_api.md).

### Runs and Sessions Tables

- `runs` - See [Runs Table](runs_and_sessions_api.md#spec-cynai-schema-runssessions)
- `sessions` - See [Sessions Table](runs_and_sessions_api.md#spec-cynai-schema-runssessions)

## Chat Threads and Messages

- Spec ID: `CYNAI.SCHEMA.ChatThreadsMessages` <a id="spec-cynai-schema-chatthreadsmessages"></a>

Chat threads and chat messages store chat history separately from task lifecycle state.
Chat message content is the amended (redacted) content.
Plaintext secrets are not persisted in chat message content.

### Source Documents

- [`chat_threads_and_messages.md`](chat_threads_and_messages.md)
- [`openai_compatible_chat_api.md`](openai_compatible_chat_api.md)

**Schema definitions:** See [Postgres Schema](chat_threads_and_messages.md#spec-cynai-schema-chatthreadsmessages) in [`chat_threads_and_messages.md`](chat_threads_and_messages.md).

### Chat Threads and Messages Tables

- `chat_threads` - See [Chat Threads Table](chat_threads_and_messages.md#spec-cynai-schema-chatthreadsmessages)
- `chat_messages` - See [Chat Messages Table](chat_threads_and_messages.md#spec-cynai-schema-chatthreadsmessages)

### Chat Message Attachments Table

- Spec ID: `CYNAI.SCHEMA.ChatMessageAttachmentsTable` <a id="spec-cynai-schema-chatmessageattachmentstable"></a>

**Schema definitions:** See [Chat Message Attachments Table](chat_threads_and_messages.md#spec-cynai-schema-chatmessageattachmentstable).

## Task Artifacts

- Spec ID: `CYNAI.SCHEMA.TaskArtifacts` <a id="spec-cynai-schema-taskartifacts"></a>

Artifacts are files or blobs stored in **scope partitions** (user, group, project, global) with optional **job** and **task** ids for lineage only.
See [Orchestrator Artifacts Storage - Artifacts Table (Metadata)](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactstablemetadata).

**Schema definitions:** See [Postgres Schema](orchestrator_artifacts_storage.md#spec-cynai-schema-taskartifacts) in [`orchestrator_artifacts_storage.md`](orchestrator_artifacts_storage.md).

### Task Artifacts Tables

- `artifacts` (or legacy `task_artifacts`) - See [Artifacts Table (Metadata)](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactstablemetadata)

## Vector Storage (`pgvector`)

- Spec ID: `CYNAI.SCHEMA.VectorStorage` <a id="spec-cynai-schema-vectorstorage"></a>

CyNodeAI uses PostgreSQL and pgvector for vector storage and similarity search.
Vector storage supports retrieval and semantic search over task-related content.

**Schema definitions:** See [Postgres Schema](vector_storage.md#spec-cynai-schema-vectorstorage-schema) in [`vector_storage.md`](vector_storage.md).

### Vector Storage Tables

- `vector_items` - See [Vector Items Table](vector_storage.md#spec-cynai-schema-vectoritemstable)

### Vector Retrieval and RBAC

- Spec ID: `CYNAI.SCHEMA.VectorRetrievalRbac` <a id="spec-cynai-schema-vectorretrievalrbac"></a>

**Schema definitions:** See [Vector Retrieval and RBAC](vector_storage.md#spec-cynai-schema-vectorretrievalrbac).

## Audit Logging

- Spec ID: `CYNAI.SCHEMA.AuditLogging` <a id="spec-cynai-schema-auditlogging"></a>

Audit logging uses domain-specific tables so that retention and query patterns can be tuned per domain.
Event tables use `timestamptz` for event time and include subject and decision or outcome where applicable.

**Schema definitions:** See [Audit logging](audit_logging.md#spec-cynai-schema-auditlogging) in [`audit_logging.md`](audit_logging.md).

### Auth Audit Log Table

**Schema definitions:** See [Auth Audit Log Table](local_user_accounts.md#spec-cynai-schema-authauditlogtable) (`auth_audit_log`).

### MCP Tool Call Audit Log Table

- Spec ID: `CYNAI.SCHEMA.McpToolCallAuditLog` <a id="spec-cynai-schema-mcptoolcallauditlog"></a>

**Schema definitions:** See [MCP Tool Call Audit Log Table](mcp/mcp_tool_call_auditing.md#spec-cynai-schema-mcptoolcallauditlog).

### Chat Audit Log Table

**Schema definitions:** See [Chat Audit Log Table](openai_compatible_chat_api.md#spec-cynai-schema-chatauditlogtable).

### Other Audit Tables

- **Access control:** `access_control_audit_log` - [Access Control](access_control.md#spec-cynai-schema-accesscontrol)
- **Preferences:** `preference_audit_log` - [Preferences](user_preferences.md#spec-cynai-schema-preferences)
- **System settings:** `system_settings_audit_log` - [Orchestrator Bootstrap](orchestrator_bootstrap.md#spec-cynai-schema-systemsettings)

## Model Registry

- Spec ID: `CYNAI.SCHEMA.ModelRegistry` <a id="spec-cynai-schema-modelregistry"></a>

Model metadata and node-model availability.
Optional for MVP; required when model management and node load workflow are implemented.

Source: [`model_management.md`](model_management.md).

**Schema definitions:** See [Postgres Schema](model_management.md#spec-cynai-schema-modelregistry) in [`model_management.md`](model_management.md).

### Model Registry Tables

- `models` - See [Models Table](model_management.md#spec-cynai-schema-modelregistry)
- `model_versions` - See [Model Versions Table](model_management.md#spec-cynai-schema-modelregistry)
- `model_artifacts` - See [Model Artifacts Table](model_management.md#spec-cynai-schema-modelregistry)
- `node_model_availability` - See [Node Model Availability Table](model_management.md#spec-cynai-schema-modelregistry)

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
11. `auth_audit_log`, `mcp_tool_call_audit_log`, `chat_audit_log`, `access_control_audit_log`, `preference_audit_log`
12. Model registry (if used): `models`, `model_versions`, `model_artifacts`, `node_model_availability`

### Naming Conventions

- Table names: `snake_case`, plural (e.g. `refresh_sessions`, `task_artifacts`).
- Primary key: `id` (uuid).
- Foreign keys: `{table_singular}_id` (e.g. `user_id`, `task_id`).
- Timestamps: `created_at`, `updated_at` (timestamptz); event tables use `created_at` or domain-specific names (e.g. `changed_at`, `reported_at`).
