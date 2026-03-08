# Worker Node Telemetry Database - Actual Usage (As of 2026-03-07)

- [Spec vs Implementation](#spec-vs-implementation)
- [What Uses the DB Today](#what-uses-the-db-today)
- [Summary](#summary)
- [Conclusion](#conclusion)

## Spec vs Implementation

- **Spec:** [worker_telemetry_api.md](../tech_specs/worker_telemetry_api.md) defines a node-local SQLite DB at `${state_dir}/telemetry/telemetry.db` with tables: `schema_version`, `node_boot`, `container_inventory`, `container_event`, `log_event`.
- It is intended to index and query operational events and logs and to back the Worker Telemetry API endpoints.
- **Location:** `worker_node/internal/telemetry/store.go` opens the DB, runs migrations, and exposes `Store` with read/write methods.
- The only **production** write path is retention/vacuum (deletes and `VACUUM`).
- There is **no production code** that inserts into `node_boot`, `container_inventory`, `container_event`, or `log_event`.

## What Uses the DB Today

Only worker-api reads from the store; no production code writes to it.

### Reads (Worker API)

- **GET /v1/worker/telemetry/containers:** `handleListContainers(store)` => `store.ListContainers()` => reads `container_inventory`.
- In production this is always empty (no writer).
- **GET /v1/worker/telemetry/containers/{id}:** `handleGetContainer(store)` => `store.GetContainer()` => reads `container_inventory`.
- Same: empty in production.
- **GET /v1/worker/telemetry/logs:** `handleQueryLogs(store)` => `store.QueryLogs()` => reads `log_event`.
- Empty in production.

### Not Backed by the DB

- **GET /v1/worker/telemetry/node:info:** `handleNodeInfo` builds JSON from env (`NODE_SLUG`, etc.) and static placeholders.
- Does **not** read from the telemetry store or `node_boot`.
- **GET /v1/worker/telemetry/node:stats:** `handleNodeStats` builds JSON from env and static/placeholder values (e.g. zeros for CPU/memory/disk).
- Does **not** read from the store.

### Writes (Production)

- **None.**
- No component (worker-api, node-manager, or elsewhere) inserts into the telemetry tables in production.
- The only insert in the telemetry package is `InsertTestContainer` in `store.go`, which is for tests only.
- **Retention and vacuum:** worker-api runs `EnforceRetention` and `Vacuum` on the store at startup and periodically (`doRetentionAndVacuumOnce`, `runRetentionAndVacuum`).
- So the DB file exists, schema is applied, and retention/vacuum run, but on an effectively empty DB.

### Orchestrator Consumption

- Control-plane `runTelemetryPullLoop` only calls `PullNodeInfo` and `PullNodeStats` (HTTP GETs to node:info and node:stats).
- It does **not** call the worker's `/v1/worker/telemetry/containers` or `/v1/worker/telemetry/logs`.
- So the orchestrator never reads data that comes from the telemetry SQLite DB.

## Summary

- **node:info:** Does not use telemetry DB.
  Live from env + placeholders.
- **node:stats:** Does not use telemetry DB.
  Live from env + placeholders.
- **GET .../containers:** Uses DB (read).
  Always empty in production (no writer).
- **GET .../containers/{id}:** Uses DB (read).
  Always empty in production.
- **GET .../logs:** Uses DB (read).
  Always empty in production.
- **Retention / vacuum:** Uses DB (write).
  Deletes and VACUUM on empty/sparse data.
- **Orchestrator telemetry:** Does not use DB.
  Only pulls node:info and node:stats.

## Conclusion

**Right now** the worker node telemetry database is used to:

1. **Serve** the containers and logs API endpoints.
   They return empty results in production because nothing writes to the DB.
2. **Run** retention and vacuum on that (empty) store.

It is **not** used for node:info or node:stats (those are computed live and do not touch the DB).
The orchestrator does not consume the DB-backed endpoints.
To match the spec (container inventory, container events, service/container log indexing), production writers would need to be added.
For example, node-manager or worker-api could record container lifecycle and log events into the store.
