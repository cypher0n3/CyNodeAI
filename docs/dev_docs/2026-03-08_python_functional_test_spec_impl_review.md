# Python Functional Test Review Against Specs and Go Implementation

- [Scope](#scope)
- [Summary](#summary)
- [Specification Compliance Issues](#specification-compliance-issues)
- [Recommended Refactor Strategy](#recommended-refactor-strategy)

## Scope

Date: 2026-03-08.

This review covers all 35 Python E2E modules under `scripts/test_scripts/e2e_*.py`.

It also considers the shared E2E harness in `scripts/test_scripts/helpers.py`, `scripts/test_scripts/config.py`, `scripts/test_scripts/e2e_state.py`, `scripts/test_scripts/e2e_tags.py`, and `scripts/test_scripts/run_e2e.py`.

The review is specification-first.

Canonical behavior was taken from `docs/requirements/*.md`.

Implementation guidance was taken from the relevant `docs/tech_specs/*.md`.

Go implementation drift was checked primarily in `cynork/`, `orchestrator/`, `worker_node/`, and `go_shared_libs/`.

The review was originally static.

This update also incorporates runtime evidence from `docs/dev_docs/2026-03-08_0118_e2e_results.txt`, which captured a `just setup-dev test-e2e` run with 57 tests, 8 failures, and 5 skips.

## Summary

The current Python functional suite is useful as smoke coverage, but it is not yet a reliable acceptance suite for the published contract.

The strongest recurring issue is that several tests validate the current implementation or MVP stub behavior instead of enforcing the canonical requirements and tech specs.

The second recurring issue is under-assertion.

Many tests only check `200`, non-empty output, or the existence of a few JSON keys, which allows materially non-compliant Go implementations to pass.

The highest-risk gaps are in task CLI/API coverage, chat project-context coverage, worker telemetry coverage, and worker managed-agent proxy coverage.

Several real Go implementation drifts are already visible and would likely fail once the tests are tightened to the spec.

The runtime E2E evidence strengthens this conclusion.

Most broad smoke paths pass, but the inference-heavy paths fail in a concentrated cluster, while several spec-drift areas still pass because the tests are too permissive.

## Review Method

The review followed the repo guidance in `.github/copilot-instructions.md`, `meta.md`, and the `justfile`.

Repo recipes were inventoried with `just --list`.

The Python suite structure was reviewed via `run_e2e.py` and the tag registry in `e2e_tags.py`.

Each E2E module was mapped to the relevant requirement and tech spec area based on explicit `# Traces:` comments where present and inferred behavior where not present.

Relevant Go handlers and CLI code were then checked for contract alignment or likely failure points.

After the initial static review, the observed E2E run in `docs/dev_docs/2026-03-08_0118_e2e_results.txt` was used as runtime evidence to confirm or refine the findings.

## Runtime E2E Evidence

The recorded `just setup-dev test-e2e` run materially confirms the main thesis of this review.

The suite currently behaves like a mixed smoke-plus-acceptance suite rather than a strict contract suite.

### What the Runtime Results Confirm

- Basic CRUD and smoke coverage is mostly green.
  Status, login, whoami, task create, task list, task get, task result, skills CRUD smoke, workflow start, node register, capability report, and several worker health surfaces passed.

- Several tests pass while still validating known drift or weak assertions.
  `scripts/test_scripts/e2e_118_api_egress_call.py` passed by asserting `501 Not Implemented`, which confirms it is validating the current stub rather than the required API egress contract.

- `scripts/test_scripts/e2e_119_worker_telemetry.py` passed even though the current worker telemetry handlers still return placeholder values in `worker_node/cmd/worker-api/main.go`.
  This confirms the current telemetry tests are too shallow to detect real contract gaps.

- `scripts/test_scripts/e2e_120_worker_api_health_readyz.py` and `scripts/test_scripts/e2e_195_gateway_health_readyz.py` passed with permissive readiness expectations.
  This confirms those tests currently prove only surface responsiveness, not readiness semantics.

### Runtime-Failing Cluster

The failing tests are strongly concentrated in inference-dependent flows:

- `scripts/test_scripts/e2e_090_task_inference.py`
- `scripts/test_scripts/e2e_110_task_models_and_chat.py`
- `scripts/test_scripts/e2e_115_pma_chat_context.py`
- `scripts/test_scripts/e2e_140_sba_task_inference.py`
- `scripts/test_scripts/e2e_145_sba_inference_reply.py`
- `scripts/test_scripts/e2e_192_chat_reliability.py`
- `scripts/test_scripts/e2e_193_chat_sequential_messages.py`
- `scripts/test_scripts/e2e_194_chat_simultaneous_messages.py`

This narrows immediate runtime risk substantially.

The current failures are not spread evenly across the platform.

They are concentrated around orchestrator-side chat inference, SBA inference execution, and worker-reachable inference paths.

### Runtime Symptoms That Narrow Root Cause

- Chat path failures repeatedly surfaced as `502 Bad Gateway` from `cynork chat` or as structured `orchestrator_inference_failed` errors.
  This indicates a real runtime failure in the orchestrator inference chain, not merely weak assertions.

- `scripts/test_scripts/e2e_140_sba_task_inference.py` failed with a timeout posting to `http://host.containers.internal:12090/v1/worker/jobs:run`.
  That points to worker API reachability, startup, or readiness issues on the SBA dispatch path.

- `scripts/test_scripts/e2e_124_worker_pma_proxy.py` did not execute its functional assertions because its worker API setup never became healthy.
  This confirms that one of the most important worker-proxy areas remains effectively unvalidated in the observed run.

- `scripts/test_scripts/e2e_121_worker_api_managed_service.py` and `scripts/test_scripts/e2e_122_secure_store_envelope_structure.py` were partially skipped due to missing environment setup.
  Those areas should therefore not be treated as runtime-validated.

## Test Inventory

The inventory below groups all reviewed Python E2E modules by the surface they target.

### Cynork, Auth, and Task E2E

- `scripts/test_scripts/e2e_010_cli_version_and_status.py`: CLI version smoke and gateway health smoke.
- `scripts/test_scripts/e2e_020_auth_login.py`: login persists tokens in config.
- `scripts/test_scripts/e2e_030_auth_negative_whoami.py`: unauthenticated `whoami` fails closed.
- `scripts/test_scripts/e2e_040_auth_whoami.py`: authenticated identity lookup works.
- `scripts/test_scripts/e2e_050_task_create.py`: task creation, task naming, and attachments.
- `scripts/test_scripts/e2e_060_task_list.py`: task listing.
- `scripts/test_scripts/e2e_070_task_get.py`: task fetch by ID.
- `scripts/test_scripts/e2e_080_task_result.py`: task result retrieval.
- `scripts/test_scripts/e2e_100_task_prompt.py`: prompt-mode task completion.
- `scripts/test_scripts/e2e_150_task_logs.py`: task logs retrieval.
- `scripts/test_scripts/e2e_160_task_cancel.py`: task cancellation.
- `scripts/test_scripts/e2e_190_auth_refresh.py`: token refresh.
- `scripts/test_scripts/e2e_196_task_list_status_filter.py`: task list filtering by status.
- `scripts/test_scripts/e2e_200_auth_logout.py`: logout clears local auth state.

### Orchestrator, Gateway, Chat, and Control Plane E2E

- `scripts/test_scripts/e2e_110_task_models_and_chat.py`: models list and one-shot chat.
- `scripts/test_scripts/e2e_115_pma_chat_context.py`: project-context chat path.
- `scripts/test_scripts/e2e_116_skills_gateway.py`: gateway-backed skills CRUD smoke.
- `scripts/test_scripts/e2e_117_workflow_api.py`: workflow start path.
- `scripts/test_scripts/e2e_118_api_egress_call.py`: API egress surface smoke.
- `scripts/test_scripts/e2e_170_control_plane_node_register.py`: node registration bootstrap.
- `scripts/test_scripts/e2e_175_prescribed_startup_config_inference_backend.py`: orchestrator-provided inference backend config.
- `scripts/test_scripts/e2e_180_control_plane_capability.py`: node capability report.
- `scripts/test_scripts/e2e_192_chat_reliability.py`: chat reliability and timeout handling.
- `scripts/test_scripts/e2e_193_chat_sequential_messages.py`: multi-turn chat continuity.
- `scripts/test_scripts/e2e_194_chat_simultaneous_messages.py`: concurrent chat requests.
- `scripts/test_scripts/e2e_195_gateway_health_readyz.py`: gateway liveness versus readiness.

### Worker Node, Worker API, Proxy, Telemetry, and SBA E2E

- `scripts/test_scripts/e2e_090_task_inference.py`: task uses node-local inference path.
- `scripts/test_scripts/e2e_119_worker_telemetry.py`: worker telemetry surface.
- `scripts/test_scripts/e2e_120_worker_api_health_readyz.py`: worker API health and readiness.
- `scripts/test_scripts/e2e_121_worker_api_managed_service.py`: worker API under node-manager managed-service control.
- `scripts/test_scripts/e2e_122_node_manager_telemetry.py`: node-manager lifecycle logs.
- `scripts/test_scripts/e2e_122_secure_store_envelope_structure.py`: secure-store envelope shape for managed-service agent tokens.
- `scripts/test_scripts/e2e_123_sba_task.py`: SBA task end-to-end success.
- `scripts/test_scripts/e2e_124_worker_pma_proxy.py`: worker managed-service proxy and PMA handoff.
- `scripts/test_scripts/e2e_130_sba_task_result_contract.py`: SBA result contract.
- `scripts/test_scripts/e2e_140_sba_task_inference.py`: SBA inference success.
- `scripts/test_scripts/e2e_145_sba_inference_reply.py`: reply-bearing SBA result.

## Specification Compliance Issues

The issues below are ordered by impact on contract coverage and risk of false confidence.

### Critical Findings

- Several tests currently codify implementation drift instead of the spec.
  `scripts/test_scripts/e2e_020_auth_login.py` validates `cynork auth login -u ... -p ...`, while `docs/tech_specs/cli_management_app_commands_core.md` requires `--handle` and `--password-stdin`, and `cynork/cmd/auth.go` still exposes `--username/-u`, `--password/-p`, and prompts with `Username:` rather than the spec language.

- `scripts/test_scripts/e2e_118_api_egress_call.py` validates the current `501` stub path rather than the required access-control flow.
  In `orchestrator/cmd/api-egress/main.go`, the no-DSN path only checks a static provider allowlist and then returns `501`, which is materially short of `REQ-APIEGR-0110` and related API egress requirements around subject resolution, policy evaluation, task context, and credential validation.

- Task identifier coverage is materially incomplete, and the current Go handlers are out of spec.
  `docs/tech_specs/cli_management_app_commands_tasks.md` requires task references to accept either UUID or human-readable task name, but the Python task tests use UUIDs only, and `orchestrator/internal/handlers/tasks.go` hard-parses `uuid.Parse` in `GetTask`, `GetTaskResult`, `CancelTask`, and `GetTaskLogs`, so name-based requests would fail.

- `scripts/test_scripts/e2e_115_pma_chat_context.py` is a false positive for project-context behavior.
  The test uses `--project-id default`, but `orchestrator/internal/handlers/openai_chat.go` `projectIDFromHeader()` accepts only UUIDs and silently ignores invalid values, while `docs/tech_specs/openai_compatible_chat_api.md` and `docs/tech_specs/cli_management_app_commands_chat.md` require proper project-context association and default-project fallback semantics.

### High Findings

- `scripts/test_scripts/e2e_150_task_logs.py` and `scripts/test_scripts/e2e_080_task_result.py` are too weak to enforce the published task contract.
  `cynork/cmd/task.go` `printTaskResult()` emits a reduced JSON shape that drops `jobs`, omits `task_name`, and synthesizes `stdout`, while `runTaskLogs()` simply prints `stdout` and `stderr` from `TaskLogsResponse` and does not expose the spec-required `--stream` and `--follow` behavior.

- `scripts/test_scripts/e2e_050_task_create.py`, `scripts/test_scripts/e2e_160_task_cancel.py`, and related task tests normalize non-canonical CLI flags and mode semantics.
  The task spec requires `--attach` and `--name`, but tests use aliases such as `--attachment` and `--task-name`, and the prompt-mode tests often use shell-looking payloads like `echo ...` and `sleep 300`, which blurs the spec distinction between prompt mode and explicit script or command modes.

- `scripts/test_scripts/e2e_195_gateway_health_readyz.py` is validating the wrong readiness semantics.
  `orchestrator/cmd/user-gateway/main.go` hardcodes `GET /readyz` to `200 ready`, while the meaningful readiness gating described by `REQ-ORCHES-0120` exists on the control plane rather than on the gateway.

- `scripts/test_scripts/e2e_175_prescribed_startup_config_inference_backend.py` checks only partial startup config semantics.
  `orchestrator/internal/handlers/nodes.go` `deriveInferenceBackend()` can mark inference enabled without proving the full backend startup details required by `REQ-ORCHES-0149`.

- `scripts/test_scripts/e2e_124_worker_pma_proxy.py` covers the bearer-token managed-service proxy path, but it does not exercise the internal managed-agent-to-orchestrator proxy path that is central to `REQ-WORKER-0162`.
  The current internal path in `worker_node/cmd/worker-api/main.go` `validateInternalProxyRequest()` returns `401` for missing caller identity and missing token record, while the documented proxy semantics call for stricter distinction between forbidden identity resolution and unavailable token material.

- The runtime E2E results show an immediate inference-path reliability problem in addition to the static contract gaps.
  Chat E2E failures returned repeated `502 Bad Gateway` and `orchestrator_inference_failed`, `scripts/test_scripts/e2e_090_task_inference.py` failed because expected inference wiring was absent from task output, and `scripts/test_scripts/e2e_140_sba_task_inference.py` failed with a timeout reaching the worker API jobs endpoint.

## Architectural Issues

- The E2E suite is heavily order-coupled.
  `scripts/test_scripts/e2e_state.py` shares mutable global state across modules, and `scripts/test_scripts/e2e_tags.py` force-includes prerequisite modules when tag filters are used.

- This shared-state design amplifies cascaded failure.
  A login or task-create failure can make multiple downstream tests fail for setup reasons instead of for the contract they claim to validate.

- The suite presently mixes smoke checks and acceptance checks without clearly separating them.
  That makes it easy for a test to pass even when it only proves process reachability or JSON parseability rather than spec compliance.

## Security Risks

- `scripts/test_scripts/e2e_200_auth_logout.py` and `scripts/test_scripts/e2e_190_auth_refresh.py` under-test session invalidation and token rotation.
  `REQ-IDENTY-0105` requires refresh-token rotation, but the tests do not verify old-token invalidation, config mutation, or rejection of stale credentials after refresh.

- The gateway auth path appears weaker than the requirements imply.
  `REQ-IDENTY-0122` says the gateway must validate the access token on every request, but the current auth path relies on JWT validation and the E2E suite does not verify server-side revocation or post-logout rejection of previously issued access tokens.

- `scripts/test_scripts/e2e_122_secure_store_envelope_structure.py` validates only a partial envelope shape and checks for the wrong secret leakage.
  The secure-store schema in `worker_node/internal/securestore/store.go` includes `nonce_b64` and conditionally `kem_ciphertext_b64`, but the test only checks a subset of fields and only verifies absence of the worker bearer token rather than absence of the managed-service agent token content that the file actually stores.

- `scripts/test_scripts/e2e_124_worker_pma_proxy.py` uses fixed local ports across multiple scenarios.
  That is a flake risk on shared developer hosts and encourages hidden environmental coupling around proxy security tests.

## Performance Concerns

- `scripts/test_scripts/e2e_194_chat_simultaneous_messages.py` is too permissive for concurrency validation.
  The test tolerates partial success instead of asserting a clear reliability threshold, which means it cannot catch degraded gateway or PMA concurrency behavior early.

- `scripts/test_scripts/e2e_192_chat_reliability.py` and `scripts/test_scripts/e2e_193_chat_sequential_messages.py` mostly assert non-empty output rather than deterministic content.
  This leaves routing, retry, timeout, and persistence regressions largely unobserved.

## Maintainability Issues

- Traceability is inconsistent across the Python E2E suite.
  Some modules include strong `# Traces:` metadata, but many still rely on inferred mapping to requirements and specs.

- Several tests accept multiple JSON shapes or multiple status codes where the spec is more precise.
  This is particularly visible in task list, task result, worker readiness, and gateway readiness tests.

- `scripts/test_scripts/helpers.py` injects `CYNORK_GATEWAY_URL` for every CLI call, which prevents the suite from catching default-config drift.
  This hides issues such as a mismatch between configured default ports in CLI config code and the ports assumed by the dev stack.

## Concrete Go Implementation Deviations Likely to Cause Failures

- `cynork/cmd/auth.go`: login flags and interactive prompts do not match the canonical CLI spec.

- `cynork/cmd/task.go`: `printTaskResult()` and `runTaskLogs()` do not emit or validate the richer task result and logs contracts described in `docs/tech_specs/cli_management_app_commands_tasks.md`.

- `orchestrator/internal/handlers/tasks.go`: task retrieval, result, cancel, and logs paths require UUID-only identifiers and therefore do not satisfy task-name addressing.

- `orchestrator/internal/handlers/openai_chat.go`: `projectIDFromHeader()` silently ignores non-UUID `OpenAI-Project` values, and the suite does not currently prove default-project fallback semantics end to end.

- `orchestrator/cmd/user-gateway/main.go`: gateway `/readyz` is hardcoded `200 ready`, so gateway readiness tests cannot currently enforce meaningful readiness semantics.

- `orchestrator/cmd/api-egress/main.go`: the non-DB code path is still a stub and cannot satisfy the API egress requirements being claimed by the test suite.

- `orchestrator/internal/handlers/nodes.go`: the inference-backend configuration assertions are only partially covered by the current tests and likely need stronger contract validation.

- `worker_node/cmd/worker-api/main.go`: `handleNodeInfo()` and `handleNodeStats()` return placeholder telemetry values, but `scripts/test_scripts/e2e_119_worker_telemetry.py` only checks top-level key presence.

- `worker_node/cmd/worker-api/main.go`: `validateInternalProxyRequest()` returns status codes that do not cleanly reflect the managed-agent internal proxy contract described in worker specs and prior worker proxy design work.

- `worker_node/internal/securestore/store.go`: the secure-store envelope is richer than the current Python test validates.

## File-Level Assessment

The following split highlights where the suite is strongest versus where it is currently weakest relative to the published contract.

### Strongest Tests

- `scripts/test_scripts/e2e_120_worker_api_health_readyz.py`: good baseline surface coverage, but the assertions still need to be exact.

- `scripts/test_scripts/e2e_170_control_plane_node_register.py`: useful bootstrap-path smoke, but it needs deeper payload validation.

- `scripts/test_scripts/e2e_123_sba_task.py`: useful E2E spine for SBA task flow, but it still needs stronger contract assertions on the final result.

### Runtime-Proven Problem Areas

- `scripts/test_scripts/e2e_110_task_models_and_chat.py`: runtime failure confirms the chat inference path is currently unstable.

- `scripts/test_scripts/e2e_115_pma_chat_context.py`: runtime failure confirms this path is not merely under-specified; it is also failing operationally.

- `scripts/test_scripts/e2e_140_sba_task_inference.py`: runtime failure confirms a real worker-dispatch or worker-reachability issue in the SBA inference flow.

- `scripts/test_scripts/e2e_145_sba_inference_reply.py`: runtime failure confirms that user-facing SBA reply behavior is not currently reliable.

- `scripts/test_scripts/e2e_192_chat_reliability.py`, `scripts/test_scripts/e2e_193_chat_sequential_messages.py`, and `scripts/test_scripts/e2e_194_chat_simultaneous_messages.py`: runtime failures confirm the current chat path is failing before higher-order reliability semantics can even be trusted.

- `scripts/test_scripts/e2e_124_worker_pma_proxy.py`: runtime skips confirm this area needs setup hardening before it can provide meaningful acceptance coverage.

### Weakest Tests Relative to the Spec

- `scripts/test_scripts/e2e_020_auth_login.py`: validates non-canonical flags rather than the published CLI interface.

- `scripts/test_scripts/e2e_080_task_result.py`: under-asserts the task result shape.

- `scripts/test_scripts/e2e_150_task_logs.py`: effectively only asserts that some JSON came back.

- `scripts/test_scripts/e2e_115_pma_chat_context.py`: does not prove actual project association.

- `scripts/test_scripts/e2e_118_api_egress_call.py`: validates a stubbed implementation path.

- `scripts/test_scripts/e2e_119_worker_telemetry.py`: too shallow to catch the current placeholder telemetry implementation.

- `scripts/test_scripts/e2e_124_worker_pma_proxy.py`: does not cover the most security-sensitive proxy path.

## Recommended Refactor Strategy

The remediation plan below is ordered to stop false positives first, then harden assertions, then close the Go implementation gaps that stronger tests will expose.

The runtime E2E evidence changes one important prioritization detail.

Inference-path stability now needs to come first, because multiple chat and SBA tests are failing before deeper contract assertions can be trusted.

### Phase 0: Restore Inference-Path Operability

- Fix the orchestrator-side chat inference path that is currently returning `502 Bad Gateway` and `orchestrator_inference_failed` in the observed E2E run.

- Fix the SBA inference dispatch path that is timing out on `POST /v1/worker/jobs:run`.

- Make `scripts/test_scripts/e2e_124_worker_pma_proxy.py` reliably start its worker API fixture so the proxy scenarios run instead of skipping during setup.

- Re-run the failing inference subset first after each fix so later contract-tightening work is not obscured by baseline runtime outages.

### Phase 1: Stop Locking in Known Drift

- Update E2E expectations so they enforce canonical CLI flag names and output contracts rather than current aliases and reduced JSON shapes.

- Split smoke tests from acceptance tests explicitly.
  A smoke test may tolerate current MVP behavior, but an acceptance test must fail on published contract drift.

- Remove ambiguous prompt-mode shell examples from task tests.
  Add separate coverage for `prompt`, `script`, and command-oriented modes.

### Phase 2: Tighten the Highest-Value Assertions

- For task tests, assert exact JSON fields for `task create`, `task get`, `task result`, `task logs`, and `task cancel`, including `task_name`, canonical status values, and job-level result shape.

- Add explicit name-based identifier coverage for `task get`, `task result`, `task cancel`, and `task logs`.

- For auth tests, assert refresh-token rotation, config-file mutation, logout local-state clearing, and rejection of stale credentials after refresh or logout.

- For chat tests, assert deterministic content where prompts make that possible, and assert project association behavior using a real project ID instead of `"default"`.

- For worker telemetry tests, assert meaningful field values rather than just key presence.
  This should include non-empty kernel version, plausible resource stats, correct source names, and lifecycle-event presence.

- For worker proxy tests, add a dedicated internal managed-agent proxy test that validates identity binding, token lookup, and status-code semantics.

### Phase 3: Fix Go Drifts That Hardened Tests Will Expose

- Align `cynork/cmd/auth.go` with the CLI auth spec.

- Align `cynork/cmd/task.go` output and flags with the task command spec.

- Add task-name resolution to `orchestrator/internal/handlers/tasks.go` and the underlying data access layer.

- Revisit gateway readiness semantics so the tested surface matches the intended contract.

- Replace placeholder telemetry in `worker_node/cmd/worker-api/main.go` with real node info and stats collection.

- Reconcile the internal worker proxy error-status contract with the worker proxy spec and acceptance tests.

### Phase 4: Reduce Flakiness and Suite Coupling

- Minimize dependence on `e2e_state.py` by creating self-contained fixtures where feasible.

- Keep prerequisite setup only for tests that truly need chained state.

- Replace fixed ports in proxy-focused tests with dynamically allocated ports where practical.

- Add one config-resolution test that does not override `CYNORK_GATEWAY_URL`, so default CLI config drift is visible to the suite.

## Priority Order

1. Restore chat and SBA inference-path operability, because the runtime E2E results show this is the dominant active failure cluster.

2. Make the worker proxy functional E2E setup reliable so proxy acceptance coverage actually executes.

3. Tighten task result and task logs tests, then fix the matching CLI and gateway contract drift.

4. Add task-name identifier tests, then implement task-name resolution in the Go handlers.

5. Split API egress smoke from true acceptance coverage, because the current test is giving false confidence.

6. Tighten worker telemetry tests, because the current Go implementation is visibly placeholder-heavy.

7. Replace the PMA chat project-context test with a real project-association test.

8. Add internal managed-agent proxy acceptance coverage and reconcile the worker proxy status-code behavior.

9. Harden refresh and logout tests to cover token rotation and invalidation semantics.

## Final Assessment

The current Python functional suite is directionally useful, but it is still closer to an integration smoke suite than a specification-enforcing acceptance suite.

The most important remediation is not simply adding more tests.

It is ensuring the tests stop accepting drift and start enforcing the published contract exactly where the Go code is already known to diverge.

The runtime E2E results also show that inference-path operability must be restored before the suite can serve as a dependable acceptance signal for chat and SBA behavior.
