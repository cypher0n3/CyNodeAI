# Orchestrator Technical Spec

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Core Responsibilities](#core-responsibilities)
- [Health Checks](#health-checks)
  - [`Orchestrator.HealthEndpoints` Rule](#orchestratorhealthendpoints-rule)
- [Task Scheduler](#task-scheduler)
  - [Task Scheduler Responsibilities](#task-scheduler-responsibilities)
  - [Job Builder (Task-To-Job)](#job-builder-task-to-job)
  - [Task Cancel and Stop Job](#task-cancel-and-stop-job)
  - [`Orchestrator.JobTimeoutTracking` Rule](#orchestratorjobtimeouttracking-rule)
  - [Scheduled Run Routing to Project Manager Agent](#scheduled-run-routing-to-project-manager-agent)
- [Project Manager Agent](#project-manager-agent)
- [OpenAI-Compatible Interactive Chat Routing](#openai-compatible-interactive-chat-routing)
  - [OpenAI-Compatible Interactive Chat Routing Traces To](#openai-compatible-interactive-chat-routing-traces-to)
  - [OpenAI-Compatible Interactive Chat Routing Behavior](#openai-compatible-interactive-chat-routing-behavior)
  - [Streaming Relay to PMA](#streaming-relay-to-pma)
  - [Shared Streaming Contract Types](#shared-streaming-contract-types)
  - [Responses Continuation State](#responses-continuation-state)
  - [Chat File Upload Flow](#chat-file-upload-flow)
- [Managed Services (Worker-Managed)](#managed-services-worker-managed)
  - [Managed Services Definitions](#managed-services-definitions)
  - [Managed Services Behavior](#managed-services-behavior)
  - [Greedy PMA Provisioning on User Login](#greedy-pma-provisioning-on-user-login)
  - [Stale PMA Lifecycle and Teardown](#stale-pma-lifecycle-and-teardown)
  - [Managed Services Related Documents](#managed-services-related-documents)
  - [Project Manager Model (Startup Selection and Warmup)](#project-manager-model-startup-selection-and-warmup)
- [API Egress Server](#api-egress-server)
- [Web Egress Proxy](#web-egress-proxy)
- [Secure Browser Service](#secure-browser-service)
- [External Model Routing](#external-model-routing)
- [Model Management](#model-management)
- [User API Gateway](#user-api-gateway)
  - [Task Create Handoff](#task-create-handoff)
- [Sandbox Image Registry](#sandbox-image-registry)
- [Node Bootstrap and Configuration](#node-bootstrap-and-configuration)
  - [Job Dispatch](#job-dispatch)
- [Postgres Schema](#postgres-schema)
  - [Task vs Job (Terminology)](#task-vs-job-terminology)
  - [Tasks Table](#tasks-table)
  - [Task Dependencies Table](#task-dependencies-table)
  - [Task Status and Closed State](#task-status-and-closed-state)
  - [Jobs Table](#jobs-table)
  - [Nodes Table](#nodes-table)
  - [Node Capabilities Table](#node-capabilities-table)
- [MCP Tool Interface](#mcp-tool-interface)
- [Workflow Engine](#workflow-engine)
  - [Task Workflow Lease Lifecycle](#task-workflow-lease-lifecycle)
- [Orchestrator Self-Metadata and Logging](#orchestrator-self-metadata-and-logging)
- [Orchestrator Shutdown](#orchestrator-shutdown)
  - [Orchestrator Shutdown Traces To](#orchestrator-shutdown-traces-to)
- [Orchestrator Bootstrap Configuration](#orchestrator-bootstrap-configuration)

## Spec IDs

- Spec ID: `CYNAI.ORCHES.Doc.Orchestrator` <a id="spec-cynai-orches-doc-orchestrator"></a>

## Document Overview

This document describes the orchestrator responsibilities and its relationship to orchestrator-side agents.

## Core Responsibilities

- Acts as the control plane for nodes, jobs, tasks, and agent workflows.
  For the distinction between **task** (durable work item in the `tasks` table) and **job** (single execution unit dispatched to a worker), see [Task vs Job (Terminology)](#task-vs-job-terminology).
- Owns the source of truth for task state, results, logs, and user task-execution preferences in PostgreSQL.
- Dispatches sandboxed execution to worker nodes (via the worker API).
- Routes model inference to local nodes or to external providers when allowed.
- Schedules sandbox execution independently of where inference occurs.
- Tracks worker-managed long-lived services (including PMA) and their endpoints for routing and readiness.
- **Greedy PMA provisioning:** when users authenticate through the gateway on any client surface, the orchestrator starts or ensures a PMA instance for that session **without** waiting for the first chat message (see [Greedy PMA Provisioning on User Login](#greedy-pma-provisioning-on-user-login)).

## Health Checks

This section defines the orchestrator health and readiness endpoints.

### `Orchestrator.HealthEndpoints` Rule

- Spec ID: `CYNAI.ORCHES.Rule.HealthEndpoints` <a id="spec-cynai-orches-rule-healthendpoints"></a>

Traces To: [REQ-ORCHES-0120](../requirements/orches.md#req-orches-0120), [REQ-ORCHES-0150](../requirements/orches.md#req-orches-0150), [REQ-ORCHES-0151](../requirements/orches.md#req-orches-0151), [REQ-ORCHES-0189](../requirements/orches.md#req-orches-0189), [REQ-BOOTST-0002](../requirements/bootst.md#req-bootst-0002)

The orchestrator exposes health endpoints that distinguish "process alive" from "ready to accept work".
The orchestrator cannot report fully ready until at least one inference path exists (a worker that has reported ready and is inference-capable, or an LLM API key for PMA via API Egress), until at least one PMA instance has informed the orchestrator that it is online, and until the **PMA inference capability check** succeeds for the bootstrap path (see [Orchestrator Readiness and PMA Startup](orchestrator_bootstrap.md#spec-cynai-bootst-orchestratorreadinessandpmastartup)).

Endpoints

- `GET /healthz`
  - Returns 200 when the orchestrator process is alive and serving HTTP.
  - This endpoint MUST NOT require that the Project Manager model is online.
- `GET /readyz`
  - Returns 200 only when the orchestrator is in a ready state.
  - Returns 503 when prerequisites for readiness are not yet satisfied (no eligible inference path, no bootstrap PMA instance started or not yet online, PMA inference capability check not yet passed, Project Manager model not loaded when using local inference, or required credentials/policy not present).
  - The orchestrator MUST NOT return 200 until at least one inference path exists, until at least one PMA instance has informed the orchestrator that it is online and is reachable (e.g. responds to its health check), and until the **PMA inference capability check** has succeeded (see [CYNAI.BOOTST.PmaInferenceCapabilityCheck](orchestrator_bootstrap.md#spec-cynai-bootst-pmainferencecapabilitycheck)).
  - PMA is a core system feature and is always required; disabling PMA is not supported.
  - The response MUST include a reason that is actionable for an operator.
  - While `GET /readyz` returns 503, the orchestrator continues to serve the management surfaces required to become ready (for example system settings and credential configuration).

## Task Scheduler

- Spec ID: `CYNAI.ORCHES.TaskScheduler` <a id="spec-cynai-orches-taskscheduler"></a>

The orchestrator MUST include a task scheduler that decides when and where to run work.

### Task Scheduler Responsibilities

- **Queue**: Maintain a queue of pending work (tasks and jobs) backed by PostgreSQL so state survives restarts.
- **Dispatch**: Select eligible nodes based on capability, load, data locality, and model availability; dispatch jobs to the worker API; collect results and update task state.
  When the orchestrator receives job completion (success, failure, or timeout), it MUST pass that reporting to the Project Manager Agent and/or Project Analyst Agent for additional work (e.g. verification, remediation, follow-up tasks).
- **Retries and leases**: Support job leases, retries on failure, and idempotency so work is not lost or duplicated when nodes fail or restart.
- **Cron tool**: MUST support a cron (or equivalent) facility for scheduled jobs, wakeups, and automation.
  - Users and agents MUST be able to enqueue work at a future time or on a recurrence (cron expression or calendar-like).
  - The scheduler is responsible for firing at the scheduled time and enqueueing the corresponding tasks or jobs.
  - Schedule evaluation MUST be time-zone aware (schedules specify or inherit a time zone; next-run and history use that zone).
  - Schedules MUST support create, update, disable (temporarily stop firing without deleting), and cancellation (cancel the schedule or the next run).
  - The system MUST retain run history per schedule (past execution times and outcomes) for visibility and debugging.
  - The cron facility SHOULD be exposed to agents (e.g. via MCP tools) so they can create and manage scheduled jobs.

The scheduler MAY be implemented as a background process, a worker that consumes the queue, or integrated into the workflow engine; it MUST use the same node selection and job-dispatch contracts as the rest of the orchestrator.
Agents (e.g. Project Manager) and the cron facility enqueue work; the scheduler is responsible for dequeueing and dispatching to nodes.
The scheduler MUST be available via the User API Gateway so users can create and manage scheduled jobs, query queue and schedule state, and trigger wakeups or automation.

### Job Builder (Task-To-Job)

- Spec ID: `CYNAI.ORCHES.JobBuilder` <a id="spec-cynai-orches-jobbuilder"></a>

The orchestrator builds a job spec from one or more tasks before dispatch.
Only tasks with `planning_state=ready` are eligible for execution; see [Task Create Handoff](#task-create-handoff) and [Workflow Start Gate](workflow_mvp.md#spec-cynai-orches-workflowstartgateplanapproved).
The job builder MUST:

1. **Eligibility:** Consider only tasks whose `planning_state=ready` (and other gates such as plan state, dependencies, lease) when selecting work to run.
2. **Task bundle:** Form a job from 1-3 tasks that share the same `persona_id`, the same `project_id`, and when associated with a plan the same `plan_id`, and are in dependency order; represent them as `task_ids` map keyed by numeric order (e.g. 10, 20, 30).
   Single-task job = one key; bundle = 2-3 keys.
3. **Persona:** Resolve the shared persona from `task.persona_id` (from the first task or from the bundle); load persona record for inline embedding (title, description) and for `default_skill_ids`, `recommended_cloud_models`, `recommended_local_model_ids`.
4. **Allowed models:** Compute the effective allowed model set as the **intersection** of system, project, and user allowlists (null at a scope = no restriction at that scope).
   Worker node model inventory is not part of the allowed set; it is used only for placement.
5. **Model selection:** Select **exactly one** model for the job: from persona's recommended cloud models (by provider, keys, quota) or recommended local model ids (by node availability and context window).
   Consider user preference and API quota.
   The selected model MUST be in the effective allowed set.
6. **Node selection:** Choose a node by workload and model size (e.g. least-loaded node that can run the selected model).
   For local inference, if the chosen node is overloaded, the implementation MAY revise selection up to 2 iterations (e.g. try next node); after that it MAY fall back to the original least-loaded node.
7. **Build payload:** Build the job spec with `task_ids` (map), one inline `persona` object, one resolved `inference.model_id` (or equivalent), and for bundles embed full per-task context in `context.task_contexts` keyed by the same numeric keys so the job is self-contained (see [cynode_sba.md](cynode_sba.md)).
   Merge persona `default_skill_ids` with each task's `recommended_skill_ids` (union; task overrides duplicates) and resolve skills into context.
8. **Dispatch:** Dispatch only when the selected model is in the allowed set and all tasks in the bundle have `planning_state=ready`.

Traces To: [REQ-ORCHES-0178](../requirements/orches.md#req-orches-0178), [REQ-ORCHES-0180](../requirements/orches.md#req-orches-0180), [REQ-ORCHES-0181](../requirements/orches.md#req-orches-0181), [REQ-ORCHES-0182](../requirements/orches.md#req-orches-0182), [REQ-ORCHES-0183](../requirements/orches.md#req-orches-0183).

### Task Cancel and Stop Job

- Spec ID: `CYNAI.ORCHES.TaskCancelAndStopJob` <a id="spec-cynai-orches-taskcancelandstopjob"></a>

On a **task cancel** request (from User API Gateway, PMA, or slash command), the orchestrator MUST (1) mark the task as canceled (or transitioning to canceled) in task state and persist; (2) if the task has an **active job** (dispatched to a worker node and not yet in a terminal state), send a **stop job** request to that worker node for the corresponding `job_id` (and `task_id`) using the node's Worker API (e.g. `POST /v1/worker/jobs:stop`).
The orchestrator MUST use the same node credentials and base URL as for job dispatch.
If the job has already completed, failed, or been stopped, the stop request is a no-op; the node MAY return success or "not running" so the orchestrator can treat the request as satisfied.
See [worker_api.md - Stop Job](worker_api.md#spec-cynai-worker-stopjob) and [REQ-ORCHES-0184](../requirements/orches.md#req-orches-0184).

The orchestrator MUST reject artifacts, results, and any other output from stopped or canceled jobs for API access, storage, and persistence; such data MUST NOT be exposed to clients or written to durable storage as task/job results.
SBA (agent) tool calls that originate from a stopped or canceled job MUST be rejected by the orchestrator and by any gateway that authorizes tool access (e.g. MCP gateway).
The orchestrator MUST invalidate the job's token (or equivalent credential) when the job is stopped or canceled so that in-flight or late-arriving tool calls for that job are rejected.

Traces To: [REQ-ORCHES-0184](../requirements/orches.md#req-orches-0184).

### `Orchestrator.JobTimeoutTracking` Rule

- Spec ID: `CYNAI.ORCHES.Rule.JobTimeoutTracking` <a id="spec-cynai-orches-rule-jobtimeouttracking"></a>

Traces To: [REQ-ORCHES-0173](../requirements/orches.md#req-orches-0173), [REQ-ORCHES-0174](../requirements/orches.md#req-orches-0174)

The orchestrator implements job timeout tracking so that jobs are not left in an ambiguous state when a node does not report back (e.g. node crash, network partition).

#### `Orchestrator.JobTimeoutTracking` Scope

- Applies to every job dispatched by the orchestrator to a worker node.
- Effective deadline is derived at dispatch time from `sandbox.timeout_seconds` and is updated when a [timeout extension](cynode_sba.md#spec-cynai-sbagnt-timeoutextension) is granted and the orchestrator is informed of the new deadline.

#### `Orchestrator.JobTimeoutTracking` Preconditions

- Job is in progress (dispatched, no completion or failure reported).
- Effective deadline has been set (at dispatch or via extension).

#### `Orchestrator.JobTimeoutTracking` Outcomes

- The orchestrator runs a scheduled task (e.g. periodic cron or timer) that finds jobs in progress whose effective deadline has been exceeded without a reported completion or granted extension.
- Those jobs are marked as **failed** (timeout) so the orchestrator can re-issue or retry as policy allows.

#### `Orchestrator.JobTimeoutTracking` Observability

- Timeout-failed jobs are distinguishable in task/job state (e.g. failure reason or status code) for auditing and retry policy.

### Scheduled Run Routing to Project Manager Agent

- Spec ID: `CYNAI.ORCHES.ScheduledRunRouting` <a id="spec-cynai-orches-scheduledrunrouting"></a>
- Traces to: [REQ-ORCHES-0109](../requirements/orches.md#req-orches-0109), [Request Source and Orchestrator Handoff](cynode_pma.md#spec-cynai-pmagnt-requestsource)

When a schedule **fires**, the orchestrator has a run payload (created at schedule-creation time: e.g. task description, prompt, or concrete job spec).
Routing of that run MUST be determined by whether the payload requires agent reasoning, task interpretation, or planning:

- When a scheduled run's payload **requires agent reasoning, task interpretation, or planning** (e.g. natural-language task, "daily standup reminder", "triage backlog"), the orchestrator MUST **hand the run payload directly to PMA** per the same handoff rules and context as in [cynode_pma.md](cynode_pma.md) (user, project, thread, schedule id, run id).
  **PMA creates the task and starts the workflow internally**; there is no separate "enqueue workflow start" step.
  The scheduler does not create the task or enqueue a workflow start; PMA owns task creation and workflow start for those runs.
  PMA may use MCP (e.g. sandbox tools, scheduler MCP tools) to carry out work; when PMA or the workflow engine enqueues **concrete** jobs, the scheduler still uses the same node selection and job-dispatch contracts to dispatch them.
- When the payload is a **pre-specified job** (e.g. concrete script/command/sandbox spec with no interpretation needed), the scheduler MAY enqueue it for **direct dispatch** using the same node selection and job-dispatch contracts, without handing off to PMA.

Schedule payload types (e.g. `task` / `prompt` vs `job_spec`) that determine "requires interpretation" are defined in the User API Gateway or a dedicated scheduler API spec so that routing is deterministic and implementable.
See [User API Gateway - Scheduler and cron](user_api_gateway.md#spec-cynai-usrgwy-corecapabilities).

See job dispatch and node selection in [`docs/tech_specs/worker_node.md`](worker_node.md), the roadmap in [`docs/tech_specs/_main.md`](_main.md), and [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md).

## Project Manager Agent

The Project Manager Agent (PMA) is an agent runtime that continuously drives work to completion for a **session binding**; the orchestrator provisions **one `cynode-pma` managed service instance per session binding** (see [CYNAI.ORCHES.PmaInstancePerSessionBinding](orchestrator_bootstrap.md#spec-cynai-orches-pmainstancepersessionbinding)).
In this system, each PMA instance is a **worker-managed service container** and the product is always required to support PMA; disabling PMA is not supported.

- Reads tasks and their acceptance criteria from the database.
- Retrieves user preferences and standards from the database and applies them during planning and verification.
- Assigns work to worker nodes, monitors progress, and requests remediation when results fail checks.
- Continuously updates task state in PostgreSQL so the system remains resumable and auditable.
- Eagerly spawns Project Analyst sub-agents for task-scoped monitoring and verification whenever possible.

See [`docs/tech_specs/project_manager_agent.md`](project_manager_agent.md), [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md), and [`docs/tech_specs/user_preferences.md`](user_preferences.md).

Orchestrator-side agents MAY use external AI providers for planning and verification when policy allows it.

## OpenAI-Compatible Interactive Chat Routing

- Spec ID: `CYNAI.ORCHES.OpenAIInteractiveChatRouting` <a id="spec-cynai-orches-openaiinteractivechatrouting"></a>

The orchestrator is the single request-routing owner for the gateway's OpenAI-compatible interactive chat surfaces.
This applies to both `POST /v1/chat/completions` and `POST /v1/responses`.

### OpenAI-Compatible Interactive Chat Routing Traces To

- [REQ-ORCHES-0131](../requirements/orches.md#req-orches-0131)
- [REQ-ORCHES-0132](../requirements/orches.md#req-orches-0132)
- [REQ-ORCHES-0162](../requirements/orches.md#req-orches-0162)
- [REQ-ORCHES-0170](../requirements/orches.md#req-orches-0170)
- [REQ-ORCHES-0171](../requirements/orches.md#req-orches-0171)
- [REQ-ORCHES-0172](../requirements/orches.md#req-orches-0172)

### OpenAI-Compatible Interactive Chat Routing Behavior

- The orchestrator MUST apply the same policy, audit, redaction, timeout, and bounded-retry rules to both interactive chat endpoints.
- The orchestrator MUST derive one effective model identifier from the request `model` field when present and non-empty, otherwise `cynodeai.pm`.
- When the effective model identifier is `cynodeai.pm`, the orchestrator MUST route the request to PMA using only the worker-mediated PMA endpoint reported by the worker.
- When the effective model identifier is not `cynodeai.pm`, the orchestrator MUST route the request to the configured direct-inference path and MUST NOT invoke PMA for that request.
- Endpoint-specific request and response shape differences are owned by the gateway compatibility layer, but the orchestrator-owned routing and policy decisions MUST remain consistent across both interactive chat surfaces.

### Streaming Relay to PMA

When the client requests `stream=true` and the effective model is `cynodeai.pm`, the orchestrator MUST forward `stream=true` to PMA and MUST consume PMA's NDJSON streaming output for real-time SSE relay.
The orchestrator is responsible for:

- Opening a streaming connection to PMA and passing PMA's NDJSON events through the gateway handler goroutine, which maps each event to the per-endpoint SSE format (see [CYNAI.USRGWY.OpenAIChatApi.StreamingPerEndpointSSEFormat](openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streamingperendpointsseformat)).
- Maintaining three `strings.Builder` accumulators (visible text, thinking content, tool-call content) and appending every relayed event to the corresponding accumulator.
- Relaying PMA overwrite events as `cynodeai.amendment` SSE events and applying them to its own accumulators (per-iteration and per-turn scopes).
- Running the authoritative post-stream secret scanner on all three accumulators before emitting the terminal `[DONE]` event (see [CYNAI.USRGWY.OpenAIChatApi.StreamingRedactionPipeline](openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streamingredactionpipeline)).
- Persisting all three content types (visible text, thinking, tool-call) as part of the structured assistant turn, using only the redacted versions.

The former `emitContentAsSSE` function (which simulated streaming by splitting a complete payload into 48-rune chunks with no inter-chunk delay) MUST be removed entirely.
It violates the streaming spec by delivering zero progressive feedback and producing fake chunks that arrive within milliseconds of each other.

When PMA signals that it cannot provide real token streaming (non-streaming JSON response, streaming wrapper failure, or configuration toggle), the orchestrator MUST use the heartbeat fallback path: emit periodic `cynodeai.heartbeat` SSE events, then deliver the full response as a single visible-text delta followed by `[DONE]` (see [CYNAI.USRGWY.OpenAIChatApi.StreamingHeartbeatFallback](openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streamingheartbeatfallback)).

PMA NDJSON event types and SSE streaming types used in this relay MUST be defined as shared types in `go_shared_libs/contracts/userapi/` so that PMA and the orchestrator use identical wire-format structs (see [Shared Streaming Contract Types](#shared-streaming-contract-types)).

### Shared Streaming Contract Types

- Spec ID: `CYNAI.ORCHES.SharedStreamingContractTypes` <a id="spec-cynai-orches-sharedstreamingcontracttypes"></a>

The PMA NDJSON event types and the orchestrator's SSE event types cross a network boundary between two separately compiled Go modules (PMA in `agents/` and the orchestrator in `orchestrator/`).
Both sides MUST use identical struct definitions for serialization and deserialization.

The following shared types MUST be defined in `go_shared_libs/contracts/userapi/`:

- **PMA NDJSON event envelope:** a tagged-union struct (or set of structs) representing `delta`, `thinking`, `iteration_start`, `tool_call`, `tool_progress`, `overwrite`, and `done` events as defined in [CYNAI.PMAGNT.PMAStreamingNDJSONFormat](cynode_pma.md#spec-cynai-pmagnt-pmastreamingndjsonformat).
- **Overwrite event payload:** a struct with `Content`, `Reason`, `Scope` (`"iteration"` or `"turn"`), and optional `Iteration` and `Kinds` fields.
- **Extended SSE event types for `cynodeai.*` events:** structs for `cynodeai.thinking_delta`, `cynodeai.tool_call`, `cynodeai.tool_progress`, `cynodeai.amendment`, `cynodeai.heartbeat`, and `cynodeai.iteration_start` payloads.

The existing `ChatCompletionChunk` and related types in `go_shared_libs/contracts/userapi/userapi.go` handle standard OpenAI CC streaming; the new types extend them for CyNodeAI-specific streaming events.

#### Shared Streaming Contract Types Traces To

- [REQ-ORCHES-0170](../requirements/orches.md#req-orches-0170)

### Responses Continuation State

- Spec ID: `CYNAI.ORCHES.ResponsesContinuationState` <a id="spec-cynai-orches-responsescontinuationstate"></a>

#### Responses Continuation State Traces To

- [REQ-ORCHES-0165](../requirements/orches.md#req-orches-0165)
- [REQ-ORCHES-0166](../requirements/orches.md#req-orches-0166)

For `POST /v1/responses`, the orchestrator MUST support retained continuation state for `previous_response_id` without changing CyNodeAI's canonical thread and message ownership model.

#### Responses Continuation State Behavior

- The orchestrator MUST resolve `previous_response_id` only against retained response metadata owned by the authenticated user and scoped to the effective project for the request.
- The orchestrator MUST NOT use cross-user, cross-project, missing, or expired response references as continuation state.
- When a valid `previous_response_id` is present, the orchestrator MUST reconstruct the prior conversation state needed for routing and PMA or inference handoff while preserving the current user input as the newest turn.
- The orchestrator MUST persist retained response metadata sufficient to support later continuation while the response remains within retention.
- This retained response metadata is continuation state for the OpenAI-compatible `responses` surface; it MUST NOT replace CyNodeAI chat threads and messages as the canonical persisted user-visible conversation history.

### Chat File Upload Flow

- Spec ID: `CYNAI.ORCHES.ChatFileUploadFlow` <a id="spec-cynai-orches-chatfileuploadflow"></a>

#### Chat File Upload Flow Traces To

- [REQ-ORCHES-0167](../requirements/orches.md#req-orches-0167)
- [REQ-ORCHES-0168](../requirements/orches.md#req-orches-0168)

When the gateway accepts user-message file references under the OpenAI-compatible chat contract, the orchestrator is responsible for preserving that accepted file context through routing, history replay, and PMA or direct-inference handoff.

#### Chat File Upload Flow Behavior

- The orchestrator MUST accept either stable file identifiers or already-resolved inline file payloads from the gateway compatibility layer.
- The orchestrator MUST resolve each accepted file reference to content or to a stable downstream representation before invoking PMA or direct inference.
- The orchestrator MUST preserve the association between the file context and the originating user turn.
- When reconstructing history from chat-thread messages or retained `previous_response_id` continuation state, the orchestrator MUST include any required user-file context for prior turns instead of silently dropping it.
- The orchestrator MUST apply the same size, type, and redaction constraints to replayed file context that applied when the gateway first accepted the file.
- The orchestrator MUST preserve the same user and project-scoped authorization boundary that applied to the original uploaded file and MUST NOT broaden access when replaying or resolving file context for a shared-project thread.

## Managed Services (Worker-Managed)

- Spec ID: `CYNAI.ORCHES.ManagedServicesWorkerManaged` <a id="spec-cynai-orches-managedservices"></a>

This section defines how the orchestrator manages and tracks worker-managed long-lived services such as PMA.

### Managed Services Definitions

- **Managed service**: A long-lived service container started and supervised by a worker node based on orchestrator-delivered desired state.
- **Desired state**: Orchestrator intent delivered via node configuration (`managed_services`).
- **Observed state**: Worker-reported service state and endpoints (`managed_services_status`).

### Managed Services Behavior

- The orchestrator MUST express managed services as desired state in node configuration payloads.
  See [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md) `node_configuration_payload_v1` `managed_services`.
- When the orchestrator includes an agent token in managed service desired state, the token is delivered to the **worker** for the worker proxy to hold and use when forwarding agent requests; the token MUST NOT be given to the agent.
  For **PMA**, MCP credentials are **per authenticated user session**; the orchestrator MUST associate each credential with **user_id**, **invocation class** (`user_gateway_session` vs `orchestrator_initiated`), and session binding metadata so the gateway can enforce user scope and policy; see [CYNAI.MCPGAT.PmaSessionTokens](mcp/mcp_gateway_enforcement.md#spec-cynai-mcpgat-pmasessiontokens) and [CYNAI.MCPGAT.PmaInvocationClass](mcp/mcp_gateway_enforcement.md#spec-cynai-mcpgat-pmainvocationclass).
  For **user-scoped** managed services other than PMA (e.g. PAA), the orchestrator MUST associate that token with the user on whose behalf the agent is acting, so the gateway can resolve user context for preferences, access control, and audit attribution.
  Traces To: [REQ-ORCHES-0163](../requirements/orches.md#req-orches-0163), [REQ-ORCHES-0186](../requirements/orches.md#req-orches-0186), [REQ-WORKER-0164](../requirements/worker.md#req-worker-0164).
- The orchestrator MUST track observed state from worker capability reports and MUST treat service endpoints as dynamic.
  See [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md) `node_capability_report_v1` `managed_services_status`.
- When a worker reports `managed_services_status.services[].agent_to_orchestrator_proxy`, the orchestrator SHOULD ingest and store the worker-generated identity-bound agent-to-orchestrator proxy endpoints.
  Those endpoints are UDS-only (see [Unified UDS Path](worker_node.md#spec-cynai-worker-unifiedudspath)); the managed agent runtime uses them to call worker internal proxy operations without holding credentials.
  The orchestrator MAY use these fields for diagnostics and reconciliation when directing managed services.
- The orchestrator MUST route traffic to managed services using the worker-mediated endpoint(s) reported by the worker.
  The orchestrator MUST NOT rely on compose DNS or direct host-port addressing for managed services.
- The orchestrator MUST consider PMA online only when a recent worker report indicates `state=ready` for the PMA service instance.
- The orchestrator MUST reconcile service placement:
  - If the selected hosting node is unavailable or not reporting, the orchestrator MUST select a replacement eligible node and deliver updated desired state.
- The orchestrator MUST ensure PMA has the required bootstrap information in desired state:
  inference connectivity mode and details, and worker-proxy URLs for agent-to-orchestrator communication (MCP, callbacks).
  When supported by the worker, the orchestrator MAY set those worker-proxy URL fields to the sentinel value `auto` to require the worker to generate identity-bound endpoints and report them in `managed_services_status`.

### Greedy PMA Provisioning on User Login

- Spec ID: `CYNAI.ORCHES.PmaGreedyProvisioningOnLogin` <a id="spec-cynai-orches-pmagreedyprovisioningonlogin"></a>

When a user **authenticates** through the User API Gateway and obtains an interactive session, the orchestrator MUST **greedily** provision PMA for that user's **session binding** (per [CYNAI.ORCHES.PmaInstancePerSessionBinding](orchestrator_bootstrap.md#spec-cynai-orches-pmainstancepersessionbinding)): start or ensure the corresponding managed service instance, push node configuration, and issue PMA MCP credentials with invocation class **user_gateway_session** per [CYNAI.MCPGAT.PmaInvocationClass](mcp/mcp_gateway_enforcement.md#spec-cynai-mcpgat-pmainvocationclass).

Normative scope

- Applies to **all** gateway client surfaces that establish or resume an authenticated interactive session, including **cynork**, **Web Console**, **messaging connector** sessions, and other first-party clients using the User API Gateway.
- The orchestrator MUST NOT defer PMA instance startup until the first `model=cynodeai.pm` request or other PMA-consuming call **solely** because no chat message has been sent yet.

#### Greedy PMA Provisioning on User Login Requirements Traces

- [REQ-ORCHES-0190](../requirements/orches.md#req-orches-0190)
- [REQ-USRGWY-0161](../requirements/usrgwy.md#req-usrgwy-0161)

### Stale PMA Lifecycle and Teardown

- Spec ID: `CYNAI.ORCHES.StalePmaTeardown` <a id="spec-cynai-orches-stalepmateardown"></a>

The orchestrator provisions **one PMA instance per session binding**; it MUST also **reclaim** resources when a binding is no longer valid.

Normative behavior

- The orchestrator MUST identify **stale** PMA instances using policy that includes at least: **session ended**, **user logout**, **idle timeout**, **credential expiry** or **rotation supersession** with no active replacement for that binding, and **orphaned** instances when clients disconnect per product rules.
- The orchestrator MUST update **desired state** so workers **stop** removed PMA managed services (remove `service_id` from `managed_services.services[]` or set an explicit stopped/removed directive per worker contract) and MUST **invalidate** associated PMA MCP credentials at the gateway.
- The orchestrator SHOULD run reconciliation on a **periodic** schedule and on relevant **session lifecycle** events so teardown is timely.

#### Stale PMA Lifecycle and Teardown Requirements Traces

- [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191)
- [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)

### Managed Services Related Documents

- [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md) (readiness and PMA startup)
- [`docs/tech_specs/worker_node.md`](worker_node.md) (managed service containers and worker proxy bidirectional)
External provider calls MUST use API Egress and SHOULD use agent-specific routing preferences.
See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md) and [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).

### Project Manager Model (Startup Selection and Warmup)

- Spec ID: `CYNAI.ORCHES.ProjectManagerModelStartup` <a id="spec-cynai-orches-projectmanagermodelstartup"></a>

The orchestrator MUST select an effective "Project Manager model" on startup to run the Project Manager Agent.
This selection is distinct from where sandbox jobs run.

#### `Orchestrator.SelectProjectManagerModel` Operation

- Spec ID: `CYNAI.ORCHES.Operation.SelectProjectManagerModel` <a id="spec-cynai-orches-operation-selectprojectmanagermodel"></a>

##### 1 `Orchestrator.SelectProjectManagerModel` Traces To

- [REQ-ORCHES-0117](../requirements/orches.md#req-orches-0117)
- [REQ-MODELS-0004](../requirements/models.md#req-models-0004)
- [REQ-MODELS-0005](../requirements/models.md#req-models-0005)
- [REQ-MODELS-0008](../requirements/models.md#req-models-0008)

##### 2 `Orchestrator.SelectProjectManagerModel` Selection Scope

- the Project Manager inference execution target (local node vs external provider routing path), and
- the effective Project Manager model reference to run on that target.

##### 3 `Orchestrator.SelectProjectManagerModel` Inputs

- System settings:
  - `agents.project_manager.model.selection.execution_mode` (string)
    - `auto` (default)
    - `force_local`
    - `force_external`
  - `agents.project_manager.model.selection.mode` (string)
    - `auto_sliding_scale` (default)
    - `fixed_model`
  - `agents.project_manager.model.selection.prefer_orchestrator_host` (boolean, default true)
  - `agents.project_manager.model.local_default_ollama_model` (string, optional; when set, pins the local PM model name)
- Current node inventory and state:
  - Dispatchable worker nodes that report local inference support.
  - Each node's latest capability report fields:
    - `node.labels[]`
    - `compute.cpu_cores`, `compute.ram_mb`
    - `gpu.devices[].vram_mb` (when present)
  - Model availability for each candidate node (loaded vs not loaded), and the ability to request a model load.
- External routing configuration and policy (when local inference is unavailable or cannot satisfy selection).

##### 4 `Orchestrator.SelectProjectManagerModel` Outputs

- An effective selection tuple:
  - `execution_mode`: `local` or `external`
  - `local_node_slug`: string (required when `execution_mode=local`)
  - `model_ref`: string (local model name, or external provider model identifier)
  - `selection_reason`: an ordered list of machine-readable reason codes (for audit/logging)

##### 5 `Orchestrator.SelectProjectManagerModel` Behavior

The orchestrator selects exactly one effective Project Manager model on startup.
The required procedure is defined in the [Orchestrator.SelectProjectManagerModel Algorithm](#algo-cynai-orches-operation-selectprojectmanagermodel).

##### 6 `Orchestrator.SelectProjectManagerModel` Determinism Requirements

- All tie-breaks in this operation are resolved lexicographically by `node_slug` (ascending).
- If a required system setting key is unset, this operation uses the default value specified in Inputs.

##### 7 `Orchestrator.SelectProjectManagerModel` Definitions

- A node is considered "on the same host as the orchestrator" if its capability report `node.labels` contains the literal label `orchestrator_host`.
- `vram_total_mb` for a node is computed as:
  - sum of all present `gpu.devices[].vram_mb` values, ignoring devices that omit `vram_mb`
  - if no `vram_mb` values are present, `vram_total_mb=0`

##### 8 `Orchestrator.SelectProjectManagerModel` Error Conditions

- If `agents.project_manager.model.selection.mode=fixed_model` and `agents.project_manager.model.local_default_ollama_model` is unset, selection fails.
- If `execution_mode=local` is selected and no local candidate model can be loaded successfully, selection fails unless `execution_mode=external` is allowed and configured.
- If selection fails and the orchestrator does not currently have an online Project Manager model, the orchestrator MUST continue to re-run selection when relevant inputs change until a Project Manager model is online.

##### 9 `Orchestrator.SelectProjectManagerModel` Ordering and Determinism

Selection proceeds in the strict order defined by the [Orchestrator.SelectProjectManagerModel Algorithm](#algo-cynai-orches-operation-selectprojectmanagermodel).

##### 10 `Orchestrator.SelectProjectManagerModel` Algorithm

<a id="algo-cynai-orches-operation-selectprojectmanagermodel"></a>

1. Enumerate dispatchable local inference nodes. <a id="algo-cynai-orches-operation-selectprojectmanagermodel-step-1"></a>
   - Candidate set is all registered, dispatchable worker nodes that report inference supported/enabled.
2. Select `execution_mode`. <a id="algo-cynai-orches-operation-selectprojectmanagermodel-step-2"></a>
   - If `agents.project_manager.model.selection.execution_mode=force_external`:
     - Set `execution_mode=external` if an external routing path is configured and allowed.
     - Otherwise, fail selection (external execution forced but no eligible external routing path exists).
   - Else if `agents.project_manager.model.selection.execution_mode=force_local`:
     - If the candidate set is non-empty, set `execution_mode=local`; otherwise fail selection (local execution forced but no dispatchable local inference worker exists).
   - Else:
     - If the candidate set is non-empty, set `execution_mode=local`.
     - Otherwise, set `execution_mode=external` if an external routing path is configured and allowed.
     - Otherwise, fail selection (no inference-capable path exists).
3. If `execution_mode=external`, select the external routing path and `model_ref` using the configured external routing preferences and return. <a id="algo-cynai-orches-operation-selectprojectmanagermodel-step-3"></a>
4. Select the local target node `local_node_slug`. <a id="algo-cynai-orches-operation-selectprojectmanagermodel-step-4"></a>
   - If `agents.project_manager.model.selection.prefer_orchestrator_host=true` and one or more candidate nodes contain label `orchestrator_host`:
     - Choose the lexicographically smallest `node_slug` among those labeled nodes.
   - Otherwise:
     - For each candidate node, compute `vram_total_mb`.
     - Choose the node with the largest `vram_total_mb`.
     - If there is a tie, choose the lexicographically smallest `node_slug`.
5. Select the effective local `model_ref`. <a id="algo-cynai-orches-operation-selectprojectmanagermodel-step-5"></a>
   - If `agents.project_manager.model.selection.mode=fixed_model`:
     - Set `model_ref` to the value of `agents.project_manager.model.local_default_ollama_model`.
   - Else if `agents.project_manager.model.local_default_ollama_model` is set:
     - Set `model_ref` to the pinned value.
   - Else compute a deterministic ordered candidate list based on `vram_total_mb` for the selected node:
     - If `vram_total_mb >= 24000`:
       - Candidates (in order): `qwen3.5:35b`, `qwen2.5:32b`, `qwen2.5:14b`, `qwen3.5:9b`, `qwen3.5:0.8b`.
     - Else if `vram_total_mb >= 16000`:
       - Candidates (in order): `qwen3.5:9b`, `qwen2.5:14b`, `qwen3.5:0.8b`.
     - Else if `vram_total_mb >= 8000`:
       - Candidates (in order): `qwen3.5:9b`, `qwen3.5:0.8b`.
     - Else:
       - Candidates (in order): `qwen3.5:0.8b`.
     - Select the first candidate model that the orchestrator can satisfy by:
       - detecting it is already loaded on the selected node, or
       - successfully requesting the node load it (via the model load workflow).
6. If no local candidate can be satisfied: <a id="algo-cynai-orches-operation-selectprojectmanagermodel-step-6"></a>
   - If external routing is configured and allowed, set `execution_mode=external` and select an external `model_ref`.
   - Otherwise, fail selection.

#### `Orchestrator.WarmupProjectManagerModel` Rule

- Spec ID: `CYNAI.ORCHES.Rule.WarmupProjectManagerModel` <a id="spec-cynai-orches-rule-warmupprojectmanagermodel"></a>

Traces To: [REQ-ORCHES-0117](../requirements/orches.md#req-orches-0117)

This Spec Item defines the required startup warmup behavior after a local Project Manager model selection has been made.

##### 1 `Orchestrator.WarmupProjectManagerModel` Outcomes

- If `execution_mode=local`, the orchestrator transitions to ready state only after the selected local `model_ref` is reported as loaded and available by the selected node.
- If `execution_mode=external`, warmup does not require a local model load.

##### 2 `Orchestrator.WarmupProjectManagerModel` Algorithm

<a id="algo-cynai-orches-rule-warmupprojectmanagermodel"></a>

1. If `execution_mode=external`, skip local warmup and proceed with startup. <a id="algo-cynai-orches-rule-warmupprojectmanagermodel-step-1"></a>
2. If `execution_mode=local`, require `node_model_availability.status=available` for (`local_node_slug`, `model_ref`). <a id="algo-cynai-orches-rule-warmupprojectmanagermodel-step-2"></a>
3. If the model is not yet available, request the selected node load `model_ref` and block readiness until: <a id="algo-cynai-orches-rule-warmupprojectmanagermodel-step-3"></a>
   - the node reports `available`, or
   - the node reports `failed` (in which case the orchestrator re-runs `Orchestrator.SelectProjectManagerModel` and retries warmup), or
   - prerequisites are still missing (in which case the orchestrator remains not ready and continues to wait and re-evaluate).
4. While the orchestrator does not have an online Project Manager model, it MUST re-run `Orchestrator.SelectProjectManagerModel` and retry warmup when any of the following occur: <a id="algo-cynai-orches-rule-warmupprojectmanagermodel-step-4"></a>
   - a system-scoped Project Manager model selection setting changes (for example `agents.project_manager.model.local_default_ollama_model`, `agents.project_manager.model.selection.mode`, `agents.project_manager.model.selection.prefer_orchestrator_host`)
   - a node registers, becomes dispatchable, or updates its capability report
   - a node model availability state changes for any candidate Project Manager model on any dispatchable local inference node

#### `Orchestrator.MonitorProjectManagerModel` Rule

- Spec ID: `CYNAI.ORCHES.Rule.MonitorProjectManagerModel` <a id="spec-cynai-orches-rule-monitorprojectmanagermodel"></a>

Traces To: [REQ-ORCHES-0129](../requirements/orches.md#req-orches-0129)

This Spec Item defines continuous monitoring of the selected Project Manager model after startup.
The goal is to ensure that readiness reflects the current availability of the Project Manager model and that the orchestrator reacts deterministically to node loss and relevant system setting changes.

Triggers (non-exhaustive)

The orchestrator MUST re-validate the currently selected Project Manager model when any of the following occur:

- The selected local node becomes non-dispatchable (for example offline, drained, or lease-expired).
- The selected local node capability report changes (for example inference disabled).
- The selected local node reports the Project Manager model is no longer available (evicted, failed, or unknown).
- Any Project Manager model selection system setting changes:
  - `agents.project_manager.model.selection.execution_mode`
  - `agents.project_manager.model.selection.mode`
  - `agents.project_manager.model.selection.prefer_orchestrator_host`
  - `agents.project_manager.model.local_default_ollama_model`
- External routing configuration or policy changes that affect whether an external execution path is configured and allowed.

##### 1 `Orchestrator.MonitorProjectManagerModel` Required Behavior

- If the orchestrator is in a ready state and the Project Manager model becomes unavailable, the orchestrator MUST transition out of ready state.
- While not ready, the orchestrator MUST continue to serve the management surfaces required to become ready (for example system settings and credential configuration).
- When the Project Manager model becomes unavailable or when relevant inputs change, the orchestrator MUST re-run `Orchestrator.SelectProjectManagerModel` and apply `Orchestrator.WarmupProjectManagerModel` until a Project Manager model is online again.
- If the orchestrator must restart the Project Manager Agent due to a model availability change, it MUST restore the agent state from PostgreSQL so task progress remains resumable.

##### 2 `Orchestrator.MonitorProjectManagerModel` Note

- For the MVP, the Project Manager model is responsible for all inference task assignment decisions.
  See [REQ-ORCHES-0119](../requirements/orches.md#req-orches-0119) and [`docs/tech_specs/project_manager_agent.md`](project_manager_agent.md).

## API Egress Server

The orchestrator provides controlled external API access through an API Egress Server.
This prevents API keys from being exposed to sandbox containers and enables policy and auditing.

See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).

## Web Egress Proxy

The orchestrator provides controlled sandbox web egress through a Web Egress Proxy.
This enables builds and verification steps that require dependency downloads without granting sandboxes direct, unrestricted internet access.

See [`docs/tech_specs/web_egress_proxy.md`](web_egress_proxy.md).

## Secure Browser Service

The orchestrator provides controlled web browsing through a Secure Browser Service.
This enables agents to retrieve web information without granting direct network access to sandbox containers.

See [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md).

## External Model Routing

The orchestrator can route model calls to configured external AI APIs when local workers are overloaded or lack required capabilities.
External calls MUST use the API Egress Server so credentials are not exposed to agents or sandbox containers.

See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md).

## Model Management

The orchestrator maintains a model registry and a local model cache that worker nodes can load from.
This enables consistent model availability and reduces node internet dependencies.

See [`docs/tech_specs/model_management.md`](model_management.md).

## User API Gateway

The orchestrator exposes a single user-facing API endpoint that surfaces its capabilities to external clients.
This is intended for UIs and integrations such as Open WebUI and messaging services.

OpenAI-compliant chat completions are processed by the orchestrator first: the orchestrator performs automatic sanitization, logging, and persistence, then either hands off to the PM agent (`cynode-pma`) when the model is `cynodeai.pm`, or routes to the selected inference model (node-local or external API via API Egress) when another model is selected.
See [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-routingpath).

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md) and [`docs/tech_specs/data_rest_api.md`](data_rest_api.md).

### Task Create Handoff

- Spec ID: `CYNAI.ORCHES.Rule.TaskCreateHandoff` <a id="spec-cynai-orches-rule-taskcreatehandoff"></a>

For task create via the User API Gateway, the orchestrator MUST (1) create the task record with `planning_state=draft`; (2) route the task to the Project Manager Agent for **review** (not for immediate execution); (3) return `201 Created` with the task response (including `task_id` and `planning_state=draft`) immediately.

The create HTTP handler MUST NOT start workflow execution; workflow execution starts only after the task is transitioned to `planning_state=ready` (e.g. by PMA after review and enrichment, or via an explicit ready transition).
See [REQ-ORCHES-0176](../requirements/orches.md#req-orches-0176), [REQ-ORCHES-0177](../requirements/orches.md#req-orches-0177), [Request Source and Orchestrator Handoff](cynode_pma.md#spec-cynai-pmagnt-requestsource), and [Task review and ready transition](project_manager_agent.md#spec-cynai-agents-taskreviewandreadytransition).

#### Task Create Handoff Traces To

- [REQ-ORCHES-0122](../requirements/orches.md#req-orches-0122)
- [REQ-ORCHES-0176](../requirements/orches.md#req-orches-0176)
- [REQ-ORCHES-0177](../requirements/orches.md#req-orches-0177)

## Sandbox Image Registry

The orchestrator uses a configurable rank-ordered list of sandbox container image registries; when none is configured, worker nodes pull sandbox images from Docker Hub (`docker.io`) only.
Allowed sandbox images and their capabilities are tracked in PostgreSQL so tasks can request safe, appropriate execution environments.

See [`docs/tech_specs/sandbox_image_registry.md`](sandbox_image_registry.md).

## Node Bootstrap and Configuration

The orchestrator MUST be able to configure worker nodes at registration time.
This includes distributing the correct endpoints, certificates, and pull credentials for orchestrator-provided services.
The orchestrator MUST support dynamic configuration updates after registration and must ingest node capability reports on registration and node startup.

Config delivery

- The orchestrator exposes the node config URL in the bootstrap payload (`node_config_url` in `node_bootstrap_payload_v1`).
- GET on that URL returns `node_configuration_payload_v1` for the authenticated node.
- POST on that URL accepts `node_config_ack_v1` and persists the acknowledgement; see [Nodes Table](#spec-cynai-schema-nodestable) columns `config_ack_at`, `config_ack_status`, `config_ack_error`.
- Endpoint paths are not mandated here; the bootstrap payload carries the concrete URLs so nodes do not rely on hard-coded paths.

### Job Dispatch

- For the initial single-node implementation, the orchestrator dispatches jobs to the Worker API via direct HTTP.
- The dispatcher uses the per-node `worker_api_target_url` and per-node bearer token; the URL is normally set from the node-reported `worker_api.base_url` at registration and when processing capability reports, and may be overridden by operator config (e.g. same-host: `WORKER_API_TARGET_URL`); see [Nodes Table](#spec-cynai-schema-nodestable) and [`worker_node_payloads.md`](worker_node_payloads.md).
- The MCP gateway is not in the loop for this job-dispatch path.

See [`docs/tech_specs/worker_node.md`](worker_node.md) and [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md).

## Postgres Schema

- Spec ID: `CYNAI.SCHEMA.TasksJobsNodes` <a id="spec-cynai-schema-tasksjobsnodes"></a>

The orchestrator owns task state, job execution records, the node registry, and capability snapshots in PostgreSQL.

**Schema definitions (index):** See [Tasks, Jobs, and Nodes](postgres_schema.md#spec-cynai-schema-tasksjobsnodes) in [`postgres_schema.md`](postgres_schema.md).

### Task vs Job (Terminology)

- Spec ID: `CYNAI.SCHEMA.TaskVsJob` <a id="spec-cynai-schema-taskvsjob"></a>

Canonical definitions so specs and implementations use consistent language.

- **Task:** A **durable work item** owned by the orchestrator and stored in the `tasks` table.
  A task is the unit of work that users and the PMA create, assign to a plan, give a persona and optional recommended skills, and order with dependencies.
  It describes *what* to do and *who* does it (one persona per task).
  A task outlives any single execution; it can be run, retried, or reassigned.
- **Job:** A **single execution unit** dispatched to a worker, stored in the `jobs` table.
  A job is the runtime instance: "run this work on this node (and in this sandbox)."
  The job payload is what the worker and SBA actually execute (e.g. job spec with persona embedded inline, context, and for bundles task_ids plus embedded full task context so the job is self-contained).
  A job is created when work is dispatched and completes (or fails); the task record is the durable authority.
- **Relationship:** The **job builder** (orchestrator or PMA) turns one or more **tasks** into a **job spec** (resolve persona, merge skills, supply per-task context).
  One **task** may result in zero or more **jobs** over time (e.g. retries, reassignment).
  One **job** may reference one task (single-task job) or 1-3 tasks in order (bundle job); the SBA runs the job and executes each referenced task in sequence when it is a bundle.
  So: **task** = durable, persona-scoped work definition; **job** = one dispatched run of that work on a worker, carrying the resolved persona and context.

### Tasks Table

- Spec ID: `CYNAI.SCHEMA.TasksTable` <a id="spec-cynai-schema-taskstable"></a>

- `id` (uuid, pk)
- `created_by` (uuid, fk to `users.id`)
  - creating user; set from authenticated request context when created via the gateway; for system-created and bootstrap tasks, use the reserved system user
- `project_id` (uuid, fk to `projects.id`, nullable)
  - optional project association for RBAC, preferences, and grouping; null unless explicitly set by client or PM/PA
- `plan_id` (uuid, fk to `project_plans.id`, nullable)
  - when set, task belongs to this plan; workflow for this task is gated on plan state active and on task dependencies (see [Task Dependencies Table](#task-dependencies-table)).
- `planning_state` (text, NOT NULL)
  - Planning phase state; distinct from `status` (execution lifecycle).
  - Allowed values: `draft`, `ready`.
  - Initial value on create: `draft`.
  - Only tasks in `planning_state=ready` are eligible for workflow execution; see [REQ-ORCHES-0178](../requirements/orches.md#req-orches-0178).
- `persona_id` (uuid, fk to `personas.id`, nullable)
  - optional; when set, the job builder resolves this persona and embeds it in the job; at most one persona per task
- `recommended_skill_ids` (jsonb, nullable)
  - optional array of skill stable identifiers; job builder merges with persona default_skill_ids (union, task overrides duplicates) and resolves into context for the SBA
- `status` (text)
  - Task lifecycle status; stored separately from open/closed.
  - Values include: pending, running, completed, failed, canceled, superseded (see [Task status and closed state](#task-status-and-closed-state)).
- `closed` (boolean, not null)
  - Binary open/closed state; when true, the task is closed (no further work).
    Set consistently when status changes (for example `true` when status is completed, failed, canceled, superseded).
- `description` (text, nullable)
  - task description for user-facing display and editing; stored as Markdown (see [REQ-PROJCT-0114](../requirements/projct.md#req-projct-0114))
- `acceptance_criteria` (jsonb, nullable)
  - structured criteria used by Project Manager for verification; any text fields used for editable criteria are stored as Markdown
- `requirements` (jsonb, nullable)
  - optional array of **requirement** objects; see [Requirement object structure](#requirement-object-structure)
- `steps` (jsonb, NOT NULL)
  - required **map** of step objects keyed by numeric step ID (integer, stored as JSON number or string that parses to integer); non-empty.
    Keys define order: when read, steps are sorted by numeric key ascending to obtain deterministic order.
    When creating steps, assign IDs in increments of 10 (e.g. 10, 20, 30) so additional steps can be inserted between (e.g. 15) without renumbering.
    Each value is a **step** object: `complete` (boolean), `description` (string).
    Job builders or agents use the sorted sequence for job context or to-dos; executors set `complete: true` as steps finish.
    Structure may otherwise align with step-executor or SBA job step types (see [cynode_step_executor.md](cynode_step_executor.md), [cynode_sba.md](cynode_sba.md)).
- `summary` (text, nullable)
  - final summary written by workflow
- `post_execution_notes` (text, nullable)
  - Markdown notes added after task execution (e.g. by PMA, PAA, or user); for verification, handoff, or retrospective
- `comments` (jsonb, nullable)
  - same structure as plan comments; see [Comments Structure (Plans and Tasks)](projects_and_scopes.md#spec-cynai-schema-commentsstructure); may be updated by agents or users when plan is locked (per lock rules)
- `metadata` (jsonb, nullable)
- `archived_at` (timestamptz, nullable)
  - when non-null, the task is archived (soft-deleted); API/CLI "delete" sets this; archived tasks are excluded from default list views; retained for audit and history
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### Tasks Table Constraints

- Index: (`created_by`)
- Index: (`project_id`)
- Index: (`plan_id`)
- Index: (`persona_id`) where not null
- Index: (`planning_state`)
- Index: (`status`)
- Index: (`closed`)
- Index: (`archived_at`) where not null (for list filter by archived)
- Index: (`created_at`)

#### Task `planning_state` Migration

When adding `planning_state` to an existing deployment, assign a value to existing tasks:

- If `tasks.status` is `running`, `completed`, `failed`, `canceled`, or `superseded`, set `planning_state=ready`.
- If `tasks.status` is `pending`, set `planning_state=draft` or `ready` per deployment choice (for example treat existing pending tasks as already reviewed).

#### Requirement Object Structure

- Spec ID: `CYNAI.SCHEMA.RequirementObject` <a id="spec-cynai-schema-requirementobject"></a>

The `tasks.requirements` column holds a JSON array of **requirement** objects (or null).
Each object has:

- `ref` (string, optional): stable content reference for the requirement (e.g. `REQ-PROJCT-0122` or a task-local tag); used for display and sorting, not a database entity id
- `description` (string, required): the requirement statement; stored as Markdown
- `source` (string, optional): provenance (e.g. requirement document path, spec section, or external ref)
- `type` (string, optional): e.g. `functional`, `non_functional`, `constraint`, or host-defined
- `priority` (string or number, optional): e.g. `must` | `should` | `could`, or numeric 1-5; host-defined semantics

Order in the array is significant unless otherwise specified; consumers preserve order or sort by `ref` for display.

### Task Dependencies Table

- Spec ID: `CYNAI.SCHEMA.TaskDependenciesTable` <a id="spec-cynai-schema-taskdependenciestable"></a>

Stores explicit task-within-plan dependencies; execution order and runnability are determined solely by the dependency graph (prerequisite and dependent tasks).
**Multiple prerequisites:** A task may depend on **multiple** other tasks; the table stores one row per (dependent task, prerequisite task).
Thus `task_id` may appear in many rows with different `depends_on_task_id` values.
The orchestrator and PMA use this structure to resolve all prerequisites for a task and to surface runnable tasks (see below).
When a task is set to `canceled`, all tasks that depend on it (directly or transitively) are set to `canceled` automatically; see [REQ-ORCHES-0154](../requirements/orches.md#req-orches-0154) and [Cancel cascades to dependents](workflow_mvp.md#spec-cynai-orches-cancelcascadestodependents).
A task is **runnable** when all tasks it depends on have `status = 'completed'`; see [Project plan and task dependencies](workflow_mvp.md#spec-cynai-orches-workflowplanorder) and [REQ-ORCHES-0153](../requirements/orches.md#req-orches-0153).

- `id` (uuid, pk)
- `task_id` (uuid, fk to `tasks.id`, NOT NULL)
  - the dependent task (this task runs after its dependencies)
- `depends_on_task_id` (uuid, fk to `tasks.id`, NOT NULL)
  - one prerequisite that must reach status `completed` before `task_id` may run; multiple prerequisites for the same task are represented by multiple rows with the same `task_id`

#### Task Dependencies Table Constraints

- Unique: (`task_id`, `depends_on_task_id`)
- Check: `task_id != depends_on_task_id` (no self-deps)
- When `plan_id` is set for both tasks, the implementation ensures both tasks belong to the same plan (`tasks.plan_id` equal for both); optionally enforce via trigger or constraint.

#### Task Dependencies Indexes

- Index: (`task_id`) for "list all prerequisites of this task" (needed to decide if task is runnable)
- Index: (`depends_on_task_id`) for "list all tasks that depend on this one" (e.g. cascade cancel, or "what becomes runnable when this completes")

**Surfacing runnable tasks:** The orchestrator and PMA query tasks that can be executed (runnable) for a given plan.
A task is runnable when: (1) its plan's state is `active`, (2) the task is not closed, and (3) either the task has **no** rows in `task_dependencies` for this `task_id`, or **every** row for this `task_id` has `depends_on_task_id` pointing to a task with `status = 'completed'`.
Implementations support an efficient query (for example tasks in plan where not closed and either no dependency rows exist for that task_id, or the set of depends_on_task_id for that task_id is a subset of completed task ids).
The indexes above support "load dependencies of task" and "check all completed"; additional indexing (for example on `tasks.plan_id`, `tasks.closed`) supports "all runnable tasks for plan" patterns.

### Task Status and Closed State

- Spec ID: `CYNAI.SCHEMA.TaskStatusAndClosed` <a id="spec-cynai-schema-taskstatusandclosed"></a>

Task **status** is stored in `tasks.status` and represents the lifecycle state (e.g. pending, running, completed, failed, canceled, superseded).
Task **closed** is stored in `tasks.closed` (boolean): when true, the task is closed (no further work); when false, the task is open.
The system keeps `closed` consistent with `status` (for example set `closed = true` when status becomes completed, failed, canceled, or superseded).
Plan completion (set plan to completed) requires the plan to have at least one task and **all such tasks to have `closed = true`**; see [REQ-PROJCT-0121](../requirements/projct.md#req-projct-0121) and [Project plan state](projects_and_scopes.md#spec-cynai-access-projectplanstate).

### Jobs Table

- Spec ID: `CYNAI.SCHEMA.JobsTable` <a id="spec-cynai-schema-jobstable"></a>

- `id` (uuid, pk)
- `task_ids` (jsonb, NOT NULL)
  - map keyed by numeric order (e.g. 10, 20, 30), value = task uuid (string); single-task job = one key; bundle = 2-3 keys; execution order = sort keys ascending; same pattern as task steps
- `task_id` (uuid, fk to `tasks.id`, nullable)
  - deprecated or derived: when task_ids has a single key, may duplicate that task id for backward compatibility; prefer task_ids for new code
- `persona_id` (uuid, fk to `personas.id`, nullable)
  - optional; for indexing, reporting, and provenance; job payload carries inline `persona: { title, description }` for SBA consumption
- `node_id` (uuid, fk to `nodes.id`, nullable)
  - set when job is dispatched to a node
- `status` (text)
  - examples: queued, running, completed, failed, canceled, lease_expired
- `payload` (jsonb, nullable)
  - job input (e.g. command, image, env)
- `result` (jsonb, nullable)
  - job output and exit info
- `lease_id` (uuid, nullable)
  - idempotency / lease for retries and heartbeats
- `lease_expires_at` (timestamptz, nullable)
- `started_at` (timestamptz, nullable)
- `ended_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### Jobs Table Constraints

- Index: (`task_ids`) (GIN or application-level by first task id when single-task)
- Index: (`task_id`)
- Index: (`persona_id`) where not null
- Index: (`node_id`)
- Index: (`status`)
- Index: (`lease_id`) where not null
- Index: (`lease_expires_at`) where not null
- Index: (`created_at`)

For bundle jobs, the payload embeds full per-task context (`context.task_contexts` keyed by same numeric keys as `task_ids`) so the job is self-contained; see [cynode_sba.md](cynode_sba.md).

### Nodes Table

- Spec ID: `CYNAI.SCHEMA.NodesTable` <a id="spec-cynai-schema-nodestable"></a>

- `id` (uuid, pk)
- `node_slug` (text, unique)
  - stable identifier used in registration and scheduling (e.g. from node startup YAML)
- `status` (text)
  - examples: registered, active, inactive, drained
- `config_version` (text, nullable)
  - version of last applied node configuration payload
- `worker_api_target_url` (text, nullable)
  - URL of the node Worker API for job dispatch; normally set from the node-reported `worker_api.base_url` at registration and when processing capability reports; may be overridden by operator config (e.g. same-host override); see [`worker_node_payloads.md`](worker_node_payloads.md) and [`worker_node.md`](worker_node.md)
- `worker_api_bearer_token` (text, nullable)
  - bearer token for orchestrator-to-node Worker API auth; stored encrypted at rest or in a secrets backend; populated from config delivery
- `config_ack_at` (timestamptz, nullable)
  - time of last config acknowledgement from the node
- `config_ack_status` (text, nullable)
  - status of last config ack: e.g. applied, failed
- `config_ack_error` (text, nullable)
  - error message when config_ack_status is failed
- `last_seen_at` (timestamptz, nullable)
- `last_capability_at` (timestamptz, nullable)
- `metadata` (jsonb, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### Nodes Table Constraints

- Index: (`node_slug`)
- Index: (`status`)
- Index: (`last_seen_at`)

### Node Capabilities Table

- Spec ID: `CYNAI.SCHEMA.NodeCapabilitiesTable` <a id="spec-cynai-schema-nodecapabilitiestable"></a>

Stores the last reported capability payload (full JSON of actual capabilities) for scheduling and display.
The orchestrator stores the capability report JSON here (or a normalized snapshot that preserves the actual capabilities per worker_node.md).

- `id` (uuid, pk)
- `node_id` (uuid, fk to `nodes.id`)
- `reported_at` (timestamptz)
- `capability_snapshot` (jsonb)
  - full capability report JSON (identity, platform, compute, gpu, sandbox, network, inference, tls, etc. per worker_node_payloads.md)

#### Node Capabilities Table Constraints

- Unique: (`node_id`) (one row per node; overwrite on new report) or allow history (then index by `node_id`, `reported_at`)
- Index: (`node_id`)

Recommendation: one row per node, updated in place when capability report is received; alternatively, append-only with retention policy.

## MCP Tool Interface

The orchestrator adopts MCP as the standard tool interface for agents.
This enables a consistent tool protocol and role-based tool access.

See [`docs/tech_specs/mcp/mcp_tooling.md`](mcp/mcp_tooling.md).

## Workflow Engine

The orchestrator uses a Go-native workflow runner to implement multi-step and multi-agent workflows.
The Project Manager Agent's behavior is implemented by the [Workflow MVP](workflow_mvp.md).

The workflow engine is a **Go-native state machine** within the orchestrator process.
The orchestrator invokes it via a stable start/resume and checkpoint contract.
The workflow start/resume API contract and workflow start triggers are defined in [workflow_mvp.md](workflow_mvp.md): [Workflow Start/Resume API Contract](workflow_mvp.md#spec-cynai-orches-workflowstartresumeapi) and [Workflow Start Triggers](workflow_mvp.md#spec-cynai-orches-workflowstarttriggers).
Lease lifecycle is defined in [Task Workflow Lease Lifecycle](#task-workflow-lease-lifecycle) below.

**Process boundaries:** **cynode-pma** (chat, MCP tools) and the **workflow runner** (workflow graph execution) are **separate processes**.
They share the MCP gateway and DB.
The orchestrator starts the workflow runner for a given task; chat and planning requests go to PMA; the workflow runner executes the graph and does not serve chat.

**Single-active-workflow-per-task:** The lease that enforces only one active workflow per task is **held in the orchestrator DB** (table or row semantics defined in [workflow_mvp.md - Task Workflow Leases Table](workflow_mvp.md#spec-cynai-schema-taskworkflowleasestable)).
The workflow runner acquires or checks the lease via the orchestrator before running.

See [workflow_mvp.md](workflow_mvp.md) for the graph topology, state model, node behaviors, checkpoint schema, and wiring to orchestrator capabilities (MCP, Worker API, model routing).

### Task Workflow Lease Lifecycle

- Spec ID: `CYNAI.ORCHES.TaskWorkflowLeaseLifecycle` <a id="spec-cynai-orches-taskworkflowleaselifecycle"></a>

The orchestrator grants and releases the task workflow lease; the workflow runner acquires or checks it via the orchestrator API (see [workflow_mvp.md](workflow_mvp.md#spec-cynai-orches-workflowstartresumeapi)).
The lease table and columns are defined in [workflow_mvp.md - Task Workflow Leases Table](workflow_mvp.md#spec-cynai-schema-taskworkflowleasestable).

#### Task Workflow Lease Lifecycle - Acquire

The workflow runner calls the orchestrator (as part of the workflow start API or a dedicated lease endpoint) to acquire the lease for a `task_id`.
The request includes the workflow runner identity (`holder_id`).
The orchestrator grants the lease only if no other holder has it; otherwise it returns a defined error (e.g. 409 Conflict).
Idempotent acquire: if the same holder re-requests for the same `task_id` with the same `lease_id` (or equivalent), the orchestrator returns success without changing state.

#### Task Workflow Lease Lifecycle - Release

On normal workflow completion or failure, the workflow runner calls the orchestrator to release the lease (e.g. as part of a completion/failure report or a dedicated release operation).
The orchestrator updates the lease row so the task is no longer held.
If the workflow runner does not release (e.g. crash), the orchestrator may release on expiry (see Expiry).

#### Task Workflow Lease Lifecycle - Expiry

Each lease row has an `expires_at` (timestamptz).
The workflow runner may renew the lease before expiry (e.g. by sending a heartbeat or an explicit renew request that the orchestrator uses to extend `expires_at`).
If the workflow runner does not renew before `expires_at`, the orchestrator treats the lease as released and may allow another start for that task.

#### Task Workflow Lease Lifecycle Traces To

- [REQ-ORCHES-0146](../requirements/orches.md#req-orches-0146)

## Orchestrator Self-Metadata and Logging

This section defines metadata the orchestrator tracks about itself (artifact storage utilization, utilization windows for scheduling, scheduler health) and how it logs that information.
It also defines orchestrator-side redaction for SBA inference logs received from workers.

### Document and Traces

Normative definitions for orchestrator self-metadata follow.

#### `Doc` Orchestrator Self-Metadata Overview

- Spec ID: `CYNAI.ORCHES.Doc.OrchestratorSelfMetadata` <a id="spec-cynai-orches-doc-orchestratorselfmetadata"></a>

This subsection defines metadata the orchestrator MUST or SHOULD track about itself and its components, and how it MUST log that information for operational use.
Implementations use this metadata to schedule maintenance during low-utilization windows and to expose a concise view of orchestrator and storage health.

##### `Doc` Orchestrator Self-Metadata Overview Traces To

- [REQ-ORCHES-0100](../requirements/orches.md#req-orches-0100)
- [REQ-ORCHES-0101](../requirements/orches.md#req-orches-0101)
- [REQ-ORCHES-0105](../requirements/orches.md#req-orches-0105)
- [REQ-ORCHES-0127](../requirements/orches.md#req-orches-0127)
- [REQ-ORCHES-0167](../requirements/orches.md#req-orches-0167)
- [Orchestrator Artifacts Storage](orchestrator_artifacts_storage.md)
- [Orchestrator](#spec-cynai-orches-doc-orchestrator)

### Blob Storage Utilization

Artifact blob storage utilization types and rules follow.

#### `Type` Blob Storage Utilization Snapshot

- Spec ID: `CYNAI.ORCHES.Type.BlobStorageUtilizationSnapshot` <a id="spec-cynai-orches-type-blobstorageutilizationsnapshot"></a>

A snapshot of artifact blob storage utilization at a point in time.

- **`capacity_bytes`** (optional): Total usable capacity of the S3-like backend (or bucket) in bytes, when the backend exposes it.
- **`used_bytes`**: Total size of blob data stored in the artifact backend, in bytes (sum of artifact sizes in the database or trusted backend metric).
- **`artifact_count`**: Number of artifact blobs that contribute to `used_bytes`.
- **`collected_at`**: Timestamp (UTC) when the snapshot was collected.

The orchestrator MUST be able to produce this snapshot on demand and MAY periodically sample and store or log it.

##### `Type` Blob Storage Utilization Snapshot Traces To

- [REQ-ORCHES-0127](../requirements/orches.md#req-orches-0127)
- [REQ-ORCHES-0167](../requirements/orches.md#req-orches-0167)
- [Orchestrator Artifacts Storage - DB metadata](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsdbmetadata)

#### `Rule` Blob Storage Utilization Tracking

- Spec ID: `CYNAI.ORCHES.Rule.BlobStorageUtilizationTracking` <a id="spec-cynai-orches-rule-blobstorageutilizationtracking"></a>

The orchestrator MUST track artifact blob storage utilization for the S3-like backend used for artifacts.
The orchestrator SHOULD refresh this snapshot on a configurable interval or on demand so scheduling and cleanup logic can consider current utilization.
Collection failure MUST NOT block readiness or task dispatch (degraded metadata only).

##### `Rule` Blob Storage Utilization Tracking Traces To

- [Orchestrator Artifacts Storage](orchestrator_artifacts_storage.md)

### Utilization Windows and Downtime Scheduling

Utilization windows and scheduling rules follow.

#### `Type` Utilization Window

- Spec ID: `CYNAI.ORCHES.Type.UtilizationWindow` <a id="spec-cynai-orches-type-utilizationwindow"></a>

A time window with an associated utilization level for deciding when to run maintenance or low-priority jobs.

- **`start_at`**, **`end_at`**: Window boundaries (UTC).
- **`level`**: `high`, `medium`, or `low` (`low` is suitable for downtime-sensitive background work).
- **`metric`** (optional), **`value`** (optional): Metric used and numeric value for logging.

#### `Rule` Utilization-Based Downtime Scheduling

- Spec ID: `CYNAI.ORCHES.Rule.UtilizationBasedDowntimeScheduling` <a id="spec-cynai-orches-rule-utilizationbaseddowntimescheduling"></a>

The orchestrator MUST use utilization metadata to prefer running downtime-sensitive tasks (stale artifact cleanup, background hash updates, and similar) during low-utilization windows, with a maximum deferral bound so work eventually runs.

##### `Rule` Utilization-Based Downtime Scheduling Traces To

- [REQ-ORCHES-0100](../requirements/orches.md#req-orches-0100)
- [REQ-ORCHES-0101](../requirements/orches.md#req-orches-0101)
- [Stale Artifact Cleanup](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsstalecleanup)

#### `Operation` Record Utilization Windows

- Spec ID: `CYNAI.ORCHES.Operation.RecordUtilizationWindows` <a id="spec-cynai-orches-operation-recordutilizationwindows"></a>

The orchestrator MUST periodically record utilization samples and MUST be able to derive high/low windows from recent history for scheduling.

##### `Operation` Record Utilization Windows Traces To

- [REQ-ORCHES-0100](../requirements/orches.md#req-orches-0100)
- [REQ-ORCHES-0105](../requirements/orches.md#req-orches-0105)

### Component and Scheduler Summary

Summary types and scheduler metrics rules follow.

#### `Type` Orchestrator Self-Metadata Summary

- Spec ID: `CYNAI.ORCHES.Type.OrchestratorSelfMetadataSummary` <a id="spec-cynai-orches-type-orchestratorselfmetadatasummary"></a>

A concise summary for status or admin APIs: optional `blob_storage`, `utilization`, `scheduler`, and `components` fields.
Optional fields MAY be omitted when unsupported or when collection fails.

#### `Rule` Scheduler Metrics for Utilization

- Spec ID: `CYNAI.ORCHES.Rule.SchedulerMetricsForUtilization` <a id="spec-cynai-orches-rule-schedulermetricsforutilization"></a>

The orchestrator MUST expose at least one of active job count or queue depth to scheduling logic for utilization classification.

##### `Rule` Scheduler Metrics for Utilization Traces To

- [REQ-ORCHES-0100](../requirements/orches.md#req-orches-0100)
- [Task Scheduler](#spec-cynai-orches-taskscheduler)

### Self-Metadata Logging

Logging obligations for self-metadata follow.

#### `Rule` Self-Metadata Logging

- Spec ID: `CYNAI.ORCHES.Rule.SelfMetadataLogging` <a id="spec-cynai-orches-rule-selfmetadatalogging"></a>

The orchestrator MUST log self-metadata and key scheduling decisions in structured form without logging secrets (credentials, user content, artifact bodies).

##### `Rule` Self-Metadata Logging Traces To

- [REQ-ORCHES-0120](../requirements/orches.md#req-orches-0120)
- [Orchestrator Artifacts Storage](orchestrator_artifacts_storage.md)

### Self-Metadata Exposure

How operators access self-metadata follows.

#### `Operation` Expose Self-Metadata

- Spec ID: `CYNAI.ORCHES.Operation.ExposeSelfMetadata` <a id="spec-cynai-orches-operation-exposeselfmetadata"></a>

The orchestrator SHOULD expose [Orchestrator Self-Metadata Summary](#spec-cynai-orches-type-orchestratorselfmetadatasummary) to authenticated operators (for example via an admin endpoint or alongside other detailed operator health or status responses).

##### `Operation` Expose Self-Metadata Traces To

- [REQ-ORCHES-0120](../requirements/orches.md#req-orches-0120)

### SBA Inference Log Redaction

Orchestrator-side redaction for stored inference logs follows.

#### `Rule` Redact SBA Inference Data Before Storage

- Spec ID: `CYNAI.ORCHES.SbaInferenceLogRedaction` <a id="spec-cynai-orches-sbainferencelogredaction"></a>

The orchestrator MUST redact secrets from SBA inference data (and any other chat or inference log content it stores) **before** persisting it, using the same shared opportunistic redaction approach as the gateway chat path where applicable.
Only redacted content MUST be persisted; redaction metadata SHOULD be stored when the schema supports it.

##### `Rule` Redact SBA Inference Data Before Storage Traces To

- [REQ-USRGWY-0132](../requirements/usrgwy.md#req-usrgwy-0132)
- [REQ-ORCHES-0120](../requirements/orches.md#req-orches-0120)

## Orchestrator Shutdown

- Spec ID: `CYNAI.ORCHES.OrchestratorShutdown` <a id="spec-cynai-orches-orchestratorshutdown"></a>

### Orchestrator Shutdown Traces To

- [REQ-ORCHES-0164](../requirements/orches.md#req-orches-0164)

When the orchestrator shuts down (e.g. SIGTERM, SIGINT, or graceful stop), it MUST notify all registered worker nodes to stop all agents and jobs that it has directed, including the Project Manager Agent (PMA).
The orchestrator MUST use the defined Worker API contract for this notification so that each node can stop orchestrator-directed managed services and all jobs dispatched by the orchestrator.
See [Worker API - Stop All Orchestrator-Directed](worker_api.md#spec-cynai-worker-stopallorchestratordirected).
The orchestrator MAY perform this notification before or in parallel with other shutdown steps; it MUST attempt to notify each registered worker that has an active `worker_api_target_url` (or equivalent) before the orchestrator process exits.

## Orchestrator Bootstrap Configuration

The orchestrator MAY import bootstrap configuration from a YAML file at startup to seed PostgreSQL and external integrations.
The system always requires at least one worker node for normal operation; for single-system setups that node MAY be on the same host as the orchestrator.
See [Worker Node Requirement](orchestrator_bootstrap.md#spec-cynai-bootst-workernoderequirement) and [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md).
