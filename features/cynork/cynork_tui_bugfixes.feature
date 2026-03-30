@suite_cynork
Feature: cynork TUI bugfix regressions (thread landmarks, slash while streaming)

  As a cynork user
  I want accurate thread messaging after auth and usable slash/shell while a response streams
  So that I am not misled and can run local commands without waiting for the model

@req_client_0181 @req_client_0207 @spec_cynai_client_cynorkchat_tuilayout
Scenario: Thread ensure without changing thread id does not emit thread switched landmark
  Then the ensure thread scrollback line for prior "t-1" after "t-1" resume "" contains "[CYNRK_THREAD_READY]" and not "[CYNRK_THREAD_SWITCHED]"

@req_client_0207 @spec_cynai_client_cynorktui_slashcommandexecution
Scenario: Slash and shell commands are not blocked while loading
  Given loading is true
  And agent streaming is true
  Then enter is not blocked for composer input "/version"
  And enter is not blocked for composer input "!echo hi"
  And enter is not blocked for composer input "hello world"
