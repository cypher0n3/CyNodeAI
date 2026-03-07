# E2E Test Suite (Python)

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Run E2E Suite](#run-e2e-suite)
  - [Run Options](#run-options)
  - [Via Just](#via-just)
- [Test Layout](#test-layout)
  - [Numbering Convention](#numbering-convention)
  - [Test Modules (Run Order)](#test-modules-run-order)
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
  Default startup uses the **prescribed sequence** (orchestrator without OLLAMA in stack; PMA via orchestrator/worker).
  If the stack does not reach ready (e.g. worker-managed PMA not yet implemented), use bypasses: `just setup-dev start --ollama-in-stack --pma-via-compose` or set `SETUP_DEV_OLLAMA_IN_STACK=1` and `SETUP_DEV_PMA_VIA_COMPOSE=1`.
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
- `--tags TAG1,TAG2` - run only tests that have at least one of these tags
- `--exclude-tags TAG1,TAG2` - exclude tests that have any of these tags
- Unittest pass-through: `-k PATTERN` (filter tests), `-v` (verbosity), `-f` (failfast)

Tags: `suite_*` (suite_orchestrator, suite_worker_node, suite_agents, suite_cynork, suite_proxy_pma), `full_demo` (run during `just setup-dev full-demo`; excludes subset-only tests), `inference`, `pma_inference`, `sba_inference`, `auth`, `task`, `chat`, `worker`, `pma`.

Examples:

```bash
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --no-build
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --list
PYTHONPATH=. python scripts/test_scripts/run_e2e.py -k test_login
PYTHONPATH=. python scripts/test_scripts/run_e2e.py -k test_05 -v
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --tags full_demo
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --tags inference
PYTHONPATH=. python scripts/test_scripts/run_e2e.py --tags suite_proxy_pma
```

### Via Just

- `just e2e` - run the Python E2E suite (stack must already be up).
  Pass options: `just e2e --no-build`, `just e2e -v`, etc.
- `just setup-dev test-e2e` - run the suite via `scripts/setup_dev.py` (same as above, ensures PYTHONPATH).
- `just setup-dev full-demo` - start stack and node, then run only tests tagged `full_demo` (excludes subset-only tests such as proxy+PMA functional tests that start their own services); use `--stop-on-success` to tear down after pass.
  - For E2E that expects OLLAMA and PMA from compose, use `just setup-dev full-demo --ollama-in-stack --pma-via-compose`.

## Test Layout

- **run_e2e.py** - Entrypoint; discovers `e2e_*.py`, waits for gateway and Ollama smoke, then runs unittest.
- **config.py** - Ports, URLs, `CYNORK_BIN`, env flags (no non-stdlib deps).
- **helpers.py** - `run_cynork()`, `run_curl()`, `wait_for_gateway()`, `run_ollama_inference_smoke()`, JSON/state helpers.
- **e2e_state.py** - Shared state: `CONFIG_DIR`, `CONFIG_PATH`, `TASK_ID`, `NODE_JWT`, etc.; set by tests, cleaned by logout.

### Numbering Convention

Modules are named `e2e_NNN_descriptive_name.py` with **zero-padded NNN in steps of 10** (010, 020, 030, ...).
Alphabetical order of module names is the run order.
Gaps (e.g. 011-019 between 010 and 020) allow inserting new tests without renumbering.

### Test Modules (Run Order)

- **e2e_010_cli_version_and_status** - Cynork version and status (gateway health); no auth.
- **e2e_020_auth_login** - Auth login (admin); creates temp config dir and writes token to `state.CONFIG_PATH`.
- **e2e_030_auth_negative_whoami** - Whoami without login fails (negative test).
- **e2e_040_auth_whoami** - Auth whoami; asserts handle=admin.
- **e2e_050_task_create** - Create task (echo); sets `state.TASK_ID`.
- **e2e_060_task_list** - Task list JSON; asserts tasks array and created task present.
- **e2e_070_task_get** - Get task by ID.
- **e2e_080_task_result** - Get task result; asserts status present.
- **e2e_090_task_inference** - Create task with `--use-inference` (skipped if `INFERENCE_PROXY_IMAGE` unset).
- **e2e_100_task_prompt** - Prompt (LLM) task; asserts non-empty stdout.
- **e2e_110_task_models_and_chat** - Models list; one-shot chat (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_115_pma_chat_context** - One-shot chat with `--project-id` (OpenAI-Project header); PMA handoff path (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_116_skills_gateway** - Skills list, load (from file), get by id, delete via cynork against user-gateway; requires auth (e2e_020).
- **e2e_123_sba_task** - SBA task; asserts job result contains `sba_result`; sets `state.SBA_TASK_ID`.
- **e2e_130_sba_task_result_contract** - SBA result shape (protocol_version, job_id, status, steps, artifacts).
  Requires state.SBA_TASK_ID from e2e_123.
- **e2e_140_sba_task_inference** - SBA task with inference prompt (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_145_sba_inference_reply** - SBA + inference prompt "Reply back with the current time."; asserts user-facing reply (not empty stdout / sba-run only); skipped if `E2E_SKIP_INFERENCE_SMOKE`.
- **e2e_150_task_logs** - Task logs for `state.TASK_ID`.
- **e2e_160_task_cancel** - Create task, cancel with `-y`, assert terminal status canceled.
- **e2e_170_control_plane_node_register** - POST `/v1/nodes/register`; sets `state.NODE_JWT`.
- **e2e_180_control_plane_capability** - POST `/v1/nodes/capability` with node JWT.
- **e2e_190_auth_refresh** - Auth refresh and whoami.
- **e2e_192_chat_reliability** - One-shot chat with extended timeout and retries; assert timely reply or clear error (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_193_chat_sequential_messages** - Two-turn chat via POST /v1/chat/completions; assert both replies (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_194_chat_simultaneous_messages** - Three concurrent chat requests; assert at least one succeeds with non-empty reply (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_195_gateway_health_readyz** - GET /healthz and /readyz; assert 200 or 503 per spec.
- **e2e_196_task_list_status_filter** - Task list with --status completed/queued; assert JSON shape and status enum.
- **e2e_200_auth_logout** - Auth logout; cleans `state.CONFIG_DIR`.

## Execution Order and State

Discovery order is alphabetical by module name (010, 020, ... 200).
Several tests depend on shared state: login (020) creates the config and token; later tests use `state.CONFIG_PATH` and task/JWT IDs set by earlier tests; logout (200) removes the config dir.
Running a single test in isolation (e.g. `-k test_task_create`) will fail if it expects `state.TASK_ID` or `state.CONFIG_PATH` from a prior test; run the full suite or a contiguous subset.

## Environment

Same as `scripts/setup_dev.py`; see also `docs/tech_specs/ports_and_endpoints.md`.

- **Ports:** `ORCHESTRATOR_PORT` (default 12080), `CONTROL_PLANE_PORT` (12082)
- **Auth/node:** `ADMIN_PASSWORD` (default admin123), `NODE_PSK` (default dev-node-psk-secret)
- **Inference:** `E2E_SKIP_INFERENCE_SMOKE` - set to skip Ollama pull and inference smoke; `INFERENCE_PROXY_IMAGE` - set to run inference-in-sandbox (05b) and prompt/chat (05c, 05d)
- **Overrides:** `CYNORK_BIN`, `PROJECT_ROOT`, `OLLAMA_CONTAINER_NAME`, `OLLAMA_E2E_MODEL`
- **Setup-dev bypasses** (when starting the stack): `SETUP_DEV_OLLAMA_IN_STACK=1`, `SETUP_DEV_PMA_VIA_COMPOSE=1` so OLLAMA and PMA run in compose for E2E.

## Adding Tests

1. Add a new module `e2e_NNN_descriptive_name.py` in `scripts/test_scripts/` with unittest `TestCase` classes.
   Use a number between two existing tests for the desired run position (e.g. 015 between 010 and 020).
   If adding at the end, use at least 10 above the current last test (e.g. 210 when the last is 200).
   No need to renumber existing files.
2. The runner discovers all `e2e_*.py`; no registration needed.
3. Use `from scripts.test_scripts import config, helpers` and `import scripts.test_scripts.e2e_state as state`.
4. If the test needs auth or task state, run after the test that sets that state (or document the required order).
5. Use `helpers.run_cynork(...)` for cynork CLI and `helpers.run_curl(...)` for control-plane HTTP; use `state.CONFIG_PATH` for cynork config when auth is required.

## Troubleshooting

- **"user-gateway not ready (healthz) after 30s"** - Start the stack first (`just setup-dev start` or `just setup-dev full-demo`); ensure nothing else is bound to `ORCHESTRATOR_PORT`.
- **"Orchestrator not ready (readyz 200) after 120s"** - Default startup uses the prescribed sequence (PMA via orchestrator/worker).
  If that path is not yet implemented, use bypasses: `just setup-dev start --ollama-in-stack --pma-via-compose`.
- **"cynork-dev not found"** - Run `just build-cynork-dev` or omit `--no-build` so the runner builds it.
- **"Ollama inference smoke failed"** - Ollama must be running (e.g. start with `--ollama-in-stack` or set `SETUP_DEV_OLLAMA_IN_STACK=1`); or set `E2E_SKIP_INFERENCE_SMOKE=1` or pass `--skip-ollama`.
- **Test 090 (task_inference) skipped** - Set `INFERENCE_PROXY_IMAGE` (e.g. `cynodeai-inference-proxy:dev`) when starting the node so inference-in-sandbox is available.
- **Single test fails with missing state** - Run the full suite or include earlier tests (e.g. login, task-create) so shared state is set.

## Lint

`just lint-python` uses default paths `scripts,.ci_scripts` and does not include `scripts/test_scripts`.
To lint this directory from repo root:

```bash
PYTHONPATH=. flake8 scripts/test_scripts --max-line-length=100
PYTHONPATH=. pylint --rcfile=.pylintrc scripts/test_scripts
```
