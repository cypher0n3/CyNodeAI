# Chat Threads and Messages

- [Document Overview](#document-overview)
- [Scope and Goals](#scope-and-goals)
- [Chat Threads](#chat-threads)
- [Chat Messages](#chat-messages)
- [Retention and Transcripts](#retention-and-transcripts)
- [Relationship to Runs and Sessions](#relationship-to-runs-and-sessions)
- [API Surface (Data REST)](#api-surface-data-rest)
- [Related Documents](#related-documents)

## Document Overview

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages` <a id="spec-cynai-usrgwy-chatthreadsmessages"></a>

This spec defines how chat conversations are stored and retrieved independently of task lifecycle.
It introduces first-class chat threads and chat messages.

Traces To:

- [REQ-USRGWY-0110](../requirements/usrgwy.md#req-usrgwy-0110)
- [REQ-USRGWY-0111](../requirements/usrgwy.md#req-usrgwy-0111)
- [REQ-USRGWY-0130](../requirements/usrgwy.md#req-usrgwy-0130)

## Scope and Goals

Chat is a conversation with the PM/PA agent surface.
Chat is not modeled as one task per message.

Goals:

- Persist chat history for UX and audit.
- Allow clients to retrieve a conversation thread and its messages.
- Separate chat message storage from task lifecycle storage.

Non-goals:

- Defining the OpenAI-compatible chat interface.
  See [OpenAI-Compatible Chat API](openai_compatible_chat_api.md).

## Chat Threads

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.Threads` <a id="spec-cynai-usrgwy-chatthreadsmessages-threads"></a>

A chat thread is a stable container for a conversation.

Recommended fields:

- `id` (uuid, pk)
- `user_id` (uuid)
- `project_id` (uuid, optional)
- `session_id` (uuid, optional)
- `title` (text, optional)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints:

- Index: (`user_id`, `updated_at`)
- If `project_id` is present, it MUST reference a valid project id.
- If `session_id` is present, it MUST reference a valid session id.

## Chat Messages

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.Messages` <a id="spec-cynai-usrgwy-chatthreadsmessages-messages"></a>

A chat message is one turn item in a thread.
Messages are append-only.

Recommended fields:

- `id` (uuid, pk)
- `thread_id` (uuid)
- `role` (text)
  Allowed values SHOULD include `user`, `assistant`, and `system`.
- `content` (text)
- `created_at` (timestamptz)
- `metadata` (jsonb, optional)
  For example model identifier, tool-call summary, or client identifiers.

Constraints:

- Foreign key: `thread_id` references `chat_threads.id`.
- Index: (`thread_id`, `created_at`)

## Retention and Transcripts

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.RetentionTranscripts` <a id="spec-cynai-usrgwy-chatthreadsmessages-retentiontranscripts"></a>

Raw chat messages are stored as chat messages.
Transcripts are stored separately as transcript segments associated with a session or run.

A transcript segment is a derived artifact.
A transcript segment MAY be a summary, a redacted view, or an operator-facing audit representation.

Retention policies MUST apply to both:

- Session transcripts and run logs per [Runs and Sessions API](runs_and_sessions_api.md).
- Chat threads and chat messages per this spec.

## Relationship to Runs and Sessions

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.Relationships` <a id="spec-cynai-usrgwy-chatthreadsmessages-relationships"></a>

Sessions are user-facing containers for interactive work.
Chat threads MAY be associated with sessions via `chat_threads.session_id`.

Runs remain execution traces.
Runs MAY reference a session id.
Runs MUST NOT be required for every chat message.

This relationship allows:

- Session-level retention and grouping across runs and chat threads.
- Persisting transcripts at the session level without conflating transcripts with raw chat messages.

## API Surface (Data REST)

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ApiSurface` <a id="spec-cynai-usrgwy-chatthreadsmessages-apisurface"></a>

Chat threads and chat messages SHOULD be exposed as Data REST resources.
The gateway MUST enforce authorization so users can only read their own chat threads and messages.

Required operations:

- Create chat thread.
- List chat threads.
- Get chat thread.
- Append message to thread.
- List messages for thread.

Clients that do not need this (for example Open WebUI) MAY use only the OpenAI-compatible endpoints and ignore thread management.

## Related Documents

- [OpenAI-Compatible Chat API](openai_compatible_chat_api.md)
- [Runs and Sessions API](runs_and_sessions_api.md)
- [User API Gateway](user_api_gateway.md)
