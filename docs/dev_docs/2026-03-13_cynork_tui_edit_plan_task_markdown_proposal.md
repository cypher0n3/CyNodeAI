# Proposal: Edit Plan/Task Markdown via `cynork` TUI

- [Document Overview](#document-overview)
- [Goals and Scope](#goals-and-scope)
  - [Proposal Scope](#proposal-scope)
- [Traces to Canonical Specs and Requirements](#traces-to-canonical-specs-and-requirements)
- [User-Visible Behavior](#user-visible-behavior)
- [Cache Directory and Downloaded File](#cache-directory-and-downloaded-file)
- [Editor Invocation](#editor-invocation)
- [Return to TUI and Composer Prefill](#return-to-tui-and-composer-prefill)
- [Slash Command Surface](#slash-command-surface)
- [Gateway and Data Source Assumptions](#gateway-and-data-source-assumptions)
- [Error Handling](#error-handling)
- [Open Questions and Alternatives](#open-questions-and-alternatives)

## Document Overview

This is a **proposed draft spec** for editing a project plan or task Markdown document from within the `cynork` TUI.
The flow: download a copy of the plan (or task) Markdown to a user-local cache directory, open it in the user's preferred editor, then return to the TUI and prefill the composer with leading prose and an `@` reference to the edited file so the user can send it (or edit further) with the next message.

This document lives in `docs/dev_docs/` for review and editing; it does not yet amend the canonical tech specs or requirements.

## Goals and Scope

- Allow the user to edit plan or task Markdown **outside** the TUI in a full editor (vim, nano, etc.) without leaving the chat session.
- Use the **same** gateway and auth as existing `cynork` commands; no new backend contract beyond existing plan get (and, when available, plan update and task description retrieval).
- After the user exits the editor, **prefill the composer** with:
  - Short leading prose (e.g. "Here are my edits to the plan/task.").
  - An `@` reference to the edited local file so the next send uploads it per [REQ-CLIENT-0198](../requirements/client.md#req-client-0198) and the gateway chat contract.
- The user MAY edit or remove the prefilled content before sending; the TUI MUST NOT auto-send.

### Proposal Scope

- **In scope:** Plan document (plan_body) edit flow; optional single-task description edit when the gateway exposes it; cache location; EDITOR resolution; composer prefill with `@` path.
- **Out of scope (for this proposal):** Pushing edits back to the gateway (plan update / task update) from the TUI; that may be a separate flow or a follow-up send to the PM with the attached file.

## Traces to Canonical Specs and Requirements

- [REQ-CLIENT-0180](../requirements/client.md#req-client-0180) (project plan review and approve via gateway; CLI parity).
- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188) (TUI local cache under CLI cache directory; no secrets or message content).
- [REQ-CLIENT-0198](../requirements/client.md#req-client-0198) (`@` shorthand for local files; resolve at send time, upload per gateway contract).
- [REQ-CLIENT-0206](../requirements/client.md#req-client-0206) (hint slash commands and `@` in or adjacent to composer).
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207) (slash command parity with shell/CLI surface).
- [REQ-PROJCT-0113](../requirements/projct.md#req-projct-0113) (users must be able to edit project plans and tasks via client tools).
- [REQ-PROJCT-0114](../requirements/projct.md#req-projct-0114) (plan and task description stored as Markdown).
- [CYNAI.CLIENT.CynorkTui](../tech_specs/cynork_tui.md) (layout, composer, local cache, local config).
- [CYNAI.CLIENT.CynorkTui.SlashCommandExecution](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-slashcommandexecution) (slash commands handled locally, not sent as chat text).
- [CYNAI.CLIENT.CynorkChat.AtFileReferences](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-atfilereferences) (`@` references resolved at send time).
- [CYNAI.ACCESS.ProjectPlanMarkdown](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanmarkdown) (plan and task text as Markdown).
- [CYNAI.ACCESS.ProjectPlanClientEdit](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanclientedit) (edit via client tools).
- [CYNAI.USRGWY.ProjectPlanApi](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-projectplanapi) (get plan returns plan_name, plan_body; update plan document assumed for future push-back).
- [CYNAI.USRGWY.OpenAIChatApi](../tech_specs/openai_compatible_chat_api.md) and At-Reference Workflow (file attachment at send time).

## User-Visible Behavior

1. User invokes a slash command to edit a plan or task (e.g. `/plan edit <plan_selector>` or `/task edit <task_selector>` when task-edit is supported).
2. TUI resolves the selector to a plan (or task), fetches the plan document (plan_body) or task description from the gateway, and writes a Markdown file into the user-local cache directory (see below).
3. TUI suspends or hides so the user's **EDITOR** (or fallback) opens the cached file with a real TTY.
   User edits and saves.
4. When the editor process exits, the TUI restores and returns focus to the composer.
5. TUI **prefills** the composer with:
   - A short leading line (e.g. "Here are my edits to the plan." or "Here are my edits to the task.").
   - An `@` reference to the **absolute path** of the cached Markdown file (so the next send resolves and uploads it per REQ-CLIENT-0198).
6. User MAY edit or delete the prefilled text and `@` reference; user sends the message when ready (no auto-send).

## Cache Directory and Downloaded File

- The downloaded Markdown file MUST be written under a **user-local cache directory** dedicated to TUI edit drafts.
- Cache directory MUST follow the same placement as the TUI local cache in [REQ-CLIENT-0188](../requirements/client.md#req-client-0188): under the CLI cache directory, no secrets, no message content.
- Canonical placement (align with existing draft/convention where specified): if `XDG_CACHE_HOME` is set, use `$XDG_CACHE_HOME/cynork/`; otherwise use `~/.cache/cynork/`.
- Subdirectory for edit drafts: e.g. `edit_drafts/` or `plan_task_edits/` under that cache root so the path is deterministic and separate from other cache data.
- File name MUST be unique per edit session and SHOULD identify the source (e.g. `plan-<plan_id>.md` or `task-<task_id>.md`) and optionally a timestamp so multiple edits do not overwrite the same file unless intended; or a single well-named file per plan_id/task_id that is overwritten each time (specifier's choice; proposal prefers one file per plan/task with overwrite so the path is stable for the same plan/task).
- The TUI MUST create the cache subdirectory with mode `0700` if it does not exist when writing the first draft.
- Cached files are **local copies**; the TUI does not push them back to the gateway in this flow.
- The user sends the message with `@<path>` so the PM receives the file content in chat; the user or a later feature may then ask the PM to update the plan/task from that content.

## Editor Invocation

- The TUI MUST invoke the user's preferred editor to open the cached Markdown file.
- Editor resolution order:
  1. If the environment variable `EDITOR` is set and non-empty, use it.
  2. Else use `vim`.
  3. Else use `nano`.
  4. Else use `vi`.
- The editor MUST run with a **real TTY** so interactive editors (vim, nano, vi) work correctly; the TUI MUST suspend or hand off the terminal in the same way as for `! command` shell escape per [REQ-CLIENT-0189](../requirements/client.md#req-client-0189) and [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md) (interactive subprocesses receive the real TTY; TUI restores cleanly afterward).
- The TUI MUST wait for the editor process to exit before restoring the TUI and prefilling the composer.
- If the editor cannot be executed (e.g. not found in PATH), the TUI MUST show a concise error and MUST NOT prefill the composer.
  The cached file MAY still exist for the user to open manually.

## Return to TUI and Composer Prefill

- When the editor exits, the TUI MUST restore the chat view and set focus to the **composer**.
- The composer MUST be prefilled with:
  - One line of leading prose: e.g. "Here are my edits to the plan." (for plan) or "Here are my edits to the task." (for task).
    Exact wording MAY be configurable or localized; the intent is to give the user a ready-to-send prompt that they can edit.
  - A newline (or space), then an `@` reference to the **absolute path** of the cached Markdown file (e.g. `@/home/user/.cache/cynork/edit_drafts/plan-abc123.md`).
- The user MUST be able to edit or delete this content before sending; the TUI MUST NOT automatically send the message.
- When the user sends the message, the existing `@` resolution and upload behavior apply.
  The client MUST resolve the `@` path at send time, upload or include the file per the gateway contract, and surface a validation error if the file is missing or invalid per [REQ-CLIENT-0198](../requirements/client.md#req-client-0198).

## Slash Command Surface

- This proposal introduces (or extends) slash commands for plan (and optionally task) edit-in-editor.
- Suggested forms:
  - **`/plan edit <plan_selector>`**: Resolve plan, fetch plan document (plan_body), write to cache, open in EDITOR, then prefill composer as above.
  - **`/task edit <task_selector>`** (optional, when gateway supports task description retrieval): Resolve task, fetch task description (or task body), write to cache, open in EDITOR, then prefill composer.
- `<plan_selector>` and `<task_selector>` MUST follow the same selector semantics as elsewhere: backend UUID or user-typeable human-readable name (see [Task Slash Commands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-taskslashcommands) and project/plan CLI).
- These commands MUST be handled locally by the TUI (not sent to the PM as chat text) per the [Slash Command Execution Model](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-slashcommandexecution).
- If the slash command catalog is extended in the canonical spec, plan edit (and task edit) MUST be added under a "Plan Slash Commands" or "Task Slash Commands" section with the same structure as existing command groups (Traces To, Algorithm).

## Gateway and Data Source Assumptions

- **Plan document:** The gateway already exposes **Get plan** (e.g. `GET /v1/plans/{plan_id}`) returning `plan_name`, `plan_body`, state, task list, etc.
  The TUI uses this to obtain the Markdown (plan_body) to write to the cache.
  Plan document **update** (e.g. PATCH plan with plan_body) is required by [REQ-PROJCT-0113](../requirements/projct.md#req-projct-0113) for client edit; this proposal does not require the TUI to call update in this flow.
  The user sends the edited file in chat and can ask the PM to apply it, or a future feature can add "save back to plan" from the TUI.
- **Task description:** When the gateway exposes retrieval of a single task's description or body (e.g. from task get or from plan get task list with description fields), the same flow applies for `/task edit`.
  If the gateway does not yet expose task description in a form suitable for a single Markdown file, `/task edit` MAY be omitted or deferred until the API supports it.

## Error Handling

- If plan (or task) resolution fails (not found, auth error, gateway error): show an inline error, do not open an editor, do not prefill the composer; keep the session active.
- If the cache directory cannot be created or the file cannot be written: show an inline error, do not open an editor; keep the session active.
- If the editor executable cannot be found or fails to start: show an inline error; the cached file MAY already exist - optionally hint that the user can open the path manually.
  Do not prefill the composer with an `@` path if the user did not complete the edit in the intended editor (prefill is still useful so the user can send the file after editing elsewhere).
- If the user deletes the cached file after the editor exits but before sending: at send time, normal `@` resolution fails and the client MUST surface the same validation error as for any missing file per REQ-CLIENT-0198.

## Open Questions and Alternatives

- **Exact prefill wording:** Configurable vs fixed string; i18n.
- **Overwrite vs unique filename:** One file per plan_id/task_id (overwrite on each edit) vs timestamped unique files (preserves history but clutters cache); proposal leans one file per plan/task for stable `@` path and simpler UX.
- **Push-back to gateway:** Whether a separate slash command or post-send tool should "apply edited plan" by calling the gateway plan update API; out of scope for this draft but may be a follow-up.
- **Task edit:** Depends on gateway exposing task description in a retrievable form; defer `/task edit` to when that API exists or document it as "when supported".
