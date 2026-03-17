# E2E: Just e2e --Single `e2e_0500_workflow_api` --No-Build / Ollama Inference Smoke Failed

## 1 Summary

Running `just e2e --single e2e_0500_workflow_api --no-build` can fail with:

- `Error: Ollama inference smoke failed`
- `Recipe e2e failed with exit code 1` (just)

## 2 Root Cause

- The **Ollama inference smoke** runs as part of the **ollama** prereq in `_run_single_prereq()` (scripts/test_scripts/run_e2e.py).
  Prereqs are run **per test**; the ollama step runs only when a test declares the `ollama` prereq and it has not already succeeded (or when `--skip-ollama` is set, the step skips the smoke).
  The smoke waits for the Ollama container, pulls the E2E model if needed, and sends one chat request to `OLLAMA_BASE_URL` (default `http://localhost:11434`).
  If the container is present but not ready, the model pull fails, or the chat request fails (e.g. no Ollama on host, or port not reachable), the smoke returns False and that prereq is recorded as failed; tests that need it are skipped.
- **When the smoke runs:** Only when a test in the suite has prereq `ollama` and the ollama prereq has not yet succeeded this run.
  With `--single e2e_0500_workflow_api`, the workflow API module does not declare `ollama`, so the ollama prereq is never run and the smoke is not executed.
- **Why it could run for e2e_0500 in older code:** Previously the runner ran all suite prereqs once at startup; if the suite was discovered as the full suite (e.g. no `--single`), the ollama prereq ran.
  With per-test prereqs, only tests that declare `ollama` trigger it.

## 3 Code Paths

- `run_e2e.py` uses `_PrereqFilterSuite` to run each test; before a test, it runs only that test's prereqs (in `e2e_tags.PREREQ_ORDER`) that have not yet succeeded, or that are in `PREREQ_ALWAYS_RERUN`.
  The **ollama** prereq is run via `_run_single_prereq("ollama", opts)`; when `opts.skip_ollama` is True, the step sets `E2E_SKIP_INFERENCE_SMOKE` and does not call the smoke.
- `_run_single_prereq("ollama", opts)` when not skipping: calls `helpers.run_ollama_inference_smoke()`; on False the prereq is recorded as failed and tests requiring it are skipped.
- `helpers.run_ollama_inference_smoke()`: skips if `E2E_SKIP_INFERENCE_SMOKE` is set or no container runtime; if container name is not in `docker/podman ps`, returns True (skip); otherwise waits for container, ensures model, and runs one chat request to `OLLAMA_BASE_URL` (default localhost:11434); returns False after 3 failed attempts.

## 4 Workaround

Run with `--skip-ollama` so the smoke is not run:

```bash
just e2e --single e2e_0500_workflow_api --no-build --skip-ollama
```

e2e_0500_workflow_api is a control-plane workflow test and does not require Ollama.

## 5 Why the Smoke Can Fail When "Ollama is Running"

The smoke only runs when a **container** named `cynodeai-ollama` (or `OLLAMA_CONTAINER_NAME`) appears in `docker/podman ps -a`.
If Ollama is running only on the **host** (e.g. `ollama serve` or systemd) and that container is not present, the smoke is skipped and does not fail.

When the smoke does run, it can still fail in three places.
First, it uses `ps` (running containers only); if the container exists but is **exited**, it never appears as running and the smoke fails after 30s.
Second, it runs `docker/podman exec cynodeai-ollama ollama pull qwen3.5:0.8b` (or `OLLAMA_E2E_MODEL`); failure can be exec fails, pull timeout (600s), network error, or OOM in the container.
Third, the script sends a POST from the **host** to `OLLAMA_BASE_URL` (default `http://localhost:11434`).
The process listening on the host's port 11434 must be the one the smoke expects.
If the **container** is running and publishes `11434:11434`, localhost:11434 hits the container; then failure is usually timeout (120s), connection refused, or response without `message.content`.
If you run Ollama on the host and **also** have a container named `cynodeai-ollama`, port 11434 can only be used by one of them; the smoke might be talking to the wrong one or to nothing.
The smoke expects JSON with `message.content`; a different API shape or an error body can make it return False.

Quick checks:

- `docker ps` or `podman ps`: is `cynodeai-ollama` **running** (not just listed with `ps -a`)?
- From the host: `curl -s http://localhost:11434/api/chat -d '{"model":"qwen3.5:0.8b","messages":[{"role":"user","content":"hi"}],"stream":false}'` - does it return JSON with `message.content`?
- In the container: `docker exec cynodeai-ollama ollama list` - is `qwen3.5:0.8b` (or your `OLLAMA_E2E_MODEL`) listed?

## 6 Fix / Recommendations

- With per-test prereqs, `--single e2e_0500_workflow_api` does not run the ollama prereq because that module does not declare it; no change needed for that case.
- For a full run without Ollama (e.g. CI), use `E2E_SKIP_INFERENCE_SMOKE=1` or `--skip-ollama`.
