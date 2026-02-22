# Documentation Standards

## Purpose

This directory defines **documentation standards** for the CyNodeAI project.

It describes how documentation (specs, requirements, READMEs, and other Markdown) should be structured, formatted, and maintained so that docs stay consistent and easy to navigate across the repo.

- **Single source of truth** for doc conventions (structure, style, linking, tooling).
- **Onboarding** for contributors and tooling (e.g. lint, validation scripts) so everyone follows the same rules.
- **Alignment** with existing automation (e.g. `just lint-md`) and future checks.

## Where the Standards Live

The standards themselves are **not** written in this README.
They are kept in separate documents in this directory and linked from here.
That keeps this file as an index and keeps each standard focused and easy to update.

When a standard is added, it will be listed below with a short description and link.

- [Markdown conventions](markdown_conventions.md) - Repository-wide Markdown formatting rules (including heading numbering and uniqueness).
- [Requirements domains](requirements_domains.md) - Canonical list of requirement/spec domains (e.g. for Spec IDs and REQ IDs); single source of truth for domain tags and requirements file mapping.
- [Spec authoring, writing and validation](spec_authoring_writing_and_validation.md) - How to write and validate specs and feature files (traceability, BDD, conventions).

## Relation to Other Docs

- **Technical specs**: [`../tech_specs/`](../tech_specs/) - follow these standards where applicable.
- **Requirements**: When used, requirement docs live under `docs/requirements/`; follow the same standards.
- **Validation**: Markdown linting is provided by `just lint-md` (see the project justfile).
  Additional doc-validation workflows or scripts may be added; conventions in this directory apply when they are.
