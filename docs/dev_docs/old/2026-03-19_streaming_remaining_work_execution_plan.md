# Streaming Remaining Work: Detailed Execution Plan

- [Plan Status](#plan-status)
  - [Instructions for AI Agents](#instructions-for-ai-agents)
- [Goal](#goal)
- [References](#references)
- [Constraints](#constraints)
- [Execution Plan](#execution-plan)
  - [Task 1: PMA Streaming State Machine, Overwrite, and Secure Buffers](#task-1-pma-streaming-state-machine-overwrite-and-secure-buffers)
  - [Task 2: Gateway Relay Completion, Persistence, and Heartbeat Fallback](#task-2-gateway-relay-completion-persistence-and-heartbeat-fallback)
  - [Task 3: PTY Test Harness Extensions](#task-3-pty-test-harness-extensions)
  - [Task 4: TUI Structured Streaming UX](#task-4-tui-structured-streaming-ux)
  - [Task 5: BDD Step Implementation and E2E Test Matrix](#task-5-bdd-step-implementation-and-e2e-test-matrix)
  - [Task 6: Documentation and Closeout](#task-6-documentation-and-closeout)

## Plan Status

**Created:** 2026-03-19.
**Source:** Extracted from [2026-03-15_streaming_specs_implementation_plan.md](2026-03-15_streaming_specs_implementation_plan.md); validated against current implementation and specs.
**Scope:** All remaining unchecked work from Tasks 2-6 of the streaming plan, including functional, unit, and BDD tests and PTY harness support.

### Instructions for AI Agents

**No work in this plan is to be deferred.**

- Every checkbox in this document represents work that MUST be completed as part of executing the plan.
- Do not mark items "deferred", "optional", or "later" unless the plan explicitly defines a narrow exception (e.g. platform impossibility with a documented follow-up).
- If a step is blocked by a prerequisite, complete the prerequisite first; do not skip or defer the step.
- When closing out a task, do not list "deferred items" as an acceptable outcome; complete the task or document a concrete blocker and follow-up, then continue.
- Agents implementing from this plan must treat each Red/Green/Refactor/Testing step as mandatory.

## Goal

Complete the streaming implementation so that:

- PMA emits typed events (visible, thinking, tool_call) and scoped overwrites on the standard path; secret-bearing buffers use the shared secure-buffer helper.
- The user gateway relays full event model, persists structured assistant turns (redacted only), uses heartbeat fallback when upstream cannot stream, and removes fake chunking (`emitContentAsSSE`).
- The cynork TUI uses a canonical structured transcript model (TranscriptTurn/TranscriptPart), renders one in-flight assistant turn, stores and toggles thinking and tool output, handles overwrite scopes and reconnect.
- All streaming behavior is covered by unit tests, BDD scenarios with implemented steps, and Python E2E (including PTY where required); the PTY harness supports cancel-retain-partial, reconnect, and scrollback assertions needed by the new tests.

## References

- Requirements: [docs/requirements/client.md](../../requirements/client.md) (REQ-CLIENT-0182, 0183, 0184, 0185, 0192, 0193, 0195, 0202, 0204, 0209, 0213-0220), [docs/requirements/usrgwy.md](../../requirements/usrgwy.md) (REQ-USRGWY-0149-0156), [docs/requirements/pmagnt.md](../../requirements/pmagnt.md) (REQ-PMAGNT-0118, 0120-0126), [docs/requirements/stands.md](../../requirements/stands.md) (REQ-STANDS-0133).
- Tech specs: [docs/tech_specs/openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md), [docs/tech_specs/cynode_pma.md](../../tech_specs/cynode_pma.md), [docs/tech_specs/cynork_tui.md](../../tech_specs/cynork_tui.md), [docs/tech_specs/chat_threads_and_messages.md](../../tech_specs/chat_threads_and_messages.md), [docs/tech_specs/cli_management_app_commands_chat.md](../../tech_specs/cli_management_app_commands_chat.md).
- Feature files: [pma_chat_and_context.feature](../../../features/agents/pma_chat_and_context.feature), [openai_compat_chat.feature](../../../features/orchestrator/openai_compat_chat.feature), [cynork_tui_streaming.feature](../../../features/cynork/cynork_tui_streaming.feature), [cynork_tui.feature](../../../features/cynork/cynork_tui.feature), [cynork_tui_threads.feature](../../../features/cynork/cynork_tui_threads.feature), [chat_openai_compatible.feature](../../../features/e2e/chat_openai_compatible.feature).
- Implementation (Go): `agents/internal/pma/`, `orchestrator/internal/handlers/openai_chat.go`, `orchestrator/internal/database/`,
  `cynork/internal/gateway/client.go`, `cynork/internal/chat/transport.go`, `cynork/internal/tui/`, `cynork/_bdd/steps2.go`.
- Implementation (tests): `scripts/test_scripts/` (e2e_0610 through e2e_0760, `tui_pty_harness.py`).

## Constraints

- **No deferral:** Nothing in this plan may be deferred to a later effort.
  Every task and step is in scope and must be completed or explicitly documented as blocked (with follow-up), not postponed.
- Requirements and tech specs are the source of truth; implementation and tests are brought into compliance.
- BDD/TDD: add or update failing tests before implementation; do not start the next task until the current task's Testing gate and Closeout are complete.
- Use repo just targets: `just test-bdd`, `just lint`, `just test-go-cover`, `just e2e --tags pma_inference`, `just e2e --tags chat`, `just e2e --tags tui_pty`, `just docs-check`, `just ci`.
- Rebuild/restart stack with `just setup-dev restart --force` before streaming E2E that depends on gateway, PMA, or TUI.
- Secret-bearing streaming buffers must use the `runtime/secret` helper strategy (REQ-STANDS-0133).
- Do not extend `emitContentAsSSE`; remove or bypass it in favor of heartbeat fallback and real streaming.

## Execution Plan

Execute tasks in order.
Each task is self-contained with Discovery, Red, Green, Refactor, Testing, and Closeout.
Do not start a later task until the current task's Testing gate and Closeout are complete.

---

### Task 1: PMA Streaming State Machine, Overwrite, and Secure Buffers

Complete the PMA standard-path streaming: configurable token state machine (route visible/thinking/tool_call), per-iteration and per-turn overwrite events, and secure-buffer wrapping for secret-bearing stream buffers.

#### Task 1 Requirements and Specifications

- [docs/requirements/pmagnt.md](../../requirements/pmagnt.md) REQ-PMAGNT-0118, 0120-0126.
- [docs/requirements/stands.md](../../requirements/stands.md) REQ-STANDS-0133.
- [docs/tech_specs/cynode_pma.md](../../tech_specs/cynode_pma.md) (StreamingAssistantOutput, StreamingTokenStateMachine, PMAStreamingOverwrite).
- [features/agents/pma_chat_and_context.feature](../../../features/agents/pma_chat_and_context.feature).

#### Discovery (Task 1) Steps

- [ ] Re-read PMA streaming requirements and cynode_pma spec for state machine, overwrite scopes, and secret handling.
- [ ] Inspect `agents/internal/pma/` (chat.go, langchain.go) for current wrapper, event emission, and buffer usage.
- [ ] Confirm where the Task 1 contract (from original plan) secure-buffer helper lives and how PMA should call it.
- [ ] List existing PMA unit tests that cover streaming and identify gaps for state machine, overwrite, and secure buffers.

#### Red (Task 1)

- [ ] Add or update failing PMA unit tests:
  - [ ] State machine routes visible text to `delta`, thinking to `thinking_delta`, tool-call content to `tool_call`; ambiguous partial tags buffered (no leak as visible).
  - [ ] Per-iteration overwrite event replaces only targeted iteration segment.
  - [ ] Per-turn overwrite event replaces entire visible in-flight content.
  - [ ] Secret-bearing append/replace paths use the shared secure-buffer helper.
- [ ] Add or extend BDD scenarios in `pma_chat_and_context.feature` for overwrite and thinking/tool separation where not already covered.
- [ ] Run targeted PMA tests and confirm they fail for the correct reasons.
- [ ] Validation gate: do not proceed until the gaps are proven by failing tests.

#### Green (Task 1)

- [ ] Implement configurable streaming token state machine in PMA:
  - [ ] Route visible text to `delta`, hidden thinking to `thinking`, detected tool-call content to `tool_call`.
  - [ ] Buffer ambiguous partial tags instead of leaking as visible text where possible.
- [ ] Emit PMA overwrite events for both scopes (per-iteration, per-turn) per spec.
- [ ] Wrap PMA secret-bearing stream buffer operations with the Task 1 (original plan) secure-buffer helper.
- [ ] Re-run PMA unit tests until they pass.
- [ ] Validation gate: do not proceed until PMA streaming state machine and overwrite are green.

#### Refactor (Task 1)

- [ ] Extract small helpers for state machine and overwrite logic; remove duplication.
- [ ] Re-run Task 1 targeted tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 1)

- [ ] Run PMA unit and behavior tests; run `just test-go-cover` for affected packages.
- [ ] Run `just e2e --tags pma_inference` and confirm `e2e_0620_pma_standard_path_streaming.py` (and any new overwrite/thinking assertions) pass when stack has PMA ready.
- [ ] Run `just test-bdd` for PMA feature coverage.
- [ ] Run `just lint-go` and `just lint-go-ci` for changed packages.
- [ ] Validation gate: do not start Task 2 until all Task 1 checks pass.

#### Closeout (Task 1)

- [ ] Generate task completion report: what changed (state machine, overwrite, secure-buffer), what tests passed.
  Do not list "deferred items"; complete or document blockers with follow-up.
- [ ] Do not start Task 2 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 2: Gateway Relay Completion, Persistence, and Heartbeat Fallback

Complete the gateway: separate visible/thinking/tool accumulators, native `/v1/responses` format, persist structured assistant turns (redacted only), remove or bypass `emitContentAsSSE`, heartbeat fallback when upstream cannot stream, and client cancellation handling.

#### Task 2 Requirements and Specifications

- [docs/requirements/usrgwy.md](../../requirements/usrgwy.md) REQ-USRGWY-0149-0156.
- [docs/requirements/client.md](../../requirements/client.md) REQ-CLIENT-0182, 0184, 0185, 0215-0220.
- [docs/requirements/stands.md](../../requirements/stands.md) REQ-STANDS-0133.
- [docs/tech_specs/openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md) (Streaming, StreamingRedactionPipeline, StreamingPerEndpointSSEFormat, StreamingHeartbeatFallback).
- [docs/tech_specs/chat_threads_and_messages.md](../../tech_specs/chat_threads_and_messages.md) (structured parts).
- [features/orchestrator/openai_compat_chat.feature](../../../features/orchestrator/openai_compat_chat.feature), [features/e2e/chat_openai_compatible.feature](../../../features/e2e/chat_openai_compatible.feature).

#### Discovery (Task 2) Steps

- [ ] Re-read gateway streaming requirements and openai_compatible_chat_api spec (relay, accumulators, persistence, heartbeat, cancellation).
- [ ] Inspect `orchestrator/internal/handlers/openai_chat.go` and database/thread persistence for current relay and persistence paths.
- [ ] Locate all uses of `emitContentAsSSE` and define replacement (heartbeat + final delta).
- [ ] Confirm e2e_0630_gateway_streaming_contract.py test list and which tests currently skip or pass.

#### Red (Task 2)

- [ ] Add or update failing gateway handler and integration tests:
  - [ ] Separate visible, thinking, and tool-call accumulators; overwrite events applied to correct scope.
  - [ ] Post-stream redaction on all three accumulators before terminal completion.
  - [ ] `/v1/responses` native event model and streamed response_id.
  - [ ] Persisted assistant turn has structured parts (thinking, tool_call) with redacted content only.
  - [ ] Heartbeat SSE when upstream does not stream; no use of `emitContentAsSSE` on standard path.
  - [ ] Client disconnect cancels stream and does not leave upstream running indefinitely.
- [ ] Add or extend database/integration tests for persisted structured parts (thinking, tool_call).
- [ ] Add or update failing Python tests in `e2e_0630_gateway_streaming_contract.py`:
  - [ ] `test_responses_stream_relays_native_response_output_text_and_completed_events`
  - [ ] `test_streamed_responses_id_can_be_reused_as_previous_response_id`
  - [ ] Ensure `test_stream_amendment_arrives_before_terminal_completion`, `test_stream_heartbeat_fallback_emits_progress_then_final_visible_text`, `test_client_disconnect_is_treated_as_stream_cancellation`, `test_streamed_structured_parts_are_persisted_redacted_only` are either failing for the right reason or updated to assert new behavior.
- [ ] Run targeted gateway and e2e_0630 tests; confirm failures for correct reasons.
- [ ] Validation gate: do not proceed until gateway gaps are proven.

#### Green (Task 2)

- [ ] Maintain separate visible-text, thinking, and tool-call accumulators in the gateway relay.
- [ ] Apply per-iteration and per-turn overwrite events to the correct accumulator scope; run post-stream secret scan on all three before terminal completion.
- [ ] Emit `/v1/responses` in native responses event model with named `cynodeai.*` extensions and streamed response_id.
- [ ] Persist final redacted structured assistant turn (including streamed response_id metadata where applicable).
- [ ] Remove or bypass `emitContentAsSSE`; use heartbeat SSE plus one final visible-text delta when true upstream streaming is unavailable.
- [ ] Treat client cancellation/disconnect as stream cancellation; do not leave upstream work running indefinitely.
- [ ] Wrap gateway secret-bearing accumulator paths with the shared secure-buffer helper.
- [ ] Re-run gateway and e2e_0630 tests until they pass.
- [ ] Validation gate: do not proceed until gateway relay, persistence, and fallback are green.

#### Refactor (Task 2)

- [ ] Extract relay and accumulator helpers; share logic between chat-completions and responses paths without collapsing wire formats.
- [ ] Remove obsolete fake-stream and single-accumulator code.
- [ ] Re-run Task 2 targeted tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 2)

- [ ] Run gateway handler, integration, and persistence tests.
- [ ] Run `just setup-dev restart --force` then `just e2e --tags chat`; confirm e2e_0630 tests pass when stack is ready.
- [ ] Run `just test-bdd` for orchestrator/openai_compat_chat feature coverage.
- [ ] Run `just lint-go` and `just lint-go-ci` for changed packages.
- [ ] Validation gate: do not start Task 3 until all Task 2 checks pass.

#### Closeout (Task 2)

- [ ] Generate task completion report: what changed (accumulators, /v1/responses, persistence, heartbeat, cancellation, secure-buffer), what tests passed.
- [ ] Do not start Task 3 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 3: PTY Test Harness Extensions

Extend the PTY harness so that E2E tests can assert cancel-and-retain-partial, reconnect-and-mark-interrupted, and scrollback content (e.g. thinking/tool placeholders) required by TUI streaming scenarios.
All listed harness work is in scope; do not defer.

#### Task 3 Requirements and Specifications

- [docs/tech_specs/cynork_tui.md](../../tech_specs/cynork_tui.md) (TranscriptRendering, GenerationState, ConnectionRecovery).
- [features/cynork/cynork_tui_streaming.feature](../../../features/cynork/cynork_tui_streaming.feature).
- Current harness: [scripts/test_scripts/tui_pty_harness.py](../../../scripts/test_scripts/tui_pty_harness.py); landmarks in `cynork/internal/chat/landmarks.go`.

#### Discovery (Task 3) Steps

- [ ] Re-read TUI streaming feature scenarios that require PTY: cancel and retain partial text; reconnect and preserve partial / mark interrupted; show-thinking / show-tool-output revealing stored content.
- [ ] Inspect `tui_pty_harness.py` for existing APIs: `TuiPtySession`, `send_keys`, `read_until_landmark`, `wait_for_prompt_ready`, `wait_for_login_form`, `capture_screen`.
- [ ] List exact assertions needed by e2e_0750 (cancel, reconnect), e2e_0760 (show-thinking, show-tool-output, hide-tool-output), e2e_0650 (progressive turn, overwrite, heartbeat) that the harness must support (e.g. wait for partial content in scrollback; new landmarks if required).
- [ ] Check whether new landmarks (e.g. interrupted turn) are needed in `landmarks.go` and TUI for E2E; document in plan.

#### Red (Task 3)

- [ ] Add failing Python tests (or extend existing e2e_0750/e2e_0760/e2e_0650) that require new harness behavior:
  - [ ] Cancel stream (Ctrl+C) then assert retained partial text in scrollback (e2e_0750).
  - [ ] Simulate reconnect (or document limitation) and assert partial text preserved and turn marked interrupted (e2e_0750).
  - [ ] After a turn with thinking/tool content, assert /show-thinking and /show-tool-output reveal content without refetch (e2e_0760); /hide-tool-output recovers collapsed placeholders.
- [ ] Run these tests and confirm they fail due to missing harness or TUI behavior (not environment).
- [ ] Validation gate: do not proceed until required harness capabilities are clearly specified and tests fail for the right reason.

#### Green (Task 3)

- [ ] Extend `tui_pty_harness.py` as needed:
  - [ ] Helper to wait for a string or pattern in scrollback (e.g. after `capture_screen` or incremental read), or extend `read_until_landmark` to support scrollback content checks.
  - [ ] Implement helper to send message, wait for in-flight, then send Ctrl+C and collect scrollback for "retained partial" assertion.
  - [ ] Implement reconnect E2E: helper or procedure to restart TUI and re-attach to same thread and assert interrupted state.
    If platform or harness limits make this impossible, document the blocker and create a follow-up; do not defer the requirement.
- [ ] If TUI or backend must emit a new landmark for "interrupted turn" or "streaming state", add it to `landmarks.go` and document in this task's closeout.
- [ ] Re-run the new or updated E2E tests that depend on the harness; confirm they pass.
  Only skip a test if it is technically impossible (e.g. no PTY on platform) and skip is documented; do not skip to defer.
- [ ] Validation gate: do not proceed until harness extensions are implemented and tests using them are green (or skip only with documented technical reason).

#### Refactor (Task 3)

- [ ] Reuse common patterns (e.g. send message + wait for landmark) in harness or test helpers; avoid duplication across e2e_0750, e2e_0760, e2e_0650.
- [ ] Re-run affected E2E tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 3)

- [ ] Run `just e2e --tags tui_pty` and confirm e2e_0750, e2e_0760 (and any new tests) pass.
  Skips only with documented technical reason; no deferral.
- [ ] Run `just lint-python` for `scripts/test_scripts/`.
- [ ] Validation gate: do not start Task 4 until Task 3 checks pass.

#### Closeout (Task 3)

- [ ] Generate task completion report: what harness APIs were added, which E2E tests use them, any new landmarks or TUI contracts.
- [ ] Do not start Task 4 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 4: TUI Structured Streaming UX

Wire the TUI to the richer event model: canonical TranscriptTurn/TranscriptPart in-memory model, one in-flight assistant turn, stored thinking and tool content with toggles, overwrite scopes, heartbeat progress, and reconnect/interrupted-turn reconciliation; secure-buffer for in-flight content.

#### Task 4 Requirements and Specifications

- [docs/requirements/client.md](../../requirements/client.md) REQ-CLIENT-0182-0185, 0192, 0193, 0195, 0202, 0204, 0209, 0213-0220.
- [docs/requirements/stands.md](../../requirements/stands.md) REQ-STANDS-0133.
- [docs/tech_specs/cynork_tui.md](../../tech_specs/cynork_tui.md) (TranscriptRendering, GenerationState, ThinkingContentStorageDuringStreaming, ToolCallContentStorageDuringStreaming, SecureBufferHandlingForInFlightStreamingContent, ConnectionRecovery).
- [features/cynork/cynork_tui_streaming.feature](../../../features/cynork/cynork_tui_streaming.feature), [features/cynork/cynork_tui.feature](../../../features/cynork/cynork_tui.feature), [features/cynork/cynork_tui_threads.feature](../../../features/cynork/cynork_tui_threads.feature).

#### Discovery (Task 4) Steps

- [ ] Re-read TUI streaming requirements and cynork_tui spec (transcript, generation state, thinking/tool storage, overwrite, heartbeat, reconnect).
- [ ] Inspect `cynork/internal/tui/state.go` and `model.go` for TranscriptTurn, TranscriptPart, and current streaming/scrollback logic.
- [ ] Confirm cynork transport already exposes thinking, tool_call, iteration_start, heartbeat (from Task 4 of original plan); list any remaining transport gaps for TUI.

#### Red (Task 4)

- [ ] Add or update failing TUI unit tests:
  - [ ] Exactly one in-flight assistant turn updated in place during streaming.
  - [ ] Hidden-by-default thinking placeholders; expand when enabled without refetch.
  - [ ] Tool-call and tool-result as distinct non-prose items; toggle show/hide.
  - [ ] Per-iteration overwrite replaces only targeted segment; per-turn overwrite replaces entire visible in-flight text.
  - [ ] Heartbeat renders as progress indicator; does not pollute transcript content.
  - [ ] Cancellation and reconnect retain received content and reconcile active turn deterministically.
- [ ] Add or update failing TUI history-loading tests: persisted structured parts rehydrate correctly after reload.
- [ ] Update e2e_0750 with failing PTY tests: `test_tui_ctrl_c_cancels_stream_and_retains_partial_text`, `test_tui_reconnect_preserves_partial_text_and_marks_turn_interrupted` (if harness supports).
- [ ] Update e2e_0760 with failing tests: `test_tui_slash_show_thinking_reveals_stored_reasoning_without_refetch`, `test_tui_slash_show_tool_output_reveals_stored_tool_content_without_refetch`, `test_tui_slash_hide_tool_output_recovers_collapsed_tool_placeholders`.
- [ ] Ensure e2e_0650 tests (`test_tui_updates_single_inflight_turn_progressively`, `test_tui_iteration_scoped_overwrite_only_replaces_target_segment`, `test_tui_turn_scoped_amendment_replaces_visible_text_without_duplication`, `test_tui_heartbeat_progress_indicator_disappears_after_final_content`) fail for the right reasons (TUI not yet wired).
- [ ] Run targeted TUI unit and E2E tests; confirm failures.
- [ ] Validation gate: do not proceed until TUI streaming UX gap is proven.

#### Green (Task 4)

- [ ] Promote TranscriptTurn, TranscriptPart, and SessionState to canonical in-memory streaming representation in TUI.
- [ ] Render exactly one logical assistant turn per user prompt; update that turn in place while streaming.
- [ ] Store and render structured streaming content: visible text; hidden-by-default thinking with instant reveal when enabled; tool-call/tool-result as non-prose items with toggle.
- [ ] Implement per-iteration and per-turn overwrite handling.
- [ ] Render heartbeat as display-only progress attached to in-flight turn; remove when final content arrives.
- [ ] Implement bounded-backoff reconnect and interrupted-turn reconciliation.
- [ ] Wrap TUI secret-bearing stream-buffer append/replace paths with the shared secure-buffer helper.
- [ ] Re-run TUI unit and E2E tests until they pass.
- [ ] Validation gate: do not proceed until TUI streaming UX is green.

#### Refactor (Task 4)

- [ ] Extract transcript-building, overwrite-handling, and status-rendering helpers; remove obsolete string-only stream bookkeeping.
- [ ] Re-run Task 4 targeted tests.
- [ ] Validation gate: do not proceed until TUI update and render flow are stable and readable.

#### Testing (Task 4)

- [ ] Run TUI unit tests for streaming, transcript rendering, and reconnect.
- [ ] Run `just setup-dev restart --force` then `just e2e --tags tui_pty`; confirm e2e_0750, e2e_0760, e2e_0650 scenarios pass.
- [ ] Run `just lint-go` for `cynork/internal/tui` and adjacent packages; `just test-go-cover` for coverage.
- [ ] Run `just test-bdd` and confirm no TUI scenario regressions.
- [ ] Validation gate: do not start Task 5 until all Task 4 checks pass.

#### Closeout (Task 4)

- [ ] Generate task completion report: what changed (transcript state, rendering, overwrite, heartbeat, reconnect, secure-buffer), what tests passed.
- [ ] Do not start Task 5 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 5: BDD Step Implementation and E2E Test Matrix

Replace remaining streaming and PTY BDD placeholders in `cynork/_bdd/steps2.go` with real step implementations or failing assertions; finish the Python E2E test matrix and ensure all streaming tags pass.

#### Task 5 Requirements and Specifications

- [features/cynork/cynork_tui.feature](../../../features/cynork/cynork_tui.feature), [features/cynork/cynork_tui_streaming.feature](../../../features/cynork/cynork_tui_streaming.feature), [features/cynork/cynork_tui_threads.feature](../../../features/cynork/cynork_tui_threads.feature).
- [features/orchestrator/openai_compat_chat.feature](../../../features/orchestrator/openai_compat_chat.feature), [features/agents/pma_chat_and_context.feature](../../../features/agents/pma_chat_and_context.feature), [features/e2e/chat_openai_compatible.feature](../../../features/e2e/chat_openai_compatible.feature).
- BDD steps: [cynork/_bdd/steps2.go/../../../cynork/_bdd/steps2.go) (streaming and PTY steps currently returning `godog.ErrPending`).

#### Discovery (Task 5) Steps

- [ ] List every step in steps2.go that returns `godog.ErrPending` and classify: streaming setup/assertion, PTY-required, or other (e.g. queue draft).
- [ ] Map each pending step to the feature scenario and to the implementation (gateway, TUI, transport) that will make it pass.
- [ ] Confirm Python E2E file ownership: e2e_0610 (API event shapes), e2e_0620 (PMA NDJSON), e2e_0630 (gateway contract), e2e_0640 (cynork transport), e2e_0650 (TUI streaming behavior), e2e_0750 (PTY cancel/reconnect), e2e_0760 (slash toggles).

#### Red (Task 5)

- [ ] Replace streaming-related `godog.ErrPending` steps with implementations that fail against current behavior where implementation is still missing, or with assertions that fail until Tasks 1-4 are complete:
  - [ ] Gateway streaming setup steps (e.g. "the TUI has sent a message and the gateway is streaming the assistant response", "the gateway returns a structured assistant turn with visible text and thinking").
  - [ ] TUI streaming assertion steps (e.g. "the visible text is shown in the transcript", "the TUI shows a visible in-flight indicator", "visible assistant text is appended token-by-token within one in-flight assistant turn", "retained thinking parts in the scrollback are displayed as expanded thinking blocks", etc.).
- [ ] For PTY-required steps: implement via PTY-driven BDD where feasible, or implement the step so it runs in the BDD environment (e.g. mock or subprocess).
  Do not leave steps pending to defer work; only use pending when the step genuinely cannot run in BDD (e.g. requires interactive PTY and BDD has no PTY), and document that in closeout.
- [ ] Run `just test-bdd` and confirm streaming scenarios fail where implementation is incomplete.
  Do not use "pending" or "skip" as a way to defer; implement steps so scenarios pass.
- [ ] Validation gate: do not proceed until BDD step strategy is clear and test-bdd reflects current state.

#### Green (Task 5)

- [ ] Implement or wire each streaming BDD step so that after Tasks 1-4 the steps pass:
  - [ ] Setup steps that configure mock gateway or TUI state for streaming.
  - [ ] Assertion steps that check transcript, in-flight indicator, thinking/tool placeholders, overwrite, heartbeat, amendment.
- [ ] Only if a step truly cannot run in BDD (e.g. requires real interactive PTY and BDD runs without one), document that and use skip/pending for that step only; ensure no false positives.
  Do not use skip/pending to defer implementable work.
- [ ] Re-run `just test-bdd` until streaming scenarios pass (or the only remaining skips are documented technical impossibilities).
- [ ] Validation gate: do not proceed until test-bdd passes for all implemented streaming scenarios.

#### Refactor (Task 5)

- [ ] Extract shared BDD step helpers (e.g. parse SSE, check scrollback content) to avoid duplication in steps2.go or test helpers.
- [ ] Re-run `just test-bdd`.
- [ ] Validation gate: do not proceed until BDD suite is stable.

#### Testing (Task 5)

- [ ] Run `just test-bdd`.
- [ ] Run `just setup-dev restart --force` then run `just e2e --tags pma_inference`, `just e2e --tags chat`, `just e2e --tags tui_pty`; confirm all streaming E2E files pass for their tags when stack is ready.
- [ ] Run full `just e2e` and confirm no regressions.
- [ ] Validation gate: do not start Task 6 until all Task 5 checks pass.

#### Closeout (Task 5)

- [ ] Generate task completion report: which BDD steps were implemented, which remain pending and why; which E2E tags and files pass.
- [ ] Do not start Task 6 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 6: Documentation and Closeout

Update user-facing and developer-facing docs to reflect streaming behavior; produce final plan completion report and verify no streaming BDD steps remain incorrectly pending and no gateway fake-stream remains on the standard path.

#### Task 6 Requirements and Specifications

- [docs/requirements/client.md](../../requirements/client.md), [docs/requirements/usrgwy.md](../../requirements/usrgwy.md), [docs/requirements/pmagnt.md](../../requirements/pmagnt.md).
- [docs/tech_specs/cynork_tui.md](../../tech_specs/cynork_tui.md), [docs/tech_specs/openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md), [docs/tech_specs/cynode_pma.md](../../tech_specs/cynode_pma.md), [docs/tech_specs/chat_threads_and_messages.md](../../tech_specs/chat_threads_and_messages.md).

#### Discovery (Task 6) Steps

- [ ] Identify user-facing help (CLI, TUI) and developer docs that must be updated for new streaming controls (e.g. /show-tool-output, /hide-tool-output), heartbeat, and reconnect behavior.
- [ ] Verify Python E2E files have accurate trace comments, tags, and scenario names matching the final contract and requirement/spec IDs.

#### Red (Task 6)

- [ ] N/A for closeout task.

#### Green (Task 6)

- [ ] Update user-facing or developer-facing documentation for streaming behavior (help text, examples, implementation notes).
- [ ] Add implementation note recording any intentionally preserved fallback behavior or non-goals.
- [ ] Finalize trace comments, class docstrings, and tags in: e2e_0610_sse_streaming.py, e2e_0620_pma_standard_path_streaming.py, e2e_0630_gateway_streaming_contract.py, e2e_0640_cynork_transport_streaming.py, e2e_0650_tui_streaming_behavior.py, e2e_0750_tui_pty.py, e2e_0760_tui_slash_commands.py.

#### Refactor (Task 6)

- [ ] Remove stale comments and references to previously deferred streaming work that no longer apply (this plan does not defer; all work is in scope).
- [ ] Re-run `just lint-md` and `just docs-check` for changed docs.

#### Testing (Task 6)

- [ ] Run `just lint`, `just test-go-cover`, `just test-bdd`, `just e2e --tags pma_inference`, `just e2e --tags chat`, `just e2e --tags tui_pty`, `just docs-check`, `just ci`.
- [ ] Confirm no streaming BDD steps remain pending unless explicitly documented.
- [ ] Confirm no gateway fake-stream (`emitContentAsSSE`) on the standard path.
- [ ] Validation gate: do not mark the streaming implementation complete until the full validation set passes.

#### Closeout (Task 6)

- [ ] Generate final plan completion report: which tasks were completed, which validation commands passed, any remaining risks or follow-up items.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)
