# Review: Worker Node Proxy Handling for Managed Agents (PMA, PAA, SBA)

## 1. Metadata

- Date: 2026-03-08
- Purpose: Document how the worker handles proxies for managed agents and why "a proxy running" may not be visible in live tests.
- Status: Documentation only; no code or spec edits.

## 2. Spec Expectation: Proxy When an Agent is Running

Per [Worker Node Technical Spec](../tech_specs/worker_node.md) (Worker Proxy Bidirectional / Managed Agents):

- Managed agent runtimes (PMA, PAA, SBA) MUST communicate with the orchestrator **through the worker proxy** in both directions whenever they run on a worker.
  There is no exception; they must never connect directly to the orchestrator.
- **Orchestrator to agent:** Worker exposes a worker-mediated endpoint (Worker API reverse proxy) for chat handoff, health, etc.
- **Agent to orchestrator:** Worker MUST expose worker-local proxy endpoints (internal proxy) that the managed agent uses to call the MCP gateway and callbacks; the worker proxy forwards those requests and attaches worker-held agent tokens.

So the **intended** behavior is: whenever a managed agent (PMA, PAA, SBA) is running on a worker, the worker should have:

1. **Orchestrator-to-agent proxy** - Handled by the Worker API public route `POST /v1/worker/managed-services/{service_id}/proxy:http` (bearer auth).
   This is always available when Worker API is up; no separate "proxy process."
2. **Agent-to-orchestrator internal proxy** - Per-service UDS listeners so the agent container can call the worker over a socket; the worker forwards to the orchestrator and attaches the agent token.
   These listeners are created by the **Worker API process** at startup, not by a separate proxy process.

There is **no separate long-lived "proxy" process** for managed agents.
The proxy is implemented inside the Worker API: public HTTP for orchestrator-to-agent, and internal UDS listeners for agent-to-orchestrator.

## 3. Two Different "Proxies" on the Worker

To avoid confusion when checking "is a proxy running":

- **Proxy:** **Internal proxy (managed agent)**
  - purpose: Agent-to-orchestrator (MCP, callbacks); worker attaches agent token.
  - when it runs: When Worker API starts **and** it has a list of managed services (from node config).
    One UDS listener per `service_id`.
  - where implemented: Worker API process: `startInternalUDSListeners()` in `worker_node/cmd/worker-api/main.go`.
- **Proxy:** **Inference proxy**
  - purpose: Per-job sidecar so the sandbox can call Ollama at `localhost:11434` without leaving the job network.
  - when it runs: Only when a **job** that uses inference is run; one sidecar container per such job.
  - where implemented: `worker_node/cmd/inference-proxy`; started by executor for each inference job.

So:

- **Managed-agent proxy:** Part of Worker API; UDS sockets under `<state_dir>/run/managed_agent_proxy/<service_id>/proxy.sock`.
  No separate process.
- **Inference proxy:** Separate sidecar container per inference job; not tied to PMA/PAA/SBA being "running."

If you expect "a proxy running whenever there's an agent (PMA, PAA, SBA)," that refers to the **internal proxy** (UDS listeners in Worker API).
You will not see a dedicated proxy process; you should see Worker API listening on those UDS paths when the node config includes managed services.

## 4. When Are Internal Proxy UDS Listeners Created?

Implementation flow:

1. **Node Manager** fetches node config from the orchestrator (includes `managed_services.services[]` when PMA or other managed agents are desired for this node).
2. **applyConfigAndStartServices** (in `worker_node/internal/nodeagent/nodemanager.go`):
   - Calls **applyWorkerProxyConfigEnv(nodeConfig)**, which sets `WORKER_NODE_CONFIG_JSON` (sanitized config including `managed_services`) and related env in the Node Manager process.
   - Calls **StartWorkerAPI** (binary or container).
3. **Worker API** at startup:
   - **setupWorkerStateAndProxyConfig** -> **loadWorkerProxyConfig**.
   - If `WORKER_NODE_CONFIG_JSON` is set, **applyNodeConfigToWorkerProxyConfig** parses it and, for each `managed_services.services[]` entry with `service_id` and `orchestrator`, adds `<state_dir>/run/managed_agent_proxy/<service_id>/proxy.sock` to `InternalProxy.SocketByService`.
   - **startInternalUDSListeners** creates one UDS listener per entry in `SocketByService` and serves the internal mux (MCP call, agent ready) on each.

So internal proxy UDS listeners exist only when:

- Worker API was started with **WORKER_NODE_CONFIG_JSON** set, and
- That JSON contains **managed_services.services** with at least one service that has `orchestrator` set.

If either is missing, `SocketByService` is empty and **no** internal proxy UDS listeners are created.

## 5. Why You Might Not See the Proxy in Live Tests

Common reasons the internal proxy is not visible or not active:

### 5.1 No Separate Process

The managed-agent proxy is not a separate process.
It is the Worker API process listening on additional UDS sockets.
So `ps` or process lists will not show a distinct "proxy" process.
To confirm it, you would check that the Worker API process has open sockets under `<state_dir>/run/managed_agent_proxy/` (e.g. `ls` or `ss -x`).

### 5.2 Worker API Run as Container Without Node Config

When Node Manager starts Worker API **as a container** (`NODE_MANAGER_WORKER_API_IMAGE` set), the code path is **startWorkerAPIContainer** in `worker_node/cmd/node-manager/main.go`.
That function only passes a fixed set of env vars into the container:

- `WORKER_API_BEARER_TOKEN`, `WORKER_API_STATE_DIR`, `LISTEN_ADDR`, `NODE_SKIP_NODE_BOOT_RECORD`
- Plus optionally `INFERENCE_PROXY_IMAGE`, `OLLAMA_UPSTREAM_URL`, `CONTAINER_RUNTIME`

It does **not** pass `WORKER_NODE_CONFIG_JSON`, `ORCHESTRATOR_INTERNAL_PROXY_BASE_URL`, or `WORKER_MANAGED_SERVICE_TARGETS_JSON`.
So when Worker API runs in a container started this way, it never receives the node config; `SocketByService` stays empty and **no internal proxy UDS listeners are created**.

So in any setup where Worker API is started as a container by Node Manager (e.g. E2E or live tests with `NODE_MANAGER_WORKER_API_IMAGE`), the internal proxy for managed agents will not be present until the container is given the node config (e.g. by passing these env vars when starting the container).

### 5.3 Orchestrator Not Sending Managed Services

If the orchestrator does not include `managed_services.services` (e.g. PMA not selected for this node, or PMA disabled), the config passed to Worker API has no managed services, so `SocketByService` is empty and no UDS listeners are created.
That is consistent with "no managed agent on this node."

### 5.4 Config Only at Worker API Startup

Worker API loads `WORKER_NODE_CONFIG_JSON` once at startup.
It does not re-read config when the orchestrator later adds or removes managed services (e.g. dynamic config).
So new managed services only get internal proxy listeners after a Worker API restart that receives the updated config.

## 6. Does the Proxy Setup Support Agent Containers With No Networking?

**Short answer: No.**
The current implementation does not start managed agent containers (PMA, PAA, SBA) with no networking (e.g. `--network none`).

### 6.1 Spec and Requirement Stance

Per [REQ-WORKER-0174](../requirements/worker.md#req-worker-0174) and [worker_node.md](../tech_specs/worker_node.md) (CYNAI.WORKER.AgentNetworkRestriction): **all** agent runtimes on a worker (whether managed service or not, including PMA, PAA, SBA) MUST be network restricted, and all inbound and outbound traffic MUST route through worker proxies; violating this violates a security boundary and is not acceptable.
Managed service containers MUST be started with network restriction so they have no network path except to the worker proxy.
Sandbox containers (job runners, including SBA) are already required to be network-restricted per [sandbox_container.md](../tech_specs/sandbox_container.md).
Implementation MUST converge to this; the current implementation (port-publish for orchestrator-to-agent) is a gap until orchestrator-to-agent uses a non-network path (e.g. UDS).

### 6.2 Current Implementation

In `worker_node/internal/nodeagent/runargs.go`, `BuildManagedServiceRunArgs`:

- Mounts the per-service UDS directory so the agent can call the internal proxy (agent-to-orchestrator) with no network.
- Does **not** set `--network none` (or equivalent).
- Publishes a host port for the service type (e.g. `-p 8090:8090` for PMA) so the Worker API can reach the agent for **orchestrator-to-agent** traffic (chat handoff, health).

Orchestrator-to-agent is implemented as Worker API issuing HTTP requests to the agent (e.g. `http://127.0.0.1:8090` or `host.containers.internal:8090`).
That requires the agent container to be reachable on a port, so the container is started with the default bridge (or host) network and port publish.
So the proxy setup today does **not** support starting agent containers with no networking.

### 6.3 What Would Be Needed for No-Network Agent Containers

Agent-to-orchestrator already uses UDS (no network).
To run agent containers with `--network none`, orchestrator-to-agent would also need a non-network path: for example a second UDS (or shared socket) on which the agent listens for incoming requests and the Worker API connects, so that no TCP port or network namespace is required.
That would be a design and implementation change (agent image would need to support a UDS server for incoming proxy requests, and the worker would need to use that instead of HTTP to a published port).

## 7. Summary Table

- **Scenario:** Node Manager starts Worker API **binary**; config has managed_services
  - internal proxy uds listeners (agent-to-orchestrator): Yes (env inherited; Worker API creates listeners).
- **Scenario:** Node Manager starts Worker API **container**; config has managed_services
  - internal proxy uds listeners (agent-to-orchestrator): No (container not given WORKER_NODE_CONFIG_JSON).
- **Scenario:** Worker API started by compose/script without WORKER_NODE_CONFIG_JSON
  - internal proxy uds listeners (agent-to-orchestrator): No.
- **Scenario:** Config has no managed_services
  - internal proxy uds listeners (agent-to-orchestrator): No (nothing to listen for).

## 8. References

- Requirement: [REQ-WORKER-0174](../requirements/worker.md#req-worker-0174) - All agents network restricted; all traffic via worker proxies; security boundary.
- Spec: [Worker Node](../tech_specs/worker_node.md) - Worker Proxy Bidirectional (Managed Agents), Agent Network Restriction (CYNAI.WORKER.AgentNetworkRestriction), Agent-To-Orchestrator UDS Binding.
- Spec: [Worker API](../tech_specs/worker_api.md) - Internal proxy routes and per-service UDS.
- Implementation: `worker_node/cmd/worker-api/main.go` - `loadWorkerProxyConfig`, `applyNodeConfigToWorkerProxyConfig`, `startInternalUDSListeners`.
- Implementation: `worker_node/cmd/node-manager/main.go` - `startWorkerAPIBinary` (inherits env), `startWorkerAPIContainer` (does not pass node config env).
- Node Manager flow: `worker_node/internal/nodeagent/nodemanager.go` - `applyConfigAndStartServices`, `applyWorkerProxyConfigEnv`.
- Dev docs: [2026-03-05_worker_proxy_spec_reconciliation_plan.md](2026-03-05_worker_proxy_spec_reconciliation_plan.md), [2026-03-06_worker_proxy_phase5_6_validation.md](2026-03-06_worker_proxy_phase5_6_validation.md).
