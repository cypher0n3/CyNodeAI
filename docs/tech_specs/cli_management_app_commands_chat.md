# CLI Management App - Chat Command

- [Document Overview](#document-overview)
- [Chat Command](#chat-command)
  - [`cynork chat` Invocation](#cynork-chat-invocation)
  - [`cynork chat` Optional Flags](#cynork-chat-optional-flags)
  - [Thread Controls](#thread-controls)
  - [One-Shot Mode](#one-shot-mode)
  - [`cynork chat` Behavior](#cynork-chat-behavior)
  - [Chat Session Warm-Up](#chat-session-warm-up)
  - [`cynork chat` Slash Commands and Discoverability](#cynork-chat-slash-commands-and-discoverability)
  - [`cynork chat` Response Output (Pretty Formatting)](#cynork-chat-response-output-pretty-formatting)
  - [`cynork chat` Error Conditions](#cynork-chat-error-conditions)

## Document Overview

This document specifies the `cynork chat` command and all chat slash commands.
It defines the user-facing chat contract used by both the explicit `cynork tui` entrypoint and the `cynork chat` compatibility path.
It is part of the [cynork CLI](cynork_cli.md) specification.

## Chat Command

- Spec ID: `CYNAI.CLIENT.CliChat` <a id="spec-cynai-client-clichat"></a>

### Chat Command Traces To

- [REQ-CLIENT-0161](../requirements/client.md#req-client-0161)
- [REQ-CLIENT-0162](../requirements/client.md#req-client-0162)
- [REQ-CLIENT-0164](../requirements/client.md#req-client-0164)
- [REQ-CLIENT-0181](../requirements/client.md#req-client-0181)
- [REQ-CLIENT-0165](../requirements/client.md#req-client-0165)
- [REQ-CLIENT-0166](../requirements/client.md#req-client-0166)
- [REQ-CLIENT-0167](../requirements/client.md#req-client-0167)
- [REQ-CLIENT-0168](../requirements/client.md#req-client-0168)
- [REQ-CLIENT-0169](../requirements/client.md#req-client-0169)
- [REQ-CLIENT-0170](../requirements/client.md#req-client-0170)
- [REQ-CLIENT-0171](../requirements/client.md#req-client-0171)
- [REQ-CLIENT-0172](../requirements/client.md#req-client-0172)
- [REQ-CLIENT-0173](../requirements/client.md#req-client-0173)
- [REQ-CLIENT-0175](../requirements/client.md#req-client-0175)
- [REQ-CLIENT-0176](../requirements/client.md#req-client-0176)
- [REQ-CLIENT-0177](../requirements/client.md#req-client-0177)
- [REQ-CLIENT-0178](../requirements/client.md#req-client-0178)
- [REQ-CLIENT-0182](../requirements/client.md#req-client-0182)
- [REQ-CLIENT-0183](../requirements/client.md#req-client-0183)
- [REQ-CLIENT-0184](../requirements/client.md#req-client-0184)
- [REQ-CLIENT-0186](../requirements/client.md#req-client-0186)

The CLI MUST provide a top-level `chat` command that starts an interactive chat session with the Project Manager (PM) model.
The session MUST use the same User API Gateway and token resolution as other commands and MUST require auth.
The chat interface MUST use the gateway's OpenAI-compatible interactive chat API.
`POST /v1/chat/completions` remains the baseline line-oriented chat contract.
As part of the TUI rollout, the client chat implementation MUST also support `POST /v1/responses` under the same user-facing chat contract.
See [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md).

### `cynork chat` Invocation

- `cynork chat`.

### `cynork chat` Optional Flags

- `-c, --config` (string): path to config file (global).
- `--no-color` (bool): disable colored output (global).
- `--plain` (bool, optional): print model responses as raw text without Markdown formatting; for scripting or piping.
- `--model` (string, optional): OpenAI model identifier to use for chat completions.
  This is sent as the OpenAI `model` field in interactive chat requests.
  If omitted, the CLI uses the gateway default.
- `--project-id` (string, optional): Project identifier to associate with the chat thread and to use as project context for the session.
  If omitted, the CLI MUST use the **active project** from `cynork project set` when one is set; otherwise the CLI does not send `OpenAI-Project`, and the gateway associates the thread with the user's default project.
  When set (explicitly or from active project), the CLI MUST send this value using the OpenAI-standard `OpenAI-Project` request header on `POST /v1/chat/completions`.
- `-m, --message` (string, optional): One-shot mode.
  When provided, the CLI MUST send this single message to the gateway via the configured OpenAI-compatible interactive chat surface, print the completion content (subject to `--plain` and `--no-color`), and exit without entering the interactive loop.
  When `--message` is present, slash commands and the interactive loop are not used; see [One-shot mode](#one-shot-mode).
- `--thread-new` (bool, optional): At startup, create a new chat thread before the first completion request.
  When set, the CLI MUST call `POST /v1/chat/threads` using the effective project context from
  `--project-id` or the active project when set before the first `POST /v1/chat/completions`
  request in the session.
  Subsequent chat completion requests MUST remain OpenAI-compatible and MUST NOT require any CyNodeAI-specific thread identifier in the request body or headers.
  When omitted, the CLI uses the gateway's active-thread behavior per (user, project) and does not create a thread explicitly.

### Thread Controls

- Spec ID: `CYNAI.CLIENT.CliChatThreadControls` <a id="spec-cynai-client-clichatthreadcontrols"></a>

#### Thread Controls Traces To

- [REQ-CLIENT-0181](../requirements/client.md#req-client-0181)
- [REQ-CLIENT-0199](../requirements/client.md#req-client-0199)
- [REQ-CLIENT-0200](../requirements/client.md#req-client-0200)

The CLI MUST support explicit fresh-thread creation at startup and during an active session, and MUST respect current project context for thread creation.
Interactive chat SHOULD also expose thread-list, switch, and rename operations using the same gateway thread APIs.

#### Startup (`--thread-new`)

- When the user invokes `cynork chat --thread-new`, the CLI MUST create a new thread via `POST /v1/chat/threads` before entering the interactive loop (or before sending the one-shot message if `--message` is also set).
- The CLI MUST apply project context to `POST /v1/chat/threads` using the same effective project rules as the rest of chat.
  When a project is set for the session, the CLI MUST use that project context; when no project is set, the gateway associates the thread with the user's default project.
- Subsequent `POST /v1/chat/completions` requests in that session MUST remain OpenAI-compatible and MUST NOT require any CyNodeAI-specific thread identifier in the request body or headers.
- The observable outcome for the user MUST be that the next completion starts a fresh conversation rather than reusing the previously active thread.

#### In-Session (`/thread new`)

- When the user types `/thread new` during an active chat session, the CLI MUST call `POST /v1/chat/threads` with the current effective session project context.
- After a successful create, the CLI MUST treat the session as switched to a fresh conversation for subsequent completions while keeping the `POST /v1/chat/completions` request shape OpenAI-compatible.
- The chat session MUST continue (no exit).

#### Additional `/thread` Actions

- `/thread list` SHOULD list threads for the current user and effective project context using `GET /v1/chat/threads`.
- `/thread switch <thread_id>` SHOULD switch the active interactive session to the specified thread when the thread is owned by the authenticated user.
- `/thread rename <title>` SHOULD update the current thread title using `PATCH /v1/chat/threads/{thread_id}`.
- When summary or archive support is present on the gateway, implementations MAY surface those fields through `/thread list` or additional thread subcommands.

#### Unknown `/thread` Subcommands

- Input that starts with `/thread` but is not a known subcommand (e.g. `/thread foo`) MUST be treated as an unknown command.
- The CLI MUST print a brief error or hint (e.g. "Unknown /thread command. Use /thread new, /thread list, /thread switch, or /thread rename.") and MUST NOT send the line to the PM model.
- The session MUST continue; the current thread remains unchanged.

#### Project Context and Openai-Project

- Thread creation (startup or in-session) MUST use the same effective project scope as the current chat session: when the user has set a project (via `--project-id`, active project, or `/project set`), the new thread MUST be created within that effective project scope; when no project is set, the gateway assigns the user's default project.
- After switching to a new thread, the CLI MUST continue sending the same `OpenAI-Project` header (or none) on `POST /v1/chat/completions` as before, so that completions remain in the same project scope.

### One-Shot Mode

- Spec ID: `CYNAI.CLIENT.CliChatOneShot` <a id="spec-cynai-client-clichatoneshot"></a>

#### One-Shot Mode Traces To

- [REQ-CLIENT-0178](../requirements/client.md#req-client-0178)

When `cynork chat` is invoked with `-m, --message <text>`, the CLI MUST operate in one-shot mode:

- Resolve gateway URL and token as for interactive chat; if token is empty, exit with code 3.
- Send exactly one request using the configured OpenAI-compatible interactive chat surface with the given message (`model` and `OpenAI-Project` from `--model` and `--project-id` or active project as for interactive chat).
- Print the resulting assistant text to stdout, honoring `--plain` (raw text) and `--no-color`.
- Exit with code 0 on success; on gateway error or non-2xx, exit with the same exit codes as defined for chat error conditions (e.g. 7 for 5xx, 3 for missing token).
- The CLI MUST NOT enter the interactive loop, show slash-command prompts, or wait for further input.

### `cynork chat` Behavior

The following applies when the CLI is not in one-shot mode (i.e. when `--message` is not provided).

- The CLI MUST resolve the gateway URL and token using the same config load and token resolution as other commands.
  If the resolved token is empty, the CLI MUST exit with code 3 and MUST NOT start a chat session.
- The CLI MUST open an interactive loop: read a line of user input, send it to the gateway using the configured OpenAI-compatible interactive chat surface, receive the response, and print the response to the user.
  This loop MUST continue until the user exits (see below).
- The CLI MUST support session-exit via the slash commands `/exit` and `/quit` (see [Slash command reference](#slash-command-reference)), or EOF (e.g. Ctrl+D).
  Implementations MUST support at least `/exit`, `/quit`, or EOF; supporting all three is recommended.
- Chat input and model output MUST NOT be recorded in shell history or in any persistent history that could expose secrets; the same rules as interactive mode (REQ-CLIENT-0140) apply.
- All communication with the PM model MUST go through the User API Gateway; the CLI MUST NOT connect directly to inference or to the database.

### Chat Session Warm-Up

- Spec ID: `CYNAI.CLIENT.CliChatWarmUp` <a id="spec-cynai-client-clichatwarmup"></a>

#### Chat Session Warm-Up Traces To

- [REQ-CLIENT-0177](../requirements/client.md#req-client-0177)

When the gateway exposes a chat warm-up endpoint (e.g. `POST /v1/chat/warm`), the CLI SHOULD call it once after auth and before entering the interactive chat loop (before showing the first prompt).
The CLI MUST NOT block the prompt on warm-up completion: use fire-and-forget or a short timeout so the user can type immediately.
The model parameter MAY be omitted (gateway default) or set from the session default (e.g. `--model` if present).

### `cynork chat` Slash Commands and Discoverability

- Spec ID: `CYNAI.CLIENT.CliChatSlashCommands` <a id="spec-cynai-client-clichatslashcommands"></a>

#### Slash Commands and Discoverability Traces To

- [REQ-CLIENT-0164](../requirements/client.md#req-client-0164)

The CLI MUST make slash commands discoverable to the user.
At least one of the following MUST be implemented: (1) display the list of available slash commands when the chat session starts, or (2) support `/help` (see below) so the user can request the list at any time.
The list MUST include every slash command defined in this spec with at least the command name and a short description.
Implementations MAY show a short description next to each command (e.g. `/exit - end chat session`).

#### Slash Command Autocomplete and Inline Suggestions

- Spec ID: `CYNAI.CLIENT.CliChatSlashAutocomplete` <a id="spec-cynai-client-clichatslashautocomplete"></a>

##### Slash Command Autocomplete Traces To

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

- Spec ID: `CYNAI.CLIENT.CliChatSlashCommandReference` <a id="spec-cynai-client-clichatslashcommandreference"></a>

All slash commands start with `/` and are case-insensitive for the command name (e.g. `/help`, `/Help`, and `/HELP` are equivalent).
Input that does not start with `/` or `!` is sent to the PM model as a chat message.
Input that starts with `/` but does not match a known command SHOULD be treated as an unknown command.
The CLI MUST print a brief error or hint (e.g. "Unknown command. Type /help for available commands.") and MUST NOT send the line to the PM model.

#### Shell Escape (`!` Command)

- Spec ID: `CYNAI.CLIENT.CliChatShellEscape` <a id="spec-cynai-client-clichatshellescape"></a>

##### Shell Escape Traces To

- [REQ-CLIENT-0175](../requirements/client.md#req-client-0175)

When the user types a line that starts with `!`, the CLI MUST treat the remainder of the line as a shell command.
The CLI MUST run the command in the user's underlying shell (e.g. `sh -c "<rest>"`) and MUST display the command's combined stdout and stderr inline in the chat.
The chat session MUST continue after the command completes; the command's exit code MAY be shown (e.g. on non-zero exit).
If the command cannot be run (e.g. executable not found), the CLI MUST print an error to stderr and MUST NOT exit the chat session.
Empty `!` (no command after the space) SHOULD print a brief usage hint (e.g. "usage: ! followed by a shell command") and continue the session.
If the command is interactive and takes over the terminal, the implementation MUST suspend the TUI or chat renderer, hand the real TTY to the subprocess, and restore the chat session cleanly when the subprocess exits.

##### Required Slash Commands

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

- **`/thread new`**
  - Create a new chat thread and switch the session to it; see [Thread Controls](#thread-controls).
  - The new thread uses the current session project context.
  - No arguments.

- **`/thread list`**
  - List available chat threads for the current user and effective project context.
  - Output SHOULD include thread id, display title, and recent activity.

- **`/thread switch <thread_id>`**
  - Switch the current session to an existing thread owned by the authenticated user.

- **`/thread rename <title>`**
  - Rename the current thread using the gateway thread-title update contract.

#### Model Selection Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatModelSelection` <a id="spec-cynai-client-clichatmodelselection"></a>

##### Model Selection Traces To

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

##### Project Context Traces To

- [REQ-CLIENT-0173](../requirements/client.md#req-client-0173)

Project context affects only chat session association and any user-initiated task operations that accept a project context.
Project context MUST NOT be implicitly assigned.
When project context is set, the CLI MUST send it using the OpenAI-standard `OpenAI-Project` request header on subsequent `POST /v1/chat/completions` calls.

The chat `/project` slash commands MUST leverage the same logic and (where applicable) the same gateway calls as the `cynork project` subcommands.
Implementations SHOULD reuse the same request-building and output code paths as `cynork project list`, `cynork project get`, and `cynork project set` so that behavior stays consistent and slash commands remain a thin adapter.

- **`/project`** [no args]
  - Show the current project context for the chat session.
  - Equivalent to showing the result of the active project (same semantics as after `cynork project set`); if none is set, indicate the user's default project is used by the gateway.

- **`/project list`** [optional flags]
  - List projects; same semantics as `cynork project list`.
  - Optional: `--limit`, `--active-only`, cursor-style pagination when supported in the chat parser.

- **`/project get <project_id>`**
  - Show project details; same as `cynork project get <project_id>`.

- **`/project set <project_id>`** or **`/project <project_id>`**
  - Set the project context for the current chat session.
  - Equivalent to `cynork project set <project_id>` for the duration of the chat session; the CLI MUST set the session project context and send it on subsequent `POST /v1/chat/completions` via the `OpenAI-Project` header.
  - The CLI MUST treat an empty or \"none\" project selection as default project (no header sent; gateway uses user's default project).

#### Task Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashTask` <a id="spec-cynai-client-clichatslashtask"></a>

##### Task Slash Commands Traces To

- [REQ-CLIENT-0166](../requirements/client.md#req-client-0166)

The CLI MUST support the following task slash commands in chat.
Each MUST call the same User API Gateway endpoints as the corresponding `cynork task` subcommand.
Output MUST be shown inline in the chat (pretty-printed per [Pretty-Printed JSON Output](cli_management_app_shell_output.md#spec-cynai-client-cliprettyprintjson) when the output is JSON).
Arguments are parsed from the remainder of the line after the slash command; the CLI MAY support a subset of flags (e.g. `--limit`, `--status`) where the chat input allows.

##### Task Slash Commands Implementation Guidance

- To prevent behavioral drift, implementations SHOULD reuse the existing cynork subcommand request-building and output code paths for the corresponding operations (task, prefs, nodes, skills, status, auth).
  Slash commands should be a thin adapter that selects a subcommand and passes the parsed arguments through.

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

##### Status and Identity Traces To

- [REQ-CLIENT-0167](../requirements/client.md#req-client-0167)

- **`/status`**
  - Show gateway reachability and optionally auth status; same as `cynork status`.
  - No arguments.

- **`/whoami`**
  - Show current identity (id, user); same as `cynork auth whoami`.
  - No arguments.

- **`/auth`** [login | logout | whoami | refresh]
  - Same subcommands as `cynork auth`: `/auth whoami` (identity), `/auth logout`, `/auth refresh`, `/auth login` (optionally `-u username -p password`).
  - With no arguments, print usage.

#### Node Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashNodes` <a id="spec-cynai-client-clichatslashnodes"></a>

##### Node Slash Commands Traces To

- [REQ-CLIENT-0168](../requirements/client.md#req-client-0168)

- **`/nodes list`** [optional flags]
  - List nodes; same semantics as `cynork nodes list`.
  - Optional: `--limit`, cursor-style pagination if supported.

- **`/nodes get <node_id>`**
  - Show node details; same as `cynork nodes get <node_id>`.

#### Preferences Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashPrefs` <a id="spec-cynai-client-clichatslashprefs"></a>

##### Preferences Slash Commands Traces To

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

##### Skills Slash Commands Traces To

- [REQ-CLIENT-0170](../requirements/client.md#req-client-0170)

- **`/skills list`** [optional `--scope`, `--owner`]
  - List skills; same as `cynork skills list`.

- **`/skills get <skill_id>`**
  - Show skill content and metadata; same as `cynork skills get <skill_id>`.

### `cynork chat` Response Output (Pretty Formatting)

- Spec ID: `CYNAI.CLIENT.CliChatResponseOutput` <a id="spec-cynai-client-clichatresponseoutput"></a>

#### Response Output Traces To

- [REQ-CLIENT-0162](../requirements/client.md#req-client-0162)
- [REQ-CLIENT-0182](../requirements/client.md#req-client-0182)
- [REQ-CLIENT-0183](../requirements/client.md#req-client-0183)
- [REQ-CLIENT-0184](../requirements/client.md#req-client-0184)
- [REQ-CLIENT-0186](../requirements/client.md#req-client-0186)

- When `--plain` is not set, the CLI MUST render model responses with pretty-formatted output: interpret Markdown in the visible assistant text and display it in a human-readable way in the terminal.
- The CLI MUST support at least: headings, lists (ordered and unordered), code blocks (with optional syntax highlighting), inline code, emphasis (bold/italic), and links.
  Display MAY use terminal styling (e.g. indentation, colors, or borders) so that structure is clear without raw Markdown syntax.
- The CLI MUST honor `--no-color` for chat output (no colors or minimal styling when set).
- When structured turn data is available, the interactive chat UI SHOULD prefer it over scraping prose from plain assistant text.
- Thinking or reasoning content MUST NOT be rendered as normal assistant transcript text by default.
  When the CLI is running in a rich interactive chat UI, it MAY expose a collapsed placeholder or explicit expand action for available thinking data.
- Tool activity SHOULD be rendered as distinct non-prose status rows when structured tool metadata is available.
- When one user prompt yields multiple assistant-side output items, the interactive chat UI MUST render those items in order as one logical assistant turn.
- When `--plain` is set, the CLI MUST print only the canonical visible assistant text with no Markdown parsing or styling, so that output is suitable for piping or scripting.
  Hidden thinking data, tool metadata, and other non-text structured items MUST NOT be emitted in `--plain` mode.

### `cynork chat` Error Conditions

- Missing or invalid token: exit code 3.
- Gateway unreachable or 5xx: exit code 7.
- Gateway 4xx (e.g. 429, 403): exit code per [Exit Codes](cynork_cli.md#spec-cynai-client-cliexitcodes) (e.g. 3 for 403, 6 for 422).

#### Slash and Shell Command Errors Must Not Exit the Session

- Spec ID: `CYNAI.CLIENT.CliChatSubcommandErrors` <a id="spec-cynai-client-clichatsubcommanderrors"></a>

##### Subcommand Errors Traces To

- [REQ-CLIENT-0176](../requirements/client.md#req-client-0176)

When a slash command (e.g. `/skills list`, `/prefs list`) or a shell-escape command (`! ...`) fails (e.g. gateway returns 404, or the shell command exits non-zero), the CLI MUST display the error to the user (e.g. on stderr) and MUST continue the chat session.
The CLI MUST NOT exit with a non-zero code or show the top-level command Usage in response to such a failure.
Only session-exit actions (`/exit`, `/quit`, EOF) or fatal startup failures (e.g. missing token) MUST cause the chat process to exit.
