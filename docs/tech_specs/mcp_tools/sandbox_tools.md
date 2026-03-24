# Sandbox MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contracts](#tool-contracts)
  - [`sandbox.create` Operation](#sandboxcreate-operation)
  - [`sandbox.exec` Operation](#sandboxexec-operation)
  - [`sandbox.put_file` Operation](#sandboxput_file-operation)
  - [`sandbox.get_file` Operation](#sandboxget_file-operation)
  - [`sandbox.stream_logs` Operation](#sandboxstream_logs-operation)
  - [`sandbox.destroy` Operation](#sandboxdestroy-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

Sandbox tools allow agents to create, run commands in, exchange files with, stream logs from, and destroy sandbox containers for a job.
The final job spec delivered to the SBA MUST contain the Agent persona **inline only** (`persona: { title, description }`); when `persona_id` is supplied, the orchestrator or PM resolves it and embeds the persona inline.

Related documents

- [Worker Node](../worker_node.md)
- [Sandbox Container](../sandbox_container.md)

## Sandbox Local Tools vs MCP Sandbox Tools

- Spec ID: `CYNAI.MCPTOO.SandboxLocalVsMcp` <a id="spec-cynai-mcptoo-sandboxlocalvsmcp"></a>

- **Local (in-sandbox) tools** are capabilities exposed **inside** the sandbox runtime to the agent process (e.g. editing files on the container filesystem, running a local shell).
  They are **not** orchestrator MCP `tools/call` operations and are not routed through the MCP gateway JSON API.
  They **do not** carry `task_id` or `job_id` as MCP arguments-the worker already bound execution to a job before the container started.
- **MCP sandbox tools** (this document: `sandbox.create`, `sandbox.exec`, `sandbox.put_file`, `sandbox.get_file`, `sandbox.stream_logs`, `sandbox.destroy`) are invoked via **POST `/v1/mcp/tools/call`** on the control plane.
  They **require** `task_id` and `job_id` so the gateway can validate allowlists, map to the correct job/sandbox, and write audit records.
- Agents and specs MUST NOT conflate "edit a file in the sandbox" (local) with "call `sandbox.put_file` over MCP" (gateway); the latter is for orchestrator-mediated control and policy, not for ordinary in-container editing.

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).

## Tool Contracts

- Spec ID: `CYNAI.MCPTOO.SandboxTools` <a id="spec-cynai-mcptoo-sandboxtools"></a>

### `sandbox.create` Operation

- **Inputs**: Required `task_id`, `job_id`, `image_ref` (OCI reference); optional `persona_id` (uuid) or `persona` (object with `title`, `description`).
  Scope: typically `pm`.
- **Outputs**: Success with job/sandbox ref or error.
- **Behavior**: Gateway validates task/job and image_ref (against allowed list when enforced), resolves persona_id to inline persona if provided, builds job spec with persona inline only, and requests sandbox creation from the worker/orchestrator.
  See [sandbox.create Algorithm](#algo-cynai-mcptoo-sandboxcreate).

#### `sandbox.create` Algorithm

<a id="algo-cynai-mcptoo-sandboxcreate"></a>

1. Resolve caller; check allowlist and scope (pm).
2. Validate `task_id`, `job_id`, `image_ref`; if PM agent, optionally validate image_ref against allowed images list.
3. If `persona_id` present: call persona.get (or equivalent) to resolve Agent persona; embed `persona: { title, description }` in job spec.
   If `persona` object provided, use it inline.
   Job spec MUST contain persona inline only, not persona_id.
4. Build job spec (task_id, job_id, image_ref, persona, and any other required fields); send create request to worker/orchestrator.
5. Emit audit record; return success with sandbox/job ref or error.

#### `sandbox.create` Error Conditions

- Invalid args; image_ref not allowed (when gated); persona_id not found; task/job not found or not accessible; worker unavailable; backend failure.

### `sandbox.exec` Operation

- **Inputs**: Required `task_id`, `job_id`, `command` (string), `argv` (array of strings).
  Scope: pm or both/sandbox.
- **Outputs**: Exit code, stdout/stderr (size-limited), or error.
- **Behavior**: Gateway validates task/job and that sandbox exists for job, forwards exec request to worker, returns exit code and output subject to policy.
  See [sandbox.exec Algorithm](#algo-cynai-mcptoo-sandboxexec).

#### `sandbox.exec` Algorithm

<a id="algo-cynai-mcptoo-sandboxexec"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `task_id`, `job_id`, `command`, `argv`; enforce command/argv size limits.
3. Resolve job and sandbox for job; reject if no sandbox exists.
4. Send exec (command, argv) to worker for that sandbox; wait for completion or timeout.
5. Apply output size limit to stdout/stderr; return exit code and truncated output if needed.
6. Emit audit record; return result.

#### `sandbox.exec` Error Conditions

- Invalid args; sandbox not found for job; timeout; command rejected by policy; output over limit; worker unavailable.

### `sandbox.put_file` Operation

- **Inputs**: Required `task_id`, `job_id`, `path` (string), `content_bytes_base64` (string).
  Scope: pm or both/sandbox.
- **Outputs**: Success confirmation or error.
- **Behavior**: Gateway validates task/job and path, decodes content, enforces size limit, sends write to worker for sandbox.
  See [sandbox.put_file Algorithm](#algo-cynai-mcptoo-sandboxputfile).

#### `sandbox.put_file` Algorithm

<a id="algo-cynai-mcptoo-sandboxputfile"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `task_id`, `job_id`, `path` (safe path); decode base64 content; enforce max file size.
3. Resolve sandbox for job; reject if none.
4. Send put_file (path, content) to worker; worker writes inside sandbox.
5. Emit audit record; return success or error.

#### `sandbox.put_file` Error Conditions

- Invalid args; path traversal or invalid path; content size exceeded; sandbox not found; worker error.

### `sandbox.get_file` Operation

- **Inputs**: Required `task_id`, `job_id`, `path`.
  Scope: pm or both/sandbox.
- **Outputs**: File content (e.g. base64); size-limited.
- **Behavior**: Gateway validates task/job/path, requests file from worker sandbox, returns size-limited content.
  See [sandbox.get_file Algorithm](#algo-cynai-mcptoo-sandboxgetfile).

#### `sandbox.get_file` Algorithm

<a id="algo-cynai-mcptoo-sandboxgetfile"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `task_id`, `job_id`, `path` (safe path).
3. Resolve sandbox for job; request file read from worker.
4. Apply response size limit; return content or truncate/error per policy.
5. Emit audit record; return result.

#### `sandbox.get_file` Error Conditions

- Invalid args; path not in sandbox or not found; size over limit; sandbox not found; worker error.

### `sandbox.stream_logs` Operation

- **Inputs**: Required `task_id`, `job_id`.
  Scope: pm or both/sandbox.
- **Outputs**: Log lines or stream handle; size-limited when returned in one response.
- **Behavior**: Gateway validates task/job, requests logs from worker for sandbox, returns or streams log content.
  See [sandbox.stream_logs Algorithm](#algo-cynai-mcptoo-sandboxstreamlogs).

#### `sandbox.stream_logs` Algorithm

<a id="algo-cynai-mcptoo-sandboxstreamlogs"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `task_id`, `job_id`.
3. Resolve sandbox for job; request logs from worker (stream or bounded fetch).
4. If single response: apply size limit and return log lines; if stream: establish stream and return handle or first chunk.
5. Emit audit record; return result.

#### `sandbox.stream_logs` Error Conditions

- Invalid args; sandbox not found; worker unavailable; stream failure.

### `sandbox.destroy` Operation

- **Inputs**: Required `task_id`, `job_id`.
  Scope: typically `pm`.
- **Outputs**: Success confirmation or error.
- **Behavior**: Gateway validates task/job, sends destroy request to worker for sandbox, then returns.
  See [sandbox.destroy Algorithm](#algo-cynai-mcptoo-sandboxdestroy).

#### `sandbox.destroy` Algorithm

<a id="algo-cynai-mcptoo-sandboxdestroy"></a>

1. Resolve caller; check allowlist and scope (pm).
2. Validate `task_id`, `job_id`.
3. Resolve sandbox for job; send destroy request to worker; worker tears down container.
4. Emit audit record; return success or error.

#### `sandbox.destroy` Error Conditions

- Invalid args; sandbox not found; worker error; timeout.

#### Traces To

- [REQ-MCPTOO-0100](../../requirements/mcptoo.md#req-mcptoo-0100)
- [REQ-MCPTOO-0106](../../requirements/mcptoo.md#req-mcptoo-0106)

## Allowlist and Scope

- Spec ID: `CYNAI.MCPTOO.SandboxToolsAllowlist` <a id="spec-cynai-mcptoo-sandboxtoolsallowlist"></a>

- **Allowlist**: PMA (and PAA per catalog) for create/exec/destroy; Worker agent for exec/put_file/get_file/stream_logs when operating in sandbox context.
- **Scope**: Create/destroy typically `pm`; exec and file operations may be `both` or `sandbox` per [Per-tool scope: Sandbox vs PM](access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).
