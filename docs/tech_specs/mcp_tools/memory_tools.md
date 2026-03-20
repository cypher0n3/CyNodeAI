# Memory (Job-Scoped) MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contracts](#tool-contracts)
  - [`memory.add` Operation](#memoryadd-operation)
  - [`memory.list` Operation](#memorylist-operation)
  - [`memory.retrieve` Operation](#memoryretrieve-operation)
  - [`memory.delete` Operation](#memorydelete-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

Job-scoped temporary memory allows sandbox agents (e.g. SBA) to persist working state across steps and LLM calls during a job.
Memories are scoped to `task_id` and `job_id` and MUST NOT persist beyond the job (or task) unless explicitly promoted.
Size limits and TTL (e.g. job lifetime) are enforced by the gateway.

Related documents

- [cynode_sba.md - Temporary memory](../cynode_sba.md#spec-cynai-sbagnt-temporarymemory)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Tool Contracts

- Spec ID: `CYNAI.MCPTOO.MemoryToolsJobScoped` <a id="spec-cynai-mcptoo-memorytoolsjobscoped"></a>

### `memory.add` Operation

- **Inputs**: Required `task_id`, `job_id`, `key` (string), `content` (string; size-limited); optional `summary` (string).
  Scope: sandbox or both.
- **Outputs**: Success confirmation or error; MUST NOT include secrets.
- **Behavior**: Gateway enforces allowlist, scope, and job-scoped limits (max entries, max size per entry); stores content under key for (task_id, job_id).
  See [memory.add Algorithm](#algo-cynai-mcptoo-memoryadd).

#### `memory.add` Algorithm

<a id="algo-cynai-mcptoo-memoryadd"></a>

1. Resolve caller identity and agent type; reject if unauthenticated.
2. Check tool allowlist and scope (sandbox or both); deny if not allowed.
3. Validate `task_id`, `job_id` (uuid), `key` (non-empty), `content`; enforce max content size per job.
4. Check job-scoped limit: total memory entries for this job MUST NOT exceed configured max; reject if at limit.
5. Persist (task_id, job_id, key, content, optional summary) in job-scoped store; TTL = job lifetime unless promoted.
6. Emit audit record.
7. Return success or error.

#### `memory.add` Error Conditions

- Invalid or missing args; content over size limit; max entries per job exceeded; access denied; backend failure.

### `memory.list` Operation

- **Inputs**: Required `task_id`, `job_id`; optional `limit`, `cursor`.
  Scope: sandbox or both.
- **Outputs**: List of keys (and optional summaries) for the job; paginated; size-limited.
- **Behavior**: Gateway enforces allowlist and scope, reads job-scoped memory keys (and summaries) with optional limit/cursor.
  See [memory.list Algorithm](#algo-cynai-mcptoo-memorylist).

#### `memory.list` Algorithm

<a id="algo-cynai-mcptoo-memorylist"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `task_id`, `job_id`.
3. Query job-scoped memory store for keys (and summaries); apply limit and cursor.
4. Enforce response size limit; return page and next cursor if applicable.
5. Emit audit record; return result.

#### `memory.list` Error Conditions

- Invalid args; access denied; backend failure.

### `memory.retrieve` Operation

- **Inputs**: Required `task_id`, `job_id`, `key`.
  Scope: sandbox or both.
- **Outputs**: Stored content for key; size-limited.
- **Behavior**: Gateway enforces allowlist and scope, retrieves content for (task_id, job_id, key).
  See [memory.retrieve Algorithm](#algo-cynai-mcptoo-memoryretrieve).

#### `memory.retrieve` Algorithm

<a id="algo-cynai-mcptoo-memoryretrieve"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `task_id`, `job_id`, `key`.
3. Look up content in job-scoped store; return not-found if key missing.
4. Apply response size limit; return content.
5. Emit audit record; return result.

#### `memory.retrieve` Error Conditions

- Invalid args; key not found; access denied; backend failure.

### `memory.delete` Operation

- **Inputs**: Required `task_id`, `job_id`, `key`.
  Scope: sandbox or both.
- **Outputs**: Success confirmation or error.
- **Behavior**: Gateway enforces allowlist and scope, removes entry for (task_id, job_id, key).
  See [memory.delete Algorithm](#algo-cynai-mcptoo-memorydelete).

#### `memory.delete` Algorithm

<a id="algo-cynai-mcptoo-memorydelete"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `task_id`, `job_id`, `key`.
3. Delete entry from job-scoped store (idempotent: no-op if key already missing).
4. Emit audit record; return success.

#### `memory.delete` Error Conditions

- Invalid args; access denied; backend failure.

#### Traces To

- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)
- [REQ-MCPTOO-0110](../../requirements/mcptoo.md#req-mcptoo-0110)

## Allowlist and Scope

- Spec ID: `CYNAI.MCPTOO.MemoryToolsAllowlist` <a id="spec-cynai-mcptoo-memorytoolsallowlist"></a>

- **Allowlist**: These tools MUST be on the [Worker Agent allowlist](access_allowlists_and_scope.md#spec-cynai-mcpgat-workeragentallowlist) for sandbox-scoped use; scope is sandbox (or both).
- **Enforcement**: Gateway MUST enforce max entries and max size per entry per job.
