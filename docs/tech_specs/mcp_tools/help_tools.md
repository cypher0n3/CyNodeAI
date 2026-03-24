# Help MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contracts](#tool-contracts)
  - [`help.get` Operation](#helpget-operation)
  - [`help.list` Operation](#helplist-operation)
- [Specification Help](#specification-help)
  - [`specification.help` Operation](#specificationhelp-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

On-demand documentation for how to interact with CyNodeAI.
Help content MUST be size-limited and MUST NOT include secrets.
Help returned for tools MUST align with [MCP Tooling - Extraneous arguments](../mcp/mcp_tooling.md#spec-cynai-mcptoo-extraneousarguments) (unknown keys are ignored) and per-tool notes (e.g. [Skills MCP Tools](skills_tools.md) and **`project_id`** for project-scoped skills).
For MVP, the gateway may serve `help.get` and `help.list` from embedded strings or an in-process map only (see [Help MCP Server](../mcp/mcp_tooling.md#spec-cynai-mcptoo-helpmcpserver)); richer resolution from live tool-definition catalogs is a later refinement.

Related documents

- [MCP Tooling - Help MCP Server](../mcp/mcp_tooling.md#spec-cynai-mcptoo-helpmcpserver)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).
The Help MCP contract defines how the gateway returns tool and invocation help to the caller (tool and invocation Help fields, size limits, no secrets).

## Tool Contracts

- Spec ID: `CYNAI.MCPTOO.HelpTools` <a id="spec-cynai-mcptoo-helptools"></a>

### `help.get` Operation

- **Inputs**: Optional `topic` (e.g. tool name), `path` (logical path).
  Help tools do **not** take `task_id`; audit correlation uses gateway request context (caller identity, tool name, timestamp), not a task id.
  Scope: `both`.
- **Outputs**: Documentation content (markdown or text); size-limited; MUST NOT include secrets.
- **Behavior**: Gateway resolves caller's allowed tools, returns overview or topic-specific help (tool Help and per-invocation Help) from tool definitions.
  See [help.get Algorithm](#algo-cynai-mcptoo-helpget).

#### `help.get` Algorithm

<a id="algo-cynai-mcptoo-helpget"></a>

1. Resolve caller identity and agent type; reject if unauthenticated.
2. Check allowlist; caller may call help.get (typically both PM and sandbox).
3. If `topic` omitted: return default overview (how to use help, list of allowed tools with short descriptions from each tool's Help field); ensure list is scoped to caller's allowed tools.
   The tool list MUST match the list returned by [`help.list`](#helplist-operation) for the same caller (same names and short descriptions).
4. If `topic` present: resolve topic to a tool name; if tool not in caller's allowed set, return not-found or restricted message.
   Otherwise return that tool's Help and each invocation's Help (in order), plus effective scope per invocation; content from tool definitions only.
5. Enforce response size limit; strip any secrets.
   Emit audit record; return result.

#### `help.get` Error Conditions

- Topic requested not in caller's allowed set; size over limit.

#### Traces to (`help.get`)

- [REQ-MCPTOO-0116](../../requirements/mcptoo.md#req-mcptoo-0116)
- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)

### `help.list` Operation

- **Inputs**: None required.
  Help tools do **not** take `task_id`.
  Scope: `both`.
- **Outputs**: The list of available help topics for this caller: each allowed tool name with a short description (from that tool's `Help` field); size-limited; MUST NOT include secrets.
- **Behavior**: Returns the same topic list as `help.get` when `topic` is omitted (the **Available tools** portion: names plus short descriptions, scoped to the caller's allowed tools).
  It MUST NOT be required to include the separate introductory "how to use help" narrative that `help.get` may include in its default overview.
  See [help.list Algorithm](#algo-cynai-mcptoo-helplist).

#### `help.list` Algorithm

<a id="algo-cynai-mcptoo-helplist"></a>

1. Resolve caller identity and agent type; reject if unauthenticated.
2. Check allowlist; caller may call `help.list` (typically both PM and sandbox).
3. Build the topic list: allowed tools for this caller, each with short description from the tool's `Help` field (same rows as step 3 of [`help.get`](#algo-cynai-mcptoo-helpget) when `topic` is omitted, without the overview prose).
4. Enforce response size limit; strip any secrets.
   Emit audit record; return result.

#### `help.list` Error Conditions

- Size over limit.

#### Traces to (`help.list`)

- [REQ-MCPTOO-0116](../../requirements/mcptoo.md#req-mcptoo-0116)
- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)
- [REQ-MCPTOO-0123](../../requirements/mcptoo.md#req-mcptoo-0123)

## Specification Help

- Spec ID: `CYNAI.MCPTOO.SpecificationHelp` <a id="spec-cynai-mcptoo-specificationhelp"></a>

### `specification.help` Operation

- **Inputs**: Optional `topic` or `path` (e.g. spec_type).
  Scope: `pm`.
- **Outputs**: Schema guidance (required/optional fields, spec_type values, examples); size-limited; read-only.
- **Behavior**: Gateway checks allowlist (PMA; PAA/SBA read-only when exposed); derives response from actual schema (orchestrator or API).
  See [specification.help Algorithm](#algo-cynai-mcptoo-specificationhelp).

#### `specification.help` Algorithm

<a id="algo-cynai-mcptoo-specificationhelp"></a>

1. Resolve caller; verify PM (or PAA/SBA if read-only allowed); deny if not on allowlist.
2. Load or derive schema for specification payloads from orchestrator/API (single source of truth).
3. Build response: required/optional fields, `spec_type` values, examples; apply optional topic/path filter.
4. Enforce response size limit; return content (no secrets).
5. Emit audit record; return result.

#### Traces to (`specification.help`)

- [REQ-MCPTOO-0116](../../requirements/mcptoo.md#req-mcptoo-0116)

## Allowlist and Scope

- **Allowlist**: Help tools are allowlisted for orchestrator-side agents (PMA, PAA) and MAY be allowlisted for Worker agents per [MCP Tooling - Help MCP Server](../mcp/mcp_tooling.md#spec-cynai-mcptoo-helpmcpserver).
- **Scope**: `help.get` and `help.list` typically `both`; `specification.help` typically `pm`.
