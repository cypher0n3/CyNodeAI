# Task 6 Completion Report: Gateway Relay (2026-03-27)

## Summary

The user-gateway OpenAI-compatible streaming path now matches `CYNAI.USRGWY.StreamingHeartbeatFallback` and `StreamingPerEndpointSSEFormat`: fake `emitContentAsSSE` chunking is removed, degraded paths emit `cynodeai.heartbeat` plus a single visible delta, `/v1/responses` streams use `response.output_text.delta` and `response.completed`, PMA relay accumulates visible, thinking, and tool-call text for redaction and `metadata.parts` persistence, and PMA `amendment` NDJSON is parsed and relayed.

## Code Changes

- **`orchestrator/internal/handlers/openai_chat.go`:** `completeViaPMAStream` uses separate accumulators, `mergeAssistantStreamMetadata`, `redactStringContent`, `secretutil.RunWithSecret` on thinking buffer writes, `OnAmendment` relay plus `secret_redaction` visible reset, responses-mode SSE, `emitDegradedStreamingFallback`; removed `emitContentAsSSE`.
- **`orchestrator/internal/handlers/openai_chat_threads.go`:** Degraded `/v1/responses` streaming calls `emitDegradedStreamingFallback` with responses mode.
- **`orchestrator/internal/pmaclient`:** `PMAAmendment`, `OnAmendment`, NDJSON `amendment` parsing.
- **`go_shared_libs/contracts/userapi`:** `SSEEventResponseOutputTextDelta`, `SSEEventResponseCompleted`, `Redacted` on `SSEAmendmentPayload` for E2E contract.
- **`cynork/internal/gateway/client.go`:** Parse `response.output_text.delta` and ignore `response.completed` until `[DONE]`.
- **`cynork/internal/gateway/client_methods_extended_test.go`:** Responses stream mocks use native events.

## Tests

- Go: `orchestrator/internal/handlers` (degraded fallback, PMA stream responses mode, structured parts, amendment), `orchestrator/internal/pmaclient` (amendment line), `go_shared_libs/contracts/userapi`, `cynork/internal/gateway`.
- BDD: `go test ./orchestrator/_bdd` (including gateway streaming scenarios in `features/orchestrator/openai_compat_chat.feature`).
- Python: `just e2e --tags chat` completed OK (16 tests, 3 skips in `e2e_0630` when live stack does not emit amendment/heartbeat on the PMA streaming path).

## Gates Run

- `just test-go-cover`, `just lint-go`, `just test-bdd`, `just e2e --tags chat`.

## References

- [2026-03-27_task6_gateway_discovery_notes.md](2026-03-27_task6_gateway_discovery_notes.md)
- [2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md](2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md) (Task 6 checkboxes updated).
