# Task 5: TUI Structured Streaming UX (Discovery Complete, Implementation Deferred)

- [Discovery Completed](#discovery-completed)
- [Deferred](#deferred-follow-up)

## Discovery Completed

**Date:** 2026-03-15  
**Plan:** [2026-03-15_streaming_specs_implementation_plan.md](2026-03-15_streaming_specs_implementation_plan.md) Task 5.

### Summary

- TUI state in `cynork/internal/tui/state.go`: `TranscriptTurn` and `TranscriptPart` exist but are not yet the canonical in-memory model; flat string scrollback remains the effective source of truth.
- Amendment path replaces whole buffer; no iteration-scoped overwrite; no heartbeat rendering or reconnect-state flow.
- E2E file ownership: e2e_198 (PTY cancel/reconnect), e2e_199 (slash toggles), e2e_204 (structured transcript) created as stubs.

## Deferred (Follow-Up)

- Promote TranscriptTurn/TranscriptPart to canonical model; render one in-flight turn; overwrite scopes; heartbeat; reconnect; `/show-tool-output`/`/hide-tool-output`; secure-buffer in TUI.
