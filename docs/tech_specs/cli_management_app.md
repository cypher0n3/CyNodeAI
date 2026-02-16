# CLI Management App

- [Document Overview](#document-overview)
- [Goals and Non-Goals](#goals-and-non-goals)
- [Security Model](#security-model)
- [Authentication and Configuration](#authentication-and-configuration)
- [Command Surface](#command-surface)
- [Interactive Mode (REPL)](#interactive-mode-repl)
- [Credential Management](#credential-management)
- [Preferences Management](#preferences-management)
- [Node Management](#node-management)
- [Output and Scripting](#output-and-scripting)
- [Implementation Specification (Go + Cobra)](#implementation-specification-go--cobra)
- [MVP Scope](#mvp-scope)

## Document Overview

This document defines a CLI management application for CyNodeAI.
The CLI is intended to support the same administrative capabilities as the Admin Web Console.

## Goals and Non-Goals

Goals

- Provide a fast, scriptable admin interface for credentials, preferences, and node management.
- Operate against the User API Gateway so the CLI does not require direct database access.
- Support secure secret input and rotation without echoing secrets to terminal logs.

Non-goals

- The CLI is not an agent tool and is not exposed to sandboxes.
- The CLI is not intended to perform worker-node internal operations directly.

## Security Model

Normative requirements

- The CLI MUST NOT connect directly to PostgreSQL.
- The CLI MUST call the User API Gateway for all operations.
- The CLI MUST avoid printing secrets.
- The CLI MUST not persist plaintext secrets to disk.
- The CLI MUST support least privilege and MUST fail closed on authorization errors.

## Authentication and Configuration

The CLI is a user client.
It authenticates to the User API Gateway and is authorized by the gateway.

Recommended configuration

- Config file path: `~/.config/cynork/config.yaml`
- Environment variable overrides:
  - `CYNORK_GATEWAY_URL`
  - `CYNORK_TOKEN`

Recommended auth approaches

- API token issued by the gateway.
- Interactive login flow that exchanges username and password for a short-lived token, when enabled.

Normative requirements

- The CLI MUST support token-based authentication.
- The CLI SHOULD support reading tokens from env vars for CI usage.
- The CLI SHOULD support optional mTLS or pinned CA bundles for enterprise deployments.

## Command Surface

The CLI SHOULD be implemented as `cynork` with subcommands.

Recommended top-level commands

- `cynork version`
- `cynork status`
- `cynork auth login`
- `cynork auth logout`
- `cynork auth whoami`
- `cynork creds ...`
- `cynork prefs ...`
- `cynork nodes ...`
- `cynork audit ...`

## Interactive Mode (REPL)

The CLI SHOULD provide an interactive mode that exposes the same command surface as the non-interactive CLI,
and provides tab completion to accelerate discovery and reduce typing errors.

Recommended entrypoint

- `cynork shell`

Normative requirements

- The interactive mode MUST provide access to the same commands and flags as the non-interactive CLI.
  - Example: `cynork creds list --output json` MUST be usable as `creds list --output json` inside the shell.
- The interactive mode MUST support tab completion for:
  - Commands and subcommands.
  - Flags for the current command.
  - Enumerated flag values where known (for example `--output (table|json)`).
- The interactive mode MUST support in-session help (for example `help`, `help <command>`, and/or `<command> --help`).
- The interactive mode MUST support `exit` and `quit`.
- The interactive mode MUST NOT store secrets in history.
  - Secret prompts (for example credential create/rotate) MUST bypass history recording.
  - The shell SHOULD support disabling history entirely (for example `--no-history`).
- If a persistent history file is implemented, it MUST be stored under the CLI config directory with permissions `0600`.
- Tab completion MUST NOT fetch or reveal secret values.
  - Dynamic completion MAY fetch metadata identifiers (for example credential names, node IDs) from the gateway when
    authenticated, but MUST treat these as non-secret and MUST fail closed on authorization errors.

Recommended behaviors

- The prompt SHOULD clearly indicate the active environment (gateway URL) and auth identity when available.
- The shell SHOULD preserve the same output semantics as non-interactive mode.
  - Default output can remain human-friendly (table), but JSON output MUST be available via the same `--output` flag.
- The shell SHOULD return non-zero exit codes when a command fails if invoked in single-command mode
  (for example `cynork shell -c "nodes list"`).

## Credential Management

The CLI MUST support credential workflows for API Egress and Git Egress.
Secrets are write-only.

Required commands

- `cynork creds list`
- `cynork creds get`
  - Returns metadata only.
- `cynork creds create`
  - Supports reading secret from:
    - stdin
    - prompt (no echo)
    - file path
- `cynork creds rotate`
  - Same secret input modes as create.
- `cynork creds disable`

Required flags (examples)

- `--provider`
- `--name`
- `--owner-type` (user|team)
- `--owner-id` (uuid, optional when owner-type is user and derived from auth context)

## Preferences Management

The CLI MUST support reading and writing preferences across scopes.

Required commands

- `cynork prefs list`
- `cynork prefs get`
- `cynork prefs set`
- `cynork prefs delete`
- `cynork prefs effective`
  - Computes effective preferences for a task or project.

Required flags (examples)

- `--scope-type` (system|user|project|task)
- `--scope-id`
- `--key`
- `--value` (json)
- `--value-file` (json file)
- `--reason`

## Node Management

The CLI MUST support basic node management actions matching the web console.

Required commands

- `cynork nodes list`
- `cynork nodes get`
- `cynork nodes enable`
- `cynork nodes disable`
- `cynork nodes drain`
- `cynork nodes refresh-config`

Recommended commands

- `cynork nodes prefetch-image`

## Output and Scripting

The CLI SHOULD be scriptable.

Normative requirements

- The CLI MUST support JSON output mode.
- The CLI SHOULD support table output mode for humans.
- The CLI SHOULD return non-zero exit codes on failures and policy denials.

Recommended flags

- `--output` (table|json)
- `--quiet`
- `--no-color`

## Implementation Specification (Go + Cobra)

The CLI SHOULD be written in Go using Cobra for command structure.

Recommended implementation structure

- `cmd/`
  - Cobra command definitions.
- `internal/gateway/`
  - Typed gateway client, auth, retries, error decoding.
- `internal/output/`
  - Output rendering for table and json.
- `internal/config/`
  - Config loading, env overrides, validation.

Recommended behaviors

- Centralize gateway base URL resolution.
- Centralize auth header injection.
- Centralize pagination support and filtering helpers.
- Standardize error mapping (401, 403, 429, 5xx).
- Avoid logging request bodies that may contain secrets.

## MVP Scope

Minimum viable CLI

- Auth token support and `whoami`.
- Interactive shell mode with tab completion for all MVP commands.
- Credential list, create, rotate, disable for API Egress.
- Preference list, get, set for system and user scopes.
- Effective preferences for a task.
- Node list, get, enable, disable, drain.

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md), [`docs/tech_specs/data_rest_api.md`](data_rest_api.md), and [`docs/tech_specs/admin_web_console.md`](admin_web_console.md).
