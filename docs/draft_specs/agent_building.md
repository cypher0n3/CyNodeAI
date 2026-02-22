## Mental Model: What an "Agent" Is

An agent is a control loop around an LLM that:

- Maintains state (goal, plan, task queue, artifacts, context pointers)
- Decides when to think vs when to call tools
- Executes tools (MCP, sandbox, git, web, etc.)
- Observes results and iterates until done or blocked

The LLM is the "policy," not the system.

Your platform (CyNodeAI) is the "actuator" that makes tool calls safe, observable, auditable, and reproducible.

## Agent Architecture That Fits CyNodeAI

### 1) Core Loop (Sense - Think - Act)

Minimum loop:

1. Ingest: goal + current state + recent observations
2. Decide: next action (tool call or message)
3. Act: call tool (via MCP or node sandbox API)
4. Observe: capture structured result
5. Update: state, plan, memory
6. Terminate: success, fail, or ask for input

Key implementation detail: **the model never directly does privileged actions**.
It emits a structured "intent," your runtime executes it.

### 2) State Model (What You Persist)

Recommended state fields (store as JSON):

- session_id, user_id, tenant_id
- objective (immutable)
- constraints (immutable-ish)
- plan (mutable)
- task_queue (list of tasks with status)
- working_memory (small, rotating)
- artifacts (files produced, diffs, outputs)
- tool_context (capabilities, limits, auth scopes)
- audit_log pointers (tool calls, approvals)
- termination_criteria

Do not store raw full transcripts as "memory" for the agent loop; store compact state plus pointers.

### 3) Tool Interface (MCP and Your Internal Tools)

Treat every tool as:

- Name
- JSON schema for arguments
- JSON schema for returns
- Policy metadata (risk level, approval required, network allowed, secrets allowed, max runtime, allowed images, etc.)

In CyNodeAI terms:

- "Skills" are tool bundles + prompts + constraints + validators
- MCP is the standardized way to expose tools (either your tools or third-party tools) to the agent

### 4) Skills (A Practical Definition)

A skill should be a versioned package with:

- skill.yaml (id, version, description, tools exposed, limits, required approvals)
- prompts/ (system and few-shot patterns)
- schemas/ (input/output JSON schema)
- validators/ (server-side checks, allowlists, lint rules)
- tests/ (golden inputs, expected tool calls, expected outputs)

Skills should be executable without the LLM in the loop for validation (schema, policy, allowlists).

## Designing Agents for Your Platform

### Pattern a - "PM Agent" (Planner and Dispatcher)

Responsibilities:

- Break down objectives into tasks
- Choose which skill to use per task
- Dispatch tasks to node workers/sandboxes
- Aggregate results and produce final deliverables

Constraints:

- Limited direct execution tools
- High emphasis on structured output and correctness
- Strong guardrails (no network by default, approvals for risky actions)

### Pattern B - "Specialist Agent" (Does a Task in a Sandbox)

Responsibilities:

- Use sandbox tools (create container, run commands, read/write files)
- Make small, verifiable progress
- Produce artifacts (diffs, reports, test results)

Constraints:

- Narrow toolset
- Tight time and file-scope limits
- Outputs must be reproducible (commands logged, env captured)

### Pattern C - "Policy Agent" (Optional)

If you want an LLM to assist with policy decisions, keep it advisory:

- It recommends allow/deny/ask-human with rationale
- The platform enforces policy deterministically

## Guardrails That Matter in Practice

### 1) Schema-First Tool Calling

Do not accept free-form "call this tool" text.

Require the model to output either:

- a tool_call object that matches schema, or
- a normal message (no ambiguous hybrid)

Server-side:

- Validate JSON schema
- Reject unknown fields
- Enforce allowlists (commands, paths, domains)
- Enforce timeouts and resource caps
- Require approvals for high-risk classes

### 2) Deterministic Policy Enforcement

Examples:

- No secrets in sandbox - enforced by network egress and secret broker, not by prompts
- Only allow git operations in a repo directory - enforced by path allowlist
- Only allow outbound HTTP through a secure browser tool - enforced by routing

### 3) Observability and Replay

Log every tool call with:

- input args (redacted)
- output (redacted)
- duration, exit code
- sandbox image hash, tool version
- files changed (hashes)
- correlation ids (session, task, tool)

This is how you debug agent behavior and regressions.

## How to Build Your First CyNodeAI-Compatible Agent

### Step 1 - Define One Skill With a Tight Scope

Example: "repo triage"

- Inputs: repo path, goal, constraints
- Tools: sandbox.exec, sandbox.read_file, sandbox.list_tree, git.diff
- Outputs: findings.json (risks, TODOs, recommended next tasks)

### Step 2 - Write the Agent Prompt Around Your Schemas

The system prompt should explicitly define:

- allowed tools
- when to call tools vs respond
- required JSON shape for tool calls
- termination criteria
- how to report errors (structured)

### Step 3 - Add Server-Side Validators

Enforce:

- command allowlist (git, go test, golangci-lint, etc.)
- max runtime and output size
- file path restrictions
- no network unless explicitly granted

### Step 4 - Add Evals Early

Minimum eval set:

- "tool call correctness" (schema-valid, arguments correct)
- "plan quality" (produces task list with statuses)
- "recovery" (handles tool failures, retries safely)
- "no policy violations" (attempted network, secrets, forbidden paths)

Run evals against multiple models and quantizations.

## What to Implement First in CyNodeAI to Support This Well

- A strict tool-call JSON protocol (single dialect)
- Skill packaging format (skill.yaml + schemas + validators)
- A policy engine that gates tools by risk level
- A task queue with idempotency keys and leases
- A structured state store (agent state JSON, artifact pointers)
- An eval harness that replays tool traces

## Minimal Example: Agent Loop Contract

A practical contract is a single JSON response type:

- type: "tool_call" | "message" | "final"
- tool_call: { name, arguments } (if tool_call)
- message: { text } (if message)
- final: { artifacts, summary, next_steps } (if final)
- state_patch: JSON Patch operations applied to stored agent state
