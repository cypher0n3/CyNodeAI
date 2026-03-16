# Failed E2E Report: e2e_199_tui_slash_commands.test_tui_slash_whoami_shows_identity

## 1 Summary

Test `e2e_199_tui_slash_commands.TestTuiSlashCommands.test_tui_slash_whoami_shows_identity` failed because the TUI did not show the prompt-ready landmark within the allowed timeout (12 seconds).
The test would send `/whoami` and assert current user identity or gateway error (Task 4D); it failed at _wait_ready(session) before reaching the slash command step.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: False is not true : TUI did not show prompt-ready landmark within timeout`
- **Root cause:** Same as other e2e_199 TUI tests: `wait_for_prompt_ready(timeout_sec=12)` returned False; prompt-ready landmark not present in PTY output within timeout.
- **Effect:** Test failed at _wait_ready before sending `/whoami` or asserting on identity output.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_199_tui_slash_commands.py](../../../scripts/test_scripts/e2e_199_tui_slash_commands.py) lines 452-462 (approx): TuiPtySession, _wait_ready(session) at line 457, then /whoami and assert identity or error in scrollback.
- [tui_pty_harness.py](../../../scripts/test_scripts/tui_pty_harness.py): wait_for_prompt_ready and landmarks.

### 3.2 TUI and Specs

- [cynork_tui_slash_commands.md](../../tech_specs/cynork_tui_slash_commands.md): Task 4D - /whoami shows current user identity or gateway error.
- [cynork_tui.md](../../tech_specs/cynork_tui.md): TUI and prompt-ready behavior.

### 3.3 Backend Path

- TUI startup; gateway whoami/identity for authenticated session.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-CLIENT](../../requirements/client.md): TUI and identity display.

### 4.2 Tech Specs

- [cynork_tui_slash_commands.md](../../tech_specs/cynork_tui_slash_commands.md): /whoami (Task 4D).

### 4.3 Feature Files

- TUI slash command feature files.

## 5 Implementation Deviation

- **Spec/requirement intent:** TUI MUST become prompt-ready; /whoami MUST show current user identity or gateway error in scrollback.
- **Observed behavior:** TUI did not reach prompt-ready within timeout; test could not validate /whoami.
- **Deviation:** Same as other e2e_199 failures: prompt-ready landmark not seen within 12s.

## 6 What Needs to Be Fixed in the Implementation

- **Same root cause as [2026-03-16_e2e_199_test_tui_slash_version.md](2026-03-16_e2e_199_test_tui_slash_version.md).**
  The test fails at _wait_ready; landmark is implemented in [cynork/internal/tui/model.go](../../../cynork/internal/tui/model.go).
  Fix: ensure TUI reaches first paint within the timeout (EnsureThread in [cynork/cmd/tui.go](../../../cynork/cmd/tui.go) non-blocking or fast, or run without token; increase ready timeout if needed; verify PTY sees the view).
