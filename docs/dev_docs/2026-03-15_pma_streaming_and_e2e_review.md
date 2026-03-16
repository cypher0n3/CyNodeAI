# PMA Streaming and E2E Review

- [Purpose](#purpose)
- [Summary](#summary)
- [E2E Report Corrections and Discrepancies](#e2e-report-corrections-and-discrepancies)
  - [Justfile Readiness Gate (Report Lines 16-19)](#justfile-readiness-gate-report-lines-16-19)
  - [`--skip-ollama` Usage (Report Lines 23-24)](#--skip-ollama-usage-report-lines-23-24)
  - [Task Create With Attachments (Report Lines 25-27)](#task-create-with-attachments-report-lines-25-27)
  - [Task Get by Name (Report Lines 29-32)](#task-get-by-name-report-lines-29-32)
  - [Task Result by Name (Report Lines 34-36)](#task-result-by-name-report-lines-34-36)
- [Specification Compliance](#specification-compliance)
  - [Assessment vs. Implementation Plan](#assessment-vs-implementation-plan)
  - [Requirement Traceability](#requirement-traceability)
- [Architectural and Test Design Issues](#architectural-and-test-design-issues)
  - [E2E Test Dependencies](#e2e-test-dependencies)
  - [Streaming Code Paths](#streaming-code-paths)
  - [Concurrency and Safety](#concurrency-and-safety)
- [Maintainability and Documentation](#maintainability-and-documentation)
- [Recommended Refactor Strategy](#recommended-refactor-strategy)

## Purpose

This document reviews the work reported against [2026-03-14_pma_streaming_to_tui_assessment.md](2026-03-14_pma_streaming_to_tui_assessment.md) and the [e2e_run_report_2026-03-15.md](e2e_run_report_2026-03-15.md).
It applies a senior Go developer reviewer lens and captures all issues and discrepancies for follow-up.
No code changes are prescribed here unless explicitly directed.

## Summary

The assessment correctly identifies that PMA's primary path (capable model + MCP) is fully blocking and that `emitContentAsSSE` must be removed in favor of heartbeat fallback or real streaming.
The streaming implementation plan and closeouts show Tasks 1-4 and 6-7 done, with Task 5 (TUI structured streaming UX) deferred.
The E2E report contains several mischaracterizations and the task E2E tests have design gaps: readiness gate attribution, `--skip-ollama` usage, attachment test tmpdir, and get/result-by-name test atomicity.

## E2E Report Corrections and Discrepancies

Subsections below correct or clarify the E2E run report and list discrepancies.

### Justfile Readiness Gate (Report Lines 16-19)

**Report states:** Wait for user-gateway readyz (port 12080) instead of control-plane (12082); timeout increased to 90s.

**Discrepancy:** Control-plane must be up for the whole system to function (node-manager polls control-plane before registering; migrations run on control-plane startup).
The fix that made `just setup-dev restart --force` succeed was likely the **90s timeout increase**, not switching the gate from control-plane to user-gateway.
If the gate was changed to user-gateway only, the system may report "ready" before control-plane is ready, leading to flaky E2E or node registration failures.

**Recommendation:** Correct the E2E report to state that the timeout increase to 90s is the likely fix.
Document that both control-plane and user-gateway must be ready for full E2E.
Consider either: (a) waiting for control-plane readyz (12082) first, then user-gateway readyz (12080), or (b) waiting for both with a single 90s loop that checks both ports, so that "ready" means the full stack is ready.

### `--skip-ollama` Usage (Report Lines 23-24)

**Report states:** When `--skip-ollama` is set, Ollama inference smoke is skipped so the suite can proceed.

**Discrepancy:** `--skip-ollama` should only be set when the Ollama container is **not** running.
In all current scenarios (full-demo, standard dev stack with inference), Ollama is expected to be in the stack or started by the node; the suite should not default to skipping Ollama when the stack is intended to have inference.

**Recommendation:** Update the E2E report and any run instructions to state that `--skip-ollama` is for environments where the Ollama container is intentionally not running (e.g. CI without GPU or a minimal stack).
Document that for normal dev and full-demo runs, Ollama should be running and `--skip-ollama` should not be used.

### Task Create With Attachments (Report Lines 25-27)

**Report states:** `test_task_create_with_attachments` skips with message when `state.CONFIG_PATH` is None (e.g. when run in isolation).

**Discrepancy:** The test should not depend on global state for its working directory.
It currently uses `config.PROJECT_ROOT + "/tmp"` for attachment paths and skips when `CONFIG_PATH` is None, which couples "has auth config" with "has a place to put attachments."

**Recommendation:** Make the test self-contained by populating a **test-local tmpdir** (e.g. `tempfile.mkdtemp()`) for attachment files during the test.
Use that tmpdir for `--attach` paths so the test can run in isolation without relying on repo `tmp/` or on `state.CONFIG_PATH` for anything other than cynork auth.
If the test requires auth, it should call `state.init_config()` and perform login (or depend on a shared auth prereq) explicitly; the skip should be limited to "auth not available" not "CONFIG_PATH is None" as a proxy for "no tmpdir."

### Task Get by Name (Report Lines 29-32)

**Report states:** `test_task_get_by_name` skips when `state.TASK_NAME` is None (run full suite or `test_task_create_named` first); when get-by-name returns non-ok, call `_assert_clear_name_resolution_error` before failing.

**Discrepancy:** The test is not atomic.
It depends on another test (or full suite order) to set `state.TASK_NAME`, which makes it fragile when run in isolation or when test order changes.

**Recommendation:** Make the test atomic: either (a) create a named task inside the same test (or in a setUp that runs only for this test class), then call `task get <name>` and assert, or (b) use a fixed unique name (e.g. prefixed with test name and timestamp) and create it in-test.
Same pattern applies to `test_task_result_by_name` below.

### Task Result by Name (Report Lines 34-36)

**Report states:** `test_task_result_by_name` same as above (skip when `TASK_NAME` is None; assert clear error when not ok).

**Discrepancy:** Same atomicity issue as get-by-name: reliance on `state.TASK_NAME` set by a different test.

**Recommendation:** Make the test atomic in the same way as get-by-name: create a named task within the test (or in a dedicated setUp), then call `task result <name>` and assert.
Do not rely on `test_task_create_named` or full suite execution order.

## Specification Compliance

This section checks alignment between the assessment, implementation plan, and requirements.

### Assessment vs. Implementation Plan

The assessment (2026-03-14) and the streaming implementation plan (2026-03-15) are aligned on: removal of `emitContentAsSSE`, heartbeat fallback when PMA cannot stream, streaming LLM wrapper around langchaingo, and per-endpoint SSE formats.
The plan defers: token state machine (think/tool-call tagging), overwrite events, tool_progress/tool_result injection, and `runtime/secret` wrapping for stream buffers.
Closeouts (Task 2, Task 3, Task 4) confirm minimal green: iteration_start, delta, done from PMA; gateway relays; cynork transport returns streamed response_id.

#### Gaps to Track

- `emitContentAsSSE` is still present; removal and replacement with heartbeat fallback (DP-5) is not yet done.
- Gateway relay currently forwards only `delta` from PMA NDJSON; named events (`cynodeai.iteration_start`, etc.) and overwrite/amendment handling per spec are partial or pending.
- Orchestrator accumulator buffers (visible, thinking, tool-call) and post-stream secret scan (DP-6, DP-7) are not fully implemented.
- TUI thinking/tool-call storage and overwrite/heartbeat UX (Task 5) are deferred.

### Requirement Traceability

Requirements REQ-PMAGNT-0118, REQ-USRGWY-0149, REQ-CLIENT-0209 and the CYNAI.PMAGNT.StreamingAssistantOutput / CYNAI.USRGWY / CYNAI.CLIENT specs are not yet fully satisfied by the current code: real token-by-token streaming on the capable-model + MCP path is in progress (streaming LLM wrapper emits deltas), but fake chunking removal and heartbeat fallback are outstanding.

## Architectural and Test Design Issues

This section calls out test design and streaming-path issues.

### E2E Test Dependencies

- **Shared state (`e2e_state`):** `TASK_ID`, `TASK_NAME`, `CONFIG_PATH` are set by earlier tests and consumed by later ones.
  This creates ordering constraints and makes isolated test runs or parallelization hard.
  Prefer tests that create their own data (e.g. create task in-test) or use a documented prerequisite step (e.g. "run auth and task-create fixtures first") instead of implicit order dependency.

### Streaming Code Paths

- **PMA:** The streaming wrapper (`streaming.go`) and `runCompletionWithLangchainStreaming` add a new path; the blocking path remains.
  No regression in the blocking path is expected, but coverage for the streaming path (e.g. `streaming.Call`, branch coverage) is below the 90% target noted in the closeout.
- **Orchestrator:** Handler branch for stream=true with capable model now calls PMA streaming; gateway relay of PMA NDJSON beyond `delta` (iteration_start, overwrite, etc.) is still incomplete per Task 3 closeout.

### Concurrency and Safety

- The assessment and plan require `runtime/secret` (or equivalent) for all stream buffers (PMA, orchestrator, TUI).
  This is deferred; no stream buffer code uses it yet.
  When implementing, scope `secret.Do` to buffer-touching code only (no goroutine creation inside the block).

## Maintainability and Documentation

- **scripts/README.md** still says "Script waits for control-plane readyz (up to 60s)."
  The justfile now waits for user-gateway readyz for 90s.
  Align the README with actual behavior and with the recommendation above (control-plane + user-gateway readiness).
- **E2E report:** Update as above so that future readers do not assume "wait for user-gateway instead of control-plane" is the correct long-term design; document timeout and optional dual-port readiness.
- **Streaming closeouts:** Task 5 deferral is documented; follow-up items (emitContentAsSSE removal, coverage to >=90%, e2e_202-e2e_204 harness completion) should remain in a single follow-up or backlog list.

## Recommended Refactor Strategy

1. **E2E report:** Revise the Justfile and Run E2E sections to correct the readiness-gate attribution, document `--skip-ollama` usage, and note the intended fixes for attachments and get/result-by-name tests (tmpdir, atomicity).
2. **Justfile/scripts:** If the gate is to remain user-gateway-only, add a comment that control-plane is a dependency and that the 90s timeout allows it to become ready before user-gateway in typical startup order; or add an explicit wait for control-plane readyz (12082) before or in parallel with user-gateway.
3. **E2E tests:** Implement tmpdir for `test_task_create_with_attachments` and make `test_task_get_by_name` and `test_task_result_by_name` atomic (create named task in-test or in setUp).
4. **Streaming:** Proceed with removal of `emitContentAsSSE` and heartbeat fallback (DP-5); complete gateway relay of named events and overwrites; raise agents/internal/pma coverage to >=90%; complete e2e_202-e2e_204 as planned.
5. **Docs:** Sync scripts/README.md with justfile readiness behavior and keep a single follow-up list for streaming and E2E test improvements.
