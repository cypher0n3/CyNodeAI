# CyNode SandBox Agent (`cynode-sba`)

- [Document Overview](#document-overview)
- [Purpose](#purpose)
- [Design Principles](#design-principles)
- [SBA Capabilities (What the Agent Can Call and How)](#sba-capabilities-what-the-agent-can-call-and-how)
  - [Local Execution (Inside the Container)](#local-execution-inside-the-container)
  - [Outbound Channels (Worker Proxies Only)](#outbound-channels-worker-proxies-only)
  - [Job Lifecycle Reporting (What the SBA Must Call)](#job-lifecycle-reporting-what-the-sba-must-call)
  - [MCP Tools Available to the SBA](#mcp-tools-available-to-the-sba)
- [Execution Model](#execution-model)
  - [Todo List](#todo-list)
- [Integration With Worker API](#integration-with-worker-api)
  - [Job Lifecycle and Status Reporting](#job-lifecycle-and-status-reporting)
  - [Worker Proxies (Inference and Web Egress)](#worker-proxies-inference-and-web-egress)
- [Job Specification](#job-specification)
  - [Inference Models (Job-Defined Allowlist)](#inference-models-job-defined-allowlist)
  - [Persona on the Job](#persona-on-the-job)
  - [Context Supplied to SBA (Requirements, Acceptance Criteria, Preferences, Skills)](#context-supplied-to-sba-requirements-acceptance-criteria-preferences-skills)
  - [SBA LLM Prompt Construction](#sba-llm-prompt-construction)
- [Local Tools (MVP)](#local-tools-mvp)
  - [Tool Argument Schemas and Common Use Cases](#tool-argument-schemas-and-common-use-cases)
- [Result Contract](#result-contract)
  - [Task Result Consumption](#task-result-consumption)
  - [Canonical Failure Codes](#canonical-failure-codes)
- [Sandbox Boundary and Security](#sandbox-boundary-and-security)
  - [Shape of Sandboxed Containers](#shape-of-sandboxed-containers)
  - [Security Constraints](#security-constraints)
- [MCP Tool Access (Sandbox Allowlist)](#mcp-tool-access-sandbox-allowlist)
- [SBA Container Image (Containerfile)](#sba-container-image-containerfile)
- [Go Implementation](#go-implementation)
- [Protocol Versioning](#protocol-versioning)

## Document Overview

- Spec ID: `CYNAI.SBAGNT.Doc.CyNodeSba` <a id="spec-cynai-sbagnt-doc-cynodesba"></a>
- Traces To: [REQ-SBAGNT-0001](../requirements/sbagnt.md#req-sbagnt-0001)

This document defines `cynode-sba`, the sandbox agent runner binary.
It is a full AI agent that runs inside a sandbox container, uses inference (LLM) to decide what to do, and invokes local and MCP tools; see [Sandbox Boundary and Security](#sandbox-boundary-and-security) for container shape and the network model (egress only via worker proxies, not airgapped).

This spec is derived from the draft runner design in:

- Draft runner design: `docs/draft_specs/cynode-agent_rough_spec.md` (not in repo)

This spec aligns with:

- [`docs/tech_specs/sandbox_container.md`](sandbox_container.md)
- [`docs/tech_specs/worker_api.md`](worker_api.md)

Abbreviation note: This doc may abbreviate "SandBox Agent" to "SBA" throughout.

## Purpose

`cynode-sba` is a full AI agent that performs work according to a job specification.
It is not an LLM itself; it MUST have access to at least one model (via worker proxy or API Egress) and uses inference (using only models the job allows) to plan and decide which tools to call.
It does not decide policy or scheduling; the orchestrator and worker-node components are responsible for policy and sandbox lifecycle.
Within the job, the SBA MUST be able to build and manage its own todo list (derived from requirements, acceptance criteria, and any suggested steps in the job) to track and drive progress.

## Design Principles

- Small attack surface.
- Strict schema adherence (unknown job fields rejected; validation before the agent is started).
- **No command or path allowlists inside the container.**
  The sandbox agent runs in an already-sandboxed environment (the container).
  It does not need strict allowlists for commands or paths; it MAY run any **user-level** command on the system and MUST have full access to the **`/workspace`** directory.
  The process MUST NOT run as root.
- Fail-closed on **schema validation** only (invalid job spec is rejected; the agent does not start).
  Runtime enforcement is bounded by the container and non-root execution, not by per-command or per-path allowlists.
  The orchestrator or worker MAY use command or path allowlists when constructing or validating job requests; the SBA does not enforce them inside the container.
- Structured, machine-parseable results.

## SBA Capabilities (What the Agent Can Call and How)

- Spec ID: `CYNAI.SBAGNT.Capabilities` <a id="spec-cynai-sbagnt-capabilities"></a>

Traces To:

- [REQ-SBAGNT-0001](../requirements/sbagnt.md#req-sbagnt-0001)
- [REQ-SBAGNT-0112](../requirements/sbagnt.md#req-sbagnt-0112)

This section summarizes what the SBA can invoke and the mechanisms used.
All outbound traffic from the sandbox goes through worker proxies or orchestrator-mediated endpoints.
See [Worker Proxies (Inference and Web Egress)](#worker-proxies-inference-and-web-egress) and [Sandbox Boundary and Security](#sandbox-boundary-and-security).

### Local Execution (Inside the Container)

- **Arbitrary command execution:** The SBA MAY run any **user-level** command (no root).
  There are **no command or path allowlists** inside the container; enforcement is the container boundary and non-root process.
  See [Design Principles](#design-principles) and [Local Tools (MVP)](#local-tools-mvp).
- **Local tools:** The SBA may invoke the following local tools (via inference-driven tool calls): `run_command`, `write_file`, `read_file`, `apply_unified_diff`, `list_tree`, `search_files`.
  Working directory and file access are under `/workspace` (full access) or as specified per tool call; symlink escape outside `/workspace` is rejected.
- **Filesystem:** Full read/write under `/workspace`; `/job/` for job input, result staging, and artifacts; `/tmp` for temporary files.

### Outbound Channels (Worker Proxies Only)

The SBA has **no direct internet or host access**.
All outbound use is via:

- **Lifecycle / status** - Report job in-progress, completion, result, timeout extension.
  Outbound HTTP to orchestrator job callback URL or job-status endpoint; URLs injected by node (e.g. env or job payload).
  See [Job Lifecycle and Status Reporting](#job-lifecycle-and-status-reporting).
- **Inference** - LLM calls for planning, tool use, refinement.
  Node-local inference proxy (`OLLAMA_BASE_URL`) or orchestrator-mediated API Egress endpoint; only models in job `inference.allowed_models`.
- **MCP gateway** - Tools: artifacts, memory, skills, web, API egress.
  SBA calls orchestrator MCP gateway with agent-scoped token; only [sandbox allowlist](#mcp-tool-access-sandbox-allowlist) tools.
  Traffic goes through worker proxy.
- **Web egress** - Outbound HTTP/HTTPS (e.g. package installs, fetches).
  When `constraints.ext_net_allowed` is true, node sets `HTTP_PROXY`/`HTTPS_PROXY` to node-local web egress proxy; proxy forwards to orchestrator Web Egress Proxy (allowlisted destinations only).
- **API Egress** - External APIs (e.g. GitHub, Slack) without credentials in sandbox.
  SBA invokes MCP tool `api.call`; gateway routes to API Egress Server; credentials stay in orchestrator.
  Allowed only when task/job policy and (for external destination) `ext_net_allowed` permit.
  See [API Egress Server](api_egress_server.md).

### Job Lifecycle Reporting (What the SBA Must Call)

- **In progress:** After validating the job spec, the SBA MUST signal in progress via outbound call through the worker proxy (e.g. to orchestrator job-status endpoint or callback URL).
- **Completion:** On success, failure, or timeout, the SBA MUST report completion via outbound call to deliver the [Result contract](#result-contract) (and optionally artifact references or inline data).
- **Artifacts:** The SBA MAY upload attachments via MCP `artifact.put` (task-scoped) or stage files under `/job/artifacts/` for node-mediated delivery.
- **Timeout extension:** The SBA MUST be able to request a time extension (e.g. via job-status callback or dedicated endpoint) up to the node maximum; remaining time or deadline MUST be available to the SBA for LLM context.
  The exact mechanism (callback payload, MCP tool, or status API) is defined in the [Worker API](worker_api.md) and/or MCP tool catalog.

### MCP Tools Available to the SBA

The SBA MAY invoke only tools on the [Worker Agent allowlist](mcp_gateway_enforcement.md#spec-cynai-mcpgat-workeragentallowlist) with sandbox (or both) scope:

- **artifact.*** - `artifact.put`, `artifact.get`, `artifact.list` (task-scoped).
- **memory.*** - `memory.add`, `memory.list`, `memory.retrieve`, `memory.delete` (job-scoped; see [Temporary Memory](#spec-cynai-sbagnt-temporarymemory)).
- **skills.list**, **skills.get** - Read-only skill fetch when allowed by policy.
- **web.fetch** - Sanitized fetch when allowed by policy (e.g. Secure Browser Service).
- **web.search** - Secure web search when allowed by policy.
- **api.call** - Via API Egress when explicitly allowed for the task; credentials never in sandbox.
- **help.*** - On-demand docs (optional for worker).

Explicitly disallowed: `db.*`, `node.*`, `sandbox.*`.
User-installed tools with sandbox scope MAY be added per [MCP Tool Access](#mcp-tool-access-sandbox-allowlist).

See [mcp_tool_catalog.md](mcp_tool_catalog.md) and [mcp_gateway_enforcement.md](mcp_gateway_enforcement.md).

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
When the job uses an image that runs `cynode-sba` as the main process (the SBA runner image), the node starts the container; `cynode-sba` reads the job spec from the agreed location (e.g. `/job/job.json`), runs as an AI agent (using inference and tools), and writes the [Result contract](#result-contract) (e.g. `/job/result.json`).

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
For synchronous Worker API implementations, node-mediated delivery typically means the node reads `/job/result.json` and `/job/artifacts/` after the container exits and returns their contents in the same HTTP response to the orchestrator; the orchestrator then persists the result and artifacts (see [Worker API - Node-Mediated SBA Result (Sync)](worker_api.md#spec-cynai-worker-nodemediatedsbaresult-sync)).
A container-internal proxy (e.g. a sidecar that SBA POSTs to, which then forwards to the node or orchestrator) is not currently specified and is not required for MVP.

#### In-Progress State

After the SBA has read and validated the job spec (and before or as it begins work), the SBA MUST confirm acceptance and signal that the job is **in progress**.
The SBA MUST do so via outbound call through the worker proxy (e.g. to an orchestrator job-status endpoint) and MAY additionally write a status file under `/job/` or use another implementation-defined signal.
The node MAY also infer in-progress when the SBA has read the job and not yet exited; the orchestrator MUST be able to update job state to in-progress.

#### Completion State

When the job finishes (success, failure, or timeout), the SBA MUST report completion by making an outbound call through the worker proxy to deliver the [Result contract](#result-contract) (and optionally artifact references or inline artifact data) to the orchestrator.
The SBA MAY also write the result to the agreed location (e.g. `/job/result.json`) for staging or node-mediated delivery; if so, the node MUST NOT clear or delete the job result until the result has been successfully persisted to the orchestrator (e.g. the node uploads from `/job/` or the SBA has already reported via proxy).
The orchestrator MUST pass job completion (status and result) to the Project Manager Agent and/or Project Analyst Agent for additional work (e.g. verification, remediation); see [Orchestrator - Task Scheduler](orchestrator.md#spec-cynai-orches-scheduledrunrouting).

See [Worker API - Job lifecycle and result persistence](worker_api.md#spec-cynai-worker-joblifecycleresultpersistence).

#### Timeout Extension

- Spec ID: `CYNAI.SBAGNT.TimeoutExtension` <a id="spec-cynai-sbagnt-timeoutextension"></a>

The SBA MUST be able to **request a time extension** for the current job (e.g. via job-status callback or a dedicated extension endpoint), up to the **node maximum** job timeout.
The orchestrator or node MAY grant or deny the request.
When granted, the job's effective deadline is extended; the node (or orchestrator) MUST enforce the new deadline.
The mechanism (e.g. MCP tool or field on the job-status callback) is defined in the Worker API and/or MCP tool catalog.

#### Time Remaining and LLM Context

- Spec ID: `CYNAI.SBAGNT.TimeRemaining` <a id="spec-cynai-sbagnt-timeremaining"></a>

The job context supplied to the SBA (or an endpoint/tool the SBA can call) MUST provide **remaining time** or an absolute **deadline** so the SBA can track it internally.
The SBA MUST be able to inject "you have X time left to complete this task" (or equivalent) into LLM prompts or step context so the agent can pace work and avoid running out of time without warning.
The job spec or runtime-injected context SHOULD include `deadline` or `remaining_seconds` (or both), updated when an extension is granted.

#### Temporary Memory (Job-Scoped)

- Spec ID: `CYNAI.SBAGNT.TemporaryMemory` <a id="spec-cynai-sbagnt-temporarymemory"></a>

The SBA MUST have a method to **store and retrieve temporary memories** during job processing, scoped to the task/job (e.g. MCP tools `memory.add`, `memory.list`, `memory.retrieve`, and `memory.delete` per [MCP tool catalog - Memory tools](mcp_tool_catalog.md#spec-cynai-mcptoo-memorytoolsjobscoped)), so it can persist working state across steps and LLM calls.
These memories are **job-scoped** (or task-scoped) and MUST NOT persist beyond the job (or task) unless explicitly promoted to artifacts or long-term storage.
Size limits and retention (e.g. max entries, max size per entry, TTL = job lifetime) are defined in the MCP tool catalog and enforced by the gateway.
The [Worker Agent allowlist](mcp_gateway_enforcement.md#spec-cynai-mcpgat-workeragentallowlist) MUST include these memory tools for sandbox-scoped use.

### Worker Proxies (Inference and Web Egress)

- Spec ID: `CYNAI.SBAGNT.WorkerProxies` <a id="spec-cynai-sbagnt-workerproxies"></a>

When `cynode-sba` runs inside a worker-node sandbox, the **node** provides proxy endpoints for inference and (when policy allows) outbound HTTP.
The SBA does not configure or discover these endpoints itself; it relies on the Node Manager to inject the appropriate environment and pod setup so that steps and tooling inside the sandbox use the worker proxies.

Inference proxy

- When the node provides Ollama inference, the Node Manager runs the sandbox in a pod that includes an **inference proxy sidecar**.
  The proxy listens on `localhost:11434` inside the pod; the node injects `OLLAMA_BASE_URL=http://localhost:11434` into the sandbox container environment.
  Any agent use of inference (e.g. LLM calls from within the sandbox) MUST use this endpoint; the proxy forwards to the node's Ollama container and keeps traffic node-local.

Web egress proxy

- When the sandbox network policy allows allowlisted egress, the node configures the sandbox to use the **node-local web egress proxy** via standard proxy environment variables (`HTTP_PROXY`, `HTTPS_PROXY`, and optionally `NO_PROXY`).
  The node-local proxy forwards requests to the orchestrator's Web Egress Proxy; only allowlisted destinations are permitted.
  Tool calls that perform outbound HTTP (e.g. `run_command` with `pip install`, `curl`, package fetches) use these env vars; the SBA does not set or override proxy URLs.

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
Validation MUST occur before the agent is started.

Minimum required fields

- `protocol_version`, `job_id`, `task_id`, `constraints`.
- `steps` is **optional**.
  When present, it is used as **recommended to-dos** for the agent (e.g. merged into the todo list or shown as suggested steps); the SBA uses inference to decide what to do and is not required to execute these steps in order or at all.
  When absent or empty, the SBA has no suggested steps and plans entirely from context (requirements, acceptance criteria, etc.).
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

### Persona on the Job

- Spec ID: `CYNAI.SBAGNT.JobPersona` <a id="spec-cynai-sbagnt-jobpersona"></a>

**Terminology:** The personas in this spec are **Agent personas**: named, reusable descriptions of how an agent (here, the SBA) should behave (role, identity, tone).
They are not to be confused with customer personas, end-user personas, or product UX personas.

The job spec carries the Agent persona **inline only**.
The SBA never receives a persona reference; it always receives the full persona object in the JSON.

- **Inline in JSON:** Top-level optional object `persona` with `title` (string) and `description` (string).
  The description MUST be short prose in the form "You are a [role] with [background] and [supporting details]." (or equivalent).
  Agent personas are stored in the database and are queriable by all agents (PMA, PAA, orchestrator job builder) via the User API Gateway or MCP; when building a job, the builder looks up the chosen Agent persona by id (or by title with scope precedence) and **embeds** `title` and `description` inline into the job spec.
  When multiple Agent personas match (e.g. same title in different scopes), the builder MUST apply the **most specific** that matches: user scope over project over group over system (global); see [project_manager_agent.md - Persona assignment and resolution](project_manager_agent.md#spec-cynai-agents-personaassignment).
- **Optional provenance:** `persona_id` (uuid) MAY be present in the job JSON for auditing; the SBA ignores it and uses only `persona.title` and `persona.description`.
- When **persona is present**, the SBA MUST use it as the first context block in every LLM prompt per [SBA LLM Prompt Construction](#spec-cynai-sbagnt-llmpromptconstruction).
  When **persona is absent**, the SBA uses only baseline context and other job context without a persona block.

Agent persona model (stored in DB, embedded at job-build time):

- **Agent persona:** A named, reusable description of how the sandbox agent should behave (role, identity, tone).
  - **Persona title** (required): Short human-readable label (e.g. "Backend Developer", "Security Reviewer").
  - **Persona description** (required): **Short** prose in the form "You are a [role] with [background] and [supporting details]."
  Agent personas MUST be kept concise.
- **Global default Agent personas:** The system MUST provide a small set of system-scoped (global) default Agent personas, seeded at bootstrap.
  This set MUST include dedicated Agent personas for the Project Manager Agent (PMA) and the Project Analyst Agent (PAA), which PMA and PAA MUST always use for their own identity/role when running; see [project_manager_agent.md](project_manager_agent.md).
  Other global defaults (e.g. "Backend Developer", "Security Reviewer", "Code Reviewer") MAY be included so SBAs can be assigned common roles without per-user configuration.
- **Agent personas vs skills:** Persona defines *who* the agent is and is the first context block; skill is procedural guidance supplied separately (e.g. `context.skills` or MCP) and appears later in the prompt.
  An Agent persona MAY hint at or recommend which skills to use; the actual skill content is supplied via job context or MCP, not embedded in the persona.

### Context Supplied to SBA (Requirements, Acceptance Criteria, Preferences, Skills)

- Spec ID: `CYNAI.SBAGNT.JobContext` <a id="spec-cynai-sbagnt-jobcontext"></a>

Traces To:

- [REQ-SBAGNT-0107](../requirements/sbagnt.md#req-sbagnt-0107)
- [REQ-SBAGNT-0111](../requirements/sbagnt.md#req-sbagnt-0111)
- [REQ-SBAGNT-0113](../requirements/sbagnt.md#req-sbagnt-0113)
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
  "persona": {
    "title": "Backend Developer",
    "description": "You are a backend developer with experience in Go and APIs and a focus on clarity and testability. You prefer small, reviewable changes and explicit error handling."
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

### SBA LLM Prompt Construction

- Spec ID: `CYNAI.SBAGNT.LlmPromptConstruction` <a id="spec-cynai-sbagnt-llmpromptconstruction"></a>

Traces To:

- [REQ-SBAGNT-0113](../requirements/sbagnt.md#req-sbagnt-0113)

**Purpose:** Define the content and order of context sent to the LLM on each request (system message and/or user message), and how tools are presented.

The SBA MUST build each LLM prompt by including the following **in this order**:

1. **Agent persona (if present in job)** - The short persona description from the job's inline `persona.description` (see [Persona on the Job](#spec-cynai-sbagnt-jobpersona)).
   MUST be the first context block so the model adopts the role before other instructions.
   Agent personas may hint at or recommend skills; the actual skill content is supplied in step 8.
2. **Baseline context** - Identity, role, responsibilities, and non-goals (from `context.baseline_context` or image-baked baseline).
   MUST be included in every LLM prompt.
3. **Project-level context** - When the job is project-scoped: project identity (id, name, slug), scope, relevant metadata.
   MUST be included when present in the job.
4. **Task-level context** - Task identity (id, name), acceptance criteria summary, status, relevant task metadata.
   MUST be included.
5. **Requirements and acceptance criteria** - From the job context.
6. **Relevant preferences** - Resolved user/task preferences that affect how work is done.
7. **User-configurable additional context** - From `agents.sandbox_agent.additional_context` (resolved at job creation).
8. **Skills (if supplied or fetched)** - Inline skill content or references; the SBA MAY fetch skills via MCP and append here.
   Skills are distinct from Agent personas; the persona may recommend which skills to use, but skill content is supplied here.
9. **Runtime context for this turn** - Remaining time or deadline; current todo list or step progress; and the current user/tool turn.

The implementation MUST concatenate or structure these blocks in a deterministic way (e.g. clear section headers or role/system vs user message split) so that the model receives a consistent, ordered context.

#### Tools Presented to the LLM

- **Local tools** (`run_command`, `write_file`, `read_file`, `apply_unified_diff`, `list_tree`, `search_files`): The SBA MUST present each tool with name, a short non-ambiguous description, and a structured arguments schema (e.g. JSON Schema) including types and constraints.
- **MCP tools** (sandbox allowlist): When the SBA wraps MCP tools as langchaingo (or equivalent) tools, each tool MUST be presented with name, description from the MCP catalog, and arguments schema; the SBA MUST inject `task_id` and `job_id` from the job spec so the LLM is not required to supply them, or the schema MUST document them as required.
- **Order and grouping:** Order MUST be deterministic for a given job (e.g. local tools first, then MCP tools).
- **No credentials or internal URLs:** Tool descriptions and schemas MUST NOT include secrets, bearer tokens, or internal callback URLs; those are injected by the runtime, not into the tool schema visible to the LLM.

## Local Tools (MVP)

- Spec ID: `CYNAI.SBAGNT.Enforcement` <a id="spec-cynai-sbagnt-enforcement"></a>

The SBA is an AI agent that uses inference to decide what to do; it may invoke the following local tools.
When the agent calls a tool, the runner executes that tool as a **non-root** user with **full access to `/workspace`**; there are no command allowlists or path allowlists inside the container.
Tool results are capped and reported in the result contract; the agent continues until it concludes or hits constraints.

- `run_command`
  - Runs a command (argv form; no shell interpretation unless the tool explicitly requests it).
    Executed as the same non-root user; may run any user-level command with full access to `/workspace`.
    Working directory is under `/workspace` or as specified in the tool args.
- `write_file`
  - Writes a file under `/workspace` (or a path relative to workspace).
    Rejects symlink escape outside workspace.
- `read_file`
  - Reads a file under `/workspace` (or a path relative to workspace) with a hard size cap from `constraints.max_output_bytes` or tool-level cap.
- `apply_unified_diff`
  - Applies a unified diff relative to the workspace root.
    Rejects patches that would write outside `/workspace`.
- `list_tree`
  - Returns a structured tree representation (not raw shell output) for paths under `/workspace`.
- `search_files`
  - Search for a pattern in files under `/workspace`; returns matching lines (path:line:content) with output capped by `max_output_bytes`.
    Does not depend on grep/rg in the image.

### Tool Argument Schemas and Common Use Cases

- Spec ID: `CYNAI.SBAGNT.StepTypeSchemas` <a id="spec-cynai-sbagnt-steptypeschemas"></a>

All tool paths MUST be under `/workspace` (or the tool-specified root); resolution MUST reject symlink escape outside workspace.
Output from tools that return content (e.g. `run_command`, `read_file`, `list_tree`, `search_files`) MUST be capped by `constraints.max_output_bytes` or a tool-level cap; truncation MUST be indicated in the tool result (e.g. suffix or status).

#### Run_command_command Step

- Required args: `argv` (array of strings; non-empty).
  Optional args: `cwd` (string; path relative to workspace; default workspace root).
- Use for arbitrary user-level commands, including: **grep/search in files** (e.g. `grep`, `rg` when present in the image), **find/glob** (e.g. `find`, shell globs), **head/sed** for line-oriented edits, and builds/tests.
  Command stdout+stderr are combined and capped; exit code is reflected in the step result.

#### Read_file_file Step

- Required args: `path` (string; file path relative to workspace).
  Optional args: `start_line`, `end_line` (integers, 1-based inclusive; when both present, only that line range is read; implementations MAY support this to avoid reading large files).
- When line range is not supported or not provided, the full file is read.
  Output MUST be capped; out-of-range or missing file MUST produce a deterministic error in the tool result.

#### Write_file_file Step

- Required args: `path` (string), `content` (string).
  Parent directories MUST be created as needed.
  Rejects path that escapes workspace (symlink or traversal).

#### Apply_unified_diff_unified_diff Step

- Required args: `diff` (string; unified diff body).
  Paths in the diff (e.g. from `---`/`+++` lines) MUST resolve under workspace; any path that would write outside workspace MUST be rejected before applying.
  Patch is applied relative to workspace root (e.g. `patch -p1 -d workspace --forward`).
  Optional arg `dry_run` (boolean) is reserved for future use (validate only, do not apply).
- Use for **patching files**: generate a unified diff (e.g. from a model or tool) and apply it with this step.

#### List_tree_tree Step

- Optional args: `path` (string; directory under workspace; default workspace root).
  Returns a structured tree (e.g. one path per line) for that directory.
  Optional arg `pattern` or `glob` is reserved for future use (filter entries by pattern).
- For **glob/find by pattern**, use `run_command` with `find` or shell glob until `list_tree` supports a pattern.

#### Search_files_files Step

- Required args: `pattern` (string; regular expression to search for; RE2 syntax).
  Optional args: `path` (string; directory under workspace; default workspace root), `include` (string; glob to restrict files, e.g. `*.go`; when empty, all readable files are considered).
- Walks the directory tree under `path`, considers only files matching `include` (if set), and reports each line that matches `pattern` as `relpath:line_num:line_content`.
  Output MUST be capped by `constraints.max_output_bytes`; truncation MUST be indicated (e.g. suffix).
  Paths MUST remain under workspace; symlink escape is rejected.
  Implementation MUST NOT depend on external grep/ripgrep binaries.

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

### Task Result Consumption

When asserting or validating the SBA result contract from a task result (e.g. gateway task result API or CLI task result), the consumer MUST only interpret or validate the `sba_result` shape when the task status is `completed`.
For task status `failed`, `canceled`, or `superseded`, the job result and any `sba_result` field may be partial or absent; contract validation is undefined.

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
When making MCP requests, the sandbox agent calls the **worker proxy** (e.g. worker-mediated MCP URL); the agent MUST NOT receive or present an agent token.
The **worker proxy** holds the sandbox agent token (issued by the orchestrator for that sandbox context, delivered to the worker) and attaches it when forwarding requests to the orchestrator MCP gateway; the token MUST be bound to task_id, project_id, and session scope, and MUST be associated with the user (e.g. task creator).
See [Agent-Scoped Tokens or API Keys](mcp_gateway_enforcement.md#spec-cynai-mcpgat-agentscopedtokens).
The gateway authenticates the token and restricts tool access to the sandbox allowlist and sandbox-scoped tools.

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

## Go Implementation

- Spec ID: `CYNAI.SBAGNT.Implementation` <a id="spec-cynai-sbagnt-implementation"></a>

The **canonical implementation** of `cynode-sba` is in **Go** using the [langchaingo](https://github.com/tmc/langchaingo) library.

Langchaingo usage:

- **LLM calls:** Via the `llms` package with provider and endpoint set from job/runtime (e.g. Ollama base URL for worker proxy, or OpenAI-compatible endpoint for API Egress).
- **Agent loop:** Via the `agents` package (e.g. ReAct/MRKL-style or tool-calling executor) for the decide-next-step / call-tool / interpret-result loop.
- **Tools:** MCP tools on the sandbox allowlist are invoked through the orchestrator MCP gateway.
  The implementation wraps MCP calls as langchaingo tools so the agent uses the existing gateway contract.

All contract requirements in this document (job spec, result contract, Worker API integration, MCP tool access, security constraints) are unchanged; only the implementation stack is specified above.

## Protocol Versioning

- Spec ID: `CYNAI.SBAGNT.ProtocolVersioning` <a id="spec-cynai-sbagnt-protocolversioning"></a>

- `protocol_version` MUST be validated at startup.
- Unknown major versions MUST be refused.
- Minor versions MAY add backward-compatible fields.
