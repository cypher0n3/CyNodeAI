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

## Purpose

- Give users and clients a stable way to inspect execution history, attach logs, stream status, and manage interactive sessions.
- Support spawning sub-runs and sub-sessions for delegation and debugging.
- Store transcripts with configurable retention for audit and UX.
- Expose background process lifecycle within sandboxes so long-running work can be managed and attributed to runs.

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md) and [`docs/tech_specs/data_rest_api.md`](data_rest_api.md).

## Runs

A **run** is a single execution trace: one workflow instance, one dispatched job, or one agent turn.
Runs are first-class resources with stable identifiers.

Normative requirements

- The orchestrator MUST assign a unique run identifier to each run and persist it in PostgreSQL.
- A run MUST be associated with a task (and optionally a job) for auditing and lineage.
- A run MAY have a parent run identifier to support sub-runs (e.g. a step or sub-agent spawn).
- The Data REST API MUST expose runs as a core resource: create, read, list, and filter by task, job, session, parent run, and time range.

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

A **session** is a user-facing container for interactive work (e.g. a chat thread or task thread) that groups runs and holds a transcript.

Normative requirements

- The orchestrator MUST support creating and listing sessions.
- A session MAY have a parent session (sub-session) for delegation or nested context.
- Runs MAY be associated with a session via `session_id`.
- The User API Gateway MUST allow creating a session, spawning sub-sessions, listing runs for a session, and attaching new work to a session.

Recommended session fields

- `id` (uuid, pk)
- `parent_session_id` (uuid, optional)
- `user_id` (uuid)
- `title` or `label` (text, optional)
- `created_at`, `updated_at` (timestamptz)

## Logs and Transcripts

Logs are append-only streams attached to runs.
Transcripts are stored conversation or execution summaries associated with a session or run.

Normative requirements

- The system MUST support attaching logs to a run (e.g. stdout, stderr, or structured events).
- Logs MUST be stored in a way that supports retrieval by run and time range and MUST be subject to retention policies.
- Transcripts (e.g. chat history, agent turn summaries) MUST be storable per session or run with a configurable retention policy.
- The Data REST API MUST expose endpoints to append and read logs for a run and to read/write transcript segments for a session or run.

## Streaming Status

Clients MUST be able to observe run and session status in near real time.

Normative requirements

- Run status changes (e.g. pending -> running -> completed) MUST be observable via the Data REST API (polling) and SHOULD be emitted as events for the User API Gateway live updates (webhooks, subscriptions).
- The gateway SHOULD support streaming run status and log tail for a run when the client requests it (e.g. Server-Sent Events or WebSocket).

See event types and subscriptions in [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md#live-updates-and-messaging).

## Background Process Management

Long-running work inside a sandbox (e.g. a build or server process) MUST be manageable as background processes tied to a run.

Normative requirements

- The orchestrator MUST support starting, listing, and terminating background processes within a sandbox, subject to sandbox constraints (no direct host access).
- Background process operations MUST be associated with a run and task for auditing.
- Process lifecycle (start, stdout/stderr capture, exit status) MUST be exposed so clients can attach output to runs and show status in the runs/sessions API.
- Background process management MAY be exposed via MCP sandbox tools (e.g. `sandbox.start_background`, `sandbox.list_processes`, `sandbox.terminate_process`) and MUST be reflected in the runs API when processes are tied to a run.

Sandbox constraints

- Processes run inside the same sandbox container as the current job; they MUST NOT outlive the sandbox unless the spec explicitly allows detached execution.
- Resource limits (CPU, memory, PIDs) apply as defined in [`docs/tech_specs/node.md`](node.md).

## Retention Policies

Transcript and log retention MUST be configurable so operators can meet compliance and storage constraints.

Normative requirements

- The system MUST support configurable retention policies for run logs and session transcripts (e.g. retain for N days or until storage limit).
- Retention policy SHOULD be defined at the orchestrator or project level and applied consistently.
- Expired data MUST be purged or archived in a way that does not break referential integrity for audit records that reference run/session identifiers (e.g. retain id and metadata, drop bulk content).

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
