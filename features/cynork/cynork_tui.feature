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

@req_client_0199
@req_client_0200
@spec_cynai_client_cynorkchat_tuilayout
Scenario: TUI exposes thread history and rename controls
  Given the mock gateway returns multiple chat threads
  When I open the thread history pane in the TUI
  Then I can see recent threads for the current user
  And I can rename the selected thread

@req_client_0192
@req_client_0195
@spec_cynai_client_cynorktui_transcriptrendering
Scenario: Thinking content is hidden by default in the TUI
  Given the gateway returns a structured assistant turn with visible text and thinking
  When the TUI renders the assistant turn
  Then the visible text is shown in the transcript
  And the thinking content is collapsed behind a compact placeholder

@req_client_0185
@spec_cynai_client_cynorktui_generationstate
Scenario: TUI shows an agent-working indicator during an in-flight turn
  Given the gateway is still generating the current assistant turn
  When the TUI renders the in-flight turn
  Then the TUI shows a visible in-flight indicator attached to the active assistant turn
  And the indicator is rendered as a distinct status chip rather than bare transcript text
  And the indicator shows the label "Working" when no structured progress state is available

@req_client_0198
@spec_cynai_client_cynorkchat_atfilereferences
Scenario: TUI validates @ file references before sending
  When I compose a message with an @ file reference
  And the referenced file is missing
  Then the TUI shows a validation error
  And the message is not sent

@req_client_0196
@spec_cynai_client_cynorkchat_tuilayout
Scenario: Queued drafts can be reordered and explicitly sent later
  Given the TUI has two queued drafts
  When I reorder the queued drafts
  And I choose to send only the first queued draft
  Then the queued drafts remain distinct from sent transcript messages
  And the unsent queued draft remains available for later edit or send

@req_client_0190
@spec_cynai_client_cynorkchat_authrecovery
Scenario: TUI prompts for re-authentication and retries the interrupted action
  Given the TUI is running with an expired login token
  When a chat request returns an authorization error
  And I complete the in-session login prompt successfully
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
