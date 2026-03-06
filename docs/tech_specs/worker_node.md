# Worker Node Technical Spec

- [Document Overview](#document-overview)
- [Node Manager](#node-manager)
- [Managed Service Containers](#managed-service-containers)
- [Worker Proxy Bidirectional (Managed Agents)](#worker-proxy-bidirectional-managed-agents)
- [Token and Credential Handling](#token-and-credential-handling)
  - [Token Authentication and Auditing](#token-authentication-and-auditing)
- [Sandbox Control Plane](#sandbox-control-plane)
  - [Sandbox Workspace and Job Mounts](#sandbox-workspace-and-job-mounts)
  - [Sandbox Rootless Execution](#sandbox-rootless-execution)
  - [Sandbox Control Plane Applicable Requirements](#sandbox-control-plane-applicable-requirements)
- [Node-Local Inference and Sandbox Workflow](#node-local-inference-and-sandbox-workflow)
  - [Node-Local Inference Applicable Requirements](#node-local-inference-applicable-requirements)
- [Node Sandbox MCP Exposure](#node-sandbox-mcp-exposure)
  - [Node Sandbox MCP Exposure Applicable Requirements](#node-sandbox-mcp-exposure-applicable-requirements)
  - [Node-Local Agent Sandbox Control (Low-Latency Path)](#node-local-agent-sandbox-control-low-latency-path)
- [Node Startup YAML](#node-startup-yaml)
  - [Node Startup YAML Applicable Requirements](#node-startup-yaml-applicable-requirements)
  - [User-Configurable Properties](#user-configurable-properties)
- [Node Startup Procedure](#node-startup-procedure)
- [Node Startup Checks and Readiness](#node-startup-checks-and-readiness)
- [Deployment and Auto-Start](#deployment-and-auto-start)
- [Existing Inference Service on Host](#existing-inference-service-on-host)
- [Ollama Container Policy](#ollama-container-policy)
- [Sandbox-Only Nodes](#sandbox-only-nodes)
  - [Sandbox-Only Nodes Applicable Requirements](#sandbox-only-nodes-applicable-requirements)
- [Registration and Bootstrap](#registration-and-bootstrap)
- [Capability Reporting](#capability-reporting)
- [Configuration Delivery](#configuration-delivery)
- [Dynamic Configuration Updates](#dynamic-configuration-updates)
- [Credential Handling](#credential-handling)
  - [Credential Handling Applicable Requirements](#credential-handling-applicable-requirements)
- [Required Node Configuration](#required-node-configuration)

## Document Overview

This document defines worker node responsibilities, including node registration, configuration bootstrap, and secure credential handling.
Nodes are configured by the orchestrator to access orchestrator-provided services such as the rank-ordered sandbox image registry list and model cache.

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

PMA as managed service (normative):

- PMA is a core system feature and is always required.
- The orchestrator MUST instruct a worker to run PMA as a managed service container.
- The worker MUST start and keep PMA running when configured as a managed service.

## Worker Proxy Bidirectional (Managed Agents)

- Spec ID: `CYNAI.WORKER.WorkerProxyBidirectionalManagedAgents` <a id="spec-cynai-worker-proxybidirectional"></a>

Managed agent runtimes (for example PMA) MUST communicate with the orchestrator through the worker proxy in both directions.

Normative behavior:

- **Orchestrator to agent:** The worker MUST expose a worker-mediated endpoint (via Worker API reverse proxy) that the orchestrator
  (and user-gateway, when applicable) can call to reach the managed agent container (e.g. PMA chat handoff and health).
- **Agent to orchestrator:** The worker MUST expose worker-local proxy endpoints that the managed agent uses to call:
  - the orchestrator MCP gateway (for tool calls), and
  - any orchestrator callback/ready endpoints.
  The worker proxy forwards those requests to the orchestrator.
- The managed agent container MUST NOT be configured to call orchestrator hostnames or ports directly.
  All agent-to-orchestrator traffic flows through the worker proxy.

## Token and Credential Handling

- Spec ID: `CYNAI.WORKER.AgentTokensWorkerHeldOnly` <a id="spec-cynai-worker-agenttokensworkerheldonly"></a>

- **Agents MUST NOT be given tokens or secrets directly.**
  The worker proxy MUST hold orchestrator-issued credentials (agent tokens, capability leases) and MUST attach the appropriate credential when forwarding agent-originated requests to the orchestrator.
  The worker MUST NOT pass agent tokens or other orchestrator-issued secrets into agent containers or to agents; the agent calls the worker proxy (e.g. worker-proxy URL for MCP), and the worker proxy adds the token when forwarding to the gateway.

Traces To: [REQ-WORKER-0164](../requirements/worker.md#req-worker-0164).

### Token Authentication and Auditing

- The worker MUST authenticate and authorize proxy requests according to orchestrator-issued credentials (agent tokens, capability leases) that the **worker** holds and MUST fail closed when validation fails.
- The worker MUST emit auditable records for proxy activity sufficient to attribute actions to the agent identity and context.

### Agent Token Storage and Lifecycle

- Spec ID: `CYNAI.WORKER.AgentTokenStorageAndLifecycle` <a id="spec-cynai-worker-agenttokenstorageandlifecycle"></a>

Traces To:

- [REQ-WORKER-0164](../requirements/worker.md#req-worker-0164)
- [REQ-WORKER-0165](../requirements/worker.md#req-worker-0165)
- [REQ-WORKER-0166](../requirements/worker.md#req-worker-0166)
- [REQ-WORKER-0167](../requirements/worker.md#req-worker-0167)
- [REQ-WORKER-0168](../requirements/worker.md#req-worker-0168)

The worker MUST store agent tokens in the node-local secure store defined by [CYNAI.WORKER.NodeLocalSecureStore](#spec-cynai-worker-nodelocalsecurestore) and MUST NOT pass agent tokens into any agent container (including managed-service containers such as PMA).

Required behavior:

- The worker MUST key agent tokens by the managed-service identity (e.g. `service_id`) so the worker proxy can deterministically select the correct token for the calling agent runtime.
- The worker proxy MUST attach the correct agent token to agent-originated requests when forwarding to the orchestrator.
- The worker MUST NOT expose agent tokens to sandboxes or agents via env vars, files, mounts, or logs.

Observability:

- Agent tokens MUST NOT appear in logs, metrics, audit payloads (beyond opaque identifiers such as `service_id` or agent identity), debug endpoints, or telemetry responses.
  Redaction MUST NOT be relied upon.

#### `AgentTokenStorageAndLifecycle` Algorithm

<a id="algo-cynai-worker-agenttokenstorageandlifecycle"></a>

1. On configuration apply, for each `managed_services.services[]` entry that includes `orchestrator.agent_token` or `agent_token_ref`, the worker resolves the token value and writes it to the node-local secure store under the key for that service identity. <a id="algo-cynai-worker-agenttokenstorageandlifecycle-step-1"></a>
2. The worker MUST NOT pass the token value to the managed-service container or agent runtime. <a id="algo-cynai-worker-agenttokenstorageandlifecycle-step-2"></a>
3. When the worker proxy receives an agent-originated request, it determines the calling service identity, loads the corresponding token from the secure store, attaches it to the outbound request, and forwards to the orchestrator. <a id="algo-cynai-worker-agenttokenstorageandlifecycle-step-3"></a>
4. On configuration update or service removal, the worker removes or overwrites the stored token for that service identity so the old token is no longer available. <a id="algo-cynai-worker-agenttokenstorageandlifecycle-step-4"></a>
5. When an expiry is provided (e.g. `agent_token_expires_at`), the worker MUST treat expired tokens as invalid and MUST NOT use them to forward requests; the worker SHOULD request a configuration refresh where applicable. <a id="algo-cynai-worker-agenttokenstorageandlifecycle-step-5"></a>

## Sandbox Control Plane

This section defines how agents and the orchestrator interact with sandbox containers on a node.
Agents do not connect to sandboxes directly over the network.
Outbound traffic from sandboxes is permitted only through worker proxies (inference proxy, node-local web egress proxy, and orchestrator API Egress); sandboxes do not have direct internet access.
See [Sandbox Boundary and Security](cynode_sba.md#spec-cynai-sbagnt-sandboxboundary) and [Network Expectations](sandbox_container.md#spec-cynai-sandbx-networkexpect).

### Sandbox Workspace and Job Mounts

- Spec ID: `CYNAI.WORKER.SandboxWorkspaceJobMounts` <a id="spec-cynai-worker-sandboxworkspacejobmounts"></a>

Traces To:

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

Traces To:

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

Traces To:

- [REQ-WORKER-0109](../requirements/worker.md#req-worker-0109)
- [REQ-WORKER-0110](../requirements/worker.md#req-worker-0110)
- [REQ-WORKER-0111](../requirements/worker.md#req-worker-0111)
- [REQ-WORKER-0112](../requirements/worker.md#req-worker-0112)
- [REQ-WORKER-0113](../requirements/worker.md#req-worker-0113)

Worker API contract

- The Worker API endpoint surface and payload shapes are defined in [`docs/tech_specs/worker_api.md`](worker_api.md).

Worker API operations (MVP Phase 1)

For MVP Phase 1, the Worker API surface is intentionally minimal and MUST implement only:

- `POST /v1/worker/jobs:run`

Future phases MAY add endpoints for file transfer, async job polling, and log streaming, but those MUST be defined in
[`docs/tech_specs/worker_api.md`](worker_api.md) before implementation.

See [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md) for the MCP tool layer that orchestrator-side agents use.

## Node-Local Inference and Sandbox Workflow

This section defines the preferred node-local workflow when a sandbox and Ollama inference are co-located on the same node.
Node-local traffic MUST remain on the node and MUST NOT traverse external networks.

### Node-Local Inference Applicable Requirements

- Spec ID: `CYNAI.WORKER.NodeLocalInference` <a id="spec-cynai-worker-nodelocalinference"></a>

Traces To:

- [REQ-WORKER-0114](../requirements/worker.md#req-worker-0114)
- [REQ-WORKER-0115](../requirements/worker.md#req-worker-0115)

Option A (required for node-local execution when inference is enabled)

- For each sandbox job, the Node Manager creates an isolated network for that job: when the runtime is Podman, a pod; when the runtime is Docker, a user-defined bridge network for that job with the sandbox and inference proxy containers attached.
  Podman is preferred for rootless operation.
- The pod (or network) contains:
  - the sandbox container
  - a lightweight inference proxy sidecar container
- The inference proxy listens on `localhost:11434` inside the pod network namespace.
- The inference proxy forwards requests to the node's single Ollama container over a node-internal container network.
- The sandbox container calls the model via `http://localhost:11434`.

See [`docs/tech_specs/ports_and_endpoints.md`](ports_and_endpoints.md#spec-cynai-stands-portsandendpoints) for consolidated default ports and conflict avoidance.

Rationale

- The sandbox can use a stable localhost endpoint.
- The pod network namespace is isolated per job, avoiding cross-job localhost sharing.
- Ollama remains a single long-lived container on the node.

Implementation notes

- The Node Manager MUST inject `OLLAMA_BASE_URL=http://localhost:11434` into the sandbox container environment.
- The inference proxy sidecar MUST be minimal and MUST NOT expose credentials.
- The inference proxy sidecar MUST enforce request size limits and timeouts.
  - Maximum request body size MUST be 10485760 bytes (10 MiB).
  - Per-request timeout MUST be 120 seconds.

## Node Sandbox MCP Exposure

For MVP Phase 2 and later, when the orchestrator needs to manage or interact with a sandbox on a node, sandbox
operations MUST be exposed as MCP tools on that node.
The orchestrator acts as the default routing point for sandbox tools for remote agent runtimes.
When an AI agent runtime is co-located on the same host as the worker node, the node MUST support a low-latency control path that allows direct interaction with node-hosted sandbox tools under orchestrator-issued capability leases.

### Node Sandbox MCP Exposure Applicable Requirements

- Spec ID: `CYNAI.WORKER.NodeSandboxMcpExposure` <a id="spec-cynai-worker-nodesandboxmcpexposure"></a>

Traces To:

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

Traces To:

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

Traces To:

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

Traces To:

- [REQ-WORKER-0253](../requirements/worker.md#req-worker-0253)
- [REQ-WORKER-0254](../requirements/worker.md#req-worker-0254)

This procedure is constrained by [Existing Inference Service on Host](#existing-inference-service-on-host) (use existing host inference when present; do not start a duplicate) and [Ollama Container Policy](#ollama-container-policy) (at most one inference service in use; when the node starts the container, it grants that container access to all GPUs and NPUs).

On startup, the Node Manager MUST contact the orchestrator and receive configuration **before** starting the Ollama (or equivalent) container.
The Worker API MUST be started and the node MUST register with the orchestrator (sending its capabilities bundle) before the node starts any local inference container.
The orchestrator acknowledges registration and returns a node configuration payload that **instructs** the node whether and how to start the local inference backend (e.g. container image and backend variant such as ROCm for AMD or CUDA for Nvidia).
The node MUST NOT start the Ollama container until it has received this instruction in the node configuration payload (see [`worker_node_payloads.md`](worker_node_payloads.md) `node_configuration_payload_v1` `inference_backend`).

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
   Only when no existing service is detected and the node configuration instructs the node to start local inference (see `inference_backend` in [`worker_node_payloads.md`](worker_node_payloads.md)), the node starts the single Ollama (or equivalent) container per [Ollama Container Policy](#ollama-container-policy) (image and variant specified by the orchestrator, e.g. ROCm for AMD or CUDA for Nvidia; container granted access to all GPUs and NPUs).
9. Report startup status and effective configuration version to the orchestrator (config ack and ongoing capability reporting).
    The node MUST report to the orchestrator when it has become ready (e.g. via config ack with status applied after services are started) so the orchestrator can consider the node as an inference path and start the Project Manager Agent when appropriate; see [REQ-WORKER-0254](../requirements/worker.md#req-worker-0254).

## Node Startup Checks and Readiness

- Spec ID: `CYNAI.WORKER.NodeStartupChecks` <a id="spec-cynai-worker-nodestartupchecks"></a>

Traces To:

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

Traces To:

- [REQ-BOOTST-0104](../requirements/bootst.md#req-bootst-0104)

Worker node deployments MUST support auto-start on the host so that Node Manager and Worker API (and related services) start on boot or on demand without manual invocation.

- **Linux:** The implementation MUST provide systemd unit files for worker node services (Node Manager, Worker API).
  Both user (rootless) and system (root) installs MUST be supported.
  See [`worker_node/systemd/README.md`](../../worker_node/systemd/README.md) for the reference layout and generation steps.
- **macOS:** The implementation MUST provide launchd plist files for worker node services so that they can start on boot or on user login, with the same capability as the Linux systemd approach (start on boot, start on demand, enable/disable).

## Existing Inference Service on Host

- Spec ID: `CYNAI.WORKER.ExistingInferenceService` <a id="spec-cynai-worker-existinginferenceservice"></a>

Traces To:

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

Traces To:

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

## Capability Reporting

- Spec ID: `CYNAI.WORKER.CapabilityReporting` <a id="spec-cynai-worker-capabilityreporting"></a>

Traces To:

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

Traces To:

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
- The node MUST NOT start the OLLAMA (or equivalent) container until it has received this instruction; see [Node Startup Procedure](#node-startup-procedure).

Required behavior

- The node MUST support receiving configuration at registration time.
- For MVP Phase 1, the node MUST fetch configuration on startup and MUST NOT poll for configuration updates (no polling in Phase 1).
- For MVP Phase 3 and later, the node MUST support configuration refresh by polling.
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
- The node MUST apply configuration updates atomically where possible and MUST roll back on failure.
- The node MUST acknowledge applied configuration version to the orchestrator.
- The node MUST request a configuration refresh on startup and when capability reports change.

## Credential Handling

Nodes require credentials to connect to orchestrator-provided services.
These credentials MUST be handled securely and with least privilege.

### Credential Handling Applicable Requirements

- Spec ID: `CYNAI.WORKER.NodeCredentialHandling` <a id="spec-cynai-worker-nodecredentialhandling"></a>

Traces To:

- [REQ-WORKER-0127](../requirements/worker.md#req-worker-0127)
- [REQ-WORKER-0128](../requirements/worker.md#req-worker-0128)
- [REQ-WORKER-0129](../requirements/worker.md#req-worker-0129)
- [REQ-WORKER-0130](../requirements/worker.md#req-worker-0130)

### Node-Local Secure Store

- Spec ID: `CYNAI.WORKER.NodeLocalSecureStore` <a id="spec-cynai-worker-nodelocalsecurestore"></a>

Traces To:

- [REQ-WORKER-0128](../requirements/worker.md#req-worker-0128)
- [REQ-WORKER-0132](../requirements/worker.md#req-worker-0132)
- [REQ-WORKER-0165](../requirements/worker.md#req-worker-0165)
- [REQ-WORKER-0166](../requirements/worker.md#req-worker-0166)
- [REQ-WORKER-0167](../requirements/worker.md#req-worker-0167)
- [REQ-WORKER-0168](../requirements/worker.md#req-worker-0168)
- [REQ-WORKER-0169](../requirements/worker.md#req-worker-0169)
- [REQ-WORKER-0170](../requirements/worker.md#req-worker-0170)

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
- The worker MUST use a post-quantum resistant symmetric algorithm for encryption at rest (e.g. AES-256-GCM) with a per-record nonce.
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

- The worker SHOULD use `runtime/secret` (Go 1.26, via `GOEXPERIMENT=secret`) to wrap code that handles the master key or decrypted plaintext secrets so temporaries are erased before returning.

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
