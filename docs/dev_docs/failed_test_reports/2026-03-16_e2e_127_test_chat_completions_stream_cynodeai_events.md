# Failed E2E Report: e2e_0610_sse_streaming.test_chat_completions_stream_exposes_named_cynodeai_extension_events

## 1 Summary

Test `e2e_0610_sse_streaming.TestSSEStreaming.test_chat_completions_stream_exposes_named_cynodeai_extension_events` failed because the parsed SSE stream had no events with a named `cynodeai.*` event type (e.g. cynodeai.heartbeat, cynodeai.thinking_delta); the event list was `[None, None]`.
The test uses `helpers.parse_sse_stream_typed()` to get event names and filters for those starting with `cynodeai.`; it asserts at least one such event exists.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: 0 not greater than 0 : Stream must expose at least one named cynodeai.* extension event (e.g. cynodeai.heartbeat or cynodeai.thinking_delta); got events: [None, None]`
- **Root cause:** The SSE stream either (1) did not include `event:` lines with cynodeai.*
  names, or (2) the parser did not associate event names with data lines, so `e.get("event")` was None for all entries.
- **Effect:** `cynodeai_events` was empty; the assertion at line 301 failed.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0610_sse_streaming.py](../../../scripts/test_scripts/e2e_0610_sse_streaming.py) lines 269-304: POST `/v1/chat/completions` stream=true, calls `helpers.parse_sse_stream_typed(resp)`, filters typed_events for `event.startswith("cynodeai.")`, asserts len(cynodeai_events) > 0.
- [helpers.py](../../../scripts/test_scripts/helpers.py): `parse_sse_stream_typed` returns list of dicts with "event" and "data"; if event lines are missing or not parsed, event will be None.

### 3.2 Gateway and Streaming

- User API Gateway: MUST emit named SSE event types per CYNAI.USRGWY.OpenAIChatApi.StreamingPerEndpointSSEFormat (e.g. cynodeai.heartbeat, cynodeai.thinking_delta).
- PMA streaming relay: gateway relays or generates these event names; if relay does not add event names or upstream does not send them, the stream will have no cynodeai.*
  events.

### 3.3 Backend Path

- Streaming implementation plan (Task 3): relay thinking/tool and iteration events; named event format may not be implemented or may be omitted when stream ends early (e.g. timeout).

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-USRGWY-0149](../../requirements/usrgwy.md), [REQ-USRGWY-0150](../../requirements/usrgwy.md): Streaming and event shape.

### 4.2 Tech Specs

- [openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md): CYNAI.USRGWY.OpenAIChatApi.StreamingPerEndpointSSEFormat; named cynodeai.*
  events.
- [2026-03-15_streaming_specs_implementation_plan.md](../2026-03-15_streaming_specs_implementation_plan.md): Task 3 relay of thinking/tool and iteration events.

### 4.3 Feature Files

- Streaming E2E coverage in e2e_0610 and related tags.

## 5 Implementation Deviation

- **Spec/requirement intent:** Stream MUST expose at least one named cynodeai.*
  extension event (e.g. cynodeai.heartbeat or cynodeai.thinking_delta).
- **Observed behavior:** Parsed events had event names [None, None]; no cynodeai.*
  event type was present.
- **Deviation:** Gateway or relay does not emit SSE `event:` lines with cynodeai.*
  names, or the stream was truncated (e.g. timeout) before any such event; implementation may not yet implement the named-event format per spec.

## 6 What Needs to Be Fixed in the Implementation

The following describes the exact implementation behavior and required changes.

### 6.1 Root Cause (Implemented Behavior)

- The gateway **does** emit at least one named cynodeai event: **cynodeai.iteration_start** is implemented in [orchestrator/internal/handlers/openai_chat.go](../../../orchestrator/internal/handlers/openai_chat.go) in `completeViaPMAStream` via `onIterationStart` -> `writeSSENamedEvent(w, userapi.SSEEventIterationStart, ...)` (lines 365-371).
  The stream is driven by [orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go) `CallChatCompletionStreamWithCallbacks` with **120s** HTTP client timeout.
  If the PMA does not send an iteration (or any token) within 120s, the client returns an error and the gateway writes only `stream_error`; the test never sees a named cynodeai.*
  event.

### 6.2 What is Not Implemented or is Misconfigured

- **Timeout:** The stream is cut by the 120s pmaclient timeout before the PMA sends iteration_start (or the stream never reaches that point).
  **Optional:** Spec may require additional events (e.g. cynodeai.heartbeat, cynodeai.thinking_delta); if so, ensure the relay emits them when the PMA sends corresponding data.

### 6.3 Exact Code or Config Changes

- Same as [2026-03-16_e2e_127_test_chat_completions_stream_returns_sse.md](2026-03-16_e2e_127_test_chat_completions_stream_returns_sse.md) section 6: increase the PMA client timeout (e.g. to 200s) when used for streaming from the gateway so the stream can progress to iteration_start and content.
  If the spec requires heartbeat or other named events, add their emission in the gateway relay when PMA provides the data.
