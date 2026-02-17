# Worker API (Node) - Contract and Payloads

- [Document Overview](#document-overview)
- [Scope](#scope)
- [Definitions](#definitions)
- [Versioning](#versioning)
- [Authentication](#authentication)
- [Error Handling](#error-handling)
- [Worker API Surface (Initial Implementation)](#worker-api-surface-initial-implementation)
  - [Run Job (Synchronous)](#run-job-synchronous)
- [Sandbox Execution Requirements (Initial Implementation)](#sandbox-execution-requirements-initial-implementation)
- [Logging and Output Limits](#logging-and-output-limits)

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

#### Endpoint Details

- `POST /v1/worker/jobs:run`

#### Required Behavior

- The node MUST execute the provided command in a sandbox container.
- The node MUST associate the execution with `task_id` and `job_id` for auditing and cleanup.
- The node MUST enforce a timeout (job-provided or node default).
- The node MUST capture stdout and stderr and include them in the response (subject to truncation limits).
- The node MUST return a deterministic result for the same inputs when execution is deterministic.

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
