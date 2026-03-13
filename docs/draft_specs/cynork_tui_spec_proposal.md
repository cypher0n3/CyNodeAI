# Unified Spec Proposal: Chat QOL and Cynork Chat TUI

## 1 Summary

- **Date:** 2026-03-06
- **Purpose:** Single cohesive draft merging (1) chat quality-of-life (history, naming, summaries, archive), (2) cynork chat as the primary TUI with shell deprecation, and (3) richer modern chat conventions such as hidden thinking, tool activity, and downloadable outputs (assistant-provided).
  User message input is **text with Markdown syntax**.
  File inclusion is supported only via an **`@` shorthand** (e.g. `@path` or `@filename`): the client resolves references from the filesystem and auto-uploads the referenced files when the message is submitted (Cursor-like).
- **Status:** Draft; partially integrated via a TUI-first MVP (see [Integration Plan and Refinements](#21-integration-plan-and-refinements)).
  Full proposal not yet merged; stable requirements and tech specs reflect deliberate refinements and repurposed REQ IDs.
- **Supersedes:** Drafts dated 2026-03-02 (chat QOL) and 2026-03-05 (cynork chat TUI upgrade recommendations).

Baseline references:

- [Chat Threads and Messages](../tech_specs/chat_threads_and_messages.md) (CYNAI.USRGWY.ChatThreadsMessages), [REQ-USRGWY-0130](../requirements/usrgwy.md#req-usrgwy-0130)
- [cli_management_app_commands_chat.md](../tech_specs/cli_management_app_commands_chat.md), [cli_management_app_shell_output.md](../tech_specs/cli_management_app_shell_output.md), [REQ-CLIENT-0161](../requirements/client.md) onward
- [OpenAI-Compatible Chat API](../tech_specs/openai_compatible_chat_api.md)

This document proposes requirements and spec extensions so that:

- Clients (Web Console and cynork) offer a better chat UX: visible history, thread names, optional summaries, and list behavior.
- `cynork chat` becomes the single interactive TUI (cursor-agent-like), with shell REPL deprecated or removed.
- Richer LLM interaction conventions are handled explicitly instead of being left implicit in plain text, especially thinking blocks, tool-call activity, and file downloads (assistant-provided).
  User input is text with Markdown; file inclusion is only via `@` shorthand with auto-upload on submit.

## 2 Scope

- **Gateway and data:** Thread title updates, optional thread summary, list sort/pagination/archive, and related API behavior.
- **Gateway and data:** Structured turn metadata so tool activity and download refs do not have to be scraped from prose.
- **OpenAI-compatible chat surface:** User message input is text (Markdown syntax supported).
  File inclusion for user messages is only via **`@` shorthand**: filesystem lookup (e.g. `@path`, `@filename`) and auto-upload of referenced files when the message is submitted (Cursor-like).
  Assistant-generated download refs are in scope.
- **Backend (orchestrator, database, PMA):** Draft requirements and specs for file upload storage, retrieval, and passthrough to PMA/LLM so that `@`-referenced files are available in the completion path (see section 3.4 and 4.8).
- **Client (all chat UIs):** History list, rename, summary display.
- **Cynork-specific:** Chat as the only interactive surface; TUI layout (composer, panes, completion, status bar); **conversation history as formatted Markdown** (same as current `cynork chat`); **`! command`** shell escape maintained; **`@` file shorthand** (filesystem lookup, auto-upload on submit); rendering for thinking, tool calls, downloads (assistant-provided); deprecation/removal of `cynork shell`; local config and cache.

### 2.1 Integration Plan and Refinements

Execution followed the **TUI-first** plan in [2026-03-12_plan_next_round_execution.md](../dev_docs/2026-03-12_plan_next_round_execution.md).
The plan cut this proposal down to a minimal first rollout; the refinements below differ from the original draft.

#### 2.1.1 Promoted to Stable (First Rollout)

- **Entry point:** `cynork tui` is the explicit first full-screen TUI entrypoint; `cynork chat` remains available as a compatibility path.
  Bare `cynork` (no subcommand) does **not** launch the TUI in the first rollout (deferred).
- **Thread UX:** Create (explicit fresh-thread), list, switch, rename.
  Thread summary and archive are **deferred**.
- **Gateway/data:** Explicit thread creation (`POST /v1/chat/threads`), list threads with pagination, get thread, patch thread title.
  Structured turns (thinking hidden, ordered assistant output, tool activity) and dual chat surface (`POST /v1/chat/completions` and `POST /v1/responses`) are in scope.
- **Stable requirements:** REQ-USRGWY-0135 (explicit thread creation), 0136--0138 (structured turns, ordering, thinking not in canonical transcript).
  REQ-CLIENT-0181 (explicit fresh-thread creation), 0182--0186 (structured-turn preference, hide thinking, multi-item turn, in-flight update, plain output).
  This draft uses **REQ-USRGWY-0142--0145** and **REQ-CLIENT-0199--0204** for the corresponding proposal items to avoid ID conflicts with stable.
- **Stable specs:** [cynork_tui.md](../tech_specs/cynork_tui.md), [cynork_cli.md](../tech_specs/cynork_cli.md) TUI scope, [chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md) StructuredTurns and API surface, [cli_management_app_commands_chat.md](../tech_specs/cli_management_app_commands_chat.md) thread controls.

#### 2.1.2 Explicitly Deferred From First Rollout (Plan Phase 2)

- Thread summary and archive; download-ref workflows (attachment upload for user messages is out of scope); queued drafts; making bare `cynork` launch the TUI; web/SSO login.
- Proposal REQ-CLIENT-0187--0198 and USRGWY 0139--0141, 0142--0145 (as written here) are not yet in stable requirements.
  Behavior for auth recovery, layout, and transcript rendering is specified in [cynork_tui.md](../tech_specs/cynork_tui.md) in prose.

#### 2.1.3 REQ ID Mapping (Draft vs Stable)

- This draft uses **non-overlapping** proposal IDs to avoid conflicts with stable:
  - **USRGWY:** Stable 0135--0138 (thread creation, structured turns, order, thinking).
    This draft uses **0142--0145** for thread title, thread summary, history list, archive.
  - **CLIENT:** Stable 0181--0186 (fresh-thread, structured preference, hide thinking, multi-item turn, in-flight update, plain output).
    This draft uses **0199--0204** for view history, rename thread, summary display, single UI, TUI experience, slash/shell parity.
- Proposal 0187--0198 and 0139--0141 are used only in this draft; when promoted, they will be assigned stable IDs.
- When comparing this draft to stable [usrgwy.md](../requirements/usrgwy.md) and [client.md](../requirements/client.md), use the stable docs as authoritative for the current contract.

## 3 Proposed Requirements

The following requirement IDs are **proposed** and would live in the indicated requirements file if accepted.
Each entry uses the canonical format: requirement line, spec reference link(s) to the proposed Spec Item in this document, then requirement anchor.

### 3.1 Gateway and Data (USRGWY)

- **REQ-USRGWY-0142 (proposed):** The Data REST API for chat threads MUST support updating a thread's user-facing title.
  Clients MUST be able to set and change the display name of a thread without creating a new thread.
  The gateway MUST derive `user_id` from authentication and MUST allow updates only for threads owned by that user.
  [CYNAI.USRGWY.ChatThreadsMessages.ThreadTitle](#spec-cynai-usrgwy-chatthreadsmessages-threadtitle)
  <a id="req-usrgwy-0142"></a>

- **REQ-USRGWY-0143 (proposed):** The system MAY store an optional short summary for a chat thread (e.g. for list/sidebar display).
  If supported, the summary MUST be derived or set in a way that does not require storing plaintext secrets; any summary derived from message content MUST use redacted content only.
  Summary generation MAY be best-effort or asynchronous.
  [CYNAI.USRGWY.ChatThreadsMessages.ThreadSummary](#spec-cynai-usrgwy-chatthreadsmessages-threadsummary)
  <a id="req-usrgwy-0143"></a>

- **REQ-USRGWY-0144 (proposed):** List chat threads endpoints MUST support sort order by `updated_at` (default: descending) and MUST support pagination so clients can implement "chat history" lists of arbitrary size.
  [CYNAI.USRGWY.ChatThreadsMessages.HistoryList](#spec-cynai-usrgwy-chatthreadsmessages-historylist)
  <a id="req-usrgwy-0144"></a>

- **REQ-USRGWY-0145 (proposed):** The gateway MAY support soft-delete or archive state for chat threads so that users can hide threads from the default history list without losing data.
  If supported, list endpoints MUST allow filtering by visibility (e.g. active vs archived) and retention MUST still apply per existing policy.
  [CYNAI.USRGWY.ChatThreadsMessages.Archive](#spec-cynai-usrgwy-chatthreadsmessages-archive)
  <a id="req-usrgwy-0145"></a>

- **REQ-USRGWY-0139 (proposed):** The gateway MUST support structured chat-turn data so clients can distinguish visible assistant text, tool activity, and downloadable outputs without parsing prose.
  Internal reasoning or thinking content MUST NOT be exposed as normal transcript content and MUST NOT be used as input for thread title or summary generation.
  [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
  <a id="req-usrgwy-0139"></a>

- **REQ-USRGWY-0140 (proposed):** The OpenAI-compatible chat surface MUST define user message content as text (plain string; Markdown syntax is supported for formatting).
  File inclusion for user messages is only via the **@ shorthand** (see section 4.6.1); when the client sends a message that contains @-resolved file references, the gateway MUST accept the upload (or inline representation per spec) and associate the files with the message.
  [CYNAI.USRGWY.OpenAIChatApi.TextInput](#spec-cynai-usrgwy-openaichatapi-textinput)
  <a id="req-usrgwy-0140"></a>

- **REQ-USRGWY-0141 (proposed):** When chat completions or tool executions produce downloadable outputs, the gateway SHOULD expose stable authenticated references and metadata suitable for an explicit client download UX.
  Clients MUST NOT be forced to scrape assistant prose to discover downloadable files.
  [CYNAI.USRGWY.ChatThreadsMessages.DownloadRefs](#spec-cynai-usrgwy-chatthreadsmessages-downloadrefs)
  <a id="req-usrgwy-0141"></a>

**Integration note:** In the TUI-first rollout, stable REQ-USRGWY-0135 is **explicit thread creation** (`POST /v1/chat/threads`), not thread title.
  Thread title update (PATCH) is implemented and specified in the API surface; stable 0136--0138 cover structured turns, ordered assistant turn, and thinking excluded from canonical transcript.
  This draft's 0142 (thread title), 0143 (summary), 0145 (archive), and 0140/0141 (text input, download refs) remain proposed or deferred as noted.

### 3.2 Client (Chat UX and History)

- **REQ-CLIENT-0199 (proposed):** Clients that provide a chat UI (e.g. Web Console, cynork chat) MUST expose a way for the user to view chat history (list of threads for the current user and project context).
  The list MUST show thread title (or a fallback such as first message preview or "Untitled") and SHOULD show last activity time.
  [CYNAI.USRGWY.ChatThreadsMessages.HistoryList](#spec-cynai-usrgwy-chatthreadsmessages-historylist)
  <a id="req-client-0199"></a>

- **REQ-CLIENT-0200 (proposed):** Clients that provide a chat UI MUST allow the user to rename the current thread (set or update title) and SHOULD allow renaming from the thread list.
  [CYNAI.USRGWY.ChatThreadsMessages.ThreadTitle](#spec-cynai-usrgwy-chatthreadsmessages-threadtitle)
  <a id="req-client-0200"></a>

- **REQ-CLIENT-0201 (proposed):** When the gateway provides a thread summary, clients SHOULD display it in the thread list or sidebar (e.g. tooltip or subtitle) to help users identify conversations without opening them.
  [CYNAI.USRGWY.ChatThreadsMessages.ThreadSummary](#spec-cynai-usrgwy-chatthreadsmessages-threadsummary)
  <a id="req-client-0201"></a>

**Integration note:** In stable docs, REQ-CLIENT-0181 is **explicit fresh-thread creation** (not "view chat history").
  Stable 0182--0186 cover structured-turn preference, hide thinking by default, multi-item assistant turn, in-flight update, and plain/one-shot output only.
  Thread list and rename are in scope.
    Summary display (0201 here) is deferred.

### 3.3 Client (Cynork Chat as Primary TUI)

- **REQ-CLIENT-0202 (proposed):** The cynork CLI MUST provide a single interactive UI surface for chat.
  The interactive REPL mode (`cynork shell`) SHALL be deprecated or removed in favor of `cynork chat` as the primary TUI.
  [CYNAI.CLIENT.CynorkChat.TUILayout](#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0202"></a>
  **Refinement:** First rollout exposes the full-screen TUI as `cynork tui`; `cynork chat` remains a compatibility path.
    Shell is deprecated, not removed.

- **REQ-CLIENT-0203 (proposed):** The cynork chat TUI SHOULD support a cursor-agent-like experience: multi-line input composer, scrollback with search and copy, persistent status bar (gateway, identity, project, model), and an optional context pane (project, tasks, slash help).
  The scrollback MUST distinguish user messages with a distinct background; MUST support Shift+Enter for newlines in the composer; Up/Down in the composer MUST cycle through previously sent messages; Ctrl+C MUST cancel the current operation (and successive Ctrl+C when idle MUST exit cleanly); and scrolling (e.g. Page Up/Page Down) MUST allow loading older history when the user scrolls back past loaded content.
  Completion and fuzzy selection SHOULD be available for task identifiers, project selection, and model selection within chat.
  [CYNAI.CLIENT.CynorkChat.TUILayout](#spec-cynai-client-cynorkchat-tuilayout)
  [CYNAI.CLIENT.CynorkChat.Completion](#spec-cynai-client-cynorkchat-completion)
  <a id="req-client-0203"></a>

- **REQ-CLIENT-0204 (proposed):** Slash commands in cynork chat MUST provide parity with the command surface previously available in the shell REPL (tasks, status, whoami, nodes, prefs, skills, model, project).
  The **`! command` shorthand** (shell escape) MUST be supported: input starting with `!` runs the remainder of the line as a shell command.
  It SHOULD be enabled by default; the implementation MAY allow disabling it (e.g. `--enable-shell`) and MUST document the behavior in the cynork chat tech spec.
  [CYNAI.CLIENT.CynorkChat.TUILayout](#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0204"></a>

- **REQ-CLIENT-0187 (proposed):** The cynork chat TUI MAY persist local configuration for TUI preferences (e.g. default model, composer single vs multi-line, context pane default visibility, keybinding overrides).
  If supported, config MUST use the same config file or config directory as the rest of cynork (see [CliConfigFileLocation](../tech_specs/cynork_cli.md#spec-cynai-client-cliconfigfilelocation)); config MUST NOT store secrets (tokens, passwords, or message content).
  [CYNAI.CLIENT.CynorkChat.LocalConfig](#spec-cynai-client-cynorkchat-localconfig)
  <a id="req-client-0187"></a>

- **REQ-CLIENT-0188 (proposed):** The cynork chat TUI MAY use a local cache for completion and list data (e.g. task ids, project ids, model ids, thread list metadata) to improve responsiveness of Tab completion and context pane.
  If supported, cache MUST be stored under a documented cache directory (e.g. XDG_CACHE_HOME); cache MUST NOT contain secrets (no tokens, no message content); cache SHOULD have a TTL or invalidation rule so stale data is refreshed.
  [CYNAI.CLIENT.CynorkChat.LocalCache](#spec-cynai-client-cynorkchat-localcache)
  <a id="req-client-0188"></a>

- **REQ-CLIENT-0189 (proposed):** When the user invokes a command via the shell escape `!`, the CLI MUST be interactive-subprocess safe: the TUI MUST suspend and give the subprocess the real TTY; when the subprocess exits, the CLI MUST restore the TUI and continue the session.
  [CYNAI.CLIENT.CynorkChat.TUILayout](#spec-cynai-client-cynorkchat-tuilayout) (Shell Escape and Interactive Subprocesses)
  <a id="req-client-0189"></a>

- **REQ-CLIENT-0190 (proposed):** When `cynork chat` is invoked without a usable login token (missing token, expired token, or gateway returns an authentication error), the chat TUI MUST offer an in-session login path rather than requiring the user to exit and re-run a command.
  The CLI MUST show a small login box (modal or overlay) with inputs for gateway URL, username, and password; gateway URL and username MUST be prepopulated when known (e.g. from config or last successful login); password MUST NOT be prepopulated and MUST be entered with secret input (no echo).
  After successful login, the CLI MUST resume the chat session.
  [CYNAI.CLIENT.CynorkChat.AuthRecovery](#spec-cynai-client-cynorkchat-authrecovery)
  <a id="req-client-0190"></a>

- **REQ-CLIENT-0191 (proposed):** The CLI SHOULD support a web-based login flow suitable for SSO (for example device-code style login or browser-based authorization) in addition to username/password login.
  This flow MUST avoid printing or persisting secrets to shell history or logs and MUST integrate with the existing token storage and credential-helper model.
  [CYNAI.CLIENT.CliWebLogin](#spec-cynai-client-cliweblogin)
  <a id="req-client-0191"></a>

- **REQ-CLIENT-0192 (proposed):** Clients with a chat UI MUST NOT render model reasoning or thinking blocks as normal assistant transcript content.
  While a response is in progress, the UI MAY show ephemeral status such as "Thinking..." or similar progress text, but that status MUST NOT be persisted as message content.
  [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
  [CYNAI.CLIENT.CynorkChat.TUILayout](#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0192"></a>

- **REQ-CLIENT-0193 (proposed):** Clients with a rich chat UI SHOULD render tool calls and tool results as structured transcript items distinct from assistant prose.
  Tool argument and result previews SHOULD be redacted and truncated for readability, and verbose payloads SHOULD be collapsed by default.
  [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
  [CYNAI.CLIENT.CynorkChat.TUILayout](#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0193"></a>

- **REQ-CLIENT-0194 (proposed):** Chat UIs SHOULD support explicit authenticated download actions for assistant-provided files when the gateway exposes them.
  The UI MUST present file metadata clearly and MUST NOT auto-download without user action.
  [CYNAI.USRGWY.OpenAIChatApi.TextInput](#spec-cynai-usrgwy-openaichatapi-textinput)
  [CYNAI.USRGWY.ChatThreadsMessages.DownloadRefs](#spec-cynai-usrgwy-chatthreadsmessages-downloadrefs)
  [CYNAI.CLIENT.CynorkChat.TUILayout](#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0194"></a>

- **REQ-CLIENT-0198 (proposed):** Chat UIs MAY support an **@ shorthand** in the composer for referencing local files (Cursor-like).
  When the user types `@`, the client MAY offer filesystem lookup (path or filename completion from a configurable search path, e.g. current directory or project root).
  When the message is submitted, the client MUST resolve each @ reference to a local file path, upload that file (or include it per gateway contract), and send the message with the resulting file references so the model receives the file content or reference.
  [CYNAI.USRGWY.OpenAIChatApi.TextInput](#spec-cynai-usrgwy-openaichatapi-textinput) (4.6.1)
  [CYNAI.CLIENT.CynorkChat.AtFileReferences](#spec-cynai-client-cynorkchat-atfilereferences)
  <a id="req-client-0198"></a>

- **REQ-CLIENT-0195 (proposed):** Rich chat UIs SHOULD provide a user-level toggle to show or hide model thinking blocks when such blocks are available.
  Thinking MUST be hidden by default; when hidden, the transcript SHOULD show a compact placeholder indicating that a thinking block exists and can be expanded.
  [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
  [CYNAI.CLIENT.CynorkChat.TUILayout](#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0195"></a>

- **REQ-CLIENT-0196 (proposed):** The cynork TUI SHOULD support queueing one or more drafted messages for later send.
  Queued messages MUST remain editable or removable before send, and the UI MUST make the pending-send state obvious so drafts are not mistaken for already-sent messages.
  [CYNAI.CLIENT.CynorkChat.TUILayout](#spec-cynai-client-cynorkchat-tuilayout)
  [CYNAI.CLIENT.CynorkChat.LocalConfig](#spec-cynai-client-cynorkchat-localconfig)
  <a id="req-client-0196"></a>

- **REQ-CLIENT-0197 (proposed):** The CLI SHOULD first expose the new full-screen TUI explicitly as `cynork tui`.
  After that implementation is feature-complete for the intended initial rollout, the CLI SHOULD make the same surface the default interactive behavior when `cynork` is invoked with no subcommand, while keeping existing command paths such as `cynork chat` and other subcommands available during migration.
  [CYNAI.CLIENT.CynorkTui.EntryPoint](#spec-cynai-client-cynorktui-entrypoint)
  [CYNAI.CLIENT.CynorkChat.TUILayout](#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0197"></a>
  **Refinement:** First rollout implements only the first sentence (`cynork tui` explicit entrypoint).
  Making bare `cynork` launch the TUI is explicitly deferred (see [Integration Plan and Refinements](#21-integration-plan-and-refinements)); REQ-CLIENT-0187--0198 (and 0199--0204) are not yet in stable requirements.

### 3.4 Orchestrator, Database, and PMA (File Upload and Retrieval)

Backend support for `@`-shorthand file uploads so that user-attached files reach the PMA/LLM.
These requirements are **proposed** and would live in the indicated requirements files if accepted.

- **REQ-ORCHES-0167 (proposed):** When the User API Gateway receives an OpenAI-compatible chat completion request that includes user message file references (e.g. `file_id` from a prior upload or inline content parts), the orchestrator MUST resolve those references to retrievable content and MUST pass the resolved content (or stable refs the PMA can resolve) into the completion path so the model receives the file content in the request context.
  Resolution MAY be delegated to the gateway or a dedicated service; the orchestrator MUST NOT drop or ignore file parts when forwarding to PMA or the inference backend.
  [CYNAI.ORCHES.ChatFileUploadFlow](#spec-cynai-orches-chatfileuploadflow)
  <a id="req-orches-0167"></a>

- **REQ-ORCHES-0168 (proposed):** When the completion path uses retained response state or thread message history, the orchestrator (or gateway) MUST include any file content or file refs associated with user messages in that history when building the request to PMA or the LLM, so that multi-turn conversations that referenced files continue to have access to that content where the contract requires it.
  [CYNAI.ORCHES.ChatFileUploadFlow](#spec-cynai-orches-chatfileuploadflow)
  <a id="req-orches-0168"></a>

- **REQ-SCHEMA-0114 (proposed):** When the gateway accepts user file uploads for chat (per section 4.6.1), the system MUST persist uploaded file content or stable references in a way that is scoped to the authenticated user and thread (or message) and is retrievable by the orchestrator or gateway for the duration of the completion request and any retention policy.
  Storage MAY be in PostgreSQL (e.g. chat_message_attachments or blob table with message_id, user_id, thread_id) or in object storage with metadata that allows authorization and retrieval by the same user/thread.
  File content MUST be subject to the same secret redaction and size/type limits as defined in the gateway upload contract.
  [CYNAI.USRGWY.ChatThreadsMessages.FileUploadStorage](#spec-cynai-usrgwy-chatthreadsmessages-fileuploadstorage)
  <a id="req-schema-0114"></a>

- **REQ-PMAGNT-0115 (proposed):** When the Project Manager Agent (or equivalent chat-serving agent) receives a chat completion request that includes user message parts with file content or file references, the agent MUST include that content in the LLM request in a form the model supports (e.g. inline text, base64 image parts, or provider-specific `image_url` / `file` content blocks) so the model can use the attached files when generating the response.
  The agent MUST NOT strip or ignore file parts that were accepted by the gateway and passed through the orchestrator.
  [CYNAI.PMAGNT.ChatFileContext](#spec-cynai-pmagnt-chatfilecontext)
  <a id="req-pmagnt-0115"></a>

**Integration note:** File upload/retrieval for chat is deferred in the first TUI rollout; these requirements are for the phase when `@` shorthand and gateway upload are promoted.
Stable orchestrator, schema, and PMA requirements do not yet include these IDs.

## 4 Proposed Spec Additions (Gateway and Chat Data)

These would extend [Chat Threads and Messages](../tech_specs/chat_threads_and_messages.md) and related specs.
Each Spec Item below follows the mandatory structure: Spec ID anchor on the first bullet line, optional metadata, then contract content and Traces To.

### 4.1 Thread Title (Naming)

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ThreadTitle` <a id="spec-cynai-usrgwy-chatthreadsmessages-threadtitle"></a>
- Status: proposed
- Traces To: [REQ-USRGWY-0142](#req-usrgwy-0142), [REQ-CLIENT-0200](#req-client-0200)
- The existing thread model already recommends `title` (text, optional).
  Spec addition: the gateway MUST allow `PATCH /v1/chat/threads/{thread_id}` with a request body that includes `title` (string).
  The gateway MUST reject PATCH for threads not owned by the authenticated user.
  No other thread fields need to be mutable for MVP; only `title` and `updated_at` (server-maintained) are updated.
- Auto-title: the spec MAY define that when `title` is absent, clients display a fallback (e.g. first N characters of the first user message, or "New chat").
  Server-side auto-generation of title from first message is optional and MAY be added later; if added, it MUST use redacted content only.

### 4.2 Thread Summary

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ThreadSummary` <a id="spec-cynai-usrgwy-chatthreadsmessages-threadsummary"></a>
- Status: proposed
- Traces To: [REQ-USRGWY-0143](#req-usrgwy-0143), [REQ-CLIENT-0201](#req-client-0201)
- Optional field on thread: `summary` (text, optional), max length TBD (e.g. 200-500 characters).
  If present, it is a short plaintext summary for list/sidebar display.
  Generation: MAY be set by client on create/update; MAY be generated asynchronously by the server from redacted message content; MUST NOT contain plaintext secrets.
  Storage: add `summary` to `chat_threads` if this feature is implemented; index not required for MVP.
- API: `GET /v1/chat/threads` and `GET /v1/chat/threads/{thread_id}` responses SHOULD include `summary` when present.
  Optional: `PATCH /v1/chat/threads/{thread_id}` MAY allow client to set or clear `summary`.

### 4.3 Chat History List Behavior

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.HistoryList` <a id="spec-cynai-usrgwy-chatthreadsmessages-historylist"></a>
- Status: proposed
- Traces To: [REQ-USRGWY-0144](#req-usrgwy-0144), [REQ-CLIENT-0199](#req-client-0199)
- `GET /v1/chat/threads` behavior to support "chat history" UX:
  - Query parameters: existing `project_id` filter and pagination (`limit`, `offset`).
  - Add optional `sort` (e.g. `updated_at_asc` | `updated_at_desc`); default MUST be `updated_at_desc`.
  - Add optional `archived` (boolean) if soft-delete/archive is implemented: when `false` (default), exclude archived threads; when `true`, return only archived threads.
  - Response: each thread object MUST include `id`, `title`, `updated_at`, `created_at`, and optionally `summary`, `project_id`, `message_count` (if the gateway chooses to expose it).

### 4.4 Archive / Soft-Delete (Optional)

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.Archive` <a id="spec-cynai-usrgwy-chatthreadsmessages-archive"></a>
- Status: proposed
- Traces To: [REQ-USRGWY-0145](#req-usrgwy-0145)
- If REQ-USRGWY-0145 is accepted: add optional `archived_at` (timestamptz, nullable) to `chat_threads`.
  When non-null, the thread is considered archived.
  `PATCH /v1/chat/threads/{thread_id}` MAY allow `archived: true | false`; setting `archived: true` sets `archived_at` to current time; `false` clears it.
  List threads MUST filter by `archived_at` according to the `archived` query parameter.
  Retention and purge rules apply to archived threads the same as active ones unless a separate policy is defined.

### 4.5 Structured Chat Turns

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns` <a id="spec-cynai-usrgwy-chatthreadsmessages-structuredturns"></a>
- Status: proposed
- Traces To: [REQ-USRGWY-0139](#req-usrgwy-0139), [REQ-CLIENT-0192](#req-client-0192), [REQ-CLIENT-0193](#req-client-0193)
- Chat messages SHOULD evolve from plain text only to a dual representation:
  - `content` remains the canonical plain-text transcript for simple clients, title fallback, list preview, search, and summary generation.
  - `parts` is an optional ordered structured array for rich clients.
- Allowed structured part kinds for the draft:
  - `text`: user-visible prose or Markdown.
  - `thinking`: optional model-emitted reasoning content that is hidden by default in clients and excluded from canonical plain-text transcript generation.
  - `tool_call`: tool invocation metadata such as `call_id`, `tool_name`, `status`, and a redacted or truncated argument preview.
  - `tool_result`: tool outcome metadata such as `call_id`, `status`, `content_type`, and a redacted or truncated result preview.
  - `download_ref`: output file metadata such as `download_id`, `filename`, `media_type`, and `size_bytes`.
  User message input is text only (no `attachment_ref` or other rich parts for user-sent messages).
- Thinking or reasoning content MUST NOT be included in the canonical plain-text `content` field, thread `summary`, or thread `title`.
  If preserved at all, it SHOULD be carried only as a dedicated `thinking` part or equivalent side-channel metadata so clients can hide it by default.
- If the system needs observability for reasoning-aware models, it MAY persist bounded `thinking` parts for the originating message or a boolean marker such as `reasoning_redacted: true` in message metadata.
  Rich clients MUST treat persisted thinking as non-default display content and simple clients MUST ignore it.
- Tool-call arguments and tool-result previews stored in `parts` MUST use already-redacted data and SHOULD be bounded in size so transcript retrieval remains cheap.
- Backward compatibility: clients that ignore `parts` MUST still get a coherent transcript from `content` alone.
  Rich clients SHOULD prefer `parts` when present and fall back to `content` otherwise.
- If the separate LLM-routing draft is promoted, the canonical chat spec should reference that document as the single source of truth for model-specific thinking-block stripping behavior.

### 4.6 Text Input (Markdown Only)

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.TextInput` <a id="spec-cynai-usrgwy-openaichatapi-textinput"></a>
- Status: proposed
- Traces To: [REQ-USRGWY-0140](#req-usrgwy-0140), [REQ-CLIENT-0194](#req-client-0194)
- User message content is text (plain string or Markdown).
  The OpenAI-compatible request parser MUST accept `content` as a string for user messages.
- Markdown syntax is supported for formatting (e.g. headings, lists, code blocks, emphasis); clients MAY render Markdown in the transcript.
- File inclusion for user messages is only via the **@ shorthand** (see section 4.6.1).
  Files referenced by @ are resolved from the filesystem and auto-uploaded when the message is submitted; the gateway MUST accept those uploads and associate them with the message per the contract in 4.6.1.
- Unsupported or disallowed content-part types (other than @-originated file refs per 4.6.1) MUST return a validation error rather than being silently dropped.
  For the OpenAI-compatible endpoint, that validation error SHOULD use the same top-level `error` object shape as other chat errors.

#### 4.6.1 File Reference Shorthand (@)

- When the client sends a message that contains @ references (e.g. `@./path/to/file` or `@filename`), the client MUST have resolved each reference to a local file and MUST upload that file (or include it in the request per the chosen contract).
- **Syntax:** The composer MAY support a shorthand such as `@` followed by a path or filename.
  Resolution is implementation-defined (e.g. current working directory, project root, or a configurable search path).
  Multiple @ references in one message are allowed.
- **On submit:** Before sending the message to the gateway, the client MUST resolve each @ reference to an absolute or relative filesystem path, read the file (subject to size and type limits), and either:
  - upload the file to a gateway-owned endpoint (e.g. `POST /v1/chat/uploads` or similar) and receive a stable `file_id`, then include `file_id` (or equivalent) in the message payload, or
  - include the file content inline in the request (e.g. base64 or multipart) where the gateway spec allows it.
- **Gateway contract:** The gateway MUST define one of: (1) an upload endpoint that returns a stable reference (e.g. `file_id`) for inclusion in the message, or (2) an accepted inline representation (e.g. content parts with type `file` or `image` keyed by @-resolved paths).
  The gateway MUST associate uploaded or inline files with the user message so the model can use them.
  Backend support (orchestrator resolution, storage, and PMA/LLM passthrough) is specified in [section 4.8](#48-backend-support-for-user-file-uploads-orchestrator-database-pma).
- **Limits:** The canonical spec SHOULD define maximum file size and allowed media types for @-referenced uploads; the client SHOULD validate before upload and surface clear errors when a file is too large or disallowed.

### 4.7 Downloadable Chat Outputs

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.DownloadRefs` <a id="spec-cynai-usrgwy-chatthreadsmessages-downloadrefs"></a>
- Status: proposed
- Traces To: [REQ-USRGWY-0141](#req-usrgwy-0141), [REQ-CLIENT-0194](#req-client-0194)
- When a completion or tool execution produces a file the user may want to keep, the gateway SHOULD surface that file as a structured `download_ref` rather than relying on prose such as "I saved a file for you."
- Minimum `download_ref` metadata:
  - `download_id`
  - `filename`
  - `media_type`
  - `size_bytes`
- Optional metadata:
  - `sha256`
  - `source_task_id`
  - `expires_at`
  - `description`
- Retrieval should use an authenticated gateway-owned contract such as `GET /v1/chat/downloads/{download_id}` or a gateway-issued signed URL.
  Authorization SHOULD match the same user, thread, and project visibility rules as the originating chat message.
- Clients MUST require explicit user action to download or open a referenced file.
  The system MUST NOT auto-download assistant files as part of normal transcript rendering.
- If a referenced download has expired or been deleted, the download operation SHOULD fail cleanly with a user-displayable `404` or `410` style result without breaking transcript retrieval.

### 4.8 Backend Support for User File Uploads (Orchestrator, Database, PMA)

These spec items define how uploaded files (from the `@` shorthand and gateway upload contract in 4.6.1) flow through the orchestrator, are stored and retrieved, and are passed to the PMA/LLM.

#### 4.8.1 Orchestrator Chat File Upload Flow

- Spec ID: `CYNAI.ORCHES.ChatFileUploadFlow` <a id="spec-cynai-orches-chatfileuploadflow"></a>
- Status: proposed
- Traces To: [REQ-ORCHES-0167](#req-orches-0167), [REQ-ORCHES-0168](#req-orches-0168)
- When the gateway receives a chat completion request (e.g. `POST /v1/chat/completions` or `POST /v1/responses`) with user message content that includes file references (e.g. `file_id` from `POST /v1/chat/uploads` or inline content parts), the gateway MUST pass those references or content to the orchestrator.
- The orchestrator MUST resolve each file reference to content (or to a stable ref that the PMA or inference backend can resolve) before or when building the request to the Project Manager Agent or the inference endpoint.
  Resolution MAY be done by the gateway (e.g. gateway stores uploads and injects content into the message payload sent to orchestrator) or by the orchestrator (e.g. orchestrator calls a storage or gateway API to fetch content by `file_id`).
- When the completion path uses thread message history (e.g. for multi-turn context), the orchestrator (or the component that assembles the history) MUST include file content or resolvable file refs for any user message that had attachments, so that the LLM receives the same context as for the original turn.
- File content MUST be subject to the same size and type limits and redaction rules as the gateway contract; the orchestrator MUST NOT forward content that the gateway would have rejected.

#### 4.8.2 File Upload Storage (Chat Message Attachments)

- Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.FileUploadStorage` <a id="spec-cynai-usrgwy-chatthreadsmessages-fileuploadstorage"></a>
- Status: proposed
- Traces To: [REQ-SCHEMA-0114](#req-schema-0114), [REQ-USRGWY-0140](#req-usrgwy-0140)
- When the gateway accepts file uploads (per 4.6.1), it MUST store the file content or a stable reference in a way that:
  - Is scoped to the authenticated user and to the thread or message being composed.
  - Allows retrieval by the orchestrator (or gateway) when building the completion request and when assembling thread history for subsequent turns.
  - Enforces the same authorization as chat thread and message access (user, project, thread visibility).
- Storage options (implementation-defined): PostgreSQL table(s) for chat message attachments (e.g. `chat_message_attachments` with `message_id`, `file_id`, `user_id`, `thread_id`, content or blob ref, `media_type`, `size_bytes`, `created_at`); or object storage with metadata that encodes user/thread scope and is used for retrieval with the same auth checks.
- Retention: Uploaded file storage SHOULD follow the same or a documented retention policy as chat messages (e.g. purge when the associated message is purged or after a TTL).
- Redaction: Content MUST be redacted for secrets before persistence per existing chat redaction requirements.

#### 4.8.3 PMA Chat File Context

- Spec ID: `CYNAI.PMAGNT.ChatFileContext` <a id="spec-cynai-pmagnt-chatfilecontext"></a>
- Status: proposed
- Traces To: [REQ-PMAGNT-0115](#req-pmagnt-0115)
- When the Project Manager Agent (or the component that builds the LLM request for chat) receives a user message that includes file content or file references (from the orchestrator/gateway pipeline), the agent MUST include that content in the payload sent to the LLM.
- Format: The agent MUST map stored or inline file content to the format(s) the target model supports (e.g. text parts, base64 image parts, or provider-specific `image_url` / file content blocks in the chat message array).
  For multi-modal models, image and other non-text types SHOULD be sent as the appropriate content type; for text-only models, the agent MAY convert to a text representation (e.g. "Attachment: filename (N bytes)" or inline extracted text) when the model cannot accept binary parts.
- The agent MUST NOT drop or ignore file parts that were successfully accepted by the gateway and passed through the orchestrator.
- If the model does not support a given file type, the agent SHOULD surface a clear user-visible error or fallback (e.g. "Model does not support image attachments") rather than silently omitting the file.

## 5 Proposed Spec Additions (Cynork Chat TUI)

These would extend [cli_management_app_commands_chat.md](../tech_specs/cli_management_app_commands_chat.md) and related cynork specs.
Spec IDs use the CLIENT domain per [requirements_domains.md](../docs_standards/requirements_domains.md).

### 5.1 TUI Layout and Interaction

- Spec ID: `CYNAI.CLIENT.CynorkChat.TUILayout` <a id="spec-cynai-client-cynorkchat-tuilayout"></a>
- Status: proposed
- Traces To: [REQ-CLIENT-0202](#req-client-0202), [REQ-CLIENT-0203](#req-client-0203), [REQ-CLIENT-0204](#req-client-0204), [REQ-CLIENT-0189](#req-client-0189), [REQ-CLIENT-0192](#req-client-0192), [REQ-CLIENT-0193](#req-client-0193), [REQ-CLIENT-0194](#req-client-0194)
- A dedicated section in the cynork chat spec specifying the chat TUI layout and interaction rules:
  - Multi-line composer (toggle, soft wrap, explicit send key).
  - Scrollback with selection and copy; in-TUI search over the visible buffer.
  - Streaming output when the gateway supports it, with graceful fallback to non-streaming.
  - Persistent status bar: gateway URL, auth identity, project context, selected model, connection state.
  - Optional right-side context pane: current project, recent tasks and status, selected task detail (result/logs), slash command help.
  - Unified command palette including slash commands and common actions.

#### 5.1.1 Region Layout and Positioning

The TUI occupies the terminal (full-screen or current terminal size).
Regions are stacked and optionally split as follows; all coordinates are relative (percent of terminal rows/columns) so the layout works across resize.

- **Status bar (bottom, fixed):** Single line at the bottom of the terminal.
  Contains: gateway base URL (truncated if needed), identity (e.g. user handle or "anonymous"), project context (id or "default"), selected model id (truncated), connection state (e.g. "connected" / "reconnecting" / "offline").
  Separators between fields (e.g. `|`) are optional; layout MUST remain readable when `--no-color` is set.
  Height: exactly 1 row.
  Width: full terminal width.

- **Composer (above status bar, fixed):** Input area for the current message.
  Default height: 1 row (single-line mode).
  Multi-line mode: 2 to 5 rows (configurable or toggleable).
  Soft wrap: when a line exceeds terminal width, it wraps to the next composer line without inserting a newline character unless the user explicitly inserts one.
  Width: full terminal width when context pane is hidden; when context pane is visible, composer width is the remaining left region (see below).
  Cursor and prompt (e.g. `>`) are shown; placeholder text (e.g. "Type a message or / for commands") is optional.

- **Scrollback (main area, above composer):** Renders the conversation history and streaming assistant output.
  Fills all remaining vertical space above the composer (i.e. from top of terminal to composer top).
  Width: same as composer (full width or left region when context pane visible).
  Content: user messages and assistant messages in order.
  **Conversation history MUST be displayed as formatted Markdown** (same as current `cynork chat` for agent responses): headings, lists, code blocks, emphasis, and links MUST be rendered in the scrollback when `--plain` is not set; when `--plain` is set, the scrollback MAY show raw text only.
  **User messages** in the scrollback MUST be visually distinguished from assistant messages (e.g. a slightly different background color or region styling) so user input stands out in the stream.
  Streaming updates append to the last assistant message until complete.
  Scrolling: vertical scroll when content exceeds visible rows; scroll position MAY follow the latest content (auto-scroll) or stay fixed when user has scrolled up; behavior SHOULD be specified (e.g. follow by default, or "scroll to bottom" key).
  **Scrollback history and reload:** The TUI MUST support scrolling through the conversation history (e.g. Page Up / Page Down by default).
  When the user scrolls back beyond the currently loaded content, the client MUST fetch and append older messages (paginated history) so that scrolling can traverse the full thread history.

- **Context pane (optional, right side):** When visible, a vertical split reserves a portion of the terminal width for the context pane.
  Recommended width: 24--32 columns or 20--30% of terminal width (whichever is smaller), with a minimum of 20 columns.
  The pane shows one of: (1) current project context (project id, title, summary), (2) recent tasks list with status, (3) selected task detail (result snippet or log tail), (4) slash command help (list of commands and short descriptions).
  Switching between these views MAY be by key (e.g. Tab or number keys) or a small inline tab strip at the top of the pane.
  When the context pane is hidden, the scrollback and composer expand to full width.

- **Command palette (overlay, optional):** When invoked (e.g. Ctrl+P or F10), a modal or overlay appears over the center of the TUI listing slash commands and common actions (e.g. "New thread", "Toggle context pane", "Search").
  User can filter by typing and select with Enter; Escape dismisses.
  This overlay does not change the underlying region layout; it is drawn on top.

#### 5.1.2 Keybindings and Input Semantics

- **Send message:** In single-line composer, Enter sends the current line (after trim) and clears the input.
  In multi-line composer, Enter inserts a newline; a dedicated "send" key (e.g. Ctrl+Enter or Alt+Enter) sends the full composer content and clears.
  The inverse (Enter to send in multi-line, Ctrl+Enter for newline) MAY be configurable; the spec SHALL pick a default and document it.
- **Newline in composer:** **Shift+Enter** MUST insert a line break (newline) in the composer in both single-line and multi-line modes, so the user can add multiple lines without sending.
- **Queue message:** A dedicated action (for example Ctrl+Q or a command-palette action) SHOULD move the current composer content into a queued-drafts list without sending it.
  The composer is then cleared so the user can continue drafting.
  A queued item MUST remain editable, reorderable, removable, and explicitly sendable later.

- **Slash and shell:** Input starting with `/` is parsed as a slash command.
  Input starting with **`!`** is the **shell escape** (`! command` shorthand): the remainder of the line is run as a shell command.
  The CLI MUST support this shorthand; it SHOULD be enabled by default (configurable per implementation; see REQ-CLIENT-0204 (proposed)).
  Tab after `/` triggers slash-command completion; Tab in other contexts (e.g. after `/task get`) triggers context-specific completion (task id, project id, model id) per [5.2 Completion and Discovery](#spec-cynai-client-cynorkchat-completion).

- **Composer input history:** **Up / Down arrow keys** when focus is in the composer MUST cycle through previously sent messages (composer history).
  Up loads the previous sent message into the composer; Down loads the next (or clears toward the most recent).
  This allows re-sending or editing a prior message without retyping.
- **Cancellation and exit (Ctrl+C):** **Ctrl+C** MUST cancel the current operation when no text is selected (e.g. cancel an in-flight completion request, or clear the composer if appropriate).
  When there is a selection, Ctrl+C copies to the system clipboard (platform copy) and does not cancel.
  **Multiple successive Ctrl+C:** When the user presses Ctrl+C again with no selection and no operation in progress (e.g. idle at composer), the second consecutive Ctrl+C MUST exit the session cleanly (similar to Cursor agent).
  Implementations MAY require two quick successive Ctrl+C within a short window (e.g. 1-2 seconds) to avoid accidental exit.
- **Scrollback interaction:** Arrow keys or **Page Up / Page Down** (default) scroll the scrollback when focus is in scrollback (e.g. after clicking or focusing the main area).
  When the user scrolls back past the loaded content, the client MUST load older message history (see scrollback history and reload above).
  Selection: mouse or shift+arrows to select text; Copy (e.g. Ctrl+C or platform copy when selection is active) copies selection to system clipboard; no secrets in scrollback (per existing security rules).
  Search: Ctrl+F (or `/` when focus in scrollback) opens an inline search field; typing filters or jumps to next match; Escape closes search.

- **Context pane:** A key (e.g. Ctrl+R or F9) toggles visibility of the context pane.
  When visible, Tab or number keys switch between pane views (project / tasks / task detail / slash help) as specified.

- **Thinking visibility toggle:** A key or command-palette action SHOULD toggle whether `thinking` parts are expanded.
  The default state MUST be hidden.
  When hidden, the UI SHOULD render a compact placeholder such as `[thinking hidden]` or `[thinking block collapsed]` instead of the raw reasoning text.

- **Command palette:** Ctrl+P or F10 opens the command palette; Escape or focus-out closes it.

- **Exit:** `/exit`, `/quit`, or EOF (e.g. Ctrl+D) ends the session; behavior per existing [CliChat](../tech_specs/cli_management_app_commands_chat.md) spec.

#### 5.1.3 Shell Escape and Interactive Subprocesses

When the user runs a command via `!` (e.g. `!vim file.txt`), that command runs in the user's shell.
If the command is **interactive** (e.g. vim, less, htop) and takes over the terminal (raw mode, alternate screen, or full-screen UI), the following applies.

- **Required behavior:** The TUI MUST suspend its own rendering and give the subprocess full control of the terminal for the duration of the command.
  Concretely: before exec'ing or spawning the shell command, the CLI MUST switch the terminal to cooked/canonical mode (if the TUI uses raw mode) and MUST NOT capture or redirect stdin/stdout/stderr so the subprocess receives the real TTY.
  When the subprocess exits, the CLI MUST restore the TUI state (re-enter raw mode if used, redraw layout) and continue the chat session.
  The scrollback MAY show a single inline line such as `[ ran: vim file.txt (exit 0) ]` or MAY show nothing; the CLI MUST NOT attempt to "capture" the interactive program's output as if it were plain stdout.

- **Non-interactive commands:** For commands that do not take over the terminal (e.g. `!ls`, `!cat file`), the CLI MAY run them in a PTY or pipe and display stdout/stderr inline in the scrollback as today; that behavior remains valid.
  The requirement above applies whenever the user invokes any command via `!`; implementations MUST support interactive subprocesses (e.g. `!vim`, `!less`) by handing off the real TTY and resuming the TUI on exit.

#### 5.1.4 Interaction Flow (Mermaid)

The following diagram summarizes the main user and system flow for the chat TUI.

```mermaid
flowchart TB
  subgraph init["Startup"]
    A[Resolve gateway + token] --> B{Token OK?}
    B -->|No| C[Exit code 3]
    B -->|Yes| D[Optional warm-up]
    D --> E[Draw layout: status bar, composer, scrollback]
    E --> F[Focus composer]
  end

  subgraph loop["Main loop"]
    F --> G[User input in composer]
    G --> H{First char?}
    H -->|/| I[Slash command path]
    H -->|!| J[Shell escape path]
    H -->|other| K[Chat message path]

    I --> I1[Complete / filter commands]
    I1 --> I2[Execute slash command]
    I2 --> I3[Output inline in scrollback]
    I3 --> G

    J --> J1[Run shell command]
    J1 --> J2[Output inline in scrollback]
    J2 --> G

    K --> K1[POST /v1/chat/completions]
    K1 --> K2[Stream or full response]
    K2 --> K3[Append to scrollback, render Markdown]
    K3 --> G
  end

  subgraph pane["Optional context pane"]
    P[Toggle pane] --> P1[Project view]
    P1 --> P2[Tasks view]
    P2 --> P3[Task detail view]
    P3 --> P4[Slash help view]
    P4 --> P1
  end
```

#### 5.1.5 Layout Structure (Mermaid)

The following diagram shows the spatial relationship of TUI regions.
Top to bottom: scrollback (flex), then composer (fixed height), then status bar (1 row).
When the context pane is visible, it occupies the right column next to scrollback and composer; status bar spans the full width.

```mermaid
flowchart TB
  subgraph row1[" "]
    direction LR
    scrollback["Scrollback<br>(conversation + streaming)"]
    pane["Context pane (optional)<br>Project | Tasks | Detail | Help"]
  end
  composer["Composer (input + prompt)"]
  status["Status bar:<br> Gateway | Identity | Project | Model | Connection"]
  scrollback --> composer
  composer --> status
  pane --> status
```

Vertical order: row1 (scrollback left, context pane right when visible), then composer, then status bar.
Horizontal: scrollback and context pane share the top row; composer spans left column only; status bar spans full width.

#### 5.1.6 TUI Visual Mockup

The following mockup illustrates the TUI with context pane visible and a sample conversation.

![Cynork chat TUI mockup](../tech_specs/images/cynork_chat_tui_mockup.png)

#### 5.1.7 Structured Transcript Rendering

The scrollback SHOULD distinguish transcript item kinds instead of flattening everything into assistant prose.

- **Conversation history and Markdown:** The full conversation history (user and assistant messages) in the scrollback MUST be displayed with **formatted Markdown** when `--plain` is not set, consistent with current `cynork chat` behavior for agent responses (headings, lists, code blocks, emphasis, links).
  This applies to both stored history and streaming assistant output.
- **User and assistant text:** Render as the main readable transcript.
  When `--plain` is not set, assistant `text` parts (and user message text, where applicable) MUST use Markdown-aware rendering so that conversation history is readable and formatted like current cynork chat.
- **User message styling:** User messages in the scrollback MUST be shown with a distinct background (e.g. slightly different background color or subtle region styling) so they are clearly identifiable as user input in the conversation stream.
- **Thinking or reasoning:** Thinking is a non-default transcript item kind.
  During generation the UI MAY show an ephemeral status row such as `Thinking...`, spinner text, or similar progress indicator.
  When the final response includes a `thinking` part and the user has not enabled expanded thinking view, the transcript SHOULD render only a compact placeholder for that block.
  When the user enables the thinking toggle, the UI MAY expand the raw block inline or in a side panel.
- **Tool calls:** Render as a distinct transcript row/card showing at least tool name and current state (`running`, `succeeded`, `failed`, or equivalent).
  Argument previews SHOULD be redacted and truncated.
- **Tool results:** Render as a distinct row/card linked to the originating tool call when a `call_id` is available.
  Large or verbose result bodies SHOULD be collapsed by default with an explicit expand action.
- **Fallback:** If the gateway provides only plain `content` and no structured `parts`, the client SHOULD fall back to text-only rendering without inventing fake tool rows.

#### 5.1.8 Queued Drafts and Deferred Send

The TUI SHOULD support an explicit outbox-like draft queue for messages the user wants to send later.

- **Queue model:** A queued draft is local client state until sent.
  It is not part of the server-side transcript and MUST NOT be shown as a sent user message.
- **Display:** Queued drafts SHOULD appear in a dedicated composer-adjacent list, context-pane view, or overlay with clear "queued" labeling.
- **Operations:** The user SHOULD be able to add the current composer to the queue, edit a queued draft, remove a queued draft, reorder queued drafts, and send one queued draft or all queued drafts.
- **Send semantics:** Sending a queued draft turns it into a normal user message at the time of actual send.
  The UI SHOULD indicate whether queued drafts will be sent sequentially and whether the client waits for each response before sending the next queued draft.
- **Safety:** Queued drafts may contain @ references; when sent, @-referenced files are uploaded at send time.
  The UI SHOULD surface any validation or send failure (e.g. missing file, upload error) clearly.

#### 5.1.9 Downloads (Assistant-Provided Only)

- **Download refs:** When the gateway exposes assistant-provided files (e.g. via `download_ref` parts), the TUI SHOULD surface them as explicit download items with filename, media type, and size when known.
  The action label MAY be a keybinding, command palette action, or slash-command hint, but the interaction MUST require user intent.
  The UI MUST NOT auto-download without user action.

#### 5.1.10 @ File References (Cursor-Like)

- Spec ID: `CYNAI.CLIENT.CynorkChat.AtFileReferences` <a id="spec-cynai-client-cynorkchat-atfilereferences"></a>
- Traces To: [REQ-CLIENT-0198](#req-client-0198)
- **Composer:** The composer accepts text with Markdown.
  The only mechanism for including files in a user message is the **@ shorthand** (no separate attachment picker or drag-and-drop).
- **Trigger:** When the user types `@`, the TUI MAY open a filesystem lookup (autocomplete or browser).
  Lookup SHOULD search from a configurable base (e.g. current working directory, project root, or `@`-search path in config).
  The user selects a file or types a path; the TUI inserts the reference (e.g. `@./path/to/file` or `@filename`) into the composer text.
- **On submit:** When the user sends the message, the client MUST resolve each @ reference in the text to a local file path, upload that file to the gateway (or include it per gateway contract), and send the message with the resulting file references so the model receives the file content.
  Resolution MUST happen at send time; if a referenced file is missing or unreadable, the client MUST surface an error and MUST NOT send the message until the user fixes or removes the reference.
- **Display:** The TUI MAY render @ references in the composer as chips or inline path hints so the user can see which files will be uploaded on send.
  After send, the transcript MAY show that the message included file refs (e.g. "1 file attached") without duplicating the raw @ syntax in the stored message if the gateway normalizes it.

### 5.2 Completion and Discovery

- Spec ID: `CYNAI.CLIENT.CynorkChat.Completion` <a id="spec-cynai-client-cynorkchat-completion"></a>
- Status: proposed
- Traces To: [REQ-CLIENT-0203](#req-client-0203)
- Completion sources and constraints:
  - Autocomplete and fuzzy selection for: task identifiers (UUID and human-readable names) in slash commands; project selection for `/project set`; model selection for `/model`.
  - Completion data: task list, project list, model list calls as defined by existing APIs; no new gateway endpoints required.

### 5.3 Non-Interactive and Scripting

- Spec ID: `CYNAI.CLIENT.CynorkChat.NonInteractive` <a id="spec-cynai-client-cynorkchat-noninteractive"></a>
- Status: proposed
- Behavior for non-interactive or scripted use (e.g. `--plain` and optional `--once` flag), so that chat can be driven from scripts without TUI embellishments.
- In `--plain` or other script-oriented modes, stdout SHOULD contain only the final assistant text.
  Structured tool rows, progress indicators, and download actions SHOULD be omitted from stdout so scripts do not need to parse UI chrome.
- If the implementation chooses to surface non-text metadata in non-interactive mode, it SHOULD do so on stderr or via an explicit machine-readable output mode rather than mixing it into the assistant text stream.

### 5.4 Shell Deprecation and Doc Alignment

Doc changes (no new Spec ID; alignment of existing specs and requirements):

- In [cynork_cli.md](../tech_specs/cynork_cli.md): remove "Interactive shell mode with tab completion" from MVP scope or mark it deprecated.
- In [cli_management_app_shell_output.md](../tech_specs/cli_management_app_shell_output.md): remove the Interactive Mode (REPL) section or rewrite as "Chat UI interaction rules" if that document remains the home for output/scripting rules.
- Requirements: remove or deprecate `REQ-CLIENT-0136` through `REQ-CLIENT-0159` (and related REPL requirements) and replace with the chat-as-primary and TUI requirements proposed above.
- BDD: replace or rewrite [features/cynork/cynork_shell.feature](../../features/cynork/cynork_shell.feature) with chat-based acceptance criteria; extend [features/cynork/cynork_chat.feature](../../features/cynork/cynork_chat.feature) for TUI behaviors (e.g. multi-line send, slash completion).

### 5.5 Local Config (Chat TUI Preferences)

- Spec ID: `CYNAI.CLIENT.CynorkChat.LocalConfig` <a id="spec-cynai-client-cynorkchat-localconfig"></a>
- Status: proposed
- Traces To: [REQ-CLIENT-0187](#req-client-0187)
- Chat-specific preferences MAY be stored in the same config file as the rest of cynork, under a dedicated key (e.g. `chat` or `tui`), or in a separate file under the same config directory (e.g. `$XDG_CONFIG_HOME/cynork/chat.yaml`).
  When using the same file, the structure MUST extend the existing [CliConfigFileLocation](../tech_specs/cynork_cli.md#spec-cynai-client-cliconfigfilelocation) YAML; unknown keys at the top level continue to be ignored; the `chat` (or `tui`) key is optional.
- Allowed keys (all optional): `default_model` (string, OpenAI model id for the session when `--model` is not set); `composer_multiline` (boolean, default false); `context_pane_visible` (boolean, default false); `context_pane_width_columns` (integer, min 20, max 48); `show_thinking_blocks` (boolean, default false); `queue_send_mode` (string, e.g. `manual` or `sequential`); `keybindings` (object mapping action names to key sequences, if overrides are supported).
  No key may hold a secret (token, password, or message content); the CLI MUST NOT write or read secrets from this config.
- Config load: the same `--config` flag and default path resolution as the rest of cynork apply; chat preferences are read after the main config load and MAY be absent (defaults apply).
- If a separate chat config file is used, it MUST live under the same config directory (e.g. `$XDG_CONFIG_HOME/cynork/chat.yaml`); file mode on write MUST be `0600`; atomic write is recommended.

### 5.6 Default Entry Point and Explicit `tui` Command

- Spec ID: `CYNAI.CLIENT.CynorkTui.EntryPoint` <a id="spec-cynai-client-cynorktui-entrypoint"></a>
- Status: proposed; first rollout implemented (explicit `cynork tui` only; bare `cynork` default deferred per [Integration Plan and Refinements](#21-integration-plan-and-refinements)).
- Traces To: [REQ-CLIENT-0197](#req-client-0197)
- Rollout order:
  - First rollout: the CLI SHOULD expose the full-screen chat TUI explicitly as `cynork tui`. **(Done in stable; see [cynork_tui.md](../tech_specs/cynork_tui.md) Entrypoint and Compatibility.)**
  - Later rollout: once the TUI is feature-complete for the intended initial scope, invoking `cynork` with no subcommand SHOULD launch that same TUI by default instead of printing top-level help.
- Existing explicit commands and subcommands MUST remain available, including `cynork chat`, `cynork task`, `cynork project`, `cynork auth`, and other established command paths.
- Compatibility handling:
  - During the first rollout, `cynork tui` SHOULD resolve to the new TUI code path while bare `cynork` continues its existing top-level behavior.
  - In the later default-TUI rollout, `cynork tui` and bare `cynork` SHOULD resolve to the same code path.
  - `cynork chat` MUST remain available as a compatibility alias to the same TUI for the near-term migration period.
  - Obsoleting `cynork chat` SHOULD be deferred to a later dedicated compatibility review and MUST NOT be part of the first TUI-default rollout.
  - `cynork --help` and `cynork help` MUST still show command help instead of entering the TUI.
- Startup auth and model-selection behavior for bare `cynork` SHOULD match the same rules already defined for the TUI entrypoint once the later default-TUI rollout occurs.

### 5.7 Local Cache (Completion and List Data)

- Spec ID: `CYNAI.CLIENT.CynorkChat.LocalCache` <a id="spec-cynai-client-cynorkchat-localcache"></a>
- Status: proposed
- Traces To: [REQ-CLIENT-0188](#req-client-0188)
- The CLI MAY cache completion and list data locally to improve Tab completion and context pane responsiveness.
  Cache location: if `XDG_CACHE_HOME` is set, use `$XDG_CACHE_HOME/cynork/`; otherwise use `~/.cache/cynork/`.
  The CLI MAY create subdirectories (e.g. `completion/`, `threads/`) under that path; directory mode SHOULD be `0700`.
- Cacheable data: task list (ids, names, status); project list (ids, titles); model list (ids); thread list metadata (ids, titles, updated_at only; no message content or summaries that could contain sensitive text).
  Cache MUST NOT contain: tokens, credentials, message content, or any field that could hold user or system secrets.
- TTL and invalidation: each cache entry or cache file SHOULD have a maximum age (e.g. 60--300 seconds); after TTL, the next completion or pane refresh MUST fetch from the gateway and MAY update the cache.
  Invalidation: after a slash command that mutates state (e.g. `/task create`, `/project set`), the CLI SHOULD invalidate the relevant cache (e.g. task list, project context) so the next completion or pane view reflects fresh data.
- File mode: cache files SHOULD be written with mode `0600` (user-only read/write).
  The CLI MAY purge or rotate cache files on startup or when size exceeds a limit; purge MUST NOT delete files outside the cache directory.

### 5.8 Auth Recovery (Login Prompt in Chat)

- Spec ID: `CYNAI.CLIENT.CynorkChat.AuthRecovery` <a id="spec-cynai-client-cynorkchat-authrecovery"></a>
- Status: proposed
- Traces To: [REQ-CLIENT-0190](#req-client-0190)
- The chat TUI MUST handle both startup auth gaps and in-session auth failures by prompting for login and resuming the session when possible.
- Startup behavior:
  - If the resolved token is empty at startup, the CLI MUST prompt the user to log in.
  - After a successful login, the CLI MUST start the chat session loop and render the TUI normally.
  - If login fails or is aborted, the CLI MUST exit with an auth error exit code (consistent with [Exit Codes](../tech_specs/cynork_cli.md#spec-cynai-client-cliexitcodes)).
- In-session behavior:
  - If the gateway responds with an auth error (e.g. HTTP 401/403) to a chat completion request or a slash-command gateway call, the CLI MUST pause the session and prompt the user to re-authenticate.
  - After successful re-authentication, the CLI SHOULD offer to retry the failed operation once (default: retry), then resume normal session flow.
  - The CLI MUST NOT loop indefinitely on repeated auth failures; after N consecutive auth failures (N TBD, e.g. 2), the CLI MUST return to the composer and show a clear error telling the user to run `/auth login` or exit.
- Login box (modal/overlay):
  - When login is required (startup or in-session auth failure), the CLI MUST display a small login box over the TUI (modal or overlay), not a full-screen takeover.
  - The login box MUST contain three inputs: gateway URL, username, and password.
  - Gateway URL and username MUST be prepopulated when known (from loaded config `gateway_url`, and from config or last-known identity if available; e.g. a stored "last_username" or config key is optional but when present, prepopulate).
  - Password MUST NOT be prepopulated and MUST use secret input (no echo; e.g. masked or hidden).
  - The box MUST include a way to submit (e.g. "Login" or Enter) and to cancel/abort (e.g. Escape); on cancel at startup, the CLI exits with auth error code; on cancel in-session, the CLI returns to the composer with an error message.
  - The CLI MUST reuse the same auth and token persistence rules as the rest of cynork (same gateway login endpoint, config write, credential helper if configured).
  - The CLI MUST NOT record password or token in history and MUST NOT echo secrets.

### 5.9 Web Login (SSO-Capable Authentication)

- Spec ID: `CYNAI.CLIENT.CliWebLogin` <a id="spec-cynai-client-cliweblogin"></a>
- Status: proposed
- Traces To: [REQ-CLIENT-0191](#req-client-0191)
- The CLI SHOULD support a web-based login flow designed for SSO-capable deployments.
  Acceptable patterns include:
  - device-code flow (CLI prints a short code and verification URL; user completes login in browser; CLI polls for token), or
  - browser-based authorization (CLI opens browser to an auth URL, receives a callback on localhost, and exchanges for token).
- Constraints:
  - The CLI MUST NOT print tokens to stdout in normal operation.
  - The CLI MUST store the resulting token using the existing token storage model (config and/or credential helper) and MUST follow the file-permission and atomic-write rules in [CliConfigFileLocation](../tech_specs/cynork_cli.md#spec-cynai-client-cliconfigfilelocation).
  - The CLI MUST time out and fail cleanly if the user does not complete the web flow within a bounded time.

## 6 Implementation Action Plan

Assumes approval of the proposed requirements and spec changes above.

**Actual execution (TUI-first):** The repository executed a **TUI-first MVP** rather than the full plan below.
  See [2026-03-12_plan_next_round_execution.md](../dev_docs/2026-03-12_plan_next_round_execution.md) for phases 0--8, locked scope (Phase 0/2), backend prerequisites (Phase 3), shared controller and PTY harness (Phase 4--5), and validation (Phase 6).
  Phases 0--2 and the minimum TUI spec cut are done; implementation (Phase 5) and validation (Phase 6) are in progress.
  The refinements in [Integration Plan and Refinements](#21-integration-plan-and-refinements) summarize how the executed plan differs from this draft (entry point, deferred features, REQ ID repurposing).

### 6.1 Phase 0: Contract Decisions

- Decide whether `cynork shell` is removed immediately or deprecated for one release.
- The `! command` shorthand (shell escape) MUST be supported and SHOULD be enabled by default; document in spec (see REQ-CLIENT-0204 (proposed)).
- Decide minimal "cursor-agent-like" TUI scope for first iteration (e.g. layout + multi-line + completion + status bar).
- Decide the canonical structured chat-turn schema (`content` only vs. `content` plus `parts`) before implementation starts.
- User message input is text (Markdown) only; no attachment transport for chat input.
- Decide the first authenticated download contract for assistant-generated files.
- Decide whether persisted `thinking` parts are retained across reloads or are session-local only after response completion.
- Decide whether queued drafts are local-memory only or optionally restored from local config or cache after restart.

### 6.2 Phase 1: Requirements, Specs, and BDD

- Update `docs/requirements/usrgwy.md` and `docs/requirements/client.md` with the proposed requirements.
- Update tech specs: Chat Threads and Messages (title, summary, history list, archive, structured turns, download refs); OpenAI-compatible chat API (text/Markdown input only; no attachment handling for user messages); cynork chat / TUI (layout, queued drafts, thinking toggle, non-interactive, default entrypoint); cynork_cli and shell_output (shell deprecation/removal).
- Update BDD: cynork_shell.feature => chat-based scenarios; extend cynork_chat.feature for TUI behavior.

### 6.3 Phase 2: Gateway and Chat Data (If Approved)

- Implement PATCH thread title, optional summary field and API, list sort/pagination/archive per spec.
- Implement structured chat-turn persistence and retrieval (`content` plus `parts` when approved).
- Implement chat input validation for text (Markdown allowed).
  Implement the @ file-reference contract: gateway upload endpoint (or inline representation) for @-resolved files, and client resolution + auto-upload on message submit.
- Implement authenticated download refs for assistant-produced files if download support is in MVP.

### 6.4 Phase 3: Cynork Chat TUI

- Implement TUI layer for `cynork chat`: composer, panes, status bar, completion (task/project/model), scrollback and search.
- Implement structured transcript rendering: thinking status, thinking placeholder plus toggle, tool call cards, tool result cards, queued drafts, and download actions.
  Implement @ shorthand in the TUI composer: filesystem lookup when user types @, insert reference, and auto-upload referenced files on submit.
- Implement local config (chat TUI preferences) and local cache (completion/list data) per [5.5 Local Config](#spec-cynai-client-cynorkchat-localconfig) and [5.6 Local Cache](#spec-cynai-client-cynorkchat-localcache).
- Implement `cynork tui` as the first explicit entrypoint for the new TUI while preserving existing subcommands and leaving `cynork chat` in place as a supported compatibility alias.
- Preserve: no secrets in local history, config, or cache; honor `--no-color`; do not print or persist tokens.

### 6.5 Phase 4: Make TUI Default After Feature Completion

- After the `cynork tui` rollout is feature-complete for the intended initial scope, switch bare `cynork` to launch the same TUI by default.
- Preserve `cynork --help` and `cynork help` behavior so help remains explicit and automation-friendly.
- Keep `cynork chat` in place as a supported compatibility alias during this default-switch phase.

### 6.6 Phase 5: Remove or Deprecate Cynork Shell

- Remove or deprecate `cynork shell` implementation and supporting packages; align help, docs, and completion.

### 6.7 Phase 6: Later Compatibility Cleanup

- Revisit whether `cynork chat` should be marked deprecated or obsoleted after the default-TUI rollout has been stable for at least one migration window.
- If obsoleted later, update help text, specs, and BDD in that later phase rather than bundling it into the initial TUI-default change.

### 6.8 Phase 7: Validation

- `just docs-check` for all updated docs.
- `just test-bdd` (or cynork-scoped suite) for chat and TUI scenarios.

## 7 Traceability (Proposed)

Canonical links to requirement anchors and Spec Item anchors in this document:

- [REQ-USRGWY-0142](#req-usrgwy-0142) => [Thread Title (4.1)](#spec-cynai-usrgwy-chatthreadsmessages-threadtitle)
- [REQ-USRGWY-0143](#req-usrgwy-0143) => [Thread Summary (4.2)](#spec-cynai-usrgwy-chatthreadsmessages-threadsummary)
- [REQ-USRGWY-0144](#req-usrgwy-0144) => [History List (4.3)](#spec-cynai-usrgwy-chatthreadsmessages-historylist)
- [REQ-USRGWY-0145](#req-usrgwy-0145) => [Archive (4.4)](#spec-cynai-usrgwy-chatthreadsmessages-archive)
- [REQ-USRGWY-0139](#req-usrgwy-0139) => [Structured Chat Turns (4.5)](#spec-cynai-usrgwy-chatthreadsmessages-structuredturns)
- [REQ-USRGWY-0140](#req-usrgwy-0140) => [Text Input (4.6)](#spec-cynai-usrgwy-openaichatapi-textinput)
- [REQ-USRGWY-0141](#req-usrgwy-0141) => [Downloadable Chat Outputs (4.7)](#spec-cynai-usrgwy-chatthreadsmessages-downloadrefs)
- [REQ-CLIENT-0199](#req-client-0199) => [History List (4.3)](#spec-cynai-usrgwy-chatthreadsmessages-historylist)
- [REQ-CLIENT-0200](#req-client-0200) => [Thread Title (4.1)](#spec-cynai-usrgwy-chatthreadsmessages-threadtitle)
- [REQ-CLIENT-0201](#req-client-0201) => [Thread Summary (4.2)](#spec-cynai-usrgwy-chatthreadsmessages-threadsummary)
- [REQ-CLIENT-0202](#req-client-0202) => [TUI Layout (5.1)](#spec-cynai-client-cynorkchat-tuilayout)
- [REQ-CLIENT-0203](#req-client-0203) => [TUI Layout (5.1)](#spec-cynai-client-cynorkchat-tuilayout), [Completion (5.2)](#spec-cynai-client-cynorkchat-completion)
- [REQ-CLIENT-0204](#req-client-0204) => [TUI Layout (5.1)](#spec-cynai-client-cynorkchat-tuilayout)
- [REQ-CLIENT-0187](#req-client-0187) => [Local Config (5.5)](#spec-cynai-client-cynorkchat-localconfig)
- [REQ-CLIENT-0188](#req-client-0188) => [Local Cache (5.6)](#spec-cynai-client-cynorkchat-localcache)
- [REQ-CLIENT-0189](#req-client-0189) => [TUI Layout (5.1)](#spec-cynai-client-cynorkchat-tuilayout), Shell Escape and Interactive Subprocesses (5.1.3)
- [REQ-CLIENT-0190](#req-client-0190) => [Auth Recovery (5.7)](#spec-cynai-client-cynorkchat-authrecovery)
- [REQ-CLIENT-0191](#req-client-0191) => [Web Login (5.8)](#spec-cynai-client-cliweblogin)
- [REQ-CLIENT-0192](#req-client-0192) => [Structured Chat Turns (4.5)](#spec-cynai-usrgwy-chatthreadsmessages-structuredturns), [TUI Layout (5.1)](#spec-cynai-client-cynorkchat-tuilayout)
- [REQ-CLIENT-0193](#req-client-0193) => [Structured Chat Turns (4.5)](#spec-cynai-usrgwy-chatthreadsmessages-structuredturns), [TUI Layout (5.1)](#spec-cynai-client-cynorkchat-tuilayout)
- [REQ-CLIENT-0194](#req-client-0194) => [Text Input (4.6)](#spec-cynai-usrgwy-openaichatapi-textinput), [Downloadable Chat Outputs (4.7)](#spec-cynai-usrgwy-chatthreadsmessages-downloadrefs), [TUI Layout (5.1)](#spec-cynai-client-cynorkchat-tuilayout)
- [REQ-CLIENT-0195](#req-client-0195) => [Structured Chat Turns (4.5)](#spec-cynai-usrgwy-chatthreadsmessages-structuredturns), [TUI Layout (5.1)](#spec-cynai-client-cynorkchat-tuilayout)
- [REQ-CLIENT-0196](#req-client-0196) => [TUI Layout (5.1)](#spec-cynai-client-cynorkchat-tuilayout), [Local Config (5.5)](#spec-cynai-client-cynorkchat-localconfig)
- [REQ-CLIENT-0197](#req-client-0197) => [Default Entry Point (5.9)](#spec-cynai-client-cynorktui-entrypoint), [TUI Layout (5.1)](#spec-cynai-client-cynorkchat-tuilayout)
- [REQ-CLIENT-0198](#req-client-0198) => [Text Input (4.6)](#spec-cynai-usrgwy-openaichatapi-textinput) (4.6.1), [@ File References (5.1.10)](#spec-cynai-client-cynorkchat-atfilereferences)
- [REQ-ORCHES-0167](#req-orches-0167) => [Orchestrator Chat File Upload Flow (4.8.1)](#spec-cynai-orches-chatfileuploadflow)
- [REQ-ORCHES-0168](#req-orches-0168) => [Orchestrator Chat File Upload Flow (4.8.1)](#spec-cynai-orches-chatfileuploadflow)
- [REQ-SCHEMA-0114](#req-schema-0114) => [File Upload Storage (4.8.2)](#spec-cynai-usrgwy-chatthreadsmessages-fileuploadstorage)
- [REQ-PMAGNT-0115](#req-pmagnt-0115) => [PMA Chat File Context (4.8.3)](#spec-cynai-pmagnt-chatfilecontext)

## 8 Out of Scope for This Draft

- Full-text search over chat message content (future).
- Export of thread to file (e.g. Markdown/JSON); can be a later requirement.
- Per-message labels or reactions.
- Thread pinning/favorite as a separate flag (could be modeled later as tag or `pinned_at`).
- New gateway endpoints for "thread selection" or "resume arbitrary thread id" (current OpenAI-compatible contract avoids CyNodeAI-specific thread identifiers on the client).

## 9 Notes and Risks

- Removing or deprecating `cynork shell` requires coordinated changes to requirements, tech specs, and BDD.
- History and naming alone can ride on existing thread endpoints plus PATCH.
  User input is text/Markdown only (no attachment upload).
  Authenticated downloads for assistant-produced files likely require at least one gateway-owned download contract.
- Structured transcript items improve UX, but they also create a compatibility choice: whether to extend the chat-message schema now or overload existing metadata fields temporarily.
- Making bare `cynork` enter the TUI improves discoverability, but that switch should happen only after the explicit `cynork tui` rollout has reached feature completeness and the CLI help path remains explicit and stable.

## 10 Related Documents

- [2026-03-12_plan_next_round_execution.md](../dev_docs/2026-03-12_plan_next_round_execution.md) (TUI-first integration plan; source of refinements and deferred scope)
- [cynork_tui.md](../tech_specs/cynork_tui.md) (stable TUI spec for first rollout)
- [Chat Threads and Messages](../tech_specs/chat_threads_and_messages.md)
- [OpenAI-Compatible Chat API](../tech_specs/openai_compatible_chat_api.md)
- [User API Gateway requirements](../requirements/usrgwy.md) (REQ-USRGWY-0130)
- [Client requirements](../requirements/client.md) (chat and REPL requirements)
- [cynork CLI tech spec](../tech_specs/cynork_cli.md)
- [cli_management_app_commands_chat.md](../tech_specs/cli_management_app_commands_chat.md)
- [cli_management_app_shell_output.md](../tech_specs/cli_management_app_shell_output.md)
