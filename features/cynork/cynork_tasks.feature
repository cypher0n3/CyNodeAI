@suite_cynork
Feature: cynork task commands

  As a user of the cynork CLI
  I want to create and manage tasks via the gateway
  So that I can run work and retrieve status and results

Background:
  Given a mock gateway is running
  And cynork is built

  # Intended semantics: task is text or markdown; system interprets and may call model and/or run sandbox.
  # This scenario uses a literal shell command for MVP testing until prompt interpretation is implemented.
@req_client_0101
@req_client_0151
@spec_cynai_client_clicommandsurface
@spec_cynai_client_clitaskcreateprompt
Scenario: Create task with inline prompt
  Given I am logged in with username "alice" and password "secret"
  When I run cynork task create with prompt "echo hello"
  Then cynork exits with code 0
  And I store the task id from cynork stdout

@req_client_0151
@spec_cynai_client_clitaskcreateprompt
@spec_cynai_usrgwy_userapigateway
Scenario: Create task with optional task name
  Given I am logged in with username "alice" and password "secret"
  When I run cynork task create with prompt "echo hello" and task name "my-task"
  Then cynork exits with code 0
  And I store the task id from cynork stdout
  And cynork task get shows task name "my-task"

@req_client_0101
@req_client_0151
@spec_cynai_client_clicommandsurface
@spec_cynai_client_clitaskcreateprompt
Scenario: Get task result after create
  Given I am logged in with username "alice" and password "secret"
  And I have created a task with prompt "echo hello" and stored the task id
  When I run cynork task result with the stored task id
  Then cynork exits with code 0
  And cynork stdout contains "status=completed"
  And cynork stdout contains "hello"

@req_orches_0124
@req_orches_0125
@spec_cynai_client_clicommandsurface
Scenario: Task result status is valid CLI enum
  Given I am logged in with username "alice" and password "secret"
  And I have created a task with prompt "echo hello" and stored the task id
  When I run cynork task result with the stored task id in JSON mode
  Then cynork exits with code 0
  And the task result status is one of "queued", "running", "completed", "failed", "canceled", "superseded"

@req_client_0151
@spec_cynai_client_clitaskcreateprompt
Scenario: Create task from task file (text or Markdown)
  Given I am logged in with username "alice" and password "secret"
  And a task file "tmp/task.md" exists with content "Tell me the current time"
  When I run cynork task create with task file "tmp/task.md"
  Then cynork exits with code 0
  And I store the task id from cynork stdout

@req_client_0151
@req_client_0157
@spec_cynai_client_clitaskcreateprompt
Scenario: Create task with attachment paths
  Given I am logged in with username "alice" and password "secret"
  When I run cynork task create with prompt "summarize the attachments" and attachments "tmp/doc1.txt" "tmp/doc2.txt"
  Then cynork exits with code 0
  And I store the task id from cynork stdout

@req_client_0153
@spec_cynai_client_clitaskcreateprompt
Scenario: Create task with script
  Given I am logged in with username "alice" and password "secret"
  And a script file "tmp/hello.sh" exists
  When I run cynork task create with script "tmp/hello.sh"
  Then cynork exits with code 0
  And I store the task id from cynork stdout

@req_client_0153
@spec_cynai_client_clitaskcreateprompt
Scenario: Create task with short series of commands
  Given I am logged in with username "alice" and password "secret"
  When I run cynork task create with command "echo one" and command "echo two"
  Then cynork exits with code 0
  And I store the task id from cynork stdout

@req_client_0155
@spec_cynai_client_clicommandsurface
Scenario: List tasks
  Given I am logged in with username "alice" and password "secret"
  When I run cynork task list
  Then cynork exits with code 0

@req_client_0155
@spec_cynai_client_clicommandsurface
Scenario: Get task status
  Given I am logged in with username "alice" and password "secret"
  And I have created a task with prompt "echo hello" and stored the task id
  When I run cynork task get with the stored task id
  Then cynork exits with code 0
  And cynork stdout contains "task_id="

@req_client_0155
@spec_cynai_client_clicommandsurface
Scenario: Cancel task with yes
  Given I am logged in with username "alice" and password "secret"
  And I have created a task with prompt "echo hello" and stored the task id
  When I run cynork task cancel with the stored task id and yes
  Then cynork exits with code 0

@req_client_0155
@spec_cynai_client_clicommandsurface
Scenario: Read task logs
  Given I am logged in with username "alice" and password "secret"
  And I have created a task with prompt "echo hello" and stored the task id
  When I run cynork task logs with the stored task id
  Then cynork exits with code 0
