# E2E Tests vs Implementation: Gap Analysis and Remediation Plan

- [1. Summary](#1-summary)
- [2. Current E2E Assets](#2-current-e2e-assets)
  - [2.1 Script-Driven E2E (`run_e2e_test` in `scripts/setup-dev.sh`)](#21-script-driven-e2e-run_e2e_test-in-scriptssetup-devsh)
  - [2.2 Gherkin E2E Features (Not Run by CI/BDD)](#22-gherkin-e2e-features-not-run-by-cibdd)
  - [2.3 BDD Suites That Run (`just test-bdd`)](#23-bdd-suites-that-run-just-test-bdd)
- [3. Implementation Feature Set (MVP Phases 1, 1.5, 1.7)](#3-implementation-feature-set-mvp-phases-1-15-17)
- [4. Gap Analysis](#4-gap-analysis)
  - [4.1 E2E Gherkin Not Executed](#41-e2e-gherkin-not-executed)
  - [4.2 Script E2E: Missing Assertions vs Implemented Behavior](#42-script-e2e-missing-assertions-vs-implemented-behavior)
  - [4.3 Chat, Task Building, and Multi-Message Positioning](#43-chat-task-building-and-multi-message-positioning)
  - [4.4 Script E2E: Flow Order vs Gherkin](#44-script-e2e-flow-order-vs-gherkin)
  - [4.5 Traceability: E2E Features vs Script](#45-traceability-e2e-features-vs-script)
- [5. Remediation Plan](#5-remediation-plan)
  - [5.1 Option A: Add Godog E2E Suite (Heavy)](#51-option-a-add-godog-e2e-suite-heavy)
  - [5.2 Option B: Extend Script and Keep Gherkin as Doc (Recommended for MVP)](#52-option-b-extend-script-and-keep-gherkin-as-doc-recommended-for-mvp)
  - [5.3 Option C: Align Script Order With Gherkin (Optional)](#53-option-c-align-script-order-with-gherkin-optional)
  - [5.4 Order of Work](#54-order-of-work)
- [6. Reference](#6-reference)

## 1. Summary

```text
Date: 2026-02-27.
Type: Gap analysis and remediation plan.
Scope: Current E2E test assets (script-driven and Gherkin) vs MVP implementation feature set.
```

- **E2E validation today** is primarily **script-driven** (`just e2e` -> `scripts/setup-dev.sh full-demo` -> `run_e2e_test`).
  The script runs nine test blocks (auth, tasks, inference-in-sandbox, prompt-mode, OpenAI models/chat, node registration, capability, refresh, logout).
- **Gherkin E2E features** (`features/e2e/single_node_happy_path.feature`, `features/e2e/chat_openai_compatible.feature`) are **not executed** by any Godog suite; `just test-bdd` runs only `orchestrator`, `worker_node`, and `cynork` `_bdd` suites.
  E2E feature files act as living documentation and traceability only.
- **Gaps:** (1) E2E Gherkin has no runner; (2) script does not assert several spec-critical behaviors (readyz 503 reason, `config_version` ULID, node config request/apply/ack flow, Worker 413/truncation, PMA readiness gating).
  (3) Chat-thread/task separation (REQ-USRGWY-0130) and multi-message task-building behavior have no automated coverage; specs do not yet state that task building may take multiple messages.
  (4) Script test order differs from the single-node happy-path flow (node-then-task vs task-then-node).
  See sections 4.2 and 4.3 for chat/task and spec-update positioning.

This document lists gaps, maps them to requirements/specs, and proposes a remediation plan.
No code changes are prescribed here; the plan is documentation-only.

## 2. Current E2E Assets

This section describes the script-driven E2E flow and the Gherkin E2E feature files.

### 2.1 Script-Driven E2E (`run_e2e_test` in `scripts/setup-dev.sh`)

- **Test:** 1
  - what it does: Login as admin (cynork-dev)
- **Test:** 2
  - what it does: Get current user (whoami)
- **Test:** 3
  - what it does: Create task with "echo Hello from sandbox"
- **Test:** 4
  - what it does: Get task details
- **Test:** 5
  - what it does: Get task result
- **Test:** 5b
  - what it does: Create task with inference in sandbox, assert stdout contains `http://localhost:11434` (conditional on `INFERENCE_PROXY_IMAGE`)
- **Test:** 5c
  - what it does: Create task with natural-language prompt, assert non-empty model output
- **Test:** 5d
  - what it does: `cynork models list` (GET /v1/models); one-shot `cynork chat` (POST /v1/chat/completions)
- **Test:** 6
  - what it does: Node registration (control-plane POST /v1/nodes/register)
- **Test:** 7
  - what it does: Report capability (control-plane POST /v1/nodes/capability with node JWT)
- **Test:** 8
  - what it does: Token refresh (cynork-dev)
- **Test:** 9
  - what it does: Logout (cynork-dev)

Invocation: `just e2e` (or `./scripts/setup-dev.sh full-demo`).
Requires orchestrator stack and (for 5b/5c/5d) inference-ready node and model.

### 2.2 Gherkin E2E Features (Not Run by CI/BDD)

- **`features/e2e/single_node_happy_path.feature`**
  - Scenario: End-to-end single-node task execution (happy path) - login, node registers with PSK, node requests config, node applies config and sends ack, create task "echo hello", dispatch, sandbox execution, result stdout "hello", task status "completed".
    Tags: `@req_identy_0104`, `@req_orches_0112`, `@req_orches_0122`, and spec tags.
  - Scenario: Single-node task execution with inference in sandbox (`@inference_in_sandbox`) - same flow with command `sh -c 'echo $OLLAMA_BASE_URL'`, pod with inference proxy, stdout "<http://localhost:11434>".
- **`features/e2e/chat_openai_compatible.feature`**
  - Scenario: GET /v1/models returns 200 and list-models payload.
  - Scenario: POST /v1/chat/completions returns 200 and completion at `choices[0].message.content`.
  - Scenario: Chat does not imply one task per message (no assertion that a user-visible task was created).

No Godog suite is wired for `@suite_e2e`; see `justfile` and `docs/mvp_plan.md` (Feature Files and BDD).

### 2.3 BDD Suites That Run (`just test-bdd`)

- **orchestrator** `_bdd`: Uses test server + optional Postgres (POSTGRES_TEST_DSN / testcontainers).
  Covers auth, task lifecycle (including prompt vs commands), node registration/config, readyz when no inference path, Chat scenario with mock inference.
- **worker_node** `_bdd`: Worker API bearer auth, sandbox run (echo, env, network_policy), inference-in-sandbox (OLLAMA_BASE_URL), GET /readyz, 413 on oversized body.
- **cynork** `_bdd`: CLI status, auth, tasks, chat.

These cover many of the same *behaviors* as the E2E feature files but in isolation (mocked or single-component).

## 3. Implementation Feature Set (MVP Phases 1, 1.5, 1.7)

From `docs/mvp_plan.md` and tech specs, the following are implemented and should be covered by E2E or BDD:

- **Phase 1:** Node registration, config delivery with ULID `config_version`, per-node dispatch, sandbox run, user-gateway auth and task APIs; orchestrator `GET /readyz` returns 503 when no dispatchable nodes; Worker API `GET /readyz`, 413 for oversized body, stdout/stderr truncation (UTF-8-safe, 256 KiB).
- **Phase 1.5:** `input_mode` (prompt/script/commands), default prompt-as-model path, inference proxy sidecar, BDD for prompt interpretation and commands mode.
- **Phase 1.7:** cynode-pma in orchestrator stack; `GET /readyz` 503 until PMA reachable when enabled; user-gateway `GET /v1/models`, `POST /v1/chat/completions` (routing to PMA vs direct inference).

Relevant requirements/specs (examples): REQ-ORCHES-0120 (readyz), REQ-BOOTST-0002, REQ-ORCHES-0129 (inference path), REQ-WORKER-0140/0142 (Worker readyz), REQ-WORKER-0145/0146/0147 (413, truncation), REQ-USRGWY-0127/0130 (OpenAI chat), worker_node_payloads (config_version ULID), orchestrator_bootstrap, openai_compatible_chat_api.

## 4. Gap Analysis

The following gaps exist between E2E test assets and the implemented feature set.

### 4.1 E2E Gherkin Not Executed

- **Gap:** No E2E Godog suite
  - detail: `features/e2e/*.feature` are never run by `just test-bdd` or `just ci`.
  - Traceability tags are present but scenarios are not automated.

**Impact:** E2E feature files can drift from implementation; no automated regression for the full single-node and OpenAI chat flows described in Gherkin.

### 4.2 Script E2E: Missing Assertions vs Implemented Behavior

- **Implemented behavior:** Orchestrator `GET /readyz` 503 with reason when no inference path
  - script coverage: Not asserted.
  - BDD covers this in orchestrator suite with mock.
- **Implemented behavior:** Orchestrator `GET /readyz` 503 until PMA reachable when enabled
  - script coverage: Not asserted in script.
- **Implemented behavior:** Node config payload contains `config_version` (ULID)
  - script coverage: Script does not drive node config request/apply/ack flow; no check of `config_version`.
- **Implemented behavior:** Node flow: register -> request config -> apply config -> send config ack
  - script coverage: Script runs registration and capability report only; no config fetch or ack step.
- **Implemented behavior:** Worker API returns 413 for oversized request body
  - script coverage: Not in script.
  - Covered by worker_node BDD.
- **Implemented behavior:** Worker API stdout/stderr truncation and `truncated.stdout` / `truncated.stderr` flags
  - script coverage: Not asserted in script.
- **Implemented behavior:** Chat threads and messages stored separately from task lifecycle (REQ-USRGWY-0130)
  - script coverage: No script or BDD assertion for chat-thread/task separation or for multi-message task-building behavior.

### 4.3 Chat, Task Building, and Multi-Message Positioning

REQ-USRGWY-0130 requires that the system store chat history as **chat threads and chat messages tracked separately from task lifecycle state**; it does not require or forbid "one task per message."

**Positioning (for spec updates):** Building up a task properly may take **multiple messages** for clarification and to properly lay out the task.
Specs (e.g. chat_threads_and_messages, openai_compatible_chat_api, or related orchestration/PM docs) should be updated to state this explicitly: multi-message conversation is the intended way to clarify and lay out a task before or as it is executed.
This clarification belongs in the normative docs; for now it is recorded here in dev_docs.

### 4.4 Script E2E: Flow Order vs Gherkin

- **Gherkin (single_node_happy_path):** Background (DB, orchestrator, admin, worker running) -> login -> **node registers -> node requests config -> node applies config and ack** -> create task -> dispatch -> execute -> result.
- **Script:** login -> whoami -> create task -> get task -> result -> (5b inference, 5c prompt, 5d models+chat) -> **node register -> capability**.

So the script validates "task flow with an already-running node" and then "control-plane node registration/capability" separately.
It does not validate the Gherkin ordering (node config flow before first task).
That may be intentional (user flow first, then admin/control-plane checks) but is a documented difference.

### 4.5 Traceability: E2E Features vs Script

- E2E feature scenarios carry `@req_*` and `@spec_*` tags.
  Script steps have no direct requirement/spec tags; coverage is implicit.
- Mapping script steps to requirements/specs would improve traceability.
- It would make it obvious which reqs are only covered by BDD vs script.

## 5. Remediation Plan

Three options are outlined below; Option B is recommended for MVP.

### 5.1 Option A: Add Godog E2E Suite (Heavy)

- Introduce an E2E Godog suite (e.g. `e2e/_bdd`) that runs `features/e2e/*.feature`.
- Implement step definitions that call real services (user-gateway, control-plane, worker) or a thin test harness.
- Requires: either full stack in CI (e.g. compose up + run Godog) or a dedicated "e2e" test mode with stable ports and fixtures.
- **Pros:** E2E Gherkin becomes executable; single source of truth for acceptance.
- **Cons:** Slower CI, more infra, flakiness risk; duplicate coverage with script unless script is retired or reduced to "smoke only."

**Decision:** Deferred; options B and C will be exercised.

### 5.2 Option B: Extend Script and Keep Gherkin as Doc (Recommended for MVP)

- **Script additions (no new runner):**
  - **Readyz 503:** Before or after existing tests, call `GET /readyz` in a state where no node is registered (or PMA disabled) and assert 503 and response body contains a reason string (e.g. "inference" or "no inference path").
    Document requirement/spec in script comment.
  - **PMA readiness (optional):** If compose starts PMA, add a step that asserts 503 until PMA is up, then 200 (or run existing tests only after readyz 200).
  - **Worker 413:** From host, POST an oversized body to the worker API and assert 413.
    Requires worker URL and token; may be conditional on E2E env.
  - **Truncation (optional):** Create a task that produces >256 KiB stdout and assert result has `truncated.stdout` (or equivalent) set.
    Heavy for script; could remain BDD-only.
  - **Chat vs task (REQ-USRGWY-0130):** If desired, assert or document that chat is stored as threads/messages separate from task lifecycle; any script check would reflect product behavior (e.g. task count before/after chat), not a "no task per message" requirement.
  - **Spec updates:** Add to relevant specs that task building may take multiple messages for clarification and to lay out the task (see section 4.3).
- **Gherkin:** Keep E2E feature files as living documentation; optionally add a short "E2E coverage" section in `features/README.md` or `docs/mvp_plan.md` stating that script-driven `just e2e` is the primary E2E automation and which scenarios are covered by script vs BDD only.
- **Traceability:** Add a small dev_docs or in-repo table mapping script test numbers to requirement IDs (e.g. Test 5d -> REQ-USRGWY-0127, REQ-USRGWY-0130).

**Decision:** Accepted.

### 5.3 Option C: Align Script Order With Gherkin (Optional)

- Restructure `run_e2e_test` so that node registration and (if feasible) config request/apply/ack run before the first task create.
  That would align script with the single-node happy path scenario order.
  It would catch regressions where "task before node config" behaves differently.
- Config request/apply/ack would require either a small helper that calls control-plane (and possibly worker) to fetch config and post ack, or a documented decision that "config flow" is covered only by BDD/orchestrator suite.

**Decision:** Accepted.

### 5.4 Order of Work

1. **Document:** Update `features/README.md` or `docs/mvp_plan.md` to state that E2E Gherkin is not run by Godog; script `just e2e` is the primary E2E validation; and list which e2e scenarios are covered by script vs BDD only.
2. **Script:** Add assertions for readyz 503 (with reason) and, if desired, for chat-thread/task separation behavior (REQ-USRGWY-0130).
   Add comments with requirement/spec IDs for key steps in the script.
3. Add script checks for Worker 413 and, if needed, PMA readiness; then consider config_version/flow (Option C) if product demands full node-config E2E coverage.

## 6. Reference

- E2E script: `scripts/setup-dev.sh` (`run_e2e_test`).
- E2E features: `features/e2e/single_node_happy_path.feature`, `features/e2e/chat_openai_compatible.feature`.
- BDD: `just test-bdd` -> `orchestrator/_bdd`, `worker_node/_bdd`, `cynork/_bdd`.
- MVP scope and status: `docs/mvp_plan.md` (Current Status, Feature Files and BDD, Implementation Order).
- Ports and E2E/BDD: `docs/tech_specs/ports_and_endpoints.md`.
