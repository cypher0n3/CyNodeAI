---
name: spec-authoring
description: Applies project tech-spec standards when writing or editing docs in docs/tech_specs or docs/draft_specs. Use when creating or editing technical specifications, spec items, or when the user asks to draft, write, or fix a tech spec.
---

# Spec Authoring

## Overview

Follow the project's spec authoring and markdown conventions.
Canonical source: [docs/docs_standards/spec_authoring_writing_and_validation.md](../../docs/docs_standards/spec_authoring_writing_and_validation.md) and [markdown_conventions.md](../../docs/docs_standards/markdown_conventions.md).

## Before Writing

- Read the spec authoring doc for Spec Items, anchors, and traceability.
- Use domains from [requirements_domains.md](../../docs/docs_standards/requirements_domains.md) for Spec IDs.

## Tech Spec Rules (Summary)

- Be **prescriptive, specific, and explicit**; no room for interpretation.
- Define contracts (interfaces, types, operations), algorithm logic, return values, status/error codes.
- Keep code minimal: signatures, constants, short snippets.
- Single source of truth; link to it from related specs.
- Do not add new RFC-2119 obligations in specs; reference requirements instead.
- Link each Spec Item to applicable `REQ-*` in a "Traces To" subsection at the end of the item.

## Spec Item Structure (Mandatory)

For each Spec Item:

1. **Heading**: Numbered per heading level (e.g. H2 => one segment, H3 => two).
   Format: `<Numbering> <Backticked Symbol> <Kind>`.
   Allowed kinds: Interface, Type, Operation, Rule, Field, Constant.
2. **Spec ID line** (first non-blank after heading): `- Spec ID: \`CYNAI.DOMAIN.PATH\` <a id="spec-cynai-domain-path"></a>`
   - Anchor: lowercase, dots to dashes, prefix `spec-`.
3. Optional metadata bullets: Status, Since, See also.
4. Contract content after the Spec ID block (Inputs, Outputs, Behavior, Error conditions, etc.).
5. For Algorithm subsections: add algorithm anchor on its own line after the Algorithm heading; step anchors at end of list items when needed.
6. Reference blocks: H5 (or H4 if Spec Item is H3), anchor line above the code block, format `##### N.N.N.X <Backticked Symbol> Reference - <Language>`.

## Inline HTML (Allowed Only)

- `<a id="spec-..."></a>` on Spec ID line.
- `<a id="ref-<lang>-..."></a>` above code blocks.
- `<a id="algo-..."></a>` and `<a id="algo-...-step-..."></a>` in Algorithm sections.

## After Editing

- Run `just lint-md` (or `just lint-md 'path/to/file.md'`) and fix issues.
