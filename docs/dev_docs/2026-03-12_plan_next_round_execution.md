# Next Round Execution Plan

## Purpose

This temporary plan tracks the next round of work for CyNodeAI as of 2026-03-12.

The priority for this round is the `cynork` TUI because it is the fastest path to realistic user-level validation of the current stack.

This plan is derived from [MVP scope](../mvp.md), [MVP implementation plan](../mvp_plan.md), [Cynork TUI draft proposal](../draft_specs/cynork_tui_spec_proposal.md), [Chat threads, PMA context, and backend env follow-ups](../draft_specs/chat_threads_pma_context_and_backend_env_followups.md), and [Single binary node manager and worker API proposal](../draft_specs/single_binary_node_manager_worker_api_proposal.md).

## Round Goals

- [x] Promote the minimum set of chat and thread contracts required to support a usable TUI-first workflow.
  (Phase 1 spec alignment completed 2026-03-12; REQ-USRGWY-0135, REQ-CLIENT-0181, thread controls and PMA conversation history in place.)

- [x] Extend the OpenAI-compatible chat contract and TUI work scope to support both `POST /v1/chat/completions` and `POST /v1/responses`.
  (Gateway, orchestrator, PMA, thread storage, and client-side spec anchors now cover both surfaces.)

- [x] Define and lock a minimal first-rollout `cynork` TUI scope that is sufficient for end-to-end stack testing.
  (Phase 0 and Phase 2 done; [cynork_tui.md](../tech_specs/cynork_tui.md) and locked scope in [cynork_cli.md](../tech_specs/cynork_cli.md).)

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

- [ ] Keep one canonical owner for each contract and cross-link from related documents instead of duplicating source-of-truth content.

- [ ] Treat the first TUI rollout as a minimum viable product for user testing, not as the final complete chat experience.

- [ ] Defer attractive but non-blocking features when they would slow the first usable TUI milestone.

- [ ] Keep `POST /v1/chat/completions` as the broad compatibility baseline while adding `POST /v1/responses` as an additive OpenAI-compatible surface owned by the gateway contract.

- [ ] Do not pull forward non-TUI MVP Phase 2 work or worker-deployment follow-on work until the TUI can drive the full chat path end to end.

- [ ] Keep the TUI implementation testable through stable seams: backend APIs first, then reusable chat or controller logic, then the fullscreen UI and Python PTY harness on top.

- [ ] Prefer machine-detectable UI landmarks and explicit state transitions over visually clever but test-hostile behavior so Python automation can validate the TUI reliably.

## Phase 0 Locked Scope Decisions

- [x] Deprecate `cynork shell` in docs and implementation planning, while keeping a temporary compatibility path during migration.
  (Normative in [cynork_cli.md](../tech_specs/cynork_cli.md) TUI Scope and Locked Decisions; MVP Scope updated.)

- [x] Ship `cynork tui` as the first full-screen entrypoint and keep `cynork chat` available as a compatibility path during rollout.

- [x] Keep the first TUI thread-management slice limited to create, list, switch, and rename.

- [x] Include a minimal structured chat `parts` model for thinking and tool activity only.

- [x] Include dual interactive chat-surface support in the first TUI rollout scope: `POST /v1/chat/completions` and `POST /v1/responses`.

- [x] Treat the Python PTY or fullscreen TUI harness as part of the primary TUI milestone rather than as a late validation add-on.

- [x] Promote the single-binary worker proposal docs in this round, but defer implementation until after the first TUI rollout.
  (Locked in cynork_cli; Phase 8 still to promote draft into stable requirements/worker-node tech-spec updates.)

## Phase 1 TUI-Enabling Spec Alignment

This phase resolves the source-of-truth issues that would otherwise make the TUI implementation unstable.

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

- [ ] Add BDD coverage for multi-turn PMA chat history handling after the spec anchor is stable.

### Rich Chat and Dual-Surface Contracts

- [x] Promote the OpenAI-compatible gateway contract so both `POST /v1/chat/completions` and `POST /v1/responses` are in scope for the TUI.

- [x] Add normalized assistant-output rules so one user prompt can yield one logical assistant turn with ordered structured parts.

- [x] Add stable structured-turn storage rules for visible text, hidden thinking, tool activity, and file references.

- [x] Add TUI transcript-rendering and generation-state rules so thinking, tool rows, and in-flight assistant updates are source-of-truth behavior rather than draft-only ideas.

- [x] Add orchestrator-side continuation-state requirements for `previous_response_id` so the TUI implementation has a stable backend contract for responses continuation.

## Phase 2 TUI MVP Spec Cut

This phase narrows the large TUI proposal down to the minimum first rollout that should be treated as in-scope.

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

### Explicitly Deferred From the First TUI Rollout

- [x] Do not implement thread summary or archive in this round.

- [x] Do not implement assistant download-reference workflows in this round.

- [x] Do not implement full attachment upload UX in this round.

- [x] Do not implement queued drafts or deferred send behavior in this round.

- [x] Do not make bare `cynork` launch the TUI by default in this round.

- [x] Do not implement web-based login or SSO-specific flows in this round.

## Phase 3 Backend Prerequisites Required for TUI Chat

This phase captures only the gateway, orchestrator, and data-path work required before the TUI can deliver the intended full chat experience.
The order inside this phase matters because later TUI work depends on these backend behaviors being stable.

### Minimum Backend Surface

- [ ] Implement `POST /v1/responses` on the gateway as an additive OpenAI-compatible interactive chat surface.

- [ ] Implement retained response metadata and `previous_response_id` continuation handling without changing CyNodeAI thread ownership rules.

- [ ] Implement normalized assistant-turn persistence so structured parts and canonical visible text are stored consistently for both interactive chat surfaces.

- [ ] Ensure chat secret redaction occurs before any chat data is persisted and before any inference handoff uses the content.

- [ ] Implement explicit thread creation with the accepted contract.

- [ ] Implement list-threads support with pagination and default recent-first ordering.

- [ ] Implement get-thread and get-thread-messages behavior needed for thread history view and reload.

- [ ] Implement patch-thread-title support for rename flows in this round.

### Required Backend Validation Before TUI Wiring

- [ ] Verify both interactive chat surfaces produce coherent canonical visible text plus structured-turn persistence for the same logical assistant turn.

- [ ] Verify the PMA path and direct-inference path both honor the same redaction, persistence, and normalized-output rules.

- [ ] Verify thread retrieval and active-thread behavior are stable enough that the TUI can depend on them for history and fresh-thread controls.

### Deferred Backend Work This Round

- [ ] Do not implement summary generation in this round.

- [ ] Do not implement archive or soft-delete in this round.

- [ ] Do not implement download-ref contracts in this round.

- [ ] Do not implement rich attachment storage contracts in this round.

- [ ] Do not implement context compaction or automatic summarization in this round.

## Phase 4 Shared Chat Controller and Testable Seams

This phase defines the shared implementation seams that both the fullscreen TUI and the Python PTY harness depend on.

### Controller and Session State

- [ ] Extract reusable chat-session or controller logic out of the CLI command layer so request shaping, slash handling, thread actions, and in-flight chat state are not owned only by the fullscreen UI.

- [ ] Keep session state for model, project, thread, and auth recovery on instance-bound objects rather than package-level globals so parallel test execution and PTY automation remain reliable.

- [ ] Define stable controller-facing actions and state transitions for send-message, fresh-thread, thread-switch, project-switch, model-switch, and auth-recovery flows.

### Transport and Rendering Seams

- [ ] Add a client-side chat transport abstraction that can target both `POST /v1/chat/completions` and `POST /v1/responses`.

- [ ] Keep transcript assembly and canonical visible-text projection reusable so non-interactive CLI paths, PTY automation, and the fullscreen TUI all validate the same logical turn behavior.

- [ ] Add stable machine-detectable UI landmarks and reduced-noise test semantics where needed so PTY tests can assert on state transitions without depending on fragile redraw timing.

## Phase 5 `cynork` TUI and Python PTY Harness Implementation

This phase delivers the first usable full-screen TUI slice and the Python PTY harness together.
They must be developed in tandem so each validates the other as behavior lands, with minimal human intervention.

### Entry Point and Core Wiring

- [ ] Add the `cynork tui` command path.

- [ ] Make interactive `cynork chat` invoke the same fullscreen TUI entry flow as `cynork tui`, while keeping `cynork chat --message` and non-interactive usage line-oriented and parseable.

- [ ] Preserve stable non-interactive chat behavior for scripting and piping use cases.

- [ ] Use `POST /v1/chat/completions` as the default interactive chat path in the first implementation while preserving full support for `POST /v1/responses` behind the same chat abstraction and test harness.

### Core TUI Experience

- [ ] Implement the full-screen layout with scrollback, composer, status bar, and a togglable context pane.

- [ ] Implement a multi-line composer with clear send semantics.

- [ ] Implement scrollback navigation, search, and copy behavior.

- [ ] Implement status-bar rendering for gateway, auth identity, project, model, and connectivity state.

- [ ] Implement local slash-command discovery and execution within the TUI.

- [ ] Implement transcript rendering for visible text, hidden-by-default thinking, tool activity rows, and ordered multi-item assistant turns.

- [ ] Implement in-flight generation handling so one assistant turn is updated progressively and reconciled cleanly on completion.

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

### Python PTY Harness

- [ ] Add Python PTY process launch and teardown helpers for the fullscreen TUI.

- [ ] Add fixed terminal sizing support for PTY-driven tests so layout assertions are reproducible.

- [ ] Add key event injection helpers for common actions such as send, fresh-thread, thread switch, project or model switch, and exit.

- [ ] Add screen or buffer capture helpers and semantic wait utilities for state transitions such as prompt-ready, assistant-in-flight, response-complete, thread-switched, and auth-recovery-ready.

- [ ] Add stable Python assertions around semantic UI landmarks instead of exact model wording or brittle full-frame diffs.

### Tandem TUI and Harness Validation

- [ ] Validate send and receive behavior through the PTY harness as the composer and transcript rendering land.

- [ ] Validate thread create, list, switch, and rename behavior through the PTY harness as soon as the corresponding TUI flows exist.

- [ ] Validate hidden-thinking, ordered assistant output, and tool-activity rendering through the PTY harness as transcript rendering lands.

- [ ] Validate startup and in-session auth recovery through the PTY harness as soon as the TUI flow exists.

### TUI Chat-Complete Exit for Implementation

- [ ] Confirm a user can send a prompt, receive a response, see thread state, observe project and model context, and continue the same conversation without leaving the TUI.

- [ ] Confirm a user can start a fresh thread and continue chatting in the new thread from the TUI.

- [ ] Confirm the TUI remains coherent whether the backend path is chat-completions or responses.

## Phase 6 TUI Validation and BDD

- [ ] Update [Cynork chat feature](../../features/cynork/cynork_chat.feature) to cover the accepted first-rollout TUI behaviors.

- [ ] Keep [Cynork shell feature](../../features/cynork/cynork_shell.feature) as compatibility coverage during this round and do not retire it yet.

- [ ] Add BDD coverage for `--thread-new` before the first completion request.

- [ ] Add BDD coverage for `/thread new` during an active session.

- [ ] Add BDD coverage for unknown `/thread` subcommands that keep the session alive.

- [ ] Add BDD coverage for thread history navigation and thread rename.

- [ ] Add BDD coverage for startup and in-session auth recovery.

- [ ] Add coverage for both supported interactive chat surfaces so TUI and gateway behavior stays aligned across `POST /v1/chat/completions` and `POST /v1/responses`.

- [ ] Add coverage for structured-turn rendering expectations that matter to the TUI, especially hidden thinking, ordered assistant output, and tool activity.

- [ ] Add Python E2E coverage for the fullscreen TUI flows that are now required for the primary milestone.

- [ ] Use the Python PTY harness continuously during TUI development rather than waiting until the end of the round to validate the fullscreen UI.

- [ ] Run `just docs-check` after the docs changes settle.

- [ ] Run `just test-bdd` after the relevant feature and behavior changes land.

- [ ] Run the Python E2E suite or targeted Python PTY subset as part of the TUI acceptance gate.

- [ ] Run `just ci` before considering the round complete.

## Phase 7 Remaining MVP Phase 2 Work After TUI MVP

- [ ] Resume the remaining MVP Phase 2 MCP tool slices beyond the currently implemented `db.preference.*` set.

- [ ] Finish the remaining LangGraph graph-node work identified in [MVP implementation plan](../mvp_plan.md).

- [ ] Finish the verification-loop work needed for PMA => Project Analyst => result review flows.

- [ ] Close the known chat/runtime drifts tracked in [MVP implementation plan](../mvp_plan.md), especially bounded wait, retry behavior, and other user-visible chat reliability gaps.

- [ ] Keep all currently deferred TUI features deferred for this round and record any pull-forward candidates only as input for the next planning cycle.

## Phase 8 Worker Deployment Simplification Docs

- [ ] Promote [Single binary node manager and worker API proposal](../draft_specs/single_binary_node_manager_worker_api_proposal.md) into stable requirements and worker-node tech-spec updates in this round.

- [ ] Keep single-binary worker implementation deferred until after the first TUI rollout.

- [ ] Ensure the promoted worker deployment docs clearly distinguish normative deployment topology decisions from deferred implementation work.

## Recommended Execution Order

- [x] First, resolve the chat-thread contract mismatch and the CLI thread-control docs.

- [x] Second, promote the PMA conversation-history clarification.

- [x] Third, promote the rich-chat and dual-surface source-of-truth work required for the TUI chat experience.
  (`openai_compatible_chat_api.md`, `chat_threads_and_messages.md`, `cynork_tui.md`, `cynode_pma.md`, and related requirements updated.)

- [x] Fourth, cut the TUI proposal down to the minimum first-rollout normative scope.
  (`cynork_tui.md` and Phase 0/2 checkboxes completed.)

- [ ] Fifth, implement the backend chat prerequisites in this order: `POST /v1/responses`, continuation metadata, normalized assistant-turn persistence, redaction-before-persistence guarantees, then thread retrieval and rename support.

- [ ] Sixth, extract or define the reusable chat or controller seams that both the TUI and Python PTY harness will depend on.

- [ ] Seventh, implement the `cynork` TUI and the Python PTY harness in tandem, using each to validate the other as behavior lands.

- [ ] Eighth, update feature coverage and run docs, BDD, Python E2E, and CI validation for the full TUI chat path.

- [ ] Ninth, promote the single-binary worker deployment docs while keeping implementation deferred until after the first TUI rollout.

- [ ] Tenth, return to the remaining MVP Phase 2 orchestration and MCP work only after the TUI can drive realistic stack testing.

## Exit Criteria for This Round

- [ ] The TUI-first docs are promoted far enough that implementation does not depend on unresolved source-of-truth conflicts.

- [ ] A user can log in, create or switch threads, chat in a multi-line TUI, and observe project and model context.

- [ ] The fullscreen TUI can be driven end to end from the Python test scripts with minimal human intervention.

- [ ] The first TUI rollout is covered by updated BDD, Python PTY validation, and passes repository validation.

- [ ] The remaining MVP Phase 2 work is clearly separated as the next follow-on implementation stage rather than mixed into the TUI-first milestone.

## Progress Notes

- **2026-03-12:** Phase 1 (TUI-enabling spec alignment) completed: chat threads and messages updated (active vs explicit thread, POST /v1/chat/threads), REQ-USRGWY-0135 and REQ-CLIENT-0181 added, CYNAI.CLIENT.CliChatThreadControls and CYNAI.PMAGNT.ConversationHistory added.
  Follow-up correction applied the same day: explicit fresh-thread creation remains a separate CyNodeAI Data REST capability, but subsequent `POST /v1/chat/completions` requests remain OpenAI-compatible and do not require any CyNodeAI-specific thread identifier.
  Execution order steps one and two done.
- **2026-03-12:** Rich-chat and dual-surface spec promotion completed for the TUI track: the stable docs now cover both `POST /v1/chat/completions` and `POST /v1/responses`, structured turns, hidden thinking, ordered assistant output, TUI transcript rendering, generation state, orchestrator continuation state, and the full end-to-end chat flow.
- **2026-03-12:** Phase 0 locked scope decisions and Phase 2 TUI MVP spec cut completed: cynork_cli.md updated (TUI scope and locked decisions, deprecate shell, cynork tui + chat in Required Commands, MVP Scope); `cynork_tui.md` defines the minimum layout, composer, thread history, status bar, transcript rendering, generation state, slash parity, auth recovery, and explicitly deferred list.
  Execution order steps three and four done.
  Next: Phase 3 backend prerequisites, Phase 4 shared controller and test seams, Phase 5 TUI plus Python PTY harness implementation, Phase 6 validation, then follow-on docs and non-TUI MVP work.
