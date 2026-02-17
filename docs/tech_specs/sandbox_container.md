# Sandbox Container

- [Document Overview](#document-overview)
- [Purpose](#purpose)
- [Threat Model](#threat-model)
- [Runtime Contract](#runtime-contract)
- [Sandbox Connectivity](#sandbox-connectivity)
- [Node-Local Inference Access](#node-local-inference-access)
- [Minimum Required Software](#minimum-required-software)
- [Recommended Optional Tooling Profiles](#recommended-optional-tooling-profiles)
- [Filesystem and Working Directories](#filesystem-and-working-directories)
- [Environment Variables](#environment-variables)
- [Network Expectations](#network-expectations)
- [Artifacts and Data Exchange](#artifacts-and-data-exchange)
- [Non-Requirements](#non-requirements)

## Document Overview

This document defines the minimum runtime contract for sandbox containers used to execute agent tools and job steps.
It focuses on what MUST be present inside a sandbox container image so dispatched jobs can run reliably and safely.

## Purpose

Sandbox containers provide isolated execution for tools and job steps.
They are started and stopped by the Node Manager on a worker node using a container runtime (Docker or Podman; Podman preferred for rootless).

See [`docs/tech_specs/node.md`](node.md) and [`docs/tech_specs/sandbox_image_registry.md`](sandbox_image_registry.md).

## Threat Model

Assumptions

- Sandbox code is untrusted.
- Sandbox containers are network-restricted by default.
- Sandboxes must not contain long-lived secrets.

### Threat Model Applicable Requirements

- Spec ID: `CYNAI.SANDBX.SandboxThreatModel` <a id="spec-cynai-sandbx-sandboxthreat"></a>

Traces To:

- [REQ-SANDBX-0100](../requirements/sandbx.md#req-sandbx-0100)
- [REQ-SANDBX-0101](../requirements/sandbx.md#req-sandbx-0101)

## Runtime Contract

The orchestrator dispatches sandbox jobs with an expected runtime contract.
Sandbox images MUST meet this contract for a given capability profile.

Baseline contract requirements

- The container MUST be able to execute arbitrary shell commands.
- The container MUST support reading and writing files in a writable workspace directory.
- The container MUST exit with a meaningful exit code.
- The container MUST produce stdout and stderr logs for capture by the node.

## Sandbox Connectivity

Agents do not connect to a sandbox container over the network.
Sandbox control is performed by the Node Manager using container runtime operations.

### Sandbox Connectivity Applicable Requirements

- Spec ID: `CYNAI.SANDBX.SandboxConnectivity` <a id="spec-cynai-sandbx-sandboxconn"></a>

Traces To:

- [REQ-SANDBX-0102](../requirements/sandbx.md#req-sandbx-0102)
- [REQ-SANDBX-0103](../requirements/sandbx.md#req-sandbx-0103)
- [REQ-SANDBX-0104](../requirements/sandbx.md#req-sandbx-0104)
- [REQ-SANDBX-0105](../requirements/sandbx.md#req-sandbx-0105)

See [`docs/tech_specs/node.md`](node.md) and [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md).

## Node-Local Inference Access

When a sandbox job is scheduled on a node that also provides Ollama inference, the sandbox SHOULD be able to access inference without leaving the node.

### Node-Local Inference Access Applicable Requirements

- Spec ID: `CYNAI.SANDBX.NodeLocalInference` <a id="spec-cynai-sandbx-nodelocalinf"></a>

Traces To:

- [REQ-SANDBX-0106](../requirements/sandbx.md#req-sandbx-0106)
- [REQ-SANDBX-0107](../requirements/sandbx.md#req-sandbx-0107)

Recommended approach

- The Node Manager provides a pod-local `localhost` endpoint for inference.
- The sandbox uses `http://localhost:11434` for inference calls inside the pod.

See [`docs/tech_specs/node.md`](node.md).

## Minimum Required Software

All sandbox images MUST include the following minimum software.

Operating system and base utilities

- A POSIX shell (`/bin/sh`).
- Core utilities sufficient to run typical build and inspection tasks.
  - Examples: `cat`, `cp`, `mv`, `rm`, `mkdir`, `chmod`, `chown`, `find`, `tar`, `gzip`, `unzip`.
- Certificate authorities for TLS validation.
  - Package example: `ca-certificates`.
- Time and locale basics.
  - The image SHOULD provide a working `date` implementation.

Process and diagnostics

- A process status tool.
  - Examples: `ps`.
- A tool to inspect environment variables.
  - Examples: `env` or `printenv`.

Compression and archive handling

- `tar`
- `gzip`
- `unzip`

Notes

- Network clients such as `curl` or `wget` are optional because sandboxes may have no outbound access.
- Git is optional because Git operations requiring credentials SHOULD be performed via Git Egress.

## Recommended Optional Tooling Profiles

Sandbox images SHOULD declare capabilities in the sandbox image registry.
Operators MAY publish multiple images for different tool profiles.

Recommended profiles

- **base-tools**
  - Minimum required software only.
- **python-tools**
  - Python runtime and common build tools.
  - Examples: `python3`, `pip`, `venv`, `gcc`, `make`.
- **node-tools**
  - Node.js runtime and package tooling.
  - Examples: `node`, `npm` or `pnpm`.
- **go-tools**
  - Go toolchain for builds and tests.
  - Example: `go`.
- **secops-tools**
  - Security and scanning tools.
  - Examples: `shellcheck`, `semgrep` (implementation-defined).

## Filesystem and Working Directories

The following requirements apply.

### Filesystem and Working Directories Applicable Requirements

- Spec ID: `CYNAI.SANDBX.FilesystemWorkingDirs` <a id="spec-cynai-sandbx-sandboxfs"></a>

Traces To:

- [REQ-SANDBX-0108](../requirements/sandbx.md#req-sandbx-0108)
- [REQ-SANDBX-0109](../requirements/sandbx.md#req-sandbx-0109)
- [REQ-SANDBX-0110](../requirements/sandbx.md#req-sandbx-0110)

Recommended paths

- Working directory: `/workspace`
- Temporary directory: `/tmp`

## Environment Variables

The Node Manager SHOULD provide a minimal set of environment variables for task context.
These variables MUST NOT contain secrets.

Recommended variables

- `CYNODE_TASK_ID`
- `CYNODE_JOB_ID`
- `CYNODE_WORKSPACE_DIR` (e.g. `/workspace`)
- `CYNODE_ARTIFACTS_DIR` (optional staging path)

## Network Expectations

Sandbox network policy is controlled by the orchestrator and the node.
Images MUST work under restricted or no-egress configurations.

### Network Expectations Applicable Requirements

- Spec ID: `CYNAI.SANDBX.NetworkExpectations` <a id="spec-cynai-sandbx-networkexpect"></a>

Traces To:

- [REQ-SANDBX-0111](../requirements/sandbx.md#req-sandbx-0111)
- [REQ-SANDBX-0112](../requirements/sandbx.md#req-sandbx-0112)

Relevant controlled services

- API Egress Server: [`docs/tech_specs/api_egress_server.md`](api_egress_server.md)
- Secure Browser Service: [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md)
- Git Egress MCP: [`docs/tech_specs/git_egress_mcp.md`](git_egress_mcp.md)

## Artifacts and Data Exchange

Sandboxes exchange data with the orchestrator through artifacts and orchestrator-managed endpoints.

### Artifacts and Data Exchange Applicable Requirements

- Spec ID: `CYNAI.SANDBX.ArtifactsDataExchange` <a id="spec-cynai-sandbx-artifactsexchange"></a>

Traces To:

- [REQ-SANDBX-0113](../requirements/sandbx.md#req-sandbx-0113)
- [REQ-SANDBX-0114](../requirements/sandbx.md#req-sandbx-0114)

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md) and [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md).

## Non-Requirements

The following are explicitly not required in all sandbox images.

- A specific language runtime.
- Git credentials or configured remotes.
- Direct access to external APIs.
