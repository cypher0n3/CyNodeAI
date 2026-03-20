# Help MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contracts](#tool-contracts)
  - [`help.get` Operation](#helpget-operation)
- [Specification Help](#specification-help)
  - [`specification.help` Operation](#specificationhelp-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

On-demand documentation for how to interact with CyNodeAI.
Help content is derived from tool definitions (MCPTool and ToolInvocation Help fields) and MUST be size-limited and MUST NOT include secrets.

Related documents

- [MCP Tooling - Help MCP Server](../mcp_tooling.md#spec-cynai-mcptoo-helpmcpserver)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).
The Help MCP contract defines how the gateway returns tool and invocation help to the caller (tool and invocation Help fields, size limits, no secrets).

## Tool Contracts

- Spec ID: `CYNAI.MCPTOO.HelpTools` <a id="spec-cynai-mcptoo-helptools"></a>

### `help.get` Operation

- **Inputs**: Required `task_id` (uuid; context and auditing); optional `topic` (e.g. tool name), `path` (logical path).
  Scope: `both`.
- **Outputs**: Documentation content (markdown or text); size-limited; MUST NOT include secrets.
- **Behavior**: Gateway resolves caller's allowed tools, returns overview or topic-specific help (tool Help and per-invocation Help) from tool definitions.
  See [help.get Algorithm](#algo-cynai-mcptoo-helpget).

#### `help.get` Algorithm

<a id="algo-cynai-mcptoo-helpget"></a>

1. Resolve caller identity and agent type; reject if unauthenticated.
2. Check allowlist; caller may call help.get (typically both PM and sandbox).
3. Validate `task_id`; use for audit context.
4. If `topic` omitted: return default overview (how to use help, list of allowed tools with short descriptions from each tool's Help field); ensure list is scoped to caller's allowed tools.
5. If `topic` present: resolve topic to a tool name; if tool not in caller's allowed set, return not-found or restricted message.
   Otherwise return that tool's Help and each invocation's Help (in order), plus effective scope per invocation; content from tool definitions only.
6. Enforce response size limit; strip any secrets.
   Emit audit record; return result.

#### `help.get` Error Conditions

- Invalid task_id; topic requested not in caller's allowed set; size over limit.

#### Traces to (`help.get`)

- [REQ-MCPTOO-0116](../../requirements/mcptoo.md#req-mcptoo-0116)
- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)

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

- **Allowlist**: Help tools are allowlisted for orchestrator-side agents (PMA, PAA) and MAY be allowlisted for Worker agents per [MCP Tooling - Help MCP Server](../mcp_tooling.md#spec-cynai-mcptoo-helpmcpserver).
- **Scope**: `help.get` typically `both`; `specification.help` typically `pm`.
