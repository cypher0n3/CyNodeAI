# ORCHES Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `ORCHES` domain.
It covers orchestrator control-plane behavior, task lifecycle, dispatch, and state management.

## 2 Requirements

- **REQ-ORCHES-0001:** Control-plane: task lifecycle, dispatch, state; bootstrap and config from PostgreSQL via MCP/gateway.
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0001"></a>
- **REQ-ORCHES-0100:** The orchestrator MUST include a task scheduler that decides when and where to run work.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0100"></a>
- **REQ-ORCHES-0101:** The orchestrator MUST support a cron (or equivalent) facility for scheduled jobs, wakeups, and automation.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0101"></a>
- **REQ-ORCHES-0102:** Users and agents MUST be able to enqueue work at a future time or on a recurrence.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0102"></a>
- **REQ-ORCHES-0103:** Schedule evaluation MUST be time-zone aware.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0103"></a>
- **REQ-ORCHES-0104:** Schedules MUST support create, update, disable, and cancellation.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0104"></a>
- **REQ-ORCHES-0105:** The system MUST retain run history per schedule for visibility and debugging.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0105"></a>
- **REQ-ORCHES-0106:** The cron facility SHOULD be exposed to agents (e.g. via MCP tools).
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0106"></a>
- **REQ-ORCHES-0107:** The scheduler implementation MUST use the same node selection and job-dispatch contracts as the rest of the orchestrator.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0107"></a>
- **REQ-ORCHES-0108:** The scheduler MUST be available via the User API Gateway to manage scheduled jobs and query schedule/queue state.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0108"></a>
- **REQ-ORCHES-0109:** When a scheduled run requires agent reasoning, task interpretation, or planning, the orchestrator MUST route that work to the Project Manager Agent for processing and tasking.
  [orchestrator.md - Scheduled run routing](../tech_specs/orchestrator.md#spec-cynai-orches-scheduledrunrouting), [cynode_pma.md](../tech_specs/cynode_pma.md)
  <a id="req-orches-0109"></a>
- **REQ-ORCHES-0110:** Orchestrator-side agents MAY use external AI providers for planning and verification when policy allows it.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0110"></a>
- **REQ-ORCHES-0111:** External provider calls MUST use API Egress and SHOULD use agent-specific routing preferences.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0111"></a>
- **REQ-ORCHES-0112:** External calls MUST use the API Egress Server so credentials are not exposed to agents or sandbox containers.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0112"></a>
- **REQ-ORCHES-0113:** The orchestrator MUST be able to configure worker nodes at registration time.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0113"></a>
- **REQ-ORCHES-0114:** The orchestrator MUST support dynamic configuration updates after registration and must ingest node capability reports on registration and node startup.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0114"></a>
- **REQ-ORCHES-0115:** The orchestrator MAY import bootstrap configuration from a YAML file at startup to seed PostgreSQL and external integrations.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0115"></a>
- **REQ-ORCHES-0116:** The system MUST have at least one worker node for normal operation.
  For single-system setups, that node MAY be on the same host as the orchestrator.
  The orchestrator MUST NOT assume it can run as the sole service with zero worker nodes.
  [CYNAI.BOOTST.WorkerNodeRequirement](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-workernoderequirement)
  <a id="req-orches-0116"></a>
- **REQ-ORCHES-0117:** On startup, the orchestrator MUST select an effective "Project Manager model" to run the Project Manager Agent.
  If a dispatchable local inference worker is available (Ollama or similar), the orchestrator MUST prefer a local Project Manager model.
  If no dispatchable local inference worker is available, the orchestrator MUST use an external model routing path when configured and allowed.
  The orchestrator MUST honor a system setting override that forces Project Manager inference to use an external model routing path when configured and allowed, even when local inference is available (`agents.project_manager.model.selection.execution_mode=force_external`).
  The orchestrator MUST select the node that will run the local Project Manager model using the automatic Project Manager model selection policy.
  By default, the orchestrator MUST prefer running the Project Manager model on a worker node that is on the same host as the orchestrator, unless there is no dispatchable worker on the orchestrator host.
  For this requirement, "on the same host as the orchestrator" MUST be determined by the node capability report label `orchestrator_host`.
  [CYNAI.ORCHES.Operation.SelectProjectManagerModel](../tech_specs/orchestrator.md#spec-cynai-orches-operation-selectprojectmanagermodel)
  [project_manager_agent.md](../tech_specs/project_manager_agent.md)
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-orches-0117"></a>
- **REQ-ORCHES-0118:** When a dispatchable local inference worker is available, the orchestrator MUST ensure the effective Project Manager model is loaded and ready before entering ready state.
  The effective Project Manager model MUST be derived from the automatic Project Manager model selection policy unless overridden by system settings.
  Overrides and policy parameters MUST be configurable via orchestrator bootstrap and via client system settings management surfaces.
  [CYNAI.ORCHES.Rule.WarmupProjectManagerModel](../tech_specs/orchestrator.md#spec-cynai-orches-rule-warmupprojectmanagermodel)
  [model_management.md](../tech_specs/model_management.md)
  <a id="req-orches-0118"></a>
- **REQ-ORCHES-0119:** For the MVP, the Project Manager model MUST handle all inference task assignments.
  This includes selecting the inference execution target (local worker vs external API via API Egress), selecting the model (and version), and requesting model loads when needed.
  [project_manager_agent.md](../tech_specs/project_manager_agent.md)
  [model_management.md](../tech_specs/model_management.md)
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-orches-0119"></a>
- **REQ-ORCHES-0120:** The orchestrator MUST remain running while it is not ready due to missing Project Manager inference prerequisites, and MUST expose health endpoints that distinguish "process alive" from "ready to accept work".
  `GET /healthz` MUST return 200 when the orchestrator process is alive.
  `GET /readyz` MUST return 200 only when the orchestrator is in a ready state; otherwise it MUST return 503 with a reason indicating what prerequisites are missing.
  The orchestrator MUST NOT report ready until at least one inference path exists (a worker node that has reported ready to the orchestrator and is inference-capable, or an LLM API key for PMA via API Egress) and until the PMA has informed the orchestrator that it is online and is reachable.
  PMA is a core system feature and is always required; disabling PMA is not supported.
  While not ready, the orchestrator MUST allow users to configure system settings and credentials required to become ready.
  [CYNAI.ORCHES.Rule.HealthEndpoints](../tech_specs/orchestrator.md#spec-cynai-orches-rule-healthendpoints)
  <a id="req-orches-0120"></a>
- **REQ-ORCHES-0121:** The orchestrator MUST persist tasks and their lifecycle state in PostgreSQL with stable identifiers.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0121"></a>
- **REQ-ORCHES-0122:** Authenticated user clients MUST be able to create tasks through the User API Gateway.
  The create operation MUST return a task identifier in the response **within a bounded time** (without waiting for task execution to complete), so that clients can poll for status and result.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [CYNAI.ORCHES.Rule.TaskCreateHandoff](../tech_specs/orchestrator.md#spec-cynai-orches-rule-taskcreatehandoff)
  [CYNAI.CLIENT.CliTaskCreatePrompt](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskcreateprompt)
  [data_rest_api.md](../tech_specs/data_rest_api.md)
  <a id="req-orches-0122"></a>
- **REQ-ORCHES-0123:** The orchestrator MUST dispatch work to worker nodes via the Worker API and update task/job state based on results.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [worker_api.md](../tech_specs/worker_api.md)
  <a id="req-orches-0123"></a>
- **REQ-ORCHES-0124:** The orchestrator MUST persist job results (including stdout/stderr and exit code) and make them retrievable to authorized clients.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [worker_api.md](../tech_specs/worker_api.md)
  [CYNAI.CLIENT.CliTaskResult](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskresult)
  [CYNAI.CLIENT.CliTaskLogs](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitasklogs)
  <a id="req-orches-0124"></a>
- **REQ-ORCHES-0125:** Authorized clients MUST be able to read task state (including status) through the User API Gateway.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [data_rest_api.md](../tech_specs/data_rest_api.md)
  [CYNAI.CLIENT.CliTaskList](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitasklist)
  [CYNAI.CLIENT.CliTaskGet](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskget)
  [CYNAI.CLIENT.CliTaskResult](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskresult)
  <a id="req-orches-0125"></a>
- **REQ-ORCHES-0126:** Task creation MUST accept a natural-language user prompt and MUST accept task input as plain text or Markdown.
  The system MUST interpret the prompt or task text to decide whether to call an AI model and/or execute sandbox jobs.
  The system SHALL use inference by default when interpreting (no user opt-in required).
  The user prompt or task text MUST NOT be executed as a literal shell command unless the user explicitly requests raw command execution (e.g. script, commands, or a dedicated raw-command flag).
  [user_api_gateway.md](../tech_specs/user_api_gateway.md)
  [cli_management_app_commands_tasks.md](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskcreateprompt)
  <a id="req-orches-0126"></a>
- **REQ-ORCHES-0127:** Task creation MUST support attachments (e.g. files or other artifacts).
  Clients MAY supply attachments as path strings (CLI) or via file upload (web console); the gateway and orchestrator define how attachment payloads are ingested and made available to the task.
  [user_api_gateway.md](../tech_specs/user_api_gateway.md)
  [cli_management_app_commands_tasks.md](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskcreateprompt)
  [web_console.md](../tech_specs/web_console.md#spec-cynai-webcon-apisurface)
  <a id="req-orches-0127"></a>
- **REQ-ORCHES-0128:** Task creation MUST support running a **script** (e.g. path to a script file) and a **short series of commands** as explicit task input types.
  When the client supplies a script or commands, the system MUST run them in the sandbox (or equivalent) rather than interpreting the input as natural language.
  [user_api_gateway.md](../tech_specs/user_api_gateway.md)
  [cli_management_app_commands_tasks.md](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskcreateprompt)
  <a id="req-orches-0128"></a>
- **REQ-ORCHES-0129:** The orchestrator MUST continuously validate that the selected Project Manager model remains online after startup.
  If the selected Project Manager model becomes unavailable due to node loss, eviction, failure, or relevant system setting changes, the orchestrator MUST transition out of ready state and MUST re-run Project Manager model selection and warmup until a Project Manager model is online again.
  [CYNAI.ORCHES.Rule.MonitorProjectManagerModel](../tech_specs/orchestrator.md#spec-cynai-orches-rule-monitorprojectmanagermodel)
  <a id="req-orches-0129"></a>
- **REQ-ORCHES-0130:** The User API Gateway MUST expose all operations required to support the cynork chat slash commands defined in the CLI management app spec.
  Operations include: task (list, get, create, cancel, result, logs, artifacts list, artifacts get), status, whoami, nodes (list, get), preferences (list, get, set, delete, effective), and skills (list, get).
  Chat slash commands MUST use the same gateway API surface as the non-interactive CLI; no separate chat-only API is required.
  [CYNAI.USRGWY.ChatSlashCommandSupport](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-chatslashcommandsupport)
  [cli_management_app_commands_chat.md](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashcommandreference)
  <a id="req-orches-0130"></a>
- **REQ-ORCHES-0131:** The orchestrator MUST enforce a maximum total wait duration when producing an OpenAI-compatible interactive chat response.
  If the response does not finish before the cap, the gateway MUST return a clear timeout error.
  [CYNAI.USRGWY.OpenAIChatApi.Reliability](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-reliability)
  <a id="req-orches-0131"></a>
- **REQ-ORCHES-0132:** The orchestrator MUST retry transient inference failures with bounded backoff before using a fallback path for OpenAI-compatible interactive chat requests.
  [CYNAI.USRGWY.OpenAIChatApi.Reliability](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-reliability)
  <a id="req-orches-0132"></a>
- **REQ-ORCHES-0133:** Task creation MUST associate the task with the authenticated user (set `created_by` to that user) when the request is authenticated.
  When created by the system (including unauthenticated bootstrap), `created_by` MUST be set to the reserved system user identity (see [REQ-IDENTY-0121](../requirements/identy.md#req-identy-0121)), and the task MUST be associated with the **system user's default project** (the system user has a default project in the same way as any user; when there is no authenticated user, that default project is used).
  Task creation MUST accept an optional `project_id`; when `project_id` is omitted, the task MUST be associated with the creating user's default project (authenticated user's default project, or system user's default project when created by the system) (see [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)).
  Tasks MUST be associable with both a user (creator) and a project (default or explicitly set) per schema.
  [REQ-PROJCT-0001](../requirements/projct.md#req-projct-0001)
  [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)
  [projects_and_scopes.md](../tech_specs/projects_and_scopes.md)
  [postgres_schema.md](../tech_specs/postgres_schema.md)
  <a id="req-orches-0133"></a>
- **REQ-ORCHES-0141:** The orchestrator MUST be able to pull node operational telemetry (logs, system info, container inventory/state) from nodes via the Worker Telemetry API.
  [CYNAI.ORCHES.NodeTelemetryPull](../tech_specs/worker_telemetry_api.md#spec-cynai-orches-nodetelemetrypull)
  <a id="req-orches-0141"></a>
- **REQ-ORCHES-0142:** The orchestrator MUST apply per-request timeouts and MUST tolerate node unavailability when pulling telemetry.
  [CYNAI.ORCHES.NodeTelemetryPull](../tech_specs/worker_telemetry_api.md#spec-cynai-orches-nodetelemetrypull)
  <a id="req-orches-0142"></a>
- **REQ-ORCHES-0143:** The orchestrator MUST treat node telemetry as non-authoritative operational data and MUST NOT make correctness-critical scheduling decisions based solely on telemetry responses.
  [CYNAI.ORCHES.NodeTelemetryPull](../tech_specs/worker_telemetry_api.md#spec-cynai-orches-nodetelemetrypull)
  <a id="req-orches-0143"></a>
- **REQ-ORCHES-0144:** The orchestrator MUST provide a stable workflow start/resume API to the workflow runner (transport, operations, and idempotency semantics prescribed in tech specs).
  The workflow runner MUST acquire or validate the task workflow lease via the orchestrator before running.
  [CYNAI.ORCHES.WorkflowStartResumeAPI](../tech_specs/langgraph_mvp.md#spec-cynai-orches-workflowstartresumeapi)
  <a id="req-orches-0144"></a>
- **REQ-ORCHES-0145:** At most one active workflow per task.
  A duplicate start request for a task that already has an active workflow (lease held) MUST return a defined response (e.g. 409 Conflict or 200 with status indicating already running) and MUST NOT start a second instance.
  [CYNAI.ORCHES.WorkflowStartResumeAPI](../tech_specs/langgraph_mvp.md#spec-cynai-orches-workflowstartresumeapi)
  <a id="req-orches-0145"></a>
- **REQ-ORCHES-0146:** Task workflow lease acquire, release (on completion or failure), and expiry (and optional renewal) semantics MUST be defined and enforced by the orchestrator.
  The workflow runner MUST use the orchestrator as the sole source of truth for the lease.
  [CYNAI.ORCHES.TaskWorkflowLeaseLifecycle](../tech_specs/orchestrator.md#spec-cynai-orches-taskworkflowleaselifecycle)
  <a id="req-orches-0146"></a>
- **REQ-ORCHES-0147:** The conditions under which the orchestrator starts a workflow for a task (triggers) MUST be defined in one place: task created via User API, task created via chat/PMA, and scheduled run handed to PMA.
  The orchestrator MUST follow these triggers when starting the workflow runner.
  [CYNAI.ORCHES.WorkflowStartTriggers](../tech_specs/langgraph_mvp.md#spec-cynai-orches-workflowstarttriggers)
  <a id="req-orches-0147"></a>

- **REQ-ORCHES-0148:** The orchestrator MUST set each node's Worker API dispatch URL from the node-reported `worker_api.base_url` (in registration and capability reports) and MUST update it when the node reports a new value; an operator MAY configure an explicit override (e.g. same-host or dev), and when an override is used it MUST be clearly documented as an override.
  [worker_node_payloads.md](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-capabilityreport-v1)
  [worker_node.md](../tech_specs/worker_node.md#spec-cynai-worker-registrationandbootstrap)
  <a id="req-orches-0148"></a>
- **REQ-ORCHES-0149:** The orchestrator MUST acknowledge node registration and return a node configuration payload that instructs the node whether and how to start the local inference backend (e.g. OLLAMA).
  When the node has reported GPU or inference capabilities, the configuration MUST include inference backend instructions (e.g. container image and backend variant such as ROCm for AMD or CUDA for Nvidia) derived from the node capability report, so the node starts the correct OLLAMA (or equivalent) container.
  Variant MUST be derived by **model and/or VRAM**, not vendor alone: when multiple GPU types are reported, the orchestrator MUST use **total VRAM per vendor** (sum of `vram_mb` per vendor) and select the variant for the vendor with the greatest total VRAM.
  [CYNAI.WORKER.ConfigurationDelivery](../tech_specs/worker_node.md#spec-cynai-worker-configurationdelivery)
  [CYNAI.ORCHES.InferenceContainerDecision](../tech_specs/orchestrator_inference_container_decision.md#spec-cynai-orches-inferencecontainerdecision)
  [CYNAI.WORKER.Payload.ConfigurationV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1)
  <a id="req-orches-0149"></a>
- **REQ-ORCHES-0150:** The orchestrator MUST start the Project Manager Agent (cynode-pma) by instructing a worker node to run PMA as a managed service container when the first inference path becomes available: either the first worker node has reported ready to the orchestrator and is inference-capable, or the orchestrator has an LLM API key configured for PMA via the API Egress Server.
  The orchestrator MUST deliver the PMA start bundle via node configuration managed services desired state.
  The orchestrator MUST NOT instruct a node to start PMA before at least one of these conditions is satisfied.
  [CYNAI.BOOTST.OrchestratorReadinessAndPmaStartup](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-orchestratorreadinessandpmastartup)
  [CYNAI.ORCHES.ManagedServicesWorkerManaged](../tech_specs/orchestrator.md#spec-cynai-orches-managedservices)
  <a id="req-orches-0150"></a>
- **REQ-ORCHES-0151:** The orchestrator MUST learn that PMA has come online via worker-reported managed service status (and worker-mediated endpoint) so that the orchestrator can use PMA and update readiness accordingly.
  [CYNAI.PMAGNT.PmaInformsOrchestratorOnline](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-pmainformsorchestratoronline)
  [CYNAI.WORKER.Payload.CapabilityReportV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-capabilityreport-v1)
  <a id="req-orches-0151"></a>
- **REQ-ORCHES-0152:** When a task is associated with a plan (task.plan_id set), the orchestrator MUST NOT start a workflow for that task until that plan's state is active (the one approved plan per project) or the PMA has explicitly requested workflow start for that task (handoff).
  When the task has no plan_id (or the project has no plans), the orchestrator MAY start the workflow per the normal triggers.
  [CYNAI.ORCHES.WorkflowStartGatePlanApproved](../tech_specs/langgraph_mvp.md#spec-cynai-orches-workflowstartgateplanapproved)
  <a id="req-orches-0152"></a>
- **REQ-ORCHES-0153:** When a task in a plan has dependencies (task_dependencies rows), the orchestrator MUST NOT start a workflow for that task until every dependency (depends_on_task_id) has status `completed`.
  Tasks that depend on a failed, canceled, or superseded task MUST NOT start until that dependency is retried and reaches status `completed`.
  Multiple tasks MAY be started in parallel when they have no unsatisfied dependencies (no dependencies, or all dependencies have status `completed`).
  Execution order and runnability are defined only by task dependencies (no ordinal).
  [CYNAI.ORCHES.WorkflowPlanOrder](../tech_specs/langgraph_mvp.md#spec-cynai-orches-workflowplanorder)
  [CYNAI.SCHEMA.TaskDependenciesTable](../tech_specs/postgres_schema.md#spec-cynai-schema-taskdependenciestable)
  <a id="req-orches-0153"></a>
- **REQ-ORCHES-0154:** When a task is set to status `canceled`, the system MUST automatically set to `canceled` every task that depends on it (each task_id that has this task as depends_on_task_id in task_dependencies).
  This MUST be applied transitively: dependents of those tasks are also canceled, so the entire downstream dependency graph from the canceled task is canceled.
  [CYNAI.ORCHES.CancelCascadesToDependents](../tech_specs/langgraph_mvp.md#spec-cynai-orches-cancelcascadestodependents)
  [CYNAI.SCHEMA.TaskDependenciesTable](../tech_specs/postgres_schema.md#spec-cynai-schema-taskdependenciestable)
  <a id="req-orches-0154"></a>

- **REQ-ORCHES-0160:** The orchestrator MUST manage worker-managed services using a desired state model delivered via node configuration payloads, and MUST be able to update that desired state after registration.
  [CYNAI.ORCHES.ManagedServicesWorkerManaged](../tech_specs/orchestrator.md#spec-cynai-orches-managedservices)
  [CYNAI.WORKER.Payload.ConfigurationV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1)
  <a id="req-orches-0160"></a>
- **REQ-ORCHES-0161:** The orchestrator MUST track observed state and worker-mediated endpoint(s) for managed services from worker capability reports and MUST treat those endpoints as dynamic for routing.
  [CYNAI.ORCHES.ManagedServicesWorkerManaged](../tech_specs/orchestrator.md#spec-cynai-orches-managedservices)
  [CYNAI.WORKER.Payload.CapabilityReportV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-capabilityreport-v1)
  <a id="req-orches-0161"></a>
- **REQ-ORCHES-0162:** The orchestrator MUST route `model=cynodeai.pm` traffic to PMA using the worker-mediated endpoint reported by the worker and MUST NOT rely on compose DNS or direct host-port addressing for PMA.
  This routing rule MUST apply to both `POST /v1/chat/completions` and `POST /v1/responses`.
  [CYNAI.ORCHES.ManagedServicesWorkerManaged](../tech_specs/orchestrator.md#spec-cynai-orches-managedservices)
  [CYNAI.WORKER.ManagedAgentProxyBidirectional](../tech_specs/worker_api.md#spec-cynai-worker-managedagentproxy)
  <a id="req-orches-0162"></a>
- **REQ-ORCHES-0163:** For **user-scoped** agents (PAA, SBA), the orchestrator MUST associate each agent token it issues with the **user on whose behalf the agent is acting** (and MUST track this association) so that when the worker proxy forwards requests on behalf of the agent (using the token the worker holds), the gateway can resolve user context for preferences, access control to user- and project-scoped data, and audit attribution.
  Agent tokens are delivered to the worker for the worker proxy to hold; agents MUST NOT be given tokens or secrets directly.
  For **SBA**, the token MUST also be bound to task_id, project_id, and session scope.
  For **system-level** agents (PMA), the agent token is not user-associated.
  [CYNAI.MCPGAT.AgentScopedTokens](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-agentscopedtokens)
  [CYNAI.WORKER.Payload.ConfigurationV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1) (managed_services orchestrator.agent_token)
  [CYNAI.WORKER.AgentTokensWorkerHeldOnly](../tech_specs/worker_node.md#spec-cynai-worker-agenttokensworkerheldonly)
  <a id="req-orches-0163"></a>
- **REQ-ORCHES-0164:** When the orchestrator shuts down, it MUST notify all registered worker nodes to stop all agents and jobs that it has directed, including the Project Manager Agent (PMA).
  Worker nodes MUST respect this notification and stop all such agents and jobs.
  [CYNAI.ORCHES.OrchestratorShutdown](../tech_specs/orchestrator.md#spec-cynai-orches-orchestratorshutdown)
  [CYNAI.WORKER.OrchestratorShutdownNotification](../tech_specs/worker_node.md#spec-cynai-worker-orchestratorshutdownnotification)
  <a id="req-orches-0164"></a>
- **REQ-ORCHES-0165:** When handling `POST /v1/responses`, the orchestrator MUST resolve `previous_response_id` only within the authenticated user's retained response state and the effective project scope for the request.
  Cross-user, cross-project, missing, or expired response references MUST NOT be used as continuation state.
  [CYNAI.ORCHES.ResponsesContinuationState](../tech_specs/orchestrator.md#spec-cynai-orches-responsescontinuationstate)
  [CYNAI.USRGWY.OpenAIChatApi](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi)
  <a id="req-orches-0165"></a>
- **REQ-ORCHES-0166:** When handling `POST /v1/responses`, the orchestrator MUST persist retained response metadata sufficient to support later `previous_response_id` continuation while that state remains within retention.
  This retained response metadata MUST preserve CyNodeAI's canonical thread and message ownership model and MUST NOT require any CyNodeAI-specific thread identifier in OpenAI-compatible requests.
  [CYNAI.ORCHES.ResponsesContinuationState](../tech_specs/orchestrator.md#spec-cynai-orches-responsescontinuationstate)
  [CYNAI.USRGWY.OpenAIChatApi](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi)
  <a id="req-orches-0166"></a>
- **REQ-ORCHES-0167:** When the User API Gateway receives an OpenAI-compatible chat request that includes user-message file references accepted under the chat upload contract, the orchestrator MUST resolve those references to retrievable content and MUST pass that content or a stable resolvable reference into the PMA or inference path.
  The orchestrator MUST NOT silently drop accepted chat file inputs.
  [CYNAI.ORCHES.ChatFileUploadFlow](../tech_specs/orchestrator.md#spec-cynai-orches-chatfileuploadflow)
  <a id="req-orches-0167"></a>
- **REQ-ORCHES-0168:** When the completion path uses retained response state or thread message history, the orchestrator MUST include any file content or file references associated with those historical user turns when the chat contract requires that context to remain available.
  The orchestrator MUST preserve the same user and project-scoped authorization that applied to the original uploaded file and MUST NOT broaden access when replaying shared-project file context.
  [CYNAI.ORCHES.ChatFileUploadFlow](../tech_specs/orchestrator.md#spec-cynai-orches-chatfileuploadflow)
  <a id="req-orches-0168"></a>
- **REQ-ORCHES-0169:** When the orchestrator directs node-local inference or a managed service that depends on node-local inference, the orchestrator MUST determine the effective backend runtime configuration needed to maximize the safe usable context window for the node's expected local model workload within the node's available resources.
  That determination MUST be derived deterministically from node capabilities and orchestrator policy, and any resulting backend-derived environment values MUST be delivered through the canonical node-configuration contract and kept consistent between the local inference backend configuration and any dependent managed-service inference configuration.
  [CYNAI.ORCHES.InferenceDecisionOutput](../tech_specs/orchestrator_inference_container_decision.md#spec-cynai-orches-inferencedecisionoutput)
  [CYNAI.WORKER.Payload.ConfigurationV1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1)
  <a id="req-orches-0169"></a>
- **REQ-ORCHES-0170:** When routing a streaming interactive chat request (`stream=true`) to PMA, the orchestrator MUST forward `stream=true` to PMA and MUST consume PMA's NDJSON streaming output for real-time SSE relay to the client, using the per-endpoint SSE format rules defined in CYNAI.USRGWY.OpenAIChatApi.StreamingPerEndpointSSEFormat.
  [CYNAI.ORCHES.OpenAIInteractiveChatRouting](../tech_specs/orchestrator.md#spec-cynai-orches-openaiinteractivechatrouting)
  [CYNAI.USRGWY.OpenAIChatApi.StreamingPerEndpointSSEFormat](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streamingperendpointsseformat)
  <a id="req-orches-0170"></a>
- **REQ-ORCHES-0171:** The orchestrator MUST NOT use synthetic chunking of complete payloads (the former `emitContentAsSSE` behavior) for any streaming path; it MUST be removed entirely.
  Any path that previously used synthetic chunking MUST use either real upstream PMA streaming or the heartbeat fallback defined in CYNAI.USRGWY.OpenAIChatApi.StreamingHeartbeatFallback.
  [CYNAI.ORCHES.OpenAIInteractiveChatRouting](../tech_specs/orchestrator.md#spec-cynai-orches-openaiinteractivechatrouting)
  [CYNAI.USRGWY.OpenAIChatApi.StreamingHeartbeatFallback](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streamingheartbeatfallback)
  <a id="req-orches-0171"></a>
- **REQ-ORCHES-0172:** When PMA signals that it cannot provide real token streaming (non-streaming JSON response, streaming wrapper failure, or explicit configuration toggle), the orchestrator MUST use the heartbeat fallback path instead of blocking silently or falling through to a non-streaming response.
  [CYNAI.ORCHES.OpenAIInteractiveChatRouting](../tech_specs/orchestrator.md#spec-cynai-orches-openaiinteractivechatrouting)
  [CYNAI.USRGWY.OpenAIChatApi.StreamingHeartbeatFallback](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streamingheartbeatfallback)
  <a id="req-orches-0172"></a>
- **REQ-ORCHES-0173:** The orchestrator MUST track the effective deadline for each dispatched job (updated when a timeout extension is granted and the orchestrator is informed) and MUST run a scheduled task (e.g. periodic cron or timer) that finds jobs in progress that have exceeded their effective deadline without a reported completion or granted extension, and MUST mark those jobs as failed (timeout) so the orchestrator can re-issue or retry as policy allows.
  [CYNAI.ORCHES.Rule.JobTimeoutTracking](../tech_specs/orchestrator.md#spec-cynai-orches-rule-jobtimeouttracking)
  <a id="req-orches-0173"></a>
- **REQ-ORCHES-0174:** When a job timeout extension is granted (by the node or by the orchestrator), the orchestrator MUST be informed of the new effective deadline and MUST update its job timeout tracking so the scheduled timeout check does not mark the job as timed out while the job is within the extended deadline.
  [CYNAI.ORCHES.Rule.JobTimeoutTracking](../tech_specs/orchestrator.md#spec-cynai-orches-rule-jobtimeouttracking)
  [worker_api.md - Job lifecycle and result persistence](../tech_specs/worker_api.md#spec-cynai-worker-joblifecycleresultpersistence)
  [cynode_sba.md - Timeout Extension](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-timeoutextension)
  <a id="req-orches-0174"></a>
- **REQ-ORCHES-0175:** For **MVP**, supported GPU vendors for node-local inference backend variant selection are **AMD** (variant `rocm`) and **NVIDIA** (variant `cuda`).
  **Intel** GPU support (worker detection of Intel devices, orchestrator variant selection for Intel, and inference backend image for Intel) is **deferred until post-MVP**.
  Until then, nodes MUST NOT use an Intel-specific inference backend variant; if only Intel GPUs are present, the orchestrator SHALL treat the node as CPU for inference (or "do not start" per policy).
  [orchestrator_inference_container_decision.md - Vendor Support](../tech_specs/orchestrator_inference_container_decision.md#spec-cynai-orches-inferencevendorsupportmvp)
  [worker_node_payloads.md - Capability Report gpu.devices](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-capabilityreport-v1)
  <a id="req-orches-0175"></a>
