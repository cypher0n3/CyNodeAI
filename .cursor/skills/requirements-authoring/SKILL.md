---
name: requirements-authoring
description: Applies project requirements-doc standards when writing or editing docs in docs/requirements. Use when creating or editing requirement entries, REQ-* IDs, or when the user asks to write or fix requirements.
---

# Requirements Authoring

## Overview

Follow the project's spec authoring and markdown conventions.
Canonical source: [docs/docs_standards/spec_authoring_writing_and_validation.md](../../docs/docs_standards/spec_authoring_writing_and_validation.md) and [markdown_conventions.md](../../docs/docs_standards/markdown_conventions.md).

## Before Writing

- Use domains from [requirements_domains.md](../../docs/docs_standards/requirements_domains.md).
  Requirement IDs: `REQ-<DOMAIN>-<NNNN>` (4 digits).
  File per domain: `docs/requirements/<domain>.md` (domain lowercased).

## Requirements Rules (Summary)

- Requirements define **what** is required; implementation-agnostic.
  Use RFC-2119 (MUST, SHOULD, etc.) here only.
- Atomic and testable; prefer one obligation per REQ entry.
- Each entry: list item with continuation lines for spec reference(s) and requirement anchor.

## Entry Format

```markdown
- REQ-<DOMAIN>-<NNNN>: <short label>.
  [CYNAI.DOMAIN.SpecName](../tech_specs/file.md#spec-cynai-domain-specname)
  <a id="req-<domain>-<nnnn>"></a>
```

- One or more spec reference links (Spec ID as link text, href to `spec-*` anchor).
- Requirement anchor on its own continuation line after spec links: `<a id="req-<domain>-<nnnn>"></a>` (lowercase domain and number).

## Allowed Inline HTML

- Requirement entry anchors only: `<a id="req-..."></a>` on a continuation line under the requirement list item (after the REQ line and after any spec reference links).

## After Editing

- Run `just lint-md` (or `just lint-md 'path/to/file.md'`) and fix issues.
