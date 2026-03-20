# MCP Tool Call Auditing

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Audit Point](#audit-point)
- [Required Audit Fields](#required-audit-fields)
- [Postgres Schema](#postgres-schema)
  - [MCP Tool Call Audit Log Table](#mcp-tool-call-audit-log-table)
- [Retention](#retention)

## Document Overview

This document defines how MCP tool calls are audited in CyNodeAI.
Tool call auditing is performed centrally by the orchestrator MCP gateway for routed calls.
When edge enforcement mode is used for node-local agent runtimes, tool call auditing is additionally performed on the node and audit records must be made available to the orchestrator.

Related documents

- MCP gateway enforcement: [`docs/tech_specs/mcp/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)
- Access control: [`docs/tech_specs/access_control.md`](../access_control.md)
- Postgres schema: [`docs/tech_specs/postgres_schema.md`](../postgres_schema.md)
- [Audit logging overview](../audit_logging.md)

## Goals

- Ensure every routed tool call is attributable and auditable.
- Avoid requiring any MCP wire extensions.
- Store only metadata in PostgreSQL for MVP.

## Audit Point

This section identifies the point in the system where tool call auditing is performed.

### Applicable Requirements

- Spec ID: `CYNAI.MCPGAT.ToolCallAuditPoint` <a id="spec-cynai-mcpgat-toolaudit"></a>

#### Traces to Requirements

- [REQ-MCPGAT-0107](../../requirements/mcpgat.md#req-mcpgat-0107)
- [REQ-MCPGAT-0108](../../requirements/mcpgat.md#req-mcpgat-0108)
- [REQ-MCPGAT-0109](../../requirements/mcpgat.md#req-mcpgat-0109)

### Edge Tool Call Auditing

- Spec ID: `CYNAI.MCPGAT.EdgeToolCallAuditing` <a id="spec-cynai-mcpgat-edgetoolaudit"></a>

#### Edge Tool Call Auditing Requirements Traces

- [REQ-MCPGAT-0112](../../requirements/mcpgat.md#req-mcpgat-0112)

In edge enforcement mode, tool calls may be invoked directly against a node-local MCP server by a node-local agent runtime.
In that case:

- The node-local MCP server (or an edge enforcement proxy) MUST emit an audit record for every tool call (allow or deny, success or failure).
- The node MUST persist audit records locally with bounded retention.
- The node MUST make audit records available to the orchestrator for centralized retention and inspection.
  The exact ingestion mechanism is implementation-defined and may be pull-based (orchestrator queries node telemetry) or push-based (node submits batches).

## Required Audit Fields

Recommended minimum fields

- `created_at`
- `task_id` (nullable)
- `project_id` (nullable)
- `run_id` (nullable)
- `job_id` (nullable)
- `subject_type` and `subject_id` (nullable)
- `user_id` (nullable)
- `group_ids` (nullable array)
- `role_names` (nullable array)
- `tool_name`
- `decision` (allow or deny)
- `status` (success or error)
- `duration_ms` (nullable)
- `error_type` (nullable)

Payload storage

- For MVP, tool argument payloads and tool result payloads are not stored in PostgreSQL audit tables.
- Tool payloads can be stored in structured logs when needed for debugging, subject to redaction.

## Postgres Schema

Canonical column definitions for the MCP tool call audit log live in this section.

### MCP Tool Call Audit Log Table

- Spec ID: `CYNAI.SCHEMA.McpToolCallAuditLog` <a id="spec-cynai-schema-mcptoolcallauditlog"></a>

This table stores append-only metadata for MCP tool calls routed by the orchestrator gateway.
Tool arguments and tool results are not stored in this table for MVP.

Index entry in [`postgres_schema.md`](../postgres_schema.md#spec-cynai-schema-auditlogging).

- `id` (uuid, pk)
- `created_at` (timestamptz)
- `task_id` (uuid, fk to `tasks.id`, nullable)
- `project_id` (uuid, fk to `projects.id`, nullable)
- `run_id` (uuid, fk to `runs.id`, nullable)
- `job_id` (uuid, fk to `jobs.id`, nullable)
- `subject_type` (text, nullable)
- `subject_id` (uuid, nullable)
- `user_id` (uuid, fk to `users.id`, nullable)
- `group_ids` (jsonb, nullable)
  - array of uuid
- `role_names` (jsonb, nullable)
  - array of string
- `tool_name` (text)
- `decision` (text)
  - allow or deny
- `status` (text)
  - success or error
- `duration_ms` (int, nullable)
- `error_type` (text, nullable)

#### MCP Tool Call Audit Log Table Constraints

- Index: (`created_at`)
- Index: (`task_id`)
- Index: (`project_id`)
- Index: (`tool_name`)

## Retention

Retention SHOULD be configurable by operators.
Retention MAY be implemented as time-based deletion or archiving.
