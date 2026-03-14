@suite_cynork
Feature: cynork TUI /status and /auth slash commands

  As a user of the cynork TUI
  I want to check gateway status and run auth commands via slash commands
  So that I can manage connectivity and identity without leaving the chat surface

Background:
  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0167
@req_client_0176
@req_client_0207
@spec_cynai_client_cynorktui_statusslashcommands
Scenario: /status shows gateway reachability
  Given the TUI is running
  When I type "/status" and press Enter
  Then the scrollback shows reachability or status output consistent with "cynork status"
  And the TUI session remains active

@req_client_0167
@req_client_0207
@spec_cynai_client_cynorktui_statusslashcommands
Scenario: /whoami shows current identity
  Given the TUI is running
  When I type "/whoami" and press Enter
  Then the scrollback shows identity output consistent with "cynork auth whoami"
  And the TUI session remains active

@req_client_0167
@req_client_0207
@spec_cynai_client_cynorktui_statusslashcommands
Scenario: /auth whoami shows current identity
  Given the TUI is running
  When I type "/auth whoami" and press Enter
  Then the scrollback shows identity output consistent with "cynork auth whoami"
  And the TUI session remains active

@req_client_0176
@req_client_0207
@spec_cynai_client_cynorktui_statusslashcommands
Scenario: /auth logout clears session and shows result inline
  Given the TUI is running
  When I type "/auth logout" and press Enter
  Then the scrollback shows logout success or confirmation
  And the TUI session remains active unless the flow explicitly exits

@req_client_0207
@spec_cynai_client_cynorktui_statusslashcommands
Scenario: /auth refresh renews session and shows result inline
  Given the TUI is running
  When I type "/auth refresh" and press Enter
  Then the scrollback shows refresh success or an inline error
  And the TUI session remains active

@req_client_0176
@spec_cynai_client_cynorktui_statusslashcommands
Scenario: Auth command failure shows error inline and keeps session active
  Given the TUI is running and the gateway will return an auth error for the next auth request
  When I type "/auth whoami" and press Enter
  Then the scrollback shows an inline error
  And the TUI session remains active
