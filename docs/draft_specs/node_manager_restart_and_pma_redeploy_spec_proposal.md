# Spec/Reqs Update: Node-Manager Independent Restart and Orchestrator-Triggered PMA Redeploy

- [Scope and Metadata](#scope-and-metadata)
- [Summary](#summary)
- [Goals](#goals)
- [Proposed Requirements and Spec Changes](#proposed-requirements-and-spec-changes)
  - [1. Restart Node-Manager Independent of the Stack](#1-restart-node-manager-independent-of-the-stack)
  - [2. Orchestrator-Triggered PMA Restart/Redeploy](#2-orchestrator-triggered-pma-restartredeploy)
  - [3. Python Setup-Dev Scripts Support](#3-python-setup-dev-scripts-support)
- [Traceability](#traceability)
- [Open Points](#open-points)
- [References](#references)

## Scope and Metadata

- Date: 2026-03-10
- Status: Proposal (draft_specs; not merged to requirements/specs)
- Scope: Node-manager lifecycle independence from orchestrator stack; orchestrator-triggered PMA restart/redeploy; Python setup-dev commands to support both.

## Summary

Three related capabilities:

1. **Independent node-manager restart:** Operators and dev workflows must be able to restart the node-manager (and its managed services, including worker-api and PMA) without stopping the orchestrator stack (compose/control-plane, user-gateway, postgres, etc.).
2. **Orchestrator-triggered PMA restart/redeploy:** The orchestrator must be able to instruct the node (node-manager) to restart or redeploy the PMA managed service when needed (e.g. config change, token rotation, image update), using current mechanisms where possible.
3. **Python setup-dev:** The Python setup-dev scripts (`setup_dev.py` / `setup_dev_impl.py`) must support (1) and (2) using the same mechanisms as production (stop/start node only; PMA redeploy via config push or documented workflow).

## Goals

- Node-manager can be stopped and started without tearing down the orchestrator stack.
- Orchestrator can trigger PMA restart/redeploy on the worker node via existing config-delivery and reconciliation behavior (dynamic config + `allow_service_restart`), without requiring a full node restart when possible.
- Dev workflow: `just setup-dev stop-node`, `just setup-dev start-node`, and optionally `just setup-dev restart-node` to restart only the node; PMA redeploy in dev uses the same mechanism as production (config push or explicit restart-node).

## Proposed Requirements and Spec Changes

Proposed new requirements and tech-spec text for independent node-manager restart, orchestrator-triggered PMA redeploy, and setup-dev support.

### 1. Restart Node-Manager Independent of the Stack

Node-manager must be restartable without tearing down the orchestrator stack.

#### 1.1. Independent Restart Requirement

- Add to `docs/requirements/worker.md` (or deployment/operations doc if one exists):
  - **REQ-WORKER-0XXX (proposed):** The deployment and dev tooling MUST support restarting the Node Manager (and only the Node Manager and its dependent processes/containers) without stopping the orchestrator stack (control-plane, user-gateway, database, MCP gateway, API egress, etc.).
  - Trace: [worker_node.md](../tech_specs/worker_node.md) Node Manager, Deployment and Auto-Start; [development_setup.md](../development_setup.md).

#### 1.2. Independent Restart Tech Spec (`Worker_node_node`)

- In `docs/tech_specs/worker_node.md`, under Deployment and Auto-Start (or Node Manager):
  - State explicitly that the Node Manager is a separate deployable unit: it MAY be stopped and started independently of the orchestrator stack.
  - When the Node Manager is restarted, it MUST follow the existing [Node Startup Procedure](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure): contact orchestrator, receive configuration, then start Worker API and managed services (e.g. PMA) per config.
  - No change to the existing shutdown behavior: Node Manager still stops its managed containers and sandbox containers on SIGTERM per [CYNAI.WORKER.NodeManagerShutdown](../tech_specs/worker_node.md#spec-cynai-worker-nodemanagershutdown).

#### 1.3. Independent Restart Implementation Note (Setup-Dev)

- Today `stop_all()` in setup-dev kills node-manager then runs compose down; there is no "stop node only" or "start node only."
- Implementation would add: a "stop node only" step (SIGTERM node-manager, wait for exit, optionally clean node-managed containers) and a "start node only" step (assume compose is up; run `start_node()`).
- These can be exposed as separate commands so that "restart node-manager only" = stop-node + start-node.

### 2. Orchestrator-Triggered PMA Restart/Redeploy

The orchestrator must be able to trigger PMA restart or redeploy on the node using current mechanisms.

#### 2.1. PMA Redeploy Requirement (Orchestrator)

- Add to `docs/requirements/orches.md` (or worker.md depending on ownership):
  - **REQ-ORCHES-0XXX (proposed):** The orchestrator MUST be able to trigger a restart or redeploy of the PMA managed service on a worker node when needed (e.g. configuration change, token rotation, image or env update).
  - Trace: [worker_node_payloads.md](../tech_specs/worker_node_payloads.md) `managed_services`, `policy.updates.allow_service_restart`; [worker_node.md](../tech_specs/worker_node.md) Dynamic Configuration Updates, Managed Service Containers.

#### 2.2. PMA Redeploy Current Mechanism (Preferred)

- Existing behavior already supports this when dynamic config and service restart are enabled:
  - Orchestrator delivers an updated `node_configuration_payload_v1` to the node (e.g. via poll or push), with `policy.updates.allow_service_restart: true` and a change in `managed_services.services[]` for the PMA (e.g. version bump, env change, image tag, or any field that causes the node to reconcile).
  - Node Manager applies the update and, per [CYNAI.WORKER.ManagedServiceContainers](../tech_specs/worker_node.md#spec-cynai-worker-managedservicecontainers), converges to desired state: "If running with a different spec (image/env/args), update it per the rollout policy (stop old, start new)."
- So the orchestrator "tells" the node to restart/redeploy PMA by pushing an updated node config that differs for the PMA managed service; the node reconciles and restarts/redeploys when `allow_service_restart` is true.

#### 2.3. PMA Redeploy Tech Spec (`Worker_node_node` and Orchestrator)

- In `docs/tech_specs/worker_node.md`, under Dynamic Configuration Updates or Managed Service Containers:
  - State explicitly: When the orchestrator needs to trigger a PMA (or other managed service) restart or redeploy, it MUST deliver an updated node configuration payload that includes the revised `managed_services` entry for that service.
  - When `policy.updates.allow_service_restart` is true, the node MUST apply the update and restart or replace the managed service as needed to converge to the new desired state.
- Optionally in `docs/tech_specs/orchestrator.md` (or control-plane spec): The control-plane MUST be able to generate and deliver such an updated node config (e.g. with a changed `managed_services.services[]` entry for PMA) so that the node restarts or redeploys PMA without requiring a node-manager process restart.

#### 2.4. PMA Redeploy Alternative (Out of Scope)

- A dedicated control-plane or gateway endpoint (e.g. "POST /v1/nodes/{node_id}/managed-services/{service_id}/restart") that the node-manager or Worker API implements could be added later; this proposal does not require it.

### 3. Python Setup-Dev Scripts Support

Python setup-dev must support stop-node, start-node, and restart-node using current mechanisms.

#### 3.1. Setup-Dev Requirement

- Document or add requirement: The Python setup-dev implementation MUST support restarting the node-manager independently of the rest of the stack, using the same operational steps as above (stop node only, start node only).

#### 3.2. Setup-Dev Spec and Implementation

- In `docs/development_setup.md` and `scripts/README.md`:
  - Document new commands (when implemented): `stop-node`, `start-node`, `restart-node`.
  - stop-node: Stop only the node-manager (SIGTERM, wait for exit) and optionally clean node-managed containers; do not run compose down.
  - start-node: Start only the node-manager (assume orchestrator stack is already up); same env and binary as `start`; wait for worker-api healthz and optionally orchestrator readyz.
  - restart-node: stop-node then start-node.
- Implementation in `scripts/setup_dev_impl.py`:
  - Add `stop_node_only()` that performs: (1) capture container logs if desired, (2) kill node-manager (read PID from `NODE_MANAGER_PID_FILE`, SIGTERM, wait up to 15s, then SIGKILL if needed), (3) optionally run the same "stop node-managed containers" cleanup used in `stop_all()`.
  - Do not call `stop_orchestrator_stack_compose()` or free worker port in a way that affects other services.
  - Add or reuse `start_node()` for "start node only"; caller must ensure orchestrator stack is already up.
- Implementation in `scripts/setup_dev.py`:
  - Add commands: `stop-node`, `start-node`, `restart-node` (restart-node = stop-node then start-node).
  - Document that these are invoked as `just setup-dev stop-node` etc. once the Python script supports them (justfile passes command through).
- PMA redeploy in dev:
  - Document that "restart PMA" in dev can be achieved by: (a) `restart-node` (restarts node-manager and all its managed services, including PMA), or (b) when orchestrator supports it, by triggering a config refresh so the node receives an updated `managed_services` and reconciles (same as production).
  - No new script command is strictly required for (b) if the orchestrator already supports pushing updated config; optionally a small helper or doc note could describe how to trigger a config refresh in dev.

#### 3.3. Setup-Dev Current Mechanisms

- Reuse existing `start_node()` and the same node-manager binary and env as `start`.
- Reuse the same kill and cleanup logic as `stop_all()` but without compose down or global port free.

## Traceability

- **REQ-WORKER-0257:** Node Manager shutdown (unchanged).
- **REQ-WORKER-0XXX (new):** Node-manager restartable independent of stack.
- **REQ-ORCHES-0XXX (new):** Orchestrator can trigger PMA restart/redeploy.
- **CYNAI.WORKER.NodeManagerShutdown:** Unchanged.
- **CYNAI.WORKER.ManagedServiceContainers:** Explicit PMA restart/redeploy via config push.
- **CYNAI.WORKER.DynamicConfigurationUpdates:** Mechanism for delivering updated config.
- **worker_node_payloads.md:** `policy.updates.allow_service_restart`, `managed_services`.

## Open Points

- Exact requirement IDs (REQ-WORKER-0XXX, REQ-ORCHES-0XXX) to be assigned when merging to requirements.
- Whether `stop-node` should free the worker port (e.g. for another process to bind) or leave it until the next start-node; recommend leaving port cleanup to start-node or to the user.
- Whether to add an explicit `redeploy-pma` setup-dev command that e.g. calls control-plane to push updated config; or rely on `restart-node` + docs for dev.

## References

- [worker_node.md](../tech_specs/worker_node.md) (Node Manager, Managed Service Containers, Dynamic Configuration Updates, Node Startup Procedure)
- [worker_node_payloads.md](../tech_specs/worker_node_payloads.md) (`node_configuration_payload_v1`, `managed_services`, `policy.updates.allow_service_restart`)
- [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md) (PMA startup, worker-reported status)
- [development_setup.md](../development_setup.md) (setup-dev commands)
- [scripts/README.md](../../scripts/README.md) (setup-dev commands, startup sequence)
- [setup_dev_impl.py](../../scripts/setup_dev_impl.py) (`start_node`, `stop_all`, node-manager PID and cleanup)
