@suite_cynork
Feature: cynork TUI thread management and connection recovery

  As a user of the cynork TUI
  I want reliable thread management and automatic connection recovery
  So that my chat session persists across reconnections and I can navigate threads

Background:

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0181
@spec_cynai_client_cynorktui_entrypoint
@spec_cynai_client_clichatthreadcontrols
Scenario: TUI starts with a new thread by default
  Given the mock gateway supports POST "/v1/chat/threads"
  # Non-interactive BDD has no controlling TTY; `cynork chat --thread-new` shares the same thread-creation path as the TUI entrypoint.
  When I run cynork chat without resume-thread and send a first message
  Then cynork creates a fresh chat thread before the first completion
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
Scenario: TUI displays current thread title
  Given the TUI is running with a current thread that has a title or fallback label
  When I view the TUI status bar or thread display
  Then the current thread title or fallback label is visible
  And the TUI updates the displayed thread title after "/thread rename"
  And the TUI updates the display to show that thread's title or fallback label when I switch threads
