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
  - [Prereq Setup](#prereq-setup)
- [Testing Standards](#testing-standards)
- [Environment](#environment)
- [Adding Tests](#adding-tests)
- [Troubleshooting](#troubleshooting)
- [Lint](#lint)

## Overview

Python-based E2E tests that exercise the user-gateway (auth, tasks, models, chat) and the control-plane (node registration, capability) via the cynork CLI and curl.
They target the same running services and ports you get from `just setup-dev start` or `just setup-dev full-demo`.
`just e2e` runs this suite by invoking `run_e2e.py`, which discovers and executes these tests with the standard library `unittest` harness (see [scripts/README.md](../README.md) for stack recipes).
Tests are discovered from all `e2e_*.py` modules in this directory.
Standard library only plus subprocess (cynork, curl); no extra Python deps.

## Prerequisites

- **Stack running:** orchestrator (compose) and node-manager on the host (`cynodeai-wnm-dev`), which embeds the Worker API in-process (same port as standalone worker-api in compose-only setups).
  Start with `just setup-dev start` or `just setup-dev full-demo` (which runs the suite after start).
  Default startup uses the **prescribed sequence** (orchestrator without OLLAMA in stack; PMA via orchestrator/worker).
  If the stack does not reach ready, use `--ollama-in-stack` or `SETUP_DEV_OLLAMA_IN_STACK=1` when OLLAMA in compose is needed.
  **AI agents must NOT use** these bypasses; they invalidate GPU variant E2E (e2e_0800).
- **Cynork:** `just build-cynork-dev` (or let `run_e2e.py` build it unless you pass `--no-build`).
- **Python 3:** run from repo root with `PYTHONPATH=.` so `scripts.test_scripts` resolves.

## Run E2E Suite

From repo root, prefer the justfile entrypoint:

```bash
just e2e
```

The underlying runner still discovers all `e2e_*.py` in this directory:

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

Tags: `suite_*` (suite_orchestrator, suite_worker_node, suite_agents, suite_cynork, suite_proxy_pma), `full_demo`, `inference`, `no_inference`, `pma_inference`, `sba_inference`, `auth`, `task`, `chat`, `worker`, `pma`.
Logical groups: `tui` (all TUI/PTY tests), `streaming` (SSE, gateway, transport, TUI streaming), `control_plane` (node register, capability, workflow), `sba` (all SBA tests), `gateway` (gateway health and streaming contract), `uds_routing` (inference proxy UDS), `suite_worker_node` (all worker/node tests).

#### E2E Invocation Examples

```bash
just e2e --no-build
just e2e --list
just e2e -k test_login
just e2e -k test_05 -v
just e2e --tags full_demo
just e2e --tags no_inference
just e2e --tags inference
just e2e --tags tui
just e2e --tags streaming
just e2e --tags sba
just e2e --tags suite_worker_node
just e2e --tags suite_proxy_pma
```

### Via Just

- `just e2e` - run the Python E2E suite (stack must already be up).
  Pass options: `just e2e --no-build`, `just e2e -v`, etc.
- `just setup-dev test-e2e` - run the suite via scripts/justfile (same as above, ensures PYTHONPATH).
- `just setup-dev full-demo` - start stack and node, then run only tests tagged `full_demo` (excludes subset-only tests such as proxy+PMA functional tests that start their own services); use `--stop-on-success` to tear down after pass.
  - For E2E that expects OLLAMA in compose, use `just setup-dev full-demo --ollama-in-stack`.
    **AI agents must NOT use** this; node-manager must start Ollama for GPU variant validation.

## Test Layout

- **run_e2e.py** - Entrypoint; discovers `e2e_*.py`, runs prereqs per-test (see [Prereq setup](#prereq-setup)), then runs unittest.
- **config.py** - Ports, URLs, `CYNORK_BIN`, env flags (no non-stdlib deps).
- **helpers.py** - `run_cynork()`, `run_curl()`, `wait_for_gateway()`, `run_ollama_inference_smoke()`, JSON/state helpers.
- **e2e_state.py** - Shared state: `CONFIG_DIR`, `CONFIG_PATH`, `TASK_ID`, `NODE_JWT`, etc.; set by tests, cleaned by logout.

### Numbering Convention

Modules are named `e2e_NNNN_descriptive_name.py` with **4-digit zero-padded NNNN in steps of 10** (0010, 0020, 0030, ...).
Alphabetical order of module names is the run order (fast/simple first, then task lifecycle, inference, streaming, SBA, TUI, teardown).
Gaps (e.g. 0011-0019 between 0010 and 0020) allow inserting new tests without renumbering.

### Test Modules (Run Order)

- **e2e_0010_cli_version_and_status** - Cynork version and status; no auth.
- **e2e_0020_gateway_health_readyz** - GET /healthz and /readyz; assert 200 or 503 per spec.
- **e2e_0030_auth_login** - Auth login acceptance coverage (`-u`/`--user` + `--password-stdin`); creates temp config dir and writes token to `state.CONFIG_PATH`.
- **e2e_0040_auth_negative_whoami** - Whoami without login fails (negative test).
- **e2e_0050_auth_whoami** - Auth whoami; asserts user=admin.
- **e2e_0300_worker_api_health_readyz** - Worker API healthz and readyz (process alive vs ready for jobs).
- **e2e_0310_worker_telemetry** - Worker API telemetry node:info and node:stats; requires WORKER_API and bearer.
- **e2e_0320_worker_api_managed_service** - Worker API (node-manager) healthz and node:info.
- **e2e_0330_node_manager_telemetry** - Telemetry logs for source_name=node_manager (node-manager lifecycle).
- **e2e_0340_uds_inference_routing** - UDS inference proxy routing coverage for worker-managed services and sandbox inference paths.
- **e2e_0380_control_plane_node_register** - POST `/v1/nodes/register`; sets `state.NODE_JWT`.
- **e2e_0390_control_plane_capability** - POST `/v1/nodes/capability` with node JWT.
- **e2e_0420_task_create** - Task create acceptance coverage for prompt mode plus canonical `--name` and `--attach`; sets `state.TASK_ID`.
- **e2e_0430_task_list** - Task list JSON; asserts tasks array and created task present.
- **e2e_0440_task_get** - Get task by ID.
- **e2e_0450_task_result** - Get task result; asserts status present.
- **e2e_0460_task_logs** - Task logs for `state.TASK_ID`; asserts `task_id`, `stdout`, and `stderr` in JSON output.
- **e2e_0470_task_cancel** - Create command-mode task (`--command`), cancel with `-y`, assert terminal status canceled.
- **e2e_0480_task_list_status_filter** - Task list with --status completed/queued; assert JSON shape and status enum.
- **e2e_0490_skills_gateway** - Skills list, load (from file), get by id, delete via cynork against user-gateway; requires auth (e2e_0030).
- **e2e_0500_workflow_api** - Control-plane workflow start/resume/checkpoint/release; uses `state.TASK_ID`.
- **e2e_0510_task_inference** - Create task with `--use-inference` (skipped if `INFERENCE_PROXY_IMAGE` unset).
- **e2e_0520_task_prompt** - Prompt (LLM) task; asserts non-empty stdout.
- **e2e_0530_task_models_and_chat** - Models list; one-shot chat (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_0540_chat_reliability** - One-shot chat with extended timeout and retries; assert timely reply or clear error (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_0550_chat_sequential_messages** - Two-turn chat via POST /v1/chat/completions; assert both replies (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_0560_chat_simultaneous_messages** - Three concurrent chat requests; assert at least one succeeds with non-empty reply (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_0570_pma_chat_context** - One-shot chat with `--project-id` (OpenAI-Project header); PMA handoff path (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_0580_pma_chat_capable_model** - Verifies the PMA chat path selects a chat-capable model when inference is enabled.
- **e2e_0610_sse_streaming** - SSE streaming for /v1/chat/completions and /v1/responses (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_0620_pma_standard_path_streaming** - PMA standard-path NDJSON streaming (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_0630_gateway_streaming_contract** - Gateway amendment, heartbeat, cancellation, persistence (skipped if events not produced).
- **e2e_0640_cynork_transport_streaming** - Cynork transport parsing and event propagation (suite_orchestrator, chat).
- **e2e_0650_tui_streaming_behavior** - TUI streaming behavior (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_0660_worker_pma_proxy** - Worker managed-service proxy and PMA handoff (suite_proxy_pma).
- **e2e_0710_sba_task** - SBA task; asserts job result contains `sba_result`; sets `state.SBA_TASK_ID`.
- **e2e_0720_sba_task_result_contract** - SBA result shape (protocol_version, job_id, status, steps, artifacts).
  Requires state.SBA_TASK_ID from e2e_0710.
- **e2e_0730_sba_task_inference** - SBA task with inference prompt (skipped if `E2E_SKIP_INFERENCE_SMOKE`).
- **e2e_0740_sba_inference_reply** - SBA + inference prompt "Reply back with the current time."; asserts user-facing reply; skipped if `E2E_SKIP_INFERENCE_SMOKE`.
- **e2e_0750_tui_pty** - TUI PTY harness; progressive visible-text updates, Ctrl+C cancel.
- **e2e_0760_tui_slash_commands** - TUI slash commands (/thread, /whoami, etc.).
- **e2e_0765_tui_composer_editor** - TUI composer footnote, multiline (Alt+Enter/Ctrl+J), Ctrl+Up history, login overlay; narrow PTY smoke.
- **e2e_0770_auth_refresh** - Auth refresh and whoami.
- **e2e_0780_auth_logout** - Auth logout; asserts tokens are cleared locally and authenticated access fails afterward.
- **e2e_0790_prescribed_startup_config_inference_backend** - Node config includes inference_backend when inference-capable.
- **e2e_0800_gpu_variant_ollama** - Ollama container image tag matches expected GPU variant (suite_worker_node, gpu_variant).

## Execution Order and State

Discovery order is alphabetical by module name.
Shared state (`state.CONFIG_PATH`, `state.TASK_ID`, `state.NODE_JWT`, `state.SBA_TASK_ID`) is established by **prereqs** or by **setUp/helpers** in the test, not by assuming a prior test ran.
Each test must be **atomic**: it may declare prereqs (run by the runner) or call helpers in setUp, but must not rely on the execution of other tests in the same run.

### Prereq Setup

Each test class declares `prereqs = ["gateway", "config", "auth", ...]` (see `e2e_tags.py`: `PREREQ_ORDER`, `PREREQ_ALWAYS_RERUN`).
The runner executes prereqs **per test** in order:

- **Succeeded prereqs** are not re-run for later tests, except those in `PREREQ_ALWAYS_RERUN` (e.g. **auth**), which run before every test that needs them to keep login token state correct.
- If a prereq step fails, it is recorded; any subsequent test that requires that prereq is **skipped** with a message like "Prereq(s) failed: gateway".

So gateway, config, task_id, ollama run once when first needed; auth runs before each test that declares it.
The **node_register** prereq runs control-plane node registration and sets `state.NODE_JWT` for capability/workflow tests.

## Testing Standards

- **Atomic tests:** Every test must be runnable in isolation (e.g. `just e2e --single e2e_0430_task_list`).
  Each run is a new process; shared state is empty and no earlier tests have run in that run.
- **No prior-test dependency:** Tests must not assume that another test (e.g. e2e_0420, e2e_0380, e2e_0710) has already run.
  Required state (e.g. `state.TASK_ID`, `state.NODE_JWT`, `state.SBA_TASK_ID`) must be established by:
  - **Prereqs** declared on the test class (e.g. `task_id`, `node_register`), which the runner runs before the test, or
  - **Helpers in setup** (e.g. `helpers.ensure_e2e_sba_task()` in e2e_0720).
- **Prereqs are the contract:** Use the whitelisted prereq names in `e2e_tags.py` (gateway, config, auth, node_register, task_id, ollama) so the runner can set up state once and skip tests when a prereq fails.

## Environment

Same as `just setup-dev` (scripts/justfile); see also `docs/tech_specs/ports_and_endpoints.md`.

- **Ports:** `ORCHESTRATOR_PORT` (default 12080), `CONTROL_PLANE_PORT` (12082)
- **Auth/node:** `ADMIN_PASSWORD` (default admin123), `NODE_PSK` (default dev-node-psk-secret)
- **Inference:** `E2E_SKIP_INFERENCE_SMOKE` - set to skip Ollama pull and inference smoke; `INFERENCE_PROXY_IMAGE` - set to run inference-in-sandbox (05b) and prompt/chat (05c, 05d)
- **Overrides:** `CYNORK_BIN`, `PROJECT_ROOT`, `OLLAMA_CONTAINER_NAME`, `OLLAMA_E2E_MODEL`
  - **Setup-dev bypass** (when starting the stack): `SETUP_DEV_OLLAMA_IN_STACK=1` so OLLAMA runs in compose for E2E.
    **AI agents must NOT use** this bypass.

## Adding Tests

1. Add a new module `e2e_NNNN_descriptive_name.py` in `scripts/test_scripts/` with unittest `TestCase` classes.
   Use a number between two existing tests for the desired run position (e.g. 0015 between 0010 and 0020).
   If adding at the end, use at least 10 above the current last test (e.g. 0790 when the last is 0780 (auth_logout)).
   No need to renumber existing files.
2. The runner discovers all `e2e_*.py`; no registration needed.
3. Use `from scripts.test_scripts import config, helpers` and `import scripts.test_scripts.e2e_state as state`.
4. If the test needs auth, task, node, or SBA state, declare the appropriate prereqs (see [Testing standards](#testing-standards)) or ensure state in setUp via helpers.
5. Use `helpers.run_cynork(...)` for cynork CLI and `helpers.run_curl(...)` for control-plane HTTP; use `state.CONFIG_PATH` for cynork config when auth is required.

## Troubleshooting

- **"user-gateway not ready (healthz) after 30s"** - Start the stack first (`just setup-dev start` or `just setup-dev full-demo`); ensure nothing else is bound to `ORCHESTRATOR_PORT`.
- **"Orchestrator not ready (readyz 200) after 120s"** - Default startup uses the prescribed sequence (PMA via orchestrator/worker).
  Use `--ollama-in-stack` when OLLAMA in compose is needed.
- **"cynork-dev not found"** - Run `just build-cynork-dev` or omit `--no-build` so the runner builds it.
- **"Ollama inference smoke failed"** - Ollama must be running (e.g. start with `--ollama-in-stack` or set `SETUP_DEV_OLLAMA_IN_STACK=1`); or set `E2E_SKIP_INFERENCE_SMOKE=1` or pass `--skip-ollama`.
- **Test 0510 (task_inference) skipped** - Set `INFERENCE_PROXY_IMAGE` (e.g. `cynodeai-inference-proxy:dev`) when starting the node so inference-in-sandbox is available.
- **Single test fails with missing state** - Run the full suite or include earlier tests (e.g. e2e_0030 login, e2e_0420 task-create) so shared state is set.

## Lint

`just lint-python` uses default paths `scripts,.ci_scripts` and does not include `scripts/test_scripts`.
To lint this directory from repo root:

```bash
PYTHONPATH=. flake8 scripts/test_scripts --max-line-length=100
PYTHONPATH=. pylint --rcfile=.pylintrc scripts/test_scripts
```
