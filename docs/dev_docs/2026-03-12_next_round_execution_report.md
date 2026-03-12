# Next Round Execution Report (2026-03-12)

- [Completed This Session](#completed-this-session-phase-1-tui-enabling-spec-alignment)
- [Phase 0 and Phase 2 Completed](#phase-0-and-phase-2-completed)
- [Validation](#validation)
- [Not Done This Session](#not-done-this-session-deferred-to-next)
- [Recommended Next Steps](#recommended-next-steps)

## Completed This Session (Phase 1 TUI-Enabling Spec Alignment)

Phase 1 spec alignment for chat threads, CLI thread controls, and PMA conversation history is done.

### Chat Thread Creation and Acquisition

- **chat_threads_and_messages.md**
  - Clarified thread association: **active-thread reuse** (single active thread per (user_id, project_id), project from `OpenAI-Project` or default) and **explicit fresh-thread creation** via `POST /v1/chat/threads`.
  - Corrected OpenAI-compatibility wording so explicit thread creation does not require any CyNodeAI-specific thread identifier on subsequent `POST /v1/chat/completions` requests.
  - Updated `POST /v1/chat/threads`: explicit creation semantics, optional `project_id` in body (default project when absent), response returns created thread including `id` for Data REST retrieval and management.
- **usrgwy.md**
  - Added **REQ-USRGWY-0135:** Gateway MUST support explicit creation of a new chat thread via `POST /v1/chat/threads`; thread scoped to user and requested/default project; returned thread id is for retrieval and management, not for OpenAI-compatible chat-completions request routing.
- **client.md**
  - Added **REQ-CLIENT-0181:** CLI chat MUST support explicit fresh-thread creation at session start (`--thread-new`) and in-session (`/thread new`); thread creation via `POST /v1/chat/threads`, respect current project context, and keep subsequent chat-completion requests OpenAI-compatible.

### CLI Thread Controls

- **cli_management_app_commands_chat.md**
  - Added **CYNAI.CLIENT.CliChatThreadControls** (Spec Item): startup `--thread-new` flag and behavior; in-session `/thread new` fresh-conversation behavior; unknown `/thread` subcommands (hint, session continues); project context and `OpenAI-Project` alignment for new threads.
  - Corrected the thread-control wording so explicit fresh-thread creation does not require the CLI to send any CyNodeAI-specific thread identifier on `POST /v1/chat/completions`.
  - Added `/thread new` to slash command reference.
  - Chat Command Traces To updated to include REQ-CLIENT-0181.

### PMA Conversation History

- **cynode_pma.md**
  - Added **CYNAI.PMAGNT.ConversationHistory** (Spec Item): prior turns preserved in context for LangChain-capable path; system-context vs conversation history (prior turns not folded into system block); final executor input = last user turn as distinct user message.

### OpenAI-Compatible Chat Surface Expansion

- **openai_compatible_chat_api.md**
  - Expanded the canonical gateway chat spec from chat-completions-only wording to a dual-surface contract: `POST /v1/chat/completions` and `POST /v1/responses`.
  - Added first-pass `POST /v1/responses` scope for TUI-era support, including `input`, `model`, retained `previous_response_id` continuation, and gateway-owned compatibility translation for providers that do not natively expose the same surface.
- **usrgwy.md**
  - Updated **REQ-USRGWY-0127** and **REQ-USRGWY-0129** so the required OpenAI-compatible interactive chat surface and error semantics now cover `POST /v1/responses` alongside `POST /v1/chat/completions`.
- **client.md**, **cli_management_app_commands_chat.md**, **cynork_tui.md**, **cynork_cli.md**
  - Updated the client and TUI specs so the first TUI rollout now explicitly includes support work for both OpenAI-compatible interactive chat endpoints, while keeping `POST /v1/chat/completions` as the broad compatibility baseline.

## Phase 0 and Phase 2 Completed

- **Phase 0 (locked scope):** [cynork_cli.md](../tech_specs/cynork_cli.md) updated with "TUI Scope and Locked Decisions": deprecate `cynork shell` (compatibility path allowed), ship `cynork tui` as first full-screen entrypoint, keep `cynork chat` available, first TUI thread slice = create/list/switch/rename, minimal parts = thinking + tool activity only, single-binary worker docs promoted this round with implementation deferred.
  Required Top-Level Commands and MVP Scope updated.
- **Phase 2 (TUI MVP spec cut):** [cynork_tui.md](../tech_specs/cynork_tui.md): entrypoint and compatibility, layout and multi-line composer, thread history (create/list/switch/rename), status bar and in-session model/project switching, slash-command parity, auth recovery, non-interactive behavior; explicitly deferred list (summary/archive, download refs, full attachment UX, queued drafts, bare `cynork` default, web/SSO login).

## Validation

`just docs-check` was run earlier in this documentation thread and passed for the prior doc set.
It has not yet been re-run after the newer dual-surface `/v1/responses` spec updates in this follow-up.

## Not Done This Session (Deferred to Next)

- Phase 3 backend work (implement thread create/list/get/patch).
- Phase 3 additive gateway work for `POST /v1/responses` and retained response-continuation metadata.
- Phase 4 cynork TUI implementation.
- Phase 5 BDD/feature updates and `just test-bdd` / `just ci`.
- Phase 6 remaining MVP Phase 2 work.
- Phase 7 single-binary worker deployment docs promotion (promote draft into stable requirements/worker-node tech specs).

## Recommended Next Steps

1. Phase 3: Implement backend thread APIs plus additive `POST /v1/responses` support.
2. Phase 4: Implement `cynork tui` and first-rollout TUI slice per `cynork_tui.md`, including dual chat-surface support.
3. Phase 5: Add BDD coverage for thread controls, TUI behaviors, and both interactive chat surfaces; run `just test-bdd` and `just ci`.
4. Re-run `just docs-check` after the newer `/v1/responses` doc updates settle.
5. Phase 7: Promote single-binary worker proposal into stable requirements and worker-node tech-spec updates.
