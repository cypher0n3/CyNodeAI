# Chat Threads and Messages

- [Document Overview](#document-overview)
  - [Requirement Traces](#requirement-traces)
- [Postgres Schema](#postgres-schema)
  - [Chat Threads Table](#chat-threads-table)
  - [Chat Messages Table](#chat-messages-table)
  - [Chat Message Attachments Table](#chat-message-attachments-table)
- [Scope and Goals](#scope-and-goals)
  - [Scope Goals](#scope-goals)
  - [Scope Non-Goals](#scope-non-goals)
- [Chat Threads](#chat-threads)
  - [Thread Association Model](#thread-association-model)
  - [Canonical Persisted Thread Fields](#canonical-persisted-thread-fields)
  - [Thread Constraints](#thread-constraints)
- [Thread Acquisition](#thread-acquisition)
- [Thread Title](#thread-title)
  - [Thread Title Traces To](#thread-title-traces-to)
- [Thread Summary](#thread-summary)
  - [Thread Summary Traces To](#thread-summary-traces-to)
  - [`ThreadSummary` Constants](#threadsummary-constants)
  - [`ThreadSummary` Deferred Implementation](#threadsummary-deferred-implementation)
- [Chat Messages](#chat-messages)
  - [Secret Handling](#secret-handling)
  - [Canonical Persisted Message Fields](#canonical-persisted-message-fields)
  - [Canonical Message Metadata Keys](#canonical-message-metadata-keys)
  - [Message Constraints](#message-constraints)
- [Chat History List Behavior](#chat-history-list-behavior)
  - [History List Traces To](#history-list-traces-to)
- [Archive and Soft Delete](#archive-and-soft-delete)
  - [Archive Traces To](#archive-traces-to)
  - [`Archive` Deferred Implementation](#archive-deferred-implementation)
- [Structured Turns](#structured-turns)
  - [Canonical Projection](#canonical-projection)
  - [Structured Representation](#structured-representation)
  - [Ordering and Grouping](#ordering-and-grouping)
  - [Thinking and Reasoning](#thinking-and-reasoning)
  - [Tool and File Metadata](#tool-and-file-metadata)
- [Download References](#download-references)
  - [Download References implementation](#download-references-implementation)
  - [Download References Traces To](#download-references-traces-to)
- [Chat File Upload Storage](#chat-file-upload-storage)
  - [Chat File Upload Storage Traces To](#chat-file-upload-storage-traces-to)
  - [Chat File Upload Storage implementation](#chat-file-upload-storage-implementation)
- [Context Size Tracking](#context-size-tracking)
  - [Context Size Tracking Traces To](#context-size-tracking-traces-to)
- [Retention and Transcripts](#retention-and-transcripts)
  - [Retention Policy Scope](#retention-policy-scope)
- [Relationship to Runs and Sessions](#relationship-to-runs-and-sessions)
  - [Relationship Outcomes](#relationship-outcomes)
- [API Surface (Data REST)](#api-surface-data-rest)
  - [Required Operations](#required-operations)
  - [Standardized Endpoints](#standardized-endpoints)
- [Related Documents](#related-documents)

## Document Overview

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages` <a id="spec-cynai-usrgwy-chatthreadsmessages"></a>

This spec defines how chat conversations are stored and retrieved independently of task lifecycle.
It introduces first-class chat threads and chat messages.

### Requirement Traces

- [REQ-USRGWY-0110](../requirements/usrgwy.md#req-usrgwy-0110)
- [REQ-USRGWY-0111](../requirements/usrgwy.md#req-usrgwy-0111)
- [REQ-USRGWY-0130](../requirements/usrgwy.md#req-usrgwy-0130)
- [REQ-USRGWY-0136](../requirements/usrgwy.md#req-usrgwy-0136)
- [REQ-USRGWY-0137](../requirements/usrgwy.md#req-usrgwy-0137)
- [REQ-USRGWY-0138](../requirements/usrgwy.md#req-usrgwy-0138)
- [REQ-USRGWY-0139](../requirements/usrgwy.md#req-usrgwy-0139)
- [REQ-USRGWY-0141](../requirements/usrgwy.md#req-usrgwy-0141)
- [REQ-USRGWY-0142](../requirements/usrgwy.md#req-usrgwy-0142)
- [REQ-USRGWY-0143](../requirements/usrgwy.md#req-usrgwy-0143)
- [REQ-USRGWY-0144](../requirements/usrgwy.md#req-usrgwy-0144)
- [REQ-USRGWY-0145](../requirements/usrgwy.md#req-usrgwy-0145)
- [REQ-CLIENT-0199](../requirements/client.md#req-client-0199)
- [REQ-CLIENT-0200](../requirements/client.md#req-client-0200)
- [REQ-CLIENT-0201](../requirements/client.md#req-client-0201)
- [REQ-CLIENT-0194](../requirements/client.md#req-client-0194)

## Postgres Schema

- Spec ID: `CYNAI.SCHEMA.ChatThreadsMessages` <a id="spec-cynai-schema-chatthreadsmessages"></a>

Chat threads and chat messages store chat history separately from task lifecycle state.
Chat message content is the amended (redacted) content.
Plaintext secrets are not persisted in chat message content.

**Schema definitions (index):** See [Chat Threads and Messages](postgres_schema.md#spec-cynai-schema-chatthreadsmessages) in [`postgres_schema.md`](postgres_schema.md).

### Chat Threads Table

- `id` (uuid, pk)
- `user_id` (uuid, fk to `users.id`)
- `project_id` (uuid, fk to `projects.id`, nullable)
- `session_id` (uuid, fk to `sessions.id`, nullable)
- `title` (text, nullable)
- `summary` (text, nullable)
- `archived_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### Chat Threads Table Constraints

- Index: (`user_id`, `updated_at`)
- Index: (`project_id`, `updated_at`)

### Chat Messages Table

- `id` (uuid, pk)
- `thread_id` (uuid, fk to `chat_threads.id`)
- `role` (text)
  - examples: user, assistant, system
- `content` (text)
- `metadata` (jsonb, nullable)
- `created_at` (timestamptz)

#### Chat Messages Table Constraints

- Index: (`thread_id`, `created_at`)

### Chat Message Attachments Table

- Spec ID: `CYNAI.SCHEMA.ChatMessageAttachmentsTable` <a id="spec-cynai-schema-chatmessageattachmentstable"></a>

#### Chat Message Attachments Table Requirements Traces

- [REQ-SCHEMA-0114](../requirements/schema.md#req-schema-0114)

This table stores user-message file uploads or stable file references accepted through the chat `@`-reference workflow.
When the originating chat thread is project-scoped, rows in this table inherit the same project authorization boundary as the thread and message they attach to.

- `id` (uuid, pk)
- `message_id` (uuid, fk to `chat_messages.id`)
- `thread_id` (uuid, fk to `chat_threads.id`)
- `user_id` (uuid, fk to `users.id`)
- `file_id` (text)
  - stable internal identifier returned by the upload layer
- `filename` (text)
- `media_type` (text, nullable)
- `size_bytes` (bigint, nullable)
- `storage_ref` (text)
  - internal blob or object-storage reference
- `checksum_sha256` (text, nullable)
- `created_at` (timestamptz)

#### Chat Message Attachments Table Constraints

- Index: (`message_id`)
- Index: (`thread_id`, `created_at`)
- Index: (`user_id`, `created_at`)
- Unique: (`message_id`, `file_id`)
- Access to attachment rows and referenced storage is enforced through the parent chat thread and message authorization, including shared-project permissions when the thread belongs to a shared project.

## Scope and Goals

Chat is a conversation with the PM/PA agent surface.
Chat is not modeled as one task per message.

### Scope Goals

- Persist chat history for UX and audit.
- Allow clients to retrieve a conversation thread and its messages.
- Separate chat message storage from task lifecycle storage.
- Ensure persisted chat content does not store plaintext secrets.
- Raw user input MUST be amended by secret redaction before storage.
- Multi-message conversation is the intended way to clarify and lay out a task (or project plan) before or as it is executed; building up a task properly may take multiple messages.
  See [REQ-AGENTS-0135](../requirements/agents.md#req-agents-0135) and [CYNAI.AGENTS.ClarificationBeforeExecution](../tech_specs/project_manager_agent.md#spec-cynai-agents-clarificationbeforeexecution).

### Scope Non-Goals

- Defining the OpenAI-compatible chat interface.
  See [OpenAI-Compatible Chat API](openai_compatible_chat_api.md).

## Chat Threads

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.Threads` <a id="spec-cynai-usrgwy-chatthreadsmessages-threads"></a>

A chat thread is a stable container for a conversation.

### Thread Association Model

- For OpenAI-compatible interactive chat requests, the orchestrator manages thread association server-side.
- The system supports exactly two thread-acquisition modes: active-thread reuse and explicit fresh-thread creation.
- Every chat thread MUST be associated with the effective project scope used when that thread is created or reused.
- That effective project scope uses the same rule as the rest of chat: `OpenAI-Project` when present, otherwise the authenticated user's default project.
- This project association MUST be persisted on the thread as `project_id` when the effective project is known.
- OpenAI-compatible chat requests MUST NOT require a CyNodeAI-specific project field in the request body to preserve that association.
- Active-thread reuse and explicit fresh-thread creation are distinct operations and MUST remain observable as distinct behaviors in handlers, persistence, and tests.

### Canonical Persisted Thread Fields

- `id` (uuid, pk)
- `user_id` (uuid)
- `project_id` (uuid, optional)
- `session_id` (uuid, optional)
- `title` (text, optional)
- `summary` (text, optional)
- `archived_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

### Thread Constraints

- Index: (`user_id`, `updated_at`)
- If `project_id` is present, it MUST reference a valid project id.
- If `session_id` is present, it MUST reference a valid session id.

## Thread Acquisition

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ThreadAcquisition` <a id="spec-cynai-usrgwy-chatthreadsmessages-threadacquisition"></a>

The system defines exactly two thread-acquisition modes.

- **Client default at session start:** Normative client behavior (e.g. cynork TUI and chat) is to start each session with a **new thread** by default.
  Resuming a previous thread is done only when the user explicitly requests it at startup (e.g. `cynork tui --resume-thread <thread_selector>` or `cynork chat --resume-thread <thread_selector>`).
- **Active-thread reuse:** When the client submits an OpenAI-compatible interactive chat request without explicit fresh-thread creation, the gateway MUST use the active thread for the effective `(user_id, project_id)` scope.
- **Active-thread rotation:** The gateway MUST rotate to a new active thread after 2 hours of inactivity for that effective scope.
- **Explicit fresh-thread creation:** When the client calls `POST /v1/chat/threads`, the gateway MUST create and return a distinct new thread for the authenticated user and effective project scope.
- A thread created or reused under one effective project scope MUST NOT be silently rebound to a different project scope by a later chat request.
- Explicit fresh-thread creation MUST NOT require or imply any CyNodeAI-specific thread identifier in subsequent OpenAI-compatible `POST /v1/chat/completions` or `POST /v1/responses` requests.
- Explicit fresh-thread creation for one client or session MUST NOT retroactively change the active-thread continuity used by unrelated clients or sessions outside that explicit operation.

## Thread Title

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ThreadTitle` <a id="spec-cynai-usrgwy-chatthreadsmessages-threadtitle"></a>

### Thread Title Traces To

- [REQ-USRGWY-0142](../requirements/usrgwy.md#req-usrgwy-0142)
- [REQ-CLIENT-0200](../requirements/client.md#req-client-0200)
- [REQ-PMAGNT-0119](../requirements/pmagnt.md#req-pmagnt-0119)

The gateway MUST allow the authenticated thread owner to update a thread title without creating a new thread.

- `PATCH /v1/chat/threads/{thread_id}` MUST accept a `title` field in the request body.
- The gateway MUST derive ownership from authentication and MUST reject updates for threads not owned by the authenticated user.
- The gateway MUST update `updated_at` when a title change succeeds.
- When `title` is absent or empty, clients MAY fall back to a derived label such as the first user-message preview or `New chat`.

#### Auto-Titling by PMA

- The system (PMA or the orchestrator path serving chat) MUST set the thread title automatically after the first user message in the thread when the user has not already set a title.
- Auto-titling MUST NOT overwrite a title the user has already set (e.g. via `PATCH` or at creation).
- The title MAY be derived from the first user message (e.g. truncated plain-text preview); implementation is defined in [Project Manager Agent - Thread titling](project_manager_agent.md#spec-cynai-agents-threadtitling).

## Thread Summary

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ThreadSummary` <a id="spec-cynai-usrgwy-chatthreadsmessages-threadsummary"></a>

### Thread Summary Traces To

- [REQ-USRGWY-0143](../requirements/usrgwy.md#req-usrgwy-0143)
- [REQ-CLIENT-0201](../requirements/client.md#req-client-0201)

The system MAY maintain an optional short thread summary for list and sidebar display.

- Persisted field: `summary` (text, nullable) on `chat_threads`; max length when present is the value of the `ThreadSummary.MaxLength` constant (see below).
- Any server-derived summary MUST be generated from redacted content only and MUST NOT contain plaintext secrets.
  Client MAY set or clear `summary` on create or via PATCH when the implementation supports it; server MAY generate or update summary asynchronously from redacted message content.
- `GET /v1/chat/threads` and `GET /v1/chat/threads/{thread_id}` responses MUST include a `summary` field when the thread has a non-null summary; the field MAY be null or omitted when the thread has no summary.
- `PATCH /v1/chat/threads/{thread_id}` request body MAY include `"summary": <string | null>` to set or clear the thread summary; the gateway MUST reject PATCH if the string length exceeds `ThreadSummary.MaxLength` with 400 and an error body.

### `ThreadSummary` Constants

- **MaxLength:** 500 (characters).
  Implementations MUST truncate or reject summary values longer than 500 characters.
  When server-generated summaries are produced, they MUST be truncated to at most 500 characters.

### `ThreadSummary` Deferred Implementation

- Server-side async generation of summary from redacted thread content: trigger (e.g. after first message or on thread update), job queue or inline, and update of `chat_threads.summary`.
  When implemented, the generator MUST use only redacted message content and MUST NOT persist plaintext secrets.

## Chat Messages

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.Messages` <a id="spec-cynai-usrgwy-chatthreadsmessages-messages"></a>

A chat message is one turn item in a thread.
Messages are append-only.

### Secret Handling

- Message `content` MUST be the amended (redacted) content.
- The system MUST NOT persist plaintext secrets in chat message content.
- If redaction occurs, it MUST be indicated directly in `content` by replacing detected secrets with the literal string `SECRET_REDACTED`.
- For streaming assistant output, the gateway streams tokens to the client in real time and runs the secret scanner after the upstream stream terminates but before the terminal `[DONE]` event.
  If secrets are detected, the gateway emits a post-stream amendment event carrying the redacted content and persists only the redacted version.
  See [Streaming Redaction Pipeline](openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streamingredactionpipeline) for the full goroutine architecture, synchronization model, and amendment event contract.

### Canonical Persisted Message Fields

- `id` (uuid, pk)
- `thread_id` (uuid)
- `role` (text)
  Allowed values SHOULD include `user`, `assistant`, and `system`.
- `content` (text)
- `created_at` (timestamptz)
- `metadata` (jsonb, optional)
  For example model identifier, tool-call summary, or client identifiers.

### Canonical Message Metadata Keys

- `model_id` (string, optional)
- `parts` (jsonb array, optional)
- `response_id` (string, optional)
- `turn_kind` (string, optional)
- `reasoning_redacted` (boolean, optional)

### Message Constraints

- Foreign key: `thread_id` references `chat_threads.id`.
- Index: (`thread_id`, `created_at`)

## Chat History List Behavior

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.HistoryList` <a id="spec-cynai-usrgwy-chatthreadsmessages-historylist"></a>

### History List Traces To

- [REQ-USRGWY-0144](../requirements/usrgwy.md#req-usrgwy-0144)
- [REQ-CLIENT-0199](../requirements/client.md#req-client-0199)

`GET /v1/chat/threads` is the canonical history-list endpoint for rich chat clients.

- The endpoint MUST support `limit` and `offset` pagination.
- The endpoint MUST support recent-activity ordering with `updated_at_desc` as the default.
- The endpoint SHOULD support filtering by `project_id`.
- Each listed thread object MUST include `id`, `created_at`, `updated_at`, and a display label source (`title` when set, otherwise a server-defined fallback).
- When available, the response SHOULD also include `summary`, `project_id`, and any lightweight metadata needed for history rendering.

## Archive and Soft Delete

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.Archive` <a id="spec-cynai-usrgwy-chatthreadsmessages-archive"></a>

### Archive Traces To

- [REQ-USRGWY-0145](../requirements/usrgwy.md#req-usrgwy-0145)

Archive support is optional but, when implemented, it MUST be represented as reversible hidden-from-default state rather than destructive deletion.

- Persisted field: `archived_at` (timestamptz, nullable) on `chat_threads`; when non-null, the thread is archived and excluded from the default list.
- `PATCH /v1/chat/threads/{thread_id}` request body MAY include `"archived": true | false`.
  When `archived: true`, the gateway sets `archived_at` to the current server time (UTC); when `archived: false`, the gateway sets `archived_at` to null.
  Ownership and authorization are the same as for other PATCH operations on the thread.
- `GET /v1/chat/threads` query parameters MUST include optional `archived` (boolean) when archive is implemented.
  Semantics: when `archived` is absent or `false`, the response MUST exclude threads with non-null `archived_at`; when `archived` is `true`, the response MUST include only threads with non-null `archived_at`.
  Pagination and ordering apply independently of the `archived` filter.
- Retention and purge rules apply to archived threads the same as active threads unless a separate policy is defined.

### `Archive` Deferred Implementation

- Add `archived_at` column to `chat_threads` if not present; implement list filtering and PATCH handling for `archived` as specified above.

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
  - `attachment_ref`: metadata for user files accepted through the documented `@`-reference upload flow.
  - `download_ref`: assistant-produced downloadable file metadata.
- User-authored chat input remains text as defined by the OpenAI-compatible chat contract.
  Clients MUST NOT rely on arbitrary rich user-message part kinds beyond the normalized `attachment_ref` metadata that results from accepted `@` file references.

### Ordering and Grouping

- When one interactive chat request produces multiple assistant-side output items, the gateway MUST preserve those items in `metadata.parts` order under one logical assistant turn.
- The gateway SHOULD persist one assistant message row with ordered `metadata.parts` for that logical turn rather than multiple unrelated assistant message rows.
- `content` for that assistant message MUST be the plain-text projection of the visible `text` parts only, in order.
  It MAY be empty when a turn has no visible text parts.
- Incremental stream events used during in-flight rendering are transport artifacts only.
  They MUST reconcile into the same final logical assistant turn rather than being persisted as separate transcript rows.

### Thinking and Reasoning

- `thinking` parts MUST NOT be copied into canonical plain-text `content`.
- `thinking` parts MUST NOT be used for thread title generation, summaries, or default list previews.
- When retained, `thinking` parts MUST remain associated with the same logical assistant turn in thread-history retrieval so rich clients can reveal them later without reconstructing prior raw provider output.
- When `thinking` parts are collapsed in a rich client, the client SHOULD still be able to render a visible secondary affordance that indicates thinking is available without flattening that content into normal assistant prose.
- If preserved, `thinking` parts SHOULD be bounded in size.
  When full reasoning is not retained, the gateway MAY instead persist a bounded reasoning summary or `reasoning_redacted: true` metadata.

### Tool and File Metadata

- Tool-call arguments and tool-result previews stored in `metadata.parts` MUST already be redacted and SHOULD be bounded in size.
- `attachment_ref` and `download_ref` parts SHOULD carry stable identifiers and user-displayable metadata such as filename, media type, and size when known.
- Local machine-specific file paths MUST NOT be persisted as canonical transcript content.

## Download References

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.DownloadRefs` <a id="spec-cynai-usrgwy-chatthreadsmessages-downloadrefs"></a>

When a completion or tool execution produces an assistant-visible file, the gateway SHOULD surface that file as structured `download_ref` metadata rather than only mentioning it in prose.
The PMA (and other assistant responses) MAY provide users with **download links** for such artifacts (e.g. a URL to `GET /v1/artifacts/{artifact_id}`); access to those links MUST be gated by RBAC so that only users authorized to read the artifact can retrieve it (see [Orchestrator Artifacts Storage - RBAC](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsrbac)).

- **Metadata shape (in `metadata.parts` or equivalent):** Each `download_ref` part MUST include `download_id` (string), `filename` (string), and SHOULD include `media_type` (string) and `size_bytes` (integer) when known.
  The part MAY include a user-facing `url` (string) that points to the artifact retrieval endpoint (e.g. `{gateway_base}/v1/artifacts/{download_id}`) so clients can render a clickable download link; when present, the same RBAC applies when the user follows the link.
  Optional fields: `sha256` (string), `source_task_id` (string), `expires_at` (ISO 8601 timestamp string), `description` (string).
- **Retrieval contract:** The gateway MUST expose retrieval via the unified artifacts endpoint **`GET /v1/artifacts/{artifact_id}`** with RBAC as defined in [Orchestrator Artifacts Storage](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsapicrud).
  The `download_id` in `download_ref` MUST be the same identifier used as `artifact_id` for that endpoint.
  Authorization MUST match the originating user, thread, and project visibility; 403 MUST be returned when the caller is not authorized.
- **Response codes for artifact retrieval:** 200 OK with body; 404 Not Found when the artifact is unknown or removed; 410 Gone when the download has expired (if `expires_at` is used); 403 Forbidden when the caller is not authorized.
  A 404 or 410 MUST NOT break transcript or thread retrieval; clients MUST treat failed download as a non-fatal error for the transcript view.
- Clients MUST require explicit user action to download; the system MUST NOT auto-download assistant files as part of transcript rendering.

### Download References Implementation

- Retrieval, storage, and RBAC for assistant-produced files are specified in [Orchestrator Artifacts Storage](orchestrator_artifacts_storage.md): [Artifacts API (CRUD)](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsapicrud) (Read), [RBAC for Artifacts](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsrbac), and [Algorithm: Database and Blob Storage](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsdbblobalgo).
- The gateway MUST serve retrieval via `GET /v1/artifacts/{artifact_id}` per that spec; `download_id` in `download_ref` is the `artifact_id`.
- Remaining implementation: emit `download_ref` parts in completion and tool-result paths when the assistant produces file output; persist or stage those files in the orchestrator artifacts backend (S3 + DB metadata) with the scope and expiry semantics above.

### Download References Traces To

- [REQ-USRGWY-0141](../requirements/usrgwy.md#req-usrgwy-0141)
- [REQ-CLIENT-0194](../requirements/client.md#req-client-0194)

## Chat File Upload Storage

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.FileUploadStorage` <a id="spec-cynai-usrgwy-chatthreadsmessages-fileuploadstorage"></a>

### Chat File Upload Storage Traces To

- [REQ-USRGWY-0140](../requirements/usrgwy.md#req-usrgwy-0140)

When the gateway accepts file uploads or inline file payloads for chat input under the documented `@` workflow, the resulting stored representation MUST remain scoped to the authenticated user and associated thread or message.
When the originating thread is project-scoped, the uploaded file MUST inherit the same project-scoped visibility and authorization as that thread and message.

- **Backend:** Uploads MUST use the orchestrator's [S3-like artifacts storage](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactss3backend): blob content is stored in the S3-compatible backend; metadata is stored in the database.
- **Upload contract:** The gateway MUST accept uploads via a dedicated endpoint (e.g. `POST /v1/chat/uploads`) and/or inline in the OpenAI-compatible completion request (e.g. content parts with type `file` or `image`).
  It MUST return a stable `file_id` (or equivalent) for each stored file that the client uses in the subsequent chat completion request.
  The implementation MUST write the blob to the orchestrator artifacts backend and record metadata (including `storage_ref`) in the [chat_message_attachments](chat_threads_and_messages.md#spec-cynai-schema-chatmessageattachmentstable) table (or equivalent with the same columns and constraints), associating the attachment with the authenticated user and the thread or message when the completion is processed.
- **Retrieval:** Stored chat files MUST be retrievable via the unified [artifacts endpoint](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsapicrud) `GET /v1/artifacts/{artifact_id}` with RBAC; `file_id` (or the attachment's stable id) serves as `artifact_id`.
- **Secret redaction:** Secret redaction MUST be applied to file content before persistence; files that fail redaction MUST be rejected with 400.
- **Size and type:** This spec does not define file size or media-type limits; they MAY be added in a later revision.

### Chat File Upload Storage Implementation

- Backend, upload contract, retrieval, and RBAC are specified in [Orchestrator Artifacts Storage](orchestrator_artifacts_storage.md): [S3-Like Block Storage](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactss3backend), [Artifacts API (CRUD)](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsapicrud) (Create, Read), [RBAC for Artifacts](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsrbac), and [Algorithm: Database and Blob Storage](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsdbblobalgo).
- The gateway MAY expose `POST /v1/chat/uploads` that delegates to `POST /v1/artifacts` with thread scope, or accept inline content in the completion request; in both cases blobs MUST be stored in the artifacts store and metadata in [chat_message_attachments](chat_threads_and_messages.md#spec-cynai-schema-chatmessageattachmentstable).
- Remaining implementation: wire the gateway upload path (dedicated and/or inline) to the artifacts API and chat_message_attachments; ensure orchestrator/PMA resolution uses the same store so file content is available in the completion path and thread history replay.

## Context Size Tracking

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ContextSizeTracking` <a id="spec-cynai-usrgwy-chatthreadsmessages-contextsizetracking"></a>

### Context Size Tracking Traces To

- [REQ-USRGWY-0146](../requirements/usrgwy.md#req-usrgwy-0146)

The system MUST deterministically track the effective context size used to construct the next interactive chat completion for a thread.

- Effective context size MUST account for the composed system or instruction block, retained prior turns included for the request, the current user turn, any accepted file-derived content included in the request, and any compacted context artifact injected for continuity.
- Measurement MAY use exact provider tokenization, a deterministic tokenizer matched to the selected model family, or a deterministic estimation adapter, but the same thread state and selected model MUST yield the same measured result.
- The tracked value MUST be computed during request construction before inference handoff.
- The implementation MAY persist or cache the measured value as thread-scoped derived metadata for observability or later decisions, but it MUST NOT rewrite canonical raw chat message content in order to record the measurement.
- Downstream consumers of the tracked value MUST include context-window policy decisions such as compaction and MAY include diagnostics or operator-facing observability.

## Retention and Transcripts

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.RetentionTranscripts` <a id="spec-cynai-usrgwy-chatthreadsmessages-retentiontranscripts"></a>

Raw chat messages are stored as chat messages.
Transcripts are stored separately as transcript segments associated with a session or run.

A transcript segment is a derived artifact.
A transcript segment MAY be a summary, a redacted view, or an operator-facing audit representation.

### Retention Policy Scope

- Session transcripts and run logs per [Runs and Sessions API](runs_and_sessions_api.md).
- Chat threads and chat messages per this spec.

## Relationship to Runs and Sessions

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.Relationships` <a id="spec-cynai-usrgwy-chatthreadsmessages-relationships"></a>

Sessions are user-facing containers for interactive work.
Chat threads MAY be associated with sessions via `chat_threads.session_id`.

Runs remain execution traces.
Runs MAY reference a session id.
Runs MUST NOT be required for every chat message.

### Relationship Outcomes

- Session-level retention and grouping across runs and chat threads.
- Persisting transcripts at the session level without conflating transcripts with raw chat messages.

## API Surface (Data REST)

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ApiSurface` <a id="spec-cynai-usrgwy-chatthreadsmessages-apisurface"></a>

Chat threads and chat messages are the canonical Data REST resources for persisted chat history.
The gateway MUST enforce authorization so users can only read their own chat threads and messages.

### Required Operations

- Create chat thread.
- List chat threads.
- Get chat thread.
- Patch chat thread metadata.
- Append message to thread.
- List messages for thread.

### Standardized Endpoints

- `POST /v1/chat/threads`
  - Create a new chat thread (explicit fresh-thread creation).
  - The request MUST require authentication.
  - Effective project scope MUST use the same chat rule as the OpenAI-compatible surface:
    `OpenAI-Project` when present, otherwise the authenticated user's default project.
  - The request body MAY be empty.
  - The create-thread contract MUST NOT require `project_id` or `title` in the request body.
  - The server MUST derive `user_id` from authentication.
  - The server MUST NOT allow the client to set `session_id`.
  - The response MUST return `201 Created` and the created thread, including `id`, for Data REST retrieval and management purposes.
  - The gateway MUST NOT require the client to send that thread id on subsequent `POST /v1/chat/completions` requests.
- `GET /v1/chat/threads`
  - List chat threads for the authenticated user.
  - SHOULD support filtering by `project_id`.
  - Pagination MUST use `limit` and `offset` query parameters.
  - Recent-first ordering by `updated_at` MUST be the default behavior.
- `PATCH /v1/chat/threads/{thread_id}`
  - Update mutable thread metadata owned by the authenticated user.
  - The MVP mutable field set MUST include `title`.
  - The implementation MAY also support `summary` and archive-state updates when those features are enabled.
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
- [Orchestrator Artifacts Storage](orchestrator_artifacts_storage.md) (chat file uploads, download_ref retrieval, RBAC)
- [Runs and Sessions API](runs_and_sessions_api.md)
- [User API Gateway](user_api_gateway.md)
