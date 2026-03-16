# Failed E2E Report: e2e_127_sse_streaming.test_chat_completions_stream_relays_thinking_tool_and_iteration_events

## 1 Summary

Test `e2e_127_sse_streaming.TestSSEStreaming.test_chat_completions_stream_relays_thinking_tool_and_iteration_events` failed because the stream had zero events named `cynodeai.iteration_start`.
The test parses the SSE stream for event names and asserts at least one event is `cynodeai.iteration_start`; it got an empty list for iteration_starts.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: 0 not greater than 0 : Stream must relay at least one cynodeai.iteration_start; got events: []`
- **Root cause:** The stream either did not include any `event: cynodeai.iteration_start` lines, or the parsed event names were empty (e.g. event field None for all), so `event_names` had no iteration_start entries.
- **Effect:** `iteration_starts` was empty; the assertion at line 334 failed.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_127_sse_streaming.py](../../../scripts/test_scripts/e2e_127_sse_streaming.py) lines 305-334: POST `/v1/chat/completions` stream=true, parse_sse_stream_typed, build event_names, filter for `cynodeai.iteration_start`, assert len(iteration_starts) > 0.

### 3.2 Gateway and PMA Relay

- User API Gateway: MUST relay iteration_start (done) and thinking/tool events from PMA as cynodeai.thinking_delta / cynodeai.tool_* / cynodeai.iteration_start.
- PMA streaming: emits iteration and thinking/tool events; gateway relay (Task 3) must forward them with correct event names.

### 3.3 Backend Path

- [2026-03-15_streaming_specs_implementation_plan.md](../2026-03-15_streaming_specs_implementation_plan.md): Task 3 Red: "fails until relay implemented"; iteration_start relay may not be implemented.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-USRGWY-0149](../../requirements/usrgwy.md), [REQ-USRGWY-0150](../../requirements/usrgwy.md): Streaming and event relay.

### 4.2 Tech Specs

- [openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md): Streaming event types and relay.
- [2026-03-15_streaming_specs_implementation_plan.md](../2026-03-15_streaming_specs_implementation_plan.md): Task 3 relay of thinking/tool and iteration_start.

### 4.3 Feature Files

- Streaming E2E in e2e_127.

## 5 Implementation Deviation

- **Spec/requirement intent:** Stream MUST relay at least one cynodeai.iteration_start when PMA sends iteration/thinking/tool events.
- **Observed behavior:** No cynodeai.iteration_start events in the stream; event_names was empty or contained no iteration_start.
- **Deviation:** Gateway relay for iteration_start (and possibly thinking_delta/tool_*) is not implemented or not emitting named event lines; test docstring notes "Task 3 Red: fails until relay implemented".

## 6 What Needs to Be Fixed in the Implementation

The following describes the relay implementation and timeout cause.

### 6.1 Root Cause (Implemented Behavior)

- **Relay is implemented.**
  In [orchestrator/internal/handlers/openai_chat.go](../../../orchestrator/internal/handlers/openai_chat.go), `completeViaPMAStream` registers `onIterationStart` and calls `writeSSENamedEvent(w, userapi.SSEEventIterationStart, string(b))` for each iteration from the PMA (lines 365-371).
- The PMA client in [orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go) uses **120s** timeout.
- If the stream times out before the PMA sends an iteration (or before any delta arrives), the gateway never writes iteration_start and instead writes stream_error.

### 6.2 What is Not Implemented or is Misconfigured

- **Timeout:** The test fails because the stream does not last long enough to receive iteration data from the PMA; the 120s pmaclient timeout ends the stream first.
  thinking_delta/tool_* relay may or may not be implemented depending on whether the PMA sends those and the gateway maps them to named events; the main fix is to allow the stream to run long enough.

### 6.3 Exact Code or Config Changes

- Increase the HTTP client timeout for `CallChatCompletionStreamWithCallbacks` when called from the gateway (e.g. 200s) so the PMA stream can deliver at least one iteration and the gateway can emit cynodeai.iteration_start.
  If the PMA sends thinking_delta or tool_* and the spec requires them as named events, ensure the pmaclient NDJSON parser invokes the corresponding callbacks and the handler emits the matching SSE event types.
