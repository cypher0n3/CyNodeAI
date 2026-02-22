@suite_cynork
Feature: cynork admin and resource commands

  As a user of the cynork CLI
  I want to manage credentials, preferences, settings, nodes, skills, and audit
  So that I can configure the system and inspect resources via the gateway

  Background:
    Given a mock gateway is running
    And cynork is built

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
