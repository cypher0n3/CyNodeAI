# Sandbox Allowed Images (PM Agent) MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contracts](#tool-contracts)
  - [`sandbox.allowed_images.list` Operation](#sandboxallowed_imageslist-operation)
  - [`sandbox.allowed_images.add` Operation](#sandboxallowed_imagesadd-operation)
- [Allowlist and Gating](#allowlist-and-gating)

## Document Overview

These tools are available only to the Project Manager agent.
Adding an image to the allowed list is gated by the orchestrator system setting `agents.project_manager.sandbox.allow_add_to_allowed_images` (default `false`).

Related documents

- [Sandbox Image Registry](../sandbox_image_registry.md#spec-cynai-sandbx-pmagentaddtoallowedimages)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope: pm`, `Tools` (single direct invocation per tool name).

## Tool Contracts

- Spec ID: `CYNAI.MCPTOO.SandboxAllowedImagesPmAgent` <a id="spec-cynai-mcptoo-sandboxallowedimagespmagent"></a>

### `sandbox.allowed_images.list` Operation

- **Inputs**: Optional `limit`, `cursor`.
  Scope: `pm`.
- **Outputs**: List of allowed image references (paginated); size-limited.
- **Behavior**: Gateway checks allowlist (PM only), reads allowed images from sandbox image registry, returns list.
  See [sandbox.allowed_images.list Algorithm](#algo-cynai-mcptoo-sandboxallowedimageslist).

#### `sandbox.allowed_images.list` Algorithm

<a id="algo-cynai-mcptoo-sandboxallowedimageslist"></a>

1. Resolve caller; verify PM agent; deny if not on allowlist.
2. Query sandbox image registry for allowed image references; apply optional limit and cursor.
3. Enforce response size limit; return list (and next cursor if paginated).
4. Emit audit record; return result.

#### `sandbox.allowed_images.list` Error Conditions

- Access denied (non-PM or not allowlisted); backend failure.

### `sandbox.allowed_images.add` Operation

- **Inputs**: Required `image_ref` (OCI string); optional `name`, `task_id` (audit).
  Scope: `pm`.
- **Outputs**: Success confirmation or error.
- **Behavior**: Gateway checks system setting `agents.project_manager.sandbox.allow_add_to_allowed_images`; if not `true`, MUST reject.
  Otherwise validates image_ref, adds to registry per [Sandbox Image Registry](../sandbox_image_registry.md#spec-cynai-sandbx-pmagentaddtoallowedimages).
  See [sandbox.allowed_images.add Algorithm](#algo-cynai-mcptoo-sandboxallowedimagesadd).

#### `sandbox.allowed_images.add` Algorithm

<a id="algo-cynai-mcptoo-sandboxallowedimagesadd"></a>

1. Resolve caller; verify PM agent; deny if not on allowlist.
2. Read system setting `agents.project_manager.sandbox.allow_add_to_allowed_images`; if not `true`, return error (operation not allowed).
3. Validate `image_ref` (OCI reference format; non-empty); optional `name` and `task_id` for audit.
4. Insert or upsert image_ref (and name) into sandbox image registry so it may be used for sandbox jobs.
5. Emit audit record (include task_id if provided); return success or error.

#### `sandbox.allowed_images.add` Error Conditions

- Access denied; setting `allow_add_to_allowed_images` is false; invalid image_ref; duplicate or backend failure.

#### Traces To

- [REQ-MCPTOO-0106](../../requirements/mcptoo.md#req-mcptoo-0106)

## Allowlist and Gating

- Spec ID: `CYNAI.MCPTOO.SandboxAllowedImagesAllowlist` <a id="spec-cynai-mcptoo-sandboxallowedimagesallowlist"></a>

- **Allowlist**: PM agent only; see [Project Manager Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-pmagentallowlist).
- **Gating**: `sandbox.allowed_images.add` MUST be rejected when system setting `agents.project_manager.sandbox.allow_add_to_allowed_images` is not `true`.
