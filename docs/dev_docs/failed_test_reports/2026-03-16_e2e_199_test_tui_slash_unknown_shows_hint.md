# Failed E2E Report: e2e_0760_tui_slash_commands.test_tui_slash_unknown_shows_hint

## 1 Summary

Test `e2e_0760_tui_slash_commands.TestTuiSlashCommands.test_tui_slash_unknown_shows_hint` failed because the TUI did not show the prompt-ready landmark within the allowed timeout (12 seconds).
The test would send `/notacommand` and assert a hint (Unknown/unknown/help) appears without exiting; it failed at _wait_ready(session) before reaching the slash command step.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: False is not true : TUI did not show prompt-ready landmark within timeout`
- **Root cause:** Same as [2026-03-16_e2e_199_test_tui_slash_version.md](2026-03-16_e2e_199_test_tui_slash_version.md): `wait_for_prompt_ready(timeout_sec=12)` returned False; PTY output did not contain `[CYNRK_PROMPT_READY]` or `[CYNRK_READY]` (or fallbacks) within timeout.
- **Effect:** Test failed at _wait_ready before sending `/notacommand` or asserting on hint.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0760_tui_slash_commands.py](../../../scripts/test_scripts/e2e_0760_tui_slash_commands.py) lines 108-118: TuiPtySession, _wait_ready(session) at line 114, then send_keys "/notacommand", read_until_landmark for hint.
- [tui_pty_harness.py](../../../scripts/test_scripts/tui_pty_harness.py): wait_for_prompt_ready, landmarks as in version report.

### 3.2 TUI and Specs

- Cynork TUI (see [cynork_tui.md](../../tech_specs/cynork_tui.md)): prompt-ready landmark; unknown slash command must show hint without exiting.
- [cynork_tui_slash_commands.md](../../tech_specs/cynork_tui_slash_commands.md): Unknown command behavior.

### 3.3 Backend Path

- TUI startup and gateway/auth; same as version report.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-CLIENT](../../requirements/client.md): TUI and slash command behavior.

### 4.2 Tech Specs

- [cynork_tui_slash_commands.md](../../tech_specs/cynork_tui_slash_commands.md): Unknown slash command shows hint.

### 4.3 Feature Files

- [cynork_tui_slash_task.feature](../../../features/cynork/cynork_tui_slash_task.feature) and related TUI slash features.

## 5 Implementation Deviation

- **Spec/requirement intent:** TUI MUST become prompt-ready and emit landmark; unknown slash command MUST show a hint without exiting.
- **Observed behavior:** TUI did not reach prompt-ready within timeout; test could not validate unknown-command hint.
- **Deviation:** Same root cause as other e2e_0760 failures: prompt-ready landmark not seen within 12s (TUI startup/landmark emission or environment).

## 6 What Needs to Be Fixed in the Implementation

- **Same root cause as [2026-03-16_e2e_199_test_tui_slash_version.md](2026-03-16_e2e_199_test_tui_slash_version.md).**
  The test fails at `_wait_ready` before sending `/unknown` or asserting on the hint.
  The landmark is implemented in [cynork/internal/tui/model.go](../../../cynork/internal/tui/model.go) View(); failure is due to TUI not reaching first paint within 12s (e.g. [cynork/cmd/tui.go](../../../cynork/cmd/tui.go) `EnsureThread` blocking when token is set, or PTY/timeout).
  Apply the same fixes as in the e2e_0760 test_tui_slash_version report section 6: ensure EnsureThread is fast or E2E runs without token, consider increasing the ready timeout, and verify PTY captures the view output.
