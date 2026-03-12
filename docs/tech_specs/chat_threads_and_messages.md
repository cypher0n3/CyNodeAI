# Chat Threads and Messages

- [Document Overview](#document-overview)
- [Scope and Goals](#scope-and-goals)
- [Chat Threads](#chat-threads)
- [Chat Messages](#chat-messages)
- [Structured Turns](#structured-turns)
  - [Canonical projection:](#canonical-projection)
  - [Structured representation:](#structured-representation)
  - [Ordering and grouping:](#ordering-and-grouping)
  - [Thinking and reasoning:](#thinking-and-reasoning)
  - [Tool and file metadata:](#tool-and-file-metadata)
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
- [REQ-USRGWY-0136](../requirements/usrgwy.md#req-usrgwy-0136)
- [REQ-USRGWY-0137](../requirements/usrgwy.md#req-usrgwy-0137)
- [REQ-USRGWY-0138](../requirements/usrgwy.md#req-usrgwy-0138)

## Scope and Goals

Chat is a conversation with the PM/PA agent surface.
Chat is not modeled as one task per message.

Goals:

- Persist chat history for UX and audit.
- Allow clients to retrieve a conversation thread and its messages.
- Separate chat message storage from task lifecycle storage.
- Ensure persisted chat content does not store plaintext secrets.
- Raw user input MUST be amended by secret redaction before storage.
- Multi-message conversation is the intended way to clarify and lay out a task (or project plan) before or as it is executed; building up a task properly may take multiple messages.
  See [REQ-AGENTS-0135](../requirements/agents.md#req-agents-0135) and [CYNAI.AGENTS.ClarificationBeforeExecution](../tech_specs/project_manager_agent.md#spec-cynai-agents-clarificationbeforeexecution).

Non-goals:

- Defining the OpenAI-compatible chat interface.
  See [OpenAI-Compatible Chat API](openai_compatible_chat_api.md).

## Chat Threads

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.Threads` <a id="spec-cynai-usrgwy-chatthreadsmessages-threads"></a>

A chat thread is a stable container for a conversation.

Thread association rules:

- For OpenAI-compatible interactive chat requests, the orchestrator manages thread association server-side.
- **Active-thread reuse:** When no explicit thread identifier is provided by the client, the gateway SHOULD use a single active thread per `(user_id, project_id)` scope and MAY rotate to a new thread after inactivity.
  Project scope is taken from the `OpenAI-Project` request header when present; when absent, the user's default project applies (see [REQ-USRGWY-0131](../requirements/usrgwy.md#req-usrgwy-0131)).
- **Explicit fresh-thread creation:** A client that needs a new conversation MUST call `POST /v1/chat/threads` to create a new thread.
  This explicit create operation is a separate CyNodeAI Data REST endpoint and MUST NOT change the OpenAI-compatible `POST /v1/chat/completions` request shape by requiring any CyNodeAI-specific thread identifier in request bodies or headers.
  Explicit creation does not affect the active-thread reuse state for other clients or sessions.
- Inactivity threshold for active-thread rotation: 2 hours.

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

Secret handling:

- Message `content` MUST be the amended (redacted) content.
- The system MUST NOT persist plaintext secrets in chat message content.
- If redaction occurs, it MUST be indicated directly in `content` by replacing detected secrets with the literal string `SECRET_REDACTED`.

Recommended fields:

- `id` (uuid, pk)
- `thread_id` (uuid)
- `role` (text)
  Allowed values SHOULD include `user`, `assistant`, and `system`.
- `content` (text)
- `created_at` (timestamptz)
- `metadata` (jsonb, optional)
  For example model identifier, tool-call summary, or client identifiers.

Recommended metadata keys:

- `model_id` (string, optional)
- `parts` (jsonb array, optional)
- `response_id` (string, optional)
- `turn_kind` (string, optional)
- `reasoning_redacted` (boolean, optional)

Constraints:

- Foreign key: `thread_id` references `chat_threads.id`.
- Index: (`thread_id`, `created_at`)

## Structured Turns

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns` <a id="spec-cynai-usrgwy-chatthreadsmessages-structuredturns"></a>

Rich clients need more than plain transcript text to render reasoning visibility, tool activity, and multi-item assistant output cleanly.
This spec uses the same general UX pattern seen in tools such as Open WebUI and LibreChat: keep the main answer readable, expose reasoning as secondary collapsed content, and avoid forcing clients to scrape prose for tool activity.

### Canonical Projection

- `content` remains the canonical plain-text transcript for simple clients, thread previews, title fallback, summaries, and text-only search.
- Rich clients SHOULD prefer `metadata.parts` when present and MUST fall back to `content` when it is absent.

### Structured Representation

- `metadata.parts` is an optional ordered array representing one logical chat turn.
- Allowed part kinds for the stable first pass are:
  - `text`: visible assistant or user prose.
  - `thinking`: hidden-by-default model reasoning or reasoning summary.
  - `tool_call`: tool invocation metadata such as tool name, call identifier, and status.
  - `tool_result`: tool outcome metadata such as status, content type, and bounded preview.
  - `attachment_ref`: user-supplied file reference metadata.
  - `download_ref`: assistant-produced downloadable file metadata.

### Ordering and Grouping

- When one interactive chat request produces multiple assistant-side output items, the gateway MUST preserve those items in `metadata.parts` order under one logical assistant turn.
- The gateway SHOULD persist one assistant message row with ordered `metadata.parts` for that logical turn rather than multiple unrelated assistant message rows.
- `content` for that assistant message MUST be the plain-text projection of the visible `text` parts only, in order.
  It MAY be empty when a turn has no visible text parts.

### Thinking and Reasoning

- `thinking` parts MUST NOT be copied into canonical plain-text `content`.
- `thinking` parts MUST NOT be used for thread title generation, summaries, or default list previews.
- If preserved, `thinking` parts SHOULD be bounded in size.
  When full reasoning is not retained, the gateway MAY instead persist a bounded reasoning summary or `reasoning_redacted: true` metadata.

### Tool and File Metadata

- Tool-call arguments and tool-result previews stored in `metadata.parts` MUST already be redacted and SHOULD be bounded in size.
- `attachment_ref` and `download_ref` parts SHOULD carry stable identifiers and user-displayable metadata such as filename, media type, and size when known.
- Local machine-specific file paths MUST NOT be persisted as canonical transcript content.

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

Standardized endpoints (Phase 1):

- `POST /v1/chat/threads`
  - Create a new chat thread (explicit fresh-thread creation).
  - Request body MUST allow:
    - `project_id` (uuid, optional): when present, the thread is scoped to that project; when absent, the server associates the thread with the authenticated user's default project (see [REQ-USRGWY-0131](../requirements/usrgwy.md#req-usrgwy-0131)).
    - `title` (string, optional)
  - The server MUST derive `user_id` from authentication.
  - The server MUST NOT allow the client to set `session_id`.
  - The response MUST return the created thread (including `id`) for Data REST retrieval and management purposes.
  - The gateway MUST NOT require the client to send that thread id on subsequent `POST /v1/chat/completions` requests.
- `GET /v1/chat/threads`
  - List chat threads for the authenticated user.
  - SHOULD support filtering by `project_id`.
  - Pagination MUST use `limit` and `offset` query parameters.
- `GET /v1/chat/threads/{thread_id}`
  - Get one chat thread owned by the authenticated user.
- `POST /v1/chat/threads/{thread_id}/messages`
  - Append one message to a thread.
  - The gateway MUST reject attempts to append plaintext secrets (redaction must occur before persistence).
  - Clients MUST only append messages with `role: user`.
  - The gateway writes assistant messages as part of the OpenAI-compatible interactive chat endpoints (`POST /v1/chat/completions` and `POST /v1/responses`).
- `GET /v1/chat/threads/{thread_id}/messages`
  - List messages for a thread in ascending `created_at` order.
  - Pagination MUST use `limit` and `offset` query parameters.

Clients that do not need this (for example Open WebUI) MAY use only the OpenAI-compatible endpoints and ignore thread management.

## Related Documents

- [OpenAI-Compatible Chat API](openai_compatible_chat_api.md)
- [Runs and Sessions API](runs_and_sessions_api.md)
- [User API Gateway](user_api_gateway.md)
