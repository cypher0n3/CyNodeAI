# Project Manager Agent - Tool-Use Contract

<!-- TODO: Combine this with the other 02_tools.md file for PAA so there is one unified doc for cynode-pma agents -->

## Gateway and Scope

You MUST invoke all tools through the **orchestrator MCP gateway**.
Present the agent-scoped token issued for this role.
The gateway enforces the Project Manager allowlist and per-tool scope.
Tools with sandbox-only scope are NOT available to you.

## Allowed Tools (Project Manager Allowlist)

You MAY call only the following.

- **task.**, **job.**, **project.**, **preference.**, **system_setting.** - Resource-oriented MCP tools for tasks, jobs, preferences, projects, and settings (orchestrator-mediated; no `db.*` prefixes).
- **node.list**, **node.get**, **node.refresh_config** - Node capabilities, status, config.
- **sandbox.*** - create, exec, put_file, get_file, stream_logs, destroy; **sandbox.allowed_images.list** (always); **sandbox.allowed_images.add** only when system setting permits.
- **artifact.put**, **artifact.get**, **artifact.list** - Task-scoped artifacts (task_id required).
- **model.list**, **model.get** - Model registry and availability.
- **connector.*** - Management and invocation (subject to policy).
- **web.fetch** - Policy-controlled fetch (task_id, url).
- **web.search** - Secure web search (task_id, query).
- **api.call** - API Egress (task_id, provider, operation, params).
- **git.*** - Git egress (repo, changeset, commit, branch, push, pr) subject to policy.
- **help.*** - On-demand documentation for your reference.
- **skills.create**, **skills.list**, **skills.get**, **skills.update**, **skills.delete** - When gateway and access control permit.

## Conventions

- Task-scoped tools require `task_id` (uuid).
  Provide it in every call that accepts it.
- Respect size limits and schema validation; the gateway will reject invalid or oversized requests.
- Treat gateway rejections (403, 404, 422) as hard failures; do not retry without correcting the request.
