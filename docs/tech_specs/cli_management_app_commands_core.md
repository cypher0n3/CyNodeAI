# CLI Management App - Core Commands

- [Document overview](#document-overview)
- [`cynork version`](#cynork-version)
- [`cynork status`](#cynork-status)
- [`cynork auth` Commands](#cynork-auth-commands)

## Document Overview

This document specifies the core CLI commands: `cynork version`, `cynork status`, and `cynork auth` (login, logout, whoami).
It is part of the [cynork CLI](cynork_cli.md) specification.

Traces To:

- [REQ-CLIENT-0101](../requirements/client.md#req-client-0101)
- [REQ-CLIENT-0155](../requirements/client.md#req-client-0155)
- [REQ-CLIENT-0156](../requirements/client.md#req-client-0156)
- [REQ-CLIENT-0158](../requirements/client.md#req-client-0158)

## `cynork version`

Invocation

- `cynork version`.

Behavior

- The CLI MUST print build and version metadata.
- The CLI MUST NOT require auth.

Output

- Table mode MUST print exactly one line containing `version=<string>`.
- JSON mode MUST print `{"version":"<string>"}`.

## `cynork status`

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

## `cynork auth` Commands

All `auth` subcommands MUST use the gateway auth endpoints.

### `cynork auth login`

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

### `cynork auth logout`

Invocation

- `cynork auth logout`.

Behavior

- The CLI MUST remove the token from the config file and MUST clear it from the credential helper if configured.
- The CLI MUST NOT require a gateway call.

Output

- Table mode MUST print exactly one line containing `logged_out=true`.
- JSON mode MUST print `{"logged_out":true}`.

### `cynork auth whoami`

Invocation

- `cynork auth whoami`.

Output

- Table mode MUST print exactly one line containing `id=<id>` and `handle=<handle>`.
- JSON mode MUST print `{"id":"<id>","handle":"<handle>"}`.
