# SBA Tools Review and Spec Additions for Common Use Cases

- [Overview](#overview)
- [Current SBA Tool Surface](#current-sba-tool-surface)
- [Common Use Cases and Gaps](#common-use-cases-and-gaps)
- [Spec Additions Applied](#spec-additions-applied)
- [References](#references)

## Overview

**Date:** 2026-02-26

This document reviews the tools available to the Sandbox Agent (SBA), maps common use cases (grep, reading lines, patching, glob/find) to current behavior, and records spec additions made to cover those use cases in [cynode_sba.md](../tech_specs/cynode_sba.md).

No code changes are made in this doc-only pass; only tech spec and dev_docs updates.

## Current SBA Tool Surface

SBA capabilities split into two layers:

1. **MCP tools** (orchestrator gateway, sandbox allowlist): `artifact.*`, `memory.*`, `skills.list`/`skills.get`, `web.fetch`, `web.search`, `api.call`, `help.*`.
   Defined in [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md) and [cynode_sba.md - MCP Tool Access](../tech_specs/cynode_sba.md#mcp-tool-access-sandbox-allowlist).
2. **Local step types** (in-container, under `/workspace`): `run_command`, `write_file`, `read_file`, `apply_unified_diff`, `list_tree`.
   Defined in [cynode_sba.md - Local Tools (MVP)](../tech_specs/cynode_sba.md#local-tools-mvp); implementation in `agents/internal/sba/runner.go` and `agent_tools.go`.

Step types are the primitives the agent uses for filesystem and shell work; they do not go through MCP.

## Common Use Cases and Gaps

- **Use case: Grep / search in files**
  - Current coverage: `run_command` with `grep` or `rg` (if present in image).
  - Output capped by `max_output_bytes`.
  - Gap / spec addition: No dedicated step; behavior depends on image.
  - Spec now documents using `run_command` for search and allows a future `search_files` step type for structured, size-capped search.
- **Use case: Read certain lines**
  - Current coverage: `read_file` reads whole file; truncation when over `max_output_bytes`.
  - Gap / spec addition: No line range.
  - Spec now adds optional `start_line`/`end_line` (or equivalent) to the step-type contract so implementations can support partial reads and avoid pulling large files.
- **Use case: Patching files**
  - Current coverage: `apply_unified_diff` applies a unified diff under workspace; paths validated to stay under workspace.
  - Gap / spec addition: Spec now spells out argument schema (`diff`), path rules, and that patches are applied relative to workspace root (e.g. `patch -p1 -d workspace`).
  - Optional `dry_run` reserved for future use.
- **Use case: Glob / find files**
  - Current coverage: `run_command` with `find`/shell glob; `list_tree` returns full tree under a path (no pattern).
  - Gap / spec addition: Spec now documents `run_command` for glob/find and allows a future optional `pattern` (or equivalent) on `list_tree` for filtered listing.
- **Use case: Overwrite / create file**
  - Current coverage: `write_file` with path and content.
  - Gap / spec addition: Already covered; spec now documents args and symlink-escape rejection.

## Spec Additions Applied

The following additions were made to [cynode_sba.md](../tech_specs/cynode_sba.md) (see [cynode_sba.md - Tool Argument Schemas](../tech_specs/cynode_sba.md#tool-argument-schemas-and-common-use-cases) for exact wording):

### Step Types (MVP)

- New subsection: **Step type argument schemas and common use cases**.
- For each step type: required/optional args, path rules (under `/workspace`, symlink escape rejected), and size/behavior rules.
- **run_command:** `argv`, optional `cwd`; use for arbitrary commands including grep, find, head, sed; output capped by `max_output_bytes`.
- **read_file:** `path`; optional `start_line`/`end_line` (inclusive, 1-based) for partial read; output capped; behavior when range out of bounds or file missing.
- **write_file:** `path`, `content`; parent dirs created as needed; symlink escape rejected.
- **apply_unified_diff:** `diff` (unified diff text); paths in diff must be under workspace; applied relative to workspace root (e.g. `patch -p1 -d workspace --forward`); optional `dry_run` reserved for future.
- **list_tree:** optional `path` (default workspace root); returns tree under path; optional `pattern`/glob reserved for future.
- **Common use cases:** Grep/search via `run_command`; partial file read via `read_file` with line range when implemented; patching via `apply_unified_diff`; glob/find via `run_command`; future `search_files` and `list_tree` pattern allowed by spec.

### Spec Traceability

- Step-type schemas and use cases are part of CYNAI.
  SBAGNT.
  Enforcement; no new requirement IDs in this pass.

## References

- [cynode_sba.md](../tech_specs/cynode_sba.md) - SBA spec; Step Types (MVP), Step type argument schemas and common use cases, Design Principles, SBA Capabilities.
- [sbagnt.md](../requirements/sbagnt.md) - REQ-SBAGNT-0102, REQ-SBAGNT-0112.
- [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md) - MCP tool names and args.
- [agents/instructions/sandbox_agent/02_tools.md](../../agents/instructions/sandbox_agent/02_tools.md) - SBA tool-use contract for LLM instructions.
