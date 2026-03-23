# Scripts

- [Overview](#overview)
- [Directory Layout](#directory-layout)
- [Dev Setup (Scripts/justfile)](#dev-setup-scriptsjustfile)
  - [Startup Sequence (Start / Full-Demo / Restart)](#startup-sequence-start--full-demo--restart)
  - [`setup-dev` Commands](#setup-dev-commands)
- [E2E Test Suite](#e2e-test-suite)
- [Justfile Entry Points](#justfile-entry-points)
- [Environment](#environment)
  - [Key Environment Variables](#key-environment-variables)
- [Troubleshooting](#troubleshooting)
- [Lint](#lint)

## Overview

The `scripts/` directory provides E2E testing and auxiliary scripts.
Development setup is implemented in **just/shell** in `scripts/justfile`; use `just setup-dev` or `just setup-dev help` from repo root.

## Directory Layout

- **test_scripts/** - Python E2E suite (unittest; discovers `e2e_*.py`).
  See [test_scripts/README.md](test_scripts/README.md) for test layout, state, and how to add tests.
- **dev_stack.sh** – Canonical shell script for the dev stack: Postgres, compose (orchestrator), and node-manager.
  Run from repo root: `./scripts/dev_stack.sh . start-db`, `./scripts/dev_stack.sh . start`, etc.
  The scripts/justfile reimplements the same flow inline.
  Use `just setup-dev` for the usual entry point.
- **justfile** – Dev and E2E recipes (start, stop, build, full-demo, e2e).
  Invoked from repo root via `just setup-dev` or `just scripts/<recipe>`.
- **requirements-lint.txt** - Python lint tooling; installed by `just venv` together with E2E deps.
- **requirements-e2e.txt** - Optional E2E deps (e.g. pexpect for TUI PTY tests), installed by `just venv`.
  TUI PTY tests are skipped if pexpect is not installed.
- **podman-generate-units.sh** - Optional/auxiliary; see justfile for primary workflows.

## Dev Setup (Scripts/justfile)

All `just setup-dev <command>` behavior is implemented natively in `scripts/justfile` (just/shell).
Run from repo root: `just setup-dev` (shows help), `just setup-dev start`, `just setup-dev full-demo --stop-on-success`, etc.

### Startup Sequence (Start / Full-Demo / Restart)

When you run `just setup-dev start`, `full-demo`, or `restart`, the following run in order:

1. **Build binaries** - `just build-dev` (orchestrator, worker_node, cynork, agents).
2. **Compose up (orchestrator stack)** - Uses `orchestrator/docker-compose.yml`.
   Compose stack: postgres, control-plane, user-gateway, api-egress.
    The deprecated standalone **`mcp-gateway`** (port 12083) is available only with compose profile **`legacy-mcp-gateway`**; MCP tool calls should use the **control-plane** (12082) `POST /v1/mcp/tools/call` route.
   MCP tool routes belong on the **control-plane**; drop **`mcp-gateway`** when [`orchestrator/docker-compose.yml`](../orchestrator/docker-compose.yml) is next edited (see [`docs/tech_specs/ports_and_endpoints.md`](../docs/tech_specs/ports_and_endpoints.md)).
   **ollama** is the only profile (use `--ollama-in-stack` or `SETUP_DEV_OLLAMA_IN_STACK=1`).
   **AI agents must NOT use** these flags; they bypass the node-manager path and invalidate GPU variant E2E (e2e_0800).
   PMA is started by the node-manager, not by compose.
3. **Start node** - Script runs the node-manager binary (`worker_node/bin/cynodeai-wnm-dev`), which starts worker-api as a subprocess.
   The node-manager polls the control-plane `/readyz` before registering; the script then waits for worker-api `/healthz` (up to 30s).
4. **PMA** - The worker node starts PMA when the orchestrator directs (see orchestrator_bootstrap.md, worker_node managed services).
   Script waits for control-plane readyz (up to 60s).

Compose images (control-plane, user-gateway, cynode-pma) are not built by `start`; they are built by `full-demo` before this sequence.
For `just setup-dev start` to reach the node step, those images must already exist (e.g. run `full-demo` once, or build them separately).

### `setup-dev` Commands

- **start-db** - Start standalone Postgres container (podman/docker).
- **stop-db** - Stop the Postgres container.
- **clean-db** - Stop and remove the Postgres container and volume.
- **migrate** - No-op; migrations run when control-plane starts.
- **build** - Run `just build-dev` (orchestrator, worker_node, cynork, agents binaries).
- **build-e2e-images** - Build inference-proxy and cynode-sba images for E2E.
- **start** - Runs the full startup sequence above (build binaries, compose up, **start node** (node polls control-plane), wait for readyz).
  Use `--ollama-in-stack` (or `SETUP_DEV_OLLAMA_IN_STACK=1`) for OLLAMA in compose.
- **stop** - Kill node-manager, free worker port, compose down, remove containers.
- **restart** - Stop all then start (same as stop + start); accepts same bypass flags as start.
- **clean** - Stop all services and remove postgres container/volume.
- **component** - Per-component **start**, **stop**, **restart**, or **rebuild**.
  Usage: `component <name> <action>`.
  **Names (start/stop/restart):** postgres, control-plane, user-gateway, api-egress, ollama, node-manager, and **mcp-gateway** (deprecated; remove when compose no longer defines it).
  **Names (rebuild only):** cynode-pma, worker-api, inference-proxy, cynode-sba (images), or any start/stop name to rebuild that image/binary.
  Rebuild uses `just build/*`; pass `--no-cache` where supported.
- **test-e2e** - Run the Python E2E suite ([test_scripts/run_e2e.py](test_scripts/run_e2e.py)); stack must be up.
- **full-demo** - Build, build E2E images, start stack and node, run E2E suite; optionally stop on success.
  Use `--stop-on-success` or `STOP_ON_SUCCESS_ENV=1` to tear down after tests pass.
  Accepts same bypass flags as start; use `--ollama-in-stack` if E2E expects OLLAMA in compose.
- **help** - Show usage, bypass flags, and environment variables.

#### Setup Examples

```bash
just setup-dev build
just setup-dev start
just setup-dev start --ollama-in-stack
just setup-dev full-demo --stop-on-success
```

Or via just: `just setup-dev build`, `just setup-dev start`, `just setup-dev start --ollama-in-stack`, `just setup-dev full-demo --stop-on-success`.

Per-component: `just setup-dev component node-manager restart`, `just setup-dev component control-plane rebuild --force`.

## E2E Test Suite

The Python E2E suite lives in [test_scripts/](test_scripts/).
It exercises the user-gateway (auth, tasks, models, chat) and control-plane (node registration, capability) via the cynork CLI and curl.
Tests are discovered from all `e2e_*.py` modules; see [test_scripts/README.md](test_scripts/README.md) for layout, execution order, and adding tests.
Run the suite with `just e2e` (stack must already be running) or as part of `just setup-dev full-demo`.

## Justfile Entry Points

- **just setup-dev** \<command\> [ARGS] - Python dev setup; e.g. `just setup-dev start`, `just setup-dev full-demo --stop-on-success`, `just setup-dev clean-db`, `just setup-dev stop`.
- **just e2e** [ARGS] - Python E2E suite; requires stack already running.
  Pass `--no-build`, `-v`, `-k PATTERN`, etc.

Use `just setup-dev full-demo --stop-on-success` to start the stack, run E2E, and tear down on pass.
Use `just setup-dev stop` to stop all services; `just setup-dev clean-db` to remove the Postgres container and volume.

## Environment

**Repo-root `.env.dev`:** Not committed (see `.gitignore`).
Create it at repo root if you use `just setup-dev start` or `./scripts/dev_stack.sh . start`; it is sourced by `dev_stack.sh` and the scripts/justfile.
Define `CYNODE_SECURE_STORE_MASTER_KEY_B64` (base64-encoded 32-byte key) for the dev secure store (node-manager/worker-api).

See `just setup-dev help` for the full list.

### Key Environment Variables

- **Variable:** `POSTGRES_*`
  - default: various
  - description: Container name, port 5432, user/password/db, image (pgvector/pgvector:pg16).
- **Variable:** `ORCHESTRATOR_PORT`
  - default: 12080
  - description: User-gateway (API) port.
- **Variable:** `CONTROL_PLANE_PORT`
  - default: 12082
  - description: Control-plane (admin/registration) port.
- **Variable:** `WORKER_PORT`
  - default: 12090
  - description: Worker-api (node) port.
- **Variable:** `NODE_PSK`
  - default: dev-node-psk-secret
  - description: Node registration pre-shared key.
- **Variable:** `ADMIN_PASSWORD`
  - default: admin123
  - description: Bootstrap admin password.
- **Variable:** `CYNODE_SECURE_STORE_MASTER_KEY_B64`
  - default: (from repo root `.env.dev`)
  - description: Base64-encoded 32-byte master key for node-manager/worker-api secure store.
- **Variable:** `CYNODEAI_LOGS_DIR`
  - default: `$TMPDIR/.../cynodeai-setup-dev-logs`
  - description: Log directory.
    Node-manager log: `cynodeai-wnm.log`.
- **Variable:** `SETUP_DEV_OLLAMA_IN_STACK`
  - default: (unset)
  - description: Set to `1` to enable OLLAMA profile.
    Same as `--ollama-in-stack`.
  - **AI agents must NOT use** this bypass; node-manager must start Ollama for GPU variant validation.
- **Variable:** `STOP_ON_SUCCESS_ENV`
  - default: (unset)
  - description: Set to `1` for full-demo to tear down after E2E pass.
- **Variable:** `INFERENCE_PROXY_IMAGE`, `OLLAMA_UPSTREAM_URL`
  - default: (dev defaults)
  - description: Used by E2E and node inference.

**Logs and state:** Node-manager PID file: `$TMPDIR/cynodeai-node-manager.pid`.
Worker-api state: `$TMPDIR/cynodeai-node-state`.
Logs (including node-manager stdout/stderr) under `CYNODEAI_LOGS_DIR`.
Ports and endpoints: [docs/tech_specs/ports_and_endpoints.md](docs/tech_specs/ports_and_endpoints.md).

## Troubleshooting

- **"node-manager not found"** – Run `just build-dev` so `worker_node/bin/cynodeai-wnm-dev` exists.
- **Compose images missing** – For a full run, use `just setup-dev full-demo` once (builds images).
  For `just setup-dev start` only, build images first: `just orchestrator/build-control-plane-image`, `just orchestrator/build-user-gateway-image`, `just agents/build-cynode-pma-image`.
- **Port in use** – Change `WORKER_PORT`, `CONTROL_PLANE_PORT`, or `ORCHESTRATOR_PORT`; ensure no other stack is using them.
- **Orchestrator not ready** – Check control-plane logs (compose or `CYNODEAI_LOGS_DIR`).
  Ensure Postgres is up and migrations have run (they run on control-plane startup).
- **Direct script use** – From repo root: `./scripts/dev_stack.sh . start-db`, `./scripts/dev_stack.sh . start`, `./scripts/dev_stack.sh . stop`.
  Third argument for `start`/`restart`: non-empty enables OLLAMA profile (e.g. `./scripts/dev_stack.sh . start 1`).

## Lint

- **just lint-python** - Runs flake8, pylint, and other Python linters on default paths (`scripts`, `.ci_scripts`).
  To include `scripts/test_scripts`, pass paths: `just lint-python paths="scripts,.ci_scripts,scripts/test_scripts"` or run flake8/pylint directly on `scripts/test_scripts` as in [test_scripts/README.md](test_scripts/README.md#lint).
- **just lint-md** - Lint Markdown; use `just lint-md scripts/README.md` to fix this file.
