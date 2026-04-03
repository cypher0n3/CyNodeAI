# Worker Node Technical Spec

- [Document Overview](#document-overview)
- [Node Manager](#node-manager)
  - [Node Manager Shutdown](#node-manager-shutdown)
  - [Orchestrator Shutdown Notification](#orchestrator-shutdown-notification)
- [Managed Service Containers](#managed-service-containers)
  - [NATS Connection and Chat Bridge for Managed Services](#nats-connection-and-chat-bridge-for-managed-services)
- [Worker Proxy Bidirectional (Managed Agents)](#worker-proxy-bidirectional-managed-agents)
  - [Agent Network Restriction (Security Boundary)](#agent-network-restriction-security-boundary)
  - [Worker Proxy Normative Behavior](#worker-proxy-normative-behavior)
- [Token and Credential Handling](#token-and-credential-handling)
  - [Token Authentication and Auditing](#token-authentication-and-auditing)
  - [Agent Token Storage and Lifecycle](#agent-token-storage-and-lifecycle)
- [Sandbox Control Plane](#sandbox-control-plane)
  - [Sandbox Workspace and Job Mounts](#sandbox-workspace-and-job-mounts)
  - [Sandbox Rootless Execution](#sandbox-rootless-execution)
  - [Sandbox Control Plane Applicable Requirements](#sandbox-control-plane-applicable-requirements)
- [Unified UDS Path (Agent and Sandbox Containers)](#unified-uds-path-agent-and-sandbox-containers)
  - [Unified UDS Path (Agent and Sandbox Containers) Requirements Traces](#unified-uds-path-agent-and-sandbox-containers-requirements-traces)
- [Node-Local Inference and Sandbox Workflow](#node-local-inference-and-sandbox-workflow)
  - [Node-Local Inference Applicable Requirements](#node-local-inference-applicable-requirements)
  - [SBA Inference Proxy Capture and Reporting](#sba-inference-proxy-capture-and-reporting)
- [Node Sandbox MCP Exposure](#node-sandbox-mcp-exposure)
  - [Node Sandbox MCP Exposure Applicable Requirements](#node-sandbox-mcp-exposure-applicable-requirements)
  - [Node-Local Agent Sandbox Control (Low-Latency Path)](#node-local-agent-sandbox-control-low-latency-path)
- [Node Startup YAML](#node-startup-yaml)
  - [Node Startup YAML Applicable Requirements](#node-startup-yaml-applicable-requirements)
  - [User-Configurable Properties](#user-configurable-properties)
- [Node Startup Procedure](#node-startup-procedure)
  - [Node Startup Procedure Requirements Traces](#node-startup-procedure-requirements-traces)
- [Node Startup Checks and Readiness](#node-startup-checks-and-readiness)
  - [Node Startup Checks and Readiness Requirements Traces](#node-startup-checks-and-readiness-requirements-traces)
- [Deployment and Auto-Start](#deployment-and-auto-start)
  - [Deployment and Auto-Start Requirements Traces](#deployment-and-auto-start-requirements-traces)
- [Deployment Topologies](#deployment-topologies)
  - [Deployment Topologies Requirements Traces](#deployment-topologies-requirements-traces)
- [Single-Process Host Binary](#single-process-host-binary)
  - [Single-Process Host Binary Requirements Traces](#single-process-host-binary-requirements-traces)
  - [Single-Process Host Binary Scope](#single-process-host-binary-scope)
  - [Single-Process Host Binary Preconditions](#single-process-host-binary-preconditions)
  - [Single-Process Host Binary Outcomes](#single-process-host-binary-outcomes)
  - [`SingleProcessHostBinary` Algorithm](#singleprocesshostbinary-algorithm)
  - [Single-Process Host Binary Error Conditions](#single-process-host-binary-error-conditions)
  - [Single-Process Host Binary Observability](#single-process-host-binary-observability)
  - [Binary Name and Invocation (Informational)](#binary-name-and-invocation-informational)
- [Existing Inference Service on Host](#existing-inference-service-on-host)
  - [Existing Inference Service on Host Requirements Traces](#existing-inference-service-on-host-requirements-traces)
- [Ollama Container Policy](#ollama-container-policy)
- [Sandbox-Only Nodes](#sandbox-only-nodes)
  - [Sandbox-Only Nodes Applicable Requirements](#sandbox-only-nodes-applicable-requirements)
- [Registration and Bootstrap](#registration-and-bootstrap)
  - [Orchestrator Registration Retry](#orchestrator-registration-retry)
- [Capability Reporting](#capability-reporting)
  - [Capability Reporting Requirements Traces](#capability-reporting-requirements-traces)
- [Configuration Delivery](#configuration-delivery)
  - [Configuration Delivery Requirements Traces](#configuration-delivery-requirements-traces)
- [Dynamic Configuration Updates](#dynamic-configuration-updates)
- [Credential Handling](#credential-handling)
  - [Credential Handling Applicable Requirements](#credential-handling-applicable-requirements)
  - [Node-Local Secure Store](#node-local-secure-store)
  - [Secure Store Process Boundary](#secure-store-process-boundary)
- [Required Node Configuration](#required-node-configuration)

## Document Overview

This document defines worker node responsibilities, including node registration, configuration bootstrap, and secure credential handling.
Nodes are configured by the orchestrator to access orchestrator-provided services such as the rank-ordered sandbox image registry list and model cache.
When the orchestrator is not yet available at worker startup, registration retries follow [Orchestrator Registration Retry](#orchestrator-registration-retry).

## Node Manager

The Node Manager is a host-level system service responsible for:

- Starting and stopping worker services (worker API, Ollama, sandbox containers).
- Managing container runtime (Docker or Podman) lifecycle for sandbox execution.
  Podman MUST be supported and MUST be the default runtime for sandbox execution.
  Docker MAY be supported as an alternative runtime.
- When the runtime supports rootless execution (e.g. Podman), the node MUST use rootless operations for sandbox containers unless overridden by the operator; see [Sandbox rootless execution](#sandbox-rootless-execution).
- Receiving configuration updates from the orchestrator and applying them locally.
- Managing local secure storage for pull credentials and certificates.
- Starting, supervising, and restarting orchestrator-directed managed service containers (for example PMA).

### Node Manager Shutdown

- Spec ID: `CYNAI.WORKER.NodeManagerShutdown` <a id="spec-cynai-worker-nodemanagershutdown"></a>

#### Traces to Requirements

- [REQ-WORKER-0267](../requirements/worker.md#req-worker-0267)

When the Node Manager receives a shutdown command (e.g. SIGTERM, SIGINT, or systemd stop), it MUST:

1. Send shutdown (stop) commands to all containers it is running: managed service containers (e.g. Ollama, PMA) and any sandbox containers still under its control.
2. Allow a configurable grace period for containers to stop gracefully (e.g. runtime stop with timeout).
3. If a container does not stop within the grace period, the Node Manager MUST force-stop or kill it (per runtime semantics) and continue shutdown.
4. After attempting to stop all dependent containers, the Node Manager MUST exit.
5. If any dependent container failed to shut down (e.g. did not exit cleanly within the grace period, or force-stop failed), the Node Manager MUST exit with a non-zero exit code.
   If all containers shut down successfully, the Node Manager MUST exit with exit code zero.

Shutdown and its outcome (success or failure, including which containers failed) MUST be recorded in the node telemetry database per [CYNAI.WORKER.TelemetryLifecycleEvents](worker_telemetry_api.md#spec-cynai-worker-telemetrylifecycleevents).

### Orchestrator Shutdown Notification

- Spec ID: `CYNAI.WORKER.OrchestratorShutdownNotification` <a id="spec-cynai-worker-orchestratorshutdownnotification"></a>

#### Orchestrator Shutdown Notification Requirements Traces

- [REQ-WORKER-0271](../requirements/worker.md#req-worker-0271)
- [REQ-ORCHES-0164](../requirements/orches.md#req-orches-0164)

When the worker receives an orchestrator-initiated notification to stop all orchestrator-directed agents and jobs (e.g. via the Worker API contract defined in [CYNAI.WORKER.StopAllOrchestratorDirected](worker_api.md#spec-cynai-worker-stopallorchestratordirected)), the worker MUST:

1. Stop all orchestrator-directed managed service containers (including PMA and any other managed agents).
2. Stop or cancel all jobs that were dispatched by the orchestrator and are still running (sandbox containers, session sandboxes, or in-progress job execution).

The worker MUST honor this notification regardless of local Node Manager lifecycle (i.e. even if the Node Manager is not itself shutting down).
Outcome (success or failure, and which resources were stopped) SHOULD be recorded in the node telemetry database per [CYNAI.WORKER.TelemetryLifecycleEvents](worker_telemetry_api.md#spec-cynai-worker-telemetrylifecycleevents).

## Managed Service Containers

- Spec ID: `CYNAI.WORKER.ManagedServiceContainers` <a id="spec-cynai-worker-managedservicecontainers"></a>

This section defines worker-managed service containers directed by the orchestrator.
Managed services are long-lived containers that are part of the system control plane and are distinct from per-job sandbox containers.

Normative behavior:

- The worker MUST support orchestrator-directed managed services via the node configuration payload.
  See [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md) `node_configuration_payload_v1` `managed_services`.
- The worker MUST treat managed services as desired state.
  When a desired service is present in configuration, the worker MUST converge to the desired state:
  - If missing, create and start it.
  - If running with a different spec (image/env/args), update it per the rollout policy (stop old, start new).
  - If exited or unhealthy, restart it per `restart_policy` with backoff.
- The worker MUST report managed service observed state to the orchestrator.
  See [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md) `node_capability_report_v1` `managed_services_status`.
- The worker MUST NOT treat managed service containers as sandbox containers.
  Managed services may be privileged relative to sandbox workloads, but must still comply with system security boundaries.
- The worker MUST start managed service containers (agent runtimes) with network restriction so that all inbound and outbound traffic routes through worker proxies via **UDS-only** proxy endpoints (no TCP to agents); see [Worker Proxy Bidirectional (Managed Agents)](#worker-proxy-bidirectional-managed-agents), [Unified UDS Path](#unified-uds-path-agent-and-sandbox-containers), and [REQ-WORKER-0174](../requirements/worker.md#req-worker-0174).
- When a managed service declares `inference.mode=node_local` and the configuration includes `inference.backend_env`, the worker MUST pass those backend environment values into the managed service container.
- When the same node configuration also includes `inference_backend.env` for the local inference backend, the worker MUST keep the effective backend-derived values aligned between the backend container and managed services that depend on that backend so they use the same orchestrator-derived context-window and runner settings.

PMA as managed service (normative):

- PMA is a core system feature and is always required.
- The orchestrator MUST instruct a worker to run **one or more** PMA managed service instances (`pma-pool-*` warm pool per [CYNAI.ORCHES.PmaWarmPool](orchestrator_bootstrap.md#spec-cynai-orches-pmawarmpool), slots assigned per session binding per [CYNAI.ORCHES.PmaInstancePerSessionBinding](orchestrator_bootstrap.md#spec-cynai-orches-pmainstancepersessionbinding)); each entry has a distinct `service_id`.
- The worker MUST start, supervise, and keep each configured PMA instance running per desired state (see [REQ-WORKER-0176](../requirements/worker.md#req-worker-0176)).

### NATS Connection and Chat Bridge for Managed Services

- Spec ID: `CYNAI.WORKER.NatsChatBridge` <a id="spec-cynai-worker-natschatbridge"></a>

The worker authenticates with the orchestrator via HTTP(S) during registration, receives a NATS JWT from the orchestrator as part of the bootstrap response, and then connects to NATS for all subsequent real-time communication (chat streaming, session activity, config notifications).
PMA remains UDS-only (per [CYNAI.WORKER.AgentNetworkRestriction](#spec-cynai-worker-agentnetworkrestriction)); the worker is the NATS boundary.
See [`docs/tech_specs/nats_messaging.md`](nats_messaging.md) for the full subject taxonomy and payload schemas.

#### NATS Connection Lifecycle

- During [registration and bootstrap](#spec-cynai-worker-registrationandbootstrap), the worker authenticates with the orchestrator via HTTP(S) using the pre-shared key (existing flow).
- The orchestrator returns a `nats` configuration block in the bootstrap payload containing the server URL, a node-scoped NATS JWT, JWT expiry, and optional TLS/subject overrides.
  See [`node_bootstrap_payload_v1`](worker_node_payloads.md#spec-cynai-worker-payload-bootstrap-v1) for the full schema and [NATS Authentication and Credentials](nats_messaging.md#spec-cynai-usrgwy-natsclientcredentials) for the credential model.
- The worker connects to NATS using the provided URL and JWT.
  The `NATS_URL` environment variable is accepted as a fallback only when the bootstrap payload omits the `nats` block (e.g. legacy orchestrator).
- After NATS connection, the worker uses NATS for chat bridging, config notifications, and session activity relay.
  HTTP(S) to the orchestrator is retained only for registration, config fetch, and capability reporting.
- If the NATS connection drops, the worker reconnects with bounded backoff.
  The HTTP-based config poll remains as a fallback for NATS downtime.
- On NATS JWT expiry or rotation, the worker requests a refreshed JWT from the orchestrator via HTTP and reconnects.

#### Chat Bridge Behavior

The bridge preserves token-by-token streaming: each token delta produced by PMA becomes a discrete NATS message delivered to the client in real-time.
Clients subscribe to `cynode.chat.stream.<session_id>.>` and `cynode.chat.done.<session_id>.>` (permissions granted by their session JWT) and see tokens arrive with the same granularity as the current HTTP/SSE path.

- For each managed PMA instance with an active session binding, the worker subscribes to `cynode.chat.request.<session_id>`.
- On receipt of a `chat.request` NATS message, the worker:
  1. Extracts the chat completion payload.
  2. Forwards the request to PMA via the existing UDS HTTP proxy (`POST /internal/chat/completion` with `stream: true`).
  3. Reads the NDJSON token stream from PMA incrementally -- one token delta per NDJSON line.
  4. As each token delta arrives, immediately publishes it as a `chat.stream` message to `cynode.chat.stream.<session_id>.<message_id>`.
     The client receives each `chat.stream` message as soon as the NATS server delivers it (typically sub-millisecond after publish).
  5. On stream completion, publishes `chat.done` to `cynode.chat.done.<session_id>.<message_id>`.
- If PMA returns an error, the worker publishes an error event on `chat.done` with error details.
- The worker MUST NOT buffer or batch token deltas; each delta is published as its own NATS message the instant it is read from UDS.
- When a managed PMA instance is stopped or removed, the worker unsubscribes from the corresponding `chat.request` subject.

#### NATS Chat Bridge Requirements Traces

- [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)
- [REQ-WORKER-0176](../requirements/worker.md#req-worker-0176)

## Worker Proxy Bidirectional (Managed Agents)

- Spec ID: `CYNAI.WORKER.WorkerProxyBidirectionalManagedAgents` <a id="spec-cynai-worker-proxybidirectional"></a>

Whenever an agent runtime (PMA, PAA, SBA, or other managed agent) runs on a worker, it MUST communicate with the orchestrator through the worker proxy in both directions.
The agent MUST NOT be given direct orchestrator URLs or network access; all traffic flows through the worker proxy.

### Agent Network Restriction (Security Boundary)

- Spec ID: `CYNAI.WORKER.AgentNetworkRestriction` <a id="spec-cynai-worker-agentnetworkrestriction"></a>

Traces To: [REQ-WORKER-0174](../requirements/worker.md#req-worker-0174).

All agent runtimes on a worker (whether running as a managed service or not, including PMA, PAA, SBA, and any other agent) MUST be network restricted.
All inbound and outbound traffic to or from those agents MUST route through worker proxies; there MUST be no direct network path that bypasses the worker proxy.
Violating this violates a security boundary and is not acceptable.
Managed service containers (e.g. PMA, PAA) MUST be started with network restriction so that they have no network path except to the worker proxy, and that path MUST be via UDS only (no TCP, including no loopback TCP, to proxy endpoints).
The worker MUST NOT start agent containers with unrestricted network access.

### Worker Proxy Normative Behavior

All proxy endpoints that the worker exposes to any local agent (managed service or sandbox) MUST be UDS-only: containers receive `http+unix://` URLs or socket paths, never TCP host:port.

- **Orchestrator to agent:** The worker MUST expose a worker-mediated endpoint (via Worker API reverse proxy) that the orchestrator
  (and user-gateway, when applicable) can call to reach the managed agent container (e.g. PMA chat handoff and health).
- **Agent to orchestrator:** The worker MUST expose worker-local proxy endpoints that the managed agent uses to call:
  - the orchestrator MCP gateway (for tool calls), and
  - any orchestrator callback/ready endpoints.
- The worker proxy forwards those requests to the orchestrator.
- The agent container MUST reach these proxy endpoints only via UDS (e.g. per [Agent-To-Orchestrator UDS Binding](#agent-to-orchestrator-uds-binding-required)); the worker MUST NOT inject TCP URLs.
- The managed agent container MUST NOT be configured to call orchestrator hostnames or ports directly.
  All agent-to-orchestrator traffic flows through the worker proxy.

## Token and Credential Handling

- Spec ID: `CYNAI.WORKER.AgentTokensWorkerHeldOnly` <a id="spec-cynai-worker-agenttokensworkerheldonly"></a>

- **Agents MUST NOT be given tokens or secrets directly.**
  The worker proxy MUST hold orchestrator-issued credentials (agent tokens, capability leases) and MUST attach the appropriate credential when forwarding agent-originated requests to the orchestrator.
  The worker MUST NOT pass agent tokens or other orchestrator-issued secrets into agent containers or to agents; the agent calls the worker proxy (e.g. worker-proxy URL for MCP), and the worker proxy adds the token when forwarding to the gateway.
  For managed agent internal proxy calls, this document specifies agent-token handling only.
  Capability leases (for example for the node-local sandbox control path) are out of scope for the managed agent internal proxy token lifecycle defined below.

Traces To: [REQ-WORKER-0164](../requirements/worker.md#req-worker-0164).

### Token Authentication and Auditing

- The worker MUST authenticate and authorize proxy requests according to orchestrator-issued credentials (agent tokens, capability leases) that the **worker** holds and MUST fail closed when validation fails.
- The worker MUST emit auditable records for proxy activity sufficient to attribute actions to the agent identity and context.

### Agent Token Storage and Lifecycle

- Spec ID: `CYNAI.WORKER.AgentTokenStorageAndLifecycle` <a id="spec-cynai-worker-agenttokenstorageandlifecycle"></a>

#### Agent Token Storage and Lifecycle Requirements Traces

- [REQ-WORKER-0164](../requirements/worker.md#req-worker-0164)
- [REQ-WORKER-0175](../requirements/worker.md#req-worker-0175)
- [REQ-WORKER-0176](../requirements/worker.md#req-worker-0176)
- [REQ-WORKER-0165](../requirements/worker.md#req-worker-0165)
- [REQ-WORKER-0166](../requirements/worker.md#req-worker-0166)
- [REQ-WORKER-0167](../requirements/worker.md#req-worker-0167)
- [REQ-WORKER-0168](../requirements/worker.md#req-worker-0168)

The worker MUST store agent tokens in the node-local secure store defined by [CYNAI.WORKER.NodeLocalSecureStore](#spec-cynai-worker-nodelocalsecurestore) and MUST NOT pass agent tokens into any agent container (including managed-service containers such as PMA).

Required behavior:

- The worker MUST key agent tokens by the managed-service identity (e.g. `service_id`) so the worker proxy can deterministically select the correct token for the calling agent runtime.
- For **PMA**, when the orchestrator delivers **per-user session** MCP credentials, the worker MUST key stored credentials so the proxy can attach the credential for the calling instance: with **one PMA instance per session binding**, the worker MAY use **`service_id` alone** (one credential binding per instance).
  See [CYNAI.ORCHES.PmaInstancePerSessionBinding](orchestrator_bootstrap.md#spec-cynai-orches-pmainstancepersessionbinding), [CYNAI.MCPGAT.PmaSessionTokens](mcp/mcp_gateway_enforcement.md#spec-cynai-mcpgat-pmasessiontokens), and [CYNAI.MCPGAT.PmaInvocationClass](mcp/mcp_gateway_enforcement.md#spec-cynai-mcpgat-pmainvocationclass).
- The worker proxy MUST attach the correct agent token to agent-originated requests when forwarding to the orchestrator.
- The worker MUST NOT expose agent tokens to sandboxes or agents via env vars, files, mounts, or logs.
- For **job-scoped (SBA) tokens**, the orchestrator invalidates the token when the job is stopped or canceled; the worker MUST NOT use an invalidated token to forward requests.
  See [Task Cancel and Stop Job](orchestrator.md#spec-cynai-orches-taskcancelandstopjob).

#### Agent Token Observability

- Agent tokens MUST NOT appear in logs, metrics, audit payloads (beyond opaque identifiers such as `service_id` or agent identity), debug endpoints, or telemetry responses.
  Redaction MUST NOT be relied upon.

#### Agent Token Ref Resolution

- Spec ID: `CYNAI.WORKER.AgentTokenRefResolution` <a id="spec-cynai-worker-agenttokenrefresolution"></a>

This section defines how the worker resolves `managed_services.services[].orchestrator.agent_token_ref` into an agent token.

##### Agent Token Ref Resolution Requirements Traces

- [REQ-WORKER-0164](../requirements/worker.md#req-worker-0164)

Required behavior:

- The worker MUST support `agent_token_ref` as specified in [CYNAI.WORKER.Payload.AgentTokenRef](worker_node_payloads.md#spec-cynai-worker-payload-agenttokenref).
- The worker MUST resolve `agent_token_ref` during configuration apply.
- Resolution failures MUST fail closed.
  The worker MUST treat the service token as missing and MUST NOT forward any agent-originated requests for that `service_id`.
- The worker MUST NOT pass the reference object or resolved token material to any managed service container or agent runtime.

`kind=orchestrator_endpoint` contract:

- The worker MUST perform an HTTP `POST` request to `agent_token_ref.url`.
- The worker MUST use `Content-Type: application/json`.
- The request body MUST be a JSON object with fields:
  - `node_slug` (string)
  - `service_id` (string)
  - `service_type` (string)
  - `role` (string, optional)
- The response body MUST be a JSON object with fields:
  - `agent_token` (string)
  - `agent_token_expires_at` (string, optional, RFC 3339 UTC timestamp)
- Non-2xx responses, invalid JSON, missing `agent_token`, or an invalid `agent_token_expires_at` value MUST be treated as resolution failures.
- The worker MUST treat the response body as secret material.
  The worker MUST NOT log it and MUST NOT expose it via metrics, telemetry, or debug endpoints.

#### Agent-To-Orchestrator UDS Binding (Required)

This section defines the required identity-binding mechanism for managed agent internal proxy calls.
It makes the managed agent identity (`service_id`) derivable from the connection binding without relying on secrets inside the agent container or request.

Host-side socket layout:

- The worker MUST create a per-service directory under the effective node state directory:
  - Base directory: `${storage.state_dir}/run/managed_agent_proxy/` when `storage.state_dir` is set.
  - Base directory: `/var/lib/cynode/state/run/managed_agent_proxy/` when `storage.state_dir` is unset.
- For each managed agent runtime instance, the worker MUST create:
  - Directory: `<base>/<service_id>/` with permissions `0700`.
  - Socket file: `<base>/<service_id>/proxy.sock` with permissions `0600`.
- The worker MUST ensure the directory and socket are owned by the worker / Node Manager user.
- The worker MUST NOT place these sockets under the secure store path (`${storage.state_dir}/secrets/`).

Container-side mount and path:

- The worker MUST mount the per-service host directory `<base>/<service_id>/` into the managed service container at:
  - Container path: `/run/cynode/managed_agent_proxy/` (directory).
- The managed agent runtime MUST use the socket path:
  - `/run/cynode/managed_agent_proxy/proxy.sock`
- The worker MUST mount only the calling service's UDS directory into that container.
  No other managed service container MUST receive this mount.
- The worker MUST mount the directory read-write for the duration of the container lifetime.
  The agent runtime is the client and the worker is the server, so the agent only needs connect permissions, but read-write mount avoids runtime-specific socket permission edge cases.

HTTP binding:

- The worker internal proxy server MUST serve HTTP over this UDS.
- The worker MUST resolve the calling `service_id` from which UDS listener accepted the connection (socket identity), not from request headers.
- This document does not specify `per_service_loopback_listener` binding for managed agent internal proxy identity.

Container runtime mount options (minimum):

- The mount MUST be a bind mount of the host directory into the container.
- The mount MUST NOT be propagated to other containers.
- For rootless runtimes, the worker MUST ensure the host path is accessible to the container runtime user namespace without relaxing permissions beyond those specified above.

#### `AgentTokenStorageAndLifecycle` Algorithm

<a id="algo-cynai-worker-agenttokenstorageandlifecycle"></a>

1. On configuration apply, for each `managed_services.services[]` entry that includes `orchestrator.agent_token` or `agent_token_ref`, the worker resolves the token value and writes it to the node-local secure store under the key for that service identity. <a id="algo-cynai-worker-agenttokenstorageandlifecycle-step-1"></a>
   If `agent_token_ref` is present, the worker MUST resolve it per [CYNAI.WORKER.AgentTokenRefResolution](#spec-cynai-worker-agenttokenrefresolution).
   Resolution failures MUST fail closed.
2. The worker MUST NOT pass the token value to the managed-service container or agent runtime. <a id="algo-cynai-worker-agenttokenstorageandlifecycle-step-2"></a>
3. When the worker proxy receives an agent-originated request, it determines the calling service identity, loads the corresponding token from the secure store (for PMA, selecting the **session-bound** credential when per-user session tokens are in use), attaches it to the outbound request, and forwards to the orchestrator. <a id="algo-cynai-worker-agenttokenstorageandlifecycle-step-3"></a>
   The worker MUST determine the calling service identity without relying on any secret in the agent container or request.
   The worker MUST achieve this using an identity-bound per-service internal proxy binding.
   The required mechanism is per-service Unix domain sockets:
   - For each `service_id`, the worker creates a dedicated UDS listener for agent-to-orchestrator internal proxy operations.
   - The worker mounts only that service's UDS into the corresponding managed service container (no other managed service container receives that mount).
   - The worker resolves the calling `service_id` from the specific UDS listener that accepted the connection.
   Unknown or ambiguous caller identities MUST fail closed.
4. On configuration update or service removal, the worker removes or overwrites the stored token for that service identity so the old token is no longer available. <a id="algo-cynai-worker-agenttokenstorageandlifecycle-step-4"></a>
5. When an expiry is provided (e.g. `managed_services.services[].orchestrator.agent_token_expires_at` in [CYNAI.WORKER.Payload.ConfigurationV1](worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1)), the worker MUST treat expired tokens as invalid and MUST NOT use them to forward requests; the worker SHOULD request a configuration refresh where applicable. <a id="algo-cynai-worker-agenttokenstorageandlifecycle-step-5"></a>

## Sandbox Control Plane

This section defines how agents and the orchestrator interact with sandbox containers on a node.
Agents do not connect to sandboxes directly over the network.
Outbound traffic from sandboxes is permitted only through worker proxies (inference proxy, node-local web egress proxy, and orchestrator API Egress); sandboxes do not have direct internet access.
See [Sandbox Boundary and Security](cynode_sba.md#spec-cynai-sbagnt-sandboxboundary) and [Network Expectations](sandbox_container.md#spec-cynai-sandbx-networkexpect).

### Sandbox Workspace and Job Mounts

- Spec ID: `CYNAI.WORKER.SandboxWorkspaceJobMounts` <a id="spec-cynai-worker-sandboxworkspacejobmounts"></a>

#### Sandbox Workspace and Job Mounts Requirements Traces

- [REQ-WORKER-0250](../requirements/worker.md#req-worker-0250)

The worker node MUST provide container paths `/workspace` and `/job` by bind-mounting real, persistent directories from the host filesystem.
The node MUST create per-job directories on the host under a single configurable parent directory and MUST bind-mount them into the sandbox so that:

- `/workspace` in the container corresponds to a host path used as the job workspace (writable working directory).
- `/job` in the container corresponds to a host path used for job payload and result files (e.g. `job.json`, `result.json`).

The host parent directory MUST be configurable by the user as follows:

- **Primary mechanism:** Node startup YAML, key `sandbox.mount_root` (string, optional).
  - Value: absolute path on the host under which the node creates per-job subdirectories.
    The node MUST use the layout `<mount_root>/<job_id>/workspace` (bind-mounted as `/workspace`) and `<mount_root>/<job_id>/job` (bind-mounted as `/job`), where `<job_id>` is the job identifier and `<mount_root>` is the configured or default parent path.
  - When `sandbox.mount_root` is set, the node MUST use this path as the parent for all sandbox workspace and job bind mounts.
  - When `sandbox.mount_root` is unset, the node MUST use the path formed by appending `/sandbox_mounts` to the effective node state directory.
    The effective node state directory is the value of `storage.state_dir` when set, otherwise `/var/lib/cynode/state`.
    The full default path is `/var/lib/cynode/state/sandbox_mounts`.
- **Override via environment:** The node MUST support the environment variable `CYNODE_SANDBOX_MOUNT_ROOT` (absolute path) read by the Node Manager at startup.
  When set, it overrides `sandbox.mount_root` (environment takes precedence over YAML).
  When unset, the node uses `sandbox.mount_root` or the default above.

The node MUST create the host directories before starting the container and MUST ensure they persist for the duration of the job and until the node has finished processing the result (e.g. after result upload to the orchestrator), so that data is not lost on container exit.

### Sandbox Rootless Execution

- Spec ID: `CYNAI.WORKER.SandboxRootless` <a id="spec-cynai-worker-sandboxrootless"></a>

#### Sandbox Rootless Execution Requirements Traces

- [REQ-WORKER-0251](../requirements/worker.md#req-worker-0251)

When the container runtime supports rootless execution (e.g. Podman), the worker node MUST run sandbox containers in rootless mode (MUST NOT run sandbox containers as root).
When the runtime does not support rootless (e.g. typical Docker setups), the node runs containers as root or root-equivalent because the runtime provides no alternative.

Configuration

- Node startup YAML key `sandbox.rootless` (boolean, optional):
  - When the runtime supports rootless and `sandbox.rootless` is `true` or unset, the node MUST use rootless execution for sandbox containers.
  - When the operator sets `sandbox.rootless` to `false` and the runtime supports rootless, the node MUST run sandbox containers in non-rootless (root) mode.
  - When the runtime does not support rootless, `sandbox.rootless` has no effect and the node runs containers as the runtime requires.

The node MUST report rootless capability and effective state in the capability report (e.g. `rootless_supported`, `rootless_enabled` per [`worker_node_payloads.md`](worker_node_payloads.md)).

### Sandbox Control Plane Applicable Requirements

- Spec ID: `CYNAI.WORKER.NodeSandboxControlPlane` <a id="spec-cynai-worker-nodesandbox"></a>

#### Sandbox Control Plane Applicable Requirements Requirements Traces

- [REQ-WORKER-0109](../requirements/worker.md#req-worker-0109)
- [REQ-WORKER-0110](../requirements/worker.md#req-worker-0110)
- [REQ-WORKER-0111](../requirements/worker.md#req-worker-0111)
- [REQ-WORKER-0112](../requirements/worker.md#req-worker-0112)
- [REQ-WORKER-0113](../requirements/worker.md#req-worker-0113)

Worker API contract

- The Worker API endpoint surface and payload shapes are defined in [`docs/tech_specs/worker_api.md`](worker_api.md).

Worker API operations

The Worker API surface is intentionally minimal and MUST implement only:

- `POST /v1/worker/jobs:run`

Future revisions MAY add endpoints for file transfer, async job polling, and log streaming, but those MUST be defined in
[`docs/tech_specs/worker_api.md`](worker_api.md) before implementation.

See [`docs/tech_specs/mcp/mcp_tooling.md`](mcp/mcp_tooling.md) for the MCP tool layer that orchestrator-side agents use.

## Unified UDS Path (Agent and Sandbox Containers)

- Spec ID: `CYNAI.WORKER.UnifiedUdsPath` <a id="spec-cynai-worker-unifiedudspath"></a>

### Unified UDS Path (Agent and Sandbox Containers) Requirements Traces

- [REQ-WORKER-0270](../requirements/worker.md#req-worker-0270)

**All local agents run by the Node Manager** (managed service containers such as PMA and PAA, and sandbox containers including SBA) MUST use **only** UDS proxy endpoints.
There are no exceptions: the worker MUST NOT expose TCP (including loopback TCP) to any agent or sandbox for proxy or inference access.

All traffic to and from **agent containers** (managed services such as PMA) and **sandbox containers** MUST use **Unix domain sockets (UDS)** at the container boundary.
The worker MUST expose every proxy endpoint that a container uses (orchestrator-to-agent, agent-to-orchestrator, inference proxy for sandbox) only via UDS; containers MUST receive `http+unix://` URLs or socket paths, not TCP endpoints.

- **Managed agents (PMA, PAA):** Agent-to-orchestrator (MCP, callbacks) is already required to use per-service UDS per [Agent-To-Orchestrator UDS Binding](#agent-to-orchestrator-uds-binding-required).
  Managed service inference (when the agent calls node-local Ollama) MUST also be provided via a worker-exposed UDS (or `http+unix` URL), not by injecting a TCP URL such as `OLLAMA_BASE_URL=http://localhost:11434` into the agent container.
- **Sandbox containers:** Inference and any other proxy access from the sandbox MUST be via UDS: the worker (or an inference-proxy sidecar that exposes only a UDS) provides a socket to the sandbox; the sandbox receives an inference proxy URL (e.g. `INFERENCE_PROXY_URL=http+unix://...` or equivalent).
  The worker MUST NOT inject `OLLAMA_BASE_URL=http://localhost:11434` or any TCP endpoint into the sandbox for inference.

This unified path ensures a single, clear contract for tests and implementations: everything to/from agent or sandbox goes over UDS to the worker (or worker-controlled proxy); the worker forwards to Ollama, orchestrator, or other backends as needed.
See [Node-Local Inference](#node-local-inference-applicable-requirements) for sandbox inference details and [worker_api.md](worker_api.md) for managed agent proxy binding.

## Node-Local Inference and Sandbox Workflow

This section defines the preferred node-local workflow when a sandbox and Ollama inference are co-located on the same node.
Node-local traffic MUST remain on the node and MUST NOT traverse external networks.
All container-facing endpoints MUST use UDS per [Unified UDS Path](#unified-uds-path-agent-and-sandbox-containers).

### Node-Local Inference Applicable Requirements

- Spec ID: `CYNAI.WORKER.NodeLocalInference` <a id="spec-cynai-worker-nodelocalinference"></a>

#### Node-Local Inference Applicable Requirements Requirements Traces

- [REQ-WORKER-0114](../requirements/worker.md#req-worker-0114)
- [REQ-WORKER-0115](../requirements/worker.md#req-worker-0115)
- [REQ-WORKER-0270](../requirements/worker.md#req-worker-0270)
- [REQ-SANDBX-0131](../requirements/sandbx.md#req-sandbx-0131)

Unified UDS approach (required)

- For each sandbox job that uses inference, the Node Manager creates an isolated environment (e.g. pod or network) with the sandbox container and an inference proxy.
- The **inference proxy** MUST expose its endpoint to the sandbox **only via a Unix domain socket** (e.g. a socket file mounted into the sandbox or a path the sandbox can connect to).
  The inference proxy MUST NOT listen on TCP (e.g. `:11434`) inside the pod for sandbox access.
- The Node Manager MUST inject into the sandbox container an **inference proxy URL** that uses UDS (e.g. `INFERENCE_PROXY_URL=http+unix://<percent-encoded-socket-path>/` or an equivalent env that conveys the socket path).
  The Node Manager MUST NOT inject `OLLAMA_BASE_URL=http://localhost:11434` or any TCP URL for inference into the sandbox.
- The inference proxy (running as a sidecar or worker-owned process) forwards requests to the node's Ollama (or equivalent) over a node-internal channel; Ollama may still listen on TCP on the host (e.g. 11434) for the proxy only, but the sandbox never sees that endpoint.

See [`docs/tech_specs/ports_and_endpoints.md`](ports_and_endpoints.md#spec-cynai-stands-portsandendpoints) for consolidated ports (host-side Ollama remains 11434; container-facing inference is UDS only).

Rationale

- A single contract (UDS) for all container traffic simplifies security, testing, and reasoning: tests validate UDS presence and connectivity, not TCP URLs.
- The sandbox and agent containers have no TCP dependency on the worker or proxy; identity and isolation are enforced at the socket boundary.

Implementation notes

- The inference proxy sidecar (or worker process) MUST be minimal and MUST NOT expose credentials to the sandbox.
- The inference proxy MUST enforce request size limits and timeouts (e.g. maximum request body 10 MiB, per-request timeout 120 seconds).

### SBA Inference Proxy Capture and Reporting

Capture, binding, and reporting rules for the inference proxy follow.

#### 1 `Rule` Inference Proxy Capture

- Spec ID: `CYNAI.WORKER.InferenceProxyCapture` <a id="spec-cynai-worker-inferenceproxycapture"></a>

The worker inference proxy that serves SBA traffic MUST capture each LLM request and response body passing through it (non-streaming path).
The proxy MUST associate each capture with the `job_id` and `task_id` for that proxy instance.
The worker MUST apply opportunistic secret redaction to captured payloads using a **shared library** also used by the orchestrator gateway path so behavior is consistent (see [Redact SBA inference data before storage](orchestrator.md#spec-cynai-orches-sbainferencelogredaction)).
The worker MUST include redacted captures in reports to the orchestrator for optional persistence (exact Worker API shape is defined in [worker_api.md](worker_api.md)).

#### 2 `Rule` Inference Proxy Job Binding

- Spec ID: `CYNAI.WORKER.InferenceProxyJobBinding` <a id="spec-cynai-worker-inferenceproxyjobbinding"></a>

Each inference proxy instance MUST be created with the `job_id` and `task_id` for the sandbox job it serves; every captured record MUST carry those identifiers.

#### 3 `Rule` SBA Non-Streaming Redact-Forward

- Spec ID: `CYNAI.WORKER.SbaNonStreamingRedactForward` <a id="spec-cynai-worker-sbanonstreamingredactforward"></a>

For SBA non-streaming completions, the proxy MAY buffer the full response, redact, forward redacted content to the SBA, and attach redacted payloads to the orchestrator report.

#### 4 `Operation` Inference Report to Orchestrator

- Spec ID: `CYNAI.WORKER.InferenceReportToOrchestrator` <a id="spec-cynai-worker-inferencereporttoorchestrator"></a>

The worker MUST NOT send unredacted request or response bodies in inference reports; only redactor output MUST be included.

**Traces To:** [REQ-WORKER-0114](../requirements/worker.md#req-worker-0114), [REQ-WORKER-0115](../requirements/worker.md#req-worker-0115), [Node-Local Inference](#spec-cynai-worker-nodelocalinference).

## Node Sandbox MCP Exposure

When the orchestrator needs to manage or interact with a sandbox on a node, sandbox
operations MUST be exposed as MCP tools on that node.
The orchestrator acts as the default routing point for sandbox tools for remote agent runtimes.
When an AI agent runtime is co-located on the same host as the worker node, the node MUST support a low-latency control path that allows direct interaction with node-hosted sandbox tools under orchestrator-issued capability leases.

### Node Sandbox MCP Exposure Applicable Requirements

- Spec ID: `CYNAI.WORKER.NodeSandboxMcpExposure` <a id="spec-cynai-worker-nodesandboxmcpexposure"></a>

#### Node Sandbox MCP Exposure Applicable Requirements Requirements Traces

- [REQ-WORKER-0116](../requirements/worker.md#req-worker-0116)
- [REQ-WORKER-0117](../requirements/worker.md#req-worker-0117)
- [REQ-WORKER-0118](../requirements/worker.md#req-worker-0118)
- [REQ-WORKER-0119](../requirements/worker.md#req-worker-0119)

Required sandbox MCP tool surface

- `sandbox.create`
- `sandbox.exec`
- `sandbox.put_file`
- `sandbox.get_file`
- `sandbox.stream_logs`
- `sandbox.destroy`

### Node-Local Agent Sandbox Control (Low-Latency Path)

- Spec ID: `CYNAI.WORKER.NodeLocalAgentSandboxControl` <a id="spec-cynai-worker-nodelocalagentsandboxcontrol"></a>

#### Node-Local Agent Sandbox Control (Low-Latency Path) Requirements Traces

- [REQ-WORKER-0154](../requirements/worker.md#req-worker-0154)
- [REQ-WORKER-0155](../requirements/worker.md#req-worker-0155)
- [REQ-WORKER-0156](../requirements/worker.md#req-worker-0156)

This section defines a low-latency control path for sandbox operations when an AI agent runtime and the worker node are co-located on the same host.
The goal is to avoid routing every sandbox tool call through the orchestrator while still preserving policy enforcement and auditing.

Required properties

- The node MUST restrict direct access to node-hosted sandbox tools.
  Only node-local agent runtimes with orchestrator-issued capability leases may call this interface.
- Capability leases MUST be short-lived and least-privilege.
  They MUST scope calls to a `task_id` and MUST identify the allowed tool namespaces and operations.
- The node MUST validate capability leases and MUST fail closed when validation fails or when required scoped ids are missing.
- The node MUST emit audit records for direct tool calls with `task_id` context and MUST make those records available to the orchestrator.

Binding and transport

- The node MUST expose the direct control path only on loopback (`127.0.0.1`), a Unix domain socket, or both.
  The node MUST NOT expose it on a non-loopback network interface.
- The direct control path MUST use the same MCP tool identities and argument schemas as the node-hosted MCP server so policy remains consistent.

## Node Startup YAML

Nodes MUST support a local startup YAML file that the Node Manager reads on boot.
This file provides the minimum information required to contact the orchestrator and allows operators to apply node-local constraints.

### Node Startup YAML Applicable Requirements

- Spec ID: `CYNAI.WORKER.NodeStartupYaml` <a id="spec-cynai-worker-nodestartupyaml"></a>
- Node startup YAML must not be treated as the source of truth for global policy.
- The orchestrator remains the source of truth for scheduling and allowed capabilities after registration.
- Node startup YAML may impose stricter local constraints than the orchestrator requests.
- If a local constraint prevents fulfilling an orchestrator request, the node must refuse the request and report the reason.

#### Node Startup YAML Applicable Requirements Requirements Traces

- [REQ-WORKER-0120](../requirements/worker.md#req-worker-0120)
- [REQ-WORKER-0121](../requirements/worker.md#req-worker-0121)
- [REQ-WORKER-0122](../requirements/worker.md#req-worker-0122)

Recommended location

- `/etc/cynode/node.yaml`

Example

- See [`docs/examples/node_bootstrap_example.yaml`](../examples/node_bootstrap_example.yaml) for a minimal node (worker) startup config.

### User-Configurable Properties

Node startup YAML MUST allow operators to set the properties below.
These settings are node-local and MAY impose stricter constraints than orchestrator policy (see REQ-WORKER-0121).

#### Top-Level Keys

- `version` (number, required)
- `orchestrator` (object, required)
- `node` (object, required)
- `worker_api` (object, optional)
- `sandbox` (object, optional)
- `inference` (object, optional)
- `storage` (object, optional)
- `logging` (object, optional)
- `updates` (object, optional)

#### Orchestrator Settings

- `orchestrator.url` (string, required)
  - Orchestrator base URL (e.g. `https://orch.example.com`).
- `orchestrator.registration_psk_env` (string, optional)
  - Environment variable name containing the registration PSK.
- `orchestrator.registration_psk_file` (string, optional)
  - File path containing the registration PSK.
- `orchestrator.tls.ca_bundle_path` (string, optional)
  - CA bundle used to validate orchestrator TLS.
- `orchestrator.tls.pinned_sha256` (string, optional)
  - SHA-256 pin for orchestrator trust, when using pinning.
- `orchestrator.timeouts.connect_seconds` (number, optional)
  - Connection timeout for orchestrator requests.
- `orchestrator.timeouts.request_seconds` (number, optional)
  - Request timeout for orchestrator requests.

##### Security Notes

- Secrets MUST be supplied via env vars or local files.
- Node startup YAML MUST NOT embed external provider API keys.

#### Node Settings

- `node.id` (string, required)
  - Stable node identifier used for registration and scheduling.
- `node.name` (string, optional)
  - Human-friendly name for inventory and logs.
- `node.labels` (array of strings, optional)
  - Labels used by scheduling and policy (e.g. `trusted`, `sandbox_only`, `region_us_east_1`).
- `node.metadata` (object, optional)
  - Free-form key/value metadata.

#### Worker API Settings

- `worker_api.listen_host` (string, optional)
  - Address/interface to bind.
  - Default MUST be `0.0.0.0`.
- `worker_api.listen_port` (number, optional)
  - Port to bind.
    Default MUST be `12090` (CyNodeAI Worker API default; see [`ports_and_endpoints.md`](ports_and_endpoints.md#spec-cynai-stands-portsandendpoints)).
- `worker_api.public_base_url` (string, optional)
  - Public URL the orchestrator should use to reach the worker API.
- `worker_api.max_request_bytes` (number, optional)
  - Maximum request size accepted by the Worker API.

#### Sandbox Settings

Node startup YAML MUST support a sandbox mode that determines whether the node is eligible for sandbox execution.

Recommended values

- `allow`
  - The node MUST be eligible for sandbox execution.
  - The node MAY provide inference if available and enabled.
- `sandbox_only`
  - The node MUST be eligible for sandbox execution.
  - The node MUST NOT run inference services.
- `disabled`
  - The node MUST NOT run sandboxes.
  - The orchestrator MUST treat the node as ineligible for sandbox scheduling.

Sandbox keys

- `sandbox.mode` (string, optional)
  - One of `allow`, `sandbox_only`, or `disabled`.
- `sandbox.runtime` (string, optional)
  - Sandbox runtime identifier: `podman` or `docker`.
    Podman is preferred and supports rootless; when runtime is Podman, the node MUST use rootless for sandboxes unless overridden.
- `sandbox.rootless` (boolean, optional)
  - When the runtime supports rootless (e.g. Podman), the node MUST use rootless execution when this is `true` or unset; when `false`, the node MUST run sandbox containers in non-rootless (root) mode.
    When the runtime does not support rootless, this key has no effect.
    See [Sandbox rootless execution](#sandbox-rootless-execution).
- `sandbox.max_concurrency` (number, optional)
  - Maximum concurrent sandbox jobs accepted by this node.
- `sandbox.default_image` (string, optional)
  - Default sandbox image reference when a job does not specify one.
- `sandbox.allowed_images` (array of strings, optional)
  - Allowlist of sandbox images this node will run.
- `sandbox.default_network_policy` (string, optional)
  - Node-local default network policy (e.g. `restricted`, `none`, `allowlist`).
- `sandbox.allowed_egress_domains` (array of strings, optional)
  - Domain allowlist for sandbox egress when policy is allowlist-based.
  - This allowlist is enforced by the controlled egress path (for example the Web Egress Proxy).
    See [`docs/tech_specs/web_egress_proxy.md`](web_egress_proxy.md).
- `sandbox.resources.max_cpu_cores` (number, optional)
  - Maximum CPU cores allowed for a sandbox job.
- `sandbox.resources.max_memory_mb` (number, optional)
  - Maximum memory allowed for a sandbox job.
- `sandbox.resources.max_pids` (number, optional)
  - Maximum process count allowed for a sandbox job.
- `sandbox.timeouts.default_seconds` (number, optional)
  - Default sandbox job timeout (seconds) when the orchestrator does not set `sandbox.timeout_seconds` on the request.
  When unset, the node MUST use 3600 (1 hour).
- `sandbox.timeouts.max_seconds` (number, optional)
  - Maximum sandbox job timeout (seconds) allowed on this node; the orchestrator (e.g. PMA/PAA) sets per-job timeout in the request, and the node caps it at this max.
  When unset, the node MUST use 10800 (3 hours).
- `sandbox.mount_root` (string, optional)
  - Absolute path on the host under which the node creates per-job directories that are bind-mounted into sandboxes as `/workspace` and `/job`.
    When unset, the node MUST use `<effective state_dir>/sandbox_mounts`, where the effective state directory is `storage.state_dir` if set, otherwise `/var/lib/cynode/state`.
    See [Sandbox workspace and job mounts](#sandbox-workspace-and-job-mounts).
- `sandbox.mounts.allowed_host_paths` (array of strings, optional)
  - Allowlist of host paths that may be mounted into sandboxes.
- `sandbox.mounts.read_only_by_default` (boolean, optional)
  - Whether allowed host mounts are read-only by default.
- `sandbox.security.allow_privileged` (boolean, optional)
  - Whether privileged containers are allowed.
- `sandbox.security.allow_host_network` (boolean, optional)
  - Whether host networking is allowed.
- `sandbox.security.allow_device_mounts` (boolean, optional)
  - Whether device mounts are allowed.

#### Inference Settings

Node startup YAML MUST allow operators to set a user-defined override for inference on this node (reported in the capability report so the orchestrator can make the decision).
The node MUST read this from local config (YAML, environment variables, etc.) at startup and include it in the capability report as `inference.mode`.

Inference keys

- `inference.mode` (string, optional)
  - User-defined override: one of `allow` (no override), `disabled` (operator requires no inference on this node), or `require` (operator requires inference when policy allows).
    When unset, the node reports `allow` or omits the field.
- `inference.max_concurrency` (number, optional)
  - Maximum concurrent inference requests accepted by local inference services.
- `inference.allow_gpu` (boolean, optional)
  - Whether GPU/NPU devices may be used for inference on this node.

#### Storage Settings

- `storage.state_dir` (string, optional)
  - Node state directory (registration cache and applied config state).
    When unset, the node MUST use `/var/lib/cynode/state`.
- `storage.cache_dir` (string, optional)
  - Cache directory (image pulls and temporary files).
- `storage.artifacts_dir` (string, optional)
  - Local artifact staging directory.

#### Logging Settings

- `logging.level` (string, optional)
  - Logging level (e.g. `debug`, `info`, `warn`, `error`).
- `logging.format` (string, optional)
  - Log format (e.g. `text`, `json`).
- `logging.file_path` (string, optional)
  - When set, write logs to this file path.

#### Update Settings

- `updates.enable_dynamic_config` (boolean, optional)
  - Whether the node will apply orchestrator configuration updates after registration.
- `updates.poll_interval_seconds` (number, optional)
  - Polling interval for configuration refresh when push updates are not used.
- `updates.allow_service_restart` (boolean, optional)
  - Whether the Node Manager may restart services to apply configuration.

Example

```yaml
version: 1
orchestrator:
  url: https://orch.example.com
  registration_psk_env: CYNODE_REGISTER_PSK
  tls:
    ca_bundle_path: /etc/cynode/ca-bundle.pem
  timeouts:
    connect_seconds: 5
    request_seconds: 30
node:
  id: sandbox-us-east-1-01
  name: Sandbox Only Node
  labels:
    - sandbox_only
    - region_us_east_1
worker_api:
  listen_host: 0.0.0.0
  listen_port: 12090
  public_base_url: https://worker-01.example.com
  max_request_bytes: 10485760
sandbox:
  mode: sandbox_only
  runtime: podman   # or docker; podman preferred for rootless
  rootless: true
  max_concurrency: 4
  # mount_root: /var/lib/cynode/sandbox_mounts   # parent for /workspace and /job bind mounts
  default_network_policy: restricted
  allowed_images:
    # When no registry list is configured, images are pulled from Docker Hub (docker.io)
    - python:3.12
    - node:22
  mounts:
    allowed_host_paths:
      - /var/lib/cynode/shared
    read_only_by_default: true
  resources:
    max_cpu_cores: 8
    max_memory_mb: 16384
    max_pids: 2048
  timeouts:
    default_seconds: 3600   # 1 hour when orchestrator does not set per-job timeout
    max_seconds: 10800     # 3 hours node cap
  security:
    allow_privileged: false
    allow_host_network: false
    allow_device_mounts: false
inference:
  mode: disabled
storage:
  state_dir: /var/lib/cynode/state
  cache_dir: /var/lib/cynode/cache
  artifacts_dir: /var/lib/cynode/artifacts
logging:
  level: info
  format: json
updates:
  enable_dynamic_config: true
  poll_interval_seconds: 60
  allow_service_restart: true
```

## Node Startup Procedure

- Spec ID: `CYNAI.WORKER.NodeStartupProcedure` <a id="spec-cynai-worker-nodestartupprocedure"></a>

### Node Startup Procedure Requirements Traces

- [REQ-WORKER-0253](../requirements/worker.md#req-worker-0253)
- [REQ-WORKER-0254](../requirements/worker.md#req-worker-0254)

This procedure is constrained by [Existing Inference Service on Host](#existing-inference-service-on-host) (use existing host inference when present; do not start a duplicate) and [Ollama Container Policy](#ollama-container-policy) (at most one inference service in use; when the node starts the container, it grants that container access to all GPUs and NPUs).

On startup, the Node Manager MUST contact the orchestrator and receive configuration **before** starting the Ollama (or equivalent) container.
The Worker API MUST be started and the node MUST register with the orchestrator (sending its capabilities bundle) before the node starts any local inference container.
The orchestrator acknowledges registration and returns a node configuration payload that **instructs** the node whether and how to start the local inference backend (e.g. container image and backend variant such as ROCm for AMD or CUDA for Nvidia).
The node MUST NOT start the Ollama container until it has received this instruction in the node configuration payload (see [`worker_node_payloads.md`](worker_node_payloads.md) `node_configuration_payload_v1` `inference_backend`).
When the orchestrator omits `inference_backend.image`, the node MUST derive the backend container image from `inference_backend.variant` per [worker_node_payloads.md](worker_node_payloads.md) (for Ollama: variant `rocm` -> `ollama/ollama:rocm`; variant `cuda` or `cpu` -> `ollama/ollama` or `ollama/ollama:latest`, since Ollama has no cuda tag) and MUST NOT use a node-local env default (e.g. a single `OLLAMA_IMAGE`) that ignores or overrides the orchestrator-supplied variant.
When the instruction includes `inference_backend.env`, the node MUST pass those orchestrator-directed backend environment values into the launched backend container.
Those values represent the orchestrator's effective runtime configuration for maximizing the safe usable context window for the expected local model workload on that node.

The system requires that the overall deployment has at least one inference-capable path.
Inference may be provided by node-local inference (Ollama) or by external model routing through API Egress when configured.
In the single-node case, the system MUST refuse to enter a ready state if the node cannot run the inference container and there is no configured external provider key.
See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md) and [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md).

Required startup flow (order is mandatory)

1. Start Node Manager system service.
2. Load node startup YAML and apply node-local constraints.
3. Perform [node startup checks](#node-startup-checks-and-readiness); the node MUST NOT report ready until these pass.
4. Collect host capabilities (platform, GPU, container runtime, etc.).
5. Register with orchestrator and send capability report (capabilities bundle).
6. Fetch the latest node configuration from orchestrator (orchestrator acks and returns config).
7. Start the Worker API service (so the node is reachable for job dispatch).
8. **Local inference:** Apply [Existing Inference Service on Host](#existing-inference-service-on-host): if an inference service (OLLAMA or equivalent) is already running on the host and reachable, the node MUST use it and MUST NOT start another.
   In either case, the node MUST treat the orchestrator-delivered `inference_backend` contract, including any derived backend environment values, as the authoritative effective runtime configuration for the node-local inference path.
   Only when no existing service is detected and the node configuration instructs the node to start local inference (see `inference_backend` in [`worker_node_payloads.md`](worker_node_payloads.md)), the node starts the single Ollama (or equivalent) container per [Ollama Container Policy](#ollama-container-policy) (image, variant, and any orchestrator-directed backend environment values specified by the orchestrator; container granted access to all GPUs and NPUs).
9. Report startup status and effective configuration version to the orchestrator (config ack and ongoing capability reporting).
    The node MUST report to the orchestrator when it has become ready (e.g. via config ack with status applied after services are started) so the orchestrator can consider the node as an inference path and start the Project Manager Agent when appropriate; see [REQ-WORKER-0254](../requirements/worker.md#req-worker-0254).

## Node Startup Checks and Readiness

- Spec ID: `CYNAI.WORKER.NodeStartupChecks` <a id="spec-cynai-worker-nodestartupchecks"></a>

### Node Startup Checks and Readiness Requirements Traces

- [REQ-WORKER-0252](../requirements/worker.md#req-worker-0252)

The worker node MUST perform startup checks before reporting ready (i.e. before `GET /readyz` returns 200).
The node MUST NOT report ready until the following prerequisites are verified (as applicable to the node configuration):

- **Container runtime:** The node MUST verify it can create and run a container (e.g. run a minimal image or a no-op container create/start/stop) using the configured runtime (Podman or Docker).
  If the runtime is unavailable or fails the check, the node MUST NOT report ready.
- **Sandbox mount root:** When sandbox execution is enabled (`sandbox.mode` is `allow` or `sandbox_only`), the node MUST verify that the effective sandbox mount root directory (from `sandbox.mount_root` or default) exists or can be created and is writable by the node process.
  If the mount root is not usable, the node MUST NOT report ready.
- **Worker API:** The Worker API HTTP server MUST be listening and responding to `GET /healthz` before the node reports ready.
- **Orchestrator connectivity (when required):** When the node must register or fetch config before accepting jobs, the node MUST complete registration and config fetch before reporting ready so that the orchestrator can schedule work to the node.

No other checks may block readiness.
When any of the above checks fails, `GET /readyz` MUST return 503.
When all applicable checks pass, `GET /readyz` MUST return 200 with body `ready`.

## Deployment and Auto-Start

- Spec ID: `CYNAI.WORKER.DeploymentAutoStart` <a id="spec-cynai-worker-deploymentautostart"></a>

### Deployment and Auto-Start Requirements Traces

- [REQ-BOOTST-0104](../requirements/bootst.md#req-bootst-0104)

Worker node deployments MUST support auto-start on the host so that Node Manager and Worker API (and related services) start on boot or on demand without manual invocation.

- **Linux:** The implementation MUST provide systemd unit files for worker node services (Node Manager, Worker API).
  Both user (rootless) and system (root) installs MUST be supported.
  See [`worker_node/systemd/README.md`](../../worker_node/systemd/README.md) for the reference layout and generation steps.
- **macOS:** The implementation MUST provide launchd plist files for worker node services so that they can start on boot or on user login, with the same capability as the Linux systemd approach (start on boot, start on demand, enable/disable).

## Deployment Topologies

- Spec ID: `CYNAI.WORKER.DeploymentTopologies` <a id="spec-cynai-worker-deploymenttopologies"></a>

### Deployment Topologies Requirements Traces

- [REQ-WORKER-0272](../requirements/worker.md#req-worker-0272)

The implementation supports **single-process (host binary) only**.
Node Manager and Worker API run in one process; one binary (`cynodeai-wnm`), one system service.
No separate Worker API process and no Worker API as a managed container.

#### Normative Topology vs. Deferred Alternatives

- **Normative (requirements):** [REQ-WORKER-0272](../requirements/worker.md#req-worker-0272) requires the Node Manager and the Worker API to run in **one host process** (one binary, one service unit).
  That process is the **worker control plane** for registration, configuration, the Worker API HTTP surface, telemetry, and Node Manager duties.
- **Compatible with single-process:** **Managed service containers** (for example PMA or a local inference backend) are **separate containers** that the Node Manager starts and supervises.
  They are not additional Worker API OS processes.
  See [Managed Service Containers](#managed-service-containers) and [Node-Local Inference and Sandbox Workflow](#node-local-inference-and-sandbox-workflow).
- **Deferred / not implemented:** A deployment that splits Node Manager and Worker API into **separate OS processes** or runs the Worker API as its **own container image** (independent of the Node Manager binary) is **out of scope** for the current implementation and is not required by REQ-WORKER-0272 for MVP.

Error conditions:

- If the Worker API HTTP server cannot bind or start (e.g. port in use), the process MUST fail startup with a clear error and non-zero exit code.

## Single-Process Host Binary

- Spec ID: `CYNAI.WORKER.SingleProcessHostBinary` <a id="spec-cynai-worker-singleprocesshostbinary"></a>

### Single-Process Host Binary Requirements Traces

- [REQ-WORKER-0272](../requirements/worker.md#req-worker-0272)
- [REQ-WORKER-0273](../requirements/worker.md#req-worker-0273)
- [REQ-WORKER-0172](../requirements/worker.md#req-worker-0172) (secure store boundary when separate processes)
- [CYNAI.WORKER.NodeStartupProcedure](#spec-cynai-worker-nodestartupprocedure)
- [CYNAI.WORKER.NodeManagerShutdown](#spec-cynai-worker-nodemanagershutdown)

This rule defines the required behavior when the worker node is run in the single-process (host binary) topology: one process runs both Node Manager and Worker API.

### Single-Process Host Binary Scope

- Applies when the deployment topology is single-process (host binary).
- The same process MUST perform: node registration, config fetch, config apply, telemetry DB lifecycle (node_boot, retention, vacuum, shutdown event), secure store writes (config apply) and reads (worker proxy), and Worker API HTTP server (healthz, jobs, telemetry, managed-service proxy, etc.).
- The Worker API HTTP server MUST be started in the same process as the Node Manager after configuration is applied; the implementation MUST NOT spawn a separate Worker API process for this topology.

### Single-Process Host Binary Preconditions

- Node startup YAML (or equivalent) and environment are loaded; orchestrator URL, node identity, and registration PSK are available.
- Container runtime (Podman or Docker) is available when the node is configured for sandbox or managed services.

### Single-Process Host Binary Outcomes

- The single process MUST open and own the telemetry SQLite database; MUST perform node_boot insert once per process start; MUST run retention and vacuum per [Worker Telemetry API](worker_telemetry_api.md) spec; MUST record node manager shutdown on exit.
- The single process MUST apply node configuration (secure store writes) and MUST serve Worker API endpoints including the internal proxy that reads from the secure store; the process boundary is the trusted boundary per [CYNAI.WORKER.SecureStoreProcessBoundary](#spec-cynai-worker-securestoreprocessboundary).
- Worker API listen address and port (e.g. `0.0.0.0:12090`) MUST be taken from node startup YAML or environment; the same process MUST bind and serve the Worker API and internal proxy (e.g. UDS for managed agents) until shutdown.
- On shutdown (SIGTERM, SIGINT, or systemd stop), the process MUST follow [CYNAI.WORKER.NodeManagerShutdown](#spec-cynai-worker-nodemanagershutdown): stop all managed containers and sandbox containers, then exit; the Worker API HTTP server MUST stop accepting new requests and MUST drain or close in coordination with that shutdown.

### `SingleProcessHostBinary` Algorithm

<a id="algo-cynai-worker-singleprocesshostbinary-startup"></a>

1. Start the single process and load node startup YAML and environment. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-1"></a>
2. Open telemetry store (create directory if needed); run retention on startup; insert node_boot once; start background retention and vacuum goroutines per telemetry spec. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-2"></a>
3. Perform node startup checks (container runtime, sandbox mount root if applicable) per [Node Startup Checks and Readiness](#spec-cynai-worker-nodestartupchecks). <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-3"></a>
4. Register with orchestrator and send capability report; obtain bootstrap data (JWT, report URL, config URL). <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-4"></a>
5. Fetch node configuration from orchestrator. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-5"></a>
6. Apply configuration: resolve and write secrets to secure store; apply worker proxy config. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-6"></a>
7. Start the Worker API HTTP server in the same process (bind to configured listen address/port and optional internal UDS); do not spawn a separate process. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-7"></a>
8. Start local inference container (Ollama) only when no existing host inference is detected and config instructs, per [Node Startup Procedure](#spec-cynai-worker-nodestartupprocedure). <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-8"></a>
9. Start orchestrator-directed managed service containers (e.g. PMA) per config. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-9"></a>
10. Send config ack to orchestrator; run capability reporting loop until shutdown. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-10"></a>

The algorithm above defines the required startup order for the single-process topology and extends the [Node Startup Procedure](#spec-cynai-worker-nodestartupprocedure) by specifying that the Worker API is started in-process (step 7).

### Single-Process Host Binary Error Conditions

- If telemetry store cannot be opened or node_boot fails, the process MUST log the error and MAY continue without telemetry or MUST exit with non-zero exit code depending on implementation policy; the spec does not mandate exit for telemetry failure.
- If registration, config fetch, or config apply fails, the process MUST exit with non-zero exit code.
- If the Worker API HTTP server fails to bind or start (e.g. port in use), the process MUST exit with non-zero exit code.
- If startup checks fail, the process MUST NOT report ready and MUST exit or retry per [Node Startup Checks and Readiness](#spec-cynai-worker-nodestartupchecks).

### Single-Process Host Binary Observability

- The single process MUST emit logs with a source identifier (e.g. `node_manager` or a unified node source) so that telemetry and logs can attribute events to the node.
- Shutdown MUST be recorded in the telemetry store per [CYNAI.WORKER.TelemetryLifecycleEvents](worker_telemetry_api.md#spec-cynai-worker-telemetrylifecycleevents).

### Binary Name and Invocation (Informational)

- The implementation MUST document the single binary name `cynodeai-wnm` and invocation for host deployment.
- One invocation runs both Node Manager and Worker API in the same process; no second binary or subcommand is required for normal operation.

## Existing Inference Service on Host

- Spec ID: `CYNAI.WORKER.ExistingInferenceService` <a id="spec-cynai-worker-existinginferenceservice"></a>

### Existing Inference Service on Host Requirements Traces

- [REQ-WORKER-0255](../requirements/worker.md#req-worker-0255)

When an OLLAMA (or equivalent) inference service is already running on the host and is reachable (e.g. on the default port or a configured address), the node MUST use that existing service and MUST NOT start its own inference container.
The node MUST detect an existing service (e.g. by probing the inference API or checking for a known container/process) before deciding to start a container per orchestrator instruction.
When the node is using an existing inference service, it MUST report this in the capability report (see [Capability Reporting](#capability-reporting) and `inference.existing_service` in [`worker_node_payloads.md`](worker_node_payloads.md)) so the orchestrator does not instruct the node to start a duplicate container.

## Ollama Container Policy

The node MUST run at most one Ollama (or equivalent) inference service in use at a time: either one the node started or an existing service on the host.
When the node starts the container itself, that container MUST be granted access to all GPUs and NPUs on the system.

Rationale

- Centralizes accelerator scheduling for model inference.
- Avoids conflicting GPU allocation and reduces operational complexity.
- When the host already runs OLLAMA, the node uses it instead of starting another (see [Existing Inference Service on Host](#existing-inference-service-on-host)).

## Sandbox-Only Nodes

CyNodeAI MUST support nodes that do not provide AI inference capabilities.
These nodes exist to run sandbox containers for tool execution, builds, tests, and other compute tasks.

### Sandbox-Only Nodes Applicable Requirements

- Spec ID: `CYNAI.WORKER.SandboxOnlyNodes` <a id="spec-cynai-worker-sandboxonlynodes"></a>

#### Sandbox-Only Nodes Applicable Requirements Requirements Traces

- [REQ-WORKER-0123](../requirements/worker.md#req-worker-0123)
- [REQ-WORKER-0124](../requirements/worker.md#req-worker-0124)
- [REQ-WORKER-0125](../requirements/worker.md#req-worker-0125)
- [REQ-WORKER-0126](../requirements/worker.md#req-worker-0126)

Capability reporting guidance

- Sandbox-only nodes MUST report `gpu.present=false`.
- Sandbox-only nodes MUST include labels that indicate sandbox execution is supported.
- Sandbox-only nodes MUST include labels that indicate inference is not supported.

## Registration and Bootstrap

- Spec ID: `CYNAI.WORKER.RegistrationAndBootstrap` <a id="spec-cynai-worker-registrationandbootstrap"></a>

During registration, the node establishes trust with the orchestrator and receives a bootstrap configuration payload.
Canonical payload shapes are defined in [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md).

Required flow

- Node registers using a pre-shared key (PSK).
- Node sends a capability report as part of registration and on startup.
  The capability report MUST include `worker_api.base_url` (the full URL the orchestrator MUST use to call the Worker API on this node), so the orchestrator can add or update the node record and dispatch jobs.
- Orchestrator validates the node and issues a JWT for ongoing communication.
- Orchestrator adds or updates the node in the database; the node's Worker API dispatch URL is set from the node-reported `worker_api.base_url` unless an explicit operator override is configured (e.g. `WORKER_API_TARGET_URL` when the worker runs on the same host as the orchestrator); any override MUST be clearly documented as an override.
- Orchestrator returns a bootstrap payload that includes:
  - orchestrator base URL and required service endpoints
  - trust material (e.g. CA bundle or pinned certificate), when applicable
  - pull endpoints and credentials required for orchestrator-provided services
  - full NATS configuration (`nats` object): server URL, node-scoped NATS JWT, JWT expiry, optional TLS CA, and optional subject overrides (see [NATS Connection and Chat Bridge](#spec-cynai-worker-natschatbridge) and [`node_bootstrap_payload_v1`](worker_node_payloads.md#spec-cynai-worker-payload-bootstrap-v1))

### Orchestrator Registration Retry

- Spec ID: `CYNAI.WORKER.OrchestratorRegistrationRetry` <a id="spec-cynai-worker-orchestratorregistrationretry"></a>

When the worker is **online before the orchestrator** (or the orchestrator is temporarily unreachable), registration or bootstrap HTTP calls to the orchestrator MAY fail (connection refused, timeout, DNS failure, TLS error, or HTTP 5xx).

#### Required Behavior

- The node MUST **continue** attempting registration (and any prerequisite orchestrator contact needed to obtain bootstrap configuration) until it **succeeds** or the operator stops the node process.
- The node MUST NOT treat a transient orchestrator outage as a fatal startup error that prevents all future retries while the process remains running.
- **Retry schedule** (wall clock from the **first failed** registration attempt):
  - **0-5 minutes:** wait **60 seconds** between each retry attempt (including the wait before the second attempt after the first failure).
  - **After 5 minutes:** wait **300 seconds** (5 minutes) between each subsequent retry attempt.
- The node SHOULD log each failed attempt at **info** or **warning** with a clear reason (without leaking secrets) so operators can diagnose orchestrator-side delays.

#### Orchestrator Registration Retry Requirements Traces

- [REQ-WORKER-0275](../requirements/worker.md#req-worker-0275)

## Capability Reporting

- Spec ID: `CYNAI.WORKER.CapabilityReporting` <a id="spec-cynai-worker-capabilityreporting"></a>

### Capability Reporting Requirements Traces

- [REQ-WORKER-0139](../requirements/worker.md#req-worker-0139)
- [REQ-WORKER-0256](../requirements/worker.md#req-worker-0256)

Nodes MUST report host capabilities to the orchestrator so the orchestrator can select compatible configuration and schedule work safely.
Nodes MUST report capabilities during registration and again on every node startup (check-in).
Canonical payload shapes are defined in [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md).

Required capability fields

- Worker API address (required for dispatch)
  - `worker_api.base_url`: full URL the orchestrator MUST use to call the Worker API (scheme, host, port); the node MUST send this at registration and in capability reports so the orchestrator can add or update the node's dispatch URL; see [`worker_node_payloads.md`](worker_node_payloads.md) capability report `worker_api`.
- Inference service status (when inference is supported)
  - The node MUST include in the capability report whether it already has a running inference service on the host (e.g. `inference.existing_service` and `inference.running` per [`worker_node_payloads.md`](worker_node_payloads.md)).
  - This is required at registration and on every check-in so the orchestrator can treat the node as inference-capable and avoid instructing the node to start an inference container when one is already present.
- Identity and platform
  - OS type and distribution details
  - architecture (e.g. amd64, arm64)
  - kernel version
  - container runtime details (Docker or Podman version, rootless support)
- Compute resources
  - CPU model and core count
  - total system RAM
  - storage available for models and images
- GPU resources
  - GPU vendor and model names
  - total VRAM and available VRAM
  - device count and device identifiers
  - supported compute features when relevant (e.g. CUDA capability, ROCm support)
- Networking and security
  - reachable orchestrator endpoints
  - TLS capabilities and trust material status
  - outbound internet policy status, if enforced by the node
- Node labels and capabilities
  - configured tags (e.g. "gpu", "low_power", "trusted")
  - supported sandbox features (e.g. network namespaces, seccomp profiles)

Change reporting

- Nodes MUST report the full capability report as JSON (actual capabilities: node identity, platform, compute, gpu, sandbox, network, inference, tls, etc.) using the schema in [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md).
- If capabilities change (hardware change, driver change, runtime change), the node MUST report an updated capability report.

## Configuration Delivery

- Spec ID: `CYNAI.WORKER.ConfigurationDelivery` <a id="spec-cynai-worker-configurationdelivery"></a>

### Configuration Delivery Requirements Traces

- [REQ-ORCHES-0149](../requirements/orches.md#req-orches-0149)
- [REQ-WORKER-0135](../requirements/worker.md#req-worker-0135)

The orchestrator MUST be able to deliver and update configuration for registered nodes.
Canonical payload shapes are defined in [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md).

Worker API target URL in config

- The node configuration payload includes `orchestrator.endpoints.worker_api_target_url`, which is the URL the orchestrator will use to call this node's Worker API.
- That URL is normally the node-reported `worker_api.base_url` from registration or the latest capability report; the orchestrator stores it and echoes it in config.
- An operator MAY configure an explicit override (e.g. when the worker runs on the same host as the orchestrator); when set, the override is used and MUST be documented as an override (e.g. in deployment docs or env var name such as `WORKER_API_TARGET_URL` for same-host override).

Inference backend instruction in config

- The node configuration payload MAY include `inference_backend` (see [`worker_node_payloads.md`](worker_node_payloads.md) `node_configuration_payload_v1`).
- The orchestrator MUST derive the inference backend instruction (whether to start, which image, and variant such as ROCm or CUDA) using the deterministic algorithm in [orchestrator_inference_container_decision.md](orchestrator_inference_container_decision.md#spec-cynai-orches-inferencecontainerdecision), and MUST include it in the config when the node is inference-capable and inference is enabled.
  Variant MUST be derived by **model and/or VRAM**, not vendor alone: when the node reports multiple GPU types, the orchestrator uses **total VRAM per vendor** (sum of `vram_mb` per vendor) and selects the variant for the vendor with the greatest total VRAM; the node MUST report all GPUs (all vendors) with per-device `vram_mb` so this is correct (see [worker_node_payloads.md](worker_node_payloads.md) and [REQ-WORKER-0265](../requirements/worker.md#req-worker-0265)).
- The node MUST NOT start the OLLAMA (or equivalent) container until it has received this instruction; see [Node Startup Procedure](#node-startup-procedure).

Required behavior

- The node MUST support receiving configuration at registration time.
- In the current minimum implementation, the node MUST fetch configuration on startup and MUST NOT poll for configuration updates.
- In implementations that add configuration refresh, the node MUST support refresh by polling.
  The node MAY additionally support push notification.
  When an update is delivered via any supported channel (polling or push), the node MUST apply it.
- The node MUST validate configuration authenticity and origin before applying it.
- The node MUST report configuration application status back to the orchestrator.

## Dynamic Configuration Updates

- Spec ID: `CYNAI.WORKER.DynamicConfigurationUpdates` <a id="spec-cynai-worker-dynamicconfigurationupdates"></a>

The orchestrator MUST be able to update node configuration after registration.
This enables rotating credentials, changing the rank-ordered registry list or per-registry endpoints, and applying new policy.

Required behavior (when `policy.updates.enable_dynamic_config=true`)

- The orchestrator MUST version node configuration payloads.
- The node MUST poll for configuration updates or receive them via a push channel.
  When NATS is deployed, the node subscribes to config change notifications ([CYNAI.WORKER.NatsConfigNotificationSubscriber](nats_messaging.md#spec-cynai-worker-natsconfignotificationsubscriber)) for near-zero-latency config push; polling remains as fallback.
- The node MUST apply configuration updates atomically where possible and MUST roll back on failure.
- The node MUST acknowledge applied configuration version to the orchestrator.
- The node MUST request a configuration refresh on startup and when capability reports change.

## Credential Handling

Nodes require credentials to connect to orchestrator-provided services.
These credentials MUST be handled securely and with least privilege.

### Credential Handling Applicable Requirements

- Spec ID: `CYNAI.WORKER.NodeCredentialHandling` <a id="spec-cynai-worker-nodecredentialhandling"></a>

#### Credential Handling Applicable Requirements Requirements Traces

- [REQ-WORKER-0127](../requirements/worker.md#req-worker-0127)
- [REQ-WORKER-0128](../requirements/worker.md#req-worker-0128)
- [REQ-WORKER-0129](../requirements/worker.md#req-worker-0129)
- [REQ-WORKER-0130](../requirements/worker.md#req-worker-0130)

### Node-Local Secure Store

- Spec ID: `CYNAI.WORKER.NodeLocalSecureStore` <a id="spec-cynai-worker-nodelocalsecurestore"></a>

#### Node-Local Secure Store Requirements Traces

- [REQ-WORKER-0128](../requirements/worker.md#req-worker-0128)
- [REQ-WORKER-0132](../requirements/worker.md#req-worker-0132)
- [REQ-WORKER-0165](../requirements/worker.md#req-worker-0165)
- [REQ-WORKER-0166](../requirements/worker.md#req-worker-0166)
- [REQ-WORKER-0167](../requirements/worker.md#req-worker-0167)
- [REQ-WORKER-0168](../requirements/worker.md#req-worker-0168)
- [REQ-WORKER-0169](../requirements/worker.md#req-worker-0169)
- [REQ-WORKER-0170](../requirements/worker.md#req-worker-0170)
- [REQ-WORKER-0173](../requirements/worker.md#req-worker-0173)

This section defines the **single** node-local secure store used by the worker for orchestrator-issued secrets (pull credentials, orchestrator bearer token, agent tokens, and capability leases).

Scope and boundaries:

- The secure store MUST be host-only and MUST NOT be accessible from inside any sandbox or managed-service container.
- No path, filesystem partition, or volume that is part of the secure store MUST be mounted into any container.
- The secure store MUST NOT be the Worker Telemetry API SQLite database and MUST be distinct from `${storage.state_dir}/telemetry/telemetry.db`.
- No API (including Worker Telemetry API endpoints) MUST query or expose the secure store.

Backing and location:

- The secure store ciphertext MUST be stored under `${storage.state_dir}/secrets/` (or `/var/lib/cynode/state/secrets/` when `storage.state_dir` is unset).
- Store files MUST be owned by the worker / Node Manager user and MUST use permissions:
  - files: `0600`
  - directories (when used): `0700`

Encryption at rest:

- All secret values persisted to disk MUST be encrypted at rest before being written and decrypted only when needed by the worker.
- Default (post-quantum): The worker MUST use a post-quantum key encapsulation mechanism to protect the key material used for encryption at rest (e.g. NIST FIPS 203 ML-KEM), with a strong symmetric AEAD for the ciphertext (e.g. AES-256-GCM).
- Fallback: When the post-quantum KEM is not available or not permitted (e.g. in a FIPS-only environment where the validated cryptographic module does not yet include ML-KEM), the worker MUST use only a FIPS-approved symmetric AEAD (e.g. AES-256-GCM).
- Each record MUST use a distinct nonce.
- The secure store master key MUST NOT be stored in plaintext on disk and MUST NOT be logged.

Master key acquisition precedence:

The worker MUST obtain a single 256-bit master key using the first available source in this order:

1. TPM-sealed key (when supported and configured).
2. OS key store.
3. System service credentials (e.g. systemd credential).
4. Environment variable fallback `CYNODE_SECURE_STORE_MASTER_KEY_B64` (base64-encoded 32 bytes).
   This is an emergency fallback only.

When `CYNODE_SECURE_STORE_MASTER_KEY_B64` is used:

- The worker MUST fail closed if the env var is invalid base64 or the decoded length is not exactly 32 bytes.
- The worker MUST NOT log the env var value, decoded bytes, or derived key material.
- The worker MUST NOT pass the env var into any container.
- The worker MUST emit a startup warning indicating the backend `env_b64` and a remediation hint.

FIPS mode:

- When the host system is configured for FIPS mode, the worker MUST use only FIPS-approved algorithms and MUST use FIPS-validated cryptographic modules where required by the platform.

Go 1.26 secure secret handling (implementation requirement):

- The worker MUST use `runtime/secret` (Go 1.26, via `GOEXPERIMENT=runtimesecret` at build time) when available to wrap code that handles the master key or decrypted plaintext secrets so temporaries are erased before returning.
  [REQ-STANDS-0133](../requirements/stands.md#req-stands-0133)
- When `runtime/secret` is not available, the worker MUST use best-effort secure erasure of temporaries (e.g. zeroing buffers) before returning from code paths that handle the master key or decrypted plaintext secrets.

### Secure Store Process Boundary

- Spec ID: `CYNAI.WORKER.SecureStoreProcessBoundary` <a id="spec-cynai-worker-securestoreprocessboundary"></a>

#### Secure Store Process Boundary Requirements Traces

- [REQ-WORKER-0168](../requirements/worker.md#req-worker-0168)
- [REQ-WORKER-0169](../requirements/worker.md#req-worker-0169)

This section defines the required trusted boundary between secure store writers and readers.

#### Trusted Boundary Definitions

- The secure store writer is the component that applies node configuration (typically the Node Manager).
- The secure store reader for managed agent proxy credentials is the worker internal proxy handler.

Required behavior:

- When the Node Manager and Worker API run in the same process, that process boundary is the trusted boundary.
- When the Node Manager and Worker API run as separate processes, the implementation MUST enforce a trusted boundary so that:
  - only the config-apply component can write secrets, and
  - only the worker proxy component can read the secrets required for proxying.
- In all deployment models, the implementation MUST document which component writes the secure store, which component reads it, and how the trusted boundary is enforced.

## Required Node Configuration

The orchestrator MUST configure nodes with:

- Orchestrator endpoints
  - worker API target URLs and registration endpoints
  - service discovery endpoints, if available
- Sandbox image registries (rank-ordered list)
  - per-registry URL and pull credentials
  - policies such as digest pinning requirements
- Model cache
  - cache endpoint URL and pull credentials
  - policies such as public internet download disablement
- Network and security policy
  - allowed egress to orchestrator-provided services
  - certificate trust material for HTTPS
