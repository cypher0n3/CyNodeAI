# CyNode SandBox Agent (`cynode-sba`)

- [Document Overview](#document-overview)
- [Purpose](#purpose)
- [Design Principles](#design-principles)
- [Execution Model](#execution-model)
  - [Todo List](#todo-list)
- [Integration With Worker API](#integration-with-worker-api)
  - [Job Lifecycle and Status Reporting](#job-lifecycle-and-status-reporting)
  - [Worker Proxies (Inference and Web Egress)](#worker-proxies-inference-and-web-egress)
- [Job Specification](#job-specification)
  - [Inference Models (Job-Defined Allowlist)](#inference-models-job-defined-allowlist)
  - [Context Supplied to SBA (Requirements, Acceptance Criteria, Preferences, Skills)](#context-supplied-to-sba-requirements-acceptance-criteria-preferences-skills)
- [Step Types (MVP)](#step-types-mvp)
- [Result Contract](#result-contract)
  - [Canonical Failure Codes](#canonical-failure-codes)
- [Sandbox Boundary and Security](#sandbox-boundary-and-security)
  - [Shape of Sandboxed Containers](#shape-of-sandboxed-containers)
  - [Security Constraints](#security-constraints)
- [MCP Tool Access (Sandbox Allowlist)](#mcp-tool-access-sandbox-allowlist)
- [SBA Container Image (Containerfile)](#sba-container-image-containerfile)
- [Protocol Versioning](#protocol-versioning)

## Document Overview

- Spec ID: `CYNAI.SBAGNT.Doc.CyNodeSba` <a id="spec-cynai-sbagnt-doc-cynodesba"></a>

This document defines `cynode-sba`, the sandbox agent runner binary.
It is a deterministic executor that runs inside a sandbox container; see [Sandbox Boundary and Security](#sandbox-boundary-and-security) for container shape and the network model (egress only via worker proxies, not airgapped).

This spec is derived from the draft runner design in:

- [`docs/draft_specs/cynode-agent_rough_spec.md`](../draft_specs/cynode-agent_rough_spec.md)

This spec aligns with:

- [`docs/tech_specs/sandbox_container.md`](sandbox_container.md)
- [`docs/tech_specs/worker_api.md`](worker_api.md)

Abbreviation note: This doc may abbreviate "SandBox Agent" to "SBA" throughout.

## Purpose

`cynode-sba` executes validated job specifications.
It is not an LLM; it MUST have access to at least one model (via worker proxy or API Egress) and calls inference using only models the job allows.
It does not decide policy or scheduling; the orchestrator and worker-node components are responsible for policy and sandbox lifecycle.
Within the job, the SBA MUST be able to build and manage its own todo list (derived from requirements, acceptance criteria, and steps) to track and drive progress.

## Design Principles

- Small attack surface.
- Strict schema adherence (unknown job fields rejected; validation before any step runs).
- **No command or path allowlists inside the container.**
  The sandbox agent runs in an already-sandboxed environment (the container).
  It does not need strict allowlists for commands or paths; it MAY run any **user-level** command on the system and MUST have full access to the **`/workspace`** directory.
  The process MUST NOT run as root.
- Fail-closed on **schema validation** only (invalid job spec is rejected; no step execution).
  Runtime enforcement is bounded by the container and non-root execution, not by per-command or per-path allowlists.
  The orchestrator or worker MAY use command or path allowlists when constructing or validating job requests; the SBA does not enforce them inside the container.
- Structured, machine-parseable results.

## Execution Model

`cynode-sba` runs as the main process inside a sandbox container.
It reads a job spec and produces a result object.

**Job directory (`/job/`)**: A well-known path inside the sandbox container that the node provides for the current job run.
It holds job input (e.g. the job spec) and MAY be used by the SBA to stage the result and artifacts before pushing them outbound (see [Result and Artifact Delivery (Outbound via Proxies)](#result-and-artifact-delivery-outbound-via-proxies)).
It is per-job, not a system-wide or host path.
See [Shape of Sandboxed Containers](#shape-of-sandboxed-containers) for the full container layout.

Invocation modes

- File-based (recommended):
  - Read: `/job/job.json`
  - Write: `/job/result.json`
  - Artifacts: `/job/artifacts/`
- Stdin/stdout (optional):
  - Read job JSON from stdin.
  - Write result JSON to stdout.

The SBA MUST be able to upload attachments (e.g. build outputs, logs, evidence) for the task.
Delivery to the orchestrator happens via outbound calls through worker proxies (see [Result and Artifact Delivery (Outbound via Proxies)](#result-and-artifact-delivery-outbound-via-proxies)); the SBA MAY also stage files under `/job/artifacts/` for node-mediated delivery where that path is supported.

### Todo List

- Spec ID: `CYNAI.SBAGNT.TodoList` <a id="spec-cynai-sbagnt-todolist"></a>

The SBA MUST be able to build and manage its own todo list based on the job.
The todo list is derived from job context (requirements, acceptance criteria, and any initial or suggested steps) and is updated as the SBA executes (e.g. add sub-tasks, mark complete, reorder).
The implementation MUST persist todo state as needed (e.g. under `/job/` or in memory) so the SBA can resume and report progress; the result contract MAY include a summary of the todo list or completed items.

## Integration With Worker API

- Spec ID: `CYNAI.SBAGNT.WorkerApiIntegration` <a id="spec-cynai-sbagnt-workerapiintegration"></a>

The orchestrator dispatches sandbox jobs to nodes via the [Worker API](worker_api.md) (e.g. `POST /v1/worker/jobs:run`).
When the job uses an image that runs `cynode-sba` as the main process (the SBA runner image), the node starts the container; `cynode-sba` reads the job spec from the agreed location (e.g. `/job/job.json`), executes the steps, and writes the [Result contract](#result-contract) (e.g. `/job/result.json`).

Contract alignment

- The Worker API request carries `task_id`, `job_id`, and sandbox config (image, command, env, timeout).
- For SBA-runner jobs, the job spec consumed by `cynode-sba` (e.g. `job.json`) MUST include `job_id` and `task_id` and MUST be produced or validated by the orchestrator/PM so that result storage and auditing can correlate with the Worker API response.
- The node MAY build the Worker API response (status, stdout/stderr, timing) from the container exit code and/or from the SBA result object so that the orchestrator receives a single, consistent result for the job.

Sandbox orchestration from the PM agent uses MCP tools (`sandbox.create`, `sandbox.exec`, etc.) per [mcp_tool_catalog.md](mcp_tool_catalog.md#spec-cynai-mcptoo-sandboxtools); the **content** of a sandbox run may be a `cynode-sba` job when the image is the SBA runner and the command/entrypoint invokes `cynode-sba`.

See:

- [`docs/tech_specs/worker_api.md`](worker_api.md)
- [`docs/tech_specs/mcp_tool_catalog.md`](mcp_tool_catalog.md#spec-cynai-mcptoo-sandboxtools)

### Job Lifecycle and Status Reporting

- Spec ID: `CYNAI.SBAGNT.JobLifecycle` <a id="spec-cynai-sbagnt-joblifecycle"></a>

The SBA MUST participate in a clear job lifecycle so the orchestrator can mark the job as in-progress and, when done, persist the result.
Connections are NOT required to stay open for the full duration of the job; the node is responsible for conveying status and result to the orchestrator (e.g. by callback, polling, or a single long-lived response with streaming).

#### Result and Artifact Delivery (Outbound via Proxies)

- Spec ID: `CYNAI.SBAGNT.ResultAndArtifactDelivery` <a id="spec-cynai-sbagnt-resultandartifactdelivery"></a>

Result, status, and artifacts move from the SBA to the orchestrator by the SBA making **outbound calls through worker proxies**.
The SBA reports job status (in-progress, completion) and uploads artifacts by calling orchestrator-mediated endpoints (e.g. job callback URL, status API) or MCP tools (e.g. `artifact.put`) over the network; all traffic goes through the worker proxies so the sandbox never has direct internet access.
The runtime (node and/or orchestrator) MUST provide the SBA with the means to reach these endpoints (e.g. job callback URL, MCP gateway URL) via the proxy path; the node injects the necessary URLs or configuration (e.g. env vars, job payload) so the SBA can report and upload without handling credentials.
The SBA MAY also write the [Result contract](#result-contract) and artifact files under `/job/` for staging or for node-mediated delivery (the node then reads and forwards after the container exits); the implementation MAY use that path as a fallback or in addition to outbound reporting.

#### In-Progress State

After the SBA has read and validated the job spec (and before or as it begins executing steps), the SBA MUST confirm acceptance and signal that the job is **in progress**.
The SBA MUST do so via outbound call through the worker proxy (e.g. to an orchestrator job-status endpoint) and MAY additionally write a status file under `/job/` or use another implementation-defined signal.
The node MAY also infer in-progress when the SBA has read the job and not yet exited; the orchestrator MUST be able to update job state to in-progress.

#### Completion State

When the job finishes (success, failure, or timeout), the SBA MUST report completion by making an outbound call through the worker proxy to deliver the [Result contract](#result-contract) (and optionally artifact references or inline artifact data) to the orchestrator.
The SBA MAY also write the result to the agreed location (e.g. `/job/result.json`) for staging or node-mediated delivery; if so, the node MUST NOT clear or delete the job result until the result has been successfully persisted to the orchestrator (e.g. the node uploads from `/job/` or the SBA has already reported via proxy).
The orchestrator MUST pass job completion (status and result) to the Project Manager Agent and/or Project Analyst Agent for additional work (e.g. verification, remediation); see [Orchestrator - Task Scheduler](orchestrator.md#spec-cynai-orches-scheduledrunrouting).

See [Worker API - Job lifecycle and result persistence](worker_api.md#spec-cynai-worker-joblifecycleresultpersistence).

### Worker Proxies (Inference and Web Egress)

- Spec ID: `CYNAI.SBAGNT.WorkerProxies` <a id="spec-cynai-sbagnt-workerproxies"></a>

When `cynode-sba` runs inside a worker-node sandbox, the **node** provides proxy endpoints for inference and (when policy allows) outbound HTTP.
The SBA does not configure or discover these endpoints itself; it relies on the Node Manager to inject the appropriate environment and pod setup so that steps and tooling inside the sandbox use the worker proxies.

Inference proxy

- When the node provides Ollama inference, the Node Manager runs the sandbox in a pod that includes an **inference proxy sidecar**.
  The proxy listens on `localhost:11434` inside the pod; the node injects `OLLAMA_BASE_URL=http://localhost:11434` into the sandbox container environment.
  Any step or tool that needs inference (e.g. an LLM call from within the sandbox) MUST use this endpoint; the proxy forwards to the node's Ollama container and keeps traffic node-local.

Web egress proxy

- When the sandbox network policy allows allowlisted egress, the node configures the sandbox to use the **node-local web egress proxy** via standard proxy environment variables (`HTTP_PROXY`, `HTTPS_PROXY`, and optionally `NO_PROXY`).
  The node-local proxy forwards requests to the orchestrator's Web Egress Proxy; only allowlisted destinations are permitted.
  Steps that perform outbound HTTP (e.g. `pip install`, `curl`, package fetches) use these env vars; the SBA and its steps do not set or override proxy URLs.

Inference access and allowed models

- The SBA MUST have access to **at least one** model; the runtime (node and/or orchestrator) MUST provide inference via either:
  - **Worker proxy (node-local):** When the node provides Ollama, the Node Manager injects `OLLAMA_BASE_URL=http://localhost:11434`; the SBA calls this endpoint using only models from the job's allowed set.
  - **API Egress (external models):** The orchestrator MUST make inference available to the sandbox via an orchestrator-mediated endpoint (e.g. a task-scoped chat completion URL that routes through the API Egress Server) for any allowed model served by an external provider; credentials remain outside the sandbox.
- The **job dictates which models the SBA is allowed to use** (see [Job Specification](#job-specification)); the SBA MUST use only models from that allowlist, and the runtime MUST inject the appropriate endpoint(s) so at least one allowed model is reachable.

Summary

- Outbound traffic from the sandbox is permitted **only** through these worker proxies (and any orchestrator-mediated API Egress); there is no direct internet or other egress from the container.
  Sandboxes are not airgapped but have strict controls on what is allowed in and out.
- The SBA runner and job spec do not define proxy URLs or inference base URLs.
  The worker node (and orchestrator, when API Egress is used) are responsible for injecting the inference endpoint and for configuring proxy env vars when web egress is allowed, per the sandbox contract.

See:

- [`docs/tech_specs/worker_node.md`](worker_node.md#spec-cynai-worker-nodelocalinference) (inference proxy sidecar, `OLLAMA_BASE_URL` injection)
- [`docs/tech_specs/sandbox_container.md`](sandbox_container.md#spec-cynai-sandbx-nodelocalinf) (sandbox inference contract)
- [`docs/tech_specs/web_egress_proxy.md`](web_egress_proxy.md#spec-cynai-sandbx-integration-webegressproxy) (node-local proxy, proxy env vars)
- [`docs/tech_specs/ports_and_endpoints.md`](ports_and_endpoints.md#spec-cynai-stands-inferenceollamaandproxy) (inference proxy port and behavior)
- [`docs/tech_specs/external_model_routing.md`](external_model_routing.md#spec-cynai-orches-externalinferencenodesandboxes) (external inference with sandboxes)
- [`docs/tech_specs/api_egress_server.md`](api_egress_server.md) (API Egress, no credentials in sandbox)

## Job Specification

- Spec ID: `CYNAI.SBAGNT.SchemaValidation` <a id="spec-cynai-sbagnt-schemavalidation"></a>

The job is a JSON object.
Unknown fields MUST be rejected.
Validation MUST occur before any step execution.

Minimum required fields

- `protocol_version`, `job_id`, `task_id`, `constraints`, `steps`.
- Within `constraints`: `max_runtime_seconds` and `max_output_bytes` are required for timeout and output caps.
  Path and command allowlists are **not** required; the runner has full access to `/workspace` and may run any user-level command (non-root).
- Optional in `constraints`: `ext_net_allowed` (default false) documents whether the job is permitted to use **external** network access (outside worker/orchestrator) via worker proxies (e.g. web egress for dependency downloads or API Egress for external APIs).
  The SBA always has outbound via proxy for job lifecycle (status reporting, result delivery), MCP (e.g. skills, artifacts), and inference; this flag concerns access to external destinations only, not network as a whole.
  See [Worker Proxies](#worker-proxies-inference-and-web-egress) and [Sandbox Boundary and Security](#sandbox-boundary-and-security).

### Inference Models (Job-Defined Allowlist)

- Spec ID: `CYNAI.SBAGNT.JobInferenceModel` <a id="spec-cynai-sbagnt-jobinferencemodel"></a>

The job MUST dictate **which models the SBA is allowed to use** for inference (e.g. for planning, refinement, or tool use).
The SBA MUST have access to at least one of the allowed models; the orchestrator and node MUST ensure at least one allowed model is available (via worker proxy or orchestrator-mediated API Egress) and MUST inject the appropriate inference endpoint(s) into the sandbox.

The job MAY include an `inference` object with:

- `allowed_models` (array of strings, required when inference is used): allowlist of model identifiers the SBA may use (e.g. `llama3.2` for node-local Ollama, or provider-specific ids for API Egress).
- `source` (string, optional): `worker`, `api_egress`, or unset; when unset, the runtime MAY infer which path(s) to enable per model id or policy.
The SBA MUST use only models listed in `allowed_models`; the runtime makes at least one of them reachable.

### Context Supplied to SBA (Requirements, Acceptance Criteria, Preferences, Skills)

- Spec ID: `CYNAI.SBAGNT.JobContext` <a id="spec-cynai-sbagnt-jobcontext"></a>

Traces To:

- [REQ-SBAGNT-0107](../requirements/sbagnt.md#req-sbagnt-0107)
- [REQ-SBAGNT-0111](../requirements/sbagnt.md#req-sbagnt-0111)
- [REQ-AGENTS-0132](../requirements/agents.md#req-agents-0132)
- [REQ-AGENTS-0133](../requirements/agents.md#req-agents-0133)
- [REQ-AGENTS-0134](../requirements/agents.md#req-agents-0134)

The orchestrator (or PM agent when constructing the job) MUST supply the sandbox agent with the context needed to perform its work.
The SBA MAY fetch relevant skills via the orchestrator MCP gateway using the sandbox-allowed read tools (`skills.list`, `skills.get`) per [MCP Tool Access](#spec-cynai-sbagnt-mcptoolaccess).
Skills (or skill content) MAY also be supplied in the job context by the orchestrator or PM; job context and MCP fetch MAY both be used.
The SBA MUST receive at least:

- **Baseline context** - Identity, role, responsibilities, and non-goals for the sandbox agent.
  MUST be included in every LLM prompt or system message used by the SBA.
  Supplied in the job (e.g. `context.baseline_context` or a dedicated instructions/baseline document) or baked into the SBA image; the orchestrator or PM MUST ensure it is present when the SBA calls an LLM.
- **Project-level context** - When the job is scoped to a project, the job MUST include project-level context: project identity (id, name, slug), scope, and relevant project metadata.
  MUST be included in every LLM prompt used by the SBA when present in the job.
- **Task-level context** - The job is task-scoped; the job MUST include task-level context: task identity (id, name), acceptance criteria summary, status, and relevant task metadata.
  MUST be included in every LLM prompt used by the SBA.
- **Requirements** - Task or project requirements that the work must satisfy (e.g. textual or structured requirements relevant to the job).
- **Acceptance criteria** - Criteria against which outputs or steps will be verified (e.g. completion conditions, quality gates).
- **Relevant preferences** - User or task-scoped preferences that affect how work is done (e.g. style, tooling, security or policy preferences).
  Only preferences relevant to the job need be included; the orchestrator is responsible for resolving and attaching them.
- **User-configurable additional context** - Resolved from preference key `agents.sandbox_agent.additional_context` using the same scope precedence as other preferences (task > project > user > group > system).
  MUST be merged into the context supplied to any LLM the SBA uses (e.g. after baseline context, project-level and task-level context, and job-specific instructions).
  See [User preferences - Agent additional context](user_preferences.md#spec-cynai-stands-agentadditionalcontext).
- **Skills** - Skills (or skill identifiers/content) that apply to the task, so the agent can use them during execution (e.g. how to use tools, conventions, or domain guidance).
  May be supplied in job context (inline content, stable skill ids, or paths under the job directory) and/or fetched by the SBA via MCP (`skills.list`, `skills.get`).

This context MAY be embedded in the job JSON (e.g. a `context` object with `baseline_context`, `project_context`, `task_context`, `requirements`, `acceptance_criteria`, `preferences`, `additional_context`, `skill_ids` or `skills`) or materialized into the container (e.g. files under `/job/context/` or `/job/requirements.md`).
The implementation MUST define how context is passed and MUST make it available to `cynode-sba` so the agent can perform and verify work against requirements and preferences.

Example shape (with optional context)

```json
{
  "protocol_version": "1.0",
  "job_id": "uuid",
  "task_id": "uuid",
  "constraints": {
    "max_runtime_seconds": 300,
    "max_output_bytes": 1048576,
    "ext_net_allowed": false
  },
  "inference": {
    "allowed_models": ["llama3.2"],
    "source": "worker"
  },
  "context": {
    "baseline_context": "Sandbox agent identity, role, responsibilities, non-goals (or path to same).",
    "project_context": "Project identity (id, name, slug), scope, relevant metadata (when job is project-scoped).",
    "task_context": "Task identity (id, name), acceptance criteria summary, status, relevant metadata.",
    "requirements": ["Requirement text or list"],
    "acceptance_criteria": ["Criterion 1", "Criterion 2"],
    "preferences": {},
    "additional_context": "User-configurable text from agents.sandbox_agent.additional_context (resolved at job creation).",
    "skill_ids": ["skill-uuid-1"],
    "skills": null
  },
  "steps": []
}
```

- `context`: **orchestrator MUST supply** requirements, acceptance criteria, relevant preferences, and skills (or references) to the SBA by job payload; the SBA MUST use this context when performing and verifying work.
- `constraints.ext_net_allowed`: when `true`, the job is permitted to use external network access (e.g. web egress, API Egress) via worker proxies; when `false`, only worker/orchestrator-mediated paths (inference, MCP, status) apply.
  The SBA always has proxy outbound for lifecycle and MCP.

## Step Types (MVP)

- Spec ID: `CYNAI.SBAGNT.Enforcement` <a id="spec-cynai-sbagnt-enforcement"></a>

The MVP step types are deterministic primitives.
The runner executes as a **non-root** user and has **full access to `/workspace`**; there are no command allowlists or path allowlists inside the container.

- `run_command`
  - Runs a command (argv form; no shell interpretation unless the step type explicitly requests it).
    Executed as the same non-root user; may run any user-level command with full access to `/workspace`.
    Working directory for the step is under `/workspace` or as specified in the step.
- `write_file`
  - Writes a file under `/workspace` (or a path relative to workspace).
    Rejects symlink escape outside workspace.
- `read_file`
  - Reads a file under `/workspace` (or a path relative to workspace) with a hard size cap from `constraints.max_output_bytes` or step-level cap.
- `apply_unified_diff`
  - Applies a unified diff relative to the workspace root.
    Rejects patches that would write outside `/workspace`.
- `list_tree`
  - Returns a structured tree representation (not raw shell output) for paths under `/workspace`.

## Result Contract

- Spec ID: `CYNAI.SBAGNT.ResultContract` <a id="spec-cynai-sbagnt-resultcontract"></a>

`cynode-sba` MUST emit a structured result object.
The result MUST be complete JSON even on failure.

Minimum result shape

```json
{
  "protocol_version": "1.0",
  "job_id": "uuid",
  "status": "success | failure | timeout",
  "steps": [],
  "artifacts": [],
  "failure_code": null,
  "failure_message": null
}
```

On failure, the SBA MUST set `failure_code` and `failure_message` so the orchestrator and user can understand why the job failed.
The SBA MUST use one of the defined failure codes when applicable; for other cases it MAY use an implementation-defined code (documented and stable).

### Canonical Failure Codes

- Spec ID: `CYNAI.SBAGNT.FailureCodes` <a id="spec-cynai-sbagnt-failurecodes"></a>

The SBA MUST set `failure_code` to one of the following when applicable:

- **`schema_validation`** - Job spec failed validation (unknown fields, missing required fields, invalid values).
  Validation MUST occur before any step runs.
- **`ext_net_required`** - Failure due to lack of external network access when required (e.g. dependency download failed, package fetch blocked).
  The SBA MUST use this when the job failed because external network was needed but not allowed or was denied by policy.
- **`step_failed`** - One or more steps failed during execution (non-zero exit, exception, or step-level error).
- **`timeout`** - Job or step exceeded allowed runtime.
  May coincide with `status: "timeout"`.
- **`constraint_violation`** - A job constraint was exceeded (e.g. `max_output_bytes` exceeded, output cap hit).

For other failure categories the SBA MAY use an implementation-defined code (documented and stable).

- **`failure_message`** (optional): Human-readable explanation for the failure (e.g. "Unable to download dependencies; external network access was not allowed for this job or was denied by policy" for `ext_net_required`).
  The orchestrator uses this to surface actionable feedback and to attribute failures in audits.

## Sandbox Boundary and Security

- Spec ID: `CYNAI.SBAGNT.SandboxBoundary` <a id="spec-cynai-sbagnt-sandboxboundary"></a>

### Shape of Sandboxed Containers

Sandboxed containers that run `cynode-sba` have a well-defined shape:

- **Filesystem**: Writable **`/workspace`** (working directory; full access for the agent).
  **`/job`** holds job input and output: e.g. `job.json`, `result.json`, `artifacts/`.
  **`/tmp`** for temporary files.
  The process runs as a **non-root** user; no command or path allowlists are required inside the container.
- **Control**: No inbound network control plane.
  The node controls the container via the container runtime (start, exec, stop).
  The SBA MUST NOT rely on inbound connectivity for control.
- **Network (not airgapped)**: Sandboxes are **not** truly airgapped.
  When outbound traffic is allowed by policy, it is allowed **only via worker proxies**.
  Direct outbound access to the internet or other services is not permitted.
  The node enforces strict egress control: inference goes through the node-local inference proxy (or orchestrator API Egress); HTTP/HTTPS goes through the node-local web egress proxy to the orchestrator's Web Egress Proxy (allowlisted destinations only).
  There are strict controls on what is allowed in and out: no arbitrary inbound, and outbound only through mediated, allowlisted proxy paths.
  When policy does not allow egress, outbound may be fully blocked.

### Security Constraints

- `cynode-sba` MUST NOT handle secrets.
- The SBA MUST assume outbound network may be blocked (when policy disallows egress).
  When egress is allowed, it MUST use only the proxy endpoints injected by the node (see [Worker Proxies (Inference and Web Egress)](#worker-proxies-inference-and-web-egress)).

See:

- [`docs/requirements/sandbx.md`](../requirements/sandbx.md)
- [`docs/tech_specs/sandbox_container.md`](sandbox_container.md)

## MCP Tool Access (Sandbox Allowlist)

- Spec ID: `CYNAI.SBAGNT.McpToolAccess` <a id="spec-cynai-sbagnt-mcptoolaccess"></a>

Sandbox agents (including `cynode-sba` when operating as an agent that can call tools) MUST use MCP tools only through the **orchestrator MCP gateway** and MUST invoke only tools that are on the **sandbox (worker) allowlist** and designated as available to sandbox agents in the orchestrator's per-tool scope (see [Per-tool scope: Sandbox vs PM](mcp_gateway_enforcement.md#spec-cynai-mcpgat-pertoolscope)).
When making MCP requests, a sandbox agent MUST present an **agent-scoped token or API key** issued by the orchestrator for that sandbox context (e.g. task_id, job_id, user); the gateway authenticates the token and restricts tool access to the sandbox allowlist and sandbox-scoped tools.
See [Agent-Scoped Tokens or API Keys](mcp_gateway_enforcement.md#spec-cynai-mcpgat-agentscopedtokens).

Sandbox allowlist (built-in)

- The gateway MUST allow sandbox agents to invoke only tools in the [Worker Agent allowlist](mcp_gateway_enforcement.md#spec-cynai-mcpgat-workeragentallowlist): at least `artifact.*` (scoped to current task), including `artifact.put` for uploading attachments, and `artifact.get`, `artifact.list`; `skills.list` and `skills.get` (read-only skill fetch when allowed by policy); `web.fetch`, `web.search`, `api.call` (via API Egress when allowed for the task); and `help.*`.
  The gateway MUST NOT allow sandbox agents to invoke `db.*`, `node.*`, or `sandbox.*`.
  Sandbox agents MUST NOT invoke `skills.create`, `skills.update`, or `skills.delete`; only read tools are on the worker allowlist.

User-installed tools

- User-installed (custom) MCP tools that are configured with sandbox scope MUST be added to the set of tools a sandbox agent may invoke, subject to per-tool enable/disable and access control.
  The implementation MUST use the orchestrator's per-tool sandbox vs PM setting to decide whether a sandbox agent may call a given tool.

See:

- [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)
- [`docs/tech_specs/mcp_tool_catalog.md`](mcp_tool_catalog.md)

## SBA Container Image (Containerfile)

The definitive spec for the SBA runner container image (minimum requirements, recommendations, and Containerfile guidance) is in [sandbox_container.md - SBA Runner Image (Containerfile)](sandbox_container.md#spec-cynai-sbagnt-sbacontainerimage).
Implementations and third-party SBA-compatible images MUST satisfy that spec.

## Protocol Versioning

- Spec ID: `CYNAI.SBAGNT.ProtocolVersioning` <a id="spec-cynai-sbagnt-protocolversioning"></a>

- `protocol_version` MUST be validated at startup.
- Unknown major versions MUST be refused.
- Minor versions MAY add backward-compatible fields.
