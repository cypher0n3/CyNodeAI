# USRGWY Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `USRGWY` domain.
It covers user-facing REST API gateway behavior and related API contracts.

## 2 Requirements

- **REQ-USRGWY-0001:** User API Gateway as single user-facing API surface; authn, authz, audit.
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  <a id="req-usrgwy-0001"></a>
- **REQ-USRGWY-0002:** Runs and sessions with stable ids; runs per task/job; sessions and sub-sessions; logs and transcripts; retention policies.
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  [CYNAI.USRGWY.Sessions](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-sessions)
  <a id="req-usrgwy-0002"></a>
- **REQ-USRGWY-0100:** The orchestrator MUST assign a unique run identifier to each run and persist it in PostgreSQL.
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  <a id="req-usrgwy-0100"></a>
- **REQ-USRGWY-0101:** A run MUST be associated with a task (and optionally a job) for auditing and lineage.
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  <a id="req-usrgwy-0101"></a>
- **REQ-USRGWY-0102:** A run MAY have a parent run identifier to support sub-runs (e.g. a step or sub-agent spawn).
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  <a id="req-usrgwy-0102"></a>
- **REQ-USRGWY-0103:** The Data REST API MUST expose runs as a core resource: create, read, list, and filter by task, job, session, parent run, and time range.
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  <a id="req-usrgwy-0103"></a>
- **REQ-USRGWY-0104:** The orchestrator MUST support creating and listing sessions.
  [CYNAI.USRGWY.Sessions](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-sessions)
  <a id="req-usrgwy-0104"></a>
- **REQ-USRGWY-0105:** A session MAY have a parent session (sub-session) for delegation or nested context.
  [CYNAI.USRGWY.Sessions](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-sessions)
  <a id="req-usrgwy-0105"></a>
- **REQ-USRGWY-0106:** Runs MAY be associated with a session via `session_id`.
  [CYNAI.USRGWY.Sessions](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-sessions)
  <a id="req-usrgwy-0106"></a>
- **REQ-USRGWY-0107:** The User API Gateway MUST allow creating a session, spawning sub-sessions, listing runs for a session, and attaching new work to a session.
  [CYNAI.USRGWY.Sessions](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-sessions)
  <a id="req-usrgwy-0107"></a>
- **REQ-USRGWY-0108:** The system MUST support attaching logs to a run (e.g. stdout, stderr, or structured events).
  [CYNAI.USRGWY.LogsTranscripts](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-logstrans)
  <a id="req-usrgwy-0108"></a>
- **REQ-USRGWY-0109:** Logs MUST be stored in a way that supports retrieval by run and time range and MUST be subject to retention policies.
  [CYNAI.USRGWY.LogsTranscripts](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-logstrans)
  <a id="req-usrgwy-0109"></a>
- **REQ-USRGWY-0110:** Transcripts (e.g. chat history, agent turn summaries) MUST be storable per session or run with a configurable retention policy.
  [CYNAI.USRGWY.LogsTranscripts](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-logstrans)
  <a id="req-usrgwy-0110"></a>
- **REQ-USRGWY-0111:** The Data REST API MUST expose endpoints to append and read logs for a run and to read/write transcript segments for a session or run.
  [CYNAI.USRGWY.LogsTranscripts](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-logstrans)
  <a id="req-usrgwy-0111"></a>
- **REQ-USRGWY-0112:** Run status changes MUST be observable via the Data REST API (polling) and SHOULD be emitted as events for User API Gateway live updates.
  [CYNAI.USRGWY.StreamingStatus](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-streamstatus)
  <a id="req-usrgwy-0112"></a>
- **REQ-USRGWY-0113:** The gateway SHOULD support streaming run status and log tail for a run when the client requests it.
  [CYNAI.USRGWY.StreamingStatus](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-streamstatus)
  <a id="req-usrgwy-0113"></a>
- **REQ-USRGWY-0114:** The orchestrator MUST support starting, listing, and terminating background processes within a sandbox, subject to sandbox constraints.
  [CYNAI.USRGWY.BackgroundProcessManagement](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-bgprocess)
  <a id="req-usrgwy-0114"></a>
- **REQ-USRGWY-0115:** Background process operations MUST be associated with a run and task for auditing.
  [CYNAI.USRGWY.BackgroundProcessManagement](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-bgprocess)
  <a id="req-usrgwy-0115"></a>
- **REQ-USRGWY-0116:** Process lifecycle MUST be exposed so clients can attach output to runs and show status in the runs/sessions API.
  [CYNAI.USRGWY.BackgroundProcessManagement](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-bgprocess)
  <a id="req-usrgwy-0116"></a>
- **REQ-USRGWY-0117:** Background process management MAY be exposed via MCP sandbox tools and MUST be reflected in the runs API when processes are tied to a run.
  [CYNAI.USRGWY.BackgroundProcessManagement](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-bgprocess)
  <a id="req-usrgwy-0117"></a>
- **REQ-USRGWY-0118:** The system MUST support configurable retention policies for run logs and session transcripts.
  [CYNAI.USRGWY.RetentionPolicies](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-retention)
  <a id="req-usrgwy-0118"></a>
- **REQ-USRGWY-0119:** Retention policy SHOULD be defined at the orchestrator or project level and applied consistently.
  [CYNAI.USRGWY.RetentionPolicies](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-retention)
  <a id="req-usrgwy-0119"></a>
- **REQ-USRGWY-0120:** Expired data MUST be purged or archived without breaking referential integrity for audit records referencing run/session identifiers.
  [CYNAI.USRGWY.RetentionPolicies](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-retention)
  <a id="req-usrgwy-0120"></a>

- **REQ-USRGWY-0121:** Compatibility layers MUST preserve orchestrator policy constraints and MUST NOT bypass auditing.
  [CYNAI.USRGWY.ClientCompatibility](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-clientcompatibility)
  <a id="req-usrgwy-0121"></a>
- **REQ-USRGWY-0122:** The User API Gateway MUST provide a Data REST API for user clients and integrations.
  [CYNAI.USRGWY.DataRestApi](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-datarestapi)
  <a id="req-usrgwy-0122"></a>
- **REQ-USRGWY-0123:** Secrets required for delivery MUST be stored securely in PostgreSQL and MUST NOT be exposed to agents.
  [CYNAI.USRGWY.MessagingAndEvents](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-messagingevents)
  <a id="req-usrgwy-0123"></a>
- **REQ-USRGWY-0124:** The gateway MUST authenticate user clients.
  [CYNAI.USRGWY.AuthAuditing](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-authauditing)
  <a id="req-usrgwy-0124"></a>
- **REQ-USRGWY-0125:** The gateway MUST authorize user actions using policy and (when applicable) user task-execution preferences and constraints.
  [CYNAI.USRGWY.AuthAuditing](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-authauditing)
  <a id="req-usrgwy-0125"></a>
- **REQ-USRGWY-0126:** The web console MUST be a client of the gateway and MUST NOT connect directly to PostgreSQL.
  [CYNAI.USRGWY.WebConsole](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-webconsole)
  <a id="req-usrgwy-0126"></a>
- **REQ-USRGWY-0127:** The User API Gateway MUST expose the OpenAI-compatible chat API as the only interactive chat interface.
  This includes `GET /v1/models`, `POST /v1/chat/completions`, and `POST /v1/responses`.
  The OpenAI-compatible contract MUST support both the OpenAI Chat Completions and OpenAI Responses surfaces as defined by the corresponding tech spec.
  The implementation MUST follow the OpenAI REST API version indicated by `openai-version: 2020-10-01` (as of 2026-02-22).
  The gateway MUST ignore unknown request fields for forward compatibility.
  `GET /v1/models` MUST return only model identifiers that the authenticated user is authorized to use (RBAC/policy).
  [CYNAI.USRGWY.OpenAIChatApi](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi)
  [CYNAI.USRGWY.ClientCompatibility](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-clientcompatibility)
  <a id="req-usrgwy-0127"></a>
- **REQ-USRGWY-0128:** The gateway MUST support configurable timeouts suitable for OpenAI-compatible chat completions.
  Deployments that use chat MUST be able to configure timeouts so chat can complete when models are cold or under load.
  [CYNAI.USRGWY.OpenAIChatApi.Timeouts](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-timeouts)
  <a id="req-usrgwy-0128"></a>
- **REQ-USRGWY-0129:** The gateway MUST provide distinct chat-completion error semantics and MUST log completion path selection for diagnostics.
  Errors MUST distinguish cancellation, inference failure, and completion timeout.
  For the OpenAI-compatible endpoints (`GET /v1/models`, `POST /v1/chat/completions`, and `POST /v1/responses`), the gateway MUST return an OpenAI-style JSON error payload with a top-level `error` object.
  [CYNAI.USRGWY.OpenAIChatApi.Errors](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-errors)
  [CYNAI.USRGWY.OpenAIChatApi.Observability](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-observability)
  <a id="req-usrgwy-0129"></a>
- **REQ-USRGWY-0130:** The system MUST store chat history as chat threads and chat messages that are tracked separately from task lifecycle state.
  Chat threads and messages MUST be retrievable to authorized clients.
  Transcript segments remain a separate derived artifact associated with a session or run.
  [CYNAI.USRGWY.ChatThreadsMessages](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages)
  [CYNAI.USRGWY.OpenAIChatApi.ConversationModel](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-conversationmodel)
  <a id="req-usrgwy-0130"></a>
- **REQ-USRGWY-0131:** Tasks MUST support association to the creating user (e.g. `created_by` from the authenticated request context) and to a project via `project_id`.
  When a task is created via the gateway by an authenticated user, the gateway MUST set the task creator from the request context.
  When created by the system (including PM/PA via MCP and bootstrap flows), the system MUST set creator per policy but MUST NOT write a null `created_by` (use the reserved system user identity when no human creator applies; see [REQ-IDENTY-0121](../requirements/identy.md#req-identy-0121)).
  When no project is explicitly set (by the user client or by the PM/PA), the gateway MUST associate the task or chat thread with the creating user's default project (authenticated user when present, otherwise system user) (see [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)).
  User clients MAY supply an explicit project via the OpenAI-standard `OpenAI-Project` request header for chat; when absent, the thread and any tasks created in that context use the creating user's default project.
  [REQ-PROJCT-0001](../requirements/projct.md#req-projct-0001)
  [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)
  [CYNAI.ACCESS.Doc.ProjectsAndScopes](../tech_specs/projects_and_scopes.md#spec-cynai-access-doc-projectsandscopes)
  [CYNAI.USRGWY.ChatThreadsMessages.Threads](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-threads)
  <a id="req-usrgwy-0131"></a>

- **REQ-USRGWY-0132:** The User API Gateway MUST redact detected secrets from OpenAI-compatible chat messages before persisting or using them for inference.
  The gateway MUST persist only the amended (redacted) content in chat threads and chat messages.
  For streaming assistant output, redaction MAY be performed in parallel in the orchestrator: the gateway MAY stream token-by-token to the client and apply post-facto redaction (e.g. persist redacted content and send updated NDJSON that removes secrets from the current chat) so that streaming is not blocked on redaction.
  [CYNAI.USRGWY.OpenAIChatApi.Pipeline](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-pipeline)
  [CYNAI.USRGWY.ChatThreadsMessages.Messages](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-messages)
  <a id="req-usrgwy-0132"></a>
- **REQ-USRGWY-0133:** Every user-facing request MUST be authenticated and authorized.
  No endpoint may rely on network location or a previous request for identity; each request MUST be verified independently.
  [CYNAI.USRGWY.AuthAuditing](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-authauditing)
  <a id="req-usrgwy-0133"></a>
- **REQ-USRGWY-0134:** The User API Gateway MAY expose an optional chat model warm-up endpoint (e.g. `POST /v1/chat/warm`).
  When exposed, warm-up MUST be best-effort: failure or timeout MUST NOT block or break chat sessions.
  The endpoint MUST require the same authentication as the chat API.
  [CYNAI.USRGWY.OpenAIChatApi.WarmUp](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-warmup)
  <a id="req-usrgwy-0134"></a>
- **REQ-USRGWY-0135:** The User API Gateway MUST support explicit creation of a new chat thread via `POST /v1/chat/threads`.
  The created thread MUST be scoped to the authenticated user and to the requested or default project.
  The response MUST return the created thread identifier for retrieval and management purposes, but the gateway MUST NOT require clients to send any CyNodeAI-specific thread identifier on subsequent OpenAI-compatible chat completion requests.
  [CYNAI.USRGWY.ChatThreadsMessages.ApiSurface](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-apisurface)
  <a id="req-usrgwy-0135"></a>
- **REQ-USRGWY-0136:** The User API Gateway MUST provide a structured chat-turn representation for rich clients while preserving a coherent plain-text fallback for simple clients.
  The structured representation MUST allow clients to distinguish at least visible assistant text, hidden thinking or reasoning content, tool activity, and file or download references without scraping prose.
  [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
  [CYNAI.USRGWY.OpenAIChatApi.NormalizedAssistantOutput](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-normalizedassistantoutput)
  <a id="req-usrgwy-0136"></a>
- **REQ-USRGWY-0137:** When one interactive chat request yields multiple assistant-side output items, the gateway MUST preserve their order within one logical assistant turn.
  This applies whether the upstream behavior comes from provider-native structured output, tool-call loops, or responses-style multi-item output.
  [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
  [CYNAI.USRGWY.OpenAIChatApi.NormalizedAssistantOutput](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-normalizedassistantoutput)
  <a id="req-usrgwy-0137"></a>
- **REQ-USRGWY-0138:** Thinking or reasoning content MUST NOT be included in the canonical plain-text transcript content used for thread title, summary, list preview, or other default user-visible transcript surfaces.
  If the system preserves thinking at all, it MUST do so only as non-default structured data or bounded metadata, and rich clients MUST treat it as hidden-by-default display content.
  [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
  [CYNAI.USRGWY.OpenAIChatApi.NormalizedAssistantOutput](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-normalizedassistantoutput)
  <a id="req-usrgwy-0138"></a>
- **REQ-USRGWY-0139:** The User API Gateway MUST support structured chat-turn data so rich clients can distinguish visible assistant text, tool activity, downloadable outputs, and hidden thinking without scraping prose.
  Internal reasoning or thinking content MUST NOT be used as canonical transcript content for thread title, summary, or list preview generation.
  [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
  [CYNAI.USRGWY.OpenAIChatApi.NormalizedAssistantOutput](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-normalizedassistantoutput)
  <a id="req-usrgwy-0139"></a>
- **REQ-USRGWY-0140:** The OpenAI-compatible chat surface MUST define user message input as text with Markdown syntax support.
  File inclusion for user messages is allowed only through the documented `@`-reference workflow, and the gateway MUST accept the resulting uploaded or inline file references according to the chat contract.
  When a chat thread is project-scoped, any uploaded file accepted as part of that user message MUST inherit the same project-scoped authorization as the originating thread and message.
  [CYNAI.USRGWY.OpenAIChatApi.TextInput](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-textinput)
  <a id="req-usrgwy-0140"></a>
- **REQ-USRGWY-0141:** When a completion or tool execution produces a downloadable output, the gateway SHOULD expose a stable authenticated download reference and metadata suitable for explicit client download UX.
  Clients MUST NOT be forced to scrape assistant prose to discover downloadable files.
  [CYNAI.USRGWY.ChatThreadsMessages.DownloadRefs](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-downloadrefs)
  <a id="req-usrgwy-0141"></a>
- **REQ-USRGWY-0142:** The Data REST API for chat threads MUST support updating a thread's user-facing title.
  Title updates MUST be authorized against the authenticated thread owner and MUST NOT require creating a new thread.
  [CYNAI.USRGWY.ChatThreadsMessages.ThreadTitle](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-threadtitle)
  <a id="req-usrgwy-0142"></a>
- **REQ-USRGWY-0143:** The system MAY store an optional short summary for a chat thread for list or sidebar display.
  If a summary is derived from message content, it MUST use redacted content only and MUST NOT store plaintext secrets.
  [CYNAI.USRGWY.ChatThreadsMessages.ThreadSummary](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-threadsummary)
  <a id="req-usrgwy-0143"></a>
- **REQ-USRGWY-0144:** List chat-thread endpoints MUST support recent-activity ordering and pagination so clients can implement chat-history lists of arbitrary size.
  Default ordering MUST be newest first by `updated_at`.
  [CYNAI.USRGWY.ChatThreadsMessages.HistoryList](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-historylist)
  <a id="req-usrgwy-0144"></a>
- **REQ-USRGWY-0145:** The gateway MAY support archive or soft-delete state for chat threads so users can hide threads from the default history list without losing retained data.
  If archive state is supported, list endpoints MUST allow filtering by active versus archived visibility.
  [CYNAI.USRGWY.ChatThreadsMessages.Archive](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-archive)
  <a id="req-usrgwy-0145"></a>
- **REQ-USRGWY-0146:** The gateway MUST track the effective context size used to build the next chat completion request for a thread.
  Context-size tracking MUST be model-aware and deterministic for a given thread state, accepted file context, and model selection.
  [CYNAI.USRGWY.ChatThreadsMessages.ContextSizeTracking](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-contextsizetracking)
  <a id="req-usrgwy-0146"></a>
- **REQ-USRGWY-0147:** When the effective context size for the next chat completion reaches at least 95 percent of the selected model's effective context window, the gateway MUST compact older conversation context before issuing the next completion request.
  Context compaction MUST preserve enough recent unsummarized context for the next user turn and expected assistant response, and MUST preserve conversation continuity in a deterministic and reviewable way.
  [CYNAI.USRGWY.OpenAIChatApi.ContextCompaction](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-contextcompaction)
  <a id="req-usrgwy-0147"></a>
- **REQ-USRGWY-0148:** When the gateway retains hidden thinking or reasoning content for an assistant turn, it MUST retain that content as structured turn data associated with the same logical assistant turn so authorized rich clients can retrieve it later through thread-history reads.
  Canonical plain-text transcript fields and projections MUST remain visible-text-only.
  [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
  [CYNAI.USRGWY.OpenAIChatApi.NormalizedAssistantOutput](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-normalizedassistantoutput)
  <a id="req-usrgwy-0148"></a>
- **REQ-USRGWY-0149:** The OpenAI-compatible interactive chat surface MUST support streaming chat responses for `POST /v1/chat/completions` and `POST /v1/responses` when the client requests `stream=true`.
  The gateway MUST emit real token-by-token (incremental) visible assistant text on the standard streaming path, MUST NOT buffer all visible assistant text until completion, and MUST finish with a clear terminal completion or error event.
  [CYNAI.USRGWY.OpenAIChatApi.Streaming](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streaming)
  [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
  <a id="req-usrgwy-0149"></a>
- **REQ-USRGWY-0150:** When a client cancels or abandons an OpenAI-compatible streaming chat request, such as closing the connection or interrupting the client with Ctrl+C, the gateway MUST treat that stream as canceled.
  The gateway MUST stop or detach upstream generation work on a best-effort basis promptly enough to avoid unnecessary continued token generation, and it MUST release request-scoped resources even when the final assistant turn is incomplete.
  If the stream remains connected long enough to emit a final transport signal, the gateway MUST end it with a clear cancellation event or cancellation-class error rather than hanging indefinitely.
  [CYNAI.USRGWY.OpenAIChatApi.Streaming](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streaming)
  [CYNAI.USRGWY.OpenAIChatApi.Errors](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-errors)
  <a id="req-usrgwy-0150"></a>
- **REQ-USRGWY-0151:** When secret redaction detects secrets in streaming assistant output, the gateway MUST emit a post-stream amendment SSE event before the terminal `[DONE]` event.
  The amendment event MUST carry the full redacted assistant content so the client can replace the accumulated visible text for the in-flight turn.
  The gateway MUST persist only the redacted content and MUST record redaction metadata in the chat audit log.
  When no secrets are detected, no amendment event is emitted and the accumulated streamed text is persisted as-is.
  [CYNAI.USRGWY.OpenAIChatApi.StreamingRedactionPipeline](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streamingredactionpipeline)
  <a id="req-usrgwy-0151"></a>
