@suite_cynork
Feature: cynork TUI authentication and login recovery

  As a user of the cynork TUI
  I want the TUI to start without requiring a valid token and prompt me to login if needed
  So that I am never blocked from launching the interactive chat surface

## Background

  Given a mock gateway is running
  And cynork is built

@req_client_0190
@req_client_0197
@spec_cynai_client_cynorktui_entrypoint
@spec_cynai_client_cynorkchat_authrecovery
Scenario: TUI starts when the config has no token
  Given the local cynork config has no token
  When I run cynork tui
  Then the full-screen chat TUI starts
  And the TUI does not exit with an auth error before rendering

@req_client_0190
@req_client_0197
@spec_cynai_client_cynorktui_entrypoint
@spec_cynai_client_cynorkchat_authrecovery
Scenario: TUI starts when the stored token is expired
  Given the local cynork config has an expired token
  When I run cynork tui
  Then the full-screen chat TUI starts
  And the TUI does not exit with an auth error before rendering

@req_client_0190
@spec_cynai_client_cynorkchat_authrecovery
Scenario: TUI defers token validation to the initial gateway connection
  Given the local cynork config has no token
  When I run cynork tui
  Then the full-screen chat TUI renders before any gateway auth check
  And the TUI validates the token on the first gateway connection attempt

@req_client_0190
@spec_cynai_client_cynorkchat_authrecovery
Scenario: TUI prompts for login when the initial connection finds no token
  Given the local cynork config has no token
  And I run cynork tui
  When the TUI attempts the initial gateway connection
  Then the TUI shows an in-session login prompt
  And the login prompt accepts a username
  And the login prompt accepts a password with secure non-echoing input

@req_client_0190
@spec_cynai_client_cynorkchat_authrecovery
Scenario: TUI prompts for login when the initial connection rejects an expired token
  Given the local cynork config has an expired token
  And I run cynork tui
  When the TUI attempts the initial gateway connection and the gateway returns 401
  Then the TUI shows an in-session login prompt
  And the login prompt accepts a username
  And the login prompt accepts a password with secure non-echoing input

@req_client_0190
@spec_cynai_client_cynorkchat_authrecovery
Scenario: Successful startup login resumes normal session flow
  Given the local cynork config has no token
  And I run cynork tui
  And the TUI shows an in-session login prompt
  When I complete the login prompt with valid credentials
  Then the TUI resumes normal session flow
  And I can send a chat message without restarting the TUI

@req_client_0190
@spec_cynai_client_cynorkchat_authrecovery
Scenario: Cancelled startup login exits cleanly
  Given the local cynork config has no token
  And I run cynork tui
  And the TUI shows an in-session login prompt
  When I cancel the login prompt
  Then the TUI exits with the normal auth failure outcome

@req_client_0190
@spec_cynai_client_cynorkchat_authrecovery
Scenario: Failed startup login allows retry before exiting
  Given the local cynork config has no token
  And I run cynork tui
  And the TUI shows an in-session login prompt
  When I enter invalid credentials in the login prompt
  Then the TUI shows an authentication error
  And the TUI allows me to retry the login prompt

@req_client_0190
@spec_cynai_client_cynorkchat_authrecovery
Scenario: Startup login password is not echoed or persisted in transcript
  Given the local cynork config has no token
  And I run cynork tui
  When the TUI shows an in-session login prompt
  Then password input uses secure non-echoing entry
  And the password is not visible in the scrollback or transcript history

@req_client_0190
@spec_cynai_client_cynorkchat_authrecovery
Scenario: Mid-session auth failure prompts re-authentication and retries
  Given I am logged in with username "alice" and password "secret"
  And the TUI is running with an expired login token
  When a chat request returns an authorization error and I complete the in-session login prompt successfully
  Then the TUI offers to retry the interrupted action once
  And the session continues without restarting the TUI

# NOTE: Capability Not Implemented Yet

@req_client_0191
@spec_cynai_client_cliweblogin
Scenario: Web login shows bounded authorization details without printing a token
  When I start the web login flow from the CLI
  Then the CLI shows a browser URL or device-code verification URL
  And the CLI shows the login expiry or timeout
  And the CLI does not print an access token
