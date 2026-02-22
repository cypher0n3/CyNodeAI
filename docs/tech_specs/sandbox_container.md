# Sandbox Container

- [Document Overview](#document-overview)
- [Purpose](#purpose)
- [Threat Model](#threat-model)
  - [Threat Model Applicable Requirements](#threat-model-applicable-requirements)
- [Runtime Contract](#runtime-contract)
  - [Runtime Contract Baseline](#runtime-contract-baseline)
  - [Image Compatibility Marking for Agent Workloads](#image-compatibility-marking-for-agent-workloads)
  - [Container Purpose and Usage Labeling](#container-purpose-and-usage-labeling)
- [Sandbox Connectivity](#sandbox-connectivity)
  - [Sandbox Connectivity Applicable Requirements](#sandbox-connectivity-applicable-requirements)
- [Node-Local Inference Access](#node-local-inference-access)
  - [Node-Local Inference Access Applicable Requirements](#node-local-inference-access-applicable-requirements)
- [Minimum Required Software](#minimum-required-software)
  - [`MinimumRequiredSoftware` Required Components](#minimumrequiredsoftware-required-components)
  - [`MinimumRequiredSoftware` Optional and Disallowed](#minimumrequiredsoftware-optional-and-disallowed)
- [Git Behavior (Local-Only)](#git-behavior-local-only)
- [Recommended Optional Tooling Profiles](#recommended-optional-tooling-profiles)
- [SandBox Agent (SBA) Runner Image (Containerfile)](#sandbox-agent-sba-runner-image-containerfile)
  - [SBA Runner Image Minimum Requirements](#sba-runner-image-minimum-requirements)
  - [SBA Runner Image Recommendations](#sba-runner-image-recommendations)
- [Filesystem and Working Directories](#filesystem-and-working-directories)
  - [Filesystem and Working Directories Applicable Requirements](#filesystem-and-working-directories-applicable-requirements)
- [Environment Variables](#environment-variables)
- [Network Expectations](#network-expectations)
  - [Network Expectations Applicable Requirements](#network-expectations-applicable-requirements)
- [Artifacts and Data Exchange](#artifacts-and-data-exchange)
  - [Artifacts and Data Exchange Applicable Requirements](#artifacts-and-data-exchange-applicable-requirements)
- [Long-Running Session Sandboxes](#long-running-session-sandboxes)
  - [`LongRunningSession` Scope](#longrunningsession-scope)
  - [`LongRunningSession` Outcomes](#longrunningsession-outcomes)
- [Non-Requirements](#non-requirements)

## Document Overview

- Spec ID: `CYNAI.SANDBX.Doc.SandboxContainer` <a id="spec-cynai-sandbx-doc-sandboxcontainer"></a>

This document defines the minimum runtime contract for sandbox containers used to execute agent tools and job steps.
It focuses on what MUST be present inside a sandbox container image so dispatched jobs can run reliably and safely.

## Purpose

Sandbox containers provide isolated execution for tools and job steps.
They are started and stopped by the Node Manager on a worker node using a container runtime (Docker or Podman; Podman preferred for rootless).

See [`docs/tech_specs/worker_node.md`](worker_node.md) and [`docs/tech_specs/sandbox_image_registry.md`](sandbox_image_registry.md).

## Threat Model

Assumptions

- Sandbox code is untrusted.
- Sandbox containers are network-restricted by default.
  When outbound egress is permitted by policy, it is only via worker proxies (inference proxy, Web Egress Proxy, API Egress); sandboxes are not airgapped but have strict controls on what is allowed in and out.
- Sandboxes must not contain long-lived secrets.

### Threat Model Applicable Requirements

- Spec ID: `CYNAI.SANDBX.SandboxThreatModel` <a id="spec-cynai-sandbx-sandboxthreat"></a>

Traces To:

- [REQ-SANDBX-0100](../requirements/sandbx.md#req-sandbx-0100)
- [REQ-SANDBX-0101](../requirements/sandbx.md#req-sandbx-0101)

## Runtime Contract

The orchestrator dispatches sandbox jobs with an expected runtime contract.
Sandbox images MUST meet this contract for a given capability profile.

### Runtime Contract Baseline

- Spec ID: `CYNAI.SANDBX.RuntimeContractBaseline` <a id="spec-cynai-sandbx-runtimecontractbaseline"></a>

Traces To:

- [REQ-SANDBX-0104](../requirements/sandbx.md#req-sandbx-0104)
- [REQ-SANDBX-0105](../requirements/sandbx.md#req-sandbx-0105)
- [REQ-SANDBX-0108](../requirements/sandbx.md#req-sandbx-0108)

Implementations MUST satisfy the following for every sandbox image used for dispatched jobs.

- The container MUST be able to execute arbitrary shell commands (via the container runtime exec mechanism per REQ-SANDBX-0104).
- The container MUST support reading and writing files in a writable workspace directory (per REQ-SANDBX-0108).
- The container MUST exit with a meaningful exit code (process exit code observable by the runtime).
- The container MUST produce stdout and stderr streams for capture by the node (per REQ-SANDBX-0105).

### Image Compatibility Marking for Agent Workloads

- Spec ID: `CYNAI.SANDBX.ImageCompatibilityMarking` <a id="spec-cynai-sandbx-imagecompatibilitymarking"></a>

Traces To:

- [REQ-SANDBX-0124](../requirements/sandbx.md#req-sandbx-0124)

Images used for full agent sandbox workloads (session sandbox, MCP tools, multi-round agent interaction) MUST be marked so the system can identify CyNodeAI-compatible containers without launching them.

- Use an OCI image config label (Containerfile: `LABEL`): `io.cynodeai.sandbox.agent-compatible="true"`.
- Images that do not carry this label may still be used for basic command tasks (single command, no MCP or session) if they meet the baseline runtime contract above and are in the allowed image list.

See [Image Compatibility Marking and Task Type](sandbox_image_registry.md#spec-cynai-sandbx-imagecompatibilitytasktype) in [`sandbox_image_registry.md`](sandbox_image_registry.md) for task-type rules and selection behavior.

### Container Purpose and Usage Labeling

- Spec ID: `CYNAI.SANDBX.ContainerPurposeUsageLabeling` <a id="spec-cynai-sandbx-containerpurposeusage"></a>

Traces To:

- [REQ-SANDBX-0127](../requirements/sandbx.md#req-sandbx-0127)

Images SHOULD declare what the container is intended for (e.g. Go builds, Python testing, Node.js tooling) so the orchestrator and operators can select images by purpose without inspecting image contents.

#### `ContainerPurposeUsageLabeling` Scope

- Applies to all sandbox container images that may be selected by task or agent logic for job execution.
- Purpose is expressed via OCI image config labels only; no runtime discovery is required.

#### `ContainerPurposeUsageLabeling` Industry Alignment

- Use **OCI image config labels** (Containerfile: `LABEL`); keys MUST use a reverse-DNS namespace to avoid collisions, per [OCI Image Spec](https://github.com/opencontainers/image-spec/blob/main/annotations.md) and common practice (e.g. `org.opencontainers.image.*` for OCI-defined, vendor keys such as `io.cynodeai.*` for this project).
- Purpose/usage is expressed as one or more label keys; values are opaque to the spec but SHOULD follow the defined taxonomy below for interoperability.

#### `ContainerPurposeUsageLabeling` Label Key and Value Taxonomy

- **Key**: `io.cynodeai.sandbox.purpose`
- **Value**: Comma-separated list of purpose tokens from the [Recommended Optional Tooling Profiles](#recommended-optional-tooling-profiles) or extended by operators.
  - Standard tokens: `base-tools`, `python-tools`, `node-tools`, `go-tools`, `secops-tools`.
  - Optional generic tokens (implementation-defined): e.g. `build`, `test`, `run`, `dev` when a single image serves multiple roles.
- Example (Containerfile): `LABEL io.cynodeai.sandbox.purpose="go-tools,build,test"`.
- Images MAY omit this label; absence means purpose is unspecified.
  The system MAY infer capabilities from other metadata or registry records.

#### `ContainerPurposeUsageLabeling` Outcomes

- When the label is present and valid, the orchestrator MAY use the stored purpose (at ingest or publish time) in sandbox image registry metadata for planning and image selection.
- Task and agent logic (e.g. Project Manager Agent) MAY use declared purpose to choose an image suitable for a job (e.g. Go compile vs Python lint).
- When the label is absent, purpose is unspecified; the system MAY infer capabilities from other metadata or registry records.

See [Allowed Images and Capabilities](sandbox_image_registry.md#spec-cynai-sandbx-allowedimagescapabilities) in [`sandbox_image_registry.md`](sandbox_image_registry.md) for how capabilities are stored and used.

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

See [`docs/tech_specs/worker_node.md`](worker_node.md) and [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md).

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

See [`docs/tech_specs/worker_node.md`](worker_node.md) and [`docs/tech_specs/ports_and_endpoints.md`](ports_and_endpoints.md#spec-cynai-stands-inferenceollamaandproxy).

## Minimum Required Software

- Spec ID: `CYNAI.SANDBX.MinimumRequiredSoftware` <a id="spec-cynai-sandbx-minimumrequiredsoftware"></a>

Traces To:

- [REQ-SANDBX-0126](../requirements/sandbx.md#req-sandbx-0126)

All sandbox images MUST include the following minimum software.
Implementations MAY satisfy a given bullet with any equivalent tool that meets the stated capability.

### `MinimumRequiredSoftware` Required Components

Operating system and base utilities

- A POSIX shell at **`/bin/sh`** (executable by the container entrypoint or exec).
- Core utilities sufficient to run typical build and inspection tasks.
  The image MUST provide at least: `cat`, `cp`, `mv`, `rm`, `mkdir`, `chmod`, `chown`, `find`, `tar`, `gzip`, `unzip` (or equivalent single-binary or minimal-set implementations that offer the same semantics).
- A TLS trust store (certificate authorities) for TLS validation of outbound connections when egress is permitted.
  Common package name: `ca-certificates`.
- The image SHOULD provide a working `date` implementation for time and locale basics; if absent, job steps that depend on `date` may fail and the image is still compliant.

Process and diagnostics

- At least one process-status tool (e.g. `ps`) visible in the container's default PATH.
- At least one tool to inspect environment variables (e.g. `env` or `printenv`) visible in the container's default PATH.

Compression and archive handling

- **`tar`** - MUST be present.
- **`gzip`** - MUST be present.
- **`unzip`** - MUST be present.
- **`zip`** - MUST be present.

### `MinimumRequiredSoftware` Optional and Disallowed

- Network clients (e.g. `curl`, `wget`) are optional; sandboxes may have no outbound access.
- Git is optional.
  If Git is present, local-only Git commands are allowed; remote-affecting operations (e.g. `git clone`, `git fetch`, `git pull`, `git push`, submodule fetch/update, Git LFS) MUST NOT be performed from inside the sandbox.
  Remote Git operations MUST be performed via Git Egress; see [Git Behavior (Local-Only)](#git-behavior-local-only).

## Git Behavior (Local-Only)

- Spec ID: `CYNAI.SANDBX.GitLocalOnly` <a id="spec-cynai-sandbx-gitlocalonly"></a>

Traces To:

- [REQ-SANDBX-0123](../requirements/sandbx.md#req-sandbx-0123)

This section removes ambiguity about Git usage inside sandboxes.

Allowed (local-only)

- Sandboxes MAY run local-only Git commands against the mounted workspace.
  Examples include `git status`, `git diff`, `git log`, `git add`, and local `git commit`.

Disallowed (remote-affecting)

- Sandboxes MUST NOT contact Git remotes directly.
  This includes `git clone`, `git fetch`, `git pull`, `git push`, submodule fetch/update, Git LFS operations, and any Git remote helper that performs network access.
- Remote Git operations MUST be performed by Git egress, using task-scoped changeset artifacts.
  See `docs/tech_specs/git_egress_mcp.md`.

## Recommended Optional Tooling Profiles

- Spec ID: `CYNAI.SANDBX.RecommendedToolingProfiles` <a id="spec-cynai-sandbx-recommendedtoolingprofiles"></a>

Sandbox images SHOULD declare capabilities in the sandbox image registry.
Operators MAY publish multiple images for different tool profiles.
Purpose tokens used in [Container Purpose and Usage Labeling](#container-purpose-and-usage-labeling) SHOULD match the profile names below for interoperability.

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

## SandBox Agent (SBA) Runner Image (Containerfile)

- Spec ID: `CYNAI.SBAGNT.SbaContainerImage` <a id="spec-cynai-sbagnt-sbacontainerimage"></a>

This section is the **definitive spec** for the SBA runner container image (`cynode-sba`).
It applies to the project's SBA Containerfile and to any party building an SBA-compatible image who does not use the project's image.
Implementations MUST satisfy the minimum requirements below; recommendations are for interoperability and for custom builds.

Traces To:

- [REQ-SBAGNT-0001](../requirements/sbagnt.md#req-sbagnt-0001) (sandbox runner in container)
- [REQ-SBAGNT-0104](../requirements/sbagnt.md#req-sbagnt-0104) (SANDBX compliance)
- [Minimum Required Software](#minimum-required-software) (sandbox baseline)

### SBA Runner Image Minimum Requirements

The image MUST satisfy the [Minimum Required Software](#minimum-required-software) defined in this document for all sandbox images (POSIX shell, core utilities, `ca-certificates`, process/diagnostics tools, `tar`/`gzip`/`unzip`, etc.).
In addition, the SBA runner image MUST include the following.

Runtime and shell

- A shell suitable for step execution and scripting: **`bash`** (or equivalent that supports the same subset used by the SBA and job steps).
- **`/bin/sh`** MUST be present (per sandbox baseline); it MAY be provided by `bash` or another POSIX shell.

Utilities (MUST be present)

- **`coreutils`** - Core GNU/BSD utilities (e.g. `cat`, `cp`, `mv`, `rm`, `mkdir`, `chmod`, `date`).
- **`findutils`** - At least `find`.
- **`diffutils`** - At least `diff` (for `apply_unified_diff` step type and verification).
- **`procps`** - At least `ps` (process status; per sandbox baseline).
- **`tar`**, **`gzip`**, **`unzip`** - Archive handling (per sandbox baseline).
- **`jq`** - JSON parsing and generation for job spec, result contract, and tooling.
- **`ca-certificates`** - TLS trust store for outbound calls via proxies (per sandbox baseline).

Git (local-only)

- **`git`** - For local-only Git operations in the workspace (e.g. `git status`, `git diff`, `git add`, local `git commit`).
  Remote Git operations (clone, fetch, push, etc.) MUST NOT be performed from inside the container; see [Git Behavior (Local-Only)](#git-behavior-local-only).

Python

- **`python3`** - A Python 3 interpreter MUST be available for steps and tooling that depend on it (e.g. scripts, lint, tests).

User and paths

- The container MUST be run as a **non-root** user; the image MUST support running the entrypoint as that user.
- The node mounts or provides **`/workspace`** and **`/job`** at runtime; the image MUST NOT assume host paths and MUST use these agreed paths for job I/O.

### SBA Runner Image Recommendations

Base image

- **Recommended base**: Fedora-based image for richer baseline tooling and package availability than minimal Alpine.
  Implementations that use a different base (e.g. Alpine, Debian, Ubuntu) MUST still satisfy all minimum requirements above.

Optional tooling (for custom or extended images)

- Additional language runtimes (e.g. `node`, `go`) if job steps require them.
- Build tools (e.g. `gcc`, `make`, `pip`, `venv`) when tasks need to compile or install dependencies (subject to job constraints and proxy access).
- Security and lint tooling (e.g. `shellcheck`, `semgrep`) when verification steps depend on them.

Image compatibility marking

- Images used for full agent sandbox workloads (SBA runner, MCP, session) SHOULD be marked per [Image Compatibility Marking for Agent Workloads](#image-compatibility-marking-for-agent-workloads) (e.g. `io.cynodeai.sandbox.agent-compatible="true"`).

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

- Spec ID: `CYNAI.SANDBX.EnvironmentVariables` <a id="spec-cynai-sandbx-environmentvariables"></a>

The Node Manager SHOULD provide a minimal set of environment variables for task context.
These variables MUST NOT contain secrets; see [Threat Model](#threat-model).

Recommended variables

- `CYNODE_TASK_ID`
- `CYNODE_JOB_ID`
- `CYNODE_WORKSPACE_DIR` (e.g. `/workspace`)
- `CYNODE_ARTIFACTS_DIR` (optional staging path)

## Network Expectations

Sandbox network policy is controlled by the orchestrator and the node.
Images MUST work under restricted or no-egress configurations.
When egress is allowed, it is only through orchestrator- and node-mediated proxies (inference proxy, Web Egress Proxy, API Egress); there is no direct outbound internet access from sandboxes.

### Network Expectations Applicable Requirements

- Spec ID: `CYNAI.SANDBX.NetworkExpectations` <a id="spec-cynai-sandbx-networkexpect"></a>

Traces To:

- [REQ-SANDBX-0111](../requirements/sandbx.md#req-sandbx-0111)
- [REQ-SANDBX-0112](../requirements/sandbx.md#req-sandbx-0112)

Relevant controlled services

- API Egress Server: [`docs/tech_specs/api_egress_server.md`](api_egress_server.md)
- Web Egress Proxy: [`docs/tech_specs/web_egress_proxy.md`](web_egress_proxy.md)
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

## Long-Running Session Sandboxes

- Spec ID: `CYNAI.SANDBX.LongRunningSession` <a id="spec-cynai-sandbx-longrunningsession"></a>

Traces To:

- [REQ-SANDBX-0121](../requirements/sandbx.md#req-sandbx-0121)
- [REQ-SANDBX-0122](../requirements/sandbx.md#req-sandbx-0122)

For longer-running tasks, the system supports **session sandboxes**: a single container that stays alive across multiple command executions.
The AI model (via the orchestrator) can send a command, receive the result, then send another command in the same container, so that work continues in the same environment with persistent workspace state.

### `LongRunningSession` Scope

- The same container is reused for multiple exec rounds; the container is not torn down after each command.
- Control is still via the container runtime (exec); no inbound SSH or long-lived network server inside the sandbox is required.
- The same threat model, connectivity rules, and artifact exchange apply as for single-shot sandboxes.

### `LongRunningSession` Outcomes

- The workspace directory (e.g. `/workspace`) persists across command rounds; files and state written by one command are visible to the next.
- The orchestrator identifies the session with a stable session identifier and optionally `task_id`; the node uses this for auditing and cleanup.
- Session lifetime and idle timeout are enforced by the node or orchestrator so that long-running sandboxes do not run indefinitely without bounds.

Optional interactive terminal support:

- Session sandboxes MAY be controlled using an interactive PTY stream exposed by the node via the Worker API.
  PTY mode MUST NOT require inbound network connectivity to the sandbox and MUST rely on container runtime primitives.

See [`docs/tech_specs/worker_api.md`](worker_api.md) for the Worker API contract for creating a session, executing commands in the session, ending the session, and PTY support.

## Non-Requirements

The following are explicitly not required in all sandbox images.

- A specific language runtime.
- Git credentials or configured remotes.
- Direct access to external APIs.
