# Sandbox Image Registry

- [Document Overview](#document-overview)
- [Registry Options](#registry-options)
- [Image Publishing Workflow](#image-publishing-workflow)
- [Node Pull Workflow](#node-pull-workflow)
- [Allowed Images and Capabilities](#allowed-images-and-capabilities)
- [Preferences and Constraints](#preferences-and-constraints)
- [Access Control and Auditing](#access-control-and-auditing)

## Document Overview

- Spec ID: `CYNAI.SANDBX.Doc.SandboxImageRegistry` <a id="spec-cynai-sandbx-doc-sandboximageregistry"></a>

This document defines a sandbox container image registry used for sandbox execution containers.
Worker nodes pull sandbox images from the configured registry, and the orchestrator controls what images are allowed.
Image metadata and capabilities are stored in PostgreSQL for planning, routing, and verification.

## Registry Options

- Spec ID: `CYNAI.SANDBX.RegistryOptions` <a id="spec-cynai-sandbx-registryoptions"></a>

The sandbox image registry MUST be configurable.

Traces To:

- [REQ-SANDBX-0115](../requirements/sandbx.md#req-sandbx-0115)
- [REQ-SANDBX-0116](../requirements/sandbx.md#req-sandbx-0116)

- User-provided registry
  - Users provide a registry URL and credentials, if needed.
  - The orchestrator uses it as the source of truth for sandbox images.
- Orchestrator-hosted default
  - Users MAY deploy a registry on the orchestrator host.
  - The default SHOULD be Project Zot (an open-source OCI-native registry) with a sane configuration:
    - private by default
    - TLS enabled
    - authenticated access for pushes and pulls

In both modes, worker nodes SHOULD be configured to pull sandbox images from the registry and SHOULD not pull arbitrary images from the public internet.

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
  - the image metadata and declared capabilities
  - optional image scanning policy, if enabled
- The orchestrator pushes the image to the configured registry.
- The orchestrator records the published image and updates allowed image state in PostgreSQL.

## Node Pull Workflow

- Spec ID: `CYNAI.SANDBX.NodePullWorkflow` <a id="spec-cynai-sandbx-nodepullworkflow"></a>

When dispatching a sandbox job, the orchestrator selects an allowed sandbox image with required capabilities.
The target node pulls the image from the configured registry and starts the sandbox container.

Recommended behavior

- The orchestrator selects an image version based on required capabilities and policy.
- The orchestrator configures the node with the correct registry endpoint and pull credentials during node registration.
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
- Sandbox image registry tables are specified in the [Sandbox Image Registry](postgres_schema.md#sandbox-image-registry) section.
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
  - examples: runtimes, tools, network_requirements, filesystem_requirements
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

## Preferences and Constraints

- Spec ID: `CYNAI.SANDBX.PreferencesConstraints` <a id="spec-cynai-sandbx-preferencesconstraints"></a>

Sandbox image registry behavior SHOULD be configurable via PostgreSQL preferences.
These preferences are user-facing task-execution preferences and constraints, not deployment or service configuration.

Traces To:

- [REQ-SANDBX-0119](../requirements/sandbx.md#req-sandbx-0119)

Suggested preference keys

- `sandbox_images.registry.mode` (string)
  - examples: orchestrator_hosted, user_provided
- `sandbox_images.registry.url` (string)
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
- The orchestrator SHOULD log:
  - who published an image, which task it was for, and what was published
  - which image version was used for each sandbox execution
