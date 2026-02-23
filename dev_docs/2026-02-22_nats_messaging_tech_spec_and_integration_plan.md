# NATS Messaging: Tech Spec, Requirements, and Platform Integration Plan

- [1. Summary](#1-summary)
- [2. Scope of the Draft Spec](#2-scope-of-the-draft-spec)
- [3. Requirements Strategy](#3-requirements-strategy)
- [4. Tech Spec Build-Out](#4-tech-spec-build-out)
- [5. Implementation Order](#5-implementation-order)
- [6. Draft Spec Location](#6-draft-spec-location)
- [7. Checklist for Authors](#7-checklist-for-authors)

## 1. Summary

**Date:** 2026-02-22  
**Source:** [docs/draft_specs/nats_messaging.md](../docs/draft_specs/nats_messaging.md)  
**Status:** Plan (not yet implemented)

This plan describes how to promote the NATS/JetStream draft into formal requirements and tech specs and integrate NATS as the platform's transport and event backbone for job dispatch, node presence, and domain events, while keeping PostgreSQL as the system of record.

## 2. Scope of the Draft Spec

The draft defines the following.

- **Subject taxonomy:** `cynode.*` prefix; job, workitem, requirement, policy, artifact, node subjects with tenant/project/job/node tokens.
- **JetStream streams:** CYNODE_JOBS, CYNODE_EVENTS, CYNODE_TELEMETRY with retention and ack policy.
- **Consumer patterns:** Dispatcher vs pull-claim for jobs; read-model updaters; indexing/embedding; live UX.
- **Message envelope:** event_id, event_type, event_version, occurred_at, producer, scope, correlation, payload.
- **Payload schemas:** Versioned payloads for job.requested/assigned/started/progress/completed, workitem.*, requirement.*, policy.*, etc.
- **RBAC:** NATS-level and message-level (scope, sensitivity).
- **Idempotency:** event_id dedupe; job_id-based worker dedupe.
- **MVP scope (draft Section 15):** job.requested, job.started, job.progress, job.completed; node.heartbeat, node.capacity; artifact.available; streams CYNODE_JOBS + CYNODE_TELEMETRY.

Current platform behavior: job dispatch is **HTTP-only** (orchestrator calls Worker API `POST /v1/worker/jobs:run`).
Integrating NATS will introduce an event-driven path alongside or instead of direct HTTP for dispatch and progress.

## 3. Requirements Strategy

Define where normative messaging requirements live and how they tie to ORCHES and WORKER.

### 3.1 New Requirements Domain: MESSG

Add a dedicated domain for messaging and event transport so that NATS/JetStream behavior is testable and traceable.

- **Domain tag:** `MESSG` (6 chars per [requirements_domains.md](../docs/docs_standards/requirements_domains.md)).
- **File:** `docs/requirements/messg.md`.
- **Scope:** At-least-once delivery, subject taxonomy, envelope and schema versioning, stream definitions, RBAC and scope validation, idempotency, payload size limits, and MVP subject/stream set.

#### 3.1.1 Suggested Requirement Areas (To Be Written as `REQ-MESSG-01xx`)

- Transport: NATS/JetStream as the event and job-dispatch transport; PostgreSQL remains authoritative for task/job state.
- Subject naming: All subjects under `cynode.*`; lowercase, dot-separated; tenant_id, project_id, job_id, node_id where applicable.
- Envelope: Every message has event_id, event_type, event_version, occurred_at, producer, scope, correlation; payload schema per event_type/event_version.
- Streams: CYNODE_JOBS, CYNODE_TELEMETRY (MVP); CYNODE_EVENTS (later); retention and ack policy as specified.
- Idempotency: Consumers MUST deduplicate by event_id; job execution MUST be idempotent by job_id.
- RBAC: Publish/subscribe restricted by tenant (and project where applicable); message-level scope/sensitivity validated by consumers.
- Payload size: Max message size enforced; large content via object storage URIs + hashes.
- MVP: Minimum subject and stream set as in draft Section 15.

#### 3.1.2 Required Actions

1. Add `MESSG` to `docs/docs_standards/requirements_domains.md` with one-line description and file path.
2. Add `MESSG` to `docs/requirements/README.md` in the requirements domains index.
3. Create `docs/requirements/messg.md` with atomic, testable REQ-MESSG-0001 and REQ-MESSG-01xx items, each linking to the new tech spec (and later to orchestrator/worker specs where relevant).

### 3.2 Cross-References From ORCHES and WORKER

- **ORCHES:** Add or extend requirements so that "dispatch to worker nodes" can be satisfied by publishing to NATS (job.requested / job.assigned) and consuming job lifecycle events (job.completed, job.failed), with references to the NATS tech spec and MESSG requirements.
  Scheduler and PMA-triggered work use the same dispatch contract.
- **WORKER:** Add or extend requirements so that nodes may consume job assignments via NATS (job.assigned or job.requested in pull-claim mode) and MUST publish job.started, job.progress, job.completed; node.heartbeat and node.capacity per NATS spec.
  Worker API HTTP MAY remain for sync run or callback-style result reporting where the implementation keeps it.

Ensure no conflict with existing REQ-ORCHES-0123 (dispatch via Worker API): either broaden 0123 to "dispatch via Worker API and/or NATS per tech spec" or add a new requirement that allows NATS as the primary dispatch transport with Worker API as one possible fulfillment.

## 4. Tech Spec Build-Out

Promote the draft into a prescriptive tech spec and wire it into orchestrator and worker specs.

### 4.1 New Tech Spec: `nats_messaging.md`

Create a single prescriptive tech spec that defines NATS subject taxonomy, streams, envelope, and payloads.

- **Location:** `docs/tech_specs/nats_messaging.md`.
- **Source:** Promote and tighten [docs/draft_specs/nats_messaging.md](../docs/draft_specs/nats_messaging.md).
- **Standards:** Follow [spec_authoring_writing_and_validation.md](../docs/docs_standards/spec_authoring_writing_and_validation.md): prescriptive language, spec IDs (`CYNAI.MESSG.*`), and "Traces To" links to REQ-MESSG-*(and REQ-ORCHES-* / REQ-WORKER-* where applicable).
- **Contents to formalize:**
  - Subject taxonomy (section per domain: job, workitem, requirement, policy, artifact, node) with exact subject patterns and producer/consumer roles.
  - JetStream stream definitions: name, subjects, retention (WorkQueue vs Time), max age, ack policy.
  - Consumer patterns: Pattern A (dispatcher assigns) vs Pattern B (pull-claim); read-model updaters; telemetry/live UX.
  - Message envelope: required fields, types, and validation rules.
  - Payload schemas: one subsection per event_type + version (e.g. job.requested v1.0.0) with explicit field list and semantics.
  - RBAC: NATS account/permission model; message-level scope/sensitivity and consumer validation.
  - Idempotency: event_id storage (e.g. processed_events table); job_id dedupe at worker.
  - Ordering and consistency: no global ordering; per-job locality; tolerance for duplicates and out-of-order progress.
  - Payload size limits and object-storage references.
  - Operational defaults (max age, heartbeat rate, progress rate).
  - MVP scope: list of subjects and streams that MUST be implemented first; rest incremental.

Add an implementation checklist (JSON schemas, envelope validation library, idempotency store, subject permissions) as a non-normative subsection or reference to a follow-up implementation doc.

### 4.2 Index and Grouping

- In `docs/tech_specs/_main.md`, add a new index section (e.g. **Messaging and Events**) and link `nats_messaging.md`.
- Alternatively, list it under **Protocols and Standards** or **Orchestrator and Nodes** if that fits the existing grouping better; "Messaging and Events" keeps NATS easy to find.

### 4.3 Orchestrator Spec Updates

- In `docs/tech_specs/orchestrator.md`:
  - Describe the **dispatch path over NATS**: scheduler or PMA produces job.requested; dispatcher (orchestrator component) assigns to a node and publishes job.assigned; orchestrator consumes job.started, job.progress, job.completed, job.failed, job.canceled and updates task/job state in PostgreSQL.
  - Reference `nats_messaging.md` for subject names, envelope, and stream.
  - State that the same node selection and job-dispatch contracts (REQ-ORCHES-0107, REQ-ORCHES-0123) apply whether the transport is HTTP or NATS.
  - Optional: mention cron/scheduler publishing to NATS (e.g. internal events for scheduled run handoff to PMA) if we define such subjects.

### 4.4 Worker Node and Worker API Spec Updates

- In `docs/tech_specs/worker_node.md`:
  - **Registration / presence:** Node publishes node.heartbeat and node.capacity to NATS (subjects and payload per nats_messaging.md).
    Orchestrator may use these for liveness and capacity in addition to or instead of HTTP.
  - **Job consumption:** Node subscribes to job.assigned for its node_id (Pattern A) or to job.requested in a queue group (Pattern B).
    On receipt, it runs the job (same sandbox contract as today), then publishes job.started, job.progress, job.completed (or job.failed).
  - **Idempotency:** Node MUST deduplicate by job_id (e.g. local SQLite) and re-emit job.completed with existing result if duplicate.
  - Link to `nats_messaging.md` and to Worker API for the actual run contract (payloads, timeouts, result shape).

- In `docs/tech_specs/worker_api.md`:
  - **Clarify transport:** Either (a) keep HTTP `POST /v1/worker/jobs:run` as the only MVP path and document NATS as a future alternative, or (b) define that job execution is triggered by NATS consumption and HTTP is used only for sync run or for result callback.
    Recommendation: document both; MVP can keep HTTP dispatch and add NATS in parallel for progress/telemetry (node.heartbeat, node.capacity, job.progress) first, then switch dispatch to NATS in a later phase.
  - Ensure job lifecycle (accepted, in_progress, completed) and result persistence semantics are satisfied whether the job was received via HTTP or NATS.

## 5. Implementation Order

Recommended sequence for requirements, tech spec, and integration docs.

### 5.1 Requirements Phase

- Add MESSG domain and `docs/requirements/messg.md`.
- Add or update ORCHES/WORKER requirements for NATS dispatch and node presence/events.

### 5.2 Tech Spec Phase

- Create `docs/tech_specs/nats_messaging.md` from the draft with spec IDs and traceability.
- Update `docs/tech_specs/_main.md`.

### 5.3 Integration Specs Phase

- Update `orchestrator.md` (dispatch and consumption over NATS).
- Update `worker_node.md` (NATS subscription and publishing).
- Update `worker_api.md` (transport clarification and lifecycle).

### 5.4 Implementation (Out of Scope for This Plan)

- Shared Go (or cross-service) envelope and payload schema validation; JetStream stream creation; idempotency store (Postgres + worker SQLite); NATS permissions per service/tenant; then orchestrator dispatcher and worker node subscribers/publishers.

## 6. Draft Spec Location

The draft remains the source for the formal tech spec until the latter is approved.

Keep `docs/draft_specs/nats_messaging.md` as the historical draft until `docs/tech_specs/nats_messaging.md` is approved and complete; then either remove the draft or add a short note at the top pointing to the tech spec.

## 7. Checklist for Authors

- [ ] Add MESSG to requirements_domains.md and README requirements index.
- [ ] Create messg.md with REQ-MESSG-0001 and REQ-MESSG-01xx.
- [ ] Create tech spec nats_messaging.md with `CYNAI.MESSG.*` spec IDs and Traces To REQ-MESSG-*.
- [ ] Update _main.md with link to nats_messaging.md.
- [ ] Update orchestrator.md (NATS dispatch and event consumption).
- [ ] Update worker_node.md (NATS publish/subscribe, idempotency).
- [ ] Update worker_api.md (transport and lifecycle).
- [ ] Resolve REQ-ORCHES-0123 wording (Worker API and/or NATS).
- [ ] Run `just docs-check` (or `just ci` if other files change).
