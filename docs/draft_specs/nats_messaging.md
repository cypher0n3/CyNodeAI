# CyNodeAI NATS and JetStream Specification

## 1. Purpose

Define the NATS subject taxonomy, JetStream streams, consumer patterns, and message schemas to support:

- Job dispatch and execution
- Work item and requirements eventing
- Policy approvals
- Artifact and indexing (pgvector ingestion triggers)
- Node presence and capacity
- Live progress streaming

NATS is transport and event backbone - not the authoritative system of record.

## 2. Design Principles

- At-least-once delivery (JetStream) + idempotent consumers
- Small messages; large payloads go to object storage and are referenced by URI + hash
- RBAC enforced via NATS publish/subscribe permissions and message-level scope fields
- Deterministic schemas with explicit versioning
- Stable subject patterns; add new versions via schema versioning, not subject churn

## 3. Naming Conventions

- Prefix all subjects with `cynode.`
- Use lowercase tokens separated by dots
- Put tenant and project in the subject for routing, but do not include secrets or PII

Recommended IDs:

- `tenant_id` - stable string or UUID
- `project_id` - stable string or UUID
- `job_id` - UUID
- `work_item_id` - UUID (story/task/subtask/requirement/etc.)
- `event_id` - UUID (unique per emitted event)

## 4. Subject Taxonomy

Subject names are hierarchical; the following sections list canonical subjects by domain.

### 4.1 Job Subjects

- `cynode.job.requested.<tenant_id>.<project_id>`
- `cynode.job.assigned.<tenant_id>.<project_id>.<node_id>`
- `cynode.job.started.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.progress.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.completed.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.canceled.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.failed.<tenant_id>.<project_id>.<job_id>`

Notes:

- `requested` is produced by orchestrator/dispatcher
- `assigned` is produced by dispatcher (or worker if using pull-claim)
- `started/progress/completed` are produced by the worker node

### 4.2 Work Items and Requirements

- `cynode.workitem.created.<tenant_id>.<project_id>`
- `cynode.workitem.updated.<tenant_id>.<project_id>`
- `cynode.workitem.transitioned.<tenant_id>.<project_id>`
- `cynode.requirement.created.<tenant_id>.<project_id>`
- `cynode.requirement.updated.<tenant_id>.<project_id>`
- `cynode.requirement.verified.<tenant_id>.<project_id>`
- `cynode.acceptance.validated.<tenant_id>.<project_id>`
- `cynode.acceptance.failed.<tenant_id>.<project_id>`

Notes:

- These are immutable events.
  Consumers update read models in Postgres.

### 4.3 Policy Approvals

- `cynode.policy.requested.<tenant_id>.<project_id>`
- `cynode.policy.decided.<tenant_id>.<project_id>`

Notes:

- Used for gated operations (network enablement, destructive actions, sensitive reads).

### 4.4 Artifacts and Indexing

- `cynode.artifact.created.<tenant_id>.<project_id>`
- `cynode.artifact.available.<tenant_id>.<project_id>`
- `cynode.index.requested.<tenant_id>.<project_id>`
- `cynode.index.completed.<tenant_id>.<project_id>`
- `cynode.embedding.requested.<tenant_id>.<project_id>.<namespace>`
- `cynode.embedding.completed.<tenant_id>.<project_id>.<namespace>`

Notes:

- `artifact.available` should reference object storage URIs and hashes
- Indexing/embedding services subscribe and act asynchronously

### 4.5 Node Presence and Capacity

- `cynode.node.heartbeat.<tenant_id>.<node_id>`
- `cynode.node.capacity.<tenant_id>.<node_id>`
- `cynode.node.status.<tenant_id>.<node_id>`

## 5. JetStream Streams

Streams below define durable storage and retention for each domain.

### 5.1 Stream: CYNODE_JOBS

Purpose:

- Durable job dispatch and job lifecycle events needed for recovery

Subjects:

- `cynode.job.requested.*.*`
- `cynode.job.assigned.*.*.*`
- `cynode.job.started.*.*.*`
- `cynode.job.completed.*.*.*`
- `cynode.job.failed.*.*.*`
- `cynode.job.canceled.*.*.*`

Retention:

- WorkQueue retention for `requested/assigned` (or Interest retention if multiple consumers must see all)
- Time-based retention for lifecycle events (days) for postmortems

Ack policy:

- Explicit ack required for all durable consumers

Recommended max age:

- 3 to 14 days (tune per audit and replay needs)

### 5.2 Stream: CYNODE_EVENTS

Purpose:

- Durable domain events for work items, requirements, policy, artifacts

Subjects:

- `cynode.workitem.*.*.*`
- `cynode.requirement.*.*.*`
- `cynode.acceptance.*.*.*`
- `cynode.policy.*.*.*`
- `cynode.artifact.*.*.*`

Retention:

- Limits or time-based (weeks to months), depending on audit requirements

Recommended max age:

- 30 to 180 days (tune per compliance posture)

### 5.3 Stream: CYNODE_TELEMETRY

Purpose:

- High-volume progress and node telemetry for live UX and short replay

Subjects:

- `cynode.job.progress.*.*.*`
- `cynode.node.*.*`

Retention:

- Short time-based retention (minutes to hours)

Recommended max age:

- 1 to 24 hours

## 6. Consumer Patterns

Recommended subscription and processing patterns per consumer type.

### 6.1 Job Execution Consumers (Workers)

Pattern A - Dispatcher assigns jobs to nodes:

- Dispatcher consumes `cynode.job.requested.<tenant>.<project>`
- Dispatcher publishes `cynode.job.assigned.<tenant>.<project>.<node>`
- Worker consumes its node-specific assigned subject

Pattern B - Workers pull-claim jobs:

- Workers form a queue group consuming `cynode.job.requested.<tenant>.<project>`
- First worker to claim persists locally and acks
- Worker publishes `cynode.job.started/...` etc.

Recommendation:

- Start with Pattern A if you need scheduling constraints (GPU locality, model cache locality)
- Use Pattern B for simple homogeneous clusters

### 6.2 Read Model Updaters (Postgres Writers)

- Subscribe to `CYNODE_EVENTS`
- Validate schema, verify scope, then update Postgres (authoritative)
- Must be idempotent by `event_id`

### 6.3 Indexing and Embedding Services

- Subscribe to `cynode.index.requested...` and `cynode.embedding.requested...`
- Emit `completed` events with artifacts and metrics
- Must enforce RBAC scope and sensitivity tags from metadata

### 6.4 Live UX Subscribers

- Subscribe to `cynode.job.progress...` and `cynode.node.capacity...`
- Should tolerate loss or truncation; do not treat telemetry as authoritative

## 7. Message Envelope Specification

All messages MUST use a common envelope with strict schema validation.

### 7.1 Envelope Fields

- `event_id` (UUID) - unique per published message
- `event_type` (string) - stable identifier, e.g., `job.requested`
- `event_version` (string) - semver for payload schema, e.g., `1.0.0`
- `occurred_at` (RFC3339 timestamp)
- `producer` (object)

  - `service` (string)
  - `instance_id` (string)
- `scope` (object)

  - `tenant_id` (string)
  - `project_id` (string, nullable for global)
  - `sensitivity` (public|internal|confidential|restricted)
- `correlation` (object)

  - `session_id` (string, nullable)
  - `work_item_id` (string, nullable)
  - `job_id` (string, nullable)
  - `trace_id` (string, nullable)
- `payload` (object) - schema depends on event_type/event_version

### 7.2 Example Envelope

```json
{
  "event_id": "8c4ef0e9-0b3a-4e8f-a5fc-1c9c7a6c0c3a",
  "event_type": "job.requested",
  "event_version": "1.0.0",
  "occurred_at": "2026-02-22T06:12:34Z",
  "producer": {
    "service": "cynode-dispatcher",
    "instance_id": "dispatch-01"
  },
  "scope": {
    "tenant_id": "t-123",
    "project_id": "p-456",
    "sensitivity": "internal"
  },
  "correlation": {
    "session_id": "s-abc",
    "work_item_id": "wi-789",
    "job_id": "j-111",
    "trace_id": "tr-222"
  },
  "payload": {}
}
```

## 8. Payload Schemas

Canonical payload shapes for core message types (versioned).

### 8.1 `job.requested` `v1.0.0`

Purpose:

- Request execution of a sandbox job (job.json) with constraints and references

Payload:

- `job_id` (UUID)
- `skill_id` (string)
- `skill_version` (string)
- `priority` (int)
- `constraints` (object)

  - `max_runtime_seconds`
  - `network_allowed` (bool)
  - `allowed_commands` (array)
  - `allowed_paths` (array)
  - `max_output_bytes`
- `inputs` (object)

  - `job_spec_uri` (string) - object storage or worker fetchable location
  - `job_spec_sha256` (string)
- `artifacts` (object)

  - `artifact_root_uri` (string) - where worker should upload
- `requirements` (object, optional)

  - `requirement_ids` (array)
  - `acceptance_criteria_ids` (array)

```json
{
  "job_id": "j-111",
  "skill_id": "skill.repo_triage",
  "skill_version": "1.2.0",
  "priority": 50,
  "constraints": {
    "max_runtime_seconds": 900,
    "network_allowed": false,
    "allowed_commands": ["git", "go", "golangci-lint"],
    "allowed_paths": ["/workspace"],
    "max_output_bytes": 1048576
  },
  "inputs": {
    "job_spec_uri": "s3://cynode-jobs/t-123/p-456/j-111/job.json",
    "job_spec_sha256": "abc123..."
  },
  "artifacts": {
    "artifact_root_uri": "s3://cynode-artifacts/t-123/p-456/j-111/"
  },
  "requirements": {
    "requirement_ids": ["REQ-SEC-005"],
    "acceptance_criteria_ids": ["AC-REQ-SEC-005-01"]
  }
}
```

### 8.2 `job.assigned` `v1.0.0`

Payload:

- `job_id`
- `node_id`
- `lease_seconds`
- `lease_expires_at`
- `attempt` (int)

### 8.3 `job.started` `v1.0.0`

Payload:

- `job_id`
- `node_id`
- `sandbox_id`
- `started_at`

### 8.4 `job.progress` `v1.0.0`

Payload (keep small, high frequency):

- `job_id`
- `node_id`
- `step_id` (string, optional)
- `phase` (queued|pulling_image|running|uploading|finalizing)
- `percent` (0-100, optional)
- `message` (string, optional, truncated)
- `log_chunk_ref` (object, optional)

  - `uri`
  - `sha256`
- `ts` (timestamp)

### 8.5 `job.completed` `v1.0.0`

Payload:

- `job_id`
- `node_id`
- `status` (success|failure|timeout|canceled)
- `exit_code` (int, optional)
- `result_uri` (string)
- `result_sha256` (string)
- `artifact_manifest_uri` (string, optional)
- `artifact_manifest_sha256` (string, optional)
- `resource_usage` (object)

  - `cpu_time_ms`
  - `max_rss_bytes`
  - `duration_ms`

### 8.6 `workitem.created` `v1.0.0`

Payload:

- `work_item_id`
- `work_item_type` (epic|feature|story|task|subtask)
- `title`
- `description` (optional)
- `parent_id` (optional)
- `status`
- `priority`
- `estimate_points` (optional)
- `links` (optional)

  - `requirement_ids`
  - `artifact_ids`

### 8.7 `workitem.transitioned` `v1.0.0`

Payload:

- `work_item_id`
- `from_status`
- `to_status`
- `reason` (optional)
- `actor` (user_id or service identity)

### 8.8 `requirement.created` `v1.0.0`

Payload:

- `requirement_id` (e.g., `REQ-SEC-005`)
- `type` (FR|NFR|SR|OR)
- `title`
- `description`
- `status` (draft|approved|implemented|verified)
- `version` (int)
- `story_id` (optional)

### 8.9 `acceptance.validated` `v1.0.0`

Payload:

- `criteria_id`
- `requirement_id`
- `validation_type` (manual|automated|sandbox_test|metric_check)
- `status` (validated|failed)
- `evidence` (object)

  - `artifact_uri`
  - `artifact_sha256`
  - `notes` (optional)

### 8.10 `policy.requested` `v1.0.0`

Payload:

- `policy_request_id`
- `action` (string)
- `risk_level` (low|medium|high)
- `requested_by`
- `context` (object)

  - `job_id` (optional)
  - `work_item_id` (optional)
- `justification` (string)

### 8.11 `policy.decided` `v1.0.0`

Payload:

- `policy_request_id`
- `decision` (approved|denied)
- `decided_by`
- `decided_at`
- `conditions` (optional)

## 9. RBAC and Multi-Tenancy Controls

Access is enforced at both NATS and message level.

### 9.1 NATS-Level Controls

- Prefer separate NATS accounts per tenant, or enforce subject permissions per tenant prefix
- Publish/subscribe permissions must restrict:

  - tenant scope
  - project scope where applicable
  - service identities (worker nodes only subscribe to their assigned patterns if using Pattern A)

### 9.2 Message-Level Controls

- Every message includes `scope.tenant_id` and `scope.project_id`
- Consumers must validate:

  - scope matches their allowed set
  - sensitivity does not exceed role allowance
- Never rely solely on subject routing for authorization

## 10. Idempotency and Deduplication

Requirements:

- Every message includes `event_id` (unique)
- Consumers must store processed `event_id`s (or a rolling window) to avoid double-apply
- Job execution must be idempotent by `job_id`

  - Worker must check local SQLite and/or central Postgres before executing a `job_id` again
  - If duplicate received, re-emit `job.completed` with existing results where possible

## 11. Ordering and Consistency

- Do not assume global ordering across subjects
- For a single job, prefer publishing lifecycle events using the same `job_id` subject token to improve locality
- Consumers must tolerate:

  - duplicates
  - out-of-order progress events
  - missing telemetry

## 12. Payload Size Limits

- Enforce a maximum message size (platform config)
- Put large content in object storage:

  - logs
  - reports
  - job specs
  - artifact manifests
- Messages carry URIs + hashes, not the bytes

## 13. Operational Defaults

Suggested initial defaults:

- CYNODE_JOBS max age: 7 days
- CYNODE_EVENTS max age: 90 days
- CYNODE_TELEMETRY max age: 6 hours
- job.progress publish rate: 1-2 Hz per active job (tunable)
- heartbeat publish rate: every 5-15 seconds per node (tunable)

## 14. Implementation Checklist

- Define JSON schemas for each `event_type` + `event_version`
- Implement a shared envelope validation library
- Implement producer-side signing or HMAC (optional, recommended for untrusted networks)
- Implement consumer idempotency store:

  - Postgres table `processed_events(event_id, processed_at)`
  - Worker SQLite table `processed_events(event_id, processed_at)`
- Implement object storage conventions for job specs, results, and artifacts
- Implement subject permissions per service identity and tenant

## 15. MVP Scope

Minimum viable NATS implementation:

- Subjects:

  - `job.requested`, `job.started`, `job.progress`, `job.completed`
  - `node.heartbeat`, `node.capacity`
  - `artifact.available`
- Streams:

  - CYNODE_JOBS
  - CYNODE_TELEMETRY
- Idempotency:

  - job_id-based worker dedupe
  - event_id-based consumer dedupe

Everything else can be added incrementally once the job pipeline is stable.
