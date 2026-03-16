@suite_orchestrator
Feature: Chat thread management and rich chat metadata

  As an authenticated chat client
  I want the gateway and orchestrator to manage thread metadata and rich chat state
  So that clients can present history, downloads, and attached-file context without scraping prose

## Background

  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"

@req_usrgwy_0142
@req_usrgwy_0144
@spec_cynai_usrgwy_chatthreadsmessages_threadtitle
@spec_cynai_usrgwy_chatthreadsmessages_historylist
Scenario: Thread list is recent-first and thread titles can be updated
  Given I have at least two existing chat threads
  When I call GET "/v1/chat/threads"
  Then the response status is 200
  And the thread list is ordered by updated_at descending
  And I call PATCH on one chat thread with a new title
  And the response status is 200
  And the thread title is updated

@req_pmagnt_0119
@spec_cynai_usrgwy_chatthreadsmessages_threadtitle
@spec_cynai_agents_threadtitling
Scenario: Thread receives auto-title after first user message
  Given I have a new chat thread with no user-set title
  When I send the first user message in that thread
  Then the system sets the thread title automatically
  And the title is available via GET thread or thread list
  And the title is derived from the first user message or a server-defined fallback

@req_usrgwy_0141
@req_usrgwy_0139
@spec_cynai_usrgwy_chatthreadsmessages_structuredturns
@spec_cynai_usrgwy_chatthreadsmessages_downloadrefs
Scenario: Rich assistant output exposes structured parts and download metadata
  When I send a chat message that produces text, tool activity, and a downloadable artifact
  Then the response status is 200
  And the persisted assistant turn contains ordered structured parts
  And any downloadable artifact is exposed as structured download metadata

@req_orches_0167
@req_orches_0168
@spec_cynai_orches_chatfileuploadflow
Scenario: Accepted chat file references survive routing and history replay
  Given I have an accepted chat upload attached to a user message
  When I continue the conversation on the same thread
  Then the orchestrator includes the file context when building the follow-up model request

@req_usrgwy_0140
@req_schema_0114
@spec_cynai_usrgwy_chatthreadsmessages_fileuploadstorage
@spec_cynai_schema_chatmessageattachmentstable
Scenario: Shared-project chat upload inherits the same project permissions
  Given I have an accepted chat upload attached to a message in a shared project thread
  When the system persists and later serves that uploaded file
  Then the uploaded file uses the same project-scoped permissions as the originating thread

@req_usrgwy_0146
@req_usrgwy_0147
@spec_cynai_usrgwy_chatthreadsmessages_contextsizetracking
@spec_cynai_usrgwy_openaichatapi_contextcompaction
Scenario: Gateway compacts older context before a near-limit chat completion
  Given a chat thread whose next completion request would reach at least 95 percent of the selected model context window
  When I send the next chat message on that thread
  Then the gateway deterministically computes the effective context size for that request
  And the gateway compacts older conversation context before issuing the completion request
  And enough recent unsummarized context remains for the next response

@req_usrgwy_0139
@spec_cynai_usrgwy_chatthreadsmessages_structuredturns
Scenario: Retained thinking survives persistence and thread-history retrieval
  Given I sent a chat message that produced an assistant turn with visible text and structured thinking
  When I call GET "/v1/chat/threads/<thread_id>/messages" or equivalent thread history
  Then the persisted messages include the visible text and the retained thinking as structured parts
  And the thinking content is available for TUI display on scrollback without leaking into canonical plain-text content
