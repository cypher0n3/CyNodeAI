@suite_cynork
Feature: cynork CLI

  As a user of the cynork CLI
  I want to call the User API Gateway for status, auth, and tasks
  So that I can manage sessions and run tasks from the command line

  Background:
    Given a mock gateway is running
    And cynork is built

  @req_client_0101
  @spec_cynai_client_clicommandsurface
  Scenario: Check gateway status
    When I run cynork status
    Then cynork exits with code 0
    And cynork stdout contains "ok"

  @req_client_0158
  @spec_cynai_client_clicommandsurface
  Scenario: Shorthand flags are accepted
    When I run cynork status with output json using shorthand "-o"
    Then cynork exits with code 0

  @req_client_0105
  @spec_cynai_client_cliauth
  Scenario: Login and whoami
    When I run cynork auth login with username "alice" and password "secret"
    Then cynork exits with code 0
    When I run cynork auth whoami
    Then cynork exits with code 0
    And cynork stdout contains "handle=alice"

  @req_client_0104
  @spec_cynai_client_cliauth
  Scenario: Whoami without token fails
    When I run cynork auth whoami
    Then cynork exits with code 3

  # Intended semantics: task is text or markdown; system interprets and may call model and/or run sandbox.
  # This scenario uses a literal shell command for MVP testing until prompt interpretation is implemented.
  @req_client_0101
  @req_client_0151
  @spec_cynai_client_clicommandsurface
  @spec_cynai_client_clitaskcreateprompt
  Scenario: Create task with inline prompt and get result
    Given I am logged in with username "alice" and password "secret"
    When I run cynork task create with prompt "echo hello"
    Then cynork exits with code 0
    And I store the task id from cynork stdout
    When I run cynork task result with the stored task id
    Then cynork exits with code 0
    And cynork stdout contains "status=completed"
    And cynork stdout contains "hello"

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
    When I run cynork task create with prompt "echo hello"
    Then cynork exits with code 0
    And I store the task id from cynork stdout
    When I run cynork task get with the stored task id
    Then cynork exits with code 0
    And cynork stdout contains "task_id="

  @req_client_0155
  @spec_cynai_client_clicommandsurface
  Scenario: Cancel task with yes
    Given I am logged in with username "alice" and password "secret"
    When I run cynork task create with prompt "echo hello"
    Then cynork exits with code 0
    And I store the task id from cynork stdout
    When I run cynork task cancel with the stored task id and yes
    Then cynork exits with code 0

  @req_client_0155
  @spec_cynai_client_clicommandsurface
  Scenario: Read task logs
    Given I am logged in with username "alice" and password "secret"
    When I run cynork task create with prompt "echo hello"
    Then cynork exits with code 0
    And I store the task id from cynork stdout
    When I run cynork task logs with the stored task id
    Then cynork exits with code 0

  @req_client_0143
  @spec_cynai_client_clicredential
  Scenario: Credentials list returns metadata only
    Given I am logged in with username "alice" and password "secret"
    When I run cynork creds list
    Then cynork exits with code 0

  @req_client_0121
  @spec_cynai_client_clipreferences
  Scenario: Set and get a preference
    Given I am logged in with username "alice" and password "secret"
    When I run cynork prefs set scope type "user" key "output.summary_style" value "\"concise\""
    Then cynork exits with code 0
    When I run cynork prefs get scope type "user" key "output.summary_style"
    Then cynork exits with code 0

  @req_client_0160
  @spec_cynai_client_clisystemsettings
  Scenario: Set and get a system setting
    Given I am logged in with username "alice" and password "secret"
    When I run cynork settings set key "agents.project_manager.model.local_default_ollama_model" value "\"tinyllama\""
    Then cynork exits with code 0
    When I run cynork settings get key "agents.project_manager.model.local_default_ollama_model"
    Then cynork exits with code 0

  @req_client_0125
  @spec_cynai_client_clinodemgmt
  Scenario: Nodes list returns inventory
    Given I am logged in with username "alice" and password "secret"
    When I run cynork nodes list
    Then cynork exits with code 0

  @req_client_0146
  @spec_cynai_client_cliskillsmanagement
  Scenario: Load and get a skill
    Given I am logged in with username "alice" and password "secret"
    And a markdown file "tmp/skill.md" exists with content "# Test skill"
    When I run cynork skills load with file "tmp/skill.md"
    Then cynork exits with code 0

  @req_client_0155
  @spec_cynai_client_cliauditcommands
  Scenario: Audit list returns events
    Given I am logged in with username "alice" and password "secret"
    When I run cynork audit list
    Then cynork exits with code 0

  @req_client_0150
  @spec_cynai_client_clisessionpersistence
  Scenario: Consecutive invocations use stored session
    When I run cynork auth login with username "alice" and password "secret"
    Then cynork exits with code 0
    When I run cynork auth whoami using the stored config
    Then cynork exits with code 0
    And cynork stdout contains "handle=alice"

  @req_client_0159
  @spec_cynai_client_cliinteractivemode
  Scenario: Interactive mode offers tab-completion of task names
    Given I am logged in with username "alice" and password "secret"
    And at least one task exists with a human-readable name
    When I run cynork shell in interactive mode
    And I request tab-completion for a task identifier position
    Then the completion candidates include task names

  @req_client_0161
  @spec_cynai_client_clichat
  Scenario: Chat without token fails with auth error
    When I run cynork chat
    Then cynork exits with code 3

  @req_client_0161
  @spec_cynai_client_clichat
  Scenario: Chat with valid session starts and accepts exit
    Given I am logged in with username "alice" and password "secret"
    When I run cynork chat
    And I send "/exit" to cynork stdin
    Then cynork exits with code 0
