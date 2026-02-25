# STEPEX Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `STEPEX` domain.
It defines the normative requirements for the simple step-by-step executor runner binary (e.g. `cynode-sse`).

The simple step executor runs a **predefined ordered list of steps** inside a sandbox container.
It does **not** use inference (LLM), MCP tools in the execution loop, or a todo list; it is distinct from the SandBox Agent (`cynode-sba`), which is langchaingo-centric and agent-driven.
The step executor executes steps strictly in array order and emits a structured result for orchestrator storage.
Sandbox boundary and isolation follow the `SANDBX` domain; dispatch uses the same Worker API model as SBA (image + command; `/job/job.json`, `/job/result.json`).

## 2 Requirements

- **REQ-STEPEX-0001:** The system MUST provide a sandbox runner binary that executes a validated **ordered list of steps** without using inference (no LLM calls).
  [CYNAI.STEPEX.Doc.CyNodeStepExecutor](../tech_specs/cynode_step_executor.md#spec-cynai-stepex-doc-cynodestepexecutor)
  <a id="req-stepex-0001"></a>

- **REQ-STEPEX-0100:** The step executor MUST accept a versioned job specification and MUST refuse unknown major protocol versions.
  [CYNAI.STEPEX.ProtocolVersioning](../tech_specs/cynode_step_executor.md#spec-cynai-stepex-protocolversioning)
  <a id="req-stepex-0100"></a>

- **REQ-STEPEX-0101:** The step executor MUST validate the job specification schema before executing any step and MUST fail closed on validation errors (unknown fields, missing required fields, invalid values).
  [CYNAI.STEPEX.SchemaValidation](../tech_specs/cynode_step_executor.md#spec-cynai-stepex-schemavalidation)
  <a id="req-stepex-0101"></a>

- **REQ-STEPEX-0102:** The step executor MUST run as a non-root user and MUST enforce timeouts and output size limits specified by job constraints.
  It MUST NOT require inference, MCP tool calls during step execution, or a todo list.
  [CYNAI.STEPEX.ExecutionModel](../tech_specs/cynode_step_executor.md#spec-cynai-stepex-executionmodel)
  [CYNAI.STEPEX.SandboxBoundary](../tech_specs/cynode_step_executor.md#spec-cynai-stepex-sandboxboundary)
  <a id="req-stepex-0102"></a>

- **REQ-STEPEX-0103:** The step executor MUST emit a structured result object (e.g. `/job/result.json`) that includes status, step results, and failure_code (and optionally failure_message) suitable for orchestrator storage and auditing.
  [CYNAI.STEPEX.ResultContract](../tech_specs/cynode_step_executor.md#spec-cynai-stepex-resultcontract)
  <a id="req-stepex-0103"></a>

- **REQ-STEPEX-0104:** The step executor MUST integrate with the Worker API: the node runs a container with an image and command that invokes the runner; the job payload is read from the agreed location (e.g. `/job/job.json`) and the result is written to the agreed location (e.g. `/job/result.json`) so the node can return it to the orchestrator.
  [CYNAI.STEPEX.WorkerApiIntegration](../tech_specs/cynode_step_executor.md#spec-cynai-stepex-workerapiintegration)
  [CYNAI.WORKER.WorkerApiRunJobSyncV1](../tech_specs/worker_api.md#spec-cynai-worker-workerapirunjobsync-v1)
  <a id="req-stepex-0104"></a>

- **REQ-STEPEX-0105:** The step executor MUST comply with sandbox constraints and non-secret requirements defined in the `SANDBX` domain where applicable (filesystem layout, non-root, egress only via worker proxies when policy allows).
  [REQ-SANDBX-0001](./sandbx.md#req-sandbx-0001)
  [CYNAI.STEPEX.SandboxBoundary](../tech_specs/cynode_step_executor.md#spec-cynai-stepex-sandboxboundary)
  <a id="req-stepex-0105"></a>
