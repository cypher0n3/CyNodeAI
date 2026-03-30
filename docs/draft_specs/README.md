# Draft Specs

## Overview

This directory holds **draft** and **proposal** specifications that are under consideration but have **not** been accepted into the project's canonical documentation.

Draft specs are incubators: they allow exploration of design options, new features, or cross-cutting concerns before being refined into requirements and technical specifications.

## Directory Layout

- **`README.md` (this file):** Navigation and conventions for the whole tree.
- **Root (`docs/draft_specs/*.md`):** Active proposals and designs.
  Filenames prefixed with `NNN_` use **increments of 10** (e.g. `010_`, `020_`, `030_`) so new drafts can be inserted between existing numbers without renumbering the whole queue.
    The number indicates **integration priority** (lower = integrate into canonical docs sooner). `README.md` in this directory is not numbered.
- **[`integrated/`](integrated/README.md):** Drafts whose normative content already lives in `docs/tech_specs/` (and requirements as applicable); kept for history only.
- **[`partial/`](partial/):** Drafts that are **partly** reflected in canonical docs; promotion or merge work remains.

## Relationship to Canonical Documentation

CyNodeAI uses two canonical layers (see [meta.md](../../meta.md) and [docs/README.md](../README.md)):

- **Requirements** ([docs/requirements/](../requirements/)): normative "what" (RFC-2119 obligations, acceptance criteria).
- **Technical specifications** ([docs/tech_specs/](../tech_specs/)): prescriptive "how" (architecture, contracts, algorithms).

Content in `draft_specs/` is **not** canonical.

It does not impose obligations on implementation and is not referenced by validators or traceability as authority.

When a draft is **accepted**, its content should be:

- Split and restated as appropriate into `docs/requirements/` (obligations) and `docs/tech_specs/` (implementation guidance).
- Written to satisfy [spec authoring and validation standards](../docs_standards/spec_authoring_writing_and_validation.md).
- Removed or archived from `draft_specs/` so that the single source of truth lives in the canonical locations.

## What Belongs Here

- New feature or capability proposals that need review before becoming requirements/specs.
- Alternative designs (e.g. messaging, storage, security) under evaluation.
- Rough or partial specs that are not yet ready for the formal structure required in [docs/tech_specs/](../tech_specs/).
- Addenda or alternatives to existing specs that are still under discussion.

## What Does Not Belong Here

- Accepted design that is already (or should be) in `docs/requirements/` or `docs/tech_specs/`.
- Implementation notes or ad-hoc design decisions; use [dev_docs/](../dev_docs/) for temporary development notes.
- Gherkin/BDD feature files; those live under [features/](../../features/).

## Conventions for Drafts

- Follow [Markdown conventions](../docs_standards/markdown_conventions.md) and keep docs ASCII-only unless explicitly allowed.
- For consistency with future promotion, consider aligning draft structure with [spec authoring standards](../docs_standards/spec_authoring_writing_and_validation.md) (e.g. clear purpose, traceability placeholders).
- Use a short, descriptive filename; new proposals in the root directory may be assigned the next free `NNN_` slot in steps of 10 to fit the integration queue (see [Directory Layout](#directory-layout)).
- Prefer one main topic per document; use addenda or numbered follow-ups if you need to extend an existing draft.

## Cross-References

- Documentation hub: [docs/README.md](../README.md).
- Requirements: [docs/requirements/](../requirements/README.md).
- Tech specs index: [docs/tech_specs/_main.md](../tech_specs/_main.md).
- Spec authoring: [docs/docs_standards/spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md).
- Project meta: [meta.md](../../meta.md).
