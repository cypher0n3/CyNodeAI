# ORCHES Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `ORCHES` domain.
It covers orchestrator control-plane behavior, task lifecycle, dispatch, and state management.

## 2 Requirements

- **REQ-ORCHES-0001:** Control-plane: task lifecycle, dispatch, state; bootstrap and config from PostgreSQL via MCP/gateway.
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0001"></a>
- **REQ-ORCHES-0100:** The orchestrator MUST include a task scheduler that decides when and where to run work.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0100"></a>
- **REQ-ORCHES-0101:** The orchestrator MUST support a cron (or equivalent) facility for scheduled jobs, wakeups, and automation.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0101"></a>
- **REQ-ORCHES-0102:** Users and agents MUST be able to enqueue work at a future time or on a recurrence.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0102"></a>
- **REQ-ORCHES-0103:** Schedule evaluation MUST be time-zone aware.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0103"></a>
- **REQ-ORCHES-0104:** Schedules MUST support create, update, disable, and cancellation.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0104"></a>
- **REQ-ORCHES-0105:** The system MUST retain run history per schedule for visibility and debugging.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0105"></a>
- **REQ-ORCHES-0106:** The cron facility SHOULD be exposed to agents (e.g. via MCP tools).
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0106"></a>
- **REQ-ORCHES-0107:** The scheduler implementation MUST use the same node selection and job-dispatch contracts as the rest of the orchestrator.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0107"></a>
- **REQ-ORCHES-0108:** The scheduler MUST be available via the User API Gateway to manage scheduled jobs and query schedule/queue state.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0108"></a>
- **REQ-ORCHES-0109:** Orchestrator-side agents MAY use external AI providers for planning and verification when policy allows it.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0109"></a>
- **REQ-ORCHES-0110:** External provider calls MUST use API Egress and SHOULD use agent-specific routing preferences.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0110"></a>
- **REQ-ORCHES-0111:** External calls MUST use the API Egress Server so credentials are not exposed to agents or sandbox containers.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0111"></a>
- **REQ-ORCHES-0112:** The orchestrator MUST be able to configure worker nodes at registration time.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0112"></a>
- **REQ-ORCHES-0113:** The orchestrator MUST support dynamic configuration updates after registration and must ingest node capability reports on registration and node startup.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0113"></a>
- **REQ-ORCHES-0114:** The orchestrator MAY import bootstrap configuration from a YAML file at startup to seed PostgreSQL and external integrations.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0114"></a>
- **REQ-ORCHES-0115:** The orchestrator SHOULD support running as the sole service with zero worker nodes and using external AI providers when allowed.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0115"></a>

- **REQ-ORCHES-0120:** The orchestrator MUST persist tasks and their lifecycle state in PostgreSQL with stable identifiers.
  [orchestrator.md](../tech_specs/orchestrator.md)
  <a id="req-orches-0120"></a>
- **REQ-ORCHES-0121:** Authenticated user clients MUST be able to create tasks through the User API Gateway.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [data_rest_api.md](../tech_specs/data_rest_api.md)
  <a id="req-orches-0121"></a>
- **REQ-ORCHES-0122:** The orchestrator MUST dispatch work to worker nodes via the Worker API and update task/job state based on results.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [worker_api.md](../tech_specs/worker_api.md)
  <a id="req-orches-0122"></a>
- **REQ-ORCHES-0123:** The orchestrator MUST persist job results (including stdout/stderr and exit code) and make them retrievable to authorized clients.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [worker_api.md](../tech_specs/worker_api.md)
  <a id="req-orches-0123"></a>
- **REQ-ORCHES-0124:** Authorized clients MUST be able to read task state (including status) through the User API Gateway.
  [orchestrator.md](../tech_specs/orchestrator.md)
  [data_rest_api.md](../tech_specs/data_rest_api.md)
  <a id="req-orches-0124"></a>
- **REQ-ORCHES-0140:** The orchestrator MUST be able to pull node operational telemetry (logs, system info, container inventory/state) from nodes via the Worker Telemetry API.
  [CYNAI.ORCHES.NodeTelemetryPull](../tech_specs/worker_telemetry_api.md#spec-cynai-orches-nodetelemetrypull)
  <a id="req-orches-0140"></a>
- **REQ-ORCHES-0141:** The orchestrator MUST apply per-request timeouts and MUST tolerate node unavailability when pulling telemetry.
  [CYNAI.ORCHES.NodeTelemetryPull](../tech_specs/worker_telemetry_api.md#spec-cynai-orches-nodetelemetrypull)
  <a id="req-orches-0141"></a>
- **REQ-ORCHES-0142:** The orchestrator MUST treat node telemetry as non-authoritative operational data and MUST NOT make correctness-critical scheduling decisions based solely on telemetry responses.
  [CYNAI.ORCHES.NodeTelemetryPull](../tech_specs/worker_telemetry_api.md#spec-cynai-orches-nodetelemetrypull)
  <a id="req-orches-0142"></a>
