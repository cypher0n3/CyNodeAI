# Draft: Chat QOL Spec and Requirements

## Summary

- **Date:** 2026-03-02
- **Purpose:** Draft requirements and spec additions for chat quality-of-life improvements: chat history UX, naming/renaming chats, chat summaries, and related behavior.
- **Status:** Draft only; not yet merged into `docs/requirements/` or `docs/tech_specs/`.
- **Existing baseline:** [Chat Threads and Messages](../tech_specs/chat_threads_and_messages.md) (CYNAI.USRGWY.ChatThreadsMessages), [REQ-USRGWY-0130](../requirements/usrgwy.md#req-usrgwy-0130).

This document proposes new requirements and technical spec extensions so that clients can offer a better chat UX: visible history, user-controlled thread names, and optional summaries in list views.

## Proposed Requirements

The following requirement IDs are **proposed** and would live in the indicated requirements file if accepted.

### Gateway and Data (USRGWY)

- **REQ-USRGWY-0132 (proposed):** The Data REST API for chat threads MUST support updating a thread's user-facing title.
  Clients MUST be able to set and change the display name of a thread without creating a new thread.
  The gateway MUST derive `user_id` from authentication and MUST allow updates only for threads owned by that user.

- **REQ-USRGWY-0133 (proposed):** The system MAY store an optional short summary for a chat thread (e.g. for list/sidebar display).
  If supported, the summary MUST be derived or set in a way that does not require storing plaintext secrets; any summary derived from message content MUST use redacted content only.
  Summary generation MAY be best-effort or asynchronous.

- **REQ-USRGWY-0134 (proposed):** List chat threads endpoints MUST support sort order by `updated_at` (default: descending) and MUST support pagination so clients can implement "chat history" lists of arbitrary size.

- **REQ-USRGWY-0135 (proposed):** The gateway MAY support soft-delete or archive state for chat threads so that users can hide threads from the default history list without losing data.
  If supported, list endpoints MUST allow filtering by visibility (e.g. active vs archived) and retention MUST still apply per existing policy.

### Client (Client)

- **REQ-CLIENT-0178 (proposed):** Clients that provide a chat UI (e.g. Web Console, CLI chat) MUST expose a way for the user to view chat history (list of threads for the current user and project context).
  The list MUST show thread title (or a fallback such as first message preview or "Untitled") and SHOULD show last activity time.

- **REQ-CLIENT-0179 (proposed):** Clients that provide a chat UI MUST allow the user to rename the current thread (set or update title) and SHOULD allow renaming from the thread list.

- **REQ-CLIENT-0180 (proposed):** When the gateway provides a thread summary, clients SHOULD display it in the thread list or sidebar (e.g. tooltip or subtitle) to help users identify conversations without opening them.

## Proposed Spec Additions

These would extend [Chat Threads and Messages](../tech_specs/chat_threads_and_messages.md) and related specs.

### Thread Title (Naming)

- Spec ID (proposed): `CYNAI.USRGWY.ChatThreadsMessages.ThreadTitle`

- The existing thread model already recommends `title` (text, optional).
  Spec addition: the gateway MUST allow `PATCH /v1/chat/threads/{thread_id}` with a request body that includes `title` (string).
  The gateway MUST reject PATCH for threads not owned by the authenticated user.
  No other thread fields need to be mutable for MVP; only `title` and `updated_at` (server-maintained) are updated.

- Auto-title: the spec MAY define that when `title` is absent, clients display a fallback (e.g. first N characters of the first user message, or "New chat").
  Server-side auto-generation of title from first message is optional and MAY be added later; if added, it MUST use redacted content only.

### Thread Summary

- Spec ID (proposed): `CYNAI.USRGWY.ChatThreadsMessages.ThreadSummary`

- Optional field on thread: `summary` (text, optional), max length TBD (e.g. 200-500 characters).
  If present, it is a short plaintext summary for list/sidebar display.
  Generation: MAY be set by client on create/update; MAY be generated asynchronously by the server from redacted message content; MUST NOT contain plaintext secrets.
  Storage: add `summary` to `chat_threads` if this feature is implemented; index not required for MVP.

- API: `GET /v1/chat/threads` and `GET /v1/chat/threads/{thread_id}` responses SHOULD include `summary` when present.
  Optional: `PATCH /v1/chat/threads/{thread_id}` MAY allow client to set or clear `summary`.

### Chat History List Behavior

- Spec ID (proposed): `CYNAI.USRGWY.ChatThreadsMessages.HistoryList`

- `GET /v1/chat/threads` behavior to support "chat history" UX:
  - Query parameters: existing `project_id` filter and pagination (`limit`, `offset`).
  - Add optional `sort` (e.g. `updated_at_asc` | `updated_at_desc`); default MUST be `updated_at_desc`.
  - Add optional `archived` (boolean) if soft-delete/archive is implemented: when `false` (default), exclude archived threads; when `true`, return only archived threads.
  - Response: each thread object MUST include `id`, `title`, `updated_at`, `created_at`, and optionally `summary`, `project_id`, `message_count` (if the gateway chooses to expose it).

### Archive / Soft-Delete (Optional)

- Spec ID (proposed): `CYNAI.USRGWY.ChatThreadsMessages.Archive`

- If REQ-USRGWY-0135 is accepted: add optional `archived_at` (timestamptz, nullable) to `chat_threads`.
  When non-null, the thread is considered archived.
  `PATCH /v1/chat/threads/{thread_id}` MAY allow `archived: true | false`; setting `archived: true` sets `archived_at` to current time; `false` clears it.
  List threads MUST filter by `archived_at` according to the `archived` query parameter.
  Retention and purge rules apply to archived threads the same as active ones unless a separate policy is defined.

## Traceability (Proposed)

- **Requirement (proposed):** REQ-USRGWY-0132
  - spec section (proposed): Thread title (PATCH, naming)
- **Requirement (proposed):** REQ-USRGWY-0133
  - spec section (proposed): Thread summary
- **Requirement (proposed):** REQ-USRGWY-0134
  - spec section (proposed): History list (sort, pagination)
- **Requirement (proposed):** REQ-USRGWY-0135
  - spec section (proposed): Archive
- **Requirement (proposed):** REQ-CLIENT-0178
  - spec section (proposed): History list API + client display
- **Requirement (proposed):** REQ-CLIENT-0179
  - spec section (proposed): Thread title API + client rename
- **Requirement (proposed):** REQ-CLIENT-0180
  - spec section (proposed): Thread summary in list

## Out of Scope for This Draft

- Full-text search over chat message content (future).
- Export of thread to file (e.g. Markdown/JSON); can be a later requirement.
- Per-message labels or reactions.
- Thread pinning/favorite as a separate flag (could be modeled later as a tag or `pinned_at`).

## Related Documents

- [Chat Threads and Messages](../tech_specs/chat_threads_and_messages.md)
- [OpenAI-Compatible Chat API](../tech_specs/openai_compatible_chat_api.md)
- [User API Gateway requirements](../requirements/usrgwy.md) (REQ-USRGWY-0130)
- [Client requirements](../requirements/client.md) (chat-related REQ-CLIENT-0161 onward)
