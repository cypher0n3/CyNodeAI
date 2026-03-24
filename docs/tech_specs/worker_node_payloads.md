# Node Payloads

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Conventions](#conventions)
- [Security Notes](#security-notes)
  - [Traces to Requirements](#traces-to-requirements)
- [Node Capability Report Payload](#node-capability-report-payload)
  - [Capability Report Schema `node_capability_report_v1`](#capability-report-schema-node_capability_report_v1)
- [Node Bootstrap Payload](#node-bootstrap-payload)
  - [Bootstrap Payload Schema `node_bootstrap_payload_v1`](#bootstrap-payload-schema-node_bootstrap_payload_v1)
- [Node Configuration Payload](#node-configuration-payload)
  - [Node Configuration Payload Requirements Traces](#node-configuration-payload-requirements-traces)
  - [Node Config Schema `node_configuration_payload_v1`](#node-config-schema-node_configuration_payload_v1)
- [Node Configuration Acknowledgement](#node-configuration-acknowledgement)
  - [Node Configuration Acknowledgement Requirements Traces](#node-configuration-acknowledgement-requirements-traces)
  - [Config Ack Schema `node_config_ack_v1`](#config-ack-schema-node_config_ack_v1)
- [Compatibility and Versioning](#compatibility-and-versioning)

## Document Overview

- Spec ID: `CYNAI.WORKER.Doc.NodePayloads` <a id="spec-cynai-worker-doc-nodepayloads"></a>

This document defines the canonical wire payloads exchanged between worker nodes and the orchestrator.
It covers node capability reporting and orchestrator configuration delivery.

This document is the canonical specification for payload shapes, field names, and versioning.
Behavioral requirements remain defined in [`docs/tech_specs/worker_node.md`](worker_node.md).

## Goals

- Define the node capability report payload used during registration and startup.
- Define the bootstrap payload returned during registration.
- Define the versioned node configuration payload used for updates after registration.
- Make payloads auditable and safe to evolve without breaking older nodes.

## Conventions

- Payloads are JSON objects with `snake_case` field names.
- Every payload includes a `version` integer.
- Timestamps are RFC 3339 strings in UTC.
- Optional fields may be omitted when unknown.
- UUID values MUST be encoded as lowercase RFC 4122 strings.

## Security Notes

- Spec ID: `CYNAI.WORKER.PayloadSecurity` <a id="spec-cynai-worker-payloadsecurity"></a>

- Payloads may include secrets (for example short-lived pull tokens).
- Secrets MUST be short-lived where possible and MUST NOT be exposed to sandbox containers.
- Nodes MUST store secrets only in a node-local secure store.

### Traces to Requirements

- [REQ-WORKER-0131](../requirements/worker.md#req-worker-0131)
- [REQ-WORKER-0132](../requirements/worker.md#req-worker-0132)

## Node Capability Report Payload

This payload is sent from a node to the orchestrator during registration and on every startup.
It may also be sent when capabilities change.

Source requirements: [`docs/tech_specs/worker_node.md`](worker_node.md#spec-cynai-worker-capabilityreporting).

### Capability Report Schema `node_capability_report_v1`

- Spec ID: `CYNAI.WORKER.Payload.CapabilityReportV1` <a id="spec-cynai-worker-payload-capabilityreport-v1"></a>

- `version` (int)
  - must be 1
- `reported_at` (string)
  - RFC 3339 UTC timestamp
- `node` (object)
  - `node_slug` (string)
    - MUST match the node startup YAML `node.id`
  - `name` (string, optional)
  - `labels` (array of strings, optional)
- `platform` (object)
  - `os` (string)
  - `distro` (string, optional)
  - `arch` (string)
  - `kernel_version` (string, optional)
- `container_runtime` (object, optional)
  - `runtime` (string)
    - examples: docker, podman
  - `version` (string, optional)
  - `rootless_supported` (boolean, optional)
  - `rootless_enabled` (boolean, optional)
- `compute` (object)
  - `cpu_model` (string, optional)
  - `cpu_cores` (int)
  - `ram_mb` (int)
  - `storage_free_mb` (int, optional)
- `gpu` (object, optional)
  - `present` (boolean)
  - `devices` (array, optional)
    - When multiple GPU types (e.g. AMD and NVIDIA) exist on the node, the node MUST report **all** devices from all supported vendors so the orchestrator can compute **total VRAM per vendor** and select the inference backend variant (rocm vs cuda) for the vendor with the greatest total VRAM.
    - each device:
      - `vendor` (string, optional): e.g. `AMD`, `NVIDIA`; required when reporting for variant selection.
        MAY include `Intel` for future use; **Intel support (variant and inference backend image) is deferred until post-MVP** ([REQ-ORCHES-0175](../requirements/orches.md#req-orches-0175)).
      - `model` (string, optional)
      - `device_id` (string, optional)
      - `vram_mb` (int, optional): Capacity in MiB; required when multiple vendors are present so the orchestrator can sum per vendor.
      - `features` (object, optional)
        - examples: `cuda_capability`, `rocm_version`
- `sandbox` (object, optional)
  - `supported` (boolean)
  - `features` (array of strings, optional)
    - examples: netns, seccomp, cgroups_v2
  - `max_concurrency` (int, optional)
- `network` (object, optional)
  - `orchestrator_reachable` (boolean, optional)
  - `outbound_policy` (string, optional)
    - examples: unrestricted, restricted, allowlist, none
- `inference` (object, optional)
  - The capability report MUST be factual so the orchestrator can make the inference decision; it MAY include a user-defined override (see `mode`) read from local config/env at node startup.
  - `supported` (boolean, optional): Factual: the node has the hardware and runtime capability to run inference (e.g. GPU or CPU, container runtime).
    Derived by the node from local detection, not from user preference.
  - `mode` (string, optional): User-defined override read by the node from local config (node startup YAML, environment variables, etc.) at startup.
    Values: `allow` (no override), `disabled` (operator requires no inference on this node), `require` (operator requires inference when capabilities and policy allow).
    When absent, the orchestrator treats as `allow`.
  - `existing_service` (boolean, optional)
    - When true, the node has detected and is using an inference service (e.g. OLLAMA) already running on the host; the node did not start it.
    - The node MUST set this when it is using a host-existing inference service so the orchestrator can treat the node as inference-capable without instructing it to start a container.
  - `running` (boolean, optional)
    - When true, inference is currently available on this node (either node-managed or existing on host).
- `worker_api` (object, optional but recommended at registration)
  - Node-reported Worker API address so the orchestrator can dispatch jobs to this node.
  - The orchestrator MUST use this address to set or update the node's `worker_api_target_url` unless an explicit operator override is configured.
  - `base_url` (string, required when `worker_api` is present)
    - Full URL the orchestrator MUST use to call the Worker API (e.g. `http://hostname:12090`, `https://worker-01.example.com:12090`).
    - MUST include scheme and authority (host and port).
- `managed_services` (object, optional)
  - Declares whether this node supports orchestrator-directed managed services (long-lived service containers) and related proxy functionality.
  - `supported` (boolean, optional)
  - `features` (array of strings, optional)
    - examples: `service_containers`, `agent_orchestrator_proxy_bidirectional`, `agent_orchestrator_proxy_identity_bound`, `agent_proxy_urls_auto`
    - `agent_orchestrator_proxy_identity_bound` means the node supports identity-bound agent-to-orchestrator proxy endpoints where the worker can deterministically resolve the calling `service_id` from the binding used.
    - `agent_proxy_urls_auto` means the node supports the sentinel `auto` value for `managed_services.services[].orchestrator.{mcp_gateway_proxy_url,ready_callback_proxy_url}` and will generate and inject identity-bound endpoints and report them in `managed_services_status.services[].agent_to_orchestrator_proxy`.
- `managed_services_status` (object, optional)
  - Observed state for managed services running on this node.
  - Nodes SHOULD include this at startup and SHOULD send updated capability reports when managed service state changes.
  - `services` (array of objects, optional)
    - each service:
      - `service_id` (string, required)
      - `service_type` (string, required)
      - `state` (string, required)
        - values: `stopped` | `starting` | `ready` | `unhealthy` | `error`
      - `endpoints` (array of strings, optional)
        - Orchestrator-callable endpoints for this service.
        - Endpoints MUST be worker-mediated by default and MUST NOT rely on direct host-port assumptions.
      - `agent_to_orchestrator_proxy` (object, optional)
        - Worker-generated, identity-bound endpoints for agent-to-orchestrator proxy calls from this managed service runtime.
        - The orchestrator SHOULD ingest and store these endpoints as part of node state for diagnostics and reconciliation.
        - `mcp_gateway_proxy_url` (string, optional)
          - Worker-local proxy URL the managed agent runtime uses for MCP tool calls.
          - This URL MUST be identity-bound to this service instance, so the worker can deterministically resolve the calling `service_id` without any secret in the agent request.
        - `ready_callback_proxy_url` (string, optional)
          - Worker-local proxy URL the managed agent runtime uses for ready/callback signaling.
          - This URL MUST be identity-bound to this service instance, so the worker can deterministically resolve the calling `service_id` without any secret in the agent request.
        - `binding` (string, optional)
          - How identity-binding is achieved for this service instance.
          - Values: `per_service_loopback_listener` | `per_service_uds` | `other`
          - `per_service_uds` means the worker created a dedicated Unix domain socket for this `service_id` and mounted it into only this managed service container.
          - When `binding=per_service_uds`, the `mcp_gateway_proxy_url` and `ready_callback_proxy_url` MUST be `http+unix://...` endpoints.
      - `ready_at` (string, optional)
        - RFC 3339 UTC timestamp
      - `image` (string, optional)
      - `container_id` (string, optional)
      - `restart_count` (int, optional)
      - `observed_generation` (string, optional)
      - `last_error` (string, optional)
- `tls` (object, optional)
  - `trust_material_status` (string, optional)
    - examples: ok, missing, invalid
  - `worker_api_server_cert_pem` (string, optional)
    - When the Worker API is served over HTTPS (e.g. behind a containerized nginx reverse proxy) with a self-signed certificate, the node MUST include the server certificate PEM in the capability report so the orchestrator can trust the worker for subsequent HTTPS connections.
    - Omit when the Worker API is not served over HTTPS or when the certificate is issued by a CA already trusted by the orchestrator.
    - See [`docs/tech_specs/worker_api.md`](worker_api.md#spec-cynai-worker-httpstransportreverseproxy).

Example

```json
{
  "version": 1,
  "reported_at": "2026-02-16T12:00:00Z",
  "node": {
    "node_slug": "sandbox-us-east-1-01",
    "name": "Sandbox Only Node",
    "labels": ["sandbox_only", "region_us_east_1"]
  },
  "platform": {
    "os": "linux",
    "distro": "arch",
    "arch": "amd64",
    "kernel_version": "6.18.9-arch1-2"
  },
  "container_runtime": {
    "runtime": "podman",
    "version": "5.5.0",
    "rootless_supported": true,
    "rootless_enabled": true
  },
  "compute": {
    "cpu_cores": 16,
    "ram_mb": 65536,
    "storage_free_mb": 250000
  },
  "gpu": {
    "present": false
  },
  "sandbox": {
    "supported": true,
    "features": ["netns", "seccomp", "cgroups_v2"],
    "max_concurrency": 4
  },
  "network": {
    "orchestrator_reachable": true,
    "outbound_policy": "restricted"
  },
  "worker_api": {
    "base_url": "http://worker-01.example.com:12090"
  }
}
```

The payload is the canonical representation of the node's capabilities; the orchestrator MUST ingest and store the full JSON (or a normalized snapshot) for scheduling and display.

## Node Bootstrap Payload

This payload is returned by the orchestrator during registration.
It establishes the ongoing communication contract and provides initial configuration hints.

Source requirements: [`docs/tech_specs/worker_node.md`](worker_node.md#spec-cynai-worker-registrationandbootstrap).

### Bootstrap Payload Schema `node_bootstrap_payload_v1`

- Spec ID: `CYNAI.WORKER.Payload.BootstrapV1` <a id="spec-cynai-worker-payload-bootstrap-v1"></a>

- `version` (int)
  - must be 1
- `issued_at` (string)
- `orchestrator` (object)
  - `base_url` (string)
  - `endpoints` (object)
    - `worker_registration_url` (string)
    - `node_config_url` (string)
    - `node_report_url` (string)
- `auth` (object)
  - `node_jwt` (string)
  - `expires_at` (string)
- `trust` (object, optional)
  - `ca_bundle_pem` (string, optional)
  - `pinned_spki_sha256` (string, optional)
- `initial_config_version` (string, optional)
- `pull_credentials` (object, optional)
  - `sandbox_registries` (array of objects, optional): rank-ordered; each has `registry_url` (string), optional `username`, `password`, `token`, `expires_at`
  - `model_cache` (object, optional)
    - `cache_url` (string)
    - `token` (string, optional)
    - `expires_at` (string, optional)

Credential delivery

- Registry and cache pull credentials SHOULD be issued as short-lived tokens.
- When a token is included, `expires_at` SHOULD be present and SHOULD be an RFC 3339 UTC timestamp.
- Tokens SHOULD be rotated by configuration refresh.

## Node Configuration Payload

This payload is delivered by the orchestrator to registered nodes.
It is versioned so nodes can apply updates safely and atomically.

Source requirements: [`docs/tech_specs/worker_node.md`](worker_node.md#spec-cynai-worker-configurationdelivery) and [`docs/tech_specs/worker_node.md`](worker_node.md#spec-cynai-worker-dynamicconfigurationupdates).

### Node Configuration Payload Requirements Traces

- [REQ-ORCHES-0149](../requirements/orches.md#req-orches-0149)

### Node Config Schema `node_configuration_payload_v1`

- Spec ID: `CYNAI.WORKER.Payload.ConfigurationV1` <a id="spec-cynai-worker-payload-configuration-v1"></a>

- `version` (int)
  - must be 1
- `config_version` (string)
  - monotonic version identifier for this node
  - For `version=1`, the orchestrator MUST use a ULID encoded as a 26-character Crockford Base32 string.
  - Nodes MUST compare `config_version` values lexicographically to determine monotonic order.
- `issued_at` (string)
- `node_slug` (string)
- `orchestrator` (object)
  - `base_url` (string)
  - `endpoints` (object)
    - `worker_api_target_url` (string)
    - `node_report_url` (string)
- `sandbox_registries` (array of objects, optional)
  - Rank-ordered list of registries for sandbox image pulls.
  - When absent or empty, the node MUST use a single default: Docker Hub (`docker.io`).
  - Each element:
    - `registry_url` (string): registry host (e.g. `docker.io`, `quay.io`, private host)
    - `pull_token` (string, optional)
    - `pull_token_expires_at` (string, optional)
  - Image resolution and pull follow this order (try first, then next, etc.).
- `require_digest_pinning` (boolean, optional)
  - Applies to sandbox image pulls when present at this level.
- `model_cache` (object)
  - `cache_url` (string)
  - `pull_token` (string, optional)
  - `pull_token_expires_at` (string, optional)
- `policy` (object)
  - `sandbox` (object, optional)
    - `allowed_images` (array of strings, optional)
    - `default_network_policy` (string, optional)
    - `allowed_egress_domains` (array of strings, optional)
    - `allow_privileged` (boolean, optional)
    - `allow_host_network` (boolean, optional)
  - `updates` (object, optional)
    - `enable_dynamic_config` (boolean, optional)
    - `poll_interval_seconds` (int, optional)
    - `allow_service_restart` (boolean, optional)
- `worker_api` (object, optional)
  - `orchestrator_bearer_token` (string, optional)
    - Bearer token the orchestrator will use to authenticate when calling the node Worker API.
    - Must be treated as a secret and must not be exposed to sandbox containers.
  - `orchestrator_bearer_token_expires_at` (string, optional)
    - RFC 3339 UTC timestamp.
    - When present, the node must reject expired tokens and request a configuration refresh.
- `inference_backend` (object, optional)
  - When present, the orchestrator instructs the node to start the local inference backend (e.g. OLLAMA) with the given parameters.
  - The orchestrator computes this field per [orchestrator_inference_container_decision.md](orchestrator_inference_container_decision.md#spec-cynai-orches-inferencecontainerdecision).
  - When absent or when `inference_backend.enabled` is false, the node MUST NOT start an inference container (sandbox-only or inference-disabled node).
  - `enabled` (boolean, optional): When true or when the object is present and the node is inference-capable per capability report, the node MUST start the backend container.
    When false, the node MUST NOT start it.
  - `image` (string, optional): OCI image reference for the inference backend container (e.g. `ollama/ollama`, or a variant-specific image such as `ollama/ollama:rocm`).
    When absent, the node MUST derive the image from the orchestrator-supplied `variant`.
    For **Ollama**: variant `rocm` -> `ollama/ollama:rocm`; variant `cuda` or `cpu` -> `ollama/ollama` or `ollama/ollama:latest` (Ollama publishes no separate cuda tag; the default image is CUDA-capable).
    The node MUST NOT use a node-local default (e.g. a single `OLLAMA_IMAGE` env) that ignores or overrides the orchestrator-supplied variant.
  - `variant` (string, optional): Backend variant derived by the orchestrator from the node capability report (e.g. `cuda`, `rocm`, `cpu`).
    The orchestrator derives variant from **total VRAM per vendor** when multiple GPU types are reported (vendor with greatest total VRAM wins); see [orchestrator_inference_container_decision.md](orchestrator_inference_container_decision.md).
    The node MUST use this to select or configure the correct image or runtime (ROCM for AMD, CUDA for Nvidia).
    When `image` is absent, the node MUST derive the backend container image from this variant (see `image` above).
  - `port` (int, optional): listen port for the inference API (default 11434 for Ollama).
  - `env` (object, optional): orchestrator-directed environment values for the backend container.
    - Keys and values MUST be strings.
    - The node MUST pass these values through when launching the backend container.
    - When a key is omitted, the node MAY use a node-local default for that key.
    - When the orchestrator changes a configured value across config revisions, the node MUST reconcile the backend container so the effective runtime configuration matches the new value.
- `managed_services` (object, optional)
  - Desired state for orchestrator-directed managed services that this node MUST run and supervise.
  - Managed services are long-lived service containers (distinct from per-job sandbox containers).
  - The orchestrator MAY include **multiple** entries with `service_type=pma` and **distinct** `service_id` values (one Project Manager PMA instance per session binding per [CYNAI.ORCHES.PmaInstancePerSessionBinding](orchestrator_bootstrap.md#spec-cynai-orches-pmainstancepersessionbinding)).
  - `services` (array of objects, optional)
    - Each element declares a desired managed service instance on this node.
    - Required fields (minimum):
      - `service_id` (string)
        - Stable id assigned by orchestrator; used as the reconciliation key.
      - `service_type` (string)
        - examples: `pma`, `paa`, `model_cache`, `tooling_proxy`
      - `image` (string)
        - OCI image reference
      - `args` (array of strings, optional)
      - `env` (object, optional)
      - `healthcheck` (object, optional)
        - At minimum: `path` (string), `expected_status` (int)
        - When the node uses Podman, it SHOULD pass container health-check options (e.g. `--health-cmd`) so the container reports `(healthy)` in `podman ps`.
          Docker run does not support inline health checks.
      - `restart_policy` (string, optional)
        - recommended: `always`
      - `network` (object, optional)
      - `resources` (object, optional)
    - Agent runtime services (e.g. `service_type=pma`) MUST additionally include:
      - `role` (string)
      - `inference` (object)
        - `mode` (string): `node_local` | `external` | `remote_node`
        - `base_url` (string, optional): required for `node_local` and `remote_node`
        - `api_egress_base_url` (string, optional): required for `external`
        - `provider_id` (string, optional): optional selector when `external` supports multiple providers
        - `default_model` (string, optional)
        - `warmup_required` (boolean, optional)
        - `backend_env` (object, optional): orchestrator-directed backend environment values the managed service MUST receive when it depends on the same node-local inference backend.
          - Keys and values MUST be strings.
          - The worker MUST pass these values into the managed service container when `mode=node_local` and the configuration includes them.
      - `orchestrator` (object)
        - `mcp_gateway_proxy_url` (string)
          - Worker-proxy URL the agent uses for MCP tool calls; the agent MUST NOT call the orchestrator MCP gateway directly.
          - This value is consumed by the managed agent runtime inside the managed service container.
          - If the value is `auto`, the worker MUST generate and inject an identity-bound endpoint for this specific managed service instance and SHOULD report it in `managed_services_status.services[].agent_to_orchestrator_proxy.mcp_gateway_proxy_url`.
          - If the value is not `auto`, it MUST be a concrete endpoint string in one of these forms:
            - `http://127.0.0.1:<port>/v1/worker/internal/orchestrator/mcp:call` (loopback-only).
            - `http+unix://<percent-encoded-absolute-uds-path>/v1/worker/internal/orchestrator/mcp:call` (HTTP over Unix domain socket).
          - The worker MUST ensure the chosen binding is identity-bound such that the worker can deterministically resolve the calling `service_id` without any secret in the agent request.
        - `ready_callback_proxy_url` (string, optional)
          - Worker-proxy URL for ready/callback signaling; the agent MUST NOT call orchestrator endpoints directly.
          - This value is consumed by the managed agent runtime inside the managed service container.
          - If the value is `auto`, the worker MUST generate and inject an identity-bound endpoint for this specific managed service instance and SHOULD report it in `managed_services_status.services[].agent_to_orchestrator_proxy.ready_callback_proxy_url`.
          - If the value is not `auto`, it MUST be a concrete endpoint string in one of these forms:
            - `http://127.0.0.1:<port>/v1/worker/internal/orchestrator/agent:ready` (loopback-only).
            - `http+unix://<percent-encoded-absolute-uds-path>/v1/worker/internal/orchestrator/agent:ready` (HTTP over Unix domain socket).
          - The worker MUST ensure the chosen binding is identity-bound such that the worker can deterministically resolve the calling `service_id` without any secret in the agent request.
        - `agent_token` (string, optional)
          - Token delivered to the **worker** (in node config); the **worker proxy** MUST hold it and attach it when forwarding agent-originated requests to the orchestrator (e.g. internal proxy to MCP gateway).
            The token MUST NOT be passed to the agent container or given to the agent; agents MUST NOT receive tokens or secrets directly.
            For **PMA**, MCP credentials are **per authenticated user session**; the orchestrator associates each delivered token with **user_id** and session binding metadata as defined in [CYNAI.MCPGAT.PmaSessionTokens](mcp/mcp_gateway_enforcement.md#spec-cynai-mcpgat-pmasessiontokens).
            For **user-scoped** managed agents other than PMA (e.g. PAA when deployed as a managed service), the orchestrator MUST associate the token with the user on whose behalf the agent is acting, so the gateway can resolve user context for preferences, access control, and auditing.
            Traces To: [REQ-WORKER-0164](../requirements/worker.md#req-worker-0164).
            See also: [CYNAI.WORKER.NodeLocalSecureStore](worker_node.md#spec-cynai-worker-nodelocalsecurestore), [CYNAI.WORKER.AgentTokenStorageAndLifecycle](worker_node.md#spec-cynai-worker-agenttokenstorageandlifecycle).
        - `agent_token_expires_at` (string, optional)
          - RFC 3339 UTC timestamp.
          - When present, the worker MUST treat the token as invalid after this time.
          - The worker MUST NOT use expired tokens to forward agent-originated requests to the orchestrator.
          - When applicable, the worker SHOULD request a configuration refresh.
        - `agent_token_ref` (object, optional)
          - Reference for how the **worker** obtains a short-lived token.
          - Raw secrets MUST be handled by the worker only and MUST NOT be given to agents.
          - See [CYNAI.WORKER.Payload.AgentTokenRef](#spec-cynai-worker-payload-agenttokenref).
- `notes` (string, optional)
- `constraints` (object, optional)
  - `max_request_bytes` (int, optional)
  - `max_job_timeout_seconds` (int, optional)

#### Agent Token Reference Object `agent_token_ref`

- Spec ID: `CYNAI.WORKER.Payload.AgentTokenRef` <a id="spec-cynai-worker-payload-agenttokenref"></a>

This section defines the schema and semantics for `managed_services.services[].orchestrator.agent_token_ref`.

##### AgentTokenRef Scope

- `agent_token_ref` is consumed by the worker only.
  The worker MUST NOT pass the reference object to any managed service container or agent runtime.
- Resolution failures MUST fail closed.
  A failure MUST result in no agent token being written to the node-local secure store for that `service_id`.

##### AgentTokenRef Schema

- `kind` (string, required)
  - Defines the resolution mechanism used by the worker.
  - Allowed values: `orchestrator_endpoint`.
- `url` (string, required when `kind=orchestrator_endpoint`)
  - HTTPS URL that the worker calls to obtain a short-lived agent token for this managed service instance.
  - The worker MUST treat any response content as secret material.

## Node Configuration Acknowledgement

Nodes MUST report configuration application status back to the orchestrator.
This acknowledgement allows safe rollout, retries, and visibility.

### Node Configuration Acknowledgement Requirements Traces

- [REQ-WORKER-0135](../requirements/worker.md#req-worker-0135)
- [REQ-WORKER-0137](../requirements/worker.md#req-worker-0137)
- [REQ-WORKER-0254](../requirements/worker.md#req-worker-0254)

### Config Ack Schema `node_config_ack_v1`

- Spec ID: `CYNAI.WORKER.Payload.ConfigAckV1` <a id="spec-cynai-worker-payload-configack-v1"></a>

- `version` (int)
  - must be 1
- `node_slug` (string)
- `config_version` (string)
- `ack_at` (string)
- `status` (string)
  - examples: applied, failed, rolled_back
  - In the current minimum implementation, only `applied` and `failed` are required; `rolled_back` may be supported later.
- `error` (object, optional)
  - `type` (string)
  - `message` (string)
  - `details` (object, optional)
- `effective_config_hash` (string, optional)
- `managed_services_status` (object, optional)
  - Initial endpoint registration snapshot for managed services directed by this config version.
  - The worker SHOULD include this when the applied configuration contains `managed_services.services[]`.
  - The worker MUST include this when the applied configuration contains any agent runtime managed service (for example `service_type=pma`).
  - Semantics:
    - This is an immediate post-apply snapshot, intended to provide initial service endpoint registration and identity-bound agent-to-orchestrator proxy endpoints without waiting for the next periodic capability report.
    - The schema is the same as `node_capability_report_v1.managed_services_status`.
    - For each service in desired state, the worker MUST include at least: `service_id`, `service_type`, `state`, and any known `endpoints` and `agent_to_orchestrator_proxy` values.
    - If `orchestrator.{mcp_gateway_proxy_url,ready_callback_proxy_url}` were `auto`, the worker MUST include the generated concrete endpoints in `managed_services_status.services[].agent_to_orchestrator_proxy` and MUST also inject those values into the managed service container runtime configuration.
    - Services that failed to start MUST be reported with `state=error` and `last_error`.

## Compatibility and Versioning

- Spec ID: `CYNAI.WORKER.Payload.CompatibilityVersioning` <a id="spec-cynai-worker-payload-versioning"></a>

- New fields MAY be added to payloads as optional fields.
- Fields MUST NOT change meaning within the same `version`.
- Nodes SHOULD reject payloads with unsupported `version` values and report a structured error.
  - When rejecting a payload, nodes MUST return a Go REST API Problem Details error with:
    - `type`: `https://cynode.ai/problems/unsupported-payload-version`
    - `title`: `Unsupported payload version`
    - `status`: 400
    - `detail`: includes the received `version` and the supported versions
