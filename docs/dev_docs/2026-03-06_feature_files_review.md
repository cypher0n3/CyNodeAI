# Feature Files and BDD Implementation Review

## Metadata

- Date: 2026-03-06
- Scope: All Gherkin `.feature` files under `features/` and their BDD (Godog) implementation.
- Purpose: Alignment with current functionality, BDD coverage, and skipped steps.

## Summary

- **29 feature files** across five suites; all describe current or explicitly @wip behavior.
- **Four suites** have Godog BDD runners (`orchestrator`, `worker_node`, `cynork`, `agents`); **one suite** (`e2e`) has no BDD runner.
- **One scenario** is tagged @wip and excluded from BDD via `~@wip`.
- **Skipped steps** exist only in the orchestrator BDD suite: conditional skips when prerequisites are missing, plus five always-skip stub steps for E2E/worker wording.

## Validation Status

- `just validate-feature-files` passes for all 29 files (suite tags, narrative block, scenario traceability tags).
- Suite tag registry in `features/README.md` matches the five suites in use.
- Four additional suites have no feature files yet (admin_web_console, api_egress_server, secure_browser_service, mcp_gateway).

## Inventory by Suite

- **orchestrator** (8 files, `features/orchestrator/`): Startup, auth, task lifecycle, node registration, workflow/lease, chat reliability, API egress call, initial auth.
  API egress describes stub contract (501/403/401).
- **worker_node** (7 files, `features/worker_node/`): Sandbox execution, node manager config/startup, SBA, secure store, telemetry, inference proxy, internal proxy.
  One scenario @wip (see below).
- **agents** (6 files, `features/agents/`): SBA runner, contract, lifecycle, inference, tools, failure codes; PMA chat/context.
- **cynork** (6 files, `features/cynork/`): Status/auth, skills, shell, chat, tasks, admin.
- **e2e** (2 files, `features/e2e/`): Single-node happy path, chat OpenAI-compatible.
  Some scenarios @inference_in_sandbox (run when inference available).

## BDD Implementation

- **`just test-bdd`** runs Godog for each Go module that has an `_bdd` directory (from `go.work`: orchestrator, worker_node, cynork, agents).
- Each suite's `_bdd/suite_test.go` sets `Paths: []string{featurePath()}` to its own `features/<suite>/` directory and `Tags: "~@wip"`.
- **orchestrator**: `orchestrator/_bdd` -> `features/orchestrator/`.
  DB-dependent scenarios require `POSTGRES_TEST_DSN` or testcontainers (see `orchestrator/_bdd/testmain_test.go`).
- **worker_node**: `worker_node/_bdd` -> `features/worker_node/`.
  Steps in `steps.go` cover sandbox execution, node manager config, SBA, secure store, telemetry, inference proxy, internal proxy (public mux 404).
- **cynork**: `cynork/_bdd` -> `features/cynork/`.
- **agents**: `agents/_bdd` -> `features/agents/`.
- **e2e**: No `_bdd` package.
  `features/e2e/` is never run by Godog; similar flows are covered by the Python E2E suite (`just e2e`).

## Missing Actual BDD Tests

**Suite @suite_e2e** (2 feature files) has no Godog BDD runner:

- `features/e2e/single_node_happy_path.feature`
- `features/e2e/chat_openai_compatible.feature`

Their Gherkin scenarios are specification-only for BDD; behavior is partially covered by Python E2E (`scripts/test_scripts/run_e2e.py`).

## Skipped Steps

Only the **orchestrator** BDD suite uses `godog.ErrSkip`.

- **Conditional skips:** Many steps in `orchestrator/_bdd/steps.go` return `ErrSkip` when required state is missing (e.g. no `POSTGRES_TEST_DSN`, no server, no DB, no `nodeJWT`, no `taskID`).
  When prerequisites are not met, the scenario is skipped at the first such step (intentional when running without a DB).
- **Always-skip stubs:** Five steps always return `ErrSkip` and are marked "E2E / worker (stubs)" in `orchestrator/_bdd/steps.go`:
  - `a worker node is running and reachable by the orchestrator`
  - `the orchestrator dispatches a job to the node`
  - `the node executes the sandbox job`
  - `the job result contains stdout "…"`
  - `the task status becomes "…"`
  They match wording in `features/e2e/single_node_happy_path.feature`, but the orchestrator suite only runs `features/orchestrator/`, so no scenario executed by `just test-bdd` invokes them.

**worker_node**, **cynork**, and **agents** _bdd steps do not use `ErrSkip`.

## Exception: @Wip Scenario

One scenario is explicitly tagged **@wip** and excluded by all suites' `~@wip` filter:

- **File:** `features/worker_node/node_manager_config_startup.feature`
- **Scenario:** "Node manager fail-fast when inference startup fails"
- **Tags:** `@req_worker_0002` `@spec_cynai_worker_failfast` `@wip`

It is not expected to pass until fail-fast-on-inference-startup is implemented.

## Optional / Environment-Gated Scenarios

- **@inference_in_sandbox:** Scenarios requiring an inference-capable node; E2E can skip with `--skip-ollama` or when inference is unavailable.
- **@phase1_sandbox:** Sandbox network_policy and context env; exercised by BDD when the worker node is configured for phase-1 sandbox.

These describe current behavior; tags mean "run when capability is present."

## Suites With No Feature Files

Registry only: `@suite_admin_web_console`, `@suite_api_egress_server`, `@suite_secure_browser_service`, `@suite_mcp_gateway`.

## Conclusion

- **29 feature files**; 28 aligned with current functionality, 1 scenario @wip.
- **27 files** are run by Godog via their suite's `_bdd` package; **2 files** (e2e) have no BDD runner.
- **Skipped steps:** orchestrator only (conditional + 5 stubs); other suites have none.

## References

- `features/README.md` (suite registry, traceability)
- `scripts/test_scripts/README.md` (E2E layout and run order)
- `orchestrator/_bdd/steps.go` (conditional and stub skips)
- `docs/dev_docs/2026-03-06_phase1_4_independent_validation_gap_report.md`
- `docs/dev_docs/2026-03-06_worker_proxy_phase5_6_validation.md`
- `.ci_scripts/validate_feature_files.py`
