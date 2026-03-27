# Task 5 Discovery: PMA Streaming State Machine (Notes)

- [Requirements (`pmagnt.md`)](#requirements-pmagntmd)
- [Current code (`agents/internal/pma/`)](#current-code-agentsinternalpma)
- [Secure-buffer helper](#secure-buffer-helper)
- [Existing tests (gaps vs Task 5 Red list)](#existing-tests-gaps-vs-task-5-red-list)
- [Next (Task 5 Red)](#next-task-5-red)

## Requirements (`pmagnt.md`)

**Date:** 2026-03-27.

- **REQ-PMAGNT-0118:** Incremental streaming on standard path (not buffer-until-done).
- **REQ-PMAGNT-0120:** `llms.Model` wrapper; tee tokens to NDJSON and internal buffer; `iteration_start` before each iteration.
- **REQ-PMAGNT-0121:** Configurable state machine: visible vs thinking vs tool-call; think-tag open/close and tool-call markers per spec.
- **REQ-PMAGNT-0122 / 0123:** Full `thinking` events; `tool_call` events suppressed from visible stream.
- **REQ-PMAGNT-0124 / 0125:** Per-iteration and per-turn overwrite events; secret scan after each iteration.
- **REQ-PMAGNT-0126:** Wrap secret-bearing buffers with `runtime/secret` via shared helper (**REQ-STANDS-0133**).

## Current Code (`agents/internal/pma/`)

- **`streaming.go`:** `streamingLLM` emits `iteration_start` and **`delta` only** (raw chunks).
  No `thinking_delta`, `tool_call`, or overwrite events yet.
- **`langchain.go`:** Think blocks via `extractThinkBlocks` / XML markers for non-stream paths; `writeLangchainNDJSONStream` tests cover thinking in stream for langchain path.
- **`chat.go`:** Routes streaming to Ollama vs langchain; multiple NDJSON writers depending on path.

## Secure-Buffer Helper

- **`go_shared_libs/secretutil`:** `RunWithSecret` delegates to `runtime/secret` `secret.Do` when `GOEXPERIMENT=runtimesecret`; else no-op fallback (`secret_fallback.go`).
- PMA does **not** yet call `secretutil` on stream buffers (gap for Task 5 Green).

## Existing Tests (Gaps vs Task 5 Red List)

- **Present:** `langchain_test.go` (`TestWriteLangchainNDJSONStream_*`, `TestExtractThinkBlocks`), `chat_test.go` / `chat_stream_test.go` for stream wiring and Ollama chunk handling.
- **Missing (for Red):** Dedicated tests for a **token state machine** classifying streams into `delta` / `thinking_delta` / `tool_call`, **overwrite** scopes (per-iteration vs per-turn), and **secretutil** wrapping on append/replace paths.

## Next (Task 5 Red)

Add failing E2E, BDD, and Go tests per the plan checklist before implementation.
