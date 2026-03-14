# `cynork` TUI Slash Commands

- [Document Overview](#document-overview)
- [Slash Command Execution Model](#slash-command-execution-model)
  - [Slash Command Execution Model Traces To](#slash-command-execution-model-traces-to)
  - [`SlashCommandExecution` Algorithm](#slashcommandexecution-algorithm)
- [Local Session Slash Commands](#local-session-slash-commands)
  - [Local Session Slash Commands Traces To](#local-session-slash-commands-traces-to)
  - [Thinking Visibility Behavior](#thinking-visibility-behavior)
  - [Thinking Visibility Behavior Traces To](#thinking-visibility-behavior-traces-to)
  - [`LocalSlashCommands` Algorithm](#localslashcommands-algorithm)
- [Thread Slash Commands](#thread-slash-commands)
  - [Thread Slash Commands Traces To](#thread-slash-commands-traces-to)
  - [`ThreadSlashCommands` Algorithm](#threadslashcommands-algorithm)
- [Model Slash Commands](#model-slash-commands)
  - [Model Slash Commands Traces To](#model-slash-commands-traces-to)
  - [`ModelSlashCommands` Algorithm](#modelslashcommands-algorithm)
- [Project Slash Commands](#project-slash-commands)
  - [Project Slash Commands Traces To](#project-slash-commands-traces-to)
  - [`ProjectSlashCommands` Algorithm](#projectslashcommands-algorithm)
- [Status and Auth Slash Commands](#status-and-auth-slash-commands)
  - [Status and Auth Slash Commands Traces To](#status-and-auth-slash-commands-traces-to)
  - [`StatusSlashCommands` Algorithm](#statusslashcommands-algorithm)
- [Task Slash Commands](#task-slash-commands)
  - [Task Slash Commands Traces To](#task-slash-commands-traces-to)
  - [`TaskSlashCommands` Algorithm](#taskslashcommands-algorithm)
- [Node Slash Commands](#node-slash-commands)
  - [Node Slash Commands Traces To](#node-slash-commands-traces-to)
  - [`NodeSlashCommands` Algorithm](#nodeslashcommands-algorithm)
- [Preference Slash Commands](#preference-slash-commands)
  - [Preference Slash Commands Traces To](#preference-slash-commands-traces-to)
  - [`PreferenceSlashCommands` Algorithm](#preferenceslashcommands-algorithm)
- [Skill Slash Commands](#skill-slash-commands)
  - [Skill Slash Commands Traces To](#skill-slash-commands-traces-to)
  - [`SkillSlashCommands` Algorithm](#skillslashcommands-algorithm)

## Document Overview

- Spec ID: `CYNAI.CLIENT.CynorkTuiSlashCommands` <a id="spec-cynai-client-cynorktuislashcommands"></a>

This document is the canonical source of truth for slash commands used by the cynork interactive chat surface.
It applies to both the explicit `cynork tui` entrypoint and any interactive `cynork chat` path that uses the same TUI contract.

Related documents:

- [Cynork TUI](cynork_tui.md)
- [CLI management app - chat command](cli_management_app_commands_chat.md)
- [Cynork CLI](cynork_cli.md)
- [User API Gateway](user_api_gateway.md)

## Slash Command Execution Model

- Spec ID: `CYNAI.CLIENT.CynorkTui.SlashCommandExecution` <a id="spec-cynai-client-cynorktui-slashcommandexecution"></a>

Slash commands are interactive client actions.
They MUST be handled locally by the TUI command dispatcher and MUST NOT be sent to the PM model as ordinary chat text.

- Command names are case-insensitive.
- Parsing MUST split the line into a command path and a remainder payload.
- Commands that map to existing non-interactive `cynork` behavior SHOULD reuse the same request builders, gateway clients, and output rendering paths as the corresponding non-interactive command.
- Commands that mutate local TUI session state MAY complete without any gateway call.
- Unknown slash commands MUST produce a concise user-visible error and MUST leave the chat session active.
- A slash-command failure MUST NOT terminate the session unless the command is an explicit exit action.

### Slash Command Execution Model Traces To

- [REQ-CLIENT-0164](../requirements/client.md#req-client-0164)
- [REQ-CLIENT-0165](../requirements/client.md#req-client-0165)
- [REQ-CLIENT-0176](../requirements/client.md#req-client-0176)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

### `SlashCommandExecution` Algorithm

<a id="algo-cynai-client-cynorktui-slashcommandexecution"></a>

1. Detect that the submitted composer line begins with `/`.
2. Normalize the command name path case-insensitively while preserving the remainder payload for argument parsing.
3. Match the normalized command path against the command groups defined in this document.
4. If the command is known, dispatch it to the matching local handler.
5. If the command maps to an existing `cynork` or gateway-backed operation, execute the same logical operation used by the corresponding non-interactive command.
6. If the command changes only local TUI state, apply that state change and re-render the current session without issuing an unnecessary gateway request.
7. If the command fails, render an inline or stderr-equivalent error and keep the session active.
8. If the command is unknown, show a concise help hint such as `Unknown command. Type /help for available commands.` and keep the session active.

## Local Session Slash Commands

- Spec ID: `CYNAI.CLIENT.CynorkTui.LocalSlashCommands` <a id="spec-cynai-client-cynorktui-localslashcommands"></a>

The TUI MUST provide these local slash commands:

- **`/help`**: Show the available slash commands and a short description for each command.
- **`/clear`**: Clear or reset the visible transcript area without mutating persisted chat history.
- **`/version`**: Show the current cynork version string.
- **`/show-thinking`**: Reveal retained `thinking` parts for the current session transcript.
- **`/hide-thinking`**: Collapse retained `thinking` parts back to the hidden-by-default state.
- **`/exit`**: End the session and return control to the shell.
- **`/quit`**: Synonym for `/exit`.

### Local Session Slash Commands Traces To

- [REQ-CLIENT-0164](../requirements/client.md#req-client-0164)
- [REQ-CLIENT-0183](../requirements/client.md#req-client-0183)
- [REQ-CLIENT-0195](../requirements/client.md#req-client-0195)
- [REQ-CLIENT-0208](../requirements/client.md#req-client-0208)
- [REQ-CLIENT-0211](../requirements/client.md#req-client-0211)

### Thinking Visibility Behavior

- Spec ID: `CYNAI.CLIENT.CynorkTui.ThinkingVisibilityBehavior` <a id="spec-cynai-client-cynorktui-thinkingvisibilitybehavior"></a>

Thinking-visibility commands are local presentation controls.
They MUST NOT modify stored message content in the database, canonical plain-text transcript content, or gateway-side persisted structured turn data.

- `/show-thinking` MUST apply to already loaded assistant turns in the current transcript view.
- `/show-thinking` MUST also apply to older assistant turns loaded later through scrollback or history retrieval in the same session.
- `/hide-thinking` MUST restore collapsed placeholders for retained `thinking` parts without removing the underlying retained structured data.
- The collapsed placeholder restored by `/hide-thinking` MUST remain visible as a secondary-styled transcript element that indicates the assistant is thinking and SHOULD hint `/show-thinking` as the expand action.
- When a turn has no retained `thinking` part, these commands MUST leave that turn unchanged.
- `/show-thinking` MUST update the persisted local config key `tui.show_thinking_by_default` to `true`.
- `/hide-thinking` MUST update the persisted local config key `tui.show_thinking_by_default` to `false`.
- Future executions of `cynork tui` or interactive `cynork chat` MUST load that stored preference and use it as the starting thinking-visibility mode.

### Thinking Visibility Behavior Traces To

- [REQ-CLIENT-0183](../requirements/client.md#req-client-0183)
- [REQ-CLIENT-0195](../requirements/client.md#req-client-0195)
- [REQ-CLIENT-0208](../requirements/client.md#req-client-0208)
- [REQ-CLIENT-0211](../requirements/client.md#req-client-0211)

### `LocalSlashCommands` Algorithm

<a id="algo-cynai-client-cynorktui-localslashcommands"></a>

1. Match the normalized command against the local-session command set.
2. For `/help`, render the command catalog defined by this document without sending a chat request.
3. For `/clear`, clear the current visible transcript buffer or scrollback view and keep local session state intact.
4. For `/version`, render the same version information as `cynork version`.
5. For `/show-thinking`, set the session thinking-visibility mode to shown.
6. Re-render all currently loaded assistant turns so retained `thinking` parts display as expanded thinking blocks instead of collapsed placeholders.
7. Persist `tui.show_thinking_by_default: true` to the local cynork YAML config using the same atomic-write rules as the main config file.
8. For `/hide-thinking`, set the session thinking-visibility mode to hidden and re-render all currently loaded assistant turns so retained `thinking` parts return to the collapsed presentation.
9. Persist `tui.show_thinking_by_default: false` to the local cynork YAML config using the same atomic-write rules as the main config file.
10. For `/exit` or `/quit`, close the interactive session cleanly and return success.

## Thread Slash Commands

- Spec ID: `CYNAI.CLIENT.CynorkTui.ThreadSlashCommands` <a id="spec-cynai-client-cynorktui-threadslashcommands"></a>

The TUI MUST provide these thread commands:

- **`/thread new`**
- **`/thread list`**
- **`/thread switch <thread_selector>`**
- **`/thread rename <title>`**

These commands MUST use the same thread-management gateway APIs as the chat and non-interactive CLI contracts.

`<thread_selector>` is a user-typeable selector rather than a raw backend UUID.
The client MAY implement the selector as a stable short handle, a list ordinal within the current thread list view, an unambiguous displayed title form, or another compact human-typable token.
Whatever selector form is supported, the client MUST display that selector in thread-list output and other thread-switching affordances so the user can type it directly.

### Thread Slash Commands Traces To

- [REQ-CLIENT-0181](../requirements/client.md#req-client-0181)
- [REQ-CLIENT-0199](../requirements/client.md#req-client-0199)
- [REQ-CLIENT-0200](../requirements/client.md#req-client-0200)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

### `ThreadSlashCommands` Algorithm

<a id="algo-cynai-client-cynorktui-threadslashcommands"></a>

1. Parse `/thread` and its required subcommand.
2. For `/thread new`, call `POST /v1/chat/threads` with the effective current project context, set the returned thread as current, and reset transcript view state for the new thread.
3. For `/thread list`, call the thread-list API, render recent-first results, and keep the current thread unchanged unless the user issues a switch command.
4. When rendering thread-list output, include the user-typeable selector for each visible thread in addition to the user-facing title or fallback label.
5. For `/thread switch <thread_selector>`, resolve the selector to exactly one visible thread, validate the result, set the requested thread as current, and reload transcript history for that thread.
6. If the selector is ambiguous or does not match any visible thread, show a concise error and keep the current thread unchanged.
7. For `/thread rename <title>`, call the thread-title update operation for the current thread and update the local session title display on success.
8. If the `/thread` subcommand is unknown or required arguments are missing, show a concise usage error and keep the session active.

## Model Slash Commands

- Spec ID: `CYNAI.CLIENT.CynorkTui.ModelSlashCommands` <a id="spec-cynai-client-cynorktui-modelslashcommands"></a>

The TUI MUST provide:

- **`/models`**
- **`/model`** [`<model_id>`]

### Model Slash Commands Traces To

- [REQ-CLIENT-0171](../requirements/client.md#req-client-0171)
- [REQ-CLIENT-0172](../requirements/client.md#req-client-0172)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

### `ModelSlashCommands` Algorithm

<a id="algo-cynai-client-cynorktui-modelslashcommands"></a>

1. For `/models`, call `GET /v1/models` and render the returned model identifiers.
2. For `/model` with no argument, render the current session model selection.
3. For `/model <model_id>`, validate the identifier when model discovery data is available, update the local session model state, and use that model for subsequent interactive chat requests.
4. Do not mutate system settings or stored user preferences as a side effect of `/model`.

## Project Slash Commands

- Spec ID: `CYNAI.CLIENT.CynorkTui.ProjectSlashCommands` <a id="spec-cynai-client-cynorktui-projectslashcommands"></a>

The TUI MUST provide:

- **`/project`**
- **`/project list`**
- **`/project get <project_id>`**
- **`/project set <project_id>`**
- **`/project <project_id>`**

### Project Slash Commands Traces To

- [REQ-CLIENT-0173](../requirements/client.md#req-client-0173)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

### `ProjectSlashCommands` Algorithm

<a id="algo-cynai-client-cynorktui-projectslashcommands"></a>

1. Parse `/project` and its optional subcommand or bare project identifier.
2. For `/project` with no argument, render the current effective project context.
3. For `/project list` and `/project get <project_id>`, call the same gateway-backed logic as the corresponding non-interactive project commands and render the results inline.
4. For `/project set <project_id>` or `/project <project_id>`, update the current session project context so subsequent chat requests use the matching `OpenAI-Project` header behavior.
5. If the user clears project context through an accepted empty or default-project form, remove the explicit project override so the gateway default project behavior applies again.

## Status and Auth Slash Commands

- Spec ID: `CYNAI.CLIENT.CynorkTui.StatusSlashCommands` <a id="spec-cynai-client-cynorktui-statusslashcommands"></a>

The TUI MUST provide:

- **`/status`**
- **`/whoami`**
- **`/auth login`**
- **`/auth logout`**
- **`/auth whoami`**
- **`/auth refresh`**

### Status and Auth Slash Commands Traces To

- [REQ-CLIENT-0167](../requirements/client.md#req-client-0167)
- [REQ-CLIENT-0176](../requirements/client.md#req-client-0176)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

### `StatusSlashCommands` Algorithm

<a id="algo-cynai-client-cynorktui-statusslashcommands"></a>

1. For `/status`, call the same reachability or status logic as `cynork status`.
2. For `/whoami` or `/auth whoami`, call the same identity logic as `cynork auth whoami`.
3. For `/auth login`, `/auth logout`, and `/auth refresh`, delegate to the same auth workflow used by the non-interactive auth commands while preserving interactive-session recovery semantics.
4. If an auth command fails, show the error inline and keep the session active unless the user explicitly exits.

## Task Slash Commands

- Spec ID: `CYNAI.CLIENT.CynorkTui.TaskSlashCommands` <a id="spec-cynai-client-cynorktui-taskslashcommands"></a>

The TUI MUST provide:

- **`/task list`**
- **`/task get <task_selector>`**
- **`/task create ...`**
- **`/task cancel <task_selector>`**
- **`/task result <task_selector>`**
- **`/task logs <task_selector>`**
- **`/task artifacts list <task_selector>`**
- **`/task artifacts get <task_selector> <artifact_id> --out <path>`** when that path-oriented variant is supported in the chat surface

Where a task slash command references an existing task, `<task_selector>` is the backend task UUID or a user-typeable human-readable task name.

### Task Slash Commands Traces To

- [REQ-CLIENT-0166](../requirements/client.md#req-client-0166)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

### `TaskSlashCommands` Algorithm

<a id="algo-cynai-client-cynorktui-taskslashcommands"></a>

1. Parse `/task` and its required subcommand plus any remaining arguments.
2. Delegate the parsed operation to the same task request-building and gateway-client logic used by the corresponding non-interactive `cynork task` subcommand.
3. Render the result inline using the same pretty-print rules that apply elsewhere in the TUI.
4. If the operation fails, render the error inline and keep the chat session active.

## Node Slash Commands

- Spec ID: `CYNAI.CLIENT.CynorkTui.NodeSlashCommands` <a id="spec-cynai-client-cynorktui-nodeslashcommands"></a>

The TUI MUST provide:

- **`/nodes list`**
- **`/nodes get <node_id>`**

### Node Slash Commands Traces To

- [REQ-CLIENT-0168](../requirements/client.md#req-client-0168)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

### `NodeSlashCommands` Algorithm

<a id="algo-cynai-client-cynorktui-nodeslashcommands"></a>

1. Parse `/nodes` and its required subcommand.
2. Delegate to the same gateway-backed logic used by the corresponding non-interactive node commands.
3. Render results inline without leaving the active chat session.

## Preference Slash Commands

- Spec ID: `CYNAI.CLIENT.CynorkTui.PreferenceSlashCommands` <a id="spec-cynai-client-cynorktui-preferenceslashcommands"></a>

The TUI MUST provide:

- **`/prefs list`**
- **`/prefs get`**
- **`/prefs set`**
- **`/prefs delete`**
- **`/prefs effective`**

### Preference Slash Commands Traces To

- [REQ-CLIENT-0169](../requirements/client.md#req-client-0169)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

### `PreferenceSlashCommands` Algorithm

<a id="algo-cynai-client-cynorktui-preferenceslashcommands"></a>

1. Parse `/prefs` and its required subcommand plus any scope or value flags.
2. Delegate to the same preference-management logic used by the corresponding non-interactive preference commands.
3. Render the result inline and keep the session active.

## Skill Slash Commands

- Spec ID: `CYNAI.CLIENT.CynorkTui.SkillSlashCommands` <a id="spec-cynai-client-cynorktui-skillslashcommands"></a>

The TUI MUST provide:

- **`/skills list`**
- **`/skills get <skill_selector>`**
- **`/skills load <file.md>`**
- **`/skills update <skill_selector> <file.md>`**
- **`/skills delete <skill_selector>`**

Where a slash command references an existing skill, `<skill_selector>` is the backend `skill_id` or a user-typeable skill selector such as an unambiguous human-readable skill name.
If a user-typeable selector matches multiple visible skills, the client MUST fail with a concise ambiguity error and require the user to disambiguate.

### Skill Slash Commands Traces To

- [REQ-CLIENT-0170](../requirements/client.md#req-client-0170)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

### `SkillSlashCommands` Algorithm

<a id="algo-cynai-client-cynorktui-skillslashcommands"></a>

1. Parse `/skills` and its required subcommand.
2. Delegate to the same skills-management logic used by the corresponding non-interactive `cynork skills` commands.
3. For selector-based commands, resolve the provided `skill_selector` using the same selector rules as the non-interactive skills command surface.
4. Render the returned skills data or success result inline and keep the session active.
