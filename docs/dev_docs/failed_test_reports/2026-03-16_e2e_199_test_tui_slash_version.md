# Failed E2E Report: e2e_199_tui_slash_commands.test_tui_slash_version

## 1 Summary

Test `e2e_199_tui_slash_commands.TestTuiSlashCommands.test_tui_slash_version` failed because the TUI did not show the prompt-ready landmark within the allowed timeout (12 seconds after startup delay).
The test starts a TuiPtySession, calls `_wait_ready(session)` (which waits for `[CYNRK_PROMPT_READY]` or `[CYNRK_READY]` in PTY output), then would run `/version` and assert version string in scrollback; it failed at _wait_ready.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: False is not true : TUI did not show prompt-ready landmark within timeout`
- **Root cause:** `session.wait_for_prompt_ready(timeout_sec=12)` returned False; the TUI process output (captured via PTY) did not contain the landmark strings `[CYNRK_PROMPT_READY]` or `[CYNRK_READY]` within the timeout, and the fallback checks (" (scrollback)" or "> ") also did not match.
- **Effect:** The test failed at line 47 in _wait_ready before sending `/version` or asserting on version output.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_199_tui_slash_commands.py](../../../scripts/test_scripts/e2e_199_tui_slash_commands.py) lines 91-98: TuiPtySession(context, timeout=25), then _wait_ready(session) at line 97; _wait_ready (lines 43-50) sleeps _TUI_STARTUP_DELAY_SEC, then session.wait_for_prompt_ready(timeout_sec=12).
- [tui_pty_harness.py](../../../scripts/test_scripts/tui_pty_harness.py): TuiPtySession spawns cynork TUI; wait_for_prompt_ready calls read_until_landmark([LANDMARK_PROMPT_READY, LANDMARK_PROMPT_READY_SHORT], timeout_sec); landmarks are `[CYNRK_PROMPT_READY]` and `[CYNRK_READY]`.

### 3.2 TUI Implementation

- Cynork TUI (see [cynork_tui.md](../../tech_specs/cynork_tui.md)): must emit the prompt-ready landmark (e.g. in status bar or debug output) when the main view is ready for input.
- [cynork_tui.md](../../tech_specs/cynork_tui.md), [cynork_tui_slash_commands.md](../../tech_specs/cynork_tui_slash_commands.md): TUI behavior and slash commands.

### 3.3 Backend Path

- TUI connects to gateway for auth and chat; if TUI blocks on login or gateway and never reaches the main prompt, the landmark is never emitted.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-CLIENT](../../requirements/client.md): CLI and TUI parity; TUI slash commands (per cynork_tui_slash_commands spec).

### 4.2 Tech Specs

- [cynork_tui.md](../../tech_specs/cynork_tui.md): TUI architecture and prompt-ready behavior.
- [cynork_tui_slash_commands.md](../../tech_specs/cynork_tui_slash_commands.md): /version and other slash commands (Task 4D, 5A references in plan).

### 4.3 Feature Files

- [cynork_tui_slash_task.feature](../../../features/cynork/cynork_tui_slash_task.feature), [cynork_tui_slash_commands](../../../features/cynork): TUI slash command scenarios.

## 5 Implementation Deviation

- **Spec/requirement intent:** TUI MUST reach a prompt-ready state and expose a landmark so E2E can detect readiness; /version MUST show cynork version in scrollback.
- **Observed behavior:** TUI did not emit the prompt-ready landmark within 12 seconds in the PTY harness; test could not proceed to /version.
- **Deviation:** Either (1) the TUI does not emit [CYNRK_PROMPT_READY] or [CYNRK_READY] when ready, (2) the TUI takes longer than 12s to become ready (e.g. auth or gateway wait), (3) the TUI exits or crashes before ready, or (4) PTY capture does not see the landmark (e.g. buffering or different output path).

## 6 What Needs to Be Fixed in the Implementation

The following describes where the landmark is implemented and why the test may not see it in time.

### 6.1 Root Cause (Implemented Behavior)

- The landmark **is implemented.**
  In [cynork/internal/tui/model.go](../../../cynork/internal/tui/model.go), `View()` (lines 786-845) always includes `chat.LandmarkPromptReady` in the status bar (line 812) and, when scrollback is empty, `chat.LandmarkPromptReadyShort` in the scrollback area (line 803).
- Constants are in [cynork/internal/chat/landmarks.go](../../../cynork/internal/chat/landmarks.go).
- The TUI is started in [cynork/cmd/tui.go](../../../cynork/cmd/tui.go): when `session.Client.Token == ""`, `OpenLoginFormOnInit = true` and the first `Init()` sends `openLoginFormMsg`, so the login overlay is shown but the main view (including status bar with landmark) is still rendered underneath.
- When token is set, `EnsureThread(tuiResumeThread)` is called **before** `tea.NewProgram` (lines 40-43); if that blocks on the gateway (e.g. create or resolve thread), the TUI never starts and no output is written.

### 6.2 What is Not Implemented or is Misconfigured

- **Blocking before first paint:** If E2E runs with a token, `EnsureThread` runs synchronously before the TUI process enters the event loop; if the gateway is slow or unavailable, this can exceed 12s and the test never sees the landmark.
  **First render timing:** Bubbletea renders after Init and on updates; if the program blocks in Init or in an early Update (e.g. waiting on gateway), the first View() may be delayed.
  **PTY:** The test uses `script -q -c "cynork ... tui"`; alt-screen or terminal mode may affect what is written to the PTY and when it is flushed.
  **Timeout:** 12s may be too short if the gateway or auth step is slow.

### 6.3 Exact Code or Config Changes

- **Ensure thread non-blocking or fast:** When running E2E with token, ensure the gateway responds to the thread-ensure (create or list) request within a few seconds, or run the TUI without token so no EnsureThread is done at startup (then the login form is shown and the landmark is still in the view).
  **E2E:** Increase `_wait_ready` timeout (e.g. 20s) or ensure the test environment has a responsive gateway.
  **Debug:** Log or write the landmark to stderr in addition to the normal view to confirm it is emitted and to rule out PTY buffering.
