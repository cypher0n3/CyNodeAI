@suite_cynork
Feature: cynork TUI /skills slash commands

  As a user of the cynork TUI
  I want to list, get, load, update, and delete skills via slash commands
  So that I can manage AI skills without leaving the chat surface

## Background

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0170
@req_client_0207
@spec_cynai_client_cynorktui_skillslashcommands
Scenario: /skills list shows skills inline
  Given the TUI is running and the gateway supports skills list
  When I type "/skills list" and press Enter
  Then the scrollback shows skill list output
  And the TUI session remains active

@req_client_0170
@req_client_0207
@spec_cynai_client_cynorktui_skillslashcommands
Scenario: /skills get with skill_selector shows skill details inline
  Given the TUI is running and the gateway has a skill with selector "team-guide"
  When I type "/skills get team-guide" and press Enter
  Then the scrollback shows skill details for that selector
  And the TUI session remains active

@req_client_0170
@req_client_0207
@spec_cynai_client_cynorktui_skillslashcommands
Scenario: /skills load with file path loads skill and shows result inline
  Given the TUI is running and a markdown file "tmp/skill.md" exists with content "# Test skill"
  And the gateway supports skills load
  When I type "/skills load tmp/skill.md" and press Enter
  Then the scrollback shows load success or skill id
  And the TUI session remains active

@req_client_0170
@req_client_0207
@spec_cynai_client_cynorktui_skillslashcommands
Scenario: /skills update with selector and file updates skill and shows result inline
  Given the TUI is running and the gateway has a skill with selector "team-guide"
  And a markdown file "tmp/skill-v2.md" exists with updated content
  When I type "/skills update team-guide tmp/skill-v2.md" and press Enter
  Then the scrollback shows update success or an inline error
  And the TUI session remains active

@req_client_0170
@req_client_0207
@spec_cynai_client_cynorktui_skillslashcommands
Scenario: /skills delete with skill_selector deletes skill and shows result inline
  Given the TUI is running and the gateway has a skill with selector "team-guide"
  When I type "/skills delete team-guide" and press Enter
  Then the scrollback shows delete success or an inline error
  And the TUI session remains active

@req_client_0170
@req_client_0207
@spec_cynai_client_cynorktui_skillslashcommands
Scenario: Ambiguous skill_selector shows concise error and keeps session active
  Given the TUI is running and the gateway has multiple skills matching "foo"
  When I type "/skills get foo" and press Enter
  Then the scrollback shows an ambiguity error or asks to disambiguate
  And the TUI session remains active
