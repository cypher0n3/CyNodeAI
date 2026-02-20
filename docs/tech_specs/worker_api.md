# Worker API (Node) - Contract and Payloads

- [Document Overview](#document-overview)
- [Scope](#scope)
- [Definitions](#definitions)
- [Versioning](#versioning)
- [Authentication](#authentication)
  - [Applicable Requirements (Authentication)](#applicable-requirements-authentication)
- [Health Checks](#health-checks)
- [Error Handling](#error-handling)
- [Worker API Surface (Initial Implementation)](#worker-api-surface-initial-implementation)
  - [Run Job (Synchronous)](#run-job-synchronous)
  - [Session Sandbox (Long-Running)](#session-sandbox-long-running)
- [Sandbox Execution Requirements (Initial Implementation)](#sandbox-execution-requirements-initial-implementation)
  - [Applicable Requirements (Sandbox Execution)](#applicable-requirements-sandbox-execution)
- [Logging and Output Limits](#logging-and-output-limits)
  - [Applicable Requirements (Logging and Output Limits)](#applicable-requirements-logging-and-output-limits)
  - [Request Size Limits (Required)](#request-size-limits-required)
  - [Stdout/stderr Capture Limits (Required)](#stdoutstderr-capture-limits-required)
  - [Secret Handling (Required)](#secret-handling-required)

## Document Overview

This document defines the **Worker API** contract exposed by a node (local worker or cloud worker).
The orchestrator uses this API to dispatch work to a node for sandbox execution and to collect results.

This document is the canonical contract for:

- endpoint paths and methods
- request and response payload shapes
- authentication requirements for orchestrator to call the node

Related specs

- Node responsibilities: [`docs/tech_specs/node.md`](node.md)
- Go API implementation standards: [`docs/tech_specs/go_rest_api_standards.md`](go_rest_api_standards.md)
- Node payloads (bootstrap/config): [`docs/tech_specs/node_payloads.md`](node_payloads.md)

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

Traces To:

- [REQ-WORKER-0100](../requirements/worker.md#req-worker-0100)
- [REQ-WORKER-0101](../requirements/worker.md#req-worker-0101)
- [REQ-WORKER-0102](../requirements/worker.md#req-worker-0102)

Token delivery

- The orchestrator MUST deliver the Worker API bearer token to the node via the node configuration payload.
  - See [`docs/tech_specs/node_payloads.md`](node_payloads.md) `node_configuration_payload_v1`.

Initial implementation (Phase 1) constraints

- Bearer token is static (delivered via node config; no refresh).
- Component-to-component traffic MUST support HTTP (not HTTPS) for MVP.
  HTTPS MAY be added later without changing endpoint paths or payload shapes.
- When `network_policy` is `restricted`, treat as deny-all (equivalent to `none`).
- CPU, memory, and PIDs limits are not applied; workspace is per-job at `/workspace` per [`docs/tech_specs/sandbox_container.md`](sandbox_container.md).

## Health Checks

The Worker API MUST expose unauthenticated health checks intended for liveness and readiness probing.
Health check endpoints MUST NOT require a `/v1/` prefix and MUST NOT require a JSON `version` field.

### Applicable Requirements (Health Checks)

- Spec ID: `CYNAI.WORKER.WorkerApiHealthChecks` <a id="spec-cynai-worker-workerapihealthchecks"></a>

Traces To:

- [REQ-WORKER-0140](../requirements/worker.md#req-worker-0140)
- [REQ-WORKER-0141](../requirements/worker.md#req-worker-0141)
- [REQ-WORKER-0142](../requirements/worker.md#req-worker-0142)

Endpoints

- `GET /healthz`
  - returns 200 with plain text body `ok` when the HTTP server is running
- `GET /readyz`
  - returns 200 with plain text body `ready` when the node is ready to accept job execution requests
  - returns 503 when the node is not ready

## Error Handling

Worker API error responses MUST follow the Go REST API error standards:

- Prefer RFC 9457 Problem Details JSON (`application/problem+json`).
- Do not leak secrets in errors.
- Use stable error `type` values where practical.

See [`docs/tech_specs/go_rest_api_standards.md`](go_rest_api_standards.md#error-format-and-status-codes).

## Worker API Surface (Initial Implementation)

This section defines the initial endpoint(s) required for orchestrator-to-node job dispatch.

### Run Job (Synchronous)

Run a sandbox job to completion and return a result in the same response.

#### Applicable Requirements (Run Job)

- Spec ID: `CYNAI.WORKER.WorkerApiRunJobSyncV1` <a id="spec-cynai-worker-workerapirunjobsync-v1"></a>

Traces To:

- [REQ-WORKER-0143](../requirements/worker.md#req-worker-0143)
- [REQ-WORKER-0144](../requirements/worker.md#req-worker-0144)

#### Endpoint Details

- `POST /v1/worker/jobs:run`

#### Required Behavior

- The node MUST execute the provided command in a sandbox container.
- The node MUST associate the execution with `task_id` and `job_id` for auditing and cleanup.
- The node MUST enforce a timeout (job-provided or node default) using the rules below.
- The node MUST capture stdout and stderr and include them in the response (subject to truncation limits).
- The node MUST return a deterministic result for the same inputs when execution is deterministic.

Timeout rules (required)

- If `sandbox.timeout_seconds` is provided, the effective timeout MUST be:
  - `min(sandbox.timeout_seconds, node_max_job_timeout_seconds)`
- If `sandbox.timeout_seconds` is not provided, the effective timeout MUST be:
  - `min(node_default_job_timeout_seconds, node_max_job_timeout_seconds)`
- `node_default_job_timeout_seconds` MUST be derived in this order:
  - node startup YAML `sandbox.timeouts.default_seconds`
  - otherwise 900 seconds
- `node_max_job_timeout_seconds` MUST be derived in this order:
  - node startup YAML `sandbox.timeouts.max_seconds`
  - otherwise 3600 seconds
- If the node configuration payload includes `constraints.max_job_timeout_seconds`, the node MUST further cap the timeout
  by taking the minimum of the values above and `constraints.max_job_timeout_seconds`.

#### Request Body Example

```json
{
  "version": 1,
  "task_id": "00000000-0000-0000-0000-000000000000",
  "job_id": "00000000-0000-0000-0000-000000000000",
  "sandbox": {
    "image": "registry.example.com/cynode/sandboxes/base:1",
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
  - `image` (string, required): OCI image reference
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

#### Status Codes

- 200: job executed (even if failed inside the sandbox; failure is indicated in body `status`)
- 400: invalid request
- 401/403: auth failure
- 413: request too large
- 500: internal node error

### Session Sandbox (Long-Running)

- Spec ID: `CYNAI.WORKER.SessionSandbox` <a id="spec-cynai-worker-sessionsandbox"></a>

Traces To:

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

See [`docs/tech_specs/sandbox_container.md`](sandbox_container.md#long-running-session-sandboxes) for the sandbox contract for long-running sessions.

### Session Sandbox PTY (Interactive Terminal Stream)

- Spec ID: `CYNAI.WORKER.SessionSandboxPty` <a id="spec-cynai-worker-sessionsandboxpty"></a>

Traces To:

- [REQ-WORKER-0153](../requirements/worker.md#req-worker-0153)

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

Recommended API shape (subject to later endpoint finalization):

- `POST /v1/worker/sessions/{session_id}/pty:open`
- `POST /v1/worker/sessions/{session_id}/pty:send`
- `POST /v1/worker/sessions/{session_id}/pty:resize`
- `POST /v1/worker/sessions/{session_id}/pty:close`
- `GET /v1/worker/sessions/{session_id}/pty:recv`

Recommended request fields (minimum):

- `version` (int, required): must be 1
- `task_id` (uuid string, required)
- `session_id` (uuid string, required)
- `pty_id` (uuid string, required for send/recv/resize/close; returned by open)

PTY I/O encoding:

- PTY payloads MUST be treated as bytes, not as UTF-8 text.
- Requests and responses SHOULD encode byte payloads using base64 fields (for example, `data_b64`) so the transport remains JSON-safe.
- The node MUST enforce strict per-message and per-buffer size limits.

Timeouts and lifecycle:

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

## Sandbox Execution Requirements (Initial Implementation)

This section describes sandbox execution constraints required by the initial Worker API implementation.

### Applicable Requirements (Sandbox Execution)

- Spec ID: `CYNAI.WORKER.SandboxExecution` <a id="spec-cynai-worker-sandboxexec"></a>

Traces To:

- [REQ-WORKER-0103](../requirements/worker.md#req-worker-0103)
- [REQ-WORKER-0104](../requirements/worker.md#req-worker-0104)
- [REQ-WORKER-0105](../requirements/worker.md#req-worker-0105)

## Logging and Output Limits

This section describes log capture and truncation limits for Worker API responses.

### Applicable Requirements (Logging and Output Limits)

- Spec ID: `CYNAI.WORKER.LoggingOutputLimits` <a id="spec-cynai-worker-loglimits"></a>

Traces To:

- [REQ-WORKER-0106](../requirements/worker.md#req-worker-0106)
- [REQ-WORKER-0107](../requirements/worker.md#req-worker-0107)
- [REQ-WORKER-0108](../requirements/worker.md#req-worker-0108)

### Request Size Limits (Required)

- Spec ID: `CYNAI.WORKER.WorkerApiRequestSizeLimits` <a id="spec-cynai-worker-workerapirequestsizelimits"></a>

Traces To:

- [REQ-WORKER-0145](../requirements/worker.md#req-worker-0145)

- The node MUST enforce a maximum request body size for `POST /v1/worker/jobs:run`.
- The effective maximum MUST be computed as the minimum of:
  - node startup YAML `worker_api.max_request_bytes` when set
  - node configuration payload `constraints.max_request_bytes` when set
  - otherwise 10485760 (10 MiB)
- Requests larger than the effective maximum MUST be rejected with HTTP 413.

### Stdout/stderr Capture Limits (Required)

- Spec ID: `CYNAI.WORKER.WorkerApiStdIoCaptureLimits` <a id="spec-cynai-worker-workerapistdiocapturelimits"></a>

Traces To:

- [REQ-WORKER-0146](../requirements/worker.md#req-worker-0146)
- [REQ-WORKER-0147](../requirements/worker.md#req-worker-0147)

- The node MUST capture sandbox stdout and stderr as UTF-8 strings.
- The node MUST enforce independent maximum sizes for `stdout` and `stderr` in the response.
- Default limits (when not configured stricter by node-local policy) MUST be:
  - `stdout`: 262144 bytes (256 KiB)
  - `stderr`: 262144 bytes (256 KiB)
- When truncation occurs for a stream, the node MUST:
  - truncate by bytes (not by line count)
  - preserve valid UTF-8 in the returned string
  - set `truncated.stdout=true` and/or `truncated.stderr=true` accordingly

### Secret Handling (Required)

- Spec ID: `CYNAI.WORKER.WorkerApiSecretHandling` <a id="spec-cynai-worker-workerapisecrethandling"></a>

Traces To:

- [REQ-WORKER-0148](../requirements/worker.md#req-worker-0148)
- [REQ-WORKER-0108](../requirements/worker.md#req-worker-0108)

- Secrets MUST NOT be written to logs.
- If a sandbox writes secrets to stdout/stderr, the node MUST treat that as best-effort user input and MUST NOT attempt
  to "fix" it by pattern-based redaction.
  The correct remediation is to prevent secrets from being placed into the sandbox environment in the first place.
