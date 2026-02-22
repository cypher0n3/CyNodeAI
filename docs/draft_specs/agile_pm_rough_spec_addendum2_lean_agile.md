# Lean Agile Amendment - Flow, WIP, and Value Optimization Layer for CyNodeAI

## 1. Purpose

This amendment extends the CyNodeAI project management architecture to fully align with Lean Agile principles by introducing:

- Explicit flow control
- Work-in-progress (WIP) enforcement
- Value-based prioritization
- Pull-based execution
- Flow metrics
- Continuous improvement loops

This amendment does not replace the structured work model.
It strengthens it with operational Lean controls.

### 1.1 Terminology: Agile Work Items vs CyNodeAI Execution

- **Agile work items:** Epic, Feature, Story, Task, Sub-task (planning/PM layer).
- **CyNodeAI execution:** **Orchestrator task** = one run in the `tasks` table; **Job** = one sandbox dispatch in the `jobs` table.
  An agile **Sub-task** typically maps to one **Job**.
Where the doc says "task" or "sub-task" in a WIP or flow context, it means the agile work item unless stated otherwise.
Canonical table definitions: [postgres_schema.md](../tech_specs/postgres_schema.md) (Tasks, Jobs, and Nodes).

## 2. Guiding Lean Principles

This amendment operationalizes the following Lean principles:

- Deliver value in small batches
- Limit work in progress
- Optimize flow, not utilization
- Build quality in
- Use pull-based execution
- Improve continuously

## 3. Explicit Flow Model

Work items follow a defined state machine with enforced transitions.

### 3.1 Work State Flow

All Stories and Tasks must follow a standardized flow:

Backlog
-> Ready
-> In Progress
-> Review
-> Done

Optional states:

Blocked
Archived

State transitions must:

- Be event-logged
- Respect WIP limits
- Trigger flow metric updates

---

## 4. Work-in-Progress (WIP) Limits

WIP limits cap concurrent work at multiple levels to preserve flow.

### 4.1 WIP Control Requirements

WIP limits must be enforced at:

- Story level per project (agile work items)
- Task level per story (agile work items)
- Sub-task / Job level per agent session (agile Sub-tasks and the CyNodeAI Jobs they spawn)
- Sandbox execution concurrency per tenant (Jobs)

### 4.2 Configuration Fields

Per project:

- max_active_stories
- max_active_tasks_per_story
- max_parallel_jobs
- max_agent_sessions

Per tenant:

- max_total_parallel_jobs
- max_total_active_projects

### 4.3 Enforcement Rules

If a WIP limit is reached:

- New work cannot transition to In Progress
- cynode-pm must prioritize completion over decomposition
- System returns structured error:

  - wip_limit_exceeded

### 4.4 Lean Rationale

This prevents:

- Over-decomposition by AI
- Excessive parallel sandbox execution
- Context thrashing
- Hidden bottlenecks

---

## 5. Pull-Based Execution Model

Work is pulled into progress only when capacity and limits allow.

### 5.1 Pull vs Push

Work must move to In Progress only when:

- Capacity exists
- WIP limits allow
- Dependencies are resolved

### 5.2 Agent Behavior Modification

cynode-pm must:

- Check capacity before generating new agile Sub-tasks (and thus Jobs)
- Prefer completing existing agile tasks over spawning new work
- Avoid speculative parallelization

### 5.3 Sandbox Pull Integration

Workers may optionally implement:

- Pull-based job consumption from queue
- Lease-based execution model
- Capacity-aware scheduling

---

## 6. Value-Based Prioritization

Stories are ordered by value signals and optional WSJF.

### 6.1 Required Value Fields (Feature/Story Level)

Add fields:

- business_value_score (1-10)
- time_criticality_score (1-10)
- risk_reduction_score (1-10)
- effort_estimate_points

### 6.2 Optional WSJF Calculation

Weighted Shortest Job First:

WSJF = (Business Value + Time Criticality + Risk Reduction) / Effort

This can be:

- Calculated automatically
- Used as a ranking signal for cynode-pm planning

### 6.3 Planning Rule

When selecting next Story to execute:

- Prefer highest WSJF
- Break ties using priority and dependency resolution

This ensures Lean value optimization without full SAFe overhead.

---

## 7. Flow Metrics

System computes lead time, cycle time, throughput, and WIP by state.

### 7.1 Required Metrics

System must calculate:

- Lead Time

  - Story creation -> Done
- Cycle Time

  - In Progress -> Done
- Throughput

  - Stories completed per sprint
- WIP Count per state
- Blocked time duration

### 7.2 Agent Awareness

cynode-pm should optionally use:

- Cycle time history to estimate completion
- Throughput to adjust decomposition granularity
- Blocked ratio to detect systemic issues

### 7.3 Visibility and Reporting

Expose metrics via:

- API endpoints
- Dashboard
- Audit reports

---

## 8. Continuous Improvement Loop

Retrospectives and recommendations feed back into process and policy.

### 8.1 Retrospective Data Capture

After Story completion:

- Record actual cycle time
- Record failed validations
- Record rework count
- Record requirement version churn

### 8.2 Improvement Suggestions

Optionally, cynode-pm may:

- Propose process improvements
- Suggest reducing WIP
- Recommend adjusting estimates
- Identify recurring validation failures

### 8.3 Change Governance

Retrospective recommendations require:

- Human approval for policy changes
- Logged change events

---

## 9. Dependency Management

Explicit dependencies block progress until predecessors are done.

### 9.1 Explicit Dependency Fields

Work items may include:

- depends_on_ids
- blocks_ids

### 9.2 Flow Enforcement

A work item cannot move to In Progress if:

- Any dependency is not Done

This preserves flow integrity.

---

## 10. Capacity Awareness

System tracks load and adapts planning to avoid over-utilization.

### 10.1 Capacity Tracking

Track:

- Active agent sessions
- Running sandbox jobs
- CPU and memory utilization
- Queue depth

### 10.2 Adaptive Planning

If system load exceeds threshold:

- Reduce parallel sub-task generation
- Defer non-critical stories
- Focus on closing near-complete work

This aligns with Lean's flow-over-utilization philosophy.

---

## 11. Governance Enhancements

Stop-the-line and root-cause logging protect quality and reproducibility.

### 11.1 Stop-the-Line Principle

If:

- Validation failure rate exceeds threshold
- Security requirement fails
- Sandbox reproducibility breaks

Then:

- Automatically pause new work generation
- Escalate to Blocked state at Feature level

### 11.2 Root Cause Logging

All systemic failures must:

- Emit structured event
- Be linked to affected requirements
- Be reviewable

---

## 12. Alignment Summary

- **Lean Principle:** Limit WIP
  - amendment coverage: Explicit limits + enforcement
- **Lean Principle:** Optimize flow
  - amendment coverage: Flow states + metrics
- **Lean Principle:** Deliver small batches
  - amendment coverage: Agile Sub-task to Job mapping
- **Lean Principle:** Build quality in
  - amendment coverage: Requirement validation gating
- **Lean Principle:** Pull system
  - amendment coverage: Capacity-aware transitions
- **Lean Principle:** Continuous improvement
  - amendment coverage: Retrospective data + analytics
- **Lean Principle:** Value-based prioritization
  - amendment coverage: WSJF-style scoring

This closes previous Lean alignment gaps.

---

## 13. MVP for Lean Amendment

Minimum additions required:

- WIP configuration fields
- Enforcement logic in status transitions
- Basic flow metrics (lead time, cycle time)
- Value scoring fields
- Dependency enforcement

Defer:

- Automatic WSJF ranking
- Advanced load-based adaptive decomposition
- Automated process recommendations

---

## 14. Architectural Outcome

With this amendment:

CyNodeAI becomes:

- Lean flow-aware
- WIP-controlled
- Value-prioritized
- Quality-enforced
- Capacity-aware

It moves from:

Structured work tracking

to:

Lean-optimized AI-native delivery system

The platform now supports:

- Deterministic execution
- Measurable flow
- Explicit value prioritization
- Controlled parallelism
- Continuous improvement

without introducing heavy framework overhead.
