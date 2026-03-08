# Full-Demo E2E: Failures and Skips Investigation

## Summary

- **Date:** 2026-03-07
- **Command:** `just setup-dev full-demo`
- **Stack:** Prescribed startup (no OLLAMA in compose; node starts inference when instructed).
- **Change applied:** Ollama model-pull timeout increased to 600s in `helpers.run_ollama_inference_smoke()` and in `e2e_124_worker_pma_proxy._ollama_ensure_model()` so prereq smoke and proxy+real-Ollama tests can complete.

- **Passed:** Majority of tests (status, auth, task create/list/get/result, prompt task, skills, workflow API, API egress, worker telemetry, worker health/readyz, SBA task, proxy payload encoding, SBA result contract).
- **Failed (3):** Inference task (e2e_090), models and chat (e2e_110), PMA chat with project context (e2e_115).
- **Skipped (4):** Secure store envelope (e2e_122), and e2e_124 proxy PMA functional/with-inference/with-real-Ollama (setUpClass).

---

## Failures

Three tests failed; all depend on the prescribed inference + PMA path.

### Failure 1: `e2e_090_task_inference.TestInferenceTask.test_inference_task`

- **Assertion:** Create inference task (`--use-inference`), poll until completed; job stdout must contain `http://localhost:11434` (OLLAMA_BASE_URL in sandbox).
- **Likely cause:** Inference task either did not reach `completed`, or the job ran in a context where OLLAMA_BASE_URL is not that value (e.g. different URL in sandbox, or task failed).
  Prescribed path: node starts inference when instructed; timing or routing may mean the inference container/sandbox is not ready or not used for this task.

### Failure 2: `e2e_110_task_models_and_chat.TestModelsAndChat.test_models_and_chat`

- **Assertion:** `cynork models list -o json` returns `object: list` with at least one model; then one-shot `cynork chat --message "Reply with exactly: OK"` succeeds (unless E2E_SKIP_INFERENCE_SMOKE).
- **Likely cause:** Either models list returned empty/failed, or the chat call failed (error/eof/502 in output).
  Depends on gateway + PMA + inference path being available.

### Failure 3: `e2e_115_pma_chat_context.TestPmaChatContext.test_chat_with_project_context`

- **Assertion:** `cynork chat --message "Reply with OK" --project-id default --plain` succeeds when inference is available (or skips with "project chat unavailable").
- **Likely cause:** Chat with project-id failed (PMA handoff or completion not returned).
  Same dependency on PMA and inference being ready and routed correctly.

**Common thread:** All three failures depend on the **prescribed** inference + PMA path: node-manager starts inference (and possibly PMA) when the orchestrator directs; gateway models/chat go through PMA.
If PMA is not yet registered or inference not ready when E2E runs, or if routing is incomplete, these tests will fail.
Options: run with `--ollama-in-stack` when OLLAMA in compose is needed, or fix timing/routing so prescribed path is ready before E2E.

---

## Skips

Four tests skipped; two categories below.

### Skip 1: `e2e_122_secure_store_envelope_structure.test_agent_token_files_are_envelopes_and_not_plaintext`

- **Reason:** `secrets/agent_tokens dir not present (no managed services)`.
- **Explanation:** Test asserts that under `NODE_STATE_DIR`, `secrets/agent_tokens` files are encrypted envelopes.
  With prescribed startup, managed services (and thus agent tokens) may not be created yet, so the test correctly skips.

### Skip 2: `e2e_124_worker_pma_proxy` (TestProxyPmaFunctional, TestProxyPmaWithInference, TestProxyPmaWithRealOllama)

- **Reason:** `worker-api did not become ready (healthz)` in setUpClass.
- **Explanation:** These tests start a **second** worker-api (and PMA/mock or real Ollama) on isolated ports (e.g. 18091, 18094).
  The subprocess worker-api did not respond with 200 on `/healthz` within 15s.
  Possible causes: resource contention when the main stack is already running (main worker-api on 12090), slow startup, or init failure (e.g. telemetry/secure store under `tmp/proxy-pma-test-state`).
  Stderr from the subprocess is not printed, so failures are silent.
- **Recommendation:** Run the proxy PMA suite in isolation (`just e2e --tags suite_proxy_pma`) when the main stack is **stopped**, so the only worker-api is the one started by the test.
  That avoids port/state contention and matches the environment where these tests were designed to pass.

---

## Recommendations

1. **Inference/PMA failures (e2e_090, e2e_110, e2e_115):** Run full-demo with bypasses to confirm E2E pass when OLLAMA and PMA are in compose:
   `just setup-dev full-demo --ollama-in-stack`
   If that passes, the gap is in the prescribed path (timing, registration, or routing).

2. **e2e_124 proxy skips:** Run proxy PMA tests alone with stack down:
   `just setup-dev stop` then `just e2e --tags suite_proxy_pma`
   Optionally increase the worker-api healthz wait in e2e_124 setUpClass if under load 15s is too short.

3. **e2e_122:** Skip is expected when there are no managed services; no change unless the prescribed flow is extended to create agent tokens before E2E.

4. **Debugging:** To see why the second worker-api fails in e2e_124 during full-demo, temporarily set `stdout=subprocess.PIPE` and `stderr=subprocess.STDOUT` (or log stderr) in the worker-api Popen so init errors are visible.
