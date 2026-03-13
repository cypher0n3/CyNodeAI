@suite_cynork
Feature: cynork chat

  As a user of the cynork CLI
  I want to chat with the PM/PA via the OpenAI-compatible gateway
  So that I can converse, manage threads, and use slash commands from cynork

Background:
  Given a mock gateway is running
  And cynork is built

@req_client_0161
@spec_cynai_client_clichat
Scenario: Chat without token fails with auth error
  When I run cynork chat
  Then cynork exits with code 3

@req_client_0161
@spec_cynai_client_clichat
Scenario: Chat uses OpenAI-compatible chat surface and accepts exit
  Given I am logged in with username "alice" and password "secret"
  When I run cynork chat and send "/exit" to cynork stdin
  Then cynork exits with code 0

@req_client_0178
@spec_cynai_client_clichatoneshot
Scenario: Chat one-shot mode prints one assistant response and exits
  Given I am logged in with username "alice" and password "secret"
  When I run cynork chat with message "Summarize the current plan"
  Then cynork exits with code 0
  And the assistant response is printed once

@req_client_0181
@spec_cynai_client_clichatthreadcontrols
Scenario: Startup thread creation happens before the first completion
  Given I am logged in with username "alice" and password "secret"
  And the mock gateway supports POST "/v1/chat/threads"
  When I run cynork chat with thread-new enabled and message "Start fresh"
  Then cynork creates a fresh chat thread before the first completion

@req_client_0181
@spec_cynai_client_clichatthreadcontrols
Scenario: In-session thread creation starts a fresh conversation
  Given I am logged in with username "alice" and password "secret"
  And the mock gateway supports POST "/v1/chat/threads"
  When I run cynork chat and send "/thread new" to cynork stdin
  Then cynork creates a fresh chat thread before the next completion

@req_client_0181
@spec_cynai_client_clichatthreadcontrols
Scenario: Unknown thread subcommand shows guidance and keeps the session alive
  Given I am logged in with username "alice" and password "secret"
  When I run cynork chat and send "/thread nope" to cynork stdin
  Then the chat session shows guidance for valid /thread commands
  And the chat session remains active

@req_client_0207
@spec_cynai_client_clichatslashcommandreference
Scenario: Chat slash help exposes thread and model controls
  Given I am logged in with username "alice" and password "secret"
  When I run cynork chat and send "/help" to cynork stdin
  Then the slash-command help includes "/thread new"
  And the slash-command help includes "/thread list"
  And the slash-command help includes "/model"
