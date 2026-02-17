# WORKER Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `WORKER` domain.
It covers worker-node behavior and the worker API contract for job execution and reporting.

## 2 Requirements

- REQ-WORKER-0001: Worker API: bearer token auth; node validates token; sandbox via container runtime; no orchestrator credentials in containers; bounded logs; no secrets in logs.
  [CYNAI.WORKER.WorkerApiAuth](../tech_specs/worker_api.md#spec-cynai-worker-workerauth)
  [CYNAI.WORKER.SandboxExecution](../tech_specs/worker_api.md#spec-cynai-worker-sandboxexec)
  <a id="req-worker-0001"></a>
- REQ-WORKER-0002: Node exposes worker API for sandbox lifecycle; no inbound SSH; container runtime primitives for sandbox ops.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0002"></a>

- REQ-WORKER-0100: The orchestrator MUST call the Worker API using a bearer token.
  [CYNAI.WORKER.WorkerApiAuth](../tech_specs/worker_api.md#spec-cynai-worker-workerauth)
  <a id="req-worker-0100"></a>
- REQ-WORKER-0101: The node MUST validate the token and reject invalid or expired tokens.
  [CYNAI.WORKER.WorkerApiAuth](../tech_specs/worker_api.md#spec-cynai-worker-workerauth)
  <a id="req-worker-0101"></a>
- REQ-WORKER-0102: Tokens MUST be treated as secrets and MUST NOT be logged.
  [CYNAI.WORKER.WorkerApiAuth](../tech_specs/worker_api.md#spec-cynai-worker-workerauth)
  <a id="req-worker-0102"></a>
- REQ-WORKER-0103: Nodes MUST support sandbox execution using a container runtime (Podman preferred).
  [CYNAI.WORKER.SandboxExecution](../tech_specs/worker_api.md#spec-cynai-worker-sandboxexec)
  <a id="req-worker-0103"></a>
- REQ-WORKER-0104: Nodes MUST NOT expose orchestrator-provided credentials to sandbox containers.
  [CYNAI.WORKER.SandboxExecution](../tech_specs/worker_api.md#spec-cynai-worker-sandboxexec)
  <a id="req-worker-0104"></a>
- REQ-WORKER-0105: Nodes MUST apply basic safety limits for sandbox execution.
  [CYNAI.WORKER.SandboxExecution](../tech_specs/worker_api.md#spec-cynai-worker-sandboxexec)
  <a id="req-worker-0105"></a>
- REQ-WORKER-0106: Worker API implementations MUST bound stdout/stderr size.
  [CYNAI.WORKER.LoggingOutputLimits](../tech_specs/worker_api.md#spec-cynai-worker-loglimits)
  <a id="req-worker-0106"></a>
- REQ-WORKER-0107: When truncation occurs, the response MUST indicate it using `truncated.stdout` and `truncated.stderr`.
  [CYNAI.WORKER.LoggingOutputLimits](../tech_specs/worker_api.md#spec-cynai-worker-loglimits)
  <a id="req-worker-0107"></a>
- REQ-WORKER-0108: Secrets MUST NOT be written to logs.
  [CYNAI.WORKER.LoggingOutputLimits](../tech_specs/worker_api.md#spec-cynai-worker-loglimits)
  <a id="req-worker-0108"></a>

- REQ-WORKER-0109: The node MUST expose a worker API that the orchestrator can call to manage sandbox lifecycle and execution.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0109"></a>
- REQ-WORKER-0110: The node MUST NOT require inbound SSH access to sandboxes for command execution.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0110"></a>
- REQ-WORKER-0111: The node SHOULD use container runtime primitives (create, exec, copy) to implement sandbox operations.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0111"></a>
- REQ-WORKER-0112: The node MUST stream sandbox stdout and stderr back to the orchestrator for logging and debugging.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0112"></a>
- REQ-WORKER-0113: The node MUST associate sandbox containers with `task_id` and `job_id` for auditing and cleanup.
  [CYNAI.WORKER.NodeSandboxControlPlane](../tech_specs/node.md#spec-cynai-worker-nodesandbox)
  <a id="req-worker-0113"></a>
- REQ-WORKER-0114: The node MUST support an execution mode where sandbox jobs can call a node-local inference endpoint without leaving the node.
  [CYNAI.WORKER.NodeLocalInference](../tech_specs/node.md#spec-cynai-worker-nodelocalinference)
  <a id="req-worker-0114"></a>
- REQ-WORKER-0115: The node MUST keep Ollama access private to the node and MUST NOT require exposing Ollama on a public interface.
  [CYNAI.WORKER.NodeLocalInference](../tech_specs/node.md#spec-cynai-worker-nodelocalinference)
  <a id="req-worker-0115"></a>
- REQ-WORKER-0116: Each node SHOULD run a node-local MCP server that exposes sandbox operations for that node.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0116"></a>
- REQ-WORKER-0117: The node MCP server MUST be reachable only by the orchestrator, not by arbitrary clients.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0117"></a>
- REQ-WORKER-0118: The orchestrator SHOULD register each node MCP server with an allowlist.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0118"></a>
- REQ-WORKER-0119: Sandbox operations MUST be audited with `task_id` context.
  [CYNAI.WORKER.NodeSandboxMcpExposure](../tech_specs/node.md#spec-cynai-worker-nodesandboxmcpexposure)
  <a id="req-worker-0119"></a>
- REQ-WORKER-0120: Node startup YAML MUST NOT be treated as the source of truth for global policy.
  [CYNAI.WORKER.NodeStartupYaml](../tech_specs/node.md#spec-cynai-worker-nodestartupyaml)
  <a id="req-worker-0120"></a>
- REQ-WORKER-0121: Node startup YAML MAY impose stricter local constraints than the orchestrator requests.
  [CYNAI.WORKER.NodeStartupYaml](../tech_specs/node.md#spec-cynai-worker-nodestartupyaml)
  <a id="req-worker-0121"></a>
- REQ-WORKER-0122: If a local constraint prevents fulfilling an orchestrator request, the node MUST refuse the request and report the reason.
  [CYNAI.WORKER.NodeStartupYaml](../tech_specs/node.md#spec-cynai-worker-nodestartupyaml)
  <a id="req-worker-0122"></a>
- REQ-WORKER-0123: A node MAY be configured to run no Ollama container.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0123"></a>
- REQ-WORKER-0124: A sandbox-only node MUST still run the worker API and Node Manager.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0124"></a>
- REQ-WORKER-0125: The orchestrator MUST be able to schedule sandbox execution on sandbox-only nodes.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0125"></a>
- REQ-WORKER-0126: Sandbox-only nodes MUST follow the same credential handling and isolation rules as other nodes.
  [CYNAI.WORKER.SandboxOnlyNodes](../tech_specs/node.md#spec-cynai-worker-sandboxonlynodes)
  <a id="req-worker-0126"></a>
- REQ-WORKER-0127: The node MUST NOT expose service credentials to sandbox containers.
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0127"></a>
- REQ-WORKER-0128: The node SHOULD store credentials in a local secure store (root-owned file with strict permissions or OS key store).
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0128"></a>
- REQ-WORKER-0129: The orchestrator SHOULD issue least-privilege pull credentials for node operations that require pulls.
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0129"></a>
- REQ-WORKER-0130: Credentials SHOULD be short-lived where possible and SHOULD support rotation.
  [CYNAI.WORKER.NodeCredentialHandling](../tech_specs/node.md#spec-cynai-worker-nodecredentialhandling)
  <a id="req-worker-0130"></a>

- REQ-WORKER-0131: Secrets MUST be short-lived where possible and MUST NOT be exposed to sandbox containers.
  [CYNAI.WORKER.PayloadSecurity](../tech_specs/node_payloads.md#spec-cynai-worker-payloadsecurity)
  <a id="req-worker-0131"></a>
- REQ-WORKER-0132: Nodes MUST store secrets only in a node-local secure store.
  [CYNAI.WORKER.PayloadSecurity](../tech_specs/node_payloads.md#spec-cynai-worker-payloadsecurity)
  <a id="req-worker-0132"></a>
- REQ-WORKER-0133: Registry and cache pull credentials SHOULD be issued as short-lived tokens.
  [CYNAI.WORKER.Payload.BootstrapV1](../tech_specs/node_payloads.md#spec-cynai-worker-payload-bootstrap-v1)
  <a id="req-worker-0133"></a>
- REQ-WORKER-0134: Tokens SHOULD be rotated by configuration refresh.
  [CYNAI.WORKER.Payload.BootstrapV1](../tech_specs/node_payloads.md#spec-cynai-worker-payload-bootstrap-v1)
  <a id="req-worker-0134"></a>
- REQ-WORKER-0135: Nodes MUST report configuration application status back to the orchestrator.
  [CYNAI.WORKER.Payload.ConfigAckV1](../tech_specs/node_payloads.md#spec-cynai-worker-payload-configack-v1)
  <a id="req-worker-0135"></a>
- REQ-WORKER-0136: New fields MAY be added to payloads as optional fields.
  [CYNAI.WORKER.Payload.CompatibilityVersioning](../tech_specs/node_payloads.md#spec-cynai-worker-payload-versioning)
  <a id="req-worker-0136"></a>
- REQ-WORKER-0137: Fields MUST NOT change meaning within the same `version`.
  [CYNAI.WORKER.Payload.CompatibilityVersioning](../tech_specs/node_payloads.md#spec-cynai-worker-payload-versioning)
  <a id="req-worker-0137"></a>
- REQ-WORKER-0138: Nodes SHOULD reject payloads with unsupported `version` values and report a structured error.
  [CYNAI.WORKER.Payload.CompatibilityVersioning](../tech_specs/node_payloads.md#spec-cynai-worker-payload-versioning)
  <a id="req-worker-0138"></a>
