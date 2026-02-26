# PMA and SBA LLM Tool-Instructions Buildout Plan

- [Overview](#overview)
- [Role Instructions Bundle (PMA)](#role-instructions-bundle-pma)
- [Default CyNodeAI Interaction Skill](#default-cynodeai-interaction-skill)
- [PMA Chat Path and Context Composition](#pma-chat-path-and-context-composition)
- [SBA Instructions and Context](#sba-instructions-and-context)
- [Maximize Code Reuse (Tool Presentation and Shared Tools)](#maximize-code-reuse-tool-presentation-and-shared-tools)
- [Backend and Tool Schema (Optional / Later)](#backend-and-tool-schema-optional--later)
- [Verification](#verification)
- [Spec References](#spec-references)

## Overview

**Date:** 2026-02-26

**Goal:** Ensure the LLMs used by `cynode-pma` and `cynode-sba` receive clear, complete instructions about available tool calls, gateway usage, and conventions; and maximize code reuse for tool presentation and shared tool implementations where PMA and SBA use the same tools.

## Role Instructions Bundle (PMA)

- Populate `agents/instructions/project_manager/` (and `project_analyst/`) with real content: baseline identity, role responsibilities, **tool-use contracts**, and references to specs (e.g. [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md), [mcp_gateway_enforcement.md](../tech_specs/mcp_gateway_enforcement.md)).
- Keep tool-use contracts aligned with PM vs PA allowlists; reference canonical tool names and usage patterns.
- Replace placeholder README and add structured .md/.txt files as needed so `LoadInstructions` yields a single, ordered system context.

## Default CyNodeAI Interaction Skill

- Define and maintain the built-in default skill content (e.g. SKILL.md or equivalent) that describes MCP tools, gateway usage, and conventions ([skills_storage_and_inference.md](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-defaultcynodeaiskill), REQ-SKILLS-0116).
- Decide storage: dedicated system store or reserved id in existing skill store; ensure not user-editable/deletable.
- In PMA: when the inference backend supports "skills," resolve the default skill and include it in every inference request (e.g. prepend to system context or use backend skill API if available).
- If the current backend (Ollama `/api/generate`) has no skill API, implement "skills" as inclusion of the default skill **content** in the system prompt until a skill-aware backend is used.

## PMA Chat Path and Context Composition

- In `agents/internal/pma/chat.go`, implement the full context order from the spec: baseline + role instructions (already present as loaded bundle) -> project-level context -> task-level context -> user additional context -> request messages.
- Project/task context and `additional_context` require request-scoped data (e.g. `project_id`, `task_id`, `user_id`) from the orchestrator handoff; extend `InternalChatCompletionRequest` and orchestrator call sites as needed.
- Keep a single system block for now (concatenated text) unless switching to an API that supports distinct system vs. skill segments.

## Backend and Tool Schema (Optional / Later)

- If the inference backend supports structured tools (e.g. OpenAI-style `tools`), consider fetching MCP tool schemas from the gateway or a catalog and sending them in the request so the model can use native tool-calling.
- Until then, tool information remains in narrative form in the instruction bundle and default skill.

## SBA Instructions and Context

- **Baseline context:** SBA receives baseline (identity, role, responsibilities, non-goals) in the job (`context.baseline_context`) or baked into the image; it MUST be included in every LLM prompt ([CYNAI.SBAGNT.JobContext](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-jobcontext)).
  Define a canonical SBA baseline (e.g. `agents/instructions/sandbox_agent/` or equivalent) so the orchestrator or PM can supply it in the job or so the image can bake it in.
  Same format as PMA (e.g. .md/.txt loadable) to allow shared loading logic.
- **Context order:** SBA MUST merge context in the spec order: baseline -> project-level -> task-level -> requirements/acceptance criteria -> preferences -> user additional context -> skills.
  Current `buildUserPrompt` + `appendContextBlock` already use `ContextSpec`; ensure baseline is always present and that default skill content (or reference) is included when the backend supports skills.
- **Tool presentation:** SBA today exposes tools to the LLM via langchaingo `Tool` Name/Description (local tools: run_command, write_file, read_file, etc.; MCP via single `mcp_call` with a hardcoded description listing allowed `tool_name` values).
  Add instructions to source shared tool descriptions from a single place (see [Maximize Code Reuse](#maximize-code-reuse-tool-presentation-and-shared-tools)) so SBA baseline or tool descriptions stay aligned with the MCP catalog and with PMA instruction text.
- **Default skill:** When the inference backend used by SBA supports skills, include the default CyNodeAI interaction skill in the same way as PMA (e.g. prepend skill content to the prompt or use backend skill API).
  SBA already has job-supplied context; skill content can be merged into that flow.

## Maximize Code Reuse (Tool Presentation and Shared Tools)

- **Single source for tool descriptions:** Maintain one canonical description of shared MCP tools (e.g. `artifact.*`, `web.fetch`, `web.search`, `api.call`, `help.*`) used by both PMA and SBA.
  Options: (1) generate narrative "tool-use contract" text from [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md) or a small catalog module for inclusion in PMA role instructions and SBA baseline; (2) shared Go package (e.g. under `agents/internal/tools` or `go_shared_libs/`) that exposes tool names, short descriptions, and optionally JSON schema for common tools so both agents can render consistent text or structured tool definitions.
- **Shared instruction/skill content:** The default CyNodeAI interaction skill is already specified as one content blob for all agents.
  Use it for both PMA and SBA (and any other agent).
  Where role-specific instructions are needed (e.g. PM vs PA vs SBA allowlists), keep a single "allowlist + tool summary" source (e.g. derived from [mcp_gateway_enforcement.md](../tech_specs/mcp_gateway_enforcement.md)) and slice by role so PMA instructions and SBA baseline reference the same underlying tool list with role-appropriate subsets.
- **Shared loading for instruction bundles:** PMA uses `pma.LoadInstructions(dir)` to read .md/.txt from a directory.
  SBA can reuse the same loader (e.g. move to `agents/internal/instructions` or shared package) for loading baseline from a path when the job supplies a path, or for building the SBA image baseline from `agents/instructions/sandbox_agent/` so format and ordering are identical.
- **Actual tools (MCP gateway):** Where PMA and SBA both call the same MCP tools (artifact.*, web.*, api.call, help.*), use a shared gateway client and tool descriptor layer.
  Today SBA has `MCPTool` + `MCPClient`; PMA will eventually invoke tools via the orchestrator.
  Share: (1) tool name constants and argument schemas for the common subset; (2) one place that builds the "allowed tools" description string or schema list per agent type (PM, PA, SBA) from the allowlist so SBA's `MCPTool.Description()` and PMA's instruction text (or future tool schema payload) stay in sync.
  Avoid duplicating tool names and descriptions in both `agents/internal/sba/mcp_tools.go` and PMA instruction files.
- **Local vs MCP:** SBA-only tools (run_command, write_file, read_file, apply_unified_diff, list_tree) remain in SBA; PMA does not run in a sandbox so it does not get those.
  Only the overlapping MCP tools and their presentation are shared.

## Verification

- After buildout: confirm PMA loads both the role bundle and the default skill content into the prompt (or skill API).
- Confirm SBA receives baseline (job or baked-in) and, when applicable, default skill content.
  Confirm SBA tool descriptions (local + MCP) are generated from or aligned with the shared source.
- Sanity-check that allowlist boundaries (PM vs PA vs SBA) are clearly stated in instructions and that shared tool descriptions match the MCP catalog and gateway allowlists.

## Spec References

- [cynode_pma.md](../tech_specs/cynode_pma.md): Instructions loading, LLM context composition, MCP tool access, skills and default skill.
- [cynode_sba.md](../tech_specs/cynode_sba.md): Context supplied to SBA (JobContext), MCP tool access (sandbox allowlist).
- [skills_storage_and_inference.md](../tech_specs/skills_storage_and_inference.md): Default CyNodeAI interaction skill.
- [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md): Canonical tool names and semantics.
- [mcp_gateway_enforcement.md](../tech_specs/mcp_gateway_enforcement.md): PM, PA, and worker (SBA) allowlists.
- [pmagnt.md](../requirements/pmagnt.md): REQ-PMAGNT-0107, REQ-PMAGNT-0108.
- [sbagnt.md](../requirements/sbagnt.md): REQ-SBAGNT-0107, REQ-SBAGNT-0111.
- [skills.md](../requirements/skills.md): REQ-SKILLS-0116, REQ-SKILLS-0117.
