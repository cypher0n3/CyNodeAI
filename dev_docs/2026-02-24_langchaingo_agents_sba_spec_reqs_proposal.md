# Proposed Spec/Reqs Update: Build SBA Using Langchaingo

- [Summary](#summary)
- [Rationale](#rationale)
- [Current State](#current-state)
- [How LangGraph Fits Into the Orchestrator](#how-langgraph-fits-into-the-orchestrator)
- [Proposed Direction](#proposed-direction)
- [Langchaingo Capabilities Relevant to CyNodeAI](#langchaingo-capabilities-relevant-to-cynodeai)
- [SBA (`cynode-sba`) First](#sba-cynode-sba-first)
- [Orchestrator Workflow (LangGraph) Retained](#orchestrator-workflow-langgraph-retained)
- [PMA Refactor: Langchaingo for Tool Use](#pma-refactor-langchaingo-for-tool-use)
- [Proposed Requirements and Spec Changes](#proposed-requirements-and-spec-changes)
- [Risks and Mitigations](#risks-and-mitigations)
- [References](#references)

## Summary

**Date:** 2026-02-24
**Mode:** Proposal only (dev_docs); no code or normative spec/req changes in this document.
**Scope:** Propose building the SandBox Agent (SBA) using the Go library [langchaingo](https://github.com/tmc/langchaingo), and refactoring the Project Manager Agent (PMA) to use langchaingo for LLM and tool execution (e.g. multiple simultaneous tool calls).
  The orchestrator workflow engine (LangGraph, checkpointing to PostgreSQL) remains as currently specified; this proposal does not replace it.

This document is a proposed spec/requirements update for stakeholder review.
Implementation of any changes would require explicit direction and updates to `docs/requirements/` and `docs/tech_specs/` per project standards.

## Rationale

- **Go-native SBA:** The SandBox Agent (`cynode-sba`) is already specified as a runner binary inside a container; it needs an LLM client, tool execution (MCP), and a simple agent loop.
  Building SBA in Go with langchaingo keeps the sandbox runtime in the same language as the worker node and avoids a separate Python stack in the container.
- **SBA alignment:** Langchaingo provides agents, tools, and LLM abstractions (including Ollama and OpenAI-compatible endpoints) in Go.
  Fits job-defined inference allowlist, worker proxy, and API Egress.
- **REST-in-Go:** All REST APIs are required to be implemented in Go; SBA is not an API server but consistency with a Go-based execution stack is desirable.

## Current State

- **LangGraph MVP:** [`docs/tech_specs/langgraph_mvp.md`](../docs/tech_specs/langgraph_mvp.md) defines the Project Manager Agent workflow as a **Python LangGraph process** invoked by the Go orchestrator (HTTP/RPC).
  Checkpointing is to PostgreSQL; graph nodes call MCP and Worker API.
- **SBA:** [`docs/tech_specs/cynode_sba.md`](../docs/tech_specs/cynode_sba.md) defines `cynode-sba` as a deterministic runner in a sandbox: job spec in, result contract out; todo list; inference via worker proxy or API Egress; MCP tools on sandbox allowlist.
  Implementation language is not specified; the SBA container could be Go or Python.
- **Requirements:** [`docs/requirements/agents.md`](../docs/requirements/agents.md) references LangGraph checkpointing (REQ-AGENTS-0004, REQ-AGENTS-0116, REQ-AGENTS-0117, REQ-AGENTS-0118). [`docs/requirements/sbagnt.md`](../docs/requirements/sbagnt.md) defines SBA behavior without mandating a framework.

## How LangGraph Fits Into the Orchestrator

This proposal does not change the orchestrator workflow; the following is context for why LangGraph is retained.

- **Role:** LangGraph is the **workflow engine** for the Project Manager Agent.
  It runs the graph (Load Task Context, Plan, Dispatch, Collect, Verify, Finalize/Mark Failed); the Go orchestrator does not execute graph steps.
- **Process boundary (Phase 2):** The workflow engine is a **separate Python LangGraph process**.
  The Go orchestrator starts and resumes it via HTTP (Workflow Start/Resume API); the LangGraph process runs the graph and does not serve REST APIs.
- **Checkpointing:** The **workflow runner (LangGraph)** is responsible for checkpointing.
  It MUST persist checkpoint data **after each node transition** so the graph can resume after restart.
  The checkpoint store MUST be backed by **PostgreSQL** (table `workflow_checkpoints`: `task_id`, `state` jsonb, `last_node_id`, `updated_at`; one row per task, upsert by `task_id`).
  So LangGraph "logs/updates" workflow state in Postgres; the orchestrator MUST NOT run workflow steps without going through this checkpoint layer.
- **Lease:** The **orchestrator DB** holds a **task workflow lease** (`task_workflow_leases`) so only one active workflow runs per task.
  The workflow runner acquires or checks the lease via the orchestrator (e.g. as part of StartWorkflow); the orchestrator is the source of truth for who holds the lease.
- **Business data:** All other database reads and writes from the workflow (task context, plans, verification, final summary) MUST go through **MCP database tools**; the workflow MUST NOT connect directly to PostgreSQL for those.
  So there are two channels: (1) **checkpoint store** (Postgres, used by the workflow runner for graph state), and (2) **business data** (only via MCP/orchestrator).

This proposal retains LangGraph as the orchestrator workflow engine; no replacement with a Go workflow engine is proposed.

## Proposed Direction

1. **Adopt langchaingo as the framework** for the SandBox Agent and for PMA's LLM and tool use (and any other agent logic implemented in Go).
2. **Build SBA** using langchaingo: `cynode-sba` implemented in Go using langchaingo for the agent loop (e.g. ReAct/MRKL-style or tool-calling), LLM abstraction (Ollama, OpenAI-compatible for API Egress), and tools (MCP client or langchaingo tools wrapping MCP).
3. **Refactor PMA** to use langchaingo for LLM calls and tool execution (e.g. multiple simultaneous tool calls).
  LangGraph remains the workflow engine (graph, checkpointing, lease); PMA nodes would invoke langchaingo for the actual agentic steps (planning, verification, MCP tool use).

## Langchaingo Capabilities Relevant to CyNodeAI

- **Repository:** <https://github.com/tmc/langchaingo>
- **Docs:** <https://tmc.github.io/langchaingo/docs/>

Relevant modules:

- **llms:** Multi-provider LLM interface (OpenAI, Ollama, etc.).
  Fits job-defined inference allowlist and worker proxy (Ollama) or API Egress (OpenAI-compatible).
- **agents:** MRKL (`mrkl`), conversational, and OpenAI functions-style agents.
  Fits SBA "decide next step / call tool / interpret result" loop.
- **tools:** Tool interface and call semantics.
  MCP tools can be wrapped as langchaingo tools so the agent calls through the existing MCP gateway contract.
- **memory:** Conversation and state persistence.
  Can be used for job-scoped temporary memory (see [CYNAI.SBAGNT.TemporaryMemory](../docs/tech_specs/cynode_sba.md#spec-cynai-sbagnt-temporarymemory)).
- **chains:** Composable sequences; optional for structured multi-step flows.

## SBA (`cynode-sba`) First

- **Tech spec:** Extend [`docs/tech_specs/cynode_sba.md`](../docs/tech_specs/cynode_sba.md) (or an implementation note) to state that the **canonical implementation** of `cynode-sba` is in **Go using langchaingo** for:
  - LLM calls (via `llms` with provider set from job/runtime: Ollama base URL or API Egress endpoint).
  - Agent loop (e.g. `agents` executor with tool-calling or MRKL).
  - Tools: MCP tools invoked via orchestrator MCP gateway (sandbox allowlist); implement as langchaingo tools that call MCP (or an in-sandbox MCP client).
- **Contract unchanged:** Job spec, result contract, Worker API integration, MCP tool access, and security constraints remain as specified; only the implementation stack is proposed to be Go + langchaingo.
- **Requirements:** No change to [`docs/requirements/sbagnt.md`](../docs/requirements/sbagnt.md) strictly required; optional addition of a REQ that the SBA implementation use a single, documented framework (e.g. langchaingo) for consistency and auditability.

## Orchestrator Workflow (LangGraph) Retained

- The **orchestrator-side workflow** (graph topology, checkpointing to PostgreSQL, lease, MCP for business data) remains as defined in [`docs/tech_specs/langgraph_mvp.md`](../docs/tech_specs/langgraph_mvp.md) and [`docs/tech_specs/orchestrator.md`](../docs/tech_specs/orchestrator.md).
- **No replacement** of the Python LangGraph process with a Go workflow engine is proposed.
- LangGraph continues to handle the graph, checkpointing, and resumability.
- PMA's **LLM and tool-calling** can be refactored to use langchaingo (e.g. multiple simultaneous tool calls) as described in [PMA Refactor: Langchaingo for Tool Use](#pma-refactor-langchaingo-for-tool-use).

## PMA Refactor: Langchaingo for Tool Use

- **Goal:** PMA should use langchaingo for LLM calls and tool execution, including **multiple simultaneous tool calls**, where supported by the model and gateway.
- **Constraint:** LangGraph remains the workflow engine (graph nodes, checkpoint after each transition, lease).
  Only the "brain" of PMA-the LLM and tool invocation layer-is refactored to langchaingo.
- **Integration options:** (a) LangGraph nodes (Python) call a **Go service** that uses langchaingo for plan-step, verify-step, or other agentic sub-steps; or (b) the workflow runner is extended so that agentic steps are executed by a Go component (e.g. same process via CGO, or sidecar) that uses langchaingo.
  MCP tool calls from PMA still go through the orchestrator MCP gateway; langchaingo tools wrap those MCP calls.
- **Spec impact:** [`docs/tech_specs/project_manager_agent.md`](../docs/tech_specs/project_manager_agent.md) and/or [`docs/tech_specs/langgraph_mvp.md`](../docs/tech_specs/langgraph_mvp.md) could be extended to state that PMA uses langchaingo (Go) for LLM and tool execution, including parallel tool calls where applicable.

## Proposed Requirements and Spec Changes

- **Area:** REQ-AGENTS-0004, 0116, 0117, 0118 and langgraph_mvp.md
  - **No change** to workflow or checkpointing; LangGraph remains the workflow implementation.
- **Area:** cynode_sba.md
  - current: No implementation framework specified.
  - proposed change: Add "Implementation: canonical implementation in Go using langchaingo" (and optionally a short subsection on langchaingo usage: llms, agents, tools).
- **Area:** project_manager_agent.md / langgraph_mvp.md (PMA tool use)
  - proposed change: Extend to state that PMA uses langchaingo (Go) for LLM and tool execution, including multiple simultaneous tool calls where supported; LangGraph remains the graph runner and checkpoint owner.
- **Area:** docs/requirements/agents.md
  - **No change** to workflow-related links; LangGraph remains the implementation.

No new REQ IDs are proposed; SBA and PMA tech specs are extended to call out Go + langchaingo where applicable.

## Risks and Mitigations

- **Langchaingo maturity:** Langchaingo is actively maintained and widely used; API and behavior may evolve.
  Mitigation: pin version in go.mod; treat agent and tool code as internal abstraction so provider can be swapped if needed.
- **MCP from Go:** SBA needs to call MCP tools (sandbox allowlist) from the Go binary.
  Mitigation: use existing MCP client in Go (or introduce one); wrap MCP calls as langchaingo tools so the agent interface is consistent.

## References

- Langchaingo: <https://github.com/tmc/langchaingo>
- Langchaingo docs: <https://tmc.github.io/langchaingo/docs/>
- [`docs/tech_specs/cynode_sba.md`](../docs/tech_specs/cynode_sba.md) - CyNode SBA spec
- [`docs/tech_specs/langgraph_mvp.md`](../docs/tech_specs/langgraph_mvp.md) - LangGraph MVP workflow
- [`docs/tech_specs/project_manager_agent.md`](../docs/tech_specs/project_manager_agent.md) - PMA behavior and tool access
- [`docs/tech_specs/orchestrator.md`](../docs/tech_specs/orchestrator.md) - Workflow engine and lease
- [`docs/tech_specs/postgres_schema.md`](../docs/tech_specs/postgres_schema.md) - `workflow_checkpoints`, `task_workflow_leases`
- [`docs/requirements/agents.md`](../docs/requirements/agents.md) - AGENTS requirements
- [`docs/requirements/sbagnt.md`](../docs/requirements/sbagnt.md) - SBAGNT requirements
- [`docs/docs_standards/spec_authoring_writing_and_validation.md`](../docs/docs_standards/spec_authoring_writing_and_validation.md) - Spec authoring standards
