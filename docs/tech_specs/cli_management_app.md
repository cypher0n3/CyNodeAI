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
  - [Task Commands](#task-commands)
  - [Task Creation (Task Input and Attachments)](#task-creation-task-input-and-attachments)
  - [Chat Command](#chat-command)
  - [Interactive Mode (REPL)](#interactive-mode-repl)
  - [Credential Management](#credential-management)
  - [Preferences Management](#preferences-management)
  - [System Settings Management](#system-settings-management)
  - [Node Management](#node-management)
  - [Skills Management](#skills-management)
  - [Audit Commands](#audit-commands)
  - [Output and Scripting](#output-and-scripting)
- [Implementation Specification (Go + Cobra)](#implementation-specification-go--cobra)
- [MVP Scope](#mvp-scope)

## Document Overview

- Spec ID: `CYNAI.CLIENT.Doc.CliManagementApp` <a id="spec-cynai-client-doc-climanagementapp"></a>

This spec is split into multiple documents.
Command and feature details live in: [Core commands (version, status, auth)](cli_management_app_commands_core.md), [Task commands](cli_management_app_commands_tasks.md), [Chat command](cli_management_app_commands_chat.md), [Admin and resource commands](cli_management_app_commands_admin.md), [Interactive mode and output](cli_management_app_shell_output.md).

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
Whenever the CLI outputs JSON (including `--output json`, JSON embedded in chat or table output, or other responses), the JSON MUST be pretty-printed (indented, with newlines) for human readability; see [Pretty-Printed JSON](cli_management_app_shell_output.md#pretty-printed-json-output).

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
- `cynork task ...`: create tasks, list tasks, get task status, watch task status, cancel tasks, and retrieve task results and artifacts; see [Task commands](cli_management_app_commands_tasks.md).
- `cynork chat`: start an interactive chat session with the Project Manager (PM) model; see [Chat command](cli_management_app_commands_chat.md).
- `cynork creds ...`: see [Credential Management](cli_management_app_commands_admin.md#credential-management); MUST use gateway credential endpoints.
- `cynork prefs ...`: see [Preferences Management](cli_management_app_commands_admin.md#preferences-management).
- `cynork nodes ...`: see [Node Management](cli_management_app_commands_admin.md#node-management).
- `cynork skills ...`: full CRUD via gateway; see [Skill Management CRUD (Web and CLI)](skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud) and [Skills Management](cli_management_app_commands_admin.md#skills-management).
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

### Core Commands (Version, Status, Auth)

Full specification: [CLI management app - Core commands](cli_management_app_commands_core.md).

### Task Commands

- Spec ID: `CYNAI.CLIENT.CliCommandSurface` (task subset) <a id="spec-cynai-client-clicommandsurface"></a>
- Full specification: [CLI management app - Task commands](cli_management_app_commands_tasks.md#task-commands).

### Task Creation (Task Input and Attachments)

- Spec ID: `CYNAI.CLIENT.CliTaskCreatePrompt` <a id="spec-cynai-client-clitaskcreateprompt"></a>
- Full specification: [CLI management app - Task commands](cli_management_app_commands_tasks.md#task-creation-task-input-and-attachments).

### Chat Command

- Spec ID: `CYNAI.CLIENT.CliChat` <a id="spec-cynai-client-clichat"></a>
- Full specification: [CLI management app - Chat command](cli_management_app_commands_chat.md#chat-command).

## Interactive Mode (REPL)

- Spec ID: `CYNAI.CLIENT.CliInteractiveMode` <a id="spec-cynai-client-cliinteractivemode"></a>
- Full specification: [CLI management app - Interactive mode and output](cli_management_app_shell_output.md#interactive-mode-repl).

## Credential Management

- Spec ID: `CYNAI.CLIENT.CliCredentialManagement` <a id="spec-cynai-client-clicredential"></a>
- Full specification: [CLI management app - Admin and resource commands](cli_management_app_commands_admin.md#credential-management).

## Preferences Management

- Spec ID: `CYNAI.CLIENT.CliPreferencesManagement` <a id="spec-cynai-client-clipreferences"></a>
- Full specification: [CLI management app - Admin and resource commands](cli_management_app_commands_admin.md#preferences-management).

## System Settings Management

- Spec ID: `CYNAI.CLIENT.CliSystemSettingsManagement` <a id="spec-cynai-client-clisystemsettings"></a>
- Full specification: [CLI management app - Admin and resource commands](cli_management_app_commands_admin.md#system-settings-management).

## Node Management

- Spec ID: `CYNAI.CLIENT.CliNodeManagement` <a id="spec-cynai-client-clinodemgmt"></a>
- Full specification: [CLI management app - Admin and resource commands](cli_management_app_commands_admin.md#node-management).

## Skills Management

- Spec ID: `CYNAI.CLIENT.CliSkillsManagement` <a id="spec-cynai-client-cliskillsmanagement"></a>
- Full specification: [CLI management app - Admin and resource commands](cli_management_app_commands_admin.md#skills-management).

## Audit Commands

- Spec ID: `CYNAI.CLIENT.CliAuditCommands` <a id="spec-cynai-client-cliauditcommands"></a>
- Full specification: [CLI management app - Admin and resource commands](cli_management_app_commands_admin.md#audit-commands).

## Output and Scripting

- Full specification: [CLI management app - Interactive mode and output](cli_management_app_shell_output.md#output-and-scripting).

### Pretty-Printed JSON Output (Stub)

- Spec ID: `CYNAI.CLIENT.CliPrettyPrintJson` <a id="spec-cynai-client-cliprettyprintjson"></a>

### Output and Scripting Applicable Requirements (Stub)

- Spec ID: `CYNAI.CLIENT.CliOutputScripting` <a id="spec-cynai-client-clioutputscripting"></a>

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
