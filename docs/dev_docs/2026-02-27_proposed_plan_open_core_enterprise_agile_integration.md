# Proposed Plan: Open-Core Backend With Enterprise Lean Agile Integration

- [1. Purpose](#1-purpose)
- [2. Scope Split: Open Source vs Enterprise](#2-scope-split-open-source-vs-enterprise)
- [3. Architectural Approach](#3-architectural-approach)
- [4. Data Model and Schema Strategy](#4-data-model-and-schema-strategy)
- [5. API and Gateway Strategy](#5-api-and-gateway-strategy)
- [6. Build, Distribution, and Licensing](#6-build-distribution-and-licensing)
- [7. Implementation Phases](#7-implementation-phases)
- [8. Traceability to Source Specs](#8-traceability-to-source-specs)

## 1. Purpose

**Date:** 2026-02-27.
**Type:** Proposed plan (docs-only).
**Audience:** Product and engineering.

This document proposes how to build out the CyNodeAI backend so that:

- **Open source** delivers only **projects** and **tasks** (orchestrator tasks and jobs): project plans at project scope, plan lock, PMA building/refining plans and clarifying before execution, Markdown storage, client-editable plans.
  No Epic, Feature, Story, (agile) Task, or (agile) Sub-task in the open codebase; no requirements/acceptance criteria entities.
- **Proprietary (enterprise)** delivers **all other agile constructs**: Epic, Feature, Story, (agile) Task, (agile) Sub-task; epic-as-sub-plan (same plan constructs at epic scope); requirements and acceptance criteria; plus the **Lean Agile** layer (flow, WIP, value/WSJF, metrics, pull-based execution, etc.) from [agile_pm_rough_spec_addendum2_lean_agile.md](../draft_specs/agile_pm_rough_spec_addendum2_lean_agile.md).
  Enterprise code integrates for paid customers only.

No code or normative spec edits are prescribed here; this is a proposed plan for future implementation and spec promotion.

## 2. Scope Split: Open Source vs Enterprise

Features are split so the open codebase remains fully useful without any proprietary code.

### 2.1 Open Source (This Repository, Normative Specs)

- **Projects and tasks only:** Project plans (task set + execution order at project scope), at most one plan per project, default project as catch-all, PMA builds/refines plans and prefers clarification before execution, Markdown storage for plan/task content, client-editable plans (Web Console, CLI, API), plan lock (plan document only; when locked, users may still edit tasks and agents may update status/comments only); RBAC for lock/unlock on shared (group) plans.
  Source: [2026-02-27_recommendations_tasks_projects_pma_spec_updates.md](2026-02-27_recommendations_tasks_projects_pma_spec_updates.md).
- **No agile work items in open source:** The open codebase does NOT include Epic, Feature, Story, (agile) Task, or (agile) Sub-task; no requirements or acceptance criteria entities.
  Orchestrator tasks and jobs remain the only execution and planning units in open source.
- **APIs and clients (open):** CRUD and status transitions for projects, project plans, tasks, and jobs only; RBAC and audit as already specified.

### 2.2 Enterprise (Proprietary, Separate Codebase or Module)

All agile constructs aside from projects and (orchestrator) tasks are enterprise-only.

- **Structured work hierarchy:** Epic, Feature, Story, (agile) Task, (agile) Sub-task; work item tables and status model; mapping of agile Sub-task to CyNodeAI Job; PM agent decomposition; traceability to artifacts.
  Source: [agile_pm_rough_spec.md](../draft_specs/agile_pm_rough_spec.md).
- **Epic-as-sub-plan:** Epics use the **same plan constructs** as project plans: an epic is a sub-plan (ordered set of work items under the epic) with the same semantics (execution order, plan document/body, optional lock, Markdown, client-editable); at most one plan per epic.
  Source: recommendations doc and [agile_pm_rough_spec.md](../draft_specs/agile_pm_rough_spec.md).
- **Requirements and acceptance criteria:** Requirement entity, acceptance criteria entity, Story linkage, validation status, gating of Story completion on verified requirements (MVP scope).
  Source: [agile_pm_rough_spec_addendum.md](../draft_specs/agile_pm_rough_spec_addendum.md).
- **Lean Agile layer:** All content of [agile_pm_rough_spec_addendum2_lean_agile.md](../draft_specs/agile_pm_rough_spec_addendum2_lean_agile.md):
  - Explicit flow model and state machine (Backlog -> Ready -> In Progress -> Review -> Done; Blocked, Archived).
  - WIP limits (per project, per story, per tenant) and enforcement (wip_limit_exceeded).
  - Pull-based execution and capacity-aware transitions.
  - Value-based prioritization and WSJF-style fields and calculation.
  - Flow metrics (lead time, cycle time, throughput, WIP by state, blocked duration).
  - Continuous improvement (retrospective data, improvement suggestions, change governance).
  - Dependency management (depends_on_ids, blocks_ids) and flow enforcement.
  - Capacity awareness and adaptive planning.
  - Governance (stop-the-line, root-cause logging).
- **Enterprise-only APIs:** Endpoints for WIP config, flow metrics, value scores, retrospectives, and any policy hooks that depend on Lean behavior.
- **Enterprise PM agent behavior:** cynode-pm extensions that respect WIP, use WSJF for ordering, prefer pull over push, and consume flow metrics (when the enterprise module is present and licensed).
- **Proper linkages across the hierarchy (enterprise requirement):** The enterprise version MUST support and enforce **proper linkages** between project plans, epics, stories, tasks, subtasks, and jobs.
  This includes: referential integrity (FKs and constraints) so that work items and jobs consistently reference their parent plan/epic/story/task; APIs to create, read, update, and traverse the full chain (project plan -> epic(s) -> stories -> tasks -> subtasks -> jobs); traceability and lineage queries; and consistent behavior for plan lock, WIP, and flow so that locking or limiting at one level respects the hierarchy.
  Enterprise owns the full agile schema and linkage; open source provides only projects, project_plans, tasks, and jobs.

## 3. Architectural Approach

The open code defines extension points; enterprise code implements them in a separate codebase.

### 3.1 Extension Points in Open Code

The open codebase should define **interfaces** (or equivalent contracts) that enterprise code can implement, without the open repo containing any Lean-specific logic:

- **Flow / WIP interface:** e.g. `FlowController` (or similar): check whether a transition to In Progress is allowed, enforce WIP limits, return structured errors.
  Open code may call this when transitioning tasks (or work items when enterprise is present); open implementation is a no-op or always-allows stub.
  Enterprise implements work-item-aware WIP enforcement.
- **Prioritization interface:** Optional hook for "order items for next execution" (tasks in open scope; work items in enterprise scope).
  Open code uses default ordering (e.g. priority, created_at).
  Enterprise implements WSJF or value-based ordering for work items.
- **Metrics interface:** Optional hook to record and expose flow metrics (lead time, cycle time, throughput, WIP).
  Open code may expose minimal or no flow metrics.
  Enterprise implements full metrics and dashboards for work items.
- **Capacity interface:** Optional hook for capacity-aware planning (e.g. "can accept more work?").
  Open code may ignore.
  Enterprise uses it for pull-based behavior.

Interfaces should live in the orchestrator (or a shared internal package) and be injectable at startup or via a registered provider.
Enterprise builds can then supply real implementations.

### 3.2 Where Enterprise Code Lives

- **Option A (recommended for clarity):** Separate **proprietary repository** that depends on the open CyNodeAI repo (e.g. Go module that imports orchestrator and implements the interfaces; or a separate service that the orchestrator calls via defined API).
- **Option B:** **Build-tag or compile-time variant** in a private fork or "enterprise" subtree that is only compiled when a flag is set (e.g. `enterprise` build tag).
  The open repo never contains enterprise code.
  The enterprise repo or vendor bundle adds files that implement the interfaces and are built only in enterprise binaries.
- **Option C:** **Plugin or sidecar:** Enterprise features live in a separate process or plugin that the orchestrator calls (e.g. over a local API).
  Orchestrator exposes extension points as outbound calls.
  Plugin implements WIP, metrics, and value logic.

Recommendation: **Option A** (separate repo importing open code) or **Option B** (build tag in a single private repo that vendors open code).
Both keep proprietary code out of the public repository and allow clean licensing.

### 3.3 Integration for Paid Customers

- **License or feature flag:** At startup or per-tenant, the system checks whether the enterprise (agile + Lean) module is enabled (e.g. license file, env var, or tenant feature flag in DB).
  If disabled, work item APIs are unavailable (404/501 or feature_not_licensed), and all flow/WIP/prioritization/metrics interfaces use no-op implementations.
- **API visibility:** Enterprise-only REST endpoints are registered only when the enterprise module is present and enabled; otherwise they return 404 or 501.
  Open API docs list them as optional or enterprise.
- **Data model:** See section 4.

## 4. Data Model and Schema Strategy

Open schema lives in this repo; enterprise adds its own migrations when present.

### 4.1 Open Schema (This Repo, Migrations in Open Repo)

- **Projects, project_plans, tasks, jobs only:** As in recommendations and existing/postgres schema (project plan as first-class or task ordinal; at most one plan per project; Markdown fields; plan lock state).
  No work item tables (epics, features, stories, agile tasks, subtasks), no requirements or acceptance_criteria tables in the open repo.

Open schema is sufficient for project-scoped planning and execution (orchestrator tasks and jobs).

Enterprise code adds all agile tables in its own migrations (see below) and may reference open `projects`, `project_plans`, `tasks`, and `jobs` (e.g. job -> task -> project; enterprise adds job -> subtask -> story -> feature -> epic when work items exist).

### 4.2 Enterprise-Only Schema (Proprietary Migrations)

Enterprise migrations (in the proprietary repo or applied only in enterprise builds) add all agile constructs not in open source:

- **Work items:** epics, features, stories, tasks (agile), subtasks (agile), work_item_links, work_item_events per [agile_pm_rough_spec.md](../draft_specs/agile_pm_rough_spec.md).
  Work items reference open `projects` and optionally open `project_plans`; jobs may reference subtask/story/feature/epic for traceability when enterprise is enabled.
- **Epic plans (sub-plans):** Plan constructs at epic scope (e.g. epic_plans or unified plans table with scope_type = epic): ordered work set, plan document/body, optional lock, same semantics as project plans.
- **Requirements and acceptance criteria:** requirements, acceptance_criteria (or equivalent) tables per [agile_pm_rough_spec_addendum.md](../draft_specs/agile_pm_rough_spec_addendum.md) MVP; Story linkage and validation gating.
- **WIP and flow config:** e.g. project-level or tenant-level WIP limits (max_active_stories, max_active_tasks_per_story, max_parallel_jobs, etc.) and any flow state machine overrides.
- **Value and WSJF:** business_value_score, time_criticality_score, risk_reduction_score, effort_estimate_points (and optionally computed WSJF) on features/stories.
- **Dependencies:** depends_on_ids, blocks_ids on work items (or a separate work_item_dependencies table).
- **Flow metrics:** tables or materialized views for lead time, cycle time, throughput, WIP by state (or enterprise service computes these on read from work_item_events).
- **Retrospective / improvement:** tables for retrospective data and improvement suggestions if stored in DB.
- **Linkage enforcement:** Enterprise MUST ensure proper linkages between project plan, epics, stories, tasks, subtasks, and jobs (referential integrity, traversal APIs, lineage queries); all such FKs and constraints live in enterprise schema.

Open code remains unaware of enterprise tables.
Enterprise code reads/writes them and implements the extension-point interfaces (e.g. WIP check reads WIP config and current counts from enterprise tables).

### 4.3 Backward Compatibility

- Open source deployments run without any enterprise migrations; they never see enterprise-only columns/tables.
- Paid deployments run open migrations first, then enterprise migrations.
  Open code paths that call extension points get no-op behavior if the enterprise module is not loaded, and real behavior when it is.

## 5. API and Gateway Strategy

- **Open APIs:** Projects, project plans, tasks, jobs, and status transitions only.
  Plan CRUD and lock at project scope only.
  Documented in open tech specs and OpenAPI.
- **Enterprise APIs:** Full CRUD and status for work items (epics, features, stories, tasks, subtasks); plan CRUD and lock at epic scope (epic-as-sub-plan); requirements and acceptance criteria; WIP configuration, flow metrics, value/WSJF updates, dependency CRUD, retrospective endpoints, and any policy/approval hooks that depend on Lean behavior.
  Enterprise MUST expose APIs for full hierarchy traversal and linkage: project plan -> epics -> stories -> tasks -> subtasks -> jobs (lineage, bulk updates, consistency checks).
  Documented in enterprise-only spec; listed in main API index as optional with "enterprise" or "paid" marker.
- **Gateway behavior:** When an enterprise endpoint is invoked and the enterprise module is not enabled, return 403 or 501 with a clear code (e.g. feature_not_licensed).
  This allows clients to degrade gracefully.

## 6. Build, Distribution, and Licensing

- **Open source:** Standard build from this repo (e.g. `just build`); binaries ship with no-op implementations for flow/WIP/prioritization/metrics/capacity.
  No license check for core features.
- **Enterprise:** Build that includes the proprietary module (from separate repo or build-tag path).
  At runtime, enable Lean behavior only when a valid enterprise license or tenant feature flag is present; otherwise behave as open source.
- **Releases:** Open source releases are public.
  Enterprise releases are distributed to paying customers only (private repo, artifact store, or customer-specific delivery).

## 7. Implementation Phases

1. **Phase 1 (Open):** Implement base project/tasks and project plan per recommendations doc (including at most one plan per project, plan lock, RBAC for lock/unlock).
   Add requirements and tech spec updates for PROJCT, PMAGNT, AGENTS, projects_and_scopes, project_manager_agent, postgres_schema; multi-message clarification in chat/PM specs.
2. **Phase 2 (Open):** Define extension-point interfaces in orchestrator (flow/WIP, prioritization, metrics, capacity).
   Implement no-op or default implementations.
   Document the extension contract in a tech spec (e.g. "Orchestrator extension points for work flow and metrics") so enterprise can implement against it.
3. **Phase 3 (Enterprise):** In proprietary codebase, implement all agile constructs: work item hierarchy (epics, features, stories, tasks, subtasks), epic-as-sub-plan (same plan constructs at epic scope), requirements and acceptance criteria (MVP), and proper linkages (project plan -> epics -> stories -> tasks -> subtasks -> jobs).
   Schema, APIs, and PMA decomposition for work items; job-to-subtask/story linkage when enterprise is enabled.
4. **Phase 4 (Enterprise):** Implement Lean Agile per addendum2: WIP enforcement, flow state machine, value/WSJF fields and logic, flow metrics, pull-based behavior, dependency enforcement, capacity awareness, stop-the-line and governance.
   Implement the open extension-point interfaces; register enterprise Lean APIs; document and test linkage behavior across the full hierarchy.
5. **Phase 5 (Enterprise):** License or feature-flag wiring; enterprise migrations; packaging and distribution for paid customers.

## 8. Traceability to Source Specs

- **Plan element (open):** Base project/tasks only, project plan (one per project), plan lock, RBAC lock/unlock, PMA behavior, Markdown, client-editable
  - source: [2026-02-27_recommendations_tasks_projects_pma_spec_updates.md](2026-02-27_recommendations_tasks_projects_pma_spec_updates.md)
- **Plan element (enterprise):** Epic/Feature/Story/Task/Sub-task, work item tables, PM decomposition, Job mapping
  - source: [agile_pm_rough_spec.md](../draft_specs/agile_pm_rough_spec.md)
- **Plan element (enterprise):** Epics use same plan constructs as project plans (epics as sub-plans)
  - source: [2026-02-27_recommendations_tasks_projects_pma_spec_updates.md](2026-02-27_recommendations_tasks_projects_pma_spec_updates.md), [agile_pm_rough_spec.md](../draft_specs/agile_pm_rough_spec.md)
- **Plan element (enterprise):** Requirements, acceptance criteria, Story linkage, validation gating
  - source: [agile_pm_rough_spec_addendum.md](../draft_specs/agile_pm_rough_spec_addendum.md)
- **Plan element (enterprise):** WIP, flow, value/WSJF, pull, metrics, improvement, dependencies, capacity, governance
  - source: [agile_pm_rough_spec_addendum2_lean_agile.md](../draft_specs/agile_pm_rough_spec_addendum2_lean_agile.md)
- **Plan element (enterprise):** Proper linkages between project plan, epics, stories, tasks, subtasks, jobs (referential integrity, traversal APIs, lineage)
  - source: this plan (enterprise requirement)

---

This plan is a proposal for discussion.
Decisions on first-class vs task-centric project plan, choice of Option A/B/C for enterprise code location, and exact interface signatures should be made before implementation.
Those decisions should be reflected in normative requirements and tech specs.
