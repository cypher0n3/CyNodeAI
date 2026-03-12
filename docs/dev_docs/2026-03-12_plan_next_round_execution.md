# Next Round Execution Plan

## Purpose

This temporary plan tracks the next round of work for CyNodeAI as of 2026-03-12.

The priority for this round is the `cynork` TUI because it is the fastest path to realistic user-level validation of the current stack.

This plan is derived from [MVP scope](../mvp.md), [MVP implementation plan](../mvp_plan.md), [Cynork TUI draft proposal](../draft_specs/cynork_tui_spec_proposal.md), [Chat threads, PMA context, and backend env follow-ups](../draft_specs/chat_threads_pma_context_and_backend_env_followups.md), and [Single binary node manager and worker API proposal](../draft_specs/single_binary_node_manager_worker_api_proposal.md).

## Round Goals

- [ ] Promote the minimum set of chat and thread contracts required to support a usable TUI-first workflow.

- [ ] Define and lock a minimal first-rollout `cynork` TUI scope that is sufficient for end-to-end stack testing.

- [ ] Implement the first TUI slice and its backend dependencies in a way that preserves existing CLI compatibility where needed.

- [ ] Resume the remaining MVP Phase 2 work after the TUI-first validation loop is in place.

- [ ] Keep the single-binary worker proposal visible as a follow-on architecture track unless it is explicitly pulled into this round.

## Execution Principles

- [ ] Resolve spec and requirement mismatches before implementation depends on them.

- [ ] Prefer the existing canonical chat scoping model based on the `OpenAI-Project` header instead of inventing a parallel project-scoping path.

- [ ] Keep one canonical owner for each contract and cross-link from related documents instead of duplicating source-of-truth content.

- [ ] Treat the first TUI rollout as a minimum viable product for user testing, not as the final complete chat experience.

- [ ] Defer attractive but non-blocking features when they would slow the first usable TUI milestone.

## Phase 0 Locked Scope Decisions

- [ ] Deprecate `cynork shell` in docs and implementation planning, while keeping a temporary compatibility path during migration.

- [ ] Ship `cynork tui` as the first full-screen entrypoint and keep `cynork chat` available as a compatibility path during rollout.

- [ ] Keep the first TUI thread-management slice limited to create, list, switch, and rename.

- [ ] Include a minimal structured chat `parts` model for thinking and tool activity only.

- [ ] Promote the single-binary worker proposal docs in this round, but defer implementation until after the first TUI rollout.

## Phase 1 TUI-Enabling Spec Alignment

This phase resolves the source-of-truth issues that would otherwise make the TUI implementation unstable.

### Chat Thread Creation and Acquisition

- [ ] Update [chat threads and messages](../tech_specs/chat_threads_and_messages.md) so `POST /v1/chat/threads` matches the intended current behavior.

- [ ] Add an explicit thread-acquisition distinction between active-thread reuse and explicit fresh-thread creation in [chat threads and messages](../tech_specs/chat_threads_and_messages.md).

- [ ] Align thread project scoping with [USRGWY requirements](../requirements/usrgwy.md) and the existing `OpenAI-Project` header model.

- [ ] Avoid introducing a separate request-body `project_id` contract for Phase 1 explicit thread creation unless a new requirement explicitly demands it.

- [ ] Add the missing requirement entry for explicit thread creation in [USRGWY requirements](../requirements/usrgwy.md).

- [ ] Add the missing client requirement entry for explicit fresh-thread controls in [CLIENT requirements](../requirements/client.md).

### CLI Thread Controls

- [ ] Add a dedicated CLI thread-control Spec Item to [CLI Management App - Chat Command](../tech_specs/cli_management_app_commands_chat.md).

- [ ] Specify startup fresh-thread behavior for `--thread-new`.

- [ ] Specify in-session fresh-thread behavior for `/thread new`.

- [ ] Specify the expected behavior for unknown `/thread` subcommands.

- [ ] Specify how thread creation interacts with current project context and the `OpenAI-Project` header.

### PMA Conversation History

- [ ] Add a PMA conversation-history Spec Item to [CyNode PMA](../tech_specs/cynode_pma.md).

- [ ] Document that prior turns are preserved in system-context composition for the langchain-capable path.

- [ ] Document that the final executor input remains the last user turn rather than being folded into the system block.

- [ ] Add BDD coverage for multi-turn PMA chat history handling after the spec anchor is stable.

## Phase 2 TUI MVP Spec Cut

This phase narrows the large TUI proposal down to the minimum first rollout that should be treated as in-scope.

### Must Land in the First TUI Rollout

- [ ] Define `cynork tui` as the explicit first full-screen TUI entrypoint in the normative docs.

- [ ] Keep `cynork chat` available during the first rollout, either as the same surface or as a compatibility alias.

- [ ] Define the minimum TUI layout contract with scrollback, composer, status bar, and an optional context pane.

- [ ] Define a multi-line composer contract suitable for long prompts and slash-command use.

- [ ] Define thread history behavior for create, list, switch, and rename.

- [ ] Define status-bar fields for gateway, identity, project, model, and connection state.

- [ ] Define in-session model and project switching behavior.

- [ ] Define auth-recovery behavior when login is missing or expires during a TUI session.

- [ ] Define minimum slash-command parity with the existing CLI chat command surface.

- [ ] Define non-interactive behavior so scripting mode remains stable and parseable.

### Explicitly Deferred From the First TUI Rollout

- [ ] Defer thread summary and archive unless they become necessary for the first testing pass.

- [ ] Defer assistant download references unless file-output workflows become a first-round testing requirement.

- [ ] Defer full attachment upload UX unless the upload contract is finalized this round.

- [ ] Defer queued drafts and deferred send behavior until after the first TUI rollout is usable.

- [ ] Defer making bare `cynork` launch the TUI by default until the explicit `cynork tui` rollout is stable.

- [ ] Defer web-based login and SSO-specific flow work unless enterprise auth becomes an immediate blocker.

## Phase 3 Backend Work Required for TUI MVP

This phase captures only the backend work required to support the first TUI rollout.

### Minimum Backend Surface

- [ ] Implement explicit thread creation with the accepted contract.

- [ ] Implement list-threads support with pagination and default recent-first ordering.

- [ ] Implement get-thread and get-thread-messages behavior needed for thread history view and reload.

- [ ] Implement patch-thread-title support for rename flows if the title-update requirement is accepted for this round.

### Backend Work to Defer Unless TUI Scope Expands

- [ ] Defer summary generation if thread title plus recency is sufficient for first-pass history navigation.

- [ ] Defer archive and soft-delete if the first TUI rollout only needs active recent conversations.

- [ ] Defer download-ref contracts unless the TUI first pass must handle assistant-produced files.

- [ ] Defer rich attachment storage contracts unless attachment support is promoted into the first TUI slice.

- [ ] Defer context compaction and automatic summarization unless long-thread behavior becomes a blocking usability issue during validation.

## Phase 4 `cynork` TUI Implementation

This phase delivers the first usable full-screen TUI slice after the required docs and backend contracts are ready.

### Entry Point and Core Wiring

- [ ] Add the `cynork tui` command path.

- [ ] Decide whether `cynork chat` reuses the same TUI code path in the first rollout.

- [ ] Preserve stable non-interactive chat behavior for scripting and piping use cases.

### Core TUI Experience

- [ ] Implement the full-screen layout with scrollback, composer, status bar, and optional context pane.

- [ ] Implement a multi-line composer with clear send semantics.

- [ ] Implement scrollback navigation, search, and copy behavior.

- [ ] Implement status-bar rendering for gateway, auth identity, project, model, and connectivity state.

- [ ] Implement local slash-command discovery and execution within the TUI.

### Thread and Session UX

- [ ] Implement thread list and thread switching.

- [ ] Implement fresh-thread creation from startup controls and in-session controls.

- [ ] Implement thread rename.

- [ ] Implement project-context switching in-session.

- [ ] Implement model selection in-session.

### Auth Recovery

- [ ] Implement startup login recovery when a usable token is missing.

- [ ] Implement in-session login recovery when the gateway returns an auth failure.

- [ ] Ensure passwords and tokens are never echoed, persisted in transcript history, or written to temporary UI state unsafely.

## Phase 5 TUI Validation and BDD

- [ ] Update [Cynork chat feature](../../features/cynork/cynork_chat.feature) to cover the accepted first-rollout TUI behaviors.

- [ ] Replace or retire [Cynork shell feature](../../features/cynork/cynork_shell.feature) only after the shell migration decision is explicit.

- [ ] Add BDD coverage for `--thread-new` before the first completion request.

- [ ] Add BDD coverage for `/thread new` during an active session.

- [ ] Add BDD coverage for unknown `/thread` subcommands that keep the session alive.

- [ ] Add BDD coverage for thread history navigation and thread rename if those contracts land in this round.

- [ ] Add BDD coverage for startup and in-session auth recovery if those contracts land in this round.

- [ ] Run `just docs-check` after the docs changes settle.

- [ ] Run `just test-bdd` after the relevant feature and behavior changes land.

- [ ] Run `just ci` before considering the round complete.

## Phase 6 Remaining MVP Phase 2 Work After TUI MVP

- [ ] Resume the remaining MVP Phase 2 MCP tool slices beyond the currently implemented `db.preference.*` set.

- [ ] Finish the remaining LangGraph graph-node work identified in [MVP implementation plan](../mvp_plan.md).

- [ ] Finish the verification-loop work needed for PMA => Project Analyst => result review flows.

- [ ] Close the known chat/runtime drifts tracked in [MVP implementation plan](../mvp_plan.md), especially bounded wait, retry behavior, and other user-visible chat reliability gaps.

- [ ] Reassess whether any deferred TUI features should be pulled forward after the first end-to-end TUI validation pass.

## Phase 7 Worker Deployment Simplification Docs

- [ ] Promote [Single binary node manager and worker API proposal](../draft_specs/single_binary_node_manager_worker_api_proposal.md) into stable requirements and worker-node tech-spec updates in this round.

- [ ] Keep single-binary worker implementation deferred until after the first TUI rollout.

- [ ] Ensure the promoted worker deployment docs clearly distinguish normative deployment topology decisions from deferred implementation work.

## Recommended Execution Order

- [ ] First, resolve the chat-thread contract mismatch and the CLI thread-control docs.

- [ ] Second, promote the PMA conversation-history clarification.

- [ ] Third, cut the TUI proposal down to the minimum first-rollout normative scope.

- [ ] Fourth, implement the backend thread capabilities required for that scope.

- [ ] Fifth, implement the `cynork` TUI first-rollout slice.

- [ ] Sixth, update feature coverage and run docs, BDD, and CI validation.

- [ ] Seventh, promote the single-binary worker deployment docs while keeping implementation deferred until after the first TUI rollout.

- [ ] Eighth, return to the remaining MVP Phase 2 orchestration and MCP work once the TUI can drive realistic stack testing.

## Exit Criteria for This Round

- [ ] The TUI-first docs are promoted far enough that implementation does not depend on unresolved source-of-truth conflicts.

- [ ] A user can log in, create or switch threads, chat in a multi-line TUI, and observe project and model context.

- [ ] The first TUI rollout is covered by updated BDD and passes repository validation.

- [ ] The remaining MVP Phase 2 work is clearly separated as the next follow-on implementation stage rather than mixed into the TUI-first milestone.
