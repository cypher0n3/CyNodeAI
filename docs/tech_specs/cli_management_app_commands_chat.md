# CLI Management App - Chat Command

- [Document overview](#document-overview)
- [Chat Command](#chat-command)
- [Slash Command Reference](#slash-command-reference)

## Document Overview

This document specifies the `cynork chat` command and all chat slash commands.
It is part of the [CLI management app](cli_management_app.md) specification.

## Chat Command

- Spec ID: `CYNAI.CLIENT.CliChat` <a id="spec-cynai-client-clichat"></a>

Traces To:

- [REQ-CLIENT-0161](../requirements/client.md#req-client-0161)
- [REQ-CLIENT-0162](../requirements/client.md#req-client-0162)
- [REQ-CLIENT-0164](../requirements/client.md#req-client-0164)
- [REQ-CLIENT-0165](../requirements/client.md#req-client-0165)
- [REQ-CLIENT-0166](../requirements/client.md#req-client-0166)
- [REQ-CLIENT-0167](../requirements/client.md#req-client-0167)
- [REQ-CLIENT-0168](../requirements/client.md#req-client-0168)
- [REQ-CLIENT-0169](../requirements/client.md#req-client-0169)
- [REQ-CLIENT-0170](../requirements/client.md#req-client-0170)
- [REQ-CLIENT-0171](../requirements/client.md#req-client-0171)
- [REQ-CLIENT-0172](../requirements/client.md#req-client-0172)
- [REQ-CLIENT-0173](../requirements/client.md#req-client-0173)

The CLI MUST provide a top-level `chat` command that starts an interactive chat session with the Project Manager (PM) model.
The session MUST use the same User API Gateway and token resolution as other commands and MUST require auth.
The chat interface MUST use the OpenAI-compatible chat API (`POST /v1/chat/completions`).
See [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md).

### `cynork chat` Invocation

- `cynork chat`.

Optional flags

- `-c, --config` (string): path to config file (global).
- `--no-color` (bool): disable colored output (global).
- `--plain` (bool, optional): print model responses as raw text without Markdown formatting; for scripting or piping.
- `--model` (string, optional): OpenAI model identifier to use for chat completions.
  This is sent as the OpenAI `model` field in requests to `POST /v1/chat/completions`.
  If omitted, the CLI uses the gateway default.
- `--project-id` (string, optional): Project identifier to associate with the chat thread and to use as project context for the session.
  If omitted, no project is associated by default.

### `cynork chat` Behavior

- The CLI MUST resolve the gateway URL and token using the same config load and token resolution as other commands.
  If the resolved token is empty, the CLI MUST exit with code 3 and MUST NOT start a chat session.
- The CLI MUST open an interactive loop: read a line of user input, send it to the gateway via `POST /v1/chat/completions` using an OpenAI-format request body, receive the completion response, and print the response to the user.
  This loop MUST continue until the user exits (see below).
- The CLI MUST support session-exit via the slash commands `/exit` and `/quit` (see [Slash command reference](#slash-command-reference)), or EOF (e.g. Ctrl+D).
  Implementations MUST support at least `/exit`, `/quit`, or EOF; supporting all three is recommended.
- Chat input and model output MUST NOT be recorded in shell history or in any persistent history that could expose secrets; the same rules as interactive mode (REQ-CLIENT-0140) apply.
- All communication with the PM model MUST go through the User API Gateway; the CLI MUST NOT connect directly to inference or to the database.

### `cynork chat` Slash Commands and Discoverability

- Spec ID: `CYNAI.CLIENT.CliChatSlashCommands` <a id="spec-cynai-client-clichatslashcommands"></a>

Traces To:

- [REQ-CLIENT-0164](../requirements/client.md#req-client-0164)

The CLI MUST make slash commands discoverable to the user.
At least one of the following MUST be implemented: (1) display the list of available slash commands when the chat session starts, or (2) support `/help` (see below) so the user can request the list at any time.
The list MUST include every slash command defined in this spec with at least the command name and a short description.
Implementations MAY show a short description next to each command (e.g. `/exit - end chat session`).

#### Slash Command Autocomplete and Inline Suggestions

- Spec ID: `CYNAI.CLIENT.CliChatSlashAutocomplete` <a id="spec-cynai-client-clichatslashautocomplete"></a>

Traces To:

- [REQ-CLIENT-0165](../requirements/client.md#req-client-0165)

When the user types `/` at the start of the chat input line, the CLI MUST display the list of available slash commands inline (e.g. directly below the input).
Each list entry MUST show the command name (e.g. `/exit`, `/help`) and a short description of what the command does.
Layout SHOULD be two-column or similar: command on the left, description on the right, so the user can scan quickly.
The CLI MUST support Tab (and MAY support Shift+Tab) to complete or cycle through the listed commands.
The CLI MUST support arrow-up and arrow-down (or equivalent keys) to move the selection through the list; the selected command is the one that will be inserted or executed on Tab or Enter.
The currently selected or suggested command MUST be visually indicated (e.g. highlighted, or prefixed with an arrow or marker) so the user knows which command will be inserted or completed.
As the user types more characters after `/`, the list MAY filter to only commands that match the prefix; Tab and arrow keys then operate on the filtered list.
When the user commits the line (e.g. Enter), the CLI MUST execute the selected or typed slash command, or send the line to the PM if it is not a slash command.
Implementations MUST honor `--no-color` for the suggestion list (e.g. no colors or minimal styling when set).

#### Slash Command Reference

All slash commands start with `/` and are case-insensitive for the command name (e.g. `/help`, `/Help`, and `/HELP` are equivalent).
Input that does not start with `/` is sent to the PM model as a chat message.
Input that starts with `/` but does not match a known command SHOULD be treated as an unknown command.
The CLI MUST print a brief error or hint (e.g. "Unknown command. Type /help for available commands.") and MUST NOT send the line to the PM model.

Required slash commands:

- **`/exit`**
  - End the chat session and return to the shell.
  - The CLI MUST close the session, release resources, and exit with code 0.
  - No arguments.
  - Synonyms: none (use `/quit` for the same effect).

- **`/quit`**
  - Same behavior as `/exit`: end the chat session and return to the shell.
  - No arguments.

- **`/help`**
  - Display the list of available slash commands and a short description of each.
  - The CLI MUST NOT send `/help` as a message to the PM model; it is handled locally.
  - No arguments.
  - Output SHOULD match or extend the list shown at session start (if the implementation shows commands on start).

- **`/clear`**
  - Clear the terminal display (or scrollback) so the user can focus on the next exchange.
  - No arguments.
  - If not supported, the CLI MAY ignore the command or print a short message that clearing is not available.

- **`/version`**
  - Print the cynork version string (same as `cynork version`).
  - No arguments.
  - Useful inside a long chat session without leaving the session.

#### Model Selection Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatModelSelection` <a id="spec-cynai-client-clichatmodelselection"></a>

Traces To:

- [REQ-CLIENT-0171](../requirements/client.md#req-client-0171)
- [REQ-CLIENT-0172](../requirements/client.md#req-client-0172)

The CLI SHOULD support model selection and discovery within a chat session.
Model selection affects only chat completion requests.
Model selection MUST NOT change any user preference or system setting.

- **`/models`**
  - List available OpenAI model identifiers for the gateway.
  - The CLI MUST call `GET /v1/models` and display the returned model ids.
  - No arguments.

- **`/model`** [`<model_id>`]
  - Show or set the model used for subsequent `POST /v1/chat/completions` calls in the current session.
  - With no argument, the CLI MUST print the current selected model id.
  - With `<model_id>`, the CLI MUST set the current selected model id.
  - The CLI SHOULD validate the id against `/models` when available.

#### Project Context Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatProjectContext` <a id="spec-cynai-client-clichatprojectcontext"></a>

Traces To:

- [REQ-CLIENT-0173](../requirements/client.md#req-client-0173)

Project context affects only chat session association and any user-initiated task operations that accept a project context.
Project context MUST NOT be implicitly assigned.

- **`/project`** [`<project_id>`]
  - Show or set the project context for the current chat session.
  - With no argument, the CLI MUST print the current selected project id (or indicate none).
  - With `<project_id>`, the CLI MUST set the current selected project id for the session.
  - The CLI MUST treat an empty project selection as \"no project\".

#### Task Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashTask` <a id="spec-cynai-client-clichatslashtask"></a>

Traces To:

- [REQ-CLIENT-0166](../requirements/client.md#req-client-0166)

The CLI MUST support the following task slash commands in chat.
Each MUST call the same User API Gateway endpoints as the corresponding `cynork task` subcommand.
Output MUST be shown inline in the chat (pretty-printed per [Pretty-Printed JSON Output](cli_management_app_shell_output.md#pretty-printed-json-output) when the output is JSON).
Arguments are parsed from the remainder of the line after the slash command; the CLI MAY support a subset of flags (e.g. `--limit`, `--status`) where the chat input allows.

- **`/task list`** [optional flags]
  - List tasks; same semantics as `cynork task list`.
  - Optional: `--limit <n>`, `--status <status>`, `--cursor <opaque>` (or equivalent).

- **`/task get <task_id>`**
  - Show task status; same as `cynork task get <task_id>`.
  - `<task_id>` is UUID or human-readable task name.

- **`/task create`** [prompt text or `--prompt "..."`]
  - Create a task.
  - The remainder of the line after `create` MAY be treated as the task prompt (plain text or Markdown).
  - Implementations MAY support `--prompt "..."` or `--task "..."` for clarity; MAY support `--name <name>`.
  - Attachments and `--task-file` are optional for chat (e.g. if the chat UI supports file references).

- **`/task cancel <task_id>`**
  - Cancel the task; same as `cynork task cancel <task_id>`.
  - Confirmation MAY be required (e.g. prompt or `--yes` in the line).

- **`/task result <task_id>`** [optional `--wait`]
  - Show task result; same as `cynork task result <task_id>`.
  - Optional `--wait` to poll until terminal status.

- **`/task logs <task_id>`** [optional `--stream stdout|stderr|all`, `--follow`]
  - Show task log lines; same as `cynork task logs <task_id>`.

- **`/task artifacts list <task_id>`**
  - List artifacts for the task; same as `cynork task artifacts list <task_id>`.

- **`/task artifacts get <task_id> <artifact_id> --out <path>`**
  - Optional in chat: requires a path; same semantics as `cynork task artifacts get`.
  - Implementations MAY support this or omit it from the chat slash set if path input is impractical.

#### Status and Identity Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashStatus` <a id="spec-cynai-client-clichatslashstatus"></a>

Traces To:

- [REQ-CLIENT-0167](../requirements/client.md#req-client-0167)

- **`/status`**
  - Show gateway reachability and optionally auth status; same as `cynork status`.
  - No arguments.

- **`/whoami`**
  - Show current identity (id, handle); same as `cynork auth whoami`.
  - No arguments.

#### Node Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashNodes` <a id="spec-cynai-client-clichatslashnodes"></a>

Traces To:

- [REQ-CLIENT-0168](../requirements/client.md#req-client-0168)

- **`/nodes list`** [optional flags]
  - List nodes; same semantics as `cynork nodes list`.
  - Optional: `--limit`, cursor-style pagination if supported.

- **`/nodes get <node_id>`**
  - Show node details; same as `cynork nodes get <node_id>`.

#### Preferences Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashPrefs` <a id="spec-cynai-client-clichatslashprefs"></a>

Traces To:

- [REQ-CLIENT-0169](../requirements/client.md#req-client-0169)

- **`/prefs list`** [optional scope flags]
  - List preference entries; same as `cynork prefs list`.

- **`/prefs get`** [key]
  - Get one or all preferences; same as `cynork prefs get` (key optional).

- **`/prefs set`** [--scope-type, --scope-id, --key, --value or --value-file]
  - Set a preference; same as `cynork prefs set`.
  - Scope and key are required; value via `--value <json>` or `--value-file <path>`.

- **`/prefs delete`** [--scope-type, --scope-id, --key]
  - Delete a preference; same as `cynork prefs delete`.

- **`/prefs effective`** [optional `--task-id <id>`, `--project-id <id>`]
  - Show effective preferences for context; same as `cynork prefs effective`.

#### Skills Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashSkills` <a id="spec-cynai-client-clichatslashskills"></a>

Traces To:

- [REQ-CLIENT-0170](../requirements/client.md#req-client-0170)

- **`/skills list`** [optional `--scope`, `--owner`]
  - List skills; same as `cynork skills list`.

- **`/skills get <skill_id>`**
  - Show skill content and metadata; same as `cynork skills get <skill_id>`.

### `cynork chat` Response Output (Pretty Formatting)

- When `--plain` is not set, the CLI MUST render model responses with pretty-formatted output: interpret Markdown in the response and display it in a human-readable way in the terminal.
- The CLI MUST support at least: headings, lists (ordered and unordered), code blocks (with optional syntax highlighting), inline code, emphasis (bold/italic), and links.
  Display MAY use terminal styling (e.g. indentation, colors, or borders) so that structure is clear without raw Markdown syntax.
- The CLI MUST honor `--no-color` for chat output (no colors or minimal styling when set).
- When `--plain` is set, the CLI MUST print the raw response text with no Markdown parsing or styling, so that output is suitable for piping or scripting.

### `cynork chat` Error Conditions

- Missing or invalid token: exit code 3.
- Gateway unreachable or 5xx: exit code 7.
- Gateway 4xx (e.g. 429, 403): exit code per [Exit Codes](cli_management_app.md#exit-codes) (e.g. 3 for 403, 6 for 422).
