# Identified Bugs

## Overview

This doc records discovered bugs and their disposition.
As of **2026-03-30**, the items below are **closed** in the current tree (Bugs 3-4 implemented in `cynork`; Bug 5 closed earlier).
Earlier investigation notes are kept for context.

## Registry Status (2026-03-30)

- **Bug:** 3
  - area: TUI thread ensure scrollback / resume
  - status: Closed
- **Bug:** 4
  - area: TUI composer while loading or streaming
  - status: Closed
- **Bug:** 5
  - area: MCP gateway `skills.*` scoped IDs
  - status: Closed (2026-03-27)

## Bug 3: `cynork tui` `/auth login` Always Makes New Thread

**Status: closed (2026-03-30).**
The section title reflects the original report; **Bug 3 Resolution** describes what shipped.

### Bug 3 Summary (Historical)

In-session `/auth login` is tied to thread ensure; users could see a new thread or messaging that looked like a thread switch.

Investigation showed two layers: when the gateway actually creates a thread vs reusing one, and UX where successful ensure (pre-fix) always used a single "switched"-style landmark even when the id was unchanged.

### Bug 3 Evidence (Code Paths)

The following subsections trace the login and thread-ensure chain.

#### Post-Login Always Schedules Thread Ensure

After a successful in-TUI login, `applyLoginResult` always combines `ensureThreadCmd()` with optional gateway health polling (`cynork/internal/tui/model.go`).
There is no branch that skips thread ensure on login success.

#### What `ensureThreadCmd` Does

`ensureThreadCmd` calls `Session.EnsureThread(m.ResumeThreadSelector)` with the selector captured at process start from `--resume-thread` (`cynork/cmd/tui.go` passes `tuiResumeThread` into `runTUIWithSession` -> `SetResumeThreadSelector`).

#### When `EnsureThread` Creates a New Thread or Not

`chat.Session.EnsureThread` (`cynork/internal/chat/session.go`):

- If `resumeSelector` is non-empty: resolve via `ListThreads` + selector and set `CurrentThreadID` (no `NewThread` unless resolution implies switching to an existing id).
- If `resumeSelector` is empty and `CurrentThreadID` is already set: returns without calling `NewThread()` (documented as keeping the thread after in-session re-login).
- If `resumeSelector` is empty and `CurrentThreadID` is empty: calls `NewThread()` -> `POST /v1/chat/threads`.

Unit coverage: `TestSession_EnsureThread_SkipsNewWhenThreadAlreadySet` asserts zero POSTs when `CurrentThreadID` is preset.

A literal "always creates a new thread on every `/auth login`" would only hold if `CurrentThreadID` is always empty at login success.

For example, a typical no-token-at-launch flow where `Init` never ran `ensureThreadCmd` (no token), so the first successful login is the first thread ensure.

### Bug 3 Spec Expectations

- [cynork_tui.md](../tech_specs/cynork/cynork_tui.md) Auth Recovery: after successful in-session re-authentication, the TUI SHOULD resume the same session state and return focus to the interrupted flow rather than forcing a full restart.
- Same doc Entry / thread semantics: default is a new thread unless `--resume-thread` is supplied; thread must exist before the first completion.

There is tension only if "same session state" is interpreted as always keeping the same thread after login.

The implementation can keep the current thread when `CurrentThreadID` is already set, but will create one when it is not (first login in a session with no prior ensure).

### Bug 3 Likely Causes (User-Visible)

Several distinct mechanisms can explain the report.

#### Cause 1: First Login in Session With No Prior Thread ID

Launch without a saved token (`OpenLoginFormOnInit` or user invokes `/auth login` before any thread exists).

`CurrentThreadID` is empty -> `EnsureThread("")` -> `NewThread()`.

This matches "every time I log in I get a new thread" for users who routinely start unauthenticated or re-open the login flow before `Init` has finished ensuring a thread (edge timing).

#### Cause 2: Misleading Scrollback After Every Successful Ensure (Pre-Fix)

Before **Bug 3 Resolution**, every successful ensure could read like a thread switch.
The current code uses `[CYNRK_THREAD_READY]` vs `[CYNRK_THREAD_SWITCHED]` and flags such as `createdNew` / `resumedFromCache` (see `ensureThreadScrollbackLine`).

#### Cause 3: `--resume-thread` Only at CLI Startup

`ResumeThreadSelector` is fixed for the process lifetime.

In-session `/auth login` does not accept a thread selector; users who need to land in an existing thread after login must have passed `--resume-thread` at launch or switch with `/thread switch` after ensure completes.

#### Cause 4: `/auth logout` Does Not Clear `CurrentThreadID`

Logout clears tokens on the client/provider but leaves `Session.CurrentThreadID` as-is (`cynork/internal/tui/slash.go` `authLogout`).

A subsequent login may keep the old id client-side (`EnsureThread` skip path).

That is the opposite problem (stale thread id) but relevant when validating "new thread" reports.

### Bug 3 Suggested Fix (Historical Design Notes)

- Differentiate scrollback messages: after `ensureThreadResult` from post-login ensure, use a line that does not imply a switch when `EnsureThread` returned without creating a thread (e.g. only emit `[CYNRK_THREAD_SWITCHED]` when `NewThread` ran or selector resolution changed `CurrentThreadID`).
  Alternatively, split landmarks: "Thread ready:" vs "Switched to thread:".
- Optional: skip `ensureThreadCmd` after login when `CurrentThreadID` is already set and gateway already had a thread ensure from `Init` (redundant network), if product wants zero extra churn; today the skip is already logical inside `EnsureThread` without a POST.
- Document that first in-session login with no thread id will create a thread; re-login with an existing `CurrentThreadID` should not POST.

The scrollback differentiation and cache-resume ideas above are implemented in **Bug 3 Resolution (2026-03-30)**.
First login with no thread id may still create a thread when none can be resolved from cache or session state.

### Bug 3 Files Involved

- `cynork/internal/tui/model.go` - `applyLoginResult`, `ensureThreadCmd`, `applyEnsureThreadResult`, `Init` (token path schedules `ensureThreadCmd`).
- `cynork/internal/chat/session.go` - `EnsureThread`, `NewThread`.
- `cynork/cmd/tui.go` - `runTUI`, `runTUIWithSession`, `SetResumeThreadSelector`.
- `cynork/internal/tui/slash.go` - `/auth login` -> `openLoginFormMsg`; `authLogout`.
- Spec: `docs/tech_specs/cynork/cynork_tui.md` (Auth Recovery, Entry / threads).

### Bug 3 Suggested Tests (Historical)

- Session: `TestSession_EnsureThread_SkipsNewWhenThreadAlreadySet`.

- Model / UX: superseded by `bug3_thread_ux_test.go` and `ensureThreadScrollbackLine` table tests in `model_test.go`.

- Manual: logged-in TUI with an active thread -> `/auth login` re-auth -> confirm scrollback uses ready vs switched appropriately and POST behavior matches expectations.

### Bug 3 Resolution (2026-03-30)

- **Scrollback:** `ensureThreadScrollbackLine` in `cynork/internal/tui/model.go` chooses `[CYNRK_THREAD_READY]` vs `[CYNRK_THREAD_SWITCHED]` (`cynork/internal/chat/landmarks.go`) from `priorThreadID`, `createdNew`, `resumedFromCache`, and selector resolution, so re-auth that only confirms the same thread no longer reads like an arbitrary switch.
- **Resume:** `readResumeThreadFromCache` + `tuicache` can supply a thread id when the session had none, aligning first-login ensure with the last thread for that gateway/user/project where applicable.
- **Tests:** `cynork/internal/tui/bug3_thread_ux_test.go`, table coverage in `cynork/internal/tui/model_test.go` for `ensureThreadScrollbackLine`.

### Bug 3 Investigation Status (Historical)

Investigated (2026-03-24): behavior traced in `cynork`; causes included empty `CurrentThreadID` at first ensure and a single landmark for every successful ensure.
Addressed by the 2026-03-30 implementation above.

## Bug 4: `cynork tui` Cannot Submit Slash or Shell Commands While Chat is Streaming

**Status: closed (2026-03-30).**
The section title names the original report; details below distinguish **historical symptoms** from the **2026-03-30 resolution**.

### Bug 4 Summary (Historical Symptom)

Before the fix, while the assistant turn was **streaming** or other work held `Loading`, **slash** (`/…`) and **shell** (`!…`) lines could be blocked by the same Enter guard as plain chat, and streaming did not match the queued-drafts model in [cynork_tui.md](../tech_specs/cynork/cynork_tui.md).

### Bug 4 Observed Behavior (Historical)

- **Scenario:** Chat in progress; model response streaming (or `Loading` true for non-streaming work).
- **Symptom:** Enter on `/help` or `!pwd` did nothing until `Loading` cleared (pre-fix).
- **Contrast:** After fix, slash/shell dispatch while loading/streaming per **Bug 4 Resolution**; plain text queues while streaming.

### Bug 4 Root Cause (Historical, Pre-Fix)

Previously, `handleEnterKey` returned when `m.Loading && line != ""` before slash/shell/chat branches, so **all** non-empty submits were suppressed during loading.

### Bug 4 Resolution (2026-03-30)

- **Loading (non-streaming):** Plain chat can still be blocked while `Loading` is true for async work; lines starting with `/` or `!` are **not** blocked by that guard (`cynork/internal/tui/model.go` `handleEnterKey`).
- **Streaming:** While `isAgentStreaming()` is true, plain Enter **queues** drafts (`queuedAutoSend`); slash/shell run immediately.
  `EnterBlockedWhileLoading` documents the matrix.
  Ctrl+Enter / Ctrl+Q / Ctrl+S paths implement queue and interrupt behavior per `cynork/internal/tui/model_message_apply.go` and `model_composer_chords.go`.
- **Tests:** `cynork/internal/tui/bug4_queue_test.go` (`TestEnterBlockedWhileLoadingSemantics`, `TestEnterQueuesDuringStream`, `TestCtrlEnterSends`, etc.); BDD hooks in `cynork/_bdd/steps_cynork_tui_bugfixes.go`.

### Bug 4 Spec Alignment (Post-Fix)

[cynork_tui.md](../tech_specs/cynork/cynork_tui.md) (**Layout and Interaction** / streaming) calls for queued drafts while streaming and Ctrl+Enter to send now / interrupt.
The 2026-03-30 implementation provides queuing for plain lines during streaming, Ctrl+Enter / Ctrl+Q / Ctrl+S behavior in `model_message_apply.go` and `model_composer_chords.go`, and exempts `/` and `!` lines from the plain-chat loading block in `handleEnterKey`.

### Bug 4 Files Involved

- `cynork/internal/tui/model.go` - `handleEnterKey`, `handleSlashLine`, `streamCmd`, `applyStreamDone` / `Loading` lifecycle.
- Spec: [cynork_tui.md](../tech_specs/cynork/cynork_tui.md) (streaming, composer keybindings); [cynork_tui_slash_commands.md](../tech_specs/cynork/cynork_tui_slash_commands.md) for slash semantics.

### Bug 4 Tests (Implemented)

See `cynork/internal/tui/bug4_queue_test.go` and `cynork/_bdd/steps_cynork_tui_bugfixes.go`.
Manual check: start a long stream; `/help`, `!date`, and plain Enter (queue) should match the resolution above.

### Bug 4 Investigation Status (Historical)

Documented 2026-03-24: root cause was the blanket `handleEnterKey` loading guard.
Superseded by the 2026-03-30 behavior in **Bug 4 Resolution** above.

## Bug 5: MCP Gateway `skills.*` Tool Calls Return `task_id required`

**Status: closed (2026-03-27).**

Direct control-plane `POST /v1/mcp/tools/call` hits `mcpgateway.ToolCallHandler` in the control-plane (and the worker UDS proxy forwards the same JSON body to that path).
`helpers.mcp_tool_call` posts only `tool_name` and `arguments`; there is no top-level `task_id` field.
`ValidateRequiredScopedIds` in `orchestrator/internal/mcpgateway/handlers.go` maps each `skills.*` tool to `{UserID: true}` only.
It returns `task_id required` only when a tool declares `TaskID: true` (for example `preference.effective`), not for `skills.*`.
If `user_id` is missing or not a valid UUID string, the gateway returns `user_id required` instead.

The historical E2E failures with body `{"error":"task_id required"}` on `skills.*` do not match the current gateway implementation (regression during consolidation or a mismatched binary was suspected).
Closure work adds regression coverage: unit tests (`TestValidateRequiredScopedIds`, `TestValidateRequiredScopedIds_SkillsNeverRequireTaskID`), BDD (`features/orchestrator/orchestrator_mcp_skills.feature`), and the existing matrix in `e2e_0810_mcp_control_plane_tools.py`.

### Bug 5 Evidence (Historical)

```text
FAIL: test_mcp_tool_routes_round_trip (tool='skills.create')
AssertionError: 400 != 200 : {"error":"task_id required"}
```

(Same pattern for other `skills.*` subtests; worker UDS path reported similarly.)

### Bug 5 Resolution

- Confirmed request path: `scripts/test_scripts/helpers.py` -> control-plane `POST /v1/mcp/tools/call` -> `ToolCallHandler` -> `ValidateRequiredScopedIds` -> `handleSkills*`.
- No code change required in the gateway for scoped-ID rules; extraneous `task_id` in `arguments` is ignored for tools that do not require it.
- Re-run `just e2e --tags control_plane` against a live dev stack to confirm `e2e_0810` green in your environment.
