# Task 1: Streaming Contract Closeout Report

- [Contract Types, Helpers, and Fixtures](#contract-types-helpers-and-fixtures)
- [Targeted Tests and Lint](#targeted-tests-and-lint)
- [Validation](#validation-completed)

## Contract Types, Helpers, and Fixtures

**Date:** 2026-03-15  
**Plan:** [2026-03-15_streaming_specs_implementation_plan.md](2026-03-15_streaming_specs_implementation_plan.md) Task 1.

- **go_shared_libs/contracts/userapi:** Added SSE event name constants (`SSEEventThinkingDelta`, `SSEEventToolCall`, `SSEEventToolProgress`, `SSEEventIterationStart`, `SSEEventAmendment`, `SSEEventHeartbeat`) and payload types (`SSEThinkingDeltaPayload`, `SSEToolCallPayload`, `SSEToolProgressPayload`, `SSEIterationStartPayload`, `SSEAmendmentPayload`, `SSEHeartbeatPayload`) per StreamingPerEndpointSSEFormat.
- **Secure-buffer helper:** No change; `go_shared_libs/secretutil.RunWithSecret` remains the shared helper for Tasks 2, 3, 5.
- **Fixtures:** Shared mock PMA/gateway fixtures deferred to later tasks; contract is locked via userapi types and E2E assertions.

## Targeted Tests and Lint

- **Go:** `go_shared_libs/contracts/userapi`: `TestSSEEventConstantsAndPayloads` passes (event names and payload JSON roundtrip).
- **Python:** `e2e_127_sse_streaming.py`: Added `_parse_sse_stream_typed()` (captures `event:` and `data:` with order).
  Added `test_chat_completions_stream_exposes_named_cynodeai_extension_events` and `test_responses_stream_uses_native_responses_events_and_exposes_streamed_response_id`.
  These tests fail when run against the current gateway (no named cynodeai.* events, no streamed response_id) until Task 3 implements the relay.
- **Lint:** `just lint-go` for go_shared_libs not re-run in this session; `read_lints` reported no issues.

## Remaining Non-Blocking Follow-Ups

- Shared mock gateway fixture (for cynork transport tests) can be added in Task 4 or Task 6.

## Validation (Completed)

- Created `.env.dev` with `CYNODE_SECURE_STORE_MASTER_KEY_B64` so the node can start PMA.
- Ran `SETUP_DEV_OLLAMA_IN_STACK=1 just setup-dev start`; stack reached "Orchestrator is ready."
- Ran the two new contract tests in isolation (auth via shared config, then unittest): both failed as expected:
  - `test_chat_completions_stream_exposes_named_cynodeai_extension_events`: stream did not end with `[DONE]` (contract not met).
  - `test_responses_stream_uses_native_responses_events_and_exposes_streamed_response_id`: gateway returned 404 for `/v1/responses` (endpoint or contract not in place).
- Task 1 validation gate satisfied: failing tests prove the shared contract gap before Task 2.
