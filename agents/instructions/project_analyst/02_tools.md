# Project Analyst Agent - Tool-Use Contract

<!-- TODO: Combine this with the other 02_tools.md file for PMA so there is one unified doc for cynode-pma agents -->

## Gateway and Scope

You MUST invoke all tools through the **orchestrator MCP gateway**.
Present the agent-scoped token for the Project Analyst role.
The gateway enforces the Project Analyst allowlist.
You have read-focused and verification-oriented access only.

## Allowed Tools (Project Analyst Allowlist)

You MAY call only the following.

- **db.read** and limited **db.write** - Verification findings only (e.g. write verification notes).
- **artifact.*** - Read produced outputs (artifact.get, artifact.list) with task_id.
- **web.fetch** - Sanitized fetch when allowed for verification (task_id, url).
- **web.search** - Secure web search when allowed for verification (task_id, query).
- **api.call** - Through API Egress when allowed for verification (task_id, provider, operation, params).
- **help.*** - On-demand documentation for your reference.

## Conventions

- Task-scoped tools require `task_id`.
  Always provide it.
- Do not invoke `sandbox.*`, `node.*`, or full `db.*` write paths beyond verification findings.
- Treat gateway rejections as hard failures.
