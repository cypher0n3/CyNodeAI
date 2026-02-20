# SANDBX Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `SANDBX` domain.
It covers sandbox execution, container constraints, and isolation requirements.

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
- **REQ-SANDBX-0115:** The sandbox image registry MUST be configurable.
  [CYNAI.SANDBX.RegistryOptions](../tech_specs/sandbox_image_registry.md#spec-cynai-sandbx-registryoptions)
  <a id="req-sandbx-0115"></a>
- **REQ-SANDBX-0116:** Worker nodes SHOULD be configured to pull sandbox images from the registry and SHOULD not pull arbitrary images from the public internet.
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
