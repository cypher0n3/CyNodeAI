# MCP Access: Role Allowlists, Per-Tool Scope, and Admin Enable or Disable

- [Document Overview](#document-overview)
- [Role-Based Tool Allowlists](#role-based-tool-allowlists)
  - [Worker Agent Allowlist](#worker-agent-allowlist)
  - [Project Manager Agent Allowlist](#project-manager-agent-allowlist)
  - [Project Analyst Agent Allowlist](#project-analyst-agent-allowlist)
- [Per-Tool Scope: Sandbox vs PM](#per-tool-scope-sandbox-vs-pm)
- [Admin-Configurable Per-Tool Enable or Disable](#admin-configurable-per-tool-enable-or-disable)

## Document Overview

This document is the **canonical** specification for which built-in MCP tools each agent role may invoke (role-based allowlists), how **per-tool scope** (sandbox vs PM vs both) interacts with those allowlists, and **admin-configurable** per-tool enable or disable.

The orchestrator MCP gateway **enforces** these rules at request time; mechanics, tokens, edge mode, and auditing are in [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md).

**Agent invocations:** Role allowlists and admin enable/disable configure which tools agents may call; they do not grant **ADMIN**-level privilege to agent tool execution.
Per-tool specs that forbid **ADMIN** via MCP (e.g. [Skills MCP Tools](skills_tools.md)) are enforced at the gateway together with these rules.

Related documents

- Per-tool contracts and algorithms: [MCP tool specs](README.md)
- Naming, common arguments, response model: [MCP Tooling](../mcp/mcp_tooling.md)
- User-installed tools and configurable scope: [User-Installable MCP Tools](../mcp/user_installable_mcp_tools.md)

## Role-Based Tool Allowlists

Allowlists are a coarse safety control.
They define which tool namespaces are eligible for routing for a given agent role.

Allowlists are not sufficient for authorization.
Fine-grained authorization MUST still be enforced by access control policy rules.

### Worker Agent Allowlist

- Spec ID: `CYNAI.MCPGAT.WorkerAgentAllowlist` <a id="spec-cynai-mcpgat-workeragentallowlist"></a>

Worker agents run in sandbox containers and SHOULD have the minimal tool surface needed to complete dispatched work.

Recommended allowlist

- `artifact.*` (scoped blobs: user / group / project / global; RBAC; optional job lineage; see [Artifact tools](artifact_tools.md))
- `memory.*` (job-scoped temporary memory: `memory.add`, `memory.list`, `memory.retrieve`, `memory.delete`; **agent-local** on-disk in the SBA at `/job/memory.json`, not orchestrator gateway; see [Memory tools](../agent_local_tools/memory_tools.md))
- `preference.get`, `preference.list`, `preference.effective` (read-only; SBA needs access to effective preferences for the task/context)
- `skills.list`, `skills.get` (read-only; when allowed by policy, so the SBA can fetch relevant skills via the gateway)
- `persona.get` (and optionally `persona.list`; when allowed by policy, so the SBA can resolve persona via worker proxy when needed)
- `web.fetch` (sanitized, when allowed by policy)
- `web.search` (secure web search, when allowed by policy)
- `api.call` (through API Egress, when explicitly allowed for the task)
- `help.*` (on-demand docs; optional for worker)

Explicitly disallowed

- Resource tools that are orchestrator-side only: `task.*`, `project.*`, `job.*`, `system_setting.*` (SBA is allowed `preference.*` read access via the allowlist above.)
- `node.*`
- `sandbox.*` (worker is already inside a sandbox)

### Project Manager Agent Allowlist

- Spec ID: `CYNAI.MCPGAT.PmAgentAllowlist` <a id="spec-cynai-mcpgat-pmagentallowlist"></a>

Recommended allowlist

- Resource tools: `task.*`, `project.*`, `preference.*`, `job.*`, `system_setting.*` (and when the specifications table is adopted: `specification.*`, `plan.specifications.set`, `task.specifications.set`)
- `specification.help` (read-only schema guidance for building specification payloads; see [Help tools](help_tools.md))
- `persona.list`, `persona.get` (so PMA can resolve persona_id when creating/updating tasks and building job specs)
- `node.*` (capabilities, status, config refresh)
- `sandbox.*` (create, exec, file transfer, logs, destroy; and when enabled by system setting, `sandbox.allowed_images.list`, `sandbox.allowed_images.add`)
  - For `sandbox.allowed_images.add`, the gateway MUST allow the call only when the system setting `agents.project_manager.sandbox.allow_add_to_allowed_images` is `true`; when `false` (default), the gateway MUST reject the call.
  - `sandbox.allowed_images.list` is allowed for the PM agent regardless of that setting.
- `artifact.*` (scoped put/get/list: user / group / project / global; RBAC; see [Artifact tools](artifact_tools.md))
- `artifacts.*` (unified artifacts API: create, get, update, delete; see [Orchestrator Artifacts Storage - MCP tooling](../orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsmcpforpmapaa))
- `model.*` (registry and availability)
- `connector.*` (management and invocation, subject to policy)
- `web.fetch` (sanitized, subject to policy)
- `web.search` (secure web search, subject to policy)
- `api.call` (through API Egress, subject to policy)
- `git.*` (through Git egress, subject to policy)
- `help.*` (on-demand docs)

### Project Analyst Agent Allowlist

- Spec ID: `CYNAI.MCPGAT.PaAgentAllowlist` <a id="spec-cynai-mcpgat-paagentallowlist"></a>

Recommended allowlist

- Resource tools: limited `task.*`, `project.*`, `preference.*`, `job.*` read and limited write (verification findings only)
- `persona.list`, `persona.get` (for job building and persona resolution when PAA is involved)
- `artifact.*` (scoped artifacts; read for produced outputs; [Artifact tools](artifact_tools.md))
- `artifacts.*` (unified artifacts API for create/get/update/delete when PAA needs to store or retrieve artifacts; see [Orchestrator Artifacts Storage - MCP tooling](../orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsmcpforpmapaa))
- `web.fetch` (sanitized, when allowed for verification)
- `web.search` (secure web search, when allowed for verification)
- `api.call` (through API Egress, when allowed for verification)
- `help.*` (on-demand docs)

## Per-Tool Scope: Sandbox vs PM

- Spec ID: `CYNAI.MCPGAT.PerToolScope` <a id="spec-cynai-mcpgat-pertoolscope"></a>

The orchestrator MUST track for each MCP tool whether the tool is available to **sandbox agents**, **PM (orchestrator-side) agents**, or **both**.
This scope is used in addition to role-based allowlists so that sandbox agents never receive PM-only tools and PM/PA agents never receive sandbox-only tools unless the tool is explicitly marked for both.

Enforcement rules

- When the caller is a **sandbox agent** (worker/cynode-sba in agent mode), the gateway MUST allow the tool call only if the tool is enabled and the tool's scope includes **sandbox** (or **both**).
  The gateway MUST reject calls to tools that are PM-only.
- When the caller is a **PM or Project Analyst agent**, the gateway MUST allow the tool call only if the tool is enabled and the tool's scope includes **PM** (or **both**).
  The gateway MUST reject calls to tools that are sandbox-only.

Built-in tools

- Built-in tools in the [Worker Agent allowlist](#worker-agent-allowlist) MUST be registered with scope **sandbox** (or **both** if they are also on a PM allowlist).
- Built-in tools in the [Project Manager Agent allowlist](#project-manager-agent-allowlist) and [Project Analyst Agent allowlist](#project-analyst-agent-allowlist) MUST be registered with scope **PM**.
  Tools that appear on both worker and PM allowlists (e.g. `help.*`) MAY be registered as **both**.

User-installed (custom) MCP tools

- User-installable MCP tools (registration, per-tool scope configuration, persistence, Web Console and CLI exposure) are defined in a dedicated spec.
  Users MUST be able to install their own MCP tools and configure per-tool scope (sandbox only, PM only, or both); the orchestrator MUST persist that scope and the gateway MUST use it when enforcing the rules above.
  See [User-Installable MCP Tools](../mcp/user_installable_mcp_tools.md).

### Per-Tool Scope Requirements Traces

- [REQ-MCPGAT-0114](../../requirements/mcpgat.md#req-mcpgat-0114)
- [REQ-MCPGAT-0115](../../requirements/mcpgat.md#req-mcpgat-0115)

## Admin-Configurable Per-Tool Enable or Disable

- Spec ID: `CYNAI.MCPGAT.AdminPerToolEnableDisable` <a id="spec-cynai-mcpgat-adminpertoolenabledisable"></a>

Admins MUST be able to turn individual MCP tools on or off.

### Admin Per-Tool Enable or Disable Requirements Traces

- [REQ-MCPGAT-0113](../../requirements/mcpgat.md#req-mcpgat-0113)

- The system MUST support admin-configurable enable/disable per tool (by canonical tool name, e.g. `web.fetch`, `sandbox.create`, `git.push`).
- When a tool is disabled, the MCP gateway MUST reject invocations of that tool regardless of role allowlist or access control allow rules.
- When a tool is enabled (or not listed as disabled), normal role allowlist and access control evaluation apply.
- Configuration MAY be stored in system settings (e.g. `mcp.tools.disabled` as an array of tool names, or per-tool keys such as `mcp.tool.<tool_name>.enabled`) and/or enforced via access control rules (e.g. deny rules for specific tools).
- The Web Console and CLI management app MUST expose the ability for admins to view and change per-tool enable/disable state, consistent with [client capability parity](../cynork/cynork_cli.md) and [Web Console](../web_console.md).
