---
name: Bugs (Bug 3 and Bug 4)
overview: |
  Completed: Bug 3 (/auth logout clears thread id; ensure-thread landmarks via
  createdNew / resumedFromCache; tests and reports in docs/dev_docs).
  Bug 4 (queue model: Enter queues while streaming, Ctrl+S/Ctrl+Enter send-now,
  Ctrl+Q explicit queue, drain on stream end; BDD, E2E, CI green).
  Remaining product notes (e.g. in-TUI login transcript scope) are documented in task reports.
todos:
  - id: bug-001
    content: "Read `cynork/internal/tui/model.go` logout path (~line 153-155 per report) and confirm `Session.CurrentThreadID` is not cleared on `/auth logout`."
    status: completed
  - id: bug-002
    content: "Read `cynork/internal/tui/model.go` scrollback landmark emission (~`THREAD_READY` / `THREAD_SWITCHED`) and map current behavior."
    status: completed
    dependencies:
      - bug-001
  - id: bug-003
    content: "Read `docs/tech_specs/cynork/cynork_tui_slash_commands.md` sections on `/auth login`, `/auth logout`, `/auth refresh` for expected behavior."
    status: completed
    dependencies:
      - bug-002
  - id: bug-004
    content: "Read `docs/tech_specs/cynork/cynork_tui.md` sections on thread initialization and scrollback landmarks for expected UX."
    status: completed
    dependencies:
      - bug-003
  - id: bug-005
    content: "Add a unit test: `/auth logout` must set `Session.CurrentThreadID` to empty string."
    status: completed
    dependencies:
      - bug-004
  - id: bug-006
    content: "Add a unit test: after `/auth login` as a different user, `runEnsureThread` must create a new thread (not reuse the stale thread from the previous session)."
    status: completed
    dependencies:
      - bug-005
  - id: bug-007
    content: "Add a unit test: scrollback landmark after thread creation emits `THREAD_READY`; landmark after `/thread switch` emits `THREAD_SWITCHED`."
    status: completed
    dependencies:
      - bug-006
  - id: bug-008
    content: "Run `go test -v -run 'TestLogoutClearsThread|TestLoginNewUser|TestScrollbackLandmark' ./cynork/internal/tui/...` and confirm expected failures."
    status: completed
    dependencies:
      - bug-007
  - id: bug-009
    content: "In `model.go` logout handler: zero `Session.CurrentThreadID` and `Session.CurrentThreadTitle` when `/auth logout` completes."
    status: completed
    dependencies:
      - bug-008
  - id: bug-010
    content: "In `model.go` `applyEnsureThreadResult`: emit `THREAD_READY` landmark for new thread creation and `THREAD_SWITCHED` for thread resumption or switch."
    status: completed
    dependencies:
      - bug-009
  - id: bug-011
    content: "Re-run `go test -v -run 'TestLogoutClearsThread|TestLoginNewUser|TestScrollbackLandmark' ./cynork/internal/tui/...` and confirm green."
    status: completed
    dependencies:
      - bug-010
  - id: bug-012
    content: "Verify no other Update() handlers read stale `CurrentThreadID` after logout; document any remaining product decisions on in-TUI login thread scope."
    status: completed
    dependencies:
      - bug-011
  - id: bug-013
    content: "Run `just lint-go` on changed cynork files and `go test -cover ./cynork/...`; confirm 90% threshold."
    status: completed
    dependencies:
      - bug-012
  - id: bug-014
    content: "Run `just e2e --tags tui_pty,no_inference` to verify TUI auth/thread regression."
    status: completed
    dependencies:
      - bug-013
  - id: bug-015
    content: "Validation gate -- do not proceed to Task 2 until all checks pass."
    status: completed
    dependencies:
      - bug-014
  - id: bug-016
    content: "Generate task completion report for Task 1 (changes, tests passed, deviations). Mark completed steps `- [x]`."
    status: completed
    dependencies:
      - bug-015
  - id: bug-017
    content: "Do not start Task 2 until Task 1 closeout is done."
    status: completed
    dependencies:
      - bug-016
  - id: bug-018
    content: "Read `cynork/internal/tui/model.go` `handleEnterKey` (~line 598) and map the current loading-guard logic during streaming."
    status: completed
    dependencies:
      - bug-017
  - id: bug-019
    content: "Read `docs/tech_specs/cynork/cynork_tui.md` Queued Drafts and Deferred Send section (~lines 164-196) for the normative Enter/Ctrl+Enter/Ctrl+Q behavior."
    status: completed
    dependencies:
      - bug-018
  - id: bug-020
    content: "Read `docs/requirements/client.md` for REQ-CLIENT-0221 (queue model) and related input-handling requirements."
    status: completed
    dependencies:
      - bug-019
  - id: bug-021
    content: "Add a unit test: while streaming, pressing Enter with non-empty composer must queue the draft (not send) and clear the composer."
    status: completed
    dependencies:
      - bug-020
  - id: bug-022
    content: "Add a unit test: while streaming, Ctrl+Enter must send immediately (interrupt streaming if needed) rather than queuing."
    status: completed
    dependencies:
      - bug-021
  - id: bug-023
    content: "Add a unit test: while not streaming, Enter must send immediately (existing behavior preserved)."
    status: completed
    dependencies:
      - bug-022
  - id: bug-024
    content: "Add a unit test: Ctrl+Q must queue without auto-send regardless of streaming state."
    status: completed
    dependencies:
      - bug-023
  - id: bug-025
    content: "Add a unit test: queued drafts are sent in FIFO order when streaming completes."
    status: completed
    dependencies:
      - bug-024
  - id: bug-026
    content: "Run `go test -v -run 'TestEnterQueues|TestCtrlEnterSends|TestEnterNotStreaming|TestCtrlQQueues|TestQueueFIFO' ./cynork/internal/tui/...` and confirm expected failures."
    status: completed
    dependencies:
      - bug-025
  - id: bug-027
    content: "Add or extend `scripts/test_scripts/e2e_0650_tui_queue_model.py` with tags `[suite_cynork, full_demo, tui_pty, no_inference]` and prereqs `[gateway, config, auth]`: Enter queues during streaming, Ctrl+Enter sends now, queue drains on stream end."
    status: completed
    dependencies:
      - bug-026
  - id: bug-028
    content: "Implement queue data structure: add `queuedDrafts []string` field to the TUI model."
    status: completed
    dependencies:
      - bug-027
  - id: bug-029
    content: "In `handleEnterKey`: when streaming is active, append composer text to `queuedDrafts` and clear composer instead of sending."
    status: completed
    dependencies:
      - bug-028
  - id: bug-030
    content: "Add `handleCtrlEnterKey`: send composer text immediately, interrupting streaming if active; clear composer."
    status: completed
    dependencies:
      - bug-029
  - id: bug-031
    content: "Add `handleCtrlQKey`: append composer text to `queuedDrafts` and clear composer regardless of streaming state."
    status: completed
    dependencies:
      - bug-030
  - id: bug-032
    content: "On stream completion (`streamDoneMsg` or equivalent): pop and send the next queued draft if `queuedDrafts` is non-empty."
    status: completed
    dependencies:
      - bug-031
  - id: bug-033
    content: "Re-run `go test -v -run 'TestEnterQueues|TestCtrlEnterSends|TestEnterNotStreaming|TestCtrlQQueues|TestQueueFIFO' ./cynork/internal/tui/...` and confirm green."
    status: completed
    dependencies:
      - bug-032
  - id: bug-034
    content: "Ensure slash and shell commands still dispatch during streaming (existing narrow guard preserved); verify with `go test -v -run TestSlashDuringStream ./cynork/internal/tui/...`."
    status: completed
    dependencies:
      - bug-033
  - id: bug-035
    content: "Run `just lint-go` on changed cynork files and `go test -race -cover ./cynork/...`; confirm 90% threshold."
    status: completed
    dependencies:
      - bug-034
  - id: bug-036
    content: "Run `just e2e --tags tui_pty,no_inference` to verify TUI input-handling regression."
    status: completed
    dependencies:
      - bug-035
  - id: bug-037
    content: "Validation gate -- do not proceed to Task 3 until all checks pass."
    status: completed
    dependencies:
      - bug-036
  - id: bug-038
    content: "Generate task completion report for Task 2 (changes, tests passed, deviations). Mark completed steps `- [x]`."
    status: completed
    dependencies:
      - bug-037
  - id: bug-039
    content: "Do not start Task 3 until Task 2 closeout is done."
    status: completed
    dependencies:
      - bug-038
  - id: bug-040
    content: "Update `docs/dev_docs/_todo.md` to mark Bug 3 and Bug 4 as resolved with links to this plan."
    status: completed
    dependencies:
      - bug-039
  - id: bug-041
    content: "Verify no follow-up work was left undocumented; note any remaining product decisions awaiting confirmation."
    status: completed
    dependencies:
      - bug-040
  - id: bug-042
    content: "Run `just docs-check` on all changed documentation."
    status: completed
    dependencies:
      - bug-041
  - id: bug-043
    content: "Run `just e2e --tags tui_pty,no_inference` as final E2E regression gate."
    status: completed
    dependencies:
      - bug-042
  - id: bug-044
    content: "Generate final plan completion report: tasks completed, overall validation, remaining risks."
    status: completed
    dependencies:
      - bug-043
  - id: bug-045
    content: "Mark all completed steps in the plan with `- [x]`. (Last step.)"
    status: completed
    dependencies:
      - bug-044
---

# Bugs Plan

## Goal

Address two open bugs from the review cycle: Bug 3 (/auth login thread UX) and Bug 4 (slash/shell blocked while streaming).
Both are in the cynork TUI module.
Bug 3 requires clearing stale thread state on logout and differentiating scrollback landmarks.
Bug 4 requires implementing the full spec queue model for chat input during streaming.

## References

- Requirements: [`docs/requirements/client.md`](../requirements/client.md)
- Tech specs: [`docs/tech_specs/cynork/cynork_tui.md`](../tech_specs/cynork/cynork_tui.md), [`docs/tech_specs/cynork/cynork_tui_slash_commands.md`](../tech_specs/cynork/cynork_tui_slash_commands.md)
- Review report: [`2026-03-29_review_report_4_cynork.md`](old/2026-03-29_review_report_4_cynork.md)
- Implementation: `cynork/internal/tui/`
- Todo: [`_todo.md`](_todo.md) section 2

## Constraints

- Requirements take precedence over tech specs; tech specs take precedence over current code.
- Follow BDD/TDD: failing tests before implementation.
- All changes must pass `just ci` before the task is considered complete.
- No changes that reduce test coverage below the 90% per-package threshold.
- No modifications to linter rules or suppression comments.
- Each task's validation gate must pass before starting the next task.
- Bug 3 has a pending product decision on in-TUI login thread context; document the gap and implement what is unambiguous now.

## Execution Plan

Tasks are ordered by independence.
Bug 3 (thread UX) has no dependency on Bug 4 (queue model) and vice versa.

### Task 1: Bug 3 -- /Auth Login Thread UX

`/auth logout` clears tokens but does not clear `Session.CurrentThreadID`.
Logging in as another user can reuse the stale thread.
Scrollback landmarks do not differentiate between new-thread-created and thread-switched, causing UX confusion.

#### Task 1 Requirements and Specifications

- [`docs/tech_specs/cynork/cynork_tui_slash_commands.md`](../tech_specs/cynork/cynork_tui_slash_commands.md) -- `/auth login`, `/auth logout`
- [`docs/tech_specs/cynork/cynork_tui.md`](../tech_specs/cynork/cynork_tui.md) -- thread initialization, scrollback landmarks
- [`docs/requirements/client.md`](../requirements/client.md) -- REQ-CLIENT-0190 (thread init defers when token exists)
- [Review Report 4](old/2026-03-29_review_report_4_cynork.md) -- stale thread after auth change (lines 153-155)

#### Discovery (Task 1) Steps

- [x] Read `cynork/internal/tui/model.go` logout path (~line 153-155 per report) and confirm `Session.CurrentThreadID` is not cleared on `/auth logout`.
- [x] Read `cynork/internal/tui/model.go` scrollback landmark emission (~`THREAD_READY` / `THREAD_SWITCHED`) and map current behavior.
- [x] Read `docs/tech_specs/cynork/cynork_tui_slash_commands.md` sections on `/auth login`, `/auth logout`, `/auth refresh` for expected behavior.
- [x] Read `docs/tech_specs/cynork/cynork_tui.md` sections on thread initialization and scrollback landmarks for expected UX.

#### Red (Task 1)

- [x] Add a unit test: `/auth logout` must set `Session.CurrentThreadID` to empty string.
- [x] Add a unit test: after `/auth login` as a different user, `runEnsureThread` must create a new thread (not reuse the stale thread from the previous session).
- [x] Add a unit test: scrollback landmark after thread creation emits `THREAD_READY`; landmark after `/thread switch` emits `THREAD_SWITCHED`.
- [x] Run `go test -v -run 'TestLogoutClearsThread|TestLoginNewUser|TestScrollbackLandmark' ./cynork/internal/tui/...` and confirm expected failures.

#### Green (Task 1)

- [x] In `model.go` logout handler: zero `Session.CurrentThreadID` and `Session.CurrentThreadTitle` when `/auth logout` completes.
- [x] In `model.go` `applyEnsureThreadResult`: emit `THREAD_READY` landmark for new thread creation and `THREAD_SWITCHED` for thread resumption or switch.
- [x] Re-run `go test -v -run 'TestLogoutClearsThread|TestLoginNewUser|TestScrollbackLandmark' ./cynork/internal/tui/...` and confirm green.

#### Refactor (Task 1)

- [x] Verify no other Update() handlers read stale `CurrentThreadID` after logout; document any remaining product decisions on in-TUI login thread scope.

#### Testing (Task 1)

- [x] Run `just lint-go` on changed cynork files and `go test -cover ./cynork/...`; confirm 90% threshold.
- [x] Run `just e2e --tags tui_pty,no_inference` to verify TUI auth/thread regression.
- [x] Validation gate -- do not proceed to Task 2 until all checks pass.

#### Closeout (Task 1)

- [x] Generate task completion report for Task 1 (changes, tests passed, deviations).
  Mark completed steps `- [x]`.
- [x] Do not start Task 2 until Task 1 closeout is done.

---

### Task 2: Bug 4 -- Slash/Shell Dispatch During Streaming and Queue Model

The `handleEnterKey` loading guard has been narrowed so slash and shell commands dispatch during streaming.
However, the full spec queue model is not implemented: while streaming, Enter should queue the draft (not block), Ctrl+Enter should send immediately (interrupt if needed), and Ctrl+Q should explicitly queue without auto-send.

#### Task 2 Requirements and Specifications

- [`docs/tech_specs/cynork/cynork_tui.md`](../tech_specs/cynork/cynork_tui.md) -- Queued Drafts and Deferred Send (~lines 164-196)
- [`docs/requirements/client.md`](../requirements/client.md) -- REQ-CLIENT-0221 (queue model)
- [Review Report 4](old/2026-03-29_review_report_4_cynork.md) -- Bug 4 partial fix (lines 62-63)

#### Discovery (Task 2) Steps

- [x] Read `cynork/internal/tui/model.go` `handleEnterKey` (~line 598) and map the current loading-guard logic during streaming.
- [x] Read `docs/tech_specs/cynork/cynork_tui.md` Queued Drafts and Deferred Send section (~lines 164-196) for the normative Enter/Ctrl+Enter/Ctrl+Q behavior.
- [x] Read `docs/requirements/client.md` for REQ-CLIENT-0221 (queue model) and related input-handling requirements.

#### Red (Task 2)

- [x] Add a unit test: while streaming, pressing Enter with non-empty composer must queue the draft (not send) and clear the composer.
- [x] Add a unit test: while streaming, Ctrl+Enter must send immediately (interrupt streaming if needed) rather than queuing.
- [x] Add a unit test: while not streaming, Enter must send immediately (existing behavior preserved).
- [x] Add a unit test: Ctrl+Q must queue without auto-send regardless of streaming state.
- [x] Add a unit test: queued drafts are sent in FIFO order when streaming completes.
- [x] Run `go test -v -run 'TestEnterQueues|TestCtrlEnterSends|TestEnterNotStreaming|TestCtrlQQueues|TestQueueFIFO' ./cynork/internal/tui/...` and confirm expected failures.
- [x] Add or extend `scripts/test_scripts/e2e_0650_tui_queue_model.py` with tags `[suite_cynork, full_demo, tui_pty, no_inference]` and prereqs `[gateway, config, auth]`: Enter queues during streaming, Ctrl+Enter sends now, queue drains on stream end.

#### Green (Task 2)

- [x] Implement queue data structure: add `queuedDrafts []string` field to the TUI model.
- [x] In `handleEnterKey`: when streaming is active, append composer text to `queuedDrafts` and clear composer instead of sending.
- [x] Add `handleCtrlEnterKey`: send composer text immediately, interrupting streaming if active; clear composer.
- [x] Add `handleCtrlQKey`: append composer text to `queuedDrafts` and clear composer regardless of streaming state.
- [x] On stream completion (`streamDoneMsg` or equivalent): pop and send the next queued draft if `queuedDrafts` is non-empty.
- [x] Re-run `go test -v -run 'TestEnterQueues|TestCtrlEnterSends|TestEnterNotStreaming|TestCtrlQQueues|TestQueueFIFO' ./cynork/internal/tui/...` and confirm green.

#### Refactor (Task 2)

- [x] Ensure slash and shell commands still dispatch during streaming (existing narrow guard preserved); verify with `go test -v -run TestSlashDuringStream ./cynork/internal/tui/...`.

#### Testing (Task 2)

- [x] Run `just lint-go` on changed cynork files and `go test -race -cover ./cynork/...`; confirm 90% threshold.
- [x] Run `just e2e --tags tui_pty,no_inference` to verify TUI input-handling regression.
- [x] Validation gate -- do not proceed to Task 3 until all checks pass.

#### Closeout (Task 2)

- [x] Generate task completion report for Task 2 (changes, tests passed, deviations).
  Mark completed steps `- [x]`.
- [x] Do not start Task 3 until Task 2 closeout is done.

---

### Task 3: Documentation and Closeout

- [x] Update `docs/dev_docs/_todo.md` to mark Bug 3 and Bug 4 as resolved with links to this plan.
- [x] Verify no follow-up work was left undocumented; note any remaining product decisions awaiting confirmation.
- [x] Run `just docs-check` on all changed documentation.
- [x] Run `just e2e --tags tui_pty,no_inference` as final E2E regression gate.
- [x] Generate final plan completion report: tasks completed, overall validation, remaining risks.
- [x] Mark all completed steps in the plan with `- [x]`. (Last step.)
