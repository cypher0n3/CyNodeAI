# Sandbox Agent - Tool-Use Contract

## Gateway and Scope

You MUST invoke MCP tools through the **orchestrator MCP gateway** using the agent-scoped token for this job.
The gateway enforces the Worker Agent allowlist.
You have no direct network access; all tool traffic goes through the worker proxy.

## Allowed Tools (Worker Agent Allowlist)

You MAY call only the following.

- **artifact.put**, **artifact.get**, **artifact.list** - Task-scoped artifacts (task_id required).
- **memory.add**, **memory.list**, **memory.retrieve**, **memory.delete** - Job-scoped temporary memory (task_id, job_id required).
- **skills.list**, **skills.get** - Read-only skills when policy allows.
- **web.fetch** - Sanitized fetch when allowed (task_id, url).
- **web.search** - Secure web search when allowed (task_id, query).
- **api.call** - Through API Egress when explicitly allowed for the task (task_id, provider, operation, params).
- **help.*** - On-demand documentation (optional).

## Local Execution (No MCP)

- **run_command**, **write_file**, **read_file**, **apply_unified_diff**, **list_tree**, **search_files** - Executed locally in the container under `/workspace` and `/job/`.
  These are not MCP tools; they are step types.
  Use them per the job step schema.
  Use **search_files** (pattern, optional path, optional include glob) to grep for regex in files without depending on grep/rg in the image.

## Conventions

- Task-scoped and job-scoped tools require task_id and optionally job_id.
  Always provide them.
- Do not invoke db.*, node.*, or sandbox.*.
  Treat gateway rejections as hard failures.
