# Sandbox Image Registry

- [Document Overview](#document-overview)
- [Registry Options](#registry-options)
- [Image Publishing Workflow](#image-publishing-workflow)
- [Node Pull Workflow](#node-pull-workflow)
- [Allowed Images and Capabilities](#allowed-images-and-capabilities)
- [Image Compatibility Marking and Task Type](#image-compatibility-marking-and-task-type)
- [Preferences and Constraints](#preferences-and-constraints)
- [Access Control and Auditing](#access-control-and-auditing)

## Document Overview

- Spec ID: `CYNAI.SANDBX.Doc.SandboxImageRegistry` <a id="spec-cynai-sandbx-doc-sandboximageregistry"></a>

This document defines sandbox container image registries used for sandbox execution containers.
Users configure a rank-ordered list of registries (e.g. private, Quay, Docker Hub); worker nodes pull sandbox images according to that order, and the orchestrator controls what images are allowed.
Image metadata and capabilities are stored in PostgreSQL for planning, routing, and verification.

## Registry Options

- Spec ID: `CYNAI.SANDBX.RegistryOptions` <a id="spec-cynai-sandbx-registryoptions"></a>

The user MAY configure a rank-ordered list of container registries for sandbox images.
When the list is empty or not specified in deployment or node configuration, the system MUST use a single default registry: Docker Hub (`docker.io`).

Traces To:

- [REQ-SANDBX-0115](../requirements/sandbx.md#req-sandbx-0115)
- [REQ-SANDBX-0116](../requirements/sandbx.md#req-sandbx-0116)

- Rank-ordered registry list
  - Users MAY specify multiple registries in order (e.g. private registry, Quay, orchestrator-hosted such as Project Zot, Docker Hub).
  - Image resolution and pull follow this order (e.g. try first registry, then next); image references may be short (e.g. `python:3.12`) or fully qualified (e.g. `quay.io/org/image:tag`, `docker.io/library/python:3.12`).
  - Each list entry typically includes registry URL and optional pull credentials.
- Default when list is empty or absent
  - When no registry list is configured, the effective list is a single entry: Docker Hub (`docker.io`).
  - No deployment-specific registry or credentials are required in that case.
- Orchestrator-hosted registry
  - Users MAY deploy a registry on the orchestrator host (e.g. Project Zot, an open-source OCI-native registry) with a sane configuration (private by default, TLS, authenticated access) and include it in the ordered list.

When one or more registries are configured, worker nodes SHOULD pull sandbox images only from the configured list (in order) and SHOULD NOT pull arbitrary images from the public internet outside that list.
When only the default is used, nodes pull from `docker.io` according to allowed image policy.

## Image Publishing Workflow

- Spec ID: `CYNAI.SANDBX.ImagePublishing` <a id="spec-cynai-sandbx-imagepublishing"></a>

Agents MUST NOT push images directly to the registry.
Instead, agents submit a publish request to the orchestrator, which performs validation and publishing.

Traces To:

- [REQ-SANDBX-0117](../requirements/sandbx.md#req-sandbx-0117)

Recommended flow

- Agent submits an image publish request to the orchestrator with task context.
- The orchestrator validates:
  - subject identity and permissions
  - the target repository and tag policy
  - the image metadata and declared capabilities (including OCI label `io.cynodeai.sandbox.agent-compatible` when present)
  - optional image scanning policy, if enabled
- The orchestrator pushes the image to the appropriate registry from the configured rank-ordered list (or to Docker Hub when the list is empty or default).
- The orchestrator records the published image and updates allowed image state in PostgreSQL (e.g. setting `capabilities.agent_compatible` from the image label when present).

## Node Pull Workflow

- Spec ID: `CYNAI.SANDBX.NodePullWorkflow` <a id="spec-cynai-sandbx-nodepullworkflow"></a>

When dispatching a sandbox job, the orchestrator selects an allowed sandbox image that matches the task type (agent vs basic command) and required capabilities.
The target node pulls the image from the first registry in the configured rank-ordered list that can provide it (or from Docker Hub when the list is empty or default) and starts the sandbox container.

Recommended behavior

- The orchestrator selects an image version based on task type (see [Image Compatibility Marking and Task Type](#image-compatibility-marking-and-task-type)), required capabilities, and policy.
- The orchestrator configures the node with the rank-ordered registry list (or default Docker Hub when none configured) and per-registry pull credentials during node registration when applicable.
- The node pulls the referenced image and verifies integrity when possible (digest pinning).
- The node starts the sandbox container using the approved image reference.
- The node reports image pull status and sandbox start status back to the orchestrator.

## Allowed Images and Capabilities

- Spec ID: `CYNAI.SANDBX.AllowedImagesCapabilities` <a id="spec-cynai-sandbx-allowedimagescapabilities"></a>

Allowed images and their capabilities MUST be stored in PostgreSQL.
This enables the Project Manager Agent to choose a sandbox environment appropriate for a task.

Traces To:

- [REQ-SANDBX-0118](../requirements/sandbx.md#req-sandbx-0118)

Database schema

- The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
- Sandbox image registry tables are specified in the [Sandbox Image Registry](postgres_schema.md#spec-cynai-schema-sandboximageregistry) section.
- Canonical table names are `sandbox_images`, `sandbox_image_versions`, and `node_sandbox_image_availability`.

### Sandbox Images Table

- `id` (uuid, pk)
- `name` (text)
  - logical image name (e.g. python-tools, node-build, secops)
- `description` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`name`)
- Index: (`name`)

### Sandbox Image Versions Table

- `id` (uuid, pk)
- `sandbox_image_id` (uuid)
  - foreign key to `sandbox_images.id`
- `version` (text)
  - tag or semantic version
- `image_ref` (text)
  - OCI reference including registry and repository
- `image_digest` (text, nullable)
  - digest for pinning (recommended)
- `capabilities` (jsonb)
  - examples: runtimes, tools, network_requirements, filesystem_requirements; SHOULD include `agent_compatible` (boolean) when known, derived from image label at ingest
- `is_allowed` (boolean)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Unique: (`sandbox_image_id`, `version`)
- Index: (`sandbox_image_id`)
- Index: (`is_allowed`)

### Node Image Availability Table

- `id` (uuid, pk)
- `node_id` (uuid)
- `sandbox_image_version_id` (uuid)
- `status` (text)
  - examples: available, pulling, failed, evicted
- `last_checked_at` (timestamptz)
- `details` (jsonb, nullable)

Constraints

- Unique: (`node_id`, `sandbox_image_version_id`)
- Index: (`node_id`)

## Image Compatibility Marking and Task Type

- Spec ID: `CYNAI.SANDBX.ImageCompatibilityTaskType` <a id="spec-cynai-sandbx-imagecompatibilitytasktype"></a>

Sandbox images MUST be distinguishable so that the system launches only CyNodeAI-compatible containers for agent workloads, unless the task is a basic command task that can run safely in a generic container.

Traces To:

- [REQ-SANDBX-0124](../requirements/sandbx.md#req-sandbx-0124)

### Industry-Standard Marking: OCI Image Label

Images that are built for full CyNodeAI agent sandbox use (session sandboxes, MCP tools, multi-step agent workflows) MUST be marked in an industry-standard way so the orchestrator and nodes can determine compatibility without launching the container.

- Use the OCI image config label (Containerfile: `LABEL`) with key `io.cynodeai.sandbox.agent-compatible` and value `"true"`.
  - In a Containerfile: `LABEL io.cynodeai.sandbox.agent-compatible="true"`.
  - This label is stored in the OCI image config and can be read at pull or ingest time (e.g. when publishing or when syncing allowed images).
- The orchestrator SHOULD record this in the sandbox image registry (e.g. in `capabilities.agent_compatible` or equivalent) when the image is ingested or published so dispatch does not require inspecting the image each time.

### Task Types and Image Selection

- **Agent workload (full capabilities)**
  - Tasks that require full agent sandbox capabilities (e.g. session sandbox, MCP tool execution, multi-round agent interaction) MUST use only images that are marked as CyNodeAI agent-compatible (label present and true).
  - The orchestrator MUST NOT dispatch such work to an image that does not have the agent-compatible marking (or equivalent in the registry).
- **Basic command task**
  - A basic command task is a single command (or short script) execution that does not require full agent capabilities: no MCP tools, no session sandbox, no multi-round agent workflow.
  - For basic command tasks, the system MAY use any allowed image that meets the minimum runtime contract (shell, workspace, exit code, logs) in [`sandbox_container.md`](sandbox_container.md#spec-cynai-sandbx-runtimecontractbaseline), including images that do NOT carry the agent-compatible label (e.g. generic `python:3.12`, `bash`, or other allowed base images).
  - Policy MAY restrict basic command tasks to agent-compatible images only; if so, the same selection rule as agent workloads applies.

### Compatibility Summary

- Only images marked with `io.cynodeai.sandbox.agent-compatible="true"` (or recorded as agent-compatible in the registry) may be used for agent workloads.
- Basic command tasks may use any allowed image that meets the runtime contract, unless policy requires agent-compatible images only.

## Preferences and Constraints

- Spec ID: `CYNAI.SANDBX.PreferencesConstraints` <a id="spec-cynai-sandbx-preferencesconstraints"></a>

Sandbox image registry behavior SHOULD be configurable via PostgreSQL preferences.
These preferences are user-facing task-execution preferences and constraints, not deployment or service configuration.

Traces To:

- [REQ-SANDBX-0119](../requirements/sandbx.md#req-sandbx-0119)

Suggested preference keys

- `sandbox_images.registries` (array of objects, optional)
  - rank-ordered list; each entry: `url` (string), optional `credentials_ref` or per-registry pull token
  - when absent or empty, effective list is Docker Hub (`docker.io`) only
- `sandbox_images.registry.mode` (string, optional, legacy)
  - examples: default, orchestrator_hosted, user_provided; when "default" or omitted, only `docker.io` is used unless `sandbox_images.registries` is set
- `sandbox_images.nodes.prefer_cached_images` (boolean)
- `sandbox_images.allow_public_internet` (boolean)
- `sandbox_images.require_digest_pinning` (boolean)

## Access Control and Auditing

- Spec ID: `CYNAI.SANDBX.AccessControlAuditing` <a id="spec-cynai-sandbx-accesscontrolauditing"></a>

Publishing and use of sandbox images MUST be policy-controlled and audited.

Traces To:

- [REQ-SANDBX-0120](../requirements/sandbx.md#req-sandbx-0120)

- Access control rules SHOULD be defined via [`docs/tech_specs/access_control.md`](access_control.md).
- Actions SHOULD include:
  - image publish requests
  - image pull and sandbox execution requests
  - Project Manager agent adding an image to the allowed-images list (when enabled by system setting)
- The orchestrator SHOULD log:
  - who published an image, which task it was for, and what was published
  - which image version was used for each sandbox execution
  - when the PM agent adds an image to the allowed list (caller, image ref, task context if any)

### PM Agent Add to Allowed Images

- Spec ID: `CYNAI.SANDBX.PMAgentAddToAllowedImages` <a id="spec-cynai-sandbx-pmagentaddtoallowedimages"></a>

Traces To:

- [REQ-SANDBX-0125](../requirements/sandbx.md#req-sandbx-0125)

An orchestrator-level system setting controls whether the Project Manager agent may add container images to the allowed-images list (whitelist).

- System setting: `agents.project_manager.sandbox.allow_add_to_allowed_images` (boolean).
  - Default: `false` (disabled).
  - When `true`, the PM agent MAY call the MCP tool `sandbox.allowed_images.add` to add an image reference to the allowed list; the orchestrator records it in the sandbox image registry and policy so nodes can pull and run it subject to existing policy.
  - When `false`, the MCP gateway MUST reject `sandbox.allowed_images.add` from the PM agent.
- MCP capabilities for the PM agent are defined in [`docs/tech_specs/mcp_tool_catalog.md`](mcp_tool_catalog.md#spec-cynai-mcptoo-sandboxallowedimagespmagent).
- The setting is stored in `system_settings` (see [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md)).

## Future Work

- Publish and maintain official CyNodeAI sandbox images on Docker Hub (`docker.io`) so deployments using the default registry can pull them without custom registry setup.
