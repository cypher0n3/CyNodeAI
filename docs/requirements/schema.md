# SCHEMA Requirements

- [SCHEMA Requirements](#schema-requirements)
  - [1 Overview](#1-overview)
  - [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `SCHEMA` domain.
It covers data persistence requirements, schema-level invariants, and database constraints.

## 2 Requirements

- **REQ-SCHEMA-0001:** Schema in code (GORM); AutoMigrate + explicit DDL bootstrap; pgvector extension and vector columns with explicit dimension and scope.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0001"></a>
- **REQ-SCHEMA-0100:** The schema MUST be represented in Go as GORM models (structs + tags).
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0100"></a>
- **REQ-SCHEMA-0101:** The orchestrator MUST provide a supported way to apply schema changes in dev and CI using GORM `AutoMigrate`.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0101"></a>
- **REQ-SCHEMA-0102:** Production deployments MUST have an explicit, supported schema-application step.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0102"></a>
- **REQ-SCHEMA-0103:** Implementations MUST pin and align the database stack versions used to generate schema changes.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0103"></a>
- **REQ-SCHEMA-0104:** Implementations MUST NOT rely on GORM `AutoMigrate` alone for all PostgreSQL DDL.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0104"></a>
- **REQ-SCHEMA-0105:** The DDL bootstrap step MUST be idempotent and safe to run repeatedly.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0105"></a>
- **REQ-SCHEMA-0106:** The database MUST enable the pgvector extension via the DDL bootstrap step.
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0106"></a>
- **REQ-SCHEMA-0107:** Vector columns MUST use an explicit pgvector dimension (for example `vector(1536)`).
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0107"></a>
- **REQ-SCHEMA-0108:** Embedding dimension changes MUST be treated as a breaking schema change and handled via a deterministic schema change process.
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0108"></a>
- **REQ-SCHEMA-0109:** Vector rows MUST be scoped so queries can filter by `task_id` and `project_id` when applicable.
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0109"></a>
- **REQ-SCHEMA-0110:** Vector rows MUST record the embedding model identifier used to produce the embedding.
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0110"></a>
- **REQ-SCHEMA-0111:** Vector rows MUST include columns that support RBAC filtering: at least project_id; and MUST include namespace and sensitivity_level (or equivalent) when vector retrieval RBAC is enforced.
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  [CYNAI.SCHEMA.VectorRetrievalRbac](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorretrievalrbac)
  <a id="req-schema-0111"></a>
- **REQ-SCHEMA-0112:** Similarity search MUST run only against rows that have been filtered by the authorized scope (project_id, namespace, sensitivity_level); composite indexes on (project_id, namespace) SHOULD be used for performance.
  [CYNAI.SCHEMA.VectorRetrievalRbac](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorretrievalrbac)
  <a id="req-schema-0112"></a>
- **REQ-SCHEMA-0113:** The schema MUST include a project git repositories table that stores per-project Git repo associations (project_id, provider, repo_identifier, optional base_url override) with a foreign key to projects.id.
  [CYNAI.SCHEMA.ProjectGitReposTable](../tech_specs/postgres_schema.md#spec-cynai-schema-projectgitrepostable)
  <a id="req-schema-0113"></a>
- **REQ-SCHEMA-0114:** When the gateway accepts user file uploads for chat, the system MUST persist uploaded file content or a stable reference in a way that is scoped to the authenticated user and associated thread or message and is retrievable for the duration required by the chat contract and retention policy.
  The stored representation MUST enforce the same authorization scope as chat thread and message access, including the same project-scoped permissions when the originating chat thread belongs to a shared project, and MUST remain subject to secret-redaction and size or type limits defined by the gateway.
  [CYNAI.SCHEMA.ChatMessageAttachmentsTable](../tech_specs/postgres_schema.md#spec-cynai-schema-chatmessageattachmentstable)
  <a id="req-schema-0114"></a>
- **REQ-SCHEMA-0115:** The schema MUST include a project-scoped specifications table and join tables (plan_specifications, task_specifications) so that plans and tasks can reference specifications; resolution MUST return specifications ordered by sort_order (nulls last), ref, created_at.
  [CYNAI.SCHEMA.SpecificationsTable](../tech_specs/postgres_schema.md#spec-cynai-schema-specificationstable)
  [CYNAI.SCHEMA.ResolveSpecificationsForPlanOrTask](../tech_specs/postgres_schema.md#spec-cynai-schema-resolvespecificationsforplanortask)
  <a id="req-schema-0115"></a>
- **REQ-SCHEMA-0116:** When specification MCP tools are adopted (db.specification.*, db.plan.specifications.set, db.task.specifications.set, specification.help), the gateway MUST allow them on the PM agent allowlist and enforce the same scope and access rules as other db tools (PMA write; SBA read-only when exposed).
  [CYNAI.MCPTOO.SpecificationHelp](../tech_specs/mcp_tool_catalog.md#spec-cynai-mcptoo-specificationhelp)
  [CYNAI.MCPGAT.PmAgentAllowlist](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-pmagentallowlist)
  <a id="req-schema-0116"></a>
- **REQ-SCHEMA-0117:** The personas table MUST support optional default_skill_ids (jsonb), recommended_cloud_models (jsonb; map by provider to model ids), and recommended_local_model_ids (jsonb); identity remains title and description.
  [CYNAI.SCHEMA.PersonasTable](../tech_specs/postgres_schema.md#spec-cynai-schema-personastable)
  <a id="req-schema-0117"></a>
- **REQ-SCHEMA-0118:** The tasks table MUST support optional persona_id (FK to personas) and recommended_skill_ids (jsonb array); at most one persona per task.
  [CYNAI.SCHEMA.TasksTable](../tech_specs/postgres_schema.md#spec-cynai-schema-taskstable)
  <a id="req-schema-0118"></a>
- **REQ-SCHEMA-0119:** The jobs table (or job payload) MUST use task_ids as a map keyed by numeric order (e.g. 10, 20, 30) with value task uuid; single-task job = one key; bundle = 1-3 keys; execution order = sort keys ascending.
  [CYNAI.SCHEMA.JobsTable](../tech_specs/postgres_schema.md#spec-cynai-schema-jobstable)
  <a id="req-schema-0119"></a>
