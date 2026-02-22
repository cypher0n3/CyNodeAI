# Proposal - Structured Work Management Model for CyNodeAI

## 1. Purpose

Introduce a structured project tracking model into CyNodeAI using modern work management constructs:

- Epics
- Features
- Stories
- Tasks
- Sub-tasks

The goal is to:

- Improve planning clarity
- Align AI-driven execution with industry-standard project management
- Provide traceability between objectives, work items, artifacts, and agent actions
- Preserve strict RBAC and audit guarantees

## 2. Design Principles

- Hierarchical but flexible
- Compatible with Agile and hybrid methodologies
- RBAC-aware at every layer
- API-first and machine-operable
- Traceable from Epic -> Artifact
- Agent-compatible (LLM planning must map to work items deterministically)
- Immutable history with append-only state transitions

### 2.1 Terminology: Agile Work Items vs CyNodeAI Execution

To avoid ambiguity in specs and implementation:

- **Agile work items (planning/PM layer):** Epic, Feature, Story, Task, Sub-task.
  These are units of planning, tracking, and value delivery.
  They live in the work-management hierarchy and are used by cynode-pm for decomposition and status.
- **CyNodeAI execution (orchestrator/worker layer):**
  - **Orchestrator task:** A row in the orchestrator `tasks` table; represents one execution run (workflow, request, or batch) and owns state such as status and acceptance_criteria.
  - **Job:** A row in the `jobs` table; the unit of work dispatched to a worker node (sandbox execution, e.g. one `job.json` or skill invocation).
  One orchestrator task may have many jobs.
- **Mapping:** An agile **Sub-task** (work item) typically corresponds to one CyNodeAI **Job** (sandbox execution); jobs reference the agile work item (e.g. `subtask_id`, `story_id`) for traceability.
  Agile **Task** and **Story** are parent work items and do not denote orchestrator tasks or jobs.

Where context could be unclear, use "agile task" or "work-item task" vs "orchestrator task", and "agile Sub-task" vs "Job".
Canonical definitions for the `tasks` and `jobs` tables: [postgres_schema.md](../tech_specs/postgres_schema.md) (Tasks, Jobs, and Nodes).

## 3. Work Item Hierarchy

The following hierarchy and definitions apply to **agile work items** only.

### 3.1 Hierarchy Overview

Epic
-> Feature
-> Story
-> Task
-> Sub-task

Not all levels are mandatory.
Small projects may start at Story level.

### 3.2 Term Definitions

Each level in the hierarchy is defined below.

#### 3.2.1 Epic Level

- Large initiative aligned to strategic objective
- Spans multiple features
- May span multiple sprints
- Linked to high-level objective in cynode-pm

Example:

- "Implement secure multi-node execution framework"

#### 3.2.2 Feature Level

- Deliverable capability within an Epic
- Produces measurable outcome
- Deployable increment if possible

Example:

- "Sandbox runner with deterministic execution contract"

#### 3.2.3 Story Level

- User- or system-focused unit of value
- Sized for sprint-level execution
- Testable and verifiable

Example:

- "As a PM agent, I can generate job specs validated against schema"

#### 3.2.4 Task Level (Agile Work Item)

- Technical implementation unit within a Story
- Directly executable by sandbox or engineer
- Often mapped to a skill
- Not to be confused with the orchestrator `tasks` table (execution run)

Example:

- "Implement JSON schema validation in cynode-agent"

#### 3.2.5 Sub-Task Level (Agile Work Item)

- Atomic step within an agile Task
- Typically deterministic
- Suitable for single sandbox execution; typically maps 1:1 to one CyNodeAI Job

## 4. Data Model Proposal

Core tables and shared fields are described below.

### 4.1 Core Tables

- epics
- features
- stories
- tasks
- subtasks
- work_item_links
- work_item_events

### 4.2 Shared Fields

All work items must include:

- id (UUID)
- tenant_id
- project_id
- title
- description
- status
- priority
- estimate_points
- created_at
- updated_at
- created_by
- assigned_to
- parent_id
- type (epic, feature, story, task, subtask)
- sensitivity_level

### 4.3 Status Model

Recommended workflow states:

Backlog
-> Ready
-> In Progress
-> Blocked
-> Review
-> Done
-> Archived

Status transitions must be logged as events.

## 5. Relationship to Agents

Agents integrate with work items as follows.

### 5.1 CyNode-Pm Integration

cynode-pm becomes work-item aware.

When receiving an objective:

- Create Epic if needed
- Break into Features
- Decompose into Stories
- Generate Tasks linked to Stories
- Generate Sub-tasks mapped to sandbox jobs

The PM agent must output:

```json
{
  "type": "work_item_update",
  "payload": {
    "create": [...],
    "update": [...],
    "link": [...]
  }
}
```

### 5.2 Sandbox Integration

Agile Sub-tasks (work items) correspond directly to CyNodeAI Jobs:

- One agile Sub-task typically drives one Job (job.json, skill invocation).
- Each Job must reference the originating agile work item: subtask_id, task_id, story_id.
- Job completion updates the linked agile Sub-task status automatically.

## 6. Traceability Model

Every artifact must reference the agile work item:

- subtask_id
- task_id
- story_id

Every CyNodeAI Job must reference:

- the agile work item (subtask_id and parent ids)
- skill_id where applicable

This ensures:

Objective -> Epic -> Feature -> Story -> (agile) Task -> (agile) Sub-task -> Job -> Artifact

Full lineage is preserved (agile work items to execution Jobs to artifacts).

## 7. RBAC Integration

Work items are subject to RBAC as follows.

### 7.1 Access Control Rules

Work items are scoped by:

- tenant_id
- project_id
- sensitivity_level

Users can:

- View only work items within authorized projects
- Modify only items allowed by role
- Change status only if role permits

### 7.2 Role Examples

PM Role:

- Create epics/features/stories
- Assign tasks
- Close stories

Developer Role:

- Update tasks/subtasks
- Upload artifacts
- Cannot close epics

Analyst Role:

- Read-only except comments
- Can propose new stories

All access enforced server-side.

## 8. Agile Best Practices Alignment

Sprint support, estimation, and definition of done are described below.

### 8.1 Sprint Support

Optional sprint table:

- sprint_id
- project_id
- start_date
- end_date
- goal
- status

Stories and tasks may be assigned to sprints.

### 8.2 Estimation Practices

- Story points at Story level
- Hour estimates at Task level
- Track actual duration via sandbox job metrics

Supports velocity calculation.

### 8.3 Definition of Done

Each Story must define:

- Acceptance criteria (structured field)
- Required artifacts
- Required validations (tests, lint, review)

Story cannot move to Done unless criteria met.

## 9. Agent-Driven Planning Rules

To align with modern PM best practices:

- No direct Task creation without Story context
- No Sub-task without Task
- All work items must have acceptance criteria
- Sub-tasks must be independently executable
- Stories must produce verifiable outcomes

## 10. Automation Hooks

Automation may update status and create events.

### 10.1 Auto-Status Updates

When:

- All subtasks complete -> Task moves to Review
- All tasks complete -> Story moves to Done (pending validation)
- Failed subtask -> Task moves to Blocked

### 10.2 Policy Hooks

Require approval when:

- Moving Story to Done
- Closing Epic
- Creating high-sensitivity work item

## 11. Integration With Pgvector

Work items can be embedded selectively:

- Story descriptions
- Acceptance criteria
- Retrospective summaries

Not embedded:

- Internal comments by default
- Raw logs

Retrieval scope limited by project and RBAC.

## 12. Audit and Event Model

All work item changes must emit events:

- Created
- Updated
- StatusChanged
- Assigned
- Linked
- Archived

Events must be immutable.

This supports:

- Timeline reconstruction
- Metrics
- Agent evaluation

## 13. API Surface

Expose:

- CRUD endpoints for work items
- Status transition endpoint (validated)
- Hierarchy queries
- Dependency graph queries
- Artifact linkage endpoints

All API calls must enforce RBAC and sensitivity checks.

## 14. MVP Scope

MVP includes:

- Epic, Story, Task, Sub-task
- Status tracking
- Parent-child relationships
- Artifact linking
- PM-agent integration
- Basic sprint support (optional)

Defer:

- Cross-project epics
- Complex dependency graphs
- Advanced portfolio analytics

## 15. Acceptance Criteria

- PM agent decomposes objective into structured work items
- Sandbox jobs update subtask status automatically
- Work item hierarchy is queryable and auditable
- RBAC enforcement prevents cross-project visibility
- Artifacts trace back to originating work item
- Stories cannot be marked Done without acceptance criteria validation

## 16. Architectural Outcome

This proposal aligns CyNodeAI with:

- Agile and hybrid best practices
- Clear value-based planning
- Traceable AI-driven execution
- Deterministic sandbox operations
- Secure multi-tenant boundaries

It transforms CyNodeAI from a task executor into a structured AI-native project management system where planning, execution, and auditability are unified under modern governance standards.
