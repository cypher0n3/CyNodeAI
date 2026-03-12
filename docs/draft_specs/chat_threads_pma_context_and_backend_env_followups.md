# Draft Follow-Ups: Chat Threads, PMA Context, and Backend Env Delivery

- [1. Scope](#1-scope)
- [2. PMA Conversation History Preservation](#2-pma-conversation-history-preservation)
- [3. Explicit Chat Thread Creation](#3-explicit-chat-thread-creation)
- [4. Thread Acquisition Clarification](#4-thread-acquisition-clarification)
- [5. Related Additional Specs and Requirements](#5-related-additional-specs-and-requirements)
- [6. Promotion Checklist](#6-promotion-checklist)
- [7. References](#7-references)

## 1. Scope

Date: 2026-03-11.
Status: Draft in `docs/draft_specs/`.
This document is not normative.
Its purpose is to preserve and organize concrete documentation follow-up work so it can be resumed after merge.

This revision also aligns the draft more closely with the repository authoring standards in [`docs/docs_standards/spec_authoring_writing_and_validation.md`](../docs_standards/spec_authoring_writing_and_validation.md).

### 1.1 Main Refinements

- separating requirement changes from tech spec changes;
- identifying the canonical document that should own each contract;
- avoiding duplicate sources of truth; and
- turning broad notes into concrete requirement, spec, and feature follow-up items.

### 1.2 Reviewed Change Groups

1. PMA chat now preserves prior conversation history for the langchain-capable path by placing prior turns into system context and passing only the last user message as the current agent input.
2. The orchestrator and CLI now expose explicit new-thread creation via `POST /v1/chat/threads`, `CreateChatThread`, `--thread-new`, and `/thread new`.
3. The orchestrator, shared node payloads, and node-manager now carry orchestrator-derived inference backend environment values into both the Ollama container and managed service containers, including PMA.
4. Tests and mocks were updated to cover the new PMA prompt-building helpers, explicit thread creation path, and the new inference-backend env plumbing.

## 2. PMA Conversation History Preservation

This section covers the PMA chat change in `agents/internal/pma`.

### 2.1 Observed Branch Behavior

The current branch replaces `buildFullPrompt()` with a split approach.
Prior turns are appended to the system context by `buildSystemContextWithHistory()`.
The last user message is extracted by `lastUserMessage()`.
The final executor input is assembled by `buildAgentInput()`.

The practical effect is that the langchain-capable path preserves conversation history without treating the current user turn as if it were part of the system instruction block.
This is consistent with the single-input shape of the current agent executor.

### 2.2 Requirement Assessment

No new requirement appears necessary for this change.
[`REQ-PMAGNT-0108`](../requirements/pmagnt.md#req-pmagnt-0108) already requires PMA to pass baseline context and resolved additional context to every LLM it uses.
[`REQ-PMAGNT-0100`](../requirements/pmagnt.md#req-pmagnt-0100) and [`REQ-PMAGNT-0101`](../requirements/pmagnt.md#req-pmagnt-0101) already cover the PMA role/instructions model behavior that this path sits within.

The gap is primarily at the tech spec level.
The implementation behavior should be described explicitly so reviewers can verify that conversation history is preserved in the correct place and that the current user turn remains the executor input.

### 2.3 PMA Tech Spec Updates

The canonical owner for this behavior should be [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md).
That document already owns PMA request handling and LLM context composition.

Add a new Spec Item under the LLM-context section.
Recommended Spec ID: `CYNAI.PMAGNT.LLMConversationHistory`.
The item should define the contract for the langchain-capable chat path and should avoid introducing new RFC-2119 language that belongs in requirements.

#### 2.3.1 `CYNAI.PMAGNT.LLMConversationHistory` Minimum Content

- scope: the langchain-capable PMA chat path only;
- inputs: system context, ordered message history, and current request model path;
- behavior: prior conversation turns are rendered into a dedicated conversation-history block within the system-context composition;
- behavior: the current turn for the executor input is the last user-role message, or the last message if no user-role message exists;
- ordering: prior turns preserve original conversation order;
- observability: logs or diagnostics should make it possible to distinguish the capable-model path from the direct-generation path; and
- algorithm: build system context first, append prior turns, extract current input, and then invoke the executor.

If the draft model-routing document at [`docs/draft_specs/llm_routing_and_model_handling_spec_draft.md`](../draft_specs/llm_routing_and_model_handling_spec_draft.md) is later promoted, it may include a short cross-reference note.
That draft should not become the canonical source of truth for this PMA-specific context-composition behavior.

### 2.4 PMA Feature Updates

Update [`features/agents/pma_chat_and_context.feature`](../../features/agents/pma_chat_and_context.feature).
Add a scenario that covers a multi-turn request such as `user => assistant => user`.
The scenario should verify that prior turns are present in the conversation-history portion of the effective prompt and that the final user turn remains the executor input.

The scenario should continue to tag [`REQ-PMAGNT-0108`](../requirements/pmagnt.md#req-pmagnt-0108).
Once the new Spec Item exists, the scenario should also tag the new PMA conversation-history spec anchor.

## 3. Explicit Chat Thread Creation

This section covers the new explicit thread-creation path across the orchestrator and `cynork`.

### 3.1 Explicit Thread Branch Behavior

The branch adds a new handler for `POST /v1/chat/threads`.
The handler authenticates the user, derives project context, creates a thread through `CreateChatThread`, and returns `201 Created` with a `thread_id`.

The branch also adds a client method in `cynork` and two user-facing controls.
The first is `cynork chat --thread-new`.
The second is `/thread new` within the interactive chat session.

### 3.2 Existing Canonical Spec Coverage and Gap

The current draft should explicitly acknowledge that [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md) already contains the canonical thread-management API surface.
Spec Item [`CYNAI.USRGWY.ChatThreadsMessages.ApiSurface`](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-apisurface) already lists `POST /v1/chat/threads` as a standardized endpoint.

That means this branch is not introducing an entirely new endpoint from a spec perspective.
It is implementing an endpoint that the current tech spec already expects.

However, there is now a spec-implementation gap that should be called out clearly.
The existing spec says the request body for `POST /v1/chat/threads` must allow `project_id` and `title`.
The current branch instead uses no request body and derives project context from the `OpenAI-Project` header path already used by chat completions.

After comparing the surrounding canonical docs with the current implementation, this looks like a case where the canonical tech spec should be updated to match the implementation.
The reason is that the implementation aligns better with the existing chat-scoping model than the current thread-create request-body wording does.

#### 3.2.1 Why the Current Implementation Fits Better

- [`REQ-USRGWY-0131`](../requirements/usrgwy.md#req-usrgwy-0131) already says chat threads use the `OpenAI-Project` header for explicit project context and use the user's default project when the header is absent.
- [`docs/tech_specs/openai_compatible_chat_api.md`](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-endpoints) already defines that same header-based project-scoping rule for chat persistence.
- [`docs/tech_specs/cli_management_app_commands_chat.md`](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichat) already defines `--project-id` in terms of the `OpenAI-Project` header, not a request body field.
- The current branch's only implemented client flow for explicit thread creation is the chat CLI flow, which naturally follows the existing chat header model.

By contrast, the current `project_id` and `title` request-body requirement in [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-apisurface) is not backed by a current requirement, is not used by the present implementation, and is not exercised by the new CLI behavior.

#### 3.2.2 Mismatch Recommendation

- Prefer the current implementation's project-scoping approach.
- Update the canonical tech spec so Phase 1 explicit thread creation uses the same `OpenAI-Project` header semantics as the rest of the chat surface.
- Do not add a requirement that mandates `project_id` or `title` in the request body for this Phase 1 endpoint.
- Treat `title` support as deferred follow-up work unless a concrete client flow needs it.

### 3.3 Proposed Requirement Additions

The current requirements do not state explicit user-facing creation of a new chat thread as clearly as the existing tech spec does.
To close that traceability gap, add the following requirement entries unless a parallel branch consumes these numbers first.

These requirement additions should cover the capability itself.
They should not encode the request-body-versus-header design choice.
That detail belongs in the canonical tech spec.

In [`docs/requirements/usrgwy.md`](../requirements/usrgwy.md), add proposed `REQ-USRGWY-0135`.

#### 3.3.1 `REQ-USRGWY-0135` Suggested Text

- The User API Gateway MUST support explicit creation of a chat thread for the authenticated user.
- Explicit thread creation MUST return the newly created thread identifier.
- Explicit thread creation MUST create a distinct thread rather than reusing the currently active thread.
- The requirement should trace to [`CYNAI.USRGWY.ChatThreadsMessages.ApiSurface`](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-apisurface) and [`CYNAI.USRGWY.ChatThreadsMessages.Threads`](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-threads).

In [`docs/requirements/client.md`](../requirements/client.md), add proposed `REQ-CLIENT-0181`.

#### 3.3.2 `REQ-CLIENT-0181` Suggested Text

- The CLI chat command MUST support starting a new conversation thread on explicit user request.
- The CLI MUST support a startup control for a fresh thread and an in-session slash command for a fresh thread.
- The CLI fresh-thread controls MUST use the gateway thread-creation API and MUST apply the resulting thread to subsequent chat completions in that session.
- The requirement should trace to the CLI thread-control spec item proposed below.

### 3.4 Explicit Thread Tech Spec Updates

Keep [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md) as the single source of truth for the thread-management endpoint contract.
Do not duplicate the full `POST /v1/chat/threads` contract into [`docs/tech_specs/openai_compatible_chat_api.md`](../tech_specs/openai_compatible_chat_api.md), because that document is the canonical source for the OpenAI-compatible surface only.

#### 3.4.1 Required Canonical Spec Work

- update [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md) to make the Phase 1 `POST /v1/chat/threads` contract match the implemented chat flow;
- state that `POST /v1/chat/threads` requires authentication, creates a distinct thread, and returns `201 Created` with `thread_id`;
- state that project context for `POST /v1/chat/threads` follows the same rule as chat completions: use the `OpenAI-Project` header when present, otherwise use the creating user's default project per [`REQ-USRGWY-0131`](../requirements/usrgwy.md#req-usrgwy-0131);
- remove the current Phase 1 `MUST allow project_id` and `title` request-body language, or explicitly move that shape to a later extension if the project wants a richer Data REST create-thread flow in the future;
- add a dedicated thread-acquisition rule or operation under [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md) to distinguish active-thread reuse from explicit fresh-thread creation; and
- cross-link, rather than duplicate, this thread-creation contract from [`docs/tech_specs/user_api_gateway.md`](../tech_specs/user_api_gateway.md).

For the CLI side, update [`docs/tech_specs/cli_management_app_commands_chat.md`](../tech_specs/cli_management_app_commands_chat.md).
Add a dedicated Spec Item for thread controls.
Recommended Spec ID: `CYNAI.CLIENT.CliChatThreadControl`.

#### 3.4.2 `CYNAI.CLIENT.CliChatThreadControl` Minimum Content

- the startup behavior of `--thread-new`;
- the slash command contract for `/thread new`;
- whether `/thread` with no subcommand is an alias for `/thread new`;
- the error behavior for unknown `/thread` subcommands;
- the relationship between thread creation and the existing `OpenAI-Project` session context; and
- that `--thread-new` applies before the first completion request in both interactive and one-shot mode, because the current implementation performs thread creation before branching into message mode.

Also update [`docs/tech_specs/cynork_cli.md`](../tech_specs/cynork_cli.md) so the gateway capability inventory includes the thread-creation endpoint now used by the CLI.

Only add a short cross-reference in [`docs/tech_specs/openai_compatible_chat_api.md`](../tech_specs/openai_compatible_chat_api.md) if needed.
That document should not become the canonical owner of the non-OpenAI `POST /v1/chat/threads` contract.

### 3.5 Explicit Thread Feature Updates

Update [`features/cynork/cynork_chat.feature`](../../features/cynork/cynork_chat.feature).
Add one scenario for `--thread-new` before the first completion request.
Add one scenario for `/thread new` during an existing session.
Add one scenario for an unknown `/thread` subcommand that prints guidance and keeps the session alive.

Add or extend an end-to-end chat feature under [`features/e2e/`](../../features/e2e/) so the gateway behavior for `POST /v1/chat/threads` is exercised with authentication and error handling.
The feature work should tag the final requirement IDs and the final CLI-thread-control spec anchor.

## 4. Thread Acquisition Clarification

This section captures a separate, but related, contract clarification suggested by the branch.

### 4.1 Clarification Needed

The code now makes the distinction between two different thread-acquisition behaviors explicit.
The first behavior is get-or-create-active-thread.
The second behavior is always-create-new-thread.

That distinction should be made explicit in the canonical thread spec rather than being left implicit in storage code and handler naming.
This is a tech spec clarification, not a new product capability.

### 4.2 Thread Acquisition Tech Spec Updates

Update [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md).
Add a dedicated Spec Item or subsection for thread acquisition.
Recommended Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ThreadAcquisition`.

#### 4.2.1 `CYNAI.USRGWY.ChatThreadsMessages.ThreadAcquisition` Modes

- active-thread acquisition for ordinary server-managed chat continuity within the inactivity window; and
- explicit fresh-thread acquisition for user-requested context reset.

#### 4.2.2 `CYNAI.USRGWY.ChatThreadsMessages.ThreadAcquisition` Minimum Content

- scope for each acquisition mode;
- project scoping rules;
- inactivity-window behavior for active-thread reuse;
- the requirement that explicit fresh-thread acquisition always returns a new thread id; and
- any observable difference in persistence and retrieval behavior.

This clarification should then be linked from the chat-completions conversation model in [`docs/tech_specs/openai_compatible_chat_api.md`](../tech_specs/openai_compatible_chat_api.md) rather than duplicated there.

## 5. Related Additional Specs and Requirements

The items in this section are not directly required by the main implementation changes summarized above.
They are adjacent follow-up work that became more obvious during review.

### 5.1 Context Size Tracking

This work belongs primarily in the `USRGWY` domain because it affects prompt construction for chat threads and is most naturally defined at the gateway chat/thread boundary.

Add proposed `REQ-USRGWY-0136` to [`docs/requirements/usrgwy.md`](../requirements/usrgwy.md).

#### 5.1.1 `REQ-USRGWY-0136` Suggested Text

- The system MUST track the effective context size used to build a chat completion request for a thread.
- Context-size tracking MUST be model-aware so the system can compare current prompt size against the effective context window of the selected model.
- Context-size tracking MUST be deterministic for a given thread state and model selection.

Add a new Spec Item to [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md).
Recommended Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ContextSizeTracking`.

#### 5.1.2 `CYNAI.USRGWY.ChatThreadsMessages.ContextSizeTracking` Minimum Content

- what content contributes to effective context size;
- whether measurement uses exact tokenization, a deterministic estimate, or a provider/model-specific adapter;
- where the tracked value is computed and stored; and
- what downstream decisions consume it.

### 5.2 Automatic Context Summarization

Add proposed `REQ-USRGWY-0137` to [`docs/requirements/usrgwy.md`](../requirements/usrgwy.md).

#### 5.2.1 `REQ-USRGWY-0137` Suggested Text

- When the effective context size for the next chat completion reaches at least 95 percent of the selected model's context window, the system MUST compact older conversation context before issuing the next completion request.
- The compaction step MUST preserve enough recent unsummarized context for the next user turn and the expected assistant response.
- The compaction step MUST preserve conversation continuity in a deterministic and reviewable way.

Add a new Spec Item to [`docs/tech_specs/openai_compatible_chat_api.md`](../tech_specs/openai_compatible_chat_api.md) or [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md), but pick one canonical owner and cross-link from the other.
Recommended Spec ID: `CYNAI.USRGWY.OpenAIChatApi.ContextCompaction` if the chat API document owns the processing pipeline.
Recommended Spec ID: `CYNAI.USRGWY.ChatThreadsMessages.ContextCompaction` if the thread document owns the lifecycle.

#### 5.2.2 Context Compaction Spec Minimum Content

- the 95 percent trigger threshold;
- how the effective context window is derived for the selected model;
- when compaction runs in the request-processing sequence;
- whether compaction is implemented as summarization, truncation plus summary, or another deterministic method;
- what recent turns or token budget remain in unsummarized form; and
- where the compacted summary artifact is stored or injected.

### 5.3 Orchestrator-Directed Ollama Launch and Env Delivery

This work belongs primarily in the `WORKER` and `ORCHES` domains because it defines who decides local inference backend runtime configuration and who is responsible for launching the backend in normal operations.

Recent branch edits now implement part of this direction already.

#### 5.3.1 Observed Backend Env Behavior

- `go_shared_libs/contracts/nodepayloads/nodepayloads.go` adds `inference_backend.env` and `managed_services.services[].inference.backend_env` so the orchestrator can deliver backend-derived environment values through the node config payload.
- `orchestrator/internal/handlers/nodes.go` now derives GPU variant and VRAM from the node capability snapshot, computes hardware-sized `OLLAMA_CONTEXT_LENGTH` and `OLLAMA_NUM_CTX`, places those values into `inference_backend.env`, and also mirrors them into PMA managed service `backend_env`.
- `worker_node/cmd/node-manager/main.go` now passes orchestrator-derived environment values as `-e` flags when launching the Ollama container.
- `worker_node/internal/nodeagent/runargs.go` now passes backend-derived environment values through to managed service containers in node-local inference mode.
- `agents/internal/pma/langchain.go` now reads `OLLAMA_NUM_CTX` and applies it as an Ollama per-request runner option, so the PMA actually consumes the orchestrator/node-manager-delivered context-size value.

The implementation direction is therefore no longer only a proposal.
It is now partially implemented across the orchestrator, payload contract, node-manager, and PMA runtime.

#### 5.3.2 Desired Steady-State Architecture

- in normal operations, Ollama should be launched and supervised by the node-manager rather than by the orchestrator Docker Compose file;
- the orchestrator should decide the effective backend configuration for the node and deliver it through the existing node-configuration flow; and
- the node-manager should pass the orchestrator-directed environment through when it launches the Ollama container and any managed service containers that depend on the same backend.

This direction already aligns with the existing worker-node payload model in [`docs/tech_specs/worker_node_payloads.md`](../tech_specs/worker_node_payloads.md), which defines `inference_backend` as orchestrator-delivered desired state for the node.
It also aligns with the broader orchestrator-managed desired-state model already used for managed services.

One implementation nuance should be reflected in the canonical docs.
The current node-manager startup code still tolerates an already-existing Ollama container and starts it if present.
That is useful for dev and compatibility flows, but the canonical docs should make clear that this is a fallback or migration path rather than the normal-operation ownership model.

Add a proposed worker requirement in [`docs/requirements/worker.md`](../requirements/worker.md).

#### 5.3.3 Worker Requirement Suggested Text

- In normal operations, a worker node that provides local Ollama inference MUST start and supervise the Ollama container through the node-manager rather than relying on the orchestrator Docker Compose stack to run Ollama directly.
- The node-manager MUST derive Ollama runtime configuration from orchestrator-delivered node configuration when such configuration is present.
- The node-manager MUST pass through orchestrator-directed environment values needed by the Ollama container at launch time.
- When managed service containers depend on the same node-local inference backend, the node-manager MUST pass through the orchestrator-directed backend environment values to those managed service containers when the config contract requires it.

Add a proposed orchestrator requirement in [`docs/requirements/orches.md`](../requirements/orches.md).

#### 5.3.4 Orchestrator Requirement Suggested Text

- The orchestrator MUST decide the effective local inference backend configuration for a node when directing that node to provide Ollama inference.
- When the orchestrator requires a specific Ollama environment value for node-local inference behavior, it MUST deliver that value to the node-manager through the canonical node-configuration contract rather than assuming a static Docker Compose environment.
- When the orchestrator directs a managed service that depends on node-local inference, it MUST include any required backend-derived environment values in the managed service inference config so the service can use the same effective backend settings as the node-local Ollama container.

Add or refine the canonical tech spec in [`docs/tech_specs/worker_node_payloads.md`](../tech_specs/worker_node_payloads.md).

#### 5.3.5 Recommended Tech Spec Approach

- extend the `inference_backend` payload so orchestrator-directed runtime configuration can include explicit environment values for the inference backend container;
- extend the managed service inference payload so backend-derived environment values can be delivered to agent containers that use node-local inference;
- define which fields are orchestrator-owned versus node-local defaults;
- define worker behavior when the orchestrator omits the environment value, provides it, or changes it across config revisions; and
- define whether applying a changed value requires container restart or replacement under the node-manager reconciliation model.

Also update [`docs/tech_specs/orchestrator_inference_container_decision.md`](../tech_specs/orchestrator_inference_container_decision.md) so the orchestrator-side decision logic explicitly owns the effective Ollama environment value and emits it into node configuration.
Update [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md) so PMA's node-local inference config explicitly includes consumption of backend-derived environment values such as `OLLAMA_NUM_CTX`.
Update [`docs/tech_specs/ports_and_endpoints.md`](../tech_specs/ports_and_endpoints.md) if needed so it no longer implies that normal-operation Ollama hosting belongs to the orchestrator stack when the node-manager-managed model is the intended steady state.

This should remain a tech-spec and requirements update.
It should not be expressed as a Compose-file requirement, because the desired direction is to remove Compose from the normal-operation ownership path for node-local Ollama.

### 5.4 Chat Timeout and Agent Loop Tuning

This section captures timeout and agent-loop tuning changes from the recent branch edits.

#### 5.4.1 Observed Runtime Tuning Changes

- `orchestrator/internal/config/config.go` increases the default gateway `WriteTimeout` from 120 seconds to 300 seconds; and
- `agents/internal/pma/langchain.go` lowers the PMA langchain maximum iteration count from 10 to 3.

These changes look like implementation tuning rather than a new capability.
They appear to be covered already by [`REQ-USRGWY-0128`](../requirements/usrgwy.md#req-usrgwy-0128), [`REQ-ORCHES-0131`](../requirements/orches.md#req-orches-0131), [`REQ-ORCHES-0132`](../requirements/orches.md#req-orches-0132), and the timeout/reliability items in [`docs/tech_specs/openai_compatible_chat_api.md`](../tech_specs/openai_compatible_chat_api.md).

#### 5.4.2 Tuning Recommendation

- do not add a new requirement for the specific default values `300s` or `3 iterations`;
- update operator-facing notes in the chat API timeout docs if the project wants the new default documented explicitly; and
- keep the exact defaults and iteration cap as implementation-tunable unless the product needs them fixed by spec.

### 5.5 Proposed Feature Coverage for Context Compaction

Add feature coverage once the requirements and canonical Spec Item exist.
The most direct home is an end-to-end chat feature under [`features/e2e/`](../../features/e2e/).

#### 5.5.1 Scenario Coverage

- a thread that remains below the threshold and does not compact;
- a thread that crosses the threshold and compacts before the next completion;
- preservation of recent turns after compaction; and
- continued successful completion within the model limit after compaction.

## 6. Promotion Checklist

When promoting this draft into normative docs, use the following order.

1. Resolve the existing spec-implementation mismatch for `POST /v1/chat/threads` in [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md) before changing tests to depend on the new handler contract.
2. Add the missing explicit-thread requirements to [`docs/requirements/usrgwy.md`](../requirements/usrgwy.md) and [`docs/requirements/client.md`](../requirements/client.md).
3. Add the PMA conversation-history Spec Item to [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md).
4. Add the CLI thread-control Spec Item to [`docs/tech_specs/cli_management_app_commands_chat.md`](../tech_specs/cli_management_app_commands_chat.md).
5. Add or refine the thread-acquisition Spec Item in [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md).
6. Add the worker/orchestrator Ollama-ownership and backend-env updates in
   [`docs/requirements/worker.md`](../requirements/worker.md),
   [`docs/requirements/orches.md`](../requirements/orches.md),
   [`docs/tech_specs/worker_node_payloads.md`](../tech_specs/worker_node_payloads.md),
   [`docs/tech_specs/orchestrator_inference_container_decision.md`](../tech_specs/orchestrator_inference_container_decision.md),
   and [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md)
   if that architecture direction is accepted.
7. Decide whether the new chat timeout defaults need only operator-doc clarification or any canonical spec wording adjustment.
8. Update feature files only after the requirement IDs and spec anchors are stable.
9. Run `just docs-check` and fix any link, format, or validation failures.

## 7. References

- [`docs/docs_standards/spec_authoring_writing_and_validation.md`](../docs_standards/spec_authoring_writing_and_validation.md)
- [`docs/requirements/pmagnt.md`](../requirements/pmagnt.md)
- [`docs/requirements/usrgwy.md`](../requirements/usrgwy.md)
- [`docs/requirements/client.md`](../requirements/client.md)
- [`docs/requirements/orches.md`](../requirements/orches.md)
- [`docs/requirements/worker.md`](../requirements/worker.md)
- [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md)
- [`docs/tech_specs/chat_threads_and_messages.md`](../tech_specs/chat_threads_and_messages.md)
- [`docs/tech_specs/openai_compatible_chat_api.md`](../tech_specs/openai_compatible_chat_api.md)
- [`docs/tech_specs/cli_management_app_commands_chat.md`](../tech_specs/cli_management_app_commands_chat.md)
- [`docs/tech_specs/cynork_cli.md`](../tech_specs/cynork_cli.md)
- [`docs/tech_specs/user_api_gateway.md`](../tech_specs/user_api_gateway.md)
- [`docs/tech_specs/worker_node_payloads.md`](../tech_specs/worker_node_payloads.md)
- [`docs/tech_specs/orchestrator_inference_container_decision.md`](../tech_specs/orchestrator_inference_container_decision.md)
