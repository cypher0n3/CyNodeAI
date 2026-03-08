# Python Functional Test Execution Plan

- [Scope](#scope)
- [Working Rules](#working-rules)
- [Phase 0: Restore Inference-Path Operability](#phase-0-restore-inference-path-operability)
- [Phase 1: Stop Locking in Known Drift](#phase-1-stop-locking-in-known-drift)
- [Phase 2: Tighten the Highest-Value Assertions](#phase-2-tighten-the-highest-value-assertions)
- [Phase 3: Fix Go Drifts Exposed by Hardened Tests](#phase-3-fix-go-drifts-exposed-by-hardened-tests)
- [Phase 4: Reduce Flakiness and Suite Coupling](#phase-4-reduce-flakiness-and-suite-coupling)
- [Completion Checklist](#completion-checklist)

## Scope

Date: 2026-03-08.

This document turns the ordered remediation plan from `docs/dev_docs/2026-03-08_python_functional_test_spec_impl_review.md` into a concrete execution checklist for an engineer who is not already familiar with the codebase.

The goal is to improve the Python functional suite in the required order while keeping each step small, testable, and reviewable.

## Working Rules

- [ ] Use repo recipes from the `justfile` rather than calling scripts directly.

- [ ] Keep all notes, temporary reports, and captured outputs in `docs/dev_docs` or `tmp`.

- [ ] Do not change requirements, tech specs, makefiles, or the `justfile` unless a separate task explicitly asks for that.

- [ ] After each meaningful change, run the smallest relevant test subset first, then rerun the broader suite for that area.

- [ ] When a change touches docs only, prefer `just docs-check <path>`.
  If that fails because of unrelated repo issues, record the unrelated failure and still verify the changed file with `just lint-md <path>`.

- [ ] When a change touches Python E2E files, run the narrowest relevant `just e2e` subset first.

- [ ] When a change touches Go code, run the narrowest relevant Go test target or package tests before broader validation.

## Phase 0: Restore Inference-Path Operability

This phase comes first because the observed E2E failures are concentrated in inference-dependent flows.

Do not harden chat or SBA assertions until the baseline runtime path is working.

### Step 0.1: Capture a Clean Baseline

- [ ] Start the stack with the same recipe used by the failing run.

- [ ] Capture a fresh inference-focused baseline with:
  `just e2e --no-build -v --tags inference`

- [ ] Save the full output to a timestamped file in `docs/dev_docs` or `tmp`.

- [ ] Confirm the current failing set still includes:
  `e2e_090`, `e2e_110`, `e2e_115`, `e2e_140`, `e2e_145`, `e2e_192`, `e2e_193`, and `e2e_194`.

### Step 0.2: Fix Orchestrator Chat Inference Failures

Target symptoms:

- `502 Bad Gateway` from `cynork chat`
- structured `orchestrator_inference_failed` errors

Primary files to inspect:

- `orchestrator/internal/handlers/openai_chat.go`
- `orchestrator/internal/handlers/tasks.go`
- `orchestrator/internal/config/config.go`
- `orchestrator/cmd/user-gateway/main.go`
- `orchestrator/docker-compose.yml`

Checklist:

- [ ] Trace how chat requests flow from `cynork` to the gateway and then to orchestrator inference.

- [ ] Determine whether the failure is caused by upstream inference configuration, connection routing, timeout handling, or bad error translation.

- [ ] Fix the root cause without weakening error handling.

- [ ] Re-run:
  `just e2e --no-build -v --tags chat`

- [ ] Verify that `e2e_110`, `e2e_115`, `e2e_192`, `e2e_193`, and `e2e_194` now reach the chat path reliably enough to exercise behavior rather than fail immediately at transport.

### Step 0.3: Fix SBA Inference Dispatch Failures

Target symptom:

- timeout on `POST /v1/worker/jobs:run`

Primary files to inspect:

- `orchestrator/internal/handlers/tasks.go`
- `worker_node/cmd/worker-api/main.go`
- `worker_node/cmd/node-manager/main.go`
- `orchestrator/docker-compose.yml`
- `worker_node/docker-compose.yml`

Checklist:

- [ ] Trace how SBA tasks are turned into worker job requests.

- [ ] Confirm the expected worker API base URL and port are reachable from the caller making the request.

- [ ] Check whether the worker API is started, healthy, and actually ready when the SBA request is sent.

- [ ] Fix the root cause of the timeout.

- [ ] Re-run:
  `just e2e --no-build -v --tags sba_inference`

- [ ] Verify that `e2e_140` and `e2e_145` move past transport timeout failures.

### Step 0.4: Make Worker Proxy Functional Tests Actually Run

Target symptom:

- `e2e_124_worker_pma_proxy.py` skips because the worker API fixture never becomes healthy

Primary files to inspect:

- `scripts/test_scripts/e2e_124_worker_pma_proxy.py`
- `worker_node/cmd/worker-api/main.go`
- `worker_node/cmd/worker-api/main_test.go`

Checklist:

- [ ] Reproduce the proxy test setup failure in isolation.

- [ ] Identify whether the failure is due to missing env vars, binary startup assumptions, fixed port collisions, or health/readiness mismatch.

- [ ] Fix the setup path so the test class runs its assertions instead of skipping.

- [ ] Re-run:
  `just e2e --no-build -v --tags suite_proxy_pma`

- [ ] Verify that the proxy test classes execute real assertions.

### Phase 0 Exit Criteria

- [ ] The inference-tagged subset no longer fails immediately on transport or startup issues.

- [ ] Proxy-functional E2E setup is reliable enough to execute its assertions.

- [ ] A new baseline run is saved for comparison before moving to Phase 1.

## Phase 1: Stop Locking in Known Drift

This phase prevents the suite from continuing to bless behavior that is out of spec.

### Step 1.1: Separate Smoke Tests From Acceptance Tests

Primary files to inspect:

- `scripts/test_scripts/README.md`
- `scripts/test_scripts/e2e_tags.py`
- each affected `scripts/test_scripts/e2e_*.py`

Checklist:

- [ ] Decide which tests are allowed to remain smoke tests for current MVP behavior.

- [ ] Mark or document which tests are intended to be acceptance-contract tests.

- [ ] Update test docstrings and comments so reviewers can tell whether a test is smoke-only or contract-enforcing.

### Step 1.2: Stop Enforcing Non-Canonical CLI Flags

Primary tests:

- `scripts/test_scripts/e2e_020_auth_login.py`
- `scripts/test_scripts/e2e_050_task_create.py`

Spec references:

- `docs/tech_specs/cli_management_app_commands_core.md`
- `docs/tech_specs/cli_management_app_commands_tasks.md`

Checklist:

- [ ] Replace login coverage that depends on `-u` and `-p` with coverage that matches the canonical auth command surface.

- [ ] Replace task-create coverage that depends on `--task-name` and `--attachment` with the canonical names from the task spec.

- [ ] Keep any backward-compatibility checks separate from the acceptance-contract tests.

- [ ] Re-run the affected narrow subsets after each update.

### Step 1.3: Remove Ambiguous Prompt-Mode Inputs

Primary tests:

- `scripts/test_scripts/e2e_050_task_create.py`
- `scripts/test_scripts/e2e_100_task_prompt.py`
- `scripts/test_scripts/e2e_160_task_cancel.py`

Checklist:

- [ ] Replace shell-like prompt examples with clearly natural-language prompt examples when testing prompt mode.

- [ ] Add or retain separate tests for literal command or script modes where needed.

- [ ] Ensure every task-mode test makes it obvious which input mode is being exercised.

### Phase 1 Exit Criteria

- [ ] No acceptance test still depends on clearly non-canonical CLI flags.

- [ ] Prompt-mode tests no longer use shell-looking payloads unless the test is explicitly covering a command or script mode.

- [ ] Smoke versus contract intent is documented for the touched tests.

## Phase 2: Tighten the Highest-Value Assertions

This phase hardens the tests that are currently green but too permissive.

### Step 2.1: Tighten Task Result and Logs Coverage

Primary tests:

- `scripts/test_scripts/e2e_080_task_result.py`
- `scripts/test_scripts/e2e_150_task_logs.py`
- `scripts/test_scripts/e2e_196_task_list_status_filter.py`

Primary Go files to keep in mind:

- `cynork/cmd/task.go`
- `orchestrator/internal/handlers/tasks.go`

Checklist:

- [ ] Make `task result` assertions check exact required fields, not just `status`.

- [ ] Make `task logs` assertions check the expected contract shape, not just that JSON was returned.

- [ ] Tighten list filtering assertions to the canonical response shape rather than accepting multiple shapes.

- [ ] Re-run only the task subset first, then rerun the full task tag group.

### Step 2.2: Add Name-Based Task Identifier Coverage

Primary tests to add or expand:

- `scripts/test_scripts/e2e_070_task_get.py`
- `scripts/test_scripts/e2e_080_task_result.py`
- `scripts/test_scripts/e2e_150_task_logs.py`
- `scripts/test_scripts/e2e_160_task_cancel.py`

Checklist:

- [ ] Create or reuse a named task in the test setup.

- [ ] Add contract checks using the human-readable task name instead of the UUID.

- [ ] Confirm failure behavior is clear if name resolution is not implemented.

### Step 2.3: Harden Auth Refresh and Logout Assertions

Primary tests:

- `scripts/test_scripts/e2e_190_auth_refresh.py`
- `scripts/test_scripts/e2e_200_auth_logout.py`

Checklist:

- [ ] Assert the refresh flow updates stored credentials.

- [ ] Assert the old refresh token cannot still be treated as the current session token.

- [ ] Assert logout clears local state and leaves the config in the expected post-logout shape.

- [ ] If the product contract requires server-side invalidation, add an assertion that proves the old access path is rejected.

### Step 2.4: Harden Chat Assertions

Primary tests:

- `scripts/test_scripts/e2e_110_task_models_and_chat.py`
- `scripts/test_scripts/e2e_115_pma_chat_context.py`
- `scripts/test_scripts/e2e_192_chat_reliability.py`
- `scripts/test_scripts/e2e_193_chat_sequential_messages.py`
- `scripts/test_scripts/e2e_194_chat_simultaneous_messages.py`

Checklist:

- [ ] Replace the `"default"` project placeholder with a real project identifier when testing project context.

- [ ] For deterministic prompts, assert actual content rather than any non-empty output.

- [ ] For reliability tests, assert a meaningful success threshold rather than minimal survivability.

- [ ] For error-path tests, assert the canonical error shape.

### Step 2.5: Tighten Worker Telemetry Assertions

Primary tests:

- `scripts/test_scripts/e2e_119_worker_telemetry.py`
- `scripts/test_scripts/e2e_122_node_manager_telemetry.py`

Checklist:

- [ ] Assert non-placeholder values for node info and node stats where the spec requires real values.

- [ ] Fix the node-manager source naming to match the canonical telemetry source name.

- [ ] Require lifecycle-event presence where the spec mandates those records.

- [ ] Verify truncated metadata fields exactly where required by the telemetry spec.

### Step 2.6: Add Internal Managed-Agent Proxy Acceptance Coverage

Primary test area:

- `scripts/test_scripts/e2e_124_worker_pma_proxy.py`

Checklist:

- [ ] Add a dedicated scenario that exercises the internal managed-agent-to-orchestrator proxy path.

- [ ] Assert the expected status-code behavior for missing identity binding, missing token material, and successful proxying.

- [ ] Keep this distinct from the external bearer-token managed-service proxy coverage.

### Phase 2 Exit Criteria

- [ ] The previously weak green tests now enforce exact contract details.

- [ ] At least one task reference test uses task name rather than UUID for each supported surface.

- [ ] Chat, auth, telemetry, and proxy assertions are strict enough to catch contract drift rather than just endpoint responsiveness.

## Phase 3: Fix Go Drifts Exposed by Hardened Tests

This phase happens after the tests have been improved enough to fail for the right reasons.

### Step 3.1: Align CLI Auth Behavior

Primary file:

- `cynork/cmd/auth.go`

Checklist:

- [ ] Update the CLI auth command surface to match the published spec.

- [ ] Update prompts, flags, and persisted-session behavior as required by the spec.

- [ ] Run the auth-focused E2E subset after the change.

### Step 3.2: Align CLI Task Output and Flags

Primary file:

- `cynork/cmd/task.go`

Checklist:

- [ ] Update task command flags to match the canonical task command spec.

- [ ] Update `task result`, `task logs`, and related output so it matches the expected contract.

- [ ] Re-run the task-focused E2E subset after the change.

### Step 3.3: Implement Task-Name Resolution

Primary areas:

- `orchestrator/internal/handlers/tasks.go`
- underlying data access used by the task handlers

Checklist:

- [ ] Add lookup support for human-readable task names where the spec requires it.

- [ ] Apply the same resolution behavior consistently across get, result, cancel, and logs.

- [ ] Re-run the task-name test cases after the implementation.

### Step 3.4: Revisit Readiness Semantics

Primary areas:

- `orchestrator/cmd/user-gateway/main.go`
- `worker_node/cmd/worker-api/main.go`

Checklist:

- [ ] Decide whether the current tested surface matches the intended readiness contract.

- [ ] If not, update code or tests so the exercised endpoint actually represents readiness rather than just liveness.

- [ ] Re-run the readiness-related E2E tests after the change.

### Step 3.5: Replace Placeholder Worker Telemetry

Primary file:

- `worker_node/cmd/worker-api/main.go`

Checklist:

- [ ] Replace placeholder node info values with real runtime values where required.

- [ ] Replace placeholder node stats values with real measurements where required by the spec.

- [ ] Re-run the telemetry E2E subset after the change.

### Step 3.6: Reconcile Internal Worker Proxy Status Codes

Primary file:

- `worker_node/cmd/worker-api/main.go`

Checklist:

- [ ] Align internal proxy error translation with the expected managed-agent proxy contract.

- [ ] Re-run the internal proxy acceptance tests after the change.

### Phase 3 Exit Criteria

- [ ] The Go implementation now passes the hardened tests for auth, task, readiness, telemetry, and proxy behavior.

- [ ] No temporary compatibility shortcuts were introduced without clear justification.

## Phase 4: Reduce Flakiness and Suite Coupling

This phase improves long-term suite reliability after the main behavior issues are addressed.

### Step 4.1: Reduce Shared Mutable Test State

Primary files:

- `scripts/test_scripts/e2e_state.py`
- `scripts/test_scripts/e2e_tags.py`
- affected `e2e_*.py` modules

Checklist:

- [ ] Identify which tests still depend on global cross-module state.

- [ ] Replace that state with per-test or per-class setup where practical.

- [ ] Keep only the minimum shared state required by the suite.

### Step 4.2: Reduce Setup-Ordering Surprises

Checklist:

- [ ] Make prerequisite setup explicit in test code or helper code rather than implicit in module ordering wherever practical.

- [ ] Verify that filtered test runs still work when only one tag group is executed.

### Step 4.3: Remove Fixed-Port Flake Where Practical

Primary test area:

- `scripts/test_scripts/e2e_124_worker_pma_proxy.py`

Checklist:

- [ ] Replace fixed ports with dynamically allocated ports where the test structure allows it.

- [ ] Verify that tests still remain debuggable after the port-allocation changes.

### Step 4.4: Add a Config-Resolution Test

Primary helper area:

- `scripts/test_scripts/helpers.py`
- relevant CLI config tests

Checklist:

- [ ] Add one focused test that does not inject `CYNORK_GATEWAY_URL`.

- [ ] Use that test to detect default-config drift between the CLI and the dev stack.

### Phase 4 Exit Criteria

- [ ] The suite is less order-dependent.

- [ ] Proxy-focused tests are less likely to fail due to local port collisions.

- [ ] At least one test protects against CLI default-config drift.

## Completion Checklist

Use this section as the final sign-off gate.

- [ ] Phase 0 complete.
- [ ] Phase 1 complete.
- [ ] Phase 2 complete.
- [ ] Phase 3 complete.
- [ ] Phase 4 complete.
- [ ] Updated test outputs captured and stored.
- [ ] New failures, if any, are documented.
- [ ] Relevant narrow test subsets pass.
- [ ] Final broad E2E run is captured.
- [ ] Follow-up gaps that need spec or product decisions are documented separately.
