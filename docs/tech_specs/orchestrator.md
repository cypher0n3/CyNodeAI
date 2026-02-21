# Orchestrator Technical Spec

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Core Responsibilities](#core-responsibilities)
- [Health Checks](#health-checks)
- [Task Scheduler](#task-scheduler)
- [Project Manager Agent](#project-manager-agent)
  - [Project Manager Model (Startup Selection and Warmup)](#project-manager-model-startup-selection-and-warmup)
- [API Egress Server](#api-egress-server)
- [Web Egress Proxy](#web-egress-proxy)
- [Secure Browser Service](#secure-browser-service)
- [External Model Routing](#external-model-routing)
- [Model Management](#model-management)
- [User API Gateway](#user-api-gateway)
- [Sandbox Image Registry](#sandbox-image-registry)
- [Node Bootstrap and Configuration](#node-bootstrap-and-configuration)
- [MCP Tool Interface](#mcp-tool-interface)
- [Workflow Engine](#workflow-engine)
- [Orchestrator Bootstrap Configuration](#orchestrator-bootstrap-configuration)

## Spec IDs

- Spec ID: `CYNAI.ORCHES.Doc.Orchestrator` <a id="spec-cynai-orches-doc-orchestrator"></a>

## Document Overview

This document describes the orchestrator responsibilities and its relationship to orchestrator-side agents.

## Core Responsibilities

- Acts as the control plane for nodes, jobs, tasks, and agent workflows.
- Owns the source of truth for task state, results, logs, and user task-execution preferences in PostgreSQL.
- Dispatches sandboxed execution to worker nodes (via the worker API).
- Routes model inference to local nodes or to external providers when allowed.
- Schedules sandbox execution independently of where inference occurs.

## Health Checks

This section defines the orchestrator health and readiness endpoints.

### `Orchestrator.HealthEndpoints` Rule

- Spec ID: `CYNAI.ORCHES.Rule.HealthEndpoints` <a id="spec-cynai-orches-rule-healthendpoints"></a>

Traces To: [REQ-ORCHES-0119](../requirements/orches.md#req-orches-0119), [REQ-BOOTST-0002](../requirements/bootst.md#req-bootst-0002)

The orchestrator exposes health endpoints that distinguish "process alive" from "ready to accept work".

Endpoints

- `GET /healthz`
  - Returns 200 when the orchestrator process is alive and serving HTTP.
  - This endpoint MUST NOT require that the Project Manager model is online.
- `GET /readyz`
  - Returns 200 only when the orchestrator is in a ready state.
  - Returns 503 when prerequisites for readiness are not yet satisfied (for example no eligible inference path, Project Manager model not online, or required credentials/policy not present).
  - The response MUST include a reason that is actionable for an operator.
  - While `GET /readyz` returns 503, the orchestrator continues to serve the management surfaces required to become ready (for example system settings and credential configuration).

## Task Scheduler

The orchestrator MUST include a task scheduler that decides when and where to run work.

Responsibilities

- **Queue**: Maintain a queue of pending work (tasks and jobs) backed by PostgreSQL so state survives restarts.
- **Dispatch**: Select eligible nodes based on capability, load, data locality, and model availability; dispatch jobs to the worker API; collect results and update task state.
- **Retries and leases**: Support job leases, retries on failure, and idempotency so work is not lost or duplicated when nodes fail or restart.
- **Cron tool**: MUST support a cron (or equivalent) facility for scheduled jobs, wakeups, and automation.
  Users and agents MUST be able to enqueue work at a future time or on a recurrence (cron expression or calendar-like).
  The scheduler is responsible for firing at the scheduled time and enqueueing the corresponding tasks or jobs.
  Schedule evaluation MUST be time-zone aware (schedules specify or inherit a time zone; next-run and history use that zone).
  Schedules MUST support create, update, disable (temporarily stop firing without deleting), and cancellation (cancel the schedule or the next run).
  The system MUST retain run history per schedule (past execution times and outcomes) for visibility and debugging.
  The cron facility SHOULD be exposed to agents (e.g. via MCP tools) so they can create and manage scheduled jobs.

The scheduler MAY be implemented as a background process, a worker that consumes the queue, or integrated into the workflow engine; it MUST use the same node selection and job-dispatch contracts as the rest of the orchestrator.
Agents (e.g. Project Manager) and the cron facility enqueue work; the scheduler is responsible for dequeueing and dispatching to nodes.
The scheduler MUST be available via the User API Gateway so users can create and manage scheduled jobs, query queue and schedule state, and trigger wakeups or automation.

See job dispatch and node selection in [`docs/tech_specs/node.md`](node.md), the roadmap in [`docs/tech_specs/_main.md`](_main.md), and [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md).

## Project Manager Agent

The Project Manager Agent is a long-lived orchestrator-side agent that continuously drives work to completion.

- Reads tasks and their acceptance criteria from the database.
- Retrieves user preferences and standards from the database and applies them during planning and verification.
- Assigns work to worker nodes, monitors progress, and requests remediation when results fail checks.
- Continuously updates task state in PostgreSQL so the system remains resumable and auditable.
- Eagerly spawns Project Analyst sub-agents for task-scoped monitoring and verification whenever possible.

See [`docs/tech_specs/project_manager_agent.md`](project_manager_agent.md), [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md), and [`docs/tech_specs/user_preferences.md`](user_preferences.md).

Orchestrator-side agents MAY use external AI providers for planning and verification when policy allows it.
External provider calls MUST use API Egress and SHOULD use agent-specific routing preferences.
See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md) and [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).

### Project Manager Model (Startup Selection and Warmup)

The orchestrator MUST select an effective "Project Manager model" on startup to run the Project Manager Agent.
This selection is distinct from where sandbox jobs run.

#### 1 `Orchestrator.SelectProjectManagerModel` Operation

- Spec ID: `CYNAI.ORCHES.Operation.SelectProjectManagerModel` <a id="spec-cynai-orches-operation-selectprojectmanagermodel"></a>

Traces To: [REQ-ORCHES-0116](../requirements/orches.md#req-orches-0116), [REQ-MODELS-0004](../requirements/models.md#req-models-0004), [REQ-MODELS-0005](../requirements/models.md#req-models-0005)

This Spec Item defines the deterministic selection of:

- the Project Manager inference execution target (local node vs external provider routing path), and
- the effective Project Manager model reference to run on that target.

##### 1.1 `Orchestrator.SelectProjectManagerModel` Inputs

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

##### 1.2 `Orchestrator.SelectProjectManagerModel` Outputs

- An effective selection tuple:
  - `execution_mode`: `local` or `external`
  - `local_node_slug`: string (required when `execution_mode=local`)
  - `model_ref`: string (local model name, or external provider model identifier)
  - `selection_reason`: an ordered list of machine-readable reason codes (for audit/logging)

##### 1.3 `Orchestrator.SelectProjectManagerModel` Behavior

The orchestrator selects exactly one effective Project Manager model on startup.

Determinism requirements:

- All tie-breaks in this operation are resolved lexicographically by `node_slug` (ascending).
- If a required system setting key is unset, this operation uses the default value specified in Inputs.

Definitions:

- A node is considered "on the same host as the orchestrator" if its capability report `node.labels` contains the literal label `orchestrator_host`.
- `vram_total_mb` for a node is computed as:
  - sum of all present `gpu.devices[].vram_mb` values, ignoring devices that omit `vram_mb`
  - if no `vram_mb` values are present, `vram_total_mb=0`

##### 1.4 `Orchestrator.SelectProjectManagerModel` Error Conditions

- If `agents.project_manager.model.selection.mode=fixed_model` and `agents.project_manager.model.local_default_ollama_model` is unset, selection fails.
- If `execution_mode=local` is selected and no local candidate model can be loaded successfully, selection fails unless `execution_mode=external` is allowed and configured.
- If selection fails and the orchestrator does not currently have an online Project Manager model, the orchestrator MUST continue to re-run selection when relevant inputs change until a Project Manager model is online.

##### 1.5 `Orchestrator.SelectProjectManagerModel` Ordering and Determinism

Selection proceeds in the strict order defined by the Algorithm section below.

##### 1.6 `Orchestrator.SelectProjectManagerModel` Algorithm

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
       - Candidates (in order): `qwen2.5-32b-instruct` (4-bit), `qwen2.5-14b-instruct` (4-bit), `llama-3.3-8b-instruct` (4-bit), `tinyllama`.
     - Else if `vram_total_mb >= 16000`:
       - Candidates (in order): `qwen2.5-14b-instruct` (4-bit), `llama-3.3-8b-instruct` (4-bit), `tinyllama`.
     - Else:
       - Candidates (in order): `tinyllama`.
     - Select the first candidate model that the orchestrator can satisfy by:
       - detecting it is already loaded on the selected node, or
       - successfully requesting the node load it (via the model load workflow).
6. If no local candidate can be satisfied: <a id="algo-cynai-orches-operation-selectprojectmanagermodel-step-6"></a>
   - If external routing is configured and allowed, set `execution_mode=external` and select an external `model_ref`.
   - Otherwise, fail selection.

#### 2 `Orchestrator.WarmupProjectManagerModel` Rule

- Spec ID: `CYNAI.ORCHES.Rule.WarmupProjectManagerModel` <a id="spec-cynai-orches-rule-warmupprojectmanagermodel"></a>

Traces To: [REQ-ORCHES-0117](../requirements/orches.md#req-orches-0117)

This Spec Item defines the required startup warmup behavior after a local Project Manager model selection has been made.

##### 2.1 `Orchestrator.WarmupProjectManagerModel` Outcomes

- If `execution_mode=local`, the orchestrator transitions to ready state only after the selected local `model_ref` is reported as loaded and available by the selected node.
- If `execution_mode=external`, warmup does not require a local model load.

##### 2.2 `Orchestrator.WarmupProjectManagerModel` Algorithm

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

#### 3 `Orchestrator.MonitorProjectManagerModel` Rule

- Spec ID: `CYNAI.ORCHES.Rule.MonitorProjectManagerModel` <a id="spec-cynai-orches-rule-monitorprojectmanagermodel"></a>

Traces To: [REQ-ORCHES-0128](../requirements/orches.md#req-orches-0128)

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

Required behavior

- If the orchestrator is in a ready state and the Project Manager model becomes unavailable, the orchestrator MUST transition out of ready state.
- While not ready, the orchestrator MUST continue to serve the management surfaces required to become ready (for example system settings and credential configuration).
- When the Project Manager model becomes unavailable or when relevant inputs change, the orchestrator MUST re-run `Orchestrator.SelectProjectManagerModel` and apply `Orchestrator.WarmupProjectManagerModel` until a Project Manager model is online again.
- If the orchestrator must restart the Project Manager Agent due to a model availability change, it MUST restore the agent state from PostgreSQL so task progress remains resumable.

Note:

- For the MVP, the Project Manager model is responsible for all inference task assignment decisions.
  See [REQ-ORCHES-0118](../requirements/orches.md#req-orches-0118) and [`docs/tech_specs/project_manager_agent.md`](project_manager_agent.md).

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

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md) and [`docs/tech_specs/data_rest_api.md`](data_rest_api.md).

## Sandbox Image Registry

The orchestrator integrates with a sandbox container image registry for worker nodes to pull sandbox images from.
Allowed sandbox images and their capabilities are tracked in PostgreSQL so tasks can request safe, appropriate execution environments.

See [`docs/tech_specs/sandbox_image_registry.md`](sandbox_image_registry.md).

## Node Bootstrap and Configuration

The orchestrator MUST be able to configure worker nodes at registration time.
This includes distributing the correct endpoints, certificates, and pull credentials for orchestrator-provided services.
The orchestrator MUST support dynamic configuration updates after registration and must ingest node capability reports on registration and node startup.

Config delivery

- The orchestrator exposes the node config URL in the bootstrap payload (`node_config_url` in `node_bootstrap_payload_v1`).
- GET on that URL returns `node_configuration_payload_v1` for the authenticated node.
- POST on that URL accepts `node_config_ack_v1` and persists the acknowledgement; see [`docs/tech_specs/postgres_schema.md`](postgres_schema.md) Nodes table columns `config_ack_at`, `config_ack_status`, `config_ack_error`.
- Endpoint paths are not mandated here; the bootstrap payload carries the concrete URLs so nodes do not rely on hard-coded paths.

Job dispatch (initial implementation)

- For the initial single-node implementation (Phase 1), the orchestrator dispatches jobs to the Worker API via direct HTTP.
- The dispatcher uses the per-node `worker_api_target_url` and per-node bearer token stored from config delivery (see [`docs/tech_specs/postgres_schema.md`](postgres_schema.md) Nodes table).
- The MCP gateway is not in the loop for job dispatch in Phase 1.

See [`docs/tech_specs/node.md`](node.md) and [`docs/tech_specs/node_payloads.md`](node_payloads.md).

## MCP Tool Interface

The orchestrator adopts MCP as the standard tool interface for agents.
This enables a consistent tool protocol and role-based tool access.

See [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md).

## Workflow Engine

The orchestrator uses LangGraph to implement multi-step and multi-agent workflows.
The Project Manager Agent's behavior is implemented by the LangGraph MVP workflow.

See [`docs/tech_specs/langgraph_mvp.md`](langgraph_mvp.md) for the graph topology, state model, and node behaviors.
For how the graph is hosted, invoked, checkpointed, and wired to orchestrator capabilities (MCP, Worker API, model routing), see the "Integration with the Orchestrator" section of that document.

## Orchestrator Bootstrap Configuration

The orchestrator MAY import bootstrap configuration from a YAML file at startup to seed PostgreSQL and external integrations.
The orchestrator SHOULD support running as the sole service with zero worker nodes and using external AI providers when allowed.

See [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md).
