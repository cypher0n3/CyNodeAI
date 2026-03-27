# Task 5 Red Phase Execution Report (2026-03-27)

- [Summary](#summary)
- [Python E2E](#python-e2e)
- [BDD (agents)](#bdd-agents)
- [Go (agents/internal/pma)](#go-agentsinternalpma)
- [Open validation](#open-validation)

## Summary

Task 5 **Red** wiring: new failing tests and step definitions for PMA streaming,
overwrite, and secure-buffer gaps. **Green is not started** in this pass.

## Python E2E

File: `scripts/test_scripts/e2e_0620_pma_standard_path_streaming.py`.

- New tests require OpenAI-style content deltas plus named `cynodeai.thinking_delta`
  and `cynodeai.tool_call` SSE events, and at least one `cynodeai.amendment` on the
  standard PMA path.
- `just e2e --tags pma_inference` was run; the suite exited **1** because the
  **ollama** prereq failed (all matching tests skipped). Log:
  `tmp/e2e_pma_inference_task5_red_2026-03-27.log`.
- **Follow-up:** With `just setup-dev restart --force` and a healthy Ollama/PMA
  stack, re-run to confirm the new tests **fail** for the expected product gaps
  (not only skips).

## BDD (agents)

File: `agents/_bdd/pma_streaming_steps.go` (registered from `agents/_bdd/steps.go`).

- Implements previously **pending** steps for `features/agents/pma_chat_and_context.feature`
  streaming/overwrite scenarios so the suite **fails** with explicit errors instead
  of passing with pending steps.
- `go test ./agents/_bdd` **fails** (9 scenarios) as of this report; includes
  responses-surface and node-local placeholders returning structured Red errors,
  streaming NDJSON assertions (no `thinking` / `tool_call` / `overwrite` lines),
  and capable-model+MCP scenarios blocked until Green.

## Go (agents/internal/pma)

Files: `streaming_fsm.go`, `streaming_fsm_red_test.go`.

- Stub FSM (`newStreamingTokenFSM`, overwrite helpers) and tests that fail until
  Green implements classification, overwrite semantics, and `secretutil` wrapping.
- `go test ./agents/internal/pma` fails on the new Red tests (expected).

## Open validation

- **Red – Python E2E gate:** Not closed until a stack run proves new assertions
  fail (or pass after Green), not skip-only on prereqs.
- **Red validation gate (plan):** Keep open until the Python E2E gate above is
  satisfied.
