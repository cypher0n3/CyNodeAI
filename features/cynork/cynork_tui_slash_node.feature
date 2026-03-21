@suite_cynork
Feature: cynork TUI /nodes slash commands

  As a user of the cynork TUI
  I want to list and get nodes via slash commands
  So that I can inspect node inventory without leaving the chat surface

Background:

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0168
@req_client_0207
@spec_cynai_client_cynorktui_nodeslashcommands
Scenario: /nodes list shows node inventory inline
  Given the TUI is running and the gateway supports nodes list
  When I type "/nodes list" and press Enter
  Then the scrollback shows node list output
  And the TUI session remains active

@req_client_0168
@req_client_0207
@spec_cynai_client_cynorktui_nodeslashcommands
Scenario: /nodes get with node_id shows node details inline
  Given the TUI is running and the gateway returns at least one node with id "node-1"
  When I type "/nodes get node-1" and press Enter
  Then the scrollback shows node details for "node-1"
  And the TUI session remains active

@req_client_0207
@spec_cynai_client_cynorktui_nodeslashcommands
Scenario: /nodes with unknown subcommand or missing args shows error and keeps session active
  Given the TUI is running
  When I type "/nodes invalid" and press Enter
  Then the scrollback shows a usage error or inline error
  And the TUI session remains active
