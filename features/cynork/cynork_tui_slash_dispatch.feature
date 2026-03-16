@suite_cynork
Feature: cynork TUI slash command dispatch and help

  As a user of the cynork TUI
  I want slash commands to be dispatched locally and discoverable via /help
  So that I can control the session without leaving the chat surface

## Background

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0164
@req_client_0165
@spec_cynai_client_cynorktui_localslashcommands
@spec_cynai_client_cynorktui_slashcommandexecution
Scenario: /help lists available slash commands
  Given the TUI is running
  When I type "/help" and press Enter
  Then the scrollback contains a list of slash command names and their descriptions

@req_client_0164
@spec_cynai_client_cynorktui_localslashcommands
@spec_cynai_client_cynorktui_slashcommandexecution
Scenario: /help includes model project thread and task command groups
  Given the TUI is running
  When I type "/help" and press Enter
  Then the scrollback contains references to model project thread and task commands

@req_client_0164
@req_client_0207
@spec_cynai_client_cynorktui_localslashcommands
@spec_cynai_client_cynorktui_slashcommandexecution
Scenario: /help includes status auth nodes prefs and skills command groups
  Given the TUI is running
  When I type "/help" and press Enter
  Then the scrollback contains references to status whoami auth nodes prefs and skills commands

@req_client_0165
@spec_cynai_client_cynorktui_slashcommandexecution
Scenario: Case-insensitive dispatch works for slash commands
  Given the TUI is running
  When I type "/HELP" and press Enter
  Then the scrollback contains a list of slash command names and their descriptions
  And the TUI session remains active

@req_client_0164
@req_client_0165
@spec_cynai_client_cynorktui_slashcommandexecution
Scenario: Unknown slash command shows a help hint without exiting
  Given the TUI is running
  When I type "/notacommand" and press Enter
  Then the scrollback contains a hint mentioning "/help"
  And the TUI session remains active

@req_client_0165
@spec_cynai_client_cynorktui_slashcommandexecution
Scenario: Unknown slash command is not sent to the PM model as chat text
  Given the TUI is running
  When I type "/notacommand" and press Enter
  Then no chat completion request was sent for that line
  And the TUI session remains active

@req_client_0176
@spec_cynai_client_cynorktui_slashcommandexecution
Scenario: Slash command failure does not terminate the session
  Given the TUI is running
  When I type "/connect <https://unreachable.invalid>" and press Enter
  Then the scrollback shows an error or connectivity failure
  And the TUI session remains active
