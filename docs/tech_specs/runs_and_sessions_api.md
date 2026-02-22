# Runs and Sessions API

- [Document Overview](#document-overview)
- [Purpose](#purpose)
- [Runs](#runs)
- [Sessions](#sessions)
- [Logs and Transcripts](#logs-and-transcripts)
- [Streaming Status](#streaming-status)
- [Background Process Management](#background-process-management)
- [Retention Policies](#retention-policies)
- [API Surface](#api-surface)

## Document Overview

This document defines a first-class runs and sessions API exposed by the User API Gateway.
It provides parity with session-based workflows: execution traces (runs), user-facing sessions, sub-runs, attached logs, streaming status, stored transcripts with retention, and background job process management within sandbox constraints.
The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
See [Runs and Sessions](postgres_schema.md#runs-and-sessions).

## Purpose

- Give users and clients a stable way to inspect execution history, attach logs, stream status, and manage interactive sessions.
- Support spawning sub-runs and sub-sessions for delegation and debugging.
- Store transcripts with configurable retention for audit and UX.
- Expose background process lifecycle within sandboxes so long-running work can be managed and attributed to runs.

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md) and [`docs/tech_specs/data_rest_api.md`](data_rest_api.md).

## Runs

A **run** is a single execution trace: one workflow instance, one dispatched job, or one agent turn.
Runs are first-class resources with stable identifiers.

### Runs Applicable Requirements

- Spec ID: `CYNAI.USRGWY.Runs` <a id="spec-cynai-usrgwy-runs"></a>

Traces To:

- [REQ-USRGWY-0100](../requirements/usrgwy.md#req-usrgwy-0100)
- [REQ-USRGWY-0101](../requirements/usrgwy.md#req-usrgwy-0101)
- [REQ-USRGWY-0102](../requirements/usrgwy.md#req-usrgwy-0102)
- [REQ-USRGWY-0103](../requirements/usrgwy.md#req-usrgwy-0103)

Recommended run fields

- `id` (uuid, pk)
- `task_id` (uuid)
- `job_id` (uuid, optional)
- `session_id` (uuid, optional)
- `parent_run_id` (uuid, optional)
- `status` (e.g. pending, running, completed, failed, cancelled)
- `started_at`, `ended_at` (timestamptz, optional)
- `metadata` (jsonb, optional)

## Sessions

A **session** is a user-facing container for interactive work that groups runs and holds transcript segments.
Raw chat history is stored separately as chat threads and chat messages.
See [`docs/tech_specs/chat_threads_and_messages.md`](chat_threads_and_messages.md).

### Sessions Applicable Requirements

- Spec ID: `CYNAI.USRGWY.Sessions` <a id="spec-cynai-usrgwy-sessions"></a>

Traces To:

- [REQ-USRGWY-0104](../requirements/usrgwy.md#req-usrgwy-0104)
- [REQ-USRGWY-0105](../requirements/usrgwy.md#req-usrgwy-0105)
- [REQ-USRGWY-0106](../requirements/usrgwy.md#req-usrgwy-0106)
- [REQ-USRGWY-0107](../requirements/usrgwy.md#req-usrgwy-0107)

Recommended session fields

- `id` (uuid, pk)
- `parent_session_id` (uuid, optional)
- `user_id` (uuid)
- `title` or `label` (text, optional)
- `created_at`, `updated_at` (timestamptz)

## Logs and Transcripts

Logs are append-only streams attached to runs.
Transcripts are stored conversation or execution summaries associated with a session or run.
Transcripts are derived artifacts and are not the canonical raw chat-message store.
Raw chat messages are stored as chat-thread messages.
See [`docs/tech_specs/chat_threads_and_messages.md`](chat_threads_and_messages.md).

### Logs and Transcripts Applicable Requirements

- Spec ID: `CYNAI.USRGWY.LogsTranscripts` <a id="spec-cynai-usrgwy-logstrans"></a>

Traces To:

- [REQ-USRGWY-0108](../requirements/usrgwy.md#req-usrgwy-0108)
- [REQ-USRGWY-0109](../requirements/usrgwy.md#req-usrgwy-0109)
- [REQ-USRGWY-0110](../requirements/usrgwy.md#req-usrgwy-0110)
- [REQ-USRGWY-0111](../requirements/usrgwy.md#req-usrgwy-0111)

## Streaming Status

Clients MUST be able to observe run and session status in near real time.

### Streaming Status Applicable Requirements

- Spec ID: `CYNAI.USRGWY.StreamingStatus` <a id="spec-cynai-usrgwy-streamstatus"></a>

Traces To:

- [REQ-USRGWY-0112](../requirements/usrgwy.md#req-usrgwy-0112)
- [REQ-USRGWY-0113](../requirements/usrgwy.md#req-usrgwy-0113)

See event types and subscriptions in [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md#live-updates-and-messaging).

## Background Process Management

Long-running work inside a sandbox (e.g. a build or server process) MUST be manageable as background processes tied to a run.

### Background Process Management Applicable Requirements

- Spec ID: `CYNAI.USRGWY.BackgroundProcessManagement` <a id="spec-cynai-usrgwy-bgprocess"></a>

Traces To:

- [REQ-USRGWY-0114](../requirements/usrgwy.md#req-usrgwy-0114)
- [REQ-USRGWY-0115](../requirements/usrgwy.md#req-usrgwy-0115)
- [REQ-USRGWY-0116](../requirements/usrgwy.md#req-usrgwy-0116)
- [REQ-USRGWY-0117](../requirements/usrgwy.md#req-usrgwy-0117)

Sandbox constraints

- Processes run inside the same sandbox container as the current job; they MUST NOT outlive the sandbox unless the spec explicitly allows detached execution.
- Resource limits (CPU, memory, PIDs) apply as defined in [`docs/tech_specs/node.md`](node.md).

## Retention Policies

Transcript and log retention MUST be configurable so operators can meet compliance and storage constraints.

### Retention Policies Applicable Requirements

- Spec ID: `CYNAI.USRGWY.RetentionPolicies` <a id="spec-cynai-usrgwy-retention"></a>

Traces To:

- [REQ-USRGWY-0118](../requirements/usrgwy.md#req-usrgwy-0118)
- [REQ-USRGWY-0119](../requirements/usrgwy.md#req-usrgwy-0119)
- [REQ-USRGWY-0120](../requirements/usrgwy.md#req-usrgwy-0120)

## API Surface

- The Runs and Sessions API is exposed through the User API Gateway and implemented as part of the Data REST API.
- Endpoints SHOULD follow the same authentication, authorization, rate limiting, and auditing rules as the Data REST API.
- Core operations:
  - Runs: create (when starting work), get, list (filter by task, session, parent run, status, time), update status.
  - Sessions: create, get, list, create sub-session, list runs for session.
  - Logs: append to run, list/stream by run and time range.
  - Transcripts: append/read for session or run; list segments with retention metadata.
  - Background processes: start, list, terminate (scoped to run/sandbox); get process output and exit status.

See [`docs/tech_specs/data_rest_api.md`](data_rest_api.md) for resource and endpoint conventions and [`docs/tech_specs/go_rest_api_standards.md`](go_rest_api_standards.md) for implementation standards.
