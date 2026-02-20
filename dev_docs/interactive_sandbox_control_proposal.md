# Interactive Sandbox Control Proposal (Long-Running AI Coding Projects)

- [Document Metadata](#document-metadata)
- [Proposal Summary](#proposal-summary)
- [Problem Motivation](#problem-motivation)
- [Design Constraints From Existing Requirements and Specs](#design-constraints-from-existing-requirements-and-specs)
- [Primary Goals](#primary-goals)
- [Non-Goals and Scope](#non-goals-and-scope)
- [Proposed Architecture](#proposed-architecture)
- [Interactive Control Without Inbound Sandbox Connectivity](#interactive-control-without-inbound-sandbox-connectivity)
- [Workspace and File Editing Model](#workspace-and-file-editing-model)
- [Egress Integration for Long-Running Coding Projects](#egress-integration-for-long-running-coding-projects)
- [Proposed MCP Tool Surface (Orchestrator-Side)](#proposed-mcp-tool-surface-orchestrator-side)
- [Multi-Agent Long-Running Projects](#multi-agent-long-running-projects)
- [Local Fast Path: Direct Worker Interaction (Co-Located Model and Sandbox)](#local-fast-path-direct-worker-interaction-co-located-model-and-sandbox)
- [Safety, Policy, and Auditing](#safety-policy-and-auditing)
- [Failure Modes and Mitigations](#failure-modes-and-mitigations)
- [Phased Implementation Plan](#phased-implementation-plan)
- [Open Questions](#open-questions)

## Document Metadata

Date (UTC): 2026-02-20

## Proposal Summary

This proposal describes how CyNodeAI can provide one or more AI systems with effectively interactive control of sandbox containers for long-running coding projects.
The design aligns with existing specs for session sandboxes, MCP gateway enforcement, and controlled egress (API egress and git egress).
The key approach is "file-first + exec-round sessions", with an optional PTY streaming extension for true terminal interactivity when required.

## Problem Motivation

Long-running coding projects require stateful environments.
State includes a persistent workspace, build artifacts, dependency caches, and the ability to iterate by running many commands over time.
The main risk is granting interactive control in a way that increases credential exposure, uncontrolled network egress, or un-audited destructive operations.

## Design Constraints From Existing Requirements and Specs

- Sandboxes are untrusted and network-restricted by default.
  See `docs/tech_specs/sandbox_container.md` and its threat model and connectivity sections.
- Sandbox control is performed by the Node Manager using container runtime operations.
  Agents do not connect inbound to a sandbox over the network.
  See `docs/tech_specs/sandbox_container.md#sandbox-connectivity`.
- Long-running session sandboxes already exist as a required capability.
  They reuse the same container across multiple exec rounds to preserve workspace state.
  See `docs/tech_specs/worker_api.md#session-sandbox-long-running` and `docs/tech_specs/sandbox_container.md#long-running-session-sandboxes`.
- Credentials must not be exposed to sandboxes.
  External provider calls must route through controlled services.
  See `docs/tech_specs/api_egress_server.md` and `docs/tech_specs/git_egress_mcp.md`.
- Git operations requiring remote access must be performed through git egress.
  Sandboxes must export credential-free changesets tied to a single `task_id`.
  See `docs/requirements/apiegr.md` and `docs/tech_specs/git_egress_mcp.md`.
- MCP tool use is centrally allowlisted and audited by the orchestrator gateway by default.
  For node-local agent runtimes, an edge enforcement mode may be used where the node enforces orchestrator-issued capability leases and emits auditable tool call records.
  Task scoping must be expressed in tool arguments.
  See `docs/tech_specs/mcp_gateway_enforcement.md`.

## Primary Goals

- Provide AI agents a stateful, iterative workflow inside sandbox containers.
- Support unattended execution (no human in the loop) while staying fail-closed and auditable.
- Make the default workflow deterministic and reviewable.
- Integrate existing egress patterns, especially git egress for code promotion.

## Non-Goals and Scope

- This proposal does not define a full CI system.
  It assumes CI is triggered externally or via existing repository tooling.
- This proposal does not require true "remote desktop" style interaction inside sandboxes.

## Proposed Architecture

This section describes the components, trust boundaries, and control paths needed for long-running sandbox work.

### High-Level Components

- Orchestrator (control-plane + MCP gateway).
- Orchestrator-side agents (for example, Project Manager Agent).
- Worker node Node Manager (container runtime control).
- Sandbox containers (job sandboxes and session sandboxes).
- Artifact service and storage (task-scoped artifacts).
- Controlled egress services.
  This includes API egress, git egress, and secure browser / sanitized fetch where applicable.

### Trust Boundaries

- Sandboxes are treated as hostile.
- Orchestrator services and egress services are trusted and must enforce policy.
- Credentials live only in controlled services (API egress, git egress) and are never returned to agents or sandboxes.
  See `docs/tech_specs/api_egress_server.md#credential-storage` and `docs/tech_specs/git_egress_mcp.md#credential-storage`.

## Interactive Control Without Inbound Sandbox Connectivity

This section defines what "interactive control" means in CyNodeAI terms.
It is the ability to iterate quickly in a persistent container via repeated command execution and file changes, with streaming visibility into outputs and state.
It does not require SSH or an inbound network server inside the sandbox.

### Primary Mechanism: Session Sandboxes With Exec Rounds (Recommended Default)

Use the existing `SessionSandbox` capability.
The orchestrator creates a session container, then issues multiple `exec` calls over time in that same container.
Each exec call returns stdout, stderr, and exit code, and the workspace persists across calls.
This is already established in `docs/tech_specs/worker_api.md#session-sandbox-long-running`.

Why this works well for long-running coding:

- The environment stays warm.
- The agent can run small steps, observe outputs, and adjust.
- The state transition between steps is explicit and auditable (one exec round at a time).
- Timeouts and output truncation are already part of the Worker API contract.
  See `docs/tech_specs/worker_api.md#logging-and-output-limits`.

### Optional Extension: PTY Streaming for True Terminal Interactivity

Some tasks are materially easier with a real TTY.
Examples include REPL-driven debugging, TUIs, interactive installers, or workflows that rely on terminal cursor control.

Proposal:

- Add an optional PTY mode for session sandboxes that is still controlled via the node and container runtime.
- PTY sessions are exposed as a streaming channel (for example, WebSocket or gRPC stream) between orchestrator and node.
- The sandbox remains non-addressable from the network.

PTY is not required for MVP.
The exec-round model should remain the default path because it is easier to constrain and audit.

## Workspace and File Editing Model

This section defines how agents edit code safely and persistently while keeping promotion paths reviewable and auditable.

### Principle: File-First Tools, Terminal-Second

Give orchestrator-side agents structured file tools for the workspace.
Use terminal execution for builds, tests, and inspection.
Avoid making the editing workflow depend on terminal-only editors.

Structured file operations also produce a clean audit trail and enable deterministic replays.

### Workspace Persistence and Mounting

Use a host-managed workspace checkout and mount it into the session sandbox at `/workspace`.
This matches the sandbox contract recommendation in `docs/tech_specs/sandbox_container.md#filesystem-and-working-directories`.
For multi-agent work, allocate one workspace per agent or per task, and prefer branch-per-agent patterns to reduce file contention.

### Change Export From Sandbox

When a sandbox produces code changes, it must export a credential-free changeset artifact tied to `task_id`.
Prefer patch-based exports as the default.
See `docs/tech_specs/git_egress_mcp.md#sandbox-output-formats` and `docs/requirements/apiegr.md#req-apiegr-0104`.

### Git Usage Inside Sandboxes (Clean Split)

Sandboxes may include Git for local workflows, but remote operations must remain outside the sandbox trust boundary.

- Allowed in sandbox: local-only Git commands against the mounted workspace (for example `git status`, `git diff`, local `git commit`).
- Disallowed in sandbox: any remote-affecting Git operation (for example `git clone`, `git fetch`, `git pull`, `git push`, submodule fetch/update, and Git LFS downloads).
- Promotion and remote Git operations: use git egress with task-scoped changeset artifacts.
  See `docs/tech_specs/git_egress_mcp.md` and `docs/tech_specs/sandbox_container.md#git-behavior-local-only`.

## Egress Integration for Long-Running Coding Projects

This section connects "interactive sandbox control" to the already-defined controlled egress model.

### Git Egress as the Standard Code Promotion Path

Do not allow sandboxes to push to Git remotes.
Instead:

- The sandbox produces a patch or bundle artifact.
- The orchestrator validates policy.
- Git egress applies the changeset to a controlled checkout and performs `git.commit.create`, `git.push`, and `git.pr.create` as allowed.
  See `docs/tech_specs/git_egress_mcp.md#architecture-and-trust-boundaries` and `docs/tech_specs/git_egress_mcp.md#mcp-tool-interface`.

This directly supports unattended operation.
It also keeps Git credentials out of sandboxes and out of agent processes.
See `docs/requirements/apiegr.md#req-apiegr-0100` through `#req-apiegr-0103`.

### API Egress for External Provider Calls

Agents should not make outbound API calls directly.
When a task needs a provider integration (issue creation, commenting, notifications, model provider calls, and similar), route through API egress.
See `docs/tech_specs/api_egress_server.md#agent-interaction-model`.

For long-running coding projects, this matters for:

- Reporting status to external systems.
- Fetching metadata needed for changes (for example, PR templates or issue context) without giving direct network egress to sandboxes.

### Web Fetch and Secure Browser

Where the system allows web content retrieval, use the controlled mechanisms already described by tool allowlists and policy.
The Worker Agent allowlist suggests `web.fetch` as a sanitized option when policy allows it.
See `docs/tech_specs/mcp_gateway_enforcement.md#worker-agent-allowlist`.

For richer workflows, use the Secure Browser Service rather than opening general outbound egress from sandboxes.
See `docs/tech_specs/secure_browser_service.md`.

## Proposed MCP Tool Surface (Orchestrator-Side)

This proposal assumes the orchestrator-side Project Manager Agent is the primary holder of `sandbox.*` capabilities.
This is aligned with `docs/tech_specs/mcp_gateway_enforcement.md#project-manager-agent-allowlist`.

### Sandbox Session Tools

Map directly to the Worker API session sandbox operations.

- `sandbox.session.create`
  - Required arguments: `task_id`, `session_id`, `image_ref`, `workspace_ref`, `constraints`.
- `sandbox.session.exec`
  - Required arguments: `task_id`, `session_id`, `argv`, `timeout_ms`, `cwd`, `env`.
- `sandbox.session.end`
  - Required arguments: `task_id`, `session_id`, `reason`.

All responses must be size-bounded and include structured fields for exit code, stdout, stderr, and truncation flags.
This should align to the Worker API execution response fields in `docs/tech_specs/worker_api.md`.

### Workspace File Tools

These tools operate on the mounted workspace in a controlled way.
They are recommended even if a terminal is available.

- `sandbox.workspace.read_file`
- `sandbox.workspace.write_file`
- `sandbox.workspace.apply_patch`
- `sandbox.workspace.list`
- `sandbox.workspace.search`

Policy recommendation:

- Apply path allowlists per task or per repo profile.
- Enforce maximum file size and maximum patch size.
- Record content hashes for auditing.

### Artifact Tools

Use artifacts as the standard boundary object between sandbox execution and egress operations.

- `artifact.put` for changesets, test outputs, and logs.
- `artifact.get` for promotion and verification workflows.

This integrates naturally with git egress, which expects patch or bundle artifacts.
See `docs/tech_specs/git_egress_mcp.md#sandbox-output-formats`.

### Git Egress Tools

Use `git.*` tools only through the git egress service.
The orchestrator gateway allowlist and access control policy are the first gate.
The git egress service enforces policy again and records a second audit trail.
See `docs/tech_specs/git_egress_mcp.md#access-control` and `docs/tech_specs/mcp_gateway_enforcement.md#gateway-enforcement-responsibilities`.

## Multi-Agent Long-Running Projects

Long-running coding projects often benefit from parallelism.
This proposal supports multiple AI systems by isolating work and using explicit merge and promotion steps.

Recommended baseline pattern:

- One session sandbox per agent.
- One workspace checkout per agent, on its own branch.
- A coordinating orchestrator-side agent assigns work and merges via git egress.

Optional enhancements:

- Workspace-level file locks for shared checkouts.
- A "patch queue" merge strategy (apply small patches, run checks, then promote).

## Local Fast Path: Direct Worker Interaction (Co-Located Model and Sandbox)

This section extends the proposal to reduce orchestrator round trips when an AI agent runtime (model) and the worker node are co-located on the same host.
The goal is for most sandbox lifecycle, exec, PTY, and workspace file operations to be handled node-locally, with the orchestrator used primarily for scheduling, durable state, and controlled egress.

### Core Idea

- The orchestrator remains the source of truth for tasks, scheduling, and policy.
- The orchestrator issues short-lived, least-privilege capability leases to a node-local agent runtime.
- The node enforces those leases and exposes sandbox operations locally, so the agent can iterate quickly without a network round trip to the orchestrator for each tool call.

This is aligned with:

- Worker session sandboxes and PTY support in `docs/tech_specs/worker_api.md`.
- Node-local sandbox control and capability leases in `docs/tech_specs/node.md#node-local-agent-sandbox-control-low-latency-path`.
- Edge enforcement mode and auditing requirements in `docs/tech_specs/mcp_gateway_enforcement.md#edge-enforcement-mode-node-local-agent-runtimes`.

### What Stays Centralized (No Direct Sandbox Egress)

Even in local fast path mode, do not expand sandbox trust or network egress.
The following remain centralized and policy-controlled:

- Git promotion via git egress (`git.*` tools).
- External provider API calls via API egress (`api.call`).
- Sanitized web retrieval via `web.fetch` / secure browser service, when allowed.
- Database access via orchestrator-side MCP database tools.

### Auditing and State Sync

Direct node-local tool calls must still be auditable.
The node emits and persists audit records for direct tool calls and makes them available to the orchestrator in bounded batches.
This allows local iteration to be low-latency while preserving centralized inspection and retention.

## Safety, Policy, and Auditing

This section defines the minimum controls required to allow unattended operation without expanding the trust boundary.

### Fail-Closed Controls

All tool routing must fail closed when task context is missing.
All tool arguments must include `task_id` and any other required IDs to support deterministic scoping.
See `docs/tech_specs/mcp_gateway_enforcement.md#tool-argument-schema-requirements`.

### Auditing Coverage

Record, at minimum:

- Sandbox session lifecycle events (create, exec, end).
- Command argv, cwd, timeout, and resulting exit code.
- Stdout and stderr hashes (and optionally truncated bodies subject to limits).
- Workspace modifications (file paths and content hashes).
- Artifact creation and retrieval.
- Egress operations, including changeset hashes and policy decisions.

Git egress and API egress already define minimum auditing requirements.
See `docs/tech_specs/git_egress_mcp.md#auditing` and `docs/tech_specs/api_egress_server.md#policy-and-auditing`.

### Secret Handling

Do not attempt to redact secrets from logs post hoc.
Prevent secrets from entering the sandbox environment.
This is consistent with `docs/tech_specs/worker_api.md#secret-handling-required`.

## Failure Modes and Mitigations

- Interactive commands that block on prompts.
  Mitigation: prefer non-interactive flags, enforce timeouts, and add PTY only when justified by the task profile.
- Output truncation hides important error context.
  Mitigation: stream logs to artifacts, and provide structured "tail" retrieval tools for bounded chunks.
- Dependency installation requires outbound network.
  Mitigation: use policy-controlled approaches, such as sanctioned mirrors, prebuilt sandbox images, or controlled fetch mechanisms.
  Do not add arbitrary outbound network egress to sandboxes by default.
- Patch apply conflicts during promotion.
  Mitigation: require base ref capture, constrain diff size, and return structured errors for agent remediation.
  See `docs/tech_specs/git_egress_mcp.md#failure-modes-and-safety`.

## Phased Implementation Plan

This section proposes a staged delivery path that builds on existing Worker API session semantics and existing egress services.

### Phase 0: Use Existing Session Sandbox Semantics

Implement orchestrator-side `sandbox.session.*` MCP tools that map to Worker API session endpoints.
Implement workspace file tools with strict bounds and audit logging.
Add artifact support for patch export.

### Phase 1: Git Egress Promotion Workflow

Standardize a patch artifact format for coding tasks.
Add a default workflow: patch artifact -> git egress apply -> commit -> push -> PR.
Gate promotion on policy, and optionally on passing task-scoped checks when required.

### Phase 2: Optional PTY Streaming

Add a PTY channel as an optional capability for session sandboxes.
Keep it off by default and enable only via policy and task profile.

### Phase 3: Multi-Agent Coordination Enhancements

Add workspace isolation defaults (branch-per-agent) and optional locking.
Add merge orchestration patterns that rely on git egress and policy.

## Open Questions

- What is the minimal set of workspace file tools required for MVP.
- Whether log streaming should be first-class or artifact-only for early phases.
- Which dependency acquisition patterns are acceptable under the controlled egress model for local-first development.
