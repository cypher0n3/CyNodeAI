# CyNode-PM - Project Manager Agent Technical Specification

## 1. Purpose

`cynode-pm` is the control-plane AI agent responsible for:

- Goal decomposition
- Task planning and prioritization
- Skill selection
- Job specification generation
- Dispatching work to sandbox workers
- Aggregating results
- Producing structured deliverables

It does not execute arbitrary code directly.
It does not run inside container sandboxes.
It operates within the trusted host/control-plane environment.

---

## 2. Responsibilities

Core functions and non-responsibilities are listed below.

### 2.1 Core Functions

- Maintain project-level agent state
- Translate objectives into executable tasks
- Select appropriate skills
- Generate deterministic `job.json` specs
- Submit jobs to Worker API
- Monitor job execution
- Process structured results
- Update plan and state
- Produce final artifacts

### 2.2 Explicit Non-Responsibilities

- Direct file system manipulation
- Running shell commands
- Managing container lifecycle
- Storing secrets locally
- Making policy decisions beyond declared constraints

---

## 3. Architecture

High-level position and components are described below.

### 3.1 High-Level Position

```text
User
  ↓
cynode-pm
  ↓
Worker API
  ↓
Sandbox (cynode-agent)
```

### 3.2 Component Overview

- LLM Interface Layer
- State Manager
- Skill Registry
- Task Planner
- Job Builder
- Result Processor
- Policy Gate Client
- Observability & Audit Client

---

## 4. Execution Model

The agent follows a loop that loads state, plans, and dispatches jobs.

### 4.1 Agent Loop

1. Load session state
2. Evaluate objective and current plan
3. Determine next action:

   - Generate job
   - Ask clarification
   - Produce final output
4. Submit job (if required)
5. Wait for structured result
6. Update state
7. Repeat or terminate

### 4.2 Deterministic Output Contract

LLM must emit one of:

```json
{
  "type": "plan_update | job_request | message | final",
  "state_patch": [],
  "payload": {}
}
```

The platform validates this before execution.

---

## 5. State Model

State is persisted and updated according to the following rules.

### 5.1 Persisted State

```json
{
  "session_id": "uuid",
  "objective": "string",
  "constraints": {},
  "plan": [
    {
      "task_id": "uuid",
      "description": "string",
      "status": "pending | running | done | blocked"
    }
  ],
  "artifacts": [],
  "active_job_id": null,
  "history": [],
  "skill_context": {}
}
```

### 5.2 State Rules

- Objective immutable after creation
- Plan entries versioned
- Artifacts referenced by hash and storage location
- No raw transcript persistence required

---

## 6. Skill Integration

Skills are registered and selected as follows.

### 6.1 Skill Registry

Skills are versioned packages containing:

- skill_id
- version
- input schema
- job template generator
- allowed tools
- policy metadata

### 6.2 Skill Selection

`cynode-pm` selects skills based on:

- Objective classification
- Task type
- Constraints
- Available sandbox capabilities

Skill selection must be explicit in job_request payload.

---

## 7. Job Specification Generation

The PM agent generates job specs and validates them before dispatch.

### 7.1 Job Builder Responsibilities

- Generate deterministic `job.json`
- Populate constraints
- Define step sequence
- Ensure command allowlists are respected
- Embed resource limits

### 7.2 Validation Rules

Before dispatch:

- Validate against skill schema
- Validate against global policy
- Validate command/path/network constraints

Failure results in:

```json
{
  "type": "error",
  "reason": "policy_violation"
}
```

---

## 8. LLM Interface

The LLM must satisfy the following requirements and prompt structure.

### 8.1 LLM Requirements

- Structured JSON output only
- Schema-constrained responses
- No free-form tool calls
- Strict role separation (system, developer, user)

### 8.2 Prompt Structure

System prompt must define:

- Output contract
- Allowed response types
- Plan management rules
- When to terminate
- When to request clarification

---

## 9. Policy Interaction

`cynode-pm` does not enforce policy directly.

It queries:

- Policy Engine
- Skill metadata
- Tenant configuration

Policy violations must be surfaced before job dispatch.

---

## 10. Result Processing

Results from the sandbox are processed as follows.

### 10.1 Expected Input

`result.json` from sandbox agent:

- Step results
- Exit codes
- Artifacts
- Resource metrics

### 10.2 Result Handler Responsibilities

- Verify integrity (hashes)
- Attach artifacts to session state
- Update plan status
- Detect failure patterns
- Decide next action

---

## 11. Concurrency Model

Single-session and multi-session behavior are described below.

### 11.1 Single-Session Behavior

- One active job per session (MVP)

### 11.2 Multi-Session Behavior

- Stateless workers
- State stored in central datastore
- Idempotency keys for job dispatch

---

## 12. Security Model

Secrets and isolation are handled as follows.

### 12.1 Secrets Handling

- No secret storage in memory beyond request scope
- Secrets accessed only through brokered APIs
- Never embedded in job.json

### 12.2 Process Isolation

- Runs outside sandbox
- Uses least-privilege service account
- No direct filesystem access to sandbox volumes

---

## 13. Observability

Logging and trace correlation are required as follows.

### 13.1 Required Logging

- Session lifecycle events
- Plan updates
- Job dispatch and completion
- Policy rejections
- LLM output validation failures

### 13.2 Trace Correlation

All logs must include:

- session_id
- task_id
- job_id
- skill_id

---

## 14. Failure Handling

Failure types and recovery are described below.

### 14.1 Failure Types

- schema_error
- model_contract_violation
- policy_violation
- worker_failure
- sandbox_timeout
- artifact_integrity_failure

### 14.2 Recovery Strategy

- Retry only if idempotent
- Escalate to user if blocked
- Mark task as failed after max attempts

---

## 15. Configuration

Runtime flags and environment variables are described below.

### 15.1 Runtime Flags

- --model-provider
- --model-name
- --max-tokens
- --temperature
- --skill-dir
- --policy-endpoint
- --worker-endpoint
- --datastore-endpoint

### 15.2 Environment Variables

- CYNODE_MODEL_API_KEY
- CYNODE_TENANT_ID
- CYNODE_LOG_LEVEL

---

## 16. Versioning

- Semantic versioning required
- Protocol version separate from binary version
- Backward compatibility maintained within major version

---

## 17. MVP Scope

Initial release should support:

- Single objective session
- Linear task plan
- Single skill invocation at a time
- Structured job dispatch
- Artifact aggregation
- Deterministic state updates

No parallel planning or autonomous branching in MVP.

---

## 18. Long-Term Extensions

- Parallel task execution
- Multi-agent collaboration
- Cross-session knowledge store
- Reinforcement feedback loop
- Skill auto-discovery
- Cost-based model routing

---

## 19. Architectural Outcome

This design ensures:

- Clear separation between planning and execution
- Deterministic sandbox operations
- Auditable decision flow
- Replaceable model backend
- Skill-driven extensibility

The PM agent remains the cognitive layer, while execution and policy remain deterministic and enforceable by the platform.
