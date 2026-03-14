# Cynork TUI - Architecture Note

## Overview

The TUI is a rich client over the gateway's OpenAI-compatible chat and thread-management REST APIs.
It does not send CyNodeAI-specific thread IDs on `POST /v1/chat/completions` or `POST /v1/responses`; thread association is server-managed per `(user_id, project_id)` and explicit `POST /v1/chat/threads` for new threads.

**Spec alignment:** [cynork_tui.md](cynork_tui.md), [chat_threads_and_messages.md](chat_threads_and_messages.md), [openai_compatible_chat_api.md](openai_compatible_chat_api.md).

## Layers

Summary of implementation layers.

### Entry (`Cmd/tui.go`)

- `cynork tui`: default = create new thread via `POST /v1/chat/threads` before first completion.
- `cynork tui --resume-thread <selector>`: start in an existing thread; selector is user-typeable (ordinal, short id, or title).
- Startup token failure opens in-session login recovery instead of exiting.

### Session (Internal/chat)

- Holds gateway client, transport (completions or responses), model, project.
- Thread create/list/get/patch use same gateway thread APIs; no thread id in completion/response bodies.

### Tui State (Internal/tui)

- **Transcript:** ordered `TranscriptTurn[]`; each turn has role, content, optional `Parts[]` (text, thinking, tool_call, tool_result, attachment_ref, download_ref).
  Prefer `metadata.parts` when present; fall back to canonical content.
- **In-flight:** exactly one assistant turn is "in flight" during streaming; updated in place; status chip attached to that turn (not prose).
- **Streaming reducer:** deltas append to in-flight visible text; `cynodeai.amendment` (secret_redaction) replaces that text in place; final reconciliation replaces in-flight row with final turn without duplicating content.
- **Thinking:** hidden by default; collapsed placeholder with `/show-thinking` hint; `/show-thinking` and `/hide-thinking` toggle presentation and persist `tui.show_thinking_by_default`.

### Gateway Client (Internal/gateway)

- Chat: `POST /v1/chat/completions` and `POST /v1/responses` with `OpenAI-Project` header only (no thread id in body).
- Threads: `POST /v1/chat/threads`, `GET /v1/chat/threads`, `GET /v1/chat/threads/{id}`, `PATCH /v1/chat/threads/{id}`.
- SSE: parses `event: cynodeai.amendment` and passes amendment (e.g. secret_redaction) to the TUI stream consumer.

### Config / Cache

- TUI prefs (e.g. `tui.show_thinking_by_default`) live in the same cynork config file; atomic writes.
  Cache holds no transcript content or secrets.

## Data Flow (Streaming)

- User sends message -> TUI appends user turn, creates one in-flight assistant turn with status chip.
- Gateway streams SSE deltas -> TUI appends to in-flight visible text.
- Optional amendment event -> TUI replaces in-flight visible text with redacted content.
- Terminal event -> TUI reconciles: remove chip, replace in-flight row with final assistant turn (no duplicate text).

## Thread Selectors

- List from `GET /v1/chat/threads` is shown with user-typeable selectors: list ordinal (e.g. `1`), short id prefix, or title when unambiguous.
- `/thread switch <selector>` and `--resume-thread <selector>` accept the same selector form.
