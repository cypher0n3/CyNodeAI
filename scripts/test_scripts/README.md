# E2E Test Suite (Python)

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Run Parity Suite](#run-parity-suite)
- [Environment](#environment)
- [Adding Tests](#adding-tests)
- [Lint](#lint)

## Overview

Python-based E2E tests with parity to the bash flow in `scripts/setup-dev.sh` (`run_e2e_test`).
One test per script: `e2e_01_login.py` through `e2e_09_logout.py` (shared state in `e2e_state.py`).
The suite is independent of the bash scripts and does not modify them.

## Prerequisites

- Stack running (e.g. `./scripts/setup-dev.sh full-demo` or `start` then run tests).
- Cynork built: `just build-cynork-dev`.
- Python 3 with standard library only for the parity suite.

## Run Parity Suite

From repo root:

```bash
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --parity-only
```

Or run all e2e modules (parity + any other `e2e_*.py`):

```bash
PYTHONPATH=. python scripts/test_scripts/run_e2e.py
```

### Options (QoL)

- `--help` - show usage and flags
- `--parity-only` - run only the parity suite (default: discover all `e2e_*.py`)
- `--no-build` - skip building cynork-dev; use existing binary (faster re-runs)
- `--skip-ollama` - skip Ollama inference smoke and one-shot chat (set `E2E_SKIP_INFERENCE_SMOKE`)
- `--list` - list test names and exit (no run)
- Unittest pass-through: `-k PATTERN` (filter tests), `-v` (verbosity), `-f` (failfast)

Examples:

```bash
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --parity-only --no-build
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --list
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --parity-only -k test_01_login
```

## Environment

Same as `scripts/setup-dev.sh`:

- `ORCHESTRATOR_PORT` (default 12080), `CONTROL_PLANE_PORT` (12082)
- `ADMIN_PASSWORD` (default admin123), `NODE_PSK` (default dev-node-psk-secret)
- `E2E_SKIP_INFERENCE_SMOKE` - set to skip Ollama pull/inference smoke
- `INFERENCE_PROXY_IMAGE` - set to run inference-in-sandbox and prompt/chat tests
- `CYNORK_BIN` - override path to cynork-dev
- `PROJECT_ROOT` - override repo root
- `OLLAMA_CONTAINER_NAME`, `OLLAMA_E2E_MODEL` - for inference smoke

## Adding Tests

- Add a new module `e2e_<name>.py` in `scripts/test_scripts/` with unittest `TestCase` classes.
- Run with `python scripts/test_scripts/run_e2e.py` (no `--parity-only`) to include it.
- Use `scripts.test_scripts.config`, `scripts.test_scripts.helpers`, and `scripts.test_scripts.e2e_state` for shared state.

## Lint

`just lint-python` uses default paths `scripts,.ci_scripts` and passes without including `scripts/test_scripts`.
To lint `scripts/test_scripts` with the project linters (from repo root):

```bash
PYTHONPATH=. flake8 scripts/test_scripts --max-line-length=100
PYTHONPATH=. pylint --rcfile=.pylintrc scripts/test_scripts
```
