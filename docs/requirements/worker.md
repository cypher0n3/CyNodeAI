# WORKER Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `WORKER` domain.
It covers worker-node behavior and the worker API contract for job execution and reporting.

## 2 Requirements

- **REQ-WORKER-0001:** Worker API: bearer token auth; node validates token; sandbox via container runtime; no orchestrator credentials in containers; bounded logs; no secrets in logs.
  [CYNAI.WORKER.WorkerApiAuth](../tech_specs/worker_api.md#spec-cynai-worker-workerauth)
  [CYNAI.WORKER.SandboxExecution](../tech_specs/worker_api.md#spec-cynai-worker-sandboxexec)
  <a id="req-worker-0001"></a>
- **REQ-WORKER-0002:** Node exposes worker API for sandbox lifecycle; no inbound SSH; container runtime primitives for sandbox ops.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/worker_node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0002"></a>
- **REQ-WORKER-0003:** Worker Telemetry: node MUST persist operational telemetry locally and expose an orchestrator-authenticated API for querying node logs, system info, and container inventory/state with bounded responses and retention.
  [CYNAI.WORKER.Doc.WorkerTelemetryApi](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-doc-workertelemetryapi)
  <a id="req-worker-0003"></a>
- **REQ-WORKER-0100:** The orchestrator MUST call the Worker API using a bearer token.
  [CYNAI.WORKER.WorkerApiAuth](../tech_specs/worker_api.md#spec-cynai-worker-workerauth)
  <a id="req-worker-0100"></a>
- **REQ-WORKER-0101:** The node MUST validate the token and reject invalid or expired tokens.
  [CYNAI.WORKER.WorkerApiAuth](../tech_specs/worker_api.md#spec-cynai-worker-workerauth)
  <a id="req-worker-0101"></a>
- **REQ-WORKER-0102:** Tokens MUST be treated as secrets and MUST NOT be logged.
  [CYNAI.WORKER.WorkerApiAuth](../tech_specs/worker_api.md#spec-cynai-worker-workerauth)
  <a id="req-worker-0102"></a>
- **REQ-WORKER-0103:** Nodes MUST support sandbox execution using a container runtime (Podman preferred).
  [CYNAI.WORKER.SandboxExecution](../tech_specs/worker_api.md#spec-cynai-worker-sandboxexec)
  <a id="req-worker-0103"></a>
- **REQ-WORKER-0104:** Nodes MUST NOT expose orchestrator-provided credentials to sandbox containers.
  [CYNAI.WORKER.SandboxExecution](../tech_specs/worker_api.md#spec-cynai-worker-sandboxexec)
  <a id="req-worker-0104"></a>
- **REQ-WORKER-0105:** Nodes MUST apply basic safety limits for sandbox execution.
  [CYNAI.WORKER.SandboxExecution](../tech_specs/worker_api.md#spec-cynai-worker-sandboxexec)
  <a id="req-worker-0105"></a>
- **REQ-WORKER-0106:** Worker API implementations MUST bound stdout/stderr size.
  [CYNAI.WORKER.LoggingOutputLimits](../tech_specs/worker_api.md#spec-cynai-worker-loglimits)
  <a id="req-worker-0106"></a>
- **REQ-WORKER-0107:** When truncation occurs, the response MUST indicate it using `truncated.stdout` and `truncated.stderr`.
  [CYNAI.WORKER.LoggingOutputLimits](../tech_specs/worker_api.md#spec-cynai-worker-loglimits)
  <a id="req-worker-0107"></a>
- **REQ-WORKER-0108:** Secrets MUST NOT be written to logs.
  [CYNAI.WORKER.LoggingOutputLimits](../tech_specs/worker_api.md#spec-cynai-worker-loglimits)
  <a id="req-worker-0108"></a>

- **REQ-WORKER-0109:** The node MUST expose a worker API that the orchestrator can call to manage sandbox lifecycle and execution.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/worker_node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0109"></a>
- **REQ-WORKER-0110:** The node MUST NOT require inbound SSH access to sandboxes for command execution.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/worker_node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0110"></a>
- **REQ-WORKER-0111:** The node SHOULD use container runtime primitives (create, exec, copy) to implement sandbox operations.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/worker_node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0111"></a>
- **REQ-WORKER-0112:** The node MUST stream sandbox stdout and stderr back to the orchestrator for logging and debugging.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/worker_node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0112"></a>
- **REQ-WORKER-0113:** The node MUST associate sandbox containers with `task_id` and `job_id` for auditing and cleanup.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/worker_node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0113"></a>
- **REQ-WORKER-0114:** The node MUST support an execution mode where sandbox jobs can call a node-local inference endpoint without leaving the node.
  [CYNAI.WORKER.NodeLocalInference](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalinference)
  [CYNAI.STANDS.PortsAndEndpoints](../tech_specs/ports_and_endpoints.md#spec-cynai-stands-portsandendpoints)
  <a id="req-worker-0114"></a>
- **REQ-WORKER-0115:** The node MUST keep Ollama access private to the node and MUST NOT require exposing Ollama on a public interface.
  [CYNAI.WORKER.NodeLocalInference](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalinference)
  [CYNAI.STANDS.PortsAndEndpoints](../tech_specs/ports_and_endpoints.md#spec-cynai-stands-portsandendpoints)
  <a id="req-worker-0115"></a>
- **REQ-WORKER-0116:** Each node SHOULD run a node-local MCP server that exposes sandbox operations for that node.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/worker_node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0116"></a>
- **REQ-WORKER-0117:** The node MCP server MUST be reachable only by the orchestrator, not by arbitrary clients.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/worker_node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0117"></a>
- **REQ-WORKER-0118:** The orchestrator SHOULD register each node MCP server with an allowlist.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/worker_node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0118"></a>
- **REQ-WORKER-0119:** Sandbox operations MUST be audited with `task_id` context.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/worker_node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0119"></a>
- **REQ-WORKER-0120:** Node startup YAML MUST NOT be treated as the source of truth for global policy.
  [CYNAI.WORKER.NodeStartupYaml](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupyaml)
  <a id="req-worker-0120"></a>
- **REQ-WORKER-0121:** Node startup YAML MAY impose stricter local constraints than the orchestrator requests.
  [CYNAI.WORKER.NodeStartupYaml](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupyaml)
  <a id="req-worker-0121"></a>
- **REQ-WORKER-0122:** If a local constraint prevents fulfilling an orchestrator request, the node MUST refuse the request and report the reason.
  [CYNAI.WORKER.NodeStartupYaml](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupyaml)
  <a id="req-worker-0122"></a>
- **REQ-WORKER-0123:** A node MAY be configured to run no Ollama container.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/worker_node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0123"></a>
- **REQ-WORKER-0124:** A sandbox-only node MUST still run the worker API and Node Manager.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/worker_node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0124"></a>
- **REQ-WORKER-0125:** The orchestrator MUST be able to schedule sandbox execution on sandbox-only nodes.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/worker_node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0125"></a>
- **REQ-WORKER-0126:** Sandbox-only nodes MUST follow the same credential handling and isolation rules as other nodes.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/worker_node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0126"></a>
- **REQ-WORKER-0127:** The node MUST NOT expose service credentials to sandbox containers.
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/worker_node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0127"></a>
- **REQ-WORKER-0128:** The node SHOULD store credentials in a local secure store (root-owned file with strict permissions or OS key store).
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/worker_node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0128"></a>
- **REQ-WORKER-0129:** The orchestrator SHOULD issue least-privilege pull credentials for node operations that require pulls.
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/worker_node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0129"></a>
- **REQ-WORKER-0130:** Credentials SHOULD be short-lived where possible and SHOULD support rotation.
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/worker_node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0130"></a>

- **REQ-WORKER-0131:** Secrets MUST be short-lived where possible and MUST NOT be exposed to sandbox containers.
  [CYNAI.WORKER.PayloadSecurity](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payloadsecurity)
  <a id="req-worker-0131"></a>
- **REQ-WORKER-0132:** Nodes MUST store secrets only in a node-local secure store.
  [CYNAI.WORKER.PayloadSecurity](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payloadsecurity)
  <a id="req-worker-0132"></a>
- **REQ-WORKER-0133:** Registry and cache pull credentials SHOULD be issued as short-lived tokens.
  [CYNAI.WORKER.Payload.BootstrapV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-bootstrap-v1)
  <a id="req-worker-0133"></a>
- **REQ-WORKER-0134:** Tokens SHOULD be rotated by configuration refresh.
  [CYNAI.WORKER.Payload.BootstrapV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-bootstrap-v1)
  <a id="req-worker-0134"></a>
- **REQ-WORKER-0135:** Nodes MUST report configuration application status back to the orchestrator.
  [CYNAI.WORKER.Payload.ConfigAckV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configack-v1)
  <a id="req-worker-0135"></a>
- **REQ-WORKER-0136:** New fields MAY be added to payloads as optional fields.
  [CYNAI.WORKER.Payload.CompatibilityVersioning](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-versioning)
  <a id="req-worker-0136"></a>
- **REQ-WORKER-0137:** Fields MUST NOT change meaning within the same `version`.
  [CYNAI.WORKER.Payload.CompatibilityVersioning](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-versioning)
  <a id="req-worker-0137"></a>
- **REQ-WORKER-0138:** Nodes SHOULD reject payloads with unsupported `version` values and report a structured error.
  [CYNAI.WORKER.Payload.CompatibilityVersioning](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-versioning)
  <a id="req-worker-0138"></a>
- **REQ-WORKER-0139:** The node MUST report its Worker API address (`worker_api.base_url`) at registration and in capability reports so the orchestrator can dispatch jobs to the node.
  [CYNAI.WORKER.Payload.CapabilityReportV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-capabilityreport-v1)
  [CYNAI.WORKER.RegistrationAndBootstrap](../tech_specs/worker_node.md#spec-cynai-worker-registrationandbootstrap)
  <a id="req-worker-0139"></a>

- **REQ-WORKER-0140:** The node MUST expose unauthenticated health check endpoints `GET /healthz` and `GET /readyz`.
  [CYNAI.WORKER.WorkerApiHealthChecks](../tech_specs/worker_api.md#spec-cynai-worker-workerapihealthchecks)
  <a id="req-worker-0140"></a>
- **REQ-WORKER-0141:** `GET /healthz` MUST return HTTP 200 with plain text body `ok` when the Worker API HTTP server is running.
  [CYNAI.WORKER.WorkerApiHealthChecks](../tech_specs/worker_api.md#spec-cynai-worker-workerapihealthchecks)
  <a id="req-worker-0141"></a>
- **REQ-WORKER-0142:** `GET /readyz` MUST return HTTP 200 with plain text body `ready` when the node is ready to accept job execution requests, and MUST return HTTP 503 otherwise.
  [CYNAI.WORKER.WorkerApiHealthChecks](../tech_specs/worker_api.md#spec-cynai-worker-workerapihealthchecks)
  <a id="req-worker-0142"></a>

- **REQ-WORKER-0143:** The node MUST implement `POST /v1/worker/jobs:run` with the request and response payload shapes defined in the Worker API tech spec.
  [CYNAI.WORKER.WorkerApiRunJobSyncV1](../tech_specs/worker_api.md#spec-cynai-worker-workerapirunjobsync-v1)
  <a id="req-worker-0143"></a>
- **REQ-WORKER-0144:** The node MUST enforce job timeouts using the precedence and defaulting rules defined in the Worker API tech spec.
  [CYNAI.WORKER.WorkerApiRunJobSyncV1](../tech_specs/worker_api.md#spec-cynai-worker-workerapirunjobsync-v1)
  <a id="req-worker-0144"></a>
- **REQ-WORKER-0145:** The node MUST enforce the Worker API request body size limit rules defined in the Worker API tech spec and MUST reject oversized requests with HTTP 413.
  [CYNAI.WORKER.WorkerApiRequestSizeLimits](../tech_specs/worker_api.md#spec-cynai-worker-workerapirequestsizelimits)
  <a id="req-worker-0145"></a>
- **REQ-WORKER-0146:** The node MUST enforce stdout and stderr capture limits for `POST /v1/worker/jobs:run` using the defaults and truncation behavior defined in the Worker API tech spec.
  [CYNAI.WORKER.WorkerApiStdIoCaptureLimits](../tech_specs/worker_api.md#spec-cynai-worker-workerapistdiocapturelimits)
  <a id="req-worker-0146"></a>
- **REQ-WORKER-0147:** When truncation occurs, the node MUST truncate by bytes, preserve valid UTF-8, and set `truncated.stdout` and `truncated.stderr` flags as defined in the Worker API tech spec.
  [CYNAI.WORKER.WorkerApiStdIoCaptureLimits](../tech_specs/worker_api.md#spec-cynai-worker-workerapistdiocapturelimits)
  <a id="req-worker-0147"></a>
- **REQ-WORKER-0148:** The node MUST NOT attempt pattern-based secret redaction of sandbox stdout and stderr, and MUST rely on sandbox environment credential handling to prevent secret exposure.
  [CYNAI.WORKER.WorkerApiSecretHandling](../tech_specs/worker_api.md#spec-cynai-worker-workerapisecrethandling)
  <a id="req-worker-0148"></a>
- **REQ-WORKER-0149:** The node MUST report to the orchestrator that a job is in progress once the sandbox process (e.g. SBA) has accepted the job, and MUST report completion and result when the job ends.
  The node MUST retain the job result (e.g. in node-local SQLite) until the result has been successfully persisted to the orchestrator database, and MUST NOT clear or delete the result until persistence is confirmed.
  [CYNAI.WORKER.JobLifecycleResultPersistence](../tech_specs/worker_api.md#spec-cynai-worker-joblifecycleresultpersistence)
  <a id="req-worker-0149"></a>
- **REQ-WORKER-0150:** The Worker API MUST support creating a session sandbox (long-running container) associated with a task or session identifier, and executing multiple commands in that same container (send command, get result, repeat) for longer-running work.
  [CYNAI.WORKER.SessionSandbox](../tech_specs/worker_api.md#spec-cynai-worker-sessionsandbox)
  <a id="req-worker-0150"></a>
- **REQ-WORKER-0151:** Session sandboxes MUST have a maximum lifetime or idle timeout; the node MUST terminate and clean up the container when the limit is reached or when the orchestrator explicitly ends the session.
  [CYNAI.WORKER.SessionSandbox](../tech_specs/worker_api.md#spec-cynai-worker-sessionsandbox)
  <a id="req-worker-0151"></a>
- **REQ-WORKER-0152:** The node MUST associate session sandbox containers with `task_id` and a stable session identifier for auditing and cleanup.
  [CYNAI.WORKER.SessionSandbox](../tech_specs/worker_api.md#spec-cynai-worker-sessionsandbox)
  <a id="req-worker-0152"></a>
- **REQ-WORKER-0153:** The Worker API MUST support an interactive PTY mode for session sandboxes so the orchestrator can exchange a bidirectional terminal byte stream and resize events without requiring inbound SSH or network access to the sandbox.
  [CYNAI.WORKER.SessionSandboxPty](../tech_specs/worker_api.md#spec-cynai-worker-sessionsandboxpty)
  <a id="req-worker-0153"></a>
- **REQ-WORKER-0154:** The node MUST support a low-latency control path for sandbox operations when an AI agent runtime is co-located on the same host as the worker node.
  This control path SHOULD allow the agent runtime to interact directly with node-hosted sandbox tools without requiring the orchestrator to route every call.
  [CYNAI.WORKER.NodeLocalAgentSandboxControl](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalagentsandboxcontrol)
  <a id="req-worker-0154"></a>
- **REQ-WORKER-0155:** Direct sandbox tool calls to a node MUST be authorized using short-lived, least-privilege capability leases issued by the orchestrator.
  The node MUST validate the lease, enforce tool allowlists and task scoping, and MUST fail closed when required context is missing or invalid.
  [CYNAI.WORKER.NodeLocalAgentSandboxControl](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalagentsandboxcontrol)
  <a id="req-worker-0155"></a>
- **REQ-WORKER-0156:** The node MUST audit direct sandbox tool calls made through the low-latency control path and MUST make audit records available to the orchestrator for centralized retention and inspection.
  [CYNAI.WORKER.NodeLocalAgentSandboxControl](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalagentsandboxcontrol)
  <a id="req-worker-0156"></a>
- **REQ-WORKER-0160:** The worker node MUST support orchestrator-directed managed service containers (long-lived service containers distinct from per-job sandboxes) and MUST reconcile desired state delivered via node configuration.
  [CYNAI.WORKER.ManagedServiceContainers](../tech_specs/worker_node.md#spec-cynai-worker-managedservicecontainers)
  [CYNAI.WORKER.Payload.ConfigurationV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1)
  <a id="req-worker-0160"></a>
- **REQ-WORKER-0161:** The worker node MUST report observed state and worker-mediated endpoint(s) for managed services to the orchestrator and MUST update the report when state changes.
  [CYNAI.WORKER.Payload.CapabilityReportV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-capabilityreport-v1)
  [CYNAI.WORKER.ManagedServiceContainers](../tech_specs/worker_node.md#spec-cynai-worker-managedservicecontainers)
  <a id="req-worker-0161"></a>
- **REQ-WORKER-0162:** The Worker API MUST provide a bidirectional proxy for managed agent runtimes so (1) the orchestrator can reach managed agents through the Worker API and (2) managed agents can reach orchestrator control surfaces (MCP gateway, callbacks) through the Worker API without direct orchestrator network access.
  [CYNAI.WORKER.ManagedAgentProxyBidirectional](../tech_specs/worker_api.md#spec-cynai-worker-managedagentproxy)
  [CYNAI.WORKER.WorkerProxyBidirectionalManagedAgents](../tech_specs/worker_node.md#spec-cynai-worker-proxybidirectional)
  <a id="req-worker-0162"></a>
- **REQ-WORKER-0163:** Agent-to-orchestrator proxy endpoints exposed by the Worker API MUST be bound only to loopback or a Unix domain socket, MUST authenticate using orchestrator-issued agent credentials or capability leases, and MUST emit audit records sufficient to attribute actions to agent identity and task context.
  [CYNAI.WORKER.ManagedAgentProxyBidirectional](../tech_specs/worker_api.md#spec-cynai-worker-managedagentproxy)
  [CYNAI.MCPGAT.EdgeEnforcementMode](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-edgeenforcement)
  <a id="req-worker-0163"></a>
- **REQ-WORKER-0164:** The worker MUST hold orchestrator-issued agent tokens (and capability leases when used) and MUST attach the appropriate credential when forwarding agent-originated requests to the orchestrator.
  The worker MUST NOT pass agent tokens or other orchestrator-issued secrets to agent containers or to agents; agents MUST NOT be given tokens or secrets directly.
  [CYNAI.WORKER.WorkerProxyBidirectionalManagedAgents](../tech_specs/worker_node.md#spec-cynai-worker-proxybidirectional)
  [CYNAI.WORKER.AgentTokensWorkerHeldOnly](../tech_specs/worker_node.md#spec-cynai-worker-agenttokensworkerheldonly)
  [CYNAI.WORKER.AgentTokenStorageAndLifecycle](../tech_specs/worker_node.md#spec-cynai-worker-agenttokenstorageandlifecycle)
  [CYNAI.WORKER.Payload.ConfigurationV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1)
  [CYNAI.MCPGAT.AgentScopedTokens](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-agentscopedtokens)
  [CYNAI.MCPGAT.AgentTokensWorkerProxyOnly](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-agenttokensworkerproxyonly)
  <a id="req-worker-0164"></a>
- **REQ-WORKER-0165:** Nodes MUST store orchestrator-issued secrets in a node-local secure store and MUST encrypt those secrets at rest when persisted to disk.
  [CYNAI.WORKER.NodeLocalSecureStore](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalsecurestore)
  <a id="req-worker-0165"></a>
- **REQ-WORKER-0166:** The node-local secure store master key MUST NOT be stored in plaintext on disk and MUST NOT be written to logs.
  [CYNAI.WORKER.NodeLocalSecureStore](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalsecurestore)
  <a id="req-worker-0166"></a>
- **REQ-WORKER-0167:** Nodes MUST support master key acquisition using a deterministic precedence order and MUST support an emergency environment variable fallback.
  Nodes MUST emit a startup warning when using the env var fallback or any backend weaker than TPM, OS key store, or system service credentials.
  [CYNAI.WORKER.NodeLocalSecureStore](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalsecurestore)
  <a id="req-worker-0167"></a>
- **REQ-WORKER-0168:** Nodes MUST NOT mount or expose any part of the node-local secure store (including ciphertext files) into sandbox containers or managed-service containers.
  [CYNAI.WORKER.NodeLocalSecureStore](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalsecurestore)
  <a id="req-worker-0168"></a>
- **REQ-WORKER-0169:** The node-local secure store MUST be distinct from the Worker Telemetry API SQLite database and MUST NOT be exposed by any API surface.
  [CYNAI.WORKER.NodeLocalSecureStore](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalsecurestore)
  <a id="req-worker-0169"></a>
- **REQ-WORKER-0170:** When the host system is configured for FIPS mode, the worker MUST use only FIPS-approved cryptographic algorithms and MUST use FIPS-validated cryptographic modules where required by the platform.
  [CYNAI.WORKER.NodeLocalSecureStore](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalsecurestore)
  <a id="req-worker-0170"></a>
- **REQ-WORKER-0171:** When node configuration includes `managed_services.services[].orchestrator.agent_token_ref`, the worker MUST resolve the reference to an agent token during configuration apply, MUST fail closed on resolution failure, and MUST NOT expose the reference or resolved token to any agent container or agent runtime.
  [CYNAI.WORKER.Payload.AgentTokenRef](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-agenttokenref)
  [CYNAI.WORKER.AgentTokenRefResolution](../tech_specs/worker_node.md#spec-cynai-worker-agenttokenrefresolution)
  <a id="req-worker-0171"></a>
- **REQ-WORKER-0172:** When the Node Manager and Worker API run as separate processes, the node MUST enforce a trusted boundary for the node-local secure store and MUST document which component writes secrets and which component reads them for proxying.
  [CYNAI.WORKER.SecureStoreProcessBoundary](../tech_specs/worker_node.md#spec-cynai-worker-securestoreprocessboundary)
  <a id="req-worker-0172"></a>
- **REQ-WORKER-0173:** Encryption at rest for the node-local secure store MUST use a post-quantum key encapsulation mechanism when permitted by the platform and MUST fall back to a FIPS-approved symmetric AEAD when the post-quantum mechanism is not available or not permitted.
  [CYNAI.WORKER.NodeLocalSecureStore](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalsecurestore)
  <a id="req-worker-0173"></a>
- **REQ-WORKER-0200:** Worker Telemetry API MUST require bearer token authentication for all telemetry endpoints.
  [CYNAI.WORKER.TelemetryApiAuth](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetryauth)
  <a id="req-worker-0200"></a>
- **REQ-WORKER-0201:** Telemetry API bearer tokens MUST be treated as secrets and MUST NOT be logged.
  [CYNAI.WORKER.TelemetryApiAuth](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetryauth)
  <a id="req-worker-0201"></a>
- **REQ-WORKER-0210:** Nodes MUST maintain a node-local SQLite database used to index and query telemetry for the Worker Telemetry API.
  [CYNAI.WORKER.TelemetryStorageSqlite](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrystorage-sqlite)
  <a id="req-worker-0210"></a>
- **REQ-WORKER-0211:** The telemetry SQLite database MUST be located at `${storage.state_dir}/telemetry/telemetry.db` (or `/var/lib/cynode/state/telemetry/telemetry.db` when `storage.state_dir` is unset).
  [CYNAI.WORKER.TelemetryStorageSqlite](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrystorage-sqlite)
  <a id="req-worker-0211"></a>
- **REQ-WORKER-0212:** Nodes MUST implement the telemetry SQLite schema defined by the Worker Telemetry API tech spec and MUST apply schema migrations on startup.
  [CYNAI.WORKER.TelemetryStorageSqlite](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrystorage-sqlite)
  <a id="req-worker-0212"></a>
- **REQ-WORKER-0220:** Nodes MUST enforce bounded retention for telemetry data so disk usage is controlled.
  [CYNAI.WORKER.TelemetryRetention](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetryretention)
  <a id="req-worker-0220"></a>
- **REQ-WORKER-0221:** Nodes MUST enforce retention on startup and at least once per hour while running.
  [CYNAI.WORKER.TelemetryRetention](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetryretention)
  <a id="req-worker-0221"></a>
- **REQ-WORKER-0222:** Nodes MUST perform SQLite vacuuming for the telemetry database at least once per day.
  [CYNAI.WORKER.TelemetryRetention](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetryretention)
  <a id="req-worker-0222"></a>
- **REQ-WORKER-0230:** Nodes MUST implement the Worker Telemetry API endpoints defined by the Worker Telemetry API tech spec.
  [CYNAI.WORKER.TelemetryApiSurfaceV1](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrysurface-v1)
  <a id="req-worker-0230"></a>
- **REQ-WORKER-0231:** Nodes MUST provide a node info endpoint that returns build and platform information and the last known capability report when available.
  [CYNAI.WORKER.TelemetryApiSurfaceV1](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrysurface-v1)
  <a id="req-worker-0231"></a>
- **REQ-WORKER-0232:** Nodes MUST provide a node stats endpoint that returns a point-in-time resource snapshot.
  [CYNAI.WORKER.TelemetryApiSurfaceV1](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrysurface-v1)
  <a id="req-worker-0232"></a>
- **REQ-WORKER-0233:** Nodes MUST provide container inventory endpoints that support filtering by `kind`, `status`, `task_id`, and `job_id`.
  [CYNAI.WORKER.TelemetryApiSurfaceV1](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrysurface-v1)
  <a id="req-worker-0233"></a>
- **REQ-WORKER-0234:** Nodes MUST provide a log query endpoint that supports time range filtering and pagination, and enforces strict response size limits.
  [CYNAI.WORKER.TelemetryApiSurfaceV1](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrysurface-v1)
  [CYNAI.WORKER.TelemetryLogQueryV1](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrylogquery-v1)
  <a id="req-worker-0234"></a>
- **REQ-WORKER-0240:** The log query endpoint MUST require a source filter (service source or container id) and MUST reject unbounded queries.
  [CYNAI.WORKER.TelemetryLogQueryV1](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrylogquery-v1)
  <a id="req-worker-0240"></a>
- **REQ-WORKER-0241:** The log query endpoint MUST enforce a maximum response body size of 1 MiB and MUST indicate truncation in the response.
  [CYNAI.WORKER.TelemetryLogQueryV1](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrylogquery-v1)
  <a id="req-worker-0241"></a>
- **REQ-WORKER-0242:** Telemetry responses MUST NOT include secrets and MUST NOT leak bearer tokens.
  [CYNAI.WORKER.TelemetryApiAuth](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetryauth)
  <a id="req-worker-0242"></a>
- **REQ-WORKER-0243:** Nodes MUST associate telemetry records for sandbox containers with `task_id` and `job_id` when known.
  [CYNAI.WORKER.TelemetryStorageSqlite](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrystorage-sqlite)
  <a id="req-worker-0243"></a>
- **REQ-WORKER-0250:** The worker node MUST map container paths `/workspace` and `/job` to real, persistent filesystem paths on the host (bind mounts).
  The host parent directory under which the node creates these paths MUST be configurable by the user via the node startup configuration.
  [CYNAI.WORKER.SandboxWorkspaceJobMounts](../tech_specs/worker_node.md#spec-cynai-worker-sandboxworkspacejobmounts)
  <a id="req-worker-0250"></a>
- **REQ-WORKER-0251:** When the container runtime supports rootless execution (e.g. Podman), the worker node MUST use rootless operations for sandbox containers (MUST NOT run sandbox containers as root).
  The operator MAY override via node startup config (e.g. `sandbox.rootless: false`) only as a documented exception path when the runtime allows.
  [CYNAI.WORKER.SandboxRootless](../tech_specs/worker_node.md#spec-cynai-worker-sandboxrootless)
  <a id="req-worker-0251"></a>
- **REQ-WORKER-0252:** The worker node MUST perform startup checks to verify it can deploy containers and meet readiness prerequisites before reporting ready (e.g. before `GET /readyz` returns 200).
  [CYNAI.WORKER.NodeStartupChecks](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupchecks)
  [CYNAI.WORKER.WorkerApiHealthChecks](../tech_specs/worker_api.md#spec-cynai-worker-workerapihealthchecks)
  <a id="req-worker-0252"></a>
- **REQ-WORKER-0253:** The node MUST start the Worker API and contact the orchestrator with its capabilities bundle (registration and capability report) before starting any local inference (OLLAMA) container.
  The node MUST NOT start the OLLAMA (or equivalent) container until the orchestrator has acknowledged registration and returned node configuration that instructs the node to start the local inference backend (including backend variant, e.g. ROCm for AMD or CUDA for Nvidia, when applicable).
  [CYNAI.WORKER.NodeStartupProcedure](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure)
  [CYNAI.WORKER.RegistrationAndBootstrap](../tech_specs/worker_node.md#spec-cynai-worker-registrationandbootstrap)
  <a id="req-worker-0253"></a>
- **REQ-WORKER-0254:** The node MUST report to the orchestrator when it has become ready (e.g. after applying config and starting Worker API and, when instructed, the local inference container) so that the orchestrator can consider the node as an inference path and start the Project Manager Agent when appropriate.
  This report MAY be the config ack with status applied or a dedicated readiness notification as defined in the tech specs.
  [CYNAI.WORKER.NodeStartupProcedure](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure)
  [CYNAI.WORKER.Payload.ConfigAckV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configack-v1)
  <a id="req-worker-0254"></a>
- **REQ-WORKER-0255:** The node MUST support using an existing OLLAMA (or equivalent) inference service already running on the host when one is present and reachable.
  The node MUST NOT start its own inference container when such an existing service is detected and usable; the node MUST use the existing service instead.
  [CYNAI.WORKER.ExistingInferenceService](../tech_specs/worker_node.md#spec-cynai-worker-existinginferenceservice)
  <a id="req-worker-0255"></a>
- **REQ-WORKER-0256:** The node MUST include in its capability report (at registration and on every check-in) whether it already has a running inference service on the host (e.g. existing OLLAMA or equivalent that the node is using rather than having started).
  This allows the orchestrator to treat the node as inference-capable and to avoid instructing the node to start an inference container when one is already present.
  [CYNAI.WORKER.CapabilityReporting](../tech_specs/worker_node.md#spec-cynai-worker-capabilityreporting)
  [CYNAI.WORKER.Payload.CapabilityReportV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-capabilityreport-v1)
  <a id="req-worker-0256"></a>
