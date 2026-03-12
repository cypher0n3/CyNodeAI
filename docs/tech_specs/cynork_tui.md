# Cynork TUI

- [Document Overview](#document-overview)
- [Entrypoint and Compatibility](#entrypoint-and-compatibility)
- [Layout and Composer](#layout-and-composer)
- [Thread History](#thread-history)
- [Status Bar and Context](#status-bar-and-context)
- [Transcript Rendering](#transcript-rendering)
- [Generation State](#generation-state)
- [Slash Commands and Non-Interactive Behavior](#slash-commands-and-non-interactive-behavior)
- [Auth Recovery](#auth-recovery)
- [Explicitly Deferred From First Rollout](#explicitly-deferred-from-first-rollout)

## Document Overview

- Spec ID: `CYNAI.CLIENT.CynorkTui` <a id="spec-cynai-client-cynorktui"></a>

This spec defines the scope for the cynork full-screen TUI.
Initial draft is sufficient for end-to-end stack testing and user-level validation; richer features are deferred but will be added to this document (or related docs) in the future.

Traces To:

- [CYNAI.CLIENT.TuiScope](cynork_cli.md#spec-cynai-client-tuiscope)
- [REQ-CLIENT-0161](../requirements/client.md#req-client-0161)
- [REQ-CLIENT-0181](../requirements/client.md#req-client-0181)
- [REQ-CLIENT-0182](../requirements/client.md#req-client-0182)
- [REQ-CLIENT-0183](../requirements/client.md#req-client-0183)
- [REQ-CLIENT-0184](../requirements/client.md#req-client-0184)
- [REQ-CLIENT-0185](../requirements/client.md#req-client-0185)

Related:

- [Chat command](cli_management_app_commands_chat.md) (thread controls, slash commands, project/model context)
- [Chat threads and messages](chat_threads_and_messages.md)
- [OpenAI-compatible chat API](openai_compatible_chat_api.md)

## Entrypoint and Compatibility

- `cynork tui` is the explicit first full-screen TUI entrypoint.
  The CLI MUST provide a top-level `tui` command that starts the TUI.
- `cynork chat` MUST remain available during the first rollout.
  Implementations MAY make `cynork chat` invoke the same TUI code path or a line-oriented compatibility path; in either case, chat behavior MUST remain stable for users who invoke `cynork chat`.
- Bare `cynork` (no subcommand) MUST NOT launch the TUI by default until the explicit `cynork tui` rollout is stable; default behavior is out of scope for the first rollout.

## Layout and Composer

- **Minimum TUI layout:** The TUI MUST provide: (1) scrollback area for conversation and slash-command output, (2) composer for user input, (3) status bar, (4) optional context pane (e.g. thread list, project, slash help).
  Layout and keybindings MUST allow the user to focus the composer and send messages without leaving the TUI.
- **Multi-line composer:** The composer MUST support multi-line input suitable for long prompts and slash-command use.
  Send semantics (e.g. Enter vs Shift+Enter for newline vs send) MUST be clearly defined and discoverable (e.g. in status bar or help).
- **Scrollback:** The user MUST be able to scroll or navigate the scrollback; search and copy behavior SHOULD be available (exact contract is implementation-defined for the first pass).

## Thread History

- **Create, list, switch, rename:** The first TUI rollout MUST support: creating a new thread (startup or in-session, per [Thread Controls](cli_management_app_commands_chat.md#spec-cynai-client-clichatthreadcontrols)), listing threads for the current user and project context, switching to another thread, and renaming the current thread (if the gateway supports title update).
  Thread list MUST use the same gateway APIs as the CLI (e.g. `GET /v1/chat/threads` with pagination); ordering SHOULD be recent-first by default.
- Thread summary and archive are **deferred** from the first rollout unless they become necessary for the first testing pass.

## Status Bar and Context

- **Status bar fields:** The TUI MUST show at least: gateway (reachability or URL hint), identity (current user or "not logged in"), project (current project context or default), model (current model id), and connection state (e.g. connected / disconnected / error).
- **In-session switching:** The user MUST be able to change project context and model within the TUI session (e.g. via slash commands or context pane); behavior MUST align with [CliChatProjectContext](cli_management_app_commands_chat.md#spec-cynai-client-clichatprojectcontext) and [CliChatModelSelection](cli_management_app_commands_chat.md#spec-cynai-client-clichatmodelselection).
- **Gateway chat surfaces:** The first TUI rollout MUST include client-side chat wiring that can work with both `POST /v1/chat/completions` and `POST /v1/responses`.
  The default interactive path MAY remain implementation-defined for the first rollout, but the TUI chat abstraction MUST NOT hard-code only one of the two OpenAI-compatible endpoints.

## Transcript Rendering

- Spec ID: `CYNAI.CLIENT.CynorkTui.TranscriptRendering` <a id="spec-cynai-client-cynorktui-transcriptrendering"></a>

The TUI SHOULD follow the same broad rendering pattern used by modern open source chat tools such as Open WebUI and LibreChat: keep the main assistant answer readable, show reasoning as secondary collapsed content, and render tool activity as distinct non-prose rows.

- **Structured-turn preference:** When the gateway provides structured turn data, the TUI MUST prefer it over scraping prose from plain assistant text.
  When structured turn data is absent, the TUI MUST fall back to canonical plain-text transcript content.
- **Visible assistant text:** `text` parts are the main transcript content.
  When `--plain` is not set, the TUI MAY apply Markdown-aware rendering to visible assistant text.
- **Thinking and reasoning:** `thinking` parts MUST be hidden by default.
  The default collapsed rendering SHOULD be a compact placeholder such as `Thinking hidden`, `Thought`, or equivalent concise label.
  The TUI MAY offer an explicit user action to expand or inspect available thinking content.
- **Tool activity:** `tool_call` and `tool_result` parts SHOULD render as distinct transcript rows or cards that are visually different from normal assistant prose.
  Tool name and state SHOULD be visible.
  Argument previews and result previews SHOULD be truncated and redacted.
- **Download and attachment references:** When `attachment_ref` or `download_ref` parts are present, the TUI SHOULD render them as explicit non-prose items rather than burying them in assistant text.
  Full attachment upload UX and full download workflows remain outside the first-rollout scope unless promoted separately.
- **Multi-item assistant turns:** When one user prompt yields multiple assistant-side output items, the TUI MUST render those items in order as one logical assistant turn rather than as unrelated assistant messages.
- **Text-only fallback:** If only canonical plain-text content is available, the TUI MUST render a coherent text transcript and MUST NOT invent fake tool or thinking rows.

## Generation State

- Spec ID: `CYNAI.CLIENT.CynorkTui.GenerationState` <a id="spec-cynai-client-cynorktui-generationstate"></a>

- **In-flight assistant turn:** While a response is being generated, the TUI SHOULD update a single in-flight assistant turn in place rather than appending duplicate partial answers.
- **Progress indicators:** When structured progress is available, the TUI SHOULD surface concise progress state such as `Thinking`, `Calling tool`, `Waiting for tool result`, or equivalent user-displayable status.
- **Streaming text:** When visible assistant text streams in incrementally, the TUI SHOULD append it to the active assistant text area for the current turn.
- **Final reconciliation:** When the final assistant turn is committed, the TUI SHOULD reconcile any placeholders, partial text, and tool-activity state into the final ordered transcript without duplicating visible assistant text.

## Slash Commands and Non-Interactive Behavior

- **Slash-command parity:** The TUI MUST support at least the same slash-command surface as the existing CLI chat command (e.g. `/exit`, `/help`, `/thread new`, `/project`, `/model`, `/task`, `/status`, `/whoami`, `/nodes`, `/prefs`, `/skills`).
  See [CLI Management App - Chat Command](cli_management_app_commands_chat.md).
- **Non-interactive behavior:** When the CLI is invoked in a non-interactive way (e.g. `cynork chat -m "..."` for one-shot, or scripting), behavior MUST remain stable and parseable; the TUI MUST NOT be required for one-shot or piped use cases.

## Auth Recovery

- **Startup:** When the user starts the TUI without a usable token (missing or expired), the TUI MUST offer a way to log in (e.g. prompt or overlay) so the user can obtain a token without exiting.
- **In-session:** When the gateway returns an auth failure (e.g. 401) during a TUI session, the TUI MUST offer in-session login recovery (e.g. re-prompt for credentials or refresh) so the session can continue after re-auth.
- **Secrets:** Passwords and tokens MUST NOT be echoed, persisted in transcript history, or written to temporary UI state in plaintext; input for secrets MUST use secure input (no echo).

## Explicitly Deferred From First Rollout

The following are **out of scope** for the first TUI rollout and are deferred until after the first usable TUI is validated:

- Thread summary and archive (unless needed for first testing pass).
- Assistant download references and file-output workflows (unless they become a first-round requirement).
- Full attachment upload UX (unless the upload contract is finalized this round).
- Queued drafts and deferred send behavior.
- Making bare `cynork` launch the TUI by default.
- Web-based login and SSO-specific flows (unless enterprise auth becomes an immediate blocker).
