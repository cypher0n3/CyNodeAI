# MCP Tool Definitions

- [1 Overview](#1-overview)
- [2 Alignment With MCP](#2-alignment-with-mcp)
- [3 Types](#3-types)
- [4 Agent Scope](#4-agent-scope)
- [5 Help MCP](#5-help-mcp)
- [6 Sub-Tools](#6-sub-tools)
- [7 Hierarchical / Namespaced Tool Names](#7-hierarchical--namespaced-tool-names)
- [8 Example](#8-example)

## 1 Overview

- Spec ID: `CYNAI.MCPTOO.ToolDefinitions` <a id="spec-cynai-mcptoo-tooldefinitions"></a>

This spec defines how **MCP tools** (internal and external) are represented as data: a logical tool backed by a single **MCP server** and an ordered sequence of one or more **tools** (direct MCP tool invocations or sub-tool references).

The **same structs** (`MCPTool`, `ToolInvocation`, `ToolAgentScope`) are used for **internal** (built-in gateway) and **external** (user-installed or remote) tool definitions; the format is identical; only registration and server context differ.

Definitions are serialized as YAML or JSON and used by the orchestrator or MCP gateway to route and execute the underlying MCP tools in order.

The **server** is a top-level property: either `default` for built-in tools (gateway-implemented) or an **API endpoint** registered with the API gateway for external tools; **tools** is the ordered list of operations to perform (each either a direct tool call on that server or a **sub-tool** reference).

Direct tool names MUST match tools offered by that server (or the [MCP tool specifications](../mcp_tools/README.md) when `Server` is `default`); argument keys and types follow the catalog and [common argument requirements](mcp_tooling.md#spec-cynai-mcptoo-commonargumentrequirements).

Each tool definition MUST declare **agent scope** (which agents may invoke it), consistent with [Per-tool scope: Sandbox vs PM](../mcp_tools/access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).

### 1.1 Internal vs External

- **Internal**: Built-in tools (catalog, gateway-implemented); registered at deploy or config load; `Server` is `default`.
- **External**: User-installed or remote MCP tools; registered via [User-Installable MCP Tools](user_installable_mcp_tools.md); `Server` is an API endpoint registered with the API gateway.
- Same definition shape; the gateway or registry distinguishes internal vs external by registration source and policy, not by struct fields.

### 1.2 Related Documents

- [MCP tool specifications](../mcp_tools/README.md)
- [MCP tooling](mcp_tooling.md)
- [MCP gateway enforcement](mcp_gateway_enforcement.md)
- [User-installable MCP tools](user_installable_mcp_tools.md)
- [MCP endpoint registry](mcp_endpoint_registry.md)
- [User API Gateway](../user_api_gateway.md)

### 1.3 Dependencies and Gaps

- Spec ID: `CYNAI.MCPTOO.DependenciesAndGaps` <a id="spec-cynai-mcptoo-dependenciesgaps"></a>

- **API gateway endpoint registration** and **same endpoint, different users** are specified in [MCP Endpoint Registry](mcp_endpoint_registry.md).
  That spec defines: the endpoint record model (slug, base_url, credential_ref, owner, scope); per-user vs shared endpoints; RBAC actions (`mcp.endpoint.create/read/update/delete/use`); the User API Gateway registration API (`/v1/mcp/endpoints`); and resolution of `Server` to (base_url, credentials) in request context.

## 2 Alignment With MCP

- Spec ID: `CYNAI.MCPTOO.MCPAlignment` <a id="spec-cynai-mcptoo-mcpalignment"></a>

The [Model Context Protocol](https://modelcontextprotocol.io/specification) defines tools with `name`, `description`, and `inputSchema` (JSON Schema); invocation is via `tools/call` with tool name and arguments.

This spec defines the **definition format** (config) for composite tools (internal or external), not the wire protocol.

When a tool defined by this format is **exposed** to agents (e.g. in `tools/list`), the orchestrator or gateway SHOULD present it as an MCP-compatible tool: a single logical tool with a `name` (the definition's `Name`), optional `description`, and an `inputSchema` that reflects the logical inputs (e.g. merged or first tool's schema, or a custom schema).

Direct tool entries use the same convention as MCP `tools/call`: tool `name` and `arguments` object.

## 3 Types

Contract types for tool definitions (internal or external), tool entries (direct call or ref), and agent scope.

### 3.1 `MCPTool` Type

- Spec ID: `CYNAI.MCPTOO.MCPTool` <a id="spec-cynai-mcptoo-mcptool"></a>

A tool (internal or external) is a logical tool backed by one **MCP server** and a fixed sequence of **tools** (each a direct MCP tool call or a reference to another tool by name).

Execution order is the slice order: tools are invoked in the order given in `Tools`.

Implementations MAY pass outputs from earlier tools into later ones when that contract is defined elsewhere (e.g. step executor or skill spec).

#### 3.1.1 `MCPTool` Fields

- `Server`: Either `default` for built-in tools (gateway-implemented catalog) or an API endpoint registered with the API gateway for external tools; all direct tool calls in `Tools` are executed on this server (sub-tool refs use the referenced tool's server).
- `Tools`: ordered list of tool entries (direct invocations or sub-tool references); MUST contain at least one element.
  Implementations MAY use a slice of pointers for stable identities when mutating or resolving.
- `Name`: logical name for this tool when exposed to agents (e.g. in MCP `tools/list`); REQUIRED.
- `Help`: Markdown help text describing how and when to use this tool; REQUIRED.
  Returned by the [Help MCP](#5-help-mcp) so agents have a canonical reference; the base help response includes this as the tool's description, and `help.<tool_name>` returns it together with per-invocation help.
- `Scope`: default scope for this tool; see [Agent scope](#4-agent-scope).
  Each invocation MAY override with its own `Scope`; when an invocation omits `Scope`, it inherits this value.
  The gateway enforces scope at the **invocation level** so that e.g. `tool.write` can be PM-only while other steps in the same composite are allowed to SBA.

### 3.2 `ToolInvocation` Type

- Spec ID: `CYNAI.MCPTOO.ToolInvocation` <a id="spec-cynai-mcptoo-toolinvocation"></a>

One entry in a tool's `Tools` list: either a **direct** MCP tool call or a **sub-tool** reference by name.

Exactly one of direct call (`Name` + `Args`) or `Ref` MUST be set; the other MUST be empty/omitted.

#### 3.2.1 `ToolInvocation` Fields (Direct Call)

- `Name`: MCP tool name on the parent's `Server` (e.g. `artifact.get`, `project.get`); MUST match a tool offered by that server (or the [MCP tool specifications](../mcp_tools/README.md) when `Server` is `default`).
  Omitted when `Ref` is set.
- `Args`: key-value arguments for the tool; keys and value types MUST conform to the catalog schema for that tool (e.g. `task_id`, `path`).
  Omitted when `Ref` is set.

#### 3.2.2 `ToolInvocation` Fields (Sub-Tool Reference)

- `Ref`: logical name of another tool (internal or external); the gateway resolves that tool and executes its `Tools` sequence on that tool's `Server`.
  Omitted when `Name` and `Args` are set.

#### 3.2.3 `ToolInvocation` Scope (Per-Invocation)

- `Scope` (optional): which agent types may execute **this** invocation (sandbox, pm, or both).
  When omitted, the invocation inherits the parent `MCPTool`'s `Scope`.
  Scope is enforced at the **invocation level** so that a composite tool can mix steps allowed only to PMA (e.g. `tool.write`) with steps allowed to SBA; the gateway MUST check the caller's agent type against each invocation's effective scope before executing that step.

#### 3.2.4 `ToolInvocation` Help (Per-Invocation)

- `Help`: Markdown help text for **this** invocation: how and when to use this step; REQUIRED.
  Returned by the Help MCP when the agent requests help for a specific tool (e.g. `help.<tool_name>`): the response includes the tool's `Help` plus each invocation's `Help` so models have a canonical reference for every tool and step they can call.

### 3.3 `ToolAgentScope` Type

- Spec ID: `CYNAI.MCPTOO.ToolAgentScope` <a id="spec-cynai-mcptoo-toolagentscope"></a>

Which agent types are allowed to invoke the tool.

MUST align with [Per-tool scope: Sandbox vs PM](../mcp_tools/access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope): the gateway uses this value in addition to role-based allowlists to allow or deny tool calls.

#### 3.3.1 `ToolAgentScope` Allowed Values

- `sandbox`: only sandbox (worker) agents may invoke the tool.
- `pm`: only orchestrator-side agents (Project Manager, Project Analyst) may invoke the tool.
- `both`: sandbox and PM/PA agents may invoke the tool.

### 3.4 Go Reference (Single Source of Truth)

- Spec ID: `CYNAI.MCPTOO.ExternalToolDefGo` <a id="spec-cynai-mcptoo-externaltooldefgo"></a>

There is no existing Go struct for **tool definitions** (server + tools + scope, internal or external) in the codebase.

The existing tool-related types are **runtime** shapes: [`MCPCallRequest`](../../../agents/internal/pma/mcp_client.go) (and the equivalent in [orchestrator mcp-gateway](../../../orchestrator/cmd/mcp-gateway/main.go) (**deprecated** standalone listener; prefer the control-plane route)) with `ToolName` and `Arguments` is the wire shape for a single MCP tool call.

When implementing this spec, the definition structs SHOULD live in a single shared package (e.g. `go_shared_libs` or orchestrator config) and match the contract below.

The **direct-call** form of `ToolInvocation` (Name + Args) aligns with that call shape: at runtime, a direct entry is sent as `tool_name` / `arguments` to the gateway; in config YAML/JSON we use `name` / `args` per this spec.

<a id="ref-go-mcptool"></a>

```go
// ToolInvocation is one entry in MCPTool.Tools: either a direct MCP call (Name+Args) or a sub-tool ref (Ref).
// Exactly one of (Name+Args) or Ref must be set. Direct form aligns with MCPCallRequest at call time.
// Scope is optional; when empty, the invocation inherits the parent MCPTool's Scope. Enforced at invocation level.
type ToolInvocation struct {
    // Name is the MCP tool name on the parent's Server (e.g. "artifact.get", "project.get").
    // Set for direct calls; omit when Ref is set.
    Name string `yaml:"name" json:"name,omitempty"`
    // Args are the key-value arguments for the tool; must conform to the catalog schema for Name.
    // Set for direct calls; omit when Ref is set.
    Args map[string]any `yaml:"args" json:"args,omitempty"`
    // Ref is the logical name of another MCPTool; gateway resolves and runs that tool's Tools on its Server.
    // Set for sub-tool reference; omit when Name and Args are set.
    Ref string `yaml:"ref" json:"ref,omitempty"`
    // Scope restricts which agent types may execute this invocation (sandbox, pm, both). Empty = inherit parent MCPTool.Scope.
    Scope ToolAgentScope `yaml:"scope" json:"scope,omitempty"`
    // Help is Markdown describing how and when to use this invocation; required. Returned by the Help MCP for help.<tool_name>.
    Help string `yaml:"help" json:"help"`
}

// JSONBToolInvocations stores []*ToolInvocation for config (yaml/json) and DB (jsonb).
// Implements sql.Scanner, driver.Valuer, and json.Marshaler/Unmarshaler so one type works for both.
type JSONBToolInvocations struct{ Invocations []*ToolInvocation }

func (j JSONBToolInvocations) Value() (driver.Value, error) {
    if j.Invocations == nil {
        return []byte("[]"), nil
    }
    return json.Marshal(j.Invocations)
}

func (j *JSONBToolInvocations) Scan(value interface{}) error {
    if value == nil {
        j.Invocations = nil
        return nil
    }
    var b []byte
    switch v := value.(type) {
    case []byte:
        b = v
    case string:
        b = []byte(v)
    default:
        return errors.New("JSONBToolInvocations: unsupported type")
    }
    return json.Unmarshal(b, &j.Invocations)
}

func (j JSONBToolInvocations) MarshalJSON() ([]byte, error)   { return json.Marshal(j.Invocations) }
func (j *JSONBToolInvocations) UnmarshalJSON(b []byte) error { return json.Unmarshal(b, &j.Invocations) }

// MCPTool is the definition format for a composite tool, internal or external (config / YAML and DB).
// Same struct is used for serialization and for GORM; embed in MCPToolDefinitionRecord.
type MCPTool struct {
    // Server is "default" for built-in tools or an API endpoint registered with the API gateway for external tools.
    Server string `yaml:"server" json:"server" gorm:"column:server;index"`
    // Name is the logical name for this tool when exposed to agents (e.g. in MCP tools/list). Required.
    Name string `yaml:"name" json:"name" gorm:"column:name;uniqueIndex"`
    // Help is Markdown describing how and when to use this tool; required. Returned by the Help MCP (base list and help.<name>).
    Help string `yaml:"help" json:"help" gorm:"column:help;type:text"`
    // Scope is the default scope for invocations that omit Scope (sandbox, pm, or both). Enforced per invocation when set on ToolInvocation.
    Scope ToolAgentScope `yaml:"scope" json:"scope" gorm:"column:scope"`
    // Tools is the ordered list of invocations (direct calls or sub-tool refs). At least one required.
    Tools JSONBToolInvocations `yaml:"tools" json:"tools" gorm:"column:tools;type:jsonb"`
}

// ToolAgentScope specifies which agent types may invoke the tool (see Agent scope section).
type ToolAgentScope string

const (
    ToolAgentScopeSandbox ToolAgentScope = "sandbox" // Only sandbox (worker) agents.
    ToolAgentScopePM      ToolAgentScope = "pm"      // Only PM/PA (orchestrator-side) agents.
    ToolAgentScopeBoth    ToolAgentScope = "both"  // Sandbox and PM/PA agents.
)
```

### 3.5 GORM Wrappers (Database Source of Truth)

- Spec ID: `CYNAI.MCPTOO.GormToolDef` <a id="spec-cynai-mcptoo-gormtooldef"></a>

The **database** is the source of truth for tool registration (internal and external).

**Preferred:** `MCPToolDefinitionRecord` **embeds** `MCPTool` directly so the record is the definition plus identity and timestamps.

`MCPTool` carries gorm column tags and uses `JSONBToolInvocations` for `Tools` so the same struct works for config (yaml/json) and DB (jsonb).

Use a **base struct** (`GormModelUUID`) for ID, CreatedAt, UpdatedAt, and DeletedAt; the record embeds it and `MCPTool`.

#### 3.5.1 Why Not `gorm.Model`?

- GORM's built-in [`gorm.Model`](https://github.com/go-gorm/gorm/blob/master/model.go) uses `ID uint` (auto-increment) and includes `DeletedAt` for soft deletes.
- This project uses **UUID primary keys** (e.g. `uuid.UUID`) across tables (see [orchestrator models](../../../orchestrator/internal/models/models.go)), so we use a project-specific base (`GormModelUUID`) with `ID uuid.UUID`, timestamps, and **soft delete** (`DeletedAt`).
- Tool definitions SHOULD support soft delete so tools can be "uninstalled" or disabled without losing history; the gateway MUST exclude soft-deleted rows when loading the tool registry.

<a id="ref-go-gorm-tooldef"></a>

```go
// GormModelUUID is the common base for GORM models that use a UUID primary key, timestamps, and soft delete.
// Embed in any record struct to get ID, CreatedAt, UpdatedAt, and DeletedAt without repeating tags.
// GORM excludes rows where DeletedAt is set from normal queries; use db.Unscoped() to include them.
// Define once in the models package (or go_shared_libs) and reuse across tables.
type GormModelUUID struct {
    ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    CreatedAt time.Time      `gorm:"column:created_at" json:"created_at"`
    UpdatedAt time.Time      `gorm:"column:updated_at" json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

// MCPToolDefinitionRecord is the GORM model for persisting an MCPTool. Table: mcp_tool_definitions.
// Embeds GormModelUUID (id, timestamps, soft delete) and MCPTool (server, name, scope, tools).
// For runtime execution use the embedded MCPTool; the invocation list is r.Tools.Invocations.
type MCPToolDefinitionRecord struct {
    GormModelUUID
    MCPTool
}

func (MCPToolDefinitionRecord) TableName() string { return "mcp_tool_definitions" }
```

#### 3.5.2 Alternative: Normalized One-To-Many

If the definition struct must stay free of GORM tags (e.g. shared with a non-Go consumer), use a separate table for invocations instead of embedding `MCPTool` and storing `Tools` as JSONB:

- **Table `mcp_tool_definitions`**: `id` (uuid), `created_at`, `updated_at`, `deleted_at`, `server`, `name`, `help` (text, required), `scope` (no `tools` column).
- **Table `mcp_tool_invocation_records`**: `id` (uuid), `tool_definition_id` (uuid FK to `mcp_tool_definitions.id`), `ordinal` (int, order), `name` (text), `args` (jsonb), `ref` (text), `scope` (text, optional), `help` (text, required; Markdown per-invocation help).
- `MCPToolDefinitionRecord` has a one-to-many association to `MCPToolInvocationRecord`; load/save the slice by querying or creating child rows.
  GORM: `Tools []MCPToolInvocationRecord` with `foreignKey:ToolDefinitionID`.
- Conversion to/from `MCPTool` (with `Tools []*ToolInvocation`) happens in the application layer when loading for the gateway or saving from config.

#### 3.5.3 Record Rules

- **Base struct**: Use a shared base (e.g. `GormModelUUID`) for all UUID-keyed GORM tables so ID, CreatedAt, UpdatedAt, and DeletedAt are defined once; embed it in each record.
  If the project already has such a type in the models package, use that instead of defining a new one.
- **Soft delete**: Rows with `DeletedAt` set are excluded from default GORM queries.
  The gateway MUST load only non-deleted tool definitions for the registry; use `Unscoped()` when querying for audit or admin list of deleted tools.
- Table name: `mcp_tool_definitions` (or as defined in [postgres_schema](../postgres_schema.md) when the table is added).
- `Name` SHOULD be unique so refs resolve deterministically; use a unique index (consider a unique partial index that excludes soft-deleted rows if the same name may be reused after delete).
- Load records at gateway startup or on demand; use the embedded `MCPTool` (e.g. `r.MCPTool` or `r.Tools.Invocations`) for resolution and execution.
- Migrations and DDL for `mcp_tool_definitions` belong in the schema spec and migration flow; this spec defines the Go contract only.

The existing agent-side type in [pma](../../../agents/internal/pma/mcp_tools.go) / [sba](../../../agents/internal/sba/mcp_tools.go) (the langchaingo tool that forwards to the gateway) is distinct from the definition type `MCPTool` in this spec; the former is not replaced by the latter.

Definitions (internal or external) are consumed by the orchestrator or gateway to resolve and execute composite tools (including sub-tool refs).

## 4 Agent Scope

- Spec ID: `CYNAI.MCPTOO.ExternalToolAgentScope` <a id="spec-cynai-mcptoo-externaltoolagentscope"></a>

Scope is enforced at the **invocation level**: each step in a composite tool's `Tools` list has an effective scope (the invocation's `Scope` if set, otherwise the parent `MCPTool`'s `Scope`).

The gateway MUST check the caller's agent type against each invocation's effective scope before executing that step.

This allows a composite tool to mix steps that are PM-only (e.g. `tool.write`) with steps allowed to SBA.

Scope is persisted and enforced by the MCP gateway per [Per-tool scope: Sandbox vs PM](../mcp_tools/access_allowlists_and_scope.md#spec-cynai-mcpgat-pertoolscope).

### 4.1 Agent Scope Rules

- Every tool definition MUST include `Name`, `Help`, and a default `Scope` on the `MCPTool`; every invocation MUST include `Help`; each invocation MAY override with its own `Scope`.
- When an invocation's effective scope is `sandbox`, only callers resolved as sandbox agents may execute that step; PM/PA agents MUST be denied for that step.
- When effective scope is `pm`, only Project Manager or Project Analyst agents may execute that step; sandbox agents MUST be denied for that step.
- When effective scope is `both`, both sandbox and PM/PA agents may execute that step subject to role allowlists.
- User-installed external tools follow the same scope model; see [User-Installable MCP Tools](user_installable_mcp_tools.md).

## 5 Help MCP

- Spec ID: `CYNAI.MCPTOO.HelpMcpContract` <a id="spec-cynai-mcptoo-helpmcpcontract"></a>

The **help MCP** is the canonical reference for agents while they are operating: agents call it (via the MCP gateway, e.g. `help.get`) to get documentation on how and when to use tools and invocations they have access to.

The gateway returns data derived from tool definitions: each `MCPTool` and each `ToolInvocation` MAY include **help text** (Markdown); the help MCP exposes that so models can look up usage without out-of-band docs.

### 5.1 Help MCP Base Call (Overview)

- When the agent calls the help MCP **without** a tool-specific topic (e.g. no `topic` or `topic=overview`), the gateway MUST return:
  1. **How to use the help MCP**: how to request help (e.g. call `help.get` with optional `topic` or `path`), and that `topic` may be a tool name to get detailed help for that tool.
  2. **Available tools**: the list of tools the **caller is allowed to use** (per allowlist and scope), each with a short **description** (the tool's `Help` field).
- The list of available tools MUST be scoped to the caller's identity (e.g. sandbox vs PM) so the agent only sees tools it can invoke.

### 5.2 Help MCP Per-Tool Call (`help.<tool_name>`)

- When the agent requests help for a specific tool (e.g. `topic=artifact.copy` or `topic=help.artifact.copy`), the gateway MUST return:
  1. The **tool's** `Help` markdown (how and when to use this tool overall).
  2. **All invocations** in that tool's `Tools` list, in order, each with:
     - The underlying tool name or ref (e.g. `artifact.get`, `ref: other_tool`).
     - That invocation's **help text** (`Help` field).
     - Effective scope for that step (sandbox / pm / both), so the agent knows who may run it.
- If the requested tool is not in the caller's allowed set, the gateway MAY return not-found or a restricted message; it MUST NOT return help for tools the caller cannot use.
- Response format (Markdown or structured) is implementation-defined; content MUST be size-limited and MUST NOT include secrets.

### 5.3 Help MCP Relationship to Tool Definitions

- Every tool and invocation has required help text (MCPTool.`Help`, ToolInvocation.`Help`); the help MCP returns that content for the caller's allowed set.
- The help MCP effectively performs an API call to the MCP gateway; the gateway resolves the caller's allowed tools from the registry and returns the corresponding help content from the tool definitions.

## 6 Sub-Tools

- Spec ID: `CYNAI.MCPTOO.SubTools` <a id="spec-cynai-mcptoo-subtools"></a>

A **sub-tool** is a reference (`Ref`) from one tool to another by logical name.

When the gateway executes a tool entry with `Ref` set, it resolves the referenced tool (by `Name` or stored ID) and runs that tool's `Tools` sequence on that tool's `Server`, then continues with the next entry in the caller's `Tools` list.

### 6.1 Sub-Tool Rules

- Reference resolution MUST be by logical tool name (the tool's `Name`); the registry of tools (internal and external) is implementation-defined (e.g. same config file, catalog, or database).
- References MUST NOT form a cycle: if tool A references B and B references A (directly or transitively), resolution MUST fail and the gateway MUST reject or error the invocation.
- When executing a referenced tool, each invocation in that tool's `Tools` sequence is checked against that invocation's effective scope (and the caller's agent type); the gateway MUST deny any step the caller is not allowed to run.

## 7 Hierarchical / Namespaced Tool Names

- Spec ID: `CYNAI.MCPTOO.NamespacedToolNames` <a id="spec-cynai-mcptoo-namespacedtoolnames"></a>

Tool names on the MCP wire are opaque strings; dotted names such as `system.properties.maxfilesize.set` are valid.

This section describes how to **break down** such a name in the definition format.

### 7.1 Name Breakdown Options

- **Single invocation**: Use the full name as `Name` in one direct `ToolInvocation`.
  The server or gateway treats the string as the canonical tool name; no structural breakdown in the definition.
- **Composition by ref**: Implement the logical operation as a short sequence.
  For example, an MCPTool with `Name: system.properties.maxfilesize.set` could have `Tools`: first a `Ref` to an MCPTool that gets/validates the property (e.g. `system.properties.maxfilesize.get` or a named composite), then a direct call to a `set`-style tool (or the inverse: get then set in two steps).
  The breakdown is then by **steps** (get -> validate -> set), not by parsing the string.
- **Structured name (future)**: The spec could later add an optional `Path` or `Segments` (e.g. `["system", "properties", "maxfile", "size", "set"]`) on `ToolInvocation` for gateways or servers that route by path; the wire would still send a single `tool_name` (e.g. the join of segments).
  Until then, use the single string `Name` or composition via Ref.

### 7.2 Naming Convention

- Segments in a dotted name MAY be used by the gateway or server for routing, allowlisting, or grouping (e.g. `system.*` on a system server); the catalog and enforcement rules define how.
  This spec does not require a particular segment semantics.

## 8 Example

The following examples show direct tool lists and sub-tool references.

### 8.1 Direct Tools Only

One tool on the default gateway server that fetches an artifact then writes it elsewhere (conceptually; actual tools and args come from the catalog).

Required `help` on the tool and on each invocation is returned by the Help MCP:

```yaml
server: default
name: artifact.copy
scope: both
help: |
  Copy an artifact from one path to another within the same task.
  Use when you need to duplicate or move a file in the task artifact store.
tools:
  - name: artifact.get
    args:
      task_id: "{{task_id}}"
      path: "src/file.txt"
    help: Fetches the source artifact content.
  - name: artifact.put
    args:
      task_id: "{{task_id}}"
      path: "dst/file.txt"
      content_bytes_base64: "{{previous_result}}"  # if executor supports substitution
    help: Writes the content to the destination path.
```

### 8.2 Per-Invocation Scope

A composite tool that allows SBA to run the first step but restricts the write step to PMA only:

```yaml
server: default
name: artifact.fetch.then.write
scope: both
tools:
  - name: artifact.get
    args: { task_id: "{{task_id}}", path: "input.txt" }
    # no scope: inherits "both"
  - name: tool.write
    args: { task_id: "{{task_id}}", path: "output.txt", content: "{{previous_result}}" }
    scope: pm
```

### 8.3 With Sub-Tool Reference

A tool that invokes another tool by name then runs a direct call:

```yaml
server: default
name: artifact.copy.then.notify
scope: pm
tools:
  - ref: artifact.copy
  - name: preference.get
    args:
      key: "notify_webhook"
# ref resolves to the tool named artifact.copy and runs its tools on its server
```

Placeholder substitution (e.g. `{{task_id}}`, `{{previous_result}}`) is execution/runtime behavior and MAY be defined in a separate spec (e.g. step executor or skill execution).
