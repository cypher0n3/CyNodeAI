@suite_cynork
Feature: cynork TUI slash commands and shell escape

  As a user of the cynork TUI
  I want to run slash commands and shell-escape commands from the composer
  So that I can control the session and run shell utilities without leaving the chat surface

Background:
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
Scenario: /clear resets the visible scrollback
  Given the TUI has existing messages in the scrollback
  When I type "/clear" and press Enter
  Then the scrollback is empty

@req_client_0164
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /version shows the cynork version string
  Given the TUI is running
  When I type "/version" and press Enter
  Then the scrollback contains the cynork version string

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

@req_client_0164
@req_client_0165
@spec_cynai_client_cynorktui_slashcommandexecution
Scenario: Unknown slash command shows a help hint without exiting
  Given the TUI is running
  When I type "/notacommand" and press Enter
  Then the scrollback contains a hint mentioning "/help"
  And the TUI session remains active

@req_client_0171
@req_client_0172
@spec_cynai_client_cynorktui_modelslashcommands
Scenario: /models lists available models
  Given the TUI is running
  When I type "/models" and press Enter
  Then the scrollback shows model identifiers or an inline error

@req_client_0171
@spec_cynai_client_cynorktui_modelslashcommands
Scenario: /model with no argument shows current model selection
  Given the TUI is running with model "cynodeai.pm"
  When I type "/model" and press Enter
  Then the scrollback contains the current model name

@req_client_0171
@spec_cynai_client_cynorktui_modelslashcommands
Scenario: /model <id> updates the session model
  Given the TUI is running
  When I type "/model test-model-v2" and press Enter
  Then the session model is updated to "test-model-v2"

@req_client_0173
@spec_cynai_client_cynorktui_projectslashcommands
Scenario: /project with no argument shows current project context
  Given the TUI is running with project "proj-abc"
  When I type "/project" and press Enter
  Then the scrollback contains the current project identifier

@req_client_0173
@spec_cynai_client_cynorktui_projectslashcommands
Scenario: /project set <id> updates the session project
  Given the TUI is running
  When I type "/project set proj-xyz" and press Enter
  Then the session project is updated to "proj-xyz"

@req_client_0175
@req_client_0176
@spec_cynai_client_clichatshellescape
Scenario: Shell escape runs a command and shows output inline
  Given the TUI is running
  When I type "! echo hello_from_shell" and press Enter
  Then the scrollback contains "hello_from_shell"
  And the TUI session remains active

@req_client_0175
@spec_cynai_client_clichatshellescape
Scenario: Shell escape with empty command shows a usage hint
  Given the TUI is running
  When I type "!" and press Enter
  Then the scrollback contains a shell usage hint
  And the TUI session remains active

@req_client_0175
@req_client_0176
@spec_cynai_client_clichatshellescape
Scenario: Shell escape non-zero exit shows exit code inline
  Given the TUI is running
  When I type "! exit 42" and press Enter
  Then the scrollback contains the exit code
  And the TUI session remains active
