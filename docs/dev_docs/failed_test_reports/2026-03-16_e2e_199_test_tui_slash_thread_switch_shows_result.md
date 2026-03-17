# Failed E2E Report: e2e_0760_tui_slash_commands.test_tui_slash_thread_switch_shows_result

## 1 Summary

Test `e2e_0760_tui_slash_commands.TestTuiSlashCommands.test_tui_slash_thread_switch_shows_result` failed because the TUI did not show the prompt-ready landmark within the allowed timeout (12 seconds).
The test would exercise /thread switch and assert 'switched' confirmation or error in scrollback (Task 5A); it failed at _wait_ready(session) before reaching the thread switch step.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: False is not true : TUI did not show prompt-ready landmark within timeout`
- **Root cause:** Same as other e2e_0760 TUI tests: `wait_for_prompt_ready(timeout_sec=12)` returned False; prompt-ready landmark not present in PTY output within timeout.
- **Effect:** Test failed at _wait_ready before sending /thread switch or asserting on result.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0760_tui_slash_commands.py](../../../scripts/test_scripts/e2e_0760_tui_slash_commands.py) lines 476-481 (approx): TuiPtySession, _wait_ready(session) at line 481, then /thread switch and assert switched/error in scrollback.
- [tui_pty_harness.py](../../../scripts/test_scripts/tui_pty_harness.py): wait_for_prompt_ready and landmarks.

### 3.2 TUI and Specs

- [cynork_tui_slash_commands.md](../../tech_specs/cynork_tui_slash_commands.md): Task 5A - /thread switch shows 'switched' or error in scrollback.
- [cynork_tui.md](../../tech_specs/cynork_tui.md): TUI and prompt-ready behavior.

### 3.3 Backend Path

- TUI startup; thread switch and chat threads API.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-CLIENT](../../requirements/client.md): TUI and thread handling.

### 4.2 Tech Specs

- [cynork_tui_slash_commands.md](../../tech_specs/cynork_tui_slash_commands.md): /thread switch (Task 5A).

### 4.3 Feature Files

- TUI slash and thread feature files.

## 5 Implementation Deviation

- **Spec/requirement intent:** TUI MUST become prompt-ready; /thread switch MUST show 'switched' confirmation or error in scrollback.
- **Observed behavior:** TUI did not reach prompt-ready within timeout; test could not validate /thread switch.
- **Deviation:** Same as other e2e_0760 failures: prompt-ready landmark not seen within 12s.

## 6 What Needs to Be Fixed in the Implementation

- **Same root cause as [2026-03-16_e2e_199_test_tui_slash_version.md](2026-03-16_e2e_199_test_tui_slash_version.md).**
  Failure at _wait_ready; landmark is in [cynork/internal/tui/model.go](../../../cynork/internal/tui/model.go) View().
  Fix: EnsureThread in [cynork/cmd/tui.go](../../../cynork/cmd/tui.go) must not block >12s when token is set; or run E2E without token; increase ready timeout or verify PTY capture.
