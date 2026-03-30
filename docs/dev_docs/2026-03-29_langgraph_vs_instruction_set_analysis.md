# LangGraph vs PMA Instruction-Set Workflow Analysis

- [Scope and Context](#scope-and-context)
- [Current State of LangGraph Integration](#current-state-of-langgraph-integration)
  - [What the Spec Calls For](#what-the-spec-calls-for)
  - [What Actually Exists](#what-actually-exists)
  - [What is Already Built in Go](#what-is-already-built-in-go)
- [Option A: Complete the Python LangGraph Integration (P2-06)](#option-a-complete-the-python-langgraph-integration-p2-06)
  - [Option A Pros](#option-a-pros)
  - [Option A Cons](#option-a-cons)
- [Option B: Go-Native Workflow With PMA Instruction Set](#option-b-go-native-workflow-with-pma-instruction-set)
  - [How It Would Work](#how-it-would-work)
  - [Option B Pros](#option-b-pros)
  - [Option B Cons](#option-b-cons)
- [Option C: Go LangGraph Library (`langgraphgo`)](#option-c-go-langgraph-library-langgraphgo)
  - [Available Go Implementations](#available-go-implementations)
  - [Option C Pros](#option-c-pros)
  - [Option C Cons](#option-c-cons)
- [Comparative Analysis](#comparative-analysis)
  - [Complexity Budget](#complexity-budget)
  - [The Graph is Simple](#the-graph-is-simple)
  - [The Go Infrastructure is Already 90% There](#the-go-infrastructure-is-already-90-there)
  - [What a Graph Framework Actually Adds vs What It Costs](#what-a-graph-framework-actually-adds-vs-what-it-costs)
- [Risk Assessment](#risk-assessment)
  - [LangGraph Risks](#langgraph-risks)
  - [Instruction-Set Risks](#instruction-set-risks)
- [Recommendation](#recommendation)
- [If Option B is Chosen: Impact on Existing Specs and Code](#if-option-b-is-chosen-impact-on-existing-specs-and-code)
  - [Specs That Would Need Updates](#specs-that-would-need-updates)
  - [Code That Would Not Change](#code-that-would-not-change)
  - [Code That Would Change](#code-that-would-change)
  - [Requirements That Remain Satisfied](#requirements-that-remain-satisfied)
- [References](#references)

## Scope and Context

This analysis evaluates whether CyNodeAI should complete the planned Python LangGraph integration (P2-06) or replace it with a Go-native workflow driven by PMA instruction sets (analogous to how PAA uses `--role=project_analyst` with a separate instruction bundle).

The analysis was prompted by the fact that LangGraph is not yet integrated -- only a spec and a stdlib reference runner exist -- while the Go-side infrastructure (workflow API, checkpoints, leases, gates, langchaingo agent loop) is already substantial.

## Current State of LangGraph Integration

This section maps the specced LangGraph architecture against what is actually implemented.

### What the Spec Calls For

Per [`langgraph_mvp.md`](../tech_specs/workflow_mvp.md) and [`orchestrator.md`](../tech_specs/orchestrator.md) Workflow Engine:

- A **separate Python LangGraph process** invoked by the Go orchestrator over HTTP.
- The Python process owns the graph topology (Load Task Context => Plan Steps => Dispatch Step => Collect Result => Verify Step Result => Finalize/Mark Failed).
- Checkpoints persisted to PostgreSQL after each node transition.
- One workflow instance per `task_id`, enforced by a DB lease.
- The Python process calls back into Go (MCP gateway, Worker API) for all data access and job dispatch.
- `cynode-pma` (Go/langchaingo) handles chat, planning, and MCP tool calls; the Python LangGraph runner handles the graph execution and does not serve chat.

### What Actually Exists

Per the [2026-03-29 implementation state report](2026-03-29_implementation_state_report.md) and [`_todo.md`](_todo.md):

- **P2-06 is explicitly listed as "not completed."**
- No Python LangGraph library import exists anywhere in the repository.
- No Python container image, no Python dependency file, no Python CI path.
- A stdlib-only reference runner ([`minimal_runner.py`](../../scripts/workflow_runner_stub/minimal_runner.py)) validates the HTTP contract without importing LangGraph.

### What is Already Built in Go

The Go orchestrator and agents already implement most of the workflow primitives:

- **Workflow HTTP API:** `POST /v1/workflow/start`, `/resume`, `/checkpoint`, `/release` -- fully implemented and tested.
- **DB checkpoint persistence:** `workflow_checkpoints` table, `UpsertWorkflowCheckpoint`, `GetWorkflowCheckpoint` -- implemented.
- **Task workflow leases:** `task_workflow_leases` table, `AcquireTaskWorkflowLease`, `ReleaseTaskWorkflowLease` -- implemented with single-active-workflow-per-task guarantee.
- **Workflow start gate:** `EvaluateWorkflowStartGate` checks `planning_state`, plan state, plan archived, task dependencies -- implemented.
- **PMA langchaingo agent loop:** `OpenAIFunctionsAgent` with up to 80 iterations, MCP tool execution, streaming NDJSON, thinking separation -- implemented.
- **MCP tool catalog:** task, job, preference, project, artifact, skills, node, system_setting tools -- implemented and allowlisted per agent role.
- **PMA instruction bundles:** baseline + tools instructions for `project_manager` and `project_analyst` roles -- implemented.
- **BDD and E2E:** workflow start/resume/lease feature files and E2E tests -- passing.

## Option A: Complete the Python LangGraph Integration (P2-06)

Build and deploy the separate Python LangGraph process as originally specced in P2-06.

### Option A Pros

- **Purpose-built graph abstraction.**
  LangGraph provides `StateGraph`, conditional edges, and parallel node execution as first-class primitives.
  Workflow topology is declarative code rather than imperative control flow.
- **Built-in Postgres checkpointing.**
  `langgraph-checkpoint-postgres` handles serialization and checkpoint storage natively.
- **LangSmith/LangGraph Studio.**
  Visual debugging, trace inspection, and graph visualization are available through the LangChain ecosystem.
- **Community and ecosystem.**
  LangGraph has a growing community and documented patterns for multi-agent workflows.
- **Explicit graph topology.**
  The workflow shape (nodes, edges, conditions) is visible in code rather than implicit in instructions.
  This aids auditing and onboarding.
- **Separation of control flow from execution.**
  The graph runner owns "what step comes next"; langchaingo nodes own "how to execute this step."
  Clean boundary.

### Option A Cons

- **Introduces a second runtime language.**
  The entire project is Go.
  Adding Python means: new runtime dependency, new package manager (pip/poetry/uv), new container image, new CI pipeline, new dependency scanning, new security surface.
- **Process management overhead.**
  The Go orchestrator must spawn, monitor, health-check, and restart the Python process.
  Failure modes multiply (Python OOM, segfault, dependency conflict, startup race).
- **IPC latency on every node transition.**
  Every graph node transition requires an HTTP round-trip between Go and Python.
  The Python process then calls *back* into Go (MCP gateway) for all data access and job dispatch.
  This creates a Go => Python => Go call chain per node.
- **Duplicate checkpoint logic.**
  The Go orchestrator already has checkpoint persistence, lease management, and the workflow start gate.
  LangGraph would bring its own checkpoint layer.
  Either we use both (duplication) or we rip out the Go layer (waste).
- **The graph is small.**
  The MVP graph has 7 nodes and one retry loop.
  This is a linear pipeline with bounded retries -- not a complex DAG, not parallel branching, not dynamic subgraph composition.
  LangGraph's power is disproportionate to the problem.
- **Operational burden.**
  Two languages means doubled expertise requirements, two sets of linting rules, two test frameworks, two dependency update cycles, two CVE triage paths.
- **LangGraph version churn.**
  LangGraph is still evolving rapidly (breaking API changes between minor versions).
  Pinning a version creates maintenance debt; tracking HEAD creates instability.
- **Vendor ecosystem coupling.**
  Dependency on LangChain/LangGraph ecosystem for a core control-flow component.
  If the project direction shifts (e.g., LangGraph is superseded or abandoned), the workflow engine must be rewritten.
- **The heavy lifting is already in Go.**
  Langchaingo already executes LLM calls, handles tool use, MCP interactions, and streaming.
  The Python LangGraph process would be a thin orchestration shim calling back into Go for everything meaningful.

## Option B: Go-Native Workflow With PMA Instruction Set

Replace the Python LangGraph process with a Go-native workflow runner using PMA instruction sets for per-node behavior.

### How It Would Work

Instead of a separate Python process, the workflow graph logic would be implemented as a Go-native workflow runner within the orchestrator, with PMA driving execution through enhanced instruction sets:

- **Go workflow runner** replaces the Python LangGraph process.
  A simple Go state machine implements the same graph topology (Load Context => Plan => Dispatch => Collect => Verify => Finalize).
  It uses the already-implemented checkpoint, lease, and gate infrastructure.
- **PMA instruction bundle** gains workflow-specific instructions (a "workflow executor" instruction set or skill) that encode the per-node behaviors: how to plan steps, what MCP tools to call for dispatch, how to evaluate verification results, when to retry vs fail.
- **The existing langchaingo agent loop** executes within each "node" -- the Go runner calls PMA (or directly uses langchaingo) for each step that requires LLM reasoning.
- **Checkpoint between nodes** uses the existing `workflow_checkpoints` table and `UpsertWorkflowCheckpoint`.
- **PAA verification** is invoked by the Go runner (or PMA via MCP) the same way it would be under LangGraph -- the invocation contract does not change.

This is analogous to how PAA already works: same binary (`cynode-pma`), different `--role`, different instruction bundle, different MCP allowlist.
The workflow runner would be a Go component with a well-defined instruction set, not an LLM-driven free-form agent.

### Option B Pros

- **Pure Go stack.**
  No new language, runtime, container, CI path, or dependency surface.
  Entire team operates in one language.
- **Zero IPC overhead.**
  Workflow node transitions are function calls, not HTTP round-trips.
  No Go => Python => Go call chain.
- **Reuse existing infrastructure.**
  Checkpoint persistence, leases, workflow start gate, and MCP tools are already implemented and tested.
  No duplication or replacement needed.
- **Lower operational complexity.**
  One binary, one deployment unit, one monitoring path.
  No Python process health-checking, restart logic, or container image to maintain.
- **Simpler testing.**
  Pure Go unit and integration tests.
  No cross-process test orchestration.
  BDD and E2E tests do not need to wait for a Python process to start.
- **Instruction sets are already proven.**
  The PMA and PAA instruction bundle pattern works.
  Adding a workflow-executor skill or instruction set follows the same pattern.
- **The graph is small enough for imperative code.**
  A 7-node linear pipeline with one retry loop is well within the complexity threshold for a Go `switch`/state-machine.
  No need for a graph framework.
- **Faster iteration.**
  Changes to workflow behavior are Go code changes or instruction-set updates -- no Python rebuild/redeploy cycle.
- **No vendor coupling.**
  No dependency on LangChain/LangGraph ecosystem for core workflow control flow.

### Option B Cons

- **Less structured graph abstraction.**
  Workflow topology is imperative Go code (state machine, switch, or similar) rather than a declarative graph definition.
  Adding new graph patterns (dynamic subgraphs, parallel branches) requires more manual work.
- **No visual debugging tools.**
  No LangSmith/LangGraph Studio equivalent for tracing graph execution.
  Debugging workflow issues relies on structured logging, checkpoint inspection, and BDD tests.
- **Instruction-set complexity.**
  The PMA instruction bundle becomes more complex as it encodes workflow execution behaviors.
  The boundary between "what the LLM decides" and "what the runner hard-codes" needs careful design to avoid prompt injection affecting control flow.
- **Graph evolution.**
  If the workflow topology grows significantly (many parallel branches, dynamic node insertion, complex error recovery), a Go state machine becomes harder to maintain than a graph framework.
  However, this is speculative for the MVP scope.
- **Spec rewrite.**
  [`langgraph_mvp.md`](../tech_specs/workflow_mvp.md), portions of [`orchestrator.md`](../tech_specs/orchestrator.md), [`project_manager_agent.md`](../tech_specs/project_manager_agent.md), [`cynode_pma.md`](../tech_specs/cynode_pma.md), and several requirements entries reference "Python LangGraph process" and would need updates.

## Option C: Go LangGraph Library (`langgraphgo`)

A Go implementation of LangGraph exists that would eliminate the Python runtime objection while preserving the graph abstraction.

### Available Go Implementations

There are two Go implementations of LangGraph:

- **`tmc/langgraphgo`** (original, by the langchaingo author Travis Cline):
  257 stars, 37 forks, **1 contributor**.
  Last published version: `v0.0.0-20240324` (March 2024).
  Never reached `v0.1.0`.
  No activity for two years.
  **Effectively abandoned.**

- **`smallnest/langgraphgo`** (active fork, via `paulnegz/langgraphgo`):
  210 stars, 34 forks.
  Latest release `v0.8.5` (January 2026).
  Actively maintained with releases through Jan 2026.
  Claims feature parity with Python LangGraph.
  Has Postgres/Redis/SQLite checkpointing, parallel execution, subgraphs, pre-built agents (ReAct, Supervisor, Planning), MCP support, streaming, generics.
  Uses `langchaingo v0.1.14`.
  Dedicated documentation site (`lango.rpcx.io`).
  Two showcase apps listed (both by the maintainer).

### Option C Pros

- **Stays in Go.**
  No Python runtime, container image, CI path, or process management.
  Eliminates the biggest objection to Option A.
- **Declarative graph abstraction.**
  `StateGraph` with `AddNode`/`AddEdge`, conditional edges, and compiled runnables.
  Same developer experience as Python LangGraph but in Go.
- **Compatible with langchaingo.**
  CyNodeAI already uses langchaingo; `smallnest/langgraphgo` is built on it.
  The PMA agent loop and MCP tool wrappers could integrate with graph nodes.
- **Postgres checkpointing included.**
  The library provides a Postgres checkpointer out of the box (via `pgx`).
- **Pre-built agent patterns.**
  ReAct, Supervisor, Planning agents are available as starting points.
- **Graph visualization.**
  Mermaid, DOT, and ASCII export built in.
- **Human-in-the-loop.**
  Interrupt/resume primitives for approval workflows.

### Option C Cons

- **Bus factor: single maintainer.**
  The original `tmc/langgraphgo` was abandoned by its sole contributor.
  `smallnest/langgraphgo` is maintained by one person.
  If that maintainer moves on, CyNodeAI inherits an unmaintained core dependency.
  This is a **critical risk** for a workflow engine -- the most important control-flow component in the system.
- **Pre-1.0 API.**
  At `v0.8.5`, the API is not yet stable.
  Breaking changes between minor versions are possible and have occurred (e.g., v0.8.0 introduced "complete generic type support" -- a fundamental API change).
- **No evidence of production adoption.**
  The two showcase apps are by the maintainer.
  No third-party production usage reports, case studies, or community testimonials were found.
  This library has not been battle-tested at scale.
- **Heavy dependency tree.**
  Direct dependencies include `pgx`, `go-redis`, `go-sqlite3`, `go-openai`, `volcengine-go-sdk`, and many more.
  CyNodeAI uses GORM (not `pgx` directly) for database access -- the Postgres checkpointer's `pgx` dependency creates a second database driver in the stack.
- **Checkpoint schema mismatch.**
  The library's Postgres checkpointer has its own table schema.
  CyNodeAI already has a bespoke `workflow_checkpoints` table with a defined schema.
  Either: (a) adopt the library's schema and migrate (disrupting existing tests), (b) use the library's schema alongside the existing one (duplication), or (c) write a custom checkpointer adapter (additional work that negates the "built-in" benefit).
- **langchaingo version alignment.**
  CyNodeAI currently pins a specific langchaingo version.
  `smallnest/langgraphgo` depends on `v0.1.14`.
  Version bumps in a transitive LLM-framework dependency can introduce subtle behavioral changes in tool calling, streaming, and model interaction.
- **Overkill for a 7-node pipeline.**
  The same "graph is simple" argument from Option A applies.
  The library provides subgraphs, parallel execution, fan-out/fan-in, pre-built agents, and dozens of other features that are unused for a linear pipeline with one retry loop.
  The ratio of framework surface area to actual usage is very high.
- **Framework coupling for core control flow.**
  The workflow engine is the most critical control-flow component.
  Coupling it to an external library (especially one with a single maintainer and no production adoption evidence) creates a dependency risk that is hard to unwind later.
  A bespoke 200-line state machine has zero external dependencies and is fully understood by the team.

## Comparative Analysis

This section compares the three options across several dimensions.

### Complexity Budget

The project is in early prototype/design phase with a small team.
Introducing a second language runtime is one of the highest-complexity decisions possible -- it impacts CI, deployment, monitoring, debugging, onboarding, and security posture permanently.
The question is whether LangGraph provides enough value to justify that cost.

### The Graph is Simple

The MVP graph topology:

```text
Load Task Context => Plan Steps => [loop: Dispatch => Collect => Verify => retry?] => Finalize/Fail
```

This is a sequential pipeline with bounded retries on the inner loop.
It has no parallel branches, no dynamic subgraph composition, no fan-out/fan-in, and no complex conditional routing.
A Go state machine can express this in under 200 lines of clear, testable code.

### The Go Infrastructure is Already 90% There

The checkpoint table, lease table, workflow HTTP API, start gate, and langchaingo agent loop are all implemented and tested.
The missing piece is the graph runner itself -- the component that moves from node to node.
Under Option A, this is a Python LangGraph process.
Under Option B, this is a Go function that walks a state machine and calls existing Go functions at each node.

### What a Graph Framework Actually Adds vs What It Costs

What a graph framework (Python or Go) adds beyond what the project already provides:

- Declarative graph definition (`StateGraph` with `add_node`/`add_edge`).
- Built-in checkpoint serialization (but Go already has this).
- Visual debugging via LangSmith (Python only) or Mermaid export (Go library).
- Community patterns for complex agent graphs (relevant later, not now).

What Python LangGraph (Option A) costs:

- Second runtime language (Python 3.x).
- New container image build and registry.
- New CI pipeline (Python lint, test, security scan).
- Process management from Go (spawn, monitor, restart, health-check).
- IPC latency on every node transition.
- Doubled dependency surface and CVE triage.
- Operational complexity for deployment and monitoring.
- Team context-switching between Go and Python.

What Go LangGraph (Option C) costs:

- Single-maintainer pre-1.0 dependency for core control flow.
- No production adoption evidence.
- Checkpoint schema mismatch with existing tables.
- Heavy transitive dependency tree (`pgx`, `go-redis`, `go-sqlite3`, etc.).
- langchaingo version alignment risk.
- Framework surface area vastly exceeding actual usage.

## Risk Assessment

Each option carries distinct risks that should be weighed against the project's current maturity and team size.

### LangGraph Risks

- **Integration risk:** LangGraph's Postgres checkpointer may conflict with or duplicate the existing Go checkpoint layer.
  Schema alignment between LangGraph's expected tables and the existing `workflow_checkpoints` table requires investigation.
- **Version stability risk:** LangGraph API is not yet stable; major releases have included breaking changes.
- **Process lifecycle risk:** Python process crashes, memory leaks, or startup failures become orchestrator failure modes.
- **Performance risk:** The Go => Python => Go call chain adds latency to every node transition.
  For verification loops (PMA => PAA round-trips), this compounds.
- **Security surface risk:** Python runtime + pip dependencies expand the attack surface.

### Instruction-Set Risks

- **Control-flow rigidity risk:** If the workflow topology needs to become significantly more complex (dynamic branching, parallel execution of graph nodes), a Go state machine requires more refactoring than a graph framework would.
  *Mitigation:* The MVP graph is simple; complexity can be addressed if and when it arises.
- **Prompt injection risk:** If workflow control flow is encoded in LLM instructions rather than hard-coded, adversarial input could influence which "node" executes next.
  *Mitigation:* The Go runner hard-codes the graph topology; PMA instructions only govern behavior *within* a node, not transitions *between* nodes.
- **Spec drift risk:** Existing specs heavily reference "Python LangGraph process."
  These need updating to avoid confusion.
  *Mitigation:* One-time spec update; the behavioral contracts (graph nodes, checkpoint schema, lease semantics) remain the same.

## Recommendation

**Option B (Go-native workflow with PMA instruction set) is the stronger choice for this project at this stage.**

The primary reasons:

- The MVP graph is simple enough that a Go state machine is a better fit than a full graph framework.
- The Go infrastructure (checkpoints, leases, gates, langchaingo) is already 90% complete.
- Introducing Python (Option A) adds permanent operational complexity disproportionate to the value LangGraph provides for this graph topology.
- The Go LangGraph library (Option C) eliminates the Python objection but introduces a single-maintainer pre-1.0 dependency with no production adoption evidence for the most critical control-flow component in the system.
  The bus factor and checkpoint schema mismatch risks outweigh the graph abstraction benefits for a 7-node pipeline.
- The instruction-set pattern is already proven in the project (PMA/PAA roles).
- If the workflow topology grows complex enough to warrant a graph framework in the future, `smallnest/langgraphgo` (or its successor) can be re-evaluated at that time -- when it may have reached 1.0, gained production adoption, and proven its stability.
  Migrating from a bespoke state machine to a graph library is straightforward when the checkpoint schema and node behavior contracts are well-defined (which they already are).

**Option C is worth watching** but not adopting today.
The library is promising but too immature and too risky for a core control-flow dependency.
The right time to re-evaluate is when (a) the graph topology outgrows a simple state machine, or (b) `smallnest/langgraphgo` reaches 1.0+ with multiple independent production users.

The decision is not "LangGraph (Python or Go) is bad" but rather "the cost and risk of either LangGraph variant exceeds the value of a graph abstraction for a 7-node linear pipeline when 90% of the infrastructure is already built."

## If Option B is Chosen: Impact on Existing Specs and Code

This section scopes the change if LangGraph is dropped in favor of a Go-native runner.

### Specs That Would Need Updates

- [`langgraph_mvp.md`](../tech_specs/workflow_mvp.md): Rename or restructure to "Workflow MVP" or similar; remove Python-specific runtime references; retain the graph topology, node behaviors, checkpoint schema, and lease contract (all of which are runtime-agnostic).
- [`orchestrator.md`](../tech_specs/orchestrator.md) Workflow Engine section: Replace "separate Python LangGraph process" with "Go-native workflow runner."
- [`cynode_pma.md`](../tech_specs/cynode_pma.md) Process Boundaries: Update the PMA/workflow-runner boundary description.
- [`project_manager_agent.md`](../tech_specs/project_manager_agent.md) LLM and Tool Execution: Update "LangGraph remains the graph runner" to reflect Go-native runner.
- [`project_analyst_agent.md`](../tech_specs/project_analyst_agent.md): Update LangGraph references in verification section.
- [`postgres_schema.md`](../tech_specs/postgres_schema.md): Update source attribution from "LangGraph" to "workflow engine."
- [`_main.md`](../tech_specs/_main.md): Update index entry.
- [`mvp.md`](../mvp.md) and [`mvp_plan.md`](../mvp_plan.md): Update Phase 2 LangGraph references.
- [`docs/requirements/orches.md`](../requirements/orches.md) and [`docs/requirements/agents.md`](../requirements/agents.md): Update references to `langgraph_mvp.md` and spec ID `CYNAI.AGENTS.LanggraphCheckpointing`.

### Code That Would Not Change

- Workflow HTTP API handlers (`orchestrator/internal/handlers/workflow.go`).
- Workflow DB operations (`orchestrator/internal/database/workflow.go`, `workflow_gate.go`).
- Domain models (`WorkflowCheckpoint`, `TaskWorkflowLease` in `models.go`).
- PMA langchaingo agent loop (`agents/internal/pma/`).
- MCP gateway and allowlists.
- BDD features and E2E tests for workflow API contract.

### Code That Would Change

- **New:** Go workflow runner (state machine implementing the graph topology, calling into existing langchaingo/MCP code at each node).
- **Update:** Comment references to "LangGraph" in `database.go`, `workflow_gate.go`, `models.go` (cosmetic).
- **Remove:** `scripts/workflow_runner_stub/minimal_runner.py` (replaced by the Go runner) or retain as an external-client contract reference.
- **New or updated:** PMA instruction set / skill for workflow-executor behaviors.

### Requirements That Remain Satisfied

All workflow-related requirements (REQ-ORCHES-0144 through 0147, 0152, 0153, 0176-0180, REQ-AGENTS-0004, 0116-0118) are about *behavior* (checkpoint per transition, resumable, idempotent, single-active-per-task, start gate), not about *Python* or *LangGraph*.
A Go-native runner that satisfies the same behavioral contracts meets every requirement.

## References

- [`docs/tech_specs/langgraph_mvp.md`](../tech_specs/workflow_mvp.md)
- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) (Workflow Engine section)
- [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md)
- [`docs/tech_specs/project_manager_agent.md`](../tech_specs/project_manager_agent.md)
- [`docs/dev_docs/2026-03-29_implementation_state_report.md`](2026-03-29_implementation_state_report.md) (Phase 2 status)
- [`docs/dev_docs/_todo.md`](_todo.md) (P2-06 gap)
- [`scripts/workflow_runner_stub/minimal_runner.py`](../../scripts/workflow_runner_stub/minimal_runner.py)
