# MVP Plan

- [Overview](#overview)
  - [MVP Objectives](#mvp-objectives)
- [Tech Spec Alignment](#tech-spec-alignment)
- [Phase Summary](#phase-summary)
- [Scope Summary](#scope-summary)
- [Current Status](#current-status)
- [Prompt Interpretation: Intended Semantics](#prompt-interpretation-intended-semantics)
  - [Actionable Sequence](#actionable-sequence)
- [Task Breakdown (4-8 Hour Chunks)](#task-breakdown-4-8-hour-chunks)
  - [Phase 1 Gap Closure (Spec Alignment)](#phase-1-gap-closure-spec-alignment)
  - [Phase 1.5 Prompt Interpretation (Make Prompts Behave Like Prompts)](#phase-15-prompt-interpretation-make-prompts-behave-like-prompts)
  - [Phase 1.7 Agent Artifacts (PMA First, Then SBA)](#phase-17-agent-artifacts-pma-first-then-sba)
  - [Phase 2 MCP in the Loop (Tool Enforcement and Auditing)](#phase-2-mcp-in-the-loop-tool-enforcement-and-auditing)
  - [Phase 3 Multi Node Robustness (Scheduling, Reliability, Telemetry)](#phase-3-multi-node-robustness-scheduling-reliability-telemetry)
  - [Phase 4 API Egress and External Routing (Controlled External Calls)](#phase-4-api-egress-and-external-routing-controlled-external-calls)
  - [Phase 5 (Not Defined in Current Tech Specs)](#phase-5-not-defined-in-current-tech-specs)
- [Phase 0 Foundations](#phase-0-foundations)
- [Phase 1 Single Node Happy Path](#phase-1-single-node-happy-path)
- [Phase 1.5 Single Node Full Capability](#phase-15-single-node-full-capability)
- [Phase 2 MCP in the Loop](#phase-2-mcp-in-the-loop)
- [Phase 3 Multi Node Robustness](#phase-3-multi-node-robustness)
- [Phase 4 Optional Controlled Egress and Integrations](#phase-4-optional-controlled-egress-and-integrations)
- [Feature Files and BDD](#feature-files-and-bdd)
- [Unit Tests and Coverage](#unit-tests-and-coverage)
- [Implementation Order (Done vs Remaining)](#implementation-order-done-vs-remaining)
  - [Done (Completed Items)](#done-completed-items)
  - [Remaining (Order)](#remaining-order)
- [References](#references)

## Overview

This document is the **canonical full MVP development plan** for CyNodeAI.
All task breakdowns, requirement and spec references, and implementation order are maintained here.

This document is the single comprehensive MVP development plan as of 2026-02-22.
It reflects the latest tech specs (including [external_model_routing.md](../docs/tech_specs/external_model_routing.md)) and post-Phase 1 implementation status.

The canonical phase list and high-level scope remain in [docs/tech_specs/\_main.md](../docs/tech_specs/_main.md); this plan adds objectives, current status, spec alignment, task breakdown with req/spec refs, and quality/BDD expectations.

### MVP Objectives

1. **Full single-node execution with inference:** Orchestrator dispatches work that runs in sandboxed containers and uses node-local inference (Ollama) from inside the sandbox, per [worker_node.md](../docs/tech_specs/worker_node.md) and [sandbox_container.md](../docs/tech_specs/sandbox_container.md).
2. **Feature files** built out for new behavior and any Phase 1 gaps from code review.
3. **Unit tests** maintain or achieve at least 90% code coverage for all touched packages (existing justfile rule).
4. **BDD suite** (orchestrator, worker_node, cynork) covers inference-in-sandbox and prompt interpretation; remains runnable via `just test-bdd`.
5. **CLI app (cynork)** as separate Go module, runnable against the orchestrator user-gateway on localhost, with basic auth and task operations.
6. **Prompt interpretation:** Natural-language task prompts are interpreted by the system (model and/or sandbox), not executed as literal shell commands (see [Prompt interpretation: intended semantics](#prompt-interpretation-intended-semantics)).

## Tech Spec Alignment

Recent tech spec updates incorporated into this plan:

- **External model routing** ([external_model_routing.md](../docs/tech_specs/external_model_routing.md)):
  - Routing goal: prefer local execution; MUST support configured external APIs when policy allows; MUST refuse to enter a ready state when no inference-capable path (local or external) is available.
  - Routing signals: capability match, worker load, data locality, task constraints, user override, policy.
  - Routing policy: attempt local first; route to external when no capable worker, overload, standalone mode, or task marked external-allowed; deny when policy disallows; record decision and reasons.
  - External provider integration: all external model calls via API Egress Server; orchestrator sends provider/operation/params/task_id; API Egress uses user-scoped credential and returns result; orchestrator stores result in PostgreSQL.
  - External inference with node sandboxes: orchestrator MAY use external model for inference while dispatching sandbox jobs to nodes for tools; sandboxes MUST NOT receive provider keys; sandbox-only nodes allowed for tool execution.
  - Settings: suggested keys for global routing (e.g. `model_routing.prefer_local`, `model_routing.allowed_external_providers`, overload thresholds, max tokens/cost) and per-agent keys for Project Manager and Project Analyst.
  - Auditing: log routing decisions and API Egress outbound calls with task context and identity.

- **Sandbox image registry** ([sandbox_image_registry.md](../docs/tech_specs/sandbox_image_registry.md), [postgres_schema.md](../docs/tech_specs/postgres_schema.md)):
  - Schema tables (`sandbox_images`, `sandbox_image_versions`, `node_sandbox_image_availability`) are in scope for MVP so the schema can be created and upgraded.
  - Full registry behavior (rank-ordered registries, allowed images, node pull workflow, publish workflow) is **deferred until after MVP**; implementation of registry flows is out of scope for Phase 0 through Phase 4.

- **User-installable MCP tools** ([user_installable_mcp_tools.md](../docs/tech_specs/user_installable_mcp_tools.md)):
  - Not in current MVP scope per [mvp.md](mvp.md).
  - Deferred; if product adds them later, this plan can be updated.

These behaviors are reflected in Phase 1 (inference path requirement), Phase 2 (workflow model routing), and Phase 4 (API Egress, external model routing fallback, and policy).

## Phase Summary

- **Phase 0** - Foundations (schema, node payloads, MCP gateway spec, LangGraph contract) - Spec complete; implementation in progress / done for Phase 1 scope
- **Phase 1** - Single node happy path (registration, dispatch, sandbox, auth, task APIs) - Complete (readyz, ULID config_version, Worker readyz, 413/truncation; see Current Status)
- **Phase 1.5** - Inference in sandbox (proxy sidecar), E2E inference, CLI, prompt interpretation - Complete (input_mode, prompt-as-model path, BDD; `just ci` passes)
- **Phase 1.7** - Implement cynode-pma; PMA startup as part of orchestrator start - Not started
- **Phase 2** - MCP in the loop, LangGraph workflow, MCP DB/artifact tools - Not started
- **Phase 3** - Multi-node selection, leases, retries, telemetry - Not started
- **Phase 4** - API Egress, Secure Browser, external model routing, CLI expansion, admin console after CLI - Not started

## Scope Summary

- **Inference in sandbox** - Inference proxy sidecar; pod/network so sandbox can call `http://localhost:11434`; `OLLAMA_BASE_URL` in sandbox env; E2E scenario that runs a task invoking inference from inside the sandbox.
- **Prompt interpretation** - Natural-language prompt is default; inference by default; prompt not executed as shell command; minimal path: prompt as model input (sandbox-based or orchestrator-side), result = model output; raw/script/commands mode for explicit shell.
  See Task Breakdown Phase 1.5.
- **Feature files** - E2E inference-in-sandbox scenario; prompt-interpretation scenario (natural-language prompt yields model result); optional worker_node (413, truncation), orchestrator fail-fast clarified.
- **Unit tests** - 90%+ coverage for orchestrator, worker_node, and cynork; no new exceptions in justfile.
- **BDD** - Steps and scenarios for inference-ready node and sandbox job; steps for "create task with natural-language prompt (default), result contains model output"; cynork BDD in place.
- **CLI** - Separate Go module; `version`, `status`, `auth login` / `logout` / `whoami`; create task and get result (natural-language default, inference by default; optional raw/script/commands mode); config via env and optional file.

## Current Status

- **Phase 1:** Complete.
  Node registration, config delivery (with ULID `config_version`), per-node dispatch, sandbox run, user-gateway auth and task APIs in place.
  Orchestrator `GET /readyz` returns 503 when no dispatchable nodes (inference path unavailable); Worker API `GET /readyz` and 413 for oversized body; stdout/stderr truncation (UTF-8-safe, 256 KiB).
- **Phase 1.5:** Complete.
  CLI (cynork), inference proxy sidecar, and prompt interpretation: `input_mode` (prompt/script/commands), default prompt-as-model path (sandbox job with fixed model-call script), BDD/feature coverage.
  `just ci` passes (lint, coverage >=90%, BDD orchestrator/worker_node/cynork).
  See [PHASE1_STATUS.md](../dev_docs/PHASE1_STATUS.md).
- **Phase 1.7:** Docs complete; implementation remaining.
  Requirements and tech specs for `cynode-pma` and `cynode-sba` are in place (PMAGNT, SBAGNT, `cynode_pma.md`, `cynode_sba.md`, PM/PA integration, OpenAI chat mapping).
  Phase 1.7 implementation: build the `cynode-pma` binary (e.g. in an `agents/` Go module), support role flag and instructions paths per spec, and start `cynode-pma` as part of orchestrator startup so the PM chat surface is backed by the real agent.

## Prompt Interpretation: Intended Semantics

**Intended behavior:** Task creation accepts a **natural-language prompt** (e.g. "Tell me what model you are").
The **system** interprets the prompt and decides whether to call the AI model and/or run sandbox job(s).
The prompt is **not** the literal shell command executed in the sandbox.

**Intended flow:** For `input_mode` `"prompt"`, the user prompt goes **directly to the Project Manager (PM) model**; the PM model then decides what to do (e.g. create and run a command in the sandbox, return an answer).
Inference is always used for prompt mode (`use_inference` true).

**Current state (MVP Phase 1):** Prompt->model MUST work.
When the user-gateway has `OLLAMA_BASE_URL` or `INFERENCE_URL` set, the orchestrator calls the model directly and stores the response as a completed job.
If that call fails, it falls back to the sandbox job path.
Explicit `input_mode` `"script"` or `"commands"` runs the prompt as literal shell for backward compatibility.
See [REQ-ORCHES-0126](../docs/requirements/orches.md#req-orches-0126) and task-prompt semantics in `docs/requirements/` and `docs/tech_specs/`.

### Actionable Sequence

1. Inference by default for task jobs (orchestrator defaults job to inference path; raw/script/commands are opt-in).
2. Minimal "prompt as model input" path (Option A: sandbox job with fixed model-call script; Option B: orchestrator calls inference and stores result without sandbox).
3. Explicit raw/script/commands mode for backward compatibility.
4. BDD/feature coverage for natural-language prompt in, model output out.
5. Phase 2: full interpretation via LangGraph/workflow (model + sandbox orchestration) per [langgraph_mvp.md](../docs/tech_specs/langgraph_mvp.md).

See Task Breakdown Phase 1.5 and Phase 2 for concrete tasks.

## Task Breakdown (4-8 Hour Chunks)

This section breaks remaining MVP work into 4-8 hour tasks.
Each task includes the minimum spec and requirement references needed to implement it without deviating from the normative requirements in `docs/requirements/`.

Note: The canonical tech-spec MVP plan currently defines Phase 0 through Phase 4 (plus Phase 1.5).
This MVP plan adds Phase 1.7: first a docs milestone (PMA/SBA requirements and specs-done), then implementation of `cynode-pma` with startup as part of orchestrator start.
Phase 1.7 is not currently listed in `docs/tech_specs/_main.md`.
There is no Phase 5 in [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md) at this time.

### Phase 1 Gap Closure (Spec Alignment)

- **P1-01 (4-6h): Orchestrator health and readiness endpoints with actionable 503 reasons.**
  - **Deliverable**: `GET /healthz` is pure liveness, and `GET /readyz` gates readiness with a stable, human-readable reason when returning 503.
  - **Deliverable**: Orchestrator remains running when not ready and still allows configuration needed to become ready.
  - **Reqs**:
    - [`REQ-ORCHES-0120`](../docs/requirements/orches.md#req-orches-0120)
  - **Specs**:
    - [`CYNAI.ORCHES.Rule.HealthEndpoints`](../docs/tech_specs/orchestrator.md#spec-cynai-orches-rule-healthendpoints)

- **P1-02 (4-8h): Ready-state gating for inference availability and Project Manager warmup.**
  - **Deliverable**: The system refuses to enter ready state until at least one inference-capable path exists.
  - **Deliverable**: When a local inference worker is available, the orchestrator does not enter ready state until the effective Project Manager model is selected and confirmed loaded and available.
  - **Reqs**:
    - [`REQ-BOOTST-0002`](../docs/requirements/bootst.md#req-bootst-0002)
    - [`REQ-ORCHES-0120`](../docs/requirements/orches.md#req-orches-0120)
    - [`REQ-ORCHES-0129`](../docs/requirements/orches.md#req-orches-0129)
  - **Specs**:
    - [`orchestrator_bootstrap.md`](../docs/tech_specs/orchestrator_bootstrap.md#standalone-operation-mode)
    - [`orchestrator.md`](../docs/tech_specs/orchestrator.md#project-manager-model-startup-selection-and-warmup)
    - [`external_model_routing.md`](../docs/tech_specs/external_model_routing.md#routing-goal)

- **P1-03 (4-6h): Node configuration `config_version` ULID generation and monotonic handling.**
  - **Deliverable**: Orchestrator emits `config_version` as a 26-char Crockford Base32 ULID for `node_configuration_payload_v1`.
  - **Deliverable**: Node compares `config_version` lexicographically for monotonic updates.
  - **Specs**:
    - [`CYNAI.WORKER.Payload.ConfigurationV1`](../docs/tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1)

- **P1-04 (4-6h): Worker API `GET /readyz` readiness check.**
  - **Deliverable**: Node exposes unauthenticated `GET /readyz` returning 200 `ready` only when ready to accept job execution requests, otherwise 503.
  - **Reqs**:
    - [`REQ-WORKER-0140`](../docs/requirements/worker.md#req-worker-0140)
    - [`REQ-WORKER-0142`](../docs/requirements/worker.md#req-worker-0142)
  - **Specs**:
    - [`CYNAI.WORKER.WorkerApiHealthChecks`](../docs/tech_specs/worker_api.md#spec-cynai-worker-workerapihealthchecks)

- **P1-05 (4-8h): Worker API request size limits (HTTP 413) and stdout/stderr truncation semantics.**
  - **Deliverable**: Node enforces Worker API request body size limits and returns HTTP 413 for oversized requests.
  - **Deliverable**: Node enforces stdout/stderr capture limits, truncates by bytes while preserving valid UTF-8, and sets `truncated.stdout` and `truncated.stderr` flags.
  - **Reqs**:
    - [`REQ-WORKER-0145`](../docs/requirements/worker.md#req-worker-0145)
    - [`REQ-WORKER-0146`](../docs/requirements/worker.md#req-worker-0146)
    - [`REQ-WORKER-0147`](../docs/requirements/worker.md#req-worker-0147)
  - **Specs**:
    - [`CYNAI.WORKER.WorkerApiRequestSizeLimits`](../docs/tech_specs/worker_api.md#request-size-limits-required)
    - [`CYNAI.WORKER.WorkerApiStdIoCaptureLimits`](../docs/tech_specs/worker_api.md#stdoutstderr-capture-limits-required)

### Phase 1.5 Prompt Interpretation (Make Prompts Behave Like Prompts)

- **P1.5-01 (4-8h): Task create request encodes input mode (prompt vs script vs commands) and enforces interpretation-by-default.**
  - **Deliverable**: For prompt text (plain text or Markdown), the system interprets and uses inference by default.
  - **Deliverable**: Prompt text MUST NOT be executed as a literal shell command unless the user explicitly selects a raw execution mode (script or commands).
  - **Deliverable**: Attachments are accepted and forwarded as task inputs.
  - **Reqs**:
    - [`REQ-ORCHES-0126`](../docs/requirements/orches.md#req-orches-0126)
    - [`REQ-ORCHES-0127`](../docs/requirements/orches.md#req-orches-0127)
    - [`REQ-ORCHES-0128`](../docs/requirements/orches.md#req-orches-0128)
    - [`REQ-CLIENT-0151`](../docs/requirements/client.md#req-client-0151)
    - [`REQ-CLIENT-0153`](../docs/requirements/client.md#req-client-0153)
    - [`REQ-CLIENT-0157`](../docs/requirements/client.md#req-client-0157)
  - **Specs**:
    - [`user_api_gateway.md`](../docs/tech_specs/user_api_gateway.md#core-capabilities)
    - [`CYNAI.CLIENT.CliTaskCreatePrompt`](../docs/tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskcreateprompt)

- **P1.5-02 (4-8h): Minimal "prompt as model input" execution path with result = model output.**
  - **Deliverable**: For a natural-language prompt, orchestrator produces a task result that contains model output, not a shell error like `Tell: not found`.
  - **Deliverable**: Implementation picks a single MVP path and documents it in code and tests.
  - **Option A**: Dispatch a sandbox job with a fixed model-call command that uses `OLLAMA_BASE_URL` and prints the response to stdout.
  - **Option B**: Orchestrator calls inference (local or external via API Egress) and stores the response directly without dispatching a sandbox job.
  - **Reqs**:
    - [`REQ-ORCHES-0126`](../docs/requirements/orches.md#req-orches-0126)
    - [`REQ-ORCHES-0119`](../docs/requirements/orches.md#req-orches-0119)
  - **Specs**:
    - [`sandbox_container.md`](../docs/tech_specs/sandbox_container.md)
    - [`worker_node.md`](../docs/tech_specs/worker_node.md)
    - [`external_model_routing.md`](../docs/tech_specs/external_model_routing.md#external-provider-integration)

- **P1.5-03 (4-6h): Orchestrator and CLI BDD coverage for prompt interpretation defaults.**
  - **Deliverable**: BDD scenario where a task created with a natural-language prompt completes with model output.
  - **Deliverable**: BDD scenario where script/commands mode runs literal shell and preserves backward compatibility for explicit execution.
  - **Reqs**:
    - [`REQ-ORCHES-0126`](../docs/requirements/orches.md#req-orches-0126)
    - [`REQ-ORCHES-0128`](../docs/requirements/orches.md#req-orches-0128)
    - [`REQ-CLIENT-0151`](../docs/requirements/client.md#req-client-0151)
    - [`REQ-CLIENT-0153`](../docs/requirements/client.md#req-client-0153)
  - **Specs**:
    - [`CYNAI.CLIENT.CliTaskCreatePrompt`](../docs/tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskcreateprompt)

### Phase 1.7 Agent Artifacts (PMA First, Then SBA)

- **P1.7-01 (4-8h): Add PMA and SBA requirements domains and tech specs (docs-only).**
  Done.
  - **Deliverable**: New requirements domains `PMAGNT` and `SBAGNT` exist and are indexed.
  - **Deliverable**: New tech specs `cynode_pma.md` and `cynode_sba.md` exist and are indexed.
  - **Reqs**:
    - [`REQ-AGENTS-0002`](../docs/requirements/agents.md#req-agents-0002)
    - [`REQ-AGENTS-0003`](../docs/requirements/agents.md#req-agents-0003)
    - [`REQ-SANDBX-0001`](../docs/requirements/sandbx.md#req-sandbx-0001)
  - **Specs**:
    - [`project_manager_agent.md`](../docs/tech_specs/project_manager_agent.md)
    - [`project_analyst_agent.md`](../docs/tech_specs/project_analyst_agent.md)
    - [`sandbox_container.md`](../docs/tech_specs/sandbox_container.md)

- **P1.7-02 (2-4h): Integrate PMA into existing PM/PA specs and resolve routing wording conflicts.**
  Done.
  - **Deliverable**: `project_manager_agent.md` and `project_analyst_agent.md` explicitly reference `cynode-pma` role modes and separate instructions paths.
  - **Deliverable**: No contradictions with the OpenAI-compatible chat contract (`cynodeai.pm` model id).
  - **Specs**:
    - [`openai_compatible_chat_api.md`](../docs/tech_specs/openai_compatible_chat_api.md)
    - [`cynode_pma.md`](../docs/tech_specs/cynode_pma.md)

- **P1.7-03 (8-16h): Implement cynode-pma binary and agents module.**
  - **Deliverable**: New `agents/` Go module (or equivalent) with `cynode-pma` binary; role flag/config (`project_manager` | `project_analyst`), separate instructions paths per role, config surface and safe defaults per [`cynode_pma.md`](../docs/tech_specs/cynode_pma.md).
  - **Deliverable**: Containerfile for cynode-pma (minimal Alpine base).
  - **Reqs**:
    - [`REQ-PMAGNT-0001`](../docs/requirements/pmagnt.md#req-pmagnt-0001)
    - [`REQ-PMAGNT-0100`](../docs/requirements/pmagnt.md#req-pmagnt-0100)
    - [`REQ-PMAGNT-0101`](../docs/requirements/pmagnt.md#req-pmagnt-0101)
    - [`REQ-PMAGNT-0105`](../docs/requirements/pmagnt.md#req-pmagnt-0105)
  - **Specs**:
    - [`cynode_pma.md`](../docs/tech_specs/cynode_pma.md)

- **P1.7-04 (4-8h): Integrate cynode-pma startup into orchestrator.**
  - **Deliverable**: Orchestrator starts `cynode-pma` (e.g. as subprocess or sidecar) as part of its own startup so the PM chat surface (`cynodeai.pm`) is backed by the real agent.
  - **Deliverable**: Orchestrator readiness/lifecycle accounts for PMA availability where required by bootstrap spec.
  - **Reqs**:
    - [`REQ-PMAGNT-0001`](../docs/requirements/pmagnt.md#req-pmagnt-0001)
    - [`REQ-BOOTST-0002`](../docs/requirements/bootst.md#req-bootst-0002)
  - **Specs**:
    - [`cynode_pma.md`](../docs/tech_specs/cynode_pma.md)
    - [`orchestrator_bootstrap.md`](../docs/tech_specs/orchestrator_bootstrap.md)
    - [`openai_compatible_chat_api.md`](../docs/tech_specs/openai_compatible_chat_api.md)

### Phase 2 MCP in the Loop (Tool Enforcement and Auditing)

- **P2-01 (4-8h): Enforce MCP tool scoping and schema (task_id/run_id/job_id) in the MCP gateway.**
  - **Deliverable**: Standard MCP protocol on the wire, with strict rejection when required scoped ids are missing or mismatched.
  - **Reqs**:
    - [`REQ-MCPGAT-0001`](../docs/requirements/mcpgat.md#req-mcpgat-0001)
    - [`REQ-MCPGAT-0100`](../docs/requirements/mcpgat.md#req-mcpgat-0100)
    - [`REQ-MCPGAT-0103`](../docs/requirements/mcpgat.md#req-mcpgat-0103)
    - [`REQ-MCPGAT-0106`](../docs/requirements/mcpgat.md#req-mcpgat-0106)
  - **Specs**:
    - [`mcp_gateway_enforcement.md`](../docs/tech_specs/mcp_gateway_enforcement.md)

- **P2-02 (4-8h): Emit an audit record for every routed MCP tool call (allow/deny and success/failure).**
  - **Deliverable**: Audit records are always written, regardless of allow/deny and success/failure.
  - **Deliverable**: For MVP, do not store tool args/results in Postgres audit tables.
  - **Reqs**:
    - [`REQ-MCPGAT-0002`](../docs/requirements/mcpgat.md#req-mcpgat-0002)
    - [`REQ-MCPGAT-0107`](../docs/requirements/mcpgat.md#req-mcpgat-0107)
    - [`REQ-MCPGAT-0110`](../docs/requirements/mcpgat.md#req-mcpgat-0110)
  - **Specs**:
    - [`mcp_tool_call_auditing.md`](../docs/tech_specs/mcp_tool_call_auditing.md)

- **P2-03 (4-8h): Implement the minimal MCP tool catalog slice for preferences (get, list, effective).**
  - **Deliverable**: Tools `db.preference.get`, `db.preference.list`, and `db.preference.effective` exist with typed schemas and size-limited responses.
  - **Reqs**:
    - [`REQ-MCPTOO-0117`](../docs/requirements/mcptoo.md#req-mcptoo-0117)
    - [`REQ-MCPTOO-0109`](../docs/requirements/mcptoo.md#req-mcptoo-0109)
    - [`REQ-MCPTOO-0110`](../docs/requirements/mcptoo.md#req-mcptoo-0110)
  - **Specs**:
    - [`mcp_tool_catalog.md`](../docs/tech_specs/mcp_tool_catalog.md)
    - [`user_preferences.md`](../docs/tech_specs/user_preferences.md)

### Phase 3 Multi Node Robustness (Scheduling, Reliability, Telemetry)

- **P3-01 (4-8h): Node selection v1 (capability, load, data locality, model availability).**
  - **Deliverable**: Scheduler selects eligible nodes using the same node selection and job dispatch contracts used by the rest of the orchestrator.
  - **Deliverable**: Selection inputs are grounded in capability reports and orchestrator-maintained node state (dispatchable, drained, etc.).
  - **Reqs**:
    - [`REQ-ORCHES-0107`](../docs/requirements/orches.md#req-orches-0107)
- [`REQ-ORCHES-0114`](../docs/requirements/orches.md#req-orches-0114)
- [`REQ-ORCHES-0123`](../docs/requirements/orches.md#req-orches-0123)
  - **Specs**:
    - [`orchestrator.md`](../docs/tech_specs/orchestrator.md#task-scheduler)
    - [`worker_node_payloads.md`](../docs/tech_specs/worker_node_payloads.md)
    - [`worker_node.md`](../docs/tech_specs/worker_node.md)

- **P3-02 (4-8h): Job leases and retry safety (lease_id, lease expiry, idempotency).**
  - **Deliverable**: Jobs use a lease to prevent duplicate execution and to recover safely when a node or orchestrator restarts.
  - **Deliverable**: Mutating dispatch and completion operations support idempotent retries where clients or internal workers may retry.
  - **Reqs**:
    - [`REQ-STANDS-0129`](../docs/requirements/stands.md#req-stands-0129)
  - **Specs**:
    - [`orchestrator.md`](../docs/tech_specs/orchestrator.md#task-scheduler)
    - [`postgres_schema.md`](../docs/tech_specs/postgres_schema.md#jobs-table)

- **P3-03 (4-8h): Dynamic node configuration updates and capability change reporting.**
  - **Deliverable**: Orchestrator supports dynamic configuration updates after registration and ingests capability reports on registration and node startup.
  - **Deliverable**: Nodes can poll for config updates when enabled, and request refresh on startup and on capability change.
  - **Deliverable**: Node configuration acknowledgement is persisted and used to determine dispatch eligibility.
  - **Reqs**:
    - [`REQ-ORCHES-0114`](../docs/requirements/orches.md#req-orches-0114)
    - [`REQ-WORKER-0135`](../docs/requirements/worker.md#req-worker-0135)
  - **Specs**:
    - [`worker_node.md`](../docs/tech_specs/worker_node.md)
    - [`worker_node_payloads.md`](../docs/tech_specs/worker_node_payloads.md#node-configuration-payload)
    - [`worker_node_payloads.md`](../docs/tech_specs/worker_node_payloads.md#node-configuration-acknowledgement)

- **P3-04 (4-8h): Worker Telemetry API integration (node => orchestrator).**
  - **Deliverable**: Orchestrator can pull node operational telemetry (logs, system info, container inventory/state) via the Worker Telemetry API.
  - **Deliverable**: Pulls use per-request timeouts and tolerate node unavailability.
  - **Deliverable**: Telemetry is treated as non-authoritative and does not drive correctness-critical scheduling decisions by itself.
  - **Reqs**:
    - [`REQ-ORCHES-0141`](../docs/requirements/orches.md#req-orches-0141)
    - [`REQ-ORCHES-0142`](../docs/requirements/orches.md#req-orches-0142)
    - [`REQ-ORCHES-0143`](../docs/requirements/orches.md#req-orches-0143)
    - [`REQ-WORKER-0003`](../docs/requirements/worker.md#req-worker-0003)
    - [`REQ-WORKER-0200`](../docs/requirements/worker.md#req-worker-0200)
    - [`REQ-WORKER-0201`](../docs/requirements/worker.md#req-worker-0201)
    - [`REQ-WORKER-0210`](../docs/requirements/worker.md#req-worker-0210)
    - [`REQ-WORKER-0211`](../docs/requirements/worker.md#req-worker-0211)
    - [`REQ-WORKER-0212`](../docs/requirements/worker.md#req-worker-0212)
    - [`REQ-WORKER-0220`](../docs/requirements/worker.md#req-worker-0220)
    - [`REQ-WORKER-0221`](../docs/requirements/worker.md#req-worker-0221)
    - [`REQ-WORKER-0222`](../docs/requirements/worker.md#req-worker-0222)
    - [`REQ-WORKER-0230`](../docs/requirements/worker.md#req-worker-0230)
    - [`REQ-WORKER-0231`](../docs/requirements/worker.md#req-worker-0231)
    - [`REQ-WORKER-0232`](../docs/requirements/worker.md#req-worker-0232)
    - [`REQ-WORKER-0233`](../docs/requirements/worker.md#req-worker-0233)
    - [`REQ-WORKER-0234`](../docs/requirements/worker.md#req-worker-0234)
    - [`REQ-WORKER-0240`](../docs/requirements/worker.md#req-worker-0240)
    - [`REQ-WORKER-0241`](../docs/requirements/worker.md#req-worker-0241)
    - [`REQ-WORKER-0242`](../docs/requirements/worker.md#req-worker-0242)
    - [`REQ-WORKER-0243`](../docs/requirements/worker.md#req-worker-0243)
  - **Specs**:
    - [`CYNAI.ORCHES.NodeTelemetryPull`](../docs/tech_specs/worker_telemetry_api.md#spec-cynai-orches-nodetelemetrypull)
    - [`CYNAI.WORKER.Doc.WorkerTelemetryApi`](../docs/tech_specs/worker_telemetry_api.md#spec-cynai-worker-doc-workertelemetryapi)

### Phase 4 API Egress and External Routing (Controlled External Calls)

- **P4-01 (4-8h): Implement API Egress access control checks and audit logging for outbound calls.**
  - **Deliverable**: Validate subject identity, provider/operation allowlists, and credential authorization before performing outbound calls.
  - **Deliverable**: Log each call with task context, provider, operation, and timing information.
  - **Reqs**:
    - [`REQ-APIEGR-0110`](../docs/requirements/apiegr.md#req-apiegr-0110)
    - [`REQ-APIEGR-0112`](../docs/requirements/apiegr.md#req-apiegr-0112)
    - [`REQ-APIEGR-0113`](../docs/requirements/apiegr.md#req-apiegr-0113)
    - [`REQ-APIEGR-0119`](../docs/requirements/apiegr.md#req-apiegr-0119)
  - **Specs**:
    - [`api_egress_server.md`](../docs/tech_specs/api_egress_server.md)
    - [`access_control.md`](../docs/tech_specs/access_control.md)

- **P4-02 (4-8h): Implement external model routing decision record and provider call plumbing via API Egress.**
  - **Deliverable**: Orchestrator can route an approved model call request to API Egress with `provider`, `operation`, `params`, and `task_id`, and can store the result in PostgreSQL task history.
  - **Deliverable**: Routing decisions are logged with chosen provider and high-level reason.
  - **Reqs**:
    - [`REQ-ORCHES-0119`](../docs/requirements/orches.md#req-orches-0119)
    - [`REQ-APIEGR-0001`](../docs/requirements/apiegr.md#req-apiegr-0001)
  - **Specs**:
    - [`external_model_routing.md`](../docs/tech_specs/external_model_routing.md)
    - [`api_egress_server.md`](../docs/tech_specs/api_egress_server.md)

### Phase 5 (Not Defined in Current Tech Specs)

Phase 5 does not exist in the canonical MVP plan in [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).
If we want a Phase 5, we should first define its scope and add it to the tech-spec plan explicitly.

- **P5-01 (4-6h): Draft a Phase 5 proposal (non-canonical) and map it to requirements.**
  - **Deliverable**: A short proposal section (or a new dev doc) that defines Phase 5 goals, acceptance criteria, and the list of requirement IDs it targets.
  - **Deliverable**: A concrete decision point for whether to update [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md) (requires explicit instruction).
  - **Specs**:
    - [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md)

## Phase 0 Foundations

- Postgres schema (users, auth sessions, groups/RBAC, tasks, jobs, nodes, artifacts, audit).
- Node payloads: capability report, config payload, registration bootstrap (PSK to JWT), config versioning, refresh/ack/rollback.
- MCP gateway enforcement: allowlists, access control, auditing, task-scoped tool schemas, policy mapping to `access_control_rules` (action `mcp.tool.invoke`).
- LangGraph MVP workflow contract and checkpointing.

Reference: [docs/tech_specs/\_main.md](../docs/tech_specs/_main.md) Phase 0.

## Phase 1 Single Node Happy Path

- Orchestrator: registration, capability ingest, config delivery, job dispatch, result collection.
- Job dispatch: direct HTTP to Worker API (per-node URL and token from config); no MCP gateway in loop.
- Node: Node Manager startup (orchestrator before Ollama); Worker API receives job, runs sandbox, returns result.
- System: at least one inference-capable path (node-local inference or external provider via API Egress when configured).
  In the single-node case, the system MUST refuse to enter a ready state if the node cannot run the inference container and there is no configured external provider key.
- Orchestrator: Project Manager model selection and warmup at startup.
- User API Gateway: local auth (login/refresh), create task, retrieve result.
- Phase 1: config on startup only; long-lived node JWT; tasks as single sandbox job (no LangGraph).
- Task creation: plain text/Markdown, attachments, script, or commands; interpretation and inference default; Phase 1 may pass prompt as sandbox command until interpretation layer exists.

Reference: [docs/tech_specs/\_main.md](../docs/tech_specs/_main.md) Phase 1; [external_model_routing.md](../docs/tech_specs/external_model_routing.md) (routing goal and ready-state gating).

## Phase 1.5 Single Node Full Capability

- **Node-local inference from sandbox:** For each job that may use inference, the node runs a pod (or equivalent isolated network) containing the sandbox container and a lightweight inference proxy sidecar that listens on `localhost:11434` inside the pod and forwards to the node's Ollama container.
  Sandbox receives `OLLAMA_BASE_URL=http://localhost:11434`.
  Proxy enforces request size (e.g. 10 MiB) and per-request timeout (e.g. 120s) and MUST NOT expose credentials.
  See [worker_node.md](../docs/tech_specs/worker_node.md) Option A and [ports_and_endpoints.md](../docs/tech_specs/ports_and_endpoints.md).
- **Implementation ownership:** Worker node (Node Manager or Worker API) creates the pod and starts sandbox + proxy when a job requests inference; injects `OLLAMA_BASE_URL`.
  Orchestrator: dispatch remains HTTP to Worker API; optional job/task hint for inference so the node can choose pod+proxy vs plain container.
- **E2E:** Scenario that runs task with inference inside the sandbox (e.g. command that calls `http://localhost:11434` or echoes `OLLAMA_BASE_URL`).
  Script-driven E2E (`just e2e` / `scripts/setup-dev.sh` full-demo) runs this when the node is started with inference and a model is loaded.
- **CLI (cynork):** Separate Go module at `cynork/`; in `go.work` and justfile `go_modules`; version, status, auth (login/logout/whoami), task create/result; config via env and optional `~/.config/cynork/config.yaml`.
  Gateway URL default `http://localhost:8080`; no direct DB access; all operations via User API Gateway.
  See [cynork_cli.md](../docs/tech_specs/cynork_cli.md) and [ports_and_endpoints.md](../docs/tech_specs/ports_and_endpoints.md).

**Done:** CLI module, inference proxy and pod path, worker_node BDD for inference; orchestrator/User API default to inference path; minimal prompt-as-model-input path (Option A); raw/script/commands mode; BDD for natural-language prompt (default) and result containing model output, and for commands mode (literal shell).
Worker BDD: GET /readyz, 413 on oversized body. `just ci` passes.
**Next:** Phase 2 (MCP in the loop, LangGraph workflow).

Reference: [docs/tech_specs/worker_node.md](../docs/tech_specs/worker_node.md), [docs/tech_specs/sandbox_container.md](../docs/tech_specs/sandbox_container.md).

## Phase 2 MCP in the Loop

- Orchestrator MCP tool gateway with role-based access.
- MCP database tools (orchestrator-side agents); MCP artifact tools (worker agents); no direct Postgres from agents.
- **Workflow engine:** **Separate Python LangGraph process** invoked by the Go orchestrator; one instance per `task_id`; **lease in orchestrator DB** (single-active-workflow-per-task); **prescriptive checkpoint schema** in PostgreSQL ([workflow_checkpoints](../docs/tech_specs/postgres_schema.md#workflow-checkpoints-table), [task_workflow_leases](../docs/tech_specs/postgres_schema.md#task-workflow-leases-table)).
  Workflow nodes map to MCP DB, model routing (local or API Egress), Worker API dispatch, result collection.
- **Scheduler:** When a run requires interpretation, **scheduler hands payload directly to PMA**; **PMA creates the task and starts the workflow internally** (no separate enqueue-workflow-start step).
- **Verify Step Result:** **PMA tasks the Project Analyst (or other sandbox agent)** to verify; findings back to workflow state.
- **Process boundaries:** **cynode-pma** (chat, MCP) and **workflow runner** (LangGraph) are **separate processes** sharing MCP gateway and DB; orchestrator starts workflow runner for a task; chat and planning go to PMA.

Model routing in Phase 2 follows [external_model_routing.md](../docs/tech_specs/external_model_routing.md): local preferred; external via API Egress when policy allows; orchestrator-side agent settings (e.g. Project Manager / Project Analyst) may override defaults.

### Phase 2 LangGraph Integration Checklist

Concrete steps for Phase 2 LangGraph integration:

- (a) Checkpoint table/schema implemented per [langgraph_mvp.md](../docs/tech_specs/langgraph_mvp.md) and [postgres_schema.md](../docs/tech_specs/postgres_schema.md).
- (b) Workflow start/resume API (Go orchestrator to Python LangGraph process).
- (c) Graph nodes wired to MCP DB tools and Worker API.
- (d) Lease acquisition in orchestrator DB (workflow runner acquires/checks lease via orchestrator).
- (e) Verify Step Result implemented as PMA tasking Project Analyst or other sandbox agent.

### Phase 2 LangGraph Tasks (Optional 4-8 Hour Chunks)

- **P2-04 (4-8h):** Checkpoint table and schema per spec; workflow runner can persist and load by task_id.
- **P2-05 (4-8h):** Workflow start/resume API from Go orchestrator to Python LangGraph process.
- **P2-06 (4-8h):** Graph nodes wired to MCP DB tools and Worker API (Load Task Context, Plan Steps, Dispatch Step, Collect Result, Finalize Summary, Mark Failed).
- **P2-07 (4-8h):** Lease acquisition in orchestrator DB; workflow runner acquires/checks lease via orchestrator before running.
- **P2-08 (4-8h):** Verify Step Result implemented as PMA tasking Project Analyst or other sandbox agent; findings written back to workflow state.

Reference: [docs/tech_specs/\_main.md](../docs/tech_specs/_main.md) Phase 2, [langgraph_mvp.md](../docs/tech_specs/langgraph_mvp.md), [orchestrator.md](../docs/tech_specs/orchestrator.md#workflow-engine).

## Phase 3 Multi Node Robustness

- Node selection: capability, load, data locality, model availability.
- Job leases, retries, idempotency, heartbeats.
- Dynamic node config updates and capability change reporting.
- Worker Telemetry API for node health and ops.

Reference: [docs/tech_specs/\_main.md](../docs/tech_specs/_main.md) Phase 3, [worker_telemetry_api.md](../docs/tech_specs/worker_telemetry_api.md).

## Phase 4 Optional Controlled Egress and Integrations

- API Egress Server: ACL enforcement, auditing; credentials in PostgreSQL; outbound calls only via API Egress (see [api_egress_server.md](../docs/tech_specs/api_egress_server.md)).
- External model routing: fallback for standalone orchestrator (no workers) and when local capacity cannot satisfy requirements.
  Routing policy, signals, and settings per [external_model_routing.md](../docs/tech_specs/external_model_routing.md).
  External inference with node sandboxes (model via API Egress, sandbox for tools); per-agent routing settings for Project Manager and Project Analyst.
- Secure Browser Service: deterministic sanitization, DB-backed rules.
- CLI expansion per [cynork_cli.md](../docs/tech_specs/cynork_cli.md) MVP Scope: auth token support and `whoami`; interactive shell mode with tab completion for all MVP commands; credential list, create, rotate, disable for API Egress; preference list, get, set for system and user scopes; effective preferences for a task; node list, get, enable, disable, drain; skills (see [skills_storage_and_inference.md](../docs/tech_specs/skills_storage_and_inference.md)).
  Recommended admin keys: [cli_management_app_commands_admin.md](../docs/tech_specs/cli_management_app_commands_admin.md).
- Web Console after CLI; parity with CLI for admin capabilities.

Reference: [docs/tech_specs/\_main.md](../docs/tech_specs/_main.md) Phase 4.

## Feature Files and BDD

- **`features/cynork/cynork_cli.feature`:** Exists; covers status, auth, create task and get result.
  Suite tag `@suite_cynork`; see [features/README.md](../features/README.md).
- **`features/e2e/single_node_happy_path.feature`:** Inference scenario exists (`@inference_in_sandbox`).
  Not run by `just test-bdd` (no e2e Godog runner); script-driven `just e2e` is primary.
- **`features/worker_node/worker_node_sandbox_execution.feature`:** Inference scenario, use_inference step, GET /readyz (200 "ready"), and 413 on oversized body scenarios in place.
- **`features/orchestrator/orchestrator_task_lifecycle.feature`:** Scenarios for task with natural-language prompt (default) completing with model output, and for input_mode commands running literal shell; step "the job sent to the worker has command containing ...".
- **`features/orchestrator/orchestrator_startup.feature`:** Orchestrator readyz returns 503 when no inference path (no dispatchable nodes).
- **BDD suites:** `just test-bdd` runs `./orchestrator/_bdd ./worker_node/_bdd ./cynork/_bdd`; orchestrator and worker_node steps for prompt interpretation, readyz, and 413 implemented; all pass under `just ci`.

## Unit Tests and Coverage

- **Target:** All Go modules (orchestrator, worker_node, cynork) maintain at least 90% package-level coverage under `just test-go-cover`; control-plane may keep existing exception if justified.
- **Practice:** New packages (inference proxy, pod/network helpers, CLI gateway client, CLI commands) must have unit tests from the start; use table-driven tests and existing patterns; mock external HTTP and container runtime where appropriate.
- **CI:** `just ci` runs `test-go-cover` and fails if any package is below threshold; no new broad exclusions.

## Implementation Order (Done vs Remaining)

Summary of what is done and what remains; task IDs refer to the Task Breakdown above.

### Done (Completed Items)

Items below are implemented.

- Phase 1 gap closure: P1-01-P1-05 (orchestrator health/readyz with 503 reason and inference-path gating; config_version ULID; Worker API GET /readyz; 413 and UTF-8-safe truncation).
- CLI module bootstrap: cynork at `cynork/`, in go.work and go_modules; version, status, auth, task create/result; unit tests and BDD; documented; `--input-mode` (prompt/script/commands).
- Inference proxy and pod/network: worker node supports `use_inference: true` jobs via Podman pod (sandbox + proxy sidecar, `OLLAMA_BASE_URL` in sandbox env).
- P1.5-01-P1.5-03: input_mode and interpretation-by-default; minimal prompt-as-model-input path (Option A); BDD for natural-language prompt (default) and commands mode; feature files and orchestrator/worker_node BDD steps (readyz, 413).
- P1.7-01, P1.7-02: PMAGNT/SBAGNT requirements and cynode_pma/cynode_sba tech specs; PM/PA integration and OpenAI chat mapping (docs complete).
- Coverage and CI: `just ci` runs fmt, lint, test-go-cover (>=90%), vulncheck-go, test-bdd for orchestrator, worker_node, cynork; all pass.

### Remaining (Order)

1. Phase 1.7 implementation: implement `cynode-pma` binary (P1.7-03) and integrate PMA startup into orchestrator (P1.7-04).
2. E2E script: optional extend setup-dev.sh to run inference-in-sandbox and prompt-interpretation path when node and model are available.
3. Phase 2: MCP in the loop, LangGraph workflow, MCP DB/artifact tools.
4. Phase 3: Multi-node selection, leases, retries, telemetry.
5. Phase 4: API Egress and external routing.

## References

- [docs/tech_specs/\_main.md](../docs/tech_specs/_main.md) - Tech spec index and MVP phase list.
- [docs/tech_specs/external_model_routing.md](../docs/tech_specs/external_model_routing.md) - Routing goal, policy, API Egress integration, external inference with sandboxes, settings, auditing.
- [docs/tech_specs/orchestrator_bootstrap.md](../docs/tech_specs/orchestrator_bootstrap.md) - Standalone mode and ready-state requirements for inference availability and Project Manager warmup.
- [docs/tech_specs/worker_node.md](../docs/tech_specs/worker_node.md) - Node-local inference, Option A (proxy sidecar).
- [docs/tech_specs/sandbox_container.md](../docs/tech_specs/sandbox_container.md) - Node-local inference access.
- [docs/tech_specs/cynork_cli.md](../docs/tech_specs/cynork_cli.md) - CLI goals, commands, MVP scope.
- [docs/tech_specs/ports_and_endpoints.md](../docs/tech_specs/ports_and_endpoints.md) - Default ports and E2E/BDD port usage.
- [docs/tech_specs/sandbox_image_registry.md](../docs/tech_specs/sandbox_image_registry.md) - Registry behavior deferred; schema tables in scope for MVP (see Tech Spec Alignment).
- [docs/tech_specs/user_installable_mcp_tools.md](../docs/tech_specs/user_installable_mcp_tools.md) - Out of MVP scope; deferred.
- [docs/development_setup.md](../docs/development_setup.md) - Local setup, scripts, E2E, troubleshooting.
- [PHASE1_STATUS.md](../dev_docs/PHASE1_STATUS.md) - Phase 1 implementation summary and running locally.
