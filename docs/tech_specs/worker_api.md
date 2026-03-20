# Worker API (Node) - Contract and Payloads

- [Document Overview](#document-overview)
- [Scope](#scope)
- [Definitions](#definitions)
- [Versioning](#versioning)
- [Authentication](#authentication)
  - [Applicable Requirements (Authentication)](#applicable-requirements-authentication)
- [HTTPS Transport and Reverse Proxy](#https-transport-and-reverse-proxy)
- [Health Checks](#health-checks)
  - [Health Check Endpoints](#health-check-endpoints)
  - [Health Check Requirements Traces](#health-check-requirements-traces)
- [Error Handling](#error-handling)
- [Worker API Surface (Initial Implementation)](#worker-api-surface-initial-implementation)
  - [Run Job (Synchronous)](#run-job-synchronous)
  - [Job Lifecycle and Result Persistence](#job-lifecycle-and-result-persistence)
  - [Stop Job (User-Directed)](#stop-job-user-directed)
  - [Node-Mediated SBA Result (Sync)](#node-mediated-sba-result-sync)
  - [Node-Mediated Step-Executor Result (Sync)](#node-mediated-step-executor-result-sync)
  - [Session Sandbox (Long-Running)](#session-sandbox-long-running)
  - [Managed Agent Proxy (Bidirectional)](#managed-agent-proxy-bidirectional)
  - [Stop All Orchestrator-Directed (Orchestrator Shutdown)](#stop-all-orchestrator-directed-orchestrator-shutdown)
  - [Session Sandbox PTY (Interactive Terminal Stream)](#session-sandbox-pty-interactive-terminal-stream)
- [Sandbox Execution Requirements (Initial Implementation)](#sandbox-execution-requirements-initial-implementation)
  - [Sandbox Execution Requirements Traces](#sandbox-execution-requirements-traces)
- [Logging and Output Limits](#logging-and-output-limits)
  - [Applicable Requirements (Logging and Output Limits)](#applicable-requirements-logging-and-output-limits)
  - [Request Size Limits (Required)](#request-size-limits-required)
  - [`stdout`/`stderr` Capture Limits (Required)](#stdoutstderr-capture-limits-required)
  - [Secret Handling (Required)](#secret-handling-required)

## Document Overview

This document defines the **Worker API** contract exposed by a node (local worker or cloud worker).
The orchestrator uses this API to dispatch work to a node for sandbox execution and to collect results.

This document is the canonical contract for:

- endpoint paths and methods
- request and response payload shapes
- authentication requirements for orchestrator to call the node

Related specs

- Node responsibilities: [`docs/tech_specs/worker_node.md`](worker_node.md)
- Go API implementation standards: [`docs/tech_specs/go_rest_api_standards.md`](go_rest_api_standards.md)
- Node payloads (bootstrap/config): [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md)

## Scope

The initial implementation requires a single-node happy path where:

- the orchestrator can dispatch one job to one node
- the node runs a sandbox container for that job
- the orchestrator receives a result suitable for storing on `jobs.result`

This spec intentionally defines only the minimum Worker API surface required for the initial implementation.
Additional endpoints (file transfer, streaming logs, async job polling, background processes) may be added later.

## Definitions

- **Worker API**: HTTP API hosted by the node.
- **Job**: a unit of execution associated with `task_id` and `job_id`.
- **Sandbox**: a container used to execute job commands with isolation and limits.

## Versioning

- All Worker API endpoints MUST be served under a stable version prefix: `/v1/`.
- All JSON request and response bodies MUST include a top-level `version` integer.
  - For the initial implementation, `version` MUST be `1`.

## Authentication

The Worker API MUST authenticate all requests (except explicit health checks).

### Applicable Requirements (Authentication)

- Spec ID: `CYNAI.WORKER.WorkerApiAuth` <a id="spec-cynai-worker-workerauth"></a>

#### Traces to Requirements

- [REQ-WORKER-0100](../requirements/worker.md#req-worker-0100)
- [REQ-WORKER-0101](../requirements/worker.md#req-worker-0101)
- [REQ-WORKER-0102](../requirements/worker.md#req-worker-0102)

Token delivery

- The orchestrator MUST deliver the Worker API bearer token to the node via the node configuration payload.
  - See [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md) `node_configuration_payload_v1`.

Initial implementation constraints

- Bearer token is static (delivered via node config; no refresh).
- Component-to-component traffic MUST support HTTP (not HTTPS) for MVP.
  HTTPS MAY be added later without changing endpoint paths or payload shapes.
- When `network_policy` is `restricted`, treat as deny-all (equivalent to `none`).
- CPU, memory, and PIDs limits are not applied; workspace is per-job at `/workspace` per [`docs/tech_specs/sandbox_container.md`](sandbox_container.md).

## HTTPS Transport and Reverse Proxy

- Spec ID: `CYNAI.WORKER.HttpsTransportReverseProxy` <a id="spec-cynai-worker-httpstransportreverseproxy"></a>

The Worker API MAY be deployed behind a containerized nginx reverse proxy for HTTPS transport.
When the Worker API is reached over HTTPS, the orchestrator MUST validate the server certificate when connecting to the node, unless explicitly configured for development (e.g. insecure skip).

Self-signed certificate handling

- When the Worker API is served over HTTPS using a self-signed certificate, the orchestrator has no public CA to validate the server.
- The initial registration data sent from the node to the orchestrator (the capability report or equivalent bundle exchanged at registration) MUST include the worker node's TLS server certificate or public key so that the orchestrator can trust the worker for subsequent HTTPS connections.
- The orchestrator MUST use this trust material (e.g. pin the certificate or add it to the TLS client trust store) when making HTTPS requests to that node's Worker API.
- The canonical place for this trust material is the node capability report; see [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md) capability report schema `tls.worker_api_server_cert_pem`.

## Health Checks

- Spec ID: `CYNAI.WORKER.WorkerApiHealthChecks` <a id="spec-cynai-worker-workerapihealthchecks"></a>

The Worker API MUST expose unauthenticated health checks intended for liveness and readiness probing.
Health check endpoints MUST NOT require a `/v1/` prefix and MUST NOT require a JSON `version` field.

### Health Check Endpoints

- `GET /healthz`
  - returns 200 with plain text body `ok` when the HTTP server is running
- `GET /readyz`
  - returns 200 with plain text body `ready` only when the node has passed [node startup checks](worker_node.md#spec-cynai-worker-nodestartupchecks) and is ready to accept job execution requests
  - returns 503 when the node is not ready (e.g. startup checks not yet passed or failed)

### Health Check Requirements Traces

- [REQ-WORKER-0140](../requirements/worker.md#req-worker-0140)
- [REQ-WORKER-0141](../requirements/worker.md#req-worker-0141)
- [REQ-WORKER-0142](../requirements/worker.md#req-worker-0142)
- [REQ-WORKER-0252](../requirements/worker.md#req-worker-0252)

## Error Handling

Worker API error responses MUST follow the Go REST API error standards:

- Prefer RFC 9457 Problem Details JSON (`application/problem+json`).
- Do not leak secrets in errors.
- Use stable error `type` values where practical.

See [`docs/tech_specs/go_rest_api_standards.md`](go_rest_api_standards.md#spec-cynai-stands-errorfmt).

## Worker API Surface (Initial Implementation)

This section defines the initial endpoint(s) required for orchestrator-to-node job dispatch.

### Run Job (Synchronous)

Run a sandbox job to completion and return a result in the same response.

#### Applicable Requirements (Run Job)

- Spec ID: `CYNAI.WORKER.WorkerApiRunJobSyncV1` <a id="spec-cynai-worker-workerapirunjobsync-v1"></a>

##### Applicable Requirements (Run Job) Requirements Traces

- [REQ-WORKER-0143](../requirements/worker.md#req-worker-0143)
- [REQ-WORKER-0144](../requirements/worker.md#req-worker-0144)

#### Endpoint Details

- `POST /v1/worker/jobs:run`

#### Required Behavior

- The node MUST execute the provided command in a sandbox container.
- The node MUST associate the execution with `task_id` and `job_id` for auditing and cleanup.
- The node MUST enforce a timeout (job-provided or node default) using the rules below.
  The **orchestrator** (e.g. PMA or Project Analyst Agent when dispatching jobs) sets the per-job timeout via `sandbox.timeout_seconds` in the request when it wants to allow long-running work; the node provides a default when not set and enforces a node-specific max.
- The node MUST capture stdout and stderr and include them in the response (subject to truncation limits).
- The node MUST return a deterministic result for the same inputs when execution is deterministic.

Timeout rules (required)

- If `sandbox.timeout_seconds` is provided, the effective timeout MUST be:
  - `min(sandbox.timeout_seconds, node_max_job_timeout_seconds)`
- If `sandbox.timeout_seconds` is not provided, the effective timeout MUST be:
  - `min(node_default_job_timeout_seconds, node_max_job_timeout_seconds)`
- `node_default_job_timeout_seconds` MUST be derived in this order:
  - node startup YAML `sandbox.timeouts.default_seconds`
  - otherwise 3600 seconds (1 hour)
- `node_max_job_timeout_seconds` MUST be derived in this order:
  - node startup YAML `sandbox.timeouts.max_seconds`
  - otherwise 10800 seconds (3 hours)
- If the node configuration payload includes `constraints.max_job_timeout_seconds`, the node MUST further cap the timeout
  by taking the minimum of the values above and `constraints.max_job_timeout_seconds`.

#### Request Body Example

```json
{
  "version": 1,
  "task_id": "00000000-0000-0000-0000-000000000000",
  "job_id": "00000000-0000-0000-0000-000000000000",
  "sandbox": {
    "image": "docker.io/library/bash:latest",
    "command": ["bash", "-lc", "echo hello"],
    "env": {
      "KEY": "VALUE"
    },
    "timeout_seconds": 300,
    "network_policy": "restricted"
  }
}
```

#### Request Fields

- `version` (int, required): must be 1
- `task_id` (uuid string, required)
- `job_id` (uuid string, required)
- `sandbox` (object, required)
  - `image` (string, required): OCI image reference (when no custom registry is configured, images are from Docker Hub, e.g. `docker.io/library/bash:latest` or `bash:latest`)
  - `command` (array of string, required): argv form; must not be empty
  - `env` (object string->string, optional)
  - `timeout_seconds` (int, optional)
  - `network_policy` (string, optional)
    - allowed values for initial implementation: `restricted`, `none`

#### Response Body Example

```json
{
  "version": 1,
  "task_id": "00000000-0000-0000-0000-000000000000",
  "job_id": "00000000-0000-0000-0000-000000000000",
  "status": "completed",
  "exit_code": 0,
  "stdout": "hello\n",
  "stderr": "",
  "started_at": "2026-02-16T12:00:00Z",
  "ended_at": "2026-02-16T12:00:01Z",
  "truncated": {
    "stdout": false,
    "stderr": false
  }
}
```

#### Response Fields

- `version` (int, required): must be 1
- `task_id` (uuid string, required)
- `job_id` (uuid string, required)
- `status` (string, required)
  - allowed values for initial implementation: `completed`, `failed`, `timeout`
- `exit_code` (int, required when `status=completed` or `status=failed`)
- `stdout` (string, required; may be empty)
- `stderr` (string, required; may be empty)
- `started_at` (RFC 3339 UTC string, required)
- `ended_at` (RFC 3339 UTC string, required)
- `truncated` (object, required)
  - `stdout` (boolean)
  - `stderr` (boolean)
- `sba_result` (object, optional): When the job used an SBA runner image, the node MAY include the SBA result contract (e.g. from `/job/result.json`) so the orchestrator can persist it to `jobs.result`.
  Shape: protocol_version, job_id, status, steps, artifacts, failure_code, failure_message per [cynode_sba.md - Result contract](cynode_sba.md#spec-cynai-sbagnt-resultcontract).
- `step_executor_result` (object, optional): When the job used a step-executor runner image (e.g. `cynode-sse`), the node MAY include the step-executor result contract (e.g. from `/job/result.json`) so the orchestrator can persist it to `jobs.result`.
  Shape: protocol_version, job_id, status, steps, failure_code, failure_message per [cynode_step_executor.md - Result contract](cynode_step_executor.md#spec-cynai-stepex-resultcontract).
- `artifacts` (array, optional): When the job used an SBA runner image and the node read `/job/artifacts/`, the node MAY include artifact descriptors or content (e.g. name, content_type, size_bytes, content_base64 or ref) so the orchestrator can persist artifact blobs and refs.
  When the job used a step-executor runner image, the node MAY similarly include artifacts from `/job/artifacts/` if the step executor staged any.
  Shape is implementation-defined but MUST be suitable for orchestrator storage and client retrieval.

#### Status Codes

- 200: job executed (even if failed inside the sandbox; failure is indicated in body `status`)
- 400: invalid request
- 401/403: auth failure
- 413: request too large
- 500: internal node error

### Job Lifecycle and Result Persistence

- Spec ID: `CYNAI.WORKER.JobLifecycleResultPersistence` <a id="spec-cynai-worker-joblifecycleresultpersistence"></a>

Jobs follow a defined lifecycle so the orchestrator can record in-progress state and persist results without relying on a single long-lived connection.
The implementation MAY keep the orchestrator's request open for the full job (e.g. chunked or streaming response with an early in-progress event and a final result), or MAY use an async pattern (e.g. 202 Accepted and node reports status and result via callback or orchestrator polling).

#### Job States

- **accepted**: orchestrator has dispatched the job to the node; node has received it.
- **in_progress**: the sandbox process (e.g. SBA) has accepted the job (read and validated the job spec) and is executing.
- **completed**, **failed**, **timeout**: terminal states; the node has the final result (and, for SBA, the result contract).

#### In-Progress Reporting (Required)

The node MUST report to the orchestrator that the job is **in progress** once the sandbox process has accepted the job (e.g. SBA has read and validated the job spec and signalled in-progress per [cynode_sba.md](cynode_sba.md#spec-cynai-sbagnt-joblifecycle)).
The exact mechanism is implementation-defined: e.g. the node sends an early chunk or event on the same HTTP response, or the node calls an orchestrator endpoint to update job status, or the orchestrator polls the node for status.
The orchestrator MUST be able to mark the job as in progress without holding the request open for the full job duration unless the implementation uses a single streaming response.

#### Completion and Result Reporting

- When the job reaches a terminal state, the node MUST report completion (and the result payload) to the orchestrator.
  The result MUST be suitable for storing in the orchestrator database (e.g. `jobs.result`).
- The node MUST report the job as **failed** or **timeout** to the orchestrator when the sandbox process errors out, crashes, does not report completion, or exceeds the job timeout-so the orchestrator can persist the failure and re-issue or retry the job as policy allows.
  The node MUST NOT leave the job in an unreported or ambiguous state; the orchestrator depends on a terminal state (completed, failed, or timeout) to update task/job state and to decide retries or re-dispatch.

#### Timeout Extension (Required When Extensions Are Supported)

When the SBA (or job) requests a [timeout extension](cynode_sba.md#spec-cynai-sbagnt-timeoutextension) and the node or orchestrator grants it, the **orchestrator** MUST be informed of the new effective deadline (e.g. via job-status update, callback payload, or orchestrator-owned extension grant).
This allows the orchestrator to update its [job timeout tracking](orchestrator.md#spec-cynai-orches-rule-jobtimeouttracking) and scheduled timeout check so it does not mark the job as timed out while the job is within the extended deadline.
The exact mechanism (field on job-status callback, MCP tool response, or orchestrator endpoint) is defined in the Worker API or [MCP tool specifications](mcp_tools/README.md).

#### Result Retention (Required)

The node MUST retain the job result (e.g. in node-local SQLite or equivalent) until the result has been **successfully persisted** by the orchestrator (e.g. uploaded to the orchestrator and written to the database, or accepted by an orchestrator endpoint that performs persistence).
The node MUST NOT clear or delete the job result until persistence is confirmed.
This ensures no result loss if the connection drops after the job completes but before the orchestrator has stored the result.
Retained results SHOULD be subject to node-local retention and disk limits; the implementation SHOULD define cleanup for results that could not be delivered after a timeout or retry limit.

#### Job Lifecycle and Result Persistence Requirements Traces

- [REQ-WORKER-0149](../requirements/worker.md#req-worker-0149)
- [REQ-WORKER-0157](../requirements/worker.md#req-worker-0157)

### Stop Job (User-Directed)

- Spec ID: `CYNAI.WORKER.StopJob` <a id="spec-cynai-worker-stopjob"></a>

The Worker API MUST support a **stop job** (cancel job) operation so the orchestrator can request that a running job be stopped at user direction (e.g. task cancel from gateway, PMA, or slash command).

- **Endpoint:** `POST /v1/worker/jobs:stop` (or equivalent; request body includes `task_id` and `job_id`).
  Authentication: same as Run Job (orchestrator bearer token).
- **Behavior:** The node MUST look up the running job (container or process) for the given `task_id` and `job_id`.
  If no such job is running, the node MUST return success (e.g. 200) with a payload indicating "not running" or "already stopped".
  If the job is running, the node MUST initiate the stop sequence: **Stage 1 (graceful):** send a signal to the SBA process (e.g. SIGTERM) or call an in-band stop hook and allow a grace period (e.g. 30 seconds) for the process to exit.
  **Stage 2 (fallback):** if the process does not exit within the grace period, the node MUST kill the SBA container (e.g. container stop then kill).
  The node MUST then report the job as stopped (e.g. status `canceled`) to the orchestrator and MUST NOT leave the job in a permanent "running" state.
- For jobs that do not use an SBA runner, the node MAY skip the graceful SBA step and proceed to stop the container (SIGTERM then kill after timeout).
- See [REQ-ORCHES-0184](../requirements/orches.md#req-orches-0184) for the requirement; full semantics for user-directed job kill are proposed in a draft spec (not yet canonical).
  The orchestrator invalidates the job's token when the job is stopped so that subsequent SBA tool calls for that job are rejected; see [Task Cancel and Stop Job](orchestrator.md#spec-cynai-orches-taskcancelandstopjob).

### Node-Mediated SBA Result (Sync)

- Spec ID: `CYNAI.WORKER.NodeMediatedSbaResultSync` <a id="spec-cynai-worker-nodemediatedsbaresult-sync"></a>

- When the job uses an SBA runner image and the implementation uses the synchronous Run Job response, the node MUST do the following.
  - **Blocking wait for container exit:** The node MUST start the container (with `/job/` and `/workspace` bind-mounted) and MUST **block** until the container process exits or the job timeout is reached (whichever comes first).
    E.g. a single goroutine or thread waiting on the container runtime's wait primitive; no periodic polling is required.
    The Run Job HTTP request remains open for the full job duration.
    On timeout, the node MUST stop the container and treat the job as timed out.
  - **Long-running jobs:** Holding the connection open for long-running jobs (e.g. 1-3 hours) is not recommended.
    For long-running work, implementations SHOULD use an **async** pattern where the SBA reports status and completion via outbound call (e.g. job callback or status API) so the node does not block the Run Job request for the full duration.
    The node may respond with 202 Accepted and deliver the result when the SBA reports completion.
    If the SBA never reports completion (e.g. crash, hang), the node MUST still report the job as failed or timeout to the orchestrator once the job timeout is reached or the container has exited, so the orchestrator can persist the failure and re-issue or retry as policy allows.
    See [cynode_sba.md - Job lifecycle and status reporting](cynode_sba.md#spec-cynai-sbagnt-joblifecycle).
  - **Read job results after exit:** After the container has exited (or been stopped), the node MUST read `/job/result.json` from the host bind-mount (and optionally `/job/artifacts/`) and MUST derive job status from the container exit code and/or from the SBA result contract when present.
    When the container exits without a valid result (e.g. crash, OOM kill, non-zero exit with missing or invalid `/job/result.json`), the node MUST still report a terminal state (e.g. `status=failed`) to the orchestrator with a result payload that allows the orchestrator to persist the failure (and re-issue or retry the job if policy allows).
  - **Build and return response:** The node MUST build the response body including `sba_result` (when present) and `artifacts` as defined in the Run Job response fields and return them in the same HTTP response to the orchestrator.
  - **Retention:** The node MUST NOT clear or delete the job directory until the response has been sent.
  The node SHOULD retain the job directory until orchestrator persistence is confirmed when the protocol supports it.
  This aligns with [cynode_sba.md - Result and Artifact Delivery](cynode_sba.md#spec-cynai-sbagnt-resultandartifactdelivery) (node-mediated delivery path).

#### See Also (Result Delivery)

- [cynode_sba.md - Job lifecycle and status reporting](cynode_sba.md#spec-cynai-sbagnt-joblifecycle) (SBA in-progress and completion contract)

### Node-Mediated Step-Executor Result (Sync)

- Spec ID: `CYNAI.WORKER.NodeMediatedStepExecutorResultSync` <a id="spec-cynai-worker-nodemediatedstepexecutorresult-sync"></a>

When the job uses a **step-executor runner image** (e.g. image that runs `cynode-sse` as the main process) and the implementation uses the synchronous Run Job response, the node MAY do the following: block until the container exits (or timeout), read `/job/result.json` from the host bind-mount (and optionally `/job/artifacts/`), and include the step-executor result in the response body as `step_executor_result` (and optionally `artifacts`) so the orchestrator can persist it to `jobs.result`.
This mirrors the pattern for [Node-Mediated SBA Result (Sync)](#node-mediated-sba-result-sync); the step executor does not require outbound callbacks for MVP.
See [cynode_step_executor.md - Worker API Integration](cynode_step_executor.md#spec-cynai-stepex-workerapiintegration).

### Session Sandbox (Long-Running)

- Spec ID: `CYNAI.WORKER.SessionSandbox` <a id="spec-cynai-worker-sessionsandbox"></a>

#### Session Sandbox (Long-Running) Requirements Traces

- [REQ-WORKER-0150](../requirements/worker.md#req-worker-0150)
- [REQ-WORKER-0151](../requirements/worker.md#req-worker-0151)
- [REQ-WORKER-0152](../requirements/worker.md#req-worker-0152)

For longer-running tasks, the Worker API MUST support **session sandboxes**: the orchestrator creates a long-running container, then sends multiple commands (exec rounds) to that same container and receives results, so the AI model can continue working on the same problem in the same environment.

#### `SessionSandbox` Scope

- The node exposes operations to: create a session (start container), execute a command in the session (exec), and end the session (stop and remove container).
- Each session is identified by a stable session identifier (e.g. UUID) and MUST be associated with `task_id` for auditing.
- The same container is used for all exec rounds in the session; the workspace persists across rounds.

#### `SessionSandbox` Behavior

- **Create session**: Orchestrator calls an endpoint (e.g. `POST /v1/worker/sessions`) with `task_id`, `session_id`, sandbox image, and optional session lifetime or idle timeout.
  The node starts the container and returns success; the container stays running.
- **Exec in session**: Orchestrator calls an endpoint (e.g. `POST /v1/worker/sessions/{session_id}/exec`) with command and optional timeout.
  The node runs the command via container exec in that session's container and returns stdout, stderr, and exit code.
  Multiple exec calls may be made for the same session.
- **End session**: Orchestrator calls an endpoint (e.g. `POST /v1/worker/sessions/{session_id}/end`) or the node automatically ends the session when max lifetime or idle timeout is reached.
  The node stops and removes the container and reclaims resources.

#### `SessionSandbox` Outcomes

- Session sandboxes MUST have a maximum lifetime or idle timeout; the node MUST terminate the container when the limit is reached or when the orchestrator explicitly ends the session.
- The node MUST associate the session container with `task_id` and the session identifier in logs and telemetry for auditing and cleanup.
- Exact endpoint paths, request/response payloads, and timeout semantics for session create, exec, and end are to be defined in a later revision of this spec or in a dedicated session-sandbox subsection; this Spec Item establishes the required capability and behavior.

See [`docs/tech_specs/sandbox_container.md`](sandbox_container.md#spec-cynai-sandbx-longrunningsession) for the sandbox contract for long-running sessions.

### Managed Agent Proxy (Bidirectional)

- Spec ID: `CYNAI.WORKER.ManagedAgentProxyBidirectional` <a id="spec-cynai-worker-managedagentproxy"></a>

The Worker API MUST support a bidirectional proxy for worker-managed agent runtimes (for example PMA):

- orchestrator (and user-gateway) -> agent container, and
- agent container -> orchestrator (MCP gateway and control-plane callbacks).

All agent runtimes on a worker (whether managed service or not) MUST be network restricted; all inbound and outbound traffic MUST route through worker proxies.
Violating this violates a security boundary and is not acceptable ([REQ-WORKER-0174](../requirements/worker.md#req-worker-0174)).
Managed agent containers MUST NOT be given direct network access to the orchestrator or any other external endpoint; the orchestrator MUST NOT be given direct network access to agent containers; all traffic flows through the Worker API proxy.

#### Orchestrator to Agent Proxy

Recommended endpoint:

- `POST /v1/worker/managed-services/{service_id}/proxy:http`

Request body (minimum):

- `version` (int, required): must be 1
- `method` (string, required): `GET` | `POST` | `PUT` | `PATCH` | `DELETE`
- `path` (string, required): path to call on the managed service
- `headers` (object, optional): worker-enforced allowlist
- `body_b64` (string, optional): base64 body

Response body (minimum):

- `version` (int, required): must be 1
- `status` (int, required): upstream status
- `headers` (object, optional): sanitized response headers
- `body_b64` (string, optional): base64 response body

Normative constraints:

- The worker MUST allow proxying only to services present in desired state (`managed_services`) and MUST reject unknown `service_id`.
- The worker MUST enforce strict request and response size limits and MUST apply header allowlists.
- The worker MUST audit proxy calls with at least: `service_id`, `service_type`, caller identity (orchestrator), and timing.

#### Agent to Orchestrator Proxy

Agent-to-orchestrator proxy endpoints are used by the managed agent container at runtime.

##### Endpoint Binding

- Per [Unified UDS Path](worker_node.md#spec-cynai-worker-unifiedudspath), all container-facing proxy endpoints MUST use **Unix domain sockets (UDS)** only.
  Loopback TCP is not permitted for agent or sandbox container access; the worker MUST expose proxy endpoints to containers only via UDS (or `http+unix` URLs).
- These endpoints MUST be exposed only on a Unix domain socket.
  They MUST NOT be exposed on a non-loopback network interface or on loopback TCP to the container.
- These endpoints MUST be identity-bound to a single managed agent runtime instance.
  The worker MUST be able to deterministically resolve the calling managed service identity (for example `service_id`) from the binding used.
  The worker MUST NOT rely on any secret being present in the agent container or request to establish identity.
  Unknown, ambiguous, or unresolvable caller identities MUST fail closed.
- The required identity-binding mechanism is **per-service Unix domain socket (UDS)**:
  - For each managed agent runtime instance (identified by `service_id`), the worker MUST create a dedicated UDS listener for the internal proxy operations.
  - The worker MUST mount only that service's UDS into the corresponding managed service container.
  - The worker MUST resolve the calling `service_id` from the specific UDS listener that accepted the connection (socket identity), not from request headers.
  - The managed service container MUST NOT be able to access any other service's internal proxy UDS.
  - If UDS binding is not supported by the selected container runtime configuration, the worker MUST fail the managed service start and report `state=error` with a structured reason in `managed_services_status`.

Minimum proxy operations:

- **MCP gateway proxy**
  - `POST /v1/worker/internal/orchestrator/mcp:call`
  - Proxies a request to the orchestrator MCP gateway.
  - The managed agent MUST NOT receive or present an agent token or other secret credential.
  - The orchestrator delivers the agent token to the worker in node configuration.
  - The worker proxy holds the token and attaches it when forwarding to the orchestrator MCP gateway.
  - The worker MUST emit audit records sufficient to attribute actions to the managed agent identity and available request context.
  - Error semantics (minimum):
    - If the worker cannot resolve the calling `service_id` from the binding, return 403 with Problem Details `type=https://cynode.ai/problems/managed-agent-identity-unresolved`.
    - If the worker has no token for the resolved `service_id`, return 503 with Problem Details `type=https://cynode.ai/problems/managed-agent-token-unavailable`.
    - If the resolved token is expired, return 503 with Problem Details `type=https://cynode.ai/problems/managed-agent-token-expired`.

- **Ready/callback proxy**
  - `POST /v1/worker/internal/orchestrator/agent:ready`
  - Proxies readiness/registration callbacks to the orchestrator control-plane.
  - Authentication and auditing requirements are the same as MCP gateway proxy.

See also:

- [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md#spec-cynai-mcpgat-agenttokensworkerproxyonly) (`CYNAI.MCPGAT.AgentTokensWorkerProxyOnly`)
- [`docs/tech_specs/worker_node.md`](worker_node.md#spec-cynai-worker-agenttokensworkerheldonly) (`CYNAI.WORKER.AgentTokensWorkerHeldOnly`)
- [`docs/tech_specs/worker_node.md`](worker_node.md#spec-cynai-worker-agenttokenstorageandlifecycle) (`CYNAI.WORKER.AgentTokenStorageAndLifecycle`)

### Stop All Orchestrator-Directed (Orchestrator Shutdown)

- Spec ID: `CYNAI.WORKER.StopAllOrchestratorDirected` <a id="spec-cynai-worker-stopallorchestratordirected"></a>

When the orchestrator shuts down, it MUST notify each registered worker to stop all orchestrator-directed agents and jobs.
The Worker API MUST expose an endpoint (or equivalent authenticated mechanism) that the orchestrator calls to signal the node to stop all orchestrator-directed managed services (including PMA) and all jobs that were dispatched by the orchestrator.

#### Normative Behavior

- The orchestrator MUST call this mechanism for each registered worker that has an active worker API target before the orchestrator process exits.
- The node MUST authenticate the request using the same bearer token contract as other Worker API calls.
- Upon receiving the request, the node MUST stop all orchestrator-directed managed service containers (e.g. PMA) and MUST stop or cancel all running jobs that were dispatched by the orchestrator.
- Exact endpoint path and request/response payload (e.g. `POST /v1/worker/admin/stop-all-orchestrator-directed` with empty or minimal body) are to be defined in a later revision; this Spec Item establishes the required capability and behavior.

See [Orchestrator Shutdown](orchestrator.md#spec-cynai-orches-orchestratorshutdown) and [Orchestrator Shutdown Notification](worker_node.md#spec-cynai-worker-orchestratorshutdownnotification).

#### Stop All Orchestrator-Directed (Orchestrator Shutdown) Requirements Traces

- [REQ-ORCHES-0164](../requirements/orches.md#req-orches-0164)
- [REQ-WORKER-0271](../requirements/worker.md#req-worker-0271)

### Session Sandbox PTY (Interactive Terminal Stream)

- Spec ID: `CYNAI.WORKER.SessionSandboxPty` <a id="spec-cynai-worker-sessionsandboxpty"></a>

This section defines an interactive PTY mode for session sandboxes.
It enables effectively interactive terminal control for long-running coding tasks without requiring inbound SSH or a network server inside the sandbox container.

#### `SessionSandboxPty` Scope

- PTY mode applies only to session sandboxes (long-running containers).
- PTY mode MUST NOT require inbound network connectivity to the sandbox.
  The node implements PTY I/O using container runtime primitives (exec/attach with a TTY), and exposes the PTY stream via the Worker API.
- PTY mode is intended for workflows that are materially harder with exec-round command calls.
  Examples include REPL-driven debugging, terminal UIs, and tools that require a TTY.

#### `SessionSandboxPty` Behavior

The Worker API MUST support the following PTY lifecycle operations for a session sandbox:

- **Open PTY**: establish an interactive PTY stream associated with `task_id` and `session_id`.
- **Send input**: write bytes to PTY stdin.
- **Receive output**: read bytes from the PTY stream (combined stdout/stderr).
- **Resize**: send terminal resize events (cols, rows).
- **Close PTY**: close the interactive stream and release node-side buffers.

#### Recommended API Shape (Subject to Later Endpoint Finalization)

- `POST /v1/worker/sessions/{session_id}/pty:open`
- `POST /v1/worker/sessions/{session_id}/pty:send`
- `POST /v1/worker/sessions/{session_id}/pty:resize`
- `POST /v1/worker/sessions/{session_id}/pty:close`
- `GET /v1/worker/sessions/{session_id}/pty:recv`

#### Recommended Request Fields (Minimum)

- `version` (int, required): must be 1
- `task_id` (uuid string, required)
- `session_id` (uuid string, required)
- `pty_id` (uuid string, required for send/recv/resize/close; returned by open)

#### PTY I/O Encoding

- PTY payloads MUST be treated as bytes, not as UTF-8 text.
- Requests and responses SHOULD encode byte payloads using base64 fields (for example, `data_b64`) so the transport remains JSON-safe.
- The node MUST enforce strict per-message and per-buffer size limits.

#### Timeouts and Lifecycle

- PTY streams MUST be bounded by the session lifetime and idle timeout rules.
- The node MUST terminate PTY streams when the session ends.
- The node SHOULD enforce a PTY idle timeout that is less than or equal to the session idle timeout.

#### Auditing and Safety (PTY)

- All PTY operations MUST be auditable with `task_id`, `session_id`, and `pty_id`.
- Audit records SHOULD include byte counts sent and received, resize events, open/close timestamps, and error codes.
- Secrets MUST NOT be written to logs.
  The node MUST NOT attempt pattern-based redaction of PTY output.
  The correct remediation is to prevent secrets from entering the sandbox environment.

#### Compatibility Notes

PTY mode is an additional capability.
Agents and orchestrator workflows SHOULD default to exec-round session operations for determinism and easier output bounding, and use PTY only when required by the task profile.

#### Session Sandbox PTY (Interactive Terminal Stream) Requirements Traces

- [REQ-WORKER-0153](../requirements/worker.md#req-worker-0153)

## Sandbox Execution Requirements (Initial Implementation)

- Spec ID: `CYNAI.WORKER.SandboxExecution` <a id="spec-cynai-worker-sandboxexec"></a>

This section describes sandbox execution constraints required by the initial Worker API implementation.

### Sandbox Execution Requirements Traces

- [REQ-WORKER-0103](../requirements/worker.md#req-worker-0103)
- [REQ-WORKER-0104](../requirements/worker.md#req-worker-0104)
- [REQ-WORKER-0105](../requirements/worker.md#req-worker-0105)

## Logging and Output Limits

This section describes log capture and truncation limits for Worker API responses.

### Applicable Requirements (Logging and Output Limits)

- Spec ID: `CYNAI.WORKER.LoggingOutputLimits` <a id="spec-cynai-worker-loglimits"></a>

#### Applicable Requirements (Logging and Output Limits) Requirements Traces

- [REQ-WORKER-0106](../requirements/worker.md#req-worker-0106)
- [REQ-WORKER-0107](../requirements/worker.md#req-worker-0107)
- [REQ-WORKER-0108](../requirements/worker.md#req-worker-0108)

### Request Size Limits (Required)

- Spec ID: `CYNAI.WORKER.WorkerApiRequestSizeLimits` <a id="spec-cynai-worker-workerapirequestsizelimits"></a>

- The node MUST enforce a maximum request body size for `POST /v1/worker/jobs:run`.
- The effective maximum MUST be computed as the minimum of:
  - node startup YAML `worker_api.max_request_bytes` when set
  - node configuration payload `constraints.max_request_bytes` when set
  - otherwise 10485760 (10 MiB)
- Requests larger than the effective maximum MUST be rejected with HTTP 413.

#### Request Size Limits (Required) Requirements Traces

- [REQ-WORKER-0145](../requirements/worker.md#req-worker-0145)

### `stdout`/`stderr` Capture Limits (Required)

- Spec ID: `CYNAI.WORKER.WorkerApiStdIoCaptureLimits` <a id="spec-cynai-worker-workerapistdiocapturelimits"></a>

- The node MUST capture sandbox stdout and stderr as UTF-8 strings.
- The node MUST enforce independent maximum sizes for `stdout` and `stderr` in the response.
- Default limits (when not configured stricter by node-local policy) MUST be:
  - `stdout`: 262144 bytes (256 KiB)
  - `stderr`: 262144 bytes (256 KiB)
- When truncation occurs for a stream, the node MUST:
  - truncate by bytes (not by line count)
  - preserve valid UTF-8 in the returned string
  - set `truncated.stdout=true` and/or `truncated.stderr=true` accordingly

#### `stdout`/`stderr` Capture Limits (Required) Requirements Traces

- [REQ-WORKER-0146](../requirements/worker.md#req-worker-0146)
- [REQ-WORKER-0147](../requirements/worker.md#req-worker-0147)

### Secret Handling (Required)

- Spec ID: `CYNAI.WORKER.WorkerApiSecretHandling` <a id="spec-cynai-worker-workerapisecrethandling"></a>

- Secrets MUST NOT be written to logs.
- If a sandbox writes secrets to stdout/stderr, the node MUST treat that as best-effort user input and MUST NOT attempt
  to "fix" it by pattern-based redaction.
  The correct remediation is to prevent secrets from being placed into the sandbox environment in the first place.

#### Secret Handling (Required) Requirements Traces

- [REQ-WORKER-0148](../requirements/worker.md#req-worker-0148)
- [REQ-WORKER-0108](../requirements/worker.md#req-worker-0108)
