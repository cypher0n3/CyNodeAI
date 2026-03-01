# Setup-Dev: Python vs Bash Parity Report

- [Objective](#objective)
- [Commands Compared](#commands-compared)
- [E2E Test Parity](#e2e-test-parity)
- [Gaps Addressed](#gaps-addressed)
- [Behavioral Differences (Acceptable)](#behavioral-differences-acceptable)
- [Justfile Targets That Invoke setup-dev.sh](#justfile-targets-that-invoke-setup-devsh)
- [How to Exercise Python Path](#how-to-exercise-python-path)
- [Conclusion](#conclusion)

## Objective

**Date:** 2026-02-28

Ensure the Python dev setup (`scripts/setup_dev.py`, `scripts/setup_dev_config.py`, `scripts/setup_dev_impl.py`) and the Python E2E suite (`scripts/test_scripts/run_e2e.py` + `e2e_*.py`) fully replace `scripts/setup-dev.sh` so the bash script and its justfile-only targets can be removed.

## Commands Compared

- **start-db:** Bash `setup-dev.sh`: start_postgres.
  Python: start_postgres.
  Parity: Yes.
- **stop-db:** Bash: stop_postgres.
  Python: stop_postgres.
  Parity: Yes.
- **clean-db:** Bash: clean_postgres.
  Python: clean_postgres.
  Parity: Yes.
- **migrate:** Bash: no-op.
  Python: no-op.
  Parity: Yes.
- **build:** Bash: just build.
  Python: just build.
  Parity: Yes.
- **build-e2e-images:** Bash: ensure_*_build_if_delta (inference-proxy, cynode-sba).
  Python: build_e2e_images (builds both).
  Parity: Yes (Python always builds; bash uses hash cache).
- **start:** Bash: build, compose up, wait control-plane, start node.
  Python: Same flow in cmd_start.
  Parity: Yes.
- **stop:** Bash: kill node-manager, fuser/lsof port, compose down, rm containers.
  Python: stop_all same logic.
  Parity: Yes.
- **test-e2e:** Bash: Inline run_e2e_test (cynork + curl).
  Python: run_python_e2e -> run_e2e.py.
  Parity: Yes.
- **full-demo:** Bash: build, e2e images, start, run_e2e_test, optional stop.
  Python: Same in cmd_full_demo.
  Parity: Yes.
- **help:** Bash: show_usage.
  Python: show_help.
  Parity: Yes.

## E2E Test Parity

Bash `run_e2e_test` steps vs Python parity suite:

- **1 Login:** Bash: cynork auth login.
  Python: e2e_01_login.
  Yes.
- **2 Whoami:** Bash: cynork auth whoami, expect handle=admin.
  Python: e2e_02_whoami.
  Yes.
- **3 Task create:** Bash: cynork task create -p "echo Hello...".
  Python: e2e_03_task_create.
  Yes.
- **4 Task get:** Bash: cynork task get $TASK_ID.
  Python: e2e_04_task_get.
  Yes.
- **5 Task result:** Bash: cynork task result $TASK_ID.
  Python: e2e_05_task_result.
  Yes.
- **5b Inference:** Bash: iff INFERENCE_PROXY_IMAGE set; create --use-inference, poll result, assert OLLAMA_BASE_URL in stdout.
  Python: e2e_05b_inference_task (skipTest if unset).
  Yes.
- **5c Prompt:** Bash: create prompt task, poll, assert non-empty stdout.
  Python: e2e_05c_prompt_task.
  Yes.
- **5d Models + chat:** Bash: models list; one-shot chat (skip if E2E_SKIP_INFERENCE_SMOKE).
  Python: e2e_05d_models_and_chat.
  Yes.
- **5e SBA:** Bash: create --use-sba, poll, assert sba_result in job result.
  Python: e2e_05e_sba_task.
  Yes (fixed: accept result from .stdout when .jobs absent).
- **6 Node register:** Bash: curl POST control-plane /v1/nodes/register.
  Python: e2e_06_node_register.
  Yes.
- **7 Capability:** Bash: curl POST /v1/nodes/capability with node JWT.
  Python: e2e_07_capability.
  Yes.
- **8 Refresh:** Bash: cynork auth refresh, whoami.
  Python: e2e_08_refresh.
  Yes.
- **9 Logout:** Bash: cynork auth logout.
  Python: e2e_09_logout.
  Yes.

Ollama inference smoke: bash runs it before tests.
Python run_e2e.py runs helpers.run_ollama_inference_smoke() before the suite.
Both skip when E2E_SKIP_INFERENCE_SMOKE is set or Ollama container absent.

## Gaps Addressed

One parity fix was applied in the Python E2E suite.

### SBA Result Shape

Bash accepts job result from `.jobs[0].result` or from `.stdout` when `.jobs` is absent (cynork sometimes returns worker response in `.stdout`).
Python e2e_05e_sba_task previously only checked `.jobs[0].result`.
**Fixed:** e2e_05e now falls back to parsing `.stdout` as JSON and asserting `sba_result` there.

## Behavioral Differences (Acceptable)

- **Stack images:** Bash pre-builds control-plane, user-gateway, cynode-pma with hash-based cache (`ensure_stack_images_build_if_delta`) before compose up.
  Python runs `compose up` only; the compose file has `build:` so Compose builds images when missing.
  No functional gap; bash is a speed optimization.
- **E2E image cache:** Bash uses `E2E_IMAGE_CACHE_DIR` and content hash to skip rebuilding inference-proxy and cynode-sba when unchanged.
  Python `build_e2e_images()` always runs `podman/docker build`.
  Slower on repeated full-demos, same outcome.

## Justfile Targets That Invoke `setup-dev.sh`

To remove the bash script, point these to Python:

- **clean-db:** Use `just setup-dev clean-db`.
- **e2e:** Python E2E suite only; use `just e2e` (stack must be up).
  Full demo (start stack and run E2E): `just setup-dev full-demo` or `just setup-dev full-demo --stop-on-success`.
- **e2e-stop:** Use `just setup-dev stop`.

The recipe `setup-dev CMD *ARGS` already runs `python3 scripts/setup_dev.py {{ CMD }} {{ ARGS }}`.
No change needed there.

## How to Exercise Python Path

From repo root:

```bash
# Build only
just setup-dev build

# Start DB only (standalone postgres container)
just setup-dev start-db

# Full demo (build, e2e images, start stack+node, run E2E, optional stop)
just setup-dev full-demo --stop-on-success

# Run E2E only (services must already be up)
just setup-dev test-e2e

# Or run Python E2E suite directly with options
just e2e --no-build --skip-ollama   # etc.
```

## Conclusion

- Python setup and E2E suite **replace** all functionality of `scripts/setup-dev.sh` and its inline E2E test.
- One parity fix was applied: SBA task result can come from `.stdout` when `.jobs` is absent.
- **Current state:** The justfile uses Python only: `just e2e` runs the Python E2E suite; `just setup-dev clean-db` and `just setup-dev stop` replace the former clean-db and e2e-stop targets.
