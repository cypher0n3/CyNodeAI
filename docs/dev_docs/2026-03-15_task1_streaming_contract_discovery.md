# Task 1: Streaming Contract Discovery Report

- [Wire-Level Contract](#wire-level-contract-implementation-must-satisfy)
- [Current Gap List](#current-gap-list-code-and-tests)
- [RunWithSecret Location](#runwithsecret-location-and-status)
- [Python Streaming Coverage Map](#python-streaming-coverage-map-existing)

## Wire-Level Contract (Implementation Must Satisfy)

Contract that implementations must satisfy.

### Chat-Completions SSE (`POST /v1/chat/completions` Stream=true)

- Unnamed `data:` lines: OpenAI chat.completion.chunk with `choices[0].delta.content`.
- Named events (CyNodeAI extensions): `event: cynodeai.thinking_delta`, `event: cynodeai.iteration_start`, `event: cynodeai.tool_call`, `event: cynodeai.tool_progress`, `event: cynodeai.amendment`, `event: cynodeai.heartbeat`.
- Terminal: `data: [DONE]`.
- Amendment payload: `{"type":"secret_redaction"|"overwrite","content":"…","redaction_kinds":["api_key"]}` (optional), `scope`/`iteration` for overwrite.

### Responses SSE (`POST /v1/responses` Stream=true)

- Native responses events: `response.output_text.delta`, `response.completed`; streamed `response_id` in event or header.
- Same CyNodeAI named events as above.
- Terminal: `response.completed` (not `[DONE]` for responses native format).

### Overwrite Metadata

- Per-iteration: `scope: "iteration"`, `iteration: N`.
- Per-turn: scope covers entire visible turn.

### Secure-Buffer Helper

- REQ-STANDS-0133 / REQ-STANDS-0135: `go_shared_libs/secretutil.RunWithSecret` is the shared helper; all PMA, gateway, and TUI secret-bearing append/replace paths must use it.

### Tool-Output Control Surface (Locked for Workstream)

- `/show-tool-output`, `/hide-tool-output`, persisted config key `tui.show_tool_output_by_default` (mirror thinking visibility).

## Current Gap List (Code and Tests)

1. **PMA standard-path streaming:** Still blocking on capable-model + MCP path; only direct-inference path streams. `pmaclient.CallChatCompletionStream` only accepts `onDelta func(string)`; no events for thinking, tool_call, iteration_start.
2. **Gateway:** `completeViaPMAStream` accumulates visible text only, emits only chat.completion.chunk deltas and `[DONE]`.
   No relay of thinking, tool_call, tool_progress, iteration_start, overwrite, or heartbeat. `emitContentAsSSE` still used for degraded path (fake chunking).
3. **Cynork:** `ChatStream` / `ResponsesStream` parse only `data:` lines; no handling of named `event:` lines. `ResponsesStream` returns response_id but it is from final JSON, not streamed.
   No transport surface for thinking, tool_call, tool_progress, iteration_start, heartbeat.
4. **Shared contract:** `go_shared_libs/contracts/userapi` has `ChatCompletionChunk` and related; no types for `cynodeai.thinking_delta`, `cynodeai.tool_call`, `cynodeai.tool_progress`, `cynodeai.iteration_start`, `cynodeai.amendment`, `cynodeai.heartbeat`, or responses native event shapes.
5. **E2E:** `e2e_127_sse_streaming.py` parses only `data:` lines; does not capture `event:` lines or assert on named extension events or response_id in stream.

## RunWithSecret Location and Status

- **Location:** `go_shared_libs/secretutil` (`RunWithSecret` in secret_goexperiment.go / secret_fallback.go).
- **Status:** Present; build-tagged for `goexperiment.runtimesecret`.
  No extraction needed; Tasks 2, 3, 5 must use it for secret-bearing buffers.

## Python Streaming Coverage Map (Existing)

- `e2e_127_sse_streaming.py`: chat completions and responses SSE; validates `data:` chunks, `[DONE]`, no `<think>` in content; does not validate event names or response_id.
- `e2e_198_tui_pty.py`: TUI PTY (cancel, reconnect).
- `e2e_199_tui_slash_commands.py`: slash commands.

Task 1 extends `e2e_127` with typed SSE parsing and two new tests that will fail until gateway emits named events and streamed response_id (Task 3).

## Task 1 Red Phase Deliverables

- Failing unit/contract tests locking shared event types and payload shapes.
- `e2e_127_sse_streaming.py`: SSE parser captures `event:` lines and preserves order; two new failing tests: `test_chat_completions_stream_exposes_named_cynodeai_extension_events`, `test_responses_stream_uses_native_responses_events_and_exposes_streamed_response_id`.
- Shared mock/fixture placeholders for later tasks.
