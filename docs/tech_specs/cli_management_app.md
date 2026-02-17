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

Recommended configuration

- Config file path: `~/.config/cynork/config.yaml`
- Environment variable overrides:
  - `CYNORK_GATEWAY_URL`
  - `CYNORK_TOKEN`

Recommended auth approaches

- API token issued by the gateway.
- Interactive login flow that exchanges username and password for a short-lived token, when enabled.

### Authentication and Configuration Applicable Requirements

- Spec ID: `CYNAI.CLIENT.CliAuthConfig` <a id="spec-cynai-client-cliauth"></a>

Traces To:

- [REQ-CLIENT-0105](../requirements/client.md#req-client-0105)
- [REQ-CLIENT-0106](../requirements/client.md#req-client-0106)
- [REQ-CLIENT-0107](../requirements/client.md#req-client-0107)

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
- `--owner-type` (user|group)
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

### Output and Scripting Applicable Requirements

- Spec ID: `CYNAI.CLIENT.CliOutputScripting` <a id="spec-cynai-client-clioutputscripting"></a>

Traces To:

- [REQ-CLIENT-0143](../requirements/client.md#req-client-0143)
- [REQ-CLIENT-0144](../requirements/client.md#req-client-0144)
- [REQ-CLIENT-0145](../requirements/client.md#req-client-0145)

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
