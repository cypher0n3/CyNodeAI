# Cynork TUI

- [Document Overview](#document-overview)
  - [Cynork TUI Traces To](#cynork-tui-traces-to)
  - [Related Documents](#related-documents)
- [Entrypoint and Compatibility](#entrypoint-and-compatibility)
  - [Entrypoint and Compatibility Traces To](#entrypoint-and-compatibility-traces-to)
- [Layout and Interaction](#layout-and-interaction)
  - [Layout and Interaction Traces To](#layout-and-interaction-traces-to)
- [Visual Mockup](#visual-mockup)
- [Thread History](#thread-history)
- [Transcript Rendering](#transcript-rendering)
  - [Transcript Rendering Requirements Traces](#transcript-rendering-requirements-traces)
- [Generation State](#generation-state)
  - [Generation State Traces To](#generation-state-traces-to)
- [Completion and Discovery](#completion-and-discovery)
  - [Completion and Discovery Traces To](#completion-and-discovery-traces-to)
- [Local File References](#local-file-references)
  - [Local File References Traces To](#local-file-references-traces-to)
- [Local Config](#local-config)
  - [Local Config Traces To](#local-config-traces-to)
- [Local Cache](#local-cache)
  - [Local Cache Traces To](#local-cache-traces-to)
- [Auth Recovery](#auth-recovery)
  - [Auth Recovery Traces To](#auth-recovery-traces-to)
- [Web Login](#web-login)
  - [Web Login Traces To](#web-login-traces-to)

## Document Overview

- Spec ID: `CYNAI.CLIENT.CynorkTui` <a id="spec-cynai-client-cynorktui"></a>

This spec defines the full-screen interactive chat TUI for cynork.
It is the canonical home for the cynork chat layout, structured transcript rendering, thread-history UX, local TUI state, and interactive auth recovery.

### Cynork TUI Traces To

- [CYNAI.CLIENT.TuiScope](cynork_cli.md#spec-cynai-client-tuiscope)
- [REQ-CLIENT-0181](../requirements/client.md#req-client-0181)
- [REQ-CLIENT-0182](../requirements/client.md#req-client-0182)
- [REQ-CLIENT-0183](../requirements/client.md#req-client-0183)
- [REQ-CLIENT-0184](../requirements/client.md#req-client-0184)
- [REQ-CLIENT-0185](../requirements/client.md#req-client-0185)
- [REQ-CLIENT-0187](../requirements/client.md#req-client-0187)
- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188)
- [REQ-CLIENT-0189](../requirements/client.md#req-client-0189)
- [REQ-CLIENT-0190](../requirements/client.md#req-client-0190)
- [REQ-CLIENT-0191](../requirements/client.md#req-client-0191)
- [REQ-CLIENT-0192](../requirements/client.md#req-client-0192)
- [REQ-CLIENT-0193](../requirements/client.md#req-client-0193)
- [REQ-CLIENT-0194](../requirements/client.md#req-client-0194)
- [REQ-CLIENT-0195](../requirements/client.md#req-client-0195)
- [REQ-CLIENT-0196](../requirements/client.md#req-client-0196)
- [REQ-CLIENT-0197](../requirements/client.md#req-client-0197)
- [REQ-CLIENT-0198](../requirements/client.md#req-client-0198)
- [REQ-CLIENT-0199](../requirements/client.md#req-client-0199)
- [REQ-CLIENT-0200](../requirements/client.md#req-client-0200)
- [REQ-CLIENT-0201](../requirements/client.md#req-client-0201)
- [REQ-CLIENT-0202](../requirements/client.md#req-client-0202)
- [REQ-CLIENT-0203](../requirements/client.md#req-client-0203)
- [REQ-CLIENT-0204](../requirements/client.md#req-client-0204)
- [REQ-CLIENT-0205](../requirements/client.md#req-client-0205)
- [REQ-CLIENT-0206](../requirements/client.md#req-client-0206)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

### Related Documents

- [Chat command](cli_management_app_commands_chat.md)
- [TUI slash commands](cynork_tui_slash_commands.md)
- [Chat threads and messages](chat_threads_and_messages.md)
- [OpenAI-compatible chat API](openai_compatible_chat_api.md)

## Entrypoint and Compatibility

- Spec ID: `CYNAI.CLIENT.CynorkTui.EntryPoint` <a id="spec-cynai-client-cynorktui-entrypoint"></a>

### Entrypoint and Compatibility Traces To

- [REQ-CLIENT-0197](../requirements/client.md#req-client-0197)
- [REQ-CLIENT-0202](../requirements/client.md#req-client-0202)

- `cynork tui` is the explicit full-screen TUI entrypoint and MUST be provided as a top-level command.
- `cynork chat` MUST remain available as a supported user-facing path to the same chat contract.
  The implementation MAY share the same TUI code path or retain a documented compatibility wrapper, but user-visible behavior MUST remain aligned.
- Bare `cynork` without a subcommand SHOULD remain help-first until the dedicated default-TUI switch is intentionally promoted.
- `cynork shell` is deprecated as the primary interactive experience.
  Interactive chat behavior belongs to the TUI and chat-command specs.

## Layout and Interaction

- Spec ID: `CYNAI.CLIENT.CynorkChat.TUILayout` <a id="spec-cynai-client-cynorkchat-tuilayout"></a>

### Layout and Interaction Traces To

- [REQ-CLIENT-0189](../requirements/client.md#req-client-0189)
- [REQ-CLIENT-0190](../requirements/client.md#req-client-0190)
- [REQ-CLIENT-0192](../requirements/client.md#req-client-0192)
- [REQ-CLIENT-0193](../requirements/client.md#req-client-0193)
- [REQ-CLIENT-0194](../requirements/client.md#req-client-0194)
- [REQ-CLIENT-0195](../requirements/client.md#req-client-0195)
- [REQ-CLIENT-0196](../requirements/client.md#req-client-0196)
- [REQ-CLIENT-0198](../requirements/client.md#req-client-0198)
- [REQ-CLIENT-0199](../requirements/client.md#req-client-0199)
- [REQ-CLIENT-0200](../requirements/client.md#req-client-0200)
- [REQ-CLIENT-0201](../requirements/client.md#req-client-0201)
- [REQ-CLIENT-0202](../requirements/client.md#req-client-0202)
- [REQ-CLIENT-0203](../requirements/client.md#req-client-0203)
- [REQ-CLIENT-0204](../requirements/client.md#req-client-0204)
- [REQ-CLIENT-0205](../requirements/client.md#req-client-0205)
- [REQ-CLIENT-0206](../requirements/client.md#req-client-0206)
- [REQ-CLIENT-0207](../requirements/client.md#req-client-0207)

The TUI is a full-screen chat surface composed of scrollback, composer, status bar, and an optional context pane.

- The scrollback MUST render conversation history and structured chat-turn output.
- The composer MUST support multi-line input suitable for long prompts and slash commands.
- Exact slash-command semantics and execution algorithms are defined in [TUI slash commands](cynork_tui_slash_commands.md).
- The TUI MUST surface a concise discoverability hint in or adjacent to the composer for slash commands, `@` file lookup or attachment, and `!` shell shorthand.
  A canonical first-rollout hint is `/ commands · @ files · ! shell`.
- The status bar MUST display at least gateway reachability, identity, effective project, selected model, and connection state.
- An optional context pane MAY show thread history, slash-command help, recent tasks, project context, queued drafts, or download items.
- When the optional context pane is visible, the TUI SHOULD allow the user to switch between pane views without leaving the active chat session.
- Visible assistant text and stored user messages SHOULD be rendered with Markdown-aware formatting when `--plain` is not set.
- User messages in the scrollback SHOULD be visually distinct from assistant output.
- The user MUST be able to scroll back through the conversation and fetch older history as needed.
- Mouse-wheel scrolling MUST scroll the scrollback or output-history region.
  It MUST NOT cycle composer-history recall and MUST NOT alter previously submitted messages.
- When the composer has focus, the TUI MUST render a visible text cursor or caret at the current insertion point.
- The cursor or caret MAY use the terminal's native cursor rendering or a TUI-rendered equivalent, but it MUST remain visually distinguishable from surrounding composer text.
- `Shift+Enter` MUST insert a newline in the composer.
- The TUI SHOULD support message-history recall from the composer and SHOULD support search and copy behavior inside the scrollback.
- `Ctrl+C` SHOULD cancel an active generation first and SHOULD require an explicit repeated user action before exiting an idle session.
- When the user invokes `! command`, non-interactive shell output MAY render inline, but interactive subprocesses MUST receive the real TTY and the TUI MUST restore itself cleanly afterward.
- When the gateway exposes assistant download references, the TUI SHOULD render them as explicit download items that require user action.
- Queued drafts, when supported, MUST remain local unsent state and MUST be clearly separated from sent messages.
- When queued drafts are supported, the TUI SHOULD provide a dedicated queued-draft list, pane view, or overlay with clear queued labeling.
- Queued drafts SHOULD support reorder, explicit send-one, and explicit send-all operations.
- If the TUI supports send-all for queued drafts, it SHOULD indicate whether drafts are sent sequentially and whether the client waits for each response before sending the next queued draft.

## Visual Mockup

The canonical retained design mockup for the TUI is shown below.

![Cynork chat TUI mockup](./images/cynork_chat_tui_mockup.png)

## Thread History

- Create, list, switch, and rename thread behavior are part of the TUI baseline.
- Thread history MUST use the same gateway thread-management APIs as the chat command.
- Recent-first ordering by `updated_at` SHOULD be the default presentation.
- When thread summaries are available, the TUI SHOULD display them in the history list or sidebar.
- When archive support exists, archived threads SHOULD be accessible through an explicit filtered history view rather than the default active list.
- Wherever the TUI offers thread switching, it MUST show a user-typeable thread selector for each visible thread rather than assuming the user will type a raw backend UUID.
- The selector MAY be a stable short handle, a list ordinal within the current thread list view, an unambiguous displayed title form, or another compact human-typeable token, but the same selector form shown in the UI MUST be accepted by `/thread switch`.

## Transcript Rendering

- Spec ID: `CYNAI.CLIENT.CynorkTui.TranscriptRendering` <a id="spec-cynai-client-cynorktui-transcriptrendering"></a>

The TUI SHOULD follow the same broad rendering pattern used by modern chat tools: keep the main assistant answer readable, keep reasoning secondary, and make tool activity explicit.

- When the gateway provides structured turn data, the TUI MUST prefer it over scraping prose from plain assistant text.
- `text` parts are the primary readable transcript content.
- `thinking` parts MUST be hidden by default and rendered as compact placeholders until the user explicitly expands them.
  The canonical slash-command controls for this behavior are `/show-thinking` and `/hide-thinking` as defined in [TUI slash commands](cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-localslashcommands).
- Hidden-by-default `thinking` placeholders MUST remain visibly present in the transcript rather than disappearing completely.
- The collapsed `thinking` placeholder MUST render as its own dedicated transcript block within the same logical assistant turn; it MUST NOT be merged into normal assistant prose paragraphs and MUST NOT disappear entirely while collapsed.
- The collapsed `thinking` block MUST contain all of the following visible elements:
  - a primary label that begins with `Thinking`
  - a collapsed-state affordance such as `...`, a chevron, or another compact indicator that the block is not currently expanded
  - an explicit expand hint that includes the literal command `/show-thinking`
- While collapsed, the block MUST NOT render any raw reasoning text, partial hidden-thinking text, or `<think>`-style tags.
- The collapsed `thinking` block MUST use a secondary visual treatment that is distinct from normal assistant prose by applying all of the following at the same time:
  - lower-emphasis foreground styling than the main assistant text
  - a dedicated container treatment, such as a border, shaded background, or both
  - compact padding and spacing so the block reads as metadata or auxiliary content rather than as the main answer body
- The collapsed block SHOULD fit on one compact row when the label and hint fit within the available transcript width.
  When they do not fit, the implementation MAY wrap to a second compact line, but the label and `/show-thinking` hint MUST remain visible without horizontal scrolling.
- If hidden `thinking` updates arrive while the turn is still streaming and the block remains collapsed, the implementation MUST update the same placeholder block in place rather than appending duplicate collapsed-thinking rows.
- `tool_call` and `tool_result` parts SHOULD render as distinct non-prose rows with redacted, truncated previews when needed.
- `download_ref` and `attachment_ref` parts SHOULD render as explicit items rather than being flattened into prose.
- When one user prompt yields multiple assistant-side items, the TUI MUST preserve their order as one logical assistant turn.
- If only canonical plain-text content is available, the TUI MUST fall back to a coherent text transcript without inventing tool or thinking rows.

### Transcript Rendering Requirements Traces

- [REQ-CLIENT-0182](../requirements/client.md#req-client-0182)
- [REQ-CLIENT-0183](../requirements/client.md#req-client-0183)
- [REQ-CLIENT-0184](../requirements/client.md#req-client-0184)
- [REQ-CLIENT-0192](../requirements/client.md#req-client-0192)
- [REQ-CLIENT-0193](../requirements/client.md#req-client-0193)
- [REQ-CLIENT-0194](../requirements/client.md#req-client-0194)
- [REQ-CLIENT-0195](../requirements/client.md#req-client-0195)
- [REQ-CLIENT-0208](../requirements/client.md#req-client-0208)

## Generation State

- Spec ID: `CYNAI.CLIENT.CynorkTui.GenerationState` <a id="spec-cynai-client-cynorktui-generationstate"></a>

### Generation State Traces To

- [REQ-CLIENT-0185](../requirements/client.md#req-client-0185)
- [REQ-CLIENT-0209](../requirements/client.md#req-client-0209)

- While a response is in progress, the TUI MUST render exactly one in-flight assistant turn and MUST update that turn in place rather than appending duplicate assistant rows.
- The in-flight status indicator MUST be attached to the active assistant turn, not only to a global status bar.
- The indicator MUST render as a visually distinct status chip rather than as bare transcript prose.
- The canonical status-chip format is `[<spinner> <label>...]`.
- The canonical spinner sequence is the Unicode Braille pattern sequence U+280B, U+2819, U+2839, U+2838, U+283C, U+2834, U+2826, U+2827, U+2807, U+280F.
- Implementations SHOULD advance the spinner at approximately 8 to 12 frames per second while the turn remains active.
- If the terminal, font, or rendering layer cannot reliably display that Braille spinner sequence, the TUI MUST fall back to the ASCII spinner sequence `-`, `\\`, `|`, `/`.
- When no structured progress state is available, the canonical status-chip text is `[⠋ Working...]` or the ASCII-fallback equivalent.
- When structured progress is available, the status-chip label MUST use one of these exact values: `Thinking`, `Calling tool`, or `Waiting for tool result`.
- Canonical examples are `[⠋ Thinking...]`, `[⠋ Calling tool...]`, and `[⠋ Waiting for tool result...]`.
- The in-flight indicator MAY also be mirrored in the status bar, but the assistant-turn indicator remains required.
- When visible text streams incrementally, the TUI MUST append that text into the active in-flight turn below or after the current status indicator instead of creating a second assistant row.
- When hidden thinking is available during streaming, the TUI SHOULD update the same collapsed thinking placeholder block in place rather than emitting raw reasoning text into the visible assistant answer area.
- When the final assistant turn is committed, the TUI MUST remove the transient in-flight indicator and replace the in-flight row with the final ordered assistant turn content.
- Final reconciliation MUST preserve already streamed visible assistant text, MUST keep the final item ordering, and MUST NOT duplicate visible assistant text that was already shown during streaming.
- Final reconciliation MUST discard ephemeral progress-only labels and MUST retain only the final persisted transcript content and any separately rendered structured items.
- If the selected backend path cannot provide true incremental visible-text streaming, the TUI MUST fall back to a degraded in-flight state indicator and then replace that row with the final ordered assistant turn once completion arrives.

## Completion and Discovery

- Spec ID: `CYNAI.CLIENT.CynorkChat.Completion` <a id="spec-cynai-client-cynorkchat-completion"></a>

### Completion and Discovery Traces To

- [REQ-CLIENT-0203](../requirements/client.md#req-client-0203)

The TUI SHOULD provide completion and fuzzy discovery for interactive chat actions.

- Slash-command completion SHOULD work when the composer input begins with `/`.
- Context-sensitive completion SHOULD be available for task identifiers, project identifiers, model identifiers, and thread actions where relevant.
- Completion data MAY be backed by live gateway calls or local cache, but results MUST remain scoped to the authenticated user.
- The TUI MAY provide a command-palette-style overlay for slash commands and common chat actions such as new thread, toggle context pane, search, or queue draft.

## Local File References

- Spec ID: `CYNAI.CLIENT.CynorkChat.AtFileReferences` <a id="spec-cynai-client-cynorkchat-atfilereferences"></a>

### Local File References Traces To

- [REQ-CLIENT-0198](../requirements/client.md#req-client-0198)

The composer MAY support `@` references to local files.

- `@` references are resolved locally at send time.
- If the user references a missing, unreadable, oversized, or disallowed file, the client MUST surface an error and MUST NOT send the message until the user resolves the issue.
- The TUI MAY provide autocomplete or browser-like lookup when the user types `@`.
- After a successful send, the transcript MAY show compact attachment metadata instead of replaying raw local paths.

## Local Config

- Spec ID: `CYNAI.CLIENT.CynorkChat.LocalConfig` <a id="spec-cynai-client-cynorkchat-localconfig"></a>

### Local Config Traces To

- [REQ-CLIENT-0187](../requirements/client.md#req-client-0187)
- [REQ-CLIENT-0211](../requirements/client.md#req-client-0211)

TUI-specific preferences MAY live in the same config directory as the rest of cynork.

- TUI-specific preferences are stored in the same cynork YAML config file described in [cynork_cli.md](cynork_cli.md#spec-cynai-client-cliconfigfilelocation).
- Supported fields MAY include default model, preferred composer mode, context-pane default visibility, queued-draft behavior, thinking-toggle default, and keybinding overrides.
- Supported fields MAY also include context-pane width and queued-draft send mode when those behaviors are implemented.
- The canonical first-pass key for persisted thinking visibility is `tui.show_thinking_by_default` (boolean).
- When `tui.show_thinking_by_default` is `true`, newly started interactive TUI or chat sessions MUST expand retained thinking blocks by default.
- When `tui.show_thinking_by_default` is `false` or absent, newly started interactive TUI or chat sessions MUST start in the collapsed-thinking presentation.
- `/show-thinking` MUST update `tui.show_thinking_by_default` to `true` in the local config and apply that change immediately to the current session.
- `/hide-thinking` MUST update `tui.show_thinking_by_default` to `false` in the local config and apply that change immediately to the current session.
- TUI-specific config MUST NOT store secrets, passwords, tokens, or message content.
- When written, TUI config SHOULD follow the same atomic-write and permission rules as the main cynork config.

## Local Cache

- Spec ID: `CYNAI.CLIENT.CynorkChat.LocalCache` <a id="spec-cynai-client-cynorkchat-localcache"></a>

### Local Cache Traces To

- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188)

The TUI MAY cache lightweight metadata that improves responsiveness of completion and history panes.

- Cacheable data includes task ids or names, project ids or titles, model ids, and thread-list metadata.
- Cacheable data MAY also include lightweight context-pane data and completion lists needed for slash-command or `@` lookup responsiveness.
- The cache MUST NOT store secrets or chat transcript content.
- Cache entries SHOULD define bounded age and SHOULD be invalidated after relevant mutating actions.

## Auth Recovery

- Spec ID: `CYNAI.CLIENT.CynorkChat.AuthRecovery` <a id="spec-cynai-client-cynorkchat-authrecovery"></a>

### Auth Recovery Traces To

- [REQ-CLIENT-0190](../requirements/client.md#req-client-0190)

The TUI MUST support interactive login recovery at startup and during the session.

- When startup token resolution fails, the TUI MUST offer an in-session login prompt instead of forcing an immediate external restart.
- The startup login prompt SHOULD allow username entry, secret-safe password entry, and explicit cancel or abort.
- When startup login is cancelled or fails, the TUI MUST exit with the normal auth failure outcome rather than entering a degraded chat session.
- When a gateway request fails with an auth error during the session, the TUI MUST allow re-authentication and SHOULD offer to retry the interrupted action once.
- After successful in-session re-authentication, the TUI SHOULD resume the same session state and return focus to the interrupted chat flow rather than forcing a full restart.
- Secret input MUST use secure input behavior and MUST NOT be echoed or persisted in transcript history.

## Web Login

- Spec ID: `CYNAI.CLIENT.CliWebLogin` <a id="spec-cynai-client-cliweblogin"></a>

### Web Login Traces To

- [REQ-CLIENT-0191](../requirements/client.md#req-client-0191)

The CLI SHOULD support a web-based login flow suitable for SSO-capable deployments.

- Acceptable patterns include browser-based authorization and device-code style login.
- Browser-based login SHOULD either open the system browser automatically or print a copyable authorization URL and bounded-lifetime code when automatic browser launch is unavailable.
- Device-code style login SHOULD show the verification URL, user code, and expiry or timeout information needed to complete the flow.
- The flow MUST not print tokens to stdout in normal operation.
- Resulting tokens MUST be stored using the same cynork token-storage model and file-permission rules as other auth flows.
