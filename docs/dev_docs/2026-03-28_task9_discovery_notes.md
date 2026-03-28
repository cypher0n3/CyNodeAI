# Task 9 Discovery: TUI Auth Recovery and In-Session Switches

- [Meta](#meta)
- [Requirements and spec](#requirements-and-spec-read)
- [Current implementation](#current-implementation-inspected)
- [PTY harness and E2E](#pty-harness-and-e2e)
- [BDD](#bdd)
- [Follow-on](#follow-on-remaining-for-task-9-closeout)

## Meta

**Date:** 2026-03-28.

## Requirements and Spec (Read)

- **REQ-CLIENT-0190** (`docs/requirements/client.md`): TUI MUST render without a valid token; on missing/invalid token, prompt for login in-TUI.
- **cynork_tui.md** (`docs/tech_specs/cynork_tui.md`): Auth Recovery, status bar, in-session switches (project, model).

## Current Implementation (Inspected)

- **Startup:** `cynork/cmd/tui.go` sets `OpenLoginFormOnInit = true` when `session.Client.Token == ""` after `runTUIWithSession`. `Model.Init` opens the login overlay.
  Satisfies startup path for empty token.
- **In-session auth failure:** Prior behavior surfaced gateway `*gateway.HTTPError` as scrollback `Error: ...` only.
  **Gap closed (2026-03-28):** `gateway.IsUnauthorized` + `applyUnauthorizedStreamEnd` / `applySendResult` branch open `applyOpenLoginForm()` and append `ScrollbackSystemLinePrefix` + `chat.LandmarkAuthRecoveryReady` guidance instead of a raw error-only line for HTTP 401.
- **Project/model:** Session carries `ProjectID` and model via slash commands (`/project`, `/model`); existing E2E in `e2e_0760` covers several slash flows.
  PTY-level startup-without-token and mid-session 401 are not yet covered by dedicated PTY E2E in this iteration.

## PTY Harness and E2E

- `scripts/test_scripts/tui_pty_harness.py`: `extract_thread_token_from_status`, `TuiPtySession`; used by `e2e_0750`, `e2e_0760`, `e2e_0765`.
- **Gaps for Task 9:** No dedicated PTY test that starts TUI with **no token file** and asserts login overlay landmarks; no PTY test that forces gateway 401 mid-chat (would require mock gateway or stack manipulation).

## BDD

- `cynork/_bdd/steps_cynork_extra_tui_deferred.go` keeps PTY/web login steps as `godog.ErrPending`; in-memory auth scenarios can be extended separately from PTY.

## Follow-On (Remaining for Task 9 Closeout)

- PTY E2E: startup without token; optional mock 401 for in-session recovery (or document reliance on unit + manual).
- BDD: add non-PTY scenarios where applicable, or keep PTY-only documentation.
- Unit tests: password/token never in scrollback/transcript (audit + tests).
- Full `just ci` / `just e2e`: ensure environment pylint/Python matches CI (local `just ci` may fail on pylint score threshold unrelated to Go edits).
