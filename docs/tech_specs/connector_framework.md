# Connector Framework

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Connector Model](#connector-model)
- [Credential Storage and Rotation](#credential-storage-and-rotation)
- [Policy and Auditing](#policy-and-auditing)
- [Tool Surface (MCP)](#tool-surface-mcp)
- [User API Gateway Surface](#user-api-gateway-surface)
- [Initial Connectors](#initial-connectors)
- [OpenClaw Connector Compatibility](#openclaw-connector-compatibility)
- [Connector MCP Adapters](#connector-mcp-adapters)

## Document Overview

This document defines a standardized connector framework for CyNodeAI.
Connectors provide consistent auth, policy, and auditing patterns for external integrations.
The connector framework is exposed to users via the User API Gateway and to orchestrator-side agents via MCP tools.

## Goals

- Provide a first-class connector catalog with install, enable, disable, and lifecycle management.
- Store connector credentials encrypted at rest and support rotation and revocation.
- Enforce per-operation policy controls (read, send, admin) and produce auditable trails.
- Ship a small initial connector set and expand later without changing the framework contract.

## Connector Model

Terminology

- **Connector type**: A named integration implementation (e.g. `imap`, `mattermost`, `discord`).
- **Connector instance**: An installed configuration of a connector type for a specific owner and scope.
- **Operation**: A named action exposed by a connector instance (e.g. read inbox, post message).

Normative requirements

- The orchestrator MUST maintain a connector catalog of supported connector types and their operations.
- Users MUST be able to install a connector instance, enable it, disable it, and uninstall it.
- A connector instance MUST be scoped to an owner identity (user or group) and MAY be scoped to a project.
- Connector instances MUST have stable identifiers for policy rules and auditing.

Recommended connector instance fields

- `id` (uuid, pk)
- `owner_type` (text)
  - one of: user|group
- `owner_id` (uuid)
- `project_id` (uuid, nullable)
- `connector_type` (text)
  - examples: imap, mattermost, discord
- `display_name` (text)
- `is_enabled` (boolean)
- `config` (jsonb)
  - non-secret configuration (e.g. server host, channel ids, mailbox filters)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Runtime boundaries

- Worker sandboxes MUST NOT contain connector credentials.
- Worker agents MUST NOT call external services directly for connector operations.
- Connector operations MUST be performed by orchestrator-controlled services with policy enforcement and auditing.

## Credential Storage and Rotation

Connector secrets MUST be encrypted at rest.
Credential handling SHOULD follow the API Egress Server model.

Normative requirements

- Connector credentials MUST be stored in PostgreSQL as ciphertext with a key identifier for envelope encryption.
- Only the service responsible for executing connector operations MUST be able to decrypt connector credentials.
- Credential rotation MUST be supported without changing connector instance identifiers.
- Credential revocation MUST support immediate deactivation.

See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md#credential-storage).

## Policy and Auditing

Connector operations MUST be governed by access control and audited.

Normative requirements

- The orchestrator MUST enforce a default-deny policy for connector operations.
- Policy MUST be evaluated per operation with subject, action, resource, and task context.
- Connector operations MUST emit audit logs including subject identity, connector instance, operation, and decision.

Recommended policy action taxonomy

- `connector.read`
- `connector.send`
- `connector.admin`

Recommended resource patterns

- connector instance id
- connector type
- operation name (provider operation)

See [`docs/tech_specs/access_control.md`](access_control.md) for policy evaluation and audit log guidance.

## Tool Surface (MCP)

Orchestrator-side agents SHOULD use MCP tools to manage and invoke connector operations.

Normative requirements

- Connector management tools (install, enable, disable, rotate, revoke) MUST be restricted to authorized subjects.
- Connector invocation tools (read, send) MUST be policy-gated per operation and per connector instance.
- Tool calls MUST be audited with task context when invoked during task execution.

Recommended MCP tools

- `connector.catalog.list`
- `connector.instance.install`
- `connector.instance.enable`
- `connector.instance.disable`
- `connector.instance.uninstall`
- `connector.credential.rotate`
- `connector.credential.revoke`
- `connector.op.invoke`

See [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md).

## User API Gateway Surface

The scheduler, connectors, and other user-facing capabilities are exposed through the User API Gateway.
Connector management and connector operation visibility MUST be available to authenticated users.

Normative requirements

- The User API Gateway MUST expose endpoints to install, enable, disable, and uninstall connector instances.
- The User API Gateway MUST expose endpoints to manage connector credentials (create, rotate, revoke, disable).
- The User API Gateway SHOULD expose connector run history and audit views for visibility and debugging.

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md) and [`docs/tech_specs/data_rest_api.md`](data_rest_api.md).

## Initial Connectors

Initial connector types to ship

- IMAP
  - inbound email read and metadata retrieval.
- Mattermost
  - post message and read channel history, subject to policy.
- Discord
  - post message and read channel history, subject to policy.

Constraints

- Initial connectors SHOULD share the same policy and auditing model.
- HTTP-based connectors SHOULD route outbound calls through the API Egress Server when feasible.
- Non-HTTP connectors (e.g. IMAP) MUST still use allowlists and the same audit and policy evaluation model.

## OpenClaw Connector Compatibility

CyNodeAI MAY support reusing existing OpenClaw connectors as connector implementations.
Reused connectors MUST adhere to the CyNodeAI security model.

Key principle

- OpenClaw connector code and responses MUST be treated as untrusted.

Normative requirements

- OpenClaw connectors MUST run behind an orchestrator-controlled boundary.
- The boundary MUST enforce policy, auditing, allowlists, and response validation for every connector operation.
- Connector credentials MUST NOT be returned to agents or worker sandboxes.
- OpenClaw connectors MUST NOT be executed inside worker sandboxes that run arbitrary agent code.
- Connector invocation MUST be scoped to a connector instance id and operation name with subject identity and task context.

Recommended adapter approaches

- MCP adapter (preferred)
  - Wrap OpenClaw connectors behind an MCP server that exposes allowlisted `connector.*` tools only.
  - Apply per-project enablement and a tool allowlist for the MCP server.
- Internal connector service
  - Load OpenClaw connectors as plugins into a dedicated connector service process.
  - Apply the same enforcement boundary: policy checks, auditing, allowlists, rate limits, and strict response normalization.

Credential handling guidance

- Prefer orchestrator-controlled credential injection where possible.
- If a connector requires direct credential access, provide credentials only within the connector boundary and only for the duration of the operation.
- The boundary SHOULD restrict egress to the minimum required endpoints per connector type and operation.

Response handling guidance

- Responses MUST be schema-validated and size-limited.
- Responses MUST be normalized and redacted to prevent accidental secret leakage.

## Connector MCP Adapters

Connectors MAY be implemented as internal services or as MCP servers.
Connector MCP servers MUST follow governance requirements: allowlists, sandbox expectations, response validation, and per-project enablement.

See [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md).
