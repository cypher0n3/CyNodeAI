# Plan: OpenAI-Compatible Chat for Orchestrator and Cynork

- [Overview](#overview)
- [Sources of Truth (Requirements and Specs)](#sources-of-truth-requirements-and-specs)
  - [Primary Requirements](#primary-requirements)
  - [Primary Tech Specs](#primary-tech-specs)
  - [Current Implementation Touchpoints (Starting State)](#current-implementation-touchpoints-starting-state)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Implementation Goals](#implementation-goals)
  - [Non-Goals (For This Plan)](#non-goals-for-this-plan)
- [Design Decisions to Lock Early](#design-decisions-to-lock-early)
  - [1) Legacy Chat Endpoint (`POST /v1/chat`)](#1-legacy-chat-endpoint-post-v1chat)
  - [2) Model Identifiers for `GET /v1/models`](#2-model-identifiers-for-get-v1models)
  - [3) Chat Thread Association for Persistence](#3-chat-thread-association-for-persistence)
- [Work Breakdown](#work-breakdown)
  - [A) Orchestrator: User API Gateway OpenAI-Compatible Endpoints](#a-orchestrator-user-api-gateway-openai-compatible-endpoints)
  - [B) Orchestrator: Chat Threads and Messages Persistence](#b-orchestrator-chat-threads-and-messages-persistence)
  - [C) Orchestrator: Integrate Persistence Into Chat-Completions Flow](#c-orchestrator-integrate-persistence-into-chat-completions-flow)
  - [D) Cynork: Switch Chat to OpenAI-Compatible Gateway Surface](#d-cynork-switch-chat-to-openai-compatible-gateway-surface)
  - [E) BDD and Feature Coverage Alignment](#e-bdd-and-feature-coverage-alignment)
- [Spec Gaps and Implementation Decisions](#spec-gaps-and-implementation-decisions)
  - [OpenAI Wire Format (Exact JSON Shapes)](#openai-wire-format-exact-json-shapes)
  - [Chat Error Response Format](#chat-error-response-format)
  - [Inference Path: Messages to Completion](#inference-path-messages-to-completion)
  - [Thread and Session Identifiers in OpenAI Request](#thread-and-session-identifiers-in-openai-request)
  - [Chat Thread and Message Schema in Postgres](#chat-thread-and-message-schema-in-postgres)
  - [Data REST URL and Method Contract for Threads and Messages](#data-rest-url-and-method-contract-for-threads-and-messages)
  - [Audit Records for Chat Completions](#audit-records-for-chat-completions)
  - [Default Model Identifier and List-Models Contents](#default-model-identifier-and-list-models-contents)
  - [Slash Command Parsing and Gateway Mapping](#slash-command-parsing-and-gateway-mapping)
- [Validation and Quality Gates](#validation-and-quality-gates)
- [Implementation Checklist (Ordered)](#implementation-checklist-ordered)

## Overview

**Date:** 2026-02-22
**Type:** Implementation plan
**Status:** Draft

This plan describes the work needed to implement the OpenAI-compatible chat surface in the orchestrator User API Gateway and update `cynork` to use it.
It also covers the related persistence and BDD feature coverage required by the referenced requirements and tech specs.

## Sources of Truth (Requirements and Specs)

This section lists the requirements and tech specs this plan traces to.

### Primary Requirements

- **User gateway compatibility and chat contract**
  - `docs/requirements/usrgwy.md`
    - `REQ-USRGWY-0121`, `REQ-USRGWY-0124`, `REQ-USRGWY-0125`, `REQ-USRGWY-0127`, `REQ-USRGWY-0128`, `REQ-USRGWY-0129`, `REQ-USRGWY-0130`
- **Orchestrator reliability for chat completions**
  - `docs/requirements/orches.md`
    - `REQ-ORCHES-0131`, `REQ-ORCHES-0132`
- **Cynork chat UX requirements**
  - `docs/requirements/client.md`
    - `REQ-CLIENT-0161` through `REQ-CLIENT-0173`

### Primary Tech Specs

- **OpenAI-compatible endpoints and behaviors**
  - `docs/tech_specs/openai_compatible_chat_api.md`
    - `CYNAI.USRGWY.OpenAIChatApi` and subsections
- **User gateway context and constraints**
  - `docs/tech_specs/user_api_gateway.md`
    - `CYNAI.USRGWY.ClientCompatibility`
    - `CYNAI.USRGWY.AuthAuditing`
    - `CYNAI.USRGWY.ChatSlashCommandSupport`
- **Chat persistence model**
  - `docs/tech_specs/chat_threads_and_messages.md`
    - `CYNAI.USRGWY.ChatThreadsMessages` and subsections
- **CLI chat and slash command contract**
  - `docs/tech_specs/cli_management_app_commands_chat.md`
    - `CYNAI.CLIENT.CliChat`
    - `CYNAI.CLIENT.CliChatSlashCommands`
    - `CYNAI.CLIENT.CliChatSlashAutocomplete`
    - `CYNAI.CLIENT.CliChatModelSelection`
    - `CYNAI.CLIENT.CliChatProjectContext`
    - `CYNAI.CLIENT.CliChatSlashTask`, `...Status`, `...Nodes`, `...Prefs`, `...Skills`
- **API implementation standards**
  - `docs/tech_specs/go_rest_api_standards.md`
- **OpenWebUI integration expectations**
  - `docs/openwebui_cynodeai_integration.md`

### Current Implementation Touchpoints (Starting State)

- **User gateway routes include legacy chat**
  - `orchestrator/cmd/user-gateway/main.go` registers `POST /v1/chat`.
- **Legacy chat handler**
  - `orchestrator/internal/handlers/tasks.go` implements `TaskHandler.Chat` using `POST /v1/chat` with `{ "message": "..." }`.
- **Cynork chat currently calls legacy endpoint**
  - `cynork/cmd/chat.go` calls `gateway.Client.Chat()` which uses `POST /v1/chat`.
- **BDD coverage currently targets legacy endpoint**
  - `features/orchestrator/orchestrator_task_lifecycle.feature` includes "Chat returns model response".
  - `orchestrator/_bdd/steps.go` implements steps calling `POST /v1/chat`.

## Goals and Non-Goals

This section describes what "done" means for Phase 1, and what is explicitly out of scope.

### Implementation Goals

- Implement the OpenAI-compatible chat surface exposed by the orchestrator User API Gateway.
  - `GET /v1/models` and `POST /v1/chat/completions` per `docs/tech_specs/openai_compatible_chat_api.md`.
  - Traces to `REQ-USRGWY-0127`.
- Ensure the OpenAI compatibility layer preserves auth, policy constraints, and auditing expectations.
  - Traces to `REQ-USRGWY-0121`, `REQ-USRGWY-0124`, `REQ-USRGWY-0125`.
- Implement reliability behavior for the chat-completions handler.
  - Poll cap and bounded retries with backoff.
  - Traces to `REQ-ORCHES-0131`, `REQ-ORCHES-0132`.
- Track and store chat history as chat threads and messages, independently of task lifecycle.
  - Traces to `REQ-USRGWY-0130`.
- Update `cynork` to use the OpenAI-compatible gateway endpoints and meet the CLI chat contract.
  - Traces to `REQ-CLIENT-0161` through `REQ-CLIENT-0173`.
- Update and/or migrate BDD coverage so chat tests validate the OpenAI-compatible surface.

### Non-Goals (For This Plan)

- Implementing the OpenAI "Responses API" or legacy OpenAI completions.
  - This plan targets only the endpoints required by `openai_compatible_chat_api.md`.
- Full server-side multi-turn session management for OpenAI clients.
  - The OpenAI-compatible request already carries a `messages` array, and clients can supply context.
  - Server-side thread association is still required for persistence, but multi-turn inference context can remain client-supplied in Phase 1.
- Streaming responses.
  - Optional per `docs/openwebui_cynodeai_integration.md`, but not required by `REQ-USRGWY-0127`.

## Design Decisions to Lock Early

This section calls out decisions that impact API shape, test updates, and persistence.

### 1) Legacy Chat Endpoint (`POST /v1/chat`)

`docs/tech_specs/openai_compatible_chat_api.md` requires a single interactive chat surface.
That implies `POST /v1/chat` is not a supported long-term contract.

- **Decision**: Implement `GET /v1/models` and `POST /v1/chat/completions` as the only supported interactive chat interface.
- **Breaking change**: Remove `POST /v1/chat` (no legacy behavior is required or documented for pre-MVP).

Traces to `REQ-USRGWY-0127` and `CYNAI.USRGWY.OpenAIChatApi.SingleSurface`.

### 2) Model Identifiers for `GET /v1/models`

The spec requires OpenAI list-models format and supports an optional `model` request field.
It does not mandate specific model ids.

- **Decision**: Define at least one stable "chat surface" model id for the PM/PA chat surface.
  - Example (placeholder): `cynodeai.pm`.
  - This id is not required to equal the underlying inference engine model name.
- **Optional**: Expose additional ids later (for admin-configured chat agents) per `docs/openwebui_cynodeai_integration.md`.

Traces to `CYNAI.USRGWY.OpenAIChatApi.Endpoints` and `docs/openwebui_cynodeai_integration.md#chat-agent-backing-implementation-requirement`.

### 3) Chat Thread Association for Persistence

`docs/tech_specs/chat_threads_and_messages.md` requires stable threads and append-only messages, retrievable to authorized clients.
The OpenAI chat-completions protocol does not include a standard thread id field.

- **Decision**: Thread association is handled entirely by the orchestrator without extending the OpenAI-compatible surface area.
- **Project scope**: If a project context is present, it is provided via the OpenAI-standard `OpenAI-Project` header.
- **Thread selection rule (Phase 1)**: Use a single active thread per `(user_id, project_id)` scope.
  - If no active thread exists, create one.
  - If the most recent thread is older than an inactivity threshold (2 hours), start a new thread.
  - This supports persistence and retrieval without requiring client-supplied thread identifiers.

Traces to `REQ-USRGWY-0130` and `CYNAI.USRGWY.ChatThreadsMessages`.

## Work Breakdown

This section decomposes the implementation into concrete tasks by component.

### A) Orchestrator: User API Gateway OpenAI-Compatible Endpoints

**Scope**: `orchestrator/cmd/user-gateway/main.go` routing and new handler(s) under `orchestrator/internal/handlers/`.

- **A1 Add routes**
  - Add `GET /v1/models`.
  - Add `POST /v1/chat/completions`.
  - Ensure both routes are protected by the existing user auth middleware.
  - Keep request body size limits consistent with other endpoints.

Traces to:

- `REQ-USRGWY-0127`
- `CYNAI.USRGWY.OpenAIChatApi.Endpoints`

- **A2 Implement OpenAI list-models response**
  - Implement minimal OpenAI list-models payload (`object: "list"`, `data: [...]`).
  - Populate at least one model id that maps to the PM/PA surface default.
  - Ensure response status is 200 and JSON content type is correct.

Traces to:

- `REQ-USRGWY-0127`
- `features/e2e/chat_openai_compatible.feature` scenario "model listing is available"

- **A3 Implement OpenAI chat-completions handler**
  - Decode OpenAI-format request body and validate:
    - `messages` array is present and non-empty.
    - Each message has `role` and `content`.
    - `model` is accepted when present.
  - Produce OpenAI-format response with completion content at `choices[0].message.content`.
  - Use the existing inference path when configured (`OLLAMA_BASE_URL` / `INFERENCE_URL`).
  - Implement a fallback path that uses the existing sandbox/job mechanism when inference fails, without leaking secrets.

Traces to:

- `REQ-USRGWY-0127`
- `CYNAI.USRGWY.OpenAIChatApi.Endpoints`
- `CYNAI.USRGWY.OpenAIChatApi.ConversationModel`
- `CYNAI.USRGWY.OpenAIChatApi.TasksVsChat`

- **A4 Implement Reliability Behaviors**
  - **Poll cap**: enforce a maximum total wait duration for the handler.
  - **Retry with backoff**: on transient inference failures, retry a bounded number of times before fallback.
  - Ensure request cancellation propagates via `context.Context`.

Traces to:

- `REQ-ORCHES-0131`
- `REQ-ORCHES-0132`
- `CYNAI.USRGWY.OpenAIChatApi.Reliability`

- **A5 Timeouts: Server Configuration and Documentation**
  - Verify the user-gateway `WriteTimeout` and `ReadTimeout` defaults are suitable for 30-120s chat.
  - If defaults are too low, update defaults and ensure they remain configurable via env vars.
  - Add operator-facing docs describing the tuning requirements for chat.

Traces to:

- `REQ-USRGWY-0128`
- `CYNAI.USRGWY.OpenAIChatApi.Timeouts`

Implementation starting point:

- `orchestrator/internal/config/config.go` currently defaults `WRITE_TIMEOUT` to 30s.

### B) Orchestrator: Chat Threads and Messages Persistence

**Scope**: DB schema and `database.Store` operations to create/list/read threads and append/list messages.

- **B1 Add DB Migrations for Projects, Sessions, and Chat Tables**
  - Add a migration to create `projects` and `sessions` tables first (to support required foreign keys).
  - Add a migration aligned with `docs/tech_specs/chat_threads_and_messages.md` for `chat_threads` and `chat_messages`.
  - Include required indexes (`user_id, updated_at` and `thread_id, created_at`).
  - Add `project_id` and `session_id` columns with foreign keys (nullable).

Traces to:

- `REQ-USRGWY-0130`
- `CYNAI.USRGWY.ChatThreadsMessages.Threads`
- `CYNAI.USRGWY.ChatThreadsMessages.Messages`

- **B2 Extend Database Store Interface and Implementation**
  - Add store methods for:
    - Create thread.
    - List threads for user (pagination can be Phase 1 minimal).
    - Get thread by id (authz enforced in handler layer).
    - Append message (append-only).
    - List messages for a thread.

Traces to:

- `CYNAI.USRGWY.ChatThreadsMessages.ApiSurface`

- **B3 Add REST Endpoints for Thread and Message Retrieval**
  - Implement minimal endpoints (shape can follow existing gateway patterns).
  - Standardize on chat-scoped resources:
    - `POST /v1/chat/threads`
    - `GET /v1/chat/threads`
    - `GET /v1/chat/threads/{thread_id}`
    - `POST /v1/chat/threads/{thread_id}/messages`
    - `GET /v1/chat/threads/{thread_id}/messages`
  - Enforce that users can only access their own threads and messages.

Traces to:

- `REQ-USRGWY-0130`
- `CYNAI.USRGWY.ChatThreadsMessages.ApiSurface`

### C) Orchestrator: Integrate Persistence Into Chat-Completions Flow

**Scope**: the OpenAI chat-completions handler stores user and assistant turns.

- **C1 Thread Selection**
  - Determine `project_id` from the OpenAI-standard `OpenAI-Project` request header when present.
  - Select the active thread for `(user_id, project_id)` or create a new thread when needed.
  - Rotate to a new thread after 2 hours of inactivity so unrelated conversations do not merge indefinitely.

Traces to:

- `REQ-USRGWY-0130`
- `docs/tech_specs/chat_threads_and_messages.md`

- **C2 Message Storage**
  - Store at minimum:
    - The last user message sent in the request, after secret redaction.
    - The generated assistant response returned to the client.
  - Store metadata (jsonb) for:
    - Selected model id.
    - Client type hint (e.g. `cynork`, `openwebui`) when known.
    - Request id (if available) for correlation.

Traces to:

- `CYNAI.USRGWY.ChatThreadsMessages.Messages`
- `CYNAI.USRGWY.OpenAIChatApi.Observability`

### D) Cynork: Switch Chat to OpenAI-Compatible Gateway Surface

**Scope**: `cynork/cmd/chat.go`, `cynork/internal/gateway/client.go`, and cynork BDD mocks.

- **D1 Update Gateway Client**
  - Add `ListModels()` calling `GET /v1/models`.
  - Add `ChatCompletions()` calling `POST /v1/chat/completions` with OpenAI-format body.
  - Deprecate `Client.Chat()` and stop calling `POST /v1/chat`.

Traces to:

- `REQ-CLIENT-0161`
- `CYNAI.USRGWY.OpenAIChatApi.Endpoints`

- **D2 Update `cynork chat` Flags and Behavior**
  - Add flags per `docs/tech_specs/cli_management_app_commands_chat.md`:
    - `--model <id>`
    - `--project-id <id>`
    - `--plain` (already present)
  - Maintain a local `messages` array and send it each turn.
  - Handle `--no-color` for output formatting.

Traces to:

- `REQ-CLIENT-0161`
- `REQ-CLIENT-0162`
- `REQ-CLIENT-0171`
- `REQ-CLIENT-0173`

- **D3 Implement Slash Commands (Local Handling Plus Gateway Calls)**
  - Local-only:
    - `/exit`, `/quit`, `/help`, `/clear`, `/version`
  - Model selection:
    - `/models` (calls `GET /v1/models`)
    - `/model [<id>]` (get/set current model for subsequent requests)
  - Project context:
    - `/project [<project_id>]` (get/set local project context; sent as the OpenAI-standard `OpenAI-Project` header on subsequent `POST /v1/chat/completions` calls)
  - Gateway-backed (MUST reuse existing cynork subcommand implementations internally to prevent drift):
    - `/status`, `/whoami`
    - `/task list|get|create|cancel|result|logs|artifacts ...`
    - `/nodes list|get`
    - `/prefs list|get|set|delete|effective`
    - `/skills list|get`

Traces to:

- `REQ-CLIENT-0164` through `REQ-CLIENT-0170`
- `CYNAI.USRGWY.ChatSlashCommandSupport`

- **D4 Autocomplete and Inline Suggestions for Slash Commands**
  - Replace the `bufio.Scanner` loop with an interactive line editor that supports:
    - Showing suggestions when input begins with `/`.
    - Tab completion / cycling.
    - Up/down navigation in suggestions.
  - Ensure behavior honors `--no-color`.

Traces to:

- `REQ-CLIENT-0165`
- `CYNAI.CLIENT.CliChatSlashAutocomplete`

Note:
This likely requires introducing an interactive TUI or readline dependency in the cynork module.

### E) BDD and Feature Coverage Alignment

**Scope**: update existing feature files and step definitions so `just test-bdd` covers the OpenAI-compatible surface.

- **E1 Orchestrator BDD**
  - Replace the legacy chat scenario in `features/orchestrator/orchestrator_task_lifecycle.feature` with OpenAI-compatible scenarios.
  - Add scenarios equivalent to `features/e2e/chat_openai_compatible.feature` into the orchestrator suite path (`features/orchestrator/`) so they run in `orchestrator/_bdd`.
  - Update `orchestrator/_bdd/steps.go` to:
    - Call `GET /v1/models` and validate list-models payload.
    - Call `POST /v1/chat/completions` with OpenAI-format messages and validate `choices[0].message.content`.

Traces to:

- `REQ-USRGWY-0127`
- `REQ-USRGWY-0130`
- `CYNAI.USRGWY.OpenAIChatApi`

- **E2 Cynork BDD**
  - Update `cynork/_bdd/steps.go` mock gateway mux:
    - Add `GET /v1/models`.
    - Add `POST /v1/chat/completions`.
    - Remove `POST /v1/chat` once cynork no longer calls it.
  - Update `features/cynork/cynork_chat.feature` to assert that cynork uses the OpenAI surface.

Traces to:

- `REQ-CLIENT-0161`
- `REQ-USRGWY-0127`

## Spec Gaps and Implementation Decisions

The following gaps may impede implementation or require the implementing agent to make decisions not fully specified in the requirements or tech specs.
Resolve or document decisions before or during implementation.

### OpenAI Wire Format (Exact JSON Shapes)

- **Resolved:** Pin to the latest OpenAI Chat Completions API reference as of 2026-02-22.
- **Contract pin:** Use the OpenAI REST API version indicated by the `openai-version` header in the OpenAI API Overview (currently `2020-10-01` as of 2026-02-22).
- **Implementation rule:** Accept the OpenAI Chat Completions request/response schema as a superset.
  - CyNodeAI MUST validate and use only the fields required by `docs/tech_specs/openai_compatible_chat_api.md`.
  - Unknown fields MUST be ignored for forward compatibility.

### Chat Error Response Format

- **Resolved:** Return an OpenAI-style error payload for OpenAI-compatible endpoints.
- **Compatibility rule:** For `GET /v1/models` and `POST /v1/chat/completions`, errors MUST be returned as an OpenAI-style JSON body with a top-level `error` object.
- **No secrets:** Error messages MUST be safe for user display and MUST NOT leak detected secrets.

### Inference Path: Messages to Completion

- **Resolved:** Define a strict request pipeline for chat completions.
- **Secret redaction:** Before any storage or inference, the gateway MUST detect and redact secrets in the chat messages (API keys in particular).
- **Persistence:** The gateway MUST store the redacted (amended) user message to the database, scoped by authenticated user and optional project context.
- **Inference:** The gateway MUST pass only the redacted (amended) messages to the LLM or agent surface to produce the completion.
  Redaction MUST be indicated in the amended message text by replacing detected secrets with `SECRET_REDACTED`.

### Thread and Session Identifiers in OpenAI Request

- **Resolved:** Do not extend the OpenAI-compatible surface area with custom thread or session identifiers.
- **Thread handling:** The orchestrator selects and manages thread association server-side.
- **Project context:** If present, project context is derived from the OpenAI-standard `OpenAI-Project` request header.

### Chat Thread and Message Schema in Postgres

- **Gap:** `chat_threads_and_messages.md` defines recommended fields for `chat_threads` and `chat_messages` but these tables are not yet in `postgres_schema.md` or in orchestrator migrations.
  `chat_threads` references `project_id` and `session_id`; `sessions` and `projects` are defined in postgres_schema but may not exist in migrations yet.
- **Impact:** Implementer must add migrations for `chat_threads` and `chat_messages`; if `projects` or `sessions` do not exist, FKs must be deferred or omitted and validated in app logic.
- **Resolved:** Add migrations that create `projects` and `sessions` first, then create `chat_threads` and `chat_messages` with foreign keys.

### Data REST URL and Method Contract for Threads and Messages

- **Gap:** Chat threads spec says "Create chat thread", "List chat threads", "Get chat thread", "Append message to thread", "List messages for thread" but does not specify HTTP methods, paths, or request/response bodies.
  The plan suggests paths like `POST /v1/chat/threads` and `GET /v1/chat/threads/{thread_id}/messages` but these are not in the Data REST API or go_rest_api_standards.
- **Impact:** Implementer must define the exact REST contract (paths, methods, body shapes, status codes) and risk divergence from future spec or other clients.
- **Resolved:** Standardize on `POST/GET /v1/chat/threads` and `POST/GET /v1/chat/threads/{thread_id}/messages`, and define request/response shapes in the chat threads/messages tech spec.

### Audit Records for Chat Completions

- **Gap:** OpenAI-compatible spec says "MUST emit audit records" but does not specify the audit schema, table, or minimum fields for a chat completion (e.g. user_id, thread_id, model, timestamp, path used).
  postgres_schema.md describes domain-specific audit tables (auth, MCP, etc.) but no "chat_audit_log" or equivalent.
- **Impact:** Implementer must choose whether to reuse an existing audit mechanism (e.g. a generic user-action log) or add a new table; may be inconsistent with other domains.
- **Resolved:** Add a dedicated `chat_audit_log` table with minimal troubleshooting fields.
  This includes whether redaction was applied and the kind of secret detected.
  The audit log MUST NOT store any additional details about the redacted content.

### Default Model Identifier and List-Models Contents

- **Gap:** Spec says "If model is omitted or empty, the gateway MUST use a default model identifier" and "The default MUST correspond to the PM/PA chat surface."
  It does not specify the literal default id (e.g. `cynodeai.pm` or `default`) or how many/models to return from `GET /v1/models`.
- **Impact:** Different implementations may use different default ids; Open WebUI and cynork need a stable id to display or send.
- **Resolved:** Expose both:
  - `cynodeai.pm` as the stable PM/PA chat surface id (default).
  - underlying inference model ids limited to the currently configured inference model(s) available to the user (RBAC).

### Slash Command Parsing and Gateway Mapping

- **Gap:** CLI chat spec defines slash commands (e.g. `/task create`, `/prefs set`) and says they "MUST use the same User API Gateway endpoints" as non-interactive cynork.
  It does not specify how cynork parses arguments from the chat input line (e.g. `--limit 5` vs positional args) or how to map each slash command to a specific HTTP call when the gateway has multiple task/prefs endpoints.
- **Impact:** Implementer must design the parser and the mapping; inconsistent parsing may confuse users or break automation.
- **Resolved:** Implement slash commands by reusing the existing cynork subcommand implementations internally (shared request-building and output handling) to prevent drift.

## Validation and Quality Gates

These are the standard local checks to run while implementing.
Use repository-provided targets.

- **Markdown**
  - `just lint-md dev_docs/2026-02-22_plan_openai_compatible_chat.md`
- **BDD**
  - `just test-bdd`
- **Full local CI**
  - `just ci`

## Implementation Checklist (Ordered)

- [ ] Implement `GET /v1/models` in user-gateway and add BDD coverage.
- [ ] Implement `POST /v1/chat/completions` request/response contract and add BDD coverage.
- [ ] Add reliability behaviors (poll cap, retries/backoff) and ensure distinct error semantics.
- [ ] Add chat thread/message persistence (migration + store methods).
- [ ] Add thread/message retrieval endpoints (minimal Data REST resources for threads/messages).
- [ ] Integrate persistence into chat-completions handler (redact secrets, store amended user message, store assistant turn, server-side thread selection, project scoping via `OpenAI-Project`).
- [ ] Update cynork gateway client to call OpenAI endpoints (`/v1/models`, `/v1/chat/completions`).
- [ ] Update `cynork chat` flags and behavior to match the CLI chat spec, including slash commands.
- [ ] Implement slash autocomplete and inline suggestions for `/` input.
- [ ] Remove or disable legacy `POST /v1/chat` (after internal clients and tests are migrated).
- [ ] Run `just test-bdd`, `just docs-check`, and `just ci` to confirm everything is green.
