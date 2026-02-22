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
- **REQ-ORCHES-0116:** The orchestrator SHOULD support running as the sole service with zero worker nodes and using external AI providers when allowed.
  [orchestrator.md](../tech_specs/orchestrator.md)
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
  While not ready, the orchestrator MUST allow users to configure system settings and credentials required to become ready.
  [CYNAI.ORCHES.Rule.HealthEndpoints](../tech_specs/orchestrator.md#spec-cynai-orches-rule-healthendpoints)
  <a id="req-orches-0120"></a>

- **REQ-ORCHES-0121:** The orchestrator MUST persist tasks and their lifecycle state in PostgreSQL with stable identifiers.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0121"></a>
- **REQ-ORCHES-0122:** Authenticated user clients MUST be able to create tasks through the User API Gateway.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [data_rest_api.md](../tech_specs/data_rest_api.md)
  <a id="req-orches-0122"></a>
- **REQ-ORCHES-0123:** The orchestrator MUST dispatch work to worker nodes via the Worker API and update task/job state based on results.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [worker_api.md](../tech_specs/worker_api.md)
  <a id="req-orches-0123"></a>
- **REQ-ORCHES-0124:** The orchestrator MUST persist job results (including stdout/stderr and exit code) and make them retrievable to authorized clients.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [worker_api.md](../tech_specs/worker_api.md)
  <a id="req-orches-0124"></a>
- **REQ-ORCHES-0125:** Authorized clients MUST be able to read task state (including status) through the User API Gateway.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [data_rest_api.md](../tech_specs/data_rest_api.md)
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
- **REQ-ORCHES-0133:** Task creation MUST associate the task with the authenticated user (set `created_by` to that user) when the request is authenticated.
  When created by the system (including unauthenticated bootstrap), `created_by` MUST be set to the reserved system user identity (see [REQ-IDENTY-0121](../requirements/identy.md#req-identy-0121)), and the task MUST be associated with the **system user's default project** (the system user has a default project in the same way as any user; when there is no authenticated user, that default project is used).
  Task creation MUST accept an optional `project_id`; when `project_id` is omitted, the task MUST be associated with the creating user's default project (authenticated user's default project, or system user's default project when created by the system) (see [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)).
  Tasks MUST be associable with both a user (creator) and a project (default or explicitly set) per schema.
  [REQ-PROJCT-0001](../requirements/projct.md#req-projct-0001)
  [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)
  [projects_and_scopes.md](../tech_specs/projects_and_scopes.md)
  [postgres_schema.md](../tech_specs/postgres_schema.md)
  <a id="req-orches-0133"></a>
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
- **REQ-ORCHES-0131:** The orchestrator MUST enforce a maximum total wait duration when producing an OpenAI-compatible chat completion response.
  If the completion does not finish before the cap, the gateway MUST return a clear timeout error.
  [CYNAI.USRGWY.OpenAIChatApi.Reliability](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-reliability)
  <a id="req-orches-0131"></a>
- **REQ-ORCHES-0132:** The orchestrator MUST retry transient inference failures with bounded backoff before using a fallback path for OpenAI-compatible chat completions.
  [CYNAI.USRGWY.OpenAIChatApi.Reliability](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-reliability)
  <a id="req-orches-0132"></a>
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
