@suite_cynork
Feature: cynork TUI /prefs slash commands

  As a user of the cynork TUI
  I want to list, get, set, delete, and view effective preferences via slash commands
  So that I can manage preferences without leaving the chat surface

Background:

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0169
@req_client_0207
@spec_cynai_client_cynorktui_preferenceslashcommands
Scenario: /prefs list shows preferences inline
  Given the TUI is running and the gateway supports prefs list
  When I type "/prefs list" and press Enter
  Then the scrollback shows preference list output
  And the TUI session remains active

@req_client_0169
@req_client_0207
@spec_cynai_client_cynorktui_preferenceslashcommands
Scenario: /prefs get shows preference value inline
  Given the TUI is running and the gateway supports prefs get
  When I type "/prefs get user output.summary_style" and press Enter
  Then the scrollback shows the preference value or an inline error
  And the TUI session remains active

@req_client_0169
@req_client_0207
@spec_cynai_client_cynorktui_preferenceslashcommands
Scenario: /prefs set updates preference and shows result inline
  Given the TUI is running and the gateway supports prefs set
  When I type "/prefs set user output.summary_style concise" and press Enter
  Then the scrollback shows success or an inline error
  And the TUI session remains active

@req_client_0169
@req_client_0207
@spec_cynai_client_cynorktui_preferenceslashcommands
Scenario: /prefs delete removes preference and shows result inline
  Given the TUI is running and the gateway supports prefs delete
  When I type "/prefs delete user output.summary_style" and press Enter
  Then the scrollback shows success or an inline error
  And the TUI session remains active

@req_client_0169
@req_client_0207
@spec_cynai_client_cynorktui_preferenceslashcommands
Scenario: /prefs effective shows effective preferences inline
  Given the TUI is running and the gateway supports prefs effective
  When I type "/prefs effective" and press Enter
  Then the scrollback shows effective preferences output
  And the TUI session remains active

@req_client_0207
@spec_cynai_client_cynorktui_preferenceslashcommands
Scenario: Prefs slash command failure shows error inline and keeps session active
  Given the TUI is running and the gateway returns an error for the next prefs request
  When I type "/prefs get invalid scope.key" and press Enter
  Then the scrollback shows an inline error
  And the TUI session remains active
