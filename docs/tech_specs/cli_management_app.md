# CLI Management App

- [Document Overview](#document-overview)
- [Capability Parity With Admin Web Console](#capability-parity-with-admin-web-console)
- [Goals and Non-Goals](#goals-and-non-goals)
- [Security Model](#security-model)
  - [Security Model Applicable Requirements](#security-model-applicable-requirements)
- [Authentication and Configuration](#authentication-and-configuration)
  - [Configuration File and Location](#configuration-file-and-location)
  - [Token Resolution (Precedence)](#token-resolution-precedence)
  - [Credential Helper Protocol](#credential-helper-protocol)
  - [Authentication and Configuration Applicable Requirements](#authentication-and-configuration-applicable-requirements)
- [Command Surface](#command-surface)
  - [Global Flags](#global-flags)
  - [Shorthand Flag Aliases (Stable Contract)](#shorthand-flag-aliases-stable-contract)
  - [Output Rules](#output-rules)
  - [Exit Codes](#exit-codes)
  - [Required Top-Level Commands](#required-top-level-commands)
  - [Standard Error Behavior](#standard-error-behavior)
  - [`cynork version`](#cynork-version)
  - [`cynork status`](#cynork-status)
  - [`cynork auth` Commands](#cynork-auth-commands)
  - [Task Commands](#task-commands)
  - [Task Creation (Task Input and Attachments)](#task-creation-task-input-and-attachments)
  - [Chat Command](#chat-command)
- [Interactive Mode (REPL)](#interactive-mode-repl)
  - [Interactive Mode Applicable Requirements](#interactive-mode-applicable-requirements)
- [Credential Management](#credential-management)
  - [`cynork creds list`](#cynork-creds-list)
  - [`cynork creds get <credential_id>`](#cynork-creds-get-credential_id)
  - [`cynork creds create`](#cynork-creds-create)
  - [`cynork creds rotate <credential_id>`](#cynork-creds-rotate-credential_id)
  - [`cynork creds disable <credential_id>`](#cynork-creds-disable-credential_id)
- [Preferences Management](#preferences-management)
- [System Settings Management](#system-settings-management)
  - [`cynork prefs list`](#cynork-prefs-list)
  - [`cynork prefs get`](#cynork-prefs-get)
  - [`cynork prefs set`](#cynork-prefs-set)
  - [`cynork prefs delete`](#cynork-prefs-delete)
  - [`cynork prefs effective`](#cynork-prefs-effective)
- [Node Management](#node-management)
  - [`cynork nodes list`](#cynork-nodes-list)
  - [`cynork nodes get <node_id>`](#cynork-nodes-get-node_id)
  - [`cynork nodes enable <node_id>`](#cynork-nodes-enable-node_id)
  - [`cynork nodes disable <node_id>`](#cynork-nodes-disable-node_id)
  - [`cynork nodes drain <node_id>`](#cynork-nodes-drain-node_id)
  - [`cynork nodes refresh-config <node_id>`](#cynork-nodes-refresh-config-node_id)
  - [`cynork nodes prefetch-image <node_id> <image_ref>`](#cynork-nodes-prefetch-image-node_id-image_ref)
- [Skills Management](#skills-management)
  - [`cynork skills load <file.md>`](#cynork-skills-load-filemd)
  - [`cynork skills list`](#cynork-skills-list)
  - [`cynork skills get <skill_id>`](#cynork-skills-get-skill_id)
  - [`cynork skills update <skill_id> <file.md>`](#cynork-skills-update-skill_id-filemd)
  - [`cynork skills delete <skill_id>`](#cynork-skills-delete-skill_id)
- [Audit Commands](#audit-commands)
  - [`cynork audit list`](#cynork-audit-list)
  - [`cynork audit get <event_id>`](#cynork-audit-get-event_id)
- [Output and Scripting](#output-and-scripting)
  - [Output and Scripting Applicable Requirements](#output-and-scripting-applicable-requirements)
- [Implementation Specification (Go + Cobra)](#implementation-specification-go--cobra)
- [MVP Scope](#mvp-scope)

## Document Overview

- Spec ID: `CYNAI.CLIENT.Doc.CliManagementApp` <a id="spec-cynai-client-doc-climanagementapp"></a>

This document defines a CLI management application for CyNodeAI.
The CLI is intended to support the same administrative capabilities as the Admin Web Console.

Traces To:

- [REQ-CLIENT-0001](../requirements/client.md#req-client-0001)
- [REQ-CLIENT-0004](../requirements/client.md#req-client-0004)

Related documents

- Client requirements: [`docs/requirements/client.md`](../requirements/client.md)
- User API Gateway: [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md)
- Admin Web Console: [`docs/tech_specs/admin_web_console.md`](admin_web_console.md)
- User preferences: [`docs/tech_specs/user_preferences.md`](user_preferences.md)
- Skills storage and CRUD: [`docs/tech_specs/skills_storage_and_inference.md`](skills_storage_and_inference.md)
- Data REST API: [`docs/tech_specs/data_rest_api.md`](data_rest_api.md)
- API Egress (credentials): [`docs/tech_specs/api_egress_server.md`](api_egress_server.md)
- Git Egress (credentials): [`docs/tech_specs/git_egress_mcp.md`](git_egress_mcp.md)

## Capability Parity With Admin Web Console

- Spec ID: `CYNAI.CLIENT.CliCapabilityParity` <a id="spec-cynai-client-clicapabilityparity"></a>

Traces To:

- [REQ-CLIENT-0004](../requirements/client.md#req-client-0004)

The CLI and the [Admin Web Console](admin_web_console.md) MUST offer the same administrative capabilities.
When adding or changing a capability in this spec (for example a new command, credential workflow, preference scope, node action, or skill operation), the Admin Web Console spec and implementation MUST be updated to match, and vice versa.
Use the same gateway APIs and the same authorization and auditing rules for both clients.

## Goals and Non-Goals

Goals

- Provide a fast, scriptable admin interface for credentials, user task-execution preferences, and node management.
- Operate against the User API Gateway so the CLI does not require direct database access.
- Support secure secret input and rotation without echoing secrets to terminal logs.

Non-goals

- The CLI is not an agent tool and is not exposed to sandboxes.
- The CLI is not intended to perform worker-node internal operations directly.

## Security Model

The following requirements apply.

### Security Model Applicable Requirements

- Spec ID: `CYNAI.CLIENT.CliSecurityModel` <a id="spec-cynai-client-clisecurity"></a>

Traces To:

- [REQ-CLIENT-0100](../requirements/client.md#req-client-0100)
- [REQ-CLIENT-0101](../requirements/client.md#req-client-0101)
- [REQ-CLIENT-0102](../requirements/client.md#req-client-0102)
- [REQ-CLIENT-0103](../requirements/client.md#req-client-0103)
- [REQ-CLIENT-0104](../requirements/client.md#req-client-0104)

## Authentication and Configuration

The CLI is a user client.
It authenticates to the User API Gateway and is authorized by the gateway.

### Configuration File and Location

- Spec ID: `CYNAI.CLIENT.CliConfigFileLocation` <a id="spec-cynai-client-cliconfigfilelocation"></a>

Traces To:

- [REQ-CLIENT-0149](../requirements/client.md#req-client-0149)
- [REQ-CLIENT-0150](../requirements/client.md#req-client-0150)

#### Config File Path

- Default path MUST be resolved as follows: if `XDG_CONFIG_HOME` is set, use `$XDG_CONFIG_HOME/cynork/config.yaml`; otherwise use `~/.config/cynork/config.yaml`.
- The `--config` flag MUST override the default path when provided.
- The CLI MUST create the config directory with mode `0700` when writing (e.g. on `auth login`); it MUST NOT create the file or directory on read if missing.

#### Config File Format (YAML)

- The file MUST be valid YAML.
- Supported top-level keys:
  - `gateway_url` (string, optional): base URL of the User API Gateway (e.g. `http://localhost:8080`).
  - `token` (string, optional): bearer token for gateway auth; MAY be omitted when using a credential helper.
  - `credential_helper` (string, optional): command or helper name to obtain the token (see [Credential Helper Protocol](#credential-helper-protocol)).
- Unknown keys MAY be ignored; the CLI MUST NOT fail load solely due to unknown keys.
- When writing the config file (e.g. after login), the CLI MUST use file mode `0600` and MUST NOT log the contents or token value.

#### Environment Overrides

- `CYNORK_GATEWAY_URL`: if set, overrides `gateway_url` from the config file after load.
- `CYNORK_TOKEN`: if set, overrides the resolved token (see [Token Resolution (Precedence)](#token-resolution-precedence)) for the session; use for CI or ephemeral runs.
- Overrides apply at config load time; the effective gateway URL and token used for requests MUST be the result of applying overrides to the loaded config and resolved token.

#### Default Gateway URL

- When `gateway_url` is empty after load and env override, the CLI MUST use the default `http://localhost:8080` (or a build-time constant matching the orchestrator default).
- See [Ports and endpoints](ports_and_endpoints.md#cli-cynork) for the consolidated default and overrides.

#### Session Persistence (Reliability)

- Spec ID: `CYNAI.CLIENT.CliSessionPersistence` <a id="spec-cynai-client-clisessionpersistence"></a>
- Traces To: [REQ-CLIENT-0150](../requirements/client.md#req-client-0150)
- When writing the config file (e.g. after `auth login` or `auth logout`), the CLI MUST write atomically (e.g. write to a temp file in the same directory then rename to the final path) so that a crash or interrupt does not leave a partial or corrupt file; subsequent invocations MUST see either the previous config or a complete new one.
- When the default config path cannot be resolved (e.g. home directory unavailable and no `--config` given), `auth login` and `auth logout` MUST fail with a clear error and MUST NOT proceed with an empty path.

### Token Resolution (Precedence)

- Spec ID: `CYNAI.CLIENT.CliTokenResolution` <a id="spec-cynai-client-clitokenresolution"></a>

Traces To:

- [REQ-CLIENT-0105](../requirements/client.md#req-client-0105)
- [REQ-CLIENT-0106](../requirements/client.md#req-client-0106)
- [REQ-CLIENT-0149](../requirements/client.md#req-client-0149)

The CLI MUST resolve the bearer token used for gateway requests by following this order; the first non-empty value wins.

1. **Environment.** If `CYNORK_TOKEN` is set and non-empty, use it and do not read config file `token` or credential helper.
2. **Config file.** If the config file was loaded and contains a non-empty `token` field, use it; skip step 3.
3. **Credential helper.** If `credential_helper` is set in the loaded config, invoke the helper (see [Credential Helper Protocol](#credential-helper-protocol)) with action `get`; if the helper returns a non-empty secret, use it.
4. **None.** If no token was obtained, the effective token is empty; commands that require auth MUST fail with a clear error (e.g. "not logged in: run 'cynork auth login' or set CYNORK_TOKEN") and MUST NOT send a request with an empty or missing Authorization header.

Implementers MUST perform token resolution once per process after config load (or when config is reloaded) and reuse the resolved value for all gateway calls in that run.
The CLI MUST NOT log or print the resolved token.

### Credential Helper Protocol

- Spec ID: `CYNAI.CLIENT.CliCredentialHelperProtocol` <a id="spec-cynai-client-clicredentialhelperprotocol"></a>

Traces To:

- [REQ-CLIENT-0149](../requirements/client.md#req-client-0149)

When the config contains a non-empty `credential_helper`, the CLI SHOULD use it to get and optionally store the token so the token is not stored in plaintext in the config file.

Helper invocation

- The value of `credential_helper` is either a short name (e.g. `pass`, `keychain`) or an absolute path to an executable (e.g. `/usr/bin/cynork-credential-helper`).
- The CLI invokes the helper as a subprocess with no arguments; communication is via JSON on stdin/stdout.
- Stdin: one JSON object per line (newline-delimited JSON).
  The CLI sends a single object: `{"action":"get"}` to retrieve the token or `{"action":"store","secret":"<token>"}` to store it after login.
- Stdout: the helper MUST respond with one JSON object per line.
  For `get`: `{"secret":"<token>"}` or `{"error":"<message>"}`.
  For `store`: `{}` on success or `{"error":"<message>"}` on failure.
  Empty secret is treated as no token.
- The CLI MUST NOT pass the token on the command line or in environment variables; only via stdin for `store`.
- Helper process MUST be invoked with minimal environment (e.g. no inherited env that could leak secrets); timeout SHOULD be applied (e.g. 10 seconds).

Well-known helper names (optional)

- Implementers MAY map short names to platform stores: e.g. `keychain` to macOS Keychain (service `cynork`, account `token`), or `pass` to the pass utility (store path configurable or default e.g. `cynork/gateway-token`).
  If a short name is unknown, the CLI MAY treat it as a path and exec it only if it looks like a path (e.g. contains `/` or starts with `.`).

Store on login

- When `cynork auth login` succeeds and `credential_helper` is set, the CLI SHOULD call the helper with `{"action":"store","secret":"<obtained_token>"}` so the token is persisted in the store.
- The CLI MAY also write the token to the config file for backward compatibility, or MAY omit writing `token` to the config file when a credential helper is configured (so the config file stays free of plaintext tokens).

### Authentication and Configuration Applicable Requirements

- Spec ID: `CYNAI.CLIENT.CliAuthConfig` <a id="spec-cynai-client-cliauth"></a>

Traces To:

- [REQ-CLIENT-0105](../requirements/client.md#req-client-0105)
- [REQ-CLIENT-0106](../requirements/client.md#req-client-0106)
- [REQ-CLIENT-0107](../requirements/client.md#req-client-0107)
- [REQ-CLIENT-0149](../requirements/client.md#req-client-0149)

## Command Surface

- Spec ID: `CYNAI.CLIENT.CliCommandSurface` <a id="spec-cynai-client-clicommandsurface"></a>

Traces To:

- [REQ-CLIENT-0101](../requirements/client.md#req-client-0101)
- [REQ-CLIENT-0155](../requirements/client.md#req-client-0155)
- [REQ-CLIENT-0156](../requirements/client.md#req-client-0156)
- [REQ-CLIENT-0158](../requirements/client.md#req-client-0158)

The CLI MUST be implemented as a single binary named `cynork` with subcommands.
All commands that require gateway auth MUST fail immediately with a non-zero exit code and a clear message if the resolved token is empty (see [Token Resolution (Precedence)](#token-resolution-precedence)).

### Global Flags

The following flags MUST be supported on the root command and MUST apply to all subcommands.

- `-c, --config` (string): path to config file; overrides default config path.
  Optional to specify; when omitted, default path is used.
- `-o, --output` (string): output format.
  Allowed values are `table` and `json`.
  Default is `table`.
- `-q, --quiet` (bool): suppress non-essential output.
  Errors MUST still be printed to stderr.
- `--no-color` (bool): disable colored output.

### Shorthand Flag Aliases (Stable Contract)

The CLI MUST support the following short flags as exact aliases of the corresponding long flags.
These shorthands MUST be supported anywhere the corresponding long flag is supported.

- `-c` => `--config`
- `-o` => `--output`
- `-q` => `--quiet`
- `-y` => `--yes`
- `-l` => `--limit`
- `-p` => `--prompt`
- `-t` => `--task`
- `-f` => `--task-file`
- `-s` => `--script`
- `-a` => `--attach`
- `-w` => `--wait`
- `-F` => `--follow`

### Output Rules

When `--output json` is selected, the CLI MUST emit exactly one JSON value to stdout.
The CLI MUST NOT write any other bytes to stdout in JSON mode.
All warnings, hints, progress messages, and errors MUST be written to stderr in JSON mode.

When `--output table` is selected, the CLI SHOULD write human-readable output to stdout.
The CLI MAY write errors to stderr in table mode.

### Exit Codes

The CLI MUST return deterministic exit codes.
If multiple failure categories apply, the CLI MUST return the exit code for the earliest failing check in this order: usage validation, auth validation, gateway request, response handling.

- Exit code 0.
  The command succeeded.
- Exit code 2.
  Usage error.
  This includes unknown flags, missing required flags, invalid flag values, missing required positional arguments, and mutually exclusive flags used together.
- Exit code 3.
  Authentication or authorization error.
  This includes missing token, gateway 401, and gateway 403.
- Exit code 4.
  Not found.
  This includes gateway 404 for a requested resource.
- Exit code 5.
  Conflict.
  This includes gateway 409 or an idempotency conflict where the server rejects the request.
- Exit code 6.
  Validation error.
  This includes gateway 400 and 422 responses where the request payload is invalid.
- Exit code 7.
  Gateway or network error.
  This includes network failures, timeouts, and gateway 5xx responses.
- Exit code 8.
  Internal CLI error.
  This includes unexpected failures before a gateway request can be made that are not usage or auth errors.

### Required Top-Level Commands

The CLI MUST implement the following top-level commands and subcommands.
All subcommands that call the gateway MUST use the resolved gateway URL and resolved token from config load.

- `cynork version`: print version string (e.g. from build); MUST NOT require auth.
- `cynork status`: report gateway reachability and optionally auth status; MAY require auth for full status.
- `cynork auth login`: interactive or flag-based login; POST to gateway login endpoint; MUST support writing token to config and/or credential helper; MUST NOT echo password.
- `cynork auth logout`: clear token from config file and optionally from credential helper; MUST NOT require gateway call.
- `cynork auth whoami`: call gateway with current token; MUST require auth.
- `cynork task ...`: create tasks, list tasks, get task status, watch task status, cancel tasks, and retrieve task results and artifacts.
- `cynork chat`: start an interactive chat session with the Project Manager (PM) model; see [Chat Command](#chat-command).
- `cynork creds ...`: see [Credential Management](#credential-management); MUST use gateway credential endpoints.
- `cynork prefs ...`: see [Preferences Management](#preferences-management).
- `cynork nodes ...`: see [Node Management](#node-management).
- `cynork skills ...`: full CRUD via gateway; see [Skill Management CRUD (Web and CLI)](skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud) and [Skills Management](#skills-management).
  - Create: `cynork skills load <file.md>` (required file path; optional `--name`, `--scope`).
  - List: `cynork skills list` (optional `--scope`, `--owner`).
  - Get: `cynork skills get <skill_id>`.
  - Update: `cynork skills update <skill_id> <file.md>` (optional `--name`, `--scope`).
  - Delete: `cynork skills delete <skill_id>`.
- `cynork audit ...`: query audit logs; MUST require auth.

### Standard Error Behavior

On gateway 401 or 403, the CLI MUST print a clear error to stderr and exit with code 3.
On gateway 404, the CLI MUST print a clear error to stderr and exit with code 4.
On gateway 409, the CLI MUST print a clear error to stderr and exit with code 5.
On gateway 400 or 422, the CLI MUST print a clear error to stderr and exit with code 6.
On gateway 5xx and on network failure, the CLI MUST exit with code 7.
On invalid config file (syntax error), the CLI MUST exit with code 2 before running any command.

### `cynork version`

Invocation

- `cynork version`.

Behavior

- The CLI MUST print build and version metadata.
- The CLI MUST NOT require auth.

Output

- Table mode MUST print exactly one line containing `version=<string>`.
- JSON mode MUST print `{"version":"<string>"}`.

### `cynork status`

Invocation

- `cynork status`.

Behavior

- The CLI MUST call the gateway health endpoint.
- The CLI MUST treat an HTTP 200 response body containing `ok` as healthy.

Output

- Table mode MUST print exactly one line containing `ok`.
- JSON mode MUST print `{"gateway":"ok"}`.

Exit behavior

- If the gateway health check fails, the CLI MUST exit with code 7.

### `cynork auth` Commands

All `auth` subcommands MUST use the gateway auth endpoints.

#### `cynork auth login`

Invocation

- `cynork auth login`.

Optional flags

- `--handle <handle>`.
- `--password-stdin`.

Behavior

- If `--handle` is not provided, the CLI MUST prompt `Handle:` on stderr and read one line from stdin.
- If `--password-stdin` is set, the CLI MUST require `--handle` to be provided.
  This is a usage error and MUST return exit code 2.
- If `--password-stdin` is set, the CLI MUST read the password from stdin as UTF-8 text.
  The CLI MUST trim exactly one trailing newline if present.
- If `--password-stdin` is not set, the CLI MUST prompt `Password:` on stderr and read the password without echo.
- The CLI MUST NOT accept a `--password <value>` flag.
- The CLI MUST NOT print the password or token.
- On success, the CLI MUST persist the token according to the config and credential helper rules in this spec.

Output

- Table mode MUST print exactly one line containing `logged_in=true` and `handle=<handle>`.
- JSON mode MUST print `{"logged_in":true,"handle":"<handle>"}`.

#### `cynork auth logout`

Invocation

- `cynork auth logout`.

Behavior

- The CLI MUST remove the token from the config file and MUST clear it from the credential helper if configured.
- The CLI MUST NOT require a gateway call.

Output

- Table mode MUST print exactly one line containing `logged_out=true`.
- JSON mode MUST print `{"logged_out":true}`.

#### `cynork auth whoami`

Invocation

- `cynork auth whoami`.

Output

- Table mode MUST print exactly one line containing `id=<id>` and `handle=<handle>`.
- JSON mode MUST print `{"id":"<id>","handle":"<handle>"}`.

### Task Commands

The CLI MUST implement the following `task` subcommands.
All `task` subcommands MUST require auth.

Task identifier

- Where a task is referenced (e.g. `task get`, `task result`, `task cancel`, `task logs`, `task artifacts list`, `task artifacts get`), the CLI MUST accept either the task UUID or the human-readable task name (see [Project Manager Agent - Task Naming](project_manager_agent.md#task-naming)).
- Task list and task get output MUST include the task name when the system provides one (e.g. in table mode as `task_name=<name>` and in JSON as `task_name`).
  For task name format and semantics, see [Project Manager Agent - Task Naming](project_manager_agent.md#task-naming).

Task status enum

- `queued`
- `running`
- `completed`
- `failed`
- `canceled`

#### `cynork task create`

Invocation

- `cynork task create` followed by exactly one task input mode.

Task input modes (exactly one MUST be provided)

- `-t, --task <string>` or `-p, --prompt <string>`.
- `-f, --task-file <path>`.
- `-s, --script <path>`.
- `--command <string>` repeated one or more times.
- `--commands-file <path>`.

Attachment flags (optional)

- `-a, --attach <path>` repeated zero or more times.

Optional flags

- `--name <string>`.
  Suggested human-readable name for the task.
  When provided, the CLI MUST include it in the task create request.
  The orchestrator accepts the value, normalizes it per [Task Naming](project_manager_agent.md#task-naming), and ensures uniqueness (e.g. appends a number) when the normalized name already exists in scope.
- `--result`.
  Default is false.
  When set, after creating the task the CLI MUST poll the gateway for the task result until the task reaches a terminal status (`completed`, `failed`, or `canceled`), then MUST print the result in the same format as `cynork task result`.
  If the user interrupts (e.g. Ctrl+C) before the task reaches a terminal status, the CLI MUST exit without printing the result.

Behavior

- The CLI MUST reject invocations that provide zero or more than one task input mode.
  This is a usage error and MUST return exit code 2.
- If `--task` or `--prompt` is provided, the system MUST interpret the task input as plain text or Markdown.
- If `--task-file` is provided, the CLI MUST read the file contents and send it as task input.
  The file contents MUST be treated as plain text or Markdown.
- If `--script` is provided, the CLI MUST read the script file contents and request script execution mode.
  The system MUST run the script in the sandbox.
- If `--command` is provided, the CLI MUST preserve the order of occurrences and MUST send the ordered list of commands.
  The system MUST run the commands in the sandbox in that order.
- If `--commands-file` is provided, the CLI MUST read the file and split it by `\n`.
  Empty lines and lines that are only whitespace MUST be ignored.
  Remaining lines are commands and MUST be run in file order.
- If any `--attach` paths are provided, the CLI MUST include them as attachments in the task create request.

Path and file validation

- For `--task-file`, `--script`, `--commands-file`, and each `--attach`, the CLI MUST validate that the path exists, is a regular file, and is readable by the current user.
  If any path fails validation, the CLI MUST exit with code 2 before making a gateway request.
- The CLI MUST reject directories and symlinks for these file inputs.
  This is a usage error and MUST return exit code 2.

Size limits

- `--task-file` contents MUST be <= 1 MiB.
- `--script` contents MUST be <= 256 KiB.
- `--commands-file` contents MUST be <= 64 KiB.
- Each `--attach` file MUST be <= 10 MiB.
- The number of `--attach` occurrences MUST be <= 16.
- If a limit is exceeded, the CLI MUST exit with code 2 before making a gateway request.

Output

- When `--result` is not set: table mode MUST print a single line containing `task_id=<id>` and when the system provides a task name, `task_name=<name>`; JSON mode MUST print at least `task_id`, and when provided, `task_name`.
- When `--result` is set: after the task reaches a terminal status, the CLI MUST print the result in the same format as `cynork task result` (task_id, task_name when provided, status, jobs and their results).

#### `cynork task list`

Invocation

- `cynork task list`.

Optional flags

- `--status <status>`.
  Allowed values are `queued`, `running`, `completed`, `failed`, and `canceled`.
- `-l, --limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

Output

- Table mode MUST print one task per line.
  Table mode MUST include at least `task_id=<id>`, `status=<status>`, and when the system provides a task name, `task_name=<name>`.
- JSON mode MUST print `{"tasks":[...],"next_cursor":"<opaque>"}`.
  Each task object MUST include at least `task_id`, `status`, and when provided, `task_name`.

#### `cynork task get <task_id>`

Invocation

- `cynork task get <task_id>`, where `<task_id>` is the task UUID or the human-readable task name.

Output

- Table mode MUST print exactly one line and MUST include at least `task_id=<id>`, `status=<status>`, and when provided, `task_name=<name>`.
- JSON mode MUST print a single JSON object representing the task.
  The JSON object MUST include at least `task_id`, `status`, and when provided, `task_name`.

#### `cynork task cancel <task_id>`

Invocation

- `cynork task cancel <task_id>`.

Optional flags

- `-y, --yes`.

Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation.
- The confirmation prompt MUST be `Cancel task <task_id>? [y/N]`.
- If the user does not enter `y` or `Y`, the CLI MUST exit with code 0 and MUST NOT make a gateway request.
- On success, table mode MUST print exactly one line including `task_id=<id>`, `canceled=true`, and when the system provides a task name, `task_name=<name>`.
- On success, JSON mode MUST print at least `task_id`, `canceled`, and when provided, `task_name`.

#### `cynork task result <task_id>`

Invocation

- `cynork task result <task_id>`.

Optional flags

- `-w, --wait`.
  Default is false.

Output

- If `--wait` is set, the CLI MUST poll the gateway until the task reaches a terminal status.
  Terminal statuses are `completed`, `failed`, and `canceled`.
- Table mode MUST print exactly one line and MUST include at least `task_id=<id>`, `status=<status>`, and when the system provides a task name, `task_name=<name>`.
- If the task is in a terminal status, table mode MUST also include `stdout=<...>` and `stderr=<...>`.
- JSON mode MUST print a single JSON object with at least `task_id`, `status`, and when provided, `task_name`; and when terminal, `stdout` and `stderr`.

#### `cynork task watch <task_id>`

Invocation

- `cynork task watch <task_id>`.

Behavior

- The CLI MUST poll the gateway for the task result at a fixed interval and redraw the output, similar to the Linux `watch(1)` command.
- The CLI MUST use the same output format as `cynork task result` (task_id, task_name when provided, status, jobs and results).
- When stdout is a terminal and `--no-clear` is not set, the CLI MUST clear the screen before each redraw so the display updates in place.
- The CLI MUST exit with code 0 when the task reaches a terminal status (`completed`, `failed`, or `canceled`), or when the user interrupts (e.g. Ctrl+C).

Optional flags

- `-n, --interval <duration>`.
  Poll interval (e.g. `2s`, `500ms`).
  Default is `2s`.
  Minimum is `1s`.
- `--no-clear`.
  Do not clear the screen between polls; output scrolls instead.
  Useful when stdout is not a terminal or when capturing output.

#### `cynork task logs <task_id>`

Invocation

- `cynork task logs <task_id>`.

Optional flags

- `--stream <stream>`.
  Allowed values are `stdout`, `stderr`, and `all`.
  Default is `all`.
- `-F, --follow`.
  Default is false.

Output

- Table mode MUST print raw log lines to stdout.
- JSON mode MUST print an object with at least `task_id`, `stream`, `lines`; and when the system provides a task name, `task_name`.

#### `cynork task artifacts list <task_id>`

Invocation

- `cynork task artifacts list <task_id>`.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `artifact_id`, `name`, `content_type`, `size_bytes`.
- Table mode MUST then print one row per artifact.
- JSON mode MUST print an object with at least `task_id`, `artifacts`; and when the system provides a task name, `task_name`.
  Each artifact object MUST include at least `artifact_id`, `name`, and `size_bytes`.

#### `cynork task artifacts get <task_id> <artifact_id> --out <path>`

Invocation

- `cynork task artifacts get <task_id> <artifact_id> --out <path>`.

Required flags

- `--out <path>`.

Behavior

- The CLI MUST write the artifact bytes to the `--out` path.
- The CLI MUST create parent directories if needed.
- If the output file already exists, the CLI MUST refuse to overwrite it unless `--force` is provided.

Optional flags

- `--force`.

Output

- Table mode MUST print exactly one line containing `saved=true` and `path=<path>`.
- JSON mode MUST print `{"saved":true,"path":"<path>"}`.

### Task Creation (Task Input and Attachments)

- Spec ID: `CYNAI.CLIENT.CliTaskCreatePrompt` <a id="spec-cynai-client-clitaskcreateprompt"></a>

Traces To:

- [REQ-ORCHES-0121](../requirements/orches.md#req-orches-0121)
- [REQ-ORCHES-0125](../requirements/orches.md#req-orches-0125)
- [REQ-ORCHES-0126](../requirements/orches.md#req-orches-0126)
- [REQ-ORCHES-0127](../requirements/orches.md#req-orches-0127)
- [REQ-CLIENT-0151](../requirements/client.md#req-client-0151)
- [REQ-CLIENT-0153](../requirements/client.md#req-client-0153)
- [REQ-CLIENT-0157](../requirements/client.md#req-client-0157)

Task create MUST accept the task as **inline text** (e.g. `--prompt "..."` or `--task "..."`) or from a **file** (e.g. `--task-file <path>`) containing plain text or Markdown.
Exactly one task input mode MUST be supplied per `cynork task create` invocation.
The CLI MUST support attachments via repeatable `--attach <path>`.
The CLI MUST support running a script via `--script <path>`.
The CLI MUST support running a short series of commands via repeatable `--command <string>` or via `--commands-file <path>`.
The user task text MUST NOT be executed as a literal shell command unless the user explicitly selects `--script`, `--command`, or `--commands-file`.

### Chat Command

- Spec ID: `CYNAI.CLIENT.CliChat` <a id="spec-cynai-client-clichat"></a>

Traces To:

- [REQ-CLIENT-0161](../requirements/client.md#req-client-0161)
- [REQ-CLIENT-0162](../requirements/client.md#req-client-0162)

The CLI MUST provide a top-level `chat` command that starts an interactive chat session with the Project Manager (PM) model.
The session MUST use the same User API Gateway and token resolution as other commands and MUST require auth.

#### `cynork chat` Invocation

- `cynork chat`.

Optional flags

- `-c, --config` (string): path to config file (global).
- `--no-color` (bool): disable colored output (global).
- `--plain` (bool, optional): print model responses as raw text without Markdown formatting; for scripting or piping.

#### `cynork chat` Behavior

- The CLI MUST resolve the gateway URL and token using the same config load and token resolution as other commands.
  If the resolved token is empty, the CLI MUST exit with code 3 and MUST NOT start a chat session.
- The CLI MUST open an interactive loop: read a line of user input, send it to the gateway as a chat message to the PM model, receive the model response, and print the response to the user.
  This loop MUST continue until the user exits (see below).
- The CLI MUST support a session-exit control (e.g. `/exit`, `/quit`, or EOF) so the user can leave the chat without sending a message.
  The exact exit control MUST be defined in this spec or in a linked spec; implementations MUST support at least one of `/exit`, `/quit`, or EOF.
- Chat input and model output MUST NOT be recorded in shell history or in any persistent history that could expose secrets; the same rules as interactive mode (REQ-CLIENT-0140) apply.
- All communication with the PM model MUST go through the User API Gateway; the CLI MUST NOT connect directly to inference or to the database.

#### `cynork chat` Response Output (Pretty Formatting)

- When `--plain` is not set, the CLI MUST render model responses with pretty-formatted output: interpret Markdown in the response and display it in a human-readable way in the terminal.
- The CLI MUST support at least: headings, lists (ordered and unordered), code blocks (with optional syntax highlighting), inline code, emphasis (bold/italic), and links.
  Display MAY use terminal styling (e.g. indentation, colors, or borders) so that structure is clear without raw Markdown syntax.
- The CLI MUST honor `--no-color` for chat output (no colors or minimal styling when set).
- When `--plain` is set, the CLI MUST print the raw response text with no Markdown parsing or styling, so that output is suitable for piping or scripting.

#### `cynork chat` Error Conditions

- Missing or invalid token: exit code 3.
- Gateway unreachable or 5xx: exit code 7.
- Gateway 4xx (e.g. 429, 403): exit code per [Exit Codes](#exit-codes) (e.g. 3 for 403, 6 for 422).

## Interactive Mode (REPL)

The CLI SHOULD provide an interactive mode that exposes the same command surface as the non-interactive CLI,
and provides tab completion to accelerate discovery and reduce typing errors.

Entrypoint and invocation

- Command MUST be `cynork shell`; optional `-c "command"` for single-command mode (then exit with that command's exit code).
- Config and token resolution MUST run the same way as for non-interactive; the same resolved gateway URL and token MUST be used for all commands entered in the shell.

### Interactive Mode Applicable Requirements

- Spec ID: `CYNAI.CLIENT.CliInteractiveMode` <a id="spec-cynai-client-cliinteractivemode"></a>

Traces To:

- [REQ-CLIENT-0136](../requirements/client.md#req-client-0136)
- [REQ-CLIENT-0137](../requirements/client.md#req-client-0137)
- [REQ-CLIENT-0138](../requirements/client.md#req-client-0138)
- [REQ-CLIENT-0139](../requirements/client.md#req-client-0139)
- [REQ-CLIENT-0140](../requirements/client.md#req-client-0140)
- [REQ-CLIENT-0141](../requirements/client.md#req-client-0141)
- [REQ-CLIENT-0142](../requirements/client.md#req-client-0142)
- [REQ-CLIENT-0159](../requirements/client.md#req-client-0159)

Required behaviors

- The prompt MUST show the active gateway URL (or a short label) and SHOULD show auth identity when available (e.g. handle from whoami).
- Commands entered in the shell MUST behave identically to non-interactive invocation: same flags, same `--output table|json`, same exit codes.
- Tab completion MUST be provided for commands, subcommands, and known flag values; MUST NOT suggest or expose secret values (REQ-CLIENT-0142).
- Tab completion MUST be provided for task names when a task identifier is expected (e.g. after `task get`, `task result`, `task watch`, `task cancel`, `task logs`, `task artifacts list`, `task artifacts get`); completion MAY be driven by gateway-backed list of task names available to the user (REQ-CLIENT-0159).
- History (if implemented) MUST NOT record lines that contain secrets or that were entered during secret prompts; secret prompts MUST bypass history.
- When invoked as `cynork shell -c "..."`, the CLI MUST run the given command once and exit with that command's exit code (zero or non-zero).

## Credential Management

- Spec ID: `CYNAI.CLIENT.CliCredentialManagement` <a id="spec-cynai-client-clicredential"></a>

Traces To:

- [REQ-CLIENT-0116](../requirements/client.md#req-client-0116)
- [REQ-CLIENT-0117](../requirements/client.md#req-client-0117)
- [REQ-CLIENT-0118](../requirements/client.md#req-client-0118)
- [REQ-CLIENT-0119](../requirements/client.md#req-client-0119)
- [REQ-CLIENT-0120](../requirements/client.md#req-client-0120)

The CLI MUST support credential workflows for API Egress and Git Egress using the gateway endpoints defined in [API Egress Server - Admin API (Gateway Endpoints)](api_egress_server.md#admin-api-gateway-endpoints).
Responses MUST return metadata only; the CLI MUST NOT print or log secret values.

### `cynork creds list`

Invocation

- `cynork creds list`.

Optional flags

- `--provider <provider>`.
- `--owner-type <owner_type>`.
  Allowed values are `user` and `group`.
- `--owner-id <uuid>`.
- `--active <bool>`.
  Allowed values are `true` and `false`.
- `-l, --limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `credential_id`, `provider`, `name`, `owner_type`, `owner_id`, `active`, `created_at`, `updated_at`.
- Table mode MUST then print one row per credential with the same tab-separated column order.
- JSON mode MUST print `{"credentials":[...],"next_cursor":"<opaque>"}`.
  Each credential object MUST include `credential_id`, `provider`, `name`, `owner_type`, `owner_id`, `active`, `created_at`, and `updated_at`.

### `cynork creds get <credential_id>`

Invocation

- `cynork creds get <credential_id>`.

Output

- Table mode MUST print exactly one line containing at least `credential_id=<id>` and `provider=<provider>` and `name=<name>` and `active=<bool>`.
- JSON mode MUST print a single JSON object containing at least `credential_id`, `provider`, `name`, and `active`.

### `cynork creds create`

Invocation

- `cynork creds create` with required flags and exactly one secret input method.

Required flags

- `--provider <provider>`.
- `--name <name>`.

Optional flags

- `--owner-type <owner_type>`.
  Allowed values are `user` and `group`.
  Default is `user`.
- `--owner-id <uuid>`.
  If `--owner-type user` and `--owner-id` is omitted, the CLI MUST default the owner to the authenticated user.

Secret input methods (exactly one MUST be used)

- `--secret-from-stdin`.
- `--secret-file <path>`.
- Interactive secret prompt.

Secret handling

- If `--secret-from-stdin` is set, the CLI MUST read the secret from stdin as UTF-8 text.
  The CLI MUST trim exactly one trailing newline if present.
- If `--secret-file` is set, the CLI MUST read the secret from the specified file.
  The CLI MUST trim exactly one trailing newline if present.
- If neither `--secret-from-stdin` nor `--secret-file` is set, the CLI MUST prompt `Secret:` on stderr and read the secret without echo.
- The CLI MUST reject invocations that specify more than one secret input method.
  This is a usage error and MUST return exit code 2.
- The CLI MUST NOT print or log the secret.

Output

- Table mode MUST print exactly one line containing `credential_id=<id>`.
- JSON mode MUST print `{"credential_id":"<id>"}`.

### `cynork creds rotate <credential_id>`

Invocation

- `cynork creds rotate <credential_id>` with exactly one secret input method.

Secret input methods and secret handling

- Secret input methods and secret handling MUST match `cynork creds create`.

Output

- Table mode MUST print exactly one line containing `credential_id=<id> rotated=true`.
- JSON mode MUST print `{"credential_id":"<id>","rotated":true}`.

### `cynork creds disable <credential_id>`

Invocation

- `cynork creds disable <credential_id>`.

Optional flags

- `-y, --yes`.

Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation.
- The confirmation prompt MUST be `Disable credential <credential_id>? [y/N]`.
- If the user does not enter `y` or `Y`, the CLI MUST exit with code 0 and MUST NOT make a gateway request.

Output

- Table mode MUST print exactly one line containing `credential_id=<id> disabled=true`.
- JSON mode MUST print `{"credential_id":"<id>","disabled":true}`.

## Preferences Management

- Spec ID: `CYNAI.CLIENT.CliPreferencesManagement` <a id="spec-cynai-client-clipreferences"></a>

Traces To:

- [REQ-CLIENT-0121](../requirements/client.md#req-client-0121)
- [REQ-CLIENT-0122](../requirements/client.md#req-client-0122)
- [REQ-CLIENT-0123](../requirements/client.md#req-client-0123)
- [REQ-CLIENT-0124](../requirements/client.md#req-client-0124)

The CLI MUST support reading and writing preferences via the Data REST API; scope and key semantics are defined in [User preferences](user_preferences.md).

All preference commands MUST require auth.

Recommended keys to support (MVP)

- `output.summary_style` (string)
  - examples: concise, detailed
- `definition_of_done.required_checks` (array)
  - examples: lint, unit_tests, docs_updated
- `language.preferred` (string)
  - examples: en, en-US
- `code.language.rank_ordered` (array)
  - Rank-ordered code language choices with optional context (project kind, task kind).
- `code.language.disallowed` (array)
  - Globally disallowed languages.
- `code.language.disallowed_by_project_kind` (object)
  - Per-project-kind disallowed languages.
- `code.language.disallowed_by_task_kind` (object)
  - Per-task-kind disallowed languages.
- `standards.markdown.line_length` (number)

Scope type enum

- `system`
- `user`
- `project`
- `task`

## System Settings Management

- Spec ID: `CYNAI.CLIENT.CliSystemSettingsManagement` <a id="spec-cynai-client-clisystemsettings"></a>

Traces To:

- [REQ-CLIENT-0160](../requirements/client.md#req-client-0160)

The CLI MUST support reading and writing system settings via the User API Gateway.
System settings are not user preferences and are not managed via `cynork prefs`.
User preferences are managed via `cynork prefs`; see [User preferences](user_preferences.md).

Recommended keys to support (MVP)

Semantics: [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#project-manager-model-startup-selection-and-warmup).

- `agents.project_manager.model.local_default_ollama_model` (string)
- `agents.project_manager.model.selection.execution_mode` (string)
- `agents.project_manager.model.selection.mode` (string)
- `agents.project_manager.model.selection.prefer_orchestrator_host` (boolean)

Command group

- `cynork settings ...`

### `cynork prefs list`

Invocation

- `cynork prefs list --scope-type <scope_type> [--scope-id <id>]`.

Required flags

- `--scope-type <scope_type>`.
  Allowed values are `system`, `user`, `project`, and `task`.

Optional flags

- `--scope-id <id>`.
  If `--scope-type user` and `--scope-id` is omitted, the CLI MUST default the scope id to the authenticated user id.
  If `--scope-type project` or `--scope-type task`, `--scope-id` is required.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `key`, `value_json`, `updated_at`.
- Table mode MUST then print one row per preference.
- JSON mode MUST print `{"preferences":[... ]}`.
  Each preference object MUST include `key` and `value`.

### `cynork prefs get`

Invocation

- `cynork prefs get --scope-type <scope_type> [--scope-id <id>] --key <key>`.

Required flags

- `--scope-type <scope_type>`.
- `--key <key>`.

Optional flags

- `--scope-id <id>`.
  Scope id rules MUST match `cynork prefs list`.

Output

- Table mode MUST print exactly one line containing `key=<key>` and `value=<json>`.
- JSON mode MUST print `{"key":"<key>","value":<json>}`.

### `cynork prefs set`

Invocation

- `cynork prefs set --scope-type <scope_type> [--scope-id <id>] --key <key>` with exactly one value input method.

Required flags

- `--scope-type <scope_type>`.
- `--key <key>`.

Value input methods (exactly one MUST be used)

- `--value <json>`.
- `--value-file <path>`.

Optional flags

- `--scope-id <id>`.
  Scope id rules MUST match `cynork prefs list`.
- `--reason <string>`.

Behavior

- If `--value` is used, the CLI MUST parse the argument as JSON.
- If `--value-file` is used, the CLI MUST read the file as UTF-8 text and parse it as JSON.
- If JSON parsing fails, the CLI MUST exit with code 2 before making a gateway request.

Output

- Table mode MUST print exactly one line containing `ok=true`.
- JSON mode MUST print `{"ok":true}`.

### `cynork prefs delete`

Invocation

- `cynork prefs delete --scope-type <scope_type> [--scope-id <id>] --key <key>`.

Required flags

- `--scope-type <scope_type>`.
- `--key <key>`.

Optional flags

- `--scope-id <id>`.
  Scope id rules MUST match `cynork prefs list`.

Output

- Table mode MUST print exactly one line containing `deleted=true`.
- JSON mode MUST print `{"deleted":true}`.

### `cynork prefs effective`

Invocation

- `cynork prefs effective` with exactly one selector.

Selectors (exactly one MUST be provided)

- `--task-id <task_id>`.
- `--project-id <project_id>`.

Output

- Table mode MUST print the merged JSON object on stdout.
- JSON mode MUST print `{"effective":<json>,"sources":[... ]}`.
  The `sources` array MUST contain one object per resolved key.
  Each source object MUST include `key`, `scope_type`, and `scope_id`.

## Node Management

- Spec ID: `CYNAI.CLIENT.CliNodeManagement` <a id="spec-cynai-client-clinodemgmt"></a>

Traces To:

- [REQ-CLIENT-0125](../requirements/client.md#req-client-0125)
- [REQ-CLIENT-0126](../requirements/client.md#req-client-0126)
- [REQ-CLIENT-0128](../requirements/client.md#req-client-0128)

The CLI MUST support node inventory and admin actions via the User API Gateway (no direct worker API calls); semantics align with [Node](node.md) and the Admin Web Console.

All node commands MUST require auth.

### `cynork nodes list`

Invocation

- `cynork nodes list`.

Optional flags

- `-l, --limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `node_id`, `status`, `enabled`, `last_heartbeat`, `capability_summary`.
- Table mode MUST then print one row per node.
- JSON mode MUST print `{"nodes":[...],"next_cursor":"<opaque>"}`.
  Each node object MUST include at least `node_id`, `status`, and `enabled`.

### `cynork nodes get <node_id>`

Invocation

- `cynork nodes get <node_id>`.

Output

- Table mode MUST print exactly one line containing at least `node_id=<id>` and `status=<status>` and `enabled=<bool>`.
- JSON mode MUST print a single JSON object containing at least `node_id`, `status`, and `enabled`.

### `cynork nodes enable <node_id>`

Invocation

- `cynork nodes enable <node_id>`.

Optional flags

- `-y, --yes`.

Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation.
- The confirmation prompt MUST be `Enable node <node_id>? [y/N]`.
- If the user does not enter `y` or `Y`, the CLI MUST exit with code 0 and MUST NOT make a gateway request.

Output

- Table mode MUST print exactly one line containing `node_id=<id> enabled=true`.
- JSON mode MUST print `{"node_id":"<id>","enabled":true}`.

### `cynork nodes disable <node_id>`

Invocation

- `cynork nodes disable <node_id>`.

Optional flags

- `-y, --yes`.

Behavior

- Confirmation behavior MUST match `cynork nodes enable`.
  The confirmation prompt MUST be `Disable node <node_id>? [y/N]`.

Output

- Table mode MUST print exactly one line containing `node_id=<id> enabled=false`.
- JSON mode MUST print `{"node_id":"<id>","enabled":false}`.

### `cynork nodes drain <node_id>`

Invocation

- `cynork nodes drain <node_id>`.

Optional flags

- `-y, --yes`.

Behavior

- Confirmation behavior MUST match `cynork nodes enable`.
  The confirmation prompt MUST be `Drain node <node_id>? [y/N]`.

Output

- Table mode MUST print exactly one line containing `node_id=<id> drained=true`.
- JSON mode MUST print `{"node_id":"<id>","drained":true}`.

### `cynork nodes refresh-config <node_id>`

Invocation

- `cynork nodes refresh-config <node_id>`.

Output

- Table mode MUST print exactly one line containing `node_id=<id> refresh_config_requested=true`.
- JSON mode MUST print `{"node_id":"<id>","refresh_config_requested":true}`.

### `cynork nodes prefetch-image <node_id> <image_ref>`

Invocation

- `cynork nodes prefetch-image <node_id> <image_ref>`.

Output

- Table mode MUST print exactly one line containing `node_id=<id> prefetch_requested=true`.
- JSON mode MUST print `{"node_id":"<id>","prefetch_requested":true}`.

## Skills Management

- Spec ID: `CYNAI.CLIENT.CliSkillsManagement` <a id="spec-cynai-client-cliskillsmanagement"></a>

Traces To:

- [REQ-CLIENT-0146](../requirements/client.md#req-client-0146)

The CLI MUST support full CRUD for skills (create/load, list, get, update, delete) via the User API Gateway, with the same controls as defined in [Skill Management CRUD (Web and CLI)](skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud).

All skills commands MUST require auth.

Scope enum

- `user`
- `group`
- `project`
- `global`

### `cynork skills load <file.md>`

Invocation

- `cynork skills load <file.md>`.

Optional flags

- `--name <name>`.
- `--scope <scope>`.
  Allowed values are `user`, `group`, `project`, and `global`.
  Default is `user`.
- `--scope-id <id>`.
  Required when `--scope` is `group` or `project`.
  Forbidden when `--scope` is `global`.

Behavior

- The CLI MUST read `<file.md>` as UTF-8 text.
- The CLI MUST reject unreadable files and MUST exit with code 2 before making a gateway request.

Output

- Table mode MUST print exactly one line containing `skill_id=<id>`.
- JSON mode MUST print `{"skill_id":"<id>"}`.

### `cynork skills list`

Invocation

- `cynork skills list`.

Optional flags

- `--scope <scope>`.
- `--owner <owner_id>`.
- `--limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `skill_id`, `name`, `scope`, `scope_id`, `owner_id`, `updated_at`.
- Table mode MUST then print one row per skill.
- JSON mode MUST print `{"skills":[...],"next_cursor":"<opaque>"}`.
  Each skill object MUST include at least `skill_id`, `name`, and `scope`.

### `cynork skills get <skill_id>`

Invocation

- `cynork skills get <skill_id>`.

Output

- Table mode MUST print the skill metadata followed by the skill content.
- Table mode MUST include a metadata line containing at least `skill_id=<id>` and `scope=<scope>`.
- JSON mode MUST print `{"skill_id":"<id>","name":"<name>","scope":"<scope>","scope_id":"<id_or_empty>","content_md":"<markdown>"}`.

### `cynork skills update <skill_id> <file.md>`

Invocation

- `cynork skills update <skill_id> <file.md>`.

Optional flags

- `--name <name>`.
- `--scope <scope>`.
- `--scope-id <id>`.
  Scope and scope-id rules MUST match `cynork skills load`.

Behavior

- The CLI MUST read `<file.md>` as UTF-8 text.

Output

- Table mode MUST print exactly one line containing `skill_id=<id> updated=true`.
- JSON mode MUST print `{"skill_id":"<id>","updated":true}`.

### `cynork skills delete <skill_id>`

Invocation

- `cynork skills delete <skill_id>`.

Optional flags

- `-y, --yes`.

Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation.
- The confirmation prompt MUST be `Delete skill <skill_id>? [y/N]`.
- If the user does not enter `y` or `Y`, the CLI MUST exit with code 0 and MUST NOT make a gateway request.

Output

- Table mode MUST print exactly one line containing `skill_id=<id> deleted=true`.
- JSON mode MUST print `{"skill_id":"<id>","deleted":true}`.

## Audit Commands

- Spec ID: `CYNAI.CLIENT.CliAuditCommands` <a id="spec-cynai-client-cliauditcommands"></a>

The CLI MUST support querying audit events via the User API Gateway.

All audit commands MUST require auth.

### `cynork audit list`

Invocation

- `cynork audit list`.

Optional flags

- `--resource-type <type>`.
- `--actor-id <id>`.
- `--since <rfc3339>`.
- `--until <rfc3339>`.
- `--limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `event_id`, `ts`, `actor_id`, `action`, `resource_type`, `resource_id`, `decision`.
- Table mode MUST then print one row per event.
- JSON mode MUST print `{"events":[...],"next_cursor":"<opaque>"}`.
  Each event object MUST include at least `event_id`, `ts`, `actor_id`, `action`, `resource_type`, and `resource_id`.

### `cynork audit get <event_id>`

Invocation

- `cynork audit get <event_id>`.

Output

- Table mode MUST print the event as key-value pairs.
- JSON mode MUST print a single JSON object representing the event.

## Output and Scripting

The CLI MUST be scriptable: JSON output and non-zero exit on failure are required for automation.

### Output and Scripting Applicable Requirements

- Spec ID: `CYNAI.CLIENT.CliOutputScripting` <a id="spec-cynai-client-clioutputscripting"></a>

Traces To:

- [REQ-CLIENT-0143](../requirements/client.md#req-client-0143)
- [REQ-CLIENT-0144](../requirements/client.md#req-client-0144)
- [REQ-CLIENT-0145](../requirements/client.md#req-client-0145)

Required and optional flags

- `--output` (string): MUST be supported globally or on list/get commands; values `table` (default, human-readable) and `json` (one JSON value to stdout, no extra text).
  When `json`, the CLI MUST output only the JSON document so that `cynork ... --output json` is parseable by `jq` or equivalent.
- `--quiet` (bool, optional): suppress non-essential output; errors MUST still be printed to stderr.
- `--no-color` (bool, optional): disable colored output; MUST be honored when set.

## Implementation Specification (Go + Cobra)

- Spec ID: `CYNAI.CLIENT.CliImplementation` <a id="spec-cynai-client-cliimpl"></a>

Traces To:

- [REQ-CLIENT-0101](../requirements/client.md#req-client-0101)
- [REQ-CLIENT-0102](../requirements/client.md#req-client-0102)
- [REQ-CLIENT-0103](../requirements/client.md#req-client-0103)

The CLI MUST be implemented in Go using Cobra for the command tree.
Implementation MUST follow [Go REST API Standards](go_rest_api_standards.md) where applicable (e.g. HTTP client behavior, error handling).

Required package layout

- `cmd/`: Cobra root and subcommands; MUST NOT contain business logic beyond delegation to internal packages.
- `internal/config/`: Config file load, XDG/default path resolution, env overrides, and token resolution (env then file then credential helper).
  MUST expose a single load entrypoint that returns a struct with at least `GatewayURL` and resolved `Token`; token resolution MUST be implemented as specified in [Token Resolution (Precedence)](#token-resolution-precedence) and [Credential Helper Protocol](#credential-helper-protocol).
- `internal/gateway/`: HTTP client for the User API Gateway; MUST set `Authorization: Bearer <token>` on every request when token is non-empty; MUST map 401/403/429/5xx to typed errors; MUST NOT log request or response bodies that may contain secrets.
- `internal/output/`: Formatters for table and JSON output; MUST support `--output table` and `--output json` for list/get commands.

Config load and lifecycle

- Root command persistent pre-run (or equivalent) MUST: load config from `--config` or default path, apply env overrides, resolve token (including credential helper if configured), and store effective gateway URL and token in a shared struct or context used by all child commands.
- If config file path is provided and the file exists but is invalid (e.g. invalid YAML), the CLI MUST exit with a non-zero code and MUST NOT proceed to run the requested command.
- Config MUST be loaded once per process; child commands MUST use the same resolved values.

Gateway client contract

- All gateway calls MUST use the same base URL and token from the resolved config.
- Requests MUST include a `User-Agent` or similar identifier (e.g. `cynork/<version>`).
- Response bodies MUST be decoded into typed structs; gateway error responses (e.g. RFC 7807 or 4xx/5xx) MUST be mapped to errors that the CLI can report without leaking secrets.
- The client MUST NOT persist or log the token; it MUST NOT include the token in any log line or error message.

Secrets and logging

- The CLI MUST NOT log config file contents, token values, credential helper stdin/stdout containing secrets, or any flag value that is a secret (e.g. `--secret-file` path MAY be logged; file contents MUST NOT).
- When reading secrets from stdin or a file, buffers MUST be cleared or not retained longer than necessary for the single request.

## MVP Scope

Minimum viable CLI

- Auth token support and `whoami`.
- Interactive shell mode with tab completion for all MVP commands.
- Credential list, create, rotate, disable for API Egress.
- Preference list, get, set for system and user scopes.
- Effective preferences for a task.
- Node list, get, enable, disable, drain.

Related specifications

- [User API Gateway](user_api_gateway.md)
- [Data REST API](data_rest_api.md)
- [Admin Web Console](admin_web_console.md)
- [Client requirements](../requirements/client.md)
