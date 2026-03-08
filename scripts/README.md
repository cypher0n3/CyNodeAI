# Scripts

- [Overview](#overview)
- [Directory Layout](#directory-layout)
- [Python Dev Setup](#python-dev-setup)
  - [Startup Sequence (Start / Full-Demo / Restart)](#startup-sequence-start--full-demo--restart)
  - [`setup-dev` Commands](#setup-dev-commands)
- [E2E Test Suite](#e2e-test-suite)
- [Justfile Entry Points](#justfile-entry-points)
- [Environment](#environment)
- [Lint](#lint)

## Overview

The `scripts/` directory provides development environment setup and E2E testing.
Prefer **justfile** entry points (`just setup-dev`, `just e2e`) over invoking scripts directly.
Python dev setup replaces bash for all commands; the bash script remains for reference but is not required.

## Directory Layout

- **setup_dev.py**, **setup_dev_config.py**, **setup_dev_impl.py**, **setup_dev_build_cache.py** - Python dev setup (no bash dependency).
  Same commands as `setup-dev.sh`; see [Python dev setup](#python-dev-setup).
  `setup_dev_build_cache.py` provides stamp/hash helpers for incremental compose and E2E image builds.
- **test_scripts/** - Python E2E suite (unittest; discovers `e2e_*.py`).
  See [test_scripts/README.md](test_scripts/README.md) for test layout, state, and how to add tests.
- **setup-dev.sh** - Legacy bash dev setup (start-db, build, start, stop, test-e2e, full-demo).
  Kept for reference; use `just setup-dev` with Python instead.
- **requirements-lint.txt** - Python lint tooling for `just venv` and `just lint-python`.
- **podman-generate-units.sh** - Optional/auxiliary; see justfile for primary workflows.

## Python Dev Setup

Run from repo root with `PYTHONPATH=.` so the `scripts` package resolves.
Use **just setup-dev** so PYTHONPATH is set for you.

### Startup Sequence (Start / Full-Demo / Restart)

When you run `just setup-dev start`, `full-demo`, or `restart`, the following run in order:

1. **Build binaries** - `just build-dev` (orchestrator, worker_node, cynork, agents).
2. **Compose up (orchestrator stack)** - Postgres, control-plane, user-gateway, optional profile (mcp-gateway, api-egress).
   OLLAMA and PMA are not started at this step unless bypasses are used.
3. **Start node** - Script runs the node-manager binary (worker_node/bin/node-manager-dev).
   The node-manager polls the orchestrator control-plane `/readyz` itself before registering (per worker_node startup procedure); the script does not wait for control-plane.
   Script then waits for worker-api `/healthz` up to 15s.
4. **PMA** - The worker node starts PMA when the orchestrator directs (orchestrator_bootstrap.md, worker_node managed services).
   Script waits for readyz.

Compose images (control-plane, user-gateway, cynode-pma) are not built by `start`; they are built by `full-demo` before this sequence.
For `just setup-dev start` to reach the node step, those images must already exist (e.g. run `full-demo` once, or build them separately).

### `setup-dev` Commands

- **start-db** - Start standalone Postgres container (podman/docker).
- **stop-db** - Stop the Postgres container.
- **clean-db** - Stop and remove the Postgres container and volume.
- **migrate** - No-op; migrations run when control-plane starts.
- **build** - Run `just build-dev` (orchestrator, worker_node, cynork, agents binaries).
- **build-e2e-images** - Build inference-proxy and cynode-sba images for E2E (incremental when unchanged; use `E2E_FORCE_REBUILD=1` or `SETUP_DEV_FORCE_BUILD=1` to force).
- **start** - Runs the full startup sequence above (build binaries, compose up, **start node** (node polls control-plane), wait for readyz).
  Use `--ollama-in-stack` (or `SETUP_DEV_OLLAMA_IN_STACK=1`) for OLLAMA in compose.
- **stop** - Kill node-manager, free worker port, compose down, remove containers.
- **restart** - Stop all then start (same as stop + start); accepts same bypass flags as start.
- **clean** - Stop all services and remove postgres container/volume.
- **test-e2e** - Run the Python E2E suite ([test_scripts/run_e2e.py](test_scripts/run_e2e.py)); stack must be up.
- **full-demo** - Build, build E2E images, start stack and node, run E2E suite; optionally stop on success.
  Use `--stop-on-success` or `STOP_ON_SUCCESS_ENV=1` to tear down after tests pass.
  Accepts same bypass flags as start; use `--ollama-in-stack` if E2E expects OLLAMA in compose.
- **help** - Show usage, bypass flags, and environment variables.

Examples:

```bash
PYTHONPATH=. python scripts/setup_dev.py build
PYTHONPATH=. python scripts/setup_dev.py start
PYTHONPATH=. python scripts/setup_dev.py start --ollama-in-stack
PYTHONPATH=. python scripts/setup_dev.py full-demo --stop-on-success
```

Or via just: `just setup-dev build`, `just setup-dev start`, `just setup-dev start --ollama-in-stack`, `just setup-dev full-demo --stop-on-success`.

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

Environment variables match `setup-dev.sh`; see `python scripts/setup_dev.py help` for the full list.
Common ones: `POSTGRES_PORT`, `ORCHESTRATOR_PORT`, `CONTROL_PLANE_PORT`, `ADMIN_PASSWORD`, `NODE_PSK`, `WORKER_PORT`, `STOP_ON_SUCCESS_ENV`, `INFERENCE_PROXY_IMAGE`, `OLLAMA_UPSTREAM_URL`.
Startup bypass (optional): `SETUP_DEV_OLLAMA_IN_STACK=1` (same as `--ollama-in-stack`).
Incremental builds: `E2E_FORCE_REBUILD=1` or `SETUP_DEV_FORCE_BUILD=1` to force rebuild of E2E and compose images.
Ports and endpoints are documented in [docs/tech_specs/ports_and_endpoints.md](docs/tech_specs/ports_and_endpoints.md).

## Lint

- **just lint-python** - Runs flake8, pylint, and other Python linters on default paths (`scripts`, `.ci_scripts`).
  To include `scripts/test_scripts`, pass paths: `just lint-python paths="scripts,.ci_scripts,scripts/test_scripts"` or run flake8/pylint directly on `scripts/test_scripts` as in [test_scripts/README.md](test_scripts/README.md#lint).
- **just lint-md** - Lint Markdown; use `just lint-md scripts/README.md` to fix this file.
