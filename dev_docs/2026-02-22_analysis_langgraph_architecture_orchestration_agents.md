# LangGraph in CyNodeAI: Architecture, Orchestration, and Agents - Analysis

- [Summary](#summary)
- [Current Role of LangGraph](#current-role-of-langgraph)
- [Architecture Placement](#architecture-placement)
- [Orchestration Boundaries](#orchestration-boundaries)
- [Agents and the Graph](#agents-and-the-graph)
- [Decisions (2026-02-22)](#decisions-2026-02-22)
- [Recommendations](#recommendations)
- [References](#references)

## Summary

**Date:** 2026-02-22.
**Scope:** How LangGraph fits into the overall CyNodeAI architecture, orchestration, and the various agents; analysis, decisions, and improvement recommendations.

LangGraph is specified as the workflow engine that implements the Project Manager Agent's execution model.
It is not yet integrated in the Phase 1 runtime (tasks run as a single sandbox job); Phase 2 will integrate the LangGraph MVP workflow with persisted checkpoints and resumability.
This report re-reviews that fit; open questions were resolved on 2026-02-22 and decisions are recorded below.

## Current Role of LangGraph

- The **workflow engine** of the orchestrator is LangGraph.
  See [Workflow Engine](../docs/tech_specs/orchestrator.md#workflow-engine) in [orchestrator.md](../docs/tech_specs/orchestrator.md).
- The **Project Manager Agent's behavior** is implemented by the LangGraph MVP workflow: the graph is the execution model for the agent, not a separate service.
  Planning, dispatch, verification, and finalization are graph nodes.
  See [Integration With the Orchestrator - Runtime and Hosting](../docs/tech_specs/langgraph_mvp.md#runtime-and-hosting) in [langgraph_mvp.md](../docs/tech_specs/langgraph_mvp.md).
- **Phase 1:** LangGraph is not in the runtime loop; tasks are executed as a single dispatched sandbox job.
  See [Phase 1 Single Node Happy Path](../docs/mvp.md#phase-1-single-node-happy-path-mvp-phase-1) in [mvp.md](../docs/mvp.md).
- **Phase 2:** The LangGraph MVP workflow is integrated as the orchestrator workflow engine for the Project Manager Agent, with MCP in the loop and checkpointing.
  See [Phase 2 MCP in the Loop](../docs/mvp.md#phase-2-mcp-in-the-loop) in [mvp.md](../docs/mvp.md).

## Architecture Placement

- **Hosting:** The workflow engine MAY be implemented in a different language or process than the orchestrator's REST APIs (e.g. a Python LangGraph runtime invoked by the Go orchestrator).
  If separate, the orchestrator MUST provide a stable contract for starting workflows, passing `task_id`, and reading/writing checkpoints for resume after restarts.
  See [Runtime and Hosting](../docs/tech_specs/langgraph_mvp.md#runtime-and-hosting).
- **Scope:** One workflow instance per task (`task_id`); only one active workflow per task at a time; duplicate starts MUST be prevented or coalesced (e.g. lease or idempotency key).
  See [Invocation Model](../docs/tech_specs/langgraph_mvp.md#invocation-model).
- **Persistence:** Checkpoints MUST be backed by PostgreSQL (or an orchestrator-owned store with PostgreSQL as source of truth); save/load by `task_id`; no workflow steps without going through the checkpoint layer.
  See [Checkpoint Persistence Contract](../docs/tech_specs/langgraph_mvp.md#checkpoint-persistence-contract) and REQ-AGENTS-0116, REQ-AGENTS-0117, REQ-AGENTS-0118 in [agents.md](../docs/requirements/agents.md).
- **Node-to-orchestrator mapping:** Each graph node uses orchestrator capabilities only: MCP DB tools, model routing, Worker API, job/result APIs.
  The workflow MUST NOT connect directly to PostgreSQL; all DB access via MCP database tools.
  See [Graph Nodes to Orchestrator Capabilities](../docs/tech_specs/langgraph_mvp.md#graph-nodes-to-orchestrator-capabilities).

## Orchestration Boundaries

- **Task scheduler vs workflow engine:** The scheduler enqueues work and dispatches jobs using the same node-selection and job-dispatch contracts.
  When a task is "ready to be driven," the orchestrator starts a workflow (one per task).
  The spec does not fully spell out whether the scheduler "enqueues a task" that a workflow runner consumes, or whether the workflow engine pulls from the same queue; the contract is that the workflow is started with `task_id` and loads context in the first node.
  See [Invocation Model](../docs/tech_specs/langgraph_mvp.md#invocation-model) and [Task Scheduler](../docs/tech_specs/orchestrator.md#task-scheduler).
- **Job dispatch:** In Phase 1, job dispatch is direct HTTP to the Worker API; MCP is not in the loop.
  In Phase 2, the **Dispatch Step** node uses Worker API (or MCP node/sandbox tools) to select node and dispatch; **Collect Result** uses Worker API or job result API.
  So the graph drives dispatch and collection; the orchestrator's node registry and worker API remain the single place for node selection and dispatch.
  See [Job dispatch](../docs/tech_specs/orchestrator.md#job-dispatch-initial-implementation) and [langgraph_mvp.md - Dispatch Step / Collect Result](../docs/tech_specs/langgraph_mvp.md#graph-nodes-to-orchestrator-capabilities).

## Agents and the Graph

- **Project Manager Agent (PMA):** Implemented by the LangGraph graph; `cynode-pma` in project_manager mode is the concrete runtime.
  The graph nodes (Load Task Context, Plan Steps, Dispatch Step, Collect Result, Verify Step Result, Finalize Summary, Mark Failed) are the PM's execution steps.
  See [project_manager_agent.md](../docs/tech_specs/project_manager_agent.md) and [cynode_pma.md](../docs/tech_specs/cynode_pma.md).
- **Project Analyst Agent (PA):** Sub-agent used for focused verification.
  Per decisions below: the Orchestrator kicks off to PMA; PMA (within the **Verify Step Result** node) tasks the Project Analyst or another sandbox agent to verify; findings are recorded back into the main workflow state so Verify Step Result can decide pass/fail.
  See [Sub-Agent Invocation](../docs/tech_specs/langgraph_mvp.md#sub-agent-invocation) and [project_analyst_agent.md](../docs/tech_specs/project_analyst_agent.md).
- **Chat vs task-driven workflow:** User-facing chat is a conversation surface; PM and PA create and manage tasks via MCP during that conversation.
  The LangGraph workflow is task-scoped (one instance per `task_id`).
  How "create task from chat" triggers workflow start and how chat turns relate to running workflow instances is described at a high level (orchestrator hands off to PMA) but the exact handoff from User API / chat to "start workflow for task_id" could be made more explicit in one place.

## Decisions (2026-02-22)

The following decisions were made to resolve open questions; tech specs should be updated to reflect them.

1. **Runtime and language:** Separate Python LangGraph process invoked by the Go orchestrator (e.g. HTTP/RPC).
   The workflow engine runs as its own service; the orchestrator provides the contract for start/resume and checkpoint read/write.

1. **Checkpoint schema and format:** Prescriptive and explicit in tech specs.
   The spec MUST define the concrete checkpoint payload and storage (e.g. table name, columns or JSONB shape) so there is no room for interpretation.
   Tech specs must be prescriptive, specific, and explicit.

1. **Sub-agent (Project Analyst) invocation:** Orchestrator kicks off to PMA; PMA tasks the Project Analyst (or other sandbox agent) to verify.
   The Verify Step Result node is implemented by PMA delegating verification to the Project Analyst or another sandbox agent (not the orchestrator calling an internal API directly).
   Findings are recorded back into the workflow state so Verify Step Result can decide pass/fail.

1. **Single-active-workflow-per-task enforcement:** Orchestrator DB holds the lease (e.g. `task_workflow_lease` or equivalent).
   The orchestrator is the source of truth; the workflow runner acquires or checks the lease via the orchestrator before running.

1. **Scheduler and workflow start:** Scheduler hands the run payload directly to PMA.
   PMA creates the task and starts the workflow internally; there is no separate "enqueue workflow start" step.
   The scheduler does not create the task or enqueue a workflow start; PMA owns task creation and workflow start for scheduled runs that require interpretation.

1. **cynode-pma (chat) vs LangGraph (task execution) process boundaries:** Hybrid (separate processes, shared MCP/DB).
   cynode-pma and the workflow runner are separate processes; they share the MCP gateway and DB.
   The orchestrator starts the workflow runner for a given task; chat requests go to PMA for planning and task creation.
   The workflow runner executes the graph; it does not serve chat.

## Recommendations

1. **Document Phase 2 runtime in tech specs:** Add a "Phase 2 implementation" or "Runtime and hosting" subsection in [langgraph_mvp.md](../docs/tech_specs/langgraph_mvp.md) and/or [orchestrator.md](../docs/tech_specs/orchestrator.md) stating that the workflow engine is a **separate Python LangGraph process** invoked by the Go orchestrator, with a stable start/resume and checkpoint contract.
   (Decision 1.)

2. **Define concrete checkpoint schema in tech specs:** In [langgraph_mvp.md](../docs/tech_specs/langgraph_mvp.md), add a prescriptive subsection under Checkpointing: table name (or store identifier), column names or JSONB schema, and the exact payload fields (task_id, state model fields, last node, timestamp, etc.) so implementations are unambiguous.
   (Decision 2.)

3. **Specify PMA tasks PA/sandbox agent for verification:** In [langgraph_mvp.md](../docs/tech_specs/langgraph_mvp.md) Sub-Agent Invocation and [project_analyst_agent.md](../docs/tech_specs/project_analyst_agent.md), state explicitly: Orchestrator kicks off to PMA; in the Verify Step Result node, PMA tasks the Project Analyst or another sandbox agent to perform verification; findings are written back into the workflow state.
   (Decision 3.)

4. **Specify lease in orchestrator DB:** In [orchestrator.md](../docs/tech_specs/orchestrator.md) or [postgres_schema.md](../docs/tech_specs/postgres_schema.md), define where the single-active-workflow-per-task lease is held (e.g. table and columns or row semantics) and how the workflow runner acquires/checks it.
   (Decision 4.)

5. **Specify scheduler handoff to PMA:** In [orchestrator.md](../docs/tech_specs/orchestrator.md) (and scheduler/User API specs), state explicitly: when a scheduled run that requires interpretation fires, the scheduler hands the run payload **directly to PMA**; PMA creates the task and starts the workflow internally (no separate enqueue-workflow-start step).
   (Decision 5.)

6. **Document process boundaries (PMA vs workflow runner):** In [orchestrator.md](../docs/tech_specs/orchestrator.md) or [cynode_pma.md](../docs/tech_specs/cynode_pma.md), state that cynode-pma (chat, MCP) and the workflow runner (LangGraph) are **separate processes** sharing MCP gateway and DB; the orchestrator starts the workflow runner for a task; chat goes to PMA for planning and task creation.
   (Decision 6.)

7. **Single diagram for task => workflow => scheduler:** Add or reference one end-to-end flow diagram: task created/updated => orchestrator starts workflow (lease in DB) => workflow runner runs graph => Dispatch Step => job dispatch => Collect Result => Verify (PMA tasks PA) => Finalize/Mark Failed; and scheduled run => scheduler hands payload to PMA => PMA creates task and starts workflow.
   Ties together Task Scheduler, PMA handoff, and LangGraph invocation.

8. **Phase 2 integration checklist:** In mvp_plan or dev_docs, list concrete steps: (a) checkpoint table/schema per spec, (b) workflow start/resume API (Go to Python), (c) graph nodes wired to MCP DB tools and Worker API, (d) lease acquisition in orchestrator DB, (e) Verify Step Result implemented as PMA tasking PA/sandbox agent.

## References

- [langgraph_mvp.md](../docs/tech_specs/langgraph_mvp.md) - LangGraph MVP workflow, topology, state, nodes, integration.
- [orchestrator.md](../docs/tech_specs/orchestrator.md) - Orchestrator responsibilities, workflow engine, task scheduler, PMA.
- [project_manager_agent.md](../docs/tech_specs/project_manager_agent.md) - PM responsibilities, tools, sub-agents.
- [project_analyst_agent.md](../docs/tech_specs/project_analyst_agent.md) - PA role and handoff.
- [agents.md](../docs/requirements/agents.md) - REQ-AGENTS-0004, 0116, 0117, 0118 (LangGraph checkpointing).
- [mvp.md](../docs/mvp.md) - Phased MVP plan; Phase 1 (no LangGraph in loop), Phase 2 (LangGraph integrated).
- [_main.md](../docs/tech_specs/_main.md) - Tech spec index.
