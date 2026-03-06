# Cynork `chat` as Primary TUI (Cursor-Agent-Like) - Recommendations and Action Plan

## Overview

Goal: make `cynork chat` the primary, fully featured interactive TUI (similar in feel to `cursor-agent`), and remove the redundant `cynork shell` interactive REPL mode.

Scope of this document: recommendations only.
This document does not modify requirements or tech specs directly.

Key constraint: removing `cynork shell` is a requirements change because `REQ-CLIENT-0136` through `REQ-CLIENT-0159` explicitly require an interactive CLI mode.

## Current Baseline (What Exists Today)

- The `cynork chat` contract is defined in [`docs/tech_specs/cli_management_app_commands_chat.md`](../tech_specs/cli_management_app_commands_chat.md).
- The `cynork shell` (REPL) contract is defined in [`docs/tech_specs/cli_management_app_shell_output.md`](../tech_specs/cli_management_app_shell_output.md) and required by [`docs/requirements/client.md`](../requirements/client.md).
- The OpenAI-compatible chat surface is pinned and defined in [`docs/tech_specs/openai_compatible_chat_api.md`](../tech_specs/openai_compatible_chat_api.md).

Notable `chat` features already specified:

- Chat session loop over `POST /v1/chat/completions`.
- Pretty Markdown rendering (with `--plain` fallback).
- Slash command discoverability and inline autocomplete suggestions on `/`.
- A fairly rich slash command surface (tasks, status, whoami, nodes, prefs, skills, model selection, project context).
- Shell escape `!` to run local shell commands and show output inline.

Notable `shell` (REPL) features specified:

- `cynork shell` interactive mode that exposes the same command surface as non-interactive invocation.
- Tab completion for commands, flags, and task names.

## Problem Statement

The current spec splits interactive UX across two surfaces (`cynork chat` and `cynork shell`), which creates redundancy and drift risk.
If `cynork chat` becomes the primary TUI, we can move the remaining "interactive accelerators" (completion, discovery, multi-command workflows) into chat and remove the shell REPL entirely.

## Recommendations

This section proposes a consolidation strategy (remove `cynork shell`) and a concrete feature set to elevate `cynork chat` into the primary TUI surface.

### Recommendation 1: Make `cynork chat` the Only Interactive UI Surface

Rationale:

- `cynork chat` is already the interactive entrypoint required by `REQ-CLIENT-0161`.
- Chat already has discoverability, inline autocomplete, and an extensible command surface via slash commands.
- Consolidation reduces the amount of UI-specific code and test surface area.

Required doc changes (recommendations, not applied here):

- Update [`docs/requirements/client.md`](../requirements/client.md) to remove or deprecate `REQ-CLIENT-0136` through `REQ-CLIENT-0159` (interactive REPL requirements).
- Update [`docs/tech_specs/cli_management_app_shell_output.md`](../tech_specs/cli_management_app_shell_output.md) to either:
  - remove `cynork shell` entirely, or
  - mark it deprecated and specify a removal timeline (if backward compatibility is desired).
- Update [`features/cynork/cynork_shell.feature`](../../features/cynork/cynork_shell.feature) to be removed or rewritten as `chat`-based acceptance criteria.

### Recommendation 2: Expand `cynork chat` Into a Cursor-Agent-Like TUI

The chat spec today is "line loop + pretty output".
The next step is to formalize a richer TUI layout and interaction model, without requiring new gateway endpoints.

Recommended UX features (client-only, no API change required):

- Multi-line input composer (toggle, soft wrap, and explicit send key).
- Scrollback with selection and copy, plus in-TUI search over the visible buffer.
- Streaming output support when the gateway supports it, with graceful fallback to non-streaming.
- A persistent status bar (gateway URL, auth identity, project context, selected model, connection state).
- A right-side "context pane" that can switch between:
  - current project context,
  - recent tasks and their status,
  - selected task detail (result/logs),
  - slash command help.
- A unified command palette that includes slash commands and common actions.
- Autocomplete and fuzzy selection for:
  - task identifiers (UUID and human-readable names) in slash commands,
  - project selection for `/project set`,
  - model selection for `/model`.

Security requirements to preserve:

- No secrets in local history files.
- Honor `--no-color`.
- Do not print or persist tokens.

### Recommendation 3: Move Shell REPL "Power User" Capabilities Into Chat

If `cynork shell` is removed, users still need:

- rapid discovery of commands and flags,
- fast access to task identifiers,
- the ability to chain operations in one interactive session.

Recommended mapping:

- Replace "shell command surface parity" with "chat slash surface parity".
  Slash commands should remain a thin adapter over the existing request-building paths used by `cynork` subcommands, as already recommended by the chat spec.
- Replace "task-name completion in REPL" with "task identifier completion in slash contexts" (for `/task get`, `/task result`, `/task logs`, etc.).
- Keep `!` shell escape, but gate it behind an explicit `--enable-shell` flag (recommendation) if you want a safer default posture.
  This would be a requirements/spec change because `REQ-CLIENT-0175` currently requires `!` behavior.

## Proposed Requirements and Spec Updates (Concrete)

This section proposes specific doc changes for a follow-on requirements/spec update set.

### Requirements Changes (Recommended)

In [`docs/requirements/client.md`](../requirements/client.md):

- Remove (or deprecate) `REQ-CLIENT-0136` through `REQ-CLIENT-0142` and `REQ-CLIENT-0159`.
- Add a replacement requirement set that makes `cynork chat` the exclusive interactive mode and carries forward the non-redundant value:
  - multi-line input,
  - task/project/model autocomplete within chat,
  - command palette,
  - scrollback search and copy,
  - optional streaming support.

### Tech Spec Changes (Recommended)

In [`docs/tech_specs/cynork_cli.md`](../tech_specs/cynork_cli.md):

- Remove `Interactive shell mode with tab completion` from MVP scope (or mark it deprecated).

In [`docs/tech_specs/cli_management_app_shell_output.md`](../tech_specs/cli_management_app_shell_output.md):

- Remove the `Interactive Mode (REPL)` section, or rewrite it as `Chat UI interaction rules` (if you want this document to remain the home for output/scripting rules).

In [`docs/tech_specs/cli_management_app_commands_chat.md`](../tech_specs/cli_management_app_commands_chat.md):

- Add a new section that specifies the TUI layout and interaction rules (composer, panes, scrolling, search, keybindings).
- Add a section that specifies completion sources and constraints (task name list calls, project list calls, model list calls).
- Add a section that specifies "non-interactive chat" behavior for scripting (for example `--plain` plus a `--once` flag, if desired).

## Implementation Action Plan (For the Subsequent Code Change)

This plan assumes you will later approve the doc changes above, then implement against the updated contracts.
It is written to minimize drift and to keep BDD coverage aligned.

### Phase 0: Contract Decisions

- Decide whether `cynork shell` is removed immediately or deprecated for one release.
- Decide whether `!` shell escape remains required by default or becomes opt-in (`--enable-shell`).
- Decide the minimal "cursor-agent-like" TUI scope for the first iteration (layout + multi-line + completion + status bar).

### Phase 1: Requirements, Specs, and BDD Alignment

- Update requirements (`docs/requirements/client.md`) to remove REPL requirements and add chat-TUI requirements.
- Update tech specs as described above.
- Update BDD:
  - Replace [`features/cynork/cynork_shell.feature`](../../features/cynork/cynork_shell.feature) with chat-based scenarios for completion.
  - Extend [`features/cynork/cynork_chat.feature`](../../features/cynork/cynork_chat.feature) with at least one scenario that exercises a chat TUI behavior (for example, multi-line send or slash completion rendering behavior).

### Phase 2: `cynork chat` TUI Implementation

- Introduce a dedicated TUI layer for `chat` (separate from Cobra command wiring) so it can own:
  - input handling,
  - pane layout,
  - rendering,
  - completion data fetching and caching.
- Implement:
  - multi-line composer,
  - slash command suggestion rendering and selection,
  - completion hooks for task ids/names, projects, and models,
  - status bar with gateway URL, identity, model, project context.
- Maintain strict adherence to output rules:
  - `--plain` remains raw text output (no TUI embellishments beyond what is needed for interaction).
  - JSON pretty printing rules remain unchanged.

### Phase 3: Remove `cynork shell` Implementation (After Contract Update)

- Remove the `shell` command wiring and its implementation.
- Remove any supporting packages that exist solely for `shell`.
- Ensure `cynork` help, docs, and completion guidance are consistent.

### Phase 4: Validation

- Run docs validation for the updated docs set:
  - `just docs-check docs/requirements/client.md docs/tech_specs/cynork_cli.md docs/tech_specs/cli_management_app_commands_chat.md docs/tech_specs/cli_management_app_shell_output.md`
- Run the relevant BDD suite:
  - `just test-bdd` (or at minimum the cynork suite once the project supports suite-scoped runs).

## Notes and Risks

- Removing `cynork shell` requires coordinated requirements, spec, and BDD updates.
- A richer TUI should not require new gateway endpoints, but "thread selection" or "resume arbitrary thread id" would.
  The current OpenAI-compatible contract intentionally avoids CyNodeAI-specific thread identifiers on the client side.
