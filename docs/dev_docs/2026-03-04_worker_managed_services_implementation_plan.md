# Implementation Plan: Worker-Managed Services and Worker-Hosted PMA

## Metadata

- Date: 2026-03-04T15:06:03-05:00
- Status: implementation plan (post-spec update)
- Inputs:
  - `docs/dev_docs/2026-03-04_pma_worker_managed_lifecycle_spec_proposal.md`
  - Canonical updates already applied across:
    - `docs/tech_specs/worker_node_payloads.md`
    - `docs/tech_specs/worker_node.md`
    - `docs/tech_specs/worker_api.md`
    - `docs/tech_specs/orchestrator_bootstrap.md`
    - `docs/tech_specs/orchestrator.md`
    - `docs/tech_specs/cynode_pma.md`
    - `docs/tech_specs/mcp_gateway_enforcement.md`
    - `docs/tech_specs/mcp_tooling.md`
    - `docs/requirements/worker.md`
    - `docs/requirements/orches.md`

## Goal

Implement worker-managed services (general framework) and make PMA (`cynode-pma`) run as a worker-managed service container with a bidirectional worker proxy:

- Orchestrator expresses **desired state** via node config `managed_services.services[]`.
- Worker reconciles desired state into containers, health checks them, restarts with backoff, and reports **observed state** via capability report `managed_services_status.services[]`.
- Orchestrator tracks managed services and routes to them via worker-mediated endpoints.
- Managed agents (PMA and future) do not connect directly to orchestrator; agent->orchestrator traffic is **worker-proxied**.
- PMA is always required.

## Non-Goals (This Plan)

- Multi-node "migrate PMA to more capable node" (post-MVP behavior).
- Full operator UI for managed services (CLI/web console).
- A full policy engine for service placement beyond minimal constraints needed for PMA placement.

## High-Level Architecture Changes

This section describes the target runtime responsibilities for worker, worker API, and orchestrator after implementing the new specs.

### Data Plane (Worker)

- Add a worker "managed services" supervisor in Node Manager:
  - Reconcile `managed_services.services[]` into long-lived containers.
  - Perform health checks and expose worker-mediated endpoints for each service.
  - Maintain restart/backoff state and bounded error reporting.
  - Emit capability report updates when service state changes.

- Add Worker API "Managed Agent Proxy (Bidirectional)" endpoints:
  - Orchestrator->agent proxy endpoint for HTTP requests to the agent container.
  - Agent->orchestrator proxy endpoints (loopback/UDS only) for:
    - MCP gateway calls
    - agent ready/callback signaling

### Control Plane (Orchestrator)

- Add an orchestrator "managed services controller":
  - Determine PMA placement (single host initially).
  - Generate desired state for PMA into node config payload (`managed_services.services[]`).
  - Track observed state from node capability reports.
  - Provide the current PMA worker-mediated endpoint to user-gateway for `cynodeai.pm` routing.
  - Gate `/readyz` on PMA observed state `ready`.

## Implementation Phases (Order Matters)

This section provides a step-by-step delivery sequence designed to keep the system testable at each stage.

### Phase 0: Repo Hygiene, Guardrails, and Baseline Tests

- Ensure local checks are passing before change:
  - `just ci`
  - `just docs-check` (already passing for spec changes)

### Phase 1: Contracts and Payload Plumbing (Go Shared Types)

Goal: both orchestrator and worker can read/write the new payload fields.

- Update shared contracts module (`go_shared_libs/`) to include:
  - `node_configuration_payload_v1.managed_services.services[]`
  - `node_capability_report_v1.managed_services_status.services[]`
- Ensure JSON tags match the canonical `snake_case` payload spec.
- Add unit tests for JSON round-trip and schema stability:
  - Parse/serialize golden fixtures for both payloads.

Acceptance criteria:

- Orchestrator can emit config payload containing managed services.
- Worker can parse managed services and accept config without errors.
- Worker can report managed services status in capability report.

### Phase 2: Worker Node Manager Managed-Services Supervisor (Desired State -> Containers)

Goal: worker starts PMA container and keeps it running based on config.

Core tasks:

- Add an in-memory desired state store keyed by `service_id`.
- Add reconciliation loop triggered by:
  - initial config fetch at startup
  - config updates (future dynamic config)
  - periodic reconcile tick (low frequency)
- For each desired service:
  - Ensure container exists and is running with desired image/args/env.
  - Apply update policy for spec changes (stop old, start new).
  - Implement bounded restart with backoff (per-service).
  - Health check per configured path/expected status.
- Generate worker-mediated endpoints per service:
  - For PMA: endpoint is the worker API reverse proxy path (not host:port).
- Produce `managed_services_status.services[]` and include it in capability report:
  - Emit on state transitions (starting->ready, ready->unhealthy, etc.).

Acceptance criteria:

- Worker starts PMA container when instructed in config.
- Worker reports PMA state transitions and endpoints.
- Worker restarts PMA if killed, with backoff, and reports recovery.

### Phase 3: Worker API Bidirectional Proxy Endpoints

Goal: implement the worker-proxy contract for managed agents.

Orchestrator->agent proxy:

- Implement `POST /v1/worker/managed-services/{service_id}/proxy:http`.
- Enforce:
  - caller auth (existing Worker API bearer token).
  - only proxy to configured managed services (`service_id` in desired state).
  - strict size limits for request/response bodies.
  - header allowlists for both directions.
  - audit log entries (at minimum: service_id, service_type, caller identity, timing).

Agent->orchestrator proxy (loopback/UDS only):

- Implement endpoints:
  - `POST /v1/worker/internal/orchestrator/mcp:call`
  - `POST /v1/worker/internal/orchestrator/agent:ready`
- Enforce:
  - binding loopback/UDS only.
  - agent authentication using orchestrator-issued agent token or capability lease.
  - fail-closed on missing/invalid auth.
  - audit log entries with agent identity + task context (when present).

Acceptance criteria:

- Orchestrator can call PMA endpoints via worker proxy.
- PMA can call MCP gateway and ready callbacks through worker proxy without direct orchestrator networking.

### Phase 4: Orchestrator Managed-Services Controller (Desired State + Tracking + Readiness)

Goal: orchestrator drives PMA lifecycle through worker config and uses observed state for routing/readiness.

Core tasks:

- Add managed services desired state generator:
  - Choose PMA host node (initial policy: first eligible dispatchable node, prefer orchestrator_host when available).
  - Emit `managed_services.services[]` entry for PMA:
    - `service_type=pma`, image, args/role, healthcheck.
    - Inference configuration (`node_local`, `external`, `remote_node`) consistent with current routing settings.
    - `orchestrator` proxy URLs for MCP and callbacks (worker-local endpoints).
    - token delivery mechanism (short-lived token preferred; token ref if needed).
- Update orchestrator node config endpoint behavior:
  - Include managed services desired state in the returned node config payload for the selected node.
- Persist/track observed state:
  - Ingest `managed_services_status` from capability reports.
  - Store last observed state + heartbeat timestamp.

Readiness:

- Update `/readyz` logic to treat PMA online only if:
  - a recent capability report indicates PMA `state=ready` for the selected service instance.

Acceptance criteria:

- Orchestrator issues PMA desired state and sees worker report it `ready`.
- Orchestrator `/readyz` stays 503 until PMA is ready, then becomes 200.

### Phase 5: User-Gateway Routing to PMA via Worker-Mediated Endpoint

Goal: `model=cynodeai.pm` goes to PMA through worker proxy, not compose DNS or `PMA_BASE_URL`.

Core tasks:

- Replace direct `PMA_BASE_URL` routing with:
  - orchestrator-resolved PMA endpoint (worker-mediated proxy URL).
  - calls that go via the orchestrator (preferred) or directly to worker API if that is the chosen architecture.
- Ensure request sanitization/logging/persistence remains on orchestrator side as required by the chat API spec.

Acceptance criteria:

- E2E: `POST /v1/chat/completions` with `model=cynodeai.pm` succeeds with PMA reachable only through the worker.

### Phase 6: Tests (Prove Spec Compliance)

Unit tests:

- Shared payload types: JSON schema and round-trip tests.
- Worker managed services supervisor:
  - reconciliation behavior for start/update/restart
  - health check gating
- Worker proxy endpoints:
  - auth, allowlist, size limits, audit emission
- Orchestrator managed services controller:
  - placement selection
  - readiness gating based on observed state

Integration tests:

- Worker API + Node Manager integration:
  - start PMA container, proxy health check, proxy chat completion.

BDD/E2E:

- Add/extend scenarios to assert:
  - PMA is not available before the worker reports it ready.
  - PMA becomes available only after orchestrator issues desired state.
  - `cynodeai.pm` routing works through worker proxy.
  - restart behavior: kill PMA container and validate worker restarts and routing recovers.

Suggested commands during implementation:

- `just test-go` (and/or `just test-go-cover`)
- `just test-bdd`
- `just e2e`
- `just ci`

## Rollout / Breaking Change Notes

- This is a breaking architecture change: PMA is no longer an "orchestrator-local" service or compose DNS peer.
- During the transition, implement a compatibility mode only if explicitly required; otherwise migrate directly:
  - Update code to prefer worker-managed PMA and remove direct-PMA assumptions.
  - Ensure docs and tests enforce the new flow.

## Deliverables Checklist

- Worker:
  - Managed services reconciliation loop implemented
  - PMA container started/kept running via desired state
  - Bidirectional worker proxy endpoints implemented and audited
  - Capability reports include `managed_services_status`

- Orchestrator:
  - Desired state generation for PMA in node config
  - Observed state tracking for managed services
  - `/readyz` gates on PMA `ready`
  - `cynodeai.pm` routing uses worker-mediated endpoint

- Tests:
  - Unit coverage for payloads, worker supervisor, proxy, orchestrator controller
  - BDD/E2E proving the new startup/routing/lifecycle
