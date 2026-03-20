# Audit Logging (PostgreSQL)

- [Document Overview](#document-overview)
- [Postgres Schema](#postgres-schema)
  - [Auth Audit Log Table](#auth-audit-log-table)
  - [MCP Tool Call Audit Log Table](#mcp-tool-call-audit-log-table)
  - [Chat Audit Log Table](#chat-audit-log-table)
- [Related Documents](#related-documents)

## Document Overview

Audit logging is implemented via domain-specific tables so that retention and query patterns can be tuned per domain.
Event tables use `timestamptz` for event time and include subject and decision or outcome where applicable.

## Postgres Schema

- Spec ID: `CYNAI.SCHEMA.AuditLogging` <a id="spec-cynai-schema-auditlogging"></a>

**Schema definitions (index):** See [Audit Logging](postgres_schema.md#spec-cynai-schema-auditlogging) in [`postgres_schema.md`](postgres_schema.md).

### Auth Audit Log Table

Table name: `auth_audit_log`.

**Schema definitions:** See [Auth Audit Log Table](local_user_accounts.md#spec-cynai-schema-authauditlogtable) in [`local_user_accounts.md`](local_user_accounts.md).

### MCP Tool Call Audit Log Table

Table name: `mcp_tool_call_audit_log`.

**Schema definitions:** See [Postgres Schema](mcp/mcp_tool_call_auditing.md#spec-cynai-schema-mcptoolcallauditlog) in [`mcp/mcp_tool_call_auditing.md`](mcp/mcp_tool_call_auditing.md).

### Chat Audit Log Table

Table name: `chat_audit_log`.

**Schema definitions:** See [Chat Audit Log Table](openai_compatible_chat_api.md#spec-cynai-schema-chatauditlogtable) in [`openai_compatible_chat_api.md`](openai_compatible_chat_api.md).

## Related Documents

- **Access control:** `access_control_audit_log` - [Access Control](access_control.md#spec-cynai-schema-accesscontrol)
- **Preferences:** `preference_audit_log` - [Preferences](user_preferences.md#spec-cynai-schema-preferences)
- **System settings:** `system_settings_audit_log` - [Orchestrator Bootstrap](orchestrator_bootstrap.md#spec-cynai-schema-systemsettings)

Additional domain-specific audit tables (for example connector operations, Git egress) may be added later; they typically include `task_id` (nullable), subject identity, action, decision or outcome, and `created_at`.
