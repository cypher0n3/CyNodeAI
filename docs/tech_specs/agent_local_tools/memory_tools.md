# Memory (Job-Scoped) Agent-Local Tools

- [Document Overview](#document-overview)
  - [Related Documents](#related-documents)
- [Storage Contract](#storage-contract)
- [Definition Compliance](#definition-compliance)
- [Implementation Placement](#implementation-placement)
- [Tool Contracts](#tool-contracts)
  - [`memory.add` Operation](#memoryadd-operation)
  - [`memory.list` Operation](#memorylist-operation)
  - [`memory.retrieve` Operation](#memoryretrieve-operation)
  - [`memory.delete` Operation](#memorydelete-operation)
- [Job Completion Snapshot](#job-completion-snapshot)
- [Allowlist and Exposure](#allowlist-and-exposure)

## Document Overview

The **`memory.*`** tools (`memory.add`, `memory.list`, `memory.retrieve`, `memory.delete`) are **agent-only** capabilities for the **Sandbox Agent (SBA)**.

They are **not** implemented by the **orchestrator MCP gateway** and MUST NOT use **outbound network** calls for storage or retrieval.
The SBA (e.g. `cynode-sba`) implements them **inside the sandbox process** against **on-disk** state under the job workspace, so the agent can persist working notes and structured state **locally** across steps and LLM calls.

Memories are **job-scoped** and MUST NOT persist beyond the job lifetime unless explicitly promoted to artifacts or other durable mechanisms outside this spec.

Size limits, maximum entries per job, and retention (TTL = job lifetime) are enforced **locally** by the agent runtime or container policy, not by the orchestrator.

### Related Documents

- [cynode_sba.md - Temporary memory](../cynode_sba.md#spec-cynai-sbagnt-temporarymemory)
- [MCP Tooling - Common argument requirements](../mcp/mcp_tooling.md#spec-cynai-mcptoo-commonargumentrequirements) (contrast **memory.*** with gateway-backed tools in the same section)
- [Worker API - Run Job response](../worker_api.md) (Node Manager includes `memory_snapshot` on job completion)

## Storage Contract

- Spec ID: `CYNAI.MCPTOO.MemoryStorageContract` <a id="spec-cynai-mcptoo-memorystoragecontract"></a>

### Storage Location

- **Canonical path**: `/job/memory.json` inside the sandbox container.
  The Node Manager bind-mounts `/job` to a host path per [Sandbox workspace and job mounts](../worker_node.md#spec-cynai-worker-sandboxworkspacejobmounts); the SBA MUST write the memory store to this path so the Node Manager can read it on job completion.
- The SBA MUST create `/job/memory.json` on first write; the file MUST exist only under `/job/` (no sibling directories; single file).

### File Format

- **Format**: JSON.
- **Encoding**: UTF-8.
- **Schema**: A JSON object with a top-level key **`entries`** mapping memory keys to entry objects:

```json
{
  "job_id": "uuid",
  "entries": {
    "key1": {
      "content": "string",
      "summary": "optional string",
      "task_id": "optional uuid",
      "updated_at": "RFC 3339 string"
    }
  }
}
```

- **`job_id`**: Must match the job; implementations MAY omit if redundant with job context.
- **`entries`**: Object keyed by memory key (string); each value is an entry object.
- **Entry fields**:
  - `content` (string, required): The stored content; size-limited per [Size and Entry Limits](#size-and-entry-limits).
  - `summary` (string, optional): Short summary for list views.
  - `task_id` (uuid string, optional): Correlation with orchestrator task.
  - `updated_at` (string, required): RFC 3339 timestamp of last write.

### Write Mechanism

- **Single file**: All entries for the job live in one JSON file.
- **Atomic writes**: The SBA SHOULD write to a temp file and rename to `/job/memory.json` to avoid partial writes on crash.
- **Key semantics**: `memory.add` with an existing key **overwrites** the entry; `memory.delete` removes the key from `entries`.

### Size and Entry Limits

Implementations MUST enforce (configurable with sensible defaults):

- **Max entries per job**: e.g. 10,000; reject `memory.add` when at limit.
- **Max content size per entry**: e.g. 512 MiB; reject `memory.add` when exceeded (must not exceed the max on-disk file size below).
- **Max on-disk file size / max total size per job**: e.g. 512 MiB for `/job/memory.json`; reject `memory.add` when adding would exceed.
- **Key length**: Non-empty string; max length e.g. 512 characters.

## Definition Compliance

Tool definitions exposed to the model SHOULD follow the project's MCP tool definition shape (`Server`, `Name`, `Help`, `Scope`, `Tools`) for catalog consistency, even though **`memory.*` are not routed through the orchestrator MCP gateway**.

## Implementation Placement

Normative placement:

- **SBA process** inside the sandbox container: read/write the on-disk memory store; enforce limits; return schema-valid, size-limited responses.
- **Not** the orchestrator service, **not** a remote MCP server, and **not** a path that implies network I/O for memory operations.

## Tool Contracts

- Spec ID: `CYNAI.MCPTOO.MemoryToolsJobScoped` <a id="spec-cynai-mcptoo-memorytoolsjobscoped"></a>

### `memory.add` Operation

- **Inputs**: Required `job_id`, `key` (string), `content` (string; size-limited); optional `task_id` (uuid; correlation or namespace), `summary` (string).
  Scope: **sandbox agent local** (SBA only); not PM/orchestrator gateway.
- **Outputs**: Success confirmation or error; MUST NOT include secrets.
- **Behavior**: Validate arguments and local limits (max content size, max entries per job); persist under the job-scoped on-disk store; no network.
  See [memory.add Algorithm](#algo-cynai-mcptoo-memoryadd).

#### `memory.add` Algorithm

<a id="algo-cynai-mcptoo-memoryadd"></a>

1. Resolve `job_id` (and optional `task_id`) from arguments and/or injected job context; reject if invalid.
2. Validate `key` (non-empty), `content`, and optional fields; enforce max content size and entry cap for this job.
3. Write or merge the entry to `/job/memory.json` per [Storage Contract](#storage-contract) (JSON format; atomic write recommended).
4. Return success or error.

#### `memory.add` Error Conditions

- Invalid or missing args; content over size limit; max entries per job exceeded; local I/O failure.

### `memory.list` Operation

- **Inputs**: Required `job_id`; optional `task_id` (filter/correlation), `limit`, `cursor`.
  Scope: **sandbox agent local** (SBA only).
- **Outputs**: List of keys (and optional summaries) for the job; paginated; size-limited.
- **Behavior**: Read the on-disk store; apply optional filters and pagination; no network.
  See [memory.list Algorithm](#algo-cynai-mcptoo-memorylist).

#### `memory.list` Algorithm

<a id="algo-cynai-mcptoo-memorylist"></a>

1. Validate `job_id`; optional `task_id` if used as filter.
2. Load keys (and summaries) from `/job/memory.json` per [Storage Contract](#storage-contract); apply limit and cursor.
3. Enforce response size limit; return page and next cursor if applicable.

#### `memory.list` Error Conditions

- Invalid args; local I/O failure.

### `memory.retrieve` Operation

- **Inputs**: Required `job_id`, `key`; optional `task_id` (correlation).
  Scope: **sandbox agent local** (SBA only).
- **Outputs**: Stored content for key; size-limited.
- **Behavior**: Look up (`job_id`, key) in the on-disk store; no network.
  See [memory.retrieve Algorithm](#algo-cynai-mcptoo-memoryretrieve).

#### `memory.retrieve` Algorithm

<a id="algo-cynai-mcptoo-memoryretrieve"></a>

1. Validate `job_id`, `key`; optional `task_id` if present.
2. Read content from `/job/memory.json` per [Storage Contract](#storage-contract); return not-found if key missing.
3. Apply response size limit; return content.

#### `memory.retrieve` Error Conditions

- Invalid args; key not found; local I/O failure.

### `memory.delete` Operation

- **Inputs**: Required `job_id`, `key`; optional `task_id` (correlation).
  Scope: **sandbox agent local** (SBA only).
- **Outputs**: Success confirmation or error.
- **Behavior**: Remove entry for (`job_id`, key) in the on-disk store; no network.
  See [memory.delete Algorithm](#algo-cynai-mcptoo-memorydelete).

#### `memory.delete` Algorithm

<a id="algo-cynai-mcptoo-memorydelete"></a>

1. Validate `job_id`, `key`; optional `task_id` if present.
2. Delete entry from `/job/memory.json` per [Storage Contract](#storage-contract) (idempotent: no-op if key already missing).
3. Return success.

#### `memory.delete` Error Conditions

- Invalid args; local I/O failure.

#### Traces To

- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)
- [REQ-MCPTOO-0110](../../requirements/mcptoo.md#req-mcptoo-0110)

## Job Completion Snapshot

- Spec ID: `CYNAI.MCPTOO.MemoryJobCompletionSnapshot` <a id="spec-cynai-mcptoo-memoryjobcompletionsnapshot"></a>

When a job reaches a **terminal state** (completed, failed, timeout, or stopped), the **Node Manager** MUST retrieve the memory blob from the sandbox so it can be analyzed later.

### Node Manager Responsibility

- After the container has exited (or been stopped), the Node Manager MUST read **`/job/memory.json`** from the host bind-mount (same path used for `/job/result.json` and `/job/artifacts/`).
- If the file exists and is valid JSON per [Storage Contract](#storage-contract), the Node Manager MUST include it in the Run Job response (or equivalent job completion payload) as **`memory_snapshot`** so the orchestrator can persist it.
- If the file is missing, invalid, or unreadable (e.g. crash before any memory write), the Node Manager MAY omit `memory_snapshot` or include `null`; the job completion MUST NOT fail solely because the memory file is absent.
- The `memory_snapshot` value MUST be the raw JSON object (entries and metadata) as stored; the orchestrator persists it (e.g. in `jobs.result` or a dedicated field) for post-hoc analysis of agent memory state.

### Use of Memory Snapshot

- Operators, PMA, or analysis tooling MAY inspect `memory_snapshot` to understand what the SBA stored during the job (e.g. debugging, auditing, replay).
- The snapshot is **read-only** after persistence; it is not re-ingested into a subsequent job.

### Worker API Contract

The Run Job response body MUST support an optional **`memory_snapshot`** field (object or null) when the job used an SBA runner image.
See [Worker API - Run Job response](../worker_api.md) for the response schema.

## Allowlist and Exposure

- Spec ID: `CYNAI.MCPTOO.MemoryToolsAllowlist` <a id="spec-cynai-mcptoo-memorytoolsallowlist"></a>

- **Catalog**: The [Worker Agent allowlist](../mcp_tools/access_allowlists_and_scope.md#spec-cynai-mcpgat-workeragentallowlist) SHOULD list **`memory.*`** so SBAs are allowed to **expose** these tool names to the model.
  That listing means the SBA runtime may register **`memory.add`**, **`memory.list`**, **`memory.retrieve`**, and **`memory.delete`** for the agent; it does **not** imply orchestrator MCP gateway routing for those calls.
- **Enforcement**: Limits (max entries, max size per entry, job lifetime) MUST be enforced **inside the SBA (or sandbox) implementation** on local disk, not by the orchestrator gateway.
