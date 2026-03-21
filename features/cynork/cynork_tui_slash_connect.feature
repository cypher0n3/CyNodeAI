@suite_cynork
Feature: cynork TUI /connect slash command

  As a user of the cynork TUI
  I want to view and change the session gateway via /connect
  So that I can switch backends without leaving the chat surface

Background:

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0214
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /connect with URL updates session gateway
  Given the TUI is running with gateway "<https://gateway.example.com>"
  When I type "/connect <https://other.example.com>" and press Enter
  Then the session gateway is updated to "<https://other.example.com>"
  And the scrollback shows the new gateway or a success indicator
  And the TUI session remains active

@req_client_0214
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /connect without URL shows current gateway
  Given the TUI is running with gateway "<https://gateway.example.com>"
  When I type "/connect" and press Enter
  Then the scrollback shows the current gateway URL or "<https://gateway.example.com>"
  And the TUI session remains active

@req_client_0214
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /connect validates connectivity before continuing
  Given the TUI is running with gateway "<https://gateway.example.com>"
  And the mock gateway exposes GET "/healthz"
  When I type "/connect <https://other.example.com>" and press Enter
  Then the client attempted to validate connectivity to the new gateway
  And the TUI session remains active

@req_client_0214
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /connect with unreachable URL shows error and keeps session active
  Given the TUI is running with gateway "<https://gateway.example.com>"
  When I type "/connect <https://unreachable.invalid>" and press Enter
  Then the scrollback shows an error or connectivity failure
  And the TUI session remains active
  And the session gateway remains "<https://gateway.example.com>"

@req_client_0214
@spec_cynai_client_cynorktui_localslashcommands
Scenario: /connect update is used for subsequent chat requests
  Given the TUI is running with gateway "<https://gateway.example.com>"
  When I type "/connect <https://other.example.com>" and press Enter and then send a chat message from the composer
  Then the chat request was sent to "<https://other.example.com>"
