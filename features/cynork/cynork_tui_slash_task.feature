@suite_cynork
Feature: cynork TUI /task slash commands

  As a user of the cynork TUI
  I want to list, get, create, cancel, and inspect tasks via slash commands
  So that I can manage work without leaving the chat surface

## Background

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0166
@req_client_0207
@spec_cynai_client_cynorktui_taskslashcommands
@spec_cynai_client_clitasklist
Scenario: /task list shows tasks inline
  Given the TUI is running and the gateway returns a task list
  When I type "/task list" and press Enter
  Then the scrollback shows task list output
  And the TUI session remains active

@req_client_0166
@req_client_0207
@spec_cynai_client_cynorktui_taskslashcommands
@spec_cynai_client_clitaskget
Scenario: /task get with selector shows task details inline
  Given the TUI is running and the mock gateway has a task with id "task-abc"
  When I type "/task get task-abc" and press Enter
  Then the scrollback shows task details for that task
  And the TUI session remains active

@req_client_0166
@req_client_0207
@spec_cynai_client_cynorktui_taskslashcommands
@spec_cynai_client_clitaskcreateprompt
Scenario: /task create with prompt creates a task and shows result inline
  Given the TUI is running and the gateway supports task create
  When I type "/task create echo hello" and press Enter
  Then the scrollback shows task creation result or task id
  And the TUI session remains active

@req_client_0166
@req_client_0207
@spec_cynai_client_cynorktui_taskslashcommands
@spec_cynai_client_clitaskcancel
Scenario: /task cancel with selector shows result inline
  Given the TUI is running and the mock gateway has a running task with id "task-xyz"
  When I type "/task cancel task-xyz" and press Enter
  Then the scrollback shows cancel result or confirmation
  And the TUI session remains active

@req_client_0166
@req_client_0207
@spec_cynai_client_cynorktui_taskslashcommands
@spec_cynai_client_clitaskresult
Scenario: /task result with selector shows result inline
  Given the TUI is running and the mock gateway has a completed task with id "task-done"
  When I type "/task result task-done" and press Enter
  Then the scrollback shows task result output
  And the TUI session remains active

@req_client_0166
@req_client_0207
@spec_cynai_client_cynorktui_taskslashcommands
@spec_cynai_client_clitasklogs
Scenario: /task logs with selector shows logs inline
  Given the TUI is running and the mock gateway has a task with id "task-123"
  When I type "/task logs task-123" and press Enter
  Then the scrollback shows task logs or an inline error
  And the TUI session remains active

@req_client_0166
@req_client_0207
@spec_cynai_client_cynorktui_taskslashcommands
@spec_cynai_client_clitaskartifactslist
Scenario: /task artifacts list with selector shows artifacts inline
  Given the TUI is running and the mock gateway has a task with id "task-art" that has artifacts
  When I type "/task artifacts list task-art" and press Enter
  Then the scrollback shows artifact list output
  And the TUI session remains active

@req_client_0166
@req_client_0207
@spec_cynai_client_cynorktui_taskslashcommands
@spec_cynai_client_clitaskget
Scenario: Task slash command failure shows error inline and keeps session active
  Given the TUI is running and the gateway returns 404 for the next task request
  When I type "/task get unknown-task-id" and press Enter
  Then the scrollback shows an inline error
  And the TUI session remains active
