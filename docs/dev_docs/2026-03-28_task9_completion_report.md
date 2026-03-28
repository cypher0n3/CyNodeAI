# Task 9 Completion Report: In-Session Auth Recovery and BDD/PTY Coverage

- [Meta](#meta)
- [Summary](#summary)
- [Code](#code)
- [Tests run](#tests-run)
- [Remaining follow-ups](#remaining-follow-ups)
- [Deviations](#deviations)

## Meta

**Date:** 2026-03-28.

## Summary

Implemented **in-session authentication recovery** when the gateway returns **HTTP 401** on streaming completion or non-streaming send result: the TUI opens the login overlay (`applyOpenLoginForm`) and appends a scrollback system line with `chat.LandmarkAuthRecoveryReady` plus user-facing text.

**BDD:** Replaced `godog.ErrPending` on login and auth recovery steps in `cynork/_bdd/steps_cynork_extra_tui_deferred.go` with in-memory `bddEnsureTui` simulation, using small **`tui.Model` BDD helpers** (`bdd_auth.go`) so `_bdd` does not reference unexported `loginResultMsg`.

**PTY E2E:** Added `test_tui_empty_env_tokens_shows_login_overlay` in `e2e_0765_tui_composer_editor.py` - clears `CYNORK_TOKEN` / `CYNORK_REFRESH_TOKEN` via `env_extra` and asserts the login overlay via `wait_for_login_form`.

Startup behavior (empty token opens login on init) remains in `cynork/cmd/tui.go` (`OpenLoginFormOnInit`).

## Code

- `cynork/internal/gateway/client_http.go`: `IsUnauthorized(err error) bool`.
- `cynork/internal/gateway/client_test.go`: `TestIsUnauthorized`.
- `cynork/internal/tui/model_message_apply.go`: `applyUnauthorizedStreamEnd`, `applyStreamDone` / `applySendResult` branches for 401.
- `cynork/internal/tui/model_unauthorized_recovery_test.go`: stream-done and send-result tests.
- `cynork/internal/tui/bdd_auth.go`: `BDDApplyLoginSuccess`, `BDDApplyLoginFailure`, `BDDApplyKey` for Godog steps.
- `cynork/_bdd/steps_cynork_streaming_bdd.go`: `bddSyncBddStream` after `Update` returns a new `tea.Model`.
- `cynork/_bdd/steps_cynork_extra_tui_deferred.go`: login, auth error, retry, mid-session 401 + login, interrupted-turn assertions.
- `cynork/internal/tui/model_credential_redaction_test.go`: password/token not echoed or persisted in scrollback/transcript (unit tests).
- `scripts/test_scripts/e2e_0765_tui_composer_editor.py`: empty-token PTY startup test.

## Tests Run

- `just test-bdd` (including `cynork/_bdd`)
- `just lint-go`
- (Prior / periodic) `just test-go-cover` for workspace modules as required by task gates.

## Remaining Follow-Ups

- **Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty` and `just e2e --tags auth` when the stack is available; confirm the new PTY test and existing auth/TUI modules pass.
- **`just ci`:** May still fail on `lint-python` / pylint in some environments; Go and BDD paths used here are clean.
- **Mid-session 401 over PTY** (live gateway returning 401 during a stream): not covered by a dedicated PTY script in this iteration; BDD and unit tests cover the model path.

## Deviations

- Plan Red-before-Green ordering: 401 behavior was locked with unit tests alongside implementation; BDD steps were filled after the `tui` API for BDD helpers existed.
