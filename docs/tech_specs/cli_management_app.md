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
- [Interactive Mode (REPL)](#interactive-mode-repl)
  - [Interactive Mode Applicable Requirements](#interactive-mode-applicable-requirements)
- [Credential Management](#credential-management)
- [Preferences Management](#preferences-management)
- [Node Management](#node-management)
- [Skills Management](#skills-management)
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

- Provide a fast, scriptable admin interface for credentials, preferences, and node management.
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

Config file path

- Default path MUST be resolved as follows: if `XDG_CONFIG_HOME` is set, use `$XDG_CONFIG_HOME/cynork/config.yaml`; otherwise use `~/.config/cynork/config.yaml`.
- The `--config` flag MUST override the default path when provided.
- The CLI MUST create the config directory with mode `0700` when writing (e.g. on `auth login`); it MUST NOT create the file or directory on read if missing.

Config file format (YAML)

- The file MUST be valid YAML.
- Supported top-level keys:
  - `gateway_url` (string, optional): base URL of the User API Gateway (e.g. `http://localhost:8080`).
  - `token` (string, optional): bearer token for gateway auth; MAY be omitted when using a credential helper.
  - `credential_helper` (string, optional): command or helper name to obtain the token (see [Credential Helper Protocol](#credential-helper-protocol)).
- Unknown keys MAY be ignored; the CLI MUST NOT fail load solely due to unknown keys.
- When writing the config file (e.g. after login), the CLI MUST use file mode `0600` and MUST NOT log the contents or token value.

Environment overrides

- `CYNORK_GATEWAY_URL`: if set, overrides `gateway_url` from the config file after load.
- `CYNORK_TOKEN`: if set, overrides the resolved token (see [Token Resolution (Precedence)](#token-resolution-precedence)) for the session; use for CI or ephemeral runs.
- Overrides apply at config load time; the effective gateway URL and token used for requests MUST be the result of applying overrides to the loaded config and resolved token.

Default gateway URL

- When `gateway_url` is empty after load and env override, the CLI MUST use the default `http://localhost:8080` (or a build-time constant matching the orchestrator default).

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

The CLI MUST be implemented as a single binary named `cynork` with subcommands.
All commands that require gateway auth MUST fail immediately with a non-zero exit code and a clear message if the resolved token is empty (see [Token Resolution (Precedence)](#token-resolution-precedence)).

Required global flag

- `--config` (string): path to config file; overrides default config path.
  Optional to specify; when omitted, default path is used.

Required top-level commands

- `cynork version`: print version string (e.g. from build); MUST NOT require auth.
- `cynork status`: report gateway reachability and optionally auth status; MAY require auth for full status.
- `cynork auth login`: interactive or flag-based login; POST to gateway login endpoint; MUST support writing token to config and/or credential helper; MUST NOT echo password.
- `cynork auth logout`: clear token from config file and optionally from credential helper; MUST NOT require gateway call.
- `cynork auth whoami`: call gateway with current token; MUST require auth; output MUST be machine-parseable (e.g. `id=... handle=...`).
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

Error behavior

- On gateway 401/403: print a clear error and exit non-zero; MUST NOT retry with the same token.
- On gateway 5xx or network failure: implementers MAY retry with backoff; if giving up, exit non-zero with a clear message.
- On invalid config file (syntax error): fail load and exit non-zero before running any command.

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

Required behaviors

- The prompt MUST show the active gateway URL (or a short label) and SHOULD show auth identity when available (e.g. handle from whoami).
- Commands entered in the shell MUST behave identically to non-interactive invocation: same flags, same `--output table|json`, same exit codes.
- Tab completion MUST be provided for commands, subcommands, and known flag values; MUST NOT suggest or expose secret values (REQ-CLIENT-0142).
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

Required commands and behavior

- `cynork creds list`: GET list endpoint; optional query filters.
  Output: table or JSON of credential metadata (id, provider, credential_name, owner_type, owner_id, is_active, created_at, updated_at).
- `cynork creds get <id>`: GET by id; response MUST be metadata only; 404 if not found or not authorized.
- `cynork creds create`: POST to create endpoint.
  MUST require `--provider` and `--name`; MUST support `--owner-type` (user|group) and `--owner-id` (default to current user when owner-type is user).
  Secret input: exactly one of `--secret-from-stdin`, prompt (no echo), or `--secret-file <path>`; MUST NOT echo secret to terminal or logs.
- `cynork creds rotate <id>`: POST to rotate endpoint.
  Secret input: same as create; MUST NOT echo secret.
- `cynork creds disable <id>`: PATCH to set inactive (or equivalent); MUST require confirmation or `--yes` for non-interactive use.

Required flags (credential scope)

- `--provider` (string, required for create): provider identifier (e.g. openai, github).
- `--name` (string, required for create): credential name (human-readable).
- `--owner-type` (string, optional): `user` or `group`; default `user`.
- `--owner-id` (string, optional): UUID; when owner-type is user and omitted, derive from current auth context.

## Preferences Management

- Spec ID: `CYNAI.CLIENT.CliPreferencesManagement` <a id="spec-cynai-client-clipreferences"></a>

Traces To:

- [REQ-CLIENT-0121](../requirements/client.md#req-client-0121)
- [REQ-CLIENT-0122](../requirements/client.md#req-client-0122)
- [REQ-CLIENT-0123](../requirements/client.md#req-client-0123)
- [REQ-CLIENT-0124](../requirements/client.md#req-client-0124)

The CLI MUST support reading and writing preferences via the Data REST API; scope and key semantics are defined in [User preferences](user_preferences.md).

Required commands and behavior

- `cynork prefs list`: list preferences for a scope; MUST require `--scope-type` (system|user|project|task); when scope-type is user, scope-id MAY default to current user.
- `cynork prefs get`: get one preference; MUST require `--scope-type` and `--key`; `--scope-id` required for project/task.
- `cynork prefs set`: set a preference; MUST require `--scope-type`, `--key`, and either `--value` (JSON string) or `--value-file` (path to JSON file); SHOULD support `--reason` for audit.
- `cynork prefs delete`: delete a preference; MUST require `--scope-type` and `--key`.
- `cynork prefs effective`: resolve effective preferences for a task or project; MUST require task id or project id (e.g. `--task-id` or `--project-id`); output MUST show merged result and MAY show precedence.

All preference commands MUST require auth except where the gateway allows unauthenticated read for system scope.

## Node Management

- Spec ID: `CYNAI.CLIENT.CliNodeManagement` <a id="spec-cynai-client-clinodemgmt"></a>

Traces To:

- [REQ-CLIENT-0125](../requirements/client.md#req-client-0125)
- [REQ-CLIENT-0126](../requirements/client.md#req-client-0126)
- [REQ-CLIENT-0128](../requirements/client.md#req-client-0128)

The CLI MUST support node inventory and admin actions via the User API Gateway (no direct worker API calls); semantics align with [Node](node.md) and the Admin Web Console.

Required commands and behavior

- `cynork nodes list`: GET node list from gateway; output MUST include at least node id, status, last heartbeat, and capability summary; MUST require auth.
- `cynork nodes get <node_id>`: GET node detail; 404 if not found; MUST require auth.
- `cynork nodes enable <node_id>`: set node enabled for scheduling; MUST require auth; SHOULD require confirmation or `--yes` for non-interactive.
- `cynork nodes disable <node_id>`: set node disabled; MUST require auth; SHOULD require confirmation or `--yes`.
- `cynork nodes drain <node_id>`: stop assigning new jobs, allow in-flight to complete; MUST require auth; SHOULD require confirmation or `--yes`.
- `cynork nodes refresh-config <node_id>`: request node to refresh config from orchestrator; MUST require auth.

Optional commands

- `cynork nodes prefetch-image <node_id> [image_ref]`: request pre-pull of a sandbox image when allowed by policy.

## Skills Management

- Spec ID: `CYNAI.CLIENT.CliSkillsManagement` <a id="spec-cynai-client-cliskillsmanagement"></a>

Traces To:

- [REQ-CLIENT-0146](../requirements/client.md#req-client-0146)

The CLI MUST support full CRUD for skills (create/load, list, get, update, delete) via the User API Gateway, with the same controls as defined in [Skill Management CRUD (Web and CLI)](skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud).

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
