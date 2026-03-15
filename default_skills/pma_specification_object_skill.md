---
name: pma-specification-object
description: Guides the Project Manager Agent (PMA) to build specification objects for referencing technical specifications (e.g. on a task or in plan content). Use when populating spec references when creating or updating tasks or plans via MCP.
ingest_skills: []
---

# PMA Specification Object Skill

- [Purpose](#purpose)
- [Use This Skill When](#use-this-skill-when)
- [Core Rules](#core-rules)
- [Specification Object Shape](#specification-object-shape)
- [Schema Guidance (MCP Help)](#schema-guidance-mcp-help)
- [Skills to Ingest](#skills-to-ingest)
- [References](#references)

## Purpose

Guide the PMA to build **specification objects** that conform to the host schema when referencing technical specifications (e.g. in a task's `specifications` array, in plan body "Requirements and Specifications" sections, or in task payload metadata).
This skill is parallel to `pma-requirement-object`: requirements state what is required; specification objects point to the technical specs that define how to implement or verify it.

## Use This Skill When

- Populating a task's `specifications` array (or equivalent) when creating or updating a task via MCP, when the host schema supports it.
- Adding or editing specification references on an existing task (when plan is not locked or when lock rules allow).
- Building plan or task content that must list canonical spec docs (e.g. "Task N Requirements and Specifications") in a structured way the host can store or validate.

## Core Rules

1. **Schema compliance:** Before building specification objects, call `specification.help` (or host-equivalent) to get current specification object structure and examples.
2. **Required field:** Each specification object MUST include a way to identify the spec (e.g. `description` or `ref` or `path` per host schema) - the spec reference or title.
3. **No simulated output:** Use only real MCP tool results and host schema; do not invent or assume values.

## Specification Object Shape

Each element in a task's `specifications` array (or equivalent) MUST be a specification object when the host supports it.
Typical structure (confirm with `specification.help` or host tech specs):

- **Required:** At least one of `description` (string, Markdown summary or spec title) or `ref` (string, spec identifier e.g. doc path, spec id, or CYNAI.SCHEMA.X).
- **Optional:** `source` (string, path or URL to the spec doc), `type` (string, e.g. tech_spec, api_spec), `section` (string, section or anchor within the spec).

Use `specification.help` to confirm the host's exact field names and semantics (e.g. allowed `type` values, whether `ref` is path vs id).
Preserve array order when updating; consumers may sort by `ref` or `source` for display when present.

## Schema Guidance (MCP Help)

- **Before building specification objects:** Call `specification.help` (or the host's equivalent MCP tool) to retrieve the specification object structure (required and optional fields) and examples.
  Use this output to build compliant `specifications` arrays (or equivalent) for tasks.

If the host does not expose `specification.help`, follow the canonical specification object structure defined in the host's tech specs or instructions.
If the host has no separate specifications array, specification references may be expressed in task content (e.g. description or acceptance_criteria) or as requirement objects with `type: spec`; follow host guidance.

## Skills to Ingest

When using this skill, no other skills are required; this skill is typically used together with skill `pma-task-creation` when building task payloads that include spec references.

## References

The host project defines the specification object schema (required and optional fields, allowed values) and MCP tool behavior for `specification.help` (or equivalent).
Where requirements and specs live (e.g. `docs/requirements/`, `docs/tech_specs/`) is defined in the project or instructions.
