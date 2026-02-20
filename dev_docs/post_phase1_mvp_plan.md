# Post-Phase 1 MVP Plan

- [1 Objectives](#1-objectives)
- [2 Scope Summary](#2-scope-summary)
  - [2.1 Current Status (2026-02-20 0637)](#21-current-status-2026-02-20-0637)
- [3 Inference in Sandboxed Containers](#3-inference-in-sandboxed-containers)
  - [3.1 Inference Proxy Sidecar (Per `node.md` Option A)](#31-inference-proxy-sidecar-per-nodemd-option-a)
  - [3.2 Implementation Ownership](#32-implementation-ownership)
  - [3.3 E2E Scenario](#33-e2e-scenario)
- [4 Feature Files](#4-feature-files)
- [5 Unit Tests and Coverage](#5-unit-tests-and-coverage)
- [6 BDD Suite](#6-bdd-suite)
- [7 CLI App (Separate Go Module)](#7-cli-app-separate-go-module)
- [8 Implementation Order](#8-implementation-order)
- [9 References](#9-references)

## 1 Objectives

This plan extends the MVP past Phase 1 so that:

1. **Full single-node execution with inference:** The orchestrator can dispatch work that runs in sandboxed containers and uses node-local inference (Ollama) from inside the sandbox, per `docs/tech_specs/node.md` and `docs/tech_specs/sandbox_container.md`.
2. **Feature files** are built out for new behavior and any Phase 1 gaps called out in the code review.
3. **Unit tests** maintain or achieve at least 90% code coverage for all touched packages (existing justfile rule).
4. **BDD suite** (orchestrator and worker_node) covers the new inference-in-sandbox path and remains runnable via `just test-bdd`.
5. **CLI app** exists as a separate Go module, runnable against the orchestrator (user-gateway) on localhost, with basic auth and task operations.

Assumptions: Phase 1 is substantially complete (node registration, config delivery, per-node dispatch, sandbox run with `--network=none`, user-gateway auth and task APIs).

Remaining Phase 1 spec gaps (e.g. config_version ULID, Worker API `GET /readyz`) can be closed in parallel or as first steps of this plan.

## 2 Scope Summary

| Area | Post-Phase 1 MVP scope |
|------|------------------------|
| Inference in sandbox | Inference proxy sidecar; pod/network so sandbox can call `http://localhost:11434`; `OLLAMA_BASE_URL` in sandbox env; optional E2E scenario that runs a task invoking inference from inside the sandbox. |
| Feature files | E2E inference-in-sandbox scenario; optional worker_node scenarios (413, truncation); orchestrator fail-fast scenario clarified or scoped to node. |
| Unit tests | 90%+ coverage for orchestrator, worker_node, and new CLI module; no new exceptions in justfile. |
| BDD | Steps and scenarios for inference-ready node and sandbox job using inference; CLI has BDD (see 2.1). |
| CLI | Separate Go module; `version`, `status`, `auth login` / `logout` / `whoami`; create task and get result against user-gateway; config via env and optional file. |

### 2.1 Current Status (2026-02-20 0637)

- **CLI (cynork):** Done.
  Module at `cynork/`, in `go.work` and justfile `go_modules`.
  Commands: `version`, `status`, `auth login` / `logout` / `whoami`, `task create`, `task result`.
  Gateway client, config (env + optional `~/.config/cynork/config.yaml`), unit tests, and BDD suite (`cynork/_bdd`, `features/cynork/cynork_cli.feature`) in place.
  `just test-bdd` runs orchestrator, worker_node, and cynork BDD.
  See [post_phase1_cli_implementation_report.md](post_phase1_cli_implementation_report.md) and [cynork_cli_localhost.md](cynork_cli_localhost.md).
- **Inference in sandbox:** Not started.
  No inference proxy sidecar, pod/network, or `OLLAMA_BASE_URL` in sandbox; sandboxes still run with `--network=none`.
  E2E and worker_node feature files do not yet include inference-in-sandbox scenarios.
- **Phase 1 gaps:** Unchanged: `config_version` ULID, Worker API `GET /readyz`, orchestrator fail-fast scenario scope (see code review).
  Optional 413/truncation worker_node scenarios not added.

## 3 Inference in Sandboxed Containers

Phase 1 runs sandboxes with `--network=none`, so jobs cannot reach Ollama.

To leverage full single-node capabilities (orchestrator dispatches work that uses inference inside the sandbox), implement the following.

### 3.1 Inference Proxy Sidecar (Per `node.md` Option A)

- For each job that may use inference, the node runs a pod (or equivalent isolated network) containing:
  - The sandbox container.
  - A lightweight inference proxy sidecar that listens on `localhost:11434` inside the pod and forwards to the node's Ollama container.
- Sandbox container receives `OLLAMA_BASE_URL=http://localhost:11434` in its environment.
- Proxy enforces request size (e.g. 10 MiB) and per-request timeout (e.g. 120s) and MUST NOT expose credentials.

### 3.2 Implementation Ownership

- **Worker node:** Node Manager or Worker API creates the pod/network and starts sandbox + proxy sidecar when a job requests or is configured for inference; inject `OLLAMA_BASE_URL` into sandbox env.
- **Orchestrator:** No change required for basic flow; dispatch remains HTTP to Worker API.
  Optional: job or task hint that inference is needed so the node can choose pod+proxy vs plain container.

### 3.3 E2E Scenario

After proxy is implemented, add a scenario: create task whose command performs a simple inference call (e.g. curl to localhost:11434 or use a small script); assert job completes and result indicates success.

Script-driven E2E (`just e2e` / `setup-dev.sh full-demo`) can run this when the node is started with inference and a model is loaded.
See [single_node_e2e_testing_plan.md](single_node_e2e_testing_plan.md).

## 4 Feature Files

- **`features/cynork/cynork_cli.feature`:** Exists.
  Covers status, auth login/whoami, whoami without token, create task and get result.
  Suite tag `@suite_cynork`; see [features/README.md](../features/README.md) suite registry.

- **`features/e2e/single_node_happy_path.feature`:** Add a scenario (e.g. "Single-node task execution with inference in sandbox") that assumes an inference-capable node with proxy and model loaded.
  Steps: login, node registered and config ack, create task that runs inference inside the sandbox, dispatch, job completes with success.
  Tag so BDD/script can select it when inference path is available.

- **`features/worker_node/worker_node_sandbox_execution.feature`:** Optionally add scenarios for request size limit (413) and stdout/stderr truncation (per [mvp_phase1_code_review_report.md](mvp_phase1_code_review_report.md) Section 3).

- **`features/orchestrator/orchestrator_startup.feature`:** Clarify or rescope "Orchestrator fails fast when no inference path is available" per code review.
  Either document as node-side only and mark scenario as such, or implement an orchestrator readiness check and keep the scenario.

- **New (optional):** `features/e2e/single_node_inference_in_sandbox.feature` if the team prefers a dedicated feature file for inference-in-sandbox rather than extending `single_node_happy_path.feature`.

## 5 Unit Tests and Coverage

- **Target:** All Go modules (orchestrator, worker_node, and the new CLI module) maintain at least 90% package-level coverage under `just test-go-cover`; control-plane may keep the existing 89% exception if still justified.
- **New code:** Any new packages (inference proxy client, pod/network helpers, CLI gateway client, CLI commands) must have unit tests from the start; no broad exclusions.
- **Practice:** Use table-driven tests and existing patterns in `orchestrator/internal/*` and `worker_node`; mock external HTTP and container runtime where appropriate.
- **CI:** No change to justfile; `just ci` already runs `test-go-cover` and fails if any package is below the threshold.

## 6 BDD Suite

- **Orchestrator (`orchestrator/_bdd`):** Add step definitions and scenarios that depend on "inference-ready" only when the new E2E/inference scenarios are added; keep existing DB-backed scenarios running with testcontainers when `POSTGRES_TEST_DSN` is unset.
- **Worker node (`worker_node/_bdd`):** Add steps for "job runs in pod with inference proxy" and "sandbox env contains OLLAMA_BASE_URL"; use mock orchestrator or a real Worker API with a test runtime (e.g. podman) when needed.
  Keep scenarios that do not require inference runnable without a real Ollama.
- **E2E:** The script-driven E2E (`just e2e`) remains the primary way to run the full single-node + inference path.
  BDD can implement the same flow in Godog for traceability (see `features/e2e/` and optional `_bdd` for e2e if added).
- **CLI (cynork):** BDD in place.
  `just test-bdd` runs `./orchestrator/_bdd ./worker_node/_bdd ./cynork/_bdd`.
  Feature file: `features/cynork/cynork_cli.feature`; steps use a mock gateway.
  Unit tests and manual runs against localhost user-gateway documented in [cynork_cli_localhost.md](cynork_cli_localhost.md).

## 7 CLI App (Separate Go Module)

- **Status:** Implemented.
  Module at `cynork/`; in `go.work` and justfile `go_modules`; included in `just ci` (fmt, lint, test-go-cover, vulncheck-go, test-bdd).

- **Tech:** Go + Cobra per `docs/tech_specs/cli_management_app.md`.
  Structure: `cmd/` (root, version, status, auth, task), `internal/gateway/`, `internal/config/`.
- **Base URL and auth:** Gateway URL from env `CYNORK_GATEWAY_URL` (default `http://localhost:8080`); token from env `CYNORK_TOKEN` or from `auth login` (stored in optional `~/.config/cynork/config.yaml`).
  No direct DB access; all operations via User API Gateway.
- **Commands:** `cynork version`, `cynork status`, `cynork auth login` / `logout` / `whoami`, `cynork task create --prompt "..."`, `cynork task result <task-id>`.
- **Testing:** Unit tests for gateway client (mocked HTTP), config, and commands; BDD suite in `cynork/_bdd` with `features/cynork/cynork_cli.feature`.
  Target 90%+ coverage per justfile rule.
- **Docs:** `dev_docs/post_phase1_cli_implementation_report.md`, `dev_docs/cynork_cli_localhost.md`.

## 8 Implementation Order

Suggested order (can be parallelized where independent):

1. **Phase 1 gap closure (optional first):** config_version ULID in orchestrator; Worker API `GET /readyz`; clarify orchestrator fail-fast scenario.
2. ~~**CLI module bootstrap**~~ **Done.** cynork module exists; in go.work and go_modules; version, status, auth, task create/result; unit tests and BDD; documented.
3. **Inference proxy and pod/network:** Design and implement the inference proxy (minimal HTTP forwarder to Ollama with size/timeout limits); implement Worker API / Node Manager path that runs jobs in a pod with sandbox + proxy and sets `OLLAMA_BASE_URL` in sandbox env; unit tests for proxy and execution path.
4. **Feature files and BDD:** Add or update feature files (E2E inference-in-sandbox scenario, optional 413/truncation, orchestrator fail-fast wording); implement BDD steps for inference-in-sandbox; ensure `just test-bdd` passes.
5. **E2E script:** Extend `scripts/setup-dev.sh` / `just e2e` to run the inference-in-sandbox scenario when the node and model are available (align with `single_node_e2e_testing_plan.md`).
6. ~~**Coverage and CI**~~ **Done.** cynork is in `go_modules`; `just ci` runs fmt, lint, test-go-cover, vulncheck-go, test-bdd for all modules including cynork.

## 9 References

- `dev_docs/PHASE1_STATUS.md` - Phase 1 implementation summary and running locally.
- `dev_docs/mvp_phase1_code_review_report.md` - Spec gaps and feature/BDD recommendations.
- `dev_docs/single_node_e2e_testing_plan.md` - E2E flow and inference readiness.
- `dev_docs/post_phase1_cli_implementation_report.md` - CLI (cynork) delivery summary.
- `dev_docs/cynork_cli_localhost.md` - Building and running cynork against localhost.
- `docs/tech_specs/_main.md` - Phase 1 and Phase 2 scope.
- `docs/tech_specs/node.md` - Node-local inference, Option A (proxy sidecar).
- `docs/tech_specs/sandbox_container.md` - Node-local inference access.
- `docs/tech_specs/cli_management_app.md` - CLI goals, commands, MVP scope, Go + Cobra.
- `docs/tech_specs/user_api_gateway.md` - Auth and task endpoints for CLI.
- `justfile` - `ci`, `test-go-cover`, `test-bdd`, `e2e`, `go_modules` (includes cynork).
- `features/` - Orchestrator, worker_node, cynork, and e2e feature files.

Report generated 2026-02-20.
Do not update tech specs without explicit direction.
