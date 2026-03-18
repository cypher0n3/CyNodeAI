# Orchestrator Self-Metadata and Logging (Proposal)

- [Scope and Metadata](#scope-and-metadata)
- [Summary](#summary)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals (Scope)](#goals-scope)
  - [Non-Goals (Out of Scope)](#non-goals-out-of-scope)
- [Spec Items](#spec-items)
  - [Document and Traces](#document-and-traces)
  - [Blob Storage Utilization](#blob-storage-utilization)
  - [Utilization Windows and Downtime Scheduling](#utilization-windows-and-downtime-scheduling)
  - [Component and Scheduler Self-Metadata](#component-and-scheduler-self-metadata)
  - [Self-Metadata Logging](#self-metadata-logging)
  - [Exposure and API](#exposure-and-api)
- [References](#references)

## Scope and Metadata

- Date: 2026-03-18
- Status: Proposal (draft_specs; not merged to tech_specs)
- Scope: Orchestrator tracking and logging of its own state and components for operational use: blob storage utilization, utilization time windows (for auto-scheduling downtime tasks), and related self-metadata.

## Summary

The orchestrator should maintain structured metadata about itself and its components (artifact blob storage usage, workload utilization over time, queue and schedule health, and component availability) and log this information in a consistent, operator-friendly way.
This metadata enables the orchestrator to make better scheduling decisions (e.g. running maintenance or cleanup during low-utilization windows) and gives operators visibility without requiring ad-hoc queries to multiple backends.

## Goals and Non-Goals

The following goals and non-goals bound this proposal.

### Goals (Scope)

- Track artifact blob storage: available capacity vs used (and optionally growth rate) so the orchestrator and operators can react to capacity pressure.
- Track utilization over time (e.g. job dispatch rate, queue depth, active job count per window) and derive high- and low-utilization windows so the orchestrator can auto-schedule downtime-sensitive tasks (e.g. stale artifact cleanup, background hash updates, compaction) during low-utilization periods.
- Maintain a minimal, well-defined set of self-metadata (storage, utilization, component health summary) that the orchestrator can use for decisions and that can be exposed to operators.
- Emit structured logs for self-metadata and key lifecycle events so operators can monitor and debug without depending on multiple internal APIs.

### Non-Goals (Out of Scope)

- Replacing or duplicating the Worker Telemetry API or node-level metrics; this spec is orchestrator-centric self-metadata only.
- Defining a full observability stack (metrics backend, dashboards, alerting); the spec defines what the orchestrator tracks and logs, not how external systems consume it.
- Changing existing health or readiness semantics (`/healthz`, `/readyz`).

## Spec Items

This section defines the spec items for orchestrator self-metadata and logging.

### Document and Traces

Overview and traceability for this document.

#### 1 `Doc` Document Overview

- Spec ID: `CYNAI.ORCHES.Doc.OrchestratorSelfMetadata` <a id="spec-cynai-orches-doc-orchestratorselfmetadata"></a>

This document defines the metadata the orchestrator MUST or SHOULD track about itself and its components, and how it MUST log that information for operational use.
Implementations use this metadata to schedule maintenance and cleanup during low-utilization windows and to expose a concise view of orchestrator and storage health.

**Traces To:** [REQ-ORCHES-0100](../requirements/orches.md#req-orches-0100), [REQ-ORCHES-0101](../requirements/orches.md#req-orches-0101), [REQ-ORCHES-0105](../requirements/orches.md#req-orches-0105), [REQ-ORCHES-0127](../requirements/orches.md#req-orches-0127), [REQ-ORCHES-0167](../requirements/orches.md#req-orches-0167), [Orchestrator Artifacts Storage](../tech_specs/orchestrator_artifacts_storage.md), [Orchestrator](../tech_specs/orchestrator.md).

### Blob Storage Utilization

Blob storage utilization snapshot type and tracking rule.

#### 1 `Type` Blob Storage Utilization Snapshot

- Spec ID: `CYNAI.ORCHES.Type.BlobStorageUtilizationSnapshot` <a id="spec-cynai-orches-type-blobstorageutilizationsnapshot"></a>

A snapshot of artifact blob storage utilization at a point in time.

- **`capacity_bytes`** (optional): Total usable capacity of the S3-like backend (or bucket) in bytes, when the backend exposes it (e.g. MinIO admin API, S3 bucket metrics).
  When the backend does not expose capacity, this field MAY be omitted.
- **`used_bytes`**: Total size of blob data stored in the artifact backend, in bytes.
  The implementation MUST compute this from the sum of artifact sizes in the database (artifact metadata rows that reference the S3-like backend) or from a backend-provided metric when available and trusted.
- **`artifact_count`**: Number of artifact blobs (rows with a non-null `storage_ref`) that contribute to `used_bytes`.
- **`collected_at`**: Timestamp (UTC) when the snapshot was collected.

The orchestrator MUST be able to produce this snapshot on demand (e.g. for a status endpoint or admin API) and MAY periodically sample and store or log it (see [Self-Metadata Logging](#spec-cynai-orches-rule-selfmetadatalogging)).

**Traces To:** [REQ-ORCHES-0127](../requirements/orches.md#req-orches-0127), [REQ-ORCHES-0167](../requirements/orches.md#req-orches-0167), [Orchestrator Artifacts Storage](../tech_specs/orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsdbmetadata).

#### 2 `Rule` Blob Storage Utilization Tracking

- Spec ID: `CYNAI.ORCHES.Rule.BlobStorageUtilizationTracking` <a id="spec-cynai-orches-rule-blobstorageutilizationtracking"></a>

The orchestrator MUST track artifact blob storage utilization (available vs used) for the S3-like backend used for artifacts.
Tracking MUST include at least: total used bytes (from artifact metadata or backend) and, when the backend exposes it, total available capacity.
The orchestrator SHOULD refresh this snapshot on a configurable interval (e.g. every N minutes) or on demand so that scheduling and cleanup logic (e.g. [Stale Artifact Cleanup](../tech_specs/orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsstalecleanup), [Artifact Hashing](../tech_specs/orchestrator_artifacts_storage.md#spec-cynai-orches-artifactshashing)) can consider current utilization.
The implementation MAY persist the latest snapshot in memory or in a small table/cache; it MUST NOT block readiness or task dispatch on collection failure (degraded metadata only).

**Traces To:** [Orchestrator Artifacts Storage](../tech_specs/orchestrator_artifacts_storage.md).

### Utilization Windows and Downtime Scheduling

Utilization window type, downtime-scheduling rule, and recording operation.

#### 1 `Type` Utilization Window

- Spec ID: `CYNAI.ORCHES.Type.UtilizationWindow` <a id="spec-cynai-orches-type-utilizationwindow"></a>

A time window with an associated utilization level, used to decide when to run maintenance or low-priority jobs.

- **`start_at`**, **`end_at`**: Window boundaries (UTC).
- **`level`**: Enum or label: `high`, `medium`, `low`.
  `low` indicates a period suitable for downtime-sensitive or background work (e.g. cleanup, hashing, compaction).
- **`metric`** (optional): The metric used to derive the level (e.g. `active_job_count`, `dispatch_rate`, `queue_depth`).
- **`value`** (optional): Numeric value of the metric over the window (e.g. average or peak) for logging and debugging.

The orchestrator derives utilization from scheduler and queue state (see [Component and Scheduler Self-Metadata](#component-and-scheduler-self-metadata) and [Scheduler Metrics for Utilization](#spec-cynai-orches-rule-schedulermetricsforutilization)).

#### 2 `Rule` Utilization-Based Downtime Scheduling

- Spec ID: `CYNAI.ORCHES.Rule.UtilizationBasedDowntimeScheduling` <a id="spec-cynai-orches-rule-utilizationbaseddowntimescheduling"></a>

The orchestrator MUST use utilization metadata to prefer running **downtime-sensitive** tasks during **low-utilization** windows.
Downtime-sensitive tasks include: stale artifact cleanup, background artifact hash updates, and any other scheduled job that is explicitly marked or configured as "run during low utilization."
The implementation MUST define a notion of "current utilization" (e.g. rolling window of active job count, queue depth, or dispatch rate) and a threshold or policy that classifies a window as low utilization (e.g. active jobs below N, or queue depth below M for the last K minutes).
When a downtime-sensitive task is due to run (cron or schedule), the scheduler SHOULD defer execution until the next low-utilization window if the current utilization is not low, subject to a maximum deferral bound (e.g. do not defer more than X hours) so that the task eventually runs.
The orchestrator MAY expose configuration for: utilization metric choice, low-utilization threshold, and max deferral.

**Traces To:** [REQ-ORCHES-0100](../requirements/orches.md#req-orches-0100), [REQ-ORCHES-0101](../requirements/orches.md#req-orches-0101), [Orchestrator Artifacts Storage - Stale Artifact Cleanup](../tech_specs/orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsstalecleanup), [Artifact Hashing](../tech_specs/orchestrator_artifacts_storage.md#spec-cynai-orches-artifactshashing).

#### 3 `Operation` Record Utilization and High/Low Windows

- Spec ID: `CYNAI.ORCHES.Operation.RecordUtilizationWindows` <a id="spec-cynai-orches-operation-recordutilizationwindows"></a>

The orchestrator MUST periodically (e.g. every 1-5 minutes) record a utilization sample: the chosen metric value (e.g. active job count, queue depth) and a timestamp.
From a rolling window of samples (e.g. last 24-48 hours), the implementation MUST be able to compute or approximate **highest** and **lowest** utilization times (e.g. "peak hour" and "quiet hour" in a day) for use by the scheduler and for optional exposure to operators.
The implementation MAY store only aggregated statistics (e.g. min/max per hour or per day) to bound storage; it MUST retain enough history to support "next low-utilization window" for deferral of downtime-sensitive tasks.
Retention period and aggregation granularity are implementation-defined or configurable.

**Traces To:** [REQ-ORCHES-0100](../requirements/orches.md#req-orches-0100), [REQ-ORCHES-0105](../requirements/orches.md#req-orches-0105).

### Component and Scheduler Self-Metadata

Summary type and scheduler metrics rule for utilization.

#### 1 `Type` Orchestrator Self-Metadata Summary

- Spec ID: `CYNAI.ORCHES.Type.OrchestratorSelfMetadataSummary` <a id="spec-cynai-orches-type-orchestratorselfmetadatasummary"></a>

A concise summary of orchestrator self-metadata for exposure via status or admin API.

- **`blob_storage`**: A [Blob Storage Utilization Snapshot](#spec-cynai-orches-type-blobstorageutilizationsnapshot) (or null if not yet collected).
- **`utilization`** (optional): Current utilization level (`high` / `medium` / `low`) and, if available, the next low-utilization window start (UTC).
- **`scheduler`** (optional): Summary of scheduler state: e.g. `pending_job_count`, `active_job_count`, `last_dispatch_at` (UTC), and optionally `schedule_run_count_24h` (number of scheduled runs in the last 24 hours).
- **`components`** (optional): High-level component health summary (e.g. control-plane ready, PMA status, worker node count, artifact backend reachable).
  This MAY align with the detailed status shape in [status_command_detailed_health_spec_proposal.md](status_command_detailed_health_spec_proposal.md) without duplicating it here.

All optional fields MAY be omitted when the implementation does not yet support them or when collection fails (degraded metadata).

#### 2 `Rule` Scheduler Metrics for Utilization

- Spec ID: `CYNAI.ORCHES.Rule.SchedulerMetricsForUtilization` <a id="spec-cynai-orches-rule-schedulermetricsforutilization"></a>

The orchestrator MUST expose to its own scheduling and metadata logic at least one of the following, for use as the utilization metric in [Utilization-Based Downtime Scheduling](#spec-cynai-orches-rule-utilizationbaseddowntimescheduling) and [Record Utilization Windows](#spec-cynai-orches-operation-recordutilizationwindows):

- **Active job count**: Number of jobs currently dispatched and not yet in a terminal state (running or in-flight).
- **Queue depth**: Number of pending tasks or jobs in the scheduler queue awaiting dispatch.

The implementation MAY use both (e.g. weighted or max) to derive utilization level.
The implementation MUST update this metric on every dispatch, completion, and queue change so that utilization samples reflect current load.

**Traces To:** [REQ-ORCHES-0100](../requirements/orches.md#req-orches-0100), [orchestrator.md - Task Scheduler](../tech_specs/orchestrator.md#spec-cynai-orches-taskscheduler).

### Self-Metadata Logging

Structured logging rule for self-metadata and lifecycle events.

#### 1 `Rule` Self-Metadata Logging

- Spec ID: `CYNAI.ORCHES.Rule.SelfMetadataLogging` <a id="spec-cynai-orches-rule-selfmetadatalogging"></a>

The orchestrator MUST log self-metadata and key lifecycle events in a structured format (e.g. JSON or key-value fields) so that operators can monitor and debug without querying internal APIs.
At minimum, the orchestrator MUST log:

- **Blob storage**: When a [Blob Storage Utilization Snapshot](#spec-cynai-orches-type-blobstorageutilizationsnapshot) is collected (periodically or on demand), log at least `used_bytes`, `artifact_count`, and optionally `capacity_bytes` and `collected_at`.
  Log level SHOULD be info or debug to avoid noise; the implementation MAY rate-limit (e.g. at most once per N minutes).
- **Utilization**: When utilization level changes (e.g. from high to low), log the new level and timestamp.
  Optionally log periodic utilization samples at debug level.
- **Downtime-sensitive run**: When a downtime-sensitive task is run or deferred (and why), log task/schedule id, decision (run vs deferred), and if deferred the reason (e.g. "utilization not low") and next eligible window if known.
- **Readiness and PMA**: Existing readiness and PMA lifecycle events MAY be kept as today; this spec does not require new events beyond ensuring that self-metadata collection failures do not spam logs (degraded metadata only, log once or at low frequency).

The implementation MUST NOT log sensitive data (credentials, user content, or artifact bodies); only aggregate counts, sizes, and identifiers as needed.

**Traces To:** [REQ-ORCHES-0120](../requirements/orches.md#req-orches-0120), [Orchestrator Artifacts Storage](../tech_specs/orchestrator_artifacts_storage.md).

### Exposure and API

Operation to expose self-metadata to operators.

#### 1 `Operation` Expose Self-Metadata

- Spec ID: `CYNAI.ORCHES.Operation.ExposeSelfMetadata` <a id="spec-cynai-orches-operation-exposeselfmetadata"></a>

The orchestrator SHOULD expose the [Orchestrator Self-Metadata Summary](#spec-cynai-orches-type-orchestratorselfmetadatasummary) to authenticated operators.
Exposure MAY be via:

- An extension to the control-plane or User API Gateway detailed status endpoint (e.g. `GET /v1/status` or equivalent as in [status_command_detailed_health_spec_proposal.md](status_command_detailed_health_spec_proposal.md)), with a subsection or query parameter for self-metadata, or
- A dedicated admin-only endpoint (e.g. `GET /v1/admin/orchestrator/metadata` or similar) that returns the summary.

The response MUST be JSON and MUST include at least `blob_storage` when collection is enabled and has succeeded; other fields are optional.
When blob or utilization collection is disabled or has never run, the implementation MUST return null or omit the corresponding fields rather than failing the request.
Access to this endpoint MUST be restricted to authenticated users with admin or operator role (per [Access control](../tech_specs/access_control.md) and [RBAC](../tech_specs/rbac_and_groups.md)).

**Traces To:** [REQ-ORCHES-0120](../requirements/orches.md#req-orches-0120), [status_command_detailed_health_spec_proposal.md](status_command_detailed_health_spec_proposal.md).

## References

- [Orchestrator](../tech_specs/orchestrator.md)
- [Orchestrator Artifacts Storage](../tech_specs/orchestrator_artifacts_storage.md)
- [Orchestrator Bootstrap](../tech_specs/orchestrator_bootstrap.md)
- [Status command detailed health (proposal)](status_command_detailed_health_spec_proposal.md)
- [ORCHES Requirements](../requirements/orches.md)
