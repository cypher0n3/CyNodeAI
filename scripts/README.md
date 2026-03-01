# Scripts

- [Overview](#overview)
- [Directory layout](#directory-layout)
- [Python dev setup](#python-dev-setup)
- [E2E test suite](#e2e-test-suite)
- [Justfile entry points](#justfile-entry-points)
- [Environment](#environment)
- [Lint](#lint)

## Overview

The `scripts/` directory provides development environment setup and E2E testing.
Prefer **justfile** entry points (`just setup-dev`, `just e2e`) over invoking scripts directly.
Python dev setup replaces bash for all commands; the bash script remains for reference but is not required.

## Directory Layout

- **setup_dev.py**, **setup_dev_config.py**, **setup_dev_impl.py** - Python dev setup (no bash dependency).
  Same commands as `setup-dev.sh`; see [Python dev setup](#python-dev-setup).
- **test_scripts/** - Python E2E suite (unittest; discovers `e2e_*.py`).
  See [test_scripts/README.md](test_scripts/README.md) for test layout, state, and how to add tests.
- **setup-dev.sh** - Legacy bash dev setup (start-db, build, start, stop, test-e2e, full-demo).
  Kept for reference; use `just setup-dev` with Python instead.
- **requirements-lint.txt** - Python lint tooling for `just venv` and `just lint-python`.
- **podman-generate-units.sh** - Optional/auxiliary; see justfile for primary workflows.

## Python Dev Setup

Run from repo root with `PYTHONPATH=.` so the `scripts` package resolves.
Use **just setup-dev** so PYTHONPATH is set for you.

Commands (same as `setup-dev.sh`):

- **start-db** - Start standalone Postgres container (podman/docker).
- **stop-db** - Stop the Postgres container.
- **clean-db** - Stop and remove the Postgres container and volume.
- **migrate** - No-op; migrations run when control-plane starts.
- **build** - Run `just build` (orchestrator, worker_node, cynork, agents binaries).
- **build-e2e-images** - Build inference-proxy and cynode-sba images for E2E.
- **start** - Build, compose up (orchestrator stack), wait for control-plane, start node-manager.
- **stop** - Kill node-manager, free worker port, compose down, remove containers.
- **restart** - Stop all then start (same as stop + start).
- **clean** - Stop all services and remove postgres container/volume.
- **test-e2e** - Run the Python E2E suite ([test_scripts/run_e2e.py](test_scripts/run_e2e.py)); stack must be up.
- **full-demo** - Build, build E2E images, start stack and node, run E2E suite; optionally stop on success.
  Use `--stop-on-success` or `STOP_ON_SUCCESS_ENV=1` to tear down after tests pass.
- **help** - Show usage and environment variables.

Examples:

```bash
PYTHONPATH=. python scripts/setup_dev.py build
PYTHONPATH=. python scripts/setup_dev.py full-demo --stop-on-success
```

Or via just: `just setup-dev build`, `just setup-dev full-demo --stop-on-success`.

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
Common ones: `POSTGRES_PORT`, `ORCHESTRATOR_PORT`, `CONTROL_PLANE_PORT`, `ADMIN_PASSWORD`, `NODE_PSK`, `WORKER_PORT`, `E2E_FORCE_REBUILD`, `STOP_ON_SUCCESS_ENV`, `INFERENCE_PROXY_IMAGE`, `OLLAMA_UPSTREAM_URL`.
Ports and endpoints are documented in [docs/tech_specs/ports_and_endpoints.md](docs/tech_specs/ports_and_endpoints.md).

## Lint

- **just lint-python** - Runs flake8, pylint, and other Python linters on default paths (`scripts`, `.ci_scripts`).
  To include `scripts/test_scripts`, pass paths: `just lint-python paths="scripts,.ci_scripts,scripts/test_scripts"` or run flake8/pylint directly on `scripts/test_scripts` as in [test_scripts/README.md](test_scripts/README.md#lint).
- **just lint-md** - Lint Markdown; use `just lint-md scripts/README.md` to fix this file.
