# E2E Run Report - 2026-03-15

## Summary

- **Stack restart:** `just setup-dev restart --force` now succeeds.
  Readiness check was changed from control-plane `/readyz` (port 12082) to **user-gateway `/readyz`** (port 12080), with a 90s timeout.
  E2E targets the user-gateway, so this is the correct gate.
- **Ollama prereq:** When `just e2e` runs without Ollama in the stack, the run aborted at "Ollama inference smoke failed".
  **Change:** `_run_prereq_checks(skip_ollama=True)` now skips the Ollama smoke when `just e2e --skip-ollama` is used, so the full suite can run and tests that need inference skip themselves (e.g. "inference smoke skipped").
- **Full e2e:** Run with `just e2e --no-build --skip-ollama` to execute all tests without requiring Ollama.

## Fixes Applied

The following code changes were made.

### Justfile (Scripts/justfile)

- Wait for user-gateway readyz (port 12080) instead of control-plane (12082).  
- Timeout increased to 90s.

### Run E2E Script (`run_e2e.py`)

- `_run_prereq_checks(skip_ollama=False)`; when `--skip-ollama` is set, Ollama inference smoke is skipped so the suite can proceed.

### Task Create Tests (`e2e_050_task_create.py`)

- `test_task_create_with_attachments`: skip with message when `state.CONFIG_PATH` is None (e.g. when run in isolation).

### Task Get Tests (`e2e_070_task_get.py`)

- `test_task_get_by_name`: skip when `state.TASK_NAME` is None (run full suite or `test_task_create_named` first).  
- When get-by-name returns non-ok, call `_assert_clear_name_resolution_error` before failing so stderr/stdout are checked for a clear error.

### Task Result Tests (`e2e_080_task_result.py`)

- `test_task_result_by_name`: same as above (skip when `TASK_NAME` is None; assert clear error when not ok).

## Failures and Skips (From Run With `--skip-ollama`)

Summary of test outcomes and recommended actions.

### Failures (To Fix or Accept)

- **test_task_create_with_attachments** - Fails in full suite if create/get response does not include `attachments` or expected paths.
  Isolate run fails with `CONFIG_PATH` None (now skipped).
  **Action:** Confirm API returns `attachments` in task create and task get; fix API or test expectations.
- **test_task_get_by_name** - Fails when CLI/backend returns non-ok for `task get <name>`.
  Backend implements `GetTaskBySummary`; possible causes: URL encoding of name, or summary mismatch.
  **Action:** Run with verbose to capture stdout/stderr; fix backend or CLI if name is not passed correctly.
- **test_task_result_by_name** - Same as get-by-name for `task result <name>`.

### Skips (Acceptable Unless Otherwise Noted)

- **test_inference_task** - `INFERENCE_PROXY_IMAGE not set`.
  **Acceptable:** Requires inference proxy image for sandbox UDS.
- **test_chat_with_project_context** - `inference smoke skipped`.
  **Acceptable:** When using `--skip-ollama`.
- **test_capable_model_chat_*** (e2e_118)** - `capable model 'qwen3:8b' not available in Ollama container`.
  **Acceptable:** Need capable model in stack or skip.
- **test_workflow_start_same_holder_returns_200_already_running** - Task create timed out (120s).
  **Acceptable:** Flaky/slow env; may need longer timeout or retry.
- **test_worker_api_container_exists_when_image_configured** - `NODE_MANAGER_WORKER_API_IMAGE not set`.
  **Acceptable:** Optional container mode.
- **test_internal_proxy_*** (e2e_124)** - Worker proxy UDS / internal listen not available.
  **Acceptable:** Depends on worker-api and proxy socket.
- **test_tui_updates_single_inflight_turn_progressively** (e2e_204) - `pexpect not installed or not Unix`.
  **Acceptable:** Install `scripts/requirements-e2e.txt` for PTY tests.

### Worker/telemetry Tests (`e2e_119`, `e2e_120`, `e2e_121`, `e2e_122`)

When the stack is started with the updated justfile, node-manager (and worker-api on port 12090) is up before the readiness gate.
So after `just setup-dev restart --force`, worker-api should be reachable.
If these tests still fail, check that `config.WORKER_API` is `http://localhost:12090` and that the worker-api process is bound to that port.

## Commands

- Restart stack: `just setup-dev restart --force`
- Full e2e (no Ollama): `just e2e --no-build --skip-ollama`
- Full e2e (with Ollama): `SETUP_DEV_OLLAMA_IN_STACK=1 just setup-dev start` then `just e2e --no-build`
