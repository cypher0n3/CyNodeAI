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

### Connector Model Applicable Requirements

- Spec ID: `CYNAI.CONNEC.ConnectorModel` <a id="spec-cynai-connec-connectormodel"></a>

Traces To:

- [REQ-CONNEC-0100](../requirements/connec.md#req-connec-0100)
- [REQ-CONNEC-0101](../requirements/connec.md#req-connec-0101)
- [REQ-CONNEC-0102](../requirements/connec.md#req-connec-0102)
- [REQ-CONNEC-0103](../requirements/connec.md#req-connec-0103)

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

### Credential Storage and Rotation Applicable Requirements

- Spec ID: `CYNAI.CONNEC.ConnectorCredentialStorage` <a id="spec-cynai-connec-conncredstorage"></a>

Traces To:

- [REQ-CONNEC-0104](../requirements/connec.md#req-connec-0104)
- [REQ-CONNEC-0105](../requirements/connec.md#req-connec-0105)
- [REQ-CONNEC-0106](../requirements/connec.md#req-connec-0106)
- [REQ-CONNEC-0107](../requirements/connec.md#req-connec-0107)

See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md#credential-storage).

## Policy and Auditing

Connector operations MUST be governed by access control and audited.

### Policy and Auditing Applicable Requirements

- Spec ID: `CYNAI.CONNEC.ConnectorPolicyAuditing` <a id="spec-cynai-connec-connpolicy"></a>

Traces To:

- [REQ-CONNEC-0108](../requirements/connec.md#req-connec-0108)
- [REQ-CONNEC-0109](../requirements/connec.md#req-connec-0109)
- [REQ-CONNEC-0110](../requirements/connec.md#req-connec-0110)

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

### Tool Surface MCP Applicable Requirements

- Spec ID: `CYNAI.CONNEC.ConnectorToolSurfaceMcp` <a id="spec-cynai-connec-conntoolmcp"></a>

Traces To:

- [REQ-CONNEC-0111](../requirements/connec.md#req-connec-0111)
- [REQ-CONNEC-0112](../requirements/connec.md#req-connec-0112)
- [REQ-CONNEC-0113](../requirements/connec.md#req-connec-0113)

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

### User API Gateway Surface Applicable Requirements

- Spec ID: `CYNAI.CONNEC.ConnectorUserApiGateway` <a id="spec-cynai-connec-connusergwy"></a>

Traces To:

- [REQ-CONNEC-0114](../requirements/connec.md#req-connec-0114)
- [REQ-CONNEC-0115](../requirements/connec.md#req-connec-0115)
- [REQ-CONNEC-0116](../requirements/connec.md#req-connec-0116)

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

### OpenClaw Connector Compatibility Applicable Requirements

- Spec ID: `CYNAI.CONNEC.OpenClawCompatibility` <a id="spec-cynai-connec-openclaw"></a>

Traces To:

- [REQ-CONNEC-0117](../requirements/connec.md#req-connec-0117)
- [REQ-CONNEC-0118](../requirements/connec.md#req-connec-0118)
- [REQ-CONNEC-0119](../requirements/connec.md#req-connec-0119)
- [REQ-CONNEC-0120](../requirements/connec.md#req-connec-0120)
- [REQ-CONNEC-0121](../requirements/connec.md#req-connec-0121)

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
