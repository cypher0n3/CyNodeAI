# Task 2: PMA Standard-Path Streaming Closeout Report

- [What Changed](#what-changed)
- [Event Emission](#event-emission-minimal-green)
- [Tests and Lint](#tests-and-lint)

## What Changed

**Date:** 2026-03-15
**Plan:** [2026-03-15_streaming_specs_implementation_plan.md](2026-03-15_streaming_specs_implementation_plan.md) Task 2.

### Summary

- **streaming.go (new):** `streamingLLM` wraps any `llms.Model`; `GenerateContent` emits NDJSON `{"iteration_start": N}` then delegates to inner with `llms.WithStreamingFunc` that writes `{"delta": "..."}` and flushes. `Call` implemented via `llms.GenerateFromSinglePrompt`.
- **langchain.go:** `runCompletionWithLangchainStreaming(ctx, fullPrompt, mcpClient, w, logger)` builds base Ollama LLM (or test double), wraps with `newStreamingLLM`, runs OpenAIFunctionsAgent executor, then writes `{"done": true}` and flushes.
  Same agent/tools as blocking path.
- **chat.go:** When `req.Stream && !canStreamCompletion(req)` (capable model + MCP), handler calls `streamCompletionLangchainToWriter` instead of `resolveContent`. `streamCompletionLangchainToWriter` sets `Content-Type: application/x-ndjson`, 200, then `runCompletionWithLangchainStreaming`.

## Event Emission (Minimal Green)

- PMA now emits: `iteration_start` (before each agent iteration), `delta` (per token from Ollama), `done` (after `exec.Call`).
- Token state machine, `tool_progress`/`tool_result` injection, overwrite events, and secure-buffer wrapping are deferred to a later refinement.

## Tests and Lint

- **Go:** `go test ./agents/internal/pma/... -count=1` passes (all existing tests; no new streaming-specific unit tests added).
- **Lint:** `just lint-go` passes.
- **E2E:** `e2e_0620_pma_standard_path_streaming.py` asserts on **gateway** SSE (e.g. `cynodeai.iteration_start`).
  The gateway currently only forwards `delta` from PMA NDJSON (`processNDJSONLine` ignores other keys).
  So e2e_0620 will remain red until Task 3 (gateway relay) emits named events.

## Fallbacks Preserved

- Direct-inference streaming path unchanged (`canStreamCompletion` true -> `streamCompletionToWriter`).
- Blocking path unchanged when `req.Stream` is false.
- On streaming-langchain error we log and do not write `{"done": true}`; client sees partial NDJSON.
