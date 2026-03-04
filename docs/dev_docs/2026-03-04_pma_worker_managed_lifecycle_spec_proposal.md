# Proposal: Worker-Managed PMA Container Lifecycle

- [Metadata](#metadata)
- [Problem Statement](#problem-statement)
- [Proposed Model (Normative)](#proposed-model-normative)
- [Required Startup Sequence (Single-Host Dev)](#required-startup-sequence-single-host-dev)
- [Required Startup Sequence (Multi-Node)](#required-startup-sequence-multi-node)
- [Worker-Managed Services Framework](#worker-managed-services-framework)
  - [Framework Definitions](#framework-definitions)
  - [Normative Model](#normative-model)
  - [Managed Service Identity](#managed-service-identity)
  - [Long-Running Agent Runtimes (Node-Local)](#long-running-agent-runtimes-node-local)
  - [Long-Running Session Sandboxes (Distinct Concept)](#long-running-session-sandboxes-distinct-concept)
- [Agent Bootstrap Information (PMA and Other Managed Agents)](#agent-bootstrap-information-pma-and-other-managed-agents)
  - [Inference Connectivity (Required)](#inference-connectivity-required)
  - [Orchestrator and MCP Connectivity (Required)](#orchestrator-and-mcp-connectivity-required)
  - [Identity and Role (Required for PMA / PAA)](#identity-and-role-required-for-pma--paa)
  - [Listen and Health (Required)](#listen-and-health-required)
  - [Instructions and Optional Config](#instructions-and-optional-config)
  - [Summary: Required Bootstrap for PMA](#summary-required-bootstrap-for-pma)
- [Configuration and Payload Shape Changes](#configuration-and-payload-shape-changes)
- [Orchestrator Tracking of Managed Services](#orchestrator-tracking-of-managed-services)
- [Reconciliation and Drift Handling](#reconciliation-and-drift-handling)
- [Readiness Semantics](#readiness-semantics)
- [Routing Semantics: Worker Proxy Bidirectional](#routing-semantics-worker-proxy-bidirectional)
- [Lifecycle Management and Restart](#lifecycle-management-and-restart)
- [Security and Trust Boundary](#security-and-trust-boundary)
- [Spec and Requirements Changes to Draft](#spec-and-requirements-changes-to-draft)
  - [Bootstrap Spec: `docs/tech_specs/orchestrator_bootstrap.md`](#bootstrap-spec-docstech_specsorchestrator_bootstrapmd)
  - [Worker Node Spec: `docs/tech_specs/worker_node.md`](#worker-node-spec-docstech_specsworker_nodemd)
  - [Payload Spec: `docs/tech_specs/worker_node_payloads.md`](#payload-spec-docstech_specsworker_node_payloadsmd)
  - [PMA Spec: `docs/tech_specs/cynode_pma.md`](#pma-spec-docstech_specscynode_pmamd)
  - [MCP Gateway and Tooling Specs: `docs/tech_specs/mcp_gateway_enforcement.md`, `docs/tech_specs/mcp_tooling.md`](#mcp-gateway-and-tooling-specs-docstech_specsmcp_gateway_enforcementmd-docstech_specsmcp_toolingmd)
  - [Worker Requirements: `docs/requirements/worker.md`](#worker-requirements-docsrequirementsworkermd)
  - [Orchestrator Requirements: `docs/requirements/orches.md`](#orchestrator-requirements-docsrequirementsorchesmd)
- [Test Implications](#test-implications)
- [Resolved Decisions (From Review)](#resolved-decisions-from-review)

## Metadata

- **Date:** 2026-03-04T12:22:54-05:00
- **Status:** draft proposal for review (spec gap)
- **Scope:** docs-only proposal; no implementation changes in this document

## Problem Statement

The current specs describe PMA startup as something the orchestrator "starts" when an inference path exists
(`docs/tech_specs/orchestrator_bootstrap.md`), and `docs/tech_specs/cynode_pma.md` explicitly classifies
`cynode-pma` as an orchestrator-side runtime.

However, the desired architecture is:

- The **worker node** is the system's container lifecycle manager for non-sandbox services it owns
  (at minimum inference, and now also PMA).
- The **orchestrator** must **instruct** the worker to start PMA by sending a **PMA start bundle**
  (image, env, inference wiring, health contract).
- The worker must start, monitor, restart, and report PMA state and endpoints back to the orchestrator.
- Orchestrator readiness and PMA routing must be driven by **worker-reported PMA readiness**.

This proposal updates the specs to make that model normative, including single-host dev.

## Proposed Model (Normative)

Definitions:

- **PMA service container:** A node-managed service container running `cynode-pma` in `project_manager` role.
  It is not a per-task sandbox job container.
- **PMA start bundle:** The orchestrator-supplied configuration that tells the node how to run PMA:
  image reference, role, listen port, environment, inference wiring, restart policy, and health contract.
- **PMA endpoint contract:** The endpoint(s) the orchestrator uses to reach PMA, which MUST flow through
  the worker node (no direct host-port assumption).

Normative behavior:

- The orchestrator MUST NOT assume PMA is running until a worker node has explicitly reported PMA ready.
- The orchestrator MUST start PMA by delivering a PMA start bundle to a selected worker node via the node
  configuration payload (or a dedicated configuration update channel).
- The worker node MUST start and manage the lifecycle of the PMA service container when instructed.
- The worker node MUST report PMA readiness and endpoints to the orchestrator and keep that report current.

## Required Startup Sequence (Single-Host Dev)

Applies when orchestrator and one worker run on the same host, which has a GPU and is inference-capable.

Required order (normative):

1. Orchestrator stack starts (postgres, control-plane, user-gateway, optional services).
2. Worker node starts Worker API, registers, and sends capability report.
3. Orchestrator acks registration and returns node configuration.
4. Worker applies configuration:
   - Starts or uses local inference backend only when instructed.
   - Starts PMA service container only when instructed.
5. Worker reports config ack and readiness, including PMA service status and endpoint.
6. Orchestrator treats "first inference path exists" as satisfied when worker reports ready and inference-capable.
7. Orchestrator delivers a configuration update that instructs the worker to start PMA.
8. Worker starts PMA container and reports it ready (with endpoint).
9. Orchestrator `/readyz` returns 200 only after PMA is ready and other prerequisites are met.

## Required Startup Sequence (Multi-Node)

In multi-node deployments, the orchestrator MUST select which node hosts the PMA service.

Selection constraints (normative):

- The selected node MUST be dispatchable and inference-capable (or the orchestrator MUST use external inference
  for PMA via API Egress and still instruct the node to start PMA with the correct external routing).
- The orchestrator MUST be able to move PMA hosting to a different node if the selected node is unavailable.

## Worker-Managed Services Framework

This section generalizes the PMA container pattern into a worker-managed services framework so the orchestrator
can direct workers to spin up additional long-lived services that are part of the larger system.

### Framework Definitions

- **Managed service**: A long-lived containerized service started and supervised by a worker node on behalf of the orchestrator.
  It is distinct from per-job sandbox containers.
- **Desired state**: The orchestrator-declared intent for which managed services should be running on which nodes and how.
- **Observed state**: The worker-reported service state (ready/unhealthy/error), runtime identity (container id), and endpoint(s).
- **Service instance**: A single placement of a managed service on a specific node.
- **Service endpoint**: An orchestrator-callable endpoint for the service that MUST be worker-mediated (proxy) unless explicitly
  specified otherwise by policy and threat model.
- **Worker proxy (bidirectional):** All traffic between a managed agent (e.g. PMA) and the orchestrator MUST flow through the
  worker's proxy: orchestrator to agent (e.g. chat handoff, health) and agent to orchestrator (e.g. MCP tool calls, ready callbacks).
  The agent MUST NOT connect directly to orchestrator hostnames or ports; the worker proxy handles both directions.

### Normative Model

- The orchestrator MUST manage managed services using a desired state model.
- The worker MUST converge observed state to the desired state for each managed service it is instructed to run.
- The orchestrator MUST track managed services as first-class resources (inventory, placement, endpoints, and health).
- The orchestrator MUST be able to update a managed service configuration (image, env, routing) via node configuration updates.
- The worker MUST restart managed service containers on failure according to the specified restart policy and report transitions.

### Managed Service Identity

Each managed service MUST have:

- A stable **service id** (string or ULID) assigned by the orchestrator.
- A **service type** (string) such as `pma`, `inference`, `model_cache`, `tooling_proxy`, etc.
- A **scope** that indicates whether it is system-scoped (global) or project-scoped.
- A **placement** that indicates the node selected to host it (service instance = service id + node id).

The worker MUST treat the service id as the stable key for reconciliation.

### Long-Running Agent Runtimes (Node-Local)

This proposal integrates the existing "node-local agent runtime" model into managed services.

Relevant existing specs:

- `docs/tech_specs/worker_node.md`:
  - Node-local agent sandbox control (low-latency path) under orchestrator-issued capability leases.
- `docs/tech_specs/mcp_gateway_enforcement.md`:
  - Edge enforcement mode (node-local agent runtimes) using capability leases, fail-closed enforcement, and auditing.
- `docs/tech_specs/mcp_tooling.md`:
  - Node-local agent runtimes may call node-hosted sandbox tools directly only under capability leases with audit.

Normative integration:

- A **long-running agent runtime** (for example, PMA, PAA, or other system agents) is a special case of a managed service:
  - It is long-lived.
  - It requires orchestrator-mediated policy enforcement and auditable tool access.
  - It may need low-latency access to node-local sandbox tools when co-located with a worker node.

The managed services framework MUST support agent runtimes as managed service types, for example:

- `service_type=pma` (Project Manager Agent runtime, always required)
- `service_type=paa` (Project Analyst Agent runtime; placement may be policy-driven)
- future: `service_type=scheduler_agent`, `service_type=connector_agent`, etc.

Tool access and enforcement:

- When a managed agent runtime is co-located on the same host as the worker node, the system MAY use the low-latency path
  for sandbox tool calls (edge enforcement mode).
- In that case:
  - The orchestrator MUST issue short-lived capability leases scoped to `task_id` and allowed tool identities.
  - The node MUST validate leases, enforce allowlists, and fail closed.
  - The node MUST emit audit records and make them available to the orchestrator (same minimum fields as gateway audit).

This proposal treats "long-running agents" as long-lived managed services plus capability-lease-based tool access, not as session sandboxes.

### Long-Running Session Sandboxes (Distinct Concept)

This proposal does not replace the existing long-running sandbox session model.

Existing specs define **session sandboxes** (long-running containers reused across multiple exec rounds) as a worker API feature for long-running work inside a sandbox environment.

Normative relationship:

- **Managed services** are for long-lived system services and agent runtimes (PMA and other services the orchestrator depends on).
- **Session sandboxes** are for long-running task execution environments (interactive or multi-step work) and remain job/task-scoped.

## Agent Bootstrap Information (PMA and Other Managed Agents)

The orchestrator MUST supply, in the managed service desired state (start bundle), all information required for the agent to start correctly and connect to inference, the orchestrator, and any other dependencies.
The following enumerates the required bootstrap information for PMA and the pattern for other CyNodeAI agents and system services.

### Inference Connectivity (Required)

The agent MUST know how it obtains inference and how to connect to it.
The orchestrator MUST set one of the following modes and supply the corresponding connection details.

- **Node-local inference**
  - This node runs the inference backend (e.g. Ollama); the agent runs on the same node.
  - Required in bundle: `inference.mode: node_local`, `inference.base_url` (URL the agent uses to reach inference on this node; e.g. `http://localhost:11434` or a worker-provided internal hostname that resolves inside the agent container).
  - Optional: `inference.default_model` (model name to use or load), `inference.warmup_required` (whether orchestrator expects model warmup before reporting ready).

- **External inference (API Egress)**
  - Inference is provided by an external provider via the orchestrator's API Egress Server; credentials are not passed to the agent.
  - Required in bundle: `inference.mode: external`, `inference.api_egress_base_url` (base URL for the API Egress Server the agent MUST use for LLM calls), `inference.provider_id` or equivalent (which configured provider/key to use, if multiple).
  - Optional: `inference.default_model`, `inference.warmup_required`.

- **Inference on another node**
  - Inference is on a different worker node; the agent reaches it via an orchestrator- or worker-mediated URL.
  - Required in bundle: `inference.mode: remote_node`, `inference.base_url` (worker-mediated or orchestrator-mediated URL to that node's inference endpoint; MUST NOT expose raw host:port of the other node).
  - Optional: `inference.default_model`, `inference.warmup_required`.

Normative: the orchestrator MUST NOT start an agent (e.g. PMA) without providing a complete, consistent inference configuration for one of the above modes.
The agent MUST fail startup or report unhealthy if inference connectivity is missing or invalid.

### Orchestrator and MCP Connectivity (Required)

The agent MUST know how to reach the orchestrator for MCP tool calls and, when applicable, for registration or callbacks.
In this model, **the worker proxy handles all agent-to-orchestrator communication**; the agent does not connect directly to the orchestrator.

- **MCP gateway (via worker proxy)**
  - Required in bundle: a URL the agent uses for MCP tool calls.
    This URL MUST be a **worker-proxy URL** (e.g. a path on the Worker API such as `/internal/mcp-gateway` or `/internal/agent/orchestrator/mcp`).
    The worker proxy forwards these requests to the actual orchestrator MCP gateway.
  - The agent MUST NOT be given the raw orchestrator MCP gateway host/port; the worker proxy is the single egress from the agent container to the orchestrator for MCP.
  - See `docs/tech_specs/mcp_gateway_enforcement.md` and `docs/tech_specs/cynode_pma.md` for tool allowlists and auth.

- **Agent credential**
  - Required: the agent MUST receive an agent-scoped token or API key (or a stable way to obtain one) for authenticating to the MCP gateway and for orchestrator-issued capability leases when using the node-local (edge) path.
  - The bundle MUST either include a short-lived token or a reference (e.g. token endpoint + scope) so the worker or agent can obtain a token; secrets MUST NOT be logged or stored in plain text in the desired state.

- **Optional callbacks (via worker proxy)**
  - If the agent must register or report ready to the orchestrator: the agent MUST call a **worker-proxy URL** (e.g. `/internal/agent/orchestrator/ready` or equivalent).
    The worker proxy forwards that to the orchestrator (e.g. control-plane ready callback).
    The agent MUST NOT call the orchestrator's callback URL directly.

### Identity and Role (Required for PMA / PAA)

- **Role**
  - Required for cynode-pma: `role` (e.g. `project_manager` or `project_analyst`); determines which instructions bundle and MCP allowlist apply.

- **Service identity**
  - Required: `service_id` (orchestrator-assigned stable id) and `service_type` (e.g. `pma`) for audit and reconciliation.

### Listen and Health (Required)

- **Listen address**
  - Required: address and port the agent listens on inside the container (e.g. `:8090`) for health checks and for handoff (e.g. user-gateway routing chat to PMA).
  - The worker MUST expose this via the worker-mediated endpoint reported back to the orchestrator; the bundle MAY include `listen_addr` or equivalent.

- **Health contract**
  - Required: path and expected response for health (e.g. `GET /healthz` -> 200); the worker uses this to determine when to report the service `ready`.

### Instructions and Optional Config

- **Instructions bundle**
  - For PMA/PAA: path or URL to the role-specific instructions bundle (or instructions root + role); may be baked into the image or supplied via env/mount.
  - The bundle MAY be provided by the orchestrator in the desired state (e.g. `instructions_root` or `instructions_url`) when not using image defaults.

- **Feature toggles and preferences**
  - Optional: feature flags (e.g. spawn analyst sub-agents), model routing preferences, or other keys that align with `docs/tech_specs/user_preferences.md` and `docs/tech_specs/external_model_routing.md` (e.g. `agents.project_manager.model_routing.prefer_local`).
  - When present, the agent MUST apply them so behavior matches orchestrator policy.

### Summary: Required Bootstrap for PMA

- **Inference:** One of: node_local + base_url, external + api_egress_base_url (+ provider), remote_node + base_url; optional default_model, warmup_required.
- **Orchestrator:** worker-proxy URL for MCP (and for ready callback if used); agent token or token reference.
  Agent MUST NOT be given raw orchestrator URLs.
- **Identity:** role (project_manager), service_id, service_type (pma).
- **Listen:** listen_addr; health path/expectation.
- **Optional:** instructions_root or instructions_url; model/preference overrides.

Other managed agents (e.g. PAA, future scheduler or connector agents) MUST receive the same categories of information as applicable to their role; the payload shape and service_type distinguish them.

## Configuration and Payload Shape Changes

This proposal requires expanding the node configuration payload (in `docs/tech_specs/worker_node_payloads.md`)
to include a node-managed services section.
For managed **agent** runtimes (PMA, PAA, etc.), the desired state MUST include the [Agent Bootstrap Information](#agent-bootstrap-information-pma-and-other-managed-agents) required for that agent to start and connect to inference and the orchestrator.

Draft additions (shape sketch; exact field names TBD in spec review):

- **Node configuration** adds `managed_services` (object).
  - `managed_services.services` (array of objects, required when managed services are supported).
    Each entry is a desired-state declaration for one managed service instance to run on this node.
    Required fields (draft):
    - `service_id` (string; stable id assigned by orchestrator)
    - `service_type` (string; e.g. `pma`, `paa`)
    - `image` (string; OCI reference)
    - `args` (array of strings; optional)
    - `env` (object; includes orchestrator endpoints and service-specific config; for agents, MUST include inference and MCP connectivity per bootstrap section)
    - `inference` (object; required for agent types; structure per [Agent Bootstrap Information - Inference Connectivity](#inference-connectivity-required): `mode`, `base_url` or `api_egress_base_url`, optional `default_model`, `warmup_required`)
    - `orchestrator` (object; required for agent types; per bootstrap: worker-proxy URL for MCP gateway and for ready callback, token or token reference; agent MUST NOT receive raw orchestrator host/port)
    - `healthcheck` (object; at minimum path and expected 200)
    - `restart_policy` (string; `always` recommended for system services)
    - `network` (object; how the service is exposed through the worker)
    - `resources` (object; optional constraints such as cpu/memory/gpu access)
    - `routing` (object; how orchestrator and other services reach it; MUST be worker-mediated by default)

For `service_type=pma`, the entry MUST also include `role: project_manager` and the full PMA bootstrap set (inference, orchestrator, identity, listen, optional instructions/preferences) so PMA has everything needed to start and connect to inference (node-local, external, or another node) and to the MCP gateway.

- **Capability report / status** adds `managed_services_status` (object).
  - `managed_services_status.services` (array of objects; observed state), with at minimum:
    - `service_id` (string)
    - `service_type` (string)
    - `state` (enum; `stopped`|`starting`|`ready`|`unhealthy`|`error`)
    - `ready_at` (timestamp; optional)
    - `endpoints` (array of strings; orchestrator-callable, worker-mediated)
    - `last_error` (string; bounded; optional)
    - `container_id` (string; optional)
    - `image` (string; effective image running)
    - `restart_count` (number; optional)
    - `observed_generation` (string or number; echoes desired state generation/version)

Key requirement: the **endpoint MUST be a worker-mediated endpoint**, not `http://localhost:8090` or any
other host assumption.

## Orchestrator Tracking of Managed Services

The orchestrator MUST treat managed services as a first-class tracked resource, so that:

- The user-gateway can route requests (e.g. `model=cynodeai.pm`) to the current PMA endpoint.
- The orchestrator can detect service loss and drive remediation (reconcile desired state).
- Operators can query which services are running where and their health.

Draft tracking model (conceptual, not schema-locked):

- **ManagedService**:
  - `service_id`
  - `service_type`
  - `desired_placement` (node selection constraints)
  - `desired_spec` (image/env/health/routing)
  - `desired_generation` (monotonic)
  - `created_at`, `updated_at`

- **ManagedServiceInstance** (service_id + node_id):
  - `node_id`
  - `observed_state`
  - `observed_generation`
  - `endpoints`
  - `last_heartbeat_at`
  - `last_error`

Normative behavior:

- The orchestrator MUST persist desired state and the last observed state for managed services.
- The orchestrator MUST expose the effective endpoint(s) for services it depends on (at minimum PMA) to internal
  components such as user-gateway.
- The orchestrator MUST consider the PMA service "online" only when it has a recent worker report with
  `state=ready` for that service instance.

## Reconciliation and Drift Handling

The system must handle drift between desired state and observed state.

Worker requirements (normative):

- The worker MUST reconcile periodically and on config update:
  - If the desired service is missing, create/start it.
  - If the desired service image/spec differs, replace it according to the update policy (stop old, start new).
  - If the service is unhealthy or exited, restart it per restart policy and backoff.
- The worker MUST report observed state changes promptly (not just at registration time).

Orchestrator requirements (normative):

- The orchestrator MUST maintain a reconciliation loop that:
  - Detects missing or stale service instance reports (heartbeat timeout).
  - Re-issues desired state to the node when needed.
  - Re-places the service onto a different node when the current node is unavailable.
- The orchestrator MUST support rolling updates by increasing desired_generation and updating node config.

## Readiness Semantics

This proposal changes readiness semantics in a specific way:

- Orchestrator readiness depends on PMA being ready, but "PMA ready" is determined by
  **worker-reported PMA ready state**, not orchestrator-side direct probing of `PMA_LISTEN_ADDR`.
- PMA is always required; orchestrator readiness MUST NOT proceed without PMA.

The worker must not report PMA ready until its health endpoint has responded successfully per the health contract.

## Routing Semantics: Worker Proxy Bidirectional

All traffic between PMA (and other managed agents) and the orchestrator MUST go through the **worker proxy**.

### Orchestrator to Agent (Inbound to Agent)

- The user-gateway route for `model=cynodeai.pm` MUST route to PMA using the orchestrator's current PMA endpoint, which MUST be a worker-mediated endpoint (e.g. Worker API reverse proxy to the PMA container).
- The user-gateway MUST NOT hardcode `http://cynode-pma:8090` (compose DNS), and MUST NOT assume PMA is on the same network as user-gateway.
- Concrete model: orchestrator (or user-gateway) calls Worker API at e.g. `http(s)://<worker_api_base>/internal/pma/...`; Worker API proxies that request to the PMA container.

### Agent to Orchestrator (Outbound From Agent)

- The worker proxy MUST also handle **agent-to-orchestrator** traffic.
  The agent MUST be configured with a worker-local URL for the MCP gateway and for any ready/callback endpoints; the worker proxy forwards those requests to the real orchestrator (MCP gateway, control-plane, etc.).
- The agent container MUST NOT be given direct orchestrator hostnames or ports.
  All outbound calls from the agent to the orchestrator (MCP tool calls, ready notification, etc.) go to the worker proxy, which forwards them to the orchestrator.
- Benefits: single egress point, consistent audit and policy at the worker, no requirement for the agent container to resolve or reach orchestrator networks.

Normative:

- The Worker API (or dedicated worker proxy component) MUST implement both directions: proxy requests from orchestrator to managed agents, and proxy requests from managed agents to the orchestrator.
- The exact paths (e.g. `/internal/pma/*`, `/internal/agent/orchestrator/mcp`, `/internal/agent/orchestrator/ready`) are to be specified and traced to the worker API and gateway specs.

## Lifecycle Management and Restart

Worker responsibilities (normative):

- When instructed to run PMA, the worker MUST ensure PMA is running, restarting it if it exits.
- The worker MUST implement a backoff strategy on repeated failures and report errors to the orchestrator.
- The worker MUST report transitions (`starting` -> `ready`, `ready` -> `unhealthy`, etc.) to the orchestrator
  promptly enough for readiness and routing correctness.

Orchestrator responsibilities (normative):

- The orchestrator MUST be able to re-issue the PMA start bundle on configuration refresh.
- The orchestrator MUST tolerate transient PMA unavailability and reflect that in readiness and routing.

## Security and Trust Boundary

This proposal changes (or clarifies) the trust boundary:

- PMA is not a per-task sandbox container.
- PMA is a node-managed service container, and must be treated as a control-plane-adjacent component.

Normative constraints:

- The worker MUST enforce that the PMA container cannot access secrets that are forbidden to agents and sandboxes.
  API keys must still be routed via API Egress, and DB access must still be via MCP tools.
- The orchestrator MUST authenticate/authorize all PMA traffic; requests to PMA MUST flow through orchestrator
  controls (logging, sanitization, persistence).

## Spec and Requirements Changes to Draft

This section lists concrete edits to propose in canonical docs.
These are draft notes for review; exact IDs
may need adjustment to match the requirements index.

### Bootstrap Spec: `docs/tech_specs/orchestrator_bootstrap.md`

Proposed edits:

- Update "Orchestrator Independent Startup" to remove "cynode-pma when enabled" as a core orchestrator-stack service.
  Replace with: "PMA service is node-managed; orchestrator starts without PMA and instructs a node to run PMA when
  an inference path exists."
- Update "PMA Startup" to explicitly state the mechanism: orchestrator sends PMA start bundle to worker via node config.
- Update "PMA Informs Orchestrator" to require worker-reported readiness and endpoint.

### Worker Node Spec: `docs/tech_specs/worker_node.md`

Proposed additions:

- Extend Node Startup Procedure to include: "apply managed services instructions (PMA) after config ack."
- Add a new section: "Managed Service Containers" with PMA as the first required managed service.
- Add a new section (or extend Worker API spec): **Worker proxy bidirectional.**
  The Worker API MUST expose proxy routes so that (1) the orchestrator can reach managed agents (e.g. PMA) via the Worker API, and (2) managed agents can reach the orchestrator (MCP gateway, ready callback) via the Worker API.
  The agent MUST NOT be given direct orchestrator URLs; the worker proxy is the single egress for agent-to-orchestrator traffic.
- Ensure this spec explicitly treats node-local agent runtimes (low-latency path) as a managed-services use case when applicable,
  aligned with the capability-lease requirements and audit expectations.

### Payload Spec: `docs/tech_specs/worker_node_payloads.md`

Proposed additions:

- Add desired state: `managed_services.services[]` to node configuration payload.
- Add observed state: `managed_services_status.services[]` to capability/status reporting.
- Include service identity (`service_id`, `service_type`) and desired/observed generation fields for reconciliation.

### PMA Spec: `docs/tech_specs/cynode_pma.md`

This doc currently asserts PMA is orchestrator-side.

Proposed edits:

- Reframe PMA as "orchestrator-owned runtime hosted as a worker-managed service container" rather than as an
  orchestrator-local process.
- Update request boundary language to reflect worker-mediated endpoints.
- Update tool-access boundary language to align with edge enforcement mode when PMA is node-local (capability leases, fail-closed,
  and node audit record forwarding).

### MCP Gateway and Tooling Specs: `docs/tech_specs/mcp_gateway_enforcement.md`, `docs/tech_specs/mcp_tooling.md`

Proposed edits:

- Explicitly call out long-running managed agent runtimes (such as node-hosted PMA) as a primary consumer of edge enforcement mode.
- Ensure the capability lease shape and minimum audit fields are sufficient to attribute tool calls made by managed services.

### Worker Requirements: `docs/requirements/worker.md`

Proposed new requirements (draft):

- **REQ-WORKER-PMA-0001:** When instructed by orchestrator configuration, the node MUST start `cynode-pma` as a
  managed service container and keep it running (restart on exit).
- **REQ-WORKER-PMA-0002:** The node MUST report PMA service status and an orchestrator-callable endpoint to the
  orchestrator, and MUST keep it updated.
- **REQ-WORKER-PMA-0003:** The node MUST proxy all traffic between the orchestrator and PMA (and other managed agents) through the Worker API: orchestrator-to-agent (e.g. chat handoff) and agent-to-orchestrator (e.g. MCP tool calls, ready callbacks).
  The agent MUST NOT connect directly to orchestrator hostnames or ports.

### Orchestrator Requirements: `docs/requirements/orches.md`

Proposed new requirements (draft):

- **REQ-ORCHES-PMA-0001:** The orchestrator MUST instruct a worker to start PMA by delivering a PMA start bundle
  only after the first inference path exists (or external PMA inference is configured).
- **REQ-ORCHES-PMA-0002:** The orchestrator MUST treat PMA as online only after the worker reports PMA ready.
- **REQ-ORCHES-PMA-0003:** The orchestrator MUST provide a stable PMA endpoint for user-gateway routing that flows
  through the worker node.

## Test Implications

E2E/BDD expectations (normative once specs are updated):

- E2E should assert PMA is **not running** before the worker reports ready/inference-capable.
- E2E should assert PMA becomes available only after orchestrator instructs the worker to start it.
- E2E should assert `model=cynodeai.pm` routing works through the worker-mediated endpoint (not compose DNS).
- E2E should include a restart test: stop PMA container and ensure worker restarts it and orchestrator routing recovers.

## Resolved Decisions (From Review)

- **No orchestrator-local PMA fallback**
  - The orchestrator MUST NOT run PMA locally as a subprocess as a fallback.
  - Worker-managed PMA is required; PMA is always required.
- **Worker proxy bidirectional**
  - All PMA (and other managed agent) traffic MUST use the worker proxy in both directions: orchestrator to agent and agent to orchestrator.
    The worker proxy handles MCP, callbacks, and chat handoff; the agent does not connect directly to the orchestrator.
- **PMA inference connectivity**
  - PMA MAY reach inference either via a node-local inference service address or via API Egress.
  - The effective method MUST be specified by the orchestrator in the PMA start bundle / node configuration.
- **Multi-node PMA placement**
  - The orchestrator picks the first available eligible node to host PMA.
  - Post-MVP, the orchestrator SHOULD periodically evaluate migrating PMA to a more capable node.
