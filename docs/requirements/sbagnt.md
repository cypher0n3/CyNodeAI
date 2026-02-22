# SBAGNT Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `SBAGNT` domain.
It defines the normative requirements for the `cynode-sba` sandbox agent runner binary.

`cynode-sba` is a deterministic runner that executes job specifications.
It is not an LLM; it MUST have access to at least one model (via worker proxy or API Egress) and may call inference only using models the job allows.
It builds and manages its own todo list from the job; it does not decide policy or scheduling (orchestrator and worker node do that).
Sandbox network egress, when permitted by policy, is only via worker proxies (inference, web egress, API Egress); see `SANDBX` and [cynode_sba](../tech_specs/cynode_sba.md).

This domain focuses on the runner protocol, inference allowlist, todo list, and enforcement behavior.
Sandbox image and isolation requirements remain in the `SANDBX` domain.

## 2 Requirements

- **REQ-SBAGNT-0001:** The system MUST provide a sandbox runner binary named `cynode-sba` that executes validated job specifications deterministically inside a sandbox container.
  [CYNAI.SBAGNT.Doc.CynodeSba](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-doc-cynodesba)
  <a id="req-sbagnt-0001"></a>

- **REQ-SBAGNT-0100:** `cynode-sba` MUST accept a versioned job specification and MUST refuse unknown major protocol versions.
  [CYNAI.SBAGNT.ProtocolVersioning](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-protocolversioning)
  <a id="req-sbagnt-0100"></a>

- **REQ-SBAGNT-0101:** `cynode-sba` MUST validate the job specification schema before executing any step and MUST fail closed on validation errors.
  [CYNAI.SBAGNT.SchemaValidation](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-schemavalidation)
  <a id="req-sbagnt-0101"></a>

- **REQ-SBAGNT-0102:** `cynode-sba` MUST run as a non-root user and MUST enforce timeouts and output size limits specified by job constraints.
  It MUST NOT enforce command or path allowlists inside the container; the sandbox container is the isolation boundary, and the runner has full access to `/workspace` and may run any user-level command.
  [CYNAI.SBAGNT.Enforcement](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-enforcement)
  [CYNAI.SBAGNT.SandboxBoundary](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-sandboxboundary)
  <a id="req-sbagnt-0102"></a>

- **REQ-SBAGNT-0103:** `cynode-sba` MUST emit a structured result object (`result.json` or equivalent) that includes step results, status, and artifact metadata suitable for orchestrator storage and auditing.
  [CYNAI.SBAGNT.ResultContract](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-resultcontract)
  <a id="req-sbagnt-0103"></a>

- **REQ-SBAGNT-0104:** `cynode-sba` MUST comply with sandbox constraints and non-secret requirements defined in the `SANDBX` domain.
  [REQ-SANDBX-0001](./sandbx.md#req-sandbx-0001)
  [CYNAI.SBAGNT.SandboxBoundary](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-sandboxboundary)
  <a id="req-sbagnt-0104"></a>

- **REQ-SBAGNT-0105:** Sandbox agents (including `cynode-sba` when operating as an agent) MUST use only MCP tools permitted for the sandbox/worker role (sandbox allowlist).
  They MUST invoke tools through the orchestrator MCP gateway and MUST NOT invoke tools designated PM-only (orchestrator-side only).
  [CYNAI.SBAGNT.McpToolAccess](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-mcptoolaccess)
  [CYNAI.MCPGAT.PerToolScope](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-pertoolscope)
  <a id="req-sbagnt-0105"></a>

- **REQ-SBAGNT-0106:** When a worker node runs a sandbox container that uses the SBA runner image, the job payload or container entrypoint MUST be such that `cynode-sba` reads the job spec from the agreed location and writes the result per the result contract; the Worker API response MUST be derivable from the SBA result and container lifecycle.
  [CYNAI.SBAGNT.WorkerApiIntegration](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-workerapiintegration)
  [CYNAI.WORKER.WorkerApiRunJobSyncV1](../tech_specs/worker_api.md#spec-cynai-worker-workerapirunjobsync-v1)
  <a id="req-sbagnt-0106"></a>

- **REQ-SBAGNT-0107:** The system MUST supply the sandbox agent with the context needed to perform its work: requirements, acceptance criteria, relevant user/task preferences, and skills (or skill references) applicable to the task.
  The orchestrator (or PM when constructing the job) is responsible for resolving and attaching this context; the SBA MUST receive it and use it when performing and verifying work.
  [CYNAI.SBAGNT.JobContext](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-jobcontext)
  <a id="req-sbagnt-0107"></a>

- **REQ-SBAGNT-0111:** The sandbox agent MUST receive and use baseline context (identity, role, responsibilities, non-goals) for every LLM it calls, and MUST receive user-configurable additional context resolved from preferences (same scope precedence as other preferences) and include it in LLM prompts.
  [REQ-AGENTS-0132](./agents.md#req-agents-0132)
  [REQ-AGENTS-0133](./agents.md#req-agents-0133)
  [CYNAI.AGENTS.LLMContext](../tech_specs/project_manager_agent.md#spec-cynai-agents-llmcontext)
  [CYNAI.SBAGNT.JobContext](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-jobcontext)
  <a id="req-sbagnt-0111"></a>

- **REQ-SBAGNT-0108:** `cynode-sba` MUST be able to build and manage its own todo list based on the job (requirements, acceptance criteria, steps).
  The todo list MUST be derived from job context and updated as the SBA executes; the implementation MUST persist todo state as needed and MAY include a summary in the result contract.
  [CYNAI.SBAGNT.TodoList](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-todolist)
  <a id="req-sbagnt-0108"></a>

- **REQ-SBAGNT-0109:** The SBA MUST have access to at least one model for inference (via worker proxy or orchestrator-mediated API Egress).
  The job MUST dictate which models the SBA is allowed to use; the SBA MUST use only models from that allowlist.
  The runtime (orchestrator and/or node) MUST ensure at least one allowed model is available and MUST inject the appropriate inference endpoint(s) into the sandbox so the SBA can call it without handling credentials.
  [CYNAI.SBAGNT.WorkerProxies](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-workerproxies)
  [CYNAI.SBAGNT.JobInferenceModel](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-jobinferencemodel)
  <a id="req-sbagnt-0109"></a>

- **REQ-SBAGNT-0110:** The SBA MUST confirm job acceptance and signal that the job is in progress (after reading and validating the job spec) so the node can report to the orchestrator; the SBA MUST report completion by writing the result contract to the agreed location.
  The worker node MUST NOT clear the job result until the result has been successfully persisted to the orchestrator database; the node MAY store the result temporarily in node-local SQLite until upload succeeds.
  [CYNAI.SBAGNT.JobLifecycle](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-joblifecycle)
  [CYNAI.WORKER.JobLifecycleResultPersistence](../tech_specs/worker_api.md#spec-cynai-worker-joblifecycleresultpersistence)
  <a id="req-sbagnt-0110"></a>
