@suite_cynork
Feature: cynork TUI composer copy and history alignment

  As a cynork user
  I want copy slash commands and history navigation to match the tech spec
  So that automated tests stay aligned with composer and slash-command behavior

@req_client_0203 @req_client_0204 @spec_cynai_client_cynorktui_slashcommandexecution
Scenario: Plain transcript for copy all excludes system-prefixed lines
  Then plain transcript for copy all excludes system-prefixed scrollback lines

@req_client_0203 @spec_cynai_client_cynorkchat_tuilayout
Scenario: Last assistant plain text is the latest Assistant line
  Then last assistant plain text for scrollback is the latest Assistant line without prefix

@req_client_0204 @spec_cynai_client_cynorkchat_tuilayout
Scenario: Ctrl+Down moves forward in sent-message history after Ctrl+Up
  Then navigating input history with Ctrl+Up then Ctrl+Down restores the newer sent line

@req_client_0204 @spec_cynai_client_cynorkchat_tuilayout
Scenario: Composer footnote documents Alt+Enter and Ctrl+J for newline
  Then the composer newline keys include Alt+Enter and Ctrl+J per spec (not Shift+Enter as mandatory newline)
