# SBA Capabilities Spec Fleshing (`cynode_sba.md`)

- [Summary](#summary)
- [Current Coverage (Scattered)](#current-coverage-scattered)
- [Proposed Addition: SBA Capabilities (What the Agent Can Call and How)](#proposed-addition-sba-capabilities-what-the-agent-can-call-and-how)
- [Gaps / Follow-Up](#gaps--follow-up)
- [References](#references)

## Summary

**Date:** 2026-02-24  
**Context:** Docs-only review of `docs/tech_specs/cynode_sba.md` to better flesh out what the agent can call and how (arbitrary command execution, API egress, job status reporting, etc.).

The SBA spec already describes these behaviors but they are spread across Design Principles, Integration With Worker API, Step Types, MCP Tool Access, and Sandbox Boundary.
This report proposes adding a single **SBA Capabilities** section that consolidates "what the agent can call and how" for implementers and reviewers.

## Current Coverage (Scattered)

- **Capability:** Arbitrary command execution
  - where it appears today: Design Principles (no allowlists), Step Types (`run_command`)
- **Capability:** Job status / completion reporting
  - where it appears today: Job Lifecycle and Status Reporting (outbound via proxy; callback URL or status API; node injects URLs)
- **Capability:** Artifact upload
  - where it appears today: Result and Artifact Delivery; MCP `artifact.put`
- **Capability:** Inference
  - where it appears today: Worker Proxies (node-local Ollama or API Egress); job allowlist
- **Capability:** Web egress
  - where it appears today: Worker Proxies (HTTP_PROXY/HTTPS_PROXY to node-local proxy); `ext_net_allowed`
- **Capability:** API Egress
  - where it appears today: Worker Proxies (orchestrator-mediated endpoint); MCP Tool Access (`api.call` when allowed)
- **Capability:** MCP tools (full list)
  - where it appears today: MCP Tool Access (sandbox allowlist); gateway enforcement doc
- **Capability:** Time extension / time remaining
  - where it appears today: Timeout Extension; Time Remaining and LLM Context
- **Capability:** Temporary memory
  - where it appears today: Temporary Memory (Job-Scoped); MCP memory.*

Missing in one place: a clear enumeration of **all** outbound mechanisms (lifecycle vs MCP vs inference vs web vs API egress) and the **exact** job-status/callback contract the SBA uses (currently "implementation-defined" in worker_api.md).

## Proposed Addition: SBA Capabilities (What the Agent Can Call and How)

Add a new top-level section after **Design Principles** and before **Execution Model**, with a spec ID and anchor, and add it to the document TOC.
Suggested placement: after Design Principles and before Execution Model in `docs/tech_specs/cynode_sba.md`.

### Suggested Section Text

```markdown
## SBA Capabilities (What the Agent Can Call and How)

- Spec ID: `CYNAI.SBAGNT.Capabilities` <a id="spec-cynai-sbagnt-capabilities"></a>

This section summarizes what the SBA can invoke and the mechanisms used.
All outbound traffic from the sandbox goes through worker proxies or orchestrator-mediated endpoints; see [Worker Proxies (Inference and Web Egress)](#worker-proxies-inference-and-web-egress) and [Sandbox Boundary and Security](#sandbox-boundary-and-security).

### Local Execution (Inside the Container)

- **Arbitrary command execution:** The SBA MAY run any **user-level** command (no root).
  There are **no command or path allowlists** inside the container; enforcement is the container boundary and non-root process.
  See [Design Principles](#design-principles) and [Step Types (MVP)](#step-types-mvp).
- **Step types:** The SBA executes validated steps: `run_command`, `write_file`, `read_file`, `apply_unified_diff`, `list_tree`.
  Working directory and file access are under `/workspace` (full access) or as specified per step; symlink escape outside `/workspace` is rejected.
- **Filesystem:** Full read/write under `/workspace`; `/job/` for job input, result staging, and artifacts; `/tmp` for temporary files.

### Outbound Channels (Worker Proxies Only)

The SBA has **no direct internet or host access**. All outbound use is via:

- **Lifecycle / status** — Report job in-progress, completion, result, timeout extension.
  Outbound HTTP to orchestrator job callback URL or job-status endpoint; URLs injected by node (e.g. env or job payload).
  See [Job Lifecycle and Status Reporting](#job-lifecycle-and-status-reporting).
- **Inference** — LLM calls for planning, tool use, refinement.
  Node-local inference proxy (`OLLAMA_BASE_URL`) or orchestrator-mediated API Egress endpoint; only models in job `inference.allowed_models`.
- **MCP gateway** — Tools: artifacts, memory, skills, web, API egress.
  SBA calls orchestrator MCP gateway with agent-scoped token; only [sandbox allowlist](#mcp-tool-access-sandbox-allowlist) tools.
  Traffic goes through worker proxy.
- **Web egress** — Outbound HTTP/HTTPS (e.g. package installs, fetches).
  When `constraints.ext_net_allowed` is true, node sets `HTTP_PROXY`/`HTTPS_PROXY` to node-local web egress proxy; proxy forwards to orchestrator Web Egress Proxy (allowlisted destinations only).
- **API Egress** — External APIs (e.g. GitHub, Slack) without credentials in sandbox.
  SBA invokes MCP tool `api.call`; gateway routes to API Egress Server; credentials stay in orchestrator.
  Allowed only when task/job policy and (for external destination) `ext_net_allowed` permit.
  See [API Egress Server](api_egress_server.md).

### Job Lifecycle Reporting (What the SBA Must Call)

- **In progress:** After validating the job spec, the SBA MUST signal in progress via outbound call through the worker proxy (e.g. to orchestrator job-status endpoint or callback URL).
- **Completion:** On success, failure, or timeout, the SBA MUST report completion via outbound call to deliver the [Result contract](#result-contract) (and optionally artifact references or inline data).
- **Artifacts:** The SBA MAY upload attachments via MCP `artifact.put` (task-scoped) or stage files under `/job/artifacts/` for node-mediated delivery.
- **Timeout extension:** The SBA MUST be able to request a time extension (e.g. via job-status callback or dedicated endpoint) up to the node maximum; remaining time or deadline MUST be available to the SBA for LLM context.
  The exact mechanism (callback payload, MCP tool, or status API) is defined in the [Worker API](worker_api.md) and/or MCP tool catalog.

### MCP Tools Available to the SBA

The SBA MAY invoke only tools on the [Worker Agent allowlist](mcp_gateway_enforcement.md#spec-cynai-mcpgat-workeragentallowlist) with sandbox (or both) scope:

- **artifact.*** — `artifact.put`, `artifact.get`, `artifact.list` (task-scoped).
- **memory.*** — `memory.add`, `memory.list`, `memory.retrieve`, `memory.delete` (job-scoped; see [Temporary Memory](#spec-cynai-sbagnt-temporarymemory)).
- **skills.list**, **skills.get** — Read-only skill fetch when allowed by policy.
- **web.fetch** — Sanitized fetch when allowed by policy (e.g. Secure Browser Service).
- **web.search** — Secure web search when allowed by policy.
- **api.call** — Via API Egress when explicitly allowed for the task; credentials never in sandbox.
- **help.*** — On-demand docs (optional for worker).

Explicitly disallowed: `db.*`, `node.*`, `sandbox.*`. User-installed tools with sandbox scope MAY be added per [MCP Tool Access](#mcp-tool-access-sandbox-allowlist).

See [mcp_tool_catalog.md](mcp_tool_catalog.md) and [mcp_gateway_enforcement.md](mcp_gateway_enforcement.md).
```

### TOC Update

In the document ToC (top of file), add:

- `- [SBA Capabilities (What the Agent Can Call and How)](#sba-capabilities-what-the-agent-can-call-and-how)`
  - Place after "Design Principles" and before "Execution Model".

## Gaps / Follow-Up

1. **Job callback / status API contract:** Worker API leaves the exact mechanism (callback URL shape, status API endpoint, or MCP tool for status) implementation-defined.
   If the orchestrator or worker API spec later defines a concrete REST or MCP contract for "job status" and "job result delivery", `cynode_sba.md` should reference that and state that the SBA MUST use it for in-progress and completion reporting.
2. **Time extension:** Same as above; the SBA spec says "via job-status callback or a dedicated extension endpoint" and defers to Worker API / MCP catalog for the mechanism.
   A single sentence in the new section (as in the suggested text) keeps that deferral explicit.

## References

- `docs/tech_specs/cynode_sba.md`
- `docs/tech_specs/worker_api.md` (Job lifecycle, Node-Mediated SBA Result)
- `docs/tech_specs/mcp_gateway_enforcement.md` (Worker Agent allowlist)
- `docs/tech_specs/mcp_tool_catalog.md` (Tool names and args)
- `docs/tech_specs/api_egress_server.md` (Agent interaction model)
