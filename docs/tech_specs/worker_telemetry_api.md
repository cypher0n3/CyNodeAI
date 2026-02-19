# Worker Telemetry API (Node) - Contract, Storage, and Retention

- [Document Overview](#document-overview)
- [Scope](#scope)
- [Definitions](#definitions)
- [Versioning](#versioning)
- [Authentication](#authentication)
- [Telemetry Storage (SQLite)](#telemetry-storage-sqlite)
- [Retention and Vacuuming](#retention-and-vacuuming)
- [API Error Handling](#api-error-handling)
- [Worker Telemetry API Surface (v1)](#worker-telemetry-api-surface-v1)
  - [Get Node Info](#get-node-info)
  - [Get Node Stats (Snapshot)](#get-node-stats-snapshot)
  - [List Containers (Inventory)](#list-containers-inventory)
  - [Get Container (Inventory)](#get-container-inventory)
  - [Query Logs](#query-logs)
- [Orchestrator Consumption Requirements](#orchestrator-consumption-requirements)

## Document Overview

- Spec ID: `CYNAI.WORKER.Doc.WorkerTelemetryApi` <a id="spec-cynai-worker-doc-workertelemetryapi"></a>

This document defines the **Worker Telemetry API** contract exposed by a node.
The orchestrator uses this API to pull node-local operational telemetry for debugging and operations.

This spec is intentionally prescriptive:

- It defines exact endpoint paths, request/response payload shapes, and required limits.
- It requires a local SQLite database for telemetry indexing and querying.
- It defines retention behavior and vacuuming requirements so disk usage remains bounded.

Related specs

- Worker job execution API: [`docs/tech_specs/worker_api.md`](worker_api.md)
- Node responsibilities: [`docs/tech_specs/node.md`](node.md)
- Go API implementation standards: [`docs/tech_specs/go_rest_api_standards.md`](go_rest_api_standards.md)
- Node startup YAML keys (including `storage.state_dir`): [`docs/tech_specs/node.md`](node.md#node-startup-yaml)

## Scope

The Worker Telemetry API is orchestrator-facing.
It is not intended for direct use by end-user clients.

The telemetry surface includes:

- node identity/build info and last reported capabilities
- node resource stats snapshot (CPU, memory, disk) at request time
- container inventory for node-managed containers (managed services and sandboxes)
- bounded log retrieval for:
  - node services (Node Manager, Worker API)
  - node-managed containers (including sandbox containers and long-lived managed containers)

Out of scope for this spec

- Real-time log streaming (websockets, SSE).
  This spec supports bounded polling with pagination tokens.
- Full fidelity replication of node logs into the orchestrator database.

## Definitions

- **Worker Telemetry API**: HTTP API hosted by the node and served by the Worker API HTTP server on the same listener.
- **Telemetry store**: node-local SQLite database that indexes operational events and logs for query.
- **Managed container**: a long-lived container started and supervised by the Node Manager (for example, Ollama).
- **Sandbox container**: a per-job or per-task container used for isolated execution.

## Versioning

- All Worker Telemetry API endpoints MUST be served under a stable version prefix: `/v1/`.
- All JSON request and response bodies MUST include a top-level `version` integer.
  - For this spec, `version` MUST be `1`.

## Authentication

- Spec ID: `CYNAI.WORKER.TelemetryApiAuth` <a id="spec-cynai-worker-telemetryauth"></a>

Traces To:

- [REQ-WORKER-0200](../requirements/worker.md#req-worker-0200)
- [REQ-WORKER-0201](../requirements/worker.md#req-worker-0201)

Requirements

- The Worker Telemetry API MUST authenticate all requests.
- Authentication MUST use the same orchestrator-to-node bearer token mechanism as the Worker API.
  - The orchestrator bearer token is delivered via the node configuration payload.
  - See [`docs/tech_specs/node_payloads.md`](node_payloads.md) `node_configuration_payload_v1.worker_api.orchestrator_bearer_token`.
- The node MUST treat bearer tokens as secrets and MUST NOT log them.

## Telemetry Storage (SQLite)

- Spec ID: `CYNAI.WORKER.TelemetryStorageSqlite` <a id="spec-cynai-worker-telemetrystorage-sqlite"></a>

Traces To:

- [REQ-WORKER-0210](../requirements/worker.md#req-worker-0210)
- [REQ-WORKER-0211](../requirements/worker.md#req-worker-0211)
- [REQ-WORKER-0212](../requirements/worker.md#req-worker-0212)

The node MUST maintain a node-local SQLite database used to index and query telemetry.

### Database Location

- The database path MUST be derived from the node startup YAML `storage.state_dir`.
- If `storage.state_dir` is not set, the implementation MUST default to `/var/lib/cynode/state`.
- The telemetry database MUST be located at:
  - `${storage.state_dir}/telemetry/telemetry.db`
- The node MUST ensure the directory `${storage.state_dir}/telemetry` exists and is writable by the node services.

### SQLite Configuration Requirements

- SQLite MUST run in WAL mode.
- SQLite MUST configure a busy timeout of at least 5 seconds to reduce write contention errors.
- The node MUST perform schema migrations on startup using an explicit `schema_version` table.
- The node MUST support safe concurrent access from multiple goroutines (use a single shared connection pool).

### Schema (V1)

The node MUST implement the schema below exactly.
Fields may be added only by creating a new schema version and migration.

```sql
CREATE TABLE IF NOT EXISTS schema_version (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  version INTEGER NOT NULL,
  applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS node_boot (
  boot_id TEXT PRIMARY KEY,
  booted_at TEXT NOT NULL,
  node_slug TEXT NOT NULL,
  build_version TEXT NOT NULL,
  git_sha TEXT NOT NULL,
  platform_os TEXT NOT NULL,
  platform_arch TEXT NOT NULL,
  kernel_version TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS container_inventory (
  container_id TEXT PRIMARY KEY,
  container_name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('managed', 'sandbox')),
  runtime TEXT NOT NULL,
  image_ref TEXT NOT NULL,
  created_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  status TEXT NOT NULL,
  exit_code INTEGER,
  task_id TEXT,
  job_id TEXT,
  labels_json TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_container_inventory_kind_status
  ON container_inventory(kind, status);

CREATE INDEX IF NOT EXISTS idx_container_inventory_task_job
  ON container_inventory(task_id, job_id);

CREATE TABLE IF NOT EXISTS container_event (
  event_id TEXT PRIMARY KEY,
  occurred_at TEXT NOT NULL,
  container_id TEXT NOT NULL,
  action TEXT NOT NULL,
  status TEXT NOT NULL,
  exit_code INTEGER,
  task_id TEXT,
  job_id TEXT,
  details_json TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_container_event_container_time
  ON container_event(container_id, occurred_at);

CREATE INDEX IF NOT EXISTS idx_container_event_task_job
  ON container_event(task_id, job_id);

CREATE TABLE IF NOT EXISTS log_event (
  log_id TEXT PRIMARY KEY,
  occurred_at TEXT NOT NULL,
  source_kind TEXT NOT NULL CHECK (source_kind IN ('service', 'container')),
  source_name TEXT NOT NULL,
  container_id TEXT,
  stream TEXT CHECK (stream IN ('stdout', 'stderr')),
  level TEXT,
  message TEXT NOT NULL,
  fields_json TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_log_event_source_time
  ON log_event(source_kind, source_name, occurred_at);

CREATE INDEX IF NOT EXISTS idx_log_event_container_time
  ON log_event(container_id, occurred_at);
```

Field semantics

- `*_at` timestamps MUST be RFC 3339 strings in UTC.
- `labels_json`, `details_json`, and `fields_json` MUST be valid JSON objects encoded as UTF-8 text.
- `task_id` and `job_id` MUST be UUID strings when present.

Event sourcing requirements

- The node MUST record:
  - container inventory updates at least when containers are created, started, stopped, and removed
  - container events for the same lifecycle transitions
  - service log events for node services (Node Manager, Worker API)
  - container log events for node-managed containers and sandbox containers
- When a container is associated with a `task_id` and `job_id`, those fields MUST be populated in both `container_inventory`
  and `container_event` rows.

## Retention and Vacuuming

- Spec ID: `CYNAI.WORKER.TelemetryRetention` <a id="spec-cynai-worker-telemetryretention"></a>

Traces To:

- [REQ-WORKER-0220](../requirements/worker.md#req-worker-0220)
- [REQ-WORKER-0221](../requirements/worker.md#req-worker-0221)
- [REQ-WORKER-0222](../requirements/worker.md#req-worker-0222)

The node MUST bound telemetry growth.

### Retention Defaults (Required)

Unless the node startup YAML configures stricter limits, the node MUST enforce all of the following defaults:

- Keep at most 7 days of `log_event` rows.
- Keep at most 30 days of `container_event` rows.
- Keep at most 30 days of `container_inventory` history by removing inventory rows that have not been `last_seen_at`
  updated in 30 days and are not currently running.

### Retention Implementation Requirements

- The node MUST run retention enforcement on:
  - startup
  - at least once per hour while running
- The node MUST perform SQLite `VACUUM` at least once per day.
- Retention and vacuuming MUST be safe under concurrent read load from the API.

## API Error Handling

Worker Telemetry API error responses MUST follow the Go REST API error standards:

- Prefer RFC 9457 Problem Details JSON (`application/problem+json`).
- Do not leak secrets in errors.
- Use stable error `type` values.

See [`docs/tech_specs/go_rest_api_standards.md`](go_rest_api_standards.md#error-format-and-status-codes).

## Worker Telemetry API Surface (V1)

- Spec ID: `CYNAI.WORKER.TelemetryApiSurfaceV1` <a id="spec-cynai-worker-telemetrysurface-v1"></a>

Traces To:

- [REQ-WORKER-0230](../requirements/worker.md#req-worker-0230)
- [REQ-WORKER-0231](../requirements/worker.md#req-worker-0231)
- [REQ-WORKER-0232](../requirements/worker.md#req-worker-0232)
- [REQ-WORKER-0233](../requirements/worker.md#req-worker-0233)
- [REQ-WORKER-0234](../requirements/worker.md#req-worker-0234)

All endpoints below MUST:

- require bearer authentication
- live under the `/v1/` prefix
- return `version: 1` in JSON responses
- enforce a maximum response body size of 2 MiB for all endpoints in this document, except [Query Logs](#query-logs)
  which has a stricter limit

### Get Node Info

Endpoint

- `GET /v1/worker/telemetry/node:info`

Response fields

- `version` (int, required): must be 1
- `node_slug` (string, required)
- `build` (object, required)
  - `build_version` (string, required)
  - `git_sha` (string, required)
- `platform` (object, required)
  - `os` (string, required)
  - `arch` (string, required)
  - `kernel_version` (string, required)
- `last_capability_report` (object, optional)
  - MUST match the `node_capability_report_v1` schema from [`docs/tech_specs/node_payloads.md`](node_payloads.md).

### Get Node Stats (Snapshot)

Endpoint

- `GET /v1/worker/telemetry/node:stats`

Response fields

- `version` (int, required): must be 1
- `captured_at` (string, required): RFC 3339 UTC
- `cpu` (object, required)
  - `cores` (int, required)
  - `load1` (number, required)
  - `load5` (number, required)
  - `load15` (number, required)
- `memory` (object, required)
  - `total_mb` (int, required)
  - `used_mb` (int, required)
  - `free_mb` (int, required)
- `disk` (object, required)
  - `state_dir_free_mb` (int, required)
  - `state_dir_total_mb` (int, required)
- `container_runtime` (object, required)
  - `runtime` (string, required)
  - `version` (string, required)

### List Containers (Inventory)

Endpoint

- `GET /v1/worker/telemetry/containers`

Query parameters (all optional)

- `kind` (string): `managed` or `sandbox`
- `status` (string): implementation MUST accept any container runtime status string, but SHOULD normalize to:
  `created`, `running`, `exited`, `paused`, `unknown`
- `task_id` (uuid string)
- `job_id` (uuid string)
- `limit` (int): default 100, max 1000
- `page_token` (string): opaque token from a previous response

Response fields

- `version` (int, required): must be 1
- `containers` (array, required)
  - each element MUST match the response shape from [Get Container (Inventory)](#get-container-inventory) minus `version`
- `next_page_token` (string, optional)

### Get Container (Inventory)

Endpoint

- `GET /v1/worker/telemetry/containers/{container_id}`

Response fields

- `version` (int, required): must be 1
- `container` (object, required)
  - `container_id` (string, required)
  - `container_name` (string, required)
  - `kind` (string, required): `managed` or `sandbox`
  - `runtime` (string, required): `docker` or `podman`
  - `image_ref` (string, required)
  - `created_at` (string, required): RFC 3339 UTC
  - `last_seen_at` (string, required): RFC 3339 UTC
  - `status` (string, required)
  - `exit_code` (int, optional)
  - `task_id` (uuid string, optional)
  - `job_id` (uuid string, optional)
  - `labels` (object string->string, required)

### Query Logs

- Spec ID: `CYNAI.WORKER.TelemetryLogQueryV1` <a id="spec-cynai-worker-telemetrylogquery-v1"></a>

Traces To:

- [REQ-WORKER-0240](../requirements/worker.md#req-worker-0240)
- [REQ-WORKER-0241](../requirements/worker.md#req-worker-0241)
- [REQ-WORKER-0242](../requirements/worker.md#req-worker-0242)
- [REQ-WORKER-0243](../requirements/worker.md#req-worker-0243)

Endpoint

- `GET /v1/worker/telemetry/logs`

Query parameters (required filters)

The implementation MUST require at least one of:

- `source_kind=service&source_name=<name>`
- `source_kind=container&container_id=<container_id>`

Query parameters (optional)

- `source_kind` (string): `service` or `container`
- `source_name` (string): when `source_kind=service`, allowed values MUST include `node_manager` and `worker_api`
- `container_id` (string): when `source_kind=container`
- `stream` (string): `stdout` or `stderr` (only applicable to `source_kind=container`)
- `since` (string): RFC 3339 UTC (inclusive)
- `until` (string): RFC 3339 UTC (exclusive)
- `limit` (int): default 1000, max 5000
- `page_token` (string): opaque token from a previous response

Response fields

- `version` (int, required): must be 1
- `events` (array, required)
  - each event:
    - `occurred_at` (string, required): RFC 3339 UTC
    - `source_kind` (string, required)
    - `source_name` (string, required)
    - `container_id` (string, optional)
    - `stream` (string, optional)
    - `level` (string, optional)
    - `message` (string, required)
    - `fields` (object, required)
- `truncated` (object, required)
  - `limited_by` (string, required): one of `count`, `bytes`, `none`
  - `max_bytes` (int, required)
- `next_page_token` (string, optional)

Response size limits (required)

- The server MUST enforce a maximum response body size of 1 MiB for this endpoint.
- The server MUST set `truncated.limited_by` to:
  - `bytes` when the response body limit would be exceeded
  - `count` when `limit` is reached and additional matching records exist
  - `none` when all matching records are returned in the response
- The server MUST set `truncated.max_bytes` to 1048576.

## Orchestrator Consumption Requirements

- Spec ID: `CYNAI.ORCHES.NodeTelemetryPull` <a id="spec-cynai-orches-nodetelemetrypull"></a>

Traces To:

- [REQ-ORCHES-0140](../requirements/orches.md#req-orches-0140)
- [REQ-ORCHES-0141](../requirements/orches.md#req-orches-0141)
- [REQ-ORCHES-0142](../requirements/orches.md#req-orches-0142)

The orchestrator MUST consume the Worker Telemetry API using a best-effort policy:

- The orchestrator MUST apply per-request timeouts.
- The orchestrator MUST tolerate node unavailability and partial responses.
- The orchestrator MUST treat telemetry responses as non-authoritative operational data.

The orchestrator MUST use the same bearer token authentication as all other orchestrator-to-node calls and MUST NOT
expose telemetry endpoints directly to untrusted clients.
