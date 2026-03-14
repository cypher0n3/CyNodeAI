@suite_cynork
Feature: cynork TUI

  As a user of the cynork CLI
  I want a full-screen chat TUI with thread history and structured rendering
  So that interactive chat is readable, stateful, and efficient

Background:
  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0197
@req_client_0202
@spec_cynai_client_cynorktui_entrypoint
Scenario: Explicit TUI entrypoint starts the interactive chat surface
  When I run cynork tui
  Then the full-screen chat TUI starts

@req_client_0181
@spec_cynai_client_cynorktui_entrypoint
@spec_cynai_client_clichatthreadcontrols
Scenario: TUI starts with a new thread by default
  Given the mock gateway supports POST "/v1/chat/threads"
  When I run cynork tui without resume-thread
  Then the TUI creates a new chat thread before the first completion
  And the session uses that new thread for subsequent turns

@req_client_0181
@spec_cynai_client_cynorktui_entrypoint
@spec_cynai_client_clichatthreadcontrols
Scenario: TUI resume-thread starts the session in an existing thread
  Given the mock gateway returns at least one chat thread with selector "inbox"
  When I run cynork tui with resume-thread "inbox"
  Then the TUI session starts in the thread identified by selector "inbox"
  And the first completion continues that thread's conversation

@req_client_0213
@spec_cynai_client_cynorktui_connectionrecovery
Scenario: Connection interrupted mid-stream triggers auto-reconnect and preserves session
  Given the TUI has sent a message and the gateway is streaming the assistant response
  When the connection to the gateway is interrupted before the stream completes
  Then the TUI attempts to auto-reconnect with bounded backoff
  And after reconnection the TUI retains any already-received visible text in the transcript
  And the in-flight turn is marked as interrupted or shows a clear indicator
  And the current thread and session are preserved
  And I can continue the session without restarting the TUI

@req_client_0199
@req_client_0200
@req_client_0210
@spec_cynai_client_cynorkchat_tuilayout
Scenario: TUI exposes thread history and rename controls
  Given the mock gateway returns multiple chat threads
  When I open the thread history pane in the TUI
  Then I can see recent threads for the current user
  And each visible thread shows a user-typeable thread selector
  And I can rename the selected thread

@req_client_0212
@spec_cynai_client_cynorkchat_tuilayout
Scenario: TUI displays current thread title and updates when title or thread changes
  Given the TUI is running with a current thread that has a title or fallback label
  When I view the TUI status bar or thread display
  Then the current thread title or fallback label is visible
  When the thread title changes after auto-title or "/thread rename"
  Then the TUI updates the displayed thread title
  When I switch to a different thread
  Then the TUI updates the display to show that thread's title or fallback label

@req_client_0192
@req_client_0195
@spec_cynai_client_cynorktui_transcriptrendering
Scenario: Thinking content is hidden by default in the TUI
  Given the gateway returns a structured assistant turn with visible text and thinking
  When the TUI renders the assistant turn
  Then the visible text is shown in the transcript
  And the thinking content is collapsed behind a compact placeholder
  And the collapsed placeholder remains visibly distinct from normal assistant prose
  And the collapsed placeholder hints that "/show-thinking" reveals the thinking content

@req_client_0208
@req_client_0211
@spec_cynai_client_cynorktui_localslashcommands
@spec_cynai_client_cynorktui_transcriptrendering
Scenario: TUI show-thinking expands retained thinking blocks
  Given the current transcript contains loaded assistant turns with retained thinking
  And older retrieved transcript history also contains retained thinking
  When I issue "/show-thinking" in the TUI
  Then retained thinking is expanded for the loaded assistant turns
  And retained thinking is expanded for the older retrieved assistant turns
  And the local cynork YAML config stores `tui.show_thinking_by_default` as true

@req_client_0208
@req_client_0211
@spec_cynai_client_cynorktui_localslashcommands
@spec_cynai_client_cynorktui_transcriptrendering
Scenario: TUI hide-thinking restores collapsed retained thinking placeholders
  Given the current transcript contains expanded retained thinking blocks
  And older retrieved transcript history also contains expanded retained thinking
  When I issue "/hide-thinking" in the TUI
  Then those retained thinking blocks return to collapsed placeholders
  And the collapsed placeholders remain visible with a "/show-thinking" hint
  And the local cynork YAML config stores `tui.show_thinking_by_default` as false

@req_client_0211
@spec_cynai_client_cynorkchat_localconfig
Scenario: TUI loads persisted thinking visibility on future startup
  Given the local cynork YAML config stores `tui.show_thinking_by_default` as true
  When I start a new cynork TUI session
  Then retained thinking blocks are expanded by default in that new session

@req_client_0185
@req_client_0209
@spec_cynai_client_cynorktui_generationstate
Scenario: TUI shows an agent-working indicator during an in-flight turn
  Given the gateway is still generating the current assistant turn
  When the TUI renders the in-flight turn
  Then the TUI shows a visible in-flight indicator attached to the active assistant turn
  And the indicator is rendered as a distinct status chip rather than bare transcript text
  And the indicator shows the label "Working" when no structured progress state is available

@req_client_0209
@spec_cynai_client_cynorktui_generationstate
@spec_cynai_usrgwy_openaichatapi_streaming
Scenario: TUI requests streaming output by default and updates one in-flight turn progressively
  Given the gateway supports stream=true and emits ordered incremental assistant text updates
  When I send a normal interactive chat turn from the TUI
  Then the TUI requests streaming output by default
  And visible assistant text is appended progressively within one in-flight assistant turn
  And the final assistant turn replaces the in-flight row without duplicating visible text

@req_client_0198
@spec_cynai_client_cynorkchat_atfilereferences
Scenario: TUI validates @ file references before sending
  When I compose a message with an @ file reference and the referenced file is missing
  Then the TUI shows a validation error
  And the message is not sent

@req_client_0196
@spec_cynai_client_cynorkchat_tuilayout
Scenario: Queued drafts can be reordered and explicitly sent later
  Given the TUI has two queued drafts
  When I reorder the queued drafts and choose to send only the first queued draft
  Then the queued drafts remain distinct from sent transcript messages
  And the unsent queued draft remains available for later edit or send

@req_client_0190
@spec_cynai_client_cynorkchat_authrecovery
Scenario: TUI prompts for re-authentication and retries the interrupted action
  Given the TUI is running with an expired login token
  When a chat request returns an authorization error and I complete the in-session login prompt successfully
  Then the TUI offers to retry the interrupted action once
  And the session continues without restarting the TUI

@req_client_0191
@spec_cynai_client_cliweblogin
Scenario: Web login shows bounded authorization details without printing a token
  When I start the web login flow from the CLI
  Then the CLI shows a browser URL or device-code verification URL
  And the CLI shows the login expiry or timeout
  And the CLI does not print an access token

@req_client_0205
@spec_cynai_client_cynorkchat_tuilayout
Scenario: Mouse wheel scrolls transcript history rather than composer history
  Given the TUI shows enough transcript output to scroll
  When I scroll with the mouse wheel
  Then the visible transcript history moves
  And the composer history selection does not change

@req_client_0206
@spec_cynai_client_cynorkchat_tuilayout
Scenario: TUI shows a composer hint for commands files and shell
  When the TUI renders an idle composer
  Then the TUI shows "/ commands · @ files · ! shell" in or adjacent to the composer

@req_client_0204
@spec_cynai_client_cynorkchat_tuilayout
Scenario: Focused composer shows a visible insertion cursor
  Given the composer has focus
  When the TUI renders the composer
  Then the composer shows a visible text cursor or caret at the current insertion point

@req_client_0204
@req_usrgwy_0150
@spec_cynai_client_cynorkchat_tuilayout
@spec_cynai_usrgwy_openaichatapi_streaming
Scenario: User cancellation stops the active stream and reconciles the in-flight turn
  Given the TUI has sent a message and the gateway is streaming the assistant response
  When I send Ctrl+C or otherwise cancel the active stream
  Then the TUI treats the stream as canceled
  And any already-received visible text is retained in the transcript
  And the in-flight turn is reconciled deterministically without duplicating content
  And the session remains active unless I explicitly exit

@req_client_0209
@spec_cynai_client_cynorktui_generationstate
Scenario: TUI degrades to in-flight indicator when backend cannot stream visible-text deltas
  Given the selected backend path does not support true incremental visible-text streaming
  When I send a normal interactive chat turn from the TUI
  Then the TUI shows a degraded in-flight state indicator
  And when the final response arrives the TUI replaces that row with the final assistant turn
  And visible assistant text is not duplicated

@req_client_0202
@spec_cynai_client_cynorktui_entrypoint
Scenario: TUI remains coherent for both completions and responses chat surfaces
  Given the gateway supports both POST "/v1/chat/completions" and POST "/v1/responses"
  When I use the TUI with the completions surface
  Then thread state, transcript, and session behavior are coherent
  When I use the TUI with the responses surface
  Then thread state, transcript, and session behavior are coherent
  And the user-visible chat contract does not diverge between the two surfaces
