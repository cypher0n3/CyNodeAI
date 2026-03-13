# CyNode PMA (`cynode-pma`)

- [Document Overview](#document-overview)
- [Purpose and Trust Boundary](#purpose-and-trust-boundary)
- [Request Source and Orchestrator Handoff](#request-source-and-orchestrator-handoff)
  - [What is Handed off to `cynode-pma`](#what-is-handed-off-to-cynode-pma)
  - [What is Not Handed off to `cynode-pma`](#what-is-not-handed-off-to-cynode-pma)
  - [Process Boundaries and Workflow Runner](#process-boundaries-and-workflow-runner)
- [Role Modes](#role-modes)
- [Instructions Loading and Routing](#instructions-loading-and-routing)
- [LLM Context Composition](#llm-context-composition)
- [PMA Conversation History](#pma-conversation-history)
- [Chat File Context](#chat-file-context)
- [Chat Surface Mapping](#chat-surface-mapping)
- [Node-Local Inference Backend Environment](#node-local-inference-backend-environment)
- [Policy and Tool Boundaries](#policy-and-tool-boundaries)
- [MCP Tool Access](#mcp-tool-access)
- [Skills and Default Skill](#skills-and-default-skill)
- [Configuration Surface](#configuration-surface)

## Document Overview

- Spec ID: `CYNAI.PMAGNT.Doc.CyNodePma` <a id="spec-cynai-pmagnt-doc-cynodepma"></a>

This document defines the `cynode-pma` agent binary.
It is the concrete implementation artifact for:

- The Project Manager Agent (`project_manager` mode).
- The Project Analyst Agent (`project_analyst` mode).

This spec is implementation-oriented.
Behavioral responsibilities remain defined by:

- [`docs/tech_specs/project_manager_agent.md`](project_manager_agent.md)
- [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md)

## Purpose and Trust Boundary

`cynode-pma` is an orchestrator-owned agent runtime hosted as a **worker-managed service container**.
It is not a per-task sandbox container and it is not a worker agent.
It MUST NOT execute arbitrary code locally.
It delegates execution to worker nodes and sandbox containers through orchestrator-mediated mechanisms.

### Network and Routing Constraints (Normative)

- `cynode-pma` MUST NOT connect directly to orchestrator hostnames or ports.
  Agent-to-orchestrator communication (MCP tool calls and callbacks) MUST flow through the worker proxy.
  See [`docs/tech_specs/worker_api.md`](worker_api.md#spec-cynai-worker-managedagentproxy).

## Request Source and Orchestrator Handoff

- Spec ID: `CYNAI.PMAGNT.RequestSource` <a id="spec-cynai-pmagnt-requestsource"></a>

`cynode-pma` receives all agent-responsibility work from the **orchestrator**; it MUST NOT be invoked directly by the gateway or by external clients.
The orchestrator routes to PMA whenever inference, planning, task refinement, job dispatch, or sub-agent coordination is needed, and performs sanitization, logging, and persistence at the boundary (e.g. per the [Chat completion routing path](openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-routingpath) for chat).

### Handoff Transport (Normative)

- Orchestrator-to-PMA traffic MUST be worker-mediated (reverse proxy through the Worker API).
  The orchestrator MUST NOT rely on compose DNS or direct addressing to reach PMA.
  See [`docs/tech_specs/worker_api.md`](worker_api.md#spec-cynai-worker-managedagentproxy).

### What is Handed off to `cynode-pma`

The orchestrator MUST hand off to `cynode-pma` (in `project_manager` mode) at least the following.
In each case the orchestrator provides the necessary context (user, task, project, thread, etc.) and PMA performs the work, using MCP tools as needed and returning results or completions as appropriate.

- **Chat completion (user-facing).**
  When the **effective model identifier** for a chat request is exactly `cynodeai.pm`, the orchestrator sends the sanitized messages to PMA and expects the agent to return the completion.
  PMA may create or update tasks, call tools, and coordinate sub-agents in the process.
- **Planning and task refinement.**
  When the orchestrator has work that requires planning, task decomposition, or refinement of tasks/subtasks/acceptance criteria, it hands that work off to PMA.
  PMA plans, refines tasks (via MCP database tools), and updates state.
- **Job dispatch to worker nodes.**
  PMA issues jobs to worker nodes via MCP (e.g. `sandbox.*`, node selection, artifact handoff).
  The orchestrator routes those MCP calls; the decision of what to run, when, and on which node is PMA's.
  PMA is the single decision-maker for inference and sandbox job assignment in the MVP.
- **Spinning up analyst sub-agents.**
  When verification or focused analysis is needed for a task, PMA spawns or requests Project Analyst sub-agents (e.g. `cynode-pma` in `project_analyst` mode).
  The orchestrator facilitates spawning and hands off task-scoped verification work to the analyst; findings flow back via MCP and PMA applies them to task remediation.
- **Other inference-needed flows.**
  Any other flow where the orchestrator needs agent reasoning, tool use, or inference (e.g. **scheduled job interpretation**, run continuation, preference-driven decisions) MUST be routed to PMA rather than to a bare inference endpoint.
  When a fired schedule requires interpretation or planning, the orchestrator hands that work to PMA; the routing rule and payload semantics are defined in [Scheduled run routing to Project Manager Agent](orchestrator.md#spec-cynai-orches-scheduledrunrouting).

### What is Not Handed off to `cynode-pma`

- **Direct-inference chat.**
  When the client sends a `model` value other than `cynodeai.pm` (one of the inference model ids from `GET /v1/models`), the orchestrator MUST NOT invoke `cynode-pma` for that request.
  The orchestrator routes that request to direct inference (node-local or API Egress) and PMA does not receive it.
  All other handoff categories above still apply for other work.

See [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md) and [`docs/tech_specs/project_manager_agent.md`](project_manager_agent.md).

### Process Boundaries and Workflow Runner

**cynode-pma** (chat, MCP) and the **workflow runner** (LangGraph) are **separate processes**.
They share the MCP gateway and DB.
The orchestrator starts the workflow runner for a given task; chat and planning go to PMA; the workflow runner executes the graph and does not serve chat.
See [orchestrator.md](orchestrator.md) Workflow Engine section.

## Role Modes

- Spec ID: `CYNAI.PMAGNT.RoleModes` <a id="spec-cynai-pmagnt-rolemodes"></a>

`cynode-pma` MUST support at least two role modes.

- `project_manager`
  - Drives task execution to completion.
  - Orchestrates sandbox work, model routing, and verification.
  - Spawns task-scoped analyst sub-agents when enabled.
- `project_analyst`
  - Performs task-scoped verification for a single task.
  - Records structured findings and remediation recommendations.

Role selection MUST be explicit at runtime.

- The implementation MUST support a command-line flag: `--role=project_manager` or `--role=project_analyst`.
  The implementation MAY also allow role to be set via config file or environment variable (e.g. `PMA_ROLE`).
- When more than one source is present, precedence MUST be: **command-line flag overrides config file, config file overrides environment variable.**
  The implementation MUST apply this order so that the effective role is deterministic.
- The runtime MUST be able to run multiple instances with different roles concurrently (each instance has a single effective role for its lifetime).

## Instructions Loading and Routing

- Spec ID: `CYNAI.PMAGNT.InstructionsLoading` <a id="spec-cynai-pmagnt-instructionsloading"></a>

The agent runtime MUST load an instructions bundle for the selected role.
The instructions bundle defines:

- Output and tool-use contracts.
- Role-specific responsibilities and non-goals.
- Required references to canonical requirements and tech specs.

### Role Separation Requirement

- `project_manager` and `project_analyst` MUST load distinct instruction bundles by default.
- `project_analyst` MUST NOT reuse the Project Manager instruction bundle by default.

### Default Layout (Required)

- The implementation MUST support a configurable instructions root directory and role-specific subpaths.
- The default layout MUST be: instructions root `instructions/`, Project Manager bundle `instructions/project_manager/`, Project Analyst bundle `instructions/project_analyst/`.
- The implementation MUST allow overriding the root and each role path via configuration (flag, config file, or environment variable).
  Paths MUST be deterministic and role-separated; the same role MUST always resolve to the same bundle path for a given configuration.

## LLM Context Composition

- Spec ID: `CYNAI.PMAGNT.LLMContextComposition` <a id="spec-cynai-pmagnt-llmcontextcomposition"></a>

Traces To:

- [REQ-AGENTS-0132](../requirements/agents.md#req-agents-0132)
- [REQ-AGENTS-0133](../requirements/agents.md#req-agents-0133)
- [REQ-AGENTS-0134](../requirements/agents.md#req-agents-0134)
- [REQ-PMAGNT-0108](../requirements/pmagnt.md#req-pmagnt-0108)

When building the system message or prompt content sent to an LLM, `cynode-pma` MUST compose context in this order:

1. **Baseline context** - Identity, role, responsibilities, and non-goals for the current role (`project_manager` or `project_analyst`).
   Sourced from the role's instructions bundle or a dedicated baseline document; MUST NOT be overridden by user preferences.
2. **Role instructions** - The remainder of the instructions bundle (output contracts, tool-use contracts, references to specs).
3. **Project-level context** - When the request has an associated `project_id` and the agent has access, include project identity (id, name, slug), scope, and relevant project metadata.
   Sourced from MCP project tools or orchestrator handoff.
4. **Task-level context** - When the request has an associated `task_id`, include task identity (id, name), acceptance criteria summary, status, and relevant task metadata.
   Sourced from MCP task tools or orchestrator handoff.
5. **User-configurable additional context** - Resolved from preferences using scope precedence (task > project > user > group > system).
   Keys: `agents.project_manager.additional_context` when role is `project_manager`, `agents.project_analyst.additional_context` when role is `project_analyst`.
   See [User preferences - Agent additional context](user_preferences.md#spec-cynai-stands-agentadditionalcontext).
6. **Request-specific messages** - Conversation turns, tool results, and other request-scoped content.

The implementation MUST resolve effective preferences for the current task/request context (including `user_id`, `project_id`, `task_id`, `group_ids`) and MUST include project-level and task-level context when available and the resolved additional context in every LLM request.

## PMA Conversation History

- Spec ID: `CYNAI.PMAGNT.ConversationHistory` <a id="spec-cynai-pmagnt-conversationhistory"></a>

For multi-turn chat, prior conversation turns MUST be preserved and included in context sent to the LLM so the agent can reference earlier user and assistant messages.

### Langchain-Capable Path

- When the agent uses a LangChain-capable pipeline (e.g. LangGraph or a chain that accepts message history), prior turns MUST be included in the composed context passed to the model.
- The system-context composition (baseline, role instructions, project/task context, preferences) MUST be sent as the system block or equivalent; prior user and assistant turns MUST be sent as conversation history in the prescribed order and MUST NOT be folded into the system block.

### Executor Input

- The final input to the executor (the prompt or message array used for the current inference step) MUST treat the **last user turn** as the current user message.
- The last user turn MUST NOT be merged into the system block; it MUST remain a distinct user message so that the model correctly attributes it and can respond to it.

## Chat File Context

- Spec ID: `CYNAI.PMAGNT.ChatFileContext` <a id="spec-cynai-pmagnt-chatfilecontext"></a>

Traces To:

- [REQ-PMAGNT-0115](../requirements/pmagnt.md#req-pmagnt-0115)

When PMA receives chat input that includes accepted user-file references or resolved file content, that file context is part of the user turn and MUST remain available to the model request builder.

- PMA MUST include accepted chat-file context in the LLM request in a representation supported by the selected model path.
- For text-capable files, PMA MAY inline extracted or normalized text content.
- For image or other multimodal files, PMA SHOULD use the provider-supported file or image part format when available.
- When the selected model path cannot support a given accepted file type, PMA SHOULD return a clear user-visible error rather than silently omitting the file.

## Chat Surface Mapping

- Spec ID: `CYNAI.PMAGNT.ChatSurfaceMapping` <a id="spec-cynai-pmagnt-chatsurfacemapping"></a>

The OpenAI-compatible interactive chat surfaces exposed by the User API Gateway define a stable model id `cynodeai.pm`.
That external model id MUST map to `cynode-pma` running in `project_manager` mode for both `POST /v1/chat/completions` and `POST /v1/responses`.

This mapping is required so that:

- Open WebUI and cynork can select a stable id.
- The underlying inference model name can remain decoupled from the agent surface identity.

### Responses-Surface Handoff

- When the gateway normalizes a `POST /v1/responses` request for PMA handoff, any prior turns resolved from retained `previous_response_id` continuation state MUST be passed as conversation history in order.
- The current user input for that request MUST remain the newest user turn and MUST NOT be folded into the system block.

### Reference Contract

- [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md)

## Node-Local Inference Backend Environment

- Spec ID: `CYNAI.PMAGNT.NodeLocalInferenceEnv` <a id="spec-cynai-pmagnt-nodelocalinferenceenv"></a>

Traces To:

- [REQ-PMAGNT-0116](../requirements/pmagnt.md#req-pmagnt-0116)

When `cynode-pma` uses node-local inference through the worker-managed service contract, it MUST consume orchestrator-directed backend environment values that affect runner behavior or effective context size when those values are supplied in the managed-service inference configuration.

- Backend-derived values supplied through the managed-service inference configuration MUST be treated as authoritative runtime inputs for PMA's node-local inference path.
- PMA MUST treat those values as the orchestrator's selected effective runtime settings for the node-local backend, including context-window sizing selected to maximize the safe usable context window for the expected workload on that node.
- PMA MUST apply supported values such as `OLLAMA_NUM_CTX` to the corresponding per-request or per-runner inference options rather than ignoring them.
- When an orchestrator-directed backend environment value is not supported by the selected local inference adapter, PMA SHOULD log a bounded operator-visible warning and MUST continue using only the supported subset.
- PMA MUST NOT require direct access to worker host environment variables when the managed-service inference contract already supplies the needed backend-derived values.

## Policy and Tool Boundaries

- Spec ID: `CYNAI.PMAGNT.PolicyAndTools` <a id="spec-cynai-pmagnt-policyandtools"></a>

`cynode-pma` MUST comply with the Project Manager and Project Analyst tool access rules.
In particular:

- All PostgreSQL access MUST occur through MCP database tools.
- External provider calls MUST be routed through API Egress.
- Provider credentials MUST NOT be stored in the agent runtime.

See:

- [`docs/requirements/agents.md`](../requirements/agents.md)
- [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md)

## MCP Tool Access

- Spec ID: `CYNAI.PMAGNT.McpToolAccess` <a id="spec-cynai-pmagnt-mcptoolaccess"></a>

`cynode-pma` MUST invoke all tool operations through the orchestrator MCP gateway.
In this model, it does so via a worker-proxy URL that forwards to the orchestrator MCP gateway.
`cynode-pma` MUST NOT call orchestrator endpoints directly; the worker proxy is the single egress from the agent container.
The gateway remains the single enforcement and audit point at the orchestrator.
When making MCP requests, `cynode-pma` calls the **worker proxy** (e.g. the URL in `mcp_gateway_proxy_url`); `cynode-pma` MUST NOT receive or present an agent token.
The **worker proxy** holds the PM agent token (issued by the orchestrator, delivered to the worker in node config) and attaches it when forwarding requests to the orchestrator MCP gateway; see [Agent-Scoped Tokens or API Keys](mcp_gateway_enforcement.md#spec-cynai-mcpgat-agentscopedtokens).
The PM token is system-level (not bound to a specific user); user context for a given request comes from the conversation or task when applicable.
The gateway authenticates the token and restricts tool access to the allowlist and per-tool scope for the resolved agent type.
The gateway restricts PM/PA to tools that have **PM scope** (or **both**) in the orchestrator's per-tool scope; see [Per-tool scope: Sandbox vs PM](mcp_gateway_enforcement.md#spec-cynai-mcpgat-pertoolscope).
Tools that are sandbox-only MUST NOT be invokable by `cynode-pma`.

### Role-Based Allowlists

- When running as **project_manager**, `cynode-pma` MUST invoke only tools permitted by the [Project Manager Agent allowlist](mcp_gateway_enforcement.md#spec-cynai-mcpgat-pmagentallowlist).
  That allowlist includes `db.*`, `node.*`, `sandbox.*`, `artifact.*`, `model.*`, `connector.*`, `web.fetch`, `web.search`, `api.call`, `git.*`, `help.*`, and when the system setting permits, `sandbox.allowed_images.list` and `sandbox.allowed_images.add`.
- When running as **project_analyst**, `cynode-pma` MUST invoke only tools permitted by the [Project Analyst Agent allowlist](mcp_gateway_enforcement.md#spec-cynai-mcpgat-paagentallowlist).
  That allowlist includes limited `db.*`, `artifact.*`, `web.fetch`, `web.search`, `api.call`, and `help.*`.

Admin-configurable per-tool enable/disable and access control rules further restrict which tools succeed; the agent MUST treat gateway rejections as hard failures.

See:

- [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)
- [`docs/tech_specs/mcp_tool_catalog.md`](mcp_tool_catalog.md)

## LLM via API Egress

- Spec ID: `CYNAI.PMAGNT.LLMViaApiEgress` <a id="spec-cynai-pmagnt-llmviaapiegress"></a>

Traces To:

- [REQ-PMAGNT-0109](../requirements/pmagnt.md#req-pmagnt-0109)

When the orchestrator provides an LLM API key (or equivalent credential) for the Project Manager Agent via the API Egress Server (e.g. configured external provider and key for PMA inference), `cynode-pma` MUST be able to use that provider for inference.
PMA MUST route such inference requests through the API Egress Server so credentials are not exposed to the agent process.
See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md) and [`docs/tech_specs/external_model_routing.md`](external_model_routing.md).

## PMA Informs Orchestrator When Online

- Spec ID: `CYNAI.PMAGNT.PmaInformsOrchestratorOnline` <a id="spec-cynai-pmagnt-pmainformsorchestratoronline"></a>

Traces To:

- [REQ-PMAGNT-0110](../requirements/pmagnt.md#req-pmagnt-0110)
- [REQ-ORCHES-0151](../requirements/orches.md#req-orches-0151)

`cynode-pma` MUST expose a health endpoint so the worker can determine readiness.
The orchestrator MUST learn that PMA is online via worker-reported managed service status (and endpoints), not by probing a PMA listen address directly.

### Normative Behavior

- `cynode-pma` MUST expose `GET /healthz` that returns HTTP 200 when the agent is online.
- The worker MUST health check PMA per its managed service health contract and report `state=ready` and endpoint(s) to the orchestrator.
  See [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md) `managed_services_status`.
- The orchestrator MUST use the worker-reported status and endpoint(s) to determine readiness.

## Skills and Default Skill

- Spec ID: `CYNAI.PMAGNT.SkillsAndDefaultSkill` <a id="spec-cynai-pmagnt-skillsanddefaultskill"></a>

When the inference backend used by `cynode-pma` supports **skills**, the system MUST supply the **default CyNodeAI interaction skill** to each inference request so the agent has consistent guidance on MCP tools, gateway usage, and conventions.
See [Default CyNodeAI Interaction Skill](skills_storage_and_inference.md#spec-cynai-skills-defaultcynodeaiskill) and [REQ-SKILLS-0116](../requirements/skills.md#req-skills-0116).

### Skills MCP Tools

- When the gateway allowlist and access control permit, `cynode-pma` MAY use the MCP skills tools `skills.create`, `skills.list`, `skills.get`, `skills.update`, and `skills.delete`.
  Permission is determined by the role allowlist in [mcp_gateway_enforcement.md](mcp_gateway_enforcement.md) and per-tool access control; the implementation MUST NOT invoke a skills tool if the gateway rejects the call.
- All skill tool invocations are audited and subject to the same scope and malicious-pattern checks as web and CLI.

See:

- [`docs/tech_specs/skills_storage_and_inference.md`](skills_storage_and_inference.md)
- [`docs/tech_specs/mcp_tool_catalog.md`](mcp_tool_catalog.md#spec-cynai-mcptoo-skillstools)

## Configuration Surface

This section defines the minimum configuration surface for `cynode-pma`.

### Required Configuration Values

- Role mode selection (command-line flag; override via config file or environment when supported).
- Instructions bundle root and role-specific bundle paths (configurable; defaults as in [Instructions Loading and Routing](#instructions-loading-and-routing)).
- Inference connectivity configuration (how PMA obtains inference and connection details), supplied by the orchestrator in the PMA managed service start bundle.
- Worker-proxy endpoints for agent-to-orchestrator communication:
  - Worker-proxy URL for MCP gateway calls.
  - Worker-proxy URL for any required callback/ready signaling.

### Optional Configuration Values

- Feature toggles for spawning analyst sub-agents.
- Concurrency limits for analyst sub-agents.

### Config Keys Alignment

- When `cynode-pma` is run in `project_manager` role, the preference keys defined for the Project Manager in [`user_preferences.md`](user_preferences.md) and [`external_model_routing.md`](external_model_routing.md) apply (including `agents.project_manager.model_routing.prefer_local`, `agents.project_manager.model_routing.allowed_external_providers`, `agents.project_manager.model_routing.fallback_provider_order`).
- When `cynode-pma` is run in `project_analyst` role, the preference keys defined for the Project Analyst in those same specs apply (including `agents.project_analyst.model_routing.*`).
  The implementation MUST NOT apply Project Manager preference keys to an instance running as Project Analyst, and vice versa.
