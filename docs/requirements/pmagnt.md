# PMAGNT Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `PMAGNT` domain.
It defines the normative requirements for the `cynode-pma` agent binary.

`cynode-pma` is the concrete implementation of:

- The Project Manager Agent.
- The Project Analyst Agent, running in a distinct role mode with separate instructions.

This domain is intentionally narrow and implementation-oriented.
Behavioral and workflow requirements still live in `AGENTS` and `ORCHES`.

## 2 Requirements

- **REQ-PMAGNT-0001:** The system MUST provide an orchestrator-side agent binary named `cynode-pma` that can operate in at least two role modes: `project_manager` and `project_analyst`.
  [CYNAI.PMAGNT.Doc.CyNodePma](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-doc-cynodepma)
  <a id="req-pmagnt-0001"></a>

- **REQ-PMAGNT-0100:** `cynode-pma` MUST support explicit role selection via configuration (flag or equivalent) and MUST enforce role separation by loading a distinct instructions bundle per role.
  [CYNAI.PMAGNT.RoleModes](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-rolemodes)
  <a id="req-pmagnt-0100"></a>

- **REQ-PMAGNT-0101:** When `cynode-pma` is running in `project_analyst` mode, it MUST use a separate instructions path from `project_manager` mode and MUST NOT reuse the Project Manager instructions bundle by default.
  [CYNAI.PMAGNT.InstructionsLoading](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-instructionsloading)
  <a id="req-pmagnt-0101"></a>

- **REQ-PMAGNT-0102:** The mapping between the OpenAI-compatible chat model id `cynodeai.pm` and the internal agent implementation MUST be documented as `cynodeai.pm` => `cynode-pma` in `project_manager` mode.
  [CYNAI.PMAGNT.ChatSurfaceMapping](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-chatsurfacemapping)
  [CYNAI.USRGWY.OpenAIChatApi.Endpoints](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-endpoints)
  <a id="req-pmagnt-0102"></a>

- **REQ-PMAGNT-0103:** `cynode-pma` MUST comply with the Project Manager and Project Analyst tool access and policy constraints defined in the `AGENTS` domain.
  [REQ-AGENTS-0002](./agents.md#req-agents-0002)
  [REQ-AGENTS-0003](./agents.md#req-agents-0003)
  [CYNAI.PMAGNT.PolicyAndTools](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-policyandtools)
  <a id="req-pmagnt-0103"></a>

- **REQ-PMAGNT-0104:** `cynode-pma` MUST emit audit or observability records for decisions that affect dispatch, model routing, or task state (e.g. role selection, instructions path chosen, tool allowlist scope).
  [CYNAI.PMAGNT.Doc.CyNodePma](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-doc-cynodepma)
  <a id="req-pmagnt-0104"></a>

- **REQ-PMAGNT-0105:** `cynode-pma` MUST expose a configurable surface (e.g. flags or environment variables) for role mode and instructions bundle paths, with safe defaults that prevent accidental reuse of the wrong role bundle.
  [CYNAI.PMAGNT.RoleModes](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-rolemodes)
  [CYNAI.PMAGNT.InstructionsLoading](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-instructionsloading)
  <a id="req-pmagnt-0105"></a>

- **REQ-PMAGNT-0106:** `cynode-pma` MUST invoke all tool operations through the orchestrator MCP gateway and MUST use only tools permitted by the gateway role-based allowlist for its current role (project_manager or project_analyst).
  [CYNAI.PMAGNT.McpToolAccess](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-mcptoolaccess)
  [CYNAI.MCPGAT.Doc.GatewayEnforcement](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-doc-gatewayenforcement)
  <a id="req-pmagnt-0106"></a>

- **REQ-PMAGNT-0107:** When the inference backend used by `cynode-pma` supports skills, the system MUST supply the default CyNodeAI interaction skill to that inference request; `cynode-pma` MAY use MCP skills tools when the gateway allowlist and access control permit.
  [REQ-SKILLS-0116](./skills.md#req-skills-0116)
  [CYNAI.PMAGNT.SkillsAndDefaultSkill](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-skillsanddefaultskill)
  <a id="req-pmagnt-0107"></a>

- **REQ-PMAGNT-0108:** `cynode-pma` MUST pass baseline context (agent identity, role, responsibilities, non-goals) to every LLM it uses, and MUST include user-configurable additional context resolved from preferences in LLM prompts.
  [REQ-AGENTS-0132](./agents.md#req-agents-0132)
  [REQ-AGENTS-0133](./agents.md#req-agents-0133)
  [CYNAI.AGENTS.LLMContext](../tech_specs/project_manager_agent.md#spec-cynai-agents-llmcontext)
  [CYNAI.PMAGNT.LLMContextComposition](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-llmcontextcomposition)
  <a id="req-pmagnt-0108"></a>
- **REQ-PMAGNT-0109:** `cynode-pma` MUST be able to use an LLM via the API Egress Server when an LLM API key (or equivalent credential) is provided for PMA through the orchestrator (e.g. configured provider and key for PMA inference).
  [CYNAI.PMAGNT.LLMViaApiEgress](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-llmviaapiegress)
  <a id="req-pmagnt-0109"></a>
- **REQ-PMAGNT-0110:** `cynode-pma` MUST inform the orchestrator when it has come online (e.g. by responding to a health check or by a registration/ready callback) so that the orchestrator can use it and update its own readiness state.
  [CYNAI.PMAGNT.PmaInformsOrchestratorOnline](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-pmainformsorchestratoronline)
  <a id="req-pmagnt-0110"></a>

- **REQ-PMAGNT-0111:** When the user's request implies multi-step or project-scoped effort, the Project Manager Agent (cynode-pma in project_manager mode) SHOULD build a project plan (tasks and execution order) from user input before creating orchestrator tasks and handing work off for execution.
  [CYNAI.AGENTS.ProjectPlanBuilding](../tech_specs/project_manager_agent.md#spec-cynai-agents-projectplanbuilding)
  <a id="req-pmagnt-0111"></a>
- **REQ-PMAGNT-0112:** The Project Manager Agent SHOULD ask the user clarifying questions when scope, acceptance criteria, or execution order are ambiguous, and SHOULD prefer clarification over inferring and creating tasks immediately.
  [CYNAI.AGENTS.ClarificationBeforeExecution](../tech_specs/project_manager_agent.md#spec-cynai-agents-clarificationbeforeexecution)
  <a id="req-pmagnt-0112"></a>
- **REQ-PMAGNT-0113:** The Project Manager Agent SHOULD refine project plans as needed based on updated information from the user (e.g. after clarification or change requests).
  [CYNAI.AGENTS.ProjectPlanBuilding](../tech_specs/project_manager_agent.md#spec-cynai-agents-projectplanbuilding)
  <a id="req-pmagnt-0113"></a>
- **REQ-PMAGNT-0114:** When a project plan is locked, the Project Manager Agent MUST NOT change the plan or its tasks; it MAY update completion status and comments on plans and tasks only.
  [CYNAI.AGENTS.WhenPlanLocked](../tech_specs/project_manager_agent.md#spec-cynai-agents-whenplanlocked)
  <a id="req-pmagnt-0114"></a>
- **REQ-PMAGNT-0115:** When the Project Manager Agent receives chat input that includes user-file content or stable file references accepted under the gateway chat contract, the agent MUST include that file context in the LLM request in a form the target model supports.
  The agent MUST NOT silently strip or ignore accepted chat file inputs.
  [CYNAI.PMAGNT.ChatFileContext](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-chatfilecontext)
  <a id="req-pmagnt-0115"></a>
- **REQ-PMAGNT-0116:** When `cynode-pma` uses a node-local inference backend, it MUST consume orchestrator-directed backend environment values that affect effective model context or runner behavior when those values are supplied through the managed-service inference contract.
  PMA MUST apply those values consistently to its inference requests instead of silently ignoring them so that PMA uses the same orchestrator-derived effective context-window settings as the node-local backend it depends on.
  [CYNAI.PMAGNT.NodeLocalInferenceEnv](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-nodelocalinferenceenv)
  <a id="req-pmagnt-0116"></a>
- **REQ-PMAGNT-0117:** When PMA inference output includes hidden thinking intermixed with visible assistant text, `cynode-pma` MUST separate the hidden thinking from the visible assistant text before returning the assistant turn upstream.
  Literal hidden-thinking wrappers or payloads such as model-emitted `<think>...</think>` blocks MUST NOT leak into canonical visible assistant text.
  [CYNAI.PMAGNT.ThinkingContentSeparation](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-thinkingcontentseparation)
  <a id="req-pmagnt-0117"></a>
- **REQ-PMAGNT-0118:** When `cynode-pma` serves an interactive chat turn through the PMA chat surface, it MUST support incremental assistant-output streaming on the standard interactive path instead of buffering all visible text until the full turn completes.
  Hidden thinking or reasoning MUST remain separate from visible text throughout that streaming path.
  [CYNAI.PMAGNT.StreamingAssistantOutput](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-streamingassistantoutput)
  <a id="req-pmagnt-0118"></a>
- **REQ-PMAGNT-0119:** The PMA MUST title the thread automatically after the first user message in the thread if the user has not already titled the thread.
  Auto-titling MUST NOT overwrite an existing title set by the user.
  [CYNAI.AGENTS.ThreadTitling](../tech_specs/project_manager_agent.md#spec-cynai-agents-threadtitling)
  <a id="req-pmagnt-0119"></a>
- **REQ-PMAGNT-0120:** PMA MUST implement a streaming LLM wrapper that satisfies the `langchaingo` `llms.Model` interface and tees Ollama tokens to both the output NDJSON stream and an internal buffer for `langchaingo` consumption, emitting `iteration_start` boundary events before each agent iteration.
  [CYNAI.PMAGNT.StreamingLLMWrapper](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-streamingllmwrapper)
  <a id="req-pmagnt-0120"></a>
- **REQ-PMAGNT-0121:** PMA MUST implement a configurable streaming state machine that classifies arriving tokens as visible text, thinking content, or tool-call content and routes each to the correct NDJSON event type, defaulting to recognizing `<think>`/`</think>` and tool-call markers.
  [CYNAI.PMAGNT.StreamingTokenStateMachine](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-streamingtokenstatemachine)
  <a id="req-pmagnt-0121"></a>
- **REQ-PMAGNT-0122:** PMA MUST emit full thinking content as `thinking` NDJSON events (not suppressed or summarized) so downstream clients can store and optionally display it.
  [CYNAI.PMAGNT.PMAStreamingNDJSONFormat](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-pmastreamingndjsonformat)
  <a id="req-pmagnt-0122"></a>
- **REQ-PMAGNT-0123:** PMA MUST emit tool-call content detected by the streaming state machine as `tool_call` NDJSON events, suppressed from the visible-text stream.
  [CYNAI.PMAGNT.PMAStreamingNDJSONFormat](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-pmastreamingndjsonformat)
  <a id="req-pmagnt-0123"></a>
- **REQ-PMAGNT-0124:** PMA MUST support per-iteration scoped and per-turn scoped overwrite events in the NDJSON stream for retroactive correction of leaked tokens, secret redaction, and agent output correction.
  [CYNAI.PMAGNT.PMAStreamingOverwrite](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-pmastreamingoverwrite)
  <a id="req-pmagnt-0124"></a>
- **REQ-PMAGNT-0125:** PMA MUST run an opportunistic secret scan on all accumulated content types (visible text, thinking, tool-call) after each LLM iteration and MUST emit an overwrite event if secrets are detected.
  [CYNAI.PMAGNT.PMAOpportunisticSecretScan](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-pmaopportunisticsecretscan)
  <a id="req-pmagnt-0125"></a>
- **REQ-PMAGNT-0126:** PMA MUST wrap secret-bearing stream buffer code paths inside `runtime/secret` (`secret.Do`) when available, per REQ-STANDS-0133.
  [CYNAI.PMAGNT.StreamingLLMWrapper](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-streamingllmwrapper)
  <a id="req-pmagnt-0126"></a>
- **REQ-PMAGNT-0127:** When creating or updating tasks, the PMA SHOULD set persona_id and recommended_skill_ids when the work is best performed by a specific execution-role persona; the PMA MAY hand off a bundle of 1-3 tasks (same persona, dependency order) for SBA execution in series in one job.
  [CYNAI.AGENTS.PersonaAssignment](../tech_specs/project_manager_agent.md#spec-cynai-agents-personaassignment)
  [Persona tools](../tech_specs/mcp_tools/persona_tools.md#spec-cynai-mcptoo-personatools)
  <a id="req-pmagnt-0127"></a>
- **REQ-PMAGNT-0128:** The PMA MUST attempt to clarify ambiguous tasks with the user before marking a task as `planning_state=ready` (part of its task creation/management skill).
  Clarification MUST occur in the thread where the user directed task creation, or via notification to the user (Notification spec TBD).
  [CYNAI.AGENTS.TaskReviewAndReadyTransition](../tech_specs/project_manager_agent.md#spec-cynai-agents-taskreviewandreadytransition)
  [CYNAI.AGENTS.ClarificationBeforeExecution](../tech_specs/project_manager_agent.md#spec-cynai-agents-clarificationbeforeexecution)
  <a id="req-pmagnt-0128"></a>
