# External Model Routing

- [Document Overview](#document-overview)
- [Routing Goal](#routing-goal)
- [Routing Signals](#routing-signals)
- [Routing Policy](#routing-policy)
- [External Provider Integration](#external-provider-integration)
- [External Inference With Node Sandboxes](#external-inference-with-node-sandboxes)
- [Preferences and Constraints](#preferences-and-constraints)
  - [Suggested Preference Keys](#suggested-preference-keys)
  - [Suggested Orchestrator-Side Agent Preference Keys](#suggested-orchestrator-side-agent-preference-keys)
- [Auditing and Safety](#auditing-and-safety)

## Document Overview

This document defines how the orchestrator decides between local worker execution and external AI API calls.
External calls are intended as an allowed fallback when local workers are overloaded or cannot satisfy capability requirements.

## Routing Goal

The orchestrator SHOULD prefer local execution when it can meet required capabilities and latency objectives.
The orchestrator MUST be able to route to configured external AI APIs when needed and when policy allows it.

## Routing Signals

The orchestrator SHOULD consider the following signals when selecting an execution target.

- Capability match
  - Required model family, context length, tool support, GPU requirement, or other declared capabilities.
- Worker load
  - Current concurrency, queue depth, average job latency, and health status.
- Data locality
  - Whether the required artifacts and sandbox execution are already on a given worker host.
- Task constraints
  - Deadline, maximum acceptable latency, and required reliability.
- User override
  - When a user explicitly requests an external provider (e.g. "route to ChatGPT"), if policy allows it.
- Policy constraints
  - Whether external providers are allowed for the requesting user, project, or task.

## Routing Policy

Recommended default routing behavior.

- The orchestrator MUST attempt local execution when a worker can satisfy capability requirements and is not overloaded.
- The orchestrator SHOULD route to an external provider when:
  - no available worker can satisfy capability requirements, or
  - workers are overloaded beyond configured thresholds, or
  - there are no registered worker nodes and the orchestrator is operating in standalone mode, or
  - the task is marked as external-allowed and requires low latency that local capacity cannot meet.
- The orchestrator SHOULD honor a user override selecting a specific external provider when policy allows it.
- The orchestrator MUST deny external routing when policy does not allow it.
- The orchestrator SHOULD record the routing decision and the primary reasons for it.

## External Provider Integration

External model calls MUST be performed through the API Egress Server so credentials are not exposed to agents or sandbox containers.

Integration expectations

- The orchestrator routes an approved request to the API Egress Server with:
  - `provider` and `operation` representing the external model call
  - `params` containing prompt inputs and request options
  - `task_id` for traceability
- The API Egress Server selects the appropriate user-scoped credential and performs the outbound call.
- The orchestrator receives the result and stores it in PostgreSQL as part of task history.

## External Inference With Node Sandboxes

External inference and sandbox execution are separate concerns.
The orchestrator SHOULD support workflows where a task uses an external provider for model inference while still running tools inside a sandbox container on a node.

Motivation

- Many tasks require deterministic tooling that should run in a sandbox even if model inference is external.
- Nodes without GPUs can still provide useful capacity for sandbox execution.

Recommended behavior

- The orchestrator routes external model calls through API Egress.
- The orchestrator dispatches sandbox execution to a selected node based on capability and policy.
- The orchestrator SHOULD allow selecting a sandbox-only node for tool execution.
- Sandboxes MUST not receive provider API keys.
- Sandboxes SHOULD access external capabilities only through orchestrator-mediated MCP tools.

Example flow

- A task step requires an external model call for planning or code generation.
- The orchestrator performs the model call through API Egress and records the response.
- The orchestrator dispatches a sandbox job to run tests or apply changes.
- The sandbox job uses MCP tools for artifacts and controlled services.

## Preferences and Constraints

Routing behavior SHOULD be configurable via PostgreSQL preferences.

### Suggested Preference Keys

- `model_routing.prefer_local` (boolean)
- `model_routing.allowed_external_providers` (array)
- `model_routing.fallback_provider_order` (array)
- `model_routing.overload.queue_depth_threshold` (number)
- `model_routing.overload.latency_seconds_threshold` (number)
- `model_routing.max_external_tokens` (number)
- `model_routing.max_external_cost_usd` (number)

### Suggested Orchestrator-Side Agent Preference Keys

Orchestrator-side agents MAY use separate preferences for external provider routing.
This allows enabling external providers for the Project Manager and Project Analyst agents without changing task routing defaults.

- `agents.project_manager.model_routing.prefer_local` (boolean)
- `agents.project_manager.model_routing.allowed_external_providers` (array)
- `agents.project_manager.model_routing.fallback_provider_order` (array)
- `agents.project_analyst.model_routing.prefer_local` (boolean)
- `agents.project_analyst.model_routing.allowed_external_providers` (array)
- `agents.project_analyst.model_routing.fallback_provider_order` (array)

## Auditing and Safety

- The orchestrator SHOULD log each external routing decision, including chosen provider and high-level reason.
- The API Egress Server MUST log each outbound call with task context and subject identity.
- Responses SHOULD be filtered for secret leakage and stored with least privilege.
