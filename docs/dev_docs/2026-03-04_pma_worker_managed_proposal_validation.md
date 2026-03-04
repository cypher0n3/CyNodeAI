# Validation Report: PMA Worker-Managed Services Proposal Implementation

## Metadata

- Date: 2026-03-04
- Input: `docs/dev_docs/2026-03-04_pma_worker_managed_lifecycle_spec_proposal.md`

## Summary

All proposal-driven spec and requirements changes have been applied to canonical documentation.
Remaining references that mention disabling PMA exist only in a historical dev_doc root-cause writeup and are explicitly marked as legacy drift.

## Proposal Coverage Checklist

Each subsection below maps a proposal area to the canonical docs where it was implemented.

### Worker-Managed Services Desired/observed/observed State in Payloads

- Implemented in `docs/tech_specs/worker_node_payloads.md`:
  - `node_capability_report_v1` includes `managed_services` capability and `managed_services_status.services[]`.
  - `node_configuration_payload_v1` includes `managed_services.services[]` desired state.
  - Agent runtime services include required bootstrap fields (inference + worker-proxy orchestrator connectivity).

### Worker Node Responsibilities for Managed Services and Bidirectional Proxy

- Implemented in `docs/tech_specs/worker_node.md`:
  - `Managed Service Containers` section (desired state convergence, restart/backoff, reporting).
  - `Worker Proxy Bidirectional (Managed Agents)` section (orchestrator->agent and agent->orchestrator via worker proxy).

### Worker API Proxy Surface (Bidirectional)

- Implemented in `docs/tech_specs/worker_api.md`:
  - `Managed Agent Proxy (Bidirectional)` section:
    - orchestrator->agent proxy (recommended endpoint: `POST /v1/worker/managed-services/{service_id}/proxy:http`)
    - agent->orchestrator proxy endpoints (loopback/UDS only) for MCP and ready callbacks
    - auth/audit expectations

### Orchestrator Startup/readiness/readiness and Managed Services Tracking

- Implemented in `docs/tech_specs/orchestrator_bootstrap.md`:
  - Removed "cynode-pma when enabled" from orchestrator independent startup.
  - PMA startup now defined as orchestrator instructing a worker to start PMA as a managed service.
  - PMA online is determined by worker-reported `managed_services_status` (not direct probing).

- Implemented in `docs/tech_specs/orchestrator.md`:
  - Added `Managed Services (Worker-Managed)` section with desired/observed state definitions and routing rules.
  - PMA described as worker-managed and always required.

### PMA Runtime Spec Updates

- Implemented in `docs/tech_specs/cynode_pma.md`:
  - PMA reframed as orchestrator-owned runtime hosted as a worker-managed service container.
  - Explicit requirement that agent->orchestrator communication is worker-proxied.
  - Readiness learned via worker-reported managed service status and endpoints.
  - Configuration surface now includes inference connectivity and worker-proxy endpoints for MCP/callbacks.

### MCP Gateway/tooling/tooling Docs Integration for Long-Running Agents

- Implemented in:
  - `docs/tech_specs/mcp_gateway_enforcement.md`: edge enforcement mode explicitly includes worker-managed long-lived agent containers (e.g. PMA) as node-local agent runtimes.
  - `docs/tech_specs/mcp_tooling.md`: node-local agent runtimes include worker-managed agent containers; clarifies edge enforcement vs worker-proxied agent->orchestrator traffic.

### Requirements Updates

- Implemented in `docs/requirements/worker.md`:
  - Added `REQ-WORKER-0160`..`REQ-WORKER-0163` for managed services and managed agent proxy bidirectional constraints.

- Implemented in `docs/requirements/orches.md`:
  - Updated `REQ-ORCHES-0150` and `REQ-ORCHES-0151` to be worker-managed and worker-reported.
  - Added `REQ-ORCHES-0160`..`REQ-ORCHES-0162` for desired state, tracking, and routing via worker-mediated endpoints.

### Remove Ambiguity About PMA Being Optional

- Canonical specs and requirements no longer present "PMA enabled/disabled" or "run without PMA" language:
  - `docs/tech_specs/orchestrator_bootstrap.md`
  - `docs/tech_specs/orchestrator.md`
  - `docs/requirements/orches.md`

- Supporting planning doc updated:
  - `docs/mvp_plan.md` updated to remove `PMA_BASE_URL` compose assumptions and subprocess language; describes worker-mediated routing and bootstrap.

Remaining legacy references:

- `docs/dev_docs/2026-03-04_pma_auto_start_root_cause.md` contains historical references to `PMA_ENABLED=false` for the prior dev setup.
  This document also explicitly states PMA is always required and that disablement wording is legacy drift.

## Validation Steps Performed

- Content verification via repository-wide searches for:
  - `PMA_ENABLED`, `run without cynode-pma`, `when PMA is enabled`, `PMA_BASE_URL`, and compose DNS assumptions.
- Markdown lint on all modified files using `just lint-md`.
