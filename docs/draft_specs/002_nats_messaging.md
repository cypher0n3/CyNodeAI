# CyNodeAI NATS and JetStream Specification

- [1 Purpose](#1-purpose)
- [2 Design Principles](#2-design-principles)
- [3 Naming Conventions](#3-naming-conventions)
- [4 Subject Taxonomy](#4-subject-taxonomy)
  - [4.6 Session Activity](#46-session-activity)
  - [4.7 Node Configuration Notifications](#47-node-configuration-notifications)
- [5 JetStream Streams](#5-jetstream-streams)
  - [5.4 Stream: `CYNODE_SESSION`](#54-stream-cynode_session)
- [6 Consumer Patterns](#6-consumer-patterns)
  - [6.5 Session Activity Consumers](#65-session-activity-consumers-orchestrator)
  - [6.6 Config Notification Consumers](#66-config-notification-consumers-node-manager)
- [7 Message Envelope Specification](#7-message-envelope-specification)
- [8 Payload Schemas](#8-payload-schemas)
- [9 RBAC and Multi-Tenancy Controls](#9-rbac-and-multi-tenancy-controls)
- [10 Idempotency and Deduplication](#10-idempotency-and-deduplication)
- [11 Ordering and Consistency](#11-ordering-and-consistency)
- [12 Payload Size Limits](#12-payload-size-limits)
- [13 Operational Defaults](#13-operational-defaults)
- [14 Implementation Checklist](#14-implementation-checklist)
- [15 MVP Scope](#15-mvp-scope)
- [16 References](#16-references)

## 1 Purpose

Define the NATS subject taxonomy, JetStream streams, consumer patterns, and message schemas to support:

- Job dispatch and execution
- Work item and requirements eventing
- Policy approvals
- Artifact and indexing (pgvector ingestion triggers)
- Node presence and capacity
- Live progress streaming
- Session activity and idle lifecycle for per-session resource provisioning

NATS is the unified transport and event backbone, not the authoritative system of record.
Postgres remains the authoritative store; NATS provides real-time signaling that reduces polling latency across component boundaries.

Session activity tracking for per-session PMA idle lifecycle is the first production use-case (Phase 1); the architecture will be incrementally refactored to leverage NATS across the board as the unified transport layer.

## 2 Design Principles

- At-least-once delivery (JetStream) + idempotent consumers
- Small messages; large payloads go to object storage and are referenced by URI + hash
- RBAC enforced via NATS publish/subscribe permissions and message-level scope fields
- Deterministic schemas with explicit versioning
- Stable subject patterns; add new versions via schema versioning, not subject churn

## 3 Naming Conventions

- Prefix all subjects with `cynode.`
- Use lowercase tokens separated by dots
- Put tenant and project in the subject for routing, but do not include secrets or PII

### 3.1 Recommended Ids

- `tenant_id` - stable string or UUID
- `project_id` - stable string or UUID
- `job_id` - UUID
- `work_item_id` - UUID (story/task/subtask/requirement/etc.)
- `event_id` - UUID (unique per emitted event)

## 4 Subject Taxonomy

Subject names are hierarchical; the following sections list canonical subjects by domain.

### 4.1 Job Subjects

- `cynode.job.requested.<tenant_id>.<project_id>`
- `cynode.job.assigned.<tenant_id>.<project_id>.<node_id>`
- `cynode.job.started.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.progress.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.completed.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.canceled.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.failed.<tenant_id>.<project_id>.<job_id>`

#### 4.1.1 Job Subject Notes

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

#### 4.2.1 Work Item Notes

- These are immutable events.
  Consumers update read models in Postgres.

### 4.3 Policy Approvals

- `cynode.policy.requested.<tenant_id>.<project_id>`
- `cynode.policy.decided.<tenant_id>.<project_id>`

#### 4.3.1 Policy Notes

- Used for gated operations (network enablement, destructive actions, sensitive reads).

### 4.4 Artifacts and Indexing

- `cynode.artifact.created.<tenant_id>.<project_id>`
- `cynode.artifact.available.<tenant_id>.<project_id>`
- `cynode.index.requested.<tenant_id>.<project_id>`
- `cynode.index.completed.<tenant_id>.<project_id>`
- `cynode.embedding.requested.<tenant_id>.<project_id>.<namespace>`
- `cynode.embedding.completed.<tenant_id>.<project_id>.<namespace>`

#### 4.4.1 Artifact Notes

- `artifact.available` should reference object storage URIs and hashes
- Indexing/embedding services subscribe and act asynchronously

### 4.5 Node Presence and Capacity

- `cynode.node.heartbeat.<tenant_id>.<node_id>`
- `cynode.node.capacity.<tenant_id>.<node_id>`
- `cynode.node.status.<tenant_id>.<node_id>`

### 4.6 Session Activity

- `cynode.session.activity.<tenant_id>.<session_id>`
- `cynode.session.attached.<tenant_id>.<session_id>`
- `cynode.session.detached.<tenant_id>.<session_id>`

#### 4.6.1 Session Activity Notes

- `activity` is published periodically by the user-gateway at `T_heartbeat` cadence for each session that has had authenticated API interaction since the last publish.
  Each message resets the idle clock for that session binding.
- `attached` is published once when a client establishes a session (login or reconnect after idle).
- `detached` is published when the client cleanly disconnects (logout or explicit close).
  If the client crashes, the absence of `activity` messages within the idle window serves as an implicit detach.
- The user-gateway derives session liveness from normal authenticated API traffic (chat, MCP tool calls, token refresh, streaming connections) and publishes to NATS server-side.
  Clients do not publish to NATS directly; NATS credentials are not exposed to external clients.
- Session activity is the first production use-case for NATS in this system; future phases expand NATS adoption across the architecture.
- See [003_pma_client_connection_session_activity_spec_proposal.md](003_pma_client_connection_session_activity_spec_proposal.md) for the full session activity model.

### 4.7 Node Configuration Notifications

- `cynode.node.config_changed.<tenant_id>.<node_id>`

#### 4.7.1 Config Notification Notes

- Published by the orchestrator control-plane when `managed_services`, policy, or other node configuration changes.
- The node-manager subscribes and immediately fetches the updated configuration, replacing or supplementing the poll interval.
- This reduces "config bump to container action" latency from poll-interval to near-zero.

## 5 JetStream Streams

Streams below define durable storage and retention for each domain.

### 5.1 Stream: `CYNODE_JOBS`

This stream stores job dispatch and lifecycle events.

#### 5.1.1 Stream Purpose (`CYNODE_JOBS`)

- Durable job dispatch and job lifecycle events needed for recovery

#### 5.1.2 Stream Subjects (`CYNODE_JOBS`)

- `cynode.job.requested.*.*`
- `cynode.job.assigned.*.*.*`
- `cynode.job.started.*.*.*`
- `cynode.job.completed.*.*.*`
- `cynode.job.failed.*.*.*`
- `cynode.job.canceled.*.*.*`

#### 5.1.3 Stream Retention (`CYNODE_JOBS`)

- WorkQueue retention for `requested/assigned` (or Interest retention if multiple consumers must see all)
- Time-based retention for lifecycle events (days) for postmortems

Ack policy:

- Explicit ack required for all durable consumers

Recommended max age:

- 3 to 14 days (tune per audit and replay needs)

### 5.2 Stream: `CYNODE_EVENTS`

This stream stores work item and requirement events.

#### 5.2.1 Stream Purpose (`CYNODE_EVENTS`)

- Durable domain events for work items, requirements, policy, artifacts

#### 5.2.2 Stream Subjects (`CYNODE_EVENTS`)

- `cynode.workitem.*.*.*`
- `cynode.requirement.*.*.*`
- `cynode.acceptance.*.*.*`
- `cynode.policy.*.*.*`
- `cynode.artifact.*.*.*`

#### 5.2.3 Stream Retention (`CYNODE_EVENTS`)

- Limits or time-based (weeks to months), depending on audit requirements

Recommended max age:

- 30 to 180 days (tune per compliance posture)

### 5.3 Stream: `CYNODE_TELEMETRY`

This stream stores node heartbeats and capacity data.

#### 5.3.1 Stream Purpose (`CYNODE_TELEMETRY`)

- High-volume progress and node telemetry for live UX and short replay

#### 5.3.2 Stream Subjects (`CYNODE_TELEMETRY`)

- `cynode.job.progress.*.*.*`
- `cynode.node.*.*`

#### 5.3.3 Stream Retention (`CYNODE_TELEMETRY`)

- Short time-based retention (minutes to hours)

Recommended max age:

- 1 to 24 hours

### 5.4 Stream: `CYNODE_SESSION`

This stream stores session activity and attachment lifecycle events.

#### 5.4.1 Stream Purpose (`CYNODE_SESSION`)

- Short-lived session presence data for PMA idle lifecycle and re-activation replay on orchestrator restart

#### 5.4.2 Stream Subjects (`CYNODE_SESSION`)

- `cynode.session.activity.*.*`
- `cynode.session.attached.*.*`
- `cynode.session.detached.*.*`

#### 5.4.3 Stream Retention (`CYNODE_SESSION`)

- Time-based retention (hours); only needed for replay if the orchestrator restarts while sessions are active

Recommended max age:

- 1 to 6 hours

## 6 Consumer Patterns

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

#### 6.1.1 Pattern Recommendation

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

### 6.5 Session Activity Consumers (Orchestrator)

- Orchestrator subscribes to `cynode.session.activity.<tenant>.<session_id>` and updates `last_activity_at` on the corresponding session binding in Postgres.
- On `session.attached`, orchestrator ensures the session binding is `active` and the PMA managed service is in desired state (re-activation path).
- On `session.detached`, orchestrator may begin an accelerated idle countdown or mark the binding for teardown immediately per policy.
- The PMA binding scanner goroutine remains as a safety net for NATS downtime, missed messages, and clock skew.

### 6.6 Config Notification Consumers (Node-Manager)

- Node-manager subscribes to `cynode.node.config_changed.<tenant>.<node_id>` for its own node.
- On receipt, node-manager fetches updated configuration from the control-plane and reconciles managed services.
- The existing poll interval remains as a fallback for missed NATS messages.

## 7 Message Envelope Specification

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

## 8 Payload Schemas

Canonical payload shapes for core message types (versioned).

### 8.1 `job.requested` `v1.0.0`

Request execution of a sandbox job; payload and semantics below.

#### 8.1.1 Purpose of `job.requested`

- Request execution of a sandbox job (job.json) with constraints and references

#### 8.1.2 Payload for `job.requested`

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

Assignment notification from dispatcher to worker; payload below.

#### 8.2.1 Payload for `job.assigned`

- `job_id`
- `node_id`
- `lease_seconds`
- `lease_expires_at`
- `attempt` (int)

### 8.3 `job.started` `v1.0.0`

Worker has started the job; payload below.

#### 8.3.1 Payload for `job.started`

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

Job finished; payload below.

#### 8.5.1 Payload for `job.completed`

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

Work item created in backlog; payload below.

#### 8.6.1 Payload for `workitem.created`

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

Work item status change; payload below.

#### 8.7.1 Payload for `workitem.transitioned`

- `work_item_id`
- `from_status`
- `to_status`
- `reason` (optional)
- `actor` (user_id or service identity)

### 8.8 `requirement.created` `v1.0.0`

Requirement created or updated; payload below.

#### 8.8.1 Payload for `requirement.created`

- `requirement_id` (e.g., `REQ-SEC-005`)
- `type` (FR|NFR|SR|OR)
- `title`
- `description`
- `status` (draft|approved|implemented|verified)
- `version` (int)
- `story_id` (optional)

### 8.9 `acceptance.validated` `v1.0.0`

Acceptance criterion validated; payload below.

#### 8.9.1 Payload for `acceptance.validated`

- `criteria_id`
- `requirement_id`
- `validation_type` (manual|automated|sandbox_test|metric_check)
- `status` (validated|failed)
- `evidence` (object)

  - `artifact_uri`
  - `artifact_sha256`
  - `notes` (optional)

### 8.10 `policy.requested` `v1.0.0`

Policy decision requested; payload below.

#### 8.10.1 Payload for `policy.requested`

- `policy_request_id`
- `action` (string)
- `risk_level` (low|medium|high)
- `requested_by`
- `context` (object)

  - `job_id` (optional)
  - `work_item_id` (optional)
- `justification` (string)

### 8.11 `policy.decided` `v1.0.0`

Policy decision recorded; payload below.

#### 8.11.1 Payload for `policy.decided`

- `policy_request_id`
- `decision` (approved|denied)
- `decided_by`
- `decided_at`
- `conditions` (optional)

### 8.12 `session.activity` `v1.0.0`

Periodic heartbeat indicating an active client session; payload below.

#### 8.12.1 Payload for `session.activity`

- `session_id` (UUID)
- `user_id` (UUID)
- `binding_key` (string, opaque session binding key)
- `client_type` (string: `cynork`|`web_console`|`other`)
- `ts` (RFC3339 timestamp)

### 8.13 `session.attached` `v1.0.0`

Client has established a session activity channel; payload below.

#### 8.13.1 Payload for `session.attached`

- `session_id` (UUID)
- `user_id` (UUID)
- `binding_key` (string)
- `client_type` (string: `cynork`|`web_console`|`other`)
- `ts` (RFC3339 timestamp)

### 8.14 `session.detached` `v1.0.0`

Client has cleanly disconnected from the session activity channel; payload below.

#### 8.14.1 Payload for `session.detached`

- `session_id` (UUID)
- `user_id` (UUID)
- `binding_key` (string)
- `reason` (string: `logout`|`client_close`|`timeout`)
- `ts` (RFC3339 timestamp)

### 8.15 `node.config_changed` `v1.0.0`

Notification that the node configuration has been updated; payload below.

#### 8.15.1 Payload for `node.config_changed`

- `node_id` (string)
- `config_version` (string, ULID or equivalent monotonic version)
- `changed_sections` (array of strings, optional: `managed_services`|`policy`|`inference_backend`|`other`)
- `ts` (RFC3339 timestamp)

## 9 RBAC and Multi-Tenancy Controls

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

## 10 Idempotency and Deduplication

Consumers must handle duplicate delivery and apply updates idempotently.

### 10.1 Idempotency Requirements

- Every message includes `event_id` (unique)
- Consumers must store processed `event_id`s (or a rolling window) to avoid double-apply
- Job execution must be idempotent by `job_id`

  - Worker must check local SQLite and/or central Postgres before executing a `job_id` again
  - If duplicate received, re-emit `job.completed` with existing results where possible

## 11 Ordering and Consistency

- Do not assume global ordering across subjects.
- For a single job, prefer publishing lifecycle events using the same `job_id` subject token to improve locality.
- Consumers must tolerate:

  - duplicates
  - out-of-order progress events
  - missing telemetry

## 12 Payload Size Limits

- Enforce a maximum message size (platform config)
- Put large content in object storage:

  - logs
  - reports
  - job specs
  - artifact manifests
- Messages carry URIs + hashes, not the bytes

## 13 Operational Defaults

Suggested initial defaults:

- CYNODE_JOBS max age: 7 days
- CYNODE_EVENTS max age: 90 days
- CYNODE_TELEMETRY max age: 6 hours
- CYNODE_SESSION max age: 6 hours
- job.progress publish rate: 1-2 Hz per active job (tunable)
- heartbeat publish rate: every 5-15 seconds per node (tunable)
- session.activity publish rate: once per `T_heartbeat` (2-3 minutes) per active session

## 14 Implementation Checklist

- Define JSON schemas for each `event_type` + `event_version`
- Implement a shared envelope validation library
- Implement producer-side signing or HMAC (optional, recommended for untrusted networks)
- Implement consumer idempotency store:

  - Postgres table `processed_events(event_id, processed_at)`
  - Worker SQLite table `processed_events(event_id, processed_at)`
- Implement object storage conventions for job specs, results, and artifacts
- Implement subject permissions per service identity and tenant

## 15 MVP Scope

Minimum viable NATS implementation, in recommended phase order.

### 15.1 Phase 1: Session Activity and Config Notifications

Deploy NATS (single-node, JetStream enabled) as part of the dev stack.
This is the first production use-case for NATS, directly supporting per-session PMA idle lifecycle.

- Subjects:

  - `session.activity`, `session.attached`, `session.detached`
  - `node.config_changed`
- Streams:

  - CYNODE_SESSION
- Consumers:

  - Orchestrator session-activity consumer (updates `last_activity_at`)
  - Node-manager config-notification consumer (fetches updated config on change)
- Gateway integration:

  - User-gateway derives session liveness from authenticated API traffic and publishes to NATS server-side
  - No dedicated client-facing heartbeat endpoint; clients interact via existing authenticated HTTP/SSE endpoints

### 15.2 Phase 2: Node Presence

Migrate node heartbeats and capacity reporting from HTTP polling to NATS pub/sub.

- Subjects:

  - `node.heartbeat`, `node.capacity`
- Streams:

  - CYNODE_TELEMETRY (already includes `cynode.node.*`)

### 15.3 Phase 3: Job Pipeline

Full job dispatch and lifecycle eventing.

- Subjects:

  - `job.requested`, `job.started`, `job.progress`, `job.completed`
  - `artifact.available`
- Streams:

  - CYNODE_JOBS
- Idempotency:

  - job_id-based worker dedupe
  - event_id-based consumer dedupe

Everything else (work items, requirements, policy, indexing) can be added incrementally once the job pipeline is stable.

## 16 References

- [003_pma_client_connection_session_activity_spec_proposal.md](003_pma_client_connection_session_activity_spec_proposal.md) (session activity model and idle lifecycle)
- [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md) (PMA startup and session binding)
- [worker_node.md](../tech_specs/worker_node.md) (managed services, reconciliation, dynamic configuration)
- [worker_node_payloads.md](../tech_specs/worker_node_payloads.md) (`managed_services`, `policy.updates.allow_service_restart`)
