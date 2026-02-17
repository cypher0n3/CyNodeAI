# Worker Node Technical Spec

- [Document Overview](#document-overview)
- [Node Manager](#node-manager)
- [Sandbox Control Plane](#sandbox-control-plane)
  - [Sandbox Control Plane Applicable Requirements](#sandbox-control-plane-applicable-requirements)
- [Node-Local Inference and Sandbox Workflow](#node-local-inference-and-sandbox-workflow)
  - [Node-Local Inference Applicable Requirements](#node-local-inference-applicable-requirements)
- [Node Sandbox MCP Exposure](#node-sandbox-mcp-exposure)
  - [Node Sandbox MCP Exposure Applicable Requirements](#node-sandbox-mcp-exposure-applicable-requirements)
- [Node Startup YAML](#node-startup-yaml)
  - [Node Startup YAML Applicable Requirements](#node-startup-yaml-applicable-requirements)
  - [User-Configurable Properties](#user-configurable-properties)
- [Node Startup Procedure](#node-startup-procedure)
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
Nodes are configured by the orchestrator to access orchestrator-provided services such as the sandbox image registry and model cache.

## Node Manager

The Node Manager is a host-level system service responsible for:

- Starting and stopping worker services (worker API, Ollama, sandbox containers).
- Managing container runtime (Docker or Podman) lifecycle for sandbox execution.
  Podman is preferred for rootless operation.
- Receiving configuration updates from the orchestrator and applying them locally.
- Managing local secure storage for pull credentials and certificates.

## Sandbox Control Plane

This section defines how agents and the orchestrator interact with sandbox containers on a node.
Agents do not connect to sandboxes directly over the network.

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

Recommended sandbox operations in the worker API

- Create sandbox container for a task job.
- Execute a command inside a sandbox container.
- Upload and download workspace files, when needed.
- Stream logs for a sandbox execution.
- Stop and remove a sandbox container.

See [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md) for the MCP tool layer that orchestrator-side agents use.

## Node-Local Inference and Sandbox Workflow

This section defines the preferred node-local workflow when a sandbox and Ollama inference are co-located on the same node.
Node-local traffic SHOULD remain on the node and SHOULD not traverse external networks.

### Node-Local Inference Applicable Requirements

- Spec ID: `CYNAI.WORKER.NodeLocalInference` <a id="spec-cynai-worker-nodelocalinference"></a>

Traces To:

- [REQ-WORKER-0114](../requirements/worker.md#req-worker-0114)
- [REQ-WORKER-0115](../requirements/worker.md#req-worker-0115)

Option A (recommended for node-local execution)

- For each sandbox job, the Node Manager creates a runtime pod (Podman) or equivalent isolated network (Docker) for that job.
  Podman is preferred for rootless operation.
- The pod (or network) contains:
  - the sandbox container
  - a lightweight inference proxy sidecar container
- The inference proxy listens on `localhost:11434` inside the pod network namespace.
- The inference proxy forwards requests to the node's single Ollama container over a node-internal container network.
- The sandbox container calls the model via `http://localhost:11434`.

Rationale

- The sandbox can use a stable localhost endpoint.
- The pod network namespace is isolated per job, avoiding cross-job localhost sharing.
- Ollama remains a single long-lived container on the node.

Implementation notes

- The Node Manager should inject `OLLAMA_BASE_URL=http://localhost:11434` into the sandbox container environment.
- The inference proxy sidecar should be minimal and should not expose credentials.
- The inference proxy sidecar should enforce request size limits and timeouts.

## Node Sandbox MCP Exposure

When the orchestrator needs to manage or interact with a sandbox on a node, sandbox operations should be exposed as MCP tools on that node.
The orchestrator acts as the routing point and agents do not connect to node MCP servers directly.

### Node Sandbox MCP Exposure Applicable Requirements

- Spec ID: `CYNAI.WORKER.NodeSandboxMcpExposure` <a id="spec-cynai-worker-nodesandboxmcpexposure"></a>

Traces To:

- [REQ-WORKER-0116](../requirements/worker.md#req-worker-0116)
- [REQ-WORKER-0117](../requirements/worker.md#req-worker-0117)
- [REQ-WORKER-0118](../requirements/worker.md#req-worker-0118)
- [REQ-WORKER-0119](../requirements/worker.md#req-worker-0119)

Recommended sandbox MCP tool surface

- `sandbox.create`
- `sandbox.exec`
- `sandbox.put_file`
- `sandbox.get_file`
- `sandbox.stream_logs`
- `sandbox.destroy`

## Node Startup YAML

Nodes SHOULD support a local startup YAML file that the Node Manager reads on boot.
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

### User-Configurable Properties

Node startup YAML SHOULD allow operators to set the properties below.
These settings are node-local and MAY be stricter than orchestrator policy.

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

- Secrets SHOULD be supplied via env vars or local files.
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
  - Address/interface to bind (default implementation-defined).
- `worker_api.listen_port` (number, optional)
  - Port to bind.
- `worker_api.public_base_url` (string, optional)
  - Public URL the orchestrator should use to reach the worker API.
- `worker_api.max_request_bytes` (number, optional)
  - Maximum request size accepted by the Worker API.

#### Sandbox Settings

Node startup YAML MUST support a sandbox mode that determines whether the node is eligible for sandbox execution.

Recommended values

- `allow`
  - The node may run sandboxes.
  - The node may also provide inference if available and enabled.
- `sandbox_only`
  - The node may run sandboxes.
  - The node MUST NOT run inference services.
- `disabled`
  - The node MUST NOT run sandboxes.
  - The orchestrator MUST treat the node as ineligible for sandbox scheduling.

Sandbox keys

- `sandbox.mode` (string, optional)
  - One of `allow`, `sandbox_only`, or `disabled`.
- `sandbox.runtime` (string, optional)
  - Sandbox runtime identifier: `podman` or `docker`.
    Podman is preferred for rootless operation.
- `sandbox.rootless` (boolean, optional)
  - Whether sandbox containers run rootless when supported (Podman supports rootless; Docker typically requires root or root-equivalent).
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
- `sandbox.resources.max_cpu_cores` (number, optional)
  - Maximum CPU cores allowed for a sandbox job.
- `sandbox.resources.max_memory_mb` (number, optional)
  - Maximum memory allowed for a sandbox job.
- `sandbox.resources.max_pids` (number, optional)
  - Maximum process count allowed for a sandbox job.
- `sandbox.timeouts.default_seconds` (number, optional)
  - Default sandbox timeout when not specified by the orchestrator.
- `sandbox.timeouts.max_seconds` (number, optional)
  - Maximum sandbox timeout allowed on this node.
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

Node startup YAML SHOULD allow operators to disable inference even if hardware is present.

Inference keys

- `inference.mode` (string, optional)
  - One of `allow` or `disabled`.
- `inference.max_concurrency` (number, optional)
  - Maximum concurrent inference requests accepted by local inference services.
- `inference.allow_gpu` (boolean, optional)
  - Whether GPU/NPU devices may be used for inference on this node.

#### Storage Settings

- `storage.state_dir` (string, optional)
  - Node state directory (registration cache and applied config state).
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
  listen_port: 8080
  public_base_url: https://worker-01.example.com
  max_request_bytes: 10485760
sandbox:
  mode: sandbox_only
  runtime: podman   # or docker; podman preferred for rootless
  rootless: true
  max_concurrency: 4
  default_network_policy: restricted
  allowed_images:
    - registry.example.com/cynode/sandboxes/python:3.12
    - registry.example.com/cynode/sandboxes/node:22
  mounts:
    allowed_host_paths:
      - /var/lib/cynode/shared
    read_only_by_default: true
  resources:
    max_cpu_cores: 8
    max_memory_mb: 16384
    max_pids: 2048
  timeouts:
    default_seconds: 900
    max_seconds: 3600
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

On startup, the Node Manager MUST contact the orchestrator before starting the Ollama container.
This ensures the orchestrator can select an Ollama container image compatible with the node and can apply current policy.

The system requires that the overall deployment has at least one inference-capable path.
Inference may be provided by node-local inference (Ollama) or by external model routing through API Egress when configured.
In the single-node case, the system MUST refuse to enter a ready state if the node cannot run the inference container and there is no configured external provider key.
See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md) and [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md).

Recommended startup flow

- Start Node Manager system service.
- Load node startup YAML and apply node-local constraints.
- Collect host capabilities.
- Register with orchestrator and send capability report.
- Fetch the latest node configuration from orchestrator.
- Start the worker API service.
- Start the single Ollama container specified by the orchestrator, when configured for inference.
- Report startup status and effective configuration version to the orchestrator.

## Ollama Container Policy

The node MUST run at most one Ollama container at a time.
That container MUST be granted access to all GPUs and NPUs on the system.

Rationale

- Centralizes accelerator scheduling for model inference.
- Avoids conflicting GPU allocation and reduces operational complexity.

## Sandbox-Only Nodes

CyNodeAI SHOULD support nodes that do not provide AI inference capabilities.
These nodes exist to run sandbox containers for tool execution, builds, tests, and other compute tasks.

### Sandbox-Only Nodes Applicable Requirements

- Spec ID: `CYNAI.WORKER.SandboxOnlyNodes` <a id="spec-cynai-worker-sandboxonlynodes"></a>

Traces To:

- [REQ-WORKER-0123](../requirements/worker.md#req-worker-0123)
- [REQ-WORKER-0124](../requirements/worker.md#req-worker-0124)
- [REQ-WORKER-0125](../requirements/worker.md#req-worker-0125)
- [REQ-WORKER-0126](../requirements/worker.md#req-worker-0126)

Capability reporting guidance

- Sandbox-only nodes SHOULD report `gpu` capabilities as absent.
- Sandbox-only nodes SHOULD include labels that indicate sandbox execution is supported.
- Sandbox-only nodes SHOULD include labels that indicate inference is not supported.

## Registration and Bootstrap

During registration, the node establishes trust with the orchestrator and receives a bootstrap configuration payload.
Canonical payload shapes are defined in [`docs/tech_specs/node_payloads.md`](node_payloads.md).

Recommended flow

- Node registers using a pre-shared key (PSK).
- Node sends a capability report as part of registration and on startup.
- Orchestrator validates the node and issues a JWT for ongoing communication.
- Orchestrator returns a bootstrap payload that includes:
  - orchestrator base URL and required service endpoints
  - trust material (e.g. CA bundle or pinned certificate), when applicable
  - pull endpoints and credentials required for orchestrator-provided services

## Capability Reporting

Nodes MUST report host capabilities to the orchestrator so the orchestrator can select compatible configuration and schedule work safely.
Nodes SHOULD report capabilities during registration and again on every node startup.
Canonical payload shapes are defined in [`docs/tech_specs/node_payloads.md`](node_payloads.md).

Recommended capability fields

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

- Nodes SHOULD compute and report a stable capability hash.
- If capabilities change (hardware change, driver change, runtime change), the node MUST report an updated capability report.

## Configuration Delivery

The orchestrator MUST be able to deliver and update configuration for registered nodes.
Canonical payload shapes are defined in [`docs/tech_specs/node_payloads.md`](node_payloads.md).

Recommended behavior

- The node MUST support receiving configuration at registration time.
- The node SHOULD support configuration refresh, either by polling or by a push notification.
- The node MUST validate configuration authenticity and origin before applying it.
- The node SHOULD report configuration application status back to the orchestrator.

## Dynamic Configuration Updates

The orchestrator MUST be able to update node configuration after registration.
This enables rotating credentials, changing registry endpoints, and applying new policy.

Recommended behavior

- The orchestrator SHOULD version node configuration payloads.
- The node SHOULD poll for configuration updates or receive them via a push channel.
- The node MUST apply configuration updates atomically where possible and MUST roll back on failure.
- The node MUST acknowledge applied configuration version to the orchestrator.
- The node SHOULD request a configuration refresh on startup and when capability reports change.

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

## Required Node Configuration

The orchestrator SHOULD configure nodes with:

- Orchestrator endpoints
  - worker API target URLs and registration endpoints
  - service discovery endpoints, if available
- Sandbox image registry
  - registry URL and pull credentials
  - policies such as digest pinning requirements
- Model cache
  - cache endpoint URL and pull credentials
  - policies such as public internet download disablement
- Network and security policy
  - allowed egress to orchestrator-provided services
  - certificate trust material for HTTPS
