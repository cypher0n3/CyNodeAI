# Default Messaging Connectors: Draft Proposal

- [1. Purpose and Scope](#1-purpose-and-scope)
- [2. Messaging as First-Class Channel](#2-messaging-as-first-class-channel)
- [3. Problem Statement](#3-problem-statement)
- [4. Proposed Default Connectors](#4-proposed-default-connectors)
- [5. Per-connector requirements and setup](#5-per-connector-requirements-and-setup)
- [6. Enable and Configure Model](#6-enable-and-configure-model)
- [7. Outbound: Event Types and Delivery](#7-outbound-event-types-and-delivery)
- [8. Inbound: Messages and Routing to PMA](#8-inbound-messages-and-routing-to-pma)
- [9. Slash Commands](#9-slash-commands)
- [10. Relationship to Existing Specs](#10-relationship-to-existing-specs)
- [11. References](#11-references)

## 1. Purpose and Scope

Document type: draft specification proposal.

CyNodeAI has no specified mechanism today for pushing updates from the system to the user via messaging or an app, or for users to interact with the orchestrator and PMA from those channels.
This proposal adds a default set of **messaging connectors** (Signal, Discord, Mattermost) that provide **bidirectional** communication: system-to-user notifications and user-to-system messages (replies, slash commands, free-form) so that **messaging is a first-class method of communicating with the orchestrator and PMA**, on par with the Web Console and CLI.

Scope: define default connector types and configuration for **bidirectional messaging** (outbound notifications and inbound messages routed to PMA), including **slash commands** for settings and other actions that PMA interprets and applies with user direction.
This draft does not define the full event bus or all event types; it focuses on the connector side, a minimal set of outbound events, inbound routing and context, and slash-command semantics.

## 2. Messaging as First-Class Channel

Messaging connectors are not an add-on notification layer; they form a **first-class communication channel** between users and the orchestrator/PMA.

- **Bidirectional:** The same channel is used for the system to send updates (notifications, escalations, task status) and for the user to send messages back (replies in thread, slash commands, free-form questions or instructions).
- **Parity with other clients:** Messages delivered to PMA from a connector carry the same standing as chat or API requests: PMA can interpret them, invoke tools (including MCP), update settings with user direction, and respond in the same channel so the user sees a continuous conversation.
- **Context preservation:** Inbound messages are associated with user identity, project, task (when applicable), and thread so that PMA can maintain conversation state and apply actions in the right scope.

Thus the connector is both an **outbound** delivery target for system events and an **inbound** entry point into the orchestrator/PMA conversation model.

## 3. Problem Statement

- The API Egress sanity checker can return **escalate** for human review; the tech spec records the event but does not specify how the user is notified (see [Escalate to Human Review](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck)).
- Other system-to-user updates (e.g. task completed, agent approval needed) are not yet mapped to any delivery channel.
- Users need a way to opt in to receiving such updates via messaging or apps they already use, with minimal deployment friction.
- Users also need to **respond** and **act** from those channels (approve, clarify, change settings) without switching to the Web Console or CLI; that requires inbound routing to PMA and structured actions such as slash commands.

A system for user-facing messaging should be enableable and configurable per user or per deployment, support true back-and-forth, and not require custom code for the first supported channels.

## 4. Proposed Default Connectors

Provide three default messaging connector types that can be enabled and configured:

- **Signal**
  - Outbound: deliver notification messages to a configured Signal recipient or group.
  - Inbound (when supported): replies or DMs to the bot are ingested and routed to PMA per [Section 8](#8-inbound-messages-and-routing-to-pma); slash commands per [Section 9](#9-slash-commands) apply where text input is available.
  - Requires configuration of Signal service endpoint and credentials (e.g. Signal bot or bridge); exact auth and topology (self-hosted vs provider) to be detailed in a later spec or operator guide.

- **Discord**
  - Outbound: deliver notification messages to a configured Discord channel or DM.
  - Inbound: user replies in thread or in channel (and DMs to the bot when configured) are ingested and routed to PMA per [Section 8](#8-inbound-messages-and-routing-to-pma); slash commands per [Section 9](#9-slash-commands) are supported.
  - Reuses or extends the existing Discord connector type in the [Connector Framework](../tech_specs/connector_framework.md#spec-cynai-connec-connectormodel) (Initial Connectors); credential and config model consistent with REQ-CONNEC-0104 and connector instance model.

- **Mattermost**
  - Outbound: deliver notification messages to a configured Mattermost channel or DM.
  - Inbound: user replies in thread or in channel are ingested and routed to PMA per [Section 8](#8-inbound-messages-and-routing-to-pma); slash commands per [Section 9](#9-slash-commands) are supported.
  - Reuses or extends the existing Mattermost connector type in the [Connector Framework](../tech_specs/connector_framework.md) (Initial Connectors); credential and config model consistent with REQ-CONNEC-0104 and connector instance model.

All three connectors are **opt-in**: no messaging is sent or accepted unless the user (or admin) has enabled and configured at least one connector instance for the relevant scope (user, group, or deployment).

## 5. Per-Connector Requirements and Setup

Each connector type has specific prerequisites, APIs, and deployment options that operators and users must satisfy before a connector instance can be enabled.
The following summarizes what must be set up for each; the CyNodeAI connector implementation uses these to send outbound messages and (where applicable) receive inbound messages.

### 5.1 Signal Connector

Signal has **no official public bot or business API**.
Integrations rely on **signal-cli** (a command-line client) and a **REST API wrapper** such as **signal-cli-rest-api** (e.g. bbernhard/signal-cli-rest-api) so that CyNodeAI can send and receive messages over HTTP.

#### 5.1.1 Signal Prerequisites

- **signal-cli:** Java 17+ (or Kotlin runtime) for the signal-cli native/normal mode; typically installed from GitHub releases (e.g. signal-cli 0.11.4+).
- **REST wrapper:** A service that exposes signal-cli as a REST API (e.g. signal-cli-rest-api in a container).
  Execution modes: `normal` (invoke signal-cli per request), `native` (lower latency), or `json-rpc` (persistent daemon; higher memory).
- **Account linkage:** A Signal account must be linked to the service:
  - **Link existing phone (recommended):** Generate a QR code (e.g. from the REST API `/v1/qrcodelink`) and scan it in the Signal mobile app under Settings -> Linked Devices.
    The linked device acts as the "bot" identity.
  - **New registration:** Use `signal-cli register` with SMS or voice verification; requires a phone number (real SIM or a provider such as Twilio that supports SMS/voice for verification).
- **Storage:** Persistent volume for signal-cli data (e.g. `~/.local/share/signal-cli`); roughly hundreds of MB for the client and database.
- **Network:** Outbound access from the host running signal-cli (or the REST API container) to Signal servers; no fixed port requirement for the client, but the REST API typically exposes a port (e.g. 8080) for CyNodeAI to call.

#### 5.1.2 Signal Connector Config and Credentials

- **Config (non-secret):** Base URL of the Signal REST API (e.g. `http://signal-api:8080`), optional device name.
- **Credentials (stored encrypted):** If the REST API requires authentication, an API key or token; for link-based setups, the credential may be the linked device identity held by the REST API service (operator-managed, not stored in CyNodeAI).
- **Outbound:** REST endpoint to send messages (e.g. `POST /v2/send` with message, recipients or group id).
  Recipients are E.164 phone numbers; groups are identified by group id.
- **Inbound:** The REST API or a separate receive path (e.g. polling or webhook if supported) must deliver received messages to CyNodeAI so they can be routed to PMA.
  signal-cli-rest-api supports receive; the connector implementation must poll or subscribe and forward to the ingest pipeline.

#### 5.1.3 Signal Operator Notes

- The Signal "bot" is a linked device; one linked device per connector instance (or per deployment) is typical.
- For multi-tenant or multi-user Signal targets, operators may run multiple REST API instances with different linked accounts or configure one instance with multiple recipient/group targets per connector instance.

### 5.2 Discord Connector

Discord provides an official **Developer Portal** and **Bot API**.
Bots use a **bot token** for authentication; they **send** messages via the **REST API** and **receive** messages and events via the **Gateway** (WebSocket) with the appropriate **intents** enabled.

#### 5.2.1 Discord Prerequisites

- **Application and bot:** Create an application in the [Discord Developer Portal](https://discord.com/developers/applications); add a **Bot** to the application and obtain the **bot token**.
  Store the token securely; it is the primary credential for the connector.
- **OAuth2 (optional):** For user-installable apps or to act on behalf of users, configure OAuth2 (authorization URL, token URL, scopes such as `bot`, `applications.commands`).
  For a single bot that posts to channels and receives messages, bot token alone is sufficient.
- **Gateway (inbound):** To receive messages and events (message create, reaction, etc.), the connector must maintain a **Gateway** WebSocket connection.
  **Gateway intents** must be enabled in the Developer Portal (e.g. `MESSAGE CONTENT INTENT` if the bot needs to read message text; `GUILD_MESSAGES`, `DIRECT_MESSAGES` for message events).
  Without the correct intents, the bot will not receive message events.
- **REST API (outbound):** Send messages via `POST /channels/{channel_id}/messages` (and equivalent for DMs).
  Rate limits and response formats are per Discord API documentation.
- **Webhook events (alternative inbound):** Discord supports sending events to a **webhook URL** (public endpoint); the endpoint must validate `X-Signature-Ed25519` and `X-Signature-Timestamp` and respond to PING with 204.
  Gateway is the standard way for bots to receive real-time messages; webhooks may be used for other event types depending on Discord's offering.

#### 5.2.2 Discord Connector Config and Credentials

- **Config (non-secret):** Optional: Discord API base URL (default `https://discord.com/api`), channel id(s) or DM target for outbound; server (guild) id if needed for context.
- **Credentials (stored encrypted):** Bot token (required).
  If using OAuth2 for user-linked flows, refresh token or access token per user.
- **Outbound:** REST POST to create message in channel or DM; support for embeds and thread id for thread replies.
- **Inbound:** Gateway connection with intents for messages (and optionally threads); dispatch message events to CyNodeAI ingest and route to PMA.
  Alternatively or in addition, register Discord slash commands (e.g. `/cynode status`) so users can trigger commands that Discord sends to the app; the app then routes to PMA.

#### 5.2.3 Discord Operator Notes

- Bot must be **invited** to the server with the required permissions (Send Messages, Read Message History, Create Public Threads, etc.).
  Permission integer or scope is set at invite URL.
- For DMs, users typically initiate contact (e.g. by opening a DM with the bot); the bot can then send and receive in that DM.
- Slash commands can be registered globally or per-guild via the API; CyNodeAI may register a set of slash commands (e.g. `/task cancel`, `/status`) so they appear in Discord's UI.

### 5.3 Mattermost Connector

Mattermost is self-hosted or cloud; it supports **incoming webhooks** (post into a channel), **outgoing webhooks** (receive messages from a channel when a trigger word is used or on every post), **slash commands**, and **bot accounts** with an access token for API calls.

#### 5.3.1 Mattermost Prerequisites

- **Mattermost server:** A running Mattermost instance (self-hosted or Mattermost Cloud) with the base URL and (if required) admin access to enable integrations.
- **Integrations enabled:** A system admin must enable **Custom Integrations** (or Integration Management) in **System Console -> Integrations** so that incoming/outgoing webhooks and slash commands can be created.
- **Incoming webhook (outbound from CyNodeAI):** Create an **Incoming Webhook** in Mattermost (Main Menu -> Integrations -> Incoming Webhook).
  Select the target channel and optionally set display name and icon.
  The webhook URL (e.g. `https://{mattermost-site}/hooks/{id}`) is the credential; POST JSON with `text` (and optionally `username`, `icon_url`, attachments) to send messages to that channel.
- **Inbound options (receive messages to CyNodeAI):**
  - **Outgoing webhook:** Create an **Outgoing Webhook** for a channel; configure trigger words (or "any message") and a **Callback URL** that Mattermost will POST to when a message matches.
    The connector implements this callback, validates the request (e.g. token or secret), and forwards the message to PMA.
    Outgoing webhooks work in **public channels** only.
  - **Slash command:** Create a **Custom Slash Command** (e.g. `/cynode`) with a **Request URL** pointing to the connector.
    When a user types `/cynode status`, Mattermost POSTs to that URL with command text and context; the connector validates the token and routes to PMA.
    Slash commands work in public channels, private channels, and DMs.
- **Bot account (optional but recommended for outbound):** Create a **Bot account** and obtain a **personal access token** (or bot token).
  Use `POST /api/v4/posts` with `Authorization: Bearer <token>` to post as the bot; this allows consistent identity and threading.
  Alternatively, incoming webhooks post as a configured "BOT" label without a separate bot account.

#### 5.3.2 Mattermost Connector Config and Credentials

- **Config (non-secret):** Mattermost server base URL (e.g. `https://mattermost.example.com`), channel id for outbound (if using bot account) or incoming webhook id; for inbound, callback URL host/path that Mattermost can reach.
- **Credentials (stored encrypted):** Either (a) incoming webhook URL (for simple outbound-only) or (b) bot personal access token for outbound plus a shared secret/token for validating outgoing webhook or slash command requests.
  Slash command and outgoing webhook creation return a **token** that must be validated on each request.
- **Outbound:** POST to incoming webhook URL, or POST to `/api/v4/posts` with bot token; support for thread_id (root post id) for thread replies.
- **Inbound:** HTTP endpoint (callback) for outgoing webhook and/or slash command; validate Mattermost token; parse body (Slack-compatible format); forward message and context (channel_id, user_id, thread, command text) to CyNodeAI ingest and route to PMA.

#### 5.3.3 Mattermost Operator Notes

- Outgoing webhooks are **public-channel only**; for private channels and DMs, use **slash commands** as the inbound trigger.
- Mattermost request bodies are **Slack-compatible** for webhooks and slash commands, which simplifies reuse of parsing logic.
- The connector callback URL must be **publicly reachable** by the Mattermost server (or on the same network); for local dev, a tunnel (e.g. ngrok) may be required.

## 6. Enable and Configure Model

- Messaging connector **instances** follow the existing connector model: connector type, owner (user or group), config (non-secret), credentials stored encrypted, enable/disable, audit.
- User or admin installs a connector instance (e.g. "My Signal", "Team Discord"), supplies required config (e.g. channel id, recipient id) and credentials, and enables it.
- A **delivery policy** (to be specified) determines which outbound event types are sent to which connector instances (e.g. "escalations only", "escalations + task completion").
- Configuration SHOULD allow selecting which events are sent to which connector instance and whether inbound messages (replies, slash commands) are accepted for that instance.

This aligns with [REQ-CONNEC-0101](../requirements/connec.md#req-connec-0101) (install, enable, disable, uninstall) and the [Connector Model](../tech_specs/connector_framework.md#spec-cynai-connec-connectormodel).

## 7. Outbound: Event Types and Delivery

Minimum event type to support in the first iteration:

- **Sanity-check escalation**
  - Payload: task_id, provider, operation, reason/category, timestamp (and any non-secret context needed for the user to review).
  - Delivered as a short, readable message to the configured channel (e.g. "CyNodeAI: API call escalated for review. Task X, operation Y. Reason: ...").

Additional event types (task completed, approval requested, etc.) can be added later; the connector contract should be generic enough (e.g. event type, title, body, link to UI if available).

Delivery semantics: best-effort; audit log MUST record delivery attempt and outcome (success, failure, skipped) per connector instance and event.

## 8. Inbound: Messages and Routing to PMA

All **inbound** messages (replies in thread, replies in channel, DMs to the bot where supported) MUST be ingested and routed to the Project Manager Agent (PMA) with proper context, so that messaging is a first-class channel for conversation and commands (similar to OpenClaw).

Required context when routing an inbound message to PMA:

- Connector instance id and connector type.
- Thread or conversation id (connector-side) and message id.
- User identity (resolved from the connector context or linked account).
- Project and task context when available (e.g. task_id or project_id associated with the original notification that started the thread, or default project for the user).
- Raw message content (sanitized; no credentials).
- Indicator if the message is a slash command (see [Section 9](#9-slash-commands)) so PMA can dispatch accordingly.

PMA receives the message in the same way it receives chat or other inbound user messages: with sufficient context to associate it with the relevant task, project, and prior conversation.
PMA may respond in the same connector channel (thread or DM) so the user sees a continuous back-and-forth.
Implementation details (webhook ingestion, idempotency, replay protection) SHOULD align with the existing [Connector Framework](../tech_specs/connector_framework.md) and [connector operation hardening](../tech_specs/connector_framework.md#spec-cynai-connec-connectoroperationhardening) (e.g. webhook signature validation, replay window).
See also archived [connector_framework_hardening](integrated/connector_framework_hardening.md).

## 9. Slash Commands

Users SHOULD be able to send **slash commands** in the messaging channel to perform structured actions (settings, config, status, help) that PMA interprets and applies with user direction.

- **Purpose:** Slash commands give users a quick, consistent way to change preferences, request status, or trigger actions without leaving the messaging app; PMA executes the intent via its tools (e.g. MCP, config APIs) and responds in the same channel.
- **Format:** Commands start with a slash (e.g. `/setting`, `/config`, `/help`, `/status`, `/task cancel`, `/kill job`).
  - Example: `/notify on escalations` to enable escalation notifications for the current scope.
  - Example: `/setting pma.model local` to direct PMA to prefer local inference (PMA applies the change with user direction via the appropriate backend).
  - Example: `/status` to ask for a short status of tasks or system.
  - Example: `/task cancel <task_id>` or `/kill job <task_id>` to cancel a task and stop its running job at user direction; PMA invokes the orchestrator cancel path and the worker node stops the SBA (graceful then container kill fallback).
    See [User-Directed Job Kill](002_user_directed_job_kill_proposal.md).
- **PMA role:** PMA receives the slash command as an inbound message with context (user, project, connector, thread).
  PMA parses the command, maps it to an action (including config/settings changes), and performs the action with user direction (e.g. via MCP tools or orchestrator APIs that update user/project settings).
  PMA responds in the same channel with confirmation or result (e.g. "Notifications for escalations enabled" or "Setting updated.").
- **Extensibility:** The set of slash commands MAY be configurable or extended (e.g. org-specific commands) so that PMA can support additional commands with user direction; the default set should cover common settings, status, and task/job control (e.g. cancel, kill).
- **Discovery:** PMA or the connector SHOULD support a `/help` (or equivalent) command that lists available slash commands and brief usage for the user.

Slash commands are first-class inbound messages; they are routed to PMA like any other message but are tagged so PMA can prioritize parsing and execution of the command before or alongside natural-language handling.

## 10. Relationship to Existing Specs

- **API Egress / Sanity Check**
  - [Sanity Check (Semantic Safety)](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck) records escalate and links to this draft for delivery.
  - When this proposal is accepted, the sanity-check section should reference the canonical messaging connector spec instead of this draft.

- **Connector Framework**
  - Messaging connectors are connector types and instances under the existing [Connector Framework](../tech_specs/connector_framework.md); they use the same credential storage, policy, and auditing model.
  - Signal is a new connector type; Discord and Mattermost are already listed as initial connector types and may be extended with bidirectional messaging and slash-command semantics as described in this draft.

- **PMA (Project Manager Agent)**
  - All inbound messages (replies and slash commands) delivered to PMA follow the same auth, context, and tooling expectations as chat and other PMA inputs; see [cynode_pma.md](../tech_specs/cynode_pma.md) and orchestrator handoff rules.
  - PMA interprets slash commands and applies settings (or other actions) with user direction via its tools and the orchestrator; slash commands are part of PMA's supported input surface.

- **Requirements**
  - No new requirements are proposed in this draft; promotion would add requirements (e.g. messaging connector types, bidirectional delivery, slash commands, delivery policy, inbound routing to PMA, audit) and update tech specs accordingly.

## 11. References

- [API Egress Server - Escalate to Human Review](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck)
- [Connector Framework](../tech_specs/connector_framework.md)
- [Connector requirements (connec.md)](../requirements/connec.md)
- [Connector Framework Hardening (archived stub)](integrated/connector_framework_hardening.md)
- [User-Directed Job Kill (draft)](002_user_directed_job_kill_proposal.md)
- [CyNode PMA](../tech_specs/cynode_pma.md)
- [Draft specs README](README.md)
