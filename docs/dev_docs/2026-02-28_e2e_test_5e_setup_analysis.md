# E2E Test 5E Failure Analysis: Test Setup vs. Debug Script (2026-02-28)

## Summary

Test 5E (SBA job via cynork `task create --use-sba`) can fail due to **test setup assumptions** that differ from the working `tmp/sba-container-debug.sh` script.
The debug script only needs the SBA image and podman; the e2e path requires the full chain: orchestrator stack + **running registered node** + SBA image.
This doc identifies setup gaps and differences.

## Differences: Debug Script vs. E2E Test 5E

- **Invocation**
  - `tmp/sba-container-debug.sh`: Direct `podman run ... -v $JOB_DIR:/job:z $IMAGE`
  - E2E Test 5E: Via API: cynork task create -> orchestrator -> dispatcher -> node executor -> podman run
- **Job spec**
  - Both use the same spec as `buildSBAJobPayload`: protocol_version 1.0, one `run_command` step `echo sba-run`.
- **Image**
  - Both use `cynodeai-cynode-sba:dev` (same as `DefaultSBARunnerImage`).
- **Env**
  - Debug script: `SBA_DIRECT_STEPS=1` only.
  - E2E: Executor adds `SBA_DIRECT_STEPS=1` plus task/job env (buildTaskEnv).
- **Workspace**
  - Debug script: No `-w` / no workspace mount.
  - E2E: Executor mounts temp dir at `/workspace` and `-w /workspace`.
- **Prerequisites**
  - Debug script: Image built, podman, jq.
  - E2E: Stack up, **node running and registered**, image built and visible to node runtime.

The debug script succeeds because it runs the container directly on the host.
E2E Test 5E fails if the **node** is not running or not registered, or if the **SBA image is not built** when the node tries to run the job.

## Test Setup Issues Identified

The following setup issues explain why E2E Test 5E can fail even when the debug script works.

### 1. Node Not Started in Documented "Start" + "Test-E2e" Flow

- `./scripts/setup-dev.sh start` runs only `build_binaries` and `start_orchestrator_stack_compose`.
- It does **not** start the node.
- The script help says: "Use '$0 test-e2e' to run the E2E demo test" after `start`.
- So after `start` + `test-e2e`, **no worker is running**.
- SBA (and other) jobs stay queued; Test 5E polls for up to 90s and fails with "SBA task did not complete".

For Test 5E to pass, the **node must be running and registered** before `run_e2e_test`.
That is only guaranteed when using `full-demo` (which calls `start_node`).
Running `test-e2e` alone assumes something else has already started the node.

### 2. SBA Image Built Only in Full-Demo

- `ensure_sba_runner_build_if_delta()` is invoked only in the `full-demo` branch.
- Neither `start` nor `test-e2e` builds the SBA image.
- If the user runs `start` (or only the orchestrator stack) and then `test-e2e` without having run `full-demo` in that session, the image `cynodeai-cynode-sba:dev` may not exist.
- When the node runs the SBA job, the executor does `podman run ... cynodeai-cynode-sba:dev`; if the image is missing, the run fails and the job is marked failed.

Test 5E implicitly requires the SBA image to be built; that is only guaranteed by `full-demo`.

### 3. No Pre-Check for Node or SBA Readiness in Test 5E

- The test does not check that at least one node is registered or that the SBA image is available before creating the SBA task.
- Failures present as: timeout ("SBA task did not complete") or "SBA task job result missing sba_result" / "SBA task has no job result", without indicating that the cause may be "node not running" or "image not built".

Adding a short pre-check (e.g. node registered or image present) and a clear error message would make Test 5E failures easier to diagnose.

### 4. Result Parsing (Not a Bug)

- Task result API returns `jobs[0].result` as a JSON **string** (the stored RunJobResponse).
- The test does `jq -r '.jobs[0].result'` then `jq -e '.sba_result != null'` on that string, which is correct.
- No change needed here.

## Why the Debug Script Returns Immediately Successfully

The debug script only runs the container with a minimal job spec and `SBA_DIRECT_STEPS=1`.
The runner executes the single `run_command` step ("echo sba-run") and writes `result.json` with `sba_result`.
It does not go through the orchestrator or node, so it does not depend on control-plane, user-gateway, node process, registration, or dispatcher.
The same container and job spec work in the script but can fail in e2e purely due to **missing node or missing image** in the e2e environment.

## Recommendations

- **Document or enforce node for test-e2e.**
  Either document that `test-e2e` requires a running registered node (and optionally SBA image), or have `test-e2e` start the node and ensure SBA image when not already running, so that "start" + "test-e2e" is valid.
- **Ensure SBA image when running Test 5E.**
  In `run_e2e_test`, before Test 5E, call `ensure_sba_runner_build_if_delta` so that standalone `test-e2e` runs have the image; or skip Test 5E with a clear message if the image cannot be built.
- **Optional: pre-check before Test 5E.**
  Before creating the SBA task, optionally check (e.g. control-plane or DB) that at least one node is registered; if not, fail fast with a message like "Test 5E requires a running registered node; start the node or run full-demo."
- **Improve failure output.**
  On "SBA task did not complete" or "missing sba_result", log the full task result (or at least `jobs[0].result` / stderr) so that image-not-found or other worker errors are visible without extra debugging.

## Verification

- Compared `buildSBAJobPayload` (orchestrator) and debug script job spec: equivalent (protocol_version 1.0, one run_command step, same constraints/context).
- Compared executor `buildSBARunArgs` and debug script: same image, same `SBA_DIRECT_STEPS=1`, same job mount; executor adds workspace mount and task/job env.
- Traced stored result: dispatcher stores full `RunJobResponse` (including `sba_result`) via `applyJobResult` -> `CompleteJob`; API returns it as `jobs[0].result` string; test jq parsing is correct.
