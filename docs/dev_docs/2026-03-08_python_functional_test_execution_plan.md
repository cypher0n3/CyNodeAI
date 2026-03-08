# Python Functional Test Execution Plan

- [Scope](#scope)
- [Working Rules](#working-rules)
- [Phase 0: Restore Inference-Path Operability](#phase-0-restore-inference-path-operability)
  - [Step 0.1: Capture a Clean Baseline](#step-01-capture-a-clean-baseline)
  - [Step 0.2: Fix Orchestrator Chat Inference Failures](#step-02-fix-orchestrator-chat-inference-failures)
  - [Step 0.3: Fix SBA Inference Dispatch Failures](#step-03-fix-sba-inference-dispatch-failures)
  - [Step 0.4: Make Worker Proxy Functional Tests Actually Run](#step-04-make-worker-proxy-functional-tests-actually-run)
  - [Phase 0 Exit Criteria](#phase-0-exit-criteria)
- [Phase 1: Stop Locking in Known Drift](#phase-1-stop-locking-in-known-drift)
  - [Step 1.1: Separate Smoke Tests From Acceptance Tests](#step-11-separate-smoke-tests-from-acceptance-tests)
  - [Step 1.2: Stop Enforcing Non-Canonical CLI Flags](#step-12-stop-enforcing-non-canonical-cli-flags)
  - [Step 1.3: Remove Ambiguous Prompt-Mode Inputs](#step-13-remove-ambiguous-prompt-mode-inputs)
  - [Phase 1 Exit Criteria](#phase-1-exit-criteria)
- [Phase 2: Tighten the Highest-Value Assertions](#phase-2-tighten-the-highest-value-assertions)
  - [Step 2.1: Tighten Task Result Coverage and Reintroduce Proper Logs Coverage](#step-21-tighten-task-result-coverage-and-reintroduce-proper-logs-coverage)
  - [Step 2.2: Add Name-Based Task Identifier Coverage](#step-22-add-name-based-task-identifier-coverage)
  - [Step 2.3: Harden Auth Refresh Assertions and Reintroduce Proper Logout Coverage](#step-23-harden-auth-refresh-assertions-and-reintroduce-proper-logout-coverage)
  - [Step 2.4: Harden Chat Assertions](#step-24-harden-chat-assertions)
  - [Step 2.5: Tighten Worker Telemetry Assertions](#step-25-tighten-worker-telemetry-assertions)
  - [Step 2.6: Add Managed-Agent Proxy and Network-Restriction Acceptance Coverage](#step-26-add-managed-agent-proxy-and-network-restriction-acceptance-coverage)
  - [Phase 2 Exit Criteria](#phase-2-exit-criteria)
- [Phase 3: Fix Go Drifts Exposed by Hardened Tests](#phase-3-fix-go-drifts-exposed-by-hardened-tests)
  - [Step 3.1: Align CLI Auth Behavior](#step-31-align-cli-auth-behavior)
  - [Step 3.2: Align CLI Task Output and Flags](#step-32-align-cli-task-output-and-flags)
  - [Step 3.3: Implement Task-Name Resolution](#step-33-implement-task-name-resolution)
  - [Step 3.4: Revisit Readiness Semantics](#step-34-revisit-readiness-semantics)
  - [Step 3.5: Replace Placeholder Worker Telemetry](#step-35-replace-placeholder-worker-telemetry)
  - [Step 3.6: Reconcile Worker Proxy Behavior With Agent Network Restriction](#step-36-reconcile-worker-proxy-behavior-with-agent-network-restriction)
  - [Phase 3 Exit Criteria](#phase-3-exit-criteria)
- [Phase 4: Reduce Flakiness and Suite Coupling](#phase-4-reduce-flakiness-and-suite-coupling)
  - [Step 4.1: Reduce Shared Mutable Test State](#step-41-reduce-shared-mutable-test-state)
  - [Step 4.2: Reduce Setup-Ordering Surprises](#step-42-reduce-setup-ordering-surprises)
  - [Step 4.3: Remove Fixed-Port Flake Where Practical](#step-43-remove-fixed-port-flake-where-practical)
  - [Step 4.4: Add a Config-Resolution Test](#step-44-add-a-config-resolution-test)
  - [Phase 4 Exit Criteria](#phase-4-exit-criteria)
- [Completion Checklist](#completion-checklist)

## Scope

Date: 2026-03-08.

This document turns the ordered remediation plan from `docs/dev_docs/2026-03-08_python_functional_test_spec_impl_review.md` into a concrete execution checklist for an engineer who is not already familiar with the codebase.

The goal is to improve the Python functional suite in the required order while keeping each step small, testable, and reviewable.

This plan also incorporates the updated worker proxy and security-boundary contract in
`REQ-WORKER-0174`, `REQ-WORKER-0162`, `REQ-WORKER-0163`, `docs/tech_specs/worker_node.md`,
and `docs/tech_specs/worker_api.md`: all agent runtimes on a worker are network-restricted,
and all inbound and outbound traffic to or from those agents must route through worker proxies.

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

Progress:

- [x] Fixed worker API bearer-token mismatch on orchestrator side (`orchestrator/internal/config/config.go` default now matches worker default token).

- [x] Removed compose-level PMA inference workaround path (`NODE_PMA_OLLAMA_BASE_URL`) from the orchestrator stack.

- [x] Implemented dynamic PMA inference base-url derivation in orchestrator managed-service desired state (`orchestrator/internal/handlers/nodes.go`) using node worker API target host.

- [x] Added/updated tests for orchestrator managed-service inference URL derivation (`orchestrator/internal/handlers/nodes_test.go`).

- [x] Runtime check after restart confirms managed PMA container now receives `OLLAMA_BASE_URL` and `INFERENCE_MODEL` from managed-service config, not from compose env override.

- [ ] Chat E2E coverage is still partially blocked by auth-prereq/config gating in the inference-tag run (`CONFIG_PATH not set` skips); this is now a test harness/prereq issue, not the original PMA endpoint wiring gap.

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

Progress:

- [x] Increased dispatcher HTTP timeout to align with synchronous worker job execution windows (`orchestrator/cmd/control-plane/dispatcher.go` and compose env).

- [x] Verified control-plane now emits managed-service desired state and config-ack flow for PMA host node without inference URL fallback env.

- [ ] SBA inference-tag E2E is still blocked by test-side config/prereq issues (`CONFIG_PATH not set` skips and `e2e_123` helper `NoneType` path failure) and needs separate remediation in Python test setup/helpers.

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

- [ ] Keep the updated security-boundary contract in mind while fixing setup:
  getting the test class to execute is necessary, but it must not lock in direct-network behavior that is now out of spec under `REQ-WORKER-0174`.

- [ ] Fix the setup path so the test class runs its assertions instead of skipping.

- [ ] Re-run:
  `just e2e --no-build -v --tags suite_proxy_pma`

- [ ] Verify that the proxy test classes execute real assertions.

Progress:

- [x] Implemented the worker-side managed-service inference wiring gap in `BuildManagedServiceRunArgs` so PMA runtime env is derived from `managed_services.services[].inference`.

- [x] Added targeted worker-node unit tests for inference env propagation, runtime defaults, and host alias override (`worker_node/internal/nodeagent/runargs_test.go`).

- [ ] `suite_proxy_pma` functional classes still intermittently skip on worker health fixture timing in full-tag runs and require focused fixture hardening in `e2e_124_worker_pma_proxy.py`.

### Phase 0 Exit Criteria

- [ ] The inference-tagged subset no longer fails immediately on transport or startup issues.

- [ ] Proxy-functional E2E setup is reliable enough to execute its assertions.

- [ ] A new baseline run is saved for comparison before moving to Phase 1.

Current status note:

- Managed-service PMA inference configuration now uses the orchestrator -> worker desired-state contract and worker runtime application.

- The remaining blockers in Phase 0 are primarily Python E2E harness/setup issues (auth config prereqs, proxy fixture readiness, and SBA helper path handling).

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

### Step 2.1: Tighten Task Result Coverage and Reintroduce Proper Logs Coverage

Primary tests:

- `scripts/test_scripts/e2e_080_task_result.py`
- `scripts/test_scripts/e2e_196_task_list_status_filter.py`

New test to add:

- a replacement `task logs` E2E module with strict contract assertions

Primary Go files to keep in mind:

- `cynork/cmd/task.go`
- `orchestrator/internal/handlers/tasks.go`

Checklist:

- [ ] Make `task result` assertions check exact required fields, not just `status`.

- [ ] Add a replacement `task logs` E2E that checks the expected contract shape rather than only proving that some JSON was returned.

- [ ] Tighten list filtering assertions to the canonical response shape rather than accepting multiple shapes.

- [ ] Re-run only the task subset first, then rerun the full task tag group.

Progress:

- [x] Added a replacement `task logs` E2E module with meaningful JSON assertions.

- [x] Tightened `task result`, `task get`, `task cancel`, and status-filter assertions so they validate more than simple JSON parseability.

### Step 2.2: Add Name-Based Task Identifier Coverage

Primary tests to add or expand:

- `scripts/test_scripts/e2e_070_task_get.py`
- `scripts/test_scripts/e2e_080_task_result.py`
- `scripts/test_scripts/e2e_160_task_cancel.py`

New test to add:

- a replacement `task logs` E2E module that also covers name-based task references

Checklist:

- [ ] Create or reuse a named task in the test setup.

- [ ] Add contract checks using the human-readable task name instead of the UUID.

- [ ] Confirm failure behavior is clear if name resolution is not implemented.

### Step 2.3: Harden Auth Refresh Assertions and Reintroduce Proper Logout Coverage

Primary tests:

- `scripts/test_scripts/e2e_190_auth_refresh.py`

New test to add:

- a replacement auth logout E2E that asserts actual post-logout behavior

Checklist:

- [ ] Assert the refresh flow updates stored credentials.

- [ ] Assert the old refresh token cannot still be treated as the current session token.

- [ ] Add a replacement logout test that asserts logout clears local state and leaves the config in the expected post-logout shape.

- [ ] If the product contract requires server-side invalidation, add an assertion that proves the old access path is rejected.

Progress:

- [x] Added a replacement logout E2E that verifies local tokens are cleared and `whoami` fails after logout.

- [x] Strengthened the refresh test so it verifies stored auth values exist before and after refresh.

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

Progress:

- [x] Removed ad-hoc login from inference chat tests that were mutating shared state in `setUp`.

- [x] Standardized inference-test setup to require the shared auth prereq config and token from `e2e_020_auth_login`.

- [ ] The project-context test still needs a real project identifier instead of the current `"default"` placeholder.

### Step 2.5: Tighten Worker Telemetry Assertions

Primary tests:

- `scripts/test_scripts/e2e_119_worker_telemetry.py`
- `scripts/test_scripts/e2e_122_node_manager_telemetry.py`

Checklist:

- [ ] Assert non-placeholder values for node info and node stats where the spec requires real values.

- [ ] Fix the node-manager source naming to match the canonical telemetry source name.

- [ ] Require lifecycle-event presence where the spec mandates those records.

- [ ] Verify truncated metadata fields exactly where required by the telemetry spec.

### Step 2.6: Add Managed-Agent Proxy and Network-Restriction Acceptance Coverage

Primary test area:

- `scripts/test_scripts/e2e_124_worker_pma_proxy.py`

Primary implementation areas to keep in mind:

- `worker_node/cmd/worker-api/main.go`
- `worker_node/internal/nodeagent/runargs.go`
- `worker_node/cmd/node-manager/main.go`

Checklist:

- [ ] Add a dedicated scenario that exercises the internal managed-agent-to-orchestrator proxy path.

- [ ] Assert the expected status-code behavior for missing identity binding, missing token material, and successful proxying.

- [ ] Keep this distinct from the external bearer-token managed-service proxy coverage.

- [ ] Add acceptance assertions that reflect the updated security contract:
  agent traffic must go through worker proxies in both directions, and direct inbound or outbound network paths to the agent are not acceptable.

- [ ] Make sure proxy-focused acceptance coverage does not bless published-port or other direct-network shortcuts as valid product behavior.

- [ ] If the current implementation still depends on direct-network behavior for orchestrator-to-agent traffic, capture that as an expected implementation gap to be fixed in Phase 3 rather than weakening the acceptance contract.

### Phase 2 Exit Criteria

- [ ] The previously weak green tests now enforce exact contract details.

- [ ] At least one task reference test uses task name rather than UUID for each supported surface.

- [ ] Chat, auth, telemetry, and proxy assertions are strict enough to catch contract drift rather than just endpoint responsiveness.

- [ ] Proxy-focused acceptance tests now enforce the updated agent network-restriction security boundary, not just happy-path proxy responsiveness.

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

### Step 3.6: Reconcile Worker Proxy Behavior With Agent Network Restriction

Primary file:

- `worker_node/cmd/worker-api/main.go`

Additional primary areas:

- `worker_node/internal/nodeagent/runargs.go`
- `worker_node/cmd/node-manager/main.go`

Checklist:

- [ ] Align internal proxy error translation with the expected managed-agent proxy contract.

- [ ] Remove or replace any direct-network orchestrator-to-agent path that conflicts with `REQ-WORKER-0174`.

- [ ] Ensure agent containers are started with network restriction and that both inbound and outbound agent traffic go through worker proxy paths.

- [ ] Replace published-port or equivalent direct-network reachability with a proxy-compatible non-direct path if needed (for example a worker-mediated local binding rather than direct container networking).

- [ ] Re-run the internal proxy acceptance tests after the change.

### Phase 3 Exit Criteria

- [ ] The Go implementation now passes the hardened tests for auth, task, readiness, telemetry, proxy behavior, and agent network restriction.

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
