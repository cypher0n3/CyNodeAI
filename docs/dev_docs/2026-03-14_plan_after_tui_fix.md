# Plan After Cynork TUI Fix (Follow-On)

- [Goal](#goal)
- [Prerequisite](#prerequisite)
- [References](#references)
- [Constraints](#constraints)
- [Execution Plan](#execution-plan)
- [Execution Order](#execution-order)
- [Exit Criteria](#exit-criteria)

## Goal

This plan runs **after** `2026-03-14_cynork_tui_fix_plan.md` (doc removed) is complete.
It addresses the outstanding work from `2026-03-12_plan_next_round_execution.md` (doc removed) that remains once the TUI fix plan has closed BDD failures, coverage gaps, undefined steps, missing slash commands, spec-compliance gaps, project stubs, and lint suppressions.
Outcome: end-to-end interactive streaming, minimum MCP-in-the-loop for PMA chat, TUI auth recovery and UX completion, full BDD and PTY validation, then worker docs and remaining MVP Phase 2 work.

## Prerequisite

- [ ] `2026-03-14_cynork_tui_fix_plan.md` (doc removed) is complete: `just ci` passes, 0 BDD failures, all packages at or above 90% coverage, only one allowed `//nolint` remains.
- [ ] Do not start this plan until the prerequisite is verified.

## References

- `2026-03-12_plan_next_round_execution.md` (doc removed; source of outstanding work; Phases 3-8 unchecked items).
- [docs/requirements/client.md](../requirements/client.md), [docs/requirements/usrgwy.md](../requirements/usrgwy.md), [docs/requirements/pmagnt.md](../requirements/pmagnt.md), [docs/requirements/orches.md](../requirements/orches.md).
- [docs/tech_specs/openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md) (streaming), [docs/tech_specs/chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md) (thinking persistence), [docs/tech_specs/cynode_pma.md](../tech_specs/cynode_pma.md) (streaming, MCP), [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md), [docs/tech_specs/mcp_gateway_enforcement.md](../tech_specs/mcp_gateway_enforcement.md).
- Implementation: gateway, orchestrator handlers, cynork/internal/chat (transport), cynork/internal/tui, agents/internal/pma, scripts/test_scripts (PTY harness).

## Constraints

- Requirements and tech specs are source of truth; implementation must align.
- Do not start the next task until the current task's Closeout is done and its validation gate has passed.
- Use BDD/TDD: add or update specs and failing tests before implementation; implement smallest change to pass; refactor only after green.
- Treat `just ci` and `just e2e` as mandatory end-of-task gates where applicable.
- Do not implement summary generation, archive, or soft-delete in this plan (deferred per 2026-03-12).

**E2E tests and dev stack:** Add or update E2E tests in `scripts/test_scripts/` to achieve full coverage of the application stack; each task that touches E2E-covered behavior should add or update the related tests in the same task.
Before running E2E: run `just setup-dev restart --force` (or `just setup-dev start --force` if the stack is not running) so the stack is fully rebuilt; then run the relevant E2E tests.
Per-task validation: run only the **relevant** E2E tests (via tags); fix any failing tests before proceeding.
Final task (Task 7 closeout): run the **full** E2E suite (`just e2e`) with all tests passing and only expected skips.

### E2E Test Inventory and What to Test

Scripts live in `scripts/test_scripts/`.

- **Task 1 Streaming** - Scripts: **e2e_0610_sse_streaming.py**, **e2e_0750_tui_pty.py**.
  Assert: e2e_127: `stream=true` on both `/v1/chat/completions` and `/v1/responses`; SSE events and `[DONE]`; no `<think>` in visible content; client disconnect or Ctrl+C causes gateway to treat request as canceled (REQ-USRGWY-0150).
  e2e_198: progressive visible-text updates in TUI; Ctrl+C cancels stream, in-flight landmark then prompt-ready; final turn reconciled in scrollback.
  Run: `just e2e --tags chat`, `--tags pma_inference`, `--tags tui_pty`.
- **Task 2 MCP** - Scripts: **e2e_0660_worker_pma_proxy.py**, **e2e_0570_pma_chat_context.py**, **e2e_0580_pma_chat_capable_model.py** or new.
  Assert: PMA chat uses real MCP tool results (db.task.get, db.project.get/list, etc.); tool success, rejection, and ambiguity surface as real outcomes (no simulated content); gateway allow-list permits the minimum tool set.
  Run: `just e2e --tags pma`, `--tags chat`.
- **Task 3 Auth and PTY** - Scripts: **e2e_0030_auth_login.py**, **e2e_0750_tui_pty.py**, **e2e_0760_tui_slash_commands.py** or new.
  Assert: Startup login recovery when token missing; in-session auth recovery when gateway returns 401; password/token never in scrollback; `/thread new`/`switch`/`rename` in PTY; `/show-thinking`/`/hide-thinking` toggle and persist (YAML reload).
  Run: `just e2e --tags auth`, `--tags tui_pty`.
- **Task 4 Phase 6 coverage** - Scripts: **e2e_0750_tui_pty.py**, **e2e_0760_tui_slash_commands.py**, **e2e_0610_sse_streaming.py**, **e2e_0540_chat_reliability.py** through **e2e_0560_chat_simultaneous_messages.py**.
  Assert: All Phase 6 behaviors (auth recovery, both chat surfaces, streaming and cancellation, thinking visibility and persist, collapsed-thinking placeholder); BDD scenarios have matching E2E or PTY assertions.
  Run: `just e2e --tags tui_pty`, `--tags chat`, `--tags auth`.
- **Task 6 Phase 7** - Scripts: per-slice (MCP, LangGraph, chat reliability).
  Assert: E2E for each resumed slice (MCP tools beyond minimum, verification loop, chat/runtime drifts retry/bounded wait); assert real outcomes.
  Run: `just e2e --tags pma` and/or `--tags chat` or per-slice tags.

When adding or updating: assert on landmarks, scrollback content, API response shape, or exit codes; tag tests so they run with the relevant subset and in full_demo.

## Execution Plan

Execute tasks in the order given in [Execution Order](#execution-order).
Each task is self-contained: it has its own Requirements and Specifications, Discovery steps, Red, Green, Refactor, Testing, and **Closeout** (task completion report, then mark completed steps with `- [x]` as the last step).
Do not start a later task until the current task's Closeout is done and its validation gate has passed.

---

### Task 1: End-To-End Streaming (Backend, Transport, TUI, PTY)

Deliver `stream=true` on both interactive chat surfaces, client-driven cancellation, PMA incremental streaming with hidden-thinking separation, shared transport streaming/cancellation support, TUI progressive rendering, and PTY validation for streaming and cancellation.

#### Task 1 Requirements and Specifications

- [REQ-USRGWY-0149](../requirements/usrgwy.md), [REQ-USRGWY-0150](../requirements/usrgwy.md), [REQ-PMAGNT-0118](../requirements/pmagnt.md), [REQ-CLIENT-0209](../requirements/client.md).
- [CYNAI.USRGWY.OpenAIChatApi.Streaming](../tech_specs/openai_compatible_chat_api.md), [CYNAI.PMAGNT.StreamingAssistantOutput](../tech_specs/cynode_pma.md), [CYNAI.CLIENT.CynorkTui.GenerationState](../tech_specs/cynork_tui.md).
- 2026-03-12 plan (doc removed) Phase 3 "Required Backend Validation Before TUI Wiring" and Phase 4 "Transport and Rendering Seams" (streaming and cancellation bullets); Phase 5 "Core TUI Experience" (streaming slice); Phase 6 streaming and cancellation coverage.

#### Discovery (Task 1) Steps

- [ ] Read the requirements and specs listed in Task 1 Requirements and Specifications.
- [ ] Inspect gateway handlers for `stream=true` on `POST /v1/chat/completions` and `POST /v1/responses`; identify gaps.
- [ ] Inspect cynork/internal/chat transport for streaming delta consumption and cancellation propagation.
- [ ] Inspect PMA path for incremental streaming and thinking separation; confirm no `<think>` in visible deltas.
- [ ] Run `just test-bdd` and `just e2e` to establish baseline; identify BDD/E2E scenarios that will cover streaming and cancellation once implemented.

#### Red (Task 1)

- [ ] Add or update BDD/E2E scenarios for: gateway `stream=true` on both surfaces; client disconnect or Ctrl+C causes gateway to treat request as canceled; TUI progressive visible-text updates and final reconciliation; degraded fallback when backend cannot stream.
- [ ] Add or update unit/integration tests for gateway streaming handlers, transport streaming path, and controller cancellation.
- [ ] Run the new or updated tests and confirm they fail for the right reason before implementation.
- [ ] Validation gate: do not proceed until failing tests prove the gap.

#### Green (Task 1)

- [ ] Verify retained `thinking` structured data survives persistence and thread-history retrieval so the TUI can reveal prior-turn thinking while scrolling back without leaking it into canonical plain-text content (backend validation).
- [ ] Implement and verify `stream=true` on both interactive chat surfaces so the gateway delivers ordered incremental events suitable for progressive TUI rendering.
- [ ] Implement and verify client-driven stream cancellation (disconnect or Ctrl+C): canceled work, upstream generation stopped or detached best-effort, request-scoped resources released.
- [ ] Implement and verify PMA incremental streaming upstream with hidden-thinking separation and no literal `<think>` leakage in visible deltas.
- [ ] Extend shared chat transport abstraction to consume streaming deltas, progress events, terminal completion/error signals, and explicit cancellation for both surfaces.
- [ ] Add controller-level cancellation propagation so Ctrl+C stops the active stream, reconciles the in-flight turn deterministically, and keeps the session alive unless the user explicitly exits.
- [ ] Implement default interactive streaming on the TUI path with degraded fallback when the selected backend path cannot stream visible-text deltas.
- [ ] Implement in-flight generation handling so one assistant turn is updated progressively and reconciled cleanly on completion.
- [ ] Add or update Python PTY validation for interactive streaming: progressive visible-text updates, cancellation via Ctrl+C, clean in-place reconciliation of the final assistant turn.
- [ ] Run targeted tests and BDD/E2E until they pass.
- [ ] Validation gate: do not proceed until targeted tests are green.

#### Refactor (Task 1)

- [ ] Refine implementation without changing behavior; keep all tests green.
- [ ] Re-run targeted test suite and BDD/E2E.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 1)

- [ ] Add or update **e2e_0610_sse_streaming.py** and **e2e_0750_tui_pty.py** per [E2E Test Inventory](#e2e-test-inventory-and-what-to-test): assert `stream=true` on both surfaces, SSE and `[DONE]`, no `<think>` in content, client cancel (REQ-USRGWY-0150); assert TUI progressive updates, Ctrl+C cancel, in-flight landmark, reconciled turn.
- [ ] Run `just setup-dev restart --force` (or `just setup-dev start --force` if the stack is not running); then run `just e2e --tags chat` and `just e2e --tags tui_pty`; fix any failing E2E tests.
- [ ] Run `just ci` and `just e2e` for the streaming and cancellation scope.
- [ ] Confirm gateway, transport, TUI, and PTY behavior match the specs listed in Task 1.
- [ ] Validation gate: do not start Task 2 until all Task 1 checks pass.

#### Closeout (Task 1)

- [ ] Generate a **task completion report** for Task 1: what was done, what passed, any deviations or notes for follow-up.
- [ ] Do not start Task 2 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 2: Minimum MCP-In-The-Loop Slice for PMA Chat

Pull forward the minimum MCP gateway allow-path and PMA chat tool set so PMA chat and tool-aware thinking models use real MCP tools instead of tool-less fallback.

#### Task 2 Requirements and Specifications

- [REQ-PMAGNT-0106](../requirements/pmagnt.md), [REQ-PMAGNT-0107](../requirements/pmagnt.md).
- [CYNAI.MCPGAT.PmAgentAllowlist](../tech_specs/mcp_gateway_enforcement.md), [CYNAI.PMAGNT.McpToolAccess](../tech_specs/cynode_pma.md), [CYNAI.AGENTS.PMLlmToolImplementation](../tech_specs/project_manager_agent.md).
- 2026-03-12 plan (doc removed) Phase 3 "Minimum MCP Slice for PMA Thinking Models".

#### Discovery (Task 2) Steps

- [ ] Read the requirements and specs listed in Task 2 Requirements and Specifications.
- [ ] Inspect current MCP gateway allow path (e.g. `db.preference.*` only); identify expansion points.
- [ ] Inspect PMA chat execution path and langchaingo tool use; identify where worker proxy and orchestrator MCP gateway must be wired.
- [ ] List the minimum PMA chat tool set from the stable catalog (e.g. db.task.get, db.project.get/list, db.system_setting.get/list, artifact.*, help.get) and confirm gateway allow-list and tool registration.

#### Red (Task 2)

- [ ] Add or update backend and PMA-facing tests that prove MCP tool success, rejection, and ambiguity are surfaced as real tool outcomes (no guessed or simulated content).
- [ ] Run the new or updated tests and confirm they fail for the right reason before implementation.
- [ ] Validation gate: do not proceed until failing tests prove the gap.

#### Green (Task 2)

- [ ] Expand the MCP allow path beyond `db.preference.*` for the minimum PMA-safe tool slice aligned with the specs.
- [ ] Land the minimum PMA chat tool set (smallest viable subset: db.task.get, db.project.get/list, db.system_setting.get/list, artifact.* where needed, help.get).
- [ ] Wire PMA chat execution so langchaingo tool use goes through the worker proxy and orchestrator MCP gateway on the active chat path.
- [ ] Run targeted tests until they pass.
- [ ] Validation gate: do not proceed until targeted tests are green.

#### Refactor (Task 2)

- [ ] Refine implementation without changing behavior; keep all tests green.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 2)

- [ ] Add or update **e2e_0660_worker_pma_proxy.py**, **e2e_0570_pma_chat_context.py**, **e2e_0580_pma_chat_capable_model.py** (or new script) per [E2E Test Inventory](#e2e-test-inventory-and-what-to-test): assert PMA chat uses real MCP tool results (db.task.get, db.project.get/list, etc.), tool success/rejection/ambiguity are real outcomes (no simulated content).
- [ ] Run `just setup-dev restart --force` (or `just setup-dev start --force` if the stack is not running); then run `just e2e --tags pma` and `just e2e --tags chat`; fix any failing E2E tests.
- [ ] Run `just ci` and any targeted E2E for PMA chat with MCP tools.
- [ ] Confirm MCP tool success, rejection, and ambiguity are real outcomes per [REQ-AGENTS-0137](../requirements/agents.md) and [CYNAI.AGENTS.NoSimulatedOutput](../tech_specs/project_manager_agent.md).
- [ ] Validation gate: do not start Task 3 until all Task 2 checks pass.

#### Closeout (Task 2)

- [ ] Generate a **task completion report** for Task 2: what was done, what passed, any deviations or notes for follow-up.
- [ ] Do not start Task 3 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 3: TUI Auth Recovery, In-Session Switches, and PTY Validation

Implement startup and in-session auth recovery, project and model in-session switching, and validate thread create/switch/rename, thinking visibility, and auth recovery through the Python PTY harness.
Optionally align interactive `cynork chat` with the fullscreen TUI entry flow while keeping `cynork chat --message` and non-interactive usage line-oriented.

#### Task 3 Requirements and Specifications

- [docs/requirements/client.md](../requirements/client.md) (auth recovery, in-session model/project).
- [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md) (auth recovery, status bar, in-session switches).
- 2026-03-12 plan (doc removed) Phase 5 "Auth Recovery", "Thread and Session UX" (project/model in-session), "Tandem TUI and Harness Validation", "TUI Chat-Complete Exit".

#### Discovery (Task 3) Steps

- [ ] Read the requirements and specs listed in Task 3 Requirements and Specifications.
- [ ] Inspect cynork TUI and cmd for login flow, token validation, and gateway auth failure handling.
- [ ] Inspect session and TUI for project and model switching; identify gaps vs spec.
- [ ] Review PTY harness and e2e scripts for thread create/switch/rename, thinking, and auth-recovery assertions; identify missing coverage.

#### Red (Task 3)

- [ ] Add or update BDD scenarios for startup and in-session auth recovery.
- [ ] Add or update PTY or E2E scenarios for auth recovery, thread create/switch/rename, and thinking visibility (including scrollback/history-reload and YAML persist for `/show-thinking`/`/hide-thinking`).
- [ ] Run the new or updated tests and confirm they fail for the right reason before implementation.
- [ ] Validation gate: do not proceed until failing tests prove the gap.

#### Green (Task 3)

- [ ] Implement startup login recovery when a usable token is missing.
- [ ] Implement in-session login recovery when the gateway returns an auth failure.
- [ ] Ensure passwords and tokens are never echoed, persisted in transcript history, or written to temporary UI state unsafely.
- [ ] Implement project-context switching in-session and model selection in-session.
- [ ] Validate thread create, list, switch, and rename through the PTY harness.
- [ ] Validate hidden-thinking, ordered assistant output, and tool-activity rendering through the PTY harness (if not already covered in Task 1).
- [ ] Validate `/show-thinking` and `/hide-thinking` through the PTY harness, including scrollback or history-reload for prior turns and YAML config persistence across sessions.
- [ ] Validate startup and in-session auth recovery through the PTY harness.
- [ ] Optionally: make interactive `cynork chat` invoke the same fullscreen TUI entry flow as `cynork tui`, keeping `cynork chat --message` and non-interactive usage line-oriented and parseable.
- [ ] Run targeted tests and PTY/E2E until they pass.
- [ ] Validation gate: do not proceed until targeted tests are green.

#### Refactor (Task 3)

- [ ] Refine implementation without changing behavior; keep all tests green.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 3)

- [ ] Add or update **e2e_0030_auth_login.py**, **e2e_0750_tui_pty.py**, **e2e_0760_tui_slash_commands.py** (or new) per [E2E Test Inventory](#e2e-test-inventory-and-what-to-test): assert startup login when token missing, in-session auth recovery on 401, password/token never in scrollback; assert `/thread new`/`switch`/`rename` in PTY; assert `/show-thinking`/`/hide-thinking` toggle and YAML persist.
- [ ] Run `just setup-dev restart --force` (or `just setup-dev start --force` if the stack is not running); then run `just e2e --tags auth` and `just e2e --tags tui_pty`; fix any failing E2E tests.
- [ ] Run `just ci` and `just e2e` for the auth, in-session, and PTY scope.
- [ ] Confirm TUI chat-complete exit: user can send, receive, see thread state, project/model context, continue conversation; user can start a fresh thread and continue in the new thread; TUI remains coherent for both chat-completions and responses paths.
- [ ] Validation gate: do not start Task 4 until all Task 3 checks pass.

#### Closeout (Task 3)

- [ ] Generate a **task completion report** for Task 3: what was done, what passed, any deviations or notes for follow-up.
- [ ] Do not start Task 4 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 4: BDD and PTY Coverage (Phase 6 Alignment)

Add or complete BDD and PTY coverage for auth recovery, both chat surfaces, interactive streaming, stream cancellation, thinking visibility (including persistence in YAML and scrollback), and collapsed-thinking placeholder so the full TUI path is machine-checkable.

#### Task 4 Requirements and Specifications

- [docs/requirements/client.md](../requirements/client.md), [docs/requirements/usrgwy.md](../requirements/usrgwy.md).
- [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md).
- 2026-03-12 plan (doc removed) Phase 6 TUI Validation and BDD (unchecked items).

#### Discovery (Task 4) Steps

- [ ] Read the requirements and specs listed in Task 4 Requirements and Specifications.
- [ ] Run `just test-bdd` and `just e2e`; list scenarios that are still missing or pending for auth, both surfaces, streaming, cancellation, thinking visibility, and collapsed-thinking.
- [ ] Review feature files in features/cynork and features/e2e for gaps vs spec.

#### Red (Task 4)

- [ ] Add or update BDD scenarios for: startup and in-session auth recovery; coverage for both `POST /v1/chat/completions` and `POST /v1/responses` in TUI; interactive streaming (progressive updates, final reconciliation, degraded fallback); stream cancellation (client disconnect, Ctrl+C, deterministic reconciliation); `/show-thinking` and `/hide-thinking` (including prior-turn thinking in scrollback and YAML persist); collapsed-thinking placeholder (secondary style, `/show-thinking` hint).
- [ ] Add or update PTY or E2E assertions for the same behaviors where applicable.
- [ ] Run the new or updated tests and confirm any missing step definitions or failing assertions are documented and addressed in the same slice.
- [ ] Validation gate: do not proceed until coverage goals are defined and tests added or updated.

#### Green (Task 4)

- [ ] Implement any missing BDD step definitions and wire scenarios so they pass.
- [ ] Run `just docs-check` after feature-file or validation-doc edits.
- [ ] Run `just test-bdd` and `just e2e` until the Phase 6 scope is covered and passing.
- [ ] Validation gate: do not proceed until BDD and E2E pass for the Phase 6 scope.

#### Refactor (Task 4)

- [ ] Refine step definitions or test helpers without changing behavior; keep all tests green.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 4)

- [ ] Add or update BDD and E2E scripts per [E2E Test Inventory](#e2e-test-inventory-and-what-to-test) (e2e_198, e2e_199, e2e_127, e2e_192-e2e_194): assert all Phase 6 behaviors (auth recovery, both chat surfaces, streaming and cancellation, thinking visibility and persist, collapsed-thinking placeholder).
- [ ] Run `just setup-dev restart --force` (or `just setup-dev start --force` if the stack is not running); then run `just e2e --tags tui_pty`, `--tags chat`, `--tags auth` as relevant; fix any failing E2E tests.
- [ ] Run `just ci` and `just e2e`.
- [ ] Confirm every Phase 6 behavior listed in 2026-03-12 has corresponding BDD or PTY/E2E coverage.
- [ ] Validation gate: do not start Task 5 until all Task 4 checks pass.

#### Closeout (Task 4)

- [ ] Generate a **task completion report** for Task 4: what was done, what passed, any deviations or notes for follow-up.
- [ ] Do not start Task 5 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 5: Worker Deployment Docs (Phase 8 Alignment)

Ensure the promoted worker deployment docs clearly distinguish normative deployment topology decisions from deferred implementation work.

#### Task 5 Requirements and Specifications

- [docs/requirements/worker.md](../requirements/worker.md), [docs/tech_specs/worker_node.md](../tech_specs/worker_node.md).
- 2026-03-12 plan (doc removed) Phase 8 Worker Deployment Simplification Docs.

#### Discovery (Task 5) Steps

- [ ] Read the worker requirements and worker_node tech spec.
- [ ] Identify sections that mix normative topology with deferred implementation; list edits needed.

#### Red (Task 5)

- [ ] No Red phase required for docs-only task; Discovery suffices.

#### Green (Task 5)

- [ ] Update worker deployment docs so normative deployment topology is clearly separated from deferred implementation (e.g. single-binary worker).
- [ ] Run `just docs-check` after edits.
- [ ] Validation gate: do not proceed until docs-check passes.

#### Refactor (Task 5)

- [ ] N/A or minimal wording polish.

#### Testing (Task 5)

- [ ] Run `just docs-check` and any targeted validation for impacted examples or deployment workflows.
- [ ] Validation gate: do not start Task 6 until all Task 5 checks pass.

#### Closeout (Task 5)

- [ ] Generate a **task completion report** for Task 5: what was done, what passed, any deviations or notes for follow-up.
- [ ] Do not start Task 6 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 6: Remaining MVP Phase 2 (Phase 7 Alignment)

Resume non-TUI MVP Phase 2 implementation only after the first usable TUI path is stable: remaining MCP tool slices beyond the minimum, LangGraph graph-node work, verification-loop work for PMA and Project Analyst, and chat/runtime drifts (bounded wait, retry, reliability).

#### Task 6 Requirements and Specifications

- [docs/mvp_plan.md](../mvp_plan.md), [docs/requirements/pmagnt.md](../requirements/pmagnt.md), [docs/requirements/orches.md](../requirements/orches.md).
- 2026-03-12 plan (doc removed) Phase 7 Remaining MVP Phase 2 Work.

#### Discovery (Task 6) Steps

- [ ] Read the MVP implementation plan and Phase 7 scope in 2026-03-12.
- [ ] List remaining MCP tool slices beyond the minimum PMA chat set; list LangGraph graph-node and verification-loop items; list chat/runtime drifts (bounded wait, retry, reliability).
- [ ] Confirm Tasks 1-4 are complete and TUI path is stable before starting.

#### Red (Task 6)

- [ ] For each Phase 7 slice, identify the unit, integration, BDD, and E2E coverage that must change.
- [ ] Add or update tests so new behavior is covered in the same slice; run and confirm failures or gaps as needed.
- [ ] Validation gate: do not proceed until test plan is defined per slice.

#### Green (Task 6)

- [ ] Resume remaining MCP tool slices beyond the minimum PMA chat slice.
- [ ] Finish the remaining LangGraph graph-node work identified in the MVP implementation plan.
- [ ] Finish the verification-loop work needed for PMA to Project Analyst to result review flows.
- [ ] Close known chat/runtime drifts (bounded wait, retry behavior, user-visible reliability gaps).
- [ ] Keep all currently deferred TUI features deferred; record any pull-forward candidates for the next planning cycle.
- [ ] Run targeted validation per slice; run `just ci` and `just e2e` when the phase closes.
- [ ] Validation gate: do not proceed until all Phase 7 slices and gates pass.

#### Refactor (Task 6)

- [ ] Refine implementation without changing behavior; keep all tests green per slice.

#### Testing (Task 6)

- [ ] Add or update E2E tests per slice per [E2E Test Inventory](#e2e-test-inventory-and-what-to-test): MCP tools beyond minimum, verification loop, chat/runtime drifts (retry, bounded wait); assert real outcomes.
- [ ] Run `just setup-dev restart --force` (or `just setup-dev start --force` if the stack is not running); then run `just e2e --tags pma` and/or `--tags chat` (or per-slice tags); fix any failing E2E tests.
- [ ] Run `just ci` and `just e2e`.
- [ ] Confirm Phase 7 scope is complete and no test debt was deferred.
- [ ] Validation gate: plan complete when Task 6 checks pass.

#### Closeout (Task 6)

- [ ] Generate a **task completion report** for Task 6: what was done, what passed, any deviations or notes for follow-up.
- [ ] Do not start Task 7 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 7: Documentation and Closeout

Update cross-cutting docs and confirm no required follow-up work was left undocumented.

#### Task 7 Requirements and Specifications

- This plan and `2026-03-12_plan_next_round_execution.md` (doc removed).

#### Discovery (Task 7) Steps

- [ ] Review 2026-03-12 Progress Notes and Exit Criteria; update Progress Notes with completion state for each phase touched by this plan.
- [ ] List any remaining risks or follow-on work that should be recorded.

#### Red / Green (Task 7)

- [ ] Update 2026-03-12 (or a successor plan) with completion status for Phases 3-8 items addressed by this plan.
- [ ] Document any explicit remaining risks or deferred work.
- [ ] Run `just setup-dev restart --force` (or `just setup-dev start --force` if the stack is not running).
- [ ] Run the **full** E2E suite (`just e2e`); fix any failures until all tests pass and only expected skips remain; then run `just docs-check` and `just ci` one final time.

#### Testing (Task 7)

- [ ] Confirm the full E2E run (in Red/Green above) passed with all tests passing and only expected skips.
- [ ] Confirm all exit criteria below are met or explicitly documented as deferred.
- [ ] Validation gate: plan complete.

#### Closeout (Task 7)

- [ ] Update any required user-facing or developer-facing documentation (if not already done).
- [ ] Verify no required follow-up work was left undocumented.
- [ ] Generate a **final plan completion report**: which tasks were completed, overall validation status (`just ci`, full E2E with only expected skips), any remaining risks or follow-up.
- [ ] Mark all completed steps in the plan with `- [x]`. (Last step.)

---

## Execution Order

Execute in this order; do not start the next task until the current task's Testing validation gate passes.

1. **Task 1** (end-to-end streaming): backend, transport, TUI, PTY; gate `just ci` and `just e2e` pass for streaming and cancellation.
2. **Task 2** (minimum MCP slice): allow path, tool set, PMA wiring; gate `just ci` and targeted E2E pass.
3. **Task 3** (auth recovery, in-session switches, PTY validation): gate `just ci` and `just e2e` pass.
4. **Task 4** (BDD and PTY coverage): gate Phase 6 scope covered and `just ci` and `just e2e` pass.
5. **Task 5** (worker deployment docs): gate `just docs-check` pass.
6. **Task 6** (remaining MVP Phase 2): gate `just ci` and `just e2e` pass.
7. **Task 7** (documentation and closeout): gate plan complete.

## Exit Criteria

- [ ] Prerequisite: 2026-03-14 TUI fix plan is complete and verified.
- [ ] A user can log in, create or switch threads, chat in a multi-line TUI, and observe project and model context.
- [ ] Interactive streaming works end to end on both supported chat surfaces, including progressive visible-text updates, deterministic final reconciliation, and explicit cancellation handling.
- [ ] The minimum MCP-in-the-loop slice required for PMA chat and tool-aware thinking models is implemented and validated against real MCP tool results.
- [ ] The fullscreen TUI can be driven end to end from the Python test scripts with minimal human intervention (auth, thread, thinking, streaming).
- [ ] BDD and PTY/E2E coverage for the TUI path is updated and passes; before final sign-off, run `just setup-dev restart --force` (or `just setup-dev start --force` if the stack is not running), run the **full** E2E suite (`just e2e`), fix any failures until all tests pass and only expected skips remain, then run `just ci` and confirm both pass.
- [ ] Worker deployment docs distinguish normative topology from deferred implementation.
- [ ] Remaining MVP Phase 2 work (Phase 7) is complete or explicitly recorded as follow-on; each task closed with same-phase test updates and gates passing.
