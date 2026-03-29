# Review Report 6: Testing, BDD, E2E, and CI/CD

- [1 Summary](#1-summary)
- [2 BDD Coverage Analysis](#2-bdd-coverage-analysis)
- [3 E2E Test Analysis](#3-e2e-test-analysis)
- [4 CI Pipeline Analysis](#4-ci-pipeline-analysis)
- [5 Go Test Coverage](#5-go-test-coverage)
- [6 Cross-Cutting Gaps](#6-cross-cutting-gaps)
- [7 Recommended Actions](#7-recommended-actions)

## 1 Summary

This report covers the testing infrastructure across all modules: 54 BDD feature files (337 scenarios), 48 Python E2E test modules (~140 test methods), 19 Godog step files, 90% Go unit test coverage threshold, and the `justfile`-based CI pipeline.

The testing infrastructure is mature and well-documented for its MVP stage.
Tagging discipline is excellent (100% suite/spec/req compliance on feature files), the E2E framework is clean with a prerequisite system, and the Go coverage threshold is enforced.
However, the review surfaces **2 high**, **5 medium**, and **4 low** severity findings.

The most impactful gaps are:

- **No automated CI pipeline** (GitHub Actions or equivalent) -- all CI is local via `just ci`.
- **E2E tests not part of `just ci`** -- requires `RUN_E2E=1` and a running dev stack.
- **44% of requirement domains have zero BDD coverage** (11 of 25 domains).
- **~40% of E2E tests skip** without inference + pexpect dependencies.
- **BDD test coverage is unmeasured** -- Godog tests are not included in coverage profiles.

## 2 BDD Coverage Analysis

Analysis of BDD feature file coverage across requirement domains.

### 2.1 Feature File Inventory

- **54 total feature files** across 5 suite directories
- **337 total scenarios**
- **Structural compliance:** 100% of files have `@suite_*`, `@spec_*`, and `@req_*` tags

#### 2.1.1 Distribution by Suite

- `features/orchestrator/`: 11 files, 67 scenarios
- `features/cynork/`: 24 files, 179 scenarios
- `features/agents/`: 8 files, 28 scenarios
- `features/worker_node/`: 9 files, 49 scenarios
- `features/e2e/`: 2 files, 14 scenarios

### 2.2 Requirement Domain Coverage

14 of 25 requirement domains are referenced by at least one `@req_*` tag:

- **Deep coverage (10+ REQ IDs):** CLIENT (~61), WORKER (~45), ORCHES (~24), USRGWY (~18), PMAGNT (~10)
- **Moderate coverage (2-9):** SBAGNT (7), IDENTY (4), APIEGR (2), SKILLS (2)
- **Fragile (1 REQ ID):** BOOTST, MODELS, SANDBX, MCPGAT, SCHEMA

### 2.3 BDD Coverage Findings

- ⚠️ **11 requirement domains have zero BDD coverage (44%).**
  ACCESS, DATAPI, PROJCT, WEBPRX, MCPTOO, AGENTS, WEBCON, CONNEC, BROWSR, STANDS, STEPEX.
  ACCESS and AGENTS are particularly concerning as they contain security and core agent behavior requirements.

- **11 scenarios marked @wip (3.3%).**
  `pma_chat_file_context.feature` and `sba_inference.feature` are 100% WIP -- every scenario unimplemented.
  These create an illusion of coverage without delivering it.

- **7 feature files have only 1 scenario each** -- extremely thin coverage.
  Single-scenario files typically only cover the happy path.

- **5 domains have only 1 referenced REQ ID** -- fragile single-point coverage.

- **Zero Scenario Outlines** across the entire corpus.
  Several files have repetitive patterns that would benefit from parameterization (e.g., `cynork_tui_slash_model.feature` with 7 near-identical scenarios).

- **E2E suite has only 2 feature files / 14 scenarios.**
  No failure/degradation E2E scenarios (node offline, orchestrator restart, token expiry during streaming).

## 3 E2E Test Analysis

Analysis of the Python E2E test suite.

### 3.1 Test Inventory

- **48 E2E test modules** (`e2e_*.py`)
- **~140 individual test methods**
- **13 helper/infrastructure modules**
- **27 of 48 modules contain skipTest calls** (~120 skip sites)

### 3.2 Domain Coverage

Coverage spans: auth, gateway health, worker node, UDS/inference routing, control plane, task lifecycle, skills, inference/chat, SSE/streaming, SBA, TUI/PTY, GPU variant, MCP tools, artifacts, config/startup.

### 3.3 Skip Analysis

- **~57 of ~140 tests (~40%) skip** without inference + pexpect
- 39 TUI tests skip without `pexpect`
- 18 inference tests skip without Ollama (`E2E_SKIP_INFERENCE_SMOKE`)
- Fast CI run (`--tags no_inference`) still covers ~80 tests

### 3.4 E2E Test Findings

- **No test teardown/cleanup in most E2E tests.**
  Tasks, skills, preferences, and artifacts created during tests are not cleaned up.
  Tests rely on unique IDs (uuid) and ephemeral dev stack.

- **Hardcoded default credentials in `config.py`.**
  `ADMIN_PASSWORD="admin123"`, `NODE_PSK="dev-node-psk-secret"`, `WORKER_API_BEARER_TOKEN="dev-worker-api-token-change-me"`.
  Env-overridable, clearly dev-only, but baked into source.

### 3.5 Positive Notes

- Prerequisite system (`prereqs` + `PREREQ_ORDER` + `PREREQ_ALWAYS_RERUN`) is clean
- Tags enable selective running by domain, inference requirement, and component
- Tests numbered for deterministic ordering with gaps for insertion
- Config fully env-driven with sensible dev defaults
- Token management handles long-running suites with JWT expiry
- `check-e2e-tags` and `check-e2e-requirements-traces` CI validators enforce conventions

## 4 CI Pipeline Analysis

Analysis of the CI pipeline configuration and coverage.

### 4.1 Target Inventory

- **`just ci`**: `build-dev` + `lint` + `vulncheck-go` + `test-go-cover` + `bdd-ci` (**no E2E**)
- **`just lint`**: 14 targets including `lint-sh`, `lint-go`, `lint-go-ci`, `go-fmt`, `lint-python`, `lint-md`, `validate-doc-links`, `validate-feature-files`, `check-e2e-tags`, `check-e2e-requirements-traces`, `lint-gherkin`, `lint-containerfiles`
- **`just test-go-cover`**: Go unit tests with 90% coverage threshold per package; Podman testcontainers for orchestrator DB tests
- **`just test-go-race`**: Go unit tests with `-race` in all modules
- **`just test-bdd` / `just bdd-ci`**: Godog BDD across 5 modules; `GODOG_STRICT=1` in `bdd-ci`
- **`just e2e`**: Python E2E suite (requires running stack; **not** called by `ci`)
- **`just docs-check`**: `lint-md` + `validate-doc-links` + `check-tech-specs` + `validate-requirements` + `validate-feature-files`

### 4.2 Pipeline Findings

- ❌ **No GitHub Actions / CI pipeline definition found.**
  No `.github/workflows/` YAML or equivalent.
  All CI is local via `just ci`.
  CI discipline depends entirely on developer compliance.

- ⚠️ **E2E tests not part of `just ci`.**
  Regressions in API contracts, auth flows, streaming, and TUI can be merged without detection.

- **Python E2E test linting excluded from `lint-python`.**
  `scripts/test_scripts` is not included in default lint paths.
  E2E test code may accumulate style issues and potential bugs undetected by CI.

## 5 Go Test Coverage

Analysis of Go unit test coverage configuration and measurement.

### 5.1 Coverage Configuration

- **Global minimum:** 90% per Go package
- **Per-package overrides:** control-plane, mcp-gateway, agents, SBA, SBA cmd, securestore all at 90%
- **Exempt:** `internal/testutil` (threshold = 0)
- **Enforcement:** `test-go-cover` parses `go test -coverprofile` output and fails if any package falls below threshold

### 5.2 Coverage Findings

- **BDD test coverage unmeasured.**
  Godog BDD tests run in separate `_bdd` packages and are not included in coverage profiles.
  Their contribution to code coverage is invisible.
  Coverage reports undercount actual exercised code.

## 6 Cross-Cutting Gaps

Testing gaps that span multiple modules and test layers.

### 6.1 Missing Testing Layers

- **No Phase 3/4 E2E coverage** (expected for MVP; mentioned for completeness).
  No tests for: multi-node orchestration, cloud LLM API quota, production deployment (K8s, TLS), RBAC beyond admin, audit logging, horizontal scaling.

- **No chaos/failure E2E scenarios.**
  Missing: node offline mid-task, orchestrator restart during active task, auth token expiry during streaming, DB failover.

- **No load/performance testing.**
  No benchmarks for streaming throughput, concurrent chat sessions, or dispatcher under load.

### 6.2 Coverage vs Requirements Summary

- **Go unit tests:** 90%+ per package (enforced)
- **BDD scenarios:** 337 across 54 files covering 14/25 domains
- **E2E tests:** ~140 methods across 48 modules covering Phase 2 MVP
- **Unmeasured:** BDD contribution to code coverage, integration coverage of cross-module interactions

## 7 Recommended Actions

Remediation items organized by priority tier.

### 7.1 P0 -- Immediate

1. **Add GitHub Actions CI workflow** that runs `just ci` on PR and push to protected branches.
2. **Add a second CI workflow** that starts the dev stack and runs `just e2e --tags no_inference` for fast E2E validation.

### 7.2 P1 -- Short-Term

1. **Add BDD feature files** for the 11 uncovered requirement domains, prioritizing ACCESS, AGENTS, MCPGAT, and MCPTOO.
2. **Implement the 11 @wip scenarios**, especially `pma_chat_file_context.feature` and `sba_inference.feature`.
3. **Add `scripts/test_scripts` to `lint-python`** default paths or create a `lint-e2e-python` target.
4. **Document the minimum CI matrix**: (1) `no_inference` fast pass, (2) `inference` pass with Ollama, (3) `tui` pass with pexpect.

### 7.3 P2 -- Planned

1. **Merge BDD coverage into Go coverage profiles** using `-coverpkg=./...` or document BDD as a separate metric.
2. **Add negative-path BDD scenarios** for auth (invalid credentials, expired tokens) and MCP (unauthorized tool access).
3. **Add Scenario Outlines** for repetitive patterns to reduce duplication.
4. **Add test cleanup** for E2E tests creating persistent state (preferences, system settings, artifacts).
5. **Expand E2E suite feature files** to include failure/degradation scenarios.

### 7.4 P3 -- Longer-Term

1. Add load/performance benchmarks for streaming, concurrent chat, and dispatcher.
2. Add chaos/failure E2E scenarios (node loss, orchestrator restart, DB failover).
3. Track Phase 3/4 E2E gaps in the roadmap.
4. Add full-demo CI job with Ollama + pexpect for complete E2E validation.
