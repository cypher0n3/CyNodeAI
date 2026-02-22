# Proposal Addendum - Requirements and Acceptance Criteria Model for CyNodeAI

## 1. Purpose

Define how **Requirements** and **Acceptance Criteria** integrate into the CyNodeAI structured work model while aligning with:

- Modern Agile and hybrid best practices
- Testability and traceability
- AI-executable workflows
- Strict RBAC and audit controls

## 2. Why Formal Requirements Matter

Without explicit requirements:

- AI planning drifts
- "Done" becomes ambiguous
- Regression detection is weak
- Traceability to artifacts is incomplete

Requirements must be:

- Structured
- Versioned
- Testable
- Linked to work items
- Linked to validation artifacts

## 3. Hierarchical Model With Requirements

Updated hierarchy:

Epic
-> Feature
-> Story
-> Requirement
-> Acceptance Criteria
-> Task
-> Sub-task

Key distinction:

- **Requirements define what must be true**
- **Tasks define how we make it true**

## 4. Requirement Types

### 4.1 Functional Requirement (FR)

Defines observable behavior.

Example:

- "The sandbox runner shall reject commands not in the allowlist."

### 4.2 Non-Functional Requirement (NFR)

Defines quality attributes.

Example:

- "Vector retrieval latency must be under 200ms p95."

### 4.3 Security Requirement (SR)

Defines access, isolation, or compliance controls.

Example:

- "Vector queries must enforce tenant and project scoping prior to similarity ranking."

### 4.4 Operational Requirement (OR)

Defines deployment, monitoring, or maintenance behavior.

Example:

- "Embeddings must be rebuildable after model upgrade."

## 5. Data Model Additions

### 5.1 Requirements Table

Fields:

- requirement_id (UUID)
- story_id (FK)
- tenant_id
- project_id
- type (FR, NFR, SR, OR)
- identifier (e.g., REQ-PM-001)
- title
- description
- priority
- sensitivity_level
- version
- status (draft, approved, implemented, verified)
- created_at
- updated_at

### 5.2 Acceptance_criteria_criteria Table

Fields:

- criteria_id (UUID)
- requirement_id (FK)
- description
- validation_type (manual, automated, sandbox_test, metric_check)
- required_artifact_type
- status (pending, validated, failed)

Each requirement must have at least one acceptance criterion.

## 6. Requirement Identifiers

Adopt stable, human-readable identifiers:

Examples:

- REQ-PM-001
- REQ-SEC-005
- REQ-RBAC-012
- REQ-VEC-003

Identifier format:

REQ-<DOMAIN>-<NUMBER>

Identifiers are immutable once approved.

## 7. Relationship to Work Items

### 7.1 Story-Level Association

Stories must reference one or more requirements.

Stories are considered complete only when:

- All linked requirements are verified.

### 7.2 Task Mapping

Tasks must reference:

- requirement_id

This ensures traceability from execution to requirement.

### 7.3 Sub-Task Mapping

Sub-tasks optionally reference:

- specific acceptance_criteria_id

Particularly when they correspond to:

- test implementation
- validation run
- metrics measurement

## 8. Acceptance Criteria Best Practices

Acceptance criteria must be:

- Objective
- Measurable
- Testable
- Unambiguous

Preferred format: structured rather than prose.

Example:

Instead of:

"The query must be fast."

Use:

"Vector similarity query p95 latency <= 200ms under load of 100 QPS."

## 9. Validation Mechanisms

### 9.1 Automated Validation

- Sandbox job executes test suite
- Metric job measures performance
- Schema validation job checks contract

Results must produce:

- artifact (report.json)
- pass/fail status
- timestamp
- linked requirement_id

### 9.2 Manual Validation

Allowed only if:

- validation_type = manual
- Requires approval role
- Audit event recorded

## 10. Integration With Cynode-Pm

### 10.1 Planning Rules

cynode-pm must:

- Generate requirements before generating tasks for new features
- Associate tasks with requirements
- Verify acceptance criteria status before marking Story Done

### 10.2 Completion Rules

Story cannot transition to Done unless:

- All linked requirements.status = verified
- All acceptance_criteria.status = validated

If any fail:

- Story -> Blocked

## 11. Traceability Chain

Full traceability must support:

Objective
-> Epic
-> Feature
-> Story
-> Requirement
-> Acceptance Criteria
-> Task
-> Sub-task
-> Job
-> Artifact
-> Verification Event

This chain must be queryable.

## 12. RBAC Integration

### 12.1 Requirement Visibility

Requirements are scoped by:

- tenant_id
- project_id
- sensitivity_level

Users may:

- View requirements within authorized scope
- Modify only if role permits

### 12.2 Approval Roles

Requirement status transitions:

draft -> approved requires PM role
implemented -> verified requires reviewer role

Separation of duties is recommended for high-sensitivity projects.

## 13. Versioning Model

Requirements must be versioned.

If modified after approval:

- New version created
- Previous version archived
- Linked tasks flagged for re-validation

This prevents silent scope drift.

## 14. Integration With Pgvector

Embed:

- Requirement descriptions
- Acceptance criteria
- Verified summaries

Do not embed:

- Raw compliance reports
- Sensitive security controls without proper tagging

Retrieval must respect:

- project scope
- sensitivity
- role access

## 15. Metrics and Reporting

System should support:

- Requirement coverage (tasks per requirement)
- Verification coverage (validated vs pending)
- Failed validation counts
- Cycle time per requirement
- Requirement churn (version updates)

## 16. MVP Scope

MVP must include:

- Requirement entity
- Acceptance criteria entity
- Story linkage
- Validation status tracking
- Blocking rule for Story completion
- Audit events for status changes

Defer:

- Advanced compliance templates
- Cross-project requirement reuse
- Regulatory export tooling

## 17. Acceptance Criteria for This Feature

The requirements system is considered implemented when:

- Stories cannot be marked Done without verified requirements
- Every sandbox job can reference a requirement
- Audit trail reconstructs requirement lifecycle
- RBAC prevents unauthorized modification
- Requirement versions are immutable once approved

## 18. Architectural Outcome

This elevates CyNodeAI from:

AI-driven task execution

to:

AI-governed, requirement-traceable delivery platform

Where:

- Planning is structured
- Execution is deterministic
- Validation is measurable
- Completion is defensible
- Security boundaries remain intact
