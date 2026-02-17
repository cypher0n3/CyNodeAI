---
alwaysApply: true
---

# AI Instructions

## General Rules

- Always check existing files before making changes.
- When creating new files, use `touch` to create the file first, then edit it.
- Check the actual date using the `date` command before writing the date.
- See [meta.md](../meta.md) for basic project info.
- See [ai_files/](../ai_files/) for AI assisted coding instructions.
- See [docs/requirements/](../docs/requirements/) for canonical requirements ("what").
- See [docs/tech_specs/](../docs/tech_specs/) for technical specifications and implementation guidance ("how").

## Tech Specs vs. Implementation

In any case where there are deviations between the requirements (`docs/requirements/`) and the actual implementation,
the implementation MUST be brought into compliance with the requirements.

Tech specs (`docs/tech_specs/`) describe implementation guidance and should trace back to requirements.
If requirements are unclear or would cause issues during implementation, STOP and ASK what you should do.

## Available Tooling

The repository provides a [`justfile`](../justfile) for developer tooling.
**Always use justfile targets instead of running commands directly.**

Key targets:

- **`just ci`** - Run all CI checks locally (use before every commit)
