# Postgres Schema

- [Document Overview](#document-overview)
- [Goals and Scope](#goals-and-scope)
- [Schema Overview](#schema-overview)
- [Identity and Authentication](#identity-and-authentication)
  - [Users Table](#users-table)
  - [Password Credentials Table](#password-credentials-table)
  - [Refresh Sessions Table](#refresh-sessions-table)
- [Projects](#projects)
  - [Projects Table](#projects-table)
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
- [Tasks, Jobs, and Nodes](#tasks-jobs-and-nodes)
  - [Tasks Table](#tasks-table)
  - [Jobs Table](#jobs-table)
  - [Nodes Table](#nodes-table)
  - [Node Capabilities Table](#node-capabilities-table)
- [Sandbox Image Registry](#sandbox-image-registry)
  - [Sandbox Images Table](#sandbox-images-table)
  - [Sandbox Image Versions Table](#sandbox-image-versions-table)
  - [Node Sandbox Image Availability Table](#node-sandbox-image-availability-table)
- [Runs and Sessions](#runs-and-sessions)
  - [Runs Table](#runs-table)
  - [Sessions Table](#sessions-table)
- [Chat Threads and Messages](#chat-threads-and-messages)
  - [Chat Threads Table](#chat-threads-table)
  - [Chat Messages Table](#chat-messages-table)
- [Task Artifacts](#task-artifacts)
- [Vector Storage (`pgvector`)](#vector-storage-pgvector)
  - [Vector Items Table](#vector-items-table)
- [Audit Logging](#audit-logging)
  - [Auth Audit Log Table](#auth-audit-log-table)
  - [MCP Tool Call Audit Log Table](#mcp-tool-call-audit-log-table)
  - [Other Audit Tables](#other-audit-tables)
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

Traces To:

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

- Node capability report and node configuration payload wire formats (see [`docs/tech_specs/node.md`](node.md)).
- MCP gateway enforcement and tool allowlists (see [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)).

## Schema Overview

Logical groups

1. **Identity and authentication:** `users`, `password_credentials`, `refresh_sessions`
2. **Projects:** `projects`
3. **Groups and RBAC:** `groups`, `group_memberships`, `roles`, `role_bindings`
4. **Access control:** `access_control_rules`, `access_control_audit_log`
5. **API egress credentials:** `api_credentials`
6. **Preferences:** `preference_entries`, `preference_audit_log`
7. **Tasks, jobs, nodes:** `tasks`, `jobs`, `nodes`, `node_capabilities`
8. **Sandbox image registry:** `sandbox_images`, `sandbox_image_versions`, `node_sandbox_image_availability`
9. **Runs and sessions:** `runs`, `sessions`
10. **Chat:** `chat_threads`, `chat_messages`
11. **Task artifacts:** `task_artifacts`
12. **Vector storage (`pgvector`):** `vector_items`
13. **Audit:** `auth_audit_log`, `mcp_tool_call_audit_log` (and domain-specific audit tables above)
14. **Model registry (optional for MVP):** `models`, `model_versions`, `model_artifacts`, `node_model_availability`

## Identity and Authentication

The orchestrator MUST store users and local auth state in PostgreSQL.
Credentials and refresh tokens MUST be stored as hashes.

Source: [`docs/tech_specs/local_user_accounts.md`](local_user_accounts.md).

### Users Table

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

## Groups and RBAC

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

Preference entries store user task-execution preferences and constraints.
Preference entries are scoped (system, user, group, project, task) with precedence.
Deployment and service configuration (ports, hostnames, database DSNs, and secrets) MUST NOT be stored as preferences.
The distinction between preferences and system settings is defined in [User preferences (Terminology)](user_preferences.md#2-terminology).
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
System settings are not user task-execution preferences; for the distinction, see [User preferences (Terminology)](user_preferences.md#2-terminology).
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

## Tasks, Jobs, and Nodes

The orchestrator owns task state and a queue of jobs backed by PostgreSQL.
Nodes register with the orchestrator and report capabilities; the orchestrator stores node registry and optional capability snapshot.

Sources: [`docs/tech_specs/orchestrator.md`](orchestrator.md), [`docs/tech_specs/node.md`](node.md), [`docs/tech_specs/langgraph_mvp.md`](langgraph_mvp.md).

### Tasks Table

- `id` (uuid, pk)
- `created_by` (uuid, fk to `users.id`)
  - creating user; set from authenticated request context when created via the gateway; for system-created and bootstrap tasks, use the reserved system user
- `project_id` (uuid, fk to `projects.id`, nullable)
  - optional project association for RBAC, preferences, and grouping; null unless explicitly set by client or PM/PA
- `status` (text)
  - examples: pending, running, completed, failed, cancelled
- `acceptance_criteria` (jsonb, nullable)
  - structured criteria used by Project Manager for verification
- `summary` (text, nullable)
  - final summary written by workflow
- `metadata` (jsonb, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`created_by`)
- Index: (`project_id`)
- Index: (`status`)
- Index: (`created_at`)

### Jobs Table

- `id` (uuid, pk)
- `task_id` (uuid, fk to `tasks.id`)
- `node_id` (uuid, fk to `nodes.id`, nullable)
  - set when job is dispatched to a node
- `status` (text)
  - examples: queued, running, completed, failed, cancelled, lease_expired
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
  - URL of the node Worker API for job dispatch; from node configuration delivery; see [`node_payloads.md`](node_payloads.md) `node_configuration_payload_v1`
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
The orchestrator MUST store the capability report JSON here (or a normalized snapshot that preserves the actual capabilities per node.md).

- `id` (uuid, pk)
- `node_id` (uuid, fk to `nodes.id`)
- `reported_at` (timestamptz)
- `capability_snapshot` (jsonb)
  - full capability report JSON (identity, platform, compute, gpu, sandbox, network, inference, tls, etc. per node_payloads.md)

Constraints

- Unique: (`node_id`) (one row per node; overwrite on new report) or allow history (then index by `node_id`, `reported_at`)
- Index: (`node_id`)

Recommendation: one row per node, updated in place when capability report is received; alternatively, append-only with retention policy.

## Sandbox Image Registry

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
  - examples: runtimes, tools, network_requirements, filesystem_requirements
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
  - examples: pending, running, completed, failed, cancelled
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

Source:

- [`docs/tech_specs/chat_threads_and_messages.md`](chat_threads_and_messages.md)
- [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md)

### Chat Threads Table

- `id` (uuid, pk)
- `user_id` (uuid, fk to `users.id`)
- `project_id` (uuid, fk to `projects.id`, nullable)
- `session_id` (uuid, fk to `sessions.id`, nullable)
- `title` (text, nullable)
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

Constraints

- Unique: (`task_id`, `path`)
- Index: (`task_id`)
- Index: (`run_id`)
- Index: (`created_at`)

## Vector Storage (`pgvector`)

CyNodeAI uses PostgreSQL and pgvector for vector storage and similarity search.
Vector storage is used to support retrieval and semantic search over task-related content.

### Vector Storage Applicable Requirements

- Spec ID: `CYNAI.SCHEMA.VectorStorage` <a id="spec-cynai-schema-vectorstorage"></a>

Traces To:

- [REQ-SCHEMA-0106](../requirements/schema.md#req-schema-0106)
- [REQ-SCHEMA-0107](../requirements/schema.md#req-schema-0107)
- [REQ-SCHEMA-0108](../requirements/schema.md#req-schema-0108)
- [REQ-SCHEMA-0109](../requirements/schema.md#req-schema-0109)
- [REQ-SCHEMA-0110](../requirements/schema.md#req-schema-0110)

Recommended behavior

- Prefer cosine distance for similarity search.
- Use an approximate index (HNSW when available; otherwise IVFFLAT) to keep queries fast at scale.
- Store only sanitized, policy-allowed content in vector storage.
- Keep similarity search queries isolated in a repository layer so they can be tuned without changing callers.

### Vector Items Table

Table name: `vector_items`.

This table stores chunked text content and its embedding.
It is intentionally generic so multiple sources can be indexed (artifacts, run logs, connector documents, sanitized web pages).

- `id` (uuid, pk)
- `project_id` (uuid, fk to `projects.id`, nullable)
- `task_id` (uuid, fk to `tasks.id`, nullable)
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

## Audit Logging

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
- **Chat completions:** `chat_audit_log` (see OpenAI-compatible chat API and chat threads/messages).

Additional domain-specific audit tables (e.g. MCP tool calls, connector operations, Git egress) MAY be added later; they SHOULD include at least `task_id` (nullable), subject identity, action, decision or outcome, and `created_at`.

### Chat Audit Log Table

This table stores audit records for OpenAI-compatible chat completion requests.
It MUST NOT store full message content.

- `id` (uuid, pk)
- `created_at` (timestamptz)
- `user_id` (uuid, fk to `users.id`, nullable)
- `project_id` (uuid, fk to `projects.id`, nullable)
- `outcome` (text)
  - examples: success, error, cancelled, timeout
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
2. `projects`, `groups`, `roles`
3. `password_credentials`, `refresh_sessions`, `group_memberships`, `role_bindings`
4. `access_control_rules`
5. `api_credentials`
6. `preference_entries`
7. `nodes`, `sandbox_images`
8. `tasks`, `sessions`
9. `sandbox_image_versions`, `jobs`, `node_capabilities`
10. `runs`, `task_artifacts`, `node_sandbox_image_availability`, `vector_items`
11. `auth_audit_log`, `mcp_tool_call_audit_log`, `access_control_audit_log`, `preference_audit_log`
12. Model registry (if used): `models`, `model_versions`, `model_artifacts`, `node_model_availability`

Naming conventions

- Table names: `snake_case`, plural (e.g. `refresh_sessions`, `task_artifacts`).
- Primary key: `id` (uuid).
- Foreign keys: `{table_singular}_id` (e.g. `user_id`, `task_id`).
- Timestamps: `created_at`, `updated_at` (timestamptz); event tables use `created_at` or domain-specific names (e.g. `changed_at`, `reported_at`).
