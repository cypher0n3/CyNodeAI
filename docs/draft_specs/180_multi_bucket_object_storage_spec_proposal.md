# Multi-Bucket Object Storage Spec Proposal

## 1 Summary

- **Date:** 2026-03-30
- **Purpose:** Draft spec for configuring multiple S3-compatible storage buckets with per-user, per-group, and per-project bucket assignment; graceful behavior when no bucket is configured; and a cross-bucket transfer mechanism for moving artifacts between buckets.
- **Status:** Draft only; not yet merged into `docs/requirements/` or `docs/tech_specs/`.
- **Related:**
  - [Orchestrator Artifacts Storage](../tech_specs/orchestrator_artifacts_storage.md) (single-bucket baseline)
  - [Large File Upload Handling (draft 170)](170_large_file_upload_spec_proposal.md) (upload limits, quotas, chunked upload)
  - [RBAC and Groups](../tech_specs/rbac_and_groups.md) (scope model, roles)
  - [Projects and Scopes](../tech_specs/projects_and_scopes.md) (project entity)
  - [System Setting MCP Tools](../tech_specs/mcp_tools/system_setting_tools.md) (admin config surface)
  - [CLI Admin Commands](../tech_specs/cynork/cli_management_app_commands_admin.md) (`cynork settings`)
  - [Web Console - System Settings](../tech_specs/web_console.md#spec-cynai-webcon-systemsettings)

## 2 Problem Statement

The current spec requires a single, deployment-wide S3-compatible bucket for all artifact storage.
This limits operational flexibility in several ways.

### 2.1 Single-Bucket Limitations

- **Isolation:** All users, projects, and groups share a single bucket namespace.
  Operators cannot isolate storage by organizational unit for compliance, billing, or data-residency reasons.
- **Scalability:** A single bucket is a single point of capacity planning.
  High-volume teams may need dedicated buckets with separate throughput and retention policies.
- **Cost attribution:** With one bucket, storage cost cannot be attributed to individual teams or projects at the infrastructure layer.
- **Provider flexibility:** Different scopes may need different storage tiers (e.g. hot storage for active projects, cold for archived ones) or different providers entirely (on-prem MinIO for sensitive data, cloud S3 for general use).

### 2.2 No-Bucket Scenario

The current spec assumes the S3-like backend is always available.
There is no defined behavior for deployments where object storage is not configured (e.g. single-node hobby deployments, air-gapped environments without MinIO, early bootstrap before the storage service is provisioned).

### 2.3 Cross-Bucket Operations

When an artifact needs to move between scopes (e.g. a user promotes a personal artifact to a project, or an admin migrates a project to a different storage tier), there is no mechanism for transferring blobs between buckets.

## 3 Design Goals

- **Multi-bucket topology:** Support zero, one, or many S3-compatible buckets, each assignable to one or more scope partitions.
- **Scope-to-bucket mapping:** Administrators can assign a bucket to a user, group, project, or the global scope; unassigned scopes fall back to a default bucket.
- **No-bucket fallback:** When no S3 bucket is configured, the system uses a local filesystem fallback for blob storage, with clear limitations and warnings.
- **Cross-bucket transfer:** A controlled mechanism for moving artifacts between buckets, preserving metadata, integrity, and RBAC.
- **Admin-managed:** All bucket configuration is managed through system settings, cynork CLI, and the web console.
- **Transparent to clients:** Clients (cynork, web console, SBA) interact with the same artifacts API regardless of the underlying bucket topology.

## 4 Proposed Requirements

The following requirement IDs are **proposed** and would live in [`docs/requirements/orches.md`](../requirements/orches.md) and [`docs/requirements/client.md`](../requirements/client.md) if accepted.

### 4.1 Multi-Bucket Configuration (ORCHES)

- **REQ-ORCHES-0180 (proposed):** The system MUST support registration of zero or more S3-compatible storage backends ("buckets"), each identified by a unique `bucket_id`.
  Each bucket registration MUST include the S3 endpoint URL, access credentials (or IAM role reference), bucket name, and an optional region.
- **REQ-ORCHES-0181 (proposed):** The system MUST support designating exactly one bucket as the **default bucket**.
  All scope partitions that do not have an explicit bucket assignment MUST use the default bucket.
  If no default bucket is designated and no per-scope assignment exists for the target partition, the system MUST use the local filesystem fallback (see REQ-ORCHES-0186).
- **REQ-ORCHES-0182 (proposed):** The system MUST support assigning a bucket to a scope partition (user, group, project, or global) via system settings.
  A scope partition MUST use at most one bucket at a time.
  When a scope partition has an explicit assignment, that assignment takes precedence over the default bucket.

### 4.2 Per-Scope Bucket Assignment (ORCHES)

- **REQ-ORCHES-0183 (proposed):** Administrators MUST be able to assign a registered bucket to a specific user, group, or project scope partition so that all artifacts created within that partition are stored in the assigned bucket.
- **REQ-ORCHES-0184 (proposed):** When resolving which bucket to use for a new artifact, the system MUST follow this precedence order: (1) explicit per-scope assignment for the target partition, (2) default bucket, (3) local filesystem fallback.
- **REQ-ORCHES-0185 (proposed):** The system MUST record the `bucket_id` in the artifact metadata row so that reads, updates, and deletes route to the correct backend regardless of subsequent configuration changes.

### 4.3 No-Bucket Fallback (ORCHES)

- **REQ-ORCHES-0186 (proposed):** When no S3-compatible bucket is registered (neither default nor per-scope), the system MUST fall back to a local filesystem directory for blob storage.
  The fallback directory MUST be configurable via system setting or environment variable; the default MUST be a well-known path under the orchestrator data directory (e.g. `$ORCHESTRATOR_DATA_DIR/artifacts`).
- **REQ-ORCHES-0187 (proposed):** When operating in local filesystem fallback mode, the system MUST log a warning at startup indicating that object storage is not configured and that local-only storage limits apply.
- **REQ-ORCHES-0188 (proposed):** The local filesystem fallback MUST support the same artifact API contract (create, read, update, delete, find) as the S3 path.
  The `storage_ref` column MUST store a local path reference instead of an S3 object key.
- **REQ-ORCHES-0189 (proposed):** The local filesystem fallback MUST enforce a configurable disk usage cap to prevent unbounded disk consumption.
  When the cap is reached, uploads MUST be rejected with 507 (Insufficient Storage).
- **REQ-ORCHES-0190 (proposed):** Chunked upload (see [draft 170](170_large_file_upload_spec_proposal.md)) MUST work with the local filesystem fallback by staging chunks into a temporary directory and assembling them into the final file on completion.

### 4.4 Cross-Bucket Transfer (ORCHES)

- **REQ-ORCHES-0191 (proposed):** The system MUST support transferring an artifact from one bucket to another while preserving artifact metadata, `artifact_id`, scope, RBAC, and checksum integrity.
- **REQ-ORCHES-0192 (proposed):** Cross-bucket transfer MUST be an admin-only operation.
  The system MUST NOT allow non-admin users to trigger transfers directly.
- **REQ-ORCHES-0193 (proposed):** Cross-bucket transfer MUST be atomic from the metadata perspective: the artifact row MUST be updated to point to the new `bucket_id` and `storage_ref` only after the blob has been fully written to the destination and integrity-verified.
  If the transfer fails, the artifact MUST remain accessible from the source bucket.
- **REQ-ORCHES-0194 (proposed):** The system MUST support bulk transfer of all artifacts in a scope partition (e.g. "move all of user X's artifacts to bucket Y") as a batch operation.
- **REQ-ORCHES-0195 (proposed):** Cross-bucket transfer MUST support transfer between an S3 bucket and the local filesystem fallback in both directions (S3 => local, local => S3) to allow operators to migrate onto or off of object storage.

### 4.5 Bucket-Level Quotas (ORCHES)

- **REQ-ORCHES-0196 (proposed):** Each registered bucket MUST support an optional storage quota (maximum total bytes stored in that bucket by CyNodeAI).
  When the bucket quota is reached, uploads targeting that bucket MUST be rejected.
- **REQ-ORCHES-0197 (proposed):** Bucket-level quotas are separate from and additive to partition-level quotas (see [draft 170](170_large_file_upload_spec_proposal.md) REQ-ORCHES-0175).
  An upload MUST pass both the partition quota check and the bucket quota check.

### 4.6 Admin Management (CLIENT)

- **REQ-CLIENT-0210 (proposed):** The cynork CLI and the web console MUST allow administrators to register, list, update, and remove S3 bucket configurations.
  Both clients MUST surface the same capabilities (capability parity).
- **REQ-CLIENT-0211 (proposed):** The cynork CLI and the web console MUST allow administrators to assign and unassign buckets to scope partitions (user, group, project).
- **REQ-CLIENT-0212 (proposed):** The cynork CLI and the web console MUST allow administrators to initiate and monitor cross-bucket transfers.
- **REQ-CLIENT-0213 (proposed):** The cynork CLI and the web console MUST display the current bucket topology: which buckets are registered, which is the default, and which scope partitions have explicit assignments.

## 5 Proposed Spec Additions

These would extend or be referenced from existing specs.

### 5.1 `StorageBucket` Type

- Spec ID (proposed): `CYNAI.ORCHES.StorageBucket`

A registered storage backend that the system can use for artifact blob storage.

- `bucket_id` (uuid, pk) -- stable identifier.
- `name` (string, unique) -- human-readable label (e.g. "primary-minio", "team-alpha-s3", "local-fallback").
- `backend_type` (enum: `s3`, `local`) -- storage backend type.
- `endpoint_url` (string, nullable) -- S3 endpoint URL (required for `s3`; null for `local`).
- `bucket_name` (string, nullable) -- S3 bucket name (required for `s3`; null for `local`).
- `region` (string, nullable) -- S3 region (optional).
- `credential_ref` (string, nullable) -- reference to stored credentials (e.g. a credential_id in the credentials table or a k8s secret name).
  Credentials MUST NOT be stored in plaintext in this table; they MUST reference a secure credential store.
- `local_path` (string, nullable) -- filesystem directory (required for `local`; null for `s3`).
- `is_default` (bool, default: false) -- whether this bucket is the deployment-wide default; exactly zero or one row may have `is_default = true`.
- `quota_bytes` (int64, nullable, default: 0) -- bucket-level storage quota; 0 = unlimited.
- `usage_bytes` (int64, default: 0) -- current tracked usage.
- `status` (enum: `active`, `read_only`, `disabled`) -- operational status.
  `read_only`: existing artifacts can be read and deleted but no new artifacts can be created.
  `disabled`: no operations allowed; used during migration or decommission.
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### 5.1.1 `StorageBucket` Table Constraints

- At most one row may have `is_default = true` at any time.
- `name` is unique.
- When `backend_type = s3`: `endpoint_url` and `bucket_name` are required.
- When `backend_type = local`: `local_path` is required.

### 5.2 `ScopeBucketAssignment` Type

- Spec ID (proposed): `CYNAI.ORCHES.ScopeBucketAssignment`

Maps a scope partition to a specific bucket.

- `id` (uuid, pk)
- `scope_level` (enum: `user`, `group`, `project`, `global`)
- `scope_anchor_id` (uuid, nullable) -- the user_id, group_id, or project_id; null for global scope.
- `bucket_id` (uuid, fk to `storage_buckets.bucket_id`)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### 5.2.1 `ScopeBucketAssignment` Table Constraints

- Unique: (`scope_level`, `scope_anchor_id`) -- each partition maps to at most one bucket.
- The referenced bucket MUST exist and MUST NOT be in `disabled` status.

### 5.3 Bucket Resolution Algorithm

- Spec ID (proposed): `CYNAI.ORCHES.BucketResolution`

When the system needs to determine which bucket to use for a new artifact, it MUST follow this procedure.

#### 5.3.1 `BucketResolution` Algorithm

1. Look up `ScopeBucketAssignment` for the target (`scope_level`, `scope_anchor_id`).
1. If found and the referenced bucket is `active`, use that bucket.
1. If not found, look up the default bucket (`is_default = true` in `StorageBucket`).
1. If a default bucket exists and is `active`, use the default bucket.
1. If no assignment and no default bucket, use the local filesystem fallback.
   If the local fallback bucket is not registered, auto-register it with `backend_type = local`, `local_path = $ORCHESTRATOR_DATA_DIR/artifacts`, and `name = "local-fallback"`.
1. Record the resolved `bucket_id` in the artifact metadata row.

### 5.4 Local Filesystem Fallback

- Spec ID (proposed): `CYNAI.ORCHES.LocalFilesystemFallback`

The local filesystem fallback stores artifact blobs in a directory on the orchestrator host.
It is used when no S3 bucket is configured.

#### 5.4.1 Local Fallback Storage Layout

Blobs are stored under `<local_path>/<scope_level>/<scope_anchor_id>/<artifact_id>`.

- Example: `/data/artifacts/user/550e8400-e29b.../a1b2c3d4-e5f6.../blob`

The directory structure is created on demand.

#### 5.4.2 Local Fallback Disk Cap

The system MUST enforce a disk usage cap via the system setting `storage.local_fallback.max_bytes` (default: 10 GiB).
Before writing a blob, the system MUST check current usage; if writing would exceed the cap, reject with 507.

Usage tracking follows the same counter pattern as S3 bucket usage (see [draft 170](170_large_file_upload_spec_proposal.md) section 5.2.2).

#### 5.4.3 Local Fallback Startup Warning

At orchestrator startup, if bucket resolution would fall through to the local fallback, the system MUST log a warning:

- `"No S3-compatible object storage configured. Using local filesystem fallback at <local_path>. This mode has limited capacity and is not recommended for production. Configure a bucket via cynork settings or ARTIFACTS_S3_ENDPOINT."`

#### 5.4.4 Local Fallback Limitations

The following limitations apply in local fallback mode and MUST be documented in operator-facing docs:

- No replication or redundancy (single-host storage).
- No server-side encryption at rest (unless the host filesystem provides it).
- Disk usage cap is the only capacity control; no S3 lifecycle policies.
- Chunked upload stages to a temp directory under `<local_path>/.tmp/` and assembles on complete.
- Not suitable for multi-instance orchestrator deployments (no shared filesystem assumed).

### 5.5 Cross-Bucket Transfer Protocol

- Spec ID (proposed): `CYNAI.ORCHES.CrossBucketTransfer`

Cross-bucket transfer moves artifact blobs from a source bucket to a destination bucket while preserving all metadata.

#### 5.5.1 Single-Artifact Transfer

`POST /v1/admin/storage/transfers`

Request body (JSON):

- `artifact_id` (uuid, required) -- the artifact to transfer.
- `destination_bucket_id` (uuid, required) -- the target bucket.

Server behavior:

1. Authenticate and authorize (admin role required).
1. Resolve the artifact's current `bucket_id` and `storage_ref` from metadata.
1. If source and destination are the same bucket, return 200 (no-op).
1. Read the blob from the source bucket.
1. Write the blob to the destination bucket, generating a new `storage_ref`.
1. Verify integrity: compare `checksum_sha256` (compute on read if not stored).
1. Atomically update the artifact metadata row: set `bucket_id` = destination, `storage_ref` = new ref.
1. Delete the blob from the source bucket.
1. Update usage counters on both buckets.
1. Return 200 with: `artifact_id`, `source_bucket_id`, `destination_bucket_id`, `size_bytes`.

Error conditions:

- 404 if `artifact_id` or `destination_bucket_id` is unknown.
- 403 if caller is not admin.
- 409 if integrity check fails after write.
- 413 if the destination bucket quota would be exceeded.
- 507 if the destination is local fallback and the disk cap would be exceeded.

#### 5.5.2 Bulk Partition Transfer

`POST /v1/admin/storage/transfers/bulk`

Request body (JSON):

- `scope_level` (string, required) -- scope to transfer (user, group, project).
- `scope_anchor_id` (uuid, required) -- the partition anchor.
- `destination_bucket_id` (uuid, required) -- the target bucket.
- `update_assignment` (bool, default: true) -- whether to update the `ScopeBucketAssignment` to point the partition to the new bucket after all artifacts are transferred.

Server behavior:

1. Authenticate and authorize (admin role required).
1. List all artifacts in the source partition.
1. Validate that the destination bucket has sufficient quota for the total size.
1. Create a `transfer_job` record (UUID, status: `in_progress`, progress: 0/N).
1. Return 202 Accepted with: `transfer_job_id`, `artifact_count`, `total_bytes`.
1. Asynchronously transfer each artifact using the single-artifact transfer flow.
1. On each artifact completion, update the transfer job progress.
1. On all-complete, if `update_assignment` is true, create or update the `ScopeBucketAssignment`.
1. Set transfer job status to `completed`.

If any single artifact fails:

- Mark the transfer job as `partial_failure`.
- Continue with remaining artifacts.
- Record the failed artifact IDs in the job record.
- The failed artifacts remain in the source bucket and are still accessible.

#### 5.5.3 Transfer Job Monitoring

`GET /v1/admin/storage/transfers/{transfer_job_id}`

Returns: `transfer_job_id`, `status` (in_progress, completed, partial_failure, failed), `total_artifacts`, `transferred_artifacts`, `failed_artifacts`, `total_bytes`, `transferred_bytes`, `started_at`, `completed_at`, `failed_artifact_ids` (list).

`GET /v1/admin/storage/transfers`

Lists all transfer jobs (paginated), filterable by status.

### 5.6 Admin Control Surfaces

Both the cynork CLI and the web console MUST surface bucket management, scope assignment, and cross-bucket transfer with capability parity.

#### 5.6.1 Cynork CLI Commands

Bucket management (new `cynork storage buckets` subcommand group):

- `cynork storage buckets list` -- list all registered buckets with status, usage, quota.
- `cynork storage buckets add --name <name> --endpoint <url> --bucket <bucket_name> --credential <cred_ref> [--region <region>] [--quota <bytes>] [--default]` -- register a new S3 bucket.
- `cynork storage buckets add --name <name> --local-path <path> [--quota <bytes>] [--default]` -- register a local filesystem bucket.
- `cynork storage buckets update <bucket_id> [--name <name>] [--quota <bytes>] [--status <active|read_only|disabled>] [--default]` -- update bucket config.
- `cynork storage buckets remove <bucket_id>` -- remove a bucket (only if empty or all artifacts transferred).
- `cynork storage buckets set-default <bucket_id>` -- designate a bucket as default.

Scope assignment (new `cynork storage assign` subcommand group):

- `cynork storage assign --scope user --id <user_id> --bucket <bucket_id>` -- assign user to bucket.
- `cynork storage assign --scope project --id <project_id> --bucket <bucket_id>` -- assign project to bucket.
- `cynork storage assign --scope group --id <group_id> --bucket <bucket_id>` -- assign group to bucket.
- `cynork storage assign list` -- list all scope-to-bucket assignments.
- `cynork storage assign remove --scope <scope> --id <anchor_id>` -- remove assignment (partition falls back to default).

Transfer (new `cynork storage transfer` subcommand group):

- `cynork storage transfer artifact <artifact_id> --to <bucket_id>` -- transfer a single artifact.
- `cynork storage transfer partition --scope <scope> --id <anchor_id> --to <bucket_id> [--update-assignment]` -- transfer all artifacts in a partition.
- `cynork storage transfer status <transfer_job_id>` -- check transfer job progress.
- `cynork storage transfer list` -- list transfer jobs.

Topology overview:

- `cynork storage topology` -- display a tree view of the bucket topology: registered buckets, default designation, per-scope assignments, usage summary.

#### 5.6.2 Web Console Panels

Bucket management panel (admin settings):

- List of registered buckets with columns: name, type (S3/local), endpoint, status, usage, quota, is-default.
- Add/edit/remove bucket dialogs.
- Set-default toggle.
- Status toggle (active / read-only / disabled) with confirmation dialog.

Scope assignment panel (admin settings):

- Searchable list of users, groups, and projects with their current bucket assignment (or "default").
- Assign/unassign bucket via dropdown per scope.
- Visual indicator when a scope uses a non-default bucket.

Transfer panel (admin settings):

- Initiate transfer: select artifact or partition, select destination bucket, optional update-assignment checkbox.
- Active transfers: progress bars, artifact counts, ETA.
- Transfer history: past transfers with status, duration, artifact count.

Topology dashboard (admin overview):

- Visual diagram or tree showing: registered buckets (boxes), scope assignments (arrows from scope partitions to buckets), default bucket highlighted.
- Per-bucket usage bars (used vs quota).
- Summary stats: total buckets, total storage, partitions with custom assignments.

User-facing storage panel (non-admin):

- The user sees which bucket their artifacts are stored in (by name, not by endpoint or credential details).
- The user does not see other users' bucket assignments or admin-only details.

### 5.7 Artifact Metadata Extension

The `artifacts` table (see [Orchestrator Artifacts Storage](../tech_specs/orchestrator_artifacts_storage.md#spec-cynai-orches-artifactstablemetadata)) requires a new column:

- `bucket_id` (uuid, nullable, fk to `storage_buckets.bucket_id`) -- the bucket where this artifact's blob is stored.
  Null for artifacts created before multi-bucket support (these use the default bucket or are migrated).

The combination of `bucket_id` + `storage_ref` uniquely identifies a blob in the storage layer.

### 5.8 Credentials Security

Bucket credentials (access keys, secret keys, IAM role ARNs) MUST NOT be stored in the `storage_buckets` table directly.
They MUST reference the existing credential store (see [Credential Management](../tech_specs/cynork/cli_management_app_commands_admin.md#spec-cynai-client-clicredential) and [API Egress Credentials](../tech_specs/api_egress_server.md)).

- For S3 buckets that use access key + secret key: the admin registers a credential via `cynork creds add` (or the web console credentials panel) and references it by `credential_id` in the bucket configuration.
- For S3 buckets that use IAM instance roles or pod service accounts: `credential_ref` MAY be set to a sentinel value like `iam-role` and no credential record is needed.
- The gateway MUST resolve credentials at request time from the secure credential store; it MUST NOT cache decrypted credentials in memory longer than the duration of the S3 operation.

## 6 Implementation Notes

This section captures guidance for implementers; it is not normative.

### 6.1 S3 Client Pool

When multiple S3 buckets are registered, the gateway SHOULD maintain a pool of pre-configured S3 clients (one per bucket) to avoid per-request client creation overhead.
Clients SHOULD be lazily initialized and evicted on bucket removal or credential rotation.

### 6.2 Local Fallback Implementation

The local fallback can be implemented as a simple `os.Create` / `os.Open` / `os.Remove` wrapper that satisfies the same `BlobStore` interface as the S3 client.
This allows the artifacts service to be polymorphic over the storage backend.

### 6.3 Transfer Streaming

For cross-bucket transfers between two S3 buckets in the same region, implementations SHOULD use server-side copy (`CopyObject`) when both endpoints support it, avoiding downloading and re-uploading the blob through the gateway.

For transfers involving different providers, different regions, or the local fallback, the gateway MUST stream the blob through memory (bounded to one chunk at a time) from source to destination.

### 6.4 Migration Path

- **Phase 1:** Add `storage_buckets` and `scope_bucket_assignments` tables.
  Add `bucket_id` column to artifacts table.
  Existing single-bucket deployments auto-register their current bucket as the default.
  The local fallback is auto-registered if no S3 env vars are set.
- **Phase 2:** Implement bucket resolution in the artifacts service.
  Add admin CLI and web console commands for bucket and assignment management.
- **Phase 3:** Implement cross-bucket transfer (single and bulk).
  Add transfer monitoring.
- **Phase 4:** Implement topology dashboard in the web console.

### 6.5 Backward Compatibility

Existing deployments that use a single bucket configured via `ARTIFACTS_S3_ENDPOINT`, `ARTIFACTS_S3_ACCESS_KEY`, `ARTIFACTS_S3_SECRET_KEY`, and `ARTIFACTS_S3_BUCKET` environment variables MUST continue to work without configuration changes.

On first startup after upgrade:

1. If `ARTIFACTS_S3_*` env vars are set and no `storage_buckets` rows exist, auto-register a bucket with `backend_type = s3`, `is_default = true`, and the env-var values.
1. If no `ARTIFACTS_S3_*` env vars are set and no `storage_buckets` rows exist, auto-register the local fallback.
1. Existing artifact rows with null `bucket_id` are treated as belonging to the default bucket.

## 7 Existing Spec Updates Required

If this proposal is accepted, the following existing specs need updates:

- [`orchestrator_artifacts_storage.md`](../tech_specs/orchestrator_artifacts_storage.md): Extend the S3-Like Block Storage section to reference multi-bucket topology; add `bucket_id` to the artifacts table; update all CRUD algorithms to resolve bucket via `BucketResolution`; add the local filesystem fallback as an alternative backend.
- [`postgres_schema.md`](../tech_specs/postgres_schema.md): Add `storage_buckets` and `scope_bucket_assignments` tables; add `bucket_id` column to the artifacts table.
- [`cynork/cli_management_app_commands_admin.md`](../tech_specs/cynork/cli_management_app_commands_admin.md): Add `cynork storage buckets`, `cynork storage assign`, `cynork storage transfer`, and `cynork storage topology` subcommand groups.
- [`web_console.md`](../tech_specs/web_console.md): Add bucket management, scope assignment, transfer, and topology panels.
- [`mcp_tools/system_setting_tools.md`](../tech_specs/mcp_tools/system_setting_tools.md): Document `storage.*` setting key namespace.
- [`orchestrator_bootstrap.md`](../tech_specs/orchestrator_bootstrap.md): Document auto-registration of default bucket from env vars on first startup.

## 8 Open Questions

- **Cross-region transfer optimization:** Should the system support S3 cross-region replication as an alternative to gateway-mediated transfer for buckets in different regions?
- **Bucket health monitoring:** Should the system periodically probe bucket endpoints for availability and surface health status to administrators?
- **Automatic tiering:** Should the system support automatic artifact migration between storage tiers (e.g. move artifacts older than N days from hot to cold bucket), or should this be admin-initiated only?
- **Multi-instance local fallback:** For multi-instance orchestrator deployments, should the local fallback require a shared filesystem (e.g. NFS), or should it be disabled entirely when multiple instances are detected?
- **Bucket-per-scope auto-creation:** Should the system support automatically creating a new S3 bucket (via the S3 API) when a new project or group is created, or must buckets always be pre-provisioned?
- **Encryption at rest:** Should per-bucket encryption-at-rest configuration (SSE-S3, SSE-KMS, SSE-C) be part of the bucket registration, or is this an infrastructure concern outside the application spec?
- **Relationship to draft 170 quotas:** Bucket-level quotas (REQ-ORCHES-0196) and partition-level quotas (draft 170 REQ-ORCHES-0175) are independent.
  Should there be a UI that shows the combined view (partition quota within a bucket vs bucket quota) to avoid admin confusion?
