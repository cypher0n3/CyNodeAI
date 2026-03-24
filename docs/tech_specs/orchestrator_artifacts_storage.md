# Orchestrator Artifacts Storage

- [Document Overview](#document-overview)
  - [Traces To](#traces-to)
- [Postgres Schema](#postgres-schema)
  - [Artifacts Table (Metadata)](#artifacts-table-metadata)
- [S3-Like Block Storage](#s3-like-block-storage)
- [Dev Stack: Containerized S3](#dev-stack-containerized-s3)
- [Artifacts API (CRUD)](#artifacts-api-crud)
  - [Create (POST)](#create-post)
  - [Read (GET)](#read-get)
  - [Update (PUT)](#update-put)
  - [Delete (DELETE)](#delete-delete)
  - [Find (List)](#find-list)
- [Artifact Lookup Implementation](#artifact-lookup-implementation)
- [Algorithm: Database and Blob Storage](#algorithm-database-and-blob-storage)
  - [Resolve `artifact_id` to Metadata and `storage_ref`](#resolve-artifact_id-to-metadata-and-storage_ref)
  - [Create (POST) Algorithm](#create-post-algorithm)
  - [Read (GET) Algorithm](#read-get-algorithm)
  - [Update (PUT) Algorithm](#update-put-algorithm)
  - [Delete (DELETE) Algorithm](#delete-delete-algorithm)
  - [Find (List) Algorithm](#find-list-algorithm)
- [RBAC for Artifacts](#rbac-for-artifacts)
  - [Artifact Subject (Identity)](#artifact-subject-identity)
  - [Artifact Scope and Ownership](#artifact-scope-and-ownership)
  - [Allow Rules](#allow-rules)
  - [Cross-Principal Access](#cross-principal-access)
  - [Permission Mapping](#permission-mapping)
- [MCP Tooling for PMA and PAA](#mcp-tooling-for-pma-and-paa)
- [Vector Ingestion Source](#vector-ingestion-source)
- [Database Metadata](#database-metadata)
- [Stale Artifact Cleanup](#stale-artifact-cleanup)
- [Artifact Hashing](#artifact-hashing)
- [Deferred Implementation](#deferred-implementation)

## Document Overview

- Spec ID: `CYNAI.ORCHES.Doc.OrchestratorArtifactsStorage` <a id="spec-cynai-orches-doc-orchestratorartifactsstorage"></a>

The orchestrator uses S3-compatible object (block) storage as its primary backend for artifact blobs.
The database holds artifact metadata and references for storage and retrieval; blob content lives in the S3-like store.
All artifact access is exposed through a single, RBAC-protected REST API with full CRUD and find (Create, Read, Update, Delete, Find/list).

### Traces To

- [REQ-SCHEMA-0114](../requirements/schema.md#req-schema-0114)
- [REQ-ORCHES-0127](../requirements/orches.md#req-orches-0127)
- [REQ-ORCHES-0167](../requirements/orches.md#req-orches-0167)

## Postgres Schema

- Spec ID: `CYNAI.SCHEMA.TaskArtifacts` <a id="spec-cynai-schema-taskartifacts"></a>

Artifacts are files or blobs whose **storage namespace** is a **scope partition** (**user**, **group**, **project**, or **global**), not a task or job.
**Tasks** and **jobs** are associated only for **lineage and audit** (optional columns below), not for defining uniqueness of `path`.

Metadata is stored in PostgreSQL; large content may be stored in object storage or node-local staging with a reference.

**Schema definitions (index):** See [Task Artifacts](postgres_schema.md#spec-cynai-schema-taskartifacts) in [`postgres_schema.md`](postgres_schema.md).

### Artifacts Table (Metadata)

- Spec ID: `CYNAI.ORCHES.ArtifactsTableMetadata` <a id="spec-cynai-orches-artifactstablemetadata"></a>

Logical table name: **`artifacts`** (implementations MAY retain the legacy name `task_artifacts` until migrated).

- `id` (uuid, pk) - client-facing **`artifact_id`**
- **`scope_level`** (text or enum): `user` | `group` | `project` | `global`
- **`owner_user_id`** (uuid, fk to `users.id`, nullable)
  - Owning user for **user**-scoped rows; creating user for group/project rows when useful for audit.
- **`group_id`** (uuid, nullable, fk when `scope_level=group`)
- **`project_id`** (uuid, nullable, fk when `scope_level=project`)
- **`path`** (text)
  - Logical path within the **scope partition** (e.g. `output/report.md`); uniqueness is enforced **per partition**, not per task.
- **`storage_ref`** (text) - blob reference (e.g. S3 object key)
- **`size_bytes`** (bigint, nullable)
- **`content_type`** (text, nullable)
- **`checksum_sha256`** (text, nullable)
- **`created_by_job_id`** (uuid, nullable, fk to `jobs.id`) - optional lineage: job that created the artifact
- **`last_modified_by_job_id`** (uuid, nullable, fk to `jobs.id`) - optional lineage: last job that updated the artifact
- **`correlation_task_id`** (uuid, nullable, fk to `tasks.id`) - optional **correlation only**; not part of the storage key
- `run_id` (uuid, fk to `runs.id`, nullable) - optional when tied to a run record
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### Artifacts Table Constraints

- Unique: (`scope_level`, `group_id`, `project_id`, `owner_user_id`, `path`) with null-safe partition semantics per implementation (e.g. partial unique indexes per `scope_level` so `path` is unique within each non-null anchor).
- Index: (`project_id`), (`group_id`), (`owner_user_id`), (`created_by_job_id`), (`correlation_task_id`)

Implementations MUST document the exact uniqueness strategy for each `scope_level` (mirror **partition** uniqueness used for skills scope).

## S3-Like Block Storage

- Spec ID: `CYNAI.ORCHES.ArtifactsS3Backend` <a id="spec-cynai-orches-artifactss3backend"></a>

- The orchestrator stack MUST provide an S3-compatible object storage service (e.g. AWS S3, MinIO, or equivalent) as the primary backend for artifact blobs.
- The control-plane and user-gateway (or the component that performs artifact write/read on behalf of the orchestrator) MUST be configured with the storage endpoint, credentials, and bucket (or equivalent namespace) via environment or configuration.
- Blobs are stored under implementation-defined object keys; the database holds the mapping from logical artifact identifier to that key (`storage_ref`).
- File size and media-type limits are NOT specified in this spec; they MAY be added in a later revision (e.g. per-endpoint or per-tenant policy).

## Dev Stack: Containerized S3

- Spec ID: `CYNAI.ORCHES.ArtifactsDevStack` <a id="spec-cynai-orches-artifactsdevstack"></a>

- For local and CI dev builds, the orchestrator stack MUST deploy a containerized S3-compatible service (e.g. MinIO) as part of [`orchestrator/docker-compose.yml`](../../orchestrator/docker-compose.yml).
- That service MUST expose the S3 API on a stable port (e.g. 9000) and MUST be on the same Docker network as the control-plane and user-gateway so they can reach it by service name.
- The compose file MUST define the service, its port mapping, and any volume required for persistence; control-plane and user-gateway MUST receive the endpoint URL and credentials via environment variables (e.g. `ARTIFACTS_S3_ENDPOINT`, `ARTIFACTS_S3_ACCESS_KEY`, `ARTIFACTS_S3_SECRET_KEY`, and optional `ARTIFACTS_S3_BUCKET`).

## Artifacts API (CRUD)

- Spec ID: `CYNAI.ORCHES.ArtifactsApiCrud` <a id="spec-cynai-orches-artifactsapicrud"></a>

The user API gateway MUST expose a unified artifacts API under `/v1/artifacts` with full CRUD.
All operations require authentication (session or bearer token) and MUST enforce **RBAC** from the artifact's **scope** (user, group, project, global) and **ownership**, plus **grants** that allow non-owners to read or write when policy permits; optional **job** / **task** fields are for lineage or correlation only.
The gateway resolves scope via the database and returns 403 when the caller lacks permission.

### Create (POST)

- **`POST /v1/artifacts`**
- **Request:** Body is the artifact blob (binary or application/octet-stream); optional headers or query/body parameters supply **scope** (`scope_level` and required anchors: `group_id`, `project_id`, or owner user) and metadata (e.g. `Content-Type`, `X-Filename` or `filename`).
  Optional **`job_id`** / **`task_id`** record lineage or correlation; they do **not** replace scope as the storage key.
  Chat or thread uploads MAY continue to use `thread_id` where that flow attaches blobs to a message (separate from partition scoped artifacts).
- **Response:** 201 Created with a JSON body that MUST include `artifact_id` (string), the stable identifier for the stored artifact used for subsequent Read, Update, Delete, and retrieval links.
  The response MAY also include optional fields such as `storage_ref`, `size_bytes`, `content_type`, `created_at`.
- **Errors:** 400 when request is invalid or scope is missing where required; 401/403 for auth failures.

### Read (GET)

- **`GET /v1/artifacts/{artifact_id}`**
- **Request:** Path parameter `artifact_id` is a stable identifier returned by Create or by a domain flow (e.g. chat upload, task artifact, `download_ref`).
- **Response:** 200 OK with the artifact blob as the response body, and appropriate `Content-Type` and `Content-Disposition` when metadata is known; 404 when `artifact_id` is unknown or the artifact was removed; 403 when the caller is not authorized.
- Chat file uploads, chat `download_ref` retrieval, and task artifact retrieval MUST all be served through this endpoint (or redirect/signed-URL to it) so that a single contract and RBAC model applies.
- The PMA MAY provide users with **artifact download links** (e.g. in chat or task results) that point to `GET /v1/artifacts/{artifact_id}`; when the user follows the link, the gateway MUST enforce [RBAC](#rbac-for-artifacts) and return the artifact only if the caller is authorized, otherwise 403.

### Update (PUT)

- **`PUT /v1/artifacts/{artifact_id}`**
- **Request:** Path parameter `artifact_id`; body is the new blob content (binary).
  Optional headers or query parameters supply updated metadata (e.g. `Content-Type`, filename) for the DB row.
- **Response:** 200 OK (optionally with updated metadata in JSON body); 404 when `artifact_id` is unknown; 403 when the caller is not authorized to modify the artifact.
  The implementation MUST overwrite the blob in the S3-like backend and MAY update metadata in the database.

### Delete (DELETE)

- **`DELETE /v1/artifacts/{artifact_id}`**
- **Request:** Path parameter `artifact_id`.
- **Response:** 204 No Content on success; 404 when `artifact_id` is unknown; 403 when the caller is not authorized to delete the artifact.
  The implementation MUST remove or invalidate the blob in the S3-like backend and MUST remove or soft-delete the corresponding metadata so the artifact is no longer accessible via Read.
  The implementation MUST also remove or invalidate any [vector_items](vector_storage.md#spec-cynai-schema-vectoritemstable) rows that reference this artifact (`source_type` and `source_ref`), so that vector search no longer returns chunks derived from the deleted artifact.

### Find (List)

- **`GET /v1/artifacts`**
- **Request:** Query parameters supply scope filters: **`scope_level`** with the appropriate anchor (`user_id`, `group_id`, `project_id`), or filters such as `thread_id` for chat-linked attachments, plus optional **`job_id`** / **`correlation_task_id`** for audit queries.
  At least one filter that resolves to an allowed partition is required; optional pagination (`limit`, `cursor` or `offset`).
  The implementation MUST only return artifacts the caller may list under [RBAC](#rbac-for-artifacts).
- **Response:** 200 OK with a JSON body containing a list of artifact metadata (e.g. `artifact_id`, `filename`, `content_type`, `size_bytes`, `created_at`, and optional scope fields).
  The list MUST be ordered (e.g. by `created_at` descending) and MAY be paginated via `next_cursor` or link headers.
- **Errors:** 400 when no scope filter is provided or filters are invalid; 401/403 for auth failures.

## Artifact Lookup Implementation

- Spec ID: `CYNAI.ORCHES.ArtifactsLookupImpl` <a id="spec-cynai-orches-artifactslookupimpl"></a>

The implementation MUST support artifact find (list) by scope using the database; it MUST NOT require a custom search stack.

- **Scope-based listing:** List/find by **`scope_level`** and anchors (`user_id`, `group_id`, `project_id`), and optionally `thread_id` for chat-linked rows, is implemented with SQL over artifact metadata tables (e.g. [chat_message_attachments](chat_threads_and_messages.md#spec-cynai-schema-chatmessageattachmentstable), [Artifacts Table (Metadata)](#artifacts-table-metadata)), with [RBAC](#rbac-for-artifacts) applied.
  The existing Go ORM (e.g. GORM) and PostgreSQL are sufficient; no additional lookup library is required for scope-only find.

- **Full-text or rich search (optional):** When the implementation needs full-text or richer search over artifact metadata (e.g. filename, content_type) or over indexed blob content, it MUST use an existing, MIT- or Apache-2.0-licensed (or compatible) library or feature.
  Recommended options:
  - **Bleve** (Apache 2.0): Go library `github.com/blevesearch/bleve/v2` for indexing and querying artifact metadata (and optionally content).
    Use when the implementation needs rich query types (phrase, prefix, fuzzy, facets) or content indexing without relying on the database.
  - **PostgreSQL full-text search:** Built-in `to_tsvector` / `to_tsquery` (or `websearch_to_tsquery`) on metadata columns; no extra dependency and BSD-style license.
    Use when scope filtering plus simple text search over metadata is sufficient.
- The implementation MUST NOT invent a custom search or index format; it MUST use one of the above or another library/feature with an explicit MIT/Apache-2.0 (or compatible) license.

## Algorithm: Database and Blob Storage

- Spec ID: `CYNAI.ORCHES.ArtifactsDbBlobAlgo` <a id="spec-cynai-orches-artifactsdbblobalgo"></a>

The gateway (or component that serves the artifacts API) MUST implement the following flow for each operation.
Blob content lives only in the S3-like store; the database holds metadata and the mapping from logical identifier to blob.

### Resolve `artifact_id` to Metadata and `storage_ref`

- Before any Read, Update, or Delete, the implementation MUST resolve `artifact_id` to a row that carries scope and `storage_ref`.
- **Resolution:** The implementation MUST look up `artifact_id` in one or more tables that reference artifacts (e.g. [chat_message_attachments](chat_threads_and_messages.md#spec-cynai-schema-chatmessageattachmentstable) using `file_id` or `id`; [task_artifacts](#spec-cynai-schema-taskartifacts) using `id` or a stable composite; or a dedicated `artifacts` table if the implementation unifies all artifact metadata).
- From the resolved row the implementation MUST obtain: `storage_ref` (S3 object key or equivalent), and **`scope_level`** plus anchors used for [RBAC](#rbac-for-artifacts) (`owner_user_id`, `group_id`, `project_id` as applicable).
- If no row is found, or the row is marked deleted, the implementation MUST return 404.

### Create (POST) Algorithm

- Step 1: Authenticate the caller and resolve the request subject (user id, group memberships) for [RBAC](#rbac-for-artifacts).
- Step 2: From the request, determine **`scope_level`** and anchors (`owner_user_id`, `group_id`, `project_id`, or global).
  Optional **`job_id`** / **`task_id`** are stored only as lineage or correlation metadata.
  The implementation MUST enforce that the caller is allowed to **create** in that scope partition (same RBAC rules as write).
- Step 3: Generate a stable `artifact_id` (e.g. UUID) and an S3 object key.
- Step 4: Upload the request body (blob) to the S3-like backend using that key.
- Step 5: Insert a row into the artifacts metadata table with `storage_ref`, `scope_level`, path, anchors, optional `created_by_job_id` / `correlation_task_id`, and metadata (filename, media_type, size_bytes).
  Chat attachments MAY use `chat_message_attachments` with thread scope; partition-scoped artifacts use the [Artifacts Table (Metadata)](#artifacts-table-metadata).
  If the artifact is below the small-artifact size threshold (see [Artifact hashing](#artifact-hashing)), compute the content hash from the request body and set `checksum_sha256` on the row; otherwise the hash MAY be deferred.
- Step 6: Return 201 with a JSON body that MUST include `artifact_id`.

### Read (GET) Algorithm

- Step 1: Authenticate the caller and resolve the request subject.
- Step 2: [Resolve artifact_id](#resolve-artifact_id-to-metadata-and-storage_ref) to the row and obtain `storage_ref` and scope.
- Step 3: [Apply RBAC](#rbac-for-artifacts): determine whether the subject may read this artifact (owner or project-scoped access).
  If not allowed, return 403.
- Step 4: Fetch the object from the S3-like backend using `storage_ref` (GET object by key).
  If the backend returns not-found, return 404.
- Step 5: Stream the blob in the response body and set `Content-Type` / `Content-Disposition` from metadata when known.

### Update (PUT) Algorithm

- Step 1: Authenticate and resolve the request subject.
- Step 2: [Resolve artifact_id](#resolve-artifact_id-to-metadata-and-storage_ref) to the row and obtain `storage_ref` and scope.
- Step 3: [Apply RBAC](#rbac-for-artifacts) for write (update); if not allowed, return 403.
- Step 4: Overwrite the object in the S3-like backend at `storage_ref` with the new request body.
- Step 5: Optionally update the row's metadata (e.g. `size_bytes`, `content_type`, `updated_at`).
  Return 200.

### Delete (DELETE) Algorithm

- Step 1: Authenticate and resolve the request subject.
- Step 2: [Resolve artifact_id](#resolve-artifact_id-to-metadata-and-storage_ref) to the row and obtain `storage_ref` and scope.
- Step 3: [Apply RBAC](#rbac-for-artifacts) for delete; if not allowed, return 403.
- Step 4: Delete the object from the S3-like backend (DELETE object by key).
- Step 5: Remove any [vector_items](postgres_schema.md#spec-cynai-schema-vectorstorage) rows whose `source_type` and `source_ref` reference this artifact, so vector search no longer returns chunks from it.
- Step 6: Remove the artifact metadata row or set a soft-delete flag so that future resolution of `artifact_id` yields 404.
  Return 204.

### Find (List) Algorithm

- Step 1: Authenticate the caller and resolve the request subject.
- Step 2: Parse query params: at least one filter that resolves to an allowed partition (**`scope_level`** + anchors, or `thread_id` for chat-linked artifacts, etc.); if none provided, return 400.
- Step 3: [Apply RBAC](#rbac-for-artifacts): return only rows the subject may list (owner, member, or grant).
- Step 4: Apply ordering (e.g. `created_at` desc) and pagination (`limit`, `cursor`/`offset`).
- Step 5: Return 200 with a JSON list of artifact metadata (no blob content).

## RBAC for Artifacts

- Spec ID: `CYNAI.ORCHES.ArtifactsRbac` <a id="spec-cynai-orches-artifactsrbac"></a>

Access is determined by the **subject** (authenticated user and their groups) and the artifact's **scope partition** (**user**, **group**, **project**, **global**).
**Scope defines ownership of the row** (which partition the blob lives in); **RBAC rules and role bindings** determine whether the subject may read, create, update, or delete, including **delegated access** (e.g. another project member or group member granted read/write by policy).

The implementation MUST use the same subject model as [RBAC and groups](rbac_and_groups.md#spec-cynai-access-rbacmodel) and [Projects and scope](projects_and_scopes.md#spec-cynai-access-rbacscope).

### Artifact Subject (Identity)

- The **subject** is the authenticated **user** and, when evaluated, the user's **group memberships** and **role bindings**.
- The gateway MUST resolve the subject from the session or bearer token and load groups and roles needed for policy evaluation.

### Artifact Scope and Ownership

- Each artifact row has a **`scope_level`**: `user`, `group`, `project`, or `global`.
- **User:** The partition is keyed by **`owner_user_id`** (or equivalent); `path` is unique within that user's namespace.
- **Group:** The partition is keyed by **`group_id`**; `path` is unique within that group; the row MAY record `owner_user_id` for audit (who uploaded).
- **Project:** The partition is keyed by **`project_id`**; `path` is unique within that project.
- **Global:** Deployment-wide partition; `path` uniqueness is global (or namespaced by policy); **create/update/delete** typically require elevated **global** roles.

**Jobs and tasks** are **not** scope keys.
Optional **`created_by_job_id`**, **`last_modified_by_job_id`**, and **`correlation_task_id`** exist for **audit and traceability** only.

### Allow Rules

- **Read:** Allow if the subject is permitted by **either**:
  - **Ownership:** The subject is the owning user for **user** scope, or a member of the **group** / **project** when the artifact is group- or project-scoped (per membership tables), **or**
  - **Grant:** A **role binding** or **access policy** grants the subject (or a group the subject belongs to) **artifact read** (or equivalent) on that scope (e.g. project viewer role).
- **Create / Update / Delete (write):** Allow if the subject has **write** permission on that scope partition: owner, or role that grants artifact write (e.g. project **member** or **admin**), or **global** admin for **global** artifacts.
- **Deny by default:** If no rule permits the operation, return **403**.

### Cross-Principal Access

- **Scope is not the same as "only the owner may access."**  
  The product MUST support **RBAC-controlled** read and write by **other users** (e.g. teammates on a project) or **groups** (e.g. group-scoped artifact visible to all group members) according to [Access control](access_control.md) and role definitions.
- The gateway MUST evaluate **artifact** actions (`artifact.read`, `artifact.write`, `artifact.delete` or mapped names) **consistently** for each `scope_level`.

### Permission Mapping

- Roles (owner, admin, member, viewer) and their mapping to artifact operations are defined in [RBAC and groups](rbac_and_groups.md) and [Access control](access_control.md).
  The gateway MUST treat artifact read as a read action and artifact create/update/delete as write actions when evaluating bindings.

## MCP Tooling for PMA and PAA

- Spec ID: `CYNAI.ORCHES.ArtifactsMcpForPmaPaa` <a id="spec-cynai-orches-artifactsmcpforpmapaa"></a>

- The Project Manager Agent (PMA) and Project Analyst Agent (PAA) MUST use **MCP tools** to perform artifact create, read, update, and delete operations; they MUST NOT call the artifacts REST API directly.
- The gateway MUST expose the artifacts API via MCP tools that invoke the same backend and **RBAC** as the REST endpoints ( **`scope_level`** and anchors; tasks and jobs are not the storage key).
- Tool names, argument schemas, and allowlist placement (PMA, PAA, and SBA when enabled) are defined in [Artifact tools](mcp_tools/artifact_tools.md) and [Access, allowlists, and per-tool scope](mcp_tools/access_allowlists_and_scope.md).
- Path-based tools (`artifact.put`, `artifact.get`, `artifact.list`) and unified tools (`artifacts.*`) MUST target the same S3-backed store and [RBAC](#rbac-for-artifacts) model.

## Vector Ingestion Source

- Spec ID: `CYNAI.ORCHES.ArtifactsVectorIngestSource` <a id="spec-cynai-orches-artifactsvectoringestsource"></a>

- When ingesting artifact content into the vector store (e.g. [pgvector `vector_items`](vector_storage.md#spec-cynai-schema-vectoritemstable)), the implementation MAY use the **same orchestrator artifacts store** (S3-backed blobs) as the source of content.
- The ingestion pipeline MUST resolve the artifact's `storage_ref` from the database (per [Resolve artifact_id to metadata and storage_ref](#resolve-artifact_id-to-metadata-and-storage_ref)), fetch the blob from the S3-like backend, then chunk and embed the content and write rows to `vector_items` with `source_type` (e.g. `task_artifact`) and `source_ref` set to the artifact identifier or path.
  Vector retrieval MUST remain tied back to the artifact so that [RBAC](#rbac-for-artifacts) and scope are preserved.
- Vector storage RBAC and scope (project, namespace, sensitivity) MUST be applied as defined in [Vector retrieval and RBAC](vector_storage.md#spec-cynai-schema-vectorretrievalrbac); the artifact's owning **scope** (`scope_level` and anchors) MUST be used when setting `project_id`, `group_id`, and namespace on the vector rows; **task** ids are optional correlation only.

## Database Metadata

- Spec ID: `CYNAI.ORCHES.ArtifactsDbMetadata` <a id="spec-cynai-orches-artifactsdbmetadata"></a>

- Artifact metadata and the mapping from logical identifier to blob MUST be stored in the database.
- Existing tables that reference artifact blobs (e.g. [chat_message_attachments](chat_threads_and_messages.md#spec-cynai-schema-chatmessageattachmentstable), [task_artifacts](#spec-cynai-schema-taskartifacts)) use a `storage_ref` (or equivalent) column to hold the S3 object key or equivalent reference.
- The implementation MUST ensure that any row referencing an artifact can be used to resolve `artifact_id` to `storage_ref` and **scope** (user, group, project, global) for RBAC and for fetching the blob from the S3-like backend.

## Stale Artifact Cleanup

- Spec ID: `CYNAI.ORCHES.ArtifactsStaleCleanup` <a id="spec-cynai-orches-artifactsstalecleanup"></a>

- The orchestrator MUST support **scheduled cleanup jobs** that prune stale artifacts (e.g. artifacts whose owning **project** or **group** is archived, artifacts past retention, or orphaned chat attachments when policy allows).
- Cleanup MUST be **user configurable** and **disabled by default**.
  When enabled, configuration MUST include at least: enable/disable flag, schedule (e.g. cron or interval), and policy (e.g. max age, or "prune only when owning resource is deleted/archived").
- The cleanup job MUST respect [RBAC](#rbac-for-artifacts) and MUST NOT remove artifacts that are still referenced by an active scope unless the policy explicitly allows it (e.g. age-based purge).
- Pruning MUST remove or invalidate the blob in the S3-like backend and MUST remove or soft-delete the corresponding metadata row so the artifact is no longer accessible.
- When an artifact is pruned, the orchestrator MUST also **clean up the vector store**: any [vector_items](vector_storage.md#spec-cynai-schema-vectoritemstable) rows whose `source_type` and `source_ref` reference that artifact (e.g. `source_type=task_artifact`, `source_ref` = artifact id or path) MUST be removed or invalidated so that vector search no longer returns chunks derived from the pruned artifact.

## Artifact Hashing

- Spec ID: `CYNAI.ORCHES.ArtifactsHashing` <a id="spec-cynai-orches-artifactshashing"></a>

- The orchestrator MUST compute and store a content hash (e.g. SHA-256) for artifacts for integrity and deduplication; the database column is `checksum_sha256` (or equivalent) per [chat_message_attachments](chat_threads_and_messages.md#spec-cynai-schema-chatmessageattachmentstable) and [task_artifacts](#spec-cynai-schema-taskartifacts).
- **Small artifacts:** For artifacts below an implementation-defined or configurable size threshold, the implementation MUST compute the hash **on upload** (during Create or Update) and persist it in the artifact row before responding.
- **Large artifacts:** For artifacts at or above that threshold, the implementation MAY omit hashing on upload and MUST defer hashing to a **non-busy background job** that runs during low-load periods.
- The orchestrator MUST run a **background hash update** during non-busy times to compute and store the hash for any artifact that does not yet have one (e.g. rows where `checksum_sha256` is null).
  The job MUST read the blob from the S3-like backend, compute the hash, and update the metadata row; it SHOULD be throttled or scheduled so it does not contend with user traffic.
- The size threshold that distinguishes small (hash on upload) from large (deferred) is implementation-defined or configurable (e.g. via system setting or env).

## Deferred Implementation

- Implement S3-compatible client in control-plane and/or user-gateway; add MinIO (or equivalent) service to `orchestrator/docker-compose.yml` with port 9000 and env wiring.
- Implement full artifacts CRUD and find: `POST /v1/artifacts` (Create), `GET /v1/artifacts` (Find/list by scope), `GET /v1/artifacts/{artifact_id}` (Read), `PUT /v1/artifacts/{artifact_id}` (Update), `DELETE /v1/artifacts/{artifact_id}` (Delete) with DB metadata and RBAC.
- Add or align MCP artifact tools so PMA and PAA can perform artifact CRUD via the catalog tools (see [MCP tooling for PMA and PAA](#mcp-tooling-for-pma-and-paa)); gateway implements tools by calling the artifacts API.
- Migrate or align chat upload and download_ref flows to use the artifacts API for storage and retrieval.
- Implement [stale artifact cleanup](#stale-artifact-cleanup) (configurable, disabled by default) and [artifact hashing](#artifact-hashing) (small artifacts on upload, large artifacts via non-busy background job).
