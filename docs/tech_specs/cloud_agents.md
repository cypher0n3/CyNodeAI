# Cloud-Based Agents

- [Document Overview](#document-overview)
- [Goals and Non-Goals](#goals-and-non-goals)
- [Definitions](#definitions)
- [Architecture Options](#architecture-options)
- [Worker API Contract](#worker-api-contract)
- [Authentication and Trust](#authentication-and-trust)
- [Capability Reporting](#capability-reporting)
- [Dispatch, Routing, and Policy](#dispatch-routing-and-policy)
- [Tool Access Model](#tool-access-model)
- [Execution Environments](#execution-environments)
- [Security and Auditing](#security-and-auditing)
- [Practical Deployment Examples](#practical-deployment-examples)
  - [Example 1: Provider-Backed Cloud Worker for OpenAI (ChatGPT)](#example-1-provider-backed-cloud-worker-for-openai-chatgpt)
  - [Example 2: No Cloud Worker, Orchestrator Direct Routing](#example-2-no-cloud-worker-orchestrator-direct-routing)
- [Operational Notes](#operational-notes)

## Document Overview

This document defines how cloud-based agents participate in CyNodeAI as first-class workers.
Cloud-based agents use the same registration, capability reporting, and job dispatch contract as local worker nodes.

## Goals and Non-Goals

Goals

- Enable burst capacity by adding cloud-hosted workers that can run jobs when local nodes are saturated.
- Support provider-backed execution for tasks that require external model APIs or specific cloud regions.
- Preserve CyNodeAI security properties by keeping API keys in the orchestrator and enforcing policy centrally.

Non-goals

- This document does not define the full Worker API schema.
- This document does not define the full sandbox security model for local execution.

See [`docs/tech_specs/node.md`](node.md) for local node details.

## Definitions

- **Cloud-based agent**
  - A worker that runs outside the local network and is operated by the user in a cloud environment.
- **Cloud worker**
  - A cloud-based agent that implements the Worker API surface.
- **Provider-backed worker**
  - A cloud worker that performs LLM inference by calling external providers (OpenAI, Anthropic) instead of running Ollama locally.
- **Orchestrator direct routing**
  - A mode where the orchestrator calls external model APIs through API Egress without dispatching to any worker.

## Architecture Options

CyNodeAI supports multiple ways to incorporate cloud compute.

- **Option A: Full cloud worker**
  - Deploy a cloud worker that implements the Worker API.
  - The orchestrator dispatches jobs to it the same way it dispatches to local workers.
- **Option B: Provider-backed cloud worker**
  - Deploy a cloud worker that implements the Worker API and uses external model APIs for inference.
  - External calls MUST be made through CyNodeAI's API Egress so provider credentials are not stored on the cloud worker.
- **Option C: Orchestrator direct routing**
  - Enable external model routing and allow the orchestrator to call providers through API Egress.
  - This is useful when you want external models but do not need a cloud worker.

See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md) and [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).

## Worker API Contract

Cloud workers implement the same Worker API surface as local workers.
The orchestrator treats cloud workers as additional capacity and schedules jobs based on capability, load, data locality preferences, and policy.

Minimum endpoints

- `POST /register`
- `GET /jobs` or `POST /jobs/poll`
- `POST /status`
- `POST /result`

Normative requirements

- A cloud worker MUST NOT access PostgreSQL directly.
- A cloud worker MUST use orchestrator-mediated tool access and MUST NOT embed provider API keys.
- A cloud worker MUST support job retries and idempotency semantics as defined by the orchestrator.

## Authentication and Trust

Cloud workers authenticate to the orchestrator using the same model as local nodes.

Recommended flow

- A cloud worker registers using a pre-shared key (PSK).
- The orchestrator issues a JWT for ongoing calls.
- The cloud worker uses the JWT for all subsequent calls to orchestrator endpoints.

Security notes

- Cloud workers SHOULD use TLS when connecting to the orchestrator.
- The orchestrator SHOULD support credential rotation for PSKs and JWT signing keys.
- Cloud workers SHOULD be uniquely identified and labeled to enable tight policy rules.

## Capability Reporting

Cloud workers MUST provide a capability report at registration and on startup.
The orchestrator uses this report for scheduling and policy decisions.

Recommended fields

- Identity
  - `node_id`
  - `labels` (e.g. `cloud`, `provider_openai`, `provider_anthropic`)
- Location
  - `region`
  - `provider` (the cloud provider where the worker runs)
- Execution mode
  - `inference_mode` (e.g. `provider_api`, `ollama_remote`, `ollama_local`)
- Model support
  - `supported_providers` (e.g. `openai`, `anthropic`)
  - `supported_models` (optional allowlist)
- Limits
  - `max_concurrency`
  - `rate_limits` (requests per minute, tokens per minute when available)
- Connectivity
  - `reachable_endpoints` (orchestrator URL reachability and tool endpoints)

## Dispatch, Routing, and Policy

The orchestrator selects a worker using routing signals and policy constraints.
Cloud selection MUST be explicitly allowed by policy and user preferences.

Routing signals

- Capability match.
- Current worker load.
- Data locality preference.
- User override selecting a specific external provider when allowed.
- Cost limits and provider allowlists.
- Regional constraints.

Policy integration

- Cloud workers are subject to the same access control stance as local workers.
- The orchestrator SHOULD be the first gate and the target service SHOULD be the final gate.

See [`docs/tech_specs/access_control.md`](access_control.md).

## Tool Access Model

Cloud workers use the same tool model as local workers.
Tool access is mediated through the orchestrator and MCP.

Normative requirements

- Cloud workers MUST call tools through the orchestrator MCP gateway.
- Cloud workers MUST NOT call arbitrary outbound network endpoints directly unless explicitly allowed by policy.
- Cloud workers MUST use API Egress for external API calls.

Relevant specs

- MCP tooling: [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md)
- API Egress: [`docs/tech_specs/api_egress_server.md`](api_egress_server.md)
- Secure Browser: [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md)

## Execution Environments

Cloud workers may run in different execution environments.
CyNodeAI does not require the orchestrator to manage cloud worker lifecycle.

Common patterns

- Provider-managed container platforms (Cloud Run, ECS/Fargate, Fly.io).
- Kubernetes deployments.
- Long-running VM service with a process supervisor.

Sandbox considerations

- Local nodes rely on Docker or Podman for sandbox isolation (Podman preferred for rootless).
- Cloud workers MAY implement an equivalent sandbox model, but this is provider-specific.
- Jobs MUST remain portable so that tasks can be retried on local or cloud workers when needed.

## Security and Auditing

Cloud workers MUST preserve the same core security properties as local workers.

Key requirements

- Provider API keys and secrets remain in the orchestrator.
- External calls MUST go through API Egress, which injects credentials server-side and enforces policy.
- Requests and decisions SHOULD be audited with task context and subject identity.

See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md) and [`docs/tech_specs/access_control.md`](access_control.md).

## Practical Deployment Examples

The examples below are practical setups that follow the CyNodeAI security model.
They focus on configuration and message flow rather than production-ready code.

### Example 1: Provider-Backed Cloud Worker for OpenAI (ChatGPT)

Intended use

- You want burst capacity in the cloud.
- You want the worker to use OpenAI models through API Egress.

Setup

- Store the OpenAI API key in CyNodeAI as an API Egress credential.
- Create an ACL rule allowing the cloud worker subject to call `api.call` for `provider=openai` and the specific model operations you allow.
- Deploy a cloud worker container configured to register with the orchestrator.

Example environment variables

- `CYNODE_ORCH_URL=https://orch.example.com`
- `CYNODE_NODE_ID=cloud-us-east-1-01`
- `CYNODE_REGISTER_PSK=...`
- `CYNODE_LABELS=cloud,provider_openai`
- `CYNODE_CAP_REGION=us-east-1`
- `CYNODE_CAP_SUPPORTED_PROVIDERS=openai`
- `CYNODE_CAP_RATE_LIMIT_RPM=300`

Job flow

- Cloud worker registers with PSK and receives a JWT.
- Orchestrator dispatches a job to the cloud worker.
- Cloud worker requests an OpenAI completion by calling the orchestrator tool endpoint for API Egress.
- API Egress enforces ACL, injects the OpenAI API key, executes the call, audits it, and returns the response.

### Example 2: No Cloud Worker, Orchestrator Direct Routing

This setup uses External Model Routing without deploying any cloud worker.
The orchestrator routes approved model calls to API Egress directly.

See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md).

## Operational Notes

- Cloud workers SHOULD report health and load so scheduling decisions remain accurate.
- The orchestrator SHOULD support disabling cloud routing quickly through preferences or policy.
- Cost controls SHOULD be enforced by policy and by per-task constraints.
