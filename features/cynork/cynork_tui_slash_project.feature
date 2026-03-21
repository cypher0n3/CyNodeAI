@suite_cynork
Feature: cynork TUI /project slash commands

  As a user of the cynork TUI
  I want to view and set project context via slash commands
  So that chat requests use the correct project without leaving the chat surface

Background:

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0173
@spec_cynai_client_cynorktui_projectslashcommands
Scenario: /project with no argument shows current project context
  Given the TUI is running with project "proj-abc"
  When I type "/project" and press Enter
  Then the scrollback contains the current project identifier

@req_client_0173
@spec_cynai_client_cynorktui_projectslashcommands
Scenario: /project set with id updates the session project
  Given the TUI is running
  When I type "/project set proj-xyz" and press Enter
  Then the session project is updated to "proj-xyz"

@req_client_0173
@spec_cynai_client_cynorktui_projectslashcommands
Scenario: /project list shows available projects
  Given the TUI is running and the gateway supports project listing
  When I type "/project list" and press Enter
  Then the scrollback shows project identifiers or project list output
  And the TUI session remains active

@req_client_0173
@spec_cynai_client_cynorktui_projectslashcommands
Scenario: /project get with id shows project details
  Given the TUI is running and the gateway supports project get
  When I type "/project get proj-abc" and press Enter
  Then the scrollback shows project details for "proj-abc"
  And the TUI session remains active

@req_client_0173
@spec_cynai_client_cynorktui_projectslashcommands
Scenario: /project with bare id updates context as shorthand
  Given the TUI is running
  When I type "/project proj-bare" and press Enter
  Then the session project is updated to "proj-bare"

@req_client_0173
@spec_cynai_client_cynorktui_projectslashcommands
Scenario: /project set causes subsequent chat requests to use OpenAI-Project header
  Given the TUI is running
  When I type "/project set proj-xyz" and press Enter and then send a chat message from the composer
  Then the chat request included OpenAI-Project header for "proj-xyz"

@req_client_0173
@spec_cynai_client_cynorktui_projectslashcommands
Scenario: Clearing project context removes explicit override
  Given the TUI is running with project "proj-abc"
  When I clear the project context via the accepted form
  Then the session has no explicit project override
  And subsequent chat requests do not send OpenAI-Project for that session
