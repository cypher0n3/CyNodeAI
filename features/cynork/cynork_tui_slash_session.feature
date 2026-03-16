@suite_cynork
Feature: cynork TUI slash commands for session and display

  As a user of the cynork TUI
  I want to clear the view, see version, and exit via slash commands
  So that I can control the session display and end the session cleanly

## Background

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0164
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /clear resets the visible scrollback
  Given the TUI has existing messages in the scrollback
  When I type "/clear" and press Enter
  Then the scrollback is empty

@req_client_0164
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /clear does not mutate persisted chat history
  Given the TUI has existing messages in the scrollback from a thread
  When I type "/clear" and press Enter
  Then the scrollback is empty
  And the thread history is still available after reload or switch

@req_client_0164
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /clear preserves local session state
  Given the TUI is running with model "cynodeai.pm" and project "proj-abc"
  And the TUI has existing messages in the scrollback
  When I type "/clear" and press Enter
  Then the scrollback is empty
  And the session model and project context are unchanged

@req_client_0164
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /version shows the cynork version string
  Given the TUI is running
  When I type "/version" and press Enter
  Then the scrollback contains the cynork version string

@req_client_0164
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /version output matches cynork version non-interactive output
  Given the TUI is running
  When I type "/version" and press Enter
  Then the scrollback contains the same version string as "cynork version"

@req_client_0164
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /exit ends the TUI session
  Given the TUI is running
  When I type "/exit" and press Enter
  Then the TUI exits cleanly

@req_client_0164
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /quit is a synonym for /exit
  Given the TUI is running
  When I type "/quit" and press Enter
  Then the TUI exits cleanly

@req_client_0183
@req_client_0195
@req_client_0208
@req_client_0211
@spec_cynai_client_cynorktui_localslashcommands
@spec_cynai_client_cynorktui_thinkingvisibilitybehavior
Scenario: /show-thinking expands retained thinking blocks in the transcript
  Given the TUI is running with transcript containing assistant turns that have retained thinking parts
  When I type "/show-thinking" and press Enter
  Then retained thinking parts in the scrollback are displayed as expanded thinking blocks
  And the TUI session remains active

@req_client_0183
@req_client_0195
@req_client_0208
@req_client_0211
@spec_cynai_client_cynorktui_localslashcommands
@spec_cynai_client_cynorktui_thinkingvisibilitybehavior
Scenario: /hide-thinking collapses retained thinking to placeholders
  Given the TUI is running with transcript containing expanded thinking blocks
  When I type "/hide-thinking" and press Enter
  Then retained thinking parts are shown as collapsed placeholders
  And placeholders hint "/show-thinking" as the expand action
  And the TUI session remains active

@req_client_0211
@spec_cynai_client_cynorktui_localslashcommands
@spec_cynai_client_cynorktui_thinkingvisibilitybehavior
Scenario: /show-thinking persists tui.show_thinking_by_default to config
  Given the TUI is running
  When I type "/show-thinking" and press Enter
  Then the local cynork config has "tui.show_thinking_by_default" set to true

@req_client_0211
@spec_cynai_client_cynorktui_localslashcommands
@spec_cynai_client_cynorktui_thinkingvisibilitybehavior
Scenario: /hide-thinking persists tui.show_thinking_by_default to config
  Given the TUI is running
  When I type "/hide-thinking" and press Enter
  Then the local cynork config has "tui.show_thinking_by_default" set to false

@req_client_0183
@spec_cynai_client_cynorktui_thinkingvisibilitybehavior
Scenario: /show-thinking does not alter assistant turns that have no retained thinking
  Given the TUI is running with transcript containing only assistant turns that have no retained thinking part
  When I type "/show-thinking" and press Enter
  Then those turns are unchanged

@req_client_0183
@spec_cynai_client_cynorktui_thinkingvisibilitybehavior
Scenario: /hide-thinking does not alter assistant turns that have no retained thinking
  Given the TUI is running with transcript containing only assistant turns that have no retained thinking part
  When I type "/hide-thinking" and press Enter
  Then those turns are unchanged
