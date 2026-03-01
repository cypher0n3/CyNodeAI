# E2E Test Suite (Python)

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Run E2E Suite](#run-e2e-suite)
  - [Run Options](#run-options)
  - [Via Just](#via-just)
- [Test Layout](#test-layout)
- [Execution Order and State](#execution-order-and-state)
- [Environment](#environment)
- [Adding Tests](#adding-tests)
- [Troubleshooting](#troubleshooting)
- [Lint](#lint)

## Overview

Python-based E2E tests that exercise the user-gateway (auth, tasks, models, chat) and the control-plane (node registration, capability) via the cynork CLI and curl.
They mirror the flow in `scripts/setup-dev.sh` `run_e2e_test` but run as unittest; the suite is independent of the bash scripts.
Tests are discovered from all `e2e_*.py` modules in this directory.
Standard library only plus subprocess (cynork, curl); no extra Python deps.

## Prerequisites

- **Stack running:** orchestrator (compose), node (node-manager + worker-api).
  Start with `just setup-dev start` or `just setup-dev full-demo` (which runs the suite after start).
- **Cynork:** `just build-cynork-dev` (or let `run_e2e.py` build it unless you pass `--no-build`).
- **Python 3:** run from repo root with `PYTHONPATH=.` so `scripts.test_scripts` resolves.

## Run E2E Suite

From repo root the runner discovers all `e2e_*.py` in this directory:

```bash
PYTHONPATH=. python scripts/test_scripts/run_e2e.py
```

### Run Options

- `--help` - show usage and flags
- `--no-build` - skip building cynork-dev; use existing binary (faster re-runs)
- `--skip-ollama` - skip Ollama inference smoke and one-shot chat (sets `E2E_SKIP_INFERENCE_SMOKE`)
- `--list` - list test names and exit (no run)
- Unittest pass-through: `-k PATTERN` (filter tests), `-v` (verbosity), `-f` (failfast)

Examples:

```bash
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --no-build
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --list
PYTHONPATH=. python scripts/test_scripts/run_e2e.py -k test_login
PYTHONPATH=. python scripts/test_scripts/run_e2e.py -k test_05 -v
```

### Via Just

- `just e2e` - run the Python E2E suite (stack must already be up).
  Pass options: `just e2e --no-build`, `just e2e -v`, etc.
- `just setup-dev test-e2e` - run the suite via `scripts/setup_dev.py` (same as above, ensures PYTHONPATH).
- `just setup-dev full-demo` - start stack and node, then run the suite; use `--stop-on-success` to tear down after pass.

## Test Layout

- **run_e2e.py** - Entrypoint; discovers `e2e_*.py`, waits for gateway and Ollama smoke, then runs unittest.
- **config.py** - Ports, URLs, `CYNORK_BIN`, env flags (no non-stdlib deps).
- **helpers.py** - `run_cynork()`, `run_curl()`, `wait_for_gateway()`, `run_ollama_inference_smoke()`, JSON/state helpers.
- **e2e_state.py** - Shared state: `CONFIG_DIR`, `CONFIG_PATH`, `TASK_ID`, `NODE_JWT`, etc.; set by tests, cleaned by logout.

Test modules (one main test per file; order depends on discovery):

- **e2e_01_login** - Auth login (admin); creates temp config dir and writes token to `state.CONFIG_PATH`.
- **e2e_02_whoami** - Auth whoami; asserts handle=admin.
- **e2e_03_task_create** - Create task (echo); sets `state.TASK_ID`.
- **e2e_04_task_get** - Get task by ID.
- **e2e_05_task_result** - Get task result; asserts status present.
- **e2e_05b_inference_task** - Create task with `--use-inference`; asserts `OLLAMA_BASE_URL` in stdout (skipped if `INFERENCE_PROXY_IMAGE` unset).
- **e2e_05c_prompt_task** - Prompt (LLM) task; asserts non-empty stdout.
- **e2e_05d_models_and_chat** - Models list; one-shot chat (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_05e_sba_task** - SBA task; asserts job result contains `sba_result`.
- **e2e_06_node_register** - POST control-plane `/v1/nodes/register`; sets `state.NODE_JWT`.
- **e2e_07_capability** - POST control-plane `/v1/nodes/capability` with node JWT.
- **e2e_08_refresh** - Auth refresh and whoami.
- **e2e_09_logout** - Auth logout; cleans `state.CONFIG_DIR`.

## Execution Order and State

Discovery order is determined by unittest (typically alphabetical by module name).
Several tests depend on shared state: login (01) creates the config and token; later tests use `state.CONFIG_PATH` and task/JWT IDs set by earlier tests; logout (09) removes the config dir.
Running a single test in isolation (e.g. `-k test_task_create`) will fail if it expects `state.TASK_ID` or `state.CONFIG_PATH` from a prior test; run the full suite or a contiguous subset (e.g. login through the test you care about).

## Environment

Same as `scripts/setup-dev.sh`; see also `docs/tech_specs/ports_and_endpoints.md`.

- **Ports:** `ORCHESTRATOR_PORT` (default 12080), `CONTROL_PLANE_PORT` (12082)
- **Auth/node:** `ADMIN_PASSWORD` (default admin123), `NODE_PSK` (default dev-node-psk-secret)
- **Inference:** `E2E_SKIP_INFERENCE_SMOKE` - set to skip Ollama pull and inference smoke; `INFERENCE_PROXY_IMAGE` - set to run inference-in-sandbox (05b) and prompt/chat (05c, 05d)
- **Overrides:** `CYNORK_BIN`, `PROJECT_ROOT`, `OLLAMA_CONTAINER_NAME`, `OLLAMA_E2E_MODEL`

## Adding Tests

1. Add a new module `e2e_<name>.py` in `scripts/test_scripts/` with unittest `TestCase` classes.
2. The runner discovers all `e2e_*.py`; no registration needed.
3. Use `from scripts.test_scripts import config, helpers` and `import scripts.test_scripts.e2e_state as state`.
4. If the test needs auth or task state, run after login/task-create in the same run (or document the required order).
5. Use `helpers.run_cynork(...)` for cynork CLI and `helpers.run_curl(...)` for control-plane HTTP; use `state.CONFIG_PATH` for cynork config when auth is required.

## Troubleshooting

- **"user-gateway not ready (healthz) after 30s"** - Start the stack first (`just setup-dev start` or `just setup-dev full-demo`); ensure nothing else is bound to `ORCHESTRATOR_PORT`.
- **"cynork-dev not found"** - Run `just build-cynork-dev` or omit `--no-build` so the runner builds it.
- **"Ollama inference smoke failed"** - Ollama container must be running (e.g. from compose); or set `E2E_SKIP_INFERENCE_SMOKE=1` or pass `--skip-ollama`.
- **Test 05b skipped** - Set `INFERENCE_PROXY_IMAGE` (e.g. `cynodeai-inference-proxy:dev`) when starting the node so inference-in-sandbox is available.
- **Single test fails with missing state** - Run the full suite or include earlier tests (e.g. login, task-create) so shared state is set.

## Lint

`just lint-python` uses default paths `scripts,.ci_scripts` and does not include `scripts/test_scripts`.
To lint this directory from repo root:

```bash
PYTHONPATH=. flake8 scripts/test_scripts --max-line-length=100
PYTHONPATH=. pylint --rcfile=.pylintrc scripts/test_scripts
```
