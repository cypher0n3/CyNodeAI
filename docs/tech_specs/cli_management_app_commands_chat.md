# CLI Management App - Chat Command

- [Document Overview](#document-overview)
- [Chat Command](#chat-command)
  - [Chat Command Requirements Traces](#chat-command-requirements-traces)
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

This document specifies the `cynork chat` command as a **separate** entry point from `cynork tui`.
Chat and tui share the same slash-command and chat contract (thread semantics, gateway API, auth); the canonical catalog and execution algorithms live in [cynork_tui_slash_commands.md](cynork_tui_slash_commands.md).
Chat MAY use a line-oriented or other distinct implementation so it can be exercised independently for testing.
It is part of the [cynork CLI](cynork_cli.md) specification.

The CLI MUST provide a top-level `chat` command that starts an interactive chat session with the Project Manager (PM) model.
The session MUST use the same User API Gateway and token resolution as other commands and MUST require auth.
The chat interface MUST use the gateway's OpenAI-compatible interactive chat API.
`POST /v1/chat/completions` remains the baseline line-oriented chat contract.
As part of the TUI rollout, the client chat implementation MUST also support `POST /v1/responses` under the same user-facing chat contract.
See [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md).

## Chat Command

- Spec ID: `CYNAI.CLIENT.CliChat` <a id="spec-cynai-client-clichat"></a>

### Chat Command Requirements Traces

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
- `--resume-thread` (string, optional): Thread selector for an existing thread to resume at startup.
  When provided, the CLI MUST resolve the selector to a thread owned by the authenticated user (within effective project scope when applicable) and MUST start the session in that thread so that the first and subsequent completion requests continue that conversation.
  When omitted, the CLI MUST start with a **new thread** (default): it MUST call `POST /v1/chat/threads` using the effective project context from `--project-id` or the active project when set, before the first `POST /v1/chat/completions` request in the session.
  Subsequent chat completion requests MUST remain OpenAI-compatible and MUST NOT require any CyNodeAI-specific thread identifier in the request body or headers.

### Thread Controls

- Spec ID: `CYNAI.CLIENT.CliChatThreadControls` <a id="spec-cynai-client-clichatthreadcontrols"></a>

#### Thread Controls Traces To

- [REQ-CLIENT-0181](../requirements/client.md#req-client-0181)
- [REQ-CLIENT-0199](../requirements/client.md#req-client-0199)
- [REQ-CLIENT-0200](../requirements/client.md#req-client-0200)

The CLI MUST support starting with a new thread by default and resuming a previous thread only when the user supplies an explicit startup option.
It MUST support explicit fresh-thread creation during an active session (e.g. `/thread new`) and MUST respect current project context for thread creation.
Interactive chat SHOULD also expose thread-list, switch, and rename operations using the same gateway thread APIs.

#### Startup (Default: New Thread)

- **Default:** When the user invokes `cynork chat` (or `cynork tui`) without `--resume-thread`, the CLI MUST start with a **new thread**: it MUST call `POST /v1/chat/threads` before entering the interactive loop (or before sending the one-shot message if `--message` is also set).
- The CLI MUST apply project context to `POST /v1/chat/threads` using the same effective project rules as the rest of chat.
  When a project is set for the session, the CLI MUST use that project context; when no project is set, the gateway associates the thread with the user's default project.
- Subsequent `POST /v1/chat/completions` requests in that session MUST remain OpenAI-compatible and MUST NOT require any CyNodeAI-specific thread identifier in the request body or headers.
- **Resume:** When the user supplies `--resume-thread <thread_selector>`, the CLI MUST resolve the selector to a thread owned by the authenticated user (and within effective project scope when the gateway enforces it), MUST start the session in that thread, and MUST NOT create a new thread at startup.
  The observable outcome MUST be that the first and subsequent completions continue the selected conversation.

#### In-Session (`/thread new`)

- When the user types `/thread new` during an active chat session, the CLI MUST call `POST /v1/chat/threads` with the current effective session project context.
- After a successful create, the CLI MUST treat the session as switched to a fresh conversation for subsequent completions while keeping the `POST /v1/chat/completions` request shape OpenAI-compatible.
- The chat session MUST continue (no exit).

#### Additional `/thread` Actions

- `/thread list` SHOULD list threads for the current user and effective project context using `GET /v1/chat/threads`.
- `/thread switch <thread_selector>` SHOULD switch the active interactive session to the specified thread when the selector resolves to a thread owned by the authenticated user.
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
- When the connection is interrupted mid-stream, the CLI MUST attempt to auto-reconnect and reconcile the in-flight turn per [Connection recovery](cynork_tui.md#spec-cynai-client-cynorktui-connectionrecovery) (REQ-CLIENT-0213).

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
The list MUST include every slash command defined in [cynork_tui_slash_commands.md](cynork_tui_slash_commands.md) with at least the command name and a short description.
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
Exact command semantics and execution algorithms are defined in [cynork_tui_slash_commands.md](cynork_tui_slash_commands.md).

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

The canonical interactive slash-command catalog, syntax, argument rules, and execution semantics are defined in [cynork_tui_slash_commands.md](cynork_tui_slash_commands.md).
Interactive `cynork chat` MUST implement the same slash-command contract without user-visible divergence.
That canonical contract includes the local-session commands, thinking-visibility controls, and thread commands that used to be restated here.

#### Model Selection Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatModelSelection` <a id="spec-cynai-client-clichatmodelselection"></a>

##### Model Selection Traces To

- [REQ-CLIENT-0171](../requirements/client.md#req-client-0171)
- [REQ-CLIENT-0172](../requirements/client.md#req-client-0172)

Interactive `cynork chat` MUST implement the model-selection slash commands exactly as defined in [CYNAI.CLIENT.CynorkTui.ModelSlashCommands](cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-modelslashcommands).
Model selection affects only interactive chat completion requests and MUST NOT change any user preference or system setting.

#### Project Context Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatProjectContext` <a id="spec-cynai-client-clichatprojectcontext"></a>

##### Project Context Traces To

- [REQ-CLIENT-0173](../requirements/client.md#req-client-0173)

Project context affects only chat session association and any user-initiated task operations that accept a project context.
Project context MUST NOT be implicitly assigned.
When project context is set, the CLI MUST send it using the OpenAI-standard `OpenAI-Project` request header on subsequent `POST /v1/chat/completions` calls.
Interactive `cynork chat` MUST implement the project slash commands exactly as defined in [CYNAI.CLIENT.CynorkTui.ProjectSlashCommands](cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-projectslashcommands).
Implementations SHOULD reuse the same request-building and output code paths as `cynork project list`, `cynork project get`, and `cynork project set` so that slash commands remain a thin adapter.

#### Task Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashTask` <a id="spec-cynai-client-clichatslashtask"></a>

##### Task Slash Commands Traces To

- [REQ-CLIENT-0166](../requirements/client.md#req-client-0166)

Interactive `cynork chat` MUST implement the task slash commands exactly as defined in [CYNAI.CLIENT.CynorkTui.TaskSlashCommands](cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-taskslashcommands).
Each MUST call the same User API Gateway endpoints as the corresponding `cynork task` subcommand.
Output MUST be shown inline in the chat (pretty-printed per [Pretty-Printed JSON Output](cli_management_app_shell_output.md#spec-cynai-client-cliprettyprintjson) when the output is JSON).

##### Task Slash Commands Implementation Guidance

- To prevent behavioral drift, implementations SHOULD reuse the existing cynork subcommand request-building and output code paths for the corresponding operations (task, prefs, nodes, skills, status, auth).
  Slash commands should be a thin adapter that selects a subcommand and passes the parsed arguments through.

#### Status and Identity Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashStatus` <a id="spec-cynai-client-clichatslashstatus"></a>

##### Status and Identity Traces To

- [REQ-CLIENT-0167](../requirements/client.md#req-client-0167)

Interactive `cynork chat` MUST implement the status and auth slash commands exactly as defined in [CYNAI.CLIENT.CynorkTui.StatusSlashCommands](cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-statusslashcommands).

#### Node Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashNodes` <a id="spec-cynai-client-clichatslashnodes"></a>

##### Node Slash Commands Traces To

- [REQ-CLIENT-0168](../requirements/client.md#req-client-0168)

Interactive `cynork chat` MUST implement the node slash commands exactly as defined in [CYNAI.CLIENT.CynorkTui.NodeSlashCommands](cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-nodeslashcommands).

#### Preferences Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashPrefs` <a id="spec-cynai-client-clichatslashprefs"></a>

##### Preferences Slash Commands Traces To

- [REQ-CLIENT-0169](../requirements/client.md#req-client-0169)

Interactive `cynork chat` MUST implement the preference slash commands exactly as defined in [CYNAI.CLIENT.CynorkTui.PreferenceSlashCommands](cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-preferenceslashcommands).

#### Skills Slash Commands

- Spec ID: `CYNAI.CLIENT.CliChatSlashSkills` <a id="spec-cynai-client-clichatslashskills"></a>

Interactive `cynork chat` MUST implement the skills slash commands exactly as defined in [CYNAI.CLIENT.CynorkTui.SkillSlashCommands](cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-skillslashcommands).
Each MUST call the same User API Gateway endpoints as the corresponding `cynork skills` subcommand.

##### Skills Slash Commands Traces To

- [REQ-CLIENT-0170](../requirements/client.md#req-client-0170)

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
