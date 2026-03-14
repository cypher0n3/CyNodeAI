@suite_cynork
Feature: cynork TUI shell escape

  As a user of the cynork TUI
  I want to run shell-escape commands from the composer
  So that I can run shell utilities without leaving the chat surface

Background:
  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0175
@req_client_0176
@spec_cynai_client_clichatshellescape
Scenario: Shell escape runs a command and shows output inline
  Given the TUI is running
  When I type "! date +%Y-%m-%d" and press Enter
  Then the scrollback contains a date-formatted string
  And the TUI session remains active

@req_client_0175
@spec_cynai_client_clichatshellescape
Scenario: Shell escape without space after bang runs the command
  Given the TUI is running
  When I type "!pwd" and press Enter
  Then the scrollback contains a filesystem path
  And the TUI session remains active

@req_client_0175
@spec_cynai_client_clichatshellescape
Scenario: Shell escape shows stderr output inline
  Given the TUI is running
  When I type "! ls /no_such_directory_xyz" and press Enter
  Then the scrollback contains an error referencing the missing path
  And the TUI session remains active

@req_client_0175
@spec_cynai_client_clichatshellescape
Scenario: Shell escape with empty command shows a usage hint
  Given the TUI is running
  When I type "!" and press Enter
  Then the scrollback contains a shell usage hint
  And the TUI session remains active

@req_client_0175
@spec_cynai_client_clichatshellescape
Scenario: Shell escape with whitespace-only command shows a usage hint
  Given the TUI is running
  When I type "!   " and press Enter
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

@req_client_0175
@req_client_0176
@spec_cynai_client_clichatshellescape
Scenario: Shell escape command not found shows error without exiting
  Given the TUI is running
  When I type "! __no_such_cmd_xyz__" and press Enter
  Then the scrollback contains an error message
  And the TUI session remains active

@req_client_0176
@spec_cynai_client_clichatsubcommanderrors
Scenario: Shell escape failure does not show top-level Usage
  Given the TUI is running
  When I type "! exit 1" and press Enter
  Then the scrollback does not contain "Usage"
  And the TUI session remains active

@req_client_0189
@spec_cynai_client_clichatshellescape
Scenario: Shell escape with interactive subprocess restores TUI cleanly
  Given the TUI is running
  When I type "! true" and press Enter
  Then the TUI session remains active
  And the composer has focus
