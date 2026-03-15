# Streaming Spec Implementation Plan

- [Plan Goal](#plan-goal)
- [Reference Set](#reference-set)
- [Outstanding Work Review](#outstanding-work-review)
- [Key Constraints](#key-constraints)
- [Execution Plan](#execution-plan)
  - [Task 1: Lock the Streaming Contract](#task-1-lock-the-streaming-contract)
  - [Task 2: Implement PMA Standard-Path Streaming](#task-2-implement-pma-standard-path-streaming)
  - [Task 3: Implement Gateway Relay, Redaction, and Persistence](#task-3-implement-gateway-relay-redaction-and-persistence)
  - [Task 4: Align Cynork Streaming Transports and Parsers](#task-4-align-cynork-streaming-transports-and-parsers)
  - [Task 5: Wire the TUI Structured Streaming UX](#task-5-wire-the-tui-structured-streaming-ux)
  - [Task 6: Finish BDD and E2E Streaming Coverage](#task-6-finish-bdd-and-e2e-streaming-coverage)
  - [Task 7: Documentation and Closeout](#task-7-documentation-and-closeout)

## Plan Goal

This plan replaces the stale streaming remainder from the prior TUI fix effort with an end-to-end implementation plan for the current streaming contract.
The target outcome is spec-compliant streaming across PMA, the user gateway, `cynork` transports, the TUI transcript, and the streaming BDD and E2E suites, with no fake chunking and no pending streaming steps left behind.

## Reference Set

- [docs/requirements/client.md](../requirements/client.md) with focus on `REQ-CLIENT-0182`, `REQ-CLIENT-0183`, `REQ-CLIENT-0184`, `REQ-CLIENT-0185`, `REQ-CLIENT-0192`, `REQ-CLIENT-0193`, `REQ-CLIENT-0195`, `REQ-CLIENT-0202`, `REQ-CLIENT-0204`, `REQ-CLIENT-0209`, and `REQ-CLIENT-0213` through `REQ-CLIENT-0220`.
- [docs/requirements/usrgwy.md](../requirements/usrgwy.md) with focus on `REQ-USRGWY-0149` through `REQ-USRGWY-0156`.
- [docs/requirements/pmagnt.md](../requirements/pmagnt.md) with focus on `REQ-PMAGNT-0118` and `REQ-PMAGNT-0120` through `REQ-PMAGNT-0126`.
- [docs/requirements/stands.md](../requirements/stands.md) with focus on `REQ-STANDS-0133`.
- [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md) with focus on `TranscriptRendering`, `GenerationState`, `ThinkingContentStorageDuringStreaming`, `ToolCallContentStorageDuringStreaming`, `SecureBufferHandlingForInFlightStreamingContent`, and `ConnectionRecovery`.
- [docs/tech_specs/openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md) with focus on `Streaming`, `StreamingRedactionPipeline`, `StreamingPerEndpointSSEFormat`, and `StreamingHeartbeatFallback`.
- [docs/tech_specs/cynode_pma.md](../tech_specs/cynode_pma.md) with focus on `StreamingAssistantOutput`, `StreamingLLMWrapper`, `StreamingTokenStateMachine`, `PMAStreamingNDJSONFormat`, and `PMAStreamingOverwrite`.
- [docs/tech_specs/chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md) with focus on structured turn `parts`, thinking retention, and tool-call persistence.
- [docs/tech_specs/cli_management_app_commands_chat.md](../tech_specs/cli_management_app_commands_chat.md) with focus on thread continuity and connection interruption behavior.
- [features/agents/pma_chat_and_context.feature](../../features/agents/pma_chat_and_context.feature).
- [features/orchestrator/openai_compat_chat.feature](../../features/orchestrator/openai_compat_chat.feature).
- [features/cynork/cynork_tui.feature](../../features/cynork/cynork_tui.feature), [features/cynork/cynork_tui_streaming.feature](../../features/cynork/cynork_tui_streaming.feature), and [features/cynork/cynork_tui_threads.feature](../../features/cynork/cynork_tui_threads.feature).
- [features/e2e/chat_openai_compatible.feature](../../features/e2e/chat_openai_compatible.feature).
- Implementation areas currently touched by the streaming gap: `agents/internal/pma/`, `orchestrator/internal/handlers/openai_chat.go`, `orchestrator/internal/database/`, `cynork/internal/gateway/client.go`, `cynork/internal/chat/transport.go`, `cynork/internal/tui/`, `cynork/_bdd/`, and `scripts/test_scripts/`.

## Outstanding Work Review

- The previous TUI fix plan closed the non-streaming slash, thread, coverage, and lint work, but it explicitly deferred the streaming feature group and the larger generation-state and reconnect work.
- PMA still provides real incremental output only on the direct-inference path.
  The capable-model plus MCP path remains blocking, which means the standard production flow still violates the new streaming requirements.
- The gateway still falls back to `emitContentAsSSE` for buffered results.
  That is fake chunking, not the heartbeat fallback or real streaming contract now required by the current specs.
- The current gateway relay and `cynork` client only cover a narrow `delta` plus post-stream amendment model.
  They do not yet implement the updated typed event surface for `thinking`, `tool_call`, `tool_progress`, `iteration_start`, scoped overwrites, heartbeat fallback, or native `/v1/responses` streaming events.
- `cynork` already has some useful groundwork in place, including a transport split between completions and responses, basic amendment replacement in `streamBuf`, and persisted `show thinking` preferences.
  That groundwork must be preserved, not reworked blindly.
- The TUI still uses flat scrollback strings as the primary streaming source of truth.
  `state.go` defines structured transcript and connection state types, but the model is not yet wired to use them as the canonical in-memory streaming representation.
- The streaming BDD steps remain pending in `cynork/_bdd/steps2.go`, and the current E2E coverage only validates basic visible deltas plus `[DONE]`.
  There is no current end-to-end coverage for heartbeat fallback, per-iteration overwrite, reconnect, stored thinking, stored tool output, or native responses-stream event handling.
- This plan locks the tool-output control surface to mirror the existing thinking-visibility controls.
  The implementation target is `/show-tool-output`, `/hide-tool-output`, and persisted local config key `tui.show_tool_output_by_default`.

## Key Constraints

- Requirements are the source of truth, then tech specs, then feature files, then current implementation.
- This plan assumes code and tests will be brought into compliance with the existing updated streaming documents, not that the canonical requirements or specs will be rewritten during implementation.
- Preserve already-complete chat and TUI work from the earlier fix effort unless a failing test proves that it must change for streaming compliance.
- Do not add more behavior on top of `emitContentAsSSE`.
  Remove or bypass that fake-streaming path as part of the gateway work rather than extending it.
- Use BDD and TDD throughout.
  Each task below must prove the gap with failing tests before implementation and must pass its validation gate before the next task begins.
- Use repo `just` targets for validation.
  The recurring gates for this work are `just test-bdd`, `just lint`, `just test-go-cover`, `just e2e --tags chat`, `just e2e --tags tui_pty`, `just docs-check`, and final `just ci`.
- Rebuild or restart the local stack with `just setup-dev restart --force` before any streaming E2E run that depends on the gateway, PMA, or TUI PTY harness.
- Every task below must include explicit Python functional test work in `scripts/test_scripts/`.
  Extend the current streaming files first (`e2e_127_sse_streaming.py`, `e2e_198_tui_pty.py`, and `e2e_199_tui_slash_commands.py`) and reserve `e2e_201_pma_standard_path_streaming.py`, `e2e_202_gateway_streaming_contract.py`, `e2e_203_cynork_transport_streaming.py`, and `e2e_204_tui_streaming_behavior.py` for the new streaming coverage that does not fit cleanly into the existing files.
- Treat secret-bearing streaming buffers as sensitive code paths.
  All PMA, gateway, and TUI append or replace operations that can temporarily hold unredacted content must use the `runtime/secret` helper strategy required by `REQ-STANDS-0133`.
- Treat tool-output controls as the tool-output analogue of thinking visibility for this workstream.
  Implement `/show-tool-output`, `/hide-tool-output`, and persisted local config key `tui.show_tool_output_by_default` as part of the execution plan below.

## Execution Plan

Execute the tasks in order.
Each task is self-contained and includes its own discovery, Red, Green, Refactor, Testing, and Closeout gates.
Do not start a later task until the current task's Testing gate and Closeout steps are complete.

---

### Task 1: Lock the Streaming Contract

Lock the updated event taxonomy, secure-buffer helper strategy, and test fixtures before touching the PMA, gateway, or TUI implementations.

#### Task 1 Requirements and Specifications

- [docs/requirements/client.md](../requirements/client.md) for `REQ-CLIENT-0209` and `REQ-CLIENT-0215` through `REQ-CLIENT-0220`.
- [docs/requirements/usrgwy.md](../requirements/usrgwy.md) for `REQ-USRGWY-0149` through `REQ-USRGWY-0156`.
- [docs/requirements/pmagnt.md](../requirements/pmagnt.md) for `REQ-PMAGNT-0118` and `REQ-PMAGNT-0120` through `REQ-PMAGNT-0126`.
- [docs/requirements/stands.md](../requirements/stands.md) for `REQ-STANDS-0133`.
- [docs/tech_specs/openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md).
- [docs/tech_specs/cynode_pma.md](../tech_specs/cynode_pma.md).
- [features/agents/pma_chat_and_context.feature](../../features/agents/pma_chat_and_context.feature).
- [features/orchestrator/openai_compat_chat.feature](../../features/orchestrator/openai_compat_chat.feature).
- [features/e2e/chat_openai_compatible.feature](../../features/e2e/chat_openai_compatible.feature).

#### Discovery Steps (Task 1)

- [ ] Re-read the streaming requirements, specs, and feature files listed above and capture the exact wire-level contract that implementation must satisfy.
- [ ] Inspect the current shared event and parser surface in `go_shared_libs/contracts/userapi/`, `agents/internal/pma/`, `orchestrator/internal/handlers/openai_chat.go`, `cynork/internal/gateway/client.go`, and `cynork/internal/chat/transport.go`.
- [ ] Confirm the current gap list in code and tests:
  - [ ] PMA standard-path streaming still blocks on the capable-model plus MCP path.
  - [ ] The gateway still emits buffered text through `emitContentAsSSE`.
  - [ ] The responses streaming path in `cynork` still parses chat-completions chunks instead of native responses events.
  - [ ] No shared contract yet covers `thinking`, `tool_call`, `tool_progress`, `iteration_start`, scoped overwrite, heartbeat, and streamed response-id handling end to end.
- [ ] Lock the tool-output control surface for this workstream to mirror thinking visibility behavior:
  - [ ] `/show-tool-output`
  - [ ] `/hide-tool-output`
  - [ ] persisted local config key `tui.show_tool_output_by_default`
- [ ] Identify the reusable secure-buffer helper that should be shared across PMA, gateway, and TUI.
  - [ ] Prefer extracting the existing build-tagged secret wrapper pattern instead of inventing module-local copies.
- [ ] Review the current Python functional streaming coverage in `scripts/test_scripts/e2e_127_sse_streaming.py`, `scripts/test_scripts/e2e_198_tui_pty.py`, and `scripts/test_scripts/e2e_199_tui_slash_commands.py` and map each existing test to the updated contract so Task 1 can name exactly what must be extended instead of creating duplicate API coverage.

#### Red Phase (Task 1)

- [ ] Add or update failing unit tests that lock the shared streaming contract before implementation:
  - [ ] Chat-completions SSE visible-text delta format.
  - [ ] Responses SSE native text-event format and streamed response id.
  - [ ] `cynodeai.thinking_delta`, `cynodeai.tool_call`, `cynodeai.tool_progress`, `cynodeai.iteration_start`, `cynodeai.amendment`, and `cynodeai.heartbeat` event expectations.
  - [ ] Overwrite metadata for `scope` and `iteration`.
- [ ] Update `scripts/test_scripts/e2e_127_sse_streaming.py` so its SSE parsing helper captures named `event:` lines, preserves event order, and can assert chat-completions versus responses native event shapes instead of treating every stream payload as anonymous `data:`.
- [ ] Add failing Python functional tests in `scripts/test_scripts/e2e_127_sse_streaming.py`:
  - [ ] `test_chat_completions_stream_exposes_named_cynodeai_extension_events`
  - [ ] `test_responses_stream_uses_native_responses_events_and_exposes_streamed_response_id`
- [ ] Add or update shared mock PMA and mock gateway fixtures so later tasks use one contract source instead of duplicating test event shapes in each module.
- [ ] Run `just setup-dev restart --force`.
- [ ] Run `just e2e --tags pma_inference` and confirm the updated `e2e_127_sse_streaming.py` contract tests fail for the expected pre-implementation reasons.
- [ ] Run the targeted contract and parser tests and confirm they fail for the expected missing or incorrect behaviors.
- [ ] Validation gate: do not proceed until the failing tests prove the shared contract gap.

#### Green Phase (Task 1)

- [ ] Define or update the shared streaming event types and parser helpers used across PMA, gateway, and `cynork`.
  - [ ] Keep `/v1/chat/completions` and `/v1/responses` native streaming formats distinct where the spec requires it.
  - [ ] Carry streamed response-id, overwrite scope, iteration metadata, and heartbeat payload data explicitly.
  - [ ] Expose the minimum structured event model that later tasks can build on without another contract rewrite.
- [ ] Extract the reusable secure-buffer helper needed for `runtime/secret` guarded append and replace operations.
- [ ] Update only the downstream compile surface needed so Tasks 2 through 5 can build against the locked contract.
- [ ] Re-run the Task 1 targeted tests until they pass.
- [ ] Validation gate: do not proceed until the shared contract and helper layer is green.

#### Refactor Phase (Task 1)

- [ ] Simplify helper names, fixture builders, and contract comments without changing behavior.
- [ ] Remove duplicate event-shape literals from tests once the shared fixtures exist.
- [ ] Re-run the targeted Task 1 tests.
- [ ] Validation gate: do not proceed until the contract surface is readable and stable.

#### Testing Steps (Task 1)

- [ ] Run the targeted contract and parser tests in the affected modules.
- [ ] Run `just setup-dev restart --force`.
- [ ] Run `just e2e --tags pma_inference` and confirm the updated `e2e_127_sse_streaming.py` contract tests now pass.
- [ ] Run `just lint-go` for any changed shared packages.
- [ ] Confirm that the locked contract now matches the updated requirement, spec, and feature set.
- [ ] Validation gate: do not start Task 2 until all Task 1 checks pass.

#### Closeout (Task 1)

- [ ] Generate a task completion report for Task 1:
  - [ ] What contract types, helpers, and fixtures were added or changed.
  - [ ] What targeted tests and lint checks passed.
  - [ ] Any remaining non-blocking follow-ups before user-facing work starts.
- [ ] Do not start Task 2 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 2: Implement PMA Standard-Path Streaming

Make PMA produce real incremental output on the standard capable-model plus MCP path, not only on the fallback direct-inference path.

#### Task 2 Requirements and Specifications

- [docs/requirements/pmagnt.md](../requirements/pmagnt.md) for `REQ-PMAGNT-0118` and `REQ-PMAGNT-0120` through `REQ-PMAGNT-0126`.
- [docs/requirements/stands.md](../requirements/stands.md) for `REQ-STANDS-0133`.
- [docs/tech_specs/cynode_pma.md](../tech_specs/cynode_pma.md) with focus on `StreamingAssistantOutput`, `StreamingLLMWrapper`, `StreamingTokenStateMachine`, `PMAStreamingNDJSONFormat`, and `PMAStreamingOverwrite`.
- [features/agents/pma_chat_and_context.feature](../../features/agents/pma_chat_and_context.feature).
- Task 1 contract artifacts and fixtures.

#### Discovery Steps (Task 2)

- [ ] Re-read the PMA requirements, specs, and feature scenarios for streaming assistant output, thinking separation, tool-call separation, iteration boundaries, overwrite behavior, and opportunistic secret scanning.
- [ ] Inspect the current standard path in `agents/internal/pma/chat.go`, `agents/internal/pma/langchain.go`, and the current direct-stream helpers that can be reused.
- [ ] Identify exactly where the current standard path blocks, where tool-call fallback rewrites happen, and where `runtime/secret` wrapping must be introduced.
- [ ] Confirm the current tests that already cover direct streaming and the gaps that still exist for the standard path.
- [ ] Keep PMA-path Python functional coverage in dedicated file `scripts/test_scripts/e2e_201_pma_standard_path_streaming.py` so the standard-path NDJSON behavior has one authoritative functional suite.

#### Red Phase (Task 2)

- [ ] Add or update failing PMA tests that lock the required standard-path behaviors before implementation:
  - [ ] True incremental visible-text streaming on the capable-model plus MCP path.
  - [ ] Separation of visible text, thinking content, and tool-call content.
  - [ ] `iteration_start` emission before each agent iteration.
  - [ ] `tool_progress` and `tool_result` emission around MCP tool calls.
  - [ ] Per-iteration and per-turn overwrite events.
  - [ ] Opportunistic secret scanning across visible, thinking, and tool-call buffers.
- [ ] Extend the PMA BDD coverage so the updated feature scenarios fail on the current implementation instead of passing through an older direct-stream-only path.
- [ ] Create `scripts/test_scripts/e2e_201_pma_standard_path_streaming.py` as the authoritative Python functional suite for PMA standard-path NDJSON streaming.
- [ ] Add failing Python functional tests for the standard PMA path:
  - [ ] `test_pma_standard_path_ndjson_stream_contains_iteration_and_thinking_events`
  - [ ] `test_pma_standard_path_ndjson_stream_contains_tool_activity_before_done`
  - [ ] `test_pma_standard_path_ndjson_stream_emits_scoped_overwrite_for_controlled_fixture_output`
- [ ] Run the PMA-targeted tests and confirm they fail for the correct reasons.
- [ ] Validation gate: do not proceed until the standard-path streaming gap is proven.

#### Green Phase (Task 2)

- [ ] Implement the streaming LLM wrapper used by the standard langchain path.
  - [ ] Tee each upstream token stream to the outward NDJSON event writer and the internal buffer returned to langchain.
  - [ ] Preserve the current fallback path only for true wrapper failure or explicitly degraded non-streaming paths, not as the default standard behavior.
- [ ] Implement the configurable streaming token state machine.
  - [ ] Route visible text to `delta`.
  - [ ] Route hidden thinking to `thinking`.
  - [ ] Route detected tool-call content to `tool_call`.
  - [ ] Buffer ambiguous partial tags instead of leaking them as visible text whenever possible.
- [ ] Emit `iteration_start` before each langchain iteration and inject `tool_progress` and `tool_result` around MCP tool execution.
- [ ] Implement PMA overwrite events for both scopes:
  - [ ] Per-iteration overwrite for leaked tags and iteration-local secret redaction.
  - [ ] Per-turn overwrite for agent-output correction or cross-iteration redaction.
- [ ] Wrap PMA secret-bearing stream buffer operations with the Task 1 secure-buffer helper.
- [ ] Re-run the PMA-targeted tests until they pass.
- [ ] Validation gate: do not proceed until the standard-path PMA streaming contract is green.

#### Refactor Phase (Task 2)

- [ ] Extract small helpers for the wrapper, state machine, and event injection so the new standard path stays readable.
- [ ] Remove duplicate direct-stream parsing logic that is now subsumed by the shared wrapper path where safe.
- [ ] Re-run the Task 2 targeted tests.
- [ ] Validation gate: do not proceed until the PMA streaming path is stable and readable.

#### Testing Steps (Task 2)

- [ ] Run the targeted PMA unit and behavior tests that cover streaming output, overwrite events, and opportunistic secret handling.
- [ ] Run `just setup-dev restart --force`.
- [ ] Run `just e2e --tags pma_inference` and confirm the new `e2e_201_pma_standard_path_streaming.py` coverage passes against the standard capable-model plus MCP path.
- [ ] Run `just test-bdd` to ensure the PMA feature coverage remains green where step definitions already exist.
- [ ] Run `just lint-go` and `just lint-go-ci` for the changed PMA packages.
- [ ] Validation gate: do not start Task 3 until all Task 2 checks pass.

#### Closeout (Task 2)

- [ ] Generate a task completion report for Task 2:
  - [ ] What changed in the PMA streaming wrapper, event emission, and secret handling.
  - [ ] What PMA tests and lint checks passed.
  - [ ] Any fallback cases intentionally preserved for later tasks.
- [ ] Do not start Task 3 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 3: Implement Gateway Relay, Redaction, and Persistence

Make the user gateway consume the updated PMA stream, emit the correct per-endpoint SSE formats, scan all content types, and persist structured assistant turns.

#### Task 3 Requirements and Specifications

- [docs/requirements/usrgwy.md](../requirements/usrgwy.md) for `REQ-USRGWY-0149` through `REQ-USRGWY-0156`.
- [docs/requirements/client.md](../requirements/client.md) for `REQ-CLIENT-0182`, `REQ-CLIENT-0184`, `REQ-CLIENT-0185`, and `REQ-CLIENT-0215` through `REQ-CLIENT-0220`.
- [docs/requirements/stands.md](../requirements/stands.md) for `REQ-STANDS-0133`.
- [docs/tech_specs/openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md) with focus on `Streaming`, `StreamingRedactionPipeline`, `StreamingPerEndpointSSEFormat`, and `StreamingHeartbeatFallback`.
- [docs/tech_specs/chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md) with focus on structured `parts`.
- [features/orchestrator/openai_compat_chat.feature](../../features/orchestrator/openai_compat_chat.feature).
- [features/e2e/chat_openai_compatible.feature](../../features/e2e/chat_openai_compatible.feature).
- Task 1 and Task 2 artifacts.

#### Discovery Steps (Task 3)

- [ ] Re-read the gateway requirements, SSE format spec, redaction pipeline, heartbeat fallback, and structured turn persistence rules.
- [ ] Inspect the current relay path in `orchestrator/internal/handlers/openai_chat.go`, the PMA client, and the thread persistence and database code.
- [ ] Confirm the current drift that must be eliminated:
  - [ ] `completeViaPMAStream` currently accumulates visible text only and emits chat-completion chunks only.
  - [ ] `/v1/responses` streaming still falls through chat-style handling instead of a native responses event model.
  - [ ] The post-stream relay does not yet maintain separate visible, thinking, and tool-call accumulators.
  - [ ] `emitContentAsSSE` still exists as a fake-streaming fallback.
- [ ] Confirm how streamed `response_id` and `metadata.parts` should be persisted so `previous_response_id` continuation and structured history both remain correct.
- [ ] Map the current API-level Python streaming coverage to the gateway relay gaps so Task 3 updates the right files instead of duplicating basic API smoke coverage that already exists in `e2e_127_sse_streaming.py`.

#### Red Phase (Task 3)

- [ ] Add or update failing gateway tests that lock the required behaviors before implementation:
  - [ ] Native chat-completions SSE mapping.
  - [ ] Native responses SSE mapping.
  - [ ] Stable streamed response id handling for `/v1/responses`.
  - [ ] Relay of `thinking`, `tool_call`, `tool_progress`, `iteration_start`, overwrite, and heartbeat events.
  - [ ] Cancellation behavior and clear terminal transport semantics.
  - [ ] Post-stream amendment ordering before terminal completion.
  - [ ] Persistence of structured assistant-turn parts and redacted content only.
- [ ] Extend database or integration tests so persisted `thinking` and `tool_call` content are validated as structured parts, not flattened into plain-text `content`.
- [ ] Extend `scripts/test_scripts/e2e_127_sse_streaming.py` with failing gateway-level functional coverage:
  - [ ] `test_chat_completions_stream_relays_thinking_tool_and_iteration_events`
  - [ ] `test_responses_stream_relays_native_response_output_text_and_completed_events`
  - [ ] `test_streamed_responses_id_can_be_reused_as_previous_response_id`
- [ ] Create `scripts/test_scripts/e2e_202_gateway_streaming_contract.py` for the gateway behaviors that are too large or too stateful for `e2e_127_sse_streaming.py`.
- [ ] Add failing Python functional tests in `scripts/test_scripts/e2e_202_gateway_streaming_contract.py`:
  - [ ] `test_stream_amendment_arrives_before_terminal_completion`
  - [ ] `test_stream_heartbeat_fallback_emits_progress_then_final_visible_text`
  - [ ] `test_client_disconnect_is_treated_as_stream_cancellation`
  - [ ] `test_streamed_structured_parts_are_persisted_redacted_only`
- [ ] Run the targeted gateway tests and confirm they fail for the correct reasons.
- [ ] Validation gate: do not proceed until the relay, redaction, and persistence gaps are proven.

#### Green Phase (Task 3)

- [ ] Replace the current PMA streaming relay with one that consumes the full Task 1 and Task 2 event model.
- [ ] Emit per-endpoint native SSE formats:
  - [ ] `/v1/chat/completions` text deltas as OpenAI chat-completion chunks plus named `cynodeai.*` events.
  - [ ] `/v1/responses` text and completion events in the native responses event model plus named `cynodeai.*` extensions.
- [ ] Maintain separate visible-text, thinking, and tool-call accumulators in the gateway.
  - [ ] Apply per-iteration and per-turn overwrite events to the correct accumulator scope.
  - [ ] Run the authoritative post-stream secret scan on all three accumulators before emitting the terminal completion event.
- [ ] Persist the final redacted structured assistant turn into thread history, including streamed `response_id` metadata for the responses surface.
- [ ] Remove or bypass `emitContentAsSSE` and replace degraded-mode handling with heartbeat SSE events plus one final visible-text delta when true upstream token streaming is unavailable.
- [ ] Ensure client cancellation or disconnect is treated as cancellation of the streaming request and does not leave upstream work running indefinitely.
- [ ] Wrap gateway secret-bearing accumulator append and replace paths with the Task 1 secure-buffer helper.
- [ ] Re-run the targeted gateway tests until they pass.
- [ ] Validation gate: do not proceed until the gateway relay, persistence, and fallback behavior are green.

#### Refactor Phase (Task 3)

- [ ] Extract small relay and accumulator helpers so chat-completions and responses streaming paths share logic without collapsing their wire formats into one parser.
- [ ] Remove any now-obsolete fake-stream or single-accumulator helper code.
- [ ] Re-run the Task 3 targeted tests.
- [ ] Validation gate: do not proceed until the gateway relay is readable and duplicate-free.

#### Testing Steps (Task 3)

- [ ] Run the targeted gateway handler, integration, and persistence tests.
- [ ] Run `just setup-dev restart --force`.
- [ ] Run `just e2e --tags pma_inference` for the API stream-shape assertions in `e2e_127_sse_streaming.py`.
- [ ] Run `just e2e --tags chat` for the continuation, heartbeat, cancellation, and persistence assertions in `e2e_202_gateway_streaming_contract.py`.
- [ ] Run `just test-bdd` for the orchestrator feature coverage that validates the updated SSE contract.
- [ ] Run `just lint-go` and `just lint-go-ci` for the changed gateway and database packages.
- [ ] Validation gate: do not start Task 4 until all Task 3 checks pass.

#### Closeout (Task 3)

- [ ] Generate a task completion report for Task 3:
  - [ ] What changed in the SSE relay, accumulator logic, heartbeat fallback, and persistence path.
  - [ ] What handler, persistence, and lint checks passed.
  - [ ] Any remaining behavior that still depends on downstream client work.
- [ ] Do not start Task 4 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 4: Align Cynork Streaming Transports and Parsers

Teach `cynork` to understand the new per-endpoint streaming surface instead of a delta-only chat-completions parser.

#### Task 4 Requirements and Specifications

- [docs/requirements/client.md](../requirements/client.md) for `REQ-CLIENT-0182`, `REQ-CLIENT-0185`, `REQ-CLIENT-0186`, `REQ-CLIENT-0209`, and `REQ-CLIENT-0215` through `REQ-CLIENT-0220`.
- [docs/tech_specs/openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md) for per-endpoint SSE behavior.
- [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md) for generation-state and transcript expectations.
- [docs/tech_specs/cli_management_app_commands_chat.md](../tech_specs/cli_management_app_commands_chat.md) for chat-surface continuity.
- [features/e2e/chat_openai_compatible.feature](../../features/e2e/chat_openai_compatible.feature).
- Task 1 through Task 3 artifacts.

#### Discovery Steps (Task 4)

- [ ] Re-read the client, gateway, and TUI specs that define what `cynork` must preserve from the stream.
- [ ] Inspect the current streaming code in `cynork/internal/gateway/client.go`, `cynork/internal/chat/transport.go`, and any consumer code that assumes `delta` plus optional amendment only.
- [ ] Confirm the current drift that must be removed:
  - [ ] `ResponsesStream` currently returns no streamed response id.
  - [ ] `/v1/responses` streaming is still parsed through chat-completion chunk logic.
  - [ ] No transport event surface exists yet for `thinking`, `tool_call`, `tool_progress`, `iteration_start`, or heartbeat.
- [ ] Identify every downstream caller that must be updated once the richer event model is introduced.
- [ ] Use `scripts/test_scripts/e2e_203_cynork_transport_streaming.py` as the deterministic mock-gateway functional harness for `cynork` transport parsing and event propagation.

#### Red Phase (Task 4)

- [ ] Add or update failing `cynork` transport and parser tests that lock the new behaviors before implementation:
  - [ ] Chat-completions SSE parsing for named `cynodeai.*` events.
  - [ ] Responses SSE native event parsing, including streamed response id and completion events.
  - [ ] Transport propagation of thinking, tool-call, tool-progress, iteration-start, overwrite, and heartbeat metadata.
  - [ ] Error and cancellation behavior for both endpoints.
- [ ] Create `scripts/test_scripts/e2e_203_cynork_transport_streaming.py` with a controlled mock-gateway fixture that drives `cynork` through both transport surfaces without depending on live upstream randomness.
- [ ] Add failing Python functional tests in `scripts/test_scripts/e2e_203_cynork_transport_streaming.py`:
  - [ ] `test_cynork_completions_transport_handles_named_extension_events`
  - [ ] `test_cynork_responses_transport_handles_native_responses_events_and_response_id`
  - [ ] `test_cynork_transport_surfaces_heartbeat_and_turn_scoped_amendment_without_parse_errors`
- [ ] Run the targeted client and transport tests and confirm they fail for the expected gaps.
- [ ] Validation gate: do not proceed until the `cynork` transport gap is proven.

#### Green Phase (Task 4)

- [ ] Split the streaming parsers by endpoint so chat-completions and responses each follow their native wire contract.
- [ ] Extend the `cynork` streaming transport event model to carry:
  - [ ] Visible text deltas.
  - [ ] Streamed response id.
  - [ ] Thinking deltas.
  - [ ] Tool-call and tool-progress payloads.
  - [ ] Iteration boundaries.
  - [ ] Heartbeat progress.
  - [ ] Overwrite scope and amendment content.
- [ ] Keep one-shot and plain output paths limited to canonical visible assistant text only.
- [ ] Re-run the targeted client and transport tests until they pass.
- [ ] Validation gate: do not proceed until the richer `cynork` stream contract is green.

#### Refactor Phase (Task 4)

- [ ] Extract small endpoint-specific parse helpers and event builders so the transport layer stays readable.
- [ ] Remove legacy assumptions that every stream can be parsed as chat-completions chunks.
- [ ] Re-run the Task 4 targeted tests.
- [ ] Validation gate: do not proceed until the transport layer is stable and clear.

#### Testing Steps (Task 4)

- [ ] Run the targeted `cynork` gateway-client and transport tests.
- [ ] Run `just setup-dev restart --force` when the mock-gateway transport tests depend on a rebuilt `cynork` binary or updated default config paths.
- [ ] Run `just e2e --tags chat` and confirm the new `e2e_203_cynork_transport_streaming.py` scenarios pass.
- [ ] Run `just lint-go` for the changed `cynork` packages.
- [ ] Run `just test-go-cover` once the transport and parser changes settle so coverage remains above the project floor.
- [ ] Validation gate: do not start Task 5 until all Task 4 checks pass.

#### Closeout (Task 4)

- [ ] Generate a task completion report for Task 4:
  - [ ] What changed in endpoint parsing and transport events.
  - [ ] What `cynork` tests and lint checks passed.
  - [ ] Any downstream UI changes that depend on the new event model.
- [ ] Do not start Task 5 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 5: Wire the TUI Structured Streaming UX

Use the richer event model to make the TUI render one logical in-flight assistant turn, stored thinking and tool content, heartbeat fallback, scoped overwrites, and reconnect state correctly.

#### Task 5 Requirements and Specifications

- [docs/requirements/client.md](../requirements/client.md) for `REQ-CLIENT-0182`, `REQ-CLIENT-0183`, `REQ-CLIENT-0184`, `REQ-CLIENT-0185`, `REQ-CLIENT-0192`, `REQ-CLIENT-0193`, `REQ-CLIENT-0195`, `REQ-CLIENT-0202`, `REQ-CLIENT-0204`, `REQ-CLIENT-0209`, and `REQ-CLIENT-0213` through `REQ-CLIENT-0220`.
- [docs/requirements/stands.md](../requirements/stands.md) for `REQ-STANDS-0133`.
- [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md) with focus on `TranscriptRendering`, `GenerationState`, `ThinkingContentStorageDuringStreaming`, `ToolCallContentStorageDuringStreaming`, `SecureBufferHandlingForInFlightStreamingContent`, and `ConnectionRecovery`.
- [docs/tech_specs/chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md).
- [docs/tech_specs/cli_management_app_commands_chat.md](../tech_specs/cli_management_app_commands_chat.md).
- [features/cynork/cynork_tui.feature](../../features/cynork/cynork_tui.feature).
- [features/cynork/cynork_tui_streaming.feature](../../features/cynork/cynork_tui_streaming.feature).
- [features/cynork/cynork_tui_threads.feature](../../features/cynork/cynork_tui_threads.feature).
- Task 1 through Task 4 artifacts.

#### Discovery Steps (Task 5)

- [ ] Re-read the TUI requirements, transcript spec, streaming feature files, and thread reconnect scenarios.
- [ ] Inspect the current TUI model and state code in `cynork/internal/tui/model.go` and `cynork/internal/tui/state.go`.
- [ ] Confirm the current drift that must be closed:
  - [ ] Flat string scrollback is still the effective streaming source of truth.
  - [ ] `TranscriptTurn` and `TranscriptPart` are not yet the canonical in-memory model.
  - [ ] The current amendment path replaces the whole buffer only and does not handle iteration-scoped overwrite tracking.
  - [ ] There is no heartbeat-rendering or reconnect-state flow.
  - [ ] Thinking persistence exists only as a user preference toggle, not as stored per-turn streaming content.
- [ ] Confirm the exact current behavior that must be preserved, including `show thinking` persistence, session commands, thread continuity, and existing login recovery behavior.
- [ ] Keep cancel and reconnect PTY assertions in `scripts/test_scripts/e2e_198_tui_pty.py`, keep slash-toggle assertions in `scripts/test_scripts/e2e_199_tui_slash_commands.py`, and place structured in-flight transcript assertions in `scripts/test_scripts/e2e_204_tui_streaming_behavior.py`.

#### Red Phase (Task 5)

- [ ] Add or update failing TUI model tests that lock the updated streaming UX before implementation:
  - [ ] Exactly one in-flight assistant turn is updated in place during streaming.
  - [ ] Hidden-by-default thinking placeholders stay visible and expand instantly when enabled.
  - [ ] Tool-call and tool-result placeholders or expanded blocks appear as distinct non-prose items.
  - [ ] Per-iteration overwrite replaces only the targeted segment.
  - [ ] Per-turn overwrite replaces the entire visible in-flight text.
  - [ ] Heartbeat events render a progress indicator and do not pollute transcript content.
  - [ ] Cancellation and reconnect retain already-received content and reconcile the active turn deterministically.
- [ ] Add or update failing TUI history-loading tests so persisted structured parts rehydrate correctly after reload.
- [ ] Update `scripts/test_scripts/e2e_198_tui_pty.py` with failing PTY functional tests:
  - [ ] `test_tui_ctrl_c_cancels_stream_and_retains_partial_text`
  - [ ] `test_tui_reconnect_preserves_partial_text_and_marks_turn_interrupted`
- [ ] Update `scripts/test_scripts/e2e_199_tui_slash_commands.py` with failing toggle tests:
  - [ ] `test_tui_slash_show_thinking_reveals_stored_reasoning_without_refetch`
  - [ ] `test_tui_slash_show_tool_output_reveals_stored_tool_content_without_refetch`
  - [ ] `test_tui_slash_hide_tool_output_recovers_collapsed_tool_placeholders`
- [ ] Create `scripts/test_scripts/e2e_204_tui_streaming_behavior.py` for the larger structured-streaming transcript assertions:
  - [ ] `test_tui_updates_single_inflight_turn_progressively`
  - [ ] `test_tui_iteration_scoped_overwrite_only_replaces_target_segment`
  - [ ] `test_tui_turn_scoped_amendment_replaces_visible_text_without_duplication`
  - [ ] `test_tui_heartbeat_progress_indicator_disappears_after_final_content`
- [ ] Run the targeted TUI tests and confirm they fail for the right reasons.
- [ ] Validation gate: do not proceed until the TUI streaming UX gap is proven.

#### Green Phase (Task 5)

- [ ] Promote `TranscriptTurn`, `TranscriptPart`, and `SessionState` to the canonical in-memory streaming representation for the TUI.
- [ ] Render exactly one logical assistant turn per user prompt, updating that turn in place while streaming.
- [ ] Store and render structured streaming content correctly:
  - [ ] Visible assistant text in order.
  - [ ] Hidden-by-default thinking content with immediate reveal when enabled.
  - [ ] Hidden-by-default or expanded tool-call and tool-result content as non-prose items.
  - [ ] Heartbeat progress as display-only status attached to the in-flight turn.
- [ ] Implement both overwrite scopes:
  - [ ] Per-iteration overwrite replaces only the targeted iteration segment.
  - [ ] Per-turn overwrite replaces the whole visible in-flight content.
- [ ] Implement bounded-backoff reconnect behavior and interrupted-turn reconciliation.
- [ ] Extend local TUI preference handling for `/show-tool-output` and `/hide-tool-output` using persisted config key `tui.show_tool_output_by_default`.
- [ ] Wrap TUI secret-bearing stream-buffer append and replace paths with the Task 1 secure-buffer helper.
- [ ] Re-run the targeted TUI tests until they pass.
- [ ] Validation gate: do not proceed until the TUI streaming UX is green.

#### Refactor Phase (Task 5)

- [ ] Extract small transcript-building, overwrite-handling, and status-rendering helpers so the model update loop does not become a monolith.
- [ ] Remove obsolete string-only stream bookkeeping once the structured-turn path is fully in place.
- [ ] Re-run the Task 5 targeted tests.
- [ ] Validation gate: do not proceed until the TUI update and render flow is stable and readable.

#### Testing Steps (Task 5)

- [ ] Run the targeted TUI unit tests for streaming, transcript rendering, and reconnect behavior.
- [ ] Run `just setup-dev restart --force`.
- [ ] Run `just e2e --tags tui_pty` and confirm the updated `e2e_198_tui_pty.py`, `e2e_199_tui_slash_commands.py`, and `e2e_204_tui_streaming_behavior.py` scenarios pass.
- [ ] Run `just lint-go` for the changed `cynork/internal/tui` and adjacent packages.
- [ ] Run `just test-go-cover` once the TUI changes settle so coverage remains above the floor.
- [ ] Run `just test-bdd` to confirm no existing TUI scenarios regressed before the dedicated streaming steps are finished in Task 6.
- [ ] Validation gate: do not start Task 6 until all Task 5 checks pass.

#### Closeout (Task 5)

- [ ] Generate a task completion report for Task 5:
  - [ ] What changed in transcript state, rendering, and reconnect behavior.
  - [ ] What TUI tests and lint checks passed.
  - [ ] Any UI affordances or toggle behavior that still need feature or E2E coverage.
- [ ] Do not start Task 6 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 6: Finish BDD and E2E Streaming Coverage

Remove the remaining pending streaming steps and make the streaming behavior executable and verifiable end to end.

#### Task 6 Requirements and Specifications

- [features/cynork/cynork_tui.feature](../../features/cynork/cynork_tui.feature).
- [features/cynork/cynork_tui_streaming.feature](../../features/cynork/cynork_tui_streaming.feature).
- [features/cynork/cynork_tui_threads.feature](../../features/cynork/cynork_tui_threads.feature).
- [features/orchestrator/openai_compat_chat.feature](../../features/orchestrator/openai_compat_chat.feature).
- [features/agents/pma_chat_and_context.feature](../../features/agents/pma_chat_and_context.feature).
- [features/e2e/chat_openai_compatible.feature](../../features/e2e/chat_openai_compatible.feature).
- Task 1 through Task 5 artifacts.

#### Discovery Steps (Task 6)

- [ ] Re-read the full streaming feature set and map every scenario to the concrete code and test harness surface that now exists after Tasks 1 through 5.
- [ ] Inspect the current pending steps in `cynork/_bdd/steps2.go` and the current streaming E2E coverage in `scripts/test_scripts/e2e_127_sse_streaming.py` and `scripts/test_scripts/e2e_198_tui_pty.py`.
- [ ] Confirm which scenarios need mock-gateway support, which need PTY orchestration, and which need live-stack E2E coverage.
- [ ] Review the complete Python streaming test inventory (`e2e_127`, `e2e_198`, `e2e_199`, `e2e_201`, `e2e_202`, `e2e_203`, and `e2e_204`) and remove any overlap that no longer adds distinct requirement coverage.

#### Red Phase (Task 6)

- [ ] Replace the remaining pending streaming BDD placeholders with failing step implementations or failing assertions rather than `godog.ErrPending`.
- [ ] Add or update failing streaming E2E tests that lock the updated behavior end to end:
  - [ ] Chat-completions and responses native streaming event coverage.
  - [ ] Streamed response id coverage for `/v1/responses`.
  - [ ] Post-stream amendment before terminal completion.
  - [ ] Heartbeat fallback.
  - [ ] Client cancel or disconnect behavior.
  - [ ] TUI progressive in-flight turn updates.
  - [ ] Stored thinking and stored tool-output toggle behavior.
  - [ ] Reconnect and interrupted-turn reconciliation.
- [ ] Finish the Python functional test matrix with specific file ownership:
  - [ ] `scripts/test_scripts/e2e_127_sse_streaming.py` owns chat-completions and responses API event-shape assertions.
  - [ ] `scripts/test_scripts/e2e_201_pma_standard_path_streaming.py` owns direct PMA standard-path NDJSON assertions.
  - [ ] `scripts/test_scripts/e2e_202_gateway_streaming_contract.py` owns gateway amendment, heartbeat, cancellation, persistence, and continuation assertions.
  - [ ] `scripts/test_scripts/e2e_203_cynork_transport_streaming.py` owns `cynork` transport parsing assertions against controlled mock streams.
  - [ ] `scripts/test_scripts/e2e_204_tui_streaming_behavior.py` owns structured TUI transcript behavior assertions.
  - [ ] `scripts/test_scripts/e2e_198_tui_pty.py` and `scripts/test_scripts/e2e_199_tui_slash_commands.py` keep the PTY cancel, reconnect, and toggle assertions that belong with the long-lived TUI harness.
- [ ] Run the targeted BDD and E2E tests and confirm they fail for the correct reasons.
- [ ] Validation gate: do not proceed until the executable streaming coverage gap is proven.

#### Green Phase (Task 6)

- [ ] Implement the missing Godog step definitions and mock-gateway support so the updated streaming feature files execute instead of remaining pending.
- [ ] Extend the designated existing E2E scripts and create the dedicated `e2e_201` through `e2e_204` streaming files defined in this plan.
- [ ] Keep the BDD and E2E assertions aligned with the actual updated canonical docs instead of the older deferred streaming behavior.
- [ ] Re-run the targeted BDD and E2E suites until they pass.
- [ ] Validation gate: do not proceed until the streaming behavior is executable across BDD and E2E.

#### Refactor Phase (Task 6)

- [ ] Extract reusable test harness helpers for SSE parsing, PTY observation, reconnect orchestration, and amendment verification so streaming assertions do not fragment across files.
- [ ] Remove obsolete streaming-test assumptions that only inspect raw visible text and `[DONE]`.
- [ ] Re-run the Task 6 targeted suites.
- [ ] Validation gate: do not proceed until the streaming test harness is stable and maintainable.

#### Testing Steps (Task 6)

- [ ] Run `just test-bdd`.
- [ ] Run `just setup-dev restart --force`.
- [ ] Run `just e2e --tags pma_inference`.
- [ ] Run `just e2e --tags chat`.
- [ ] Run `just e2e --tags tui_pty`.
- [ ] If the new streaming coverage is spread across multiple tags, run each affected tag subset before moving on.
- [ ] Validation gate: do not start Task 7 until all Task 6 checks pass.

#### Closeout (Task 6)

- [ ] Generate a task completion report for Task 6:
  - [ ] Which streaming scenarios are now executable.
  - [ ] Which BDD and E2E suites passed.
  - [ ] Any environment-specific skips or known infrastructure limits that must be documented.
- [ ] Do not start Task 7 until this closeout is done.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)

---

### Task 7: Documentation and Closeout

Close the work with validation, user-facing help updates, and any approved doc follow-up that the implementation proves necessary.

#### Task 7 Requirements and Specifications

- [docs/requirements/client.md](../requirements/client.md).
- [docs/requirements/usrgwy.md](../requirements/usrgwy.md).
- [docs/requirements/pmagnt.md](../requirements/pmagnt.md).
- [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md).
- [docs/tech_specs/openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md).
- [docs/tech_specs/cynode_pma.md](../tech_specs/cynode_pma.md).
- [docs/tech_specs/chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md).
- Task 1 through Task 6 artifacts.

#### Discovery Steps (Task 7)

- [ ] Review which user-visible help text, CLI help surfaces, or developer docs must be updated because the implementation changed behavior that the current docs already define.
- [ ] Identify any true canonical doc gaps that the implementation could not resolve from the current requirements and specs.
- [ ] Record any canonical doc follow-up that still needs explicit user direction, but do not edit canonical requirements or tech specs as part of this plan.
- [ ] Review every streaming Python functional file touched in Tasks 1 through 6 and verify that its trace comments, tags, and scenario names still match the final implemented contract and the canonical requirement or spec IDs.

#### Red Phase (Task 7)

- [ ] N/A for the closeout task.
- [ ] Validation gate: skip to Green only after recording any canonical-doc follow-up and confirming whether doc edits are in scope.

#### Green Phase (Task 7)

- [ ] Update any approved user-facing or developer-facing documentation that must reflect the implemented streaming behavior.
- [ ] Update implementation-level help or examples if new streaming controls or toggle behavior were introduced.
- [ ] Add a final implementation note that records any intentionally preserved fallback behavior or explicit non-goals.
- [ ] Update the streaming Python functional test files so their class docstrings, test docstrings, trace comments, and tag sets are final and non-stale:
  - [ ] `scripts/test_scripts/e2e_127_sse_streaming.py`
  - [ ] `scripts/test_scripts/e2e_201_pma_standard_path_streaming.py`
  - [ ] `scripts/test_scripts/e2e_202_gateway_streaming_contract.py`
  - [ ] `scripts/test_scripts/e2e_203_cynork_transport_streaming.py`
  - [ ] `scripts/test_scripts/e2e_204_tui_streaming_behavior.py`
  - [ ] `scripts/test_scripts/e2e_198_tui_pty.py`
  - [ ] `scripts/test_scripts/e2e_199_tui_slash_commands.py`

#### Refactor Phase (Task 7)

- [ ] Remove stale comments, temporary notes, or outdated references to deferred streaming work that no longer apply.
- [ ] Re-run the final doc and lint checks for changed docs and help text.

#### Testing Steps (Task 7)

- [ ] Run `just lint`.
- [ ] Run `just test-go-cover`.
- [ ] Run `just test-bdd`.
- [ ] Run `just setup-dev restart --force`.
- [ ] Run `just e2e --tags pma_inference`.
- [ ] Run `just e2e --tags chat`.
- [ ] Run `just e2e --tags tui_pty`.
- [ ] Run `just e2e`.
- [ ] Run `just docs-check`.
- [ ] Run `just ci`.
- [ ] Validation gate: do not mark the streaming implementation complete until the full validation set above passes.

#### Closeout (Task 7)

- [ ] Generate a final plan completion report:
  - [ ] Which tasks were completed.
  - [ ] Which repo validation commands passed.
  - [ ] Any explicit remaining risks, environment caveats, or follow-up items.
- [ ] Verify that no streaming BDD steps remain pending and that no gateway fake-stream fallback remains on the standard path.
- [ ] Mark every completed step in this task's section of the plan with `- [x]`. (Last step.)
