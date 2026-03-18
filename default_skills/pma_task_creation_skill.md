---
name: pma-task-creation
description: Guides the Project Manager Agent (PMA) to create and update task payloads (fields, steps, requirements, comments) for MCP. Use when building or updating tasks as part of a plan.
ingest_skills:
  - pma-requirement-object
---

# PMA Task Creation Skill

- [Purpose](#purpose)
- [Use This Skill When](#use-this-skill-when)
- [Core Rules](#core-rules)
- [Task Payload Shape](#task-payload-shape)
- [Schema Guidance (MCP Help)](#schema-guidance-mcp-help)
- [Task Creation Checklist](#task-creation-checklist)
- [Skills to Ingest](#skills-to-ingest)
- [References](#references)

## Purpose

Guide the PMA to build **task create/update payloads** that conform to the host schema when creating or updating tasks via MCP.
This skill covers task identity, content, steps, optional requirements array and comments; for requirement **object** structure ingest and follow skill `pma-requirement-object`.

## Use This Skill When

- Creating tasks via MCP as part of building or updating a plan.
- Updating existing tasks (description, acceptance_criteria, steps, post_execution_notes, comments, status) when the plan is not locked or when lock rules allow (e.g. completion status and comments only when locked).

## Core Rules

1. **Entity ids are system-managed:** Never supply the task's own `id`; the orchestrator assigns it.
   Supply `plan_id` only when associating with an existing plan (use the plan id returned when the plan was created).
2. **Use MCP only:** All task read/write goes through the orchestrator MCP gateway.
3. **Schema compliance:** Before building payloads, call `task.help` (or host-equivalent) to get current task structure, required/optional fields, and host-specific rules (e.g. task naming, steps map format).
4. **No simulated output:** Use only real MCP tool results; do not invent or assume values.
5. **Clarify before ready:** Before marking a task as `planning_state=ready`, attempt to clarify ambiguous scope, acceptance criteria, or execution order with the user.
   Clarification is in the thread where the user directed task creation, or via notification to the user (Notification spec TBD).
   Do not mark ready until the task is sufficiently specified or clarification has been attempted.

## Task Payload Shape

When creating or updating a task via MCP, the payload MUST conform to the host schema.
Typical structure (confirm with `task.help` or host tech specs):

- **Identity and scope:** Do not supply `id`.
  Supply `plan_id` when associating with an existing plan.
  Supply normalized `name` (lowercase, single dashes; unique within scope per host rules).
- **Content:** `description` (Markdown), `acceptance_criteria` (per host schema).
- **Persona and skills:** One persona per task.
  When the host supports it, supply `persona_id` (uuid; resolve via `persona.list` / `persona.get`) and optional `recommended_skill_ids` (array of skill stable identifiers).
  Execution-ready tasks should have `persona_id` set when the host uses personas for job building.
- **Requirements:** `requirements` (optional) - array of requirement objects; structure per skill `pma-requirement-object`.
- **Steps:** `steps` (required, non-empty map per host): keys = numeric step IDs (e.g. 10, 20, 30 so steps can be inserted between); each value = step object with `complete` (boolean) and `description` (string).
  When creating, set all steps `complete: false`; executors set `complete: true` as steps finish.
  When reading steps, sort by numeric key ascending.
- **Optional:** `post_execution_notes` (Markdown, typically set after execution or during closeout), `comments` (per host structure; when plan is locked, only status and comments may be updated per lock rules).

Do not send payloads that omit required fields (e.g. non-empty `steps`) or use invalid step keys.

## Schema Guidance (MCP Help)

- **Before building tasks:** Call `task.help` (or the host's equivalent MCP tool) to retrieve the task structure, required and optional fields, and host-specific rules (e.g. task naming, steps map format, requirement object shape).
  Use this output to build compliant task create/update payloads.

If the host does not expose `task.help`, follow the canonical task schema defined in the host's tech specs or instructions.

## Task Creation Checklist

When creating or updating a task via MCP:

1. Do not supply the task's own `id`; supply `plan_id` when associating with an existing plan.
2. Supply normalized `name` (lowercase, single dashes; unique within scope).
3. Set `description` (Markdown) and `acceptance_criteria` per host schema.
4. When the host supports personas: use `persona.list` / `persona.get` to resolve and set `persona_id`; set optional `recommended_skill_ids` (array of skill stable ids) when appropriate.
5. Set `requirements` (optional) using requirement objects per skill `pma-requirement-object`.
6. Set `steps` (required, non-empty map): numeric IDs (10, 20, 30, …), each value `{ "complete": false, "description": "<string>" }` when creating.
7. Set `post_execution_notes` or `comments` when appropriate (e.g. after execution or when adding comments).
8. **Before marking a task ready:** Attempt to clarify ambiguous scope, acceptance criteria, or execution order with the user (in the creating thread or via notification per host); do not transition to ready until sufficiently specified or clarification has been attempted.
9. When handing off work for execution: the host may support bundles of 1-3 tasks (same persona, dependency order) for SBA; form such bundles when it fits the plan and the host's job builder expects them.

When reading back steps, sort by numeric key ascending.

## Skills to Ingest

When using this skill, ingest the following skill (by frontmatter `name`) so the agent has the needed guidance:

- `pma-requirement-object` - for building requirement objects in a task's `requirements` array.

## References

The host project defines the task schema (fields, steps map, requirement object shape), MCP tool names for task create/update/list/get, and lock rules (what may be updated when the plan is locked).
