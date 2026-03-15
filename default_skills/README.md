# Default Skills

- [Overview](#overview)
- [Standards](#standards)
  - [File Naming](#file-naming)
  - [Registry Name and Front Matter](#registry-name-and-front-matter)
  - [Document Structure](#document-structure)
  - [Atomic; No Cross-Document Links](#atomic-no-cross-document-links)
  - [Markdown and References](#markdown-and-references)
  - [Content Rules](#content-rules)
- [Out of Scope Here](#out-of-scope-here)
- [Contents](#contents)

## Overview

This directory holds **default skills** that the orchestrator system includes and that agents can pull in when needed.
These are optional, domain-specific skills (e.g. plan creation, task creation, requirement or specification objects), not the baseline startup skill.

## Standards

Skills in this directory MUST follow these conventions so they can be stored, registered, and exposed consistently by the orchestrator (see [docs/tech_specs/skills_storage_and_inference.md](../docs/tech_specs/skills_storage_and_inference.md)).

### File Naming

- Use lowercase with underscores; suffix filename with `_skill.md` (e.g. `pma_plan_creation_skill.md`).
- One skill per file.

### Registry Name and Front Matter

- Each skill SHOULD have YAML front matter at the top of the file.
- **name** (required in front matter): kebab-case identifier used in the skill registry and when referencing the skill (e.g. `pma-plan-creation`).
  Must be unique across default skills.
- **description** (required in front matter): one-line summary for discovery and ingestion; state when to use the skill.
- **ingest_skills** (optional): list of registry names (kebab-case) of other skills the agent should load when using this skill (e.g. `pma-task-creation`, `pma-requirement-object`).

### Document Structure

- **H1:** Single title that describes the skill (e.g. "PMA Plan Creation Skill").
- **Purpose:** Section that states the skill's goal and scope in one or two sentences.
- **Use This Skill When:** Bullet list of situations that trigger use of this skill.
- **Core Rules:** Numbered list of obligations (MUST/SHOULD); include "no simulated output" and "MCP only" where the skill involves tool use.
- Further sections as needed (workflows, payload shapes, schema guidance, references).
- Optional table of contents after the H1 for long skills.

### Atomic; No Cross-Document Links

- Skill files MUST be **atomic**: self-contained, with no Markdown links to other documents (no `[text](path)` or `[text](path#anchor)` to other files or URLs).
- Reference other skills only by their **registry name** in backticks (e.g. `pma-task-creation`); the runtime resolves ingestion.
- Do not embed links to tech_specs, requirements, or any external doc inside a skill file; keep the skill content standalone.

### Markdown and References

- Follow repository [Markdown conventions](../docs/docs_standards/markdown_conventions.md): one sentence per line, blank line after headings and lists, no pseudo-headings.
- For in-document navigation use same-file anchors only (e.g. `[Purpose](#purpose)`).

### Content Rules

- Skills that involve plan/task/data operations MUST require the agent to use MCP only (no direct DB or filesystem writes for orchestrator-managed state).
- Do not instruct the agent to guess or simulate tool output; require real results and explicit error or gap reporting.

## Out of Scope Here

The **default CyNodeAI interaction skill** (how to access MCP, skills tools, task and project context, sandbox conventions) is maintained separately.
See [agents/instructions/default_cynodeai_skill.md](../agents/instructions/default_cynodeai_skill.md) and [docs/tech_specs/skills_storage_and_inference.md](../docs/tech_specs/skills_storage_and_inference.md).

## Contents

- **PMA plan creation**: [pma_plan_creation_skill.md](pma_plan_creation_skill.md)
- **PMA task creation**: [pma_task_creation_skill.md](pma_task_creation_skill.md)
- **PMA requirement object**: [pma_requirement_object_skill.md](pma_requirement_object_skill.md)
- **PMA specification object**: [pma_specification_object_skill.md](pma_specification_object_skill.md)
