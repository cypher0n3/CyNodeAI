# Next Round Execution Plan

- [Purpose](#purpose)
- [Round Goals](#round-goals)
- [Execution Principles](#execution-principles)
- [Phase 0 Locked Scope Decisions](#phase-0-locked-scope-decisions)
  - [Phase 0 Testing Gate](#phase-0-testing-gate)
- [Phase 1 TUI-Enabling Spec Alignment](#phase-1-tui-enabling-spec-alignment)
  - [Phase 1 Testing Gate](#phase-1-testing-gate)
  - [Chat Thread Creation and Acquisition](#chat-thread-creation-and-acquisition)
  - [CLI Thread Controls](#cli-thread-controls)
  - [PMA Conversation History](#pma-conversation-history)
  - [Rich Chat and Dual-Surface Contracts](#rich-chat-and-dual-surface-contracts)
- [Phase 2 TUI MVP Spec Cut](#phase-2-tui-mvp-spec-cut)
  - [Phase 2 Testing Gate](#phase-2-testing-gate)
  - [Must Land in the First TUI Rollout](#must-land-in-the-first-tui-rollout)
  - [Explicitly Deferred From the First TUI Rollout](#explicitly-deferred-from-the-first-tui-rollout)
- [Phase 3 Backend Prerequisites Required for TUI Chat](#phase-3-backend-prerequisites-required-for-tui-chat)
  - [Phase 3 Testing Gate](#phase-3-testing-gate)
  - [Minimum Backend Surface](#minimum-backend-surface)
  - [Required Backend Validation Before TUI Wiring](#required-backend-validation-before-tui-wiring)
  - [Deferred Backend Work This Round](#deferred-backend-work-this-round)
- [Phase 4 Shared Chat Controller and Testable Seams](#phase-4-shared-chat-controller-and-testable-seams)
  - [Phase 4 Testing Gate](#phase-4-testing-gate)
  - [Controller and Session State](#controller-and-session-state)
  - [Transport and Rendering Seams](#transport-and-rendering-seams)
- [Phase 5 `cynork` TUI and Python PTY Harness Implementation](#phase-5-cynork-tui-and-python-pty-harness-implementation)
  - [Phase 5 Testing Gate](#phase-5-testing-gate)
  - [Entry Point and Core Wiring](#entry-point-and-core-wiring)
  - [Core TUI Experience](#core-tui-experience)
  - [Thread and Session UX](#thread-and-session-ux)
  - [Auth Recovery](#auth-recovery)
  - [Python PTY Harness](#python-pty-harness)
  - [Tandem TUI and Harness Validation](#tandem-tui-and-harness-validation)
  - [TUI Chat-Complete Exit for Implementation](#tui-chat-complete-exit-for-implementation)
- [Phase 6 TUI Validation and BDD](#phase-6-tui-validation-and-bdd)
  - [Phase 6 Testing Gate](#phase-6-testing-gate)
- [Phase 7 Remaining MVP Phase 2 Work After TUI MVP](#phase-7-remaining-mvp-phase-2-work-after-tui-mvp)
  - [Phase 7 Testing Gate](#phase-7-testing-gate)
- [Phase 8 Worker Deployment Simplification Docs](#phase-8-worker-deployment-simplification-docs)
  - [Phase 8 Testing Gate](#phase-8-testing-gate)
- [Recommended Execution Order](#recommended-execution-order)
- [Exit Criteria for This Round](#exit-criteria-for-this-round)
- [Progress Notes](#progress-notes)

## Purpose

This temporary plan tracks the next round of work for CyNodeAI as of 2026-03-12 and reflects the stable spec, requirement, and feature updates through 2026-03-13.

The priority for this round is the `cynork` TUI because it is the fastest path to realistic
user-level validation of the current stack.

This plan is derived from:

- [MVP scope](../mvp.md)
- [MVP implementation plan](../mvp_plan.md)
- [client requirements](../requirements/client.md)
- [USRGWY requirements](../requirements/usrgwy.md)
- [ORCHES requirements](../requirements/orches.md)
- [SCHEMA requirements](../requirements/schema.md)
- [PMAGNT requirements](../requirements/pmagnt.md)
- [cynork_tui.md](../tech_specs/cynork_tui.md)
- [chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md)
- [openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md)
- [orchestrator.md](../tech_specs/orchestrator.md)
- [orchestrator_inference_container_decision.md](../tech_specs/orchestrator_inference_container_decision.md)
- [postgres_schema.md](../tech_specs/postgres_schema.md)
- [cynode_pma.md](../tech_specs/cynode_pma.md)
- [worker_node.md](../tech_specs/worker_node.md)
- [worker_node_payloads.md](../tech_specs/worker_node_payloads.md)

The original proposal remains useful as design history in:

- [Cynork TUI draft proposal](../draft_specs/cynork_tui_spec_proposal.md)
- [Chat threads, PMA context, and backend env follow-ups](../draft_specs/chat_threads_pma_context_and_backend_env_followups.md)

The retained TUI design mockup now lives with the canonical TUI spec at:

- [docs/tech_specs/images/cynork_chat_tui_mockup.png](../tech_specs/images/cynork_chat_tui_mockup.png)

The behavior-spec layer for the promoted work now also includes:

- [features/e2e/chat_openai_compatible.feature](../../features/e2e/chat_openai_compatible.feature)
- [features/cynork/cynork_chat.feature](../../features/cynork/cynork_chat.feature)
- [features/cynork/cynork_shell.feature](../../features/cynork/cynork_shell.feature)
- [features/cynork/cynork_tui.feature](../../features/cynork/cynork_tui.feature)
- [features/orchestrator/chat_thread_management.feature](../../features/orchestrator/chat_thread_management.feature)
- [features/agents/pma_chat_file_context.feature](../../features/agents/pma_chat_file_context.feature)
- [features/agents/pma_chat_and_context.feature](../../features/agents/pma_chat_and_context.feature)
- [features/worker_node/node_manager_config_startup.feature](../../features/worker_node/node_manager_config_startup.feature)

The single-binary worker track is in [worker requirements](../requirements/worker.md) and [worker_node.md](../tech_specs/worker_node.md); promotion to stable was completed in a prior round.

## Round Goals

- [x] Promote the chat, thread, TUI, and backend file-context contracts needed for a stable TUI-first source of truth.
  (Stable coverage now includes [REQ-USRGWY-0135](../requirements/usrgwy.md#req-usrgwy-0135) through [REQ-USRGWY-0147](../requirements/usrgwy.md#req-usrgwy-0147), [REQ-CLIENT-0181](../requirements/client.md#req-client-0181) through [REQ-CLIENT-0207](../requirements/client.md#req-client-0207), and [REQ-ORCHES-0167](../requirements/orches.md#req-orches-0167) through [REQ-ORCHES-0169](../requirements/orches.md#req-orches-0169).)
  (It also includes [REQ-SCHEMA-0114](../requirements/schema.md#req-schema-0114), [REQ-PMAGNT-0115](../requirements/pmagnt.md#req-pmagnt-0115), [REQ-PMAGNT-0116](../requirements/pmagnt.md#req-pmagnt-0116), and [REQ-WORKER-0264](../requirements/worker.md#req-worker-0264).)

- [x] Extend the OpenAI-compatible chat contract and TUI work scope to support both `POST /v1/chat/completions` and `POST /v1/responses`.
  (Gateway, orchestrator, PMA, thread storage, and client-side spec anchors now cover both surfaces.)

- [x] Define and lock a first-rollout `cynork` TUI scope that is sufficient for end-to-end stack testing, while also promoting the broader stable contract for later slices.
  (See [CYNAI.CLIENT.CynorkTui.EntryPoint](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-entrypoint), [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout), [CYNAI.CLIENT.CynorkTui.TranscriptRendering](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-transcriptrendering), and [CYNAI.CLIENT.TuiScope](../tech_specs/cynork_cli.md#spec-cynai-client-tuiscope).)

- [ ] Implement the full first usable TUI chat interface and its backend prerequisites in a way that preserves existing CLI compatibility where needed.
  (This is the primary goal for the round and takes precedence over the remaining MVP Phase 2 work.)

- [ ] Build the Python PTY or fullscreen TUI harness in tandem with the TUI so the primary chat flows can be validated with minimal human intervention as behavior lands.

- [ ] Resume the remaining MVP Phase 2 work after the TUI-first validation loop is in place.

- [ ] Keep the single-binary worker proposal visible as a follow-on architecture track, but do not implement single-binary worker changes in this round.

## Execution Principles

- [x] Resolve spec and requirement mismatches before implementation depends on them.
  (Phase 1 completed: chat threads, CLI thread controls, PMA conversation history, and OpenAI-compatible thread-selection wording corrected.)

- [x] Prefer the existing canonical chat scoping model based on the `OpenAI-Project` header instead of inventing a parallel project-scoping path.
  (Thread scoping and explicit fresh-thread creation wording now preserve the OpenAI-compatible chat-completions request shape.)

- [x] Keep one canonical owner for each contract and cross-link from related documents instead of duplicating source-of-truth content.
  (Stable ownership now lives in the requirement files above and in [cynork_tui.md](../tech_specs/cynork_tui.md), [chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md), [openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md), [orchestrator.md](../tech_specs/orchestrator.md), [postgres_schema.md](../tech_specs/postgres_schema.md), and [cynode_pma.md](../tech_specs/cynode_pma.md).)

- [x] Use real tool, API, database, and system results during implementation and validation.
  (The canon now explicitly forbids guessed or simulated output in [REQ-AGENTS-0137](../requirements/agents.md#req-agents-0137) and [CYNAI.AGENTS.NoSimulatedOutput](../tech_specs/project_manager_agent.md#spec-cynai-agents-nosimulatedoutput).)

- [ ] Treat the first TUI rollout as a minimum viable product for user testing, not as the final complete chat experience.

- [ ] Defer attractive but non-blocking features when they would slow the first usable TUI milestone.

- [ ] Keep `POST /v1/chat/completions` as the broad compatibility baseline while adding `POST /v1/responses` as an additive OpenAI-compatible surface owned by the gateway contract.

- [ ] Do not pull forward non-TUI MVP Phase 2 work or worker-deployment follow-on work until the TUI can drive the full chat path end to end.

- [ ] Keep the TUI implementation testable through stable seams: backend APIs first, then reusable chat or controller logic, then the fullscreen UI and Python PTY harness on top.

- [ ] Prefer machine-detectable UI landmarks and explicit state transitions over visually clever but test-hostile behavior so Python automation can validate the TUI reliably.

- [ ] Do not defer test creation, test updates, or test execution to a later phase; each phase must land its own unit, integration, BDD, and E2E coverage updates as applicable.

- [ ] Treat `just ci` and `just e2e` as mandatory end-of-phase gates for every active phase; a phase stays open until both commands pass.

## Phase 0 Locked Scope Decisions

- [x] Deprecate `cynork shell` in docs and implementation planning, while keeping a temporary compatibility path during migration.
  (Normative in [cynork_cli.md](../tech_specs/cynork_cli.md) TUI Scope and Locked Decisions; MVP Scope updated.)

- [x] Ship `cynork tui` as the first full-screen entrypoint and keep `cynork chat` available as a compatibility path during rollout.

- [x] Keep the first implementation slice for TUI thread management limited to create, list, switch, and rename even though the stable docs now also define optional summary and archive contracts.
  (Implementation slice anchored by [REQ-CLIENT-0199](../requirements/client.md#req-client-0199), [REQ-CLIENT-0200](../requirements/client.md#req-client-0200), [REQ-USRGWY-0142](../requirements/usrgwy.md#req-usrgwy-0142), and [REQ-USRGWY-0144](../requirements/usrgwy.md#req-usrgwy-0144); optional later scope in [REQ-CLIENT-0201](../requirements/client.md#req-client-0201), [REQ-USRGWY-0143](../requirements/usrgwy.md#req-usrgwy-0143), and [REQ-USRGWY-0145](../requirements/usrgwy.md#req-usrgwy-0145).)

- [x] Keep the first implementation slice centered on thinking and tool activity while accepting the broader stable structured-turn model in the canonical docs.
  (Stable model now includes `download_ref` and normalized `attachment_ref` metadata in [CYNAI.USRGWY.ChatThreadsMessages.StructuredTurns](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-structuredturns), with download behavior in [CYNAI.USRGWY.ChatThreadsMessages.DownloadRefs](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-downloadrefs).)

- [x] Include dual interactive chat-surface support in the first TUI rollout scope: `POST /v1/chat/completions` and `POST /v1/responses`.

- [x] Treat the Python PTY or fullscreen TUI harness as part of the primary TUI milestone rather than as a late validation add-on.

- [x] Promote the single-binary worker proposal docs in this round, but defer implementation until after the first TUI rollout.
  (Locked in cynork_cli; Phase 8 still to promote draft into stable requirements/worker-node tech-spec updates.)

### Phase 0 Testing Gate

- [ ] When Phase 0 scope is reopened or adjusted, update the affected docs, feature files, and any scope-sensitive tests in the same phase rather than carrying test debt forward.

- [ ] Run `just docs-check` after the Phase 0 doc edits settle.

- [ ] Run any targeted validation needed for impacted behavior or fixtures before leaving the phase.

- [ ] Do not close the active Phase 0 slice until `just ci` passes.

- [ ] Do not close the active Phase 0 slice until `just e2e` passes.

## Phase 1 TUI-Enabling Spec Alignment

This phase resolves the source-of-truth issues that would otherwise make the TUI implementation unstable.

### Phase 1 Testing Gate

- [ ] For each Phase 1 contract or spec change, identify the affected BDD, integration, and E2E coverage before changing the source-of-truth docs.

- [ ] Create or update those tests in the same Phase 1 slice as the contract change; do not defer them to backend or TUI implementation phases.

- [ ] Run `just docs-check` after the spec and requirement edits settle.

- [ ] Run the targeted tests that prove the contract wording still matches executable behavior before leaving the slice.

- [ ] Do not close the active Phase 1 slice until `just ci` passes.

- [ ] Do not close the active Phase 1 slice until `just e2e` passes.

### Chat Thread Creation and Acquisition

- [x] Update [chat threads and messages](../tech_specs/chat_threads_and_messages.md) so `POST /v1/chat/threads` matches the intended current behavior.

- [x] Add an explicit thread-acquisition distinction between active-thread reuse and explicit fresh-thread creation in [chat threads and messages](../tech_specs/chat_threads_and_messages.md).

- [x] Align thread project scoping with [USRGWY requirements](../requirements/usrgwy.md) and the existing `OpenAI-Project` header model.

- [x] Retain the existing optional request-body `project_id` field for explicit thread creation and do not add a second parallel project-scoping contract.
  (Existing optional `project_id` in request body retained; no second parallel contract added.)

- [x] Add the missing requirement entry for explicit thread creation in [USRGWY requirements](../requirements/usrgwy.md).
  (REQ-USRGWY-0135 added and corrected so returned thread id is for retrieval and management, not for OpenAI-compatible chat-completions request routing.)

- [x] Add the missing client requirement entry for explicit fresh-thread controls in [CLIENT requirements](../requirements/client.md).
  (REQ-CLIENT-0181 added and corrected so subsequent chat-completion requests remain OpenAI-compatible.)

### CLI Thread Controls

- [x] Add a dedicated CLI thread-control Spec Item to [CLI Management App - Chat Command](../tech_specs/cli_management_app_commands_chat.md).
  (CYNAI.CLIENT.CliChatThreadControls.)

- [x] Specify startup fresh-thread behavior for `--thread-new`.

- [x] Specify in-session fresh-thread behavior for `/thread new`.

- [x] Specify the expected behavior for unknown `/thread` subcommands.

- [x] Specify how thread creation interacts with current project context and the `OpenAI-Project` header.

- [x] Correct explicit fresh-thread wording so neither the gateway nor the CLI requires any CyNodeAI-specific thread identifier on `POST /v1/chat/completions`.
  (Canonical thread and CLI chat specs now preserve the OpenAI-compatible request shape.)

### PMA Conversation History

- [x] Add a PMA conversation-history Spec Item to [CyNode PMA](../tech_specs/cynode_pma.md).
  (CYNAI.PMAGNT.ConversationHistory.)

- [x] Document that prior turns are preserved in system-context composition for the langchain-capable path.

- [x] Document that the final executor input remains the last user turn rather than being folded into the system block.

- [x] Add BDD coverage for multi-turn PMA chat history handling after the spec anchor is stable.
  (pma_chat_and_context.feature: Scenario "Responses-surface continuation preserves prior turns and keeps current user input distinct" with @spec_cynai_pmagnt_conversationhistory.)

### Rich Chat and Dual-Surface Contracts

- [x] Promote the OpenAI-compatible gateway contract so both `POST /v1/chat/completions` and `POST /v1/responses` are in scope for the TUI.

- [x] Add normalized assistant-output rules so one user prompt can yield one logical assistant turn with ordered structured parts.

- [x] Add stable structured-turn storage rules for visible text, hidden thinking, tool activity, and file references.

- [x] Add TUI transcript-rendering and generation-state rules so thinking, tool rows, and in-flight assistant updates are source-of-truth behavior rather than draft-only ideas.

- [x] Add orchestrator-side continuation-state requirements for `previous_response_id` so the TUI implementation has a stable backend contract for responses continuation.

## Phase 2 TUI MVP Spec Cut

This phase narrows the large TUI proposal down to the minimum first rollout that should be treated as in-scope.

### Phase 2 Testing Gate

- [ ] For each Phase 2 scope or UX decision, identify which BDD, PTY, and integration tests must be added or updated to lock the rollout boundary.

- [ ] Create or update those tests in the same Phase 2 slice as the scope decision so deferred items and required items stay machine-checkable.

- [ ] Run `just docs-check` after the MVP-scope doc updates settle.

- [ ] Run the targeted validation that proves the retained and deferred behaviors are covered before leaving the slice.

- [ ] Do not close the active Phase 2 slice until `just ci` passes.

- [ ] Do not close the active Phase 2 slice until `just e2e` passes.

### Must Land in the First TUI Rollout

- [x] Define `cynork tui` as the explicit first full-screen TUI entrypoint in the normative docs.
  ([cynork_tui.md](../tech_specs/cynork_tui.md), [cynork_cli.md](../tech_specs/cynork_cli.md) Required Top-Level Commands.)

- [x] Keep `cynork chat` available during the first rollout, either as the same surface or as a compatibility alias.

- [x] Define the minimum TUI layout contract with scrollback, composer, status bar, and an optional context pane.

- [x] Define a multi-line composer contract suitable for long prompts and slash-command use.

- [x] Define thread history behavior for create, list, switch, and rename.

- [x] Define status-bar fields for gateway, identity, project, model, and connection state.

- [x] Define in-session model and project switching behavior.

- [x] Define the first-pass dual chat-surface contract for TUI work so the client-side chat abstraction can support both `POST /v1/chat/completions` and `POST /v1/responses`.

- [x] Define auth-recovery behavior when login is missing or expires during a TUI session.

- [x] Define minimum slash-command parity with the existing CLI chat command surface.

- [x] Define non-interactive behavior so scripting mode remains stable and parseable.

- [x] Define transcript rendering for hidden-by-default thinking, ordered multi-item assistant turns, and distinct tool activity rows.

- [x] Define generation-state behavior for in-flight assistant updates and final reconciliation of partial output into one logical turn.

- [x] Promote to stable [cynork_tui.md](../tech_specs/cynork_tui.md): conversation history as formatted Markdown and user messages in scrollback with a distinct background.
  (See [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout).)

- [x] Promote to stable [cynork_tui.md](../tech_specs/cynork_tui.md): composer and scrollback keybindings -> Shift+Enter for newlines; Up/Down in composer for previously sent messages; Ctrl+C to cancel, successive Ctrl+C when idle to exit; Page Up/Page Down to scroll and load older history.
  (See [REQ-CLIENT-0204](../requirements/client.md#req-client-0204) and [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout).)

- [x] Promote to stable [cynork_tui.md](../tech_specs/cynork_tui.md): a visible assistant-turn status chip for in-flight work, a visible composer cursor, mouse-wheel transcript scrolling, and a composer hint for `/`, `@`, and `!`.
  (See [REQ-CLIENT-0185](../requirements/client.md#req-client-0185), [REQ-CLIENT-0204](../requirements/client.md#req-client-0204), [REQ-CLIENT-0205](../requirements/client.md#req-client-0205), and [REQ-CLIENT-0206](../requirements/client.md#req-client-0206).)

### Explicitly Deferred From the First TUI Rollout

- [x] Do not implement thread summary or archive in this round even though the stable canon now defines them.
  (Stable anchors: [CYNAI.USRGWY.ChatThreadsMessages.ThreadSummary](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-threadsummary) and [CYNAI.USRGWY.ChatThreadsMessages.Archive](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-archive).)

- [x] Do not implement assistant download-reference workflows in this round even though the stable canon now defines them.
  (Stable anchors: [REQ-USRGWY-0141](../requirements/usrgwy.md#req-usrgwy-0141), [REQ-CLIENT-0194](../requirements/client.md#req-client-0194), and [CYNAI.USRGWY.ChatThreadsMessages.DownloadRefs](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-downloadrefs).)

- [x] Do not implement full attachment upload UX in this round even though the stable canon now includes the contract.
  (Stable requirement anchors: [REQ-USRGWY-0140](../requirements/usrgwy.md#req-usrgwy-0140), [REQ-CLIENT-0198](../requirements/client.md#req-client-0198), [REQ-ORCHES-0167](../requirements/orches.md#req-orches-0167), [REQ-ORCHES-0168](../requirements/orches.md#req-orches-0168), [REQ-SCHEMA-0114](../requirements/schema.md#req-schema-0114), and [REQ-PMAGNT-0115](../requirements/pmagnt.md#req-pmagnt-0115).)
  (Stable spec anchors: [CYNAI.USRGWY.OpenAIChatApi.TextInput](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-textinput), [CYNAI.ORCHES.ChatFileUploadFlow](../tech_specs/orchestrator.md#spec-cynai-orches-chatfileuploadflow), [CYNAI.SCHEMA.ChatMessageAttachmentsTable](../tech_specs/postgres_schema.md#spec-cynai-schema-chatmessageattachmentstable), and [CYNAI.PMAGNT.ChatFileContext](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-chatfilecontext).)

- [x] Do not implement queued drafts or deferred send behavior in this round even though the stable TUI requirements and spec now define the optional contract.
  (See [REQ-CLIENT-0196](../requirements/client.md#req-client-0196) and [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout).)

- [x] Do not make bare `cynork` launch the TUI by default in this round.

- [x] Do not implement web-based login or SSO-specific flows in this round even though the stable contract now exists.
  (See [REQ-CLIENT-0191](../requirements/client.md#req-client-0191) and [CYNAI.CLIENT.CliWebLogin](../tech_specs/cynork_tui.md#spec-cynai-client-cliweblogin).)

## Phase 3 Backend Prerequisites Required for TUI Chat

This phase captures only the gateway, orchestrator, and data-path work required before the TUI can deliver the intended full chat experience.
The order inside this phase matters because later TUI work depends on these backend behaviors being stable.

### Phase 3 Testing Gate

- [ ] Before each Phase 3 backend change, identify the unit, integration, BDD, and E2E coverage that must move with that behavior.

- [ ] Create or update the backend-facing tests in the same slice as the implementation; do not leave contract or persistence coverage for a later cleanup pass.

- [ ] Run the targeted backend and contract checks continuously as each handler, storage path, or transport behavior lands.

- [ ] Do not close the active Phase 3 slice until `just ci` passes.

- [ ] Do not close the active Phase 3 slice until `just e2e` passes.

### Minimum Backend Surface

- [x] Implement `POST /v1/responses` on the gateway as an additive OpenAI-compatible interactive chat surface.

- [x] Implement retained response metadata and `previous_response_id` continuation handling without changing CyNodeAI thread ownership rules.

- [x] Implement normalized assistant-turn persistence so structured parts and canonical visible text are stored consistently for both interactive chat surfaces.
  (Assistant turns persisted with `response_id` in message metadata; same pipeline for completions and responses.)

- [x] Ensure chat secret redaction occurs before any chat data is persisted and before any inference handoff uses the content.

- [x] Implement explicit thread creation with the accepted contract.

- [x] Implement list-threads support with pagination and default recent-first ordering.

- [x] Implement get-thread and get-thread-messages behavior needed for thread history view and reload.

- [x] Implement patch-thread-title support for rename flows in this round.

### Required Backend Validation Before TUI Wiring

- [x] Verify both interactive chat surfaces produce coherent canonical visible text plus structured-turn persistence for the same logical assistant turn.

- [x] Verify the PMA path and direct-inference path both honor the same redaction, persistence, and normalized-output rules.

- [x] Verify thread retrieval and active-thread behavior are stable enough that the TUI can depend on them for history and fresh-thread controls.
  (Handlers and integration tests cover thread CRUD, list messages, patch title; `just ci` passes. See Progress Notes for database coverage exception.)

### Deferred Backend Work This Round

- [ ] Do not implement summary generation in this round.

- [ ] Do not implement archive or soft-delete in this round.

- [x] Do not implement download-ref contracts in this round even though the stable requirement and spec anchors now exist.
  (See [REQ-USRGWY-0141](../requirements/usrgwy.md#req-usrgwy-0141) and [CYNAI.USRGWY.ChatThreadsMessages.DownloadRefs](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-downloadrefs).)

- [x] Do not implement rich attachment storage contracts in this round even though the stable requirement and spec anchors now exist.
  (See [REQ-ORCHES-0167](../requirements/orches.md#req-orches-0167), [REQ-ORCHES-0168](../requirements/orches.md#req-orches-0168), [REQ-SCHEMA-0114](../requirements/schema.md#req-schema-0114), [REQ-PMAGNT-0115](../requirements/pmagnt.md#req-pmagnt-0115), [CYNAI.ORCHES.ChatFileUploadFlow](../tech_specs/orchestrator.md#spec-cynai-orches-chatfileuploadflow), [CYNAI.SCHEMA.ChatMessageAttachmentsTable](../tech_specs/postgres_schema.md#spec-cynai-schema-chatmessageattachmentstable), and [CYNAI.PMAGNT.ChatFileContext](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-chatfilecontext).)

- [x] Do not implement context compaction or automatic summarization in this round even though the stable gateway contract now exists.
  (See [REQ-USRGWY-0146](../requirements/usrgwy.md#req-usrgwy-0146), [REQ-USRGWY-0147](../requirements/usrgwy.md#req-usrgwy-0147), [CYNAI.USRGWY.ChatThreadsMessages.ContextSizeTracking](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-contextsizetracking), and [CYNAI.USRGWY.OpenAIChatApi.ContextCompaction](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-contextcompaction).)

- [x] Keep orchestrator-directed node-local inference context sizing and backend-env propagation as stable follow-on backend or deployment work unless the active TUI slice explicitly depends on it.
  (See [REQ-ORCHES-0169](../requirements/orches.md#req-orches-0169), [REQ-WORKER-0264](../requirements/worker.md#req-worker-0264), [REQ-PMAGNT-0116](../requirements/pmagnt.md#req-pmagnt-0116), and [CYNAI.ORCHES.InferenceBackendEnv](../tech_specs/orchestrator_inference_container_decision.md#spec-cynai-orches-inferencebackendenv).)

## Phase 4 Shared Chat Controller and Testable Seams

This phase defines the shared implementation seams that both the fullscreen TUI and the Python PTY harness depend on.

### Phase 4 Testing Gate

- [ ] Before extracting or refactoring shared chat seams, identify the unit and integration tests that must pin controller, transport, transcript, and state behavior.

- [ ] Create or update those seam-level tests in the same Phase 4 slice as the refactor so behavior does not become indirectly validated only by later TUI tests.

- [ ] Run the targeted controller and transport tests continuously while the seam work is landing.

- [ ] Do not close the active Phase 4 slice until `just ci` passes.

- [ ] Do not close the active Phase 4 slice until `just e2e` passes.

### Controller and Session State

- [x] Extract reusable chat-session or controller logic out of the CLI command layer so request shaping, slash handling, thread actions, and in-flight chat state are not owned only by the fullscreen UI.

- [x] Keep session state for model, project, thread, and auth recovery on instance-bound objects rather than package-level globals so parallel test execution and PTY automation remain reliable.

- [x] Define stable controller-facing actions and state transitions for send-message, fresh-thread, thread-switch, project-switch, model-switch, and auth-recovery flows.

### Transport and Rendering Seams

- [x] Add a client-side chat transport abstraction that can target both `POST /v1/chat/completions` and `POST /v1/responses`.

- [x] Keep transcript assembly and canonical visible-text projection reusable so non-interactive CLI paths, PTY automation, and the fullscreen TUI all validate the same logical turn behavior.

- [x] Add stable machine-detectable UI landmarks and reduced-noise test semantics where needed so PTY tests can assert on state transitions without depending on fragile redraw timing.

## Phase 5 `cynork` TUI and Python PTY Harness Implementation

This phase delivers the first usable full-screen TUI slice and the Python PTY harness together.
They must be developed in tandem so each validates the other as behavior lands, with minimal human intervention.

### Phase 5 Testing Gate

- [ ] Before each TUI slice, identify the matching Go tests, BDD coverage, and Python PTY or E2E assertions that must land with the UI behavior.

- [ ] Create or update the PTY harness and the corresponding tests in the same slice as the TUI behavior; do not postpone harness work until after the UI "mostly works."

- [ ] Run targeted TUI and PTY validation continuously during the phase as each command, rendering rule, or state transition lands.

- [ ] Do not close the active Phase 5 slice until `just ci` passes.

- [ ] Do not close the active Phase 5 slice until `just e2e` passes.

### Entry Point and Core Wiring

- [x] Add the `cynork tui` command path.

- [x] Preserve stable non-interactive chat behavior for scripting and piping use cases.
  (`cynork chat --message` remains one-shot, and non-TTY `cynork chat` still uses the line-oriented scanner path.)

- [x] Use `POST /v1/chat/completions` as the default interactive chat path in the first implementation while preserving full support for `POST /v1/responses` behind the same chat abstraction and test harness.
  (Session uses CompletionsTransport by default; TUI uses that session.)

### Core TUI Experience

- [x] Implement the full-screen layout with scrollback, composer, status bar, and a togglable context pane.

- [x] Implement a multi-line composer with clear send semantics.
  (Enter = send, Shift+Enter = newline; status bar hint; visible lines capped at 5.)

- [x] Implement composer input history: Up/Down in composer cycles through previously sent messages.

- [ ] Implement Ctrl+C semantics: cancel current operation when no selection; when selection active, copy; successive Ctrl+C when idle exits cleanly (Cursor-agent style).

- [ ] Implement scrollback navigation (Page Up/Page Down), search, and copy behavior; load older message history when user scrolls back past loaded content.

- [x] Implement status-bar rendering for gateway, auth identity, project, model, and connectivity state.
  (Status bar shows landmark, gateway, project, model, thread; "Enter send, Shift+Enter newline" hint. Auth identity and connectivity are minimal for first slice.)

- [ ] Implement local slash-command discovery and execution within the TUI.

- [x] Implement the canonical in-flight assistant-turn indicator as a distinct status chip with spinner and required labels.
  (LandmarkAssistantInFlight shown in status bar when Loading; landmarks are machine-detectable for PTY.)

- [ ] Implement the composer discoverability hint for `/ commands`, `@ files`, and `! shell`.

- [ ] Implement visible composer-cursor behavior and mouse-wheel transcript scrolling that does not alter composer-history recall.

- [ ] Implement transcript rendering for visible text, hidden-by-default thinking, tool activity rows, and ordered multi-item assistant turns; user messages in scrollback with distinct background.

- [ ] Implement in-flight generation handling so one assistant turn is updated progressively and reconciled cleanly on completion.

### Thread and Session UX

- [x] Implement thread list and thread switching.
  (Gateway `ListChatThreads`, `PatchThreadTitle`; session `CurrentThreadID`, `ListThreads`, `SetCurrentThreadID`; TUI `/thread list`, `/thread switch <id>`.)

- [x] Implement fresh-thread creation from startup controls and in-session controls.
  (Both `cynork tui --thread-new` and in-session `/thread new` now create a new gateway thread and set `CurrentThreadID`.)

- [x] Implement thread rename.
  (TUI `/thread rename <title>`; session `PatchThreadTitle`; gateway PATCH thread.)

- [ ] Implement project-context switching in-session.

- [ ] Implement model selection in-session.

### Auth Recovery

- [ ] Implement startup login recovery when a usable token is missing.

- [ ] Implement in-session login recovery when the gateway returns an auth failure.

- [ ] Ensure passwords and tokens are never echoed, persisted in transcript history, or written to temporary UI state unsafely.

### Python PTY Harness

- [x] Add Python PTY process launch and teardown helpers for the fullscreen TUI.
  (scripts/test_scripts/tui_pty_harness.py using pexpect; scripts/requirements-e2e.txt.)

- [x] Add fixed terminal sizing support for PTY-driven tests so layout assertions are reproducible.
  (pexpect.spawn dimensions=(rows, cols); default 80x24.)

- [x] Add key event injection helpers for common actions such as send, fresh-thread, thread switch, project or model switch, and exit.
  (send_keys: enter, ctrl+c, ctrl+d, literal text.)

- [x] Add screen or buffer capture helpers and semantic wait utilities for state transitions such as prompt-ready, assistant-in-flight, response-complete, thread-switched, and auth-recovery-ready.
  (read_until_landmark, wait_for_prompt_ready, capture_screen; landmarks match cynork/internal/chat/landmarks.go.)

- [x] Add stable Python assertions around semantic UI landmarks instead of exact model wording or brittle full-frame diffs.
  (E2E tests assert on LANDMARK_* and "Threads" header; e2e_198_tui_pty.py.)

### Tandem TUI and Harness Validation

- [x] Validate send and receive behavior through the PTY harness as the composer and transcript rendering land.
  (`test_tui_pty_send_receive_round_trip` waits for prompt-ready after a real send.)

- [ ] Validate thread create, list, switch, and rename behavior through the PTY harness as soon as the corresponding TUI flows exist.
  (Current PTY coverage exercises thread list only; create, switch, and rename still need fullscreen validation.)

- [ ] Validate hidden-thinking, ordered assistant output, and tool-activity rendering through the PTY harness as transcript rendering lands.

- [ ] Validate startup and in-session auth recovery through the PTY harness as soon as the TUI flow exists.

### TUI Chat-Complete Exit for Implementation

- [ ] Confirm a user can send a prompt, receive a response, see thread state, observe project and model context, and continue the same conversation without leaving the TUI.

- [ ] Confirm a user can start a fresh thread and continue chatting in the new thread from the TUI.

- [ ] Confirm the TUI remains coherent whether the backend path is chat-completions or responses.

## Phase 6 TUI Validation and BDD

This phase turns the promoted chat and TUI contract into executable behavior checks and PTY-driven validation.

### Phase 6 Testing Gate

- [ ] For every Phase 6 coverage addition, identify the exact behavior gap first, then add or update the BDD and PTY or E2E checks in the same slice.

- [ ] Run `just docs-check` after feature-file or validation-doc edits settle.

- [ ] Run `just test-bdd` in the same slice as the related feature or behavior updates; do not defer BDD execution.

- [ ] Run targeted `just e2e` coverage while the TUI validation work is landing, then run full `just e2e` as the phase-closing gate.

- [ ] Do not close the active Phase 6 slice until `just ci` passes.

- [ ] Do not close the active Phase 6 slice until `just e2e` passes.

- [x] Update [Cynork chat feature](../../features/cynork/cynork_chat.feature) to cover the accepted first-rollout chat and thread behaviors.

- [x] Keep [Cynork shell feature](../../features/cynork/cynork_shell.feature) as compatibility coverage during this round and do not retire it yet.

- [x] Add OpenAI-compatible thread-creation coverage in [chat_openai_compatible.feature](../../features/e2e/chat_openai_compatible.feature) and BDD coverage for `--thread-new` before the first completion request.

- [x] Add BDD coverage for `/thread new` during an active session.

- [x] Add BDD coverage for unknown `/thread` subcommands that keep the session alive.

- [x] Add behavior-spec coverage for thread history navigation and thread rename.
  ([cynork_tui.feature](../../features/cynork/cynork_tui.feature) and [chat_thread_management.feature](../../features/orchestrator/chat_thread_management.feature).)

- [ ] Add BDD coverage for startup and in-session auth recovery.

- [ ] Add coverage for both supported interactive chat surfaces so TUI and gateway behavior stays aligned across `POST /v1/chat/completions` and `POST /v1/responses`.

- [x] Add behavior-spec coverage for structured-turn rendering expectations that matter to the TUI, especially hidden thinking, ordered assistant output, and tool activity.
  ([cynork_tui.feature](../../features/cynork/cynork_tui.feature) and [chat_thread_management.feature](../../features/orchestrator/chat_thread_management.feature).)

- [x] Add TUI behavior-spec coverage for the working indicator, composer hint, visible cursor, mouse-wheel transcript scrolling, queued drafts, auth recovery, and web login.
  ([cynork_tui.feature](../../features/cynork/cynork_tui.feature).)

- [x] Add backend-facing behavior-spec coverage for accepted file-context replay, PMA file-context consumption, PMA node-local backend-env consumption, and worker node-manager backend-env propagation.
  ([chat_thread_management.feature](../../features/orchestrator/chat_thread_management.feature), [pma_chat_file_context.feature](../../features/agents/pma_chat_file_context.feature), [pma_chat_and_context.feature](../../features/agents/pma_chat_and_context.feature), and [node_manager_config_startup.feature](../../features/worker_node/node_manager_config_startup.feature).)

- [x] Add Python E2E coverage for the fullscreen TUI flows that are now required for the primary milestone.
  (e2e_198_tui_pty.py: prompt-ready, exit via ctrl+c, thread list, and send/receive round-trip; skip when pexpect not installed.)

- [ ] Use the Python PTY harness continuously during TUI development; each TUI slice must add or update its PTY coverage before the slice is considered done.

- [ ] Make interactive `cynork chat` invoke the same fullscreen TUI entry flow as `cynork tui`, while keeping `cynork chat --message` and non-interactive usage line-oriented and parseable.

- [ ] Run `just docs-check` after the docs changes settle inside the same phase as the doc or feature update.

- [ ] Run `just test-bdd` after the relevant feature and behavior changes land in the same phase, not in a later cleanup pass.

- [ ] Run the Python E2E suite or targeted Python PTY subset during implementation, then run full `just e2e` as part of the TUI acceptance gate.

- [ ] Run `just ci` before considering the phase or round complete.

## Phase 7 Remaining MVP Phase 2 Work After TUI MVP

This phase resumes non-TUI MVP Phase 2 implementation only after the first usable TUI path is stable.

### Phase 7 Testing Gate

- [ ] Before each resumed MVP Phase 2 slice, identify the unit, integration, BDD, and E2E coverage that must change with it.

- [ ] Create or update those tests in the same Phase 7 slice rather than carrying test debt into follow-on planning.

- [ ] Run the targeted validation for each resumed MVP behavior while that slice is active.

- [ ] Do not close the active Phase 7 slice until `just ci` passes.

- [ ] Do not close the active Phase 7 slice until `just e2e` passes.

- [ ] Resume the remaining MVP Phase 2 MCP tool slices beyond the currently implemented `db.preference.*` set.

- [ ] Finish the remaining LangGraph graph-node work identified in [MVP implementation plan](../mvp_plan.md).

- [ ] Finish the verification-loop work needed for PMA => Project Analyst => result review flows.

- [ ] Close the known chat/runtime drifts tracked in [MVP implementation plan](../mvp_plan.md), especially bounded wait, retry behavior, and other user-visible chat reliability gaps.

- [ ] Keep all currently deferred TUI features deferred for this round and record any pull-forward candidates only as input for the next planning cycle.

## Phase 8 Worker Deployment Simplification Docs

This phase keeps the worker-deployment follow-on docs aligned without mixing them into the active TUI implementation slice.

### Phase 8 Testing Gate

- [ ] For each Phase 8 doc promotion or topology clarification, update any affected docs, feature files, and validation coverage in the same slice.

- [ ] Run `just docs-check` after the worker-deployment doc edits settle.

- [ ] Run any targeted validation needed for impacted examples, feature coverage, or deployment workflows before leaving the slice.

- [ ] Do not close the active Phase 8 slice until `just ci` passes.

- [ ] Do not close the active Phase 8 slice until `just e2e` passes.

- [x] Promote the single-binary node manager and worker API proposal into stable requirements and worker-node tech-spec updates (completed in a prior round; see [worker.md](../requirements/worker.md) and [worker_node.md](../tech_specs/worker_node.md)).

- [ ] Ensure the promoted worker deployment docs clearly distinguish normative deployment topology decisions from deferred implementation work.

- [x] Keep the promoted worker deployment follow-on docs aligned with the new orchestrator-directed local-inference sizing contract and matching feature coverage.
  (See [orchestrator_inference_container_decision.md](../tech_specs/orchestrator_inference_container_decision.md), [worker_node.md](../tech_specs/worker_node.md), [worker_node_payloads.md](../tech_specs/worker_node_payloads.md), [pma_chat_and_context.feature](../../features/agents/pma_chat_and_context.feature), and [node_manager_config_startup.feature](../../features/worker_node/node_manager_config_startup.feature).)

## Recommended Execution Order

- [x] First, resolve the chat-thread contract mismatch and the CLI thread-control docs.

- [x] Second, promote the PMA conversation-history clarification.

- [x] Third, promote the rich-chat and dual-surface source-of-truth work required for the TUI chat experience.
  (`openai_compatible_chat_api.md`, `chat_threads_and_messages.md`, `cynork_tui.md`, `cynode_pma.md`, and related requirements updated.)

- [x] Fourth, cut the TUI proposal down to the minimum first-rollout normative scope.
  (`cynork_tui.md` and Phase 0/2 checkboxes completed.)

- [x] Fifth, implement the backend chat prerequisites in this order: `POST /v1/responses`, continuation metadata, normalized assistant-turn persistence, redaction-before-persistence guarantees, then thread retrieval and rename support.

- [x] Sixth, extract or define the reusable chat or controller seams that both the TUI and Python PTY harness will depend on.

- [ ] Seventh, implement the `cynork` TUI and the Python PTY harness in tandem, using each to validate the other as behavior lands.

- [ ] Eighth, update feature coverage and run docs, BDD, Python E2E, and CI validation for the full TUI chat path.

- [ ] Ninth, promote the single-binary worker deployment docs while keeping implementation deferred until after the first TUI rollout.

- [ ] Tenth, return to the remaining MVP Phase 2 orchestration and MCP work only after the TUI can drive realistic stack testing.

## Exit Criteria for This Round

- [x] The TUI-first docs are promoted far enough that implementation does not depend on unresolved source-of-truth conflicts.

- [ ] A user can log in, create or switch threads, chat in a multi-line TUI, and observe project and model context.

- [ ] The fullscreen TUI can be driven end to end from the Python test scripts with minimal human intervention.

- [ ] The first TUI rollout is covered by updated BDD, Python PTY validation, and passes repository validation.

- [ ] The remaining MVP Phase 2 work is clearly separated as the next follow-on implementation stage rather than mixed into the TUI-first milestone.

- [ ] Each active phase closed with its required same-phase test creation or updates completed and both `just ci` and `just e2e` passing.

## Progress Notes

- **2026-03-12:** Phase 1 (TUI-enabling spec alignment) completed: chat threads and messages updated (active vs explicit thread, POST /v1/chat/threads), REQ-USRGWY-0135 and REQ-CLIENT-0181 added, CYNAI.CLIENT.CliChatThreadControls and CYNAI.PMAGNT.ConversationHistory added.
  Follow-up correction applied the same day: explicit fresh-thread creation remains a separate CyNodeAI Data REST capability, but subsequent `POST /v1/chat/completions` requests remain OpenAI-compatible and do not require any CyNodeAI-specific thread identifier.
  Execution order steps one and two done.

- **2026-03-12:** Rich-chat and dual-surface spec promotion completed for the TUI track: the stable docs now cover both `POST /v1/chat/completions` and `POST /v1/responses`, structured turns, hidden thinking, ordered assistant output, TUI transcript rendering, generation state, orchestrator continuation state, and the full end-to-end chat flow.

- **2026-03-12:** Phase 0 locked scope decisions and Phase 2 TUI MVP spec cut completed: cynork_cli.md updated (TUI scope and locked decisions, deprecate shell, cynork tui + chat in Required Commands, MVP Scope); `cynork_tui.md` defines the minimum layout, composer, thread history, status bar, transcript rendering, generation state, slash parity, auth recovery, and explicitly deferred list.
  Execution order steps three and four done.
  Next: Phase 3 backend prerequisites, Phase 4 shared controller and test seams, Phase 5 TUI plus Python PTY harness implementation, Phase 6 validation, then follow-on docs and non-TUI MVP work.

- **2026-03-12:** Phase 3 backend prerequisites completed: gateway exposes `POST /v1/responses`, thread CRUD, redaction before persistence, response metadata and `previous_response_id`, normalized assistant-turn persistence with `response_id` in metadata.
  Handlers and database coverage >=90%.
  `GetThreadByResponseID` reimplemented with GORM (jsonb containment + GetChatThreadByID); no Raw SQL; integration passes.
  Execution order step five done.
  Next: Phase 4 shared chat controller and testable seams.

- **2026-03-12:** Phase 4 shared chat controller and testable seams completed: added `cynork/internal/chat` with `Session`, `ChatTransport` (CompletionsTransport, ResponsesTransport), `AssistantTurn`, and machine-detectable landmarks; refactored `cmd/chat.go` and `cmd/chat_slash.go` to use instance-bound session; gateway client now has `ResponsesWithOptions`; `just ci` passes.
  Next: Phase 5 cynork TUI and Python PTY harness implementation.

- **2026-03-12:** Phase 5 first slice: added `cynork tui` command and `cynork/internal/tui` Bubble Tea TUI with scrollback, single-line composer (Enter to send), status bar (gateway, project, model, landmarks), and chat session wiring.
  Interactive `cynork chat` still uses line-oriented flow; TUI is entrypoint via `cynork tui`.
  Tests and coverage for cmd and tui added; `tuiRunProgram` hook for tests. `just ci` passes.

- **2026-03-12:** Execution order step Sixth marked complete (Phase 4 delivered the shared controller and testable seams).
  `just test-bdd` run and passed; prior failure was an ephemeral port conflict from another agent running tests in parallel.

- **2026-03-12:** Phase 5 slice (multi-line composer, thread UX): Multi-line composer with Enter=send and Shift+Enter=newline, status bar hint, and 5-line cap.
  Gateway `ListChatThreads` (GET with pagination, optional OpenAI-Project) and `PatchThreadTitle` (PATCH).
  Session `CurrentThreadID`, `NewThread()`, `SetCurrentThreadID`, `ListThreads`, `PatchThreadTitle`.
  TUI `/thread new`, `/thread list`, `/thread switch <id>`, `/thread rename <title>`, status bar shows current thread (truncated).
  Tests and coverage for gateway, session, and tui; lint fixes (evalOrder, httpNoBody, goconst, dupl).
  Re-run `just ci` if a previous run failed with "parallel golangci-lint is running".
  Follow-up: Coverage raised to >=90% for cynork/internal/tui and cynork/internal/gateway (tests for threadListCmd/threadRenameCmd nil session and list-with-items, ListTasks/GetTask normalizeTaskResponse, PatchThreadTitle Do failure).
  Lint: dupl (taskGetHandler), goconst (threadListHeader, inputThreadList), hugeParam (task by pointer).
  `just ci` passes.

- **2026-03-12:** Phase 5 Python PTY harness: Replaced custom PTY code with pexpect.
  Added scripts/requirements-e2e.txt (pexpect>=4.8), scripts/test_scripts/tui_pty_harness.py (TuiPtySession, landmarks, send_keys, read_until_landmark, wait_for_prompt_ready, capture_screen), and scripts/test_scripts/e2e_198_tui_pty.py (prompt-ready, exit via ctrl+c, thread list, send/receive round-trip; skip when pexpect not installed).
  Tag tui_pty added to check_e2e_tags.
  Harness asserts on semantic landmarks; E2E tests run with `just e2e` and skip gracefully without pexpect.
  Doc/justfile follow-up: root `just venv` now installs both scripts/requirements-lint.txt and scripts/requirements-e2e.txt so one .venv supports lint and E2E (including TUI PTY); development_setup and scripts/README updated.

- **2026-03-12:** Plan aligned with updated [Cynork TUI draft proposal](../draft_specs/cynork_tui_spec_proposal.md).
  Draft now uses non-overlapping REQ IDs (USRGWY 0142--0145, CLIENT 0199--0204) to avoid conflicts with stable 0135--0138 and 0181--0186.
  Draft adds: conversation history as formatted Markdown; user messages in scrollback with distinct background; Shift+Enter for newlines; Up/Down in composer for previously sent messages; Ctrl+C cancel and successive Ctrl+C exit; Page Up/Page Down scrollback with load-older-history when scrolling past loaded content; `! command` and @ shorthand (latter deferred in first rollout).
  Phase 2 "Must Land" and Phase 5 "Core TUI Experience" updated to reflect these behaviors; broken link to single-binary proposal removed from Purpose (file not in repo); Phase 8 wording made link-free.

- **2026-03-12:** Draft proposal extended with backend file upload/retrieval: section 3.4 (REQ-ORCHES-0167/0168, REQ-SCHEMA-0114, REQ-PMAGNT-0115) and section 4.8 (orchestrator chat file flow, file upload storage, PMA chat file context).
  When @ shorthand and gateway upload are promoted, orchestrator must resolve file refs and pass to PMA; schema must persist uploads scoped to user/thread; PMA must include file content in LLM requests.
  Plan Phase 2 deferred and Phase 3 deferred backend bullets updated to reference the draft.

- **2026-03-13:** Re-reviewed against the current repo state.
  `cynork tui --thread-new` is now wired, composer input history (Up/Down recall) is implemented with tests, and the PTY suite now covers a basic send/receive round-trip in addition to prompt-ready, exit, and thread-list flows.
  Interactive `cynork chat` still uses the line-oriented path rather than the fullscreen TUI entry flow, and richer TUI work remains open for Ctrl+C cancel semantics, scrollback navigation and search, identity/connectivity status fields, broader slash-command parity, auth recovery, formatted Markdown transcript rendering, and distinct user-message styling.

- **2026-03-13:** The stable source-of-truth docs and behavior specs were expanded beyond the original TUI-first MVP cut.
  Stable requirements now include:
  [REQ-USRGWY-0139](../requirements/usrgwy.md#req-usrgwy-0139) through [REQ-USRGWY-0147](../requirements/usrgwy.md#req-usrgwy-0147), [REQ-CLIENT-0187](../requirements/client.md#req-client-0187) through [REQ-CLIENT-0207](../requirements/client.md#req-client-0207), and [REQ-ORCHES-0167](../requirements/orches.md#req-orches-0167) through [REQ-ORCHES-0169](../requirements/orches.md#req-orches-0169).
  It also includes [REQ-SCHEMA-0114](../requirements/schema.md#req-schema-0114), [REQ-PMAGNT-0115](../requirements/pmagnt.md#req-pmagnt-0115), [REQ-PMAGNT-0116](../requirements/pmagnt.md#req-pmagnt-0116), and [REQ-WORKER-0264](../requirements/worker.md#req-worker-0264).
  Stable tech-spec anchors now cover the same scope in [cynork_tui.md](../tech_specs/cynork_tui.md), [chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md), [openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md), [orchestrator.md](../tech_specs/orchestrator.md), and [orchestrator_inference_container_decision.md](../tech_specs/orchestrator_inference_container_decision.md).
  They also cover [postgres_schema.md](../tech_specs/postgres_schema.md), [cynode_pma.md](../tech_specs/cynode_pma.md), [worker_node.md](../tech_specs/worker_node.md), and [worker_node_payloads.md](../tech_specs/worker_node_payloads.md).
  Feature coverage was also expanded in [chat_openai_compatible.feature](../../features/e2e/chat_openai_compatible.feature), [cynork_chat.feature](../../features/cynork/cynork_chat.feature), [cynork_shell.feature](../../features/cynork/cynork_shell.feature), and [cynork_tui.feature](../../features/cynork/cynork_tui.feature).
  It also expanded in [chat_thread_management.feature](../../features/orchestrator/chat_thread_management.feature), [pma_chat_file_context.feature](../../features/agents/pma_chat_file_context.feature), [pma_chat_and_context.feature](../../features/agents/pma_chat_and_context.feature), and [node_manager_config_startup.feature](../../features/worker_node/node_manager_config_startup.feature).
  Planning implication: the canon is now broader than the intended first implementation slice, so the remaining execution work must treat summary or archive, downloads, `@` upload flow, queued drafts, web login, context compaction, and orchestrator-directed backend-env sizing or propagation as stable but intentionally deferred implementation items unless the round scope is reopened.

- **2026-03-13:** Canonical docs were also normalized to remove rollout-phase wording from stable requirements, tech specs, and feature files where that wording had leaked out of planning documents.
  The TUI design mockup was retained with the canonical TUI spec under `docs/tech_specs/images/`, and the shell docs or feature coverage were tightened to treat `cynork shell` as compatibility rather than the primary interactive surface.
  The implementation guidance and agent canon now also explicitly prohibit guessed or simulated tool or system output.

- **2026-03-13:** CI and phase 0-2 follow-up: fixed `TestNewServer_SuccessWithDefaults` (set `InternalListenAddr` explicitly so empty means no internal listener).
  Added workerapiserver tests for managed-service proxy (method not allowed, auth, service not found, invalid body, success, upstream error, path normalization, empty bearer) and for embedded telemetry (node:info, node:stats, containers, logs with and without store).
  Implemented telemetry routes on the embedded worker API (`registerEmbedTelemetryHandlers`: node:info, node:stats, containers, logs) so e2e worker telemetry tests get 200 when node-manager is running.
  Fixed e2e logout assertion to accept `logged_out=true`.
  Reached workerapiserver coverage >=90% via `TestServer_Shutdown_ReturnsErrorWhenBusy`, `TestServer_Shutdown_InternalBusy`, and lint fixes (table-driven telemetry tests, nolint for integration test).
  Run `just e2e` with dev stack (node-manager with embedded API) to validate worker telemetry and phase 0-2 gate.

- **2026-03-13:** Plan updated for completed implementation work.
  Phase 1: BDD coverage for multi-turn PMA conversation history is in place (pma_chat_and_context.feature, @spec_cynai_pmagnt_conversationhistory).
  Phase 5: Default interactive chat path is POST /v1/chat/completions (Session/CompletionsTransport).
  Phase 5: Status bar implemented with landmark, gateway, project, model, thread, and composer hint (Enter send, Shift+Enter newline); auth identity and connectivity minimal for first slice.
  Phase 5: In-flight assistant-turn indicator implemented via LandmarkAssistantInFlight in status bar when Loading.
  Remaining Phase 5 open: Ctrl+C cancel/copy/successive-exit semantics, scrollback navigation (Page Up/Down, search, load older), composer hint for `/` `@` `!`, visible cursor and mouse-wheel scroll, transcript rendering for thinking/tool rows and distinct user background, progressive in-flight generation, project/model in-session switch, auth recovery.
