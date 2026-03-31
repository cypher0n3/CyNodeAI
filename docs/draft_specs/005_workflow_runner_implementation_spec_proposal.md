# Workflow Runner Implementation Spec Proposal

- [Document Overview](#document-overview)
  - [Addressed Gaps](#addressed-gaps)
  - [Integration Targets](#integration-targets)
- [Transport Clarification](#transport-clarification)
  - [In-Process vs HTTP](#in-process-vs-http)
- [`WorkflowRunner` Interface](#workflowrunner-interface)
  - [`WorkflowRunner` Inputs](#workflowrunner-inputs)
  - [`WorkflowRunner` Behavior](#workflowrunner-behavior)
  - [`WorkflowRunner` Error Conditions](#workflowrunner-error-conditions)
  - [`WorkflowRunner` Concurrency](#workflowrunner-concurrency)
  - [`WorkflowRunner` Reference - Go](#workflowrunner-reference---go)
- [`RunWorkflow` Operation](#runworkflow-operation)
  - [`RunWorkflow` Inputs](#runworkflow-inputs)
  - [`RunWorkflow` Outputs](#runworkflow-outputs)
  - [`RunWorkflow` Behavior](#runworkflow-behavior)
  - [`RunWorkflow` Algorithm](#runworkflow-algorithm)
  - [`RunWorkflow` Error Conditions](#runworkflow-error-conditions)
  - [`RunWorkflow` Concurrency](#runworkflow-concurrency)
  - [`RunWorkflow` Reference - Go](#runworkflow-reference---go)
- [`NodeFunc` Type](#nodefunc-type)
  - [`NodeFunc` Inputs](#nodefunc-inputs)
  - [`NodeFunc` Outputs](#nodefunc-outputs)
  - [`NodeFunc` Behavior](#nodefunc-behavior)
  - [`NodeFunc` Error Conditions](#nodefunc-error-conditions)
  - [`NodeFunc` Concurrency](#nodefunc-concurrency)
  - [`NodeFunc` Reference - Go](#nodefunc-reference---go)
- [`NodeErrorPolicy` Rule](#nodeerrorpolicy-rule)
  - [`NodeErrorPolicy` Scope](#nodeerrorpolicy-scope)
  - [`NodeErrorPolicy` Preconditions](#nodeerrorpolicy-preconditions)
  - [`NodeErrorPolicy` Outcomes](#nodeerrorpolicy-outcomes)
  - [`NodeErrorPolicy` Algorithm](#nodeerrorpolicy-algorithm)
  - [`NodeErrorPolicy` Error Conditions](#nodeerrorpolicy-error-conditions)
  - [`NodeErrorPolicy` Observability](#nodeerrorpolicy-observability)
- [`MaxRetryConfig` Rule](#maxretryconfig-rule)
  - [`MaxRetryConfig` Scope](#maxretryconfig-scope)
  - [`MaxRetryConfig` Preconditions](#maxretryconfig-preconditions)
  - [`MaxRetryConfig` Outcomes](#maxretryconfig-outcomes)
  - [`MaxRetryConfig` Error Conditions](#maxretryconfig-error-conditions)
  - [`MaxRetryConfig` Observability](#maxretryconfig-observability)
- [`WorkflowNodeTimeout` Rule](#workflownodetimeout-rule)
  - [`WorkflowNodeTimeout` Scope](#workflownodetimeout-scope)
  - [`WorkflowNodeTimeout` Preconditions](#workflownodetimeout-preconditions)
  - [`WorkflowNodeTimeout` Outcomes](#workflownodetimeout-outcomes)
  - [`WorkflowNodeTimeout` Error Conditions](#workflownodetimeout-error-conditions)
  - [`WorkflowNodeTimeout` Observability](#workflownodetimeout-observability)
- [Runner-to-LLM Interaction Model](#runner-to-llm-interaction-model)
  - [`LLMNodeInvocation` Operation](#llmnodeinvocation-operation)
  - [`LLMNodeInvocation` Inputs](#llmnodeinvocation-inputs)
  - [`LLMNodeInvocation` Outputs](#llmnodeinvocation-outputs)
  - [`LLMNodeInvocation` Behavior](#llmnodeinvocation-behavior)
  - [`LLMNodeInvocation` Error Conditions](#llmnodeinvocation-error-conditions)
  - [`LLMNodeInvocation` Concurrency](#llmnodeinvocation-concurrency)
- [Workflow-Executor Instruction Set](#workflow-executor-instruction-set)
  - [`WorkflowInstructionSet` Type](#workflowinstructionset-type)
  - [`WorkflowInstructionSet` Layout](#workflowinstructionset-layout)
  - [`WorkflowInstructionSet` Constraints](#workflowinstructionset-constraints)
  - [`WorkflowInstructionSet` Validation](#workflowinstructionset-validation)
- [Open Questions](#open-questions)

## Document Overview

This proposal addresses implementation gaps identified during the transition from a Python LangGraph workflow engine to a Go-native workflow runner (state machine) within the orchestrator process.
The existing [Workflow MVP](../tech_specs/workflow_mvp.md) and [Orchestrator](../tech_specs/orchestrator.md) specs define the graph topology, state model, node behaviors, checkpointing, and lease lifecycle.
They do not yet specify the Go runner interface, node dispatch model, error/timeout semantics, retry configuration, runner-to-LLM interaction, or the instruction sets that drive LLM-based nodes.

This draft fills those gaps.
Once accepted, its content should be split into [Workflow MVP](../tech_specs/workflow_mvp.md) (runner interface, node dispatch, error/timeout, retries, LLM invocation) and [CyNode PMA](../tech_specs/cynode_pma.md) (instruction sets).

### Addressed Gaps

- **Transport clarification:** Reconcile the in-process runner with the existing HTTP API language in Workflow MVP.
- **Go runner interface and operation:** Define the `WorkflowRunner` interface, `RunWorkflow` operation, and `NodeFunc` type.
- **Per-node error and timeout semantics:** Define what happens when a node fails or exceeds its deadline.
- **Max retry configuration:** Define where the retry bound comes from and its default.
- **Runner-to-LLM interaction:** Define how LLM-backed nodes invoke inference via langchaingo.
- **Workflow-executor instruction set:** Define the instruction bundles that drive LLM reasoning within workflow nodes.

### Integration Targets

When this proposal is promoted, its content integrates into:

- [`docs/tech_specs/workflow_mvp.md`](../tech_specs/workflow_mvp.md) -- Transport Clarification through Runner-to-LLM Interaction Model.
- [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md) -- Workflow-Executor Instruction Set.

## Transport Clarification

The Workflow MVP Runtime and Hosting section ([`workflow_mvp.md`](../tech_specs/workflow_mvp.md)) states that the workflow engine is a "Go-native workflow runner (state machine) within the orchestrator process."
The [Workflow Start/Resume API Contract](../tech_specs/workflow_mvp.md#spec-cynai-orches-workflowstartresumeapi) section describes an HTTP transport.

These two statements are not contradictory but require clarification.

### In-Process vs HTTP

The workflow runner executes within the orchestrator Go process.
It is started via in-process function calls, not via HTTP.

The HTTP API described in [Workflow Start/Resume API Contract](../tech_specs/workflow_mvp.md#spec-cynai-orches-workflowstartresumeapi) is the orchestrator's **external** contract for:

- The contract reference runner stub ([`scripts/workflow_runner_stub/minimal_runner.py`](../../scripts/workflow_runner_stub/minimal_runner.py)) used for integration testing.
- Future external workflow clients that may run out-of-process.

The in-process runner does not call HTTP endpoints on itself.
Instead, it invokes the same underlying service functions that the HTTP handlers call.
The checkpoint and lease persistence layer is shared: both the in-process runner and the HTTP-based contract stub write to the same PostgreSQL tables ([`workflow_checkpoints`](../tech_specs/workflow_mvp.md#spec-cynai-schema-workflowcheckpointstable), [`task_workflow_leases`](../tech_specs/workflow_mvp.md#spec-cynai-schema-taskworkflowleasestable)).

When this section is promoted, the Workflow MVP Runtime and Hosting section ([`workflow_mvp.md`](../tech_specs/workflow_mvp.md)) should be amended to state:

- The orchestrator invokes the runner via in-process Go function calls.
- The HTTP API contract remains as the stable external interface for testing and for any future out-of-process runner.

## `WorkflowRunner` Interface

- Spec ID: `CYNAI.ORCHES.WorkflowRunner` <a id="spec-cynai-orches-workflowrunner"></a>
- Status: draft

The `WorkflowRunner` interface defines the contract that the orchestrator uses to start, resume, and manage workflow execution for a single task.
The runner is instantiated per workflow invocation; each instance holds the graph definition, the checkpoint store reference, and the node function registry.

### `WorkflowRunner` Inputs

Construction requires:

- **Graph definition:** The set of named nodes and their transition edges (the topology from the Graph Topology section of [`workflow_mvp.md`](../tech_specs/workflow_mvp.md)).
- **Checkpoint store:** An interface to the PostgreSQL-backed checkpoint persistence layer ([`workflow_checkpoints`](../tech_specs/workflow_mvp.md#spec-cynai-schema-workflowcheckpointstable)).
- **Lease manager:** An interface to the task workflow lease layer ([`task_workflow_leases`](../tech_specs/workflow_mvp.md#spec-cynai-schema-taskworkflowleasestable), [lease lifecycle](../tech_specs/orchestrator.md#spec-cynai-orches-taskworkflowleaselifecycle)).
- **Node function registry:** A map from node name to `NodeFunc` (see [`NodeFunc` Type](#nodefunc-type)).
- **LLM client:** A langchaingo model client for nodes that require inference (see [Runner-to-LLM Interaction Model](#runner-to-llm-interaction-model)).
- **MCP client:** A client for MCP tool calls routed through the orchestrator MCP gateway.

### `WorkflowRunner` Behavior

- The runner is not a long-lived service; it is created per task workflow invocation and discarded after the workflow completes or fails.
- The runner executes the graph by calling node functions in sequence according to the graph edges, starting from the entry node or from the last checkpointed node on resume.
- After each node completes, the runner persists the updated state to the checkpoint store before transitioning to the next node.
- The runner acquires the task workflow lease before executing any node and renews it periodically during execution.
  On completion or terminal failure the runner releases the lease.
- The runner does not serve HTTP, gRPC, or any network protocol; it is invoked by the orchestrator as an in-process component.

### `WorkflowRunner` Error Conditions

- **Lease unavailable:** If the lease cannot be acquired (held by another instance), the runner returns an error without executing any node.
- **Checkpoint load failure:** If the checkpoint store is unreachable or returns a corrupt checkpoint, the runner returns an error.
  A missing checkpoint for a `task_id` is not an error; it indicates a fresh run starting from the entry node.
- **Node failure:** Handled per [`NodeErrorPolicy` Rule](#nodeerrorpolicy-rule).
- **Lease renewal failure:** If lease renewal fails during execution, the runner should attempt to complete or checkpoint the current node, then return an error.
  The runner should not continue to the next node without a valid lease.

### `WorkflowRunner` Concurrency

- A single `WorkflowRunner` instance executes nodes sequentially (one node at a time per task).
- The orchestrator may run multiple `WorkflowRunner` instances concurrently for **different** tasks.
  Each instance operates on its own task-scoped state and lease.
- Cancellation: the orchestrator passes a `context.Context` to the runner.
  When the context is cancelled (e.g. orchestrator shutdown, user-initiated cancel), the runner should complete or checkpoint the current node, release the lease, and return.
  Nodes that perform I/O (LLM calls, MCP calls, job dispatch) must propagate the context and respect cancellation.

### `WorkflowRunner` Reference - Go

<a id="ref-go-cynai-orches-workflowrunner"></a>

```go
type WorkflowRunner interface {
    RunWorkflow(ctx context.Context, taskID uuid.UUID) (WorkflowResult, error)
}
```

## `RunWorkflow` Operation

- Spec ID: `CYNAI.ORCHES.RunWorkflow` <a id="spec-cynai-orches-runworkflow"></a>
- Status: draft

`RunWorkflow` is the sole entry point for executing or resuming a task workflow.
The orchestrator calls this operation when a workflow start trigger fires (see [Workflow Start Triggers](../tech_specs/workflow_mvp.md#spec-cynai-orches-workflowstarttriggers)).

### `RunWorkflow` Inputs

- `ctx` (`context.Context`): Cancellation and deadline propagation.
- `taskID` (`uuid.UUID`): The task identifier scoping this workflow run.

### `RunWorkflow` Outputs

- `WorkflowResult`: A struct containing:
  - `TaskID` (`uuid.UUID`): Echo of the input task identifier.
  - `FinalNode` (`string`): The name of the last node executed (e.g. `"finalize_summary"` or `"mark_failed"`).
  - `State` (`WorkflowState`): The terminal workflow state snapshot.
- `error`: Non-nil on infrastructure failure (lease, checkpoint, context cancellation).
  A workflow that reaches `Mark Failed` is not an infrastructure error; it returns a `WorkflowResult` with `FinalNode = "mark_failed"` and a nil error.

### `RunWorkflow` Behavior

See [`RunWorkflow` Algorithm](#runworkflow-algorithm).

### `RunWorkflow` Algorithm

<a id="algo-cynai-orches-runworkflow"></a>

1. Acquire the task workflow lease for `taskID`.
   If the lease is held by another instance, return error (lease conflict). <a id="algo-cynai-orches-runworkflow-step-1"></a>
2. Load checkpoint for `taskID` from the checkpoint store. <a id="algo-cynai-orches-runworkflow-step-2"></a>
   - If a checkpoint exists, set `currentNode` to the node **after** `last_node_id` (the next node per the graph edges) and restore `state` from the checkpoint.
   - If no checkpoint exists, set `currentNode` to the entry node (`load_task_context`) and initialize `state` to a zero-value `WorkflowState` with `TaskID` set.
3. **Node loop:** While `currentNode` is not a terminal node: <a id="algo-cynai-orches-runworkflow-step-3"></a>
   1. Check `ctx` for cancellation.
      If cancelled, persist checkpoint with `last_node_id` set to the **previous** completed node, release lease, return context-cancelled error.
   2. Look up `NodeFunc` for `currentNode` in the node function registry.
   3. Invoke the `NodeFunc` with `ctx`, `state`, and node-specific dependencies (LLM client, MCP client).
   4. On success: update `state` with the node's output, persist checkpoint (state + `last_node_id = currentNode`), resolve the next node via graph edges and any branch predicate from the returned state.
   5. On error: apply [`NodeErrorPolicy` Rule](#nodeerrorpolicy-rule).
      If the policy outcome is `retry`, re-invoke the same node (up to the node-level retry limit).
      If the policy outcome is `fail_workflow`, set `currentNode` to `mark_failed` and continue the loop.
   6. Renew lease.
      If renewal fails, persist checkpoint, release lease (best-effort), return lease-renewal error.
4. Execute the terminal node (`finalize_summary` or `mark_failed`). <a id="algo-cynai-orches-runworkflow-step-4"></a>
5. Persist final checkpoint. <a id="algo-cynai-orches-runworkflow-step-5"></a>
6. Release lease. <a id="algo-cynai-orches-runworkflow-step-6"></a>
7. Return `WorkflowResult`. <a id="algo-cynai-orches-runworkflow-step-7"></a>

### `RunWorkflow` Error Conditions

- `ErrLeaseConflict`: Another instance holds the lease for this task.
- `ErrCheckpointCorrupt`: Checkpoint data could not be deserialized.
- `ErrCheckpointStoreUnavailable`: Checkpoint store (PostgreSQL) is unreachable.
- `ErrContextCancelled`: The parent context was cancelled during execution.
- `ErrLeaseRenewalFailed`: Lease renewal failed mid-execution; workflow was checkpointed and stopped.

### `RunWorkflow` Concurrency

- One goroutine per `RunWorkflow` invocation.
  The orchestrator may call `RunWorkflow` concurrently for different `taskID` values.
- The lease guarantees at most one concurrent `RunWorkflow` per `taskID`.
- Cancellation via `ctx` is the primary mechanism for stopping a running workflow.
  The runner should checkpoint and release the lease within a bounded grace period after cancellation.

### `RunWorkflow` Reference - Go

<a id="ref-go-cynai-orches-runworkflow"></a>

```go
func (r *runner) RunWorkflow(ctx context.Context, taskID uuid.UUID) (WorkflowResult, error)
```

## `NodeFunc` Type

- Spec ID: `CYNAI.ORCHES.NodeFunc` <a id="spec-cynai-orches-nodefunc"></a>
- Status: draft

A `NodeFunc` is the unit of execution within the workflow runner.
Each graph node (Load Task Context, Plan Steps, Dispatch Step, Collect Result, Verify Step Result, Finalize Summary, Mark Failed) is implemented as a `NodeFunc`.

### `NodeFunc` Inputs

- `ctx` (`context.Context`): Cancellation and deadline propagation.
  The runner wraps the parent context with a per-node timeout (see [`WorkflowNodeTimeout` Rule](#workflownodetimeout-rule)).
- `state` (`*WorkflowState`): Mutable pointer to the current workflow state.
  The node reads inputs from and writes outputs to this state.
- **Dependency injection:** Nodes that require LLM inference or MCP tools receive those clients via the runner's dependency container (constructor injection or a `NodeDeps` struct).
  Nodes that do not need LLM (e.g. `Collect Result`) do not receive an LLM client.

### `NodeFunc` Outputs

- `NodeResult`: Contains:
  - `NextEdge` (`string`): The name of the outgoing edge to follow.
    For nodes with a single outgoing edge, this is a fixed constant.
    For branch nodes (e.g. `Has Next Step?`, `Pass?`, `Retries Left?`), this is the evaluated branch label.
- `error`: Non-nil on node-level failure.
  Infrastructure errors (context cancelled, MCP unreachable) and domain errors (LLM returned invalid output) are both expressed as errors.
  The runner applies [`NodeErrorPolicy` Rule](#nodeerrorpolicy-rule) to decide the next action.

### `NodeFunc` Behavior

- A `NodeFunc` performs one bounded unit of work corresponding to a single graph node.
- It reads from `state`, performs its work (MCP calls, LLM calls, worker API calls), writes results back into `state`, and returns the edge to follow.
- A `NodeFunc` must not call other `NodeFunc` instances directly; node sequencing is the runner's responsibility.
- A `NodeFunc` must not persist checkpoints; the runner handles checkpointing after each successful node return.

### `NodeFunc` Error Conditions

A `NodeFunc` may return the following categories of error:

- **Transient infrastructure:** MCP gateway timeout, checkpoint store briefly unavailable, LLM provider rate-limited.
  These are retryable per [`NodeErrorPolicy` Rule](#nodeerrorpolicy-rule).
- **Permanent infrastructure:** Context cancelled, lease lost.
  These are not retryable; the runner checkpoints and stops.
- **Domain failure:** LLM returned unparseable output, verification found unrecoverable gap.
  Whether these are retryable depends on the node and the error policy configuration.

### `NodeFunc` Concurrency

- A `NodeFunc` executes synchronously within the runner's node loop (one node at a time).
- Internally, a `NodeFunc` may use concurrency (e.g. parallel MCP tool calls via langchaingo's multi-tool support), but it must return a single `NodeResult` and must propagate `ctx` cancellation to all internal goroutines.

### `NodeFunc` Reference - Go

<a id="ref-go-cynai-orches-nodefunc"></a>

```go
type NodeFunc func(ctx context.Context, state *WorkflowState, deps NodeDeps) (NodeResult, error)
```

## `NodeErrorPolicy` Rule

- Spec ID: `CYNAI.ORCHES.NodeErrorPolicy` <a id="spec-cynai-orches-nodeerrorpolicy"></a>
- Status: draft

This rule defines how the runner handles errors returned by a `NodeFunc`.

### `NodeErrorPolicy` Scope

Applies to every `NodeFunc` invocation within the runner's node loop ([`RunWorkflow` Algorithm step 3.5](#algo-cynai-orches-runworkflow-step-3)).

### `NodeErrorPolicy` Preconditions

- A `NodeFunc` has returned a non-nil error.
- The runner has the node's retry count from `state.attempts_by_step` (or a runner-level attempt counter for non-step nodes).

### `NodeErrorPolicy` Outcomes

The policy produces one of:

- **`retry`**: Re-invoke the same node.
  The runner increments the attempt counter and re-invokes the `NodeFunc` after an optional backoff.
- **`fail_workflow`**: Transition to the `mark_failed` node.
  The error details are recorded in `state.verification.findings`.

### `NodeErrorPolicy` Algorithm

<a id="algo-cynai-orches-nodeerrorpolicy"></a>

1. If the error is a permanent infrastructure error (context cancelled, lease lost), outcome is `fail_workflow`. <a id="algo-cynai-orches-nodeerrorpolicy-step-1"></a>
2. If the node's attempt count has reached the node-level retry limit (see [`MaxRetryConfig` Rule](#maxretryconfig-rule)), outcome is `fail_workflow`. <a id="algo-cynai-orches-nodeerrorpolicy-step-2"></a>
3. If the error is transient infrastructure or a retryable domain error, outcome is `retry` with exponential backoff (base interval configurable, default 1 second, multiplied by 2 per attempt, capped at 30 seconds). <a id="algo-cynai-orches-nodeerrorpolicy-step-3"></a>
4. Otherwise, outcome is `fail_workflow`. <a id="algo-cynai-orches-nodeerrorpolicy-step-4"></a>

### `NodeErrorPolicy` Error Conditions

The policy itself does not produce errors.
It consumes errors from `NodeFunc` and produces an outcome.

### `NodeErrorPolicy` Observability

- Every policy evaluation must emit a structured log entry with: `task_id`, `node_name`, `attempt`, `error_category` (transient/permanent/domain), `outcome` (retry/fail_workflow).
- When outcome is `fail_workflow`, the log entry should include the original error message.

## `MaxRetryConfig` Rule

- Spec ID: `CYNAI.ORCHES.MaxRetryConfig` <a id="spec-cynai-orches-maxretryconfig"></a>
- Status: draft

This rule defines how the maximum retry count for the workflow's step-level retry loop (`Retries Left?` branch in the Graph Topology section of [`workflow_mvp.md`](../tech_specs/workflow_mvp.md)) and for per-node infrastructure retries is determined.

### `MaxRetryConfig` Scope

Two distinct retry limits apply:

- **Step-level retries** (the `Retries Left?` branch): How many times a step may be re-dispatched after verification failure before the workflow transitions to `Mark Failed`.
  This is tracked in `state.attempts_by_step[step_index]`.
- **Node-level retries** (infrastructure): How many times a `NodeFunc` invocation is retried on transient errors before the error policy declares `fail_workflow`.
  This is tracked per node invocation within the runner, not in persisted state.

### `MaxRetryConfig` Preconditions

- The workflow state has been initialized (step-level) or the runner has been constructed (node-level).

### `MaxRetryConfig` Outcomes

- **Step-level default:** 3 attempts (1 initial + 2 retries).
- **Node-level default:** 3 attempts (1 initial + 2 retries).

Override sources, in precedence order (highest wins):

1. **Task-level preference:** If the task's effective preferences (resolved via the preference system; see [`docs/tech_specs/user_preferences.md`](../tech_specs/user_preferences.md)) include a `workflow.max_step_retries` or `workflow.max_node_retries` key, that value is used.
2. **System default:** The values above (3 attempts each).

The runner reads the step-level retry limit from `state.preferences_effective` after the `Load Task Context` node populates it.
The node-level retry limit is read from runner configuration at construction time.

### `MaxRetryConfig` Error Conditions

- If a preference value is present but is not a positive integer, the runner falls back to the system default and logs a warning.

### `MaxRetryConfig` Observability

- On workflow start, the runner should log the effective step-level and node-level retry limits for the task.

## `WorkflowNodeTimeout` Rule

- Spec ID: `CYNAI.ORCHES.WorkflowNodeTimeout` <a id="spec-cynai-orches-workflownodetimeout"></a>
- Status: draft

This rule defines per-node execution deadlines to prevent nodes from running indefinitely.

### `WorkflowNodeTimeout` Scope

Applies to every `NodeFunc` invocation.
The runner wraps the parent context with a per-node timeout before calling the `NodeFunc`.

### `WorkflowNodeTimeout` Preconditions

- The runner is about to invoke a `NodeFunc`.

### `WorkflowNodeTimeout` Outcomes

Default timeouts per node (wall-clock time):

- `load_task_context`: 30 seconds.
- `plan_steps`: 120 seconds (LLM inference may be slow).
- `dispatch_step`: 30 seconds.
- `collect_result`: 600 seconds (job execution on worker; this is the wait ceiling, not the job timeout itself).
- `verify_step_result`: 120 seconds (LLM inference).
- `finalize_summary`: 60 seconds.
- `mark_failed`: 30 seconds.

Override sources, in precedence order (highest wins):

1. **Task-level preference:** `workflow.node_timeouts.<node_name>` in effective preferences (value in seconds, positive integer).
2. **Runner configuration:** A node timeout map provided at runner construction time.
3. **System defaults:** The values above.

When a node's context deadline is exceeded, the `NodeFunc` receives a `context.DeadlineExceeded` error.
The runner treats this as a transient error and applies [`NodeErrorPolicy` Rule](#nodeerrorpolicy-rule).

### `WorkflowNodeTimeout` Error Conditions

- If a preference timeout value is present but not a positive integer, the runner falls back to the next precedence source and logs a warning.
- A `collect_result` timeout exceeding the runner's overall workflow timeout (if one exists) is capped to the workflow timeout.

### `WorkflowNodeTimeout` Observability

- On each node invocation, the runner should log the effective timeout for that node.
- On timeout, the runner should log the node name, task ID, and configured timeout.

## Runner-To-LLM Interaction Model

This section defines how the workflow runner invokes LLM inference for nodes that require it.

The workflow runner is the PMA's execution model (see the Runtime and Hosting section of [`workflow_mvp.md`](../tech_specs/workflow_mvp.md)).
It does not call `cynode-pma` over HTTP for LLM reasoning.
Instead, LLM-backed nodes invoke langchaingo directly within the orchestrator process, using the instruction set for the current node as the system message.

### `LLMNodeInvocation` Operation

- Spec ID: `CYNAI.ORCHES.LLMNodeInvocation` <a id="spec-cynai-orches-llmnodeinvocation"></a>
- Status: draft

This operation defines how a `NodeFunc` that requires LLM reasoning (e.g. `plan_steps`, `verify_step_result`) invokes the model.

### `LLMNodeInvocation` Inputs

- `ctx` (`context.Context`): Propagated from the runner; includes the per-node timeout.
- `instructionSet` (`WorkflowInstructionSet`): The instruction bundle for this node (see [Workflow-Executor Instruction Set](#workflow-executor-instruction-set)).
- `state` (`*WorkflowState`): The current workflow state, from which the node extracts task context, plan, last result, and other inputs for the LLM prompt.
- `llmClient` (`llms.Model`): The langchaingo model client, configured for the effective model (resolved via model routing; see [Project Manager Agent - LLM and Tool Execution](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmllmtoolimplementation)).
- `tools` (`[]tool.Tool`): The set of MCP-backed langchaingo tools available to this node.
  Tool availability is scoped per node: `plan_steps` may have access to `task.*` and `preference.*` tools; `dispatch_step` may additionally have `sandbox.*` and `node.*` tools.

### `LLMNodeInvocation` Outputs

- Structured output parsed from the LLM response.
  The expected schema is defined per node by the instruction set's output contract.
- `error`: Non-nil if the LLM call failed, the response was unparseable, or tool calls failed.

### `LLMNodeInvocation` Behavior

- The node composes the LLM prompt from the instruction set (system message), the relevant state fields (user/context messages), and any prior conversation turns within this node (for multi-turn tool use).
- The node calls the langchaingo model client with the composed messages and the tool set.
- If the model returns tool calls, the node executes them via the MCP client, appends the tool results to the conversation, and re-invokes the model (agentic loop) until the model returns a final text response.
- The node parses the final response according to the instruction set's output contract and returns the structured result.
- Multi-turn tool use within a single node invocation is bounded: the node must enforce a maximum number of LLM turns per invocation (default: 10).
  If the model does not produce a final response within that limit, the node returns a domain error.

### `LLMNodeInvocation` Error Conditions

- **LLM call failure:** Model provider unreachable, rate-limited, or returned an HTTP error.
  Classified as transient infrastructure.
- **Unparseable response:** Model returned text that does not match the expected output schema.
  Classified as retryable domain error (the retry may succeed with a different generation).
- **Tool call failure:** An MCP tool call returned an error.
  Classification depends on the MCP error: transient (gateway timeout) vs permanent (tool not found, policy denied).
- **Turn limit exceeded:** The agentic loop exceeded the maximum turn count.
  Classified as retryable domain error.

### `LLMNodeInvocation` Concurrency

- The agentic loop (LLM call => tool calls => LLM call) executes sequentially within a single goroutine.
- Multiple simultaneous tool calls within a single LLM turn may execute concurrently (per langchaingo's parallel tool call support), but the node waits for all tool results before the next LLM turn.
- Cancellation via `ctx` is propagated to all LLM and tool calls.

## Workflow-Executor Instruction Set

This section defines the instruction bundles that drive LLM reasoning within workflow nodes.
Instruction sets are distinct from the chat-facing PMA instruction bundles defined in [CyNode PMA - Instructions Loading and Routing](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-instructionsloading).

### `WorkflowInstructionSet` Type

- Spec ID: `CYNAI.AGENTS.WorkflowInstructionSet` <a id="spec-cynai-agents-workflowinstructionset"></a>
- Status: draft

A `WorkflowInstructionSet` is a named collection of files that together define the system message, output contract, and behavioral constraints for a single LLM-backed workflow node.

### `WorkflowInstructionSet` Layout

Instruction sets are stored on disk alongside the existing PMA instruction bundles.
The default layout extends the layout defined in [CyNode PMA - Instructions Loading and Routing](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-instructionsloading):

- Instructions root: `instructions/` (same root).
- Workflow instruction sets: `instructions/workflow/`.
- Per-node subdirectory: `instructions/workflow/<node_name>/`.

Each per-node directory contains:

- `system.md`: The system message template.
  It defines the node's identity, responsibilities, non-goals, and references to relevant specs.
- `output_contract.md` (or `output_contract.json`): The expected output schema.
  For structured output, this is a JSON Schema document.
  For free-text-with-markers output, this is a Markdown document defining required sections and markers.
- `tools.md` (optional): A description of the available tools and their intended use within this node.
  This supplements the MCP tool definitions with node-specific guidance.

Nodes that do not use LLM inference (e.g. `load_task_context`, `dispatch_step`, `collect_result`, `mark_failed`) do not have instruction sets.

Required instruction sets (MVP):

- `instructions/workflow/plan_steps/`
- `instructions/workflow/verify_step_result/`
- `instructions/workflow/finalize_summary/`

### `WorkflowInstructionSet` Constraints

- Instruction sets are loaded once at runner construction time and cached for the lifetime of the runner instance.
- The runner must validate at construction time that every LLM-backed node has a corresponding instruction set directory with at least `system.md` and `output_contract.md` (or `.json`).
  Missing instruction sets must cause a startup error.
- Instruction sets are **not** loaded from the database or from MCP; they are local files bundled with the `cynode-pma` binary or mounted into its container.
- The `system.md` template may contain placeholders (e.g. `{{.TaskID}}`, `{{.AcceptanceCriteria}}`) that the runner populates from `WorkflowState` before sending to the LLM.
  The template engine is Go's `text/template`.

### `WorkflowInstructionSet` Validation

- At runner construction, for each required instruction set:
  1. Verify the directory exists under the configured instructions root.
  2. Verify `system.md` exists and is non-empty.
  3. Verify `output_contract.md` or `output_contract.json` exists and is non-empty.
  4. If `output_contract.json` is present, validate it as syntactically valid JSON.
- Validation failures must produce a startup error with a message identifying the missing or invalid file.

## Open Questions

- **`collect_result` timeout vs job timeout:** The `collect_result` node waits for a worker job to complete.
  The node timeout (600s default) is the maximum wait time for the result, not the job execution timeout.
  The job execution timeout is set at dispatch time and enforced by the worker.
  Should the `collect_result` timeout be derived from the dispatched job's timeout plus a margin, rather than a fixed default?
- **Instruction set hot-reload:** Should the runner support reloading instruction sets without restarting the orchestrator process?
  The current proposal loads once at construction.
  Hot-reload would add complexity but enable iteration without restarts.
- **Graceful degradation for missing preferences:** When preference keys for retry limits or timeouts are absent, the runner falls back to system defaults.
  Should the runner log at `info` or `debug` level when using defaults?
- **`finalize_summary` as LLM node:** The MVP lists `finalize_summary` as requiring an instruction set.
  If the summary is a mechanical aggregation of verification results and artifacts (no LLM reasoning needed), it could be a non-LLM node.
  The decision depends on whether the summary requires natural-language generation.
