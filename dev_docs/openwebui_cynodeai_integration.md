# OpenWebUI and CyNodeAI Integration

## Purpose

This document describes how to integrate [Open WebUI](https://github.com/open-webui/open-webui) with CyNodeAI so that OpenWebUI can use the CyNodeAI orchestrator as an OpenAI-compatible chat backend.
It covers the intended architecture, prerequisites, and setup steps once the User API Gateway exposes an OpenAI-compatible subset.

## Scope and Current State

- CyNodeAI exposes a single user-facing surface via the **User API Gateway** (orchestrator).
- The gateway is designed to integrate with external tools such as Open WebUI; see [User API Gateway](docs/tech_specs/user_api_gateway.md).
- The gateway **MAY** expose an OpenAI-compatible subset for chat and model listing, backed by orchestrator task workflows (Client Compatibility in [user_api_gateway.md](docs/tech_specs/user_api_gateway.md)).
- That OpenAI-compatible layer is optional in the spec and may not be implemented yet.
- This doc assumes the gateway will (or does) offer at least:
  - `GET /v1/models` (or equivalent) for model listing
  - `POST /v1/chat/completions` (or equivalent) for chat, backed by orchestrator task/session workflows
- All compatibility layers MUST preserve orchestrator policy and auditing; see [REQ-USRGWY-0121](docs/requirements/usrgwy.md#req-usrgwy-0121).

## Chat Agent Backing (Implementation Requirement)

The implementation MUST allow users to interact via OpenWebUI with one of the following backings:

- **Project Manager Agent**: The default or primary option so users can create and manage tasks, get task and project info, and drive work through the orchestrator.
  See [Project Manager Agent](docs/tech_specs/project_manager_agent.md).
- **Separate configurable chat agent**: An alternative chat agent (or multiple agents) that can be selected and configured via the **Admin Web Console** (e.g. model, system prompt, capabilities), so admins can offer different chat experiences (e.g. general assistant vs. task-focused Project Manager).

The gateway MUST support at least interaction with the Project Manager Agent for tasking and info; it MAY also expose additional chat agents configurable in the admin console.

## How OpenWebUI Connects to Backends

OpenWebUI uses the **OpenAI Chat Completions** protocol.
It connects to any backend that implements an OpenAI-compatible API.

- **API URL**: Base URL of the backend, e.g. `http://localhost:8080/v1` (trailing path is typically `/v1`).
- **API Key**: Optional; when required, OpenWebUI sends it as the `Authorization: Bearer <key>` header.
- **Model listing**: OpenWebUI can call `GET .../v1/models` to discover models; if the backend does not support it, models can be added manually in OpenWebUI connection settings.

References: [Open WebUI - Starting with OpenAI-Compatible](https://docs.openwebui.com/getting-started/quick-start/starting-with-openai-compatible).

## Prerequisites

- CyNodeAI orchestrator running with the **user-gateway** service up.
- User API Gateway configured with an **OpenAI-compatibility** mode that exposes:
  - A base path for the OpenAI API (e.g. `/v1` or `/openai/v1`).
  - Chat completions and (optionally) models endpoints that route to the Project Manager Agent (for tasking and info) and/or to additional chat agents configurable in the admin console.
- A local user account and credentials (or API token) for gateway authentication; see [Local User Accounts](docs/tech_specs/local_user_accounts.md).
- OpenWebUI installed (Docker, Docker Compose, or native).

## Architecture Overview

1. **User** interacts with OpenWebUI in the browser.
2. **OpenWebUI** sends chat requests to the CyNodeAI User API Gateway using the OpenAI-compatible API (e.g. `POST /v1/chat/completions`), optionally selecting a "model" that maps to a specific agent (e.g. Project Manager Agent or a configurable chat agent).
3. **User API Gateway** authenticates the request, enforces policy, and routes to the appropriate agent (Project Manager Agent for tasking and info, or a separate chat agent configured via the admin console).
4. **Orchestrator** runs the agent (e.g. Project Manager drives tasks and sub-agents; configurable chat agent uses selected model and settings) and returns the result.
5. **Gateway** returns an OpenAI-format response to OpenWebUI so it can display the reply in the chat UI.

All requests remain within CyNodeAI policy and auditing; the compatibility layer does not bypass them.
Interaction with the Project Manager Agent MUST be supported for task creation, task status, and project info; additional chat agents MAY be configurable via the [Admin Web Console](docs/tech_specs/admin_web_console.md).

## Setup Steps

Follow these steps to connect OpenWebUI to the CyNodeAI User API Gateway once the OpenAI-compatible layer is available.

### 1. Start CyNodeAI User Gateway

Ensure the orchestrator and user-gateway are running.
See [orchestrator/README.md](orchestrator/README.md) and, if using containers, [orchestrator/docker-compose.yml](orchestrator/docker-compose.yml) or [orchestrator/systemd/README.md](orchestrator/systemd/README.md).

- Note the gateway base URL (e.g. `http://localhost:8080` or `https://cynodeai.example.com`).
- If an OpenAI-compat path is implemented under a subpath (e.g. `/openai/v1`), the full base URL for OpenWebUI will be `{gateway_base}/openai/v1`; otherwise `{gateway_base}/v1`.

### 2. Obtain Gateway Credentials

- **Option A (login)**: Use the cynork CLI or another client to log in and obtain a JWT (or session token).
  Example with cynork: `cynork auth login -u <handle> -p <password>`; the stored token can be used as the OpenWebUI API key if the gateway accepts Bearer tokens for the OpenAI-compat endpoint.
- **Option B (API key)**: If the gateway supports long-lived API keys for integrations, create one in the admin console or via API and use it as the OpenWebUI API key.

The gateway MUST authenticate user clients; see [user_api_gateway.md - Authentication and Auditing](docs/tech_specs/user_api_gateway.md#authentication-and-auditing).

### 3. Deploy and Configure OpenWebUI

- Install OpenWebUI (see [Open WebUI docs](https://docs.openwebui.com/)).
- In OpenWebUI: **Admin Settings** => **Connections** => **OpenAI** (or equivalent "Connect a provider" flow).
- **API URL**: Set to the CyNodeAI gateway OpenAI-compat base, e.g. `http://localhost:8080/v1` or `http://host.docker.internal:8080/v1` if OpenWebUI runs in Docker and the gateway is on the host.
- **API Key**: Set to the gateway token or API key from step 2.
- Save the connection.

If the gateway does not support `GET /v1/models`, add model IDs manually in the OpenWebUI connection (allowlist) so OpenWebUI knows which "models" to offer; those IDs should match what the gateway or orchestrator model registry expects.

### 4. Network and Deployment Notes

- **Same host**: Use `http://localhost:<port>/v1` (or the configured path) as the API URL.
- **OpenWebUI in Docker, gateway on host**: Use `http://host.docker.internal:<port>/v1` (or the configured path) so the container can reach the host.
- **Different hosts**: Use the reachable gateway URL (e.g. `https://gateway.example.com/v1`) and ensure TLS and firewall allow OpenWebUI to reach the gateway.
- **Reverse proxy**: If the gateway is behind a reverse proxy, the API URL should be the proxy URL that terminates TLS and forwards to the user-gateway.

## What CyNodeAI Must Provide (Implementation Notes)

For a complete integration, the User API Gateway implementation should provide:

- **OpenAI-compat base path**: A stable path (e.g. `/v1`) under which the following are served.
- **Authentication**: Accept Bearer tokens (JWT or API key) and map them to a user identity; enforce the same auth as the rest of the gateway.
- **GET /v1/models**: Return a list of "models" that include at least one entry corresponding to the **Project Manager Agent** (for tasking, task status, project info); optionally additional entries for other chat agents configurable via the admin web console, in OpenAI list-models format.
- **POST /v1/chat/completions**: Accept OpenAI-format messages and route them to the selected agent.
  When the selected "model" is the Project Manager Agent, the gateway MUST create or use a session/task and drive the request through the Project Manager Agent (task intake, dispatch, verification, info retrieval).
  When the selected "model" is a separate configurable chat agent, the gateway MUST use the agent configuration defined in the admin console (e.g. model, system prompt) and return the response in OpenAI chat-completion format.
- **Streaming (optional)**: If OpenWebUI is used with streaming, the gateway should support streaming responses in the same format as the OpenAI API.
- **Policy and auditing**: All requests through this compatibility layer MUST be subject to existing gateway policy and audit logging (REQ-USRGWY-0121).
- **Admin Web Console**: If additional chat agents are supported, their configuration (which model, system prompt, capabilities) MUST be manageable via the Admin Web Console so admins can define and expose multiple chat experiences.

These are implementation details for the gateway; the exact URL layout and response shapes should follow the tech spec once the OpenAI-compat mode is fully specified.

## References

- CyNodeAI: [README.md](README.md), [meta.md](meta.md)
- User API Gateway: [docs/tech_specs/user_api_gateway.md](docs/tech_specs/user_api_gateway.md)
- Project Manager Agent: [docs/tech_specs/project_manager_agent.md](docs/tech_specs/project_manager_agent.md)
- Admin Web Console: [docs/tech_specs/admin_web_console.md](docs/tech_specs/admin_web_console.md)
- Data REST API: [docs/tech_specs/data_rest_api.md](docs/tech_specs/data_rest_api.md)
- Local user accounts and auth: [docs/tech_specs/local_user_accounts.md](docs/tech_specs/local_user_accounts.md)
- Open WebUI: [OpenAI-Compatible setup](https://docs.openwebui.com/getting-started/quick-start/starting-with-openai-compatible)
