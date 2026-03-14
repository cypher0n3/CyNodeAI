@suite_cynork
Feature: cynork TUI /thread slash commands

  As a user of the cynork TUI
  I want to create, list, switch, and rename threads via slash commands
  So that I can manage conversation threads without leaving the chat surface

Background:
  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0181
@req_client_0199
@req_client_0200
@req_client_0207
@spec_cynai_client_cynorktui_threadslashcommands
Scenario: /thread new creates a new thread and sets it as current
  Given the TUI is running and the mock gateway supports POST "/v1/chat/threads"
  When I type "/thread new" and press Enter
  Then the scrollback shows a success indicator or the new thread
  And the current session uses the new thread for subsequent turns
  And the TUI session remains active

@req_client_0199
@req_client_0200
@req_client_0207
@spec_cynai_client_cynorktui_threadslashcommands
Scenario: /thread list shows recent threads with user-typeable selectors
  Given the TUI is running and the mock gateway returns multiple chat threads
  When I type "/thread list" and press Enter
  Then the scrollback shows thread list output recent-first
  And each visible thread includes a user-typeable thread selector
  And the current thread is unchanged
  And the TUI session remains active

@req_client_0199
@req_client_0200
@req_client_0207
@spec_cynai_client_cynorktui_threadslashcommands
Scenario: /thread switch with selector changes current thread and reloads transcript
  Given the TUI is running and the mock gateway returns at least one thread with selector "inbox"
  When I type "/thread switch inbox" and press Enter
  Then the current thread is set to the thread identified by "inbox"
  And the transcript view shows that thread's history
  And the TUI session remains active

@req_client_0207
@spec_cynai_client_cynorktui_threadslashcommands
Scenario: /thread switch with unknown selector shows error and keeps current thread
  Given the TUI is running and the mock gateway returns a known thread list
  When I type "/thread switch no-such-thread" and press Enter
  Then the scrollback shows a concise error
  And the current thread is unchanged
  And the TUI session remains active

@req_client_0200
@req_client_0207
@spec_cynai_client_cynorktui_threadslashcommands
Scenario: /thread rename updates the current thread title
  Given the TUI is running with a current thread
  And the mock gateway supports thread title update
  When I type "/thread rename My New Title" and press Enter
  Then the scrollback shows success or the updated title
  And the TUI session displays the new thread title
  And the TUI session remains active

@req_client_0207
@spec_cynai_client_cynorktui_threadslashcommands
Scenario: Unknown /thread subcommand shows usage error and keeps session active
  Given the TUI is running
  When I type "/thread invalid" and press Enter
  Then the scrollback shows a concise usage error
  And the TUI session remains active
