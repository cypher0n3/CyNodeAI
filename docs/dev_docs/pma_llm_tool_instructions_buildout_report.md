# PMA/LLM Tool Instructions Buildout - Execution Report

## Summary

**Date:** 2026-02-26. **Plan:** [pma_llm_tool_instructions_plan.md](pma_llm_tool_instructions_plan.md)

The plan was executed to populate role instruction bundles, add the default CyNodeAI skill, implement PMA chat context composition, and add the SBA baseline. `just ci` passes.

## Completed Work

Items below were implemented in this pass.

### 1. Role Instructions Bundle (PMA)

- **project_manager:** `agents/instructions/project_manager/01_baseline.md` (identity, role, responsibilities, non-goals, spec refs), `02_tools.md` (PM allowlist and tool-use contract from mcp_tool_catalog + mcp_gateway_enforcement).
  README updated.
- **project_analyst:** `agents/instructions/project_analyst/01_baseline.md`, `02_tools.md` (PA allowlist and conventions).
  README updated.
- **Loader:** `LoadInstructions` now sorts directory entries by name for deterministic order (01_baseline, 02_tools, README).

### 2. Default CyNodeAI Interaction Skill

- **File:** `agents/instructions/default_cynodeai_skill.md` - MCP/gateway usage, task/project context, sandbox/node conventions, help and skills.
  System-owned; referenced by REQ-SKILLS-0116/0117.
- **Loading:** `pma.LoadDefaultSkill(instructionsRoot)` reads the file; missing file returns empty (no error). `agents/cmd/cynode-pma/main.go` loads role bundle then default skill and concatenates into system context.

### 3. PMA Chat Path and Context Composition

- **Request:** `InternalChatCompletionRequest` extended with optional `project_id`, `task_id`, `user_id`, `additional_context` (JSON).
- **Composition:** `buildSystemContext(instructionsContent, req)` builds: baseline+role+skill (instructionsContent) -> project block (if project_id) -> task block (if task_id) -> user additional context.
  Used in `ChatCompletionHandler` before calling inference.
- **Tests:** `TestBuildSystemContext` and `TestLoadDefaultSkill_*` added.

### 4. SBA Baseline Instructions

- **Directory:** `agents/instructions/sandbox_agent/` with `01_baseline.md` (identity, role, responsibilities, worker proxy model), `02_tools.md` (Worker Agent allowlist: artifact.*, memory.*, skills.list/get, web.fetch/search, api.call, help.*).
  README notes same format as PMA and use of shared loader.

### 5. Verification and Status

- **just ci:** All lint and tests pass (including agents).
- **just e2e:** Fails when the Ollama model pull times out (e.g. `lookup registry.ollama.ai: i/o timeout`).
  This is an environment/network issue, not the buildout code.
  To run E2E without pulling the model (skip inference smoke): `E2E_SKIP_INFERENCE_SMOKE=1 just e2e`.

## Not Done in This Pass

- **Shared Go package for tool descriptions:** Tool narrative remains in instruction files; no new `agents/internal/tools` or generated text from catalog.
- **SBA wiring to load baseline:** Sandbox agent bundle is present for job or image use; SBA code path to load it via `LoadInstructions` not changed.
- **Orchestrator sending project_id/task_id:** Request fields are in place; orchestrator call sites can be extended later to pass them.

## Spec References

- cynode_pma.md (LLM context composition, instructions loading, default skill)
- skills_storage_and_inference.md (Default CyNodeAI Interaction Skill)
- mcp_tool_catalog.md, mcp_gateway_enforcement.md (allowlists, tool names)
- project_manager_agent.md, project_analyst_agent.md, cynode_sba.md
