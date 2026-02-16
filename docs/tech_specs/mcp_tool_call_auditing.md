# MCP Tool Call Auditing

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Audit Point](#audit-point)
- [Required Audit Fields](#required-audit-fields)
- [Storage in PostgreSQL](#storage-in-postgresql)
- [Retention](#retention)

## Document Overview

This document defines how MCP tool calls are audited in CyNodeAI.
Tool call auditing is performed centrally by the orchestrator MCP gateway.

Related documents

- MCP gateway enforcement: [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)
- Access control: [`docs/tech_specs/access_control.md`](access_control.md)
- Postgres schema: [`docs/tech_specs/postgres_schema.md`](postgres_schema.md)

## Goals

- Ensure every routed tool call is attributable and auditable.
- Avoid requiring any MCP wire extensions.
- Store only metadata in PostgreSQL for MVP.

## Audit Point

Normative requirements

- The orchestrator MCP gateway MUST emit an audit record for every tool call it routes.
- Audit records MUST be written regardless of allow or deny decisions.
- Audit records MUST be written regardless of tool call success or failure.

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

- For MVP, tool argument payloads and tool result payloads MUST NOT be stored in PostgreSQL audit tables.
- Tool payloads MAY be stored in structured logs when needed for debugging, subject to redaction.

## Storage in PostgreSQL

The canonical table definition is in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md#mcp-tool-call-audit-log-table).

## Retention

Retention SHOULD be configurable by operators.
Retention MAY be implemented as time-based deletion or archiving.
