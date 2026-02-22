# Scheduled (Cron-Like) Jobs and PM Agent Routing - Review

- [Summary](#summary)
- [What the Specs Say Today](#what-the-specs-say-today)
  - [Scheduler and Cron (Requirements + Orchestrator)](#scheduler-and-cron-requirements--orchestrator)
  - [PM Agent Handoff (`cynode_pma.md`)](#pm-agent-handoff-cynode_pmamd)
  - [User API Gateway](#user-api-gateway)
- [Gap: Explicit Routing Path for Scheduled Runs](#gap-explicit-routing-path-for-scheduled-runs)
- [Recommendations](#recommendations)
  - [Orchestrator Spec (`orchestrator.md`, Task Scheduler)](#orchestrator-spec-orchestratormd-task-scheduler)
  - [Cynode PMA Spec (`cynode_pma.md`)](#cynode-pma-spec-cynode_pmamd)
  - [Requirements (`orches.md`)](#requirements-orchesmd)
  - [User API Gateway / Scheduler Surface (Future)](#user-api-gateway--scheduler-surface-future)
- [Applied (2026-02-22)](#applied-2026-02-22)
- [References](#references)

## Summary

**Date:** 2026-02-22.
**Scope:** Tech specs and requirements for scheduled/cron jobs; clarity of routing to the PM agent for processing and tasking.

Scheduler and cron requirements are well covered in `docs/requirements/orches.md` and `docs/tech_specs/orchestrator.md`.
Routing of **scheduled job interpretation** to the PM agent is mentioned in `cynode_pma.md` but the **end-to-end flow** (cron fires -> what gets enqueued -> who consumes it -> handoff to PMA) is not spelled out in one place.
Making that flow explicit would remove ambiguity for implementers.

---

## What the Specs Say Today

This section summarizes scheduler, PM handoff, and gateway wording.

### Scheduler and Cron (Requirements + Orchestrator)

- **REQ-ORCHES-0100 / 0101:** Orchestrator has a task scheduler; MUST support a cron (or equivalent) facility for scheduled jobs, wakeups, and automation.
- **REQ-ORCHES-0102:** Users and agents MUST be able to enqueue work at a future time or on a recurrence.
- **REQ-ORCHES-0107:** Scheduler MUST use the same node selection and job-dispatch contracts as the rest of the orchestrator.
- **REQ-ORCHES-0108:** Scheduler MUST be available via the User API Gateway (create/manage scheduled jobs, query queue/schedule state).

From `orchestrator.md` (Task Scheduler):

- Scheduler fires at the scheduled time and **enqueues the corresponding tasks or jobs**.
- **Agents (e.g. Project Manager) and the cron facility enqueue work; the scheduler is responsible for dequeueing and dispatching to nodes.**

So: cron/agents enqueue, and the scheduler dequeues and dispatches.
The spec does not distinguish "enqueue a PMA work item" vs "enqueue a direct job spec."

### PM Agent Handoff (`cynode_pma.md`)

- **Request source:** All agent-responsibility work comes from the orchestrator; PMA is never invoked directly by the gateway or clients.
- **Handoff categories:** Chat completion, planning and task refinement, job dispatch to worker nodes, analyst sub-agents, and:
  - **Other inference-needed flows:** "Any other flow where the orchestrator needs agent reasoning, tool use, or inference (e.g. **scheduled job interpretation**, run continuation, preference-driven decisions) MUST be routed to PMA rather than to a bare inference endpoint."

So "scheduled job interpretation" is explicitly a PMA handoff case, but the **mechanism** (how a fired schedule turns into a PMA handoff) is not specified.

### User API Gateway

- Scheduler surface: create/list/update/disable/delete scheduled jobs (cron or one-off), cancel schedule or next run, run history, time-zone awareness, queue/schedule state, wakeups and automation triggers.

No definition of **schedule payload shape** (e.g. free-form task description vs concrete job spec) or **routing rule** (when the result of a fired schedule goes to PMA vs direct dispatch).

---

## Gap: Explicit Routing Path for Scheduled Runs

The following is **implied** but not **stated** in one place:

1. When a schedule **fires**, the orchestrator has a **run payload** (created at schedule-creation time: e.g. task description, prompt, or concrete job spec).
2. If that payload **requires agent reasoning, task decomposition, or interpretation** (e.g. "daily standup reminder", "triage backlog", or a natural-language task), the orchestrator MUST **hand the work to PMA** (same handoff contract as chat/planning), not enqueue a raw job for direct node dispatch.
3. PMA then plans, creates/updates tasks, and may use MCP (e.g. sandbox tools, scheduler MCP tools) to carry out work; the **scheduler** still uses the same node selection and job-dispatch contracts when PMA (or the workflow engine) enqueues concrete jobs.

Missing pieces today:

- **No explicit "scheduled run -> PMA" rule** in `orchestrator.md` or in orches requirements (e.g. "When a scheduled run's payload requires interpretation or reasoning, the orchestrator MUST hand it to the Project Manager Agent").
- **No definition of schedule payload types** (e.g. "task/prompt" vs "pre-specified job") that would determine routing.
- **No single place** that describes the full flow: cron fires -> orchestrator loads schedule + payload -> if interpretation needed -> hand off to PMA with context (user, project, thread, etc.) -> PMA interprets, tasks, dispatches via MCP; scheduler still dispatches concrete jobs from the queue.

---

## Recommendations

Suggested spec and requirement updates so scheduled-run-to-PMA routing is explicit.

### Orchestrator Spec (`orchestrator.md`, Task Scheduler)

Add a short subsection that states:

- When a scheduled run's payload requires **agent reasoning, task interpretation, or planning**, the orchestrator MUST **hand that work off to the Project Manager Agent** (per the same handoff rules as in `cynode_pma.md`), supplying the usual context (user, project, thread, schedule id, run id).
- When the payload is a **pre-specified job** (e.g. concrete script/command/sandbox spec with no interpretation needed), the scheduler MAY enqueue it for direct dispatch using the same node selection and job-dispatch contracts.
- Optionally: reference that schedule payload types (e.g. "task/prompt" vs "job spec") may be defined in the User API Gateway or a dedicated scheduler spec.

### Cynode PMA Spec (`cynode_pma.md`)

Under "What is Handed off to cynode-pma", keep "scheduled job interpretation" but add a cross-reference to the Task Scheduler subsection in `orchestrator.md` that defines when a fired schedule is handed to PMA (so the routing rule lives in one place and is referenced from both sides).

### Requirements (`orches.md`)

Consider adding a single requirement, e.g.: "When a scheduled run requires agent reasoning or task interpretation, the orchestrator MUST route that work to the Project Manager Agent for processing and tasking."
Trace it to the new orchestrator subsection and to `cynode_pma.md`.

### User API Gateway / Scheduler Surface (Future)

When the API for creating scheduled jobs is specified, define whether a schedule has a payload type (e.g. `task` / `prompt` vs `job_spec`) so that "requires interpretation" is deterministic and implementable.

---

## Applied (2026-02-22)

Spec and requirement updates were applied per the recommendations above:

- **orchestrator.md**: New subsection [Scheduled Run Routing to Project Manager Agent](../docs/tech_specs/orchestrator.md#scheduled-run-routing-to-project-manager-agent) (Spec ID `CYNAI.ORCHES.ScheduledRunRouting`) with routing rules, payload semantics, and trace to REQ-ORCHES-0109 and cynode_pma.
- **cynode_pma.md**: Cross-reference from "Other inference-needed flows" to the orchestrator subsection.
- **orches.md**: New **REQ-ORCHES-0109** (scheduled run requires interpretation -> route to PMA); existing REQ-ORCHES-0109 through 0142 renumbered to 0110 through 0143; REQ-ORCHES-0132 (created_by) renumbered to 0133.
  All cross-references in docs updated.
- **user_api_gateway.md**: Scheduler and cron bullet added: when the create-scheduled-jobs API is specified, payload type (`task`/`prompt` vs `job_spec`) MUST be defined for deterministic routing; link to orchestrator Scheduled Run Routing.

## References

- `docs/requirements/orches.md` - REQ-ORCHES-0100 through 0109 (scheduler, cron, gateway, scheduled-run-to-PMA), then 0110+.
- `docs/tech_specs/orchestrator.md` - Task Scheduler, Scheduled Run Routing to Project Manager Agent, Project Manager Agent.
- `docs/tech_specs/cynode_pma.md` - Request Source and Orchestrator Handoff; "scheduled job interpretation" under "Other inference-needed flows".
- `docs/tech_specs/user_api_gateway.md` - Scheduler and cron capabilities.
