# Running Cynork CLI Against Localhost

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Build](#build)
- [Configuration](#configuration)
- [Commands](#commands)
- [After just e2e](#after-just-e2e)

## Overview

The `cynork` CLI is a separate Go module at repo root (`cynork/`).
It talks to the User API Gateway (user-gateway) for auth, tasks, and (later) admin operations.
See [post_phase1_mvp_plan.md](post_phase1_mvp_plan.md) and [docs/tech_specs/cli_management_app.md](../docs/tech_specs/cli_management_app.md).

## Prerequisites

- User-gateway running on a known host/port (e.g. `http://localhost:8080` after `just e2e` or manual start).
- For task operations: at least one worker node registered and config ack'd so tasks can be dispatched.

## Build

From repo root run one of:

```bash
go build -o tmp/cynork ./cynork
# Or: cd cynork && go build -o ../tmp/cynork .
```

## Configuration

- **Gateway URL:** Env `CYNORK_GATEWAY_URL` or config file; default `http://localhost:8080`.
- **Token:** Env `CYNORK_TOKEN` or config file (or use `cynork auth login` to store token in config).
- **Config file (optional):** `~/.config/cynork/config.yaml` with `gateway_url` and `token`.
  Created automatically on first `cynork auth login` if the directory exists or can be created.

Example config:

```yaml
gateway_url: http://localhost:8080
token: <access-token-from-login>
```

## Commands

- `cynork version` - binary version/build info.
- `cynork status` - GET gateway `/healthz`; prints "ok" if reachable.
- `cynork auth login` - interactive or `-u` / `-p`; stores token in config.
- `cynork auth logout` - clears stored token.
- `cynork auth whoami` - GET `/v1/users/me`; shows current user (requires token).
- `cynork task create --prompt "echo hello"` - POST `/v1/tasks`; prints task ID.
- `cynork task result <task-id>` - GET `/v1/tasks/{id}/result`; prints status and job results.

All operations use the User API Gateway; no direct database access.

## After Just E2e

1. Run `just e2e` (or start Postgres, control-plane, user-gateway, and node manually).
2. Build: `go build -o tmp/cynork ./cynork`.
3. Check gateway: `./tmp/cynork status` (expect "ok" if user-gateway is on port 8080).
4. Log in (use the bootstrap admin user from your env or migrations):  
   `./tmp/cynork auth login -u <handle> -p <password>`  
   Or without flags for interactive prompts.
5. Create a task: `./tmp/cynork task create --prompt "echo hello"`.
6. Poll or fetch result: `./tmp/cynork task result <task-id>`.

If the gateway listens on a different port, set `CYNORK_GATEWAY_URL` (e.g. `http://localhost:18080`) or set `gateway_url` in `~/.config/cynork/config.yaml`.

Report generated 2026-02-20.
