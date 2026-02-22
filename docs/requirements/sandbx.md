# SANDBX Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `SANDBX` domain.
It covers sandbox execution, container constraints, and isolation requirements.
When outbound egress is permitted by policy, it is only via worker proxies (inference, Web Egress Proxy, API Egress); sandboxes are not airgapped but have strict controls on inbound and outbound traffic.

## 2 Requirements

- **REQ-SANDBX-0001:** No orchestrator/Git/provider credentials in sandboxes; outbound may be blocked; no inbound SSH; exec and logs via runtime; ephemeral workspace; no direct DB; artifact upload/download.
  [CYNAI.SANDBX.SandboxThreatModel](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxthreat)
  [CYNAI.SANDBX.SandboxConnectivity](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxconn)
  <a id="req-sandbx-0001"></a>
- **REQ-SANDBX-0002:** Sandbox image registry and versioning.
  [CYNAI.SANDBX.Doc.SandboxImageRegistry](../tech_specs/sandbox_image_registry.md#spec-cynai-sandbx-doc-sandboximageregistry)
  <a id="req-sandbx-0002"></a>
- **REQ-SANDBX-0100:** Sandbox containers MUST NOT include orchestrator credentials, Git credentials, or external provider API keys.
  [CYNAI.SANDBX.SandboxThreatModel](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxthreat)
  <a id="req-sandbx-0100"></a>
- **REQ-SANDBX-0101:** Sandbox containers MUST assume that all outbound network access may be blocked by policy.
  [CYNAI.SANDBX.SandboxThreatModel](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxthreat)
  <a id="req-sandbx-0101"></a>
- **REQ-SANDBX-0102:** Sandbox images MUST NOT assume inbound connectivity for control (no SSH requirement).
  [CYNAI.SANDBX.SandboxConnectivity](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxconn)
  <a id="req-sandbx-0102"></a>
- **REQ-SANDBX-0103:** Sandbox images MUST work without exposing any listening ports.
  [CYNAI.SANDBX.SandboxConnectivity](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxconn)
  <a id="req-sandbx-0103"></a>
- **REQ-SANDBX-0104:** Sandbox execution MUST support command execution via the container runtime exec mechanism.
  [CYNAI.SANDBX.SandboxConnectivity](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxconn)
  <a id="req-sandbx-0104"></a>
- **REQ-SANDBX-0105:** Logs MUST be available via stdout and stderr for streaming back to the orchestrator.
  [CYNAI.SANDBX.SandboxConnectivity](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxconn)
  <a id="req-sandbx-0105"></a>
- **REQ-SANDBX-0106:** The sandbox SHOULD call a node-local inference endpoint.
  [CYNAI.SANDBX.NodeLocalInference](../tech_specs/sandbox_container.md#spec-cynai-sandbx-nodelocalinf)
  [CYNAI.STANDS.PortsAndEndpoints](../tech_specs/ports_and_endpoints.md#spec-cynai-stands-portsandendpoints)
  <a id="req-sandbx-0106"></a>
- **REQ-SANDBX-0107:** The node MUST NOT require exposing Ollama on a public network interface.
  [CYNAI.SANDBX.NodeLocalInference](../tech_specs/sandbox_container.md#spec-cynai-sandbx-nodelocalinf)
  <a id="req-sandbx-0107"></a>
- **REQ-SANDBX-0108:** A sandbox container MUST have a writable working directory for job execution.
  [CYNAI.SANDBX.FilesystemWorkingDirs](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxfs)
  <a id="req-sandbx-0108"></a>
- **REQ-SANDBX-0109:** The Node Manager SHOULD mount a per-task workspace directory into the sandbox.
  [CYNAI.SANDBX.FilesystemWorkingDirs](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxfs)
  <a id="req-sandbx-0109"></a>
- **REQ-SANDBX-0110:** The sandbox SHOULD treat the working directory as ephemeral unless artifacts are explicitly uploaded.
  [CYNAI.SANDBX.FilesystemWorkingDirs](../tech_specs/sandbox_container.md#spec-cynai-sandbx-sandboxfs)
  <a id="req-sandbx-0110"></a>
- **REQ-SANDBX-0111:** Sandbox execution MUST NOT depend on public internet access.
  [CYNAI.SANDBX.NetworkExpectations](../tech_specs/sandbox_container.md#spec-cynai-sandbx-networkexpect)
  <a id="req-sandbx-0111"></a>
- **REQ-SANDBX-0112:** Required external calls MUST be routed through orchestrator-mediated services when allowed.
  [CYNAI.SANDBX.NetworkExpectations](../tech_specs/sandbox_container.md#spec-cynai-sandbx-networkexpect)
  <a id="req-sandbx-0112"></a>
- **REQ-SANDBX-0113:** Sandboxes MUST NOT write directly to PostgreSQL.
  [CYNAI.SANDBX.ArtifactsDataExchange](../tech_specs/sandbox_container.md#spec-cynai-sandbx-artifactsexchange)
  <a id="req-sandbx-0113"></a>
- **REQ-SANDBX-0114:** Sandboxes SHOULD use artifact upload and download mechanisms for data exchange.
  [CYNAI.SANDBX.ArtifactsDataExchange](../tech_specs/sandbox_container.md#spec-cynai-sandbx-artifactsexchange)
  <a id="req-sandbx-0114"></a>
- **REQ-SANDBX-0115:** The sandbox image registries MAY be configurable as a rank-ordered list; when not configured, the system MUST use the default Docker Hub registry (`docker.io`) only.
  [CYNAI.SANDBX.RegistryOptions](../tech_specs/sandbox_image_registry.md#spec-cynai-sandbx-registryoptions)
  <a id="req-sandbx-0115"></a>
- **REQ-SANDBX-0116:** Worker nodes SHOULD be configured to pull sandbox images only from the configured rank-ordered registry list (or Docker Hub when none configured) and SHOULD NOT pull arbitrary images from the public internet.
  [CYNAI.SANDBX.RegistryOptions](../tech_specs/sandbox_image_registry.md#spec-cynai-sandbx-registryoptions)
  <a id="req-sandbx-0116"></a>
- **REQ-SANDBX-0117:** Agents MUST NOT push images directly to the registry.
  [CYNAI.SANDBX.ImagePublishing](../tech_specs/sandbox_image_registry.md#spec-cynai-sandbx-imagepublishing)
  <a id="req-sandbx-0117"></a>
- **REQ-SANDBX-0118:** Allowed images and their capabilities MUST be stored in PostgreSQL.
  [CYNAI.SANDBX.AllowedImagesCapabilities](../tech_specs/sandbox_image_registry.md#spec-cynai-sandbx-allowedimagescapabilities)
  <a id="req-sandbx-0118"></a>
- **REQ-SANDBX-0119:** Sandbox image registry behavior SHOULD be configurable via PostgreSQL preferences.
  [CYNAI.SANDBX.PreferencesConstraints](../tech_specs/sandbox_image_registry.md#spec-cynai-sandbx-preferencesconstraints)
  <a id="req-sandbx-0119"></a>
- **REQ-SANDBX-0120:** Publishing and use of sandbox images MUST be policy-controlled and audited.
  [CYNAI.SANDBX.AccessControlAuditing](../tech_specs/sandbox_image_registry.md#spec-cynai-sandbx-accesscontrolauditing)
  <a id="req-sandbx-0120"></a>
- **REQ-SANDBX-0121:** The system MUST support long-running sandbox sessions where the same container remains alive across multiple command executions so that an AI model can send commands, receive results, and continue working on the same problem in the same environment.
  [CYNAI.SANDBX.LongRunningSession](../tech_specs/sandbox_container.md#spec-cynai-sandbx-longrunningsession)
  <a id="req-sandbx-0121"></a>
- **REQ-SANDBX-0122:** Within a session sandbox, the workspace MUST persist across command rounds so that state and artifacts from one command are available to the next.
  [CYNAI.SANDBX.LongRunningSession](../tech_specs/sandbox_container.md#spec-cynai-sandbx-longrunningsession)
  <a id="req-sandbx-0122"></a>
- **REQ-SANDBX-0123:** If Git is present in a sandbox image, sandboxes MAY run local-only Git commands against the mounted workspace, but sandboxes MUST NOT perform any Git operation that contacts a remote or requires Git host network access.
  Remote Git operations MUST be performed via Git egress.
  [CYNAI.SANDBX.GitLocalOnly](../tech_specs/sandbox_container.md#spec-cynai-sandbx-gitlocalonly)
  <a id="req-sandbx-0123"></a>

- **REQ-SANDBX-0124:** For sandbox jobs that require full agent capabilities (session sandbox, MCP tools, multi-round agent workflow), the system MUST use only images marked as CyNodeAI agent-compatible (e.g. OCI image label `io.cynodeai.sandbox.agent-compatible="true"` or equivalent in the registry).
  For basic command tasks (single command, no MCP or session), the system MAY use any allowed image that meets the runtime contract, unless policy restricts to agent-compatible images only.
  [CYNAI.SANDBX.ImageCompatibilityTaskType](../tech_specs/sandbox_image_registry.md#spec-cynai-sandbx-imagecompatibilitytasktype)
  <a id="req-sandbx-0124"></a>
- **REQ-SANDBX-0125:** An orchestrator-level system setting MUST control whether the Project Manager agent may add container images to the allowed-images list; the default MUST be disabled.
  When enabled, the PM agent MAY use the MCP tool `sandbox.allowed_images.add` to add images; when disabled, the MCP gateway MUST reject that tool when invoked by the PM agent.
  The PM agent MUST have MCP capability to list allowed images (`sandbox.allowed_images.list`) regardless of this setting.
  [CYNAI.SANDBX.PMAgentAddToAllowedImages](../tech_specs/sandbox_image_registry.md#spec-cynai-sandbx-pmagentaddtoallowedimages)
  <a id="req-sandbx-0125"></a>
- **REQ-SANDBX-0126:** Sandbox images MUST include a POSIX shell (`/bin/sh`), core utilities sufficient for typical build and inspection tasks, a TLS trust store (e.g. ca-certificates), at least one process-status tool (e.g. ps), at least one tool to inspect environment variables (e.g. env or printenv), and archive handling (tar, gzip, unzip).
  [CYNAI.SANDBX.MinimumRequiredSoftware](../tech_specs/sandbox_container.md#spec-cynai-sandbx-minimumrequiredsoftware)
  <a id="req-sandbx-0126"></a>
- **REQ-SANDBX-0127:** Sandbox images SHOULD declare purpose or usage (e.g. Go builds, Python testing) via an OCI image config label so the orchestrator and operators can select images by purpose without inspecting image contents.
  [CYNAI.SANDBX.ContainerPurposeUsageLabeling](../tech_specs/sandbox_container.md#spec-cynai-sandbx-containerpurposeusage)
  <a id="req-sandbx-0127"></a>
- **REQ-SANDBX-0130:** When sandbox web egress is permitted by policy for dependency downloads, sandboxes MUST be configured to use the Web Egress Proxy and MUST NOT have direct outbound internet access that bypasses it.
  [CYNAI.SANDBX.Integration.WebEgressProxy](../tech_specs/web_egress_proxy.md#spec-cynai-sandbx-integration-webegressproxy)
  <a id="req-sandbx-0130"></a>
