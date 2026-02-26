# Model Warm-up Proposal

- [Document Status](#document-status)
- [Problem](#problem)
- [Goals](#goals)
- [Trigger Points](#trigger-points)
- [Options](#options)
  - [Option A: Client-Triggered Warm-up (Explicit)](#option-a-client-triggered-warm-up-explicit)
  - [Option B: Gateway-Triggered on First Chat Request](#option-b-gateway-triggered-on-first-chat-request)
  - [Option C: Gateway Warm-up Endpoint; CLI/Web Call It on Session Start](#option-c-gateway-warm-up-endpoint-cliweb-call-it-on-session-start)
  - [Option D: Backend (Orchestrator/pma/PMA) Periodic or On-Demand Keep-Warm](#option-d-backend-orchestratorpmapma-periodic-or-on-demand-keep-warm)
- [Recommended Direction](#recommended-direction)
- [Open Questions](#open-questions)
- [References](#references)

## Document Status

Reduces perceived latency when a user starts a chat session (`cynork chat` or Web Console chat UI).

## Problem

The first chat completion after starting a session (or after idle) can be slow because:

- **Ollama** (and similar backends) load the model into memory on first use.
  Until then, the first request pays cold-start cost (load + first token).
- Chat is served via `POST /v1/chat/completions`; routing is either **cynodeai.pm** (PMA, which calls Ollama) or **direct inference** (gateway calls Ollama).
  In both paths the bottleneck is inference backend readiness.

Warming the model before the user sends their first real message improves perceived responsiveness.

## Goals

- Reduce time-to-first-meaningful-response when the user enters chat (CLI or Web).
- Keep warm-up best-effort: failure must not block or break the chat session.
- Avoid duplicate warm-up work when multiple clients or tabs open chat.
- Stay compatible with existing API and auth; no new required client behaviour for correctness.

## Trigger Points

- **Trigger:** `cynork chat`
  - description: When the CLI starts an interactive chat session (after auth, before first prompt).
- **Trigger:** Web Console chat
  - When the user navigates to the chat interface (e.g. route or view load).
  - To be defined when the Web Console is built.

In both cases we want the **default chat model** (typically `cynodeai.pm` and its underlying inference model) to be warmed before the first user message.

## Options

The following approaches are possible; each has different trade-offs for client, gateway, and backend.

### Option A: Client-Triggered Warm-up (Explicit)

- **CLI:** After starting the chat loop and before showing the first prompt, `cynork` calls a dedicated gateway endpoint (e.g. `POST /v1/chat/warm` or `GET /v1/models` with a warm-up hint) or a minimal `POST /v1/chat/completions` with a no-op/empty message that the gateway treats as warm-only.
- **Web:** Same: when the chat view loads, the front end calls the warm-up endpoint once.
- **Gateway:** New optional endpoint (e.g. `POST /v1/chat/warm`) that triggers backend warm-up for the default (or requested) model and returns immediately; or gateway interprets a special request and triggers warm-up in the background without running a full completion.
- **Pros:** Clear trigger; client controls when to warm. **Cons:** New endpoint or request shape; clients must be updated.

### Option B: Gateway-Triggered on First Chat Request

- No new client behaviour.
  When the gateway receives the first `POST /v1/chat/completions` for a given model (or default), it could:
  - Option B1: Perform warm-up synchronously before processing the real request (adds latency to that first request).
  - Option B2: Start warm-up asynchronously and process the request as today (first request still cold; subsequent sessions benefit if warm-up is reused).
- **Pros:** No client changes. **Cons:** B1 worsens first-request latency; B2 does not help the first request.

### Option C: Gateway Warm-up Endpoint; CLI/Web Call It on Session Start

- Gateway exposes a best-effort **warm-up** endpoint (e.g. `POST /v1/chat/warm`), optionally with `model` (default: `cynodeai.pm`).
  Requires auth like chat.
- Gateway triggers backend warm-up: for `cynodeai.pm` it calls PMA (or the inference URL PMA uses); for direct inference it calls the inference backend.
  Backend warm-up = call Ollama with a no-op (e.g. empty prompt) or a dedicated load API if available (Ollama can load via `POST /api/generate` with empty prompt).
- **CLI:** In `runChat` (or equivalent), after auth and before entering the chat loop, fire a single warm-up request (same token, optional model from session/default).
  Do not block the prompt on completion; run warm-up in background or with short timeout so the user can type immediately.
- **Web:** On chat route/view load, call the warm-up endpoint once (e.g. fire-and-forget or short timeout).
- **Pros:** Explicit, predictable; first user message often hits a warm model. **Cons:** One extra endpoint and client call per session.

### Option D: Backend (Orchestrator/pma/PMA) Periodic or On-Demand Keep-Warm

- Orchestrator or PMA periodically (or on a health check) calls Ollama to load the configured model so it stays in memory.
  No client or gateway API change.
- **Pros:** No client changes; model can stay warm. **Cons:** Resource usage when idle; may not match the exact model the user selects; less predictable for "first request after session start."

## Recommended Direction

- **Prefer Option C** (dedicated warm-up endpoint + CLI/Web call on session start):
  - Single, clear contract: "warm this model for chat."
  - Clients (cynork, Web) trigger it at the right moment (session start) without changing the chat completion API.
  - Warm-up remains best-effort: gateway returns 200 quickly (e.g. "warm-up started" or "already warm") and does actual loading asynchronously, or with a short timeout so the client does not block.
- **Backend implementation:** For Ollama, warm-up = `POST baseURL/api/generate` with the target model and an empty or minimal prompt; discard the response.
  Alternatively use a dedicated load API if Ollama exposes one and we adopt it.
  For PMA path, gateway (or a small internal call) triggers the same on the PMA's inference URL so the model PMA uses is warmed.
- **Idempotency:** Multiple rapid warm-up calls (e.g. several tabs or restarts) should be safe: backend can debounce or ignore duplicate in-flight warm-up for the same model.

## Open Questions

1. **Endpoint shape:** `POST /v1/chat/warm` with optional `model` in body vs. query vs. header; and whether to return 202 (accepted) vs. 200 after triggering.
2. **Timeout and behaviour:** Should the gateway wait for Ollama to finish loading (with a cap, e.g. 30s) or return immediately and warm in background?
3. **Web Console:** Exact trigger (route enter, first focus, etc.) and whether to warm only on first navigation or also on tab focus (to be defined when the Web chat UI is built).
4. **Observability:** Log or metric when warm-up is requested and when it succeeds/fails (to avoid masking backend issues).

## References

- Chat API: `docs/tech_specs/openai_compatible_chat_api.md`
- CLI chat: `docs/tech_specs/cli_management_app_commands_chat.md`
- Orchestrator chat routing: `orchestrator/internal/handlers/openai_chat.go` (routeAndComplete; PMA vs direct inference)
- Ollama: loading model via `POST /api/generate` with empty prompt is a common warm-up approach; see Ollama API docs for any dedicated load endpoint.
