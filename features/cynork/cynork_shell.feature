@suite_cynork
Feature: cynork shell compatibility

  As a user of the cynork CLI
  I want legacy shell behavior to remain clearly secondary to the chat TUI
  So that compatibility is preserved without redefining the primary interactive UX

@req_client_0202
@spec_cynai_client_tuiscope
Scenario: Primary interactive entrypoint is the TUI, not the legacy shell
  Given a mock gateway is running
  And cynork is built
  When I review the documented interactive entrypoints
  Then `cynork tui` is the documented primary interactive chat surface
  And `cynork shell` is documented as deprecated compatibility

@req_client_0189
@spec_cynai_client_clichatshellescape
Scenario: Interactive shell-style commands remain available through chat shell escape
  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"
  When I run cynork chat and send "!echo hello" to cynork stdin
  Then the shell command output is shown inline in chat
