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
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
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
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0109"></a>
- **REQ-WORKER-0110:** The node MUST NOT require inbound SSH access to sandboxes for command execution.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0110"></a>
- **REQ-WORKER-0111:** The node SHOULD use container runtime primitives (create, exec, copy) to implement sandbox operations.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0111"></a>
- **REQ-WORKER-0112:** The node MUST stream sandbox stdout and stderr back to the orchestrator for logging and debugging.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0112"></a>
- **REQ-WORKER-0113:** The node MUST associate sandbox containers with `task_id` and `job_id` for auditing and cleanup.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0113"></a>
- **REQ-WORKER-0114:** The node MUST support an execution mode where sandbox jobs can call a node-local inference endpoint without leaving the node.
  [CYNAI.WORKER.NodeLocalInference](../tech_specs/node.md#spec-cynai-worker-nodelocalinference)
  <a id="req-worker-0114"></a>
- **REQ-WORKER-0115:** The node MUST keep Ollama access private to the node and MUST NOT require exposing Ollama on a public interface.
  [CYNAI.WORKER.NodeLocalInference](../tech_specs/node.md#spec-cynai-worker-nodelocalinference)
  <a id="req-worker-0115"></a>
- **REQ-WORKER-0116:** Each node SHOULD run a node-local MCP server that exposes sandbox operations for that node.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0116"></a>
- **REQ-WORKER-0117:** The node MCP server MUST be reachable only by the orchestrator, not by arbitrary clients.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0117"></a>
- **REQ-WORKER-0118:** The orchestrator SHOULD register each node MCP server with an allowlist.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0118"></a>
- **REQ-WORKER-0119:** Sandbox operations MUST be audited with `task_id` context.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0119"></a>
- **REQ-WORKER-0120:** Node startup YAML MUST NOT be treated as the source of truth for global policy.
  [CYNAI.WORKER.NodeStartupYaml](../tech_specs/node.md#spec-cynai-worker-nodestartupyaml)
  <a id="req-worker-0120"></a>
- **REQ-WORKER-0121:** Node startup YAML MAY impose stricter local constraints than the orchestrator requests.
  [CYNAI.WORKER.NodeStartupYaml](../tech_specs/node.md#spec-cynai-worker-nodestartupyaml)
  <a id="req-worker-0121"></a>
- **REQ-WORKER-0122:** If a local constraint prevents fulfilling an orchestrator request, the node MUST refuse the request and report the reason.
  [CYNAI.WORKER.NodeStartupYaml](../tech_specs/node.md#spec-cynai-worker-nodestartupyaml)
  <a id="req-worker-0122"></a>
- **REQ-WORKER-0123:** A node MAY be configured to run no Ollama container.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0123"></a>
- **REQ-WORKER-0124:** A sandbox-only node MUST still run the worker API and Node Manager.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0124"></a>
- **REQ-WORKER-0125:** The orchestrator MUST be able to schedule sandbox execution on sandbox-only nodes.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0125"></a>
- **REQ-WORKER-0126:** Sandbox-only nodes MUST follow the same credential handling and isolation rules as other nodes.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0126"></a>
- **REQ-WORKER-0127:** The node MUST NOT expose service credentials to sandbox containers.
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0127"></a>
- **REQ-WORKER-0128:** The node SHOULD store credentials in a local secure store (root-owned file with strict permissions or OS key store).
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0128"></a>
- **REQ-WORKER-0129:** The orchestrator SHOULD issue least-privilege pull credentials for node operations that require pulls.
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0129"></a>
- **REQ-WORKER-0130:** Credentials SHOULD be short-lived where possible and SHOULD support rotation.
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0130"></a>

- **REQ-WORKER-0131:** Secrets MUST be short-lived where possible and MUST NOT be exposed to sandbox containers.
  [CYNAI.WORKER.PayloadSecurity](../tech_specs/node_payloads.md#spec-cynai-worker-payloadsecurity)
  <a id="req-worker-0131"></a>
- **REQ-WORKER-0132:** Nodes MUST store secrets only in a node-local secure store.
  [CYNAI.WORKER.PayloadSecurity](../tech_specs/node_payloads.md#spec-cynai-worker-payloadsecurity)
  <a id="req-worker-0132"></a>
- **REQ-WORKER-0133:** Registry and cache pull credentials SHOULD be issued as short-lived tokens.
  [CYNAI.WORKER.Payload.BootstrapV1](../tech_specs/node_payloads.md#spec-cynai-worker-payload-bootstrap-v1)
  <a id="req-worker-0133"></a>
- **REQ-WORKER-0134:** Tokens SHOULD be rotated by configuration refresh.
  [CYNAI.WORKER.Payload.BootstrapV1](../tech_specs/node_payloads.md#spec-cynai-worker-payload-bootstrap-v1)
  <a id="req-worker-0134"></a>
- **REQ-WORKER-0135:** Nodes MUST report configuration application status back to the orchestrator.
  [CYNAI.WORKER.Payload.ConfigAckV1](../tech_specs/node_payloads.md#spec-cynai-worker-payload-configack-v1)
  <a id="req-worker-0135"></a>
- **REQ-WORKER-0136:** New fields MAY be added to payloads as optional fields.
  [CYNAI.WORKER.Payload.CompatibilityVersioning](../tech_specs/node_payloads.md#spec-cynai-worker-payload-versioning)
  <a id="req-worker-0136"></a>
- **REQ-WORKER-0137:** Fields MUST NOT change meaning within the same `version`.
  [CYNAI.WORKER.Payload.CompatibilityVersioning](../tech_specs/node_payloads.md#spec-cynai-worker-payload-versioning)
  <a id="req-worker-0137"></a>
- **REQ-WORKER-0138:** Nodes SHOULD reject payloads with unsupported `version` values and report a structured error.
  [CYNAI.WORKER.Payload.CompatibilityVersioning](../tech_specs/node_payloads.md#spec-cynai-worker-payload-versioning)
  <a id="req-worker-0138"></a>

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
  [CYNAI.WORKER.NodeLocalAgentSandboxControl](../tech_specs/node.md#spec-cynai-worker-nodelocalagentsandboxcontrol)
  <a id="req-worker-0154"></a>
- **REQ-WORKER-0155:** Direct sandbox tool calls to a node MUST be authorized using short-lived, least-privilege capability leases issued by the orchestrator.
  The node MUST validate the lease, enforce tool allowlists and task scoping, and MUST fail closed when required context is missing or invalid.
  [CYNAI.WORKER.NodeLocalAgentSandboxControl](../tech_specs/node.md#spec-cynai-worker-nodelocalagentsandboxcontrol)
  <a id="req-worker-0155"></a>
- **REQ-WORKER-0156:** The node MUST audit direct sandbox tool calls made through the low-latency control path and MUST make audit records available to the orchestrator for centralized retention and inspection.
  [CYNAI.WORKER.NodeLocalAgentSandboxControl](../tech_specs/node.md#spec-cynai-worker-nodelocalagentsandboxcontrol)
  <a id="req-worker-0156"></a>
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
