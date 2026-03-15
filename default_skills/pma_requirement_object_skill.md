---
name: pma-requirement-object
description: Guides the Project Manager Agent (PMA) to build requirement objects for a task's requirements array. Use when populating task.requirements when creating or updating tasks via MCP.
ingest_skills: []
---

# PMA Requirement Object Skill

- [Purpose](#purpose)
- [Use This Skill When](#use-this-skill-when)
- [Core Rules](#core-rules)
- [Requirement Object Shape](#requirement-object-shape)
- [Schema Guidance (MCP Help)](#schema-guidance-mcp-help)
- [Skills to Ingest](#skills-to-ingest)
- [References](#references)

## Purpose

Guide the PMA to build **requirement objects** that conform to the host schema when populating a task's `requirements` array in a task create/update payload (skill `pma-task-creation` covers the full task payload).

## Use This Skill When

- Populating a task's `requirements` array when creating or updating a task via MCP.
- Adding or editing requirement entries on an existing task (when plan is not locked or when lock rules allow).

## Core Rules

1. **Schema compliance:** Before building requirement objects, call `requirement.help` (or host-equivalent) to get current requirement object structure and examples.
2. **Required field:** Each requirement object MUST include `description` (string, Markdown) - the requirement statement.
3. **No simulated output:** Use only real MCP tool results and host schema; do not invent or assume values.

## Requirement Object Shape

Each element in a task's `requirements` array MUST be a requirement object.
Typical structure (confirm with `requirement.help` or host tech specs):

- **Required:** `description` (string, Markdown) - the requirement statement.
- **Optional:** `ref` (string, content reference e.g. REQ-X-001), `source` (string), `type` (string), `priority` (string or number).

Use `requirement.help` to confirm the host's exact field names and semantics (e.g. allowed `type` or `priority` values).
Preserve array order when updating; consumers may sort by `ref` for display when present.

## Schema Guidance (MCP Help)

- **Before building requirements:** Call `requirement.help` (or the host's equivalent MCP tool) to retrieve the requirement object structure (e.g. `description` required; optional `ref`, `source`, `type`, `priority`) and examples.
  Use this output to build compliant `requirements` arrays for tasks.

If the host does not expose `requirement.help`, follow the canonical requirement object structure defined in the host's tech specs or instructions.

## Skills to Ingest

When using this skill, no other skills are required; this skill is typically used together with skill `pma-task-creation` when building task payloads.

## References

The host project defines the requirement object schema (required and optional fields, allowed values) and MCP tool behavior for `requirement.help` (or equivalent).
