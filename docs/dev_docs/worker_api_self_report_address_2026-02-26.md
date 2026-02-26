# Worker API Self-Reported Address (2026-02-26)

## Summary

The worker node now reports its Worker API address at registration and in capability reports.
The orchestrator uses that address to set or update the node's dispatch URL in the database.
An explicit operator override (e.g. same-host dev) remains supported and must be documented as an override.

## Changes

- **Requirements:** REQ-WORKER-0139 (node MUST report `worker_api.base_url`), REQ-ORCHES-0114a (orchestrator MUST use node-reported URL unless explicit override).
- **Payloads:** Capability report and registration request include optional `worker_api.base_url` (full URL for dispatch); see `worker_node_payloads.md`.
- **Orchestrator:** On Register (new and existing) and ReportCapability, the handler calls `applyWorkerAPIURLFromCapability`; when `WORKER_API_TARGET_URL` is set it is used as explicit override, otherwise the node-reported URL is stored.
- **Node:** Config has `AdvertisedWorkerAPIURL` from env `NODE_ADVERTISED_WORKER_API_URL`; `buildCapability` includes `worker_api` when set.
- **E2E:** `setup-dev.sh` exports `NODE_ADVERTISED_WORKER_API_URL` in `start_node` so the node advertises the URL the orchestrator (in container) can use.
- **Features:** New scenario "Node registration includes worker API base_url" and step for verifying stored `worker_api_target_url`.

## Override

When the worker runs on the same host as the orchestrator (e.g. dev), the operator may set `WORKER_API_TARGET_URL` so the control-plane uses a known URL (e.g. `http://host.containers.internal:12090`).
That is an explicit override; when unset, the node-reported URL is used.
