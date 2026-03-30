# Large File Upload Handling Spec Proposal

## 1 Summary

- **Date:** 2026-03-30
- **Purpose:** Draft spec for configurable upload size limits, per-scope storage quotas, chunked/multipart upload, resumable transfers, and streaming to object storage so that legitimately large files (model artifacts, datasets, large code archives, SBA output) can be uploaded without hitting the hardcoded ceiling.
- **Status:** Draft only; not yet merged into `docs/requirements/` or `docs/tech_specs/`.
- **Related:**
  - [Orchestrator Artifacts Storage](../tech_specs/orchestrator_artifacts_storage.md) (CRUD, S3 backend, hashing)
  - [Chat File Upload Storage](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-fileuploadstorage)
  - [OpenAI-Compatible Chat API - At-Reference Workflow](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-atreferenceworkflow)
  - [Go REST API Standards - Timeouts and Resource Limits](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-timeouts)
  - [`go_shared_libs/httplimits`](../../go_shared_libs/httplimits/httplimits.go) (implementation constants)
  - [Multi-Bucket Object Storage (draft 180)](180_multi_bucket_object_storage_spec_proposal.md) -- companion proposal for per-scope bucket topology
  - [System Setting MCP Tools](../tech_specs/mcp_tools/system_setting_tools.md) (admin config surface)
  - [CLI Admin Commands](../tech_specs/cynork/cli_management_app_commands_admin.md) (`cynork settings`)
  - [Web Console - System Settings](../tech_specs/web_console.md#spec-cynai-webcon-systemsettings)

## 2 Problem Statement

The implementation now enforces hard upload ceilings that were never specified; there is no mechanism for files that legitimately exceed those ceilings, no per-user or per-scope quotas, and no admin surface for managing limits.

### 2.1 Current State

[Plan 003 Task 1](../dev_docs/_plan_003_short_term.md) added `http.MaxBytesReader` and `io.LimitReader` guards across all modules, codifying three hardcoded constants in [`go_shared_libs/httplimits`](../../go_shared_libs/httplimits/httplimits.go):

- `DefaultMaxAPIRequestBodyBytes` = 10 MiB (JSON/API request bodies).
- `DefaultMaxArtifactUploadBytes` = 100 MiB (artifact blob uploads).
- `DefaultMaxHTTPResponseBytes` = 100 MiB (outbound response reads).

These limits were driven by a security review finding (unbounded reads => OOM) and address [REQ-STANDS-0104](../requirements/stands.md#req-stands-0104).

### 2.2 Spec Gap

The existing specs explicitly defer size-limit policy:

- [Orchestrator Artifacts Storage](../tech_specs/orchestrator_artifacts_storage.md#spec-cynai-orches-artifactss3backend): "File size and media-type limits are NOT specified in this spec; they MAY be added in a later revision."
- [Chat File Upload Storage](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-fileuploadstorage): "This spec does not define file size or media-type limits; they MAY be added in a later revision."

The implementation now enforces a hard 100 MiB ceiling on artifact uploads with no mechanism to:

- Configure limits per endpoint, per scope, or per tenant.
- Accept files larger than the single-request ceiling.
- Resume interrupted uploads without re-transmitting the entire payload.
- Stream through to S3 without buffering the full blob in gateway memory.
- Let administrators control per-user, per-project, or global storage quotas.

### 2.3 Affected Use Cases

- **Model artifact import:** Users importing GGUF, safetensors, or adapter weights often exceed 100 MiB (commonly 4-70 GiB).
- **Dataset uploads:** Training or evaluation datasets attached to projects.
- **Large code archives:** Compressed source trees or dependency caches for sandbox provisioning.
- **SBA output artifacts:** Job outputs that accumulate logs, generated assets, or compiled binaries.
- **Chat file references:** `@`-referenced files in chat that happen to be large (less common but possible).

## 3 Design Goals

- **Configurable limits:** Per-endpoint and per-scope (user, project, group, global) ceilings, runtime-adjustable by administrators.
- **Storage quotas:** Per-user, per-project, per-group, and global storage consumption quotas enforced at upload time.
- **Bucket-level quotas:** Total storage cap for the S3-like bucket itself, preventing unbounded growth.
- **Large-file path:** A chunked or multipart upload contract that allows files well above the single-request body limit.
- **Resumability:** Interrupted uploads can be resumed from the last acknowledged chunk.
- **Memory-safe:** The gateway streams chunks to S3; it never buffers more than one chunk in memory.
- **Backward compatible:** Existing single-request uploads continue to work for files within the inline limit.
- **Admin-first:** All limits and quotas are manageable through the existing admin surfaces (cynork CLI, web console, system settings API).

## 4 Proposed Requirements

The following requirement IDs are **proposed** and would live in [`docs/requirements/stands.md`](../requirements/stands.md) (cross-cutting), [`docs/requirements/orches.md`](../requirements/orches.md) (artifact-specific), and [`docs/requirements/client.md`](../requirements/client.md) (admin surfaces) if accepted.

### 4.1 Configurable Limits (STANDS)

- **REQ-STANDS-0140 (proposed):** Request body size limits MUST be configurable per endpoint class (API JSON, artifact upload, chat file upload) via system setting or per-scope policy.
  Implementations MUST ship safe defaults (e.g. 10 MiB for API JSON, 100 MiB for single-request artifact uploads) that take effect when no override is configured.
- **REQ-STANDS-0141 (proposed):** When a request exceeds the configured limit, the server MUST reject it with HTTP 413 (Payload Too Large) and a standard error body that includes the configured limit and the observed size (or an indication that the limit was exceeded).

### 4.2 Storage Quotas (ORCHES)

- **REQ-ORCHES-0175 (proposed):** The system MUST support configurable storage quotas that cap the total artifact storage consumed by a scope partition (user, group, project, or global).
  Quotas MUST be enforced at upload time: if completing the upload would cause the partition to exceed its quota, the server MUST reject the upload before writing the blob.
- **REQ-ORCHES-0176 (proposed):** Quota values MUST be configurable per scope partition via system settings.
  When no per-partition quota is set, the system MUST fall back to a global default quota.
  A quota value of zero or unset MUST mean "no limit" (quota enforcement disabled for that partition).
- **REQ-ORCHES-0177 (proposed):** The system MUST track current storage consumption per scope partition (sum of `size_bytes` for all artifacts in that partition) and MUST expose this value to administrators and to the owning user.
- **REQ-ORCHES-0178 (proposed):** The system MUST support a global bucket-level storage quota that caps the total bytes stored in the S3-like backend across all scope partitions.
  When the bucket quota is reached, all uploads MUST be rejected until storage is freed.

### 4.3 Chunked Upload (ORCHES)

- **REQ-ORCHES-0170 (proposed):** The artifacts API MUST support a chunked upload contract for files that exceed the single-request body limit.
  The contract MUST allow the client to upload a file in multiple sequential requests (chunks), each independently size-limited, and the server MUST assemble the chunks into a single artifact blob in the S3-like backend.
- **REQ-ORCHES-0171 (proposed):** Each chunk MUST be individually acknowledged by the server before the client sends the next chunk.
  The server MUST track upload progress and MUST allow the client to query which byte ranges have been received.
- **REQ-ORCHES-0172 (proposed):** Chunked uploads MUST have a configurable maximum total size (e.g. per-scope or global ceiling).
  The server MUST reject initiation of an upload whose declared total size exceeds this ceiling.
- **REQ-ORCHES-0173 (proposed):** Incomplete chunked uploads that receive no new chunk within a configurable timeout MUST be cleaned up (partial blobs removed, metadata marked abandoned).

### 4.4 Resumable Upload (ORCHES)

- **REQ-ORCHES-0174 (proposed):** The chunked upload contract MUST support resumption: if a client connection drops mid-transfer, the client MUST be able to query the server for the last acknowledged byte offset and resume from that point.
  The server MUST NOT require re-transmission of previously acknowledged chunks.

### 4.5 Streaming and Memory Safety (STANDS)

- **REQ-STANDS-0142 (proposed):** When handling chunked or multipart uploads, the gateway MUST stream each chunk directly to the storage backend.
  The gateway MUST NOT buffer more than one chunk (or a bounded implementation-defined buffer) in process memory at any time during an upload.

### 4.6 Chat File Upload Limits (USRGWY)

- **REQ-USRGWY-0150 (proposed):** The chat file upload endpoint MUST enforce a configurable per-file size limit.
  Files that exceed the limit MUST be rejected with HTTP 413 before the server reads the full body.
  The default limit SHOULD be the same as the artifact upload inline limit (e.g. 100 MiB).
- **REQ-USRGWY-0151 (proposed):** The chat upload path SHOULD support chunked upload for files that exceed the inline limit, reusing the same contract as the artifacts chunked upload.

### 4.7 Admin Management Surface (CLIENT)

- **REQ-CLIENT-0200 (proposed):** The cynork CLI and the web console MUST allow system administrators to view and configure upload size limits and storage quotas.
  Both clients MUST surface the same capabilities (capability parity).
- **REQ-CLIENT-0201 (proposed):** Administrators MUST be able to view current storage consumption per scope partition (user, group, project, global) and the configured quota for each.
- **REQ-CLIENT-0202 (proposed):** Administrators MUST be able to set, update, and remove per-scope upload size limit overrides and storage quotas.
- **REQ-CLIENT-0203 (proposed):** Non-admin users MUST be able to view their own storage consumption and quota status (consumed bytes, quota ceiling, percentage used) but MUST NOT be able to modify quotas.

## 5 Proposed Spec Additions

These would extend or be referenced from existing specs.

### 5.1 `UploadLimitsConfig` Type

- Spec ID (proposed): `CYNAI.STANDS.UploadLimitsConfig`

A configuration type that defines upload-size ceilings.
Implementations source values from system settings (backed by PostgreSQL); environment variables provide bootstrap defaults before the database is available.
Runtime changes via system settings take effect on the next request.

Fields (all optional with defaults):

- `max_api_json_body_bytes` (int64, default: 10 MiB) -- ceiling for JSON API request bodies.
- `max_inline_upload_bytes` (int64, default: 100 MiB) -- ceiling for single-request artifact or file uploads.
- `max_chunked_upload_bytes` (int64, default: 10 GiB) -- ceiling for total size of a chunked upload session.
- `chunk_size_bytes` (int64, default: 64 MiB) -- recommended (and maximum allowed) chunk size for chunked uploads.
- `max_chat_file_bytes` (int64, default: 100 MiB) -- ceiling for per-file chat uploads; falls back to `max_inline_upload_bytes` when unset.
- `abandoned_upload_timeout` (duration, default: 24h) -- time after which an incomplete chunked upload is cleaned up.
- `max_concurrent_chunked_uploads_per_user` (int, default: 3) -- maximum number of active chunked upload sessions per user.

#### 5.1.1 `UploadLimitsConfig` System Setting Keys

Values are stored in the system settings table (see [System Setting MCP Tools](../tech_specs/mcp_tools/system_setting_tools.md)).
Admins manage them via `cynork settings set <key> <value>` or the web console system settings panel.

- `upload.max_api_json_body_bytes` (value_type: `int64`)
- `upload.max_inline_bytes` (value_type: `int64`)
- `upload.max_chunked_bytes` (value_type: `int64`)
- `upload.chunk_size_bytes` (value_type: `int64`)
- `upload.max_chat_file_bytes` (value_type: `int64`)
- `upload.abandoned_timeout` (value_type: `duration`)
- `upload.max_concurrent_chunked_per_user` (value_type: `int`)

#### 5.1.2 `UploadLimitsConfig` Environment Variable Bootstrapping

When the system settings table is not yet populated (first boot, migration), environment variables provide bootstrap defaults.
Once the corresponding system setting key exists in the database, it takes precedence over the environment variable.

- `UPLOAD_MAX_API_JSON_BODY_BYTES`
- `UPLOAD_MAX_INLINE_BYTES`
- `UPLOAD_MAX_CHUNKED_BYTES`
- `UPLOAD_CHUNK_SIZE_BYTES`
- `UPLOAD_MAX_CHAT_FILE_BYTES`
- `UPLOAD_ABANDONED_TIMEOUT`
- `UPLOAD_MAX_CONCURRENT_CHUNKED_PER_USER`

#### 5.1.3 Per-Scope Upload Limit Overrides

Administrators MAY set per-scope overrides that raise or lower the global default for a specific user, group, or project.
Overrides are stored as system settings with a scoped key convention:

- `upload.max_inline_bytes.user.<user_id>`
- `upload.max_chunked_bytes.project.<project_id>`
- `upload.max_chunked_bytes.group.<group_id>`

Resolution order (first match wins):

1. Per-user override (if the scope is user or the uploading user has a personal override).
1. Per-project override (if the upload targets a project-scoped partition).
1. Per-group override (if the upload targets a group-scoped partition).
1. Global system setting.
1. Environment variable bootstrap.
1. Hardcoded default.

### 5.2 `StorageQuotaConfig` Type

- Spec ID (proposed): `CYNAI.ORCHES.StorageQuotaConfig`

Storage quotas cap the total artifact bytes stored within a scope partition or across the entire bucket.

#### 5.2.1 Quota System Setting Keys

- `quota.storage.default_per_user_bytes` (int64, default: 0 = unlimited) -- default quota for each user partition.
- `quota.storage.default_per_project_bytes` (int64, default: 0 = unlimited) -- default quota for each project partition.
- `quota.storage.default_per_group_bytes` (int64, default: 0 = unlimited) -- default quota for each group partition.
- `quota.storage.global_bucket_bytes` (int64, default: 0 = unlimited) -- total bytes allowed in the S3-like bucket.

Per-partition overrides follow the same scoped key convention:

- `quota.storage.user.<user_id>` (int64) -- quota for a specific user.
- `quota.storage.project.<project_id>` (int64) -- quota for a specific project.
- `quota.storage.group.<group_id>` (int64) -- quota for a specific group.

#### 5.2.2 Quota Enforcement Algorithm

At upload initiation (both inline and chunked):

1. Determine the target scope partition from the request.
1. Compute `current_usage` = sum of `size_bytes` for all artifacts in the partition.
1. Compute `projected_usage` = `current_usage` + declared upload size.
1. Resolve the effective quota for the partition (per-scope override or default).
1. If effective quota > 0 and `projected_usage` > effective quota, reject with 413 and a body that includes `current_usage`, `quota`, and `requested_size`.
1. If `quota.storage.global_bucket_bytes` > 0, also check global usage; reject if global quota would be exceeded.
1. On upload completion, update the cached partition usage.

Usage tracking SHOULD be maintained as a cached aggregate (e.g. a materialized counter per partition) that is updated on artifact create and delete, rather than re-summing on every upload.

#### 5.2.3 Quota Usage Visibility

The system MUST expose quota usage to users and administrators.

For users:

- `GET /v1/storage/usage/me` -- returns the authenticated user's storage consumption across their user-scoped artifacts and any project/group partitions they belong to.
  Response: `user_usage_bytes`, `user_quota_bytes`, `user_usage_percent`, plus a list of project/group partitions with their usage and quota.

For administrators:

- `GET /v1/storage/usage` -- returns a summary of storage usage for all partitions (optionally filtered by scope level or anchor).
  Response: list of partitions with `scope_level`, anchor id, `usage_bytes`, `quota_bytes`, `usage_percent`.
- `GET /v1/storage/usage/bucket` -- returns global bucket usage: `total_bytes`, `bucket_quota_bytes`, `usage_percent`, `artifact_count`.

### 5.3 Admin Control Surfaces

Both the cynork CLI and the web console MUST surface upload limits and quota management with capability parity.

#### 5.3.1 Cynork CLI Commands

Upload limits (via `cynork settings`):

- `cynork settings list --prefix upload.` -- list all upload-related settings.
- `cynork settings set upload.max_inline_bytes 524288000` -- set the inline upload limit to 500 MiB.
- `cynork settings set upload.max_chunked_bytes.project.<project_id> 53687091200` -- set per-project chunked limit to 50 GiB.

Storage quotas (via `cynork settings`):

- `cynork settings set quota.storage.default_per_user_bytes 10737418240` -- set default per-user quota to 10 GiB.
- `cynork settings set quota.storage.user.<user_id> 53687091200` -- set specific user quota to 50 GiB.
- `cynork settings set quota.storage.global_bucket_bytes 1099511627776` -- set global bucket quota to 1 TiB.

Storage usage (new subcommand):

- `cynork storage usage` -- show the caller's own storage usage and quota.
- `cynork storage usage --scope user --id <user_id>` -- show a specific user's usage (admin only).
- `cynork storage usage --scope project --id <project_id>` -- show a project's usage.
- `cynork storage usage --scope bucket` -- show global bucket usage (admin only).
- `cynork storage uploads` -- list active chunked upload sessions (admin only).
- `cynork storage uploads abort <upload_id>` -- abort a specific upload session (admin only).

#### 5.3.2 Web Console Panels

Storage settings panel (system settings page):

- Display all `upload.*` and `quota.storage.*` settings in a grouped, editable form.
- Validate input ranges before saving (e.g. reject negative values, warn on values below 1 MiB).
- Show effective resolved value when a per-scope override exists (display both the global default and the override).

Storage dashboard panel (new page or tab):

- Display a summary card showing global bucket usage (bar chart: used vs quota).
- Display a searchable, sortable list of scope partitions with columns: scope level, anchor (user name / project name / group name), usage, quota, percentage.
- Color coding: green (<70%), yellow (70-90%), red (>90%), with a critical alert at >95%.
- Allow inline quota editing for each partition (admin role required).
- Display active chunked uploads with progress, user, and time since last chunk; allow abort.

User self-service panel (user profile or storage section):

- Display the user's own storage consumption and quota across all partitions they own or are a member of.
- Display a breakdown by partition (user personal, each project, each group).
- Display a clear "X of Y used (Z%)" indicator for each partition.
- No quota editing capability for non-admin users.

### 5.4 Chunked Upload Protocol

- Spec ID (proposed): `CYNAI.ORCHES.ChunkedUpload`

A three-phase protocol (initiate, transfer, complete) layered on top of the existing artifacts API.

#### 5.4.1 Phase 1: Initiate

`POST /v1/artifacts/uploads`

Request body (JSON):

- `filename` (string, required) -- intended filename or path.
- `content_type` (string, optional) -- media type of the final blob.
- `total_size_bytes` (int64, required) -- declared total file size.
- `scope_level` (string, required) -- same as artifacts API.
- Scope anchors (`project_id`, `group_id`, etc.) as applicable.
- Optional `job_id` / `task_id` for lineage.

Server behavior:

1. Authenticate and authorize (same RBAC as artifact create).
1. Validate `total_size_bytes` against `max_chunked_upload_bytes` (resolved for the target scope); reject with 413 if exceeded.
1. Check storage quota for the target partition; reject with 413 if completing this upload would exceed the partition or bucket quota.
1. Check per-user concurrent upload limit; reject with 429 if exceeded.
1. Generate an `upload_id` (UUID) and reserve an S3 multipart upload (or equivalent staging area).
1. Return 201 with JSON body: `upload_id`, `chunk_size_bytes` (server-mandated maximum), `expires_at`, `quota_remaining_bytes`.

Error conditions:

- 400 if `total_size_bytes` is missing or zero.
- 413 if `total_size_bytes` exceeds the configured ceiling or would exceed the storage quota.
- 429 if the per-user concurrent upload limit is reached.
- 401/403 for auth failures.

#### 5.4.2 Phase 2: Transfer Chunks

`PUT /v1/artifacts/uploads/{upload_id}/chunks/{chunk_index}`

Request body: raw bytes for this chunk.

##### 5.4.2.1 Chunk Transfer Headers

- `Content-Length` (required) -- chunk byte count.
- `Content-Range` (optional but recommended) -- `bytes <start>-<end>/<total>`.
- `X-Chunk-SHA256` (optional) -- hex-encoded SHA-256 of the chunk for integrity.

Server behavior:

1. Validate `upload_id` exists and is not expired or completed.
1. Validate `chunk_index` is the expected next chunk or a retry of a previously failed chunk.
1. Validate chunk size does not exceed `chunk_size_bytes`.
1. Stream the chunk body directly to S3 as a multipart upload part (or stage it in a temporary object).
1. If `X-Chunk-SHA256` is present, verify the hash; reject with 409 on mismatch.
1. Record the chunk as received (byte range, part ETag or equivalent).
1. Return 200 with JSON: `chunk_index`, `bytes_received`, `total_received`, `remaining`.

Error conditions:

- 404 if `upload_id` is unknown or expired.
- 409 if chunk hash mismatch.
- 413 if chunk exceeds `chunk_size_bytes`.
- 416 if `chunk_index` is out of expected sequence and is not a retry.
- 401/403 for auth failures.

#### 5.4.3 Phase 3: Complete

`POST /v1/artifacts/uploads/{upload_id}/complete`

Request body (JSON, optional):

- `checksum_sha256` (string, optional) -- full-file hash for final integrity check.

Server behavior:

1. Validate all expected chunks have been received (`total_received == total_size_bytes`).
1. Re-check storage quota (in case other uploads completed while this one was in progress); reject with 413 if exceeded.
1. Finalize the S3 multipart upload (CompleteMultipartUpload or equivalent).
1. Insert the artifact metadata row (same as single-request Create: `artifact_id`, `storage_ref`, `scope_level`, anchors, `size_bytes`, `content_type`, `checksum_sha256`).
1. If the caller supplied `checksum_sha256`, verify it against the assembled object; if mismatch, abort the multipart upload, clean up, and return 409.
1. Update the partition usage counter.
1. Remove the upload session record.
1. Return 201 with JSON: `artifact_id`, `size_bytes`, `checksum_sha256`, `partition_usage_bytes`.

Error conditions:

- 400 if chunks are incomplete.
- 409 if full-file checksum mismatch.
- 413 if storage quota would be exceeded (race condition with other uploads).
- 404 if `upload_id` is unknown or expired.
- 401/403 for auth failures.

#### 5.4.4 Resume (Query Progress)

`GET /v1/artifacts/uploads/{upload_id}`

Returns the current upload state: `upload_id`, `total_size_bytes`, `total_received`, `next_chunk_index`, `expires_at`, `status` (active, completed, abandoned).

The client uses this to determine where to resume after a connection drop.

#### 5.4.5 Abort Upload

`DELETE /v1/artifacts/uploads/{upload_id}`

Aborts an in-progress upload: the server cancels the S3 multipart upload (AbortMultipartUpload), removes partial parts, and deletes the session record.
Returns 204 on success; 404 if unknown.
Administrators MAY abort another user's upload.

#### 5.4.6 List Active Uploads (Admin)

`GET /v1/artifacts/uploads`

Returns a paginated list of all active chunked upload sessions.
Admin-only endpoint.
Response includes: `upload_id`, `user_id`, `scope_level`, `total_size_bytes`, `total_received`, `status`, `created_at`, `last_chunk_at`, `expires_at`.

### 5.5 Abandoned Upload Cleanup

- Spec ID (proposed): `CYNAI.ORCHES.AbandonedUploadCleanup`

A periodic background job that scans for upload sessions whose `expires_at` has passed or whose last chunk was received more than `abandoned_upload_timeout` ago.

For each abandoned session:

1. Abort the S3 multipart upload.
1. Remove the session record.

The job MUST be configurable (enable/disable, scan interval) and SHOULD run alongside the existing [Stale Artifact Cleanup](../tech_specs/orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsstalecleanup) job.

### 5.6 Gateway Streaming Behavior

- Spec ID (proposed): `CYNAI.STANDS.GatewayUploadStreaming`

When the gateway receives an upload (inline or chunked), it MUST stream the body to the storage backend without buffering more than one chunk (or a bounded implementation-defined buffer, no larger than `chunk_size_bytes`) in process memory.

For single-request uploads within the inline limit:

- The gateway reads the request body through `http.MaxBytesReader` (enforcing the inline ceiling) and pipes it directly to the S3 PutObject call (streaming, not buffering the whole body before write).

For chunked uploads:

- Each chunk transfer request is individually bounded by `chunk_size_bytes`.
- The gateway streams each chunk to an S3 UploadPart call.

### 5.7 Client Integration

All CyNodeAI clients that upload files MUST detect when a file exceeds the inline limit and switch to the chunked upload protocol automatically.

#### 5.7.1 Cynork CLI Upload Behavior

- For `@`-referenced files that exceed the inline upload limit, the CLI MUST automatically switch to the chunked upload protocol.
- The CLI MUST display a progress indicator showing chunk upload progress (bytes sent / total, percentage, estimated time remaining).
- On connection failure during chunked upload, the CLI MUST automatically query the upload status and resume from the last acknowledged chunk without user intervention (up to a configurable retry count, default: 5).
- Before starting a large upload, the CLI SHOULD query the user's quota and display a warning if the upload would consume more than 80% of the remaining quota.
- The CLI MUST display a clear error message when an upload is rejected due to quota limits, including the current usage, quota ceiling, and the size of the rejected upload.

#### 5.7.2 Web Console Upload Behavior

- The web console MUST detect when a selected file exceeds the inline limit and use the chunked upload protocol.
- The web console MUST display a progress bar during chunked uploads showing bytes sent, percentage, and estimated time remaining.
- The web console SHOULD support resuming a failed upload within the same browser session.
- The web console MUST display the user's current storage usage and quota alongside the file picker (e.g. "Using 2.3 GiB of 10 GiB").
- When an upload would exceed the user's quota, the web console MUST display a clear error before the upload begins.

#### 5.7.3 SBA (Sandbox Agent) Upload Behavior

- When an SBA job produces output artifacts that exceed the inline limit, the SBA (or the node-mediated delivery path) MUST use the chunked upload protocol to store the artifact.
- SBA uploads are attributed to the task's owning user/project scope for quota accounting.

## 6 Implementation Notes

This section captures guidance for implementers; it is not normative.

### 6.1 S3 Multipart Upload Mapping

The chunked upload protocol maps naturally to S3 multipart upload:

- Initiate => `CreateMultipartUpload`
- Transfer chunk => `UploadPart`
- Complete => `CompleteMultipartUpload`
- Abort => `AbortMultipartUpload`

Implementations using MinIO (dev stack) or AWS S3 (production) can use the native multipart API.
The minimum part size for S3 multipart is 5 MiB (except the last part); the proposed default `chunk_size_bytes` of 64 MiB comfortably exceeds this.

### 6.2 Upload Session Storage

Upload session state (upload_id, S3 upload ID, expected total, received chunks) can be stored in:

- PostgreSQL (preferred for consistency with other metadata).
- Redis or in-memory with a write-ahead to Postgres (if sub-second lookup is needed).

The session record is transient and is removed on complete or abandon.

### 6.3 Usage Counter Maintenance

Partition usage counters SHOULD be maintained as a denormalized column or a dedicated `storage_usage` table with one row per partition.
Counters are incremented on artifact create and decremented on artifact delete.
A periodic reconciliation job SHOULD re-sum actual artifact `size_bytes` and correct any drift (e.g. from incomplete deletes or migration).

### 6.4 Migration Path

- **Phase 1:** Make existing limits configurable via system settings with env-var bootstrapping (no protocol change; addresses REQ-STANDS-0140).
  Add quota system setting keys and enforcement at the existing single-request upload path.
  Add `cynork storage usage` and web console storage dashboard.
- **Phase 2:** Implement the chunked upload protocol on the artifacts API (addresses REQ-ORCHES-0170 through REQ-ORCHES-0174).
- **Phase 3:** Update cynork, web console, and SBA to use chunked upload when file size exceeds the inline limit.

## 7 Existing Spec Updates Required

If this proposal is accepted, the following existing specs need updates:

- [`orchestrator_artifacts_storage.md`](../tech_specs/orchestrator_artifacts_storage.md): Remove the "NOT specified" deferral in S3-Like Block Storage; add references to `UploadLimitsConfig`, `StorageQuotaConfig`, and `ChunkedUpload`; extend the Create algorithm to handle chunked path and quota checks.
- [`chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md): Remove the "does not define file size or media-type limits" deferral in Chat File Upload Storage; add size limit reference and optional chunked path.
- [`go_rest_api_standards.md`](../tech_specs/go_rest_api_standards.md): Extend the Timeouts and Resource Limits section to reference `UploadLimitsConfig` as the configuration model for body size limits.
- [`openai_compatible_chat_api.md`](../tech_specs/openai_compatible_chat_api.md): Update the At-Reference Workflow gateway contract to specify the size limit and the chunked fallback.
- [`cynork/cli_management_app_commands_admin.md`](../tech_specs/cynork/cli_management_app_commands_admin.md): Add `cynork storage` subcommand group.
- [`web_console.md`](../tech_specs/web_console.md): Add storage dashboard panel and user storage usage panel.
- [`mcp_tools/system_setting_tools.md`](../tech_specs/mcp_tools/system_setting_tools.md): Document the `upload.*` and `quota.storage.*` setting key namespaces.

## 8 Open Questions

- **Media-type restrictions:** Should this proposal also define an allowlist/denylist of accepted media types per endpoint, or keep that as a separate concern?
- **Maximum total size:** Is 10 GiB a reasonable default ceiling for chunked uploads, or should it be higher for model artifact import use cases?
- **Chat upload chunked path:** Should chat file uploads support chunked upload immediately, or should this be deferred until there is a demonstrated need for chat files larger than 100 MiB?
- **Quota soft vs hard limits:** Should the system support soft quotas (warn but allow) in addition to hard quotas (reject)?
- **Quota grace period:** Should there be a grace period after exceeding a quota during which existing artifacts are not affected but new uploads are blocked?
- **Usage counter accuracy:** Is eventual consistency acceptable for usage counters (incremented async), or must they be transactionally accurate?
- **Multi-bucket interaction:** When multiple S3 buckets are configured (see [draft 180](180_multi_bucket_object_storage_spec_proposal.md)), how do bucket-level quotas interact with partition-level quotas?
