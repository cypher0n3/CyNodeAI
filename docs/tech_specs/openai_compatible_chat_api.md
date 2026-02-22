# OpenAI-Compatible Chat API

- [Document Overview](#document-overview)
- [Single Chat Surface](#single-chat-surface)
- [Compatibility Goals](#compatibility-goals)
- [Endpoints](#endpoints)
- [Conversation Model](#conversation-model)
- [Chat Completion Routing Path](#chat-completion-routing-path)
- [Tasks Versus Chat (Non-Goals)](#tasks-versus-chat-non-goals)
- [Authentication, Policy, and Auditing](#authentication-policy-and-auditing)
- [Gateway Timeouts and Long-Running Behavior](#gateway-timeouts-and-long-running-behavior)
- [Reliability Requirements](#reliability-requirements)
- [Error Semantics](#error-semantics)
- [Observability](#observability)
- [Request Processing Pipeline](#request-processing-pipeline)
  - [Pipeline Steps (Order is Mandatory)](#pipeline-steps-order-is-mandatory)
- [Optional: Async Chat (Deferred)](#optional-async-chat-deferred)
- [Related Documents](#related-documents)

## Document Overview

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi` <a id="spec-cynai-usrgwy-openaichatapi"></a>

This spec defines the OpenAI-compatible chat interface exposed by the User API Gateway.
It is the **only** interactive chat surface for Open WebUI, cynork, and E2E.

Compatibility contract (pinned as of 2026-02-22):

- The OpenAI-compatible surface in this spec is pinned to the OpenAI Chat Completions API as documented in the OpenAI API Reference.
- The OpenAI REST API version header reported by the OpenAI API Overview is `openai-version: 2020-10-01` as of 2026-02-22.
- Reference: [OpenAI API Overview](https://platform.openai.com/docs/api-reference) and [Chat Completions API Reference](https://platform.openai.com/docs/api-reference/chat).

Traces To:

- [REQ-USRGWY-0121](../requirements/usrgwy.md#req-usrgwy-0121)
- [REQ-USRGWY-0127](../requirements/usrgwy.md#req-usrgwy-0127)

## Single Chat Surface

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.SingleSurface` <a id="spec-cynai-usrgwy-openaichatapi-singlesurface"></a>

The User API Gateway MUST expose interactive chat only through the OpenAI-compatible API surface.
There is no separate legacy chat endpoint.

Traces To:

- [REQ-USRGWY-0127](../requirements/usrgwy.md#req-usrgwy-0127)

## Compatibility Goals

The gateway MUST support:

- Open WebUI as an OpenAI-compatible client.
- Cynork chat as an OpenAI-compatible client.
- E2E scenarios that exercise the same endpoints.

Compatibility layers MUST preserve orchestrator policy constraints and MUST NOT bypass auditing.

Traces To:

- [REQ-USRGWY-0121](../requirements/usrgwy.md#req-usrgwy-0121)

## Endpoints

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.Endpoints` <a id="spec-cynai-usrgwy-openaichatapi-endpoints"></a>

The gateway MUST provide:

- `GET /v1/models` in OpenAI list-models format.
- `POST /v1/chat/completions` in OpenAI chat-completions format.

The gateway MUST accept an OpenAI-format request body containing `messages: [{ role, content }, ...]`.
The gateway MUST return an OpenAI-format response containing the completion content at `choices[0].message.content`.

The gateway MUST accept the OpenAI `model` field when provided.
If `model` is omitted or empty, the gateway MUST use a default model identifier.
The default MUST correspond to the PM/PA chat surface for typical user chat.

Forward compatibility:

- The gateway MUST ignore unknown request fields in the OpenAI chat-completions request body.
- The gateway MUST ignore unknown fields inside message objects.

Project scoping:

- If an OpenAI-standard `OpenAI-Project` request header is present, the gateway MUST treat its value as the project context for persistence.
- If the header is absent, the gateway MUST associate the thread (and any tasks created in that context) with the creating user's default project (see [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104) and [Default project](../tech_specs/projects_and_scopes.md#spec-cynai-access-defaultproject)).

Model identifiers:

- The gateway MUST expose a stable PM/PA chat surface model id `cynodeai.pm`.
- When the client omits `model` or provides an empty `model`, the gateway MUST behave as if `model` was `cynodeai.pm`.
- The internal implementation of the PM/PA chat surface is the `cynode-pma` binary (see [cynode_pma.md](cynode_pma.md)); the external id `cynodeai.pm` is stable and MUST NOT change for client compatibility.
- The gateway MUST also expose underlying inference model identifiers in `GET /v1/models`.
  These identifiers MUST be limited to the currently configured inference model(s) that the authenticated user is authorized to use.
  The gateway MUST NOT disclose model identifiers the user is not authorized to use.

## Conversation Model

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.ConversationModel` <a id="spec-cynai-usrgwy-openaichatapi-conversationmodel"></a>

Chat is a conversation with the PM/PA (Project Manager / Project Analyst) agent surface.
A chat completion is message-in and completion-out.

Conversation state and history are tracked separately from tasks.
Chat messages are stored as chat-thread messages (see [Chat Threads and Messages](chat_threads_and_messages.md)).

Thread identifiers and association:

- The OpenAI-compatible surface MUST NOT require any CyNodeAI-specific thread or session identifiers in request bodies or headers.
- The orchestrator MUST manage chat thread association server-side.
- The orchestrator MUST maintain a single active thread per `(user_id, project_id)` scope.
  The orchestrator MUST rotate to a new active thread after 2 hours of inactivity.

Traces To:

- [REQ-USRGWY-0130](../requirements/usrgwy.md#req-usrgwy-0130)

## Chat Completion Routing Path

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.RoutingPath` <a id="spec-cynai-usrgwy-openaichatapi-routingpath"></a>

All OpenAI-compliant chat/completion requests are processed by the **orchestrator** first.
The orchestrator is the single point of entry for request handling, policy, and routing.
The gateway MUST NOT send chat completion requests directly to an agent or inference endpoint; every request MUST flow through the orchestrator.

Effective model identifier

- The **effective model identifier** for routing is the request body `model` field if present and non-empty; otherwise it is `cynodeai.pm`.

Orchestrator responsibilities before routing

- Perform automatic **sanitization**: detect and redact secrets in message content (at least API keys), replace detected secrets with the literal `SECRET_REDACTED`, and record redaction in audit log.
- Perform **logging** and audit recording for the request.
- Persist the amended user message to the chat thread.
- Then route to the backend that generates the completion using the effective model identifier.

What is routed to the PM agent

- The orchestrator MUST hand off the request to the **PM agent** (`cynode-pma` in `project_manager` mode) if and only if the effective model identifier is exactly `cynodeai.pm`.
- In that case the orchestrator sends the sanitized messages to `cynode-pma`; the PM agent performs further processing (including tool use and inference) and returns the completion.
  The orchestrator does not call an inference API directly for that request.

What is not routed to the PM agent

- The orchestrator MUST route the request to **direct inference** (no PM agent) if and only if the effective model identifier is not `cynodeai.pm`.
- In that case the effective model identifier MUST be one of the inference model ids returned by `GET /v1/models` for the authenticated user.
  The orchestrator MUST call the corresponding inference backend: node-local inference (e.g. Ollama on the selected worker or sidecar) or an external API via API Egress, per [External Model Routing](external_model_routing.md).
  The orchestrator MUST NOT invoke `cynode-pma` for that request.

Summary

- **Effective model `cynodeai.pm`:** Routed to PM agent (`cynode-pma`).
  Orchestrator hands off sanitized messages to the PM agent; the agent returns the completion.
  Orchestrator does not call an inference API directly.
- **Effective model any other value:** Routed to direct inference.
  Orchestrator calls the inference backend (node-local or API Egress); the orchestrator does not invoke the PM agent.

See [Request Processing Pipeline](#request-processing-pipeline) for the ordered steps and [External Model Routing](external_model_routing.md) for inference routing policy.

## Tasks Versus Chat (Non-Goals)

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.TasksVsChat` <a id="spec-cynai-usrgwy-openaichatapi-tasksvschat"></a>

This API MUST NOT define or imply a one-to-one mapping of chat messages to tasks.
The PM/PA MAY create zero or many tasks via MCP during a conversation.
Users MAY create tasks manually through the task API or cynork task commands.

If an implementation uses internal runs or jobs to produce a completion, that is an implementation detail.
The external contract remains a chat completion request and response.

Traces To:

- [REQ-USRGWY-0130](../requirements/usrgwy.md#req-usrgwy-0130)

## Authentication, Policy, and Auditing

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.AuthPolicy` <a id="spec-cynai-usrgwy-openaichatapi-authpolicy"></a>

- Authentication MUST use the same Bearer token mechanism as the rest of the User API Gateway.
- Requests MUST be subject to policy enforcement and MUST emit audit records.

Traces To:

- [REQ-USRGWY-0121](../requirements/usrgwy.md#req-usrgwy-0121)
- [REQ-USRGWY-0124](../requirements/usrgwy.md#req-usrgwy-0124)
- [REQ-USRGWY-0125](../requirements/usrgwy.md#req-usrgwy-0125)

## Gateway Timeouts and Long-Running Behavior

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.Timeouts` <a id="spec-cynai-usrgwy-openaichatapi-timeouts"></a>

Chat can take 30-120 seconds or more when the model is cold or under load.
The gateway MUST support a `WriteTimeout` (and optionally `ReadTimeout`) long enough for chat to complete.
The gateway MUST support configuring these timeouts for deployments that use chat.

Documentation (dev_docs or operator docs) MUST describe the expected duration and required timeout tuning.

Traces To:

- [REQ-USRGWY-0128](../requirements/usrgwy.md#req-usrgwy-0128)

## Reliability Requirements

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.Reliability` <a id="spec-cynai-usrgwy-openaichatapi-reliability"></a>

The gateway handler backing `POST /v1/chat/completions` MUST implement the following reliability behavior.

- **Poll cap.**
  If the handler must wait on internal state to produce a completion, it MUST enforce a maximum total wait duration (for example 90-120 seconds).
  After the cap is reached, it MUST return a clear timeout error.
- **Retry with backoff.**
  On transient orchestrator inference failures (for example connection error, 5xx, or model loading), the handler MUST retry a small number of times (for example 2-3) with short backoff before using a fallback path.

Traces To:

- [REQ-ORCHES-0131](../requirements/orches.md#req-orches-0131)
- [REQ-ORCHES-0132](../requirements/orches.md#req-orches-0132)

## Error Semantics

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.Errors` <a id="spec-cynai-usrgwy-openaichatapi-errors"></a>

The gateway MUST return errors that allow clients and operators to distinguish at least:

- Request cancelled.
- Orchestrator inference failed.
- Completion did not finish before the maximum wait duration.

Errors MUST NOT leak secrets.

Error format for OpenAI-compatible endpoints:

- For `GET /v1/models` and `POST /v1/chat/completions`, error responses MUST use an OpenAI-style JSON payload with a top-level `error` object.
- The gateway MUST NOT return RFC 9457 Problem Details for these OpenAI-compatible endpoints.
- The payload MUST follow this shape:

```json
{
  "error": {
    "message": "Safe, user-displayable error message.",
    "type": "cynodeai_error",
    "param": null,
    "code": "cynodeai_completion_timeout"
  }
}
```

HTTP status mapping:

- Request cancelled: `408`.
- Orchestrator inference failed: `502` or `503` depending on whether the failure is upstream or overload.
- Completion timeout (poll cap reached): `504`.

Traces To:

- [REQ-USRGWY-0129](../requirements/usrgwy.md#req-usrgwy-0129)

## Observability

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.Observability` <a id="spec-cynai-usrgwy-openaichatapi-observability"></a>

The gateway MUST log which internal path was used for a completion (for example direct orchestrator inference versus fallback).
The gateway MUST log timeouts and request cancellations.

Traces To:

- [REQ-USRGWY-0129](../requirements/usrgwy.md#req-usrgwy-0129)

## Request Processing Pipeline

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.Pipeline` <a id="spec-cynai-usrgwy-openaichatapi-pipeline"></a>

This section defines the required request-processing steps for `POST /v1/chat/completions`.

### Pipeline Steps (Order is Mandatory)

1. Authenticate the caller using the standard gateway Bearer token mechanism.
2. Decode the OpenAI chat-completions request body and validate that `messages` is present and non-empty.
3. Determine `project_id`: from the OpenAI-standard `OpenAI-Project` header when present; when absent, use the creating user's default project.
4. Detect and redact secrets in the message content.
   - API keys are the priority.
   - The gateway MUST redact secrets before any persistence or inference.
   - Detected secrets MUST be replaced with the literal string `SECRET_REDACTED` in the amended message content.
   - The gateway MUST record whether redaction was applied and the kind(s) of secrets detected in `chat_audit_log`.
5. Persist the amended (redacted) user message content to the database as a chat-thread message scoped to `(user_id, project_id)`.
6. **Orchestrator routing:** Compute the effective model identifier (request `model` if present and non-empty, else `cynodeai.pm`).
   Using only the amended (redacted) messages and that identifier:
   - If the effective model identifier is exactly `cynodeai.pm`: hand off the request to the PM agent (`cynode-pma`) for processing and response generation.
     Do not call an inference API directly.
   - If the effective model identifier is not `cynodeai.pm`: route to direct inference (node-local or external API via API Egress per [External Model Routing](external_model_routing.md)).
     Do not invoke the PM agent.
7. Persist the assistant output as a chat-thread message scoped to the same `(user_id, project_id)`.
8. Return an OpenAI-format chat-completions response where the content is present at `choices[0].message.content`.

Traces To:

- [REQ-USRGWY-0121](../requirements/usrgwy.md#req-usrgwy-0121)
- [REQ-USRGWY-0127](../requirements/usrgwy.md#req-usrgwy-0127)
- [REQ-USRGWY-0130](../requirements/usrgwy.md#req-usrgwy-0130)

## Optional: Async Chat (Deferred)

- Spec ID: `CYNAI.USRGWY.OpenAIChatApi.AsyncDeferred` <a id="spec-cynai-usrgwy-openaichatapi-asyncdeferred"></a>

This spec defers an async chat mode to a later phase.
If async chat is added, it MUST use a completion-scoped or chat-scoped identifier and poll endpoint.
Async chat MUST NOT reuse task identifiers or task-result endpoints for chat.

## Related Documents

- [User API Gateway](user_api_gateway.md)
- [Chat Threads and Messages](chat_threads_and_messages.md)
- [Runs and Sessions API](runs_and_sessions_api.md)
- [CLI management app - Chat command](cli_management_app_commands_chat.md)
- [OpenWebUI and CyNodeAI Integration](../openwebui_cynodeai_integration.md)
