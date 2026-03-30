---
name: Phase 2 MCP in the Loop
overview: |
  Implement the two remaining Phase 2 MCP-in-the-loop items.
  P2-06: Production-grade LangGraph graph-node process wired to MCP and
  Worker API for multi-step workflow orchestration (Load Task Context => Plan
  Steps => Dispatch Step => Collect Result => Verify => Finalize).
  P2-08: Full verification loop with PMA tasking Project Analyst and writing
  findings back through the workflow (multi-agent round-trip automation).
  Both build on the existing minimal_runner.py reference and persistence
  contract.
todos:
  - id: p2-001
    content: "Read `scripts/minimal_runner.py` (or equivalent stdlib reference runner) to understand the current Worker API contract validation and what it covers."
    status: pending
  - id: p2-002
    content: "Read `docs/tech_specs/` for the multi-step workflow orchestration spec: Load Task Context => Plan Steps => Dispatch Step => Collect Result => Verify => Finalize."
    status: pending
    dependencies:
      - p2-001
  - id: p2-003
    content: "Read `docs/requirements/` for workflow orchestration requirements (task lifecycle, step dispatch, result collection)."
    status: pending
    dependencies:
      - p2-002
  - id: p2-004
    content: "Read existing `agents/` code to identify where the LangGraph graph-node process would integrate with the Worker API and MCP tool interface."
    status: pending
    dependencies:
      - p2-003
  - id: p2-005
    content: "Read LangGraph documentation for Python graph-node architecture and state management patterns (current best practices)."
    status: pending
    dependencies:
      - p2-004
  - id: p2-006
    content: "Design the graph-node architecture: define nodes (LoadTaskContext, PlanSteps, DispatchStep, CollectResult, Verify, Finalize), edges, and state schema."
    status: pending
    dependencies:
      - p2-005
  - id: p2-007
    content: "Define the MCP tool interface for each graph node: which MCP tools each node calls, expected inputs/outputs, and error handling."
    status: pending
    dependencies:
      - p2-006
  - id: p2-008
    content: "Define the Worker API contract for step dispatch: how the graph-node process submits jobs, receives results, and reports status."
    status: pending
    dependencies:
      - p2-007
  - id: p2-009
    content: "Add Python unit tests: each graph node must produce expected output given mock MCP tool responses and mock Worker API responses."
    status: pending
    dependencies:
      - p2-008
  - id: p2-010
    content: "Add Python integration test: end-to-end graph execution with mock MCP server and mock Worker API must traverse all nodes in order."
    status: pending
    dependencies:
      - p2-009
  - id: p2-011
    content: "Run `python -m pytest tests/test_graph_node.py -v` and confirm failures (graph-node not yet implemented)."
    status: pending
    dependencies:
      - p2-010
  - id: p2-012
    content: "Add or extend E2E test `scripts/test_scripts/e2e_0900_workflow_graph_node.py` with tags `[suite_orchestrator, full_demo, workflow, no_inference]` and prereqs `[gateway, config, auth, node_register]`: submit a multi-step task, verify graph-node traverses all stages."
    status: pending
    dependencies:
      - p2-011
  - id: p2-013
    content: "Implement `LoadTaskContext` node: fetch task details and context from orchestrator via MCP `task.get` tool."
    status: pending
    dependencies:
      - p2-012
  - id: p2-014
    content: "Implement `PlanSteps` node: given task context, produce a step plan (list of sub-tasks or tool calls) using LLM planning."
    status: pending
    dependencies:
      - p2-013
  - id: p2-015
    content: "Implement `DispatchStep` node: submit each planned step as a Worker API job; track job IDs in graph state."
    status: pending
    dependencies:
      - p2-014
  - id: p2-016
    content: "Implement `CollectResult` node: poll or await Worker API job completion; collect results into graph state."
    status: pending
    dependencies:
      - p2-015
  - id: p2-017
    content: "Implement `Verify` node: validate collected results against task acceptance criteria; route to Finalize or back to PlanSteps for retry."
    status: pending
    dependencies:
      - p2-016
  - id: p2-018
    content: "Implement `Finalize` node: write final results back to orchestrator via MCP `task.update` tool; set task status to complete."
    status: pending
    dependencies:
      - p2-017
  - id: p2-019
    content: "Wire graph nodes into a LangGraph `StateGraph` with edges and conditional routing (Verify => Finalize or Verify => PlanSteps)."
    status: pending
    dependencies:
      - p2-018
  - id: p2-020
    content: "Re-run `python -m pytest tests/test_graph_node.py -v` and confirm green."
    status: pending
    dependencies:
      - p2-019
  - id: p2-021
    content: "Ensure the graph-node process can run as a standalone Python process invoked by the Worker API (container entry point)."
    status: pending
    dependencies:
      - p2-020
  - id: p2-022
    content: "Run `just lint-python` on all new and changed Python files."
    status: pending
    dependencies:
      - p2-021
  - id: p2-023
    content: "Run `just e2e --tags workflow,no_inference` to verify the graph-node integration with the dev stack."
    status: pending
    dependencies:
      - p2-022
  - id: p2-024
    content: "Validation gate -- do not proceed to Task 2 until all checks pass."
    status: pending
    dependencies:
      - p2-023
  - id: p2-025
    content: "Generate task completion report for Task 1. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - p2-024
  - id: p2-026
    content: "Do not start Task 2 until Task 1 closeout is done."
    status: pending
    dependencies:
      - p2-025
  - id: p2-027
    content: "Read existing PMA and PA (Project Analyst) agent code to understand the current tasking interface and result-write-back contract."
    status: pending
    dependencies:
      - p2-026
  - id: p2-028
    content: "Read `docs/tech_specs/project_manager_agent.md` and `docs/tech_specs/project_analyst_agent.md` (or equivalent) for the PMA => PA tasking and verification loop spec."
    status: pending
    dependencies:
      - p2-027
  - id: p2-029
    content: "Read `docs/requirements/` for multi-agent round-trip requirements: PMA creates sub-task for PA, PA executes and writes findings, PMA verifies findings."
    status: pending
    dependencies:
      - p2-028
  - id: p2-030
    content: "Design the verification loop: PMA creates a sub-task with verification criteria, dispatches to PA via the workflow graph-node, PA executes and writes findings back, PMA evaluates findings against criteria."
    status: pending
    dependencies:
      - p2-029
  - id: p2-031
    content: "Add Python unit tests: PMA tasking PA must create a sub-task with verification criteria; PA result must be written back to the workflow state; PMA must evaluate findings."
    status: pending
    dependencies:
      - p2-030
  - id: p2-032
    content: "Add Python integration test: end-to-end PMA => PA => PMA verification loop with mock agents must complete the full round-trip."
    status: pending
    dependencies:
      - p2-031
  - id: p2-033
    content: "Run `python -m pytest tests/test_verification_loop.py -v` and confirm failures."
    status: pending
    dependencies:
      - p2-032
  - id: p2-034
    content: "Add or extend E2E test `scripts/test_scripts/e2e_0910_verification_loop.py` with tags `[suite_orchestrator, full_demo, workflow, pma_inference]` and prereqs `[gateway, config, auth, node_register]`: PMA tasks PA, PA writes findings, PMA verifies."
    status: pending
    dependencies:
      - p2-033
  - id: p2-035
    content: "Implement PMA tasking: PMA creates sub-task for PA with verification criteria via MCP `task.create` tool with `parent_task_id` and acceptance criteria."
    status: pending
    dependencies:
      - p2-034
  - id: p2-036
    content: "Implement PA result write-back: PA writes findings to task result via MCP `task.update` tool with structured findings payload."
    status: pending
    dependencies:
      - p2-035
  - id: p2-037
    content: "Implement PMA verification: PMA reads PA findings via MCP `task.get` tool, evaluates against acceptance criteria, and marks the parent task as verified or failed."
    status: pending
    dependencies:
      - p2-036
  - id: p2-038
    content: "Wire the verification loop into the LangGraph graph: add a `VerifyWithPA` node that creates sub-task, waits for PA completion, and evaluates findings."
    status: pending
    dependencies:
      - p2-037
  - id: p2-039
    content: "Re-run `python -m pytest tests/test_verification_loop.py -v` and confirm green."
    status: pending
    dependencies:
      - p2-038
  - id: p2-040
    content: "Run `just lint-python` on all new and changed Python files."
    status: pending
    dependencies:
      - p2-039
  - id: p2-041
    content: "Run `just e2e --tags workflow,pma_inference` (requires inference; skip if unavailable) to verify the full multi-agent round-trip."
    status: pending
    dependencies:
      - p2-040
  - id: p2-042
    content: "Validation gate -- do not proceed to Task 3 until all checks pass."
    status: pending
    dependencies:
      - p2-041
  - id: p2-043
    content: "Generate task completion report for Task 2. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - p2-042
  - id: p2-044
    content: "Do not start Task 3 until Task 2 closeout is done."
    status: pending
    dependencies:
      - p2-043
  - id: p2-045
    content: "Update `docs/dev_docs/_todo.md` to mark section 6 items as complete with links to this plan."
    status: pending
    dependencies:
      - p2-044
  - id: p2-046
    content: "Verify no follow-up work was left undocumented."
    status: pending
    dependencies:
      - p2-045
  - id: p2-047
    content: "Run `just docs-check` on all changed documentation."
    status: pending
    dependencies:
      - p2-046
  - id: p2-048
    content: "Run `just e2e --tags no_inference` as final E2E regression gate."
    status: pending
    dependencies:
      - p2-047
  - id: p2-049
    content: "Generate final plan completion report: tasks completed, overall validation, remaining risks."
    status: pending
    dependencies:
      - p2-048
  - id: p2-050
    content: "Mark all completed steps in the plan with `- [x]`. (Last step.)"
    status: pending
    dependencies:
      - p2-049
---

# Phase 2 MCP in the Loop Plan

## Goal

Implement the two remaining Phase 2 MCP-in-the-loop items: a production-grade LangGraph graph-node process for multi-step workflow orchestration (P2-06) and a full PMA-to-PA verification loop for multi-agent round-trip automation (P2-08).
Both build on the existing `minimal_runner.py` reference runner and the persistence contract.

## References

- Todo: [`_todo.md`](_todo.md) section 6
- Tech specs: [`docs/tech_specs/project_manager_agent.md`](../tech_specs/project_manager_agent.md), [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md), [`docs/tech_specs/cynode_sba.md`](../tech_specs/cynode_sba.md)
- Requirements: [`docs/requirements/orches.md`](../requirements/orches.md) (task lifecycle, workflow), [`docs/requirements/pmagnt.md`](../requirements/pmagnt.md) (PMA tasking)
- Existing reference: `scripts/minimal_runner.py` (stdlib reference runner for Worker API contract)
- MCP tool catalog: [`docs/tech_specs/mcp_tools/README.md`](../tech_specs/mcp_tools/README.md)
- Implementation: `agents/`, `orchestrator/`, `worker_node/`, `scripts/`

## Constraints

- Requirements take precedence over tech specs; tech specs take precedence over current code.
- Follow BDD/TDD: failing tests before implementation.
- All changes must pass `just ci` before the task is considered complete.
- No changes that reduce test coverage below the 90% per-package threshold.
- No modifications to linter rules or suppression comments.
- Each task's validation gate must pass before starting the next task.
- Task 2 (verification loop) depends on Task 1 (graph-node process) being complete and working.
- Inference-dependent tests should be tagged and skippable when inference is unavailable.

## Execution Plan

Tasks are ordered by dependency: the graph-node process (P2-06) must be working before the verification loop (P2-08) can be wired end-to-end.

### Task 1: P2-06 -- LangGraph Graph-Node Process for Multi-Step Workflow Orchestration

Build a production-grade Python LangGraph process that implements the multi-step workflow: Load Task Context => Plan Steps => Dispatch Step => Collect Result => Verify => Finalize.
The current `minimal_runner.py` validates the API contract but does not implement production orchestration.

#### Task 1 Requirements and Specifications

- [`docs/tech_specs/`](../tech_specs/) -- multi-step workflow orchestration spec
- [`docs/requirements/orches.md`](../requirements/orches.md) -- task lifecycle, step dispatch
- MCP tool catalog: [`docs/tech_specs/mcp_tools/README.md`](../tech_specs/mcp_tools/README.md)
- Existing reference: `scripts/minimal_runner.py`

#### Discovery (Task 1) Steps

- [ ] Read `scripts/minimal_runner.py` (or equivalent stdlib reference runner) to understand the current Worker API contract validation and what it covers.
- [ ] Read `docs/tech_specs/` for the multi-step workflow orchestration spec: Load Task Context => Plan Steps => Dispatch Step => Collect Result => Verify => Finalize.
- [ ] Read `docs/requirements/` for workflow orchestration requirements (task lifecycle, step dispatch, result collection).
- [ ] Read existing `agents/` code to identify where the LangGraph graph-node process would integrate with the Worker API and MCP tool interface.
- [ ] Read LangGraph documentation for Python graph-node architecture and state management patterns (current best practices).

#### Red (Task 1)

- [ ] Design the graph-node architecture: define nodes (LoadTaskContext, PlanSteps, DispatchStep, CollectResult, Verify, Finalize), edges, and state schema.
- [ ] Define the MCP tool interface for each graph node: which MCP tools each node calls, expected inputs/outputs, and error handling.
- [ ] Define the Worker API contract for step dispatch: how the graph-node process submits jobs, receives results, and reports status.
- [ ] Add Python unit tests: each graph node must produce expected output given mock MCP tool responses and mock Worker API responses.
- [ ] Add Python integration test: end-to-end graph execution with mock MCP server and mock Worker API must traverse all nodes in order.
- [ ] Run `python -m pytest tests/test_graph_node.py -v` and confirm failures (graph-node not yet implemented).
- [ ] Add or extend E2E test `scripts/test_scripts/e2e_0900_workflow_graph_node.py` with tags `[suite_orchestrator, full_demo, workflow, no_inference]` and prereqs `[gateway, config, auth, node_register]`: submit a multi-step task, verify graph-node traverses all stages.

#### Green (Task 1)

- [ ] Implement `LoadTaskContext` node: fetch task details and context from orchestrator via MCP `task.get` tool.
- [ ] Implement `PlanSteps` node: given task context, produce a step plan (list of sub-tasks or tool calls) using LLM planning.
- [ ] Implement `DispatchStep` node: submit each planned step as a Worker API job; track job IDs in graph state.
- [ ] Implement `CollectResult` node: poll or await Worker API job completion; collect results into graph state.
- [ ] Implement `Verify` node: validate collected results against task acceptance criteria; route to Finalize or back to PlanSteps for retry.
- [ ] Implement `Finalize` node: write final results back to orchestrator via MCP `task.update` tool; set task status to complete.
- [ ] Wire graph nodes into a LangGraph `StateGraph` with edges and conditional routing (Verify => Finalize or Verify => PlanSteps).
- [ ] Re-run `python -m pytest tests/test_graph_node.py -v` and confirm green.

#### Refactor (Task 1)

- [ ] Ensure the graph-node process can run as a standalone Python process invoked by the Worker API (container entry point).

#### Testing (Task 1)

- [ ] Run `just lint-python` on all new and changed Python files.
- [ ] Run `just e2e --tags workflow,no_inference` to verify the graph-node integration with the dev stack.
- [ ] Validation gate -- do not proceed to Task 2 until all checks pass.

#### Closeout (Task 1)

- [ ] Generate task completion report for Task 1.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 2 until Task 1 closeout is done.

---

### Task 2: P2-08 -- Full Verification Loop (PMA => PA => PMA)

Wire the multi-agent round-trip: PMA creates a sub-task for the Project Analyst (PA), PA executes and writes findings back through the workflow, PMA verifies findings against acceptance criteria.
The current persistence contract exists but the round-trip is not automated end-to-end.

#### Task 2 Requirements and Specifications

- [`docs/tech_specs/project_manager_agent.md`](../tech_specs/project_manager_agent.md) -- PMA tasking PA
- [`docs/requirements/pmagnt.md`](../requirements/pmagnt.md) -- PMA multi-agent coordination
- MCP tool catalog: [`docs/tech_specs/mcp_tools/README.md`](../tech_specs/mcp_tools/README.md) -- `task.create`, `task.get`, `task.update`

#### Discovery (Task 2) Steps

- [ ] Read existing PMA and PA (Project Analyst) agent code to understand the current tasking interface and result-write-back contract.
- [ ] Read `docs/tech_specs/project_manager_agent.md` and `docs/tech_specs/project_analyst_agent.md` (or equivalent) for the PMA => PA tasking and verification loop spec.
- [ ] Read `docs/requirements/` for multi-agent round-trip requirements: PMA creates sub-task for PA, PA executes and writes findings, PMA verifies findings.

#### Red (Task 2)

- [ ] Design the verification loop: PMA creates a sub-task with verification criteria, dispatches to PA via the workflow graph-node, PA executes and writes findings back, PMA evaluates findings against criteria.
- [ ] Add Python unit tests: PMA tasking PA must create a sub-task with verification criteria; PA result must be written back to the workflow state; PMA must evaluate findings.
- [ ] Add Python integration test: end-to-end PMA => PA => PMA verification loop with mock agents must complete the full round-trip.
- [ ] Run `python -m pytest tests/test_verification_loop.py -v` and confirm failures.
- [ ] Add or extend E2E test `scripts/test_scripts/e2e_0910_verification_loop.py` with tags `[suite_orchestrator, full_demo, workflow, pma_inference]` and prereqs `[gateway, config, auth, node_register]`: PMA tasks PA, PA writes findings, PMA verifies.

#### Green (Task 2)

- [ ] Implement PMA tasking: PMA creates sub-task for PA with verification criteria via MCP `task.create` tool with `parent_task_id` and acceptance criteria.
- [ ] Implement PA result write-back: PA writes findings to task result via MCP `task.update` tool with structured findings payload.
- [ ] Implement PMA verification: PMA reads PA findings via MCP `task.get` tool, evaluates against acceptance criteria, and marks the parent task as verified or failed.
- [ ] Wire the verification loop into the LangGraph graph: add a `VerifyWithPA` node that creates sub-task, waits for PA completion, and evaluates findings.
- [ ] Re-run `python -m pytest tests/test_verification_loop.py -v` and confirm green.

#### Refactor (Task 2)

No additional refactor needed.

#### Testing (Task 2)

- [ ] Run `just lint-python` on all new and changed Python files.
- [ ] Run `just e2e --tags workflow,pma_inference` (requires inference; skip if unavailable) to verify the full multi-agent round-trip.
- [ ] Validation gate -- do not proceed to Task 3 until all checks pass.

#### Closeout (Task 2)

- [ ] Generate task completion report for Task 2.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 3 until Task 2 closeout is done.

---

### Task 3: Documentation and Closeout

- [ ] Update `docs/dev_docs/_todo.md` to mark section 6 items as complete with links to this plan.
- [ ] Verify no follow-up work was left undocumented.
- [ ] Run `just docs-check` on all changed documentation.
- [ ] Run `just e2e --tags no_inference` as final E2E regression gate.
- [ ] Generate final plan completion report: tasks completed, overall validation, remaining risks.
- [ ] Mark all completed steps in the plan with `- [x]`. (Last step.)
