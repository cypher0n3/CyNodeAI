@suite_cynork
Feature: cynork TUI /model and /models slash commands

  As a user of the cynork TUI
  I want to list and select the chat model via slash commands
  So that I can switch models without leaving the chat surface

Background:

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0171
@req_client_0172
@spec_cynai_client_cynorktui_modelslashcommands
Scenario: /models lists available model identifiers
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
Scenario: /model with id updates the session model
  Given the TUI is running
  When I type "/model test-model-v2" and press Enter
  Then the session model is updated to "test-model-v2"

@req_client_0171
@spec_cynai_client_cynorktui_modelslashcommands
Scenario: /model change does not mutate system settings or user preferences
  Given the TUI is running with model "cynodeai.pm"
  When I type "/model other-model" and press Enter
  Then the session model is updated to "other-model"
  And stored user preferences are unchanged

@req_client_0171
@spec_cynai_client_cynorktui_modelslashcommands
Scenario: /model updated id is used for subsequent chat completion requests
  Given the TUI is running
  When I type "/model test-model-v2" and press Enter and then send a chat message from the composer
  Then the chat completion request used model "test-model-v2"

@req_client_0172
@spec_cynai_client_cynorktui_modelslashcommands
Scenario: /models with gateway error shows inline error and session continues
  Given the TUI is running and the gateway returns an error for GET /v1/models
  When I type "/models" and press Enter
  Then the scrollback shows an inline error
  And the TUI session remains active

@req_client_0171
@spec_cynai_client_cynorktui_modelslashcommands
Scenario: /model validates identifier when model discovery data is available
  Given the TUI is running and the gateway exposes known model ids
  When I type "/model unknown-model-xyz" and press Enter
  Then the scrollback shows a validation message or the session model is updated
  And the TUI session remains active
