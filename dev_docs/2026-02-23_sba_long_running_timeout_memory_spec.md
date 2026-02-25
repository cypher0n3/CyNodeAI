# SBA Long-Running Jobs: Wait Semantics, Timeout Extension, Time Remaining, and Temporary Memory

- [Summary](#summary)
- [Wait Semantics and Preference for SBA-Outbound](#wait-semantics-and-preference-for-sba-outbound)
- [Timeout Extension](#timeout-extension)
- [Time Remaining and LLM Context](#time-remaining-and-llm-context)
- [Temporary Memory (Job-Scoped)](#temporary-memory-job-scoped)
- [Spec and Gap Doc Updates Applied](#spec-and-gap-doc-updates-applied)
- [References](#references)

## Summary

**Date:** 2026-02-23  
**Mode:** Specs-only (design and spec updates; no code changes).  
**Scope:** Clarify how the node "waits" for SBA jobs; prefer SBA-outbound for long-running work; timeout extension; time-remaining tracking; job-scoped temporary memory for SBA.

This doc records design decisions and spec updates for:

1. **Wait semantics:** Making it explicit that the sync "node-mediated" path uses a blocking wait (e.g. one goroutine/thread waiting on container exit), which is not suitable for long-running jobs.
   The preferred design for long-running work is **SBA-outbound** (SBA contacts worker/orchestrator to report status and completion) so the node does not hold a connection or thread open for the full job.
2. **Timeout extension:** SBA MUST be able to request a time extension (up to the node maximum); orchestrator or node MAY grant or deny.
3. **Time remaining:** SBA MUST be able to track remaining time and inject it into LLM prompts (e.g. "you have X time left to complete this task").
4. **Temporary memory:** SBA MUST have a method to store and retrieve temporary memories during job processing (e.g. MCP memory.add / memory.list / memory.retrieve), scoped to the job/task, for working state across steps and LLM calls.

## Wait Semantics and Preference for SBA-Outbound

**Clarification:** In the **synchronous** Run Job implementation, the node "monitoring the container for exit" means the node **blocks** (e.g. a single goroutine or thread) waiting for the container process to exit or for the job timeout to elapse.
Typically via a wait on the container runtime (e.g. `Wait()` on the container handle).
There is no periodic poll required for "did the container exit?"; the runtime blocks until exit or timeout.
That same HTTP request from the orchestrator to the node remains open for the full duration of the job.

**Implication:** Keeping a thread/connection open for a long-running job (e.g. 1-3 hours) is undesirable.
Therefore the **preferred** design for long-running jobs is that the **SBA contacts the worker/orchestrator** to report in-progress and completion (outbound via worker proxy to job callback URL or status API).
The node then does **not** need to block for the full job: it can accept the job (202 Accepted or similar), and when the SBA reports completion via callback, the node (or orchestrator) persists the result.
The sync "node-mediated" path (blocking wait, then read `/job/result.json` after exit) remains valid for **short** jobs where holding the connection open is acceptable.

Spec wording will state explicitly: (a) sync path = blocking wait on container exit; (b) for long-running jobs, implementations SHOULD use an async pattern where SBA reports completion via outbound call so the node does not hold a thread or connection open.

## Timeout Extension

- SBA MUST be able to **request a time extension** for the current job (e.g. via the same job-status/callback surface or a dedicated extension endpoint), up to the **node maximum** (`node_max_job_timeout_seconds`).
- The orchestrator or node MAY grant or deny the request (e.g. based on policy or current load).
- When granted, the job's effective timeout is extended and the SBA can continue; the node (or orchestrator) MUST enforce the new deadline.
- The mechanism (e.g. MCP tool `job.request_timeout_extension` or a field on the job-status callback) is to be defined in the Worker API and/or MCP tool catalog; the spec commits to the capability.

## Time Remaining and LLM Context

- The job context supplied to the SBA (or an endpoint/tool the SBA can call) MUST provide **remaining time** (or an absolute deadline) so the SBA can track it internally.
- The SBA MUST be able to inject "you have X time left to complete this task" (or equivalent) into LLM prompts or step context so the agent can pace work and avoid running out of time without warning.
- The job spec or runtime-injected context SHOULD include `deadline` or `remaining_seconds` (or both), updated when an extension is granted.

## Temporary Memory (Job-Scoped)

- SBA MUST have a method to **store and retrieve temporary memories** during job processing, scoped to the task/job (e.g. MCP tools `memory.add`, `memory.list`, `memory.retrieve`, and optionally `memory.delete`), so it can persist working state across steps and LLM calls.
- These memories are **job-scoped** (or task-scoped) and MUST NOT persist beyond the job (or task) unless explicitly promoted to artifacts or long-term storage.
- Size limits and retention (e.g. max entries, max size per entry, TTL = job lifetime) MUST be defined in the MCP tool catalog and enforced by the gateway.
- The Worker (sandbox) agent allowlist MUST include these memory tools for sandbox-scoped use.

## Spec and Gap Doc Updates Applied

- **tmp/sba_result_artifact_delivery_gap.md:** Clarified wait = blocking wait; preferred long-running = SBA-outbound; added timeout extension, time remaining, temporary memory.
- **docs/tech_specs/worker_api.md:** Clarified sync = blocking wait; for long-running jobs async/SBA-outbound preferred.
- **docs/tech_specs/cynode_sba.md:** Added Timeout extension, Time remaining, and Temporary memory (job-scoped) sections; referenced MCP memory tools.
- **docs/tech_specs/mcp_tool_catalog.md:** Added Memory tools (memory.add, memory.list, memory.retrieve, memory.delete) for job/task scope.
- **docs/tech_specs/mcp_gateway_enforcement.md:** Added memory.*
  to Worker agent allowlist.

## References

- `docs/tech_specs/cynode_sba.md` - Job lifecycle, Result and Artifact Delivery
- `docs/tech_specs/worker_api.md` - Job Lifecycle and Result Persistence
- `docs/tech_specs/mcp_tool_catalog.md` - Tool catalog
- `docs/tech_specs/mcp_gateway_enforcement.md` - Worker allowlist
- `tmp/sba_result_artifact_delivery_gap.md` - Result/artifact delivery gap
