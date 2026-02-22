# Proposal: Chat Conversation Context Support

- [Summary](#summary)
- [Current Behavior](#current-behavior)
- [Goals](#goals)
- [Existing Context](#existing-context)
- [Proposed Direction](#proposed-direction)
- [Implementation Outline](#implementation-outline)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)

## Summary

**Date:** 2026-02-21  
**Type:** Proposal  
**Status:** Draft

Add support for multi-turn conversation context to the User API Gateway chat endpoint (`POST /v1/chat`) so that the model receives prior turns and can maintain coherent dialogue.
Today chat is stateless: each request sends a single message and receives a single response with no session or history.

## Current Behavior

- **Request:** `POST /v1/chat` body is `{ "message": "..." }`.
  No session ID, no history.
- **Handler:** Creates a new task per request, passes only `req.Message` to inference or to the job payload.
- **Inference:** `orchestrator/internal/inference` calls Ollama `/api/generate` with a single `prompt`.
  No conversation context is sent.

References: `orchestrator/internal/handlers/tasks.go` (Chat, ChatRequest), `orchestrator/internal/inference/client.go` (CallGenerate).

## Goals

1. Allow clients to send a sequence of messages in one conversation and have the model use prior turns as context.
2. Optionally persist conversation history for UX (e.g. resume, transcripts) and align with existing session/transcript concepts where applicable.
3. Keep backward compatibility: requests without a session or history continue to work as today (single-turn).

## Existing Context

- **Sessions and transcripts:** [runs_and_sessions_api.md](../docs/tech_specs/runs_and_sessions_api.md) defines sessions as containers for interactive work (e.g. chat thread) with transcripts; [REQ-USRGWY-0110](../docs/requirements/usrgwy.md) requires storable transcripts with configurable retention.
- **CLI chat:** [cynork_cli.md](../docs/tech_specs/cynork_cli.md) specifies `cynork chat` as an interactive loop sending one message at a time to the gateway; adding context would allow the gateway to maintain the thread for that client.
- **Open WebUI:** [openwebui_cynodeai_integration.md](../docs/openwebui_cynodeai_integration.md) describes OpenAI-compatible chat; OpenAI format is message-list based and expects conversation context.

## Proposed Direction

We recommend the following approach.

### 1. API: Optional Session and Message List

Two options: session-based (A) or client-supplied message list (B).

#### 1.1. Option A - Session Id + Single Message (Recommended for Mvp)

- Request body: `{ "message": "...", "session_id": "<uuid>" }` (optional).
- If `session_id` is present and valid, load that conversation's last N turns (or full history within a cap), append the new user message, send full context to the model, then store the new user message and model reply in that session.
- If `session_id` is omitted or invalid, behave as today: single message in, single response out (no context).

#### 1.2. Option B - Explicit Message List

- Request body: `{ "messages": [ { "role": "user", "content": "..." }, ... ] }` (and optionally `session_id` to persist).
- Client sends full history each time.
  No server-side history required for inference; session only for persistence/audit.

Recommendation: **Option A** for MVP.
Clients stay simple (one message per request); server owns loading and persisting history and fits CLI and future Open WebUI adapter (adapter can create/use session and send one message per request).

### 2. Chat Storage

- **New tables (or extend sessions):**
  - **Chat session:** `chat_sessions` - `id` (uuid), `user_id` (uuid), `created_at`, `updated_at`; optionally `title` (e.g. first message snippet).
    Tied to user for isolation and retention.
  - **Chat messages:** `chat_messages` - `id` (uuid), `session_id` (uuid), `role` (user | assistant), `content` (text), `created_at`.
    Ordered by `created_at` for building context.
- **Retention:** Apply configurable retention (e.g. max messages per session, max sessions per user, TTL).
  Reference REQ-USRGWY-0110 and runs_and_sessions_api.md transcripts.
- **Alternative:** Reuse generic "sessions" and "transcripts" from runs_and_sessions_api if the schema can represent chat turns (e.g. transcript as JSON array of { role, content }).
  Either extend that schema or add chat-specific tables and later map to sessions/transcripts for a unified view.

### 3. Inference: Send Context to the Model

- **Ollama:** Use `/api/chat` with a `messages` array (Ollama supports this) instead of `/api/generate` with a single prompt.
  Build `messages` from session history (and optionally a system prompt) plus the new user message; append model reply when storing.
- **Orchestrator inference client:** Add `CallChat(ctx, client, baseURL, model, messages []Message) (string, error)` (or equivalent) that POSTs to `/api/chat` with `messages`.
  Keep `CallGenerate` for single-turn and non-chat task flows.
- **Context window:** Cap total messages or total token count (if available) per request to avoid overrunning model limits; drop or summarize oldest turns if needed.

### 4. Handler Flow (Option A)

1. Parse `message` and optional `session_id`.
2. If `session_id` present: load session and recent messages (e.g. last 20 or last N tokens); validate session belongs to authenticated user.
   If missing/invalid, treat as no session.
3. Build context: optional system message + history + new user message.
4. If orchestrator inference is configured: call `CallChat` with that context; on success, store user message and assistant reply in session (create session if new), return reply.
5. If no inference or fallback: create task/job as today (single prompt); optionally attach to session for transcript only (model still sees only current message until we pass history into sandbox/inference there).
6. If no session: behave as today (single turn, no persistence).

### 5. Backward Compatibility

- No `session_id` -> same as current behavior (stateless, one message, one response).
- Invalid or missing session -> ignore session, proceed stateless.
- Existing clients and tests that send only `message` remain valid.

## Implementation Outline

1. **Schema:** Migration adding `chat_sessions` and `chat_messages` (or equivalent under runs_and_sessions), plus retention/limits config.
2. **Store layer:** Create/Get session by ID (and user); append message; list recent messages for a session (with cap).
3. **Inference:** Implement `CallChat` (or equivalent) for Ollama `/api/chat` with `messages` array; keep `CallGenerate` unchanged.
4. **Handler:** Extend `Chat` to accept optional `session_id`; load history, call `CallChat` when inference is used and session is present; persist turns; return response.
   Fallbacks as today when no session or no inference.
5. **Optional:** `POST /v1/chat/sessions` to create a session and return `session_id`; `GET /v1/chat/sessions` to list user's sessions; `GET /v1/chat/sessions/:id/messages` for transcript.
   Can follow after MVP.
6. **Tests:** Unit tests for new store and inference paths; BDD for "chat with session_id returns response and persists turn"; optional E2E with two messages in same session.

## Alternatives Considered

- **Client sends full history (Option B):** Simpler server (no session storage for inference), but larger payloads and client complexity; less aligned with CLI "send one line at a time" and with transcript retention in one place.
- **No persistence:** Only pass history in request (client sends last N messages).
  No server-side transcript; REQ-USRGWY-0110 and session/transcript concepts would be addressed separately for chat.
- **Reuse only runs/sessions:** Possible if we map one "chat session" to one session and store transcript as structured blob; schema in runs_and_sessions_api may need extension for chat messages.
  Chat-specific tables keep chat simple and can be linked to sessions later.

## References

- [runs_and_sessions_api.md](../docs/tech_specs/runs_and_sessions_api.md) - Sessions, transcripts
- [REQ-USRGWY-0110](../docs/requirements/usrgwy.md) - Transcripts storable with retention
- [user_api_gateway.md](../docs/tech_specs/user_api_gateway.md) - Core capabilities, interactive sessions
- [cynork_cli.md](../docs/tech_specs/cynork_cli.md) - Chat command
- [openwebui_cynodeai_integration.md](../docs/openwebui_cynodeai_integration.md) - OpenAI-style chat backing
- Current implementation: `orchestrator/internal/handlers/tasks.go` (Chat), `orchestrator/internal/inference/client.go` (CallGenerate)
