# Node Payloads

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Conventions](#conventions)
- [Security Notes](#security-notes)
- [Node Capability Report Payload](#node-capability-report-payload)
  - [Capability Report Schema `node_capability_report_v1`](#capability-report-schema-node_capability_report_v1)
- [Node Bootstrap Payload](#node-bootstrap-payload)
  - [Bootstrap Payload Schema `node_bootstrap_payload_v1`](#bootstrap-payload-schema-node_bootstrap_payload_v1)
- [Node Configuration Payload](#node-configuration-payload)
  - [Node Config Schema `node_configuration_payload_v1`](#node-config-schema-node_configuration_payload_v1)
- [Node Configuration Acknowledgement](#node-configuration-acknowledgement)
  - [Config Ack Schema `node_config_ack_v1`](#config-ack-schema-node_config_ack_v1)
- [Compatibility and Versioning](#compatibility-and-versioning)

## Document Overview

- Spec ID: `CYNAI.WORKER.Doc.NodePayloads` <a id="spec-cynai-worker-doc-nodepayloads"></a>

This document defines the canonical wire payloads exchanged between worker nodes and the orchestrator.
It covers node capability reporting and orchestrator configuration delivery.

This document is the canonical specification for payload shapes, field names, and versioning.
Behavioral requirements remain defined in [`docs/tech_specs/node.md`](node.md).

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

## Security Notes

- Spec ID: `CYNAI.WORKER.PayloadSecurity` <a id="spec-cynai-worker-payloadsecurity"></a>

- Payloads may include secrets (for example short-lived pull tokens).
- Secrets MUST be short-lived where possible and MUST NOT be exposed to sandbox containers.
- Nodes MUST store secrets only in a node-local secure store.

Traces To:

- [REQ-WORKER-0131](../requirements/worker.md#req-worker-0131)
- [REQ-WORKER-0132](../requirements/worker.md#req-worker-0132)

## Node Capability Report Payload

This payload is sent from a node to the orchestrator during registration and on every startup.
It may also be sent when capabilities change.

Source requirements: [`docs/tech_specs/node.md`](node.md#capability-reporting).

### Capability Report Schema `node_capability_report_v1`

- Spec ID: `CYNAI.WORKER.Payload.CapabilityReportV1` <a id="spec-cynai-worker-payload-capabilityreport-v1"></a>

- `version` (int)
  - must be 1
- `reported_at` (string)
- `node` (object)
  - `node_slug` (string)
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
    - each device:
      - `vendor` (string, optional)
      - `model` (string, optional)
      - `device_id` (string, optional)
      - `vram_mb` (int, optional)
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
- `capability_hash` (string, optional)
  - stable hash over the normalized report
- `inference` (object, optional)
  - `supported` (boolean)
  - `mode` (string, optional)
    - examples: allow, disabled
- `tls` (object, optional)
  - `trust_material_status` (string, optional)
    - examples: ok, missing, invalid

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
  "capability_hash": "sha256:...redacted..."
}
```

## Node Bootstrap Payload

This payload is returned by the orchestrator during registration.
It establishes the ongoing communication contract and provides initial configuration hints.

Source requirements: [`docs/tech_specs/node.md`](node.md#registration-and-bootstrap).

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
  - `sandbox_registry` (object, optional)
    - `registry_url` (string)
    - `username` (string, optional)
    - `password` (string, optional)
    - `token` (string, optional)
    - `expires_at` (string, optional)
  - `model_cache` (object, optional)
    - `cache_url` (string)
    - `token` (string, optional)
    - `expires_at` (string, optional)

Credential delivery

- Registry and cache pull credentials SHOULD be issued as short-lived tokens.
- Tokens SHOULD be rotated by configuration refresh.

## Node Configuration Payload

This payload is delivered by the orchestrator to registered nodes.
It is versioned so nodes can apply updates safely and atomically.

Source requirements: [`docs/tech_specs/node.md`](node.md#configuration-delivery) and [`docs/tech_specs/node.md`](node.md#dynamic-configuration-updates).

### Node Config Schema `node_configuration_payload_v1`

- Spec ID: `CYNAI.WORKER.Payload.ConfigurationV1` <a id="spec-cynai-worker-payload-configuration-v1"></a>

- `version` (int)
  - must be 1
- `config_version` (string)
  - monotonic version identifier for this node
- `issued_at` (string)
- `node_slug` (string)
- `orchestrator` (object)
  - `base_url` (string)
  - `endpoints` (object)
    - `worker_api_target_url` (string)
    - `node_report_url` (string)
- `sandbox_registry` (object)
  - `registry_url` (string)
  - `require_digest_pinning` (boolean, optional)
  - `pull_token` (string, optional)
  - `pull_token_expires_at` (string, optional)
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
- `notes` (string, optional)
- `constraints` (object, optional)
  - `max_request_bytes` (int, optional)
  - `max_job_timeout_seconds` (int, optional)

## Node Configuration Acknowledgement

Nodes MUST report configuration application status back to the orchestrator.
This acknowledgement allows safe rollout, retries, and visibility.

Traces To:

- [REQ-WORKER-0137](../requirements/worker.md#req-worker-0137)

### Config Ack Schema `node_config_ack_v1`

- Spec ID: `CYNAI.WORKER.Payload.ConfigAckV1` <a id="spec-cynai-worker-payload-configack-v1"></a>

- `version` (int)
  - must be 1
- `node_slug` (string)
- `config_version` (string)
- `ack_at` (string)
- `status` (string)
  - examples: applied, failed, rolled_back
- `error` (object, optional)
  - `type` (string)
  - `message` (string)
  - `details` (object, optional)
- `effective_config_hash` (string, optional)

## Compatibility and Versioning

- Spec ID: `CYNAI.WORKER.Payload.CompatibilityVersioning` <a id="spec-cynai-worker-payload-versioning"></a>

- New fields MAY be added to payloads as optional fields.
- Fields MUST NOT change meaning within the same `version`.
- Nodes SHOULD reject payloads with unsupported `version` values and report a structured error.
