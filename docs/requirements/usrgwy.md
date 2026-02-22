# USRGWY Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `USRGWY` domain.
It covers user-facing REST API gateway behavior and related API contracts.

## 2 Requirements

- **REQ-USRGWY-0001:** User API Gateway as single user-facing API surface; authn, authz, audit.
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  <a id="req-usrgwy-0001"></a>
- **REQ-USRGWY-0002:** Runs and sessions with stable ids; runs per task/job; sessions and sub-sessions; logs and transcripts; retention policies.
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  [CYNAI.USRGWY.Sessions](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-sessions)
  <a id="req-usrgwy-0002"></a>
- **REQ-USRGWY-0100:** The orchestrator MUST assign a unique run identifier to each run and persist it in PostgreSQL.
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  <a id="req-usrgwy-0100"></a>
- **REQ-USRGWY-0101:** A run MUST be associated with a task (and optionally a job) for auditing and lineage.
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  <a id="req-usrgwy-0101"></a>
- **REQ-USRGWY-0102:** A run MAY have a parent run identifier to support sub-runs (e.g. a step or sub-agent spawn).
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  <a id="req-usrgwy-0102"></a>
- **REQ-USRGWY-0103:** The Data REST API MUST expose runs as a core resource: create, read, list, and filter by task, job, session, parent run, and time range.
  [CYNAI.USRGWY.Runs](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-runs)
  <a id="req-usrgwy-0103"></a>
- **REQ-USRGWY-0104:** The orchestrator MUST support creating and listing sessions.
  [CYNAI.USRGWY.Sessions](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-sessions)
  <a id="req-usrgwy-0104"></a>
- **REQ-USRGWY-0105:** A session MAY have a parent session (sub-session) for delegation or nested context.
  [CYNAI.USRGWY.Sessions](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-sessions)
  <a id="req-usrgwy-0105"></a>
- **REQ-USRGWY-0106:** Runs MAY be associated with a session via `session_id`.
  [CYNAI.USRGWY.Sessions](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-sessions)
  <a id="req-usrgwy-0106"></a>
- **REQ-USRGWY-0107:** The User API Gateway MUST allow creating a session, spawning sub-sessions, listing runs for a session, and attaching new work to a session.
  [CYNAI.USRGWY.Sessions](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-sessions)
  <a id="req-usrgwy-0107"></a>
- **REQ-USRGWY-0108:** The system MUST support attaching logs to a run (e.g. stdout, stderr, or structured events).
  [CYNAI.USRGWY.LogsTranscripts](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-logstrans)
  <a id="req-usrgwy-0108"></a>
- **REQ-USRGWY-0109:** Logs MUST be stored in a way that supports retrieval by run and time range and MUST be subject to retention policies.
  [CYNAI.USRGWY.LogsTranscripts](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-logstrans)
  <a id="req-usrgwy-0109"></a>
- **REQ-USRGWY-0110:** Transcripts (e.g. chat history, agent turn summaries) MUST be storable per session or run with a configurable retention policy.
  [CYNAI.USRGWY.LogsTranscripts](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-logstrans)
  <a id="req-usrgwy-0110"></a>
- **REQ-USRGWY-0111:** The Data REST API MUST expose endpoints to append and read logs for a run and to read/write transcript segments for a session or run.
  [CYNAI.USRGWY.LogsTranscripts](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-logstrans)
  <a id="req-usrgwy-0111"></a>
- **REQ-USRGWY-0112:** Run status changes MUST be observable via the Data REST API (polling) and SHOULD be emitted as events for User API Gateway live updates.
  [CYNAI.USRGWY.StreamingStatus](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-streamstatus)
  <a id="req-usrgwy-0112"></a>
- **REQ-USRGWY-0113:** The gateway SHOULD support streaming run status and log tail for a run when the client requests it.
  [CYNAI.USRGWY.StreamingStatus](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-streamstatus)
  <a id="req-usrgwy-0113"></a>
- **REQ-USRGWY-0114:** The orchestrator MUST support starting, listing, and terminating background processes within a sandbox, subject to sandbox constraints.
  [CYNAI.USRGWY.BackgroundProcessManagement](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-bgprocess)
  <a id="req-usrgwy-0114"></a>
- **REQ-USRGWY-0115:** Background process operations MUST be associated with a run and task for auditing.
  [CYNAI.USRGWY.BackgroundProcessManagement](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-bgprocess)
  <a id="req-usrgwy-0115"></a>
- **REQ-USRGWY-0116:** Process lifecycle MUST be exposed so clients can attach output to runs and show status in the runs/sessions API.
  [CYNAI.USRGWY.BackgroundProcessManagement](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-bgprocess)
  <a id="req-usrgwy-0116"></a>
- **REQ-USRGWY-0117:** Background process management MAY be exposed via MCP sandbox tools and MUST be reflected in the runs API when processes are tied to a run.
  [CYNAI.USRGWY.BackgroundProcessManagement](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-bgprocess)
  <a id="req-usrgwy-0117"></a>
- **REQ-USRGWY-0118:** The system MUST support configurable retention policies for run logs and session transcripts.
  [CYNAI.USRGWY.RetentionPolicies](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-retention)
  <a id="req-usrgwy-0118"></a>
- **REQ-USRGWY-0119:** Retention policy SHOULD be defined at the orchestrator or project level and applied consistently.
  [CYNAI.USRGWY.RetentionPolicies](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-retention)
  <a id="req-usrgwy-0119"></a>
- **REQ-USRGWY-0120:** Expired data MUST be purged or archived without breaking referential integrity for audit records referencing run/session identifiers.
  [CYNAI.USRGWY.RetentionPolicies](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-retention)
  <a id="req-usrgwy-0120"></a>

- **REQ-USRGWY-0121:** Compatibility layers MUST preserve orchestrator policy constraints and MUST NOT bypass auditing.
  [CYNAI.USRGWY.ClientCompatibility](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-clientcompatibility)
  <a id="req-usrgwy-0121"></a>
- **REQ-USRGWY-0122:** The User API Gateway MUST provide a Data REST API for user clients and integrations.
  [CYNAI.USRGWY.DataRestApi](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-datarestapi)
  <a id="req-usrgwy-0122"></a>
- **REQ-USRGWY-0123:** Secrets required for delivery MUST be stored securely in PostgreSQL and MUST NOT be exposed to agents.
  [CYNAI.USRGWY.MessagingAndEvents](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-messagingevents)
  <a id="req-usrgwy-0123"></a>
- **REQ-USRGWY-0124:** The gateway MUST authenticate user clients.
  [CYNAI.USRGWY.AuthAuditing](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-authauditing)
  <a id="req-usrgwy-0124"></a>
- **REQ-USRGWY-0125:** The gateway MUST authorize user actions using policy and (when applicable) user task-execution preferences and constraints.
  [CYNAI.USRGWY.AuthAuditing](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-authauditing)
  <a id="req-usrgwy-0125"></a>
- **REQ-USRGWY-0126:** The web console MUST be a client of the gateway and MUST NOT connect directly to PostgreSQL.
  [CYNAI.USRGWY.AdminConsole](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-adminconsole)
  <a id="req-usrgwy-0126"></a>
- **REQ-USRGWY-0127:** The User API Gateway MUST expose the OpenAI-compatible chat API as the only interactive chat interface.
  This includes `GET /v1/models` and `POST /v1/chat/completions`.
  [CYNAI.USRGWY.OpenAIChatApi](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi)
  [CYNAI.USRGWY.ClientCompatibility](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-clientcompatibility)
  <a id="req-usrgwy-0127"></a>
- **REQ-USRGWY-0128:** The gateway MUST support configurable timeouts suitable for OpenAI-compatible chat completions.
  Deployments that use chat MUST be able to configure timeouts so chat can complete when models are cold or under load.
  [CYNAI.USRGWY.OpenAIChatApi.Timeouts](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-timeouts)
  <a id="req-usrgwy-0128"></a>
- **REQ-USRGWY-0129:** The gateway MUST provide distinct chat-completion error semantics and MUST log completion path selection for diagnostics.
  Errors MUST distinguish cancellation, inference failure, and completion timeout.
  [CYNAI.USRGWY.OpenAIChatApi.Errors](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-errors)
  [CYNAI.USRGWY.OpenAIChatApi.Observability](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-observability)
  <a id="req-usrgwy-0129"></a>
- **REQ-USRGWY-0130:** The system MUST store chat history as chat threads and chat messages that are tracked separately from task lifecycle state.
  Chat threads and messages MUST be retrievable to authorized clients.
  Transcript segments remain a separate derived artifact associated with a session or run.
  [CYNAI.USRGWY.ChatThreadsMessages](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages)
  [CYNAI.USRGWY.OpenAIChatApi.ConversationModel](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-conversationmodel)
  <a id="req-usrgwy-0130"></a>
- **REQ-USRGWY-0131:** Tasks and chat threads MUST support optional association to a project via `project_id`.
  There is no default project association; `project_id` is null unless explicitly set by the user client or by the PM/PA via MCP.
  [CYNAI.ACCESS.Doc.ProjectsAndScopes](../tech_specs/projects_and_scopes.md#spec-cynai-access-doc-projectsandscopes)
  [CYNAI.USRGWY.ChatThreadsMessages.Threads](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-threads)
  <a id="req-usrgwy-0131"></a>
