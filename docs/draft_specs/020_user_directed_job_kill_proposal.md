# User-Directed Job Kill: Draft Specification Proposal

- [1. Purpose and Scope](#1-purpose-and-scope)
- [2. User Direction and Entry Points](#2-user-direction-and-entry-points)
- [3. Authorization and Orchestrator Role](#3-authorization-and-orchestrator-role)
- [4. Worker Node: Stop Job Request](#4-worker-node-stop-job-request)
- [5. Worker Node: Graceful SBA Stop and Container Kill Fallback](#5-worker-node-graceful-sba-stop-and-container-kill-fallback)
- [6. Task and Job State](#6-task-and-job-state)
- [7. Relationship to Existing Specs](#7-relationship-to-existing-specs)
- [8. References](#8-references)

## 1. Purpose and Scope

Document type: draft specification proposal.

Users MUST be able to **kill a job at their direction** so that long-running or stuck work can be stopped without waiting for timeout or manual node intervention.
This proposal specifies how user-directed job kill is initiated (via messaging connectors or PMA), how the orchestrator and worker node participate, and how the worker node stops the Sandbox Agent (SBA) gracefully with a container-kill fallback.

Scope: entry points for user direction (slash command, PMA natural language), orchestrator handling of task cancel and worker stop request, Worker API extension for stop job, and worker behavior (signal SBA to stop first, then kill container if SBA does not stop in time).

## 2. User Direction and Entry Points

Job kill MUST be possible at **user direction** through the following entry points.

### 2.1 Slash Command (Messaging Connectors)

When the user sends a **slash command** in a configured messaging connector (Signal, Discord, Mattermost), the command MAY include job or task cancellation.

- Example: `/kill job <task_id>` or `/task cancel <task_id>` (or task name when supported).
- The connector delivers the message to PMA with context (user, connector, thread); PMA parses the slash command and performs the same cancel flow as below (invokes task cancel and, when applicable, worker stop).
- Slash commands for job kill are part of the default slash-command set described in [Default Messaging Connectors - Slash Commands](010_default_messaging_connectors_proposal.md).

### 2.2 PMA (Natural Language)

When the user tells the Project Manager Agent in natural language to **kill a job**, **cancel a task**, or **stop a running job**, PMA MUST interpret the intent and execute the kill flow.

- PMA resolves the task (e.g. by task_id, task name, or from conversation context).
- PMA invokes the same orchestrator path as task cancel (e.g. gateway task-cancel API or internal orchestrator cancel) and, when the task has an active job on a worker node, ensures the orchestrator sends a **stop job** request to that node.
- PMA MAY use an MCP tool (e.g. `task.cancel` or equivalent) that triggers orchestrator task cancel and worker stop when applicable.

### 2.3 Gateway and CLI (Existing)

The User API Gateway already exposes **task cancel** (e.g. `POST /v1/tasks/{task_id}/cancel` or equivalent) used by the CLI (`cynork task cancel <task_id>`).
This proposal does not change the gateway contract; it specifies that when task cancel is invoked (from gateway, CLI, PMA, or slash command), the orchestrator MUST also send a stop request to the worker node when the task has an active job running on that node.

## 3. Authorization and Orchestrator Role

- **Authorization:** Only the **user** (or an identity acting with the user's permission) who is allowed to cancel the task MAY trigger a job kill for that task.
  The same access rules as for [task cancel](../tech_specs/cynork/cli_management_app_commands_tasks.md#spec-cynai-client-clicommandsurface) and gateway task operations apply: the subject must be authorized to cancel the task (e.g. task owner, project member, or admin as defined by access control).

- **Orchestrator role:** On receiving a task cancel request (from gateway, PMA, or internal call triggered by slash command), the orchestrator MUST:
  1. Mark the task as canceled (or transitioning to canceled) in task state and persist.
  2. If the task has an **active job** (dispatched to a worker node and not yet in a terminal state), the orchestrator MUST send a **stop job** request to that worker node for the corresponding `job_id` (and `task_id`).
  3. The orchestrator MUST use the node's Worker API base URL and credentials (same as for job dispatch) to call the stop endpoint.

- **Idempotency:** If the job has already completed, failed, or been stopped, the orchestrator's stop request is a no-op on the node; the node MAY return success or a "not running" status so the orchestrator can treat the request as satisfied.

## 4. Worker Node: Stop Job Request

The Worker API MUST support a **stop job** (or cancel job) operation so the orchestrator can request that a running job be stopped.

### 4.1 Endpoint (Proposed)

- **Method and path:** `POST /v1/worker/jobs:stop` (or `POST /v1/worker/jobs/{job_id}/stop`; exact path to be fixed in Worker API spec).
- **Request body:** At least `task_id` (uuid) and `job_id` (uuid) so the node can identify the running job.
- **Authentication:** Same as Run Job (orchestrator bearer token).

### 4.2 Required Behavior

- The node MUST look up the running job (container or process) associated with `task_id` and `job_id`.
- If no such job is running, the node MUST return success (e.g. 200) with a payload indicating "not running" or "already stopped" so the orchestrator can consider the request satisfied.
- If the job is running, the node MUST initiate the stop sequence defined in [Section 5](#5-worker-node-graceful-sba-stop-and-container-kill-fallback).
- The node MUST return a response that indicates the stop was accepted (e.g. 202 Accepted or 200 with status "stopping"); the actual termination may complete asynchronously.

Exact status codes and response shape are to be defined in [`worker_api.md`](../tech_specs/worker_api.md) when this proposal is adopted.

## 5. Worker Node: Graceful SBA Stop and Container Kill Fallback

When the node receives a stop job request for a running job that uses an **SBA** (Sandbox Agent) runner, the node MUST stop the job in two stages.

### 5.1 Stage 1: Stop SBA Directly (Graceful)

- The node MUST first attempt to **stop the SBA process directly** so the agent can shut down cleanly (e.g. flush state, write partial result, exit gracefully).
- Allowable mechanisms (implementation chooses one or more):
  - Send a signal to the SBA process (e.g. SIGTERM) and wait for the process to exit.
  - If the SBA exposes an in-band stop or shutdown hook (e.g. HTTP endpoint or file-based signal inside the container), the node MAY call that first and then wait for exit.
- The node MUST allow a **grace period** (e.g. 30 seconds; configurable via node config or request) for the SBA to exit.
  If the SBA exits within the grace period, the node MUST treat the job as stopped and report the result (e.g. canceled or failed with a cancel reason) to the orchestrator per the existing job result contract.

### 5.2 Stage 2: Container Kill Fallback

- If the SBA does **not** exit within the grace period, the node MUST **kill the SBA container** (e.g. container stop with a short timeout, then container kill, or equivalent runtime primitive).
- After the container is gone, the node MUST report the job as stopped (e.g. status `canceled` or `failed` with a cancel/failure reason) to the orchestrator and MUST NOT leave the job in a permanent "running" state.

### 5.3 Non-SBA Jobs

- For jobs that do **not** use an SBA runner (e.g. plain command or step-executor), the node MAY skip the "stop SBA directly" step and proceed to **stop the container** (e.g. SIGTERM to the main process, then container kill after a short timeout).
  The same principle applies: graceful stop first where possible, then container kill if the process does not exit in time.

## 6. Task and Job State

- When a job is stopped by user direction, the **task** MUST transition to a terminal state that reflects cancellation (e.g. `canceled`).
- The **job** record on the orchestrator MUST be updated to a terminal state (e.g. `canceled` or `failed` with a reason indicating user-initiated stop).
- The orchestrator MUST record an audit event (task canceled, job stopped at user direction, user identity, task_id, job_id, timestamp).

## 7. Relationship to Existing Specs

- **Task cancel (CLI / Gateway):** [CLI task cancel](../tech_specs/cynork/cli_management_app_commands_tasks.md#spec-cynai-client-clicommandsurface) and the User API Gateway task cancel remain the canonical way to request cancellation; this proposal extends the **orchestrator and worker** behavior so that cancel results in a stop request to the node when the task has an active job.
- **Worker API:** [worker_api.md](../tech_specs/worker_api.md) currently defines only `POST /v1/worker/jobs:run`.
  This proposal adds a **stop job** endpoint and required behavior; the exact path and payloads MUST be added to the Worker API spec upon adoption.
- **Worker node:** [worker_node.md](../tech_specs/worker_node.md) and [worker_node_payloads.md](../tech_specs/worker_node_payloads.md) do not yet define stop job; adoption would add a Spec Item for stop job and reference the Worker API stop endpoint.
- **PMA:** PMA already has context for tasks and can call MCP or gateway to perform task operations; this proposal requires that when the user directs PMA to kill a job, PMA triggers the same orchestrator cancel path and that the orchestrator sends the worker stop request.
- **Messaging connectors:** [Default Messaging Connectors](010_default_messaging_connectors_proposal.md) (slash commands) MUST include a way to cancel a task or kill a job (e.g. `/task cancel <task_id>` or `/kill job <task_id>`); see [Section 2.1](#21-slash-command-messaging-connectors).

## 8. References

- [Worker API](../tech_specs/worker_api.md)
- [Worker Node](../tech_specs/worker_node.md)
- [CLI task cancel](../tech_specs/cynork/cli_management_app_commands_tasks.md#spec-cynai-client-clicommandsurface)
- [CyNode PMA](../tech_specs/cynode_pma.md)
- [Default Messaging Connectors - Slash Commands](010_default_messaging_connectors_proposal.md)
- [CyNode SBA - Job lifecycle](../tech_specs/cynode_sba.md) (for SBA in-progress and completion)
