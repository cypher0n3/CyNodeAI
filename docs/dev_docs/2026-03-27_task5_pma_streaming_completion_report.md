# Task 5 Completion Report: PMA Streaming State Machine (2026-03-27)

## Summary

Task 5 delivered the PMA streaming token classifier, NDJSON emission for `delta`, `thinking`, and `tool_call`, secure-buffer accumulation for inference streams, orchestrator relay of `tool_call` NDJSON, and tests to keep `agents/internal/pma` and `orchestrator/internal/pmaclient` at or above coverage thresholds.

## Implementation

- **`agents/internal/pma/streaming_fsm.go`:** Incremental classifier for think and tool-call tags; stray close-tag stripping; `iterationOverwriteReplace` / `turnOverwriteReplace`.
- **`agents/internal/pma/chat.go`:** `emitNDJSONEmissions`, Ollama NDJSON path with classifier and `done: true`; `appendStreamBufferSecure` for stream accumulation.
- **`agents/internal/pma/langchain.go` / `streaming.go`:** Classifier wired into langchain and streaming LLM callbacks.
- **`orchestrator/internal/pmaclient`:** `OnToolCall` and `processNDJSONLine` parsing for `tool_call` objects.
- **`orchestrator/internal/handlers/openai_chat.go`:** Maps PMA `tool_call` to `cynodeai.tool_call` SSE where applicable.

## Tests Added (Green / Coverage)

- **`agents/internal/pma/streaming_fsm_red_test.go`:** FSM edge cases (tool blocks, EOF flush, stray closes, overwrite bounds).
- **`agents/internal/pma/chat_stream_test.go`:** `emitNDJSONEmissions`, `streamLangchainNDJSONToWriter`, `streamCapableModelNDJSON` (success, hard error, `ErrNotFinished` fallback), Ollama NDJSON with tool-call chunks.
- **`orchestrator/internal/pmaclient/client_test.go`:** `tool_call` line parsing, empty `delta`, thinking errors, managed-proxy stream bearer.

## E2E Prerequisite Fix

`just e2e --tags pma_inference` failed when `wait_for_pma_chat_ready` used a **180s** timeout after a cold `just setup-dev restart --force`, because Ollama can spend several minutes loading a large model into GPU/ROCm before the first chat returns **2xx**.

**Change:** `scripts/test_scripts/run_e2e.py` now uses **600s** for that wait when the `pma_inference` tag is selected.

## Validation Gates (Task 5 Testing)

Commands run successfully on **2026-03-27** (America/New_York):

- `just test-go-cover`
- `just lint-go`
- `just lint-python paths=scripts/test_scripts/run_e2e.py`
- `just lint-python paths=scripts/test_scripts/e2e_0620_pma_standard_path_streaming.py`
- `just test-bdd`
- `just setup-dev restart --force`
- `just e2e --tags pma_inference` -> **OK** (3 skips documented in test output: optional amendment/heartbeat paths).

## Related Documents

- [2026-03-27_task5_red_phase_execution_report.md](2026-03-27_task5_red_phase_execution_report.md)
- [2026-03-27_task5_discovery_streaming_notes.md](2026-03-27_task5_discovery_streaming_notes.md)
- [2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md](2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md) (Task 5 checkboxes updated).
