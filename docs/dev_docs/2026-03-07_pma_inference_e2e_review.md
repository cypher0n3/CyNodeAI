# PMA Inference E2E Test Review and Troubleshooting

## Summary

**Date:** 2026-03-07

- Reviewed and ran PMA inference E2E tests per copilot-instructions and meta.md.
- Fixed one failure in `suite_proxy_pma`: mock inference server port collision.
- All 8 tests in `suite_proxy_pma` now pass (7 OK, 1 expected skip).

## Test Layout

- **Tags:** `pma_inference`, `inference`, `suite_proxy_pma` (see `scripts/test_scripts/e2e_tags.py` and README).
- **PMA inference tests:**
  - `e2e_124_worker_pma_proxy.py`: proxy payload encoding, proxy->PMA functional, proxy+PMA with mock inference, proxy+PMA with real Ollama.
  - `e2e_110_task_models_and_chat.py`, `e2e_115_pma_chat_context.py`: require full stack (user-gateway healthz) and run via `just e2e --tags pma_inference` after `just setup-dev start` (or full-demo).

## Failures Observed and Fixes

Two failure modes and how they were addressed.

### 1. `just e2e --tags pma_inference` Without Stack

- **Symptom:** `Error: user-gateway not ready (healthz) after 30s`
- **Cause:** `pma_inference` includes tests that need the orchestrator stack (gateway, cynork).
  Prereq checks require gateway + Ollama smoke unless only `suite_proxy_pma` is requested.
- **Action:** For PMA inference tests that need the stack, run `just setup-dev start` (or full-demo) first, then `just e2e --tags pma_inference`.
  For proxy+PMA only, use `just e2e --tags suite_proxy_pma` (no gateway).

### 2. TestProxyPmaWithInference Setupclass: Address Already in Use

- **Symptom:** `OSError: [Errno 98] Address already in use` when binding mock inference server to `PROXY_PMA_TEST_MOCK_INFERENCE_PORT` (18092).
- **Cause:** Fixed port can stay in use from a previous run or another process.
- **Fix:** In `scripts/test_scripts/e2e_124_worker_pma_proxy.py`, start the mock inference server with port `0` (OS-assigned), then use `cls._mock_server.server_address[1]` for `mock_url`.
  No config or justfile changes.

## Run Commands Used

- **Proxy + PMA only (no stack):**  
  `just e2e --tags suite_proxy_pma -v`  
  or  
  `PYTHONPATH=. python3 scripts/test_scripts/run_e2e.py --tags suite_proxy_pma -v`
- **PMA inference with stack:**  
  Start stack (e.g. `just setup-dev start`), then  
  `just e2e --tags pma_inference -v`

## Results After Fix

```text
Ran 8 tests in 214.942s
OK (skipped=1)
```

- **Skipped (expected):** `test_proxy_forwards_chat_completion` - "PMA returned 500 (no inference in minimal env); proxy path is verified".
- **Passed:** Payload encoding (2), proxy healthz/bearer/404 (3), proxy->PMA->mock inference (1), proxy->PMA->real Ollama (1).

## Full `pma_inference` Run (With Stack)

Issues found when running with the full stack and fixes applied.

### 1. TypeError: Expected Str, Bytes or `os.PathLike` Object, Not NoneType (e2e_110, e2e_115)

- **Symptom:** In `run_cynork(..., state.CONFIG_PATH)`, subprocess received `None` (config path never set when only `pma_inference` tests run).
- **Fix:** In `e2e_110_task_models_and_chat.py` and `e2e_115_pma_chat_context.py`: `setUp()` now calls `state.init_config()` and runs `auth login` so config exists and has credentials when tests run in isolation.

### 2. 503 Service Unavailable on Chat (e2e_110, e2e_115) - Root Cause Fixed

- **Symptom:** Gateway returned 503 for chat (no PMA endpoint; node never reported PMA ready).
- **Root cause:** Control-plane had `PMA_ENABLED: false` in compose, so it never sent PMA in node config.
  Node therefore never started PMA.
- **Fixes (no test skips):**
  - **orchestrator/docker-compose.yml:** `PMA_ENABLED: "${PMA_ENABLED:-false}"`, `PMA_IMAGE: "${PMA_IMAGE:-...}"`, `NODE_PMA_OLLAMA_BASE_URL: "${NODE_PMA_OLLAMA_BASE_URL:-}"`.
  - **orchestrator/internal/handlers/nodes.go:** Use `NODE_PMA_OLLAMA_BASE_URL` (e.g. `http://host.containers.internal:11434`) for PMA container inference URL so node's PMA container can reach Ollama.
  - **scripts/setup_dev_impl.py:** When `ollama_in_stack`: build control-plane + cynode-pma images; set `PMA_ENABLED=true`, `PMA_IMAGE=cynodeai-cynode-pma:dev`, `NODE_PMA_OLLAMA_BASE_URL=http://host.containers.internal:11434`.
  - **scripts/test_scripts/run_e2e.py:** When `pma_inference` and not `--skip-ollama`: require Ollama container running (exit with instructions if not); after prereq run `wait_for_pma_chat_ready(180s)` and exit 1 if PMA chat not ready.
  - **scripts/test_scripts/helpers.py:** `ollama_container_running()`, `wait_for_pma_chat_ready()` (login + poll POST /v1/chat/completions until 2xx).

### 3. e2e_124 Worker-Api / PMA Healthz Timeouts

- **Symptom:** TestProxyPmaWithInference and TestProxyPmaWithRealOllama skipped with "worker-api did not become ready (healthz)" or "PMA did not become ready".
- **Fix:** In `e2e_124_worker_pma_proxy.py`, PMA healthz timeout increased to 25s and worker-api healthz timeout to 45s for the inference and real-Ollama classes.

### 4. How to Run Full `pma_inference` (No Skips)

1. Stop any existing stack: `just setup-dev stop`
2. Start stack **with Ollama** so the node gets PMA config:  
   `SETUP_DEV_OLLAMA_IN_STACK=1 just setup-dev start`  
   (First run builds control-plane and cynode-pma images; ensure port 5432 is free.)
3. Run E2E: `just e2e --tags pma_inference -v`

If the Ollama container is not running when you run step 3, the runner exits with instructions to use step 2. `wait_for_pma_chat_ready` waits up to 180s for the node to report PMA and for chat to return 2xx before running tests.

## Artifacts

- E2E output: `tmp/e2e_suite_proxy_pma_run2.txt`, `tmp/e2e_pma_inference_skip_ollama.txt`
- Report: `docs/dev_docs/2026-03-07_pma_inference_e2e_review.md`
