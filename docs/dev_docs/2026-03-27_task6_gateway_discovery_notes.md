# Task 6 Discovery: Gateway Relay (2026-03-27)

## Spec Anchors

- `openai_compatible_chat_api.md`: Streaming Heartbeat Fallback removes fake `emitContentAsSSE` chunking; use `cynodeai.heartbeat` then a single per-endpoint visible delta; `/v1/responses` uses native SSE (`response.output_text.delta`, `response.completed`).
- `chat_threads_and_messages.md`: Assistant rows may carry `metadata.parts` (`text`, `thinking`, `tool_call`); canonical `content` is visible text only.

## Code State (Pre-Change)

- `completeViaPMAStream` appended only visible deltas to `fullContent`; thinking/tool were relayed on the wire but not accumulated for persistence.
- `emitContentAsSSE` split full text into 48-rune chunks (degraded chat and `/v1/responses` blocking paths) - violates Streaming Heartbeat Fallback.
- `/v1/responses` PMA stream reused `chat.completion.chunk` for deltas; cynork `ResponsesStream` parsed those chunks.
- `e2e_0630_gateway_streaming_contract.py` expects heartbeat on degraded path and amendment shape when present; several cases skip when events are absent.

## `e2e_0630`

- Tests amendment ordering, heartbeat + final text, disconnect tolerance, and amendment payload `redacted` field when amendments exist.

## Planned Implementation Summary

- Replace `emitContentAsSSE` with heartbeat + single delta + terminal events; native responses SSE on `/v1/responses` streams.
- Accumulate visible, thinking, and tool-call strings during PMA relay; redact before `AppendChatMessage`; merge `metadata.parts` with existing `response_id` JSON.
- Extend `pmaclient` with optional `OnAmendment` for NDJSON `amendment` lines; relay `cynodeai.amendment` and adjust visible accumulator for `secret_redaction`.
- Update cynork `processChatSSEDataLine` to consume `response.output_text.delta` and ignore `response.completed` until `[DONE]`.
- Use `go_shared_libs/secretutil` when appending thinking/tool accumulators.
